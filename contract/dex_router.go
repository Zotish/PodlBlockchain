//go:build ignore
// +build ignore

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"sort"
	"strconv"
	"strings"

	bc "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/BlockchainComponent"
)

// DEX Router
//
// The factory owns pair creation and pair registry. The router owns user-facing
// swap, add/remove liquidity, and LP lock calls by delegating to the selected
// pair contract. Use "lqd" as the native LQD token sentinel.

const NATIVE = "lqd"

type Router struct{}

type swapRoute struct {
	kind         string
	directPair   string
	hop1Addr     string
	hop2Addr     string
	hop3Addr     string
	midToken     string
	midToken2    string
	weight       int64
	depth        *big.Int
	amountOut    *big.Int
	netAmountOut *big.Int
	hops         int
	pairs        []string
	pathTokens   []string
}

type pairQuote struct {
	amountOut      *big.Int
	priceImpactBps *big.Int
	reserveIn      *big.Int
	reserveOut     *big.Int
	fee            *big.Int
	raw            string
}

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

func pairKey(a, b string) (pk, t0, t1 string) {
	a, b = normAddr(a), normAddr(b)
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

func requireDeadline(ctx *bc.Context, deadline string) {
	dl := parseBig(deadline)
	if dl.Sign() <= 0 {
		ctx.Revert("deadline must be > 0")
	}
	if ctx.BlockTime > dl.Int64() {
		ctx.Revert("transaction expired")
	}
}

func quoteOutputBps(amountIn, amountOut, reserveIn, reserveOut *big.Int) *big.Int {
	if amountIn.Sign() <= 0 || amountOut.Sign() <= 0 || reserveIn.Sign() <= 0 || reserveOut.Sign() <= 0 {
		return big.NewInt(0)
	}
	spotOut := new(big.Int).Div(new(big.Int).Mul(amountIn, reserveOut), reserveIn)
	if spotOut.Sign() <= 0 || spotOut.Cmp(amountOut) <= 0 {
		return big.NewInt(0)
	}
	return new(big.Int).Div(new(big.Int).Mul(new(big.Int).Sub(spotOut, amountOut), big.NewInt(10000)), spotOut)
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
	rScore, oScore := r.netAmountOut, other.netAmountOut
	if rScore == nil {
		rScore = r.amountOut
	}
	if oScore == nil {
		oScore = other.amountOut
	}
	if rScore != nil && oScore != nil {
		if cmp := rScore.Cmp(oScore); cmp != 0 {
			return cmp > 0
		}
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

func (r *Router) quoteRoute(ctx *bc.Context, route swapRoute, amountIn, tokenIn string) *big.Int {
	if !route.valid() {
		return big.NewInt(0)
	}
	if len(route.pairs) > 0 && len(route.pathTokens) == len(route.pairs)+1 {
		amount := parseBig(amountIn)
		for i, pair := range route.pairs {
			amount = r.quotePair(ctx, pair, amount.String(), route.pathTokens[i]).amountOut
			if amount.Sign() <= 0 {
				return big.NewInt(0)
			}
		}
		return amount
	}
	if route.kind == "direct" {
		return r.quotePair(ctx, route.directPair, amountIn, tokenIn).amountOut
	}
	q1 := r.quotePair(ctx, route.hop1Addr, amountIn, tokenIn)
	if q1.amountOut.Sign() <= 0 {
		return big.NewInt(0)
	}
	q2 := r.quotePair(ctx, route.hop2Addr, q1.amountOut.String(), route.midToken)
	if route.kind == "2hop" || q2.amountOut.Sign() <= 0 {
		return q2.amountOut
	}
	return r.quotePair(ctx, route.hop3Addr, q2.amountOut.String(), route.midToken2).amountOut
}

// Init stores the canonical factory address this router should use.
func (r *Router) Init(ctx *bc.Context, factory string) {
	factory = normAddr(factory)
	if factory == "" || factory == NATIVE {
		ctx.Revert("invalid factory address")
	}
	if ctx.Get("factory") != "" {
		ctx.Revert("already initialized")
	}
	ctx.Set("factory", factory)
	ctx.Commit()
	ctx.Emit("RouterInitialized", map[string]interface{}{"factory": factory})
}

func (r *Router) factory(ctx *bc.Context) string {
	factory := normAddr(ctx.Get("factory"))
	if factory == "" {
		ctx.Revert("router not initialized")
	}
	return factory
}

func (r *Router) pairFor(ctx *bc.Context, tokenA, tokenB string) string {
	pairAddr := r.optionalPairFor(ctx, tokenA, tokenB)
	if pairAddr == "" {
		ctx.Revert("pair does not exist — call CreatePair on factory first")
	}
	return pairAddr
}

func (r *Router) optionalPairFor(ctx *bc.Context, tokenA, tokenB string) string {
	res, err := ctx.Call(r.factory(ctx), "GetPair", []string{tokenA, tokenB})
	if err != nil || res == nil {
		return ""
	}
	return normAddr(res.Output)
}

func (r *Router) pairWeight(ctx *bc.Context, pairAddr string) int64 {
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

func (r *Router) pairDepth(ctx *bc.Context, pairAddr string) *big.Int {
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

func (r *Router) quotePair(ctx *bc.Context, pairAddr, amountIn, tokenIn string) pairQuote {
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

func (r *Router) pairToken(ctx *bc.Context, pairAddr, fn string) string {
	res, err := ctx.Call(pairAddr, fn, []string{})
	if err != nil || res == nil {
		return ""
	}
	return normAddr(res.Output)
}

func (r *Router) allPairsLength(ctx *bc.Context) int64 {
	res, err := ctx.Call(r.factory(ctx), "AllPairsLength", []string{})
	if err != nil || res == nil || res.Output == "" {
		return 0
	}
	if v := parseBig(res.Output); v.IsInt64() {
		return v.Int64()
	}
	return 0
}

func (r *Router) allPairAt(ctx *bc.Context, index int64) string {
	res, err := ctx.Call(r.factory(ctx), "AllPairs", []string{strconv.FormatInt(index, 10)})
	if err != nil || res == nil {
		return ""
	}
	return normAddr(res.Output)
}

func (r *Router) findBestRoute(ctx *bc.Context, tokenIn, tokenOut string) (midToken, hop1Addr, hop2Addr string) {
	tokenIn, tokenOut = normAddr(tokenIn), normAddr(tokenOut)
	bestScore := int64(-1)
	count := r.allPairsLength(ctx)

	for i := int64(0); i < count; i++ {
		hop1 := r.allPairAt(ctx, i)
		if hop1 == "" {
			continue
		}
		pt0 := r.pairToken(ctx, hop1, "Token0")
		pt1 := r.pairToken(ctx, hop1, "Token1")
		if pt0 == "" || pt1 == "" {
			continue
		}

		var mid string
		switch {
		case pt0 == tokenIn:
			mid = pt1
		case pt1 == tokenIn:
			mid = pt0
		default:
			continue
		}
		if mid == tokenOut {
			continue
		}

		hop2 := r.optionalPairFor(ctx, mid, tokenOut)
		if hop2 == "" || hop2 == hop1 {
			continue
		}

		w1 := r.pairWeight(ctx, hop1)
		w2 := r.pairWeight(ctx, hop2)
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

func (r *Router) selectBestSwapRoute(ctx *bc.Context, tokenIn, tokenOut string) swapRoute {
	return r.selectBestSwapRouteForAmount(ctx, "1", tokenIn, tokenOut)
}

func (r *Router) selectBestSwapRouteForAmount(ctx *bc.Context, amountIn, tokenIn, tokenOut string) swapRoute {
	tokenIn, tokenOut = normAddr(tokenIn), normAddr(tokenOut)
	if cached := r.loadRouteCache(ctx, amountIn, tokenIn, tokenOut); cached.valid() {
		return cached
	}
	best := swapRoute{}

	// Build a canonical undirected token graph from the factory registry, then
	// enumerate simple paths. The hop cap bounds gas while avoiding the former
	// hard-coded direct/2/3-hop limitation.
	type edge struct{ pair, to string }
	graph := map[string][]edge{}
	count := r.allPairsLength(ctx)
	for i := int64(0); i < count; i++ {
		pair := r.allPairAt(ctx, i)
		a, b := r.pairToken(ctx, pair, "Token0"), r.pairToken(ctx, pair, "Token1")
		if pair == "" || a == "" || b == "" || a == b {
			continue
		}
		graph[a] = append(graph[a], edge{pair: pair, to: b})
		graph[b] = append(graph[b], edge{pair: pair, to: a})
	}
	maxHops := int64(5)
	if configured := parseBig(ctx.Get("max_route_hops")); configured.IsInt64() && configured.Int64() >= 1 {
		maxHops = configured.Int64()
	}
	if maxHops > 8 {
		maxHops = 8
	}
	maxCandidates := int64(4096)
	if configured := parseBig(ctx.Get("max_route_candidates")); configured.IsInt64() && configured.Int64() >= 16 {
		maxCandidates = configured.Int64()
	}
	if maxCandidates > 100000 {
		maxCandidates = 100000
	}
	gasPenaltyPerExtraHopBPS := int64(3)
	if configured := parseBig(ctx.Get("route_gas_penalty_bps")); configured.IsInt64() && configured.Int64() >= 0 && configured.Int64() <= 1000 {
		gasPenaltyPerExtraHopBPS = configured.Int64()
	}
	for token := range graph {
		sort.SliceStable(graph[token], func(i, j int) bool {
			wi, wj := r.pairWeight(ctx, graph[token][i].pair), r.pairWeight(ctx, graph[token][j].pair)
			if wi != wj {
				return wi > wj
			}
			return graph[token][i].pair < graph[token][j].pair
		})
	}
	visited := map[string]bool{tokenIn: true}
	candidates := int64(0)
	var walk func(string, *big.Int, []string, []string, int64, *big.Int)
	walk = func(current string, amount *big.Int, pairs, tokens []string, minWeight int64, minDepth *big.Int) {
		if len(pairs) >= int(maxHops) || candidates >= maxCandidates {
			return
		}
		for _, next := range graph[current] {
			if candidates >= maxCandidates {
				return
			}
			candidates++
			if visited[next.to] {
				continue
			}
			out := r.quotePair(ctx, next.pair, amount.String(), current).amountOut
			if out.Sign() <= 0 {
				continue
			}
			weight, depth := r.pairWeight(ctx, next.pair), r.pairDepth(ctx, next.pair)
			if len(pairs) > 0 {
				if weight > minWeight {
					weight = minWeight
				}
				if depth.Cmp(minDepth) > 0 {
					depth = new(big.Int).Set(minDepth)
				}
			}
			pathPairs := append(append([]string{}, pairs...), next.pair)
			pathTokens := append(append([]string{}, tokens...), next.to)
			if next.to == tokenOut {
				kind := strconv.Itoa(len(pathPairs)) + "hop"
				if len(pathPairs) == 1 {
					kind = "direct"
				}
				penalty := gasPenaltyPerExtraHopBPS * int64(len(pathPairs)-1)
				if penalty > 9000 {
					penalty = 9000
				}
				netOut := new(big.Int).Div(new(big.Int).Mul(out, big.NewInt(10000-penalty)), big.NewInt(10000))
				candidate := swapRoute{kind: kind, hops: len(pathPairs), pairs: pathPairs, pathTokens: pathTokens, amountOut: out, netAmountOut: netOut, weight: weight, depth: depth}
				candidate.directPair = pathPairs[0]
				if len(pathPairs) > 1 {
					candidate.hop1Addr, candidate.hop2Addr, candidate.midToken = pathPairs[0], pathPairs[1], pathTokens[1]
				}
				if len(pathPairs) > 2 {
					candidate.hop3Addr, candidate.midToken2 = pathPairs[2], pathTokens[2]
				}
				if candidate.betterThan(best) {
					best = candidate
				}
				continue
			}
			visited[next.to] = true
			walk(next.to, out, pathPairs, pathTokens, weight, depth)
			delete(visited, next.to)
		}
	}
	walk(tokenIn, parseBig(amountIn), nil, []string{tokenIn}, 0, big.NewInt(0))

	return best
}

func routeCacheKey(amountIn, tokenIn, tokenOut string) string {
	material := strings.Join([]string{strings.TrimSpace(amountIn), normAddr(tokenIn), normAddr(tokenOut)}, "|")
	sum := sha256.Sum256([]byte(material))
	return "route_cache:" + hex.EncodeToString(sum[:12])
}

func (r *Router) loadRouteCache(ctx *bc.Context, amountIn, tokenIn, tokenOut string) swapRoute {
	raw := ctx.Get(routeCacheKey(amountIn, tokenIn, tokenOut))
	parts := strings.Split(raw, "|")
	if len(parts) != 3 || parseBig(parts[0]).Int64() < ctx.BlockTime {
		return swapRoute{}
	}
	pairs, tokens := strings.Split(parts[1], ","), strings.Split(parts[2], ",")
	if len(pairs) == 0 || len(tokens) != len(pairs)+1 || normAddr(tokens[0]) != normAddr(tokenIn) || normAddr(tokens[len(tokens)-1]) != normAddr(tokenOut) {
		return swapRoute{}
	}
	seen, amount, weight, depth := map[string]bool{}, parseBig(amountIn), int64(0), big.NewInt(0)
	for i, pair := range pairs {
		pair, tokens[i], tokens[i+1] = normAddr(pair), normAddr(tokens[i]), normAddr(tokens[i+1])
		if pair == "" || seen[pair] || r.optionalPairFor(ctx, tokens[i], tokens[i+1]) != pair {
			return swapRoute{}
		}
		seen[pair] = true
		amount = r.quotePair(ctx, pair, amount.String(), tokens[i]).amountOut
		if amount.Sign() <= 0 {
			return swapRoute{}
		}
		w, d := r.pairWeight(ctx, pair), r.pairDepth(ctx, pair)
		if i == 0 || w < weight {
			weight = w
		}
		if i == 0 || d.Cmp(depth) < 0 {
			depth = d
		}
	}
	kind := strconv.Itoa(len(pairs)) + "hop"
	if len(pairs) == 1 {
		kind = "direct"
	}
	return swapRoute{kind: kind, directPair: pairs[0], hops: len(pairs), pairs: pairs, pathTokens: tokens, amountOut: amount, netAmountOut: amount, weight: weight, depth: depth}
}

// WarmRouteCache stores only route topology. Every use revalidates pair
// registry entries and requotes current reserves, so stale output is never
// trusted. The short TTL bounds changes in weights and graph topology.
func (r *Router) WarmRouteCache(ctx *bc.Context, amountIn, tokenIn, tokenOut, ttlSeconds string) {
	key := routeCacheKey(amountIn, tokenIn, tokenOut)
	ctx.Set(key, "")
	route := r.selectBestSwapRouteForAmount(ctx, amountIn, tokenIn, tokenOut)
	ttl := parseBig(ttlSeconds).Int64()
	if !route.valid() || ttl < 1 || ttl > 300 {
		ctx.Revert("valid route and cache ttl 1..300 required")
	}
	ctx.Set(key, strings.Join([]string{strconv.FormatInt(ctx.BlockTime+ttl, 10), strings.Join(route.pairs, ","), strings.Join(route.pathTokens, ",")}, "|"))
	ctx.Emit("RouteCacheWarmed", map[string]interface{}{"key": key, "pairs": strings.Join(route.pairs, ","), "expiresAt": ctx.BlockTime + ttl})
}

func (r *Router) AddLiquidity(ctx *bc.Context, tokenA string, tokenB string, amountA string, amountB string) {
	pairAddr := r.pairFor(ctx, tokenA, tokenB)
	amt0, amt1 := sortPairAmounts(tokenA, tokenB, amountA, amountB)
	if _, err := ctx.Call(pairAddr, "AddLiquidity", []string{amt0, amt1}); err != nil {
		ctx.Revert("AddLiquidity failed: " + err.Error())
	}
}

func (r *Router) RemoveLiquidity(ctx *bc.Context, tokenA string, tokenB string, lpAmount string) {
	pairAddr := r.pairFor(ctx, tokenA, tokenB)
	if _, err := ctx.Call(pairAddr, "RemoveLiquidity", []string{lpAmount}); err != nil {
		ctx.Revert("RemoveLiquidity failed: " + err.Error())
	}
}

func (r *Router) SwapExactTokensForTokens(ctx *bc.Context, amountIn string, minAmountOut string, tokenIn string, tokenOut string) {
	tokenIn, tokenOut = normAddr(tokenIn), normAddr(tokenOut)
	route := r.selectBestSwapRouteForAmount(ctx, amountIn, tokenIn, tokenOut)
	if !route.valid() {
		ctx.Revert("no route found for " + tokenIn + " -> " + tokenOut)
	}

	if route.hops == 1 {
		quote := r.quotePair(ctx, route.directPair, amountIn, tokenIn)
		if quote.amountOut.Cmp(parseBig(minAmountOut)) < 0 {
			ctx.Revert("slippage: insufficient output amount")
		}
		if _, err := ctx.Call(route.directPair, "Swap", []string{amountIn, minAmountOut, tokenIn}); err != nil {
			ctx.Revert("Swap failed: " + err.Error())
		}
		return
	}
	if len(route.pairs) != route.hops || len(route.pathTokens) != route.hops+1 {
		ctx.Revert("malformed multi-hop route")
	}
	amounts := make([]*big.Int, route.hops+1)
	amounts[0] = parseBig(amountIn)
	for i, pair := range route.pairs {
		amounts[i+1] = r.quotePair(ctx, pair, amounts[i].String(), route.pathTokens[i]).amountOut
		if amounts[i+1].Sign() <= 0 {
			ctx.Revert("route hop gives zero output")
		}
	}
	finalQuote := amounts[len(amounts)-1]
	if finalQuote.Cmp(parseBig(minAmountOut)) < 0 {
		ctx.Revert("slippage: routed output below minimum")
	}
	if _, err := ctx.Call(route.pairs[0], "SwapTo", []string{ctx.ContractAddr, amountIn, "0", route.pathTokens[0]}); err != nil {
		ctx.Revert("route hop1 swap failed: " + err.Error())
	}
	for i := 1; i < len(route.pairs); i++ {
		inputToken := route.pathTokens[i]
		if inputToken != NATIVE {
			if _, err := ctx.Call(inputToken, "Approve", []string{route.pairs[i], amounts[i].String()}); err != nil {
				ctx.Revert("route approval failed: " + err.Error())
			}
		}
		receiver, hopMin := ctx.ContractAddr, "0"
		if i == len(route.pairs)-1 {
			receiver, hopMin = swapReceiver(ctx), minAmountOut
		}
		if _, err := ctx.Call(route.pairs[i], "SwapFromContract", []string{receiver, amounts[i].String(), hopMin, inputToken}); err != nil {
			ctx.Revert("routed swap failed: " + err.Error())
		}
	}
	ctx.Set("output", finalQuote.String())
	ctx.Emit("MultiHopSwap", map[string]interface{}{
		"type": route.kind, "tokenIn": tokenIn, "tokenOut": tokenOut, "pairs": strings.Join(route.pairs, ","),
		"path": strings.Join(route.pathTokens, ","), "hops": route.hops, "amountOut": finalQuote.String(),
	})
}

func (r *Router) SwapExactTokensForTokensWithDeadline(ctx *bc.Context, amountIn string, minAmountOut string, tokenIn string, tokenOut string, deadline string) {
	requireDeadline(ctx, deadline)
	r.SwapExactTokensForTokens(ctx, amountIn, minAmountOut, tokenIn, tokenOut)
}

func (r *Router) GetBestRoute(ctx *bc.Context, tokenIn string, tokenOut string) {
	route := r.selectBestSwapRoute(ctx, tokenIn, tokenOut)
	if !route.valid() {
		ctx.Set("output", "")
		ctx.Emit("BestRoute", map[string]interface{}{"type": "none"})
		return
	}
	if route.hops == 1 {
		ctx.Set("output", route.directPair)
		ctx.Emit("BestRoute", map[string]interface{}{"type": "direct", "pairAddr": route.directPair, "hops": "1"})
		return
	}
	ctx.Set("output", strings.Join(route.pairs, ","))
	ctx.Emit("BestRoute", map[string]interface{}{"type": route.kind, "pairs": strings.Join(route.pairs, ","), "path": strings.Join(route.pathTokens, ","), "hops": route.hops})
}

func (r *Router) GetBestRouteForAmount(ctx *bc.Context, amountIn string, tokenIn string, tokenOut string) {
	route := r.selectBestSwapRouteForAmount(ctx, amountIn, tokenIn, tokenOut)
	if !route.valid() {
		ctx.Set("output", "")
		return
	}
	if route.hops == 1 {
		ctx.Set("output", route.directPair)
		return
	}
	ctx.Set("output", strings.Join(route.pairs, ","))
}

// GetSplitQuote evaluates a 50/50 direct + disjoint two-hop split. The output
// includes the unsplit best quote so clients can execute a split only when it
// improves price after the extra-hop fees.
func (r *Router) GetSplitQuote(ctx *bc.Context, amountIn string, tokenIn string, tokenOut string) {
	amt := parseBig(amountIn)
	if amt.Cmp(big.NewInt(2)) < 0 {
		ctx.Set("output", "none|0")
		return
	}
	half := new(big.Int).Div(amt, big.NewInt(2))
	rest := new(big.Int).Sub(amt, half)
	direct := r.optionalPairFor(ctx, tokenIn, tokenOut)
	mid, h1, h2 := r.findBestRoute(ctx, tokenIn, tokenOut)
	if direct == "" || mid == "" || mid == NATIVE {
		ctx.Set("output", "none|0")
		return
	}
	qDirect := r.quotePair(ctx, direct, half.String(), tokenIn)
	q1 := r.quotePair(ctx, h1, rest.String(), tokenIn)
	q2 := r.quotePair(ctx, h2, q1.amountOut.String(), mid)
	total := new(big.Int).Add(qDirect.amountOut, q2.amountOut)
	best := r.selectBestSwapRouteForAmount(ctx, amountIn, tokenIn, tokenOut)
	bestOut := r.quoteRoute(ctx, best, amountIn, tokenIn)
	ctx.Set("output", strings.Join([]string{"split50", total.String(), bestOut.String(), direct, h1 + "," + h2, mid}, "|"))
	ctx.Emit("SplitQuote", map[string]interface{}{"amountOut": total.String(), "unsplitBestOut": bestOut.String(), "direct": direct, "hop1": h1, "hop2": h2, "midToken": mid})
}

func (r *Router) SwapSplitExactTokensForTokens(ctx *bc.Context, amountIn string, minAmountOut string, tokenIn string, tokenOut string) {
	amt := parseBig(amountIn)
	if amt.Cmp(big.NewInt(2)) < 0 {
		ctx.Revert("split amount too small")
	}
	half := new(big.Int).Div(amt, big.NewInt(2))
	rest := new(big.Int).Sub(amt, half)
	direct := r.optionalPairFor(ctx, tokenIn, tokenOut)
	mid, h1, h2 := r.findBestRoute(ctx, tokenIn, tokenOut)
	if direct == "" || mid == "" || mid == NATIVE {
		ctx.Revert("two disjoint split paths unavailable")
	}
	qDirect := r.quotePair(ctx, direct, half.String(), tokenIn)
	q1 := r.quotePair(ctx, h1, rest.String(), tokenIn)
	q2 := r.quotePair(ctx, h2, q1.amountOut.String(), mid)
	total := new(big.Int).Add(qDirect.amountOut, q2.amountOut)
	if total.Cmp(parseBig(minAmountOut)) < 0 {
		ctx.Revert("split output below minimum")
	}
	if _, err := ctx.Call(direct, "Swap", []string{half.String(), "0", tokenIn}); err != nil {
		ctx.Revert("split direct path failed: " + err.Error())
	}
	if _, err := ctx.Call(h1, "SwapTo", []string{ctx.ContractAddr, rest.String(), "0", tokenIn}); err != nil {
		ctx.Revert("split hop1 failed: " + err.Error())
	}
	if _, err := ctx.Call(mid, "Approve", []string{h2, q1.amountOut.String()}); err != nil {
		ctx.Revert("split hop approval failed: " + err.Error())
	}
	if _, err := ctx.Call(h2, "SwapFromContract", []string{swapReceiver(ctx), q1.amountOut.String(), "0", mid}); err != nil {
		ctx.Revert("split hop2 failed: " + err.Error())
	}
	ctx.Set("output", total.String())
	ctx.Emit("SplitSwap", map[string]interface{}{"amountIn": amt.String(), "amountOut": total.String(), "direct": direct, "hop1": h1, "hop2": h2})
}

func swapCommitment(owner, amountIn, minOut, tokenIn, tokenOut, salt string) string {
	material := strings.Join([]string{normAddr(owner), strings.TrimSpace(amountIn), strings.TrimSpace(minOut), normAddr(tokenIn), normAddr(tokenOut), strings.TrimSpace(salt)}, "|")
	sum := sha256.Sum256([]byte(material))
	return "0x" + hex.EncodeToString(sum[:])
}

func (r *Router) CommitSwap(ctx *bc.Context, commitment string, validAfter string, expiresAt string) {
	owner := normAddr(ctx.OriginAddr)
	if owner == "" {
		owner = normAddr(ctx.CallerAddr)
	}
	valid, expires := parseBig(validAfter).Int64(), parseBig(expiresAt).Int64()
	if !strings.HasPrefix(commitment, "0x") || len(commitment) != 66 || valid <= ctx.BlockTime || expires <= valid || expires-valid > 3600 {
		ctx.Revert("invalid swap commitment window")
	}
	ctx.Set("swap_commit:"+owner, strings.ToLower(commitment))
	ctx.Set("swap_commit_valid:"+owner, big.NewInt(valid).String())
	ctx.Set("swap_commit_expiry:"+owner, big.NewInt(expires).String())
	ctx.Emit("SwapCommitted", map[string]interface{}{"owner": owner, "commitment": strings.ToLower(commitment), "validAfter": valid, "expiresAt": expires})
}

func (r *Router) RevealSwap(ctx *bc.Context, amountIn string, minAmountOut string, tokenIn string, tokenOut string, salt string) {
	owner := normAddr(ctx.OriginAddr)
	if owner == "" {
		owner = normAddr(ctx.CallerAddr)
	}
	expected := swapCommitment(owner, amountIn, minAmountOut, tokenIn, tokenOut, salt)
	if ctx.Get("swap_commit:"+owner) != expected || ctx.BlockTime < parseBig(ctx.Get("swap_commit_valid:"+owner)).Int64() || ctx.BlockTime > parseBig(ctx.Get("swap_commit_expiry:"+owner)).Int64() {
		ctx.Revert("invalid or premature swap reveal")
	}
	ctx.Set("swap_commit:"+owner, "")
	ctx.Set("swap_commit_valid:"+owner, "0")
	ctx.Set("swap_commit_expiry:"+owner, "0")
	r.SwapExactTokensForTokens(ctx, amountIn, minAmountOut, tokenIn, tokenOut)
}

func (r *Router) GetAmountOut(ctx *bc.Context, amountIn string, tokenIn string, tokenOut string) {
	route := r.selectBestSwapRouteForAmount(ctx, amountIn, tokenIn, tokenOut)
	if !route.valid() {
		ctx.Set("output", "0")
		return
	}
	if route.hops == 1 {
		ctx.Set("output", r.quotePair(ctx, route.directPair, amountIn, tokenIn).amountOut.String())
		return
	}
	ctx.Set("output", r.quoteRoute(ctx, route, amountIn, tokenIn).String())
}

func (r *Router) GetSwapQuote(ctx *bc.Context, amountIn string, tokenIn string, tokenOut string) {
	tokenIn, tokenOut = normAddr(tokenIn), normAddr(tokenOut)
	route := r.selectBestSwapRouteForAmount(ctx, amountIn, tokenIn, tokenOut)
	if !route.valid() {
		ctx.Set("output", "none|0|0")
		ctx.Emit("SwapQuote", map[string]interface{}{"type": "none", "amountOut": "0"})
		return
	}
	if route.hops == 1 {
		q := r.quotePair(ctx, route.directPair, amountIn, tokenIn)
		ctx.Set("output", strings.Join([]string{"direct", q.amountOut.String(), q.priceImpactBps.String(), route.directPair, q.reserveIn.String(), q.reserveOut.String(), q.fee.String()}, "|"))
		ctx.Emit("SwapQuote", map[string]interface{}{
			"type": "direct", "amountOut": q.amountOut.String(), "priceImpactBps": q.priceImpactBps.String(), "pair": route.directPair,
		})
		return
	}
	if route.hops > 2 {
		out := r.quoteRoute(ctx, route, amountIn, tokenIn)
		ctx.Set("output", strings.Join([]string{route.kind, out.String(), "0", strings.Join(route.pairs, ","), strings.Join(route.pathTokens[1:len(route.pathTokens)-1], ",")}, "|"))
		ctx.Emit("SwapQuote", map[string]interface{}{"type": route.kind, "amountOut": out.String(), "pairs": strings.Join(route.pairs, ","), "path": strings.Join(route.pathTokens, ",")})
		return
	}
	hop1 := r.quotePair(ctx, route.pairs[0], amountIn, route.pathTokens[0])
	hop2 := r.quotePair(ctx, route.pairs[1], hop1.amountOut.String(), route.pathTokens[1])
	impact := big.NewInt(0)
	if hop1.reserveIn.Sign() > 0 && hop1.reserveOut.Sign() > 0 && hop2.reserveIn.Sign() > 0 && hop2.reserveOut.Sign() > 0 {
		spotMid := new(big.Int).Div(new(big.Int).Mul(parseBig(amountIn), hop1.reserveOut), hop1.reserveIn)
		spotFinal := new(big.Int).Div(new(big.Int).Mul(spotMid, hop2.reserveOut), hop2.reserveIn)
		if spotFinal.Sign() > 0 && spotFinal.Cmp(hop2.amountOut) > 0 {
			impact.Div(new(big.Int).Mul(new(big.Int).Sub(spotFinal, hop2.amountOut), big.NewInt(10000)), spotFinal)
		}
	} else {
		impact = quoteOutputBps(parseBig(amountIn), hop2.amountOut, hop1.reserveIn, hop1.reserveOut)
	}
	ctx.Set("output", strings.Join([]string{"2hop", hop2.amountOut.String(), impact.String(), route.hop1Addr + "," + route.hop2Addr, route.midToken, hop1.amountOut.String(), hop1.priceImpactBps.String(), hop2.priceImpactBps.String()}, "|"))
	ctx.Emit("SwapQuote", map[string]interface{}{
		"type": "2hop", "amountOut": hop2.amountOut.String(), "priceImpactBps": impact.String(), "midToken": route.midToken,
		"hop1": route.hop1Addr, "hop2": route.hop2Addr, "hop1Out": hop1.amountOut.String(),
	})
}

func (r *Router) GetPair(ctx *bc.Context, tokenA string, tokenB string) {
	res, err := ctx.Call(r.factory(ctx), "GetPair", []string{tokenA, tokenB})
	if err != nil || res == nil {
		ctx.Set("output", "")
		return
	}
	ctx.Set("output", res.Output)
}

func (r *Router) GetPoolInfo(ctx *bc.Context, tokenA string, tokenB string) {
	pairAddr := r.pairFor(ctx, tokenA, tokenB)
	res, err := ctx.Call(pairAddr, "GetInfo", []string{})
	if err != nil || res == nil {
		ctx.Revert("GetPoolInfo failed")
	}
	ctx.Set("output", res.Output)
}

func (r *Router) GetLPBalance(ctx *bc.Context, tokenA string, tokenB string, addr string) {
	pairAddr := r.pairFor(ctx, tokenA, tokenB)
	res, err := ctx.Call(pairAddr, "BalanceOf", []string{addr})
	if err != nil || res == nil {
		ctx.Set("output", "0")
		return
	}
	ctx.Set("output", res.Output)
}

func (r *Router) LockLPForValidation(ctx *bc.Context, tokenA string, tokenB string, lpAmount string, lockSeconds string) {
	pairAddr := r.pairFor(ctx, tokenA, tokenB)
	if _, err := ctx.Call(pairAddr, "LockLPForValidation", []string{lpAmount, lockSeconds}); err != nil {
		ctx.Revert("LockLPForValidation failed: " + err.Error())
	}
}

func (r *Router) UnlockValidatorLP(ctx *bc.Context, tokenA string, tokenB string) {
	pairAddr := r.pairFor(ctx, tokenA, tokenB)
	if _, err := ctx.Call(pairAddr, "UnlockValidatorLP", []string{}); err != nil {
		ctx.Revert("UnlockValidatorLP failed: " + err.Error())
	}
}

func (r *Router) GetValidatorLP(ctx *bc.Context, tokenA string, tokenB string, validatorAddr string) {
	pairAddr := r.pairFor(ctx, tokenA, tokenB)
	res, err := ctx.Call(pairAddr, "GetValidatorLP", []string{validatorAddr})
	if err != nil || res == nil {
		ctx.Revert("GetValidatorLP failed")
	}
	ctx.Set("output", res.Output)
}

var Contract = &Router{}
