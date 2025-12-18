package blockchaincomponent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	constantset "github.com/Zotish/DefenceProject/ConstantSet"
	"github.com/ethereum/go-ethereum/crypto"
)

func ValidateAddress(addr string) bool {
	return strings.HasPrefix(addr, "0x") && len(addr) == 42
}

const (
	TransactionTTL = 24 * time.Hour
	MaxRecentTxs   = 50000000000000000
)

type LiquidityProvider struct {
	Address        string `json:"address"`
	StakeAmount    uint64 `json:"stake_amount"`
	LiquidityPower uint64 `json:"liquidity_power"`
	LockTime       int64  `json:"lock_time"`
	TotalRewards   uint64 `json:"total_rewards"`
	PendingRewards uint64 `json:"pending_rewards"`

	IsUnstaking      bool   `json:"is_unstaking"`
	UnstakeStartTime int64  `json:"unstake_start_time"`
	UnstakeAmount    uint64 `json:"unstake_amount"`
	ReleasedSoFar    uint64 `json:"released_so_far"`
}

type BlockRewardBreakdown struct {
	Validator          string            `json:"validator"`
	ValidatorReward    uint64            `json:"validator_reward"`
	LiquidityRewards   map[string]uint64 `json:"liquidity_rewards"`
	ParticipantRewards map[string]uint64 `json:"participant_rewards"`
}
type LockRecord struct {
	Amount    uint64    `json:"amount"`
	UnlockAt  time.Time `json:"unlock_at"`
	CreatedAt time.Time `json:"created_at"`
}

type RewardSnapshot struct {
	BlockNumber uint64            `json:"block_number"`
	BaseFee     uint64            `json:"base_fee"`
	GasUsed     uint64            `json:"gas_used"`
	Dist        map[string]uint64 `json:"dist"`
}

type Blockchain_struct struct {
	Blocks           []*Block          `json:"blocks"`
	Transaction_pool []*Transaction    `json:"transaction_pool"`
	Validators       []*Validator      `json:"validator"`
	Accounts         map[string]uint64 `json:"accounts"`
	MinStake         float64           `json:"min_stake"`
	SlashingPool     float64           `json:"slashing_pool"`
	Network          *NetworkService   `json:"-"`
	Mutex            sync.Mutex        `json:"-"`
	BaseFee          uint64            `json:"base_fee"`
	//VM                  *VM                     `json:"vm"`
	LiquidityLocks      map[string][]LockRecord `json:"liquidity_locks"`
	TotalLiquidity      uint64                  `json:"total_liquidity"`
	RewardHistory       []RewardSnapshot        `json:"reward_history"`
	RecentTxs           []*Transaction          `json:"recent_txs"`
	PendingFeePool      map[string]uint64       `json:"pending_fee_pool"`
	ContractEngine      *LQDContractEngine      `json:"-"`
	LastBlockMiningTime time.Duration           `json:"last_block_mining_time"`
	LiquidityProviders  map[string]*LiquidityProvider

	FixedBlockReward    uint64
	GasRewardMultiplier uint64

	MinLiquidityStake uint64
}

func (bc *Blockchain_struct) SaveBlockToDB(block *Block) error {
	return SaveBlockToDB(block)
}
func (bc *Blockchain_struct) RecordRecentTx(tx *Transaction) {
	if tx == nil {
		return
	}

	h := strings.ToLower(tx.TxHash)
	if h == "" {
		return
	}

	// Dedup by hash
	filtered := make([]*Transaction, 0, len(bc.RecentTxs))
	for _, existing := range bc.RecentTxs {
		if strings.ToLower(existing.TxHash) != h {
			filtered = append(filtered, existing)
		}
	}

	// Insert newest first
	filtered = append([]*Transaction{tx}, filtered...)

	// Keep max length
	if len(filtered) > MaxRecentTxs {
		filtered = filtered[:MaxRecentTxs]
	}

	bc.RecentTxs = filtered
}

// GetLock is the exported wrapper so other packages can read active locked amount.
func (bc *Blockchain_struct) GetLock(address string) uint64 {
	return bc.getLock(address)
}

// Only keep recent blocks in memory for performance
func (bc *Blockchain_struct) TrimInMemoryBlocks(keepLastN int) {
	if len(bc.Blocks) <= keepLastN {
		return
	}

	// Keep only the last N blocks in memory
	bc.Blocks = bc.Blocks[len(bc.Blocks)-keepLastN:]
	log.Printf("Trimmed in-memory blocks, keeping last %d blocks", keepLastN)
}

// Efficient transaction pool cleanup
func (bc *Blockchain_struct) CleanTransactionPool() {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()

	// Remove old failed transactions
	now := uint64(time.Now().Unix())
	validTxs := make([]*Transaction, 0, len(bc.Transaction_pool))

	for _, tx := range bc.Transaction_pool {
		// Keep transactions that are recent or have high fees
		if now-tx.Timestamp < uint64(3600) || tx.GasPrice > bc.BaseFee*2 {
			validTxs = append(validTxs, tx)
		}
	}

	if len(validTxs) < len(bc.Transaction_pool) {
		removed := len(bc.Transaction_pool) - len(validTxs)
		bc.Transaction_pool = validTxs
		log.Printf("Cleaned transaction pool: removed %d old transactions", removed)
	}
}

// In NewBlockchain function, ensure network starts properly
func NewBlockchain(genesisBlock Block) *Blockchain_struct {
	exist, _ := KeyExist()
	if exist {
		blockchainStruct, err := GetBlockchain()
		if err != nil {
			log.Printf("Error loading blockchain from DB: %v", err)
			return nil
		}
		// Restart network service for loaded blockchain
		blockchainStruct.Network = NewNetworkService(blockchainStruct)
		if err := blockchainStruct.Network.Start(); err != nil {
			log.Printf("Failed to restart network service: %v", err)
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
		newBlockchain.LiquidityProviders = make(map[string]*LiquidityProvider)
		newBlockchain.MinStake = 100000 * float64(constantset.Decimals)
		newBlockchain.SlashingPool = 0
		newBlockchain.Accounts[constantset.LiquidityPoolAddress] = 0

		//newBlockchain.VM = NewVM()
		newBlockchain.Validators = []*Validator{}
		newBlockchain.Network = NewNetworkService(newBlockchain)
		newBlockchain.Mutex = sync.Mutex{}
		newBlockchain.LiquidityLocks = make(map[string][]LockRecord)
		newBlockchain.TotalLiquidity = 0
		newBlockchain.RewardHistory = []RewardSnapshot{}
		newBlockchain.RecentTxs = []*Transaction{}
		newBlockchain.PendingFeePool = make(map[string]uint64)
		engine, _ := NewLQDContractEngine()

		newBlockchain.ContractEngine = engine

		// Start network service
		if err := newBlockchain.Network.Start(); err != nil {
			log.Printf("Failed to start network service: %v", err)
		}
		// Save to DB
		blockchainCopy := *newBlockchain
		blockchainCopy.Mutex = sync.Mutex{}
		err := PutIntoDB(blockchainCopy)
		if err != nil {
			log.Printf("Failed to save blockchain to DB: %v", err)
			return nil
		}

		return newBlockchain
	}
}

func (bc *Blockchain_struct) CleanStaleTransactions() {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()

	if len(bc.Transaction_pool) == 0 {
		return
	}

	now := uint64(time.Now().Unix())
	cutoff := now - uint64(constantset.TransactionTTL)

	filtered := bc.Transaction_pool[:0]

	for _, tx := range bc.Transaction_pool {
		// If tx is still within TTL, keep it in the mempool
		if tx.Timestamp >= cutoff {
			filtered = append(filtered, tx)
			continue
		}

		// Too old -> mark as failed/expired and move to recent history
		if tx.Status == constantset.StatusPending {
			tx.Status = constantset.StatusFailed
		}

		bc.RecordRecentTx(tx)
	}

	bc.Transaction_pool = filtered
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

	if bc.BaseFee == 0 {
		bc.BaseFee = bc.CalculateBaseFee()
	}

	// TTL check first – if expired, mark failed and store in recent story
	if uint64(time.Now().Unix())-tx.Timestamp > uint64(TransactionTTL.Seconds()) {
		tx.Status = constantset.StatusFailed
		// make sure hash exists so explorer can reference it
		if tx.TxHash == "" {
			tx.TxHash = CalculateTransactionHash(*tx)
		}
		bc.RecordRecentTx(tx)
		return fmt.Errorf("transaction expired")
	}

	// Effective priority fee used for replacement logic
	eff := bc.BaseFee + tx.PriorityFee
	replaced := false

	for i, existing := range bc.Transaction_pool {
		if strings.EqualFold(existing.From, tx.From) {
			// && existing.Nonce == tx.Nonce  // if you re-enable nonce
			oldEff := bc.BaseFee + existing.PriorityFee

			// Require >= 10% bump
			if eff*100 >= oldEff*110 {
				bc.Transaction_pool[i] = tx
				replaced = true
			} else {
				return fmt.Errorf("replacement requires >=10%% higher effective fee")
			}
			break
		}
	}

	if !replaced {
		if bc.countTxsFrom(tx.From) >= constantset.MaxTxsPerAccount {
			return fmt.Errorf("account tx pool limit reached (%d/%d)",
				bc.countTxsFrom(tx.From), constantset.MaxTxsPerAccount)
		}
		bc.Transaction_pool = append(bc.Transaction_pool, tx)

	}

	// sort by effective priority fee (desc)
	sort.Slice(bc.Transaction_pool, func(i, j int) bool {
		ip := bc.BaseFee + bc.Transaction_pool[i].PriorityFee
		jp := bc.BaseFee + bc.Transaction_pool[j].PriorityFee
		return ip > jp
	})

	if len(bc.Transaction_pool) > constantset.MaxTxPoolSize {
		// Optionally mark this tx as failed + story
		tx.Status = constantset.StatusFailed
		if tx.TxHash == "" {
			tx.TxHash = CalculateTransactionHash(*tx)
		}
		bc.RecordRecentTx(tx)
		bc.Transaction_pool = bc.Transaction_pool[:constantset.MaxTxPoolSize]
		return fmt.Errorf("txpool full")
	}

	// Now that it *is* accepted into the pool, give it pending status + hash
	tx.Status = constantset.StatusPending
	tx.TxHash = CalculateTransactionHash(*tx)

	// 🔥 THIS is where we add it to the global explorer story
	bc.RecordRecentTx(tx)

	// Persist chain state
	dbCopy := *bc
	dbCopy.Mutex = sync.Mutex{}
	if err := PutIntoDB(dbCopy); err != nil {
		return fmt.Errorf("failed to update blockchain in DB: %v", err)
	}
	return nil
}

func (bc *Blockchain_struct) GetWalletBalance(address string) (uint64, error) {
	// First, try the in-memory cache if it’s fresh enough
	if bal, ok := bc.Accounts[address]; ok {
		return bal, nil
	}

	// Otherwise query the wallet server (or on-chain DB)
	walletNode := "http://127.0.0.1:8080" // or use os.Getenv("WALLET_NODE")
	resp, err := http.Get(fmt.Sprintf("%s/wallet/balance?address=%s", walletNode, url.QueryEscape(address)))
	if err != nil {
		return 0, fmt.Errorf("wallet node unreachable: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("wallet node error: %s", string(body))
	}

	var result struct {
		Balance uint64 `json:"balance"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("decode error: %v", err)
	}

	// Optionally update the local cache
	bc.Accounts[address] = result.Balance
	return result.Balance, nil
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

func (bc *Blockchain_struct) VerifyTransaction(tx *Transaction) bool {

	isSystem := tx.IsSystem ||
		tx.Type == "stake" ||
		tx.Type == "unstake" ||
		tx.Type == "lp_reward"

	if isSystem {
		// Ensure ChainID is correct, even if not set
		if tx.ChainID == 0 {
			tx.ChainID = uint64(constantset.ChainID)
		}
		// No gas / sig / balance checks for internal bookkeeping txs
		tx.Status = constantset.StatusPending
		return true
	}

	// 0) Basic shape
	if tx.From == "" || tx.To == "" {
		tx.Status = constantset.StatusFailed
		fmt.Printf("TX %s failed: missing from/to", tx.TxHash)
		return false
	}

	// 1) Address + ChainID
	if !ValidateAddress(tx.From) || !ValidateAddress(tx.To) {
		tx.Status = constantset.StatusFailed
		log.Printf("TX %s failed: invalid address format", tx.TxHash)
		return false
	}

	if tx.ChainID != uint64(constantset.ChainID) {
		tx.Status = constantset.StatusFailed
		log.Printf("TX %s failed: invalid chain ID", tx.TxHash)
		return false
	}

	// 2) Timestamp sanity (allow small future skew)
	now := uint64(time.Now().Unix())
	const maxPast = uint64(3600)  // 1h old -> reject
	const maxFuture = uint64(600) // >5m in future -> reject
	if tx.Timestamp+maxFuture < now || now-tx.Timestamp > maxPast {
		tx.Status = constantset.StatusFailed
		log.Printf("TX %s failed: timestamp out of range (ts=%d now=%d)", tx.TxHash, tx.Timestamp, now)
		return false
	}

	// 3) Fee policy: require gas price to meet baseFee (+ optional priority)
	baseFee := bc.CalculateBaseFee()
	minRequired := baseFee + tx.PriorityFee
	if tx.Gas == 0 {
		tx.Gas = uint64(constantset.MinGas) // defensive default
	}
	if tx.GasPrice < minRequired {
		tx.Status = constantset.StatusFailed
		log.Printf("TX %s failed: gas_price < baseFee+tip (%d < %d)", tx.TxHash, tx.GasPrice, minRequired)
		return false
	}

	// 4) Nonce policy
	//expected := bc.GetAccountNonce(tx.From)
	// If your node stores "current" nonce (last used), uncomment:
	//expected++
	// if tx.Nonce != expected {
	// 	tx.Status = constantset.StatusFailed
	// 	log.Printf("TX %s failed: bad nonce (got %d want %d)", tx.TxHash, tx.Nonce, expected)
	// 	return false
	// }

	// 5) Signature (v normalized in wallet: v∈{27,28})

	isVerifySig := bc.VerifyTransactionSignature(tx)
	if !isVerifySig {
		tx.Status = constantset.StatusFailed
		log.Printf("TX %s failed: signature verify", tx.TxHash)
		return false
	}

	// 6) Balance (live wallet) — light precheck to avoid junk in pool
	// NOTE: final authoritative debit happens in MineNewBlock().
	totalCost := tx.Value + (tx.GasPrice * tx.CalculateGasCost())
	bal, err := bc.GetWalletBalance(tx.From)
	if err != nil {
		tx.Status = constantset.StatusFailed
		log.Printf("TX %s failed: balance lookup error: %v", tx.TxHash, err)
		return false
	}
	if bal < totalCost {
		tx.Status = constantset.StatusFailed
		log.Printf("TX %s failed: insufficient funds (have %d need %d)", tx.TxHash, bal, totalCost)
		return false
	}

	// Passes admission checks
	tx.Status = constantset.StatusPending
	return true
}

func (bc *Blockchain_struct) GetAccountNonce(address string) uint64 {
	// Check confirmed transactions in blocks first
	highestNonce := uint64(0)
	for _, block := range bc.Blocks {
		for _, tx := range block.Transactions {
			if tx.From == address && tx.Nonce >= highestNonce {
				fmt.Printf("Found confirmed tx: From=%s, Nonce=%d\n", tx.From, tx.Nonce)
				highestNonce = tx.Nonce + 1
			}
		}
	}

	// Then check pending transactions
	for _, tx := range bc.Transaction_pool {
		if tx.From == address && tx.Nonce >= highestNonce {
			fmt.Printf("Found pending tx: From=%s, Nonce=%d\n", tx.From, tx.Nonce)
			highestNonce = tx.Nonce + 1
		}
	}
	fmt.Printf("Returning nonce for %s: %d\n", address, highestNonce)

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

func (bc *Blockchain_struct) VerifyTransactionSignature(tx *Transaction) bool {

	// 0) Chain sanity

	if tx.ChainID != uint64(constantset.ChainID) {
		log.Printf("Invalid chain ID: got %d, want %d", tx.ChainID, constantset.ChainID)
		return false
	}

	// 1) Signature shape
	if len(tx.Sig) != 65 {
		log.Printf("Invalid signature length: %d", len(tx.Sig))
		return false
	}
	v := tx.Sig[64]
	if v != 0 && v != 1 && v != 27 && v != 28 {
		log.Printf("Invalid recovery ID: %d", v)
		return false
	}

	// Add timestamp validation (prevent replay of old transactions)
	if uint64(time.Now().Unix())-tx.Timestamp > 3600 { // 1 hour expiry
		tx.Status = constantset.StatusFailed
		log.Printf("Transaction %s expired", tx.TxHash)
		return false
	}

	// 2) Rebuild EXACT signing payload (keep nonce commented out to match wallet right now)
	signingData := map[string]interface{}{
		"from":      tx.From,
		"to":        tx.To,
		"value":     tx.Value,
		"data":      hex.EncodeToString(tx.Data),
		"gas":       tx.Gas,
		"gas_price": tx.GasPrice,
		// "nonce":  tx.Nonce,          // keep commented if wallet also omits
		"chain_id":  tx.ChainID,
		"timestamp": tx.Timestamp,
	}

	b, err := json.Marshal(signingData)
	if err != nil {
		log.Printf("marshal signing data: %v", err)
		return false
	}

	// 3) Double SHA-256 (matches wallet)
	h1 := sha256.Sum256(b)
	hash := sha256.Sum256(h1[:])

	// 4) Normalize V then recover
	sig := make([]byte, 65)
	copy(sig, tx.Sig)
	if sig[64] >= 27 {
		sig[64] -= 27 // 27/28 -> 0/1
	}

	pubKeyBytes, err := crypto.Ecrecover(hash[:], sig)
	if err != nil {
		log.Printf("Error recovering public key: %v", err)
		return false
	}
	if !crypto.VerifySignature(pubKeyBytes, hash[:], sig[:64]) {
		log.Printf("Signature verification failed (RS mismatch)")
		return false
	}

	// 5) Check recovered address
	pubKey, err := crypto.UnmarshalPubkey(pubKeyBytes)
	if err != nil {
		log.Printf("unmarshal pubkey: %v", err)
		return false
	}
	recoveredAddr := crypto.PubkeyToAddress(*pubKey).Hex()
	if !strings.EqualFold(recoveredAddr, tx.From) {
		log.Printf("Recovered %s != from %s", recoveredAddr, tx.From)
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

func (bc *Blockchain_struct) RecordSystemTx(
	from, to string,
	value, gasUsed, gasPrice uint64,
	status string,
	isContract bool,
	function string,
	args []string,
) *Transaction {
	tx := &Transaction{
		From:       from,
		To:         to,
		Value:      value,
		Gas:        gasUsed,
		GasPrice:   gasPrice,
		ChainID:    uint64(constantset.ChainID),
		Timestamp:  uint64(time.Now().Unix()),
		Status:     status,
		IsContract: isContract,
		Function:   function,
		Args:       args,
		Type:       "system",
		// Sig/Nonce left empty for system/HTTP-driven tx
	}

	tx.TxHash = CalculateTransactionHash(*tx)
	bc.RecordRecentTx(tx)

	return tx
}

// Add this inside the constructor AFTER your original fields initialize
func (bc *Blockchain_struct) InitLiquiditySystem() {
	if bc.LiquidityProviders == nil {
		bc.LiquidityProviders = make(map[string]*LiquidityProvider)
	}

	// set your fixed reward for block
	bc.FixedBlockReward = 100

	// gas reward = gasFees * multiplier
	bc.GasRewardMultiplier = 1

	// min liquidity stake
	bc.MinLiquidityStake = 100
}

// Liquidity Functions (ADD-ONLY)

func (bc *Blockchain_struct) NewSystemTx(txType, from, to string, value uint64) *Transaction {
	tx := &Transaction{
		From:      from,
		To:        to,
		Value:     value,
		Gas:       21000,
		GasPrice:  1,
		ChainID:   uint64(constantset.ChainID),
		Timestamp: uint64(time.Now().Unix()),
		Status:    constantset.StatusPending,
		Type:      txType,
		IsSystem:  true,
	}

	tx.TxHash = CalculateTransactionHash(*tx)
	return tx
}

func (bc *Blockchain_struct) ProvideLiquidity(address string, amount uint64, lockDays int64) error {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()

	if amount < bc.MinLiquidityStake {
		return fmt.Errorf("minimum liquidity stake is %d LQD", bc.MinLiquidityStake)
	}

	if bc.Accounts[address] < amount {
		return fmt.Errorf("insufficient balance to stake")
	}

	bc.Accounts[address] -= amount

	lockTime := time.Now().Add(time.Hour * 24 * time.Duration(lockDays)).Unix()

	lp, exists := bc.LiquidityProviders[address]
	if !exists {
		lp = &LiquidityProvider{
			Address: address,
		}
	}

	lp.StakeAmount += amount
	lp.LiquidityPower = lp.StakeAmount * uint64(lockDays)
	lp.LockTime = lockTime

	bc.LiquidityProviders[address] = lp
	stakeTx := bc.NewSystemTx("stake", address, constantset.LiquidityPoolAddress, amount)
	bc.Transaction_pool = append(bc.Transaction_pool, stakeTx)

	return nil
}

// Start unstake request (does not release instantly)
func (bc *Blockchain_struct) StartUnstake(address string) error {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()

	lp, exists := bc.LiquidityProviders[address]
	if !exists {
		return fmt.Errorf("no liquidity position found")
	}

	now := time.Now().Unix()
	if now < lp.LockTime {
		return fmt.Errorf("liquidity still locked")
	}

	if lp.IsUnstaking {
		return fmt.Errorf("already unstaking")
	}

	if lp.PendingRewards > 0 {
		bc.Accounts[address] += lp.PendingRewards
		rewardTx := bc.NewSystemTx("lp_reward", constantset.LiquidityPoolAddress, lp.Address, lp.PendingRewards)

		//rewardTx := bc.NewSystemTx("lp_reward", constantset.LiquidityPoolAddress, address, lp.PendingRewards)

		bc.Transaction_pool = append(bc.Transaction_pool, rewardTx)
		lp.PendingRewards = 0
	}

	lp.IsUnstaking = true
	lp.UnstakeStartTime = now
	lp.UnstakeAmount = lp.StakeAmount
	lp.ReleasedSoFar = 0

	// LP stops earning new rewards
	lp.LiquidityPower = 0
	lp.StakeAmount = 0

	unstakeTx := bc.NewSystemTx("unstake", address, constantset.LiquidityPoolAddress, lp.UnstakeAmount)
	bc.Transaction_pool = append(bc.Transaction_pool, unstakeTx)

	return nil
}

// Release 1% daily to wallet
func (bc *Blockchain_struct) ProcessUnstakeReleases() {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()

	for _, lp := range bc.LiquidityProviders {
		if !lp.IsUnstaking {
			continue
		}

		daysPassed := (time.Now().Unix() - lp.UnstakeStartTime) / 86400
		if daysPassed <= 0 {
			continue
		}

		maxReleasable := lp.UnstakeAmount * uint64(daysPassed) / 100
		if maxReleasable > lp.UnstakeAmount {
			maxReleasable = lp.UnstakeAmount
		}

		if maxReleasable > lp.ReleasedSoFar {
			delta := maxReleasable - lp.ReleasedSoFar
			lp.ReleasedSoFar = maxReleasable
			bc.Accounts[lp.Address] += delta
			rewardTx := bc.NewSystemTx("lp_reward", constantset.LiquidityPoolAddress, lp.Address, delta)

			bc.Transaction_pool = append(bc.Transaction_pool, rewardTx)
		}
	}
}

// Add LP reward
func (bc *Blockchain_struct) AddLPReward(address string, reward uint64) {
	lp := bc.LiquidityProviders[address]
	if lp == nil {
		return
	}
	lp.PendingRewards += reward
	lp.TotalRewards += reward
}

// Add participant reward
func (bc *Blockchain_struct) AddParticipantReward(address string, reward uint64) {
	bc.Accounts[address] += reward
}
