package blockchaincomponent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
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
	NativeDecimals  int      `json:"native_decimals,omitempty"`
	Confirmations   int      `json:"confirmations,omitempty"`
	Finality        string   `json:"finality,omitempty"`
	DepositAddress  string   `json:"deposit_address,omitempty"`
	VaultAddress    string   `json:"vault_address,omitempty"`
	ProgramID       string   `json:"program_id,omitempty"`
	TokenProgramID  string   `json:"token_program_id,omitempty"`
	VerifierURL     string   `json:"verifier_url,omitempty"`
	FeeEstimate     float64  `json:"fee_estimate,omitempty"`
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

func bridgeEnvKey(id string) string {
	id = strings.ToUpper(strings.TrimSpace(id))
	var b strings.Builder
	lastUnderscore := false
	for _, r := range id {
		ok := (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
		if ok {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	return strings.Trim(b.String(), "_")
}

func bridgeEnv(names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value
		}
	}
	return ""
}

func bridgeEnvBool(fallback bool, names ...string) bool {
	value := strings.ToLower(bridgeEnv(names...))
	if value == "" {
		return fallback
	}
	return value == "1" || value == "true" || value == "yes" || value == "on" || value == "enabled"
}

func bridgeEnvInt(fallback int, names ...string) int {
	value := bridgeEnv(names...)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return fallback
	}
	return parsed
}

func bridgeEnvFloat(fallback float64, names ...string) float64 {
	value := bridgeEnv(names...)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed < 0 {
		return fallback
	}
	return parsed
}

func bridgeChainIsEVMFamily(family string) bool {
	switch NormalizeBridgeFamilyID(family) {
	case "", "evm", "harmony", "sei", "monad":
		return true
	default:
		return false
	}
}

func bridgeNativeDecimalsDefault(family string) int {
	switch NormalizeBridgeFamilyID(family) {
	case "utxo", "bitcoin", "btc", "litecoin", "dogecoin", "cardano", "aptos":
		return 8
	case "cosmos", "sei", "injective", "tron":
		return 6
	case "near":
		return 24
	case "solana", "sui", "ton":
		return 9
	default:
		return 18
	}
}

func bridgeConfirmationsDefault(family string) int {
	switch NormalizeBridgeFamilyID(family) {
	case "utxo", "bitcoin", "btc", "litecoin", "dogecoin", "cardano":
		return 6
	case "solana":
		return 32
	case "ton", "near", "aptos", "sui", "cosmos", "sei", "injective":
		return 16
	default:
		return 12
	}
}

func bridgeFinalityDefault(family string) string {
	if bridgeChainIsEVMFamily(family) {
		return "confirmed"
	}
	return "finalized"
}

func applyBridgeChainDefaults(cfg *BridgeChainConfig) {
	if cfg == nil {
		return
	}
	cfg.ID = normalizeBridgeChainID(cfg.ID)
	cfg.Family = NormalizeBridgeFamilyID(cfg.Family)
	if cfg.Family == "" {
		cfg.Family = "evm"
	}
	cfg.Adapter = NormalizeBridgeFamilyID(cfg.Adapter)
	if cfg.Adapter == "" {
		cfg.Adapter = cfg.Family
	}
	if cfg.NativeDecimals == 0 {
		cfg.NativeDecimals = bridgeNativeDecimalsDefault(cfg.Family)
	}
	if cfg.Confirmations == 0 {
		cfg.Confirmations = bridgeConfirmationsDefault(cfg.Family)
	}
	if strings.TrimSpace(cfg.Finality) == "" {
		cfg.Finality = bridgeFinalityDefault(cfg.Family)
	}
	if cfg.RPC == "" && len(cfg.RPCs) > 0 {
		cfg.RPC = cfg.RPCs[0]
	}
	if len(cfg.RPCs) == 0 && cfg.RPC != "" {
		cfg.RPCs = []string{cfg.RPC}
	}
}

func bridgeChainDisableIfIncomplete(cfg *BridgeChainConfig) {
	if cfg == nil || !cfg.Enabled {
		return
	}
	adapter := BridgeAdapterByFamily(cfg.Family)
	if adapter == nil || adapter.ValidateConfig(cfg) != nil {
		cfg.Enabled = false
	}
}

func bridgeCSV(raw string) []string {
	out := make([]string, 0)
	seen := make(map[string]bool)
	for _, part := range strings.Split(raw, ",") {
		item := strings.TrimSpace(part)
		if item == "" {
			continue
		}
		key := strings.ToLower(item)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	return out
}

func bridgeApplyEnvOverrides(reg *BridgeChainRegistry) {
	if reg == nil {
		return
	}
	reg.ensure()
	if raw := strings.TrimSpace(os.Getenv("BRIDGE_CHAINS_JSON")); raw != "" {
		var list []*BridgeChainConfig
		if err := json.Unmarshal([]byte(raw), &list); err == nil {
			for _, cfg := range list {
				reg.Upsert(cfg)
			}
		} else {
			var wrapped BridgeChainRegistry
			if err := json.Unmarshal([]byte(raw), &wrapped); err == nil {
				for _, cfg := range wrapped.List() {
					reg.Upsert(cfg)
				}
			}
		}
	}
	enabledSet := map[string]bool{}
	for _, id := range bridgeCSV(os.Getenv("BRIDGE_ENABLED_CHAINS")) {
		enabledSet[normalizeBridgeChainID(id)] = true
	}
	for _, cfg := range reg.Chains {
		if cfg == nil {
			continue
		}
		prefix := bridgeEnvKey(cfg.ID)
		chainPrefix := bridgeEnvKey(cfg.ChainID)
		if value := bridgeEnv("BRIDGE_" + prefix + "_ID"); value != "" {
			cfg.ID = value
		}
		if value := bridgeEnv("BRIDGE_" + prefix + "_NAME"); value != "" {
			cfg.Name = value
		}
		if value := bridgeEnv("BRIDGE_" + prefix + "_CHAIN_ID"); value != "" {
			cfg.ChainID = value
		}
		if value := bridgeEnv("BRIDGE_" + prefix + "_FAMILY"); value != "" {
			cfg.Family = value
		}
		if value := bridgeEnv("BRIDGE_" + prefix + "_ADAPTER"); value != "" {
			cfg.Adapter = value
		}
		if value := bridgeEnv("BRIDGE_"+prefix+"_RPC", "BRIDGE_"+chainPrefix+"_RPC"); value != "" {
			cfg.RPC = value
		}
		if value := bridgeEnv("BRIDGE_"+prefix+"_RPCS", "BRIDGE_"+chainPrefix+"_RPCS"); value != "" {
			cfg.RPCs = bridgeCSV(value)
		}
		if value := bridgeEnv("BRIDGE_" + prefix + "_EXPLORER_URL"); value != "" {
			cfg.ExplorerURL = value
		}
		if value := bridgeEnv("BRIDGE_" + prefix + "_BRIDGE_ADDRESS"); value != "" {
			cfg.BridgeAddress = value
		}
		if value := bridgeEnv("BRIDGE_" + prefix + "_LOCK_ADDRESS"); value != "" {
			cfg.LockAddress = value
		}
		if value := bridgeEnv("BRIDGE_" + prefix + "_NATIVE_SYMBOL"); value != "" {
			cfg.NativeSymbol = value
		}
		cfg.NativeDecimals = bridgeEnvInt(cfg.NativeDecimals, "BRIDGE_"+prefix+"_NATIVE_DECIMALS", "BRIDGE_"+chainPrefix+"_NATIVE_DECIMALS")
		cfg.Confirmations = bridgeEnvInt(cfg.Confirmations, "BRIDGE_"+prefix+"_CONFIRMATIONS", "BRIDGE_"+chainPrefix+"_CONFIRMATIONS")
		cfg.FeeEstimate = bridgeEnvFloat(cfg.FeeEstimate, "BRIDGE_"+prefix+"_FEE_ESTIMATE", "BRIDGE_"+chainPrefix+"_FEE_ESTIMATE")
		if value := bridgeEnv("BRIDGE_" + prefix + "_FINALITY"); value != "" {
			cfg.Finality = value
		}
		if value := bridgeEnv("BRIDGE_" + prefix + "_DEPOSIT_ADDRESS"); value != "" {
			cfg.DepositAddress = value
		}
		if value := bridgeEnv("BRIDGE_" + prefix + "_VAULT_ADDRESS"); value != "" {
			cfg.VaultAddress = value
		}
		if value := bridgeEnv("BRIDGE_" + prefix + "_PROGRAM_ID"); value != "" {
			cfg.ProgramID = value
		}
		if value := bridgeEnv("BRIDGE_" + prefix + "_TOKEN_PROGRAM_ID"); value != "" {
			cfg.TokenProgramID = value
		}
		if value := bridgeEnv("BRIDGE_" + prefix + "_VERIFIER_URL"); value != "" {
			cfg.VerifierURL = value
		}
		cfg.SupportsPublic = bridgeEnvBool(cfg.SupportsPublic, "BRIDGE_"+prefix+"_SUPPORTS_PUBLIC")
		cfg.SupportsPrivate = bridgeEnvBool(cfg.SupportsPrivate, "BRIDGE_"+prefix+"_SUPPORTS_PRIVATE")
		cfg.Enabled = bridgeEnvBool(cfg.Enabled, "BRIDGE_"+prefix+"_ENABLED")
		if enabledSet[normalizeBridgeChainID(cfg.ID)] || enabledSet[normalizeBridgeChainID(cfg.ChainID)] {
			cfg.Enabled = true
		}
		if cfg.RPC == "" && len(cfg.RPCs) > 0 {
			cfg.RPC = cfg.RPCs[0]
		}
		if len(cfg.RPCs) == 0 && cfg.RPC != "" {
			cfg.RPCs = []string{cfg.RPC}
		}
		applyBridgeChainDefaults(cfg)
		bridgeChainDisableIfIncomplete(cfg)
	}
	if cfg := reg.Get("bsc-testnet"); cfg != nil {
		if value := bridgeEnv("BSC_TESTNET_RPC"); value != "" {
			cfg.RPC = value
		}
		if value := bridgeEnv("BSC_TESTNET_RPCS"); value != "" {
			cfg.RPCs = bridgeCSV(value)
		}
		if value := bridgeEnv("BSC_CHAIN_ID"); value != "" {
			cfg.ChainID = value
		}
		if value := bridgeEnv("BSC_BRIDGE_ADDRESS"); value != "" {
			cfg.BridgeAddress = value
		}
		if value := bridgeEnv("BSC_LOCK_ADDRESS"); value != "" {
			cfg.LockAddress = value
		}
		cfg.NativeDecimals = bridgeEnvInt(cfg.NativeDecimals, "BSC_NATIVE_DECIMALS")
		cfg.Confirmations = bridgeEnvInt(cfg.Confirmations, "BSC_CONFIRMATIONS")
		cfg.FeeEstimate = bridgeEnvFloat(cfg.FeeEstimate, "BSC_FEE_ESTIMATE")
		if cfg.RPC == "" && len(cfg.RPCs) > 0 {
			cfg.RPC = cfg.RPCs[0]
		}
		if len(cfg.RPCs) == 0 && cfg.RPC != "" {
			cfg.RPCs = []string{cfg.RPC}
		}
		cfg.Enabled = bridgeEnvBool(cfg.Enabled, "BSC_BRIDGE_ENABLED", "BRIDGE_BSC_TESTNET_ENABLED")
		applyBridgeChainDefaults(cfg)
		bridgeChainDisableIfIncomplete(cfg)
	}
}

func defaultBridgeChainRegistry() *BridgeChainRegistry {
	r := &BridgeChainRegistry{
		UpdatedAt: time.Now().Unix(),
		Chains:    make(map[string]*BridgeChainConfig),
	}
	bscRPC := strings.TrimSpace(os.Getenv("BSC_TESTNET_RPC"))
	bscBridge := strings.TrimSpace(os.Getenv("BSC_BRIDGE_ADDRESS"))
	bscLock := strings.TrimSpace(os.Getenv("BSC_LOCK_ADDRESS"))
	bscKey := strings.TrimSpace(os.Getenv("BSC_TESTNET_PRIVATE_KEY"))
	bscReady := bscRPC != "" && bscBridge != "" && bscLock != "" && bscKey != ""
	// Default to the current BSC testnet setup if configured.
	r.Upsert(&BridgeChainConfig{
		ID:              "bsc-testnet",
		Name:            "BSC Testnet",
		ChainID:         "97",
		Family:          "evm",
		Adapter:         "evm",
		RPC:             bscRPC,
		RPCs:            BridgeRPCEndpoints(bscRPC),
		ExplorerURL:     "https://testnet.bscscan.com",
		BridgeAddress:   bscBridge,
		LockAddress:     bscLock,
		NativeSymbol:    "BNB",
		NativeDecimals:  18,
		Confirmations:   15,
		Finality:        "confirmed",
		FeeEstimate:     0.002,
		Enabled:         bscReady || bridgeEnvBool(false, "BSC_BRIDGE_ENABLED", "BRIDGE_BSC_TESTNET_ENABLED"),
		SupportsPublic:  true,
		SupportsPrivate: true,
	})
	r.Upsert(&BridgeChainConfig{
		ID:              "ethereum-sepolia",
		Name:            "Ethereum Sepolia",
		ChainID:         "11155111",
		Family:          "evm",
		Adapter:         "evm",
		ExplorerURL:     "https://sepolia.etherscan.io",
		NativeSymbol:    "ETH",
		Enabled:         false,
		SupportsPublic:  true,
		SupportsPrivate: true,
	})
	r.Upsert(&BridgeChainConfig{
		ID:              "base-sepolia",
		Name:            "Base Sepolia",
		ChainID:         "84532",
		Family:          "evm",
		Adapter:         "evm",
		ExplorerURL:     "https://sepolia.basescan.org",
		NativeSymbol:    "ETH",
		Enabled:         false,
		SupportsPublic:  true,
		SupportsPrivate: true,
	})
	r.Upsert(&BridgeChainConfig{
		ID:              "polygon-amoy",
		Name:            "Polygon Amoy",
		ChainID:         "80002",
		Family:          "evm",
		Adapter:         "evm",
		ExplorerURL:     "https://amoy.polygonscan.com",
		NativeSymbol:    "POL",
		Enabled:         false,
		SupportsPublic:  true,
		SupportsPrivate: true,
	})
	r.Upsert(&BridgeChainConfig{
		ID:              "arbitrum-sepolia",
		Name:            "Arbitrum Sepolia",
		ChainID:         "421614",
		Family:          "evm",
		Adapter:         "evm",
		ExplorerURL:     "https://sepolia.arbiscan.io",
		NativeSymbol:    "ETH",
		Enabled:         false,
		SupportsPublic:  true,
		SupportsPrivate: true,
	})
	r.Upsert(&BridgeChainConfig{
		ID:              "optimism-sepolia",
		Name:            "Optimism Sepolia",
		ChainID:         "11155420",
		Family:          "evm",
		Adapter:         "evm",
		ExplorerURL:     "https://sepolia-optimism.etherscan.io",
		NativeSymbol:    "ETH",
		Enabled:         false,
		SupportsPublic:  true,
		SupportsPrivate: true,
	})
	r.Upsert(&BridgeChainConfig{
		ID:              "avalanche-fuji",
		Name:            "Avalanche Fuji",
		ChainID:         "43113",
		Family:          "evm",
		Adapter:         "evm",
		ExplorerURL:     "https://testnet.snowtrace.io",
		NativeSymbol:    "AVAX",
		Enabled:         false,
		SupportsPublic:  true,
		SupportsPrivate: true,
	})
	r.Upsert(&BridgeChainConfig{
		ID:              "linea-sepolia",
		Name:            "Linea Sepolia",
		ChainID:         "59141",
		Family:          "evm",
		Adapter:         "evm",
		ExplorerURL:     "https://sepolia.lineascan.build",
		NativeSymbol:    "ETH",
		Enabled:         false,
		SupportsPublic:  true,
		SupportsPrivate: true,
	})
	r.Upsert(&BridgeChainConfig{
		ID:              "monad-testnet",
		Name:            "Monad Testnet",
		ChainID:         "10143",
		Family:          "evm",
		Adapter:         "evm",
		NativeSymbol:    "MON",
		Enabled:         false,
		SupportsPublic:  true,
		SupportsPrivate: true,
	})
	r.Upsert(&BridgeChainConfig{
		ID:              "solana-devnet",
		Name:            "Solana Devnet",
		ChainID:         "solana-devnet",
		Family:          "solana",
		Adapter:         "solana",
		ExplorerURL:     "https://explorer.solana.com/?cluster=devnet",
		NativeSymbol:    "SOL",
		Enabled:         false,
		SupportsPublic:  true,
		SupportsPrivate: true,
	})
	r.Upsert(&BridgeChainConfig{
		ID:              "ton-testnet",
		Name:            "TON Testnet",
		ChainID:         "ton-testnet",
		Family:          "ton",
		Adapter:         "ton",
		NativeSymbol:    "TON",
		Enabled:         false,
		SupportsPublic:  true,
		SupportsPrivate: true,
	})
	r.Upsert(&BridgeChainConfig{
		ID:              "tron-shasta",
		Name:            "Tron Shasta",
		ChainID:         "tron-shasta",
		Family:          "tron",
		Adapter:         "tron",
		ExplorerURL:     "https://shasta.tronscan.org",
		NativeSymbol:    "TRX",
		Enabled:         false,
		SupportsPublic:  true,
		SupportsPrivate: true,
	})
	r.Upsert(&BridgeChainConfig{
		ID:              "aptos-testnet",
		Name:            "Aptos Testnet",
		ChainID:         "aptos-testnet",
		Family:          "aptos",
		Adapter:         "aptos",
		NativeSymbol:    "APT",
		Enabled:         false,
		SupportsPublic:  true,
		SupportsPrivate: true,
	})
	r.Upsert(&BridgeChainConfig{
		ID:              "sui-testnet",
		Name:            "Sui Testnet",
		ChainID:         "sui-testnet",
		Family:          "sui",
		Adapter:         "sui",
		NativeSymbol:    "SUI",
		Enabled:         false,
		SupportsPublic:  true,
		SupportsPrivate: true,
	})
	r.Upsert(&BridgeChainConfig{
		ID:              "starknet-sepolia",
		Name:            "Starknet Sepolia",
		ChainID:         "starknet-sepolia",
		Family:          "starknet",
		Adapter:         "starknet",
		ExplorerURL:     "https://sepolia.starkscan.co",
		NativeSymbol:    "ETH",
		Enabled:         false,
		SupportsPublic:  true,
		SupportsPrivate: true,
	})
	r.Upsert(&BridgeChainConfig{
		ID:              "near-testnet",
		Name:            "NEAR Testnet",
		ChainID:         "near-testnet",
		Family:          "near",
		Adapter:         "near",
		NativeSymbol:    "NEAR",
		Enabled:         false,
		SupportsPublic:  true,
		SupportsPrivate: true,
	})
	r.Upsert(&BridgeChainConfig{
		ID:              "cosmos-theta-testnet",
		Name:            "Cosmos Theta Testnet",
		ChainID:         "theta-testnet-001",
		Family:          "cosmos",
		Adapter:         "cosmos",
		NativeSymbol:    "ATOM",
		Enabled:         false,
		SupportsPublic:  true,
		SupportsPrivate: true,
	})
	r.Upsert(&BridgeChainConfig{
		ID:              "injective-testnet",
		Name:            "Injective Testnet",
		ChainID:         "injective-888",
		Family:          "cosmos",
		Adapter:         "cosmos",
		NativeSymbol:    "INJ",
		Enabled:         false,
		SupportsPublic:  true,
		SupportsPrivate: true,
	})
	r.Upsert(&BridgeChainConfig{
		ID:              "sei-atlantic",
		Name:            "Sei Atlantic Testnet",
		ChainID:         "atlantic-2",
		Family:          "cosmos",
		Adapter:         "cosmos",
		NativeSymbol:    "SEI",
		Enabled:         false,
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
	bridgeApplyEnvOverrides(r)
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
	bridgeApplyEnvOverrides(&reg)
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
	applyBridgeChainDefaults(cfg)
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
