package blockchaincomponent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	constantset "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/ConstantSet"
)

const (
	GasLimitAdjustmentFactor = 1024
	MinGasLimit              = 21000000  // minimum 1000 simple transfers per block
	MaxGasLimit              = 500000000 // max = MaxBlockGas
)

type Block struct {
	BlockNumber  uint64         `json:"block_number"`
	PreviousHash string         `json:"previous_hash"`
	CurrentHash  string         `json:"current_hash"`
	TimeStamp    uint64         `json:"timestamp"`
	Transactions []*Transaction `json:"transactions"`

	BaseFee         uint64               `json:"base_fee"`
	GasUsed         uint64               `json:"gas_used"`
	GasLimit        uint64               `json:"gas_limit"`
	RewardBreakdown BlockRewardBreakdown `json:"reward_breakdown,omitempty"`
	ProtocolVersion uint32               `json:"protocol_version,omitempty"`
	StateRoot       string               `json:"state_root,omitempty"`
	ParentStateRoot string               `json:"parent_state_root,omitempty"`
	ConsensusRound  uint32               `json:"consensus_round,omitempty"`
	QuorumHash      string               `json:"quorum_hash,omitempty"`
	ProposerProof   *ProposerCertificate `json:"proposer_proof,omitempty"`
	NextVRFProof    *ConsensusVRFProof   `json:"next_vrf_proof,omitempty"`
}

func NewBlock(blockNumber uint64, prevHash string) Block {
	newBlock := new(Block)
	newBlock.BlockNumber = blockNumber + 1
	newBlock.TimeStamp = uint64(time.Now().Unix())
	newBlock.PreviousHash = prevHash
	newBlock.Transactions = []*Transaction{}
	newBlock.GasLimit = uint64(constantset.MaxBlockGas)
	newBlock.BaseFee = 0
	newBlock.ProtocolVersion = CurrentProtocolVersion
	newBlock.RewardBreakdown.ValidatorReward = AmountString(new(big.Int).Mul(big.NewInt(200), big.NewInt(1e8)))
	newBlock.RewardBreakdown.ParticipantRewards = make(map[string]string)
	newBlock.RewardBreakdown.ParticipantRewardAddresses = make(map[string]string)
	newBlock.RewardBreakdown.LiquidityRewards = make(map[string]string)
	return *newBlock
}
func (bc *Blockchain_struct) CalculateNextGasLimit() uint64 {
	if len(bc.Blocks) == 0 {
		return MaxGasLimit
	}

	parent := bc.Blocks[len(bc.Blocks)-1]

	// Adjust based on parent gas used
	var newLimit uint64
	if parent.GasUsed > parent.GasLimit*3/4 {
		// Increase if block was mostly full
		newLimit = parent.GasLimit + parent.GasLimit/GasLimitAdjustmentFactor
	} else if parent.GasUsed < parent.GasLimit/2 {
		// Decrease if block was mostly empty
		newLimit = parent.GasLimit - parent.GasLimit/GasLimitAdjustmentFactor
	} else {
		// Keep the same
		newLimit = parent.GasLimit
	}

	// Apply bounds
	if newLimit < MinGasLimit {
		return MinGasLimit
	}
	if newLimit > MaxGasLimit {
		return MaxGasLimit
	}

	return newLimit
}

type VerifiedTx struct {
	Tx      *Transaction
	GasUsed uint64
	Fee     uint64
	Valid   bool
	Err     error
}

const TxWorkers = 10

// Worker uses ONLY in-memory accounts for speed.

func (bc *Blockchain_struct) verifyTxWorker(
	tasks <-chan *Transaction,
	out chan<- VerifiedTx,
	baseFee uint64,
) {
	for tx := range tasks {
		gasUnits := tx.CalculateGasCost()
		if gasUnits == 0 {
			out <- VerifiedTx{Tx: tx, Valid: false, Err: fmt.Errorf("gas cost is zero")}
			continue
		}

		minRequired := tx.PriorityFee + baseFee
		if tx.GasPrice < minRequired {
			out <- VerifiedTx{Tx: tx, Valid: false, Err: fmt.Errorf("gas_price < baseFee+tip (%d < %d)", tx.GasPrice, minRequired)}
			continue
		}

		if !bc.VerifyTransaction(tx) {
			err := fmt.Errorf("transaction verification failed")
			if strings.TrimSpace(tx.FailureReason) != "" {
				err = fmt.Errorf("%s", tx.FailureReason)
			}
			out <- VerifiedTx{Tx: tx, Valid: false, Err: err}
			continue
		}

		if tx.IsSystem {
			out <- VerifiedTx{
				Tx:      tx,
				GasUsed: gasUnits,
				Fee:     0,
				Valid:   true,
			}
			continue
		}

		// Read sender balance from in-memory map.
		senderBal, _ := bc.getAccountBalance(tx.From)
		if senderBal == nil {
			senderBal = big.NewInt(0)
		}

		feeTokens := gasUnits * tx.GasPrice
		totalCost := new(big.Int).Add(CopyAmount(tx.Value), NewAmountFromUint64(feeTokens))

		if senderBal.Cmp(totalCost) < 0 {
			out <- VerifiedTx{Tx: tx, Valid: false, Err: fmt.Errorf("insufficient funds (have %s need %s)", AmountString(senderBal), AmountString(totalCost))}
			continue
		}

		out <- VerifiedTx{
			Tx:      tx,
			GasUsed: gasUnits,
			Fee:     feeTokens,
			Valid:   true,
		}
	}
}

func cloneTransactionPool(pool []*Transaction) []*Transaction {
	out := make([]*Transaction, 0, len(pool))
	for _, tx := range pool {
		if tx == nil {
			continue
		}
		cp := *tx
		cp.Value = CopyAmount(tx.Value)
		cp.Data = append([]byte(nil), tx.Data...)
		cp.Sig = append([]byte(nil), tx.Sig...)
		cp.Args = append([]string(nil), tx.Args...)
		cp.ExtraData = append([]byte(nil), tx.ExtraData...)
		out = append(out, &cp)
	}
	return out
}

// MineNewBlock builds against an isolated copy-on-write state. The proposer
// therefore never exposes speculative account/contract mutations while a
// signed QC is still pending.
func (bc *Blockchain_struct) MineNewBlock() *Block {
	started := time.Now()
	shadow, overlay, err := bc.cloneForBlockReplay()
	if err != nil {
		log.Printf("MineNewBlock: failed to isolate candidate state: %v", err)
		return nil
	}
	shadow.Transaction_pool = cloneTransactionPool(bc.Transaction_pool)
	candidate := shadow.mineNewBlockCandidate(true)
	if candidate == nil {
		return nil
	}
	referenceRoot := shadow.ComputeReferenceStateRootAt(candidate.BlockNumber)
	if referenceRoot == "" || referenceRoot != candidate.StateRoot {
		log.Printf("MineNewBlock: independent reference root mismatch: production=%s reference=%s", candidate.StateRoot, referenceRoot)
		return nil
	}
	// The elected proposer commits a standards-based VRF output for the next
	// height inside this block. It is generated only after state execution and
	// before hashing, so every receiver verifies the same canonical commitment.
	if err := bc.AttachNextProposerVRF(candidate); err != nil {
		if !bc.ChainSpec.AllowLegacyFinality {
			log.Printf("MineNewBlock: strict finality requires next-height ECVRF proof: %v", err)
			return nil
		}
		log.Printf("MineNewBlock: optional next-height ECVRF proof unavailable: %v", err)
	} else {
		candidate.CurrentHash = CalculateHash(candidate)
	}
	transition := captureReplayTransition(candidate.CurrentHash, candidate.StateRoot, shadow, overlay)
	transition.ReferenceStateRoot = referenceRoot
	bc.stageReplayTransition(transition)
	bc.AddBlockVote(candidate.CurrentHash, candidate.RewardBreakdown.Validator)
	bc.AddPendingBlock(candidate)
	if bc.Network != nil {
		if qc, voteErr := bc.CastLocalConsensusStep(candidate, StepPrevote); voteErr != nil {
			log.Printf("Signed BFT vote unavailable, using configured bootstrap finality: %v", voteErr)
		} else if qc != nil {
			log.Printf("Signed BFT quorum certificate ready: %s", shortHash(qc.Hash))
		}
	}
	finalized := bc.TryFinalizePending(candidate.CurrentHash, 0.67)
	bc.LastBlockMiningTime = time.Since(started)
	if !finalized {
		log.Printf("⏳ Block #%d proposed without speculative state commit | tx=%d | gas=%d", candidate.BlockNumber, len(candidate.Transactions)-1, candidate.GasUsed)
		return nil
	}
	log.Printf("⛏ Merged Block #%d | tx=%d | time=%s | gas=%d", candidate.BlockNumber, len(candidate.Transactions)-1, bc.LastBlockMiningTime, candidate.GasUsed)
	return candidate
}

// mineNewBlockCandidate executes only inside the isolated shadow when
// candidateOnly is true. The legacy completion branch is retained for focused
// tests, but production calls always use the wrapper above.
func (bc *Blockchain_struct) mineNewBlockCandidate(candidateOnly bool) *Block {
	start := time.Now()

	bc.EnsureRuntimeState()
	if !bc.EnsureMineableTip(1024) {
		log.Printf("MineNewBlock: refusing to mine without a valid durable tip")
		return nil
	}
	bc.PrunePendingBlocksAtOrBelowTip()
	bc.PruneExpiredPendingBlocks(2 * time.Minute)

	if len(bc.Blocks) == 0 {
		return nil
	}

	lastBlock := bc.Blocks[len(bc.Blocks)-1]
	if lastBlock == nil || strings.TrimSpace(lastBlock.CurrentHash) == "" {
		log.Printf("MineNewBlock: refusing to mine on invalid in-memory tip")
		return nil
	}
	baseFee := bc.CalculateBaseFee()

	newBlock := NewBlock(lastBlock.BlockNumber, lastBlock.CurrentHash)
	newBlock.ConsensusRound = bc.CurrentConsensusRound(newBlock.BlockNumber)
	newBlock.GasLimit = bc.CalculateNextGasLimit()
	newBlock.BaseFee = baseFee
	newBlock.ParentStateRoot = bc.ComputeDeterministicStateRootAt(lastBlock.BlockNumber)

	validator, err := bc.SelectBlockProposer(newBlock.BlockNumber, newBlock.ConsensusRound)
	if err != nil {
		log.Printf("Validator selection error: %v", err)
		return nil
	}
	newBlock.ProposerProof, err = bc.BuildProposerCertificate(newBlock.BlockNumber, newBlock.ConsensusRound)
	if err != nil {
		log.Printf("Proposer proof error: %v", err)
		return nil
	}
	for _, registered := range bc.Validators {
		if registered != nil && strings.EqualFold(registered.Address, validator.Address) {
			registered.BlocksProposed++
			validator.BlocksProposed = registered.BlocksProposed
			break
		}
	}

	txPool := bc.Transaction_pool

	// Sort by gas price descending (highest fee first) for block inclusion
	sort.Slice(txPool, func(i, j int) bool {
		return txPool[i].GasPrice > txPool[j].GasPrice
	})

	taskChan := make(chan *Transaction, len(txPool))
	resultChan := make(chan VerifiedTx, len(txPool))

	workers := TxWorkers
	if workers > runtime.NumCPU() {
		workers = runtime.NumCPU()
	}

	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func() {
			bc.verifyTxWorker(taskChan, resultChan, baseFee)
			wg.Done()
		}()
	}

	for _, tx := range txPool {
		taskChan <- tx
	}
	close(taskChan)

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	var totalGasUsed uint64
	var totalGasCost uint64

	finalTxs := make([]*Transaction, 0, len(txPool))
	failedTxHashes := make(map[string]struct{})

	markFailed := func(tx *Transaction, reason string) {
		if tx == nil {
			return
		}
		tx.Status = constantset.StatusFailed
		if strings.TrimSpace(reason) == "" {
			reason = "transaction failed during block execution"
		}
		tx.FailureReason = reason
		if tx.TxHash == "" {
			tx.TxHash = CalculateTransactionHash(*tx)
		}
		failedTxHashes[tx.TxHash] = struct{}{}
		bc.RecordRecentTx(tx)
	}

	for res := range resultChan {

		// FAST-PATH: FORCE SUCCESS FOR SYSTEM/LP TX (but NOT contract calls)
		isSystem := res.Tx != nil && (res.Tx.IsSystem ||
			res.Tx.Type == "stake" ||
			res.Tx.Type == "unstake" ||
			res.Tx.Type == "lp_reward")
		if isSystem && res.Tx.Type != "contract_call" && res.Tx.Type != "contract_create" {
			res.Tx.Status = constantset.StatusSuccess
			finalTxs = append(finalTxs, res.Tx)
			bc.RecordRecentTx(res.Tx)
			continue
		}

		if !res.Valid || res.Tx == nil {
			reason := "transaction verification failed"
			if res.Err != nil {
				reason = res.Err.Error()
			}
			markFailed(res.Tx, reason)
			continue
		}
		if totalGasUsed+res.GasUsed > newBlock.GasLimit {
			markFailed(res.Tx, "block gas limit exceeded")
			continue
		}

		if res.Tx.IsContract {
			if res.Tx.Type == "contract_call" {
				_, err := bc.ContractEngine.Pipeline.ExecuteContractTxAt(res.Tx, int64(newBlock.TimeStamp))
				if err != nil {
					log.Printf("ContractTx FAILED fn=%s addr=%s err=%v", res.Tx.Function, res.Tx.To, err)
					markFailed(res.Tx, err.Error())
					continue
				}
			}
			// contract_create is a state registration already done at deploy time
		}
		if res.Tx.Type == "oracle_update" {
			if _, err := bc.ValidateOracleUpdateTransactionAt(res.Tx, int64(newBlock.TimeStamp)); err != nil {
				markFailed(res.Tx, err.Error())
				continue
			}
		}
		if res.Tx.Type == "governance_action" {
			if err := ValidateGovernanceTransaction(res.Tx); err != nil {
				markFailed(res.Tx, err.Error())
				continue
			}
		}

		totalTxCost := new(big.Int).Add(CopyAmount(res.Tx.Value), NewAmountFromUint64(res.Fee))

		senderBal, _ := bc.getAccountBalance(res.Tx.From)
		if senderBal == nil {
			senderBal = big.NewInt(0)
		}
		if senderBal.Cmp(totalTxCost) < 0 {
			markFailed(res.Tx, fmt.Sprintf("insufficient funds (have %s need %s)", AmountString(senderBal), AmountString(totalTxCost)))
			continue
		}
		if res.Tx.Type == "oracle_update" {
			if err := bc.ApplyOracleUpdateTransactionAt(res.Tx, int64(newBlock.TimeStamp)); err != nil {
				markFailed(res.Tx, err.Error())
				continue
			}
		}
		if res.Tx.Type == "governance_action" {
			if err := bc.applyGovernanceTransactionAt(res.Tx, newBlock.BlockNumber); err != nil {
				markFailed(res.Tx, err.Error())
				continue
			}
		}

		_ = bc.subAccountBalance(res.Tx.From, totalTxCost)
		if !(res.Tx.IsContract && res.Tx.Type == "contract_call") {
			bc.addAccountBalance(res.Tx.To, CopyAmount(res.Tx.Value))
		}

		res.Tx.Status = constantset.StatusSuccess
		res.Tx.FailureReason = ""

		if res.Tx.Type == "bridge_lock" || res.Tx.Type == "bridge_lock_private" {
			toBSC := ""
			if len(res.Tx.Args) > 0 {
				toBSC = res.Tx.Args[0]
			}
			if res.Tx.Type == "bridge_lock_private" {
				bc.AddPrivateBridgeRequest(res.Tx, toBSC)
			} else {
				bc.AddBridgeRequest(res.Tx, toBSC)
			}
		}

		finalTxs = append(finalTxs, res.Tx)

		totalGasUsed += res.GasUsed
		totalGasCost += res.Fee

		bc.RecordRecentTx(res.Tx)
	}

	if len(failedTxHashes) > 0 {
		filteredPool := make([]*Transaction, 0, len(bc.Transaction_pool))
		for _, tx := range bc.Transaction_pool {
			if tx == nil {
				continue
			}
			if strings.EqualFold(tx.Status, constantset.StatusFailed) {
				continue
			}
			if _, failed := failedTxHashes[tx.TxHash]; failed {
				continue
			}
			filteredPool = append(filteredPool, tx)
		}
		bc.Transaction_pool = filteredPool
	}

	newBlock.Transactions = finalTxs
	newBlock.GasUsed = totalGasUsed

	breakdown := bc.CalculateBlockRewards(
		validator.Address,
		finalTxs,
		totalGasCost,
		newBlock.BlockNumber,
	)
	newBlock.RewardBreakdown = breakdown
	// newBlock.RewardBreakdown.ValidatorReward=CalculateRewardForValidator(totalGasCost)[validator.Address]
	// newBlock.RewardBreakdown.ParticipantRewards=make(map[string]uint64)
	// newBlock.RewardBreakdown.LiquidityRewards=make(map[string]uint64)
	// Legacy equalization is retired. Physical strategy-vault rebalancing is
	// the only liquidity-movement path; routing weights never fabricate reserves.

	rewardTx := &Transaction{
		From:       "0x0000000000000000000000000000000000000000",
		To:         validator.Address,
		Value:      NewAmountFromStringOrZero(breakdown.ValidatorReward),
		GasPrice:   0,
		Timestamp:  newBlock.TimeStamp,
		Status:     constantset.StatusSuccess,
		ExtraData:  []byte("block_reward"),
		IsContract: false,
		Type:       "reward",
	}
	rewardTx.TxHash = CalculateTransactionHash(*rewardTx)

	newBlock.Transactions = append(newBlock.Transactions, rewardTx)
	bc.RecordRecentTx(rewardTx)
	// Epoch transitions are consensus state. Apply them before committing the
	// block root, using the block timestamp so every node replays the same
	// routing, congestion and arbitrage decision.
	if bc.DLEngine != nil {
		bc.DLEngine.RunEpochAt(bc, newBlock.BlockNumber, int64(newBlock.TimeStamp))
	}
	if err := bc.ReconcileDEXProtocolFees(newBlock.BlockNumber, int64(newBlock.TimeStamp)); err != nil {
		log.Printf("Protocol fee reconciliation failed: %v", err)
		return nil
	}
	// Commit the root only after transaction execution, reward accounting and
	// scheduled protocol accounting have all completed.
	newBlock.StateRoot = bc.ComputeDeterministicStateRootAt(newBlock.BlockNumber)

	newBlock.CurrentHash = CalculateHash(&newBlock)
	if candidateOnly {
		return &newBlock
	}

	// Proposer self-votes, then route through pending → quorum → finalize
	bc.AddBlockVote(newBlock.CurrentHash, validator.Address)
	bc.AddPendingBlock(&newBlock)
	// Prefer signed two-phase BFT finality whenever the validator identity key
	// is configured. Test/dev chains without a key retain legacy bootstrap
	// voting, and readiness reports that downgrade explicitly.
	if bc.Network != nil {
		if qc, err := bc.CastLocalConsensusStep(&newBlock, StepPrevote); err != nil {
			log.Printf("Signed BFT vote unavailable, using bootstrap finality: %v", err)
		} else if qc != nil {
			log.Printf("Signed BFT quorum certificate ready: %s", shortHash(qc.Hash))
		}
	}
	// TryFinalizePending handles txpool cleanup and DB save.
	// Bootstrap nodes finalize against the active voting set; remote validators
	// join that set only after their P2P connection is healthy.
	finalized := bc.TryFinalizePending(newBlock.CurrentHash, 0.67)

	bc.LastBlockMiningTime = time.Since(start)

	if finalized {
		log.Printf("⛏ Merged Block #%d | tx=%d  | time=%d | gas=%d | reward=%+v",
			newBlock.BlockNumber,
			len(finalTxs),
			bc.LastBlockMiningTime,
			newBlock.GasUsed,
			newBlock.RewardBreakdown,
		)
	} else {
		log.Printf("⏳ Block #%d mined but waiting for validator votes | tx=%d | gas=%d",
			newBlock.BlockNumber,
			len(finalTxs),
			newBlock.GasUsed,
		)
		return nil
	}
	log.Printf("Winner %s | validator_participants=%d | participant_txs=%d",
		newBlock.RewardBreakdown.Validator,
		len(newBlock.RewardBreakdown.ValidatorPartRewards),
		len(newBlock.RewardBreakdown.ParticipantRewards),
	)

	return &newBlock
}

func CalculateHash(newBlock *Block) string {

	blockCopy := *newBlock
	blockCopy.RewardBreakdown = BlockRewardBreakdown{}
	data, _ := json.Marshal(blockCopy)
	hash := sha256.Sum256(data)
	HexRePresent := hex.EncodeToString(hash[:32])
	formatedToHex := constantset.BlockHexPrefix + HexRePresent

	return formatedToHex

}

func ToJsonBlock(genesisBlock Block) string {
	nBlock := genesisBlock
	block, err := json.Marshal(nBlock)
	if err != nil {
		log.Println("error")
	}
	return string(block)
}
