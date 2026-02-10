package blockchaincomponent

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
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

// Save stores a high-level Contract into the registry/DB.
// It converts Contract -> ContractMetadata + SmartContractState and reuses RegisterContract.
func (reg *ContractRegistry) Save(c *Contract) {
	if reg == nil || reg.DB == nil || c == nil {
		return
	}

	// Marshal ABI entries into the stored ABI bytes
	var abiBytes []byte
	if len(c.ABI) > 0 {
		abi := ContractABI{Entries: c.ABI}
		abiBytes, _ = json.Marshal(abi)
	}

	meta := &ContractMetadata{
		Address:   c.Address,
		Type:      c.Type,
		Owner:     "", // unknown here; can be filled elsewhere if you track owners
		ABI:       abiBytes,
		Timestamp: time.Now().Unix(),
	}

	// SourceCode / Bytecode -> Code field (you can adjust this depending on your usage)
	if c.SourceCode != "" {
		meta.Code = []byte(c.SourceCode)
	}
	if c.PluginPath != "" {
		meta.PluginPath = c.PluginPath
	}

	state := &SmartContractState{
		Address:   c.Address,
		Balance:   "0",
		Storage:   make(map[string]string),
		IsActive:  true,
		CreatedAt: time.Now().Unix(),
	}

	// Flatten Contract.State (map[string]interface{}) into string storage
	for k, v := range c.State {
		state.Storage[k] = fmt.Sprintf("%v", v)
	}

	// Reuse existing logic that writes metadata + storage
	_ = reg.RegisterContract(meta, state)
}

// Load reconstructs a high-level Contract from the registry/DB.
// It loads ContractRecord and maps it back to Contract.
func (reg *ContractRegistry) Load(address string) *Contract {
	if reg == nil || reg.DB == nil {
		return nil
	}

	rec, err := reg.LoadContract(address)
	if err != nil || rec == nil || rec.Metadata == nil || rec.State == nil {
		return nil
	}

	// Decode ABI
	var entries []ABIEntry
	if len(rec.Metadata.ABI) > 0 {
		var cabi ContractABI
		if err := json.Unmarshal(rec.Metadata.ABI, &cabi); err == nil {
			entries = cabi.Entries
		}
	}

	// Map storage back to map[string]interface{}
	state := make(map[string]interface{}, len(rec.State.Storage))
	for k, v := range rec.State.Storage {
		state[k] = v
	}

	return &Contract{
		Address:    rec.Metadata.Address,
		Type:       rec.Metadata.Type,
		ABI:        entries,
		InitParams: nil, // not tracked separately in your current structs
		SourceCode: string(rec.Metadata.Code),
		Bytecode:   "", // you can fill this if you separately store bytecode
		PluginPath: rec.Metadata.PluginPath,
		State:      state,
	}
}

func NewContractInteractionAPI(bc *Blockchain_struct) *ContractInteractionAPI {
	return &ContractInteractionAPI{BlockchainPtr: bc}
}

func (api *ContractInteractionAPI) RegisterRoutes(r *mux.Router) {
	http.HandleFunc("/contract/info", api.ContractInfo)
	http.HandleFunc("/contract/functions", api.ContractFunctions)
	http.HandleFunc("/contract/prepare-call", api.PrepareCall)
}

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

func (api *ContractInteractionAPI) ContractFunctions(w http.ResponseWriter, r *http.Request) {
	addr := r.URL.Query().Get("address")

	abi, err := api.BlockchainPtr.ContractEngine.Registry.LoadABI(addr)
	if err != nil {
		http.Error(w, err.Error(), 404)
		return
	}

	w.Write(abi)
}

func (api *ContractInteractionAPI) PrepareCall(w http.ResponseWriter, r *http.Request) {
	type Req struct {
		From     string   `json:"from"`
		Contract string   `json:"contract"`
		Fn       string   `json:"function"`
		Args     []string `json:"args"`
		Value    string   `json:"value"`
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
func (r *ContractRegistry) List() []string {
	iter := r.DB.db.NewIterator(nil, nil)
	defer iter.Release()

	out := []string{}
	prefix := "contract:"

	for iter.Next() {
		key := string(iter.Key())
		if strings.HasPrefix(key, prefix) && strings.HasSuffix(key, ":meta") {
			addr := strings.TrimPrefix(key, "contract:")
			addr = strings.TrimSuffix(addr, ":meta")
			out = append(out, addr)
		}
	}

	return out
}
