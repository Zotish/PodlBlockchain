package blockchaincomponent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"plugin"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	constantset "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/ConstantSet"
	"github.com/syndtr/goleveldb/leveldb"
)

// CONTRACT   STRUCT

type LQDContractEngine struct {
	DB       *ContractDB
	EventDB  *EventDB
	Registry *ContractRegistry
	Pipeline *ExecutionPipeline
}

func NewLQDContractEngine() (*LQDContractEngine, error) {

	cdb, err := InitContractDB()
	if err != nil {
		return nil, err
	}

	edb, err := InitEventDB()
	if err != nil {
		return nil, err
	}

	reg := NewContractRegistry(cdb, edb)
	pipe := NewExecutionPipeline(reg)

	return &LQDContractEngine{
		DB:       cdb,
		EventDB:  edb,
		Registry: reg,
		Pipeline: pipe,
	}, nil
}

// DB LAYER

type ContractDB struct {
	db             *leveldb.DB
	parent         *ContractDB
	overlay        map[string][]byte
	overlayDeletes map[string]bool
	overlayMu      sync.RWMutex
}
type Contract struct {
	Address    string
	Type       string
	ABI        []ABIEntry
	InitParams []string
	SourceCode string
	Bytecode   string
	PluginPath string
	State      map[string]interface{}
}

func (db *EventDB) GetEventsFromDB(address string) []ContractEvent {
	iter := db.db.NewIterator(nil, nil)
	defer iter.Release()

	out := []ContractEvent{}
	//prefix := "event:"

	for iter.Next() {
		val := iter.Value()
		var ev ContractEvent
		json.Unmarshal(val, &ev)

		if ev.Address == address {
			out = append(out, ev)
		}
	}

	return out
}
func (db *EventDB) SaveEventToDB(event ContractEvent) error {
	key := fmt.Sprintf("event:%s:%d", event.Address, event.Timestamp)
	b, _ := json.Marshal(event)
	return db.db.Put([]byte(key), b, nil)
}

func ensureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}

func InitContractDB() (*ContractDB, error) {
	base := appDataDir()
	if err := ensureDir(base); err != nil {
		return nil, err
	}

	path := appDataPath("contracts_db")
	d, err := leveldb.OpenFile(path, nil)
	if err != nil {
		return nil, err
	}
	return &ContractDB{db: d}, nil
}

func (c *ContractDB) Write(key string, val []byte) error {
	if c == nil {
		return fmt.Errorf("nil contract db")
	}
	if c.overlay != nil {
		c.overlayMu.Lock()
		defer c.overlayMu.Unlock()
		c.overlay[key] = append([]byte(nil), val...)
		delete(c.overlayDeletes, key)
		return nil
	}
	return c.db.Put([]byte(key), val, nil)
}
func (c *ContractDB) Read(key string) ([]byte, error) {
	if c == nil {
		return nil, leveldb.ErrNotFound
	}
	if c.overlay != nil {
		c.overlayMu.RLock()
		if c.overlayDeletes[key] {
			c.overlayMu.RUnlock()
			return nil, leveldb.ErrNotFound
		}
		if value, ok := c.overlay[key]; ok {
			out := append([]byte(nil), value...)
			c.overlayMu.RUnlock()
			return out, nil
		}
		c.overlayMu.RUnlock()
		return c.parent.Read(key)
	}
	return c.db.Get([]byte(key), nil)
}
func (c *ContractDB) Delete(key string) error {
	if c == nil {
		return fmt.Errorf("nil contract db")
	}
	if c.overlay != nil {
		c.overlayMu.Lock()
		defer c.overlayMu.Unlock()
		delete(c.overlay, key)
		c.overlayDeletes[key] = true
		return nil
	}
	return c.db.Delete([]byte(key), nil)
}

// NewOverlayContractDB creates a copy-on-write contract-state view. It is used
// by incoming block replay so unfinalized or invalid blocks never mutate the
// durable LevelDB.
func NewOverlayContractDB(parent *ContractDB) *ContractDB {
	return &ContractDB{parent: parent, overlay: make(map[string][]byte), overlayDeletes: make(map[string]bool)}
}

func (c *ContractDB) FlushOverlay() error {
	if c == nil || c.overlay == nil || c.parent == nil {
		return fmt.Errorf("contract db is not an overlay")
	}
	c.overlayMu.RLock()
	defer c.overlayMu.RUnlock()
	if c.parent.overlay != nil {
		for key := range c.overlayDeletes {
			if err := c.parent.Delete(key); err != nil {
				return err
			}
		}
		for key, value := range c.overlay {
			if err := c.parent.Write(key, value); err != nil {
				return err
			}
		}
		return nil
	}
	batch := new(leveldb.Batch)
	for key := range c.overlayDeletes {
		batch.Delete([]byte(key))
	}
	for key, value := range c.overlay {
		batch.Put([]byte(key), value)
	}
	return c.parent.db.Write(batch, nil)
}

func (c *ContractDB) SaveContractMetadata(addr string, meta *ContractMetadata) error {
	b, _ := json.Marshal(meta)
	return c.Write("contract:"+addr+":meta", b)
}

func (c *ContractDB) LoadContractMetadata(addr string) (*ContractMetadata, error) {
	b, err := c.Read("contract:" + addr + ":meta")
	if err != nil {
		return nil, err
	}
	var m ContractMetadata
	json.Unmarshal(b, &m)
	return &m, nil
}

func (c *ContractDB) SaveStorage(addr, key, val string) error {
	return c.Write("contract:"+addr+":storage:"+key, []byte(val))
}

// SaveStorageBatch atomically applies storage changes across one or more
// contracts. Durable databases use one LevelDB batch; replay overlays publish
// the complete write set while holding one lock.
func (c *ContractDB) SaveStorageBatch(writes map[string]map[string]string) error {
	if c == nil {
		return fmt.Errorf("nil contract db")
	}
	if c.overlay != nil {
		c.overlayMu.Lock()
		defer c.overlayMu.Unlock()
		for addr, values := range writes {
			for key, value := range values {
				storageKey := "contract:" + addr + ":storage:" + key
				c.overlay[storageKey] = []byte(value)
				delete(c.overlayDeletes, storageKey)
			}
		}
		return nil
	}
	if c.db == nil {
		return fmt.Errorf("contract database unavailable")
	}
	batch := new(leveldb.Batch)
	for addr, values := range writes {
		for key, value := range values {
			batch.Put([]byte("contract:"+addr+":storage:"+key), []byte(value))
		}
	}
	return c.db.Write(batch, nil)
}

func (c *ContractDB) LoadStorage(addr, key string) (string, error) {
	b, err := c.Read("contract:" + addr + ":storage:" + key)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ListContractAddresses returns all deployed contract addresses by scanning
// metadata keys in the format "contract:{addr}:meta".
func (c *ContractDB) ListContractAddresses() []string {
	if c.overlay != nil {
		seen := map[string]bool{}
		out := []string{}
		if c.parent != nil {
			for _, addr := range c.parent.ListContractAddresses() {
				seen[addr] = true
				out = append(out, addr)
			}
		}
		c.overlayMu.RLock()
		for key := range c.overlay {
			if strings.HasPrefix(key, "contract:") && strings.HasSuffix(key, ":meta") {
				addr := key[len("contract:") : len(key)-len(":meta")]
				if addr != "" && !seen[addr] && !c.overlayDeletes[key] {
					seen[addr] = true
					out = append(out, addr)
				}
			}
		}
		c.overlayMu.RUnlock()
		sort.Strings(out)
		return out
	}
	iter := c.db.NewIterator(nil, nil)
	defer iter.Release()

	const prefix = "contract:"
	const suffix = ":meta"
	seen := make(map[string]bool)
	var addrs []string

	for iter.Next() {
		k := string(iter.Key())
		if strings.HasPrefix(k, prefix) && strings.HasSuffix(k, suffix) {
			addr := k[len(prefix) : len(k)-len(suffix)]
			if addr != "" && !seen[addr] {
				seen[addr] = true
				addrs = append(addrs, addr)
			}
		}
	}
	return addrs
}

func (c *ContractDB) LoadAllStorage(addr string) (map[string]string, error) {
	if c.overlay != nil {
		out := map[string]string{}
		if c.parent != nil {
			parentValues, err := c.parent.LoadAllStorage(addr)
			if err != nil {
				return nil, err
			}
			for key, value := range parentValues {
				out[key] = value
			}
		}
		prefix := "contract:" + addr + ":storage:"
		c.overlayMu.RLock()
		for key := range c.overlayDeletes {
			if strings.HasPrefix(key, prefix) {
				delete(out, key[len(prefix):])
			}
		}
		for key, value := range c.overlay {
			if strings.HasPrefix(key, prefix) {
				out[key[len(prefix):]] = string(value)
			}
		}
		c.overlayMu.RUnlock()
		return out, nil
	}
	iter := c.db.NewIterator(nil, nil)
	defer iter.Release()

	prefix := "contract:" + addr + ":storage:"
	out := make(map[string]string)

	for iter.Next() {
		k := string(iter.Key())
		if strings.HasPrefix(k, prefix) {
			sub := k[len(prefix):]
			out[sub] = string(iter.Value())
		}
	}
	return out, nil
}

//  CORE TYPES

type ContractMetadata struct {
	Address             string   `json:"address"`
	Type                string   `json:"type"` // plugin | gocode | dsl | builtin
	Owner               string   `json:"owner"`
	ABI                 []byte   `json:"abi"`
	Timestamp           int64    `json:"timestamp"`
	Pool                bool     `json:"pool"`
	PoolType            string   `json:"pool_type,omitempty"`
	PluginPath          string   `json:"plugin_path,omitempty"`
	RuntimeFingerprint  string   `json:"runtime_fingerprint,omitempty"`
	RuntimeFingerprints []string `json:"runtime_fingerprints,omitempty"`
	RuntimeMigratedFrom string   `json:"runtime_migrated_from,omitempty"`
	RuntimeMigratedAt   int64    `json:"runtime_migrated_at,omitempty"`
	Code                []byte   `json:"code,omitempty"`
	BuiltinName         string   `json:"builtin_name,omitempty"`
}

var (
	pluginRuntimeFingerprintOnce sync.Once
	pluginRuntimeFingerprint     string
	pluginRuntimeFingerprintErr  error
)

func CurrentPluginRuntimeFingerprint() (string, error) {
	pluginRuntimeFingerprintOnce.Do(func() {
		pluginRuntimeFingerprint, pluginRuntimeFingerprintErr = computePluginRuntimeFingerprint()
	})
	return pluginRuntimeFingerprint, pluginRuntimeFingerprintErr
}

func computePluginRuntimeFingerprint() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	paths := []string{
		"go.mod",
		"go.sum",
		"main.go",
	}
	dirs := []string{
		"BlockchainComponent",
		"BlockchainServer",
		"WalletServer",
		"AggregatorServer",
		"ConstantSet",
		"contract",
	}

	files := make([]string, 0, 256)
	for _, path := range paths {
		full := filepath.Join(cwd, path)
		if info, err := os.Stat(full); err == nil && !info.IsDir() {
			files = append(files, path)
		}
	}
	for _, dir := range dirs {
		root := filepath.Join(cwd, dir)
		_ = filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil || info == nil || info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(info.Name(), ".go") || strings.HasSuffix(info.Name(), "_test.go") {
				return nil
			}
			rel, err := filepath.Rel(cwd, path)
			if err != nil {
				return nil
			}
			files = append(files, filepath.ToSlash(rel))
			return nil
		})
	}

	sort.Strings(files)
	hasher := sha256.New()
	for _, rel := range files {
		content, err := os.ReadFile(filepath.Join(cwd, rel))
		if err != nil {
			return "", err
		}
		_, _ = hasher.Write([]byte(rel))
		_, _ = hasher.Write([]byte{'\n'})
		_, _ = hasher.Write(content)
		_, _ = hasher.Write([]byte{'\n'})
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

type SmartContractState struct {
	Address   string            `json:"address"`
	Balance   string            `json:"balance"`
	Storage   map[string]string `json:"storage"`
	IsActive  bool              `json:"is_active"`
	CreatedAt int64             `json:"created_at"`
}

type ContractRecord struct {
	Metadata *ContractMetadata   `json:"metadata"`
	State    *SmartContractState `json:"state"`
}

type ContractEvent struct {
	EventName string                 `json:"event_name"`
	Data      map[string]interface{} `json:"data"`
	Address   string                 `json:"address"`
	Timestamp int64                  `json:"timestamp"`
}

type ContractExecutionResult struct {
	Success bool              `json:"success"`
	GasUsed uint64            `json:"gas_used"`
	Output  string            `json:"output"`
	Storage map[string]string `json:"storage"`
	Events  []ContractEvent   `json:"events"`
}

// ── TxBuffer — atomic, re-entrancy-safe state buffer ─────────────────────────
//
// All state mutations within a single TX (including cross-contract calls) are
// written here. The buffer is committed to the DB only when the top-level call
// succeeds. On revert/panic the buffer is simply discarded — full rollback.

const maxCallDepth = 16

// nativeSend represents a pending native LQD transfer queued by ctx.SendNative.
type nativeSend struct {
	from string
	to   string
	amt  *big.Int
}

type TxBuffer struct {
	changes      map[string]map[string]string // contract addr → storage key → value
	events       []ContractEvent              // all events from all nested calls
	callStack    map[string]bool              // re-entrancy guard
	depth        int                          // current call depth
	nativeValue  *big.Int                     // LQD sent with the top-level TX (msg.value)
	nativeEscrow *big.Int                     // unclaimed native LQD attached to this TX
	nativeEntry  string                       // top-level contract address receiving any unclaimed native LQD
	nativeCredit map[string]*big.Int          // staged native LQD credits per contract address
	nativeSends  []nativeSend                 // queued native LQD transfers (applied after commit)
	originAddr   string                       // original external caller for this TX
}

func NewTxBuffer(value *big.Int, origin, entry string) *TxBuffer {
	v := new(big.Int)
	if value != nil {
		v.Set(value)
	}
	return &TxBuffer{
		changes:      make(map[string]map[string]string),
		events:       []ContractEvent{},
		callStack:    make(map[string]bool),
		nativeValue:  new(big.Int).Set(v),
		nativeEscrow: new(big.Int).Set(v),
		nativeEntry:  strings.ToLower(strings.TrimSpace(entry)),
		nativeCredit: make(map[string]*big.Int),
		originAddr:   origin,
	}
}

func (tb *TxBuffer) creditNative(addr string, amt *big.Int) {
	if amt == nil || amt.Sign() <= 0 {
		return
	}
	addr = strings.ToLower(strings.TrimSpace(addr))
	if addr == "" {
		return
	}
	if tb.nativeCredit[addr] == nil {
		tb.nativeCredit[addr] = big.NewInt(0)
	}
	tb.nativeCredit[addr].Add(tb.nativeCredit[addr], amt)
}

func (tb *TxBuffer) nativeCreditsWithRemainder() map[string]*big.Int {
	out := make(map[string]*big.Int, len(tb.nativeCredit)+1)
	for addr, amt := range tb.nativeCredit {
		out[addr] = new(big.Int).Set(amt)
	}
	if tb.nativeEscrow != nil && tb.nativeEscrow.Sign() > 0 && tb.nativeEntry != "" {
		if out[tb.nativeEntry] == nil {
			out[tb.nativeEntry] = big.NewInt(0)
		}
		out[tb.nativeEntry].Add(out[tb.nativeEntry], tb.nativeEscrow)
	}
	return out
}

func (tb *TxBuffer) ensureNativeFlows(blockchain *Blockchain_struct) error {
	credits := tb.nativeCreditsWithRemainder()
	available := make(map[string]*big.Int, len(credits)+len(tb.nativeSends))

	getAvailable := func(addr string) *big.Int {
		addr = strings.ToLower(strings.TrimSpace(addr))
		if bal, ok := available[addr]; ok {
			return bal
		}
		base, _ := blockchain.getAccountBalance(addr)
		if base == nil {
			base = big.NewInt(0)
		}
		if credit := credits[addr]; credit != nil {
			base = new(big.Int).Add(base, credit)
		} else {
			base = new(big.Int).Set(base)
		}
		available[addr] = base
		return base
	}

	for _, send := range tb.nativeSends {
		from := strings.ToLower(strings.TrimSpace(send.from))
		to := strings.ToLower(strings.TrimSpace(send.to))
		if getAvailable(from).Cmp(send.amt) < 0 {
			return fmt.Errorf("native send failed: insufficient balance in %s", from)
		}
		available[from].Sub(available[from], send.amt)
		getAvailable(to).Add(available[to], send.amt)
	}
	return nil
}

func (tb *TxBuffer) applyNativeFlows(blockchain *Blockchain_struct) error {
	for addr, amt := range tb.nativeCreditsWithRemainder() {
		if amt.Sign() > 0 {
			blockchain.addAccountBalance(addr, amt)
		}
	}
	for _, send := range tb.nativeSends {
		if !blockchain.subAccountBalance(send.from, send.amt) {
			return fmt.Errorf("native send failed: insufficient balance in %s", send.from)
		}
		blockchain.addAccountBalance(send.to, send.amt)
	}
	return nil
}

// GetStorage reads from buffer first, falls back to persistent DB.
func (tb *TxBuffer) GetStorage(contractAddr, key string, db *ContractDB) string {
	if c, ok := tb.changes[contractAddr]; ok {
		if v, ok2 := c[key]; ok2 {
			return v
		}
	}
	val, _ := db.LoadStorage(contractAddr, key)
	return val
}

// SetStorage writes into the buffer (never touches the DB).
func (tb *TxBuffer) SetStorage(contractAddr, key, val string) {
	if tb.changes[contractAddr] == nil {
		tb.changes[contractAddr] = make(map[string]string)
	}
	tb.changes[contractAddr][key] = val
}

// CommitToDB flushes all buffered changes to the persistent DB atomically.
func (tb *TxBuffer) CommitToDB(db *ContractDB) error {
	for addr, kv := range tb.changes {
		for k, v := range kv {
			if err := db.SaveStorage(addr, k, v); err != nil {
				return fmt.Errorf("commit failed for %s/%s: %w", addr, k, err)
			}
		}
	}
	return nil
}

// PushCall registers a contract as active; returns error on re-entrancy or depth overflow.
func (tb *TxBuffer) PushCall(addr string) error {
	if tb.depth >= maxCallDepth {
		return fmt.Errorf("max call depth (%d) exceeded", maxCallDepth)
	}
	if tb.callStack[addr] {
		return fmt.Errorf("re-entrancy detected for contract %s", addr)
	}
	tb.callStack[addr] = true
	tb.depth++
	return nil
}

// PopCall unregisters a contract from the active set.
func (tb *TxBuffer) PopCall(addr string) {
	delete(tb.callStack, addr)
	if tb.depth > 0 {
		tb.depth--
	}
}

//  CONTEXT — SANDBOXED EXECUTION ENVIRONMENT

type Context struct {
	ContractAddr string
	CallerAddr   string
	OriginAddr   string
	OwnerAddr    string
	BlockTime    int64
	GasUsed      uint64
	GasLimit     uint64
	DB           *ContractDB
	tempStorage  map[string]string
	events       []ContractEvent
	callFunc     func(target string, fn string, args []string) (*ContractExecutionResult, error)
	deployFunc   func(newAddr, pluginPath, owner string) error
	txBuffer     *TxBuffer // nil for read-only calls; set for state-changing TXs
}

func NewContext(addr, caller, origin, owner string, db *ContractDB, gas uint64) *Context {
	return &Context{
		ContractAddr: addr,
		CallerAddr:   caller,
		OriginAddr:   origin,
		OwnerAddr:    owner,
		BlockTime:    time.Now().Unix(),
		GasUsed:      0,
		GasLimit:     gas,
		DB:           db,
		tempStorage:  make(map[string]string),
		events:       []ContractEvent{},
	}
}

func (ctx *Context) Get(key string) string {
	// 1. Uncommitted writes in this call frame
	if v, ok := ctx.tempStorage[key]; ok {
		return v
	}
	// 2. Writes from earlier calls in this TX (atomic buffer)
	if ctx.txBuffer != nil {
		return ctx.txBuffer.GetStorage(ctx.ContractAddr, key, ctx.DB)
	}
	// 3. Persistent DB (read-only path)
	val, _ := ctx.DB.LoadStorage(ctx.ContractAddr, key)
	return val
}

func (ctx *Context) Set(key, value string) {
	ctx.consumeGas(200)
	ctx.tempStorage[key] = value
}

func (ctx *Context) balanceOf(addr string) *big.Int {
	key := "__bal:" + addr
	if v, ok := ctx.tempStorage[key]; ok {
		return parseBig(v)
	}
	raw, _ := ctx.DB.LoadStorage(ctx.ContractAddr, key)
	return parseBig(raw)
}

func (ctx *Context) AddBalance(addr string, amt *big.Int) {
	ctx.consumeGas(150)
	sum := new(big.Int).Add(ctx.balanceOf(addr), amt)
	ctx.tempStorage["__bal:"+addr] = sum.String()
}

func (ctx *Context) SubBalance(addr string, amt *big.Int) {
	ctx.consumeGas(150)
	b := ctx.balanceOf(addr)
	if b.Cmp(amt) < 0 {
		ctx.Revert("insufficient balance")
	}
	ctx.tempStorage["__bal:"+addr] = new(big.Int).Sub(b, amt).String()
}

func (ctx *Context) Emit(ev string, data map[string]interface{}) {
	ctx.consumeGas(500)
	event := ContractEvent{
		EventName: ev,
		Data:      data,
		Address:   ctx.ContractAddr,
		Timestamp: time.Now().Unix(),
	}
	// In atomic TX mode events are collected in the shared buffer so all
	// nested-call events appear together in the top-level result.
	if ctx.txBuffer != nil {
		ctx.txBuffer.events = append(ctx.txBuffer.events, event)
	} else {
		ctx.events = append(ctx.events, event)
	}
}

func (ctx *Context) Call(target, fn string, args []string) (*ContractExecutionResult, error) {
	ctx.consumeGas(10000)
	if ctx.callFunc == nil {
		return nil, fmt.Errorf("cross-call disabled")
	}
	return ctx.callFunc(target, fn, args)
}

// MsgValue returns the native LQD amount sent with this transaction (like msg.value in EVM).
// Returns 0 for read-only calls or when no value was attached.
func (ctx *Context) MsgValue() *big.Int {
	if ctx.txBuffer == nil || ctx.txBuffer.nativeValue == nil {
		return big.NewInt(0)
	}
	return new(big.Int).Set(ctx.txBuffer.nativeValue)
}

// ReceiveNative claims native LQD attached to the top-level TX for the current contract.
// This is used by native-aware contracts so routed calls can attribute msg.value to the
// contract that actually consumes it.
func (ctx *Context) ReceiveNative(amt *big.Int) {
	ctx.consumeGas(5000)
	if ctx.txBuffer == nil {
		ctx.Revert("ReceiveNative only available in state-changing TX mode")
	}
	if amt == nil || amt.Sign() <= 0 {
		ctx.Revert("ReceiveNative: amount must be > 0")
	}
	if ctx.txBuffer.nativeEscrow == nil || ctx.txBuffer.nativeEscrow.Cmp(amt) < 0 {
		ctx.Revert("msg.value less than required native LQD amount")
	}
	ctx.txBuffer.nativeEscrow.Sub(ctx.txBuffer.nativeEscrow, amt)
	ctx.txBuffer.creditNative(ctx.ContractAddr, amt)
}

// ReceiveNativeFromCaller queues a native LQD transfer from the immediate caller
// contract/account into the current contract. It enables safe contract-to-contract
// native flows, for example a strategy vault moving native LQD from its balance
// into a target DEX pair during an atomic rebalance.
func (ctx *Context) ReceiveNativeFromCaller(amt *big.Int) {
	ctx.consumeGas(5000)
	if ctx.txBuffer == nil {
		ctx.Revert("ReceiveNativeFromCaller only available in state-changing TX mode")
	}
	if amt == nil || amt.Sign() <= 0 {
		ctx.Revert("ReceiveNativeFromCaller: amount must be > 0")
	}
	from := strings.ToLower(strings.TrimSpace(ctx.CallerAddr))
	to := strings.ToLower(strings.TrimSpace(ctx.ContractAddr))
	if from == "" || to == "" {
		ctx.Revert("ReceiveNativeFromCaller: invalid native flow")
	}
	ctx.txBuffer.nativeSends = append(ctx.txBuffer.nativeSends, nativeSend{
		from: from,
		to:   to,
		amt:  new(big.Int).Set(amt),
	})
}

// SendNative queues a native LQD transfer from the contract to `to`.
// The transfer is applied atomically after the entire TX succeeds.
// Reverts if amount exceeds the contract's available native balance.
func (ctx *Context) SendNative(to string, amt *big.Int) {
	ctx.consumeGas(5000)
	if ctx.txBuffer == nil {
		ctx.Revert("SendNative only available in state-changing TX mode")
	}
	if amt == nil || amt.Sign() <= 0 {
		ctx.Revert("SendNative: amount must be > 0")
	}
	to = strings.ToLower(strings.TrimSpace(to))
	if to == "" {
		ctx.Revert("SendNative: invalid recipient")
	}
	ctx.txBuffer.nativeSends = append(ctx.txBuffer.nativeSends, nativeSend{
		from: ctx.ContractAddr,
		to:   to,
		amt:  new(big.Int).Set(amt),
	})
}

// DeployContract deploys a new contract cloned from the given plugin path at newAddr.
// The new contract is registered immediately so that ctx.Call(newAddr, ...) works in
// the same transaction. If the TX later reverts the orphan metadata is harmless because
// the factory's pairExists key is never committed to DB.
// owner is set to ctx.ContractAddr (the factory) by convention.
func (ctx *Context) DeployContract(newAddr, pluginPath string) {
	ctx.consumeGas(50000)
	if ctx.txBuffer == nil {
		ctx.Revert("DeployContract only available in state-changing TX mode")
	}
	if ctx.deployFunc == nil {
		ctx.Revert("DeployContract: deploy function not wired")
	}
	newAddr = strings.ToLower(strings.TrimSpace(newAddr))
	if newAddr == "" {
		ctx.Revert("DeployContract: invalid address")
	}
	if err := ctx.deployFunc(newAddr, pluginPath, ctx.ContractAddr); err != nil {
		ctx.Revert("DeployContract failed: " + err.Error())
	}
}

func (ctx *Context) consumeGas(n uint64) {
	ctx.GasUsed += n
	if ctx.GasUsed > ctx.GasLimit {
		ctx.Revert("out of gas")
	}
}

func (ctx *Context) Revert(reason string) {
	panic("REVERT: " + reason)
}

func (ctx *Context) Commit() error {
	if ctx.txBuffer != nil {
		// Atomic mode: flush tempStorage into shared buffer (NOT the DB yet).
		// The top-level ExecuteAtomic will commit the whole buffer on success.
		for k, v := range ctx.tempStorage {
			ctx.txBuffer.SetStorage(ctx.ContractAddr, k, v)
		}
		return nil
	}
	// Read-only / legacy path: write directly to DB.
	for k, v := range ctx.tempStorage {
		if err := ctx.DB.SaveStorage(ctx.ContractAddr, k, v); err != nil {
			return err
		}
	}
	return nil
}

func (ctx *Context) Events() []ContractEvent {
	return ctx.events
}

//  GO PLUGIN VM

type PluginContract struct {
	Instance any
	Methods  map[string]reflect.Method
}

type PluginVM struct {
	mu       sync.RWMutex
	plugins  map[string]*PluginContract
	byPath   map[string]*PluginContract
	byDigest map[string]*PluginContract
}

func NewPluginVM() *PluginVM {
	return &PluginVM{
		plugins:  make(map[string]*PluginContract),
		byPath:   make(map[string]*PluginContract),
		byDigest: make(map[string]*PluginContract),
	}
}

func (p *PluginVM) LoadPlugin(addr, path string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if path == "" {
		return fmt.Errorf("plugin path required")
	}
	if existing := p.byPath[path]; existing != nil {
		p.plugins[addr] = existing
		return nil
	}
	digest := ""
	if soBytes, err := os.ReadFile(path); err == nil {
		sum := sha256.Sum256(soBytes)
		digest = hex.EncodeToString(sum[:])
		if existing := p.byDigest[digest]; existing != nil {
			p.plugins[addr] = existing
			p.byPath[path] = existing
			return nil
		}
	}

	pl, err := plugin.Open(path)
	if err != nil {
		if strings.Contains(err.Error(), "plugin already loaded") {
			if existing := p.byPath[path]; existing != nil {
				p.plugins[addr] = existing
				return nil
			}
			if digest != "" {
				if existing := p.byDigest[digest]; existing != nil {
					p.plugins[addr] = existing
					p.byPath[path] = existing
					return nil
				}
			}
			return fmt.Errorf("plugin already loaded but no cached contract found for %s; rebuild with a unique plugin package path: %w", path, err)
		}
		if strings.Contains(err.Error(), "different version of package") {
			return fmt.Errorf("plugin binary is incompatible with the current runtime; recompile or redeploy this contract plugin: %w", err)
		}
		return err
	}

	sym, err := pl.Lookup("Contract")
	if err != nil {
		return fmt.Errorf("plugin missing `Contract` symbol: %v", err)
	}

	inst := reflect.ValueOf(sym).Elem().Interface()
	t := reflect.TypeOf(inst)

	methods := map[string]reflect.Method{}
	for i := 0; i < t.NumMethod(); i++ {
		m := t.Method(i)
		methods[m.Name] = m
		methods[strings.ToLower(m.Name)] = m
	}

	pc := &PluginContract{Instance: inst, Methods: methods}
	p.plugins[addr] = pc
	p.byPath[path] = pc
	if digest != "" {
		p.byDigest[digest] = pc
	}
	return nil
}

func (p *PluginVM) CallPlugin(addr, fn string, ctx *Context, args []string) (*ContractExecutionResult, error) {
	p.mu.RLock()
	pc := p.plugins[addr]
	p.mu.RUnlock()
	if pc == nil {
		return nil, fmt.Errorf("plugin not loaded")
	}

	m, ok := pc.Methods[fn]
	if !ok {
		// try lowercase key
		if mm, ok2 := pc.Methods[strings.ToLower(fn)]; ok2 {
			m = mm
			ok = true
		} else {
			// case-insensitive scan
			for name, cand := range pc.Methods {
				if strings.EqualFold(name, fn) {
					m = cand
					ok = true
					break
				}
			}
		}
	}
	if !ok {
		return nil, fmt.Errorf("method not found: %s", fn)
	}

	argv := []reflect.Value{reflect.ValueOf(pc.Instance), reflect.ValueOf(ctx)}
	for _, a := range args {
		argv = append(argv, reflect.ValueOf(a))
	}

	defer func() {
		if r := recover(); r != nil {
			ctx.Revert(fmt.Sprintf("panic: %v", r))
		}
	}()

	m.Func.Call(argv)

	_ = ctx.Commit()

	out := ""
	if v, ok := ctx.tempStorage["output"]; ok {
		out = v
	}

	return &ContractExecutionResult{
		Success: true,
		GasUsed: ctx.GasUsed,
		Output:  out,
		Storage: ctx.tempStorage,
		Events:  ctx.events,
	}, nil
}

func (p *PluginVM) GetPlugin(addr string) *PluginContract {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.plugins[addr]
}

//  INTERPRETER VM

type OpCode byte

const (
	OP_NOOP OpCode = iota
	OP_SET
	OP_GET
	OP_ADD
	OP_SUB
	OP_EQ
	OP_NEQ
	OP_JMP
	OP_JMPIF
	OP_CALL
	OP_REVERT
)

type Bytecode struct {
	Ops          []OpCode        `json:"ops"`
	Args         []string        `json:"args,omitempty"`
	Instructions []VMInstruction `json:"instructions"`
}

type VMInstruction struct {
	Op   OpCode   `json:"op"`
	Args []string `json:"args,omitempty"`
}

type InterpreterVM struct{}

func NewInterpreterVM() *InterpreterVM { return &InterpreterVM{} }

func (ivm *InterpreterVM) CompileGoSubset(src string) (*Bytecode, error) {
	out := &Bytecode{}
	lines := strings.FieldsFunc(src, func(r rune) bool { return r == '\n' || r == ';' })
	if len(lines) == 0 || len(lines) > 1024 {
		return nil, fmt.Errorf("hardened VM requires 1..1024 instructions")
	}
	definitions := map[string]struct {
		op       OpCode
		min, max int
	}{
		"SET": {OP_SET, 2, 2}, "GET": {OP_GET, 1, 1}, "ADD": {OP_ADD, 2, 2}, "SUB": {OP_SUB, 2, 2},
		"EQ": {OP_EQ, 2, 2}, "NEQ": {OP_NEQ, 2, 2}, "JMP": {OP_JMP, 1, 1}, "JMPIF": {OP_JMPIF, 1, 1},
		"CALL": {OP_CALL, 2, 2}, "REVERT": {OP_REVERT, 1, 32},
	}
	for lineNumber, line := range lines {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) == 0 {
			continue
		}
		definition, ok := definitions[strings.ToUpper(fields[0])]
		if !ok {
			return nil, fmt.Errorf("hardened VM line %d: unknown opcode %q", lineNumber+1, fields[0])
		}
		args := append([]string(nil), fields[1:]...)
		if len(args) < definition.min || len(args) > definition.max {
			return nil, fmt.Errorf("hardened VM line %d: invalid operand count", lineNumber+1)
		}
		if definition.op == OP_REVERT {
			args = []string{strings.Join(args, " ")}
		}
		for _, arg := range args {
			if len(arg) == 0 || len(arg) > 512 {
				return nil, fmt.Errorf("hardened VM line %d: operand length invalid", lineNumber+1)
			}
		}
		out.Instructions = append(out.Instructions, VMInstruction{Op: definition.op, Args: args})
		out.Ops = append(out.Ops, definition.op)
		out.Args = append(out.Args, args...)
	}
	if len(out.Instructions) == 0 {
		return nil, fmt.Errorf("hardened VM requires at least one non-empty instruction")
	}
	for pc, instruction := range out.Instructions {
		if instruction.Op != OP_JMP && instruction.Op != OP_JMPIF {
			continue
		}
		target, err := strconv.Atoi(instruction.Args[0])
		if err != nil || target < 0 || target >= len(out.Instructions) {
			return nil, fmt.Errorf("hardened VM instruction %d: jump target out of range", pc)
		}
	}
	return out, nil
}

func (ivm *InterpreterVM) ExecuteBytecode(addr string, bc *Bytecode, ctx *Context) (*ContractExecutionResult, error) {
	if bc == nil || ctx == nil || len(bc.Instructions) == 0 || len(bc.Instructions) > 1024 {
		return nil, fmt.Errorf("invalid hardened VM bytecode")
	}
	pc := 0
	steps := 0
	valueOf := func(operand string) *big.Int {
		if literal, ok := new(big.Int).SetString(operand, 10); ok {
			return literal
		}
		return parseBig(ctx.Get(operand))
	}
	for pc < len(bc.Instructions) {
		steps++
		if steps > 100000 {
			return nil, fmt.Errorf("hardened VM step limit exceeded")
		}
		ctx.consumeGas(50)
		instruction := bc.Instructions[pc]
		switch instruction.Op {
		case OP_SET:
			ctx.Set(instruction.Args[0], instruction.Args[1])
		case OP_GET:
			ctx.Set("output", ctx.Get(instruction.Args[0]))
		case OP_ADD:
			a, b := parseBig(ctx.Get(instruction.Args[0])), valueOf(instruction.Args[1])
			ctx.Set(instruction.Args[0], new(big.Int).Add(a, b).String())
		case OP_SUB:
			a, b := parseBig(ctx.Get(instruction.Args[0])), valueOf(instruction.Args[1])
			if a.Cmp(b) < 0 {
				return nil, fmt.Errorf("hardened VM unsigned subtraction underflow")
			}
			ctx.Set(instruction.Args[0], new(big.Int).Sub(a, b).String())
		case OP_EQ:
			if ctx.Get(instruction.Args[0]) == ctx.Get(instruction.Args[1]) {
				ctx.Set("__cmp", "1")
			} else {
				ctx.Set("__cmp", "0")
			}
		case OP_NEQ:
			if ctx.Get(instruction.Args[0]) != ctx.Get(instruction.Args[1]) {
				ctx.Set("__cmp", "1")
			} else {
				ctx.Set("__cmp", "0")
			}
		case OP_JMP:
			pc, _ = strconv.Atoi(instruction.Args[0])
			continue
		case OP_JMPIF:
			if ctx.Get("__cmp") == "1" {
				pc, _ = strconv.Atoi(instruction.Args[0])
				continue
			}
		case OP_CALL:
			target, fn := strings.ToLower(instruction.Args[0]), instruction.Args[1]
			if !ValidateAddress(target) || strings.TrimSpace(fn) == "" {
				return nil, fmt.Errorf("hardened VM invalid call target")
			}
			if _, err := ctx.Call(target, fn, []string{}); err != nil {
				return nil, err
			}
		case OP_REVERT:
			ctx.Revert(instruction.Args[0])
		default:
			return nil, fmt.Errorf("hardened VM invalid opcode")
		}
		pc++
	}
	if err := ctx.Commit(); err != nil {
		return nil, err
	}
	return &ContractExecutionResult{
		Success: true,
		GasUsed: ctx.GasUsed,
		Output:  ctx.Get("output"),
		Storage: ctx.tempStorage,
		Events:  ctx.events,
	}, nil
}

func parseBig(s string) *big.Int {
	s = strings.TrimSpace(s)
	if s == "" {
		return big.NewInt(0)
	}
	z := new(big.Int)
	if _, ok := z.SetString(s, 10); !ok {
		return big.NewInt(0)
	}
	return z
}

// SECTION 7: DSL VM

type DSLVM struct{}

func NewDSLVM() *DSLVM { return &DSLVM{} }

func (d *DSLVM) CompileDSL(src string) ([]string, error) {
	parts := strings.FieldsFunc(src, func(r rune) bool { return r == '\n' || r == ';' })
	if len(parts) == 0 || len(parts) > 1024 {
		return nil, fmt.Errorf("DSL requires 1..1024 statements")
	}
	out := make([]string, 0, len(parts))
	for i, raw := range parts {
		line := strings.TrimSpace(raw)
		valid := false
		switch {
		case strings.HasPrefix(line, "emit "):
			valid = len(strings.Fields(line)) == 2
		case strings.HasPrefix(line, "call "):
			body := strings.TrimSpace(strings.TrimPrefix(line, "call "))
			pieces := strings.Split(body, ".")
			valid = len(pieces) == 2 && ValidateAddress(strings.ToLower(pieces[0])) && strings.TrimSpace(pieces[1]) != ""
		case strings.Contains(line, "+="):
			kv := strings.SplitN(line, "+=", 2)
			_, numberOK := new(big.Int).SetString(strings.TrimSpace(kv[1]), 10)
			valid = strings.TrimSpace(kv[0]) != "" && numberOK
		case strings.Contains(line, "="):
			kv := strings.SplitN(line, "=", 2)
			valid = strings.TrimSpace(kv[0]) != "" && strings.TrimSpace(kv[1]) != ""
		}
		if !valid || len(line) > 1024 {
			return nil, fmt.Errorf("invalid DSL statement %d", i+1)
		}
		out = append(out, line)
	}
	return out, nil
}

func (d *DSLVM) ExecuteDSL(addr string, lines []string, ctx *Context) (*ContractExecutionResult, error) {

	for _, ln := range lines {
		ctx.consumeGas(50)

		// key=value
		if strings.Contains(ln, "=") && !strings.Contains(ln, "+=") {
			kv := strings.SplitN(ln, "=", 2)
			ctx.Set(strings.TrimSpace(kv[0]), strings.TrimSpace(kv[1]))
			continue
		}

		// key+=N
		if strings.Contains(ln, "+=") {
			kv := strings.SplitN(ln, "+=", 2)
			key := strings.TrimSpace(kv[0])
			cur := parseBig(ctx.Get(key))
			add := parseBig(strings.TrimSpace(kv[1]))
			ctx.Set(key, new(big.Int).Add(cur, add).String())
			continue
		}

		// emit X
		if strings.HasPrefix(ln, "emit ") {
			ev := strings.TrimSpace(strings.TrimPrefix(ln, "emit "))
			ctx.Emit(ev, map[string]interface{}{"msg": ev})
			continue
		}

		// call A.fn
		if strings.HasPrefix(ln, "call ") {
			body := strings.TrimSpace(strings.TrimPrefix(ln, "call "))
			parts := strings.Split(body, ".")
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid DSL call")
			}
			tgt := strings.ToLower(strings.TrimSpace(parts[0]))
			fn := strings.TrimSpace(parts[1])
			_, err := ctx.Call(tgt, fn, []string{})
			if err != nil {
				return nil, err
			}
			continue
		}
	}

	if err := ctx.Commit(); err != nil {
		return nil, err
	}

	return &ContractExecutionResult{
		Success: true,
		GasUsed: ctx.GasUsed,
		Storage: ctx.tempStorage,
		Events:  ctx.events,
	}, nil
}

// SECTION 8: ABI GENERATOR

type ABIEntry struct {
	Name    string   `json:"name"`
	Inputs  []string `json:"inputs"`
	Payable bool     `json:"payable"`
	Type    string   `json:"type"`
}

type ContractABI struct {
	Entries []ABIEntry `json:"entries"`
}

func GenerateABIForPlugin(pc *PluginContract) ([]byte, error) {

	abi := ContractABI{}

	for name, m := range pc.Methods {

		args := []string{}
		for i := 2; i < m.Type.NumIn(); i++ {
			args = append(args, m.Type.In(i).Name())
		}

		abi.Entries = append(abi.Entries, ABIEntry{
			Name:    name,
			Inputs:  args,
			Payable: false,
			Type:    "function",
		})
	}

	return json.Marshal(abi)
}

func GenerateABIForBytecode(_ *Bytecode) ([]byte, error) {
	abi := ContractABI{
		Entries: []ABIEntry{
			{Name: "Execute", Inputs: []string{"string..."}, Type: "function"},
		},
	}
	return json.Marshal(abi)
}

func GenerateABIForDSL() ([]byte, error) {
	abi := ContractABI{
		Entries: []ABIEntry{
			{Name: "Execute", Inputs: []string{"string..."}, Type: "function"},
		},
	}
	return json.Marshal(abi)
}

// EVENT DB

type EventDB struct {
	db *leveldb.DB
}

func (ep *ExecutionPipeline) ApplyContractCall(addr, caller, fn string, args []string) (*ContractExecutionResult, error) {
	return ep.Execute(addr, caller, fn, args, 5_000_000)
}

// ApplyContractCallWithValue is like ApplyContractCall but for state-changing calls with native value.
func (ep *ExecutionPipeline) ApplyContractCallWithValue(addr, caller, fn string, args []string, value *big.Int) (*ContractExecutionResult, error) {
	return ep.ExecuteAtomic(addr, caller, fn, args, 5_000_000, value)
}

func InitEventDB() (*EventDB, error) {
	base := appDataDir()
	if err := ensureDir(base); err != nil {
		return nil, err
	}

	path := appDataPath("contract_events_db")
	d, err := leveldb.OpenFile(path, nil)
	if err != nil {
		return nil, err
	}
	return &EventDB{db: d}, nil
}

func (e *EventDB) SaveEvent(block uint64, tx string, idx int, ev ContractEvent) error {
	b, _ := json.Marshal(ev)
	key := fmt.Sprintf("event:%d:%s:%d", block, tx, idx)
	return e.db.Put([]byte(key), b, nil)
}

func (e *EventDB) GetEventsByBlock(block uint64) ([]ContractEvent, error) {
	out := []ContractEvent{}
	iter := e.db.NewIterator(nil, nil)
	defer iter.Release()

	prefix := fmt.Sprintf("event:%d:", block)

	for iter.Next() {
		key := string(iter.Key())
		if strings.HasPrefix(key, prefix) {
			var ev ContractEvent
			json.Unmarshal(iter.Value(), &ev)
			out = append(out, ev)
		}
	}

	return out, nil
}

// CONTRACT REGISTRY

type ContractRegistry struct {
	DB         *ContractDB
	EventDB    *EventDB
	PluginVM   *PluginVM
	IVM        *InterpreterVM
	DSL        *DSLVM
	Blockchain *Blockchain_struct
}

func NewContractRegistry(cdb *ContractDB, edb *EventDB) *ContractRegistry {
	return &ContractRegistry{
		DB:         cdb,
		EventDB:    edb,
		PluginVM:   NewPluginVM(),
		IVM:        NewInterpreterVM(),
		DSL:        NewDSLVM(),
		Blockchain: &Blockchain_struct{},
	}
}

func (r *ContractRegistry) RegisterContract(meta *ContractMetadata, st *SmartContractState) error {

	if err := r.DB.SaveContractMetadata(meta.Address, meta); err != nil {
		return err
	}
	if err := r.DB.SaveStorage(meta.Address, "__bal:"+meta.Owner, st.Balance); err != nil {
		return err
	}

	for k, v := range st.Storage {
		r.DB.SaveStorage(meta.Address, k, v)
	}

	return nil
}

func (r *ContractRegistry) LoadContract(addr string) (*ContractRecord, error) {

	meta, err := r.DB.LoadContractMetadata(addr)
	if err != nil {
		return nil, err
	}

	storage, _ := r.DB.LoadAllStorage(addr)
	bal := parseBig(storage["__bal:"+meta.Owner])

	state := &SmartContractState{
		Address:   addr,
		Balance:   bal.String(),
		Storage:   storage,
		IsActive:  true,
		CreatedAt: time.Now().Unix(),
	}

	return &ContractRecord{Metadata: meta, State: state}, nil
}

// DeployClone registers a new contract at newAddr using the given plugin path.
// The new contract shares the same compiled plugin (.so) as the template but
// has its own isolated storage namespace in the DB.
func (r *ContractRegistry) DeployClone(newAddr, pluginPath, owner string) error {
	fingerprint, err := CurrentPluginRuntimeFingerprint()
	if err != nil {
		return fmt.Errorf("runtime fingerprint: %w", err)
	}
	meta := &ContractMetadata{
		Address:            newAddr,
		Owner:              owner,
		Type:               "plugin",
		PluginPath:         pluginPath,
		RuntimeFingerprint: fingerprint,
	}
	if err := r.DB.SaveContractMetadata(newAddr, meta); err != nil {
		return fmt.Errorf("save metadata: %w", err)
	}
	// Load plugin (Go plugin.Open caches same path — safe to call multiple times)
	return r.EnsurePluginLoaded(newAddr, meta)
}

func (r *ContractRegistry) LoadABI(addr string) ([]byte, error) {
	m, err := r.DB.LoadContractMetadata(addr)
	if err != nil {
		return nil, err
	}
	return m.ABI, nil
}

func (r *ContractRegistry) EnsurePluginLoaded(addr string, meta *ContractMetadata) error {

	if meta.Type != "plugin" {
		return nil
	}
	if r.PluginVM.GetPlugin(addr) != nil {
		return nil
	}
	currentFingerprint, err := CurrentPluginRuntimeFingerprint()
	if err != nil {
		return fmt.Errorf("runtime fingerprint: %w", err)
	}
	previousFingerprint := meta.RuntimeFingerprint

	if err := r.PluginVM.LoadPlugin(addr, meta.PluginPath); err != nil {
		loadErr := err
		if migrated, migrateErr := r.rebindLegacyBuiltinPlugin(addr, meta, currentFingerprint); migrateErr != nil {
			loadErr = fmt.Errorf("%w; builtin runtime migration failed: %v", err, migrateErr)
		} else if migrated {
			if retryErr := r.PluginVM.LoadPlugin(addr, meta.PluginPath); retryErr == nil {
				return nil
			} else {
				loadErr = retryErr
			}
		}
		if previousFingerprint != "" && previousFingerprint != currentFingerprint {
			return fmt.Errorf("legacy plugin load failed after runtime compatibility migration for %s (stored runtime %s, current runtime %s): %w", addr, previousFingerprint, currentFingerprint, loadErr)
		}
		return loadErr
	}

	if currentFingerprint != "" && (previousFingerprint == "" || previousFingerprint != currentFingerprint || !hasRuntimeFingerprint(meta, currentFingerprint)) {
		if previousFingerprint != "" && previousFingerprint != currentFingerprint {
			meta.RuntimeFingerprints = appendRuntimeFingerprint(meta.RuntimeFingerprints, previousFingerprint)
			meta.RuntimeMigratedFrom = previousFingerprint
			meta.RuntimeMigratedAt = time.Now().Unix()
		}
		meta.RuntimeFingerprints = appendRuntimeFingerprint(meta.RuntimeFingerprints, currentFingerprint)
		meta.RuntimeFingerprint = currentFingerprint
		if err := r.DB.SaveContractMetadata(addr, meta); err != nil {
			return fmt.Errorf("persist runtime compatibility metadata: %w", err)
		}
	}

	return nil
}

func (r *ContractRegistry) rebindLegacyBuiltinPlugin(addr string, meta *ContractMetadata, currentFingerprint string) (bool, error) {
	builtinName := inferBuiltinName(meta)
	if builtinName == "" {
		return false, nil
	}
	soBytes, err := findPrebuiltBuiltinPlugin(builtinName)
	if err != nil {
		return false, err
	}
	pluginDir := ContractArtifactsDir()
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		return false, err
	}
	safeAddr := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(addr)), "0x")
	if safeAddr == "" {
		safeAddr = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(meta.Address)), "0x")
	}
	if safeAddr == "" {
		safeAddr = "contract"
	}
	pluginPath := filepath.Join(pluginDir, fmt.Sprintf("%s_%s_runtime_%d.so", safeAddr, builtinName, time.Now().UnixNano()))
	if err := os.WriteFile(pluginPath, soBytes, 0644); err != nil {
		return false, err
	}

	previousFingerprint := meta.RuntimeFingerprint
	if previousFingerprint != "" && previousFingerprint != currentFingerprint {
		meta.RuntimeFingerprints = appendRuntimeFingerprint(meta.RuntimeFingerprints, previousFingerprint)
		meta.RuntimeMigratedFrom = previousFingerprint
		meta.RuntimeMigratedAt = time.Now().Unix()
	}
	if currentFingerprint != "" {
		meta.RuntimeFingerprints = appendRuntimeFingerprint(meta.RuntimeFingerprints, currentFingerprint)
		meta.RuntimeFingerprint = currentFingerprint
	}
	meta.PluginPath = pluginPath
	meta.BuiltinName = builtinName
	if err := r.DB.SaveContractMetadata(addr, meta); err != nil {
		return false, err
	}
	return true, nil
}

func inferBuiltinName(meta *ContractMetadata) string {
	if meta == nil {
		return ""
	}
	known := map[string]bool{
		"advanced_pool":        true,
		"bridge_token":         true,
		"dao_treasury":         true,
		"dex_factory":          true,
		"dex_pair":             true,
		"dex_router":           true,
		"dex_swap":             true,
		"insurance_vault":      true,
		"lending_pool":         true,
		"lqd20":                true,
		"nft_collection":       true,
		"strategy_vault":       true,
		"test_liquidity_token": true,
		"validator_bond":       true,
		"wlqd":                 true,
	}
	if name := strings.ToLower(strings.TrimSpace(meta.BuiltinName)); known[name] {
		return name
	}
	pathHint := strings.ToLower(filepath.Base(meta.PluginPath))
	for name := range known {
		if strings.Contains(pathHint, name) {
			return name
		}
	}
	abi := strings.ToLower(string(meta.ABI))
	switch {
	case strings.Contains(abi, "createpair") && strings.Contains(abi, "getpair"):
		return "dex_factory"
	case strings.Contains(abi, "getreserves") && strings.Contains(abi, "addliquidity"):
		return "dex_pair"
	case strings.Contains(abi, "getamountsout") || strings.Contains(abi, "swapexacttokensfortokens"):
		return "dex_router"
	case strings.Contains(abi, "balanceof") && strings.Contains(abi, "totalsupply") && strings.Contains(abi, "transfer"):
		return "lqd20"
	case strings.Contains(abi, "rebalance") && strings.Contains(abi, "keeper"):
		return "strategy_vault"
	}
	return ""
}

func findPrebuiltBuiltinPlugin(name string) ([]byte, error) {
	var roots []string
	if dir := strings.TrimSpace(os.Getenv("LQD_BUILTINS_DIR")); dir != "" {
		roots = append(roots, dir)
	}
	if cwd, err := os.Getwd(); err == nil {
		for dir := cwd; dir != ""; dir = filepath.Dir(dir) {
			roots = append(roots, filepath.Join(dir, "bin", "builtins"))
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
		}
	}
	seen := make(map[string]bool, len(roots))
	for _, root := range roots {
		if root == "" || seen[root] {
			continue
		}
		seen[root] = true
		path := filepath.Join(root, name+".so")
		if soBytes, err := os.ReadFile(path); err == nil && len(soBytes) > 0 {
			return soBytes, nil
		}
	}
	return nil, fmt.Errorf("prebuilt builtin plugin not found for %s", name)
}

func hasRuntimeFingerprint(meta *ContractMetadata, fingerprint string) bool {
	if fingerprint == "" {
		return true
	}
	if meta.RuntimeFingerprint == fingerprint {
		return true
	}
	for _, existing := range meta.RuntimeFingerprints {
		if existing == fingerprint {
			return true
		}
	}
	return false
}

func appendRuntimeFingerprint(fingerprints []string, fingerprint string) []string {
	if fingerprint == "" {
		return fingerprints
	}
	for _, existing := range fingerprints {
		if existing == fingerprint {
			return fingerprints
		}
	}
	return append(fingerprints, fingerprint)
}

// EXECUTION PIPELINE

type ExecutionPipeline struct {
	Registry *ContractRegistry
}

func NewExecutionPipeline(reg *ContractRegistry) *ExecutionPipeline {
	return &ExecutionPipeline{Registry: reg}
}

// Execute is the read-only call path (GetPoolInfo, BalanceOf, etc.).
// It commits directly to DB and does NOT use a TxBuffer.
func (ep *ExecutionPipeline) Execute(addr, caller, fn string, args []string, gas uint64) (res *ContractExecutionResult, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("contract panic: %v", r)
			res = nil
		}
	}()

	rec, err := ep.Registry.LoadContract(addr)
	if err != nil {
		return nil, err
	}

	ctx := NewContext(addr, caller, caller, rec.Metadata.Owner, ep.Registry.DB, gas)
	// No txBuffer — writes go directly to DB (read-only semantics).

	ctx.callFunc = func(tgt, method string, a []string) (*ContractExecutionResult, error) {
		return ep.Execute(tgt, addr, method, a, gas/2)
	}

	return ep.dispatchVM(addr, fn, ctx, args, rec)
}

// ExecuteAtomic is the state-changing TX path.
// ALL state changes (including cross-contract calls) are buffered and committed
// only when the entire call tree succeeds. On any revert the buffer is discarded.
// value is the native LQD attached to this TX (msg.value); pass nil for 0.
func (ep *ExecutionPipeline) ExecuteAtomic(addr, caller, fn string, args []string, gas uint64, value *big.Int) (res *ContractExecutionResult, err error) {
	return ep.ExecuteAtomicAt(addr, caller, fn, args, gas, value, time.Now().Unix())
}

// ExecuteAtomicAt is the deterministic execution entry used by block replay.
// Contract time is derived from the proposed block, never the receiver's wall
// clock.
func (ep *ExecutionPipeline) ExecuteAtomicAt(addr, caller, fn string, args []string, gas uint64, value *big.Int, blockTime int64) (res *ContractExecutionResult, err error) {
	txBuf := NewTxBuffer(value, caller, addr)
	res, err = ep.executeInner(addr, caller, fn, args, gas, txBuf, blockTime)
	if err != nil {
		return nil, err // buffer discarded — full rollback
	}

	if ep.Registry.Blockchain != nil {
		if err := txBuf.ensureNativeFlows(ep.Registry.Blockchain); err != nil {
			return nil, err
		}
	}

	// Commit all buffered storage changes atomically
	if commitErr := txBuf.CommitToDB(ep.Registry.DB); commitErr != nil {
		return nil, fmt.Errorf("atomic commit failed: %w", commitErr)
	}

	// Apply staged native LQD credits/sends after storage commit.
	if ep.Registry.Blockchain != nil {
		if err := txBuf.applyNativeFlows(ep.Registry.Blockchain); err != nil {
			return nil, err
		}
	}

	// Attach all collected events to the result
	res.Events = txBuf.events
	return res, nil
}

// ExecuteContractTxAt re-executes a contract transaction without producing
// relayer side effects or synthetic mempool event transactions. With an
// overlay ContractDB, its commit remains isolated until block finalization.
func (ep *ExecutionPipeline) ExecuteContractTxAt(tx *Transaction, blockTime int64) (*ContractExecutionResult, error) {
	if tx == nil {
		return nil, fmt.Errorf("nil transaction")
	}
	if tx.Type == "contract_create" {
		return &ContractExecutionResult{Success: true, Output: "contract created"}, nil
	}
	fn, args := tx.Function, tx.Args
	if fn == "" {
		parsedFn, parsedArgs, err := parseContractCallData(tx.Data)
		if err != nil {
			return nil, err
		}
		fn, args = parsedFn, parsedArgs
	}
	if fn == "" {
		return nil, fmt.Errorf("tx missing function selector")
	}
	return ep.ExecuteAtomicAt(tx.To, tx.From, fn, args, 5_000_000, tx.Value, blockTime)
}

// executeInner is the recursive implementation shared by ExecuteAtomic and
// cross-contract calls within the same TX.
func (ep *ExecutionPipeline) executeInner(addr, caller, fn string, args []string, gas uint64, txBuf *TxBuffer, blockTime int64) (res *ContractExecutionResult, err error) {
	defer func() {
		if r := recover(); r != nil {
			msg := fmt.Sprintf("%v", r)
			// Distinguish explicit REVERT from unexpected panics
			if strings.HasPrefix(msg, "REVERT:") {
				err = fmt.Errorf("%s", msg)
			} else {
				err = fmt.Errorf("contract panic: %s", msg)
			}
			res = nil
		}
	}()

	// Re-entrancy guard + depth check
	if pushErr := txBuf.PushCall(addr); pushErr != nil {
		return nil, pushErr
	}
	defer txBuf.PopCall(addr)

	rec, err := ep.Registry.LoadContract(addr)
	if err != nil {
		return nil, err
	}

	origin := caller
	if txBuf.originAddr != "" {
		origin = txBuf.originAddr
	}
	ctx := NewContext(addr, caller, origin, rec.Metadata.Owner, ep.Registry.DB, gas)
	ctx.BlockTime = blockTime
	ctx.txBuffer = txBuf // atomic mode

	// Cross-contract calls within this TX share the same buffer
	ctx.callFunc = func(tgt, method string, a []string) (*ContractExecutionResult, error) {
		return ep.executeInner(tgt, addr, method, a, gas/2, txBuf, blockTime)
	}

	// Contracts can deploy new contracts (e.g., factory creating pair contracts)
	ctx.deployFunc = func(newAddr, pluginPath, owner string) error {
		return ep.Registry.DeployClone(newAddr, pluginPath, owner)
	}

	return ep.dispatchVM(addr, fn, ctx, args, rec)
}

// dispatchVM routes to the correct VM for the contract type.
func (ep *ExecutionPipeline) dispatchVM(addr, fn string, ctx *Context, args []string, rec *ContractRecord) (*ContractExecutionResult, error) {
	switch rec.Metadata.Type {

	case "plugin":
		if err := ep.Registry.EnsurePluginLoaded(addr, rec.Metadata); err != nil {
			return nil, err
		}
		return ep.Registry.PluginVM.CallPlugin(addr, fn, ctx, args)

	case "gocode":
		bc, err := ep.Registry.IVM.CompileGoSubset(string(rec.Metadata.Code))
		if err != nil {
			return nil, err
		}
		return ep.Registry.IVM.ExecuteBytecode(addr, bc, ctx)

	case "dsl":
		code, err := ep.Registry.DSL.CompileDSL(string(rec.Metadata.Code))
		if err != nil {
			return nil, err
		}
		return ep.Registry.DSL.ExecuteDSL(addr, code, ctx)

	case "builtin":
		return nil, fmt.Errorf("builtin contract - native handler required")
	}

	return nil, fmt.Errorf("invalid contract type: %s", rec.Metadata.Type)
}

// SECTION 12: BLOCKCHAIN INTEGRATION

func (ep *ExecutionPipeline) ExecuteContractTx(tx *Transaction, block uint64) (*ContractExecutionResult, error) {

	if tx.Type == "contract_create" {
		return &ContractExecutionResult{Success: true, Output: "contract created"}, nil
	}

	fn := tx.Function
	args := tx.Args

	if fn == "" {
		parsedFn, parsedArgs, err := parseContractCallData(tx.Data)
		if err != nil {
			return nil, err
		}
		fn = parsedFn
		args = parsedArgs
	}
	if fn == "" {
		return nil, fmt.Errorf("tx missing function selector")
	}

	// Execute contract atomically — all state changes are buffered and only
	// committed to DB if the entire call tree succeeds (re-entrancy protected).
	// Pass tx.Value so contracts can read msg.value and send native LQD back.
	res, err := ep.ExecuteAtomic(tx.To, tx.From, fn, args, 5_000_000, tx.Value)
	if err != nil {
		return nil, err
	}

	// Process emitted events
	for i, ev := range res.Events {

		// Save to event DB
		ep.Registry.EventDB.SaveEvent(block, tx.TxHash, i, ev)

		//-----------------------------------------------
		// 🔥 CREATE A CONTRACT EVENT TRANSACTION
		//-----------------------------------------------
		eventTx := &Transaction{
			From:      ev.Address,
			To:        ev.Address,
			Type:      "contract_event",
			Function:  ev.EventName,
			Args:      mapToArgs(ev.Data),
			Timestamp: uint64(time.Now().Unix()),
			Status:    constantset.StatusPending,
			IsSystem:  true,
			Gas:       0,
			GasPrice:  0,
			ChainID:   uint64(constantset.ChainID),
		}

		eventTx.TxHash = CalculateTransactionHash(*eventTx)

		//-----------------------------------------------
		// 🔥 Push event transaction into mempool
		//-----------------------------------------------
		ep.Registry.Blockchain.Transaction_pool = append(
			ep.Registry.Blockchain.Transaction_pool,
			eventTx,
		)

		ep.Registry.Blockchain.RecordRecentTx(eventTx)

		// Bridge burn detection (LQD -> BSC)
		if ev.EventName == "Burn" {
			toBsc, _ := ev.Data["to_bsc"].(string)
			bscToken, _ := ev.Data["bsc"].(string)
			amountStr, _ := ev.Data["amount"].(string)
			if toBsc != "" && bscToken != "" && amountStr != "" {
				amt, err := NewAmountFromString(amountStr)
				if err == nil && amt.Sign() > 0 {
					ep.Registry.Blockchain.AddBridgeRequestBurn(tx.TxHash, bscToken, tx.From, toBsc, amt)
				}
			}
		}
	}

	return res, nil
}

func parseContractCallData(data []byte) (string, []string, error) {
	if len(data) == 0 {
		return "", nil, nil
	}

	// JSON form: {"fn":"transfer","args":["a","b"]}
	var payload struct {
		Fn   string   `json:"fn"`
		Args []string `json:"args"`
	}
	if data[0] == '{' {
		if err := json.Unmarshal(data, &payload); err == nil {
			return payload.Fn, payload.Args, nil
		}
	}

	// Fallback: fn|arg1|arg2
	raw := strings.TrimSpace(string(data))
	parts := strings.Split(raw, "|")
	if len(parts) == 0 {
		return "", nil, fmt.Errorf("invalid contract call data")
	}
	fn := parts[0]
	args := []string{}
	if len(parts) > 1 {
		args = parts[1:]
	}
	return fn, args, nil
}

func mapToArgs(m map[string]interface{}) []string {
	out := []string{}
	for k, v := range m {
		out = append(out, fmt.Sprintf("%s=%v", k, v))
	}
	return out
}
