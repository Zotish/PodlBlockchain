package walletserver

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	blockchaincomponent "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/BlockchainComponent"
	constantset "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/ConstantSet"
	wallet "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/WalletComponent"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

type WalletServer struct {
	Port                  uint64
	BlockchainNodeAddress string
}

func resolveBridgeChainConfig(chainID string) *blockchaincomponent.BridgeChainConfig {
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

func bridgeChainIDFromValue(value string) string {
	chainID := strings.ToLower(strings.TrimSpace(value))
	if chainID == "" {
		chainID = "bsc-testnet"
	}
	return chainID
}

func (ws *WalletServer) fetchNextNonce(client *http.Client, addr string) (uint64, error) {
	nonceURL := fmt.Sprintf("%s/account/%s/nonce", ws.BlockchainNodeAddress, url.QueryEscape(addr))
	resp, err := client.Get(nonceURL)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if len(body) > 0 {
			return 0, fmt.Errorf("nonce lookup failed: %s", strings.TrimSpace(string(body)))
		}
		return 0, fmt.Errorf("nonce lookup failed: %s", resp.Status)
	}

	var nonceResp struct {
		Nonce          uint64 `json:"nonce"`
		NextNonce      uint64 `json:"next_nonce"`
		ConfirmedNonce uint64 `json:"confirmed_nonce"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&nonceResp); err != nil {
		return 0, err
	}

	switch {
	case nonceResp.NextNonce > 0:
		return nonceResp.NextNonce, nil
	case nonceResp.Nonce > 0:
		return nonceResp.Nonce, nil
	default:
		return nonceResp.ConfirmedNonce + 1, nil
	}
}

func NewWalletServer(port uint64, blockchainNodeAddress string) *WalletServer {
	return &WalletServer{
		Port:                  port,
		BlockchainNodeAddress: blockchainNodeAddress,
	}
}

func (ws *WalletServer) CreateNewWallet(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
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
	w.Header().Set("Access-Control-Allow-Origin", "*")
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

func (ws *WalletServer) GetBalance(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
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

func (ws *WalletServer) SendTransaction(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
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
		Value      Amount `json:"value"`
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
	txNonce, err := ws.fetchNextNonce(client, request.From)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Failed to get nonce from blockchain: %v"}`, err), http.StatusBadGateway)
		return
	}

	// 4) Get NONCE (next usable)
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
		Balance string `json:"balance"`
	}
	if err := json.NewDecoder(balResp.Body).Decode(&bal); err != nil {
		http.Error(w, `{"error":"Failed to decode balance response"}`, http.StatusInternalServerError)
		return
	}
	fee := gas * gasPrice
	balAmt, err := blockchaincomponent.NewAmountFromString(bal.Balance)
	if err != nil {
		http.Error(w, `{"error":"Invalid balance format"}`, http.StatusInternalServerError)
		return
	}
	valueAmt := request.Value.Int
	if valueAmt == nil {
		valueAmt = big.NewInt(0)
	}
	total := new(big.Int).Add(valueAmt, blockchaincomponent.NewAmountFromUint64(fee))
	if balAmt.Cmp(total) < 0 {
		http.Error(w, fmt.Sprintf(`{"error":"Insufficient funds: balance=%s required=%s (value %s + fee %d)"}`, balAmt.String(), total.String(), valueAmt.String(), fee), http.StatusBadRequest)
		return
	}

	// 7) Build tx
	tx := blockchaincomponent.NewTransaction(
		request.From,
		request.To,
		valueAmt,
		dataBytes,
	)
	tx.Gas = gas
	tx.GasPrice = gasPrice
	tx.Nonce = txNonce
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

func (ws *WalletServer) bridgeLockWithMode(w http.ResponseWriter, r *http.Request, forcedMode string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
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
		From       string `json:"from"`
		ToBSC      string `json:"to_bsc"`
		ChainID    string `json:"chain_id"`
		Amount     Amount `json:"amount"`
		Gas        uint64 `json:"gas"`
		GasPrice   uint64 `json:"gas_price"`
		PrivateKey string `json:"private_key"`
		Mode       string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, `{"error":"Invalid request format"}`, http.StatusBadRequest)
		return
	}
	mode := strings.ToLower(strings.TrimSpace(request.Mode))
	if forcedMode != "" {
		mode = strings.ToLower(strings.TrimSpace(forcedMode))
	}

	if !wallet.ValidateAddress(request.From) {
		http.Error(w, `{"error":"Invalid from address"}`, http.StatusBadRequest)
		return
	}
	if request.ToBSC == "" {
		http.Error(w, `{"error":"Missing to_bsc address"}`, http.StatusBadRequest)
		return
	}
	if request.Amount.Int == nil || request.Amount.Int.Sign() == 0 {
		http.Error(w, `{"error":"Missing amount"}`, http.StatusBadRequest)
		return
	}

	signer, err := wallet.ImportFromPrivateKey(request.PrivateKey)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Failed to import wallet: %v"}`, err), http.StatusBadRequest)
		return
	}
	if !strings.EqualFold(signer.Address, request.From) {
		http.Error(w, `{"error":"From address does not match private key"}`, http.StatusBadRequest)
		return
	}

	gas := request.Gas
	if gas == 0 {
		gas = uint64(constantset.ContractCallGas)
	}
	gasPrice := request.GasPrice
	if gasPrice < uint64(constantset.InitialBaseFee) {
		gasPrice = uint64(constantset.InitialBaseFee)
	}

	tx := blockchaincomponent.NewTransaction(
		request.From,
		constantset.BridgeEscrowAddress,
		request.Amount.Int,
		nil,
	)
	tx.Type = "bridge_lock"
	if mode == "private" {
		tx.Type = "bridge_lock_private"
	}
	tx.Args = []string{request.ToBSC}
	tx.Gas = gas
	tx.GasPrice = gasPrice
	tx.TxHash = blockchaincomponent.CalculateTransactionHash(*tx)

	if err := signer.SignTransaction(tx); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Failed to sign transaction: %v"}`, err), http.StatusBadRequest)
		return
	}

	chainID := bridgeChainIDFromValue(request.ChainID)
	body, _ := json.Marshal(tx)
	resp, err := http.Post(ws.BlockchainNodeAddress+"/send_tx", "application/json", bytes.NewReader(body))
	if err != nil {
		http.Error(w, `{"error":"Blockchain node unreachable"}`, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)

	go func() {
		payload := map[string]any{
			"chain_id": chainID,
			"tx_hash":  tx.TxHash,
			"from":     request.From,
			"to_lqd":   request.ToBSC,
			"amount":   request.Amount.Int.String(),
			"mode":     mode,
		}
		body, _ := json.Marshal(payload)
		_, _ = http.Post(ws.BlockchainNodeAddress+"/bridge/lock_chain", "application/json", bytes.NewReader(body))
	}()
}

func (ws *WalletServer) BridgeLock(w http.ResponseWriter, r *http.Request) {
	ws.bridgeLockWithMode(w, r, "")
}

func (ws *WalletServer) BridgePrivateLock(w http.ResponseWriter, r *http.Request) {
	ws.bridgeLockWithMode(w, r, "private")
}

func (ws *WalletServer) bridgeBurnWithMode(w http.ResponseWriter, r *http.Request, forcedMode string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
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
		Amount     Amount `json:"amount"`
		ToLqd      string `json:"to_lqd"`
		ChainID    string `json:"chain_id"`
		Mode       string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, `{"error":"Invalid request format"}`, http.StatusBadRequest)
		return
	}
	_ = forcedMode
	if request.PrivateKey == "" || request.Amount.Int == nil || request.Amount.Int.Sign() == 0 || request.ToLqd == "" {
		http.Error(w, `{"error":"Missing fields"}`, http.StatusBadRequest)
		return
	}
	if !wallet.ValidateAddress(request.ToLqd) {
		http.Error(w, `{"error":"Invalid to_lqd address"}`, http.StatusBadRequest)
		return
	}

	rpc := os.Getenv("BSC_TESTNET_RPC")
	bridgeAddr := os.Getenv("BSC_BRIDGE_ADDRESS")
	if rpc == "" || bridgeAddr == "" {
		http.Error(w, `{"error":"BSC bridge not configured"}`, http.StatusBadRequest)
		return
	}

	client, _, err := blockchaincomponent.DialBscClient(blockchaincomponent.BridgeRPCEndpoints(rpc))
	if err != nil {
		http.Error(w, `{"error":"Failed to connect to BSC RPC"}`, http.StatusBadGateway)
		return
	}

	parsedABI, err := abi.JSON(strings.NewReader(`[{"inputs":[{"internalType":"uint256","name":"amount","type":"uint256"},{"internalType":"string","name":"toLqd","type":"string"}],"name":"burn","outputs":[],"stateMutability":"nonpayable","type":"function"}]`))
	if err != nil {
		http.Error(w, `{"error":"ABI error"}`, http.StatusInternalServerError)
		return
	}

	key, err := crypto.HexToECDSA(strings.TrimPrefix(request.PrivateKey, "0x"))
	if err != nil {
		http.Error(w, `{"error":"Invalid private key"}`, http.StatusBadRequest)
		return
	}
	auth, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(97))
	if err != nil {
		http.Error(w, `{"error":"Signer error"}`, http.StatusBadRequest)
		return
	}

	gp, _ := client.SuggestGasPrice(context.Background())
	auth.GasPrice = gp
	auth.GasLimit = 200000

	contract := bind.NewBoundContract(common.HexToAddress(bridgeAddr), parsedABI, client, client, client)
	tx, err := contract.Transact(auth, "burn", request.Amount.Int, request.ToLqd)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusBadRequest)
		return
	}

	if consensus, err := blockchaincomponent.ConsensusReceipt(blockchaincomponent.BridgeRPCEndpoints(rpc), tx.Hash().Hex(), 2*time.Minute, 2*time.Second); err != nil || !blockchaincomponent.ReceiptSuccessful(consensus.Receipt) {
		msg := "BSC burn pending"
		if err != nil {
			msg = fmt.Sprintf("BSC burn receipt wait failed: %v", err)
		}
		http.Error(w, fmt.Sprintf(`{"error":"%s","tx_hash":"%s"}`, msg, tx.Hash().Hex()), http.StatusBadRequest)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"tx_hash": tx.Hash().Hex(),
	})
}

// BridgeLockBscToken locks a BEP20 token on BSC to mint on LQD.
func (ws *WalletServer) bridgeLockBscTokenWithMode(w http.ResponseWriter, r *http.Request, forcedMode string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
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
		PrivateKey     string `json:"private_key"`
		Token          string `json:"token"`
		Amount         Amount `json:"amount"`
		ToLqd          string `json:"to_lqd"`
		ChainID        string `json:"chain_id"`
		Family         string `json:"family"`
		Adapter        string `json:"adapter"`
		SourceTxHash   string `json:"source_tx_hash"`
		SourceAddress  string `json:"source_address"`
		SourceMemo     string `json:"source_memo"`
		SourceSequence string `json:"source_sequence"`
		SourceOutput   string `json:"source_output"`
		Mode           string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, `{"error":"Invalid request format"}`, http.StatusBadRequest)
		return
	}
	mode := strings.ToLower(strings.TrimSpace(request.Mode))
	if forcedMode != "" {
		mode = strings.ToLower(strings.TrimSpace(forcedMode))
	}
	if request.PrivateKey == "" || request.Token == "" || request.Amount.Int == nil || request.Amount.Int.Sign() == 0 || request.ToLqd == "" {
		http.Error(w, `{"error":"Missing fields"}`, http.StatusBadRequest)
		return
	}
	if !wallet.ValidateAddress(request.ToLqd) {
		http.Error(w, `{"error":"Invalid to_lqd address"}`, http.StatusBadRequest)
		return
	}

	chainID := bridgeChainIDFromValue(request.ChainID)
	cfg := resolveBridgeChainConfig(chainID)
	rpc := ""
	lockAddr := ""
	chainNativeID := uint64(97)
	if cfg != nil {
		rpc = strings.TrimSpace(cfg.RPC)
		if rpc == "" && len(cfg.RPCs) > 0 {
			rpc = strings.TrimSpace(cfg.RPCs[0])
		}
		lockAddr = strings.TrimSpace(cfg.LockAddress)
		if lockAddr == "" {
			lockAddr = strings.TrimSpace(cfg.BridgeAddress)
		}
		if parsed, err := strconv.ParseUint(strings.TrimSpace(cfg.ChainID), 10, 64); err == nil && parsed > 0 {
			chainNativeID = parsed
		}
	}
	if rpc == "" {
		rpc = os.Getenv("BSC_TESTNET_RPC")
	}
	if lockAddr == "" {
		lockAddr = os.Getenv("BSC_LOCK_ADDRESS")
	}
	if rpc == "" || lockAddr == "" {
		http.Error(w, `{"error":"bridge chain not configured"}`, http.StatusBadRequest)
		return
	}

	client, _, err := blockchaincomponent.DialBscClient(blockchaincomponent.BridgeRPCEndpoints(rpc))
	if err != nil {
		http.Error(w, `{"error":"Failed to connect to BSC RPC"}`, http.StatusBadGateway)
		return
	}

	parsedABI, err := abi.JSON(strings.NewReader(`[{"inputs":[{"internalType":"address","name":"token","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"},{"internalType":"string","name":"toLqd","type":"string"}],"name":"lock","outputs":[{"internalType":"bytes32","name":"","type":"bytes32"}],"stateMutability":"nonpayable","type":"function"}]`))
	if err != nil {
		http.Error(w, `{"error":"ABI error"}`, http.StatusInternalServerError)
		return
	}
	erc20ABI, err := abi.JSON(strings.NewReader(`[{"inputs":[{"internalType":"address","name":"owner","type":"address"},{"internalType":"address","name":"spender","type":"address"}],"name":"allowance","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"spender","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"}],"name":"approve","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"nonpayable","type":"function"}]`))
	if err != nil {
		http.Error(w, `{"error":"ERC20 ABI error"}`, http.StatusInternalServerError)
		return
	}

	key, err := crypto.HexToECDSA(strings.TrimPrefix(request.PrivateKey, "0x"))
	if err != nil {
		http.Error(w, `{"error":"Invalid private key"}`, http.StatusBadRequest)
		return
	}
	auth, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(int64(chainNativeID)))
	if err != nil {
		http.Error(w, `{"error":"Signer error"}`, http.StatusBadRequest)
		return
	}

	gp, _ := client.SuggestGasPrice(context.Background())
	auth.GasPrice = gp
	auth.GasLimit = 300000

	// Auto-approve if allowance < amount
	tokenAddr := common.HexToAddress(request.Token)
	ownerAddr := crypto.PubkeyToAddress(key.PublicKey)
	lockContract := common.HexToAddress(lockAddr)
	erc20 := bind.NewBoundContract(tokenAddr, erc20ABI, client, client, client)
	var allowanceRes []interface{}
	if err := erc20.Call(&bind.CallOpts{Context: context.Background()}, &allowanceRes, "allowance", ownerAddr, lockContract); err == nil {
		allowance := big.NewInt(0)
		if len(allowanceRes) > 0 {
			if v, ok := allowanceRes[0].(*big.Int); ok {
				allowance = v
			}
		}
		if allowance.Cmp(request.Amount.Int) < 0 {
			approveTx, err := erc20.Transact(auth, "approve", lockContract, request.Amount.Int)
			if err != nil {
				http.Error(w, fmt.Sprintf(`{"error":"approve failed: %v"}`, err), http.StatusBadRequest)
				return
			}
			log.Printf("BSC approve tx: %s", approveTx.Hash().Hex())
			// wait a moment for approve to be mined
			time.Sleep(2 * time.Second)
		}
	}

	contract := bind.NewBoundContract(common.HexToAddress(lockAddr), parsedABI, client, client, client)
	tx, err := contract.Transact(auth, "lock", common.HexToAddress(request.Token), request.Amount.Int, request.ToLqd)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusBadRequest)
		return
	}

	if consensus, err := blockchaincomponent.ConsensusReceipt(blockchaincomponent.BridgeRPCEndpoints(rpc), tx.Hash().Hex(), 2*time.Minute, 2*time.Second); err != nil || !blockchaincomponent.ReceiptSuccessful(consensus.Receipt) {
		msg := "BSC lock pending"
		if err != nil {
			msg = fmt.Sprintf("BSC lock receipt wait failed: %v", err)
		}
		http.Error(w, fmt.Sprintf(`{"error":"%s","tx_hash":"%s"}`, msg, tx.Hash().Hex()), http.StatusBadRequest)
		return
	}

	// Register the BSC lock on LQD immediately (fallback if RPC log scan misses)
	go func() {
		payload := map[string]interface{}{
			"chain_id":        chainID,
			"family":          strings.TrimSpace(request.Family),
			"adapter":         strings.TrimSpace(request.Adapter),
			"bsc_tx":          tx.Hash().Hex(),
			"token":           request.Token,
			"from":            ownerAddr.Hex(),
			"to_lqd":          request.ToLqd,
			"amount":          request.Amount.Int.String(),
			"source_tx_hash":  request.SourceTxHash,
			"source_address":  request.SourceAddress,
			"source_memo":     request.SourceMemo,
			"source_sequence": request.SourceSequence,
			"source_output":   request.SourceOutput,
			"mode":            mode,
		}
		body, _ := json.Marshal(payload)
		_, _ = http.Post(ws.BlockchainNodeAddress+"/bridge/lock_chain", "application/json", bytes.NewReader(body))
	}()

	json.NewEncoder(w).Encode(map[string]string{
		"tx_hash": tx.Hash().Hex(),
	})
}

func (ws *WalletServer) BridgeLockBscToken(w http.ResponseWriter, r *http.Request) {
	ws.bridgeLockBscTokenWithMode(w, r, "")
}

func (ws *WalletServer) BridgePrivateLockBscToken(w http.ResponseWriter, r *http.Request) {
	ws.bridgeLockBscTokenWithMode(w, r, "private")
}

// BridgeBscLockTxData prepares calldata for BSC lock() so frontend can send via injected wallet.
func (ws *WalletServer) BridgeBscLockTxData(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
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
		Token  string `json:"token"`
		Amount string `json:"amount"` // raw amount (uint256) as string
		ToLqd  string `json:"to_lqd"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, `{"error":"Invalid request format"}`, http.StatusBadRequest)
		return
	}
	if request.Token == "" || request.Amount == "" || request.ToLqd == "" {
		http.Error(w, `{"error":"Missing fields"}`, http.StatusBadRequest)
		return
	}
	if !wallet.ValidateAddress(request.Token) {
		http.Error(w, `{"error":"Invalid token address"}`, http.StatusBadRequest)
		return
	}

	lockAddr := os.Getenv("BSC_LOCK_ADDRESS")
	if lockAddr == "" {
		http.Error(w, `{"error":"BSC_LOCK_ADDRESS not set"}`, http.StatusInternalServerError)
		return
	}
	if !wallet.ValidateAddress(lockAddr) {
		http.Error(w, `{"error":"Invalid lock contract address"}`, http.StatusInternalServerError)
		return
	}

	parsedABI, err := abi.JSON(strings.NewReader(`[{"inputs":[{"internalType":"address","name":"token","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"},{"internalType":"string","name":"toLqd","type":"string"}],"name":"lock","outputs":[{"internalType":"bytes32","name":"","type":"bytes32"}],"stateMutability":"nonpayable","type":"function"}]`))
	if err != nil {
		http.Error(w, `{"error":"ABI parse failed"}`, http.StatusInternalServerError)
		return
	}

	amount := new(big.Int)
	if _, ok := amount.SetString(request.Amount, 10); !ok {
		http.Error(w, `{"error":"Invalid amount"}`, http.StatusBadRequest)
		return
	}

	data, err := parsedABI.Pack(
		"lock",
		common.HexToAddress(request.Token),
		amount,
		request.ToLqd,
	)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"pack failed: %v"}`, err), http.StatusBadRequest)
		return
	}

	json.NewEncoder(w).Encode(map[string]any{
		"to":   lockAddr,
		"data": "0x" + hex.EncodeToString(data),
		"gas":  300000,
	})
}

// BridgeBurnLqdToken burns a LQD bridge token to release on BSC.
func (ws *WalletServer) bridgeBurnLqdTokenWithMode(w http.ResponseWriter, r *http.Request, forcedMode string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
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
		PrivateKey     string `json:"private_key"`
		Token          string `json:"token"`
		Amount         Amount `json:"amount"`
		ToBsc          string `json:"to_bsc"`
		ChainID        string `json:"chain_id"`
		Family         string `json:"family"`
		Adapter        string `json:"adapter"`
		SourceTxHash   string `json:"source_tx_hash"`
		SourceAddress  string `json:"source_address"`
		SourceMemo     string `json:"source_memo"`
		SourceSequence string `json:"source_sequence"`
		SourceOutput   string `json:"source_output"`
		Mode           string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, `{"error":"Invalid request format"}`, http.StatusBadRequest)
		return
	}
	mode := strings.ToLower(strings.TrimSpace(request.Mode))
	if forcedMode != "" {
		mode = strings.ToLower(strings.TrimSpace(forcedMode))
	}
	if request.PrivateKey == "" || request.Token == "" || request.Amount.Int == nil || request.Amount.Int.Sign() == 0 || request.ToBsc == "" {
		http.Error(w, `{"error":"Missing fields"}`, http.StatusBadRequest)
		return
	}
	if !wallet.ValidateAddress(request.Token) {
		http.Error(w, `{"error":"Invalid token address"}`, http.StatusBadRequest)
		return
	}

	signer, err := wallet.ImportFromPrivateKey(request.PrivateKey)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Failed to import wallet: %v"}`, err), http.StatusBadRequest)
		return
	}

	// Execute burn directly on the node and register a bridge request.
	payload := map[string]any{
		"chain_id":        bridgeChainIDFromValue(request.ChainID),
		"family":          strings.TrimSpace(request.Family),
		"adapter":         strings.TrimSpace(request.Adapter),
		"token":           request.Token,
		"from":            signer.Address,
		"to_addr":         request.ToBsc,
		"amount":          request.Amount.Int.String(),
		"source_tx_hash":  request.SourceTxHash,
		"source_address":  request.SourceAddress,
		"source_memo":     request.SourceMemo,
		"source_sequence": request.SourceSequence,
		"source_output":   request.SourceOutput,
		"mode":            mode,
	}
	body, _ := json.Marshal(payload)
	resp, err := http.Post(ws.BlockchainNodeAddress+"/bridge/burn_chain", "application/json", bytes.NewReader(body))
	if err != nil {
		http.Error(w, `{"error":"Blockchain node unreachable"}`, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (ws *WalletServer) BridgeBurnLqdToken(w http.ResponseWriter, r *http.Request) {
	ws.bridgeBurnLqdTokenWithMode(w, r, "")
}

func (ws *WalletServer) BridgePrivateBurnLqdToken(w http.ResponseWriter, r *http.Request) {
	ws.bridgeBurnLqdTokenWithMode(w, r, "private")
}

func (ws *WalletServer) BridgeBurn(w http.ResponseWriter, r *http.Request) {
	ws.bridgeBurnWithMode(w, r, "")
}

func (ws *WalletServer) BridgePrivateBurn(w http.ResponseWriter, r *http.Request) {
	ws.bridgeBurnWithMode(w, r, "private")
}

func (ws *WalletServer) SendTransactionBatch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
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
		From       string `json:"from"`
		To         string `json:"to"`
		Value      Amount `json:"value"`
		Data       string `json:"data"`
		Gas        uint64 `json:"gas"`
		GasPrice   uint64 `json:"gas_price"`
		PrivateKey string `json:"private_key"`
		Count      int    `json:"count"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, `{"error":"Invalid request format"}`, http.StatusBadRequest)
		return
	}
	if request.Count <= 0 {
		http.Error(w, `{"error":"count must be > 0"}`, http.StatusBadRequest)
		return
	}

	if !wallet.ValidateAddress(request.From) || !wallet.ValidateAddress(request.To) {
		http.Error(w, `{"error":"Invalid address format"}`, http.StatusBadRequest)
		return
	}

	signer, err := wallet.ImportFromPrivateKey(request.PrivateKey)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Failed to import wallet: %v"}`, err), http.StatusBadRequest)
		return
	}
	if !strings.EqualFold(signer.Address, request.From) {
		http.Error(w, `{"error":"From address does not match private key"}`, http.StatusBadRequest)
		return
	}

	var baseData []byte
	if request.Data != "" {
		if strings.HasPrefix(request.Data, "0x") || strings.HasPrefix(request.Data, "0X") {
			db, derr := hex.DecodeString(request.Data[2:])
			if derr != nil {
				http.Error(w, `{"error":"Invalid hex in 'data'"}`, http.StatusBadRequest)
				return
			}
			baseData = db
		} else {
			baseData = []byte(request.Data)
		}
	}

	minBaseFee := uint64(constantset.InitialBaseFee)
	gas := request.Gas
	if gas == 0 {
		gas = uint64(constantset.MinGas)
	}
	gasPrice := request.GasPrice
	if gasPrice < minBaseFee {
		gasPrice = minBaseFee
	}

	client := &http.Client{Timeout: 10 * time.Second}
	baseNonce, err := ws.fetchNextNonce(client, request.From)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Failed to get nonce from blockchain: %v"}`, err), http.StatusBadGateway)
		return
	}
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
		Balance string `json:"balance"`
	}
	if err := json.NewDecoder(balResp.Body).Decode(&bal); err != nil {
		http.Error(w, `{"error":"Failed to decode balance response"}`, http.StatusInternalServerError)
		return
	}
	balInt, err := blockchaincomponent.NewAmountFromString(bal.Balance)
	if err != nil {
		http.Error(w, `{"error":"Failed to parse balance"}`, http.StatusInternalServerError)
		return
	}
	fee := new(big.Int).SetUint64(gas * gasPrice)
	valueAmt := request.Value.Int
	if valueAmt == nil {
		valueAmt = new(big.Int)
	}
	total := new(big.Int).Add(valueAmt, fee)
	totalNeeded := new(big.Int).Mul(total, new(big.Int).SetInt64(int64(request.Count)))
	if balInt.Cmp(totalNeeded) < 0 {
		http.Error(w, fmt.Sprintf(`{"error":"Insufficient funds: balance=%s required=%s"}`, balInt.String(), totalNeeded.String()), http.StatusBadRequest)
		return
	}

	txs := make([]blockchaincomponent.Transaction, 0, request.Count)
	for i := 0; i < request.Count; i++ {
		dataBytes := make([]byte, 0, len(baseData)+16)
		dataBytes = append(dataBytes, baseData...)
		dataBytes = append(dataBytes, []byte(fmt.Sprintf("|%d", i))...)

		tx := blockchaincomponent.NewTransaction(
			request.From,
			request.To,
			valueAmt,
			dataBytes,
		)
		tx.Gas = gas
		tx.GasPrice = gasPrice
		tx.Nonce = baseNonce + uint64(i)
		tx.ChainID = uint64(constantset.ChainID)

		if err := signer.SignTransaction(tx); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"Failed to sign transaction: %v"}`, err), http.StatusInternalServerError)
			return
		}
		txs = append(txs, *tx)
	}

	txJSON, err := json.Marshal(txs)
	if err != nil {
		http.Error(w, `{"error":"Failed to marshal transactions"}`, http.StatusInternalServerError)
		return
	}
	resp, err := client.Post(ws.BlockchainNodeAddress+"/send_tx/batch", "application/json", bytes.NewBuffer(txJSON))
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Failed to send batch: %v"}`, err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (ws *WalletServer) ContractTemplate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
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
		Address         string   `json:"address"`
		ContractAddress string   `json:"contract_address"`
		Function        string   `json:"function"`
		Args            []string `json:"args"`
		Value           Amount   `json:"value"`
		Gas             uint64   `json:"gas"`
		GasPrice        uint64   `json:"gas_price"`
		PrivateKey      string   `json:"private_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid request format"}`, http.StatusBadRequest)
		return
	}

	if !wallet.ValidateAddress(req.Address) || !wallet.ValidateAddress(req.ContractAddress) {
		http.Error(w, `{"error":"Invalid address format"}`, http.StatusBadRequest)
		return
	}
	if req.Function == "" {
		http.Error(w, `{"error":"Function name is required"}`, http.StatusBadRequest)
		return
	}

	signer, err := wallet.ImportFromPrivateKey(req.PrivateKey)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Failed to import wallet: %v"}`, err), http.StatusBadRequest)
		return
	}
	if !strings.EqualFold(signer.Address, req.Address) {
		http.Error(w, `{"error":"Address does not match private key"}`, http.StatusBadRequest)
		return
	}

	payload := map[string]any{
		"fn":   req.Function,
		"args": req.Args,
	}
	dataBytes, _ := json.Marshal(payload)

	valueAmt := req.Value.Int
	if valueAmt == nil {
		valueAmt = new(big.Int)
	}
	tx := &blockchaincomponent.Transaction{
		From:       req.Address,
		To:         req.ContractAddress,
		Value:      valueAmt,
		Data:       dataBytes,
		Type:       "contract_call",
		Function:   req.Function,
		Args:       req.Args,
		IsContract: true,
		ChainID:    uint64(constantset.ChainID),
		Gas:        uint64(constantset.ContractCallGas),
		GasPrice:   1,
		Timestamp:  uint64(time.Now().Unix()),
		Status:     constantset.StatusPending,
	}
	if req.Gas > 0 {
		tx.Gas = req.Gas
	}
	if req.GasPrice > 0 {
		tx.GasPrice = req.GasPrice
	} else {
		feeURL := ws.BlockchainNodeAddress + "/basefee"
		if resp, err := http.Get(feeURL); err == nil {
			defer resp.Body.Close()
			var feeResp struct {
				BaseFee uint64 `json:"base_fee"`
			}
			if json.NewDecoder(resp.Body).Decode(&feeResp) == nil && feeResp.BaseFee > 0 {
				tx.GasPrice = feeResp.BaseFee + 1
			}
		}
	}
	txNonce, err := ws.fetchNextNonce(&http.Client{Timeout: 10 * time.Second}, req.Address)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Failed to get nonce from blockchain: %v"}`, err), http.StatusBadGateway)
		return
	}
	tx.Nonce = txNonce

	if err := signer.SignTransaction(tx); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Failed to sign tx: %v"}`, err), http.StatusBadRequest)
		return
	}
	tx.TxHash = blockchaincomponent.CalculateTransactionHash(*tx)

	body, _ := json.Marshal(tx)
	resp, err := http.Post(ws.BlockchainNodeAddress+"/send_tx", "application/json", bytes.NewReader(body))
	if err != nil {
		http.Error(w, `{"error":"Blockchain node unreachable"}`, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (ws *WalletServer) GetTokenBalance(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
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
		"address": contract,
		"caller":  holder,
		"fn":      "balanceOf",
		"args":    []string{holder},
		"value":   0,
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

// apiKeyMiddleware checks X-API-Key header against LQD_API_KEY env var.
// If LQD_API_KEY is not set, all requests are allowed (dev mode).
func apiKeyMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// CORS preflight always passes
		if r.Method == http.MethodOptions {
			next(w, r)
			return
		}
		requiredKey := os.Getenv("LQD_API_KEY")
		if requiredKey != "" {
			clientKey := r.Header.Get("X-API-Key")
			if clientKey == "" {
				clientKey = r.URL.Query().Get("api_key")
			}
			if clientKey != requiredKey {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error":"invalid or missing API key"}`))
				return
			}
		}
		next(w, r)
	}
}

func (ws *WalletServer) Start() {
	portStr := fmt.Sprintf("%d", ws.Port)
	http.HandleFunc("/wallet/new", ws.CreateNewWallet)
	http.HandleFunc("/wallet/import/mnemonic", ws.ImportFromMnemonic)
	http.HandleFunc("/wallet/import/private-key", ws.ImportFromPrivateKey)
	http.HandleFunc("/wallet/balance", ws.GetBalance)
	http.HandleFunc("/wallet/send", ws.SendTransaction)
	http.HandleFunc("/wallet/send_batch", ws.SendTransactionBatch)
	http.HandleFunc("/wallet/contract-template", ws.ContractTemplate)
	http.HandleFunc("/wallet/bridge/lock", ws.BridgeLock)
	http.HandleFunc("/wallet/bridge/private/lock", ws.BridgePrivateLock)
	http.HandleFunc("/wallet/bridge/burn", ws.BridgeBurn)
	http.HandleFunc("/wallet/bridge/private/burn", ws.BridgePrivateBurn)
	http.HandleFunc("/wallet/bridge/lock_bsc_token", ws.BridgeLockBscToken)
	http.HandleFunc("/wallet/bridge/private/lock_bsc_token", ws.BridgePrivateLockBscToken)
	http.HandleFunc("/wallet/bridge/burn_lqd_token", ws.BridgeBurnLqdToken)
	http.HandleFunc("/wallet/bridge/private/burn_lqd_token", ws.BridgePrivateBurnLqdToken)
	http.HandleFunc("/wallet/bridge/bsc_lock_tx", ws.BridgeBscLockTxData)
	http.HandleFunc("/wallet/token-balance", ws.GetTokenBalance)

	log.Printf("Starting wallet server on port %d\n", ws.Port)
	log.Printf("Connected to blockchain node at %s\n", ws.BlockchainNodeAddress)

	if err := http.ListenAndServe(":"+portStr, nil); err != nil {
		log.Fatalf("Failed to start wallet server: %v", err)
	}
}
