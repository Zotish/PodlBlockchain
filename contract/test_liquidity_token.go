//go:build ignore
// +build ignore

package main

import (
	"math/big"
	"strings"

	bc "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/BlockchainComponent"
)

// TestLiquidityToken is a deterministic, faucet-capped token for isolated
// validator/localnet tests. It is deliberately not a production asset: every
// address may claim once and the total faucet budget is fixed at deployment.
type TestLiquidityToken struct{}

func tltAmount(raw string) *big.Int {
	z := new(big.Int)
	if _, ok := z.SetString(strings.TrimSpace(raw), 10); !ok || z.Sign() < 0 {
		return big.NewInt(-1)
	}
	return z
}

func tltAddress(raw string) string { return strings.ToLower(strings.TrimSpace(raw)) }

func (t *TestLiquidityToken) Init(ctx *bc.Context, faucetBudget string, claimAmount string) {
	if ctx.Get("initialized") == "true" {
		ctx.Revert("already initialized")
	}
	budget, claim := tltAmount(faucetBudget), tltAmount(claimAmount)
	if budget.Sign() <= 0 || claim.Sign() <= 0 || claim.Cmp(budget) > 0 {
		ctx.Revert("invalid faucet configuration")
	}
	ctx.Set("initialized", "true")
	ctx.Set("name", "PoDL Test Liquidity")
	ctx.Set("symbol", "tLQD-LP")
	ctx.Set("decimals", "18")
	ctx.Set("faucet_budget", budget.String())
	ctx.Set("claim_amount", claim.String())
	ctx.Set("total_supply", "0")
	ctx.Emit("TestLiquidityTokenInitialized", map[string]interface{}{"budget": budget.String(), "claimAmount": claim.String()})
}

func (t *TestLiquidityToken) Claim(ctx *bc.Context) {
	caller := tltAddress(ctx.CallerAddr)
	if caller == "" || ctx.Get("claimed:"+caller) == "true" {
		ctx.Revert("address already claimed or invalid")
	}
	claim, budget := tltAmount(ctx.Get("claim_amount")), tltAmount(ctx.Get("faucet_budget"))
	if claim.Sign() <= 0 || budget.Cmp(claim) < 0 {
		ctx.Revert("faucet exhausted")
	}
	ctx.Set("claimed:"+caller, "true")
	ctx.Set("faucet_budget", new(big.Int).Sub(budget, claim).String())
	ctx.Set("balance:"+caller, claim.String())
	ctx.Set("total_supply", new(big.Int).Add(tltAmount(ctx.Get("total_supply")), claim).String())
	ctx.Emit("TestLiquidityClaimed", map[string]interface{}{"account": caller, "amount": claim.String()})
}

func (t *TestLiquidityToken) Transfer(ctx *bc.Context, to string, amount string) {
	from, to, value := tltAddress(ctx.CallerAddr), tltAddress(to), tltAmount(amount)
	fromBalance := tltAmount(ctx.Get("balance:" + from))
	if to == "" || value.Sign() <= 0 || fromBalance.Cmp(value) < 0 {
		ctx.Revert("invalid transfer or insufficient balance")
	}
	ctx.Set("balance:"+from, new(big.Int).Sub(fromBalance, value).String())
	ctx.Set("balance:"+to, new(big.Int).Add(tltAmount(ctx.Get("balance:"+to)), value).String())
	ctx.Emit("Transfer", map[string]interface{}{"from": from, "to": to, "amount": value.String()})
}

func (t *TestLiquidityToken) BalanceOf(ctx *bc.Context, account string) {
	value := tltAmount(ctx.Get("balance:" + tltAddress(account)))
	if value.Sign() < 0 {
		value.SetInt64(0)
	}
	ctx.Set("output", value.String())
}

func (t *TestLiquidityToken) TotalSupply(ctx *bc.Context) { ctx.Set("output", ctx.Get("total_supply")) }
func (t *TestLiquidityToken) Name(ctx *bc.Context)        { ctx.Set("output", ctx.Get("name")) }
func (t *TestLiquidityToken) Symbol(ctx *bc.Context)      { ctx.Set("output", ctx.Get("symbol")) }
func (t *TestLiquidityToken) Decimals(ctx *bc.Context)    { ctx.Set("output", ctx.Get("decimals")) }

var Contract = &TestLiquidityToken{}
