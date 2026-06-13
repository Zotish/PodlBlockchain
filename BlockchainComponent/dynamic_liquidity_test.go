package blockchaincomponent

import "testing"

func TestDynamicLiquidityOracleSignalSnapshot(t *testing.T) {
	oldPersist := persistRuntimeState
	persistRuntimeState = func(Blockchain_struct) error { return nil }
	defer func() { persistRuntimeState = oldPersist }()

	bc := &Blockchain_struct{}
	signal, err := bc.SetDynamicLiquidityOracleSignal("0xabc", 8200, "unit-test")
	if err != nil {
		t.Fatalf("set oracle signal failed: %v", err)
	}
	if signal.PairAddress != "0xabc" || signal.DemandBps != 8200 || signal.Source != "unit-test" {
		t.Fatalf("unexpected signal: %#v", signal)
	}

	got := bc.DynamicLiquidityOracleSnapshot()
	if len(got) != 1 {
		t.Fatalf("expected one signal, got %d", len(got))
	}
	if got[0].DemandBps != 8200 {
		t.Fatalf("expected demand 8200, got %d", got[0].DemandBps)
	}
}

func TestDynamicLiquiditySafetyCapsWeightStep(t *testing.T) {
	engine := NewDynamicLiquidityEngine()
	metrics := []PoolMetrics{{
		DemandWeight:          100,
		TimeMultiplier:        1,
		ExistingRoutingWeight: 20,
	}}
	engine.combineFinalWeights(metrics)
	if metrics[0].RoutingWeight != 45 {
		t.Fatalf("expected capped routing weight 45, got %d", metrics[0].RoutingWeight)
	}
	if !metrics[0].SafetyCapped {
		t.Fatal("expected safety cap flag")
	}
}
