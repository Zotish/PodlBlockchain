package blockchainserver

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"

	blockchaincomponent "github.com/Zotish/DefenceProject/BlockchainComponent"
	wallet "github.com/Zotish/DefenceProject/WalletComponent"
	"github.com/gorilla/mux"

	//"github.com/rs/cors"
	"github.com/gorilla/handlers"
)

type BlockchainServer struct {
	Port          uint                                   `json:"port"`
	BlockchainPtr *blockchaincomponent.Blockchain_struct `json:"blockchain_ptr"`
}

func NewBlockchainServer(port uint, blockchainPtr *blockchaincomponent.Blockchain_struct) *BlockchainServer {
	return &BlockchainServer{
		Port:          port,
		BlockchainPtr: blockchainPtr,
	}
}

func (b *BlockchainServer) getBlockchain(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == http.MethodGet {
		io.WriteString(w, b.BlockchainPtr.ToJsonChain())
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
}

func (bcs *BlockchainServer) GetAccountNonce(w http.ResponseWriter, r *http.Request) {
	address := mux.Vars(r)["address"]
	nonce := bcs.BlockchainPtr.GetAccountNonce(address)

	json.NewEncoder(w).Encode(map[string]uint64{
		"nonce": nonce,
	})
}
func (b *BlockchainServer) sendTransaction(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
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
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

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
	height := uint64(len(bcs.BlockchainPtr.Blocks))
	json.NewEncoder(w).Encode(map[string]uint64{"height": height})
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
func (bcs *BlockchainServer) GetBalance(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*") // Allow all origins (or restrict to 8080)
	w.Header().Set("Content-Type", "application/json")
	address := r.URL.Query().Get("address")
	balance := bcs.BlockchainPtr.Accounts[address]
	json.NewEncoder(w).Encode(map[string]interface{}{"balance": balance})
}

// blockchain_server.go
func (bcs *BlockchainServer) Faucet(w http.ResponseWriter, r *http.Request) {
	address := r.URL.Query().Get("address")
	if !wallet.ValidateAddress(address) {
		http.Error(w, "Invalid address", http.StatusBadRequest)
		return
	}
	tx := blockchaincomponent.NewTransaction("0xFaucet", address, 100000, []byte{}, bcs.BlockchainPtr.GetAccountNonce("0xFaucet"))
	bcs.BlockchainPtr.AddNewTxToTheTransaction_pool(tx)
}

func (bcs *BlockchainServer) ValidatorStats(w http.ResponseWriter, r *http.Request) {
	address := mux.Vars(r)["address"]
	stats := bcs.BlockchainPtr.GetValidatorStats(address)
	if stats == nil {
		http.Error(w, "validator not found", http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(stats)
}

func (bcs *BlockchainServer) NetworkStats(w http.ResponseWriter, r *http.Request) {
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

// func corsMiddleware(next http.Handler) http.Handler {
// 	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		w.Header().Set("Access-Control-Allow-Origin", "*")
// 		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
// 		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
// 		if r.Method == "OPTIONS" {
// 			w.WriteHeader(http.StatusOK)
// 			return
// 		}
// 		next.ServeHTTP(w, r)
// 	})
// }

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

// 	log.Println("Blockchain server is starting on port:", b.Port)
// 	err := http.ListenAndServe("127.0.0.1:"+portStr, handlers.CORS(

//			handlers.AllowedOrigins([]string{"http://localhost:3000"}),
//			handlers.AllowedMethods([]string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}),
//			handlers.AllowedHeaders([]string{"X-Requested-With", "Content-Type", "Authorization"}),
//		)(router))
//		if err != nil {
//			log.Fatalf("Failed to start blockchain server: %v", err)
//		}
//		log.Println("Blockchain server started successfully")
//	}
//
// Deploy a contract
// Add these endpoints to your server
func (bcs *BlockchainServer) DeployContract(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Code  string `json:"code"`
		Value uint64 `json:"value"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	address, err := bcs.BlockchainPtr.VM.DeployContract(request.Code, "sender", request.Value)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"contract_address": address,
	})
}

func (bcs *BlockchainServer) CallContract(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Address  string   `json:"address"`
		Function string   `json:"function"`
		Args     []string `json:"args"`
		Value    uint64   `json:"value"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	result, err := bcs.BlockchainPtr.VM.ExecuteContract(request.Address, "caller", request.Function, request.Args, request.Value)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(result)
}
func (b *BlockchainServer) Start() {
	portStr := fmt.Sprintf("%d", b.Port)
	router := mux.NewRouter()

	// Configure CORS properly
	corsHandler := handlers.CORS(
		handlers.AllowedOrigins([]string{"http://localhost:3000", "http://127.0.0.1:3000"}),
		handlers.AllowedMethods([]string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}),
		handlers.AllowedHeaders([]string{"X-Requested-With", "Content-Type", "Authorization", "Accept"}),
		handlers.AllowCredentials(),
	)

	// Define your routes
	router.HandleFunc("/", b.getBlockchain).Methods("GET")
	router.HandleFunc("/balance", b.GetBalance).Methods("GET")
	router.HandleFunc("/send_tx", b.sendTransaction).Methods("POST")
	router.HandleFunc("/fetch_last_n_block", b.fetchNBlocks).Methods("GET")
	router.HandleFunc("/account/{address}/nonce", b.GetAccountNonce).Methods("GET")
	router.HandleFunc("/getheight", b.GetBlockchainHeight).Methods("GET")
	router.HandleFunc("/validator/{address}", b.ValidatorStats).Methods("GET")
	router.HandleFunc("/network", b.NetworkStats).Methods("GET")
	router.HandleFunc("/faucet", b.Faucet).Methods("POST")
	router.HandleFunc("/block/{id}", b.GetBlock).Methods("GET")

	// Add OPTIONS handler for preflight requests
	router.Methods("OPTIONS").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.WriteHeader(http.StatusOK)
	})

	log.Println("Blockchain server is starting on port:", b.Port)

	// Use the CORS handler
	err := http.ListenAndServe("127.0.0.1:"+portStr, corsHandler(router))
	if err != nil {
		log.Fatalf("Failed to start blockchain server: %v", err)
	}
	log.Println("Blockchain server started successfully")
}
