package blockchaincomponent

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	constantset "github.com/Zotish/DefenceProject/ConstantSet"
)

// DeployBridgeToken deploys the bridge token plugin and initializes metadata.
func (bc *Blockchain_struct) DeployBridgeToken(name, symbol, decimals, bscToken string) (string, error) {
	if bc.ContractEngine == nil || bc.ContractEngine.Registry == nil {
		return "", fmt.Errorf("contract engine not initialized")
	}

	owner := constantset.BridgeEscrowAddress
	addr := generateContractAddress(owner, uint64(time.Now().UnixNano()))

	pluginSrc := filepath.Join("contract", "bridge_token.so")
	if _, err := os.Stat(pluginSrc); err != nil {
		return "", fmt.Errorf("bridge_token.so not found (compile contract/bridge_token.go)")
	}
	pluginDir := filepath.Join("data", "contracts")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		return "", err
	}
	pluginPath := filepath.Join(pluginDir, addr+".so")
	if err := copyFile(pluginSrc, pluginPath); err != nil {
		return "", err
	}

	meta := &ContractMetadata{
		Address:   addr,
		Type:      "plugin",
		Owner:     owner,
		Timestamp: time.Now().Unix(),
		PluginPath: pluginPath,
	}

	if err := bc.ContractEngine.Registry.PluginVM.LoadPlugin(addr, pluginPath); err != nil {
		return "", err
	}
	pc := bc.ContractEngine.Registry.PluginVM.GetPlugin(addr)
	if pc == nil {
		return "", fmt.Errorf("plugin load failed")
	}
	abi, err := GenerateABIForPlugin(pc)
	if err != nil {
		return "", err
	}
	meta.ABI = abi
	meta.Pool = false
	meta.PoolType = ""

	state := &SmartContractState{
		Address:   addr,
		Balance:   "0",
		Storage:   map[string]string{},
		IsActive:  true,
		CreatedAt: time.Now().Unix(),
	}

	if err := bc.ContractEngine.Registry.RegisterContract(meta, state); err != nil {
		return "", err
	}

	// Initialize contract metadata
	_, err = bc.ContractEngine.Pipeline.Execute(addr, owner, "Init", []string{name, symbol, decimals, bscToken}, 5_000_000)
	if err != nil {
		return "", err
	}

	return addr, nil
}

func generateContractAddress(owner string, nonce uint64) string {
	input := owner + ":" + strconv.FormatUint(nonce, 10)
	sum := sha256.Sum256([]byte(input))
	return "0x" + hex.EncodeToString(sum[:20])
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		_ = out.Close()
	}()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Chmod(0o755)
}
