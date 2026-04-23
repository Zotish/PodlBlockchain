package blockchaincomponent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	constantset "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/ConstantSet"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

var (
	dbOnce     sync.Once
	dbInstance *leveldb.DB
	dbErr      error
)

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

func SaveBlockToDB(block *Block) error {
	db, err := getDB()
	if err != nil {
		return fmt.Errorf("failed to open block DB: %v", err)
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
	batch.Put([]byte("latest_block"), []byte(blockKey))

	// Fast write (no fsync)
	if err := db.Write(batch, &opt.WriteOptions{Sync: false}); err != nil {
		return fmt.Errorf("failed to write block batch: %v", err)
	}

	return nil
}

func GetBlockFromDB(blockNumber uint64) (*Block, error) {
	db, err := getDB()
	if err != nil {
		return nil, err
	}

	blockKey := fmt.Sprintf("block_%d", blockNumber)
	data, err := db.Get([]byte(blockKey), nil)
	if err != nil {
		return nil, err
	}

	var block Block
	if err := json.Unmarshal(data, &block); err != nil {
		return nil, err
	}

	return &block, nil
}

func GetLatestBlockNumberFromDB() (uint64, error) {
	db, err := getDB()
	if err != nil {
		return 0, err
	}

	raw, err := db.Get([]byte("latest_block"), nil)
	if err != nil {
		return 0, err
	}

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
	return db.Write(batch, &opt.WriteOptions{Sync: false})
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
