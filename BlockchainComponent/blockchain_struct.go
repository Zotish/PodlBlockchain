package blockchaincomponent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	constantset "github.com/Zotish/DefenceProject/ConstantSet"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	TransactionTTL = 1 * time.Hour
)

type Blockchain_struct struct {
	Blocks           []*Block          `json:"blocks"`
	Transaction_pool []*Transaction    `json:"transaction_pool"`
	Validators       []*Validator      `json:"validator"`
	Accounts         map[string]uint64 `json:"accounts"`
	MinStake         float64           `json:"min_stake"`
	SlashingPool     float64           `json:"slashing_pool"`
	Network          *NetworkService   `json:"-"`
	Mutex            sync.Mutex        `json:"-"`
	BaseFee          uint64            `json:"base_fee"` // Add this field
	VM               *VM               `json:"vm"`       // Add this line

}

func NewBlockchain(genesisBlock Block) *Blockchain_struct {

	exist, _ := KeyExist()
	if exist {
		blockchainStruct, err := GetBlockchain()
		if err != nil {
			return nil
		}
		return blockchainStruct
	} else {
		newBlockchain := new(Blockchain_struct)
		newBlockchain.Blocks = []*Block{}
		if genesisBlock.CurrentHash == "" {
			genesisBlock.CurrentHash = CalculateHash(&genesisBlock)
		}

		if len(newBlockchain.Blocks) == 0 {
			newBlockchain.Blocks = append(newBlockchain.Blocks, &genesisBlock)
		}
		newBlockchain.Transaction_pool = []*Transaction{}
		newBlockchain.Accounts = make(map[string]uint64)
		newBlockchain.MinStake = 100000 * float64(constantset.Decimals)
		newBlockchain.SlashingPool = 0
		newBlockchain.VM = NewVM()
		newBlockchain.Validators = []*Validator{}
		newBlockchain.Network = NewNetworkService(newBlockchain)
		newBlockchain.Mutex = sync.Mutex{}
		if err := newBlockchain.Network.Start(); err != nil {
			log.Printf("Failed to start network service: %v", err)
		}
		// Initialize accounts with some default values

		// newBlockchain.Accounts["0x1"] = 1000000 // Starting balance
		// newBlockchain.Accounts["0x2"] = 1000000 // Starting balance
		// Avoid copying the Mutex field when saving to DB
		blockchainCopy := *newBlockchain
		blockchainCopy.Mutex = sync.Mutex{} // zero value, will be ignored if struct tag is `json:"-"`
		err := PutIntoDB(blockchainCopy)
		if err != nil {
			return nil
		}

		return newBlockchain
	}
}

func (bc *Blockchain_struct) CleanStaleTransactions() {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()

	now := uint64(time.Now().Unix())
	validTxs := make([]*Transaction, 0, len(bc.Transaction_pool))

	for _, tx := range bc.Transaction_pool {
		// Keep transactions that are either:
		// 1. Recent enough (within TTL)
		// 2. High enough priority to potentially be mined
		if now-tx.Timestamp < uint64(TransactionTTL.Seconds()) ||
			tx.GasPrice > bc.BaseFee*2 {
			validTxs = append(validTxs, tx)
		}
	}

	bc.Transaction_pool = validTxs
}
func (bs *Blockchain_struct) ToJsonChain() string {

	block, err := json.Marshal(bs)
	if err != nil {
		log.Println("error")
	}
	return string(block)
}
func (bc *Blockchain_struct) VerifyBlock(block *Blockchain_struct) bool {
	if len(block.Blocks) < 2 {
		return true
	}

	for i := 1; i < len(block.Blocks); i++ {
		current := block.Blocks[i]
		previous := block.Blocks[i-1]

		if current.BlockNumber != previous.BlockNumber+1 {

			return false
		}
		if current.PreviousHash != previous.CurrentHash {

			return false
		}
		if current.TimeStamp < previous.TimeStamp {

			return false
		}
		verifyBlock := *current
		verifyBlock.CurrentHash = ""
		if current.CurrentHash != CalculateHash(&verifyBlock) {
			block.SlashValidator(current.CurrentHash[:8], 0.1, " block hash mismatch")
			return false
		}
		// Add to VerifyBlock():
		// fmt.Printf("Expected: %s\nActual: %s\n",
		// 	current.CurrentHash,
		// 	CalculateHash(&verifyBlock))

	}

	return true
}
func (bc *Blockchain_struct) CopyTransactions() []*Transaction {
	txCopy := make([]*Transaction, len(bc.Transaction_pool))
	copy(txCopy, bc.Transaction_pool)
	return txCopy
}

func (bc *Blockchain_struct) AddNewTxToTheTransaction_pool(tx *Transaction) error {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()

	// Calculate current base fee if not set
	if bc.BaseFee == 0 {
		bc.BaseFee = bc.CalculateBaseFee()
	}

	if uint64(time.Now().Unix())-tx.Timestamp > uint64(TransactionTTL.Seconds()) {
		return fmt.Errorf("transaction expired")
	}

	// Calculate effective gas price
	//effectiveGasPrice := bc.BaseFee + tx.PriorityFee

	for i, existing := range bc.Transaction_pool {
		if existing.From == tx.From && existing.Nonce == tx.Nonce {
			existingEffectivePrice := bc.BaseFee + existing.PriorityFee
			newEffectivePrice := bc.BaseFee + tx.PriorityFee

			if newEffectivePrice > existingEffectivePrice*11/10 { // 10% bump required
				bc.Transaction_pool[i] = tx
				return nil
			}
			return fmt.Errorf("existing transaction has higher or insufficiently lower fee")
		}
	}
	// Replace existing tx if new one has higher fee
	// for i, existing := range bc.Transaction_pool {
	// 	if strings.EqualFold(existing.From, tx.From) && existing.Nonce == tx.Nonce {
	// 		existingEffectivePrice := bc.BaseFee + existing.PriorityFee
	// 		if effectiveGasPrice > existingEffectivePrice {
	// 			bc.Transaction_pool[i] = tx
	// 			return nil
	// 		}
	// 		return fmt.Errorf("existing transaction has higher fee")
	// 	}
	// }

	// Enforce per-account limits
	if bc.countTxsFrom(tx.From) >= constantset.MaxTxsPerAccount {
		return fmt.Errorf("account tx pool limit reached (%d/%d)",
			bc.countTxsFrom(tx.From), constantset.MaxTxsPerAccount)
	}

	bc.Transaction_pool = append(bc.Transaction_pool, tx)

	// Sort by descending effective gas price (BaseFee + PriorityFee)
	sort.Slice(bc.Transaction_pool, func(i, j int) bool {
		iPrice := bc.BaseFee + bc.Transaction_pool[i].PriorityFee
		jPrice := bc.BaseFee + bc.Transaction_pool[j].PriorityFee
		return iPrice > jPrice
	})

	// Enforce global pool limit
	if len(bc.Transaction_pool) > constantset.MaxTxPoolSize {
		// Return error including the minimum required priority fee
		minPrice := bc.BaseFee + bc.Transaction_pool[constantset.MaxTxPoolSize].PriorityFee
		bc.Transaction_pool = bc.Transaction_pool[:constantset.MaxTxPoolSize]
		return fmt.Errorf("txpool full, need at least %d wei priority fee", minPrice-bc.BaseFee)
	}

	tx.Status = constantset.StatusPending
	tx.TxHash = CalculateTransactionHash(*tx)
	bc.Transaction_pool = append(bc.Transaction_pool, tx)
	dbCopy := *bc
	dbCopy.Mutex = sync.Mutex{}
	if err := PutIntoDB(dbCopy); err != nil {
		return fmt.Errorf("failed to update blockchain in DB: %v", err)
	}
	return nil
}

func (bc *Blockchain_struct) CalculateBaseFee() uint64 {
	// If no blocks yet, return initial base fee
	if len(bc.Blocks) == 0 {
		return uint64(constantset.InitialBaseFee)
	}

	lastBlock := bc.Blocks[len(bc.Blocks)-1]

	// For genesis block, return initial base fee
	if lastBlock.BlockNumber == 0 {
		return uint64(constantset.InitialBaseFee)
	}

	// Calculate new base fee based on last block's gas usage
	targetGas := lastBlock.GasLimit / 2
	if targetGas == 0 {
		targetGas = 1
	}

	gasRatio := float64(lastBlock.GasUsed) / float64(targetGas)
	if gasRatio < 0.75 {
		gasRatio = 0.75
	} else if gasRatio > 1.25 {
		gasRatio = 1.25
	}

	newBaseFee := uint64(float64(lastBlock.BaseFee) * gasRatio)

	// Enforce min/max bounds
	if newBaseFee < uint64(constantset.MinBaseFee) {
		return uint64(constantset.MinBaseFee)
	}
	if newBaseFee > uint64(constantset.MaxBaseFee) {
		return uint64(constantset.MaxBaseFee)
	}

	return newBaseFee
}

func (bc *Blockchain_struct) countTxsFrom(from string) int {
	count := 0

	// Check transaction pool first
	for _, tx := range bc.Transaction_pool {
		if strings.EqualFold(tx.From, from) {
			count++
		}
	}

	// Optionally include recent mined transactions (last N blocks)
	recentBlocks := 5 // Configurable
	startBlock := len(bc.Blocks) - recentBlocks
	if startBlock < 0 {
		startBlock = 0
	}

	for i := startBlock; i < len(bc.Blocks); i++ {
		for _, tx := range bc.Blocks[i].Transactions {
			if strings.EqualFold(tx.From, from) {
				count++
			}
		}
	}

	return count
}

//	func (bc *Blockchain_struct) AddNewTxToTheTransaction_pool(transaction *Transaction) error {
//		if bc.Accounts[transaction.From] < transaction.Value {
//			return fmt.Errorf("insufficient balance")
//		}
//		transaction.Status = constantset.StatusPending
//		transaction.TxHash = CalculateTransactionHash(*transaction)
//		bc.Transaction_pool = append(bc.Transaction_pool, transaction)
//		dbCopy := *bc
//		dbCopy.Mutex = sync.Mutex{}
//		if err := PutIntoDB(dbCopy); err != nil {
//			return fmt.Errorf("failed to update blockchain in DB: %v", err)
//		}
//		return nil
//	}

func (bc *Blockchain_struct) CheckBalance(add string) uint64 {
	return bc.Accounts[add]
}

func (bc *Blockchain_struct) FetchBalanceOfWallet(address string) uint64 {
	sum := uint64(0)

	for _, block := range bc.Blocks {
		for _, txn := range block.Transactions {
			if txn.Status == constantset.StatusSuccess {
				if txn.To == address {
					sum += txn.Value
				} else if txn.From == address {
					sum -= txn.Value
				}
			}
		}
	}
	return sum
}

func (bc *Blockchain_struct) VerifySingleBlock(block *Block) bool {
	// Reject blocks that don't extend the longest chain
	lastBlock := bc.Blocks[len(bc.Blocks)-1]
	if block.BlockNumber <= lastBlock.BlockNumber {
		return false
	}

	// Existing hash/transaction validation
	tempHash := block.CurrentHash
	block.CurrentHash = ""
	calculatedHash := CalculateHash(block)
	block.CurrentHash = tempHash

	if calculatedHash != tempHash {
		return false
	}

	// Verify transactions (existing logic)
	for _, tx := range block.Transactions {
		if !bc.VerifyTransaction(tx) {
			return false
		}
	}
	now := uint64(time.Now().Unix())
	if block.TimeStamp > now+30 { // 30 seconds in future max
		return false
	}
	if now-block.TimeStamp > 3600 { // 1 hour in past max
		return false
	}

	// 2. Check gas limits
	totalGas := uint64(0)
	for _, tx := range block.Transactions {
		totalGas += tx.Gas * tx.GasPrice
		if totalGas > uint64(constantset.MaxBlockGas) {
			return false
		}
	}

	// 3. Check validator is active
	validatorActive := false
	for _, v := range bc.Validators {
		if strings.HasPrefix(block.CurrentHash, v.Address) {
			validatorActive = true
			break
		}
	}

	expectedBaseFee := bc.CalculateBaseFee()
	if block.BaseFee != expectedBaseFee {
		log.Printf("Invalid base fee: got %d, expected %d",
			block.BaseFee, expectedBaseFee)
		return false
	}
	return validatorActive
}

func (bc *Blockchain_struct) GetValidatorStats(address string) map[string]interface{} {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()

	for _, v := range bc.Validators {
		if v.Address == address {
			return map[string]interface{}{
				"address":         v.Address,
				"stake":           v.LPStakeAmount,
				"liquidity_power": v.LiquidityPower,
				"penalty_score":   v.PenaltyScore,
				"blocks_proposed": v.BlocksProposed,
				"blocks_included": v.BlocksIncluded,
				"last_active":     v.LastActive,
				"lock_time":       v.LockTime,
			}
		}
	}
	return nil
}

func (bc *Blockchain_struct) GetNetworkStats() map[string]interface{} {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()

	validators := make([]map[string]interface{}, len(bc.Validators))
	for i, v := range bc.Validators {
		validators[i] = map[string]interface{}{
			"address":         v.Address,
			"stake":           v.LPStakeAmount,
			"liquidity_power": v.LiquidityPower,
			"penalty_score":   v.PenaltyScore,
		}
	}

	return map[string]interface{}{
		"block_height":       len(bc.Blocks),
		"validators":         validators,
		"transaction_pool":   len(bc.Transaction_pool),
		"slashing_pool":      bc.SlashingPool,
		"average_block_time": bc.CalculateAverageBlockTime(),
	}
}

func (bc *Blockchain_struct) CalculateAverageBlockTime() float64 {
	if len(bc.Blocks) < 2 {
		return 0
	}

	totalTime := float64(bc.Blocks[len(bc.Blocks)-1].TimeStamp - bc.Blocks[0].TimeStamp)
	return totalTime / float64(len(bc.Blocks)-1)
}

// func (bc *Blockchain_struct) VerifySingleBlock(block *Block) bool {
// 	if block.BlockNumber <= bc.Blocks[len(bc.Blocks)-1].BlockNumber {
// 		return false
// 	}
// 	// 1. Verify block hash
// 	tempHash := block.CurrentHash
// 	block.CurrentHash = ""
// 	calculatedHash := CalculateHash(block)
// 	block.CurrentHash = tempHash
// 	if calculatedHash != tempHash {
// 		return false
// 	}
// 	// 2. Verify transactions in the block
// 	for _, tx := range block.Transactions {
// 		if !bc.VerifyTransaction(tx) {
// 			return false
// 		}
// 	}
// 	// 3. Verify block number is sequential
// 	if len(bc.Blocks) > 0 {
// 		lastBlock := bc.Blocks[len(bc.Blocks)-1]
// 		if block.BlockNumber != lastBlock.BlockNumber+1 {
// 			return false
// 		}
// 	}
// 	return true
// }

func (bc *Blockchain_struct) VerifyTransaction(transaction *Transaction) bool {
	// Basic validation
	if transaction.Value == 0 || transaction.From == "" || transaction.To == "" {
		transaction.Status = constantset.StatusFailed
		return false
	}

	// Validate addresses
	if !strings.HasPrefix(transaction.From, "0x") || len(transaction.From) != 42 ||
		!strings.HasPrefix(transaction.To, "0x") || len(transaction.To) != 42 {
		transaction.Status = constantset.StatusFailed
		return false
	}
	if transaction.ChainID != uint64(constantset.ChainID) {
		transaction.Status = constantset.StatusFailed
		log.Printf("Transaction %s failed: invalid chain ID", transaction.TxHash)
		return false
	}

	// Add timestamp validation (prevent replay of old transactions)
	if uint64(time.Now().Unix())-transaction.Timestamp > 3600 { // 1 hour expiry
		transaction.Status = constantset.StatusFailed
		log.Printf("Transaction %s expired", transaction.TxHash)
		return false
	}

	// Calculate total cost (value + gas)
	totalCost := transaction.Value + (transaction.GasPrice * transaction.CalculateGasCost())

	// Check sender balance
	if bc.Accounts[transaction.From] < totalCost {
		transaction.Status = constantset.StatusFailed
		return false
	}

	// Check nonce
	expectedNonce := bc.GetAccountNonce(transaction.From)
	if transaction.Nonce != expectedNonce {
		transaction.Status = constantset.StatusFailed
		return false
	}

	// Verify signature
	if !bc.VerifyTransactionSignature(transaction) {
		transaction.Status = constantset.StatusFailed
		return false
	}

	transaction.Status = constantset.StatusPending
	return true
}

// func (bc *Blockchain_struct) VerifyTransaction(transaction *Transaction) bool {
// 	log.Printf("Verifying TX: From=%s, To=%s, Value=%d, Nonce=%d",
// 		transaction.From, transaction.To, transaction.Value, transaction.Nonce)
// 	if transaction.Value == 0 || transaction.From == "" || transaction.To == "" {
// 		log.Println("Fail reason: Basic validation failed")
// 		transaction.Status = constantset.StatusFailed
// 		return false
// 	}
// 	requiredAmount := (transaction.Value + (transaction.GasPrice * transaction.CalculateGasCost()))
// 	if requiredAmount > bc.Accounts[transaction.From] {
// 		log.Printf("Fail reason: Insufficient funds (need %d, have %d)",
// 			requiredAmount, bc.Accounts[transaction.From])
// 		transaction.Status = constantset.StatusFailed
// 		return false
// 	}
// 	if transaction.Nonce <= bc.GetAccountNonce(transaction.From) {
// 		log.Printf("Fail reason: Bad nonce (tx nonce %d, expected > %d)",
// 			transaction.Nonce, bc.GetAccountNonce(transaction.From))
// 		transaction.Status = constantset.StatusFailed
// 		return false
// 	}
// 	if !bc.VerifyTransactionSignature(transaction) {
// 		log.Println("Fail reason: Invalid signature")
// 		transaction.Status = constantset.StatusFailed
// 		return false
// 	}
// 	transaction.Status = constantset.StatusPending
// 	log.Printf("Transaction %s verified successfully", transaction.TxHash)
// 	return true
// }
// func (bc *Blockchain_struct) VerifyTransaction(transaction *Transaction) bool {
// 	if transaction.Value == 0 || transaction.From == "" || transaction.To == "" || len(transaction.From) == 0 || len(transaction.To) == 0 {
// 		transaction.Status = constants.StatusFailed
// 		return false
// 	}
// 	requiredAmount := (transaction.Value + (transaction.GasPrice * transaction.CalculateGasCost()))
// 	if requiredAmount > bc.Accounts[transaction.From] {
// 		transaction.Status = constants.StatusFailed
// 		return false
// 	}
// 	if transaction.Nonce <= bc.GetAccountNonce(transaction.From) {
// 		transaction.Status = constants.StatusFailed
// 		return false
// 	}
// 	// Check transaction expiration (e.g., 1 hour)
// 	if uint64(time.Now().Unix())-transaction.Timestamp > 3600 {
// 		transaction.Status = constants.StatusFailed
// 		return false
// 	}
// 	// Verify signature (implement this)
// 	if !bc.VerifyTransactionSignature(transaction) {
// 		transaction.Status = constants.StatusFailed
// 		return false
// 	}
// 	transaction.Status = constants.StatusPending
//		return true
//	}
// func (bc *Blockchain_struct) GetAccountNonce(address string) uint64 {
// 	// First check pending transactions in the pool
// 	pendingNonce := uint64(0)
// 	for _, tx := range bc.Transaction_pool {
// 		if tx.From == address && tx.Nonce >= pendingNonce {
// 			pendingNonce = tx.Nonce + 1
// 		}
// 	}
// 	// Then check confirmed transactions in blocks
// 	confirmedNonce := uint64(0)
// 	for _, block := range bc.Blocks {
// 		for _, tx := range block.Transactions {
// 			if tx.From == address {
// 				if tx.Nonce >= confirmedNonce {
// 					confirmedNonce = tx.Nonce + 1
// 				}
// 			}
// 		}
// 	}
//		// Return the highest nonce found + 1
//		if pendingNonce > confirmedNonce {
//			return pendingNonce
//		}
//		return confirmedNonce
//	}

func (bc *Blockchain_struct) GetAccountNonce(address string) uint64 {
	// Check confirmed transactions in blocks first
	highestNonce := uint64(0)
	for _, block := range bc.Blocks {
		for _, tx := range block.Transactions {
			if tx.From == address && tx.Nonce >= highestNonce {
				highestNonce = tx.Nonce + 1
			}
		}
	}

	// Then check pending transactions
	for _, tx := range bc.Transaction_pool {
		if tx.From == address && tx.Nonce >= highestNonce {
			highestNonce = tx.Nonce + 1
		}
	}

	return highestNonce
}
func RemoveFailedTx(pool []*Transaction, tx *Transaction) []*Transaction {
	for i, t := range pool {
		if t.TxHash == tx.TxHash {
			return append(pool[:i], pool[i+1:]...)
		}
	}
	return pool
}

// func (bc *Blockchain_struct) VerifyTransactionSignature(tx *Transaction) bool {
// 	// Reconstruct the exact data that was signed
// 	signingData := map[string]interface{}{
// 		"from":      tx.From,
// 		"to":        tx.To,
// 		"value":     tx.Value,
// 		"data":      hex.EncodeToString(tx.Data), // Encode binary data as hex
// 		"gas":       tx.Gas,
// 		"gas_price": tx.GasPrice,
// 		"nonce":     tx.Nonce,
// 		"chain_id":  tx.ChainID,
// 		"timestamp": tx.Timestamp,
// 		"status":    tx.Status, // Include status in signature
// 	}
// 	// Convert to canonical JSON (sorted keys, no whitespace)
// 	data, err := json.Marshal(signingData)
// 	if err != nil {
// 		log.Printf("Error marshaling signing data: %v", err)
// 		return false
// 	}
// 	// Double SHA-256 hash (common in blockchain systems)
// 	firstHash := sha256.Sum256(data)
// 	hash := sha256.Sum256(firstHash[:])
// 	// Verify the signature using ECDSA
// 	if len(tx.Sig) != 65 {
// 		log.Printf("Invalid signature length: %d", len(tx.Sig))
// 		return false
// 	}
// 	// The signature should be in [R || S || V] format
// 	// r := new(big.Int).SetBytes(tx.Sig[:32])
// 	// s := new(big.Int).SetBytes(tx.Sig[32:64])
// 	// v := tx.Sig[64]
// 	// Recover the public key
// 	pubKey, err := crypto.Ecrecover(hash[:], tx.Sig)
// 	if err != nil {
// 		log.Printf("Error recovering public key: %v", err)
// 		return false
// 	}
// 	// Verify the signature
// 	if !crypto.VerifySignature(pubKey, hash[:], tx.Sig[:64]) {
// 		log.Printf("Signature verification failed")
// 		return false
// 	}
// 	// Verify the recovered address matches the transaction 'from' address
// 	pubKeyObj, err := crypto.UnmarshalPubkey(pubKey)
// 	if err != nil {
// 		log.Printf("Error unmarshaling public key: %v", err)
// 		return false
// 	}
// 	recoveredAddr := crypto.PubkeyToAddress(*pubKeyObj).Hex()
// 	if !strings.EqualFold(recoveredAddr, tx.From) {
// 		log.Printf("Recovered address %s doesn't match transaction from %s",
// 			recoveredAddr, tx.From)
// 		return false
// 	}
// 	return true
// }

func (bc *Blockchain_struct) VerifyTransactionSignature(tx *Transaction) bool {
	// 1. Verify chain ID matches our network
	if tx.ChainID != uint64(constantset.ChainID) {
		log.Printf("Invalid chain ID: got %d, want %d", tx.ChainID, constantset.ChainID)
		return false
	}

	// Check signature format
	if len(tx.Sig) != 65 {
		log.Printf("Invalid signature length: %d", len(tx.Sig))
		return false
	}

	// EIP-155 recovery ID check
	if tx.Sig[64] != 27 && tx.Sig[64] != 28 {
		log.Printf("Invalid recovery ID: %d", tx.Sig[64])
		return false
	}

	// 2. Reconstruct signing data exactly as signed
	signingData := map[string]interface{}{
		"from":      tx.From,
		"to":        tx.To,
		"value":     tx.Value,
		"data":      hex.EncodeToString(tx.Data),
		"gas":       tx.Gas,
		"gas_price": tx.GasPrice,
		"nonce":     tx.Nonce,
		"chain_id":  tx.ChainID, // Must match original
		"timestamp": tx.Timestamp,
	}

	data, err := json.Marshal(signingData)
	if err != nil {
		log.Printf("Error marshaling signing data: %v", err)
		return false
	}

	// 3. Double hash verification
	firstHash := sha256.Sum256(data)
	hash := sha256.Sum256(firstHash[:])

	// 4. Signature recovery with chain ID protection
	if len(tx.Sig) != 65 {
		log.Printf("Invalid signature length: %d", len(tx.Sig))
		return false
	}

	pubKey, err := crypto.Ecrecover(hash[:], tx.Sig)
	if err != nil {
		log.Printf("Error recovering public key: %v", err)
		return false
	}

	// 5. Verify the signature
	if !crypto.VerifySignature(pubKey, hash[:], tx.Sig[:64]) {
		log.Printf("Signature verification failed")
		return false
	}

	// 6. Verify the recovered address
	pubKeyObj, err := crypto.UnmarshalPubkey(pubKey)
	if err != nil {
		log.Printf("Error unmarshaling public key: %v", err)
		return false
	}

	recoveredAddr := crypto.PubkeyToAddress(*pubKeyObj).Hex()
	if !strings.EqualFold(recoveredAddr, tx.From) {
		log.Printf("Recovered address %s doesn't match transaction from %s",
			recoveredAddr, tx.From)
		return false
	}

	return true
}

func (bc *Blockchain_struct) ResolveForks(newBlocks []*Block) error {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()

	currentHeight := len(bc.Blocks)
	newChain := make([]*Block, len(newBlocks))
	copy(newChain, newBlocks)

	// Verify the new chain
	if !bc.VerifyChain(newChain) {
		return fmt.Errorf("invalid chain received")
	}

	// Longest chain rule
	if len(newChain) > currentHeight {
		// Reorganize transactions from orphaned blocks
		var orphanedTxs []*Transaction
		for _, block := range bc.Blocks[currentHeight:] {
			orphanedTxs = append(orphanedTxs, block.Transactions...)
		}

		// Switch to new chain
		bc.Blocks = bc.Blocks[:currentHeight]
		bc.Blocks = append(bc.Blocks, newChain...)

		// Re-add valid transactions from orphaned blocks
		for _, tx := range orphanedTxs {
			if tx.Status == constantset.StatusSuccess {
				tx.Status = constantset.StatusPending
				bc.AddNewTxToTheTransaction_pool(tx)
			}
		}

		log.Printf("Chain reorganization occurred. New height: %d", len(bc.Blocks))
	}

	return nil
}

func (bc *Blockchain_struct) VerifyChain(chain []*Block) bool {
	if len(chain) == 0 {
		return false
	}

	// Verify genesis block
	if chain[0].BlockNumber != 0 || chain[0].PreviousHash != "0x_Genesis" {
		return false
	}

	// Verify subsequent blocks
	for i := 1; i < len(chain); i++ {
		if chain[i].BlockNumber != chain[i-1].BlockNumber+1 ||
			chain[i].PreviousHash != chain[i-1].CurrentHash ||
			!bc.VerifySingleBlock(chain[i]) {
			return false
		}
	}

	return true
}
