package blockchaincomponent

import (
	"math/big"
	"testing"
)

func TestChainSpecStableAndValid(t *testing.T) {
	spec := DefaultChainSpec("0xgenesis")
	if err := spec.Validate(); err != nil {
		t.Fatal(err)
	}
	if spec.Hash() != spec.Hash() {
		t.Fatal("chain spec hash must be deterministic")
	}
}

func TestDeterministicStateRootIgnoresMapInsertionOrder(t *testing.T) {
	a := newTestBlockchain()
	b := newTestBlockchain()
	a.Accounts["0xbb"] = big.NewInt(2)
	a.Accounts["0xaa"] = big.NewInt(1)
	b.Accounts["0xaa"] = big.NewInt(1)
	b.Accounts["0xbb"] = big.NewInt(2)
	if got, want := a.ComputeDeterministicStateRoot(), b.ComputeDeterministicStateRoot(); got != want {
		t.Fatalf("state roots differ: %s != %s", got, want)
	}
	b.Accounts["0xbb"] = big.NewInt(3)
	if a.ComputeDeterministicStateRoot() == b.ComputeDeterministicStateRoot() {
		t.Fatal("state root must change when consensus state changes")
	}
}
