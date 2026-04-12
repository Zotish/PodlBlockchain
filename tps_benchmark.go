//go:build ignore

// TPS Benchmark — go run tps_benchmark.go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

const (
	chainURL  = "http://127.0.0.1:6500"
	walletURL = "http://127.0.0.1:8080"
	toAddr    = "0xFd220d301291dc32D491A9Fe5b872b8aB5F028D5"
)

func main() {
	fmt.Println("╔══════════════════════════════════════════╗")
	fmt.Println("║      LQD Chain TPS Benchmark v3.0        ║")
	fmt.Println("╚══════════════════════════════════════════╝")
	fmt.Println()

	// ── 1. Block production speed ──────────────────────────────────
	fmt.Println("► Measuring block production speed (10 seconds)...")
	h1 := getHeight()
	t1 := time.Now()
	time.Sleep(10 * time.Second)
	h2 := getHeight()
	t2 := time.Now()
	elapsed := t2.Sub(t1).Seconds()
	blocksProduced := h2 - h1
	blocksPerSec := float64(blocksProduced) / elapsed
	avgBlockMs := elapsed * 1000.0 / float64(blocksProduced)
	fmt.Printf("  Height: %d → %d  (+%d blocks in %.1fs)\n", h1, h2, blocksProduced, elapsed)
	fmt.Printf("  Avg block time   : %.0f ms\n", avgBlockMs)
	fmt.Printf("  Block speed      : %.1f blocks/sec\n\n", blocksPerSec)

	// ── 2. Create & fund wallets ───────────────────────────────────
	fmt.Println("► Creating 5 test wallets...")
	wallets := make([]struct{ addr, pk string }, 0)
	for i := 0; i < 5; i++ {
		resp, _ := http.Post(walletURL+"/wallet/new", "application/json",
			bytes.NewBufferString(`{"password":"bench123"}`))
		if resp == nil { continue }
		var w struct {
			Address string `json:"address"`
			PK      string `json:"private_key"`
		}
		json.NewDecoder(resp.Body).Decode(&w)
		resp.Body.Close()
		if w.Address == "" { continue }
		http.Post(chainURL+"/faucet", "application/json",
			bytes.NewBufferString(fmt.Sprintf(`{"address":"%s"}`, w.Address)))
		wallets = append(wallets, struct{ addr, pk string }{w.Address, w.PK})
		fmt.Printf("  wallet %d: %s\n", i+1, w.Address)
	}
	time.Sleep(2 * time.Second)
	fmt.Println()

	// ── 3. Send 50 txs per wallet (sequential per wallet = proper nonce) ──
	txPerWallet := 50
	total := txPerWallet * len(wallets)
	fmt.Printf("► Sending %d transactions (%d wallets × %d each)...\n",
		total, len(wallets), txPerWallet)

	var sent, failed atomic.Int64
	var wg sync.WaitGroup
	start := time.Now()

	for _, w := range wallets {
		wg.Add(1)
		go func(addr, pk string) {
			defer wg.Done()
			for i := 0; i < txPerWallet; i++ {
				body, _ := json.Marshal(map[string]any{
					"from": addr, "to": toAddr,
					"value": "100", "gas": 21000, "gas_price": 10,
					"private_key": pk,
				})
				r, err := http.Post(walletURL+"/wallet/send",
					"application/json", bytes.NewReader(body))
				if err != nil || r.StatusCode >= 400 {
					failed.Add(1)
				} else {
					sent.Add(1)
				}
				if r != nil { r.Body.Close() }
				time.Sleep(30 * time.Millisecond) // avoid nonce race
			}
		}(w.addr, w.pk)
	}
	wg.Wait()
	sendTime := time.Since(start).Seconds()

	fmt.Printf("  Sent: %d ✓  |  Failed: %d ✗  |  Send time: %.1fs\n",
		sent.Load(), failed.Load(), sendTime)

	// wait for confirmation
	fmt.Println("  Waiting 5s for confirmations...")
	time.Sleep(5 * time.Second)

	// ── 4. Count confirmed user txs ───────────────────────────────
	confirmed := countRecentUserTxs()
	confirmedTPS := float64(confirmed) / (sendTime + 5)
	fmt.Printf("  Confirmed user txs in window: %d\n\n", confirmed)

	// ── 5. Summary ────────────────────────────────────────────────
	maxTxPerBlock := 21_000_000 / 21_000  // gas_limit / min_gas
	theoreticalTPS := blocksPerSec * float64(maxTxPerBlock)

	fmt.Println("╔══════════════════════════════════════════╗")
	fmt.Println("║               FINAL RESULTS              ║")
	fmt.Println("╠══════════════════════════════════════════╣")
	fmt.Printf("║  Block time         : %.0f ms\n", avgBlockMs)
	fmt.Printf("║  Block speed        : %.1f blocks/sec\n", blocksPerSec)
	fmt.Printf("║  Gas limit/block    : 21,000,000\n")
	fmt.Printf("║  Max txs/block      : %d txs\n", maxTxPerBlock)
	fmt.Printf("║  Theoretical TPS    : ~%.0f tx/sec\n", theoreticalTPS)
	fmt.Printf("║  Measured send rate : %.0f tx/sec\n", float64(sent.Load())/sendTime)
	fmt.Printf("║  Confirmed TPS      : ~%.0f tx/sec\n", confirmedTPS)
	fmt.Println("╚══════════════════════════════════════════╝")
}

func getHeight() int {
	r, err := http.Get(chainURL + "/getheight")
	if err != nil { return 0 }
	defer r.Body.Close()
	var res struct{ Height int `json:"height"` }
	json.NewDecoder(r.Body).Decode(&res)
	return res.Height
}

func countRecentUserTxs() int {
	r, err := http.Get(chainURL + "/transactions/recent")
	if err != nil { return 0 }
	defer r.Body.Close()
	var txs []struct {
		From   string `json:"from"`
		Type   string `json:"type"`
		Status string `json:"status"`
	}
	json.NewDecoder(r.Body).Decode(&txs)
	count := 0
	for _, t := range txs {
		if t.From != "0x0000000000000000000000000000000000000000" &&
			t.Type != "reward" && t.Status == "succsess" {
			count++
		}
	}
	return count
}
