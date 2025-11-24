// package blockchainserver

// import (
// 	"encoding/hex"
// 	"encoding/json"
// 	"fmt"
// 	"io"
// 	"log"
// 	"net/http"
// 	"sort"
// 	"strconv"
// 	"strings"
// 	"sync"
// 	"time"

// 	blockchaincomponent "github.com/Zotish/DefenceProject/BlockchainComponent"
// 	constantset "github.com/Zotish/DefenceProject/ConstantSet"
// 	wallet "github.com/Zotish/DefenceProject/WalletComponent"
// 	"github.com/gorilla/mux"
// 	//"github.com/rs/cors"
// )

// type BlockchainServer struct {
// 	Port          uint                                   `json:"port"`
// 	BlockchainPtr *blockchaincomponent.Blockchain_struct `json:"blockchain_ptr"`
// }
// type TxEnvelope struct {
// 	Transaction *blockchaincomponent.Transaction `json:"transaction"`
// 	Source      string                           `json:"source"`                 // "mempool" or "block"
// 	BlockHash   string                           `json:"block_hash,omitempty"`   // if confirmed
// 	BlockNumber uint64                           `json:"block_number,omitempty"` // if confirmed
// 	TxIndex     int                              `json:"tx_index,omitempty"`     // if confirmed
// }

// func NewBlockchainServer(port uint, blockchainPtr *blockchaincomponent.Blockchain_struct) *BlockchainServer {
// 	return &BlockchainServer{
// 		Port:          port,
// 		BlockchainPtr: blockchainPtr,
// 	}
// }

// func (b *BlockchainServer) getBlockchain(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
// 	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
// 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
// 	if r.Method == http.MethodGet {
// 		io.WriteString(w, b.BlockchainPtr.ToJsonChain())
// 	} else {
// 		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// 		return
// 	}
// }

// func (bcs *BlockchainServer) GetAccountNonce(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
// 	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
// 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

// 	address := mux.Vars(r)["address"]

// 	bcs.BlockchainPtr.Mutex.Lock()
// 	defer bcs.BlockchainPtr.Mutex.Unlock()

// 	// confirmed (last used) nonce from chain state
// 	confirmed := bcs.BlockchainPtr.GetAccountNonce(address) // existing behavior:contentReference[oaicite:2]{index=2}

// 	// compute next free nonce including pending txs from this address
// 	next := confirmed + 1
// 	for _, tx := range bcs.BlockchainPtr.Transaction_pool {
// 		if tx.From == address && tx.Nonce >= next {
// 			next = tx.Nonce + 1
// 		}
// 	}

// 	// Backward-compatible: keep "nonce" (now meaning next usable)
// 	_ = json.NewEncoder(w).Encode(map[string]uint64{
// 		"confirmed_nonce": confirmed,
// 		"next_nonce":      next,
// 		"nonce":           next,
// 	})
// }

// // func (bcs *BlockchainServer) GetAccountNonce(w http.ResponseWriter, r *http.Request) {
// // 	w.Header().Set("Content-Type", "application/json")
// // 	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
// // 	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
// // 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
// // 	address := mux.Vars(r)["address"]
// // 	nonce := bcs.BlockchainPtr.GetAccountNonce(address)

// //		json.NewEncoder(w).Encode(map[string]uint64{
// //			"nonce": nonce,
// //		})
// //	}
// func (b *BlockchainServer) sendTransaction(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
// 	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
// 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
// 	if r.Method != http.MethodOptions {
// 		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// 		return
// 	}
// 	if r.Method == http.MethodPost {

// 		request, err := io.ReadAll(r.Body)
// 		if err != nil {
// 			http.Error(w, "Failed to read request body", http.StatusBadRequest)
// 			return
// 		}
// 		defer r.Body.Close()
// 		var tx blockchaincomponent.Transaction

// 		err = json.Unmarshal(request, &tx)
// 		if err != nil {
// 			http.Error(w, "Invalid transaction data", http.StatusBadRequest)
// 			return
// 		}

// 		go b.BlockchainPtr.AddNewTxToTheTransaction_pool(&tx)
// 		io.WriteString(w, tx.ToJsonTx())
// 	} else {
// 		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// 		return
// 	}
// }

// // func (b *BlockchainServer) fetchNBlocks(w http.ResponseWriter, r *http.Request) {
// // 	log.Printf("fetchNBlocks called - Method: %s, Origin: %s", r.Method, r.Header.Get("Origin"))

// // 	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
// // 	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
// // 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
// // 	w.Header().Set("Content-Type", "application/json")

// // 	// Handle preflight requests
// // 	if r.Method == "OPTIONS" {
// // 		log.Println("Handling OPTIONS preflight request")
// // 		w.WriteHeader(http.StatusOK)
// // 		return
// // 	}

// // 	if r.Method != http.MethodGet {
// // 		log.Printf("Method not allowed: %s", r.Method)
// // 		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// // 		return
// // 	}

// // 	b.BlockchainPtr.Mutex.Lock()
// // 	defer b.BlockchainPtr.Mutex.Unlock()

// // 	blocks := b.BlockchainPtr.Blocks
// // 	var blocksToReturn []*blockchaincomponent.Block
// // 	if len(blocks) < 10 {
// // 		blocksToReturn = blocks
// // 	} else {
// // 		blocksToReturn = blocks[len(blocks)-10:]
// // 	}

// // 	log.Printf("Returning %d blocks", len(blocksToReturn))
// // 	json.NewEncoder(w).Encode(blocksToReturn)
// // }

// func (b *BlockchainServer) fetchNBlocks(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
// 	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
// 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

// 	// Handle preflight requests
// 	if r.Method == "OPTIONS" {
// 		w.WriteHeader(http.StatusOK)
// 		return
// 	}

// 	b.BlockchainPtr.Mutex.Lock()
// 	defer b.BlockchainPtr.Mutex.Unlock()

// 	if r.Method == http.MethodGet {
// 		blocks := b.BlockchainPtr.Blocks
// 		var blocksToReturn []*blockchaincomponent.Block
// 		if len(blocks) < 10 {
// 			blocksToReturn = blocks
// 		} else {
// 			blocksToReturn = blocks[len(blocks)-10:]
// 		}

// 		json.NewEncoder(w).Encode(blocksToReturn) // Actually return the data
// 	} else {
// 		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// 	}

// }
// func (bcs *BlockchainServer) GetBlockchainHeight(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
// 	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
// 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
// 	height := uint64(len(bcs.BlockchainPtr.Blocks))
// 	json.NewEncoder(w).Encode(map[string]uint64{"height": height})
// }

// func (bcs *BlockchainServer) GetBalance(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
// 	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
// 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

// 	address := r.URL.Query().Get("address")

// 	bcs.BlockchainPtr.Mutex.Lock()
// 	defer bcs.BlockchainPtr.Mutex.Unlock()

// 	// Get confirmed balance from accounts
// 	confirmedBalance := bcs.BlockchainPtr.Accounts[address]

// 	// Calculate pending balance changes from transaction pool
// 	pendingBalanceChange := uint64(0)
// 	for _, tx := range bcs.BlockchainPtr.Transaction_pool {
// 		if tx.From == address && tx.Status == constantset.StatusPending {
// 			pendingBalanceChange -= uint64(tx.Value + (tx.GasPrice * tx.CalculateGasCost()))
// 		}
// 		if tx.To == address && tx.Status == constantset.StatusPending {
// 			pendingBalanceChange += uint64(tx.Value)
// 		}
// 	}

// 	totalBalance := uint64(max(0, int(uint64(confirmedBalance)+uint64(pendingBalanceChange))))

// 	json.NewEncoder(w).Encode(map[string]interface{}{
// 		"address":                address,
// 		"balance":                totalBalance,
// 		"confirmed_balance":      confirmedBalance,
// 		"pending_balance_change": pendingBalanceChange,
// 	})
// }

// //	func (b *BlockchainServer) fetchNBlocks(w http.ResponseWriter, r *http.Request) {
// //		w.Header().Set("Content-Type", "application/json")
// //		if r.Method == http.MethodGet {
// //			blocks := b.BlockchainPtr.Blocks
// //			blockchain1 := new(blockchaincomponent.Blockchain_struct)
// //			if len(blocks) < 10 {
// //				blockchain1.Blocks = blocks
// //			} else {
// //				blockchain1.Blocks = blocks[len(blocks)-10:]
// //			}
// //		} else {
// //			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// //			return
// //		}
// //	}
// // func (bcs *BlockchainServer) GetBalance(w http.ResponseWriter, r *http.Request) {
// // 	w.Header().Set("Content-Type", "application/json")
// // 	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
// // 	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
// // 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
// // 	address := r.URL.Query().Get("address")
// // 	balance := bcs.BlockchainPtr.Accounts[address]
// // 	json.NewEncoder(w).Encode(map[string]interface{}{"balance": balance})
// // }

// // blockchain_server.go
// //
// //	func (bcs *BlockchainServer) Faucet(w http.ResponseWriter, r *http.Request) {
// //		w.Header().Set("Content-Type", "application/json")
// //		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
// //		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
// //		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
// //		address := r.URL.Query().Get("address")
// //		if !wallet.ValidateAddress(address) {
// //			http.Error(w, "Invalid address", http.StatusBadRequest)
// //			return
// //		}
// //		tx := blockchaincomponent.NewTransaction("0xFaucet", address, 100000, []byte{}, bcs.BlockchainPtr.GetAccountNonce("0xFaucet"))
// //		bcs.BlockchainPtr.AddNewTxToTheTransaction_pool(tx)
// //	}
// // func (bcs *BlockchainServer) AddValidatorFromPeer(w http.ResponseWriter, r *http.Request) {
// // 	w.Header().Set("Content-Type", "application/json")
// // 	var v blockchaincomponent.Validator
// // 	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
// // 		http.Error(w, "invalid", http.StatusBadRequest)
// // 		return
// // 	}

// // 	// Check for duplicates
// // 	for _, existing := range bcs.BlockchainPtr.Validators {
// // 		if existing.Address == v.Address {
// // 			w.WriteHeader(http.StatusOK)
// // 			return
// // 		}
// // 	}

// // 	bcs.BlockchainPtr.Mutex.Lock()
// // 	bcs.BlockchainPtr.Validators = append(bcs.BlockchainPtr.Validators, &v)
// // 	bcs.BlockchainPtr.Mutex.Unlock()
// // 	log.Printf("✅ Added validator from peer: %s", v.Address)
// // 	w.WriteHeader(http.StatusOK)
// // }

// func (bcs *BlockchainServer) Faucet(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
// 	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
// 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
// 	if r.Method == "OPTIONS" {
// 		w.WriteHeader(http.StatusOK)
// 		return
// 	}
// 	if r.Method != http.MethodPost {
// 		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// 		return
// 	}

// 	// accept either JSON body or query param
// 	var payload struct {
// 		Address string `json:"address"`
// 	}
// 	_ = json.NewDecoder(r.Body).Decode(&payload)
// 	address := strings.TrimSpace(payload.Address)
// 	if address == "" {
// 		address = strings.TrimSpace(r.URL.Query().Get("address"))
// 	}
// 	if !wallet.ValidateAddress(address) {
// 		http.Error(w, "Invalid address", http.StatusBadRequest)
// 		return
// 	}

// 	// credit directly (test faucet)
// 	amount := uint64(100_000_00)
// 	bcs.BlockchainPtr.Mutex.Lock()
// 	bcs.BlockchainPtr.Accounts[address] += amount
// 	bcs.BlockchainPtr.Mutex.Unlock()

// 	json.NewEncoder(w).Encode(map[string]interface{}{"credited": amount, "address": address})
// }

// func (bcs *BlockchainServer) ValidatorStats(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
// 	w.Header().Set("Access-Control-Allow-Methods", "GET,POST, OPTIONS")
// 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
// 	address := mux.Vars(r)["address"]
// 	stats := bcs.BlockchainPtr.GetValidatorStats(address)
// 	if stats == nil {
// 		http.Error(w, "validator not found", http.StatusNotFound)
// 		return
// 	}
// 	json.NewEncoder(w).Encode(stats)
// }

// func (bcs *BlockchainServer) NetworkStats(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
// 	w.Header().Set("Access-Control-Allow-Methods", "GET,POST, OPTIONS")
// 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
// 	stats := bcs.BlockchainPtr.GetNetworkStats()
// 	json.NewEncoder(w).Encode(stats)
// }
// func (bcs *BlockchainServer) Metrics(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
// 	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
// 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
// 	fmt.Fprintf(w, "podl_blocks_total %d\n", len(bcs.BlockchainPtr.Blocks))
// 	fmt.Fprintf(w, "podl_validators_total %d\n", len(bcs.BlockchainPtr.Validators))
// 	fmt.Fprintf(w, "podl_slashing_pool %.2f\n", bcs.BlockchainPtr.SlashingPool)
// }

// // Add to blockchain_server.go
// func (b *BlockchainServer) GetBlock(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
// 	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
// 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
// 	vars := mux.Vars(r)
// 	blockNumber, err := strconv.ParseUint(vars["id"], 10, 64)
// 	if err != nil {
// 		http.Error(w, "Invalid block number", http.StatusBadRequest)
// 		return
// 	}

// 	b.BlockchainPtr.Mutex.Lock()
// 	defer b.BlockchainPtr.Mutex.Unlock()

// 	if blockNumber >= uint64(len(b.BlockchainPtr.Blocks)) {
// 		http.Error(w, "Block not found", http.StatusNotFound)
// 		return
// 	}

// 	block := b.BlockchainPtr.Blocks[blockNumber]
// 	json.NewEncoder(w).Encode(block)
// }

// // Add to blockchain_server.go
// // In blockchain_server.go - ensure this exists
// func (bcs *BlockchainServer) GetValidators(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
// 	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
// 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

// 	bcs.BlockchainPtr.Mutex.Lock()
// 	defer bcs.BlockchainPtr.Mutex.Unlock()

// 	// Return all validators
// 	validators := make([]map[string]interface{}, len(bcs.BlockchainPtr.Validators))
// 	for i, v := range bcs.BlockchainPtr.Validators {
// 		validators[i] = map[string]interface{}{
// 			"address":         v.Address,
// 			"stake":           v.LPStakeAmount,
// 			"liquidity_power": v.LiquidityPower,
// 			"penalty_score":   v.PenaltyScore,
// 			"blocks_proposed": v.BlocksProposed,
// 			"blocks_included": v.BlocksIncluded,
// 			"last_active":     v.LastActive.Format(time.RFC3339),
// 			"lock_time":       v.LockTime.Format(time.RFC3339),
// 		}
// 	}

// 	json.NewEncoder(w).Encode(validators)
// }

// // func (bcs *BlockchainServer) GetRecentTransactions(w http.ResponseWriter, r *http.Request) {
// // 	w.Header().Set("Content-Type", "application/json")
// // 	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
// // 	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
// // 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

// // 	if r.Method == http.MethodOptions {
// // 		w.WriteHeader(http.StatusOK)
// // 		return
// // 	}
// // 	if r.Method != http.MethodGet {
// // 		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
// // 		return
// // 	}

// // 	bcs.BlockchainPtr.Mutex.Lock()
// // 	defer bcs.BlockchainPtr.Mutex.Unlock()

// // 	json.NewEncoder(w).Encode(bcs.BlockchainPtr.RecentTxs)
// // }

// // BlockchainServer/blockchain_server.go

// func (bcs *BlockchainServer) GetRecentTransactions(w http.ResponseWriter, r *http.Request) {
// 	// CORS
// 	w.Header().Set("Content-Type", "application/json")
// 	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
// 	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
// 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

// 	if r.Method == http.MethodOptions {
// 		w.WriteHeader(http.StatusOK)
// 		return
// 	}
// 	if r.Method != http.MethodGet {
// 		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
// 		return
// 	}

// 	bcs.BlockchainPtr.Mutex.Lock()
// 	defer bcs.BlockchainPtr.Mutex.Unlock()

// 	// Merge mempool + recent history, dedupe by tx hash
// 	seen := make(map[string]struct{})
// 	var out []*blockchaincomponent.Transaction

// 	// 1) Pending / active mempool transactions
// 	for _, tx := range bcs.BlockchainPtr.Transaction_pool {
// 		h := strings.ToLower(strings.TrimSpace(tx.TxHash))
// 		if h == "" {
// 			continue
// 		}
// 		if _, ok := seen[h]; ok {
// 			continue
// 		}
// 		seen[h] = struct{}{}
// 		out = append(out, tx)
// 	}

// 	// 2) Recent history (confirmed / failed)
// 	for _, tx := range bcs.BlockchainPtr.RecentTxs {
// 		h := strings.ToLower(strings.TrimSpace(tx.TxHash))
// 		if h == "" {
// 			continue
// 		}
// 		if _, ok := seen[h]; ok {
// 			continue
// 		}
// 		seen[h] = struct{}{}
// 		out = append(out, tx)
// 	}

// 	// 3) Sort by timestamp desc (newest first)
// 	sort.Slice(out, func(i, j int) bool {
// 		return out[i].Timestamp > out[j].Timestamp
// 	})

// 	json.NewEncoder(w).Encode(out)
// }

// func min(a, b int) int {
// 	if a < b {
// 		return a
// 	}
// 	return b
// }

// func max(a, b int) int {
// 	if a > b {
// 		return a
// 	}
// 	return b
// }

// // func (b *BlockchainServer) Start() {
// // 	portStr := fmt.Sprintf("%d", b.Port)
// // 	router := mux.NewRouter()
// // 	//router.Use(enableCORS)

// // 	http.HandleFunc("/", b.getBlockchain)
// // 	http.HandleFunc("/balance", b.GetBalance)
// // 	http.HandleFunc("/send_tx", b.sendTransaction)
// // 	http.HandleFunc("/fetch_last_n_block", b.fetchNBlocks)
// // 	http.HandleFunc("/account/{address}/nonce", b.GetAccountNonce)
// // 	http.HandleFunc("/getheight", b.GetBlockchainHeight)
// // 	http.HandleFunc("/validator/{address}", b.ValidatorStats)
// // 	http.HandleFunc("/network", b.NetworkStats)
// // 	http.HandleFunc("/faucet", b.Faucet)

// // 	// You can also specify the allowed methods, headers, etc.

// //	log.Println("Blockchain server is starting on port:", b.Port)
// //	err := http.ListenAndServe("127.0.0.1:"+portStr,
// //		if err != nil {
// //			log.Fatalf("Failed to start blockchain server: %v", err)
// //		}
// //		log.Println("Blockchain server started successfully")
// //	}
// //
// // Deploy a contract
// // Add these endpoints to your server
// // func (bcs *BlockchainServer) DeployContract(w http.ResponseWriter, r *http.Request) {
// // 	var request struct {
// // 		Code  string `json:"code"`
// // 		Value uint64 `json:"value"`
// // 	}

// // 	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
// // 		http.Error(w, "Invalid request", http.StatusBadRequest)
// // 		return
// // 	}

// // 	address, err := bcs.BlockchainPtr.VM.DeployContract(request.Code, "sender", request.Value)
// // 	if err != nil {
// // 		http.Error(w, err.Error(), http.StatusInternalServerError)
// // 		return
// // 	}

// // 	json.NewEncoder(w).Encode(map[string]string{
// // 		"contract_address": address,
// // 	})
// // }

// // func (bcs *BlockchainServer) CallContract(w http.ResponseWriter, r *http.Request) {
// // 	var request struct {
// // 		Address  string   `json:"address"`
// // 		Function string   `json:"function"`
// // 		Args     []string `json:"args"`
// // 		Value    uint64   `json:"value"`
// // 	}

// // 	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
// // 		http.Error(w, "Invalid request", http.StatusBadRequest)
// // 		return
// // 	}

// // 	result, err := bcs.BlockchainPtr.VM.ExecuteContract(request.Address, "caller", request.Function, request.Args, request.Value)
// // 	if err != nil {
// // 		http.Error(w, err.Error(), http.StatusInternalServerError)
// // 		return
// // 	}

// // 	json.NewEncoder(w).Encode(result)
// // }

// // --- Add handlers ---
// func (bcs *BlockchainServer) LiquidityLock(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
// 	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
// 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

// 	if r.Method == "OPTIONS" {
// 		w.WriteHeader(http.StatusOK)
// 		return
// 	}
// 	if r.Method != http.MethodPost {
// 		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// 		return
// 	}

// 	var req struct {
// 		Address string `json:"address"`
// 		Amount  uint64 `json:"amount"`
// 		Days    int    `json:"days"`
// 	}
// 	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
// 		http.Error(w, "invalid request", http.StatusBadRequest)
// 		return
// 	}
// 	if !wallet.ValidateAddress(req.Address) {
// 		http.Error(w, "invalid address", http.StatusBadRequest)
// 		return
// 	}
// 	if req.Amount == 0 || req.Days <= 0 {
// 		http.Error(w, "invalid amount/duration", http.StatusBadRequest)
// 		return
// 	}

// 	if err := bcs.BlockchainPtr.LockLiquidity(req.Address, req.Amount, time.Duration(req.Days)*24*time.Hour); err != nil {
// 		http.Error(w, err.Error(), http.StatusBadRequest)
// 		return
// 	}
// 	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
// }

// func (bcs *BlockchainServer) LiquidityUnlock(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
// 	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
// 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

// 	if r.Method == "OPTIONS" {
// 		w.WriteHeader(http.StatusOK)
// 		return
// 	}
// 	if r.Method != http.MethodPost {
// 		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// 		return
// 	}

// 	var req struct {
// 		Address string `json:"address"`
// 	}
// 	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
// 		http.Error(w, "invalid request", http.StatusBadRequest)
// 		return
// 	}
// 	if !wallet.ValidateAddress(req.Address) {
// 		http.Error(w, "invalid address", http.StatusBadRequest)
// 		return
// 	}

// 	released, err := bcs.BlockchainPtr.UnlockAvailable(req.Address)
// 	if err != nil {
// 		http.Error(w, err.Error(), http.StatusBadRequest)
// 		return
// 	}
// 	json.NewEncoder(w).Encode(map[string]interface{}{"released": released})
// }

// func (bcs *BlockchainServer) LiquidityView(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
// 	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
// 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

// 	address := r.URL.Query().Get("address")
// 	if !wallet.ValidateAddress(address) {
// 		http.Error(w, "invalid address", http.StatusBadRequest)
// 		return
// 	}

// 	bcs.BlockchainPtr.Mutex.Lock()
// 	defer bcs.BlockchainPtr.Mutex.Unlock()

// 	locks := bcs.BlockchainPtr.LiquidityLocks[address]
// 	json.NewEncoder(w).Encode(map[string]interface{}{
// 		"total_liquidity": bcs.BlockchainPtr.TotalLiquidity,
// 		"locked":          locks,
// 		"active_locked":   bcs.BlockchainPtr.GetLock(address),
// 	})
// }

// func (bcs *BlockchainServer) RewardsRecent(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
// 	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
// 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
// 	bcs.BlockchainPtr.Mutex.Lock()
// 	defer bcs.BlockchainPtr.Mutex.Unlock()
// 	hist := bcs.BlockchainPtr.RewardHistory
// 	if len(hist) > 50 {
// 		hist = hist[len(hist)-50:]
// 	}
// 	json.NewEncoder(w).Encode(hist)
// }

// // blockchain_server.go
// func (bcs *BlockchainServer) AddValidatorFromPeer(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
// 	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
// 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

// 	if r.Method != http.MethodPost {
// 		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
// 		return
// 	}

// 	var incoming blockchaincomponent.Validator
// 	if err := json.NewDecoder(r.Body).Decode(&incoming); err != nil {
// 		http.Error(w, "invalid payload", http.StatusBadRequest)
// 		return
// 	}

// 	// Merge/dedupe
// 	bcs.BlockchainPtr.Mutex.Lock()
// 	defer bcs.BlockchainPtr.Mutex.Unlock()

// 	for _, v := range bcs.BlockchainPtr.Validators {
// 		if v.Address == incoming.Address {
// 			// Optional: update stake/LP/penalty/stats from the incoming object
// 			v.LPStakeAmount = incoming.LPStakeAmount
// 			v.LockTime = incoming.LockTime
// 			v.LiquidityPower = incoming.LiquidityPower
// 			v.PenaltyScore = incoming.PenaltyScore
// 			v.BlocksProposed = incoming.BlocksProposed
// 			v.BlocksIncluded = incoming.BlocksIncluded
// 			v.LastActive = incoming.LastActive
// 			w.WriteHeader(http.StatusOK)
// 			_ = json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
// 			return
// 		}
// 	}

// 	// New validator
// 	copy := incoming // ensure addressable
// 	bcs.BlockchainPtr.Validators = append(bcs.BlockchainPtr.Validators, &copy)

// 	// Persist (optional but recommended)
// 	dbCopy := *bcs.BlockchainPtr
// 	dbCopy.Mutex = sync.Mutex{}
// 	if err := blockchaincomponent.PutIntoDB(dbCopy); err != nil {
// 		log.Printf("persist validator merge failed: %v", err)
// 	}

// 	w.WriteHeader(http.StatusOK)
// 	_ = json.NewEncoder(w).Encode(map[string]string{"status": "added"})
// }

// func (bcs *BlockchainServer) AddValidator(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	if r.Method != http.MethodPost {
// 		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
// 		return
// 	}

// 	var req struct {
// 		Address string  `json:"address"`
// 		Amount  float64 `json:"amount"`
// 		Days    int     `json:"days"`
// 	}
// 	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
// 		http.Error(w, "invalid payload", http.StatusBadRequest)
// 		return
// 	}

// 	if !wallet.ValidateAddress(req.Address) || req.Amount <= 0 || req.Days <= 0 {
// 		http.Error(w, "invalid address/amount/days", http.StatusBadRequest)
// 		return
// 	}

// 	if err := bcs.BlockchainPtr.AddNewValidators(req.Address, req.Amount, time.Duration(req.Days)*24*time.Hour); err != nil {
// 		http.Error(w, err.Error(), http.StatusBadRequest)
// 		return
// 	}

// 	// rebroadcast all so peers converge
// 	for _, v := range bcs.BlockchainPtr.Validators {
// 		go bcs.BlockchainPtr.Network.BroadcastValidator(v)
// 	}
// 	_ = json.NewEncoder(w).Encode(map[string]any{
// 		"status": "ok", "address": req.Address, "amount": req.Amount, "minStake": bcs.BlockchainPtr.MinStake,
// 	})
// }

// // BlockchainServer/blockchain_server.go

// func (bcs *BlockchainServer) GetTransactionByHash(w http.ResponseWriter, r *http.Request) {
// 	// CORS
// 	w.Header().Set("Content-Type", "application/json")
// 	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
// 	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
// 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

// 	if r.Method == http.MethodOptions {
// 		w.WriteHeader(http.StatusOK)
// 		return
// 	}

// 	if r.Method != http.MethodGet {
// 		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
// 		return
// 	}

// 	path := r.URL.Path
// 	queryHash := r.URL.Query().Get("hash")

// 	var hash string
// 	if queryHash != "" {
// 		hash = queryHash
// 	} else if strings.HasPrefix(path, "/tx/") {
// 		hash = strings.TrimPrefix(path, "/tx/")
// 	}

// 	hash = strings.ToLower(strings.TrimSpace(hash))
// 	if hash == "" {
// 		http.Error(w, `{"error":"missing hash"}`, http.StatusBadRequest)
// 		return
// 	}

// 	bcs.BlockchainPtr.Mutex.Lock()
// 	defer bcs.BlockchainPtr.Mutex.Unlock()

// 	// 1) Look in mempool
// 	for _, tx := range bcs.BlockchainPtr.Transaction_pool {
// 		if strings.ToLower(tx.TxHash) == hash {
// 			resp := map[string]interface{}{
// 				"transaction": tx,
// 				"source":      "mempool",
// 			}
// 			json.NewEncoder(w).Encode(resp)
// 			return
// 		}
// 	}

// 	// 2) Look in in-memory blocks (confirmed)
// 	for i := len(bcs.BlockchainPtr.Blocks) - 1; i >= 0; i-- {
// 		blk := bcs.BlockchainPtr.Blocks[i]
// 		for idx, tx := range blk.Transactions {
// 			if strings.ToLower(tx.TxHash) == hash {
// 				resp := map[string]interface{}{
// 					"transaction":  tx,
// 					"source":       "block",
// 					"block_hash":   blk.CurrentHash,
// 					"block_number": blk.BlockNumber,
// 					"tx_index":     idx,
// 				}
// 				json.NewEncoder(w).Encode(resp)
// 				return
// 			}
// 		}
// 	}

// 	// 3) Look in recent tx history (failed / expired / very old)
// 	for _, tx := range bcs.BlockchainPtr.RecentTxs {
// 		if strings.ToLower(tx.TxHash) == hash {
// 			resp := map[string]interface{}{
// 				"transaction": tx,
// 				"source":      "recent",
// 			}
// 			json.NewEncoder(w).Encode(resp)
// 			return
// 		}
// 	}

// 	// 4) Not found anywhere
// 	http.Error(w, `{"error":"transaction not found"}`, http.StatusNotFound)
// }

// // BlockchainServer/blockchain_server.go

// func (bcs *BlockchainServer) GetAddressTransactions(w http.ResponseWriter, r *http.Request) {
// 	// CORS headers
// 	w.Header().Set("Content-Type", "application/json")
// 	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
// 	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
// 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

// 	if r.Method == http.MethodOptions {
// 		w.WriteHeader(http.StatusOK)
// 		return
// 	}
// 	if r.Method != http.MethodGet {
// 		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
// 		return
// 	}

// 	vars := mux.Vars(r)
// 	rawAddress := strings.TrimSpace(vars["address"])
// 	addr := strings.ToLower(rawAddress)
// 	if addr == "" {
// 		http.Error(w, `{"error":"address is required"}`, http.StatusBadRequest)
// 		return
// 	}

// 	// Optional: validate format using your existing wallet validator
// 	if !wallet.ValidateAddress(rawAddress) {
// 		http.Error(w, `{"error":"invalid address format"}`, http.StatusBadRequest)
// 		return
// 	}

// 	// Pagination params (optional; default page 1, size 50, max 200)
// 	q := r.URL.Query()
// 	page := 1
// 	pageSize := 50

// 	if pStr := q.Get("page"); pStr != "" {
// 		if p, err := strconv.Atoi(pStr); err == nil && p > 0 {
// 			page = p
// 		}
// 	}
// 	if psStr := q.Get("page_size"); psStr != "" {
// 		if ps, err := strconv.Atoi(psStr); err == nil && ps > 0 && ps <= 200 {
// 			pageSize = ps
// 		}
// 	}

// 	type addressTx struct {
// 		*blockchaincomponent.Transaction
// 		BlockNumber *uint64 `json:"block_number,omitempty"`
// 		Source      string  `json:"source,omitempty"`
// 	}

// 	bc := bcs.BlockchainPtr

// 	bc.Mutex.Lock()
// 	defer bc.Mutex.Unlock()

// 	// Collect and de-duplicate by tx_hash (lowercase)
// 	byHash := make(map[string]addressTx)

// 	// 1) Pending / mempool
// 	for _, tx := range bc.Transaction_pool {
// 		if !txTouchesAddress(tx, addr) {
// 			continue
// 		}
// 		h := strings.ToLower(strings.TrimSpace(tx.TxHash))
// 		if h == "" {
// 			continue
// 		}
// 		// Prefer confirmed later, so only set if not present
// 		if _, exists := byHash[h]; !exists {
// 			byHash[h] = addressTx{
// 				Transaction: tx,
// 				Source:      "mempool",
// 			}
// 		}
// 	}

// 	// 2) Confirmed transactions from in-memory blocks
// 	for _, blk := range bc.Blocks {
// 		blockNum := blk.BlockNumber
// 		for _, tx := range blk.Transactions {
// 			if !txTouchesAddress(tx, addr) {
// 				continue
// 			}
// 			h := strings.ToLower(strings.TrimSpace(tx.TxHash))
// 			if h == "" {
// 				continue
// 			}
// 			// Confirmed tx overrides mempool copy
// 			byHash[h] = addressTx{
// 				Transaction: tx,
// 				BlockNumber: &blockNum,
// 				Source:      "block",
// 			}
// 		}
// 	}

// 	// 3) Recent history (failed / expired / very old)
// 	// NOTE: This assumes you have a RecentTxs slice as discussed earlier.
// 	for _, tx := range bc.RecentTxs {
// 		if !txTouchesAddress(tx, addr) {
// 			continue
// 		}
// 		h := strings.ToLower(strings.TrimSpace(tx.TxHash))
// 		if h == "" {
// 			continue
// 		}
// 		// Only add if we don't already have mempool/confirmed
// 		if _, exists := byHash[h]; !exists {
// 			byHash[h] = addressTx{
// 				Transaction: tx,
// 				Source:      "recent",
// 			}
// 		}
// 	}

// 	// Flatten into slice
// 	list := make([]addressTx, 0, len(byHash))
// 	for _, v := range byHash {
// 		list = append(list, v)
// 	}

// 	// Sort by timestamp (newest first)
// 	sort.Slice(list, func(i, j int) bool {
// 		ti := uint64(0)
// 		tj := uint64(0)
// 		if list[i].Transaction != nil {
// 			ti = list[i].Transaction.Timestamp
// 		}
// 		if list[j].Transaction != nil {
// 			tj = list[j].Transaction.Timestamp
// 		}
// 		return ti > tj
// 	})

// 	// Pagination
// 	total := len(list)
// 	start := (page - 1) * pageSize
// 	if start >= total {
// 		json.NewEncoder(w).Encode([]addressTx{})
// 		return
// 	}
// 	end := start + pageSize
// 	if end > total {
// 		end = total
// 	}
// 	paged := list[start:end]

// 	// For backward-compat with your frontend, return just an array.
// 	// If you later want metadata (total/pages), change to an object.
// 	json.NewEncoder(w).Encode(paged)
// }

// func txTouchesAddress(tx *blockchaincomponent.Transaction, addr string) bool {
// 	if tx == nil {
// 		return false
// 	}
// 	a := strings.ToLower(addr)
// 	if a == "" {
// 		return false
// 	}
// 	if strings.ToLower(strings.TrimSpace(tx.From)) == a {
// 		return true
// 	}
// 	if strings.ToLower(strings.TrimSpace(tx.To)) == a {
// 		return true
// 	}
// 	return false
// }

// // code: "LQD20", "ERC20", "BEP20", "LQD721", "ERC721" or custom

// func (bcs *BlockchainServer) DeployContract(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
// 	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
// 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

// 	if r.Method == http.MethodOptions {
// 		w.WriteHeader(http.StatusOK)
// 		return
// 	}

// 	var req struct {
// 		Code  string `json:"code"`
// 		Owner string `json:"owner"`
// 		Value uint64 `json:"value"`
// 	}
// 	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
// 		http.Error(w, "invalid body", 400)
// 		return
// 	}

// 	// --------------------------------------------
// 	// 🔥 CALCULATE DEPLOY GAS USING OPTION B
// 	// --------------------------------------------
// 	contractBytes := []byte(req.Code) // VM "code" string treated as payload
// 	byteSize := len(contractBytes)

// 	gasUsed := uint64(constantset.BaseDeployGas) + (uint64(byteSize) * uint64(constantset.DeployGasPerByte))
// 	deployFee := gasUsed * uint64(constantset.GasPrice)

// 	// --------------------------------------------
// 	// 🔥 BALANCE CHECK
// 	// --------------------------------------------
// 	bal, err := bcs.BlockchainPtr.GetWalletBalance(req.Owner)
// 	if err != nil {
// 		http.Error(w, "wallet lookup failed", 500)
// 		return
// 	}
// 	if bal < deployFee {
// 		http.Error(w, fmt.Sprintf("insufficient balance: need %d LQD", deployFee), 400)
// 		return
// 	}

// 	// Deduct deploy fee NOW
// 	bcs.BlockchainPtr.Accounts[req.Owner] = bal - deployFee

// 	// --------------------------------------------
// 	// 🔥 DEPLOY CONTRACT
// 	// --------------------------------------------
// 	addr, err := bcs.BlockchainPtr.VM.DeployContract(req.Code, req.Owner, req.Value)
// 	if err != nil {
// 		http.Error(w, err.Error(), 500)
// 		return
// 	}

// 	// --------------------------------------------
// 	// 🔥 RECORD FEE SO VALIDATOR GETS PAID
// 	// --------------------------------------------
// 	if bcs.BlockchainPtr.PendingFeePool == nil {
// 		bcs.BlockchainPtr.PendingFeePool = make(map[string]uint64)
// 	}
// 	bcs.BlockchainPtr.PendingFeePool[req.Owner] += deployFee

// 	// Record synthetic tx so explorer sees it
// 	bcs.BlockchainPtr.RecordSystemTx(
// 		req.Owner,
// 		addr,
// 		req.Value, // value sent to contract, if any
// 		gasUsed,   // estimated gas used for deploy
// 		uint64(constantset.GasPrice),
// 		constantset.StatusSuccess,
// 		true, // isContract
// 		"deployContract",
// 		[]string{addr},
// 	)
// 	json.NewEncoder(w).Encode(map[string]interface{}{
// 		"contract_address": addr,
// 		"deploy_gas_used":  gasUsed,
// 		"deploy_fee_paid":  deployFee,
// 		"gas_price":        constantset.GasPrice,
// 	})
// }

// func (bcs *BlockchainServer) CallContract(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
// 	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
// 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

// 	if r.Method == http.MethodOptions {
// 		w.WriteHeader(http.StatusOK)
// 		return
// 	}

// 	var req struct {
// 		Address  string   `json:"address"`
// 		Function string   `json:"function"`
// 		Args     []string `json:"args"`
// 		Value    uint64   `json:"value"`
// 		Caller   string   `json:"caller"`
// 	}
// 	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
// 		http.Error(w, "bad body", http.StatusBadRequest)
// 		return
// 	}

// 	// --------------------------------------------
// 	// 🔍 VIEW-ONLY SHORTCUT (NO GAS, NO SYSTEM TX)
// 	// --------------------------------------------
// 	viewFn := req.Function

// 	if viewFn == "balanceOf" ||
// 		viewFn == "totalSupply" ||
// 		viewFn == "symbol" ||
// 		viewFn == "name" ||
// 		viewFn == "decimals" || // ✅ added
// 		viewFn == "allowance" || // view only
// 		viewFn == "ownerOf" || // NFT view
// 		viewFn == "getApproved" || // NFT view
// 		viewFn == "getBalanceNative" || // generic view
// 		viewFn == "getStorage" { // generic view

// 		res, err := bcs.BlockchainPtr.VM.ExecuteContract(
// 			req.Address,
// 			req.Caller,
// 			req.Function,
// 			req.Args,
// 			req.Value,
// 		)
// 		if err != nil {
// 			http.Error(w, err.Error(), http.StatusInternalServerError)
// 			return
// 		}

// 		// Special handling for balanceOf: keep compatibility with wallet JS
// 		if viewFn == "balanceOf" {
// 			var parsed uint64
// 			if res.Output != "" {
// 				if v, err := strconv.ParseUint(res.Output, 10, 64); err == nil {
// 					parsed = v
// 				}
// 			}

// 			json.NewEncoder(w).Encode(map[string]interface{}{
// 				"success":     res.Success,
// 				"gas_used":    res.GasUsed, // informational
// 				"output":      res.Output,  // ✅ needed by React wallet
// 				"raw_output":  res.Output,
// 				"balance":     parsed, // parsed uint64
// 				"function":    req.Function,
// 				"contract":    req.Address,
// 				"holder_addr": firstArgOrEmpty(req.Args),
// 			})
// 			return
// 		}

// 		// Other view calls: just return VM result
// 		json.NewEncoder(w).Encode(res)
// 		return
// 	}

// 	// --------------------------------------------
// 	// 🔥 STATE-CHANGING CALLS (WITH GAS & SYSTEM TX)
// 	// --------------------------------------------

// 	// Execute core VM
// 	res, err := bcs.BlockchainPtr.VM.ExecuteContract(
// 		req.Address,
// 		req.Caller,
// 		req.Function,
// 		req.Args,
// 		req.Value,
// 	)
// 	if err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}

// 	// 🔥 CONTRACT CALL GAS FEE
// 	gasUsed := res.GasUsed
// 	callFee := uint64(gasUsed) * uint64(constantset.GasPrice)

// 	bal, err := bcs.BlockchainPtr.GetWalletBalance(req.Caller)
// 	if err != nil {
// 		http.Error(w, "wallet node unreachable", http.StatusInternalServerError)
// 		return
// 	}
// 	if bal < callFee {
// 		http.Error(w, fmt.Sprintf("insufficient balance: need %d LQD", callFee), http.StatusBadRequest)
// 		return
// 	}

// 	// Deduct call fee
// 	bcs.BlockchainPtr.Accounts[req.Caller] = bal - callFee

// 	// Add to pending fee pool to reward validator during block production
// 	if bcs.BlockchainPtr.PendingFeePool == nil {
// 		bcs.BlockchainPtr.PendingFeePool = make(map[string]uint64)
// 	}
// 	bcs.BlockchainPtr.PendingFeePool[req.Caller] += callFee

// 	status := constantset.StatusSuccess
// 	if !res.Success {
// 		status = constantset.StatusFailed
// 	}

// 	// Record synthetic tx for this contract call
// 	bcs.BlockchainPtr.RecordSystemTx(
// 		req.Caller,
// 		req.Address,
// 		req.Value, // value in LQD sent along, often 0
// 		res.GasUsed,
// 		uint64(constantset.GasPrice),
// 		status,
// 		true,
// 		req.Function,
// 		req.Args,
// 	)

// 	json.NewEncoder(w).Encode(res)
// }

// // func (bcs *BlockchainServer) CallContract(w http.ResponseWriter, r *http.Request) {
// // 	w.Header().Set("Content-Type", "application/json")
// // 	w.Header().Set("Access-Control-Allow-Origin", "*")
// // 	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
// // 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

// // 	if r.Method == http.MethodOptions {
// // 		w.WriteHeader(http.StatusOK)
// // 		return
// // 	}

// // 	var req struct {
// // 		Address  string   `json:"address"`
// // 		Function string   `json:"function"`
// // 		Args     []string `json:"args"`
// // 		Value    uint64   `json:"value"`
// // 		Caller   string   `json:"caller"`
// // 	}
// // 	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
// // 		http.Error(w, "bad body", 400)
// // 		return
// // 	}

// // 	// --------------------------------------------
// // 	// 🔍 VIEW-ONLY SHORTCUT (NO GAS, NO SYSTEM TX)
// // 	// --------------------------------------------
// // 	if req.Function == "balanceOf" ||
// // 		req.Function == "totalSupply" ||
// // 		req.Function == "symbol" ||
// // 		req.Function == "name" {

// // 		res, err := bcs.BlockchainPtr.VM.ExecuteContract(
// // 			req.Address,
// // 			req.Caller,
// // 			req.Function,
// // 			req.Args,
// // 			req.Value,
// // 		)
// // 		if err != nil {
// // 			http.Error(w, err.Error(), 500)
// // 			return
// // 		}

// // 		// For balanceOf, expose a nice "balance" field
// // 		if req.Function == "balanceOf" {
// // 			var parsed uint64
// // 			if res.Output != "" {
// // 				if v, err := strconv.ParseUint(res.Output, 10, 64); err == nil {
// // 					parsed = v
// // 				}
// // 			}

// // 			json.NewEncoder(w).Encode(map[string]interface{}{
// // 				"success":     res.Success,
// // 				"gas_used":    res.GasUsed, // can be 0 for view
// // 				"raw_output":  res.Output,  // original VM string
// // 				"balance":     parsed,      // uint64 for wallet UI
// // 				"function":    req.Function,
// // 				"contract":    req.Address,
// // 				"holder_addr": firstArgOrEmpty(req.Args),
// // 			})
// // 			return
// // 		}

// // 		// Other view calls: just return VM result as-is
// // 		json.NewEncoder(w).Encode(res)
// // 		return
// // 	}

// // 	// --------------------------------------------
// // 	// (original logic stays the same below)
// // 	// --------------------------------------------

// // 	// Execute core VM
// // 	res, err := bcs.BlockchainPtr.VM.ExecuteContract(req.Address, req.Caller, req.Function, req.Args, req.Value)
// // 	if err != nil {
// // 		http.Error(w, err.Error(), 500)
// // 		return
// // 	}

// // 	// 🔥 CONTRACT CALL GAS FEE
// // 	gasUsed := res.GasUsed
// // 	callFee := uint64(gasUsed) * uint64(constantset.GasPrice)

// // 	bal, err := bcs.BlockchainPtr.GetWalletBalance(req.Caller)
// // 	if err != nil {
// // 		http.Error(w, "wallet node unreachable", 500)
// // 		return
// // 	}
// // 	if bal < callFee {
// // 		http.Error(w, fmt.Sprintf("insufficient balance: need %d LQD", callFee), 400)
// // 		return
// // 	}

// // 	// Deduct call fee
// // 	bcs.BlockchainPtr.Accounts[req.Caller] = bal - callFee

// // 	// Add to pending fee pool to reward validator during block production
// // 	if bcs.BlockchainPtr.PendingFeePool == nil {
// // 		bcs.BlockchainPtr.PendingFeePool = make(map[string]uint64)
// // 	}
// // 	bcs.BlockchainPtr.PendingFeePool[req.Caller] += callFee

// // 	status := constantset.StatusSuccess
// // 	if !res.Success {
// // 		status = constantset.StatusFailed
// // 	}

// // 	// Record synthetic tx for this contract call
// // 	bcs.BlockchainPtr.RecordSystemTx(
// // 		req.Caller,
// // 		req.Address,
// // 		req.Value, // value in LQD sent along, often 0
// // 		res.GasUsed,
// // 		uint64(constantset.GasPrice),
// // 		status,
// // 		true,
// // 		req.Function,
// // 		req.Args,
// // 	)

// //		json.NewEncoder(w).Encode(res)
// //	}
// func firstArgOrEmpty(args []string) string {
// 	if len(args) > 0 {
// 		return args[0]
// 	}
// 	return ""
// }

// // func (bcs *BlockchainServer) CallContract(w http.ResponseWriter, r *http.Request) {
// // 	w.Header().Set("Content-Type", "application/json")
// // 	w.Header().Set("Access-Control-Allow-Origin", "*")
// // 	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
// // 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

// // 	if r.Method == http.MethodOptions {
// // 		w.WriteHeader(http.StatusOK)
// // 		return
// // 	}

// // 	var req struct {
// // 		Address  string   `json:"address"`
// // 		Function string   `json:"function"`
// // 		Args     []string `json:"args"`
// // 		Value    uint64   `json:"value"`
// // 		Caller   string   `json:"caller"`
// // 	}
// // 	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
// // 		http.Error(w, "bad body", 400)
// // 		return
// // 	}

// // 	// Execute core VM
// // 	res, err := bcs.BlockchainPtr.VM.ExecuteContract(req.Address, req.Caller, req.Function, req.Args, req.Value)
// // 	if err != nil {
// // 		http.Error(w, err.Error(), 500)
// // 		return
// // 	}

// // 	// --------------------------------------------
// // 	// 🔥 CONTRACT CALL GAS FEE
// // 	// --------------------------------------------
// // 	gasUsed := res.GasUsed
// // 	callFee := uint64(gasUsed) * uint64(constantset.GasPrice)

// // 	bal, err := bcs.BlockchainPtr.GetWalletBalance(req.Caller)
// // 	if err != nil {
// // 		http.Error(w, "wallet node unreachable", 500)
// // 		return
// // 	}
// // 	if bal < callFee {
// // 		http.Error(w, fmt.Sprintf("insufficient balance: need %d LQD", callFee), 400)
// // 		return
// // 	}

// // 	// Deduct call fee
// // 	bcs.BlockchainPtr.Accounts[req.Caller] = bal - callFee

// // 	// Add to pending fee pool to reward validator during block production
// // 	if bcs.BlockchainPtr.PendingFeePool == nil {
// // 		bcs.BlockchainPtr.PendingFeePool = make(map[string]uint64)
// // 	}
// // 	bcs.BlockchainPtr.PendingFeePool[req.Caller] += callFee

// // 	status := constantset.StatusSuccess
// // 	if !res.Success {
// // 		status = constantset.StatusFailed
// // 	}

// // 	// Record synthetic tx for this contract call
// // 	bcs.BlockchainPtr.RecordSystemTx(
// // 		req.Caller,
// // 		req.Address,
// // 		req.Value, // value in LQD sent along, often 0
// // 		res.GasUsed,
// // 		uint64(constantset.GasPrice),
// // 		status,
// // 		true,
// // 		req.Function,
// // 		req.Args,
// // 	)

// //		json.NewEncoder(w).Encode(res)
// //	}
// //
// // GET /token-balance?contract=0x...&holder=0x...
// func (bcs *BlockchainServer) GetTokenBalance(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	w.Header().Set("Access-Control-Allow-Origin", "localhost:3000")
// 	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
// 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

// 	if r.Method == http.MethodOptions {
// 		w.WriteHeader(http.StatusOK)
// 		return
// 	}

// 	if r.Method != http.MethodGet {
// 		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
// 		return
// 	}

// 	q := r.URL.Query()
// 	contractAddr := q.Get("contract")
// 	holderAddr := q.Get("holder")

// 	if contractAddr == "" || holderAddr == "" {
// 		http.Error(w, `{"error":"contract and holder are required"}`, http.StatusBadRequest)
// 		return
// 	}

// 	// Call VM directly as a view (no gas, no tx)
// 	res, err := bcs.BlockchainPtr.VM.ExecuteContract(
// 		contractAddr,
// 		holderAddr,
// 		"balanceOf",
// 		[]string{holderAddr},
// 		0,
// 	)
// 	if err != nil {
// 		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
// 		return
// 	}

// 	// Parse numeric balance from res.Output (string -> uint64)
// 	var parsed uint64
// 	if res.Output != "" {
// 		if v, err := strconv.ParseUint(res.Output, 10, 64); err == nil {
// 			parsed = v
// 		}
// 	}

// 	resp := map[string]interface{}{
// 		"success":     res.Success,
// 		"contract":    contractAddr,
// 		"holder":      holderAddr,
// 		"raw_output":  res.Output,
// 		"balance":     parsed,
// 		"function":    "balanceOf",
// 		"vm_gas_used": res.GasUsed, // usually 0 for view
// 	}

// 	json.NewEncoder(w).Encode(resp)
// }
// func (bcs *BlockchainServer) BlockTimeLatest(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
// 	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
// 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
// 	if r.Method == http.MethodOptions {
// 		w.WriteHeader(http.StatusOK)
// 		return
// 	}
// 	if r.Method != http.MethodGet {
// 		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
// 		return
// 	}
// 	bc := bcs.BlockchainPtr

// 	if len(bc.Blocks) < 1 {
// 		json.NewEncoder(w).Encode(map[string]interface{}{
// 			"error": "not enough blocks",
// 		})
// 		return
// 	}

// 	last := bc.Blocks[len(bc.Blocks)-1]

// 	// mining duration measured in MineNewBlock
// 	mining := bc.LastBlockMiningTime

// 	// optional: also keep interval between block timestamps for comparison
// 	var interval time.Duration
// 	if len(bc.Blocks) >= 2 {
// 		//prev := bc.Blocks[len(bc.Blocks)-2]
// 		//interval = last.Timestamp.Sub(prev.Timestamp)
// 	}

// 	json.NewEncoder(w).Encode(map[string]interface{}{
// 		"block_number":      last.BlockNumber,
// 		"mining_time_ms":    mining.Milliseconds(),
// 		"mining_time_sec":   mining.Seconds(),
// 		"interval_time_ms":  interval.Milliseconds(),
// 		"interval_time_sec": interval.Seconds(),
// 	})
// }

// func (bcs *BlockchainServer) DeployWASMContract(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	w.Header().Set("Access-Control-Allow-Origin", "localhost:3000")
// 	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
// 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

// 	if r.Method != http.MethodPost {
// 		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
// 		return
// 	}

// 	var req struct {
// 		Address string `json:"address"`  // contract address (0x...)
// 		Creator string `json:"creator"`  // deployer address
// 		CodeHex string `json:"code_hex"` // hex-encoded wasm bytes
// 	}

// 	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
// 		http.Error(w, `{"error":"invalid payload"}`, http.StatusBadRequest)
// 		return
// 	}

// 	code, err := hex.DecodeString(strings.TrimPrefix(req.CodeHex, "0x"))
// 	if err != nil {
// 		http.Error(w, `{"error":"invalid wasm hex"}`, http.StatusBadRequest)
// 		return
// 	}

// 	if err := bcs.BlockchainPtr.WVM.Deploy(req.Address, code, req.Creator); err != nil {
// 		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
// 		return
// 	}

// 	json.NewEncoder(w).Encode(map[string]any{
// 		"status":  "ok",
// 		"address": req.Address,
// 		"creator": req.Creator,
// 	})
// }
// func (bcs *BlockchainServer) CallWASMContract(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	w.Header().Set("Access-Control-Allow-Origin", "localhost:3000")
// 	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
// 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

// 	if r.Method != http.MethodPost {
// 		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
// 		return
// 	}

// 	var req struct {
// 		Contract string   `json:"contract"` // contract address
// 		Caller   string   `json:"caller"`   // msg.sender
// 		Method   string   `json:"method"`   // ABI method name
// 		Args     []string `json:"args"`     // arguments as strings
// 	}

// 	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
// 		http.Error(w, `{"error":"invalid payload"}`, http.StatusBadRequest)
// 		return
// 	}

// 	out, err := bcs.BlockchainPtr.WVM.Call(req.Contract, req.Caller, req.Method, req.Args)
// 	if err != nil {
// 		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
// 		return
// 	}

// 	json.NewEncoder(w).Encode(map[string]any{
// 		"status": "ok",
// 		"output": out,
// 		"method": req.Method,
// 		"args":   req.Args,
// 	})
// }

// func (b *BlockchainServer) Start() {
// 	portStr := fmt.Sprintf("%d", b.Port)
// 	//router := mux.NewRouter()

// 	// Define your routes
// 	http.HandleFunc("/", b.getBlockchain)
// 	http.HandleFunc("/balance", b.GetBalance)
// 	http.HandleFunc("/send_tx", b.sendTransaction)
// 	http.HandleFunc("/fetch_last_n_block", b.fetchNBlocks)
// 	http.HandleFunc("/account/{address}/nonce", b.GetAccountNonce)
// 	http.HandleFunc("/getheight", b.GetBlockchainHeight)
// 	http.HandleFunc("/validator/{address}", b.ValidatorStats)
// 	http.HandleFunc("/network", b.NetworkStats)
// 	http.HandleFunc("/faucet", b.Faucet)
// 	http.HandleFunc("/block/{id}", b.GetBlock)
// 	http.HandleFunc("/validators", b.GetValidators)
// 	http.HandleFunc("/transactions/recent", b.GetRecentTransactions)
// 	http.HandleFunc("/liquidity/lock", b.LiquidityLock)
// 	http.HandleFunc("/liquidity/unlock", b.LiquidityUnlock)
// 	http.HandleFunc("/liquidity", b.LiquidityView)
// 	http.HandleFunc("/rewards/recent", b.RewardsRecent)
// 	http.HandleFunc("/validator/new", b.AddValidatorFromPeer)
// 	http.HandleFunc("/validator/add", b.AddValidator)
// 	http.HandleFunc("/tx/", b.GetTransactionByHash)
// 	http.HandleFunc("/contract/call", b.CallContract)
// 	http.HandleFunc("/contract/deploy", b.DeployContract)
// 	http.HandleFunc("/address/{address}/transactions", b.GetAddressTransactions)
// 	http.HandleFunc("/blocktime/latest", b.BlockTimeLatest)
// 	http.HandleFunc("/contract/wasm/deploy", b.DeployWASMContract)
// 	http.HandleFunc("/contract/wasm/call", b.CallWASMContract)

// 	log.Println("Blockchain server is starting on port:", b.Port)

// 	// Use the CORS handler
// 	err := http.ListenAndServe("127.0.0.1:"+portStr, nil)
// 	if err != nil {
// 		log.Fatalf("Failed to start blockchain server: %v", err)
// 	}
// 	log.Println("Blockchain server started successfully")
// }

package blockchainserver

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	blockchaincomponent "github.com/Zotish/DefenceProject/BlockchainComponent"
	constantset "github.com/Zotish/DefenceProject/ConstantSet"
	wallet "github.com/Zotish/DefenceProject/WalletComponent"
	"github.com/gorilla/mux"
	//"github.com/rs/cors"
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

// func (bcs *BlockchainServer) GetAccountNonce(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
// 	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
// 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
// 	address := mux.Vars(r)["address"]
// 	nonce := bcs.BlockchainPtr.GetAccountNonce(address)

//		json.NewEncoder(w).Encode(map[string]uint64{
//			"nonce": nonce,
//		})
//	}
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

// func (b *BlockchainServer) fetchNBlocks(w http.ResponseWriter, r *http.Request) {
// 	log.Printf("fetchNBlocks called - Method: %s, Origin: %s", r.Method, r.Header.Get("Origin"))

// 	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
// 	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
// 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
// 	w.Header().Set("Content-Type", "application/json")

// 	// Handle preflight requests
// 	if r.Method == "OPTIONS" {
// 		log.Println("Handling OPTIONS preflight request")
// 		w.WriteHeader(http.StatusOK)
// 		return
// 	}

// 	if r.Method != http.MethodGet {
// 		log.Printf("Method not allowed: %s", r.Method)
// 		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// 		return
// 	}

// 	b.BlockchainPtr.Mutex.Lock()
// 	defer b.BlockchainPtr.Mutex.Unlock()

// 	blocks := b.BlockchainPtr.Blocks
// 	var blocksToReturn []*blockchaincomponent.Block
// 	if len(blocks) < 10 {
// 		blocksToReturn = blocks
// 	} else {
// 		blocksToReturn = blocks[len(blocks)-10:]
// 	}

// 	log.Printf("Returning %d blocks", len(blocksToReturn))
// 	json.NewEncoder(w).Encode(blocksToReturn)
// }

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
	confirmedBalance := bcs.BlockchainPtr.Accounts[address]

	// Calculate pending balance changes from transaction pool
	pendingBalanceChange := uint64(0)
	for _, tx := range bcs.BlockchainPtr.Transaction_pool {
		if tx.From == address && tx.Status == constantset.StatusPending {
			pendingBalanceChange -= uint64(tx.Value + (tx.GasPrice * tx.CalculateGasCost()))
		}
		if tx.To == address && tx.Status == constantset.StatusPending {
			pendingBalanceChange += uint64(tx.Value)
		}
	}

	totalBalance := uint64(max(0, int(uint64(confirmedBalance)+uint64(pendingBalanceChange))))

	json.NewEncoder(w).Encode(map[string]interface{}{
		"address":                address,
		"balance":                totalBalance,
		"confirmed_balance":      confirmedBalance,
		"pending_balance_change": pendingBalanceChange,
	})
}

//	func (b *BlockchainServer) fetchNBlocks(w http.ResponseWriter, r *http.Request) {
//		w.Header().Set("Content-Type", "application/json")
//		if r.Method == http.MethodGet {
//			blocks := b.BlockchainPtr.Blocks
//			blockchain1 := new(blockchaincomponent.Blockchain_struct)
//			if len(blocks) < 10 {
//				blockchain1.Blocks = blocks
//			} else {
//				blockchain1.Blocks = blocks[len(blocks)-10:]
//			}
//		} else {
//			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
//			return
//		}
//	}
// func (bcs *BlockchainServer) GetBalance(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
// 	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
// 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
// 	address := r.URL.Query().Get("address")
// 	balance := bcs.BlockchainPtr.Accounts[address]
// 	json.NewEncoder(w).Encode(map[string]interface{}{"balance": balance})
// }

// blockchain_server.go
//
//	func (bcs *BlockchainServer) Faucet(w http.ResponseWriter, r *http.Request) {
//		w.Header().Set("Content-Type", "application/json")
//		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
//		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
//		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
//		address := r.URL.Query().Get("address")
//		if !wallet.ValidateAddress(address) {
//			http.Error(w, "Invalid address", http.StatusBadRequest)
//			return
//		}
//		tx := blockchaincomponent.NewTransaction("0xFaucet", address, 100000, []byte{}, bcs.BlockchainPtr.GetAccountNonce("0xFaucet"))
//		bcs.BlockchainPtr.AddNewTxToTheTransaction_pool(tx)
//	}
// func (bcs *BlockchainServer) AddValidatorFromPeer(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	var v blockchaincomponent.Validator
// 	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
// 		http.Error(w, "invalid", http.StatusBadRequest)
// 		return
// 	}

// 	// Check for duplicates
// 	for _, existing := range bcs.BlockchainPtr.Validators {
// 		if existing.Address == v.Address {
// 			w.WriteHeader(http.StatusOK)
// 			return
// 		}
// 	}

// 	bcs.BlockchainPtr.Mutex.Lock()
// 	bcs.BlockchainPtr.Validators = append(bcs.BlockchainPtr.Validators, &v)
// 	bcs.BlockchainPtr.Mutex.Unlock()
// 	log.Printf("✅ Added validator from peer: %s", v.Address)
// 	w.WriteHeader(http.StatusOK)
// }

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
	amount := uint64(100_000_00)
	bcs.BlockchainPtr.Mutex.Lock()
	bcs.BlockchainPtr.Accounts[address] += amount
	bcs.BlockchainPtr.Mutex.Unlock()

	json.NewEncoder(w).Encode(map[string]interface{}{"credited": amount, "address": address})
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
	stats := bcs.BlockchainPtr.GetNetworkStats()
	json.NewEncoder(w).Encode(stats)
}
func (bcs *BlockchainServer) Metrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "podl_blocks_total %d\n", len(bcs.BlockchainPtr.Blocks))
	fmt.Fprintf(w, "podl_validators_total %d\n", len(bcs.BlockchainPtr.Validators))
	fmt.Fprintf(w, "podl_slashing_pool %.2f\n", bcs.BlockchainPtr.SlashingPool)
}

// Add to blockchain_server.go
func (b *BlockchainServer) GetBlock(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	vars := mux.Vars(r)
	blockNumber, err := strconv.ParseUint(vars["id"], 10, 64)
	if err != nil {
		http.Error(w, "Invalid block number", http.StatusBadRequest)
		return
	}

	b.BlockchainPtr.Mutex.Lock()
	defer b.BlockchainPtr.Mutex.Unlock()

	if blockNumber >= uint64(len(b.BlockchainPtr.Blocks)) {
		http.Error(w, "Block not found", http.StatusNotFound)
		return
	}

	block := b.BlockchainPtr.Blocks[blockNumber]
	json.NewEncoder(w).Encode(block)
}

// Add to blockchain_server.go
// In blockchain_server.go - ensure this exists
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

// func (bcs *BlockchainServer) GetRecentTransactions(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
// 	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
// 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

// 	if r.Method == http.MethodOptions {
// 		w.WriteHeader(http.StatusOK)
// 		return
// 	}
// 	if r.Method != http.MethodGet {
// 		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
// 		return
// 	}

// 	bcs.BlockchainPtr.Mutex.Lock()
// 	defer bcs.BlockchainPtr.Mutex.Unlock()

// 	json.NewEncoder(w).Encode(bcs.BlockchainPtr.RecentTxs)
// }

// BlockchainServer/blockchain_server.go

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

	// Merge mempool + recent history, dedupe by tx hash
	seen := make(map[string]struct{})
	var out []*blockchaincomponent.Transaction

	// 1) Pending / active mempool transactions
	for _, tx := range bcs.BlockchainPtr.Transaction_pool {
		h := strings.ToLower(strings.TrimSpace(tx.TxHash))
		if h == "" {
			continue
		}
		if _, ok := seen[h]; ok {
			continue
		}
		seen[h] = struct{}{}
		out = append(out, tx)
	}

	// 2) Recent history (confirmed / failed)
	for _, tx := range bcs.BlockchainPtr.RecentTxs {
		h := strings.ToLower(strings.TrimSpace(tx.TxHash))
		if h == "" {
			continue
		}
		if _, ok := seen[h]; ok {
			continue
		}
		seen[h] = struct{}{}
		out = append(out, tx)
	}

	// 3) Sort by timestamp desc (newest first)
	sort.Slice(out, func(i, j int) bool {
		return out[i].Timestamp > out[j].Timestamp
	})

	json.NewEncoder(w).Encode(out)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// func (b *BlockchainServer) Start() {
// 	portStr := fmt.Sprintf("%d", b.Port)
// 	router := mux.NewRouter()
// 	//router.Use(enableCORS)

// 	http.HandleFunc("/", b.getBlockchain)
// 	http.HandleFunc("/balance", b.GetBalance)
// 	http.HandleFunc("/send_tx", b.sendTransaction)
// 	http.HandleFunc("/fetch_last_n_block", b.fetchNBlocks)
// 	http.HandleFunc("/account/{address}/nonce", b.GetAccountNonce)
// 	http.HandleFunc("/getheight", b.GetBlockchainHeight)
// 	http.HandleFunc("/validator/{address}", b.ValidatorStats)
// 	http.HandleFunc("/network", b.NetworkStats)
// 	http.HandleFunc("/faucet", b.Faucet)

// 	// You can also specify the allowed methods, headers, etc.

//	log.Println("Blockchain server is starting on port:", b.Port)
//	err := http.ListenAndServe("127.0.0.1:"+portStr,
//		if err != nil {
//			log.Fatalf("Failed to start blockchain server: %v", err)
//		}
//		log.Println("Blockchain server started successfully")
//	}
//
// Deploy a contract
// Add these endpoints to your server
// func (bcs *BlockchainServer) DeployContract(w http.ResponseWriter, r *http.Request) {
// 	var request struct {
// 		Code  string `json:"code"`
// 		Value uint64 `json:"value"`
// 	}

// 	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
// 		http.Error(w, "Invalid request", http.StatusBadRequest)
// 		return
// 	}

// 	address, err := bcs.BlockchainPtr.VM.DeployContract(request.Code, "sender", request.Value)
// 	if err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}

// 	json.NewEncoder(w).Encode(map[string]string{
// 		"contract_address": address,
// 	})
// }

// func (bcs *BlockchainServer) CallContract(w http.ResponseWriter, r *http.Request) {
// 	var request struct {
// 		Address  string   `json:"address"`
// 		Function string   `json:"function"`
// 		Args     []string `json:"args"`
// 		Value    uint64   `json:"value"`
// 	}

// 	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
// 		http.Error(w, "Invalid request", http.StatusBadRequest)
// 		return
// 	}

// 	result, err := bcs.BlockchainPtr.VM.ExecuteContract(request.Address, "caller", request.Function, request.Args, request.Value)
// 	if err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}

// 	json.NewEncoder(w).Encode(result)
// }

// --- Add handlers ---
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
		Amount  uint64 `json:"amount"`
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
	if req.Amount == 0 || req.Days <= 0 {
		http.Error(w, "invalid amount/duration", http.StatusBadRequest)
		return
	}

	if err := bcs.BlockchainPtr.LockLiquidity(req.Address, req.Amount, time.Duration(req.Days)*24*time.Hour); err != nil {
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
	json.NewEncoder(w).Encode(map[string]interface{}{"released": released})
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
		"total_liquidity": bcs.BlockchainPtr.TotalLiquidity,
		"locked":          locks,
		"active_locked":   bcs.BlockchainPtr.GetLock(address),
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

// blockchain_server.go
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

// BlockchainServer/blockchain_server.go

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

// BlockchainServer/blockchain_server.go

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

// code: "LQD20", "ERC20", "BEP20", "LQD721", "ERC721" or custom

// func (bcs *BlockchainServer) DeployContract(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	w.Header().Set("Access-Control-Allow-Origin", "*")
// 	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
// 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

// 	if r.Method == http.MethodOptions {
// 		w.WriteHeader(http.StatusOK)
// 		return
// 	}

// 	var req struct {
// 		Code  string `json:"code"`
// 		Owner string `json:"owner"`
// 		Value uint64 `json:"value"`
// 	}
// 	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
// 		http.Error(w, "invalid body", 400)
// 		return
// 	}

// 	// --------------------------------------------
// 	// 🔥 CALCULATE DEPLOY GAS USING OPTION B
// 	// --------------------------------------------
// 	contractBytes := []byte(req.Code) // VM "code" string treated as payload
// 	byteSize := len(contractBytes)

// 	gasUsed := uint64(constantset.BaseDeployGas) + (uint64(byteSize) * uint64(constantset.DeployGasPerByte))
// 	deployFee := gasUsed * uint64(constantset.GasPrice)

// 	// --------------------------------------------
// 	// 🔥 BALANCE CHECK
// 	// --------------------------------------------
// 	bal, err := bcs.BlockchainPtr.GetWalletBalance(req.Owner)
// 	if err != nil {
// 		http.Error(w, "wallet lookup failed", 500)
// 		return
// 	}
// 	if bal < deployFee {
// 		http.Error(w, fmt.Sprintf("insufficient balance: need %d LQD", deployFee), 400)
// 		return
// 	}

// 	// Deduct deploy fee NOW
// 	bcs.BlockchainPtr.Accounts[req.Owner] = bal - deployFee

// 	// --------------------------------------------
// 	// 🔥 DEPLOY CONTRACT
// 	// --------------------------------------------
// 	addr, err := bcs.BlockchainPtr.VM.DeployContract(req.Code, req.Owner, req.Value)
// 	if err != nil {
// 		http.Error(w, err.Error(), 500)
// 		return
// 	}

// 	// --------------------------------------------
// 	// 🔥 RECORD FEE SO VALIDATOR GETS PAID
// 	// --------------------------------------------
// 	if bcs.BlockchainPtr.PendingFeePool == nil {
// 		bcs.BlockchainPtr.PendingFeePool = make(map[string]uint64)
// 	}
// 	bcs.BlockchainPtr.PendingFeePool[req.Owner] += deployFee

// 	// Record synthetic tx so explorer sees it
// 	bcs.BlockchainPtr.RecordSystemTx(
// 		req.Owner,
// 		addr,
// 		req.Value, // value sent to contract, if any
// 		gasUsed,   // estimated gas used for deploy
// 		uint64(constantset.GasPrice),
// 		constantset.StatusSuccess,
// 		true, // isContract
// 		"deployContract",
// 		[]string{addr},
// 	)
// 	json.NewEncoder(w).Encode(map[string]interface{}{
// 		"contract_address": addr,
// 		"deploy_gas_used":  gasUsed,
// 		"deploy_fee_paid":  deployFee,
// 		"gas_price":        constantset.GasPrice,
// 	})
// }

// func (bcs *BlockchainServer) CallContract(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	w.Header().Set("Access-Control-Allow-Origin", "*")
// 	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
// 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

// 	if r.Method == http.MethodOptions {
// 		w.WriteHeader(http.StatusOK)
// 		return
// 	}

// 	var req struct {
// 		Address  string   `json:"address"`
// 		Function string   `json:"function"`
// 		Args     []string `json:"args"`
// 		Value    uint64   `json:"value"`
// 		Caller   string   `json:"caller"`
// 	}
// 	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
// 		http.Error(w, "bad body", http.StatusBadRequest)
// 		return
// 	}

// 	// --------------------------------------------
// 	// 🔍 VIEW-ONLY SHORTCUT (NO GAS, NO SYSTEM TX)
// 	// --------------------------------------------
// 	viewFn := req.Function

// 	if viewFn == "balanceOf" ||
// 		viewFn == "totalSupply" ||
// 		viewFn == "symbol" ||
// 		viewFn == "name" ||
// 		viewFn == "decimals" || // ✅ added
// 		viewFn == "allowance" || // view only
// 		viewFn == "ownerOf" || // NFT view
// 		viewFn == "getApproved" || // NFT view
// 		viewFn == "getBalanceNative" || // generic view
// 		viewFn == "getStorage" { // generic view

// 		res, err := bcs.BlockchainPtr.VM.ExecuteContract(
// 			req.Address,
// 			req.Caller,
// 			req.Function,
// 			req.Args,
// 			req.Value,
// 		)
// 		if err != nil {
// 			http.Error(w, err.Error(), http.StatusInternalServerError)
// 			return
// 		}

// 		// Special handling for balanceOf: keep compatibility with wallet JS
// 		if viewFn == "balanceOf" {
// 			var parsed uint64
// 			if res.Output != "" {
// 				if v, err := strconv.ParseUint(res.Output, 10, 64); err == nil {
// 					parsed = v
// 				}
// 			}

// 			json.NewEncoder(w).Encode(map[string]interface{}{
// 				"success":     res.Success,
// 				"gas_used":    res.GasUsed, // informational
// 				"output":      res.Output,  // ✅ needed by React wallet
// 				"raw_output":  res.Output,
// 				"balance":     parsed, // parsed uint64
// 				"function":    req.Function,
// 				"contract":    req.Address,
// 				"holder_addr": firstArgOrEmpty(req.Args),
// 			})
// 			return
// 		}

// 		// Other view calls: just return VM result
// 		json.NewEncoder(w).Encode(res)
// 		return
// 	}

// 	// --------------------------------------------
// 	// 🔥 STATE-CHANGING CALLS (WITH GAS & SYSTEM TX)
// 	// --------------------------------------------

// 	// Execute core VM
// 	res, err := bcs.BlockchainPtr.VM.ExecuteContract(
// 		req.Address,
// 		req.Caller,
// 		req.Function,
// 		req.Args,
// 		req.Value,
// 	)
// 	if err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}

// 	// 🔥 CONTRACT CALL GAS FEE
// 	gasUsed := res.GasUsed
// 	callFee := uint64(gasUsed) * uint64(constantset.GasPrice)

// 	bal, err := bcs.BlockchainPtr.GetWalletBalance(req.Caller)
// 	if err != nil {
// 		http.Error(w, "wallet node unreachable", http.StatusInternalServerError)
// 		return
// 	}
// 	if bal < callFee {
// 		http.Error(w, fmt.Sprintf("insufficient balance: need %d LQD", callFee), http.StatusBadRequest)
// 		return
// 	}

// 	// Deduct call fee
// 	bcs.BlockchainPtr.Accounts[req.Caller] = bal - callFee

// 	// Add to pending fee pool to reward validator during block production
// 	if bcs.BlockchainPtr.PendingFeePool == nil {
// 		bcs.BlockchainPtr.PendingFeePool = make(map[string]uint64)
// 	}
// 	bcs.BlockchainPtr.PendingFeePool[req.Caller] += callFee

// 	status := constantset.StatusSuccess
// 	if !res.Success {
// 		status = constantset.StatusFailed
// 	}

// 	// Record synthetic tx for this contract call
// 	bcs.BlockchainPtr.RecordSystemTx(
// 		req.Caller,
// 		req.Address,
// 		req.Value, // value in LQD sent along, often 0
// 		res.GasUsed,
// 		uint64(constantset.GasPrice),
// 		status,
// 		true,
// 		req.Function,
// 		req.Args,
// 	)

// 	json.NewEncoder(w).Encode(res)
// }

// func (bcs *BlockchainServer) CallContract(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	w.Header().Set("Access-Control-Allow-Origin", "*")
// 	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
// 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

// 	if r.Method == http.MethodOptions {
// 		w.WriteHeader(http.StatusOK)
// 		return
// 	}

// 	var req struct {
// 		Address  string   `json:"address"`
// 		Function string   `json:"function"`
// 		Args     []string `json:"args"`
// 		Value    uint64   `json:"value"`
// 		Caller   string   `json:"caller"`
// 	}
// 	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
// 		http.Error(w, "bad body", 400)
// 		return
// 	}

// 	// --------------------------------------------
// 	// 🔍 VIEW-ONLY SHORTCUT (NO GAS, NO SYSTEM TX)
// 	// --------------------------------------------
// 	if req.Function == "balanceOf" ||
// 		req.Function == "totalSupply" ||
// 		req.Function == "symbol" ||
// 		req.Function == "name" {

// 		res, err := bcs.BlockchainPtr.VM.ExecuteContract(
// 			req.Address,
// 			req.Caller,
// 			req.Function,
// 			req.Args,
// 			req.Value,
// 		)
// 		if err != nil {
// 			http.Error(w, err.Error(), 500)
// 			return
// 		}

// 		// For balanceOf, expose a nice "balance" field
// 		if req.Function == "balanceOf" {
// 			var parsed uint64
// 			if res.Output != "" {
// 				if v, err := strconv.ParseUint(res.Output, 10, 64); err == nil {
// 					parsed = v
// 				}
// 			}

// 			json.NewEncoder(w).Encode(map[string]interface{}{
// 				"success":     res.Success,
// 				"gas_used":    res.GasUsed, // can be 0 for view
// 				"raw_output":  res.Output,  // original VM string
// 				"balance":     parsed,      // uint64 for wallet UI
// 				"function":    req.Function,
// 				"contract":    req.Address,
// 				"holder_addr": firstArgOrEmpty(req.Args),
// 			})
// 			return
// 		}

// 		// Other view calls: just return VM result as-is
// 		json.NewEncoder(w).Encode(res)
// 		return
// 	}

// 	// --------------------------------------------
// 	// (original logic stays the same below)
// 	// --------------------------------------------

// 	// Execute core VM
// 	res, err := bcs.BlockchainPtr.VM.ExecuteContract(req.Address, req.Caller, req.Function, req.Args, req.Value)
// 	if err != nil {
// 		http.Error(w, err.Error(), 500)
// 		return
// 	}

// 	// 🔥 CONTRACT CALL GAS FEE
// 	gasUsed := res.GasUsed
// 	callFee := uint64(gasUsed) * uint64(constantset.GasPrice)

// 	bal, err := bcs.BlockchainPtr.GetWalletBalance(req.Caller)
// 	if err != nil {
// 		http.Error(w, "wallet node unreachable", 500)
// 		return
// 	}
// 	if bal < callFee {
// 		http.Error(w, fmt.Sprintf("insufficient balance: need %d LQD", callFee), 400)
// 		return
// 	}

// 	// Deduct call fee
// 	bcs.BlockchainPtr.Accounts[req.Caller] = bal - callFee

// 	// Add to pending fee pool to reward validator during block production
// 	if bcs.BlockchainPtr.PendingFeePool == nil {
// 		bcs.BlockchainPtr.PendingFeePool = make(map[string]uint64)
// 	}
// 	bcs.BlockchainPtr.PendingFeePool[req.Caller] += callFee

// 	status := constantset.StatusSuccess
// 	if !res.Success {
// 		status = constantset.StatusFailed
// 	}

// 	// Record synthetic tx for this contract call
// 	bcs.BlockchainPtr.RecordSystemTx(
// 		req.Caller,
// 		req.Address,
// 		req.Value, // value in LQD sent along, often 0
// 		res.GasUsed,
// 		uint64(constantset.GasPrice),
// 		status,
// 		true,
// 		req.Function,
// 		req.Args,
// 	)

//		json.NewEncoder(w).Encode(res)
//	}
func firstArgOrEmpty(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return ""
}

// func (bcs *BlockchainServer) CallContract(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	w.Header().Set("Access-Control-Allow-Origin", "*")
// 	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
// 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

// 	if r.Method == http.MethodOptions {
// 		w.WriteHeader(http.StatusOK)
// 		return
// 	}

// 	var req struct {
// 		Address  string   `json:"address"`
// 		Function string   `json:"function"`
// 		Args     []string `json:"args"`
// 		Value    uint64   `json:"value"`
// 		Caller   string   `json:"caller"`
// 	}
// 	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
// 		http.Error(w, "bad body", 400)
// 		return
// 	}

// 	// Execute core VM
// 	res, err := bcs.BlockchainPtr.VM.ExecuteContract(req.Address, req.Caller, req.Function, req.Args, req.Value)
// 	if err != nil {
// 		http.Error(w, err.Error(), 500)
// 		return
// 	}

// 	// --------------------------------------------
// 	// 🔥 CONTRACT CALL GAS FEE
// 	// --------------------------------------------
// 	gasUsed := res.GasUsed
// 	callFee := uint64(gasUsed) * uint64(constantset.GasPrice)

// 	bal, err := bcs.BlockchainPtr.GetWalletBalance(req.Caller)
// 	if err != nil {
// 		http.Error(w, "wallet node unreachable", 500)
// 		return
// 	}
// 	if bal < callFee {
// 		http.Error(w, fmt.Sprintf("insufficient balance: need %d LQD", callFee), 400)
// 		return
// 	}

// 	// Deduct call fee
// 	bcs.BlockchainPtr.Accounts[req.Caller] = bal - callFee

// 	// Add to pending fee pool to reward validator during block production
// 	if bcs.BlockchainPtr.PendingFeePool == nil {
// 		bcs.BlockchainPtr.PendingFeePool = make(map[string]uint64)
// 	}
// 	bcs.BlockchainPtr.PendingFeePool[req.Caller] += callFee

// 	status := constantset.StatusSuccess
// 	if !res.Success {
// 		status = constantset.StatusFailed
// 	}

// 	// Record synthetic tx for this contract call
// 	bcs.BlockchainPtr.RecordSystemTx(
// 		req.Caller,
// 		req.Address,
// 		req.Value, // value in LQD sent along, often 0
// 		res.GasUsed,
// 		uint64(constantset.GasPrice),
// 		status,
// 		true,
// 		req.Function,
// 		req.Args,
// 	)

//		json.NewEncoder(w).Encode(res)
//	}
//
// GET /token-balance?contract=0x...&holder=0x...
// func (bcs *BlockchainServer) GetTokenBalance(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	w.Header().Set("Access-Control-Allow-Origin", "*")
// 	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
// 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

// 	if r.Method == http.MethodOptions {
// 		w.WriteHeader(http.StatusOK)
// 		return
// 	}

// 	if r.Method != http.MethodGet {
// 		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
// 		return
// 	}

// 	q := r.URL.Query()
// 	contractAddr := q.Get("contract")
// 	holderAddr := q.Get("holder")

// 	if contractAddr == "" || holderAddr == "" {
// 		http.Error(w, `{"error":"contract and holder are required"}`, http.StatusBadRequest)
// 		return
// 	}

// 	// Call VM directly as a view (no gas, no tx)
// 	res, err := bcs.BlockchainPtr.VM.ExecuteContract(
// 		contractAddr,
// 		holderAddr,
// 		"balanceOf",
// 		[]string{holderAddr},
// 		0,
// 	)
// 	if err != nil {
// 		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
// 		return
// 	}

// 	// Parse numeric balance from res.Output (string -> uint64)
// 	var parsed uint64
// 	if res.Output != "" {
// 		if v, err := strconv.ParseUint(res.Output, 10, 64); err == nil {
// 			parsed = v
// 		}
// 	}

// 	resp := map[string]interface{}{
// 		"success":     res.Success,
// 		"contract":    contractAddr,
// 		"holder":      holderAddr,
// 		"raw_output":  res.Output,
// 		"balance":     parsed,
// 		"function":    "balanceOf",
// 		"vm_gas_used": res.GasUsed, // usually 0 for view
// 	}

//		json.NewEncoder(w).Encode(resp)
//	}
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

func (bcs *BlockchainServer) ContractDeploy(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	type Req struct {
		Type       string `json:"type"`
		Owner      string `json:"owner"`
		Code       string `json:"code"`
		PluginPath string `json:"plugin_path"`
	}

	var req Req
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
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

	// 3. Save code
	if req.Type == "plugin" {
		meta.PluginPath = req.PluginPath
	} else {
		meta.Code = []byte(req.Code)
	}

	// 4. Generate ABI
	var abi []byte
	var err error
	log.Println(err)
	switch req.Type {
	case "plugin":
		// ABI will be generated after plugin is loaded
		abi = []byte(`[]`)
	case "gocode":
		abi, err = blockchaincomponent.GenerateABIForBytecode(nil)
	case "dsl":
		abi, err = blockchaincomponent.GenerateABIForDSL()
	default:
		http.Error(w, "invalid contract type", 400)
		return
	}
	meta.ABI = abi

	// 5. Create initial state
	state := &blockchaincomponent.SmartContractState{
		Address:   addr,
		Balance:   0,
		Storage:   map[string]string{},
		IsActive:  true,
		CreatedAt: time.Now().Unix(),
	}

	// 6. Register
	if err := bcs.BlockchainPtr.ContractEngine.Registry.RegisterContract(meta, state); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	json.NewEncoder(w).Encode(map[string]any{
		"status":  "deployed",
		"address": addr,
	})
}

////////////////////////////////////////////////////////////////////////////////
// POST /contract/call
//
// BODY:
// {
//   "address": "0xContract",
//   "caller": "0xUser",
//   "fn": "transfer",
//   "args": ["0xABC", "100"]
// }
////////////////////////////////////////////////////////////////////////////////

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

////////////////////////////////////////////////////////////////////////////////
// GET /contract/state?address=0x123
////////////////////////////////////////////////////////////////////////////////

func (bcs *BlockchainServer) ContractState(w http.ResponseWriter, r *http.Request) {
	addr := r.URL.Query().Get("address")

	rec, err := bcs.BlockchainPtr.ContractEngine.Registry.LoadContract(addr)
	if err != nil {
		http.Error(w, err.Error(), 404)
		return
	}

	json.NewEncoder(w).Encode(rec)
}

////////////////////////////////////////////////////////////////////////////////
// GET /contract/abi?address=0x123
////////////////////////////////////////////////////////////////////////////////

func (bcs *BlockchainServer) ContractABI(w http.ResponseWriter, r *http.Request) {
	addr := r.URL.Query().Get("address")

	abi, err := bcs.BlockchainPtr.ContractEngine.Registry.LoadABI(addr)
	if err != nil {
		http.Error(w, err.Error(), 404)
		return
	}

	w.Write(abi)
}

////////////////////////////////////////////////////////////////////////////////
// GET /contract/events?address=0x123&block=55
////////////////////////////////////////////////////////////////////////////////

// Ethereum-style contract address: 0x + 40 hex
func GenerateContractAddress(owner string, nonce uint64) string {
	input := owner + ":" + strconv.FormatUint(nonce, 10)
	sum := sha256.Sum256([]byte(input))
	return "0x" + hex.EncodeToString(sum[:20])
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

////////////////////////////////////////////////////////////////////////////////
// END OF CONTRACT API
////////////////////////////////////////////////////////////////////////////////

// -----------------------------------------------------------------------------
// Dummy existing handlers (replace with your real ones)
// -----------------------------------------------------------------------------

func (bcs *BlockchainServer) GetBlockchain(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(bcs.BlockchainPtr)
}

func (bcs *BlockchainServer) MineBlock(w http.ResponseWriter, r *http.Request) {
	blk := bcs.BlockchainPtr.MineNewBlock()
	json.NewEncoder(w).Encode(blk)
}
func (b *BlockchainServer) Start() {
	portStr := fmt.Sprintf("%d", b.Port)
	//router := mux.NewRouter()

	// Define your routes
	http.HandleFunc("/", b.getBlockchain)
	http.HandleFunc("/balance", b.GetBalance)
	http.HandleFunc("/send_tx", b.sendTransaction)
	http.HandleFunc("/fetch_last_n_block", b.fetchNBlocks)
	http.HandleFunc("/account/{address}/nonce", b.GetAccountNonce)
	http.HandleFunc("/getheight", b.GetBlockchainHeight)
	http.HandleFunc("/validator/{address}", b.ValidatorStats)
	http.HandleFunc("/network", b.NetworkStats)
	http.HandleFunc("/faucet", b.Faucet)
	http.HandleFunc("/block/{id}", b.GetBlock)
	http.HandleFunc("/validators", b.GetValidators)
	http.HandleFunc("/transactions/recent", b.GetRecentTransactions)
	http.HandleFunc("/liquidity/lock", b.LiquidityLock)
	http.HandleFunc("/liquidity/unlock", b.LiquidityUnlock)
	http.HandleFunc("/liquidity", b.LiquidityView)
	http.HandleFunc("/rewards/recent", b.RewardsRecent)
	http.HandleFunc("/validator/new", b.AddValidatorFromPeer)
	http.HandleFunc("/validator/add", b.AddValidator)
	http.HandleFunc("/tx/", b.GetTransactionByHash)
	http.HandleFunc("/contract/deploy", b.ContractDeploy)
	http.HandleFunc("/contract/call", b.ContractCall)
	http.HandleFunc("/contract/getAbi", b.ContractABI)
	http.HandleFunc("/contract/con1", b.ContractState)
	http.HandleFunc("/contract/con2", b.ContractEvents)
	http.HandleFunc("/address/{address}/transactions", b.GetAddressTransactions)

	//http.HandleFunc("/token-balance", b.GetTokenBalance)
	http.HandleFunc("/blocktime/latest", b.BlockTimeLatest)

	log.Println("Blockchain server is starting on port:", b.Port)

	// Use the CORS handler
	err := http.ListenAndServe("127.0.0.1:"+portStr, nil)
	if err != nil {
		log.Fatalf("Failed to start blockchain server: %v", err)
	}
	log.Println("Blockchain server started successfully")
}
