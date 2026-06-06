package blockchaincomponent

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	constantset "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/ConstantSet"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

func resetChainDBForTest(t *testing.T) {
	t.Helper()

	oldPath := constantset.BLOCKCHAIN_DB_PATH
	if dbInstance != nil {
		_ = dbInstance.Close()
	}

	constantset.BLOCKCHAIN_DB_PATH = filepath.Join(t.TempDir(), "evodb")
	dbOnce = sync.Once{}
	dbInstance = nil
	dbErr = nil

	t.Cleanup(func() {
		if dbInstance != nil {
			_ = dbInstance.Close()
		}
		constantset.BLOCKCHAIN_DB_PATH = oldPath
		dbOnce = sync.Once{}
		dbInstance = nil
		dbErr = nil
	})
}

func putTestBlock(t *testing.T, block *Block) {
	t.Helper()

	db, err := getDB()
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	data, err := json.Marshal(block)
	if err != nil {
		t.Fatalf("marshal block: %v", err)
	}
	if err := db.Put([]byte(fmt.Sprintf("block_%d", block.BlockNumber)), data, &opt.WriteOptions{Sync: true}); err != nil {
		t.Fatalf("put block: %v", err)
	}
}

func TestRepairChainDBMetadataDiscoversHighestPersistedBlock(t *testing.T) {
	resetChainDBForTest(t)

	blockOne := NewBlock(0, "0xgenesis")
	blockOne.CurrentHash = CalculateHash(&blockOne)
	blockTwo := NewBlock(blockOne.BlockNumber, blockOne.CurrentHash)
	blockTwo.CurrentHash = CalculateHash(&blockTwo)
	blockThree := NewBlock(blockTwo.BlockNumber, blockTwo.CurrentHash)
	blockThree.CurrentHash = CalculateHash(&blockThree)

	putTestBlock(t, &blockOne)
	putTestBlock(t, &blockTwo)
	putTestBlock(t, &blockThree)

	db, err := getDB()
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.Put([]byte("latest_block"), []byte("block_1"), nil); err != nil {
		t.Fatalf("write stale latest marker: %v", err)
	}
	staleMeta, _ := json.Marshal(ChainDBMetadata{SchemaVersion: chainDBSchemaVersion, LatestBlock: 1})
	if err := db.Put([]byte(chainDBMetaKey), staleMeta, nil); err != nil {
		t.Fatalf("write stale metadata: %v", err)
	}

	meta, err := RepairChainDBMetadata()
	if err != nil {
		t.Fatalf("repair metadata: %v", err)
	}
	if meta.LatestBlock != blockThree.BlockNumber {
		t.Fatalf("expected repaired latest block %d, got %d", blockThree.BlockNumber, meta.LatestBlock)
	}
	if meta.LatestHash != blockThree.CurrentHash {
		t.Fatalf("expected repaired latest hash %s, got %s", blockThree.CurrentHash, meta.LatestHash)
	}

	latest, err := GetLatestBlockNumberFromDB()
	if err != nil {
		t.Fatalf("latest block after repair: %v", err)
	}
	if latest != blockThree.BlockNumber {
		t.Fatalf("expected latest marker %d, got %d", blockThree.BlockNumber, latest)
	}
}

func TestTryFinalizePendingRejectsNonExtendingBlock(t *testing.T) {
	bc := newTestBlockchain()

	bad := NewBlock(99, "0xnot-the-tip")
	bad.CurrentHash = CalculateHash(&bad)
	bc.AddPendingBlock(&bad)
	bc.AddBlockVote(bad.CurrentHash, "0xlocal")

	if bc.TryFinalizePending(bad.CurrentHash, 0.67) {
		t.Fatalf("expected non-extending block to be rejected")
	}
	if _, ok := bc.PendingBlocks[bad.CurrentHash]; ok {
		t.Fatalf("expected rejected pending block to be pruned")
	}
	if got := bc.LatestBlockNumber(); got != 0 {
		t.Fatalf("expected chain tip to remain genesis, got %d", got)
	}
}

func TestHydrateInMemoryBlocksFromDBDoesNotRegressTip(t *testing.T) {
	resetChainDBForTest(t)

	blockOne := NewBlock(0, "0xgenesis")
	blockOne.CurrentHash = CalculateHash(&blockOne)
	putTestBlock(t, &blockOne)
	if _, err := RepairChainDBMetadata(); err != nil {
		t.Fatalf("repair metadata: %v", err)
	}

	bc := newTestBlockchain()
	blockTwo := NewBlock(0, bc.LatestBlockHash())
	blockTwo.CurrentHash = CalculateHash(&blockTwo)
	bc.Blocks = append(bc.Blocks, &blockTwo)

	changed, err := bc.RecoverInMemoryTipFromDB(32)
	if err != nil {
		t.Fatalf("recover tip: %v", err)
	}
	if changed {
		t.Fatalf("expected no recovery from lower DB tip")
	}
	if got := bc.LatestBlockNumber(); got != blockTwo.BlockNumber {
		t.Fatalf("expected memory tip %d to be preserved, got %d", blockTwo.BlockNumber, got)
	}
}

func TestActiveVotingSetSizeIgnoresOfflineRegisteredValidators(t *testing.T) {
	bc := newTestBlockchain()
	bc.Validators = []*Validator{
		{Address: "0x0000000000000000000000000000000000000001"},
		{Address: "0x0000000000000000000000000000000000000002"},
	}

	if got := bc.ActiveVotingSetSize(); got != 1 {
		t.Fatalf("expected only local validator in voting set without peers, got %d", got)
	}
}
