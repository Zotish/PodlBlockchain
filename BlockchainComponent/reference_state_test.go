package blockchaincomponent

import (
	"math/big"
	"testing"
	"time"
)

func TestReferenceStateRootMatchesProductionCanonicalizer(t *testing.T) {
	bc := newTestBlockchain()
	bc.EnsureRuntimeState()
	bc.Accounts["0xBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"] = big.NewInt(22)
	bc.Accounts["0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"] = big.NewInt(11)
	bc.Validators = append(bc.Validators, &Validator{Address: "0xcccccccccccccccccccccccccccccccccccccccc", NativeBond: 1234, LiquidityPower: 4.5, PenaltyScore: 0.1, JailedUntil: time.Unix(99, 0)})
	bc.StrategyVaults["v2"] = &StrategyVaultPosition{ID: "v2", Owner: "0xBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB", CurrentPool: "0xdddddddddddddddddddddddddddddddddddddddd", TokenA: "LQD", TokenB: "USD", AmountA: big.NewInt(3), AmountB: big.NewInt(4), Shares: big.NewInt(5), Status: "active"}
	bc.StrategyVaultMoves = append(bc.StrategyVaultMoves, StrategyVaultMovement{ID: "move-1", VaultID: "v2", FromPool: "p1", ToPool: "p2", AmountA: big.NewInt(1), AmountB: big.NewInt(2), Shares: big.NewInt(3)})
	bc.LiquidityLocks["0xBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"] = []LockRecord{{Amount: big.NewInt(44), CreatedAt: time.Unix(10, 0), UnlockAt: time.Unix(20, 0)}}
	bc.TotalLiquidity = big.NewInt(44)
	bc.PendingFeePool["LQD"] = big.NewInt(6)
	bc.PoolLiquidity["pool-z"] = big.NewInt(7)
	bc.UnallocatedLiquidity = big.NewInt(8)
	bc.LiquidityProviders["0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"] = &LiquidityProvider{Address: "0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee", StakeAmount: big.NewInt(9), LiquidityPower: 2.5, LockTime: 10, LockDays: 90, PendingRewards: big.NewInt(1), TotalRewards: big.NewInt(2), UnstakeAmount: big.NewInt(3), ReleasedSoFar: big.NewInt(4), IsUnstaking: true, UnstakeStartTime: 11}
	bc.DynamicLiquidityOracleSignals["PAIR-Z"] = DynamicLiquidityOracleSignal{PairAddress: "PAIR-Z", DemandBps: 123, Source: "signed", UpdatedAt: 12}
	bc.PoolPriceHistory["PAIR-Z"] = []PoolPriceObservation{{PairAddress: "PAIR-Z", Price: 2, Timestamp: 12}, {PairAddress: "PAIR-Z", Price: 1, Timestamp: 11}}
	bc.EconomicBalances["z"] = big.NewInt(9)
	bc.EconomicBalances["a"] = big.NewInt(7)
	bc.ProtocolPauses["router"] = true
	bc.OracleObservations["LQD"] = map[string]OracleObservation{"z": {Asset: "LQD", Source: "z", PriceUSD: 1, Confidence: 1, Timestamp: 10}, "a": {Asset: "LQD", Source: "a", PriceUSD: 1, Confidence: 1, Timestamp: 10}}
	bc.OraclePublishers["z"] = "0x9999999999999999999999999999999999999999"
	bc.OracleNonces["z"] = 8
	bc.OracleNonces["retired-source"] = 7
	bc.BridgeRequests["bridge-1"] = &BridgeRequest{ID: "bridge-1", From: "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", To: "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Amount: "1", SourceChain: "a", TargetChain: "b", Status: BridgeStatusLocked}
	bc.BridgeTokenMap["chain|token"] = &BridgeTokenInfo{ChainID: "chain", SourceToken: "token", LqdToken: "lqd", Name: "T", Symbol: "T", Decimals: "18"}
	bc.BusinessAgreements["agreement-1"] = &LiquidityServiceAgreement{ID: "agreement-1", Client: "dao", CapitalLimit: big.NewInt(100), FeesCollected: big.NewInt(1), SettledInvoices: map[string]string{"i": "1"}}
	bc.TreasuryDeployments = append(bc.TreasuryDeployments, TreasuryDeployment{ID: "deployment-1", Target: "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Amount: big.NewInt(3), ProtocolOnly: true})
	for _, height := range []uint64{0, 1, 999} {
		production := bc.ComputeDeterministicStateRootAt(height)
		reference := bc.ComputeReferenceStateRootAt(height)
		if reference != production {
			t.Fatalf("reference root mismatch at %d: production=%s reference=%s", height, production, reference)
		}
	}
}

func TestReferenceRootDetectsEveryMutatedBalance(t *testing.T) {
	bc := newTestBlockchain()
	bc.EnsureRuntimeState()
	bc.Accounts["0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"] = big.NewInt(10)
	before := bc.ComputeReferenceStateRootAt(1)
	bc.Accounts["0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"] = big.NewInt(11)
	after := bc.ComputeReferenceStateRootAt(1)
	if before == after || after != bc.ComputeDeterministicStateRootAt(1) {
		t.Fatal("reference client failed to detect or reproduce a state mutation")
	}
}

func TestStateRootCommitsExtendedConsensusState(t *testing.T) {
	tests := map[string]func(*Blockchain_struct){
		"fee policy":        func(b *Blockchain_struct) { b.BaseFee++ },
		"reward policy":     func(b *Blockchain_struct) { b.FixedBlockReward++ },
		"legacy slash pool": func(b *Blockchain_struct) { b.SlashingPool++ },
		"liquidity locks": func(b *Blockchain_struct) {
			b.LiquidityLocks["0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"] = []LockRecord{{Amount: big.NewInt(1), CreatedAt: time.Unix(1, 0), UnlockAt: time.Unix(2, 0)}}
		},
		"pending fees":   func(b *Blockchain_struct) { b.PendingFeePool["lqd"] = big.NewInt(1) },
		"pool liquidity": func(b *Blockchain_struct) { b.PoolLiquidity["pool"] = big.NewInt(1) },
		"vault movement": func(b *Blockchain_struct) {
			b.StrategyVaultMoves = append(b.StrategyVaultMoves, StrategyVaultMovement{ID: "m", AmountA: big.NewInt(1), AmountB: big.NewInt(0), Shares: big.NewInt(1)})
		},
		"dynamic signal": func(b *Blockchain_struct) {
			b.DynamicLiquidityOracleSignals["pool"] = DynamicLiquidityOracleSignal{PairAddress: "pool", DemandBps: 1}
		},
		"price history": func(b *Blockchain_struct) {
			b.PoolPriceHistory["pool"] = []PoolPriceObservation{{PairAddress: "pool", Price: 1, Timestamp: 1}}
		},
		"orphan oracle nonce": func(b *Blockchain_struct) { b.OracleNonces["source"] = 1 },
		"bridge request": func(b *Blockchain_struct) {
			b.BridgeRequests["r"] = &BridgeRequest{ID: "r", Amount: "1", Status: BridgeStatusLocked}
		},
		"bridge token": func(b *Blockchain_struct) {
			b.BridgeTokenMap["c|t"] = &BridgeTokenInfo{ChainID: "c", SourceToken: "t", LqdToken: "lqd"}
		},
		"business agreement": func(b *Blockchain_struct) {
			b.BusinessAgreements["a"] = &LiquidityServiceAgreement{ID: "a", CapitalLimit: big.NewInt(1), FeesCollected: big.NewInt(0)}
		},
		"treasury deployment": func(b *Blockchain_struct) {
			b.TreasuryDeployments = append(b.TreasuryDeployments, TreasuryDeployment{ID: "d", Amount: big.NewInt(1)})
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			bc := newTestBlockchain()
			bc.EnsureRuntimeState()
			before := bc.ComputeDeterministicStateRootAt(1)
			mutate(bc)
			after := bc.ComputeDeterministicStateRootAt(1)
			if before == after || after != bc.ComputeReferenceStateRootAt(1) {
				t.Fatalf("extended state mutation was not independently committed: before=%s after=%s", before, after)
			}
		})
	}
}
