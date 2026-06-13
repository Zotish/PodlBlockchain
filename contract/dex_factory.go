//go:build ignore
// +build ignore

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"strconv"
	"strings"

	bc "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/BlockchainComponent"
)

// ─────────────────────────────────────────────────────────────────────────────
// LQD DEX Factory + Router  (Native-LQD aware)
//
// Use "lqd" as the token address to mean the native LQD coin.
// No wrapping needed — the contract uses ctx.MsgValue() to receive native LQD
// and ctx.SendNative() to send it back.
//
// Examples:
//   CreatePair("lqd", "<M2_ADDR>")
//   AddLiquidity("lqd", "<M2_ADDR>", "500", "500")   tx.value = 500
//   SwapExactTokensForTokens("500","0","lqd","<M2>")  tx.value = 500
//   SwapExactTokensForTokens("500","0","<M2>","lqd")  get native LQD back
//   RemoveLiquidity("lqd", "<M2_ADDR>", "100")        get native LQD + M2 back
//
// Storage layout  (pk = sorted token0:token1)
//   p:{pk}:t0, p:{pk}:t1
//   p:{pk}:r0, p:{pk}:r1   reserves
//   p:{pk}:lp              totalLP
//   p:{pk}:lp:{addr}       LP balance
//   pairCount, pairAt:{n}
//   pairExists:{pk}
//   p:{pk}:vlp:{addr}      validator locked LP
//   p:{pk}:vlu:{addr}      lock-until unix ts
// ─────────────────────────────────────────────────────────────────────────────

// NATIVE is the sentinel address representing the native LQD coin.
const NATIVE = "lqd"

const minLiquidity = int64(1000)

type Factory struct{}

// ─── Math ─────────────────────────────────────────────────────────────────────

func parseBig(v string) *big.Int {
	v = strings.TrimSpace(v)
	if v == "" {
		return big.NewInt(0)
	}
	z := new(big.Int)
	if _, ok := z.SetString(v, 10); !ok {
		return big.NewInt(0)
	}
	return z
}

func normAddr(a string) string {
	a = strings.ToLower(strings.TrimSpace(a))
	if a == NATIVE {
		return NATIVE
	}
	return a
}

func swapReceiver(ctx *bc.Context) string {
	receiver := normAddr(ctx.OriginAddr)
	if receiver == "" {
		receiver = normAddr(ctx.CallerAddr)
	}
	return receiver
}

func isNative(addr string) bool { return addr == NATIVE }

func sqrtBig(n *big.Int) *big.Int {
	if n.Sign() <= 0 {
		return big.NewInt(0)
	}
	x := new(big.Int).Set(n)
	z := new(big.Int).Add(new(big.Int).Rsh(n, 1), big.NewInt(1))
	for z.Cmp(x) < 0 {
		x.Set(z)
		z = new(big.Int).Rsh(new(big.Int).Add(new(big.Int).Div(n, z), z), 1)
	}
	return x
}

func minBig(a, b *big.Int) *big.Int {
	if a.Cmp(b) < 0 {
		return a
	}
	return b
}

func calcAmountOut(amtIn, resIn, resOut *big.Int) *big.Int {
	if amtIn.Sign() == 0 || resIn.Sign() == 0 || resOut.Sign() == 0 {
		return big.NewInt(0)
	}
	fee := new(big.Int).Mul(amtIn, big.NewInt(997))
	num := new(big.Int).Mul(fee, resOut)
	den := new(big.Int).Add(new(big.Int).Mul(resIn, big.NewInt(1000)), fee)
	return new(big.Int).Div(num, den)
}

func calcAmountIn(amtOut, resIn, resOut *big.Int) *big.Int {
	if amtOut.Sign() == 0 || resIn.Sign() == 0 || resOut.Sign() == 0 {
		return big.NewInt(0)
	}
	if amtOut.Cmp(resOut) >= 0 {
		return big.NewInt(0)
	}
	num := new(big.Int).Mul(new(big.Int).Mul(resIn, amtOut), big.NewInt(1000))
	den := new(big.Int).Mul(new(big.Int).Sub(resOut, amtOut), big.NewInt(997))
	return new(big.Int).Add(new(big.Int).Div(num, den), big.NewInt(1))
}

// ─── Pair key ─────────────────────────────────────────────────────────────────

func pairKey(a, b string) (pk, t0, t1 string) {
	a, b = normAddr(a), normAddr(b)
	// "lqd" always sorts before any hex address lexicographically
	if a < b {
		return a + ":" + b, a, b
	}
	return b + ":" + a, b, a
}

func sortPairAmounts(tokenA, tokenB, amountA, amountB string) (string, string) {
	_, t0, _ := pairKey(tokenA, tokenB)
	if normAddr(tokenA) == t0 {
		return amountA, amountB
	}
	return amountB, amountA
}

// ─── Storage helpers ──────────────────────────────────────────────────────────

func (f *Factory) pget(ctx *bc.Context, pk, field string) string {
	return ctx.Get("p:" + pk + ":" + field)
}
func (f *Factory) pset(ctx *bc.Context, pk, field, val string) {
	ctx.Set("p:"+pk+":"+field, val)
}
func (f *Factory) pbig(ctx *bc.Context, pk, field string) *big.Int {
	return parseBig(f.pget(ctx, pk, field))
}
func (f *Factory) psetBig(ctx *bc.Context, pk, field string, v *big.Int) {
	f.pset(ctx, pk, field, v.String())
}

// ─── Token transfer helpers (native-aware) ────────────────────────────────────

// pullToken moves tokens from caller into the contract.
// For native LQD it verifies msg.value; for LQD20 it calls TransferFrom.
func (f *Factory) pullToken(ctx *bc.Context, token, from string, amt *big.Int) {
	if isNative(token) {
		ctx.ReceiveNative(amt)
		return
	}
	if _, err := ctx.Call(token, "TransferFrom", []string{from, ctx.ContractAddr, amt.String()}); err != nil {
		ctx.Revert("TransferFrom failed: " + err.Error())
	}
}

// pushToken moves tokens from the contract to a recipient.
// For native LQD it uses ctx.SendNative; for LQD20 it calls Transfer.
func (f *Factory) pushToken(ctx *bc.Context, token, to string, amt *big.Int) {
	if isNative(token) {
		ctx.SendNative(to, amt)
		return
	}
	if _, err := ctx.Call(token, "Transfer", []string{to, amt.String()}); err != nil {
		ctx.Revert("Transfer failed: " + err.Error())
	}
}

// ─── FACTORY ─────────────────────────────────────────────────────────────────

// deterministicPairAddr generates a unique contract address for a pair.
// Uses first 20 bytes of SHA256(factory:token0:token1).
func deterministicPairAddr(factory, t0, t1 string) string {
	h := sha256.Sum256([]byte(strings.ToLower(factory) + ":" + t0 + ":" + t1))
	return "0x" + hex.EncodeToString(h[:20])
}

// Init stores the pair plugin path so CreatePair can deploy pair contracts.
// pairPluginPath is the compiled dex_pair.so path, set by the server at deploy time.
func (f *Factory) Init(ctx *bc.Context, pairPluginPath string) {
	if ctx.Get("__pairPlugin") != "" {
		ctx.Revert("already initialized")
	}
	ctx.Set("__pairPlugin", pairPluginPath)
	ctx.Commit()
	ctx.Emit("FactoryInitialized", map[string]interface{}{"pairPlugin": pairPluginPath})
}

// CreatePair deploys a new AMM pair contract and registers it.
// Use "lqd" as tokenA or tokenB for a native-LQD pair.
func (f *Factory) CreatePair(ctx *bc.Context, tokenA string, tokenB string) {
	tokenA, tokenB = normAddr(tokenA), normAddr(tokenB)
	if tokenA == "" || tokenB == "" || tokenA == tokenB {
		ctx.Revert("invalid token addresses")
	}
	pk, t0, t1 := pairKey(tokenA, tokenB)
	if ctx.Get("pairExists:"+pk) == "1" {
		ctx.Revert("pair already exists")
	}

	pairPluginPath := ctx.Get("__pairPlugin")
	if pairPluginPath == "" {
		ctx.Revert("factory not initialized — call Init(pairPluginPath) first")
	}

	// Generate deterministic pair address
	pairAddr := deterministicPairAddr(ctx.ContractAddr, t0, t1)

	// Deploy the pair contract (registers metadata + loads plugin immediately)
	ctx.DeployContract(pairAddr, pairPluginPath)

	// Initialize the pair via cross-contract call (in same atomic TX)
	if _, err := ctx.Call(pairAddr, "Init", []string{ctx.ContractAddr, t0, t1}); err != nil {
		ctx.Revert("pair Init failed: " + err.Error())
	}

	// Register in factory storage
	ctx.Set("pairExists:"+pk, "1")
	ctx.Set("pairAddr:"+pk, pairAddr)
	f.pset(ctx, pk, "t0", t0)
	f.pset(ctx, pk, "t1", t1)

	n := parseBig(ctx.Get("pairCount"))
	ctx.Set("pairAt:"+n.String(), pk)
	ctx.Set("pairAddr:"+n.String(), pairAddr)
	ctx.Set("pairCount", new(big.Int).Add(n, big.NewInt(1)).String())

	ctx.Set("output", pairAddr)
	ctx.Commit()
	ctx.Emit("PairCreated", map[string]interface{}{
		"token0":   t0,
		"token1":   t1,
		"pair":     pk,
		"pairAddr": pairAddr,
		"index":    n.String(),
	})
}

// GetPair returns the pair contract address for two tokens.
func (f *Factory) GetPair(ctx *bc.Context, tokenA string, tokenB string) {
	pk, t0, t1 := pairKey(tokenA, tokenB)
	pairAddr := ctx.Get("pairAddr:" + pk)
	exists := pairAddr != ""
	ctx.Set("output", pairAddr)
	ctx.Emit("PairInfo", map[string]interface{}{
		"pairAddr": pairAddr,
		"pair":     pk,
		"token0":   t0,
		"token1":   t1,
		"exists":   exists,
	})
}

// AllPairsLength returns the total number of registered pairs.
func (f *Factory) AllPairsLength(ctx *bc.Context) {
	n := ctx.Get("pairCount")
	if n == "" {
		n = "0"
	}
	ctx.Set("output", n)
	ctx.Emit("AllPairsLength", map[string]interface{}{"length": n})
}

// AllPairs returns the pair contract address at a given index.
func (f *Factory) AllPairs(ctx *bc.Context, index string) {
	idx := strings.TrimSpace(index)
	pk := ctx.Get("pairAt:" + idx)
	pairAddr := ctx.Get("pairAddr:" + idx)
	ctx.Set("output", pairAddr)
	t0, t1 := "", ""
	if pk != "" {
		parts := strings.SplitN(pk, ":", 2)
		if len(parts) == 2 {
			t0, t1 = parts[0], parts[1]
		}
	}
	ctx.Emit("PairAt", map[string]interface{}{
		"index": index, "pair": pk, "pairAddr": pairAddr, "token0": t0, "token1": t1,
	})
}

// ─── LIQUIDITY ────────────────────────────────────────────────────────────────

// ─── Router helpers — delegates to the pair contract ─────────────────────────

func (f *Factory) requirePair(ctx *bc.Context, tokenA, tokenB string) (pairAddr, pk string) {
	tokenA, tokenB = normAddr(tokenA), normAddr(tokenB)
	pk, _, _ = pairKey(tokenA, tokenB)
	pairAddr = ctx.Get("pairAddr:" + pk)
	if pairAddr == "" {
		ctx.Revert("pair does not exist — call CreatePair first")
	}
	return
}

type swapRoute struct {
	kind       string
	directPair string
	hop1Addr   string
	hop2Addr   string
	midToken   string
	weight     int64
	depth      *big.Int
	hops       int
}

type pairQuote struct {
	amountOut      *big.Int
	priceImpactBps *big.Int
	reserveIn      *big.Int
	reserveOut     *big.Int
	fee            *big.Int
	raw            string
}

func requireDeadline(ctx *bc.Context, deadline string) {
	dl := parseBig(deadline)
	if dl.Sign() <= 0 {
		ctx.Revert("deadline must be > 0")
	}
	if ctx.BlockTime > dl.Int64() {
		ctx.Revert("transaction expired")
	}
}

func (r swapRoute) valid() bool {
	return r.kind != "" && r.hops > 0
}

func (r swapRoute) betterThan(other swapRoute) bool {
	if !r.valid() {
		return false
	}
	if !other.valid() {
		return true
	}
	if r.weight != other.weight {
		return r.weight > other.weight
	}
	rd := r.depth
	od := other.depth
	if rd == nil {
		rd = big.NewInt(0)
	}
	if od == nil {
		od = big.NewInt(0)
	}
	if cmp := rd.Cmp(od); cmp != 0 {
		return cmp > 0
	}
	return r.hops < other.hops
}

func (f *Factory) pairWeight(ctx *bc.Context, pairAddr string) int64 {
	if pairAddr == "" {
		return 0
	}
	res, err := ctx.Call(pairAddr, "GetRoutingWeight", []string{})
	if err != nil || res == nil || res.Output == "" {
		return 50
	}
	if v := parseBig(res.Output); v.IsInt64() {
		return v.Int64()
	}
	return 50
}

func (f *Factory) pairDepth(ctx *bc.Context, pairAddr string) *big.Int {
	if pairAddr == "" {
		return big.NewInt(0)
	}
	res, err := ctx.Call(pairAddr, "GetReserves", []string{})
	if err != nil || res == nil || res.Output == "" {
		return big.NewInt(0)
	}
	parts := strings.SplitN(res.Output, ",", 3)
	if len(parts) < 2 {
		return big.NewInt(0)
	}
	r0 := parseBig(parts[0])
	r1 := parseBig(parts[1])
	if r0.Cmp(r1) < 0 {
		return r0
	}
	return r1
}

func (f *Factory) quotePair(ctx *bc.Context, pairAddr, amountIn, tokenIn string) pairQuote {
	zero := pairQuote{
		amountOut:      big.NewInt(0),
		priceImpactBps: big.NewInt(0),
		reserveIn:      big.NewInt(0),
		reserveOut:     big.NewInt(0),
		fee:            big.NewInt(0),
	}
	if pairAddr == "" {
		return zero
	}
	res, err := ctx.Call(pairAddr, "GetQuote", []string{amountIn, tokenIn})
	if err != nil || res == nil || res.Output == "" {
		legacy, legacyErr := ctx.Call(pairAddr, "GetAmountOut", []string{amountIn, tokenIn})
		if legacyErr != nil || legacy == nil || legacy.Output == "" {
			return zero
		}
		zero.amountOut = parseBig(legacy.Output)
		zero.raw = legacy.Output
		return zero
	}
	parts := strings.Split(res.Output, ",")
	q := zero
	q.raw = res.Output
	if len(parts) > 0 {
		q.amountOut = parseBig(parts[0])
	}
	if len(parts) > 1 {
		q.priceImpactBps = parseBig(parts[1])
	}
	if len(parts) > 2 {
		q.reserveIn = parseBig(parts[2])
	}
	if len(parts) > 3 {
		q.reserveOut = parseBig(parts[3])
	}
	if len(parts) > 8 {
		q.fee = parseBig(parts[8])
	}
	return q
}

func (f *Factory) selectBestSwapRoute(ctx *bc.Context, tokenIn, tokenOut string) swapRoute {
	tokenIn, tokenOut = normAddr(tokenIn), normAddr(tokenOut)

	best := swapRoute{}

	// Direct route first, but only if it truly scores best.
	pk, _, _ := pairKey(tokenIn, tokenOut)
	if direct := ctx.Get("pairAddr:" + pk); direct != "" {
		best = swapRoute{
			kind:       "direct",
			directPair: direct,
			weight:     f.pairWeight(ctx, direct),
			depth:      f.pairDepth(ctx, direct),
			hops:       1,
		}
	}

	// Compare against the best 2-hop route.
	mid, hop1, hop2 := f.findBestRoute(ctx, tokenIn, tokenOut)
	if mid != "" {
		candidate := swapRoute{
			kind:     "2hop",
			hop1Addr: hop1,
			hop2Addr: hop2,
			midToken: mid,
			weight:   50,
			depth:    big.NewInt(0),
			hops:     2,
		}
		w1 := f.pairWeight(ctx, hop1)
		w2 := f.pairWeight(ctx, hop2)
		candidate.weight = w1
		if w2 < candidate.weight {
			candidate.weight = w2
		}
		d1 := f.pairDepth(ctx, hop1)
		d2 := f.pairDepth(ctx, hop2)
		candidate.depth = d1
		if d2.Cmp(candidate.depth) < 0 {
			candidate.depth = d2
		}

		if candidate.betterThan(best) {
			best = candidate
		}
	}

	return best
}

// ─── LIQUIDITY ROUTER ─────────────────────────────────────────────────────────

// AddLiquidity deposits tokenA + tokenB and mints LP tokens via the pair contract.
// For native LQD pairs: set tx.value = amount of the "lqd" token.
func (f *Factory) AddLiquidity(ctx *bc.Context, tokenA string, tokenB string, amountA string, amountB string) {
	pairAddr, _ := f.requirePair(ctx, tokenA, tokenB)
	amt0, amt1 := sortPairAmounts(tokenA, tokenB, amountA, amountB)
	_, err := ctx.Call(pairAddr, "AddLiquidity", []string{amt0, amt1})
	if err != nil {
		ctx.Revert("AddLiquidity failed: " + err.Error())
	}
}

// RemoveLiquidity burns LP tokens and returns proportional token amounts.
func (f *Factory) RemoveLiquidity(ctx *bc.Context, tokenA string, tokenB string, lpAmount string) {
	pairAddr, _ := f.requirePair(ctx, tokenA, tokenB)
	_, err := ctx.Call(pairAddr, "RemoveLiquidity", []string{lpAmount})
	if err != nil {
		ctx.Revert("RemoveLiquidity failed: " + err.Error())
	}
}

// ─── SWAP ROUTER ─────────────────────────────────────────────────────────────

// SwapExactTokensForTokens swaps an exact input for a minimum output.
//
// Routing logic (Virtual Routing — PosDL innovation):
//  1. Try the direct pair first.
//  2. If no direct pair exists, find the best 2-hop route guided by
//     routing_weight set by the Dynamic Liquidity Engine.
//     The intermediate token with the highest combined weight wins.
//
// For native LQD input:  set tx.value = amountIn, tokenIn = "lqd"
// For native LQD output: tokenOut = "lqd", native LQD sent back automatically
func (f *Factory) SwapExactTokensForTokens(ctx *bc.Context, amountIn string, minAmountOut string, tokenIn string, tokenOut string) {
	tokenIn, tokenOut = normAddr(tokenIn), normAddr(tokenOut)
	route := f.selectBestSwapRoute(ctx, tokenIn, tokenOut)
	if !route.valid() {
		ctx.Revert("no route found for " + tokenIn + " → " + tokenOut)
	}

	if route.kind == "direct" {
		quote := f.quotePair(ctx, route.directPair, amountIn, tokenIn)
		if quote.amountOut.Cmp(parseBig(minAmountOut)) < 0 {
			ctx.Revert("slippage: insufficient output amount")
		}
		_, err := ctx.Call(route.directPair, "Swap", []string{amountIn, minAmountOut, tokenIn})
		if err != nil {
			ctx.Revert("Swap failed: " + err.Error())
		}
		return
	}

	// Hop 1: tokenIn → midToken  (get intermediate amount out)
	hop1Quote := f.quotePair(ctx, route.hop1Addr, amountIn, tokenIn)
	if hop1Quote.amountOut.Sign() <= 0 {
		ctx.Revert("route hop1 gives zero output")
	}
	hop2Quote := f.quotePair(ctx, route.hop2Addr, hop1Quote.amountOut.String(), route.midToken)
	if hop2Quote.amountOut.Cmp(parseBig(minAmountOut)) < 0 {
		ctx.Revert("slippage: routed output below minimum")
	}

	// Execute hop 1: tokenIn → midToken into this factory/router contract.
	_, err := ctx.Call(route.hop1Addr, "SwapTo", []string{ctx.ContractAddr, amountIn, "0", tokenIn})
	if err != nil {
		ctx.Revert("route hop1 swap failed: " + err.Error())
	}

	if route.midToken != NATIVE {
		if _, err := ctx.Call(route.midToken, "Approve", []string{route.hop2Addr, hop1Quote.amountOut.String()}); err != nil {
			ctx.Revert("route hop2 approval failed: " + err.Error())
		}
	}

	// Execute hop 2 from the factory/router balance and send final output to user.
	_, err = ctx.Call(route.hop2Addr, "SwapFromContract", []string{swapReceiver(ctx), hop1Quote.amountOut.String(), minAmountOut, route.midToken})
	if err != nil {
		ctx.Revert("route hop2 swap failed: " + err.Error())
	}
	ctx.Set("output", hop2Quote.amountOut.String())

	ctx.Emit("MultiHopSwap", map[string]interface{}{
		"tokenIn":   tokenIn,
		"midToken":  route.midToken,
		"tokenOut":  tokenOut,
		"hop1":      route.hop1Addr,
		"hop2":      route.hop2Addr,
		"amountOut": hop2Quote.amountOut.String(),
	})
}

func (f *Factory) SwapExactTokensForTokensWithDeadline(ctx *bc.Context, amountIn string, minAmountOut string, tokenIn string, tokenOut string, deadline string) {
	requireDeadline(ctx, deadline)
	f.SwapExactTokensForTokens(ctx, amountIn, minAmountOut, tokenIn, tokenOut)
}

// findBestRoute finds the optimal 2-hop path from tokenIn → X → tokenOut.
// It iterates all registered pairs to find intermediate tokens, then scores
// each candidate route by the minimum routing_weight of its two hops.
// Higher routing_weight = preferred by the Dynamic Liquidity Engine.
func (f *Factory) findBestRoute(ctx *bc.Context, tokenIn, tokenOut string) (midToken, hop1Addr, hop2Addr string) {
	countStr := ctx.Get("pairCount")
	if countStr == "" {
		return
	}
	count := parseBig(countStr).Int64()

	bestScore := int64(-1)

	for i := int64(0); i < count; i++ {
		pk := ctx.Get("pairAt:" + strconv.FormatInt(i, 10))
		if pk == "" {
			continue
		}
		parts := strings.SplitN(pk, ":", 2)
		if len(parts) != 2 {
			continue
		}
		pt0, pt1 := parts[0], parts[1]

		// Identify if this pair involves tokenIn → find the mid token
		var mid string
		switch {
		case strings.EqualFold(pt0, tokenIn):
			mid = pt1
		case strings.EqualFold(pt1, tokenIn):
			mid = pt0
		default:
			continue // pair doesn't include tokenIn
		}

		// Check if there is a second pair: mid → tokenOut
		pk2, _, _ := pairKey(mid, tokenOut)
		hop2 := ctx.Get("pairAddr:" + pk2)
		if hop2 == "" {
			continue // no closing pair
		}

		hop1 := ctx.Get("pairAddr:" + pk)
		if hop1 == "" {
			continue
		}

		// Score = min(weight1, weight2) — bottleneck metric
		// Read routing_weight from each pair contract via cross-contract call
		w1 := int64(50)
		w2 := int64(50)
		if r1, err := ctx.Call(hop1, "GetRoutingWeight", []string{}); err == nil && r1 != nil && r1.Output != "" {
			if v := parseBig(r1.Output); v.IsInt64() {
				w1 = v.Int64()
			}
		}
		if r2, err := ctx.Call(hop2, "GetRoutingWeight", []string{}); err == nil && r2 != nil && r2.Output != "" {
			if v := parseBig(r2.Output); v.IsInt64() {
				w2 = v.Int64()
			}
		}
		score := w1
		if w2 < score {
			score = w2
		}

		if score > bestScore {
			bestScore = score
			midToken = mid
			hop1Addr = hop1
			hop2Addr = hop2
		}
	}
	return
}

// GetBestRoute returns the best routing path for a swap (view function).
// Returns the intermediate token address and both hop pair addresses.
func (f *Factory) GetBestRoute(ctx *bc.Context, tokenIn string, tokenOut string) {
	tokenIn, tokenOut = normAddr(tokenIn), normAddr(tokenOut)
	route := f.selectBestSwapRoute(ctx, tokenIn, tokenOut)
	if !route.valid() {
		ctx.Set("output", "")
		ctx.Emit("BestRoute", map[string]interface{}{"type": "none"})
		return
	}

	if route.kind == "direct" {
		ctx.Set("output", route.directPair)
		ctx.Emit("BestRoute", map[string]interface{}{
			"type":     "direct",
			"pairAddr": route.directPair,
			"hops":     "1",
		})
		return
	}

	ctx.Set("output", route.hop1Addr+","+route.hop2Addr)
	ctx.Emit("BestRoute", map[string]interface{}{
		"type":     "2hop",
		"midToken": route.midToken,
		"hop1":     route.hop1Addr,
		"hop2":     route.hop2Addr,
		"hops":     "2",
	})
}

// GetPairWeight returns the current routing weight for a pair (set by DLEngine).
func (f *Factory) GetPairWeight(ctx *bc.Context, tokenA string, tokenB string) {
	pairAddr, _ := f.requirePair(ctx, tokenA, tokenB)
	res, err := ctx.Call(pairAddr, "GetRoutingWeight", []string{})
	if err != nil || res == nil {
		ctx.Set("output", "50") // default
		return
	}
	ctx.Set("output", res.Output)
}

// ─── VIEW HELPERS ─────────────────────────────────────────────────────────────

// GetAmountOut returns expected output for a given input (read-only, via pair contract).
func (f *Factory) GetAmountOut(ctx *bc.Context, amountIn string, tokenIn string, tokenOut string) {
	tokenIn, tokenOut = normAddr(tokenIn), normAddr(tokenOut)
	route := f.selectBestSwapRoute(ctx, tokenIn, tokenOut)
	if !route.valid() {
		ctx.Set("output", "0")
		return
	}
	if route.kind == "direct" {
		ctx.Set("output", f.quotePair(ctx, route.directPair, amountIn, tokenIn).amountOut.String())
		return
	}
	hop1 := f.quotePair(ctx, route.hop1Addr, amountIn, tokenIn)
	if hop1.amountOut.Sign() <= 0 {
		ctx.Set("output", "0")
		return
	}
	hop2 := f.quotePair(ctx, route.hop2Addr, hop1.amountOut.String(), route.midToken)
	ctx.Set("output", hop2.amountOut.String())
}

// GetAmountIn returns required input to receive an exact output (read-only).
func (f *Factory) GetAmountIn(ctx *bc.Context, amountOut string, tokenIn string, tokenOut string) {
	tokenIn, tokenOut = normAddr(tokenIn), normAddr(tokenOut)
	pk, _, _ := pairKey(tokenIn, tokenOut)
	pairAddr := ctx.Get("pairAddr:" + pk)
	if pairAddr == "" {
		ctx.Set("output", "0")
		return
	}
	res, err := ctx.Call(pairAddr, "GetAmountIn", []string{amountOut, tokenOut})
	if err != nil {
		ctx.Set("output", "0")
		return
	}
	ctx.Set("output", res.Output)
}

func (f *Factory) GetSwapQuote(ctx *bc.Context, amountIn string, tokenIn string, tokenOut string) {
	tokenIn, tokenOut = normAddr(tokenIn), normAddr(tokenOut)
	route := f.selectBestSwapRoute(ctx, tokenIn, tokenOut)
	if !route.valid() {
		ctx.Set("output", "none|0|0")
		ctx.Emit("SwapQuote", map[string]interface{}{"type": "none", "amountOut": "0"})
		return
	}
	if route.kind == "direct" {
		q := f.quotePair(ctx, route.directPair, amountIn, tokenIn)
		ctx.Set("output", strings.Join([]string{"direct", q.amountOut.String(), q.priceImpactBps.String(), route.directPair, q.reserveIn.String(), q.reserveOut.String(), q.fee.String()}, "|"))
		ctx.Emit("SwapQuote", map[string]interface{}{
			"type": "direct", "amountOut": q.amountOut.String(), "priceImpactBps": q.priceImpactBps.String(), "pair": route.directPair,
		})
		return
	}
	hop1 := f.quotePair(ctx, route.hop1Addr, amountIn, tokenIn)
	hop2 := f.quotePair(ctx, route.hop2Addr, hop1.amountOut.String(), route.midToken)
	impact := big.NewInt(0)
	if hop1.reserveIn.Sign() > 0 && hop1.reserveOut.Sign() > 0 && hop2.reserveIn.Sign() > 0 && hop2.reserveOut.Sign() > 0 {
		spotMid := new(big.Int).Div(new(big.Int).Mul(parseBig(amountIn), hop1.reserveOut), hop1.reserveIn)
		spotFinal := new(big.Int).Div(new(big.Int).Mul(spotMid, hop2.reserveOut), hop2.reserveIn)
		if spotFinal.Sign() > 0 && spotFinal.Cmp(hop2.amountOut) > 0 {
			impact.Div(new(big.Int).Mul(new(big.Int).Sub(spotFinal, hop2.amountOut), big.NewInt(10000)), spotFinal)
		}
	}
	ctx.Set("output", strings.Join([]string{"2hop", hop2.amountOut.String(), impact.String(), route.hop1Addr + "," + route.hop2Addr, route.midToken, hop1.amountOut.String(), hop1.priceImpactBps.String(), hop2.priceImpactBps.String()}, "|"))
	ctx.Emit("SwapQuote", map[string]interface{}{
		"type": "2hop", "amountOut": hop2.amountOut.String(), "priceImpactBps": impact.String(), "midToken": route.midToken,
		"hop1": route.hop1Addr, "hop2": route.hop2Addr, "hop1Out": hop1.amountOut.String(),
	})
}

// GetPoolInfo returns full state for a pair (delegates to pair contract).
func (f *Factory) GetPoolInfo(ctx *bc.Context, tokenA string, tokenB string) {
	pairAddr, pk := f.requirePair(ctx, tokenA, tokenB)
	_ = pk
	res, err := ctx.Call(pairAddr, "GetInfo", []string{})
	if err != nil {
		ctx.Revert("GetPoolInfo failed: " + err.Error())
	}
	if res != nil {
		ctx.Set("output", res.Output)
	}
}

// GetLPBalance returns LP balance for an address (delegates to pair contract).
func (f *Factory) GetLPBalance(ctx *bc.Context, tokenA string, tokenB string, addr string) {
	pairAddr, _ := f.requirePair(ctx, tokenA, tokenB)
	res, err := ctx.Call(pairAddr, "BalanceOf", []string{addr})
	if err != nil {
		ctx.Set("output", "0")
		return
	}
	ctx.Set("output", res.Output)
}

// GetLPValue returns the pool-backing value of a given LP amount.
func (f *Factory) GetLPValue(ctx *bc.Context, tokenA string, tokenB string, lpAmount string) {
	pairAddr, _ := f.requirePair(ctx, tokenA, tokenB)
	res, err := ctx.Call(pairAddr, "GetReserves", []string{})
	if err != nil {
		return
	}
	_ = res
	ctx.Emit("LPValue", map[string]interface{}{
		"pairAddr": pairAddr, "lpAmount": lpAmount,
	})
}

// ─── PROOF OF DYNAMIC LIQUIDITY ───────────────────────────────────────────────

// LockLPForValidation locks LP tokens for a pair for validator consensus power.
func (f *Factory) LockLPForValidation(ctx *bc.Context, tokenA string, tokenB string, lpAmount string, lockSeconds string) {
	pairAddr, _ := f.requirePair(ctx, tokenA, tokenB)
	_, err := ctx.Call(pairAddr, "LockLPForValidation", []string{lpAmount, lockSeconds})
	if err != nil {
		ctx.Revert("LockLPForValidation failed: " + err.Error())
	}
}

// UnlockValidatorLP releases locked LP after the lock period expires.
func (f *Factory) UnlockValidatorLP(ctx *bc.Context, tokenA string, tokenB string) {
	pairAddr, _ := f.requirePair(ctx, tokenA, tokenB)
	_, err := ctx.Call(pairAddr, "UnlockValidatorLP", []string{})
	if err != nil {
		ctx.Revert("UnlockValidatorLP failed: " + err.Error())
	}
}

// GetValidatorLP returns locked LP info for a validator.
func (f *Factory) GetValidatorLP(ctx *bc.Context, tokenA string, tokenB string, validatorAddr string) {
	pairAddr, _ := f.requirePair(ctx, tokenA, tokenB)
	res, err := ctx.Call(pairAddr, "GetValidatorLP", []string{validatorAddr})
	if err != nil {
		ctx.Revert("GetValidatorLP failed: " + err.Error())
	}
	if res != nil {
		ctx.Set("output", res.Output)
	}
}

// REQUIRED EXPORT
var Contract = &Factory{}
