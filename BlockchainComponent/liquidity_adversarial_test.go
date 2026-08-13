package blockchaincomponent

import (
	"strings"
	"testing"
)

func TestOrganicFlowRejectsSameEntityCircularAndCheapVolume(t *testing.T) {
	flows := []LiquidityFlow{
		{TxID: "self", From: "0xa", To: "0xb", FromCluster: "owner-1", ToCluster: "owner-1", AmountUSD: 1_000, FeeUSD: .1, Timestamp: 1},
		{TxID: "round-a", From: "0xc", To: "0xd", AmountUSD: 2_000, FeeUSD: 1, Timestamp: 2},
		{TxID: "round-b", From: "0xd", To: "0xc", AmountUSD: 1_990, FeeUSD: 1, Timestamp: 3},
		{TxID: "organic", From: "0xe", To: "0xf", AmountUSD: 3_000, FeeUSD: 3, Timestamp: 4},
	}
	result := AssessOrganicFlow(flows, 60, 5)
	if result.WashVolumeUSD != 1_000 || result.CircularVolumeUSD != 3_990 || result.OrganicVolumeUSD != 3_000 {
		t.Fatalf("flow classification failed: %#v", result)
	}
	if result.OrganicBPS <= 0 || result.OrganicBPS >= 10_000 || len(result.Flags) < 2 {
		t.Fatalf("risk signals missing: %#v", result)
	}
}

func TestLiquidityBacktestCSVAndOptimizer(t *testing.T) {
	csv := `depth,demand,volatility,oracle,concentration,manipulated
0.9,0.9,0.9,0.9,0.9,false
0.8,0.8,0.8,0.9,0.8,false
0.9,0.9,0.1,0.1,0.1,true
0.8,0.8,0.2,0.1,0.1,true
`
	points, err := LoadLiquidityBacktestCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatal(err)
	}
	result := OptimizeLiquidityWeights(points, .6)
	if !result.Weights.Valid() || result.Samples != 4 || result.AttackAcceptance != 0 || result.OrganicRejection != 0 {
		t.Fatalf("optimizer failed to separate synthetic attack data: %#v", result)
	}
}

func TestLiquidityAttackCostSecurityGate(t *testing.T) {
	weak := EstimateLiquidityAttackCost(100, 100, 1, 30, 10, .5, 1_000, .5)
	if weak.MeetsSecurityTarget {
		t.Fatal("under-collateralized attack incorrectly passed security target")
	}
	strong := EstimateLiquidityAttackCost(1_000, 2_000, 3, 30, 500, 1, 1_000, 1)
	if !strong.MeetsSecurityTarget || strong.TotalUSD <= 3_000 {
		t.Fatalf("strong attack budget not measured: %#v", strong)
	}
}
