package blockchaincomponent

import (
	"encoding/json"
	"fmt"
	"strings"
)

type GovernanceTxPayload struct {
	Operation       string             `json:"operation"`
	ProposalID      string             `json:"proposal_id,omitempty"`
	Title           string             `json:"title,omitempty"`
	DescriptionHash string             `json:"description_hash,omitempty"`
	Actions         []GovernanceAction `json:"actions,omitempty"`
	Choice          string             `json:"choice,omitempty"`
	ReasonHash      string             `json:"reason_hash,omitempty"`
	Module          string             `json:"module,omitempty"`
	ActionID        string             `json:"action_id,omitempty"`
	ExpiresAtHeight uint64             `json:"expires_at_height,omitempty"`
	Delegatee       string             `json:"delegatee,omitempty"`
	CaseID          string             `json:"case_id,omitempty"`
	Decision        string             `json:"decision,omitempty"`
}

type governanceExecSnapshot struct {
	Policy           EconomicPolicy            `json:"policy"`
	Pauses           map[string]bool           `json:"pauses"`
	Publishers       map[string]string         `json:"publishers"`
	Pairs            map[string]PairRiskPolicy `json:"pairs"`
	Arb              ProtocolArbPolicy         `json:"arb"`
	Guardians        map[string]bool           `json:"guardians"`
	Threshold        int                       `json:"threshold"`
	Expiry           uint64                    `json:"expiry"`
	Bridge           *BridgeSecurityState      `json:"bridge"`
	Timeout          int64                     `json:"timeout"`
	Liquidity        uint32                    `json:"liquidity"`
	Council          map[string]bool           `json:"slashing_council"`
	CouncilThreshold int                       `json:"slashing_council_threshold"`
	CouncilExpiry    uint64                    `json:"slashing_council_expiry"`
}

func DecodeGovernanceTransaction(tx *Transaction) (GovernanceTxPayload, error) {
	var p GovernanceTxPayload
	if tx == nil || tx.Type != "governance_action" {
		return p, fmt.Errorf("governance_action transaction required")
	}
	if err := json.Unmarshal(tx.ExtraData, &p); err != nil {
		return p, err
	}
	p.Operation = strings.ToLower(strings.TrimSpace(p.Operation))
	switch p.Operation {
	case "create", "vote", "queue", "execute", "cancel", "guardian_pause", "delegate", "undelegate", "guardian_veto", "slash_decide":
	default:
		return p, fmt.Errorf("unsupported governance operation")
	}
	return p, nil
}

func ValidateGovernanceTransaction(tx *Transaction) error {
	p, err := DecodeGovernanceTransaction(tx)
	if err != nil {
		return err
	}
	if !ValidateAddress(tx.From) {
		return fmt.Errorf("signed governance actor required")
	}
	if p.Operation == "create" && (strings.TrimSpace(p.Title) == "" || len(p.Actions) == 0 || len(p.Actions) > 16) {
		return fmt.Errorf("invalid proposal payload")
	}
	if p.Operation != "create" && p.Operation != "guardian_pause" && p.Operation != "delegate" && p.Operation != "undelegate" && p.Operation != "slash_decide" && strings.TrimSpace(p.ProposalID) == "" {
		return fmt.Errorf("proposal id required")
	}
	if p.Operation == "vote" && p.Choice != "for" && p.Choice != "against" && p.Choice != "abstain" {
		return fmt.Errorf("invalid governance vote")
	}
	if p.Operation == "slash_decide" && (strings.TrimSpace(p.CaseID) == "" || (strings.ToLower(p.Decision) != "uphold" && strings.ToLower(p.Decision) != "dismiss") || strings.TrimSpace(p.ReasonHash) == "") {
		return fmt.Errorf("case, uphold/dismiss decision and reason hash required")
	}
	return nil
}

func (bc *Blockchain_struct) auditGovernance(height uint64, operation, actor, proposal, detail string) {
	bc.Governance.AuditTrail = append(bc.Governance.AuditTrail, GovernanceAuditEvent{Height: height, Operation: operation, Actor: strings.ToLower(actor), ProposalID: proposal, Detail: detail})
	if len(bc.Governance.AuditTrail) > 10000 {
		bc.Governance.AuditTrail = bc.Governance.AuditTrail[len(bc.Governance.AuditTrail)-10000:]
	}
}

func (bc *Blockchain_struct) applyGovernanceTransactionAt(tx *Transaction, height uint64) error {
	if err := ValidateGovernanceTransaction(tx); err != nil {
		return err
	}
	p, _ := DecodeGovernanceTransaction(tx)
	actor := strings.ToLower(tx.From)
	bc.EnsureRuntimeState()
	switch p.Operation {
	case "delegate":
		delegatee := strings.ToLower(strings.TrimSpace(p.Delegatee))
		if !ValidateAddress(delegatee) || delegatee == actor {
			return fmt.Errorf("valid distinct delegatee required")
		}
		// Delegation is intentionally prospective: existing proposal snapshots
		// never change. Reject cycles so voting power has one deterministic sink.
		cursor := delegatee
		for i := 0; i < 64; i++ {
			if cursor == actor {
				return fmt.Errorf("delegation cycle")
			}
			next := strings.ToLower(strings.TrimSpace(bc.Governance.Delegations[cursor]))
			if next == "" {
				break
			}
			cursor = next
			if i == 63 {
				return fmt.Errorf("delegation chain too deep")
			}
		}
		bc.Governance.Delegations[actor] = delegatee
		bc.auditGovernance(height, "delegate", actor, "", delegatee)
	case "undelegate":
		delete(bc.Governance.Delegations, actor)
		bc.auditGovernance(height, "undelegate", actor, "", "")
	case "slash_decide":
		if err := bc.approveSlashingCaseCouncilLocked(p.CaseID, actor, p.Decision, p.ReasonHash, height, int64(tx.Timestamp)); err != nil {
			return err
		}
		bc.auditGovernance(height, "slash_decide", actor, p.CaseID, strings.ToLower(p.Decision)+":"+p.ReasonHash)
	case "create":
		for _, a := range p.Actions {
			if err := validateGovernanceAction(a); err != nil {
				return err
			}
		}
		snapshot := bc.governancePowerSnapshot()
		if snapshot[actor] <= 0 {
			return fmt.Errorf("proposer has no bonded power")
		}
		bc.Governance.Nonce++
		id := governanceProposalID(actor, p.Title, bc.Governance.Nonce)
		bc.Governance.Proposals[id] = &GovernanceProposal{ID: id, Proposer: actor, Title: strings.TrimSpace(p.Title), DescriptionHash: p.DescriptionHash, Actions: append([]GovernanceAction(nil), p.Actions...), StartHeight: height, EndHeight: height + bc.Governance.VotingPeriodBlocks, Status: "voting", SnapshotPower: snapshot, Votes: map[string]GovernanceVote{}, VetoApprovals: map[string]bool{}}
		bc.auditGovernance(height, "create", actor, id, p.DescriptionHash)
	case "vote":
		proposal := bc.Governance.Proposals[p.ProposalID]
		if proposal == nil || proposal.Status != "voting" || height > proposal.EndHeight {
			return fmt.Errorf("proposal not open")
		}
		power := proposal.SnapshotPower[actor]
		if power <= 0 {
			return fmt.Errorf("voter has no snapshot power")
		}
		if _, ok := proposal.Votes[actor]; ok {
			return fmt.Errorf("duplicate vote")
		}
		proposal.Votes[actor] = GovernanceVote{Voter: actor, Choice: p.Choice, Power: power}
		switch p.Choice {
		case "for":
			proposal.ForPower += power
		case "against":
			proposal.AgainstPower += power
		default:
			proposal.AbstainPower += power
		}
		bc.auditGovernance(height, "vote", actor, p.ProposalID, p.Choice)
	case "queue":
		proposal := bc.Governance.Proposals[p.ProposalID]
		if proposal == nil || proposal.Status != "voting" || height <= proposal.EndHeight {
			return fmt.Errorf("voting not complete")
		}
		total := 0.0
		for _, power := range proposal.SnapshotPower {
			total += power
		}
		participating := proposal.ForPower + proposal.AgainstPower + proposal.AbstainPower
		denom := proposal.ForPower + proposal.AgainstPower
		if total <= 0 || participating*10000 < total*float64(bc.Governance.QuorumBPS) || denom <= 0 || proposal.ForPower*10000 < denom*float64(bc.Governance.ApprovalBPS) {
			proposal.Status = "rejected"
		} else {
			proposal.Status = "timelocked"
			proposal.ExecuteHeight = height + bc.Governance.TimelockBlocks
		}
		bc.auditGovernance(height, "queue", actor, p.ProposalID, proposal.Status)
	case "execute":
		proposal := bc.Governance.Proposals[p.ProposalID]
		if proposal == nil || proposal.Status != "timelocked" || height < proposal.ExecuteHeight {
			return fmt.Errorf("proposal not executable")
		}
		raw, _ := json.Marshal(governanceExecSnapshot{
			Policy: bc.EconomicPolicy, Pauses: bc.ProtocolPauses, Publishers: bc.OraclePublishers,
			Pairs: bc.PairRiskPolicies, Arb: bc.ArbPolicy, Guardians: bc.Governance.Guardians,
			Threshold: bc.Governance.GuardianThreshold, Expiry: bc.Governance.GuardianExpiryBlock,
			Bridge: bc.BridgeSecurity, Timeout: bc.ConsensusV2.RoundTimeoutSeconds,
			Liquidity: bc.ConsensusV2.MaxLiquidityCreditBPS,
			Council:   bc.Governance.SlashingCouncil, CouncilThreshold: bc.Governance.SlashingCouncilThreshold, CouncilExpiry: bc.Governance.SlashingCouncilExpiryBlock,
		})
		rollback := func() {
			var restored governanceExecSnapshot
			_ = json.Unmarshal(raw, &restored)
			bc.EconomicPolicy, bc.ProtocolPauses, bc.OraclePublishers, bc.PairRiskPolicies, bc.ArbPolicy = restored.Policy, restored.Pauses, restored.Publishers, restored.Pairs, restored.Arb
			bc.Governance.Guardians, bc.Governance.GuardianThreshold, bc.Governance.GuardianExpiryBlock = restored.Guardians, restored.Threshold, restored.Expiry
			bc.Governance.SlashingCouncil, bc.Governance.SlashingCouncilThreshold, bc.Governance.SlashingCouncilExpiryBlock = restored.Council, restored.CouncilThreshold, restored.CouncilExpiry
			bc.BridgeSecurity = restored.Bridge
			bc.ConsensusV2.RoundTimeoutSeconds, bc.ConsensusV2.MaxLiquidityCreditBPS = restored.Timeout, restored.Liquidity
		}
		for _, action := range proposal.Actions {
			if err := bc.applyGovernanceAction(action); err != nil {
				rollback()
				return fmt.Errorf("typed action failed before commit: %w", err)
			}
		}
		if err := bc.EconomicPolicy.Validate(); err != nil {
			rollback()
			return err
		}
		proposal.Status = "executed"
		bc.auditGovernance(height, "execute", actor, p.ProposalID, "typed-actions")
	case "cancel":
		proposal := bc.Governance.Proposals[p.ProposalID]
		if proposal == nil || (proposal.Status != "voting" && proposal.Status != "timelocked") {
			return fmt.Errorf("proposal not cancellable")
		}
		if actor != proposal.Proposer {
			return fmt.Errorf("only proposer may cancel before execution")
		}
		if proposal.ForPower+proposal.AgainstPower+proposal.AbstainPower > 0 || strings.TrimSpace(p.ReasonHash) == "" {
			return fmt.Errorf("voted proposal requires governance resolution")
		}
		proposal.Status = "cancelled"
		bc.auditGovernance(height, "cancel", actor, p.ProposalID, p.ReasonHash)
	case "guardian_pause":
		module := strings.ToLower(strings.TrimSpace(p.Module))
		if module != "bridge" && module != "vault" && module != "router" {
			return fmt.Errorf("guardian module not pausable")
		}
		if !bc.Governance.Guardians[actor] || bc.Governance.GuardianExpiryBlock > 0 && height > bc.Governance.GuardianExpiryBlock {
			return fmt.Errorf("guardian authorization expired")
		}
		if p.ActionID == "" || p.ExpiresAtHeight <= height {
			return fmt.Errorf("guardian action id and expiry required")
		}
		action := bc.Governance.GuardianActions[p.ActionID]
		if action == nil {
			action = &GuardianAction{ID: p.ActionID, Module: module, ExpiresAtHeight: p.ExpiresAtHeight, Approvals: map[string]bool{}}
			bc.Governance.GuardianActions[p.ActionID] = action
		}
		if action.Module != module || action.ExpiresAtHeight != p.ExpiresAtHeight || action.Executed {
			return fmt.Errorf("guardian action mismatch")
		}
		action.Approvals[actor] = true
		if len(action.Approvals) >= bc.Governance.GuardianThreshold {
			bc.ProtocolPauses[module] = true
			action.Executed = true
		}
		bc.auditGovernance(height, "guardian_pause", actor, p.ActionID, module)
	case "guardian_veto":
		proposal := bc.Governance.Proposals[p.ProposalID]
		if proposal == nil || proposal.Status != "timelocked" || height >= proposal.ExecuteHeight {
			return fmt.Errorf("proposal is outside guardian veto window")
		}
		if !bc.Governance.Guardians[actor] || bc.Governance.GuardianExpiryBlock > 0 && height > bc.Governance.GuardianExpiryBlock {
			return fmt.Errorf("guardian authorization expired")
		}
		if strings.TrimSpace(p.ReasonHash) == "" {
			return fmt.Errorf("public veto reason hash required")
		}
		if proposal.VetoApprovals == nil {
			proposal.VetoApprovals = map[string]bool{}
		}
		proposal.VetoApprovals[actor] = true
		if len(proposal.VetoApprovals) >= bc.Governance.GuardianThreshold {
			proposal.Status = "vetoed"
		}
		bc.auditGovernance(height, "guardian_veto", actor, p.ProposalID, p.ReasonHash)
	}
	return nil
}

func (bc *Blockchain_struct) CancelGovernanceProposal(id, caller, reason string, height uint64) error {
	payload := GovernanceTxPayload{Operation: "cancel", ProposalID: id, ReasonHash: reason}
	raw, _ := json.Marshal(payload)
	tx := &Transaction{From: caller, Type: "governance_action", ExtraData: raw}
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	if err := bc.applyGovernanceTransactionAt(tx, height); err != nil {
		return err
	}
	bc.persistRuntimeStateLocked()
	return nil
}
