package blockchainserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	blockchaincomponent "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/BlockchainComponent"
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
