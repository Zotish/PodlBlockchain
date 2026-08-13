package main

import (
	"encoding/json"
	"fmt"
	"math/big"

	bc "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/BlockchainComponent"
)

func main() {
	policy := bc.DefaultEconomicPolicy()
	revenue := big.NewInt(1_000_000_000)
	allocation := func(bps int64) *big.Int {
		return new(big.Int).Div(new(big.Int).Mul(revenue, big.NewInt(bps)), big.NewInt(10000))
	}
	insurance := allocation(policy.InsuranceBPS + policy.BuybackBPS) // buyback disabled by default
	lp := allocation(policy.LPYieldBPS)
	operations := allocation(policy.OperationsBPS)
	total := new(big.Int).Add(new(big.Int).Add(insurance, lp), operations)
	chain := &bc.Blockchain_struct{}
	chain.EnsureRuntimeState()
	projection := chain.ProjectSupply(5, 2)
	stress, err := bc.SimulateEconomicScenarios(policy, bc.EconomicScenarioInput{
		Years: 10, Paths: 5000, Seed: 42,
		InitialMonthlyRevenue: "1000000000", InitialInsuranceReserve: "5000000000",
		MonthlyRevenueGrowthBPS: 50, RevenueVolatilityBPS: 500,
		MonthlyLossProbabilityBPS: 500, LossSeverityRevenueBPS: 50000,
	}, bc.NewAmountFromStringOrZero(projection.ScheduledIssuance))
	if err != nil {
		panic(err)
	}
	result := map[string]any{
		"realized_revenue":                     revenue.String(),
		"default_buyback_enabled":              policy.BuybackEnabled,
		"insurance_including_disabled_buyback": insurance.String(),
		"lp_real_yield":                        lp.String(),
		"operations":                           operations.String(),
		"five_year_supply_projection":          projection,
		"ten_year_seeded_stress":               stress,
	}
	raw, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(raw))
	if policy.Validate() != nil || total.Cmp(revenue) != 0 {
		panic("economic conservation invariant failed")
	}
}
