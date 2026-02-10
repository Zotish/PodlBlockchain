package main

import (
	"math/big"
	"strings"

	blockchaincomponent "github.com/Zotish/DefenceProject/BlockchainComponent"
)

type LendingPool struct{}

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

func (l *LendingPool) Init(ctx *blockchaincomponent.Context, token string) {
	if token == "" {
		token = "LQD"
	}
	ctx.Set("token", token)
	ctx.Set("totalDeposits", "0")
	ctx.Set("totalBorrows", "0")
	ctx.Emit("PoolInit", map[string]interface{}{"token": token})
}

func (l *LendingPool) Deposit(ctx *blockchaincomponent.Context, amount string) {
	amt := parseBig(amount)
	if amt.Sign() == 0 {
		ctx.Revert("invalid deposit amount")
	}
	key := "dep:" + ctx.CallerAddr
	cur := parseBig(ctx.Get(key))
	total := parseBig(ctx.Get("totalDeposits"))
	ctx.Set(key, new(big.Int).Add(cur, amt).String())
	ctx.Set("totalDeposits", new(big.Int).Add(total, amt).String())
	ctx.Emit("Deposit", map[string]interface{}{
		"from":   ctx.CallerAddr,
		"amount": amt.String(),
	})
}

func (l *LendingPool) Withdraw(ctx *blockchaincomponent.Context, amount string) {
	amt := parseBig(amount)
	if amt.Sign() == 0 {
		ctx.Revert("invalid withdraw amount")
	}
	key := "dep:" + ctx.CallerAddr
	cur := parseBig(ctx.Get(key))
	if cur.Cmp(amt) < 0 {
		ctx.Revert("insufficient deposit")
	}
	total := parseBig(ctx.Get("totalDeposits"))
	ctx.Set(key, new(big.Int).Sub(cur, amt).String())
	ctx.Set("totalDeposits", new(big.Int).Sub(total, amt).String())
	ctx.Emit("Withdraw", map[string]interface{}{
		"to":     ctx.CallerAddr,
		"amount": amt.String(),
	})
}

func (l *LendingPool) Borrow(ctx *blockchaincomponent.Context, amount string) {
	amt := parseBig(amount)
	if amt.Sign() == 0 {
		ctx.Revert("invalid borrow amount")
	}
	dep := parseBig(ctx.Get("dep:" + ctx.CallerAddr))
	debtKey := "debt:" + ctx.CallerAddr
	debt := parseBig(ctx.Get(debtKey))

	// Simple 50% LTV
	maxBorrow := new(big.Int).Div(dep, big.NewInt(2))
	if new(big.Int).Add(debt, amt).Cmp(maxBorrow) > 0 {
		ctx.Revert("borrow limit exceeded")
	}

	total := parseBig(ctx.Get("totalBorrows"))
	ctx.Set(debtKey, new(big.Int).Add(debt, amt).String())
	ctx.Set("totalBorrows", new(big.Int).Add(total, amt).String())
	ctx.Emit("Borrow", map[string]interface{}{
		"borrower": ctx.CallerAddr,
		"amount":   amt.String(),
	})
}

func (l *LendingPool) Repay(ctx *blockchaincomponent.Context, amount string) {
	amt := parseBig(amount)
	if amt.Sign() == 0 {
		ctx.Revert("invalid repay amount")
	}
	debtKey := "debt:" + ctx.CallerAddr
	debt := parseBig(ctx.Get(debtKey))
	if debt.Sign() == 0 {
		ctx.Revert("no debt")
	}
	if amt.Cmp(debt) > 0 {
		amt = debt
	}
	total := parseBig(ctx.Get("totalBorrows"))
	ctx.Set(debtKey, new(big.Int).Sub(debt, amt).String())
	ctx.Set("totalBorrows", new(big.Int).Sub(total, amt).String())
	ctx.Emit("Repay", map[string]interface{}{
		"borrower": ctx.CallerAddr,
		"amount":   amt.String(),
	})
}

func (l *LendingPool) BalanceOf(ctx *blockchaincomponent.Context, addr string) {
	bal := ctx.Get("dep:" + addr)
	if bal == "" {
		bal = "0"
	}
	ctx.Set("output", bal)
}

func (l *LendingPool) DebtOf(ctx *blockchaincomponent.Context, addr string) {
	bal := ctx.Get("debt:" + addr)
	if bal == "" {
		bal = "0"
	}
	ctx.Set("output", bal)
}

func (l *LendingPool) TotalDeposits(ctx *blockchaincomponent.Context) {
	ctx.Set("output", ctx.Get("totalDeposits"))
}

func (l *LendingPool) TotalBorrows(ctx *blockchaincomponent.Context) {
	ctx.Set("output", ctx.Get("totalBorrows"))
}

// REQUIRED EXPORT
var Contract = &LendingPool{}
