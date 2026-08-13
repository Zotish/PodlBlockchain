package blockchaincomponent

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

type OracleUpdatePayload struct {
	Asset      string  `json:"asset"`
	Source     string  `json:"source"`
	PriceUSD   float64 `json:"price_usd"`
	Confidence float64 `json:"confidence"`
	ObservedAt int64   `json:"observed_at"`
	Nonce      uint64  `json:"nonce"`
}

func DecodeOracleUpdateTransaction(tx *Transaction) (OracleUpdatePayload, error) {
	var payload OracleUpdatePayload
	if tx == nil || tx.Type != "oracle_update" || len(tx.ExtraData) == 0 {
		return payload, fmt.Errorf("oracle_update transaction required")
	}
	if err := json.Unmarshal(tx.ExtraData, &payload); err != nil {
		return payload, fmt.Errorf("invalid oracle payload: %w", err)
	}
	payload.Asset = strings.ToUpper(strings.TrimSpace(payload.Asset))
	payload.Source = strings.ToLower(strings.TrimSpace(payload.Source))
	return payload, nil
}

func (bc *Blockchain_struct) ValidateOracleUpdateTransactionAt(tx *Transaction, blockTime int64) (OracleUpdatePayload, error) {
	payload, err := DecodeOracleUpdateTransaction(tx)
	if err != nil {
		return payload, err
	}
	if payload.Asset == "" || payload.Source == "" || payload.PriceUSD <= 0 || payload.Confidence < 0.5 || payload.Confidence > 1 {
		return payload, fmt.Errorf("invalid oracle observation")
	}
	if blockTime <= 0 {
		blockTime = time.Now().Unix()
	}
	if payload.ObservedAt > blockTime+30 || blockTime-payload.ObservedAt > OracleMaxAgeSeconds {
		return payload, fmt.Errorf("oracle observation outside freshness window")
	}
	if !strings.EqualFold(bc.OraclePublishers[payload.Source], tx.From) {
		return payload, fmt.Errorf("transaction signer is not publisher for source")
	}
	if payload.Nonce != bc.OracleNonces[payload.Source]+1 {
		return payload, fmt.Errorf("oracle nonce mismatch")
	}
	return payload, nil
}

func (bc *Blockchain_struct) ApplyOracleUpdateTransactionAt(tx *Transaction, blockTime int64) error {
	payload, err := bc.ValidateOracleUpdateTransactionAt(tx, blockTime)
	if err != nil {
		return err
	}
	if bc.OracleObservations[payload.Asset] == nil {
		bc.OracleObservations[payload.Asset] = make(map[string]OracleObservation)
	}
	prior := bc.OracleObservations[payload.Asset][payload.Source]
	if prior.Timestamp > payload.ObservedAt {
		return fmt.Errorf("oracle timestamp regression")
	}
	bc.OracleObservations[payload.Asset][payload.Source] = OracleObservation{Asset: payload.Asset, Source: payload.Source, PriceUSD: payload.PriceUSD, Confidence: payload.Confidence, Timestamp: payload.ObservedAt}
	bc.OracleNonces[payload.Source] = payload.Nonce
	return nil
}

type PairRiskPolicy struct {
	PairAddress       string `json:"pair_address"`
	RiskClass         string `json:"risk_class"`
	CorrelatedGroup   string `json:"correlated_group,omitempty"`
	ApprovedAtHeight  uint64 `json:"approved_at_height"`
	ActiveAfterHeight uint64 `json:"active_after_height"`
	MaxExposureBPS    int64  `json:"max_exposure_bps"`
	RecoveryDelaySec  int64  `json:"recovery_delay_seconds"`
	CircuitTrippedAt  int64  `json:"circuit_tripped_at,omitempty"`
	Enabled           bool   `json:"enabled"`
}

func (p PairRiskPolicy) Validate() error {
	if strings.TrimSpace(p.PairAddress) == "" || p.ActiveAfterHeight <= p.ApprovedAtHeight || p.MaxExposureBPS < 100 || p.MaxExposureBPS > 10000 || p.RecoveryDelaySec < 60 {
		return fmt.Errorf("invalid pair onboarding/risk policy")
	}
	switch strings.ToLower(p.RiskClass) {
	case "bluechip", "stable", "longtail", "experimental":
		return nil
	default:
		return fmt.Errorf("unsupported asset risk class")
	}
}

func (bc *Blockchain_struct) SetPairRiskPolicy(policy PairRiskPolicy) error {
	policy.PairAddress = strings.ToLower(strings.TrimSpace(policy.PairAddress))
	policy.RiskClass = strings.ToLower(strings.TrimSpace(policy.RiskClass))
	policy.CorrelatedGroup = strings.ToLower(strings.TrimSpace(policy.CorrelatedGroup))
	if err := policy.Validate(); err != nil {
		return err
	}
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	bc.EnsureRuntimeState()
	bc.PairRiskPolicies[policy.PairAddress] = policy
	bc.persistRuntimeStateLocked()
	return nil
}

func (bc *Blockchain_struct) pairRoutable(pair string, height uint64, now int64) (PairRiskPolicy, bool, string) {
	policy, configured := bc.PairRiskPolicies[strings.ToLower(strings.TrimSpace(pair))]
	if !configured {
		if bc.ChainSpec.AllowLegacyFinality {
			return policy, true, "legacy-testnet"
		}
		return policy, false, "pair has no governance risk policy"
	}
	if !policy.Enabled || height < policy.ActiveAfterHeight {
		return policy, false, "pair onboarding delay or disablement"
	}
	if policy.CircuitTrippedAt > 0 && now-policy.CircuitTrippedAt < policy.RecoveryDelaySec {
		return policy, false, "circuit recovery delay active"
	}
	return policy, true, ""
}

type CongestionBucket struct {
	HourUTC       int     `json:"hour_utc"`
	Samples       uint64  `json:"samples"`
	EWMAUtil      float64 `json:"ewma_utilization"`
	EWMASwapCount float64 `json:"ewma_swap_count"`
}

func (bc *Blockchain_struct) learnCongestionProfile(metrics []PoolMetrics, evaluationUnix int64) {
	if bc == nil || len(metrics) == 0 {
		return
	}
	hour := time.Unix(evaluationUnix, 0).UTC().Hour()
	util, swaps := 0.0, 0.0
	for _, metric := range metrics {
		util += metric.UtilScore
		swaps += float64(metric.SwapCount)
	}
	util /= float64(len(metrics))
	bucket := bc.CongestionProfile[hour]
	bucket.HourUTC, bucket.Samples = hour, bucket.Samples+1
	alpha := 0.25
	if bucket.Samples == 1 {
		bucket.EWMAUtil, bucket.EWMASwapCount = util, swaps
	} else {
		bucket.EWMAUtil = alpha*util + (1-alpha)*bucket.EWMAUtil
		bucket.EWMASwapCount = alpha*swaps + (1-alpha)*bucket.EWMASwapCount
	}
	bc.CongestionProfile[hour] = bucket
}

func (bc *Blockchain_struct) learnedTimeMultiplier(evaluationUnix int64) float64 {
	if bc == nil || len(bc.CongestionProfile) < 3 {
		return timeMultiplierAt(evaluationUnix)
	}
	hour := time.Unix(evaluationUnix, 0).UTC().Hour()
	current, ok := bc.CongestionProfile[hour]
	if !ok || current.Samples == 0 {
		return 1
	}
	values := make([]float64, 0, len(bc.CongestionProfile))
	for _, bucket := range bc.CongestionProfile {
		if bucket.Samples > 0 {
			values = append(values, bucket.EWMAUtil)
		}
	}
	sort.Float64s(values)
	median := values[len(values)/2]
	if median <= 0 {
		return 1
	}
	return math.Max(0.7, math.Min(1.2, 1+(current.EWMAUtil-median)/median*0.2))
}
