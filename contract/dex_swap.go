//go:build ignore
// +build ignore

package main

import (
	"math/big"
	"strconv"
	"strings"
	"time"

	blockchaincomponent "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/BlockchainComponent"
)

// ─────────────────────────────────────────────────────────────────────────────
// LQD DEX — Uniswap v2-compatible AMM
//
// Key properties (matching Uniswap v2):
//   • 0.3 % swap fee  (997 / 1000 numerator)
//   • Geometric-mean first-mint with MINIMUM_LIQUIDITY burned
//   • k-invariant check after every swap
//   • GetAmountOut / GetAmountIn view helpers
//   • Proof of Dynamic Liquidity: validators lock LP tokens for consensus power
// ─────────────────────────────────────────────────────────────────────────────

type DEX struct{}

// MINIMUM_LIQUIDITY is permanently locked in the pool on the first deposit.
// This prevents the "share inflation" attack that would let the first LP
// manipulate the price arbitrarily.
const minimumLiquidity = int64(1000)

// ─── Helpers ─────────────────────────────────────────────────────────────────

func parseBig(v string) *big.Int {
	v = strings.TrimSpace(v)
	if v == "" {
		return big.NewInt(0)
	}
	z := new(big.Int)
	if _, ok := z.SetString(v, 10); !ok {
		return big.NewInt(0)
	}
	return z
}

func normAddr(addr string) string {
	return strings.ToLower(strings.TrimSpace(addr))
}

func (d *DEX) bal(ctx *blockchaincomponent.Context, key string) *big.Int {
	v := ctx.Get(key)
	if v == "" {
		return big.NewInt(0)
	}
	return parseBig(v)
}

func (d *DEX) set(ctx *blockchaincomponent.Context, key string, val *big.Int) {
	ctx.Set(key, val.String())
}

func (d *DEX) requireTokens(ctx *blockchaincomponent.Context) (string, string) {
	tokenA := ctx.Get("tokenA")
	tokenB := ctx.Get("tokenB")
	if tokenA == "" || tokenB == "" {
		ctx.Revert("pool not initialized")
	}
	return tokenA, tokenB
}

func (d *DEX) transferFrom(ctx *blockchaincomponent.Context, token, from, to string, amount *big.Int) {
	if token == "" || from == "" || to == "" {
		ctx.Revert("invalid token transfer")
	}
	_, err := ctx.Call(token, "TransferFrom", []string{from, to, amount.String()})
	if err != nil {
		ctx.Revert("transferFrom failed")
	}
}

func (d *DEX) transfer(ctx *blockchaincomponent.Context, token, to string, amount *big.Int) {
	if token == "" || to == "" {
		ctx.Revert("invalid token transfer")
	}
	_, err := ctx.Call(token, "Transfer", []string{to, amount.String()})
	if err != nil {
		ctx.Revert("transfer failed")
	}
}

// sqrtBig returns floor(sqrt(n)) via Newton's method (big.Int).
func sqrtBig(n *big.Int) *big.Int {
	if n.Sign() <= 0 {
		return big.NewInt(0)
	}
	// initial estimate: n >> 1 + 1  (upper bound)
	x := new(big.Int).Set(n)
	z := new(big.Int).Add(new(big.Int).Rsh(n, 1), big.NewInt(1))
	for z.Cmp(x) < 0 {
		x.Set(z)
		// z = (n/z + z) >> 1
		z = new(big.Int).Rsh(
			new(big.Int).Add(new(big.Int).Div(n, z), z),
			1,
		)
	}
	return x
}

func minBig(a, b *big.Int) *big.Int {
	if a.Cmp(b) < 0 {
		return a
	}
	return b
}

// ─── Uniswap v2 AMM math ─────────────────────────────────────────────────────

// getAmountOut computes exact output for a given input with 0.3 % fee.
//
//	amountOut = (amountIn * 997 * reserveOut) / (reserveIn * 1000 + amountIn * 997)
func getAmountOut(amountIn, reserveIn, reserveOut *big.Int) *big.Int {
	if amountIn.Sign() == 0 || reserveIn.Sign() == 0 || reserveOut.Sign() == 0 {
		return big.NewInt(0)
	}
	amtInWithFee := new(big.Int).Mul(amountIn, big.NewInt(997))
	numerator := new(big.Int).Mul(amtInWithFee, reserveOut)
	denominator := new(big.Int).Add(
		new(big.Int).Mul(reserveIn, big.NewInt(1000)),
		amtInWithFee,
	)
	return new(big.Int).Div(numerator, denominator)
}

// getAmountIn computes required input to receive an exact output with 0.3 % fee.
//
//	amountIn = (reserveIn * amountOut * 1000) / ((reserveOut - amountOut) * 997) + 1
func getAmountIn(amountOut, reserveIn, reserveOut *big.Int) *big.Int {
	if amountOut.Sign() == 0 || reserveIn.Sign() == 0 || reserveOut.Sign() == 0 {
		return big.NewInt(0)
	}
	if amountOut.Cmp(reserveOut) >= 0 {
		return big.NewInt(0) // impossible
	}
	numerator := new(big.Int).Mul(
		new(big.Int).Mul(reserveIn, amountOut),
		big.NewInt(1000),
	)
	denominator := new(big.Int).Mul(
		new(big.Int).Sub(reserveOut, amountOut),
		big.NewInt(997),
	)
	return new(big.Int).Add(
		new(big.Int).Div(numerator, denominator),
		big.NewInt(1),
	)
}

// ─── Initialization ───────────────────────────────────────────────────────────

// Init(tokenA, tokenB) — creates an empty LP pool for two tokens.
func (d *DEX) Init(ctx *blockchaincomponent.Context, tokenA string, tokenB string) {
	tokenA = normAddr(tokenA)
	tokenB = normAddr(tokenB)
	if tokenA == "" || tokenB == "" || tokenA == tokenB {
		ctx.Revert("invalid token addresses")
	}
	ctx.Set("tokenA", tokenA)
	ctx.Set("tokenB", tokenB)
	ctx.Set("reserveA", "0")
	ctx.Set("reserveB", "0")
	ctx.Set("totalLP", "0")

	ctx.Emit("DEXInitialized", map[string]interface{}{
		"tokenA": tokenA,
		"tokenB": tokenB,
	})
}

// ─── Liquidity ────────────────────────────────────────────────────────────────

// AddLiquidity(amountA, amountB) — deposit tokens and receive LP tokens.
//
// First deposit mints sqrt(amtA*amtB) total LP; MINIMUM_LIQUIDITY is burned to
// address(0) and the rest goes to the provider. Subsequent deposits use the
// standard Uniswap proportional formula.
func (d *DEX) AddLiquidity(ctx *blockchaincomponent.Context, amountA string, amountB string) {
	amtA := parseBig(amountA)
	amtB := parseBig(amountB)

	if amtA.Sign() == 0 || amtB.Sign() == 0 {
		ctx.Revert("amounts must be > 0")
	}

	tokenA, tokenB := d.requireTokens(ctx)
	caller := normAddr(ctx.CallerAddr)
	contract := normAddr(ctx.ContractAddr)

	d.transferFrom(ctx, tokenA, caller, contract, amtA)
	d.transferFrom(ctx, tokenB, caller, contract, amtB)

	resA := d.bal(ctx, "reserveA")
	resB := d.bal(ctx, "reserveB")
	totalLP := d.bal(ctx, "totalLP")

	var mintedLP *big.Int
	minLiq := big.NewInt(minimumLiquidity)

	if resA.Sign() == 0 && resB.Sign() == 0 {
		// First liquidity provider:
		// total minted = sqrt(amtA * amtB)
		// MINIMUM_LIQUIDITY burned to 0x0 to prevent inflation attack
		product := new(big.Int).Mul(amtA, amtB)
		sqrtLP := sqrtBig(product)
		if sqrtLP.Cmp(minLiq) <= 0 {
			ctx.Revert("initial liquidity too small (sqrt < MINIMUM_LIQUIDITY)")
		}
		// Burn MINIMUM_LIQUIDITY permanently
		burnKey := "lp:0x0000000000000000000000000000000000000000"
		d.set(ctx, burnKey, minLiq)
		totalLP = new(big.Int).Set(minLiq)
		mintedLP = new(big.Int).Sub(sqrtLP, minLiq)
	} else {
		// Subsequent providers: proportional mint, take the smaller share
		mintedA := new(big.Int).Div(new(big.Int).Mul(amtA, totalLP), resA)
		mintedB := new(big.Int).Div(new(big.Int).Mul(amtB, totalLP), resB)
		mintedLP = minBig(mintedA, mintedB)
	}

	if mintedLP.Sign() == 0 {
		ctx.Revert("insufficient liquidity minted")
	}

	// Update reserves
	d.set(ctx, "reserveA", new(big.Int).Add(resA, amtA))
	d.set(ctx, "reserveB", new(big.Int).Add(resB, amtB))

	// Mint LP to provider
	providerKey := "lp:" + caller
	d.set(ctx, providerKey, new(big.Int).Add(d.bal(ctx, providerKey), mintedLP))
	d.set(ctx, "totalLP", new(big.Int).Add(totalLP, mintedLP))

	ctx.Emit("Mint", map[string]interface{}{
		"provider": caller,
		"amountA":  amtA.String(),
		"amountB":  amtB.String(),
		"lpMinted": mintedLP.String(),
	})
	ctx.Emit("Sync", map[string]interface{}{
		"reserveA": new(big.Int).Add(resA, amtA).String(),
		"reserveB": new(big.Int).Add(resB, amtB).String(),
	})
}

// RemoveLiquidity(lpAmount) — burn LP tokens, receive proportional tokens.
func (d *DEX) RemoveLiquidity(ctx *blockchaincomponent.Context, lpAmount string) {
	lp := parseBig(lpAmount)
	if lp.Sign() == 0 {
		ctx.Revert("lp amount must be > 0")
	}
	tokenA, tokenB := d.requireTokens(ctx)

	caller := normAddr(ctx.CallerAddr)
	providerKey := "lp:" + caller
	userLP := d.bal(ctx, providerKey)
	if userLP.Cmp(lp) < 0 {
		ctx.Revert("insufficient LP balance")
	}

	totalLP := d.bal(ctx, "totalLP")
	resA := d.bal(ctx, "reserveA")
	resB := d.bal(ctx, "reserveB")

	// Proportional withdrawal
	outA := new(big.Int).Div(new(big.Int).Mul(lp, resA), totalLP)
	outB := new(big.Int).Div(new(big.Int).Mul(lp, resB), totalLP)

	if outA.Sign() == 0 || outB.Sign() == 0 {
		ctx.Revert("insufficient liquidity burned")
	}

	newResA := new(big.Int).Sub(resA, outA)
	newResB := new(big.Int).Sub(resB, outB)

	d.set(ctx, "reserveA", newResA)
	d.set(ctx, "reserveB", newResB)
	d.set(ctx, providerKey, new(big.Int).Sub(userLP, lp))
	d.set(ctx, "totalLP", new(big.Int).Sub(totalLP, lp))

	d.transfer(ctx, tokenA, caller, outA)
	d.transfer(ctx, tokenB, caller, outB)

	ctx.Emit("Burn", map[string]interface{}{
		"provider": caller,
		"lpBurned": lp.String(),
		"outA":     outA.String(),
		"outB":     outB.String(),
	})
	ctx.Emit("Sync", map[string]interface{}{
		"reserveA": newResA.String(),
		"reserveB": newResB.String(),
	})
}

// ─── Swaps ────────────────────────────────────────────────────────────────────

// SwapAtoB(amountIn) — swap token A for token B with 0.3 % fee.
func (d *DEX) SwapAtoB(ctx *blockchaincomponent.Context, amountIn string) {
	amtIn := parseBig(amountIn)
	resA := d.bal(ctx, "reserveA")
	resB := d.bal(ctx, "reserveB")

	if amtIn.Sign() == 0 || resA.Sign() == 0 || resB.Sign() == 0 {
		ctx.Revert("invalid swap parameters")
	}

	amtOut := getAmountOut(amtIn, resA, resB)
	if amtOut.Sign() == 0 {
		ctx.Revert("insufficient output amount")
	}

	tokenA, tokenB := d.requireTokens(ctx)
	caller := normAddr(ctx.CallerAddr)
	contract := normAddr(ctx.ContractAddr)

	d.transferFrom(ctx, tokenA, caller, contract, amtIn)
	d.transfer(ctx, tokenB, caller, amtOut)

	newResA := new(big.Int).Add(resA, amtIn)
	newResB := new(big.Int).Sub(resB, amtOut)

	// k-invariant guard (Uniswap v2 style, accounts for fee staying in pool)
	// balance0Adjusted = newResA*1000 − amtIn*3
	// balance1Adjusted = newResB*1000
	// require: balance0Adjusted * balance1Adjusted >= resA * resB * 1_000_000
	bal0Adj := new(big.Int).Sub(
		new(big.Int).Mul(newResA, big.NewInt(1000)),
		new(big.Int).Mul(amtIn, big.NewInt(3)),
	)
	bal1Adj := new(big.Int).Mul(newResB, big.NewInt(1000))
	kNew := new(big.Int).Mul(bal0Adj, bal1Adj)
	kOld := new(big.Int).Mul(
		new(big.Int).Mul(resA, resB),
		big.NewInt(1_000_000),
	)
	if kNew.Cmp(kOld) < 0 {
		ctx.Revert("K invariant violated")
	}

	d.set(ctx, "reserveA", newResA)
	d.set(ctx, "reserveB", newResB)

	ctx.Emit("Swap", map[string]interface{}{
		"trader":    caller,
		"amountIn":  amtIn.String(),
		"amountOut": amtOut.String(),
		"direction": "AtoB",
		"fee":       new(big.Int).Div(new(big.Int).Mul(amtIn, big.NewInt(3)), big.NewInt(1000)).String(),
	})
	ctx.Emit("Sync", map[string]interface{}{
		"reserveA": newResA.String(),
		"reserveB": newResB.String(),
	})
}

// SwapBtoA(amountIn) — swap token B for token A with 0.3 % fee.
func (d *DEX) SwapBtoA(ctx *blockchaincomponent.Context, amountIn string) {
	amtIn := parseBig(amountIn)
	resA := d.bal(ctx, "reserveA")
	resB := d.bal(ctx, "reserveB")

	if amtIn.Sign() == 0 || resA.Sign() == 0 || resB.Sign() == 0 {
		ctx.Revert("invalid swap parameters")
	}

	amtOut := getAmountOut(amtIn, resB, resA)
	if amtOut.Sign() == 0 {
		ctx.Revert("insufficient output amount")
	}

	tokenA, tokenB := d.requireTokens(ctx)
	caller := normAddr(ctx.CallerAddr)
	contract := normAddr(ctx.ContractAddr)

	d.transferFrom(ctx, tokenB, caller, contract, amtIn)
	d.transfer(ctx, tokenA, caller, amtOut)

	newResA := new(big.Int).Sub(resA, amtOut)
	newResB := new(big.Int).Add(resB, amtIn)

	// k-invariant guard
	bal1Adj := new(big.Int).Sub(
		new(big.Int).Mul(newResB, big.NewInt(1000)),
		new(big.Int).Mul(amtIn, big.NewInt(3)),
	)
	bal0Adj := new(big.Int).Mul(newResA, big.NewInt(1000))
	kNew := new(big.Int).Mul(bal0Adj, bal1Adj)
	kOld := new(big.Int).Mul(
		new(big.Int).Mul(resA, resB),
		big.NewInt(1_000_000),
	)
	if kNew.Cmp(kOld) < 0 {
		ctx.Revert("K invariant violated")
	}

	d.set(ctx, "reserveA", newResA)
	d.set(ctx, "reserveB", newResB)

	ctx.Emit("Swap", map[string]interface{}{
		"trader":    caller,
		"amountIn":  amtIn.String(),
		"amountOut": amtOut.String(),
		"direction": "BtoA",
		"fee":       new(big.Int).Div(new(big.Int).Mul(amtIn, big.NewInt(3)), big.NewInt(1000)).String(),
	})
	ctx.Emit("Sync", map[string]interface{}{
		"reserveA": newResA.String(),
		"reserveB": newResB.String(),
	})
}

// ─── View helpers ─────────────────────────────────────────────────────────────

// GetAmountOut(amountIn) — view: how much token B for a given token A input.
func (d *DEX) GetAmountOut(ctx *blockchaincomponent.Context, amountIn string) {
	amtIn := parseBig(amountIn)
	resA := d.bal(ctx, "reserveA")
	resB := d.bal(ctx, "reserveB")
	amtOut := getAmountOut(amtIn, resA, resB)
	// Price impact: how much amountIn moves the price (%)
	var impact string
	if resA.Sign() > 0 {
		pct := new(big.Int).Div(new(big.Int).Mul(amtIn, big.NewInt(10000)), resA)
		impact = pct.String() // in basis points; divide by 100 in frontend
	} else {
		impact = "0"
	}
	ctx.Emit("AmountOut", map[string]interface{}{
		"amountIn":        amtIn.String(),
		"amountOut":       amtOut.String(),
		"fee":             new(big.Int).Div(new(big.Int).Mul(amtIn, big.NewInt(3)), big.NewInt(1000)).String(),
		"priceImpactBps":  impact,
		"reserveA":        resA.String(),
		"reserveB":        resB.String(),
	})
}

// GetAmountIn(amountOut) — view: how much token A needed for a desired token B output.
func (d *DEX) GetAmountIn(ctx *blockchaincomponent.Context, amountOut string) {
	amtOut := parseBig(amountOut)
	resA := d.bal(ctx, "reserveA")
	resB := d.bal(ctx, "reserveB")
	amtIn := getAmountIn(amtOut, resA, resB)
	ctx.Emit("AmountIn", map[string]interface{}{
		"amountIn":  amtIn.String(),
		"amountOut": amtOut.String(),
		"reserveA":  resA.String(),
		"reserveB":  resB.String(),
	})
}

// GetPoolInfo — view: current pool state.
func (d *DEX) GetPoolInfo(ctx *blockchaincomponent.Context) {
	resA := d.bal(ctx, "reserveA")
	resB := d.bal(ctx, "reserveB")
	totalLP := d.bal(ctx, "totalLP")

	// spot price: resB / resA in basis-points (multiply by 10000 to keep integer)
	var price0Bps, price1Bps string
	if resA.Sign() > 0 {
		price0Bps = new(big.Int).Div(new(big.Int).Mul(resB, big.NewInt(10000)), resA).String()
	} else {
		price0Bps = "0"
	}
	if resB.Sign() > 0 {
		price1Bps = new(big.Int).Div(new(big.Int).Mul(resA, big.NewInt(10000)), resB).String()
	} else {
		price1Bps = "0"
	}

	ctx.Emit("PoolInfo", map[string]interface{}{
		"reserveA":   resA.String(),
		"reserveB":   resB.String(),
		"totalLP":    totalLP.String(),
		"price0Bps":  price0Bps,  // tokenB per tokenA × 10000
		"price1Bps":  price1Bps,  // tokenA per tokenB × 10000
		"tokenA":     ctx.Get("tokenA"),
		"tokenB":     ctx.Get("tokenB"),
	})
}

// GetLPBalance(addr) — view: LP balance for a specific address.
func (d *DEX) GetLPBalance(ctx *blockchaincomponent.Context, addr string) {
	a := normAddr(addr)
	lp := d.bal(ctx, "lp:"+a)
	totalLP := d.bal(ctx, "totalLP")
	resA := d.bal(ctx, "reserveA")
	resB := d.bal(ctx, "reserveB")

	var shareA, shareB string
	if totalLP.Sign() > 0 && lp.Sign() > 0 {
		shareA = new(big.Int).Div(new(big.Int).Mul(lp, resA), totalLP).String()
		shareB = new(big.Int).Div(new(big.Int).Mul(lp, resB), totalLP).String()
	} else {
		shareA = "0"
		shareB = "0"
	}

	ctx.Emit("LPBalance", map[string]interface{}{
		"address": a,
		"lp":      lp.String(),
		"shareA":  shareA,
		"shareB":  shareB,
	})
}

// GetLPValue(lpAmount) — view: pool-backing value of a given LP amount.
func (d *DEX) GetLPValue(ctx *blockchaincomponent.Context, lpAmount string) {
	lp := parseBig(lpAmount)
	totalLP := d.bal(ctx, "totalLP")
	resA := d.bal(ctx, "reserveA")
	resB := d.bal(ctx, "reserveB")

	value := big.NewInt(0)
	if lp.Sign() > 0 && totalLP.Sign() > 0 {
		poolTotal := new(big.Int).Add(resA, resB)
		value = new(big.Int).Div(new(big.Int).Mul(lp, poolTotal), totalLP)
	}

	ctx.Emit("LPValue", map[string]interface{}{
		"value":    value.String(),
		"reserveA": resA.String(),
		"reserveB": resB.String(),
		"totalLP":  totalLP.String(),
	})
}

// ─── Proof of Dynamic Liquidity — Validator LP Locking ───────────────────────
//
// Validators lock their DEX LP tokens here instead of staking raw LQD.
// Consensus power = real pool-backing value of locked LP × time multiplier.

// LockLPForValidation locks LP tokens for validation. Args: lpAmount, lockSeconds.
func (d *DEX) LockLPForValidation(ctx *blockchaincomponent.Context, lpAmount string, lockSeconds string) {
	caller := normAddr(ctx.CallerAddr)
	lp := parseBig(lpAmount)
	if lp.Sign() == 0 {
		ctx.Revert("lp amount must be > 0")
	}

	providerKey := "lp:" + caller
	userLP := d.bal(ctx, providerKey)
	if userLP.Cmp(lp) < 0 {
		ctx.Revert("insufficient LP balance to lock")
	}

	valLPKey := "val_lp:" + caller
	if d.bal(ctx, valLPKey).Sign() > 0 {
		ctx.Revert("already locked; call UnlockValidatorLP first")
	}

	secs, err := strconv.ParseInt(strings.TrimSpace(lockSeconds), 10, 64)
	if err != nil || secs <= 0 {
		ctx.Revert("invalid lock duration (must be > 0 seconds)")
	}

	d.set(ctx, providerKey, new(big.Int).Sub(userLP, lp))
	d.set(ctx, valLPKey, lp)
	lockUntil := time.Now().Unix() + secs
	ctx.Set("val_lock_until:"+caller, strconv.FormatInt(lockUntil, 10))

	ctx.Emit("LPLockedForValidation", map[string]interface{}{
		"validator": caller,
		"lpAmount":  lp.String(),
		"lockUntil": lockUntil,
		"lockSecs":  secs,
	})
}

// UnlockValidatorLP releases locked LP after the lock period expires.
func (d *DEX) UnlockValidatorLP(ctx *blockchaincomponent.Context) {
	caller := normAddr(ctx.CallerAddr)
	valLPKey := "val_lp:" + caller
	lockedLP := d.bal(ctx, valLPKey)
	if lockedLP.Sign() == 0 {
		ctx.Revert("no LP locked for validation")
	}

	lockUntilStr := ctx.Get("val_lock_until:" + caller)
	lockUntil, err := strconv.ParseInt(lockUntilStr, 10, 64)
	if err != nil {
		ctx.Revert("invalid lock record")
	}
	if time.Now().Unix() < lockUntil {
		ctx.Revert("lock period not expired yet")
	}

	providerKey := "lp:" + caller
	d.set(ctx, providerKey, new(big.Int).Add(d.bal(ctx, providerKey), lockedLP))
	d.set(ctx, valLPKey, big.NewInt(0))
	ctx.Set("val_lock_until:"+caller, "0")

	ctx.Emit("ValidatorLPUnlocked", map[string]interface{}{
		"validator": caller,
		"lpAmount":  lockedLP.String(),
	})
}

// GetValidatorLP queries the locked LP info for a validator.
func (d *DEX) GetValidatorLP(ctx *blockchaincomponent.Context, validatorAddr string) {
	addr := normAddr(validatorAddr)
	lockedLP := d.bal(ctx, "val_lp:"+addr)
	lockUntilStr := ctx.Get("val_lock_until:" + addr)
	lockUntil, _ := strconv.ParseInt(lockUntilStr, 10, 64)
	isActive := time.Now().Unix() < lockUntil && lockedLP.Sign() > 0

	// Compute liquidity power from pool backing
	resA := d.bal(ctx, "reserveA")
	resB := d.bal(ctx, "reserveB")
	totalLP := d.bal(ctx, "totalLP")
	poolBacking := big.NewInt(0)
	if totalLP.Sign() > 0 && lockedLP.Sign() > 0 {
		poolTotal := new(big.Int).Add(resA, resB)
		poolBacking = new(big.Int).Div(new(big.Int).Mul(lockedLP, poolTotal), totalLP)
	}

	ctx.Emit("ValidatorLPInfo", map[string]interface{}{
		"validator":    addr,
		"lockedLP":     lockedLP.String(),
		"lockUntil":    lockUntil,
		"isActive":     isActive,
		"poolBacking":  poolBacking.String(),
	})
}

// REQUIRED EXPORT
var Contract = &DEX{}
