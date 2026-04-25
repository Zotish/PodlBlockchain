package main

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"time"

	blockchaincomponent "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/BlockchainComponent"
	constantset "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/ConstantSet"
)

type probeResult struct {
	TreasuryBefore string            `json:"treasury_before"`
	TreasuryAfter  string            `json:"treasury_after"`
	Delta          string            `json:"delta"`
	ExpectedOut    string            `json:"expected_out"`
	ExpectedProfit string            `json:"expected_profit"`
	TimedOut       bool              `json:"timed_out"`
	PairsBefore    map[string]string `json:"pairs_before"`
	PairsAfter     map[string]string `json:"pairs_after"`
}

func bi(v string) *big.Int {
	z, ok := new(big.Int).SetString(v, 10)
	if !ok {
		return big.NewInt(0)
	}
	return z
}

func putPair(db *blockchaincomponent.ContractDB, pairAddr, token0, token1, reserve0, reserve1 string) error {
	items := map[string]string{
		"token0":   token0,
		"token1":   token1,
		"reserve0": reserve0,
		"reserve1": reserve1,
	}
	for k, v := range items {
		if err := db.SaveStorage(pairAddr, k, v); err != nil {
			return err
		}
	}
	return nil
}

func mustStorage(db *blockchaincomponent.ContractDB, pairAddr string) map[string]string {
	state, err := db.LoadAllStorage(pairAddr)
	if err != nil {
		panic(err)
	}
	return state
}

func amountOut(amtIn, resIn, resOut *big.Int) *big.Int {
	if amtIn.Sign() == 0 || resIn.Sign() == 0 || resOut.Sign() == 0 {
		return big.NewInt(0)
	}
	fee := new(big.Int).Mul(amtIn, big.NewInt(997))
	num := new(big.Int).Mul(fee, resOut)
	den := new(big.Int).Add(new(big.Int).Mul(resIn, big.NewInt(1000)), fee)
	return new(big.Int).Div(num, den)
}

func main() {
	constantset.BLOCKCHAIN_DB_PATH = fmt.Sprintf("/tmp/podl-arb-probe-%d", time.Now().UnixNano())
	tmpDir, err := os.MkdirTemp("/tmp", "podl-arb-probe-work-*")
	if err != nil {
		panic(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		panic(err)
	}

	engine, err := blockchaincomponent.NewLQDContractEngine()
	if err != nil {
		panic(err)
	}
	bc := &blockchaincomponent.Blockchain_struct{
		Accounts:       map[string]*big.Int{},
		BridgeRequests: map[string]*blockchaincomponent.BridgeRequest{},
		BridgeTokenMap: map[string]*blockchaincomponent.BridgeTokenInfo{},
		ContractEngine: engine,
	}
	engine.Registry.Blockchain = bc

	db := engine.DB
	pairLA := "0x1000000000000000000000000000000000000001"
	pairAB := "0x1000000000000000000000000000000000000002"
	pairBL := "0x1000000000000000000000000000000000000003"
	tokenA := "0x2000000000000000000000000000000000000001"
	tokenB := "0x2000000000000000000000000000000000000002"

	if err := putPair(db, pairLA, "lqd", tokenA, "100000000000", "100000000000"); err != nil {
		panic(err)
	}
	if err := putPair(db, pairAB, tokenA, tokenB, "100000000000", "250000000000"); err != nil {
		panic(err)
	}
	if err := putPair(db, pairBL, tokenB, "lqd", "100000000000", "100000000000"); err != nil {
		panic(err)
	}

	bc.AccountsMu.Lock()
	bc.Accounts[constantset.LiquidityPoolAddress] = bi("100000000000")
	bc.AccountsMu.Unlock()

	metrics := []blockchaincomponent.PoolMetrics{
		{
			PairAddress: pairLA,
			Token0:      "lqd",
			Token1:      tokenA,
			Reserve0:    bi("100000000000"),
			Reserve1:    bi("100000000000"),
			VolumeIn:    big.NewInt(0),
		},
		{
			PairAddress: pairAB,
			Token0:      tokenA,
			Token1:      tokenB,
			Reserve0:    bi("100000000000"),
			Reserve1:    bi("250000000000"),
			VolumeIn:    big.NewInt(0),
		},
		{
			PairAddress: pairBL,
			Token0:      tokenB,
			Token1:      "lqd",
			Reserve0:    bi("100000000000"),
			Reserve1:    bi("100000000000"),
			VolumeIn:    big.NewInt(0),
		},
	}

	beforeLA := mustStorage(db, pairLA)
	beforeAB := mustStorage(db, pairAB)
	beforeBL := mustStorage(db, pairBL)

	bc.AccountsMu.RLock()
	treasuryBefore := new(big.Int).Set(bc.Accounts[constantset.LiquidityPoolAddress])
	bc.AccountsMu.RUnlock()

	maxCapital := new(big.Int).Div(new(big.Int).Mul(new(big.Int).Set(treasuryBefore), big.NewInt(1000)), big.NewInt(10000))
	testInput := new(big.Int).Div(new(big.Int).Set(maxCapital), big.NewInt(10))
	expA := amountOut(testInput, bi("100000000000"), bi("100000000000"))
	expB := amountOut(expA, bi("100000000000"), bi("250000000000"))
	expOut := amountOut(expB, bi("100000000000"), bi("100000000000"))
	expProfit := new(big.Int).Sub(expOut, testInput)

	done := make(chan struct{})
	go func() {
		blockchaincomponent.NewProtocolArb().RunArbitrage(bc, metrics)
		close(done)
	}()

	timedOut := false
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		timedOut = true
	}

	afterLA := beforeLA
	afterAB := beforeAB
	afterBL := beforeBL
	treasuryAfter := new(big.Int).Set(treasuryBefore)
	if !timedOut {
		afterLA = mustStorage(db, pairLA)
		afterAB = mustStorage(db, pairAB)
		afterBL = mustStorage(db, pairBL)
		bc.AccountsMu.RLock()
		treasuryAfter = new(big.Int).Set(bc.Accounts[constantset.LiquidityPoolAddress])
		bc.AccountsMu.RUnlock()
	}

	out := probeResult{
		TreasuryBefore: treasuryBefore.String(),
		TreasuryAfter:  treasuryAfter.String(),
		Delta:          new(big.Int).Sub(treasuryAfter, treasuryBefore).String(),
		ExpectedOut:    expOut.String(),
		ExpectedProfit: expProfit.String(),
		TimedOut:       timedOut,
		PairsBefore: map[string]string{
			"lqd_a": beforeLA["reserve0"] + "/" + beforeLA["reserve1"],
			"a_b":   beforeAB["reserve0"] + "/" + beforeAB["reserve1"],
			"b_lqd": beforeBL["reserve0"] + "/" + beforeBL["reserve1"],
		},
		PairsAfter: map[string]string{
			"lqd_a": afterLA["reserve0"] + "/" + afterLA["reserve1"],
			"a_b":   afterAB["reserve0"] + "/" + afterAB["reserve1"],
			"b_lqd": afterBL["reserve0"] + "/" + afterBL["reserve1"],
		},
	}

	buf, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(buf))
}
