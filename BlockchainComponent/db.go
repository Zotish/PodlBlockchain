package blockchaincomponent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	constantset "github.com/Zotish/DefenceProject/ConstantSet"
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
