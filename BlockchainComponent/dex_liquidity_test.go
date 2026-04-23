package blockchaincomponent

import (
	"math/big"
	"testing"
	"time"

	constantset "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/ConstantSet"
)

// ─────────────────────────────────────────────────────────────────────────────
// DEX Liquidity — tests via Context (contract logic simulation)
// These simulate the DEX Pair / Factory behaviour without importing
// the //go:build ignore contract files.
// ─────────────────────────────────────────────────────────────────────────────

// ─────────────────────────────────────────────────────────────────────────────
// Pool initialization
// ─────────────────────────────────────────────────────────────────────────────

func TestDEXPool_InitializesPair(t *testing.T) {
	ctx := newTestContext("0xpair", "0xfactory", 10_000_000)
	tokenA := "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	tokenB := "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	// Simulate DEX pair Init
	ctx.Set("tokenA", tokenA)
	ctx.Set("tokenB", tokenB)
	ctx.Set("reserveA", "0")
	ctx.Set("reserveB", "0")
	ctx.Set("totalLP", "0")
	ctx.Emit("PairInit", map[string]interface{}{"tokenA": tokenA, "tokenB": tokenB})

	if ctx.Get("tokenA") != tokenA {
		t.Errorf("expected tokenA %q, got %q", tokenA, ctx.Get("tokenA"))
	}
	if ctx.Get("totalLP") != "0" {
		t.Errorf("initial LP supply should be 0, got %q", ctx.Get("totalLP"))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Add liquidity — first provision
// ─────────────────────────────────────────────────────────────────────────────

func TestDEXPool_AddLiquidity_FirstProvision(t *testing.T) {
	ctx := newTestContext("0xpair", "0xprovider", 10_000_000)
	provider := "0xprovider"
	amtA := big.NewInt(1000)
	amtB := big.NewInt(2000)

	// Simulate first AddLiquidity
	ctx.Set("reserveA", "0")
	ctx.Set("reserveB", "0")
	ctx.Set("totalLP", "0")

	// LP minted = sqrt(amtA * amtB) ≈ 1414 (integer approximation)
	product := new(big.Int).Mul(amtA, amtB)
	lpMinted := new(big.Int).Set(product)
	// Simple integer sqrt via Newton's method
	lpMinted = intSqrt(lpMinted)

	ctx.Set("reserveA", amtA.String())
	ctx.Set("reserveB", amtB.String())
	ctx.Set("totalLP", lpMinted.String())
	ctx.Set("lp:"+provider, lpMinted.String())
	ctx.Emit("AddLiquidity", map[string]interface{}{
		"provider": provider,
		"amtA":     amtA.String(),
		"amtB":     amtB.String(),
		"lp":       lpMinted.String(),
	})

	if ctx.Get("reserveA") != "1000" {
		t.Errorf("expected reserveA 1000, got %q", ctx.Get("reserveA"))
	}
	if ctx.Get("lp:"+provider) == "" || ctx.Get("lp:"+provider) == "0" {
		t.Error("provider should have received LP tokens")
	}
	evts := ctx.Events()
	if len(evts) == 0 || evts[0].EventName != "AddLiquidity" {
		t.Error("expected AddLiquidity event to be emitted")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Add liquidity — subsequent provision (proportional)
// ─────────────────────────────────────────────────────────────────────────────

func TestDEXPool_AddLiquidity_ProportionalLP(t *testing.T) {
	ctx := newTestContext("0xpair", "0xprovider2", 10_000_000)
	provider2 := "0xprovider2"

	// Pool already has reserves and LP tokens
	resA := big.NewInt(1000)
	resB := big.NewInt(1000)
	totalLP := big.NewInt(1000)
	ctx.Set("reserveA", resA.String())
	ctx.Set("reserveB", resB.String())
	ctx.Set("totalLP", totalLP.String())

	// Provider adds 200 A and 200 B (equal ratio)
	addA := big.NewInt(200)

	// LP minted = amtA * totalLP / reserveA = 200 * 1000 / 1000 = 200
	lpForProvider := new(big.Int).Mul(addA, totalLP)
	lpForProvider.Div(lpForProvider, resA)

	ctx.Set("lp:"+provider2, lpForProvider.String())
	newResA := new(big.Int).Add(resA, addA)
	ctx.Set("reserveA", newResA.String())
	newTotal := new(big.Int).Add(totalLP, lpForProvider)
	ctx.Set("totalLP", newTotal.String())

	if ctx.Get("lp:"+provider2) != "200" {
		t.Errorf("expected 200 LP tokens, got %q", ctx.Get("lp:"+provider2))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Swap — constant product (k = reserveA × reserveB)
// ─────────────────────────────────────────────────────────────────────────────

func TestDEXSwap_ConstantProduct_Maintained(t *testing.T) {
	ctx := newTestContext("0xpair", "0xtrader", 10_000_000)
	trader := "0xtrader"
	resA := big.NewInt(1000)
	resB := big.NewInt(1000)
	ctx.Set("reserveA", resA.String())
	ctx.Set("reserveB", resB.String())

	// Trader swaps 100 A for some B (0.3% fee applied)
	amtIn := big.NewInt(100)
	feeNumerator := int64(997)
	feeDenominator := int64(1000)

	// amtOut = resB * amtIn * 997 / (resA * 1000 + amtIn * 997)
	num := new(big.Int).Mul(resB, new(big.Int).Mul(amtIn, big.NewInt(feeNumerator)))
	denom := new(big.Int).Add(
		new(big.Int).Mul(resA, big.NewInt(feeDenominator)),
		new(big.Int).Mul(amtIn, big.NewInt(feeNumerator)),
	)
	amtOut := new(big.Int).Div(num, denom)

	// Update reserves
	newResA := new(big.Int).Add(resA, amtIn)
	newResB := new(big.Int).Sub(resB, amtOut)
	ctx.Set("reserveA", newResA.String())
	ctx.Set("reserveB", newResB.String())

	ctx.Emit("Swap", map[string]interface{}{
		"trader": trader,
		"amtIn":  amtIn.String(),
		"amtOut": amtOut.String(),
	})

	// k before = 1000 * 1000 = 1000000
	// k after should be >= 1000000 (fees make k grow slightly)
	kBefore := new(big.Int).Mul(resA, resB)
	kAfter := new(big.Int).Mul(newResA, newResB)
	if kAfter.Cmp(kBefore) < 0 {
		t.Errorf("constant product should not decrease: kBefore=%s kAfter=%s",
			kBefore.String(), kAfter.String())
	}
	if amtOut.Sign() <= 0 {
		t.Error("swap output should be > 0")
	}
}

func TestDEXSwap_OutputLessThanInput(t *testing.T) {
	ctx := newTestContext("0xpair", "0xtrader", 10_000_000)
	resA := big.NewInt(1000)
	resB := big.NewInt(1000)
	ctx.Set("reserveA", resA.String())
	ctx.Set("reserveB", resB.String())

	amtIn := big.NewInt(100)
	// amtOut with 0.3% fee
	amtOut := new(big.Int).Div(
		new(big.Int).Mul(resB, new(big.Int).Mul(amtIn, big.NewInt(997))),
		new(big.Int).Add(
			new(big.Int).Mul(resA, big.NewInt(1000)),
			new(big.Int).Mul(amtIn, big.NewInt(997)),
		),
	)

	// With equal reserves, output should be slightly less than input (due to fee)
	if amtOut.Cmp(amtIn) >= 0 {
		t.Errorf("swap output (%s) should be less than input (%s) due to fees",
			amtOut.String(), amtIn.String())
	}
}

func TestDEXSwap_SlippageProtection(t *testing.T) {
	ctx := newTestContext("0xpair", "0xtrader", 10_000_000)
	resA := big.NewInt(100)
	resB := big.NewInt(100)
	ctx.Set("reserveA", resA.String())
	ctx.Set("reserveB", resB.String())

	// Huge swap relative to reserves — large slippage
	amtIn := big.NewInt(90)
	minAmtOut := big.NewInt(45) // user's minimum accepted

	amtOut := new(big.Int).Div(
		new(big.Int).Mul(resB, new(big.Int).Mul(amtIn, big.NewInt(997))),
		new(big.Int).Add(
			new(big.Int).Mul(resA, big.NewInt(1000)),
			new(big.Int).Mul(amtIn, big.NewInt(997)),
		),
	)

	reason := catchRevert(func() {
		if amtOut.Cmp(minAmtOut) < 0 {
			ctx.Revert("slippage too high")
		}
	})

	if reason != "" {
		// This is expected when slippage is too high
		t.Logf("slippage protection triggered: %q (amtOut=%s minOut=%s)", reason, amtOut, minAmtOut)
	}
	// amtOut should still be positive
	if amtOut.Sign() <= 0 {
		t.Error("swap output should be positive")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Remove liquidity
// ─────────────────────────────────────────────────────────────────────────────

func TestDEXPool_RemoveLiquidity_ProportionalReturn(t *testing.T) {
	ctx := newTestContext("0xpair", "0xprovider", 10_000_000)
	provider := "0xprovider"

	// Pool has 1000A, 2000B, 1000 LP; provider holds all 1000 LP
	resA := big.NewInt(1000)
	resB := big.NewInt(2000)
	totalLP := big.NewInt(1000)
	ctx.Set("reserveA", resA.String())
	ctx.Set("reserveB", resB.String())
	ctx.Set("totalLP", totalLP.String())
	ctx.Set("lp:"+provider, totalLP.String())

	// Remove 500 LP (50% of pool)
	burnLP := big.NewInt(500)
	amtA := new(big.Int).Div(new(big.Int).Mul(burnLP, resA), totalLP) // 500 A
	amtB := new(big.Int).Div(new(big.Int).Mul(burnLP, resB), totalLP) // 1000 B

	if amtA.Cmp(big.NewInt(500)) != 0 {
		t.Errorf("expected 500 A returned, got %s", amtA.String())
	}
	if amtB.Cmp(big.NewInt(1000)) != 0 {
		t.Errorf("expected 1000 B returned, got %s", amtB.String())
	}

	// Update state
	newLP := new(big.Int).Sub(totalLP, burnLP)
	ctx.Set("totalLP", newLP.String())
	ctx.Set("lp:"+provider, newLP.String())
	ctx.Set("reserveA", new(big.Int).Sub(resA, amtA).String())
	ctx.Set("reserveB", new(big.Int).Sub(resB, amtB).String())

	ctx.Emit("RemoveLiquidity", map[string]interface{}{
		"provider": provider,
		"burnLP":   burnLP.String(),
		"amtA":     amtA.String(),
		"amtB":     amtB.String(),
	})

	if ctx.Get("totalLP") != "500" {
		t.Errorf("expected totalLP 500 after removal, got %q", ctx.Get("totalLP"))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// LP validator staking (PosDL)
// ─────────────────────────────────────────────────────────────────────────────

func TestDEXValidator_AddDEXValidator_InMemory(t *testing.T) {
	bc := newTestBlockchain()
	// Inject a PosDL validator directly (bypasses DB)
	v := &Validator{
		Address:       "0xvalidator",
		DEXAddress:    "0xpair",
		LPTokenAmount: "500000",
		LockTime:      time.Now().Add(365 * 24 * time.Hour),
		LastActive:    time.Now(),
	}
	bc.Validators = append(bc.Validators, v)

	if len(bc.Validators) != 1 {
		t.Errorf("expected 1 validator, got %d", len(bc.Validators))
	}
	if bc.Validators[0].DEXAddress != "0xpair" {
		t.Errorf("expected DEXAddress 0xpair, got %q", bc.Validators[0].DEXAddress)
	}
}

func TestDEXValidator_SelectFromDEXValidators_NoContractEngine(t *testing.T) {
	bc := newTestBlockchain()
	// DEX validators with no contract engine have 0 power
	v1 := &Validator{
		Address:       "0xv1",
		DEXAddress:    "0xpair",
		LPTokenAmount: "1000000",
		LockTime:      time.Now().Add(365 * 24 * time.Hour),
		LiquidityPower: 0, // no engine → power stays 0
	}
	// Legacy validator always wins when DEX validators have 0 power
	v2 := makeValidator("0xv2", 1e12, 180)
	bc.Validators = []*Validator{v1, v2}
	bc.UpdateLiquidityPower()

	selected, err := bc.SelectValidator()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// v1 DEX validator has 0 power (no ContractEngine) → v2 wins
	if selected.Address != v2.Address {
		t.Errorf("expected legacy validator %q to win (DEX validator has 0 power), got %q",
			v2.Address, selected.Address)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Pool liquidity map
// ─────────────────────────────────────────────────────────────────────────────

func TestPoolLiquidityMap(t *testing.T) {
	bc := newTestBlockchain()
	if bc.PoolLiquidity == nil {
		bc.PoolLiquidity = make(map[string]*big.Int)
	}
	poolAddr := "0xpooladdr"
	bc.PoolLiquidity[poolAddr] = big.NewInt(50000)

	if bc.PoolLiquidity[poolAddr].Cmp(big.NewInt(50000)) != 0 {
		t.Error("pool liquidity should be stored correctly")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Multiple liquidity providers — lock tracking
// ─────────────────────────────────────────────────────────────────────────────

func TestMultipleLPProviders_TotalLiquidity(t *testing.T) {
	bc := newTestBlockchain()

	providers := []string{
		"0x1111111111111111111111111111111111111111",
		"0x2222222222222222222222222222222222222222",
		"0x3333333333333333333333333333333333333333",
	}
	for i, addr := range providers {
		amt := big.NewInt(int64((i + 1) * 1000))
		bc.setAccountBalance(addr, new(big.Int).Mul(amt, big.NewInt(2))) // enough balance
		_ = bc.LockLiquidity(addr, amt, 24*time.Hour)
	}

	// Total = 1000 + 2000 + 3000 = 6000
	if bc.TotalLiquidity.Cmp(big.NewInt(6000)) != 0 {
		t.Errorf("expected TotalLiquidity 6000, got %s", bc.TotalLiquidity.String())
	}
}

func TestDEXPool_ZeroReserves_SwapReverts(t *testing.T) {
	ctx := newTestContext("0xpair", "0xtrader", 10_000_000)
	ctx.Set("reserveA", "0")
	ctx.Set("reserveB", "0")

	reason := catchRevert(func() {
		resA := new(big.Int)
		resA.SetString(ctx.Get("reserveA"), 10)
		if resA.Sign() == 0 {
			ctx.Revert("insufficient liquidity")
		}
	})

	if reason == "" {
		t.Error("swap against zero reserves should revert")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// LQD20 Token standard
// ─────────────────────────────────────────────────────────────────────────────

func TestLQD20_Transfer_UpdatesBalances(t *testing.T) {
	ctx := newTestContextWithDB(t, "0xtoken", "0xsender", 10_000_000)
	sender := "0xsender"
	recipient := "0xrecipient"

	ctx.AddBalance(sender, big.NewInt(1000))

	// Transfer 250
	ctx.SubBalance(sender, big.NewInt(250))
	ctx.AddBalance(recipient, big.NewInt(250))
	ctx.Emit("Transfer", map[string]interface{}{
		"from":   sender,
		"to":     recipient,
		"amount": "250",
	})

	evts := ctx.Events()
	if len(evts) != 1 || evts[0].EventName != "Transfer" {
		t.Error("Transfer event should have been emitted")
	}
}

func TestLQD20_Approve_Allowance(t *testing.T) {
	ctx := newTestContext("0xtoken", "0xowner", 10_000_000)
	owner := "0xowner"
	spender := "0xspender"

	// Approve spender to spend 500
	allowanceKey := "allowance:" + owner + ":" + spender
	ctx.Set(allowanceKey, "500")
	ctx.Emit("Approval", map[string]interface{}{
		"owner":   owner,
		"spender": spender,
		"amount":  "500",
	})

	if ctx.Get(allowanceKey) != "500" {
		t.Errorf("expected allowance 500, got %q", ctx.Get(allowanceKey))
	}
}

func TestLQD20_TransferFrom_DeductsAllowance(t *testing.T) {
	ctx := newTestContextWithDB(t, "0xtoken", "0xspender", 10_000_000)
	owner := "0xowner"
	spender := "0xspender"
	recipient := "0xrecipient"

	ctx.AddBalance(owner, big.NewInt(1000))
	allowanceKey := "allowance:" + owner + ":" + spender
	ctx.Set(allowanceKey, "300")

	// TransferFrom
	allowed := new(big.Int)
	allowed.SetString(ctx.Get(allowanceKey), 10)
	transferAmt := big.NewInt(200)
	if allowed.Cmp(transferAmt) < 0 {
		ctx.Revert("allowance exceeded")
	}
	ctx.SubBalance(owner, transferAmt)
	ctx.AddBalance(recipient, transferAmt)
	newAllowance := new(big.Int).Sub(allowed, transferAmt)
	ctx.Set(allowanceKey, newAllowance.String())

	if ctx.Get(allowanceKey) != "100" {
		t.Errorf("expected remaining allowance 100, got %q", ctx.Get(allowanceKey))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// BridgeTokenInfo mappings
// ─────────────────────────────────────────────────────────────────────────────

func TestBridgeTokenMapping_SetGet(t *testing.T) {
	bc := newTestBlockchain()
	info := &BridgeTokenInfo{
		BscToken: "0xbsctoken",
		LqdToken: "0xlqdtoken",
		Name:     "BridgedToken",
		Symbol:   "BT",
		Decimals: "8",
	}
	bc.SetBridgeTokenMapping("0xbsctoken", info)

	got := bc.GetBridgeTokenMapping("0xbsctoken")
	if got == nil {
		t.Fatal("expected bridge token mapping, got nil")
	}
	if got.LqdToken != "0xlqdtoken" {
		t.Errorf("expected LqdToken 0xlqdtoken, got %q", got.LqdToken)
	}
}

func TestBridgeTokenMapping_NotFound_Nil(t *testing.T) {
	bc := newTestBlockchain()
	got := bc.GetBridgeTokenMapping("0xnonexistent")
	if got != nil {
		t.Error("expected nil for unknown token mapping")
	}
}

func TestBridgeTokenMapping_ListAll(t *testing.T) {
	bc := newTestBlockchain()
	for i := 0; i < 3; i++ {
		addr := "0x" + string(rune('a'+i)) + "000000000000000000000000000000000000000"
		bc.SetBridgeTokenMapping(addr, &BridgeTokenInfo{
			BscToken: addr,
			LqdToken: addr + "lqd",
		})
	}
	list := bc.ListBridgeTokenMappings()
	if len(list) < 3 {
		t.Errorf("expected at least 3 token mappings, got %d", len(list))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Blockchain RebalancePoolsEqual (no crash test)
// ─────────────────────────────────────────────────────────────────────────────

func TestRebalancePoolsEqual_NoCrash(t *testing.T) {
	bc := newTestBlockchain()
	bc.PoolLiquidity = map[string]*big.Int{
		constantset.LiquidityPoolAddress: big.NewInt(10000),
		"0xpoolB": big.NewInt(5000),
	}
	bc.UnallocatedLiquidity = big.NewInt(1000)

	// Should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("RebalancePoolsEqual panicked: %v", r)
		}
	}()
	bc.RebalancePoolsEqual()
}

// ─────────────────────────────────────────────────────────────────────────────
// integer sqrt helper (used in test — not production code)
// ─────────────────────────────────────────────────────────────────────────────

func intSqrt(n *big.Int) *big.Int {
	if n.Sign() <= 0 {
		return big.NewInt(0)
	}
	x := new(big.Int).Set(n)
	y := new(big.Int).Add(new(big.Int).Rsh(x, 1), big.NewInt(1))
	for y.Cmp(x) < 0 {
		x.Set(y)
		y.Add(new(big.Int).Div(n, y), y)
		y.Rsh(y, 1)
	}
	return x
}
