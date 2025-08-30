package blockchaincomponent

import (
	"fmt"
	"sync"
	"time"
)

// SmartContract represents a deployable contract
type SmartContract struct {
	Address   string            `json:"address"`
	Code      string            `json:"code"`
	Storage   map[string]string `json:"storage"`
	Owner     string            `json:"owner"`
	Balance   uint64            `json:"balance"`
	IsActive  bool              `json:"is_active"`
	CreatedAt uint64            `json:"created_at"`
}

// ContractExecutionResult holds execution results
type ContractExecutionResult struct {
	Success bool              `json:"success"`
	Output  string            `json:"output"`
	GasUsed uint64            `json:"gas_used"`
	Storage map[string]string `json:"storage"`
	Events  []ContractEvent   `json:"events"`
}

// ContractEvent represents contract events/logs
type ContractEvent struct {
	EventName string                 `json:"event_name"`
	Data      map[string]interface{} `json:"data"`
	Address   string                 `json:"address"`
}

// VM is the virtual machine for contract execution
type VM struct {
	Contracts map[string]*SmartContract `json:"contracts"`
	Mutex     sync.Mutex                `json:"-"`
}

// NewVM creates a new virtual machine
func NewVM() *VM {
	return &VM{
		Contracts: make(map[string]*SmartContract),
	}
}

// DeployContract deploys a new smart contract
func (vm *VM) DeployContract(code, owner string, initialBalance uint64) (string, error) {
	vm.Mutex.Lock()
	defer vm.Mutex.Unlock()

	contractAddress := GenerateContractAddress(owner, uint64(len(vm.Contracts)))

	contract := &SmartContract{
		Address:   contractAddress,
		Code:      code,
		Storage:   make(map[string]string),
		Owner:     owner,
		Balance:   initialBalance,
		IsActive:  true,
		CreatedAt: uint64(time.Now().Unix()),
	}

	vm.Contracts[contractAddress] = contract
	return contractAddress, nil
}

// ExecuteContract executes contract code
func (vm *VM) ExecuteContract(contractAddress, caller string, function string, args []string, value uint64) (*ContractExecutionResult, error) {
	vm.Mutex.Lock()
	defer vm.Mutex.Unlock()

	contract, exists := vm.Contracts[contractAddress]
	if !exists || !contract.IsActive {
		return nil, fmt.Errorf("contract not found or inactive")
	}

	// Basic gas calculation
	gasUsed := uint64(10000) // Base gas
	gasUsed += uint64(len(function)) * 10
	for _, arg := range args {
		gasUsed += uint64(len(arg)) * 5
	}

	result := &ContractExecutionResult{
		GasUsed: gasUsed,
		Storage: make(map[string]string),
		Events:  []ContractEvent{},
	}

	// Simple contract execution logic
	switch function {
	case "transfer":
		if len(args) < 2 {
			return nil, fmt.Errorf("transfer requires recipient and amount")
		}
		recipient := args[0]
		amount := parseUint(args[1])

		if contract.Balance < amount {
			result.Success = false
			result.Output = "insufficient contract balance"
			return result, nil
		}

		contract.Balance -= amount
		result.Success = true
		result.Output = fmt.Sprintf("transferred %d to %s", amount, recipient)

		// Emit event
		result.Events = append(result.Events, ContractEvent{
			EventName: "Transfer",
			Data: map[string]interface{}{
				"from":   caller,
				"to":     recipient,
				"amount": amount,
			},
			Address: contractAddress,
		})

	case "getBalance":
		result.Success = true
		result.Output = fmt.Sprintf("%d", contract.Balance)

	case "setStorage":
		if len(args) < 2 {
			return nil, fmt.Errorf("setStorage requires key and value")
		}
		key := args[0]
		value := args[1]
		contract.Storage[key] = value
		result.Success = true
		result.Output = "storage updated"

	case "getStorage":
		if len(args) < 1 {
			return nil, fmt.Errorf("getStorage requires key")
		}
		key := args[0]
		value, exists := contract.Storage[key]
		if !exists {
			result.Output = ""
		} else {
			result.Output = value
		}
		result.Success = true

	default:
		result.Success = false
		result.Output = "unknown function"
	}

	// Copy storage to result
	for k, v := range contract.Storage {
		result.Storage[k] = v
	}

	return result, nil
}

// GenerateContractAddress generates a unique contract address
func GenerateContractAddress(owner string, nonce uint64) string {
	// Simple address generation - in production use cryptographic hashing
	return fmt.Sprintf("0x%s%d", owner[len(owner)-8:], nonce)
}

func parseUint(s string) uint64 {
	var result uint64
	fmt.Sscanf(s, "%d", &result)
	return result
}
