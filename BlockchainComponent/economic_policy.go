package blockchaincomponent

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"
)

const (
	EconomicInsurance  = "insurance_reserve"
	EconomicLPYield    = "lp_real_yield"
	EconomicOperations = "operations"
	EconomicBuyback    = "verified_surplus_buyback"
)

type EconomicPolicy struct {
	InsuranceBPS         int64  `json:"insurance_bps"`
	LPYieldBPS           int64  `json:"lp_yield_bps"`
	OperationsBPS        int64  `json:"operations_bps"`
	BuybackBPS           int64  `json:"buyback_bps"`
	MinInsuranceBalance  string `json:"min_insurance_balance"`
	BuybackEnabled       bool   `json:"buyback_enabled"`
	IssuanceCap          string `json:"issuance_cap"`
	TreasuryDeployCapBPS int64  `json:"treasury_deployment_cap_bps"`
	LQDExposureCapBPS    int64  `json:"lqd_exposure_cap_bps"`
}

type ProtocolRevenueEntry struct {
	ID          string            `json:"id"`
	Source      string            `json:"source"`
	Amount      *big.Int          `json:"amount"`
	Timestamp   int64             `json:"timestamp"`
	Reference   string            `json:"reference"`
	Allocations map[string]string `json:"allocations"`
}

func DefaultEconomicPolicy() EconomicPolicy {
	return EconomicPolicy{InsuranceBPS: 2500, LPYieldBPS: 4500, OperationsBPS: 2000, BuybackBPS: 1000, MinInsuranceBalance: "0", BuybackEnabled: false, IssuanceCap: "100000000000000000", TreasuryDeployCapBPS: 2000, LQDExposureCapBPS: 3000}
}

func (p EconomicPolicy) Validate() error {
	if p.InsuranceBPS < 0 || p.LPYieldBPS < 0 || p.OperationsBPS < 0 || p.BuybackBPS < 0 || p.InsuranceBPS+p.LPYieldBPS+p.OperationsBPS+p.BuybackBPS != 10000 {
		return fmt.Errorf("economic waterfall must total 10000 bps")
	}
	if _, ok := new(big.Int).SetString(strings.TrimSpace(p.MinInsuranceBalance), 10); !ok {
		return fmt.Errorf("invalid minimum insurance balance")
	}
	if cap, ok := new(big.Int).SetString(strings.TrimSpace(p.IssuanceCap), 10); !ok || cap.Sign() <= 0 || p.TreasuryDeployCapBPS < 0 || p.TreasuryDeployCapBPS > 5000 || p.LQDExposureCapBPS < 500 || p.LQDExposureCapBPS > 10000 {
		return fmt.Errorf("invalid issuance, treasury deployment or LQD exposure cap")
	}
	return nil
}

func revenueID(source, reference string, amount *big.Int) string {
	sum := sha256.Sum256([]byte(strings.ToLower(source) + "|" + strings.TrimSpace(reference) + "|" + amount.String()))
	return "rev_" + hex.EncodeToString(sum[:12])
}

func (bc *Blockchain_struct) economicBalance(key string) *big.Int {
	if bc.EconomicBalances[key] == nil {
		bc.EconomicBalances[key] = big.NewInt(0)
	}
	return bc.EconomicBalances[key]
}

// RecordProtocolRevenue allocates only realized external revenue. Token
// emissions are deliberately excluded, preventing an inflation-funded APY from
// being presented as business income.
func (bc *Blockchain_struct) RecordProtocolRevenue(source string, amount *big.Int, reference string, timestamp int64) (ProtocolRevenueEntry, error) {
	if bc == nil || amount == nil || amount.Sign() <= 0 {
		return ProtocolRevenueEntry{}, fmt.Errorf("positive realized revenue is required")
	}
	source = strings.ToLower(strings.TrimSpace(source))
	allowed := map[string]bool{"trading_fee": true, "protocol_arb": true, "b2b_liquidity": true, "lending_interest": true, "bridge_fee": true, "slashing": true}
	if !allowed[source] {
		return ProtocolRevenueEntry{}, fmt.Errorf("unsupported realized revenue source")
	}
	if timestamp <= 0 {
		timestamp = time.Now().Unix()
	}
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	bc.EnsureRuntimeState()
	entry, err := bc.recordProtocolRevenueConsensus(source, amount, reference, timestamp)
	if err == nil {
		bc.persistRuntimeStateLocked()
	}
	return entry, err
}

// recordProtocolRevenueConsensus mutates only deterministic in-memory state.
// It is used during block execution/replay, where disk writes and wall-clock
// side effects are forbidden.
func (bc *Blockchain_struct) recordProtocolRevenueConsensus(source string, amount *big.Int, reference string, timestamp int64) (ProtocolRevenueEntry, error) {
	id := revenueID(source, reference, amount)
	for _, existing := range bc.ProtocolRevenue {
		if existing.ID == id {
			return existing, nil
		}
	}
	p := bc.EconomicPolicy
	insurance := new(big.Int).Div(new(big.Int).Mul(amount, big.NewInt(p.InsuranceBPS)), big.NewInt(10000))
	lpYield := new(big.Int).Div(new(big.Int).Mul(amount, big.NewInt(p.LPYieldBPS)), big.NewInt(10000))
	operations := new(big.Int).Div(new(big.Int).Mul(amount, big.NewInt(p.OperationsBPS)), big.NewInt(10000))
	// Slashing is a security-loss recovery, not ordinary operating income.
	// Classify it entirely as insurance so governance cannot market validator
	// misconduct as LP yield or divert it to a token buyback.
	if source == "slashing" {
		insurance.Set(amount)
		lpYield.SetInt64(0)
		operations.SetInt64(0)
	}
	buyback := new(big.Int).Sub(new(big.Int).Set(amount), new(big.Int).Add(new(big.Int).Add(insurance, lpYield), operations))
	minInsurance, _ := new(big.Int).SetString(p.MinInsuranceBalance, 10)
	if !p.BuybackEnabled || bc.economicBalance(EconomicInsurance).Cmp(minInsurance) < 0 {
		insurance.Add(insurance, buyback)
		buyback.SetInt64(0)
	}
	alloc := map[string]*big.Int{EconomicInsurance: insurance, EconomicLPYield: lpYield, EconomicOperations: operations, EconomicBuyback: buyback}
	entry := ProtocolRevenueEntry{ID: id, Source: source, Amount: CopyAmount(amount), Timestamp: timestamp, Reference: reference, Allocations: map[string]string{}}
	for bucket, value := range alloc {
		bc.economicBalance(bucket).Add(bc.economicBalance(bucket), value)
		entry.Allocations[bucket] = value.String()
	}
	bc.ProtocolRevenue = append(bc.ProtocolRevenue, entry)
	if len(bc.ProtocolRevenue) > 100000 {
		bc.ProtocolRevenue = bc.ProtocolRevenue[len(bc.ProtocolRevenue)-100000:]
	}
	return entry, nil
}

func (bc *Blockchain_struct) EconomicStatus() map[string]interface{} {
	if bc == nil {
		return map[string]interface{}{}
	}
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	bc.EnsureRuntimeState()
	balances := map[string]string{}
	for key, value := range bc.EconomicBalances {
		balances[key] = AmountString(value)
	}
	bySource := map[string]string{}
	for _, row := range bc.ProtocolRevenue {
		current, _ := new(big.Int).SetString(bySource[row.Source], 10)
		if current == nil {
			current = big.NewInt(0)
		}
		current.Add(current, row.Amount)
		bySource[row.Source] = current.String()
	}
	sources := make([]string, 0, len(bySource))
	for source := range bySource {
		sources = append(sources, source)
	}
	sort.Strings(sources)
	return map[string]interface{}{"policy": bc.EconomicPolicy, "balances": balances, "realized_revenue_by_source": bySource, "sources": sources, "revenue_entries": len(bc.ProtocolRevenue), "emissions_count_as_revenue": false}
}

type EconomicDailyPoint struct {
	Date        string            `json:"date"`
	Revenue     string            `json:"revenue"`
	BySource    map[string]string `json:"by_source"`
	Allocations map[string]string `json:"allocations"`
}

// EconomicTimeSeries returns exact realized-ledger history. Missing dates are
// included as zeros so dashboards cannot visually hide inactive periods.
func (bc *Blockchain_struct) EconomicTimeSeries(days int, now int64) []EconomicDailyPoint {
	if days < 1 {
		days = 1
	}
	if days > 3650 {
		days = 3650
	}
	if now <= 0 {
		now = time.Now().Unix()
	}
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	bc.EnsureRuntimeState()
	rows := make(map[string]*EconomicDailyPoint, days)
	for offset := days - 1; offset >= 0; offset-- {
		date := time.Unix(now-int64(offset*86400), 0).UTC().Format("2006-01-02")
		rows[date] = &EconomicDailyPoint{Date: date, Revenue: "0", BySource: map[string]string{}, Allocations: map[string]string{}}
	}
	add := func(values map[string]string, key string, amount *big.Int) {
		current, _ := new(big.Int).SetString(values[key], 10)
		if current == nil {
			current = big.NewInt(0)
		}
		values[key] = current.Add(current, amount).String()
	}
	for _, entry := range bc.ProtocolRevenue {
		date := time.Unix(entry.Timestamp, 0).UTC().Format("2006-01-02")
		row := rows[date]
		if row == nil {
			continue
		}
		total := NewAmountFromStringOrZero(row.Revenue)
		total.Add(total, entry.Amount)
		row.Revenue = total.String()
		add(row.BySource, entry.Source, entry.Amount)
		for bucket, raw := range entry.Allocations {
			add(row.Allocations, bucket, NewAmountFromStringOrZero(raw))
		}
	}
	out := make([]EconomicDailyPoint, 0, len(rows))
	for _, row := range rows {
		out = append(out, *row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out
}
