package blockchaincomponent

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

func TestBridgeLightClientHeaderQuorumMerkleProofAndReplay(t *testing.T) {
	previousPersist := persistRuntimeState
	persistRuntimeState = func(*Blockchain_struct) error { return nil }
	defer func() { persistRuntimeState = previousPersist }()
	bc := &Blockchain_struct{}
	bc.EnsureRuntimeState()
	keys, validators := []string{}, []BridgeLightValidator{}
	for i := byte(1); i <= 4; i++ {
		seed := make([]byte, 32)
		seed[31] = i
		key, err := crypto.ToECDSA(seed)
		if err != nil {
			t.Fatal(err)
		}
		keys = append(keys, hex.EncodeToString(seed))
		validators = append(validators, BridgeLightValidator{Address: crypto.PubkeyToAddress(key.PublicKey).Hex(), Power: 1})
	}
	chain := "source-test"
	trusted := "0x" + hex.EncodeToString(make([]byte, 32))
	if err := configureBridgeLightClient(bc.BridgeSecurity, BridgeLightClientConfig{ChainID: chain, TrustedHeight: 10, TrustedHash: trusted, Validators: validators}); err != nil {
		t.Fatal(err)
	}
	request := &BridgeRequest{ID: "request-1", SourceChainID: chain, SourceTxHash: "0xsource", Token: "lqd", Amount: "100", To: "0x1111111111111111111111111111111111111111"}
	bc.BridgeRequests = map[string]*BridgeRequest{request.ID: request}
	leaf := BridgeEventLeafHash(request, request.SourceTxHash, "0")
	siblingDigest := sha256.Sum256([]byte("sibling"))
	rootDigest := sha256.Sum256(append(append([]byte(nil), leaf...), siblingDigest[:]...))
	header := BridgeLightHeader{ChainID: chain, Height: 11, ParentHash: trusted, EventRoot: "0x" + hex.EncodeToString(rootDigest[:]), ValidatorSetHash: BridgeLightValidatorSetHash(validators), NextValidatorSetHash: BridgeLightValidatorSetHash(validators)}
	header.Hash = BridgeLightHeaderHash(header)
	for i := 0; i < 3; i++ {
		if err := SignBridgeLightHeader(&header, keys[i]); err != nil {
			t.Fatal(err)
		}
	}
	if err := bc.SubmitBridgeLightHeader(header); err != nil {
		t.Fatal(err)
	}
	proof := BridgeEventMerkleProof{RequestID: request.ID, ChainID: chain, HeaderHeight: 11, SourceTxHash: request.SourceTxHash, EventIndex: "0", Siblings: []string{"0x" + hex.EncodeToString(siblingDigest[:])}, SiblingOnLeft: []bool{false}}
	if err := bc.SubmitBridgeEventProof(proof); err != nil {
		t.Fatal(err)
	}
	if !bc.BridgeSecurity.ProofAuthorized[request.ID] {
		t.Fatal("valid proof did not authorize request")
	}
	if err := bc.SubmitBridgeEventProof(proof); err == nil {
		t.Fatal("light-client event proof replay accepted")
	}
	tampered := header
	tampered.Height = 12
	tampered.ParentHash = header.Hash
	tampered.EventRoot = "0x" + hex.EncodeToString(make([]byte, 32))
	tampered.Hash = BridgeLightHeaderHash(tampered)
	tampered.Signatures = nil
	for i := 0; i < 2; i++ {
		_ = SignBridgeLightHeader(&tampered, keys[i])
	}
	if err := bc.SubmitBridgeLightHeader(tampered); err == nil {
		t.Fatal("sub-quorum light header accepted")
	}
}
