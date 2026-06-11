package blockchaincomponent

import (
	"bufio"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	constantset "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/ConstantSet"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"
)

var (
	dbOnce     sync.Once
	dbInstance *leveldb.DB
	dbErr      error
)

const (
	chainDBMetaKey       = "chain_meta"
	chainDBSchemaVersion = 2
	chainDBSnapshotFmt   = "podl-leveldb-jsonl-v1"
)

type ChainDBMetadata struct {
	SchemaVersion int    `json:"schema_version"`
	LatestBlock   uint64 `json:"latest_block"`
	LatestHash    string `json:"latest_hash"`
	UpdatedAt     int64  `json:"updated_at"`
	Format        string `json:"format,omitempty"`
	Compat        string `json:"compat,omitempty"`
}

type ChainDBContinuityReport struct {
	LatestBlock   uint64   `json:"latest_block"`
	CheckedFrom   uint64   `json:"checked_from"`
	CheckedTo     uint64   `json:"checked_to"`
	CheckedAt     int64    `json:"checked_at"`
	Valid         bool     `json:"valid"`
	MissingBlocks []uint64 `json:"missing_blocks,omitempty"`
	CorruptBlocks []uint64 `json:"corrupt_blocks,omitempty"`
}

type ChainDBSnapshotManifest struct {
	Format        string `json:"format"`
	SchemaVersion int    `json:"schema_version"`
	LatestBlock   uint64 `json:"latest_block"`
	LatestHash    string `json:"latest_hash"`
	ExportedAt    int64  `json:"exported_at"`
	KeyCount      uint64 `json:"key_count"`
	DataSHA256    string `json:"data_sha256"`
}

type chainDBSnapshotRecord struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func getDB() (*leveldb.DB, error) {
	dbOnce.Do(func() {
		if err := os.MkdirAll(filepath.Dir(constantset.BLOCKCHAIN_DB_PATH), 0755); err != nil {
			dbErr = fmt.Errorf("failed to create DB directory: %v", err)
			return
		}
		dbInstance, dbErr = leveldb.OpenFile(constantset.BLOCKCHAIN_DB_PATH, &opt.Options{
			NoSync:      false,
			WriteBuffer: 64 * opt.MiB,
		})
	})
	return dbInstance, dbErr
}

func metadataForBlock(block *Block) ChainDBMetadata {
	meta := ChainDBMetadata{
		SchemaVersion: chainDBSchemaVersion,
		UpdatedAt:     time.Now().Unix(),
		Format:        chainDBSnapshotFmt,
		Compat:        "append-only-blocks-v1",
	}
	if block != nil {
		meta.LatestBlock = block.BlockNumber
		meta.LatestHash = block.CurrentHash
	}
	return meta
}

func GetChainDBMetadata() (ChainDBMetadata, error) {
	db, err := getDB()
	if err != nil {
		return ChainDBMetadata{}, err
	}
	meta, err := getChainDBMetadataRaw(db)
	if err != nil || meta.SchemaVersion != chainDBSchemaVersion {
		return repairChainDBMetadataWithDB(db)
	}

	if markerLatest, markerErr := getLatestBlockMarkerFromDB(db); markerErr == nil && markerLatest > meta.LatestBlock {
		return repairChainDBMetadataWithDB(db)
	}

	return meta, nil
}

func getChainDBMetadataRaw(db *leveldb.DB) (ChainDBMetadata, error) {
	data, err := db.Get([]byte(chainDBMetaKey), nil)
	if err != nil {
		return ChainDBMetadata{}, err
	}
	var meta ChainDBMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return ChainDBMetadata{}, err
	}
	return meta, nil
}

func PutChainDBMetadata(meta ChainDBMetadata) error {
	db, err := getDB()
	if err != nil {
		return err
	}
	if meta.SchemaVersion == 0 {
		meta.SchemaVersion = chainDBSchemaVersion
	}
	meta.UpdatedAt = time.Now().Unix()
	data, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	return db.Put([]byte(chainDBMetaKey), data, &opt.WriteOptions{Sync: true})
}

func SaveBlockToDB(block *Block) error {
	db, err := getDB()
	if err != nil {
		return fmt.Errorf("failed to open block DB: %v", err)
	}
	if block == nil {
		return fmt.Errorf("cannot save nil block")
	}

	// Build block key
	blockKey := fmt.Sprintf("block_%d", block.BlockNumber)

	// Marshal block
	blockData, err := json.Marshal(block)
	if err != nil {
		return fmt.Errorf("failed to marshal block: %v", err)
	}

	// Use batch write
	batch := new(leveldb.Batch)
	batch.Put([]byte(blockKey), blockData)
	currentLatest, latestErr := GetLatestBlockNumberFromDB()
	if latestErr != nil || block.BlockNumber >= currentLatest {
		batch.Put([]byte("latest_block"), []byte(blockKey))
		metaData, metaErr := json.Marshal(metadataForBlock(block))
		if metaErr == nil {
			batch.Put([]byte(chainDBMetaKey), metaData)
		}
	}

	// Finalized block writes are consensus-critical; fsync prevents height
	// regression after process/container crashes.
	if err := db.Write(batch, &opt.WriteOptions{Sync: true}); err != nil {
		return fmt.Errorf("failed to write block batch: %v", err)
	}

	return nil
}

func parseLatestBlockMarker(raw []byte) (uint64, error) {
	key := strings.TrimSpace(string(raw))
	key = strings.TrimPrefix(key, "block_")
	if key == "" {
		return 0, fmt.Errorf("latest block key missing block number")
	}

	n, err := strconv.ParseUint(key, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid latest block number %q: %w", key, err)
	}
	return n, nil
}

func getLatestBlockMarkerFromDB(db *leveldb.DB) (uint64, error) {
	raw, err := db.Get([]byte("latest_block"), nil)
	if err != nil {
		return 0, err
	}
	return parseLatestBlockMarker(raw)
}

func discoverBlockNumbersFromDB(db *leveldb.DB) ([]uint64, error) {
	iter := db.NewIterator(util.BytesPrefix([]byte("block_")), nil)
	defer iter.Release()

	numbers := []uint64{}
	for iter.Next() {
		n, err := parseLatestBlockMarker(iter.Key())
		if err != nil {
			continue
		}
		numbers = append(numbers, n)
	}
	if err := iter.Error(); err != nil {
		return nil, err
	}
	sort.Slice(numbers, func(i, j int) bool { return numbers[i] > numbers[j] })
	return numbers, nil
}

func loadBlockFromDBWithHandle(db *leveldb.DB, blockNumber uint64) (*Block, error) {
	blockKey := fmt.Sprintf("block_%d", blockNumber)
	data, err := db.Get([]byte(blockKey), nil)
	if err != nil {
		return nil, err
	}

	var block Block
	if err := json.Unmarshal(data, &block); err != nil {
		return nil, err
	}
	if block.BlockNumber != blockNumber {
		return nil, fmt.Errorf("block key/number mismatch: key=%d block=%d", blockNumber, block.BlockNumber)
	}
	if strings.TrimSpace(block.CurrentHash) == "" {
		return nil, fmt.Errorf("block %d missing current hash", blockNumber)
	}
	return &block, nil
}

func discoverLatestUsableBlockFromDB(db *leveldb.DB) (*Block, error) {
	numbers, err := discoverBlockNumbersFromDB(db)
	if err != nil {
		return nil, err
	}
	for _, n := range numbers {
		blk, err := loadBlockFromDBWithHandle(db, n)
		if err == nil && blk != nil {
			return blk, nil
		}
	}
	return nil, nil
}

func repairChainDBMetadataWithDB(db *leveldb.DB) (ChainDBMetadata, error) {
	latestBlock, scanErr := discoverLatestUsableBlockFromDB(db)
	if scanErr != nil {
		return ChainDBMetadata{}, scanErr
	}

	// If block records exist, they are the canonical source of truth. Metadata
	// and latest_block can be stale/corrupt after interrupted deploys, so only
	// use them when no individual block records are present yet.
	latest := uint64(0)
	latestHash := ""
	if latestBlock != nil {
		latest = latestBlock.BlockNumber
		latestHash = latestBlock.CurrentHash
	} else {
		if markerLatest, err := getLatestBlockMarkerFromDB(db); err == nil && markerLatest > latest {
			latest = markerLatest
		}
		if meta, err := getChainDBMetadataRaw(db); err == nil && meta.LatestBlock > latest {
			latest = meta.LatestBlock
			latestHash = meta.LatestHash
		}
	}

	meta := ChainDBMetadata{
		SchemaVersion: chainDBSchemaVersion,
		LatestBlock:   latest,
		LatestHash:    latestHash,
		UpdatedAt:     time.Now().Unix(),
		Format:        chainDBSnapshotFmt,
		Compat:        "append-only-blocks-v1",
	}

	batch := new(leveldb.Batch)
	if latest > 0 {
		batch.Put([]byte("latest_block"), []byte(fmt.Sprintf("block_%d", latest)))
	}
	metaData, err := json.Marshal(meta)
	if err != nil {
		return ChainDBMetadata{}, err
	}
	batch.Put([]byte(chainDBMetaKey), metaData)

	if err := db.Write(batch, &opt.WriteOptions{Sync: true}); err != nil {
		return ChainDBMetadata{}, err
	}
	return meta, nil
}

func RepairChainDBMetadata() (ChainDBMetadata, error) {
	db, err := getDB()
	if err != nil {
		return ChainDBMetadata{}, err
	}
	return repairChainDBMetadataWithDB(db)
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	tmpPath := path + ".tmp"
	file, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func ValidateChainDBContinuity(limit uint64) (ChainDBContinuityReport, error) {
	db, err := getDB()
	if err != nil {
		return ChainDBContinuityReport{}, err
	}

	meta, err := repairChainDBMetadataWithDB(db)
	if err != nil {
		return ChainDBContinuityReport{}, err
	}

	latest := meta.LatestBlock
	if limit == 0 || limit > latest {
		limit = latest
	}
	from := uint64(1)
	if latest > 0 && limit > 0 && latest >= limit {
		from = latest - limit + 1
	}

	report := ChainDBContinuityReport{
		LatestBlock: latest,
		CheckedFrom: from,
		CheckedTo:   latest,
		CheckedAt:   time.Now().Unix(),
		Valid:       true,
	}

	var previous *Block
	for blockNumber := from; blockNumber <= latest && blockNumber > 0; blockNumber++ {
		key := []byte(fmt.Sprintf("block_%d", blockNumber))
		has, err := db.Has(key, nil)
		if err != nil {
			return report, err
		}
		if !has {
			report.Valid = false
			report.MissingBlocks = append(report.MissingBlocks, blockNumber)
			previous = nil
			continue
		}

		block, err := loadBlockFromDBWithHandle(db, blockNumber)
		if err != nil {
			report.Valid = false
			report.CorruptBlocks = append(report.CorruptBlocks, blockNumber)
			previous = nil
			continue
		}
		if previous != nil && block.PreviousHash != previous.CurrentHash {
			report.Valid = false
			report.CorruptBlocks = append(report.CorruptBlocks, blockNumber)
		}
		previous = block
	}

	return report, nil
}

func CreateChainDBSnapshot(snapshotRoot string) (ChainDBSnapshotManifest, string, error) {
	db, err := getDB()
	if err != nil {
		return ChainDBSnapshotManifest{}, "", err
	}

	meta, err := repairChainDBMetadataWithDB(db)
	if err != nil {
		return ChainDBSnapshotManifest{}, "", err
	}

	if strings.TrimSpace(snapshotRoot) == "" {
		snapshotRoot = filepath.Join(filepath.Dir(constantset.BLOCKCHAIN_DB_PATH), "snapshots")
	}
	snapshotDir := filepath.Join(snapshotRoot, fmt.Sprintf("chain-snapshot-%d-%d", meta.LatestBlock, time.Now().Unix()))
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		return ChainDBSnapshotManifest{}, "", err
	}

	dataPath := filepath.Join(snapshotDir, "leveldb.jsonl")
	tmpPath := dataPath + ".tmp"
	file, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return ChainDBSnapshotManifest{}, "", err
	}

	hash := sha256.New()
	encoder := json.NewEncoder(io.MultiWriter(file, hash))
	iter := db.NewIterator(nil, nil)
	keyCount := uint64(0)
	for iter.Next() {
		record := chainDBSnapshotRecord{
			Key:   base64.StdEncoding.EncodeToString(iter.Key()),
			Value: base64.StdEncoding.EncodeToString(iter.Value()),
		}
		if err := encoder.Encode(record); err != nil {
			iter.Release()
			file.Close()
			return ChainDBSnapshotManifest{}, "", err
		}
		keyCount++
	}
	if err := iter.Error(); err != nil {
		iter.Release()
		file.Close()
		return ChainDBSnapshotManifest{}, "", err
	}
	iter.Release()

	if err := file.Sync(); err != nil {
		file.Close()
		return ChainDBSnapshotManifest{}, "", err
	}
	if err := file.Close(); err != nil {
		return ChainDBSnapshotManifest{}, "", err
	}
	if err := os.Rename(tmpPath, dataPath); err != nil {
		return ChainDBSnapshotManifest{}, "", err
	}

	manifest := ChainDBSnapshotManifest{
		Format:        chainDBSnapshotFmt,
		SchemaVersion: chainDBSchemaVersion,
		LatestBlock:   meta.LatestBlock,
		LatestHash:    meta.LatestHash,
		ExportedAt:    time.Now().Unix(),
		KeyCount:      keyCount,
		DataSHA256:    hex.EncodeToString(hash.Sum(nil)),
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return ChainDBSnapshotManifest{}, "", err
	}
	if err := writeFileAtomic(filepath.Join(snapshotDir, "manifest.json"), manifestBytes, 0644); err != nil {
		return ChainDBSnapshotManifest{}, "", err
	}

	return manifest, snapshotDir, nil
}

func RestoreChainDBSnapshotToPath(snapshotDir string, targetPath string, allowOverwrite bool) error {
	if strings.TrimSpace(snapshotDir) == "" {
		return fmt.Errorf("snapshot directory is required")
	}
	if strings.TrimSpace(targetPath) == "" {
		targetPath = constantset.BLOCKCHAIN_DB_PATH
	}

	manifestBytes, err := os.ReadFile(filepath.Join(snapshotDir, "manifest.json"))
	if err != nil {
		return err
	}
	var manifest ChainDBSnapshotManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return err
	}
	if manifest.Format != chainDBSnapshotFmt {
		return fmt.Errorf("unsupported chain snapshot format %q", manifest.Format)
	}

	if _, err := os.Stat(targetPath); err == nil {
		if !allowOverwrite {
			return fmt.Errorf("target DB path already exists: %s", targetPath)
		}
		backupPath := fmt.Sprintf("%s.pre-restore-%d", targetPath, time.Now().Unix())
		if err := os.Rename(targetPath, backupPath); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return err
	}
	db, err := leveldb.OpenFile(targetPath, &opt.Options{
		NoSync:      false,
		WriteBuffer: 64 * opt.MiB,
	})
	if err != nil {
		return err
	}
	defer db.Close()

	dataFile, err := os.Open(filepath.Join(snapshotDir, "leveldb.jsonl"))
	if err != nil {
		return err
	}
	defer dataFile.Close()

	hash := sha256.New()
	scanner := bufio.NewScanner(dataFile)
	scanner.Buffer(make([]byte, 1024), 64*1024*1024)
	batch := new(leveldb.Batch)
	keyCount := uint64(0)
	for scanner.Scan() {
		line := append([]byte(nil), scanner.Bytes()...)
		hash.Write(line)
		hash.Write([]byte("\n"))

		var record chainDBSnapshotRecord
		if err := json.Unmarshal(line, &record); err != nil {
			return err
		}
		key, err := base64.StdEncoding.DecodeString(record.Key)
		if err != nil {
			return err
		}
		value, err := base64.StdEncoding.DecodeString(record.Value)
		if err != nil {
			return err
		}
		batch.Put(key, value)
		keyCount++
		if keyCount%1000 == 0 {
			if err := db.Write(batch, &opt.WriteOptions{Sync: true}); err != nil {
				return err
			}
			batch.Reset()
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if batch.Len() > 0 {
		if err := db.Write(batch, &opt.WriteOptions{Sync: true}); err != nil {
			return err
		}
	}

	if keyCount != manifest.KeyCount {
		return fmt.Errorf("snapshot key count mismatch: got %d expected %d", keyCount, manifest.KeyCount)
	}
	if gotHash := hex.EncodeToString(hash.Sum(nil)); gotHash != manifest.DataSHA256 {
		return fmt.Errorf("snapshot checksum mismatch: got %s expected %s", gotHash, manifest.DataSHA256)
	}
	_, err = repairChainDBMetadataWithDB(db)
	return err
}

func GetBlockFromDB(blockNumber uint64) (*Block, error) {
	db, err := getDB()
	if err != nil {
		return nil, err
	}
	return loadBlockFromDBWithHandle(db, blockNumber)
}

func GetBlockByHashFromDB(hash string) (*Block, error) {
	hash = strings.ToLower(strings.TrimSpace(hash))
	if hash == "" {
		return nil, fmt.Errorf("missing block hash")
	}

	latest, err := GetLatestBlockNumberFromDB()
	if err != nil {
		return nil, err
	}
	if latest == 0 {
		return nil, fmt.Errorf("block not found")
	}

	for num := latest; num >= 1; num-- {
		blk, err := GetBlockFromDB(num)
		if err == nil && blk != nil && strings.ToLower(blk.CurrentHash) == hash {
			return blk, nil
		}
		if num == 1 {
			break
		}
	}

	return nil, fmt.Errorf("block not found")
}

func GetLatestBlockNumberFromDB() (uint64, error) {
	db, err := getDB()
	if err != nil {
		return 0, err
	}

	meta, metaErr := getChainDBMetadataRaw(db)
	if metaErr != nil || meta.SchemaVersion != chainDBSchemaVersion || meta.LatestBlock == 0 {
		repaired, repairErr := repairChainDBMetadataWithDB(db)
		if repairErr != nil {
			return 0, repairErr
		}
		if repaired.LatestBlock == 0 {
			return 0, fmt.Errorf("latest block metadata not found")
		}
		return repaired.LatestBlock, nil
	}

	if markerLatest, markerErr := getLatestBlockMarkerFromDB(db); markerErr == nil && markerLatest != meta.LatestBlock {
		repaired, repairErr := repairChainDBMetadataWithDB(db)
		if repairErr != nil {
			return 0, repairErr
		}
		return repaired.LatestBlock, nil
	}

	if _, err := loadBlockFromDBWithHandle(db, meta.LatestBlock); err != nil {
		repaired, repairErr := repairChainDBMetadataWithDB(db)
		if repairErr != nil {
			return 0, repairErr
		}
		return repaired.LatestBlock, nil
	}

	return meta.LatestBlock, nil
}

func GetRecentBlocksFromDB(limit int) ([]*Block, uint64, error) {
	if limit < 1 {
		limit = 15
	}

	latest, err := GetLatestBlockNumberFromDB()
	if err != nil {
		return nil, 0, err
	}
	if latest == 0 {
		return []*Block{}, 0, nil
	}

	blocks := make([]*Block, 0, limit)
	for num, count := latest, 0; num >= 1 && count < limit; num-- {
		blk, err := GetBlockFromDB(num)
		if err == nil && blk != nil {
			blocks = append(blocks, blk)
			count++
		}
		if num == 1 {
			break
		}
	}

	return blocks, latest, nil
}

func GetPaginatedBlocksFromDB(page, size int) ([]*Block, uint64, int, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 200 {
		size = 10
	}

	latest, err := GetLatestBlockNumberFromDB()
	if err != nil {
		return nil, 0, 0, err
	}
	if latest == 0 {
		return []*Block{}, 0, 1, nil
	}

	total := int(latest)
	totalPages := (total + size - 1) / size
	if totalPages == 0 {
		totalPages = 1
	}

	endNum := total - (page-1)*size
	startNum := endNum - size + 1
	if endNum < 1 {
		return []*Block{}, latest, totalPages, nil
	}
	if startNum < 1 {
		startNum = 1
	}

	blocks := make([]*Block, 0, size)
	for num := endNum; num >= startNum; num-- {
		blk, err := GetBlockFromDB(uint64(num))
		if err == nil && blk != nil {
			blocks = append(blocks, blk)
		}
	}

	return blocks, latest, totalPages, nil
}

func PutIntoDB(bs Blockchain_struct) error {
	db, err := getDB()
	if err != nil {
		return err
	}

	batch := new(leveldb.Batch)
	dbCopy := bs
	dbCopy.Mutex = sync.Mutex{}
	data, err := json.Marshal(dbCopy)
	if err != nil {
		return err
	}

	batch.Put([]byte(constantset.BLOCKCHAIN_KEY), data)
	return db.Write(batch, &opt.WriteOptions{Sync: true})
}

func GetBlockchain() (*Blockchain_struct, error) {
	db, err := getDB()
	if err != nil {
		return nil, err
	}
	data, err := db.Get([]byte(constantset.BLOCKCHAIN_KEY), nil)
	if err != nil {
		return nil, err
	}
	var blockchain Blockchain_struct
	err = json.Unmarshal(data, &blockchain)
	if err != nil {
		return nil, err
	}
	return &blockchain, nil
}

func KeyExist() (bool, error) {
	db, err := getDB()
	if err != nil {
		return false, err
	}
	exists, err := db.Has([]byte(constantset.BLOCKCHAIN_KEY), nil)
	if err != nil {
		return false, err
	}
	return exists, nil
}
