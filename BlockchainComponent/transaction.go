package blockchaincomponent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"math/big"
	"time"

	constantset "github.com/Zotish/DefenceProject/ConstantSet"
)

type Transaction struct {
	From        string   `json:"from"`
	To          string   `json:"to"`
	Value       *big.Int `json:"value"`
	Data        []byte   `json:"data"`
	TxHash      string   `json:"tx_hash"`
	Status      string   `json:"status"`
	Gas         uint64   `json:"gas"`
	GasPrice    uint64   `json:"gas_price"`
	Sig         []byte   `json:"sig"`
	Nonce       uint64   `json:"nonce"`
	ChainID     uint64   `json:"chain_id"`
	Timestamp   uint64   `json:"timestamp"`
	PriorityFee uint64   `json:"priority_fee"`
	IsContract  bool     `json:"is_contract"`
	Function    string   `json:"function"`
	Args        []string `json:"args"`
	Type        string   `json:"type"`
	ExtraData   []byte   `json:"extra_data"`
	IsSystem    bool     `json:"is_system"`
}

func NewTransaction(from string, to string, value *big.Int, data []byte) *Transaction {
	newTx := new(Transaction)
	newTx.From = from
	newTx.To = to
	newTx.Data = data
	newTx.Gas = uint64(constantset.MinGas)
	newTx.GasPrice = 1
	newTx.Value = CopyAmount(value)
	newTx.Status = constantset.StatusPending
	//newTx.Nonce = nonce + 1
	newTx.ChainID = uint64(constantset.ChainID)
	newTx.Timestamp = uint64(time.Now().Unix())
	newTx.Sig = []byte{}
	newTx.IsContract = false
	newTx.Args = nil
	newTx.Function = ""

	if newTx.IsContract {
		newTx.Gas = uint64(constantset.ContractCallGas) // Higher gas for contracts
	}
	newTx.TxHash = CalculateTransactionHash(*newTx)
	return newTx
}

func (newTx *Transaction) ToJsonTx() string {
	nTx := newTx
	tx, err := json.Marshal(nTx)
	if err != nil {
		log.Println("error")
	}
	return string(tx)
}
func CalculateTransactionHash(transaction Transaction) string {
	JsonData, _ := json.Marshal(transaction)
	sumData := sha256.Sum256(JsonData)
	HexRePresent := hex.EncodeToString(sumData[:32])
	formateHex := constantset.BlockHexPrefix + HexRePresent
	return formateHex

}
func (tx *Transaction) CalculateGasCost() uint64 {
	baseCost := constantset.MinGas
	dataCost := len(tx.Data) * constantset.GasPerByte
	return uint64(baseCost + dataCost)
}
