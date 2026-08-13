package blockchaincomponent

import (
	"math/big"
	"testing"
)

func TestRetailSuitabilityRejectsUndisclosedPrincipalRisk(t *testing.T) {
	got := EvaluateRetailSuitability(SuitabilityAnswers{LossToleranceBPS: 5000, InvestmentHorizonDays: 1000, DeFiExperienceYears: 5, AcceptsSmartContract: true})
	if got.Eligible || got.RecommendedProfile != nil {
		t.Fatal("user who rejects impermanent loss must not receive a profile")
	}
}

func TestRetailSuitabilitySelectsGrowthOnlyWithStrongAnswers(t *testing.T) {
	got := EvaluateRetailSuitability(SuitabilityAnswers{LossToleranceBPS: 3500, InvestmentHorizonDays: 500, LiquidityNeedDays: 14, DeFiExperienceYears: 3, AcceptsImpermanentLoss: true, AcceptsSmartContract: true})
	if !got.Eligible || got.RecommendedProfile == nil || got.RecommendedProfile.ID != "growth" {
		t.Fatalf("expected growth profile: %+v", got)
	}
}

func TestLiquidityServiceInvoiceIsGovernedCustodiedAndIdempotent(t *testing.T) {
	previousPersist := persistRuntimeState
	persistRuntimeState = func(*Blockchain_struct) error { return nil }
	defer func() { persistRuntimeState = previousPersist }()

	chain := newTestBlockchain()
	payer := "0x1111111111111111111111111111111111111111"
	chain.setAccountBalance(payer, big.NewInt(1_000_000))
	agreement, err := chain.CreateLiquidityServiceAgreement(payer, []string{"LQD/USDC"}, big.NewInt(500_000), 100, 1000, true)
	if err != nil {
		t.Fatal(err)
	}
	chain.Governance.Proposals["gov-laas"] = &GovernanceProposal{ID: "gov-laas", Status: "executed"}
	if err := chain.ActivateLiquidityServiceAgreement(agreement.ID, "gov-laas"); err != nil {
		t.Fatal(err)
	}
	entry, err := chain.SettleLiquidityServiceInvoice(agreement.ID, payer, big.NewInt(100_000), big.NewInt(10_000), "invoice-1", 100)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Amount.Cmp(big.NewInt(2_000)) != 0 || chain.AccountBalanceAmount(payer).Cmp(big.NewInt(998_000)) != 0 || chain.AccountBalanceAmount(ProtocolRevenueEscrowAddress).Cmp(big.NewInt(2_000)) != 0 {
		t.Fatalf("unexpected custody-backed invoice result: entry=%s payer=%s escrow=%s", entry.Amount, chain.AccountBalanceAmount(payer), chain.AccountBalanceAmount(ProtocolRevenueEscrowAddress))
	}
	if _, err := chain.SettleLiquidityServiceInvoice(agreement.ID, payer, big.NewInt(100_000), big.NewInt(10_000), "invoice-1", 999); err != nil {
		t.Fatal(err)
	}
	if chain.AccountBalanceAmount(payer).Cmp(big.NewInt(998_000)) != 0 || chain.BusinessAgreements[agreement.ID].FeesCollected.Cmp(big.NewInt(2_000)) != 0 || len(chain.ProtocolRevenue) != 1 {
		t.Fatal("duplicate invoice debited or accounted twice")
	}
}

func TestBridgeFeeCaptureRequiresRequestAndIsIdempotent(t *testing.T) {
	previousPersist := persistRuntimeState
	persistRuntimeState = func(*Blockchain_struct) error { return nil }
	defer func() { persistRuntimeState = previousPersist }()

	chain := newTestBlockchain()
	payer := "0x2222222222222222222222222222222222222222"
	chain.setAccountBalance(payer, big.NewInt(100_000))
	chain.EnsureRuntimeState()
	chain.BridgeRequests["bridge-1"] = &BridgeRequest{ID: "bridge-1", From: payer, To: "0x3333333333333333333333333333333333333333", Status: BridgeStatusLocked}
	if _, err := chain.CaptureBridgeFee("missing", payer, big.NewInt(500), "fee-tx", 100); err == nil {
		t.Fatal("missing bridge request accepted")
	}
	if _, err := chain.CaptureBridgeFee("bridge-1", payer, big.NewInt(500), "fee-tx", 100); err != nil {
		t.Fatal(err)
	}
	if _, err := chain.CaptureBridgeFee("bridge-1", payer, big.NewInt(500), "fee-tx", 200); err != nil {
		t.Fatal(err)
	}
	if chain.AccountBalanceAmount(payer).Cmp(big.NewInt(99_500)) != 0 || len(chain.ProtocolRevenue) != 1 || chain.ProtocolRevenue[0].Source != "bridge_fee" {
		t.Fatal("bridge fee custody/idempotency failed")
	}
}
