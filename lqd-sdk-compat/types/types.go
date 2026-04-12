// Package types provides core LQD blockchain types for smart contract development.
package types

import "math/big"

// Transaction represents an LQD blockchain transaction.
type Transaction struct {
	From        string   `json:"from"`
	To          string   `json:"to"`
	Value       *big.Int `json:"value"`
	Data        []byte   `json:"data"`
	TxHash      string   `json:"tx_hash"`
	Status      string   `json:"status"`
	Gas         uint64   `json:"gas"`
	GasPrice    uint64   `json:"gas_price"`
	Nonce       uint64   `json:"nonce"`
	ChainID     uint64   `json:"chain_id"`
	Timestamp   uint64   `json:"timestamp"`
	PriorityFee uint64   `json:"priority_fee"`
	IsContract  bool     `json:"is_contract"`
	Function    string   `json:"function"`
	Args        []string `json:"args"`
	Type        string   `json:"type"`
	IsSystem    bool     `json:"is_system"`
}

// Block represents an LQD blockchain block.
type Block struct {
	BlockNumber  uint64         `json:"block_number"`
	PreviousHash string         `json:"previous_hash"`
	CurrentHash  string         `json:"current_hash"`
	TimeStamp    uint64         `json:"timestamp"`
	Transactions []*Transaction `json:"transactions"`
	BaseFee      uint64         `json:"base_fee"`
	GasUsed      uint64         `json:"gas_used"`
	GasLimit     uint64         `json:"gas_limit"`
}

// LQD constants
const (
	Decimals   = 8   // 1 LQD = 10^8 base units
	ChainID    = 139
	StatusOK   = "success"
	StatusFail = "failed"
)
