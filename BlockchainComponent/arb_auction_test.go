package blockchaincomponent

import (
	"math/big"
	"sync"
	"testing"
	"time"

	constantset "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/ConstantSet"
)

func TestArbMarketLocksOverlapButIndependentPoolsProceed(t *testing.T) {
	bc := newTestBlockchain()
	poolA := PoolMetrics{PairAddress: "0x1000000000000000000000000000000000000001"}
	poolB := PoolMetrics{PairAddress: "0x2000000000000000000000000000000000000002"}
	releaseA := bc.lockArbMarkets([]PoolMetrics{poolA})
	overlapAcquired := make(chan struct{})
	go func() {
		release := bc.lockArbMarkets([]PoolMetrics{poolA, poolB})
		close(overlapAcquired)
		release()
	}()
	select {
	case <-overlapAcquired:
		t.Fatal("overlapping market lock did not block")
	case <-time.After(20 * time.Millisecond):
	}
	independentAcquired := make(chan struct{})
	go func() {
		release := bc.lockArbMarkets([]PoolMetrics{poolB})
		close(independentAcquired)
		release()
	}()
	select {
	case <-independentAcquired:
	case <-time.After(time.Second):
		t.Fatal("independent market was unnecessarily serialized")
	}
	releaseA()
	select {
	case <-overlapAcquired:
	case <-time.After(time.Second):
		t.Fatal("overlapping market did not resume after release")
	}
}

func TestArbAuctionBindsWinnerSnapshotAndExecution(t *testing.T) {
	previousPersist := persistRuntimeState
	persistRuntimeState = func(*Blockchain_struct) error { return nil }
	defer func() { persistRuntimeState = previousPersist }()
	db, err := InitContractDBAtPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	bc := newTestBlockchain()
	bc.ContractEngine = &LQDContractEngine{DB: db}
	bc.ArbPolicy = DefaultProtocolArbPolicy()
	bc.ArbPolicy.Enabled = true
	bc.Accounts[normalizeAccountAddress(constantset.LiquidityPoolAddress)] = big.NewInt(10_000_000)
	metrics := []PoolMetrics{
		{PairAddress: "0x1000000000000000000000000000000000000001", Token0: "lqd", Token1: "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Reserve0: big.NewInt(1_000_000), Reserve1: big.NewInt(1_000_000)},
		{PairAddress: "0x2000000000000000000000000000000000000002", Token0: "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Token1: "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Reserve0: big.NewInt(1_000_000), Reserve1: big.NewInt(1_000_000)},
		{PairAddress: "0x3000000000000000000000000000000000000003", Token0: "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Token1: "lqd", Reserve0: big.NewInt(1_000_000), Reserve1: big.NewInt(2_000_000)},
	}
	for _, m := range metrics {
		if err := db.SaveStorage(m.PairAddress, "reserve0", m.Reserve0.String()); err != nil {
			t.Fatal(err)
		}
		if err := db.SaveStorage(m.PairAddress, "reserve1", m.Reserve1.String()); err != nil {
			t.Fatal(err)
		}
	}
	auction, err := bc.OpenArbAuction(ArbOpportunityHash(metrics), 1, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	keeperA := "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa1"
	keeperB := "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb2"
	bc.Accounts[normalizeAccountAddress(keeperA)] = big.NewInt(1_000_000)
	bc.Accounts[normalizeAccountAddress(keeperB)] = big.NewInt(1_000_000)
	if err := bc.BondArbKeeper(keeperA, big.NewInt(100_000)); err != nil {
		t.Fatal(err)
	}
	if err := bc.BondArbKeeper(keeperB, big.NewInt(100_000)); err != nil {
		t.Fatal(err)
	}
	if err := bc.CommitArbBid(auction.ID, keeperA, arbBidCommitment(auction.ID, keeperA, 120, "a"), 2); err != nil {
		t.Fatal(err)
	}
	if err := bc.CommitArbBid(auction.ID, keeperB, arbBidCommitment(auction.ID, keeperB, 50, "b"), 2); err != nil {
		t.Fatal(err)
	}
	if err := bc.RevealArbBid(auction.ID, keeperA, 120, "a", 4); err != nil {
		t.Fatal(err)
	}
	if err := bc.RevealArbBid(auction.ID, keeperB, 50, "b", 4); err != nil {
		t.Fatal(err)
	}
	finalized, err := bc.FinalizeArbAuction(auction.ID, 6)
	if err != nil || !stringsEqualFold(finalized.Winner, keeperB) {
		t.Fatalf("lowest valid bid did not win: %#v err=%v", finalized, err)
	}
	tampered := append([]PoolMetrics(nil), metrics...)
	tampered[0].Reserve0 = big.NewInt(999_999)
	if _, err := bc.ExecuteArbAuction(auction.ID, keeperB, 6, tampered); err == nil {
		t.Fatal("tampered opportunity snapshot executed")
	}
	type executionResult struct {
		auction *ArbAuction
		err     error
	}
	results := make(chan executionResult, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			executed, executeErr := bc.ExecuteArbAuction(auction.ID, keeperB, 6, metrics)
			results <- executionResult{auction: executed, err: executeErr}
		}()
	}
	wg.Wait()
	close(results)
	successes := 0
	for result := range results {
		if result.err == nil {
			successes++
			if result.auction == nil || !result.auction.Executed || result.auction.RealizedProfit == "" {
				t.Fatalf("successful execution did not record settlement: %#v", result.auction)
			}
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent settlement succeeded %d times; want exactly one", successes)
	}
	if len(bc.ProtocolRevenue) != 1 || bc.ProtocolRevenue[0].Source != "protocol_arb" || bc.AccountBalanceAmount(ProtocolRevenueEscrowAddress).Sign() <= 0 {
		t.Fatalf("arb profit was not custody-backed and reconciled: revenue=%#v escrow=%s", bc.ProtocolRevenue, bc.AccountBalanceAmount(ProtocolRevenueEscrowAddress))
	}
	if _, err := bc.ExecuteArbAuction(auction.ID, keeperB, 7, metrics); err == nil {
		t.Fatal("auction replay executed twice")
	}
}

func stringsEqualFold(a, b string) bool {
	return normalizeAccountAddress(a) == normalizeAccountAddress(b)
}

func TestArbAuctionMissedWinnerSlashedAndFallbackPromoted(t *testing.T) {
	previousPersist := persistRuntimeState
	persistRuntimeState = func(*Blockchain_struct) error { return nil }
	defer func() { persistRuntimeState = previousPersist }()
	bc := newTestBlockchain()
	bc.EnsureRuntimeState()
	bc.ArbPolicy.ExecutionTimeout = 2
	keeperA, keeperB := "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa1", "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb2"
	bc.Accounts[normalizeAccountAddress(keeperA)], bc.Accounts[normalizeAccountAddress(keeperB)] = big.NewInt(1_000_000), big.NewInt(1_000_000)
	if err := bc.BondArbKeeper(keeperA, big.NewInt(100_000)); err != nil {
		t.Fatal(err)
	}
	if err := bc.BondArbKeeper(keeperB, big.NewInt(100_000)); err != nil {
		t.Fatal(err)
	}
	auction, err := bc.OpenArbAuction("opportunity", 1, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err = bc.CommitArbBid(auction.ID, keeperA, arbBidCommitment(auction.ID, keeperA, 20, "a"), 2); err != nil {
		t.Fatal(err)
	}
	if err = bc.CommitArbBid(auction.ID, keeperB, arbBidCommitment(auction.ID, keeperB, 10, "b"), 2); err != nil {
		t.Fatal(err)
	}
	if err = bc.RevealArbBid(auction.ID, keeperA, 20, "a", 3); err != nil {
		t.Fatal(err)
	}
	if err = bc.RevealArbBid(auction.ID, keeperB, 10, "b", 3); err != nil {
		t.Fatal(err)
	}
	if _, err = bc.FinalizeArbAuction(auction.ID, 4); err != nil {
		t.Fatal(err)
	}
	fallback, err := bc.ExpireArbAuction(auction.ID, 7)
	if err != nil {
		t.Fatal(err)
	}
	if fallback.Winner != normalizeAccountAddress(keeperA) || bc.ArbKeeperBonds[normalizeAccountAddress(keeperB)].Cmp(big.NewInt(90_000)) != 0 {
		t.Fatalf("fallback/slash failed: auction=%+v bonds=%v", fallback, bc.ArbKeeperBonds)
	}
	if bc.economicBalance(EconomicInsurance).Cmp(big.NewInt(10_000)) != 0 || bc.economicBalance(EconomicLPYield).Sign() != 0 {
		t.Fatalf("arb missed-duty slash was not insurance-first: %+v", bc.EconomicBalances)
	}
}
