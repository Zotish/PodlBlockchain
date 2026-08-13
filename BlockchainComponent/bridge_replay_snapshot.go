package blockchaincomponent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
)

type BridgeReplaySnapshot struct {
	Version      uint32            `json:"version"`
	Consumed     map[string]bool   `json:"consumed"`
	Authorized   map[string]bool   `json:"authorized"`
	DailyVolume  map[string]string `json:"daily_volume"`
	HourlyVolume map[string]string `json:"hourly_volume"`
	Checksum     string            `json:"checksum"`
}

func bridgeReplayChecksum(snapshot BridgeReplaySnapshot) string {
	snapshot.Checksum = ""
	raw, _ := json.Marshal(snapshot)
	sum := sha256.Sum256(raw)
	return "0x" + hex.EncodeToString(sum[:])
}

func (bc *Blockchain_struct) ExportBridgeReplaySnapshot() BridgeReplaySnapshot {
	bc.EnsureRuntimeState()
	s := bc.BridgeSecurity
	s.ensure()
	out := BridgeReplaySnapshot{Version: 1, Consumed: map[string]bool{}, Authorized: map[string]bool{}, DailyVolume: map[string]string{}, HourlyVolume: map[string]string{}}
	for key, value := range s.Consumed {
		out.Consumed[key] = value
	}
	for key, value := range s.Authorized {
		out.Authorized[key] = value
	}
	for key, value := range s.DailyVolume {
		out.DailyVolume[key] = AmountString(value)
	}
	for key, value := range s.HourlyVolume {
		out.HourlyVolume[key] = AmountString(value)
	}
	out.Checksum = bridgeReplayChecksum(out)
	return out
}

// RestoreBridgeReplaySnapshot is fail-closed: it rejects bad checksums and a
// snapshot that would forget any event already consumed by this node.
func (bc *Blockchain_struct) RestoreBridgeReplaySnapshot(snapshot BridgeReplaySnapshot) error {
	if bc == nil || snapshot.Version != 1 || snapshot.Checksum != bridgeReplayChecksum(snapshot) {
		return fmt.Errorf("invalid bridge replay snapshot")
	}
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	bc.EnsureRuntimeState()
	for key, consumed := range bc.BridgeSecurity.Consumed {
		if consumed && !snapshot.Consumed[key] {
			return fmt.Errorf("snapshot would regress consumed event state")
		}
	}
	daily := map[string]*big.Int{}
	for key, value := range snapshot.DailyVolume {
		amount, err := NewAmountFromString(value)
		if err != nil {
			return err
		}
		daily[key] = amount
	}
	hourly := map[string]*big.Int{}
	for key, value := range snapshot.HourlyVolume {
		amount, err := NewAmountFromString(value)
		if err != nil {
			return err
		}
		hourly[key] = amount
	}
	bc.BridgeSecurity.Consumed = snapshot.Consumed
	bc.BridgeSecurity.Authorized = snapshot.Authorized
	bc.BridgeSecurity.DailyVolume = daily
	bc.BridgeSecurity.HourlyVolume = hourly
	bc.persistRuntimeStateLocked()
	return nil
}
