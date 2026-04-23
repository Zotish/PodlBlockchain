package walletserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCreateNewWalletDoesNotRequireAdminKey(t *testing.T) {
	ws := NewWalletServer(8080, "http://127.0.0.1:6500")
	req := httptest.NewRequest(http.MethodPost, "/wallet/new", strings.NewReader(`{"password":"test-pass"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	ws.CreateNewWallet(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected wallet creation to succeed without admin key, got %d body=%s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	for _, needle := range []string{"\"address\":", "\"mnemonic\":", "\"private_key\":"} {
		if !strings.Contains(body, needle) {
			t.Fatalf("expected wallet creation response to contain %s, got %s", needle, body)
		}
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("expected wildcard CORS for wallet create, got %q", got)
	}
}

func TestCreateNewWalletOptionsPreflight(t *testing.T) {
	ws := NewWalletServer(8080, "http://127.0.0.1:6500")
	req := httptest.NewRequest(http.MethodOptions, "/wallet/new", nil)
	rr := httptest.NewRecorder()

	ws.CreateNewWallet(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected OPTIONS to succeed, got %d", rr.Code)
	}
}
