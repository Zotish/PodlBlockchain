package main

import (
	"fmt"
	"strconv"

	blockchaincomponent "github.com/Zotish/DefenceProject/BlockchainComponent"
)

// Example minimal token
type LQDToken struct{}

func ParseUint(s string) uint64 {
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0
	}
	return v
}

func (c *LQDToken) Init(ctx *blockchaincomponent.Context, name string, symbol string, supply string) {
	ctx.Set("name", name)
	ctx.Set("symbol", symbol)
	ctx.Set("totalSupply", supply)

	ctx.Set("bal:"+ctx.OwnerAddr, supply)

	ctx.Emit("Init", map[string]interface{}{
		"name":   name,
		"symbol": symbol,
		"supply": supply,
	})
}

func (c *LQDToken) BalanceOf(ctx *blockchaincomponent.Context, addr string) {
	bal := ctx.Get("bal:" + addr)
	if bal == "" {
		bal = "0"
	}
	ctx.Set("output", bal)
}

func (c *LQDToken) Transfer(ctx *blockchaincomponent.Context, to string, amount string) {
	from := ctx.CallerAddr

	fromKey := "bal:" + from
	toKey := "bal:" + to

	fromBal := ParseUint(ctx.Get(fromKey))
	amt := ParseUint(amount)

	if fromBal < amt {
		ctx.Revert("insufficient balance")
	}

	ctx.Set(fromKey, fmt.Sprintf("%d", fromBal-amt))
	ctx.Set(toKey, fmt.Sprintf("%d", ParseUint(ctx.Get(toKey))+amt))

	ctx.Emit("Transfer", map[string]any{
		"from":   from,
		"to":     to,
		"amount": amount,
	})
}

// REQUIRED EXPORT
var Contract = &LQDToken{}







func (bc *Blockchain_struct) MineNewBlock() *Block { startTime := time.Now() defer func() { if r := recover(); r != nil { log.Printf("Recovered from panic in MineNewBlock: %v", r) } }() // Base fee for this block baseFee := bc.CalculateBaseFee() if len(bc.Transaction_pool) > 10000 { runtime.GOMAXPROCS(runtime.NumCPU() / 2) defer runtime.GOMAXPROCS(runtime.NumCPU()) } // Select proposer validator, err := bc.SelectValidator() if err != nil { log.Printf("Validator selection error: %v", err) return nil } // Build block shell lastBlock := bc.Blocks[len(bc.Blocks)-1] newBlock := NewBlock(lastBlock.BlockNumber, lastBlock.CurrentHash) newBlock.GasLimit = bc.CalculateNextGasLimit() newBlock.BaseFee = baseFee // Track gas UNITS separately from fee TOKENS var totalGasUnits uint64 = 0 var totalGasFeesTokens uint64 = 0 processedTxs := make([]*Transaction, 0, len(bc.Transaction_pool)) // Snapshot the pool so we iterate over a stable view txPool := make([]*Transaction, len(bc.Transaction_pool)) copy(txPool, bc.Transaction_pool) for _, tx := range txPool { gasUnits := tx.CalculateGasCost() // units, not tokens // Enforce block gas cap (units) if totalGasUnits+gasUnits > uint64(constantset.MaxBlockGas) { break } // Basic validity (sig, nonce, format) if !bc.VerifyTransaction(tx) { tx.Status = constantset.StatusFailed continue } // EIP-1559-ish constraint: require gasPrice >= baseFee + tip minRequired := tx.PriorityFee + newBlock.BaseFee effectiveGasPrice := tx.GasPrice if effectiveGasPrice < minRequired { tx.Status = constantset.StatusFailed continue } // ✅ REAL WALLET BALANCE CHECK senderBal, err := bc.GetWalletBalance(tx.From) if err != nil { log.Printf("GetWalletBalance(%s) failed: %v", tx.From, err) tx.Status = constantset.StatusFailed continue } receiverBal, err := bc.GetWalletBalance(tx.To) if err != nil { // For a brand-new account, treat as 0 receiverBal = 0 } feeTokens := gasUnits * effectiveGasPrice totalCost := tx.Value + feeTokens if senderBal < totalCost { tx.Status = constantset.StatusFailed continue } // Execute state changes using the live balances; cache them in bc.Accounts senderBal -= totalCost receiverBal += tx.Value bc.Accounts[tx.From] = senderBal bc.Accounts[tx.To] = receiverBal totalGasUnits += gasUnits totalGasFeesTokens += feeTokens tx.Status = constantset.StatusSuccess processedTxs = append(processedTxs, tx) } // 🔥 Merge pending contract-call & deployment fees into gas pool if bc.PendingFeePool != nil { for _, fee := range bc.PendingFeePool { totalGasFeesTokens += fee } bc.PendingFeePool = make(map[string]uint64) } // Finalize block fields newBlock.Transactions = processedTxs for _, tx := range newBlock.Transactions { tx.Status = constantset.StatusSuccess bc.RecordRecentTx(tx) } newBlock.GasUsed = totalGasUnits // store gas units newBlock.CurrentHash = CalculateHash(&newBlock) // hash // Append block to chain bc.Blocks = append(bc.Blocks, &newBlock) // --- PoDL Reward Distribution --- // Build a reward pool from base reward + slice of fees (in TOKENS) baseBlockReward := uint64(10) variableSlice := totalGasFeesTokens / 2 // Modulate by load (based on gas UNITS consumption vs target units) targetGas := newBlock.GasLimit / 2 load := 1.0 if targetGas > 0 { ratio := float64(newBlock.GasUsed) / float64(targetGas) if ratio > 1.5 { ratio = 1.5 } if ratio < 0.75 { ratio = 0.75 } load = ratio } totalRewardPool := uint64(float64(baseBlockReward+variableSlice) * load) // Distribute leaderCut := (totalRewardPool * uint64(constantset.Leader)) / 100 bc.Accounts[validator.Address] += leaderCut valMap := bc.CalculateRewardForValidator(totalRewardPool) lpMap := bc.CalculateRewardForLiquidity(totalRewardPool) // dist collects the final per-address reward (leader + validator + LP) dist := map[string]uint64{validator.Address: leaderCut} for addr, amt := range valMap { if amt == 0 { continue } bc.Accounts[addr] += amt dist[addr] += amt } for addr, amt := range lpMap { if amt == 0 { continue } bc.Accounts[addr] += amt dist[addr] += amt } // Track reward history bc.RewardHistory = append(bc.RewardHistory, RewardSnapshot{ BlockNumber: newBlock.BlockNumber, BaseFee: newBlock.BaseFee, GasUsed: newBlock.GasUsed, Dist: dist, }) // --- NEW: Write reward payouts as REAL block txs --- // NOTE: we ALREADY updated bc.Accounts above, so here we only // materialize tx objects & push to block / recent-tx history. const rewardPoolAddr = "0x0000000000000000000000000000000000000000" for addr, amount := range dist { if amount == 0 { continue } // Use helper so TxHash + timestamps + recent-tx are handled in one place tx := bc.RecordSystemTx( rewardPoolAddr, // system / reward pool addr, amount, 0, // gasUsed 0, // gasPrice constantset.StatusSuccess, false, // not a contract call "blockReward", []string{strconv.FormatUint(newBlock.BlockNumber, 10)}, ) // Attach to block so explorer can display them as mined txs newBlock.Transactions = append(newBlock.Transactions, tx) } // Trim tx pool by removing only included txs if len(processedTxs) > 0 { included := make(map[string]struct{}, len(processedTxs)) for _, itx := range processedTxs { included[itx.TxHash] = struct{}{} } remaining := make([]*Transaction, 0, len(bc.Transaction_pool)) for _, ptx := range bc.Transaction_pool { if _, ok := included[ptx.TxHash]; !ok { remaining = append(remaining, ptx) } } bc.Transaction_pool = remaining } miningDuration := time.Since(startTime) bc.LastBlockMiningTime = miningDuration // Persist if err := SaveBlockToDB(&newBlock); err != nil { log.Printf("Error saving block: %v", err) return nil } log.Printf("⏱️ Block #%d mined in %v | txs=%d | gasUsed(units)=%d | gasFees(tokens)=%d | rewardPool=%d", newBlock.BlockNumber, time.Since(startTime), len(newBlock.Transactions), totalGasUnits, totalGasFeesTokens, totalRewardPool) return &newBlock }