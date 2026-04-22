package blockchaincomponent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// BridgeChainConfig describes an EVM-compatible chain that participates in the bridge.
// This is intentionally bridge-only and does not touch consensus, blocks, or tx engine internals.
type BridgeChainConfig struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	ChainID         string   `json:"chain_id"`
	Family          string   `json:"family,omitempty"`
	Adapter         string   `json:"adapter,omitempty"`
	RPC             string   `json:"rpc"`
	RPCs            []string `json:"rpcs,omitempty"`
	ExplorerURL     string   `json:"explorer_url,omitempty"`
	BridgeAddress   string   `json:"bridge_address"`
	LockAddress     string   `json:"lock_address"`
	NativeSymbol    string   `json:"native_symbol,omitempty"`
	Enabled         bool     `json:"enabled"`
	SupportsPublic  bool     `json:"supports_public"`
	SupportsPrivate bool     `json:"supports_private"`
	CreatedAt       int64    `json:"created_at"`
	UpdatedAt       int64    `json:"updated_at"`
}

type BridgeChainRegistry struct {
	UpdatedAt int64                         `json:"updated_at"`
	Chains    map[string]*BridgeChainConfig `json:"chains"`
}

var bridgeChainRegistryMu sync.Mutex

func bridgeChainRegistryPath() string {
	return filepath.Join(bridgeRegistryDataDir(), "bridge_chains.json")
}

func normalizeBridgeChainID(id string) string {
	return strings.ToLower(strings.TrimSpace(id))
}

func defaultBridgeChainRegistry() *BridgeChainRegistry {
	r := &BridgeChainRegistry{
		UpdatedAt: time.Now().Unix(),
		Chains:    make(map[string]*BridgeChainConfig),
	}
	// Default to the current BSC testnet setup if configured.
	r.Upsert(&BridgeChainConfig{
		ID:              "bsc-testnet",
		Name:            "BSC Testnet",
		ChainID:         "97",
		Family:          "evm",
		Adapter:         "evm",
		RPC:             strings.TrimSpace(os.Getenv("BSC_TESTNET_RPC")),
		RPCs:            BridgeRPCEndpoints(os.Getenv("BSC_TESTNET_RPC")),
		ExplorerURL:     "https://testnet.bscscan.com",
		BridgeAddress:   strings.TrimSpace(os.Getenv("BSC_BRIDGE_ADDRESS")),
		LockAddress:     strings.TrimSpace(os.Getenv("BSC_LOCK_ADDRESS")),
		NativeSymbol:    "BNB",
		Enabled:         true,
		SupportsPublic:  true,
		SupportsPrivate: true,
	})
	r.Upsert(&BridgeChainConfig{
		ID:              "bsc-mainnet",
		Name:            "BSC Mainnet",
		ChainID:         "56",
		Family:          "evm",
		Adapter:         "evm",
		NativeSymbol:    "BNB",
		Enabled:         false,
		SupportsPublic:  true,
		SupportsPrivate: true,
	})
	r.Upsert(&BridgeChainConfig{
		ID:              "ethereum-mainnet",
		Name:            "Ethereum Mainnet",
		ChainID:         "1",
		Family:          "evm",
		Adapter:         "evm",
		NativeSymbol:    "ETH",
		Enabled:         false,
		SupportsPublic:  true,
		SupportsPrivate: true,
	})
	r.Upsert(&BridgeChainConfig{
		ID:              "base-mainnet",
		Name:            "Base Mainnet",
		ChainID:         "8453",
		Family:          "evm",
		Adapter:         "evm",
		NativeSymbol:    "ETH",
		Enabled:         false,
		SupportsPublic:  true,
		SupportsPrivate: true,
	})
	r.Upsert(&BridgeChainConfig{
		ID:              "polygon-mainnet",
		Name:            "Polygon Mainnet",
		ChainID:         "137",
		Family:          "evm",
		Adapter:         "evm",
		NativeSymbol:    "MATIC",
		Enabled:         false,
		SupportsPublic:  true,
		SupportsPrivate: true,
	})
	r.Upsert(&BridgeChainConfig{
		ID:              "arbitrum-one",
		Name:            "Arbitrum One",
		ChainID:         "42161",
		Family:          "evm",
		Adapter:         "evm",
		NativeSymbol:    "ETH",
		Enabled:         false,
		SupportsPublic:  true,
		SupportsPrivate: true,
	})
	r.Upsert(&BridgeChainConfig{
		ID:              "optimism-mainnet",
		Name:            "Optimism Mainnet",
		ChainID:         "10",
		Family:          "evm",
		Adapter:         "evm",
		NativeSymbol:    "ETH",
		Enabled:         false,
		SupportsPublic:  true,
		SupportsPrivate: true,
	})
	r.Upsert(&BridgeChainConfig{
		ID:              "avalanche-c",
		Name:            "Avalanche C-Chain",
		ChainID:         "43114",
		Family:          "evm",
		Adapter:         "evm",
		NativeSymbol:    "AVAX",
		Enabled:         false,
		SupportsPublic:  true,
		SupportsPrivate: true,
	})
	r.Upsert(&BridgeChainConfig{
		ID:              "linea-mainnet",
		Name:            "Linea Mainnet",
		ChainID:         "59144",
		Family:          "evm",
		Adapter:         "evm",
		NativeSymbol:    "ETH",
		Enabled:         false,
		SupportsPublic:  true,
		SupportsPrivate: true,
	})
	// Non-EVM chain presets start disabled until an operator supplies the live
	// RPC / bridge / lock endpoints and turns them on from the admin panel.
	r.Upsert(&BridgeChainConfig{
		ID:              "bitcoin-mainnet",
		Name:            "Bitcoin Mainnet",
		ChainID:         "btc-mainnet",
		Family:          "utxo",
		Adapter:         "utxo",
		NativeSymbol:    "BTC",
		Enabled:         false,
		SupportsPublic:  true,
		SupportsPrivate: true,
	})
	r.Upsert(&BridgeChainConfig{
		ID:              "litecoin-mainnet",
		Name:            "Litecoin Mainnet",
		ChainID:         "ltc-mainnet",
		Family:          "utxo",
		Adapter:         "utxo",
		NativeSymbol:    "LTC",
		Enabled:         false,
		SupportsPublic:  true,
		SupportsPrivate: true,
	})
	r.Upsert(&BridgeChainConfig{
		ID:              "dogecoin-mainnet",
		Name:            "Dogecoin Mainnet",
		ChainID:         "doge-mainnet",
		Family:          "utxo",
		Adapter:         "utxo",
		NativeSymbol:    "DOGE",
		Enabled:         false,
		SupportsPublic:  true,
		SupportsPrivate: true,
	})
	r.Upsert(&BridgeChainConfig{
		ID:              "cardano-mainnet",
		Name:            "Cardano Mainnet",
		ChainID:         "cardano-mainnet",
		Family:          "cardano",
		Adapter:         "cardano",
		NativeSymbol:    "ADA",
		Enabled:         false,
		SupportsPublic:  true,
		SupportsPrivate: true,
	})
	r.Upsert(&BridgeChainConfig{
		ID:              "near-mainnet",
		Name:            "NEAR Mainnet",
		ChainID:         "near-mainnet",
		Family:          "near",
		Adapter:         "near",
		NativeSymbol:    "NEAR",
		Enabled:         false,
		SupportsPublic:  true,
		SupportsPrivate: true,
	})
	r.Upsert(&BridgeChainConfig{
		ID:              "aptos-mainnet",
		Name:            "Aptos Mainnet",
		ChainID:         "aptos-mainnet",
		Family:          "aptos",
		Adapter:         "aptos",
		NativeSymbol:    "APT",
		Enabled:         false,
		SupportsPublic:  true,
		SupportsPrivate: true,
	})
	return r
}

func loadBridgeChainRegistry() (*BridgeChainRegistry, error) {
	bridgeChainRegistryMu.Lock()
	defer bridgeChainRegistryMu.Unlock()

	path := bridgeChainRegistryPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return defaultBridgeChainRegistry(), nil
	}
	var reg BridgeChainRegistry
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, err
	}
	if reg.Chains == nil {
		reg.Chains = make(map[string]*BridgeChainConfig)
	}
	return &reg, nil
}

func saveBridgeChainRegistry(reg *BridgeChainRegistry) error {
	if reg == nil {
		return fmt.Errorf("registry is nil")
	}
	bridgeChainRegistryMu.Lock()
	defer bridgeChainRegistryMu.Unlock()

	reg.UpdatedAt = time.Now().Unix()
	if reg.Chains == nil {
		reg.Chains = make(map[string]*BridgeChainConfig)
	}
	if err := os.MkdirAll(filepath.Dir(bridgeChainRegistryPath()), 0755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return err
	}
	tmp := bridgeChainRegistryPath() + ".tmp"
	if err := os.WriteFile(tmp, payload, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, bridgeChainRegistryPath())
}

func (r *BridgeChainRegistry) ensure() {
	if r.Chains == nil {
		r.Chains = make(map[string]*BridgeChainConfig)
	}
}

func (r *BridgeChainRegistry) Upsert(cfg *BridgeChainConfig) {
	if r == nil || cfg == nil {
		return
	}
	r.ensure()
	key := normalizeBridgeChainID(cfg.ID)
	if key == "" {
		key = normalizeBridgeChainID(cfg.ChainID)
	}
	if key == "" {
		return
	}
	now := time.Now().Unix()
	if cfg.CreatedAt == 0 {
		cfg.CreatedAt = now
	}
	cfg.UpdatedAt = now
	if cfg.ID == "" {
		cfg.ID = key
	}
	if cfg.Family == "" {
		cfg.Family = "evm"
	}
	if cfg.Adapter == "" {
		cfg.Adapter = cfg.Family
	}
	r.Chains[key] = cfg
	r.UpdatedAt = now
}

func (r *BridgeChainRegistry) Remove(id string) {
	if r == nil {
		return
	}
	r.ensure()
	delete(r.Chains, normalizeBridgeChainID(id))
	r.UpdatedAt = time.Now().Unix()
}

func (r *BridgeChainRegistry) Get(id string) *BridgeChainConfig {
	if r == nil {
		return nil
	}
	r.ensure()
	if cfg, ok := r.Chains[normalizeBridgeChainID(id)]; ok {
		return cfg
	}
	needle := normalizeBridgeChainID(id)
	for _, cfg := range r.Chains {
		if normalizeBridgeChainID(cfg.ChainID) == needle {
			return cfg
		}
	}
	return nil
}

func (r *BridgeChainRegistry) ChainByID(chainID string) *BridgeChainConfig {
	return r.Get(chainID)
}

func (r *BridgeChainRegistry) ChainByName(name string) *BridgeChainConfig {
	if r == nil {
		return nil
	}
	r.ensure()
	needle := strings.ToLower(strings.TrimSpace(name))
	if needle == "" {
		return nil
	}
	for _, cfg := range r.Chains {
		if strings.ToLower(strings.TrimSpace(cfg.Name)) == needle {
			return cfg
		}
	}
	return nil
}

func (r *BridgeChainRegistry) AnyEnabled() *BridgeChainConfig {
	if r == nil {
		return nil
	}
	r.ensure()
	for _, cfg := range r.Chains {
		if cfg != nil && cfg.Enabled {
			return cfg
		}
	}
	return nil
}

func (r *BridgeChainRegistry) List() []*BridgeChainConfig {
	if r == nil {
		return nil
	}
	r.ensure()
	out := make([]*BridgeChainConfig, 0, len(r.Chains))
	for _, cfg := range r.Chains {
		out = append(out, cfg)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].ID) < strings.ToLower(out[j].ID)
	})
	return out
}

func LoadBridgeChainRegistry() (*BridgeChainRegistry, error) {
	return loadBridgeChainRegistry()
}

func SaveBridgeChainRegistry(reg *BridgeChainRegistry) error {
	return saveBridgeChainRegistry(reg)
}
