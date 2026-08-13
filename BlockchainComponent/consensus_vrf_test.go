package blockchaincomponent

import "testing"

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
	if err != nil || cert.Entropy != beacon.Output || !bc.VerifyProposerCertificate(*cert) {
		t.Fatalf("proposer certificate did not bind finalized beacon: cert=%#v err=%v", cert, err)
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
