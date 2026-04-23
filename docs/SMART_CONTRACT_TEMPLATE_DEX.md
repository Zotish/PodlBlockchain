# PoDL DEX Smart Contract Starter Template

This is a starter template for a Uniswap-v2-style AMM pair contract on PoDL.

Use this when you want to build a custom pool contract, experiment with liquidity math, or learn the pair storage layout.

## What this template covers

- pair initialization
- reserve tracking
- LP mint/burn
- add liquidity
- remove liquidity
- swap

## Starter DEX pair template

```go
package main

import (
	"math/big"
	"strings"

	lqdctx "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/lqd-sdk-compat/context"
)

const Native = "lqd"

type DEXPair struct{}

func normAddr(addr string) string {
	return strings.ToLower(strings.TrimSpace(addr))
}

func parseBig(s string) *big.Int {
	s = strings.TrimSpace(s)
	if s == "" {
		return big.NewInt(0)
	}
	z := new(big.Int)
	if _, ok := z.SetString(s, 10); !ok {
		return big.NewInt(0)
	}
	return z
}

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

func calcAmountOut(amtIn, resIn, resOut *big.Int) *big.Int {
	if amtIn.Sign() == 0 || resIn.Sign() == 0 || resOut.Sign() == 0 {
		return big.NewInt(0)
	}
	fee := new(big.Int).Mul(amtIn, big.NewInt(997))
	num := new(big.Int).Mul(fee, resOut)
	den := new(big.Int).Add(new(big.Int).Mul(resIn, big.NewInt(1000)), fee)
	return new(big.Int).Div(num, den)
}

func (p *DEXPair) Init(ctx *lqdctx.Context, factory string, token0 string, token1 string) {
	ctx.Set("factory", normAddr(factory))
	ctx.Set("token0", normAddr(token0))
	ctx.Set("token1", normAddr(token1))
	ctx.Set("reserve0", "0")
	ctx.Set("reserve1", "0")
	ctx.Set("totalLP", "0")
}

func (p *DEXPair) GetPoolInfo(ctx *lqdctx.Context) {
	ctx.Set("output", strings.Join([]string{
		ctx.Get("token0"),
		ctx.Get("token1"),
		ctx.Get("reserve0"),
		ctx.Get("reserve1"),
		ctx.Get("totalLP"),
	}, ","))
}

func (p *DEXPair) AddLiquidity(ctx *lqdctx.Context, amount0 string, amount1 string) {
	a0 := parseBig(amount0)
	a1 := parseBig(amount1)
	if a0.Sign() <= 0 || a1.Sign() <= 0 {
		ctx.Revert("amounts must be > 0")
	}
	r0 := parseBig(ctx.Get("reserve0"))
	r1 := parseBig(ctx.Get("reserve1"))
	lp := parseBig(ctx.Get("totalLP"))

	// Pull tokens from the caller into the pair.
	if ctx.Get("token0") == Native {
		ctx.ReceiveNative(a0)
	} else {
		if _, err := ctx.Call(ctx.Get("token0"), "TransferFrom", []string{ctx.CallerAddr, ctx.ContractAddr, a0.String()}); err != nil {
			ctx.Revert(err.Error())
		}
	}
	if ctx.Get("token1") == Native {
		ctx.ReceiveNative(a1)
	} else {
		if _, err := ctx.Call(ctx.Get("token1"), "TransferFrom", []string{ctx.CallerAddr, ctx.ContractAddr, a1.String()}); err != nil {
			ctx.Revert(err.Error())
		}
	}

	minted := big.NewInt(0)
	if lp.Sign() == 0 {
		minted = sqrtBig(new(big.Int).Mul(a0, a1))
	} else {
		lp0 := new(big.Int).Div(new(big.Int).Mul(a0, lp), r0)
		lp1 := new(big.Int).Div(new(big.Int).Mul(a1, lp), r1)
		if lp0.Cmp(lp1) < 0 {
			minted = lp0
		} else {
			minted = lp1
		}
	}
	if minted.Sign() <= 0 {
		ctx.Revert("zero LP minted")
	}

	ctx.Set("reserve0", new(big.Int).Add(r0, a0).String())
	ctx.Set("reserve1", new(big.Int).Add(r1, a1).String())
	ctx.Set("totalLP", new(big.Int).Add(lp, minted).String())
	ctx.Set("lp:"+strings.ToLower(ctx.CallerAddr), new(big.Int).Add(parseBig(ctx.Get("lp:"+strings.ToLower(ctx.CallerAddr))), minted).String())
}

func (p *DEXPair) RemoveLiquidity(ctx *lqdctx.Context, lpAmount string) {
	lpAmt := parseBig(lpAmount)
	if lpAmt.Sign() <= 0 {
		ctx.Revert("LP amount must be > 0")
	}
	totalLP := parseBig(ctx.Get("totalLP"))
	if totalLP.Sign() == 0 {
		ctx.Revert("no liquidity")
	}

	r0 := parseBig(ctx.Get("reserve0"))
	r1 := parseBig(ctx.Get("reserve1"))
	out0 := new(big.Int).Div(new(big.Int).Mul(lpAmt, r0), totalLP)
	out1 := new(big.Int).Div(new(big.Int).Mul(lpAmt, r1), totalLP)

	ctx.Set("reserve0", new(big.Int).Sub(r0, out0).String())
	ctx.Set("reserve1", new(big.Int).Sub(r1, out1).String())
	ctx.Set("totalLP", new(big.Int).Sub(totalLP, lpAmt).String())

	if ctx.Get("token0") == Native {
		ctx.SendNative(ctx.CallerAddr, out0)
	} else {
		if _, err := ctx.Call(ctx.Get("token0"), "Transfer", []string{ctx.CallerAddr, out0.String()}); err != nil {
			ctx.Revert(err.Error())
		}
	}
	if ctx.Get("token1") == Native {
		ctx.SendNative(ctx.CallerAddr, out1)
	} else {
		if _, err := ctx.Call(ctx.Get("token1"), "Transfer", []string{ctx.CallerAddr, out1.String()}); err != nil {
			ctx.Revert(err.Error())
		}
	}
}

func (p *DEXPair) Swap(ctx *lqdctx.Context, tokenIn string, amountIn string, minOut string) {
	tIn := normAddr(tokenIn)
	amtIn := parseBig(amountIn)
	minAmtOut := parseBig(minOut)
	if amtIn.Sign() <= 0 {
		ctx.Revert("amountIn must be > 0")
	}

	t0 := ctx.Get("token0")
	t1 := ctx.Get("token1")
	r0 := parseBig(ctx.Get("reserve0"))
	r1 := parseBig(ctx.Get("reserve1"))

	var out *big.Int
	if tIn == t0 {
		out = calcAmountOut(amtIn, r0, r1)
		if out.Cmp(minAmtOut) < 0 {
			ctx.Revert("slippage too high")
		}
		if _, err := ctx.Call(t0, "TransferFrom", []string{ctx.CallerAddr, ctx.ContractAddr, amtIn.String()}); err != nil {
			ctx.Revert(err.Error())
		}
		if t1 == Native {
			ctx.SendNative(ctx.CallerAddr, out)
		} else {
			if _, err := ctx.Call(t1, "Transfer", []string{ctx.CallerAddr, out.String()}); err != nil {
				ctx.Revert(err.Error())
			}
		}
		ctx.Set("reserve0", new(big.Int).Add(r0, amtIn).String())
		ctx.Set("reserve1", new(big.Int).Sub(r1, out).String())
		return
	}

	out = calcAmountOut(amtIn, r1, r0)
	if out.Cmp(minAmtOut) < 0 {
		ctx.Revert("slippage too high")
	}
	if _, err := ctx.Call(t1, "TransferFrom", []string{ctx.CallerAddr, ctx.ContractAddr, amtIn.String()}); err != nil {
		ctx.Revert(err.Error())
	}
	if t0 == Native {
		ctx.SendNative(ctx.CallerAddr, out)
	} else {
		if _, err := ctx.Call(t0, "Transfer", []string{ctx.CallerAddr, out.String()}); err != nil {
			ctx.Revert(err.Error())
		}
	}
	ctx.Set("reserve1", new(big.Int).Add(r1, amtIn).String())
	ctx.Set("reserve0", new(big.Int).Sub(r0, out).String())
}

var Contract = &DEXPair{}
```

## How to use it

1. Deploy the shared factory on-chain.
2. Use the factory to deploy pair contracts from this pair template.
3. Have users approve the pair contract before adding liquidity or swapping.

## Notes

- This is a starter pair template, not a production-audited AMM.
- If you want full factory support, build a factory contract that deploys this pair template deterministically.
- For native LQD pools, use `"lqd"` as the token sentinel.
