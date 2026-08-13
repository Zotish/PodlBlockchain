package blockchainserver

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	bc "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/BlockchainComponent"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/util"
)

type ExplorerIndexedTransaction struct {
	Hash        string `json:"hash"`
	BlockHash   string `json:"block_hash"`
	BlockNumber uint64 `json:"block_number"`
	Index       int    `json:"index"`
	Timestamp   uint64 `json:"timestamp"`
	From        string `json:"from"`
	To          string `json:"to"`
	Value       string `json:"value"`
	Type        string `json:"type"`
	Function    string `json:"function,omitempty"`
	Status      string `json:"status"`
}
type ExplorerIndexedBlock struct {
	Hash         string `json:"hash"`
	Number       uint64 `json:"number"`
	Timestamp    uint64 `json:"timestamp"`
	Transactions int    `json:"transactions"`
	GasUsed      uint64 `json:"gas_used"`
	StateRoot    string `json:"state_root"`
}
type ExplorerIndexer struct {
	mu            sync.Mutex
	db            *leveldb.DB
	path          string
	indexedHeight uint64
	indexedHash   string
	txCount       uint64
	lastSync      int64
}

func explorerIndexPath() string {
	if path := strings.TrimSpace(os.Getenv("LQD_EXPLORER_INDEX_PATH")); path != "" {
		return path
	}
	if root := strings.TrimSpace(os.Getenv("LQD_DATA_DIR")); root != "" {
		return filepath.Join(root, "explorer_index")
	}
	config, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "podl-explorer-index")
	}
	return filepath.Join(config, "lqd", "explorer_index")
}
func OpenExplorerIndexer(path string) (*ExplorerIndexer, error) {
	if strings.TrimSpace(path) == "" {
		path = explorerIndexPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	db, err := leveldb.OpenFile(path, nil)
	if err != nil {
		return nil, err
	}
	index := &ExplorerIndexer{db: db, path: path}
	index.loadMeta()
	return index, nil
}
func (x *ExplorerIndexer) Close() error {
	if x == nil || x.db == nil {
		return nil
	}
	return x.db.Close()
}
func (x *ExplorerIndexer) loadMeta() {
	if raw, err := x.db.Get([]byte("meta:height"), nil); err == nil {
		x.indexedHeight, _ = strconv.ParseUint(string(raw), 10, 64)
	}
	if raw, err := x.db.Get([]byte("meta:hash"), nil); err == nil {
		x.indexedHash = string(raw)
	}
	if raw, err := x.db.Get([]byte("meta:tx_count"), nil); err == nil {
		x.txCount, _ = strconv.ParseUint(string(raw), 10, 64)
	}
}
func explorerHeightKey(height uint64) []byte {
	key := make([]byte, 8)
	binary.BigEndian.PutUint64(key, height)
	return key
}
func explorerAddressKey(address string, height uint64, index int, hash string) []byte {
	return []byte(fmt.Sprintf("address:%s:%020d:%08d:%s", strings.ToLower(address), height, index, strings.ToLower(hash)))
}
func (x *ExplorerIndexer) clear() error {
	batch := new(leveldb.Batch)
	iterator := x.db.NewIterator(nil, nil)
	for iterator.Next() {
		batch.Delete(append([]byte(nil), iterator.Key()...))
	}
	iterator.Release()
	if err := iterator.Error(); err != nil {
		return err
	}
	if err := x.db.Write(batch, nil); err != nil {
		return err
	}
	x.indexedHeight = 0
	x.indexedHash = ""
	x.txCount = 0
	return nil
}
func (x *ExplorerIndexer) Sync(chain *bc.Blockchain_struct) error {
	if x == nil || x.db == nil || chain == nil {
		return fmt.Errorf("indexer unavailable")
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	chain.Mutex.Lock()
	blocks := append([]*bc.Block(nil), chain.Blocks...)
	chain.Mutex.Unlock()
	if len(blocks) == 0 {
		return nil
	}
	if x.indexedHash != "" {
		matched := false
		for _, block := range blocks {
			if block != nil && block.BlockNumber == x.indexedHeight && strings.EqualFold(block.CurrentHash, x.indexedHash) {
				matched = true
				break
			}
		}
		if !matched {
			if err := x.clear(); err != nil {
				return err
			}
		}
	}
	for _, block := range blocks {
		if block == nil || block.BlockNumber < x.indexedHeight || (block.BlockNumber == x.indexedHeight && x.indexedHash != "") {
			continue
		}
		batch := new(leveldb.Batch)
		blockRow := ExplorerIndexedBlock{Hash: strings.ToLower(block.CurrentHash), Number: block.BlockNumber, Timestamp: block.TimeStamp, Transactions: len(block.Transactions), GasUsed: block.GasUsed, StateRoot: block.StateRoot}
		raw, _ := json.Marshal(blockRow)
		batch.Put(append([]byte("block:number:"), explorerHeightKey(block.BlockNumber)...), raw)
		batch.Put([]byte("block:hash:"+strings.ToLower(block.CurrentHash)), raw)
		for index, tx := range block.Transactions {
			if tx == nil {
				continue
			}
			hash := strings.ToLower(strings.TrimSpace(tx.TxHash))
			if hash == "" {
				hash = strings.ToLower(bc.CalculateTransactionHash(*tx))
			}
			row := ExplorerIndexedTransaction{Hash: hash, BlockHash: blockRow.Hash, BlockNumber: block.BlockNumber, Index: index, Timestamp: tx.Timestamp, From: strings.ToLower(tx.From), To: strings.ToLower(tx.To), Value: bc.AmountString(tx.Value), Type: tx.Type, Function: tx.Function, Status: tx.Status}
			encoded, _ := json.Marshal(row)
			batch.Put([]byte("tx:"+hash), encoded)
			for _, address := range []string{row.From, row.To} {
				if bc.ValidateAddress(address) {
					batch.Put(explorerAddressKey(address, block.BlockNumber, index, hash), encoded)
				}
			}
			x.txCount++
		}
		x.indexedHeight, x.indexedHash = block.BlockNumber, blockRow.Hash
		batch.Put([]byte("meta:height"), []byte(strconv.FormatUint(x.indexedHeight, 10)))
		batch.Put([]byte("meta:hash"), []byte(x.indexedHash))
		batch.Put([]byte("meta:tx_count"), []byte(strconv.FormatUint(x.txCount, 10)))
		if err := x.db.Write(batch, nil); err != nil {
			return err
		}
	}
	x.lastSync = time.Now().Unix()
	return nil
}
func (x *ExplorerIndexer) Status() map[string]interface{} {
	x.mu.Lock()
	defer x.mu.Unlock()
	return map[string]interface{}{"indexed_height": x.indexedHeight, "indexed_hash": x.indexedHash, "indexed_transactions": x.txCount, "last_sync": x.lastSync, "path": x.path, "persistent": true}
}

// Metrics returns a lock-consistent index checkpoint and its distance from the
// finalized chain tip. Keeping this calculation in the indexer avoids races
// between status/Prometheus readers and an incremental sync.
func (x *ExplorerIndexer) Metrics(chainHeight uint64) (indexedHeight, indexedTransactions, lag uint64, lastSync int64) {
	x.mu.Lock()
	defer x.mu.Unlock()
	indexedHeight, indexedTransactions, lastSync = x.indexedHeight, x.txCount, x.lastSync
	if chainHeight > indexedHeight {
		lag = chainHeight - indexedHeight
	}
	return
}
func (x *ExplorerIndexer) Search(query string, limit int) (interface{}, error) {
	x.mu.Lock()
	defer x.mu.Unlock()
	query = strings.TrimSpace(query)
	if limit < 1 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	if bc.ValidateAddress(query) {
		prefix := []byte("address:" + strings.ToLower(query) + ":")
		iterator := x.db.NewIterator(util.BytesPrefix(prefix), nil)
		defer iterator.Release()
		rows := []ExplorerIndexedTransaction{}
		if iterator.Last() {
			for {
				var row ExplorerIndexedTransaction
				if json.Unmarshal(iterator.Value(), &row) == nil {
					rows = append(rows, row)
				}
				if len(rows) >= limit || !iterator.Prev() {
					break
				}
			}
		}
		return map[string]interface{}{"type": "address", "query": strings.ToLower(query), "transactions": rows}, iterator.Error()
	}
	if raw, err := x.db.Get([]byte("tx:"+strings.ToLower(query)), nil); err == nil {
		var row ExplorerIndexedTransaction
		if json.Unmarshal(raw, &row) == nil {
			return map[string]interface{}{"type": "transaction", "transaction": row}, nil
		}
	}
	if raw, err := x.db.Get([]byte("block:hash:"+strings.ToLower(query)), nil); err == nil {
		var row ExplorerIndexedBlock
		if json.Unmarshal(raw, &row) == nil {
			return map[string]interface{}{"type": "block", "block": row}, nil
		}
	}
	if number, err := strconv.ParseUint(query, 10, 64); err == nil {
		if raw, getErr := x.db.Get(append([]byte("block:number:"), explorerHeightKey(number)...), nil); getErr == nil {
			var row ExplorerIndexedBlock
			if json.Unmarshal(raw, &row) == nil {
				return map[string]interface{}{"type": "block", "block": row}, nil
			}
		}
	}
	return nil, leveldb.ErrNotFound
}
func (b *BlockchainServer) ensureExplorerIndexer() (*ExplorerIndexer, error) {
	b.indexerMu.Lock()
	defer b.indexerMu.Unlock()
	if b.indexer == nil && b.indexerErr == nil {
		b.indexer, b.indexerErr = OpenExplorerIndexer("")
	}
	if b.indexerErr != nil {
		return nil, b.indexerErr
	}
	if err := b.indexer.Sync(b.BlockchainPtr); err != nil {
		return nil, err
	}
	return b.indexer, nil
}
func (b *BlockchainServer) ExplorerIndexStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	setCORSHeaders(w, r)
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	index, err := b.ensureExplorerIndexer()
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusServiceUnavailable)
		return
	}
	status := index.Status()
	chainHeight, _ := b.currentChainTip()
	indexedHeight, _, lag, _ := index.Metrics(chainHeight)
	status["chain_height"] = chainHeight
	status["indexed_height"] = indexedHeight
	status["lag_blocks"] = lag
	_ = json.NewEncoder(w).Encode(status)
}
func (b *BlockchainServer) ExplorerIndexSearch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	setCORSHeaders(w, r)
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	index, err := b.ensureExplorerIndexer()
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusServiceUnavailable)
		return
	}
	result, err := index.Search(r.URL.Query().Get("q"), 25)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	_ = json.NewEncoder(w).Encode(result)
}
