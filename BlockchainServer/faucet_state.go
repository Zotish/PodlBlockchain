package blockchainserver

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type persistentFaucetState struct {
	Version   uint32           `json:"version"`
	Addresses map[string]int64 `json:"addresses"`
	IPHashes  map[string]int64 `json:"ip_hashes"`
	Claims    uint64           `json:"claims"`
	Denied    uint64           `json:"denied"`
	Issued    string           `json:"issued"`
}

func defaultFaucetStatePath() string {
	if path := strings.TrimSpace(os.Getenv("LQD_FAUCET_STATE_PATH")); path != "" {
		return path
	}
	if root := strings.TrimSpace(os.Getenv("LQD_DATA_DIR")); root != "" {
		return filepath.Join(root, "faucet_state.json")
	}
	config, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "podl-faucet-state.json")
	}
	return filepath.Join(config, "lqd", "faucet_state.json")
}

func faucetIPKey(ip string) string {
	sum := sha256.Sum256([]byte("PODL-FAUCET-IP-V1:" + strings.TrimSpace(ip)))
	return hex.EncodeToString(sum[:])
}

func (b *BlockchainServer) loadFaucetState() error {
	if b == nil || strings.TrimSpace(b.faucetStatePath) == "" {
		return fmt.Errorf("faucet state path unavailable")
	}
	raw, err := os.ReadFile(b.faucetStatePath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var state persistentFaucetState
	if err = json.Unmarshal(raw, &state); err != nil || state.Version != 1 {
		return fmt.Errorf("invalid faucet state")
	}
	issued, ok := new(big.Int).SetString(strings.TrimSpace(state.Issued), 10)
	if !ok || issued.Sign() < 0 {
		return fmt.Errorf("invalid faucet issued amount")
	}
	b.faucetAddress, b.faucetIP = make(map[string]time.Time), make(map[string]time.Time)
	for address, timestamp := range state.Addresses {
		b.faucetAddress[address] = time.Unix(timestamp, 0)
	}
	for ipHash, timestamp := range state.IPHashes {
		b.faucetIP[ipHash] = time.Unix(timestamp, 0)
	}
	b.faucetClaims, b.faucetDenied, b.faucetIssued = state.Claims, state.Denied, issued
	return nil
}

// persistFaucetStateLocked uses write+rename so a crash cannot replace the last
// valid anti-abuse checkpoint with a partially written JSON document.
func (b *BlockchainServer) persistFaucetStateLocked() error {
	state := persistentFaucetState{Version: 1, Addresses: map[string]int64{}, IPHashes: map[string]int64{}, Claims: b.faucetClaims, Denied: b.faucetDenied, Issued: b.faucetIssued.String()}
	for address, claimedAt := range b.faucetAddress {
		state.Addresses[address] = claimedAt.Unix()
	}
	for ipHash, claimedAt := range b.faucetIP {
		state.IPHashes[ipHash] = claimedAt.Unix()
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(b.faucetStatePath), 0o700); err != nil {
		return err
	}
	temporary := b.faucetStatePath + ".tmp"
	if err = os.WriteFile(temporary, raw, 0o600); err != nil {
		return err
	}
	if err = os.Rename(temporary, b.faucetStatePath); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return nil
}
