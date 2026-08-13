//go:build ignore
// +build ignore

package main

import (
	"encoding/json"
	"math/big"
	"strings"

	bc "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/BlockchainComponent"
)

const advancedNative = "lqd"

type AdvancedPool struct{}

func apBig(raw string) *big.Int {
	z := new(big.Int)
	if _, ok := z.SetString(strings.TrimSpace(raw), 10); !ok {
		return big.NewInt(0)
	}
	return z
}

func apNorm(raw string) string { return strings.ToLower(strings.TrimSpace(raw)) }

func apSqrt(n *big.Int) *big.Int {
	if n.Sign() <= 0 {
		return big.NewInt(0)
	}
	x, z := new(big.Int).Set(n), new(big.Int).Add(new(big.Int).Rsh(n, 1), big.NewInt(1))
	for z.Cmp(x) < 0 {
		x.Set(z)
		z.Rsh(new(big.Int).Add(new(big.Int).Div(n, z), z), 1)
	}
	return x
}

func apMin(a, b *big.Int) *big.Int {
	if a.Cmp(b) < 0 {
		return a
	}
	return b
}

func (p *AdvancedPool) actor(ctx *bc.Context) string {
	if actor := apNorm(ctx.OriginAddr); actor != "" {
		return actor
	}
	return apNorm(ctx.CallerAddr)
}

func (p *AdvancedPool) pull(ctx *bc.Context, token, from string, amount *big.Int, fromCaller bool) {
	if token == advancedNative {
		if fromCaller {
			ctx.ReceiveNativeFromCaller(amount)
		} else {
			ctx.ReceiveNative(amount)
		}
		return
	}
	if _, err := ctx.Call(token, "TransferFrom", []string{from, ctx.ContractAddr, amount.String()}); err != nil {
		ctx.Revert("token pull failed: " + err.Error())
	}
}

func (p *AdvancedPool) push(ctx *bc.Context, token, to string, amount *big.Int) {
	if token == advancedNative {
		ctx.SendNative(to, amount)
		return
	}
	if _, err := ctx.Call(token, "Transfer", []string{to, amount.String()}); err != nil {
		ctx.Revert("token push failed: " + err.Error())
	}
}

// Init creates either an amplified stable pool or a concentrated range pool.
// parameter is amplification (stable) or initial sqrt-price X18 (concentrated).
func (p *AdvancedPool) Init(ctx *bc.Context, factory, token0, token1, poolType, parameter string) {
	if ctx.Get("token0") != "" {
		ctx.Revert("already initialized")
	}
	t0, t1, kind := apNorm(token0), apNorm(token1), apNorm(poolType)
	if t0 == "" || t1 == "" || t0 == t1 || (kind != "stable" && kind != "concentrated") {
		ctx.Revert("invalid advanced pool")
	}
	param := apBig(parameter)
	if kind == "stable" && (param.Cmp(big.NewInt(1)) < 0 || param.Cmp(big.NewInt(1_000_000)) > 0) {
		ctx.Revert("amplification must be 1..1000000")
	}
	if kind == "concentrated" && param.Sign() <= 0 {
		ctx.Revert("initial sqrt price required")
	}
	ctx.Set("factory", apNorm(factory))
	ctx.Set("token0", t0)
	ctx.Set("token1", t1)
	ctx.Set("pool_type", kind)
	ctx.Set("parameter", param.String())
	ctx.Set("reserve0", "0")
	ctx.Set("reserve1", "0")
	ctx.Set("totalLP", "0")
	ctx.Set("fee_bps", "4")
	ctx.Set("stable_warning_deviation_bps", "100")
	ctx.Set("stable_emergency_deviation_bps", "1000")
	ctx.Set("stable_depeg_surcharge_bps", "50")
	ctx.Set("stable_emergency_max_swap_bps", "100")
	ctx.Set("routing_weight", "50")
	if kind == "concentrated" {
		pool := bc.ConcentratedPool{SqrtPriceX18: param, Liquidity: big.NewInt(0), FeeBPS: 4, Ticks: []bc.ConcentratedTick{}}
		raw, _ := json.Marshal(pool)
		ctx.Set("concentrated_state", string(raw))
	}
	ctx.Commit()
	ctx.Emit("AdvancedPoolInitialized", map[string]interface{}{"type": kind, "token0": t0, "token1": t1, "parameter": parameter})
}

func (p *AdvancedPool) stableFeeAndCap(ctx *bc.Context) (int64, int64, bool) {
	base := apBig(ctx.Get("fee_bps")).Int64()
	p0, p1 := apBig(ctx.Get("oracle_price0_x18")), apBig(ctx.Get("oracle_price1_x18"))
	if p0.Sign() <= 0 || p1.Sign() <= 0 {
		return base, 10000, false
	}
	updated := apBig(ctx.Get("oracle_updated_at")).Int64()
	if updated <= 0 || ctx.BlockTime-updated > 900 {
		return base + apBig(ctx.Get("stable_depeg_surcharge_bps")).Int64(), apBig(ctx.Get("stable_emergency_max_swap_bps")).Int64(), true
	}
	maxP := new(big.Int).Set(p0)
	if p1.Cmp(maxP) > 0 {
		maxP.Set(p1)
	}
	delta := new(big.Int).Sub(p0, p1)
	if delta.Sign() < 0 {
		delta.Neg(delta)
	}
	deviation := new(big.Int).Div(new(big.Int).Mul(delta, big.NewInt(10000)), maxP).Int64()
	fee, capBPS, emergency := base, int64(10000), false
	if deviation >= apBig(ctx.Get("stable_warning_deviation_bps")).Int64() {
		fee += apBig(ctx.Get("stable_depeg_surcharge_bps")).Int64()
	}
	if deviation >= apBig(ctx.Get("stable_emergency_deviation_bps")).Int64() {
		capBPS, emergency = apBig(ctx.Get("stable_emergency_max_swap_bps")).Int64(), true
	}
	if fee > 1000 {
		fee = 1000
	}
	return fee, capBPS, emergency
}

func (p *AdvancedPool) SetStableRiskPolicy(ctx *bc.Context, warningBPS, emergencyBPS, surchargeBPS, maxSwapBPS string) {
	if apNorm(ctx.CallerAddr) != ctx.Get("factory") || ctx.Get("pool_type") != "stable" {
		ctx.Revert("stable factory policy only")
	}
	w, e, s, c := apBig(warningBPS), apBig(emergencyBPS), apBig(surchargeBPS), apBig(maxSwapBPS)
	if !w.IsInt64() || !e.IsInt64() || !s.IsInt64() || !c.IsInt64() || w.Int64() < 10 || e.Int64() <= w.Int64() || e.Int64() > 5000 || s.Int64() < 0 || s.Int64() > 1000 || c.Int64() < 1 || c.Int64() > 10000 {
		ctx.Revert("invalid stable risk policy")
	}
	ctx.Set("stable_warning_deviation_bps", w.String())
	ctx.Set("stable_emergency_deviation_bps", e.String())
	ctx.Set("stable_depeg_surcharge_bps", s.String())
	ctx.Set("stable_emergency_max_swap_bps", c.String())
	ctx.Emit("StableRiskPolicy", map[string]interface{}{"warningBps": w.String(), "emergencyBps": e.String(), "surchargeBps": s.String(), "maxSwapBps": c.String()})
}

func (p *AdvancedPool) SetStableOraclePrices(ctx *bc.Context, price0X18, price1X18, observedAt string) {
	if apNorm(ctx.CallerAddr) != ctx.Get("factory") || ctx.Get("pool_type") != "stable" {
		ctx.Revert("stable factory oracle only")
	}
	p0, p1, ts := apBig(price0X18), apBig(price1X18), apBig(observedAt)
	if p0.Sign() <= 0 || p1.Sign() <= 0 || !ts.IsInt64() || ts.Int64() > ctx.BlockTime+30 || ctx.BlockTime-ts.Int64() > 900 {
		ctx.Revert("invalid or stale stable oracle")
	}
	ctx.Set("oracle_price0_x18", p0.String())
	ctx.Set("oracle_price1_x18", p1.String())
	ctx.Set("oracle_updated_at", ts.String())
	ctx.Emit("StableOracleUpdated", map[string]interface{}{"price0X18": p0.String(), "price1X18": p1.String(), "observedAt": ts.String()})
}

func (p *AdvancedPool) loadConcentrated(ctx *bc.Context) bc.ConcentratedPool {
	var pool bc.ConcentratedPool
	if json.Unmarshal([]byte(ctx.Get("concentrated_state")), &pool) != nil {
		ctx.Revert("invalid concentrated state")
	}
	return pool
}

func (p *AdvancedPool) saveConcentrated(ctx *bc.Context, pool bc.ConcentratedPool) {
	raw, err := json.Marshal(pool)
	if err != nil {
		ctx.Revert("cannot encode concentrated state")
	}
	ctx.Set("concentrated_state", string(raw))
}

func (p *AdvancedPool) mintLP(ctx *bc.Context, owner string, amount *big.Int) {
	ctx.Set("lp:"+owner, new(big.Int).Add(apBig(ctx.Get("lp:"+owner)), amount).String())
	ctx.Set("totalLP", new(big.Int).Add(apBig(ctx.Get("totalLP")), amount).String())
}

func (p *AdvancedPool) addLiquidity(ctx *bc.Context, receiver string, amount0, amount1 *big.Int, fromCaller bool) {
	if amount0.Sign() <= 0 || amount1.Sign() <= 0 {
		ctx.Revert("positive liquidity required")
	}
	from := p.actor(ctx)
	if fromCaller {
		from = apNorm(ctx.CallerAddr)
	}
	p.pull(ctx, ctx.Get("token0"), from, amount0, fromCaller)
	p.pull(ctx, ctx.Get("token1"), from, amount1, fromCaller)
	r0, r1, total := apBig(ctx.Get("reserve0")), apBig(ctx.Get("reserve1")), apBig(ctx.Get("totalLP"))
	minted := apSqrt(new(big.Int).Mul(amount0, amount1))
	if total.Sign() > 0 {
		minted = apMin(new(big.Int).Div(new(big.Int).Mul(amount0, total), r0), new(big.Int).Div(new(big.Int).Mul(amount1, total), r1))
	}
	if minted.Sign() <= 0 {
		ctx.Revert("zero LP minted")
	}
	p.mintLP(ctx, receiver, minted)
	ctx.Set("reserve0", new(big.Int).Add(r0, amount0).String())
	ctx.Set("reserve1", new(big.Int).Add(r1, amount1).String())
	if ctx.Get("pool_type") == "concentrated" {
		pool := p.loadConcentrated(ctx)
		lower := new(big.Int).Div(new(big.Int).Mul(pool.SqrtPriceX18, big.NewInt(50)), big.NewInt(100))
		upper := new(big.Int).Div(new(big.Int).Mul(pool.SqrtPriceX18, big.NewInt(150)), big.NewInt(100))
		if lower.Sign() == 0 {
			lower.SetInt64(1)
		}
		// Minted shares and token amounts use the same atomic-unit scale. The
		// concentrated engine applies its own X18 price scale; multiplying
		// liquidity by another 1e18 would make normal swaps round to zero.
		liquidity := new(big.Int).Set(minted)
		if err := pool.AddRange(lower, upper, liquidity); err != nil {
			ctx.Revert(err.Error())
		}
		p.saveConcentrated(ctx, pool)
	}
	ctx.Set("output", minted.String())
	ctx.Commit()
	ctx.Emit("AdvancedLiquidityAdded", map[string]interface{}{"receiver": receiver, "amount0": amount0.String(), "amount1": amount1.String(), "shares": minted.String()})
}

func (p *AdvancedPool) AddLiquidity(ctx *bc.Context, amount0, amount1 string) {
	p.addLiquidity(ctx, p.actor(ctx), apBig(amount0), apBig(amount1), false)
}

func (p *AdvancedPool) AddLiquidityFromContract(ctx *bc.Context, receiver, amount0, amount1 string) {
	p.addLiquidity(ctx, apNorm(receiver), apBig(amount0), apBig(amount1), true)
}

func (p *AdvancedPool) quote(ctx *bc.Context, amount *big.Int, tokenIn string) *big.Int {
	t0, t1 := ctx.Get("token0"), ctx.Get("token1")
	tokenIn = apNorm(tokenIn)
	if amount.Sign() <= 0 || (tokenIn != t0 && tokenIn != t1) {
		return big.NewInt(0)
	}
	r0, r1 := apBig(ctx.Get("reserve0")), apBig(ctx.Get("reserve1"))
	rin, rout := r0, r1
	zeroForOne := true
	if tokenIn == t1 {
		rin, rout, zeroForOne = r1, r0, false
	}
	if ctx.Get("pool_type") == "stable" {
		fee, _, _ := p.stableFeeAndCap(ctx)
		out, err := bc.StableSwapAmountOut(amount, rin, rout, apBig(ctx.Get("parameter")).Int64(), fee)
		if err != nil {
			return big.NewInt(0)
		}
		return out
	}
	pool := p.loadConcentrated(ctx)
	out, err := pool.Swap(amount, zeroForOne)
	if err != nil || out.Cmp(rout) >= 0 {
		return big.NewInt(0)
	}
	return out
}

func (p *AdvancedPool) swap(ctx *bc.Context, receiver, amountIn, minOut, tokenIn string, fromCaller bool) {
	amount, minimum := apBig(amountIn), apBig(minOut)
	if ctx.Get("pool_type") == "stable" {
		_, capBPS, emergency := p.stableFeeAndCap(ctx)
		if emergency {
			r0, r1 := apBig(ctx.Get("reserve0")), apBig(ctx.Get("reserve1"))
			reserve := r0
			if apNorm(tokenIn) == ctx.Get("token1") {
				reserve = r1
			}
			max := new(big.Int).Div(new(big.Int).Mul(reserve, big.NewInt(capBPS)), big.NewInt(10000))
			if amount.Cmp(max) > 0 {
				ctx.Revert("stable emergency swap cap exceeded")
			}
		}
	}
	out := p.quote(ctx, amount, tokenIn)
	if out.Sign() <= 0 || out.Cmp(minimum) < 0 {
		ctx.Revert("advanced quote below minimum")
	}
	t0, t1, input := ctx.Get("token0"), ctx.Get("token1"), apNorm(tokenIn)
	output := t1
	if input == t1 {
		output = t0
	}
	from := p.actor(ctx)
	if fromCaller {
		from = apNorm(ctx.CallerAddr)
	}
	p.pull(ctx, input, from, amount, fromCaller)
	r0, r1 := apBig(ctx.Get("reserve0")), apBig(ctx.Get("reserve1"))
	if input == t0 {
		ctx.Set("reserve0", new(big.Int).Add(r0, amount).String())
		ctx.Set("reserve1", new(big.Int).Sub(r1, out).String())
	} else {
		ctx.Set("reserve1", new(big.Int).Add(r1, amount).String())
		ctx.Set("reserve0", new(big.Int).Sub(r0, out).String())
	}
	if ctx.Get("pool_type") == "concentrated" {
		pool := p.loadConcentrated(ctx)
		if _, err := pool.Swap(amount, input == t0); err != nil {
			ctx.Revert(err.Error())
		}
		p.saveConcentrated(ctx, pool)
	}
	ctx.Set("output", out.String())
	ctx.Commit()
	p.push(ctx, output, apNorm(receiver), out)
	ctx.Emit("AdvancedSwap", map[string]interface{}{"type": ctx.Get("pool_type"), "tokenIn": input, "amountIn": amount.String(), "amountOut": out.String()})
}

func (p *AdvancedPool) MintConcentratedPosition(ctx *bc.Context, lowerSqrtX18, upperSqrtX18, amount0, amount1 string) {
	if ctx.Get("pool_type") != "concentrated" {
		ctx.Revert("concentrated pool required")
	}
	owner := p.actor(ctx)
	lower, upper, a0, a1 := apBig(lowerSqrtX18), apBig(upperSqrtX18), apBig(amount0), apBig(amount1)
	if lower.Sign() <= 0 || lower.Cmp(upper) >= 0 || a0.Sign() <= 0 || a1.Sign() <= 0 {
		ctx.Revert("invalid concentrated position")
	}
	p.pull(ctx, ctx.Get("token0"), owner, a0, false)
	p.pull(ctx, ctx.Get("token1"), owner, a1, false)
	r0, r1, total := apBig(ctx.Get("reserve0")), apBig(ctx.Get("reserve1")), apBig(ctx.Get("totalLP"))
	liquidity := apSqrt(new(big.Int).Mul(a0, a1))
	if total.Sign() > 0 {
		liquidity = apMin(new(big.Int).Div(new(big.Int).Mul(a0, total), r0), new(big.Int).Div(new(big.Int).Mul(a1, total), r1))
	}
	if liquidity.Sign() <= 0 {
		ctx.Revert("zero position liquidity")
	}
	seq := new(big.Int).Add(apBig(ctx.Get("position_seq")), big.NewInt(1))
	id := "position_" + seq.String()
	pool := p.loadConcentrated(ctx)
	if err := pool.AddPosition(id, owner, lower, upper, liquidity); err != nil {
		ctx.Revert(err.Error())
	}
	p.saveConcentrated(ctx, pool)
	p.mintLP(ctx, ctx.ContractAddr, liquidity)
	ctx.Set("reserve0", new(big.Int).Add(r0, a0).String())
	ctx.Set("reserve1", new(big.Int).Add(r1, a1).String())
	ctx.Set("position_seq", seq.String())
	ctx.Set("output", id)
	ctx.Emit("ConcentratedPositionMinted", map[string]interface{}{"id": id, "owner": owner, "liquidity": liquidity.String(), "lower": lower.String(), "upper": upper.String()})
}

func (p *AdvancedPool) TransferPosition(ctx *bc.Context, id, to string) {
	if ctx.Get("pool_type") != "concentrated" {
		ctx.Revert("concentrated pool required")
	}
	from := p.actor(ctx)
	to = apNorm(to)
	pool := p.loadConcentrated(ctx)
	if err := pool.TransferPosition(id, from, to); err != nil {
		ctx.Revert(err.Error())
	}
	p.saveConcentrated(ctx, pool)
	ctx.Emit("ConcentratedPositionTransferred", map[string]interface{}{"id": id, "from": from, "to": to})
}

func (p *AdvancedPool) CollectPositionFees(ctx *bc.Context, id string) {
	if ctx.Get("pool_type") != "concentrated" {
		ctx.Revert("concentrated pool required")
	}
	owner := p.actor(ctx)
	pool := p.loadConcentrated(ctx)
	owed0, owed1, err := pool.CollectPositionFees(id, owner)
	if err != nil {
		ctx.Revert(err.Error())
	}
	r0, r1 := apBig(ctx.Get("reserve0")), apBig(ctx.Get("reserve1"))
	if r0.Cmp(owed0) < 0 || r1.Cmp(owed1) < 0 {
		ctx.Revert("position fee reserve unavailable")
	}
	ctx.Set("reserve0", new(big.Int).Sub(r0, owed0).String())
	ctx.Set("reserve1", new(big.Int).Sub(r1, owed1).String())
	p.saveConcentrated(ctx, pool)
	p.push(ctx, ctx.Get("token0"), owner, owed0)
	p.push(ctx, ctx.Get("token1"), owner, owed1)
	ctx.Set("output", owed0.String()+","+owed1.String())
	ctx.Emit("ConcentratedFeesCollected", map[string]interface{}{"id": id, "amount0": owed0.String(), "amount1": owed1.String()})
}

func (p *AdvancedPool) BurnConcentratedPosition(ctx *bc.Context, id string) {
	if ctx.Get("pool_type") != "concentrated" {
		ctx.Revert("concentrated pool required")
	}
	owner := p.actor(ctx)
	pool := p.loadConcentrated(ctx)
	position := pool.Positions[id]
	if position == nil || position.Owner != owner {
		ctx.Revert("position owner required")
	}
	liquidity := new(big.Int).Set(position.Liquidity)
	owed0, owed1, err := pool.CollectPositionFees(id, owner)
	if err != nil {
		ctx.Revert(err.Error())
	}
	if _, err = pool.RemovePosition(id, owner, liquidity); err != nil {
		ctx.Revert(err.Error())
	}
	total := apBig(ctx.Get("totalLP"))
	r0, r1 := apBig(ctx.Get("reserve0")), apBig(ctx.Get("reserve1"))
	if total.Sign() <= 0 {
		ctx.Revert("empty LP supply")
	}
	principal0 := new(big.Int).Div(new(big.Int).Mul(new(big.Int).Sub(r0, owed0), liquidity), total)
	principal1 := new(big.Int).Div(new(big.Int).Mul(new(big.Int).Sub(r1, owed1), liquidity), total)
	out0, out1 := new(big.Int).Add(principal0, owed0), new(big.Int).Add(principal1, owed1)
	contractLP := apBig(ctx.Get("lp:" + ctx.ContractAddr))
	if contractLP.Cmp(liquidity) < 0 || r0.Cmp(out0) < 0 || r1.Cmp(out1) < 0 {
		ctx.Revert("position custody invariant")
	}
	ctx.Set("lp:"+ctx.ContractAddr, new(big.Int).Sub(contractLP, liquidity).String())
	ctx.Set("totalLP", new(big.Int).Sub(total, liquidity).String())
	ctx.Set("reserve0", new(big.Int).Sub(r0, out0).String())
	ctx.Set("reserve1", new(big.Int).Sub(r1, out1).String())
	p.saveConcentrated(ctx, pool)
	p.push(ctx, ctx.Get("token0"), owner, out0)
	p.push(ctx, ctx.Get("token1"), owner, out1)
	ctx.Set("output", out0.String()+","+out1.String())
	ctx.Emit("ConcentratedPositionBurned", map[string]interface{}{"id": id, "amount0": out0.String(), "amount1": out1.String()})
}

func (p *AdvancedPool) PositionInfo(ctx *bc.Context, id string) {
	pool := p.loadConcentrated(ctx)
	position := pool.Positions[id]
	if position == nil {
		ctx.Set("output", "")
		return
	}
	raw, _ := json.Marshal(position)
	ctx.Set("output", string(raw))
}

func (p *AdvancedPool) Swap(ctx *bc.Context, amountIn, minOut, tokenIn string) {
	p.swap(ctx, p.actor(ctx), amountIn, minOut, tokenIn, false)
}
func (p *AdvancedPool) SwapTo(ctx *bc.Context, receiver, amountIn, minOut, tokenIn string) {
	p.swap(ctx, receiver, amountIn, minOut, tokenIn, false)
}
func (p *AdvancedPool) SwapFromContract(ctx *bc.Context, receiver, amountIn, minOut, tokenIn string) {
	p.swap(ctx, receiver, amountIn, minOut, tokenIn, true)
}

func (p *AdvancedPool) RemoveLiquidity(ctx *bc.Context, shares string) {
	p.removeLiquidityTo(ctx, p.actor(ctx), p.actor(ctx), apBig(shares))
}

func (p *AdvancedPool) removeLiquidityTo(ctx *bc.Context, owner, receiver string, amount *big.Int) {
	bal, total := apBig(ctx.Get("lp:"+owner)), apBig(ctx.Get("totalLP"))
	if amount.Sign() <= 0 || bal.Cmp(amount) < 0 || total.Sign() <= 0 {
		ctx.Revert("invalid LP withdrawal")
	}
	r0, r1 := apBig(ctx.Get("reserve0")), apBig(ctx.Get("reserve1"))
	out0 := new(big.Int).Div(new(big.Int).Mul(r0, amount), total)
	out1 := new(big.Int).Div(new(big.Int).Mul(r1, amount), total)
	ctx.Set("lp:"+owner, new(big.Int).Sub(bal, amount).String())
	ctx.Set("totalLP", new(big.Int).Sub(total, amount).String())
	ctx.Set("reserve0", new(big.Int).Sub(r0, out0).String())
	ctx.Set("reserve1", new(big.Int).Sub(r1, out1).String())
	ctx.Set("output", out0.String()+","+out1.String())
	ctx.Commit()
	p.push(ctx, ctx.Get("token0"), receiver, out0)
	p.push(ctx, ctx.Get("token1"), receiver, out1)
}

func (p *AdvancedPool) RemoveLiquidityTo(ctx *bc.Context, receiver, shares string) {
	p.removeLiquidityTo(ctx, apNorm(ctx.CallerAddr), apNorm(receiver), apBig(shares))
}

func (p *AdvancedPool) Transfer(ctx *bc.Context, to, shares string) {
	from, to, amount := apNorm(ctx.CallerAddr), apNorm(to), apBig(shares)
	bal := apBig(ctx.Get("lp:" + from))
	if to == "" || amount.Sign() <= 0 || bal.Cmp(amount) < 0 {
		ctx.Revert("invalid LP transfer")
	}
	ctx.Set("lp:"+from, new(big.Int).Sub(bal, amount).String())
	ctx.Set("lp:"+to, new(big.Int).Add(apBig(ctx.Get("lp:"+to)), amount).String())
	ctx.Emit("Transfer", map[string]interface{}{"from": from, "to": to, "amount": amount.String()})
}

func (p *AdvancedPool) Approve(ctx *bc.Context, spender, shares string) {
	owner, spender, amount := apNorm(ctx.CallerAddr), apNorm(spender), apBig(shares)
	if spender == "" || amount.Sign() < 0 {
		ctx.Revert("invalid LP approval")
	}
	ctx.Set("lp_allow:"+owner+":"+spender, amount.String())
}

func (p *AdvancedPool) Allowance(ctx *bc.Context, owner, spender string) {
	ctx.Set("output", ctx.Get("lp_allow:"+apNorm(owner)+":"+apNorm(spender)))
}

func (p *AdvancedPool) TransferFrom(ctx *bc.Context, from, to, shares string) {
	spender, from, to, amount := apNorm(ctx.CallerAddr), apNorm(from), apNorm(to), apBig(shares)
	allowance, balance := apBig(ctx.Get("lp_allow:"+from+":"+spender)), apBig(ctx.Get("lp:"+from))
	if to == "" || amount.Sign() <= 0 || allowance.Cmp(amount) < 0 || balance.Cmp(amount) < 0 {
		ctx.Revert("LP allowance or balance exceeded")
	}
	ctx.Set("lp_allow:"+from+":"+spender, new(big.Int).Sub(allowance, amount).String())
	ctx.Set("lp:"+from, new(big.Int).Sub(balance, amount).String())
	ctx.Set("lp:"+to, new(big.Int).Add(apBig(ctx.Get("lp:"+to)), amount).String())
	ctx.Emit("Transfer", map[string]interface{}{"from": from, "to": to, "amount": amount.String(), "spender": spender})
}

func (p *AdvancedPool) GetQuote(ctx *bc.Context, amountIn, tokenIn string) {
	out := p.quote(ctx, apBig(amountIn), tokenIn)
	ctx.Set("output", out.String()+",0,"+ctx.Get("reserve0")+","+ctx.Get("reserve1")+",0,0,0,0,0")
}
func (p *AdvancedPool) GetAmountOut(ctx *bc.Context, amountIn, tokenIn string) {
	ctx.Set("output", p.quote(ctx, apBig(amountIn), tokenIn).String())
}
func (p *AdvancedPool) GetReserves(ctx *bc.Context) {
	ctx.Set("output", ctx.Get("reserve0")+","+ctx.Get("reserve1")+","+ctx.Get("totalLP"))
}
func (p *AdvancedPool) Token0(ctx *bc.Context)   { ctx.Set("output", ctx.Get("token0")) }
func (p *AdvancedPool) Token1(ctx *bc.Context)   { ctx.Set("output", ctx.Get("token1")) }
func (p *AdvancedPool) PoolType(ctx *bc.Context) { ctx.Set("output", ctx.Get("pool_type")) }
func (p *AdvancedPool) GetRoutingWeight(ctx *bc.Context) {
	ctx.Set("output", ctx.Get("routing_weight"))
}
func (p *AdvancedPool) BalanceOf(ctx *bc.Context, owner string) {
	ctx.Set("output", ctx.Get("lp:"+apNorm(owner)))
}

var Contract = &AdvancedPool{}
