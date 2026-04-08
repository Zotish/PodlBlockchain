package blockchaincomponent

import (
	"fmt"
	"log"
	"math/big"
	"sort"
	"strings"
	"sync"
	"time"

	constantset "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/ConstantSet"
)

const (
	MaxBlockGas             = 8000000
	InactivityThreshold     = 60 * time.Minute
	DoubleSigningPenalty    = 0.2
	PerformancePenaltyScale = 0.05
	MinPerformanceThreshold = 0.5
)

type Validator struct {
	Address string `json:"address"`

	// ── True Proof of Dynamic Liquidity ──────────────────────────────────────
	// When DEXAddress is set the validator's power is derived from their locked
	// LP position in that DEX pool (multi-asset liquidity), not from a single-
	// asset stake.  This is the canonical PosDL mode.
	DEXAddress    string `json:"dex_address,omitempty"`
	LPTokenAmount string `json:"lp_token_amount,omitempty"` // decimal big-int string

	// ── Legacy PoS (used when DEXAddress == "") ───────────────────────────────
	LPStakeAmount float64 `json:"lp_stake_amount"`

	// ── Common ───────────────────────────────────────────────────────────────
	LockTime       time.Time `json:"lock_time"`
	LiquidityPower float64   `json:"liquidity_power"`
	PenaltyScore   float64   `json:"penalty_score"`
	BlocksProposed int       `json:"blocks_proposed"`
	BlocksIncluded int       `json:"blocks_included"`
	LastActive     time.Time `json:"last_active"`
}

func (bc *Blockchain_struct) AddNewValidators(address string, amount float64, lockDuration time.Duration) error {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()

	// Check if validator already exists
	for _, v := range bc.Validators {
		if v.Address == address {
			// Allow restart with existing validator without failing.
			log.Printf("Validator %s already exists; continuing with existing entry", address)
			return nil
		}
	}

	newVal := new(Validator)
	lp := amount * (lockDuration.Hours() / 8760)

	if amount < bc.MinStake {
		return fmt.Errorf("staking amount is lower than min stake %f", bc.MinStake)
	}

	newVal.Address = address
	newVal.LPStakeAmount = amount
	newVal.LockTime = time.Now().Add(lockDuration)
	newVal.LiquidityPower = lp
	newVal.LastActive = time.Now()
	bc.Validators = append(bc.Validators, newVal)

	// Broadcast new validator to network
	if bc.Network != nil {
		go bc.Network.BroadcastValidator(newVal)
	}

	// Save to database
	dbCopy := *bc
	dbCopy.Mutex = sync.Mutex{}
	if err := PutIntoDB(dbCopy); err != nil {
		return fmt.Errorf("error while adding new validator: %v", err)
	}

	log.Printf("Successfully added validator: %s with stake: %f", address, amount)
	return nil
}

// AddDEXValidator registers a validator using a DEX LP position — the True PosDL mode.
// The validator must already hold LP tokens in the DEX pool at dexAddress.
// lpTokenAmount is the amount of LP tokens (decimal string) to lock for validation.
func (bc *Blockchain_struct) AddDEXValidator(address, dexAddress, lpTokenAmount string, lockDuration time.Duration) error {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()

	for _, v := range bc.Validators {
		if v.Address == address {
			log.Printf("PosDL validator %s already registered; continuing", address)
			return nil
		}
	}
	if dexAddress == "" {
		return fmt.Errorf("dex_address is required for PosDL validator registration")
	}
	if lpTokenAmount == "" || lpTokenAmount == "0" {
		return fmt.Errorf("lp_token_amount must be > 0")
	}

	newVal := &Validator{
		Address:       address,
		DEXAddress:    strings.ToLower(dexAddress),
		LPTokenAmount: lpTokenAmount,
		LockTime:      time.Now().Add(lockDuration),
		LastActive:    time.Now(),
	}
	newVal.LiquidityPower = bc.getDEXLPPower(newVal)

	bc.Validators = append(bc.Validators, newVal)
	if bc.Network != nil {
		go bc.Network.BroadcastValidator(newVal)
	}

	dbCopy := *bc
	dbCopy.Mutex = sync.Mutex{}
	if err := PutIntoDB(dbCopy); err != nil {
		return fmt.Errorf("error saving PosDL validator: %v", err)
	}
	log.Printf("PosDL validator registered: %s DEX=%s lpAmount=%s", address, dexAddress, lpTokenAmount)
	return nil
}

// UpdateLiquidityPower refreshes every validator's LiquidityPower.
// PosDL validators query the DEX contract storage; legacy validators use
// time-weighted single-asset stake (backward-compatible).
func (bc *Blockchain_struct) UpdateLiquidityPower() {
	for _, v := range bc.Validators {
		if v.DEXAddress != "" {
			// True PosDL: power comes from DEX LP position value
			v.LiquidityPower = bc.getDEXLPPower(v)
		} else {
			// Legacy PoS: time-weighted single-asset stake
			remainingLock := time.Until(v.LockTime).Hours()
			if remainingLock < 0 {
				remainingLock = 0
			}
			v.LiquidityPower = v.LPStakeAmount * (remainingLock / 8760)
		}
	}
}

// getDEXLPPower reads the validator's locked LP position directly from contract
// storage and computes:
//
//	power = (lockedLP / totalLP) × (reserveA + reserveB) × lockMultiplier
//
// where lockMultiplier = 1 + remainingLockYears, so longer locks earn more power.
func (bc *Blockchain_struct) getDEXLPPower(v *Validator) float64 {
	if bc.ContractEngine == nil || v.DEXAddress == "" {
		return 0
	}

	storage, err := bc.ContractEngine.DB.LoadAllStorage(strings.ToLower(v.DEXAddress))
	if err != nil || len(storage) == 0 {
		return 0
	}

	// Prefer the on-chain locked amount; fall back to the registered amount
	// (useful during bootstrapping before the contract tx is mined).
	valKey := "val_lp:" + strings.ToLower(v.Address)
	lockedLPStr, ok := storage[valKey]
	if !ok || lockedLPStr == "" || lockedLPStr == "0" {
		lockedLPStr = v.LPTokenAmount
	}

	lockedLP := parseBigToFloat(lockedLPStr)
	totalLP := parseBigToFloat(storage["totalLP"])
	resA := parseBigToFloat(storage["reserveA"])
	resB := parseBigToFloat(storage["reserveB"])

	if totalLP <= 0 || lockedLP <= 0 {
		return 0
	}

	// LP backing value
	lpValue := lockedLP * (resA + resB) / totalLP

	// Lock time multiplier: base 1.0 + remaining fraction of a year
	remaining := time.Until(v.LockTime).Hours()
	if remaining < 0 {
		remaining = 0
	}
	lockMultiplier := 1.0 + (remaining / 8760.0)

	return lpValue * lockMultiplier
}

// parseBigToFloat converts a decimal integer string (e.g. big.Int.String()) to float64.
func parseBigToFloat(v string) float64 {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	z := new(big.Int)
	if _, ok := z.SetString(v, 10); !ok {
		return 0
	}
	f, _ := new(big.Float).SetInt(z).Float64()
	return f
}

func (bc *Blockchain_struct) MonitorValidators() {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()

	minActiveTime := 5 * time.Minute
	currentTime := time.Now()

	// Build block map for double signing check
	blockMap := make(map[string]string) // block hash -> validator address

	for _, block := range bc.Blocks {
		if existing, exists := blockMap[block.CurrentHash]; exists {
			// Double signing detected!
			bc.SlashValidator(existing, DoubleSigningPenalty, "double signing")
			log.Printf("Double signing detected by validator %s for block %s", existing, block.CurrentHash)
		}
		blockMap[block.CurrentHash] = block.CurrentHash[:42] // Assuming first 42 chars contain validator address
	}

	for _, v := range bc.Validators {
		// Skip newly added validators
		if currentTime.Sub(v.LastActive) < minActiveTime {
			continue
		}

		// Check for inactivity
		if currentTime.Sub(v.LastActive) > InactivityThreshold {
			bc.SlashValidator(v.Address, 0.05, "inactivity")
			log.Printf("Validator %s slashed for inactivity", v.Address)
			continue
		}

		// Check performance if validator has proposed blocks
		if v.BlocksProposed > 0 {
			successRate := float64(v.BlocksIncluded) / float64(v.BlocksProposed)
			if successRate < MinPerformanceThreshold {
				penalty := PerformancePenaltyScale * (1 - successRate)
				bc.SlashValidator(v.Address, penalty, fmt.Sprintf("poor performance (%.2f%%)", successRate*100))
				log.Printf("Validator %s slashed for poor performance (%.2f%%)", v.Address, successRate*100)
			}
		}

		// Check stake lock time
		if currentTime.After(v.LockTime) {
			bc.SlashValidator(v.Address, 0.1, "stake lock expired")
			log.Printf("Validator %s slashed for expired stake lock", v.Address)
		}

		// Check for sequential missed blocks
		if v.BlocksProposed > 10 {
			recentMissRate := float64(v.BlocksProposed-v.BlocksIncluded) / float64(v.BlocksProposed)
			if recentMissRate > 0.5 {
				bc.SlashValidator(v.Address, 0.15, "high miss rate")
				log.Printf("Validator %s slashed for high miss rate (%.2f%%)", v.Address, recentMissRate*100)
			}
		}
	}
}

func (bc *Blockchain_struct) SlashValidator(add string, penalty float64, reason string) {
	for i := 0; i < len(bc.Validators); i++ {
		v := bc.Validators[i]
		if v.Address == add {
			// Calculate penalty based on severity and history
			effectivePenalty := penalty * (1 + v.PenaltyScore)

			// Cap penalty to prevent complete slashing from single offense
			if effectivePenalty > 0.3 {
				effectivePenalty = 0.3
			}

			LocalPenalty := v.LPStakeAmount * effectivePenalty
			bc.SlashingPool += LocalPenalty
			bc.Validators[i].LPStakeAmount -= LocalPenalty

			// Increase penalty score for future offenses
			bc.Validators[i].PenaltyScore += 0.1

			// Log the slashing event
			log.Printf("Validator %s slashed: %f tokens (reason: %s)", add, LocalPenalty, reason)

			if bc.Validators[i].LPStakeAmount < bc.MinStake {
				bc.Validators = append(bc.Validators[:i], bc.Validators[i+1:]...)
				i--
				log.Printf("Validator %s removed due to insufficient stake", add)
			}
			return
		}
	}
}

func (bc *Blockchain_struct) UpdateMinStake(networkLoad float64) {
	bc.MinStake = 1000000 * float64(constantset.Decimals) * (1 + networkLoad/10)
}

func (bc *Blockchain_struct) SelectValidator() (Validator, error) {
	if len(bc.Validators) == 0 {
		return Validator{}, fmt.Errorf("no validator for selection")
	}

	bc.UpdateLiquidityPower()
	type weightedValidator struct {
		v      *Validator
		weight float64
	}

	eligible := make([]weightedValidator, 0, len(bc.Validators))
	for _, v := range bc.Validators {
		weight := v.LiquidityPower * (1.0 - v.PenaltyScore)
		if weight < 0 {
			weight = 0
		}
		if weight == 0 {
			continue
		}
		eligible = append(eligible, weightedValidator{v: v, weight: weight})
	}

	if len(eligible) == 0 {
		return Validator{}, fmt.Errorf("no validators with positive weight")
	}

	sort.Slice(eligible, func(i, j int) bool {
		if eligible[i].weight == eligible[j].weight {
			return eligible[i].v.Address < eligible[j].v.Address
		}
		return eligible[i].weight > eligible[j].weight
	})

	selected := eligible[0].v
	selected.BlocksProposed++
	selected.LastActive = time.Now()
	return *selected, nil
}
