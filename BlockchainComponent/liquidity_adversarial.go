package blockchaincomponent

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
)

type LiquidityFlow struct {
	TxID        string  `json:"tx_id"`
	From        string  `json:"from"`
	To          string  `json:"to"`
	FromCluster string  `json:"from_cluster,omitempty"`
	ToCluster   string  `json:"to_cluster,omitempty"`
	AmountUSD   float64 `json:"amount_usd"`
	FeeUSD      float64 `json:"fee_usd"`
	Timestamp   int64   `json:"timestamp"`
}

type OrganicFlowAssessment struct {
	GrossVolumeUSD       float64  `json:"gross_volume_usd"`
	OrganicVolumeUSD     float64  `json:"organic_volume_usd"`
	WashVolumeUSD        float64  `json:"wash_volume_usd"`
	CircularVolumeUSD    float64  `json:"circular_volume_usd"`
	FeesPaidUSD          float64  `json:"fees_paid_usd"`
	UniqueEntities       int      `json:"unique_entities"`
	OrganicBPS           int64    `json:"organic_bps"`
	AttackCostBPS        int64    `json:"attack_cost_bps"`
	IdentityDiversityBPS int64    `json:"identity_diversity_bps"`
	Score                float64  `json:"score"`
	Flags                []string `json:"flags,omitempty"`
}

func flowEntity(address, cluster string) string {
	if strings.TrimSpace(cluster) != "" {
		return "cluster:" + strings.ToLower(strings.TrimSpace(cluster))
	}
	return "address:" + strings.ToLower(strings.TrimSpace(address))
}

// AssessOrganicFlow penalizes same-entity flow, low-cost volume and quick
// circular round trips. Clusters are supplied by a deterministic/public
// classifier; when absent, addresses remain separate entities.
func AssessOrganicFlow(flows []LiquidityFlow, circularWindowSeconds int64, minimumEconomicFeeBPS int64) OrganicFlowAssessment {
	if circularWindowSeconds <= 0 {
		circularWindowSeconds = 900
	}
	if minimumEconomicFeeBPS <= 0 {
		minimumEconomicFeeBPS = 5
	}
	ordered := append([]LiquidityFlow(nil), flows...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Timestamp < ordered[j].Timestamp })
	out, entities := OrganicFlowAssessment{}, map[string]bool{}
	classified := make([]bool, len(ordered))
	for i, flow := range ordered {
		if flow.AmountUSD <= 0 || flow.FeeUSD < 0 {
			continue
		}
		out.GrossVolumeUSD += flow.AmountUSD
		out.FeesPaidUSD += flow.FeeUSD
		from, to := flowEntity(flow.From, flow.FromCluster), flowEntity(flow.To, flow.ToCluster)
		entities[from], entities[to] = true, true
		if from == to {
			classified[i] = true
			out.WashVolumeUSD += flow.AmountUSD
			continue
		}
		for j := i + 1; j < len(ordered); j++ {
			reverse := ordered[j]
			if reverse.Timestamp-flow.Timestamp > circularWindowSeconds {
				break
			}
			if classified[j] || flowEntity(reverse.From, reverse.FromCluster) != to || flowEntity(reverse.To, reverse.ToCluster) != from {
				continue
			}
			ratio := reverse.AmountUSD / flow.AmountUSD
			if ratio >= 0.95 && ratio <= 1.05 {
				classified[i], classified[j] = true, true
				out.CircularVolumeUSD += flow.AmountUSD + reverse.AmountUSD
				break
			}
		}
	}
	for i, flow := range ordered {
		if flow.AmountUSD > 0 && !classified[i] {
			out.OrganicVolumeUSD += flow.AmountUSD
		}
	}
	out.UniqueEntities = len(entities)
	if out.GrossVolumeUSD <= 0 {
		return out
	}
	out.OrganicBPS = int64(math.Round(out.OrganicVolumeUSD / out.GrossVolumeUSD * 10_000))
	out.AttackCostBPS = int64(math.Round(out.FeesPaidUSD / out.GrossVolumeUSD * 10_000))
	if out.AttackCostBPS > 10_000 {
		out.AttackCostBPS = 10_000
	}
	// Diversity reaches full credit at 32 distinct public entities. It is a
	// cap, not proof of personhood, and is therefore never the sole signal.
	out.IdentityDiversityBPS = int64(math.Min(10_000, float64(out.UniqueEntities)*10_000/32))
	costScore := math.Min(1, float64(out.AttackCostBPS)/float64(minimumEconomicFeeBPS))
	out.Score = clamp01(float64(out.OrganicBPS)/10_000) * (0.7 + 0.3*float64(out.IdentityDiversityBPS)/10_000) * costScore
	if out.WashVolumeUSD > 0 {
		out.Flags = append(out.Flags, "same-entity-flow")
	}
	if out.CircularVolumeUSD > 0 {
		out.Flags = append(out.Flags, "circular-flow")
	}
	if out.AttackCostBPS < minimumEconomicFeeBPS {
		out.Flags = append(out.Flags, "low-economic-cost")
	}
	return out
}

type LiquidityQualityWeights struct {
	Depth, Demand, Volatility, Oracle, Concentration float64
}

func DefaultLiquidityQualityWeights() LiquidityQualityWeights {
	return LiquidityQualityWeights{Depth: .35, Demand: .25, Volatility: .20, Oracle: .15, Concentration: .05}
}

func (w LiquidityQualityWeights) Valid() bool {
	values := []float64{w.Depth, w.Demand, w.Volatility, w.Oracle, w.Concentration}
	total := 0.0
	for _, value := range values {
		if value < 0 || value > 1 {
			return false
		}
		total += value
	}
	return math.Abs(total-1) < 0.0000001
}

func ScoreLiquidityComponents(w LiquidityQualityWeights, depth, demand, volatility, oracle, concentration float64) float64 {
	if !w.Valid() {
		w = DefaultLiquidityQualityWeights()
	}
	return clamp01(w.Depth*clamp01(depth) + w.Demand*clamp01(demand) + w.Volatility*clamp01(volatility) + w.Oracle*clamp01(oracle) + w.Concentration*clamp01(concentration))
}

type LiquidityBacktestPoint struct {
	Depth, Demand, Volatility, Oracle, Concentration float64
	Manipulated                                      bool
}

type LiquidityBacktestResult struct {
	Weights          LiquidityQualityWeights `json:"weights"`
	Samples          int                     `json:"samples"`
	AttackAcceptance int                     `json:"attack_acceptance"`
	OrganicRejection int                     `json:"organic_rejection"`
	AccuracyBPS      int64                   `json:"accuracy_bps"`
}

func LoadLiquidityBacktestCSV(r io.Reader) ([]LiquidityBacktestPoint, error) {
	records, err := csv.NewReader(r).ReadAll()
	if err != nil {
		return nil, err
	}
	points := make([]LiquidityBacktestPoint, 0, len(records))
	for row, record := range records {
		if row == 0 && len(record) > 0 && strings.EqualFold(strings.TrimSpace(record[0]), "depth") {
			continue
		}
		if len(record) != 6 {
			return nil, fmt.Errorf("row %d must contain five components and manipulated", row+1)
		}
		values := make([]float64, 5)
		for i := range values {
			values[i], err = strconv.ParseFloat(strings.TrimSpace(record[i]), 64)
			if err != nil || values[i] < 0 || values[i] > 1 {
				return nil, fmt.Errorf("row %d component %d outside [0,1]", row+1, i+1)
			}
		}
		manipulated, parseErr := strconv.ParseBool(strings.TrimSpace(record[5]))
		if parseErr != nil {
			return nil, fmt.Errorf("row %d invalid manipulated flag", row+1)
		}
		points = append(points, LiquidityBacktestPoint{values[0], values[1], values[2], values[3], values[4], manipulated})
	}
	return points, nil
}

func EvaluateLiquidityWeights(points []LiquidityBacktestPoint, weights LiquidityQualityWeights, acceptanceThreshold float64) LiquidityBacktestResult {
	out := LiquidityBacktestResult{Weights: weights, Samples: len(points)}
	correct := 0
	for _, point := range points {
		accepted := ScoreLiquidityComponents(weights, point.Depth, point.Demand, point.Volatility, point.Oracle, point.Concentration) >= acceptanceThreshold
		if point.Manipulated && accepted {
			out.AttackAcceptance++
		}
		if !point.Manipulated && !accepted {
			out.OrganicRejection++
		}
		if accepted != point.Manipulated {
			correct++
		}
	}
	if len(points) > 0 {
		out.AccuracyBPS = int64(correct * 10_000 / len(points))
	}
	return out
}

// OptimizeLiquidityWeights performs a deterministic 5% grid search. False
// attack acceptance costs three times an organic false rejection.
func OptimizeLiquidityWeights(points []LiquidityBacktestPoint, acceptanceThreshold float64) LiquidityBacktestResult {
	maxInt := int(^uint(0) >> 1)
	best := LiquidityBacktestResult{Samples: len(points), AttackAcceptance: maxInt, OrganicRejection: maxInt}
	bestCost := maxInt
	for depth := 0; depth <= 20; depth++ {
		for demand := 0; demand <= 20-depth; demand++ {
			for volatility := 0; volatility <= 20-depth-demand; volatility++ {
				for oracle := 0; oracle <= 20-depth-demand-volatility; oracle++ {
					concentration := 20 - depth - demand - volatility - oracle
					weights := LiquidityQualityWeights{float64(depth) / 20, float64(demand) / 20, float64(volatility) / 20, float64(oracle) / 20, float64(concentration) / 20}
					result := EvaluateLiquidityWeights(points, weights, acceptanceThreshold)
					cost := result.AttackAcceptance*3 + result.OrganicRejection
					if cost < bestCost || (cost == bestCost && result.AccuracyBPS > best.AccuracyBPS) {
						best, bestCost = result, cost
					}
				}
			}
		}
	}
	return best
}

type LiquidityAttackCost struct {
	NativeBondUSD, LiquidityCapitalUSD, TradingFeesUSD, OracleCorruptionUSD, SlashingExposureUSD float64
	TotalUSD, SecurityBudgetRatio                                                                float64
	MeetsSecurityTarget                                                                          bool
}

type AMMManipulationCost struct {
	ReserveUSD       float64 `json:"reserve_usd"`
	TargetMoveBPS    int64   `json:"target_move_bps"`
	RequiredInputUSD float64 `json:"required_input_usd"`
	TradingFeeUSD    float64 `json:"trading_fee_usd"`
	RoundTripLossUSD float64 `json:"round_trip_loss_usd"`
	Feasible         bool    `json:"feasible"`
}

// EstimateConstantProductManipulationCost computes the minimum one-sided
// input needed to move a 50/50 pool spot price by targetMoveBPS. It explicitly
// includes fees and a conservative immediate round-trip loss; a flash loan
// changes financing duration but cannot remove AMM price impact or fees.
func EstimateConstantProductManipulationCost(reserveUSD float64, targetMoveBPS, feeBPS int64, availableCapitalUSD float64) AMMManipulationCost {
	out := AMMManipulationCost{ReserveUSD: math.Max(0, reserveUSD), TargetMoveBPS: targetMoveBPS}
	if reserveUSD <= 0 || targetMoveBPS <= 0 || feeBPS < 0 || feeBPS >= 10000 {
		return out
	}
	ratio := 1 + float64(targetMoveBPS)/10000
	netInput := reserveUSD * (math.Sqrt(ratio) - 1)
	grossInput := netInput / (1 - float64(feeBPS)/10000)
	out.RequiredInputUSD = grossInput
	out.TradingFeeUSD = grossInput - netInput
	// Reversing the manipulation pays fees twice; adverse arbitrage can only
	// increase this lower bound.
	out.RoundTripLossUSD = out.TradingFeeUSD * 2
	out.Feasible = availableCapitalUSD >= grossInput+out.RoundTripLossUSD
	return out
}

func EstimateLiquidityAttackCost(nativeBondUSD, liquidityCapitalUSD, turnoverMultiplier, feeBPS, oracleCorruptionUSD, slashFraction, protectedValueUSD, requiredRatio float64) LiquidityAttackCost {
	if turnoverMultiplier < 0 {
		turnoverMultiplier = 0
	}
	if feeBPS < 0 {
		feeBPS = 0
	}
	if slashFraction < 0 {
		slashFraction = 0
	}
	if slashFraction > 1 {
		slashFraction = 1
	}
	out := LiquidityAttackCost{NativeBondUSD: math.Max(0, nativeBondUSD), LiquidityCapitalUSD: math.Max(0, liquidityCapitalUSD), OracleCorruptionUSD: math.Max(0, oracleCorruptionUSD)}
	out.TradingFeesUSD = out.LiquidityCapitalUSD * turnoverMultiplier * feeBPS / 10_000
	out.SlashingExposureUSD = out.NativeBondUSD * slashFraction
	out.TotalUSD = out.LiquidityCapitalUSD + out.TradingFeesUSD + out.OracleCorruptionUSD + out.SlashingExposureUSD
	if protectedValueUSD > 0 {
		out.SecurityBudgetRatio = out.TotalUSD / protectedValueUSD
	}
	out.MeetsSecurityTarget = protectedValueUSD > 0 && out.SecurityBudgetRatio >= requiredRatio
	return out
}
