package blockchaincomponent

import (
	"math/big"
	"testing"
)

func TestStableSwapCorrelatedAssetsHasLowerSlippage(t *testing.T) {
	reserve := big.NewInt(1_000_000_000)
	in := big.NewInt(10_000_000)
	out, err := StableSwapAmountOut(in, reserve, reserve, 100, 4)
	if err != nil {
		t.Fatal(err)
	}
	volatile := new(big.Int).Div(new(big.Int).Mul(new(big.Int).Mul(in, big.NewInt(9970)), reserve), new(big.Int).Add(new(big.Int).Mul(reserve, big.NewInt(10000)), new(big.Int).Mul(in, big.NewInt(9970))))
	if out.Cmp(volatile) <= 0 || out.Cmp(reserve) >= 0 {
		t.Fatalf("unexpected stable output stable=%s volatile=%s", out, volatile)
	}
}

func TestConcentratedPoolCrossesInitializedTick(t *testing.T) {
	scale := new(big.Int).Set(ammScaleX18)
	p := &ConcentratedPool{SqrtPriceX18: new(big.Int).Set(scale), Liquidity: big.NewInt(0), FeeBPS: 5}
	if err := p.AddRange(new(big.Int).Div(new(big.Int).Mul(scale, big.NewInt(9)), big.NewInt(10)), new(big.Int).Div(new(big.Int).Mul(scale, big.NewInt(11)), big.NewInt(10)), big.NewInt(1_000_000_000)); err != nil {
		t.Fatal(err)
	}
	out, err := p.Swap(big.NewInt(1_000_000), true)
	if err != nil || out.Sign() <= 0 || p.SqrtPriceX18.Cmp(scale) >= 0 {
		t.Fatalf("concentrated swap failed out=%v price=%v err=%v", out, p.SqrtPriceX18, err)
	}
}

func FuzzStableSwapInvariant(f *testing.F) {
	f.Add(uint64(1_000_000), uint64(1_000_000), uint64(10_000), int64(100))
	f.Fuzz(func(t *testing.T, x, y, amount uint64, amplification int64) {
		x, y, amount = x%1_000_000_000+10_000, y%1_000_000_000+10_000, amount%100_000+1
		amplification = amplification%10_000 + 1
		if amplification < 1 {
			amplification = 1
		}
		out, err := StableSwapAmountOut(new(big.Int).SetUint64(amount), new(big.Int).SetUint64(x), new(big.Int).SetUint64(y), amplification, 4)
		if err == nil && (out.Sign() <= 0 || out.Cmp(new(big.Int).SetUint64(y)) >= 0) {
			t.Fatalf("unsafe stable output %s", out)
		}
	})
}
