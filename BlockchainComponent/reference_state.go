package blockchaincomponent

// This file is intentionally a second implementation of state canonicalization.
// Do not refactor it to call ComputeDeterministicStateRootAt: equivalence tests
// are useful only while the two code paths remain mechanically independent.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"sort"
	"strings"
)

func referenceAmount(v *big.Int) string {
	if v == nil {
		return "0"
	}
	return v.String()
}

type referenceKeyValue struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type referenceNamedAmount struct {
	Name   string `json:"bucket"`
	Amount string `json:"amount"`
}

type referenceAccount struct {
	Address string `json:"address"`
	Balance string `json:"balance"`
}

type referenceValidator struct {
	Address        string  `json:"address"`
	NativeBond     float64 `json:"native_bond"`
	LiquidityPower float64 `json:"liquidity_power"`
	PenaltyScore   float64 `json:"penalty_score"`
	JailedUntil    int64   `json:"jailed_until"`
}

type referenceVault struct {
	ID, Owner, CurrentPool, TokenA, TokenB, AmountA, AmountB, Shares, Status string
}

func (v referenceVault) MarshalJSON() ([]byte, error) {
	// Explicit field order and names match the protocol specification without
	// sharing the primary canonicalState type.
	return json.Marshal(struct {
		ID          string `json:"id"`
		Owner       string `json:"owner"`
		CurrentPool string `json:"current_pool"`
		TokenA      string `json:"token_a"`
		TokenB      string `json:"token_b"`
		AmountA     string `json:"amount_a"`
		AmountB     string `json:"amount_b"`
		Shares      string `json:"shares"`
		Status      string `json:"status"`
	}{v.ID, v.Owner, v.CurrentPool, v.TokenA, v.TokenB, v.AmountA, v.AmountB, v.Shares, v.Status})
}

type referenceProvider struct {
	Address, Stake, PendingRewards, TotalRewards, UnstakeAmount, ReleasedSoFar string
	LiquidityPower                                                             float64
	LockTime, LockDays                                                         int64
	IsUnstaking                                                                bool
	UnstakeStartTime                                                           int64
}

func (p referenceProvider) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
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
	}{p.Address, p.Stake, p.LiquidityPower, p.LockTime, p.LockDays, p.PendingRewards, p.TotalRewards, p.UnstakeAmount, p.ReleasedSoFar, p.IsUnstaking, p.UnstakeStartTime})
}

type referenceLiquidityLocks struct {
	Address string       `json:"address"`
	Records []LockRecord `json:"records"`
}

type referenceDynamicSignal struct {
	Key    string                       `json:"key"`
	Signal DynamicLiquidityOracleSignal `json:"signal"`
}

type referencePriceHistory struct {
	Pair         string                 `json:"pair"`
	Observations []PoolPriceObservation `json:"observations"`
}

type referenceBridgeToken struct {
	Key   string          `json:"key"`
	Token BridgeTokenInfo `json:"token"`
}

type referencePublisher struct {
	Source  string `json:"source"`
	Address string `json:"address"`
	Nonce   uint64 `json:"nonce"`
}

type referenceBool struct {
	Key   string `json:"key"`
	Value bool   `json:"value"`
}
type referenceHeight struct {
	Key    string `json:"key"`
	Height uint64 `json:"height"`
}

type referenceContract struct {
	Address string              `json:"address"`
	Storage []referenceKeyValue `json:"storage"`
}

type referenceConsensusPolicy struct {
	EpochLength           uint64 `json:"epoch_length"`
	MaxLiquidityCreditBPS uint32 `json:"max_liquidity_credit_bps"`
	RoundTimeoutSeconds   int64  `json:"round_timeout_seconds"`
}

type referenceCanonicalState struct {
	Version              uint32                       `json:"version"`
	Height               uint64                       `json:"height"`
	BaseFee              uint64                       `json:"base_fee"`
	MinStake             float64                      `json:"min_stake"`
	MinLiquidityStake    string                       `json:"min_liquidity_stake"`
	FixedBlockReward     uint64                       `json:"fixed_block_reward"`
	GasRewardMultiplier  uint64                       `json:"gas_reward_multiplier"`
	SlashingPool         float64                      `json:"slashing_pool"`
	Accounts             []referenceAccount           `json:"accounts"`
	Validators           []referenceValidator         `json:"validators"`
	Vaults               []referenceVault             `json:"vaults"`
	VaultMoves           []StrategyVaultMovement      `json:"vault_moves,omitempty"`
	VaultSafety          StrategyVaultSafetyConfig    `json:"vault_safety"`
	Contracts            []referenceContract          `json:"contracts,omitempty"`
	LiquidityLocks       []referenceLiquidityLocks    `json:"liquidity_locks,omitempty"`
	TotalLiquidity       string                       `json:"total_liquidity"`
	PendingFeePool       []referenceNamedAmount       `json:"pending_fee_pool,omitempty"`
	PoolLiquidity        []referenceNamedAmount       `json:"pool_liquidity,omitempty"`
	UnallocatedLiquidity string                       `json:"unallocated_liquidity"`
	LiquidityProviders   []referenceProvider          `json:"liquidity_providers,omitempty"`
	DynamicOracleSignals []referenceDynamicSignal     `json:"dynamic_oracle_signals,omitempty"`
	PoolPriceHistory     []referencePriceHistory      `json:"pool_price_history,omitempty"`
	OracleObservations   []OracleObservation          `json:"oracle_observations,omitempty"`
	OraclePublishers     []referencePublisher         `json:"oracle_publishers,omitempty"`
	OracleNonces         []referenceHeight            `json:"oracle_nonces,omitempty"`
	PairRiskPolicies     []PairRiskPolicy             `json:"pair_risk_policies,omitempty"`
	CongestionProfile    []CongestionBucket           `json:"congestion_profile,omitempty"`
	EconomicPolicy       EconomicPolicy               `json:"economic_policy"`
	EconomicBalances     []referenceNamedAmount       `json:"economic_balances,omitempty"`
	Governance           *GovernanceState             `json:"governance,omitempty"`
	ProtocolPauses       []referenceBool              `json:"protocol_pauses,omitempty"`
	TotalBurned          string                       `json:"total_burned"`
	ArbPolicy            ProtocolArbPolicy            `json:"arb_policy"`
	ArbAuctions          []*ArbAuction                `json:"arb_auctions,omitempty"`
	ArbKeeperBonds       []referenceNamedAmount       `json:"arb_keeper_bonds,omitempty"`
	ArbKeeperUnbondAt    []referenceHeight            `json:"arb_keeper_unbond_at,omitempty"`
	SlashingCases        []*SlashingCase              `json:"slashing_cases,omitempty"`
	BridgeSecurity       *BridgeSecurityState         `json:"bridge_security,omitempty"`
	BridgeRequests       []*BridgeRequest             `json:"bridge_requests,omitempty"`
	BridgeTokens         []referenceBridgeToken       `json:"bridge_tokens,omitempty"`
	BusinessAgreements   []*LiquidityServiceAgreement `json:"business_agreements,omitempty"`
	TreasuryDeployments  []TreasuryDeployment         `json:"treasury_deployments,omitempty"`
	ConsensusPolicy      referenceConsensusPolicy     `json:"consensus_policy"`
	ProtocolRevenue      []ProtocolRevenueEntry       `json:"protocol_revenue,omitempty"`
	RevenueCheckpoints   []referenceNamedAmount       `json:"revenue_checkpoints,omitempty"`
	RevenueAssets        []referenceNamedAmount       `json:"revenue_assets,omitempty"`
	CumulativeEmission   string                       `json:"cumulative_emission"`
}

// ComputeReferenceStateRootAt is a lightweight reference-client root
// implementation. It independently snapshots, sorts and serializes every
// consensus-visible field. Release tests compare it with the production path.
func (bc *Blockchain_struct) ComputeReferenceStateRootAt(height uint64) string {
	if bc == nil {
		return ""
	}
	s := referenceCanonicalState{Version: CurrentStateVersion, Height: height, BaseFee: bc.BaseFee, MinStake: bc.MinStake, MinLiquidityStake: referenceAmount(bc.MinLiquidityStake), FixedBlockReward: bc.FixedBlockReward, GasRewardMultiplier: bc.GasRewardMultiplier, SlashingPool: bc.SlashingPool, VaultSafety: bc.StrategyVaultSafety, TotalLiquidity: referenceAmount(bc.TotalLiquidity), UnallocatedLiquidity: referenceAmount(bc.UnallocatedLiquidity), EconomicPolicy: bc.EconomicPolicy, Governance: bc.Governance, TotalBurned: referenceAmount(bc.TotalBurned), ArbPolicy: bc.ArbPolicy, BridgeSecurity: bc.BridgeSecurity, CumulativeEmission: referenceAmount(bc.CumulativeEmission), ProtocolRevenue: append([]ProtocolRevenueEntry(nil), bc.ProtocolRevenue...)}
	appendAmounts := func(dst *[]referenceNamedAmount, values map[string]*big.Int, normalize bool) {
		for name, amount := range values {
			if normalize {
				name = strings.ToLower(strings.TrimSpace(name))
			}
			*dst = append(*dst, referenceNamedAmount{Name: name, Amount: referenceAmount(amount)})
		}
		sort.Slice(*dst, func(i, j int) bool { return (*dst)[i].Name < (*dst)[j].Name })
	}
	appendAmounts(&s.PendingFeePool, bc.PendingFeePool, true)
	appendAmounts(&s.PoolLiquidity, bc.PoolLiquidity, true)
	for address, records := range bc.LiquidityLocks {
		s.LiquidityLocks = append(s.LiquidityLocks, referenceLiquidityLocks{Address: strings.ToLower(strings.TrimSpace(address)), Records: append([]LockRecord(nil), records...)})
	}
	sort.Slice(s.LiquidityLocks, func(i, j int) bool { return s.LiquidityLocks[i].Address < s.LiquidityLocks[j].Address })
	s.VaultMoves = append([]StrategyVaultMovement(nil), bc.StrategyVaultMoves...)
	sort.Slice(s.VaultMoves, func(i, j int) bool { return s.VaultMoves[i].ID < s.VaultMoves[j].ID })
	for key, signal := range bc.DynamicLiquidityOracleSignals {
		s.DynamicOracleSignals = append(s.DynamicOracleSignals, referenceDynamicSignal{Key: strings.ToLower(strings.TrimSpace(key)), Signal: signal})
	}
	sort.Slice(s.DynamicOracleSignals, func(i, j int) bool { return s.DynamicOracleSignals[i].Key < s.DynamicOracleSignals[j].Key })
	for pair, observations := range bc.PoolPriceHistory {
		rows := append([]PoolPriceObservation(nil), observations...)
		sort.Slice(rows, func(i, j int) bool {
			if rows[i].Timestamp == rows[j].Timestamp {
				return rows[i].Price < rows[j].Price
			}
			return rows[i].Timestamp < rows[j].Timestamp
		})
		s.PoolPriceHistory = append(s.PoolPriceHistory, referencePriceHistory{Pair: strings.ToLower(strings.TrimSpace(pair)), Observations: rows})
	}
	sort.Slice(s.PoolPriceHistory, func(i, j int) bool { return s.PoolPriceHistory[i].Pair < s.PoolPriceHistory[j].Pair })
	for _, request := range bc.BridgeRequests {
		if request != nil {
			s.BridgeRequests = append(s.BridgeRequests, request)
		}
	}
	sort.Slice(s.BridgeRequests, func(i, j int) bool { return s.BridgeRequests[i].ID < s.BridgeRequests[j].ID })
	for key, token := range bc.BridgeTokenMap {
		if token != nil {
			s.BridgeTokens = append(s.BridgeTokens, referenceBridgeToken{Key: strings.ToLower(strings.TrimSpace(key)), Token: *token})
		}
	}
	sort.Slice(s.BridgeTokens, func(i, j int) bool { return s.BridgeTokens[i].Key < s.BridgeTokens[j].Key })
	for _, agreement := range bc.BusinessAgreements {
		if agreement != nil {
			s.BusinessAgreements = append(s.BusinessAgreements, agreement)
		}
	}
	sort.Slice(s.BusinessAgreements, func(i, j int) bool { return s.BusinessAgreements[i].ID < s.BusinessAgreements[j].ID })
	s.TreasuryDeployments = append([]TreasuryDeployment(nil), bc.TreasuryDeployments...)
	sort.Slice(s.TreasuryDeployments, func(i, j int) bool { return s.TreasuryDeployments[i].ID < s.TreasuryDeployments[j].ID })
	for _, auction := range bc.ArbAuctions {
		if auction != nil {
			s.ArbAuctions = append(s.ArbAuctions, auction)
		}
	}
	sort.Slice(s.ArbAuctions, func(i, j int) bool { return s.ArbAuctions[i].ID < s.ArbAuctions[j].ID })
	for keeper, bond := range bc.ArbKeeperBonds {
		s.ArbKeeperBonds = append(s.ArbKeeperBonds, referenceNamedAmount{Name: strings.ToLower(keeper), Amount: referenceAmount(bond)})
	}
	sort.Slice(s.ArbKeeperBonds, func(i, j int) bool { return s.ArbKeeperBonds[i].Name < s.ArbKeeperBonds[j].Name })
	for keeper, unlock := range bc.ArbKeeperUnbondAt {
		s.ArbKeeperUnbondAt = append(s.ArbKeeperUnbondAt, referenceHeight{Key: strings.ToLower(keeper), Height: unlock})
	}
	sort.Slice(s.ArbKeeperUnbondAt, func(i, j int) bool { return s.ArbKeeperUnbondAt[i].Key < s.ArbKeeperUnbondAt[j].Key })
	for _, slashingCase := range bc.SlashingCases {
		if slashingCase != nil {
			s.SlashingCases = append(s.SlashingCases, slashingCase)
		}
	}
	sort.Slice(s.SlashingCases, func(i, j int) bool { return s.SlashingCases[i].ID < s.SlashingCases[j].ID })
	if bc.ConsensusV2 != nil {
		s.ConsensusPolicy = referenceConsensusPolicy{bc.ConsensusV2.EpochLength, bc.ConsensusV2.MaxLiquidityCreditBPS, bc.ConsensusV2.RoundTimeoutSeconds}
	}
	appendAmounts(&s.EconomicBalances, bc.EconomicBalances, false)
	appendAmounts(&s.RevenueCheckpoints, bc.RevenueCheckpoints, false)
	appendAmounts(&s.RevenueAssets, bc.CapturedRevenueAssets, false)
	for key, value := range bc.ProtocolPauses {
		s.ProtocolPauses = append(s.ProtocolPauses, referenceBool{key, value})
	}
	sort.Slice(s.ProtocolPauses, func(i, j int) bool { return s.ProtocolPauses[i].Key < s.ProtocolPauses[j].Key })
	bc.AccountsMu.RLock()
	for address, balance := range bc.Accounts {
		s.Accounts = append(s.Accounts, referenceAccount{strings.ToLower(strings.TrimSpace(address)), referenceAmount(balance)})
	}
	bc.AccountsMu.RUnlock()
	sort.Slice(s.Accounts, func(i, j int) bool { return s.Accounts[i].Address < s.Accounts[j].Address })
	for _, validator := range bc.Validators {
		if validator != nil {
			s.Validators = append(s.Validators, referenceValidator{strings.ToLower(strings.TrimSpace(validator.Address)), validator.NativeBond, validator.LiquidityPower, validator.PenaltyScore, validator.JailedUntil.Unix()})
		}
	}
	sort.Slice(s.Validators, func(i, j int) bool { return s.Validators[i].Address < s.Validators[j].Address })
	for _, vault := range bc.StrategyVaults {
		if vault != nil {
			s.Vaults = append(s.Vaults, referenceVault{vault.ID, strings.ToLower(strings.TrimSpace(vault.Owner)), strings.ToLower(strings.TrimSpace(vault.CurrentPool)), strings.ToLower(strings.TrimSpace(vault.TokenA)), strings.ToLower(strings.TrimSpace(vault.TokenB)), referenceAmount(vault.AmountA), referenceAmount(vault.AmountB), referenceAmount(vault.Shares), vault.Status})
		}
	}
	sort.Slice(s.Vaults, func(i, j int) bool { return s.Vaults[i].ID < s.Vaults[j].ID })
	for _, provider := range bc.LiquidityProviders {
		if provider != nil {
			s.LiquidityProviders = append(s.LiquidityProviders, referenceProvider{Address: strings.ToLower(strings.TrimSpace(provider.Address)), Stake: referenceAmount(provider.StakeAmount), LiquidityPower: provider.LiquidityPower, LockTime: provider.LockTime, LockDays: provider.LockDays, PendingRewards: referenceAmount(provider.PendingRewards), TotalRewards: referenceAmount(provider.TotalRewards), UnstakeAmount: referenceAmount(provider.UnstakeAmount), ReleasedSoFar: referenceAmount(provider.ReleasedSoFar), IsUnstaking: provider.IsUnstaking, UnstakeStartTime: provider.UnstakeStartTime})
		}
	}
	sort.Slice(s.LiquidityProviders, func(i, j int) bool { return s.LiquidityProviders[i].Address < s.LiquidityProviders[j].Address })
	for _, bySource := range bc.OracleObservations {
		for _, observation := range bySource {
			s.OracleObservations = append(s.OracleObservations, observation)
		}
	}
	sort.Slice(s.OracleObservations, func(i, j int) bool {
		if s.OracleObservations[i].Asset == s.OracleObservations[j].Asset {
			return s.OracleObservations[i].Source < s.OracleObservations[j].Source
		}
		return s.OracleObservations[i].Asset < s.OracleObservations[j].Asset
	})
	for source, address := range bc.OraclePublishers {
		s.OraclePublishers = append(s.OraclePublishers, referencePublisher{source, strings.ToLower(address), bc.OracleNonces[source]})
	}
	sort.Slice(s.OraclePublishers, func(i, j int) bool { return s.OraclePublishers[i].Source < s.OraclePublishers[j].Source })
	for source, nonce := range bc.OracleNonces {
		s.OracleNonces = append(s.OracleNonces, referenceHeight{Key: strings.ToLower(strings.TrimSpace(source)), Height: nonce})
	}
	sort.Slice(s.OracleNonces, func(i, j int) bool { return s.OracleNonces[i].Key < s.OracleNonces[j].Key })
	for _, policy := range bc.PairRiskPolicies {
		s.PairRiskPolicies = append(s.PairRiskPolicies, policy)
	}
	sort.Slice(s.PairRiskPolicies, func(i, j int) bool { return s.PairRiskPolicies[i].PairAddress < s.PairRiskPolicies[j].PairAddress })
	for _, bucket := range bc.CongestionProfile {
		s.CongestionProfile = append(s.CongestionProfile, bucket)
	}
	sort.Slice(s.CongestionProfile, func(i, j int) bool { return s.CongestionProfile[i].HourUTC < s.CongestionProfile[j].HourUTC })
	if bc.ContractEngine != nil && bc.ContractEngine.DB != nil {
		for _, address := range bc.ContractEngine.DB.ListContractAddresses() {
			values, err := bc.ContractEngine.DB.LoadAllStorage(address)
			if err != nil {
				return ""
			}
			contract := referenceContract{Address: strings.ToLower(strings.TrimSpace(address))}
			for key, value := range values {
				contract.Storage = append(contract.Storage, referenceKeyValue{key, value})
			}
			sort.Slice(contract.Storage, func(i, j int) bool { return contract.Storage[i].Key < contract.Storage[j].Key })
			s.Contracts = append(s.Contracts, contract)
		}
		sort.Slice(s.Contracts, func(i, j int) bool { return s.Contracts[i].Address < s.Contracts[j].Address })
	}
	raw, err := json.Marshal(s)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return "0x" + hex.EncodeToString(sum[:])
}
