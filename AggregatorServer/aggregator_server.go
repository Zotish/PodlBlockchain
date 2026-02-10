package aggregatorserver

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type AggregatorServer struct {
	Port  uint
	Nodes []string
	Canonical string
	WalletURL string
	DiscoverInterval time.Duration
	lastDiscover time.Time
	nodesMu sync.Mutex
}

func NewAggregatorServer(port uint, nodes []string, canonical string, walletURL string) *AggregatorServer {
	return &AggregatorServer{
		Port:  port,
		Nodes: nodes,
		Canonical: canonical,
		WalletURL: walletURL,
		DiscoverInterval: 5 * time.Second,
	}
}

func (a *AggregatorServer) Start() {
	http.HandleFunc("/chain/global", a.GlobalSummary)
	http.HandleFunc("/health", a.Health)
	http.HandleFunc("/", a.ProxyOrAggregate)

	log.Println("Aggregator server is starting on port:", a.Port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", a.Port), nil); err != nil {
		log.Fatalf("Failed to start aggregator server: %v", err)
	}
}

func (a *AggregatorServer) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (a *AggregatorServer) GlobalSummary(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	nodes := a.Nodes
	if nodesParam := r.URL.Query().Get("nodes"); nodesParam != "" {
		nodes = parseNodes(nodesParam)
	}
	if len(nodes) == 0 {
		nodes = a.discoverNodes()
	}

	type nodeResult struct {
		Node   string      `json:"node"`
		Summary interface{} `json:"summary,omitempty"`
		Error  string      `json:"error,omitempty"`
	}

	results := make([]nodeResult, 0, len(nodes))
	client := &http.Client{Timeout: 2 * time.Second}
	for _, node := range nodes {
		url := fmt.Sprintf("http://%s/chain/summary?limit=%d", node, limit)
		resp, err := client.Get(url)
		if err != nil {
			results = append(results, nodeResult{Node: node, Error: err.Error()})
			continue
		}
		var summary interface{}
		if err := json.NewDecoder(resp.Body).Decode(&summary); err != nil {
			resp.Body.Close()
			results = append(results, nodeResult{Node: node, Error: err.Error()})
			continue
		}
		resp.Body.Close()
		results = append(results, nodeResult{Node: node, Summary: summary})
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"nodes": results,
	})
}

func parseNodes(nodesParam string) []string {
	parts := strings.Split(nodesParam, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func (a *AggregatorServer) ProxyOrAggregate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	path := r.URL.Path
	if strings.HasPrefix(path, "/wallet/") {
		a.proxyRequest(w, r, a.WalletURL)
		return
	}
	if path == "/chain/global" || path == "/health" {
		http.NotFound(w, r)
		return
	}
	if strings.HasPrefix(path, "/block/") || strings.HasPrefix(path, "/tx/") {
		a.proxyRequest(w, r, a.Canonical)
		return
	}
	if strings.HasPrefix(path, "/contract/") {
		a.proxyRequest(w, r, a.Canonical)
		return
	}
	if strings.HasPrefix(path, "/bridge/") {
		a.proxyRequest(w, r, a.Canonical)
		return
	}
	if path == "/rpc" || r.Method != http.MethodGet {
		a.proxyRequest(w, r, a.Canonical)
		return
	}

	results := a.aggregateRequest(r)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"path":  path,
		"nodes": results,
	})
}

func (a *AggregatorServer) proxyRequest(w http.ResponseWriter, r *http.Request, target string) {
	if target == "" {
		http.Error(w, "proxy target not configured", http.StatusBadGateway)
		return
	}
	url := fmt.Sprintf("%s%s", target, r.URL.Path)
	if r.URL.RawQuery != "" {
		url = url + "?" + r.URL.RawQuery
	}

	req, err := http.NewRequest(r.Method, url, r.Body)
	if err != nil {
		http.Error(w, "proxy request failed", http.StatusBadGateway)
		return
	}
	req.Header = r.Header.Clone()

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (a *AggregatorServer) aggregateRequest(r *http.Request) []map[string]interface{} {
	nodes := a.Nodes
	if nodesParam := r.URL.Query().Get("nodes"); nodesParam != "" {
		nodes = parseNodes(nodesParam)
	}
	if len(nodes) == 0 {
		nodes = a.discoverNodes()
	}

	results := make([]map[string]interface{}, 0, len(nodes))
	client := &http.Client{Timeout: 3 * time.Second}
	for _, node := range nodes {
		url := fmt.Sprintf("http://%s%s", node, r.URL.Path)
		if r.URL.RawQuery != "" {
			url = url + "?" + r.URL.RawQuery
		}
		resp, err := client.Get(url)
		if err != nil {
			results = append(results, map[string]interface{}{
				"node":  node,
				"error": err.Error(),
			})
			continue
		}
		var payload interface{}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			resp.Body.Close()
			results = append(results, map[string]interface{}{
				"node":  node,
				"error": err.Error(),
			})
			continue
		}
		resp.Body.Close()
		results = append(results, map[string]interface{}{
			"node":   node,
			"result": payload,
		})
	}
	return results
}

func (a *AggregatorServer) discoverNodes() []string {
	a.nodesMu.Lock()
	defer a.nodesMu.Unlock()

	if time.Since(a.lastDiscover) < a.DiscoverInterval && len(a.Nodes) > 0 {
		return append([]string{}, a.Nodes...)
	}

	canonicalHost := canonicalHostPort(a.Canonical)
	nodesSet := map[string]struct{}{}
	if canonicalHost != "" {
		nodesSet[canonicalHost] = struct{}{}
	}

	if a.Canonical != "" {
		client := &http.Client{Timeout: 2 * time.Second}
		resp, err := client.Get(fmt.Sprintf("%s/peers", a.Canonical))
		if err == nil {
			defer resp.Body.Close()
			var peers []struct {
				Address  string `json:"address"`
				HTTPPort int    `json:"http_port"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&peers); err == nil {
				for _, p := range peers {
					if p.Address == "" || p.HTTPPort <= 0 {
						continue
					}
					nodesSet[fmt.Sprintf("%s:%d", p.Address, p.HTTPPort)] = struct{}{}
				}
			}
		}
	}

	nodes := make([]string, 0, len(nodesSet))
	for n := range nodesSet {
		nodes = append(nodes, n)
	}
	a.Nodes = nodes
	a.lastDiscover = time.Now()
	return append([]string{}, nodes...)
}

func canonicalHostPort(base string) string {
	if base == "" {
		return ""
	}
	if strings.HasPrefix(base, "http://") || strings.HasPrefix(base, "https://") {
		u, err := url.Parse(base)
		if err == nil && u.Host != "" {
			return u.Host
		}
		return ""
	}
	return base
}
