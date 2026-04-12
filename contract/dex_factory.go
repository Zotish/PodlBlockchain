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

	// ── 1. Direct pair ────────────────────────────────────────────────────────
	pk, _, _ := pairKey(tokenIn, tokenOut)
	pairAddr := ctx.Get("pairAddr:" + pk)
	if pairAddr != "" {
		_, err := ctx.Call(pairAddr, "Swap", []string{amountIn, minAmountOut, tokenIn})
		if err != nil {
			ctx.Revert("Swap failed: " + err.Error())
		}
		return
	}

	// ── 2. No direct pair — find best 2-hop route via Virtual Routing ─────────
	midToken, hop1Addr, hop2Addr := f.findBestRoute(ctx, tokenIn, tokenOut)
	if midToken == "" {
		ctx.Revert("no route found for " + tokenIn + " → " + tokenOut)
	}

	// Hop 1: tokenIn → midToken  (get intermediate amount out)
	hop1Res, err := ctx.Call(hop1Addr, "GetAmountOut", []string{amountIn, tokenIn})
	if err != nil || hop1Res == nil {
		ctx.Revert("route hop1 GetAmountOut failed")
	}
	midAmountOut := hop1Res.Output
	if midAmountOut == "" || midAmountOut == "0" {
		ctx.Revert("route hop1 gives zero output")
	}

	// Execute hop 1: tokenIn → midToken (0 minOut, hop2 will enforce slippage)
	_, err = ctx.Call(hop1Addr, "Swap", []string{amountIn, "0", tokenIn})
	if err != nil {
		ctx.Revert("route hop1 swap failed: " + err.Error())
	}

	// Execute hop 2: midToken → tokenOut (enforce minAmountOut here)
	_, err = ctx.Call(hop2Addr, "Swap", []string{midAmountOut, minAmountOut, midToken})
	if err != nil {
		ctx.Revert("route hop2 swap failed: " + err.Error())
	}

	ctx.Emit("MultiHopSwap", map[string]interface{}{
		"tokenIn":  tokenIn,
		"midToken": midToken,
		"tokenOut": tokenOut,
		"hop1":     hop1Addr,
		"hop2":     hop2Addr,
	})
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

	// Check direct first
	pk, _, _ := pairKey(tokenIn, tokenOut)
	direct := ctx.Get("pairAddr:" + pk)
	if direct != "" {
		ctx.Set("output", direct)
		ctx.Emit("BestRoute", map[string]interface{}{
			"type":     "direct",
			"pairAddr": direct,
			"hops":     "1",
		})
		return
	}

	mid, hop1, hop2 := f.findBestRoute(ctx, tokenIn, tokenOut)
	if mid == "" {
		ctx.Set("output", "")
		ctx.Emit("BestRoute", map[string]interface{}{"type": "none"})
		return
	}

	ctx.Set("output", hop1+","+hop2)
	ctx.Emit("BestRoute", map[string]interface{}{
		"type":     "2hop",
		"midToken": mid,
		"hop1":     hop1,
		"hop2":     hop2,
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
	pk, _, _ := pairKey(tokenIn, tokenOut)
	pairAddr := ctx.Get("pairAddr:" + pk)
	if pairAddr == "" {
		ctx.Set("output", "0")
		return
	}
	res, err := ctx.Call(pairAddr, "GetAmountOut", []string{amountIn, tokenIn})
	if err != nil {
		ctx.Set("output", "0")
		return
	}
	ctx.Set("output", res.Output)
}

// GetAmountIn returns required input to receive an exact output (read-only).
func (f *Factory) GetAmountIn(ctx *bc.Context, amountOut string, tokenIn string, tokenOut string) {
	tokenIn, tokenOut = normAddr(tokenIn), normAddr(tokenOut)
	pk, t0, _ := pairKey(tokenIn, tokenOut)
	pairAddr := ctx.Get("pairAddr:" + pk)
	if pairAddr == "" {
		ctx.Set("output", "0")
		return
	}
	// GetAmountIn: reverse lookup — call pair's GetAmountOut from the other direction
	// then use calcAmountIn
	tIn2 := tokenOut
	if tIn2 == t0 {
		tIn2 = tokenIn
	}
	_ = tIn2
	// Delegate to pair for read
	res, err := ctx.Call(pairAddr, "GetAmountOut", []string{amountOut, tokenOut})
	if err != nil {
		ctx.Set("output", "0")
		return
	}
	ctx.Set("output", res.Output)
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
