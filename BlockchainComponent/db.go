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

func SaveBlockToDB(block *Block) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(constantset.BLOCKCHAIN_DB_PATH), 0755); err != nil {
		return fmt.Errorf("failed to create DB directory: %v", err)
	}

	db, err := leveldb.OpenFile(constantset.BLOCKCHAIN_DB_PATH, &opt.Options{
		NoSync:      false,        // let OS handle sync
		WriteBuffer: 64 * opt.MiB, // huge performance boost
	})
	if err != nil {
		return fmt.Errorf("failed to open block DB: %v", err)
	}
	defer db.Close()

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

// Add to db.go
// func SaveBlockToDB(block *Block) error {
// 	db, err := leveldb.OpenFile(constantset.BLOCKCHAIN_DB_PATH, &opt.Options{
// 		WriteBuffer: 64 * opt.MiB,
// 	})
// 	if err != nil {
// 		return err
// 	}
// 	defer db.Close()

// 	// Save individual block
// 	blockKey := fmt.Sprintf("block_%d", block.BlockNumber)
// 	blockData, err := json.Marshal(block)
// 	if err != nil {
// 		return err
// 	}

// 	// Save to individual block storage
// 	if err := db.Put([]byte(blockKey), blockData, nil); err != nil {
// 		return err
// 	}

// 	// Update latest block pointer
// 	if err := db.Put([]byte("latest_block"), []byte(blockKey), nil); err != nil {
// 		return err
// 	}

// 	return nil
// }

func GetBlockFromDB(blockNumber uint64) (*Block, error) {
	db, err := leveldb.OpenFile(constantset.BLOCKCHAIN_DB_PATH, nil)
	if err != nil {
		return nil, err
	}
	defer db.Close()

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

// In DB.go, add directory creation:
//
//	func PutIntoDB(bs Blockchain_struct) error {
//		// Ensure directory exists
//		if err := os.MkdirAll(filepath.Dir(constantset.BLOCKCHAIN_DB_PATH), 0755); err != nil {
//			return fmt.Errorf("failed to create DB directory: %v", err)
//		}
//		db, err := leveldb.OpenFile(constantset.BLOCKCHAIN_DB_PATH, nil)
//		if err != nil {
//			return fmt.Errorf("failed to open DB: %v", err)
//		}
//		defer db.Close()
//		JsonFormat, err := json.Marshal(bs)
//		if err != nil {
//			return fmt.Errorf("failed to marshal blockchain: %v", err)
//		}
//		return db.Put([]byte(constantset.BLOCKCHAIN_KEY), JsonFormat, nil)
//	}
// func PutIntoDB(bs Blockchain_struct) error {
// 	db, err := leveldb.OpenFile(constantset.BLOCKCHAIN_DB_PATH, nil)
// 	if err != nil {
// 		return err
// 	}
// 	defer db.Close()
// 	batch := new(leveldb.Batch)
// 	data, err := json.Marshal(bs)
// 	if err != nil {
// 		return err
// 	}
// 	batch.Put([]byte(constantset.BLOCKCHAIN_KEY), data)
// 	return db.Write(batch, nil)
// }

// this is last one i have commented out
// func PutIntoDB(bs Blockchain_struct) error {
// 	db, err := leveldb.OpenFile(constantset.BLOCKCHAIN_DB_PATH, &opt.Options{
// 		NoSync:      false,        // Faster writes
// 		WriteBuffer: 64 * opt.MiB, // Larger buffer
// 	})
// 	if err != nil {
// 		return err
// 	}
// 	defer db.Close()

// 	// Batch writes
// 	batch := new(leveldb.Batch)
// 	dbCopy := bs
// 	dbCopy.Mutex = sync.Mutex{}
// 	data, err := json.Marshal(dbCopy)
// 	if err != nil {
// 		return err
// 	}

// 	batch.Put([]byte(constantset.BLOCKCHAIN_KEY), data)
// 	return db.Write(batch, &opt.WriteOptions{Sync: false})
// }

func PutIntoDB(bs Blockchain_struct) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(constantset.BLOCKCHAIN_DB_PATH), 0755); err != nil {
		return fmt.Errorf("failed to create DB directory: %v", err)
	}
	db, err := leveldb.OpenFile(constantset.BLOCKCHAIN_DB_PATH, &opt.Options{
		NoSync:      false,
		WriteBuffer: 64 * opt.MiB,
	})
	if err != nil {
		return err
	}
	defer db.Close()

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

	db, err := leveldb.OpenFile(constantset.BLOCKCHAIN_DB_PATH, nil)
	if err != nil {
		return nil, err
	}
	defer db.Close()
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
	db, err := leveldb.OpenFile(constantset.BLOCKCHAIN_DB_PATH, nil)
	if err != nil {
		return false, err
	}
	defer db.Close()
	exists, err := db.Has([]byte(constantset.BLOCKCHAIN_KEY), nil)
	if err != nil {
		return false, err
	}
	return exists, nil
}
