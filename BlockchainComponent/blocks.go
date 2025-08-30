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

	BaseFee  uint64 `json:"base_fee"`
	GasUsed  uint64 `json:"gas_used"`
	GasLimit uint64 `json:"gas_limit"`
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
func (bc *Blockchain_struct) MineNewBlock() *Block {

	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from panic in MineNewBlock: %v", r)
		}
	}()

	baseFee := bc.CalculateBaseFee()

	if len(bc.Transaction_pool) > 10000 {
		runtime.GOMAXPROCS(runtime.NumCPU() / 2)
		defer runtime.GOMAXPROCS(runtime.NumCPU())
	}
	validator, err := bc.SelectValidator()
	if err != nil {
		log.Printf("Validator selection error: %v", err)
		return nil
	}

	lastBlock := bc.Blocks[len(bc.Blocks)-1]
	newBlock := NewBlock(lastBlock.BlockNumber, lastBlock.CurrentHash)
	newBlock.GasLimit = bc.CalculateNextGasLimit()

	newBlock.BaseFee = baseFee
	newBlock.Transactions = bc.CopyTransactions()

	// Parallel transaction validation
	var wg sync.WaitGroup
	txChan := make(chan *Transaction, len(bc.Transaction_pool))
	validTxs := make([]*Transaction, 0, len(bc.Transaction_pool))

	// Start worker pool
	for i := 0; i < runtime.NumCPU(); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for tx := range txChan {
				if bc.VerifyTransaction(tx) {
					validTxs = append(validTxs, tx)
				}
			}
		}()
	}

	// Feed transactions to workers
	for _, tx := range bc.Transaction_pool {
		txChan <- tx
	}
	close(txChan)
	wg.Wait()

	newBlock.Transactions = validTxs
	// Process transactions
	totalGasFees := uint64(0)
	for _, tx := range newBlock.Transactions {
		gasTotal := tx.CalculateGasCost() * tx.GasPrice

		if totalGasFees+tx.Gas > uint64(constantset.MaxBlockGas) {
			break
		}

		if tx.GasPrice < (baseFee + tx.PriorityFee) {
			tx.Status = constantset.StatusFailed
			continue
		}

		if bc.Accounts[tx.From] < (tx.Value + gasTotal) {
			tx.Status = constantset.StatusFailed
			continue
		}

		totalGasFees += tx.Gas
		newBlock.Transactions = append(newBlock.Transactions, tx)

		// Deduct from sender
		bc.Accounts[tx.From] -= (tx.Value + gasTotal)

		// Add value to recipient
		bc.Accounts[tx.To] += tx.Value

		// Collect gas fees
		totalGasFees += gasTotal

		tx.Status = constantset.StatusSuccess
		log.Println("tx is succcess", tx.Status)
	}

	// Distribute rewards
	if totalGasFees > 0 {
		// 50% to validator, 50% to slashing pool
		validatorReward := totalGasFees / 2
		bc.Accounts[validator.Address] += validatorReward
		bc.SlashingPool += float64(validatorReward)
	}

	// Block reward (fixed amount)
	blockReward := uint64(10)
	bc.Accounts[validator.Address] += blockReward
	newBlock.GasUsed = totalGasFees
	newBlock.CurrentHash = CalculateHash(&newBlock)
	bc.Blocks = append(bc.Blocks, &newBlock)
	bc.Transaction_pool = []*Transaction{}
	dbCopy := *bc
	dbCopy.Mutex = sync.Mutex{}
	if err := PutIntoDB(dbCopy); err != nil {
		log.Printf("Error saving block: %v", err)
		return nil
	}

	return &newBlock
}

//	func (bc *Blockchain_struct) MineNewBlock() *Block {
//		validator, err := bc.SelectValidator()
//		if err != nil {
//			return nil
//		}
//		lastBlock := bc.Blocks[len(bc.Blocks)-1]
//		newBlock := NewBlock(lastBlock.BlockNumber, lastBlock.CurrentHash)
//		newBlock.Transactions = bc.CopyTransactions()
//		//fmt.Println("here is newblock.transaction", newBlock.Transactions)
//		// blockHash := CalculateHash(&newBlock)
//		// signature, err := validator.SignMessage([]byte(blockHash))
//		// if err == nil {
//		// 	newBlock.ValidatorSignature = hex.EncodeToString(signature)
//		// }
//		for _, tx := range newBlock.Transactions {
//			gasTotal := tx.CalculateGasCost() * tx.GasPrice
//			if bc.Accounts[tx.From] < (tx.Value + gasTotal) {
//				tx.Status = constantset.StatusFailed
//				continue
//			}
//			bc.Accounts[tx.From] -= (tx.Value + gasTotal)
//			bc.Accounts[validator.Address] += gasTotal / 2
//			bc.SlashingPool += float64(gasTotal / 2)
//			bc.Accounts[tx.To] += tx.Value
//			tx.Status = constantset.StatusSuccess
//		}
//		newBlock.CurrentHash = CalculateHash(&newBlock)
//		// Reward validator
//		bc.Accounts[validator.Address] += 10
//			bc.Blocks = append(bc.Blocks, &newBlock)
//			bc.Transaction_pool = []*Transaction{}
//			err = PutIntoDB(*bc)
//			if err != nil {
//				log.Println("Error putting new block into DB:", err)
//				return nil
//			}
//			return &newBlock
//		}
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
