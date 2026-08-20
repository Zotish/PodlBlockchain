package blockchaincomponent

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/crypto"
)

type ConsensusStep string

const (
	StepPrevote   ConsensusStep = "prevote"
	StepPrecommit ConsensusStep = "precommit"
)

type ValidatorPower struct {
	Address          string  `json:"address"`
	Power            float64 `json:"power"`
	NativeBondWeight float64 `json:"native_bond_weight"`
	LiquidityCredit  float64 `json:"liquidity_credit"`
	HybridBonded     bool    `json:"hybrid_bonded"`
}

type ConsensusVote struct {
	Height    uint64        `json:"height"`
	Round     uint32        `json:"round"`
	Step      ConsensusStep `json:"step"`
	BlockHash string        `json:"block_hash"`
	Validator string        `json:"validator"`
	Signature string        `json:"signature,omitempty"`
}

type QuorumCertificate struct {
	Height        uint64        `json:"height"`
	Round         uint32        `json:"round"`
	Step          ConsensusStep `json:"step"`
	BlockHash     string        `json:"block_hash"`
	VotingPower   float64       `json:"voting_power"`
	RequiredPower float64       `json:"required_power"`
	Validators    []string      `json:"validators"`
	Randomness    string        `json:"randomness"`
	Hash          string        `json:"hash"`
}

// ConsensusTimeoutVote is a signed request to abandon one stalled round.  It
// deliberately does not name a block, so it cannot be reused as a prevote or
// precommit for a conflicting value.
type ConsensusTimeoutVote struct {
	Height    uint64 `json:"height"`
	Round     uint32 `json:"round"`
	Validator string `json:"validator"`
	Signature string `json:"signature"`
}

type TimeoutCertificate struct {
	Height        uint64   `json:"height"`
	Round         uint32   `json:"round"`
	VotingPower   float64  `json:"voting_power"`
	RequiredPower float64  `json:"required_power"`
	Validators    []string `json:"validators"`
	Hash          string   `json:"hash"`
}

// ProposerCertificate makes proposer election independently reproducible. Its
// entropy is sourced from the parent block's canonical RFC 9381 proof when
// present, with the prior signed QC as the compatibility fallback.
type ProposerCertificate struct {
	Height      uint64 `json:"height"`
	Round       uint32 `json:"round"`
	Epoch       uint64 `json:"epoch"`
	SpecHash    string `json:"spec_hash"`
	SetHash     string `json:"validator_set_hash"`
	Proposer    string `json:"proposer"`
	Entropy     string `json:"entropy"`
	Seed        string `json:"seed"`
	ScoreDigest string `json:"score_digest"`
}

type EquivocationEvidence struct {
	Validator string        `json:"validator"`
	Height    uint64        `json:"height"`
	Round     uint32        `json:"round"`
	Step      ConsensusStep `json:"step"`
	VoteA     ConsensusVote `json:"vote_a"`
	VoteB     ConsensusVote `json:"vote_b"`
	Hash      string        `json:"hash"`
}

type ValidatorSetTransition struct {
	ActivationHeight uint64           `json:"activation_height"`
	OldSet           []ValidatorPower `json:"old_set"`
	NewSet           []ValidatorPower `json:"new_set"`
}

type BFTConsensusState struct {
	mu                     sync.Mutex                                 `json:"-"`
	EpochLength            uint64                                     `json:"epoch_length"`
	ActiveSet              []ValidatorPower                           `json:"active_set"`
	PendingTransition      *ValidatorSetTransition                    `json:"pending_transition,omitempty"`
	Votes                  map[string]map[string]ConsensusVote        `json:"votes"`
	LockedHash             string                                     `json:"locked_hash,omitempty"`
	LockedRound            uint32                                     `json:"locked_round,omitempty"`
	LastQC                 *QuorumCertificate                         `json:"last_qc,omitempty"`
	Evidence               []EquivocationEvidence                     `json:"evidence,omitempty"`
	MaxLiquidityCreditBPS  uint32                                     `json:"max_liquidity_credit_bps"`
	CurrentRounds          map[uint64]uint32                          `json:"current_rounds,omitempty"`
	RoundStartedAt         map[uint64]int64                           `json:"round_started_at,omitempty"`
	RoundTimeoutSeconds    int64                                      `json:"round_timeout_seconds"`
	TimeoutVotes           map[string]map[string]ConsensusTimeoutVote `json:"timeout_votes,omitempty"`
	LastTimeoutCertificate *TimeoutCertificate                        `json:"last_timeout_certificate,omitempty"`
	VRFProofs              map[string]map[string]ConsensusVRFProof    `json:"vrf_proofs,omitempty"`
	LastVRFBeacon          *ConsensusVRFBeacon                        `json:"last_vrf_beacon,omitempty"`
}

func NewBFTConsensusState(epochLength uint64) *BFTConsensusState {
	if epochLength == 0 {
		epochLength = DefaultEpochLength
	}
	return &BFTConsensusState{
		EpochLength:           epochLength,
		Votes:                 make(map[string]map[string]ConsensusVote),
		MaxLiquidityCreditBPS: 5000,
		CurrentRounds:         make(map[uint64]uint32),
		RoundStartedAt:        make(map[uint64]int64),
		RoundTimeoutSeconds:   8,
		TimeoutVotes:          make(map[string]map[string]ConsensusTimeoutVote),
		VRFProofs:             make(map[string]map[string]ConsensusVRFProof),
	}
}

func (s *BFTConsensusState) ensure() {
	if s.EpochLength == 0 {
		s.EpochLength = DefaultEpochLength
	}
	if s.Votes == nil {
		s.Votes = make(map[string]map[string]ConsensusVote)
	}
	if s.MaxLiquidityCreditBPS == 0 {
		s.MaxLiquidityCreditBPS = 5000
	}
	if s.CurrentRounds == nil {
		s.CurrentRounds = make(map[uint64]uint32)
	}
	if s.RoundStartedAt == nil {
		s.RoundStartedAt = make(map[uint64]int64)
	}
	if s.RoundTimeoutSeconds <= 0 {
		s.RoundTimeoutSeconds = 8
	}
	if s.TimeoutVotes == nil {
		s.TimeoutVotes = make(map[string]map[string]ConsensusTimeoutVote)
	}
	if s.VRFProofs == nil {
		s.VRFProofs = make(map[string]map[string]ConsensusVRFProof)
	}
}

// CurrentConsensusRound starts and returns the local round timer for a height.
// The round is part of signed votes and proposer selection, so a stalled
// proposer can be replaced without weakening the existing lock.
func (bc *Blockchain_struct) CurrentConsensusRound(height uint64) uint32 {
	if bc == nil {
		return 0
	}
	bc.EnsureRuntimeState()
	s := bc.ConsensusV2
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensure()
	if s.RoundStartedAt[height] == 0 {
		s.RoundStartedAt[height] = time.Now().Unix()
	}
	return s.CurrentRounds[height]
}

// AdvanceConsensusRound performs the view-change timeout transition. Locks are
// deliberately retained (Tendermint safety rule); only proposer/round changes.
func (bc *Blockchain_struct) AdvanceConsensusRound(height uint64, nowUnix int64) (uint32, bool) {
	if bc == nil || height == 0 {
		return 0, false
	}
	bc.EnsureRuntimeState()
	s := bc.ConsensusV2
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensure()
	if nowUnix <= 0 {
		nowUnix = time.Now().Unix()
	}
	started := s.RoundStartedAt[height]
	if started == 0 {
		s.RoundStartedAt[height] = nowUnix
		return s.CurrentRounds[height], false
	}
	if nowUnix-started < s.RoundTimeoutSeconds {
		return s.CurrentRounds[height], false
	}
	currentRound := s.CurrentRounds[height]
	if s.LastTimeoutCertificate == nil || s.LastTimeoutCertificate.Height != height || s.LastTimeoutCertificate.Round != currentRound {
		return currentRound, false
	}
	if s.LastQC != nil && s.LastQC.Height >= height && s.LastQC.Step == StepPrecommit {
		return s.CurrentRounds[height], false
	}
	s.CurrentRounds[height]++
	s.RoundStartedAt[height] = nowUnix
	return s.CurrentRounds[height], true
}

// ConsensusRoundTimedOut reports whether the current round has reached its
// configured timeout. Callers use it to collect a signed timeout certificate
// before AdvanceConsensusRound is allowed to change proposer/round.
func (bc *Blockchain_struct) ConsensusRoundTimedOut(height uint64, nowUnix int64) bool {
	if bc == nil || height == 0 {
		return false
	}
	bc.EnsureRuntimeState()
	s := bc.ConsensusV2
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensure()
	if nowUnix <= 0 {
		nowUnix = time.Now().Unix()
	}
	started := s.RoundStartedAt[height]
	return started > 0 && nowUnix-started >= s.RoundTimeoutSeconds
}

func (bc *Blockchain_struct) pruneConsensusRounds(finalizedHeight uint64) {
	if bc == nil || bc.ConsensusV2 == nil {
		return
	}
	s := bc.ConsensusV2
	s.mu.Lock()
	defer s.mu.Unlock()
	for height := range s.CurrentRounds {
		if height <= finalizedHeight {
			delete(s.CurrentRounds, height)
			delete(s.RoundStartedAt, height)
		}
	}
}

func consensusVoteKey(v ConsensusVote) string {
	return fmt.Sprintf("%d/%d/%s", v.Height, v.Round, v.Step)
}

func consensusVoteMessage(v ConsensusVote) string {
	return fmt.Sprintf("PODL-BFT:%d:%d:%s:%s", v.Height, v.Round, v.Step, strings.ToLower(strings.TrimSpace(v.BlockHash)))
}

func SignConsensusVote(v *ConsensusVote, privateKeyHex string) error {
	if v == nil {
		return fmt.Errorf("nil vote")
	}
	key, err := crypto.HexToECDSA(strings.TrimPrefix(strings.TrimSpace(privateKeyHex), "0x"))
	if err != nil {
		return err
	}
	sig, err := crypto.Sign(accounts.TextHash([]byte(consensusVoteMessage(*v))), key)
	if err != nil {
		return err
	}
	v.Validator = crypto.PubkeyToAddress(key.PublicKey).Hex()
	v.Signature = "0x" + hex.EncodeToString(sig)
	return nil
}

func SignConsensusVoteWithSigner(ctx context.Context, v *ConsensusVote, signer ValidatorSigner) error {
	if v == nil || signer == nil {
		return fmt.Errorf("vote and validator signer are required")
	}
	slot := fmt.Sprintf("vote/%d/%d/%s", v.Height, v.Round, v.Step)
	signature, err := signer.SignMessage(ctx, SignerDomainConsensusVote, []byte(consensusVoteMessage(*v)), slot)
	if err != nil {
		return err
	}
	v.Validator = signer.Address()
	v.Signature = signature
	return nil
}

func VerifyConsensusVote(v ConsensusVote) bool {
	if v.Height == 0 || v.Round > 1000000 || (v.Step != StepPrevote && v.Step != StepPrecommit) || strings.TrimSpace(v.BlockHash) == "" {
		return false
	}
	raw, err := hex.DecodeString(strings.TrimPrefix(strings.TrimSpace(v.Signature), "0x"))
	if err != nil || len(raw) != 65 {
		return false
	}
	pub, err := crypto.SigToPub(accounts.TextHash([]byte(consensusVoteMessage(v))), raw)
	if err != nil {
		return false
	}
	return strings.EqualFold(crypto.PubkeyToAddress(*pub).Hex(), v.Validator)
}

func (bc *Blockchain_struct) buildValidatorPowerSet() []ValidatorPower {
	if bc == nil {
		return nil
	}
	set := make([]ValidatorPower, 0, len(bc.Validators))
	for _, v := range bc.Validators {
		if v == nil || strings.TrimSpace(v.Address) == "" || validatorEffectiveWeight(v) <= 0 {
			continue
		}
		nativeBond := v.NativeBond
		if nativeBond <= 0 && !isDEXBackedValidator(v) {
			nativeBond = v.LPStakeAmount
		}
		nativeWeight := 0.0
		if nativeBond > 0 {
			nativeWeight = math.Sqrt(nativeBond)
		}
		liquidity := math.Max(0, v.LiquidityPower)
		capBase := nativeWeight
		hybrid := nativeWeight > 0
		if capBase == 0 {
			// Testnet bootstrap keeps an LP validator observable but marks it as
			// non-hybrid; mainnet readiness rejects such validators.
			capBase = math.Sqrt(math.Max(1, bc.MinStake))
		}
		maxCredit := capBase * 0.5
		if liquidity > maxCredit {
			liquidity = maxCredit
		}
		power := (capBase + liquidity) * math.Max(0, 1-v.PenaltyScore)
		if power <= 0 {
			continue
		}
		set = append(set, ValidatorPower{
			Address:          strings.ToLower(v.Address),
			Power:            power,
			NativeBondWeight: nativeWeight,
			LiquidityCredit:  liquidity,
			HybridBonded:     hybrid,
		})
	}
	sort.Slice(set, func(i, j int) bool { return set[i].Address < set[j].Address })
	return set
}

func cloneValidatorPowerSet(in []ValidatorPower) []ValidatorPower {
	return append([]ValidatorPower(nil), in...)
}

// PrepareValidatorSetTransition snapshots power at an epoch boundary and
// activates it one epoch later. During the transition, quorum must be reached
// independently in both the old and new sets.
func (bc *Blockchain_struct) PrepareValidatorSetTransition(height uint64) {
	if bc == nil {
		return
	}
	bc.EnsureRuntimeState()
	s := bc.ConsensusV2
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensure()
	if len(s.ActiveSet) == 0 {
		s.ActiveSet = bc.buildValidatorPowerSet()
		return
	}
	if height == 0 || height%s.EpochLength != 0 {
		return
	}
	if s.PendingTransition != nil && height >= s.PendingTransition.ActivationHeight {
		s.ActiveSet = cloneValidatorPowerSet(s.PendingTransition.NewSet)
		s.PendingTransition = nil
	}
	if s.PendingTransition == nil {
		s.PendingTransition = &ValidatorSetTransition{
			ActivationHeight: height + s.EpochLength,
			OldSet:           cloneValidatorPowerSet(s.ActiveSet),
			NewSet:           bc.buildValidatorPowerSet(),
		}
	}
}

func validatorSetPower(set []ValidatorPower) float64 {
	total := 0.0
	for _, v := range set {
		total += v.Power
	}
	return total
}

func validatorPowerFor(set []ValidatorPower, address string) float64 {
	for _, v := range set {
		if strings.EqualFold(v.Address, address) {
			return v.Power
		}
	}
	return 0
}

func requiredQuorum(total float64) float64 {
	// Nextafter is representable at the magnitude of total; adding the
	// smallest subnormal is rounded away for ordinary validator power values.
	return math.Nextafter(total*2/3, math.Inf(1))
}

func votePowerForHash(votes map[string]ConsensusVote, set []ValidatorPower, hash string) (float64, []string) {
	power := 0.0
	validators := []string{}
	for address, vote := range votes {
		if !strings.EqualFold(vote.BlockHash, hash) {
			continue
		}
		p := validatorPowerFor(set, address)
		if p <= 0 {
			continue
		}
		power += p
		validators = append(validators, strings.ToLower(address))
	}
	sort.Strings(validators)
	return power, validators
}

func qcHash(qc *QuorumCertificate) string {
	if qc == nil {
		return ""
	}
	material := fmt.Sprintf("%d|%d|%s|%s|%s|%s", qc.Height, qc.Round, qc.Step, strings.ToLower(qc.BlockHash), strings.Join(qc.Validators, ","), qc.Randomness)
	sum := sha256.Sum256([]byte(material))
	return "0x" + hex.EncodeToString(sum[:])
}

func qcSignatureRandomness(votes map[string]ConsensusVote, validators []string, blockHash string) string {
	parts := make([]string, 0, len(validators))
	for _, validator := range validators {
		vote, ok := votes[strings.ToLower(validator)]
		if !ok || !strings.EqualFold(vote.BlockHash, blockHash) || !VerifyConsensusVote(vote) {
			continue
		}
		parts = append(parts, strings.ToLower(validator)+":"+strings.ToLower(vote.Signature))
	}
	sort.Strings(parts)
	sum := sha256.Sum256([]byte("PODL-QC-RANDOMNESS-V1:" + strings.Join(parts, "|")))
	return "0x" + hex.EncodeToString(sum[:])
}

func evidenceHash(a, b ConsensusVote) string {
	material := consensusVoteMessage(a) + "|" + a.Signature + "|" + consensusVoteMessage(b) + "|" + b.Signature
	sum := sha256.Sum256([]byte(material))
	return "0x" + hex.EncodeToString(sum[:])
}

// AddConsensusVote validates signatures, records equivocation evidence and
// returns a QC only after >=2/3 voting power (and joint quorum during changes).
func (bc *Blockchain_struct) AddConsensusVote(vote ConsensusVote) (*QuorumCertificate, *EquivocationEvidence, error) {
	if bc == nil {
		return nil, nil, fmt.Errorf("nil blockchain")
	}
	if !VerifyConsensusVote(vote) {
		return nil, nil, fmt.Errorf("invalid consensus vote signature")
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
		return nil, nil, fmt.Errorf("validator is not in the active or pending set")
	}

	key := consensusVoteKey(vote)
	if s.Votes[key] == nil {
		s.Votes[key] = make(map[string]ConsensusVote)
	}
	address := strings.ToLower(vote.Validator)
	if prior, ok := s.Votes[key][address]; ok {
		if strings.EqualFold(prior.BlockHash, vote.BlockHash) {
			return nil, nil, nil
		}
		evidence := EquivocationEvidence{
			Validator: address, Height: vote.Height, Round: vote.Round, Step: vote.Step,
			VoteA: prior, VoteB: vote,
		}
		evidence.Hash = evidenceHash(prior, vote)
		s.Evidence = append(s.Evidence, evidence)
		return nil, &evidence, fmt.Errorf("equivocation detected")
	}
	s.Votes[key][address] = vote

	power, validators := votePowerForHash(s.Votes[key], s.ActiveSet, vote.BlockHash)
	required := requiredQuorum(validatorSetPower(s.ActiveSet))
	if power < required {
		return nil, nil, nil
	}
	if transition := s.PendingTransition; transition != nil && vote.Height < transition.ActivationHeight {
		newPower, _ := votePowerForHash(s.Votes[key], transition.NewSet, vote.BlockHash)
		if newPower < requiredQuorum(validatorSetPower(transition.NewSet)) {
			return nil, nil, nil
		}
	}
	qc := &QuorumCertificate{
		Height: vote.Height, Round: vote.Round, Step: vote.Step, BlockHash: vote.BlockHash,
		VotingPower: power, RequiredPower: required, Validators: validators,
	}
	qc.Randomness = qcSignatureRandomness(s.Votes[key], validators, vote.BlockHash)
	qc.Hash = qcHash(qc)
	if vote.Step == StepPrevote {
		s.LockedHash = vote.BlockHash
		s.LockedRound = vote.Round
	}
	if vote.Step == StepPrecommit {
		if s.LockedHash != "" && !strings.EqualFold(s.LockedHash, vote.BlockHash) {
			return nil, nil, fmt.Errorf("precommit conflicts with locked block")
		}
		s.LastQC = qc
	}
	return qc, nil, nil
}

// SelectBlockProposer uses deterministic weighted rendezvous selection. Every
// node with the same epoch snapshot derives the same proposer, while the seed
// changes by height and round so one high-power validator cannot always win.
func (bc *Blockchain_struct) SelectBlockProposer(height uint64, round uint32) (Validator, error) {
	if bc == nil {
		return Validator{}, fmt.Errorf("no validator for selection")
	}
	// Initialize ChainSpec/ConsensusV2 before deriving entropy. Otherwise the
	// first call could hash a zero-value spec and the second call the repaired
	// spec, producing two proposers for the same height/round.
	bc.EnsureRuntimeState()
	return bc.selectBlockProposerWithEntropy(height, round, bc.proposerEntropy(height))
}

func (bc *Blockchain_struct) proposerEntropy(height uint64) string {
	if output := bc.canonicalBlockVRFEntropy(height); output != "" {
		return output
	}
	if bc != nil && bc.ConsensusV2 != nil && bc.ConsensusV2.LastQC != nil && bc.ConsensusV2.LastQC.Height+1 == height && bc.ConsensusV2.LastQC.Randomness != "" {
		return bc.ConsensusV2.LastQC.Randomness
	}
	if bc != nil {
		return bc.ChainSpec.Hash()
	}
	return ""
}

func (bc *Blockchain_struct) selectBlockProposerWithEntropy(height uint64, round uint32, entropy string) (Validator, error) {
	if bc == nil || len(bc.Validators) == 0 {
		return Validator{}, fmt.Errorf("no validator for selection")
	}
	bc.EnsureRuntimeState()
	bc.PrepareValidatorSetTransition(height)
	set := bc.ConsensusV2.ActiveSet
	if len(set) == 0 {
		return Validator{}, fmt.Errorf("empty validator power set")
	}
	seed := make([]byte, 12)
	binary.BigEndian.PutUint64(seed[:8], height)
	binary.BigEndian.PutUint32(seed[8:], round)
	seedMaterial := append([]byte("PODL-QC-SORTITION-V1:"+strings.ToLower(entropy)+":"), seed...)
	bestAddress := ""
	bestScore := math.Inf(1)
	for _, candidate := range set {
		if candidate.Power <= 0 {
			continue
		}
		sum := sha256.Sum256(append(append([]byte(nil), seedMaterial...), []byte(strings.ToLower(candidate.Address))...))
		raw := binary.BigEndian.Uint64(sum[:8])
		u := (float64(raw) + 1) / (float64(^uint64(0)) + 1)
		score := -math.Log(u) / candidate.Power
		if score < bestScore {
			bestScore = score
			bestAddress = candidate.Address
		}
	}
	for _, validator := range bc.Validators {
		if validator != nil && strings.EqualFold(validator.Address, bestAddress) {
			return *validator, nil
		}
	}
	return Validator{}, fmt.Errorf("selected proposer is unavailable")
}

func (bc *Blockchain_struct) ConsensusStatus() map[string]interface{} {
	if bc == nil {
		return map[string]interface{}{"ready": false}
	}
	bc.EnsureRuntimeState()
	s := bc.ConsensusV2
	s.mu.Lock()
	defer s.mu.Unlock()
	hybrid := true
	for _, v := range s.ActiveSet {
		if !v.HybridBonded {
			hybrid = false
		}
	}
	return map[string]interface{}{
		"ready":              len(s.ActiveSet) >= 4 && hybrid,
		"epoch_length":       s.EpochLength,
		"active_set":         cloneValidatorPowerSet(s.ActiveSet),
		"pending_transition": s.PendingTransition,
		"locked_hash":        s.LockedHash,
		"locked_round":       s.LockedRound,
		"last_qc":            s.LastQC,
		"evidence_count":     len(s.Evidence),
		"hybrid_bonded":      hybrid,
		"round_timeout_s":    s.RoundTimeoutSeconds,
		"current_rounds":     s.CurrentRounds,
		"last_vrf_beacon":    s.LastVRFBeacon,
	}
}

func (bc *Blockchain_struct) hasConsensusVote(height uint64, round uint32, step ConsensusStep, validator string) bool {
	if bc == nil || bc.ConsensusV2 == nil {
		return false
	}
	s := bc.ConsensusV2
	s.mu.Lock()
	defer s.mu.Unlock()
	key := fmt.Sprintf("%d/%d/%s", height, round, step)
	_, ok := s.Votes[key][strings.ToLower(strings.TrimSpace(validator))]
	return ok
}

func (bc *Blockchain_struct) hasConsensusTimeoutVote(height uint64, round uint32, validator string) bool {
	if bc == nil || bc.ConsensusV2 == nil {
		return false
	}
	s := bc.ConsensusV2
	s.mu.Lock()
	defer s.mu.Unlock()
	key := timeoutVoteKey(height, round)
	_, ok := s.TimeoutVotes[key][strings.ToLower(strings.TrimSpace(validator))]
	return ok
}

func (bc *Blockchain_struct) HasPrecommitQC(height uint64, blockHash string) bool {
	if bc == nil || bc.ConsensusV2 == nil {
		return false
	}
	s := bc.ConsensusV2
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.LastQC != nil && s.LastQC.Height == height && s.LastQC.Step == StepPrecommit && strings.EqualFold(s.LastQC.BlockHash, blockHash)
}

func (bc *Blockchain_struct) createLocalConsensusVote(block *Block, step ConsensusStep) (ConsensusVote, error) {
	if bc == nil || block == nil || bc.Network == nil {
		return ConsensusVote{}, fmt.Errorf("consensus network unavailable")
	}
	address, signer := bc.Network.ValidatorSignerSnapshot()
	if address == "" || signer == nil {
		return ConsensusVote{}, fmt.Errorf("validator signing identity is not configured")
	}
	vote := ConsensusVote{Height: block.BlockNumber, Round: block.ConsensusRound, Step: step, BlockHash: block.CurrentHash, Validator: address}
	if err := SignConsensusVoteWithSigner(context.Background(), &vote, signer); err != nil {
		return ConsensusVote{}, err
	}
	if !strings.EqualFold(vote.Validator, address) || (bc.LocalValidator != "" && !strings.EqualFold(vote.Validator, bc.LocalValidator)) {
		return ConsensusVote{}, fmt.Errorf("validator private key does not match configured address")
	}
	return vote, nil
}

func (bc *Blockchain_struct) CastLocalConsensusStep(block *Block, step ConsensusStep) (*QuorumCertificate, error) {
	vote, err := bc.createLocalConsensusVote(block, step)
	if err != nil {
		return nil, err
	}
	if bc.hasConsensusVote(vote.Height, vote.Round, vote.Step, vote.Validator) {
		return nil, nil
	}
	qc, evidence, err := bc.AddConsensusVote(vote)
	if evidence != nil {
		_, _ = bc.OpenSlashingCase(evidence.Validator, evidence.Hash, "signed BFT equivocation", DoubleSigningPenalty, evidence.Height, 100)
	}
	if err != nil {
		return nil, err
	}
	bc.Network.BroadcastConsensusVote(vote)
	if qc != nil && qc.Step == StepPrevote {
		return bc.CastLocalConsensusStep(block, StepPrecommit)
	}
	return qc, nil
}

func (bc *Blockchain_struct) ProcessConsensusVote(vote ConsensusVote) (bool, error) {
	qc, evidence, err := bc.AddConsensusVote(vote)
	if evidence != nil {
		_, _ = bc.OpenSlashingCase(evidence.Validator, evidence.Hash, "signed BFT equivocation", DoubleSigningPenalty, evidence.Height, 100)
	}
	if err != nil {
		return false, err
	}
	if qc == nil {
		return false, nil
	}
	if qc.Step == StepPrevote {
		block := bc.PendingBlocks[vote.BlockHash]
		if block != nil {
			_, _ = bc.CastLocalConsensusStep(block, StepPrecommit)
		}
		return false, nil
	}
	bc.Mutex.Lock()
	finalized := bc.TryFinalizePending(vote.BlockHash, 0.67)
	bc.Mutex.Unlock()
	return finalized, nil
}
