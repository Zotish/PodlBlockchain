package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type walletCreateResponse struct {
	Address    string `json:"address"`
	PrivateKey string `json:"private_key"`
	Mnemonic   string `json:"mnemonic"`
}

func main() {
	nodeURL := getenv("NODE_URL", "http://127.0.0.1:6500")
	walletURL := getenv("WALLET_URL", "http://127.0.0.1:8080")
	aggURL := getenv("AGGREGATOR_URL", "http://127.0.0.1:9000")
	client := &http.Client{Timeout: 20 * time.Second}

	mustCheckGET(client, nodeURL+"/bridge/families", "bridge families")
	mustCheckGET(client, nodeURL+"/bridge/chains", "bridge chains")
	mustCheckGET(client, nodeURL+"/bridge/tokens", "bridge tokens")
	mustCheckGET(client, aggURL+"/", "aggregator root")

	wallet := mustCreateWallet(client, walletURL)
	fmt.Printf("wallet.create ok address=%s\n", wallet.Address)

	publicReq := map[string]any{
		"chain_id":        "cosmos-mainnet",
		"family":          "cosmos",
		"adapter":         "cosmos",
		"tx_hash":         "0xpub-smoke-cosmos",
		"source_tx_hash":  "0xpub-smoke-cosmos",
		"source_address":  "cosmos1smoketestaddr",
		"source_memo":     "bridge-smoke",
		"token":           "0x1111111111111111111111111111111111111111",
		"from":            "cosmos1smoketestaddr",
		"to_lqd":          wallet.Address,
		"amount":          "1",
		"mode":            "public",
	}
	privateReq := map[string]any{
		"chain_id":        "bitcoin-mainnet",
		"family":          "utxo",
		"adapter":         "utxo",
		"tx_hash":         "0xpriv-smoke-utxo",
		"source_tx_hash":  "0xpriv-smoke-utxo",
		"source_address":  "bc1qsmoketestaddr",
		"source_output":   "0",
		"token":           "0x2222222222222222222222222222222222222222",
		"from":            "bc1qsmoketestaddr",
		"to_lqd":          wallet.Address,
		"amount":          "1",
		"mode":            "private",
	}

	publicResp := mustPOSTJSON(client, nodeURL+"/bridge/lock_chain", publicReq, "public bridge lock")
	privateResp := mustPOSTJSON(client, nodeURL+"/bridge/lock_chain", privateReq, "private bridge lock")
	fmt.Printf("bridge.public ok status=%v\n", publicResp["status"])
	fmt.Printf("bridge.private ok status=%v\n", privateResp["status"])

	publicHistory := mustGETJSON(client, nodeURL+"/bridge/requests?mode=public", "public bridge history")
	privateHistory := mustGETJSON(client, nodeURL+"/bridge/requests?mode=private", "private bridge history")
	fmt.Printf("history.public count=%d\n", len(asSlice(publicHistory)))
	fmt.Printf("history.private count=%d\n", len(asSlice(privateHistory)))

	fmt.Println("bridge smoke test: PASS")
}

func getenv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func mustCheckGET(client *http.Client, url, label string) {
	resp, err := client.Get(url)
	if err != nil {
		fatalf("%s request failed: %v", label, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		fatalf("%s returned %s: %s", label, resp.Status, strings.TrimSpace(string(body)))
	}
}

func mustGETJSON(client *http.Client, url, label string) any {
	resp, err := client.Get(url)
	if err != nil {
		fatalf("%s request failed: %v", label, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fatalf("%s returned %s: %s", label, resp.Status, strings.TrimSpace(string(body)))
	}
	var out any
	if err := json.Unmarshal(body, &out); err != nil {
		fatalf("%s decode failed: %v", label, err)
	}
	return out
}

func mustPOSTJSON(client *http.Client, url string, payload any, label string) map[string]any {
	body, err := json.Marshal(payload)
	if err != nil {
		fatalf("%s marshal failed: %v", label, err)
	}
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		fatalf("%s request failed: %v", label, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fatalf("%s returned %s: %s", label, resp.Status, strings.TrimSpace(string(raw)))
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		fatalf("%s decode failed: %v", label, err)
	}
	return out
}

func mustCreateWallet(client *http.Client, walletURL string) walletCreateResponse {
	payload := map[string]any{"password": "bridge-smoke-test"}
	body, _ := json.Marshal(payload)
	resp, err := client.Post(walletURL+"/wallet/new", "application/json", bytes.NewReader(body))
	if err != nil {
		fatalf("wallet.create request failed: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fatalf("wallet.create returned %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	var out walletCreateResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		fatalf("wallet.create decode failed: %v", err)
	}
	if out.Address == "" {
		fatalf("wallet.create returned empty address")
	}
	return out
}

func asSlice(v any) []any {
	if v == nil {
		return nil
	}
	if s, ok := v.([]any); ok {
		return s
	}
	return nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "bridge smoke test failed: "+format+"\n", args...)
	os.Exit(1)
}
