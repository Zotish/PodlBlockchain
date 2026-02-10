package main

import (
	"math/big"
	"strings"

	blockchaincomponent "github.com/Zotish/DefenceProject/BlockchainComponent"
)

// Example minimal token
type LQDToken struct{}

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

func (c *LQDToken) ensureInit(ctx *blockchaincomponent.Context) {
	if ctx.Get("name") != "" {
		return
	}
	name := "test token"
	symbol := "test"
	decimals := "8"
	// 10,000,000 * 10^8 = 1,000,000,000,000,000
	totalSupply := "1000000000000000"

	ctx.Set("name", name)
	ctx.Set("symbol", symbol)
	ctx.Set("decimals", decimals)
	ctx.Set("totalSupply", totalSupply)
	ctx.Set("bal:"+ctx.OwnerAddr, totalSupply)

	ctx.Emit("Init", map[string]interface{}{
		"name":        name,
		"symbol":      symbol,
		"decimals":    decimals,
		"totalSupply": totalSupply,
	})
}

func (c *LQDToken) Init(ctx *blockchaincomponent.Context, name string, symbol string, supply string) {
	if name == "" || symbol == "" || supply == "" {
		c.ensureInit(ctx)
		return
	}
	ctx.Set("name", name)
	ctx.Set("symbol", symbol)
	ctx.Set("totalSupply", supply)
	ctx.Set("decimals", "8")

	ctx.Set("bal:"+ctx.OwnerAddr, supply)

	ctx.Emit("Init", map[string]interface{}{
		"name":   name,
		"symbol": symbol,
		"supply": supply,
	})
}

func (c *LQDToken) BalanceOf(ctx *blockchaincomponent.Context, addr string) {
	c.ensureInit(ctx)
	bal := ctx.Get("bal:" + addr)
	if bal == "" {
		bal = "0"
	}
	ctx.Set("output", bal)
}

func (c *LQDToken) Transfer(ctx *blockchaincomponent.Context, to string, amount string) {
	c.ensureInit(ctx)
	from := ctx.CallerAddr

	fromKey := "bal:" + from
	toKey := "bal:" + to

	fromBal := parseBig(ctx.Get(fromKey))
	amt := parseBig(amount)

	if fromBal.Cmp(amt) < 0 {
		ctx.Revert("insufficient balance")
	}

	ctx.Set(fromKey, new(big.Int).Sub(fromBal, amt).String())
	ctx.Set(toKey, new(big.Int).Add(parseBig(ctx.Get(toKey)), amt).String())

	ctx.Emit("Transfer", map[string]any{
		"from":   from,
		"to":     to,
		"amount": amount,
	})
}

func (c *LQDToken) Name(ctx *blockchaincomponent.Context) {
	c.ensureInit(ctx)
	ctx.Set("output", ctx.Get("name"))
}

func (c *LQDToken) Symbol(ctx *blockchaincomponent.Context) {
	c.ensureInit(ctx)
	ctx.Set("output", ctx.Get("symbol"))
}

func (c *LQDToken) Decimals(ctx *blockchaincomponent.Context) {
	c.ensureInit(ctx)
	ctx.Set("output", ctx.Get("decimals"))
}

func (c *LQDToken) TotalSupply(ctx *blockchaincomponent.Context) {
	c.ensureInit(ctx)
	ctx.Set("output", ctx.Get("totalSupply"))
}

// REQUIRED EXPORT
var Contract = &LQDToken{}
