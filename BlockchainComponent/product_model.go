package blockchaincomponent

import (
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"
)

type VaultRiskProfile struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	MaxVolatility       float64  `json:"max_volatility"`
	MinOracleConfidence float64  `json:"min_oracle_confidence"`
	MinQualityBPS       int64    `json:"min_quality_bps"`
	MaxLQDExposureBPS   int64    `json:"max_lqd_exposure_bps"`
	LiquidBufferBPS     int64    `json:"liquid_buffer_bps"`
	AllowedAssets       []string `json:"allowed_assets"`
	RiskDisclosure      string   `json:"risk_disclosure"`
}

type LiquidityServiceAgreement struct {
	ID                      string            `json:"id"`
	Client                  string            `json:"client"`
	Markets                 []string          `json:"markets"`
	CapitalLimit            *big.Int          `json:"capital_limit"`
	ManagementFeeBPS        int64             `json:"management_fee_bps"`
	PerformanceFeeBPS       int64             `json:"performance_fee_bps"`
	UsesProtocolCapitalOnly bool              `json:"uses_protocol_capital_only"`
	Status                  string            `json:"status"`
	CreatedAt               int64             `json:"created_at"`
	GovernanceID            string            `json:"governance_id,omitempty"`
	FeesCollected           *big.Int          `json:"fees_collected,omitempty"`
	LastSettlementAt        int64             `json:"last_settlement_at,omitempty"`
	SettledInvoices         map[string]string `json:"settled_invoices,omitempty"`
}

type SuitabilityAnswers struct {
	LossToleranceBPS       int64 `json:"loss_tolerance_bps"`
	InvestmentHorizonDays  int64 `json:"investment_horizon_days"`
	LiquidityNeedDays      int64 `json:"liquidity_need_days"`
	DeFiExperienceYears    int64 `json:"defi_experience_years"`
	AcceptsImpermanentLoss bool  `json:"accepts_impermanent_loss"`
	AcceptsSmartContract   bool  `json:"accepts_smart_contract_risk"`
}

type SuitabilityResult struct {
	Eligible            bool              `json:"eligible"`
	RecommendedProfile  *VaultRiskProfile `json:"recommended_profile,omitempty"`
	Warnings            []string          `json:"warnings"`
	RequiresHumanReview bool              `json:"requires_human_review"`
}

func RetailVaultRiskProfiles() []VaultRiskProfile {
	return []VaultRiskProfile{
		{ID: "conservative", Name: "Conservative", MaxVolatility: 0.05, MinOracleConfidence: 0.90, MinQualityBPS: 8000, MaxLQDExposureBPS: 2500, LiquidBufferBPS: 2000, AllowedAssets: []string{"USDC", "USDT", "ETH", "BTC"}, RiskDisclosure: "Capital is not guaranteed; proportional LP exit can realize impermanent loss."},
		{ID: "balanced", Name: "Balanced", MaxVolatility: 0.12, MinOracleConfidence: 0.80, MinQualityBPS: 6500, MaxLQDExposureBPS: 4000, LiquidBufferBPS: 1500, AllowedAssets: []string{"USDC", "USDT", "ETH", "BTC", "LQD"}, RiskDisclosure: "Moderate smart-contract, market, oracle and impermanent-loss risk."},
		{ID: "growth", Name: "Growth", MaxVolatility: 0.20, MinOracleConfidence: 0.70, MinQualityBPS: 5000, MaxLQDExposureBPS: 6000, LiquidBufferBPS: 1000, AllowedAssets: []string{"GOVERNANCE_APPROVED"}, RiskDisclosure: "High volatility and liquidity migration risk; losses can be substantial."},
	}
}

// EvaluateRetailSuitability is a deterministic risk gate, not financial
// advice. A user who rejects either principal risk cannot enter any profile.
func EvaluateRetailSuitability(a SuitabilityAnswers) SuitabilityResult {
	result := SuitabilityResult{Warnings: []string{"Returns and principal are not guaranteed.", "Withdrawal value depends on pool prices and available liquidity."}}
	if !a.AcceptsImpermanentLoss || !a.AcceptsSmartContract || a.LossToleranceBPS < 500 || a.InvestmentHorizonDays < 30 {
		result.Warnings = append(result.Warnings, "Answers are incompatible with an LP strategy; no vault profile is suitable.")
		return result
	}
	profiles := RetailVaultRiskProfiles()
	index := 0
	if a.LossToleranceBPS >= 1500 && a.InvestmentHorizonDays >= 180 && a.DeFiExperienceYears >= 1 {
		index = 1
	}
	if a.LossToleranceBPS >= 3000 && a.InvestmentHorizonDays >= 365 && a.DeFiExperienceYears >= 2 && a.LiquidityNeedDays >= 7 {
		index = 2
	}
	profile := profiles[index]
	result.Eligible, result.RecommendedProfile = true, &profile
	result.RequiresHumanReview = a.LossToleranceBPS >= 5000 || a.LiquidityNeedDays == 0
	if result.RequiresHumanReview {
		result.Warnings = append(result.Warnings, "High stated risk tolerance or immediate liquidity need requires manual review.")
	}
	return result
}

func (bc *Blockchain_struct) CreateLiquidityServiceAgreement(client string, markets []string, capitalLimit *big.Int, managementFeeBPS, performanceFeeBPS int64, protocolCapitalOnly bool) (*LiquidityServiceAgreement, error) {
	client = strings.TrimSpace(client)
	if bc == nil || client == "" || len(markets) == 0 || capitalLimit == nil || capitalLimit.Sign() <= 0 {
		return nil, fmt.Errorf("client, markets and positive capital limit are required")
	}
	if !protocolCapitalOnly {
		return nil, fmt.Errorf("pilot agreements may use protocol-owned capital only")
	}
	if managementFeeBPS < 0 || managementFeeBPS > 500 || performanceFeeBPS < 0 || performanceFeeBPS > 3000 {
		return nil, fmt.Errorf("service fee exceeds governance safety bounds")
	}
	now := time.Now().Unix()
	id := hashedID("laas", client, strings.Join(markets, ","), capitalLimit, now)
	agreement := &LiquidityServiceAgreement{ID: id, Client: client, Markets: append([]string(nil), markets...), CapitalLimit: CopyAmount(capitalLimit), ManagementFeeBPS: managementFeeBPS, PerformanceFeeBPS: performanceFeeBPS, UsesProtocolCapitalOnly: true, Status: "pilot_pending_governance", CreatedAt: now, FeesCollected: big.NewInt(0), SettledInvoices: map[string]string{}}
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	bc.EnsureRuntimeState()
	bc.BusinessAgreements[id] = agreement
	bc.persistRuntimeStateLocked()
	copyAgreement := *agreement
	copyAgreement.CapitalLimit = CopyAmount(agreement.CapitalLimit)
	return &copyAgreement, nil
}

func (bc *Blockchain_struct) ActivateLiquidityServiceAgreement(agreementID, governanceID string) error {
	if bc == nil || strings.TrimSpace(governanceID) == "" {
		return fmt.Errorf("agreement and governance approval required")
	}
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	bc.EnsureRuntimeState()
	agreement := bc.BusinessAgreements[agreementID]
	proposal := bc.Governance.Proposals[governanceID]
	if agreement == nil || agreement.Status != "pilot_pending_governance" || proposal == nil || proposal.Status != "executed" {
		return fmt.Errorf("pending agreement and executed governance proposal required")
	}
	agreement.Status, agreement.GovernanceID = "active", governanceID
	bc.persistRuntimeStateLocked()
	return nil
}

// SettleLiquidityServiceInvoice calculates the contracted management and
// performance fee, captures actual LQD from the client, and records only that
// realized amount as B2B revenue. The caller supplies the agreed billing-period
// management base and realized positive performance for that period.
func (bc *Blockchain_struct) SettleLiquidityServiceInvoice(agreementID, payer string, managementBase, realizedProfit *big.Int, invoiceReference string, timestamp int64) (ProtocolRevenueEntry, error) {
	if bc == nil || managementBase == nil || realizedProfit == nil || managementBase.Sign() < 0 || realizedProfit.Sign() < 0 || strings.TrimSpace(invoiceReference) == "" {
		return ProtocolRevenueEntry{}, fmt.Errorf("valid non-negative invoice bases and reference required")
	}
	bc.Mutex.Lock()
	bc.EnsureRuntimeState()
	agreement := bc.BusinessAgreements[agreementID]
	if agreement == nil || agreement.Status != "active" || !strings.EqualFold(agreement.Client, payer) || managementBase.Cmp(agreement.CapitalLimit) > 0 {
		bc.Mutex.Unlock()
		return ProtocolRevenueEntry{}, fmt.Errorf("active payer-owned agreement and bounded capital base required")
	}
	managementBPS, performanceBPS := agreement.ManagementFeeBPS, agreement.PerformanceFeeBPS
	bc.Mutex.Unlock()
	managementFee := new(big.Int).Div(new(big.Int).Mul(managementBase, big.NewInt(managementBPS)), big.NewInt(10000))
	performanceFee := new(big.Int).Div(new(big.Int).Mul(realizedProfit, big.NewInt(performanceBPS)), big.NewInt(10000))
	fee := new(big.Int).Add(managementFee, performanceFee)
	if fee.Sign() <= 0 {
		return ProtocolRevenueEntry{}, fmt.Errorf("invoice produces no realized fee")
	}
	reference := "laas:" + agreementID + ":" + strings.TrimSpace(invoiceReference)
	entry, err := bc.CaptureProtocolRevenue("b2b_liquidity", payer, fee, reference, timestamp)
	if err != nil {
		return ProtocolRevenueEntry{}, err
	}
	bc.Mutex.Lock()
	if current := bc.BusinessAgreements[agreementID]; current != nil {
		if current.FeesCollected == nil {
			current.FeesCollected = big.NewInt(0)
		}
		if current.SettledInvoices == nil {
			current.SettledInvoices = map[string]string{}
		}
		// CaptureProtocolRevenue returns the existing entry for duplicate invoice
		// references, so agreement totals use the same entry ID as their idempotency key.
		if current.SettledInvoices[entry.ID] == "" {
			current.FeesCollected.Add(current.FeesCollected, entry.Amount)
			current.LastSettlementAt = entry.Timestamp
			current.SettledInvoices[entry.ID] = entry.Amount.String()
		}
		bc.persistRuntimeStateLocked()
	}
	bc.Mutex.Unlock()
	return entry, nil
}

func (bc *Blockchain_struct) ProductStatus() map[string]interface{} {
	if bc == nil {
		return map[string]interface{}{}
	}
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	bc.EnsureRuntimeState()
	agreements := make([]*LiquidityServiceAgreement, 0, len(bc.BusinessAgreements))
	for _, agreement := range bc.BusinessAgreements {
		if agreement != nil {
			cp := *agreement
			cp.CapitalLimit = CopyAmount(agreement.CapitalLimit)
			agreements = append(agreements, &cp)
		}
	}
	sort.Slice(agreements, func(i, j int) bool { return agreements[i].CreatedAt < agreements[j].CreatedAt })
	return map[string]interface{}{"retail_vault_profiles": RetailVaultRiskProfiles(), "liquidity_as_a_service": agreements, "pilot_capital_boundary": "protocol-owned capital only", "fixed_apy_promised": false, "principal_guaranteed": false}
}

func (bc *Blockchain_struct) InvestorMetrics() map[string]interface{} {
	if bc == nil {
		return map[string]interface{}{}
	}
	set := bc.buildValidatorPowerSet()
	totalPower, maxPower := 0.0, 0.0
	for _, validator := range set {
		totalPower += validator.Power
		if validator.Power > maxPower {
			maxPower = validator.Power
		}
	}
	concentration := 0.0
	if totalPower > 0 {
		concentration = maxPower / totalPower
	}
	realizedRevenue := big.NewInt(0)
	bc.Mutex.Lock()
	for _, entry := range bc.ProtocolRevenue {
		if entry.Amount != nil {
			realizedRevenue.Add(realizedRevenue, entry.Amount)
		}
	}
	agreements := len(bc.BusinessAgreements)
	blocks := len(bc.Blocks)
	now := time.Now().Unix()
	active7, active30, daysByUser := map[string]bool{}, map[string]bool{}, map[string]map[int64]bool{}
	for _, block := range bc.Blocks {
		if block == nil {
			continue
		}
		age := now - int64(block.TimeStamp)
		for _, tx := range block.Transactions {
			if tx == nil || tx.IsSystem || !ValidateAddress(tx.From) {
				continue
			}
			user := strings.ToLower(tx.From)
			if age >= 0 && age <= 30*86400 {
				active30[user] = true
				if daysByUser[user] == nil {
					daysByUser[user] = map[int64]bool{}
				}
				daysByUser[user][int64(block.TimeStamp)/86400] = true
			}
			if age >= 0 && age <= 7*86400 {
				active7[user] = true
			}
		}
	}
	returning := 0
	for _, days := range daysByUser {
		if len(days) >= 2 {
			returning++
		}
	}
	bc.Mutex.Unlock()
	return map[string]interface{}{"finalized_blocks_in_memory": blocks, "validator_count": len(set), "largest_validator_power_share": concentration, "realized_protocol_revenue": realizedRevenue.String(), "business_pilot_count": agreements, "active_users_7d": len(active7), "active_users_30d": len(active30), "returning_users_30d": returning, "retention_definition": "address active on at least two distinct UTC days", "consensus": bc.ConsensusStatus(), "liquidity_quality": bc.LiquidityQualityStatus(), "forward_revenue_projection_included": false, "unaudited_metrics": true}
}
