package main

import (
	"encoding/json"
	"fmt"

	bc "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/BlockchainComponent"
	constantset "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/ConstantSet"
)

type row struct {
	UtilizationBPS int64  `json:"utilization_bps"`
	NextBaseFee    uint64 `json:"next_base_fee"`
}

func nextBaseFee(current uint64, utilization float64) uint64 {
	ratio := utilization * 2
	if ratio < .75 {
		ratio = .75
	}
	if ratio > 1.25 {
		ratio = 1.25
	}
	next := uint64(float64(current) * ratio)
	if next < uint64(constantset.MinBaseFee) {
		return uint64(constantset.MinBaseFee)
	}
	if next > uint64(constantset.MaxBaseFee) {
		return uint64(constantset.MaxBaseFee)
	}
	return next
}

func main() {
	current := uint64(constantset.InitialBaseFee)
	rows := []row{}
	for _, bps := range []int64{0, 2500, 5000, 7500, 10000} {
		rows = append(rows, row{bps, nextBaseFee(current, float64(bps)/10000)})
	}
	theoreticalTxPerBlock := uint64(constantset.MaxBlockGas) / uint64(constantset.MinGas)
	result := map[string]any{
		"base_fee_scenarios":              rows,
		"minimum_transfer_gas":            constantset.MinGas,
		"max_block_gas":                   constantset.MaxBlockGas,
		"theoretical_simple_tx_per_block": theoreticalTxPerBlock,
		"theoretical_simple_tps_at_2s":    theoreticalTxPerBlock / 2,
		"measured_tps_claim":              false,
		"live_measurement_command":        "go run scripts/load_tps.go -node http://127.0.0.1:16500 -wallet http://127.0.0.1:18080",
		"next_gas_limit_bounds":           []uint64{bc.MinGasLimit, bc.MaxGasLimit},
	}
	raw, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(raw))
	if rows[0].NextBaseFee > current || rows[len(rows)-1].NextBaseFee < current || theoreticalTxPerBlock == 0 {
		panic("fee-market calibration invariant failed")
	}
}
