//go:build ignore
// +build ignore

package main

import (
	"math/big"
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
	tokenIn, tokenOut = normAddr(tokenIn), normAddr(tokenOut)
	best := swapRoute{}

	direct := r.optionalPairFor(ctx, tokenIn, tokenOut)
	if direct != "" {
		best = swapRoute{
			kind:       "direct",
			directPair: direct,
			weight:     r.pairWeight(ctx, direct),
			depth:      r.pairDepth(ctx, direct),
			hops:       1,
		}
	}

	mid, hop1, hop2 := r.findBestRoute(ctx, tokenIn, tokenOut)
	if mid != "" && mid != NATIVE {
		candidate := swapRoute{
			kind:     "2hop",
			hop1Addr: hop1,
			hop2Addr: hop2,
			midToken: mid,
			hops:     2,
			weight:   r.pairWeight(ctx, hop1),
			depth:    r.pairDepth(ctx, hop1),
		}
		w2 := r.pairWeight(ctx, hop2)
		if w2 < candidate.weight {
			candidate.weight = w2
		}
		d2 := r.pairDepth(ctx, hop2)
		if d2.Cmp(candidate.depth) < 0 {
			candidate.depth = d2
		}
		if candidate.betterThan(best) {
			best = candidate
		}
	}

	return best
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
	route := r.selectBestSwapRoute(ctx, tokenIn, tokenOut)
	if !route.valid() {
		ctx.Revert("no route found for " + tokenIn + " -> " + tokenOut)
	}

	if route.kind == "direct" {
		quote := r.quotePair(ctx, route.directPair, amountIn, tokenIn)
		if quote.amountOut.Cmp(parseBig(minAmountOut)) < 0 {
			ctx.Revert("slippage: insufficient output amount")
		}
		if _, err := ctx.Call(route.directPair, "Swap", []string{amountIn, minAmountOut, tokenIn}); err != nil {
			ctx.Revert("Swap failed: " + err.Error())
		}
		return
	}

	hop1Quote := r.quotePair(ctx, route.hop1Addr, amountIn, tokenIn)
	if hop1Quote.amountOut.Sign() <= 0 {
		ctx.Revert("route hop1 gives zero output")
	}
	hop2Quote := r.quotePair(ctx, route.hop2Addr, hop1Quote.amountOut.String(), route.midToken)
	if hop2Quote.amountOut.Cmp(parseBig(minAmountOut)) < 0 {
		ctx.Revert("slippage: routed output below minimum")
	}
	if _, err := ctx.Call(route.hop1Addr, "SwapTo", []string{ctx.ContractAddr, amountIn, "0", tokenIn}); err != nil {
		ctx.Revert("route hop1 swap failed: " + err.Error())
	}
	if route.midToken != NATIVE {
		if _, err := ctx.Call(route.midToken, "Approve", []string{route.hop2Addr, hop1Quote.amountOut.String()}); err != nil {
			ctx.Revert("route hop2 approval failed: " + err.Error())
		}
	}
	if _, err := ctx.Call(route.hop2Addr, "SwapFromContract", []string{swapReceiver(ctx), hop1Quote.amountOut.String(), minAmountOut, route.midToken}); err != nil {
		ctx.Revert("route hop2 swap failed: " + err.Error())
	}
	ctx.Set("output", hop2Quote.amountOut.String())
	ctx.Emit("MultiHopSwap", map[string]interface{}{
		"tokenIn": tokenIn, "midToken": route.midToken, "tokenOut": tokenOut,
		"hop1": route.hop1Addr, "hop2": route.hop2Addr, "amountOut": hop2Quote.amountOut.String(),
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
	if route.kind == "direct" {
		ctx.Set("output", route.directPair)
		ctx.Emit("BestRoute", map[string]interface{}{"type": "direct", "pairAddr": route.directPair, "hops": "1"})
		return
	}
	ctx.Set("output", route.hop1Addr+","+route.hop2Addr)
	ctx.Emit("BestRoute", map[string]interface{}{
		"type": "2hop", "midToken": route.midToken, "hop1": route.hop1Addr, "hop2": route.hop2Addr, "hops": "2",
	})
}

func (r *Router) GetAmountOut(ctx *bc.Context, amountIn string, tokenIn string, tokenOut string) {
	route := r.selectBestSwapRoute(ctx, tokenIn, tokenOut)
	if !route.valid() {
		ctx.Set("output", "0")
		return
	}
	if route.kind == "direct" {
		ctx.Set("output", r.quotePair(ctx, route.directPair, amountIn, tokenIn).amountOut.String())
		return
	}
	hop1 := r.quotePair(ctx, route.hop1Addr, amountIn, tokenIn)
	if hop1.amountOut.Sign() <= 0 {
		ctx.Set("output", "0")
		return
	}
	hop2 := r.quotePair(ctx, route.hop2Addr, hop1.amountOut.String(), route.midToken)
	ctx.Set("output", hop2.amountOut.String())
}

func (r *Router) GetSwapQuote(ctx *bc.Context, amountIn string, tokenIn string, tokenOut string) {
	tokenIn, tokenOut = normAddr(tokenIn), normAddr(tokenOut)
	route := r.selectBestSwapRoute(ctx, tokenIn, tokenOut)
	if !route.valid() {
		ctx.Set("output", "none|0|0")
		ctx.Emit("SwapQuote", map[string]interface{}{"type": "none", "amountOut": "0"})
		return
	}
	if route.kind == "direct" {
		q := r.quotePair(ctx, route.directPair, amountIn, tokenIn)
		ctx.Set("output", strings.Join([]string{"direct", q.amountOut.String(), q.priceImpactBps.String(), route.directPair, q.reserveIn.String(), q.reserveOut.String(), q.fee.String()}, "|"))
		ctx.Emit("SwapQuote", map[string]interface{}{
			"type": "direct", "amountOut": q.amountOut.String(), "priceImpactBps": q.priceImpactBps.String(), "pair": route.directPair,
		})
		return
	}
	hop1 := r.quotePair(ctx, route.hop1Addr, amountIn, tokenIn)
	hop2 := r.quotePair(ctx, route.hop2Addr, hop1.amountOut.String(), route.midToken)
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
