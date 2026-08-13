package blockchaincomponent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type GovernanceAction struct {
	Module    string `json:"module"`
	Parameter string `json:"parameter"`
	Value     string `json:"value"`
}

type GovernanceVote struct {
	Voter  string  `json:"voter"`
	Choice string  `json:"choice"`
	Power  float64 `json:"power"`
}

type GovernanceProposal struct {
	ID              string                    `json:"id"`
	Proposer        string                    `json:"proposer"`
	Title           string                    `json:"title"`
	DescriptionHash string                    `json:"description_hash"`
	Actions         []GovernanceAction        `json:"actions"`
	StartHeight     uint64                    `json:"start_height"`
	EndHeight       uint64                    `json:"end_height"`
	ExecuteHeight   uint64                    `json:"execute_height,omitempty"`
	Status          string                    `json:"status"`
	SnapshotPower   map[string]float64        `json:"snapshot_power"`
	Votes           map[string]GovernanceVote `json:"votes"`
	ForPower        float64                   `json:"for_power"`
	AgainstPower    float64                   `json:"against_power"`
	AbstainPower    float64                   `json:"abstain_power"`
	VetoApprovals   map[string]bool           `json:"veto_approvals,omitempty"`
}

type GovernanceState struct {
	Proposals                  map[string]*GovernanceProposal `json:"proposals"`
	VotingPeriodBlocks         uint64                         `json:"voting_period_blocks"`
	TimelockBlocks             uint64                         `json:"timelock_blocks"`
	QuorumBPS                  int64                          `json:"quorum_bps"`
	ApprovalBPS                int64                          `json:"approval_bps"`
	Guardian                   string                         `json:"guardian,omitempty"`
	GuardianExpiryBlock        uint64                         `json:"guardian_expiry_block,omitempty"`
	Nonce                      uint64                         `json:"nonce"`
	Guardians                  map[string]bool                `json:"guardians,omitempty"`
	GuardianThreshold          int                            `json:"guardian_threshold,omitempty"`
	GuardianActions            map[string]*GuardianAction     `json:"guardian_actions,omitempty"`
	AuditTrail                 []GovernanceAuditEvent         `json:"audit_trail,omitempty"`
	Delegations                map[string]string              `json:"delegations,omitempty"`
	SlashingCouncil            map[string]bool                `json:"slashing_council,omitempty"`
	SlashingCouncilThreshold   int                            `json:"slashing_council_threshold,omitempty"`
	SlashingCouncilExpiryBlock uint64                         `json:"slashing_council_expiry_block,omitempty"`
}

type GuardianAction struct {
	ID              string          `json:"id"`
	Module          string          `json:"module"`
	ExpiresAtHeight uint64          `json:"expires_at_height"`
	Approvals       map[string]bool `json:"approvals"`
	Executed        bool            `json:"executed"`
}
type GovernanceAuditEvent struct {
	Height     uint64 `json:"height"`
	Operation  string `json:"operation"`
	Actor      string `json:"actor"`
	ProposalID string `json:"proposal_id,omitempty"`
	Detail     string `json:"detail,omitempty"`
}

func NewGovernanceState() *GovernanceState {
	return &GovernanceState{Proposals: make(map[string]*GovernanceProposal), VotingPeriodBlocks: 1000, TimelockBlocks: 500, QuorumBPS: 4000, ApprovalBPS: 5001, Guardians: map[string]bool{}, GuardianThreshold: 2, GuardianActions: map[string]*GuardianAction{}, AuditTrail: []GovernanceAuditEvent{}, Delegations: map[string]string{}, SlashingCouncil: map[string]bool{}}
}

func governanceProposalID(proposer, title string, nonce uint64) string {
	sum := sha256.Sum256([]byte(strings.ToLower(proposer) + "|" + strings.TrimSpace(title) + "|" + strconv.FormatUint(nonce, 10)))
	return "gov_" + hex.EncodeToString(sum[:12])
}

func (bc *Blockchain_struct) governancePowerSnapshot() map[string]float64 {
	power := map[string]float64{}
	for _, validator := range bc.buildValidatorPowerSet() {
		power[strings.ToLower(validator.Address)] = validator.Power
	}
	if bc.Governance == nil || len(bc.Governance.Delegations) == 0 {
		return power
	}
	delegated := make(map[string]float64, len(power))
	for owner, amount := range power {
		destination := owner
		seen := map[string]bool{owner: true}
		for i := 0; i < 64; i++ {
			next := strings.ToLower(strings.TrimSpace(bc.Governance.Delegations[destination]))
			if next == "" || seen[next] {
				break
			}
			seen[next] = true
			destination = next
		}
		delegated[destination] += amount
	}
	return delegated
}

func validateGovernanceAction(action GovernanceAction) error {
	action.Module = strings.ToLower(strings.TrimSpace(action.Module))
	action.Parameter = strings.ToLower(strings.TrimSpace(action.Parameter))
	allowed := map[string]map[string]bool{
		"economics":    {"insurance_bps": true, "lp_yield_bps": true, "operations_bps": true, "buyback_bps": true, "buyback_enabled": true, "min_insurance_balance": true, "issuance_cap": true, "treasury_deployment_cap_bps": true, "lqd_exposure_cap_bps": true},
		"consensus":    {"round_timeout_seconds": true, "max_liquidity_credit_bps": true},
		"safety":       {"bridge_paused": true, "vault_paused": true, "router_paused": true},
		"oracle":       {"publisher": true},
		"liquidity":    {"pair_policy": true},
		"protocol_arb": {"enabled": true, "max_capital_bps": true, "min_profit_bps": true},
		"governance":   {"guardian_config": true, "slashing_council": true},
		"bridge":       {"risk_policy": true, "light_client": true},
	}
	if !allowed[action.Module][action.Parameter] {
		return fmt.Errorf("unsupported governance action %s.%s", action.Module, action.Parameter)
	}
	return nil
}

func (bc *Blockchain_struct) CreateGovernanceProposal(proposer, title, descriptionHash string, actions []GovernanceAction, currentHeight uint64) (*GovernanceProposal, error) {
	if bc == nil || !ValidateAddress(proposer) || strings.TrimSpace(title) == "" || len(actions) == 0 || len(actions) > 16 {
		return nil, fmt.Errorf("valid proposer, title and 1..16 actions are required")
	}
	for _, action := range actions {
		if err := validateGovernanceAction(action); err != nil {
			return nil, err
		}
	}
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	bc.EnsureRuntimeState()
	snapshot := bc.governancePowerSnapshot()
	if snapshot[strings.ToLower(proposer)] <= 0 {
		return nil, fmt.Errorf("proposer has no bonded governance power")
	}
	bc.Governance.Nonce++
	p := &GovernanceProposal{ID: governanceProposalID(proposer, title, bc.Governance.Nonce), Proposer: strings.ToLower(proposer), Title: strings.TrimSpace(title), DescriptionHash: strings.TrimSpace(descriptionHash), Actions: append([]GovernanceAction(nil), actions...), StartHeight: currentHeight, EndHeight: currentHeight + bc.Governance.VotingPeriodBlocks, Status: "voting", SnapshotPower: snapshot, Votes: map[string]GovernanceVote{}, VetoApprovals: map[string]bool{}}
	bc.Governance.Proposals[p.ID] = p
	bc.persistRuntimeStateLocked()
	copyProposal := *p
	return &copyProposal, nil
}

func (bc *Blockchain_struct) VoteGovernance(proposalID, voter, choice string, currentHeight uint64) (*GovernanceProposal, error) {
	choice = strings.ToLower(strings.TrimSpace(choice))
	if choice != "for" && choice != "against" && choice != "abstain" {
		return nil, fmt.Errorf("choice must be for, against or abstain")
	}
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	bc.EnsureRuntimeState()
	p := bc.Governance.Proposals[proposalID]
	if p == nil || p.Status != "voting" || currentHeight > p.EndHeight {
		return nil, fmt.Errorf("proposal is not open")
	}
	voter = strings.ToLower(strings.TrimSpace(voter))
	power := p.SnapshotPower[voter]
	if power <= 0 {
		return nil, fmt.Errorf("voter has no snapshot power")
	}
	if _, exists := p.Votes[voter]; exists {
		return nil, fmt.Errorf("voter already voted")
	}
	p.Votes[voter] = GovernanceVote{Voter: voter, Choice: choice, Power: power}
	switch choice {
	case "for":
		p.ForPower += power
	case "against":
		p.AgainstPower += power
	default:
		p.AbstainPower += power
	}
	bc.persistRuntimeStateLocked()
	return p, nil
}

func (bc *Blockchain_struct) QueueGovernanceProposal(proposalID string, currentHeight uint64) (*GovernanceProposal, error) {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	bc.EnsureRuntimeState()
	p := bc.Governance.Proposals[proposalID]
	if p == nil || p.Status != "voting" || currentHeight <= p.EndHeight {
		return nil, fmt.Errorf("proposal voting is not complete")
	}
	total := 0.0
	for _, power := range p.SnapshotPower {
		total += power
	}
	participating := p.ForPower + p.AgainstPower + p.AbstainPower
	approvalDenom := p.ForPower + p.AgainstPower
	quorumReached := total > 0 && participating*10000 >= total*float64(bc.Governance.QuorumBPS)
	approved := approvalDenom > 0 && p.ForPower*10000 >= approvalDenom*float64(bc.Governance.ApprovalBPS)
	if !quorumReached || !approved {
		p.Status = "rejected"
		bc.persistRuntimeStateLocked()
		return p, nil
	}
	p.Status = "timelocked"
	p.ExecuteHeight = currentHeight + bc.Governance.TimelockBlocks
	bc.persistRuntimeStateLocked()
	return p, nil
}

func parseGovernanceInt(value string) (int64, error) {
	n, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return n, err
}

func (bc *Blockchain_struct) applyGovernanceAction(action GovernanceAction) error {
	module := strings.ToLower(action.Module)
	parameter := strings.ToLower(action.Parameter)
	switch module {
	case "economics":
		p := bc.EconomicPolicy
		switch parameter {
		case "buyback_enabled":
			p.BuybackEnabled = strings.EqualFold(action.Value, "true")
		case "min_insurance_balance":
			p.MinInsuranceBalance = action.Value
		case "issuance_cap":
			p.IssuanceCap = action.Value
		default:
			v, err := parseGovernanceInt(action.Value)
			if err != nil {
				return err
			}
			switch parameter {
			case "insurance_bps":
				p.InsuranceBPS = v
			case "lp_yield_bps":
				p.LPYieldBPS = v
			case "operations_bps":
				p.OperationsBPS = v
			case "buyback_bps":
				p.BuybackBPS = v
			case "treasury_deployment_cap_bps":
				p.TreasuryDeployCapBPS = v
			case "lqd_exposure_cap_bps":
				p.LQDExposureCapBPS = v
			}
		}
		bc.EconomicPolicy = p
	case "consensus":
		v, err := parseGovernanceInt(action.Value)
		if err != nil {
			return err
		}
		if parameter == "round_timeout_seconds" {
			if v < 2 || v > 120 {
				return fmt.Errorf("round timeout out of range")
			}
			bc.ConsensusV2.RoundTimeoutSeconds = v
		}
		if parameter == "max_liquidity_credit_bps" {
			if v < 0 || v > 5000 {
				return fmt.Errorf("liquidity credit cap out of range")
			}
			bc.ConsensusV2.MaxLiquidityCreditBPS = uint32(v)
		}
	case "safety":
		paused := strings.EqualFold(action.Value, "true")
		bc.ProtocolPauses[strings.TrimSuffix(parameter, "_paused")] = paused
	case "oracle":
		parts := strings.SplitN(action.Value, ",", 2)
		if parameter != "publisher" || len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || !ValidateAddress(strings.TrimSpace(parts[1])) {
			return fmt.Errorf("oracle publisher value must be source,address")
		}
		bc.OraclePublishers[strings.ToLower(strings.TrimSpace(parts[0]))] = strings.ToLower(strings.TrimSpace(parts[1]))
	case "liquidity":
		if parameter != "pair_policy" {
			return fmt.Errorf("unsupported liquidity action")
		}
		var policy PairRiskPolicy
		if err := json.Unmarshal([]byte(action.Value), &policy); err != nil {
			return fmt.Errorf("invalid pair policy: %w", err)
		}
		policy.PairAddress = strings.ToLower(strings.TrimSpace(policy.PairAddress))
		if err := policy.Validate(); err != nil {
			return err
		}
		bc.PairRiskPolicies[policy.PairAddress] = policy
	case "protocol_arb":
		if parameter == "enabled" {
			bc.ArbPolicy.Enabled = strings.EqualFold(action.Value, "true")
			break
		}
		v, err := parseGovernanceInt(action.Value)
		if err != nil {
			return err
		}
		if parameter == "max_capital_bps" {
			if v < 1 || v > 1000 {
				return fmt.Errorf("arb capital cap out of range")
			}
			bc.ArbPolicy.MaxCapitalBPS = v
		}
		if parameter == "min_profit_bps" {
			if v < 1 || v > 1000 {
				return fmt.Errorf("arb profit threshold out of range")
			}
			bc.ArbPolicy.MinProfitBPS = v
		}
	case "governance":
		if parameter != "guardian_config" && parameter != "slashing_council" {
			return fmt.Errorf("unsupported governance configuration")
		}
		var cfg struct {
			Members      []string `json:"members"`
			Threshold    int      `json:"threshold"`
			ExpiryHeight uint64   `json:"expiry_height"`
		}
		if err := json.Unmarshal([]byte(action.Value), &cfg); err != nil || len(cfg.Members) < 2 || cfg.Threshold < 2 || cfg.Threshold > len(cfg.Members) {
			return fmt.Errorf("invalid guardian multisig configuration")
		}
		members := map[string]bool{}
		for _, member := range cfg.Members {
			if !ValidateAddress(member) {
				return fmt.Errorf("invalid guardian address")
			}
			members[strings.ToLower(member)] = true
		}
		if parameter == "guardian_config" {
			bc.Governance.Guardians = members
			bc.Governance.GuardianThreshold = cfg.Threshold
			bc.Governance.GuardianExpiryBlock = cfg.ExpiryHeight
			bc.Governance.Guardian = ""
		} else {
			bc.Governance.SlashingCouncil = members
			bc.Governance.SlashingCouncilThreshold = cfg.Threshold
			bc.Governance.SlashingCouncilExpiryBlock = cfg.ExpiryHeight
		}
	case "bridge":
		switch parameter {
		case "risk_policy":
			var policy BridgeRiskPolicy
			if err := json.Unmarshal([]byte(action.Value), &policy); err != nil {
				return err
			}
			if err := policy.Validate(); err != nil {
				return err
			}
			oldLightClients, oldProofAuthorized := bc.BridgeSecurity.LightClients, bc.BridgeSecurity.ProofAuthorized
			bc.BridgeSecurity.Policy = policy
			bc.BridgeSecurity.ensure()
			bc.BridgeSecurity.LightClients, bc.BridgeSecurity.ProofAuthorized = oldLightClients, oldProofAuthorized
		case "light_client":
			var config BridgeLightClientConfig
			if err := json.Unmarshal([]byte(action.Value), &config); err != nil {
				return err
			}
			if err := configureBridgeLightClient(bc.BridgeSecurity, config); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported bridge action")
		}
	default:
		return fmt.Errorf("unsupported module")
	}
	return nil
}

func (bc *Blockchain_struct) ExecuteGovernanceProposal(proposalID string, currentHeight uint64) (*GovernanceProposal, error) {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	bc.EnsureRuntimeState()
	p := bc.Governance.Proposals[proposalID]
	if p == nil || p.Status != "timelocked" || currentHeight < p.ExecuteHeight {
		return nil, fmt.Errorf("proposal is not executable")
	}
	// Validate the full action set on a shallow policy copy before mutation.
	for _, action := range p.Actions {
		if err := validateGovernanceAction(action); err != nil {
			return nil, err
		}
	}
	oldPolicy := bc.EconomicPolicy
	oldPauses := map[string]bool{}
	for key, value := range bc.ProtocolPauses {
		oldPauses[key] = value
	}
	oldTimeout := bc.ConsensusV2.RoundTimeoutSeconds
	oldLiquidityCap := bc.ConsensusV2.MaxLiquidityCreditBPS
	oldPublishers := make(map[string]string, len(bc.OraclePublishers))
	for key, value := range bc.OraclePublishers {
		oldPublishers[key] = value
	}
	oldPairPolicies := make(map[string]PairRiskPolicy, len(bc.PairRiskPolicies))
	for key, value := range bc.PairRiskPolicies {
		oldPairPolicies[key] = value
	}
	oldArb := bc.ArbPolicy
	oldGuardians, oldThreshold, oldExpiry, oldLegacyGuardian := bc.Governance.Guardians, bc.Governance.GuardianThreshold, bc.Governance.GuardianExpiryBlock, bc.Governance.Guardian
	oldCouncil, oldCouncilThreshold, oldCouncilExpiry := bc.Governance.SlashingCouncil, bc.Governance.SlashingCouncilThreshold, bc.Governance.SlashingCouncilExpiryBlock
	bridgeRaw, _ := json.Marshal(bc.BridgeSecurity)
	rollback := func() {
		bc.EconomicPolicy, bc.ProtocolPauses = oldPolicy, oldPauses
		bc.ConsensusV2.RoundTimeoutSeconds, bc.ConsensusV2.MaxLiquidityCreditBPS = oldTimeout, oldLiquidityCap
		bc.OraclePublishers, bc.PairRiskPolicies = oldPublishers, oldPairPolicies
		bc.ArbPolicy = oldArb
		bc.Governance.Guardians, bc.Governance.GuardianThreshold, bc.Governance.GuardianExpiryBlock, bc.Governance.Guardian = oldGuardians, oldThreshold, oldExpiry, oldLegacyGuardian
		bc.Governance.SlashingCouncil, bc.Governance.SlashingCouncilThreshold, bc.Governance.SlashingCouncilExpiryBlock = oldCouncil, oldCouncilThreshold, oldCouncilExpiry
		var bridge BridgeSecurityState
		if json.Unmarshal(bridgeRaw, &bridge) == nil {
			bc.BridgeSecurity = &bridge
		}
	}
	for _, action := range p.Actions {
		if err := bc.applyGovernanceAction(action); err != nil {
			rollback()
			return nil, err
		}
	}
	if err := bc.EconomicPolicy.Validate(); err != nil {
		rollback()
		return nil, err
	}
	p.Status = "executed"
	bc.persistRuntimeStateLocked()
	return p, nil
}

// GuardianPause is intentionally limited to temporary safety stops. It cannot
// transfer funds, change code or unpause; recovery requires governance.
func (bc *Blockchain_struct) GuardianPause(guardian, module string, currentHeight uint64) error {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	bc.EnsureRuntimeState()
	module = strings.ToLower(strings.TrimSpace(module))
	if module != "bridge" && module != "vault" && module != "router" {
		return fmt.Errorf("guardian cannot pause this module")
	}
	if len(bc.Governance.Guardians) > 0 || bc.Governance.GuardianThreshold > 1 {
		return fmt.Errorf("legacy single-guardian path disabled; submit signed guardian_pause transactions")
	}
	if !strings.EqualFold(guardian, bc.Governance.Guardian) || bc.Governance.Guardian == "" || (bc.Governance.GuardianExpiryBlock > 0 && currentHeight > bc.Governance.GuardianExpiryBlock) {
		return fmt.Errorf("guardian authorization unavailable")
	}
	bc.ProtocolPauses[module] = true
	bc.persistRuntimeStateLocked()
	return nil
}

func (bc *Blockchain_struct) GovernanceStatus() map[string]interface{} {
	if bc == nil {
		return map[string]interface{}{}
	}
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	bc.EnsureRuntimeState()
	active, executed := 0, 0
	for _, p := range bc.Governance.Proposals {
		if p.Status == "executed" {
			executed++
		} else if p.Status == "voting" || p.Status == "timelocked" {
			active++
		}
	}
	return map[string]interface{}{"voting_period_blocks": bc.Governance.VotingPeriodBlocks, "timelock_blocks": bc.Governance.TimelockBlocks, "quorum_bps": bc.Governance.QuorumBPS, "approval_bps": bc.Governance.ApprovalBPS, "proposal_count": len(bc.Governance.Proposals), "active_proposals": active, "executed_proposals": executed, "pauses": bc.ProtocolPauses, "voting_basis": "epoch validator-power snapshot", "slashing_council_members": len(bc.Governance.SlashingCouncil), "slashing_council_threshold": bc.Governance.SlashingCouncilThreshold, "slashing_council_expiry_block": bc.Governance.SlashingCouncilExpiryBlock}
}
