package blockchaincomponent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"runtime"
	"sync"
	"time"

	constantset "github.com/Zotish/DefenceProject/ConstantSet"
)

const (
	GasLimitAdjustmentFactor = 1024
	MinGasLimit              = 5000
	MaxGasLimit              = 8000000
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

const TxWorkers = 8

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

		// Read sender balance from in-memory map.
		// Concurrency-safe as long as ONLY main goroutine writes.
		senderBal := bc.Accounts[tx.From]

		feeTokens := gasUnits * tx.GasPrice
		totalCost := tx.Value + feeTokens

		if senderBal < totalCost {
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

		isContractTx := res.Tx.Type == "contract_create" ||
			res.Tx.Type == "contract_call" ||
			res.Tx.Type == "contract_event"

		if isContractTx {
			res.Tx.Status = constantset.StatusSuccess
			finalTxs = append(finalTxs, res.Tx)
			bc.RecordRecentTx(res.Tx)
			continue
		}

		// 🔥 FAST-PATH: FORCE SUCCESS FOR ALL SYSTEM/LP TX
		isSystem := res.Tx != nil && (res.Tx.IsSystem ||
			res.Tx.Type == "stake" ||
			res.Tx.Type == "unstake" ||
			res.Tx.Type == "lp_reward")

		if isSystem {
			res.Tx.Status = constantset.StatusSuccess

			finalTxs = append(finalTxs, res.Tx)
			bc.RecordRecentTx(res.Tx)

			// DO NOT re-create tx, do NOT deduct balances again
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
			_, err := bc.ContractEngine.Pipeline.ExecuteContractTx(
				res.Tx,
				newBlock.BlockNumber,
			)
			if err != nil {
				res.Tx.Status = constantset.StatusFailed
				continue
			}
		}

		totalTxCost := res.Tx.Value + res.Fee

		if bc.Accounts[res.Tx.From] < totalTxCost {
			res.Tx.Status = constantset.StatusFailed
			continue
		}

		bc.Accounts[res.Tx.From] -= totalTxCost
		bc.Accounts[res.Tx.To] += res.Tx.Value

		res.Tx.Status = constantset.StatusSuccess

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
		totalGasUsed,
		1,
	)
	newBlock.RewardBreakdown = breakdown

	rewardTx := &Transaction{
		From:       "0x0000000000000000000000000000000000000000",
		To:         validator.Address,
		Value:      breakdown.ValidatorReward,
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
	bc.Blocks = append(bc.Blocks, &newBlock)

	used := make(map[string]struct{})
	for _, t := range finalTxs {
		used[t.TxHash] = struct{}{}
	}

	remaining := make([]*Transaction, 0, len(txPool))
	for _, t := range txPool {
		if _, ok := used[t.TxHash]; !ok {
			remaining = append(remaining, t)
		}
	}
	bc.Transaction_pool = remaining

	if err := SaveBlockToDB(&newBlock); err != nil {
		log.Printf("SaveBlockToDB error: %v", err)
	}

	bc.LastBlockMiningTime = time.Since(start)

	log.Printf("⛏ Merged Block #%d | tx=%d  | time=%d | gas=%d | reward=%+v",
		newBlock.BlockNumber,
		len(finalTxs),
		bc.LastBlockMiningTime,
		newBlock.GasUsed,
		newBlock.RewardBreakdown,
	)

	return &newBlock
}

func CalculateHash(newBlock *Block) string {

	data, _ := json.Marshal(newBlock)
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
