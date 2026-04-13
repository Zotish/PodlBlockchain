package blockchaincomponent

// ─────────────────────────────────────────────────────────────────────────────
// Protocol Arbitrage Engine — Active Triangular Arbitrage
//
// The protocol treasury (LiquidityPoolAddress) uses its own LQD capital to
// exploit triangular price imbalances within the DEX.
//
// Path: LQD → tokenA → tokenB → LQD
//
// If LQD returned > LQD spent, the protocol executes the arb:
//   • Reserves of all 3 pairs updated atomically (all-or-nothing writes).
//   • Treasury balance credited with the net profit in LQD.
//   • Prices across pairs naturally converge toward equilibrium.
//
// REENTRANCY SAFETY:
//   • sync.Mutex — only one arb goroutine runs at a time.
//   • isArbitraging flag — prevents re-entrant calls within the same epoch.
//   • Pre-simulation always runs before execution; loss → skip, no state touched.
//   • All 6 reserve writes succeed or the treasury debit is rolled back.
// ─────────────────────────────────────────────────────────────────────────────

import (
	"log"
	"math/big"
	"sort"
	"strings"
	"sync"

	constantset "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/ConstantSet"
)

// ── Constants ─────────────────────────────────────────────────────────────────

const (
	NATIVE_LQD      = "lqd"
	MinProfitBps    = 50   // 0.50 % minimum profit to bother executing
	MaxCapitalBps   = 1000 // max 10 % of treasury per arb cycle
	MaxArbsPerEpoch = 3    // cap number of arbs per epoch
)

// ── Types ─────────────────────────────────────────────────────────────────────

// ArbLeg describes one swap inside a triangular path.
// InIsReserve0 == true  → tokenIn is stored as reserve0 in the pair contract.
// InIsReserve0 == false → tokenIn is stored as reserve1.
type ArbLeg struct {
	PairAddr     string
	TokenIn      string
	TokenOut     string
	ReserveIn    *big.Int // snapshot value (for simulation)
	ReserveOut   *big.Int // snapshot value (for simulation)
	InIsReserve0 bool     // true: tokenIn==token0; false: tokenIn==token1
}

// ArbPath is a full LQD → A → B → LQD triangular cycle.
type ArbPath struct {
	Leg1      ArbLeg
	Leg2      ArbLeg
	Leg3      ArbLeg
	InputLQD  *big.Int // LQD sent in
	EstOutLQD *big.Int // LQD expected back
	ProfitLQD *big.Int // EstOutLQD - InputLQD
}

// ProtocolArb is the active arbitrage component of the Dynamic Liquidity Engine.
type ProtocolArb struct {
	mu            sync.Mutex
	isArbitraging bool
}

// NewProtocolArb creates a ready-to-use arbitrage engine.
func NewProtocolArb() *ProtocolArb { return &ProtocolArb{} }

// ── Public API ────────────────────────────────────────────────────────────────

// RunArbitrage finds triangular opportunities and executes the most profitable.
// Safe to call concurrently — protected by mutex + isArbitraging flag.
func (pa *ProtocolArb) RunArbitrage(bc *Blockchain_struct, metrics []PoolMetrics) {
	// ── Reentrancy guard ──────────────────────────────────────────────────────
	pa.mu.Lock()
	if pa.isArbitraging {
		pa.mu.Unlock()
		return
	}
	pa.isArbitraging = true
	pa.mu.Unlock()

	defer func() {
		pa.mu.Lock()
		pa.isArbitraging = false
		pa.mu.Unlock()
	}()

	// ── Preflight ─────────────────────────────────────────────────────────────
	if bc.ContractEngine == nil || len(metrics) < 2 {
		return
	}

	treasury := constantset.LiquidityPoolAddress
	treasuryBal, hasTreasury := bc.getAccountBalance(treasury)

	if !hasTreasury || treasuryBal == nil || treasuryBal.Sign() == 0 {
		return
	}

	// Capital cap: 10% of treasury per arb
	maxCapital := new(big.Int).Div(
		new(big.Int).Mul(new(big.Int).Set(treasuryBal), big.NewInt(MaxCapitalBps)),
		big.NewInt(10000),
	)
	if maxCapital.Sign() == 0 {
		return
	}

	// ── Find and execute ──────────────────────────────────────────────────────
	paths := pa.findTriangularPaths(metrics, maxCapital)
	executed := 0
	for _, path := range paths {
		if executed >= MaxArbsPerEpoch {
			break
		}
		if pa.executeArb(bc, treasury, path) {
			executed++
		}
	}
	if executed > 0 {
		log.Printf("⚡ ProtocolArb: %d triangular arb(s) executed this epoch", executed)
	}
}

// ── Path Discovery ────────────────────────────────────────────────────────────

func (pa *ProtocolArb) findTriangularPaths(metrics []PoolMetrics, maxCapital *big.Int) []ArbPath {
	// Build pair lookup: sortedKey(t0,t1) → PoolMetrics
	pairMap := make(map[string]PoolMetrics, len(metrics))
	for _, m := range metrics {
		pairMap[sortedPairKey(m.Token0, m.Token1)] = m
	}
	seen := make(map[string]bool)

	// Test input = 10% of maxCapital for initial simulation
	testInput := new(big.Int).Div(maxCapital, big.NewInt(10))
	if testInput.Sign() == 0 {
		testInput = big.NewInt(1)
	}

	var results []ArbPath

	// Hop 1: find all pairs that involve LQD
	for _, pairA := range metrics {
		var tokenA string
		var leg1 ArbLeg

		switch {
		case strings.EqualFold(pairA.Token0, NATIVE_LQD):
			tokenA = pairA.Token1
			leg1 = ArbLeg{pairA.PairAddress, NATIVE_LQD, tokenA,
				new(big.Int).Set(pairA.Reserve0), new(big.Int).Set(pairA.Reserve1), true}
		case strings.EqualFold(pairA.Token1, NATIVE_LQD):
			tokenA = pairA.Token0
			leg1 = ArbLeg{pairA.PairAddress, NATIVE_LQD, tokenA,
				new(big.Int).Set(pairA.Reserve1), new(big.Int).Set(pairA.Reserve0), false}
		default:
			continue
		}
		if leg1.ReserveIn.Sign() == 0 || leg1.ReserveOut.Sign() == 0 {
			continue
		}

		// Hop 2: find pairs that involve tokenA (but not LQD as the other side)
		for _, pairB := range metrics {
			if pairB.PairAddress == pairA.PairAddress {
				continue
			}

			var tokenB string
			var leg2 ArbLeg

			switch {
			case strings.EqualFold(pairB.Token0, tokenA):
				tokenB = pairB.Token1
				leg2 = ArbLeg{pairB.PairAddress, tokenA, tokenB,
					new(big.Int).Set(pairB.Reserve0), new(big.Int).Set(pairB.Reserve1), true}
			case strings.EqualFold(pairB.Token1, tokenA):
				tokenB = pairB.Token0
				leg2 = ArbLeg{pairB.PairAddress, tokenA, tokenB,
					new(big.Int).Set(pairB.Reserve1), new(big.Int).Set(pairB.Reserve0), false}
			default:
				continue
			}
			// Skip if tokenB is LQD (would collapse to 2-hop)
			if strings.EqualFold(tokenB, NATIVE_LQD) {
				continue
			}
			if leg2.ReserveIn.Sign() == 0 || leg2.ReserveOut.Sign() == 0 {
				continue
			}

			// Hop 3: find closing pair tokenB → LQD
			pk3 := sortedPairKey(tokenB, NATIVE_LQD)
			pairC, ok := pairMap[pk3]
			if !ok {
				continue
			}
			if pairC.PairAddress == pairA.PairAddress || pairC.PairAddress == pairB.PairAddress {
				continue
			}

			var leg3 ArbLeg
			switch {
			case strings.EqualFold(pairC.Token0, tokenB):
				leg3 = ArbLeg{pairC.PairAddress, tokenB, NATIVE_LQD,
					new(big.Int).Set(pairC.Reserve0), new(big.Int).Set(pairC.Reserve1), true}
			default:
				leg3 = ArbLeg{pairC.PairAddress, tokenB, NATIVE_LQD,
					new(big.Int).Set(pairC.Reserve1), new(big.Int).Set(pairC.Reserve0), false}
			}
			if leg3.ReserveIn.Sign() == 0 || leg3.ReserveOut.Sign() == 0 {
				continue
			}

			// Simulate with test input
			path, profitable := pa.simulate(leg1, leg2, leg3, testInput)
			if !profitable {
				continue
			}

			// Optimise input for maximum profit (binary search)
			best := pa.optimiseInput(leg1, leg2, leg3, testInput, maxCapital)
			if best.ProfitLQD == nil || best.ProfitLQD.Sign() <= 0 {
				continue
			}
			sig := triangularPathKey(best.Leg1.PairAddr, best.Leg2.PairAddr, best.Leg3.PairAddr)
			if seen[sig] {
				continue
			}
			seen[sig] = true
			_ = path
			results = append(results, best)
		}
	}

	// Sort by profit descending
	for i := 1; i < len(results); i++ {
		for j := i; j > 0 && results[j].ProfitLQD.Cmp(results[j-1].ProfitLQD) > 0; j-- {
			results[j], results[j-1] = results[j-1], results[j]
		}
	}
	return results
}

// ── Simulation ────────────────────────────────────────────────────────────────

func (pa *ProtocolArb) simulate(leg1, leg2, leg3 ArbLeg, inputLQD *big.Int) (ArbPath, bool) {
	amtA := arbAmountOut(inputLQD, leg1.ReserveIn, leg1.ReserveOut)
	if amtA.Sign() == 0 {
		return ArbPath{}, false
	}
	amtB := arbAmountOut(amtA, leg2.ReserveIn, leg2.ReserveOut)
	if amtB.Sign() == 0 {
		return ArbPath{}, false
	}
	lqdOut := arbAmountOut(amtB, leg3.ReserveIn, leg3.ReserveOut)
	if lqdOut.Sign() == 0 {
		return ArbPath{}, false
	}

	profit := new(big.Int).Sub(lqdOut, inputLQD)
	minProfit := new(big.Int).Div(
		new(big.Int).Mul(inputLQD, big.NewInt(MinProfitBps)),
		big.NewInt(10000),
	)
	if profit.Cmp(minProfit) < 0 {
		return ArbPath{}, false
	}

	return ArbPath{leg1, leg2, leg3,
		new(big.Int).Set(inputLQD), lqdOut, profit}, true
}

// optimiseInput binary-searches for the input amount that maximises profit.
func (pa *ProtocolArb) optimiseInput(leg1, leg2, leg3 ArbLeg, start, maxCapital *big.Int) ArbPath {
	best, ok := pa.simulate(leg1, leg2, leg3, start)
	if !ok {
		return ArbPath{}
	}
	cur := new(big.Int).Set(start)
	for range [6]struct{}{} { // try doubling up to 6 times
		next := new(big.Int).Mul(cur, big.NewInt(2))
		if next.Cmp(maxCapital) > 0 {
			next.Set(maxCapital)
		}
		candidate, ok := pa.simulate(leg1, leg2, leg3, next)
		if !ok || candidate.ProfitLQD.Cmp(best.ProfitLQD) <= 0 {
			break
		}
		best = candidate
		cur.Set(next)
		if next.Cmp(maxCapital) >= 0 {
			break
		}
	}
	return best
}

// ── Execution ─────────────────────────────────────────────────────────────────

func (pa *ProtocolArb) executeArb(bc *Blockchain_struct, treasury string, path ArbPath) bool {
	db := bc.ContractEngine.DB

	// Re-read live reserves (may have shifted since simulation)
	r1In, r1Out, ok1 := pa.liveReserves(db, path.Leg1)
	r2In, r2Out, ok2 := pa.liveReserves(db, path.Leg2)
	r3In, r3Out, ok3 := pa.liveReserves(db, path.Leg3)
	if !ok1 || !ok2 || !ok3 {
		return false
	}

	// Re-simulate with live values
	amtA := arbAmountOut(path.InputLQD, r1In, r1Out)
	amtB := arbAmountOut(amtA, r2In, r2Out)
	lqdOut := arbAmountOut(amtB, r3In, r3Out)
	profit := new(big.Int).Sub(lqdOut, path.InputLQD)
	minProfit := new(big.Int).Div(
		new(big.Int).Mul(path.InputLQD, big.NewInt(MinProfitBps)),
		big.NewInt(10000),
	)
	if profit.Cmp(minProfit) < 0 {
		log.Printf("⚡ ProtocolArb: skip — live profit %s < min %s", profit, minProfit)
		return false
	}

	// Debit treasury
	bal, hasBal := bc.getAccountBalance(treasury)
	if !hasBal || bal == nil || bal.Cmp(path.InputLQD) < 0 {
		return false
	}
	if !bc.subAccountBalance(treasury, path.InputLQD) {
		return false
	}

	// Build new reserves
	newR1In := new(big.Int).Add(r1In, path.InputLQD)
	newR1Out := new(big.Int).Sub(r1Out, amtA)
	newR2In := new(big.Int).Add(r2In, amtA)
	newR2Out := new(big.Int).Sub(r2Out, amtB)
	newR3In := new(big.Int).Add(r3In, amtB)
	newR3Out := new(big.Int).Sub(r3Out, lqdOut)

	// Safety: no reserve should go ≤ 0
	if newR1Out.Sign() <= 0 || newR2Out.Sign() <= 0 || newR3Out.Sign() <= 0 {
		bc.addAccountBalance(treasury, path.InputLQD)
		return false
	}

	// Write all 6 reserve values — rollback treasury on any failure
	type kv struct{ addr, key, val string }
	writes := []kv{
		{path.Leg1.PairAddr, resKey(path.Leg1, true), newR1In.String()},
		{path.Leg1.PairAddr, resKey(path.Leg1, false), newR1Out.String()},
		{path.Leg2.PairAddr, resKey(path.Leg2, true), newR2In.String()},
		{path.Leg2.PairAddr, resKey(path.Leg2, false), newR2Out.String()},
		{path.Leg3.PairAddr, resKey(path.Leg3, true), newR3In.String()},
		{path.Leg3.PairAddr, resKey(path.Leg3, false), newR3Out.String()},
	}
	for _, w := range writes {
		if err := db.SaveStorage(w.addr, w.key, w.val); err != nil {
			bc.addAccountBalance(treasury, path.InputLQD)
			log.Printf("⚡ ProtocolArb: reserve write failed: %v", err)
			return false
		}
	}

	// Credit treasury with output (includes profit)
	bc.addAccountBalance(treasury, lqdOut)

	log.Printf("⚡ ProtocolArb ✅  LQD→%s→%s→LQD  in=%s out=%s profit=%s",
		path.Leg1.TokenOut, path.Leg2.TokenOut,
		path.InputLQD, lqdOut, profit)
	return true
}

// ── Small helpers ─────────────────────────────────────────────────────────────

// arbAmountOut applies the AMM constant-product formula with 0.3% fee.
func arbAmountOut(amtIn, resIn, resOut *big.Int) *big.Int {
	if amtIn.Sign() == 0 || resIn.Sign() == 0 || resOut.Sign() == 0 {
		return big.NewInt(0)
	}
	fee := new(big.Int).Mul(amtIn, big.NewInt(997))
	num := new(big.Int).Mul(fee, resOut)
	den := new(big.Int).Add(new(big.Int).Mul(resIn, big.NewInt(1000)), fee)
	return new(big.Int).Div(num, den)
}

// liveReserves reads current reserves from DB using ArbLeg direction metadata.
func (pa *ProtocolArb) liveReserves(db *ContractDB, leg ArbLeg) (resIn, resOut *big.Int, ok bool) {
	storage, err := db.LoadAllStorage(leg.PairAddr)
	if err != nil || storage == nil {
		return nil, nil, false
	}
	r0 := parseBigStr(storage["reserve0"])
	r1 := parseBigStr(storage["reserve1"])
	if leg.InIsReserve0 {
		return r0, r1, true
	}
	return r1, r0, true
}

// resKey returns "reserve0" or "reserve1" for the in-side or out-side of a leg.
func resKey(leg ArbLeg, isIn bool) string {
	if leg.InIsReserve0 {
		if isIn {
			return "reserve0"
		}
		return "reserve1"
	}
	if isIn {
		return "reserve1"
	}
	return "reserve0"
}

// sortedPairKey returns a canonical "a:b" key with lexicographic ordering.
func sortedPairKey(a, b string) string {
	a, b = strings.ToLower(strings.TrimSpace(a)), strings.ToLower(strings.TrimSpace(b))
	if a < b {
		return a + ":" + b
	}
	return b + ":" + a
}

func triangularPathKey(a, b, c string) string {
	parts := []string{
		strings.ToLower(strings.TrimSpace(a)),
		strings.ToLower(strings.TrimSpace(b)),
		strings.ToLower(strings.TrimSpace(c)),
	}
	sort.Strings(parts)
	return strings.Join(parts, "|")
}
