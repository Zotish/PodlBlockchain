package main

import (
	"testing"
	"time"

	blockchaincomponent "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/BlockchainComponent"
)

func TestConfiguredBlockProductionInterval(t *testing.T) {
	bc := &blockchaincomponent.Blockchain_struct{}
	if got := configuredBlockProductionInterval(bc); got != defaultBlockProductionInterval {
		t.Fatalf("default interval = %s, want %s", got, defaultBlockProductionInterval)
	}
	bc.ChainSpec.BlockTimeMS = 1500
	if got := configuredBlockProductionInterval(bc); got != 1500*time.Millisecond {
		t.Fatalf("configured interval = %s, want 1.5s", got)
	}
}

func TestRemainingBlockProductionDelay(t *testing.T) {
	tests := []struct {
		name    string
		target  time.Duration
		elapsed time.Duration
		want    time.Duration
	}{
		{name: "work below target", target: 2 * time.Second, elapsed: 350 * time.Millisecond, want: 1650 * time.Millisecond},
		{name: "work equals target", target: 2 * time.Second, elapsed: 2 * time.Second, want: 0},
		{name: "work exceeds target", target: 2 * time.Second, elapsed: 9 * time.Second, want: 0},
		{name: "invalid target falls back", target: 0, elapsed: 500 * time.Millisecond, want: 1500 * time.Millisecond},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := remainingBlockProductionDelay(tc.target, tc.elapsed); got != tc.want {
				t.Fatalf("delay = %s, want %s", got, tc.want)
			}
		})
	}
}
