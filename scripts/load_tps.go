package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type walletResp struct {
	Address    string `json:"address"`
	PrivateKey string `json:"private_key"`
	Mnemonic   string `json:"mnemonic"`
}

func main() {
	walletURL := flag.String("wallet", "http://127.0.0.1:8080", "Wallet server base URL")
	nodeURL := flag.String("node", "http://127.0.0.1:5000", "Blockchain node base URL")
	rate := flag.Int("rate", 1600, "Transactions per interval")
	interval := flag.Duration("interval", 100*time.Millisecond, "Interval duration")
	seconds := flag.Int("seconds", 1, "Total test duration in seconds")
	concurrency := flag.Int("concurrency", 200, "Concurrent workers")
	batchSize := flag.Int("batch_size", 200, "Transactions per batch request")
	fromAddr := flag.String("from", "", "Existing sender address (skip wallet/new)")
	fromPriv := flag.String("private_key", "", "Existing sender private key (skip wallet/new)")
	toAddr := flag.String("to", "", "Receiver address (optional)")
	skipFaucet := flag.Bool("skip_faucet", false, "Skip faucet funding")
	value := flag.Uint64("value", 1, "Value per transaction")
	gas := flag.Uint64("gas", 21000, "Gas per transaction")
	gasPrice := flag.Uint64("gas_price", 10, "Gas price")
	flag.Parse()

	client := &http.Client{Timeout: 10 * time.Second}

	var sender *walletResp
	var err error
	if *fromAddr != "" && *fromPriv != "" {
		sender = &walletResp{Address: *fromAddr, PrivateKey: *fromPriv}
	} else {
		sender, err = newWallet(client, *walletURL)
		if err != nil {
			panic(err)
		}
	}

	var receiver *walletResp
	if *toAddr != "" {
		receiver = &walletResp{Address: *toAddr}
	} else {
		receiver, err = newWallet(client, *walletURL)
		if err != nil {
			panic(err)
		}
	}

	if !*skipFaucet {
		if err := faucet(client, *nodeURL, sender.Address); err != nil {
			fmt.Printf("warn: faucet failed: %v\n", err)
		}
	}

	var sent uint64
	var ok uint64
	var failed uint64

	jobs := make(chan int, *rate)
	var wg sync.WaitGroup

	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for count := range jobs {
				if count <= 0 {
					continue
				}
				atomic.AddUint64(&sent, uint64(count))
				acc, fail, err := sendBatch(client, *walletURL, sender, receiver.Address, *value, *gas, *gasPrice, count)
				if err != nil {
					atomic.AddUint64(&failed, uint64(count))
					continue
				}
				atomic.AddUint64(&ok, uint64(acc))
				atomic.AddUint64(&failed, uint64(fail))
			}
		}()
	}

	ticks := int(time.Duration(*seconds) * time.Second / *interval)
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	start := time.Now()
	for i := 0; i < ticks; i++ {
		<-ticker.C
		remaining := *rate
		for remaining > 0 {
			batch := *batchSize
			if batch > remaining {
				batch = remaining
			}
			remaining -= batch
			jobs <- batch
		}
	}

	close(jobs)
	wg.Wait()

	elapsed := time.Since(start)
	fmt.Printf("sent=%d ok=%d failed=%d elapsed=%s\n", sent, ok, failed, elapsed)
	fmt.Printf("approx_tps=%.2f\n", float64(ok)/elapsed.Seconds())
}

func newWallet(client *http.Client, walletURL string) (*walletResp, error) {
	payload := []byte(`{"password":"loadtest"}`)
	resp, err := client.Post(walletURL+"/wallet/new", "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return nil, fmt.Errorf("wallet/new: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("wallet/new status=%d body=%s", resp.StatusCode, string(body))
	}
	var out walletResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("wallet/new decode: %w", err)
	}
	return &out, nil
}

func faucet(client *http.Client, nodeURL, address string) error {
	body, _ := json.Marshal(map[string]string{"address": address})
	resp, err := client.Post(nodeURL+"/faucet", "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("faucet: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("faucet status=%d body=%s", resp.StatusCode, string(b))
	}
	return nil
}

func sendBatch(client *http.Client, walletURL string, sender *walletResp, to string, value, gas, gasPrice uint64, count int) (int, int, error) {
	req := map[string]interface{}{
		"from":        sender.Address,
		"to":          to,
		"value":       value,
		"data":        "loadtest",
		"gas":         gas,
		"gas_price":   gasPrice,
		"private_key": sender.PrivateKey,
		"count":       count,
	}
	body, _ := json.Marshal(req)
	resp, err := client.Post(walletURL+"/wallet/send_batch", "application/json", bytes.NewBuffer(body))
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return 0, 0, fmt.Errorf("send_batch status=%d body=%s", resp.StatusCode, string(b))
	}
	var out struct {
		Accepted int `json:"accepted"`
		Failed   int `json:"failed"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return 0, 0, err
	}
	return out.Accepted, out.Failed, nil
}
