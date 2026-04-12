package blockchaincomponent

import (
	"fmt"
	"math"
	"math/big"
	"sync"
	"time"

	constantset "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/ConstantSet"
)

// LockLiquidity lets any address lock tokens that:
//   - are removed from its liquid balance
//   - contribute to TotalLiquidity (for LP rewards)
//   - are unlockable after lockDuration
func (bc *Blockchain_struct) LockLiquidity(address string, amount *big.Int, lockDuration time.Duration) error {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()

	if amount == nil || amount.Sign() == 0 {
		return fmt.Errorf("lock amount must be > 0")
	}
	if lockDuration <= 0 {
		return fmt.Errorf("lock duration must be > 0")
	}

	// Check balance (using your existing wallet balance logic).
	bal, err := bc.GetWalletBalance(address)
	if err != nil {
		return fmt.Errorf("cannot get balance for %s: %w", address, err)
	}
	if bal.Cmp(amount) < 0 {
		return fmt.Errorf("insufficient balance: have %s, need %s", AmountString(bal), AmountString(amount))
	}

	// Deduct from liquid balance and reflect in Accounts cache
	newBal := new(big.Int).Sub(bal, amount)
	bc.setAccountBalance(address, newBal)

	// Append a new lock record
	now := time.Now()
	lock := LockRecord{
		Amount:    CopyAmount(amount),
		UnlockAt:  now.Add(lockDuration),
		CreatedAt: now,
	}

	if bc.LiquidityLocks == nil {
		bc.LiquidityLocks = make(map[string][]LockRecord)
	}
	bc.LiquidityLocks[address] = append(bc.LiquidityLocks[address], lock)

	// Recompute total liquidity
	bc.recalculateTotalLiquidityLocked()

	return nil
}

// UnlockLiquidity releases all *matured* locks for an address.
// It credits the unlocked amount back to the address balance.
// Returns the total unlocked amount.
func (bc *Blockchain_struct) UnlockLiquidity(address string) (*big.Int, error) {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()

	if bc.LiquidityLocks == nil {
		return big.NewInt(0), nil
	}
	locks := bc.LiquidityLocks[address]
	if len(locks) == 0 {
		return big.NewInt(0), nil
	}

	now := time.Now()
	var remaining []LockRecord
	unlocked := big.NewInt(0)

	for _, l := range locks {
		if now.After(l.UnlockAt) || now.Equal(l.UnlockAt) {
			if l.Amount != nil {
				unlocked.Add(unlocked, l.Amount)
			}
		} else {
			remaining = append(remaining, l)
		}
	}

	if unlocked.Sign() == 0 {
		// Nothing yet unlocked; keep all as-is
		return big.NewInt(0), fmt.Errorf("no matured liquidity locks for address %s", address)
	}

	// Update locks & total liquidity
	bc.LiquidityLocks[address] = remaining
	if len(remaining) == 0 {
		delete(bc.LiquidityLocks, address)
	}

	if bc.TotalLiquidity == nil {
		bc.TotalLiquidity = big.NewInt(0)
	} else if bc.TotalLiquidity.Cmp(unlocked) <= 0 {
		bc.TotalLiquidity = big.NewInt(0)
	} else {
		bc.TotalLiquidity.Sub(bc.TotalLiquidity, unlocked)
	}

	// Credit back to balance cache
	currentBal, _ := bc.getAccountBalance(address)
	if currentBal == nil {
		currentBal = big.NewInt(0)
	}
	newBal := new(big.Int).Add(currentBal, unlocked)
	bc.setAccountBalance(address, newBal)

	return unlocked, nil
}

// recalculateTotalLiquidityLocked recomputes TotalLiquidity from all LockRecords.
func (bc *Blockchain_struct) recalculateTotalLiquidityLocked() {
	sum := big.NewInt(0)
	for _, locks := range bc.LiquidityLocks {
		for _, l := range locks {
			if l.Amount != nil {
				sum.Add(sum, l.Amount)
			}
		}
	}
	bc.TotalLiquidity = sum
}

// getLock returns the total locked amount (all non-expired locks) for an address.
func (bc *Blockchain_struct) getLock(address string) *big.Int {
	if bc.LiquidityLocks == nil {
		return big.NewInt(0)
	}
	locks := bc.LiquidityLocks[address]
	if len(locks) == 0 {
		return big.NewInt(0)
	}
	now := time.Now()
	total := big.NewInt(0)
	for _, l := range locks {
		// Only count still-locked positions; already-mature locks should be
		// handled by UnlockLiquidity.
		if now.Before(l.UnlockAt) {
			if l.Amount != nil {
				total.Add(total, l.Amount)
			}
		}
	}
	return total
}
func (bc *Blockchain_struct) UnlockAvailable(address string) (*big.Int, error) {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()

	recs := bc.LiquidityLocks[address]
	if len(recs) == 0 {
		return big.NewInt(0), nil
	}

	now := time.Now()
	var kept []LockRecord
	released := big.NewInt(0)

	for _, r := range recs {
		if now.After(r.UnlockAt) {
			if r.Amount != nil {
				released.Add(released, r.Amount)
			}
		} else {
			kept = append(kept, r)
		}
	}
	if released.Sign() > 0 {
		bc.addAccountBalance(address, released)
		if bc.TotalLiquidity == nil {
			bc.TotalLiquidity = big.NewInt(0)
		} else if bc.TotalLiquidity.Cmp(released) <= 0 {
			bc.TotalLiquidity = big.NewInt(0)
		} else {
			bc.TotalLiquidity.Sub(bc.TotalLiquidity, released)
		}
	}
	bc.LiquidityLocks[address] = kept

	snap := *bc
	snap.Mutex = sync.Mutex{}
	if err := PutIntoDB(snap); err != nil {
		return big.NewInt(0), err
	}
	return released, nil
}

// CalculateRewardForLiquidity distributes the LP slice of the reward pool:
//
//   - LP slice = totalRewards * Liquidity_provider / 100
//   - Each address gets proportional share based on its locked amount.
//
// This is called from blocks.go with totalRewards equal to the *whole* pool;
// here we only take the LP share from it.
func (bc *Blockchain_struct) CalculateRewardForLiquidity(totalRewards uint64) map[string]uint64 {
	lpRewards := (totalRewards * uint64(constantset.Liquidity_provider)) / 100
	out := make(map[string]uint64)
	if lpRewards == 0 {
		return out
	}
	if bc.TotalLiquidity == nil || bc.TotalLiquidity.Sign() == 0 {
		return out
	}
	if bc.LiquidityLocks == nil {
		return out
	}

	for addr := range bc.LiquidityLocks {
		locked := bc.getLock(addr)
		if locked == nil || locked.Sign() == 0 {
			continue
		}
		share := uint64(float64(lpRewards) * (AmountToFloat64(locked) / AmountToFloat64(bc.TotalLiquidity)))
		if share > 0 {
			out[addr] = share
		}
	}
	return out
}

// CalculateRewardForValidator distributes the validator slice:
//
//   - Validator slice = totalRewards * ValidatorReward / 100
//   - Weighted by LiquidityPower × (1 − PenaltyScore)
//   - Result is a map validatorAddress → rewardAmount
func (bc *Blockchain_struct) CalculateRewardForValidator(totalRewards uint64) map[string]uint64 {
	valRewards := (totalRewards * uint64(constantset.ValidatorReward)) / 100
	if valRewards == 0 || len(bc.Validators) == 0 {
		return map[string]uint64{}
	}

	// Compute weights
	var sum float64
	weights := make([]float64, len(bc.Validators))
	for i, v := range bc.Validators {
		w := v.LiquidityPower * (1.0 - v.PenaltyScore)
		if w < 0 {
			w = 0
		}
		weights[i] = w
		sum += w
	}

	out := make(map[string]uint64)
	if sum == 0 {
		return out
	}

	for i, v := range bc.Validators {
		portion := uint64(float64(valRewards) * (weights[i] / sum))
		if portion > 0 {
			out[v.Address] = portion
		}
	}
	return out
}

// ══════════════════════════════════════════════════════════════════════════════
// EMISSION SCHEDULE
// ══════════════════════════════════════════════════════════════════════════════
//
//  Block time   : 2 seconds
//  Halving every: 4 years  = 63,115,200 blocks  (4 × 365.25 × 24 × 1800)
//  Genesis base : 10 LQD/block  (= 1_000_000_000 satoshis)
//  50 % cut each halving epoch → converges toward 1 B LQD max supply
//
//  Year  0– 4  : 10.000000000 LQD/block
//  Year  4– 8  :  5.000000000 LQD/block
//  Year  8–12  :  2.500000000 LQD/block
//  Year 12–16  :  1.250000000 LQD/block
//  … (reaches 1 B supply ~year 27)

const (
	BlocksPerHalving  = uint64(63_115_200)   // 4 years at 2 s/block
	GenesisRewardSats = uint64(2_000_000_000) // 20 LQD in satoshis
)

// EmissionReward returns the block reward in satoshis for a given block number.
// Each halving epoch halves the reward (right-shift by 1).
// Returns 1 satoshi as the absolute floor so miners are always incentivised.
func EmissionReward(blockNumber uint64) *big.Int {
	halvings := blockNumber / BlocksPerHalving
	if halvings >= 64 {
		return big.NewInt(1) // floor: 1 satoshi
	}
	reward := GenesisRewardSats >> halvings
	if reward == 0 {
		reward = 1
	}
	return new(big.Int).SetUint64(reward)
}

// ══════════════════════════════════════════════════════════════════════════════
// BLOCK REWARD DISTRIBUTION
// ══════════════════════════════════════════════════════════════════════════════
//
//  Total pool = EmissionReward(block) + gas fees
//
//  ┌─────────────────────────────────────────────────────┐
//  │  40 %  Proposer Validator  (block winner)           │
//  │  30 %  LP Providers        (sqrt-curve weighted)    │
//  │   5 %  Long-lock LP only   (365–2000 days)          │
//  │  12 %  Other Validators    (non-winner, LP-weighted)│
//  │   2 %  TX Participants     (sqrt(value) weighted)   │
//  │  11 %  Treasury            (5 % LP + 6 % Validator) │
//  └─────────────────────────────────────────────────────┘

func (bc *Blockchain_struct) CalculateBlockRewards(
	validator string,
	txs []*Transaction,
	gasFees uint64,
	blockNumber uint64,
) BlockRewardBreakdown {

	treasury := constantset.LiquidityPoolAddress

	breakdown := BlockRewardBreakdown{
		Validator:            validator,
		ValidatorReward:      "0",
		ValidatorRewards:     make(map[string]string),
		ValidatorPartRewards: make(map[string]string),
		LiquidityRewards:     make(map[string]string),
		ParticipantRewards:   make(map[string]string),
	}

	// ── 1. Total pool = emission + gas ────────────────────────────────────────
	emission := EmissionReward(blockNumber)
	gasReward := new(big.Int).SetUint64(gasFees)
	total := new(big.Int).Add(emission, gasReward)

	// ── 2. Slice out each category ────────────────────────────────────────────
	proposerShare    := pctAmount(total, 40) // 40 % → block winner
	lpCurveShare     := pctAmount(total, 30) // 30 % → all LPs  (was 35%, -5% treasury)
	lpLongLockShare  := pctAmount(total, 5)  //  5 % → long-lock LPs
	otherValShare    := pctAmount(total, 12) // 12 % → other validators (was 18%, -6% treasury)
	txPartShare      := pctAmount(total, 2)  //  2 % → TX senders
	treasuryShare    := pctAmount(total, 11) // 11 % → treasury (5+6)

	// ── 3. Proposer (40 %) ────────────────────────────────────────────────────
	if proposerShare.Sign() > 0 {
		breakdown.ValidatorReward = AmountString(proposerShare)
		breakdown.ValidatorRewards[validator] = AmountString(proposerShare)
		bc.addAccountBalance(validator, CopyAmount(proposerShare))
	}

	// ── 4. LP Providers — 30 % (sqrt-curve weighted) ─────────────────────────
	totalLPWeight := 0.0
	for _, lp := range bc.LiquidityProviders {
		if lp.StakeAmount != nil && lp.StakeAmount.Sign() > 0 {
			totalLPWeight += math.Sqrt(AmountToFloat64(lp.StakeAmount))
		}
	}
	if totalLPWeight > 0 && lpCurveShare.Sign() > 0 {
		for _, lp := range bc.LiquidityProviders {
			if lp.StakeAmount == nil || lp.StakeAmount.Sign() == 0 {
				continue
			}
			share := portionFromWeight(lpCurveShare, math.Sqrt(AmountToFloat64(lp.StakeAmount)), totalLPWeight)
			addStringAmount(breakdown.LiquidityRewards, lp.Address, share)
			bc.AddLPReward(lp.Address, share)
		}
	}

	// ── 5. Long-lock LP — 5 % (365–2000 days only, sqrt-curve) ───────────────
	totalLongWeight := 0.0
	for _, lp := range bc.LiquidityProviders {
		if lp.StakeAmount != nil && lp.StakeAmount.Sign() > 0 &&
			lp.LockDays >= 365 && lp.LockDays <= 2000 {
			totalLongWeight += math.Sqrt(AmountToFloat64(lp.StakeAmount))
		}
	}
	if totalLongWeight > 0 && lpLongLockShare.Sign() > 0 {
		for _, lp := range bc.LiquidityProviders {
			if lp.StakeAmount == nil || lp.StakeAmount.Sign() == 0 {
				continue
			}
			if lp.LockDays >= 365 && lp.LockDays <= 2000 {
				share := portionFromWeight(lpLongLockShare, math.Sqrt(AmountToFloat64(lp.StakeAmount)), totalLongWeight)
				addStringAmount(breakdown.LiquidityRewards, lp.Address, share)
				bc.AddLPReward(lp.Address, share)
			}
		}
	}

	// ── 6. Other Validators — 12 % (non-winner, LP-weighted) ─────────────────
	var otherValWeightSum float64
	for _, v := range bc.Validators {
		if v.Address != validator && v.LPStakeAmount > 0 {
			otherValWeightSum += math.Sqrt(v.LPStakeAmount)
		}
	}
	if otherValWeightSum > 0 && otherValShare.Sign() > 0 {
		for _, v := range bc.Validators {
			if v.Address == validator || v.LPStakeAmount <= 0 {
				continue
			}
			portion := portionFromWeight(otherValShare, math.Sqrt(v.LPStakeAmount), otherValWeightSum)
			addStringAmount(breakdown.ValidatorPartRewards, v.Address, portion)
			bc.addAccountBalance(v.Address, portion)
		}
	}

	// ── 7. TX Participants — 2 % (sqrt(value) weighted) ──────────────────────
	if len(txs) > 0 && txPartShare.Sign() > 0 {
		totalTxWeight := 0.0
		for _, tx := range txs {
			totalTxWeight += math.Sqrt(AmountToFloat64(tx.Value) + 1)
		}
		if totalTxWeight > 0 {
			for _, tx := range txs {
				portion := portionFromWeight(txPartShare, math.Sqrt(AmountToFloat64(tx.Value)+1), totalTxWeight)
				addStringAmount(breakdown.ParticipantRewards, tx.TxHash, portion)
				bc.AddParticipantReward(tx.From, portion)
			}
		}
	}

	// ── 8. Treasury — 11 % (5 % LP redirect + 6 % validator redirect) ────────
	if treasuryShare.Sign() > 0 {
		bc.addAccountBalance(treasury, CopyAmount(treasuryShare))
	}

	return breakdown
}

func pctAmount(amount *big.Int, pct int64) *big.Int {
	if amount == nil || amount.Sign() == 0 {
		return big.NewInt(0)
	}
	out := new(big.Int).Mul(amount, big.NewInt(pct))
	out.Div(out, big.NewInt(100))
	return out
}

func portionFromWeight(pool *big.Int, weight, total float64) *big.Int {
	if pool == nil || pool.Sign() == 0 || total <= 0 || weight <= 0 {
		return big.NewInt(0)
	}
	f := new(big.Float).SetInt(pool)
	f.Mul(f, big.NewFloat(weight/total))
	out, _ := f.Int(nil)
	return out
}

func addStringAmount(dst map[string]string, key string, amt *big.Int) {
	if amt == nil || amt.Sign() == 0 {
		return
	}
	if existing, ok := dst[key]; ok && existing != "" {
		ex, err := NewAmountFromString(existing)
		if err == nil && ex != nil {
			ex.Add(ex, amt)
			dst[key] = AmountString(ex)
			return
		}
	}
	dst[key] = AmountString(amt)
}
