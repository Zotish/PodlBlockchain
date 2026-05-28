package blockchaincomponent

import (
	"math"
	"strconv"
	"strings"
	"time"
)

type lpRewardPosition struct {
	Address      string
	PairAddress  string
	PairKey      string
	Tier         int
	TierWeight   float64
	LiquidityUSD float64
	Weight       float64
	Locked       bool
}

func (bc *Blockchain_struct) dexLPRewardPositions() []lpRewardPosition {
	if bc.ContractEngine == nil || bc.ContractEngine.DB == nil {
		return nil
	}

	var out []lpRewardPosition
	for _, pairAddress := range bc.ContractEngine.DB.ListContractAddresses() {
		storage, err := bc.ContractEngine.DB.LoadAllStorage(pairAddress)
		if err != nil || len(storage) == 0 {
			continue
		}

		token0 := strings.ToLower(strings.TrimSpace(storage["token0"]))
		token1 := strings.ToLower(strings.TrimSpace(storage["token1"]))
		if token0 == "" || token1 == "" {
			continue
		}

		sym0 := bc.validatorTokenSymbol(token0)
		sym1 := bc.validatorTokenSymbol(token1)
		pairKey := lpRewardPairKey(sym0, sym1)
		tier, tierWeight, approved := lpRewardPoolTier(pairKey, pairAddress)
		if !approved {
			continue
		}

		totalLP := parseBigToFloat(storage["totalLP"])
		if totalLP <= 0 {
			continue
		}
		totalUSD := bc.validatorReserveUSD(token0, storage["reserve0"]) + bc.validatorReserveUSD(token1, storage["reserve1"])
		if totalUSD <= 0 {
			continue
		}

		positions := make(map[string]*lpRewardPosition)
		addPosition := func(prefix string, locked bool) {
			now := timeNowUnix()
			for key, raw := range storage {
				if !strings.HasPrefix(key, prefix) {
					continue
				}
				addr := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(key, prefix)))
				if addr == "" || addr == "0x0000000000000000000000000000000000000000" {
					continue
				}
				amount := parseBigToFloat(raw)
				if amount <= 0 {
					continue
				}
				if locked && parseInt64(storage["vlu:"+addr]) <= now {
					continue
				}
				valueUSD := totalUSD * (amount / totalLP)
				if valueUSD <= 0 {
					continue
				}
				pos := positions[addr]
				if pos == nil {
					pos = &lpRewardPosition{
						Address:     addr,
						PairAddress: strings.ToLower(pairAddress),
						PairKey:     pairKey,
						Tier:        tier,
						TierWeight:  tierWeight,
					}
					positions[addr] = pos
				}
				pos.LiquidityUSD += valueUSD
				pos.Locked = pos.Locked || locked
			}
		}

		addPosition("lp:", false)
		addPosition("vlp:", true)

		for _, pos := range positions {
			pos.Weight = math.Sqrt(pos.LiquidityUSD) * pos.TierWeight
			if pos.Weight > 0 {
				out = append(out, *pos)
			}
		}
	}
	return out
}

func lpRewardPairKey(symbolA, symbolB string) string {
	a := strings.ToUpper(strings.TrimSpace(symbolA))
	b := strings.ToUpper(strings.TrimSpace(symbolB))
	if a == "" || b == "" {
		return ""
	}
	if a == "LQD" {
		return "LQD/" + b
	}
	if b == "LQD" {
		return "LQD/" + a
	}
	if a < b {
		return a + "/" + b
	}
	return b + "/" + a
}

func lpRewardPoolTier(pairKey, pairAddress string) (int, float64, bool) {
	pairKey = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(pairKey), "-", "/"))
	pairAddress = strings.ToLower(strings.TrimSpace(pairAddress))
	if pairKey == "" || pairAddress == "" {
		return 0, 0, false
	}

	defaults := map[string]struct {
		tier   int
		weight float64
	}{
		"LQD/USDT": {tier: 1, weight: 1.25},
		"LQD/USDC": {tier: 1, weight: 1.25},
		"LQD/ETH":  {tier: 2, weight: 1.00},
		"LQD/BNB":  {tier: 2, weight: 0.90},
		"LQD/BTC":  {tier: 2, weight: 1.00},
	}
	if cfg, ok := defaults[pairKey]; ok {
		return cfg.tier, lpRewardWeightOverride(pairKey, cfg.weight), true
	}

	for key, value := range parseKVEnv("LQD_LP_REWARD_TIER3_PAIRS") {
		key = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(key), "-", "/"))
		if key != pairKey {
			continue
		}
		approvedAddr, weight := parseRewardPairApproval(value, 0.35)
		if approvedAddr == "" || !strings.EqualFold(approvedAddr, pairAddress) {
			continue
		}
		return 3, lpRewardWeightOverride(pairKey, weight), true
	}
	return 0, 0, false
}

func parseRewardPairApproval(value string, fallbackWeight float64) (string, float64) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fallbackWeight
	}
	addr, weightRaw, ok := strings.Cut(value, "|")
	if !ok {
		addr, weightRaw, ok = strings.Cut(value, "@")
	}
	weight := fallbackWeight
	if ok {
		if f, err := strconv.ParseFloat(strings.TrimSpace(weightRaw), 64); err == nil && f > 0 {
			weight = f
		}
	} else {
		addr = value
	}
	return strings.ToLower(strings.TrimSpace(addr)), weight
}

func lpRewardWeightOverride(pairKey string, fallback float64) float64 {
	for key, value := range parseKVEnv("LQD_LP_REWARD_POOL_WEIGHTS") {
		key = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(key), "-", "/"))
		if key != strings.ToUpper(pairKey) {
			continue
		}
		if f, err := strconv.ParseFloat(value, 64); err == nil && f > 0 {
			return f
		}
	}
	return fallback
}

func timeNowUnix() int64 {
	return time.Now().Unix()
}
