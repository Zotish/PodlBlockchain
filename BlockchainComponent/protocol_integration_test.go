package blockchaincomponent

import (
	"math/big"
	"strings"
	"testing"
	"time"
)

func TestSignedGovernanceCreateAndGuardianThreshold(t *testing.T) {
	bc := newTestBlockchain()
	createTx, proposer := signedControlTx(t, "governance_action", GovernanceTxPayload{Operation: "create", Title: "Raise reserve", DescriptionHash: "0xproposal", Actions: []GovernanceAction{{Module: "economics", Parameter: "buyback_enabled", Value: "false"}}})
	bc.Validators = append(bc.Validators, &Validator{Address: proposer, NativeBond: 1e12, LPStakeAmount: 1e12, LockTime: time.Now().Add(time.Hour)})
	bc.EnsureRuntimeState()
	bc.ConsensusV2.ActiveSet = bc.buildValidatorPowerSet()
	if !bc.VerifyTransactionSignature(createTx) {
		t.Fatal("governance control signature rejected")
	}
	if err := bc.applyGovernanceTransactionAt(createTx, 1); err != nil {
		t.Fatal(err)
	}
	if len(bc.Governance.Proposals) != 1 || len(bc.Governance.AuditTrail) != 1 {
		t.Fatal("signed governance action was not audited")
	}

	_, guardianA := signedControlTx(t, "governance_action", GovernanceTxPayload{})
	_, guardianB := signedControlTx(t, "governance_action", GovernanceTxPayload{})
	bc.Governance.Guardians = map[string]bool{lower(guardianA): true, lower(guardianB): true}
	bc.Governance.GuardianThreshold = 2
	payload := GovernanceTxPayload{Operation: "guardian_pause", Module: "bridge", ActionID: "incident-1", ExpiresAtHeight: 100}
	txA, _ := signedControlTxForAddress(t, guardianA, payload)
	txB, _ := signedControlTxForAddress(t, guardianB, payload)
	if err := bc.applyGovernanceTransactionAt(txA, 2); err != nil || bc.ProtocolPauses["bridge"] {
		t.Fatalf("first guardian incorrectly paused bridge: %v", err)
	}
	if err := bc.applyGovernanceTransactionAt(txB, 2); err != nil || !bc.ProtocolPauses["bridge"] {
		t.Fatalf("guardian threshold failed: %v", err)
	}
}

// signedControlTxForAddress uses the deterministic test key registered by
// signedControlTxAddress. It is split out to make multisig identity explicit.
var controlTestKeys = map[string]string{}

func lower(v string) string {
	return strings.ToLower(v)
}

func signedControlTxForAddress(t *testing.T, address string, payload any) (*Transaction, string) {
	t.Helper()
	// This helper is replaced in init by the key-returning helper below; an
	// absent key indicates a test construction error.
	keyHex := controlTestKeys[lower(address)]
	if keyHex == "" {
		t.Fatalf("test key unavailable for %s", address)
	}
	return signControlWithKey(t, keyHex, "governance_action", payload)
}

func TestDEXFeeReconciliationRequiresBuiltinPairAndIsIdempotent(t *testing.T) {
	bc := newTestBlockchain()
	db := NewOverlayContractDB(nil)
	pair := "0x1111111111111111111111111111111111111111"
	if err := db.SaveContractMetadata(pair, &ContractMetadata{Address: pair, Type: "builtin", BuiltinName: "dex_pair"}); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveStorage(pair, "protocol_fee_total:lqd", "1000"); err != nil {
		t.Fatal(err)
	}
	bc.ContractEngine = &LQDContractEngine{DB: db}
	if err := bc.ReconcileDEXProtocolFees(10, 1000); err != nil {
		t.Fatal(err)
	}
	if len(bc.ProtocolRevenue) != 1 || bc.ProtocolRevenue[0].Amount.Cmp(big.NewInt(1000)) != 0 {
		t.Fatal("custodied DEX revenue not reconciled")
	}
	if err := bc.ReconcileDEXProtocolFees(10, 1000); err != nil || len(bc.ProtocolRevenue) != 1 {
		t.Fatal("DEX revenue reconciliation is not idempotent")
	}
}

func TestValidatorBondSlashingRevenueReconciliationIsIdempotent(t *testing.T) {
	bc := newTestBlockchain()
	db := NewOverlayContractDB(nil)
	bond := "0x4444444444444444444444444444444444444444"
	if err := db.SaveContractMetadata(bond, &ContractMetadata{Address: bond, Type: "builtin", BuiltinName: "validator_bond"}); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveStorage(bond, "protocol_slash_total", "2500"); err != nil {
		t.Fatal(err)
	}
	bc.ContractEngine = &LQDContractEngine{DB: db}
	if err := bc.ReconcileDEXProtocolFees(11, 1000); err != nil {
		t.Fatal(err)
	}
	if len(bc.ProtocolRevenue) != 1 || bc.ProtocolRevenue[0].Source != "slashing" || bc.ProtocolRevenue[0].Amount.Cmp(big.NewInt(2500)) != 0 {
		t.Fatal("validator bond slashing proceeds not reconciled")
	}
	if err := bc.ReconcileDEXProtocolFees(12, 1001); err != nil || len(bc.ProtocolRevenue) != 1 {
		t.Fatal("validator bond slashing reconciliation is not idempotent")
	}
}

func TestSlashingAppealAndGovernanceDismissal(t *testing.T) {
	bc := newTestBlockchain()
	validator := "0x1111111111111111111111111111111111111111"
	bc.Validators = append(bc.Validators, &Validator{Address: validator, NativeBond: 1e12, LPStakeAmount: 1e12, LockTime: time.Now().Add(time.Hour)})
	caseRow, err := bc.OpenSlashingCase(validator, "0xevidence", "double sign", .10, 10, 20)
	if err != nil {
		t.Fatal(err)
	}
	if err := bc.AppealSlashingCase(caseRow.ID, validator, "0xappeal", 15); err != nil {
		t.Fatal(err)
	}
	bc.Governance.Proposals["gov-resolution"] = &GovernanceProposal{ID: "gov-resolution", Status: "executed"}
	if err := bc.ResolveSlashingCase(caseRow.ID, "gov-resolution", false, 16); err != nil {
		t.Fatal(err)
	}
	if bc.SlashingCases[caseRow.ID].Status != "dismissed" {
		t.Fatal("slashing appeal was not adjudicated")
	}
}
