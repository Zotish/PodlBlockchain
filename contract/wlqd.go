//go:build ignore
// +build ignore

package main

import (
	"math/big"
	"strings"

	bc "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/BlockchainComponent"
)

// ─────────────────────────────────────────────────────────────────────────────
// WLQD — Wrapped LQD
//
// Mirrors WETH on Ethereum: lets native LQD participate in DEX pairs.
//
//   Deposit(amount)   — send native LQD with the TX (msg.value), receive WLQD
//   Withdraw(amount)  — burn WLQD, receive native LQD back
//
// Full LQD20 interface so it works with the DEX Factory exactly like any token:
//   Transfer / TransferFrom / Approve / Allowance / BalanceOf / TotalSupply
//
// Usage flow:
//   1. User calls Deposit with value=500 LQD  →  gets 500 WLQD
//   2. User approves DEX Factory to spend WLQD
//   3. User calls Factory.AddLiquidity(WLQD, M2, 500, 500)
//   4. User calls Factory.SwapExactTokensForTokens(100, 0, WLQD, M2)
//   5. User calls Withdraw(500)  →  gets 500 native LQD back
// ─────────────────────────────────────────────────────────────────────────────

type WLQD struct{}

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

func normAddr(a string) string { return strings.ToLower(strings.TrimSpace(a)) }

// ─── Deposit ─────────────────────────────────────────────────────────────────

// Deposit wraps native LQD into WLQD.
// The caller must set value = amount in the transaction.
// args[0] = amount to deposit (must equal msg.value)
func (w *WLQD) Deposit(ctx *bc.Context, amount string) {
	amt := parseBig(amount)
	if amt.Sign() == 0 {
		ctx.Revert("deposit amount must be > 0")
	}

	msgVal := ctx.MsgValue()
	if msgVal.Cmp(amt) < 0 {
		ctx.Revert("msg.value less than deposit amount")
	}
	ctx.ReceiveNative(amt)

	caller := normAddr(ctx.CallerAddr)

	// Mint WLQD to caller
	balKey := "bal:" + caller
	newBal := new(big.Int).Add(parseBig(ctx.Get(balKey)), amt)
	ctx.Set(balKey, newBal.String())

	// Update total supply
	supply := parseBig(ctx.Get("totalSupply"))
	ctx.Set("totalSupply", new(big.Int).Add(supply, amt).String())

	ctx.Emit("Deposit", map[string]interface{}{
		"from":   caller,
		"amount": amt.String(),
	})
	ctx.Emit("Transfer", map[string]interface{}{
		"from":   "0x0000000000000000000000000000000000000000",
		"to":     caller,
		"amount": amt.String(),
	})
}

// ─── Withdraw ────────────────────────────────────────────────────────────────

// Withdraw burns WLQD and returns native LQD to the caller.
// args[0] = amount to withdraw
func (w *WLQD) Withdraw(ctx *bc.Context, amount string) {
	amt := parseBig(amount)
	if amt.Sign() == 0 {
		ctx.Revert("withdraw amount must be > 0")
	}

	caller := normAddr(ctx.CallerAddr)
	balKey := "bal:" + caller
	bal := parseBig(ctx.Get(balKey))
	if bal.Cmp(amt) < 0 {
		ctx.Revert("insufficient WLQD balance")
	}

	// Burn WLQD
	ctx.Set(balKey, new(big.Int).Sub(bal, amt).String())
	supply := parseBig(ctx.Get("totalSupply"))
	ctx.Set("totalSupply", new(big.Int).Sub(supply, amt).String())

	// Send native LQD back to caller
	ctx.SendNative(caller, amt)

	ctx.Emit("Withdrawal", map[string]interface{}{
		"to":     caller,
		"amount": amt.String(),
	})
	ctx.Emit("Transfer", map[string]interface{}{
		"from":   ctx.ContractAddr,
		"to":     "0x0000000000000000000000000000000000000000",
		"amount": amt.String(),
	})
}

// ─── LQD20 Standard Interface ─────────────────────────────────────────────────

func (w *WLQD) Name(ctx *bc.Context) {
	ctx.Set("output", "Wrapped LQD")
}

func (w *WLQD) Symbol(ctx *bc.Context) {
	ctx.Set("output", "WLQD")
}

func (w *WLQD) Decimals(ctx *bc.Context) {
	ctx.Set("output", "8")
}

func (w *WLQD) TotalSupply(ctx *bc.Context) {
	supply := ctx.Get("totalSupply")
	if supply == "" {
		supply = "0"
	}
	ctx.Set("output", supply)
}

func (w *WLQD) BalanceOf(ctx *bc.Context, addr string) {
	bal := ctx.Get("bal:" + normAddr(addr))
	if bal == "" {
		bal = "0"
	}
	ctx.Set("output", bal)
}

func (w *WLQD) Transfer(ctx *bc.Context, to string, amount string) {
	from := normAddr(ctx.CallerAddr)
	to = normAddr(to)
	amt := parseBig(amount)

	fromKey := "bal:" + from
	toKey := "bal:" + to
	fromBal := parseBig(ctx.Get(fromKey))

	if fromBal.Cmp(amt) < 0 {
		ctx.Revert("insufficient balance")
	}

	ctx.Set(fromKey, new(big.Int).Sub(fromBal, amt).String())
	ctx.Set(toKey, new(big.Int).Add(parseBig(ctx.Get(toKey)), amt).String())

	ctx.Emit("Transfer", map[string]interface{}{
		"from":   from,
		"to":     to,
		"amount": amt.String(),
	})
}

func (w *WLQD) Approve(ctx *bc.Context, spender string, amount string) {
	owner := normAddr(ctx.CallerAddr)
	spender = normAddr(spender)
	ctx.Set("allow:"+owner+":"+spender, amount)
	ctx.Emit("Approval", map[string]interface{}{
		"owner":   owner,
		"spender": spender,
		"amount":  amount,
	})
}

func (w *WLQD) Allowance(ctx *bc.Context, owner string, spender string) {
	val := ctx.Get("allow:" + normAddr(owner) + ":" + normAddr(spender))
	if val == "" {
		val = "0"
	}
	ctx.Set("output", val)
}

func (w *WLQD) TransferFrom(ctx *bc.Context, from string, to string, amount string) {
	spender := normAddr(ctx.CallerAddr)
	from = normAddr(from)
	to = normAddr(to)
	amt := parseBig(amount)

	allowKey := "allow:" + from + ":" + spender
	allow := parseBig(ctx.Get(allowKey))
	if allow.Cmp(amt) < 0 {
		ctx.Revert("allowance too low")
	}

	fromKey := "bal:" + from
	toKey := "bal:" + to
	fromBal := parseBig(ctx.Get(fromKey))
	if fromBal.Cmp(amt) < 0 {
		ctx.Revert("insufficient balance")
	}

	ctx.Set(fromKey, new(big.Int).Sub(fromBal, amt).String())
	ctx.Set(toKey, new(big.Int).Add(parseBig(ctx.Get(toKey)), amt).String())
	ctx.Set(allowKey, new(big.Int).Sub(allow, amt).String())

	ctx.Emit("Transfer", map[string]interface{}{
		"spender": spender,
		"from":    from,
		"to":      to,
		"amount":  amt.String(),
	})
}

// ─── View: native LQD balance held by this contract ──────────────────────────

// TotalReserve emits how much native LQD this contract holds (== totalSupply for WLQD).
func (w *WLQD) TotalReserve(ctx *bc.Context) {
	supply := ctx.Get("totalSupply")
	if supply == "" {
		supply = "0"
	}
	ctx.Set("output", supply)
	ctx.Emit("TotalReserve", map[string]interface{}{
		"reserve": supply,
	})
}

// REQUIRED EXPORT
var Contract = &WLQD{}
