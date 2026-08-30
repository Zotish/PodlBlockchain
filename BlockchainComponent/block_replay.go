package blockchaincomponent

import (
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
	"strings"

	constantset "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/ConstantSet"
)

// BlockReplayTransition is a fully isolated post-state produced by executing an
// incoming block against the local finalized pre-state.
type BlockReplayTransition struct {
	BlockHash            string
	PostStateRoot        string
	ReferenceStateRoot   string
	BaseFee              uint64
	MinStake             float64
	SlashingPool         float64
	FixedBlockReward     uint64
	GasRewardMultiplier  uint64
	MinLiquidityStake    *big.Int
	Accounts             map[string]*big.Int
	Validators           []*Validator
	LiquidityProviders   map[string]*LiquidityProvider
	LiquidityLocks       map[string][]LockRecord
	TotalLiquidity       *big.Int
	PendingFeePool       map[string]*big.Int
	PoolLiquidity        map[string]*big.Int
	UnallocatedLiquidity *big.Int
	StrategyVaults       map[string]*StrategyVaultPosition
	StrategyVaultMoves   []StrategyVaultMovement
	StrategyVaultSafety  StrategyVaultSafetyConfig
	DynamicOracleSignals map[string]DynamicLiquidityOracleSignal
	PoolPriceHistory     map[string][]PoolPriceObservation
	OracleObservations   map[string]map[string]OracleObservation
	OracleNonces         map[string]uint64
	Governance           *GovernanceState
	EconomicPolicy       EconomicPolicy
	ProtocolPauses       map[string]bool
	OraclePublishers     map[string]string
	PairRiskPolicies     map[string]PairRiskPolicy
	CongestionProfile    map[int]CongestionBucket
	EconomicBalances     map[string]*big.Int
	TotalBurned          *big.Int
	ArbPolicy            ProtocolArbPolicy
	ArbAuctions          map[string]*ArbAuction
	ArbKeeperBonds       map[string]*big.Int
	ArbKeeperUnbondAt    map[string]uint64
	BridgeSecurity       *BridgeSecurityState
	BridgeRequests       map[string]*BridgeRequest
	BridgeTokenMap       map[string]*BridgeTokenInfo
	BusinessAgreements   map[string]*LiquidityServiceAgreement
	TreasuryDeployments  []TreasuryDeployment
	ConsensusPolicy      canonicalConsensusPolicy
	ProtocolRevenue      []ProtocolRevenueEntry
	RevenueCheckpoints   map[string]*big.Int
	RevenueAssets        map[string]*big.Int
	CumulativeEmission   *big.Int
	SlashingCases        map[string]*SlashingCase
	ContractOverlay      *ContractDB
	// RejectedTransactions are local proposal transactions that failed during
	// deterministic execution and therefore were intentionally excluded from
	// the finalized block. They are not consensus state, but the proposer must
	// evict them from its canonical mempool once the proposal finalizes.
	RejectedTransactions []*Transaction
}

func rejectedReplayTransactions(recent []*Transaction) []*Transaction {
	rejected := make([]*Transaction, 0)
	for _, tx := range recent {
		if tx == nil || !strings.EqualFold(tx.Status, constantset.StatusFailed) {
			continue
		}
		rejected = append(rejected, cloneTransactionPool([]*Transaction{tx})...)
	}
	return rejected
}

func (bc *Blockchain_struct) evictRejectedTransactions(transactions []*Transaction) {
	if bc == nil || len(transactions) == 0 {
		return
	}
	rejected := make(map[string]*Transaction, len(transactions))
	for _, tx := range transactions {
		if tx == nil || strings.TrimSpace(tx.TxHash) == "" {
			continue
		}
		rejected[strings.ToLower(tx.TxHash)] = tx
		bc.RecordRecentTx(tx)
	}
	remaining := make([]*Transaction, 0, len(bc.Transaction_pool))
	for _, tx := range bc.Transaction_pool {
		if tx == nil {
			continue
		}
		if _, failed := rejected[strings.ToLower(tx.TxHash)]; failed {
			continue
		}
		remaining = append(remaining, tx)
	}
	bc.Transaction_pool = remaining
}

func captureReplayTransition(blockHash, postStateRoot string, shadow *Blockchain_struct, overlay *ContractDB) *BlockReplayTransition {
	policy := canonicalConsensusPolicy{}
	if shadow.ConsensusV2 != nil {
		policy = canonicalConsensusPolicy{EpochLength: shadow.ConsensusV2.EpochLength, MaxLiquidityCreditBPS: shadow.ConsensusV2.MaxLiquidityCreditBPS, RoundTimeoutSeconds: shadow.ConsensusV2.RoundTimeoutSeconds}
	}
	return &BlockReplayTransition{
		BlockHash: blockHash, PostStateRoot: postStateRoot,
		BaseFee: shadow.BaseFee, MinStake: shadow.MinStake, SlashingPool: shadow.SlashingPool,
		FixedBlockReward: shadow.FixedBlockReward, GasRewardMultiplier: shadow.GasRewardMultiplier, MinLiquidityStake: shadow.MinLiquidityStake,
		Accounts: shadow.Accounts, Validators: shadow.Validators, LiquidityProviders: shadow.LiquidityProviders,
		LiquidityLocks: shadow.LiquidityLocks, TotalLiquidity: shadow.TotalLiquidity, PendingFeePool: shadow.PendingFeePool,
		PoolLiquidity: shadow.PoolLiquidity, UnallocatedLiquidity: shadow.UnallocatedLiquidity,
		StrategyVaults: shadow.StrategyVaults, StrategyVaultMoves: shadow.StrategyVaultMoves, StrategyVaultSafety: shadow.StrategyVaultSafety,
		DynamicOracleSignals: shadow.DynamicLiquidityOracleSignals, PoolPriceHistory: shadow.PoolPriceHistory,
		OracleObservations: shadow.OracleObservations, OracleNonces: shadow.OracleNonces,
		Governance: shadow.Governance, EconomicPolicy: shadow.EconomicPolicy, ProtocolPauses: shadow.ProtocolPauses,
		OraclePublishers: shadow.OraclePublishers, PairRiskPolicies: shadow.PairRiskPolicies, CongestionProfile: shadow.CongestionProfile,
		EconomicBalances: shadow.EconomicBalances, TotalBurned: shadow.TotalBurned, ArbPolicy: shadow.ArbPolicy,
		ArbAuctions: shadow.ArbAuctions, ArbKeeperBonds: shadow.ArbKeeperBonds, ArbKeeperUnbondAt: shadow.ArbKeeperUnbondAt,
		BridgeSecurity: shadow.BridgeSecurity, BridgeRequests: shadow.BridgeRequests, BridgeTokenMap: shadow.BridgeTokenMap,
		BusinessAgreements: shadow.BusinessAgreements, TreasuryDeployments: shadow.TreasuryDeployments,
		ConsensusPolicy: policy, ProtocolRevenue: shadow.ProtocolRevenue, RevenueCheckpoints: shadow.RevenueCheckpoints,
		RevenueAssets: shadow.CapturedRevenueAssets, CumulativeEmission: shadow.CumulativeEmission,
		SlashingCases: shadow.SlashingCases, ContractOverlay: overlay,
		RejectedTransactions: rejectedReplayTransactions(shadow.RecentTxs),
	}
}

func cloneLiquidityProvider(provider *LiquidityProvider) *LiquidityProvider {
	if provider == nil {
		return nil
	}
	out := *provider
	out.StakeAmount = CopyAmount(provider.StakeAmount)
	out.TotalRewards = CopyAmount(provider.TotalRewards)
	out.PendingRewards = CopyAmount(provider.PendingRewards)
	out.UnstakeAmount = CopyAmount(provider.UnstakeAmount)
	out.ReleasedSoFar = CopyAmount(provider.ReleasedSoFar)
	return &out
}

func (bc *Blockchain_struct) cloneForBlockReplay() (*Blockchain_struct, *ContractDB, error) {
	if bc == nil {
		return nil, nil, fmt.Errorf("nil blockchain")
	}
	// Only consensus/runtime state belongs in a speculative execution copy.
	// Blocks are persisted individually and the reward/recent-transaction
	// ledgers are explorer caches; serializing hundreds of thousands of those
	// rows before every block made production time increase with chain age.
	// Keep the recent block window needed by nonce/base-fee checks, while the
	// JSON boundary below still provides a complete deep copy of consensus
	// state without pointer aliasing.
	snapshot := *bc
	snapshot.Blocks = replayBlockWindow(bc.Blocks)
	snapshot.Transaction_pool = nil
	snapshot.RewardHistory = nil
	snapshot.RewardLedger = nil
	snapshot.RecentTxs = nil
	snapshot.RecentTxCounter = 0
	snapshot.BlockVotes = nil
	snapshot.PendingBlocks = nil
	snapshot.PendingBlockSeenAt = nil
	snapshot.LastBlockMiningTime = 0
	// A JSON round-trip is the canonical deep-copy boundary for consensus
	// state. It avoids copying live mutex values and prevents maps/pointers in
	// the speculative execution state from aliasing finalized state.
	rawState, err := json.Marshal(&snapshot)
	if err != nil {
		return nil, nil, fmt.Errorf("encode replay snapshot: %w", err)
	}
	var shadow Blockchain_struct
	if err := json.Unmarshal(rawState, &shadow); err != nil {
		return nil, nil, fmt.Errorf("decode replay snapshot: %w", err)
	}
	shadow.EnsureRuntimeState()
	shadow.Network = nil
	shadow.DLEngine = NewDynamicLiquidityEngine()
	if bc.DLEngine != nil {
		shadow.DLEngine.EpochBlocks = bc.DLEngine.EpochBlocks
		shadow.DLEngine.LowThreshold = bc.DLEngine.LowThreshold
		shadow.DLEngine.HighThreshold = bc.DLEngine.HighThreshold
	}
	shadow.PendingReplayTransitions = make(map[string]*BlockReplayTransition)
	shadow.Transaction_pool = nil
	shadow.RecentTxs = nil
	if bc.ContractEngine == nil || bc.ContractEngine.DB == nil || bc.ContractEngine.Registry == nil {
		return &shadow, nil, nil
	}
	overlay := NewOverlayContractDB(bc.ContractEngine.DB)
	registry := NewContractRegistry(overlay, bc.ContractEngine.EventDB)
	// Go plugins are process-global and plugin.Open refuses to reopen the same
	// package in a fresh cache. Reuse the canonical, concurrency-safe VM while
	// keeping all contract storage in the isolated overlay. Plugin instances are
	// already shared by digest across contract addresses and carry no state;
	// execution state lives exclusively in Context/ContractDB.
	if bc.ContractEngine.Registry.PluginVM != nil {
		registry.PluginVM = bc.ContractEngine.Registry.PluginVM
	}
	registry.Blockchain = &shadow
	shadow.ContractEngine = &LQDContractEngine{DB: overlay, EventDB: bc.ContractEngine.EventDB, Registry: registry, Pipeline: NewExecutionPipeline(registry)}
	return &shadow, overlay, nil
}

const replayRecentBlockWindow = 5

func replayBlockWindow(blocks []*Block) []*Block {
	if len(blocks) <= replayRecentBlockWindow {
		return append([]*Block(nil), blocks...)
	}
	return append([]*Block(nil), blocks[len(blocks)-replayRecentBlockWindow:]...)
}

func rewardBreakdownEqual(a, b BlockRewardBreakdown) bool {
	// JSON round-trip normalizes nil versus empty map representations before a
	// structural comparison and keeps the check independent of map order.
	var na, nb BlockRewardBreakdown
	ra, _ := json.Marshal(a)
	rb, _ := json.Marshal(b)
	_ = json.Unmarshal(ra, &na)
	_ = json.Unmarshal(rb, &nb)
	return reflect.DeepEqual(na, nb)
}

func (bc *Blockchain_struct) ReplayIncomingBlock(block *Block) (*BlockReplayTransition, error) {
	if bc == nil || block == nil || len(bc.Blocks) == 0 {
		return nil, fmt.Errorf("block and finalized parent are required")
	}
	parent := bc.Blocks[len(bc.Blocks)-1]
	localParentRoot := bc.ComputeDeterministicStateRootAt(parent.BlockNumber)
	if block.ParentStateRoot == "" || block.ParentStateRoot != localParentRoot {
		return nil, fmt.Errorf("parent state root mismatch")
	}
	shadow, overlay, err := bc.cloneForBlockReplay()
	if err != nil {
		return nil, err
	}
	var gasUsed, gasFees uint64
	finalTxs := make([]*Transaction, 0, len(block.Transactions))
	seenNonce := map[string]map[uint64]bool{}
	rewardTransactions := 0
	for _, tx := range block.Transactions {
		if tx == nil {
			return nil, fmt.Errorf("nil transaction")
		}
		if strings.EqualFold(tx.Type, "reward") {
			rewardTransactions++
			expectedReward := NewAmountFromStringOrZero(block.RewardBreakdown.ValidatorReward)
			if rewardTransactions != 1 || !strings.EqualFold(tx.To, block.RewardBreakdown.Validator) || CopyAmount(tx.Value).Cmp(expectedReward) != 0 || !strings.EqualFold(tx.From, "0x0000000000000000000000000000000000000000") {
				return nil, fmt.Errorf("invalid block reward transaction")
			}
			continue
		}
		finalTxs = append(finalTxs, tx)
		isSystem := tx.IsSystem || tx.Type == "stake" || tx.Type == "unstake" || tx.Type == "lp_reward"
		if isSystem && tx.Type != "contract_call" && tx.Type != "contract_create" {
			continue
		}
		address := strings.ToLower(tx.From)
		if seenNonce[address] == nil {
			seenNonce[address] = map[uint64]bool{}
		}
		if seenNonce[address][tx.Nonce] {
			return nil, fmt.Errorf("duplicate sender nonce in block")
		}
		seenNonce[address][tx.Nonce] = true
		units := tx.CalculateGasCost()
		if tx.GasPrice > 0 && units > ^uint64(0)/tx.GasPrice {
			return nil, fmt.Errorf("gas fee multiplication overflow")
		}
		fee := units * tx.GasPrice
		if ^uint64(0)-gasUsed < units || ^uint64(0)-gasFees < fee {
			return nil, fmt.Errorf("gas arithmetic overflow")
		}
		gasUsed += units
		gasFees += fee
		if tx.IsContract && tx.Type == "contract_call" {
			if shadow.ContractEngine == nil {
				return nil, fmt.Errorf("contract engine unavailable for replay")
			}
			if _, err := shadow.ContractEngine.Pipeline.ExecuteContractTxAt(tx, int64(block.TimeStamp)); err != nil {
				return nil, fmt.Errorf("contract replay failed for %s: %w", tx.TxHash, err)
			}
		}
		if tx.Type == "oracle_update" {
			if err := shadow.ApplyOracleUpdateTransactionAt(tx, int64(block.TimeStamp)); err != nil {
				return nil, fmt.Errorf("oracle replay failed: %w", err)
			}
		}
		if tx.Type == "governance_action" {
			if err := shadow.applyGovernanceTransactionAt(tx, block.BlockNumber); err != nil {
				return nil, fmt.Errorf("governance replay failed: %w", err)
			}
		}
		total := new(big.Int).Add(CopyAmount(tx.Value), NewAmountFromUint64(fee))
		balance, _ := shadow.getAccountBalance(tx.From)
		if balance == nil || balance.Cmp(total) < 0 || !shadow.subAccountBalance(tx.From, total) {
			return nil, fmt.Errorf("replay insufficient balance for %s", tx.From)
		}
		if !(tx.IsContract && tx.Type == "contract_call") {
			shadow.addAccountBalance(tx.To, CopyAmount(tx.Value))
		}
	}
	if rewardTransactions != 1 {
		return nil, fmt.Errorf("exactly one block reward transaction is required")
	}
	if gasUsed != block.GasUsed {
		return nil, fmt.Errorf("gas used mismatch: replay=%d block=%d", gasUsed, block.GasUsed)
	}
	expectedRewards := shadow.CalculateBlockRewards(block.RewardBreakdown.Validator, finalTxs, gasFees, block.BlockNumber)
	if !rewardBreakdownEqual(expectedRewards, block.RewardBreakdown) {
		return nil, fmt.Errorf("reward breakdown mismatch")
	}
	shadow.applyValidatorCleanUptimeRecovery(block.RewardBreakdown.Validator)
	shadow.DLEngine.RunEpochAt(shadow, block.BlockNumber, int64(block.TimeStamp))
	if err := shadow.ReconcileDEXProtocolFees(block.BlockNumber, int64(block.TimeStamp)); err != nil {
		return nil, err
	}
	postRoot := shadow.ComputeDeterministicStateRootAt(block.BlockNumber)
	if postRoot != block.StateRoot {
		return nil, fmt.Errorf("post-state root mismatch: replay=%s block=%s", postRoot, block.StateRoot)
	}
	referenceRoot := shadow.ComputeReferenceStateRootAt(block.BlockNumber)
	if referenceRoot == "" || referenceRoot != postRoot {
		return nil, fmt.Errorf("reference replay mismatch: production=%s reference=%s", postRoot, referenceRoot)
	}
	transition := captureReplayTransition(block.CurrentHash, postRoot, shadow, overlay)
	transition.ReferenceStateRoot = referenceRoot
	return transition, nil
}

func (bc *Blockchain_struct) stageReplayTransition(transition *BlockReplayTransition) {
	if bc == nil || transition == nil {
		return
	}
	bc.ReplayMu.Lock()
	defer bc.ReplayMu.Unlock()
	if bc.PendingReplayTransitions == nil {
		bc.PendingReplayTransitions = make(map[string]*BlockReplayTransition)
	}
	bc.PendingReplayTransitions[transition.BlockHash] = transition
}

func (bc *Blockchain_struct) applyReplayTransition(block *Block) error {
	if bc == nil || block == nil {
		return fmt.Errorf("nil replay target")
	}
	// Locally mined blocks have already applied their state transition.
	if bc.ComputeDeterministicStateRootAt(block.BlockNumber) == block.StateRoot {
		return nil
	}
	bc.ReplayMu.Lock()
	transition := bc.PendingReplayTransitions[block.CurrentHash]
	delete(bc.PendingReplayTransitions, block.CurrentHash)
	bc.ReplayMu.Unlock()
	if transition == nil || transition.PostStateRoot != block.StateRoot || transition.ReferenceStateRoot != transition.PostStateRoot {
		return fmt.Errorf("verified replay transition unavailable")
	}
	if transition.ContractOverlay != nil {
		if err := transition.ContractOverlay.FlushOverlay(); err != nil {
			return fmt.Errorf("contract replay commit failed: %w", err)
		}
	}
	bc.AccountsMu.Lock()
	bc.Accounts = transition.Accounts
	bc.AccountsMu.Unlock()
	bc.BaseFee, bc.MinStake, bc.SlashingPool = transition.BaseFee, transition.MinStake, transition.SlashingPool
	bc.FixedBlockReward, bc.GasRewardMultiplier = transition.FixedBlockReward, transition.GasRewardMultiplier
	bc.MinLiquidityStake = transition.MinLiquidityStake
	bc.Validators = transition.Validators
	bc.LiquidityProviders = transition.LiquidityProviders
	bc.LiquidityLocks, bc.TotalLiquidity = transition.LiquidityLocks, transition.TotalLiquidity
	bc.PendingFeePool, bc.PoolLiquidity, bc.UnallocatedLiquidity = transition.PendingFeePool, transition.PoolLiquidity, transition.UnallocatedLiquidity
	bc.StrategyVaults, bc.StrategyVaultMoves, bc.StrategyVaultSafety = transition.StrategyVaults, transition.StrategyVaultMoves, transition.StrategyVaultSafety
	bc.DynamicLiquidityOracleSignals, bc.PoolPriceHistory = transition.DynamicOracleSignals, transition.PoolPriceHistory
	bc.OracleObservations = transition.OracleObservations
	bc.OracleNonces = transition.OracleNonces
	bc.Governance, bc.EconomicPolicy, bc.ProtocolPauses = transition.Governance, transition.EconomicPolicy, transition.ProtocolPauses
	bc.OraclePublishers, bc.PairRiskPolicies, bc.ArbPolicy = transition.OraclePublishers, transition.PairRiskPolicies, transition.ArbPolicy
	bc.CongestionProfile, bc.EconomicBalances, bc.TotalBurned = transition.CongestionProfile, transition.EconomicBalances, transition.TotalBurned
	bc.ArbAuctions, bc.ArbKeeperBonds, bc.ArbKeeperUnbondAt = transition.ArbAuctions, transition.ArbKeeperBonds, transition.ArbKeeperUnbondAt
	bc.BridgeSecurity, bc.BridgeRequests, bc.BridgeTokenMap = transition.BridgeSecurity, transition.BridgeRequests, transition.BridgeTokenMap
	bc.BusinessAgreements, bc.TreasuryDeployments = transition.BusinessAgreements, transition.TreasuryDeployments
	bc.ProtocolRevenue, bc.RevenueCheckpoints, bc.CapturedRevenueAssets = transition.ProtocolRevenue, transition.RevenueCheckpoints, transition.RevenueAssets
	bc.CumulativeEmission = transition.CumulativeEmission
	bc.SlashingCases = transition.SlashingCases
	if bc.ConsensusV2 != nil {
		bc.ConsensusV2.EpochLength = transition.ConsensusPolicy.EpochLength
		bc.ConsensusV2.MaxLiquidityCreditBPS = transition.ConsensusPolicy.MaxLiquidityCreditBPS
		bc.ConsensusV2.RoundTimeoutSeconds = transition.ConsensusPolicy.RoundTimeoutSeconds
	}
	if bc.ComputeDeterministicStateRootAt(block.BlockNumber) != block.StateRoot {
		return fmt.Errorf("committed replay root mismatch")
	}
	bc.evictRejectedTransactions(transition.RejectedTransactions)
	return nil
}

func successfulBlockTx(tx *Transaction) bool {
	return tx != nil && (tx.Status == "" || strings.EqualFold(tx.Status, constantset.StatusSuccess) || strings.EqualFold(tx.Status, constantset.StatusPending))
}
