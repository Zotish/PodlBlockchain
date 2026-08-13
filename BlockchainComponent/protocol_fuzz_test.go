package blockchaincomponent

import (
	"math/big"
	"strings"
	"testing"
)

func FuzzAMMOutputPreservesConstantProduct(f *testing.F) {
	f.Add(uint64(1_000_000), uint64(2_000_000), uint64(10_000))
	f.Add(uint64(100), uint64(100), uint64(1))
	f.Fuzz(func(t *testing.T, x, y, input uint64) {
		if x == 0 || y == 0 || input == 0 {
			return
		}
		rin, rout, amt := new(big.Int).SetUint64(x), new(big.Int).SetUint64(y), new(big.Int).SetUint64(input)
		out := arbAmountOut(amt, rin, rout)
		if out.Sign() < 0 || out.Cmp(rout) >= 0 {
			t.Fatalf("invalid output %s", out)
		}
		before := new(big.Int).Mul(rin, rout)
		after := new(big.Int).Mul(new(big.Int).Add(rin, amt), new(big.Int).Sub(rout, out))
		if after.Cmp(before) < 0 {
			t.Fatalf("constant product decreased")
		}
	})
}

func FuzzBridgeReplaySnapshotRejectsTamper(f *testing.F) {
	f.Add("event-a")
	f.Fuzz(func(t *testing.T, key string) {
		if key == "" {
			return
		}
		bc := &Blockchain_struct{}
		bc.EnsureRuntimeState()
		bc.BridgeSecurity.Consumed[key] = true
		snapshot := bc.ExportBridgeReplaySnapshot()
		snapshot.Consumed[key] = false
		if bc.RestoreBridgeReplaySnapshot(snapshot) == nil {
			t.Fatal("tampered snapshot accepted")
		}
	})
}

func FuzzConcentratedSwapAtomicAndNonNegative(f *testing.F) {
	f.Add(uint64(1_000_000), uint64(10_000), true)
	f.Add(uint64(5_000_000), uint64(50_000), false)
	f.Fuzz(func(t *testing.T, rawLiquidity, rawInput uint64, zeroForOne bool) {
		liquidity := rawLiquidity%1_000_000_000 + 10_000
		amount := rawInput%100_000_000 + 1
		pool := &ConcentratedPool{SqrtPriceX18: new(big.Int).Set(ammScaleX18), Liquidity: big.NewInt(0), FeeBPS: 30}
		lower := new(big.Int).Div(new(big.Int).Set(ammScaleX18), big.NewInt(2))
		upper := new(big.Int).Mul(new(big.Int).Set(ammScaleX18), big.NewInt(2))
		if err := pool.AddPosition("position", "owner", lower, upper, new(big.Int).SetUint64(liquidity)); err != nil {
			t.Fatal(err)
		}
		before := pool.deepCopy()
		out, err := pool.Swap(new(big.Int).SetUint64(amount), zeroForOne)
		if err != nil {
			if pool.SqrtPriceX18.Cmp(before.SqrtPriceX18) != 0 || pool.Liquidity.Cmp(before.Liquidity) != 0 || pool.Positions["position"].TokensOwed0.Sign() != 0 || pool.Positions["position"].TokensOwed1.Sign() != 0 {
				t.Fatal("failed swap partially mutated concentrated pool")
			}
			return
		}
		if out.Sign() <= 0 || pool.SqrtPriceX18.Sign() <= 0 || pool.Liquidity.Sign() < 0 {
			t.Fatalf("invalid successful concentrated swap: out=%s price=%s liquidity=%s", out, pool.SqrtPriceX18, pool.Liquidity)
		}
		position := pool.Positions["position"]
		if position == nil || position.Liquidity.Sign() <= 0 || position.TokensOwed0.Sign() < 0 || position.TokensOwed1.Sign() < 0 {
			t.Fatal("position accounting invariant broken")
		}
	})
}

func FuzzHardenedVMCompileAndExecuteIsBounded(f *testing.F) {
	f.Add("SET balance 10; ADD balance 5; GET balance")
	f.Add("JMP 0")
	f.Add("UNKNOWN anything")
	f.Fuzz(func(t *testing.T, source string) {
		if len(source) > 16*1024 {
			return
		}
		vm := NewInterpreterVM()
		bytecode, err := vm.CompileGoSubset(source)
		if err != nil {
			return
		}
		if len(bytecode.Instructions) < 1 || len(bytecode.Instructions) > 1024 || len(bytecode.Ops) != len(bytecode.Instructions) {
			t.Fatalf("compiler escaped bytecode bounds: %+v", bytecode)
		}
		db := NewOverlayContractDB(nil)
		ctx := NewContext("0x1111111111111111111111111111111111111111", "0x2222222222222222222222222222222222222222", "0x2222222222222222222222222222222222222222", "0x2222222222222222222222222222222222222222", db, 10_000)
		// REVERT and out-of-gas are expected program outcomes. The property is
		// that arbitrary source cannot escape validation, metering or panic with
		// a runtime-internal nil dereference.
		panicReason := catchRevert(func() { _, _ = vm.ExecuteBytecode(ctx.ContractAddr, bytecode, ctx) })
		if panicReason != "" && !strings.HasPrefix(panicReason, "REVERT:") {
			t.Fatalf("hardened VM leaked an internal panic: %s", panicReason)
		}
	})
}
