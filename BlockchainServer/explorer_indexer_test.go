package blockchainserver

import (
	"math/big"
	"testing"

	bc "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/BlockchainComponent"
)

func TestExplorerIndexerPersistentSearchAndReorgRebuild(t *testing.T) {
	path := t.TempDir() + "/index"
	index, err := OpenExplorerIndexer(path)
	if err != nil {
		t.Fatal(err)
	}
	from, to := "0x1111111111111111111111111111111111111111", "0x2222222222222222222222222222222222222222"
	tx := &bc.Transaction{From: from, To: to, Value: big.NewInt(99), TxHash: "0xabc", Timestamp: 10, Status: "success", Type: "transfer"}
	chain := &bc.Blockchain_struct{Blocks: []*bc.Block{{BlockNumber: 1, CurrentHash: "0xblock1", TimeStamp: 10, StateRoot: "0xstate", Transactions: []*bc.Transaction{tx}}}}
	if err = index.Sync(chain); err != nil {
		t.Fatal(err)
	}
	result, err := index.Search(from, 10)
	if err != nil {
		t.Fatal(err)
	}
	rows := result.(map[string]interface{})["transactions"].([]ExplorerIndexedTransaction)
	if len(rows) != 1 || rows[0].Hash != "0xabc" {
		t.Fatalf("address index mismatch: %+v", result)
	}
	if err = index.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenExplorerIndexer(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if reopened.Status()["indexed_height"].(uint64) != 1 {
		t.Fatal("persistent index checkpoint missing")
	}
	chain.Blocks = []*bc.Block{{BlockNumber: 1, CurrentHash: "0xreplacement", TimeStamp: 11, Transactions: []*bc.Transaction{}}}
	if err = reopened.Sync(chain); err != nil {
		t.Fatal(err)
	}
	if _, err = reopened.Search("0xabc", 10); err == nil {
		t.Fatal("reorg rebuild retained orphan transaction")
	}
}
