package blockchaincomponent

import (
	"os"
	"path/filepath"
	"strings"
)

func appDataDir() string {
	if dir := strings.TrimSpace(os.Getenv("LQD_DATA_DIR")); dir != "" {
		return dir
	}
	if dir := strings.TrimSpace(os.Getenv("RAILWAY_VOLUME_MOUNT_PATH")); dir != "" {
		return dir
	}
	return "data"
}

func appDataPath(parts ...string) string {
	all := append([]string{appDataDir()}, parts...)
	return filepath.Join(all...)
}

func bridgeDataDir() string {
	if dir := strings.TrimSpace(os.Getenv("LQD_BRIDGE_DATA_DIR")); dir != "" {
		return dir
	}
	return appDataDir()
}

func contractArtifactsDir() string {
	return appDataPath("contracts")
}

func ContractArtifactsDir() string {
	return contractArtifactsDir()
}
