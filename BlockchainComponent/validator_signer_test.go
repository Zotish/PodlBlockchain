package blockchaincomponent

import (
	"context"
	"encoding/asn1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/crypto"
)

const testP256VRFSecret = "c9afa9d845ba75166b5c215767b1d6934e50c3db36e89b127b8a622b120f6721"

func newTestValidatorSigner(t *testing.T, slashingPath string) (*ProtectedValidatorSigner, string) {
	t.Helper()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	privateHex := hex.EncodeToString(crypto.FromECDSA(key))
	signer, err := NewLocalValidatorSigner(privateHex, testP256VRFSecret, slashingPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = signer.Close() })
	return signer, privateHex
}

func TestValidatorSignerDurableSlashingProtection(t *testing.T) {
	database := filepath.Join(t.TempDir(), "slashing.json")
	signer, privateHex := newTestValidatorSigner(t, database)
	ctx := context.Background()

	vote := ConsensusVote{Height: 99, Round: 3, Step: StepPrecommit, BlockHash: "0xaaa"}
	if err := SignConsensusVoteWithSigner(ctx, &vote, signer); err != nil {
		t.Fatal(err)
	}
	if !VerifyConsensusVote(vote) {
		t.Fatal("signer-produced consensus vote did not verify")
	}
	firstSignature := vote.Signature
	repeat := ConsensusVote{Height: 99, Round: 3, Step: StepPrecommit, BlockHash: "0xaaa"}
	if err := SignConsensusVoteWithSigner(ctx, &repeat, signer); err != nil {
		t.Fatalf("idempotent repeat was rejected: %v", err)
	}
	if repeat.Signature != firstSignature {
		t.Fatal("same slot and message did not return the persisted signature")
	}

	conflict := ConsensusVote{Height: 99, Round: 3, Step: StepPrecommit, BlockHash: "0xbbb"}
	if err := SignConsensusVoteWithSigner(ctx, &conflict, signer); err == nil || !strings.Contains(err.Error(), "conflicting") {
		t.Fatalf("conflicting vote was not rejected: %v", err)
	}

	if err := signer.Close(); err != nil {
		t.Fatal(err)
	}
	restarted, err := NewLocalValidatorSigner(privateHex, testP256VRFSecret, database)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	restartConflict := ConsensusVote{Height: 99, Round: 3, Step: StepPrecommit, BlockHash: "0xccc"}
	if err := SignConsensusVoteWithSigner(ctx, &restartConflict, restarted); err == nil {
		t.Fatal("conflicting vote was accepted after signer restart")
	}
	info, err := os.Stat(database)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("slashing database permissions = %o, want 600", info.Mode().Perm())
	}
	databaseInfo, err := os.Stat(database + ".leveldb")
	if err != nil {
		t.Fatal(err)
	}
	if databaseInfo.Mode().Perm() != 0o700 || !databaseInfo.IsDir() {
		t.Fatalf("slashing LevelDB mode=%o is_dir=%t", databaseInfo.Mode().Perm(), databaseInfo.IsDir())
	}
}

func TestDurableSlashingProtectorMigratesLegacySnapshotToLevelDB(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "slashing.json")
	digest := strings.Repeat("ab", 32)
	legacy := signerSlashingState{Version: 1, Records: map[string]signerSlashingRecord{
		"vote/9/0/prevote": {Digest: "0x" + digest, Signature: "0xlegacy", Prepared: 10},
	}}
	raw, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	protector, err := NewDurableSlashingProtector(path)
	if err != nil {
		t.Fatal(err)
	}
	decodedDigest, _ := hex.DecodeString(digest)
	if cached, err := protector.prepare("vote/9/0/prevote", decodedDigest); err != nil || cached != "0xlegacy" {
		t.Fatalf("legacy record was not loaded: cached=%q err=%v", cached, err)
	}
	newDigest := make([]byte, 32)
	newDigest[31] = 1
	if _, err := protector.prepare("vote/10/0/prevote", newDigest); err != nil {
		t.Fatal(err)
	}
	if err := protector.commit("vote/10/0/prevote", newDigest, "0xnew"); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(raw) {
		t.Fatal("legacy snapshot was rewritten while appending a new signing record")
	}
	if err := protector.Close(); err != nil {
		t.Fatal(err)
	}
	restarted, err := NewDurableSlashingProtector(path)
	if err != nil {
		t.Fatal(err)
	}
	if cached, err := restarted.prepare("vote/10/0/prevote", newDigest); err != nil || cached != "0xnew" {
		t.Fatalf("LevelDB record was not recovered: cached=%q err=%v", cached, err)
	}
	conflict := make([]byte, 32)
	conflict[31] = 2
	if _, err := restarted.prepare("vote/10/0/prevote", conflict); err == nil {
		t.Fatal("LevelDB-backed protector accepted a conflicting digest")
	}
	if err := restarted.Close(); err != nil {
		t.Fatal(err)
	}
}

func BenchmarkDurableSlashingProtectorLevelDBLargeHistory(b *testing.B) {
	path := filepath.Join(b.TempDir(), "slashing.json")
	records := make(map[string]signerSlashingRecord, 100_000)
	for i := 0; i < 100_000; i++ {
		records[fmt.Sprintf("vote/%d/0/prevote", i)] = signerSlashingRecord{Digest: "0x" + strings.Repeat("01", 32), Prepared: 1}
	}
	raw, err := json.Marshal(signerSlashingState{Version: 1, Records: records})
	if err != nil {
		b.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		b.Fatal(err)
	}
	protector, err := NewDurableSlashingProtector(path)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = protector.Close() })
	digest := make([]byte, 32)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		digest[24] = byte(i >> 8)
		digest[25] = byte(i)
		if _, err := protector.prepare(fmt.Sprintf("vote/new/%d/prevote", i), digest); err != nil {
			b.Fatal(err)
		}
	}
}

func TestValidatorSignerTimeoutAndDomainSeparation(t *testing.T) {
	signer, _ := newTestValidatorSigner(t, filepath.Join(t.TempDir(), "slashing.json"))
	vote := ConsensusTimeoutVote{Height: 17, Round: 4}
	if err := SignConsensusTimeoutVoteWithSigner(context.Background(), &vote, signer); err != nil {
		t.Fatal(err)
	}
	if !VerifyConsensusTimeoutVote(vote) {
		t.Fatal("signer-produced timeout vote did not verify")
	}
	if _, err := signer.SignMessage(context.Background(), "arbitrary", []byte("PODL-BFT:anything"), "bad/slot"); err == nil {
		t.Fatal("unknown signer domain was accepted")
	}
	if _, err := signer.SignMessage(context.Background(), SignerDomainConsensusVote, []byte("not-a-vote"), "bad/payload"); err == nil {
		t.Fatal("cross-domain payload was accepted")
	}
}

func TestValidatorSignerVRFBindingAndConflicts(t *testing.T) {
	signer, _ := newTestValidatorSigner(t, filepath.Join(t.TempDir(), "slashing.json"))
	alpha := []byte("parent-bound consensus alpha")
	result, err := signer.ProveVRF(context.Background(), alpha, "200/1")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyValidatorVRFResult(signer.Address(), alpha, result) {
		t.Fatal("validator-bound RFC 9381 proof did not verify")
	}
	if VerifyValidatorVRFResult(signer.Address(), []byte("wrong alpha"), result) {
		t.Fatal("VRF proof verified for the wrong alpha")
	}
	repeated, err := signer.ProveVRF(context.Background(), alpha, "200/1")
	if err != nil || repeated.Proof != result.Proof || repeated.Output != result.Output {
		t.Fatalf("deterministic repeat failed: result=%+v err=%v", repeated, err)
	}
	if _, err := signer.ProveVRF(context.Background(), []byte("conflict"), "200/1"); err == nil {
		t.Fatal("conflicting VRF input for one slot was accepted")
	}
	tampered := result
	tampered.KeyBinding = "0x" + strings.Repeat("00", 65)
	if VerifyValidatorVRFResult(signer.Address(), alpha, tampered) {
		t.Fatal("VRF proof with tampered validator-key binding verified")
	}
}

func TestEncryptedValidatorKeyFileRoundTrip(t *testing.T) {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	privateHex := hex.EncodeToString(crypto.FromECDSA(key))
	path := filepath.Join(t.TempDir(), "validator-key.json")
	passphrase := "correct horse battery staple"
	if err := os.WriteFile(path, []byte("old-insecure-file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteEncryptedValidatorKeyFile(path, privateHex, testP256VRFSecret, passphrase); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("encrypted key permissions = %o, want 600", info.Mode().Perm())
	}
	signer, err := LoadEncryptedLocalValidatorSigner(path, passphrase, filepath.Join(t.TempDir(), "slashing.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer signer.Close()
	want := strings.ToLower(crypto.PubkeyToAddress(key.PublicKey).Hex())
	if signer.Address() != want {
		t.Fatalf("loaded signer address = %s, want %s", signer.Address(), want)
	}
	if _, err := LoadEncryptedLocalValidatorSigner(path, "definitely-wrong-passphrase", ""); err == nil {
		t.Fatal("wrong encrypted-key passphrase was accepted")
	}
}

func TestPKCS11SignatureEncodingConversion(t *testing.T) {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	digest := accounts.TextHash([]byte("PODL-BFT:1:0:prevote:0xabc"))
	signature, err := crypto.Sign(digest, key)
	if err != nil {
		t.Fatal(err)
	}
	for name, raw := range map[string][]byte{
		"pkcs11-raw": append([]byte(nil), signature[:64]...),
		"asn1-der": func() []byte {
			encoded, marshalErr := asn1.Marshal(pkcs11ECDSASignature{R: new(big.Int).SetBytes(signature[:32]), S: new(big.Int).SetBytes(signature[32:64])})
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			return encoded
		}(),
	} {
		t.Run(name, func(t *testing.T) {
			recoverable, err := recoverableSecp256k1Signature(digest, raw, &key.PublicKey)
			if err != nil {
				t.Fatal(err)
			}
			address, ok := recoverSignerAddress(digest, recoverable)
			if !ok || !strings.EqualFold(address, crypto.PubkeyToAddress(key.PublicKey).Hex()) {
				t.Fatal("converted HSM signature did not recover the pinned key")
			}
		})
	}
}

func TestCorruptSlashingDatabaseFailsClosed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "slashing.json")
	if err := os.WriteFile(path, []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewDurableSlashingProtector(path); err == nil {
		t.Fatal("corrupt slashing database was accepted")
	}
}
