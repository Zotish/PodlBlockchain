//go:build ignore

package main

import (
	"math/big"
	"strings"
	"time"

	bc "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/BlockchainComponent"
)

// ─────────────────────────────────────────────────────────────────────────────
// LQD DEX Pair Contract  (Uniswap v2 style — per-pair deployed contract)
//
// Each pair has its own address and storage.  The factory deploys one of these
// for every (token0, token1) combination.  Use "lqd" as a token address for
// the native LQD coin.
//
// Storage keys:
//   token0, token1        — the two sorted token addresses
//   factory               — factory that deployed this pair
//   reserve0, reserve1    — AMM reserves
//   totalLP               — total LP supply
//   lp:{addr}             — LP balance per address
//   vlp:{addr}            — validator locked LP
//   vlu:{addr}            — validator lock-until unix timestamp
// ─────────────────────────────────────────────────────────────────────────────

const NATIVE = "lqd"
const minLiquidity = int64(1000)
const protocolRevenueEscrow = "0x0000000000000000000000000000000000000e01"

type Pair struct{}

// ─── Math helpers ────────────────────────────────────────────────────────────

func parseBig(v string) *big.Int {
	v = strings.TrimSpace(v)
	z := new(big.Int)
	if v == "" {
		return z
	}
	z.SetString(v, 10)
	return z
}

func isNative(addr string) bool { return addr == NATIVE }

func sqrtBig(n *big.Int) *big.Int {
	if n.Sign() <= 0 {
		return big.NewInt(0)
	}
	x := new(big.Int).Set(n)
	z := new(big.Int).Add(new(big.Int).Rsh(n, 1), big.NewInt(1))
	for z.Cmp(x) < 0 {
		x.Set(z)
		z = new(big.Int).Rsh(new(big.Int).Add(new(big.Int).Div(n, z), z), 1)
	}
	return x
}

func minBig(a, b *big.Int) *big.Int {
	if a.Cmp(b) < 0 {
		return a
	}
	return b
}

func actorAddr(ctx *bc.Context) string {
	actor := strings.TrimSpace(ctx.OriginAddr)
	if actor == "" {
		actor = ctx.CallerAddr
	}
	return strings.ToLower(actor)
}

func calcAmountOut(amtIn, resIn, resOut *big.Int) *big.Int {
	return calcAmountOutWithFee(amtIn, resIn, resOut, 30)
}

func calcAmountOutWithFee(amtIn, resIn, resOut *big.Int, feeBPS int64) *big.Int {
	if amtIn.Sign() == 0 || resIn.Sign() == 0 || resOut.Sign() == 0 {
		return big.NewInt(0)
	}
	if feeBPS < 1 {
		feeBPS = 1
	}
	if feeBPS > 100 {
		feeBPS = 100
	}
	feeAdjusted := new(big.Int).Mul(amtIn, big.NewInt(10000-feeBPS))
	num := new(big.Int).Mul(feeAdjusted, resOut)
	den := new(big.Int).Add(new(big.Int).Mul(resIn, big.NewInt(10000)), feeAdjusted)
	return new(big.Int).Div(num, den)
}

func dynamicFeeBPS(ctx *bc.Context, amtIn, resIn *big.Int) int64 {
	fee := int64(30)
	if resIn.Sign() > 0 {
		utilBPS := new(big.Int).Div(new(big.Int).Mul(amtIn, big.NewInt(10000)), resIn).Int64()
		if utilBPS > 100 {
			fee += (utilBPS - 100) / 100
		}
	}
	if weight := parseBig(ctx.Get("routing_weight")).Int64(); weight > 0 && weight < 25 {
		fee += 10
	}
	if fee > 100 {
		fee = 100
	}
	return fee
}

func (p *Pair) updateTWAPAccumulator(ctx *bc.Context, reserve0, reserve1 *big.Int) {
	now := ctx.BlockTime
	last := parseBig(ctx.Get("twap_last_timestamp")).Int64()
	if last == 0 {
		ctx.Set("twap_last_timestamp", big.NewInt(now).String())
		return
	}
	if now <= last || reserve0.Sign() <= 0 || reserve1.Sign() <= 0 {
		return
	}
	elapsed := big.NewInt(now - last)
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	price0 := new(big.Int).Div(new(big.Int).Mul(reserve1, scale), reserve0)
	price1 := new(big.Int).Div(new(big.Int).Mul(reserve0, scale), reserve1)
	c0 := new(big.Int).Add(parseBig(ctx.Get("price0_cumulative_x18")), new(big.Int).Mul(price0, elapsed))
	c1 := new(big.Int).Add(parseBig(ctx.Get("price1_cumulative_x18")), new(big.Int).Mul(price1, elapsed))
	ctx.Set("price0_cumulative_x18", c0.String())
	ctx.Set("price1_cumulative_x18", c1.String())
	ctx.Set("twap_last_timestamp", big.NewInt(now).String())
}

func (p *Pair) recordOrganicFlow(ctx *bc.Context, actor string, amount, totalReserve *big.Int) {
	epoch := ctx.Get("flow_epoch")
	if epoch == "" {
		epoch = "0"
	}
	key := "flow:" + epoch + ":" + strings.ToLower(actor)
	prior := parseBig(ctx.Get(key))
	// One identity can contribute at most 5% of epoch-start-like reserves to
	// organic demand. Repeated self-trading therefore has sharply diminishing
	// consensus credit and always pays the AMM fee.
	capPerActor := new(big.Int).Div(totalReserve, big.NewInt(20))
	remaining := new(big.Int).Sub(capPerActor, prior)
	credit := new(big.Int).Set(amount)
	if remaining.Sign() <= 0 {
		credit.SetInt64(0)
	} else if credit.Cmp(remaining) > 0 {
		credit.Set(remaining)
	}
	if prior.Sign() == 0 && credit.Sign() > 0 {
		ctx.Set("epoch_unique_traders", new(big.Int).Add(parseBig(ctx.Get("epoch_unique_traders")), big.NewInt(1)).String())
	}
	ctx.Set(key, new(big.Int).Add(prior, credit).String())
	organic := new(big.Int).Add(parseBig(ctx.Get("epoch_organic_volume")), credit)
	ctx.Set("epoch_organic_volume", organic.String())
	total := new(big.Int).Add(parseBig(ctx.Get("epoch_volume")), amount)
	if total.Sign() > 0 {
		ctx.Set("unique_flow_bps", new(big.Int).Div(new(big.Int).Mul(organic, big.NewInt(10000)), total).String())
	}
}

func calcAmountIn(amtOut, resIn, resOut *big.Int) *big.Int {
	if amtOut.Sign() == 0 || resIn.Sign() == 0 || resOut.Sign() == 0 || amtOut.Cmp(resOut) >= 0 {
		return big.NewInt(0)
	}
	num := new(big.Int).Mul(new(big.Int).Mul(resIn, amtOut), big.NewInt(1000))
	den := new(big.Int).Mul(new(big.Int).Sub(resOut, amtOut), big.NewInt(997))
	return new(big.Int).Add(new(big.Int).Div(num, den), big.NewInt(1))
}

func calcPriceImpactBps(amtIn, amtOut, resIn, resOut *big.Int) *big.Int {
	if amtIn.Sign() <= 0 || amtOut.Sign() <= 0 || resIn.Sign() <= 0 || resOut.Sign() <= 0 {
		return big.NewInt(0)
	}
	spotOut := new(big.Int).Div(new(big.Int).Mul(amtIn, resOut), resIn)
	if spotOut.Sign() <= 0 || spotOut.Cmp(amtOut) <= 0 {
		return big.NewInt(0)
	}
	return new(big.Int).Div(new(big.Int).Mul(new(big.Int).Sub(spotOut, amtOut), big.NewInt(10000)), spotOut)
}

func requireDeadline(ctx *bc.Context, deadline string) {
	dl := parseBig(deadline)
	if dl.Sign() <= 0 {
		ctx.Revert("deadline must be > 0")
	}
	if ctx.BlockTime > dl.Int64() {
		ctx.Revert("transaction expired")
	}
}

// ─── Token transfer helpers ───────────────────────────────────────────────────

func (p *Pair) pullToken(ctx *bc.Context, token, from string, amt *big.Int) {
	if isNative(token) {
		ctx.ReceiveNative(amt)
		return
	}
	if _, err := ctx.Call(token, "TransferFrom", []string{from, ctx.ContractAddr, amt.String()}); err != nil {
		ctx.Revert("TransferFrom failed: " + err.Error())
	}
}

func (p *Pair) pullTokenFromCallerBalance(ctx *bc.Context, token string, amt *big.Int) {
	if isNative(token) {
		ctx.ReceiveNativeFromCaller(amt)
		return
	}
	from := strings.ToLower(strings.TrimSpace(ctx.CallerAddr))
	if _, err := ctx.Call(token, "TransferFrom", []string{from, ctx.ContractAddr, amt.String()}); err != nil {
		ctx.Revert("TransferFrom caller balance failed: " + err.Error())
	}
}

func (p *Pair) pushToken(ctx *bc.Context, token, to string, amt *big.Int) {
	if isNative(token) {
		ctx.SendNative(to, amt)
		return
	}
	if _, err := ctx.Call(token, "Transfer", []string{to, amt.String()}); err != nil {
		ctx.Revert("Transfer failed: " + err.Error())
	}
}

// ─── LP token helpers ─────────────────────────────────────────────────────────

func (p *Pair) lpBalance(ctx *bc.Context, addr string) *big.Int {
	return parseBig(ctx.Get("lp:" + strings.ToLower(addr)))
}

func (p *Pair) lpAllowance(ctx *bc.Context, owner, spender string) *big.Int {
	return parseBig(ctx.Get("lp_allow:" + strings.ToLower(owner) + ":" + strings.ToLower(spender)))
}

func (p *Pair) mintLP(ctx *bc.Context, to string, amt *big.Int) {
	total := parseBig(ctx.Get("totalLP"))
	bal := p.lpBalance(ctx, to)
	ctx.Set("totalLP", new(big.Int).Add(total, amt).String())
	ctx.Set("lp:"+strings.ToLower(to), new(big.Int).Add(bal, amt).String())
}

func (p *Pair) burnLP(ctx *bc.Context, from string, amt *big.Int) {
	total := parseBig(ctx.Get("totalLP"))
	bal := p.lpBalance(ctx, from)
	if bal.Cmp(amt) < 0 {
		ctx.Revert("insufficient LP balance")
	}
	ctx.Set("totalLP", new(big.Int).Sub(total, amt).String())
	ctx.Set("lp:"+strings.ToLower(from), new(big.Int).Sub(bal, amt).String())
}

func (p *Pair) moveLP(ctx *bc.Context, from, to string, amt *big.Int) {
	from = strings.ToLower(strings.TrimSpace(from))
	to = strings.ToLower(strings.TrimSpace(to))
	if amt == nil || amt.Sign() <= 0 {
		ctx.Revert("LP amount must be > 0")
	}
	if from == "" {
		ctx.Revert("invalid LP sender")
	}
	if to == "" {
		ctx.Revert("invalid LP recipient")
	}
	bal := p.lpBalance(ctx, from)
	if bal.Cmp(amt) < 0 {
		ctx.Revert("insufficient LP balance")
	}
	ctx.Set("lp:"+from, new(big.Int).Sub(bal, amt).String())
	ctx.Set("lp:"+to, new(big.Int).Add(p.lpBalance(ctx, to), amt).String())
}

// ─── Init ─────────────────────────────────────────────────────────────────────

// Init is called by the factory immediately after deploying this pair.
// factory = factory contract address, t0/t1 = sorted token addresses.
func (p *Pair) Init(ctx *bc.Context, factory string, token0 string, token1 string) {
	if ctx.Get("token0") != "" {
		ctx.Revert("already initialized")
	}
	t0 := strings.ToLower(strings.TrimSpace(token0))
	t1 := strings.ToLower(strings.TrimSpace(token1))
	if t0 == "" || t1 == "" || t0 == t1 {
		ctx.Revert("invalid token addresses")
	}
	ctx.Set("factory", strings.ToLower(strings.TrimSpace(factory)))
	ctx.Set("token0", t0)
	ctx.Set("token1", t1)
	ctx.Set("reserve0", "0")
	ctx.Set("reserve1", "0")
	ctx.Set("totalLP", "0")
	ctx.Set("protocol_fee_bps", "5")
	ctx.Set("protocol_fee_collector", protocolRevenueEscrow)
	ctx.Commit()

	ctx.Emit("PairInitialized", map[string]interface{}{
		"factory": factory,
		"token0":  t0,
		"token1":  t1,
	})
}

func (p *Pair) collectProtocolFee(ctx *bc.Context, token string, amountIn *big.Int, totalFeeBPS int64) (*big.Int, int64) {
	protocolBPS := parseBig(ctx.Get("protocol_fee_bps")).Int64()
	if protocolBPS < 0 {
		protocolBPS = 0
	}
	if protocolBPS > totalFeeBPS {
		protocolBPS = totalFeeBPS
	}
	fee := new(big.Int).Div(new(big.Int).Mul(amountIn, big.NewInt(protocolBPS)), big.NewInt(10000))
	if fee.Sign() > 0 {
		collector := strings.ToLower(strings.TrimSpace(ctx.Get("protocol_fee_collector")))
		if collector == "" {
			collector = protocolRevenueEscrow
		}
		p.pushToken(ctx, token, collector, fee)
		key := "protocol_fee_total:" + strings.ToLower(token)
		ctx.Set(key, new(big.Int).Add(parseBig(ctx.Get(key)), fee).String())
	}
	return fee, protocolBPS
}

// SetProtocolFee is factory-governed and capped at half of the minimum base
// fee so LPs always receive the majority of swap fees.
func (p *Pair) SetProtocolFee(ctx *bc.Context, feeBPS string, collector string) {
	if !strings.EqualFold(ctx.CallerAddr, ctx.Get("factory")) && !strings.EqualFold(ctx.OriginAddr, ctx.Get("factory")) {
		ctx.Revert("factory only")
	}
	fee := parseBig(feeBPS)
	collector = strings.ToLower(strings.TrimSpace(collector))
	if !fee.IsInt64() || fee.Int64() < 0 || fee.Int64() > 15 || collector == "" {
		ctx.Revert("protocol fee must be 0..15 bps with collector")
	}
	ctx.Set("protocol_fee_bps", fee.String())
	ctx.Set("protocol_fee_collector", collector)
	ctx.Emit("ProtocolFeePolicyUpdated", map[string]interface{}{"feeBps": fee.String(), "collector": collector})
}

// ─── AddLiquidity ─────────────────────────────────────────────────────────────

func (p *Pair) AddLiquidity(ctx *bc.Context, amountA string, amountB string) {
	t0 := ctx.Get("token0")
	t1 := ctx.Get("token1")
	if t0 == "" {
		ctx.Revert("pair not initialized")
	}

	amtA := parseBig(amountA)
	amtB := parseBig(amountB)
	if amtA.Sign() == 0 || amtB.Sign() == 0 {
		ctx.Revert("amounts must be > 0")
	}

	caller := actorAddr(ctx)

	p.pullToken(ctx, t0, caller, amtA)
	p.pullToken(ctx, t1, caller, amtB)

	res0 := parseBig(ctx.Get("reserve0"))
	res1 := parseBig(ctx.Get("reserve1"))
	totalLP := parseBig(ctx.Get("totalLP"))

	var minted *big.Int
	minLiq := big.NewInt(minLiquidity)

	if res0.Sign() == 0 && res1.Sign() == 0 {
		sqrtLP := sqrtBig(new(big.Int).Mul(amtA, amtB))
		if sqrtLP.Cmp(minLiq) <= 0 {
			ctx.Revert("initial liquidity too small")
		}
		minted = new(big.Int).Sub(sqrtLP, minLiq)
		// Burn MINIMUM_LIQUIDITY to zero address (Uniswap v2 style)
		p.mintLP(ctx, "0x0000000000000000000000000000000000000000", minLiq)
	} else {
		lpFromA := new(big.Int).Div(new(big.Int).Mul(amtA, totalLP), res0)
		lpFromB := new(big.Int).Div(new(big.Int).Mul(amtB, totalLP), res1)
		minted = minBig(lpFromA, lpFromB)
	}
	if minted.Sign() == 0 {
		ctx.Revert("zero LP minted")
	}

	p.mintLP(ctx, caller, minted)
	ctx.Set("reserve0", new(big.Int).Add(res0, amtA).String())
	ctx.Set("reserve1", new(big.Int).Add(res1, amtB).String())
	ctx.Commit()

	ctx.Emit("Mint", map[string]interface{}{
		"sender":   caller,
		"amount0":  amtA.String(),
		"amount1":  amtB.String(),
		"lpMinted": minted.String(),
	})
	ctx.Set("output", minted.String())
}

func (p *Pair) AddLiquidityFromContract(ctx *bc.Context, receiver string, amountA string, amountB string) {
	t0 := ctx.Get("token0")
	t1 := ctx.Get("token1")
	if t0 == "" {
		ctx.Revert("pair not initialized")
	}

	receiver = strings.ToLower(strings.TrimSpace(receiver))
	if receiver == "" {
		ctx.Revert("invalid receiver")
	}
	amtA := parseBig(amountA)
	amtB := parseBig(amountB)
	if amtA.Sign() == 0 || amtB.Sign() == 0 {
		ctx.Revert("amounts must be > 0")
	}

	p.pullTokenFromCallerBalance(ctx, t0, amtA)
	p.pullTokenFromCallerBalance(ctx, t1, amtB)

	res0 := parseBig(ctx.Get("reserve0"))
	res1 := parseBig(ctx.Get("reserve1"))
	totalLP := parseBig(ctx.Get("totalLP"))

	var minted *big.Int
	minLiq := big.NewInt(minLiquidity)
	if res0.Sign() == 0 && res1.Sign() == 0 {
		sqrtLP := sqrtBig(new(big.Int).Mul(amtA, amtB))
		if sqrtLP.Cmp(minLiq) <= 0 {
			ctx.Revert("initial liquidity too small")
		}
		minted = new(big.Int).Sub(sqrtLP, minLiq)
		p.mintLP(ctx, "0x0000000000000000000000000000000000000000", minLiq)
	} else {
		lpFromA := new(big.Int).Div(new(big.Int).Mul(amtA, totalLP), res0)
		lpFromB := new(big.Int).Div(new(big.Int).Mul(amtB, totalLP), res1)
		minted = minBig(lpFromA, lpFromB)
	}
	if minted.Sign() == 0 {
		ctx.Revert("zero LP minted")
	}

	p.mintLP(ctx, receiver, minted)
	ctx.Set("reserve0", new(big.Int).Add(res0, amtA).String())
	ctx.Set("reserve1", new(big.Int).Add(res1, amtB).String())
	ctx.Set("output", minted.String())
	ctx.Commit()
	ctx.Emit("Mint", map[string]interface{}{
		"sender":   strings.ToLower(ctx.CallerAddr),
		"receiver": receiver,
		"amount0":  amtA.String(),
		"amount1":  amtB.String(),
		"lpMinted": minted.String(),
	})
}

// ─── RemoveLiquidity ──────────────────────────────────────────────────────────

func (p *Pair) RemoveLiquidity(ctx *bc.Context, lpAmount string) {
	t0 := ctx.Get("token0")
	t1 := ctx.Get("token1")
	if t0 == "" {
		ctx.Revert("pair not initialized")
	}

	lpAmt := parseBig(lpAmount)
	if lpAmt.Sign() == 0 {
		ctx.Revert("LP amount must be > 0")
	}

	res0 := parseBig(ctx.Get("reserve0"))
	res1 := parseBig(ctx.Get("reserve1"))
	totalLP := parseBig(ctx.Get("totalLP"))
	if totalLP.Sign() == 0 {
		ctx.Revert("no liquidity")
	}

	out0 := new(big.Int).Div(new(big.Int).Mul(lpAmt, res0), totalLP)
	out1 := new(big.Int).Div(new(big.Int).Mul(lpAmt, res1), totalLP)
	if out0.Sign() == 0 && out1.Sign() == 0 {
		ctx.Revert("insufficient output")
	}

	caller := actorAddr(ctx)
	p.burnLP(ctx, caller, lpAmt)
	ctx.Set("reserve0", new(big.Int).Sub(res0, out0).String())
	ctx.Set("reserve1", new(big.Int).Sub(res1, out1).String())
	ctx.Commit()

	p.pushToken(ctx, t0, caller, out0)
	p.pushToken(ctx, t1, caller, out1)

	ctx.Emit("Burn", map[string]interface{}{
		"sender":  caller,
		"amount0": out0.String(),
		"amount1": out1.String(),
		"lpBurnt": lpAmt.String(),
	})
	ctx.Set("output", out0.String()+","+out1.String())
}

func (p *Pair) RemoveLiquidityTo(ctx *bc.Context, receiver string, lpAmount string) {
	t0 := ctx.Get("token0")
	t1 := ctx.Get("token1")
	if t0 == "" {
		ctx.Revert("pair not initialized")
	}
	receiver = strings.ToLower(strings.TrimSpace(receiver))
	if receiver == "" {
		ctx.Revert("invalid receiver")
	}

	lpAmt := parseBig(lpAmount)
	if lpAmt.Sign() == 0 {
		ctx.Revert("LP amount must be > 0")
	}
	res0 := parseBig(ctx.Get("reserve0"))
	res1 := parseBig(ctx.Get("reserve1"))
	totalLP := parseBig(ctx.Get("totalLP"))
	if totalLP.Sign() == 0 {
		ctx.Revert("no liquidity")
	}

	out0 := new(big.Int).Div(new(big.Int).Mul(lpAmt, res0), totalLP)
	out1 := new(big.Int).Div(new(big.Int).Mul(lpAmt, res1), totalLP)
	if out0.Sign() == 0 && out1.Sign() == 0 {
		ctx.Revert("insufficient output")
	}

	from := strings.ToLower(strings.TrimSpace(ctx.CallerAddr))
	p.burnLP(ctx, from, lpAmt)
	ctx.Set("reserve0", new(big.Int).Sub(res0, out0).String())
	ctx.Set("reserve1", new(big.Int).Sub(res1, out1).String())
	ctx.Set("output", out0.String()+","+out1.String())
	ctx.Commit()

	p.pushToken(ctx, t0, receiver, out0)
	p.pushToken(ctx, t1, receiver, out1)
	ctx.Emit("Burn", map[string]interface{}{
		"sender":   from,
		"receiver": receiver,
		"amount0":  out0.String(),
		"amount1":  out1.String(),
		"lpBurnt":  lpAmt.String(),
	})
}

// ─── Swap ─────────────────────────────────────────────────────────────────────

func (p *Pair) swapTo(ctx *bc.Context, receiver string, amountIn string, minAmountOut string, tokenIn string, pullFromActor bool) *big.Int {
	t0 := ctx.Get("token0")
	t1 := ctx.Get("token1")
	if t0 == "" {
		ctx.Revert("pair not initialized")
	}
	receiver = strings.ToLower(strings.TrimSpace(receiver))
	if receiver == "" {
		ctx.Revert("invalid receiver")
	}

	tIn := strings.ToLower(strings.TrimSpace(tokenIn))
	if tIn != t0 && tIn != t1 {
		ctx.Revert("tokenIn not in pair")
	}

	amtIn := parseBig(amountIn)
	minOut := parseBig(minAmountOut)
	if amtIn.Sign() == 0 {
		ctx.Revert("amountIn must be > 0")
	}

	caller := actorAddr(ctx)

	res0 := parseBig(ctx.Get("reserve0"))
	res1 := parseBig(ctx.Get("reserve1"))

	var resIn, resOut *big.Int
	var tOut string
	if tIn == t0 {
		resIn, resOut, tOut = res0, res1, t1
	} else {
		resIn, resOut, tOut = res1, res0, t0
	}

	feeBPS := dynamicFeeBPS(ctx, amtIn, resIn)
	amtOut := calcAmountOutWithFee(amtIn, resIn, resOut, feeBPS)
	if amtOut.Cmp(minOut) < 0 {
		ctx.Revert("slippage: insufficient output amount")
	}
	if amtOut.Cmp(resOut) >= 0 {
		ctx.Revert("insufficient reserves")
	}

	if pullFromActor {
		p.pullToken(ctx, tIn, caller, amtIn)
	} else {
		p.pullTokenFromCallerBalance(ctx, tIn, amtIn)
	}
	protocolFee, protocolBPS := p.collectProtocolFee(ctx, tIn, amtIn, feeBPS)

	p.updateTWAPAccumulator(ctx, res0, res1)
	newResIn := new(big.Int).Add(resIn, new(big.Int).Sub(amtIn, protocolFee))
	newResOut := new(big.Int).Sub(resOut, amtOut)

	// k-invariant check (1000x to account for fee)
	lhs := new(big.Int).Mul(
		new(big.Int).Sub(new(big.Int).Mul(newResIn, big.NewInt(10000)), new(big.Int).Mul(amtIn, big.NewInt(feeBPS-protocolBPS))),
		new(big.Int).Mul(newResOut, big.NewInt(10000)),
	)
	rhs := new(big.Int).Mul(
		new(big.Int).Mul(resIn, big.NewInt(10000)),
		new(big.Int).Mul(resOut, big.NewInt(10000)),
	)
	if lhs.Cmp(rhs) < 0 {
		ctx.Revert("k-invariant violated")
	}

	if tIn == t0 {
		ctx.Set("reserve0", newResIn.String())
		ctx.Set("reserve1", newResOut.String())
	} else {
		ctx.Set("reserve1", newResIn.String())
		ctx.Set("reserve0", newResOut.String())
	}

	// ── Dynamic Liquidity Engine metrics (tracked per epoch) ─────────────────
	epochSwaps := new(big.Int).Add(parseBig(ctx.Get("epoch_swaps")), big.NewInt(1))
	epochVol := new(big.Int).Add(parseBig(ctx.Get("epoch_volume")), amtIn)
	ctx.Set("epoch_swaps", epochSwaps.String())
	p.recordOrganicFlow(ctx, caller, amtIn, new(big.Int).Add(res0, res1))
	ctx.Set("epoch_volume", epochVol.String())
	// ─────────────────────────────────────────────────────────────────────────

	ctx.Commit()

	p.pushToken(ctx, tOut, receiver, amtOut)

	ctx.Emit("Swap", map[string]interface{}{
		"sender":         caller,
		"receiver":       receiver,
		"tokenIn":        tIn,
		"tokenOut":       tOut,
		"amountIn":       amtIn.String(),
		"amountOut":      amtOut.String(),
		"priceImpactBps": calcPriceImpactBps(amtIn, amtOut, resIn, resOut).String(),
		"feeBps":         feeBPS,
		"protocolFee":    protocolFee.String(),
	})
	ctx.Set("output", amtOut.String())
	return amtOut
}

// Swap: send amountIn of tokenIn, receive at least minAmountOut of the other token.
func (p *Pair) Swap(ctx *bc.Context, amountIn string, minAmountOut string, tokenIn string) {
	p.swapTo(ctx, actorAddr(ctx), amountIn, minAmountOut, tokenIn, true)
}

func (p *Pair) SwapWithDeadline(ctx *bc.Context, amountIn string, minAmountOut string, tokenIn string, deadline string) {
	requireDeadline(ctx, deadline)
	p.Swap(ctx, amountIn, minAmountOut, tokenIn)
}

func (p *Pair) SwapTo(ctx *bc.Context, receiver string, amountIn string, minAmountOut string, tokenIn string) {
	p.swapTo(ctx, receiver, amountIn, minAmountOut, tokenIn, true)
}

func (p *Pair) SwapToWithDeadline(ctx *bc.Context, receiver string, amountIn string, minAmountOut string, tokenIn string, deadline string) {
	requireDeadline(ctx, deadline)
	p.SwapTo(ctx, receiver, amountIn, minAmountOut, tokenIn)
}

// SwapFromContract swaps tokens held by the calling contract and sends output
// to receiver. Strategy/vault contracts use this for routed liquidity movement.
func (p *Pair) SwapFromContract(ctx *bc.Context, receiver string, amountIn string, minAmountOut string, tokenIn string) {
	t0 := ctx.Get("token0")
	t1 := ctx.Get("token1")
	if t0 == "" {
		ctx.Revert("pair not initialized")
	}

	receiver = strings.ToLower(strings.TrimSpace(receiver))
	if receiver == "" {
		ctx.Revert("invalid receiver")
	}

	tIn := strings.ToLower(strings.TrimSpace(tokenIn))
	if tIn != t0 && tIn != t1 {
		ctx.Revert("tokenIn not in pair")
	}

	amtIn := parseBig(amountIn)
	minOut := parseBig(minAmountOut)
	if amtIn.Sign() == 0 {
		ctx.Revert("amountIn must be > 0")
	}

	res0 := parseBig(ctx.Get("reserve0"))
	res1 := parseBig(ctx.Get("reserve1"))

	var resIn, resOut *big.Int
	var tOut string
	if tIn == t0 {
		resIn, resOut, tOut = res0, res1, t1
	} else {
		resIn, resOut, tOut = res1, res0, t0
	}

	feeBPS := dynamicFeeBPS(ctx, amtIn, resIn)
	amtOut := calcAmountOutWithFee(amtIn, resIn, resOut, feeBPS)
	if amtOut.Cmp(minOut) < 0 {
		ctx.Revert("slippage: insufficient output amount")
	}
	if amtOut.Cmp(resOut) >= 0 {
		ctx.Revert("insufficient reserves")
	}

	p.pullTokenFromCallerBalance(ctx, tIn, amtIn)
	protocolFee, protocolBPS := p.collectProtocolFee(ctx, tIn, amtIn, feeBPS)

	p.updateTWAPAccumulator(ctx, res0, res1)
	newResIn := new(big.Int).Add(resIn, new(big.Int).Sub(amtIn, protocolFee))
	newResOut := new(big.Int).Sub(resOut, amtOut)
	lhs := new(big.Int).Mul(
		new(big.Int).Sub(new(big.Int).Mul(newResIn, big.NewInt(10000)), new(big.Int).Mul(amtIn, big.NewInt(feeBPS-protocolBPS))),
		new(big.Int).Mul(newResOut, big.NewInt(10000)),
	)
	rhs := new(big.Int).Mul(
		new(big.Int).Mul(resIn, big.NewInt(10000)),
		new(big.Int).Mul(resOut, big.NewInt(10000)),
	)
	if lhs.Cmp(rhs) < 0 {
		ctx.Revert("k-invariant violated")
	}

	if tIn == t0 {
		ctx.Set("reserve0", newResIn.String())
		ctx.Set("reserve1", newResOut.String())
	} else {
		ctx.Set("reserve1", newResIn.String())
		ctx.Set("reserve0", newResOut.String())
	}

	epochSwaps := new(big.Int).Add(parseBig(ctx.Get("epoch_swaps")), big.NewInt(1))
	epochVol := new(big.Int).Add(parseBig(ctx.Get("epoch_volume")), amtIn)
	ctx.Set("epoch_swaps", epochSwaps.String())
	p.recordOrganicFlow(ctx, actorAddr(ctx), amtIn, new(big.Int).Add(res0, res1))
	ctx.Set("epoch_volume", epochVol.String())
	ctx.Set("output", amtOut.String())
	ctx.Commit()

	p.pushToken(ctx, tOut, receiver, amtOut)
	ctx.Emit("SwapFromContract", map[string]interface{}{
		"sender":         strings.ToLower(strings.TrimSpace(ctx.CallerAddr)),
		"receiver":       receiver,
		"tokenIn":        tIn,
		"tokenOut":       tOut,
		"amountIn":       amtIn.String(),
		"amountOut":      amtOut.String(),
		"priceImpactBps": calcPriceImpactBps(amtIn, amtOut, resIn, resOut).String(),
		"feeBps":         feeBPS,
		"protocolFee":    protocolFee.String(),
	})
}

func (p *Pair) SwapFromContractWithDeadline(ctx *bc.Context, receiver string, amountIn string, minAmountOut string, tokenIn string, deadline string) {
	requireDeadline(ctx, deadline)
	p.SwapFromContract(ctx, receiver, amountIn, minAmountOut, tokenIn)
}

// ─── View helpers ─────────────────────────────────────────────────────────────

func (p *Pair) GetReserves(ctx *bc.Context) {
	t0 := ctx.Get("token0")
	t1 := ctx.Get("token1")
	r0 := ctx.Get("reserve0")
	r1 := ctx.Get("reserve1")
	totalLP := ctx.Get("totalLP")
	ctx.Set("output", r0+","+r1+","+totalLP)
	ctx.Emit("Reserves", map[string]interface{}{
		"token0": t0, "token1": t1,
		"reserve0": r0, "reserve1": r1, "totalLP": totalLP,
	})
}

func (p *Pair) GetInfo(ctx *bc.Context) {
	ctx.Set("output", ctx.ContractAddr)
	ctx.Emit("PairInfo", map[string]interface{}{
		"pair":     ctx.ContractAddr,
		"token0":   ctx.Get("token0"),
		"token1":   ctx.Get("token1"),
		"reserve0": ctx.Get("reserve0"),
		"reserve1": ctx.Get("reserve1"),
		"totalLP":  ctx.Get("totalLP"),
		"factory":  ctx.Get("factory"),
	})
}

func (p *Pair) Token0(ctx *bc.Context) {
	ctx.Set("output", ctx.Get("token0"))
}

func (p *Pair) Token1(ctx *bc.Context) {
	ctx.Set("output", ctx.Get("token1"))
}

// GetRoutingWeight returns this pair's current routing weight (0-100).
// Set by the Dynamic Liquidity Engine every epoch based on swap volume.
// Higher weight = more in-demand = preferred routing target.
func (p *Pair) GetRoutingWeight(ctx *bc.Context) {
	w := ctx.Get("routing_weight")
	if w == "" {
		w = "50" // default mid-weight before DLEngine runs first epoch
	}
	ctx.Set("output", w)
	ctx.Emit("RoutingWeight", map[string]interface{}{
		"pair":   ctx.ContractAddr,
		"weight": w,
	})
}

func (p *Pair) ObserveTWAP(ctx *bc.Context) {
	r0, r1 := parseBig(ctx.Get("reserve0")), parseBig(ctx.Get("reserve1"))
	p.updateTWAPAccumulator(ctx, r0, r1)
	ctx.Set("output", strings.Join([]string{ctx.Get("price0_cumulative_x18"), ctx.Get("price1_cumulative_x18"), ctx.Get("twap_last_timestamp")}, ","))
}

func (p *Pair) GetDynamicFeeBPS(ctx *bc.Context, amountIn string, tokenIn string) {
	t0 := ctx.Get("token0")
	resIn := parseBig(ctx.Get("reserve1"))
	if strings.EqualFold(strings.TrimSpace(tokenIn), t0) {
		resIn = parseBig(ctx.Get("reserve0"))
	}
	ctx.Set("output", big.NewInt(dynamicFeeBPS(ctx, parseBig(amountIn), resIn)).String())
}

func (p *Pair) GetAmountOut(ctx *bc.Context, amountIn string, tokenIn string) {
	t0 := ctx.Get("token0")
	t1 := ctx.Get("token1")
	tIn := strings.ToLower(strings.TrimSpace(tokenIn))
	if tIn != t0 && tIn != t1 {
		ctx.Set("output", "0")
		return
	}
	res0 := parseBig(ctx.Get("reserve0"))
	res1 := parseBig(ctx.Get("reserve1"))

	var resIn, resOut *big.Int
	if tIn == t0 {
		resIn, resOut = res0, res1
	} else {
		resIn, resOut = res1, res0
	}
	out := calcAmountOutWithFee(parseBig(amountIn), resIn, resOut, dynamicFeeBPS(ctx, parseBig(amountIn), resIn))
	ctx.Set("output", out.String())
	ctx.Emit("AmountOut", map[string]interface{}{"amountOut": out.String()})
}

func (p *Pair) GetAmountIn(ctx *bc.Context, amountOut string, tokenOut string) {
	t0 := ctx.Get("token0")
	t1 := ctx.Get("token1")
	tOut := strings.ToLower(strings.TrimSpace(tokenOut))
	if tOut != t0 && tOut != t1 {
		ctx.Set("output", "0")
		return
	}
	res0 := parseBig(ctx.Get("reserve0"))
	res1 := parseBig(ctx.Get("reserve1"))

	var resIn, resOut *big.Int
	if tOut == t1 {
		resIn, resOut = res0, res1
	} else {
		resIn, resOut = res1, res0
	}
	amtIn := calcAmountIn(parseBig(amountOut), resIn, resOut)
	ctx.Set("output", amtIn.String())
	ctx.Emit("AmountIn", map[string]interface{}{"amountIn": amtIn.String()})
}

func (p *Pair) GetQuote(ctx *bc.Context, amountIn string, tokenIn string) {
	t0 := ctx.Get("token0")
	t1 := ctx.Get("token1")
	tIn := strings.ToLower(strings.TrimSpace(tokenIn))
	if tIn != t0 && tIn != t1 {
		ctx.Set("output", "0,0,0,0,0,0,0,0")
		return
	}
	amtIn := parseBig(amountIn)
	res0 := parseBig(ctx.Get("reserve0"))
	res1 := parseBig(ctx.Get("reserve1"))
	var resIn, resOut *big.Int
	var tokenOut string
	if tIn == t0 {
		resIn, resOut, tokenOut = res0, res1, t1
	} else {
		resIn, resOut, tokenOut = res1, res0, t0
	}
	feeBPS := dynamicFeeBPS(ctx, amtIn, resIn)
	amtOut := calcAmountOutWithFee(amtIn, resIn, resOut, feeBPS)
	impact := calcPriceImpactBps(amtIn, amtOut, resIn, resOut)
	spotPriceBps := big.NewInt(0)
	if resIn.Sign() > 0 {
		spotPriceBps.Div(new(big.Int).Mul(resOut, big.NewInt(10000)), resIn)
	}
	execPriceBps := big.NewInt(0)
	if amtIn.Sign() > 0 {
		execPriceBps.Div(new(big.Int).Mul(amtOut, big.NewInt(10000)), amtIn)
	}
	fee := new(big.Int).Div(new(big.Int).Mul(amtIn, big.NewInt(feeBPS)), big.NewInt(10000))
	ctx.Set("output", strings.Join([]string{
		amtOut.String(),
		impact.String(),
		resIn.String(),
		resOut.String(),
		res0.String(),
		res1.String(),
		spotPriceBps.String(),
		execPriceBps.String(),
		fee.String(),
		tokenOut,
	}, ","))
	ctx.Emit("Quote", map[string]interface{}{
		"tokenIn": tokenIn, "tokenOut": tokenOut, "amountIn": amtIn.String(), "amountOut": amtOut.String(),
		"reserveIn": resIn.String(), "reserveOut": resOut.String(), "priceImpactBps": impact.String(),
	})
}

// ─── LP token (ERC20-style) ───────────────────────────────────────────────────

func (p *Pair) BalanceOf(ctx *bc.Context, addr string) {
	bal := p.lpBalance(ctx, addr)
	ctx.Set("output", bal.String())
	ctx.Emit("BalanceOf", map[string]interface{}{"address": addr, "balance": bal.String()})
}

func (p *Pair) TotalSupply(ctx *bc.Context) {
	total := ctx.Get("totalLP")
	ctx.Set("output", total)
	ctx.Emit("TotalSupply", map[string]interface{}{"totalSupply": total})
}

func (p *Pair) Transfer(ctx *bc.Context, to string, amount string) {
	from := strings.ToLower(ctx.CallerAddr)
	to = strings.ToLower(strings.TrimSpace(to))
	amt := parseBig(amount)
	p.moveLP(ctx, from, to, amt)
	ctx.Commit()
	ctx.Emit("Transfer", map[string]interface{}{"from": from, "to": to, "amount": amount})
}

func (p *Pair) Approve(ctx *bc.Context, spender string, amount string) {
	owner := strings.ToLower(strings.TrimSpace(ctx.CallerAddr))
	spender = strings.ToLower(strings.TrimSpace(spender))
	if owner == "" || spender == "" {
		ctx.Revert("invalid approval")
	}
	amt := parseBig(amount)
	if amt.Sign() < 0 {
		ctx.Revert("amount must be >= 0")
	}
	ctx.Set("lp_allow:"+owner+":"+spender, amt.String())
	ctx.Commit()
	ctx.Emit("Approval", map[string]interface{}{"owner": owner, "spender": spender, "amount": amt.String()})
}

func (p *Pair) Allowance(ctx *bc.Context, owner string, spender string) {
	ctx.Set("output", p.lpAllowance(ctx, owner, spender).String())
}

func (p *Pair) TransferFrom(ctx *bc.Context, from string, to string, amount string) {
	spender := strings.ToLower(strings.TrimSpace(ctx.CallerAddr))
	from = strings.ToLower(strings.TrimSpace(from))
	to = strings.ToLower(strings.TrimSpace(to))
	amt := parseBig(amount)
	allowed := p.lpAllowance(ctx, from, spender)
	if allowed.Cmp(amt) < 0 {
		ctx.Revert("allowance exceeded")
	}
	p.moveLP(ctx, from, to, amt)
	ctx.Set("lp_allow:"+from+":"+spender, new(big.Int).Sub(allowed, amt).String())
	ctx.Commit()
	ctx.Emit("Transfer", map[string]interface{}{"from": from, "to": to, "amount": amt.String()})
}

// ─── Proof of Dynamic Liquidity — validator LP locking ───────────────────────

func (p *Pair) LockLPForValidation(ctx *bc.Context, lpAmount string, durationSecs string) {
	caller := actorAddr(ctx)
	lpAmt := parseBig(lpAmount)
	if lpAmt.Sign() == 0 {
		ctx.Revert("LP amount must be > 0")
	}
	bal := p.lpBalance(ctx, caller)
	if bal.Cmp(lpAmt) < 0 {
		ctx.Revert("insufficient LP balance")
	}
	dur := parseBig(durationSecs).Int64()
	if dur <= 0 {
		ctx.Revert("duration must be > 0")
	}
	existing := parseBig(ctx.Get("vlp:" + caller))
	ctx.Set("lp:"+caller, new(big.Int).Sub(bal, lpAmt).String())
	ctx.Set("vlp:"+caller, new(big.Int).Add(existing, lpAmt).String())
	lockUntil := time.Now().Unix() + dur
	ctx.Set("vlu:"+caller, big.NewInt(lockUntil).String())
	ctx.Commit()
	ctx.Emit("LPLocked", map[string]interface{}{
		"validator": caller, "lpAmount": lpAmount, "lockUntil": lockUntil,
	})
}

func (p *Pair) UnlockValidatorLP(ctx *bc.Context) {
	caller := actorAddr(ctx)
	lockUntil := parseBig(ctx.Get("vlu:" + caller)).Int64()
	if time.Now().Unix() < lockUntil {
		ctx.Revert("lock period not expired")
	}
	lockedLP := parseBig(ctx.Get("vlp:" + caller))
	if lockedLP.Sign() == 0 {
		ctx.Revert("no locked LP")
	}
	bal := p.lpBalance(ctx, caller)
	ctx.Set("lp:"+caller, new(big.Int).Add(bal, lockedLP).String())
	ctx.Set("vlp:"+caller, "0")
	ctx.Set("vlu:"+caller, "0")
	ctx.Commit()
	ctx.Emit("LPUnlocked", map[string]interface{}{
		"validator": caller, "lpAmount": lockedLP.String(),
	})
}

func (p *Pair) GetValidatorLP(ctx *bc.Context, addr string) {
	addr = strings.ToLower(strings.TrimSpace(addr))
	locked := ctx.Get("vlp:" + addr)
	until := ctx.Get("vlu:" + addr)
	ctx.Set("output", locked)
	ctx.Emit("ValidatorLP", map[string]interface{}{
		"address": addr, "lockedLP": locked, "lockUntil": until,
	})
}

// Contract is the exported plugin entry-point required by the LQD plugin loader.
var Contract = &Pair{}
