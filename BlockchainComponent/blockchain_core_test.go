package blockchaincomponent

import (
	"math/big"
	"strings"
	"testing"
	"time"

	constantset "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/ConstantSet"
)

// ─────────────────────────────────────────────────────────────────────────────
// helpers
// ─────────────────────────────────────────────────────────────────────────────

// newTestBlockchain builds a Blockchain_struct entirely in memory – no DB.
func newTestBlockchain() *Blockchain_struct {
	genesis := Block{
		BlockNumber:  0,
		PreviousHash: "0x0000000000000000000000000000000000000000000000000000000000000000",
		TimeStamp:    uint64(time.Now().Unix()),
		Transactions: []*Transaction{},
		GasLimit:     uint64(constantset.MaxBlockGas),
	}
	genesis.CurrentHash = CalculateHash(&genesis)

	bc := &Blockchain_struct{
		Blocks:           []*Block{&genesis},
		Transaction_pool: []*Transaction{},
		Accounts:         make(map[string]*big.Int),
		Validators:       []*Validator{},
		LiquidityLocks:   make(map[string][]LockRecord),
		TotalLiquidity:   big.NewInt(0),
		RecentTxs:        []*Transaction{},
		PendingFeePool:   make(map[string]*big.Int),
		BlockVotes:       make(map[string]map[string]bool),
		PendingBlocks:    make(map[string]*Block),
		BridgeRequests:   make(map[string]*BridgeRequest),
		BridgeTokenMap:   make(map[string]*BridgeTokenInfo),
		MinStake:         1000000 * 1e8,
		FixedBlockReward: 20,
	}
	return bc
}

// ─────────────────────────────────────────────────────────────────────────────
// Block creation
// ─────────────────────────────────────────────────────────────────────────────

func TestNewBlock_NumberIncrement(t *testing.T) {
	b := NewBlock(5, "0xprevhash")
	if b.BlockNumber != 6 {
		t.Errorf("expected block number 6, got %d", b.BlockNumber)
	}
}

func TestNewBlock_PreviousHashStored(t *testing.T) {
	prev := "0xabc123"
	b := NewBlock(0, prev)
	if b.PreviousHash != prev {
		t.Errorf("expected prev hash %q, got %q", prev, b.PreviousHash)
	}
}

func TestNewBlock_TimestampSet(t *testing.T) {
	before := uint64(time.Now().Unix()) - 2
	b := NewBlock(0, "0x0")
	after := uint64(time.Now().Unix()) + 2
	if b.TimeStamp < before || b.TimeStamp > after {
		t.Errorf("timestamp %d out of expected range [%d, %d]", b.TimeStamp, before, after)
	}
}

func TestNewBlock_EmptyTransactions(t *testing.T) {
	b := NewBlock(0, "0x0")
	if b.Transactions == nil {
		t.Error("Transactions should be initialized (not nil)")
	}
	if len(b.Transactions) != 0 {
		t.Errorf("expected 0 transactions, got %d", len(b.Transactions))
	}
}

func TestNewBlock_GasLimitSet(t *testing.T) {
	b := NewBlock(0, "0x0")
	if b.GasLimit == 0 {
		t.Error("GasLimit should be non-zero after NewBlock")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Hash calculation
// ─────────────────────────────────────────────────────────────────────────────

func TestCalculateHash_Deterministic(t *testing.T) {
	b := NewBlock(1, "0xprev")
	h1 := CalculateHash(&b)
	h2 := CalculateHash(&b)
	if h1 != h2 {
		t.Errorf("hash is not deterministic: %q vs %q", h1, h2)
	}
}

func TestCalculateHash_HasPrefix(t *testing.T) {
	b := NewBlock(1, "0xprev")
	h := CalculateHash(&b)
	if !strings.HasPrefix(h, "0x") {
		t.Errorf("hash %q should start with 0x", h)
	}
}

func TestCalculateHash_ChangesWithContent(t *testing.T) {
	b1 := NewBlock(1, "0xprev1")
	b2 := NewBlock(1, "0xprev2")
	h1 := CalculateHash(&b1)
	h2 := CalculateHash(&b2)
	if h1 == h2 {
		t.Error("different blocks should produce different hashes")
	}
}

func TestCalculateHash_Length(t *testing.T) {
	b := NewBlock(1, "0xprev")
	h := CalculateHash(&b)
	// "0x" + 64 hex chars
	if len(h) != 66 {
		t.Errorf("expected hash length 66, got %d: %q", len(h), h)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Transaction hash
// ─────────────────────────────────────────────────────────────────────────────

func TestCalculateTransactionHash_Deterministic(t *testing.T) {
	tx := Transaction{
		From:     "0x1111111111111111111111111111111111111111",
		To:       "0x2222222222222222222222222222222222222222",
		Value:    big.NewInt(1000),
		ChainID:  uint64(constantset.ChainID),
		GasPrice: 10,
	}
	h1 := CalculateTransactionHash(tx)
	h2 := CalculateTransactionHash(tx)
	if h1 != h2 {
		t.Error("transaction hash should be deterministic")
	}
}

func TestCalculateTransactionHash_HasPrefix(t *testing.T) {
	tx := Transaction{
		From:  "0x1111111111111111111111111111111111111111",
		To:    "0x2222222222222222222222222222222222222222",
		Value: big.NewInt(100),
	}
	h := CalculateTransactionHash(tx)
	if !strings.HasPrefix(h, "0x") {
		t.Errorf("tx hash should start with 0x, got %q", h)
	}
}

func TestCalculateTransactionHash_UniquePerTx(t *testing.T) {
	tx1 := Transaction{From: "0x1111111111111111111111111111111111111111", To: "0x2222222222222222222222222222222222222222", Value: big.NewInt(100)}
	tx2 := Transaction{From: "0x1111111111111111111111111111111111111111", To: "0x2222222222222222222222222222222222222222", Value: big.NewInt(200)}
	if CalculateTransactionHash(tx1) == CalculateTransactionHash(tx2) {
		t.Error("different transactions should have different hashes")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Address validation
// ─────────────────────────────────────────────────────────────────────────────

func TestValidateAddress_Valid(t *testing.T) {
	valid := []string{
		"0x1234567890abcdef1234567890abcdef12345678",
		"0xAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		"0x0000000000000000000000000000000000000000",
	}
	for _, addr := range valid {
		if !ValidateAddress(addr) {
			t.Errorf("expected address %q to be valid", addr)
		}
	}
}

func TestValidateAddress_Invalid(t *testing.T) {
	// ValidateAddress checks: HasPrefix("0x") && len==42
	invalid := []string{
		"",
		"0x123",                                        // too short
		"1234567890abcdef1234567890abcdef12345678",     // no prefix
		"0x1234567890abcdef1234567890abcdef123456789",  // too long (43 chars)
		"0X1234567890abcdef1234567890abcdef12345678",   // uppercase X prefix
	}
	for _, addr := range invalid {
		if ValidateAddress(addr) {
			t.Errorf("expected address %q to be invalid", addr)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Gas limit adjustment
// ─────────────────────────────────────────────────────────────────────────────

func TestCalculateNextGasLimit_NoBlocks(t *testing.T) {
	bc := newTestBlockchain()
	bc.Blocks = []*Block{} // empty chain
	limit := bc.CalculateNextGasLimit()
	if limit != MaxGasLimit {
		t.Errorf("no blocks: expected MaxGasLimit %d, got %d", MaxGasLimit, limit)
	}
}

func TestCalculateNextGasLimit_FullBlock_Increases(t *testing.T) {
	bc := newTestBlockchain()
	parent := &Block{
		BlockNumber: 1,
		GasLimit:    1000000,
		GasUsed:     900000, // > 75% → increase
	}
	bc.Blocks = []*Block{parent}
	limit := bc.CalculateNextGasLimit()
	if limit <= parent.GasLimit {
		t.Errorf("full block should increase gas limit: parent=%d new=%d", parent.GasLimit, limit)
	}
}

func TestCalculateNextGasLimit_EmptyBlock_Decreases(t *testing.T) {
	bc := newTestBlockchain()
	parent := &Block{
		BlockNumber: 1,
		GasLimit:    100_000_000, // well above MinGasLimit (21000000) so decrease keeps us in bounds
		GasUsed:     5_000_000,  // < 50% of 100M → decrease
	}
	bc.Blocks = []*Block{parent}
	limit := bc.CalculateNextGasLimit()
	if limit >= parent.GasLimit {
		t.Errorf("empty block should decrease gas limit: parent=%d new=%d", parent.GasLimit, limit)
	}
}

func TestCalculateNextGasLimit_Bounds(t *testing.T) {
	bc := newTestBlockchain()
	// Max bound: block way over capacity
	parent := &Block{GasLimit: MaxGasLimit, GasUsed: MaxGasLimit}
	bc.Blocks = []*Block{parent}
	limit := bc.CalculateNextGasLimit()
	if limit > MaxGasLimit {
		t.Errorf("gas limit exceeded maximum: %d", limit)
	}
	// Min bound: nearly empty block at minimum
	parent2 := &Block{GasLimit: MinGasLimit, GasUsed: 0}
	bc.Blocks = []*Block{parent2}
	limit2 := bc.CalculateNextGasLimit()
	if limit2 < MinGasLimit {
		t.Errorf("gas limit below minimum: %d", limit2)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Base fee
// ─────────────────────────────────────────────────────────────────────────────

func TestCalculateBaseFee_NoBlocks(t *testing.T) {
	bc := newTestBlockchain()
	bc.Blocks = []*Block{}
	fee := bc.CalculateBaseFee()
	if fee != uint64(constantset.InitialBaseFee) {
		t.Errorf("no blocks: expected InitialBaseFee %d, got %d", constantset.InitialBaseFee, fee)
	}
}

func TestCalculateBaseFee_GenesisBlock(t *testing.T) {
	bc := newTestBlockchain()
	// Genesis block number = 0
	fee := bc.CalculateBaseFee()
	if fee != uint64(constantset.InitialBaseFee) {
		t.Errorf("genesis block: expected InitialBaseFee %d, got %d", constantset.InitialBaseFee, fee)
	}
}

func TestCalculateBaseFee_BoundsEnforced(t *testing.T) {
	bc := newTestBlockchain()
	bc.Blocks = []*Block{
		{BlockNumber: 0, CurrentHash: "0x0"},
		{BlockNumber: 1, GasLimit: 1000, GasUsed: 1000, BaseFee: 9999999999},
	}
	fee := bc.CalculateBaseFee()
	if fee > uint64(constantset.MaxBaseFee) {
		t.Errorf("base fee %d exceeds MaxBaseFee %d", fee, constantset.MaxBaseFee)
	}
	if fee < uint64(constantset.MinBaseFee) {
		t.Errorf("base fee %d below MinBaseFee %d", fee, constantset.MinBaseFee)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Account balances
// ─────────────────────────────────────────────────────────────────────────────

func TestAddAccountBalance_Sets(t *testing.T) {
	bc := newTestBlockchain()
	addr := "0x1111111111111111111111111111111111111111"
	bc.AddAccountBalance(addr, big.NewInt(5000))
	bal := bc.CheckBalance(addr)
	if bal.Cmp(big.NewInt(5000)) != 0 {
		t.Errorf("expected balance 5000, got %s", bal.String())
	}
}

func TestAddAccountBalance_Accumulates(t *testing.T) {
	bc := newTestBlockchain()
	addr := "0x2222222222222222222222222222222222222222"
	bc.AddAccountBalance(addr, big.NewInt(100))
	bc.AddAccountBalance(addr, big.NewInt(200))
	bal := bc.CheckBalance(addr)
	if bal.Cmp(big.NewInt(300)) != 0 {
		t.Errorf("expected balance 300, got %s", bal.String())
	}
}

func TestCheckBalance_ZeroForUnknown(t *testing.T) {
	bc := newTestBlockchain()
	bal := bc.CheckBalance("0x9999999999999999999999999999999999999999")
	if bal.Sign() != 0 {
		t.Errorf("unknown address should have zero balance, got %s", bal.String())
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Transaction pool
// ─────────────────────────────────────────────────────────────────────────────

func TestRecordRecentTx_AddsTx(t *testing.T) {
	bc := newTestBlockchain()
	tx := &Transaction{
		TxHash:    "0xabc",
		From:      "0x1111111111111111111111111111111111111111",
		To:        "0x2222222222222222222222222222222222222222",
		Timestamp: uint64(time.Now().Unix()),
	}
	bc.RecordRecentTx(tx)
	if len(bc.RecentTxs) != 1 {
		t.Errorf("expected 1 recent tx, got %d", len(bc.RecentTxs))
	}
}

func TestRecordRecentTx_Deduplication(t *testing.T) {
	bc := newTestBlockchain()
	tx := &Transaction{TxHash: "0xsame"}
	bc.RecordRecentTx(tx)
	bc.RecordRecentTx(tx)
	if len(bc.RecentTxs) != 1 {
		t.Errorf("duplicate tx should be deduplicated, got %d entries", len(bc.RecentTxs))
	}
}

func TestRecordRecentTx_NewestFirst(t *testing.T) {
	bc := newTestBlockchain()
	tx1 := &Transaction{TxHash: "0xfirst"}
	tx2 := &Transaction{TxHash: "0xsecond"}
	bc.RecordRecentTx(tx1)
	bc.RecordRecentTx(tx2)
	if bc.RecentTxs[0].TxHash != "0xsecond" {
		t.Errorf("newest tx should be first, got %q", bc.RecentTxs[0].TxHash)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Block votes & pending blocks
// ─────────────────────────────────────────────────────────────────────────────

func TestAddBlockVote(t *testing.T) {
	bc := newTestBlockchain()
	bc.AddBlockVote("0xblockhash", "0xvalidator1")
	bc.AddBlockVote("0xblockhash", "0xvalidator2")
	if len(bc.BlockVotes["0xblockhash"]) != 2 {
		t.Errorf("expected 2 votes, got %d", len(bc.BlockVotes["0xblockhash"]))
	}
}

func TestAddBlockVote_NoDuplicates(t *testing.T) {
	bc := newTestBlockchain()
	bc.AddBlockVote("0xblockhash", "0xvalidator1")
	bc.AddBlockVote("0xblockhash", "0xvalidator1")
	if len(bc.BlockVotes["0xblockhash"]) != 1 {
		t.Errorf("expected 1 unique vote, got %d", len(bc.BlockVotes["0xblockhash"]))
	}
}

func TestAddPendingBlock(t *testing.T) {
	bc := newTestBlockchain()
	b := &Block{BlockNumber: 2, CurrentHash: "0xhash2"}
	bc.AddPendingBlock(b)
	if _, ok := bc.PendingBlocks["0xhash2"]; !ok {
		t.Error("block should be in PendingBlocks")
	}
}

func TestAddPendingBlock_NoDuplicate(t *testing.T) {
	bc := newTestBlockchain()
	b := &Block{BlockNumber: 2, CurrentHash: "0xhash2"}
	bc.AddPendingBlock(b)
	bc.AddPendingBlock(b)
	if len(bc.PendingBlocks) != 1 {
		t.Errorf("expected 1 pending block, got %d", len(bc.PendingBlocks))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Amount helpers
// ─────────────────────────────────────────────────────────────────────────────

func TestNewAmountFromUint64(t *testing.T) {
	v := NewAmountFromUint64(12345)
	if v.Uint64() != 12345 {
		t.Errorf("expected 12345, got %s", v.String())
	}
}

func TestCopyAmount_NilSafe(t *testing.T) {
	c := CopyAmount(nil)
	if c == nil || c.Sign() != 0 {
		t.Error("CopyAmount(nil) should return zero big.Int")
	}
}

func TestCopyAmount_Independence(t *testing.T) {
	orig := big.NewInt(100)
	copy := CopyAmount(orig)
	copy.Add(copy, big.NewInt(1))
	if orig.Cmp(big.NewInt(100)) != 0 {
		t.Error("modifying copy should not affect original")
	}
}

func TestAmountString_Nil(t *testing.T) {
	if AmountString(nil) != "0" {
		t.Error("AmountString(nil) should return \"0\"")
	}
}

func TestAmountString_Value(t *testing.T) {
	if AmountString(big.NewInt(9999)) != "9999" {
		t.Error("AmountString should return decimal representation")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Transaction gas cost
// ─────────────────────────────────────────────────────────────────────────────

func TestTransactionGasCost_Base(t *testing.T) {
	tx := &Transaction{Data: []byte{}}
	cost := tx.CalculateGasCost()
	if cost != uint64(constantset.MinGas) {
		t.Errorf("base gas cost should be %d, got %d", constantset.MinGas, cost)
	}
}

func TestTransactionGasCost_WithData(t *testing.T) {
	tx := &Transaction{Data: make([]byte, 10)}
	cost := tx.CalculateGasCost()
	expected := uint64(constantset.MinGas + 10*constantset.GasPerByte)
	if cost != expected {
		t.Errorf("expected gas cost %d, got %d", expected, cost)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// GetAccountNonce
// ─────────────────────────────────────────────────────────────────────────────

func TestGetAccountNonce_NoHistory(t *testing.T) {
	bc := newTestBlockchain()
	nonce := bc.GetAccountNonce("0x1111111111111111111111111111111111111111")
	if nonce != 0 {
		t.Errorf("expected nonce 0 for new address, got %d", nonce)
	}
}

func TestGetAccountNonce_FromPool(t *testing.T) {
	bc := newTestBlockchain()
	from := "0x1111111111111111111111111111111111111111"
	bc.Transaction_pool = append(bc.Transaction_pool, &Transaction{
		From:  from,
		Nonce: 5,
	})
	nonce := bc.GetAccountNonce(from)
	if nonce != 6 {
		t.Errorf("expected nonce 6 (5+1), got %d", nonce)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// VerifyTransaction – system/internal transactions bypass all checks
// ─────────────────────────────────────────────────────────────────────────────

func TestVerifyTransaction_SystemTxAlwaysPasses(t *testing.T) {
	bc := newTestBlockchain()
	systemTx := &Transaction{
		From:     "0x0000000000000000000000000000000000000000",
		To:       "0x1111111111111111111111111111111111111111",
		Value:    big.NewInt(0),
		IsSystem: true,
		Type:     "reward",
		ChainID:  uint64(constantset.ChainID),
	}
	if !bc.VerifyTransaction(systemTx) {
		t.Error("system transaction should always pass verification")
	}
}

func TestVerifyTransaction_StakeTxPasses(t *testing.T) {
	bc := newTestBlockchain()
	tx := &Transaction{
		From:    "0x1111111111111111111111111111111111111111",
		To:      "0x2222222222222222222222222222222222222222",
		Value:   big.NewInt(0),
		Type:    "stake",
		ChainID: uint64(constantset.ChainID),
	}
	if !bc.VerifyTransaction(tx) {
		t.Error("stake transaction should pass as system tx")
	}
}

func TestVerifyTransaction_MissingFrom_Fails(t *testing.T) {
	bc := newTestBlockchain()
	tx := &Transaction{
		From:      "",
		To:        "0x2222222222222222222222222222222222222222",
		Value:     big.NewInt(100),
		ChainID:   uint64(constantset.ChainID),
		Timestamp: uint64(time.Now().Unix()),
	}
	if bc.VerifyTransaction(tx) {
		t.Error("transaction with empty From should fail verification")
	}
}

func TestVerifyTransaction_InvalidChainID_Fails(t *testing.T) {
	bc := newTestBlockchain()
	tx := &Transaction{
		From:      "0x1111111111111111111111111111111111111111",
		To:        "0x2222222222222222222222222222222222222222",
		Value:     big.NewInt(100),
		ChainID:   9999, // wrong chain
		Timestamp: uint64(time.Now().Unix()),
		GasPrice:  100,
	}
	if bc.VerifyTransaction(tx) {
		t.Error("transaction with wrong ChainID should fail verification")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// AverageBlockTime
// ─────────────────────────────────────────────────────────────────────────────

func TestCalculateAverageBlockTime_NotEnoughBlocks(t *testing.T) {
	bc := newTestBlockchain()
	avg := bc.CalculateAverageBlockTime()
	if avg != 0 {
		t.Errorf("single block should give 0 average, got %f", avg)
	}
}

func TestCalculateAverageBlockTime_TwoBlocks(t *testing.T) {
	bc := newTestBlockchain()
	bc.Blocks = []*Block{
		{BlockNumber: 0, TimeStamp: 1000},
		{BlockNumber: 1, TimeStamp: 1002},
	}
	avg := bc.CalculateAverageBlockTime()
	if avg != 2.0 {
		t.Errorf("expected avg block time 2, got %f", avg)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TrimInMemoryBlocks
// ─────────────────────────────────────────────────────────────────────────────

func TestTrimInMemoryBlocks(t *testing.T) {
	bc := newTestBlockchain()
	for i := 0; i < 10; i++ {
		bc.Blocks = append(bc.Blocks, &Block{BlockNumber: uint64(i)})
	}
	bc.TrimInMemoryBlocks(5)
	if len(bc.Blocks) != 5 {
		t.Errorf("expected 5 blocks after trim, got %d", len(bc.Blocks))
	}
}

func TestTrimInMemoryBlocks_NoOpIfSmall(t *testing.T) {
	bc := newTestBlockchain()
	// Only has genesis (1 block)
	bc.TrimInMemoryBlocks(100)
	if len(bc.Blocks) != 1 {
		t.Errorf("should not trim when blocks < keepN, got %d", len(bc.Blocks))
	}
}
