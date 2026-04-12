package blockchaincomponent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"math/big"
	"runtime"
	"sort"
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
}

func NewBlock(blockNumber uint64, prevHash string) Block {
	newBlock := new(Block)
	newBlock.BlockNumber = blockNumber + 1
	newBlock.TimeStamp = uint64(time.Now().Unix())
	newBlock.PreviousHash = prevHash
	newBlock.Transactions = []*Transaction{}
	newBlock.GasLimit = uint64(constantset.MaxBlockGas)
	newBlock.BaseFee = 0
	newBlock.RewardBreakdown.ValidatorReward = AmountString(new(big.Int).Mul(big.NewInt(200), big.NewInt(1e8)))
	newBlock.RewardBreakdown.ParticipantRewards = make(map[string]string)
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
			out <- VerifiedTx{Tx: tx, Valid: false}
			continue
		}

		minRequired := tx.PriorityFee + baseFee
		if tx.GasPrice < minRequired {
			out <- VerifiedTx{Tx: tx, Valid: false}
			continue
		}

		if !bc.VerifyTransaction(tx) {
			out <- VerifiedTx{Tx: tx, Valid: false}
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
			out <- VerifiedTx{Tx: tx, Valid: false}
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

// MineNewBlock() — Parallel Pipeline + Reward
func (bc *Blockchain_struct) MineNewBlock() *Block {
	start := time.Now()

	if len(bc.Blocks) == 0 {
		return nil
	}

	lastBlock := bc.Blocks[len(bc.Blocks)-1]
	baseFee := bc.CalculateBaseFee()

	newBlock := NewBlock(lastBlock.BlockNumber, lastBlock.CurrentHash)
	newBlock.GasLimit = bc.CalculateNextGasLimit()
	newBlock.BaseFee = baseFee

	validator, err := bc.SelectValidator()
	if err != nil {
		log.Printf("Validator selection error: %v", err)
		return nil
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
			if res.Tx != nil {
				res.Tx.Status = constantset.StatusFailed
			}
			continue
		}
		if totalGasUsed+res.GasUsed > newBlock.GasLimit {
			res.Tx.Status = constantset.StatusFailed
			continue
		}

		if res.Tx.IsContract {
			if res.Tx.Type == "contract_call" {
				_, err := bc.ContractEngine.Pipeline.ExecuteContractTx(
					res.Tx,
					newBlock.BlockNumber,
				)
				if err != nil {
					log.Printf("ContractTx FAILED fn=%s addr=%s err=%v", res.Tx.Function, res.Tx.To, err)
					res.Tx.Status = constantset.StatusFailed
					continue
				}
			}
			// contract_create is a state registration already done at deploy time
		}

		totalTxCost := new(big.Int).Add(CopyAmount(res.Tx.Value), NewAmountFromUint64(res.Fee))

		senderBal, _ := bc.getAccountBalance(res.Tx.From)
		if senderBal == nil {
			senderBal = big.NewInt(0)
		}
		if senderBal.Cmp(totalTxCost) < 0 {
			res.Tx.Status = constantset.StatusFailed
			continue
		}

		_ = bc.subAccountBalance(res.Tx.From, totalTxCost)
		if !(res.Tx.IsContract && res.Tx.Type == "contract_call") {
			bc.addAccountBalance(res.Tx.To, CopyAmount(res.Tx.Value))
		}

		res.Tx.Status = constantset.StatusSuccess

		if res.Tx.Type == "bridge_lock" {
			toBSC := ""
			if len(res.Tx.Args) > 0 {
				toBSC = res.Tx.Args[0]
			}
			bc.AddBridgeRequest(res.Tx, toBSC)
		}

		finalTxs = append(finalTxs, res.Tx)

		totalGasUsed += res.GasUsed
		totalGasCost += res.Fee

		bc.RecordRecentTx(res.Tx)
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
	bc.RebalancePoolsEqual()

	rewardTx := &Transaction{
		From:       "0x0000000000000000000000000000000000000000",
		To:         validator.Address,
		Value:      NewAmountFromStringOrZero(breakdown.ValidatorReward),
		GasPrice:   0,
		Timestamp:  uint64(time.Now().Unix()),
		Status:     constantset.StatusSuccess,
		ExtraData:  []byte("block_reward"),
		IsContract: false,
		Type:       "reward",
	}
	rewardTx.TxHash = CalculateTransactionHash(*rewardTx)

	newBlock.Transactions = append(newBlock.Transactions, rewardTx)
	bc.RecordRecentTx(rewardTx)

	newBlock.CurrentHash = CalculateHash(&newBlock)

	// Proposer self-votes, then route through pending → quorum → finalize
	bc.AddBlockVote(newBlock.CurrentHash, validator.Address)
	bc.AddPendingBlock(&newBlock)
	// TryFinalizePending handles txpool cleanup and DB save.
	// On a single-validator node this finalizes immediately (1/1 votes met).
	// On multi-validator nodes finalization waits for peer votes via network.go.
	bc.TryFinalizePending(newBlock.CurrentHash, 0.67)

	// Dynamic Liquidity Engine — runs every EpochBlocks, no-op otherwise.
	if bc.DLEngine != nil {
		bc.DLEngine.RunEpoch(bc, newBlock.BlockNumber)
	}

	bc.LastBlockMiningTime = time.Since(start)

	log.Printf("⛏ Merged Block #%d | tx=%d  | time=%d | gas=%d | reward=%+v",
		newBlock.BlockNumber,
		len(finalTxs),
		bc.LastBlockMiningTime,
		newBlock.GasUsed,
		newBlock.RewardBreakdown,
	)
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
