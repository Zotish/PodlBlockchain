package blockchaincomponent

import (
	"math/big"
	"reflect"
	"testing"
)

func TestEconomicScenarioDeterminismAndInsuranceStress(t *testing.T) {
	input := EconomicScenarioInput{Years: 10, Paths: 1000, Seed: 42, InitialMonthlyRevenue: "1000000", InitialInsuranceReserve: "5000000", MonthlyRevenueGrowthBPS: 50, RevenueVolatilityBPS: 500, MonthlyLossProbabilityBPS: 500, LossSeverityRevenueBPS: 50000}
	a, err := SimulateEconomicScenarios(DefaultEconomicPolicy(), input, big.NewInt(500000000))
	if err != nil {
		t.Fatal(err)
	}
	b, err := SimulateEconomicScenarios(DefaultEconomicPolicy(), input, big.NewInt(500000000))
	if err != nil || !reflect.DeepEqual(a, b) {
		t.Fatalf("scenario is not deterministic: a=%+v b=%+v err=%v", a, b, err)
	}
	p10, median, p90 := NewAmountFromStringOrZero(a.EndingInsuranceP10), NewAmountFromStringOrZero(a.EndingInsuranceMedian), NewAmountFromStringOrZero(a.EndingInsuranceP90)
	if p10.Cmp(median) > 0 || median.Cmp(p90) > 0 || a.UncoveredLossProbabilityBPS < 0 || a.UncoveredLossProbabilityBPS > 10000 || a.AssumptionsAreForecasts {
		t.Fatalf("invalid scenario distribution: %+v", a)
	}
}
