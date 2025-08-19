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
	"github.com/rs/cors"
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

func (b *BlockchainServer) fetchNBlocks(w http.ResponseWriter, r *http.Request) {
	//w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
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

func enableCORS1(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set headers
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		//Access to fetch at 'http://localhost:5000/fetch_last_n_block?n=10' from origin 'http://localhost:3000' has been blocked by CORS policy: No 'Access-Control-Allow-Origin' header is present on the requested resource.
		w.Header().Set("Access-Control-Allow-Origin", "*")                     // Allow all origins (or restrict to specific origin)
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000") // Allow specific origin
		w.Header().Set("Access-Control-Allow-Credentials", "true")             // Allow credentials
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		w.Header().Set("Content-Type", "application/json")
		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (b *BlockchainServer) Start() {
	portStr := fmt.Sprintf("%d", b.Port)
	router := mux.NewRouter()
	//router.Use(enableCORS)

	http.HandleFunc("/", b.getBlockchain)
	http.HandleFunc("/balance", b.GetBalance)
	http.HandleFunc("/send_tx", b.sendTransaction)
	http.HandleFunc("/fetch_last_n_block", b.fetchNBlocks)
	http.HandleFunc("/account/{address}/nonce", b.GetAccountNonce)
	http.HandleFunc("/getheight", b.GetBlockchainHeight)
	http.HandleFunc("/validator/{address}", b.ValidatorStats)
	http.HandleFunc("/network", b.NetworkStats)
	http.HandleFunc("/faucet", b.Faucet)

	c := cors.New(cors.Options{
		// The `AllowOrigin` field is crucial. It tells the browser that requests
		// from 'http://localhost:3000' are allowed.
		AllowedOrigins: []string{"http://localhost:3000"},
		// You can also specify the allowed methods, headers, etc.
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
		// Setting AllowCredentials to true is important if your frontend
		// sends cookies or authorization headers.
		AllowCredentials: true,
	})

	handler := c.Handler(router)

	log.Println("Blockchain server is starting on port:", b.Port)
	err := http.ListenAndServe(":"+portStr, handler)
	if err != nil {
		log.Fatalf("Failed to start blockchain server: %v", err)
	}
	log.Println("Blockchain server started successfully")
}

// func (b *BlockchainServer) Start() {
// 	portStr := fmt.Sprintf("%d", b.Port)

// 	// Then wrap your router:
// 	router := mux.NewRouter()
// 	router.Use(enableCORS)

// 	router.HandleFunc("/", b.getBlockchain)
// 	router.HandleFunc("/balance", b.GetBalance)
// 	router.HandleFunc("/send_tx", b.sendTransaction)
// 	router.HandleFunc("/fetch_last_n_block", b.fetchNBlocks)
// 	router.HandleFunc("/account/{address}/nonce", b.GetAccountNonce)
// 	router.HandleFunc("/getheight", b.GetBlockchainHeight)
// 	router.HandleFunc("/validator/{address}", b.ValidatorStats)
// 	//router.HandleFunc("/network", b.NetworkStats)
// 	//router.HandleFunc("/faucet", b.Faucet)
// 	// Add new endpoints
// 	router.HandleFunc("/block/{id}", b.GetBlock)
// 	// router.HandleFunc("/tx/{hash}", b.GetTransaction)
// 	// router.HandleFunc("/blocks", b.GetBlocks)
// 	// router.HandleFunc("/transactions", b.GetTransactions)
// 	// router.HandleFunc("/address/{address}/transactions", b.GetAddressTransactions)

// 	log.Println("Blockchain server is starting on port:", b.Port)
// 	err := http.ListenAndServe("127.0.0.1:"+portStr, nil)
// 	if err != nil {
// 		log.Fatalf("Failed to start blockchain server: %v", err)
// 	}
// }
