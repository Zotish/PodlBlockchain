//go:build ignore

package main

import (
	"encoding/json"
	"math/big"
	"strings"

	bc "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/BlockchainComponent"
)

const NATIVE = "lqd"

type StrategyVault struct{}

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

func (v *StrategyVault) setShare(ctx *bc.Context, addr string, amt *big.Int) {
	ctx.Set("share:"+norm(addr), amt.String())
}

func (v *StrategyVault) setPosition(ctx *bc.Context, pair string, amt *big.Int) {
	ctx.Set("position:"+norm(pair), amt.String())
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

func (v *StrategyVault) routePairs(ctx *bc.Context, tokenIn string, tokenOut string) []string {
	router := norm(ctx.Get("router"))
	if router == "" {
		ctx.Revert("router required for cross-asset movement")
	}
	res, err := ctx.Call(router, "GetBestRoute", []string{norm(tokenIn), norm(tokenOut)})
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
	if len(out) == 0 || len(out) > 2 {
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
	route := v.routePairs(ctx, tokenIn, tokenOut)
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
	ctx.Commit()
	ctx.Emit("VaultInitialized", map[string]interface{}{
		"router":  norm(router),
		"manager": manager,
	})
}

// DepositLP transfers LP tokens into the vault and mints 1:1 vault shares.
// User must approve this vault on the pair contract first.
func (v *StrategyVault) DepositLP(ctx *bc.Context, pair string, amount string) {
	pair = norm(pair)
	user := actorAddr(ctx)
	amt := parseBig(amount)
	requirePositive(ctx, "LP amount", amt)
	if pair == "" || user == "" {
		ctx.Revert("invalid deposit")
	}

	if _, err := ctx.Call(pair, "TransferFrom", []string{user, ctx.ContractAddr, amt.String()}); err != nil {
		ctx.Revert("LP transfer failed: " + err.Error())
	}

	v.setShare(ctx, user, new(big.Int).Add(v.shareOf(ctx, user), amt))
	ctx.Set("totalShares", new(big.Int).Add(v.totalShares(ctx), amt).String())
	v.setPosition(ctx, pair, new(big.Int).Add(v.positionOf(ctx, pair), amt))
	if ctx.Get("current_pair") == "" {
		ctx.Set("current_pair", pair)
	}
	ctx.Set("output", amt.String())
	ctx.Commit()
	ctx.Emit("VaultDeposit", map[string]interface{}{
		"user": user, "pair": pair, "lpAmount": amt.String(), "shares": amt.String(),
	})
}

// WithdrawLP returns the user's pro-rata LP tokens from a selected pool.
func (v *StrategyVault) WithdrawLP(ctx *bc.Context, pair string, shares string) {
	pair = norm(pair)
	user := actorAddr(ctx)
	amt := parseBig(shares)
	requirePositive(ctx, "shares", amt)
	if v.shareOf(ctx, user).Cmp(amt) < 0 {
		ctx.Revert("insufficient vault shares")
	}
	lpAmt := v.redeemLPAmount(ctx, pair, amt)

	if _, err := ctx.Call(pair, "Transfer", []string{user, lpAmt.String()}); err != nil {
		ctx.Revert("LP withdraw failed: " + err.Error())
	}

	v.setShare(ctx, user, new(big.Int).Sub(v.shareOf(ctx, user), amt))
	ctx.Set("totalShares", new(big.Int).Sub(v.totalShares(ctx), amt).String())
	v.setPosition(ctx, pair, new(big.Int).Sub(v.positionOf(ctx, pair), lpAmt))
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
	requirePositive(ctx, "shares", amt)
	if v.shareOf(ctx, user).Cmp(amt) < 0 {
		ctx.Revert("insufficient vault shares")
	}
	lpAmt := v.redeemLPAmount(ctx, pair, amt)

	res, err := ctx.Call(pair, "RemoveLiquidityTo", []string{user, lpAmt.String()})
	if err != nil || !res.Success {
		ctx.Revert("remove liquidity failed")
	}

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
	fromPair = norm(fromPair)
	toPair = norm(toPair)
	amt := parseBig(lpAmount)
	requirePositive(ctx, "LP amount", amt)
	if fromPair == "" || toPair == "" || fromPair == toPair {
		ctx.Revert("invalid rebalance pair")
	}
	if v.positionOf(ctx, fromPair).Cmp(amt) < 0 {
		ctx.Revert("insufficient vault source position")
	}
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
			target0 = new(big.Int).Add(target0, v.swapVaultToken(ctx, token, to0, amount, big.NewInt(0)))
			continue
		}
		if target1.Sign() == 0 {
			target1 = new(big.Int).Add(target1, v.swapVaultToken(ctx, token, to1, amount, big.NewInt(0)))
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
	if v.positionOf(ctx, fromPair).Cmp(amt) < 0 {
		ctx.Revert("insufficient vault source position")
	}
	if v.sameTokenSet(ctx, fromPair, toPair) {
		v.rebalanceSameAsset(ctx, fromPair, toPair, amt)
		return
	}
	v.rebalanceCrossAsset(ctx, fromPair, toPair, amt, minOut)
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
		score := new(big.Int).Mul(depth, weight)
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

func (v *StrategyVault) GetVaultInfo(ctx *bc.Context, user string) {
	user = norm(user)
	data := map[string]string{
		"manager":        norm(ctx.Get("manager")),
		"router":         norm(ctx.Get("router")),
		"current_pair":   norm(ctx.Get("current_pair")),
		"last_rebalance": ctx.Get("last_rebalance"),
		"total_shares":   v.totalShares(ctx).String(),
		"user_shares":    v.shareOf(ctx, user).String(),
	}
	encoded, _ := json.Marshal(data)
	ctx.Set("output", string(encoded))
}

var Contract = &StrategyVault{}
