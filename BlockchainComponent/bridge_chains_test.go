package blockchaincomponent

import "testing"

func TestBridgeChainRegistryRoundTripUsesTempDataDir(t *testing.T) {
	t.Setenv("LQD_BRIDGE_DATA_DIR", t.TempDir())

	reg, err := LoadBridgeChainRegistry()
	if err != nil {
		t.Fatalf("LoadBridgeChainRegistry failed: %v", err)
	}
	if reg.ChainByID("bsc-testnet") == nil {
		t.Fatal("expected default BSC testnet preset to exist")
	}

	reg.Upsert(&BridgeChainConfig{
		ID:              "custom-utxo",
		Name:            "Custom UTXO",
		ChainID:         "custom-utxo",
		Family:          "utxo",
		Adapter:         "utxo",
		NativeSymbol:    "CUTXO",
		Enabled:         true,
		SupportsPublic:  true,
		SupportsPrivate: true,
	})
	if err := SaveBridgeChainRegistry(reg); err != nil {
		t.Fatalf("SaveBridgeChainRegistry failed: %v", err)
	}

	loaded, err := LoadBridgeChainRegistry()
	if err != nil {
		t.Fatalf("LoadBridgeChainRegistry reload failed: %v", err)
	}
	cfg := loaded.ChainByID("custom-utxo")
	if cfg == nil {
		t.Fatal("expected custom chain to survive round-trip")
	}
	if !cfg.Enabled || cfg.Family != "utxo" {
		t.Fatalf("unexpected chain config after reload: %+v", cfg)
	}

	loaded.Remove("custom-utxo")
	if err := SaveBridgeChainRegistry(loaded); err != nil {
		t.Fatalf("SaveBridgeChainRegistry remove failed: %v", err)
	}
	afterRemove, err := LoadBridgeChainRegistry()
	if err != nil {
		t.Fatalf("LoadBridgeChainRegistry after remove failed: %v", err)
	}
	if afterRemove.ChainByID("custom-utxo") != nil {
		t.Fatal("expected removed chain to be absent")
	}
}
