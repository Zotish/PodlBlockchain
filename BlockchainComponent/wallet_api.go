package blockchaincomponent

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

type WalletAPIServer struct {
	BlockchainPtr *Blockchain_struct
}

func NewWalletAPIServer(bc *Blockchain_struct) *WalletAPIServer {
	return &WalletAPIServer{BlockchainPtr: bc}
}

func (api *WalletAPIServer) RegisterRoutes(r *mux.Router) {
	//r.HandleFunc("/wallet/nonce", api.GetNonce).Methods("GET")
	r.HandleFunc("/wallet/tx-template", api.TxTemplate).Methods("POST")
	r.HandleFunc("/wallet/contract-template", api.ContractTemplate).Methods("POST")
	r.HandleFunc("/wallet/abi", api.GetABI).Methods("GET")
	r.HandleFunc("/wallet/gas-estimate", api.GasEstimate).Methods("POST")
}

///////////////////////////////////////////////////////////////////////////////
//  GET NONCE
//
//  GET /wallet/nonce?address=0x123
///////////////////////////////////////////////////////////////////////////////

// func (api *WalletAPIServer) GetNonce(w http.ResponseWriter, r *http.Request) {
// 	addr := r.URL.Query().Get("address")
// 	if addr == "" {
// 		http.Error(w, "address required", 400)
// 		return
// 	}

// 	nonce := api.BlockchainPtr.GetNextNonce(addr)

// 	json.NewEncoder(w).Encode(map[string]any{
// 		"address": addr,
// 		"nonce":   nonce,
// 	})
// }

///////////////////////////////////////////////////////////////////////////////
//  GAS ESTIMATE FOR NATIVE TRANSFER
//
//  POST /wallet/gas-estimate
//  BODY: { "from": "...", "to": "...", "value": 100 }
///////////////////////////////////////////////////////////////////////////////

func (api *WalletAPIServer) GasEstimate(w http.ResponseWriter, r *http.Request) {
	type Req struct {
		From  string `json:"from"`
		To    string `json:"to"`
		Value uint64 `json:"value"`
	}

	var req Req
	json.NewDecoder(r.Body).Decode(&req)

	estimate := uint64(21000)

	json.NewEncoder(w).Encode(map[string]any{
		"gas": estimate,
	})
}

///////////////////////////////////////////////////////////////////////////////
//  RAW TX TEMPLATE (UNSIGNED)
//
//  POST /wallet/tx-template
//  BODY:
//  {
//     "from": "0xA",
//     "to": "0xB",
//     "value": 1000,
//     "gas": 50000,
//     "gasPrice": 1
//  }
///////////////////////////////////////////////////////////////////////////////

func (api *WalletAPIServer) TxTemplate(w http.ResponseWriter, r *http.Request) {
	type Req struct {
		From     string `json:"from"`
		To       string `json:"to"`
		Value    uint64 `json:"value"`
		Gas      uint64 `json:"gas"`
		GasPrice uint64 `json:"gasPrice"`
	}

	var req Req
	json.NewDecoder(r.Body).Decode(&req)

	//nonce := api.BlockchainPtr.GetNextNonce(req.From)

	tx := map[string]any{
		"from":     req.From,
		"to":       req.To,
		"value":    req.Value,
		"gas":      req.Gas,
		"gasPrice": req.GasPrice,
		//"nonce":     nonce,
		"timestamp": uint64(time.Now().Unix()),
	}

	json.NewEncoder(w).Encode(tx)
}

///////////////////////////////////////////////////////////////////////////////
//  CONTRACT CALL TEMPLATE (UNSIGNED)
//
//  POST /wallet/contract-template
//  BODY:
//  {
//     "from": "0xUSER",
//     "contract": "0xCONTRACT",
//     "function": "transfer",
//     "args": ["0xABC", "50"],
//     "gas": 200000,
//     "gasPrice": 1,
//     "value": 0
//  }
///////////////////////////////////////////////////////////////////////////////

func (api *WalletAPIServer) ContractTemplate(w http.ResponseWriter, r *http.Request) {
	type Req struct {
		From     string   `json:"from"`
		Contract string   `json:"contract"`
		Fn       string   `json:"function"`
		Args     []string `json:"args"`
		Gas      uint64   `json:"gas"`
		GasPrice uint64   `json:"gasPrice"`
		Value    uint64   `json:"value"`
	}

	var req Req
	json.NewDecoder(r.Body).Decode(&req)

	//nonce := api.BlockchainPtr.GetNextNonce(req.From)

	tx := map[string]any{
		"from":     req.From,
		"to":       req.Contract,
		"value":    req.Value,
		"gas":      req.Gas,
		"gasPrice": req.GasPrice,
		//"nonce":          nonce,
		"isContractCall": true,
		"function":       req.Fn,
		"args":           req.Args,
		"data": append([][]byte{
			[]byte(req.Fn),
		}, encodeArgs(req.Args)...),
	}

	json.NewEncoder(w).Encode(tx)
}

func encodeArgs(args []string) [][]byte {
	out := [][]byte{}
	for _, a := range args {
		out = append(out, []byte(a))
	}
	return out
}

///////////////////////////////////////////////////////////////////////////////
//  GET CONTRACT ABI
//
//  GET /wallet/abi?address=0x123
///////////////////////////////////////////////////////////////////////////////

func (api *WalletAPIServer) GetABI(w http.ResponseWriter, r *http.Request) {
	addr := r.URL.Query().Get("address")
	abi, err := api.BlockchainPtr.ContractEngine.Registry.LoadABI(addr)

	if err != nil {
		http.Error(w, err.Error(), 404)
		return
	}

	w.Write(abi)
}
