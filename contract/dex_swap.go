package main

import (
	"fmt"
	"strconv"

	blockchaincomponent "github.com/Zotish/DefenceProject/BlockchainComponent"
)

type DEX struct{}

// ------------------------
// Helpers
// ------------------------
func parseUint(v string) uint64 {
	u, _ := strconv.ParseUint(v, 10, 64)
	return u
}

func (d *DEX) bal(ctx *blockchaincomponent.Context, key string) uint64 {
	v := ctx.Get(key)
	if v == "" {
		return 0
	}
	return parseUint(v)
}

func (d *DEX) set(ctx *blockchaincomponent.Context, key string, val uint64) {
	ctx.Set(key, fmt.Sprintf("%d", val))
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
	amtA := parseUint(amountA)
	amtB := parseUint(amountB)

	if amtA == 0 || amtB == 0 {
		ctx.Revert("invalid liquidity amounts")
	}

	// reserves
	resA := d.bal(ctx, "reserveA")
	resB := d.bal(ctx, "reserveB")

	// total LP supply
	totalLP := d.bal(ctx, "totalLP")

	var mintedLP uint64

	if resA == 0 && resB == 0 {
		// first liquidity provider
		mintedLP = uint64(amtA + amtB)
	} else {
		// proportional LP minting
		mintedLP = min(amtA*totalLP/resA, amtB*totalLP/resB)
	}

	// update reserves
	d.set(ctx, "reserveA", resA+amtA)
	d.set(ctx, "reserveB", resB+amtB)

	// mint LP to provider
	providerKey := "lp:" + ctx.CallerAddr
	old := d.bal(ctx, providerKey)
	d.set(ctx, providerKey, old+mintedLP)

	d.set(ctx, "totalLP", totalLP+mintedLP)

	ctx.Emit("LiquidityAdded", map[string]interface{}{
		"provider": ctx.CallerAddr,
		"amountA":  amtA,
		"amountB":  amtB,
		"lpMinted": mintedLP,
	})
}

// ------------------------
// RemoveLiquidity(lpAmount)
// ------------------------
func (d *DEX) RemoveLiquidity(ctx *blockchaincomponent.Context, lpAmount string) {
	lp := parseUint(lpAmount)
	if lp == 0 {
		ctx.Revert("invalid lp amount")
	}

	providerKey := "lp:" + ctx.CallerAddr
	userLP := d.bal(ctx, providerKey)
	if lp > userLP {
		ctx.Revert("insufficient LP")
	}

	totalLP := d.bal(ctx, "totalLP")
	resA := d.bal(ctx, "reserveA")
	resB := d.bal(ctx, "reserveB")

	// proportional withdrawal
	outA := lp * resA / totalLP
	outB := lp * resB / totalLP

	// update reserves
	d.set(ctx, "reserveA", resA-outA)
	d.set(ctx, "reserveB", resB-outB)

	// burn LP
	d.set(ctx, providerKey, userLP-lp)
	d.set(ctx, "totalLP", totalLP-lp)

	ctx.Emit("LiquidityRemoved", map[string]interface{}{
		"provider": ctx.CallerAddr,
		"lpBurned": lp,
		"outA":     outA,
		"outB":     outB,
	})
}

// ------------------------
// Swap tokenA → tokenB
// ------------------------
func (d *DEX) SwapAtoB(ctx *blockchaincomponent.Context, amountIn string) {
	amtIn := parseUint(amountIn)

	resA := d.bal(ctx, "reserveA")
	resB := d.bal(ctx, "reserveB")

	if amtIn == 0 || resA == 0 || resB == 0 {
		ctx.Revert("invalid swap")
	}

	// AMM: output = (amountIn * reserveB) / (reserveA + amountIn)
	amtOut := (amtIn * resB) / (resA + amtIn)

	d.set(ctx, "reserveA", resA+amtIn)
	d.set(ctx, "reserveB", resB-amtOut)

	ctx.Emit("SwapAtoB", map[string]interface{}{
		"trader": ctx.CallerAddr,
		"inA":    amtIn,
		"outB":   amtOut,
	})
}

// ------------------------
// Swap tokenB → tokenA
// ------------------------
func (d *DEX) SwapBtoA(ctx *blockchaincomponent.Context, amountIn string) {
	amtIn := parseUint(amountIn)

	resA := d.bal(ctx, "reserveA")
	resB := d.bal(ctx, "reserveB")

	if amtIn == 0 || resA == 0 || resB == 0 {
		ctx.Revert("invalid swap")
	}

	amtOut := (amtIn * resA) / (resB + amtIn)

	d.set(ctx, "reserveA", resA-amtOut)
	d.set(ctx, "reserveB", resB+amtIn)

	ctx.Emit("SwapBtoA", map[string]interface{}{
		"trader": ctx.CallerAddr,
		"inB":    amtIn,
		"outA":   amtOut,
	})
}

// utility
func min(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}

var Contract1 = &DEX{}
