//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"math/big"
	"strings"

	blockchaincomponent "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/BlockchainComponent"
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
	ctx.Set("interest_rate", "500") // 5% annual = 500 basis points
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

	// Record borrow time for interest accrual (store as string of int64)
	ctx.Set("borrow_time:"+ctx.CallerAddr, fmt.Sprintf("%d", ctx.BlockTime))

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

// calcAccruedInterest returns the accrued interest for addr given the current block time.
// Formula: principal * rate * elapsed_seconds / (10000 * 365 * 24 * 3600)
// rate is in basis points (500 = 5%).
func (l *LendingPool) calcAccruedInterest(ctx *blockchaincomponent.Context, addr string) *big.Int {
	principal := parseBig(ctx.Get("borrow:" + addr))
	if principal.Sign() == 0 {
		// Fall back to legacy debt key as well
		principal = parseBig(ctx.Get("debt:" + addr))
	}
	if principal.Sign() == 0 {
		return big.NewInt(0)
	}

	borrowTimeStr := ctx.Get("borrow_time:" + addr)
	if borrowTimeStr == "" {
		return big.NewInt(0)
	}

	var borrowTime int64
	fmt.Sscanf(borrowTimeStr, "%d", &borrowTime)

	elapsed := ctx.BlockTime - borrowTime
	if elapsed <= 0 {
		return big.NewInt(0)
	}

	rateStr := ctx.Get("interest_rate")
	if rateStr == "" {
		rateStr = "500"
	}
	rate := parseBig(rateStr)

	// interest = principal * rate * elapsed / (10000 * secondsPerYear)
	secondsPerYear := big.NewInt(365 * 24 * 3600)
	basisDivisor := big.NewInt(10000)
	divisor := new(big.Int).Mul(basisDivisor, secondsPerYear)

	interest := new(big.Int).Mul(principal, rate)
	interest.Mul(interest, big.NewInt(elapsed))
	interest.Div(interest, divisor)

	return interest
}

// AccrueInterest calculates and records the current accrued interest for the caller.
// It adds interest to the stored debt and resets the borrow timestamp to now.
func (l *LendingPool) AccrueInterest(ctx *blockchaincomponent.Context, addr string) {
	if addr == "" {
		addr = ctx.CallerAddr
	}

	interest := l.calcAccruedInterest(ctx, addr)
	if interest.Sign() == 0 {
		ctx.Set("output", "0")
		return
	}

	// Add interest to the principal stored under debt: key
	debtKey := "debt:" + addr
	oldDebt := parseBig(ctx.Get(debtKey))
	newDebt := new(big.Int).Add(oldDebt, interest)
	ctx.Set(debtKey, newDebt.String())

	// Also update borrow: key to stay in sync
	ctx.Set("borrow:"+addr, newDebt.String())

	// Reset borrow timestamp to now so interest is not double-counted
	ctx.Set("borrow_time:"+addr, fmt.Sprintf("%d", ctx.BlockTime))

	ctx.Set("output", interest.String())
	ctx.Emit("AccrueInterest", map[string]interface{}{
		"addr":     addr,
		"interest": interest.String(),
		"newDebt":  newDebt.String(),
	})
}

// GetDebt returns the total debt (principal + accrued interest) for an address.
// args[0] = address to query
func (l *LendingPool) GetDebt(ctx *blockchaincomponent.Context, addr string) {
	if addr == "" {
		addr = ctx.CallerAddr
	}

	principal := parseBig(ctx.Get("debt:" + addr))
	interest := l.calcAccruedInterest(ctx, addr)
	total := new(big.Int).Add(principal, interest)
	ctx.Set("output", total.String())
}

// healthFactor100 returns health factor * 100 for addr.
// health factor = (collateral * 100) / (debt * 120)
// A value >= 100 means the position is healthy (>= 120% collateral ratio).
func (l *LendingPool) healthFactor100(ctx *blockchaincomponent.Context, addr string) *big.Int {
	collateral := parseBig(ctx.Get("dep:" + addr))
	principal := parseBig(ctx.Get("debt:" + addr))
	interest := l.calcAccruedInterest(ctx, addr)
	totalDebt := new(big.Int).Add(principal, interest)

	if totalDebt.Sign() == 0 {
		// No debt — perfectly healthy; return a large sentinel value
		return big.NewInt(999999)
	}
	if collateral.Sign() == 0 {
		return big.NewInt(0)
	}

	// HF * 100 = (collateral * 10000) / (totalDebt * 120)
	// Using 120 as the liquidation threshold (120% collateral ratio)
	numerator := new(big.Int).Mul(collateral, big.NewInt(10000))
	denominator := new(big.Int).Mul(totalDebt, big.NewInt(120))
	return new(big.Int).Div(numerator, denominator)
}

// HealthFactor returns the health factor * 100 for an address.
// Values >= 100 are healthy. Values < 100 are subject to liquidation.
// args[0] = address to check
func (l *LendingPool) HealthFactor(ctx *blockchaincomponent.Context, addr string) {
	if addr == "" {
		addr = ctx.CallerAddr
	}
	hf := l.healthFactor100(ctx, addr)
	ctx.Set("output", hf.String())
}

// Liquidate liquidates an undercollateralised position.
// args[0] = borrower address to liquidate
// The caller receives the borrower's collateral (minus the debt they clear).
func (l *LendingPool) Liquidate(ctx *blockchaincomponent.Context, borrower string) {
	if borrower == "" {
		ctx.Revert("borrower address required")
	}

	hf := l.healthFactor100(ctx, borrower)
	// Health factor must be below 100 (i.e. collateral ratio < 120%)
	if hf.Cmp(big.NewInt(100)) >= 0 {
		ctx.Revert("position is healthy, cannot liquidate")
	}

	// Accrue interest before liquidating so debt is fully up to date
	interest := l.calcAccruedInterest(ctx, borrower)
	debtKey := "debt:" + borrower
	principal := parseBig(ctx.Get(debtKey))
	totalDebt := new(big.Int).Add(principal, interest)

	collateral := parseBig(ctx.Get("dep:" + borrower))

	// Liquidator receives the collateral
	liquidatorDepKey := "dep:" + ctx.CallerAddr
	liquidatorDep := parseBig(ctx.Get(liquidatorDepKey))
	ctx.Set(liquidatorDepKey, new(big.Int).Add(liquidatorDep, collateral).String())

	// Clear borrower's collateral and debt
	ctx.Set("dep:"+borrower, "0")
	ctx.Set(debtKey, "0")
	ctx.Set("borrow:"+borrower, "0")
	ctx.Set("borrow_time:"+borrower, "")

	// Update pool totals
	totalBorrows := parseBig(ctx.Get("totalBorrows"))
	if totalBorrows.Cmp(totalDebt) >= 0 {
		ctx.Set("totalBorrows", new(big.Int).Sub(totalBorrows, totalDebt).String())
	} else {
		ctx.Set("totalBorrows", "0")
	}

	ctx.Emit("Liquidate", map[string]interface{}{
		"liquidator": ctx.CallerAddr,
		"borrower":   borrower,
		"debt":       totalDebt.String(),
		"collateral": collateral.String(),
		"healthFactor": hf.String(),
	})
	ctx.Set("output", fmt.Sprintf("liquidated %s: debt=%s collateral=%s", borrower, totalDebt.String(), collateral.String()))
}

// REQUIRED EXPORT
var Contract = &LendingPool{}
