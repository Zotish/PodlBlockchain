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
	if amtIn.Sign() == 0 || resIn.Sign() == 0 || resOut.Sign() == 0 {
		return big.NewInt(0)
	}
	fee := new(big.Int).Mul(amtIn, big.NewInt(997))
	num := new(big.Int).Mul(fee, resOut)
	den := new(big.Int).Add(new(big.Int).Mul(resIn, big.NewInt(1000)), fee)
	return new(big.Int).Div(num, den)
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
	ctx.Commit()

	ctx.Emit("PairInitialized", map[string]interface{}{
		"factory": factory,
		"token0":  t0,
		"token1":  t1,
	})
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
}

// ─── Swap ─────────────────────────────────────────────────────────────────────

// Swap: send amountIn of tokenIn, receive at least minAmountOut of the other token.
func (p *Pair) Swap(ctx *bc.Context, amountIn string, minAmountOut string, tokenIn string) {
	t0 := ctx.Get("token0")
	t1 := ctx.Get("token1")
	if t0 == "" {
		ctx.Revert("pair not initialized")
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

	amtOut := calcAmountOut(amtIn, resIn, resOut)
	if amtOut.Cmp(minOut) < 0 {
		ctx.Revert("slippage: insufficient output amount")
	}
	if amtOut.Cmp(resOut) >= 0 {
		ctx.Revert("insufficient reserves")
	}

	p.pullToken(ctx, tIn, caller, amtIn)

	newResIn := new(big.Int).Add(resIn, amtIn)
	newResOut := new(big.Int).Sub(resOut, amtOut)

	// k-invariant check (1000x to account for fee)
	lhs := new(big.Int).Mul(
		new(big.Int).Sub(new(big.Int).Mul(newResIn, big.NewInt(1000)), new(big.Int).Mul(amtIn, big.NewInt(3))),
		new(big.Int).Mul(newResOut, big.NewInt(1000)),
	)
	rhs := new(big.Int).Mul(
		new(big.Int).Mul(resIn, big.NewInt(1000)),
		new(big.Int).Mul(resOut, big.NewInt(1000)),
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
	ctx.Set("epoch_volume", epochVol.String())
	// ─────────────────────────────────────────────────────────────────────────

	ctx.Commit()

	p.pushToken(ctx, tOut, caller, amtOut)

	ctx.Emit("Swap", map[string]interface{}{
		"sender":    caller,
		"tokenIn":   tIn,
		"tokenOut":  tOut,
		"amountIn":  amtIn.String(),
		"amountOut": amtOut.String(),
	})
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

func (p *Pair) GetAmountOut(ctx *bc.Context, amountIn string, tokenIn string) {
	t0 := ctx.Get("token0")
	tIn := strings.ToLower(strings.TrimSpace(tokenIn))
	res0 := parseBig(ctx.Get("reserve0"))
	res1 := parseBig(ctx.Get("reserve1"))

	var resIn, resOut *big.Int
	if tIn == t0 {
		resIn, resOut = res0, res1
	} else {
		resIn, resOut = res1, res0
	}
	out := calcAmountOut(parseBig(amountIn), resIn, resOut)
	ctx.Set("output", out.String())
	ctx.Emit("AmountOut", map[string]interface{}{"amountOut": out.String()})
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
	bal := p.lpBalance(ctx, from)
	if bal.Cmp(amt) < 0 {
		ctx.Revert("insufficient LP balance")
	}
	ctx.Set("lp:"+from, new(big.Int).Sub(bal, amt).String())
	ctx.Set("lp:"+to, new(big.Int).Add(p.lpBalance(ctx, to), amt).String())
	ctx.Commit()
	ctx.Emit("Transfer", map[string]interface{}{"from": from, "to": to, "amount": amount})
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
