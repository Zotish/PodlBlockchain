package blockchaincomponent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/crypto"
)

// BridgeLightClient verifies a sequential BFT header chain and Merkle event
// inclusion. An adapter for a source chain must map its finalized header/event
// commitment into this deterministic format; no relayer can authorize an
// event merely by supplying metadata.
type BridgeLightValidator struct {
	Address string `json:"address"`
	Power   uint64 `json:"power"`
}
type BridgeLightSignature struct {
	Signer    string `json:"signer"`
	Signature string `json:"signature"`
}
type BridgeLightHeader struct {
	ChainID              string                 `json:"chain_id"`
	Height               uint64                 `json:"height"`
	ParentHash           string                 `json:"parent_hash"`
	Hash                 string                 `json:"hash"`
	EventRoot            string                 `json:"event_root"`
	ValidatorSetHash     string                 `json:"validator_set_hash"`
	NextValidatorSetHash string                 `json:"next_validator_set_hash"`
	NextValidators       []BridgeLightValidator `json:"next_validators,omitempty"`
	Signatures           []BridgeLightSignature `json:"signatures,omitempty"`
}
type BridgeLightClient struct {
	ChainID    string                       `json:"chain_id"`
	Height     uint64                       `json:"height"`
	HeaderHash string                       `json:"header_hash"`
	Validators []BridgeLightValidator       `json:"validators"`
	Headers    map[uint64]BridgeLightHeader `json:"headers"`
}
type BridgeLightClientConfig struct {
	ChainID       string                 `json:"chain_id"`
	TrustedHeight uint64                 `json:"trusted_height"`
	TrustedHash   string                 `json:"trusted_hash"`
	Validators    []BridgeLightValidator `json:"validators"`
}
type BridgeEventMerkleProof struct {
	RequestID     string   `json:"request_id"`
	ChainID       string   `json:"chain_id"`
	HeaderHeight  uint64   `json:"header_height"`
	SourceTxHash  string   `json:"source_tx_hash"`
	EventIndex    string   `json:"event_index"`
	Siblings      []string `json:"siblings"`
	SiblingOnLeft []bool   `json:"sibling_on_left"`
}

func bridgeLightValidators(validators []BridgeLightValidator) ([]BridgeLightValidator, error) {
	out := append([]BridgeLightValidator(nil), validators...)
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Address) < strings.ToLower(out[j].Address) })
	total, seen := uint64(0), map[string]bool{}
	for i := range out {
		out[i].Address = strings.ToLower(strings.TrimSpace(out[i].Address))
		if !ValidateAddress(out[i].Address) || out[i].Power == 0 || seen[out[i].Address] || math.MaxUint64-total < out[i].Power {
			return nil, fmt.Errorf("invalid light-client validator set")
		}
		seen[out[i].Address] = true
		total += out[i].Power
	}
	if len(out) < 4 || total == 0 {
		return nil, fmt.Errorf("light-client validator set requires at least four powered validators")
	}
	return out, nil
}
func BridgeLightValidatorSetHash(validators []BridgeLightValidator) string {
	normalized, err := bridgeLightValidators(validators)
	if err != nil {
		return ""
	}
	raw, _ := json.Marshal(normalized)
	digest := sha256.Sum256(append([]byte("PODL-BRIDGE-VALIDATORS-V1:"), raw...))
	return "0x" + hex.EncodeToString(digest[:])
}
func bridgeLightHeaderDigest(header BridgeLightHeader) []byte {
	material := strings.ToLower(strings.TrimSpace(header.ChainID)) + "|" + strconv.FormatUint(header.Height, 10) + "|" + strings.ToLower(header.ParentHash) + "|" + strings.ToLower(header.EventRoot) + "|" + strings.ToLower(header.ValidatorSetHash) + "|" + strings.ToLower(header.NextValidatorSetHash)
	digest := sha256.Sum256([]byte("PODL-BRIDGE-HEADER-V1:" + material))
	return digest[:]
}
func BridgeLightHeaderHash(header BridgeLightHeader) string {
	return "0x" + hex.EncodeToString(bridgeLightHeaderDigest(header))
}
func SignBridgeLightHeader(header *BridgeLightHeader, privateKeyHex string) error {
	if header == nil {
		return fmt.Errorf("nil light header")
	}
	key, err := crypto.HexToECDSA(strings.TrimPrefix(privateKeyHex, "0x"))
	if err != nil {
		return err
	}
	signature, err := crypto.Sign(accounts.TextHash(bridgeLightHeaderDigest(*header)), key)
	if err != nil {
		return err
	}
	header.Signatures = append(header.Signatures, BridgeLightSignature{Signer: strings.ToLower(crypto.PubkeyToAddress(key.PublicKey).Hex()), Signature: "0x" + hex.EncodeToString(signature)})
	return nil
}
func configureBridgeLightClient(state *BridgeSecurityState, config BridgeLightClientConfig) error {
	config.ChainID = strings.ToLower(strings.TrimSpace(config.ChainID))
	config.TrustedHash = strings.ToLower(strings.TrimSpace(config.TrustedHash))
	validators, err := bridgeLightValidators(config.Validators)
	if err != nil || config.ChainID == "" || config.TrustedHeight == 0 || len(config.TrustedHash) != 66 {
		return fmt.Errorf("invalid trusted light-client checkpoint")
	}
	state.ensure()
	state.LightClients[config.ChainID] = &BridgeLightClient{ChainID: config.ChainID, Height: config.TrustedHeight, HeaderHash: config.TrustedHash, Validators: validators, Headers: map[uint64]BridgeLightHeader{}}
	return nil
}
func (bc *Blockchain_struct) SubmitBridgeLightHeader(header BridgeLightHeader) error {
	if bc == nil {
		return fmt.Errorf("nil chain")
	}
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	bc.EnsureRuntimeState()
	chain := strings.ToLower(strings.TrimSpace(header.ChainID))
	client := bc.BridgeSecurity.LightClients[chain]
	if client == nil || header.Height != client.Height+1 || !strings.EqualFold(header.ParentHash, client.HeaderHash) || !strings.EqualFold(header.Hash, BridgeLightHeaderHash(header)) || !strings.EqualFold(header.ValidatorSetHash, BridgeLightValidatorSetHash(client.Validators)) || len(strings.TrimPrefix(header.EventRoot, "0x")) != 64 {
		return fmt.Errorf("light-client header does not extend trusted state")
	}
	byAddress, total := map[string]uint64{}, uint64(0)
	for _, validator := range client.Validators {
		byAddress[validator.Address] = validator.Power
		total += validator.Power
	}
	signed, seen := uint64(0), map[string]bool{}
	digest := accounts.TextHash(bridgeLightHeaderDigest(header))
	for _, row := range header.Signatures {
		signer := strings.ToLower(strings.TrimSpace(row.Signer))
		if seen[signer] || byAddress[signer] == 0 {
			continue
		}
		raw, err := hex.DecodeString(strings.TrimPrefix(row.Signature, "0x"))
		if err != nil || len(raw) != 65 {
			continue
		}
		publicKey, err := crypto.SigToPub(digest, raw)
		if err != nil || !strings.EqualFold(crypto.PubkeyToAddress(*publicKey).Hex(), signer) {
			continue
		}
		seen[signer] = true
		signed += byAddress[signer]
	}
	if signed <= total*2/3 {
		return fmt.Errorf("light-client header lacks strict two-thirds signed power")
	}
	next := client.Validators
	if len(header.NextValidators) > 0 {
		normalized, err := bridgeLightValidators(header.NextValidators)
		if err != nil || !strings.EqualFold(header.NextValidatorSetHash, BridgeLightValidatorSetHash(normalized)) {
			return fmt.Errorf("invalid committed next validator set")
		}
		next = normalized
	} else if !strings.EqualFold(header.NextValidatorSetHash, header.ValidatorSetHash) {
		return fmt.Errorf("next validator hash changed without validator set")
	}
	header.ChainID, header.Hash = chain, strings.ToLower(header.Hash)
	header.Signatures = nil
	client.Headers[header.Height] = header
	if len(client.Headers) > 2048 {
		delete(client.Headers, header.Height-2048)
	}
	client.Height, client.HeaderHash, client.Validators = header.Height, header.Hash, next
	bc.persistRuntimeStateLocked()
	return nil
}
func BridgeEventLeafHash(request *BridgeRequest, sourceTxHash, eventIndex string) []byte {
	chain := bridgeChainRiskKey(request)
	material := strings.ToLower(request.ID) + "|" + chain + "|" + strings.ToLower(strings.TrimSpace(sourceTxHash)) + "|" + strings.TrimSpace(eventIndex) + "|" + strings.ToLower(strings.TrimSpace(request.Token)) + "|" + request.Amount + "|" + strings.ToLower(strings.TrimSpace(request.To))
	digest := sha256.Sum256([]byte("PODL-BRIDGE-EVENT-V1:" + material))
	return digest[:]
}
func verifyBridgeMerkleProof(leaf []byte, siblings []string, left []bool, root string) bool {
	if len(leaf) != 32 || len(siblings) != len(left) || len(siblings) > 64 {
		return false
	}
	current := append([]byte(nil), leaf...)
	for i, siblingHex := range siblings {
		sibling, err := hex.DecodeString(strings.TrimPrefix(siblingHex, "0x"))
		if err != nil || len(sibling) != 32 {
			return false
		}
		payload := append([]byte(nil), current...)
		if left[i] {
			payload = append(append([]byte(nil), sibling...), current...)
		} else {
			payload = append(payload, sibling...)
		}
		digest := sha256.Sum256(payload)
		current = digest[:]
	}
	return strings.EqualFold("0x"+hex.EncodeToString(current), root)
}
func (bc *Blockchain_struct) SubmitBridgeEventProof(proof BridgeEventMerkleProof) error {
	if bc == nil {
		return fmt.Errorf("nil chain")
	}
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	bc.EnsureRuntimeState()
	request := bc.BridgeRequests[strings.ToLower(strings.TrimSpace(proof.RequestID))]
	if request == nil {
		request = bc.BridgeRequests[proof.RequestID]
	}
	if request == nil {
		return fmt.Errorf("bridge request not found")
	}
	chain := strings.ToLower(strings.TrimSpace(proof.ChainID))
	if chain != bridgeChainRiskKey(request) {
		return fmt.Errorf("event proof source chain mismatch")
	}
	client := bc.BridgeSecurity.LightClients[chain]
	if client == nil {
		return fmt.Errorf("source light client not configured")
	}
	header, ok := client.Headers[proof.HeaderHeight]
	if !ok {
		return fmt.Errorf("event header is not retained and finalized")
	}
	expectedTx := request.SourceTxHash
	if expectedTx == "" {
		expectedTx = request.LqdTxHash
	}
	if expectedTx == "" {
		expectedTx = request.BscTxHash
	}
	if !strings.EqualFold(expectedTx, proof.SourceTxHash) || strings.TrimSpace(proof.EventIndex) == "" {
		return fmt.Errorf("event proof does not match request")
	}
	if !verifyBridgeMerkleProof(BridgeEventLeafHash(request, proof.SourceTxHash, proof.EventIndex), proof.Siblings, proof.SiblingOnLeft, header.EventRoot) {
		return fmt.Errorf("invalid bridge event inclusion proof")
	}
	proofKey := strings.ToLower(chain + "|" + proof.SourceTxHash + "|" + proof.EventIndex)
	digest := sha256.Sum256([]byte(proofKey))
	consumed := "light:" + hex.EncodeToString(digest[:])
	if bc.BridgeSecurity.Consumed[consumed] {
		return fmt.Errorf("light-client event proof replay")
	}
	bc.BridgeSecurity.Consumed[consumed] = true
	bc.BridgeSecurity.ProofAuthorized[request.ID] = true
	bc.persistRuntimeStateLocked()
	return nil
}
