# PoDL Smart Contract Starter Template

This is a simple starter template for a Go plugin token contract.

Use it when you want to build a custom token, test token behavior, or learn the contract format used by PoDL.

## What you need

- Go 1.21+
- `package main`
- exported `var Contract = &YourType{}`
- methods that accept `*lqd-sdk-compat/context.Context`
- storage values encoded as decimal strings

## Starter token template

```go
package main

import (
	"math/big"
	"strings"

	lqdctx "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/lqd-sdk-compat/context"
)

type StarterToken struct{}

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

func (t *StarterToken) Init(ctx *lqdctx.Context, name string, symbol string, supply string) {
	if name == "" {
		name = "Starter Token"
	}
	if symbol == "" {
		symbol = "STT"
	}
	if supply == "" {
		supply = "1000000000000000"
	}

	ctx.Set("name", name)
	ctx.Set("symbol", symbol)
	ctx.Set("decimals", "8")
	ctx.Set("totalSupply", supply)
	ctx.Set("bal:"+normAddr(ctx.OwnerAddr), supply)
}

func (t *StarterToken) Name(ctx *lqdctx.Context) {
	ctx.Set("output", ctx.Get("name"))
}

func (t *StarterToken) Symbol(ctx *lqdctx.Context) {
	ctx.Set("output", ctx.Get("symbol"))
}

func (t *StarterToken) Decimals(ctx *lqdctx.Context) {
	ctx.Set("output", ctx.Get("decimals"))
}

func (t *StarterToken) TotalSupply(ctx *lqdctx.Context) {
	ctx.Set("output", ctx.Get("totalSupply"))
}

func (t *StarterToken) BalanceOf(ctx *lqdctx.Context, addr string) {
	bal := ctx.Get("bal:" + normAddr(addr))
	if bal == "" {
		bal = "0"
	}
	ctx.Set("output", bal)
}

func (t *StarterToken) Transfer(ctx *lqdctx.Context, to string, amount string) {
	from := normAddr(ctx.CallerAddr)
	to = normAddr(to)

	amt := parseBig(amount)
	fromBal := parseBig(ctx.Get("bal:" + from))
	if fromBal.Cmp(amt) < 0 {
		ctx.Revert("insufficient balance")
	}

	ctx.Set("bal:"+from, new(big.Int).Sub(fromBal, amt).String())
	ctx.Set("bal:"+to, new(big.Int).Add(parseBig(ctx.Get("bal:"+to)), amt).String())
}

func (t *StarterToken) Approve(ctx *lqdctx.Context, spender string, amount string) {
	owner := normAddr(ctx.CallerAddr)
	spender = normAddr(spender)
	ctx.Set("allow:"+owner+":"+spender, amount)
}

func (t *StarterToken) Allowance(ctx *lqdctx.Context, owner string, spender string) {
	val := ctx.Get("allow:" + normAddr(owner) + ":" + normAddr(spender))
	if val == "" {
		val = "0"
	}
	ctx.Set("output", val)
}

func (t *StarterToken) TransferFrom(ctx *lqdctx.Context, from string, to string, amount string) {
	spender := normAddr(ctx.CallerAddr)
	from = normAddr(from)
	to = normAddr(to)

	allowKey := "allow:" + from + ":" + spender
	allow := parseBig(ctx.Get(allowKey))
	amt := parseBig(amount)
	if allow.Cmp(amt) < 0 {
		ctx.Revert("allowance too low")
	}

	fromBal := parseBig(ctx.Get("bal:" + from))
	if fromBal.Cmp(amt) < 0 {
		ctx.Revert("insufficient balance")
	}

	ctx.Set("bal:"+from, new(big.Int).Sub(fromBal, amt).String())
	ctx.Set("bal:"+to, new(big.Int).Add(parseBig(ctx.Get("bal:"+to)), amt).String())
	ctx.Set(allowKey, new(big.Int).Sub(allow, amt).String())
}

var Contract = &StarterToken{}
```

## How to use it

1. Paste the template into the explorer contract compiler.
2. Compile as `Go Plugin (.so)`.
3. Deploy with a connected wallet.
4. Import the deployed token address into the DEX.
5. Approve the pair contract when adding liquidity or swapping.

## Notes

- This is a starter template, not a final audited token.
- Keep supply, decimals, and mint rules aligned with your project economics.
- If you want a mintable/burnable/governed token, add those methods carefully and audit the access control.
