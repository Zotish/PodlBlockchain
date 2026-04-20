package blockchaincomponent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type BridgeRelayerCheckpoint struct {
	LastBurnChecked uint64 `json:"last_burn_checked"`
	LastLockChecked uint64 `json:"last_lock_checked"`
	ActiveRPC       string `json:"active_rpc,omitempty"`
	UpdatedAt       int64  `json:"updated_at"`
}

func bridgeRelayerCheckpointPath() string {
	if raw := strings.TrimSpace(os.Getenv("BRIDGE_STATE_FILE")); raw != "" {
		return raw
	}
	return appDataPath("bridge_relayer_state.json")
}

func bridgeRelayerCheckpointPathFor(chainID string) string {
	if raw := strings.TrimSpace(os.Getenv("BRIDGE_STATE_FILE")); raw != "" {
		return raw
	}
	chainID = strings.ToLower(strings.TrimSpace(chainID))
	if chainID == "" || chainID == "bsc-testnet" || chainID == "bsc" {
		return bridgeRelayerCheckpointPath()
	}
	return appDataPath("bridge_relayer_state_" + chainID + ".json")
}

func loadBridgeRelayerCheckpoint() (*BridgeRelayerCheckpoint, error) {
	path := bridgeRelayerCheckpointPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cp BridgeRelayerCheckpoint
	if err := json.Unmarshal(data, &cp); err != nil {
		return nil, err
	}
	return &cp, nil
}

func saveBridgeRelayerCheckpoint(cp *BridgeRelayerCheckpoint) error {
	if cp == nil {
		return nil
	}
	cp.UpdatedAt = time.Now().Unix()
	path := bridgeRelayerCheckpointPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func loadBridgeRelayerCheckpointFor(chainID string) (*BridgeRelayerCheckpoint, error) {
	path := bridgeRelayerCheckpointPathFor(chainID)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cp BridgeRelayerCheckpoint
	if err := json.Unmarshal(data, &cp); err != nil {
		return nil, err
	}
	return &cp, nil
}

func saveBridgeRelayerCheckpointFor(chainID string, cp *BridgeRelayerCheckpoint) error {
	if cp == nil {
		return nil
	}
	cp.UpdatedAt = time.Now().Unix()
	path := bridgeRelayerCheckpointPathFor(chainID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
