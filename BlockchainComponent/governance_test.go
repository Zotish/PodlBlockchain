package blockchaincomponent

import (
	"encoding/json"
	"testing"
)

func TestIndependentSlashingCouncilSignedThresholdDecision(t *testing.T) {
	previousPersist := persistRuntimeState
	persistRuntimeState = func(*Blockchain_struct) error { return nil }
	defer func() { persistRuntimeState = previousPersist }()
	validator := "0x1111111111111111111111111111111111111111"
	g1, g2, outsider := "0x2222222222222222222222222222222222222222", "0x3333333333333333333333333333333333333333", "0x4444444444444444444444444444444444444444"
	bc := &Blockchain_struct{Validators: []*Validator{{Address: validator, LPStakeAmount: 1000, NativeBond: 1000}}}
	bc.EnsureRuntimeState()
	bc.Governance.SlashingCouncil = map[string]bool{g1: true, g2: true}
	bc.Governance.SlashingCouncilThreshold = 2
	bc.Governance.SlashingCouncilExpiryBlock = 100
	slashingCase, err := bc.OpenSlashingCase(validator, "evidence", "double-sign", .10, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	apply := func(actor string) error {
		payload, _ := json.Marshal(GovernanceTxPayload{Operation: "slash_decide", CaseID: slashingCase.ID, Decision: "uphold", ReasonHash: "ipfs://decision"})
		return bc.applyGovernanceTransactionAt(&Transaction{From: actor, Type: "governance_action", ExtraData: payload, Timestamp: 1000}, 3)
	}
	if err = apply(outsider); err == nil {
		t.Fatal("non-council signer approved slashing")
	}
	if err = apply(g1); err != nil || slashingCase.Status == "upheld" {
		t.Fatalf("single council member crossed threshold: status=%s err=%v", slashingCase.Status, err)
	}
	if err = apply(g2); err != nil {
		t.Fatal(err)
	}
	if slashingCase.Status != "upheld" || slashingCase.CouncilDecision != "uphold" || slashingCase.ResolvedAt != 1000 || bc.Validators[0].PenaltyScore <= 0 {
		t.Fatalf("threshold council decision was not deterministic: case=%+v validator=%+v", slashingCase, bc.Validators[0])
	}
}

func TestGovernanceSnapshotTimelockAndExecution(t *testing.T) {
	previousPersist := persistRuntimeState
	persistRuntimeState = func(*Blockchain_struct) error { return nil }
	defer func() { persistRuntimeState = previousPersist }()
	addr := "0x1111111111111111111111111111111111111111"
	bc := &Blockchain_struct{Validators: []*Validator{{Address: addr, LPStakeAmount: 1000000, NativeBond: 1000000, LiquidityPower: 1000}}}
	bc.EnsureRuntimeState()
	bc.Governance.VotingPeriodBlocks = 2
	bc.Governance.TimelockBlocks = 2
	p, err := bc.CreateGovernanceProposal(addr, "Pause bridge", "0xdoc", []GovernanceAction{{Module: "safety", Parameter: "bridge_paused", Value: "true"}}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = bc.VoteGovernance(p.ID, addr, "for", 11); err != nil {
		t.Fatal(err)
	}
	if _, err = bc.QueueGovernanceProposal(p.ID, 13); err != nil {
		t.Fatal(err)
	}
	if _, err = bc.ExecuteGovernanceProposal(p.ID, 14); err == nil {
		t.Fatal("timelock should block early execution")
	}
	if _, err = bc.ExecuteGovernanceProposal(p.ID, 15); err != nil {
		t.Fatal(err)
	}
	if !bc.ProtocolPauses["bridge"] {
		t.Fatal("bridge pause action was not applied")
	}
}

func TestGovernanceDelegationSnapshotAndThresholdVeto(t *testing.T) {
	previousPersist := persistRuntimeState
	persistRuntimeState = func(*Blockchain_struct) error { return nil }
	defer func() { persistRuntimeState = previousPersist }()
	a, b, g1, g2 := "0x1111111111111111111111111111111111111111", "0x2222222222222222222222222222222222222222", "0x3333333333333333333333333333333333333333", "0x4444444444444444444444444444444444444444"
	bc := &Blockchain_struct{Validators: []*Validator{{Address: a, LPStakeAmount: 100, NativeBond: 100, LiquidityPower: 10}, {Address: b, LPStakeAmount: 100, NativeBond: 100, LiquidityPower: 10}}}
	bc.EnsureRuntimeState()
	bc.Governance.VotingPeriodBlocks, bc.Governance.TimelockBlocks = 1, 5
	bc.Governance.Guardians = map[string]bool{g1: true, g2: true}
	bc.Governance.GuardianThreshold = 2
	bc.Governance.GuardianExpiryBlock = 100
	apply := func(from string, payload GovernanceTxPayload, height uint64) {
		raw, _ := json.Marshal(payload)
		if err := bc.applyGovernanceTransactionAt(&Transaction{From: from, Type: "governance_action", ExtraData: raw}, height); err != nil {
			t.Fatal(err)
		}
	}
	apply(a, GovernanceTxPayload{Operation: "delegate", Delegatee: b}, 1)
	p, err := bc.CreateGovernanceProposal(b, "delegated proposal", "doc", []GovernanceAction{{Module: "safety", Parameter: "router_paused", Value: "true"}}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if p.SnapshotPower[normalizeAccountAddress(b)] <= p.SnapshotPower[normalizeAccountAddress(a)] || p.SnapshotPower[normalizeAccountAddress(a)] != 0 {
		t.Fatalf("delegation not reflected prospectively: %+v", p.SnapshotPower)
	}
	if _, err = bc.VoteGovernance(p.ID, b, "for", 2); err != nil {
		t.Fatal(err)
	}
	if _, err = bc.QueueGovernanceProposal(p.ID, 4); err != nil {
		t.Fatal(err)
	}
	apply(g1, GovernanceTxPayload{Operation: "guardian_veto", ProposalID: p.ID, ReasonHash: "reason-1"}, 5)
	if bc.Governance.Proposals[p.ID].Status != "timelocked" {
		t.Fatal("one guardian bypassed veto threshold")
	}
	apply(g2, GovernanceTxPayload{Operation: "guardian_veto", ProposalID: p.ID, ReasonHash: "reason-2"}, 6)
	if bc.Governance.Proposals[p.ID].Status != "vetoed" {
		t.Fatal("threshold veto did not stop timelocked proposal")
	}
	apply(a, GovernanceTxPayload{Operation: "undelegate"}, 7)
	if bc.Governance.Delegations[normalizeAccountAddress(a)] != "" {
		t.Fatal("undelegation did not clear prospective delegation")
	}
}
