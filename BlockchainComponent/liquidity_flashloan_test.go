package blockchaincomponent

import (
	"math"
	"testing"
)

func TestOneBlockFlashShockHasBoundedTWAPInfluence(t *testing.T) {
	bc := &Blockchain_struct{}
	bc.EnsureRuntimeState()
	pair := "0x1111111111111111111111111111111111111111"
	bc.PoolPriceHistory[pair] = []PoolPriceObservation{{PairAddress: pair, Price: 1, Timestamp: 0}, {PairAddress: pair, Price: 100, Timestamp: 3598}, {PairAddress: pair, Price: 1, Timestamp: 3600}}
	twap := bc.PoolTWAP(pair, 3600, 3600)
	expected := (3598.0 + 200.0) / 3600.0
	if math.Abs(twap-expected) > 1e-9 || twap > 1.06 {
		t.Fatalf("one-block shock influenced TWAP too much: got=%f expected=%f", twap, expected)
	}
}

func TestFlashLiquidityDoesNotRemoveManipulationCost(t *testing.T) {
	shallow := EstimateConstantProductManipulationCost(100_000, 2000, 30, 20_000)
	deep := EstimateConstantProductManipulationCost(10_000_000, 2000, 30, 20_000)
	if shallow.RequiredInputUSD <= 0 || deep.RequiredInputUSD < shallow.RequiredInputUSD*99 {
		t.Fatalf("cost does not scale with depth: shallow=%+v deep=%+v", shallow, deep)
	}
	if deep.Feasible || shallow.RoundTripLossUSD <= 0 {
		t.Fatalf("flash-loan feasibility/cost invalid: shallow=%+v deep=%+v", shallow, deep)
	}
}
