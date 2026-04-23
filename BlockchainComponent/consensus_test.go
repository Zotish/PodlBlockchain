package blockchaincomponent

import (
	"math/big"
	"testing"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// helpers — build a blockchain with pre-loaded legacy PoS validators
// (no DB interaction)
// ─────────────────────────────────────────────────────────────────────────────

func newBCWithValidators(vs []*Validator) *Blockchain_struct {
	bc := newTestBlockchain()
	bc.Validators = vs
	return bc
}

func makeValidator(addr string, stake float64, lockDays int) *Validator {
	return &Validator{
		Address:      addr,
		LPStakeAmount: stake,
		LockTime:     time.Now().Add(time.Duration(lockDays) * 24 * time.Hour),
		LastActive:   time.Now(),
		PenaltyScore: 0,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// SelectValidator
// ─────────────────────────────────────────────────────────────────────────────

func TestSelectValidator_NoValidators_Error(t *testing.T) {
	bc := newBCWithValidators(nil)
	_, err := bc.SelectValidator()
	if err == nil {
		t.Error("expected error when no validators present")
	}
}

func TestSelectValidator_SingleValidator_Wins(t *testing.T) {
	v := makeValidator("0xAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", 1e12, 365)
	bc := newBCWithValidators([]*Validator{v})
	selected, err := bc.SelectValidator()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if selected.Address != v.Address {
		t.Errorf("expected %q, got %q", v.Address, selected.Address)
	}
}

func TestSelectValidator_HigherStakeWins(t *testing.T) {
	low := makeValidator("0x1111111111111111111111111111111111111111", 1e10, 365)
	high := makeValidator("0x2222222222222222222222222222222222222222", 1e14, 365)
	bc := newBCWithValidators([]*Validator{low, high})
	selected, err := bc.SelectValidator()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if selected.Address != high.Address {
		t.Errorf("expected high-stake validator %q to win, got %q", high.Address, selected.Address)
	}
}

func TestSelectValidator_PenaltyReducesWeight(t *testing.T) {
	// v1: large stake but extremely high penalty → very small effective weight
	// v2: moderate stake, no penalty → larger effective weight
	// With v1.stake=1e14, penalty=0.999, 365-day lock:
	//   LiquidityPower ≈ 1e14, weight = 1e14 * 0.001 = 1e11
	// With v2.stake=1e12, no penalty, 365-day lock:
	//   LiquidityPower ≈ 1e12, weight = 1e12 * 1.0 = 1e12  (10x more than v1)
	v1 := makeValidator("0x1111111111111111111111111111111111111111", 1e14, 365)
	v1.PenaltyScore = 0.999 // near-full penalty → effective weight ≈ 1e11
	v2 := makeValidator("0x2222222222222222222222222222222222222222", 1e12, 365)

	bc := newBCWithValidators([]*Validator{v1, v2})
	selected, err := bc.SelectValidator()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if selected.Address != v2.Address {
		t.Errorf("expected penalty-free validator %q to win, got %q", v2.Address, selected.Address)
	}
}

func TestSelectValidator_ExpiredLockHasZeroWeight(t *testing.T) {
	expired := &Validator{
		Address:      "0x1111111111111111111111111111111111111111",
		LPStakeAmount: 1e15,
		LockTime:     time.Now().Add(-24 * time.Hour), // already expired
		LastActive:   time.Now(),
	}
	active := makeValidator("0x2222222222222222222222222222222222222222", 1e10, 10)
	bc := newBCWithValidators([]*Validator{expired, active})
	selected, err := bc.SelectValidator()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Expired lock should have zero legacyLiquidityPower → active wins
	if selected.Address != active.Address {
		t.Errorf("expected active validator to win over expired-lock validator, got %q", selected.Address)
	}
}

func TestSelectValidator_IncreasesBlocksProposed(t *testing.T) {
	v := makeValidator("0x1111111111111111111111111111111111111111", 1e12, 365)
	bc := newBCWithValidators([]*Validator{v})
	before := v.BlocksProposed
	selected, err := bc.SelectValidator()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if selected.BlocksProposed != before+1 {
		t.Errorf("expected BlocksProposed to increase by 1 after selection")
	}
}

func TestSelectValidator_AllZeroWeight_Error(t *testing.T) {
	v := &Validator{
		Address:      "0x1111111111111111111111111111111111111111",
		LPStakeAmount: 0,
		LockTime:     time.Now().Add(-1 * time.Hour), // expired
	}
	bc := newBCWithValidators([]*Validator{v})
	_, err := bc.SelectValidator()
	if err == nil {
		t.Error("expected error when all validators have zero weight")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// SlashValidator
// ─────────────────────────────────────────────────────────────────────────────

func TestSlashValidator_ReducesStake(t *testing.T) {
	v := makeValidator("0x1111111111111111111111111111111111111111", 1e12, 365)
	bc := newBCWithValidators([]*Validator{v})
	bc.SlashValidator(v.Address, 0.1, "test slash")

	for _, val := range bc.Validators {
		if val.Address == v.Address {
			if val.LPStakeAmount >= 1e12 {
				t.Error("stake should decrease after slashing")
			}
			return
		}
	}
}

func TestSlashValidator_CapsAt30Percent(t *testing.T) {
	initialStake := 1e12
	v := makeValidator("0x1111111111111111111111111111111111111111", initialStake, 365)
	bc := newBCWithValidators([]*Validator{v})
	// Request 50% penalty — should be capped at 30%
	bc.SlashValidator(v.Address, 0.5, "heavy slash test")

	for _, val := range bc.Validators {
		if val.Address == v.Address {
			// effective slash ≤ 30% → remaining ≥ 70%
			if val.LPStakeAmount < initialStake*0.69 { // small float tolerance
				t.Errorf("slash exceeded 30%% cap: remaining stake %.0f", val.LPStakeAmount)
			}
			return
		}
	}
}

func TestSlashValidator_AccumulatesPenaltyScore(t *testing.T) {
	v := makeValidator("0x1111111111111111111111111111111111111111", 1e14, 365)
	v.PenaltyScore = 0.0
	bc := newBCWithValidators([]*Validator{v})
	bc.SlashValidator(v.Address, 0.05, "first offense")

	for _, val := range bc.Validators {
		if val.Address == v.Address {
			if val.PenaltyScore <= 0 {
				t.Error("penalty score should increase after slashing")
			}
			return
		}
	}
}

func TestSlashValidator_RemovesValidatorBelowMinStake(t *testing.T) {
	// Use stake just above MinStake so slashing drops it below
	v := makeValidator("0x1111111111111111111111111111111111111111", 100001*1e8, 365)
	bc := newBCWithValidators([]*Validator{v})
	bc.MinStake = 100000 * 1e8

	// Big enough slash to bring stake below min
	bc.SlashValidator(v.Address, 0.3, "remove test")
	// If still present check stake; if removed that's also correct
	for _, val := range bc.Validators {
		if val.Address == v.Address && val.LPStakeAmount < bc.MinStake {
			t.Error("validator with stake below MinStake should have been removed")
		}
	}
}

func TestSlashValidator_UnknownAddress_Noop(t *testing.T) {
	v := makeValidator("0x1111111111111111111111111111111111111111", 1e12, 365)
	bc := newBCWithValidators([]*Validator{v})
	before := len(bc.Validators)
	bc.SlashValidator("0x9999999999999999999999999999999999999999", 0.1, "unknown")
	if len(bc.Validators) != before {
		t.Error("slashing unknown address should be a no-op")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// UpdateLiquidityPower (legacy PoS only)
// ─────────────────────────────────────────────────────────────────────────────

func TestUpdateLiquidityPower_LegacyPositive(t *testing.T) {
	v := makeValidator("0x1111111111111111111111111111111111111111", 1e12, 180)
	bc := newBCWithValidators([]*Validator{v})
	bc.UpdateLiquidityPower()

	for _, val := range bc.Validators {
		if val.Address == v.Address {
			if val.LiquidityPower <= 0 {
				t.Errorf("expected positive LiquidityPower for active validator, got %f", val.LiquidityPower)
			}
			return
		}
	}
}

func TestUpdateLiquidityPower_ExpiredLock_ZeroOrLow(t *testing.T) {
	v := &Validator{
		Address:      "0x1111111111111111111111111111111111111111",
		LPStakeAmount: 1e12,
		LockTime:     time.Now().Add(-1 * time.Hour), // expired
	}
	bc := newBCWithValidators([]*Validator{v})
	bc.UpdateLiquidityPower()

	for _, val := range bc.Validators {
		if val.Address == v.Address {
			if val.LiquidityPower > 0 {
				t.Errorf("expired lock should have zero liquidity power, got %f", val.LiquidityPower)
			}
			return
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// legacyLiquidityPower (tested indirectly via UpdateLiquidityPower)
// ─────────────────────────────────────────────────────────────────────────────

func TestLegacyLiquidityPower_LongerLock_HigherPower(t *testing.T) {
	v1 := makeValidator("0x1111111111111111111111111111111111111111", 1e12, 30)
	v2 := makeValidator("0x2222222222222222222222222222222222222222", 1e12, 365)
	bc := newBCWithValidators([]*Validator{v1, v2})
	bc.UpdateLiquidityPower()

	var p1, p2 float64
	for _, v := range bc.Validators {
		if v.Address == v1.Address {
			p1 = v.LiquidityPower
		}
		if v.Address == v2.Address {
			p2 = v.LiquidityPower
		}
	}
	if p1 >= p2 {
		t.Errorf("longer lock should give higher power: 30-day=%.4f  365-day=%.4f", p1, p2)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// GetNetworkStats
// ─────────────────────────────────────────────────────────────────────────────

func TestGetNetworkStats_Fields(t *testing.T) {
	bc := newTestBlockchain()
	bc.AddAccountBalance("0x1111111111111111111111111111111111111111", big.NewInt(1000))

	stats := bc.GetNetworkStats()
	if _, ok := stats["block_height"]; !ok {
		t.Error("stats should contain block_height")
	}
	if _, ok := stats["validators"]; !ok {
		t.Error("stats should contain validators")
	}
	if _, ok := stats["transaction_pool"]; !ok {
		t.Error("stats should contain transaction_pool")
	}
}

func TestGetNetworkStats_BlockHeight(t *testing.T) {
	bc := newTestBlockchain()
	stats := bc.GetNetworkStats()
	height := stats["block_height"].(int)
	if height != 1 {
		t.Errorf("expected block_height 1 (genesis), got %d", height)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// GetValidatorStats
// ─────────────────────────────────────────────────────────────────────────────

func TestGetValidatorStats_ExistingValidator(t *testing.T) {
	v := makeValidator("0x1111111111111111111111111111111111111111", 1e12, 365)
	bc := newBCWithValidators([]*Validator{v})
	stats := bc.GetValidatorStats(v.Address)
	if stats == nil {
		t.Fatal("expected stats for existing validator")
	}
	if stats["address"] != v.Address {
		t.Errorf("wrong address in stats: %v", stats["address"])
	}
}

func TestGetValidatorStats_UnknownAddress_Nil(t *testing.T) {
	bc := newTestBlockchain()
	stats := bc.GetValidatorStats("0x9999999999999999999999999999999999999999")
	if stats != nil {
		t.Error("unknown validator address should return nil stats")
	}
}
