package blockchaincomponent

import (
	"fmt"
	"math/big"
	"testing"
	"time"
)

func BenchmarkDeterministicStateRoot10KAccounts(b *testing.B) {
	bc := &Blockchain_struct{Accounts: make(map[string]*big.Int, 10000)}
	for i := 0; i < 10000; i++ {
		bc.Accounts[fmt.Sprintf("0x%040x", i)] = big.NewInt(int64(i + 1))
	}
	bc.EnsureRuntimeState()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = bc.ComputeDeterministicStateRootAt(100)
	}
}

func BenchmarkWeightedProposer100Validators(b *testing.B) {
	bc := &Blockchain_struct{}
	for i := 0; i < 100; i++ {
		bc.Validators = append(bc.Validators, &Validator{Address: fmt.Sprintf("0x%040x", i+1), LPStakeAmount: float64(1000000 + i), NativeBond: float64(1000000 + i), LiquidityPower: float64(1000 + i), LockTime: time.Now().Add(24 * time.Hour)})
	}
	bc.EnsureRuntimeState()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = bc.SelectBlockProposer(uint64(i+1), uint32(i%3))
	}
}
