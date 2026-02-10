package main

import (
	"math/big"
	"strings"

	blockchaincomponent "github.com/Zotish/DefenceProject/BlockchainComponent"
)

type DAOTreasury struct{}

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

func (d *DAOTreasury) Init(ctx *blockchaincomponent.Context, name string) {
	if name == "" {
		name = "DAO Treasury"
	}
	ctx.Set("name", name)
	ctx.Set("treasury", "0")
	ctx.Emit("Init", map[string]interface{}{"name": name})
}

func (d *DAOTreasury) Name(ctx *blockchaincomponent.Context) {
	ctx.Set("output", ctx.Get("name"))
}

func (d *DAOTreasury) Deposit(ctx *blockchaincomponent.Context, amount string) {
	amt := parseBig(amount)
	if amt.Sign() == 0 {
		ctx.Revert("invalid deposit amount")
	}
	cur := parseBig(ctx.Get("treasury"))
	ctx.Set("treasury", new(big.Int).Add(cur, amt).String())
	ctx.Emit("Deposit", map[string]interface{}{
		"from":   ctx.CallerAddr,
		"amount": amt.String(),
	})
}

func (d *DAOTreasury) Withdraw(ctx *blockchaincomponent.Context, to string, amount string) {
	if ctx.CallerAddr != ctx.OwnerAddr {
		ctx.Revert("only owner can withdraw")
	}
	amt := parseBig(amount)
	if amt.Sign() == 0 {
		ctx.Revert("invalid withdraw amount")
	}
	cur := parseBig(ctx.Get("treasury"))
	if cur.Cmp(amt) < 0 {
		ctx.Revert("insufficient treasury")
	}
	ctx.Set("treasury", new(big.Int).Sub(cur, amt).String())
	ctx.Emit("Withdraw", map[string]interface{}{
		"to":     to,
		"amount": amt.String(),
	})
}

func (d *DAOTreasury) Treasury(ctx *blockchaincomponent.Context) {
	ctx.Set("output", ctx.Get("treasury"))
}

// REQUIRED EXPORT
var Contract = &DAOTreasury{}
