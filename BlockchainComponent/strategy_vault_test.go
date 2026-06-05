package blockchaincomponent

import (
	"math/big"
	"testing"
)

func TestStrategyVaultDepositRebalanceWithdraw(t *testing.T) {
	oldPersist := persistRuntimeState
	persistRuntimeState = func(Blockchain_struct) error { return nil }
	defer func() { persistRuntimeState = oldPersist }()

	bc := &Blockchain_struct{}
	owner := "0xad3606E1ddA48BAF4653d8185B667390a1a5D9c6"
	usdt := "0xba6048302d6a48ead56d0c4df0a0f9a0da4da814"

	pos, err := bc.StrategyVaultDeposit(owner, "LQD/USDT", "lqd", usdt, big.NewInt(100), big.NewInt(200))
	if err != nil {
		t.Fatalf("deposit failed: %v", err)
	}
	if pos.Status != "active" {
		t.Fatalf("expected active position, got %q", pos.Status)
	}
	if pos.Shares.String() != "300" {
		t.Fatalf("expected 300 shares, got %s", pos.Shares)
	}

	move, err := bc.StrategyVaultRebalance(pos.ID, "LQD/ETH", 9900, "price-gap")
	if err != nil {
		t.Fatalf("rebalance failed: %v", err)
	}
	if move.FromPool != "LQD/USDT" || move.ToPool != "LQD/ETH" {
		t.Fatalf("unexpected movement: %#v", move)
	}

	positions := bc.StrategyVaultStatus(owner)
	if len(positions) != 1 {
		t.Fatalf("expected one position, got %d", len(positions))
	}
	if positions[0].CurrentPool != "LQD/ETH" {
		t.Fatalf("expected LQD/ETH current pool, got %q", positions[0].CurrentPool)
	}

	withdrawn, err := bc.StrategyVaultWithdraw(pos.ID)
	if err != nil {
		t.Fatalf("withdraw failed: %v", err)
	}
	if withdrawn.Status != "withdrawn" {
		t.Fatalf("expected withdrawn status, got %q", withdrawn.Status)
	}
}
