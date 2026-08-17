package blockchaincomponent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// ConsensusVRFProof is an RFC 9381 ECVRF-P256-SHA256-TAI randomness
// contribution. The validator's secp256k1 identity key signs a binding to the
// P-256 VRF public key, while the ECVRF proof supplies uniqueness and
// pseudorandomness for the parent-bound consensus seed.
type ConsensusVRFProof struct {
	Height     uint64 `json:"height"`
	Round      uint32 `json:"round"`
	SpecHash   string `json:"spec_hash"`
	Seed       string `json:"seed"`
	Validator  string `json:"validator"`
	Suite      string `json:"suite"`
	PublicKey  string `json:"public_key"`
	Proof      string `json:"proof"`
	Output     string `json:"output"`
	KeyBinding string `json:"key_binding"`
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

// ConsensusBlockVRFSeed binds a next-height VRF proof to the executed block
// without introducing a circular dependency on that block's final hash.
func ConsensusBlockVRFSeed(specHash string, blockHeight uint64, previousHash, stateRoot string, targetHeight uint64) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("PODL-BLOCK-VRF-SEED-V1:%s:%d:%s:%s:%d", strings.ToLower(specHash), blockHeight, strings.ToLower(previousHash), strings.ToLower(stateRoot), targetHeight)))
	return "0x" + hex.EncodeToString(sum[:])
}

func consensusVRFMessage(proof ConsensusVRFProof) string {
	return fmt.Sprintf("PODL-RFC9381-VRF-ALPHA-V2:%s:%s:%d:%d", strings.ToLower(proof.SpecHash), strings.ToLower(proof.Seed), proof.Height, proof.Round)
}

func consensusVRFOutput(proofHex string) string {
	proof, err := decodeFixedHex(proofHex, ecvrfP256ProofLen)
	if err != nil {
		return ""
	}
	output, err := ECVRFP256ProofToHash(proof)
	if err != nil {
		return ""
	}
	return "0x" + hex.EncodeToString(output)
}

func SignConsensusVRFProof(proof *ConsensusVRFProof, privateKeyHex string) error {
	signer, err := NewLocalValidatorSigner(privateKeyHex, "", "")
	if err != nil {
		return err
	}
	defer signer.Close()
	return SignConsensusVRFProofWithSigner(context.Background(), proof, signer)
}

func SignConsensusVRFProofWithSigner(ctx context.Context, proof *ConsensusVRFProof, signer ValidatorSigner) error {
	if proof == nil || proof.Height == 0 || strings.TrimSpace(proof.SpecHash) == "" || strings.TrimSpace(proof.Seed) == "" {
		return fmt.Errorf("complete VRF request required")
	}
	if signer == nil || !ValidateAddress(signer.Address()) {
		return fmt.Errorf("validator signer is required")
	}
	result, err := signer.ProveVRF(ctx, []byte(consensusVRFMessage(*proof)), consensusVRFKey(proof.Height, proof.Round))
	if err != nil {
		return err
	}
	proof.Validator = strings.ToLower(signer.Address())
	proof.Suite = result.Suite
	proof.PublicKey = result.PublicKey
	proof.Proof = result.Proof
	proof.Output = result.Output
	proof.KeyBinding = result.KeyBinding
	return nil
}

func VerifyConsensusVRFProof(proof ConsensusVRFProof) bool {
	if proof.Height == 0 || proof.Round > 1_000_000 || !ValidateAddress(proof.Validator) || strings.TrimSpace(proof.SpecHash) == "" || strings.TrimSpace(proof.Seed) == "" {
		return false
	}
	result := ValidatorVRFResult{Suite: proof.Suite, PublicKey: proof.PublicKey, Proof: proof.Proof, Output: proof.Output, KeyBinding: proof.KeyBinding}
	return VerifyValidatorVRFResult(proof.Validator, []byte(consensusVRFMessage(proof)), result)
}

// AttachNextProposerVRF adds the elected proposer's RFC 9381 proof for the
// following height. Unlike an arrival-order-dependent off-chain beacon, this
// proof becomes part of the canonical block hash and is therefore identical
// on every node before it can influence proposer selection.
func (bc *Blockchain_struct) AttachNextProposerVRF(block *Block) error {
	if bc == nil || block == nil || block.BlockNumber == 0 || strings.TrimSpace(block.StateRoot) == "" || bc.Network == nil {
		return fmt.Errorf("complete executed block and validator network required")
	}
	address, signer := bc.Network.ValidatorSignerSnapshot()
	if signer == nil || !strings.EqualFold(address, block.RewardBreakdown.Validator) {
		return fmt.Errorf("elected proposer signer is unavailable")
	}
	proof := &ConsensusVRFProof{
		Height:   block.BlockNumber + 1,
		Round:    0,
		SpecHash: bc.ChainSpec.Hash(),
		Seed:     ConsensusBlockVRFSeed(bc.ChainSpec.Hash(), block.BlockNumber, block.PreviousHash, block.StateRoot, block.BlockNumber+1),
	}
	if err := SignConsensusVRFProofWithSigner(context.Background(), proof, signer); err != nil {
		return err
	}
	if !strings.EqualFold(proof.Validator, block.RewardBreakdown.Validator) {
		return fmt.Errorf("next-height VRF proof is not signed by the elected proposer")
	}
	block.NextVRFProof = proof
	return nil
}

func (bc *Blockchain_struct) VerifyBlockVRFContribution(block *Block) bool {
	if bc == nil || block == nil || block.NextVRFProof == nil {
		return false
	}
	proof := *block.NextVRFProof
	expectedSeed := ConsensusBlockVRFSeed(bc.ChainSpec.Hash(), block.BlockNumber, block.PreviousHash, block.StateRoot, block.BlockNumber+1)
	return proof.Height == block.BlockNumber+1 && proof.Round == 0 &&
		proof.SpecHash == bc.ChainSpec.Hash() && proof.Seed == expectedSeed &&
		strings.EqualFold(proof.Validator, block.RewardBreakdown.Validator) && VerifyConsensusVRFProof(proof)
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

func (bc *Blockchain_struct) canonicalBlockVRFEntropy(height uint64) string {
	if bc == nil || len(bc.Blocks) == 0 {
		return ""
	}
	parent := bc.Blocks[len(bc.Blocks)-1]
	if parent == nil || parent.BlockNumber+1 != height || parent.NextVRFProof == nil || !bc.VerifyBlockVRFContribution(parent) {
		return ""
	}
	return parent.NextVRFProof.Output
}
