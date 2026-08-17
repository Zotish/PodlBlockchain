package blockchaincomponent

import (
	"context"
	"crypto/ecdsa"
	"encoding/asn1"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/miekg/pkcs11"
)

type PKCS11ValidatorSignerConfig struct {
	ModulePath   string
	TokenLabel   string
	PIN          string
	KeyLabel     string
	PublicKeyHex string
	SlotID       *uint
}

// PKCS11ValidatorSigningBackend keeps the validator secp256k1 key inside an
// OASIS PKCS#11 token. PublicKeyHex is required because PKCS#11 EC point
// encodings vary between vendors; pinning it also prevents selecting a wrong
// same-label key after an HSM restore.
type PKCS11ValidatorSigningBackend struct {
	mu        sync.Mutex
	ctx       *pkcs11.Ctx
	session   pkcs11.SessionHandle
	key       pkcs11.ObjectHandle
	publicKey *ecdsa.PublicKey
	address   string
	closed    bool
}

func NewPKCS11ValidatorSigner(cfg PKCS11ValidatorSignerConfig, vrfSecretHex, slashingPath string) (*ProtectedValidatorSigner, error) {
	if strings.TrimSpace(vrfSecretHex) == "" {
		return nil, fmt.Errorf("PKCS#11 signer requires a distinct configured P-256 VRF key")
	}
	backend, err := NewPKCS11ValidatorSigningBackend(cfg)
	if err != nil {
		return nil, err
	}
	vrf, err := newLocalP256VRFBackend(vrfSecretHex, nil)
	if err != nil {
		_ = backend.Close()
		return nil, fmt.Errorf("PKCS#11 signer requires a distinct configured P-256 VRF key: %w", err)
	}
	signer, err := NewProtectedValidatorSigner(backend, vrf, slashingPath)
	if err != nil {
		_ = backend.Close()
		return nil, err
	}
	return signer, nil
}

func NewPKCS11ValidatorSigningBackend(cfg PKCS11ValidatorSignerConfig) (*PKCS11ValidatorSigningBackend, error) {
	if strings.TrimSpace(cfg.ModulePath) == "" || strings.TrimSpace(cfg.KeyLabel) == "" || strings.TrimSpace(cfg.PublicKeyHex) == "" {
		return nil, fmt.Errorf("PKCS#11 module, key label and pinned public key are required")
	}
	publicRaw, err := hex.DecodeString(strings.TrimPrefix(strings.TrimSpace(cfg.PublicKeyHex), "0x"))
	if err != nil {
		return nil, fmt.Errorf("decode pinned HSM public key: %w", err)
	}
	publicKey, err := crypto.UnmarshalPubkey(publicRaw)
	if err != nil || publicKey.Curve != crypto.S256() {
		return nil, fmt.Errorf("pinned HSM public key must be uncompressed secp256k1")
	}
	p := pkcs11.New(strings.TrimSpace(cfg.ModulePath))
	if p == nil {
		return nil, fmt.Errorf("load PKCS#11 module")
	}
	if err := p.Initialize(); err != nil && err != pkcs11.Error(pkcs11.CKR_CRYPTOKI_ALREADY_INITIALIZED) {
		p.Destroy()
		return nil, fmt.Errorf("initialize PKCS#11 module: %w", err)
	}
	slots, err := p.GetSlotList(true)
	if err != nil {
		_ = p.Finalize()
		p.Destroy()
		return nil, fmt.Errorf("list PKCS#11 slots: %w", err)
	}
	var selected uint
	found := false
	for _, slot := range slots {
		if cfg.SlotID != nil && slot != *cfg.SlotID {
			continue
		}
		if cfg.SlotID == nil && strings.TrimSpace(cfg.TokenLabel) != "" {
			info, infoErr := p.GetTokenInfo(slot)
			if infoErr != nil || strings.TrimSpace(info.Label) != strings.TrimSpace(cfg.TokenLabel) {
				continue
			}
		}
		selected, found = slot, true
		break
	}
	if !found {
		_ = p.Finalize()
		p.Destroy()
		return nil, fmt.Errorf("configured PKCS#11 token was not found")
	}
	session, err := p.OpenSession(selected, pkcs11.CKF_SERIAL_SESSION|pkcs11.CKF_RW_SESSION)
	if err != nil {
		_ = p.Finalize()
		p.Destroy()
		return nil, fmt.Errorf("open PKCS#11 session: %w", err)
	}
	if strings.TrimSpace(cfg.PIN) != "" {
		if err := p.Login(session, pkcs11.CKU_USER, cfg.PIN); err != nil && err != pkcs11.Error(pkcs11.CKR_USER_ALREADY_LOGGED_IN) {
			_ = p.CloseSession(session)
			_ = p.Finalize()
			p.Destroy()
			return nil, fmt.Errorf("login to PKCS#11 token: %w", err)
		}
	}
	template := []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_CLASS, pkcs11.CKO_PRIVATE_KEY),
		pkcs11.NewAttribute(pkcs11.CKA_KEY_TYPE, pkcs11.CKK_EC),
		pkcs11.NewAttribute(pkcs11.CKA_LABEL, cfg.KeyLabel),
		pkcs11.NewAttribute(pkcs11.CKA_SIGN, true),
	}
	if err := p.FindObjectsInit(session, template); err != nil {
		_ = p.CloseSession(session)
		_ = p.Finalize()
		p.Destroy()
		return nil, fmt.Errorf("find HSM validator key: %w", err)
	}
	objects, _, findErr := p.FindObjects(session, 2)
	finalErr := p.FindObjectsFinal(session)
	if findErr != nil || finalErr != nil || len(objects) != 1 {
		_ = p.CloseSession(session)
		_ = p.Finalize()
		p.Destroy()
		return nil, fmt.Errorf("HSM key label must resolve to exactly one signing key")
	}
	return &PKCS11ValidatorSigningBackend{ctx: p, session: session, key: objects[0], publicKey: publicKey, address: strings.ToLower(crypto.PubkeyToAddress(*publicKey).Hex())}, nil
}

func (b *PKCS11ValidatorSigningBackend) Address() string     { return b.address }
func (b *PKCS11ValidatorSigningBackend) BackendName() string { return "pkcs11-hsm-secp256k1" }

type pkcs11ECDSASignature struct {
	R *big.Int
	S *big.Int
}

func parsePKCS11ECDSASignature(raw []byte) (*big.Int, *big.Int, error) {
	if len(raw) == 64 {
		return new(big.Int).SetBytes(raw[:32]), new(big.Int).SetBytes(raw[32:]), nil
	}
	parsed := pkcs11ECDSASignature{}
	if rest, err := asn1.Unmarshal(raw, &parsed); err != nil || len(rest) != 0 || parsed.R == nil || parsed.S == nil {
		return nil, nil, fmt.Errorf("HSM returned an unsupported ECDSA signature encoding")
	}
	return parsed.R, parsed.S, nil
}

func recoverableSecp256k1Signature(digest, hsmSignature []byte, expected *ecdsa.PublicKey) ([]byte, error) {
	if len(digest) != 32 || expected == nil {
		return nil, fmt.Errorf("digest and expected public key are required")
	}
	r, s, err := parsePKCS11ECDSASignature(hsmSignature)
	if err != nil {
		return nil, err
	}
	n := crypto.S256().Params().N
	if r.Sign() <= 0 || s.Sign() <= 0 || r.Cmp(n) >= 0 || s.Cmp(n) >= 0 {
		return nil, fmt.Errorf("HSM ECDSA signature scalar is outside secp256k1")
	}
	halfN := new(big.Int).Rsh(new(big.Int).Set(n), 1)
	if s.Cmp(halfN) > 0 {
		s.Sub(n, s)
	}
	compact := append(fixedWidthInt(r, 32), fixedWidthInt(s, 32)...)
	want := crypto.FromECDSAPub(expected)
	for recovery := byte(0); recovery <= 1; recovery++ {
		candidate := append(append([]byte(nil), compact...), recovery)
		pub, err := crypto.SigToPub(digest, candidate)
		if err == nil && strings.EqualFold(hex.EncodeToString(crypto.FromECDSAPub(pub)), hex.EncodeToString(want)) {
			return candidate, nil
		}
	}
	return nil, fmt.Errorf("HSM signature does not match pinned validator public key")
}

func (b *PKCS11ValidatorSigningBackend) SignDigest(ctx context.Context, digest []byte) ([]byte, error) {
	if b == nil || b.ctx == nil || len(digest) != 32 {
		return nil, fmt.Errorf("HSM signer requires a 32-byte digest")
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, fmt.Errorf("HSM signer is closed")
	}
	if err := b.ctx.SignInit(b.session, []*pkcs11.Mechanism{pkcs11.NewMechanism(pkcs11.CKM_ECDSA, nil)}, b.key); err != nil {
		return nil, fmt.Errorf("initialize HSM ECDSA signature: %w", err)
	}
	raw, err := b.ctx.Sign(b.session, digest)
	if err != nil {
		return nil, fmt.Errorf("HSM ECDSA signature failed: %w", err)
	}
	return recoverableSecp256k1Signature(digest, raw, b.publicKey)
}

func (b *PKCS11ValidatorSigningBackend) Close() error {
	if b == nil || b.ctx == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil
	}
	b.closed = true
	_ = b.ctx.Logout(b.session)
	closeErr := b.ctx.CloseSession(b.session)
	finalErr := b.ctx.Finalize()
	b.ctx.Destroy()
	if closeErr != nil {
		return closeErr
	}
	return finalErr
}
