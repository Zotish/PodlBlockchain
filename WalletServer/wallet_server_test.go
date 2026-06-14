package walletserver

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

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

func TestWalletHealthEndpoint(t *testing.T) {
	ws := NewWalletServer(8080, "http://127.0.0.1:6500")
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	ws.Health(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected health endpoint to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	for _, needle := range []string{`"status":"ok"`, `"service":"wallet"`} {
		if !strings.Contains(body, needle) {
			t.Fatalf("expected health body to contain %s, got %s", needle, body)
		}
	}
}

func TestFetchNextNonceUsesNextNonceEvenWithoutPendingCount(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if !strings.HasSuffix(req.URL.Path, "/nonce") {
			t.Fatalf("unexpected path %s", req.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"confirmed_nonce":0,"next_nonce":1,"nonce":1}`)),
		}, nil
	})}

	ws := NewWalletServer(8080, "http://chain.test")
	nonce, err := ws.fetchNextNonce(client, "0x1111111111111111111111111111111111111111")
	if err != nil {
		t.Fatalf("fetchNextNonce() error = %v", err)
	}
	if nonce != 1 {
		t.Fatalf("expected next nonce 1, got %d", nonce)
	}
}
