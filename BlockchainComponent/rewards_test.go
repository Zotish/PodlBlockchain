package blockchaincomponent

import (
	"math/big"
	"testing"
	"time"

	constantset "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/ConstantSet"
)

// ─────────────────────────────────────────────────────────────────────────────
// EmissionReward — halving schedule
// ─────────────────────────────────────────────────────────────────────────────

func TestEmissionReward_GenesisBlock(t *testing.T) {
	r := EmissionReward(0)
	expected := new(big.Int).SetUint64(GenesisRewardSats)
	if r.Cmp(expected) != 0 {
		t.Errorf("genesis reward: expected %s, got %s", expected.String(), r.String())
	}
}

func TestEmissionReward_FirstHalving(t *testing.T) {
	// At block BlocksPerHalving, reward should be halved exactly once
	r := EmissionReward(BlocksPerHalving)
	expected := new(big.Int).SetUint64(GenesisRewardSats >> 1)
	if r.Cmp(expected) != 0 {
		t.Errorf("first halving: expected %s, got %s", expected.String(), r.String())
	}
}

func TestEmissionReward_SecondHalving(t *testing.T) {
	r := EmissionReward(2 * BlocksPerHalving)
	expected := new(big.Int).SetUint64(GenesisRewardSats >> 2)
	if r.Cmp(expected) != 0 {
		t.Errorf("second halving: expected %s, got %s", expected.String(), r.String())
	}
}

func TestEmissionReward_HalvingMonotoneDecreasing(t *testing.T) {
	prev := EmissionReward(0)
	for epoch := uint64(1); epoch <= 10; epoch++ {
		curr := EmissionReward(epoch * BlocksPerHalving)
		if curr.Cmp(prev) >= 0 {
			t.Errorf("reward should decrease after each halving (epoch %d): prev=%s curr=%s",
				epoch, prev.String(), curr.String())
		}
		prev = curr
	}
}

func TestEmissionReward_FloorIsOnesat(t *testing.T) {
	// After 64+ halvings, the reward should be 1 satoshi
	r := EmissionReward(64 * BlocksPerHalving)
	if r.Cmp(big.NewInt(1)) != 0 {
		t.Errorf("expected floor reward of 1 satoshi after 64 halvings, got %s", r.String())
	}
}

func TestEmissionReward_NeverZero(t *testing.T) {
	// Test many epochs including well beyond 64 halvings
	testBlocks := []uint64{
		0, BlocksPerHalving, 10 * BlocksPerHalving,
		63 * BlocksPerHalving, 100 * BlocksPerHalving,
		1000 * BlocksPerHalving,
	}
	for _, b := range testBlocks {
		r := EmissionReward(b)
		if r.Sign() <= 0 {
			t.Errorf("reward should never be zero or negative at block %d, got %s", b, r.String())
		}
	}
}

func TestEmissionReward_BeforeFirstHalving(t *testing.T) {
	// Block just before the first halving should still return full genesis reward
	r := EmissionReward(BlocksPerHalving - 1)
	expected := new(big.Int).SetUint64(GenesisRewardSats)
	if r.Cmp(expected) != 0 {
		t.Errorf("block before first halving: expected %s, got %s", expected.String(), r.String())
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// CalculateBlockRewards — 6-way split
// ─────────────────────────────────────────────────────────────────────────────

func makeRewardTestBC() *Blockchain_struct {
	bc := newTestBlockchain()
	// Add some validators with positive power
	v1 := makeValidator("0x1111111111111111111111111111111111111111", 1e12, 365)
	v2 := makeValidator("0x2222222222222222222222222222222222222222", 1e11, 180)
	bc.Validators = []*Validator{v1, v2}
	bc.UpdateLiquidityPower()

	// Add LP providers (used by CalculateBlockRewards for LP slice)
	lpAddr := "0x3333333333333333333333333333333333333333"
	bc.LiquidityProviders = map[string]*LiquidityProvider{
		lpAddr: {
			Address:     lpAddr,
			StakeAmount: big.NewInt(1e12),
			LockDays:    400, // qualifies for long-lock bonus
		},
	}

	// Also add to liquidity locks (used by CalculateRewardForLiquidity)
	bc.LiquidityLocks[lpAddr] = []LockRecord{
		{
			Amount:   big.NewInt(1e12),
			UnlockAt: time.Now().Add(24 * time.Hour),
		},
	}
	bc.recalculateTotalLiquidityLocked()
	return bc
}

func TestCalculateBlockRewards_ValidatorSet(t *testing.T) {
	bc := makeRewardTestBC()
	breakdown := bc.CalculateBlockRewards("0x1111111111111111111111111111111111111111",
		[]*Transaction{}, 0, 1)

	if breakdown.Validator == "" {
		t.Error("breakdown should have a validator address")
	}
	if breakdown.ValidatorReward == "" || breakdown.ValidatorReward == "0" {
		t.Error("proposer validator reward should be non-zero")
	}
}

func TestCalculateBlockRewards_EmissionPlusFees(t *testing.T) {
	bc := makeRewardTestBC()
	// Use block 0 (genesis reward) + 1000 gas fees
	breakdown := bc.CalculateBlockRewards("0x1111111111111111111111111111111111111111",
		[]*Transaction{}, 1000, 0)

	// proposer reward should be 40% of (GenesisRewardSats + 1000)
	total := new(big.Int).Add(EmissionReward(0), big.NewInt(1000))
	proposer := pctAmount(total, 40)
	if NewAmountFromStringOrZero(breakdown.ValidatorReward).Cmp(proposer) != 0 {
		t.Errorf("proposer should get 40%% of total pool: expected %s, got %s",
			proposer.String(), breakdown.ValidatorReward)
	}
}

func TestCalculateBlockRewards_TreasuryGetsShare(t *testing.T) {
	bc := makeRewardTestBC()
	bc.setAccountBalance(constantset.LiquidityPoolAddress, big.NewInt(0))

	bc.CalculateBlockRewards("0x1111111111111111111111111111111111111111",
		[]*Transaction{}, 10000, 1)

	// Treasury address should have received something
	treasuryBal := bc.CheckBalance(constantset.LiquidityPoolAddress)
	if treasuryBal.Sign() <= 0 {
		t.Error("treasury (LiquidityPoolAddress) should receive a share of block rewards")
	}
}

func TestCalculateBlockRewards_LPRewardsNonEmpty(t *testing.T) {
	bc := makeRewardTestBC()
	breakdown := bc.CalculateBlockRewards("0x1111111111111111111111111111111111111111",
		[]*Transaction{}, 10000, 1)

	if len(breakdown.LiquidityRewards) == 0 {
		t.Error("expected LP rewards to be distributed when there are locked LP providers")
	}
}

func TestCalculateBlockRewards_TxParticipants(t *testing.T) {
	bc := makeRewardTestBC()
	txSender := "0x4444444444444444444444444444444444444444"
	txs := []*Transaction{
		{
			From:   txSender,
			To:     "0x5555555555555555555555555555555555555555",
			Value:  big.NewInt(1000),
			Status: constantset.StatusSuccess,
		},
	}
	breakdown := bc.CalculateBlockRewards("0x1111111111111111111111111111111111111111",
		txs, 5000, 1)

	if len(breakdown.ParticipantRewards) == 0 {
		t.Error("expected participant rewards to be distributed to TX senders")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// CalculateRewardForLiquidity
// ─────────────────────────────────────────────────────────────────────────────

func TestCalculateRewardForLiquidity_EmptyLocks(t *testing.T) {
	bc := newTestBlockchain()
	out := bc.CalculateRewardForLiquidity(1000000)
	if len(out) != 0 {
		t.Errorf("expected no LP rewards with no locks, got %d entries", len(out))
	}
}

func TestCalculateRewardForLiquidity_ProportionalDistribution(t *testing.T) {
	bc := newTestBlockchain()
	addr1 := "0x1111111111111111111111111111111111111111"
	addr2 := "0x2222222222222222222222222222222222222222"

	// addr1 locks 2x as much as addr2 → should get ~2x the LP reward
	bc.LiquidityLocks[addr1] = []LockRecord{{Amount: big.NewInt(2000), UnlockAt: time.Now().Add(time.Hour)}}
	bc.LiquidityLocks[addr2] = []LockRecord{{Amount: big.NewInt(1000), UnlockAt: time.Now().Add(time.Hour)}}
	bc.recalculateTotalLiquidityLocked()

	out := bc.CalculateRewardForLiquidity(1000000)
	r1 := out[addr1]
	r2 := out[addr2]

	if r1 == 0 || r2 == 0 {
		t.Errorf("both addresses should receive LP rewards: addr1=%d addr2=%d", r1, r2)
	}
	// addr1 should get ~2x addr2 (within 1 satoshi rounding)
	if r1 < r2 {
		t.Errorf("addr1 (2x lock) should get more reward than addr2 (1x): %d vs %d", r1, r2)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// CalculateRewardForValidator
// ─────────────────────────────────────────────────────────────────────────────

func TestCalculateRewardForValidator_EmptyValidators(t *testing.T) {
	bc := newTestBlockchain()
	out := bc.CalculateRewardForValidator(1000000)
	if len(out) != 0 {
		t.Errorf("expected no validator rewards with no validators, got %d", len(out))
	}
}

func TestCalculateRewardForValidator_DistributedProportionally(t *testing.T) {
	v1 := makeValidator("0x1111111111111111111111111111111111111111", 1e12, 365)
	v2 := makeValidator("0x2222222222222222222222222222222222222222", 1e12, 365)
	bc := newBCWithValidators([]*Validator{v1, v2})
	bc.UpdateLiquidityPower()

	out := bc.CalculateRewardForValidator(1000000)
	if len(out) == 0 {
		t.Error("expected validator rewards to be distributed")
	}
	for addr, reward := range out {
		if reward == 0 {
			t.Errorf("validator %q should receive non-zero reward", addr)
		}
	}
}

func TestCalculateRewardForValidator_HigherPowerGetsMore(t *testing.T) {
	v1 := makeValidator("0x1111111111111111111111111111111111111111", 1e10, 30)  // low power
	v2 := makeValidator("0x2222222222222222222222222222222222222222", 1e14, 365) // high power
	bc := newBCWithValidators([]*Validator{v1, v2})
	bc.UpdateLiquidityPower()

	out := bc.CalculateRewardForValidator(1000000)
	r1 := out[v1.Address]
	r2 := out[v2.Address]
	if r2 <= r1 {
		t.Errorf("higher-power validator should get more reward: low=%d high=%d", r1, r2)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// LockLiquidity / UnlockLiquidity
// ─────────────────────────────────────────────────────────────────────────────

func TestLockLiquidity_ReducesBalance(t *testing.T) {
	bc := newTestBlockchain()
	addr := "0x1111111111111111111111111111111111111111"
	bc.setAccountBalance(addr, big.NewInt(10000))

	err := bc.LockLiquidity(addr, big.NewInt(5000), time.Hour)
	if err != nil {
		t.Fatalf("LockLiquidity failed: %v", err)
	}
	bal := bc.CheckBalance(addr)
	if bal.Cmp(big.NewInt(5000)) != 0 {
		t.Errorf("expected balance 5000 after lock, got %s", bal.String())
	}
}

func TestLockLiquidity_UpdatesTotalLiquidity(t *testing.T) {
	bc := newTestBlockchain()
	addr := "0x1111111111111111111111111111111111111111"
	bc.setAccountBalance(addr, big.NewInt(10000))

	_ = bc.LockLiquidity(addr, big.NewInt(3000), time.Hour)
	if bc.TotalLiquidity.Cmp(big.NewInt(3000)) != 0 {
		t.Errorf("expected TotalLiquidity 3000, got %s", bc.TotalLiquidity.String())
	}
}

func TestLockLiquidity_InsufficientBalance_Error(t *testing.T) {
	bc := newTestBlockchain()
	addr := "0x1111111111111111111111111111111111111111"
	bc.setAccountBalance(addr, big.NewInt(100))

	err := bc.LockLiquidity(addr, big.NewInt(9999), time.Hour)
	if err == nil {
		t.Error("expected error for insufficient balance")
	}
}

func TestLockLiquidity_ZeroAmount_Error(t *testing.T) {
	bc := newTestBlockchain()
	addr := "0x1111111111111111111111111111111111111111"
	bc.setAccountBalance(addr, big.NewInt(1000))

	err := bc.LockLiquidity(addr, big.NewInt(0), time.Hour)
	if err == nil {
		t.Error("expected error for zero lock amount")
	}
}

func TestLockLiquidity_ZeroDuration_Error(t *testing.T) {
	bc := newTestBlockchain()
	addr := "0x1111111111111111111111111111111111111111"
	bc.setAccountBalance(addr, big.NewInt(1000))

	err := bc.LockLiquidity(addr, big.NewInt(100), 0)
	if err == nil {
		t.Error("expected error for zero lock duration")
	}
}

func TestGetLock_ReturnsLockedAmount(t *testing.T) {
	bc := newTestBlockchain()
	addr := "0x1111111111111111111111111111111111111111"
	bc.setAccountBalance(addr, big.NewInt(10000))
	_ = bc.LockLiquidity(addr, big.NewInt(4000), time.Hour)

	locked := bc.GetLock(addr)
	if locked.Cmp(big.NewInt(4000)) != 0 {
		t.Errorf("expected locked amount 4000, got %s", locked.String())
	}
}

func TestUnlockLiquidity_MatureLock(t *testing.T) {
	bc := newTestBlockchain()
	addr := "0x1111111111111111111111111111111111111111"
	bc.setAccountBalance(addr, big.NewInt(10000))

	// Add a lock that's already matured
	bc.LiquidityLocks[addr] = []LockRecord{
		{
			Amount:   big.NewInt(2000),
			UnlockAt: time.Now().Add(-1 * time.Second), // already expired
		},
	}
	bc.recalculateTotalLiquidityLocked()

	unlocked, err := bc.UnlockLiquidity(addr)
	if err != nil {
		t.Fatalf("UnlockLiquidity error: %v", err)
	}
	if unlocked.Cmp(big.NewInt(2000)) != 0 {
		t.Errorf("expected unlocked 2000, got %s", unlocked.String())
	}
}

func TestUnlockLiquidity_ActiveLock_NoUnlock(t *testing.T) {
	bc := newTestBlockchain()
	addr := "0x1111111111111111111111111111111111111111"
	bc.setAccountBalance(addr, big.NewInt(10000))
	_ = bc.LockLiquidity(addr, big.NewInt(2000), 24*time.Hour)

	_, err := bc.UnlockLiquidity(addr)
	if err == nil {
		t.Error("should return error when no matured locks exist")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// pctAmount helper (private, tested indirectly)
// ─────────────────────────────────────────────────────────────────────────────

func TestCalculateBlockRewards_ProposerIs40Pct(t *testing.T) {
	bc := newTestBlockchain()
	v := makeValidator("0x1111111111111111111111111111111111111111", 1e12, 365)
	bc.Validators = []*Validator{v}
	bc.UpdateLiquidityPower()
	bc.setAccountBalance(constantset.LiquidityPoolAddress, big.NewInt(0))

	gasFees := uint64(0)
	blockNum := uint64(0)
	breakdown := bc.CalculateBlockRewards(v.Address, []*Transaction{}, gasFees, blockNum)

	totalPool := new(big.Int).Add(EmissionReward(blockNum), new(big.Int).SetUint64(gasFees))
	expected40pct := pctAmount(totalPool, 40)

	if NewAmountFromStringOrZero(breakdown.ValidatorReward).Cmp(expected40pct) != 0 {
		t.Errorf("proposer should receive exactly 40%% of total pool: expected %s, got %s",
			expected40pct.String(), breakdown.ValidatorReward)
	}
}
