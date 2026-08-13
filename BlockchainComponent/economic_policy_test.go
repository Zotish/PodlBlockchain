package blockchaincomponent

import (
	"math/big"
	"testing"
)

func TestProtocolRevenueWaterfallAndIdempotency(t *testing.T) {
	previousPersist := persistRuntimeState
	persistRuntimeState = func(*Blockchain_struct) error { return nil }
	defer func() { persistRuntimeState = previousPersist }()
	bc := &Blockchain_struct{}
	bc.EnsureRuntimeState()
	entry, err := bc.RecordProtocolRevenue("trading_fee", big.NewInt(10000), "tx-1", 100)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Allocations[EconomicLPYield] != "4500" || entry.Allocations[EconomicBuyback] != "0" || entry.Allocations[EconomicInsurance] != "3500" {
		t.Fatalf("unexpected guarded waterfall: %#v", entry.Allocations)
	}
	if _, err := bc.RecordProtocolRevenue("trading_fee", big.NewInt(10000), "tx-1", 101); err != nil {
		t.Fatal(err)
	}
	if len(bc.ProtocolRevenue) != 1 {
		t.Fatalf("duplicate revenue was recorded")
	}
}

func TestEconomicPolicyRequiresExactWaterfall(t *testing.T) {
	p := DefaultEconomicPolicy()
	p.LPYieldBPS++
	if p.Validate() == nil {
		t.Fatal("expected invalid bps total")
	}
}
