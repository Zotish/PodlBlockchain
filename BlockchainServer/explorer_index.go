package blockchainserver

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	blockchaincomponent "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/BlockchainComponent"
	wallet "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/WalletComponent"
)

const explorerZeroAddress = "0x0000000000000000000000000000000000000000"

func explorerPathParam(r *http.Request, marker string) string {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	for i := 0; i+1 < len(parts); i++ {
		if strings.EqualFold(parts[i], marker) {
			return strings.TrimSpace(parts[i+1])
		}
	}
	return ""
}

func explorerStorageValue(storage map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(storage[key]); value != "" {
			return value
		}
	}
	for candidate, value := range storage {
		for _, key := range keys {
			if strings.EqualFold(candidate, key) && strings.TrimSpace(value) != "" {
				return value
			}
		}
	}
	return ""
}

func explorerHasPositive(raw string) bool {
	n := new(big.Int)
	if _, ok := n.SetString(strings.TrimSpace(raw), 10); !ok {
		return false
	}
	return n.Sign() > 0
}

func explorerTokenBalance(storage map[string]string, owner string) string {
	owner = strings.TrimSpace(owner)
	lowerOwner := strings.ToLower(owner)
	keys := []string{
		"balance:" + owner,
		"balance:" + lowerOwner,
		"balances:" + owner,
		"balances:" + lowerOwner,
		"bal:" + owner,
		"bal:" + lowerOwner,
		"__bal:" + owner,
		"__bal:" + lowerOwner,
		owner,
		lowerOwner,
	}
	if value := explorerStorageValue(storage, keys...); value != "" {
		return value
	}
	for key, value := range storage {
		k := strings.ToLower(key)
		if strings.Contains(k, lowerOwner) && (strings.Contains(k, "bal") || strings.Contains(k, "holder")) {
			return value
		}
	}
	return "0"
}

func explorerParseHolders(storage map[string]string, decimals int) []map[string]interface{} {
	holders := map[string]*big.Int{}
	add := func(address, raw string) {
		address = strings.TrimSpace(address)
		if !wallet.ValidateAddress(address) {
			return
		}
		amount := new(big.Int)
		if _, ok := amount.SetString(strings.TrimSpace(raw), 10); !ok || amount.Sign() <= 0 {
			return
		}
		key := strings.ToLower(address)
		if holders[key] == nil {
			holders[key] = big.NewInt(0)
		}
		holders[key].Add(holders[key], amount)
	}
	for key, value := range storage {
		if wallet.ValidateAddress(key) {
			add(key, value)
			continue
		}
		if strings.HasPrefix(strings.ToLower(key), "owner:") || strings.HasPrefix(strings.ToLower(key), "ownerof:") {
			continue
		}
		for _, part := range strings.Split(strings.NewReplacer(":", " ", "_", " ", "/", " ").Replace(key), " ") {
			if wallet.ValidateAddress(part) {
				add(part, value)
				break
			}
		}
	}
	out := make([]map[string]interface{}, 0, len(holders))
	for address, amount := range holders {
		out = append(out, map[string]interface{}{
			"address": address,
			"amount":  amount.String(),
			"rank":    0,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		ai, _ := new(big.Int).SetString(out[i]["amount"].(string), 10)
		aj, _ := new(big.Int).SetString(out[j]["amount"].(string), 10)
		return ai.Cmp(aj) > 0
	})
	for i := range out {
		out[i]["rank"] = i + 1
		out[i]["decimals"] = decimals
	}
	return out
}

func explorerNFTsForOwner(storage map[string]string, owner string) []string {
	owner = strings.ToLower(strings.TrimSpace(owner))
	tokenIDs := []string{}
	for key, value := range storage {
		k := strings.ToLower(key)
		if (strings.HasPrefix(k, "owner:") || strings.HasPrefix(k, "ownerof:")) && strings.EqualFold(value, owner) {
			tokenIDs = append(tokenIDs, strings.Join(strings.Split(key, ":")[1:], ":"))
		}
	}
	sort.Strings(tokenIDs)
	return tokenIDs
}

func explorerContractSummary(addr string, rec *blockchaincomponent.ContractRecord) map[string]interface{} {
	if rec == nil || rec.Metadata == nil {
		return nil
	}
	meta := rec.Metadata
	storage := map[string]string{}
	if rec.State != nil && rec.State.Storage != nil {
		storage = rec.State.Storage
	}
	abiCount := 0
	if len(meta.ABI) > 0 {
		var wrapped struct {
			Entries []interface{} `json:"entries"`
		}
		var entries []interface{}
		if json.Unmarshal(meta.ABI, &wrapped) == nil && wrapped.Entries != nil {
			abiCount = len(wrapped.Entries)
		} else if json.Unmarshal(meta.ABI, &entries) == nil {
			abiCount = len(entries)
		}
	}
	codeHash := ""
	if len(meta.Code) > 0 {
		sum := sha256.Sum256(meta.Code)
		codeHash = hex.EncodeToString(sum[:])
	}
	return map[string]interface{}{
		"address":             addr,
		"type":                meta.Type,
		"owner":               meta.Owner,
		"timestamp":           meta.Timestamp,
		"pool":                meta.Pool,
		"pool_type":           meta.PoolType,
		"builtin_name":        meta.BuiltinName,
		"runtime_fingerprint": meta.RuntimeFingerprint,
		"name":                explorerStorageValue(storage, "name", "Name", "token_name", "TokenName"),
		"symbol":              explorerStorageValue(storage, "symbol", "Symbol", "ticker", "Ticker"),
		"decimals":            explorerStorageValue(storage, "decimals", "Decimals"),
		"totalSupply":         explorerStorageValue(storage, "totalSupply", "total_supply", "TotalSupply", "supply", "Supply"),
		"verified":            explorerStorageValue(storage, "__verified") == "true",
		"source_hash":         explorerStorageValue(storage, "__source_hash"),
		"code_hash":           codeHash,
		"abi_count":           abiCount,
		"storage_keys":        len(storage),
		"has_source":          len(meta.Code) > 0,
	}
}

func explorerTxTouchesAddress(tx *blockchaincomponent.Transaction, addr string) bool {
	if txTouchesAddress(tx, addr) {
		return true
	}
	needle := strings.ToLower(strings.TrimSpace(addr))
	if needle == "" || tx == nil {
		return false
	}
	for _, arg := range tx.Args {
		if strings.Contains(strings.ToLower(arg), needle) {
			return true
		}
	}
	return strings.Contains(strings.ToLower(string(tx.Data)), needle)
}

func explorerInternalTx(tx *blockchaincomponent.Transaction) bool {
	if tx == nil {
		return false
	}
	return isExplorerInternalTx(tx) || strings.EqualFold(tx.From, explorerZeroAddress) || tx.IsSystem
}

func (bcs *BlockchainServer) ExplorerAddressOverview(w http.ResponseWriter, r *http.Request) {
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
	address := explorerPathParam(r, "address")
	if !wallet.ValidateAddress(address) {
		http.Error(w, `{"error":"invalid address format"}`, http.StatusBadRequest)
		return
	}
	addr := strings.ToLower(address)

	bc := bcs.BlockchainPtr
	transactions := []map[string]interface{}{}
	pending := []map[string]interface{}{}
	internal := []map[string]interface{}{}
	tokenTxs := []map[string]interface{}{}
	daily := map[string]map[string]interface{}{}

	bc.Mutex.Lock()
	for _, tx := range bc.Transaction_pool {
		if !explorerTxTouchesAddress(tx, addr) {
			continue
		}
		item := transactionListItem(tx, "mempool", nil, -1)
		transactions = append(transactions, item)
		pending = append(pending, item)
		if explorerInternalTx(tx) {
			internal = append(internal, item)
		}
	}
	for i := len(bc.Blocks) - 1; i >= 0; i-- {
		blk := bc.Blocks[i]
		if blk == nil {
			continue
		}
		for idx := len(blk.Transactions) - 1; idx >= 0; idx-- {
			tx := blk.Transactions[idx]
			if !explorerTxTouchesAddress(tx, addr) {
				continue
			}
			item := transactionListItem(tx, "block", blk, idx)
			transactions = append(transactions, item)
			if explorerInternalTx(tx) {
				internal = append(internal, item)
			}
			if strings.Contains(strings.ToLower(tx.Type), "token") || strings.Contains(strings.ToLower(tx.Function), "transfer") {
				tokenTxs = append(tokenTxs, item)
			}
			day := time.Unix(int64(tx.Timestamp), 0).UTC().Format("2006-01-02")
			if daily[day] == nil {
				daily[day] = map[string]interface{}{"date": day, "count": 0, "volume": "0"}
			}
			daily[day]["count"] = daily[day]["count"].(int) + 1
			vol, _ := new(big.Int).SetString(daily[day]["volume"].(string), 10)
			if tx.Value != nil {
				vol.Add(vol, tx.Value)
			}
			daily[day]["volume"] = vol.String()
		}
	}
	bc.Mutex.Unlock()

	sort.Slice(transactions, func(i, j int) bool {
		return explorerInt64(transactions[i]["timestamp"]) > explorerInt64(transactions[j]["timestamp"])
	})
	contracts := []map[string]interface{}{}
	tokenBalances := []map[string]interface{}{}
	nfts := []map[string]interface{}{}
	var contractInfo map[string]interface{}
	if bc.ContractEngine != nil && bc.ContractEngine.Registry != nil {
		for _, contractAddr := range bc.ContractEngine.Registry.List() {
			rec, err := bc.ContractEngine.Registry.LoadContract(contractAddr)
			if err != nil || rec == nil || rec.Metadata == nil {
				continue
			}
			summary := explorerContractSummary(contractAddr, rec)
			if summary == nil {
				continue
			}
			contracts = append(contracts, summary)
			if strings.EqualFold(contractAddr, address) {
				contractInfo = summary
			}
			storage := map[string]string{}
			if rec.State != nil && rec.State.Storage != nil {
				storage = rec.State.Storage
			}
			typeText := strings.ToLower(rec.Metadata.Type + " " + rec.Metadata.BuiltinName + " " + explorerStorageValue(storage, "symbol", "Symbol"))
			if strings.Contains(typeText, "token") || explorerStorageValue(storage, "totalSupply", "TotalSupply", "decimals", "Decimals") != "" {
				bal := explorerTokenBalance(storage, address)
				if explorerHasPositive(bal) {
					tokenBalances = append(tokenBalances, map[string]interface{}{
						"address":  contractAddr,
						"name":     summary["name"],
						"symbol":   summary["symbol"],
						"decimals": summary["decimals"],
						"balance":  bal,
						"verified": summary["verified"],
					})
				}
			}
			ids := explorerNFTsForOwner(storage, address)
			if len(ids) > 0 {
				nfts = append(nfts, map[string]interface{}{
					"address":   contractAddr,
					"name":      summary["name"],
					"symbol":    summary["symbol"],
					"count":     len(ids),
					"token_ids": ids,
				})
			}
		}
	}
	dailyRows := make([]map[string]interface{}, 0, len(daily))
	for _, row := range daily {
		dailyRows = append(dailyRows, row)
	}
	sort.Slice(dailyRows, func(i, j int) bool { return dailyRows[i]["date"].(string) < dailyRows[j]["date"].(string) })
	if len(dailyRows) > 30 {
		dailyRows = dailyRows[len(dailyRows)-30:]
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"address":               address,
		"balance":               bc.AccountBalanceString(address),
		"confirmed_balance":     bc.AccountBalanceString(address),
		"is_contract":           contractInfo != nil,
		"contract":              contractInfo,
		"transactions":          transactions,
		"pending_transactions":  pending,
		"internal_transactions": internal,
		"token_transactions":    tokenTxs,
		"token_balances":        tokenBalances,
		"nft_balances":          nfts,
		"analytics": map[string]interface{}{
			"tx_count":       len(transactions),
			"pending_count":  len(pending),
			"internal_count": len(internal),
			"token_tx_count": len(tokenTxs),
			"daily":          dailyRows,
			"contracts_seen": len(contracts),
		},
		"timestamp": time.Now().Unix(),
	})
}

func (bcs *BlockchainServer) ExplorerTokenHolders(w http.ResponseWriter, r *http.Request) {
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
	address := explorerPathParam(r, "token")
	if !wallet.ValidateAddress(address) {
		http.Error(w, `{"error":"invalid token address"}`, http.StatusBadRequest)
		return
	}
	rec, err := bcs.BlockchainPtr.ContractEngine.Registry.LoadContract(address)
	if err != nil || rec == nil || rec.State == nil {
		http.Error(w, `{"error":"token contract not found"}`, http.StatusNotFound)
		return
	}
	storage := rec.State.Storage
	decimals, _ := strconv.Atoi(explorerStorageValue(storage, "decimals", "Decimals"))
	if decimals <= 0 {
		decimals = 8
	}
	holders := explorerParseHolders(storage, decimals)
	if len(holders) > 500 {
		holders = holders[:500]
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token":         explorerContractSummary(address, rec),
		"holders":       holders,
		"total_holders": len(holders),
		"timestamp":     time.Now().Unix(),
	})
}

func (bcs *BlockchainServer) ContractVerificationStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	setCORSHeaders(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	address := strings.TrimSpace(r.URL.Query().Get("address"))
	if !wallet.ValidateAddress(address) {
		http.Error(w, `{"error":"invalid address"}`, http.StatusBadRequest)
		return
	}
	rec, err := bcs.BlockchainPtr.ContractEngine.Registry.LoadContract(address)
	if err != nil || rec == nil {
		http.Error(w, `{"error":"contract not found"}`, http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"contract": explorerContractSummary(address, rec)})
}

func (bcs *BlockchainServer) VerifyContractSource(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	setCORSHeaders(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Address         string `json:"address"`
		SourceCode      string `json:"source_code"`
		CompilerVersion string `json:"compiler_version"`
		Optimization    bool   `json:"optimization"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}
	if !wallet.ValidateAddress(req.Address) || strings.TrimSpace(req.SourceCode) == "" {
		http.Error(w, `{"error":"address and source_code required"}`, http.StatusBadRequest)
		return
	}
	rec, err := bcs.BlockchainPtr.ContractEngine.Registry.LoadContract(req.Address)
	if err != nil || rec == nil || rec.Metadata == nil {
		http.Error(w, `{"error":"contract not found"}`, http.StatusNotFound)
		return
	}
	sourceSum := sha256.Sum256([]byte(req.SourceCode))
	sourceHash := hex.EncodeToString(sourceSum[:])
	storage := map[string]string{}
	if rec.State != nil && rec.State.Storage != nil {
		storage = rec.State.Storage
	}
	match := false
	if len(rec.Metadata.Code) > 0 {
		codeSum := sha256.Sum256(rec.Metadata.Code)
		match = sourceHash == hex.EncodeToString(codeSum[:])
	} else {
		match = strings.EqualFold(explorerStorageValue(storage, "__source_hash"), sourceHash)
	}
	if !match && len(rec.Metadata.Code) > 0 {
		http.Error(w, `{"error":"source hash does not match deployed code"}`, http.StatusBadRequest)
		return
	}
	_ = bcs.BlockchainPtr.ContractEngine.DB.SaveStorage(req.Address, "__verified", "true")
	_ = bcs.BlockchainPtr.ContractEngine.DB.SaveStorage(req.Address, "__source_hash", sourceHash)
	_ = bcs.BlockchainPtr.ContractEngine.DB.SaveStorage(req.Address, "__verified_at", strconv.FormatInt(time.Now().Unix(), 10))
	_ = bcs.BlockchainPtr.ContractEngine.DB.SaveStorage(req.Address, "__compiler_version", req.CompilerVersion)
	_ = bcs.BlockchainPtr.ContractEngine.DB.SaveStorage(req.Address, "__optimization", strconv.FormatBool(req.Optimization))
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "verified",
		"address":     req.Address,
		"source_hash": sourceHash,
		"matched":     match,
	})
}
