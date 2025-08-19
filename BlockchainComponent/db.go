package blockchaincomponent

import (
	"encoding/json"
	"sync"

	constantset "github.com/Zotish/DefenceProject/ConstantSet"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

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

func PutIntoDB(bs Blockchain_struct) error {
	db, err := leveldb.OpenFile(constantset.BLOCKCHAIN_DB_PATH, &opt.Options{
		NoSync:      false,        // Faster writes
		WriteBuffer: 64 * opt.MiB, // Larger buffer
	})
	if err != nil {
		return err
	}
	defer db.Close()

	// Batch writes
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
