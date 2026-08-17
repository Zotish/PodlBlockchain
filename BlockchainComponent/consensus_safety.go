package blockchaincomponent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/crypto"
)

func timeoutVoteKey(height uint64, round uint32) string {
	return fmt.Sprintf("%d/%d", height, round)
}

func timeoutVoteMessage(v ConsensusTimeoutVote) string {
	return fmt.Sprintf("PODL-BFT-TIMEOUT:%d:%d", v.Height, v.Round)
}

func SignConsensusTimeoutVote(v *ConsensusTimeoutVote, privateKeyHex string) error {
	if v == nil || v.Height == 0 {
		return fmt.Errorf("valid timeout vote required")
	}
	key, err := crypto.HexToECDSA(strings.TrimPrefix(strings.TrimSpace(privateKeyHex), "0x"))
	if err != nil {
		return err
	}
	sig, err := crypto.Sign(accounts.TextHash([]byte(timeoutVoteMessage(*v))), key)
	if err != nil {
		return err
	}
	v.Validator = strings.ToLower(crypto.PubkeyToAddress(key.PublicKey).Hex())
	v.Signature = "0x" + hex.EncodeToString(sig)
	return nil
}

func SignConsensusTimeoutVoteWithSigner(ctx context.Context, v *ConsensusTimeoutVote, signer ValidatorSigner) error {
	if v == nil || v.Height == 0 || signer == nil {
		return fmt.Errorf("valid timeout vote and signer required")
	}
	slot := fmt.Sprintf("timeout/%d/%d", v.Height, v.Round)
	signature, err := signer.SignMessage(ctx, SignerDomainConsensusTimeout, []byte(timeoutVoteMessage(*v)), slot)
	if err != nil {
		return err
	}
	v.Validator = signer.Address()
	v.Signature = signature
	return nil
}

func VerifyConsensusTimeoutVote(v ConsensusTimeoutVote) bool {
	if v.Height == 0 || v.Round > 1_000_000 || strings.TrimSpace(v.Validator) == "" {
		return false
	}
	raw, err := hex.DecodeString(strings.TrimPrefix(v.Signature, "0x"))
	if err != nil || len(raw) != 65 {
		return false
	}
	pub, err := crypto.SigToPub(accounts.TextHash([]byte(timeoutVoteMessage(v))), raw)
	return err == nil && strings.EqualFold(crypto.PubkeyToAddress(*pub).Hex(), v.Validator)
}

func timeoutCertificateHash(tc *TimeoutCertificate) string {
	if tc == nil {
		return ""
	}
	material := fmt.Sprintf("PODL-TC:%d:%d:%s", tc.Height, tc.Round, strings.Join(tc.Validators, ","))
	sum := sha256.Sum256([]byte(material))
	return "0x" + hex.EncodeToString(sum[:])
}

// AddConsensusTimeoutVote creates a certificate only after the active set and,
// during validator-set transition, the pending set each reach >2/3 power.
func (bc *Blockchain_struct) AddConsensusTimeoutVote(vote ConsensusTimeoutVote) (*TimeoutCertificate, error) {
	if bc == nil || !VerifyConsensusTimeoutVote(vote) {
		return nil, fmt.Errorf("invalid timeout vote")
	}
	bc.EnsureRuntimeState()
	s := bc.ConsensusV2
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensure()
	if len(s.ActiveSet) == 0 {
		s.ActiveSet = bc.buildValidatorPowerSet()
	}
	if validatorPowerFor(s.ActiveSet, vote.Validator) <= 0 && (s.PendingTransition == nil || validatorPowerFor(s.PendingTransition.NewSet, vote.Validator) <= 0) {
		return nil, fmt.Errorf("timeout voter is not in validator set")
	}
	key := timeoutVoteKey(vote.Height, vote.Round)
	if s.TimeoutVotes[key] == nil {
		s.TimeoutVotes[key] = make(map[string]ConsensusTimeoutVote)
	}
	address := strings.ToLower(vote.Validator)
	if _, exists := s.TimeoutVotes[key][address]; exists {
		return nil, nil
	}
	s.TimeoutVotes[key][address] = vote
	voters := make(map[string]ConsensusVote, len(s.TimeoutVotes[key]))
	for signer := range s.TimeoutVotes[key] {
		voters[signer] = ConsensusVote{Validator: signer, BlockHash: "timeout"}
	}
	power, validators := votePowerForHash(voters, s.ActiveSet, "timeout")
	required := requiredQuorum(validatorSetPower(s.ActiveSet))
	if power < required {
		return nil, nil
	}
	if tr := s.PendingTransition; tr != nil && vote.Height < tr.ActivationHeight {
		newPower, _ := votePowerForHash(voters, tr.NewSet, "timeout")
		if newPower < requiredQuorum(validatorSetPower(tr.NewSet)) {
			return nil, nil
		}
	}
	tc := &TimeoutCertificate{Height: vote.Height, Round: vote.Round, VotingPower: power, RequiredPower: required, Validators: validators}
	tc.Hash = timeoutCertificateHash(tc)
	s.LastTimeoutCertificate = tc
	return tc, nil
}

func validatorPowerSetHash(set []ValidatorPower) string {
	copySet := cloneValidatorPowerSet(set)
	sort.Slice(copySet, func(i, j int) bool { return copySet[i].Address < copySet[j].Address })
	raw, _ := json.Marshal(copySet)
	sum := sha256.Sum256(raw)
	return "0x" + hex.EncodeToString(sum[:])
}

func proposerSeed(specHash, setHash, entropy string, height uint64, round uint32) string {
	material := fmt.Sprintf("PODL-PROPOSER-V3:%s:%s:%s:%d:%d", specHash, setHash, entropy, height, round)
	sum := sha256.Sum256([]byte(material))
	return "0x" + hex.EncodeToString(sum[:])
}

func proposerScoreDigest(seed, proposer string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(seed) + "|" + strings.ToLower(proposer)))
	return "0x" + hex.EncodeToString(sum[:])
}

func (bc *Blockchain_struct) BuildProposerCertificate(height uint64, round uint32) (*ProposerCertificate, error) {
	if bc == nil {
		return nil, fmt.Errorf("nil blockchain")
	}
	proposer, err := bc.SelectBlockProposer(height, round)
	if err != nil {
		return nil, err
	}
	set := bc.ConsensusV2.ActiveSet
	setHash := validatorPowerSetHash(set)
	specHash := bc.ChainSpec.Hash()
	entropy := bc.proposerEntropy(height)
	seed := proposerSeed(specHash, setHash, entropy, height, round)
	return &ProposerCertificate{Height: height, Round: round, Epoch: height / bc.ConsensusV2.EpochLength, SpecHash: specHash, SetHash: setHash, Proposer: strings.ToLower(proposer.Address), Entropy: entropy, Seed: seed, ScoreDigest: proposerScoreDigest(seed, proposer.Address)}, nil
}

func (bc *Blockchain_struct) VerifyProposerCertificate(cert ProposerCertificate) bool {
	if bc == nil || cert.Height == 0 || cert.SpecHash != bc.ChainSpec.Hash() {
		return false
	}
	setHash := validatorPowerSetHash(bc.ConsensusV2.ActiveSet)
	if cert.Entropy == "" || cert.SetHash != setHash || cert.Seed != proposerSeed(cert.SpecHash, setHash, cert.Entropy, cert.Height, cert.Round) || cert.ScoreDigest != proposerScoreDigest(cert.Seed, cert.Proposer) {
		return false
	}
	if cert.Entropy != bc.proposerEntropy(cert.Height) {
		return false
	}
	expected, err := bc.selectBlockProposerWithEntropy(cert.Height, cert.Round, cert.Entropy)
	return err == nil && strings.EqualFold(expected.Address, cert.Proposer)
}

type SlashingCase struct {
	ID              string                         `json:"id"`
	Validator       string                         `json:"validator"`
	EvidenceHash    string                         `json:"evidence_hash"`
	Reason          string                         `json:"reason"`
	Penalty         float64                        `json:"penalty"`
	OpenedHeight    uint64                         `json:"opened_height"`
	ChallengeUntil  uint64                         `json:"challenge_until"`
	Status          string                         `json:"status"`
	AppealHash      string                         `json:"appeal_hash,omitempty"`
	GovernanceID    string                         `json:"governance_id,omitempty"`
	ResolvedAt      int64                          `json:"resolved_at,omitempty"`
	CouncilVotes    map[string]SlashingCouncilVote `json:"council_votes,omitempty"`
	CouncilDecision string                         `json:"council_decision,omitempty"`
}

type SlashingCouncilVote struct {
	Decision   string `json:"decision"`
	ReasonHash string `json:"reason_hash"`
	Height     uint64 `json:"height"`
}

func slashingCaseID(validator, evidence string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(validator) + "|" + strings.ToLower(evidence)))
	return "slash_" + hex.EncodeToString(sum[:12])
}

func (bc *Blockchain_struct) OpenSlashingCase(validator, evidenceHash, reason string, penalty float64, height, challengeBlocks uint64) (*SlashingCase, error) {
	if bc == nil || !ValidateAddress(validator) || strings.TrimSpace(evidenceHash) == "" || penalty <= 0 || penalty > 1 {
		return nil, fmt.Errorf("valid validator, evidence and penalty required")
	}
	if challengeBlocks == 0 {
		challengeBlocks = 100
	}
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	bc.EnsureRuntimeState()
	id := slashingCaseID(validator, evidenceHash)
	if existing := bc.SlashingCases[id]; existing != nil {
		return existing, nil
	}
	c := &SlashingCase{ID: id, Validator: strings.ToLower(validator), EvidenceHash: evidenceHash, Reason: reason, Penalty: penalty, OpenedHeight: height, ChallengeUntil: height + challengeBlocks, Status: "challengeable", CouncilVotes: map[string]SlashingCouncilVote{}}
	bc.SlashingCases[id] = c
	bc.persistRuntimeStateLocked()
	return c, nil
}

func (bc *Blockchain_struct) AppealSlashingCase(caseID, validator, appealHash string, height uint64) error {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	bc.EnsureRuntimeState()
	c := bc.SlashingCases[caseID]
	if c == nil || c.Status != "challengeable" || height > c.ChallengeUntil || !strings.EqualFold(c.Validator, validator) || strings.TrimSpace(appealHash) == "" {
		return fmt.Errorf("slashing case is not appealable")
	}
	c.AppealHash, c.Status = appealHash, "appealed"
	bc.persistRuntimeStateLocked()
	return nil
}

func (bc *Blockchain_struct) deterministicTimeForHeight(height uint64) time.Time {
	for i := len(bc.Blocks) - 1; i >= 0; i-- {
		if bc.Blocks[i] != nil && bc.Blocks[i].BlockNumber <= height {
			return time.Unix(int64(bc.Blocks[i].TimeStamp), 0)
		}
	}
	return time.Unix(int64(height), 0)
}

// ResolveSlashingCase is the backward-compatible governance path. New
// deployments should configure the independent slashing council and use its
// signed transaction approvals below.
func (bc *Blockchain_struct) ResolveSlashingCase(caseID, governanceID string, uphold bool, height uint64) error {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	bc.EnsureRuntimeState()
	c := bc.SlashingCases[caseID]
	proposal := bc.Governance.Proposals[governanceID]
	if c == nil || proposal == nil || proposal.Status != "executed" || (c.Status != "challengeable" && c.Status != "appealed") {
		return fmt.Errorf("executed governance adjudication required")
	}
	if c.Status == "challengeable" && height <= c.ChallengeUntil {
		return fmt.Errorf("challenge window remains open")
	}
	if uphold {
		bc.slashValidatorAt(c.Validator, c.Penalty, c.Reason, bc.deterministicTimeForHeight(height))
		c.Status = "upheld"
	} else {
		c.Status = "dismissed"
	}
	c.GovernanceID, c.ResolvedAt = governanceID, bc.deterministicTimeForHeight(height).Unix()
	bc.persistRuntimeStateLocked()
	return nil
}

func (bc *Blockchain_struct) approveSlashingCaseCouncilLocked(caseID, member, decision, reasonHash string, height uint64, timestamp int64) error {
	c := bc.SlashingCases[caseID]
	member, decision = strings.ToLower(strings.TrimSpace(member)), strings.ToLower(strings.TrimSpace(decision))
	if c == nil || (c.Status != "challengeable" && c.Status != "appealed") || height <= c.ChallengeUntil {
		return fmt.Errorf("slashing case is not council-decidable")
	}
	if decision != "uphold" && decision != "dismiss" {
		return fmt.Errorf("council decision must be uphold or dismiss")
	}
	if strings.TrimSpace(reasonHash) == "" || bc.Governance == nil || !bc.Governance.SlashingCouncil[member] || bc.Governance.SlashingCouncilThreshold < 2 || (bc.Governance.SlashingCouncilExpiryBlock > 0 && height > bc.Governance.SlashingCouncilExpiryBlock) {
		return fmt.Errorf("active slashing council authorization required")
	}
	if c.CouncilVotes == nil {
		c.CouncilVotes = map[string]SlashingCouncilVote{}
	}
	if previous, exists := c.CouncilVotes[member]; exists {
		if previous.Decision == decision && previous.ReasonHash == reasonHash {
			return nil
		}
		return fmt.Errorf("council member already voted")
	}
	c.CouncilVotes[member] = SlashingCouncilVote{Decision: decision, ReasonHash: reasonHash, Height: height}
	approvals := 0
	for _, vote := range c.CouncilVotes {
		if vote.Decision == decision {
			approvals++
		}
	}
	if approvals < bc.Governance.SlashingCouncilThreshold {
		return nil
	}
	decisionTime := time.Unix(timestamp, 0)
	if timestamp <= 0 {
		decisionTime = bc.deterministicTimeForHeight(height)
	}
	if decision == "uphold" {
		bc.slashValidatorAt(c.Validator, c.Penalty, c.Reason, decisionTime)
		c.Status = "upheld"
	} else {
		c.Status = "dismissed"
	}
	c.CouncilDecision, c.ResolvedAt = decision, decisionTime.Unix()
	return nil
}

func (bc *Blockchain_struct) ApproveSlashingCaseCouncil(caseID, member, decision, reasonHash string, height uint64, timestamp int64) error {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	bc.EnsureRuntimeState()
	if err := bc.approveSlashingCaseCouncilLocked(caseID, member, decision, reasonHash, height, timestamp); err != nil {
		return err
	}
	bc.persistRuntimeStateLocked()
	return nil
}
