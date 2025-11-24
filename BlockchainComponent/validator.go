// package blockchaincomponent

// import (
// 	"crypto/rand"
// 	"fmt"
// 	"log"
// 	"math/big"
// 	"sync"

// 	"time"

// 	constantset "github.com/Zotish/DefenceProject/ConstantSet"
// )

// const (
// 	MaxBlockGas             = 8000000
// 	InactivityThreshold     = 60 * time.Minute
// 	DoubleSigningPenalty    = 0.2
// 	PerformancePenaltyScale = 0.05
// 	MinPerformanceThreshold = 0.5
// )

// type Validator struct {
// 	Address        string    `json:"address"`
// 	LPStakeAmount  float64   `json:"lp_stake_amount"`
// 	LockTime       time.Time `json:"lock_time"`
// 	LiquidityPower float64   `json:"liquidity_power"`
// 	PenaltyScore   float64   `json:"penalty_score"`
// 	BlocksProposed int       `json:"blocks_proposed"`
// 	BlocksIncluded int       `json:"blocks_included"`
// 	LastActive     time.Time `json:"last_active"`
// }

// func (bc *Blockchain_struct) AddNewValidators(address string, amount float64, lockDuration time.Duration) error {
// 	bc.Mutex.Lock()
// 	defer bc.Mutex.Unlock()

// 	// Check if validator already exists
// 	for _, v := range bc.Validators {
// 		if v.Address == address {
// 			return fmt.Errorf("validator %s already exists", address)
// 		}
// 	}

// 	newVal := new(Validator)
// 	lp := amount * (lockDuration.Hours() / 8760)

// 	if amount < bc.MinStake {
// 		return fmt.Errorf("staking amount is lower than min stake %f", bc.MinStake)
// 	}

// 	newVal.Address = address
// 	newVal.LPStakeAmount = amount
// 	newVal.LockTime = time.Now().Add(lockDuration)
// 	newVal.LiquidityPower = lp
// 	newVal.LastActive = time.Now()
// 	bc.Validators = append(bc.Validators, newVal)

// 	// Broadcast new validator to network
// 	if bc.Network != nil {
// 		go bc.Network.BroadcastValidator(newVal)
// 	}

// 	// Save to database
// 	dbCopy := *bc
// 	dbCopy.Mutex = sync.Mutex{}
// 	if err := PutIntoDB(dbCopy); err != nil {
// 		return fmt.Errorf("error while adding new validator: %v", err)
// 	}

// 	log.Printf("Successfully added validator: %s with stake: %f", address, amount)
// 	return nil
// }

// // func (bc *Blockchain_struct) AddNewValidators(address string, amount float64, lockDuration time.Duration) error {
// // 	bc.Mutex.Lock()
// // 	defer bc.Mutex.Unlock()

// // 	newVal := new(Validator)
// // 	lp := amount * (lockDuration.Hours() / 8760)

// // 	if amount < bc.MinStake {
// // 		return fmt.Errorf("staking amount is lower than min stake %f", bc.MinStake)
// // 	}

// // 	newVal.Address = address
// // 	newVal.LPStakeAmount = amount
// // 	newVal.LockTime = time.Now().Add(lockDuration)
// // 	newVal.LiquidityPower = lp
// // 	bc.Validators = append(bc.Validators, newVal)

// // 	//this is added for mutex issues
// // 	dbCopy := *bc
// // 	dbCopy.Mutex = sync.Mutex{}

// // 	err := PutIntoDB(dbCopy)
// // 	if err != nil {
// // 		return fmt.Errorf("error while adding new validator: %v", err)
// // 	}
// // 	return nil
// // }

// //this is the last one func (bc *Blockchain_struct) AddNewValidators(address string, amount float64, lockDuration time.Duration) error {
// // 	newVal := new(Validator)
// // 	lp := amount * (lockDuration.Hours() / 8760)

// // 	if amount < bc.MinStake { // Changed to correct comparison
// // 		return fmt.Errorf("staking amount is lower than min stake %f", bc.MinStake)
// // 	}

// // 	newVal.Address = address
// // 	newVal.LPStakeAmount = amount
// // 	newVal.LockTime = time.Now().Add(lockDuration)
// // 	newVal.LiquidityPower = lp
// // 	bc.Validators = append(bc.Validators, newVal)
// // 	err := PutIntoDB(*bc)
// // 	if err != nil {
// // 		return fmt.Errorf("error while adding new validator: %v", err)
// // 	}
// // 	return nil

// // }
// func (bc *Blockchain_struct) UpdateLiquidityPower() {
// 	for _, v := range bc.Validators {
// 		remainingLock := time.Until(v.LockTime).Hours()
// 		v.LiquidityPower = v.LPStakeAmount * (remainingLock / 8760)
// 	}
// }

// //this one is the last one func (bc *Blockchain_struct) SelectValidator() (Validator, error) {
// // 	if len(bc.Validators) == 0 {
// // 		return Validator{}, fmt.Errorf("no validator for selection")
// // 	}

// // 	bc.UpdateLiquidityPower()

// // 	// Calculate total weighted liquidity power
// // 	totalWeight := 0.0
// // 	validators := make([]Validator, len(bc.Validators))
// // 	weights := make([]float64, len(bc.Validators))

// // 	for i, v := range bc.Validators {
// // 		// Apply penalty score (higher penalty = lower chance)
// // 		weight := v.LiquidityPower * (1.0 - v.PenaltyScore)
// // 		if weight < 0 {
// // 			weight = 0
// // 		}
// // 		validators[i] = *v
// // 		weights[i] = weight
// // 		totalWeight += weight
// // 	}

// // 	if totalWeight <= 0 {
// // 		return Validator{}, fmt.Errorf("no validators with positive weight")
// // 	}

// // 	// Use crypto/rand for better randomness
// // 	randValue := rand.Int63n(1 << 62)
// // 	r := float64(randValue) / float64(1<<62) * totalWeight

// // 	// Select validator based on weighted probability
// // 	cumulative := 0.0
// // 	for i, weight := range weights {
// // 		cumulative += weight
// // 		if r <= cumulative {
// // 			// Update validator stats
// // 			bc.Validators[i].BlocksProposed++
// // 			bc.Validators[i].LastActive = time.Now()
// // 			return validators[i], nil
// // 		}
// // 	}

// // 	return Validator{}, fmt.Errorf("selection failed")
// // }

// func (bc *Blockchain_struct) MonitorValidators() {
// 	bc.Mutex.Lock()
// 	defer bc.Mutex.Unlock()

// 	minActiveTime := 5 * time.Minute
// 	currentTime := time.Now()

// 	// Build block map for double signing check
// 	blockMap := make(map[string]string) // block hash -> validator address

// 	for _, block := range bc.Blocks {
// 		if existing, exists := blockMap[block.CurrentHash]; exists {
// 			// Double signing detected!
// 			bc.SlashValidator(existing, DoubleSigningPenalty, "double signing")
// 			log.Printf("Double signing detected by validator %s for block %s", existing, block.CurrentHash)
// 		}
// 		blockMap[block.CurrentHash] = block.CurrentHash[:42] // Assuming first 42 chars contain validator address
// 	}

// 	for _, v := range bc.Validators {
// 		// Skip newly added validators
// 		if currentTime.Sub(v.LastActive) < minActiveTime {
// 			continue
// 		}

// 		// Check for inactivity
// 		if currentTime.Sub(v.LastActive) > InactivityThreshold {
// 			bc.SlashValidator(v.Address, 0.05, "inactivity")
// 			log.Printf("Validator %s slashed for inactivity", v.Address)
// 			continue
// 		}

// 		// Check performance if validator has proposed blocks
// 		if v.BlocksProposed > 0 {
// 			successRate := float64(v.BlocksIncluded) / float64(v.BlocksProposed)
// 			if successRate < MinPerformanceThreshold {
// 				penalty := PerformancePenaltyScale * (1 - successRate)
// 				bc.SlashValidator(v.Address, penalty, fmt.Sprintf("poor performance (%.2f%%)", successRate*100))
// 				log.Printf("Validator %s slashed for poor performance (%.2f%%)", v.Address, successRate*100)
// 			}
// 		}

// 		// Check stake lock time
// 		if currentTime.After(v.LockTime) {
// 			bc.SlashValidator(v.Address, 0.1, "stake lock expired")
// 			log.Printf("Validator %s slashed for expired stake lock", v.Address)
// 		}

// 		// Check for sequential missed blocks
// 		if v.BlocksProposed > 10 {
// 			recentMissRate := float64(v.BlocksProposed-v.BlocksIncluded) / float64(v.BlocksProposed)
// 			if recentMissRate > 0.5 {
// 				bc.SlashValidator(v.Address, 0.15, "high miss rate")
// 				log.Printf("Validator %s slashed for high miss rate (%.2f%%)", v.Address, recentMissRate*100)
// 			}
// 		}
// 	}
// }

// //this is the 2nd last func (bc *Blockchain_struct) MonitorValidators() {
// // 	minActiveTime := 5 * time.Minute

// // 	for _, v := range bc.Validators {
// // 		// Skip newly added validators
// // 		if time.Since(v.LastActive) < minActiveTime {
// // 			continue
// // 		}

// // 		// Check for double signing (compare proposed blocks)
// // 		proposedBlocks := make(map[string]bool)
// // 		for _, block := range bc.Blocks {
// // 			if strings.HasPrefix(block.CurrentHash, v.Address) {
// // 				if proposedBlocks[block.CurrentHash] {
// // 					bc.SlashValidator(v.Address, 0.3, "double signing")
// // 					continue
// // 				}
// // 				proposedBlocks[block.CurrentHash] = true
// // 			}
// // 		}

// // 		// Check for inactivity (more lenient threshold)
// // 		if time.Since(v.LastActive) > 30*time.Minute {
// // 			bc.SlashValidator(v.Address, 0.05, "inactivity")
// // 			continue
// // 		}

// // 		// Check performance if validator has proposed blocks
// // 		if v.BlocksProposed > 0 {
// // 			successRate := float64(v.BlocksIncluded) / float64(v.BlocksProposed)
// // 			if successRate < 0.7 {
// // 				penalty := 0.05 * (1 - successRate)
// // 				bc.SlashValidator(v.Address, penalty, fmt.Sprintf("poor performance (%.2f%%)", successRate*100))
// // 			}
// // 		}

// // 		// Check stake lock time
// // 		if time.Now().After(v.LockTime) {
// // 			bc.SlashValidator(v.Address, 0.1, "stake lock expired")
// // 		}
// // 	}
// // }

// // func (bc *Blockchain_struct) SelectValidator() (Validator, error) {
// // 	if len(bc.Validators) == 0 {
// // 		return Validator{}, fmt.Errorf("no validator for selection")
// // 	}

// // 	bc.UpdateLiquidityPower()

// // 	totalWeightedLp := 0.0
// // 	for _, v := range bc.Validators {
// // 		weight := v.LiquidityPower * (1 - v.PenaltyScore)
// // 		totalWeightedLp += weight
// // 	}

// // 	if totalWeightedLp <= 0 {
// // 		return Validator{}, fmt.Errorf("no validators with positive weight")
// // 	}

// // 	rand.Seed(time.Now().UnixNano())
// // 	r := rand.Float64() * totalWeightedLp

// // 	cumulative := 0.0
// // 	for i, v := range bc.Validators {
// // 		weight := v.LiquidityPower * (1 - v.PenaltyScore)
// // 		cumulative += weight
// // 		if r <= cumulative {
// // 			bc.Validators[i].BlocksProposed++
// // 			bc.Validators[i].LastActive = time.Now()
// // 			return *bc.Validators[i], nil
// // 		}
// // 	}

// // 	return Validator{}, fmt.Errorf("selection failed")
// // }

// // Add to Validators.go
// // func (v *Validator) SignMessage(message []byte) ([]byte, error) {
// // 	// You'll need to implement this function to retrieve wallets
// // 	wallet := GetWalletByAddress(v.Address)
// // 	if wallet == nil {
// // 		return nil, fmt.Errorf("wallet not found for validator %s", v.Address)
// // 	}
// // 	return wallet.Sign(message)
// // }

// // Add this helper function (also in Validators.go)
// // var walletRegistry = make(map[string]*wallet.Wallet) // Maps address to Wallet

// // func RegisterValidatorWallet(address string, w *wallet.Wallet) {
// // 	walletRegistry[address] = w
// // }

// // func GetWalletByAddress(address string) *wallet.Wallet {
// // 	return walletRegistry[address]
// // }

// // func (bc *Blockchain_struct) SelectValidator() (Validator, error) {
// // 	if len(bc.Validators) == 0 {
// // 		return Validator{}, fmt.Errorf("no validator for selection")
// // 	}
// // 	if len(bc.Validators) == 1 {
// // 		return *bc.Validators[0], fmt.Errorf("1 validator has for selection")
// // 	}
// // 	bc.UpdateLiquidityPower()

// // 	totalWeightedLp := 0.0
// // 	for _, v := range bc.Validators {
// // 		// Validators with higher penalty scores have reduced selection probability
// // 		weight := v.LiquidityPower * (1 - v.PenaltyScore)
// // 		totalWeightedLp += weight
// // 	}

// // 	rand.Seed(time.Now().UnixNano())
// // 	r := rand.Float64() * totalWeightedLp

// // 	cumulative := 0.0
// // 	for i, v := range bc.Validators {
// // 		weight := v.LiquidityPower * (1 - v.PenaltyScore)
// // 		cumulative += weight
// // 		if r <= cumulative {
// // 			// Update validator stats
// // 			bc.Validators[i].BlocksProposed++
// // 			bc.Validators[i].LastActive = time.Now()
// // 			return *bc.Validators[i], nil
// // 		}
// // 	}

// // 	return Validator{}, fmt.Errorf("selection failed")
// // 	// totalLp := 0.0

// // 	// for _, v := range bc.Validators {
// // 	// 	totalLp += v.LiquidityPower
// // 	// }

// // 	// rand.Seed(time.Now().UnixNano())
// // 	// r := rand.Float64() * totalLp

// // 	// cumulative := 0.0
// // 	// for i, v := range bc.Validators {
// // 	// 	cumulative += v.LiquidityPower
// // 	// 	if r <= cumulative {
// // 	// 		return *bc.Validators[i], nil
// // 	// 	}
// // 	// }

// // 	// return Validator{}, fmt.Errorf("selection failed")

// // }

// // func (bc *Blockchain_struct) SelectValidator() (Validator, error) {
// // 	if len(bc.Validators) == 0 {
// // 		return Validator{}, fmt.Errorf("no validator for selection")
// // 	}

// // 	bc.UpdateLiquidityPower()

// // 	totalWeight := 0.0
// // 	validators := make([]Validator, len(bc.Validators))
// // 	weights := make([]float64, len(bc.Validators))

// // 	for i, v := range bc.Validators {
// // 		// Calculate reputation score (0.5 - 1.5 range)
// // 		reputation := 1.0 + (0.5 - v.PenaltyScore)

// // 		// Calculate weight considering both liquidity power and reputation
// // 		weight := v.LiquidityPower * reputation
// // 		if weight < 0 {
// // 			weight = 0
// // 		}
// // 		validators[i] = *v
// // 		weights[i] = weight
// // 		totalWeight += weight
// // 	}

// // 	if totalWeight <= 0 {
// // 		return Validator{}, fmt.Errorf("no validators with positive weight")
// // 	}

// // 	// Use crypto/rand for better randomness
// // 	randValue := rand.Int63n(1 << 62)
// // 	r := float64(randValue) / float64(1<<62) * totalWeight

// // 	cumulative := 0.0
// // 	for i, weight := range weights {
// // 		cumulative += weight
// // 		if r <= cumulative {
// // 			// Update validator stats
// // 			bc.Validators[i].BlocksProposed++
// // 			bc.Validators[i].LastActive = time.Now()

// // 			// Log selection for monitoring
// // 			log.Printf("Selected validator: %s (LP: %.2f, Penalty: %.2f, Weight: %.2f)",
// // 				validators[i].Address,
// // 				validators[i].LiquidityPower,
// // 				validators[i].PenaltyScore,
// // 				weight)

// // 			return validators[i], nil
// // 		}
// // 	}

// // 	return Validator{}, fmt.Errorf("selection failed")
// // }

// func (bc *Blockchain_struct) SlashValidator(add string, penalty float64, reason string) {
// 	for i := 0; i < len(bc.Validators); i++ {
// 		v := bc.Validators[i]
// 		if v.Address == add {
// 			// Calculate penalty based on severity and history
// 			effectivePenalty := penalty * (1 + v.PenaltyScore)

// 			// Cap penalty to prevent complete slashing from single offense
// 			if effectivePenalty > 0.3 {
// 				effectivePenalty = 0.3
// 			}

// 			LocalPenalty := v.LPStakeAmount * effectivePenalty
// 			bc.SlashingPool += LocalPenalty
// 			bc.Validators[i].LPStakeAmount -= LocalPenalty

// 			// Increase penalty score for future offenses
// 			bc.Validators[i].PenaltyScore += 0.1

// 			// Log the slashing event
// 			log.Printf("Validator %s slashed: %f tokens (reason: %s)", add, LocalPenalty, reason)

// 			if bc.Validators[i].LPStakeAmount < bc.MinStake {
// 				bc.Validators = append(bc.Validators[:i], bc.Validators[i+1:]...)
// 				i--
// 				log.Printf("Validator %s removed due to insufficient stake", add)
// 			}
// 			return
// 		}
// 	}
// }

// // func (bc *Blockchain_struct) MonitorValidators() {
// // 	// Add startup delay for new validators
// // 	if time.Since(bc.Blocks[0].TimeStamp) < 5*time.Minute {
// // 		return // Skip monitoring during first 5 minutes
// // 	}

// // 	for _, v := range bc.Validators {
// // 		// Only check validators that have been active for at least 30 minutes
// // 		if time.Since(v.LastActive) < 30*time.Minute {
// // 			continue
// // 		}

// // 		// More lenient inactivity threshold (12 hours)
// // 		if time.Since(v.LastActive) > 12*time.Hour {
// // 			bc.SlashValidator(v.Address, 0.05, "inactivity")
// // 			continue
// // 		}

// // 		// Performance checks only after some activity
// // 		if v.BlocksProposed > 0 {
// // 			successRate := float64(v.BlocksIncluded) / float64(v.BlocksProposed)
// // 			if successRate < 0.7 {
// // 				penalty := 0.1 * (1 - successRate)
// // 				bc.SlashValidator(v.Address, penalty, "poor performance")
// // 			}
// // 		}
// // 	}
// // }

// // In Validators.go, adjust MonitorValidators:
// // func (bc *Blockchain_struct) MonitorValidators() {
// // 	for _, v := range bc.Validators {
// // 		// More lenient inactivity check (24 hours)
// // 		if time.Since(v.LastActive) > 24*time.Hour {
// // 			bc.SlashValidator(v.Address, 0.05, "inactivity")
// // 		}

// // 		// Only check performance if validator has proposed blocks
// // 		if v.BlocksProposed > 10 { // Require minimum 10 blocks before evaluating
// // 			successRate := float64(v.BlocksIncluded) / float64(v.BlocksProposed)
// // 			if successRate < 0.7 { // More lenient threshold
// // 				penalty := 0.05 * (1 - successRate) // Reduced penalty
// // 				bc.SlashValidator(v.Address, penalty, "poor block inclusion rate")
// // 			}
// // 		}
// // 	}
// // }

// // this one was last one func (bc *Blockchain_struct) MonitorValidators() {
// // 	// Only check validators that have been active for at least 5 minutes
// // 	minActiveTime := 5 * time.Minute

// // 	for _, v := range bc.Validators {
// // 		// Skip newly added validators
// // 		if time.Since(v.LastActive) < minActiveTime {
// // 			continue
// // 		}

// // 		// More lenient inactivity check (30 minutes)
// // 		if time.Since(v.LastActive) > 30*time.Minute {
// // 			bc.SlashValidator(v.Address, 0.05, "inactivity")
// // 			continue
// // 		}

// // 		// Only check performance if validator has proposed blocks
// // 		if v.BlocksProposed > 0 {
// // 			successRate := float64(v.BlocksIncluded) / float64(v.BlocksProposed)
// // 			if successRate < 0.7 {
// // 				penalty := 0.05 * (1 - successRate) // Reduced penalty
// // 				bc.SlashValidator(v.Address, penalty, "poor block inclusion rate")
// // 			}
// // 		}
// // 	}
// // }

// // func (bc *Blockchain_struct) SlashValidator(add string, penalty float64) {
// // 	for i := 0; i < len(bc.Validators); i++ {
// // 		v := bc.Validators[i]
// // 		if v.Address == add {
// // 			LocalPenalty := v.LPStakeAmount * penalty
// // 			bc.SlashingPool += LocalPenalty
// // 			bc.Validators[i].LPStakeAmount -= LocalPenalty

// //				if bc.Validators[i].LPStakeAmount < bc.MinStake {
// //					bc.Validators = append(bc.Validators[:i], bc.Validators[i+1:]...)
// //					i--
// //				}
// //				return
// //			}
// //		}
// //	}
// func (bc *Blockchain_struct) UpdateMinStake(networkLoad float64) {
// 	bc.MinStake = 1000000 * float64(constantset.Decimals) * (1 + networkLoad/10)
// }

// func (bc *Blockchain_struct) SelectValidator() (Validator, error) {
// 	if len(bc.Validators) == 0 {
// 		return Validator{}, fmt.Errorf("no validator for selection")
// 	}

// 	bc.UpdateLiquidityPower()

// 	totalWeight := 0.0
// 	validators := make([]Validator, len(bc.Validators))
// 	weights := make([]float64, len(bc.Validators))

// 	for i, v := range bc.Validators {
// 		weight := v.LiquidityPower * (1.0 - v.PenaltyScore)
// 		if weight < 0 {
// 			weight = 0
// 		}
// 		validators[i] = *v
// 		weights[i] = weight
// 		totalWeight += weight
// 	}

// 	if totalWeight <= 0 {
// 		return Validator{}, fmt.Errorf("no validators with positive weight")
// 	}

// 	// Cryptographically secure random selection
// 	randVal, err := rand.Int(rand.Reader, big.NewInt(1<<62))
// 	if err != nil {
// 		return Validator{}, fmt.Errorf("random selection failed: %v", err)
// 	}
// 	r := float64(randVal.Int64()) / float64(1<<62) * totalWeight

// 	cumulative := 0.0
// 	for i, weight := range weights {
// 		cumulative += weight
// 		if r <= cumulative {
// 			bc.Validators[i].BlocksProposed++
// 			bc.Validators[i].LastActive = time.Now()
// 			return validators[i], nil
// 		}
// 	}

// 	return Validator{}, fmt.Errorf("selection failed")
// }

package blockchaincomponent

import (
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"sync"

	"time"

	constantset "github.com/Zotish/DefenceProject/ConstantSet"
)

const (
	MaxBlockGas             = 8000000
	InactivityThreshold     = 60 * time.Minute
	DoubleSigningPenalty    = 0.2
	PerformancePenaltyScale = 0.05
	MinPerformanceThreshold = 0.5
)

type Validator struct {
	Address        string    `json:"address"`
	LPStakeAmount  float64   `json:"lp_stake_amount"`
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
			return fmt.Errorf("validator %s already exists", address)
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

// func (bc *Blockchain_struct) AddNewValidators(address string, amount float64, lockDuration time.Duration) error {
// 	bc.Mutex.Lock()
// 	defer bc.Mutex.Unlock()

// 	newVal := new(Validator)
// 	lp := amount * (lockDuration.Hours() / 8760)

// 	if amount < bc.MinStake {
// 		return fmt.Errorf("staking amount is lower than min stake %f", bc.MinStake)
// 	}

// 	newVal.Address = address
// 	newVal.LPStakeAmount = amount
// 	newVal.LockTime = time.Now().Add(lockDuration)
// 	newVal.LiquidityPower = lp
// 	bc.Validators = append(bc.Validators, newVal)

// 	//this is added for mutex issues
// 	dbCopy := *bc
// 	dbCopy.Mutex = sync.Mutex{}

// 	err := PutIntoDB(dbCopy)
// 	if err != nil {
// 		return fmt.Errorf("error while adding new validator: %v", err)
// 	}
// 	return nil
// }

//this is the last one func (bc *Blockchain_struct) AddNewValidators(address string, amount float64, lockDuration time.Duration) error {
// 	newVal := new(Validator)
// 	lp := amount * (lockDuration.Hours() / 8760)

// 	if amount < bc.MinStake { // Changed to correct comparison
// 		return fmt.Errorf("staking amount is lower than min stake %f", bc.MinStake)
// 	}

// 	newVal.Address = address
// 	newVal.LPStakeAmount = amount
// 	newVal.LockTime = time.Now().Add(lockDuration)
// 	newVal.LiquidityPower = lp
// 	bc.Validators = append(bc.Validators, newVal)
// 	err := PutIntoDB(*bc)
// 	if err != nil {
// 		return fmt.Errorf("error while adding new validator: %v", err)
// 	}
// 	return nil

// }
func (bc *Blockchain_struct) UpdateLiquidityPower() {
	for _, v := range bc.Validators {
		remainingLock := time.Until(v.LockTime).Hours()
		v.LiquidityPower = v.LPStakeAmount * (remainingLock / 8760)
	}
}

//this one is the last one func (bc *Blockchain_struct) SelectValidator() (Validator, error) {
// 	if len(bc.Validators) == 0 {
// 		return Validator{}, fmt.Errorf("no validator for selection")
// 	}

// 	bc.UpdateLiquidityPower()

// 	// Calculate total weighted liquidity power
// 	totalWeight := 0.0
// 	validators := make([]Validator, len(bc.Validators))
// 	weights := make([]float64, len(bc.Validators))

// 	for i, v := range bc.Validators {
// 		// Apply penalty score (higher penalty = lower chance)
// 		weight := v.LiquidityPower * (1.0 - v.PenaltyScore)
// 		if weight < 0 {
// 			weight = 0
// 		}
// 		validators[i] = *v
// 		weights[i] = weight
// 		totalWeight += weight
// 	}

// 	if totalWeight <= 0 {
// 		return Validator{}, fmt.Errorf("no validators with positive weight")
// 	}

// 	// Use crypto/rand for better randomness
// 	randValue := rand.Int63n(1 << 62)
// 	r := float64(randValue) / float64(1<<62) * totalWeight

// 	// Select validator based on weighted probability
// 	cumulative := 0.0
// 	for i, weight := range weights {
// 		cumulative += weight
// 		if r <= cumulative {
// 			// Update validator stats
// 			bc.Validators[i].BlocksProposed++
// 			bc.Validators[i].LastActive = time.Now()
// 			return validators[i], nil
// 		}
// 	}

// 	return Validator{}, fmt.Errorf("selection failed")
// }

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

//this is the 2nd last func (bc *Blockchain_struct) MonitorValidators() {
// 	minActiveTime := 5 * time.Minute

// 	for _, v := range bc.Validators {
// 		// Skip newly added validators
// 		if time.Since(v.LastActive) < minActiveTime {
// 			continue
// 		}

// 		// Check for double signing (compare proposed blocks)
// 		proposedBlocks := make(map[string]bool)
// 		for _, block := range bc.Blocks {
// 			if strings.HasPrefix(block.CurrentHash, v.Address) {
// 				if proposedBlocks[block.CurrentHash] {
// 					bc.SlashValidator(v.Address, 0.3, "double signing")
// 					continue
// 				}
// 				proposedBlocks[block.CurrentHash] = true
// 			}
// 		}

// 		// Check for inactivity (more lenient threshold)
// 		if time.Since(v.LastActive) > 30*time.Minute {
// 			bc.SlashValidator(v.Address, 0.05, "inactivity")
// 			continue
// 		}

// 		// Check performance if validator has proposed blocks
// 		if v.BlocksProposed > 0 {
// 			successRate := float64(v.BlocksIncluded) / float64(v.BlocksProposed)
// 			if successRate < 0.7 {
// 				penalty := 0.05 * (1 - successRate)
// 				bc.SlashValidator(v.Address, penalty, fmt.Sprintf("poor performance (%.2f%%)", successRate*100))
// 			}
// 		}

// 		// Check stake lock time
// 		if time.Now().After(v.LockTime) {
// 			bc.SlashValidator(v.Address, 0.1, "stake lock expired")
// 		}
// 	}
// }

// func (bc *Blockchain_struct) SelectValidator() (Validator, error) {
// 	if len(bc.Validators) == 0 {
// 		return Validator{}, fmt.Errorf("no validator for selection")
// 	}

// 	bc.UpdateLiquidityPower()

// 	totalWeightedLp := 0.0
// 	for _, v := range bc.Validators {
// 		weight := v.LiquidityPower * (1 - v.PenaltyScore)
// 		totalWeightedLp += weight
// 	}

// 	if totalWeightedLp <= 0 {
// 		return Validator{}, fmt.Errorf("no validators with positive weight")
// 	}

// 	rand.Seed(time.Now().UnixNano())
// 	r := rand.Float64() * totalWeightedLp

// 	cumulative := 0.0
// 	for i, v := range bc.Validators {
// 		weight := v.LiquidityPower * (1 - v.PenaltyScore)
// 		cumulative += weight
// 		if r <= cumulative {
// 			bc.Validators[i].BlocksProposed++
// 			bc.Validators[i].LastActive = time.Now()
// 			return *bc.Validators[i], nil
// 		}
// 	}

// 	return Validator{}, fmt.Errorf("selection failed")
// }

// Add to Validators.go
// func (v *Validator) SignMessage(message []byte) ([]byte, error) {
// 	// You'll need to implement this function to retrieve wallets
// 	wallet := GetWalletByAddress(v.Address)
// 	if wallet == nil {
// 		return nil, fmt.Errorf("wallet not found for validator %s", v.Address)
// 	}
// 	return wallet.Sign(message)
// }

// Add this helper function (also in Validators.go)
// var walletRegistry = make(map[string]*wallet.Wallet) // Maps address to Wallet

// func RegisterValidatorWallet(address string, w *wallet.Wallet) {
// 	walletRegistry[address] = w
// }

// func GetWalletByAddress(address string) *wallet.Wallet {
// 	return walletRegistry[address]
// }

// func (bc *Blockchain_struct) SelectValidator() (Validator, error) {
// 	if len(bc.Validators) == 0 {
// 		return Validator{}, fmt.Errorf("no validator for selection")
// 	}
// 	if len(bc.Validators) == 1 {
// 		return *bc.Validators[0], fmt.Errorf("1 validator has for selection")
// 	}
// 	bc.UpdateLiquidityPower()

// 	totalWeightedLp := 0.0
// 	for _, v := range bc.Validators {
// 		// Validators with higher penalty scores have reduced selection probability
// 		weight := v.LiquidityPower * (1 - v.PenaltyScore)
// 		totalWeightedLp += weight
// 	}

// 	rand.Seed(time.Now().UnixNano())
// 	r := rand.Float64() * totalWeightedLp

// 	cumulative := 0.0
// 	for i, v := range bc.Validators {
// 		weight := v.LiquidityPower * (1 - v.PenaltyScore)
// 		cumulative += weight
// 		if r <= cumulative {
// 			// Update validator stats
// 			bc.Validators[i].BlocksProposed++
// 			bc.Validators[i].LastActive = time.Now()
// 			return *bc.Validators[i], nil
// 		}
// 	}

// 	return Validator{}, fmt.Errorf("selection failed")
// 	// totalLp := 0.0

// 	// for _, v := range bc.Validators {
// 	// 	totalLp += v.LiquidityPower
// 	// }

// 	// rand.Seed(time.Now().UnixNano())
// 	// r := rand.Float64() * totalLp

// 	// cumulative := 0.0
// 	// for i, v := range bc.Validators {
// 	// 	cumulative += v.LiquidityPower
// 	// 	if r <= cumulative {
// 	// 		return *bc.Validators[i], nil
// 	// 	}
// 	// }

// 	// return Validator{}, fmt.Errorf("selection failed")

// }

// func (bc *Blockchain_struct) SelectValidator() (Validator, error) {
// 	if len(bc.Validators) == 0 {
// 		return Validator{}, fmt.Errorf("no validator for selection")
// 	}

// 	bc.UpdateLiquidityPower()

// 	totalWeight := 0.0
// 	validators := make([]Validator, len(bc.Validators))
// 	weights := make([]float64, len(bc.Validators))

// 	for i, v := range bc.Validators {
// 		// Calculate reputation score (0.5 - 1.5 range)
// 		reputation := 1.0 + (0.5 - v.PenaltyScore)

// 		// Calculate weight considering both liquidity power and reputation
// 		weight := v.LiquidityPower * reputation
// 		if weight < 0 {
// 			weight = 0
// 		}
// 		validators[i] = *v
// 		weights[i] = weight
// 		totalWeight += weight
// 	}

// 	if totalWeight <= 0 {
// 		return Validator{}, fmt.Errorf("no validators with positive weight")
// 	}

// 	// Use crypto/rand for better randomness
// 	randValue := rand.Int63n(1 << 62)
// 	r := float64(randValue) / float64(1<<62) * totalWeight

// 	cumulative := 0.0
// 	for i, weight := range weights {
// 		cumulative += weight
// 		if r <= cumulative {
// 			// Update validator stats
// 			bc.Validators[i].BlocksProposed++
// 			bc.Validators[i].LastActive = time.Now()

// 			// Log selection for monitoring
// 			log.Printf("Selected validator: %s (LP: %.2f, Penalty: %.2f, Weight: %.2f)",
// 				validators[i].Address,
// 				validators[i].LiquidityPower,
// 				validators[i].PenaltyScore,
// 				weight)

// 			return validators[i], nil
// 		}
// 	}

// 	return Validator{}, fmt.Errorf("selection failed")
// }

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

// func (bc *Blockchain_struct) MonitorValidators() {
// 	// Add startup delay for new validators
// 	if time.Since(bc.Blocks[0].TimeStamp) < 5*time.Minute {
// 		return // Skip monitoring during first 5 minutes
// 	}

// 	for _, v := range bc.Validators {
// 		// Only check validators that have been active for at least 30 minutes
// 		if time.Since(v.LastActive) < 30*time.Minute {
// 			continue
// 		}

// 		// More lenient inactivity threshold (12 hours)
// 		if time.Since(v.LastActive) > 12*time.Hour {
// 			bc.SlashValidator(v.Address, 0.05, "inactivity")
// 			continue
// 		}

// 		// Performance checks only after some activity
// 		if v.BlocksProposed > 0 {
// 			successRate := float64(v.BlocksIncluded) / float64(v.BlocksProposed)
// 			if successRate < 0.7 {
// 				penalty := 0.1 * (1 - successRate)
// 				bc.SlashValidator(v.Address, penalty, "poor performance")
// 			}
// 		}
// 	}
// }

// In Validators.go, adjust MonitorValidators:
// func (bc *Blockchain_struct) MonitorValidators() {
// 	for _, v := range bc.Validators {
// 		// More lenient inactivity check (24 hours)
// 		if time.Since(v.LastActive) > 24*time.Hour {
// 			bc.SlashValidator(v.Address, 0.05, "inactivity")
// 		}

// 		// Only check performance if validator has proposed blocks
// 		if v.BlocksProposed > 10 { // Require minimum 10 blocks before evaluating
// 			successRate := float64(v.BlocksIncluded) / float64(v.BlocksProposed)
// 			if successRate < 0.7 { // More lenient threshold
// 				penalty := 0.05 * (1 - successRate) // Reduced penalty
// 				bc.SlashValidator(v.Address, penalty, "poor block inclusion rate")
// 			}
// 		}
// 	}
// }

// this one was last one func (bc *Blockchain_struct) MonitorValidators() {
// 	// Only check validators that have been active for at least 5 minutes
// 	minActiveTime := 5 * time.Minute

// 	for _, v := range bc.Validators {
// 		// Skip newly added validators
// 		if time.Since(v.LastActive) < minActiveTime {
// 			continue
// 		}

// 		// More lenient inactivity check (30 minutes)
// 		if time.Since(v.LastActive) > 30*time.Minute {
// 			bc.SlashValidator(v.Address, 0.05, "inactivity")
// 			continue
// 		}

// 		// Only check performance if validator has proposed blocks
// 		if v.BlocksProposed > 0 {
// 			successRate := float64(v.BlocksIncluded) / float64(v.BlocksProposed)
// 			if successRate < 0.7 {
// 				penalty := 0.05 * (1 - successRate) // Reduced penalty
// 				bc.SlashValidator(v.Address, penalty, "poor block inclusion rate")
// 			}
// 		}
// 	}
// }

// func (bc *Blockchain_struct) SlashValidator(add string, penalty float64) {
// 	for i := 0; i < len(bc.Validators); i++ {
// 		v := bc.Validators[i]
// 		if v.Address == add {
// 			LocalPenalty := v.LPStakeAmount * penalty
// 			bc.SlashingPool += LocalPenalty
// 			bc.Validators[i].LPStakeAmount -= LocalPenalty

//				if bc.Validators[i].LPStakeAmount < bc.MinStake {
//					bc.Validators = append(bc.Validators[:i], bc.Validators[i+1:]...)
//					i--
//				}
//				return
//			}
//		}
//	}
func (bc *Blockchain_struct) UpdateMinStake(networkLoad float64) {
	bc.MinStake = 1000000 * float64(constantset.Decimals) * (1 + networkLoad/10)
}

func (bc *Blockchain_struct) SelectValidator() (Validator, error) {
	if len(bc.Validators) == 0 {
		return Validator{}, fmt.Errorf("no validator for selection")
	}

	bc.UpdateLiquidityPower()

	totalWeight := 0.0
	validators := make([]Validator, len(bc.Validators))
	weights := make([]float64, len(bc.Validators))

	for i, v := range bc.Validators {
		weight := v.LiquidityPower * (1.0 - v.PenaltyScore)
		if weight < 0 {
			weight = 0
		}
		validators[i] = *v
		weights[i] = weight
		totalWeight += weight
	}

	if totalWeight <= 0 {
		return Validator{}, fmt.Errorf("no validators with positive weight")
	}

	// Cryptographically secure random selection
	randVal, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		return Validator{}, fmt.Errorf("random selection failed: %v", err)
	}
	r := float64(randVal.Int64()) / float64(1<<62) * totalWeight

	cumulative := 0.0
	for i, weight := range weights {
		cumulative += weight
		if r <= cumulative {
			bc.Validators[i].BlocksProposed++
			bc.Validators[i].LastActive = time.Now()
			return validators[i], nil
		}
	}

	return Validator{}, fmt.Errorf("selection failed")
}
