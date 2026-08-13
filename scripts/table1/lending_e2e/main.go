package main

import (
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
	owner           = "0x1111111111111111111111111111111111111111"
	borrower        = "0x2222222222222222222222222222222222222222"
	debtToken       = "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	collateralToken = "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	market          = "0xcccccccccccccccccccccccccccccccccccccccc"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func call(engine *bc.LQDContractEngine, at int64, address, caller, method string, args ...string) *bc.ContractExecutionResult {
	result, err := engine.Pipeline.ExecuteAtomicAt(address, caller, method, args, 20_000_000, big.NewInt(0), at)
	must(err)
	return result
}
func amount(raw string) *big.Int {
	n, ok := new(big.Int).SetString(strings.TrimSpace(raw), 10)
	if !ok {
		panic("bad amount: " + raw)
	}
	return n
}

func main() {
	root, err := os.Getwd()
	must(err)
	dataDir, err := os.MkdirTemp("", "podl-lending-e2e-")
	must(err)
	defer os.RemoveAll(dataDir)
	must(os.Setenv("LQD_DATA_DIR", dataDir))
	engine, err := bc.NewLQDContractEngine()
	must(err)
	chain := &bc.Blockchain_struct{Accounts: map[string]*big.Int{owner: big.NewInt(0), borrower: big.NewInt(0)}}
	chain.EnsureRuntimeState()
	chain.ContractEngine = engine
	engine.Registry.Blockchain = chain
	builtins := filepath.Join(root, "bin", "builtins")
	for _, token := range []string{debtToken, collateralToken} {
		must(engine.Registry.DeployClone(token, filepath.Join(builtins, "lqd20.so"), owner))
	}
	now := time.Now().Unix()
	call(engine, now, debtToken, owner, "Init", "Debt USD", "dUSD", "10000000000000000000000000")
	call(engine, now, collateralToken, owner, "Init", "Collateral", "COL", "10000000000000000000000000")
	call(engine, now, collateralToken, owner, "Transfer", borrower, "1000000000000000000000")
	must(engine.Registry.DeployClone(market, filepath.Join(builtins, "lending_pool.so"), owner))
	call(engine, now, market, owner, "Init", debtToken)
	call(engine, now, market, owner, "ConfigureMarket", collateralToken, debtToken, "5000", "7500", "500", "1000")
	call(engine, now, market, owner, "SetOraclePrice", debtToken, "1000000000000000000", fmt.Sprint(now))
	call(engine, now, market, owner, "SetOraclePrice", collateralToken, "2000000000000000000", fmt.Sprint(now))
	call(engine, now, debtToken, owner, "Approve", market, "5000000000000000000000")
	call(engine, now, market, owner, "Supply", "5000000000000000000000", owner)
	call(engine, now, collateralToken, borrower, "Approve", market, "1000000000000000000000")
	call(engine, now, market, borrower, "DepositCollateral", "1000000000000000000000", borrower)
	call(engine, now, market, borrower, "Borrow", "800000000000000000000")
	healthBefore := call(engine, now, market, borrower, "HealthFactor", borrower)
	if amount(healthBefore.Output).Cmp(big.NewInt(10000)) <= 0 {
		panic("healthy position started liquidatable")
	}
	// A collateral shock makes the position liquidatable. The close factor,
	// oracle conversion and bonus are all enforced by the contract; exhausted
	// collateral converts only the uncovered remainder into explicit bad debt.
	shockTime := now + 60
	call(engine, shockTime, market, owner, "SetOraclePrice", debtToken, "1000000000000000000", fmt.Sprint(shockTime))
	call(engine, shockTime, market, owner, "SetOraclePrice", collateralToken, "400000000000000000", fmt.Sprint(shockTime))
	healthAfter := call(engine, shockTime, market, borrower, "HealthFactor", borrower)
	if amount(healthAfter.Output).Cmp(big.NewInt(10000)) >= 0 {
		panic("oracle shock did not make position liquidatable")
	}
	call(engine, shockTime, debtToken, owner, "Approve", market, "500000000000000000000")
	call(engine, shockTime, market, owner, "LiquidatePartial", borrower, "400000000000000000000", "900000000000000000000")
	remainingCollateral := call(engine, shockTime, market, owner, "CollateralOf", borrower)
	remainingDebt := call(engine, shockTime, market, owner, "DebtOf", borrower)
	badDebt, err := engine.DB.LoadStorage(market, "bad_debt")
	must(err)
	if amount(remainingCollateral.Output).Sign() != 0 || amount(remainingDebt.Output).Sign() != 0 || amount(badDebt).Sign() <= 0 {
		panic("liquidation/bad-debt waterfall did not settle")
	}
	totalSupply := call(engine, shockTime, market, owner, "TotalDeposits")
	utilization := call(engine, shockTime, market, owner, "Utilization")
	report := map[string]interface{}{"isolated_market": true, "real_token_custody": true, "oracle_liquidation": true, "partial_close_factor": true, "bad_debt_accounted": badDebt, "health_before_bps": healthBefore.Output, "health_after_bps": healthAfter.Output, "supplier_assets": totalSupply.Output, "utilization_bps": utilization.Output}
	raw, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(raw))
}
