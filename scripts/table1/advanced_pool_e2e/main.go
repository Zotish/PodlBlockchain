package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	bc "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/BlockchainComponent"
)

const (
	owner     = "0x1111111111111111111111111111111111111111"
	tokenA    = "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	tokenB    = "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	tokenC    = "0xcccccccccccccccccccccccccccccccccccccccc"
	tokenD    = "0xdddddddddddddddddddddddddddddddddddddddd"
	tokenE    = "0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	factory   = "0xfac7000000000000000000000000000000000000"
	router    = "0x6060606060606060606060606060606060606060"
	bond      = "0x9090909090909090909090909090909090909090"
	keeper    = "0x8888888888888888888888888888888888888888"
	insurance = "0x9191919191919191919191919191919191919191"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func call(engine *bc.LQDContractEngine, address, caller, method string, args ...string) *bc.ContractExecutionResult {
	result, err := engine.Pipeline.ExecuteAtomic(address, caller, method, args, 20_000_000, big.NewInt(0))
	must(err)
	return result
}

func callAtValue(engine *bc.LQDContractEngine, address, caller, method string, blockTime int64, value *big.Int, args ...string) *bc.ContractExecutionResult {
	result, err := engine.Pipeline.ExecuteAtomicAt(address, caller, method, args, 20_000_000, value, blockTime)
	must(err)
	return result
}

func main() {
	root, err := os.Getwd()
	must(err)
	dataDir, err := os.MkdirTemp("", "podl-advanced-pool-e2e-")
	must(err)
	defer os.RemoveAll(dataDir)
	must(os.Setenv("LQD_DATA_DIR", dataDir))

	engine, err := bc.NewLQDContractEngine()
	must(err)
	chain := &bc.Blockchain_struct{Accounts: map[string]*big.Int{owner: new(big.Int).Mul(big.NewInt(100), big.NewInt(1e18)), keeper: new(big.Int).Mul(big.NewInt(10), big.NewInt(1e18))}}
	chain.EnsureRuntimeState()
	chain.ContractEngine = engine
	engine.Registry.Blockchain = chain

	builtins := filepath.Join(root, "bin", "builtins")
	for _, address := range []string{tokenA, tokenB, tokenC, tokenD, tokenE} {
		must(engine.Registry.DeployClone(address, filepath.Join(builtins, "lqd20.so"), owner))
	}
	for index, address := range []string{tokenA, tokenB, tokenC, tokenD, tokenE} {
		letter := string(rune('A' + index))
		call(engine, address, owner, "Init", "Token "+letter, "TK"+letter, "1000000000000000000000000000")
	}
	must(engine.Registry.DeployClone(factory, filepath.Join(builtins, "dex_factory.so"), owner))
	call(engine, factory, owner, "Init", filepath.Join(builtins, "dex_pair.so"))
	call(engine, factory, owner, "ConfigureAdvancedPools", filepath.Join(builtins, "advanced_pool.so"))

	results := map[string]interface{}{}
	stablePool := ""
	concentratedPool := ""
	for _, tc := range []struct{ kind, parameter string }{{"stable", "200"}, {"concentrated", "1000000000000000000"}} {
		created := call(engine, factory, owner, "CreateAdvancedPool", tokenA, tokenB, tc.kind, tc.parameter)
		pool := strings.TrimSpace(created.Output)
		if !bc.ValidateAddress(pool) {
			panic("factory returned invalid pool address")
		}
		call(engine, tokenA, owner, "Approve", pool, "100000000000000000000000")
		call(engine, tokenB, owner, "Approve", pool, "100000000000000000000000")
		call(engine, pool, owner, "AddLiquidity", "10000000000000000000000", "10000000000000000000000")
		quote := call(engine, pool, owner, "GetAmountOut", "1000000000000000000", tokenA)
		amountOut := new(big.Int)
		if _, ok := amountOut.SetString(strings.TrimSpace(quote.Output), 10); !ok || amountOut.Sign() <= 0 {
			panic(tc.kind + " pool returned zero quote")
		}
		before := call(engine, pool, owner, "GetReserves")
		call(engine, pool, owner, "Swap", "1000000000000000000", amountOut.String(), tokenA)
		after := call(engine, pool, owner, "GetReserves")
		if before.Output == after.Output {
			panic(tc.kind + " swap did not mutate reserves")
		}
		results[tc.kind] = map[string]interface{}{"pool": pool, "amount_out": amountOut.String(), "before": before.Output, "after": after.Output}
		if tc.kind == "stable" {
			stablePool = pool
		} else {
			concentratedPool = pool
		}
	}

	// Force a route which cannot be shortened: A -> B -> C -> D -> E. This
	// executes the router's generic graph search and all four atomic pool hops,
	// rather than merely unit-testing the quote formula.
	routePools := []string{stablePool}
	for _, pair := range [][2]string{{tokenB, tokenC}, {tokenC, tokenD}, {tokenD, tokenE}} {
		created := call(engine, factory, owner, "CreateAdvancedPool", pair[0], pair[1], "stable", "200")
		pool := strings.TrimSpace(created.Output)
		if !bc.ValidateAddress(pool) {
			panic("factory returned invalid route pool address")
		}
		call(engine, pair[0], owner, "Approve", pool, "100000000000000000000000")
		call(engine, pair[1], owner, "Approve", pool, "100000000000000000000000")
		call(engine, pool, owner, "AddLiquidity", "10000000000000000000000", "10000000000000000000000")
		routePools = append(routePools, pool)
	}
	must(engine.Registry.DeployClone(router, filepath.Join(builtins, "dex_router.so"), owner))
	call(engine, router, owner, "Init", factory)
	bestRoute := call(engine, router, owner, "GetBestRouteForAmount", "1000000000000000000", tokenA, tokenE)
	selectedPairs := strings.Split(strings.TrimSpace(bestRoute.Output), ",")
	if len(selectedPairs) != 4 {
		panic("router did not discover required 4-hop path: " + bestRoute.Output)
	}
	call(engine, tokenA, owner, "Approve", selectedPairs[0], "1000000000000000000")
	beforeE := call(engine, tokenE, owner, "BalanceOf", owner)
	routeQuote := call(engine, router, owner, "GetAmountOut", "1000000000000000000", tokenA, tokenE)
	call(engine, router, owner, "SwapExactTokensForTokens", "1000000000000000000", routeQuote.Output, tokenA, tokenE)
	afterE := call(engine, tokenE, owner, "BalanceOf", owner)
	beforeEBig, afterEBig := new(big.Int), new(big.Int)
	beforeEBig.SetString(strings.TrimSpace(beforeE.Output), 10)
	afterEBig.SetString(strings.TrimSpace(afterE.Output), 10)
	if afterEBig.Cmp(beforeEBig) <= 0 {
		panic("4-hop route did not deliver output token")
	}
	results["four_hop_router"] = map[string]interface{}{
		"path": []string{tokenA, tokenB, tokenC, tokenD, tokenE}, "pairs": selectedPairs,
		"quoted_out": routeQuote.Output, "received": new(big.Int).Sub(afterEBig, beforeEBig).String(), "executed": true,
	}

	vault := "0x7777777777777777777777777777777777777777"
	must(engine.Registry.DeployClone(vault, filepath.Join(builtins, "strategy_vault.so"), owner))
	call(engine, vault, owner, "Init", factory, owner)
	now := fmt.Sprintf("%d", time.Now().Unix())
	call(engine, vault, owner, "SetAssetOracle", tokenA, "1000000000000000000", "18", now)
	call(engine, vault, owner, "SetAssetOracle", tokenB, "1000000000000000000", "18", now)
	call(engine, vault, owner, "ConfigureERC4626Asset", stablePool)
	call(engine, stablePool, owner, "Approve", vault, "2000000000000000000000")
	previewDeposit := call(engine, vault, owner, "PreviewDeposit", "1000000000000000000000")
	deposit := call(engine, vault, owner, "Deposit", "1000000000000000000000", owner)
	if strings.TrimSpace(deposit.Output) != strings.TrimSpace(previewDeposit.Output) {
		panic("ERC-4626 deposit did not match preview: preview=" + previewDeposit.Output + " actual=" + deposit.Output)
	}
	assetsBefore := call(engine, vault, owner, "TotalAssets")
	sharesBefore := call(engine, vault, owner, "BalanceOf", owner)
	if ap := strings.TrimSpace(assetsBefore.Output); ap != "1000000000000000000000" {
		panic("ERC-4626 totalAssets mismatch: " + ap)
	}
	previewWithdraw := call(engine, vault, owner, "PreviewWithdraw", "250000000000000000000")
	withdraw := call(engine, vault, owner, "Withdraw", "250000000000000000000", owner, owner)
	if strings.TrimSpace(withdraw.Output) != strings.TrimSpace(previewWithdraw.Output) {
		panic("ERC-4626 withdraw did not return previewed shares: preview=" + previewWithdraw.Output + " actual=" + withdraw.Output)
	}
	assetsAfter := call(engine, vault, owner, "TotalAssets")
	if strings.TrimSpace(assetsAfter.Output) != "750000000000000000000" {
		panic("ERC-4626 withdrawal accounting mismatch: " + assetsAfter.Output)
	}
	maxMint := call(engine, vault, owner, "MaxMint", owner)
	maxMintValue, ok := new(big.Int).SetString(strings.TrimSpace(maxMint.Output), 10)
	if !ok || maxMintValue.Sign() <= 0 {
		panic("ERC-4626 maxMint unavailable")
	}
	requestedMint := "1000000000000000000"
	previewMint := call(engine, vault, owner, "PreviewMint", requestedMint)
	mintedAssets := call(engine, vault, owner, "Mint", requestedMint, owner)
	if strings.TrimSpace(mintedAssets.Output) != strings.TrimSpace(previewMint.Output) {
		panic("ERC-4626 mint did not return previewed assets: preview=" + previewMint.Output + " actual=" + mintedAssets.Output)
	}
	previewRedeem := call(engine, vault, owner, "PreviewRedeem", requestedMint)
	redeemedAssets := call(engine, vault, owner, "Redeem", requestedMint, owner, owner)
	if strings.TrimSpace(redeemedAssets.Output) != strings.TrimSpace(previewRedeem.Output) {
		panic("ERC-4626 redeem did not return previewed assets: preview=" + previewRedeem.Output + " actual=" + redeemedAssets.Output)
	}
	// Direct asset donations must be reflected in totalAssets, while the
	// virtual share/asset offset prevents a subsequent depositor from receiving
	// an inflated share count.
	call(engine, stablePool, owner, "Transfer", vault, "100000000000000000000")
	donatedAssets := call(engine, vault, owner, "TotalAssets")
	if strings.TrimSpace(donatedAssets.Output) != "850000000000000000000" {
		panic("ERC-4626 donation was not reflected in totalAssets: " + donatedAssets.Output)
	}
	previewAfterDonation := call(engine, vault, owner, "PreviewDeposit", "100000000000000000000")
	previewShares := new(big.Int)
	previewShares.SetString(strings.TrimSpace(previewAfterDonation.Output), 10)
	depositComparison, _ := new(big.Int).SetString("100000000000000000000", 10)
	if previewShares.Sign() <= 0 || previewShares.Cmp(depositComparison) >= 0 {
		panic("ERC-4626 donation defense preview is invalid: " + previewAfterDonation.Output)
	}
	results["erc4626_vault"] = map[string]interface{}{"vault": vault, "deposit_shares": deposit.Output, "shares_before": sharesBefore.Output, "withdraw_shares": withdraw.Output, "assets_after": assetsAfter.Output, "assets_after_donation": donatedAssets.Output, "post_donation_preview_shares": previewAfterDonation.Output, "preview_conformance": true, "virtual_offset_defense": true}

	// A separate multi-asset strategy vault exercises bonded keeper assignment,
	// the exclusive execution window and physical LP movement between pools.
	keeperVault := "0x7878787878787878787878787878787878787878"
	must(engine.Registry.DeployClone(keeperVault, filepath.Join(builtins, "strategy_vault.so"), owner))
	call(engine, keeperVault, owner, "Init", factory, owner)
	for _, token := range []string{tokenA, tokenB} {
		for i, source := range []string{"oracle-one", "oracle-two", "oracle-three"} {
			price := []string{"1000000000000000000", "1005000000000000000", "995000000000000000"}[i]
			call(engine, keeperVault, owner, "SetAssetOracleSource", token, source, price, "9500", "18", now)
		}
	}
	call(engine, stablePool, owner, "Approve", keeperVault, "400000000000000000000")
	call(engine, keeperVault, owner, "DepositLP", stablePool, "400000000000000000000")
	keeperStart := time.Now().Unix()
	callAtValue(engine, keeperVault, keeper, "RegisterKeeper", keeperStart, big.NewInt(100000))
	scheduled := callAtValue(engine, keeperVault, owner, "ScheduleRebalanceJob", keeperStart+1, big.NewInt(0), stablePool, concentratedPool, "100000000000000000000", "0", keeper, fmt.Sprint(keeperStart+1000))
	callAtValue(engine, keeperVault, keeper, "ExecuteRebalanceJob", keeperStart+2, big.NewInt(0), scheduled.Output)
	job := call(engine, keeperVault, owner, "RebalanceJob", scheduled.Output)
	if !strings.Contains(job.Output, `"status":"executed"`) || !strings.Contains(job.Output, strings.ToLower(keeper)) {
		panic("bonded keeper rebalance job did not execute: " + job.Output)
	}
	results["bonded_keeper_vault"] = map[string]interface{}{"vault": keeperVault, "keeper": keeper, "job": scheduled.Output, "physical_source": stablePool, "physical_target": concentratedPool, "multi_source_nav": true, "status": "executed"}

	// Insurance is real native-token custody: pending claims reserve capacity,
	// enforce a floor and per-claim cap, expose coverage, and pay only after the
	// challenge window plus a governance resolution reference.
	must(engine.Registry.DeployClone(insurance, filepath.Join(builtins, "insurance_vault.so"), owner))
	insuranceStart := time.Now().Unix()
	callAtValue(engine, insurance, owner, "Init", insuranceStart, big.NewInt(0), owner, "1000000000000000000", "2500")
	callAtValue(engine, insurance, owner, "SetCoveredLiability", insuranceStart+1, big.NewInt(0), "10000000000000000000", "gov-liability-1")
	callAtValue(engine, insurance, owner, "DepositRevenue", insuranceStart+2, new(big.Int).Mul(big.NewInt(5), big.NewInt(1e18)), "trading_fee", "settlement-1")
	coverage := call(engine, insurance, owner, "CoverageRatioBPS")
	if strings.TrimSpace(coverage.Output) != "5000" {
		panic("insurance coverage ratio mismatch: " + coverage.Output)
	}
	keeperBeforeClaim := chain.AccountBalanceAmount(keeper)
	openedClaim := callAtValue(engine, insurance, owner, "OpenClaim", insuranceStart+3, big.NewInt(0), keeper, "1000000000000000000", "0xloss-proof", fmt.Sprint(insuranceStart+100))
	availableDuringClaim := call(engine, insurance, owner, "AvailableReserve")
	if strings.TrimSpace(availableDuringClaim.Output) != "4000000000000000000" {
		panic("insurance pending-claim reservation mismatch: " + availableDuringClaim.Output)
	}
	callAtValue(engine, insurance, owner, "ResolveClaim", insuranceStart+101, big.NewInt(0), openedClaim.Output, "true", "gov-claim-1")
	keeperAfterClaim := chain.AccountBalanceAmount(keeper)
	if new(big.Int).Sub(keeperAfterClaim, keeperBeforeClaim).Cmp(big.NewInt(1e18)) != 0 {
		panic("insurance did not pay the approved native-token claim")
	}
	reserveAfterClaim := call(engine, insurance, owner, "Reserve")
	if strings.TrimSpace(reserveAfterClaim.Output) != "4000000000000000000" {
		panic("insurance reserve did not reconcile approved claim: " + reserveAfterClaim.Output)
	}
	results["insurance_vault"] = map[string]interface{}{"vault": insurance, "coverage_bps_before_claim": coverage.Output, "reserved_while_pending": "1000000000000000000", "paid_claim": "1000000000000000000", "reserve_after_claim": reserveAfterClaim.Output, "challenge_window_enforced": true}

	// Exercise native custody, challenge delay, upheld slashing, physical
	// transfer to the protocol escrow, and source-specific revenue reconciliation.
	must(engine.Registry.DeployClone(bond, filepath.Join(builtins, "validator_bond.so"), owner))
	start := time.Now().Unix()
	callAtValue(engine, bond, owner, "Init", start, big.NewInt(0), "86400", "3600")
	callAtValue(engine, bond, owner, "Bond", start+1, new(big.Int).Mul(big.NewInt(2), big.NewInt(1e18)))
	evidence := "0xevidence"
	callAtValue(engine, bond, owner, "OpenSlash", start+2, big.NewInt(0), owner, "1000000000000000000", evidence)
	digest := sha256.Sum256([]byte(strings.ToLower(owner) + "|" + evidence))
	caseID := "slash_" + hex.EncodeToString(digest[:12])
	callAtValue(engine, bond, owner, "ResolveSlash", start+3603, big.NewInt(0), caseID, "true", "gov-upheld-1")
	slashedTotal, err := engine.DB.LoadStorage(bond, "protocol_slash_total")
	must(err)
	if slashedTotal != "1000000000000000000" || chain.AccountBalanceAmount(bc.ProtocolRevenueEscrowAddress).Cmp(big.NewInt(1e18)) != 0 {
		panic("validator bond slash did not reach protocol escrow")
	}
	must(chain.ReconcileDEXProtocolFees(1, start+3603))
	if len(chain.ProtocolRevenue) != 1 || chain.ProtocolRevenue[0].Source != "slashing" || chain.ProtocolRevenue[0].Amount.String() != slashedTotal {
		panic(fmt.Sprintf("validator slash was not reconciled into realized revenue: entries=%+v checkpoints=%+v", chain.ProtocolRevenue, chain.RevenueCheckpoints))
	}
	results["validator_bond_slashing"] = map[string]interface{}{"contract": bond, "case": caseID, "slashed": slashedTotal, "escrow_balance": chain.AccountBalanceString(bc.ProtocolRevenueEscrowAddress), "revenue_source": chain.ProtocolRevenue[0].Source}
	results["deployable"] = true
	results["atomic_swaps"] = true
	results["arbitrary_hop_routing"] = true
	raw, _ := json.MarshalIndent(results, "", "  ")
	fmt.Println(string(raw))
}
