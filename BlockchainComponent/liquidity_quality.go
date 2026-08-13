package blockchaincomponent

import (
	"fmt"
	"math"
	"math/big"
	"sort"
	"strings"
	"time"
)

const (
	OracleMaxAgeSeconds        int64 = 900
	OracleMinSources                 = 3
	OracleMaxDeviationBPS      int64 = 2000
	LiquidityCircuitVolatility       = 0.25
)

type OracleObservation struct {
	Asset      string  `json:"asset"`
	Source     string  `json:"source"`
	PriceUSD   float64 `json:"price_usd"`
	Confidence float64 `json:"confidence"`
	Timestamp  int64   `json:"timestamp"`
}

type MedianOraclePrice struct {
	Asset       string   `json:"asset"`
	PriceUSD    float64  `json:"price_usd"`
	Confidence  float64  `json:"confidence"`
	Sources     []string `json:"sources"`
	UpdatedAt   int64    `json:"updated_at"`
	Valid       bool     `json:"valid"`
	Stale       bool     `json:"stale"`
	RejectCount int      `json:"reject_count"`
}

type PoolPriceObservation struct {
	PairAddress string  `json:"pair_address"`
	Price       float64 `json:"price"`
	Timestamp   int64   `json:"timestamp"`
}

type LiquidityQuality struct {
	PairAddress        string  `json:"pair_address"`
	DepthUSD           float64 `json:"depth_usd"`
	ExecutableDepthUSD float64 `json:"executable_depth_usd"`
	OrganicDemandScore float64 `json:"organic_demand_score"`
	DepthScore         float64 `json:"depth_score"`
	Volatility         float64 `json:"volatility"`
	TWAPPrice          float64 `json:"twap_price"`
	VolatilityScore    float64 `json:"volatility_score"`
	OracleConfidence   float64 `json:"oracle_confidence"`
	ConcentrationScore float64 `json:"concentration_score"`
	QualityScore       float64 `json:"quality_score"`
	QualityBPS         int64   `json:"quality_bps"`
	CircuitBroken      bool    `json:"circuit_broken"`
	CircuitBreakReason string  `json:"circuit_break_reason,omitempty"`
	Valid              bool    `json:"valid"`
}

// PoolTWAP computes a true time-weighted average over the requested rolling
// window. Observations are piecewise-constant until the next observation.
func (bc *Blockchain_struct) PoolTWAP(pair string, evaluationUnix, windowSeconds int64) float64 {
	if bc == nil || windowSeconds <= 0 {
		return 0
	}
	pair = strings.ToLower(strings.TrimSpace(pair))
	history := append([]PoolPriceObservation(nil), bc.PoolPriceHistory[pair]...)
	if len(history) == 0 {
		return 0
	}
	sort.Slice(history, func(i, j int) bool { return history[i].Timestamp < history[j].Timestamp })
	start := evaluationUnix - windowSeconds
	price := history[0].Price
	for _, observation := range history {
		if observation.Timestamp <= start && observation.Price > 0 {
			price = observation.Price
		}
	}
	cursor, weighted, duration := start, 0.0, int64(0)
	for _, observation := range history {
		if observation.Timestamp <= start || observation.Timestamp > evaluationUnix || observation.Price <= 0 {
			continue
		}
		dt := observation.Timestamp - cursor
		if dt > 0 && price > 0 {
			weighted += price * float64(dt)
			duration += dt
		}
		price, cursor = observation.Price, observation.Timestamp
	}
	if cursor < evaluationUnix && price > 0 {
		dt := evaluationUnix - cursor
		weighted += price * float64(dt)
		duration += dt
	}
	if duration == 0 {
		return price
	}
	return weighted / float64(duration)
}

func (bc *Blockchain_struct) SubmitOracleObservation(obs OracleObservation) error {
	if bc == nil {
		return fmt.Errorf("nil blockchain")
	}
	obs.Asset = strings.ToUpper(strings.TrimSpace(obs.Asset))
	obs.Source = strings.ToLower(strings.TrimSpace(obs.Source))
	if obs.Asset == "" || obs.Source == "" || obs.PriceUSD <= 0 {
		return fmt.Errorf("asset, source and positive price are required")
	}
	if obs.Confidence <= 0 || obs.Confidence > 1 {
		return fmt.Errorf("confidence must be within (0,1]")
	}
	now := time.Now().Unix()
	if obs.Timestamp == 0 {
		obs.Timestamp = now
	}
	if obs.Timestamp > now+30 || now-obs.Timestamp > OracleMaxAgeSeconds {
		return fmt.Errorf("oracle observation is future-dated or stale")
	}
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	bc.EnsureRuntimeState()
	if bc.OracleObservations[obs.Asset] == nil {
		bc.OracleObservations[obs.Asset] = make(map[string]OracleObservation)
	}
	prior := bc.OracleObservations[obs.Asset][obs.Source]
	if prior.Timestamp > obs.Timestamp {
		return fmt.Errorf("oracle timestamp regression")
	}
	bc.OracleObservations[obs.Asset][obs.Source] = obs
	bc.persistRuntimeStateLocked()
	return nil
}

func medianFloat(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sort.Float64s(values)
	if len(values)%2 == 1 {
		return values[len(values)/2]
	}
	return (values[len(values)/2-1] + values[len(values)/2]) / 2
}

func (bc *Blockchain_struct) MedianOraclePrice(asset string, now int64) MedianOraclePrice {
	asset = strings.ToUpper(strings.TrimSpace(asset))
	if now == 0 {
		now = time.Now().Unix()
	}
	out := MedianOraclePrice{Asset: asset}
	if bc == nil {
		return out
	}
	bc.EnsureRuntimeState()
	bySource := bc.OracleObservations[asset]
	values := make([]float64, 0, len(bySource))
	fresh := make([]OracleObservation, 0, len(bySource))
	for _, obs := range bySource {
		if obs.PriceUSD <= 0 || obs.Confidence <= 0 || now-obs.Timestamp > OracleMaxAgeSeconds || obs.Timestamp > now+30 {
			out.RejectCount++
			continue
		}
		values = append(values, obs.PriceUSD)
		fresh = append(fresh, obs)
	}
	if len(fresh) == 0 {
		out.Stale = len(bySource) > 0
		return out
	}
	median := medianFloat(append([]float64(nil), values...))
	acceptedPrices := []float64{}
	confidence := 0.0
	for _, obs := range fresh {
		deviation := math.Abs(obs.PriceUSD-median) / median * 10000
		if deviation > float64(OracleMaxDeviationBPS) {
			out.RejectCount++
			continue
		}
		acceptedPrices = append(acceptedPrices, obs.PriceUSD)
		confidence += obs.Confidence
		out.Sources = append(out.Sources, obs.Source)
		if obs.Timestamp > out.UpdatedAt {
			out.UpdatedAt = obs.Timestamp
		}
	}
	sort.Strings(out.Sources)
	if len(acceptedPrices) < OracleMinSources {
		return out
	}
	out.PriceUSD = medianFloat(acceptedPrices)
	out.Confidence = confidence / float64(len(acceptedPrices))
	out.Valid = out.Confidence >= 0.5
	return out
}

func (bc *Blockchain_struct) recordPoolPriceObservation(pairAddress string, price float64, timestamp int64) {
	if bc == nil || price <= 0 {
		return
	}
	pairAddress = strings.ToLower(strings.TrimSpace(pairAddress))
	if timestamp == 0 {
		timestamp = time.Now().Unix()
	}
	bc.EnsureRuntimeState()
	history := append(bc.PoolPriceHistory[pairAddress], PoolPriceObservation{PairAddress: pairAddress, Price: price, Timestamp: timestamp})
	if len(history) > 256 {
		history = history[len(history)-256:]
	}
	bc.PoolPriceHistory[pairAddress] = history
}

func priceVolatility(history []PoolPriceObservation) float64 {
	if len(history) < 3 {
		return 0
	}
	returns := make([]float64, 0, len(history)-1)
	for i := 1; i < len(history); i++ {
		if history[i-1].Price <= 0 || history[i].Price <= 0 {
			continue
		}
		returns = append(returns, math.Log(history[i].Price/history[i-1].Price))
	}
	if len(returns) < 2 {
		return 0
	}
	mean := 0.0
	for _, r := range returns {
		mean += r
	}
	mean /= float64(len(returns))
	variance := 0.0
	for _, r := range returns {
		variance += (r - mean) * (r - mean)
	}
	return math.Sqrt(variance / float64(len(returns)-1))
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func (bc *Blockchain_struct) AssessLiquidityQuality(pairAddress string) LiquidityQuality {
	return bc.AssessLiquidityQualityAt(pairAddress, time.Now().Unix())
}

// AssessLiquidityQualityAt makes epoch scoring reproducible across validators
// by evaluating oracle freshness against the finalized block timestamp.
func (bc *Blockchain_struct) AssessLiquidityQualityAt(pairAddress string, evaluationUnix int64) LiquidityQuality {
	if evaluationUnix <= 0 {
		evaluationUnix = time.Now().Unix()
	}
	out := LiquidityQuality{PairAddress: strings.ToLower(strings.TrimSpace(pairAddress))}
	if bc == nil || bc.ContractEngine == nil || out.PairAddress == "" {
		out.CircuitBroken = true
		out.CircuitBreakReason = "pair or contract engine unavailable"
		return out
	}
	storage, err := bc.ContractEngine.DB.LoadAllStorage(out.PairAddress)
	if err != nil || storage == nil {
		out.CircuitBroken = true
		out.CircuitBreakReason = "pair storage unavailable"
		return out
	}
	r0 := parseBigStr(storage["reserve0"])
	r1 := parseBigStr(storage["reserve1"])
	if r0.Sign() <= 0 || r1.Sign() <= 0 {
		out.CircuitBroken = true
		out.CircuitBreakReason = "empty reserves"
		return out
	}
	t0 := bc.validatorTokenSymbol(storage["token0"])
	t1 := bc.validatorTokenSymbol(storage["token1"])
	p0 := bc.MedianOraclePrice(t0, evaluationUnix)
	p1 := bc.MedianOraclePrice(t1, evaluationUnix)
	price0 := p0.PriceUSD
	price1 := p1.PriceUSD
	if !p0.Valid {
		price0 = validatorTokenPriceUSD(t0)
	}
	if !p1.Valid {
		price1 = validatorTokenPriceUSD(t1)
	}
	dec0 := bc.validatorTokenDecimals(storage["token0"])
	dec1 := bc.validatorTokenDecimals(storage["token1"])
	r0f, _ := r0.Float64()
	r1f, _ := r1.Float64()
	out.DepthUSD = r0f/math.Pow10(dec0)*price0 + r1f/math.Pow10(dec1)*price1
	out.ExecutableDepthUSD = out.DepthUSD * 0.02
	out.DepthScore = clamp01(1 - math.Exp(-out.ExecutableDepthUSD/50000))

	volume := parseBigStr(storage["epoch_volume"])
	volF, _ := volume.Float64()
	totalRaw, _ := new(big.Float).SetInt(new(big.Int).Add(r0, r1)).Float64()
	util := 0.0
	if totalRaw > 0 {
		util = volF / totalRaw
	}
	uniqueFlowBPS := parseBigStr(storage["unique_flow_bps"]).Int64()
	if uniqueFlowBPS <= 0 {
		uniqueFlowBPS = 5000
	}
	out.OrganicDemandScore = clamp01(util/0.5) * clamp01(float64(uniqueFlowBPS)/10000)
	out.Volatility = priceVolatility(bc.PoolPriceHistory[out.PairAddress])
	out.TWAPPrice = bc.PoolTWAP(out.PairAddress, evaluationUnix, 3600)
	out.VolatilityScore = clamp01(math.Exp(-8 * out.Volatility))
	if p0.Valid && p1.Valid {
		out.OracleConfidence = math.Min(p0.Confidence, p1.Confidence)
	} else {
		out.OracleConfidence = 0.35
	}
	largestLPBPS := parseBigStr(storage["largest_lp_share_bps"]).Int64()
	if largestLPBPS <= 0 {
		largestLPBPS = 5000
	}
	out.ConcentrationScore = clamp01(1 - float64(largestLPBPS)/10000)
	out.QualityScore = ScoreLiquidityComponents(DefaultLiquidityQualityWeights(), out.DepthScore, out.OrganicDemandScore, out.VolatilityScore, out.OracleConfidence, out.ConcentrationScore)
	out.QualityBPS = int64(math.Round(out.QualityScore * 10000))
	out.Valid = out.DepthUSD > 0
	if out.Volatility > LiquidityCircuitVolatility {
		out.CircuitBroken = true
		out.CircuitBreakReason = "price volatility exceeds circuit threshold"
	}
	if p0.Stale || p1.Stale {
		out.CircuitBroken = true
		out.CircuitBreakReason = "oracle data is stale"
	}
	if out.CircuitBroken {
		out.QualityScore = 0
		out.QualityBPS = 0
	}
	return out
}

func (bc *Blockchain_struct) LiquidityQualityStatus() map[string]interface{} {
	if bc == nil {
		return map[string]interface{}{"pools": []LiquidityQuality{}}
	}
	qualities := []LiquidityQuality{}
	if bc.ContractEngine != nil {
		for _, address := range bc.ContractEngine.DB.ListContractAddresses() {
			storage, _ := bc.ContractEngine.DB.LoadAllStorage(address)
			if storage["token0"] != "" && storage["token1"] != "" {
				qualities = append(qualities, bc.AssessLiquidityQuality(address))
			}
		}
	}
	return map[string]interface{}{
		"pools":                    qualities,
		"oracle_min_sources":       OracleMinSources,
		"oracle_max_age_seconds":   OracleMaxAgeSeconds,
		"oracle_max_deviation_bps": OracleMaxDeviationBPS,
	}
}
