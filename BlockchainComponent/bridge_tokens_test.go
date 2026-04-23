package blockchaincomponent

import (
	"path/filepath"
	"testing"
)

func TestListBridgeTokenMappingsDeduplicatesLegacyAliases(t *testing.T) {
	bc := &Blockchain_struct{
		BridgeTokenMap: make(map[string]*BridgeTokenInfo),
	}
	info := &BridgeTokenInfo{
		ChainID:     "bsc-testnet",
		SourceToken: "0xabc",
		TargetToken: "0xlqd",
		LqdToken:    "0xlqd",
		Name:        "Token",
	}
	bc.SetBridgeTokenMappingForChain("bsc-testnet", "0xAbC", info)

	got := bc.ListBridgeTokenMappings()
	if len(got) != 1 {
		t.Fatalf("expected 1 deduplicated token mapping, got %d", len(got))
	}
	if got[0].SourceToken != "0xabc" {
		t.Fatalf("expected normalized source token, got %q", got[0].SourceToken)
	}
}

func TestBridgeTokenRegistryRoundTripAndStateLoad(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("LQD_BRIDGE_DATA_DIR", tempDir)

	reg := defaultBridgeTokenRegistry()
	reg.Upsert(&BridgeTokenInfo{
		ChainID:       "bsc-testnet",
		ChainName:     "BSC Testnet",
		SourceToken:   "0xsource",
		TargetChainID: "lqd",
		TargetToken:   "0xtarget",
		LqdToken:      "0xtarget",
		Name:          "Temp Token",
		Symbol:        "TMP",
		Decimals:      "18",
	})
	if err := SaveBridgeTokenRegistry(reg); err != nil {
		t.Fatalf("SaveBridgeTokenRegistry failed: %v", err)
	}

	if _, err := filepath.Abs(bridgeTokenRegistryPath()); err != nil {
		t.Fatalf("bridge token path should be resolvable: %v", err)
	}

	loaded, err := LoadBridgeTokenRegistry()
	if err != nil {
		t.Fatalf("LoadBridgeTokenRegistry failed: %v", err)
	}
	if len(loaded.List()) != 1 {
		t.Fatalf("expected 1 token in registry, got %d", len(loaded.List()))
	}

	bc := &Blockchain_struct{BridgeTokenMap: make(map[string]*BridgeTokenInfo)}
	if err := bc.LoadBridgeTokenRegistryIntoState(); err != nil {
		t.Fatalf("LoadBridgeTokenRegistryIntoState failed: %v", err)
	}
	info := bc.GetBridgeTokenMappingForChain("bsc-testnet", "0xsource")
	if info == nil {
		t.Fatal("expected bridge token mapping to load into blockchain state")
	}
	if info.TargetToken != "0xtarget" {
		t.Fatalf("expected target token to survive round-trip, got %q", info.TargetToken)
	}
}
