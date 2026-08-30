package blockchaincomponent

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestPersistentBlockchainSnapshotBoundsHistoricalCaches(t *testing.T) {
	bc := newTestBlockchain()
	for i := 1; i <= persistedBlockWindow+3; i++ {
		bc.Blocks = append(bc.Blocks, &Block{BlockNumber: uint64(i), CurrentHash: fmt.Sprintf("block-%d", i)})
	}
	for i := 0; i < persistedRewardHistoryWindow+3; i++ {
		bc.RewardHistory = append(bc.RewardHistory, RewardSnapshot{BlockNumber: uint64(i)})
	}
	for i := 0; i < persistedRewardLedgerWindow+3; i++ {
		bc.RewardLedger = append(bc.RewardLedger, RewardLedgerEntry{ID: fmt.Sprintf("ledger-%d", i)})
	}
	for i := 0; i < persistedRecentTxWindow+3; i++ {
		bc.RecentTxs = append(bc.RecentTxs, &Transaction{TxHash: fmt.Sprintf("recent-%d", i)})
	}

	data, err := marshalPersistentBlockchainState(bc)
	if err != nil {
		t.Fatal(err)
	}
	var stored Blockchain_struct
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatal(err)
	}
	if len(stored.Blocks) != persistedBlockWindow {
		t.Fatalf("stored blocks = %d, want %d", len(stored.Blocks), persistedBlockWindow)
	}
	if len(stored.RewardHistory) != persistedRewardHistoryWindow {
		t.Fatalf("stored reward history = %d, want %d", len(stored.RewardHistory), persistedRewardHistoryWindow)
	}
	if len(stored.RewardLedger) != persistedRewardLedgerWindow {
		t.Fatalf("stored reward ledger = %d, want %d", len(stored.RewardLedger), persistedRewardLedgerWindow)
	}
	if len(stored.RecentTxs) != persistedRecentTxWindow {
		t.Fatalf("stored recent txs = %d, want %d", len(stored.RecentTxs), persistedRecentTxWindow)
	}
	if got, want := stored.Blocks[0].BlockNumber, bc.Blocks[len(bc.Blocks)-persistedBlockWindow].BlockNumber; got != want {
		t.Fatalf("stored block window starts at %d, want %d", got, want)
	}
	if got, want := stored.RewardLedger[0].ID, bc.RewardLedger[len(bc.RewardLedger)-persistedRewardLedgerWindow].ID; got != want {
		t.Fatalf("stored ledger window starts at %q, want %q", got, want)
	}
	if got, want := stored.RecentTxs[0].TxHash, bc.RecentTxs[0].TxHash; got != want {
		t.Fatalf("stored recent window starts at %q, want %q", got, want)
	}
	if len(bc.Blocks) != persistedBlockWindow+4 || len(bc.RewardLedger) != persistedRewardLedgerWindow+3 {
		t.Fatal("persistent snapshot mutated in-memory history")
	}
}
