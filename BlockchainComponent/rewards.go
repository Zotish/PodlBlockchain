// package blockchaincomponent

// import (
// 	"fmt"
// 	"sync"
// 	"time"

// 	constantset "github.com/Zotish/DefenceProject/ConstantSet"
// )

// // LockLiquidity lets any address lock capital that contributes to LiquidityPower (via validators) and earns LP rewards.
// func (bc *Blockchain_struct) LockLiquidity(address string, amount uint64, lockDuration time.Duration) error {
// 	bc.Mutex.Lock()
// 	defer bc.Mutex.Unlock()

// 	// Require balance
// 	if bc.Accounts[address] < amount {
// 		return fmt.Errorf("insufficient balance")
// 	}

// 	bc.Accounts[address] -= amount
// 	rec := LockRecord{
// 		Amount:    amount,
// 		CreatedAt: time.Now(),
// 		UnlockAt:  time.Now().Add(lockDuration),
// 	}
// 	bc.LiquidityLocks[address] = append(bc.LiquidityLocks[address], rec)
// 	bc.TotalLiquidity += amount

// 	// Persist
// 	snap := *bc
// 	snap.Mutex = sync.Mutex{}
// 	return PutIntoDB(snap)
// }

// // UnlockAvailable moves any matured locks back to the account.

// // getLock returns currently locked (not yet matured) capital for address.
// func (bc *Blockchain_struct) getLock(address string) uint64 {
// 	var sum uint64
// 	now := time.Now()
// 	for _, r := range bc.LiquidityLocks[address] {
// 		if now.Before(r.UnlockAt) {
// 			sum += r.Amount
// 		}
// 	}
// 	return sum
// }

// // CalculateRewardForLiquidity distributes the LP slice by proportional locked amounts.
// func (bc *Blockchain_struct) CalculateRewardForLiquidity(totalRewards uint64) map[string]uint64 {
// 	lpRewards := (totalRewards * uint64(constantset.Liquidity_provider)) / 100
// 	if lpRewards == 0 || bc.TotalLiquidity == 0 {
// 		return map[string]uint64{}
// 	}
// 	out := make(map[string]uint64)
// 	for addr := range bc.LiquidityLocks {
// 		locked := bc.getLock(addr)
// 		if locked == 0 {
// 			continue
// 		}
// 		out[addr] = (lpRewards * locked) / bc.TotalLiquidity
// 	}
// 	return out
// }

// // CalculateRewardForValidator distributes the validator slice by (LiquidityPower × (1 - PenaltyScore)).
// func (bc *Blockchain_struct) CalculateRewardForValidator(totalRewards uint64) map[string]uint64 {
// 	valRewards := (totalRewards * uint64(constantset.ValidatorReward)) / 100
// 	if valRewards == 0 || len(bc.Validators) == 0 {
// 		return map[string]uint64{}
// 	}
// 	// Compute weights
// 	var sum float64
// 	weights := make([]float64, len(bc.Validators))
// 	for i, v := range bc.Validators {
// 		w := v.LiquidityPower * (1.0 - v.PenaltyScore)
// 		if w < 0 {
// 			w = 0
// 		}
// 		weights[i] = w
// 		sum += w
// 	}
// 	out := make(map[string]uint64)
// 	if sum == 0 {
// 		return out
// 	}
// 	for i, v := range bc.Validators {
// 		out[v.Address] = uint64(float64(valRewards) * (weights[i] / sum))
// 	}
// 	return out
// }

// rewards.go
package blockchaincomponent

import (
	"fmt"
	"sync"
	"time"

	constantset "github.com/Zotish/DefenceProject/ConstantSet"
)

// LockLiquidity lets any address lock tokens that:
//   - are removed from its liquid balance
//   - contribute to TotalLiquidity (for LP rewards)
//   - are unlockable after lockDuration
func (bc *Blockchain_struct) LockLiquidity(address string, amount uint64, lockDuration time.Duration) error {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()

	if amount == 0 {
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
	if bal < amount {
		return fmt.Errorf("insufficient balance: have %d, need %d", bal, amount)
	}

	// Deduct from liquid balance and reflect in Accounts cache
	bc.Accounts[address] = bal - amount

	// Append a new lock record
	now := time.Now()
	lock := LockRecord{
		Amount:    amount,
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
func (bc *Blockchain_struct) UnlockLiquidity(address string) (uint64, error) {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()

	if bc.LiquidityLocks == nil {
		return 0, nil
	}
	locks := bc.LiquidityLocks[address]
	if len(locks) == 0 {
		return 0, nil
	}

	now := time.Now()
	var remaining []LockRecord
	var unlocked uint64

	for _, l := range locks {
		if now.After(l.UnlockAt) || now.Equal(l.UnlockAt) {
			unlocked += l.Amount
		} else {
			remaining = append(remaining, l)
		}
	}

	if unlocked == 0 {
		// Nothing yet unlocked; keep all as-is
		return 0, fmt.Errorf("no matured liquidity locks for address %s", address)
	}

	// Update locks & total liquidity
	bc.LiquidityLocks[address] = remaining
	if len(remaining) == 0 {
		delete(bc.LiquidityLocks, address)
	}

	if unlocked > bc.TotalLiquidity {
		bc.TotalLiquidity = 0
	} else {
		bc.TotalLiquidity -= unlocked
	}

	// Credit back to balance cache
	currentBal := bc.Accounts[address]
	bc.Accounts[address] = currentBal + unlocked

	return unlocked, nil
}

// recalculateTotalLiquidityLocked recomputes TotalLiquidity from all LockRecords.
func (bc *Blockchain_struct) recalculateTotalLiquidityLocked() {
	var sum uint64
	for _, locks := range bc.LiquidityLocks {
		for _, l := range locks {
			sum += l.Amount
		}
	}
	bc.TotalLiquidity = sum
}

// getLock returns the total locked amount (all non-expired locks) for an address.
func (bc *Blockchain_struct) getLock(address string) uint64 {
	if bc.LiquidityLocks == nil {
		return 0
	}
	locks := bc.LiquidityLocks[address]
	if len(locks) == 0 {
		return 0
	}
	now := time.Now()
	var total uint64
	for _, l := range locks {
		// Only count still-locked positions; already-mature locks should be
		// handled by UnlockLiquidity.
		if now.Before(l.UnlockAt) {
			total += l.Amount
		}
	}
	return total
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
	if bc.TotalLiquidity == 0 {
		return out
	}
	if bc.LiquidityLocks == nil {
		return out
	}

	for addr := range bc.LiquidityLocks {
		locked := bc.getLock(addr)
		if locked == 0 {
			continue
		}
		share := (lpRewards * locked) / bc.TotalLiquidity
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

func (bc *Blockchain_struct) UnlockAvailable(address string) (uint64, error) {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()

	recs := bc.LiquidityLocks[address]
	if len(recs) == 0 {
		return 0, nil
	}

	now := time.Now()
	var kept []LockRecord
	var released uint64

	for _, r := range recs {
		if now.After(r.UnlockAt) {
			released += r.Amount
		} else {
			kept = append(kept, r)
		}
	}
	if released > 0 {
		bc.Accounts[address] += released
		bc.TotalLiquidity -= released
	}
	bc.LiquidityLocks[address] = kept

	snap := *bc
	snap.Mutex = sync.Mutex{}
	if err := PutIntoDB(snap); err != nil {
		return 0, err
	}
	return released, nil
}
