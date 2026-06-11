package blockchaincomponent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	constantset "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/ConstantSet"
	"github.com/syndtr/goleveldb/leveldb"
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

func putRawTestValue(t *testing.T, key string, value []byte) {
	t.Helper()

	db, err := getDB()
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.Put([]byte(key), value, &opt.WriteOptions{Sync: true}); err != nil {
		t.Fatalf("put raw value: %v", err)
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

func TestRepairChainDBMetadataIgnoresCorruptHighestBlock(t *testing.T) {
	resetChainDBForTest(t)

	blockOne := NewBlock(0, "0xgenesis")
	blockOne.CurrentHash = CalculateHash(&blockOne)
	blockTwo := NewBlock(blockOne.BlockNumber, blockOne.CurrentHash)
	blockTwo.CurrentHash = CalculateHash(&blockTwo)

	putTestBlock(t, &blockOne)
	putTestBlock(t, &blockTwo)
	putRawTestValue(t, "block_99", []byte("{not-json"))
	putRawTestValue(t, "latest_block", []byte("block_99"))
	staleMeta, _ := json.Marshal(ChainDBMetadata{SchemaVersion: chainDBSchemaVersion, LatestBlock: 99, LatestHash: "0xcorrupt"})
	putRawTestValue(t, chainDBMetaKey, staleMeta)

	meta, err := RepairChainDBMetadata()
	if err != nil {
		t.Fatalf("repair metadata: %v", err)
	}
	if meta.LatestBlock != blockTwo.BlockNumber {
		t.Fatalf("expected corrupt high tip to repair to block %d, got %d", blockTwo.BlockNumber, meta.LatestBlock)
	}
	if meta.LatestHash != blockTwo.CurrentHash {
		t.Fatalf("expected repaired hash %s, got %s", blockTwo.CurrentHash, meta.LatestHash)
	}

	latest, err := GetLatestBlockNumberFromDB()
	if err != nil {
		t.Fatalf("latest block after corrupt repair: %v", err)
	}
	if latest != blockTwo.BlockNumber {
		t.Fatalf("expected latest %d after corrupt repair, got %d", blockTwo.BlockNumber, latest)
	}
}

func TestChainDBSnapshotRestoreRoundTrip(t *testing.T) {
	resetChainDBForTest(t)

	blockOne := NewBlock(0, "0xgenesis")
	blockOne.CurrentHash = CalculateHash(&blockOne)
	blockTwo := NewBlock(blockOne.BlockNumber, blockOne.CurrentHash)
	blockTwo.CurrentHash = CalculateHash(&blockTwo)

	putTestBlock(t, &blockOne)
	putTestBlock(t, &blockTwo)
	if _, err := RepairChainDBMetadata(); err != nil {
		t.Fatalf("repair metadata before snapshot: %v", err)
	}

	manifest, snapshotDir, err := CreateChainDBSnapshot(filepath.Join(t.TempDir(), "snapshots"))
	if err != nil {
		t.Fatalf("create snapshot: %v", err)
	}
	if manifest.LatestBlock != blockTwo.BlockNumber || manifest.KeyCount == 0 || manifest.DataSHA256 == "" {
		t.Fatalf("unexpected snapshot manifest: %+v", manifest)
	}

	restorePath := filepath.Join(t.TempDir(), "restored-evodb")
	if err := RestoreChainDBSnapshotToPath(snapshotDir, restorePath, false); err != nil {
		t.Fatalf("restore snapshot: %v", err)
	}

	restoredDB, err := leveldb.OpenFile(restorePath, nil)
	if err != nil {
		t.Fatalf("open restored DB: %v", err)
	}
	defer restoredDB.Close()

	meta, err := getChainDBMetadataRaw(restoredDB)
	if err != nil {
		t.Fatalf("restored metadata: %v", err)
	}
	if meta.LatestBlock != blockTwo.BlockNumber || meta.LatestHash != blockTwo.CurrentHash {
		t.Fatalf("restored metadata mismatch: %+v", meta)
	}

	restoredBlock, err := loadBlockFromDBWithHandle(restoredDB, blockTwo.BlockNumber)
	if err != nil {
		t.Fatalf("restored latest block: %v", err)
	}
	if restoredBlock.CurrentHash != blockTwo.CurrentHash {
		t.Fatalf("restored block hash mismatch: got %s want %s", restoredBlock.CurrentHash, blockTwo.CurrentHash)
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

func TestTryFinalizePendingDoesNotAdvanceMemoryWhenPersistenceFails(t *testing.T) {
	resetChainDBForTest(t)

	if dbInstance != nil {
		_ = dbInstance.Close()
	}
	badPath := filepath.Join(t.TempDir(), "db-file")
	if err := os.WriteFile(badPath, []byte("not-a-leveldb-dir"), 0644); err != nil {
		t.Fatalf("write bad db path: %v", err)
	}
	constantset.BLOCKCHAIN_DB_PATH = badPath
	dbOnce = sync.Once{}
	dbInstance = nil
	dbErr = nil

	bc := newTestBlockchain()
	next := NewBlock(bc.LatestBlockNumber(), bc.LatestBlockHash())
	next.CurrentHash = CalculateHash(&next)
	bc.AddPendingBlock(&next)
	bc.AddBlockVote(next.CurrentHash, "0xlocal")

	if bc.TryFinalizePending(next.CurrentHash, 0.67) {
		t.Fatalf("expected finalize to fail when durable DB write fails")
	}
	if got := bc.LatestBlockNumber(); got != 0 {
		t.Fatalf("memory tip advanced despite DB write failure: got %d", got)
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

func TestEnsureMineableTipHydratesHigherDBTip(t *testing.T) {
	resetChainDBForTest(t)

	bc := newTestBlockchain()
	blockOne := NewBlock(bc.LatestBlockNumber(), bc.LatestBlockHash())
	blockOne.CurrentHash = CalculateHash(&blockOne)
	blockTwo := NewBlock(blockOne.BlockNumber, blockOne.CurrentHash)
	blockTwo.CurrentHash = CalculateHash(&blockTwo)

	putTestBlock(t, &blockOne)
	putTestBlock(t, &blockTwo)
	if _, err := RepairChainDBMetadata(); err != nil {
		t.Fatalf("repair metadata: %v", err)
	}

	bc.Blocks = bc.Blocks[:1]
	if !bc.EnsureMineableTip(8) {
		t.Fatalf("expected mineable tip recovery to succeed")
	}
	if got := bc.LatestBlockNumber(); got != blockTwo.BlockNumber {
		t.Fatalf("expected recovered tip %d, got %d", blockTwo.BlockNumber, got)
	}
	if gotHash := bc.LatestBlockHash(); gotHash != blockTwo.CurrentHash {
		t.Fatalf("expected recovered hash %s, got %s", blockTwo.CurrentHash, gotHash)
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
