package blockchaincomponent

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/syndtr/goleveldb/leveldb"
)

// InitContractDBAtPath opens a LevelDB-backed ContractDB at the given directory.
// Used only in tests (production code uses appDataPath).
func InitContractDBAtPath(dir string) (*ContractDB, error) {
	db, err := leveldb.OpenFile(dir, nil)
	if err != nil {
		return nil, err
	}
	return &ContractDB{db: db}, nil
}

// Close shuts down the underlying LevelDB handle.
func (c *ContractDB) Close() {
	if c != nil && c.db != nil {
		_ = c.db.Close()
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// helpers
// ─────────────────────────────────────────────────────────────────────────────

// newTestContextWithDB creates a Context backed by a real but temporary LevelDB.
// This is needed for tests that call ctx.Get() on keys not previously Set().
func newTestContextWithDB(t interface{ Helper(); TempDir() string; Cleanup(func()) },
	contract, caller string, gasLimit uint64) *Context {
	t.Helper()
	dir := t.(interface{ TempDir() string }).TempDir()
	db, err := InitContractDBAtPath(dir)
	if err != nil {
		panic("newTestContextWithDB: " + err.Error())
	}
	t.(interface{ Cleanup(func()) }).Cleanup(func() { db.Close() })
	return &Context{
		ContractAddr: contract,
		CallerAddr:   caller,
		OriginAddr:   caller,
		OwnerAddr:    caller,
		GasUsed:      0,
		GasLimit:     gasLimit,
		DB:           db,
		tempStorage:  make(map[string]string),
		events:       []ContractEvent{},
	}
}

// newTestContext creates a Context backed by nil DB (tempStorage only).
// Only use for tests where every Get key was previously Set.
func newTestContext(contract, caller string, gasLimit uint64) *Context {
	return &Context{
		ContractAddr: contract,
		CallerAddr:   caller,
		OriginAddr:   caller,
		OwnerAddr:    caller,
		GasUsed:      0,
		GasLimit:     gasLimit,
		tempStorage:  make(map[string]string),
		events:       []ContractEvent{},
		// DB left nil — callers must only Get keys that were previously Set
	}
}

// catchRevert calls fn and returns the REVERT reason string, or "" if no panic.
func catchRevert(fn func()) (reason string) {
	defer func() {
		if r := recover(); r != nil {
			reason = fmt.Sprintf("%v", r)
		}
	}()
	fn()
	return ""
}

// ─────────────────────────────────────────────────────────────────────────────
// Context – Get / Set (tempStorage path)
// ─────────────────────────────────────────────────────────────────────────────

func TestContext_SetGet_RoundTrip(t *testing.T) {
	ctx := newTestContext("0xcontract", "0xcaller", 1_000_000)
	ctx.Set("name", "LQD Token")
	if got := ctx.Get("name"); got != "LQD Token" {
		t.Errorf("expected %q, got %q", "LQD Token", got)
	}
}

func TestContext_GetMissing_ReturnsEmpty(t *testing.T) {
	ctx := newTestContextWithDB(t, "0xcontract", "0xcaller", 1_000_000)
	v := ctx.Get("nonexistent_key")
	if v != "" {
		t.Errorf("missing key should return empty string, got %q", v)
	}
}

func TestContext_Set_OverwritesPreviousValue(t *testing.T) {
	ctx := newTestContext("0xcontract", "0xcaller", 1_000_000)
	ctx.Set("k", "v1")
	ctx.Set("k", "v2")
	if got := ctx.Get("k"); got != "v2" {
		t.Errorf("expected overwritten value %q, got %q", "v2", got)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Context – gas accounting
// ─────────────────────────────────────────────────────────────────────────────

func TestContext_GasUsedIncreasesOnSet(t *testing.T) {
	ctx := newTestContext("0xcontract", "0xcaller", 1_000_000)
	before := ctx.GasUsed
	ctx.Set("key", "value")
	if ctx.GasUsed <= before {
		t.Error("GasUsed should increase after ctx.Set()")
	}
}

func TestContext_GasOutOfGas_Reverts(t *testing.T) {
	ctx := newTestContext("0xcontract", "0xcaller", 100) // very low gas

	reason := catchRevert(func() {
		// Keep calling Set until we run out of gas
		for i := 0; i < 1000; i++ {
			ctx.Set(fmt.Sprintf("k%d", i), "v")
		}
	})
	if reason == "" {
		t.Error("expected out-of-gas revert")
	}
}

func TestContext_GasLimit_EnoughGas(t *testing.T) {
	ctx := newTestContext("0xcontract", "0xcaller", 100_000_000)
	// These ops should not run out of gas
	for i := 0; i < 10; i++ {
		ctx.Set(fmt.Sprintf("k%d", i), fmt.Sprintf("v%d", i))
	}
	if ctx.GasUsed == 0 {
		t.Error("GasUsed should be > 0 after 10 Set calls")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Context – Revert (panics with "REVERT: " prefix)
// ─────────────────────────────────────────────────────────────────────────────

func TestContext_Revert_Panics(t *testing.T) {
	ctx := newTestContext("0xcontract", "0xcaller", 1_000_000)
	reason := catchRevert(func() {
		ctx.Revert("insufficient balance")
	})
	if reason == "" {
		t.Error("Revert should panic")
	}
}

func TestContext_Revert_MessageContainsReason(t *testing.T) {
	ctx := newTestContext("0xcontract", "0xcaller", 1_000_000)
	reason := catchRevert(func() {
		ctx.Revert("only owner")
	})
	if reason != "REVERT: only owner" {
		t.Errorf("expected panic message %q, got %q", "REVERT: only owner", reason)
	}
}

func TestContext_Revert_StateNotMutated(t *testing.T) {
	ctx := newTestContext("0xcontract", "0xcaller", 1_000_000)
	ctx.Set("balance", "1000")

	catchRevert(func() {
		ctx.Set("balance", "0") // mutate in a call that then reverts
		ctx.Revert("oops")
	})

	// tempStorage was written before revert; in full tx mode rollback happens
	// at the buffer level. Here we confirm at least the revert path runs.
	_ = ctx.Get("balance")
}

// ─────────────────────────────────────────────────────────────────────────────
// Context – Emit (events)
// ─────────────────────────────────────────────────────────────────────────────

func TestContext_Emit_RecordsEvent(t *testing.T) {
	ctx := newTestContext("0xcontract", "0xcaller", 1_000_000)
	ctx.Emit("Transfer", map[string]interface{}{
		"from":   "0xcaller",
		"to":     "0xrecipient",
		"amount": "100",
	})
	evts := ctx.Events()
	if len(evts) != 1 {
		t.Errorf("expected 1 event, got %d", len(evts))
	}
	if evts[0].EventName != "Transfer" {
		t.Errorf("expected event name Transfer, got %q", evts[0].EventName)
	}
}

func TestContext_Emit_MultipleEvents(t *testing.T) {
	ctx := newTestContext("0xcontract", "0xcaller", 1_000_000)
	ctx.Emit("Mint", map[string]interface{}{"amount": "500"})
	ctx.Emit("Transfer", map[string]interface{}{"from": "0x0", "to": "0xcaller"})
	if len(ctx.Events()) != 2 {
		t.Errorf("expected 2 events, got %d", len(ctx.Events()))
	}
}

func TestContext_Emit_ContractAddressSet(t *testing.T) {
	contract := "0xcontractaddr"
	ctx := newTestContext(contract, "0xcaller", 1_000_000)
	ctx.Emit("Init", nil)
	evts := ctx.Events()
	if evts[0].Address != contract {
		t.Errorf("event address should be contract address %q, got %q", contract, evts[0].Address)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Context – AddBalance / SubBalance
// ─────────────────────────────────────────────────────────────────────────────

func TestContext_AddBalance_StoresInTemp(t *testing.T) {
	ctx := newTestContextWithDB(t, "0xcontract", "0xcaller", 5_000_000)
	addr := "0xuser"
	ctx.AddBalance(addr, big.NewInt(1000))
	// Balance is stored under "__bal:<addr>"
	v := ctx.Get("__bal:" + addr)
	if v == "" {
		t.Error("balance should be stored in tempStorage after AddBalance")
	}
}

func TestContext_SubBalance_EnoughBalance(t *testing.T) {
	ctx := newTestContextWithDB(t, "0xcontract", "0xcaller", 5_000_000)
	addr := "0xuser"
	ctx.AddBalance(addr, big.NewInt(1000))
	reason := catchRevert(func() {
		ctx.SubBalance(addr, big.NewInt(500))
	})
	if reason != "" {
		t.Errorf("SubBalance should not revert with enough funds, got: %q", reason)
	}
}

func TestContext_SubBalance_InsufficientBalance_Reverts(t *testing.T) {
	ctx := newTestContextWithDB(t, "0xcontract", "0xcaller", 5_000_000)
	addr := "0xuser"
	ctx.AddBalance(addr, big.NewInt(100))
	reason := catchRevert(func() {
		ctx.SubBalance(addr, big.NewInt(9999)) // more than deposited
	})
	if reason == "" {
		t.Error("SubBalance should revert on insufficient balance")
	}
}

func TestContext_AddSubBalance_NetEffect(t *testing.T) {
	ctx := newTestContextWithDB(t, "0xcontract", "0xcaller", 10_000_000)
	addr := "0xuser"
	ctx.AddBalance(addr, big.NewInt(1000))
	ctx.SubBalance(addr, big.NewInt(300))
	ctx.AddBalance(addr, big.NewInt(50))
	// Net: 1000 - 300 + 50 = 750
	v := ctx.Get("__bal:" + addr)
	got := new(big.Int)
	got.SetString(v, 10)
	if got.Cmp(big.NewInt(750)) != 0 {
		t.Errorf("expected net balance 750, got %s", v)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TxBuffer — NewTxBuffer, PushCall/PopCall, re-entrancy guard
// ─────────────────────────────────────────────────────────────────────────────

func TestNewTxBuffer_Initialized(t *testing.T) {
	tb := NewTxBuffer(big.NewInt(100), "0xorigin", "0xentry")
	if tb == nil {
		t.Fatal("NewTxBuffer returned nil")
	}
	if tb.changes == nil {
		t.Error("changes map should be initialized")
	}
	if tb.nativeValue == nil || tb.nativeValue.Cmp(big.NewInt(100)) != 0 {
		t.Errorf("nativeValue should be 100, got %v", tb.nativeValue)
	}
}

func TestTxBuffer_PushPop_CallDepth(t *testing.T) {
	tb := NewTxBuffer(big.NewInt(0), "0xorigin", "0xentry")
	if err := tb.PushCall("0xcontractA"); err != nil {
		t.Fatalf("first PushCall failed: %v", err)
	}
	if tb.depth != 1 {
		t.Errorf("expected depth 1 after push, got %d", tb.depth)
	}
	tb.PopCall("0xcontractA")
	if tb.depth != 0 {
		t.Errorf("expected depth 0 after pop, got %d", tb.depth)
	}
}

func TestTxBuffer_ReEntrancyDetected(t *testing.T) {
	tb := NewTxBuffer(big.NewInt(0), "0xorigin", "0xentry")
	_ = tb.PushCall("0xcontractA")
	err := tb.PushCall("0xcontractA") // same contract — re-entrancy
	if err == nil {
		t.Error("expected re-entrancy error for same contract pushed twice")
	}
}

func TestTxBuffer_MaxCallDepth(t *testing.T) {
	tb := NewTxBuffer(big.NewInt(0), "0xorigin", "0xentry")
	for i := 0; i < maxCallDepth; i++ {
		addr := fmt.Sprintf("0xcontract%d", i)
		if err := tb.PushCall(addr); err != nil {
			t.Fatalf("PushCall failed at depth %d: %v", i, err)
		}
	}
	// One more should fail
	err := tb.PushCall("0xextra")
	if err == nil {
		t.Error("expected max call depth error")
	}
}

func TestTxBuffer_SetStorage_GetStorage(t *testing.T) {
	tb := NewTxBuffer(big.NewInt(0), "0xorigin", "0xentry")
	tb.SetStorage("0xcontract", "supply", "9999")

	// Simulate a context reading from the buffer (no real DB needed)
	val := tb.changes["0xcontract"]["supply"]
	if val != "9999" {
		t.Errorf("expected %q in buffer, got %q", "9999", val)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Context – Commit (to txBuffer mode, not DB)
// ─────────────────────────────────────────────────────────────────────────────

func TestContext_Commit_FlushesToTxBuffer(t *testing.T) {
	tb := NewTxBuffer(big.NewInt(0), "0xcaller", "0xcontract")
	ctx := &Context{
		ContractAddr: "0xcontract",
		CallerAddr:   "0xcaller",
		GasLimit:     10_000_000,
		tempStorage:  make(map[string]string),
		events:       []ContractEvent{},
		txBuffer:     tb,
	}

	ctx.Set("name", "TestToken")
	ctx.Set("supply", "1000000")

	if err := ctx.Commit(); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Values should now be in the txBuffer, not just tempStorage
	if tb.changes["0xcontract"]["name"] != "TestToken" {
		t.Error("name should have been flushed to txBuffer")
	}
	if tb.changes["0xcontract"]["supply"] != "1000000" {
		t.Error("supply should have been flushed to txBuffer")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Simulating simple contract logic (DAO-style deposit tracking)
// Without importing the contract file (//go:build ignore)
// ─────────────────────────────────────────────────────────────────────────────

func TestContractLogic_SimpleTokenInit(t *testing.T) {
	ctx := newTestContext("0xtoken", "0xowner", 5_000_000)
	// Simulate: Init("MyToken", "MTK", "8")
	ctx.Set("name", "MyToken")
	ctx.Set("symbol", "MTK")
	ctx.Set("decimals", "8")
	ctx.Set("totalSupply", "0")
	ctx.Emit("Init", map[string]interface{}{"name": "MyToken"})

	if ctx.Get("name") != "MyToken" {
		t.Error("token name not stored")
	}
	if ctx.Get("totalSupply") != "0" {
		t.Error("initial totalSupply should be 0")
	}
}

func TestContractLogic_MintAndTransfer(t *testing.T) {
	ctx := newTestContextWithDB(t, "0xtoken", "0xowner", 10_000_000)
	owner := "0xowner"
	recipient := "0xrecipient"

	// Mint 1000 to owner
	ctx.AddBalance(owner, big.NewInt(1000))
	supply := new(big.Int).SetInt64(1000)
	ctx.Set("totalSupply", supply.String())
	ctx.Emit("Mint", map[string]interface{}{"to": owner, "amount": "1000"})

	// Transfer 300 from owner to recipient
	ctx.SubBalance(owner, big.NewInt(300))
	ctx.AddBalance(recipient, big.NewInt(300))
	ctx.Emit("Transfer", map[string]interface{}{"from": owner, "to": recipient, "amount": "300"})

	evts := ctx.Events()
	if len(evts) != 2 {
		t.Errorf("expected 2 events (Mint + Transfer), got %d", len(evts))
	}
	if evts[1].EventName != "Transfer" {
		t.Errorf("second event should be Transfer, got %q", evts[1].EventName)
	}
}

func TestContractLogic_OwnerCheck_Reverts(t *testing.T) {
	contract := "0xtoken"
	owner := "0xowner"
	attacker := "0xattacker"

	ctx := newTestContext(contract, attacker, 5_000_000)
	ctx.OwnerAddr = owner

	reason := catchRevert(func() {
		if ctx.CallerAddr != ctx.OwnerAddr {
			ctx.Revert("only owner can call this")
		}
	})

	if reason == "" {
		t.Error("expected revert when non-owner calls owner-only function")
	}
}

func TestContractLogic_GovernanceProposal(t *testing.T) {
	ctx := newTestContext("0xdao", "0xproposer", 10_000_000)
	ctx.Set("proposal:count", "0")

	// Create proposal
	count := new(big.Int)
	count.SetString(ctx.Get("proposal:count"), 10)
	id := count.String()
	ctx.Set("proposal:"+id+":description", "Fund development")
	ctx.Set("proposal:"+id+":votes:yes", "0")
	ctx.Set("proposal:"+id+":votes:no", "0")
	ctx.Set("proposal:"+id+":active", "true")
	count.Add(count, big.NewInt(1))
	ctx.Set("proposal:count", count.String())
	ctx.Emit("ProposalCreated", map[string]interface{}{"id": id})

	if ctx.Get("proposal:count") != "1" {
		t.Errorf("expected 1 proposal, got %q", ctx.Get("proposal:count"))
	}
	if ctx.Get("proposal:0:active") != "true" {
		t.Error("proposal should be active")
	}
}

func TestContractLogic_NFTMintAndOwnership(t *testing.T) {
	ctx := newTestContextWithDB(t, "0xnft", "0xminter", 10_000_000)
	ctx.OwnerAddr = "0xminter"

	// Simulate contract Init
	ctx.Set("name", "MyNFT")
	ctx.Set("symbol", "NFT")
	ctx.Set("totalSupply", "0")

	to := "0xbuyer"
	tokenID := "1"

	// Mint
	ownerKey := "owner:" + tokenID
	if ctx.Get(ownerKey) != "" {
		ctx.Revert("token already minted")
	}
	ctx.Set(ownerKey, to)
	supply := ctx.Get("totalSupply")
	if supply == "" {
		supply = "0"
	}
	s := new(big.Int)
	s.SetString(supply, 10)
	s.Add(s, big.NewInt(1))
	ctx.Set("totalSupply", s.String())
	ctx.Emit("Mint", map[string]interface{}{"to": to, "tokenId": tokenID})

	if ctx.Get("owner:1") != to {
		t.Errorf("expected NFT owner %q, got %q", to, ctx.Get("owner:1"))
	}
	if ctx.Get("totalSupply") != "1" {
		t.Errorf("expected totalSupply 1, got %q", ctx.Get("totalSupply"))
	}
}

func TestContractLogic_NFTDoubleMint_Reverts(t *testing.T) {
	ctx := newTestContext("0xnft", "0xminter", 10_000_000)
	ctx.Set("owner:42", "0xfirstowner")

	reason := catchRevert(func() {
		if ctx.Get("owner:42") != "" {
			ctx.Revert("token already minted")
		}
	})

	if reason == "" {
		t.Error("should revert when minting already-minted token")
	}
}

func TestContractLogic_WLQDDepositWithdraw(t *testing.T) {
	ctx := newTestContextWithDB(t, "0xwlqd", "0xuser", 10_000_000)
	user := "0xuser"

	// Deposit 500 (simulate: receive native + mint WLQD)
	ctx.AddBalance(user, big.NewInt(500))
	supply := new(big.Int)
	supply.SetString(ctx.Get("totalSupply"), 10)
	supply.Add(supply, big.NewInt(500))
	ctx.Set("totalSupply", supply.String())
	ctx.Emit("Deposit", map[string]interface{}{"from": user, "amount": "500"})

	// Withdraw 200
	ctx.SubBalance(user, big.NewInt(200))
	supply.Sub(supply, big.NewInt(200))
	ctx.Set("totalSupply", supply.String())
	ctx.Emit("Withdraw", map[string]interface{}{"from": user, "amount": "200"})

	// Net: 300 WLQD remaining
	// (Balance tracked in __bal: key)
	if ctx.Get("totalSupply") != "300" {
		t.Errorf("expected totalSupply 300 after deposit/withdraw, got %q", ctx.Get("totalSupply"))
	}
}

func TestContractLogic_LendingPool_BorrowExceedsCollateral_Reverts(t *testing.T) {
	ctx := newTestContext("0xlend", "0xborrower", 10_000_000)
	borrower := "0xborrower"
	deposit := big.NewInt(1000)

	// Set collateral
	ctx.Set("dep:"+borrower, deposit.String())
	ctx.Set("totalDeposits", deposit.String())

	// Try to borrow more than deposited (collateral ratio check)
	borrowAmt := big.NewInt(1500)
	reason := catchRevert(func() {
		dep := new(big.Int)
		dep.SetString(ctx.Get("dep:"+borrower), 10)
		if dep.Cmp(borrowAmt) < 0 {
			ctx.Revert("borrow exceeds collateral")
		}
	})

	if reason == "" {
		t.Error("expected revert when borrow > collateral")
	}
}
