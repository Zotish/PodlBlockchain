// package blockchaincomponent

// import (
// 	"crypto/sha256"
// 	"encoding/hex"
// 	"encoding/json"
// 	"log"
// 	"runtime"
// 	"strconv"
// 	"time"

// 	constantset "github.com/Zotish/DefenceProject/ConstantSet"
// )

// const (
// 	GasLimitAdjustmentFactor = 1024
// 	MinGasLimit              = 5000
// 	MaxGasLimit              = 8000000
// )

// type VerifiedTx struct {
// 	Tx      *Transaction
// 	GasUsed uint64
// 	Fee     uint64
// 	Valid   bool
// 	Err     error
// }

// // -------- WORKER POOL CONFIG --------
// const TxWorkers = 8 // set to runtime.NumCPU() if you want full speed

// // -------- WORKER POOL --------
// func (bc *Blockchain_struct) verifyTxWorker(tasks <-chan *Transaction, out chan<- VerifiedTx, baseFee uint64) {
// 	for tx := range tasks {

// 		// GAS UNITS for tx
// 		gasUnits := tx.CalculateGasCost()

// 		// Fast fail: block gas cap
// 		if gasUnits == 0 {
// 			out <- VerifiedTx{Tx: tx, Valid: false}
// 			continue
// 		}

// 		// EIP-1559: require gas price >= baseFee + priorityFee
// 		minRequired := tx.PriorityFee + baseFee
// 		if tx.GasPrice < minRequired {
// 			out <- VerifiedTx{Tx: tx, Valid: false}
// 			continue
// 		}

// 		// Check signature / format / nonce
// 		if !bc.VerifyTransaction(tx) {
// 			out <- VerifiedTx{Tx: tx, Valid: false}
// 			continue
// 		}

// 		// Balance checks
// 		senderBal, _ := bc.GetWalletBalance(tx.From)

// 		feeTokens := gasUnits * tx.GasPrice
// 		totalCost := tx.Value + feeTokens

// 		if senderBal < totalCost {
// 			out <- VerifiedTx{Tx: tx, Valid: false}
// 			continue
// 		}

// 		out <- VerifiedTx{
// 			Tx:      tx,
// 			GasUsed: gasUnits,
// 			Fee:     feeTokens,
// 			Valid:   true,
// 		}
// 	}
// }

// type Block struct {
// 	BlockNumber  uint64         `json:"block_number"`
// 	PreviousHash string         `json:"previous_hash"`
// 	CurrentHash  string         `json:"current_hash"`
// 	TimeStamp    uint64         `json:"timestamp"`
// 	Transactions []*Transaction `json:"transactions"`

// 	BaseFee  uint64 `json:"base_fee"`
// 	GasUsed  uint64 `json:"gas_used"`
// 	GasLimit uint64 `json:"gas_limit"`
// }

// func NewBlock(blockNumber uint64, prevHash string) Block {
// 	newBlock := new(Block)
// 	newBlock.BlockNumber = blockNumber + 1
// 	newBlock.TimeStamp = uint64(time.Now().Unix())
// 	newBlock.PreviousHash = prevHash
// 	newBlock.Transactions = []*Transaction{}
// 	newBlock.GasLimit = uint64(constantset.MaxBlockGas)
// 	newBlock.BaseFee = 0
// 	return *newBlock
// }
// func (bc *Blockchain_struct) CalculateNextGasLimit() uint64 {
// 	if len(bc.Blocks) == 0 {
// 		return MaxGasLimit
// 	}

// 	parent := bc.Blocks[len(bc.Blocks)-1]

// 	// Adjust based on parent gas used
// 	var newLimit uint64
// 	if parent.GasUsed > parent.GasLimit*3/4 {
// 		// Increase if block was mostly full
// 		newLimit = parent.GasLimit + parent.GasLimit/GasLimitAdjustmentFactor
// 	} else if parent.GasUsed < parent.GasLimit/2 {
// 		// Decrease if block was mostly empty
// 		newLimit = parent.GasLimit - parent.GasLimit/GasLimitAdjustmentFactor
// 	} else {
// 		// Keep the same
// 		newLimit = parent.GasLimit
// 	}

// 	// Apply bounds
// 	if newLimit < MinGasLimit {
// 		return MinGasLimit
// 	}
// 	if newLimit > MaxGasLimit {
// 		return MaxGasLimit
// 	}

// 	return newLimit
// }

// // func (bc *Blockchain_struct) MineNewBlock() *Block {
// // 	start := time.Now()

// // 	lastBlock := bc.Blocks[len(bc.Blocks)-1]
// // 	baseFee := bc.CalculateBaseFee()

// // 	newBlock := NewBlock(lastBlock.BlockNumber, lastBlock.CurrentHash)
// // 	newBlock.GasLimit = bc.CalculateNextGasLimit()
// // 	newBlock.BaseFee = baseFee

// // 	validator, _ := bc.SelectValidator()

// // 	// --- WORKER POOL SETUP ---
// // 	taskChan := make(chan *Transaction, len(bc.Transaction_pool))
// // 	resultChan := make(chan VerifiedTx, len(bc.Transaction_pool))

// // 	workers := runtime.NumCPU()
// // 	var wg sync.WaitGroup
// // 	wg.Add(workers)

// // 	for i := 0; i < workers; i++ {
// // 		go func() {
// // 			bc.verifyTxWorker(taskChan, resultChan, baseFee)
// // 			wg.Done()
// // 		}()
// // 	}

// // 	for _, tx := range bc.Transaction_pool {
// // 		taskChan <- tx
// // 	}
// // 	close(taskChan)

// // 	go func() {
// // 		wg.Wait()
// // 		close(resultChan)
// // 	}()

// // 	// --- SHARDED STATE BUFFER ---
// // 	stateCommit := NewStateCommit()
// // 	accepted := make([]*Transaction, 0, len(bc.Transaction_pool))

// // 	var totalGasUsed uint64
// // 	var totalFees uint64

// // 	// --- COLLECT RESULTS ---
// // 	for res := range resultChan {
// // 		tx := res.Tx
// // 		if !res.Valid {
// // 			tx.Status = constantset.StatusFailed
// // 			continue
// // 		}

// // 		if totalGasUsed+res.GasUsed > uint64(constantset.MaxBlockGas) {
// // 			tx.Status = constantset.StatusFailed
// // 			continue
// // 		}

// // 		// read actual balances only once
// // 		sBal, _ := bc.GetWalletBalance(tx.From)
// // 		rBal, _ := bc.GetWalletBalance(tx.To)

// // 		// anti–double-spend:
// // 		// accumulate balance inside shard commit
// // 		sBal -= (tx.Value + res.Fee)
// // 		rBal += tx.Value

// // 		stateCommit.AddBalance(tx.From, sBal)
// // 		stateCommit.AddBalance(tx.To, rBal)

// // 		tx.Status = constantset.StatusSuccess

// // 		accepted = append(accepted, tx)
// // 		totalGasUsed += res.GasUsed
// // 		totalFees += res.Fee
// // 	}

// // 	// --- APPLY SHARDED STATE IN PARALLEL ---
// // 	var wgState sync.WaitGroup
// // 	wgState.Add(StateShardCount)

// // 	for i := 0; i < StateShardCount; i++ {
// // 		go func(idx int) {
// // 			sh := stateCommit.shards[idx]

// // 			sh.mu.Lock()
// // 			for addr, bal := range sh.data {
// // 				bc.Accounts[addr] = bal
// // 			}
// // 			sh.mu.Unlock()

// // 			wgState.Done()
// // 		}(i)
// // 	}

// // 	wgState.Wait()

// // 	// --- BLOCK FINALIZATION ---
// // 	newBlock.Transactions = accepted
// // 	newBlock.GasUsed = totalGasUsed
// // 	newBlock.CurrentHash = CalculateHash(&newBlock)

// // 	bc.Blocks = append(bc.Blocks, &newBlock)

// // 	// --- REWARD LOGIC (unchanged) ---
// // 	baseReward := uint64(10)
// // 	variable := totalFees / 2
// // 	totalRewardPool := baseReward + variable
// // 	bc.Accounts[validator.Address] += totalRewardPool

// // 	bc.RecordSystemTx(
// // 		"0x0000000000000000000000000000000000000000",
// // 		validator.Address,
// // 		totalRewardPool,
// // 		0, 0,
// // 		constantset.StatusSuccess,
// // 		false, "blockReward",
// // 		[]string{strconv.FormatUint(newBlock.BlockNumber, 10)},
// // 	)

// // 	// --- FAST MEMPOOL CLEANUP ---
// // 	included := make(map[string]bool, len(accepted))
// // 	for _, tx := range accepted {
// // 		included[tx.TxHash] = true
// // 	}
// // 	left := bc.Transaction_pool[:0]
// // 	for _, tx := range bc.Transaction_pool {
// // 		if !included[tx.TxHash] {
// // 			left = append(left, tx)
// // 		}
// // 	}
// // 	bc.Transaction_pool = left

// // 	// --- DB BATCH WRITE ---
// // 	SaveBlockToDB(&newBlock)

// // 	bc.LastBlockMiningTime = time.Since(start)

// // 	log.Printf(
// // 		"⚡ OptimizedC block #%d | tx=%d | time=%v | gas=%d",
// // 		newBlock.BlockNumber, len(accepted),
// // 		bc.LastBlockMiningTime, totalGasUsed,
// // 	)

// // 	return &newBlock
// // }

// //this work for 20-80 tps
// // func (bc *Blockchain_struct) MineNewBlock() *Block {
// // 	startTime := time.Now()
// // 	defer func() {
// // 		if r := recover(); r != nil {
// // 			log.Printf("Recovered from panic in MineNewBlock: %v", r)
// // 		}
// // 	}()

// // 	// Base fee for this block
// // 	baseFee := bc.CalculateBaseFee()

// // 	if len(bc.Transaction_pool) > 10000 {
// // 		runtime.GOMAXPROCS(runtime.NumCPU() / 2)
// // 		defer runtime.GOMAXPROCS(runtime.NumCPU())
// // 	}

// // 	// Select proposer
// // 	validator, err := bc.SelectValidator()
// // 	if err != nil {
// // 		log.Printf("Validator selection error: %v", err)
// // 		return nil
// // 	}

// // 	// Build block shell
// // 	lastBlock := bc.Blocks[len(bc.Blocks)-1]
// // 	newBlock := NewBlock(lastBlock.BlockNumber, lastBlock.CurrentHash)
// // 	newBlock.GasLimit = bc.CalculateNextGasLimit()
// // 	newBlock.BaseFee = baseFee

// // 	// Track gas UNITS separately from fee TOKENS
// // 	var totalGasUnits uint64 = 0
// // 	var totalGasFeesTokens uint64 = 0

// // 	processedTxs := make([]*Transaction, 0, len(bc.Transaction_pool))

// // 	// Snapshot the pool so we iterate over a stable view
// // 	txPool := make([]*Transaction, len(bc.Transaction_pool))
// // 	copy(txPool, bc.Transaction_pool)

// // 	for _, tx := range txPool {
// // 		gasUnits := tx.CalculateGasCost() // units, not tokens

// // 		// Enforce block gas cap (units)
// // 		if totalGasUnits+gasUnits > uint64(constantset.MaxBlockGas) {
// // 			break
// // 		}

// // 		// Basic validity (sig, nonce, format)
// // 		if !bc.VerifyTransaction(tx) {
// // 			tx.Status = constantset.StatusFailed
// // 			continue
// // 		}

// // 		// EIP-1559-ish constraint: require gasPrice >= baseFee + tip
// // 		minRequired := tx.PriorityFee + newBlock.BaseFee
// // 		effectiveGasPrice := tx.GasPrice
// // 		if effectiveGasPrice < minRequired {
// // 			tx.Status = constantset.StatusFailed
// // 			continue
// // 		}

// // 		// ✅ REAL WALLET BALANCE CHECK
// // 		senderBal, err := bc.GetWalletBalance(tx.From)
// // 		if err != nil {
// // 			log.Printf("GetWalletBalance(%s) failed: %v", tx.From, err)
// // 			tx.Status = constantset.StatusFailed
// // 			continue
// // 		}
// // 		receiverBal, err := bc.GetWalletBalance(tx.To)
// // 		if err != nil {
// // 			// For a brand-new account, treat as 0
// // 			receiverBal = 0
// // 		}

// // 		feeTokens := gasUnits * effectiveGasPrice
// // 		totalCost := tx.Value + feeTokens
// // 		if senderBal < totalCost {
// // 			tx.Status = constantset.StatusFailed
// // 			continue
// // 		}

// // 		// Execute state changes using the live balances; cache them in bc.Accounts
// // 		senderBal -= totalCost
// // 		receiverBal += tx.Value

// // 		bc.Accounts[tx.From] = senderBal
// // 		bc.Accounts[tx.To] = receiverBal

// // 		totalGasUnits += gasUnits
// // 		totalGasFeesTokens += feeTokens

// // 		tx.Status = constantset.StatusSuccess
// // 		processedTxs = append(processedTxs, tx)
// // 	}

// // 	// 🔥 Merge pending contract-call & deployment fees into gas pool
// // 	if bc.PendingFeePool != nil {
// // 		for _, fee := range bc.PendingFeePool {
// // 			totalGasFeesTokens += fee
// // 		}
// // 		bc.PendingFeePool = make(map[string]uint64)
// // 	}

// // 	// Finalize block fields
// // 	newBlock.Transactions = processedTxs
// // 	for _, tx := range newBlock.Transactions {
// // 		tx.Status = constantset.StatusSuccess
// // 		bc.RecordRecentTx(tx)
// // 	}
// // 	newBlock.GasUsed = totalGasUnits                // store gas units
// // 	newBlock.CurrentHash = CalculateHash(&newBlock) // hash

// // 	// Append block to chain
// // 	bc.Blocks = append(bc.Blocks, &newBlock)

// // 	// --- PoDL Reward Distribution ---
// // 	// Build a reward pool from base reward + slice of fees (in TOKENS)
// // 	baseBlockReward := uint64(10)
// // 	variableSlice := totalGasFeesTokens / 2

// // 	// Modulate by load (based on gas UNITS consumption vs target units)
// // 	targetGas := newBlock.GasLimit / 2
// // 	load := 1.0
// // 	if targetGas > 0 {
// // 		ratio := float64(newBlock.GasUsed) / float64(targetGas)
// // 		if ratio > 1.5 {
// // 			ratio = 1.5
// // 		}
// // 		if ratio < 0.75 {
// // 			ratio = 0.75
// // 		}
// // 		load = ratio
// // 	}

// // 	totalRewardPool := uint64(float64(baseBlockReward+variableSlice) * load)

// // 	// Distribute
// // 	leaderCut := (totalRewardPool * uint64(constantset.Leader)) / 100
// // 	bc.Accounts[validator.Address] += leaderCut

// // 	valMap := bc.CalculateRewardForValidator(totalRewardPool)
// // 	lpMap := bc.CalculateRewardForLiquidity(totalRewardPool)

// // 	// dist collects the final per-address reward (leader + validator + LP)
// // 	dist := map[string]uint64{validator.Address: leaderCut}
// // 	for addr, amt := range valMap {
// // 		if amt == 0 {
// // 			continue
// // 		}
// // 		bc.Accounts[addr] += amt
// // 		dist[addr] += amt
// // 	}
// // 	for addr, amt := range lpMap {
// // 		if amt == 0 {
// // 			continue
// // 		}
// // 		bc.Accounts[addr] += amt
// // 		dist[addr] += amt
// // 	}

// // 	// Track reward history
// // 	bc.RewardHistory = append(bc.RewardHistory, RewardSnapshot{
// // 		BlockNumber: newBlock.BlockNumber,
// // 		BaseFee:     newBlock.BaseFee,
// // 		GasUsed:     newBlock.GasUsed,
// // 		Dist:        dist,
// // 	})

// // 	// --- NEW: Write reward payouts as REAL block txs ---
// // 	// NOTE: we ALREADY updated bc.Accounts above, so here we only
// // 	// materialize tx objects & push to block / recent-tx history.
// // 	const rewardPoolAddr = "0x0000000000000000000000000000000000000000"

// // 	for addr, amount := range dist {
// // 		if amount == 0 {
// // 			continue
// // 		}

// // 		// Use helper so TxHash + timestamps + recent-tx are handled in one place
// // 		tx := bc.RecordSystemTx(
// // 			rewardPoolAddr, // system / reward pool
// // 			addr,
// // 			amount,
// // 			0, // gasUsed
// // 			0, // gasPrice
// // 			constantset.StatusSuccess,
// // 			false, // not a contract call
// // 			"blockReward",
// // 			[]string{strconv.FormatUint(newBlock.BlockNumber, 10)},
// // 		)

// // 		// Attach to block so explorer can display them as mined txs
// // 		newBlock.Transactions = append(newBlock.Transactions, tx)
// // 	}

// // 	// Trim tx pool by removing only included txs
// // 	if len(processedTxs) > 0 {
// // 		included := make(map[string]struct{}, len(processedTxs))
// // 		for _, itx := range processedTxs {
// // 			included[itx.TxHash] = struct{}{}
// // 		}
// // 		remaining := make([]*Transaction, 0, len(bc.Transaction_pool))
// // 		for _, ptx := range bc.Transaction_pool {
// // 			if _, ok := included[ptx.TxHash]; !ok {
// // 				remaining = append(remaining, ptx)
// // 			}
// // 		}
// // 		bc.Transaction_pool = remaining
// // 	}

// // 	// Persist
// // 	if err := SaveBlockToDB(&newBlock); err != nil {
// // 		log.Printf("Error saving block: %v", err)
// // 		return nil
// // 	}
// // 	miningDuration := time.Since(startTime)

// // 	bc.LastBlockMiningTime = miningDuration
// // 	log.Printf("⏱️ Block #%d mined in %v | txs=%d | gasUsed(units)=%d | gasFees(tokens)=%d | rewardPool=%d",
// // 		newBlock.BlockNumber, time.Since(startTime), len(newBlock.Transactions), totalGasUnits, totalGasFeesTokens, totalRewardPool)

// // 	return &newBlock
// // }

// func (bc *Blockchain_struct) MineNewBlock() *Block {
// 	startTime := time.Now()
// 	defer func() {
// 		if r := recover(); r != nil {
// 			log.Printf("Recovered from panic in MineNewBlock: %v", r)
// 		}
// 	}()

// 	// Base fee for this block
// 	baseFee := bc.CalculateBaseFee()

// 	if len(bc.Transaction_pool) > 10000 {
// 		runtime.GOMAXPROCS(runtime.NumCPU() / 2)
// 		defer runtime.GOMAXPROCS(runtime.NumCPU())
// 	}
// 	// Select proposer
// 	validator, err := bc.SelectValidator()
// 	if err != nil {
// 		log.Printf("Validator selection error: %v", err)
// 		return nil
// 	}

// 	// Build block shell
// 	lastBlock := bc.Blocks[len(bc.Blocks)-1]
// 	newBlock := NewBlock(lastBlock.BlockNumber, lastBlock.CurrentHash)
// 	newBlock.GasLimit = bc.CalculateNextGasLimit()
// 	newBlock.BaseFee = baseFee

// 	// Track gas UNITS separately from fee TOKENS
// 	var totalGasUnits uint64 = 0
// 	var totalGasFeesTokens uint64 = 0

// 	processedTxs := make([]*Transaction, 0, len(bc.Transaction_pool))

// 	// Snapshot the pool so we iterate over a stable view
// 	txPool := make([]*Transaction, len(bc.Transaction_pool))
// 	copy(txPool, bc.Transaction_pool)

// 	for _, tx := range txPool {
// 		gasUnits := tx.CalculateGasCost() // units, not tokens

// 		// Enforce block gas cap (units)
// 		if totalGasUnits+gasUnits > uint64(constantset.MaxBlockGas) {
// 			break
// 		}

// 		// Basic validity (sig, nonce, format)
// 		if !bc.VerifyTransaction(tx) {
// 			tx.Status = constantset.StatusFailed
// 			continue
// 		}

// 		// EIP-1559-ish constraint: require gasPrice >= baseFee + tip
// 		minRequired := tx.PriorityFee + newBlock.BaseFee
// 		effectiveGasPrice := tx.GasPrice
// 		if effectiveGasPrice < minRequired {
// 			tx.Status = constantset.StatusFailed
// 			continue
// 		}

// 		// ✅ REAL WALLET BALANCE CHECK
// 		senderBal, err := bc.GetWalletBalance(tx.From)
// 		if err != nil {
// 			log.Printf("GetWalletBalance(%s) failed: %v", tx.From, err)
// 			tx.Status = constantset.StatusFailed
// 			continue
// 		}
// 		receiverBal, err := bc.GetWalletBalance(tx.To)
// 		if err != nil {
// 			// For a brand-new account, treat as 0
// 			receiverBal = 0
// 		}

// 		feeTokens := gasUnits * effectiveGasPrice
// 		totalCost := tx.Value + feeTokens
// 		if senderBal < totalCost {
// 			tx.Status = constantset.StatusFailed
// 			continue
// 		}

// 		// Execute state changes using the live balances; cache them in bc.Accounts
// 		senderBal -= totalCost
// 		receiverBal += tx.Value

// 		bc.Accounts[tx.From] = senderBal
// 		bc.Accounts[tx.To] = receiverBal

// 		totalGasUnits += gasUnits
// 		totalGasFeesTokens += feeTokens

// 		tx.Status = constantset.StatusSuccess
// 		processedTxs = append(processedTxs, tx)
// 	}

// 	// 🔥 Merge pending contract-call & deployment fees into gas pool
// 	if bc.PendingFeePool != nil {
// 		for _, fee := range bc.PendingFeePool {
// 			totalGasFeesTokens += fee
// 		}
// 		bc.PendingFeePool = make(map[string]uint64)
// 	}

// 	// Finalize block fields
// 	newBlock.Transactions = processedTxs
// 	for _, tx := range newBlock.Transactions {
// 		tx.Status = constantset.StatusSuccess
// 		bc.RecordRecentTx(tx)
// 	}
// 	newBlock.GasUsed = totalGasUnits                // store gas units
// 	newBlock.CurrentHash = CalculateHash(&newBlock) // hash

// 	// Append block to chain
// 	bc.Blocks = append(bc.Blocks, &newBlock)

// 	// --- PoDL Reward Distribution ---
// 	// Build a reward pool from base reward + slice of fees (in TOKENS)
// 	baseBlockReward := uint64(10)
// 	variableSlice := totalGasFeesTokens / 2

// 	// Modulate by load (based on gas UNITS consumption vs target units)
// 	targetGas := newBlock.GasLimit / 2
// 	load := 1.0
// 	if targetGas > 0 {
// 		ratio := float64(newBlock.GasUsed) / float64(targetGas)
// 		if ratio > 1.5 {
// 			ratio = 1.5
// 		}
// 		if ratio < 0.75 {
// 			ratio = 0.75
// 		}
// 		load = ratio
// 	}

// 	totalRewardPool := uint64(float64(baseBlockReward+variableSlice) * load)

// 	// Distribute
// 	leaderCut := (totalRewardPool * uint64(constantset.Leader)) / 100
// 	bc.Accounts[validator.Address] += leaderCut

// 	valMap := bc.CalculateRewardForValidator(totalRewardPool)
// 	lpMap := bc.CalculateRewardForLiquidity(totalRewardPool)

// 	dist := map[string]uint64{validator.Address: leaderCut}
// 	for addr, amt := range valMap {
// 		bc.Accounts[addr] += amt
// 		dist[addr] += amt
// 	}
// 	for addr, amt := range lpMap {
// 		bc.Accounts[addr] += amt
// 		dist[addr] += amt
// 	}

// 	// Track reward history
// 	bc.RewardHistory = append(bc.RewardHistory, RewardSnapshot{
// 		BlockNumber: newBlock.BlockNumber,
// 		BaseFee:     newBlock.BaseFee,
// 		GasUsed:     newBlock.GasUsed,
// 		Dist:        dist,
// 	})

// 	// NEW: write reward payouts as system txs so UI can show them
// 	for addr, amount := range dist {
// 		if amount == 0 {
// 			continue
// 		}

// 		bc.RecordSystemTx(
// 			"0x0000000000000000000000000000000000000000", // system / reward pool
// 			addr,
// 			amount,
// 			0, // no gas
// 			0,
// 			constantset.StatusSuccess,
// 			false, // not a contract call
// 			"blockReward",
// 			[]string{strconv.FormatUint(newBlock.BlockNumber, 10)},
// 		)

// 	}

// 	// Trim tx pool by removing only included txs
// 	if len(processedTxs) > 0 {
// 		included := make(map[string]struct{}, len(processedTxs))
// 		for _, itx := range processedTxs {
// 			included[itx.TxHash] = struct{}{}
// 		}
// 		remaining := make([]*Transaction, 0, len(bc.Transaction_pool))
// 		for _, ptx := range bc.Transaction_pool {
// 			if _, ok := included[ptx.TxHash]; !ok {
// 				remaining = append(remaining, ptx)
// 			}
// 		}
// 		bc.Transaction_pool = remaining
// 	}

// 	// Persist
// 	if err := SaveBlockToDB(&newBlock); err != nil {
// 		log.Printf("Error saving block: %v", err)
// 		return nil
// 	}

// 	log.Printf("⏱️ Block #%d mined in %v | txs=%d | gasUsed(units)=%d | gasFees(tokens)=%d | rewardPool=%d",
// 		newBlock.BlockNumber, time.Since(startTime), len(processedTxs), totalGasUnits, totalGasFeesTokens, totalRewardPool)

// 	return &newBlock
// }

// // last modified
// // func (bc *Blockchain_struct) MineNewBlock() *Block {
// // 	startTime := time.Now()
// // 	defer func() {
// // 		if r := recover(); r != nil {
// // 			log.Printf("Recovered from panic in MineNewBlock: %v", r)
// // 		}
// // 	}()

// // 	// Base fee for this block
// // 	baseFee := bc.CalculateBaseFee()

// // 	// Select proposer
// // 	validator, err := bc.SelectValidator()
// // 	if err != nil {
// // 		log.Printf("Validator selection error: %v", err)
// // 		return nil
// // 	}

// // 	// Build block shell
// // 	lastBlock := bc.Blocks[len(bc.Blocks)-1]
// // 	newBlock := NewBlock(lastBlock.BlockNumber, lastBlock.CurrentHash)
// // 	newBlock.GasLimit = bc.CalculateNextGasLimit()
// // 	newBlock.BaseFee = baseFee

// // 	// We must track gas UNITS separately from fee TOKENS
// // 	var totalGasUnits uint64 = 0
// // 	var totalGasFeesTokens uint64 = 0

// // 	processedTxs := make([]*Transaction, 0, len(bc.Transaction_pool))

// // 	// Snapshot the pool so we iterate over a stable view
// // 	txPool := make([]*Transaction, len(bc.Transaction_pool))
// // 	copy(txPool, bc.Transaction_pool)

// // 	for _, tx := range txPool {
// // 		gasUnits := tx.CalculateGasCost() // units, not tokens

// // 		// Enforce block gas cap (units)
// // 		if totalGasUnits+gasUnits > uint64(constantset.MaxBlockGas) {
// // 			break
// // 		}

// // 		// Basic validity (sig, nonce, format). Make sure VerifyTransaction
// // 		// itself does NOT reject only because of bc.Accounts balance;
// // 		// we do real-wallet balance checks below.
// // 		if !bc.VerifyTransaction(tx) {
// // 			tx.Status = constantset.StatusFailed
// // 			continue
// // 		}

// // 		// EIP-1559-ish constraint (or your fee policy):
// // 		// require the effective price to meet baseFee + tip
// // 		minRequired := tx.PriorityFee + newBlock.BaseFee
// // 		effectiveGasPrice := tx.GasPrice
// // 		if effectiveGasPrice < minRequired {
// // 			tx.Status = constantset.StatusFailed
// // 			continue
// // 		}

// // 		// ✅ REAL WALLET BALANCE CHECK (no bc.Accounts here)
// // 		senderBal, err := bc.GetWalletBalance(tx.From)
// // 		if err != nil {
// // 			log.Printf("GetWalletBalance(%s) failed: %v", tx.From, err)
// // 			tx.Status = constantset.StatusFailed
// // 			continue
// // 		}
// // 		receiverBal, err := bc.GetWalletBalance(tx.To)
// // 		if err != nil {
// // 			// For a brand-new account, treat as 0
// // 			receiverBal = 0
// // 		}

// // 		feeTokens := gasUnits * effectiveGasPrice
// // 		totalCost := tx.Value + feeTokens
// // 		if senderBal < totalCost {
// // 			tx.Status = constantset.StatusFailed
// // 			continue
// // 		}

// // 		// Execute state changes using the live balances; cache them in bc.Accounts
// // 		senderBal -= totalCost
// // 		receiverBal += tx.Value

// // 		bc.Accounts[tx.From] = senderBal
// // 		bc.Accounts[tx.To] = receiverBal

// // 		totalGasUnits += gasUnits
// // 		totalGasFeesTokens += feeTokens

// // 		tx.Status = constantset.StatusSuccess
// // 		processedTxs = append(processedTxs, tx)
// // 	}

// // 	// Finalize block fields
// // 	newBlock.Transactions = processedTxs
// // 	for _, tx := range newBlock.Transactions {
// // 		tx.Status = constantset.StatusSuccess
// // 		bc.RecordRecentTx(tx)
// // 	}
// // 	newBlock.GasUsed = totalGasUnits                // store units here
// // 	newBlock.CurrentHash = CalculateHash(&newBlock) // your hash function

// // 	// Append block to chain
// // 	bc.Blocks = append(bc.Blocks, &newBlock)

// // 	// --- PoDL Reward Distribution ---
// // 	// Build a reward pool from base reward + slice of fees (in TOKENS)
// // 	baseBlockReward := uint64(10)
// // 	variableSlice := totalGasFeesTokens / 2

// // 	// Modulate by load (based on gas UNITS consumption vs target units)
// // 	targetGas := newBlock.GasLimit / 2
// // 	load := 1.0
// // 	if targetGas > 0 {
// // 		ratio := float64(newBlock.GasUsed) / float64(targetGas)
// // 		if ratio > 1.5 {
// // 			ratio = 1.5
// // 		}
// // 		if ratio < 0.75 {
// // 			ratio = 0.75
// // 		}
// // 		load = ratio
// // 	}

// // 	totalRewardPool := uint64(float64(baseBlockReward+variableSlice) * load)

// // 	// Distribute
// // 	leaderCut := (totalRewardPool * uint64(constantset.Leader)) / 100
// // 	bc.Accounts[validator.Address] += leaderCut

// // 	valMap := bc.CalculateRewardForValidator(totalRewardPool)
// // 	lpMap := bc.CalculateRewardForLiquidity(totalRewardPool)

// // 	dist := map[string]uint64{validator.Address: leaderCut}
// // 	for addr, amt := range valMap {
// // 		bc.Accounts[addr] += amt
// // 		dist[addr] += amt
// // 	}
// // 	for addr, amt := range lpMap {
// // 		bc.Accounts[addr] += amt
// // 		dist[addr] += amt
// // 	}

// // 	// Track reward history
// // 	bc.RewardHistory = append(bc.RewardHistory, RewardSnapshot{
// // 		BlockNumber: newBlock.BlockNumber,
// // 		BaseFee:     newBlock.BaseFee,
// // 		GasUsed:     newBlock.GasUsed,
// // 		Dist:        dist,
// // 	})

// // 	// Trim tx pool by removing only included txs (don’t slice by count — order can differ)
// // 	if len(processedTxs) > 0 {
// // 		included := make(map[string]struct{}, len(processedTxs))
// // 		for _, itx := range processedTxs {
// // 			included[itx.TxHash] = struct{}{}
// // 		}
// // 		remaining := make([]*Transaction, 0, len(bc.Transaction_pool))
// // 		for _, ptx := range bc.Transaction_pool {
// // 			if _, ok := included[ptx.TxHash]; !ok {
// // 				remaining = append(remaining, ptx)
// // 			}
// // 		}
// // 		bc.Transaction_pool = remaining
// // 	}

// // 	// Persist
// // 	if err := SaveBlockToDB(&newBlock); err != nil {
// // 		log.Printf("Error saving block: %v", err)
// // 		return nil
// // 	}

// // 	log.Printf("⏱️ Block #%d mined in %v | txs=%d | gasUsed(units)=%d | gasFees(tokens)=%d | rewardPool=%d",
// // 		newBlock.BlockNumber, time.Since(startTime), len(processedTxs), totalGasUnits, totalGasFeesTokens, totalRewardPool)

// // 	return &newBlock
// // }

// // func (bc *Blockchain_struct) MineNewBlock() *Block {
// // 	startTime := time.Now()
// // 	defer func() {
// // 		if r := recover(); r != nil {
// // 			log.Printf("Recovered from panic in MineNewBlock: %v", r)
// // 		}
// // 	}()

// // 	baseFee := bc.CalculateBaseFee()

// // 	validator, err := bc.SelectValidator()
// // 	if err != nil {
// // 		log.Printf("Validator selection error: %v", err)
// // 		return nil
// // 	}

// // 	lastBlock := bc.Blocks[len(bc.Blocks)-1]
// // 	newBlock := NewBlock(lastBlock.BlockNumber, lastBlock.CurrentHash)
// // 	newBlock.GasLimit = bc.CalculateNextGasLimit()
// // 	newBlock.BaseFee = baseFee

// // 	totalGasFees := uint64(0)
// // 	processedTxs := make([]*Transaction, 0, len(bc.Transaction_pool))

// // 	for _, tx := range bc.Transaction_pool {
// // 		// Check block gas budget by gas * price, but also enforce MaxBlockGas cap
// // 		// NOTE: Your previous code compared totalGasFees+tx.Gas to MaxBlockGas (units mismatch). Fix by using gas cost tokens.
// // 		gasCost := tx.CalculateGasCost() * tx.GasPrice
// // 		if totalGasFees+gasCost > uint64(constantset.MaxBlockGas) {
// // 			break
// // 		}

// // 		if !bc.VerifyTransaction(tx) {
// // 			tx.Status = constantset.StatusFailed
// // 			continue
// // 		}

// // 		if tx.GasPrice < (baseFee + tx.PriorityFee) {
// // 			tx.Status = constantset.StatusFailed
// // 			continue
// // 		}
// // 		if bc.Accounts[tx.From] < (tx.Value + gasCost) {
// // 			tx.Status = constantset.StatusFailed
// // 			continue
// // 		}

// // 		// Execute transfer
// // 		bc.Accounts[tx.From] -= (tx.Value + gasCost)
// // 		bc.Accounts[tx.To] += tx.Value
// // 		totalGasFees += gasCost

// // 		tx.Status = constantset.StatusSuccess
// // 		processedTxs = append(processedTxs, tx)
// // 	}

// // 	newBlock.Transactions = processedTxs
// // 	newBlock.GasUsed = totalGasFees
// // 	newBlock.CurrentHash = CalculateHash(&newBlock)
// // 	bc.Blocks = append(bc.Blocks, &newBlock)

// // 	// --- PoDL Reward Distribution ---
// // 	// Base block reward + a slice of gas fees form the reward pool
// // 	baseBlockReward := uint64(10)
// // 	variableSlice := totalGasFees / 2

// // 	// Dynamic modulation (economic feedback): boost when network is hot
// // 	// factor ~ 1.0 .. 1.5 depending on last block load
// // 	targetGas := newBlock.GasLimit / 2
// // 	load := 1.0
// // 	if targetGas > 0 {
// // 		ratio := float64(newBlock.GasUsed) / float64(targetGas)
// // 		if ratio > 1.5 {
// // 			ratio = 1.5
// // 		}
// // 		if ratio < 0.75 {
// // 			ratio = 0.75
// // 		}
// // 		load = ratio
// // 	}

// // 	totalRewardPool := uint64(float64(baseBlockReward+variableSlice) * load)

// // 	// Split pool per constants
// // 	leaderCut := (totalRewardPool * uint64(constantset.Leader)) / 100
// // 	bc.Accounts[validator.Address] += leaderCut

// // 	// Validators slice
// // 	valMap := bc.CalculateRewardForValidator(totalRewardPool)
// // 	// LP slice
// // 	lpMap := bc.CalculateRewardForLiquidity(totalRewardPool)

// // 	// Apply transfers
// // 	dist := map[string]uint64{validator.Address: leaderCut}
// // 	for addr, amt := range valMap {
// // 		bc.Accounts[addr] += amt
// // 		dist[addr] += amt
// // 	}
// // 	for addr, amt := range lpMap {
// // 		bc.Accounts[addr] += amt
// // 		dist[addr] += amt
// // 	}

// // 	// Save reward snapshot
// // 	bc.RewardHistory = append(bc.RewardHistory, RewardSnapshot{
// // 		BlockNumber: newBlock.BlockNumber,
// // 		BaseFee:     newBlock.BaseFee,
// // 		GasUsed:     newBlock.GasUsed,
// // 		Dist:        dist,
// // 	})

// // 	// Trim tx pool (remove only what we included)
// // 	bc.Transaction_pool = bc.Transaction_pool[len(processedTxs):]

// // 	if err := SaveBlockToDB(&newBlock); err != nil {
// // 		log.Printf("Error saving block: %v", err)
// // 		return nil
// // 	}

// // 	log.Printf("⏱️ Block #%d mined in %v | txs=%d | Gas=%d | rewardPool=%d",
// // 		newBlock.BlockNumber, time.Since(startTime), len(processedTxs), newBlock.GasUsed, totalRewardPool)

// // 	return &newBlock
// // }

// // func (bc *Blockchain_struct) MineNewBlock() *Block {
// // 	startTime := time.Now()

// // 	defer func() {
// // 		if r := recover(); r != nil {
// // 			log.Printf("Recovered from panic in MineNewBlock: %v", r)
// // 		}
// // 	}()

// // 	baseFee := bc.CalculateBaseFee()

// // 	validator, err := bc.SelectValidator()
// // 	if err != nil {
// // 		log.Printf("Validator selection error: %v", err)
// // 		return nil
// // 	}

// // 	lastBlock := bc.Blocks[len(bc.Blocks)-1]
// // 	newBlock := NewBlock(lastBlock.BlockNumber, lastBlock.CurrentHash)
// // 	newBlock.GasLimit = bc.CalculateNextGasLimit()
// // 	newBlock.BaseFee = baseFee

// // 	// Process transactions directly without full copy
// // 	totalGasFees := uint64(0)
// // 	successfulTxs := 0
// // 	processedTxs := make([]*Transaction, 0, len(bc.Transaction_pool))

// // 	for _, tx := range bc.Transaction_pool {
// // 		// Check if block gas limit reached
// // 		if totalGasFees+tx.Gas > uint64(constantset.MaxBlockGas) {
// // 			break
// // 		}

// // 		// Validate transaction
// // 		if !bc.VerifyTransaction(tx) {
// // 			tx.Status = constantset.StatusFailed
// // 			continue
// // 		}

// // 		gasTotal := tx.CalculateGasCost() * tx.GasPrice

// // 		if tx.GasPrice < (baseFee + tx.PriorityFee) {
// // 			tx.Status = constantset.StatusFailed
// // 			continue
// // 		}

// // 		if bc.Accounts[tx.From] < (tx.Value + gasTotal) {
// // 			tx.Status = constantset.StatusFailed
// // 			continue
// // 		}

// // 		// Execute transaction
// // 		bc.Accounts[tx.From] -= (tx.Value + gasTotal)
// // 		bc.Accounts[tx.To] += tx.Value
// // 		totalGasFees += gasTotal

// // 		tx.Status = constantset.StatusSuccess
// // 		processedTxs = append(processedTxs, tx)
// // 		successfulTxs++

// // 		// Remove from pool as we process (more efficient)

// // 		// We'll handle pool cleanup at the end
// // 	}

// // 	newBlock.Transactions = processedTxs
// // 	newBlock.GasUsed = totalGasFees

// // 	// Distribute rewards
// // 	if totalGasFees > 0 {
// // 		validatorReward := totalGasFees / 2
// // 		bc.Accounts[validator.Address] += validatorReward
// // 		bc.SlashingPool += float64(validatorReward)
// // 	}

// // 	// Block reward
// // 	blockReward := uint64(10)
// // 	bc.Accounts[validator.Address] += blockReward

// // 	// Calculate hash
// // 	newBlock.CurrentHash = CalculateHash(&newBlock)

// // 	// Add to blockchain
// // 	bc.Blocks = append(bc.Blocks, &newBlock)

// // 	// Clean transaction pool - only remove processed transactions
// // 	bc.Transaction_pool = bc.Transaction_pool[len(processedTxs):]

// // 	// Optimized database save - only save new block, not entire chain
// // 	if err := SaveBlockToDB(&newBlock); err != nil {
// // 		log.Printf("Error saving block: %v", err)
// // 		return nil
// // 	}

// // 	totalTime := time.Since(startTime)
// // 	log.Printf("⏱️  Block #%d mined in %v | Txs: %d/%d | Gas: %d",
// // 		newBlock.BlockNumber, totalTime, successfulTxs, len(processedTxs), newBlock.GasUsed)

// // 	return &newBlock
// // }

// // func (bc *Blockchain_struct) MineNewBlock() *Block {

// // 	defer func() {
// // 		if r := recover(); r != nil {
// // 			log.Printf("Recovered from panic in MineNewBlock: %v", r)
// // 		}
// // 	}()

// // 	baseFee := bc.CalculateBaseFee()

// // 	if len(bc.Transaction_pool) > 10000 {
// // 		runtime.GOMAXPROCS(runtime.NumCPU() / 2)
// // 		defer runtime.GOMAXPROCS(runtime.NumCPU())
// // 	}
// // 	validator, err := bc.SelectValidator()
// // 	if err != nil {
// // 		log.Printf("Validator selection error: %v", err)
// // 		return nil
// // 	}

// // 	lastBlock := bc.Blocks[len(bc.Blocks)-1]
// // 	newBlock := NewBlock(lastBlock.BlockNumber, lastBlock.CurrentHash)
// // 	newBlock.GasLimit = bc.CalculateNextGasLimit()

// // 	newBlock.BaseFee = baseFee
// // 	newBlock.Transactions = bc.CopyTransactions()

// // 	// Parallel transaction validation
// // 	var wg sync.WaitGroup
// // 	txChan := make(chan *Transaction, len(bc.Transaction_pool))
// // 	validTxs := make([]*Transaction, 0, len(bc.Transaction_pool))

// // 	// Start worker pool
// // 	for i := 0; i < runtime.NumCPU(); i++ {
// // 		wg.Add(1)
// // 		go func() {
// // 			defer wg.Done()
// // 			for tx := range txChan {
// // 				if bc.VerifyTransaction(tx) {
// // 					validTxs = append(validTxs, tx)
// // 				}
// // 			}
// // 		}()
// // 	}

// // 	// Feed transactions to workers
// // 	for _, tx := range bc.Transaction_pool {
// // 		txChan <- tx
// // 	}
// // 	close(txChan)
// // 	wg.Wait()

// // 	newBlock.Transactions = validTxs
// // 	// Process transactions
// // 	totalGasFees := uint64(0)
// // 	for _, tx := range newBlock.Transactions {
// // 		gasTotal := tx.CalculateGasCost() * tx.GasPrice

// // 		if totalGasFees+tx.Gas > uint64(constantset.MaxBlockGas) {
// // 			break
// // 		}

// // 		if tx.GasPrice < (baseFee + tx.PriorityFee) {
// // 			tx.Status = constantset.StatusFailed
// // 			continue
// // 		}

// // 		if bc.Accounts[tx.From] < (tx.Value + gasTotal) {
// // 			tx.Status = constantset.StatusFailed
// // 			continue
// // 		}

// // 		totalGasFees += tx.Gas
// // 		newBlock.Transactions = append(newBlock.Transactions, tx)

// // 		// Deduct from sender
// // 		bc.Accounts[tx.From] -= (tx.Value + gasTotal)

// // 		// Add value to recipient
// // 		bc.Accounts[tx.To] += tx.Value

// // 		// Collect gas fees
// // 		totalGasFees += gasTotal

// // 		tx.Status = constantset.StatusSuccess
// // 		log.Println("tx is succcess", tx.Status)
// // 	}

// // 	// Distribute rewards
// // 	if totalGasFees > 0 {
// // 		// 50% to validator, 50% to slashing pool
// // 		validatorReward := totalGasFees / 2
// // 		bc.Accounts[validator.Address] += validatorReward
// // 		bc.SlashingPool += float64(validatorReward)
// // 	}

// // 	// Block reward (fixed amount)
// // 	blockReward := uint64(10)
// // 	bc.Accounts[validator.Address] += blockReward
// // 	newBlock.GasUsed = totalGasFees
// // 	newBlock.CurrentHash = CalculateHash(&newBlock)
// // 	bc.Blocks = append(bc.Blocks, &newBlock)
// // 	bc.Transaction_pool = []*Transaction{}
// // 	dbCopy := *bc
// // 	dbCopy.Mutex = sync.Mutex{}
// // 	if err := PutIntoDB(dbCopy); err != nil {
// // 		log.Printf("Error saving block: %v", err)
// // 		return nil
// // 	}

// // 	return &newBlock
// // }

// //	func (bc *Blockchain_struct) MineNewBlock() *Block {
// //		validator, err := bc.SelectValidator()
// //		if err != nil {
// //			return nil
// //		}
// //		lastBlock := bc.Blocks[len(bc.Blocks)-1]
// //		newBlock := NewBlock(lastBlock.BlockNumber, lastBlock.CurrentHash)
// //		newBlock.Transactions = bc.CopyTransactions()
// //		//fmt.Println("here is newblock.transaction", newBlock.Transactions)
// //		// blockHash := CalculateHash(&newBlock)
// //		// signature, err := validator.SignMessage([]byte(blockHash))
// //		// if err == nil {
// //		// 	newBlock.ValidatorSignature = hex.EncodeToString(signature)
// //		// }
// //		for _, tx := range newBlock.Transactions {
// //			gasTotal := tx.CalculateGasCost() * tx.GasPrice
// //			if bc.Accounts[tx.From] < (tx.Value + gasTotal) {
// //				tx.Status = constantset.StatusFailed
// //				continue
// //			}
// //			bc.Accounts[tx.From] -= (tx.Value + gasTotal)
// //			bc.Accounts[validator.Address] += gasTotal / 2
// //			bc.SlashingPool += float64(gasTotal / 2)
// //			bc.Accounts[tx.To] += tx.Value
// //			tx.Status = constantset.StatusSuccess
// //		}
// //		newBlock.CurrentHash = CalculateHash(&newBlock)
// //		// Reward validator
// //		bc.Accounts[validator.Address] += 10
// //			bc.Blocks = append(bc.Blocks, &newBlock)
// //			bc.Transaction_pool = []*Transaction{}
// //			err = PutIntoDB(*bc)
// //			if err != nil {
// //				log.Println("Error putting new block into DB:", err)
// //				return nil
// //			}
// //			return &newBlock
// //		}
// func CalculateHash(newBlock *Block) string {

// 	data, _ := json.Marshal(newBlock)
// 	hash := sha256.Sum256(data)
// 	HexRePresent := hex.EncodeToString(hash[:32])
// 	formatedToHex := constantset.BlockHexPrefix + HexRePresent

// 	return formatedToHex

// }

// func ToJsonBlock(genesisBlock Block) string {
// 	nBlock := genesisBlock
// 	block, err := json.Marshal(nBlock)
// 	if err != nil {
// 		log.Println("error")
// 	}
// 	return string(block)
// }

package blockchaincomponent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"runtime"
	"strconv"
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

type VerifiedTx struct {
	Tx      *Transaction
	GasUsed uint64
	Fee     uint64
	Valid   bool
	Err     error
}

// -------- WORKER POOL CONFIG --------
const TxWorkers = 8 // set to runtime.NumCPU() if you want full speed

// -------- WORKER POOL --------
// Worker uses ONLY in-memory accounts for speed.
// No DB calls here.
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

func (bc *Blockchain_struct) MineNewBlock() *Block {
	start := time.Now()

	if len(bc.Blocks) == 0 {
		return nil
	}

	lastBlock := bc.Blocks[len(bc.Blocks)-1]
	baseFee := bc.CalculateBaseFee()

	//------------------------------------------
	// 🆕 Create next block shell
	//------------------------------------------
	newBlock := NewBlock(lastBlock.BlockNumber, lastBlock.CurrentHash)
	newBlock.GasLimit = bc.CalculateNextGasLimit()
	newBlock.BaseFee = baseFee

	//------------------------------------------
	// 🧑‍⚖️ Select validator
	//------------------------------------------
	validator, err := bc.SelectValidator()
	if err != nil {
		log.Printf("SelectValidator error: %v", err)
		return nil
	}

	//------------------------------------------
	// 🧵 Worker pool for parallel tx validation
	//------------------------------------------
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

	//------------------------------------------
	// ✔ Collect verified txs & apply state
	//------------------------------------------
	var totalGasUsed uint64
	var totalFees uint64

	accepted := make([]*Transaction, 0, len(txPool))

	for res := range resultChan {
		if !res.Valid {
			if res.Tx != nil {
				res.Tx.Status = constantset.StatusFailed
			}
			continue
		}

		// block gas cap
		if totalGasUsed+res.GasUsed > newBlock.GasLimit {
			res.Tx.Status = constantset.StatusFailed
			continue
		}

		// state application (balances)
		sBal := bc.Accounts[res.Tx.From]
		rBal := bc.Accounts[res.Tx.To]
		totalCost := res.Tx.Value + res.Fee

		if sBal < totalCost {
			res.Tx.Status = constantset.StatusFailed
			continue
		}

		sBal -= totalCost
		rBal += res.Tx.Value

		bc.Accounts[res.Tx.From] = sBal
		bc.Accounts[res.Tx.To] = rBal

		res.Tx.Status = constantset.StatusSuccess
		accepted = append(accepted, res.Tx)

		totalGasUsed += res.GasUsed
		totalFees += res.Fee

		bc.RecordRecentTx(res.Tx)
	}

	newBlock.Transactions = accepted
	newBlock.GasUsed = totalGasUsed
	newBlock.CurrentHash = CalculateHash(&newBlock)

	//------------------------------------------
	// 🔥 Execute smart contract calls
	//------------------------------------------
	for _, tx := range accepted {
		if tx.IsContract {
			_, err := bc.ContractEngine.Pipeline.ExecuteContractTx(
				tx,
				newBlock.BlockNumber,
			)

			if err != nil {
				log.Printf("❌ Contract exec failed for %s: %v", tx.TxHash, err)
				tx.Status = constantset.StatusFailed
				continue
			}

			tx.Status = constantset.StatusSuccess

			// NOTE: event storage already happens inside ExecuteContractTx
		}
	}

	//------------------------------------------
	// ⛓ Append block to chain
	//------------------------------------------
	bc.Blocks = append(bc.Blocks, &newBlock)

	//------------------------------------------
	// 💰 Distribute validator reward (PoDL)
	//------------------------------------------
	baseReward := uint64(10)
	variable := totalFees / 2
	totalReward := baseReward + variable

	bc.Accounts[validator.Address] += totalReward

	dist := map[string]uint64{
		validator.Address: totalReward,
	}

	bc.RewardHistory = append(bc.RewardHistory, RewardSnapshot{
		BlockNumber: newBlock.BlockNumber,
		BaseFee:     newBlock.BaseFee,
		GasUsed:     newBlock.GasUsed,
		Dist:        dist,
	})

	//------------------------------------------
	// 🔵 Record reward as system tx
	//------------------------------------------
	const rewardPoolAddr = "0x0000000000000000000000000000000000000000"

	rewardTx := bc.RecordSystemTx(
		rewardPoolAddr,
		validator.Address,
		totalReward,
		0,
		0,
		constantset.StatusSuccess,
		false,
		"blockReward",
		[]string{strconv.FormatUint(newBlock.BlockNumber, 10)},
	)

	newBlock.Transactions = append(newBlock.Transactions, rewardTx)

	//------------------------------------------
	// 🧹 Mempool cleanup
	//------------------------------------------
	if len(accepted) > 0 {
		used := make(map[string]struct{})
		for _, t := range accepted {
			used[t.TxHash] = struct{}{}
		}

		rest := make([]*Transaction, 0, len(txPool))
		for _, t := range txPool {
			if _, ok := used[t.TxHash]; !ok {
				rest = append(rest, t)
			}
		}

		bc.Transaction_pool = rest
	}

	//------------------------------------------
	// 💾 Persist block
	//------------------------------------------
	if err := bc.SaveBlockToDB(&newBlock); err != nil {
		log.Printf("❌ Error saving block: %v", err)
	}

	bc.LastBlockMiningTime = time.Since(start)

	log.Printf(
		"⏱️ Block #%d mined in %v | tx=%d | gas=%d",
		newBlock.BlockNumber,
		bc.LastBlockMiningTime,
		len(accepted),
		totalGasUsed,
	)

	return &newBlock
}

// this is 23 nov 13:30
// func (bc *Blockchain_struct) MineNewBlock() *Block {
// 	start := time.Now()

// 	if len(bc.Blocks) == 0 {
// 		return nil
// 	}

// 	lastBlock := bc.Blocks[len(bc.Blocks)-1]
// 	baseFee := bc.CalculateBaseFee()

// 	newBlock := NewBlock(lastBlock.BlockNumber, lastBlock.CurrentHash)
// 	newBlock.GasLimit = bc.CalculateNextGasLimit()
// 	newBlock.BaseFee = baseFee

// 	validator, err := bc.SelectValidator()
// 	if err != nil {
// 		log.Printf("SelectValidator error: %v", err)
// 		return nil
// 	}

// 	// --- Worker pool setup ---
// 	txPool := bc.Transaction_pool
// 	taskChan := make(chan *Transaction, len(txPool))
// 	resultChan := make(chan VerifiedTx, len(txPool))

// 	workers := TxWorkers
// 	if workers > runtime.NumCPU() {
// 		workers = runtime.NumCPU()
// 	}

// 	var wg sync.WaitGroup
// 	wg.Add(workers)
// 	for i := 0; i < workers; i++ {
// 		go func() {
// 			bc.verifyTxWorker(taskChan, resultChan, baseFee)
// 			wg.Done()
// 		}()
// 	}

// 	for _, tx := range txPool {
// 		taskChan <- tx
// 	}
// 	close(taskChan)

// 	go func() {
// 		wg.Wait()
// 		close(resultChan)
// 	}()

// 	// --- Collect & apply state (still single-threaded = safe) ---
// 	var totalGasUsed uint64
// 	var totalFees uint64

// 	accepted := make([]*Transaction, 0, len(txPool))

// 	for res := range resultChan {
// 		if !res.Valid {
// 			if res.Tx != nil {
// 				res.Tx.Status = constantset.StatusFailed
// 			}
// 			continue
// 		}

// 		// enforce dynamic block gas cap
// 		if totalGasUsed+res.GasUsed > newBlock.GasLimit {
// 			res.Tx.Status = constantset.StatusFailed
// 			continue
// 		}

// 		// Apply state using in-memory balances only
// 		sBal := bc.Accounts[res.Tx.From]
// 		rBal := bc.Accounts[res.Tx.To]

// 		totalCost := res.Tx.Value + res.Fee
// 		if sBal < totalCost {
// 			// can happen if multiple tx from same sender race
// 			res.Tx.Status = constantset.StatusFailed
// 			continue
// 		}

// 		sBal -= totalCost
// 		rBal += res.Tx.Value

// 		bc.Accounts[res.Tx.From] = sBal
// 		bc.Accounts[res.Tx.To] = rBal

// 		res.Tx.Status = constantset.StatusSuccess
// 		accepted = append(accepted, res.Tx)

// 		totalGasUsed += res.GasUsed
// 		totalFees += res.Fee

// 		// optional: track recent tx
// 		bc.RecordRecentTx(res.Tx)
// 	}

// 	newBlock.Transactions = accepted
// 	newBlock.GasUsed = totalGasUsed
// 	newBlock.CurrentHash = CalculateHash(&newBlock)

// 	bc.Blocks = append(bc.Blocks, &newBlock)

// 	// 🔥 Execute contract calls inside block

// 	// --- Rewards (keep your logic, simplified here) ---
// 	baseBlockReward := uint64(10)
// 	variableSlice := totalFees / 2
// 	totalRewardPool := baseBlockReward + variableSlice

// 	bc.Accounts[validator.Address] += totalRewardPool

// 	dist := map[string]uint64{
// 		validator.Address: totalRewardPool,
// 	}

// 	bc.RewardHistory = append(bc.RewardHistory, RewardSnapshot{
// 		BlockNumber: newBlock.BlockNumber,
// 		BaseFee:     newBlock.BaseFee,
// 		GasUsed:     newBlock.GasUsed,
// 		Dist:        dist,
// 	})

// 	const rewardPoolAddr = "0x0000000000000000000000000000000000000000"
// 	tx := bc.RecordSystemTx(
// 		rewardPoolAddr,
// 		validator.Address,
// 		totalRewardPool,
// 		0,
// 		0,
// 		constantset.StatusSuccess,
// 		false,
// 		"blockReward",
// 		[]string{strconv.FormatUint(newBlock.BlockNumber, 10)},
// 	)
// 	newBlock.Transactions = append(newBlock.Transactions, tx)

// 	// --- Fast mempool cleanup ---
// 	if len(accepted) > 0 {
// 		included := make(map[string]struct{}, len(accepted))
// 		for _, tx := range accepted {
// 			included[tx.TxHash] = struct{}{}
// 		}

// 		remaining := make([]*Transaction, 0, len(txPool)-len(accepted))
// 		for _, tx := range txPool {
// 			if _, ok := included[tx.TxHash]; !ok {
// 				remaining = append(remaining, tx)
// 			}
// 		}
// 		bc.Transaction_pool = remaining
// 	}

// 	if err := bc.SaveBlockToDB(&newBlock); err != nil {
// 		log.Printf("Error saving block: %v", err)
// 	}
// 	bc.LastBlockMiningTime = time.Since(start)

// 	log.Printf(
// 		"⏱️ Optimized C block #%d mined in %v | tx=%d | gas=%d",
// 		newBlock.BlockNumber, bc.LastBlockMiningTime,
// 		len(accepted), totalGasUsed,
// 	)

// 	return &newBlock
// }

// this is at 15:08
// func (bc *Blockchain_struct) MineNewBlock() *Block {
// 	start := time.Now()

// 	lastBlock := bc.Blocks[len(bc.Blocks)-1]
// 	baseFee := bc.CalculateBaseFee()

// 	newBlock := NewBlock(lastBlock.BlockNumber, lastBlock.CurrentHash)
// 	newBlock.GasLimit = bc.CalculateNextGasLimit()
// 	newBlock.BaseFee = baseFee

// 	validator, _ := bc.SelectValidator()

// 	// --- WORKER POOL SETUP ---
// 	taskChan := make(chan *Transaction, len(bc.Transaction_pool))
// 	resultChan := make(chan VerifiedTx, len(bc.Transaction_pool))

// 	workers := runtime.NumCPU()
// 	var wg sync.WaitGroup
// 	wg.Add(workers)

// 	for i := 0; i < workers; i++ {
// 		go func() {
// 			bc.verifyTxWorker(taskChan, resultChan, baseFee)
// 			wg.Done()
// 		}()
// 	}

// 	for _, tx := range bc.Transaction_pool {
// 		taskChan <- tx
// 	}
// 	close(taskChan)

// 	go func() {
// 		wg.Wait()
// 		close(resultChan)
// 	}()

// 	// --- SHARDED STATE BUFFER ---
// 	stateCommit := NewStateCommit()
// 	accepted := make([]*Transaction, 0, len(bc.Transaction_pool))

// 	var totalGasUsed uint64
// 	var totalFees uint64

// 	// --- COLLECT RESULTS ---
// 	for res := range resultChan {
// 		tx := res.Tx
// 		if !res.Valid {
// 			tx.Status = constantset.StatusFailed
// 			continue
// 		}

// 		if totalGasUsed+res.GasUsed > uint64(constantset.MaxBlockGas) {
// 			tx.Status = constantset.StatusFailed
// 			continue
// 		}

// 		// read actual balances only once
// 		sBal, _ := bc.GetWalletBalance(tx.From)
// 		rBal, _ := bc.GetWalletBalance(tx.To)

// 		// anti–double-spend:
// 		// accumulate balance inside shard commit
// 		sBal -= (tx.Value + res.Fee)
// 		rBal += tx.Value

// 		stateCommit.AddBalance(tx.From, sBal)
// 		stateCommit.AddBalance(tx.To, rBal)

// 		tx.Status = constantset.StatusSuccess

// 		accepted = append(accepted, tx)
// 		totalGasUsed += res.GasUsed
// 		totalFees += res.Fee
// 	}

// 	// --- APPLY SHARDED STATE IN PARALLEL ---
// 	var wgState sync.WaitGroup
// 	wgState.Add(StateShardCount)

// 	for i := 0; i < StateShardCount; i++ {
// 		go func(idx int) {
// 			sh := stateCommit.shards[idx]

// 			sh.mu.Lock()
// 			for addr, bal := range sh.data {
// 				bc.Accounts[addr] = bal
// 			}
// 			sh.mu.Unlock()

// 			wgState.Done()
// 		}(i)
// 	}

// 	wgState.Wait()

// 	// --- BLOCK FINALIZATION ---
// 	newBlock.Transactions = accepted
// 	newBlock.GasUsed = totalGasUsed
// 	newBlock.CurrentHash = CalculateHash(&newBlock)

// 	bc.Blocks = append(bc.Blocks, &newBlock)

// 	// --- REWARD LOGIC (unchanged) ---
// 	baseReward := uint64(10)
// 	variable := totalFees / 2
// 	totalRewardPool := baseReward + variable
// 	bc.Accounts[validator.Address] += totalRewardPool

// 	bc.RecordSystemTx(
// 		"0x0000000000000000000000000000000000000000",
// 		validator.Address,
// 		totalRewardPool,
// 		0, 0,
// 		constantset.StatusSuccess,
// 		false, "blockReward",
// 		[]string{strconv.FormatUint(newBlock.BlockNumber, 10)},
// 	)

// 	// --- FAST MEMPOOL CLEANUP ---
// 	included := make(map[string]bool, len(accepted))
// 	for _, tx := range accepted {
// 		included[tx.TxHash] = true
// 	}
// 	left := bc.Transaction_pool[:0]
// 	for _, tx := range bc.Transaction_pool {
// 		if !included[tx.TxHash] {
// 			left = append(left, tx)
// 		}
// 	}
// 	bc.Transaction_pool = left

// 	// --- DB BATCH WRITE ---
// 	SaveBlockToDB(&newBlock)

// 	bc.LastBlockMiningTime = time.Since(start)

// 	log.Printf(
// 		"⚡ OptimizedC block #%d | tx=%d | time=%v | gas=%d",
// 		newBlock.BlockNumber, len(accepted),
// 		bc.LastBlockMiningTime, totalGasUsed,
// 	)

// 	return &newBlock
// }

// this is the last modified one at 22 14:13
// func (bc *Blockchain_struct) MineNewBlock() *Block {
// 	startTime := time.Now()
// 	defer func() {
// 		if r := recover(); r != nil {
// 			log.Printf("Recovered from panic in MineNewBlock: %v", r)
// 		}
// 	}()

// 	// Base fee for this block
// 	baseFee := bc.CalculateBaseFee()

// 	if len(bc.Transaction_pool) > 10000 {
// 		runtime.GOMAXPROCS(runtime.NumCPU() / 2)
// 		defer runtime.GOMAXPROCS(runtime.NumCPU())
// 	}

// 	// Select proposer
// 	validator, err := bc.SelectValidator()
// 	if err != nil {
// 		log.Printf("Validator selection error: %v", err)
// 		return nil
// 	}

// 	// Build block shell
// 	lastBlock := bc.Blocks[len(bc.Blocks)-1]
// 	newBlock := NewBlock(lastBlock.BlockNumber, lastBlock.CurrentHash)
// 	newBlock.GasLimit = bc.CalculateNextGasLimit()
// 	newBlock.BaseFee = baseFee

// 	// Track gas UNITS separately from fee TOKENS
// 	var totalGasUnits uint64 = 0
// 	var totalGasFeesTokens uint64 = 0

// 	processedTxs := make([]*Transaction, 0, len(bc.Transaction_pool))

// 	// Snapshot the pool so we iterate over a stable view
// 	txPool := make([]*Transaction, len(bc.Transaction_pool))
// 	copy(txPool, bc.Transaction_pool)

// 	for _, tx := range txPool {
// 		gasUnits := tx.CalculateGasCost() // units, not tokens

// 		// Enforce block gas cap (units)
// 		if totalGasUnits+gasUnits > uint64(constantset.MaxBlockGas) {
// 			break
// 		}

// 		// Basic validity (sig, nonce, format)
// 		if !bc.VerifyTransaction(tx) {
// 			tx.Status = constantset.StatusFailed
// 			continue
// 		}

// 		// EIP-1559-ish constraint: require gasPrice >= baseFee + tip
// 		minRequired := tx.PriorityFee + newBlock.BaseFee
// 		effectiveGasPrice := tx.GasPrice
// 		if effectiveGasPrice < minRequired {
// 			tx.Status = constantset.StatusFailed
// 			continue
// 		}

// 		// ✅ REAL WALLET BALANCE CHECK
// 		senderBal, err := bc.GetWalletBalance(tx.From)
// 		if err != nil {
// 			log.Printf("GetWalletBalance(%s) failed: %v", tx.From, err)
// 			tx.Status = constantset.StatusFailed
// 			continue
// 		}
// 		receiverBal, err := bc.GetWalletBalance(tx.To)
// 		if err != nil {
// 			// For a brand-new account, treat as 0
// 			receiverBal = 0
// 		}

// 		feeTokens := gasUnits * effectiveGasPrice
// 		totalCost := tx.Value + feeTokens
// 		if senderBal < totalCost {
// 			tx.Status = constantset.StatusFailed
// 			continue
// 		}

// 		// Execute state changes using the live balances; cache them in bc.Accounts
// 		senderBal -= totalCost
// 		receiverBal += tx.Value

// 		bc.Accounts[tx.From] = senderBal
// 		bc.Accounts[tx.To] = receiverBal

// 		totalGasUnits += gasUnits
// 		totalGasFeesTokens += feeTokens

// 		tx.Status = constantset.StatusSuccess
// 		processedTxs = append(processedTxs, tx)
// 	}

// 	// 🔥 Merge pending contract-call & deployment fees into gas pool
// 	if bc.PendingFeePool != nil {
// 		for _, fee := range bc.PendingFeePool {
// 			totalGasFeesTokens += fee
// 		}
// 		bc.PendingFeePool = make(map[string]uint64)
// 	}

// 	// Finalize block fields
// 	newBlock.Transactions = processedTxs
// 	for _, tx := range newBlock.Transactions {
// 		tx.Status = constantset.StatusSuccess
// 		bc.RecordRecentTx(tx)
// 	}
// 	newBlock.GasUsed = totalGasUnits                // store gas units
// 	newBlock.CurrentHash = CalculateHash(&newBlock) // hash

// 	// Append block to chain
// 	bc.Blocks = append(bc.Blocks, &newBlock)

// 	// --- PoDL Reward Distribution ---
// 	// Build a reward pool from base reward + slice of fees (in TOKENS)
// 	baseBlockReward := uint64(10)
// 	variableSlice := totalGasFeesTokens / 2

// 	// Modulate by load (based on gas UNITS consumption vs target units)
// 	targetGas := newBlock.GasLimit / 2
// 	load := 1.0
// 	if targetGas > 0 {
// 		ratio := float64(newBlock.GasUsed) / float64(targetGas)
// 		if ratio > 1.5 {
// 			ratio = 1.5
// 		}
// 		if ratio < 0.75 {
// 			ratio = 0.75
// 		}
// 		load = ratio
// 	}

// 	totalRewardPool := uint64(float64(baseBlockReward+variableSlice) * load)

// 	// Distribute
// 	leaderCut := (totalRewardPool * uint64(constantset.Leader)) / 100
// 	bc.Accounts[validator.Address] += leaderCut

// 	valMap := bc.CalculateRewardForValidator(totalRewardPool)
// 	lpMap := bc.CalculateRewardForLiquidity(totalRewardPool)

// 	// dist collects the final per-address reward (leader + validator + LP)
// 	dist := map[string]uint64{validator.Address: leaderCut}
// 	for addr, amt := range valMap {
// 		if amt == 0 {
// 			continue
// 		}
// 		bc.Accounts[addr] += amt
// 		dist[addr] += amt
// 	}
// 	for addr, amt := range lpMap {
// 		if amt == 0 {
// 			continue
// 		}
// 		bc.Accounts[addr] += amt
// 		dist[addr] += amt
// 	}

// 	// Track reward history
// 	bc.RewardHistory = append(bc.RewardHistory, RewardSnapshot{
// 		BlockNumber: newBlock.BlockNumber,
// 		BaseFee:     newBlock.BaseFee,
// 		GasUsed:     newBlock.GasUsed,
// 		Dist:        dist,
// 	})

// 	// --- NEW: Write reward payouts as REAL block txs ---
// 	// NOTE: we ALREADY updated bc.Accounts above, so here we only
// 	// materialize tx objects & push to block / recent-tx history.
// 	const rewardPoolAddr = "0x0000000000000000000000000000000000000000"

// 	for addr, amount := range dist {
// 		if amount == 0 {
// 			continue
// 		}

// 		// Use helper so TxHash + timestamps + recent-tx are handled in one place
// 		tx := bc.RecordSystemTx(
// 			rewardPoolAddr, // system / reward pool
// 			addr,
// 			amount,
// 			0, // gasUsed
// 			0, // gasPrice
// 			constantset.StatusSuccess,
// 			false, // not a contract call
// 			"blockReward",
// 			[]string{strconv.FormatUint(newBlock.BlockNumber, 10)},
// 		)

// 		// Attach to block so explorer can display them as mined txs
// 		newBlock.Transactions = append(newBlock.Transactions, tx)
// 	}

// 	// Trim tx pool by removing only included txs
// 	if len(processedTxs) > 0 {
// 		included := make(map[string]struct{}, len(processedTxs))
// 		for _, itx := range processedTxs {
// 			included[itx.TxHash] = struct{}{}
// 		}
// 		remaining := make([]*Transaction, 0, len(bc.Transaction_pool))
// 		for _, ptx := range bc.Transaction_pool {
// 			if _, ok := included[ptx.TxHash]; !ok {
// 				remaining = append(remaining, ptx)
// 			}
// 		}
// 		bc.Transaction_pool = remaining
// 	}

// 	// Persist
// 	if err := SaveBlockToDB(&newBlock); err != nil {
// 		log.Printf("Error saving block: %v", err)
// 		return nil
// 	}

// 	bc.LastBlockMiningTime = time.Since(startTime)

// 	log.Printf("⏱️ Block #%d mined in %v | txs=%d | gasUsed(units)=%d | gasFees(tokens)=%d | rewardPool=%d",
// 		newBlock.BlockNumber, time.Since(startTime), len(newBlock.Transactions), totalGasUnits, totalGasFeesTokens, totalRewardPool)

// 	return &newBlock
// }

// func (bc *Blockchain_struct) MineNewBlock() *Block {
// 	startTime := time.Now()
// 	defer func() {
// 		if r := recover(); r != nil {
// 			log.Printf("Recovered from panic in MineNewBlock: %v", r)
// 		}
// 	}()

// 	// Base fee for this block
// 	baseFee := bc.CalculateBaseFee()

// 	if len(bc.Transaction_pool) > 10000 {
// 		runtime.GOMAXPROCS(runtime.NumCPU() / 2)
// 		defer runtime.GOMAXPROCS(runtime.NumCPU())
// 	}
// 	// Select proposer
// 	validator, err := bc.SelectValidator()
// 	if err != nil {
// 		log.Printf("Validator selection error: %v", err)
// 		return nil
// 	}

// 	// Build block shell
// 	lastBlock := bc.Blocks[len(bc.Blocks)-1]
// 	newBlock := NewBlock(lastBlock.BlockNumber, lastBlock.CurrentHash)
// 	newBlock.GasLimit = bc.CalculateNextGasLimit()
// 	newBlock.BaseFee = baseFee

// 	// Track gas UNITS separately from fee TOKENS
// 	var totalGasUnits uint64 = 0
// 	var totalGasFeesTokens uint64 = 0

// 	processedTxs := make([]*Transaction, 0, len(bc.Transaction_pool))

// 	// Snapshot the pool so we iterate over a stable view
// 	txPool := make([]*Transaction, len(bc.Transaction_pool))
// 	copy(txPool, bc.Transaction_pool)

// 	for _, tx := range txPool {
// 		gasUnits := tx.CalculateGasCost() // units, not tokens

// 		// Enforce block gas cap (units)
// 		if totalGasUnits+gasUnits > uint64(constantset.MaxBlockGas) {
// 			break
// 		}

// 		// Basic validity (sig, nonce, format)
// 		if !bc.VerifyTransaction(tx) {
// 			tx.Status = constantset.StatusFailed
// 			continue
// 		}

// 		// EIP-1559-ish constraint: require gasPrice >= baseFee + tip
// 		minRequired := tx.PriorityFee + newBlock.BaseFee
// 		effectiveGasPrice := tx.GasPrice
// 		if effectiveGasPrice < minRequired {
// 			tx.Status = constantset.StatusFailed
// 			continue
// 		}

// 		// ✅ REAL WALLET BALANCE CHECK
// 		senderBal, err := bc.GetWalletBalance(tx.From)
// 		if err != nil {
// 			log.Printf("GetWalletBalance(%s) failed: %v", tx.From, err)
// 			tx.Status = constantset.StatusFailed
// 			continue
// 		}
// 		receiverBal, err := bc.GetWalletBalance(tx.To)
// 		if err != nil {
// 			// For a brand-new account, treat as 0
// 			receiverBal = 0
// 		}

// 		feeTokens := gasUnits * effectiveGasPrice
// 		totalCost := tx.Value + feeTokens
// 		if senderBal < totalCost {
// 			tx.Status = constantset.StatusFailed
// 			continue
// 		}

// 		// Execute state changes using the live balances; cache them in bc.Accounts
// 		senderBal -= totalCost
// 		receiverBal += tx.Value

// 		bc.Accounts[tx.From] = senderBal
// 		bc.Accounts[tx.To] = receiverBal

// 		totalGasUnits += gasUnits
// 		totalGasFeesTokens += feeTokens

// 		tx.Status = constantset.StatusSuccess
// 		processedTxs = append(processedTxs, tx)
// 	}

// 	// 🔥 Merge pending contract-call & deployment fees into gas pool
// 	if bc.PendingFeePool != nil {
// 		for _, fee := range bc.PendingFeePool {
// 			totalGasFeesTokens += fee
// 		}
// 		bc.PendingFeePool = make(map[string]uint64)
// 	}

// 	// Finalize block fields
// 	newBlock.Transactions = processedTxs
// 	for _, tx := range newBlock.Transactions {
// 		tx.Status = constantset.StatusSuccess
// 		bc.RecordRecentTx(tx)
// 	}
// 	newBlock.GasUsed = totalGasUnits                // store gas units
// 	newBlock.CurrentHash = CalculateHash(&newBlock) // hash

// 	// Append block to chain
// 	bc.Blocks = append(bc.Blocks, &newBlock)

// 	// --- PoDL Reward Distribution ---
// 	// Build a reward pool from base reward + slice of fees (in TOKENS)
// 	baseBlockReward := uint64(10)
// 	variableSlice := totalGasFeesTokens / 2

// 	// Modulate by load (based on gas UNITS consumption vs target units)
// 	targetGas := newBlock.GasLimit / 2
// 	load := 1.0
// 	if targetGas > 0 {
// 		ratio := float64(newBlock.GasUsed) / float64(targetGas)
// 		if ratio > 1.5 {
// 			ratio = 1.5
// 		}
// 		if ratio < 0.75 {
// 			ratio = 0.75
// 		}
// 		load = ratio
// 	}

// 	totalRewardPool := uint64(float64(baseBlockReward+variableSlice) * load)

// 	// Distribute
// 	leaderCut := (totalRewardPool * uint64(constantset.Leader)) / 100
// 	bc.Accounts[validator.Address] += leaderCut

// 	valMap := bc.CalculateRewardForValidator(totalRewardPool)
// 	lpMap := bc.CalculateRewardForLiquidity(totalRewardPool)

// 	dist := map[string]uint64{validator.Address: leaderCut}
// 	for addr, amt := range valMap {
// 		bc.Accounts[addr] += amt
// 		dist[addr] += amt
// 	}
// 	for addr, amt := range lpMap {
// 		bc.Accounts[addr] += amt
// 		dist[addr] += amt
// 	}

// 	// Track reward history
// 	bc.RewardHistory = append(bc.RewardHistory, RewardSnapshot{
// 		BlockNumber: newBlock.BlockNumber,
// 		BaseFee:     newBlock.BaseFee,
// 		GasUsed:     newBlock.GasUsed,
// 		Dist:        dist,
// 	})

// 	// NEW: write reward payouts as system txs so UI can show them
// 	for addr, amount := range dist {
// 		if amount == 0 {
// 			continue
// 		}

// 		bc.RecordSystemTx(
// 			"0x0000000000000000000000000000000000000000", // system / reward pool
// 			addr,
// 			amount,
// 			0, // no gas
// 			0,
// 			constantset.StatusSuccess,
// 			false, // not a contract call
// 			"blockReward",
// 			[]string{strconv.FormatUint(newBlock.BlockNumber, 10)},
// 		)

// 	}

// 	// Trim tx pool by removing only included txs
// 	if len(processedTxs) > 0 {
// 		included := make(map[string]struct{}, len(processedTxs))
// 		for _, itx := range processedTxs {
// 			included[itx.TxHash] = struct{}{}
// 		}
// 		remaining := make([]*Transaction, 0, len(bc.Transaction_pool))
// 		for _, ptx := range bc.Transaction_pool {
// 			if _, ok := included[ptx.TxHash]; !ok {
// 				remaining = append(remaining, ptx)
// 			}
// 		}
// 		bc.Transaction_pool = remaining
// 	}

// 	// Persist
// 	if err := SaveBlockToDB(&newBlock); err != nil {
// 		log.Printf("Error saving block: %v", err)
// 		return nil
// 	}

// 	log.Printf("⏱️ Block #%d mined in %v | txs=%d | gasUsed(units)=%d | gasFees(tokens)=%d | rewardPool=%d",
// 		newBlock.BlockNumber, time.Since(startTime), len(processedTxs), totalGasUnits, totalGasFeesTokens, totalRewardPool)

// 	return &newBlock
// }

//last modified
// func (bc *Blockchain_struct) MineNewBlock() *Block {
// 	startTime := time.Now()
// 	defer func() {
// 		if r := recover(); r != nil {
// 			log.Printf("Recovered from panic in MineNewBlock: %v", r)
// 		}
// 	}()

// 	// Base fee for this block
// 	baseFee := bc.CalculateBaseFee()

// 	// Select proposer
// 	validator, err := bc.SelectValidator()
// 	if err != nil {
// 		log.Printf("Validator selection error: %v", err)
// 		return nil
// 	}

// 	// Build block shell
// 	lastBlock := bc.Blocks[len(bc.Blocks)-1]
// 	newBlock := NewBlock(lastBlock.BlockNumber, lastBlock.CurrentHash)
// 	newBlock.GasLimit = bc.CalculateNextGasLimit()
// 	newBlock.BaseFee = baseFee

// 	// We must track gas UNITS separately from fee TOKENS
// 	var totalGasUnits uint64 = 0
// 	var totalGasFeesTokens uint64 = 0

// 	processedTxs := make([]*Transaction, 0, len(bc.Transaction_pool))

// 	// Snapshot the pool so we iterate over a stable view
// 	txPool := make([]*Transaction, len(bc.Transaction_pool))
// 	copy(txPool, bc.Transaction_pool)

// 	for _, tx := range txPool {
// 		gasUnits := tx.CalculateGasCost() // units, not tokens

// 		// Enforce block gas cap (units)
// 		if totalGasUnits+gasUnits > uint64(constantset.MaxBlockGas) {
// 			break
// 		}

// 		// Basic validity (sig, nonce, format). Make sure VerifyTransaction
// 		// itself does NOT reject only because of bc.Accounts balance;
// 		// we do real-wallet balance checks below.
// 		if !bc.VerifyTransaction(tx) {
// 			tx.Status = constantset.StatusFailed
// 			continue
// 		}

// 		// EIP-1559-ish constraint (or your fee policy):
// 		// require the effective price to meet baseFee + tip
// 		minRequired := tx.PriorityFee + newBlock.BaseFee
// 		effectiveGasPrice := tx.GasPrice
// 		if effectiveGasPrice < minRequired {
// 			tx.Status = constantset.StatusFailed
// 			continue
// 		}

// 		// ✅ REAL WALLET BALANCE CHECK (no bc.Accounts here)
// 		senderBal, err := bc.GetWalletBalance(tx.From)
// 		if err != nil {
// 			log.Printf("GetWalletBalance(%s) failed: %v", tx.From, err)
// 			tx.Status = constantset.StatusFailed
// 			continue
// 		}
// 		receiverBal, err := bc.GetWalletBalance(tx.To)
// 		if err != nil {
// 			// For a brand-new account, treat as 0
// 			receiverBal = 0
// 		}

// 		feeTokens := gasUnits * effectiveGasPrice
// 		totalCost := tx.Value + feeTokens
// 		if senderBal < totalCost {
// 			tx.Status = constantset.StatusFailed
// 			continue
// 		}

// 		// Execute state changes using the live balances; cache them in bc.Accounts
// 		senderBal -= totalCost
// 		receiverBal += tx.Value

// 		bc.Accounts[tx.From] = senderBal
// 		bc.Accounts[tx.To] = receiverBal

// 		totalGasUnits += gasUnits
// 		totalGasFeesTokens += feeTokens

// 		tx.Status = constantset.StatusSuccess
// 		processedTxs = append(processedTxs, tx)
// 	}

// 	// Finalize block fields
// 	newBlock.Transactions = processedTxs
// 	for _, tx := range newBlock.Transactions {
// 		tx.Status = constantset.StatusSuccess
// 		bc.RecordRecentTx(tx)
// 	}
// 	newBlock.GasUsed = totalGasUnits                // store units here
// 	newBlock.CurrentHash = CalculateHash(&newBlock) // your hash function

// 	// Append block to chain
// 	bc.Blocks = append(bc.Blocks, &newBlock)

// 	// --- PoDL Reward Distribution ---
// 	// Build a reward pool from base reward + slice of fees (in TOKENS)
// 	baseBlockReward := uint64(10)
// 	variableSlice := totalGasFeesTokens / 2

// 	// Modulate by load (based on gas UNITS consumption vs target units)
// 	targetGas := newBlock.GasLimit / 2
// 	load := 1.0
// 	if targetGas > 0 {
// 		ratio := float64(newBlock.GasUsed) / float64(targetGas)
// 		if ratio > 1.5 {
// 			ratio = 1.5
// 		}
// 		if ratio < 0.75 {
// 			ratio = 0.75
// 		}
// 		load = ratio
// 	}

// 	totalRewardPool := uint64(float64(baseBlockReward+variableSlice) * load)

// 	// Distribute
// 	leaderCut := (totalRewardPool * uint64(constantset.Leader)) / 100
// 	bc.Accounts[validator.Address] += leaderCut

// 	valMap := bc.CalculateRewardForValidator(totalRewardPool)
// 	lpMap := bc.CalculateRewardForLiquidity(totalRewardPool)

// 	dist := map[string]uint64{validator.Address: leaderCut}
// 	for addr, amt := range valMap {
// 		bc.Accounts[addr] += amt
// 		dist[addr] += amt
// 	}
// 	for addr, amt := range lpMap {
// 		bc.Accounts[addr] += amt
// 		dist[addr] += amt
// 	}

// 	// Track reward history
// 	bc.RewardHistory = append(bc.RewardHistory, RewardSnapshot{
// 		BlockNumber: newBlock.BlockNumber,
// 		BaseFee:     newBlock.BaseFee,
// 		GasUsed:     newBlock.GasUsed,
// 		Dist:        dist,
// 	})

// 	// Trim tx pool by removing only included txs (don’t slice by count — order can differ)
// 	if len(processedTxs) > 0 {
// 		included := make(map[string]struct{}, len(processedTxs))
// 		for _, itx := range processedTxs {
// 			included[itx.TxHash] = struct{}{}
// 		}
// 		remaining := make([]*Transaction, 0, len(bc.Transaction_pool))
// 		for _, ptx := range bc.Transaction_pool {
// 			if _, ok := included[ptx.TxHash]; !ok {
// 				remaining = append(remaining, ptx)
// 			}
// 		}
// 		bc.Transaction_pool = remaining
// 	}

// 	// Persist
// 	if err := SaveBlockToDB(&newBlock); err != nil {
// 		log.Printf("Error saving block: %v", err)
// 		return nil
// 	}

// 	log.Printf("⏱️ Block #%d mined in %v | txs=%d | gasUsed(units)=%d | gasFees(tokens)=%d | rewardPool=%d",
// 		newBlock.BlockNumber, time.Since(startTime), len(processedTxs), totalGasUnits, totalGasFeesTokens, totalRewardPool)

// 	return &newBlock
// }

// func (bc *Blockchain_struct) MineNewBlock() *Block {
// 	startTime := time.Now()
// 	defer func() {
// 		if r := recover(); r != nil {
// 			log.Printf("Recovered from panic in MineNewBlock: %v", r)
// 		}
// 	}()

// 	baseFee := bc.CalculateBaseFee()

// 	validator, err := bc.SelectValidator()
// 	if err != nil {
// 		log.Printf("Validator selection error: %v", err)
// 		return nil
// 	}

// 	lastBlock := bc.Blocks[len(bc.Blocks)-1]
// 	newBlock := NewBlock(lastBlock.BlockNumber, lastBlock.CurrentHash)
// 	newBlock.GasLimit = bc.CalculateNextGasLimit()
// 	newBlock.BaseFee = baseFee

// 	totalGasFees := uint64(0)
// 	processedTxs := make([]*Transaction, 0, len(bc.Transaction_pool))

// 	for _, tx := range bc.Transaction_pool {
// 		// Check block gas budget by gas * price, but also enforce MaxBlockGas cap
// 		// NOTE: Your previous code compared totalGasFees+tx.Gas to MaxBlockGas (units mismatch). Fix by using gas cost tokens.
// 		gasCost := tx.CalculateGasCost() * tx.GasPrice
// 		if totalGasFees+gasCost > uint64(constantset.MaxBlockGas) {
// 			break
// 		}

// 		if !bc.VerifyTransaction(tx) {
// 			tx.Status = constantset.StatusFailed
// 			continue
// 		}

// 		if tx.GasPrice < (baseFee + tx.PriorityFee) {
// 			tx.Status = constantset.StatusFailed
// 			continue
// 		}
// 		if bc.Accounts[tx.From] < (tx.Value + gasCost) {
// 			tx.Status = constantset.StatusFailed
// 			continue
// 		}

// 		// Execute transfer
// 		bc.Accounts[tx.From] -= (tx.Value + gasCost)
// 		bc.Accounts[tx.To] += tx.Value
// 		totalGasFees += gasCost

// 		tx.Status = constantset.StatusSuccess
// 		processedTxs = append(processedTxs, tx)
// 	}

// 	newBlock.Transactions = processedTxs
// 	newBlock.GasUsed = totalGasFees
// 	newBlock.CurrentHash = CalculateHash(&newBlock)
// 	bc.Blocks = append(bc.Blocks, &newBlock)

// 	// --- PoDL Reward Distribution ---
// 	// Base block reward + a slice of gas fees form the reward pool
// 	baseBlockReward := uint64(10)
// 	variableSlice := totalGasFees / 2

// 	// Dynamic modulation (economic feedback): boost when network is hot
// 	// factor ~ 1.0 .. 1.5 depending on last block load
// 	targetGas := newBlock.GasLimit / 2
// 	load := 1.0
// 	if targetGas > 0 {
// 		ratio := float64(newBlock.GasUsed) / float64(targetGas)
// 		if ratio > 1.5 {
// 			ratio = 1.5
// 		}
// 		if ratio < 0.75 {
// 			ratio = 0.75
// 		}
// 		load = ratio
// 	}

// 	totalRewardPool := uint64(float64(baseBlockReward+variableSlice) * load)

// 	// Split pool per constants
// 	leaderCut := (totalRewardPool * uint64(constantset.Leader)) / 100
// 	bc.Accounts[validator.Address] += leaderCut

// 	// Validators slice
// 	valMap := bc.CalculateRewardForValidator(totalRewardPool)
// 	// LP slice
// 	lpMap := bc.CalculateRewardForLiquidity(totalRewardPool)

// 	// Apply transfers
// 	dist := map[string]uint64{validator.Address: leaderCut}
// 	for addr, amt := range valMap {
// 		bc.Accounts[addr] += amt
// 		dist[addr] += amt
// 	}
// 	for addr, amt := range lpMap {
// 		bc.Accounts[addr] += amt
// 		dist[addr] += amt
// 	}

// 	// Save reward snapshot
// 	bc.RewardHistory = append(bc.RewardHistory, RewardSnapshot{
// 		BlockNumber: newBlock.BlockNumber,
// 		BaseFee:     newBlock.BaseFee,
// 		GasUsed:     newBlock.GasUsed,
// 		Dist:        dist,
// 	})

// 	// Trim tx pool (remove only what we included)
// 	bc.Transaction_pool = bc.Transaction_pool[len(processedTxs):]

// 	if err := SaveBlockToDB(&newBlock); err != nil {
// 		log.Printf("Error saving block: %v", err)
// 		return nil
// 	}

// 	log.Printf("⏱️ Block #%d mined in %v | txs=%d | Gas=%d | rewardPool=%d",
// 		newBlock.BlockNumber, time.Since(startTime), len(processedTxs), newBlock.GasUsed, totalRewardPool)

// 	return &newBlock
// }

// func (bc *Blockchain_struct) MineNewBlock() *Block {
// 	startTime := time.Now()

// 	defer func() {
// 		if r := recover(); r != nil {
// 			log.Printf("Recovered from panic in MineNewBlock: %v", r)
// 		}
// 	}()

// 	baseFee := bc.CalculateBaseFee()

// 	validator, err := bc.SelectValidator()
// 	if err != nil {
// 		log.Printf("Validator selection error: %v", err)
// 		return nil
// 	}

// 	lastBlock := bc.Blocks[len(bc.Blocks)-1]
// 	newBlock := NewBlock(lastBlock.BlockNumber, lastBlock.CurrentHash)
// 	newBlock.GasLimit = bc.CalculateNextGasLimit()
// 	newBlock.BaseFee = baseFee

// 	// Process transactions directly without full copy
// 	totalGasFees := uint64(0)
// 	successfulTxs := 0
// 	processedTxs := make([]*Transaction, 0, len(bc.Transaction_pool))

// 	for _, tx := range bc.Transaction_pool {
// 		// Check if block gas limit reached
// 		if totalGasFees+tx.Gas > uint64(constantset.MaxBlockGas) {
// 			break
// 		}

// 		// Validate transaction
// 		if !bc.VerifyTransaction(tx) {
// 			tx.Status = constantset.StatusFailed
// 			continue
// 		}

// 		gasTotal := tx.CalculateGasCost() * tx.GasPrice

// 		if tx.GasPrice < (baseFee + tx.PriorityFee) {
// 			tx.Status = constantset.StatusFailed
// 			continue
// 		}

// 		if bc.Accounts[tx.From] < (tx.Value + gasTotal) {
// 			tx.Status = constantset.StatusFailed
// 			continue
// 		}

// 		// Execute transaction
// 		bc.Accounts[tx.From] -= (tx.Value + gasTotal)
// 		bc.Accounts[tx.To] += tx.Value
// 		totalGasFees += gasTotal

// 		tx.Status = constantset.StatusSuccess
// 		processedTxs = append(processedTxs, tx)
// 		successfulTxs++

// 		// Remove from pool as we process (more efficient)

// 		// We'll handle pool cleanup at the end
// 	}

// 	newBlock.Transactions = processedTxs
// 	newBlock.GasUsed = totalGasFees

// 	// Distribute rewards
// 	if totalGasFees > 0 {
// 		validatorReward := totalGasFees / 2
// 		bc.Accounts[validator.Address] += validatorReward
// 		bc.SlashingPool += float64(validatorReward)
// 	}

// 	// Block reward
// 	blockReward := uint64(10)
// 	bc.Accounts[validator.Address] += blockReward

// 	// Calculate hash
// 	newBlock.CurrentHash = CalculateHash(&newBlock)

// 	// Add to blockchain
// 	bc.Blocks = append(bc.Blocks, &newBlock)

// 	// Clean transaction pool - only remove processed transactions
// 	bc.Transaction_pool = bc.Transaction_pool[len(processedTxs):]

// 	// Optimized database save - only save new block, not entire chain
// 	if err := SaveBlockToDB(&newBlock); err != nil {
// 		log.Printf("Error saving block: %v", err)
// 		return nil
// 	}

// 	totalTime := time.Since(startTime)
// 	log.Printf("⏱️  Block #%d mined in %v | Txs: %d/%d | Gas: %d",
// 		newBlock.BlockNumber, totalTime, successfulTxs, len(processedTxs), newBlock.GasUsed)

// 	return &newBlock
// }

// func (bc *Blockchain_struct) MineNewBlock() *Block {

// 	defer func() {
// 		if r := recover(); r != nil {
// 			log.Printf("Recovered from panic in MineNewBlock: %v", r)
// 		}
// 	}()

// 	baseFee := bc.CalculateBaseFee()

// 	if len(bc.Transaction_pool) > 10000 {
// 		runtime.GOMAXPROCS(runtime.NumCPU() / 2)
// 		defer runtime.GOMAXPROCS(runtime.NumCPU())
// 	}
// 	validator, err := bc.SelectValidator()
// 	if err != nil {
// 		log.Printf("Validator selection error: %v", err)
// 		return nil
// 	}

// 	lastBlock := bc.Blocks[len(bc.Blocks)-1]
// 	newBlock := NewBlock(lastBlock.BlockNumber, lastBlock.CurrentHash)
// 	newBlock.GasLimit = bc.CalculateNextGasLimit()

// 	newBlock.BaseFee = baseFee
// 	newBlock.Transactions = bc.CopyTransactions()

// 	// Parallel transaction validation
// 	var wg sync.WaitGroup
// 	txChan := make(chan *Transaction, len(bc.Transaction_pool))
// 	validTxs := make([]*Transaction, 0, len(bc.Transaction_pool))

// 	// Start worker pool
// 	for i := 0; i < runtime.NumCPU(); i++ {
// 		wg.Add(1)
// 		go func() {
// 			defer wg.Done()
// 			for tx := range txChan {
// 				if bc.VerifyTransaction(tx) {
// 					validTxs = append(validTxs, tx)
// 				}
// 			}
// 		}()
// 	}

// 	// Feed transactions to workers
// 	for _, tx := range bc.Transaction_pool {
// 		txChan <- tx
// 	}
// 	close(txChan)
// 	wg.Wait()

// 	newBlock.Transactions = validTxs
// 	// Process transactions
// 	totalGasFees := uint64(0)
// 	for _, tx := range newBlock.Transactions {
// 		gasTotal := tx.CalculateGasCost() * tx.GasPrice

// 		if totalGasFees+tx.Gas > uint64(constantset.MaxBlockGas) {
// 			break
// 		}

// 		if tx.GasPrice < (baseFee + tx.PriorityFee) {
// 			tx.Status = constantset.StatusFailed
// 			continue
// 		}

// 		if bc.Accounts[tx.From] < (tx.Value + gasTotal) {
// 			tx.Status = constantset.StatusFailed
// 			continue
// 		}

// 		totalGasFees += tx.Gas
// 		newBlock.Transactions = append(newBlock.Transactions, tx)

// 		// Deduct from sender
// 		bc.Accounts[tx.From] -= (tx.Value + gasTotal)

// 		// Add value to recipient
// 		bc.Accounts[tx.To] += tx.Value

// 		// Collect gas fees
// 		totalGasFees += gasTotal

// 		tx.Status = constantset.StatusSuccess
// 		log.Println("tx is succcess", tx.Status)
// 	}

// 	// Distribute rewards
// 	if totalGasFees > 0 {
// 		// 50% to validator, 50% to slashing pool
// 		validatorReward := totalGasFees / 2
// 		bc.Accounts[validator.Address] += validatorReward
// 		bc.SlashingPool += float64(validatorReward)
// 	}

// 	// Block reward (fixed amount)
// 	blockReward := uint64(10)
// 	bc.Accounts[validator.Address] += blockReward
// 	newBlock.GasUsed = totalGasFees
// 	newBlock.CurrentHash = CalculateHash(&newBlock)
// 	bc.Blocks = append(bc.Blocks, &newBlock)
// 	bc.Transaction_pool = []*Transaction{}
// 	dbCopy := *bc
// 	dbCopy.Mutex = sync.Mutex{}
// 	if err := PutIntoDB(dbCopy); err != nil {
// 		log.Printf("Error saving block: %v", err)
// 		return nil
// 	}

// 	return &newBlock
// }

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
