package blockchainserver

import (
	"bytes"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	blockchaincomponent "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/BlockchainComponent"
	constantset "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/ConstantSet"
	"github.com/gorilla/mux"
)

func TestBridgeAdminKeyMatches(t *testing.T) {
	t.Setenv("LQD_API_KEY", "secret-key")

	req := httptest.NewRequest(http.MethodPost, "/bridge/token", nil)
	req.Header.Set("X-API-Key", "secret-key")
	if !bridgeAdminKeyMatches(req) {
		t.Fatal("expected request header API key to match")
	}

	req = httptest.NewRequest(http.MethodPost, "/bridge/token?api_key=secret-key", nil)
	if !bridgeAdminKeyMatches(req) {
		t.Fatal("expected query API key to match")
	}

	req = httptest.NewRequest(http.MethodPost, "/bridge/token", nil)
	if bridgeAdminKeyMatches(req) {
		t.Fatal("expected missing API key to fail")
	}
}

func TestSetCORSHeadersAllowsAdminConsoleOrigin(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, "/bridge/token", nil)
	req.Header.Set("Origin", "http://localhost:4173")

	rr := httptest.NewRecorder()
	setCORSHeaders(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:4173" {
		t.Fatalf("expected admin console origin to be allowed, got %q", got)
	}
	if !strings.Contains(rr.Header().Get("Access-Control-Allow-Headers"), "X-API-Key") {
		t.Fatalf("expected X-API-Key in allowed headers, got %q", rr.Header().Get("Access-Control-Allow-Headers"))
	}
}

func TestSetCORSHeadersAllowsProductionPreviewOrigins(t *testing.T) {
	for _, origin := range []string{
		"https://178-105-133-94.sslip.io",
		"https://api.178-105-133-94.sslip.io",
		"https://podl-mainnet.netlify.app",
		"https://podl-explorer.vercel.app",
	} {
		req := httptest.NewRequest(http.MethodOptions, "/blocks", nil)
		req.Header.Set("Origin", origin)

		rr := httptest.NewRecorder()
		setCORSHeaders(rr, req)

		if got := rr.Header().Get("Access-Control-Allow-Origin"); got != origin {
			t.Fatalf("expected production origin %s to be allowed, got %q", origin, got)
		}
		if !strings.Contains(rr.Header().Get("Access-Control-Allow-Methods"), "OPTIONS") {
			t.Fatalf("expected OPTIONS in allowed methods, got %q", rr.Header().Get("Access-Control-Allow-Methods"))
		}
	}
}

func TestSetCORSHeadersAllowsWildcardEnvOrigin(t *testing.T) {
	t.Setenv("LQD_ALLOWED_ORIGINS", "*")
	origin := "https://custom-explorer.example.com"
	req := httptest.NewRequest(http.MethodOptions, "/balance", nil)
	req.Header.Set("Origin", origin)

	rr := httptest.NewRecorder()
	setCORSHeaders(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != origin {
		t.Fatalf("expected env wildcard origin to be reflected, got %q", got)
	}
}

func TestGetAccountNonceReportsConfirmedAndPending(t *testing.T) {
	addr := "0x1111111111111111111111111111111111111111"
	bc := &blockchaincomponent.Blockchain_struct{
		Blocks: []*blockchaincomponent.Block{{
			Transactions: []*blockchaincomponent.Transaction{{
				From:   strings.ToUpper(addr),
				Nonce:  0,
				Status: "success",
			}},
		}},
		Transaction_pool: []*blockchaincomponent.Transaction{{
			From:   addr,
			Nonce:  1,
			Status: "pending",
		}},
	}
	server := NewBlockchainServer(6500, bc)
	req := httptest.NewRequest(http.MethodGet, "/account/"+addr+"/nonce", nil)
	req = mux.SetURLVars(req, map[string]string{"address": addr})
	rr := httptest.NewRecorder()

	server.GetAccountNonce(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	for _, needle := range []string{`"confirmed_nonce":1`, `"next_nonce":2`, `"pending_count":1`, `"pending_nonces":[1]`} {
		if !strings.Contains(body, needle) {
			t.Fatalf("expected nonce response to contain %s, got %s", needle, body)
		}
	}
}

func TestGetAccountNonceReadsStandardServeMuxPathValue(t *testing.T) {
	addr := "0x1111111111111111111111111111111111111111"
	bc := &blockchaincomponent.Blockchain_struct{
		Blocks: []*blockchaincomponent.Block{{
			Transactions: []*blockchaincomponent.Transaction{{
				From:   addr,
				Nonce:  0,
				Status: "success",
			}},
		}},
	}
	server := NewBlockchainServer(6500, bc)
	req := httptest.NewRequest(http.MethodGet, "/account/"+addr+"/nonce", nil)
	req.SetPathValue("address", addr)
	rr := httptest.NewRecorder()

	server.GetAccountNonce(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	for _, needle := range []string{`"confirmed_nonce":1`, `"next_nonce":1`, `"nonce":1`} {
		if !strings.Contains(rr.Body.String(), needle) {
			t.Fatalf("expected standard ServeMux path value to produce %s, got %s", needle, rr.Body.String())
		}
	}
}

func TestSendTransactionIndexesAcceptedAndFailedNonceTxs(t *testing.T) {
	oldDBPath := constantset.BLOCKCHAIN_DB_PATH
	constantset.BLOCKCHAIN_DB_PATH = filepath.Join(t.TempDir(), "evodb")
	defer func() { constantset.BLOCKCHAIN_DB_PATH = oldDBPath }()

	from := "0x1111111111111111111111111111111111111111"
	to := "0x2222222222222222222222222222222222222222"
	bc := &blockchaincomponent.Blockchain_struct{
		Blocks: []*blockchaincomponent.Block{{
			BlockNumber:  0,
			Transactions: []*blockchaincomponent.Transaction{},
		}},
		Transaction_pool: []*blockchaincomponent.Transaction{},
		RecentTxs:        []*blockchaincomponent.Transaction{},
		BaseFee:          1,
	}
	server := NewBlockchainServer(6500, bc)
	submit := func(tx blockchaincomponent.Transaction) map[string]interface{} {
		body, _ := json.Marshal(tx)
		req := httptest.NewRequest(http.MethodPost, "/send_tx", bytes.NewReader(body))
		rr := httptest.NewRecorder()
		server.sendTransaction(rr, req)
		var resp map[string]interface{}
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("send_tx response is not JSON: status=%d body=%s", rr.Code, rr.Body.String())
		}
		resp["_status_code"] = float64(rr.Code)
		return resp
	}
	baseTx := blockchaincomponent.Transaction{
		From:      from,
		To:        to,
		Value:     big.NewInt(0),
		Data:      []byte("first"),
		Gas:       uint64(constantset.MinGas),
		GasPrice:  1,
		Nonce:     0,
		ChainID:   uint64(constantset.ChainID),
		Timestamp: uint64(time.Now().Unix()),
	}

	accepted := submit(baseTx)
	if accepted["_status_code"] != float64(http.StatusAccepted) {
		t.Fatalf("expected accepted tx, got %#v", accepted)
	}
	acceptedHash, _ := accepted["tx_hash"].(string)
	if acceptedHash == "" {
		t.Fatalf("expected accepted tx hash, got %#v", accepted)
	}

	req := httptest.NewRequest(http.MethodGet, "/tx/"+acceptedHash, nil)
	rr := httptest.NewRecorder()
	server.GetTransactionByHash(rr, req)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"source":"mempool"`) || !strings.Contains(rr.Body.String(), `"status":"pending"`) {
		t.Fatalf("expected accepted tx to be indexed as mempool pending, status=%d body=%s", rr.Code, rr.Body.String())
	}

	duplicate := baseTx
	duplicate.Data = []byte("second-same-nonce")
	failed := submit(duplicate)
	if failed["_status_code"] != float64(http.StatusBadRequest) {
		t.Fatalf("expected duplicate nonce tx to fail, got %#v", failed)
	}
	failedHash, _ := failed["tx_hash"].(string)
	if failedHash == "" || failedHash == acceptedHash {
		t.Fatalf("expected distinct failed tx hash, accepted=%s failed=%#v", acceptedHash, failed)
	}

	req = httptest.NewRequest(http.MethodGet, "/tx/"+failedHash, nil)
	rr = httptest.NewRecorder()
	server.GetTransactionByHash(rr, req)
	body := rr.Body.String()
	if rr.Code != http.StatusOK || !strings.Contains(body, `"source":"recent"`) || !strings.Contains(body, `"status":"failed"`) || !strings.Contains(body, "failure_reason") {
		t.Fatalf("expected failed tx to be indexed with reason, status=%d body=%s", rr.Code, body)
	}
}

func TestGetBridgeFamiliesReturnsJSONAndCORS(t *testing.T) {
	server := NewBlockchainServer(6500, &blockchaincomponent.Blockchain_struct{})
	req := httptest.NewRequest(http.MethodGet, "/bridge/families", nil)
	req.Header.Set("Origin", "http://localhost:4173")
	rr := httptest.NewRecorder()

	server.GetBridgeFamilies(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	for _, needle := range []string{"\"id\":\"evm\"", "\"id\":\"utxo\"", "\"id\":\"cosmos\""} {
		if !strings.Contains(body, needle) {
			t.Fatalf("expected response body to contain %s, got %s", needle, body)
		}
	}
	if rr.Header().Get("Access-Control-Allow-Origin") != "http://localhost:4173" {
		t.Fatalf("expected CORS origin header, got %q", rr.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestGetBridgeTokensReturnsDeduplicatedMappings(t *testing.T) {
	bc := &blockchaincomponent.Blockchain_struct{
		BridgeTokenMap: make(map[string]*blockchaincomponent.BridgeTokenInfo),
	}
	bc.SetBridgeTokenMappingForChain("bsc-testnet", "0xABC", &blockchaincomponent.BridgeTokenInfo{
		ChainID:     "bsc-testnet",
		SourceToken: "0xabc",
		TargetToken: "0xlqd",
		LqdToken:    "0xlqd",
	})
	server := NewBlockchainServer(6500, bc)

	req := httptest.NewRequest(http.MethodGet, "/bridge/tokens", nil)
	rr := httptest.NewRecorder()
	server.GetBridgeTokens(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if count := strings.Count(body, "\"source_token\":\"0xabc\""); count != 1 {
		t.Fatalf("expected deduplicated token mapping in JSON, got count=%d body=%s", count, body)
	}
}

func TestPersistAndRemoveBridgeTokenRegistryHelpers(t *testing.T) {
	t.Setenv("LQD_BRIDGE_DATA_DIR", t.TempDir())

	info := &blockchaincomponent.BridgeTokenInfo{
		ChainID:     "bsc-testnet",
		SourceToken: "0xabc",
		TargetToken: "0xlqd",
		LqdToken:    "0xlqd",
	}
	if err := persistBridgeTokenRegistry(info); err != nil {
		t.Fatalf("persistBridgeTokenRegistry failed: %v", err)
	}

	reg, err := blockchaincomponent.LoadBridgeTokenRegistry()
	if err != nil {
		t.Fatalf("LoadBridgeTokenRegistry failed: %v", err)
	}
	if len(reg.List()) != 1 {
		t.Fatalf("expected one persisted bridge token, got %d", len(reg.List()))
	}

	if err := removeBridgeTokenRegistry("bsc-testnet", "0xabc", "0xlqd"); err != nil {
		t.Fatalf("removeBridgeTokenRegistry failed: %v", err)
	}
	reg, err = blockchaincomponent.LoadBridgeTokenRegistry()
	if err != nil {
		t.Fatalf("LoadBridgeTokenRegistry after remove failed: %v", err)
	}
	if len(reg.List()) != 0 {
		t.Fatalf("expected registry to be empty after remove, got %d", len(reg.List()))
	}
}
