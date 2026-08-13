package blockchaincomponent

// ─────────────────────────────────────────────────────────────────────────────
// Dynamic Liquidity Engine — core PosDL innovation
//
// Three strategies run every epoch and together produce a final routing_weight
// for every DEX pair. The factory router uses this weight to prefer high-value
// paths when building swap routes. No physical reserves ever move.
//
// STRATEGY 1 — DEMAND-BASED  (weight 0-100)
//   utilScore = epochVolume / totalReserves
//   High volume relative to reserves → high weight → more swap traffic routed here.
//
// STRATEGY 2 — PRICE-BASED   (bonus ±20)
//   Compares the implied price of a shared token across all pairs.
//   A pair whose price deviates from the median gets a weight boost so
//   arbitrageurs are naturally directed there, closing the gap.
//
// STRATEGY 3 — TIME-BASED    (multiplier 0.7 – 1.2)
//   Uses the block timestamp to estimate time-of-day UTC.
//   Off-peak hours (00:00–08:00 UTC) → consolidate to fewer pairs (×0.7 floor).
//   Peak hours    (08:00–20:00 UTC) → distribute across all pairs  (×1.2 ceiling).
//
// Final weight = clamp(demandWeight + priceBonus, 1, 100) × timeMultiplier
//
// KEY SAFETY PROPERTIES:
//   ✅  k = x*y invariant is NEVER broken (reserves are never touched)
//   ✅  LP providers keep 100 % of their tokens at all times
//   ✅  Validator LP locks are completely unaffected
//   ✅  Privileged protocol operation — costs users zero gas
// ─────────────────────────────────────────────────────────────────────────────

import (
	"fmt"
	"log"
	"math"
	"math/big"
	"sort"
	"strings"
	"time"
)

// ── Constants ────────────────────────────────────────────────────────────────

const (
	DefaultEpochBlocks   uint64  = 100  // run engine every 100 blocks
	DefaultLowThreshold  float64 = 0.10 // utilScore below → low demand
	DefaultHighThreshold float64 = 0.60 // utilScore above → peak demand
	MinRoutingWeight     int64   = 10   // floor weight (always routable)
	MaxRoutingWeight     int64   = 100  // ceiling weight

	// Time-based thresholds (UTC hour)
	OffPeakStart = 0  // 00:00 UTC — consolidation window begins
	OffPeakEnd   = 8  // 08:00 UTC — consolidation window ends
	PeakStart    = 8  // 08:00 UTC — distribution window begins
	PeakEnd      = 20 // 20:00 UTC — distribution window ends

	// Strategy weights
	OffPeakMultiplier float64 = 0.70 // quiet hours: reduce spread
	PeakMultiplier    float64 = 1.20 // busy hours:  increase spread
	NormalMultiplier  float64 = 1.00 // transition hours

	// Price-based bonus range
	MaxPriceBonus     int64   = 20   // added to weight when price deviates
	PriceDeviationPct float64 = 0.02 // 2 % gap triggers bonus

	MaxRoutingWeightStep int64 = 25
	MaxOracleAgeSeconds  int64 = 3600
)

// ── Types ─────────────────────────────────────────────────────────────────────

// PoolMetrics holds a complete snapshot of one DEX pair after an epoch.
type PoolMetrics struct {
	PairAddress string
	Token0      string
	Token1      string
	Reserve0    *big.Int
	Reserve1    *big.Int
	SwapCount   uint64
	VolumeIn    *big.Int

	// Strategy 1 — demand
	UtilScore       float64
	DemandWeight    int64
	OracleDemandBps int64
	OracleSource    string

	// Strategy 2 — price
	ImpliedPrice float64 // reserve1 / reserve0 (token0 price in token1 units)
	TWAPPrice    float64
	PriceBonus   int64

	// Strategy 3 — time (applied as a multiplier to the combined score)
	TimeMultiplier float64

	// Final
	ExistingRoutingWeight int64
	RoutingWeight         int64
	SafetyCapped          bool
	LiquidityQuality      float64
	QualityBPS            int64
	CircuitBroken         bool
	CircuitBreakReason    string
	RiskClass             string
	CorrelatedGroup       string
	MaxExposureBPS        int64
}

// DynamicLiquidityEngine is the protocol-level routing optimiser.
type DynamicLiquidityEngine struct {
	EpochBlocks   uint64
	LowThreshold  float64
	HighThreshold float64
	Arb           *ProtocolArb // active triangular arbitrage engine
}

// NewDynamicLiquidityEngine returns a ready-to-use engine with defaults.
func NewDynamicLiquidityEngine() *DynamicLiquidityEngine {
	return &DynamicLiquidityEngine{
		EpochBlocks:   DefaultEpochBlocks,
		LowThreshold:  DefaultLowThreshold,
		HighThreshold: DefaultHighThreshold,
		Arb:           NewProtocolArb(),
	}
}

// ── Public API ────────────────────────────────────────────────────────────────

// RunEpoch is called from MineNewBlock after every block.
// It is a no-op unless blockNumber is exactly on an epoch boundary.
func (e *DynamicLiquidityEngine) RunEpoch(bc *Blockchain_struct, blockNumber uint64) {
	evaluationUnix := time.Now().Unix()
	if bc != nil {
		for _, block := range bc.Blocks {
			if block != nil && block.BlockNumber == blockNumber {
				evaluationUnix = int64(block.TimeStamp)
				break
			}
		}
	}
	e.RunEpochAt(bc, blockNumber, evaluationUnix)
}

func (e *DynamicLiquidityEngine) RunEpochAt(bc *Blockchain_struct, blockNumber uint64, evaluationUnix int64) {
	if !e.shouldRun(blockNumber) {
		return
	}
	e.runAt(bc, blockNumber, "scheduled", evaluationUnix)
}

func (e *DynamicLiquidityEngine) RunNow(bc *Blockchain_struct, reason string) []PoolMetrics {
	return e.runAt(bc, bc.LatestBlockNumber(), reason, time.Now().Unix())
}

func (e *DynamicLiquidityEngine) Preview(bc *Blockchain_struct) []PoolMetrics {
	return e.PreviewAt(bc, time.Now().Unix())
}

func (e *DynamicLiquidityEngine) PreviewAt(bc *Blockchain_struct, evaluationUnix int64) []PoolMetrics {
	metrics := e.scanPairsAt(bc, evaluationUnix)
	if len(metrics) == 0 {
		return nil
	}
	e.applyDemandStrategy(metrics)
	e.applyPriceStrategy(metrics)
	e.applyLearnedTimeStrategyAt(bc, metrics, evaluationUnix)
	e.combineFinalWeights(metrics)
	e.applyExposurePolicy(metrics)
	return metrics
}

func (bc *Blockchain_struct) SetDynamicLiquidityOracleSignal(pairAddress string, demandBps int64, source string) (DynamicLiquidityOracleSignal, error) {
	if bc == nil {
		return DynamicLiquidityOracleSignal{}, fmt.Errorf("nil blockchain")
	}
	if bc.strictConsensusMode() {
		return DynamicLiquidityOracleSignal{}, fmt.Errorf("direct demand signals are disabled in strict consensus mode")
	}
	pairAddress = strings.ToLower(strings.TrimSpace(pairAddress))
	source = strings.TrimSpace(source)
	if pairAddress == "" {
		return DynamicLiquidityOracleSignal{}, fmt.Errorf("pair_address is required")
	}
	if demandBps < 0 || demandBps > 10000 {
		return DynamicLiquidityOracleSignal{}, fmt.Errorf("demand_bps must be between 0 and 10000")
	}
	if source == "" {
		source = "admin"
	}
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	bc.EnsureRuntimeState()
	signal := DynamicLiquidityOracleSignal{
		PairAddress: pairAddress,
		DemandBps:   demandBps,
		Source:      source,
		UpdatedAt:   time.Now().Unix(),
	}
	bc.DynamicLiquidityOracleSignals[pairAddress] = signal
	bc.persistRuntimeStateLocked()
	return signal, nil
}

func (bc *Blockchain_struct) DynamicLiquidityOracleSnapshot() []DynamicLiquidityOracleSignal {
	if bc == nil {
		return nil
	}
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	bc.EnsureRuntimeState()
	out := make([]DynamicLiquidityOracleSignal, 0, len(bc.DynamicLiquidityOracleSignals))
	for _, signal := range bc.DynamicLiquidityOracleSignals {
		out = append(out, signal)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].UpdatedAt > out[j].UpdatedAt
	})
	return out
}

func (bc *Blockchain_struct) RunDynamicLiquidityEpochNow(reason string) []map[string]interface{} {
	if bc == nil {
		return nil
	}
	if bc.strictConsensusMode() {
		return nil
	}
	if bc.DLEngine == nil {
		bc.DLEngine = NewDynamicLiquidityEngine()
	}
	metrics := bc.DLEngine.RunNow(bc, reason)
	return dynamicLiquidityMetricRows(metrics)
}

func (bc *Blockchain_struct) DynamicLiquidityStatus() map[string]interface{} {
	if bc == nil {
		return map[string]interface{}{"metrics": []map[string]interface{}{}}
	}
	if bc.DLEngine == nil {
		bc.DLEngine = NewDynamicLiquidityEngine()
	}
	metrics := bc.DLEngine.Preview(bc)
	return map[string]interface{}{
		"metrics":                dynamicLiquidityMetricRows(metrics),
		"oracle_signals":         bc.DynamicLiquidityOracleSnapshot(),
		"safety":                 bc.StrategyVaultSafetySnapshot(),
		"max_weight_step":        MaxRoutingWeightStep,
		"max_oracle_age_seconds": MaxOracleAgeSeconds,
		"timestamp":              time.Now().Unix(),
	}
}

func (bc *Blockchain_struct) dynamicLiquidityOracleSignal(pairAddress string) DynamicLiquidityOracleSignal {
	return bc.dynamicLiquidityOracleSignalAt(pairAddress, time.Now().Unix())
}

func (bc *Blockchain_struct) dynamicLiquidityOracleSignalAt(pairAddress string, evaluationUnix int64) DynamicLiquidityOracleSignal {
	if bc == nil {
		return DynamicLiquidityOracleSignal{}
	}
	if bc.strictConsensusMode() {
		return DynamicLiquidityOracleSignal{}
	}
	bc.EnsureRuntimeState()
	signal := bc.DynamicLiquidityOracleSignals[strings.ToLower(strings.TrimSpace(pairAddress))]
	if signal.UpdatedAt == 0 || evaluationUnix-signal.UpdatedAt > MaxOracleAgeSeconds || signal.UpdatedAt > evaluationUnix+30 {
		return DynamicLiquidityOracleSignal{}
	}
	return signal
}

// strictConsensusMode distinguishes an initialized signed-finality network
// from a zero-value Blockchain_struct used by compatibility callers and unit
// tests. A configured chain can therefore never use the direct/manual DLE
// signal path, while the legacy API remains usable off-chain.
func (bc *Blockchain_struct) strictConsensusMode() bool {
	return bc != nil && bc.ChainSpec.ProtocolVersion != 0 && !bc.ChainSpec.AllowLegacyFinality
}

func (e *DynamicLiquidityEngine) run(bc *Blockchain_struct, blockNumber uint64, reason string) []PoolMetrics {
	return e.runAt(bc, blockNumber, reason, time.Now().Unix())
}

func (e *DynamicLiquidityEngine) runAt(bc *Blockchain_struct, blockNumber uint64, reason string, evaluationUnix int64) []PoolMetrics {
	metrics := e.PreviewAt(bc, evaluationUnix)
	if len(metrics) == 0 {
		return nil
	}
	bc.learnCongestionProfile(metrics, evaluationUnix)

	// ── Write routing weights to contract storage ────────────────────────────
	updated := e.applyWeights(bc, metrics)
	e.resetEpochCounters(bc, metrics)

	log.Printf("🔄 DLEngine #%d — updated %d pair(s) | trigger=%s | time=%s",
		blockNumber, updated, strings.TrimSpace(reason), currentTimeWindowAt(evaluationUnix))

	// ── Strategy 4: Active Protocol Arbitrage ────────────────────────────────
	// Runs AFTER weights are applied so it sees fresh utilisation scores.
	// Uses treasury LQD to exploit triangular price gaps. Reentrancy-safe.
	if e.Arb != nil {
		e.Arb.RunArbitrage(bc, metrics)
	}
	for _, m := range metrics {
		log.Printf("   %s [%s/%s]  util=%.3f demand=%d price±=%d time×%.2f → weight=%d",
			shortAddr(m.PairAddress), m.Token0, m.Token1,
			m.UtilScore, m.DemandWeight, m.PriceBonus, m.TimeMultiplier, m.RoutingWeight)
	}
	return metrics
}

// ── Strategy 1: DEMAND-BASED ──────────────────────────────────────────────────
//
// Pools with high swap volume relative to their depth get a higher weight.
// This routes more future swaps to where the market is most active.

func (e *DynamicLiquidityEngine) applyDemandStrategy(metrics []PoolMetrics) {
	for i := range metrics {
		m := &metrics[i]
		s := m.UtilScore

		var w float64
		switch {
		case s <= 0:
			w = float64(MinRoutingWeight)
		case s >= e.HighThreshold:
			w = float64(MaxRoutingWeight)
		case s <= e.LowThreshold:
			// linear: MinRoutingWeight → 40 over [0, LowThreshold]
			w = float64(MinRoutingWeight) + (s/e.LowThreshold)*30.0
		default:
			// linear: 40 → MaxRoutingWeight over [LowThreshold, HighThreshold]
			ratio := (s - e.LowThreshold) / (e.HighThreshold - e.LowThreshold)
			w = 40.0 + ratio*float64(MaxRoutingWeight-40)
		}

		m.DemandWeight = int64(w)
		if m.OracleDemandBps > 0 {
			oracle := float64(MinRoutingWeight) + (float64(MaxRoutingWeight-MinRoutingWeight) * (float64(m.OracleDemandBps) / 10000.0))
			m.DemandWeight = int64((w * 0.70) + (oracle * 0.30))
		}
	}
}

// ── Strategy 2: PRICE-BASED ───────────────────────────────────────────────────
//
// For every token that appears in multiple pairs, compute a median implied price.
// Pairs whose price deviates > PriceDeviationPct from the median receive a
// PriceBonus — routing more swaps their way closes the arbitrage gap naturally.

func (e *DynamicLiquidityEngine) applyPriceStrategy(metrics []PoolMetrics) {
	// Collect implied price per token across all pairs that share that asset.
	// We score both sides of each pair so sorted token order does not weaken
	// price discovery for a token that commonly appears as token1.
	//
	// map: token → list of (index, impliedPrice)
	byToken := make(map[string][]priceEntry)

	for i := range metrics {
		m := &metrics[i]
		m.PriceBonus = 0

		if m.Reserve0.Sign() == 0 || m.Reserve1.Sign() == 0 {
			m.ImpliedPrice = 0
			continue
		}
		r0f, _ := new(big.Float).SetInt(m.Reserve0).Float64()
		r1f, _ := new(big.Float).SetInt(m.Reserve1).Float64()
		if r0f == 0 || r1f == 0 {
			continue
		}
		m.ImpliedPrice = r1f / r0f
		if m.TWAPPrice > 0 {
			m.ImpliedPrice = m.TWAPPrice
		}
		// Persist a bounded history used by TWAP/volatility quality scoring.
		// The history is consensus-visible and only advances at epoch evaluation.
		// Duplicate timestamps are harmless for the volatility calculation.
		byToken[strings.ToLower(m.Token0)] = append(byToken[strings.ToLower(m.Token0)], priceEntry{i, m.ImpliedPrice})
		byToken[strings.ToLower(m.Token1)] = append(byToken[strings.ToLower(m.Token1)], priceEntry{i, 1.0 / m.ImpliedPrice})
	}

	// For each token that appears in 2+ pairs, find median and apply bonus.
	for _, entries := range byToken {
		if len(entries) < 2 {
			continue // need at least 2 pairs for comparison
		}

		// Sort by price to find median
		sort.Slice(entries, func(a, b int) bool {
			return entries[a].price < entries[b].price
		})
		median := medianPrice(entries)
		if median == 0 {
			continue
		}

		for _, entry := range entries {
			deviation := math.Abs(entry.price-median) / median
			if deviation >= PriceDeviationPct {
				// This pair has a price discrepancy — boost it so arbitrageurs
				// are routed here and the price converges back to median.
				bonus := int64(math.Min(float64(MaxPriceBonus), deviation*float64(MaxPriceBonus)*10))
				metrics[entry.idx].PriceBonus = bonus
				log.Printf("   📈 PriceBonus +%d for %s (deviation %.2f%% from median %.6f)",
					bonus, shortAddr(metrics[entry.idx].PairAddress), deviation*100, median)
			}
		}
	}
}

// ── Strategy 3: TIME-BASED ────────────────────────────────────────────────────
//
// Uses wall-clock UTC time to apply a multiplier:
//   Off-peak  00:00–08:00 UTC → ×0.70  (consolidate weight to top pairs)
//   Peak      08:00–20:00 UTC → ×1.20  (distribute weight broadly)
//   Transition 20:00–24:00    → ×1.00  (neutral)
//
// Effect: during quiet hours, only the strongest pairs get meaningful weight,
// reducing fragmentation. During busy hours all pairs are competitive.

func (e *DynamicLiquidityEngine) applyTimeStrategy(metrics []PoolMetrics) {
	e.applyTimeStrategyAt(metrics, time.Now().Unix())
}

func (e *DynamicLiquidityEngine) applyTimeStrategyAt(metrics []PoolMetrics, evaluationUnix int64) {
	multiplier := timeMultiplierAt(evaluationUnix)
	for i := range metrics {
		metrics[i].TimeMultiplier = multiplier
	}
}

func (e *DynamicLiquidityEngine) applyLearnedTimeStrategyAt(bc *Blockchain_struct, metrics []PoolMetrics, evaluationUnix int64) {
	multiplier := bc.learnedTimeMultiplier(evaluationUnix)
	for i := range metrics {
		metrics[i].TimeMultiplier = multiplier
	}
}

// ── Combine ───────────────────────────────────────────────────────────────────

func (e *DynamicLiquidityEngine) combineFinalWeights(metrics []PoolMetrics) {
	for i := range metrics {
		m := &metrics[i]
		combined := float64(m.DemandWeight+m.PriceBonus) * m.TimeMultiplier
		w := int64(combined)
		if w < MinRoutingWeight {
			w = MinRoutingWeight
		}
		if w > MaxRoutingWeight {
			w = MaxRoutingWeight
		}
		if m.ExistingRoutingWeight > 0 {
			upper := m.ExistingRoutingWeight + MaxRoutingWeightStep
			lower := m.ExistingRoutingWeight - MaxRoutingWeightStep
			if lower < MinRoutingWeight {
				lower = MinRoutingWeight
			}
			if w > upper {
				w = upper
				m.SafetyCapped = true
			}
			if w < lower {
				w = lower
				m.SafetyCapped = true
			}
		}
		m.RoutingWeight = w
		if m.CircuitBroken {
			m.RoutingWeight = MinRoutingWeight
		}
	}
}

func (e *DynamicLiquidityEngine) applyExposurePolicy(metrics []PoolMetrics) {
	groupTotals := map[string]int64{}
	for i := range metrics {
		m := &metrics[i]
		if m.MaxExposureBPS > 0 {
			capWeight := m.MaxExposureBPS / 100
			if capWeight < MinRoutingWeight {
				capWeight = MinRoutingWeight
			}
			if m.RoutingWeight > capWeight {
				m.RoutingWeight, m.SafetyCapped = capWeight, true
			}
		}
		if m.CorrelatedGroup != "" {
			groupTotals[m.CorrelatedGroup] += m.RoutingWeight
		}
	}
	for group, total := range groupTotals {
		if total <= MaxRoutingWeight || total == 0 {
			continue
		}
		for i := range metrics {
			if metrics[i].CorrelatedGroup == group {
				metrics[i].RoutingWeight = maxInt64(MinRoutingWeight, metrics[i].RoutingWeight*MaxRoutingWeight/total)
				metrics[i].SafetyCapped = true
			}
		}
	}
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// ── Storage I/O ───────────────────────────────────────────────────────────────

// scanPairs reads every deployed contract and returns metrics for DEX pairs
// identified by having "token0" and "token1" in their contract storage.
func (e *DynamicLiquidityEngine) scanPairs(bc *Blockchain_struct) []PoolMetrics {
	return e.scanPairsAt(bc, time.Now().Unix())
}

func (e *DynamicLiquidityEngine) scanPairsAt(bc *Blockchain_struct, evaluationUnix int64) []PoolMetrics {
	if bc.ContractEngine == nil {
		return nil
	}

	addrs := bc.ContractEngine.DB.ListContractAddresses()
	var out []PoolMetrics

	for _, addr := range addrs {
		storage, err := bc.ContractEngine.DB.LoadAllStorage(addr)
		if err != nil || storage == nil {
			continue
		}

		t0 := storage["token0"]
		t1 := storage["token1"]
		if t0 == "" || t1 == "" {
			continue
		}

		r0 := parseBigStr(storage["reserve0"])
		r1 := parseBigStr(storage["reserve1"])
		if r0.Sign() == 0 && r1.Sign() == 0 {
			continue
		}

		vol := parseBigStr(storage["epoch_volume"])
		swaps := parseBigStr(storage["epoch_swaps"])
		existingWeight := parseBigStr(storage["routing_weight"]).Int64()

		totalReserve := new(big.Int).Add(r0, r1)
		utilScore := 0.0
		if totalReserve.Sign() > 0 {
			vF, _ := new(big.Float).SetInt(vol).Float64()
			rF, _ := new(big.Float).SetInt(totalReserve).Float64()
			if rF > 0 {
				utilScore = vF / rF
			}
		}

		signal := bc.dynamicLiquidityOracleSignalAt(addr, evaluationUnix)
		quality := bc.AssessLiquidityQualityAt(addr, evaluationUnix)
		if r0.Sign() > 0 {
			r0f, _ := new(big.Float).SetInt(r0).Float64()
			r1f, _ := new(big.Float).SetInt(r1).Float64()
			if r0f > 0 && r1f > 0 {
				bc.recordPoolPriceObservation(addr, r1f/r0f, evaluationUnix)
			}
		}
		policy, routable, policyReason := bc.pairRoutable(addr, bc.LatestBlockNumber(), evaluationUnix)
		if !routable {
			quality.CircuitBroken, quality.CircuitBreakReason, quality.QualityBPS, quality.QualityScore = true, policyReason, 0, 0
		}
		out = append(out, PoolMetrics{
			PairAddress:           addr,
			Token0:                t0,
			Token1:                t1,
			Reserve0:              r0,
			Reserve1:              r1,
			SwapCount:             swaps.Uint64(),
			VolumeIn:              vol,
			UtilScore:             utilScore,
			OracleDemandBps:       signal.DemandBps,
			OracleSource:          signal.Source,
			ExistingRoutingWeight: existingWeight,
			LiquidityQuality:      quality.QualityScore,
			QualityBPS:            quality.QualityBPS,
			CircuitBroken:         quality.CircuitBroken,
			CircuitBreakReason:    quality.CircuitBreakReason,
			TWAPPrice:             quality.TWAPPrice,
			RiskClass:             policy.RiskClass,
			CorrelatedGroup:       policy.CorrelatedGroup,
			MaxExposureBPS:        policy.MaxExposureBPS,
		})
	}
	return out
}

// applyWeights writes routing_weight to each pair's contract storage.
// Returns the number of pairs successfully updated.
func (e *DynamicLiquidityEngine) applyWeights(bc *Blockchain_struct, metrics []PoolMetrics) int {
	db := bc.ContractEngine.DB
	done := 0
	for _, m := range metrics {
		if err := db.SaveStorage(m.PairAddress, "routing_weight", big.NewInt(m.RoutingWeight).String()); err != nil {
			log.Printf("DLEngine: failed to write weight for %s: %v", shortAddr(m.PairAddress), err)
			continue
		}
		done++
	}
	return done
}

// resetEpochCounters clears epoch_swaps / epoch_volume on every scanned pair.
func (e *DynamicLiquidityEngine) resetEpochCounters(bc *Blockchain_struct, metrics []PoolMetrics) {
	db := bc.ContractEngine.DB
	for _, m := range metrics {
		_ = db.SaveStorage(m.PairAddress, "epoch_swaps", "0")
		_ = db.SaveStorage(m.PairAddress, "epoch_volume", "0")
		_ = db.SaveStorage(m.PairAddress, "epoch_organic_volume", "0")
		_ = db.SaveStorage(m.PairAddress, "epoch_unique_traders", "0")
		_ = db.SaveStorage(m.PairAddress, "unique_flow_bps", "0")
		epoch := parseBigStr(mustLoadStorage(db, m.PairAddress, "flow_epoch"))
		_ = db.SaveStorage(m.PairAddress, "flow_epoch", new(big.Int).Add(epoch, big.NewInt(1)).String())
	}
}

func mustLoadStorage(db *ContractDB, address, key string) string {
	value, _ := db.LoadStorage(address, key)
	return value
}

// ── Internal helpers ──────────────────────────────────────────────────────────

func (e *DynamicLiquidityEngine) shouldRun(blockNumber uint64) bool {
	return e.EpochBlocks > 0 && blockNumber > 0 && blockNumber%e.EpochBlocks == 0
}

// timeMultiplier returns the time-based multiplier for the current UTC hour.
func timeMultiplier() float64 {
	return timeMultiplierAt(time.Now().Unix())
}

func timeMultiplierAt(unix int64) float64 {
	hour := time.Unix(unix, 0).UTC().Hour()
	switch {
	case hour >= OffPeakStart && hour < OffPeakEnd:
		return OffPeakMultiplier // 00-08 UTC: consolidate
	case hour >= PeakStart && hour < PeakEnd:
		return PeakMultiplier // 08-20 UTC: distribute
	default:
		return NormalMultiplier // 20-24 UTC: neutral
	}
}

// currentTimeWindow returns a human-readable label for the current time window.
func currentTimeWindow() string {
	return currentTimeWindowAt(time.Now().Unix())
}

func currentTimeWindowAt(unix int64) string {
	hour := time.Unix(unix, 0).UTC().Hour()
	switch {
	case hour >= OffPeakStart && hour < OffPeakEnd:
		return "OFF-PEAK (consolidating)"
	case hour >= PeakStart && hour < PeakEnd:
		return "PEAK (distributing)"
	default:
		return "TRANSITION (neutral)"
	}
}

type priceEntry struct {
	idx   int
	price float64
}

func medianPrice(entries []priceEntry) float64 {
	n := len(entries)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return entries[n/2].price
	}
	return (entries[n/2-1].price + entries[n/2].price) / 2.0
}

// ── Small helpers ─────────────────────────────────────────────────────────────

func parseBigStr(s string) *big.Int {
	s = strings.TrimSpace(s)
	n := new(big.Int)
	if s == "" {
		return n
	}
	n.SetString(s, 10)
	return n
}

func dynamicLiquidityMetricRows(metrics []PoolMetrics) []map[string]interface{} {
	rows := make([]map[string]interface{}, 0, len(metrics))
	for _, m := range metrics {
		reserve0 := "0"
		reserve1 := "0"
		volume := "0"
		if m.Reserve0 != nil {
			reserve0 = m.Reserve0.String()
		}
		if m.Reserve1 != nil {
			reserve1 = m.Reserve1.String()
		}
		if m.VolumeIn != nil {
			volume = m.VolumeIn.String()
		}
		rows = append(rows, map[string]interface{}{
			"pair_address":            strings.ToLower(m.PairAddress),
			"token0":                  m.Token0,
			"token1":                  m.Token1,
			"reserve0":                reserve0,
			"reserve1":                reserve1,
			"swap_count":              m.SwapCount,
			"volume_in":               volume,
			"util_score":              m.UtilScore,
			"demand_weight":           m.DemandWeight,
			"oracle_demand_bps":       m.OracleDemandBps,
			"oracle_source":           m.OracleSource,
			"price_bonus":             m.PriceBonus,
			"twap_price":              m.TWAPPrice,
			"time_multiplier":         m.TimeMultiplier,
			"existing_routing_weight": m.ExistingRoutingWeight,
			"routing_weight":          m.RoutingWeight,
			"safety_capped":           m.SafetyCapped,
			"liquidity_quality":       m.LiquidityQuality,
			"quality_bps":             m.QualityBPS,
			"circuit_broken":          m.CircuitBroken,
			"circuit_break_reason":    m.CircuitBreakReason,
			"risk_class":              m.RiskClass,
			"correlated_group":        m.CorrelatedGroup,
			"max_exposure_bps":        m.MaxExposureBPS,
		})
	}
	return rows
}

func pctOfBig(amount *big.Int, pct float64) *big.Int {
	if amount == nil || amount.Sign() == 0 || pct <= 0 {
		return big.NewInt(0)
	}
	f := new(big.Float).SetInt(amount)
	f.Mul(f, big.NewFloat(pct))
	result, _ := f.Int(nil)
	return result
}

func sharesToken(a, b PoolMetrics) bool {
	return strings.EqualFold(a.Token0, b.Token0) ||
		strings.EqualFold(a.Token0, b.Token1) ||
		strings.EqualFold(a.Token1, b.Token0) ||
		strings.EqualFold(a.Token1, b.Token1)
}

func shortAddr(addr string) string {
	if len(addr) > 10 {
		return addr[:10] + "..."
	}
	return addr
}
