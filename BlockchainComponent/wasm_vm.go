package blockchaincomponent

import (
	"errors"
	"fmt"
	"sync"
	"time"

	wasmer "github.com/wasmerio/wasmer-go/wasmer"
)

// Contract structure
type WASMContract struct {
	Address   string
	Code      []byte
	Storage   map[string]string
	Creator   string
	CreatedAt time.Time
}

// VM
type WASMVM struct {
	Engine    *wasmer.Engine
	Store     *wasmer.Store
	Mutex     sync.Mutex
	Contracts map[string]*WASMContract
}

// Constructor
func NewWASMVM() *WASMVM {
	engine := wasmer.NewEngine()
	store := wasmer.NewStore(engine)

	return &WASMVM{
		Engine:    engine,
		Store:     store,
		Contracts: make(map[string]*WASMContract),
	}
}

// Deploy contract
func (vm *WASMVM) Deploy(address string, wasmCode []byte, creator string) error {
	vm.Mutex.Lock()
	defer vm.Mutex.Unlock()

	if _, ok := vm.Contracts[address]; ok {
		return fmt.Errorf("contract already exists: %s", address)
	}

	vm.Contracts[address] = &WASMContract{
		Address:   address,
		Code:      wasmCode,
		Storage:   make(map[string]string),
		Creator:   creator,
		CreatedAt: time.Now(),
	}
	return nil
}

// -----------------------------
// MEMORY HELPERS
// -----------------------------

// Read string from WASM memory
func readMemory(instance *wasmer.Instance, offset int32, length int32) (string, error) {
	memory, err := instance.Exports.GetMemory("memory")
	if err != nil {
		return "", err
	}

	data := memory.Data()[offset : offset+length]
	return string(data), nil
}

// Allocate space in WASM memory & write string
func writeMemory(instance *wasmer.Instance, value string) (int32, error) {
	// Get malloc export
	malloc, err := instance.Exports.GetFunction("malloc")
	if err != nil {
		return 0, errors.New("malloc() not found in WASM contract")
	}

	size := len(value)
	ptrVal, err := malloc(size)
	if err != nil {
		return 0, err
	}

	ptr := ptrVal.(int32)

	memory, err := instance.Exports.GetMemory("memory")
	if err != nil {
		return 0, err
	}

	copy(memory.Data()[ptr:ptr+int32(size)], []byte(value))
	return ptr, nil
}

// -----------------------------
// EXECUTION
// -----------------------------

func (vm *WASMVM) Call(address string, caller string, method string, args []string) (string, error) {
	vm.Mutex.Lock()
	contract := vm.Contracts[address]
	vm.Mutex.Unlock()

	if contract == nil {
		return "", fmt.Errorf("contract not found: %s", address)
	}

	// Load module
	module, err := wasmer.NewModule(vm.Store, contract.Code)
	if err != nil {
		return "", fmt.Errorf("wasm load failed: %v", err)
	}

	// Create import object
	importObject := wasmer.NewImportObject()

	// Temporary instance pointer for closures
	var instance *wasmer.Instance

	// Register host functions (env.get, env.set)
	importObject.Register(
		"env",
		map[string]wasmer.IntoExtern{
			"get": wasmer.NewFunction(
				vm.Store,
				wasmer.NewFunctionType(
					[]*wasmer.ValueType{
						wasmer.NewValueType(wasmer.I32), // key ptr
						wasmer.NewValueType(wasmer.I32), // key len
					},
					[]*wasmer.ValueType{
						wasmer.NewValueType(wasmer.I32), // return ptr
					},
				),
				func(args []wasmer.Value) ([]wasmer.Value, error) {
					keyPtr := args[0].I32()
					keyLen := args[1].I32()

					key, err := readMemory(instance, keyPtr, keyLen)
					if err != nil {
						return []wasmer.Value{wasmer.NewI32(0)}, nil
					}

					val := contract.Storage[key]

					ptr, err := writeMemory(instance, val)
					if err != nil {
						return []wasmer.Value{wasmer.NewI32(0)}, nil
					}
					return []wasmer.Value{wasmer.NewI32(ptr)}, nil
				},
			),

			"set": wasmer.NewFunction(
				vm.Store,
				wasmer.NewFunctionType(
					[]*wasmer.ValueType{
						wasmer.NewValueType(wasmer.I32), // key ptr
						wasmer.NewValueType(wasmer.I32), // key len
						wasmer.NewValueType(wasmer.I32), // val ptr
						wasmer.NewValueType(wasmer.I32), // val len
					},
					[]*wasmer.ValueType{},
				),
				func(args []wasmer.Value) ([]wasmer.Value, error) {
					keyPtr := args[0].I32()
					keyLen := args[1].I32()
					valPtr := args[2].I32()
					valLen := args[3].I32()

					key, _ := readMemory(instance, keyPtr, keyLen)
					val, _ := readMemory(instance, valPtr, valLen)

					contract.Storage[key] = val
					return []wasmer.Value{}, nil
				},
			),
		},
	)

	// Instantiate with imports
	instance, err = wasmer.NewInstance(module, importObject)
	if err != nil {
		return "", fmt.Errorf("wasm instantiation failed: %v", err)
	}

	// Find Execute()
	execFn, err := instance.Exports.GetFunction("Execute")
	if err != nil {
		return "", fmt.Errorf("Execute() missing in contract")
	}

	// Convert args to interface
	callArgs := []interface{}{method}
	for _, a := range args {
		callArgs = append(callArgs, a)
	}

	_, err = execFn(callArgs...)
	if err != nil {
		return "", fmt.Errorf("execution failed: %v", err)
	}

	return "ok", nil
}
