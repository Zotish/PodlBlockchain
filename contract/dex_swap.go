package main

import (
	"math/big"
	"strings"

	blockchaincomponent "github.com/Zotish/DefenceProject/BlockchainComponent"
)

type DEX struct{}

// ------------------------
// Helpers
// ------------------------
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

// ------------------------
// Initialization
// ------------------------
//
// Init(tokenA, tokenB)
//
// Creates an empty LP pool.
// ------------------------
func (d *DEX) Init(ctx *blockchaincomponent.Context, tokenA string, tokenB string) {
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

// ------------------------
// AddLiquidity(amountA, amountB)
//
// LP minted = proportional to pool size
// ------------------------
func (d *DEX) AddLiquidity(ctx *blockchaincomponent.Context, amountA string, amountB string) {
	amtA := parseBig(amountA)
	amtB := parseBig(amountB)

	if amtA.Sign() == 0 || amtB.Sign() == 0 {
		ctx.Revert("invalid liquidity amounts")
	}

	// reserves
	resA := d.bal(ctx, "reserveA")
	resB := d.bal(ctx, "reserveB")

	// total LP supply
	totalLP := d.bal(ctx, "totalLP")

	var mintedLP *big.Int

	if resA.Sign() == 0 && resB.Sign() == 0 {
		// first liquidity provider
		mintedLP = new(big.Int).Add(amtA, amtB)
	} else {
		// proportional LP minting
		mintedA := new(big.Int).Div(new(big.Int).Mul(amtA, totalLP), resA)
		mintedB := new(big.Int).Div(new(big.Int).Mul(amtB, totalLP), resB)
		mintedLP = minBig(mintedA, mintedB)
	}

	// update reserves
	d.set(ctx, "reserveA", new(big.Int).Add(resA, amtA))
	d.set(ctx, "reserveB", new(big.Int).Add(resB, amtB))

	// mint LP to provider
	providerKey := "lp:" + ctx.CallerAddr
	old := d.bal(ctx, providerKey)
	d.set(ctx, providerKey, new(big.Int).Add(old, mintedLP))

	d.set(ctx, "totalLP", new(big.Int).Add(totalLP, mintedLP))

	ctx.Emit("LiquidityAdded", map[string]interface{}{
		"provider": ctx.CallerAddr,
		"amountA":  amtA.String(),
		"amountB":  amtB.String(),
		"lpMinted": mintedLP.String(),
	})
}

// ------------------------
// RemoveLiquidity(lpAmount)
// ------------------------
func (d *DEX) RemoveLiquidity(ctx *blockchaincomponent.Context, lpAmount string) {
	lp := parseBig(lpAmount)
	if lp.Sign() == 0 {
		ctx.Revert("invalid lp amount")
	}

	providerKey := "lp:" + ctx.CallerAddr
	userLP := d.bal(ctx, providerKey)
	if userLP.Cmp(lp) < 0 {
		ctx.Revert("insufficient LP")
	}

	totalLP := d.bal(ctx, "totalLP")
	resA := d.bal(ctx, "reserveA")
	resB := d.bal(ctx, "reserveB")

	// proportional withdrawal
	outA := new(big.Int).Div(new(big.Int).Mul(lp, resA), totalLP)
	outB := new(big.Int).Div(new(big.Int).Mul(lp, resB), totalLP)

	// update reserves
	d.set(ctx, "reserveA", new(big.Int).Sub(resA, outA))
	d.set(ctx, "reserveB", new(big.Int).Sub(resB, outB))

	// burn LP
	d.set(ctx, providerKey, new(big.Int).Sub(userLP, lp))
	d.set(ctx, "totalLP", new(big.Int).Sub(totalLP, lp))

	ctx.Emit("LiquidityRemoved", map[string]interface{}{
		"provider": ctx.CallerAddr,
		"lpBurned": lp.String(),
		"outA":     outA.String(),
		"outB":     outB.String(),
	})
}

// ------------------------
// Swap tokenA → tokenB
// ------------------------
func (d *DEX) SwapAtoB(ctx *blockchaincomponent.Context, amountIn string) {
	amtIn := parseBig(amountIn)

	resA := d.bal(ctx, "reserveA")
	resB := d.bal(ctx, "reserveB")

	if amtIn.Sign() == 0 || resA.Sign() == 0 || resB.Sign() == 0 {
		ctx.Revert("invalid swap")
	}

	// AMM: output = (amountIn * reserveB) / (reserveA + amountIn)
	amtOut := new(big.Int).Div(new(big.Int).Mul(amtIn, resB), new(big.Int).Add(resA, amtIn))

	d.set(ctx, "reserveA", new(big.Int).Add(resA, amtIn))
	d.set(ctx, "reserveB", new(big.Int).Sub(resB, amtOut))

	ctx.Emit("SwapAtoB", map[string]interface{}{
		"trader": ctx.CallerAddr,
		"inA":    amtIn.String(),
		"outB":   amtOut.String(),
	})
}

// ------------------------
// Swap tokenB → tokenA
// ------------------------
func (d *DEX) SwapBtoA(ctx *blockchaincomponent.Context, amountIn string) {
	amtIn := parseBig(amountIn)

	resA := d.bal(ctx, "reserveA")
	resB := d.bal(ctx, "reserveB")

	if amtIn.Sign() == 0 || resA.Sign() == 0 || resB.Sign() == 0 {
		ctx.Revert("invalid swap")
	}

	amtOut := new(big.Int).Div(new(big.Int).Mul(amtIn, resA), new(big.Int).Add(resB, amtIn))

	d.set(ctx, "reserveA", new(big.Int).Sub(resA, amtOut))
	d.set(ctx, "reserveB", new(big.Int).Add(resB, amtIn))

	ctx.Emit("SwapBtoA", map[string]interface{}{
		"trader": ctx.CallerAddr,
		"inB":    amtIn.String(),
		"outA":   amtOut.String(),
	})
}

// utility
func minBig(a, b *big.Int) *big.Int {
	if a.Cmp(b) < 0 {
		return a
	}
	return b
}

// REQUIRED EXPORT
var Contract = &DEX{}
