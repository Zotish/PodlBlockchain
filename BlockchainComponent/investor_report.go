package blockchaincomponent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/crypto"
)

// InvestorEvidenceReport is a validator-attested, point-in-time snapshot. It
// is evidence of what one validator reported; it is deliberately not called an
// audit and does not turn unaudited protocol metrics into audited financials.
type InvestorEvidenceReport struct {
	Domain          string                 `json:"domain"`
	ReportVersion   uint32                 `json:"report_version"`
	ChainID         uint                   `json:"chain_id"`
	NetworkID       string                 `json:"network_id"`
	SpecHash        string                 `json:"spec_hash"`
	Height          uint64                 `json:"height"`
	LatestBlockHash string                 `json:"latest_block_hash"`
	StateRoot       string                 `json:"state_root"`
	ReportedAt      int64                  `json:"reported_at"`
	Metrics         map[string]interface{} `json:"metrics"`
	PayloadHash     string                 `json:"payload_hash"`
	Signer          string                 `json:"signer"`
	Signature       string                 `json:"signature"`
	Verified        bool                   `json:"verified"`
}

type investorEvidencePayload struct {
	Domain          string                 `json:"domain"`
	ReportVersion   uint32                 `json:"report_version"`
	ChainID         uint                   `json:"chain_id"`
	NetworkID       string                 `json:"network_id"`
	SpecHash        string                 `json:"spec_hash"`
	Height          uint64                 `json:"height"`
	LatestBlockHash string                 `json:"latest_block_hash"`
	StateRoot       string                 `json:"state_root"`
	ReportedAt      int64                  `json:"reported_at"`
	Metrics         map[string]interface{} `json:"metrics"`
}

func (r InvestorEvidenceReport) payload() investorEvidencePayload {
	return investorEvidencePayload{r.Domain, r.ReportVersion, r.ChainID, r.NetworkID, r.SpecHash, r.Height, r.LatestBlockHash, r.StateRoot, r.ReportedAt, r.Metrics}
}

func investorEvidenceDigest(payload investorEvidencePayload) ([]byte, string, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, "", err
	}
	digest := sha256.Sum256(raw)
	return raw, "0x" + hex.EncodeToString(digest[:]), nil
}

func (bc *Blockchain_struct) SignInvestorEvidenceReport(reportedAt int64) (InvestorEvidenceReport, error) {
	if bc == nil || bc.Network == nil || reportedAt <= 0 {
		return InvestorEvidenceReport{}, fmt.Errorf("validator signing identity and timestamp required")
	}
	signer, validatorSigner := bc.Network.ValidatorSignerSnapshot()
	if validatorSigner == nil || !strings.EqualFold(validatorSigner.Address(), signer) {
		return InvestorEvidenceReport{}, fmt.Errorf("validator signing identity mismatch")
	}
	height := bc.LatestBlockNumber()
	latestHash := ""
	bc.Mutex.Lock()
	if len(bc.Blocks) > 0 && bc.Blocks[len(bc.Blocks)-1] != nil {
		latestHash = bc.Blocks[len(bc.Blocks)-1].CurrentHash
	}
	bc.Mutex.Unlock()
	report := InvestorEvidenceReport{
		Domain: "PODL-INVESTOR-EVIDENCE-V1", ReportVersion: 1,
		ChainID: bc.ChainSpec.ChainID, NetworkID: bc.ChainSpec.NetworkID, SpecHash: bc.ChainSpec.Hash(),
		Height: height, LatestBlockHash: latestHash, StateRoot: bc.ComputeDeterministicStateRootAt(height),
		ReportedAt: reportedAt, Metrics: bc.InvestorMetrics(), Signer: strings.ToLower(signer),
	}
	raw, payloadHash, err := investorEvidenceDigest(report.payload())
	if err != nil {
		return InvestorEvidenceReport{}, err
	}
	signature, err := validatorSigner.SignMessage(context.Background(), SignerDomainInvestorEvidence, raw, "")
	if err != nil {
		return InvestorEvidenceReport{}, err
	}
	report.PayloadHash = payloadHash
	report.Signature = signature
	report.Verified = VerifyInvestorEvidenceReport(report)
	if !report.Verified {
		return InvestorEvidenceReport{}, fmt.Errorf("self-verification of investor evidence failed")
	}
	return report, nil
}

func VerifyInvestorEvidenceReport(report InvestorEvidenceReport) bool {
	if report.Domain != "PODL-INVESTOR-EVIDENCE-V1" || report.ReportVersion != 1 || report.ReportedAt <= 0 || !ValidateAddress(report.Signer) || strings.TrimSpace(report.Signature) == "" {
		return false
	}
	raw, payloadHash, err := investorEvidenceDigest(report.payload())
	if err != nil || !strings.EqualFold(payloadHash, report.PayloadHash) {
		return false
	}
	signature, err := hex.DecodeString(strings.TrimPrefix(report.Signature, "0x"))
	if err != nil || len(signature) != 65 {
		return false
	}
	publicKey, err := crypto.SigToPub(accounts.TextHash(raw), signature)
	return err == nil && strings.EqualFold(crypto.PubkeyToAddress(*publicKey).Hex(), report.Signer)
}
