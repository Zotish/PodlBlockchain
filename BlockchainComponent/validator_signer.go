package blockchaincomponent

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/crypto"
	"golang.org/x/crypto/scrypt"
)

const (
	SignerDomainConsensusVote    = "consensus_vote"
	SignerDomainConsensusTimeout = "consensus_timeout"
	SignerDomainVRFBinding       = "vrf_key_binding"
	SignerDomainP2PHandshake     = "p2p_handshake"
	SignerDomainInvestorEvidence = "investor_evidence"
)

type ValidatorSignerStatus struct {
	Address            string `json:"address"`
	Backend            string `json:"backend"`
	VRFSuite           string `json:"vrf_suite"`
	VRFPublicKey       string `json:"vrf_public_key"`
	SlashingProtection bool   `json:"slashing_protection"`
	Healthy            bool   `json:"healthy"`
	Detail             string `json:"detail,omitempty"`
}

type ValidatorVRFResult struct {
	Suite      string `json:"suite"`
	PublicKey  string `json:"public_key"`
	Proof      string `json:"proof"`
	Output     string `json:"output"`
	KeyBinding string `json:"key_binding"`
}

// ValidatorSigner is the only interface through which validator identity keys
// are used. Implementations may keep the key locally, in a remote mTLS service,
// or inside a PKCS#11 HSM. SignMessage always returns a recoverable secp256k1
// signature over accounts.TextHash(message).
type ValidatorSigner interface {
	Address() string
	SignMessage(ctx context.Context, domain string, message []byte, slot string) (string, error)
	ProveVRF(ctx context.Context, alpha []byte, slot string) (ValidatorVRFResult, error)
	Status(ctx context.Context) ValidatorSignerStatus
	Close() error
}

type validatorSigningBackend interface {
	Address() string
	SignDigest(context.Context, []byte) ([]byte, error)
	BackendName() string
	Close() error
}

type validatorVRFBackend interface {
	Suite() string
	PublicKey() []byte
	Prove([]byte) ([]byte, []byte, error)
}

type localSecp256k1Backend struct {
	key *ecdsa.PrivateKey
}

func newLocalSecp256k1Backend(privateKeyHex string) (*localSecp256k1Backend, error) {
	key, err := crypto.HexToECDSA(strings.TrimPrefix(strings.TrimSpace(privateKeyHex), "0x"))
	if err != nil {
		return nil, fmt.Errorf("invalid validator private key: %w", err)
	}
	return &localSecp256k1Backend{key: key}, nil
}

func (b *localSecp256k1Backend) Address() string {
	if b == nil || b.key == nil {
		return ""
	}
	return strings.ToLower(crypto.PubkeyToAddress(b.key.PublicKey).Hex())
}

func (b *localSecp256k1Backend) SignDigest(_ context.Context, digest []byte) ([]byte, error) {
	if b == nil || b.key == nil || len(digest) != 32 {
		return nil, fmt.Errorf("local signer requires a 32-byte digest")
	}
	return crypto.Sign(digest, b.key)
}

func (b *localSecp256k1Backend) BackendName() string { return "local-secp256k1" }
func (b *localSecp256k1Backend) Close() error {
	if b != nil && b.key != nil {
		b.key.D.SetInt64(0)
		b.key = nil
	}
	return nil
}

type localP256VRFBackend struct {
	secret []byte
	public []byte
}

func newLocalP256VRFBackend(secretHex string, identityMaterial []byte) (*localP256VRFBackend, error) {
	var secret []byte
	var err error
	if strings.TrimSpace(secretHex) != "" {
		secret, err = decodeFixedHex(secretHex, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid P-256 VRF secret: %w", err)
		}
	} else {
		// Backward-compatible deterministic derivation. Production remote/HSM
		// deployments should configure a distinct P-256 key in the signer.
		digest := sha256.Sum256(append([]byte("PODL-P256-VRF-DERIVE-V1:"), identityMaterial...))
		n := elliptic.P256().Params().N
		scalar := new(big.Int).SetBytes(digest[:])
		scalar.Mod(scalar, new(big.Int).Sub(n, big.NewInt(1)))
		scalar.Add(scalar, big.NewInt(1))
		secret = fixedWidthInt(scalar, 32)
	}
	if _, err := parseP256Secret(secret); err != nil {
		return nil, err
	}
	public, err := P256VRFPublicKey(secret)
	if err != nil {
		return nil, err
	}
	return &localP256VRFBackend{secret: append([]byte(nil), secret...), public: public}, nil
}

func (b *localP256VRFBackend) Suite() string     { return ECVRFP256SHA256TAI }
func (b *localP256VRFBackend) PublicKey() []byte { return append([]byte(nil), b.public...) }
func (b *localP256VRFBackend) Prove(alpha []byte) ([]byte, []byte, error) {
	proof, output, _, err := ECVRFP256Prove(b.secret, alpha)
	return proof, output, err
}
func (b *localP256VRFBackend) Close() {
	if b == nil {
		return
	}
	for i := range b.secret {
		b.secret[i] = 0
	}
	b.secret = nil
}

type signerSlashingRecord struct {
	Digest    string `json:"digest"`
	Signature string `json:"signature,omitempty"`
	Prepared  int64  `json:"prepared_at"`
}

type signerSlashingState struct {
	Version uint32                          `json:"version"`
	Records map[string]signerSlashingRecord `json:"records"`
}

// DurableSlashingProtector writes an intent before asking a key backend to
// sign. A crash after intent creation can repeat the same message but can never
// authorize a conflicting message for the same consensus slot.
type DurableSlashingProtector struct {
	mu      sync.Mutex
	path    string
	records map[string]signerSlashingRecord
}

func NewDurableSlashingProtector(path string) (*DurableSlashingProtector, error) {
	p := &DurableSlashingProtector{path: strings.TrimSpace(path), records: map[string]signerSlashingRecord{}}
	if p.path == "" {
		return p, nil
	}
	raw, err := os.ReadFile(p.path)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read signer slashing database: %w", err)
	}
	if len(raw) > 0 {
		state := signerSlashingState{}
		if err := json.Unmarshal(raw, &state); err != nil || state.Version != 1 || state.Records == nil {
			return nil, fmt.Errorf("signer slashing database is corrupt or unsupported")
		}
		p.records = state.Records
	}
	return p, nil
}

func (p *DurableSlashingProtector) enabled() bool { return p != nil && p.path != "" }

func (p *DurableSlashingProtector) persistLocked() error {
	if !p.enabled() {
		return nil
	}
	dir := filepath.Dir(p.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	raw, err := json.Marshal(signerSlashingState{Version: 1, Records: p.records})
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".podl-signer-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err = tmp.Chmod(0o600); err == nil {
		_, err = tmp.Write(raw)
	}
	if err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err := os.Rename(tmpName, p.path); err != nil {
		return err
	}
	if directory, err := os.Open(dir); err == nil {
		_ = directory.Sync()
		_ = directory.Close()
	}
	return nil
}

func (p *DurableSlashingProtector) prepare(slot string, digest []byte) (string, error) {
	if p == nil || strings.TrimSpace(slot) == "" {
		return "", nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	key := strings.TrimSpace(slot)
	digestHex := "0x" + hex.EncodeToString(digest)
	if record, exists := p.records[key]; exists {
		if !strings.EqualFold(record.Digest, digestHex) {
			return "", fmt.Errorf("slashing protection rejected conflicting signature for slot %s", key)
		}
		return record.Signature, nil
	}
	p.records[key] = signerSlashingRecord{Digest: digestHex, Prepared: time.Now().Unix()}
	if err := p.persistLocked(); err != nil {
		delete(p.records, key)
		return "", fmt.Errorf("persist signer intent: %w", err)
	}
	return "", nil
}

func (p *DurableSlashingProtector) commit(slot string, digest []byte, signature string) error {
	if p == nil || strings.TrimSpace(slot) == "" {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	key := strings.TrimSpace(slot)
	record, exists := p.records[key]
	if !exists || !strings.EqualFold(record.Digest, "0x"+hex.EncodeToString(digest)) {
		return fmt.Errorf("signer intent disappeared before commit")
	}
	record.Signature = signature
	p.records[key] = record
	return p.persistLocked()
}

type ProtectedValidatorSigner struct {
	backend   validatorSigningBackend
	vrf       validatorVRFBackend
	protector *DurableSlashingProtector
}

func NewProtectedValidatorSigner(backend validatorSigningBackend, vrf validatorVRFBackend, slashingPath string) (*ProtectedValidatorSigner, error) {
	if backend == nil || !ValidateAddress(backend.Address()) || vrf == nil {
		return nil, fmt.Errorf("validator signing and VRF backends are required")
	}
	protector, err := NewDurableSlashingProtector(slashingPath)
	if err != nil {
		return nil, err
	}
	return &ProtectedValidatorSigner{backend: backend, vrf: vrf, protector: protector}, nil
}

func NewLocalValidatorSigner(privateKeyHex, vrfSecretHex, slashingPath string) (*ProtectedValidatorSigner, error) {
	backend, err := newLocalSecp256k1Backend(privateKeyHex)
	if err != nil {
		return nil, err
	}
	identity, _ := hex.DecodeString(strings.TrimPrefix(strings.TrimSpace(privateKeyHex), "0x"))
	vrf, err := newLocalP256VRFBackend(vrfSecretHex, identity)
	if err != nil {
		_ = backend.Close()
		return nil, err
	}
	return NewProtectedValidatorSigner(backend, vrf, slashingPath)
}

func (s *ProtectedValidatorSigner) Address() string {
	if s == nil || s.backend == nil {
		return ""
	}
	return strings.ToLower(s.backend.Address())
}

func validateSignerDomainMessage(domain string, message []byte) error {
	text := string(message)
	switch domain {
	case SignerDomainConsensusVote:
		if !strings.HasPrefix(text, "PODL-BFT:") {
			return fmt.Errorf("consensus vote domain requires PODL-BFT payload")
		}
	case SignerDomainConsensusTimeout:
		if !strings.HasPrefix(text, "PODL-BFT-TIMEOUT:") {
			return fmt.Errorf("timeout domain requires PODL-BFT-TIMEOUT payload")
		}
	case SignerDomainVRFBinding:
		if !strings.HasPrefix(text, "PODL-VRF-KEY-BINDING-V1:") {
			return fmt.Errorf("VRF binding domain requires a key-binding payload")
		}
	case SignerDomainP2PHandshake:
		if !strings.HasPrefix(text, "PODL-P2P:") {
			return fmt.Errorf("P2P domain requires PODL-P2P payload")
		}
	case SignerDomainInvestorEvidence:
		var payload map[string]interface{}
		if json.Unmarshal(message, &payload) != nil || payload["domain"] != "PODL-INVESTOR-EVIDENCE-V1" {
			return fmt.Errorf("invalid investor evidence payload")
		}
	default:
		return fmt.Errorf("signer domain %q is not allowed", domain)
	}
	return nil
}

func recoverSignerAddress(digest, signature []byte) (string, bool) {
	if len(digest) != 32 || len(signature) != 65 {
		return "", false
	}
	pub, err := crypto.SigToPub(digest, signature)
	if err != nil {
		return "", false
	}
	return strings.ToLower(crypto.PubkeyToAddress(*pub).Hex()), true
}

func (s *ProtectedValidatorSigner) SignMessage(ctx context.Context, domain string, message []byte, slot string) (string, error) {
	if s == nil || s.backend == nil {
		return "", fmt.Errorf("validator signer is unavailable")
	}
	if err := validateSignerDomainMessage(domain, message); err != nil {
		return "", err
	}
	digest := accounts.TextHash(message)
	if cached, err := s.protector.prepare(slot, digest); err != nil {
		return "", err
	} else if cached != "" {
		return cached, nil
	}
	signature, err := s.backend.SignDigest(ctx, digest)
	if err != nil {
		return "", err
	}
	address, ok := recoverSignerAddress(digest, signature)
	if !ok || !strings.EqualFold(address, s.Address()) {
		return "", fmt.Errorf("signer backend returned a signature for the wrong validator")
	}
	encoded := "0x" + hex.EncodeToString(signature)
	if err := s.protector.commit(slot, digest, encoded); err != nil {
		return "", err
	}
	return encoded, nil
}

func vrfBindingMessage(validator, suite, publicKey string) string {
	return fmt.Sprintf("PODL-VRF-KEY-BINDING-V1:%s:%s:%s", strings.ToLower(validator), suite, strings.ToLower(publicKey))
}

func (s *ProtectedValidatorSigner) ProveVRF(ctx context.Context, alpha []byte, slot string) (ValidatorVRFResult, error) {
	if s == nil || s.vrf == nil {
		return ValidatorVRFResult{}, fmt.Errorf("validator VRF backend is unavailable")
	}
	digest := sha256.Sum256(append([]byte("PODL-VRF-SLASHING-V1:"), alpha...))
	if _, err := s.protector.prepare("vrf/"+slot, digest[:]); err != nil {
		return ValidatorVRFResult{}, err
	}
	proof, output, err := s.vrf.Prove(alpha)
	if err != nil {
		return ValidatorVRFResult{}, err
	}
	publicKey := "0x" + hex.EncodeToString(s.vrf.PublicKey())
	bindingMessage := []byte(vrfBindingMessage(s.Address(), s.vrf.Suite(), publicKey))
	binding, err := s.SignMessage(ctx, SignerDomainVRFBinding, bindingMessage, "vrf-binding/"+strings.ToLower(publicKey))
	if err != nil {
		return ValidatorVRFResult{}, err
	}
	result := ValidatorVRFResult{Suite: s.vrf.Suite(), PublicKey: publicKey, Proof: "0x" + hex.EncodeToString(proof), Output: "0x" + hex.EncodeToString(output), KeyBinding: binding}
	encoded, _ := json.Marshal(result)
	if err := s.protector.commit("vrf/"+slot, digest[:], "0x"+hex.EncodeToString(encoded)); err != nil {
		return ValidatorVRFResult{}, err
	}
	return result, nil
}

func (s *ProtectedValidatorSigner) Status(_ context.Context) ValidatorSignerStatus {
	if s == nil || s.backend == nil || s.vrf == nil {
		return ValidatorSignerStatus{Healthy: false, Detail: "validator signing or VRF backend is unavailable"}
	}
	status := ValidatorSignerStatus{Address: s.Address(), Backend: s.backend.BackendName(), VRFSuite: s.vrf.Suite(), VRFPublicKey: "0x" + hex.EncodeToString(s.vrf.PublicKey()), Healthy: true}
	status.SlashingProtection = s.protector != nil && s.protector.enabled()
	return status
}

func (s *ProtectedValidatorSigner) Close() error {
	if s == nil {
		return nil
	}
	if closer, ok := s.vrf.(interface{ Close() }); ok {
		closer.Close()
	}
	if s.backend == nil {
		return nil
	}
	return s.backend.Close()
}

type encryptedValidatorKeyFile struct {
	Version    uint32 `json:"version"`
	ScryptN    int    `json:"scrypt_n"`
	ScryptR    int    `json:"scrypt_r"`
	ScryptP    int    `json:"scrypt_p"`
	Salt       string `json:"salt"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

type validatorKeyMaterial struct {
	PrivateKey string `json:"private_key"`
	VRFSecret  string `json:"vrf_secret"`
}

func writePrivateFileAtomic(path string, raw []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".podl-private-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err = tmp.Chmod(0o600); err == nil {
		_, err = tmp.Write(raw)
	}
	if err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	if directory, err := os.Open(dir); err == nil {
		_ = directory.Sync()
		_ = directory.Close()
	}
	return nil
}

// WriteEncryptedValidatorKeyFile stores local fallback material using scrypt
// and AES-256-GCM. Production validators should prefer a remote PKCS#11 signer.
func WriteEncryptedValidatorKeyFile(path, privateKeyHex, vrfSecretHex, passphrase string) error {
	if strings.TrimSpace(path) == "" || len(passphrase) < 12 {
		return fmt.Errorf("key path and a passphrase of at least 12 characters are required")
	}
	if _, err := newLocalSecp256k1Backend(privateKeyHex); err != nil {
		return err
	}
	if strings.TrimSpace(vrfSecretHex) != "" {
		if _, err := decodeFixedHex(vrfSecretHex, 32); err != nil {
			return err
		}
	}
	plain, _ := json.Marshal(validatorKeyMaterial{PrivateKey: privateKeyHex, VRFSecret: vrfSecretHex})
	salt := make([]byte, 32)
	nonce := make([]byte, 12)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	const n, r, p = 1 << 17, 8, 1
	key, err := scrypt.Key([]byte(passphrase), salt, n, r, p, 32)
	if err != nil {
		return err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	ciphertext := aead.Seal(nil, nonce, plain, []byte("PODL-VALIDATOR-KEY-V1"))
	file := encryptedValidatorKeyFile{Version: 1, ScryptN: n, ScryptR: r, ScryptP: p, Salt: base64.StdEncoding.EncodeToString(salt), Nonce: base64.StdEncoding.EncodeToString(nonce), Ciphertext: base64.StdEncoding.EncodeToString(ciphertext)}
	raw, _ := json.MarshalIndent(file, "", "  ")
	return writePrivateFileAtomic(path, raw)
}

func LoadEncryptedLocalValidatorSigner(path, passphrase, slashingPath string) (*ProtectedValidatorSigner, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	file := encryptedValidatorKeyFile{}
	if err := json.Unmarshal(raw, &file); err != nil || file.Version != 1 || file.ScryptN < 1<<15 || file.ScryptN > 1<<20 || file.ScryptR <= 0 || file.ScryptR > 32 || file.ScryptP <= 0 || file.ScryptP > 16 {
		return nil, fmt.Errorf("invalid encrypted validator key file")
	}
	salt, err1 := base64.StdEncoding.DecodeString(file.Salt)
	nonce, err2 := base64.StdEncoding.DecodeString(file.Nonce)
	ciphertext, err3 := base64.StdEncoding.DecodeString(file.Ciphertext)
	if err1 != nil || err2 != nil || err3 != nil || len(salt) != 32 || len(nonce) != 12 || len(ciphertext) < 16 {
		return nil, fmt.Errorf("invalid encrypted validator key encoding")
	}
	key, err := scrypt.Key([]byte(passphrase), salt, file.ScryptN, file.ScryptR, file.ScryptP, 32)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	plain, err := aead.Open(nil, nonce, ciphertext, []byte("PODL-VALIDATOR-KEY-V1"))
	if err != nil {
		return nil, fmt.Errorf("validator key decryption failed")
	}
	material := validatorKeyMaterial{}
	if err := json.Unmarshal(plain, &material); err != nil {
		return nil, fmt.Errorf("invalid decrypted validator key material")
	}
	return NewLocalValidatorSigner(material.PrivateKey, material.VRFSecret, slashingPath)
}
