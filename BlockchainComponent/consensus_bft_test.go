package blockchaincomponent

import (
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
)

func makeSignedVote(t *testing.T, keyHex string, height uint64, round uint32, step ConsensusStep, hash string) ConsensusVote {
	t.Helper()
	v := ConsensusVote{Height: height, Round: round, Step: step, BlockHash: hash}
	if err := SignConsensusVote(&v, keyHex); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestBFTQuorumCertificateAndEquivocation(t *testing.T) {
	bc := newTestBlockchain()
	keys := make([]string, 4)
	for i := range keys {
		key, err := crypto.GenerateKey()
		if err != nil {
			t.Fatal(err)
		}
		keys[i] = hex.EncodeToString(crypto.FromECDSA(key))
		bc.Validators = append(bc.Validators, &Validator{
			Address: crypto.PubkeyToAddress(key.PublicKey).Hex(), NativeBond: 1e12,
			LPStakeAmount: 1e12, LockTime: time.Now().Add(24 * time.Hour), LiquidityPower: 10,
		})
	}
	bc.EnsureRuntimeState()
	bc.PrepareValidatorSetTransition(1)
	var qc *QuorumCertificate
	for i := 0; i < 3; i++ {
		var err error
		qc, _, err = bc.AddConsensusVote(makeSignedVote(t, keys[i], 1, 0, StepPrevote, "0xabc"))
		if err != nil {
			t.Fatal(err)
		}
	}
	if qc == nil || qc.Step != StepPrevote || qc.Hash == "" {
		t.Fatal("expected prevote QC after 3 of 4 equal-power votes")
	}
	if qc.Randomness == "" {
		t.Fatal("QC did not commit aggregate signature randomness")
	}

	conflict := makeSignedVote(t, keys[0], 1, 0, StepPrevote, "0xdef")
	_, evidence, err := bc.AddConsensusVote(conflict)
	if err == nil || evidence == nil || evidence.Hash == "" {
		t.Fatal("expected cryptographic equivocation evidence")
	}
}

func TestProposerCertificateBindsPreviousQCRandomness(t *testing.T) {
	bc, keys := consensusFixture(t, 4)
	for i := 0; i < 3; i++ {
		if _, _, err := bc.AddConsensusVote(makeSignedVote(t, keys[i], 1, 0, StepPrevote, "0xblock")); err != nil {
			t.Fatal(err)
		}
	}
	var qc *QuorumCertificate
	for i := 0; i < 3; i++ {
		var err error
		qc, _, err = bc.AddConsensusVote(makeSignedVote(t, keys[i], 1, 0, StepPrecommit, "0xblock"))
		if err != nil {
			t.Fatal(err)
		}
	}
	if qc == nil || qc.Randomness == "" {
		t.Fatal("precommit QC randomness unavailable")
	}
	cert, err := bc.BuildProposerCertificate(2, 0)
	if err != nil || cert.Entropy != qc.Randomness || !bc.VerifyProposerCertificate(*cert) {
		t.Fatalf("certificate did not bind prior QC: cert=%#v err=%v", cert, err)
	}
	tampered := *cert
	tampered.Entropy = "0xdeadbeef"
	if bc.VerifyProposerCertificate(tampered) {
		t.Fatal("tampered proposer entropy verified")
	}
}

func TestWeightedProposerRotatesDeterministically(t *testing.T) {
	bc := newTestBlockchain()
	for i, address := range []string{
		"0x1111111111111111111111111111111111111111",
		"0x2222222222222222222222222222222222222222",
		"0x3333333333333333333333333333333333333333",
	} {
		bc.Validators = append(bc.Validators, &Validator{Address: address, NativeBond: float64(i+1) * 1e12, LPStakeAmount: float64(i+1) * 1e12, LockTime: time.Now().Add(time.Hour)})
	}
	a, err := bc.SelectBlockProposer(10, 0)
	if err != nil {
		t.Fatal(err)
	}
	b, err := bc.SelectBlockProposer(10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if a.Address != b.Address {
		t.Fatal("same height/round must select the same proposer")
	}
	seen := map[string]bool{}
	for height := uint64(1); height <= 50; height++ {
		v, err := bc.SelectBlockProposer(height, 0)
		if err != nil {
			t.Fatal(err)
		}
		seen[strings.ToLower(v.Address)] = true
	}
	if len(seen) < 2 {
		t.Fatal("weighted proposer schedule did not rotate")
	}
}

func TestConsensusViewChangeRetainsLock(t *testing.T) {
	bc := newTestBlockchain()
	keys := make([]string, 4)
	for i := range keys {
		key, err := crypto.GenerateKey()
		if err != nil {
			t.Fatal(err)
		}
		keys[i] = hex.EncodeToString(crypto.FromECDSA(key))
		bc.Validators = append(bc.Validators, &Validator{Address: crypto.PubkeyToAddress(key.PublicKey).Hex(), NativeBond: 1e12, LPStakeAmount: 1e12, LockTime: time.Now().Add(time.Hour)})
	}
	bc.EnsureRuntimeState()
	bc.PrepareValidatorSetTransition(9)
	bc.ConsensusV2.RoundTimeoutSeconds = 8
	if round := bc.CurrentConsensusRound(9); round != 0 {
		t.Fatalf("initial round=%d", round)
	}
	started := bc.ConsensusV2.RoundStartedAt[9]
	bc.ConsensusV2.LockedHash = "0xlocked"
	if _, changed := bc.AdvanceConsensusRound(9, started+7); changed {
		t.Fatal("round advanced before timeout")
	}
	if _, changed := bc.AdvanceConsensusRound(9, started+8); changed {
		t.Fatal("round advanced without timeout certificate")
	}
	for i := 0; i < 3; i++ {
		vote := ConsensusTimeoutVote{Height: 9, Round: 0}
		if err := SignConsensusTimeoutVote(&vote, keys[i]); err != nil {
			t.Fatal(err)
		}
		if _, err := bc.AddConsensusTimeoutVote(vote); err != nil {
			t.Fatal(err)
		}
	}
	round, changed := bc.AdvanceConsensusRound(9, started+8)
	if !changed || round != 1 {
		t.Fatalf("view change failed: round=%d changed=%v", round, changed)
	}
	if bc.ConsensusV2.LockedHash != "0xlocked" {
		t.Fatal("view change discarded consensus lock")
	}
}

func TestLocalTimeoutVoteFormsCertificateAndAdvancesRound(t *testing.T) {
	bc, keys := consensusFixture(t, 1)
	key, err := crypto.HexToECDSA(keys[0])
	if err != nil {
		t.Fatal(err)
	}
	address := crypto.PubkeyToAddress(key.PublicKey).Hex()
	signer, err := NewLocalValidatorSigner(keys[0], testP256VRFSecret, filepath.Join(t.TempDir(), "slashing.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer signer.Close()
	bc.Network = NewNetworkService(bc)
	bc.LocalValidator = address
	if err := bc.Network.SetValidatorSigner(address, signer); err != nil {
		t.Fatal(err)
	}
	const height = uint64(2)
	if round := bc.CurrentConsensusRound(height); round != 0 {
		t.Fatalf("unexpected initial round %d", round)
	}
	started := bc.ConsensusV2.RoundStartedAt[height]
	now := started + bc.ConsensusV2.RoundTimeoutSeconds
	if !bc.ConsensusRoundTimedOut(height, now) {
		t.Fatal("round timeout was not detected")
	}
	tc, err := bc.CastLocalConsensusTimeout(height, 0)
	if err != nil || tc == nil {
		t.Fatalf("local timeout certificate unavailable: tc=%#v err=%v", tc, err)
	}
	if round, changed := bc.AdvanceConsensusRound(height, now); !changed || round != 1 {
		t.Fatalf("certified local view change failed: round=%d changed=%v", round, changed)
	}
}

func TestPruneConsensusRoundsBoundsFinalizedWorkingHistory(t *testing.T) {
	bc := newTestBlockchain()
	bc.EnsureRuntimeState()
	s := bc.ConsensusV2
	const finalized = uint64(1000)
	cutoff := finalized - consensusHistoryRetentionHeights
	oldKey := fmt.Sprintf("%d/0/prevote", cutoff)
	recentKey := fmt.Sprintf("%d/0/prevote", cutoff+1)
	futureKey := fmt.Sprintf("%d/0/prevote", finalized+1)
	s.Votes[oldKey] = map[string]ConsensusVote{"validator": {Height: cutoff}}
	s.Votes[recentKey] = map[string]ConsensusVote{"validator": {Height: cutoff + 1}}
	s.Votes[futureKey] = map[string]ConsensusVote{"validator": {Height: finalized + 1}}
	s.Votes["malformed"] = map[string]ConsensusVote{}
	s.TimeoutVotes[fmt.Sprintf("%d/0", cutoff)] = map[string]ConsensusTimeoutVote{}
	s.TimeoutVotes[fmt.Sprintf("%d/0", cutoff+1)] = map[string]ConsensusTimeoutVote{}
	s.VRFProofs[fmt.Sprintf("%d/0", cutoff)] = map[string]ConsensusVRFProof{}
	s.VRFProofs[fmt.Sprintf("%d/0", cutoff+1)] = map[string]ConsensusVRFProof{}
	s.CurrentRounds[finalized] = 2
	s.CurrentRounds[finalized+1] = 0
	s.RoundStartedAt[finalized] = 1
	s.RoundStartedAt[finalized+1] = 2
	s.LastQC = &QuorumCertificate{Height: finalized}
	s.LastVRFBeacon = &ConsensusVRFBeacon{Height: finalized}

	bc.pruneConsensusRounds(finalized)

	if _, exists := s.Votes[oldKey]; exists {
		t.Fatal("finalized vote history beyond the retention window was not pruned")
	}
	for _, key := range []string{recentKey, futureKey, "malformed"} {
		if _, exists := s.Votes[key]; !exists {
			t.Fatalf("vote key %q was pruned despite being recent, future, or unparseable", key)
		}
	}
	if _, exists := s.TimeoutVotes[fmt.Sprintf("%d/0", cutoff)]; exists {
		t.Fatal("old timeout vote history was not pruned")
	}
	if _, exists := s.VRFProofs[fmt.Sprintf("%d/0", cutoff)]; exists {
		t.Fatal("old VRF proof history was not pruned")
	}
	if _, exists := s.TimeoutVotes[fmt.Sprintf("%d/0", cutoff+1)]; !exists {
		t.Fatal("recent timeout vote history was pruned")
	}
	if _, exists := s.VRFProofs[fmt.Sprintf("%d/0", cutoff+1)]; !exists {
		t.Fatal("recent VRF proof history was pruned")
	}
	if _, exists := s.CurrentRounds[finalized]; exists {
		t.Fatal("finalized current round was not pruned")
	}
	if _, exists := s.RoundStartedAt[finalized]; exists {
		t.Fatal("finalized round timer was not pruned")
	}
	if _, exists := s.CurrentRounds[finalized+1]; !exists {
		t.Fatal("future current round was pruned")
	}
	if s.LastQC == nil || s.LastQC.Height != finalized || s.LastVRFBeacon == nil || s.LastVRFBeacon.Height != finalized {
		t.Fatal("canonical consensus certificates were pruned")
	}
}

func TestConsensusHistoryHeightParsing(t *testing.T) {
	for key, want := range map[string]uint64{"9/0": 9, "12/4/prevote": 12} {
		if got, ok := consensusHistoryHeight(key); !ok || got != want {
			t.Fatalf("consensusHistoryHeight(%q) = %d, %t; want %d, true", key, got, ok, want)
		}
	}
	for _, key := range []string{"", "9", "bad/0", "/0"} {
		if _, ok := consensusHistoryHeight(key); ok {
			t.Fatalf("malformed key %q was accepted", key)
		}
	}
}
