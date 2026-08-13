package blockchaincomponent

import (
	"math/big"
	"testing"
)

func TestConsensusEmissionHardCap(t *testing.T) {
	bc := newTestBlockchain()
	bc.EnsureRuntimeState()
	bc.EconomicPolicy.IssuanceCap = "100"
	bc.CumulativeEmission = big.NewInt(90)
	if got := bc.takeCappedEmission(0); got.Cmp(big.NewInt(10)) != 0 {
		t.Fatalf("expected capped final emission 10, got %s", got)
	}
	if got := bc.takeCappedEmission(1); got.Sign() != 0 || bc.CumulativeEmission.Cmp(big.NewInt(100)) != 0 {
		t.Fatalf("emission continued after cap: reward=%s total=%s", got, bc.CumulativeEmission)
	}
}
