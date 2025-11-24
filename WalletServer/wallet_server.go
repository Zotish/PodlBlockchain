// package walletserver

// import (
// 	"bytes"
// 	"encoding/hex"
// 	"encoding/json"
// 	"fmt"
// 	"io"
// 	"log"
// 	"net/http"
// 	"net/url"
// 	"os"
// 	"strings"
// 	"time"

// 	blockchaincomponent "github.com/Zotish/DefenceProject/BlockchainComponent"
// 	constantset "github.com/Zotish/DefenceProject/ConstantSet"
// 	wallet "github.com/Zotish/DefenceProject/WalletComponent"
// )

// var (
// 	allowedOrigins = []string{
// 		"http://localhost:3000",
// 		"http://frontend:3000", // Add internal Docker network address
// 	}
// )

// func init() {
// 	// Load from environment
// 	if envOrigins := os.Getenv("CORS_ALLOWED_ORIGINS"); envOrigins != "" {
// 		allowedOrigins = strings.Split(envOrigins, ",")
// 	}
// }

// type WalletServer struct {
// 	Port                  uint64
// 	BlockchainNodeAddress string
// }

// func NewWalletServer(port uint64, blockchainNodeAddress string) *WalletServer {
// 	return &WalletServer{
// 		Port:                  port,
// 		BlockchainNodeAddress: blockchainNodeAddress,
// 	}
// }

// func (ws *WalletServer) CreateNewWallet(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
// 	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
// 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

// 	if r.Method == http.MethodOptions {
// 		w.WriteHeader(http.StatusOK)
// 		return
// 	}

// 	if r.Method != http.MethodPost {
// 		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// 		return
// 	}

// 	// Read password from request body
// 	body, err := io.ReadAll(r.Body)
// 	if err != nil {
// 		http.Error(w, "Failed to read request body", http.StatusBadRequest)
// 		return
// 	}
// 	defer r.Body.Close()

// 	var request struct {
// 		Password string `json:"password"`
// 	}
// 	if err := json.Unmarshal(body, &request); err != nil {
// 		http.Error(w, "Invalid request format", http.StatusBadRequest)
// 		return
// 	}

// 	// Create new wallet
// 	newWallet, err := wallet.NewWallet(request.Password)
// 	if err != nil {
// 		http.Error(w, fmt.Sprintf("Failed to create wallet: %v", err), http.StatusInternalServerError)
// 		return
// 	}

// 	// Prepare response
// 	response := struct {
// 		Address    string `json:"address"`
// 		PrivateKey string `json:"private_key"`
// 		Mnemonic   string `json:"mnemonic"`
// 	}{
// 		Address:    newWallet.Address,
// 		PrivateKey: newWallet.GetPrivateKeyHex(),
// 		Mnemonic:   newWallet.Mnemonic,
// 	}
// 	// if isValidatorWallet(newWallet.Address) { // You'll need to implement this check
// 	// 	blockchain.RegisterValidatorWallet(newWallet.Address, newWallet)
// 	// }
// 	jsonResponse, err := json.Marshal(response)
// 	if err != nil {
// 		http.Error(w, "Failed to marshal response", http.StatusInternalServerError)
// 		return
// 	}

// 	w.WriteHeader(http.StatusCreated)
// 	w.Write(jsonResponse)
// }

// func (ws *WalletServer) ImportFromMnemonic(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")

// 	if r.Method != http.MethodPost {
// 		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// 		return
// 	}

// 	var request struct {
// 		Mnemonic string `json:"mnemonic"`
// 		Password string `json:"password"`
// 	}

// 	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
// 		http.Error(w, "Invalid request format", http.StatusBadRequest)
// 		return
// 	}

// 	importedWallet, err := wallet.ImportFromMnemonic(request.Mnemonic, request.Password)
// 	if err != nil {
// 		http.Error(w, fmt.Sprintf("Failed to import wallet: %v", err), http.StatusBadRequest)
// 		return
// 	}

// 	response := struct {
// 		Address    string `json:"address"`
// 		PrivateKey string `json:"private_key"`
// 	}{
// 		Address:    importedWallet.Address,
// 		PrivateKey: importedWallet.GetPrivateKeyHex(),
// 	}

// 	json.NewEncoder(w).Encode(response)
// }

// func (ws *WalletServer) ImportFromPrivateKey(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
// 	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
// 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

// 	if r.Method == http.MethodOptions {
// 		w.WriteHeader(http.StatusOK)
// 		return
// 	}
// 	if r.Method != http.MethodPost {
// 		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// 		return
// 	}

// 	var request struct {
// 		PrivateKey string `json:"private_key"`
// 	}

// 	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
// 		http.Error(w, "Invalid request format", http.StatusBadRequest)
// 		return
// 	}

// 	importedWallet, err := wallet.ImportFromPrivateKey(request.PrivateKey)
// 	if err != nil {
// 		http.Error(w, fmt.Sprintf("Failed to import wallet: %v", err), http.StatusBadRequest)
// 		return
// 	}

// 	response := struct {
// 		Address string `json:"address"`
// 	}{
// 		Address: importedWallet.Address,
// 	}

// 	json.NewEncoder(w).Encode(response)
// }

// // func (ws *WalletServer) ImportFromPrivateKey(w http.ResponseWriter, r *http.Request) {
// // 	w.Header().Set("Content-Type", "application/json")
// // 	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
// // 	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
// // 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

// // 	if r.Method != http.MethodPost {
// // 		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// // 		return
// // 	}

// // 	var request struct {
// // 		PrivateKey string `json:"private_key"`
// // 	}

// // 	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
// // 		http.Error(w, "Invalid request format", http.StatusBadRequest)
// // 		return
// // 	}

// // 	importedWallet, err := wallet.ImportFromPrivateKey(request.PrivateKey)
// // 	if err != nil {
// // 		http.Error(w, fmt.Sprintf("Failed to import wallet: %v", err), http.StatusBadRequest)
// // 		return
// // 	}

// // 	response := struct {
// // 		Address string `json:"address"`
// // 	}{
// // 		Address: importedWallet.Address,
// // 	}

// // 	json.NewEncoder(w).Encode(response)
// // }

// func (ws *WalletServer) GetBalance(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
// 	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
// 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
// 	if r.Method == http.MethodOptions {
// 		w.WriteHeader(http.StatusOK)
// 		return
// 	}

// 	w.Header().Set("Content-Type", "application/json")

// 	if r.Method != http.MethodGet {
// 		http.Error(w, `{"error": "Method not allowed"}`, http.StatusMethodNotAllowed)
// 		return
// 	}

// 	address := r.URL.Query().Get("address")
// 	if address == "" {
// 		http.Error(w, `{"error": "Address is required"}`, http.StatusBadRequest)
// 		return
// 	}

// 	// Validate address format
// 	if !wallet.ValidateAddress(address) {
// 		http.Error(w, `{"error": "Invalid address format"}`, http.StatusBadRequest)
// 		return
// 	}

// 	client := &http.Client{Timeout: 10 * time.Second}
// 	resp, err := client.Get(fmt.Sprintf("%s/balance?address=%s", ws.BlockchainNodeAddress, url.QueryEscape(address)))
// 	if err != nil {
// 		http.Error(w, `{"error": "Blockchain node unreachable"}`, http.StatusBadGateway)
// 		return
// 	}
// 	defer resp.Body.Close()

// 	w.WriteHeader(resp.StatusCode)
// 	io.Copy(w, resp.Body)
// }

// // func (ws *WalletServer) GetBalance(w http.ResponseWriter, r *http.Request) {
// // 	w.Header().Set("Access-Control-Allow-Origin", "*") // Enable CORS
// // 	w.Header().Set("Content-Type", "application/json")

// // 	if r.Method != http.MethodGet {
// // 		http.Error(w, `{"error": "Method not allowed"}`, http.StatusMethodNotAllowed)
// // 		return
// // 	}

// // 	address := r.URL.Query().Get("address")
// // 	if address == "" {
// // 		http.Error(w, `{"error": "Address is required"}`, http.StatusBadRequest)
// // 		return
// // 	}

// // 	url := fmt.Sprintf("%s/balance?address=%s", ws.BlockchainNodeAddress, address)
// // 	resp, err := http.Get(url)
// // 	if err != nil {
// // 		http.Error(w, `{"error": "Blockchain node unreachable"}`, http.StatusBadGateway)
// // 		return
// // 	}
// // 	defer resp.Body.Close()

// // 	if resp.StatusCode != http.StatusOK {
// // 		body, _ := io.ReadAll(resp.Body)
// // 		http.Error(w, string(body), resp.StatusCode)
// // 		return
// // 	}

// // 	io.Copy(w, resp.Body)
// // }

// // func (ws *WalletServer) GetBalance(w http.ResponseWriter, r *http.Request) {
// // 	w.Header().Set("Content-Type", "application/json")

// // 	if r.Method == http.MethodGet {
// // 		params := url.Values{}
// // 		params.Add("address", r.URL.Query().Get("address"))
// // 		//address := r.URL.Query().Get("address")
// // 		ourUrl := fmt.Sprintf("%s/balance?%s", ws.BlockchainNodeAddress, params.Encode())
// // 		resp, err := http.Get(ourUrl)
// // 		if err != nil {
// // 			http.Error(w, err.Error(), http.StatusBadRequest)
// // 			return
// // 		}
// // 		// if address == "" {
// // 		// 	http.Error(w, `{"error": "Address is required"}`, http.StatusBadRequest)
// // 		// 	return
// // 		// }

// // 		// // Validate address format
// // 		// if !strings.HasPrefix(address, "0x") || len(address) != 42 {
// // 		// 	http.Error(w, `{"error": "Invalid address format"}`, http.StatusBadRequest)
// // 		// 	return
// // 		// }

// // 		defer resp.Body.Close()
// // 		data, err := io.ReadAll(resp.Body)
// // 		if err != nil {
// // 			http.Error(w, err.Error(), http.StatusBadRequest)
// // 			return
// // 		}
// // 		w.Write(data)

// // 	} else {
// // 		http.Error(w, "Invalid Method", http.StatusBadRequest)
// // 	}

// // }

// // func (ws *WalletServer) GetBalance(w http.ResponseWriter, r *http.Request) {
// // 	w.Header().Set("Access-Control-Allow-Origin", "*")
// // 	w.Header().Set("Content-Type", "application/json")

// // 	if r.Method != http.MethodGet {
// // 		http.Error(w, `{"error": "Method not allowed"}`, http.StatusMethodNotAllowed)
// // 		return
// // 	}

// // 	address := r.URL.Query().Get("address")
// // 	if address == "" || !wallet.ValidateAddress(address) {
// // 		http.Error(w, `{"error": "Valid address is required"}`, http.StatusBadRequest)
// // 		return
// // 	}

// // 	resp, err := http.Get(fmt.Sprintf("%s/balance?address=%s", ws.BlockchainNodeAddress, address))
// // 	if err != nil {
// // 		http.Error(w, `{"error": "Blockchain node unreachable"}`, http.StatusBadGateway)
// // 		return
// // 	}
// // 	defer resp.Body.Close()

// // 	if resp.StatusCode != http.StatusOK {
// // 		body, _ := io.ReadAll(resp.Body)
// // 		http.Error(w, string(body), resp.StatusCode)
// // 		return
// // 	}

// // 	// Forward the response
// // 	w.WriteHeader(resp.StatusCode)
// // 	io.Copy(w, resp.Body)
// // }

// // func (ws *WalletServer) GetBalance(w http.ResponseWriter, r *http.Request) {
// // 	w.Header().Set("Access-Control-Allow-Origin", "*") // CORS
// // 	w.Header().Set("Content-Type", "application/json")

// // 	if r.Method != http.MethodGet {
// // 		http.Error(w, `{"error": "Invalid request method"}`, http.StatusMethodNotAllowed)
// // 		return
// // 	}

// // 	address := r.URL.Query().Get("address")
// // 	if address == "" {
// // 		http.Error(w, `{"error": "Address is required"}`, http.StatusBadRequest)
// // 		return
// // 	}

// // 	// Forward the request to the blockchain node
// // 	url := fmt.Sprintf("%s/balance?address=%s", ws.BlockchainNodeAddress, address)
// // 	resp, err := http.Get(url)
// // 	if err != nil {
// // 		http.Error(w, `{"error": "Blockchain node unreachable"}`, http.StatusBadGateway)
// // 		return
// // 	}
// // 	defer resp.Body.Close()

// // 	// Proxy the response back to the client
// // 	w.WriteHeader(resp.StatusCode)
// // 	io.Copy(w, resp.Body)
// // }

// // func (ws *WalletServer) GetBalance(w http.ResponseWriter, r *http.Request) {
// // 	w.Header().Set("Content-Type", "application/json")

// // 	if r.Method != http.MethodGet {
// // 		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// // 		return
// // 	}

// // 	address := r.URL.Query().Get("address")
// // 	if address == "" {
// // 		http.Error(w, "Address parameter is required", http.StatusBadRequest)
// // 		return
// // 	}

// // 	// Forward request to blockchain node
// // 	queryParams := url.Values{}
// // 	queryParams.Add("address", address)
// // 	url := fmt.Sprintf("%s/balance?%s", ws.BlockchainNodeAddress, queryParams.Encode())

// // 	resp, err := http.Get(url)
// // 	if err != nil {
// // 		http.Error(w, fmt.Sprintf("Failed to query blockchain: %v", err), http.StatusInternalServerError)
// // 		return
// // 	}
// // 	defer resp.Body.Close()

// // 	if resp.StatusCode != http.StatusOK {
// // 		body, _ := io.ReadAll(resp.Body)
// // 		http.Error(w, string(body), resp.StatusCode)
// // 		return
// // 	}

// // 	// Stream the response directly to the client
// // 	io.Copy(w, resp.Body)
// // }

// func (ws *WalletServer) SendTransaction(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
// 	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
// 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
// 	if r.Method == http.MethodOptions {
// 		w.WriteHeader(http.StatusOK)
// 		return
// 	}
// 	if r.Method != http.MethodPost {
// 		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// 		return
// 	}

// 	// Accept data as string for safety; we'll decode to bytes ourselves.
// 	var request struct {
// 		From       string `json:"from"`
// 		To         string `json:"to"`
// 		Value      uint64 `json:"value"`
// 		Data       string `json:"data"` // <— string now
// 		Gas        uint64 `json:"gas"`
// 		GasPrice   uint64 `json:"gas_price"`
// 		PrivateKey string `json:"private_key"`
// 	}
// 	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
// 		http.Error(w, `{"error":"Invalid request format"}`, http.StatusBadRequest)
// 		return
// 	}

// 	// 1) Validate addresses
// 	if !wallet.ValidateAddress(request.From) || !wallet.ValidateAddress(request.To) {
// 		http.Error(w, `{"error":"Invalid address format"}`, http.StatusBadRequest)
// 		return
// 	}

// 	// 2) Load signer and enforce From == signer address
// 	signer, err := wallet.ImportFromPrivateKey(request.PrivateKey)
// 	if err != nil {
// 		http.Error(w, fmt.Sprintf(`{"error":"Failed to import wallet: %v"}`, err), http.StatusBadRequest)
// 		return
// 	}
// 	if !strings.EqualFold(signer.Address, request.From) {
// 		http.Error(w, `{"error":"From address does not match private key"}`, http.StatusBadRequest)
// 		return
// 	}

// 	// 3) Decode data string → []byte
// 	var dataBytes []byte
// 	if request.Data != "" {
// 		if strings.HasPrefix(request.Data, "0x") || strings.HasPrefix(request.Data, "0X") {
// 			// hex
// 			db, derr := hex.DecodeString(request.Data[2:])
// 			if derr != nil {
// 				http.Error(w, `{"error":"Invalid hex in 'data'"}`, http.StatusBadRequest)
// 				return
// 			}
// 			dataBytes = db
// 		} else {
// 			// treat as UTF-8 text
// 			dataBytes = []byte(request.Data)
// 		}
// 	}

// 	client := &http.Client{Timeout: 10 * time.Second}

// 	// 4) Get NONCE (next usable)
// 	// nonceURL := fmt.Sprintf("%s/account/%s/nonce", ws.BlockchainNodeAddress, request.From)
// 	// resp, err := client.Get(nonceURL)
// 	// if err != nil {
// 	// 	http.Error(w, `{"error":"Failed to get nonce from blockchain"}`, http.StatusBadGateway)
// 	// 	return
// 	// }
// 	// defer resp.Body.Close()
// 	// if resp.StatusCode != http.StatusOK {
// 	// 	body, _ := io.ReadAll(resp.Body)
// 	// 	http.Error(w, string(body), resp.StatusCode)
// 	// 	return
// 	// }
// 	// var nonceResp struct {
// 	// 	Nonce          uint64 `json:"nonce"`
// 	// 	NextNonce      uint64 `json:"next_nonce"`
// 	// 	ConfirmedNonce uint64 `json:"confirmed_nonce"`
// 	// }
// 	// if err := json.NewDecoder(resp.Body).Decode(&nonceResp); err != nil {
// 	// 	http.Error(w, `{"error":"Failed to decode nonce response"}`, http.StatusInternalServerError)
// 	// 	return
// 	// }

// 	// // Prefer next_nonce; fallback to nonce (which our node also sets to next)
// 	// txNonce := nonceResp.NextNonce
// 	// if txNonce == 0 {
// 	// 	txNonce = nonceResp.Nonce
// 	// }

// 	// 4) Get NONCE (if your endpoint returns CURRENT nonce, uncomment the ++)
// 	// nonceURL := fmt.Sprintf("%s/account/%s/nonce", ws.BlockchainNodeAddress, request.From)
// 	// resp, err := client.Get(nonceURL)
// 	// if err != nil {
// 	// 	http.Error(w, `{"error":"Failed to get nonce from blockchain"}`, http.StatusBadGateway)
// 	// 	return
// 	// }
// 	// defer resp.Body.Close()
// 	// if resp.StatusCode != http.StatusOK {
// 	// 	body, _ := io.ReadAll(resp.Body)
// 	// 	http.Error(w, string(body), resp.StatusCode)
// 	// 	return
// 	// }
// 	// var nonceResp struct {
// 	// 	Nonce uint64 `json:"nonce"`
// 	// }
// 	// if err := json.NewDecoder(resp.Body).Decode(&nonceResp); err != nil {
// 	// 	http.Error(w, `{"error":"Failed to decode nonce response"}`, http.StatusInternalServerError)
// 	// 	return
// 	// }
// 	// If your nonce API returns CURRENT nonce, use the next:
// 	//nonceResp.Nonce++

// 	// 5) Enforce sane gas & price (avoid underpriced txs)
// 	minBaseFee := uint64(constantset.InitialBaseFee) // your chain’s base fee constant
// 	gas := request.Gas
// 	if gas == 0 {
// 		gas = uint64(constantset.MinGas) // e.g., 21000
// 	}
// 	gasPrice := request.GasPrice
// 	if gasPrice < minBaseFee {
// 		gasPrice = minBaseFee
// 	}
// 	// (Optional) add a small tip if your chain needs it:
// 	// gasPrice += uint64(constantset.DefaultPriorityFee)

// 	// 6) Check balance is enough to pay value + fee before signing
// 	balResp, err := client.Get(fmt.Sprintf("%s/balance?address=%s", ws.BlockchainNodeAddress, url.QueryEscape(request.From)))
// 	if err != nil {
// 		http.Error(w, `{"error":"Failed to get balance from blockchain"}`, http.StatusBadGateway)
// 		return
// 	}
// 	defer balResp.Body.Close()
// 	if balResp.StatusCode != http.StatusOK {
// 		body, _ := io.ReadAll(balResp.Body)
// 		http.Error(w, string(body), balResp.StatusCode)
// 		return
// 	}
// 	var bal struct {
// 		Balance uint64 `json:"balance"`
// 	}
// 	if err := json.NewDecoder(balResp.Body).Decode(&bal); err != nil {
// 		http.Error(w, `{"error":"Failed to decode balance response"}`, http.StatusInternalServerError)
// 		return
// 	}
// 	fee := gas * gasPrice
// 	total := request.Value + fee
// 	if bal.Balance < total {
// 		http.Error(w, fmt.Sprintf(`{"error":"Insufficient funds: balance=%d required=%d (value %d + fee %d)"}`, bal.Balance, total, request.Value, fee), http.StatusBadRequest)
// 		return
// 	}

// 	// 7) Build tx
// 	tx := blockchaincomponent.NewTransaction(
// 		request.From,
// 		request.To,
// 		request.Value,
// 		dataBytes,
// 		//nonceResp.Nonce,
// 	)
// 	tx.Gas = gas
// 	tx.GasPrice = gasPrice
// 	tx.ChainID = uint64(constantset.ChainID)

// 	// 8) Sign
// 	if err := signer.SignTransaction(tx); err != nil {
// 		http.Error(w, fmt.Sprintf(`{"error":"Failed to sign transaction: %v"}`, err), http.StatusInternalServerError)
// 		return
// 	}

// 	// 9) POST to node
// 	txJSON, err := json.Marshal(tx)
// 	if err != nil {
// 		http.Error(w, `{"error":"Failed to marshal transaction"}`, http.StatusInternalServerError)
// 		return
// 	}
// 	resp, err := client.Post(ws.BlockchainNodeAddress+"/send_tx", "application/json", bytes.NewBuffer(txJSON))
// 	if err != nil {
// 		http.Error(w, fmt.Sprintf(`{"error":"Failed to send transaction: %v"}`, err), http.StatusBadGateway)
// 		return
// 	}
// 	defer resp.Body.Close()

// 	w.WriteHeader(resp.StatusCode)
// 	io.Copy(w, resp.Body)
// }

// //this is last changes i did nov 6th
// // func (ws *WalletServer) SendTransaction(w http.ResponseWriter, r *http.Request) {
// // 	w.Header().Set("Content-Type", "application/json")
// // 	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
// // 	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
// // 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
// // 	if r.Method == http.MethodOptions {
// // 		w.WriteHeader(http.StatusOK)
// // 		return
// // 	}

// // 	w.Header().Set("Content-Type", "application/json")

// // 	if r.Method != http.MethodPost {
// // 		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// // 		return
// // 	}

// // 	var request struct {
// // 		From       string `json:"from"`
// // 		To         string `json:"to"`
// // 		Value      uint64 `json:"value"`
// // 		Data       []byte `json:"data"`
// // 		Gas        uint64 `json:"gas"`
// // 		GasPrice   uint64 `json:"gas_price"`
// // 		PrivateKey string `json:"private_key"`
// // 	}

// // 	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
// // 		http.Error(w, `{"error": "Invalid request format"}`, http.StatusBadRequest)
// // 		return
// // 	}

// // 	// Validate addresses
// // 	if !wallet.ValidateAddress(request.From) || !wallet.ValidateAddress(request.To) {
// // 		http.Error(w, `{"error": "Invalid address format"}`, http.StatusBadRequest)
// // 		return
// // 	}

// // 	// Import wallet
// // 	wallet, err := wallet.ImportFromPrivateKey(request.PrivateKey)
// // 	if err != nil {
// // 		http.Error(w, fmt.Sprintf(`{"error": "Failed to import wallet: %v"}`, err), http.StatusBadRequest)
// // 		return
// // 	}

// // 	// Get nonce from blockchain node
// // 	client := &http.Client{Timeout: 10 * time.Second}
// // 	resp, err := client.Get(fmt.Sprintf("%s/account/%s/nonce", ws.BlockchainNodeAddress, request.From))
// // 	if err != nil {
// // 		http.Error(w, `{"error": "Failed to get nonce from blockchain"}`, http.StatusBadGateway)
// // 		return
// // 	}
// // 	defer resp.Body.Close()

// // 	if resp.StatusCode != http.StatusOK {
// // 		body, _ := io.ReadAll(resp.Body)
// // 		http.Error(w, string(body), resp.StatusCode)
// // 		return
// // 	}

// // 	var nonceResp struct {
// // 		Nonce uint64 `json:"nonce"`
// // 	}
// // 	if err := json.NewDecoder(resp.Body).Decode(&nonceResp); err != nil {
// // 		http.Error(w, `{"error": "Failed to decode nonce response"}`, http.StatusInternalServerError)
// // 		return
// // 	}

// // 	// Create and sign transaction
// // 	tx := blockchaincomponent.NewTransaction(
// // 		request.From,
// // 		request.To,
// // 		request.Value,
// // 		request.Data,
// // 		nonceResp.Nonce,
// // 	)
// // 	tx.Gas = request.Gas
// // 	tx.GasPrice = request.GasPrice

// // 	if err := wallet.SignTransaction(tx); err != nil {
// // 		http.Error(w, fmt.Sprintf(`{"error": "Failed to sign transaction: %v"}`, err), http.StatusInternalServerError)
// // 		return
// // 	}

// // 	// Send to blockchain node
// // 	txJSON, err := json.Marshal(tx)
// // 	if err != nil {
// // 		http.Error(w, `{"error": "Failed to marshal transaction"}`, http.StatusInternalServerError)
// // 		return
// // 	}

// // 	resp, err = client.Post(
// // 		ws.BlockchainNodeAddress+"/send_tx",
// // 		"application/json",
// // 		bytes.NewBuffer(txJSON),
// // 	)
// // 	if err != nil {
// // 		http.Error(w, fmt.Sprintf(`{"error": "Failed to send transaction: %v"}`, err), http.StatusBadGateway)
// // 		return
// // 	}
// // 	defer resp.Body.Close()

// // 	w.WriteHeader(resp.StatusCode)
// // 	io.Copy(w, resp.Body)
// // }

// // func (ws *WalletServer) SendTransaction(w http.ResponseWriter, r *http.Request) {
// // 	w.Header().Set("Content-Type", "application/json")
// // 	w.Header().Set("Access-Control-Allow-Origin", "*")

// // 	if r.Method != http.MethodPost {
// // 		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// // 		return
// // 	}

// // 	var request struct {
// // 		From       string `json:"from"`
// // 		To         string `json:"to"`
// // 		Value      uint64 `json:"value"`
// // 		Data       []byte `json:"data"`
// // 		Gas        uint64 `json:"gas"`
// // 		GasPrice   uint64 `json:"gas_price"`
// // 		PrivateKey string `json:"private_key"`
// // 	}

// // 	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
// // 		http.Error(w, "Invalid request format", http.StatusBadRequest)
// // 		return
// // 	}

// // 	// Import wallet from private key
// // 	wallet, err := wallet.ImportFromPrivateKey(request.PrivateKey)
// // 	if err != nil {
// // 		http.Error(w, fmt.Sprintf("Failed to import wallet: %v", err), http.StatusBadRequest)
// // 		return
// // 	}

// // 	// Get the current nonce from the blockchain node
// // 	nonce, err := ws.getAccountNonce(request.From)
// // 	if err != nil {
// // 		http.Error(w, fmt.Sprintf("Failed to get account nonce: %v", err), http.StatusInternalServerError)
// // 		return
// // 	}

// // 	// Create transaction
// // 	tx := blockchaincomponent.NewTransaction(
// // 		request.From,
// // 		request.To,
// // 		request.Value,
// // 		request.Data,
// // 		nonce+1, // Use the next nonce
// // 	)
// // 	tx.Gas = request.Gas
// // 	tx.GasPrice = request.GasPrice

// // 	// Sign transaction
// // 	if err := wallet.SignTransaction(tx); err != nil {
// // 		http.Error(w, fmt.Sprintf("Failed to sign transaction: %v", err), http.StatusInternalServerError)
// // 		return
// // 	}

// // 	// Send to blockchain node
// // 	txJSON, err := json.Marshal(tx)
// // 	if err != nil {
// // 		http.Error(w, "Failed to marshal transaction", http.StatusInternalServerError)
// // 		return
// // 	}

// // 	resp, err := http.Post(
// // 		ws.BlockchainNodeAddress+"/send_tx",
// // 		"application/json",
// // 		bytes.NewBuffer(txJSON),
// // 	)
// // 	if err != nil {
// // 		http.Error(w, fmt.Sprintf("Failed to send transaction: %v", err), http.StatusInternalServerError)
// // 		return
// // 	}
// // 	defer resp.Body.Close()

// // 	if resp.StatusCode != http.StatusOK {
// // 		body, _ := io.ReadAll(resp.Body)
// // 		http.Error(w, string(body), resp.StatusCode)
// // 		return
// // 	}

// // 	// Return the transaction with its hash and status
// // 	json.NewEncoder(w).Encode(tx)
// // }

// // Helper function to get account nonce from blockchain node
// // func (ws *WalletServer) getAccountNonce(address string) (uint64, error) {
// // 	// Call the blockchain node's API to get the nonce
// // 	resp, err := http.Get(fmt.Sprintf("%s/account/%s/nonce", ws.BlockchainNodeAddress, address))
// // 	if err != nil {
// // 		return 0, err
// // 	}
// // 	defer resp.Body.Close()

// // 	if resp.StatusCode != http.StatusOK {
// // 		return 0, fmt.Errorf("failed to get nonce: status %d", resp.StatusCode)
// // 	}

// // 	var result struct {
// // 		Nonce uint64 `json:"nonce"`
// // 	}
// // 	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
// // 		return 0, err
// // 	}

// // 	return result.Nonce, nil
// // }

// // func corsMiddleware(next http.Handler) http.Handler {
// // 	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// // 		w.Header().Set("Access-Control-Allow-Origin", "*")
// // 		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
// // 		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
// // 		if r.Method == "OPTIONS" {
// // 			w.WriteHeader(http.StatusOK)
// // 			return
// // 		}
// // 		next.ServeHTTP(w, r)
// // 	})
// // }

// // func (ws *WalletServer) GetBalance(w http.ResponseWriter, r *http.Request) {
// // 	w.Header().Set("Content-Type", "application/json")
// // 	w.Header().Set("Access-Control-Allow-Origin", "*") // Enable CORS

// // 	if r.Method != http.MethodGet {
// // 		http.Error(w, `{"error": "Method not allowed"}`, http.StatusMethodNotAllowed)
// // 		return
// // 	}

// // 	address := r.URL.Query().Get("address")
// // 	if address == "" {
// // 		http.Error(w, `{"error": "Address is required"}`, http.StatusBadRequest)
// // 		return
// // 	}

// // 	// Properly encode the address in the URL
// // 	url := fmt.Sprintf("%s/balance?address=%s", ws.BlockchainNodeAddress, url.QueryEscape(address))

// // 	resp, err := http.Get(url)
// // 	if err != nil {
// // 		http.Error(w, fmt.Sprintf(`{"error": "Blockchain node unreachable: %v"}`, err), http.StatusBadGateway)
// // 		return
// // 	}
// // 	defer resp.Body.Close()

// //		// Forward the exact response from the blockchain node
// //		w.WriteHeader(resp.StatusCode)
// //		if _, err := io.Copy(w, resp.Body); err != nil {
// //			log.Printf("Failed to forward balance response: %v", err)
// //		}
// //	}

// // GET /wallet/token-balance?contract=0x...&holder=0x...
// func (ws *WalletServer) GetTokenBalance(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
// 	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
// 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

// 	if r.Method == http.MethodOptions {
// 		w.WriteHeader(http.StatusOK)
// 		return
// 	}
// 	if r.Method != http.MethodGet {
// 		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
// 		return
// 	}

// 	contract := r.URL.Query().Get("contract")
// 	holder := r.URL.Query().Get("holder")
// 	if !wallet.ValidateAddress(contract) || !wallet.ValidateAddress(holder) {
// 		http.Error(w, `{"error":"invalid address"}`, http.StatusBadRequest)
// 		return
// 	}

// 	// Call node contract endpoint
// 	client := &http.Client{Timeout: 10 * time.Second}
// 	payload := map[string]interface{}{
// 		"address":  contract,
// 		"caller":   holder,
// 		"function": "balanceOf",
// 		"args":     []string{holder},
// 		"value":    0,
// 	}
// 	body, _ := json.Marshal(payload)
// 	resp, err := client.Post(ws.BlockchainNodeAddress+"/contract/call",
// 		"application/json",
// 		bytes.NewBuffer(body))
// 	if err != nil {
// 		http.Error(w, `{"error":"node call failed"}`, http.StatusBadGateway)
// 		return
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode != http.StatusOK {
// 		io.Copy(w, resp.Body)
// 		return
// 	}

// 	var result struct {
// 		Success bool   `json:"success"`
// 		Output  string `json:"output"` // token balance as string
// 	}
// 	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
// 		http.Error(w, `{"error":"decode error"}`, http.StatusInternalServerError)
// 		return
// 	}

// 	json.NewEncoder(w).Encode(result)
// }

// func (ws *WalletServer) Start() {
// 	// router := mux.NewRouter()
// 	// router.Use(corsMiddleware)
// 	portStr := fmt.Sprintf("%d", ws.Port)

// 	http.HandleFunc("/wallet/new", ws.CreateNewWallet)
// 	http.HandleFunc("/wallet/import/mnemonic", ws.ImportFromMnemonic)
// 	http.HandleFunc("/wallet/import/private-key", ws.ImportFromPrivateKey)
// 	http.HandleFunc("/wallet/balance", ws.GetBalance)
// 	http.HandleFunc("/wallet/send", ws.SendTransaction)
// 	http.HandleFunc("/wallet/token-balance", ws.GetTokenBalance)

// 	log.Printf("Starting wallet server on port %d\n", ws.Port)
// 	log.Printf("Connected to blockchain node at %s\n", ws.BlockchainNodeAddress)

// 	if err := http.ListenAndServe(":"+portStr, nil); err != nil {
// 		log.Fatalf("Failed to start wallet server: %v", err)
// 	}
// }

package walletserver

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	blockchaincomponent "github.com/Zotish/DefenceProject/BlockchainComponent"
	constantset "github.com/Zotish/DefenceProject/ConstantSet"
	wallet "github.com/Zotish/DefenceProject/WalletComponent"
)

var (
	allowedOrigins = []string{
		"http://localhost:3000",
		"http://frontend:3000", // Add internal Docker network address
	}
)

func init() {
	// Load from environment
	if envOrigins := os.Getenv("CORS_ALLOWED_ORIGINS"); envOrigins != "" {
		allowedOrigins = strings.Split(envOrigins, ",")
	}
}

type WalletServer struct {
	Port                  uint64
	BlockchainNodeAddress string
}

func NewWalletServer(port uint64, blockchainNodeAddress string) *WalletServer {
	return &WalletServer{
		Port:                  port,
		BlockchainNodeAddress: blockchainNodeAddress,
	}
}

func (ws *WalletServer) CreateNewWallet(w http.ResponseWriter, r *http.Request) {
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

	// Read password from request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var request struct {
		Password string `json:"password"`
	}
	if err := json.Unmarshal(body, &request); err != nil {
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Create new wallet
	newWallet, err := wallet.NewWallet(request.Password)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create wallet: %v", err), http.StatusInternalServerError)
		return
	}

	// Prepare response
	response := struct {
		Address    string `json:"address"`
		PrivateKey string `json:"private_key"`
		Mnemonic   string `json:"mnemonic"`
	}{
		Address:    newWallet.Address,
		PrivateKey: newWallet.GetPrivateKeyHex(),
		Mnemonic:   newWallet.Mnemonic,
	}
	// if isValidatorWallet(newWallet.Address) { // You'll need to implement this check
	// 	blockchain.RegisterValidatorWallet(newWallet.Address, newWallet)
	// }
	jsonResponse, err := json.Marshal(response)
	if err != nil {
		http.Error(w, "Failed to marshal response", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write(jsonResponse)
}

func (ws *WalletServer) ImportFromMnemonic(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		Mnemonic string `json:"mnemonic"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	importedWallet, err := wallet.ImportFromMnemonic(request.Mnemonic, request.Password)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to import wallet: %v", err), http.StatusBadRequest)
		return
	}

	response := struct {
		Address    string `json:"address"`
		PrivateKey string `json:"private_key"`
	}{
		Address:    importedWallet.Address,
		PrivateKey: importedWallet.GetPrivateKeyHex(),
	}

	json.NewEncoder(w).Encode(response)
}

func (ws *WalletServer) ImportFromPrivateKey(w http.ResponseWriter, r *http.Request) {
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

	var request struct {
		PrivateKey string `json:"private_key"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	importedWallet, err := wallet.ImportFromPrivateKey(request.PrivateKey)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to import wallet: %v", err), http.StatusBadRequest)
		return
	}

	response := struct {
		Address string `json:"address"`
	}{
		Address: importedWallet.Address,
	}

	json.NewEncoder(w).Encode(response)
}

// func (ws *WalletServer) ImportFromPrivateKey(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
// 	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
// 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

// 	if r.Method != http.MethodPost {
// 		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// 		return
// 	}

// 	var request struct {
// 		PrivateKey string `json:"private_key"`
// 	}

// 	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
// 		http.Error(w, "Invalid request format", http.StatusBadRequest)
// 		return
// 	}

// 	importedWallet, err := wallet.ImportFromPrivateKey(request.PrivateKey)
// 	if err != nil {
// 		http.Error(w, fmt.Sprintf("Failed to import wallet: %v", err), http.StatusBadRequest)
// 		return
// 	}

// 	response := struct {
// 		Address string `json:"address"`
// 	}{
// 		Address: importedWallet.Address,
// 	}

// 	json.NewEncoder(w).Encode(response)
// }

func (ws *WalletServer) GetBalance(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		http.Error(w, `{"error": "Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	address := r.URL.Query().Get("address")
	if address == "" {
		http.Error(w, `{"error": "Address is required"}`, http.StatusBadRequest)
		return
	}

	// Validate address format
	if !wallet.ValidateAddress(address) {
		http.Error(w, `{"error": "Invalid address format"}`, http.StatusBadRequest)
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(fmt.Sprintf("%s/balance?address=%s", ws.BlockchainNodeAddress, url.QueryEscape(address)))
	if err != nil {
		http.Error(w, `{"error": "Blockchain node unreachable"}`, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// func (ws *WalletServer) GetBalance(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Access-Control-Allow-Origin", "*") // Enable CORS
// 	w.Header().Set("Content-Type", "application/json")

// 	if r.Method != http.MethodGet {
// 		http.Error(w, `{"error": "Method not allowed"}`, http.StatusMethodNotAllowed)
// 		return
// 	}

// 	address := r.URL.Query().Get("address")
// 	if address == "" {
// 		http.Error(w, `{"error": "Address is required"}`, http.StatusBadRequest)
// 		return
// 	}

// 	url := fmt.Sprintf("%s/balance?address=%s", ws.BlockchainNodeAddress, address)
// 	resp, err := http.Get(url)
// 	if err != nil {
// 		http.Error(w, `{"error": "Blockchain node unreachable"}`, http.StatusBadGateway)
// 		return
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode != http.StatusOK {
// 		body, _ := io.ReadAll(resp.Body)
// 		http.Error(w, string(body), resp.StatusCode)
// 		return
// 	}

// 	io.Copy(w, resp.Body)
// }

// func (ws *WalletServer) GetBalance(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")

// 	if r.Method == http.MethodGet {
// 		params := url.Values{}
// 		params.Add("address", r.URL.Query().Get("address"))
// 		//address := r.URL.Query().Get("address")
// 		ourUrl := fmt.Sprintf("%s/balance?%s", ws.BlockchainNodeAddress, params.Encode())
// 		resp, err := http.Get(ourUrl)
// 		if err != nil {
// 			http.Error(w, err.Error(), http.StatusBadRequest)
// 			return
// 		}
// 		// if address == "" {
// 		// 	http.Error(w, `{"error": "Address is required"}`, http.StatusBadRequest)
// 		// 	return
// 		// }

// 		// // Validate address format
// 		// if !strings.HasPrefix(address, "0x") || len(address) != 42 {
// 		// 	http.Error(w, `{"error": "Invalid address format"}`, http.StatusBadRequest)
// 		// 	return
// 		// }

// 		defer resp.Body.Close()
// 		data, err := io.ReadAll(resp.Body)
// 		if err != nil {
// 			http.Error(w, err.Error(), http.StatusBadRequest)
// 			return
// 		}
// 		w.Write(data)

// 	} else {
// 		http.Error(w, "Invalid Method", http.StatusBadRequest)
// 	}

// }

// func (ws *WalletServer) GetBalance(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Access-Control-Allow-Origin", "*")
// 	w.Header().Set("Content-Type", "application/json")

// 	if r.Method != http.MethodGet {
// 		http.Error(w, `{"error": "Method not allowed"}`, http.StatusMethodNotAllowed)
// 		return
// 	}

// 	address := r.URL.Query().Get("address")
// 	if address == "" || !wallet.ValidateAddress(address) {
// 		http.Error(w, `{"error": "Valid address is required"}`, http.StatusBadRequest)
// 		return
// 	}

// 	resp, err := http.Get(fmt.Sprintf("%s/balance?address=%s", ws.BlockchainNodeAddress, address))
// 	if err != nil {
// 		http.Error(w, `{"error": "Blockchain node unreachable"}`, http.StatusBadGateway)
// 		return
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode != http.StatusOK {
// 		body, _ := io.ReadAll(resp.Body)
// 		http.Error(w, string(body), resp.StatusCode)
// 		return
// 	}

// 	// Forward the response
// 	w.WriteHeader(resp.StatusCode)
// 	io.Copy(w, resp.Body)
// }

// func (ws *WalletServer) GetBalance(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Access-Control-Allow-Origin", "*") // CORS
// 	w.Header().Set("Content-Type", "application/json")

// 	if r.Method != http.MethodGet {
// 		http.Error(w, `{"error": "Invalid request method"}`, http.StatusMethodNotAllowed)
// 		return
// 	}

// 	address := r.URL.Query().Get("address")
// 	if address == "" {
// 		http.Error(w, `{"error": "Address is required"}`, http.StatusBadRequest)
// 		return
// 	}

// 	// Forward the request to the blockchain node
// 	url := fmt.Sprintf("%s/balance?address=%s", ws.BlockchainNodeAddress, address)
// 	resp, err := http.Get(url)
// 	if err != nil {
// 		http.Error(w, `{"error": "Blockchain node unreachable"}`, http.StatusBadGateway)
// 		return
// 	}
// 	defer resp.Body.Close()

// 	// Proxy the response back to the client
// 	w.WriteHeader(resp.StatusCode)
// 	io.Copy(w, resp.Body)
// }

// func (ws *WalletServer) GetBalance(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")

// 	if r.Method != http.MethodGet {
// 		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// 		return
// 	}

// 	address := r.URL.Query().Get("address")
// 	if address == "" {
// 		http.Error(w, "Address parameter is required", http.StatusBadRequest)
// 		return
// 	}

// 	// Forward request to blockchain node
// 	queryParams := url.Values{}
// 	queryParams.Add("address", address)
// 	url := fmt.Sprintf("%s/balance?%s", ws.BlockchainNodeAddress, queryParams.Encode())

// 	resp, err := http.Get(url)
// 	if err != nil {
// 		http.Error(w, fmt.Sprintf("Failed to query blockchain: %v", err), http.StatusInternalServerError)
// 		return
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode != http.StatusOK {
// 		body, _ := io.ReadAll(resp.Body)
// 		http.Error(w, string(body), resp.StatusCode)
// 		return
// 	}

// 	// Stream the response directly to the client
// 	io.Copy(w, resp.Body)
// }

func (ws *WalletServer) SendTransaction(w http.ResponseWriter, r *http.Request) {
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

	// Accept data as string for safety; we'll decode to bytes ourselves.
	var request struct {
		From       string `json:"from"`
		To         string `json:"to"`
		Value      uint64 `json:"value"`
		Data       string `json:"data"` // <— string now
		Gas        uint64 `json:"gas"`
		GasPrice   uint64 `json:"gas_price"`
		PrivateKey string `json:"private_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, `{"error":"Invalid request format"}`, http.StatusBadRequest)
		return
	}

	// 1) Validate addresses
	if !wallet.ValidateAddress(request.From) || !wallet.ValidateAddress(request.To) {
		http.Error(w, `{"error":"Invalid address format"}`, http.StatusBadRequest)
		return
	}

	// 2) Load signer and enforce From == signer address
	signer, err := wallet.ImportFromPrivateKey(request.PrivateKey)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Failed to import wallet: %v"}`, err), http.StatusBadRequest)
		return
	}
	if !strings.EqualFold(signer.Address, request.From) {
		http.Error(w, `{"error":"From address does not match private key"}`, http.StatusBadRequest)
		return
	}

	// 3) Decode data string → []byte
	var dataBytes []byte
	if request.Data != "" {
		if strings.HasPrefix(request.Data, "0x") || strings.HasPrefix(request.Data, "0X") {
			// hex
			db, derr := hex.DecodeString(request.Data[2:])
			if derr != nil {
				http.Error(w, `{"error":"Invalid hex in 'data'"}`, http.StatusBadRequest)
				return
			}
			dataBytes = db
		} else {
			// treat as UTF-8 text
			dataBytes = []byte(request.Data)
		}
	}

	client := &http.Client{Timeout: 10 * time.Second}

	// 4) Get NONCE (next usable)
	// nonceURL := fmt.Sprintf("%s/account/%s/nonce", ws.BlockchainNodeAddress, request.From)
	// resp, err := client.Get(nonceURL)
	// if err != nil {
	// 	http.Error(w, `{"error":"Failed to get nonce from blockchain"}`, http.StatusBadGateway)
	// 	return
	// }
	// defer resp.Body.Close()
	// if resp.StatusCode != http.StatusOK {
	// 	body, _ := io.ReadAll(resp.Body)
	// 	http.Error(w, string(body), resp.StatusCode)
	// 	return
	// }
	// var nonceResp struct {
	// 	Nonce          uint64 `json:"nonce"`
	// 	NextNonce      uint64 `json:"next_nonce"`
	// 	ConfirmedNonce uint64 `json:"confirmed_nonce"`
	// }
	// if err := json.NewDecoder(resp.Body).Decode(&nonceResp); err != nil {
	// 	http.Error(w, `{"error":"Failed to decode nonce response"}`, http.StatusInternalServerError)
	// 	return
	// }

	// // Prefer next_nonce; fallback to nonce (which our node also sets to next)
	// txNonce := nonceResp.NextNonce
	// if txNonce == 0 {
	// 	txNonce = nonceResp.Nonce
	// }

	// 4) Get NONCE (if your endpoint returns CURRENT nonce, uncomment the ++)
	// nonceURL := fmt.Sprintf("%s/account/%s/nonce", ws.BlockchainNodeAddress, request.From)
	// resp, err := client.Get(nonceURL)
	// if err != nil {
	// 	http.Error(w, `{"error":"Failed to get nonce from blockchain"}`, http.StatusBadGateway)
	// 	return
	// }
	// defer resp.Body.Close()
	// if resp.StatusCode != http.StatusOK {
	// 	body, _ := io.ReadAll(resp.Body)
	// 	http.Error(w, string(body), resp.StatusCode)
	// 	return
	// }
	// var nonceResp struct {
	// 	Nonce uint64 `json:"nonce"`
	// }
	// if err := json.NewDecoder(resp.Body).Decode(&nonceResp); err != nil {
	// 	http.Error(w, `{"error":"Failed to decode nonce response"}`, http.StatusInternalServerError)
	// 	return
	// }
	// If your nonce API returns CURRENT nonce, use the next:
	//nonceResp.Nonce++

	// 5) Enforce sane gas & price (avoid underpriced txs)
	minBaseFee := uint64(constantset.InitialBaseFee) // your chain’s base fee constant
	gas := request.Gas
	if gas == 0 {
		gas = uint64(constantset.MinGas) // e.g., 21000
	}
	gasPrice := request.GasPrice
	if gasPrice < minBaseFee {
		gasPrice = minBaseFee
	}
	// (Optional) add a small tip if your chain needs it:
	// gasPrice += uint64(constantset.DefaultPriorityFee)

	// 6) Check balance is enough to pay value + fee before signing
	balResp, err := client.Get(fmt.Sprintf("%s/balance?address=%s", ws.BlockchainNodeAddress, url.QueryEscape(request.From)))
	if err != nil {
		http.Error(w, `{"error":"Failed to get balance from blockchain"}`, http.StatusBadGateway)
		return
	}
	defer balResp.Body.Close()
	if balResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(balResp.Body)
		http.Error(w, string(body), balResp.StatusCode)
		return
	}
	var bal struct {
		Balance uint64 `json:"balance"`
	}
	if err := json.NewDecoder(balResp.Body).Decode(&bal); err != nil {
		http.Error(w, `{"error":"Failed to decode balance response"}`, http.StatusInternalServerError)
		return
	}
	fee := gas * gasPrice
	total := request.Value + fee
	if bal.Balance < total {
		http.Error(w, fmt.Sprintf(`{"error":"Insufficient funds: balance=%d required=%d (value %d + fee %d)"}`, bal.Balance, total, request.Value, fee), http.StatusBadRequest)
		return
	}

	// 7) Build tx
	tx := blockchaincomponent.NewTransaction(
		request.From,
		request.To,
		request.Value,
		dataBytes,
		//nonceResp.Nonce,
	)
	tx.Gas = gas
	tx.GasPrice = gasPrice
	tx.ChainID = uint64(constantset.ChainID)

	// 8) Sign
	if err := signer.SignTransaction(tx); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Failed to sign transaction: %v"}`, err), http.StatusInternalServerError)
		return
	}

	// 9) POST to node
	txJSON, err := json.Marshal(tx)
	if err != nil {
		http.Error(w, `{"error":"Failed to marshal transaction"}`, http.StatusInternalServerError)
		return
	}
	resp, err := client.Post(ws.BlockchainNodeAddress+"/send_tx", "application/json", bytes.NewBuffer(txJSON))
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Failed to send transaction: %v"}`, err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

//this is last changes i did nov 6th
// func (ws *WalletServer) SendTransaction(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
// 	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
// 	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
// 	if r.Method == http.MethodOptions {
// 		w.WriteHeader(http.StatusOK)
// 		return
// 	}

// 	w.Header().Set("Content-Type", "application/json")

// 	if r.Method != http.MethodPost {
// 		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// 		return
// 	}

// 	var request struct {
// 		From       string `json:"from"`
// 		To         string `json:"to"`
// 		Value      uint64 `json:"value"`
// 		Data       []byte `json:"data"`
// 		Gas        uint64 `json:"gas"`
// 		GasPrice   uint64 `json:"gas_price"`
// 		PrivateKey string `json:"private_key"`
// 	}

// 	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
// 		http.Error(w, `{"error": "Invalid request format"}`, http.StatusBadRequest)
// 		return
// 	}

// 	// Validate addresses
// 	if !wallet.ValidateAddress(request.From) || !wallet.ValidateAddress(request.To) {
// 		http.Error(w, `{"error": "Invalid address format"}`, http.StatusBadRequest)
// 		return
// 	}

// 	// Import wallet
// 	wallet, err := wallet.ImportFromPrivateKey(request.PrivateKey)
// 	if err != nil {
// 		http.Error(w, fmt.Sprintf(`{"error": "Failed to import wallet: %v"}`, err), http.StatusBadRequest)
// 		return
// 	}

// 	// Get nonce from blockchain node
// 	client := &http.Client{Timeout: 10 * time.Second}
// 	resp, err := client.Get(fmt.Sprintf("%s/account/%s/nonce", ws.BlockchainNodeAddress, request.From))
// 	if err != nil {
// 		http.Error(w, `{"error": "Failed to get nonce from blockchain"}`, http.StatusBadGateway)
// 		return
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode != http.StatusOK {
// 		body, _ := io.ReadAll(resp.Body)
// 		http.Error(w, string(body), resp.StatusCode)
// 		return
// 	}

// 	var nonceResp struct {
// 		Nonce uint64 `json:"nonce"`
// 	}
// 	if err := json.NewDecoder(resp.Body).Decode(&nonceResp); err != nil {
// 		http.Error(w, `{"error": "Failed to decode nonce response"}`, http.StatusInternalServerError)
// 		return
// 	}

// 	// Create and sign transaction
// 	tx := blockchaincomponent.NewTransaction(
// 		request.From,
// 		request.To,
// 		request.Value,
// 		request.Data,
// 		nonceResp.Nonce,
// 	)
// 	tx.Gas = request.Gas
// 	tx.GasPrice = request.GasPrice

// 	if err := wallet.SignTransaction(tx); err != nil {
// 		http.Error(w, fmt.Sprintf(`{"error": "Failed to sign transaction: %v"}`, err), http.StatusInternalServerError)
// 		return
// 	}

// 	// Send to blockchain node
// 	txJSON, err := json.Marshal(tx)
// 	if err != nil {
// 		http.Error(w, `{"error": "Failed to marshal transaction"}`, http.StatusInternalServerError)
// 		return
// 	}

// 	resp, err = client.Post(
// 		ws.BlockchainNodeAddress+"/send_tx",
// 		"application/json",
// 		bytes.NewBuffer(txJSON),
// 	)
// 	if err != nil {
// 		http.Error(w, fmt.Sprintf(`{"error": "Failed to send transaction: %v"}`, err), http.StatusBadGateway)
// 		return
// 	}
// 	defer resp.Body.Close()

// 	w.WriteHeader(resp.StatusCode)
// 	io.Copy(w, resp.Body)
// }

// func (ws *WalletServer) SendTransaction(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	w.Header().Set("Access-Control-Allow-Origin", "*")

// 	if r.Method != http.MethodPost {
// 		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// 		return
// 	}

// 	var request struct {
// 		From       string `json:"from"`
// 		To         string `json:"to"`
// 		Value      uint64 `json:"value"`
// 		Data       []byte `json:"data"`
// 		Gas        uint64 `json:"gas"`
// 		GasPrice   uint64 `json:"gas_price"`
// 		PrivateKey string `json:"private_key"`
// 	}

// 	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
// 		http.Error(w, "Invalid request format", http.StatusBadRequest)
// 		return
// 	}

// 	// Import wallet from private key
// 	wallet, err := wallet.ImportFromPrivateKey(request.PrivateKey)
// 	if err != nil {
// 		http.Error(w, fmt.Sprintf("Failed to import wallet: %v", err), http.StatusBadRequest)
// 		return
// 	}

// 	// Get the current nonce from the blockchain node
// 	nonce, err := ws.getAccountNonce(request.From)
// 	if err != nil {
// 		http.Error(w, fmt.Sprintf("Failed to get account nonce: %v", err), http.StatusInternalServerError)
// 		return
// 	}

// 	// Create transaction
// 	tx := blockchaincomponent.NewTransaction(
// 		request.From,
// 		request.To,
// 		request.Value,
// 		request.Data,
// 		nonce+1, // Use the next nonce
// 	)
// 	tx.Gas = request.Gas
// 	tx.GasPrice = request.GasPrice

// 	// Sign transaction
// 	if err := wallet.SignTransaction(tx); err != nil {
// 		http.Error(w, fmt.Sprintf("Failed to sign transaction: %v", err), http.StatusInternalServerError)
// 		return
// 	}

// 	// Send to blockchain node
// 	txJSON, err := json.Marshal(tx)
// 	if err != nil {
// 		http.Error(w, "Failed to marshal transaction", http.StatusInternalServerError)
// 		return
// 	}

// 	resp, err := http.Post(
// 		ws.BlockchainNodeAddress+"/send_tx",
// 		"application/json",
// 		bytes.NewBuffer(txJSON),
// 	)
// 	if err != nil {
// 		http.Error(w, fmt.Sprintf("Failed to send transaction: %v", err), http.StatusInternalServerError)
// 		return
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode != http.StatusOK {
// 		body, _ := io.ReadAll(resp.Body)
// 		http.Error(w, string(body), resp.StatusCode)
// 		return
// 	}

// 	// Return the transaction with its hash and status
// 	json.NewEncoder(w).Encode(tx)
// }

// Helper function to get account nonce from blockchain node
// func (ws *WalletServer) getAccountNonce(address string) (uint64, error) {
// 	// Call the blockchain node's API to get the nonce
// 	resp, err := http.Get(fmt.Sprintf("%s/account/%s/nonce", ws.BlockchainNodeAddress, address))
// 	if err != nil {
// 		return 0, err
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode != http.StatusOK {
// 		return 0, fmt.Errorf("failed to get nonce: status %d", resp.StatusCode)
// 	}

// 	var result struct {
// 		Nonce uint64 `json:"nonce"`
// 	}
// 	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
// 		return 0, err
// 	}

// 	return result.Nonce, nil
// }

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

// func (ws *WalletServer) GetBalance(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	w.Header().Set("Access-Control-Allow-Origin", "*") // Enable CORS

// 	if r.Method != http.MethodGet {
// 		http.Error(w, `{"error": "Method not allowed"}`, http.StatusMethodNotAllowed)
// 		return
// 	}

// 	address := r.URL.Query().Get("address")
// 	if address == "" {
// 		http.Error(w, `{"error": "Address is required"}`, http.StatusBadRequest)
// 		return
// 	}

// 	// Properly encode the address in the URL
// 	url := fmt.Sprintf("%s/balance?address=%s", ws.BlockchainNodeAddress, url.QueryEscape(address))

// 	resp, err := http.Get(url)
// 	if err != nil {
// 		http.Error(w, fmt.Sprintf(`{"error": "Blockchain node unreachable: %v"}`, err), http.StatusBadGateway)
// 		return
// 	}
// 	defer resp.Body.Close()

//		// Forward the exact response from the blockchain node
//		w.WriteHeader(resp.StatusCode)
//		if _, err := io.Copy(w, resp.Body); err != nil {
//			log.Printf("Failed to forward balance response: %v", err)
//		}
//	}

// GET /wallet/token-balance?contract=0x...&holder=0x...
func (ws *WalletServer) GetTokenBalance(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	contract := r.URL.Query().Get("contract")
	holder := r.URL.Query().Get("holder")
	if !wallet.ValidateAddress(contract) || !wallet.ValidateAddress(holder) {
		http.Error(w, `{"error":"invalid address"}`, http.StatusBadRequest)
		return
	}

	// Call node contract endpoint
	client := &http.Client{Timeout: 10 * time.Second}
	payload := map[string]interface{}{
		"address":  contract,
		"caller":   holder,
		"function": "balanceOf",
		"args":     []string{holder},
		"value":    0,
	}
	body, _ := json.Marshal(payload)
	resp, err := client.Post(ws.BlockchainNodeAddress+"/contract/call",
		"application/json",
		bytes.NewBuffer(body))
	if err != nil {
		http.Error(w, `{"error":"node call failed"}`, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		io.Copy(w, resp.Body)
		return
	}

	var result struct {
		Success bool   `json:"success"`
		Output  string `json:"output"` // token balance as string
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		http.Error(w, `{"error":"decode error"}`, http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(result)
}

func (ws *WalletServer) Start() {
	// router := mux.NewRouter()
	// router.Use(corsMiddleware)
	portStr := fmt.Sprintf("%d", ws.Port)

	http.HandleFunc("/wallet/new", ws.CreateNewWallet)
	http.HandleFunc("/wallet/import/mnemonic", ws.ImportFromMnemonic)
	http.HandleFunc("/wallet/import/private-key", ws.ImportFromPrivateKey)
	http.HandleFunc("/wallet/balance", ws.GetBalance)
	http.HandleFunc("/wallet/send", ws.SendTransaction)
	http.HandleFunc("/wallet/token-balance", ws.GetTokenBalance)

	log.Printf("Starting wallet server on port %d\n", ws.Port)
	log.Printf("Connected to blockchain node at %s\n", ws.BlockchainNodeAddress)

	if err := http.ListenAndServe(":"+portStr, nil); err != nil {
		log.Fatalf("Failed to start wallet server: %v", err)
	}
}
