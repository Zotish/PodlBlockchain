package blockchaincomponent

import (
	"math/big"
	"testing"
)

type ammPool struct {
	in  *big.Int
	out *big.Int
}

func testAmountOut(amountIn, reserveIn, reserveOut *big.Int) *big.Int {
	if amountIn.Sign() <= 0 || reserveIn.Sign() <= 0 || reserveOut.Sign() <= 0 {
		return big.NewInt(0)
	}
	amountInWithFee := new(big.Int).Mul(amountIn, big.NewInt(997))
	numerator := new(big.Int).Mul(amountInWithFee, reserveOut)
	denominator := new(big.Int).Add(new(big.Int).Mul(reserveIn, big.NewInt(1000)), amountInWithFee)
	return new(big.Int).Div(numerator, denominator)
}

func (p *ammPool) quote(amountIn *big.Int) *big.Int {
	return testAmountOut(amountIn, p.in, p.out)
}

func (p *ammPool) swap(amountIn, minOut *big.Int) (*big.Int, bool) {
	amountOut := p.quote(amountIn)
	if amountOut.Cmp(minOut) < 0 || amountOut.Sign() <= 0 || amountOut.Cmp(p.out) >= 0 {
		return big.NewInt(0), false
	}
	p.in = new(big.Int).Add(p.in, amountIn)
	p.out = new(big.Int).Sub(p.out, amountOut)
	return amountOut, true
}

func TestDEXMultiHopQuoteMatchesReserveMutation(t *testing.T) {
	hop1 := &ammPool{in: big.NewInt(1_000_000_000_000), out: big.NewInt(2_000_000_000_000)}
	hop2 := &ammPool{in: big.NewInt(2_000_000_000_000), out: big.NewInt(500_000_000_000)}
	amountIn := big.NewInt(10_000_000_000)

	quotedHop1 := hop1.quote(amountIn)
	quotedFinal := hop2.quote(quotedHop1)
	if quotedHop1.Sign() <= 0 || quotedFinal.Sign() <= 0 {
		t.Fatal("expected positive routed quote")
	}

	gotHop1, ok := hop1.swap(amountIn, big.NewInt(0))
	if !ok {
		t.Fatal("hop1 swap failed")
	}
	gotFinal, ok := hop2.swap(gotHop1, quotedFinal)
	if !ok {
		t.Fatal("hop2 swap failed")
	}
	if gotHop1.Cmp(quotedHop1) != 0 {
		t.Fatalf("hop1 quote mismatch: got %s want %s", gotHop1, quotedHop1)
	}
	if gotFinal.Cmp(quotedFinal) != 0 {
		t.Fatalf("final quote mismatch: got %s want %s", gotFinal, quotedFinal)
	}

	wantHop1In := new(big.Int).Add(big.NewInt(1_000_000_000_000), amountIn)
	wantHop1Out := new(big.Int).Sub(big.NewInt(2_000_000_000_000), quotedHop1)
	wantHop2In := new(big.Int).Add(big.NewInt(2_000_000_000_000), quotedHop1)
	wantHop2Out := new(big.Int).Sub(big.NewInt(500_000_000_000), quotedFinal)
	if hop1.in.Cmp(wantHop1In) != 0 || hop1.out.Cmp(wantHop1Out) != 0 {
		t.Fatalf("hop1 reserves mismatch: got %s/%s want %s/%s", hop1.in, hop1.out, wantHop1In, wantHop1Out)
	}
	if hop2.in.Cmp(wantHop2In) != 0 || hop2.out.Cmp(wantHop2Out) != 0 {
		t.Fatalf("hop2 reserves mismatch: got %s/%s want %s/%s", hop2.in, hop2.out, wantHop2In, wantHop2Out)
	}
}

func TestDEXMultiHopSlippageFailureKeepsReserves(t *testing.T) {
	hop := &ammPool{in: big.NewInt(1_000_000), out: big.NewInt(1_000_000)}
	beforeIn := new(big.Int).Set(hop.in)
	beforeOut := new(big.Int).Set(hop.out)
	quote := hop.quote(big.NewInt(10_000))
	tooHigh := new(big.Int).Add(quote, big.NewInt(1))

	if _, ok := hop.swap(big.NewInt(10_000), tooHigh); ok {
		t.Fatal("swap should fail when minOut exceeds quote")
	}
	if hop.in.Cmp(beforeIn) != 0 || hop.out.Cmp(beforeOut) != 0 {
		t.Fatalf("failed swap mutated reserves: got %s/%s want %s/%s", hop.in, hop.out, beforeIn, beforeOut)
	}
}
