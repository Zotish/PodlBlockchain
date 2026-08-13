package blockchaincomponent

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

func TestSignedInvestorEvidenceReportAndTamperRejection(t *testing.T) {
	bc := newTestBlockchain()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	privateKey := hex.EncodeToString(crypto.FromECDSA(key))
	address := crypto.PubkeyToAddress(key.PublicKey).Hex()
	bc.Network = NewNetworkService(bc)
	bc.Network.SetValidatorIdentity(address, privateKey)
	report, err := bc.SignInvestorEvidenceReport(1_700_000_000)
	if err != nil || !report.Verified || !VerifyInvestorEvidenceReport(report) || !strings.EqualFold(report.Signer, address) {
		t.Fatalf("signed evidence report failed: report=%+v err=%v", report, err)
	}
	report.Metrics["realized_protocol_revenue"] = "999999"
	if VerifyInvestorEvidenceReport(report) {
		t.Fatal("tampered evidence report retained a valid signature")
	}
}
