package blockchainserver

import (
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFaucetAbuseStatePersistsWithoutRawIP(t *testing.T) {
	path := filepath.Join(t.TempDir(), "faucet.json")
	t.Setenv("LQD_FAUCET_STATE_PATH", path)
	first := NewBlockchainServer(1, nil)
	now := time.Unix(1234, 0)
	address, rawIP := "0x1111111111111111111111111111111111111111", "203.0.113.9"
	first.faucetMu.Lock()
	first.faucetAddress[address] = now
	first.faucetIP[faucetIPKey(rawIP)] = now
	first.faucetClaims = 7
	first.faucetIssued = big.NewInt(9000)
	err := first.persistFaucetStateLocked()
	first.faucetMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	second := NewBlockchainServer(1, nil)
	if second.faucetStateErr != nil || second.faucetClaims != 7 || second.faucetIssued.Cmp(big.NewInt(9000)) != 0 || !second.faucetAddress[address].Equal(now) || !second.faucetIP[faucetIPKey(rawIP)].Equal(now) {
		t.Fatalf("faucet state did not survive restart: err=%v claims=%d issued=%s", second.faucetStateErr, second.faucetClaims, second.faucetIssued)
	}
	if _, leaked := second.faucetIP[rawIP]; leaked {
		t.Fatal("raw client IP persisted")
	}
}

func TestFaucetCorruptStateFailsClosed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "faucet.json")
	t.Setenv("LQD_FAUCET_STATE_PATH", path)
	if err := os.WriteFile(path, []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	server := NewBlockchainServer(1, nil)
	if server.faucetStateErr == nil {
		t.Fatal("corrupt anti-abuse state did not fail closed")
	}
}
