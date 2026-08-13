package blockchaincomponent

import (
	"math/big"
	"testing"
)

func TestConcentratedPositionOwnershipFeesAndRemoval(t *testing.T) {
	pool := &ConcentratedPool{SqrtPriceX18: new(big.Int).Set(ammScaleX18), Liquidity: big.NewInt(0), FeeBPS: 30}
	lower := new(big.Int).Div(new(big.Int).Set(ammScaleX18), big.NewInt(2))
	upper := new(big.Int).Mul(new(big.Int).Set(ammScaleX18), big.NewInt(2))
	if err := pool.AddPosition("p1", "alice", lower, upper, big.NewInt(1_000_000)); err != nil {
		t.Fatal(err)
	}
	if err := pool.AddPosition("p2", "bob", lower, upper, big.NewInt(1_000_000)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Swap(big.NewInt(10_000), false); err != nil {
		t.Fatal(err)
	}
	a0, a1, err := pool.CollectPositionFees("p1", "alice")
	if err != nil {
		t.Fatal(err)
	}
	b0, b1, err := pool.CollectPositionFees("p2", "bob")
	if err != nil {
		t.Fatal(err)
	}
	if a0.Sign() != 0 || b0.Sign() != 0 || a1.Sign() <= 0 || a1.Cmp(b1) != 0 {
		t.Fatalf("position fees not proportional: a=%s/%s b=%s/%s", a0, a1, b0, b1)
	}
	if err := pool.TransferPosition("p1", "alice", "carol"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.RemovePosition("p1", "alice", big.NewInt(100)); err == nil {
		t.Fatal("former owner removed position")
	}
	if _, err := pool.RemovePosition("p1", "carol", big.NewInt(1_000_000)); err != nil {
		t.Fatal(err)
	}
}

func TestStableDepegPolicyRaisesFeeAndCapsEmergencySwap(t *testing.T) {
	policy := StablePoolRiskPolicy{BaseFeeBPS: 4, DepegSurchargeBPS: 50, WarningDeviationBPS: 100, EmergencyDeviationBPS: 1000, EmergencyMaxSwapBPS: 75}
	fee, capBPS, emergency := policy.EffectiveFeeAndCap(1, .995)
	if fee != 4 || capBPS != 10000 || emergency {
		t.Fatalf("healthy stable pair restricted: %d %d %v", fee, capBPS, emergency)
	}
	fee, capBPS, emergency = policy.EffectiveFeeAndCap(1, .8)
	if fee != 54 || capBPS != 75 || !emergency {
		t.Fatalf("depeg mode not activated: %d %d %v", fee, capBPS, emergency)
	}
}
