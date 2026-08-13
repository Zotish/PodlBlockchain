package blockchaincomponent

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/crypto"
)

type BridgeRiskPolicy struct {
	EnforceAttestations     bool              `json:"enforce_attestations"`
	RequireLightClientProof bool              `json:"require_light_client_proof"`
	RequiredSigners         int               `json:"required_signers"`
	AllowedSigners          map[string]bool   `json:"allowed_signers"`
	PerTransactionCaps      map[string]string `json:"per_transaction_caps"`
	DailyCaps               map[string]string `json:"daily_caps"`
	HourlyCaps              map[string]string `json:"hourly_caps,omitempty"`
	AssetTransactionCaps    map[string]string `json:"asset_transaction_caps,omitempty"`
	AssetDailyCaps          map[string]string `json:"asset_daily_caps,omitempty"`
	AttestationTTL          int64             `json:"attestation_ttl_seconds"`
}

func (p BridgeRiskPolicy) Validate() error {
	if p.RequiredSigners < 1 || p.RequiredSigners > 100 || p.AttestationTTL < 60 || p.AttestationTTL > 86400 {
		return fmt.Errorf("invalid bridge threshold or attestation ttl")
	}
	if p.EnforceAttestations && (p.RequiredSigners < 3 || len(p.AllowedSigners) < p.RequiredSigners) {
		return fmt.Errorf("enforced bridge policy requires at least three independent signer addresses")
	}
	for _, limits := range []map[string]string{p.PerTransactionCaps, p.DailyCaps, p.HourlyCaps, p.AssetTransactionCaps, p.AssetDailyCaps} {
		for _, raw := range limits {
			amount, ok := new(big.Int).SetString(strings.TrimSpace(raw), 10)
			if !ok || amount.Sign() <= 0 {
				return fmt.Errorf("bridge caps must be positive integers")
			}
		}
	}
	return nil
}

type BridgeAttestation struct {
	RequestID       string `json:"request_id"`
	SourceChainID   string `json:"source_chain_id"`
	SourceTxHash    string `json:"source_tx_hash"`
	SourceBlockHash string `json:"source_block_hash"`
	EventIndex      string `json:"event_index"`
	Confirmations   int    `json:"confirmations"`
	ObservedAt      int64  `json:"observed_at"`
	Signer          string `json:"signer"`
	Signature       string `json:"signature"`
}

type BridgeSecurityState struct {
	Policy          BridgeRiskPolicy                        `json:"policy"`
	Attestations    map[string]map[string]BridgeAttestation `json:"attestations,omitempty"`
	Authorized      map[string]bool                         `json:"authorized,omitempty"`
	Consumed        map[string]bool                         `json:"consumed_source_events,omitempty"`
	DailyVolume     map[string]*big.Int                     `json:"daily_volume,omitempty"`
	HourlyVolume    map[string]*big.Int                     `json:"hourly_volume,omitempty"`
	LightClients    map[string]*BridgeLightClient           `json:"light_clients,omitempty"`
	ProofAuthorized map[string]bool                         `json:"proof_authorized,omitempty"`
}

func NewBridgeSecurityState() *BridgeSecurityState {
	return &BridgeSecurityState{Policy: BridgeRiskPolicy{EnforceAttestations: false, RequiredSigners: 3, AllowedSigners: map[string]bool{}, PerTransactionCaps: map[string]string{}, DailyCaps: map[string]string{}, HourlyCaps: map[string]string{}, AssetTransactionCaps: map[string]string{}, AssetDailyCaps: map[string]string{}, AttestationTTL: 900}, Attestations: map[string]map[string]BridgeAttestation{}, Authorized: map[string]bool{}, Consumed: map[string]bool{}, DailyVolume: map[string]*big.Int{}, HourlyVolume: map[string]*big.Int{}, LightClients: map[string]*BridgeLightClient{}, ProofAuthorized: map[string]bool{}}
}

func (s *BridgeSecurityState) ensure() {
	if s.Policy.RequiredSigners <= 0 {
		s.Policy.RequiredSigners = 3
	}
	if s.Policy.AttestationTTL <= 0 {
		s.Policy.AttestationTTL = 900
	}
	if s.Policy.AllowedSigners == nil {
		s.Policy.AllowedSigners = map[string]bool{}
	}
	if s.Policy.PerTransactionCaps == nil {
		s.Policy.PerTransactionCaps = map[string]string{}
	}
	if s.Policy.DailyCaps == nil {
		s.Policy.DailyCaps = map[string]string{}
	}
	if s.Policy.HourlyCaps == nil {
		s.Policy.HourlyCaps = map[string]string{}
	}
	if s.Policy.AssetTransactionCaps == nil {
		s.Policy.AssetTransactionCaps = map[string]string{}
	}
	if s.Policy.AssetDailyCaps == nil {
		s.Policy.AssetDailyCaps = map[string]string{}
	}
	if s.Attestations == nil {
		s.Attestations = map[string]map[string]BridgeAttestation{}
	}
	if s.Authorized == nil {
		s.Authorized = map[string]bool{}
	}
	if s.Consumed == nil {
		s.Consumed = map[string]bool{}
	}
	if s.DailyVolume == nil {
		s.DailyVolume = map[string]*big.Int{}
	}
	if s.HourlyVolume == nil {
		s.HourlyVolume = map[string]*big.Int{}
	}
	if s.LightClients == nil {
		s.LightClients = map[string]*BridgeLightClient{}
	}
	if s.ProofAuthorized == nil {
		s.ProofAuthorized = map[string]bool{}
	}
}

func bridgeAttestationMessage(a BridgeAttestation) string {
	return fmt.Sprintf("PODL-BRIDGE:%s:%s:%s:%s:%s:%d:%d", strings.ToLower(a.RequestID), strings.ToLower(a.SourceChainID), strings.ToLower(a.SourceTxHash), strings.ToLower(a.SourceBlockHash), a.EventIndex, a.Confirmations, a.ObservedAt)
}

func SignBridgeAttestation(a *BridgeAttestation, privateKeyHex string) error {
	if a == nil {
		return fmt.Errorf("nil attestation")
	}
	key, err := crypto.HexToECDSA(strings.TrimPrefix(privateKeyHex, "0x"))
	if err != nil {
		return err
	}
	sig, err := crypto.Sign(accounts.TextHash([]byte(bridgeAttestationMessage(*a))), key)
	if err != nil {
		return err
	}
	a.Signer = strings.ToLower(crypto.PubkeyToAddress(key.PublicKey).Hex())
	a.Signature = "0x" + hex.EncodeToString(sig)
	return nil
}

func VerifyBridgeAttestation(a BridgeAttestation) bool {
	raw, err := hex.DecodeString(strings.TrimPrefix(a.Signature, "0x"))
	if err != nil || len(raw) != 65 {
		return false
	}
	pub, err := crypto.SigToPub(accounts.TextHash([]byte(bridgeAttestationMessage(a))), raw)
	if err != nil {
		return false
	}
	return strings.EqualFold(crypto.PubkeyToAddress(*pub).Hex(), a.Signer)
}

func bridgeEventKey(a BridgeAttestation) string {
	sum := sha256.Sum256([]byte(strings.ToLower(a.SourceChainID) + "|" + strings.ToLower(a.SourceTxHash) + "|" + a.EventIndex))
	return hex.EncodeToString(sum[:])
}

func (bc *Blockchain_struct) SubmitBridgeAttestation(a BridgeAttestation, now int64) error {
	if bc == nil || !VerifyBridgeAttestation(a) {
		return fmt.Errorf("invalid bridge attestation signature")
	}
	if now <= 0 {
		now = time.Now().Unix()
	}
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	bc.EnsureRuntimeState()
	s := bc.BridgeSecurity
	s.ensure()
	a.Signer = strings.ToLower(a.Signer)
	if !s.Policy.AllowedSigners[a.Signer] {
		return fmt.Errorf("bridge signer is not allowlisted")
	}
	req := bc.BridgeRequests[strings.ToLower(a.RequestID)]
	if req == nil {
		return fmt.Errorf("bridge request not found")
	}
	if a.ObservedAt > now+30 || now-a.ObservedAt > s.Policy.AttestationTTL {
		return fmt.Errorf("bridge attestation is stale or future-dated")
	}
	if !strings.EqualFold(a.SourceChainID, req.SourceChainID) || (!strings.EqualFold(a.SourceTxHash, req.SourceTxHash) && !strings.EqualFold(a.SourceTxHash, req.LqdTxHash) && !strings.EqualFold(a.SourceTxHash, req.BscTxHash)) {
		return fmt.Errorf("attestation does not match request source")
	}
	if strings.TrimSpace(a.SourceBlockHash) == "" || strings.TrimSpace(a.EventIndex) == "" {
		return fmt.Errorf("source block hash and event index are required")
	}
	if s.Consumed[bridgeEventKey(a)] {
		return fmt.Errorf("source event already consumed")
	}
	if s.Attestations[req.ID] == nil {
		s.Attestations[req.ID] = map[string]BridgeAttestation{}
	}
	for _, existing := range s.Attestations[req.ID] {
		if existing.SourceBlockHash != a.SourceBlockHash || existing.EventIndex != a.EventIndex || existing.SourceTxHash != a.SourceTxHash {
			return fmt.Errorf("signer observations disagree")
		}
	}
	s.Attestations[req.ID][a.Signer] = a
	bc.persistRuntimeStateLocked()
	return nil
}

func bridgeChainRiskKey(req *BridgeRequest) string {
	if req == nil {
		return ""
	}
	key := strings.ToLower(strings.TrimSpace(req.SourceChainID))
	if key == "" {
		key = strings.ToLower(strings.TrimSpace(req.SourceChain))
	}
	return key
}

func (bc *Blockchain_struct) authorizeBridgeExecutionLocked(req *BridgeRequest, now int64) error {
	s := bc.BridgeSecurity
	s.ensure()
	if !s.Policy.EnforceAttestations && !s.Policy.RequireLightClientProof {
		return nil
	}
	if bc.ProtocolPauses["bridge"] {
		return fmt.Errorf("bridge is paused")
	}
	if s.Policy.RequireLightClientProof && !s.ProofAuthorized[req.ID] {
		return fmt.Errorf("bridge light-client event proof required")
	}
	if s.Authorized[req.ID] {
		return nil
	}
	if !s.Policy.EnforceAttestations {
		s.Authorized[req.ID] = true
		return nil
	}
	attestations := s.Attestations[req.ID]
	if len(attestations) < s.Policy.RequiredSigners {
		return fmt.Errorf("bridge threshold not reached: %d/%d", len(attestations), s.Policy.RequiredSigners)
	}
	chain := bridgeChainRiskKey(req)
	assetKey := chain + ":" + strings.ToLower(strings.TrimSpace(req.Token))
	amount, err := NewAmountFromString(req.Amount)
	if err != nil || amount.Sign() <= 0 {
		return fmt.Errorf("invalid bridge amount")
	}
	if capValue, ok := new(big.Int).SetString(s.Policy.PerTransactionCaps[chain], 10); ok && capValue.Sign() > 0 && amount.Cmp(capValue) > 0 {
		return fmt.Errorf("bridge per-transaction cap exceeded")
	}
	if capValue, ok := new(big.Int).SetString(s.Policy.AssetTransactionCaps[assetKey], 10); ok && capValue.Sign() > 0 && amount.Cmp(capValue) > 0 {
		return fmt.Errorf("bridge asset per-transaction cap exceeded")
	}
	dayKey := chain + ":" + time.Unix(now, 0).UTC().Format("2006-01-02")
	used := s.DailyVolume[dayKey]
	if used == nil {
		used = big.NewInt(0)
	}
	if capValue, ok := new(big.Int).SetString(s.Policy.DailyCaps[chain], 10); ok && capValue.Sign() > 0 && new(big.Int).Add(used, amount).Cmp(capValue) > 0 {
		return fmt.Errorf("bridge daily cap exceeded")
	}
	assetDayKey := assetKey + ":" + time.Unix(now, 0).UTC().Format("2006-01-02")
	assetUsed := s.DailyVolume[assetDayKey]
	if assetUsed == nil {
		assetUsed = big.NewInt(0)
	}
	if capValue, ok := new(big.Int).SetString(s.Policy.AssetDailyCaps[assetKey], 10); ok && capValue.Sign() > 0 && new(big.Int).Add(assetUsed, amount).Cmp(capValue) > 0 {
		return fmt.Errorf("bridge asset daily cap exceeded")
	}
	hourKey := chain + ":" + time.Unix(now, 0).UTC().Format("2006-01-02T15")
	hourUsed := s.HourlyVolume[hourKey]
	if hourUsed == nil {
		hourUsed = big.NewInt(0)
	}
	if capValue, ok := new(big.Int).SetString(s.Policy.HourlyCaps[chain], 10); ok && capValue.Sign() > 0 && new(big.Int).Add(hourUsed, amount).Cmp(capValue) > 0 {
		return fmt.Errorf("bridge hourly cap exceeded")
	}
	var first BridgeAttestation
	for _, a := range attestations {
		first = a
		break
	}
	minConfirmations := 1
	if reg, err := LoadBridgeChainRegistry(); err == nil && reg != nil {
		if cfg := reg.ChainByID(chain); cfg != nil && cfg.Confirmations > minConfirmations {
			minConfirmations = cfg.Confirmations
		}
	}
	if first.Confirmations < minConfirmations {
		return fmt.Errorf("source finality confirmations insufficient")
	}
	s.DailyVolume[dayKey] = new(big.Int).Add(used, amount)
	s.DailyVolume[assetDayKey] = new(big.Int).Add(assetUsed, amount)
	s.HourlyVolume[hourKey] = new(big.Int).Add(hourUsed, amount)
	s.Authorized[req.ID] = true
	s.Consumed[bridgeEventKey(first)] = true
	return nil
}

func (bc *Blockchain_struct) BridgeExecutionAuthorized(requestID string, now int64) error {
	if bc == nil {
		return fmt.Errorf("nil blockchain")
	}
	if now <= 0 {
		now = time.Now().Unix()
	}
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	bc.EnsureRuntimeState()
	req := bc.BridgeRequests[strings.ToLower(requestID)]
	if req == nil {
		return fmt.Errorf("bridge request not found")
	}
	if err := bc.authorizeBridgeExecutionLocked(req, now); err != nil {
		return err
	}
	bc.persistRuntimeStateLocked()
	return nil
}

func (bc *Blockchain_struct) BridgeSecurityStatus() map[string]interface{} {
	if bc == nil {
		return map[string]interface{}{}
	}
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	bc.EnsureRuntimeState()
	s := bc.BridgeSecurity
	s.ensure()
	signers := []string{}
	for signer := range s.Policy.AllowedSigners {
		signers = append(signers, signer)
	}
	sort.Strings(signers)
	return map[string]interface{}{"policy": s.Policy, "allowed_signers": signers, "attested_requests": len(s.Attestations), "authorized_requests": len(s.Authorized), "consumed_events": len(s.Consumed), "light_clients": len(s.LightClients), "proof_authorized_requests": len(s.ProofAuthorized), "mainnet_threshold_ready": s.Policy.EnforceAttestations && s.Policy.RequiredSigners >= 3 && len(signers) >= s.Policy.RequiredSigners, "light_client_mode_ready": s.Policy.RequireLightClientProof && len(s.LightClients) > 0}
}
