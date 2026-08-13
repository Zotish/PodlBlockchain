package blockchaincomponent

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"
)

const (
	ProtocolRevenueEscrowAddress = "0x0000000000000000000000000000000000000e01"
	ProtocolBurnAddress          = "0x000000000000000000000000000000000000dead"
)

type TreasuryDeployment struct {
	ID           string   `json:"id"`
	Target       string   `json:"target"`
	Amount       *big.Int `json:"amount"`
	GovernanceID string   `json:"governance_id"`
	PurposeHash  string   `json:"purpose_hash"`
	Timestamp    int64    `json:"timestamp"`
	ProtocolOnly bool     `json:"protocol_capital_only"`
}

// CaptureProtocolRevenue is the custody-backed entry point. It first moves
// native LQD into the protocol escrow and only then records the waterfall.
func (bc *Blockchain_struct) CaptureProtocolRevenue(source, payer string, amount *big.Int, reference string, timestamp int64) (ProtocolRevenueEntry, error) {
	if bc == nil || !ValidateAddress(payer) || amount == nil || amount.Sign() <= 0 {
		return ProtocolRevenueEntry{}, fmt.Errorf("valid payer and positive amount required")
	}
	if timestamp <= 0 {
		timestamp = time.Now().Unix()
	}
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	bc.EnsureRuntimeState()
	id := revenueID(source, reference, amount)
	for _, existing := range bc.ProtocolRevenue {
		if existing.ID == id {
			return existing, nil
		}
	}
	if !bc.subAccountBalance(payer, amount) {
		return ProtocolRevenueEntry{}, fmt.Errorf("payer lacks captured revenue")
	}
	bc.addAccountBalance(ProtocolRevenueEscrowAddress, amount)
	entry, err := bc.recordProtocolRevenueConsensus(strings.ToLower(strings.TrimSpace(source)), amount, reference, timestamp)
	if err != nil {
		_ = bc.subAccountBalance(ProtocolRevenueEscrowAddress, amount)
		bc.addAccountBalance(payer, amount)
		return ProtocolRevenueEntry{}, err
	}
	bc.persistRuntimeStateLocked()
	return entry, nil
}

// CaptureBridgeFee binds a custody-backed fee to an existing bridge request.
// A repeated settlement reference is idempotent and cannot debit the payer
// twice. Relayers should call this before external execution.
func (bc *Blockchain_struct) CaptureBridgeFee(requestID, payer string, amount *big.Int, settlementReference string, timestamp int64) (ProtocolRevenueEntry, error) {
	if bc == nil || strings.TrimSpace(settlementReference) == "" {
		return ProtocolRevenueEntry{}, fmt.Errorf("bridge request and settlement reference required")
	}
	bc.Mutex.Lock()
	bc.EnsureRuntimeState()
	req := bc.BridgeRequests[strings.ToLower(strings.TrimSpace(requestID))]
	valid := req != nil && (strings.EqualFold(req.From, payer) || strings.EqualFold(req.To, payer)) && req.Status != BridgeStatusFailed
	bc.Mutex.Unlock()
	if !valid {
		return ProtocolRevenueEntry{}, fmt.Errorf("active bridge request owned by payer required")
	}
	reference := "bridge:" + strings.ToLower(strings.TrimSpace(requestID)) + ":" + strings.TrimSpace(settlementReference)
	return bc.CaptureProtocolRevenue("bridge_fee", payer, amount, reference, timestamp)
}

func (bc *Blockchain_struct) RealizedRevenueAPYBPS(vaultNAV *big.Int, windowDays int64, now int64) int64 {
	if bc == nil || vaultNAV == nil || vaultNAV.Sign() <= 0 || windowDays <= 0 {
		return 0
	}
	if now <= 0 {
		now = time.Now().Unix()
	}
	cutoff, revenue := now-windowDays*86400, big.NewInt(0)
	for _, entry := range bc.ProtocolRevenue {
		if entry.Timestamp < cutoff {
			continue
		}
		allocated := NewAmountFromStringOrZero(entry.Allocations[EconomicLPYield])
		revenue.Add(revenue, allocated)
	}
	annualized := new(big.Int).Mul(revenue, big.NewInt(36500))
	annualized.Div(annualized, big.NewInt(windowDays))
	annualized.Div(annualized, vaultNAV)
	if !annualized.IsInt64() {
		return 0
	}
	return annualized.Int64()
}

func (bc *Blockchain_struct) ExecuteVerifiedBuybackBurn(amount *big.Int, reference string) error {
	if bc == nil || amount == nil || amount.Sign() <= 0 || strings.TrimSpace(reference) == "" {
		return fmt.Errorf("positive burn and public reference required")
	}
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	bc.EnsureRuntimeState()
	if !bc.EconomicPolicy.BuybackEnabled || bc.economicBalance(EconomicBuyback).Cmp(amount) < 0 {
		return fmt.Errorf("guarded buyback bucket is not executable")
	}
	minimum := NewAmountFromStringOrZero(bc.EconomicPolicy.MinInsuranceBalance)
	if bc.economicBalance(EconomicInsurance).Cmp(minimum) < 0 {
		return fmt.Errorf("insurance floor is not met")
	}
	if !bc.subAccountBalance(ProtocolRevenueEscrowAddress, amount) {
		return fmt.Errorf("custodied revenue escrow is insufficient")
	}
	bc.addAccountBalance(ProtocolBurnAddress, amount)
	bc.economicBalance(EconomicBuyback).Sub(bc.economicBalance(EconomicBuyback), amount)
	bc.TotalBurned.Add(bc.TotalBurned, amount)
	bc.RewardLedger = append(bc.RewardLedger, RewardLedgerEntry{ID: "burn:" + reference, Timestamp: time.Now().Unix(), Address: ProtocolBurnAddress, Bucket: "buyback_burn", Source: "verified_surplus", Amount: amount.String(), Status: "burned", BalanceAfter: bc.AccountBalanceString(ProtocolBurnAddress)})
	bc.persistRuntimeStateLocked()
	return nil
}

func (bc *Blockchain_struct) DeployProtocolCapital(target string, amount *big.Int, governanceID, purposeHash string) (TreasuryDeployment, error) {
	if bc == nil || !ValidateAddress(target) || amount == nil || amount.Sign() <= 0 || strings.TrimSpace(purposeHash) == "" {
		return TreasuryDeployment{}, fmt.Errorf("invalid deployment")
	}
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	bc.EnsureRuntimeState()
	proposal := bc.Governance.Proposals[governanceID]
	if proposal == nil || proposal.Status != "executed" {
		return TreasuryDeployment{}, fmt.Errorf("executed governance approval required")
	}
	operations := bc.economicBalance(EconomicOperations)
	cap := new(big.Int).Div(new(big.Int).Mul(operations, big.NewInt(bc.EconomicPolicy.TreasuryDeployCapBPS)), big.NewInt(10000))
	if amount.Cmp(cap) > 0 || operations.Cmp(amount) < 0 || !bc.subAccountBalance(ProtocolRevenueEscrowAddress, amount) {
		return TreasuryDeployment{}, fmt.Errorf("treasury deployment cap or custody balance exceeded")
	}
	bc.addAccountBalance(target, amount)
	operations.Sub(operations, amount)
	sum := sha256.Sum256([]byte(strings.ToLower(target) + "|" + amount.String() + "|" + governanceID + "|" + purposeHash))
	row := TreasuryDeployment{ID: "deploy_" + hex.EncodeToString(sum[:12]), Target: strings.ToLower(target), Amount: CopyAmount(amount), GovernanceID: governanceID, PurposeHash: purposeHash, Timestamp: time.Now().Unix(), ProtocolOnly: true}
	bc.TreasuryDeployments = append(bc.TreasuryDeployments, row)
	bc.persistRuntimeStateLocked()
	return row, nil
}

type SupplyProjection struct {
	Years                 int    `json:"years"`
	Blocks                uint64 `json:"blocks"`
	NewIssuance           string `json:"new_issuance"`
	ScheduledIssuance     string `json:"scheduled_issuance_before_cap"`
	RemainingCap          string `json:"remaining_cap"`
	Cap                   string `json:"issuance_cap"`
	CapExceeded           bool   `json:"cap_exceeded"`
	RevenueReplacementBPS int64  `json:"revenue_replacement_bps"`
}

func (bc *Blockchain_struct) ProjectSupply(years int, blockTimeSeconds uint64) SupplyProjection {
	if years < 1 {
		years = 1
	}
	if years > 50 {
		years = 50
	}
	if blockTimeSeconds == 0 {
		blockTimeSeconds = 2
	}
	blocks := uint64(years) * 365 * 24 * 3600 / blockTimeSeconds
	issuance := big.NewInt(0)
	start := bc.LatestBlockNumber() + 1
	for height := start; height < start+blocks; {
		epochEnd := ((height / BlocksPerHalving) + 1) * BlocksPerHalving
		if epochEnd > start+blocks {
			epochEnd = start + blocks
		}
		count := epochEnd - height
		issuance.Add(issuance, new(big.Int).Mul(EmissionReward(height), new(big.Int).SetUint64(count)))
		height = epochEnd
	}
	cap := NewAmountFromStringOrZero(bc.EconomicPolicy.IssuanceCap)
	remainingCap := new(big.Int).Sub(cap, CopyAmount(bc.CumulativeEmission))
	if remainingCap.Sign() < 0 {
		remainingCap.SetInt64(0)
	}
	scheduled := new(big.Int).Set(issuance)
	capExceeded := issuance.Cmp(remainingCap) > 0
	if capExceeded {
		issuance.Set(remainingCap)
	}
	revenue := big.NewInt(0)
	for _, entry := range bc.ProtocolRevenue {
		revenue.Add(revenue, entry.Amount)
	}
	replacement := int64(0)
	if issuance.Sign() > 0 {
		v := new(big.Int).Div(new(big.Int).Mul(revenue, big.NewInt(10000)), issuance)
		if v.IsInt64() {
			replacement = v.Int64()
		}
	}
	return SupplyProjection{Years: years, Blocks: blocks, NewIssuance: issuance.String(), ScheduledIssuance: scheduled.String(), RemainingCap: remainingCap.String(), Cap: cap.String(), CapExceeded: capExceeded, RevenueReplacementBPS: replacement}
}
