package blockchaincomponent

import (
	"fmt"
	"strings"
)

type BridgeFamilySpec struct {
	ID                     string `json:"id"`
	Name                   string `json:"name"`
	Description            string `json:"description"`
	SupportsPublic         bool   `json:"supports_public"`
	SupportsPrivate        bool   `json:"supports_private"`
	RequiresBridgeContract bool   `json:"requires_bridge_contract"`
	RequiresLockContract   bool   `json:"requires_lock_contract"`
	RequiresExternalSigner bool   `json:"requires_external_signer"`
	Notes                  string `json:"notes,omitempty"`
}

type BridgeAdapter interface {
	ID() string
	Family() string
	Spec() *BridgeFamilySpec
	ValidateConfig(*BridgeChainConfig) error
}

type bridgeFamilyAdapter struct {
	spec *BridgeFamilySpec
}

func (a *bridgeFamilyAdapter) ID() string {
	if a == nil || a.spec == nil {
		return ""
	}
	return a.spec.ID
}

func (a *bridgeFamilyAdapter) Family() string {
	return a.ID()
}

func (a *bridgeFamilyAdapter) Spec() *BridgeFamilySpec {
	if a == nil || a.spec == nil {
		return nil
	}
	cp := *a.spec
	return &cp
}

func (a *bridgeFamilyAdapter) ValidateConfig(cfg *BridgeChainConfig) error {
	if cfg == nil {
		return nil
	}
	family := NormalizeBridgeFamilyID(cfg.Family)
	if family == "" {
		family = "evm"
	}
	if strings.TrimSpace(cfg.ID) == "" {
		return fmt.Errorf("bridge chain id is required")
	}
	if strings.TrimSpace(cfg.Name) == "" {
		return fmt.Errorf("%s bridge chain name is required", cfg.ID)
	}
	if strings.TrimSpace(cfg.ChainID) == "" {
		return fmt.Errorf("%s bridge chain_id is required", cfg.ID)
	}
	if !cfg.Enabled {
		return nil
	}
	if strings.TrimSpace(cfg.RPC) == "" && len(cfg.RPCs) == 0 {
		return fmt.Errorf("%s bridge requires rpc or rpcs before enabling", cfg.ID)
	}
	if bridgeChainIsEVMFamily(family) {
		if strings.TrimSpace(cfg.BridgeAddress) == "" {
			return fmt.Errorf("%s EVM bridge requires bridge_address before enabling", cfg.ID)
		}
		if strings.TrimSpace(cfg.LockAddress) == "" {
			return fmt.Errorf("%s EVM bridge requires lock_address before enabling", cfg.ID)
		}
		return nil
	}
	switch family {
	case "utxo", "bitcoin", "btc", "litecoin", "dogecoin", "cardano":
		if strings.TrimSpace(cfg.DepositAddress) == "" {
			return fmt.Errorf("%s bridge requires deposit_address before enabling", cfg.ID)
		}
		if strings.TrimSpace(cfg.VerifierURL) == "" {
			return fmt.Errorf("%s bridge requires verifier_url before enabling", cfg.ID)
		}
	case "solana", "aptos", "sui":
		if strings.TrimSpace(cfg.ProgramID) == "" && strings.TrimSpace(cfg.VaultAddress) == "" {
			return fmt.Errorf("%s bridge requires program_id or vault_address before enabling", cfg.ID)
		}
		if strings.TrimSpace(cfg.VerifierURL) == "" {
			return fmt.Errorf("%s bridge requires verifier_url before enabling", cfg.ID)
		}
	default:
		if strings.TrimSpace(cfg.VerifierURL) == "" {
			return fmt.Errorf("%s bridge requires verifier_url before enabling", cfg.ID)
		}
		if strings.TrimSpace(cfg.DepositAddress) == "" && strings.TrimSpace(cfg.VaultAddress) == "" && strings.TrimSpace(cfg.ProgramID) == "" {
			return fmt.Errorf("%s bridge requires deposit_address, vault_address, or program_id before enabling", cfg.ID)
		}
	}
	return nil
}

func familyNeedsExternalSourceData(family string) bool {
	switch NormalizeBridgeFamilyID(family) {
	case "utxo", "bitcoin", "btc", "litecoin", "dogecoin", "cardano", "cosmos", "cosmos-testnet", "sei", "injective", "solana", "substrate", "xrpl", "ton", "near", "aptos", "sui", "starknet", "tron":
		return true
	default:
		return false
	}
}

func ValidateBridgeRequestMetadata(family string, req *BridgeRequest) error {
	if req == nil {
		return nil
	}
	family = NormalizeBridgeFamilyID(family)
	if family == "" || family == "evm" {
		return nil
	}
	if !familyNeedsExternalSourceData(family) {
		return nil
	}
	if strings.TrimSpace(req.SourceTxHash) == "" {
		return fmt.Errorf("%s bridge requires source tx hash", strings.ToUpper(family))
	}
	if strings.TrimSpace(req.SourceAddress) == "" {
		return fmt.Errorf("%s bridge requires source address", strings.ToUpper(family))
	}
	switch family {
	case "cosmos", "cosmos-testnet", "sei", "injective":
		if strings.TrimSpace(req.SourceMemo) == "" {
			return fmt.Errorf("%s bridge requires memo", strings.ToUpper(family))
		}
	case "utxo", "bitcoin", "btc", "litecoin", "dogecoin", "cardano":
		if strings.TrimSpace(req.SourceOutput) == "" {
			return fmt.Errorf("%s bridge requires source output index", strings.ToUpper(family))
		}
	case "solana":
		if strings.TrimSpace(req.SourceSequence) == "" {
			return fmt.Errorf("SOLANA bridge requires recent blockhash or sequence")
		}
	case "substrate":
		if strings.TrimSpace(req.SourceSequence) == "" {
			return fmt.Errorf("SUBSTRATE bridge requires nonce or runtime sequence")
		}
	case "xrpl":
		if strings.TrimSpace(req.SourceSequence) == "" {
			return fmt.Errorf("XRPL bridge requires ledger sequence or delivery sequence")
		}
	case "ton":
		if strings.TrimSpace(req.SourceSequence) == "" {
			return fmt.Errorf("TON bridge requires message sequence or logical time")
		}
	case "near":
		if strings.TrimSpace(req.SourceSequence) == "" {
			return fmt.Errorf("NEAR bridge requires access key nonce or sequence")
		}
	case "aptos":
		if strings.TrimSpace(req.SourceSequence) == "" {
			return fmt.Errorf("APTOS bridge requires sequence number or ledger sequence")
		}
	case "sui":
		if strings.TrimSpace(req.SourceSequence) == "" {
			return fmt.Errorf("SUI bridge requires checkpoint or object version")
		}
	case "starknet":
		if strings.TrimSpace(req.SourceSequence) == "" {
			return fmt.Errorf("STARKNET bridge requires block number or transaction index")
		}
	case "tron":
		if strings.TrimSpace(req.SourceSequence) == "" {
			return fmt.Errorf("TRON bridge requires block number or transaction index")
		}
	}
	return nil
}

func bridgeFamilyDefaults(family string) (supportsPublic bool, supportsPrivate bool, requiresExternalSigner bool) {
	switch NormalizeBridgeFamilyID(family) {
	case "utxo", "cardano", "cosmos":
		return true, true, true
	case "substrate", "solana", "xrpl", "ton", "aptos", "sui", "near", "icp", "starknet", "tron":
		return true, true, true
	default:
		return true, true, false
	}
}

var bridgeFamilySpecs = []*BridgeFamilySpec{
	{
		ID:                     "evm",
		Name:                   "EVM",
		Description:            "Ethereum-compatible chains using RPC, gas, and contract-based lock/mint flows.",
		SupportsPublic:         true,
		SupportsPrivate:        true,
		RequiresBridgeContract: true,
		RequiresLockContract:   true,
		RequiresExternalSigner: false,
		Notes:                  "Current production path already supports this family.",
	},
	{
		ID:                     "utxo",
		Name:                   "UTXO",
		Description:            "Bitcoin-like chains such as Bitcoin, Litecoin, and Dogecoin.",
		SupportsPublic:         true,
		SupportsPrivate:        true,
		RequiresBridgeContract: false,
		RequiresLockContract:   false,
		RequiresExternalSigner: true,
		Notes:                  "Uses PSBT-style or native UTXO signing and chain-specific finality.",
	},
	{
		ID:                     "cosmos",
		Name:                   "Cosmos",
		Description:            "Cosmos SDK / Tendermint chains such as Cosmos, Osmosis, Injective, Kava, and Sei.",
		SupportsPublic:         true,
		SupportsPrivate:        true,
		RequiresBridgeContract: false,
		RequiresLockContract:   false,
		RequiresExternalSigner: true,
		Notes:                  "Requires protobuf/SignDoc-based signing and chain-specific finality handling.",
	},
	{
		ID:                     "substrate",
		Name:                   "Substrate",
		Description:            "Polkadot/Substrate-family chains such as Polkadot, Moonbeam, and Astar.",
		SupportsPublic:         true,
		SupportsPrivate:        true,
		RequiresBridgeContract: false,
		RequiresLockContract:   false,
		RequiresExternalSigner: true,
		Notes:                  "Requires runtime metadata, nonce/spec version, and Substrate-style signing.",
	},
	{
		ID:                     "solana",
		Name:                   "Solana",
		Description:            "Solana / SPL ecosystem with recent blockhash and ed25519 signing.",
		SupportsPublic:         true,
		SupportsPrivate:        true,
		RequiresBridgeContract: false,
		RequiresLockContract:   false,
		RequiresExternalSigner: true,
		Notes:                  "Requires commitment/finality polling, recent blockhash, and Solana transaction assembly.",
	},
	{
		ID:                     "xrpl",
		Name:                   "XRPL",
		Description:            "XRP Ledger with ledger-based finality and XRPL transaction model.",
		SupportsPublic:         true,
		SupportsPrivate:        true,
		RequiresBridgeContract: false,
		RequiresLockContract:   false,
		RequiresExternalSigner: true,
		Notes:                  "Requires ledger sequence, trust lines, and XRPL signing rules.",
	},
	{
		ID:                     "tron",
		Name:                   "Tron",
		Description:            "Tron/TRC-20 chain using TronGrid-style RPC and TRC-20 token contracts.",
		SupportsPublic:         true,
		SupportsPrivate:        true,
		RequiresBridgeContract: false,
		RequiresLockContract:   false,
		RequiresExternalSigner: true,
		Notes:                  "Requires Tron-native transaction construction, bandwidth/energy handling, and finality polling.",
	},
	{
		ID:                     "ton",
		Name:                   "TON",
		Description:            "The Open Network with TON-native message and account model.",
		SupportsPublic:         true,
		SupportsPrivate:        true,
		RequiresBridgeContract: false,
		RequiresLockContract:   false,
		RequiresExternalSigner: true,
		Notes:                  "Requires TON wallet/message handling and account-based finality.",
	},
	{
		ID:                     "cardano",
		Name:                   "Cardano",
		Description:            "Cardano with UTxO-like model, different signing and finality assumptions.",
		SupportsPublic:         true,
		SupportsPrivate:        true,
		RequiresBridgeContract: false,
		RequiresLockContract:   false,
		RequiresExternalSigner: true,
		Notes:                  "Cardano-specific transaction building and witness handling required.",
	},
	{
		ID:                     "aptos",
		Name:                   "Aptos",
		Description:            "Aptos Move-based chain with its own transaction signing and sequencing.",
		SupportsPublic:         true,
		SupportsPrivate:        true,
		RequiresBridgeContract: false,
		RequiresLockContract:   false,
		RequiresExternalSigner: true,
	},
	{
		ID:                     "sui",
		Name:                   "Sui",
		Description:            "Sui object-based chain with chain-specific signing and execution model.",
		SupportsPublic:         true,
		SupportsPrivate:        true,
		RequiresBridgeContract: false,
		RequiresLockContract:   false,
		RequiresExternalSigner: true,
	},
	{
		ID:                     "starknet",
		Name:                   "Starknet",
		Description:            "Starknet account abstraction and Cairo contract-based bridge routes.",
		SupportsPublic:         true,
		SupportsPrivate:        true,
		RequiresBridgeContract: false,
		RequiresLockContract:   false,
		RequiresExternalSigner: true,
	},
	{
		ID:                     "near",
		Name:                   "NEAR",
		Description:            "NEAR protocol with its own account, access key, and finality model.",
		SupportsPublic:         true,
		SupportsPrivate:        true,
		RequiresBridgeContract: false,
		RequiresLockContract:   false,
		RequiresExternalSigner: true,
	},
	{
		ID:                     "icp",
		Name:                   "ICP",
		Description:            "Internet Computer with canister-centric execution and distinct signing flow.",
		SupportsPublic:         true,
		SupportsPrivate:        true,
		RequiresBridgeContract: false,
		RequiresLockContract:   false,
		RequiresExternalSigner: true,
	},
}

func NormalizeBridgeFamilyID(id string) string {
	return strings.ToLower(strings.TrimSpace(id))
}

func SupportedBridgeFamilies() []*BridgeFamilySpec {
	out := make([]*BridgeFamilySpec, 0, len(bridgeFamilySpecs))
	for _, spec := range bridgeFamilySpecs {
		if spec == nil {
			continue
		}
		cp := *spec
		out = append(out, &cp)
	}
	return out
}

func BridgeFamilyByID(id string) *BridgeFamilySpec {
	needle := NormalizeBridgeFamilyID(id)
	if needle == "" {
		return nil
	}
	for _, spec := range bridgeFamilySpecs {
		if spec != nil && NormalizeBridgeFamilyID(spec.ID) == needle {
			cp := *spec
			return &cp
		}
	}
	return nil
}

func BridgeAdapterByFamily(id string) BridgeAdapter {
	spec := BridgeFamilyByID(id)
	if spec == nil {
		return nil
	}
	return &bridgeFamilyAdapter{spec: spec}
}
