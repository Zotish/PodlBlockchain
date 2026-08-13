package blockchaincomponent

import (
	"encoding/hex"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

func TestBridgeThresholdAttestationsAndReplayProtection(t *testing.T) {
	previousPersist := persistRuntimeState
	persistRuntimeState = func(*Blockchain_struct) error { return nil }
	defer func() { persistRuntimeState = previousPersist }()
	bc := &Blockchain_struct{BridgeRequests: map[string]*BridgeRequest{"req": {ID: "req", Amount: "100", SourceChainID: "bsc", SourceTxHash: "0xsource"}}}
	bc.EnsureRuntimeState()
	bc.BridgeSecurity.Policy.EnforceAttestations = true
	bc.BridgeSecurity.Policy.RequiredSigners = 3
	bc.BridgeSecurity.Policy.PerTransactionCaps["bsc"] = "1000"
	bc.BridgeSecurity.Policy.DailyCaps["bsc"] = "5000"
	for i := 0; i < 3; i++ {
		key, _ := crypto.GenerateKey()
		addr := strings.ToLower(crypto.PubkeyToAddress(key.PublicKey).Hex())
		bc.BridgeSecurity.Policy.AllowedSigners[addr] = true
		a := BridgeAttestation{RequestID: "req", SourceChainID: "bsc", SourceTxHash: "0xsource", SourceBlockHash: "0xblock", EventIndex: "1", Confirmations: 20, ObservedAt: 100}
		if err := SignBridgeAttestation(&a, hex.EncodeToString(crypto.FromECDSA(key))); err != nil {
			t.Fatal(err)
		}
		if err := bc.SubmitBridgeAttestation(a, 100); err != nil {
			t.Fatal(err)
		}
	}
	if err := bc.BridgeExecutionAuthorized("req", 100); err != nil {
		t.Fatal(err)
	}
	if bc.BridgeSecurity.DailyVolume["bsc:1970-01-01"].Cmp(big.NewInt(100)) != 0 {
		t.Fatal("daily volume not reserved")
	}
}
