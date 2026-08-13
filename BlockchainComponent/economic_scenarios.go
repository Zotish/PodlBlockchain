package blockchaincomponent

import (
	"fmt"
	"math/big"
	"sort"
)

// EconomicScenarioInput is deliberately expressed in integer basis points.
// It is a reproducible stress tool, not a price forecast: assumptions and
// outputs remain explicit and no external market data is invented.
type EconomicScenarioInput struct {
	Years                     int    `json:"years"`
	Paths                     int    `json:"paths"`
	Seed                      uint64 `json:"seed"`
	InitialMonthlyRevenue     string `json:"initial_monthly_revenue"`
	InitialInsuranceReserve   string `json:"initial_insurance_reserve"`
	MonthlyRevenueGrowthBPS   int64  `json:"monthly_revenue_growth_bps"`
	RevenueVolatilityBPS      int64  `json:"revenue_volatility_bps"`
	MonthlyLossProbabilityBPS int64  `json:"monthly_loss_probability_bps"`
	LossSeverityRevenueBPS    int64  `json:"loss_severity_revenue_bps"`
}

type EconomicScenarioResult struct {
	Years                       int    `json:"years"`
	Paths                       int    `json:"paths"`
	EndingInsuranceP10          string `json:"ending_insurance_p10"`
	EndingInsuranceMedian       string `json:"ending_insurance_median"`
	EndingInsuranceP90          string `json:"ending_insurance_p90"`
	CumulativeRevenueMedian     string `json:"cumulative_revenue_median"`
	UncoveredLossProbabilityBPS int64  `json:"uncovered_loss_probability_bps"`
	RevenueReplacementMedianBPS int64  `json:"revenue_replacement_median_bps"`
	AssumptionsAreForecasts     bool   `json:"assumptions_are_forecasts"`
}

func (in EconomicScenarioInput) validate() error {
	if in.Years < 1 || in.Years > 10 || in.Paths < 1 || in.Paths > 10000 {
		return fmt.Errorf("scenario requires 1..10 years and 1..10000 paths")
	}
	for _, raw := range []string{in.InitialMonthlyRevenue, in.InitialInsuranceReserve} {
		if n, ok := new(big.Int).SetString(raw, 10); !ok || n.Sign() < 0 {
			return fmt.Errorf("scenario amounts must be non-negative integers")
		}
	}
	if in.MonthlyRevenueGrowthBPS < -5000 || in.MonthlyRevenueGrowthBPS > 5000 || in.RevenueVolatilityBPS < 0 || in.RevenueVolatilityBPS > 10000 || in.MonthlyLossProbabilityBPS < 0 || in.MonthlyLossProbabilityBPS > 10000 || in.LossSeverityRevenueBPS < 0 || in.LossSeverityRevenueBPS > 100000 {
		return fmt.Errorf("scenario basis-point assumption out of range")
	}
	return nil
}

type economicRNG struct{ state uint64 }

func (r *economicRNG) next() uint64 {
	x := r.state
	if x == 0 {
		x = 0x9e3779b97f4a7c15
	}
	x ^= x << 13
	x ^= x >> 7
	x ^= x << 17
	r.state = x
	return x
}
func (r *economicRNG) signedBPS(amplitude int64) int64 {
	if amplitude <= 0 {
		return 0
	}
	width := uint64(amplitude*2 + 1)
	return int64(r.next()%width) - amplitude
}

type economicPath struct {
	insurance, revenue *big.Int
	uncovered          bool
}

// SimulateEconomicScenarios runs deterministic seeded stress paths over the
// protocol waterfall. Slashing is excluded because it is not business income.
func SimulateEconomicScenarios(policy EconomicPolicy, input EconomicScenarioInput, scheduledIssuance *big.Int) (EconomicScenarioResult, error) {
	if err := policy.Validate(); err != nil {
		return EconomicScenarioResult{}, err
	}
	if err := input.validate(); err != nil {
		return EconomicScenarioResult{}, err
	}
	initialRevenue, _ := new(big.Int).SetString(input.InitialMonthlyRevenue, 10)
	initialInsurance, _ := new(big.Int).SetString(input.InitialInsuranceReserve, 10)
	rng := &economicRNG{state: input.Seed}
	paths := make([]economicPath, 0, input.Paths)
	uncovered := 0
	for pathIndex := 0; pathIndex < input.Paths; pathIndex++ {
		monthly, reserve, cumulative := new(big.Int).Set(initialRevenue), new(big.Int).Set(initialInsurance), big.NewInt(0)
		pathUncovered := false
		for month := 0; month < input.Years*12; month++ {
			growth := input.MonthlyRevenueGrowthBPS + rng.signedBPS(input.RevenueVolatilityBPS)
			factor := int64(10000) + growth
			if factor < 0 {
				factor = 0
			}
			monthly.Mul(monthly, big.NewInt(factor)).Div(monthly, big.NewInt(10000))
			cumulative.Add(cumulative, monthly)
			insuranceBPS := policy.InsuranceBPS
			if !policy.BuybackEnabled {
				insuranceBPS += policy.BuybackBPS
			}
			reserve.Add(reserve, new(big.Int).Div(new(big.Int).Mul(monthly, big.NewInt(insuranceBPS)), big.NewInt(10000)))
			if int64(rng.next()%10000) < input.MonthlyLossProbabilityBPS {
				loss := new(big.Int).Div(new(big.Int).Mul(monthly, big.NewInt(input.LossSeverityRevenueBPS)), big.NewInt(10000))
				if reserve.Cmp(loss) < 0 {
					reserve.SetInt64(0)
					pathUncovered = true
				} else {
					reserve.Sub(reserve, loss)
				}
			}
		}
		if pathUncovered {
			uncovered++
		}
		paths = append(paths, economicPath{insurance: new(big.Int).Set(reserve), revenue: cumulative, uncovered: pathUncovered})
	}
	sort.Slice(paths, func(i, j int) bool { return paths[i].insurance.Cmp(paths[j].insurance) < 0 })
	revenues := append([]economicPath(nil), paths...)
	sort.Slice(revenues, func(i, j int) bool { return revenues[i].revenue.Cmp(revenues[j].revenue) < 0 })
	percentile := func(rows []economicPath, bps int, insurance bool) *big.Int {
		index := (len(rows) - 1) * bps / 10000
		if insurance {
			return rows[index].insurance
		}
		return rows[index].revenue
	}
	medianRevenue := percentile(revenues, 5000, false)
	replacement := int64(0)
	if scheduledIssuance != nil && scheduledIssuance.Sign() > 0 {
		r := new(big.Int).Div(new(big.Int).Mul(medianRevenue, big.NewInt(10000)), scheduledIssuance)
		if r.IsInt64() {
			replacement = r.Int64()
		}
	}
	return EconomicScenarioResult{Years: input.Years, Paths: input.Paths, EndingInsuranceP10: percentile(paths, 1000, true).String(), EndingInsuranceMedian: percentile(paths, 5000, true).String(), EndingInsuranceP90: percentile(paths, 9000, true).String(), CumulativeRevenueMedian: medianRevenue.String(), UncoveredLossProbabilityBPS: int64(uncovered * 10000 / input.Paths), RevenueReplacementMedianBPS: replacement, AssumptionsAreForecasts: false}, nil
}
