package blockchaincomponent

import (
	"math/big"
	"reflect"
	"testing"
)

func TestUniformBatchIsOrderIndependentAndConservative(t *testing.T) {
	orders := []BatchSwapOrder{
		{Owner: "0x1111111111111111111111111111111111111111", TokenIn: "a", TokenOut: "b", AmountIn: big.NewInt(100), MinOut: big.NewInt(190), ValidFrom: 1, ExpiresAt: 10, Nonce: 1},
		{Owner: "0x2222222222222222222222222222222222222222", TokenIn: "a", TokenOut: "b", AmountIn: big.NewInt(50), MinOut: big.NewInt(90), ValidFrom: 1, ExpiresAt: 10, Nonce: 2},
		{Owner: "0x3333333333333333333333333333333333333333", TokenIn: "b", TokenOut: "a", AmountIn: big.NewInt(240), MinOut: big.NewInt(100), ValidFrom: 1, ExpiresAt: 10, Nonce: 3},
	}
	a, err := ClearUniformBatch("a", "b", big.NewInt(1_000), big.NewInt(2_000), 5, orders)
	if err != nil {
		t.Fatal(err)
	}
	reversed := []BatchSwapOrder{orders[2], orders[1], orders[0]}
	b, err := ClearUniformBatch("a", "b", big.NewInt(1_000), big.NewInt(2_000), 5, reversed)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("batch outcome depended on transaction ordering:\n%#v\n%#v", a, b)
	}
	if a.MatchedToken0.Cmp(big.NewInt(120)) != 0 || a.MatchedToken1.Cmp(big.NewInt(240)) != 0 || len(a.Fills) != 3 {
		t.Fatalf("unexpected conservative clearing: %#v", a)
	}
}

func TestUniformBatchRejectsExpiredDuplicateAndLimitViolation(t *testing.T) {
	base := BatchSwapOrder{Owner: "0x1111111111111111111111111111111111111111", TokenIn: "a", TokenOut: "b", AmountIn: big.NewInt(10), MinOut: big.NewInt(100), ValidFrom: 1, ExpiresAt: 2, Nonce: 1}
	base.ID = BatchOrderID(base)
	result, err := ClearUniformBatch("a", "b", big.NewInt(100), big.NewInt(100), 3, []BatchSwapOrder{base, base})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Fills) != 0 || len(result.Unfilled) != 2 {
		t.Fatalf("unsafe orders entered batch: %#v", result)
	}
}
