package blockchaincomponent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"strings"

	constantset "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/ConstantSet"
)

const (
	CurrentProtocolVersion uint32 = 2
	CurrentStateVersion    uint32 = 4
	DefaultEpochLength     uint64 = 100
)

// ChainSpec is the consensus-critical, versioned description of a PoDL chain.
// It intentionally contains no maps so its canonical JSON representation is
// stable across nodes and Go versions.
type ChainSpec struct {
	Name                string `json:"name"`
	ChainID             uint   `json:"chain_id"`
	NetworkID           string `json:"network_id"`
	ProtocolVersion     uint32 `json:"protocol_version"`
	StateVersion        uint32 `json:"state_version"`
	GenesisHash         string `json:"genesis_hash"`
	BlockTimeMS         uint64 `json:"block_time_ms"`
	EpochLength         uint64 `json:"epoch_length"`
	MaxBlockGas         uint64 `json:"max_block_gas"`
	MaxBlockBytes       uint64 `json:"max_block_bytes"`
	BFTQuorumBPS        uint32 `json:"bft_quorum_bps"`
	AllowLegacyFinality bool   `json:"allow_legacy_finality"`
}

func DefaultChainSpec(genesisHash string) ChainSpec {
	return ChainSpec{
		Name:                "Proof of Dynamic Liquidity",
		ChainID:             constantset.ChainID,
		NetworkID:           "podl-v2",
		ProtocolVersion:     CurrentProtocolVersion,
		StateVersion:        CurrentStateVersion,
		GenesisHash:         strings.TrimSpace(genesisHash),
		BlockTimeMS:         2000,
		EpochLength:         DefaultEpochLength,
		MaxBlockGas:         uint64(constantset.MaxBlockGas),
		MaxBlockBytes:       uint64(constantset.MaxBlockSize),
		BFTQuorumBPS:        6667,
		AllowLegacyFinality: true,
	}
}

func (s ChainSpec) Validate() error {
	if strings.TrimSpace(s.Name) == "" || strings.TrimSpace(s.NetworkID) == "" {
		return fmt.Errorf("chain name and network id are required")
	}
	if s.ChainID == 0 || s.ProtocolVersion == 0 || s.StateVersion == 0 {
		return fmt.Errorf("chain, protocol and state versions must be non-zero")
	}
	if strings.TrimSpace(s.GenesisHash) == "" {
		return fmt.Errorf("genesis hash is required")
	}
	if s.BlockTimeMS < 250 || s.EpochLength < 10 {
		return fmt.Errorf("unsafe block time or epoch length")
	}
	if s.MaxBlockGas == 0 || s.MaxBlockBytes == 0 {
		return fmt.Errorf("block limits must be non-zero")
	}
	if s.BFTQuorumBPS < 6667 || s.BFTQuorumBPS > 10000 {
		return fmt.Errorf("bft quorum must be between 6667 and 10000 bps")
	}
	return nil
}

func (s ChainSpec) Hash() string {
	raw, _ := json.Marshal(s)
	sum := sha256.Sum256(raw)
	return "0x" + hex.EncodeToString(sum[:])
}

type canonicalAccount struct {
	Address string `json:"address"`
	Balance string `json:"balance"`
}

type canonicalValidator struct {
	Address        string  `json:"address"`
	NativeBond     float64 `json:"native_bond"`
	LiquidityPower float64 `json:"liquidity_power"`
	PenaltyScore   float64 `json:"penalty_score"`
	JailedUntil    int64   `json:"jailed_until"`
}

type canonicalVault struct {
	ID          string `json:"id"`
	Owner       string `json:"owner"`
	CurrentPool string `json:"current_pool"`
	TokenA      string `json:"token_a"`
	TokenB      string `json:"token_b"`
	AmountA     string `json:"amount_a"`
	AmountB     string `json:"amount_b"`
	Shares      string `json:"shares"`
	Status      string `json:"status"`
}

type canonicalState struct {
	Version              uint32                       `json:"version"`
	Height               uint64                       `json:"height"`
	BaseFee              uint64                       `json:"base_fee"`
	MinStake             float64                      `json:"min_stake"`
	MinLiquidityStake    string                       `json:"min_liquidity_stake"`
	FixedBlockReward     uint64                       `json:"fixed_block_reward"`
	GasRewardMultiplier  uint64                       `json:"gas_reward_multiplier"`
	SlashingPool         float64                      `json:"slashing_pool"`
	Accounts             []canonicalAccount           `json:"accounts"`
	Validators           []canonicalValidator         `json:"validators"`
	Vaults               []canonicalVault             `json:"vaults"`
	VaultMoves           []StrategyVaultMovement      `json:"vault_moves,omitempty"`
	VaultSafety          StrategyVaultSafetyConfig    `json:"vault_safety"`
	Contracts            []canonicalContract          `json:"contracts,omitempty"`
	LiquidityLocks       []canonicalLiquidityLocks    `json:"liquidity_locks,omitempty"`
	TotalLiquidity       string                       `json:"total_liquidity"`
	PendingFeePool       []canonicalEconomicBalance   `json:"pending_fee_pool,omitempty"`
	PoolLiquidity        []canonicalEconomicBalance   `json:"pool_liquidity,omitempty"`
	UnallocatedLiquidity string                       `json:"unallocated_liquidity"`
	LiquidityProviders   []canonicalLiquidityProvider `json:"liquidity_providers,omitempty"`
	DynamicOracleSignals []canonicalDynamicSignal     `json:"dynamic_oracle_signals,omitempty"`
	PoolPriceHistory     []canonicalPriceHistory      `json:"pool_price_history,omitempty"`
	OracleObservations   []OracleObservation          `json:"oracle_observations,omitempty"`
	OraclePublishers     []canonicalOraclePublisher   `json:"oracle_publishers,omitempty"`
	OracleNonces         []canonicalHeight            `json:"oracle_nonces,omitempty"`
	PairRiskPolicies     []PairRiskPolicy             `json:"pair_risk_policies,omitempty"`
	CongestionProfile    []CongestionBucket           `json:"congestion_profile,omitempty"`
	EconomicPolicy       EconomicPolicy               `json:"economic_policy"`
	EconomicBalances     []canonicalEconomicBalance   `json:"economic_balances,omitempty"`
	Governance           *GovernanceState             `json:"governance,omitempty"`
	ProtocolPauses       []canonicalBool              `json:"protocol_pauses,omitempty"`
	TotalBurned          string                       `json:"total_burned"`
	ArbPolicy            ProtocolArbPolicy            `json:"arb_policy"`
	ArbAuctions          []*ArbAuction                `json:"arb_auctions,omitempty"`
	ArbKeeperBonds       []canonicalEconomicBalance   `json:"arb_keeper_bonds,omitempty"`
	ArbKeeperUnbondAt    []canonicalHeight            `json:"arb_keeper_unbond_at,omitempty"`
	SlashingCases        []*SlashingCase              `json:"slashing_cases,omitempty"`
	BridgeSecurity       *BridgeSecurityState         `json:"bridge_security,omitempty"`
	BridgeRequests       []*BridgeRequest             `json:"bridge_requests,omitempty"`
	BridgeTokens         []canonicalBridgeToken       `json:"bridge_tokens,omitempty"`
	BusinessAgreements   []*LiquidityServiceAgreement `json:"business_agreements,omitempty"`
	TreasuryDeployments  []TreasuryDeployment         `json:"treasury_deployments,omitempty"`
	ConsensusPolicy      canonicalConsensusPolicy     `json:"consensus_policy"`
	ProtocolRevenue      []ProtocolRevenueEntry       `json:"protocol_revenue,omitempty"`
	RevenueCheckpoints   []canonicalEconomicBalance   `json:"revenue_checkpoints,omitempty"`
	RevenueAssets        []canonicalEconomicBalance   `json:"revenue_assets,omitempty"`
	CumulativeEmission   string                       `json:"cumulative_emission"`
}

type canonicalConsensusPolicy struct {
	EpochLength           uint64 `json:"epoch_length"`
	MaxLiquidityCreditBPS uint32 `json:"max_liquidity_credit_bps"`
	RoundTimeoutSeconds   int64  `json:"round_timeout_seconds"`
}

type canonicalEconomicBalance struct {
	Bucket string `json:"bucket"`
	Amount string `json:"amount"`
}
type canonicalBool struct {
	Key   string `json:"key"`
	Value bool   `json:"value"`
}
type canonicalHeight struct {
	Key    string `json:"key"`
	Height uint64 `json:"height"`
}

type canonicalOraclePublisher struct {
	Source  string `json:"source"`
	Address string `json:"address"`
	Nonce   uint64 `json:"nonce"`
}

type canonicalLiquidityProvider struct {
	Address          string  `json:"address"`
	Stake            string  `json:"stake"`
	LiquidityPower   float64 `json:"liquidity_power"`
	LockTime         int64   `json:"lock_time"`
	LockDays         int64   `json:"lock_days"`
	PendingRewards   string  `json:"pending_rewards"`
	TotalRewards     string  `json:"total_rewards"`
	UnstakeAmount    string  `json:"unstake_amount"`
	ReleasedSoFar    string  `json:"released_so_far"`
	IsUnstaking      bool    `json:"is_unstaking"`
	UnstakeStartTime int64   `json:"unstake_start_time"`
}

type canonicalLiquidityLocks struct {
	Address string       `json:"address"`
	Records []LockRecord `json:"records"`
}

type canonicalDynamicSignal struct {
	Key    string                       `json:"key"`
	Signal DynamicLiquidityOracleSignal `json:"signal"`
}

type canonicalPriceHistory struct {
	Pair         string                 `json:"pair"`
	Observations []PoolPriceObservation `json:"observations"`
}

type canonicalBridgeToken struct {
	Key   string          `json:"key"`
	Token BridgeTokenInfo `json:"token"`
}

type canonicalStorage struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type canonicalContract struct {
	Address string             `json:"address"`
	Storage []canonicalStorage `json:"storage"`
}

func amountOrZero(v *big.Int) string {
	if v == nil {
		return "0"
	}
	return v.String()
}

// ComputeDeterministicStateRoot commits the consensus-visible account,
// validator and strategy-vault state using stable sorting and integer strings.
func (bc *Blockchain_struct) ComputeDeterministicStateRoot() string {
	return bc.ComputeDeterministicStateRootAt(bc.LatestBlockNumber())
}

func (bc *Blockchain_struct) ComputeDeterministicStateRootAt(height uint64) string {
	if bc == nil {
		return ""
	}
	state := canonicalState{Version: CurrentStateVersion, Height: height, BaseFee: bc.BaseFee, MinStake: bc.MinStake, MinLiquidityStake: amountOrZero(bc.MinLiquidityStake), FixedBlockReward: bc.FixedBlockReward, GasRewardMultiplier: bc.GasRewardMultiplier, SlashingPool: bc.SlashingPool, VaultSafety: bc.StrategyVaultSafety, TotalLiquidity: amountOrZero(bc.TotalLiquidity), UnallocatedLiquidity: amountOrZero(bc.UnallocatedLiquidity), EconomicPolicy: bc.EconomicPolicy, Governance: bc.Governance, TotalBurned: amountOrZero(bc.TotalBurned), ArbPolicy: bc.ArbPolicy, BridgeSecurity: bc.BridgeSecurity, CumulativeEmission: amountOrZero(bc.CumulativeEmission)}
	appendBalances := func(dst *[]canonicalEconomicBalance, values map[string]*big.Int) {
		for key, amount := range values {
			*dst = append(*dst, canonicalEconomicBalance{Bucket: strings.ToLower(strings.TrimSpace(key)), Amount: amountOrZero(amount)})
		}
		sort.Slice(*dst, func(i, j int) bool { return (*dst)[i].Bucket < (*dst)[j].Bucket })
	}
	appendBalances(&state.PendingFeePool, bc.PendingFeePool)
	appendBalances(&state.PoolLiquidity, bc.PoolLiquidity)
	for address, records := range bc.LiquidityLocks {
		state.LiquidityLocks = append(state.LiquidityLocks, canonicalLiquidityLocks{Address: strings.ToLower(strings.TrimSpace(address)), Records: append([]LockRecord(nil), records...)})
	}
	sort.Slice(state.LiquidityLocks, func(i, j int) bool { return state.LiquidityLocks[i].Address < state.LiquidityLocks[j].Address })
	state.VaultMoves = append([]StrategyVaultMovement(nil), bc.StrategyVaultMoves...)
	sort.Slice(state.VaultMoves, func(i, j int) bool { return state.VaultMoves[i].ID < state.VaultMoves[j].ID })
	for key, signal := range bc.DynamicLiquidityOracleSignals {
		state.DynamicOracleSignals = append(state.DynamicOracleSignals, canonicalDynamicSignal{Key: strings.ToLower(strings.TrimSpace(key)), Signal: signal})
	}
	sort.Slice(state.DynamicOracleSignals, func(i, j int) bool { return state.DynamicOracleSignals[i].Key < state.DynamicOracleSignals[j].Key })
	for pair, observations := range bc.PoolPriceHistory {
		rows := append([]PoolPriceObservation(nil), observations...)
		sort.Slice(rows, func(i, j int) bool {
			if rows[i].Timestamp == rows[j].Timestamp {
				return rows[i].Price < rows[j].Price
			}
			return rows[i].Timestamp < rows[j].Timestamp
		})
		state.PoolPriceHistory = append(state.PoolPriceHistory, canonicalPriceHistory{Pair: strings.ToLower(strings.TrimSpace(pair)), Observations: rows})
	}
	sort.Slice(state.PoolPriceHistory, func(i, j int) bool { return state.PoolPriceHistory[i].Pair < state.PoolPriceHistory[j].Pair })
	for _, request := range bc.BridgeRequests {
		if request != nil {
			state.BridgeRequests = append(state.BridgeRequests, request)
		}
	}
	sort.Slice(state.BridgeRequests, func(i, j int) bool { return state.BridgeRequests[i].ID < state.BridgeRequests[j].ID })
	for key, token := range bc.BridgeTokenMap {
		if token != nil {
			state.BridgeTokens = append(state.BridgeTokens, canonicalBridgeToken{Key: strings.ToLower(strings.TrimSpace(key)), Token: *token})
		}
	}
	sort.Slice(state.BridgeTokens, func(i, j int) bool { return state.BridgeTokens[i].Key < state.BridgeTokens[j].Key })
	for _, agreement := range bc.BusinessAgreements {
		if agreement != nil {
			state.BusinessAgreements = append(state.BusinessAgreements, agreement)
		}
	}
	sort.Slice(state.BusinessAgreements, func(i, j int) bool { return state.BusinessAgreements[i].ID < state.BusinessAgreements[j].ID })
	state.TreasuryDeployments = append([]TreasuryDeployment(nil), bc.TreasuryDeployments...)
	sort.Slice(state.TreasuryDeployments, func(i, j int) bool { return state.TreasuryDeployments[i].ID < state.TreasuryDeployments[j].ID })
	for _, auction := range bc.ArbAuctions {
		if auction != nil {
			state.ArbAuctions = append(state.ArbAuctions, auction)
		}
	}
	sort.Slice(state.ArbAuctions, func(i, j int) bool { return state.ArbAuctions[i].ID < state.ArbAuctions[j].ID })
	for keeper, bond := range bc.ArbKeeperBonds {
		state.ArbKeeperBonds = append(state.ArbKeeperBonds, canonicalEconomicBalance{Bucket: strings.ToLower(keeper), Amount: amountOrZero(bond)})
	}
	sort.Slice(state.ArbKeeperBonds, func(i, j int) bool { return state.ArbKeeperBonds[i].Bucket < state.ArbKeeperBonds[j].Bucket })
	for keeper, unlock := range bc.ArbKeeperUnbondAt {
		state.ArbKeeperUnbondAt = append(state.ArbKeeperUnbondAt, canonicalHeight{Key: strings.ToLower(keeper), Height: unlock})
	}
	sort.Slice(state.ArbKeeperUnbondAt, func(i, j int) bool { return state.ArbKeeperUnbondAt[i].Key < state.ArbKeeperUnbondAt[j].Key })
	for _, slashingCase := range bc.SlashingCases {
		if slashingCase != nil {
			state.SlashingCases = append(state.SlashingCases, slashingCase)
		}
	}
	sort.Slice(state.SlashingCases, func(i, j int) bool { return state.SlashingCases[i].ID < state.SlashingCases[j].ID })
	state.ProtocolRevenue = append([]ProtocolRevenueEntry(nil), bc.ProtocolRevenue...)
	if bc.ConsensusV2 != nil {
		state.ConsensusPolicy = canonicalConsensusPolicy{EpochLength: bc.ConsensusV2.EpochLength, MaxLiquidityCreditBPS: bc.ConsensusV2.MaxLiquidityCreditBPS, RoundTimeoutSeconds: bc.ConsensusV2.RoundTimeoutSeconds}
	}
	for bucket, amount := range bc.EconomicBalances {
		state.EconomicBalances = append(state.EconomicBalances, canonicalEconomicBalance{Bucket: bucket, Amount: amountOrZero(amount)})
	}
	sort.Slice(state.EconomicBalances, func(i, j int) bool { return state.EconomicBalances[i].Bucket < state.EconomicBalances[j].Bucket })
	for key, amount := range bc.RevenueCheckpoints {
		state.RevenueCheckpoints = append(state.RevenueCheckpoints, canonicalEconomicBalance{Bucket: key, Amount: amountOrZero(amount)})
	}
	sort.Slice(state.RevenueCheckpoints, func(i, j int) bool { return state.RevenueCheckpoints[i].Bucket < state.RevenueCheckpoints[j].Bucket })
	for asset, amount := range bc.CapturedRevenueAssets {
		state.RevenueAssets = append(state.RevenueAssets, canonicalEconomicBalance{Bucket: asset, Amount: amountOrZero(amount)})
	}
	sort.Slice(state.RevenueAssets, func(i, j int) bool { return state.RevenueAssets[i].Bucket < state.RevenueAssets[j].Bucket })
	for key, value := range bc.ProtocolPauses {
		state.ProtocolPauses = append(state.ProtocolPauses, canonicalBool{Key: key, Value: value})
	}
	sort.Slice(state.ProtocolPauses, func(i, j int) bool { return state.ProtocolPauses[i].Key < state.ProtocolPauses[j].Key })

	bc.AccountsMu.RLock()
	for address, balance := range bc.Accounts {
		state.Accounts = append(state.Accounts, canonicalAccount{
			Address: strings.ToLower(strings.TrimSpace(address)),
			Balance: amountOrZero(balance),
		})
	}
	bc.AccountsMu.RUnlock()
	sort.Slice(state.Accounts, func(i, j int) bool { return state.Accounts[i].Address < state.Accounts[j].Address })

	for _, validator := range bc.Validators {
		if validator == nil {
			continue
		}
		state.Validators = append(state.Validators, canonicalValidator{
			Address:        strings.ToLower(strings.TrimSpace(validator.Address)),
			NativeBond:     validator.NativeBond,
			LiquidityPower: validator.LiquidityPower,
			PenaltyScore:   validator.PenaltyScore,
			JailedUntil:    validator.JailedUntil.Unix(),
		})
	}
	sort.Slice(state.Validators, func(i, j int) bool { return state.Validators[i].Address < state.Validators[j].Address })

	for _, vault := range bc.StrategyVaults {
		if vault == nil {
			continue
		}
		state.Vaults = append(state.Vaults, canonicalVault{
			ID:          vault.ID,
			Owner:       strings.ToLower(strings.TrimSpace(vault.Owner)),
			CurrentPool: strings.ToLower(strings.TrimSpace(vault.CurrentPool)),
			TokenA:      strings.ToLower(strings.TrimSpace(vault.TokenA)),
			TokenB:      strings.ToLower(strings.TrimSpace(vault.TokenB)),
			AmountA:     amountOrZero(vault.AmountA),
			AmountB:     amountOrZero(vault.AmountB),
			Shares:      amountOrZero(vault.Shares),
			Status:      vault.Status,
		})
	}
	sort.Slice(state.Vaults, func(i, j int) bool { return state.Vaults[i].ID < state.Vaults[j].ID })
	for _, provider := range bc.LiquidityProviders {
		if provider == nil {
			continue
		}
		state.LiquidityProviders = append(state.LiquidityProviders, canonicalLiquidityProvider{
			Address: strings.ToLower(strings.TrimSpace(provider.Address)), Stake: amountOrZero(provider.StakeAmount), LiquidityPower: provider.LiquidityPower, LockTime: provider.LockTime, LockDays: provider.LockDays,
			PendingRewards: amountOrZero(provider.PendingRewards), TotalRewards: amountOrZero(provider.TotalRewards),
			UnstakeAmount: amountOrZero(provider.UnstakeAmount), ReleasedSoFar: amountOrZero(provider.ReleasedSoFar), IsUnstaking: provider.IsUnstaking, UnstakeStartTime: provider.UnstakeStartTime,
		})
	}
	sort.Slice(state.LiquidityProviders, func(i, j int) bool {
		return state.LiquidityProviders[i].Address < state.LiquidityProviders[j].Address
	})
	for _, bySource := range bc.OracleObservations {
		for _, observation := range bySource {
			state.OracleObservations = append(state.OracleObservations, observation)
		}
	}
	sort.Slice(state.OracleObservations, func(i, j int) bool {
		if state.OracleObservations[i].Asset == state.OracleObservations[j].Asset {
			return state.OracleObservations[i].Source < state.OracleObservations[j].Source
		}
		return state.OracleObservations[i].Asset < state.OracleObservations[j].Asset
	})
	for source, address := range bc.OraclePublishers {
		state.OraclePublishers = append(state.OraclePublishers, canonicalOraclePublisher{Source: source, Address: strings.ToLower(address), Nonce: bc.OracleNonces[source]})
	}
	sort.Slice(state.OraclePublishers, func(i, j int) bool { return state.OraclePublishers[i].Source < state.OraclePublishers[j].Source })
	for source, nonce := range bc.OracleNonces {
		state.OracleNonces = append(state.OracleNonces, canonicalHeight{Key: strings.ToLower(strings.TrimSpace(source)), Height: nonce})
	}
	sort.Slice(state.OracleNonces, func(i, j int) bool { return state.OracleNonces[i].Key < state.OracleNonces[j].Key })
	for _, policy := range bc.PairRiskPolicies {
		state.PairRiskPolicies = append(state.PairRiskPolicies, policy)
	}
	sort.Slice(state.PairRiskPolicies, func(i, j int) bool {
		return state.PairRiskPolicies[i].PairAddress < state.PairRiskPolicies[j].PairAddress
	})
	for _, bucket := range bc.CongestionProfile {
		state.CongestionProfile = append(state.CongestionProfile, bucket)
	}
	sort.Slice(state.CongestionProfile, func(i, j int) bool { return state.CongestionProfile[i].HourUTC < state.CongestionProfile[j].HourUTC })

	if bc.ContractEngine != nil && bc.ContractEngine.DB != nil {
		for _, address := range bc.ContractEngine.DB.ListContractAddresses() {
			values, err := bc.ContractEngine.DB.LoadAllStorage(address)
			if err != nil {
				return ""
			}
			contract := canonicalContract{Address: strings.ToLower(strings.TrimSpace(address))}
			for key, value := range values {
				contract.Storage = append(contract.Storage, canonicalStorage{Key: key, Value: value})
			}
			sort.Slice(contract.Storage, func(i, j int) bool { return contract.Storage[i].Key < contract.Storage[j].Key })
			state.Contracts = append(state.Contracts, contract)
		}
		sort.Slice(state.Contracts, func(i, j int) bool { return state.Contracts[i].Address < state.Contracts[j].Address })
	}

	raw, err := json.Marshal(state)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return "0x" + hex.EncodeToString(sum[:])
}

func (bc *Blockchain_struct) ProtocolStatus() map[string]interface{} {
	if bc == nil {
		return map[string]interface{}{"ready": false}
	}
	bc.EnsureRuntimeState()
	return map[string]interface{}{
		"ready":              bc.ChainSpec.Validate() == nil,
		"chain_spec":         bc.ChainSpec,
		"chain_spec_hash":    bc.ChainSpec.Hash(),
		"current_state_root": bc.ComputeDeterministicStateRoot(),
	}
}
