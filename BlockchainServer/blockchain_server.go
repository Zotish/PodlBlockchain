package blockchainserver

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	blockchaincomponent "github.com/Zotish/DefenceProject/BlockchainComponent"
	constantset "github.com/Zotish/DefenceProject/ConstantSet"
	wallet "github.com/Zotish/DefenceProject/WalletComponent"
	"github.com/gorilla/mux"
)

type BlockchainServer struct {
	Port          uint                                   `json:"port"`
	BlockchainPtr *blockchaincomponent.Blockchain_struct `json:"blockchain_ptr"`
}
type TxEnvelope struct {
	Transaction *blockchaincomponent.Transaction `json:"transaction"`
	Source      string                           `json:"source"`                 // "mempool" or "block"
	BlockHash   string                           `json:"block_hash,omitempty"`   // if confirmed
	BlockNumber uint64                           `json:"block_number,omitempty"` // if confirmed
	TxIndex     int                              `json:"tx_index,omitempty"`     // if confirmed
}

type amountField struct {
	V *big.Int
}

func (a *amountField) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "" || s == "null" {
		a.V = big.NewInt(0)
		return nil
	}
	if strings.HasPrefix(s, "\"") {
		unq, err := strconv.Unquote(s)
		if err != nil {
			return err
		}
		s = unq
	}
	amt, err := blockchaincomponent.NewAmountFromString(s)
	if err != nil {
		return err
	}
	a.V = amt
	return nil
}

func NewBlockchainServer(port uint, blockchainPtr *blockchaincomponent.Blockchain_struct) *BlockchainServer {
	return &BlockchainServer{
		Port:          port,
		BlockchainPtr: blockchainPtr,
	}
}

func (b *BlockchainServer) getBlockchain(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodGet {
		io.WriteString(w, b.BlockchainPtr.ToJsonChain())
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
}

func (bcs *BlockchainServer) GetAccountNonce(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	address := mux.Vars(r)["address"]

	bcs.BlockchainPtr.Mutex.Lock()
	defer bcs.BlockchainPtr.Mutex.Unlock()

	// confirmed (last used) nonce from chain state
	confirmed := bcs.BlockchainPtr.GetAccountNonce(address) // existing behavior:contentReference[oaicite:2]{index=2}

	// compute next free nonce including pending txs from this address
	next := confirmed + 1
	for _, tx := range bcs.BlockchainPtr.Transaction_pool {
		if tx.From == address && tx.Nonce >= next {
			next = tx.Nonce + 1
		}
	}

	// Backward-compatible: keep "nonce" (now meaning next usable)
	_ = json.NewEncoder(w).Encode(map[string]uint64{
		"confirmed_nonce": confirmed,
		"next_nonce":      next,
		"nonce":           next,
	})
}

func (b *BlockchainServer) sendTransaction(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodPost {

		request, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()
		var tx blockchaincomponent.Transaction

		err = json.Unmarshal(request, &tx)
		if err != nil {
			http.Error(w, "Invalid transaction data", http.StatusBadRequest)
			return
		}

		go b.BlockchainPtr.AddNewTxToTheTransaction_pool(&tx)
		io.WriteString(w, tx.ToJsonTx())
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
}

func (b *BlockchainServer) sendTransactionBatch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var txs []blockchaincomponent.Transaction
	if err := json.NewDecoder(r.Body).Decode(&txs); err != nil {
		http.Error(w, "Invalid batch payload", http.StatusBadRequest)
		return
	}
	if len(txs) == 0 {
		json.NewEncoder(w).Encode(map[string]int{"accepted": 0, "failed": 0})
		return
	}

	batch := make([]*blockchaincomponent.Transaction, 0, len(txs))
	for i := range txs {
		txCopy := txs[i]
		batch = append(batch, &txCopy)
	}

	accepted, failed := b.BlockchainPtr.AddNewTxBatch(batch)
	json.NewEncoder(w).Encode(map[string]int{
		"accepted": accepted,
		"failed":   failed,
	})
}

func (b *BlockchainServer) fetchNBlocks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Handle preflight requests
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	b.BlockchainPtr.Mutex.Lock()
	defer b.BlockchainPtr.Mutex.Unlock()

	if r.Method == http.MethodGet {
		blocks := b.BlockchainPtr.Blocks
		var blocksToReturn []*blockchaincomponent.Block
		if len(blocks) < 10 {
			blocksToReturn = blocks
		} else {
			blocksToReturn = blocks[len(blocks)-10:]
		}

		json.NewEncoder(w).Encode(blocksToReturn) // Actually return the data
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}

}
func (bcs *BlockchainServer) GetBlockchainHeight(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	height := uint64(len(bcs.BlockchainPtr.Blocks))
	json.NewEncoder(w).Encode(map[string]uint64{"height": height})
}

func (bcs *BlockchainServer) GetBalance(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	address := r.URL.Query().Get("address")

	bcs.BlockchainPtr.Mutex.Lock()
	defer bcs.BlockchainPtr.Mutex.Unlock()

	// Get confirmed balance from accounts
	confirmedBalance := blockchaincomponent.CopyAmount(bcs.BlockchainPtr.Accounts[address])
	if confirmedBalance == nil {
		confirmedBalance = big.NewInt(0)
	}

	// Calculate pending balance changes from transaction pool
	pendingBalanceChange := big.NewInt(0)
	for _, tx := range bcs.BlockchainPtr.Transaction_pool {
		if tx.From == address && tx.Status == constantset.StatusPending {
			cost := new(big.Int).Add(blockchaincomponent.CopyAmount(tx.Value), blockchaincomponent.NewAmountFromUint64(tx.GasPrice*tx.CalculateGasCost()))
			pendingBalanceChange.Sub(pendingBalanceChange, cost)
		}
		if tx.To == address && tx.Status == constantset.StatusPending {
			pendingBalanceChange.Add(pendingBalanceChange, blockchaincomponent.CopyAmount(tx.Value))
		}
	}

	totalBalance := new(big.Int).Add(confirmedBalance, pendingBalanceChange)
	if totalBalance.Sign() < 0 {
		totalBalance = big.NewInt(0)
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"address":                address,
		"balance":                blockchaincomponent.AmountString(totalBalance),
		"confirmed_balance":      blockchaincomponent.AmountString(confirmedBalance),
		"pending_balance_change": blockchaincomponent.AmountString(pendingBalanceChange),
	})
}

func (bcs *BlockchainServer) GetBridgeRequests(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	addr := r.URL.Query().Get("address")
	list := bcs.BlockchainPtr.ListBridgeRequests(addr)
	json.NewEncoder(w).Encode(list)
}

func (bcs *BlockchainServer) GetBridgeTokens(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	list := bcs.BlockchainPtr.ListBridgeTokenMappings()
	json.NewEncoder(w).Encode(list)
}

// BridgeLockBsc registers a BSC lock request directly (fallback when RPC log scan misses).
func (bcs *BlockchainServer) BridgeLockBsc(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		BscTx string `json:"bsc_tx"`
		Token string `json:"token"`
		From  string `json:"from"`
		ToLqd string `json:"to_lqd"`
		Amount string `json:"amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}
	if req.BscTx == "" || req.Token == "" || req.ToLqd == "" || req.Amount == "" {
		http.Error(w, `{"error":"missing fields"}`, http.StatusBadRequest)
		return
	}
	if !wallet.ValidateAddress(req.ToLqd) {
		http.Error(w, `{"error":"invalid to_lqd"}`, http.StatusBadRequest)
		return
	}
	amt, err := blockchaincomponent.NewAmountFromString(req.Amount)
	if err != nil || amt.Sign() <= 0 {
		http.Error(w, `{"error":"invalid amount"}`, http.StatusBadRequest)
		return
	}
	bcs.BlockchainPtr.AddBridgeRequestBSC(req.BscTx, req.Token, req.From, req.ToLqd, amt)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}

// BridgeBurnLqd executes a burn on LQD and registers a release request for BSC.
func (bcs *BlockchainServer) BridgeBurnLqd(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Token string `json:"token"`
		From  string `json:"from"`
		ToBsc string `json:"to_bsc"`
		Amount string `json:"amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}
	if req.Token == "" || req.From == "" || req.ToBsc == "" || req.Amount == "" {
		http.Error(w, `{"error":"missing fields"}`, http.StatusBadRequest)
		return
	}
	amt, err := blockchaincomponent.NewAmountFromString(req.Amount)
	if err != nil || amt.Sign() <= 0 {
		http.Error(w, `{"error":"invalid amount"}`, http.StatusBadRequest)
		return
	}
	if !wallet.ValidateAddress(req.Token) || !wallet.ValidateAddress(req.From) {
		http.Error(w, `{"error":"invalid address"}`, http.StatusBadRequest)
		return
	}

	info := bcs.BlockchainPtr.GetBridgeTokenMappingByLqd(req.Token)
	if info == nil || info.BscToken == "" {
		http.Error(w, `{"error":"token not mapped"}`, http.StatusBadRequest)
		return
	}

	// Execute burn directly (state change) and register request.
	_, err = bcs.BlockchainPtr.ContractEngine.Pipeline.Execute(
		req.Token,
		req.From,
		"Burn",
		[]string{blockchaincomponent.AmountString(amt), req.ToBsc},
		5_000_000,
	)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"burn failed: %v"}`, err), http.StatusBadRequest)
		return
	}

	tx := bcs.BlockchainPtr.NewSystemTx("contract_call", req.From, req.Token, blockchaincomponent.NewAmountFromUint64(0))
	tx.Function = "Burn"
	tx.Args = []string{blockchaincomponent.AmountString(amt), req.ToBsc}
	tx.IsContract = true
	tx.Status = constantset.StatusSuccess
	tx.TxHash = blockchaincomponent.CalculateTransactionHash(*tx)
	bcs.BlockchainPtr.RecordRecentTx(tx)

	bcs.BlockchainPtr.AddBridgeRequestBurn(tx.TxHash, info.BscToken, req.From, req.ToBsc, amt)

	json.NewEncoder(w).Encode(map[string]string{
		"tx_hash": tx.TxHash,
		"status":  "burned",
	})
}

func (bcs *BlockchainServer) Faucet(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// accept either JSON body or query param
	var payload struct {
		Address string `json:"address"`
	}
	_ = json.NewDecoder(r.Body).Decode(&payload)
	address := strings.TrimSpace(payload.Address)
	if address == "" {
		address = strings.TrimSpace(r.URL.Query().Get("address"))
	}
	if !wallet.ValidateAddress(address) {
		http.Error(w, "Invalid address", http.StatusBadRequest)
		return
	}

	// credit directly (test faucet)
	amount := blockchaincomponent.NewAmountFromUint64(100000000000000)
	bcs.BlockchainPtr.AddAccountBalance(address, amount)

	json.NewEncoder(w).Encode(map[string]interface{}{"credited": amount.String(), "address": address})
}

func (bcs *BlockchainServer) ValidatorStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	address := mux.Vars(r)["address"]
	stats := bcs.BlockchainPtr.GetValidatorStats(address)
	if stats == nil {
		http.Error(w, "validator not found", http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(stats)
}

func (bcs *BlockchainServer) NetworkStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	stats := bcs.BlockchainPtr.GetNetworkStats()
	json.NewEncoder(w).Encode(stats)
}

func (bcs *BlockchainServer) GetPeers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if bcs.BlockchainPtr.Network == nil {
		http.Error(w, "network not initialized", http.StatusServiceUnavailable)
		return
	}

	bcs.BlockchainPtr.Network.Mutex.Lock()
	defer bcs.BlockchainPtr.Network.Mutex.Unlock()

	out := []map[string]interface{}{}
	for _, p := range bcs.BlockchainPtr.Network.Peers {
		if p == nil {
			continue
		}
		out = append(out, map[string]interface{}{
			"address":    p.Address,
			"port":       p.Port,
			"http_port":  p.HTTPPort,
			"last_seen":  p.LastSeen,
			"reputation": p.Reputation,
			"height":     p.Height,
			"is_active":  p.IsActive,
		})
	}
	json.NewEncoder(w).Encode(out)
}

func (bcs *BlockchainServer) AddPeer(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if bcs.BlockchainPtr.Network == nil {
		http.Error(w, "network not initialized", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		Address string `json:"address"`
		Port    int    `json:"port"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if req.Address == "" || req.Port == 0 {
		http.Error(w, "address and port required", http.StatusBadRequest)
		return
	}

	bcs.BlockchainPtr.Network.AddPeer(req.Address, req.Port, true)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
func (bcs *BlockchainServer) Metrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "podl_blocks_total %d\n", len(bcs.BlockchainPtr.Blocks))
	fmt.Fprintf(w, "podl_validators_total %d\n", len(bcs.BlockchainPtr.Validators))
	fmt.Fprintf(w, "podl_slashing_pool %.2f\n", bcs.BlockchainPtr.SlashingPool)
}

func (b *BlockchainServer) GetBlock(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		// Fallback for default ServeMux (no gorilla mux vars)
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) > 0 {
			id = parts[len(parts)-1]
		}
	}
	blockNumber, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		http.Error(w, "Invalid block number", http.StatusBadRequest)
		return
	}

	b.BlockchainPtr.Mutex.Lock()
	defer b.BlockchainPtr.Mutex.Unlock()

	// Try in-memory blocks first (handles trimmed chains)
	for _, blk := range b.BlockchainPtr.Blocks {
		if blk != nil && blk.BlockNumber == blockNumber {
			json.NewEncoder(w).Encode(blk)
			return
		}
	}

	// Fall back to DB if not in memory
	block, err := blockchaincomponent.GetBlockFromDB(blockNumber)
	if err != nil || block == nil {
		http.Error(w, "Block not found", http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(block)
}

func (bcs *BlockchainServer) GetValidators(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	bcs.BlockchainPtr.Mutex.Lock()
	defer bcs.BlockchainPtr.Mutex.Unlock()

	// Return all validators
	validators := make([]map[string]interface{}, len(bcs.BlockchainPtr.Validators))
	for i, v := range bcs.BlockchainPtr.Validators {
		validators[i] = map[string]interface{}{
			"address":         v.Address,
			"stake":           v.LPStakeAmount,
			"liquidity_power": v.LiquidityPower,
			"penalty_score":   v.PenaltyScore,
			"blocks_proposed": v.BlocksProposed,
			"blocks_included": v.BlocksIncluded,
			"last_active":     v.LastActive.Format(time.RFC3339),
			"lock_time":       v.LockTime.Format(time.RFC3339),
		}
	}

	json.NewEncoder(w).Encode(validators)
}

func (bcs *BlockchainServer) GetRecentTransactions(w http.ResponseWriter, r *http.Request) {
	// CORS
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	bcs.BlockchainPtr.Mutex.Lock()
	defer bcs.BlockchainPtr.Mutex.Unlock()

	// Merge recent history + mempool, prefer confirmed/failed over pending
	const pendingTTLSeconds = uint64(120)
	now := uint64(time.Now().Unix())

	type entry struct {
		tx   *blockchaincomponent.Transaction
		rank int
	}
	byHash := make(map[string]entry)

	statusRank := func(s string) int {
		ss := strings.ToLower(strings.TrimSpace(s))
		if ss == strings.ToLower(constantset.StatusSuccess) {
			return 2
		}
		if ss == strings.ToLower(constantset.StatusFailed) {
			return 1
		}
		return 0 // pending/unknown
	}

	// 1) Recent history (confirmed / failed) + drop stale pending
	for _, tx := range bcs.BlockchainPtr.RecentTxs {
		h := strings.ToLower(strings.TrimSpace(tx.TxHash))
		if h == "" {
			continue
		}
		if strings.ToLower(tx.Status) != strings.ToLower(constantset.StatusSuccess) &&
			strings.ToLower(tx.Status) != strings.ToLower(constantset.StatusFailed) {
			ts := uint64(tx.Timestamp)
			if ts > 0 && now > ts && now-ts > pendingTTLSeconds {
				continue
			}
		}
		r := statusRank(tx.Status)
		byHash[h] = entry{tx: tx, rank: r}
	}

	// 2) Pending / active mempool transactions (skip stale)
	for _, tx := range bcs.BlockchainPtr.Transaction_pool {
		h := strings.ToLower(strings.TrimSpace(tx.TxHash))
		if h == "" {
			continue
		}
		ts := uint64(tx.Timestamp)
		if ts > 0 && now > ts && now-ts > pendingTTLSeconds {
			continue
		}
		r := statusRank(tx.Status)
		if existing, ok := byHash[h]; ok {
			if existing.rank >= r {
				continue
			}
		}
		byHash[h] = entry{tx: tx, rank: r}
	}

	out := make([]*blockchaincomponent.Transaction, 0, len(byHash))
	for _, e := range byHash {
		out = append(out, e.tx)
	}

	// 3) Sort by timestamp desc (newest first)
	sort.Slice(out, func(i, j int) bool {
		return out[i].Timestamp > out[j].Timestamp
	})

	json.NewEncoder(w).Encode(out)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (bcs *BlockchainServer) LiquidityLock(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Address string `json:"address"`
		Amount  amountField `json:"amount"`
		Days    int    `json:"days"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if !wallet.ValidateAddress(req.Address) {
		http.Error(w, "invalid address", http.StatusBadRequest)
		return
	}
	if req.Amount.V == nil || req.Amount.V.Sign() <= 0 || req.Days <= 0 {
		http.Error(w, "invalid amount/duration", http.StatusBadRequest)
		return
	}

	if err := bcs.BlockchainPtr.LockLiquidity(req.Address, req.Amount.V, time.Duration(req.Days)*24*time.Hour); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (bcs *BlockchainServer) LiquidityUnlock(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Address string `json:"address"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if !wallet.ValidateAddress(req.Address) {
		http.Error(w, "invalid address", http.StatusBadRequest)
		return
	}

	released, err := bcs.BlockchainPtr.UnlockAvailable(req.Address)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"released": blockchaincomponent.AmountString(released)})
}

func (bcs *BlockchainServer) LiquidityView(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	address := r.URL.Query().Get("address")
	if !wallet.ValidateAddress(address) {
		http.Error(w, "invalid address", http.StatusBadRequest)
		return
	}

	bcs.BlockchainPtr.Mutex.Lock()
	defer bcs.BlockchainPtr.Mutex.Unlock()

	locks := bcs.BlockchainPtr.LiquidityLocks[address]
	json.NewEncoder(w).Encode(map[string]interface{}{
		"total_liquidity": blockchaincomponent.AmountString(bcs.BlockchainPtr.TotalLiquidity),
		"locked":          locks,
		"active_locked":   blockchaincomponent.AmountString(bcs.BlockchainPtr.GetLock(address)),
	})
}

func (bcs *BlockchainServer) RewardsRecent(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	bcs.BlockchainPtr.Mutex.Lock()
	defer bcs.BlockchainPtr.Mutex.Unlock()
	hist := bcs.BlockchainPtr.RewardHistory
	if len(hist) > 50 {
		hist = hist[len(hist)-50:]
	}
	json.NewEncoder(w).Encode(hist)
}

func (bcs *BlockchainServer) RewardsLatest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	bcs.BlockchainPtr.Mutex.Lock()
	defer bcs.BlockchainPtr.Mutex.Unlock()

	if len(bcs.BlockchainPtr.Blocks) == 0 {
		http.Error(w, "no blocks", http.StatusNotFound)
		return
	}

	last := bcs.BlockchainPtr.Blocks[len(bcs.BlockchainPtr.Blocks)-1]
	out := map[string]interface{}{
		"block_number":           last.BlockNumber,
		"validator":              last.RewardBreakdown.Validator,
		"validator_reward":       last.RewardBreakdown.ValidatorReward,
		"validator_rewards":      last.RewardBreakdown.ValidatorRewards,
		"validator_part_rewards": last.RewardBreakdown.ValidatorPartRewards,
		"liquidity_rewards":      last.RewardBreakdown.LiquidityRewards,
		"participant_rewards":    last.RewardBreakdown.ParticipantRewards,
	}
	json.NewEncoder(w).Encode(out)
}

func (bcs *BlockchainServer) ActiveValidatorsLatest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	bcs.BlockchainPtr.Mutex.Lock()
	defer bcs.BlockchainPtr.Mutex.Unlock()

	if len(bcs.BlockchainPtr.Blocks) == 0 {
		http.Error(w, "no blocks", http.StatusNotFound)
		return
	}

	last := bcs.BlockchainPtr.Blocks[len(bcs.BlockchainPtr.Blocks)-1]
	active := make([]map[string]interface{}, 0, len(last.RewardBreakdown.ValidatorPartRewards))
	votes := bcs.BlockchainPtr.BlockVotes[last.CurrentHash]
	for addr, amount := range last.RewardBreakdown.ValidatorPartRewards {
		var stake float64
		var power float64
		for _, v := range bcs.BlockchainPtr.Validators {
			if v.Address == addr {
				stake = v.LPStakeAmount
				power = v.LiquidityPower
				break
			}
		}
		voted := false
		if votes != nil {
			voted = votes[addr]
		}
		active = append(active, map[string]interface{}{
			"address": addr,
			"stake":   stake,
			"power":   power,
			"reward":  amount,
			"winner":  false,
			"voted":   voted,
		})
	}

	winnerAddr := last.RewardBreakdown.Validator
	if winnerAddr != "" {
		var stake float64
		var power float64
		for _, v := range bcs.BlockchainPtr.Validators {
			if v.Address == winnerAddr {
				stake = v.LPStakeAmount
				power = v.LiquidityPower
				break
			}
		}
		winnerVoted := false
		if votes != nil {
			winnerVoted = votes[winnerAddr]
		}
		active = append(active, map[string]interface{}{
			"address": winnerAddr,
			"stake":   stake,
			"power":   power,
			"reward":  last.RewardBreakdown.ValidatorReward,
			"winner":  true,
			"voted":   winnerVoted,
		})
	}

	out := map[string]interface{}{
		"block_number": last.BlockNumber,
		"winner":       last.RewardBreakdown.Validator,
		"active":       active,
		"votes":        votes,
	}
	json.NewEncoder(w).Encode(out)
}

func (bcs *BlockchainServer) SyncValidatorsAll(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if bcs.BlockchainPtr.Network == nil {
		http.Error(w, "network not initialized", http.StatusServiceUnavailable)
		return
	}

	type syncResult struct {
		Peer  string `json:"peer"`
		Error string `json:"error,omitempty"`
	}

	results := []syncResult{}
	for _, peer := range bcs.BlockchainPtr.Network.Peers {
		if peer == nil {
			continue
		}
		if err := bcs.BlockchainPtr.Network.SyncValidators(peer); err != nil {
			results = append(results, syncResult{Peer: peer.Address, Error: err.Error()})
			continue
		}
		results = append(results, syncResult{Peer: peer.Address})
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"results": results,
	})
}

func (bcs *BlockchainServer) ChainSummary(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	limit := parseLimit(r, 20)
	out := bcs.buildChainSummary(limit)
	json.NewEncoder(w).Encode(out)
}

func (bcs *BlockchainServer) GlobalChainSummary(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	limit := parseLimit(r, 20)
	nodesParam := r.URL.Query().Get("nodes")
	nodes := parseNodes(nodesParam)

	if len(nodes) == 0 && bcs.BlockchainPtr.Network != nil {
		for _, peer := range bcs.BlockchainPtr.Network.Peers {
			httpPort := peer.HTTPPort
			if httpPort == 0 {
				httpPort = peer.Port - 1000
			}
			if httpPort <= 0 {
				continue
			}
			nodes = append(nodes, fmt.Sprintf("%s:%d", peer.Address, httpPort))
		}
	}

	type nodeResult struct {
		Node    string      `json:"node"`
		Summary interface{} `json:"summary,omitempty"`
		Error   string      `json:"error,omitempty"`
	}

	results := []nodeResult{
		{Node: "local", Summary: bcs.buildChainSummary(limit)},
	}

	client := &http.Client{Timeout: 2 * time.Second}
	for _, node := range nodes {
		url := fmt.Sprintf("http://%s/chain/summary?limit=%d", node, limit)
		resp, err := client.Get(url)
		if err != nil {
			results = append(results, nodeResult{Node: node, Error: err.Error()})
			continue
		}
		var summary interface{}
		if err := json.NewDecoder(resp.Body).Decode(&summary); err != nil {
			resp.Body.Close()
			results = append(results, nodeResult{Node: node, Error: err.Error()})
			continue
		}
		resp.Body.Close()
		results = append(results, nodeResult{Node: node, Summary: summary})
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"nodes": results,
	})
}

func (bcs *BlockchainServer) buildChainSummary(limit int) map[string]interface{} {
	bcs.BlockchainPtr.Mutex.Lock()
	defer bcs.BlockchainPtr.Mutex.Unlock()

	height := len(bcs.BlockchainPtr.Blocks)
	validators := make([]map[string]interface{}, len(bcs.BlockchainPtr.Validators))
	for i, v := range bcs.BlockchainPtr.Validators {
		validators[i] = map[string]interface{}{
			"address":         v.Address,
			"stake":           v.LPStakeAmount,
			"liquidity_power": v.LiquidityPower,
			"penalty_score":   v.PenaltyScore,
		}
	}

	start := 0
	if height > limit {
		start = height - limit
	}

	winners := make([]map[string]interface{}, 0, height-start)
	for i := start; i < height; i++ {
		blk := bcs.BlockchainPtr.Blocks[i]
		participants := make([]string, 0, len(blk.RewardBreakdown.ValidatorPartRewards))
		for addr := range blk.RewardBreakdown.ValidatorPartRewards {
			participants = append(participants, addr)
		}
		votes := bcs.BlockchainPtr.BlockVotes[blk.CurrentHash]
		winners = append(winners, map[string]interface{}{
			"block_number":           blk.BlockNumber,
			"winner":                 blk.RewardBreakdown.Validator,
			"winner_reward":          blk.RewardBreakdown.ValidatorReward,
			"validator_part_rewards": blk.RewardBreakdown.ValidatorPartRewards,
			"participant_rewards":    blk.RewardBreakdown.ParticipantRewards,
			"validator_participants": participants,
			"votes":                  votes,
		})
	}

	return map[string]interface{}{
		"height":     height,
		"validators": validators,
		"winners":    winners,
	}
}

func parseLimit(r *http.Request, defaultLimit int) int {
	limit := defaultLimit
	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	return limit
}

func parseNodes(nodesParam string) []string {
	if nodesParam == "" {
		return nil
	}
	parts := strings.Split(nodesParam, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func (bcs *BlockchainServer) JSONRPC(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	var req struct {
		JSONRPC string        `json:"jsonrpc"`
		ID      interface{}   `json:"id"`
		Method  string        `json:"method"`
		Params  []interface{} `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	type rpcError struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	resp := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      req.ID,
	}

	bcs.BlockchainPtr.Mutex.Lock()
	defer bcs.BlockchainPtr.Mutex.Unlock()

	switch req.Method {
	case "web3_clientVersion":
		resp["result"] = "DefenceProject/1.0"
	case "net_version":
		resp["result"] = "139"
	case "eth_chainId":
		resp["result"] = "0x8b"
	case "eth_blockNumber":
		if len(bcs.BlockchainPtr.Blocks) == 0 {
			resp["result"] = "0x0"
		} else {
			resp["result"] = fmt.Sprintf("0x%x", bcs.BlockchainPtr.Blocks[len(bcs.BlockchainPtr.Blocks)-1].BlockNumber)
		}
	case "eth_getBalance":
		if len(req.Params) < 1 {
			resp["error"] = rpcError{Code: -32602, Message: "missing address"}
			break
		}
		addr, _ := req.Params[0].(string)
		bal := blockchaincomponent.CopyAmount(bcs.BlockchainPtr.Accounts[addr])
		if bal == nil {
			bal = big.NewInt(0)
		}
		resp["result"] = fmt.Sprintf("0x%x", bal)
	case "eth_getTransactionCount":
		if len(req.Params) < 1 {
			resp["error"] = rpcError{Code: -32602, Message: "missing address"}
			break
		}
		addr, _ := req.Params[0].(string)
		nonce := bcs.BlockchainPtr.GetAccountNonce(addr)
		resp["result"] = fmt.Sprintf("0x%x", nonce)
	case "eth_getBlockByNumber":
		if len(req.Params) < 1 {
			resp["error"] = rpcError{Code: -32602, Message: "missing block tag"}
			break
		}
		tag, _ := req.Params[0].(string)
		var blk *blockchaincomponent.Block
		if tag == "latest" || tag == "" {
			if len(bcs.BlockchainPtr.Blocks) > 0 {
				blk = bcs.BlockchainPtr.Blocks[len(bcs.BlockchainPtr.Blocks)-1]
			}
		} else if strings.HasPrefix(tag, "0x") {
			n, err := strconv.ParseUint(strings.TrimPrefix(tag, "0x"), 16, 64)
			if err == nil && int(n) < len(bcs.BlockchainPtr.Blocks) {
				blk = bcs.BlockchainPtr.Blocks[n]
			}
		}
		if blk == nil {
			resp["result"] = nil
			break
		}
		resp["result"] = map[string]interface{}{
			"number":       fmt.Sprintf("0x%x", blk.BlockNumber),
			"hash":         blk.CurrentHash,
			"parentHash":   blk.PreviousHash,
			"timestamp":    fmt.Sprintf("0x%x", blk.TimeStamp),
			"transactions": []interface{}{},
		}
	case "eth_gasPrice":
		resp["result"] = "0x1"
	case "eth_getTransactionByHash":
		if len(req.Params) < 1 {
			resp["error"] = rpcError{Code: -32602, Message: "missing tx hash"}
			break
		}
		hash, _ := req.Params[0].(string)
		tx, blockNumber := findTxByHash(bcs.BlockchainPtr, hash)
		if tx == nil {
			resp["result"] = nil
			break
		}
		result := map[string]interface{}{
			"hash":     tx.TxHash,
			"from":     tx.From,
			"to":       tx.To,
			"value":    "0x0",
			"gas":      fmt.Sprintf("0x%x", tx.Gas),
			"gasPrice": fmt.Sprintf("0x%x", tx.GasPrice),
			"nonce":    fmt.Sprintf("0x%x", tx.Nonce),
		}
		if tx.Value != nil {
			result["value"] = fmt.Sprintf("0x%s", tx.Value.Text(16))
		}
		if blockNumber >= 0 {
			result["blockNumber"] = fmt.Sprintf("0x%x", blockNumber)
		} else {
			result["blockNumber"] = nil
		}
		resp["result"] = result
	case "eth_getTransactionReceipt":
		if len(req.Params) < 1 {
			resp["error"] = rpcError{Code: -32602, Message: "missing tx hash"}
			break
		}
		hash, _ := req.Params[0].(string)
		tx, blockNumber := findTxByHash(bcs.BlockchainPtr, hash)
		if tx == nil || blockNumber < 0 {
			resp["result"] = nil
			break
		}
		resp["result"] = map[string]interface{}{
			"transactionHash": tx.TxHash,
			"blockNumber":     fmt.Sprintf("0x%x", blockNumber),
			"status":          "0x1",
		}
	case "eth_syncing":
		resp["result"] = false
	case "eth_accounts":
		resp["result"] = []string{}
	case "eth_sendRawTransaction":
		resp["error"] = rpcError{Code: -32601, Message: "method not supported"}
	default:
		resp["error"] = rpcError{Code: -32601, Message: "method not found"}
	}

	json.NewEncoder(w).Encode(resp)
}

func findTxByHash(bc *blockchaincomponent.Blockchain_struct, hash string) (*blockchaincomponent.Transaction, int64) {
	for _, blk := range bc.Blocks {
		for _, tx := range blk.Transactions {
			if tx.TxHash == hash {
				return tx, int64(blk.BlockNumber)
			}
		}
	}
	for _, tx := range bc.Transaction_pool {
		if tx.TxHash == hash {
			return tx, -1
		}
	}
	return nil, -1
}

func (bcs *BlockchainServer) AddValidatorFromPeer(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var incoming blockchaincomponent.Validator
	if err := json.NewDecoder(r.Body).Decode(&incoming); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	// Merge/dedupe
	bcs.BlockchainPtr.Mutex.Lock()
	defer bcs.BlockchainPtr.Mutex.Unlock()

	for _, v := range bcs.BlockchainPtr.Validators {
		if v.Address == incoming.Address {
			// Optional: update stake/LP/penalty/stats from the incoming object
			v.LPStakeAmount = incoming.LPStakeAmount
			v.LockTime = incoming.LockTime
			v.LiquidityPower = incoming.LiquidityPower
			v.PenaltyScore = incoming.PenaltyScore
			v.BlocksProposed = incoming.BlocksProposed
			v.BlocksIncluded = incoming.BlocksIncluded
			v.LastActive = incoming.LastActive
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
			return
		}
	}

	// New validator
	copy := incoming // ensure addressable
	bcs.BlockchainPtr.Validators = append(bcs.BlockchainPtr.Validators, &copy)

	// Persist (optional but recommended)
	dbCopy := *bcs.BlockchainPtr
	dbCopy.Mutex = sync.Mutex{}
	if err := blockchaincomponent.PutIntoDB(dbCopy); err != nil {
		log.Printf("persist validator merge failed: %v", err)
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "added"})
}

func (bcs *BlockchainServer) AddValidator(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Address string  `json:"address"`
		Amount  float64 `json:"amount"`
		Days    int     `json:"days"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	if !wallet.ValidateAddress(req.Address) || req.Amount <= 0 || req.Days <= 0 {
		http.Error(w, "invalid address/amount/days", http.StatusBadRequest)
		return
	}

	if err := bcs.BlockchainPtr.AddNewValidators(req.Address, req.Amount, time.Duration(req.Days)*24*time.Hour); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// rebroadcast all so peers converge
	for _, v := range bcs.BlockchainPtr.Validators {
		go bcs.BlockchainPtr.Network.BroadcastValidator(v)
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status": "ok", "address": req.Address, "amount": req.Amount, "minStake": bcs.BlockchainPtr.MinStake,
	})
}

func (bcs *BlockchainServer) GetTransactionByHash(w http.ResponseWriter, r *http.Request) {
	// CORS
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	path := r.URL.Path
	queryHash := r.URL.Query().Get("hash")

	var hash string
	if queryHash != "" {
		hash = queryHash
	} else if strings.HasPrefix(path, "/tx/") {
		hash = strings.TrimPrefix(path, "/tx/")
	}

	hash = strings.ToLower(strings.TrimSpace(hash))
	if hash == "" {
		http.Error(w, `{"error":"missing hash"}`, http.StatusBadRequest)
		return
	}

	bcs.BlockchainPtr.Mutex.Lock()
	defer bcs.BlockchainPtr.Mutex.Unlock()

	// 1) Look in mempool
	for _, tx := range bcs.BlockchainPtr.Transaction_pool {
		if strings.ToLower(tx.TxHash) == hash {
			resp := map[string]interface{}{
				"transaction": tx,
				"source":      "mempool",
			}
			json.NewEncoder(w).Encode(resp)
			return
		}
	}

	// 2) Look in in-memory blocks (confirmed)
	for i := len(bcs.BlockchainPtr.Blocks) - 1; i >= 0; i-- {
		blk := bcs.BlockchainPtr.Blocks[i]
		for idx, tx := range blk.Transactions {
			if strings.ToLower(tx.TxHash) == hash {
				resp := map[string]interface{}{
					"transaction":  tx,
					"source":       "block",
					"block_hash":   blk.CurrentHash,
					"block_number": blk.BlockNumber,
					"tx_index":     idx,
				}
				json.NewEncoder(w).Encode(resp)
				return
			}
		}
	}

	// 3) Look in recent tx history (failed / expired / very old)
	for _, tx := range bcs.BlockchainPtr.RecentTxs {
		if strings.ToLower(tx.TxHash) == hash {
			resp := map[string]interface{}{
				"transaction": tx,
				"source":      "recent",
			}
			json.NewEncoder(w).Encode(resp)
			return
		}
	}

	// 4) Not found anywhere
	http.Error(w, `{"error":"transaction not found"}`, http.StatusNotFound)
}
func (b *BlockchainServer) GetContractEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	address := r.URL.Query().Get("address")
	ev := b.BlockchainPtr.ContractEngine.EventDB.GetEventsFromDB(address)
	json.NewEncoder(w).Encode(ev)
}

func (bcs *BlockchainServer) GetAddressTransactions(w http.ResponseWriter, r *http.Request) {
	// CORS headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	vars := mux.Vars(r)
	rawAddress := strings.TrimSpace(vars["address"])
	if rawAddress == "" {
		// Fallback for default ServeMux (no gorilla mux vars)
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		for i := 0; i+1 < len(parts); i++ {
			if parts[i] == "address" {
				rawAddress = parts[i+1]
				break
			}
		}
	}
	rawAddress = strings.TrimSpace(rawAddress)
	addr := strings.ToLower(rawAddress)
	if addr == "" {
		http.Error(w, `{"error":"address is required"}`, http.StatusBadRequest)
		return
	}

	// Optional: validate format using your existing wallet validator
	if !wallet.ValidateAddress(rawAddress) {
		http.Error(w, `{"error":"invalid address format"}`, http.StatusBadRequest)
		return
	}

	// Pagination params (optional; default page 1, size 50, max 200)
	q := r.URL.Query()
	page := 1
	pageSize := 50

	if pStr := q.Get("page"); pStr != "" {
		if p, err := strconv.Atoi(pStr); err == nil && p > 0 {
			page = p
		}
	}
	if psStr := q.Get("page_size"); psStr != "" {
		if ps, err := strconv.Atoi(psStr); err == nil && ps > 0 && ps <= 200 {
			pageSize = ps
		}
	}

	type addressTx struct {
		*blockchaincomponent.Transaction
		BlockNumber *uint64 `json:"block_number,omitempty"`
		Source      string  `json:"source,omitempty"`
	}

	bc := bcs.BlockchainPtr

	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()

	// Collect and de-duplicate by tx_hash (lowercase)
	byHash := make(map[string]addressTx)

	// 1) Pending / mempool
	for _, tx := range bc.Transaction_pool {
		if !txTouchesAddress(tx, addr) {
			continue
		}
		h := strings.ToLower(strings.TrimSpace(tx.TxHash))
		if h == "" {
			continue
		}
		// Prefer confirmed later, so only set if not present
		if _, exists := byHash[h]; !exists {
			byHash[h] = addressTx{
				Transaction: tx,
				Source:      "mempool",
			}
		}
	}

	// 2) Confirmed transactions from in-memory blocks
	for _, blk := range bc.Blocks {
		blockNum := blk.BlockNumber
		for _, tx := range blk.Transactions {
			if !txTouchesAddress(tx, addr) {
				continue
			}
			h := strings.ToLower(strings.TrimSpace(tx.TxHash))
			if h == "" {
				continue
			}
			// Confirmed tx overrides mempool copy
			byHash[h] = addressTx{
				Transaction: tx,
				BlockNumber: &blockNum,
				Source:      "block",
			}
		}
	}

	// 3) Recent history (failed / expired / very old)
	// NOTE: This assumes you have a RecentTxs slice as discussed earlier.
	for _, tx := range bc.RecentTxs {
		if !txTouchesAddress(tx, addr) {
			continue
		}
		h := strings.ToLower(strings.TrimSpace(tx.TxHash))
		if h == "" {
			continue
		}
		// Only add if we don't already have mempool/confirmed
		if _, exists := byHash[h]; !exists {
			byHash[h] = addressTx{
				Transaction: tx,
				Source:      "recent",
			}
		}
	}

	// Flatten into slice
	list := make([]addressTx, 0, len(byHash))
	for _, v := range byHash {
		list = append(list, v)
	}

	// Sort by timestamp (newest first)
	sort.Slice(list, func(i, j int) bool {
		ti := uint64(0)
		tj := uint64(0)
		if list[i].Transaction != nil {
			ti = list[i].Transaction.Timestamp
		}
		if list[j].Transaction != nil {
			tj = list[j].Transaction.Timestamp
		}
		return ti > tj
	})

	// Pagination
	total := len(list)
	start := (page - 1) * pageSize
	if start >= total {
		json.NewEncoder(w).Encode([]addressTx{})
		return
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	paged := list[start:end]

	// For backward-compat with your frontend, return just an array.
	// If you later want metadata (total/pages), change to an object.
	json.NewEncoder(w).Encode(paged)
}

func txTouchesAddress(tx *blockchaincomponent.Transaction, addr string) bool {
	if tx == nil {
		return false
	}
	a := strings.ToLower(addr)
	if a == "" {
		return false
	}
	if strings.ToLower(strings.TrimSpace(tx.From)) == a {
		return true
	}
	if strings.ToLower(strings.TrimSpace(tx.To)) == a {
		return true
	}
	return false
}

func (bcs *BlockchainServer) BlockTimeLatest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	bc := bcs.BlockchainPtr

	if len(bc.Blocks) < 1 {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "not enough blocks",
		})
		return
	}

	last := bc.Blocks[len(bc.Blocks)-1]

	// mining duration measured in MineNewBlock
	mining := bc.LastBlockMiningTime

	// optional: also keep interval between block timestamps for comparison
	var interval time.Duration
	if len(bc.Blocks) >= 2 {
		//prev := bc.Blocks[len(bc.Blocks)-2]
		//interval = last.Timestamp.Sub(prev.Timestamp)
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"block_number":      last.BlockNumber,
		"mining_time_ms":    mining.Milliseconds(),
		"mining_time_sec":   mining.Seconds(),
		"interval_time_ms":  interval.Milliseconds(),
		"interval_time_sec": interval.Seconds(),
	})
}

func (bcs *BlockchainServer) MineBlock(w http.ResponseWriter, r *http.Request) {
	blk := bcs.BlockchainPtr.MineNewBlock()
	json.NewEncoder(w).Encode(blk)
}

func (bcs *BlockchainServer) ProvideLiquidity(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "invalid method", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Address  string     `json:"address"`
		Amount   amountField `json:"amount"`
		LockDays int64      `json:"lock_days"`
	}

	body, _ := io.ReadAll(r.Body)
	json.Unmarshal(body, &req)

	if req.Amount.V == nil || req.Amount.V.Sign() <= 0 {
		http.Error(w, `{"error":"invalid amount"}`, 400)
		return
	}

	err := bcs.BlockchainPtr.ProvideLiquidity(req.Address, req.Amount.V, req.LockDays)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), 400)
		return
	}

	io.WriteString(w, `{"status":"success"}`)
}

func (bcs *BlockchainServer) UnstakeLiquidity(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Address string `json:"address"`
	}

	body, _ := io.ReadAll(r.Body)
	json.Unmarshal(body, &req)

	err := bcs.BlockchainPtr.StartUnstake(req.Address)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), 400)
		return
	}

	io.WriteString(w, `{"status":"unstake_started"}`)
}

func (bcs *BlockchainServer) GetLiquidityInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	bcs.BlockchainPtr.ProcessUnstakeReleases()
	address := r.URL.Query().Get("address")
	lp := bcs.BlockchainPtr.LiquidityProviders[address]
	if lp == nil {
		io.WriteString(w, `{"exists":false}`)
		return
	}

	json.NewEncoder(w).Encode(lp)
}

func (bcs *BlockchainServer) GetAllLiquidityProviders(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	out := []*blockchaincomponent.LiquidityProvider{}
	for _, lp := range bcs.BlockchainPtr.LiquidityProviders {
		out = append(out, lp)
	}

	json.NewEncoder(w).Encode(out)
}

func (bcs *BlockchainServer) GetPoolLiquidity(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	bcs.BlockchainPtr.Mutex.Lock()
	defer bcs.BlockchainPtr.Mutex.Unlock()

	total := big.NewInt(0)
	for _, v := range bcs.BlockchainPtr.PoolLiquidity {
		if v != nil {
			total.Add(total, v)
		}
	}
	target := big.NewInt(0)
	if len(bcs.BlockchainPtr.PoolLiquidity) > 0 {
		target.Div(total, big.NewInt(int64(len(bcs.BlockchainPtr.PoolLiquidity))))
	}
	json.NewEncoder(w).Encode(map[string]any{
		"pools":        bcs.BlockchainPtr.PoolLiquidity,
		"total":        blockchaincomponent.AmountString(total),
		"target_equal": blockchaincomponent.AmountString(target),
		"unallocated":  blockchaincomponent.AmountString(bcs.BlockchainPtr.UnallocatedLiquidity),
	})
}

func (bcs *BlockchainServer) RebalancePools(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	bcs.BlockchainPtr.Mutex.Lock()
	defer bcs.BlockchainPtr.Mutex.Unlock()
	bcs.BlockchainPtr.RebalancePoolsEqual()
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// SyncPools registers existing contracts that qualify as pools.
// This is optional for developers when older contracts predate pool detection.
func (bcs *BlockchainServer) SyncPools(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	bcs.BlockchainPtr.Mutex.Lock()
	defer bcs.BlockchainPtr.Mutex.Unlock()

	addrs := bcs.BlockchainPtr.ContractEngine.Registry.List()
	for _, addr := range addrs {
		rec, err := bcs.BlockchainPtr.ContractEngine.Registry.LoadContract(addr)
		if err != nil || rec == nil || rec.Metadata == nil {
			continue
		}
		if rec.Metadata.Pool {
			bcs.BlockchainPtr.RegisterPool(addr)
		}
	}

	json.NewEncoder(w).Encode(map[string]any{
		"status": "ok",
		"count":  len(bcs.BlockchainPtr.PoolLiquidity),
	})
}

// RegisterPoolManual allows devs to force a pool registration.
func (bcs *BlockchainServer) RegisterPoolManual(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Address string `json:"address"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if req.Address == "" {
		http.Error(w, "address required", 400)
		return
	}

	bcs.BlockchainPtr.Mutex.Lock()
	defer bcs.BlockchainPtr.Mutex.Unlock()
	bcs.BlockchainPtr.RegisterPool(req.Address)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
func (b *BlockchainServer) GetContractStorage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	address := r.URL.Query().Get("address")

	data := b.BlockchainPtr.ContractEngine.Registry.Load(address)
	json.NewEncoder(w).Encode(data)
}

func (bcs *BlockchainServer) ContractDeploy(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	type Req struct {
		Type       string `json:"type"`
		Owner      string `json:"owner"`
		Code       string `json:"code"`
		PluginPath string `json:"plugin_path"`
		PrivateKey string `json:"private_key"`
		Gas        uint64 `json:"gas"`
		GasPrice   uint64 `json:"gas_price"`
	}

	var req Req
	var codeBytes []byte

	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "multipart/form-data") {
		if err := r.ParseMultipartForm(25 << 20); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		req.Type = r.FormValue("type")
		req.Owner = r.FormValue("owner")
		req.PluginPath = r.FormValue("plugin_path")
		req.PrivateKey = r.FormValue("private_key")
		if gasStr := r.FormValue("gas"); gasStr != "" {
			if v, err := strconv.ParseUint(gasStr, 10, 64); err == nil {
				req.Gas = v
			}
		}
		if gpStr := r.FormValue("gas_price"); gpStr != "" {
			if v, err := strconv.ParseUint(gpStr, 10, 64); err == nil {
				req.GasPrice = v
			}
		}

		file, _, err := r.FormFile("contract_file")
		if err == nil && file != nil {
			defer file.Close()
			codeBytes, _ = io.ReadAll(file)
		}
	} else {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if req.Code != "" {
			codeBytes = []byte(req.Code)
		}
	}

	if req.Owner == "" || !wallet.ValidateAddress(req.Owner) {
		http.Error(w, "invalid owner address", 400)
		return
	}
	if req.Type == "" {
		http.Error(w, "contract type required", 400)
		return
	}

	// 1. Generate contract address
	addr := GenerateContractAddress(req.Owner, uint64(time.Now().UnixNano()))

	// 2. Build metadata
	meta := &blockchaincomponent.ContractMetadata{
		Address:   addr,
		Type:      req.Type,
		Owner:     req.Owner,
		Timestamp: time.Now().Unix(),
	}

	// 3. Save code/plugin
	switch req.Type {
	case "plugin":
		if req.PluginPath != "" {
			meta.PluginPath = req.PluginPath
			break
		}
		if len(codeBytes) == 0 {
			http.Error(w, "plugin file required", 400)
			return
		}
		pluginDir := filepath.Join("data", "contracts")
		_ = os.MkdirAll(pluginDir, 0o755)
		pluginPath := filepath.Join(pluginDir, addr+".so")
		if err := os.WriteFile(pluginPath, codeBytes, 0o755); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		meta.PluginPath = pluginPath
	case "gocode", "dsl":
		if len(codeBytes) == 0 {
			http.Error(w, "contract code required", 400)
			return
		}
		meta.Code = codeBytes
	default:
		http.Error(w, "invalid contract type", 400)
		return
	}

	// 4. Generate ABI
	var abi []byte
	var err error

	switch req.Type {
	case "plugin":
		if err := bcs.BlockchainPtr.ContractEngine.Registry.PluginVM.LoadPlugin(addr, meta.PluginPath); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		pc := bcs.BlockchainPtr.ContractEngine.Registry.PluginVM.GetPlugin(addr)
		if pc == nil {
			http.Error(w, "plugin load failed", 500)
			return
		}
		abi, err = blockchaincomponent.GenerateABIForPlugin(pc)
	case "gocode":
		abi, err = blockchaincomponent.GenerateABIForBytecode(nil)
	case "dsl":
		abi, err = blockchaincomponent.GenerateABIForDSL()
	}
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	meta.ABI = abi
	meta.Pool, meta.PoolType = detectPoolFromABI(abi)

	// 5. Create initial state
	state := &blockchaincomponent.SmartContractState{
		Address:   addr,
		Balance:   "0",
		Storage:   map[string]string{},
		IsActive:  true,
		CreatedAt: time.Now().Unix(),
	}

	// 6. Register
	if err := bcs.BlockchainPtr.ContractEngine.Registry.RegisterContract(meta, state); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if meta.Pool {
		bcs.BlockchainPtr.RegisterPool(addr)
	}

	// 7. Create contract deployment transaction (fee-paying)
	fnPayload := map[string]any{
		"fn":   "constructor",
		"args": []string{},
	}
	payloadBytes, _ := json.Marshal(fnPayload)
	deployTx := &blockchaincomponent.Transaction{
		From:       req.Owner,
		To:         addr,
		Type:       "contract_create",
		Function:   "constructor",
		Args:       []string{},
		Data:       payloadBytes,
		Timestamp:  uint64(time.Now().Unix()),
		Status:     constantset.StatusPending,
		IsSystem:   false,
		ChainID:    uint64(constantset.ChainID),
		Gas:        uint64(constantset.ContractCallGas),
		GasPrice:   1,
		IsContract: true,
	}
	if req.Gas > 0 {
		deployTx.Gas = req.Gas
	}
	if req.GasPrice > 0 {
		deployTx.GasPrice = req.GasPrice
	} else {
		deployTx.GasPrice = bcs.BlockchainPtr.CalculateBaseFee() + 1
	}
	if req.PrivateKey != "" {
		signer, err := wallet.ImportFromPrivateKey(req.PrivateKey)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"Failed to import wallet: %v"}`, err), http.StatusBadRequest)
			return
		}
		if !strings.EqualFold(signer.Address, req.Owner) {
			http.Error(w, `{"error":"Owner does not match private key"}`, http.StatusBadRequest)
			return
		}
		_ = signer.SignTransaction(deployTx)
	}
	deployTx.TxHash = blockchaincomponent.CalculateTransactionHash(*deployTx)

	bcs.BlockchainPtr.AddNewTxToTheTransaction_pool(deployTx)
	bcs.BlockchainPtr.RecordRecentTx(deployTx)

	// 8. Respond
	json.NewEncoder(w).Encode(map[string]any{
		"status":  "deployed",
		"address": addr,
		"type":    req.Type,
		"owner":   req.Owner,
	})
}

func detectPoolFromABI(abi []byte) (bool, string) {
	if len(abi) == 0 {
		return false, ""
	}
	var entries []map[string]any
	if err := json.Unmarshal(abi, &entries); err != nil {
		var wrapped struct {
			Entries []map[string]any `json:"entries"`
		}
		if err := json.Unmarshal(abi, &wrapped); err != nil {
			return false, ""
		}
		entries = wrapped.Entries
	}
	names := map[string]struct{}{}
	for _, e := range entries {
		if n, ok := e["name"].(string); ok {
			names[strings.ToLower(n)] = struct{}{}
		}
	}

	has := func(n string) bool {
		_, ok := names[strings.ToLower(n)]
		return ok
	}

	if has("addliquidity") || has("removeliquidity") || has("swap") || has("swapatob") || has("swapbtoa") {
		return true, "dex"
	}
	if (has("deposit") && has("withdraw")) || has("borrow") || has("repay") {
		return true, "lending"
	}
	if has("mint") && has("ownerof") {
		return true, "nft"
	}

	return false, ""
}

func (bcs *BlockchainServer) ContractCall(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	type Req struct {
		Address string   `json:"address"`
		Caller  string   `json:"caller"`
		Fn      string   `json:"fn"`
		Args    []string `json:"args"`
	}

	var req Req
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	// Execute WITHOUT a transaction (direct call)
	res, err := bcs.BlockchainPtr.ContractEngine.Pipeline.ApplyContractCall(
		req.Address,
		req.Caller,
		req.Fn,
		req.Args,
	)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	json.NewEncoder(w).Encode(res)
}

func (bcs *BlockchainServer) ContractState(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	addr := r.URL.Query().Get("address")

	rec, err := bcs.BlockchainPtr.ContractEngine.Registry.LoadContract(addr)
	if err != nil {
		http.Error(w, err.Error(), 404)
		return
	}

	json.NewEncoder(w).Encode(rec)
}

func (bcs *BlockchainServer) ContractABI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	addr := r.URL.Query().Get("address")

	abi, err := bcs.BlockchainPtr.ContractEngine.Registry.LoadABI(addr)
	if err != nil {
		http.Error(w, err.Error(), 404)
		return
	}

	w.Write(abi)
}

func (b *BlockchainServer) ContractList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	addrs := b.BlockchainPtr.ContractEngine.Registry.List()
	out := []map[string]any{}
	for _, addr := range addrs {
		rec, err := b.BlockchainPtr.ContractEngine.Registry.LoadContract(addr)
		if err != nil || rec == nil || rec.Metadata == nil {
			continue
		}
		out = append(out, map[string]any{
			"address": addr,
			"type":    rec.Metadata.Type,
			"owner":   rec.Metadata.Owner,
		})
	}
	json.NewEncoder(w).Encode(out)
}

func (b *BlockchainServer) BaseFee(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	fee := b.BlockchainPtr.CalculateBaseFee()
	json.NewEncoder(w).Encode(map[string]uint64{"base_fee": fee})
}

func GenerateContractAddress(owner string, nonce uint64) string {
	input := owner + ":" + strconv.FormatUint(nonce, 10)
	sum := sha256.Sum256([]byte(input))
	return "0x" + hex.EncodeToString(sum[:20])
}
func (b *BlockchainServer) GetContractCode(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	address := r.URL.Query().Get("address")

	reg := b.BlockchainPtr.ContractEngine.Registry
	con := reg.Load(address)

	if con == nil {
		json.NewEncoder(w).Encode(map[string]string{
			"error": "contract not found",
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"type":       con.Type,
		"source":     con.SourceCode,
		"bytecode":   con.Bytecode,
		"pluginPath": con.PluginPath,
	})
}
func (b *BlockchainServer) StreamContractEvents(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	address := r.URL.Query().Get("address") // optional

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	prev := len(b.BlockchainPtr.ContractEngine.EventDB.GetEventsFromDB(address))

	for {
		events := b.BlockchainPtr.ContractEngine.Registry.EventDB.GetEventsFromDB(address)

		if len(events) > prev {
			newEvents := events[prev:]

			for _, ev := range newEvents {
				if address != "" && ev.Address != address {
					continue
				}

				evJSON, _ := json.Marshal(ev)
				fmt.Fprintf(w, "data: %s\n\n", evJSON)
				flusher.Flush()
			}

			prev = len(events)
		}

		time.Sleep(1 * time.Second)
	}
}
func (b *BlockchainServer) CompileContract(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	var req struct {
		Type   string `json:"type"`   // solidity | gocode | dsl
		Source string `json:"source"` // contract code
	}

	json.NewDecoder(r.Body).Decode(&req)

	switch req.Type {

	case "solidity":
		http.Error(w, "solidity compiler not supported in LQD engine", 400)
		return

	case "gocode":
		bc, err := b.BlockchainPtr.ContractEngine.Registry.IVM.CompileGoSubset(req.Source)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"bytecode": bc})
		return

	case "dsl":
		code, err := b.BlockchainPtr.ContractEngine.Registry.DSL.CompileDSL(req.Source)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"bytecode": code})
		return
	}

	http.Error(w, "invalid contract type", 400)
}
func (bcs *BlockchainServer) ContractEvents(w http.ResponseWriter, r *http.Request) {
	addr := r.URL.Query().Get("address")
	blockStr := r.URL.Query().Get("block")

	if blockStr == "" {
		http.Error(w, "block required", 400)
		return
	}

	block, _ := strconv.ParseUint(blockStr, 10, 64)

	events, _ := bcs.BlockchainPtr.ContractEngine.EventDB.GetEventsByBlock(block)

	// filter for contract
	out := []blockchaincomponent.ContractEvent{}
	for _, ev := range events {
		if ev.Address == addr {
			out = append(out, ev)
		}
	}

	json.NewEncoder(w).Encode(out)
}
func (b *BlockchainServer) ContractCompile(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		Type   string `json:"type"`
		Source string `json:"source"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	switch req.Type {

	case "solidity":
		tmp := "./tmp_compile.sol"
		os.WriteFile(tmp, []byte(req.Source), 0644)

		out, err := exec.Command("solc", "--combined-json", "abi,bin", tmp).CombinedOutput()
		if err != nil {
			http.Error(w, string(out), 500)
			return
		}
		w.Write(out)
		return

	case "gocode":
		bytecode, err := b.BlockchainPtr.ContractEngine.Registry.IVM.CompileGoSubset(req.Source)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"bytecode": bytecode})
		return

	case "dsl":
		code, err := b.BlockchainPtr.ContractEngine.Registry.DSL.CompileDSL(req.Source)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"bytecode": code})
		return

	default:
		http.Error(w, "invalid compiler type", 400)
	}
}

func (b *BlockchainServer) Start() {
	portStr := fmt.Sprintf("%d", b.Port)

	http.HandleFunc("/", b.getBlockchain)
	http.HandleFunc("/balance", b.GetBalance)
	http.HandleFunc("/send_tx", b.sendTransaction)
	http.HandleFunc("/send_tx/batch", b.sendTransactionBatch)
	http.HandleFunc("/fetch_last_n_block", b.fetchNBlocks)
	http.HandleFunc("/account/{address}/nonce", b.GetAccountNonce)
	http.HandleFunc("/getheight", b.GetBlockchainHeight)
	http.HandleFunc("/validator/{address}", b.ValidatorStats)
	http.HandleFunc("/network", b.NetworkStats)
	http.HandleFunc("/peers", b.GetPeers)
	http.HandleFunc("/peers/add", b.AddPeer)
	http.HandleFunc("/faucet", b.Faucet)
	http.HandleFunc("/block/{id}", b.GetBlock)
	http.HandleFunc("/validators", b.GetValidators)
	http.HandleFunc("/transactions/recent", b.GetRecentTransactions)
	http.HandleFunc("/liquidity/lock", b.LiquidityLock)
	http.HandleFunc("/liquidity/unlock", b.LiquidityUnlock)
	http.HandleFunc("/liquidity", b.LiquidityView)
	http.HandleFunc("/rewards/recent", b.RewardsRecent)
	http.HandleFunc("/rewards/latest", b.RewardsLatest)
	http.HandleFunc("/validators/active", b.ActiveValidatorsLatest)
	http.HandleFunc("/validators/sync", b.SyncValidatorsAll)
	http.HandleFunc("/chain/summary", b.ChainSummary)
	http.HandleFunc("/chain/global", b.GlobalChainSummary)
	http.HandleFunc("/rpc", b.JSONRPC)
	http.HandleFunc("/validator/new", b.AddValidatorFromPeer)
	http.HandleFunc("/validator/add", b.AddValidator)
	http.HandleFunc("/tx/", b.GetTransactionByHash)
	http.HandleFunc("/contract/deploy", b.ContractDeploy)
	http.HandleFunc("/contract/call", b.ContractCall)
	http.HandleFunc("/contract/getAbi", b.ContractABI)
	http.HandleFunc("/contract/con1", b.ContractState)
	http.HandleFunc("/contract/con2", b.ContractEvents)
	http.HandleFunc("/contract/list", b.ContractList)
	http.HandleFunc("/contract/compile", b.CompileContract)
	http.HandleFunc("/contract/storage", b.GetContractStorage)
	http.HandleFunc("/contract/code", b.GetContractCode)
	http.HandleFunc("/contract/events", b.ContractEvents)
	http.HandleFunc("/basefee", b.BaseFee)
	http.HandleFunc("/blocktime/latest", b.BlockTimeLatest)
	http.HandleFunc("/address/{address}/transactions", b.GetAddressTransactions)
	http.HandleFunc("/liquidity/provide", b.ProvideLiquidity)
	http.HandleFunc("/liquidity/unstake", b.UnstakeLiquidity)
	http.HandleFunc("/liquidity/info", b.GetLiquidityInfo)
	http.HandleFunc("/liquidity/all", b.GetAllLiquidityProviders)
	http.HandleFunc("/liquidity/pools", b.GetPoolLiquidity)
	http.HandleFunc("/liquidity/rebalance", b.RebalancePools)
	http.HandleFunc("/liquidity/pools/sync", b.SyncPools)
	http.HandleFunc("/liquidity/pools/register", b.RegisterPoolManual)
	http.HandleFunc("/bridge/requests", b.GetBridgeRequests)
	http.HandleFunc("/bridge/tokens", b.GetBridgeTokens)
	http.HandleFunc("/bridge/lock_bsc", b.BridgeLockBsc)
	http.HandleFunc("/bridge/burn_lqd", b.BridgeBurnLqd)

	//http.HandleFunc("/contract/compile", b.CompileContract)

	// http.HandleFunc("/contract/compile", b.ContractCompile)
	// http.HandleFunc("/contract/events", b.GetContractEvents)
	// http.HandleFunc("/contract/storage", b.GetContractStorage)
	// http.HandleFunc("/contract/code", b.GetContractCode)
	// http.HandleFunc("/contract/streamEvents", b.StreamContractEvents)

	//http.HandleFunc("/token-balance", b.GetTokenBalance)

	log.Println("Blockchain server is starting on port:", b.Port)

	// Use the CORS handler
	err := http.ListenAndServe("127.0.0.1:"+portStr, nil)
	if err != nil {
		log.Fatalf("Failed to start blockchain server: %v", err)
	}
	log.Println("Blockchain server started successfully")
}
