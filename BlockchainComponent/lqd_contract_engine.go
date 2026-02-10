package blockchaincomponent

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"plugin"
	"reflect"
	"strconv"
	"strings"
	"time"

	constantset "github.com/Zotish/DefenceProject/ConstantSet"
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
	db *leveldb.DB
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

//  CORE TYPES

type ContractMetadata struct {
	Address     string `json:"address"`
	Type        string `json:"type"` // plugin | gocode | dsl | builtin
	Owner       string `json:"owner"`
	ABI         []byte `json:"abi"`
	Timestamp   int64  `json:"timestamp"`
	Pool        bool   `json:"pool"`
	PoolType    string `json:"pool_type,omitempty"`
	PluginPath  string `json:"plugin_path,omitempty"`
	Code        []byte `json:"code,omitempty"`
	BuiltinName string `json:"builtin_name,omitempty"`
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

//  CONTEXT — SANDBOXED EXECUTION ENVIRONMENT

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
	ctx.events = append(ctx.events, ContractEvent{
		EventName: ev,
		Data:      data,
		Address:   ctx.ContractAddr,
		Timestamp: time.Now().Unix(),
	})
}

func (ctx *Context) Call(target, fn string, args []string) (*ContractExecutionResult, error) {
	ctx.consumeGas(10000)
	if ctx.callFunc == nil {
		return nil, fmt.Errorf("cross-call disabled")
	}
	return ctx.callFunc(target, fn, args)
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
	plugins map[string]*PluginContract
	byPath  map[string]*PluginContract
}

func NewPluginVM() *PluginVM {
	return &PluginVM{
		plugins: make(map[string]*PluginContract),
		byPath:  make(map[string]*PluginContract),
	}
}

func (p *PluginVM) LoadPlugin(addr, path string) error {

	if path == "" {
		return fmt.Errorf("plugin path required")
	}
	if existing := p.byPath[path]; existing != nil {
		p.plugins[addr] = existing
		return nil
	}

	pl, err := plugin.Open(path)
	if err != nil {
		// Go plugins can only be loaded once per process
		if strings.Contains(err.Error(), "plugin already loaded") {
			if existing := p.byPath[path]; existing != nil {
				p.plugins[addr] = existing
				return nil
			}
			// Fallback: reuse any already-loaded plugin (single-plugin limitation)
			for _, existing := range p.plugins {
				if existing != nil {
					p.plugins[addr] = existing
					return nil
				}
			}
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
	return nil
}

func (p *PluginVM) CallPlugin(addr, fn string, ctx *Context, args []string) (*ContractExecutionResult, error) {
	pc := p.plugins[addr]
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
			a := parseBig(ctx.Get(bc.Args[pc*2]))
			b := parseBig(ctx.Get(bc.Args[pc*2+1]))
			ctx.Set(bc.Args[pc*2], new(big.Int).Add(a, b).String())

		case OP_SUB:
			a := parseBig(ctx.Get(bc.Args[pc*2]))
			b := parseBig(ctx.Get(bc.Args[pc*2+1]))
			ctx.Set(bc.Args[pc*2], new(big.Int).Sub(a, b).String())

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
			cur := parseBig(ctx.Get(kv[0]))
			add := parseBig(kv[1])
			ctx.Set(kv[0], new(big.Int).Add(cur, add).String())
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

// EXECUTION PIPELINE

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

	// Execute contract
	res, err := ep.Execute(tx.To, tx.From, fn, args, 5_000_000)
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
