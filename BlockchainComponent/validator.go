package blockchaincomponent

import (
	"fmt"
	"log"
	"math"
	"math/big"
	"os"
	"sort"
	"strconv"
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
	DEXAddress          string  `json:"dex_address,omitempty"`
	DEXFactoryAddress   string  `json:"dex_factory_address,omitempty"`
	PairKey             string  `json:"pair_key,omitempty"`
	Token0              string  `json:"token0,omitempty"`
	Token1              string  `json:"token1,omitempty"`
	LPTokenAmount       string  `json:"lp_token_amount,omitempty"` // decimal big-int string
	LockedLiquidityUSD  float64 `json:"locked_liquidity_usd,omitempty"`
	ValidatorPairWeight float64 `json:"validator_pair_weight,omitempty"`

	// ── Legacy PoS (used when DEXAddress == "") ───────────────────────────────
	LPStakeAmount float64 `json:"lp_stake_amount"`

	// ── Common ───────────────────────────────────────────────────────────────
	LockTime       time.Time `json:"lock_time"`
	LiquidityPower float64   `json:"liquidity_power"`
	PenaltyScore   float64   `json:"penalty_score"`
	BlocksProposed int       `json:"blocks_proposed"`
	BlocksIncluded int       `json:"blocks_included"`
	LastActive     time.Time `json:"last_active"`
	MissedRounds   int       `json:"missed_rounds,omitempty"`
	LastPenaltyAt  time.Time `json:"last_penalty_at,omitempty"`
	JailedUntil    time.Time `json:"jailed_until,omitempty"`
	SlashReason    string    `json:"slash_reason,omitempty"`
}

func isDEXBackedValidator(v *Validator) bool {
	if v == nil {
		return false
	}
	return strings.TrimSpace(v.DEXAddress) != "" ||
		strings.TrimSpace(v.LPTokenAmount) != "" ||
		v.LockedLiquidityUSD > 0 ||
		strings.TrimSpace(v.PairKey) != ""
}

type DEXValidatorAssessment struct {
	Address            string  `json:"address"`
	PairAddress        string  `json:"pair_address"`
	PairKey            string  `json:"pair_key"`
	Token0             string  `json:"token0"`
	Token1             string  `json:"token1"`
	Token0Symbol       string  `json:"token0_symbol"`
	Token1Symbol       string  `json:"token1_symbol"`
	LockedLP           string  `json:"locked_lp"`
	TotalLP            string  `json:"total_lp"`
	LockUntil          int64   `json:"lock_until"`
	LockedLiquidityUSD float64 `json:"locked_liquidity_usd"`
	MinLiquidityUSD    float64 `json:"min_liquidity_usd"`
	PairWeight         float64 `json:"pair_weight"`
	LockMultiplier     float64 `json:"lock_multiplier"`
	LiquidityPower     float64 `json:"liquidity_power"`
	Eligible           bool    `json:"eligible"`
	Reason             string  `json:"reason,omitempty"`
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

	if amount < bc.MinStake {
		return fmt.Errorf("staking amount is lower than min stake %f", bc.MinStake)
	}

	newVal.Address = address
	newVal.LPStakeAmount = amount
	newVal.LockTime = time.Now().Add(lockDuration)
	newVal.LiquidityPower = legacyLiquidityPower(newVal.LPStakeAmount, newVal.LockTime)
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
// The validator must have locked LP in a canonical LQD pair. Eligibility is based on
// USD/LQD-equivalent locked liquidity value, not raw LP token count.
func (bc *Blockchain_struct) AddDEXValidator(address, dexAddress, lpTokenAmount string, lockDuration time.Duration) error {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()

	assessment, err := bc.assessDEXValidatorNoLock(address, dexAddress, lpTokenAmount)
	if err != nil {
		return err
	}
	if !assessment.Eligible {
		return fmt.Errorf("validator not eligible: %s", assessment.Reason)
	}

	applyAssessment := func(v *Validator) {
		v.DEXAddress = strings.ToLower(dexAddress)
		v.PairKey = assessment.PairKey
		v.Token0 = assessment.Token0
		v.Token1 = assessment.Token1
		v.LPTokenAmount = assessment.LockedLP
		v.LockedLiquidityUSD = assessment.LockedLiquidityUSD
		v.ValidatorPairWeight = assessment.PairWeight
		v.LockTime = time.Unix(assessment.LockUntil, 0)
		v.LiquidityPower = assessment.LiquidityPower
		v.LastActive = time.Now()
	}

	for _, v := range bc.Validators {
		if strings.EqualFold(v.Address, address) {
			applyAssessment(v)
			dbCopy := *bc
			dbCopy.Mutex = sync.Mutex{}
			if err := PutIntoDB(dbCopy); err != nil {
				return fmt.Errorf("error saving PosDL validator: %v", err)
			}
			log.Printf("PosDL validator updated: %s DEX=%s lpAmount=%s", address, dexAddress, assessment.LockedLP)
			return nil
		}
	}

	newVal := &Validator{
		Address: address,
	}
	applyAssessment(newVal)

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

func (bc *Blockchain_struct) AssessDEXValidator(address, pairAddress string) (*DEXValidatorAssessment, error) {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	return bc.assessDEXValidatorNoLock(address, pairAddress, "")
}

func (bc *Blockchain_struct) RegisterDEXValidator(address, pairAddress string) (*DEXValidatorAssessment, error) {
	if err := bc.AddDEXValidator(address, pairAddress, "", 0); err != nil {
		assessment, _ := bc.AssessDEXValidator(address, pairAddress)
		return assessment, err
	}
	return bc.AssessDEXValidator(address, pairAddress)
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
			// Legacy PoS: time-weighted single-asset stake, rounded to whole
			// lock days so small node start-time differences do not fork winner selection.
			v.LiquidityPower = legacyLiquidityPower(v.LPStakeAmount, v.LockTime)
		}
	}
}

func legacyLiquidityPower(stakeAmount float64, lockTime time.Time) float64 {
	remainingHours := time.Until(lockTime).Hours()
	if remainingHours <= 0 {
		return 0
	}
	remainingDays := math.Ceil(remainingHours / 24.0)
	return stakeAmount * ((remainingDays * 24.0) / 8760.0)
}

// getDEXLPPower reads the validator's locked LP position directly from contract
// storage and computes:
//
//	power = (lockedLP / totalLP) × (reserveA + reserveB) × lockMultiplier
//
// where lockMultiplier = 1 + remainingLockYears, so longer locks earn more power.
func (bc *Blockchain_struct) getDEXLPPower(v *Validator) float64 {
	assessment, err := bc.assessDEXValidatorNoLock(v.Address, v.DEXAddress, v.LPTokenAmount)
	if err != nil || assessment == nil {
		return 0
	}
	v.PairKey = assessment.PairKey
	v.Token0 = assessment.Token0
	v.Token1 = assessment.Token1
	v.LPTokenAmount = assessment.LockedLP
	v.LockedLiquidityUSD = assessment.LockedLiquidityUSD
	v.ValidatorPairWeight = assessment.PairWeight
	v.LockTime = time.Unix(assessment.LockUntil, 0)
	return assessment.LiquidityPower
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

func (bc *Blockchain_struct) assessDEXValidatorNoLock(address, pairAddress, fallbackLockedLP string) (*DEXValidatorAssessment, error) {
	address = strings.ToLower(strings.TrimSpace(address))
	pairAddress = strings.ToLower(strings.TrimSpace(pairAddress))
	if address == "" || pairAddress == "" {
		return nil, fmt.Errorf("address and pair_address are required")
	}
	if bc.ContractEngine == nil {
		return nil, fmt.Errorf("contract engine unavailable")
	}

	storage, err := bc.ContractEngine.DB.LoadAllStorage(pairAddress)
	if err != nil || len(storage) == 0 {
		return nil, fmt.Errorf("pair contract storage not found")
	}

	token0 := strings.ToLower(strings.TrimSpace(storage["token0"]))
	token1 := strings.ToLower(strings.TrimSpace(storage["token1"]))
	if token0 == "" || token1 == "" {
		return nil, fmt.Errorf("pair contract missing token0/token1")
	}

	sym0 := bc.validatorTokenSymbol(token0)
	sym1 := bc.validatorTokenSymbol(token1)
	pairKey, pairWeight, pairOK := validatorCanonicalPair(sym0, sym1)
	if !pairOK {
		return &DEXValidatorAssessment{
			Address:      address,
			PairAddress:  pairAddress,
			Token0:       token0,
			Token1:       token1,
			Token0Symbol: sym0,
			Token1Symbol: sym1,
			Eligible:     false,
			Reason:       "pair is not one of LQD/USDT, LQD/USDC, LQD/ETH, LQD/BNB, LQD/BTC",
		}, nil
	}

	lockedLP := strings.TrimSpace(storage["vlp:"+address])
	if (lockedLP == "" || lockedLP == "0") && fallbackLockedLP != "" {
		lockedLP = strings.TrimSpace(fallbackLockedLP)
	}
	totalLP := strings.TrimSpace(storage["totalLP"])
	lockUntil := parseInt64(storage["vlu:"+address])
	minUSD := validatorMinLiquidityUSD()
	if lockedLP == "" {
		lockedLP = "0"
	}
	if totalLP == "" {
		totalLP = "0"
	}

	assessment := &DEXValidatorAssessment{
		Address:         address,
		PairAddress:     pairAddress,
		PairKey:         pairKey,
		Token0:          token0,
		Token1:          token1,
		Token0Symbol:    sym0,
		Token1Symbol:    sym1,
		LockedLP:        lockedLP,
		TotalLP:         totalLP,
		LockUntil:       lockUntil,
		MinLiquidityUSD: minUSD,
		PairWeight:      pairWeight,
	}

	lockedLPFloat := parseBigToFloat(lockedLP)
	totalLPFloat := parseBigToFloat(totalLP)
	if lockedLPFloat <= 0 {
		assessment.Reason = "no locked LP found for validator address"
		return assessment, nil
	}
	if totalLPFloat <= 0 {
		assessment.Reason = "pair total LP supply is zero"
		return assessment, nil
	}
	if lockUntil <= time.Now().Unix() {
		assessment.Reason = "validator LP lock is expired or missing"
		return assessment, nil
	}

	reserve0USD := bc.validatorReserveUSD(token0, storage["reserve0"])
	reserve1USD := bc.validatorReserveUSD(token1, storage["reserve1"])
	totalPairUSD := reserve0USD + reserve1USD
	if totalPairUSD <= 0 {
		assessment.Reason = "pair reserve value is zero or token price unavailable"
		return assessment, nil
	}

	lockedValueUSD := totalPairUSD * (lockedLPFloat / totalLPFloat)
	remainingHours := time.Until(time.Unix(lockUntil, 0)).Hours()
	if remainingHours < 0 {
		remainingHours = 0
	}
	lockMultiplier := 1.0 + (remainingHours / 8760.0)
	if lockMultiplier < 1 {
		lockMultiplier = 1
	}
	assessment.LockedLiquidityUSD = lockedValueUSD
	assessment.LockMultiplier = lockMultiplier
	assessment.LiquidityPower = math.Sqrt(lockedValueUSD) * pairWeight * lockMultiplier

	if lockedValueUSD < minUSD {
		assessment.Reason = fmt.Sprintf("locked liquidity %.2f USD is below minimum %.2f USD", lockedValueUSD, minUSD)
		return assessment, nil
	}

	assessment.Eligible = true
	return assessment, nil
}

func (bc *Blockchain_struct) validatorReserveUSD(token string, rawReserve string) float64 {
	decimals := bc.validatorTokenDecimals(token)
	human := parseBigToFloat(rawReserve) / math.Pow10(decimals)
	return human * validatorTokenPriceUSD(bc.validatorTokenSymbol(token))
}

func (bc *Blockchain_struct) validatorTokenSymbol(token string) string {
	token = strings.ToLower(strings.TrimSpace(token))
	if token == "" {
		return ""
	}
	if token == "lqd" {
		return "LQD"
	}
	for addr, symbol := range parseKVEnv("LQD_VALIDATOR_TOKEN_SYMBOLS") {
		if strings.EqualFold(addr, token) {
			return strings.ToUpper(symbol)
		}
	}
	if bc.ContractEngine != nil {
		storage, err := bc.ContractEngine.DB.LoadAllStorage(token)
		if err == nil {
			if symbol := strings.TrimSpace(storage["symbol"]); symbol != "" {
				return strings.ToUpper(symbol)
			}
		}
	}
	return strings.ToUpper(token)
}

func (bc *Blockchain_struct) validatorTokenDecimals(token string) int {
	token = strings.ToLower(strings.TrimSpace(token))
	if token == "" || token == "lqd" {
		return 8
	}
	for addr, decimals := range parseKVEnv("LQD_VALIDATOR_TOKEN_DECIMALS") {
		if strings.EqualFold(addr, token) {
			if n, err := strconv.Atoi(decimals); err == nil && n >= 0 && n <= 36 {
				return n
			}
		}
	}
	if bc.ContractEngine != nil {
		storage, err := bc.ContractEngine.DB.LoadAllStorage(token)
		if err == nil {
			if n, err := strconv.Atoi(strings.TrimSpace(storage["decimals"])); err == nil && n >= 0 && n <= 36 {
				return n
			}
		}
	}
	return 8
}

func validatorCanonicalPair(symbolA, symbolB string) (string, float64, bool) {
	a := strings.ToUpper(strings.TrimSpace(symbolA))
	b := strings.ToUpper(strings.TrimSpace(symbolB))
	if a != "LQD" && b != "LQD" {
		return "", 0, false
	}
	quote := a
	if quote == "LQD" {
		quote = b
	}
	allowed := map[string]float64{
		"USDT": 1.00,
		"USDC": 1.00,
		"ETH":  1.10,
		"BNB":  1.10,
		"BTC":  1.20,
	}
	weight, ok := allowed[quote]
	if !ok {
		return "", 0, false
	}
	if override := validatorPairWeight("LQD/" + quote); override > 0 {
		weight = override
	}
	return "LQD/" + quote, weight, true
}

func validatorMinLiquidityUSD() float64 {
	if v := parseEnvFloat("LQD_VALIDATOR_MIN_LIQUIDITY_USD"); v > 0 {
		return v
	}
	return 100000
}

func validatorTokenPriceUSD(symbol string) float64 {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	defaults := map[string]float64{
		"LQD":  1,
		"USDT": 1,
		"USDC": 1,
		"ETH":  3500,
		"BNB":  650,
		"BTC":  100000,
	}
	if prices := parseKVEnv("LQD_VALIDATOR_TOKEN_PRICES"); len(prices) > 0 {
		if v, ok := prices[symbol]; ok {
			if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
				return f
			}
		}
	}
	return defaults[symbol]
}

func validatorPairWeight(pairKey string) float64 {
	for key, value := range parseKVEnv("LQD_VALIDATOR_PAIR_WEIGHTS") {
		if strings.EqualFold(strings.ReplaceAll(key, "-", "/"), pairKey) {
			if f, err := strconv.ParseFloat(value, 64); err == nil && f > 0 {
				return f
			}
		}
	}
	return 0
}

func parseKVEnv(name string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.Split(os.Getenv(name), ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			key, value, ok = strings.Cut(part, ":")
		}
		if !ok {
			continue
		}
		out[strings.ToUpper(strings.TrimSpace(key))] = strings.TrimSpace(value)
	}
	return out
}

func parseEnvFloat(name string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(os.Getenv(name)), 64)
	if err != nil {
		return 0
	}
	return v
}

func parseEnvInt(name string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 0 {
		return fallback
	}
	return v
}

func validatorOfflineGraceRounds() int {
	return parseEnvInt("LQD_VALIDATOR_OFFLINE_GRACE_ROUNDS", 30)
}

func validatorPenaltyCooldown() time.Duration {
	return time.Duration(parseEnvInt("LQD_VALIDATOR_PENALTY_COOLDOWN_SEC", 600)) * time.Second
}

func validatorJailDuration() time.Duration {
	return time.Duration(parseEnvInt("LQD_VALIDATOR_JAIL_DURATION_SEC", 3600)) * time.Second
}

func validatorMaxPenaltyScore() float64 {
	v := parseEnvFloat("LQD_VALIDATOR_MAX_PENALTY_SCORE")
	if v > 0 && v <= 1 {
		return v
	}
	return 0.95
}

func parseInt64(v string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	return n
}

func (bc *Blockchain_struct) MonitorValidators() {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()

	minActiveTime := 5 * time.Minute
	currentTime := time.Now()
	localValidator := strings.TrimSpace(bc.LocalValidator)
	changed := false

	seenByHeight := make(map[uint64]map[string]string)
	for _, block := range bc.Blocks {
		if block == nil {
			continue
		}
		validator := strings.TrimSpace(block.RewardBreakdown.Validator)
		if validator == "" {
			continue
		}
		if seenByHeight[block.BlockNumber] == nil {
			seenByHeight[block.BlockNumber] = make(map[string]string)
		}
		key := strings.ToLower(validator)
		if existingHash, exists := seenByHeight[block.BlockNumber][key]; exists && existingHash != block.CurrentHash {
			bc.SlashValidator(validator, DoubleSigningPenalty, "double signing")
			changed = true
			log.Printf("Double signing detected by validator %s for height %d", validator, block.BlockNumber)
		}
		seenByHeight[block.BlockNumber][key] = block.CurrentHash
	}

	for _, v := range bc.Validators {
		if v == nil {
			continue
		}

		if !v.JailedUntil.IsZero() && currentTime.Before(v.JailedUntil) {
			continue
		}

		if isDEXBackedValidator(v) {
			if currentTime.After(v.LockTime) {
				bc.SlashValidator(v.Address, 0.1, "dex lp lock expired")
				changed = true
				log.Printf("PosDL validator %s penalized for expired DEX LP lock", v.Address)
				continue
			}

			if !bc.validatorSelectableOnThisNode(v, localValidator) {
				v.MissedRounds++
				v.SlashReason = "validator node offline or not synced"
				changed = true
				if v.MissedRounds > validatorOfflineGraceRounds() && currentTime.Sub(v.LastPenaltyAt) >= validatorPenaltyCooldown() {
					bc.SlashValidator(v.Address, 0.05, "validator node offline or not synced")
					changed = true
					log.Printf("PosDL validator %s penalized for offline/sync miss rounds=%d", v.Address, v.MissedRounds)
				}
				continue
			}

			if v.MissedRounds != 0 || v.SlashReason == "validator node offline or not synced" {
				v.MissedRounds = 0
				v.SlashReason = ""
				changed = true
			}
			if !v.JailedUntil.IsZero() && currentTime.After(v.JailedUntil) {
				v.JailedUntil = time.Time{}
				changed = true
			}
			v.LastActive = currentTime
			continue
		}

		if currentTime.Sub(v.LastActive) < minActiveTime {
			continue
		}

		if currentTime.Sub(v.LastActive) > InactivityThreshold {
			bc.SlashValidator(v.Address, 0.05, "inactivity")
			changed = true
			log.Printf("Validator %s slashed for inactivity", v.Address)
			continue
		}

		if v.BlocksProposed > 0 {
			successRate := float64(v.BlocksIncluded) / float64(v.BlocksProposed)
			if successRate < MinPerformanceThreshold {
				penalty := PerformancePenaltyScale * (1 - successRate)
				bc.SlashValidator(v.Address, penalty, fmt.Sprintf("poor performance (%.2f%%)", successRate*100))
				changed = true
				log.Printf("Validator %s slashed for poor performance (%.2f%%)", v.Address, successRate*100)
			}
		}

		if currentTime.After(v.LockTime) {
			bc.SlashValidator(v.Address, 0.1, "stake lock expired")
			changed = true
			log.Printf("Validator %s slashed for expired stake lock", v.Address)
		}

		if v.BlocksProposed > 10 {
			recentMissRate := float64(v.BlocksProposed-v.BlocksIncluded) / float64(v.BlocksProposed)
			if recentMissRate > 0.5 {
				bc.SlashValidator(v.Address, 0.15, "high miss rate")
				changed = true
				log.Printf("Validator %s slashed for high miss rate (%.2f%%)", v.Address, recentMissRate*100)
			}
		}
	}

	if changed {
		dbCopy := *bc
		dbCopy.Mutex = sync.Mutex{}
		if err := PutIntoDB(dbCopy); err != nil {
			log.Printf("Failed to persist validator monitor state: %v", err)
		}
	}
}

func (bc *Blockchain_struct) SlashValidator(add string, penalty float64, reason string) {
	if penalty <= 0 {
		penalty = 0.01
	}
	now := time.Now()
	maxPenalty := validatorMaxPenaltyScore()

	for i := 0; i < len(bc.Validators); i++ {
		v := bc.Validators[i]
		if v == nil || !strings.EqualFold(v.Address, add) {
			continue
		}

		effectivePenalty := penalty * (1 + v.PenaltyScore)
		if effectivePenalty > 0.3 {
			effectivePenalty = 0.3
		}

		updated := bc.Validators[i]
		updated.PenaltyScore += effectivePenalty
		if updated.PenaltyScore > maxPenalty {
			updated.PenaltyScore = maxPenalty
		}
		updated.LastPenaltyAt = now
		updated.SlashReason = reason
		if updated.PenaltyScore >= maxPenalty {
			updated.JailedUntil = now.Add(validatorJailDuration())
		}

		if isDEXBackedValidator(v) {
			log.Printf("PosDL validator %s penalized (reason: %s, penalty: %.4f, score: %.4f, jailed_until: %s)", add, reason, effectivePenalty, updated.PenaltyScore, updated.JailedUntil.Format(time.RFC3339))
			return
		}

		localPenalty := v.LPStakeAmount * effectivePenalty
		bc.SlashingPool += localPenalty
		updated.LPStakeAmount -= localPenalty

		log.Printf("Validator %s slashed: %f tokens (reason: %s)", add, localPenalty, reason)

		if updated.LPStakeAmount < bc.MinStake {
			bc.Validators = append(bc.Validators[:i], bc.Validators[i+1:]...)
			i--
			log.Printf("Validator %s removed due to insufficient stake", add)
		}
		return
	}
}

func (bc *Blockchain_struct) UpdateMinStake(networkLoad float64) {
	bc.MinStake = 1000000 * float64(constantset.Decimals) * (1 + networkLoad/10)
}

func (bc *Blockchain_struct) validatorSelectableOnThisNode(v *Validator, localValidator string) bool {
	if v == nil {
		return false
	}
	address := strings.TrimSpace(v.Address)
	if address == "" {
		return false
	}
	now := time.Now()
	if v.PenaltyScore >= validatorMaxPenaltyScore() {
		return false
	}
	if !v.JailedUntil.IsZero() {
		if now.Before(v.JailedUntil) {
			return false
		}
		v.JailedUntil = time.Time{}
	}
	if localValidator == "" || strings.EqualFold(address, localValidator) {
		return true
	}
	if bc.Network == nil {
		return false
	}
	return bc.Network.HasVotingPeerForValidator(address, bc.latestBlockNumberForVoting())
}

func (bc *Blockchain_struct) validatorEligibleForParticipantReward(v *Validator) bool {
	if v == nil {
		return false
	}
	localValidator := strings.TrimSpace(bc.LocalValidator)
	if localValidator == "" && bc.Network == nil {
		// Preserve in-memory/unit-test behavior where there is no configured
		// canonical node or P2P service to verify remote validators.
		return true
	}
	return bc.validatorSelectableOnThisNode(v, localValidator)
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
	localValidator := strings.TrimSpace(bc.LocalValidator)
	for _, v := range bc.Validators {
		if !bc.validatorSelectableOnThisNode(v, localValidator) {
			continue
		}
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
		if localValidator != "" {
			return Validator{}, fmt.Errorf("local validator %s is not eligible for selection", bc.LocalValidator)
		}
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
