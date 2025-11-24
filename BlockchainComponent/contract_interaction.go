package blockchaincomponent

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

/*
   CONTRACT INTERACTION LAYER (WALLET-FRIENDLY)

   Wallet does not encode ABI.
   Wallet only sends function + args.
   Node handles executing all underlying VM types.
*/

type ContractInteractionAPI struct {
	BlockchainPtr *Blockchain_struct
}

func NewContractInteractionAPI(bc *Blockchain_struct) *ContractInteractionAPI {
	return &ContractInteractionAPI{BlockchainPtr: bc}
}

func (api *ContractInteractionAPI) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/contract/info", api.ContractInfo).Methods("GET")
	r.HandleFunc("/contract/functions", api.ContractFunctions).Methods("GET")
	r.HandleFunc("/contract/prepare-call", api.PrepareCall).Methods("POST")
}

///////////////////////////////////////////////////////////////////////////////
// GET CONTRACT INFO
//
// GET /contract/info?address=0x123
///////////////////////////////////////////////////////////////////////////////

func (api *ContractInteractionAPI) ContractInfo(w http.ResponseWriter, r *http.Request) {
	addr := r.URL.Query().Get("address")

	rec, err := api.BlockchainPtr.ContractEngine.Registry.LoadContract(addr)
	if err != nil {
		http.Error(w, err.Error(), 404)
		return
	}

	json.NewEncoder(w).Encode(map[string]any{
		"address": rec.Metadata.Address,
		"type":    rec.Metadata.Type,
		"owner":   rec.Metadata.Owner,
		"created": rec.Metadata.Timestamp,
	})
}

///////////////////////////////////////////////////////////////////////////////
// GET FUNCTION LIST (ABI-LITE VERSION)
//
// GET /contract/functions?address=0x123
///////////////////////////////////////////////////////////////////////////////

func (api *ContractInteractionAPI) ContractFunctions(w http.ResponseWriter, r *http.Request) {
	addr := r.URL.Query().Get("address")

	abi, err := api.BlockchainPtr.ContractEngine.Registry.LoadABI(addr)
	if err != nil {
		http.Error(w, err.Error(), 404)
		return
	}

	w.Write(abi)
}

///////////////////////////////////////////////////////////////////////////////
// PREPARE CONTRACT CALL (UNSIGNED TX)
//
// POST /contract/prepare-call
//
// BODY:
// {
//   "from": "0xUSER",
//   "contract": "0xCONTRACT",
//   "function": "mint",
//   "args": ["1000"]
// }
///////////////////////////////////////////////////////////////////////////////

func (api *ContractInteractionAPI) PrepareCall(w http.ResponseWriter, r *http.Request) {
	type Req struct {
		From     string   `json:"from"`
		Contract string   `json:"contract"`
		Fn       string   `json:"function"`
		Args     []string `json:"args"`
		Value    uint64   `json:"value"`
	}

	var req Req
	json.NewDecoder(r.Body).Decode(&req)

	//nonce := api.BlockchainPtr.GetNextNonce(req.From)

	template := map[string]any{
		"from":     req.From,
		"to":       req.Contract,
		"value":    req.Value,
		"gas":      uint64(300000), // default
		"gasPrice": uint64(1),
		//"nonce":          nonce,
		"isContractCall": true,
		"function":       req.Fn,
		"args":           req.Args,
		"timestamp":      time.Now().Unix(),
	}

	json.NewEncoder(w).Encode(template)
}
