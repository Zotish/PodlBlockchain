package blockchaincomponent

import (
	"strings"
	"testing"

	"github.com/syndtr/goleveldb/leveldb"
)

func TestCurrentPluginRuntimeFingerprintStableAndNonEmpty(t *testing.T) {
	t.Parallel()

	a, err := CurrentPluginRuntimeFingerprint()
	if err != nil {
		t.Fatalf("CurrentPluginRuntimeFingerprint() error = %v", err)
	}
	b, err := CurrentPluginRuntimeFingerprint()
	if err != nil {
		t.Fatalf("CurrentPluginRuntimeFingerprint() second call error = %v", err)
	}
	if a == "" || b == "" {
		t.Fatal("expected non-empty runtime fingerprints")
	}
	if a != b {
		t.Fatalf("expected cached runtime fingerprint to be stable, got %q vs %q", a, b)
	}
}

func TestEnsurePluginLoadedRejectsFingerprintMismatch(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	db, err := leveldb.OpenFile(tempDir, nil)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	defer db.Close()

	reg := &ContractRegistry{
		DB:       &ContractDB{db: db},
		PluginVM: NewPluginVM(),
	}
	meta := &ContractMetadata{
		Address:            "0xplugin",
		Type:               "plugin",
		PluginPath:         "missing.so",
		RuntimeFingerprint: strings.Repeat("0", 64),
	}

	err = reg.EnsurePluginLoaded(meta.Address, meta)
	if err == nil {
		t.Fatal("expected fingerprint mismatch error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "fingerprint mismatch") {
		t.Fatalf("expected fingerprint mismatch error, got %v", err)
	}
}
