package blockchaincomponent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"plugin"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/syndtr/goleveldb/leveldb"
)

// =================================================================================================
// SECTION 1: CONTRACT ENGINE TOP-LEVEL STRUCT
// =================================================================================================

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

// =================================================================================================
// SECTION 2: DB LAYER
// =================================================================================================

type ContractDB struct {
	db *leveldb.DB
}

func ensureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}

func InitContractDB() (*ContractDB, error) {
	// base under current working dir: ./data/contracts_db
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	base := filepath.Join(cwd, "data")
	if err := ensureDir(base); err != nil {
		return nil, err
	}

	path := filepath.Join(base, "contracts_db")
	d, err := leveldb.OpenFile(path, nil)
	if err != nil {
		return nil, err
	}
	return &ContractDB{db: d}, nil
}

func (c *ContractDB) Write(key string, val []byte) error {
	return c.db.Put([]byte(key), val, nil)
}
func (c *ContractDB) Read(key string) ([]byte, error) {
	return c.db.Get([]byte(key), nil)
}
func (c *ContractDB) Delete(key string) error {
	return c.db.Delete([]byte(key), nil)
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

func (c *ContractDB) LoadStorage(addr, key string) (string, error) {
	b, err := c.Read("contract:" + addr + ":storage:" + key)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (c *ContractDB) LoadAllStorage(addr string) (map[string]string, error) {
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

// =================================================================================================
// SECTION 3: CORE TYPES
// =================================================================================================

type ContractMetadata struct {
	Address     string `json:"address"`
	Type        string `json:"type"` // plugin | gocode | dsl | builtin
	Owner       string `json:"owner"`
	ABI         []byte `json:"abi"`
	Timestamp   int64  `json:"timestamp"`
	PluginPath  string `json:"plugin_path,omitempty"`
	Code        []byte `json:"code,omitempty"`
	BuiltinName string `json:"builtin_name,omitempty"`
}

type SmartContractState struct {
	Address   string            `json:"address"`
	Balance   uint64            `json:"balance"`
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

// =================================================================================================
// SECTION 4: CONTEXT — SANDBOXED EXECUTION ENVIRONMENT
// =================================================================================================

type Context struct {
	ContractAddr string
	CallerAddr   string
	OwnerAddr    string
	BlockTime    int64
	GasUsed      uint64
	GasLimit     uint64
	DB           *ContractDB
	tempStorage  map[string]string
	events       []ContractEvent
	callFunc     func(target string, fn string, args []string) (*ContractExecutionResult, error)
}

func NewContext(addr, caller, owner string, db *ContractDB, gas uint64) *Context {
	return &Context{
		ContractAddr: addr,
		CallerAddr:   caller,
		OwnerAddr:    owner,
		BlockTime:    time.Now().Unix(),
		GasUsed:      0,
		GasLimit:     gas,
		DB:           db,
		tempStorage:  make(map[string]string),
		events:       []ContractEvent{},
	}
}

// Storage
func (ctx *Context) Get(key string) string {
	if v, ok := ctx.tempStorage[key]; ok {
		return v
	}
	val, _ := ctx.DB.LoadStorage(ctx.ContractAddr, key)
	return val
}

func (ctx *Context) Set(key, value string) {
	ctx.consumeGas(200)
	ctx.tempStorage[key] = value
}

// Balances
func (ctx *Context) balanceOf(addr string) uint64 {
	key := "__bal:" + addr
	if v, ok := ctx.tempStorage[key]; ok {
		x, _ := strconv.ParseUint(v, 10, 64)
		return x
	}
	raw, _ := ctx.DB.LoadStorage(ctx.ContractAddr, key)
	out, _ := strconv.ParseUint(raw, 10, 64)
	return out
}

func (ctx *Context) AddBalance(addr string, amt uint64) {
	ctx.consumeGas(150)
	ctx.tempStorage["__bal:"+addr] = fmt.Sprintf("%d", ctx.balanceOf(addr)+amt)
}

func (ctx *Context) SubBalance(addr string, amt uint64) {
	ctx.consumeGas(150)
	b := ctx.balanceOf(addr)
	if b < amt {
		ctx.Revert("insufficient balance")
	}
	ctx.tempStorage["__bal:"+addr] = fmt.Sprintf("%d", b-amt)
}

// Events
func (ctx *Context) Emit(ev string, data map[string]interface{}) {
	ctx.consumeGas(500)
	ctx.events = append(ctx.events, ContractEvent{
		EventName: ev,
		Data:      data,
		Address:   ctx.ContractAddr,
		Timestamp: time.Now().Unix(),
	})
}

// Cross-contract call
func (ctx *Context) Call(target, fn string, args []string) (*ContractExecutionResult, error) {
	ctx.consumeGas(10000)
	if ctx.callFunc == nil {
		return nil, fmt.Errorf("cross-call disabled")
	}
	return ctx.callFunc(target, fn, args)
}

// Gas / revert
func (ctx *Context) consumeGas(n uint64) {
	ctx.GasUsed += n
	if ctx.GasUsed > ctx.GasLimit {
		ctx.Revert("out of gas")
	}
}

func (ctx *Context) Revert(reason string) {
	panic("REVERT: " + reason)
}

// Commit
func (ctx *Context) Commit() error {
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

// =================================================================================================
// SECTION 5: GO PLUGIN VM
// =================================================================================================

type PluginContract struct {
	Instance any
	Methods  map[string]reflect.Method
}

type PluginVM struct {
	plugins map[string]*PluginContract
}

func NewPluginVM() *PluginVM {
	return &PluginVM{plugins: make(map[string]*PluginContract)}
}

func (p *PluginVM) LoadPlugin(addr, path string) error {

	pl, err := plugin.Open(path)
	if err != nil {
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
	}

	p.plugins[addr] = &PluginContract{Instance: inst, Methods: methods}
	return nil
}

func (p *PluginVM) CallPlugin(addr, fn string, ctx *Context, args []string) (*ContractExecutionResult, error) {
	pc := p.plugins[addr]
	if pc == nil {
		return nil, fmt.Errorf("plugin not loaded")
	}

	m, ok := pc.Methods[fn]
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

	return &ContractExecutionResult{
		Success: true,
		GasUsed: ctx.GasUsed,
		Storage: ctx.tempStorage,
		Events:  ctx.events,
	}, nil
}

// =================================================================================================
// SECTION 6: INTERPRETER VM
// =================================================================================================

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
	Ops  []OpCode
	Args []string
}

type InterpreterVM struct{}

func NewInterpreterVM() *InterpreterVM { return &InterpreterVM{} }

func (ivm *InterpreterVM) CompileGoSubset(src string) (*Bytecode, error) {
	out := &Bytecode{}
	lines := strings.Split(src, " ")

	for _, ln := range lines {
		if ln == "" {
			continue
		}

		parts := strings.Split(ln, " ")

		switch parts[0] {

		case "SET":
			out.Ops = append(out.Ops, OP_SET)
			out.Args = append(out.Args, parts[1], parts[2])

		case "GET":
			out.Ops = append(out.Ops, OP_GET)
			out.Args = append(out.Args, parts[1])

		case "ADD":
			out.Ops = append(out.Ops, OP_ADD)
			out.Args = append(out.Args, parts[1], parts[2])

		case "SUB":
			out.Ops = append(out.Ops, OP_SUB)
			out.Args = append(out.Args, parts[1], parts[2])

		case "EQ":
			out.Ops = append(out.Ops, OP_EQ)
			out.Args = append(out.Args, parts[1], parts[2])

		case "NEQ":
			out.Ops = append(out.Ops, OP_NEQ)
			out.Args = append(out.Args, parts[1], parts[2])

		case "JMP":
			out.Ops = append(out.Ops, OP_JMP)
			out.Args = append(out.Args, parts[1])

		case "JMPIF":
			out.Ops = append(out.Ops, OP_JMPIF)
			out.Args = append(out.Args, parts[1])

		case "CALL":
			out.Ops = append(out.Ops, OP_CALL)
			out.Args = append(out.Args, parts[1], parts[2])

		case "REVERT":
			out.Ops = append(out.Ops, OP_REVERT)
			out.Args = append(out.Args, parts[1])

		default:
			out.Ops = append(out.Ops, OP_NOOP)
		}
	}

	return out, nil
}

func (ivm *InterpreterVM) ExecuteBytecode(addr string, bc *Bytecode, ctx *Context) (*ContractExecutionResult, error) {

	pc := 0

	for pc < len(bc.Ops) {

		op := bc.Ops[pc]

		switch op {

		case OP_SET:
			k := bc.Args[pc*2]
			v := bc.Args[pc*2+1]
			ctx.Set(k, v)

		case OP_GET:
			_ = ctx.Get(bc.Args[pc])

		case OP_ADD:
			a := parseUint(ctx.Get(bc.Args[pc*2]))
			b := parseUint(ctx.Get(bc.Args[pc*2+1]))
			ctx.Set(bc.Args[pc*2], fmt.Sprintf("%d", a+b))

		case OP_SUB:
			a := parseUint(ctx.Get(bc.Args[pc*2]))
			b := parseUint(ctx.Get(bc.Args[pc*2+1]))
			ctx.Set(bc.Args[pc*2], fmt.Sprintf("%d", a-b))

		case OP_EQ:
			if ctx.Get(bc.Args[pc*2]) == ctx.Get(bc.Args[pc*2+1]) {
				ctx.Set("__cmp", "1")
			} else {
				ctx.Set("__cmp", "0")
			}

		case OP_NEQ:
			if ctx.Get(bc.Args[pc*2]) != ctx.Get(bc.Args[pc*2+1]) {
				ctx.Set("__cmp", "1")
			} else {
				ctx.Set("__cmp", "0")
			}

		case OP_JMP:
			idx, _ := strconv.Atoi(bc.Args[pc])
			pc = idx
			continue

		case OP_JMPIF:
			idx, _ := strconv.Atoi(bc.Args[pc])
			if ctx.Get("__cmp") == "1" {
				pc = idx
				continue
			}

		case OP_CALL:
			target := bc.Args[pc*2]
			fn := bc.Args[pc*2+1]
			_, err := ctx.Call(target, fn, []string{})
			if err != nil {
				return nil, err
			}

		case OP_REVERT:
			ctx.Revert(bc.Args[pc])
		}

		pc++
	}

	ctx.Commit()

	return &ContractExecutionResult{
		Success: true,
		GasUsed: ctx.GasUsed,
		Storage: ctx.tempStorage,
		Events:  ctx.events,
	}, nil
}

func parseUint(s string) uint64 {
	v, _ := strconv.ParseUint(s, 10, 64)
	return v
}

// =================================================================================================
// SECTION 7: DSL VM
// =================================================================================================

type DSLVM struct{}

func NewDSLVM() *DSLVM { return &DSLVM{} }

func (d *DSLVM) CompileDSL(src string) ([]string, error) {
	out := []string{}
	parts := strings.Split(src, " ")

	for _, s := range parts {
		if strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out, nil
}

func (d *DSLVM) ExecuteDSL(addr string, lines []string, ctx *Context) (*ContractExecutionResult, error) {

	for _, ln := range lines {

		// key=value
		if strings.Contains(ln, "=") && !strings.Contains(ln, "+=") {
			kv := strings.SplitN(ln, "=", 2)
			ctx.Set(kv[0], kv[1])
			continue
		}

		// key+=N
		if strings.Contains(ln, "+=") {
			kv := strings.SplitN(ln, "+=", 2)
			cur := parseUint(ctx.Get(kv[0]))
			add := parseUint(kv[1])
			ctx.Set(kv[0], fmt.Sprintf("%d", cur+add))
			continue
		}

		// emit X
		if strings.HasPrefix(ln, "emit") {
			ev := strings.TrimPrefix(ln, "emit")
			ctx.Emit(ev, map[string]interface{}{"msg": ev})
			continue
		}

		// call A.fn
		if strings.Contains(ln, "call") {
			body := strings.TrimPrefix(ln, "call")
			parts := strings.Split(body, ".")
			tgt := parts[0]
			fn := parts[1]
			_, err := ctx.Call(tgt, fn, []string{})
			if err != nil {
				return nil, err
			}
			continue
		}
	}

	ctx.Commit()

	return &ContractExecutionResult{
		Success: true,
		GasUsed: ctx.GasUsed,
		Storage: ctx.tempStorage,
		Events:  ctx.events,
	}, nil
}

// =================================================================================================
// SECTION 8: ABI GENERATOR
// =================================================================================================

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

// =================================================================================================
// SECTION 9: EVENT DB
// =================================================================================================

type EventDB struct {
	db *leveldb.DB
}

func (ep *ExecutionPipeline) ApplyContractCall(addr, caller, fn string, args []string) (*ContractExecutionResult, error) {
	return ep.Execute(addr, caller, fn, args, 5_000_000)
}

func InitEventDB() (*EventDB, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	base := filepath.Join(cwd, "data")
	if err := ensureDir(base); err != nil {
		return nil, err
	}

	path := filepath.Join(base, "contract_events_db")
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

// =================================================================================================
// SECTION 10: CONTRACT REGISTRY
// =================================================================================================

type ContractRegistry struct {
	DB       *ContractDB
	EventDB  *EventDB
	PluginVM *PluginVM
	IVM      *InterpreterVM
	DSL      *DSLVM
}

func NewContractRegistry(cdb *ContractDB, edb *EventDB) *ContractRegistry {
	return &ContractRegistry{
		DB:       cdb,
		EventDB:  edb,
		PluginVM: NewPluginVM(),
		IVM:      NewInterpreterVM(),
		DSL:      NewDSLVM(),
	}
}

func (r *ContractRegistry) RegisterContract(meta *ContractMetadata, st *SmartContractState) error {

	if err := r.DB.SaveContractMetadata(meta.Address, meta); err != nil {
		return err
	}
	if err := r.DB.SaveStorage(meta.Address, "__bal:"+meta.Owner, fmt.Sprintf("%d", st.Balance)); err != nil {
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
	bal := parseUint(storage["__bal:"+meta.Owner])

	state := &SmartContractState{
		Address:   addr,
		Balance:   bal,
		Storage:   storage,
		IsActive:  true,
		CreatedAt: time.Now().Unix(),
	}

	return &ContractRecord{Metadata: meta, State: state}, nil
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
	if _, ok := r.PluginVM.plugins[addr]; ok {
		return nil
	}

	return r.PluginVM.LoadPlugin(addr, meta.PluginPath)
}

// =================================================================================================
// SECTION 11: EXECUTION PIPELINE
// =================================================================================================

type ExecutionPipeline struct {
	Registry *ContractRegistry
}

func NewExecutionPipeline(reg *ContractRegistry) *ExecutionPipeline {
	return &ExecutionPipeline{Registry: reg}
}

func (ep *ExecutionPipeline) Execute(addr, caller, fn string, args []string, gas uint64) (*ContractExecutionResult, error) {

	rec, err := ep.Registry.LoadContract(addr)
	if err != nil {
		return nil, err
	}

	ctx := NewContext(addr, caller, rec.Metadata.Owner, ep.Registry.DB, gas)

	ctx.callFunc = func(tgt, method string, a []string) (*ContractExecutionResult, error) {
		return ep.Execute(tgt, addr, method, a, gas/2)
	}

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

	return nil, fmt.Errorf("invalid contract type")
}

// =================================================================================================
// SECTION 12: BLOCKCHAIN INTEGRATION
// =================================================================================================

// Your blockchain Transaction type MUST have a `Data [][]byte` field.

func (ep *ExecutionPipeline) ExecuteContractTx(tx *Transaction, block uint64) (*ContractExecutionResult, error) {

	if len(tx.Data) < 1 {
		return nil, fmt.Errorf("tx missing function selector")
	}

	fn := string(tx.Data[0])
	args := []string{}
	if len(tx.Data) > 1 {
		for _, b := range tx.Data[1:] {
			args = append(args, string(b))
		}
	}

	res, err := ep.Execute(tx.To, tx.From, fn, args, 5_000_000)
	if err != nil {
		return nil, err
	}

	for i, ev := range res.Events {
		ep.Registry.EventDB.SaveEvent(block, tx.TxHash, i, ev)
	}

	return res, nil
}

// =================================================================================================
// DONE — NO API CODE INCLUDED (YOU ADD THAT IN blockchain_server.go)
// =================================================================================================
