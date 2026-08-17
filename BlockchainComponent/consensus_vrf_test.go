package blockchaincomponent

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestThresholdVRFBeaconAndProposerBinding(t *testing.T) {
	bc, keys := consensusFixture(t, 4)
	seed := ConsensusVRFSeed(bc.ChainSpec.Hash(), bc.proposerParentEntropy(2), 2, 0)
	var beacon *ConsensusVRFBeacon
	for i := 0; i < 3; i++ {
		proof := ConsensusVRFProof{Height: 2, Round: 0, SpecHash: bc.ChainSpec.Hash(), Seed: seed}
		if err := SignConsensusVRFProof(&proof, keys[i]); err != nil {
			t.Fatal(err)
		}
		var err error
		beacon, err = bc.AddConsensusVRFProof(proof)
		if err != nil {
			t.Fatal(err)
		}
		if i < 2 && beacon != nil {
			t.Fatal("VRF beacon formed without strict quorum")
		}
	}
	if beacon == nil || beacon.Output == "" || len(beacon.Validators) != 3 {
		t.Fatalf("threshold candidate not formed: %#v", beacon)
	}
	if beacon.Finalized || bc.ConsensusV2.LastVRFBeacon != nil {
		t.Fatal("arrival-order-dependent quorum subset became proposer entropy")
	}
	fallback, err := bc.BuildProposerCertificate(2, 0)
	if err != nil || fallback.Entropy == beacon.Output {
		t.Fatalf("non-final candidate influenced proposer selection: cert=%#v err=%v", fallback, err)
	}
	proof := ConsensusVRFProof{Height: 2, Round: 0, SpecHash: bc.ChainSpec.Hash(), Seed: seed}
	if err := SignConsensusVRFProof(&proof, keys[3]); err != nil {
		t.Fatal(err)
	}
	beacon, err = bc.AddConsensusVRFProof(proof)
	if err != nil {
		t.Fatal(err)
	}
	if beacon == nil || !beacon.Finalized || len(beacon.Validators) != 4 {
		t.Fatalf("all-contributor beacon not finalized: %#v", beacon)
	}
	cert, err := bc.BuildProposerCertificate(2, 0)
	if err != nil || cert.Entropy == beacon.Output || !bc.VerifyProposerCertificate(*cert) {
		t.Fatalf("off-chain beacon influenced canonical proposer selection: cert=%#v err=%v", cert, err)
	}
}

func TestCanonicalBlockVRFDrivesNextProposerEntropy(t *testing.T) {
	bc, keys := consensusFixture(t, 4)
	bc.Network = NewNetworkService(bc)
	proposer, err := bc.SelectBlockProposer(1, 0)
	if err != nil {
		t.Fatal(err)
	}
	var proposerKey string
	for _, key := range keys {
		proof := ConsensusVRFProof{Height: 9, SpecHash: bc.ChainSpec.Hash(), Seed: "lookup"}
		if err := SignConsensusVRFProof(&proof, key); err == nil && strings.EqualFold(proof.Validator, proposer.Address) {
			proposerKey = key
			break
		}
	}
	if proposerKey == "" {
		t.Fatal("fixture proposer key not found")
	}
	signer, err := NewLocalValidatorSigner(proposerKey, testP256VRFSecret, filepath.Join(t.TempDir(), "slashing.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer signer.Close()
	if err := bc.Network.SetValidatorSigner(proposer.Address, signer); err != nil {
		t.Fatal(err)
	}
	parent := bc.Blocks[len(bc.Blocks)-1]
	block := &Block{BlockNumber: 1, PreviousHash: parent.CurrentHash, StateRoot: "0xstate-root", RewardBreakdown: BlockRewardBreakdown{Validator: proposer.Address}}
	if err := bc.AttachNextProposerVRF(block); err != nil {
		t.Fatal(err)
	}
	if !bc.VerifyBlockVRFContribution(block) {
		t.Fatal("canonical block VRF contribution did not verify")
	}
	block.CurrentHash = CalculateHash(block)
	bc.Blocks = append(bc.Blocks, block)
	cert, err := bc.BuildProposerCertificate(2, 0)
	if err != nil {
		t.Fatal(err)
	}
	if cert.Entropy != block.NextVRFProof.Output || !bc.VerifyProposerCertificate(*cert) {
		t.Fatal("next proposer certificate did not consume canonical RFC 9381 output")
	}
	tampered := *block.NextVRFProof
	tampered.Output = "0xdead"
	block.NextVRFProof = &tampered
	if bc.VerifyBlockVRFContribution(block) || bc.canonicalBlockVRFEntropy(2) != "" {
		t.Fatal("tampered canonical VRF output was accepted")
	}
}

func TestVRFProofRejectsTamperWrongSeedAndConflict(t *testing.T) {
	bc, keys := consensusFixture(t, 4)
	seed := ConsensusVRFSeed(bc.ChainSpec.Hash(), bc.proposerParentEntropy(2), 2, 0)
	proof := ConsensusVRFProof{Height: 2, SpecHash: bc.ChainSpec.Hash(), Seed: seed}
	if err := SignConsensusVRFProof(&proof, keys[0]); err != nil {
		t.Fatal(err)
	}
	tampered := proof
	tampered.Output = "0xdead"
	if VerifyConsensusVRFProof(tampered) {
		t.Fatal("tampered VRF output verified")
	}
	wrong := proof
	wrong.Seed = "0xwrong"
	if _, err := bc.AddConsensusVRFProof(wrong); err == nil {
		t.Fatal("wrong-seed proof accepted")
	}
	if _, err := bc.AddConsensusVRFProof(proof); err != nil {
		t.Fatal(err)
	}
	conflict := ConsensusVRFProof{Height: 2, Round: 0, SpecHash: bc.ChainSpec.Hash(), Seed: seed}
	if err := SignConsensusVRFProof(&conflict, keys[0]); err != nil {
		t.Fatal(err)
	}
	conflict.Proof = "0x" + conflict.Proof[4:6] + conflict.Proof[2:4] + conflict.Proof[6:]
	conflict.Output = consensusVRFOutput(conflict.Proof)
	if _, err := bc.AddConsensusVRFProof(conflict); err == nil {
		t.Fatal("conflicting invalid VRF proof accepted")
	}
}
