package main

import (
	"fmt"
	"strconv"

	blockchaincomponent "github.com/Zotish/DefenceProject/BlockchainComponent"
)

// Example minimal token
type LQDToken struct{}

func ParseUint(s string) uint64 {
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0
	}
	return v
}

func (c *LQDToken) Init(ctx *blockchaincomponent.Context, name string, symbol string, supply string) {
	ctx.Set("name", name)
	ctx.Set("symbol", symbol)
	ctx.Set("totalSupply", supply)

	ctx.Set("bal:"+ctx.OwnerAddr, supply)

	ctx.Emit("Init", map[string]interface{}{
		"name":   name,
		"symbol": symbol,
		"supply": supply,
	})
}

func (c *LQDToken) BalanceOf(ctx *blockchaincomponent.Context, addr string) {
	bal := ctx.Get("bal:" + addr)
	if bal == "" {
		bal = "0"
	}
	ctx.Set("output", bal)
}

func (c *LQDToken) Transfer(ctx *blockchaincomponent.Context, to string, amount string) {
	from := ctx.CallerAddr

	fromKey := "bal:" + from
	toKey := "bal:" + to

	fromBal := ParseUint(ctx.Get(fromKey))
	amt := ParseUint(amount)

	if fromBal < amt {
		ctx.Revert("insufficient balance")
	}

	ctx.Set(fromKey, fmt.Sprintf("%d", fromBal-amt))
	ctx.Set(toKey, fmt.Sprintf("%d", ParseUint(ctx.Get(toKey))+amt))

	ctx.Emit("Transfer", map[string]any{
		"from":   from,
		"to":     to,
		"amount": amount,
	})
}

// REQUIRED EXPORT
var Contract = &LQDToken{}
