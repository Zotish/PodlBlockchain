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
	MaxPriceBonus   int64   = 20   // added to weight when price deviates
	PriceDeviationPct float64 = 0.02 // 2 % gap triggers bonus
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
	UtilScore    float64
	DemandWeight int64

	// Strategy 2 — price
	ImpliedPrice float64 // reserve1 / reserve0 (token0 price in token1 units)
	PriceBonus   int64

	// Strategy 3 — time (applied as a multiplier to the combined score)
	TimeMultiplier float64

	// Final
	RoutingWeight int64
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
	if !e.shouldRun(blockNumber) {
		return
	}

	metrics := e.scanPairs(bc)
	if len(metrics) == 0 {
		return
	}

	// ── Strategy 1: Demand-based ──────────────────────────────────────────────
	e.applyDemandStrategy(metrics)

	// ── Strategy 2: Price-based ───────────────────────────────────────────────
	e.applyPriceStrategy(metrics)

	// ── Strategy 3: Time-based ────────────────────────────────────────────────
	e.applyTimeStrategy(metrics)

	// ── Combine into final weight ─────────────────────────────────────────────
	e.combineFinalWeights(metrics)

	// ── Write routing weights to contract storage ────────────────────────────
	updated := e.applyWeights(bc, metrics)
	e.resetEpochCounters(bc, metrics)

	log.Printf("🔄 DLEngine #%d — updated %d pair(s) | time=%s",
		blockNumber, updated, currentTimeWindow())

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
		byToken[strings.ToLower(m.Token0)] = append(byToken[strings.ToLower(m.Token0)], priceEntry{i, m.ImpliedPrice})
		byToken[strings.ToLower(m.Token1)] = append(byToken[strings.ToLower(m.Token1)], priceEntry{i, 1.0/m.ImpliedPrice})
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
	multiplier := timeMultiplier()
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
		m.RoutingWeight = w
	}
}

// ── Storage I/O ───────────────────────────────────────────────────────────────

// scanPairs reads every deployed contract and returns metrics for DEX pairs
// identified by having "token0" and "token1" in their contract storage.
func (e *DynamicLiquidityEngine) scanPairs(bc *Blockchain_struct) []PoolMetrics {
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

		totalReserve := new(big.Int).Add(r0, r1)
		utilScore := 0.0
		if totalReserve.Sign() > 0 {
			vF, _ := new(big.Float).SetInt(vol).Float64()
			rF, _ := new(big.Float).SetInt(totalReserve).Float64()
			if rF > 0 {
				utilScore = vF / rF
			}
		}

		out = append(out, PoolMetrics{
			PairAddress: addr,
			Token0:      t0,
			Token1:      t1,
			Reserve0:    r0,
			Reserve1:    r1,
			SwapCount:   swaps.Uint64(),
			VolumeIn:    vol,
			UtilScore:   utilScore,
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
	}
}

// ── Internal helpers ──────────────────────────────────────────────────────────

func (e *DynamicLiquidityEngine) shouldRun(blockNumber uint64) bool {
	return e.EpochBlocks > 0 && blockNumber > 0 && blockNumber%e.EpochBlocks == 0
}

// timeMultiplier returns the time-based multiplier for the current UTC hour.
func timeMultiplier() float64 {
	hour := time.Now().UTC().Hour()
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
	hour := time.Now().UTC().Hour()
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
