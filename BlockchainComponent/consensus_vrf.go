package blockchaincomponent

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/crypto"
)

// ConsensusVRFProof is a publicly verifiable, deterministic signature-based
// randomness contribution. The secp256k1 proof is recoverable to the validator
// address, while Output is domain-separated from the proof itself. A single
// contributor cannot choose the final beacon. A >2/3 contribution produces a
// verifiable candidate for observability, but proposer election only consumes a
// beacon after every active validator has contributed. If a validator withholds,
// consensus stays live by falling back to the prior QC entropy.
type ConsensusVRFProof struct {
	Height    uint64 `json:"height"`
	Round     uint32 `json:"round"`
	SpecHash  string `json:"spec_hash"`
	Seed      string `json:"seed"`
	Validator string `json:"validator"`
	Proof     string `json:"proof"`
	Output    string `json:"output"`
}

type ConsensusVRFBeacon struct {
	Height        uint64   `json:"height"`
	Round         uint32   `json:"round"`
	Seed          string   `json:"seed"`
	Output        string   `json:"output"`
	Validators    []string `json:"validators"`
	VotingPower   float64  `json:"voting_power"`
	RequiredPower float64  `json:"required_power"`
	Finalized     bool     `json:"finalized"`
}

func consensusVRFKey(height uint64, round uint32) string {
	return fmt.Sprintf("%d/%d", height, round)
}

func ConsensusVRFSeed(specHash, parentEntropy string, height uint64, round uint32) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("PODL-VRF-SEED-V1:%s:%s:%d:%d", strings.ToLower(specHash), strings.ToLower(parentEntropy), height, round)))
	return "0x" + hex.EncodeToString(sum[:])
}

func consensusVRFMessage(proof ConsensusVRFProof) string {
	return fmt.Sprintf("PODL-VRF-PROOF-V1:%s:%s:%d:%d", strings.ToLower(proof.SpecHash), strings.ToLower(proof.Seed), proof.Height, proof.Round)
}

func consensusVRFOutput(signature string) string {
	sum := sha256.Sum256([]byte("PODL-VRF-OUTPUT-V1:" + strings.ToLower(signature)))
	return "0x" + hex.EncodeToString(sum[:])
}

func SignConsensusVRFProof(proof *ConsensusVRFProof, privateKeyHex string) error {
	if proof == nil || proof.Height == 0 || strings.TrimSpace(proof.SpecHash) == "" || strings.TrimSpace(proof.Seed) == "" {
		return fmt.Errorf("complete VRF request required")
	}
	key, err := crypto.HexToECDSA(strings.TrimPrefix(strings.TrimSpace(privateKeyHex), "0x"))
	if err != nil {
		return err
	}
	sig, err := crypto.Sign(accounts.TextHash([]byte(consensusVRFMessage(*proof))), key)
	if err != nil {
		return err
	}
	proof.Validator = strings.ToLower(crypto.PubkeyToAddress(key.PublicKey).Hex())
	proof.Proof = "0x" + hex.EncodeToString(sig)
	proof.Output = consensusVRFOutput(proof.Proof)
	return nil
}

func VerifyConsensusVRFProof(proof ConsensusVRFProof) bool {
	if proof.Height == 0 || proof.Round > 1_000_000 || !ValidateAddress(proof.Validator) || proof.Output != consensusVRFOutput(proof.Proof) {
		return false
	}
	raw, err := hex.DecodeString(strings.TrimPrefix(strings.TrimSpace(proof.Proof), "0x"))
	if err != nil || len(raw) != 65 {
		return false
	}
	pub, err := crypto.SigToPub(accounts.TextHash([]byte(consensusVRFMessage(proof))), raw)
	return err == nil && strings.EqualFold(crypto.PubkeyToAddress(*pub).Hex(), proof.Validator)
}

// AddConsensusVRFProof accepts one proof per active validator. A strict weighted
// quorum forms a candidate, while only the deterministic all-contributor set is
// finalized for proposer selection. This avoids subset/arrival-order bias. A
// withheld contribution never stalls consensus because proposerEntropy falls
// back to the prior finalized QC. Duplicate proofs are idempotent; conflicting
// proofs from one validator are rejected.
func (bc *Blockchain_struct) AddConsensusVRFProof(proof ConsensusVRFProof) (*ConsensusVRFBeacon, error) {
	if bc == nil || !VerifyConsensusVRFProof(proof) {
		return nil, fmt.Errorf("invalid VRF proof")
	}
	bc.EnsureRuntimeState()
	expectedSeed := ConsensusVRFSeed(bc.ChainSpec.Hash(), bc.proposerParentEntropy(proof.Height), proof.Height, proof.Round)
	if proof.SpecHash != bc.ChainSpec.Hash() || proof.Seed != expectedSeed {
		return nil, fmt.Errorf("VRF proof is bound to the wrong chain or parent entropy")
	}
	s := bc.ConsensusV2
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensure()
	if len(s.ActiveSet) == 0 {
		s.ActiveSet = bc.buildValidatorPowerSet()
	}
	if validatorPowerFor(s.ActiveSet, proof.Validator) <= 0 {
		return nil, fmt.Errorf("VRF contributor is not in the active set")
	}
	key, address := consensusVRFKey(proof.Height, proof.Round), strings.ToLower(proof.Validator)
	if s.VRFProofs[key] == nil {
		s.VRFProofs[key] = make(map[string]ConsensusVRFProof)
	}
	if prior, exists := s.VRFProofs[key][address]; exists {
		if prior.Proof == proof.Proof {
			if s.LastVRFBeacon != nil && s.LastVRFBeacon.Height == proof.Height && s.LastVRFBeacon.Round == proof.Round {
				return s.LastVRFBeacon, nil
			}
			return nil, nil
		}
		return nil, fmt.Errorf("validator submitted conflicting VRF proofs")
	}
	s.VRFProofs[key][address] = proof
	total, power := validatorSetPower(s.ActiveSet), 0.0
	validators := make([]string, 0, len(s.VRFProofs[key]))
	for signer := range s.VRFProofs[key] {
		p := validatorPowerFor(s.ActiveSet, signer)
		if p > 0 {
			power += p
			validators = append(validators, signer)
		}
	}
	required := requiredQuorum(total)
	if power < required {
		return nil, nil
	}
	sort.Strings(validators)
	parts := make([]string, 0, len(validators))
	for _, signer := range validators {
		parts = append(parts, signer+":"+s.VRFProofs[key][signer].Output)
	}
	sum := sha256.Sum256([]byte("PODL-VRF-BEACON-V1:" + proof.Seed + ":" + strings.Join(parts, "|")))
	beacon := &ConsensusVRFBeacon{Height: proof.Height, Round: proof.Round, Seed: proof.Seed, Output: "0x" + hex.EncodeToString(sum[:]), Validators: validators, VotingPower: power, RequiredPower: required, Finalized: power+1e-9 >= total}
	if beacon.Finalized {
		s.LastVRFBeacon = beacon
	}
	return beacon, nil
}

// proposerParentEntropy deliberately ignores an already-created beacon for
// this height; contributors must all sign the same parent-derived seed.
func (bc *Blockchain_struct) proposerParentEntropy(height uint64) string {
	if bc != nil && bc.ConsensusV2 != nil && bc.ConsensusV2.LastQC != nil && bc.ConsensusV2.LastQC.Height+1 == height && bc.ConsensusV2.LastQC.Randomness != "" {
		return bc.ConsensusV2.LastQC.Randomness
	}
	if bc != nil {
		return bc.ChainSpec.Hash()
	}
	return ""
}
