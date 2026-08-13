//go:build ignore

package main

import (
	"encoding/json"
	"math/big"
	"sort"
	"strings"

	bc "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/BlockchainComponent"
)

const NATIVE = "lqd"

type StrategyVault struct{}

type withdrawalRequest struct {
	ID           string `json:"id"`
	Owner        string `json:"owner"`
	Shares       string `json:"shares"`
	MinOutBps    int64  `json:"min_out_bps"`
	RequestedAt  int64  `json:"requested_at"`
	ExecutableAt int64  `json:"executable_at"`
	Status       string `json:"status"`
}

type rebalanceJob struct {
	ID          string `json:"id"`
	FromPair    string `json:"from_pair"`
	ToPair      string `json:"to_pair"`
	LPAmount    string `json:"lp_amount"`
	MinSwapOut  string `json:"min_swap_out"`
	Assigned    string `json:"assigned"`
	ScheduledAt int64  `json:"scheduled_at"`
	FallbackAt  int64  `json:"fallback_at"`
	Deadline    int64  `json:"deadline"`
	Status      string `json:"status"`
	Executor    string `json:"executor,omitempty"`
	ExecutedAt  int64  `json:"executed_at,omitempty"`
}

func parseBig(v string) *big.Int {
	v = strings.TrimSpace(v)
	z := new(big.Int)
	if v == "" {
		return z
	}
	if _, ok := z.SetString(v, 10); !ok {
		return big.NewInt(0)
	}
	return z
}

func norm(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}

func actorAddr(ctx *bc.Context) string {
	actor := strings.TrimSpace(ctx.OriginAddr)
	if actor == "" {
		actor = ctx.CallerAddr
	}
	return norm(actor)
}

func isNative(token string) bool {
	return norm(token) == NATIVE
}

func requirePositive(ctx *bc.Context, label string, amt *big.Int) {
	if amt == nil || amt.Sign() <= 0 {
		ctx.Revert(label + " must be > 0")
	}
}

func (v *StrategyVault) requireManager(ctx *bc.Context) {
	caller := norm(ctx.CallerAddr)
	origin := norm(ctx.OriginAddr)
	manager := norm(ctx.Get("manager"))
	owner := norm(ctx.OwnerAddr)
	if manager == "" {
		manager = owner
	}
	if caller != manager && origin != manager && caller != owner && origin != owner {
		ctx.Revert("manager only")
	}
}

func (v *StrategyVault) totalShares(ctx *bc.Context) *big.Int {
	return parseBig(ctx.Get("totalShares"))
}

func (v *StrategyVault) shareOf(ctx *bc.Context, addr string) *big.Int {
	return parseBig(ctx.Get("share:" + norm(addr)))
}

func (v *StrategyVault) positionOf(ctx *bc.Context, pair string) *big.Int {
	return parseBig(ctx.Get("position:" + norm(pair)))
}

func (v *StrategyVault) erc4626TotalAssets(ctx *bc.Context, asset string) *big.Int {
	result, err := ctx.Call(norm(asset), "BalanceOf", []string{ctx.ContractAddr})
	if err != nil || result == nil || !result.Success {
		ctx.Revert("canonical ERC-4626 asset balance unavailable")
	}
	assets := parseBig(result.Output)
	if assets.Sign() < 0 {
		ctx.Revert("canonical ERC-4626 asset returned invalid balance")
	}
	return assets
}

func (v *StrategyVault) setShare(ctx *bc.Context, addr string, amt *big.Int) {
	ctx.Set("share:"+norm(addr), amt.String())
}

func (v *StrategyVault) shareAllowance(ctx *bc.Context, owner, spender string) *big.Int {
	return parseBig(ctx.Get("share_allow:" + norm(owner) + ":" + norm(spender)))
}

func (v *StrategyVault) moveShares(ctx *bc.Context, from, to string, amount *big.Int) {
	from, to = norm(from), norm(to)
	if from == "" || to == "" || amount == nil || amount.Sign() <= 0 || v.shareOf(ctx, from).Cmp(amount) < 0 {
		ctx.Revert("invalid vault share transfer")
	}
	fromBefore := v.shareOf(ctx, from)
	basis := parseBig(ctx.Get("basis_nav:" + from))
	basisMoved := new(big.Int).Div(new(big.Int).Mul(basis, amount), fromBefore)
	v.setShare(ctx, from, new(big.Int).Sub(fromBefore, amount))
	v.setShare(ctx, to, new(big.Int).Add(v.shareOf(ctx, to), amount))
	ctx.Set("basis_nav:"+from, new(big.Int).Sub(basis, basisMoved).String())
	ctx.Set("basis_nav:"+to, new(big.Int).Add(parseBig(ctx.Get("basis_nav:"+to)), basisMoved).String())
}

func (v *StrategyVault) accrueFees(ctx *bc.Context) {
	total := v.totalShares(ctx)
	last := parseBig(ctx.Get("fee_last_accrual")).Int64()
	if last == 0 {
		ctx.Set("fee_last_accrual", big.NewInt(ctx.BlockTime).String())
		return
	}
	if total.Sign() <= 0 || ctx.BlockTime <= last {
		return
	}
	recipient := norm(ctx.Get("fee_recipient"))
	if recipient == "" {
		recipient = norm(ctx.Get("manager"))
	}
	feeShares := big.NewInt(0)
	mgmtBPS := v.configInt(ctx, "management_fee_bps", 0)
	if mgmtBPS > 0 {
		annual := int64(365 * 24 * 3600)
		feeShares.Add(feeShares, new(big.Int).Div(new(big.Int).Mul(new(big.Int).Mul(total, big.NewInt(mgmtBPS)), big.NewInt(ctx.BlockTime-last)), big.NewInt(10000*annual)))
	}
	nav := v.totalNAV18(ctx)
	if nav.Sign() > 0 && total.Sign() > 0 {
		scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
		pps := new(big.Int).Div(new(big.Int).Mul(nav, scale), total)
		hwm := parseBig(ctx.Get("high_water_mark_x18"))
		if hwm.Sign() == 0 {
			hwm.Set(pps)
		}
		perfBPS := v.configInt(ctx, "performance_fee_bps", 0)
		if perfBPS > 0 && pps.Cmp(hwm) > 0 {
			gainAssets := new(big.Int).Div(new(big.Int).Mul(new(big.Int).Sub(pps, hwm), total), scale)
			feeAssets := new(big.Int).Div(new(big.Int).Mul(gainAssets, big.NewInt(perfBPS)), big.NewInt(10000))
			if feeAssets.Sign() > 0 && nav.Cmp(feeAssets) > 0 {
				feeShares.Add(feeShares, new(big.Int).Div(new(big.Int).Mul(feeAssets, total), new(big.Int).Sub(nav, feeAssets)))
			}
		}
		ctx.Set("high_water_mark_x18", pps.String())
	}
	if feeShares.Sign() > 0 {
		v.setShare(ctx, recipient, new(big.Int).Add(v.shareOf(ctx, recipient), feeShares))
		ctx.Set("totalShares", new(big.Int).Add(total, feeShares).String())
		ctx.Set("fee_shares_minted", new(big.Int).Add(parseBig(ctx.Get("fee_shares_minted")), feeShares).String())
		ctx.Emit("VaultFeesAccrued", map[string]interface{}{"recipient": recipient, "shares": feeShares.String()})
	}
	ctx.Set("fee_last_accrual", big.NewInt(ctx.BlockTime).String())
}

func (v *StrategyVault) setPosition(ctx *bc.Context, pair string, amt *big.Int) {
	ctx.Set("position:"+norm(pair), amt.String())
}

func (v *StrategyVault) trackPair(ctx *bc.Context, pair string) {
	pair = norm(pair)
	if pair == "" || ctx.Get("pair_seen:"+pair) == "1" {
		return
	}
	count := parseBig(ctx.Get("position_pair_count"))
	ctx.Set("position_pair_at:"+count.String(), pair)
	ctx.Set("position_pair_count", new(big.Int).Add(count, big.NewInt(1)).String())
	ctx.Set("pair_seen:"+pair, "1")
}

func (v *StrategyVault) tokenPrice18(ctx *bc.Context, token string) *big.Int {
	token = norm(token)
	// Once a token opts into multi-source valuation, the legacy manager value
	// cannot silently substitute for a failed quorum. The aggregate uses a
	// median anchor, rejects 20% outliers and confidence-weights accepted feeds.
	if count := parseBig(ctx.Get("oracle_source_count:" + token)).Int64(); count > 0 {
		type observation struct {
			price                 *big.Int
			confidence, timestamp int64
		}
		observations := make([]observation, 0, count)
		for i := int64(0); i < count; i++ {
			source := ctx.Get("oracle_source_at:" + token + ":" + big.NewInt(i).String())
			price := parseBig(ctx.Get("oracle_source_price18:" + token + ":" + source))
			confidence := parseBig(ctx.Get("oracle_source_confidence_bps:" + token + ":" + source))
			timestamp := parseBig(ctx.Get("oracle_source_updated_at:" + token + ":" + source))
			if price.Sign() > 0 && confidence.IsInt64() && confidence.Int64() >= 5000 && confidence.Int64() <= 10000 && timestamp.IsInt64() && timestamp.Int64() <= ctx.BlockTime+30 && ctx.BlockTime-timestamp.Int64() <= 900 {
				observations = append(observations, observation{price: price, confidence: confidence.Int64(), timestamp: timestamp.Int64()})
			}
		}
		if len(observations) < 3 {
			return big.NewInt(0)
		}
		sort.Slice(observations, func(i, j int) bool { return observations[i].price.Cmp(observations[j].price) < 0 })
		median := observations[len(observations)/2].price
		weighted, weight := big.NewInt(0), big.NewInt(0)
		oldest := ctx.BlockTime
		for _, observation := range observations {
			deviation := new(big.Int).Abs(new(big.Int).Sub(observation.price, median))
			deviation.Mul(deviation, big.NewInt(10000)).Div(deviation, median)
			if deviation.Cmp(big.NewInt(2000)) > 0 {
				continue
			}
			weighted.Add(weighted, new(big.Int).Mul(observation.price, big.NewInt(observation.confidence)))
			weight.Add(weight, big.NewInt(observation.confidence))
			if observation.timestamp < oldest {
				oldest = observation.timestamp
			}
		}
		if weight.Sign() == 0 || parseBig(ctx.Get("valuation_delay_seconds")).Int64() > ctx.BlockTime-oldest {
			return big.NewInt(0)
		}
		return weighted.Div(weighted, weight)
	}
	updated := parseBig(ctx.Get("oracle_updated_at:" + token))
	if !updated.IsInt64() || ctx.BlockTime-updated.Int64() > 900 || updated.Int64() > ctx.BlockTime+30 || parseBig(ctx.Get("valuation_delay_seconds")).Int64() > ctx.BlockTime-updated.Int64() {
		return big.NewInt(0)
	}
	return parseBig(ctx.Get("oracle_price18:" + token))
}

func (v *StrategyVault) tokenDecimals(ctx *bc.Context, token string) int64 {
	n := parseBig(ctx.Get("token_decimals:" + norm(token)))
	if n.IsInt64() && n.Int64() >= 0 && n.Int64() <= 36 {
		return n.Int64()
	}
	return 18
}

// lpNAV18 returns the USD value (18 decimals) represented by lpAmount.
// Missing oracle prices intentionally return zero; mainnet deposits must never
// mint shares from admin/default prices.
func (v *StrategyVault) lpNAV18(ctx *bc.Context, pair string, lpAmount *big.Int) *big.Int {
	if lpAmount == nil || lpAmount.Sign() <= 0 {
		return big.NewInt(0)
	}
	t0 := v.pairToken(ctx, pair, "Token0")
	t1 := v.pairToken(ctx, pair, "Token1")
	p0 := v.tokenPrice18(ctx, t0)
	p1 := v.tokenPrice18(ctx, t1)
	if p0.Sign() <= 0 || p1.Sign() <= 0 {
		return big.NewInt(0)
	}
	r0, r1, totalLP := v.pairReserves(ctx, pair)
	if totalLP.Sign() <= 0 {
		return big.NewInt(0)
	}
	value0 := new(big.Int).Div(new(big.Int).Mul(r0, p0), new(big.Int).Exp(big.NewInt(10), big.NewInt(v.tokenDecimals(ctx, t0)), nil))
	value1 := new(big.Int).Div(new(big.Int).Mul(r1, p1), new(big.Int).Exp(big.NewInt(10), big.NewInt(v.tokenDecimals(ctx, t1)), nil))
	poolNAV := new(big.Int).Add(value0, value1)
	return new(big.Int).Div(new(big.Int).Mul(poolNAV, lpAmount), totalLP)
}

func (v *StrategyVault) totalNAV18(ctx *bc.Context) *big.Int {
	total := big.NewInt(0)
	count := parseBig(ctx.Get("position_pair_count")).Int64()
	for i := int64(0); i < count; i++ {
		pair := norm(ctx.Get("position_pair_at:" + big.NewInt(i).String()))
		if pair != "" {
			total.Add(total, v.lpNAV18(ctx, pair, v.positionOf(ctx, pair)))
		}
	}
	return total
}

// The HODL benchmark records the underlying token basket contributed at each
// deposit. Comparing its current oracle value with vault NAV isolates the
// strategy/AMM result (including impermanent loss and earned fees) from simple
// token-price movement.
func (v *StrategyVault) recordHODLDeposit(ctx *bc.Context, pair string, lpAmount *big.Int) {
	r0, r1, totalLP := v.pairReserves(ctx, pair)
	if totalLP.Sign() <= 0 {
		return
	}
	tokens := []string{v.pairToken(ctx, pair, "Token0"), v.pairToken(ctx, pair, "Token1")}
	amounts := []*big.Int{
		new(big.Int).Div(new(big.Int).Mul(r0, lpAmount), totalLP),
		new(big.Int).Div(new(big.Int).Mul(r1, lpAmount), totalLP),
	}
	for i, token := range tokens {
		key := "hodl_amount:" + token
		ctx.Set(key, new(big.Int).Add(parseBig(ctx.Get(key)), amounts[i]).String())
		if ctx.Get("hodl_seen:"+token) != "1" {
			count := parseBig(ctx.Get("hodl_token_count"))
			ctx.Set("hodl_token_at:"+count.String(), token)
			ctx.Set("hodl_token_count", new(big.Int).Add(count, big.NewInt(1)).String())
			ctx.Set("hodl_seen:"+token, "1")
		}
	}
}

func (v *StrategyVault) reduceHODLBenchmark(ctx *bc.Context, shares, totalSharesBefore *big.Int) {
	if shares.Sign() <= 0 || totalSharesBefore.Sign() <= 0 {
		return
	}
	count := parseBig(ctx.Get("hodl_token_count")).Int64()
	for i := int64(0); i < count; i++ {
		token := ctx.Get("hodl_token_at:" + big.NewInt(i).String())
		key := "hodl_amount:" + token
		amount := parseBig(ctx.Get(key))
		removed := new(big.Int).Div(new(big.Int).Mul(amount, shares), totalSharesBefore)
		ctx.Set(key, new(big.Int).Sub(amount, removed).String())
	}
}

func (v *StrategyVault) hodlNAV18(ctx *bc.Context) *big.Int {
	value := big.NewInt(0)
	count := parseBig(ctx.Get("hodl_token_count")).Int64()
	for i := int64(0); i < count; i++ {
		token := ctx.Get("hodl_token_at:" + big.NewInt(i).String())
		price := v.tokenPrice18(ctx, token)
		amount := parseBig(ctx.Get("hodl_amount:" + token))
		if price.Sign() > 0 && amount.Sign() > 0 {
			unit := new(big.Int).Exp(big.NewInt(10), big.NewInt(v.tokenDecimals(ctx, token)), nil)
			value.Add(value, new(big.Int).Div(new(big.Int).Mul(amount, price), unit))
		}
	}
	return value
}

func (v *StrategyVault) recordRedemptionAccounting(ctx *bc.Context, user string, shares, userSharesBefore, withdrawNAV *big.Int) {
	basis := parseBig(ctx.Get("basis_nav:" + user))
	basisUsed := big.NewInt(0)
	if userSharesBefore.Sign() > 0 {
		basisUsed.Div(new(big.Int).Mul(basis, shares), userSharesBefore)
	}
	ctx.Set("basis_nav:"+user, new(big.Int).Sub(basis, basisUsed).String())
	ctx.Set("realized_pnl_nav", new(big.Int).Add(parseBig(ctx.Get("realized_pnl_nav")), new(big.Int).Sub(withdrawNAV, basisUsed)).String())
	ctx.Set("capital_out_nav", new(big.Int).Add(parseBig(ctx.Get("capital_out_nav")), withdrawNAV).String())
}

func (v *StrategyVault) pairToken(ctx *bc.Context, pair string, fn string) string {
	res, err := ctx.Call(norm(pair), fn, []string{})
	if err != nil || !res.Success {
		ctx.Revert("pair " + fn + " failed")
	}
	return norm(res.Output)
}

func (v *StrategyVault) pairReserves(ctx *bc.Context, pair string) (*big.Int, *big.Int, *big.Int) {
	res, err := ctx.Call(norm(pair), "GetReserves", []string{})
	if err != nil || !res.Success {
		ctx.Revert("pair reserves failed")
	}
	parts := strings.Split(res.Output, ",")
	if len(parts) < 3 {
		ctx.Revert("invalid pair reserves")
	}
	return parseBig(parts[0]), parseBig(parts[1]), parseBig(parts[2])
}

func (v *StrategyVault) routingWeight(ctx *bc.Context, pair string) *big.Int {
	res, err := ctx.Call(norm(pair), "GetRoutingWeight", []string{})
	if err != nil || !res.Success || strings.TrimSpace(res.Output) == "" {
		return big.NewInt(50)
	}
	weight := parseBig(res.Output)
	if weight.Sign() <= 0 {
		return big.NewInt(50)
	}
	return weight
}

func (v *StrategyVault) configInt(ctx *bc.Context, key string, fallback int64) int64 {
	raw := strings.TrimSpace(ctx.Get(key))
	if raw == "" {
		return fallback
	}
	n := parseBig(raw)
	if !n.IsInt64() || n.Sign() < 0 {
		return fallback
	}
	return n.Int64()
}

func (v *StrategyVault) oracleDemand(ctx *bc.Context, pair string) *big.Int {
	score := parseBig(ctx.Get("oracle_demand:" + norm(pair)))
	if score.Sign() < 0 {
		return big.NewInt(0)
	}
	if score.Cmp(big.NewInt(10000)) > 0 {
		return big.NewInt(10000)
	}
	return score
}

func (v *StrategyVault) requireDeadline(ctx *bc.Context, deadline string) {
	if strings.TrimSpace(deadline) == "" {
		ctx.Revert("deadline required")
	}
	dl := parseBig(deadline)
	if dl.Sign() <= 0 || !dl.IsInt64() {
		ctx.Revert("invalid deadline")
	}
	if ctx.BlockTime > dl.Int64() {
		ctx.Revert("rebalance deadline expired")
	}
}

func (v *StrategyVault) requireSafety(ctx *bc.Context, fromPair string, toPair string, amt *big.Int, minSwapOut *big.Int, strictMinOut bool) {
	position := v.positionOf(ctx, fromPair)
	if position.Cmp(amt) < 0 {
		ctx.Revert("insufficient vault source position")
	}
	maxMoveBps := v.configInt(ctx, "max_move_bps", 2500)
	if maxMoveBps <= 0 || maxMoveBps > 10000 {
		ctx.Revert("invalid max_move_bps")
	}
	if maxMoveBps < 10000 {
		maxMove := new(big.Int).Div(new(big.Int).Mul(position, big.NewInt(maxMoveBps)), big.NewInt(10000))
		if maxMove.Sign() == 0 {
			maxMove = big.NewInt(1)
		}
		if amt.Cmp(maxMove) > 0 {
			ctx.Revert("rebalance amount exceeds max_move_bps")
		}
	}
	bufferBps := v.configInt(ctx, "liquid_buffer_bps", 1500)
	minimumRemaining := new(big.Int).Div(new(big.Int).Mul(position, big.NewInt(bufferBps)), big.NewInt(10000))
	if new(big.Int).Sub(position, amt).Cmp(minimumRemaining) < 0 {
		ctx.Revert("rebalance would breach withdrawal liquidity buffer")
	}
	interval := v.configInt(ctx, "min_rebalance_interval", 300)
	last := parseBig(ctx.Get("last_rebalance"))
	if interval > 0 && last.IsInt64() && last.Int64() > 0 && ctx.BlockTime < last.Int64()+interval {
		ctx.Revert("rebalance cooldown active")
	}
	if !v.sameTokenSet(ctx, fromPair, toPair) && strictMinOut && (minSwapOut == nil || minSwapOut.Sign() <= 0) {
		ctx.Revert("cross-asset rebalance requires minSwapOut")
	}
}

func (v *StrategyVault) sameTokenSet(ctx *bc.Context, fromPair string, toPair string) bool {
	a0 := v.pairToken(ctx, fromPair, "Token0")
	a1 := v.pairToken(ctx, fromPair, "Token1")
	b0 := v.pairToken(ctx, toPair, "Token0")
	b1 := v.pairToken(ctx, toPair, "Token1")
	return (a0 == b0 && a1 == b1) || (a0 == b1 && a1 == b0)
}

func (v *StrategyVault) approveIfNeeded(ctx *bc.Context, token string, spender string, amt *big.Int) {
	if isNative(token) || amt.Sign() <= 0 {
		return
	}
	if _, err := ctx.Call(norm(token), "Approve", []string{norm(spender), amt.String()}); err != nil {
		ctx.Revert("token approve failed: " + err.Error())
	}
}

func (v *StrategyVault) parseAmounts(ctx *bc.Context, out string) (*big.Int, *big.Int) {
	parts := strings.Split(out, ",")
	if len(parts) < 2 {
		ctx.Revert("invalid liquidity output")
	}
	return parseBig(parts[0]), parseBig(parts[1])
}

func (v *StrategyVault) sendVaultToken(ctx *bc.Context, token, to string, amount *big.Int) {
	if amount == nil || amount.Sign() <= 0 {
		return
	}
	if isNative(token) {
		ctx.SendNative(norm(to), amount)
		return
	}
	if _, err := ctx.Call(norm(token), "Transfer", []string{norm(to), amount.String()}); err != nil {
		ctx.Revert("vault token transfer failed: " + err.Error())
	}
}

func (v *StrategyVault) redeemLPAmount(ctx *bc.Context, pair string, shares *big.Int) *big.Int {
	total := v.totalShares(ctx)
	position := v.positionOf(ctx, pair)
	if total.Sign() <= 0 || position.Sign() <= 0 {
		ctx.Revert("empty vault position")
	}
	lpAmt := new(big.Int).Div(new(big.Int).Mul(shares, position), total)
	if lpAmt.Sign() <= 0 {
		ctx.Revert("shares too small for selected position")
	}
	if position.Cmp(lpAmt) < 0 {
		ctx.Revert("insufficient vault position in pair")
	}
	return lpAmt
}

func (v *StrategyVault) addIdle(ctx *bc.Context, token string, amt *big.Int) {
	token = norm(token)
	if token == "" || amt == nil || amt.Sign() <= 0 {
		return
	}
	key := "idle:" + token
	ctx.Set(key, new(big.Int).Add(parseBig(ctx.Get(key)), amt).String())
}

func (v *StrategyVault) pairOtherToken(ctx *bc.Context, pair string, tokenIn string) string {
	tokenIn = norm(tokenIn)
	t0 := v.pairToken(ctx, pair, "Token0")
	t1 := v.pairToken(ctx, pair, "Token1")
	switch tokenIn {
	case t0:
		return t1
	case t1:
		return t0
	default:
		ctx.Revert("route pair does not contain token")
	}
	return ""
}

func (v *StrategyVault) routePairs(ctx *bc.Context, tokenIn string, tokenOut string, amountIn *big.Int) []string {
	router := norm(ctx.Get("router"))
	if router == "" {
		ctx.Revert("router required for cross-asset movement")
	}
	res, err := ctx.Call(router, "GetBestRouteForAmount", []string{amountIn.String(), norm(tokenIn), norm(tokenOut)})
	if err != nil || !res.Success {
		// Backward-compatible fallback for an older router deployment.
		res, err = ctx.Call(router, "GetBestRoute", []string{norm(tokenIn), norm(tokenOut)})
	}
	if err != nil || !res.Success {
		ctx.Revert("route lookup failed")
	}
	raw := strings.TrimSpace(res.Output)
	if raw == "" {
		ctx.Revert("no swap route")
	}
	out := []string{}
	for _, part := range strings.Split(raw, ",") {
		pair := norm(part)
		if pair != "" {
			out = append(out, pair)
		}
	}
	if len(out) == 0 || len(out) > 3 {
		ctx.Revert("unsupported swap route")
	}
	return out
}

func (v *StrategyVault) swapVaultToken(ctx *bc.Context, tokenIn string, tokenOut string, amountIn *big.Int, minAmountOut *big.Int) *big.Int {
	tokenIn = norm(tokenIn)
	tokenOut = norm(tokenOut)
	if amountIn == nil || amountIn.Sign() <= 0 {
		return big.NewInt(0)
	}
	if tokenIn == tokenOut {
		return new(big.Int).Set(amountIn)
	}
	if minAmountOut == nil {
		minAmountOut = big.NewInt(0)
	}

	currentToken := tokenIn
	currentAmount := new(big.Int).Set(amountIn)
	route := v.routePairs(ctx, tokenIn, tokenOut, amountIn)
	for i, pair := range route {
		nextToken := v.pairOtherToken(ctx, pair, currentToken)
		minOut := big.NewInt(0)
		if i == len(route)-1 {
			minOut = minAmountOut
		}
		v.approveIfNeeded(ctx, currentToken, pair, currentAmount)
		swapped, err := ctx.Call(pair, "SwapFromContract", []string{ctx.ContractAddr, currentAmount.String(), minOut.String(), currentToken})
		if err != nil || !swapped.Success {
			ctx.Revert("vault swap failed")
		}
		currentAmount = parseBig(swapped.Output)
		requirePositive(ctx, "swap output", currentAmount)
		currentToken = nextToken
	}
	if currentToken != tokenOut {
		ctx.Revert("swap route ended with wrong token")
	}
	return currentAmount
}

// Init configures the strategy vault. router is optional metadata for UIs;
// manager controls rebalances. If manager is empty, the deploy owner is used.
func (v *StrategyVault) Init(ctx *bc.Context, router string, manager string) {
	if ctx.Get("initialized") == "true" {
		ctx.Revert("already initialized")
	}
	manager = norm(manager)
	if manager == "" {
		manager = norm(ctx.OwnerAddr)
	}
	ctx.Set("initialized", "true")
	ctx.Set("router", norm(router))
	ctx.Set("manager", manager)
	ctx.Set("totalShares", "0")
	ctx.Set("max_move_bps", "2500")
	ctx.Set("min_rebalance_interval", "300")
	ctx.Set("max_slippage_bps", "500")
	ctx.Set("withdrawal_delay", "60")
	ctx.Set("liquid_buffer_bps", "1500")
	ctx.Set("withdrawal_head", "1")
	ctx.Set("withdrawal_tail", "0")
	ctx.Set("management_fee_bps", "0")
	ctx.Set("performance_fee_bps", "0")
	ctx.Set("fee_recipient", manager)
	ctx.Set("fee_last_accrual", big.NewInt(ctx.BlockTime).String())
	ctx.Set("emergency_mode", "false")
	ctx.Set("emergency_max_loss_bps", "3000")
	ctx.Set("valuation_delay_seconds", "0")
	ctx.Set("keeper_min_bond", "100000")
	ctx.Set("keeper_fallback_delay", "120")
	ctx.Set("keeper_unbond_delay", "86400")
	ctx.Set("keeper_miss_slash_bps", "1000")
	ctx.Set("keeper_insurance_recipient", "0x0000000000000000000000000000000000000e02")
	ctx.Set("rebalance_job_seq", "0")
	ctx.Set("erc4626_min_initial_assets", "1000")
	ctx.Commit()
	ctx.Emit("VaultInitialized", map[string]interface{}{
		"router":     norm(router),
		"manager":    manager,
		"maxMoveBps": "2500",
	})
}

// DepositLP transfers LP tokens into the vault and mints 1:1 vault shares.
// User must approve this vault on the pair contract first.
func (v *StrategyVault) depositLPFor(ctx *bc.Context, pair string, amount *big.Int, user, receiver string) *big.Int {
	pair = norm(pair)
	user, receiver = norm(user), norm(receiver)
	amt := new(big.Int).Set(amount)
	requirePositive(ctx, "LP amount", amt)
	if pair == "" || user == "" || receiver == "" {
		ctx.Revert("invalid deposit")
	}
	v.accrueFees(ctx)

	priorNAV := v.totalNAV18(ctx)
	depositNAV := v.lpNAV18(ctx, pair, amt)
	if depositNAV.Sign() <= 0 {
		ctx.Revert("verified token oracle prices required before deposit")
	}
	priorShares := v.totalShares(ctx)
	mintedShares := new(big.Int).Set(depositNAV)
	if ctx.Get("erc4626_mode") == "true" && pair == norm(ctx.Get("erc4626_asset")) {
		priorAssets := v.erc4626TotalAssets(ctx, pair)
		if priorShares.Sign() == 0 {
			if amt.Cmp(parseBig(ctx.Get("erc4626_min_initial_assets"))) < 0 {
				ctx.Revert("initial ERC-4626 deposit below anti-inflation minimum")
			}
		}
		mintedShares.Div(new(big.Int).Mul(amt, new(big.Int).Add(priorShares, big.NewInt(1))), new(big.Int).Add(priorAssets, big.NewInt(1)))
	} else if priorShares.Sign() > 0 {
		if priorNAV.Sign() <= 0 {
			ctx.Revert("vault NAV unavailable")
		}
		mintedShares.Div(new(big.Int).Mul(depositNAV, priorShares), priorNAV)
	}
	requirePositive(ctx, "minted shares", mintedShares)

	if _, err := ctx.Call(pair, "TransferFrom", []string{user, ctx.ContractAddr, amt.String()}); err != nil {
		ctx.Revert("LP transfer failed: " + err.Error())
	}
	v.recordHODLDeposit(ctx, pair, amt)

	v.setShare(ctx, receiver, new(big.Int).Add(v.shareOf(ctx, receiver), mintedShares))
	ctx.Set("totalShares", new(big.Int).Add(priorShares, mintedShares).String())
	if ctx.Get("erc4626_mode") == "true" && pair == norm(ctx.Get("erc4626_asset")) {
		v.setPosition(ctx, pair, v.erc4626TotalAssets(ctx, pair))
	} else {
		v.setPosition(ctx, pair, new(big.Int).Add(v.positionOf(ctx, pair), amt))
	}
	v.trackPair(ctx, pair)
	ctx.Set("basis_nav:"+receiver, new(big.Int).Add(parseBig(ctx.Get("basis_nav:"+receiver)), depositNAV).String())
	ctx.Set("capital_in_nav", new(big.Int).Add(parseBig(ctx.Get("capital_in_nav")), depositNAV).String())
	if ctx.Get("current_pair") == "" {
		ctx.Set("current_pair", pair)
	}
	ctx.Set("output", mintedShares.String())
	ctx.Commit()
	ctx.Emit("VaultDeposit", map[string]interface{}{
		"user": user, "receiver": receiver, "pair": pair, "lpAmount": amt.String(), "depositNAV18": depositNAV.String(), "shares": mintedShares.String(),
	})
	return mintedShares
}

func (v *StrategyVault) DepositLP(ctx *bc.Context, pair string, amount string) {
	user := actorAddr(ctx)
	v.depositLPFor(ctx, pair, parseBig(amount), user, user)
}

// WithdrawLP returns the user's pro-rata LP tokens from a selected pool.
func (v *StrategyVault) WithdrawLP(ctx *bc.Context, pair string, shares string) {
	pair = norm(pair)
	user := actorAddr(ctx)
	amt := parseBig(shares)
	v.accrueFees(ctx)
	requirePositive(ctx, "shares", amt)
	if v.shareOf(ctx, user).Cmp(amt) < 0 {
		ctx.Revert("insufficient vault shares")
	}
	lpAmt := v.redeemLPAmount(ctx, pair, amt)
	withdrawNAV := v.lpNAV18(ctx, pair, lpAmt)
	userShares := v.shareOf(ctx, user)
	basisUsed := new(big.Int).Div(new(big.Int).Mul(parseBig(ctx.Get("basis_nav:"+user)), amt), userShares)

	if _, err := ctx.Call(pair, "Transfer", []string{user, lpAmt.String()}); err != nil {
		ctx.Revert("LP withdraw failed: " + err.Error())
	}

	v.reduceHODLBenchmark(ctx, amt, v.totalShares(ctx))
	v.setShare(ctx, user, new(big.Int).Sub(v.shareOf(ctx, user), amt))
	ctx.Set("totalShares", new(big.Int).Sub(v.totalShares(ctx), amt).String())
	v.setPosition(ctx, pair, new(big.Int).Sub(v.positionOf(ctx, pair), lpAmt))
	ctx.Set("basis_nav:"+user, new(big.Int).Sub(parseBig(ctx.Get("basis_nav:"+user)), basisUsed).String())
	ctx.Set("realized_pnl_nav", new(big.Int).Add(parseBig(ctx.Get("realized_pnl_nav")), new(big.Int).Sub(withdrawNAV, basisUsed)).String())
	ctx.Set("capital_out_nav", new(big.Int).Add(parseBig(ctx.Get("capital_out_nav")), withdrawNAV).String())
	ctx.Set("output", lpAmt.String())
	ctx.Commit()
	ctx.Emit("VaultWithdrawLP", map[string]interface{}{
		"user": user, "pair": pair, "shares": amt.String(), "lpAmount": lpAmt.String(),
	})
}

// WithdrawToTokens removes liquidity from the selected pair and sends the
// underlying assets directly back to the user.
func (v *StrategyVault) WithdrawToTokens(ctx *bc.Context, pair string, shares string) {
	pair = norm(pair)
	user := actorAddr(ctx)
	amt := parseBig(shares)
	v.accrueFees(ctx)
	requirePositive(ctx, "shares", amt)
	if v.shareOf(ctx, user).Cmp(amt) < 0 {
		ctx.Revert("insufficient vault shares")
	}
	lpAmt := v.redeemLPAmount(ctx, pair, amt)
	withdrawNAV := v.lpNAV18(ctx, pair, lpAmt)
	userShares := v.shareOf(ctx, user)

	res, err := ctx.Call(pair, "RemoveLiquidityTo", []string{user, lpAmt.String()})
	if err != nil || !res.Success {
		ctx.Revert("remove liquidity failed")
	}

	v.reduceHODLBenchmark(ctx, amt, v.totalShares(ctx))
	v.recordRedemptionAccounting(ctx, user, amt, userShares, withdrawNAV)
	v.setShare(ctx, user, new(big.Int).Sub(v.shareOf(ctx, user), amt))
	ctx.Set("totalShares", new(big.Int).Sub(v.totalShares(ctx), amt).String())
	v.setPosition(ctx, pair, new(big.Int).Sub(v.positionOf(ctx, pair), lpAmt))
	ctx.Set("output", res.Output)
	ctx.Commit()
	ctx.Emit("VaultWithdrawTokens", map[string]interface{}{
		"user": user, "pair": pair, "shares": amt.String(), "lpAmount": lpAmt.String(), "amounts": res.Output,
	})
}

func (v *StrategyVault) rebalance(ctx *bc.Context, fromPair string, toPair string, lpAmount string) {
	if ctx.Get("erc4626_mode") == "true" {
		ctx.Revert("ERC-4626 single-asset mode does not permit cross-pool rebalancing")
	}
	fromPair = norm(fromPair)
	toPair = norm(toPair)
	amt := parseBig(lpAmount)
	requirePositive(ctx, "LP amount", amt)
	if fromPair == "" || toPair == "" || fromPair == toPair {
		ctx.Revert("invalid rebalance pair")
	}
	v.requireSafety(ctx, fromPair, toPair, amt, big.NewInt(0), false)
	if v.sameTokenSet(ctx, fromPair, toPair) {
		v.rebalanceSameAsset(ctx, fromPair, toPair, amt)
		return
	}
	v.rebalanceCrossAsset(ctx, fromPair, toPair, amt, big.NewInt(0))
}

func (v *StrategyVault) rebalanceSameAsset(ctx *bc.Context, fromPair string, toPair string, amt *big.Int) {
	from0 := v.pairToken(ctx, fromPair, "Token0")
	from1 := v.pairToken(ctx, fromPair, "Token1")
	to0 := v.pairToken(ctx, toPair, "Token0")
	to1 := v.pairToken(ctx, toPair, "Token1")

	removed, err := ctx.Call(fromPair, "RemoveLiquidityTo", []string{ctx.ContractAddr, amt.String()})
	if err != nil || !removed.Success {
		ctx.Revert("remove source liquidity failed")
	}
	out0, out1 := v.parseAmounts(ctx, removed.Output)

	target0 := out0
	target1 := out1
	if to0 == from1 && to1 == from0 {
		target0 = out1
		target1 = out0
	}

	v.approveIfNeeded(ctx, to0, toPair, target0)
	v.approveIfNeeded(ctx, to1, toPair, target1)

	added, err := ctx.Call(toPair, "AddLiquidityFromContract", []string{ctx.ContractAddr, target0.String(), target1.String()})
	if err != nil || !added.Success {
		ctx.Revert("add target liquidity failed")
	}
	minted := parseBig(added.Output)
	requirePositive(ctx, "minted LP", minted)

	v.setPosition(ctx, fromPair, new(big.Int).Sub(v.positionOf(ctx, fromPair), amt))
	v.setPosition(ctx, toPair, new(big.Int).Add(v.positionOf(ctx, toPair), minted))
	v.trackPair(ctx, toPair)
	ctx.Set("current_pair", toPair)
	ctx.Set("last_rebalance", big.NewInt(ctx.BlockTime).String())
	ctx.Set("output", minted.String())
	ctx.Commit()
	ctx.Emit("VaultRebalanced", map[string]interface{}{
		"type": "same_asset", "fromPair": fromPair, "toPair": toPair, "burnedLP": amt.String(), "mintedLP": minted.String(),
	})
}

func (v *StrategyVault) rebalanceCrossAsset(ctx *bc.Context, fromPair string, toPair string, amt *big.Int, minSwapOut *big.Int) {
	from0 := v.pairToken(ctx, fromPair, "Token0")
	from1 := v.pairToken(ctx, fromPair, "Token1")
	to0 := v.pairToken(ctx, toPair, "Token0")
	to1 := v.pairToken(ctx, toPair, "Token1")

	removed, err := ctx.Call(fromPair, "RemoveLiquidityTo", []string{ctx.ContractAddr, amt.String()})
	if err != nil || !removed.Success {
		ctx.Revert("remove source liquidity failed")
	}
	out0, out1 := v.parseAmounts(ctx, removed.Output)

	balances := map[string]*big.Int{}
	addBalance := func(token string, amount *big.Int) {
		token = norm(token)
		if amount == nil || amount.Sign() <= 0 {
			return
		}
		if balances[token] == nil {
			balances[token] = big.NewInt(0)
		}
		balances[token] = new(big.Int).Add(balances[token], amount)
	}
	addBalance(from0, out0)
	addBalance(from1, out1)

	target0 := big.NewInt(0)
	target1 := big.NewInt(0)
	if balances[to0] != nil {
		target0 = balances[to0]
		delete(balances, to0)
	}
	if balances[to1] != nil {
		target1 = balances[to1]
		delete(balances, to1)
	}

	for token, amount := range balances {
		if amount.Sign() <= 0 {
			continue
		}
		if target0.Sign() == 0 {
			target0 = new(big.Int).Add(target0, v.swapVaultToken(ctx, token, to0, amount, minSwapOut))
			continue
		}
		if target1.Sign() == 0 {
			target1 = new(big.Int).Add(target1, v.swapVaultToken(ctx, token, to1, amount, minSwapOut))
			continue
		}
		v.addIdle(ctx, token, amount)
	}

	if target0.Sign() == 0 && target1.Cmp(big.NewInt(1)) > 0 {
		half := new(big.Int).Div(new(big.Int).Set(target1), big.NewInt(2))
		target1.Sub(target1, half)
		target0.Add(target0, v.swapVaultToken(ctx, to1, to0, half, minSwapOut))
	}
	if target1.Sign() == 0 && target0.Cmp(big.NewInt(1)) > 0 {
		half := new(big.Int).Div(new(big.Int).Set(target0), big.NewInt(2))
		target0.Sub(target0, half)
		target1.Add(target1, v.swapVaultToken(ctx, to0, to1, half, minSwapOut))
	}
	requirePositive(ctx, "target token0 amount", target0)
	requirePositive(ctx, "target token1 amount", target1)

	v.approveIfNeeded(ctx, to0, toPair, target0)
	v.approveIfNeeded(ctx, to1, toPair, target1)
	added, err := ctx.Call(toPair, "AddLiquidityFromContract", []string{ctx.ContractAddr, target0.String(), target1.String()})
	if err != nil || !added.Success {
		ctx.Revert("add target liquidity failed")
	}
	minted := parseBig(added.Output)
	requirePositive(ctx, "minted LP", minted)

	v.setPosition(ctx, fromPair, new(big.Int).Sub(v.positionOf(ctx, fromPair), amt))
	v.setPosition(ctx, toPair, new(big.Int).Add(v.positionOf(ctx, toPair), minted))
	v.trackPair(ctx, toPair)
	ctx.Set("current_pair", toPair)
	ctx.Set("last_rebalance", big.NewInt(ctx.BlockTime).String())
	ctx.Set("output", minted.String())
	ctx.Commit()
	ctx.Emit("VaultRebalanced", map[string]interface{}{
		"type":          "cross_asset",
		"fromPair":      fromPair,
		"toPair":        toPair,
		"burnedLP":      amt.String(),
		"mintedLP":      minted.String(),
		"sourceToken0":  from0,
		"sourceToken1":  from1,
		"targetToken0":  to0,
		"targetToken1":  to1,
		"targetAmount0": target0.String(),
		"targetAmount1": target1.String(),
	})
}

// Rebalance moves vault-owned liquidity from one pool to another. If assets
// differ, it uses the configured DEX router's direct or two-hop swap route.
func (v *StrategyVault) Rebalance(ctx *bc.Context, fromPair string, toPair string, lpAmount string) {
	v.requireManager(ctx)
	v.rebalance(ctx, fromPair, toPair, lpAmount)
}

// RebalanceWithMinOut is the slippage-aware variant for cross-asset movement.
func (v *StrategyVault) RebalanceWithMinOut(ctx *bc.Context, fromPair string, toPair string, lpAmount string, minSwapOut string) {
	v.requireManager(ctx)
	fromPair = norm(fromPair)
	toPair = norm(toPair)
	amt := parseBig(lpAmount)
	minOut := parseBig(minSwapOut)
	requirePositive(ctx, "LP amount", amt)
	if fromPair == "" || toPair == "" || fromPair == toPair {
		ctx.Revert("invalid rebalance pair")
	}
	v.requireSafety(ctx, fromPair, toPair, amt, minOut, true)
	if v.sameTokenSet(ctx, fromPair, toPair) {
		v.rebalanceSameAsset(ctx, fromPair, toPair, amt)
		return
	}
	v.rebalanceCrossAsset(ctx, fromPair, toPair, amt, minOut)
}

func (v *StrategyVault) RebalanceWithLimits(ctx *bc.Context, fromPair string, toPair string, lpAmount string, minSwapOut string, maxSlippageBps string, deadline string) {
	v.requireManager(ctx)
	v.requireDeadline(ctx, deadline)
	slippage := parseBig(maxSlippageBps)
	maxAllowed := big.NewInt(v.configInt(ctx, "max_slippage_bps", 500))
	if slippage.Sign() <= 0 || slippage.Cmp(maxAllowed) > 0 {
		ctx.Revert("slippage limit exceeds vault safety")
	}
	fromPair = norm(fromPair)
	toPair = norm(toPair)
	amt := parseBig(lpAmount)
	minOut := parseBig(minSwapOut)
	requirePositive(ctx, "LP amount", amt)
	if fromPair == "" || toPair == "" || fromPair == toPair {
		ctx.Revert("invalid rebalance pair")
	}
	v.requireSafety(ctx, fromPair, toPair, amt, minOut, true)
	if v.sameTokenSet(ctx, fromPair, toPair) {
		v.rebalanceSameAsset(ctx, fromPair, toPair, amt)
		return
	}
	v.rebalanceCrossAsset(ctx, fromPair, toPair, amt, minOut)
}

func (v *StrategyVault) loadRebalanceJob(ctx *bc.Context, id string) rebalanceJob {
	var job rebalanceJob
	_ = json.Unmarshal([]byte(ctx.Get("rebalance_job:"+strings.TrimSpace(id))), &job)
	return job
}

func (v *StrategyVault) saveRebalanceJob(ctx *bc.Context, job rebalanceJob) {
	raw, _ := json.Marshal(job)
	ctx.Set("rebalance_job:"+job.ID, string(raw))
}

// RegisterKeeper bonds native LQD. Jobs are immutable once scheduled and can
// become permissionless after the assigned keeper's exclusive window.
func (v *StrategyVault) RegisterKeeper(ctx *bc.Context) {
	keeper, amount := actorAddr(ctx), ctx.MsgValue()
	if keeper == "" || amount.Sign() <= 0 {
		ctx.Revert("native keeper bond required")
	}
	ctx.ReceiveNative(amount)
	bond := new(big.Int).Add(parseBig(ctx.Get("keeper_bond:"+keeper)), amount)
	ctx.Set("keeper_bond:"+keeper, bond.String())
	ctx.Set("keeper_active:"+keeper, "true")
	ctx.Set("total_keeper_bond", new(big.Int).Add(parseBig(ctx.Get("total_keeper_bond")), amount).String())
	ctx.Emit("KeeperBonded", map[string]interface{}{"keeper": keeper, "amount": amount.String(), "totalBond": bond.String()})
}

func (v *StrategyVault) RequestKeeperUnbond(ctx *bc.Context) {
	keeper := actorAddr(ctx)
	if parseBig(ctx.Get("keeper_bond:"+keeper)).Sign() <= 0 {
		ctx.Revert("keeper has no bond")
	}
	unlock := ctx.BlockTime + v.configInt(ctx, "keeper_unbond_delay", 86400)
	ctx.Set("keeper_active:"+keeper, "false")
	ctx.Set("keeper_unbond_at:"+keeper, big.NewInt(unlock).String())
	ctx.Emit("KeeperUnbondRequested", map[string]interface{}{"keeper": keeper, "unlockAt": unlock})
}

func (v *StrategyVault) WithdrawKeeperBond(ctx *bc.Context) {
	keeper := actorAddr(ctx)
	unlock, bond := parseBig(ctx.Get("keeper_unbond_at:"+keeper)), parseBig(ctx.Get("keeper_bond:"+keeper))
	if bond.Sign() <= 0 || !unlock.IsInt64() || unlock.Int64() == 0 || ctx.BlockTime < unlock.Int64() {
		ctx.Revert("keeper bond is locked")
	}
	ctx.Set("keeper_bond:"+keeper, "0")
	ctx.Set("keeper_unbond_at:"+keeper, "0")
	ctx.Set("total_keeper_bond", new(big.Int).Sub(parseBig(ctx.Get("total_keeper_bond")), bond).String())
	ctx.SendNative(keeper, bond)
	ctx.Emit("KeeperBondWithdrawn", map[string]interface{}{"keeper": keeper, "amount": bond.String()})
}

func (v *StrategyVault) SetKeeperPolicy(ctx *bc.Context, minBond string, fallbackDelay string, unbondDelay string, missSlashBPS string, insuranceRecipient string) {
	v.requireManager(ctx)
	bond, fallback, unbond, slash := parseBig(minBond), parseBig(fallbackDelay), parseBig(unbondDelay), parseBig(missSlashBPS)
	if bond.Sign() <= 0 || !fallback.IsInt64() || fallback.Int64() < 30 || fallback.Int64() > 3600 || !unbond.IsInt64() || unbond.Int64() < 86400 || !slash.IsInt64() || slash.Int64() < 100 || slash.Int64() > 5000 || norm(insuranceRecipient) == "" {
		ctx.Revert("invalid keeper policy")
	}
	ctx.Set("keeper_min_bond", bond.String())
	ctx.Set("keeper_fallback_delay", fallback.String())
	ctx.Set("keeper_unbond_delay", unbond.String())
	ctx.Set("keeper_miss_slash_bps", slash.String())
	ctx.Set("keeper_insurance_recipient", norm(insuranceRecipient))
	ctx.Emit("KeeperPolicyUpdated", map[string]interface{}{"minBond": bond.String(), "fallbackDelay": fallback.String(), "unbondDelay": unbond.String(), "missSlashBPS": slash.String()})
}

func (v *StrategyVault) ScheduleRebalanceJob(ctx *bc.Context, fromPair string, toPair string, lpAmount string, minSwapOut string, assignedKeeper string, deadline string) {
	v.requireManager(ctx)
	fromPair, toPair, assignedKeeper = norm(fromPair), norm(toPair), norm(assignedKeeper)
	amount, minOut, expires := parseBig(lpAmount), parseBig(minSwapOut), parseBig(deadline)
	if fromPair == "" || toPair == "" || fromPair == toPair || amount.Sign() <= 0 || !expires.IsInt64() || expires.Int64() <= ctx.BlockTime {
		ctx.Revert("invalid rebalance job")
	}
	if assignedKeeper != "" && (ctx.Get("keeper_active:"+assignedKeeper) != "true" || parseBig(ctx.Get("keeper_bond:"+assignedKeeper)).Cmp(parseBig(ctx.Get("keeper_min_bond"))) < 0) {
		ctx.Revert("assigned keeper is not sufficiently bonded")
	}
	seq := new(big.Int).Add(parseBig(ctx.Get("rebalance_job_seq")), big.NewInt(1))
	job := rebalanceJob{ID: seq.String(), FromPair: fromPair, ToPair: toPair, LPAmount: amount.String(), MinSwapOut: minOut.String(), Assigned: assignedKeeper, ScheduledAt: ctx.BlockTime, FallbackAt: ctx.BlockTime + v.configInt(ctx, "keeper_fallback_delay", 120), Deadline: expires.Int64(), Status: "scheduled"}
	ctx.Set("rebalance_job_seq", job.ID)
	v.saveRebalanceJob(ctx, job)
	ctx.Set("output", job.ID)
	ctx.Commit()
	ctx.Emit("RebalanceJobScheduled", map[string]interface{}{"id": job.ID, "assigned": assignedKeeper, "fallbackAt": job.FallbackAt, "deadline": job.Deadline})
}

func (v *StrategyVault) ExecuteRebalanceJob(ctx *bc.Context, id string) {
	job, executor := v.loadRebalanceJob(ctx, id), actorAddr(ctx)
	if job.Status != "scheduled" || ctx.BlockTime > job.Deadline {
		ctx.Revert("rebalance job is not executable")
	}
	if job.Assigned != "" && executor != job.Assigned && ctx.BlockTime < job.FallbackAt {
		ctx.Revert("assigned keeper exclusive window active")
	}
	if executor == job.Assigned && (ctx.Get("keeper_active:"+executor) != "true" || parseBig(ctx.Get("keeper_bond:"+executor)).Cmp(parseBig(ctx.Get("keeper_min_bond"))) < 0) {
		ctx.Revert("assigned keeper bond inactive")
	}
	fromPair, toPair, amount, minOut := job.FromPair, job.ToPair, parseBig(job.LPAmount), parseBig(job.MinSwapOut)
	v.requireSafety(ctx, fromPair, toPair, amount, minOut, !v.sameTokenSet(ctx, fromPair, toPair))
	if v.sameTokenSet(ctx, fromPair, toPair) {
		v.rebalanceSameAsset(ctx, fromPair, toPair, amount)
	} else {
		v.rebalanceCrossAsset(ctx, fromPair, toPair, amount, minOut)
	}
	job.Status, job.Executor, job.ExecutedAt = "executed", executor, ctx.BlockTime
	v.saveRebalanceJob(ctx, job)
	ctx.Emit("RebalanceJobExecuted", map[string]interface{}{"id": job.ID, "executor": executor, "permissionlessFallback": executor != job.Assigned})
}

func (v *StrategyVault) ReportMissedRebalanceJob(ctx *bc.Context, id string) {
	job := v.loadRebalanceJob(ctx, id)
	if job.Status != "scheduled" || ctx.BlockTime <= job.Deadline {
		ctx.Revert("rebalance job is not missed")
	}
	job.Status = "missed"
	if job.Assigned != "" {
		bond := parseBig(ctx.Get("keeper_bond:" + job.Assigned))
		slash := new(big.Int).Div(new(big.Int).Mul(bond, big.NewInt(v.configInt(ctx, "keeper_miss_slash_bps", 1000))), big.NewInt(10000))
		if slash.Sign() == 0 && bond.Sign() > 0 {
			slash.SetInt64(1)
		}
		ctx.Set("keeper_bond:"+job.Assigned, new(big.Int).Sub(bond, slash).String())
		ctx.Set("total_keeper_bond", new(big.Int).Sub(parseBig(ctx.Get("total_keeper_bond")), slash).String())
		if new(big.Int).Sub(bond, slash).Cmp(parseBig(ctx.Get("keeper_min_bond"))) < 0 {
			ctx.Set("keeper_active:"+job.Assigned, "false")
		}
		ctx.SendNative(ctx.Get("keeper_insurance_recipient"), slash)
		ctx.Emit("KeeperSlashed", map[string]interface{}{"keeper": job.Assigned, "job": job.ID, "amount": slash.String()})
	}
	v.saveRebalanceJob(ctx, job)
	ctx.Emit("RebalanceJobMissed", map[string]interface{}{"id": job.ID, "reporter": actorAddr(ctx)})
}

func (v *StrategyVault) RebalanceJob(ctx *bc.Context, id string) {
	ctx.Set("output", ctx.Get("rebalance_job:"+strings.TrimSpace(id)))
}
func (v *StrategyVault) KeeperBond(ctx *bc.Context, keeper string) {
	ctx.Set("output", parseBig(ctx.Get("keeper_bond:"+norm(keeper))).String())
}

func (v *StrategyVault) SetSafetyLimits(ctx *bc.Context, maxMoveBps string, minRebalanceInterval string, maxSlippageBps string) {
	v.requireManager(ctx)
	maxMove := parseBig(maxMoveBps)
	interval := parseBig(minRebalanceInterval)
	slippage := parseBig(maxSlippageBps)
	if !maxMove.IsInt64() || maxMove.Int64() <= 0 || maxMove.Int64() > 10000 {
		ctx.Revert("maxMoveBps must be 1..10000")
	}
	if !interval.IsInt64() || interval.Sign() < 0 {
		ctx.Revert("invalid minRebalanceInterval")
	}
	if !slippage.IsInt64() || slippage.Int64() <= 0 || slippage.Int64() > 10000 {
		ctx.Revert("maxSlippageBps must be 1..10000")
	}
	ctx.Set("max_move_bps", maxMove.String())
	ctx.Set("min_rebalance_interval", interval.String())
	ctx.Set("max_slippage_bps", slippage.String())
	ctx.Commit()
	ctx.Emit("VaultSafetyLimitsUpdated", map[string]interface{}{
		"maxMoveBps":           maxMove.String(),
		"minRebalanceInterval": interval.String(),
		"maxSlippageBps":       slippage.String(),
	})
}

func (v *StrategyVault) SetOracleDemand(ctx *bc.Context, pair string, demandBps string) {
	v.requireManager(ctx)
	pair = norm(pair)
	score := parseBig(demandBps)
	if pair == "" {
		ctx.Revert("pair required")
	}
	if !score.IsInt64() || score.Sign() < 0 || score.Int64() > 10000 {
		ctx.Revert("demandBps must be 0..10000")
	}
	ctx.Set("oracle_demand:"+pair, score.String())
	ctx.Commit()
	ctx.Emit("VaultOracleDemandUpdated", map[string]interface{}{"pair": pair, "demandBps": score.String()})
}

// SetAssetOracle configures the externally medianized price committed by the
// protocol oracle adapter. priceUSD18 uses 18 decimals; timestamp must be no
// older than 15 minutes at the current block time.
func (v *StrategyVault) SetAssetOracle(ctx *bc.Context, token string, priceUSD18 string, decimals string, timestamp string) {
	v.requireManager(ctx)
	token = norm(token)
	price := parseBig(priceUSD18)
	dec := parseBig(decimals)
	ts := parseBig(timestamp)
	if token == "" || price.Sign() <= 0 || !dec.IsInt64() || dec.Int64() < 0 || dec.Int64() > 36 || !ts.IsInt64() {
		ctx.Revert("invalid asset oracle update")
	}
	if ts.Int64() > ctx.BlockTime+30 || ctx.BlockTime-ts.Int64() > 900 {
		ctx.Revert("asset oracle update is stale or future-dated")
	}
	ctx.Set("oracle_price18:"+token, price.String())
	ctx.Set("oracle_updated_at:"+token, ts.String())
	ctx.Set("token_decimals:"+token, dec.String())
	ctx.Commit()
	ctx.Emit("VaultAssetOracleUpdated", map[string]interface{}{"token": token, "priceUSD18": price.String(), "decimals": dec.String(), "timestamp": ts.String()})
}

// SetAssetOracleSource records one independent valuation feed. Three fresh,
// sufficiently-confident sources are required; aggregation is computed during
// valuation so stale feeds cannot leave a cached price usable.
func (v *StrategyVault) SetAssetOracleSource(ctx *bc.Context, token string, source string, priceUSD18 string, confidenceBPS string, decimals string, timestamp string) {
	v.requireManager(ctx)
	token, source = norm(token), norm(source)
	price, confidence, dec, ts := parseBig(priceUSD18), parseBig(confidenceBPS), parseBig(decimals), parseBig(timestamp)
	if token == "" || source == "" || price.Sign() <= 0 || !confidence.IsInt64() || confidence.Int64() < 5000 || confidence.Int64() > 10000 || !dec.IsInt64() || dec.Int64() < 0 || dec.Int64() > 36 || !ts.IsInt64() {
		ctx.Revert("invalid multi-source oracle update")
	}
	if ts.Int64() > ctx.BlockTime+30 || ctx.BlockTime-ts.Int64() > 900 {
		ctx.Revert("oracle source is stale or future-dated")
	}
	seenKey := "oracle_source_seen:" + token + ":" + source
	if ctx.Get(seenKey) != "1" {
		count := parseBig(ctx.Get("oracle_source_count:" + token))
		if count.Cmp(big.NewInt(9)) >= 0 {
			ctx.Revert("at most 9 oracle sources")
		}
		ctx.Set("oracle_source_at:"+token+":"+count.String(), source)
		ctx.Set("oracle_source_count:"+token, new(big.Int).Add(count, big.NewInt(1)).String())
		ctx.Set(seenKey, "1")
	}
	prefix := "oracle_source_"
	ctx.Set(prefix+"price18:"+token+":"+source, price.String())
	ctx.Set(prefix+"confidence_bps:"+token+":"+source, confidence.String())
	ctx.Set(prefix+"updated_at:"+token+":"+source, ts.String())
	ctx.Set("token_decimals:"+token, dec.String())
	ctx.Commit()
	ctx.Emit("VaultOracleSourceUpdated", map[string]interface{}{"token": token, "source": source, "priceUSD18": price.String(), "confidenceBPS": confidence.String(), "timestamp": ts.String()})
}

func (v *StrategyVault) SetValuationPolicy(ctx *bc.Context, delaySeconds string) {
	v.requireManager(ctx)
	delay := parseBig(delaySeconds)
	if !delay.IsInt64() || delay.Int64() < 0 || delay.Int64() > 300 {
		ctx.Revert("valuation delay must be 0..300 seconds")
	}
	ctx.Set("valuation_delay_seconds", delay.String())
	ctx.Emit("VaultValuationPolicyUpdated", map[string]interface{}{"delaySeconds": delay.String(), "sourceQuorum": 3, "outlierBPS": 2000})
}

func (v *StrategyVault) AssetOracleStatus(ctx *bc.Context, token string) {
	token = norm(token)
	result := map[string]interface{}{"token": token, "sources": parseBig(ctx.Get("oracle_source_count:" + token)).String(), "priceUSD18": v.tokenPrice18(ctx, token).String(), "delaySeconds": ctx.Get("valuation_delay_seconds")}
	raw, _ := json.Marshal(result)
	ctx.Set("output", string(raw))
}

func (v *StrategyVault) SetWithdrawalPolicy(ctx *bc.Context, delaySeconds string, liquidBufferBps string) {
	v.requireManager(ctx)
	delay := parseBig(delaySeconds)
	buffer := parseBig(liquidBufferBps)
	if !delay.IsInt64() || delay.Int64() < 0 || delay.Int64() > 604800 {
		ctx.Revert("withdrawal delay must be 0..604800 seconds")
	}
	if !buffer.IsInt64() || buffer.Int64() < 500 || buffer.Int64() > 5000 {
		ctx.Revert("liquid buffer must be 500..5000 bps")
	}
	ctx.Set("withdrawal_delay", delay.String())
	ctx.Set("liquid_buffer_bps", buffer.String())
	ctx.Commit()
	ctx.Emit("VaultWithdrawalPolicyUpdated", map[string]interface{}{"delaySeconds": delay.String(), "liquidBufferBps": buffer.String()})
}

func (v *StrategyVault) SetFeePolicy(ctx *bc.Context, managementFeeBps string, performanceFeeBps string, recipient string) {
	v.requireManager(ctx)
	v.accrueFees(ctx)
	management, performance := parseBig(managementFeeBps), parseBig(performanceFeeBps)
	if !management.IsInt64() || management.Int64() < 0 || management.Int64() > 200 || !performance.IsInt64() || performance.Int64() < 0 || performance.Int64() > 2000 || norm(recipient) == "" {
		ctx.Revert("fee limits: management <=200 bps/year, performance <=2000 bps")
	}
	ctx.Set("management_fee_bps", management.String())
	ctx.Set("performance_fee_bps", performance.String())
	ctx.Set("fee_recipient", norm(recipient))
	ctx.Emit("VaultFeePolicyUpdated", map[string]interface{}{"managementFeeBps": management.String(), "performanceFeeBps": performance.String(), "recipient": norm(recipient)})
}

func (v *StrategyVault) Transfer(ctx *bc.Context, to string, shares string) {
	from := actorAddr(ctx)
	amt := parseBig(shares)
	v.moveShares(ctx, from, to, amt)
	ctx.Emit("VaultShareTransfer", map[string]interface{}{"from": from, "to": norm(to), "shares": amt.String()})
}

func (v *StrategyVault) Approve(ctx *bc.Context, spender string, shares string) {
	owner, spender, amt := actorAddr(ctx), norm(spender), parseBig(shares)
	if spender == "" || amt.Sign() < 0 {
		ctx.Revert("invalid share approval")
	}
	ctx.Set("share_allow:"+owner+":"+spender, amt.String())
	ctx.Emit("VaultShareApproval", map[string]interface{}{"owner": owner, "spender": spender, "shares": amt.String()})
}

func (v *StrategyVault) Allowance(ctx *bc.Context, owner string, spender string) {
	ctx.Set("output", v.shareAllowance(ctx, owner, spender).String())
}

func (v *StrategyVault) TransferFrom(ctx *bc.Context, from string, to string, shares string) {
	spender, amt := actorAddr(ctx), parseBig(shares)
	allowed := v.shareAllowance(ctx, from, spender)
	if allowed.Cmp(amt) < 0 {
		ctx.Revert("share allowance exceeded")
	}
	v.moveShares(ctx, from, to, amt)
	ctx.Set("share_allow:"+norm(from)+":"+spender, new(big.Int).Sub(allowed, amt).String())
	ctx.Emit("VaultShareTransfer", map[string]interface{}{"from": norm(from), "to": norm(to), "shares": amt.String(), "spender": spender})
}

func (v *StrategyVault) loadWithdrawal(ctx *bc.Context, id string) withdrawalRequest {
	var req withdrawalRequest
	_ = json.Unmarshal([]byte(ctx.Get("withdrawal:"+id)), &req)
	return req
}

func (v *StrategyVault) saveWithdrawal(ctx *bc.Context, req withdrawalRequest) {
	raw, _ := json.Marshal(req)
	ctx.Set("withdrawal:"+req.ID, string(raw))
}

// RequestWithdrawal escrows shares in a FIFO queue. Processing returns the
// same pro-rata basket of LP positions, preserving ownership through every
// dynamic movement without promising a fixed fiat value.
func (v *StrategyVault) RequestWithdrawal(ctx *bc.Context, shares string, minOutBps string) {
	user := actorAddr(ctx)
	amt := parseBig(shares)
	minOut := parseBig(minOutBps)
	requirePositive(ctx, "shares", amt)
	if v.shareOf(ctx, user).Cmp(amt) < 0 {
		ctx.Revert("insufficient vault shares")
	}
	if !minOut.IsInt64() || minOut.Int64() < 9000 || minOut.Int64() > 10000 {
		ctx.Revert("minOutBps must be 9000..10000")
	}
	tail := new(big.Int).Add(parseBig(ctx.Get("withdrawal_tail")), big.NewInt(1))
	id := tail.String()
	delay := v.configInt(ctx, "withdrawal_delay", 60)
	req := withdrawalRequest{ID: id, Owner: user, Shares: amt.String(), MinOutBps: minOut.Int64(), RequestedAt: ctx.BlockTime, ExecutableAt: ctx.BlockTime + delay, Status: "queued"}
	v.setShare(ctx, user, new(big.Int).Sub(v.shareOf(ctx, user), amt))
	ctx.Set("queued_shares:"+user, new(big.Int).Add(parseBig(ctx.Get("queued_shares:"+user)), amt).String())
	ctx.Set("withdrawal_tail", id)
	v.saveWithdrawal(ctx, req)
	ctx.Set("output", id)
	ctx.Commit()
	ctx.Emit("VaultWithdrawalQueued", map[string]interface{}{"id": id, "user": user, "shares": amt.String(), "executableAt": req.ExecutableAt})
}

func (v *StrategyVault) advanceWithdrawalHead(ctx *bc.Context) {
	head := parseBig(ctx.Get("withdrawal_head"))
	tail := parseBig(ctx.Get("withdrawal_tail"))
	for head.Cmp(tail) <= 0 {
		req := v.loadWithdrawal(ctx, head.String())
		if req.Status == "queued" {
			break
		}
		head.Add(head, big.NewInt(1))
	}
	ctx.Set("withdrawal_head", head.String())
}

func (v *StrategyVault) CancelWithdrawal(ctx *bc.Context, id string) {
	user := actorAddr(ctx)
	req := v.loadWithdrawal(ctx, strings.TrimSpace(id))
	if req.Status != "queued" || req.Owner != user {
		ctx.Revert("withdrawal is not cancellable by caller")
	}
	amt := parseBig(req.Shares)
	req.Status = "cancelled"
	v.saveWithdrawal(ctx, req)
	v.setShare(ctx, user, new(big.Int).Add(v.shareOf(ctx, user), amt))
	ctx.Set("queued_shares:"+user, new(big.Int).Sub(parseBig(ctx.Get("queued_shares:"+user)), amt).String())
	v.advanceWithdrawalHead(ctx)
	ctx.Commit()
	ctx.Emit("VaultWithdrawalCancelled", map[string]interface{}{"id": req.ID, "user": user})
}

func (v *StrategyVault) TransferWithdrawalReceipt(ctx *bc.Context, id string, newOwner string) {
	owner, newOwner := actorAddr(ctx), norm(newOwner)
	req := v.loadWithdrawal(ctx, strings.TrimSpace(id))
	if req.Status != "queued" || req.Owner != owner || newOwner == "" {
		ctx.Revert("withdrawal receipt is not transferable")
	}
	amt := parseBig(req.Shares)
	ctx.Set("queued_shares:"+owner, new(big.Int).Sub(parseBig(ctx.Get("queued_shares:"+owner)), amt).String())
	ctx.Set("queued_shares:"+newOwner, new(big.Int).Add(parseBig(ctx.Get("queued_shares:"+newOwner)), amt).String())
	req.Owner = newOwner
	v.saveWithdrawal(ctx, req)
	ctx.Emit("WithdrawalReceiptTransferred", map[string]interface{}{"id": id, "from": owner, "to": newOwner})
}

func (v *StrategyVault) GetWithdrawalReceipt(ctx *bc.Context, id string) {
	ctx.Set("output", ctx.Get("withdrawal:"+strings.TrimSpace(id)))
}

func (v *StrategyVault) ProcessWithdrawal(ctx *bc.Context, id string) {
	v.accrueFees(ctx)
	v.advanceWithdrawalHead(ctx)
	if strings.TrimSpace(id) != ctx.Get("withdrawal_head") {
		ctx.Revert("FIFO: request is not at queue head")
	}
	req := v.loadWithdrawal(ctx, strings.TrimSpace(id))
	if req.Status != "queued" || ctx.BlockTime < req.ExecutableAt {
		ctx.Revert("withdrawal is not executable")
	}
	shares := parseBig(req.Shares)
	totalShares := v.totalShares(ctx)
	if shares.Sign() <= 0 || totalShares.Cmp(shares) < 0 {
		ctx.Revert("invalid queued share amount")
	}
	outputs := map[string]string{}
	withdrawNAV := big.NewInt(0)
	count := parseBig(ctx.Get("position_pair_count")).Int64()
	for i := int64(0); i < count; i++ {
		pair := norm(ctx.Get("position_pair_at:" + big.NewInt(i).String()))
		position := v.positionOf(ctx, pair)
		if pair == "" || position.Sign() <= 0 {
			continue
		}
		lpOut := new(big.Int).Div(new(big.Int).Mul(position, shares), totalShares)
		if lpOut.Sign() <= 0 {
			continue
		}
		withdrawNAV.Add(withdrawNAV, v.lpNAV18(ctx, pair, lpOut))
		if _, err := ctx.Call(pair, "Transfer", []string{req.Owner, lpOut.String()}); err != nil {
			ctx.Revert("queued LP transfer failed: " + err.Error())
		}
		v.setPosition(ctx, pair, new(big.Int).Sub(position, lpOut))
		outputs[pair] = lpOut.String()
	}
	if len(outputs) == 0 {
		ctx.Revert("withdrawal produced no LP output")
	}
	v.reduceHODLBenchmark(ctx, shares, totalShares)
	ctx.Set("totalShares", new(big.Int).Sub(totalShares, shares).String())
	ctx.Set("queued_shares:"+req.Owner, new(big.Int).Sub(parseBig(ctx.Get("queued_shares:"+req.Owner)), shares).String())
	basis := parseBig(ctx.Get("basis_nav:" + req.Owner))
	ownerEconomicShares := new(big.Int).Add(shares, v.shareOf(ctx, req.Owner))
	basisUsed := big.NewInt(0)
	if ownerEconomicShares.Sign() > 0 {
		basisUsed.Div(new(big.Int).Mul(basis, shares), ownerEconomicShares)
	}
	ctx.Set("basis_nav:"+req.Owner, new(big.Int).Sub(basis, basisUsed).String())
	ctx.Set("realized_pnl_nav", new(big.Int).Add(parseBig(ctx.Get("realized_pnl_nav")), new(big.Int).Sub(withdrawNAV, basisUsed)).String())
	ctx.Set("capital_out_nav", new(big.Int).Add(parseBig(ctx.Get("capital_out_nav")), withdrawNAV).String())
	req.Status = "processed"
	v.saveWithdrawal(ctx, req)
	v.advanceWithdrawalHead(ctx)
	raw, _ := json.Marshal(outputs)
	ctx.Set("output", string(raw))
	ctx.Commit()
	ctx.Emit("VaultWithdrawalProcessed", map[string]interface{}{"id": req.ID, "user": req.Owner, "shares": shares.String(), "lpOutputs": string(raw)})
}

func (v *StrategyVault) TotalAssets(ctx *bc.Context) {
	if ctx.Get("erc4626_mode") == "true" {
		ctx.Set("output", v.erc4626TotalAssets(ctx, ctx.Get("erc4626_asset")).String())
		return
	}
	ctx.Set("output", v.totalNAV18(ctx).String())
}

func (v *StrategyVault) PreviewDepositLP(ctx *bc.Context, pair string, lpAmount string) {
	depositNAV, totalNAV, totalShares := v.lpNAV18(ctx, norm(pair), parseBig(lpAmount)), v.totalNAV18(ctx), v.totalShares(ctx)
	if depositNAV.Sign() <= 0 {
		ctx.Set("output", "0")
		return
	}
	if totalShares.Sign() == 0 {
		ctx.Set("output", depositNAV.String())
		return
	}
	if totalNAV.Sign() <= 0 {
		ctx.Set("output", "0")
		return
	}
	ctx.Set("output", new(big.Int).Div(new(big.Int).Mul(depositNAV, totalShares), totalNAV).String())
}

// ConfigureERC4626Asset enables strict single-underlying mode. It can only be
// enabled before deposits (or while the sole position is already this asset),
// because ERC-4626 cannot honestly represent a dynamically changing basket as
// one ERC-20 underlying.
func (v *StrategyVault) ConfigureERC4626Asset(ctx *bc.Context, pair string) {
	v.requireManager(ctx)
	pair = norm(pair)
	if pair == "" {
		ctx.Revert("canonical LP asset required")
	}
	count := parseBig(ctx.Get("position_pair_count")).Int64()
	for i := int64(0); i < count; i++ {
		existing := norm(ctx.Get("position_pair_at:" + big.NewInt(i).String()))
		if existing != "" && existing != pair && v.positionOf(ctx, existing).Sign() > 0 {
			ctx.Revert("vault already contains another asset")
		}
	}
	ctx.Set("erc4626_asset", pair)
	ctx.Set("erc4626_mode", "true")
	if ctx.Get("erc4626_min_initial_assets") == "" {
		ctx.Set("erc4626_min_initial_assets", "1000")
	}
	ctx.Emit("ERC4626ModeConfigured", map[string]interface{}{"asset": pair})
}

func (v *StrategyVault) SetERC4626MinimumInitialAssets(ctx *bc.Context, minimum string) {
	v.requireManager(ctx)
	if v.totalShares(ctx).Sign() > 0 {
		ctx.Revert("minimum cannot change after first deposit")
	}
	amount := parseBig(minimum)
	if amount.Sign() <= 0 {
		ctx.Revert("positive minimum required")
	}
	ctx.Set("erc4626_min_initial_assets", amount.String())
	ctx.Emit("ERC4626MinimumUpdated", map[string]interface{}{"minimumAssets": amount.String()})
}

func (v *StrategyVault) erc4626Asset(ctx *bc.Context) string {
	asset := norm(ctx.Get("erc4626_asset"))
	if ctx.Get("erc4626_mode") != "true" || asset == "" {
		ctx.Revert("ERC-4626 mode is not configured")
	}
	return asset
}

func (v *StrategyVault) ConvertToShares(ctx *bc.Context, assets string) {
	asset, amount := v.erc4626Asset(ctx), parseBig(assets)
	totalAssets, totalShares := v.erc4626TotalAssets(ctx, asset), v.totalShares(ctx)
	if amount.Sign() <= 0 {
		ctx.Set("output", "0")
		return
	}
	ctx.Set("output", new(big.Int).Div(new(big.Int).Mul(amount, new(big.Int).Add(totalShares, big.NewInt(1))), new(big.Int).Add(totalAssets, big.NewInt(1))).String())
}

func (v *StrategyVault) PreviewDeposit(ctx *bc.Context, assets string) {
	v.ConvertToShares(ctx, assets)
}
func (v *StrategyVault) PreviewMint(ctx *bc.Context, shares string) {
	asset, amount := v.erc4626Asset(ctx), parseBig(shares)
	totalAssets, totalShares := v.erc4626TotalAssets(ctx, asset), v.totalShares(ctx)
	if amount.Sign() <= 0 {
		ctx.Set("output", amount.String())
		return
	}
	// Round up assets required for exact shares.
	denominator := new(big.Int).Add(totalShares, big.NewInt(1))
	n := new(big.Int).Mul(amount, new(big.Int).Add(totalAssets, big.NewInt(1)))
	ctx.Set("output", new(big.Int).Div(new(big.Int).Add(n, new(big.Int).Sub(denominator, big.NewInt(1))), denominator).String())
}

func (v *StrategyVault) Deposit(ctx *bc.Context, assets string, receiver string) {
	asset, owner := v.erc4626Asset(ctx), actorAddr(ctx)
	shares := v.depositLPFor(ctx, asset, parseBig(assets), owner, receiver)
	ctx.Emit("Deposit", map[string]interface{}{"sender": owner, "owner": norm(receiver), "assets": assets, "shares": shares.String()})
}

func (v *StrategyVault) Mint(ctx *bc.Context, shares string, receiver string) {
	requested := parseBig(shares)
	asset := v.erc4626Asset(ctx)
	totalAssets, totalShares := v.erc4626TotalAssets(ctx, asset), v.totalShares(ctx)
	denominator := new(big.Int).Add(totalShares, big.NewInt(1))
	n := new(big.Int).Mul(requested, new(big.Int).Add(totalAssets, big.NewInt(1)))
	assets := new(big.Int).Div(new(big.Int).Add(n, new(big.Int).Sub(denominator, big.NewInt(1))), denominator)
	minted := v.depositLPFor(ctx, asset, assets, actorAddr(ctx), receiver)
	if minted.Cmp(requested) < 0 {
		ctx.Revert("mint rounded below requested shares")
	}
	// ERC-4626 mint returns the amount of assets consumed, not shares minted.
	ctx.Set("output", assets.String())
	ctx.Emit("Deposit", map[string]interface{}{"sender": actorAddr(ctx), "owner": norm(receiver), "assets": assets.String(), "shares": requested.String()})
}

func (v *StrategyVault) MaxDeposit(ctx *bc.Context, receiver string) {
	_ = receiver
	v.erc4626Asset(ctx)
	ctx.Set("output", new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1)).String())
}

func (v *StrategyVault) MaxMint(ctx *bc.Context, receiver string) {
	_ = receiver
	v.erc4626Asset(ctx)
	ctx.Set("output", new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1)).String())
}

func (v *StrategyVault) MaxWithdraw(ctx *bc.Context, owner string) {
	asset := v.erc4626Asset(ctx)
	shares, totalShares := v.shareOf(ctx, owner), v.totalShares(ctx)
	if totalShares.Sign() == 0 {
		ctx.Set("output", "0")
		return
	}
	ctx.Set("output", new(big.Int).Div(new(big.Int).Mul(new(big.Int).Add(v.erc4626TotalAssets(ctx, asset), big.NewInt(1)), shares), new(big.Int).Add(totalShares, big.NewInt(1))).String())
}

func (v *StrategyVault) PreviewRedeem(ctx *bc.Context, shares string) { v.ConvertToAssets(ctx, shares) }
func (v *StrategyVault) MaxRedeem(ctx *bc.Context, owner string) {
	ctx.Set("output", v.shareOf(ctx, owner).String())
}
func (v *StrategyVault) Asset(ctx *bc.Context) {
	if ctx.Get("erc4626_mode") == "true" {
		ctx.Set("output", ctx.Get("erc4626_asset"))
		return
	}
	ctx.Set("output", "MULTI_ASSET_LP_BASKET")
}

func (v *StrategyVault) PreviewWithdraw(ctx *bc.Context, assets string) {
	asset, amount := v.erc4626Asset(ctx), parseBig(assets)
	totalAssets, totalShares := v.erc4626TotalAssets(ctx, asset), v.totalShares(ctx)
	if amount.Sign() <= 0 || totalAssets.Sign() <= 0 {
		ctx.Set("output", "0")
		return
	}
	denominator := new(big.Int).Add(totalAssets, big.NewInt(1))
	n := new(big.Int).Mul(amount, new(big.Int).Add(totalShares, big.NewInt(1)))
	ctx.Set("output", new(big.Int).Div(new(big.Int).Add(n, new(big.Int).Sub(denominator, big.NewInt(1))), denominator).String())
}

func (v *StrategyVault) redeemERC4626(ctx *bc.Context, shares *big.Int, receiver, owner string, exactAssets *big.Int) *big.Int {
	asset, caller := v.erc4626Asset(ctx), actorAddr(ctx)
	receiver, owner = norm(receiver), norm(owner)
	if receiver == "" || owner == "" || shares.Sign() <= 0 || v.shareOf(ctx, owner).Cmp(shares) < 0 {
		ctx.Revert("invalid ERC-4626 redemption")
	}
	if caller != owner {
		allowance := v.shareAllowance(ctx, owner, caller)
		if allowance.Cmp(shares) < 0 {
			ctx.Revert("share allowance exceeded")
		}
		ctx.Set("share_allow:"+owner+":"+caller, new(big.Int).Sub(allowance, shares).String())
	}
	totalShares, position := v.totalShares(ctx), v.erc4626TotalAssets(ctx, asset)
	assets := new(big.Int).Div(new(big.Int).Mul(new(big.Int).Add(position, big.NewInt(1)), shares), new(big.Int).Add(totalShares, big.NewInt(1)))
	if exactAssets != nil {
		assets = new(big.Int).Set(exactAssets)
	}
	if assets.Sign() <= 0 || position.Cmp(assets) < 0 {
		ctx.Revert("insufficient canonical assets")
	}
	withdrawNAV := v.lpNAV18(ctx, asset, assets)
	ownerShares := v.shareOf(ctx, owner)
	basis := parseBig(ctx.Get("basis_nav:" + owner))
	basisUsed := new(big.Int).Div(new(big.Int).Mul(basis, shares), ownerShares)
	if _, err := ctx.Call(asset, "Transfer", []string{receiver, assets.String()}); err != nil {
		ctx.Revert("canonical LP transfer failed: " + err.Error())
	}
	v.reduceHODLBenchmark(ctx, shares, totalShares)
	v.setShare(ctx, owner, new(big.Int).Sub(ownerShares, shares))
	ctx.Set("totalShares", new(big.Int).Sub(totalShares, shares).String())
	v.setPosition(ctx, asset, new(big.Int).Sub(position, assets))
	ctx.Set("basis_nav:"+owner, new(big.Int).Sub(basis, basisUsed).String())
	ctx.Set("realized_pnl_nav", new(big.Int).Add(parseBig(ctx.Get("realized_pnl_nav")), new(big.Int).Sub(withdrawNAV, basisUsed)).String())
	ctx.Set("capital_out_nav", new(big.Int).Add(parseBig(ctx.Get("capital_out_nav")), withdrawNAV).String())
	ctx.Set("output", assets.String())
	ctx.Commit()
	ctx.Emit("Withdraw", map[string]interface{}{"sender": caller, "receiver": receiver, "owner": owner, "assets": assets.String(), "shares": shares.String()})
	return assets
}

func (v *StrategyVault) Withdraw(ctx *bc.Context, assets string, receiver string, owner string) {
	amount := parseBig(assets)
	asset := v.erc4626Asset(ctx)
	totalAssets, totalShares := v.erc4626TotalAssets(ctx, asset), v.totalShares(ctx)
	if amount.Sign() <= 0 || totalAssets.Sign() <= 0 {
		ctx.Revert("invalid withdrawal assets")
	}
	denominator := new(big.Int).Add(totalAssets, big.NewInt(1))
	n := new(big.Int).Mul(amount, new(big.Int).Add(totalShares, big.NewInt(1)))
	shares := new(big.Int).Div(new(big.Int).Add(n, new(big.Int).Sub(denominator, big.NewInt(1))), denominator)
	v.redeemERC4626(ctx, shares, receiver, owner, amount)
	// ERC-4626 withdraw returns the shares burned.
	ctx.Set("output", shares.String())
}

func (v *StrategyVault) Redeem(ctx *bc.Context, shares string, receiver string, owner string) {
	v.redeemERC4626(ctx, parseBig(shares), receiver, owner, nil)
}

func (v *StrategyVault) WithdrawSingleAsset(ctx *bc.Context, pair string, shares string, targetToken string, minAmountOut string) {
	pair, target, user := norm(pair), norm(targetToken), actorAddr(ctx)
	amt := parseBig(shares)
	v.accrueFees(ctx)
	if amt.Sign() <= 0 || v.shareOf(ctx, user).Cmp(amt) < 0 {
		ctx.Revert("insufficient shares")
	}
	lpAmount := v.redeemLPAmount(ctx, pair, amt)
	withdrawNAV := v.lpNAV18(ctx, pair, lpAmount)
	userShares := v.shareOf(ctx, user)
	removed, err := ctx.Call(pair, "RemoveLiquidityTo", []string{ctx.ContractAddr, lpAmount.String()})
	if err != nil || !removed.Success {
		ctx.Revert("single-asset remove liquidity failed")
	}
	a0, a1 := v.parseAmounts(ctx, removed.Output)
	t0, t1 := v.pairToken(ctx, pair, "Token0"), v.pairToken(ctx, pair, "Token1")
	total := big.NewInt(0)
	if target == t0 {
		total.Add(a0, v.swapVaultToken(ctx, t1, t0, a1, big.NewInt(0)))
	} else if target == t1 {
		total.Add(a1, v.swapVaultToken(ctx, t0, t1, a0, big.NewInt(0)))
	} else {
		ctx.Revert("target token is not in selected pair")
	}
	if total.Cmp(parseBig(minAmountOut)) < 0 {
		ctx.Revert("single-asset output below minimum")
	}
	v.sendVaultToken(ctx, target, user, total)
	v.reduceHODLBenchmark(ctx, amt, v.totalShares(ctx))
	v.recordRedemptionAccounting(ctx, user, amt, userShares, withdrawNAV)
	v.setShare(ctx, user, new(big.Int).Sub(v.shareOf(ctx, user), amt))
	ctx.Set("totalShares", new(big.Int).Sub(v.totalShares(ctx), amt).String())
	v.setPosition(ctx, pair, new(big.Int).Sub(v.positionOf(ctx, pair), lpAmount))
	ctx.Set("output", total.String())
	ctx.Emit("VaultSingleAssetWithdrawal", map[string]interface{}{"user": user, "pair": pair, "targetToken": target, "shares": amt.String(), "amountOut": total.String()})
}

func (v *StrategyVault) SetEmergencyLossPolicy(ctx *bc.Context, enabled string, maxLossBps string, incidentHash string) {
	v.requireManager(ctx)
	maxLoss := parseBig(maxLossBps)
	if !maxLoss.IsInt64() || maxLoss.Int64() < 0 || maxLoss.Int64() > 5000 || (strings.EqualFold(enabled, "true") && strings.TrimSpace(incidentHash) == "") {
		ctx.Revert("invalid emergency loss policy")
	}
	ctx.Set("emergency_mode", strings.ToLower(enabled))
	ctx.Set("emergency_max_loss_bps", maxLoss.String())
	ctx.Set("emergency_incident_hash", strings.TrimSpace(incidentHash))
	ctx.Emit("VaultEmergencyPolicy", map[string]interface{}{"enabled": strings.EqualFold(enabled, "true"), "maxLossBps": maxLoss.String(), "incidentHash": incidentHash})
}

func (v *StrategyVault) EmergencyRedeemBasket(ctx *bc.Context, shares string) {
	if ctx.Get("emergency_mode") != "true" {
		ctx.Revert("emergency mode inactive")
	}
	user, amt, totalShares := actorAddr(ctx), parseBig(shares), v.totalShares(ctx)
	if amt.Sign() <= 0 || v.shareOf(ctx, user).Cmp(amt) < 0 || totalShares.Sign() <= 0 {
		ctx.Revert("invalid emergency redemption")
	}
	basis := parseBig(ctx.Get("basis_nav:" + user))
	claimBasis := new(big.Int).Div(new(big.Int).Mul(basis, amt), v.shareOf(ctx, user))
	claimNAV := new(big.Int).Div(new(big.Int).Mul(v.totalNAV18(ctx), amt), totalShares)
	if claimBasis.Sign() > 0 && claimNAV.Cmp(claimBasis) < 0 {
		lossBPS := new(big.Int).Div(new(big.Int).Mul(new(big.Int).Sub(claimBasis, claimNAV), big.NewInt(10000)), claimBasis)
		if lossBPS.Cmp(big.NewInt(v.configInt(ctx, "emergency_max_loss_bps", 3000))) > 0 {
			ctx.Revert("loss exceeds disclosed emergency bound")
		}
	}
	outputs := map[string]string{}
	count := parseBig(ctx.Get("position_pair_count")).Int64()
	for i := int64(0); i < count; i++ {
		pair := norm(ctx.Get("position_pair_at:" + big.NewInt(i).String()))
		position := v.positionOf(ctx, pair)
		lpOut := new(big.Int).Div(new(big.Int).Mul(position, amt), totalShares)
		if lpOut.Sign() > 0 {
			if _, err := ctx.Call(pair, "Transfer", []string{user, lpOut.String()}); err != nil {
				ctx.Revert("emergency LP transfer failed")
			}
			v.setPosition(ctx, pair, new(big.Int).Sub(position, lpOut))
			outputs[pair] = lpOut.String()
		}
	}
	v.setShare(ctx, user, new(big.Int).Sub(v.shareOf(ctx, user), amt))
	ctx.Set("totalShares", new(big.Int).Sub(totalShares, amt).String())
	ctx.Set("basis_nav:"+user, new(big.Int).Sub(basis, claimBasis).String())
	v.reduceHODLBenchmark(ctx, amt, totalShares)
	ctx.Set("realized_pnl_nav", new(big.Int).Add(parseBig(ctx.Get("realized_pnl_nav")), new(big.Int).Sub(claimNAV, claimBasis)).String())
	ctx.Set("capital_out_nav", new(big.Int).Add(parseBig(ctx.Get("capital_out_nav")), claimNAV).String())
	raw, _ := json.Marshal(outputs)
	ctx.Set("output", string(raw))
	ctx.Emit("VaultEmergencyRedeemed", map[string]interface{}{"user": user, "shares": amt.String(), "outputs": string(raw)})
}

func (v *StrategyVault) AccountingStatus(ctx *bc.Context) {
	nav, in, out := v.totalNAV18(ctx), parseBig(ctx.Get("capital_in_nav")), parseBig(ctx.Get("capital_out_nav"))
	totalReturn := new(big.Int).Sub(new(big.Int).Add(nav, out), in)
	hodlNAV := v.hodlNAV18(ctx)
	strategyVsHODL := new(big.Int).Sub(nav, hodlNAV)
	data := map[string]string{"nav": nav.String(), "capital_in": in.String(), "capital_out": out.String(), "total_return": totalReturn.String(), "realized_pnl": ctx.Get("realized_pnl_nav"), "hodl_benchmark_nav": hodlNAV.String(), "strategy_vs_hodl_nav": strategyVsHODL.String(), "fee_shares": ctx.Get("fee_shares_minted"), "high_water_mark_x18": ctx.Get("high_water_mark_x18")}
	raw, _ := json.Marshal(data)
	ctx.Set("output", string(raw))
}

func (v *StrategyVault) ConvertToAssets(ctx *bc.Context, shares string) {
	amt := parseBig(shares)
	total := v.totalShares(ctx)
	if amt.Sign() <= 0 || total.Sign() <= 0 {
		ctx.Set("output", "0")
		return
	}
	assets := v.totalNAV18(ctx)
	if ctx.Get("erc4626_mode") == "true" {
		assets = v.erc4626TotalAssets(ctx, ctx.Get("erc4626_asset"))
		ctx.Set("output", new(big.Int).Div(new(big.Int).Mul(new(big.Int).Add(assets, big.NewInt(1)), amt), new(big.Int).Add(total, big.NewInt(1))).String())
		return
	}
	ctx.Set("output", new(big.Int).Div(new(big.Int).Mul(assets, amt), total).String())
}

// AutoRebalance picks the highest scoring pair from a comma-separated candidate
// list using current routing weight and reserve depth, then moves liquidity
// there from current_pair.
func (v *StrategyVault) AutoRebalance(ctx *bc.Context, candidatePairsCSV string, lpAmount string) {
	v.requireManager(ctx)
	current := norm(ctx.Get("current_pair"))
	if current == "" {
		ctx.Revert("no current pair")
	}
	best := v.bestPair(ctx, candidatePairsCSV)
	if best == "" {
		ctx.Revert("no target pair")
	}
	if best == current {
		ctx.Set("output", current)
		ctx.Emit("VaultRebalanceSkipped", map[string]interface{}{"pair": current, "reason": "already best"})
		return
	}
	v.rebalance(ctx, current, best, lpAmount)
}

func (v *StrategyVault) bestPair(ctx *bc.Context, candidatePairsCSV string) string {
	best := ""
	bestScore := big.NewInt(-1)
	for _, raw := range strings.Split(candidatePairsCSV, ",") {
		pair := norm(raw)
		if pair == "" {
			continue
		}
		r0, r1, _ := v.pairReserves(ctx, pair)
		depth := r0
		if r1.Cmp(depth) < 0 {
			depth = r1
		}
		weight := v.routingWeight(ctx, pair)
		oracle := new(big.Int).Div(v.oracleDemand(ctx, pair), big.NewInt(100))
		scoreWeight := new(big.Int).Add(weight, oracle)
		score := new(big.Int).Mul(depth, scoreWeight)
		if score.Cmp(bestScore) > 0 {
			bestScore = score
			best = pair
		}
	}
	return best
}

func (v *StrategyVault) SelectBestTarget(ctx *bc.Context, candidatePairsCSV string) {
	best := v.bestPair(ctx, candidatePairsCSV)
	ctx.Set("output", best)
	ctx.Emit("VaultBestTarget", map[string]interface{}{"pair": best})
}

func (v *StrategyVault) BalanceOf(ctx *bc.Context, user string) {
	ctx.Set("output", v.shareOf(ctx, user).String())
}

func (v *StrategyVault) TotalSupply(ctx *bc.Context) {
	ctx.Set("output", v.totalShares(ctx).String())
}

func (v *StrategyVault) PositionOf(ctx *bc.Context, pair string) {
	ctx.Set("output", v.positionOf(ctx, pair).String())
}

func (v *StrategyVault) CurrentPair(ctx *bc.Context) {
	ctx.Set("output", norm(ctx.Get("current_pair")))
}

func (v *StrategyVault) GetSafetyLimits(ctx *bc.Context) {
	data := map[string]string{
		"max_move_bps":           big.NewInt(v.configInt(ctx, "max_move_bps", 2500)).String(),
		"min_rebalance_interval": big.NewInt(v.configInt(ctx, "min_rebalance_interval", 300)).String(),
		"max_slippage_bps":       big.NewInt(v.configInt(ctx, "max_slippage_bps", 500)).String(),
		"total_assets_usd18":     v.totalNAV18(ctx).String(),
		"withdrawal_head":        ctx.Get("withdrawal_head"),
		"withdrawal_tail":        ctx.Get("withdrawal_tail"),
		"liquid_buffer_bps":      big.NewInt(v.configInt(ctx, "liquid_buffer_bps", 1500)).String(),
	}
	encoded, _ := json.Marshal(data)
	ctx.Set("output", string(encoded))
}

func (v *StrategyVault) GetOracleDemand(ctx *bc.Context, pair string) {
	ctx.Set("output", v.oracleDemand(ctx, pair).String())
}

func (v *StrategyVault) GetVaultInfo(ctx *bc.Context, user string) {
	user = norm(user)
	data := map[string]string{
		"manager":                norm(ctx.Get("manager")),
		"router":                 norm(ctx.Get("router")),
		"current_pair":           norm(ctx.Get("current_pair")),
		"last_rebalance":         ctx.Get("last_rebalance"),
		"total_shares":           v.totalShares(ctx).String(),
		"user_shares":            v.shareOf(ctx, user).String(),
		"max_move_bps":           big.NewInt(v.configInt(ctx, "max_move_bps", 2500)).String(),
		"min_rebalance_interval": big.NewInt(v.configInt(ctx, "min_rebalance_interval", 300)).String(),
		"max_slippage_bps":       big.NewInt(v.configInt(ctx, "max_slippage_bps", 500)).String(),
	}
	encoded, _ := json.Marshal(data)
	ctx.Set("output", string(encoded))
}

var Contract = &StrategyVault{}
