package blockchaincomponent

import (
	"math/big"
	"strings"
	"testing"
)

func TestAccountBalanceCanonicalizesAddressCase(t *testing.T) {
	bc := &Blockchain_struct{Accounts: map[string]*big.Int{}}
	owner := "0xAD3606E1ddA48BAF4653d8185B667390a1a5D9c6"
	lower := strings.ToLower(owner)

	bc.addAccountBalance(owner, big.NewInt(100))
	bc.addAccountBalance(lower, big.NewInt(50))

	gotOwner, ok := bc.getAccountBalance(owner)
	if !ok {
		t.Fatal("expected owner balance")
	}
	gotLower, ok := bc.getAccountBalance(lower)
	if !ok {
		t.Fatal("expected lower-case owner balance")
	}
	if gotOwner.String() != "150" || gotLower.String() != "150" {
		t.Fatalf("expected merged balance 150, got owner=%s lower=%s", gotOwner, gotLower)
	}
	if len(bc.Accounts) != 1 {
		t.Fatalf("expected one canonical account entry, got %d: %#v", len(bc.Accounts), bc.Accounts)
	}
}
