package blockchainserver

import (
	"crypto/sha256"
	"encoding/base64"
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

	blockchaincomponent "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/BlockchainComponent"
	constantset "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/ConstantSet"
	wallet "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/WalletComponent"
	"github.com/gorilla/mux"
)

type BlockchainServer struct {
	Port          uint                                   `json:"port"`
	BlockchainPtr *blockchaincomponent.Blockchain_struct `json:"blockchain_ptr"`
	limiter       *rateLimiter
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
		limiter:       newRateLimiter(100, 200),
	}
}

func (b *BlockchainServer) getBlockchain(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	setCORSHeaders(w, r)
	if r.Method == http.MethodGet {
		if r.URL.Query().Get("full") == "1" || strings.EqualFold(r.URL.Query().Get("full"), "true") {
			io.WriteString(w, b.BlockchainPtr.ToJsonChain())
			return
		}

		b.BlockchainPtr.Mutex.Lock()
		height := len(b.BlockchainPtr.Blocks)
		peers := len(b.BlockchainPtr.Network.Peers)
		mempool := len(b.BlockchainPtr.Transaction_pool)
		validators := len(b.BlockchainPtr.Validators)
		baseFee := b.BlockchainPtr.BaseFee
		var latestHash string
		if height > 0 && b.BlockchainPtr.Blocks[height-1] != nil {
			latestHash = b.BlockchainPtr.Blocks[height-1].CurrentHash
		}
		b.BlockchainPtr.Mutex.Unlock()

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":            "ok",
			"service":           "podl-chain",
			"height":            height,
			"peers":             peers,
			"mempool":           mempool,
			"validators":        validators,
			"base_fee":          baseFee,
			"latest_block_hash": latestHash,
			"full_chain_path":   "/?full=1",
			"health_path":       "/health",
			"timestamp":         time.Now().Unix(),
		})
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
}

func (b *BlockchainServer) GetFullBlockchain(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	setCORSHeaders(w, r)
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	io.WriteString(w, b.BlockchainPtr.ToJsonChain())
}

func (bcs *BlockchainServer) GetAccountNonce(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	setCORSHeaders(w, r)

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
	setCORSHeaders(w, r)
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
	setCORSHeaders(w, r)
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
	setCORSHeaders(w, r)

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// `n` keeps the dashboard recent-blocks widget lightweight.
	// `page`/`size` powers the full historical blocks explorer.
	q := r.URL.Query()

	pageStr := q.Get("page")
	sizeStr := q.Get("size")

	if pageStr != "" || sizeStr != "" {
		page, _ := strconv.Atoi(pageStr)
		size, _ := strconv.Atoi(sizeStr)
		result, total, totalPages, err := blockchaincomponent.GetPaginatedBlocksFromDB(page, size)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to paginate blocks: %v", err), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"blocks":      result,
			"total":       total,
			"page":        page,
			"size":        size,
			"total_pages": totalPages,
		})
		return
	}

	// legacy mode: return last n blocks (default 14 for dashboard/recent views)
	n := 14
	if nStr := q.Get("n"); nStr != "" {
		if parsed, err := strconv.Atoi(nStr); err == nil && parsed > 0 {
			n = parsed
		}
	}
	blocksToReturn, _, err := blockchaincomponent.GetRecentBlocksFromDB(n)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to load recent blocks: %v", err), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(blocksToReturn)
}
func (bcs *BlockchainServer) GetBlockchainHeight(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	setCORSHeaders(w, r)
	height, err := blockchaincomponent.GetLatestBlockNumberFromDB()
	if err != nil {
		height = uint64(len(bcs.BlockchainPtr.Blocks))
	}
	json.NewEncoder(w).Encode(map[string]uint64{"height": height})
}

func (bcs *BlockchainServer) GetBalance(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	setCORSHeaders(w, r)

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
	setCORSHeaders(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	addr := r.URL.Query().Get("address")
	mode := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("mode")))
	list := bcs.BlockchainPtr.ListBridgeRequestsView(addr)
	if mode == "public" || mode == "private" {
		filtered := make([]*blockchaincomponent.BridgeRequest, 0, len(list))
		for _, req := range list {
			if req == nil {
				continue
			}
			if strings.EqualFold(req.Mode, mode) {
				filtered = append(filtered, req)
			}
		}
		list = filtered
	}
	json.NewEncoder(w).Encode(list)
}

func (bcs *BlockchainServer) GetBridgeTokens(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	setCORSHeaders(w, r)
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

func (bcs *BlockchainServer) persistBridgeTokenState() error {
	bcs.BlockchainPtr.Mutex.Lock()
	dbCopy := *bcs.BlockchainPtr
	dbCopy.Mutex = sync.Mutex{}
	bcs.BlockchainPtr.Mutex.Unlock()
	return blockchaincomponent.PutIntoDB(dbCopy)
}

func persistBridgeTokenRegistry(info *blockchaincomponent.BridgeTokenInfo) error {
	reg, err := blockchaincomponent.LoadBridgeTokenRegistry()
	if err != nil {
		return err
	}
	reg.Upsert(info)
	return blockchaincomponent.SaveBridgeTokenRegistry(reg)
}

func removeBridgeTokenRegistry(chainID, sourceToken, lqdToken string) error {
	reg, err := blockchaincomponent.LoadBridgeTokenRegistry()
	if err != nil {
		return err
	}
	reg.Remove(chainID, sourceToken, lqdToken)
	return blockchaincomponent.SaveBridgeTokenRegistry(reg)
}

func (bcs *BlockchainServer) UpsertBridgeToken(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	setCORSHeaders(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !bridgeAdminKeyMatches(r) {
		http.Error(w, `{"error":"admin key required"}`, http.StatusUnauthorized)
		return
	}
	var req struct {
		ChainID         string `json:"chain_id"`
		ChainName       string `json:"chain_name"`
		SourceToken     string `json:"source_token"`
		TargetChainID   string `json:"target_chain_id"`
		TargetChainName string `json:"target_chain_name"`
		TargetToken     string `json:"target_token"`
		Name            string `json:"name"`
		Symbol          string `json:"symbol"`
		Decimals        string `json:"decimals"`
		BscToken        string `json:"bsc_token"`
		LqdToken        string `json:"lqd_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}
	sourceToken := req.SourceToken
	if sourceToken == "" {
		sourceToken = req.BscToken
	}
	lqdToken := req.LqdToken
	if lqdToken == "" {
		lqdToken = req.TargetToken
	}
	if req.ChainID == "" || sourceToken == "" || lqdToken == "" {
		http.Error(w, `{"error":"missing fields"}`, http.StatusBadRequest)
		return
	}
	info := &blockchaincomponent.BridgeTokenInfo{
		ChainID:         req.ChainID,
		ChainName:       req.ChainName,
		SourceToken:     sourceToken,
		TargetChainID:   req.TargetChainID,
		TargetChainName: req.TargetChainName,
		TargetToken:     lqdToken,
		BscToken:        sourceToken,
		LqdToken:        lqdToken,
		Name:            req.Name,
		Symbol:          req.Symbol,
		Decimals:        req.Decimals,
	}
	bcs.BlockchainPtr.SetBridgeTokenMappingForChain(req.ChainID, sourceToken, info)
	if err := persistBridgeTokenRegistry(info); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
		return
	}
	if err := bcs.persistBridgeTokenState(); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"status": "ok", "token": info})
}

func (bcs *BlockchainServer) RemoveBridgeToken(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	setCORSHeaders(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !bridgeAdminKeyMatches(r) {
		http.Error(w, `{"error":"admin key required"}`, http.StatusUnauthorized)
		return
	}
	var req struct {
		ChainID     string `json:"chain_id"`
		SourceToken string `json:"source_token"`
		LqdToken    string `json:"lqd_token"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.ChainID == "" {
		req.ChainID = r.URL.Query().Get("chain_id")
	}
	if req.SourceToken == "" {
		req.SourceToken = r.URL.Query().Get("source_token")
	}
	if req.LqdToken == "" {
		req.LqdToken = r.URL.Query().Get("lqd_token")
	}
	if req.ChainID == "" || (req.SourceToken == "" && req.LqdToken == "") {
		http.Error(w, `{"error":"missing fields"}`, http.StatusBadRequest)
		return
	}
	if req.SourceToken != "" {
		bcs.BlockchainPtr.RemoveBridgeTokenMappingForChain(req.ChainID, req.SourceToken)
	}
	if req.LqdToken != "" {
		bcs.BlockchainPtr.RemoveBridgeTokenMappingByLqdForChain(req.ChainID, req.LqdToken)
	}
	if err := removeBridgeTokenRegistry(req.ChainID, req.SourceToken, req.LqdToken); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
		return
	}
	if err := bcs.persistBridgeTokenState(); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
}

func bridgeAdminKeyMatches(r *http.Request) bool {
	requiredKey := strings.TrimSpace(os.Getenv("LQD_API_KEY"))
	if requiredKey == "" {
		return true
	}
	clientKey := strings.TrimSpace(r.Header.Get("X-API-Key"))
	if clientKey == "" {
		clientKey = strings.TrimSpace(r.URL.Query().Get("api_key"))
	}
	return clientKey != "" && clientKey == requiredKey
}

func bridgeLookupChain(chainID string) *blockchaincomponent.BridgeChainConfig {
	reg, err := blockchaincomponent.LoadBridgeChainRegistry()
	if err != nil || reg == nil {
		return nil
	}
	if cfg := reg.ChainByID(chainID); cfg != nil {
		return cfg
	}
	if cfg := reg.ChainByName(chainID); cfg != nil {
		return cfg
	}
	if cfg := reg.AnyEnabled(); cfg != nil {
		return cfg
	}
	return nil
}

func (bcs *BlockchainServer) GetBridgeFamilies(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	setCORSHeaders(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	json.NewEncoder(w).Encode(blockchaincomponent.SupportedBridgeFamilies())
}

func (bcs *BlockchainServer) GetBridgeChains(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	setCORSHeaders(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	reg, err := blockchaincomponent.LoadBridgeChainRegistry()
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(reg.List())
}

func (bcs *BlockchainServer) UpsertBridgeChain(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	setCORSHeaders(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !bridgeAdminKeyMatches(r) {
		http.Error(w, `{"error":"admin key required"}`, http.StatusUnauthorized)
		return
	}
	var cfg blockchaincomponent.BridgeChainConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}
	reg, err := blockchaincomponent.LoadBridgeChainRegistry()
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
		return
	}
	reg.Upsert(&cfg)
	if err := blockchaincomponent.SaveBridgeChainRegistry(reg); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"status": "ok", "chain": cfg})
}

func (bcs *BlockchainServer) RemoveBridgeChain(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	setCORSHeaders(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !bridgeAdminKeyMatches(r) {
		http.Error(w, `{"error":"admin key required"}`, http.StatusUnauthorized)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && r.Method != http.MethodDelete {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}
	if req.ID == "" {
		req.ID = r.URL.Query().Get("id")
	}
	if req.ID == "" {
		http.Error(w, `{"error":"missing id"}`, http.StatusBadRequest)
		return
	}
	reg, err := blockchaincomponent.LoadBridgeChainRegistry()
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
		return
	}
	reg.Remove(req.ID)
	if err := blockchaincomponent.SaveBridgeChainRegistry(reg); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"status": "ok", "removed": req.ID})
}

func bridgeNormalizeMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode != "private" {
		return "public"
	}
	return "private"
}

// BridgeLockBsc registers a BSC lock request directly (fallback when RPC log scan misses).
func (bcs *BlockchainServer) BridgeLockBsc(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	setCORSHeaders(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ChainID string `json:"chain_id"`
		BscTx   string `json:"bsc_tx"`
		Token   string `json:"token"`
		From    string `json:"from"`
		ToLqd   string `json:"to_lqd"`
		Amount  string `json:"amount"`
		Mode    string `json:"mode"`
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
	if req.BscTx != "" {
		endpoints := blockchaincomponent.BridgeRPCEndpoints(os.Getenv("BSC_TESTNET_RPC"))
		if len(endpoints) > 0 {
			if consensus, rerr := blockchaincomponent.ConsensusReceipt(endpoints, req.BscTx, 2*time.Minute, 2*time.Second); rerr != nil || !blockchaincomponent.ReceiptSuccessful(consensus.Receipt) {
				msg := "bridge lock pending"
				if rerr != nil {
					msg = fmt.Sprintf("bridge lock receipt wait failed: %v", rerr)
				}
				http.Error(w, fmt.Sprintf(`{"error":"%s"}`, msg), http.StatusBadRequest)
				return
			}
		}
	}
	chainID := strings.TrimSpace(req.ChainID)
	if chainID == "" {
		chainID = "bsc-testnet"
	}
	if strings.EqualFold(req.Mode, "private") {
		bcs.BlockchainPtr.AddPrivateBridgeRequestChain(chainID, req.BscTx, req.Token, req.From, req.ToLqd, amt)
	} else {
		bcs.BlockchainPtr.AddBridgeRequestChain(chainID, req.BscTx, req.Token, req.From, req.ToLqd, amt)
	}
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}

func (bcs *BlockchainServer) BridgeLockChain(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	setCORSHeaders(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ChainID        string `json:"chain_id"`
		Family         string `json:"family"`
		Adapter        string `json:"adapter"`
		BscTx          string `json:"tx_hash"`
		SourceTxHash   string `json:"source_tx_hash"`
		SourceAddress  string `json:"source_address"`
		SourceMemo     string `json:"source_memo"`
		SourceSequence string `json:"source_sequence"`
		SourceOutput   string `json:"source_output"`
		Token          string `json:"token"`
		From           string `json:"from"`
		ToLqd          string `json:"to_lqd"`
		Amount         string `json:"amount"`
		Mode           string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}
	if req.ChainID == "" || req.BscTx == "" || req.Token == "" || req.ToLqd == "" || req.Amount == "" {
		http.Error(w, `{"error":"missing fields"}`, http.StatusBadRequest)
		return
	}
	if req.SourceTxHash == "" {
		req.SourceTxHash = req.BscTx
	}
	amt, err := blockchaincomponent.NewAmountFromString(req.Amount)
	if err != nil || amt.Sign() <= 0 {
		http.Error(w, `{"error":"invalid amount"}`, http.StatusBadRequest)
		return
	}
	family := strings.ToLower(strings.TrimSpace(req.Family))
	if family == "" {
		if cfg := bridgeLookupChain(req.ChainID); cfg != nil {
			family = strings.ToLower(strings.TrimSpace(cfg.Family))
		}
	}
	reqBridge := &blockchaincomponent.BridgeRequest{
		SourceTxHash:   req.SourceTxHash,
		SourceAddress:  req.SourceAddress,
		SourceMemo:     req.SourceMemo,
		SourceSequence: req.SourceSequence,
		SourceOutput:   req.SourceOutput,
	}
	if err := blockchaincomponent.ValidateBridgeRequestMetadata(family, reqBridge); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}
	if strings.EqualFold(req.Mode, "private") {
		bcs.BlockchainPtr.AddPrivateBridgeRequestChainWithMetadata(req.ChainID, req.BscTx, req.Token, req.From, req.ToLqd, amt, req.SourceAddress, req.SourceMemo, req.SourceSequence, req.SourceOutput)
	} else {
		bcs.BlockchainPtr.AddBridgeRequestChainWithMetadata(req.ChainID, req.BscTx, req.Token, req.From, req.ToLqd, amt, req.SourceAddress, req.SourceMemo, req.SourceSequence, req.SourceOutput)
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// BridgeBurnLqd executes a burn on LQD and registers a release request for BSC.
func (bcs *BlockchainServer) BridgeBurnLqd(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	setCORSHeaders(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Token  string `json:"token"`
		From   string `json:"from"`
		ToBsc  string `json:"to_bsc"`
		Amount string `json:"amount"`
		Mode   string `json:"mode"`
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

	if strings.EqualFold(req.Mode, "private") {
		bcs.BlockchainPtr.AddPrivateBridgeRequestBurn(tx.TxHash, info.BscToken, req.From, req.ToBsc, amt)
	} else {
		bcs.BlockchainPtr.AddBridgeRequestBurn(tx.TxHash, info.BscToken, req.From, req.ToBsc, amt)
	}

	json.NewEncoder(w).Encode(map[string]string{
		"tx_hash": tx.TxHash,
		"status":  "burned",
	})
}

func (bcs *BlockchainServer) BridgeBurnChain(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	setCORSHeaders(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ChainID        string `json:"chain_id"`
		Family         string `json:"family"`
		Adapter        string `json:"adapter"`
		Token          string `json:"token"`
		From           string `json:"from"`
		ToAddr         string `json:"to_addr"`
		ToBsc          string `json:"to_bsc"`
		Amount         string `json:"amount"`
		SourceTxHash   string `json:"source_tx_hash"`
		SourceAddress  string `json:"source_address"`
		SourceMemo     string `json:"source_memo"`
		SourceSequence string `json:"source_sequence"`
		SourceOutput   string `json:"source_output"`
		Mode           string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}
	if req.ChainID == "" || req.Token == "" || req.From == "" || req.Amount == "" {
		http.Error(w, `{"error":"missing fields"}`, http.StatusBadRequest)
		return
	}
	if req.ToAddr == "" {
		req.ToAddr = req.ToBsc
	}
	if req.ToAddr == "" {
		http.Error(w, `{"error":"missing destination address"}`, http.StatusBadRequest)
		return
	}
	amt, err := blockchaincomponent.NewAmountFromString(req.Amount)
	if err != nil || amt.Sign() <= 0 {
		http.Error(w, `{"error":"invalid amount"}`, http.StatusBadRequest)
		return
	}
	family := strings.ToLower(strings.TrimSpace(req.Family))
	if family == "" {
		if cfg := bridgeLookupChain(req.ChainID); cfg != nil {
			family = strings.ToLower(strings.TrimSpace(cfg.Family))
		}
	}
	reqBridge := &blockchaincomponent.BridgeRequest{
		SourceTxHash:   req.SourceTxHash,
		SourceAddress:  req.SourceAddress,
		SourceMemo:     req.SourceMemo,
		SourceSequence: req.SourceSequence,
		SourceOutput:   req.SourceOutput,
	}
	if err := blockchaincomponent.ValidateBridgeRequestMetadata(family, reqBridge); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}
	chainID := strings.TrimSpace(req.ChainID)
	if chainID == "" {
		chainID = "bsc-testnet"
	}
	if strings.EqualFold(req.Mode, "private") {
		bcs.BlockchainPtr.AddPrivateBridgeRequestBurnToChainWithMetadata(chainID, "generic-lqd-burn", req.Token, req.From, req.ToAddr, amt, req.SourceAddress, req.SourceMemo, req.SourceSequence, req.SourceOutput)
	} else {
		bcs.BlockchainPtr.AddBridgeRequestBurnToChainWithMetadata(chainID, "generic-lqd-burn", req.Token, req.From, req.ToAddr, amt, req.SourceAddress, req.SourceMemo, req.SourceSequence, req.SourceOutput)
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "burned"})
}

func (bcs *BlockchainServer) BridgePrivateLockBsc(w http.ResponseWriter, r *http.Request) {
	bcs.BridgeLockBsc(w, r)
}

func (bcs *BlockchainServer) BridgePrivateBurnLqd(w http.ResponseWriter, r *http.Request) {
	bcs.BridgeBurnLqd(w, r)
}

func (bcs *BlockchainServer) Faucet(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	setCORSHeaders(w, r)
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
	setCORSHeaders(w, r)
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
	setCORSHeaders(w, r)
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	stats := bcs.BlockchainPtr.GetNetworkStats()
	json.NewEncoder(w).Encode(stats)
}

func (bcs *BlockchainServer) GetPeers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	setCORSHeaders(w, r)
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
	setCORSHeaders(w, r)
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
	setCORSHeaders(w, r)
	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		// Fallback for default ServeMux (no gorilla mux vars)
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) > 0 {
			id = parts[len(parts)-1]
		}
	}

	if blockNumber, err := strconv.ParseUint(id, 10, 64); err == nil {
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
		return
	}

	blockHash := strings.ToLower(strings.TrimSpace(id))

	b.BlockchainPtr.Mutex.Lock()
	defer b.BlockchainPtr.Mutex.Unlock()

	for _, blk := range b.BlockchainPtr.Blocks {
		if blk != nil && strings.ToLower(blk.CurrentHash) == blockHash {
			json.NewEncoder(w).Encode(blk)
			return
		}
	}

	block, err := blockchaincomponent.GetBlockByHashFromDB(blockHash)
	if err != nil || block == nil {
		http.Error(w, "Block not found", http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(block)
}

func (bcs *BlockchainServer) GetValidators(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	setCORSHeaders(w, r)

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
	setCORSHeaders(w, r)

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
	setCORSHeaders(w, r)

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Address string      `json:"address"`
		Amount  amountField `json:"amount"`
		Days    int         `json:"days"`
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
	setCORSHeaders(w, r)

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
	setCORSHeaders(w, r)

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
	setCORSHeaders(w, r)
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
	setCORSHeaders(w, r)

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
	setCORSHeaders(w, r)

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
	setCORSHeaders(w, r)
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
	setCORSHeaders(w, r)

	limit := parseLimit(r, 20)
	out := bcs.buildChainSummary(limit)
	json.NewEncoder(w).Encode(out)
}

func (bcs *BlockchainServer) GlobalChainSummary(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	setCORSHeaders(w, r)
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
	setCORSHeaders(w, r)
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
	setCORSHeaders(w, r)

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
	setCORSHeaders(w, r)

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
	setCORSHeaders(w, r)
	address := r.URL.Query().Get("address")
	ev := b.BlockchainPtr.ContractEngine.EventDB.GetEventsFromDB(address)
	json.NewEncoder(w).Encode(ev)
}

func (bcs *BlockchainServer) GetAddressTransactions(w http.ResponseWriter, r *http.Request) {
	// CORS headers
	w.Header().Set("Content-Type", "application/json")
	setCORSHeaders(w, r)

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
	setCORSHeaders(w, r)
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
	setCORSHeaders(w, r)

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "invalid method", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Address  string      `json:"address"`
		Amount   amountField `json:"amount"`
		LockDays int64       `json:"lock_days"`
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
	setCORSHeaders(w, r)

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
	setCORSHeaders(w, r)
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
	setCORSHeaders(w, r)

	out := []*blockchaincomponent.LiquidityProvider{}
	for _, lp := range bcs.BlockchainPtr.LiquidityProviders {
		out = append(out, lp)
	}

	json.NewEncoder(w).Encode(out)
}

func (bcs *BlockchainServer) GetPoolLiquidity(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	setCORSHeaders(w, r)
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
	setCORSHeaders(w, r)
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
	setCORSHeaders(w, r)
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
	setCORSHeaders(w, r)
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
	setCORSHeaders(w, r)

	address := r.URL.Query().Get("address")

	data := b.BlockchainPtr.ContractEngine.Registry.Load(address)
	json.NewEncoder(w).Encode(data)
}

func (bcs *BlockchainServer) ContractDeploy(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	setCORSHeaders(w, r)
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
		fingerprint, err := blockchaincomponent.CurrentPluginRuntimeFingerprint()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		meta.RuntimeFingerprint = fingerprint
		if req.PluginPath != "" {
			meta.PluginPath = req.PluginPath
			break
		}
		if len(codeBytes) == 0 {
			http.Error(w, "plugin file required", 400)
			return
		}
		pluginDir := blockchaincomponent.ContractArtifactsDir()
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
		GasPrice:   0,
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
	deployTx.Status = constantset.StatusSuccess
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
	setCORSHeaders(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
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
	setCORSHeaders(w, r)
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
	setCORSHeaders(w, r)
	addr := r.URL.Query().Get("address")

	abi, err := bcs.BlockchainPtr.ContractEngine.Registry.LoadABI(addr)
	if err != nil {
		http.Error(w, err.Error(), 404)
		return
	}

	// Deduplicate: remove lowercase duplicates (LoadPlugin stores each method twice —
	// once with original name and once lowercased for case-insensitive lookup).
	// Only return exported (capital-first) names in the ABI response.
	var parsed struct {
		Entries []blockchaincomponent.ABIEntry `json:"entries"`
	}
	if json.Unmarshal(abi, &parsed) == nil {
		var deduped []blockchaincomponent.ABIEntry
		for _, e := range parsed.Entries {
			if len(e.Name) > 0 && e.Name[0] >= 'A' && e.Name[0] <= 'Z' {
				deduped = append(deduped, e)
			}
		}
		parsed.Entries = deduped
		if out, err2 := json.Marshal(parsed); err2 == nil {
			w.Write(out)
			return
		}
	}
	w.Write(abi)
}

func (b *BlockchainServer) ContractList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	setCORSHeaders(w, r)
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

func (b *BlockchainServer) CurrentDEXFactory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	setCORSHeaders(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	addrs := b.BlockchainPtr.ContractEngine.Registry.List()
	type candidate struct {
		addr      string
		timestamp int64
	}
	best := candidate{}

	for _, addr := range addrs {
		rec, err := b.BlockchainPtr.ContractEngine.Registry.LoadContract(addr)
		if err != nil || rec == nil || rec.Metadata == nil {
			continue
		}
		meta := rec.Metadata
		isFactory := strings.EqualFold(meta.BuiltinName, "dex_factory")
		if !isFactory && len(meta.ABI) > 0 {
			var entries []map[string]any
			if err := json.Unmarshal(meta.ABI, &entries); err == nil {
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
				isFactory = has("CreatePair") || has("GetPair") || has("AllPairs")
			}
		}
		if !isFactory {
			continue
		}
		if meta.Timestamp >= best.timestamp {
			best = candidate{addr: addr, timestamp: meta.Timestamp}
		}
	}

	json.NewEncoder(w).Encode(map[string]any{
		"address": best.addr,
		"kind":    "factory",
	})
}

func (b *BlockchainServer) BaseFee(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	setCORSHeaders(w, r)
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
	setCORSHeaders(w, r)
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
	setCORSHeaders(w, r)

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

// CompileGoPlugin — compile Go source into a .so plugin on-the-fly and return base64 bytes
func (b *BlockchainServer) CompileGoPlugin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	setCORSHeaders(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	var req struct {
		Source string `json:"source"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, 400)
		return
	}
	if strings.TrimSpace(req.Source) == "" {
		http.Error(w, `{"error":"source code is empty"}`, 400)
		return
	}

	// Compile INSIDE project root to avoid spaces-in-path issues with go.mod replace.
	// We create a unique subdirectory _plugin_builds/<id>/ relative to project root.
	projectRoot := findProjectRoot()
	buildsDir := filepath.Join(projectRoot, "_plugin_builds")
	_ = os.MkdirAll(buildsDir, 0755)

	tmpDir, err := os.MkdirTemp(buildsDir, "build_*")
	if err != nil {
		http.Error(w, `{"error":"failed to create build dir"}`, 500)
		return
	}
	defer os.RemoveAll(tmpDir)

	// Write contract source
	srcPath := filepath.Join(tmpDir, "contract.go")
	if err := os.WriteFile(srcPath, []byte(req.Source), 0644); err != nil {
		http.Error(w, `{"error":"failed to write source file"}`, 500)
		return
	}

	// Build from WITHIN the root module (no separate go.mod) — ensures all
	// shared packages compile with identical build IDs to the host binary.
	// The user-supplied source lands in a unique sub-dir of _plugin_builds/;
	// since there is no go.mod there, Go inherits the root module automatically.

	// Run: go build -buildmode=plugin -o contract.so <relPkg>
	relPkg := "./" + strings.TrimPrefix(filepath.ToSlash(tmpDir), filepath.ToSlash(projectRoot)+"/")
	outPath := filepath.Join(tmpDir, "contract.so")
	cmd := exec.Command("go", "build", "-buildmode=plugin", "-o", outPath, relPkg)
	cmd.Dir = projectRoot
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1")
	cmdOutput, err := cmd.CombinedOutput()
	if err != nil {
		// Return compiler error as JSON so frontend can display it
		json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   string(cmdOutput),
		})
		return
	}

	// Read compiled .so
	soBytes, err := os.ReadFile(outPath)
	if err != nil {
		http.Error(w, `{"error":"failed to read compiled plugin"}`, 500)
		return
	}

	// Return base64-encoded binary
	encoded := base64.StdEncoding.EncodeToString(soBytes)
	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"binary":  encoded,
		"size":    len(soBytes),
	})
}

// DeployBuiltin compiles and deploys a named builtin contract template.
// POST /contract/deploy-builtin
// Body (JSON): { "template": "lqd20"|"dex_swap"|..., "owner": "0x...", "private_key": "...", "gas": 500000 }
func (b *BlockchainServer) DeployBuiltin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	setCORSHeaders(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	// Catch any panics (e.g., from plugin.Open ABI mismatch) and return them as JSON errors
	defer func() {
		if rec := recover(); rec != nil {
			log.Printf("deploy-builtin panic: %v", rec)
			http.Error(w, fmt.Sprintf(`{"error":"internal panic: %v"}`, rec), 500)
		}
	}()

	var req struct {
		Template   string   `json:"template"`
		Owner      string   `json:"owner"`
		PrivateKey string   `json:"private_key"`
		Gas        uint64   `json:"gas"`
		InitArgs   []string `json:"init_args"` // optional: name, symbol, supply etc.
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, 400)
		return
	}

	// Validate template name (whitelist)
	validTemplates := map[string]bool{
		"lqd20": true, "wlqd": true, "dex_swap": true, "dex_factory": true, "dex_pair": true,
		"bridge_token": true, "lending_pool": true, "nft_collection": true, "dao_treasury": true,
	}
	if !validTemplates[req.Template] {
		http.Error(w, `{"error":"unknown template"}`, 400)
		return
	}
	if req.Owner == "" || !wallet.ValidateAddress(req.Owner) {
		http.Error(w, `{"error":"invalid owner address"}`, 400)
		return
	}
	if req.Gas == 0 {
		req.Gas = 500000
	}

	projectRoot := findProjectRoot()
	soBytes, err := loadBuiltinPluginBytes(projectRoot, req.Template)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
		return
	}
	// Deploy ─────────────────────────────────────────────────────────────────
	addr := GenerateContractAddress(req.Owner, uint64(time.Now().UnixNano()))
	meta := &blockchaincomponent.ContractMetadata{
		Address:     addr,
		Type:        "plugin",
		Owner:       req.Owner,
		Timestamp:   time.Now().Unix(),
		BuiltinName: req.Template,
	}
	fingerprint, err := blockchaincomponent.CurrentPluginRuntimeFingerprint()
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"runtime fingerprint: %v"}`, err), 500)
		return
	}
	meta.RuntimeFingerprint = fingerprint
	pluginDir := blockchaincomponent.ContractArtifactsDir()
	_ = os.MkdirAll(pluginDir, 0o755)
	pluginPath := filepath.Join(pluginDir, addr+".so")
	if err := os.WriteFile(pluginPath, soBytes, 0o755); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	meta.PluginPath = pluginPath

	// Nil-safety checks before touching blockchain internals
	if b.BlockchainPtr == nil {
		http.Error(w, `{"error":"blockchain not initialized"}`, 500)
		return
	}
	if b.BlockchainPtr.ContractEngine == nil {
		http.Error(w, `{"error":"contract engine not initialized"}`, 500)
		return
	}
	if b.BlockchainPtr.ContractEngine.Registry == nil {
		http.Error(w, `{"error":"contract registry not initialized"}`, 500)
		return
	}
	if b.BlockchainPtr.ContractEngine.Registry.PluginVM == nil {
		http.Error(w, `{"error":"plugin VM not initialized"}`, 500)
		return
	}

	if err := b.BlockchainPtr.ContractEngine.Registry.PluginVM.LoadPlugin(addr, meta.PluginPath); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"plugin load: %v"}`, err), 500)
		return
	}
	pc := b.BlockchainPtr.ContractEngine.Registry.PluginVM.GetPlugin(addr)
	if pc == nil {
		http.Error(w, `{"error":"plugin instance nil after load"}`, 500)
		return
	}
	abi, err := blockchaincomponent.GenerateABIForPlugin(pc)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"ABI gen: %v"}`, err), 500)
		return
	}
	meta.ABI = abi
	meta.Pool, meta.PoolType = detectPoolFromABI(abi)

	state := &blockchaincomponent.SmartContractState{
		Address:   addr,
		Balance:   "0",
		Storage:   map[string]string{},
		IsActive:  true,
		CreatedAt: time.Now().Unix(),
	}
	if err := b.BlockchainPtr.ContractEngine.Registry.RegisterContract(meta, state); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"register: %v"}`, err), 500)
		return
	}
	if meta.Pool {
		b.BlockchainPtr.RegisterPool(addr)
	}

	// For dex_factory: auto-compile dex_pair template and call factory.Init(pairPluginPath)
	if req.Template == "dex_factory" {
		pairPluginPath, pairCompileErr := b.compilePairTemplate(projectRoot)
		if pairCompileErr != nil {
			log.Printf("deploy-builtin: dex_pair compile failed (non-fatal): %v", pairCompileErr)
		} else {
			if _, err := b.BlockchainPtr.ContractEngine.Pipeline.ApplyContractCall(
				addr, req.Owner, "Init", []string{pairPluginPath},
			); err != nil {
				log.Printf("deploy-builtin: factory Init() failed (non-fatal): %v", err)
			}
		}
	}

	// Call Init() with user-supplied args (e.g. name, symbol, supply).
	// Filter out empty strings so contracts with no init args skip gracefully.
	// (skip for dex_factory — already handled above)
	if req.Template != "dex_factory" {
		initArgs := []string{}
		for _, a := range req.InitArgs {
			if strings.TrimSpace(a) != "" {
				initArgs = append(initArgs, a)
			}
		}
		if len(initArgs) > 0 {
			if _, err := b.BlockchainPtr.ContractEngine.Pipeline.ApplyContractCall(
				addr, req.Owner, "Init", initArgs,
			); err != nil {
				log.Printf("deploy-builtin: Init() call failed (non-fatal): %v", err)
			}
		}
	}

	// Create and submit a contract_create transaction (same as regular deploy)
	fnPayload, _ := json.Marshal(map[string]any{"fn": "constructor", "args": []string{}})
	deployTx := &blockchaincomponent.Transaction{
		From:       req.Owner,
		To:         addr,
		Type:       "contract_create",
		Function:   "constructor",
		Args:       []string{},
		Data:       fnPayload,
		Timestamp:  uint64(time.Now().Unix()),
		Status:     constantset.StatusPending,
		IsSystem:   false,
		ChainID:    uint64(constantset.ChainID),
		Gas:        req.Gas,
		GasPrice:   b.BlockchainPtr.CalculateBaseFee() + 1,
		IsContract: true,
	}
	if req.PrivateKey != "" {
		signer, err := wallet.ImportFromPrivateKey(req.PrivateKey)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"invalid private key: %v"}`, err), 400)
			return
		}
		if !strings.EqualFold(signer.Address, req.Owner) {
			http.Error(w, `{"error":"owner does not match private key"}`, 400)
			return
		}
		_ = signer.SignTransaction(deployTx)
	}
	deployTx.TxHash = blockchaincomponent.CalculateTransactionHash(*deployTx)
	b.BlockchainPtr.AddNewTxToTheTransaction_pool(deployTx)
	deployTx.Status = constantset.StatusSuccess
	b.BlockchainPtr.RecordRecentTx(deployTx)

	json.NewEncoder(w).Encode(map[string]any{
		"status":   "deployed",
		"address":  addr,
		"type":     "plugin",
		"template": req.Template,
		"tx_hash":  deployTx.TxHash,
	})
}

func prebuiltBuiltinPluginPath(projectRoot, template string) string {
	return filepath.Join(projectRoot, "bin", "builtins", template+".so")
}

func loadBuiltinPluginBytes(projectRoot, template string) ([]byte, error) {
	prebuiltPath := prebuiltBuiltinPluginPath(projectRoot, template)
	if soBytes, err := os.ReadFile(prebuiltPath); err == nil && len(soBytes) > 0 {
		return soBytes, nil
	}
	return compileBuiltinTemplate(projectRoot, template)
}

func compileBuiltinTemplate(projectRoot, template string) ([]byte, error) {
	srcFile := filepath.Join(projectRoot, "contract", template+".go")
	srcBytes, err := os.ReadFile(srcFile)
	if err != nil {
		return nil, fmt.Errorf("builtin source not found: %s", template)
	}
	source := string(srcBytes)
	source = strings.Replace(source, "//go:build ignore\n", "", 1)
	source = strings.Replace(source, "// +build ignore\n", "", 1)

	buildsDir := filepath.Join(projectRoot, "_plugin_builds")
	if err := os.MkdirAll(buildsDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create build dir: %v", err)
	}
	tmpDir, err := os.MkdirTemp(buildsDir, "builtin_*")
	if err != nil {
		return nil, fmt.Errorf("failed to create build dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := os.WriteFile(filepath.Join(tmpDir, "contract.go"), []byte(source), 0o644); err != nil {
		return nil, fmt.Errorf("failed to write source: %v", err)
	}

	relPkg := "./" + strings.TrimPrefix(filepath.ToSlash(tmpDir), filepath.ToSlash(projectRoot)+"/")
	outPath := filepath.Join(tmpDir, "contract.so")
	cmd := exec.Command("go", "build", "-buildmode=plugin", "-o", outPath, relPkg)
	cmd.Dir = projectRoot
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("compile failed: %s", msg)
	}
	soBytes, err := os.ReadFile(outPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read plugin: %v", err)
	}
	return soBytes, nil
}

// compilePairTemplate compiles the dex_pair template plugin and returns the .so path.
// Called when deploying a dex_factory so the factory knows where to find pair plugin code.
func (b *BlockchainServer) compilePairTemplate(projectRoot string) (string, error) {
	prebuiltPath := prebuiltBuiltinPluginPath(projectRoot, "dex_pair")
	if _, err := os.Stat(prebuiltPath); err == nil {
		return prebuiltPath, nil
	}
	// Strategy: build the pair plugin from WITHIN the root module so that all
	// shared packages (ConstantSet, BlockchainComponent, …) compile with
	// IDENTICAL build IDs as the host binary.
	//
	// dex_pair.go has `//go:build ignore` to keep it out of the normal build;
	// we strip that tag and drop the file into a sub-directory of the root
	// module that has NO go.mod of its own — inheriting root go.mod.
	// `go build -buildmode=plugin` then compiles it with the same dependency
	// graph as the host binary → compatible package hashes → no ABI mismatch.

	srcFile := filepath.Join(projectRoot, "contract", "dex_pair.go")
	srcBytes, err := os.ReadFile(srcFile)
	if err != nil {
		return "", fmt.Errorf("dex_pair source not found: %w", err)
	}
	source := string(srcBytes)
	// Strip build-constraint tags that would cause `go build` to skip the file
	source = strings.Replace(source, "//go:build ignore\n", "", 1)
	source = strings.Replace(source, "// +build ignore\n", "", 1)

	// A stable sub-directory inside the root module (no go.mod → uses root)
	activeDir := filepath.Join(projectRoot, "_plugin_builds", "pair_active")
	if err := os.MkdirAll(activeDir, 0755); err != nil {
		return "", fmt.Errorf("mkdir pair_active: %w", err)
	}
	defer os.RemoveAll(activeDir)

	if err := os.WriteFile(filepath.Join(activeDir, "dex_pair.go"), []byte(source), 0644); err != nil {
		return "", err
	}

	soPath := filepath.Join(projectRoot, "data", "pair_template.so")
	cmd := exec.Command("go", "build", "-buildmode=plugin", "-o", soPath, "./_plugin_builds/pair_active")
	cmd.Dir = projectRoot
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1")
	out, errRun := cmd.CombinedOutput()
	if errRun != nil {
		return "", fmt.Errorf("compile pair plugin: %s: %w", string(out), errRun)
	}
	return soPath, nil
}

// findProjectRoot walks up from the current executable's directory
// looking for a go.mod file — returns that directory (= project root).
func findProjectRoot() string {
	// Try executable path first
	exe, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(exe)
		for i := 0; i < 6; i++ {
			if _, e := os.Stat(filepath.Join(dir, "go.mod")); e == nil {
				return dir
			}
			dir = filepath.Dir(dir)
		}
	}
	// Fallback: current working directory walk
	dir, _ := os.Getwd()
	for i := 0; i < 6; i++ {
		if _, e := os.Stat(filepath.Join(dir, "go.mod")); e == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return dir
}

// ── Rate Limiter ─────────────────────────────────────────────────────────────
type rateLimiter struct {
	mu      sync.Mutex
	clients map[string]*clientLimit
	rate    int // max requests per second
	burst   int
}

type clientLimit struct {
	tokens   float64
	lastTime time.Time
}

func newRateLimiter(rate, burst int) *rateLimiter {
	rl := &rateLimiter{
		clients: make(map[string]*clientLimit),
		rate:    rate,
		burst:   burst,
	}
	// Clean up old clients every 5 minutes
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			rl.mu.Lock()
			for ip, cl := range rl.clients {
				if time.Since(cl.lastTime) > 10*time.Minute {
					delete(rl.clients, ip)
				}
			}
			rl.mu.Unlock()
		}
	}()
	return rl
}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	cl, ok := rl.clients[ip]
	if !ok {
		cl = &clientLimit{tokens: float64(rl.burst), lastTime: now}
		rl.clients[ip] = cl
	}
	elapsed := now.Sub(cl.lastTime).Seconds()
	cl.lastTime = now
	cl.tokens += elapsed * float64(rl.rate)
	if cl.tokens > float64(rl.burst) {
		cl.tokens = float64(rl.burst)
	}
	if cl.tokens < 1 {
		return false
	}
	cl.tokens--
	return true
}

func (rl *rateLimiter) middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		if idx := strings.LastIndex(ip, ":"); idx != -1 {
			ip = ip[:idx]
		}
		if !rl.allow(ip) {
			w.Header().Set("Retry-After", "1")
			http.Error(w, `{"error":"rate limit exceeded, slow down"}`, http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}

// ── Health & Mempool ─────────────────────────────────────────────────────────

// HealthCheck returns a JSON summary of node liveness.  Peers call this
// endpoint periodically to decide whether a node should be kept in their peer
// table.
func (b *BlockchainServer) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	setCORSHeaders(w, r)
	b.BlockchainPtr.Mutex.Lock()
	height := len(b.BlockchainPtr.Blocks)
	peers := len(b.BlockchainPtr.Network.Peers)
	b.BlockchainPtr.Mutex.Unlock()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "ok",
		"height":    height,
		"peers":     peers,
		"version":   "1.0.0",
		"timestamp": time.Now().Unix(),
	})
}

// GetMempool returns all pending (unconfirmed) transactions so other nodes
// can pull the mempool on start-up or after reconnecting.
func (b *BlockchainServer) GetMempool(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	setCORSHeaders(w, r)
	b.BlockchainPtr.Mutex.Lock()
	txs := b.BlockchainPtr.Transaction_pool
	b.BlockchainPtr.Mutex.Unlock()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"transactions": txs,
		"count":        len(txs),
	})
}

// ── CORS ──────────────────────────────────────────────────────────────────────
func configuredAllowedOrigins() []string {
	origins := []string{
		"http://localhost:3000",
		"http://localhost:3001",
		"http://localhost:4173",
		"http://127.0.0.1:3000",
		"http://127.0.0.1:3001",
		"http://127.0.0.1:4173",
		"chrome-extension://",
	}
	for _, raw := range strings.Split(os.Getenv("LQD_ALLOWED_ORIGINS"), ",") {
		origin := strings.TrimSpace(raw)
		if origin == "" {
			continue
		}
		origins = append(origins, origin)
	}
	return origins
}

func setCORSHeaders(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	allowed := false
	for _, o := range configuredAllowedOrigins() {
		if strings.HasPrefix(origin, o) {
			allowed = true
			break
		}
	}
	if allowed {
		w.Header().Set("Access-Control-Allow-Origin", origin)
	} else if origin == "" {
		// Direct API call (no browser) - allow
		w.Header().Set("Access-Control-Allow-Origin", "*")
	}
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
	w.Header().Set("Access-Control-Max-Age", "86400")
}

// ── Input Validation ──────────────────────────────────────────────────────────

// ValidateAddress checks that an address is a valid 0x-prefixed 20-byte hex string.
func ValidateAddress(addr string) bool {
	if !strings.HasPrefix(addr, "0x") || len(addr) != 42 {
		return false
	}
	// Check all chars are valid hex
	for _, c := range addr[2:] {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func validatePositiveAmount(amtStr string) bool {
	if amtStr == "" || amtStr == "0" {
		return false
	}
	n := new(big.Int)
	if _, ok := n.SetString(amtStr, 10); !ok {
		return false
	}
	return n.Sign() > 0
}

// ── Request Size Limit ────────────────────────────────────────────────────────
func maxBytesMiddleware(next http.HandlerFunc, maxBytes int64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
		next(w, r)
	}
}

func (b *BlockchainServer) Start() {
	portStr := fmt.Sprintf("%d", b.Port)

	const maxBodySize = 10 * 1024 * 1024    // 10 MB  — general limit
	const deployBodySize = 60 * 1024 * 1024 // 60 MB  — Go plugin .so files can be ~20 MB

	http.HandleFunc("/", b.getBlockchain)
	http.HandleFunc("/chain/export", b.GetFullBlockchain)
	http.HandleFunc("/balance", b.GetBalance)
	http.HandleFunc("/send_tx", b.limiter.middleware(maxBytesMiddleware(b.sendTransaction, maxBodySize)))
	http.HandleFunc("/send_tx/batch", b.limiter.middleware(maxBytesMiddleware(b.sendTransactionBatch, maxBodySize)))
	http.HandleFunc("/fetch_last_n_block", b.fetchNBlocks)
	http.HandleFunc("/account/{address}/nonce", b.GetAccountNonce)
	http.HandleFunc("/getheight", b.GetBlockchainHeight)
	http.HandleFunc("/validator/{address}", b.ValidatorStats)
	http.HandleFunc("/network", b.NetworkStats)
	http.HandleFunc("/peers", b.GetPeers)
	http.HandleFunc("/peers/add", b.AddPeer)
	http.HandleFunc("/health", b.HealthCheck)
	http.HandleFunc("/mempool", b.GetMempool)
	http.HandleFunc("/faucet", b.limiter.middleware(b.Faucet))
	http.HandleFunc("/block/{id}", b.GetBlock)
	http.HandleFunc("/validators", b.GetValidators)
	http.HandleFunc("/transactions/recent", b.GetRecentTransactions)
	http.HandleFunc("/liquidity/lock", b.limiter.middleware(maxBytesMiddleware(b.LiquidityLock, maxBodySize)))
	http.HandleFunc("/liquidity/unlock", b.limiter.middleware(maxBytesMiddleware(b.LiquidityUnlock, maxBodySize)))
	http.HandleFunc("/liquidity", b.LiquidityView)
	http.HandleFunc("/rewards/recent", b.RewardsRecent)
	http.HandleFunc("/rewards/latest", b.RewardsLatest)
	http.HandleFunc("/validators/active", b.ActiveValidatorsLatest)
	http.HandleFunc("/validators/sync", b.SyncValidatorsAll)
	http.HandleFunc("/chain/summary", b.ChainSummary)
	http.HandleFunc("/chain/global", b.GlobalChainSummary)
	http.HandleFunc("/rpc", b.limiter.middleware(maxBytesMiddleware(b.JSONRPC, maxBodySize)))
	http.HandleFunc("/validator/new", b.AddValidatorFromPeer)
	http.HandleFunc("/validator/add", b.AddValidator)
	http.HandleFunc("/tx/", b.GetTransactionByHash)
	http.HandleFunc("/contract/deploy", b.limiter.middleware(maxBytesMiddleware(b.ContractDeploy, deployBodySize)))
	http.HandleFunc("/contract/call", b.limiter.middleware(maxBytesMiddleware(b.ContractCall, maxBodySize)))
	http.HandleFunc("/contract/getAbi", b.ContractABI)
	http.HandleFunc("/contract/con1", b.ContractState)
	http.HandleFunc("/contract/con2", b.ContractEvents)
	http.HandleFunc("/contract/list", b.ContractList)
	http.HandleFunc("/dex/current", b.CurrentDEXFactory)
	http.HandleFunc("/contract/compile", b.limiter.middleware(maxBytesMiddleware(b.CompileContract, maxBodySize)))
	http.HandleFunc("/contract/compile-plugin", b.limiter.middleware(maxBytesMiddleware(b.CompileGoPlugin, maxBodySize)))
	http.HandleFunc("/contract/deploy-builtin", b.limiter.middleware(maxBytesMiddleware(b.DeployBuiltin, maxBodySize)))
	http.HandleFunc("/contract/storage", b.GetContractStorage)
	http.HandleFunc("/contract/code", b.GetContractCode)
	http.HandleFunc("/contract/events", b.GetContractEvents) // address-only, no block required
	http.HandleFunc("/basefee", b.BaseFee)
	http.HandleFunc("/blocktime/latest", b.BlockTimeLatest)
	http.HandleFunc("/address/{address}/transactions", b.GetAddressTransactions)
	http.HandleFunc("/liquidity/provide", b.limiter.middleware(maxBytesMiddleware(b.ProvideLiquidity, maxBodySize)))
	http.HandleFunc("/liquidity/unstake", b.limiter.middleware(maxBytesMiddleware(b.UnstakeLiquidity, maxBodySize)))
	http.HandleFunc("/liquidity/info", b.GetLiquidityInfo)
	http.HandleFunc("/liquidity/all", b.GetAllLiquidityProviders)
	http.HandleFunc("/liquidity/pools", b.GetPoolLiquidity)
	http.HandleFunc("/liquidity/rebalance", b.limiter.middleware(b.RebalancePools))
	http.HandleFunc("/liquidity/pools/sync", b.SyncPools)
	http.HandleFunc("/liquidity/pools/register", b.limiter.middleware(maxBytesMiddleware(b.RegisterPoolManual, maxBodySize)))
	http.HandleFunc("/bridge/requests", b.GetBridgeRequests)
	http.HandleFunc("/bridge/families", b.GetBridgeFamilies)
	http.HandleFunc("/bridge/tokens", b.GetBridgeTokens)
	http.HandleFunc("/bridge/token", b.limiter.middleware(maxBytesMiddleware(b.UpsertBridgeToken, maxBodySize)))
	http.HandleFunc("/bridge/token/remove", b.limiter.middleware(maxBytesMiddleware(b.RemoveBridgeToken, maxBodySize)))
	http.HandleFunc("/bridge/chains", b.GetBridgeChains)
	http.HandleFunc("/bridge/chain", b.UpsertBridgeChain)
	http.HandleFunc("/bridge/chain/remove", b.RemoveBridgeChain)
	http.HandleFunc("/bridge/lock_bsc", b.limiter.middleware(maxBytesMiddleware(b.BridgeLockBsc, maxBodySize)))
	http.HandleFunc("/bridge/burn_lqd", b.limiter.middleware(maxBytesMiddleware(b.BridgeBurnLqd, maxBodySize)))
	http.HandleFunc("/bridge/private_lock_bsc", b.limiter.middleware(maxBytesMiddleware(b.BridgePrivateLockBsc, maxBodySize)))
	http.HandleFunc("/bridge/private_burn_lqd", b.limiter.middleware(maxBytesMiddleware(b.BridgePrivateBurnLqd, maxBodySize)))
	http.HandleFunc("/bridge/lock_chain", b.limiter.middleware(maxBytesMiddleware(b.BridgeLockChain, maxBodySize)))
	http.HandleFunc("/bridge/burn_chain", b.limiter.middleware(maxBytesMiddleware(b.BridgeBurnChain, maxBodySize)))

	//http.HandleFunc("/contract/compile", b.CompileContract)

	// http.HandleFunc("/contract/compile", b.ContractCompile)
	// http.HandleFunc("/contract/events", b.GetContractEvents)
	// http.HandleFunc("/contract/storage", b.GetContractStorage)
	// http.HandleFunc("/contract/code", b.GetContractCode)
	// http.HandleFunc("/contract/streamEvents", b.StreamContractEvents)

	//http.HandleFunc("/token-balance", b.GetTokenBalance)

	log.Println("Blockchain server is starting on port:", b.Port)

	// Use the CORS handler
	// Bind on all interfaces so mobile devices on the same LAN can reach it.
	err := http.ListenAndServe(":"+portStr, nil)
	if err != nil {
		log.Fatalf("Failed to start blockchain server: %v", err)
	}
	log.Println("Blockchain server started successfully")
}
