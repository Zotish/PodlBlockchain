package blockchaincomponent

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// WaitForTxReceipt polls for a transaction receipt until timeout or confirmation.
func WaitForTxReceipt(client *ethclient.Client, txHashHex string, timeout, pollInterval time.Duration) (*types.Receipt, error) {
	if client == nil {
		return nil, fmt.Errorf("client is nil")
	}
	txHashHex = strings.TrimSpace(txHashHex)
	if txHashHex == "" {
		return nil, fmt.Errorf("empty tx hash")
	}
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	if pollInterval <= 0 {
		pollInterval = 2 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	hash := common.HexToHash(txHashHex)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		receipt, err := client.TransactionReceipt(ctx, hash)
		if err == nil && receipt != nil {
			return receipt, nil
		}
		if ctx.Err() != nil {
			if err != nil {
				return nil, err
			}
			return nil, ctx.Err()
		}
		select {
		case <-ctx.Done():
			if err != nil {
				return nil, err
			}
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

// ReceiptSuccessful reports whether the receipt succeeded.
func ReceiptSuccessful(receipt *types.Receipt) bool {
	return receipt != nil && receipt.Status == types.ReceiptStatusSuccessful
}

// BridgeRPCEndpoints returns the configured BSC RPC endpoints in priority order.
func BridgeRPCEndpoints(primary string) []string {
	raw := strings.TrimSpace(os.Getenv("BSC_TESTNET_RPCS"))
	endpoints := make([]string, 0, 4)
	if raw != "" {
		for _, part := range strings.Split(raw, ",") {
			if ep := strings.TrimSpace(part); ep != "" {
				endpoints = append(endpoints, ep)
			}
		}
	}
	if primary = strings.TrimSpace(primary); primary != "" {
		endpoints = append([]string{primary}, endpoints...)
	}
	if len(endpoints) == 0 {
		if single := strings.TrimSpace(os.Getenv("BSC_TESTNET_RPC")); single != "" {
			endpoints = append(endpoints, single)
		}
	}
	seen := make(map[string]bool, len(endpoints))
	out := make([]string, 0, len(endpoints))
	for _, ep := range endpoints {
		key := strings.ToLower(ep)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, ep)
	}
	return out
}

// DialBscClient tries configured RPC endpoints until one succeeds.
func DialBscClient(endpoints []string) (*ethclient.Client, string, error) {
	if len(endpoints) == 0 {
		return nil, "", fmt.Errorf("no rpc endpoints configured")
	}
	timeout := 5 * time.Second
	if raw := strings.TrimSpace(os.Getenv("BRIDGE_RPC_DIAL_TIMEOUT_SEC")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			timeout = time.Duration(n) * time.Second
		}
	}
	for _, ep := range endpoints {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		client, err := ethclient.DialContext(ctx, ep)
		cancel()
		if err == nil {
			return client, ep, nil
		}
	}
	return nil, "", fmt.Errorf("unable to connect to any rpc endpoint")
}

type BridgeReceiptConsensus struct {
	Receipt    *types.Receipt
	Header     *types.Header
	Endpoints  []string
	QuorumSize int
}

type bridgeReceiptObservation struct {
	ep      string
	receipt *types.Receipt
	header  *types.Header
}

// ConsensusReceipt checks the same transaction receipt against multiple RPC endpoints
// and returns the first quorum-matching receipt/header pair.
func ConsensusReceipt(endpoints []string, txHashHex string, timeout, pollInterval time.Duration) (*BridgeReceiptConsensus, error) {
	if len(endpoints) == 0 {
		endpoints = BridgeRPCEndpoints("")
	}
	if len(endpoints) == 0 {
		return nil, fmt.Errorf("no rpc endpoints configured")
	}
	minQuorum := 2
	if raw := strings.TrimSpace(os.Getenv("BRIDGE_RPC_MIN_QUORUM")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			minQuorum = n
		}
	}
	if minQuorum > len(endpoints) {
		minQuorum = len(endpoints)
	}
	if minQuorum <= 0 {
		minQuorum = 1
	}

	observations := make([]bridgeReceiptObservation, 0, len(endpoints))
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	out := make(chan bridgeReceiptObservation, len(endpoints))
	var wg sync.WaitGroup
	for _, ep := range endpoints {
		wg.Add(1)
		go func(endpoint string) {
			defer wg.Done()
			client, err := ethclient.DialContext(ctx, endpoint)
			if err != nil {
				return
			}
			defer client.Close()
			receipt, err := waitForTxReceiptCtx(ctx, client, txHashHex, pollInterval)
			if err != nil || receipt == nil {
				return
			}
			header, herr := client.HeaderByNumber(ctx, receipt.BlockNumber)
			if herr != nil || header == nil || receipt.BlockHash != header.Hash() {
				return
			}
			select {
			case out <- bridgeReceiptObservation{ep: endpoint, receipt: receipt, header: header}:
			case <-ctx.Done():
			}
		}(ep)
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	for ob := range out {
		observations = append(observations, ob)
	}
	if len(observations) == 0 {
		return nil, fmt.Errorf("no rpc endpoint returned a confirmed receipt")
	}

	type key struct {
		blockHash   string
		blockNumber string
		status      string
	}
	grouped := make(map[key][]bridgeReceiptObservation)
	for _, ob := range observations {
		k := key{
			blockHash:   ob.receipt.BlockHash.Hex(),
			blockNumber: ob.receipt.BlockNumber.String(),
			status:      fmt.Sprintf("%d", ob.receipt.Status),
		}
		grouped[k] = append(grouped[k], ob)
	}

	keys := make([]key, 0, len(grouped))
	for k := range grouped {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return len(grouped[keys[i]]) > len(grouped[keys[j]])
	})
	best := grouped[keys[0]]
	if len(best) < minQuorum {
		return nil, fmt.Errorf("receipt quorum not reached: have %d want %d", len(best), minQuorum)
	}
	chosen := best[0]
	return &BridgeReceiptConsensus{
		Receipt:    chosen.receipt,
		Header:     chosen.header,
		Endpoints:  collectEndpoints(best),
		QuorumSize: len(best),
	}, nil
}

func waitForTxReceiptCtx(ctx context.Context, client *ethclient.Client, txHashHex string, pollInterval time.Duration) (*types.Receipt, error) {
	if client == nil {
		return nil, fmt.Errorf("client is nil")
	}
	txHashHex = strings.TrimSpace(txHashHex)
	if txHashHex == "" {
		return nil, fmt.Errorf("empty tx hash")
	}
	if pollInterval <= 0 {
		pollInterval = 2 * time.Second
	}
	hash := common.HexToHash(txHashHex)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		receipt, err := client.TransactionReceipt(ctx, hash)
		if err == nil && receipt != nil {
			return receipt, nil
		}
		if ctx.Err() != nil {
			if err != nil {
				return nil, err
			}
			return nil, ctx.Err()
		}
		select {
		case <-ctx.Done():
			if err != nil {
				return nil, err
			}
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

func collectEndpoints(items []bridgeReceiptObservation) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.ep)
	}
	return out
}
