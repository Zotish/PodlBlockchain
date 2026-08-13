package blockchaincomponent

import (
	"math"
	"sort"
	"strings"
)

type OracleSourcePolicy struct {
	PrimarySources  []string `json:"primary_sources"`
	FallbackSources []string `json:"fallback_sources"`
	MinimumPrimary  int      `json:"minimum_primary"`
	MinimumFallback int      `json:"minimum_fallback"`
	MaxAgeSeconds   int64    `json:"max_age_seconds"`
	MaxDeviationBPS int64    `json:"max_deviation_bps"`
}

type OracleSourceHealth struct {
	Source       string  `json:"source"`
	Fresh        bool    `json:"fresh"`
	InBand       bool    `json:"in_band"`
	AgeSeconds   int64   `json:"age_seconds"`
	DeviationBPS int64   `json:"deviation_bps"`
	Confidence   float64 `json:"confidence"`
	Healthy      bool    `json:"healthy"`
}

type ResilientOraclePrice struct {
	MedianOraclePrice
	Tier          string               `json:"tier"`
	SourceHealth  []OracleSourceHealth `json:"source_health"`
	FallbackUsed  bool                 `json:"fallback_used"`
	PrimaryFailed bool                 `json:"primary_failed"`
}

func normalizedSourceSet(sources []string) map[string]bool {
	out := make(map[string]bool, len(sources))
	for _, source := range sources {
		if source = strings.ToLower(strings.TrimSpace(source)); source != "" {
			out[source] = true
		}
	}
	return out
}

func oracleTierPrice(asset string, observations map[string]OracleObservation, sources map[string]bool, minimum int, now, maxAge, maxDeviation int64) (MedianOraclePrice, []OracleSourceHealth) {
	out := MedianOraclePrice{Asset: strings.ToUpper(strings.TrimSpace(asset))}
	if minimum <= 0 {
		minimum = OracleMinSources
	}
	if maxAge <= 0 {
		maxAge = OracleMaxAgeSeconds
	}
	if maxDeviation <= 0 {
		maxDeviation = OracleMaxDeviationBPS
	}
	eligible := make([]OracleObservation, 0, len(observations))
	prices := []float64{}
	for source, observation := range observations {
		if len(sources) > 0 && !sources[strings.ToLower(source)] {
			continue
		}
		age := now - observation.Timestamp
		if observation.PriceUSD > 0 && observation.Confidence >= .5 && age >= -30 && age <= maxAge {
			eligible = append(eligible, observation)
			prices = append(prices, observation.PriceUSD)
		}
	}
	median := medianFloat(prices)
	health := make([]OracleSourceHealth, 0, len(observations))
	accepted := []float64{}
	confidence := 0.0
	for source, observation := range observations {
		if len(sources) > 0 && !sources[strings.ToLower(source)] {
			continue
		}
		age := now - observation.Timestamp
		deviation := int64(0)
		if median > 0 {
			deviation = int64(math.Round(math.Abs(observation.PriceUSD-median) / median * 10_000))
		}
		entry := OracleSourceHealth{Source: strings.ToLower(source), Fresh: age >= -30 && age <= maxAge, InBand: deviation <= maxDeviation, AgeSeconds: age, DeviationBPS: deviation, Confidence: observation.Confidence}
		entry.Healthy = entry.Fresh && entry.InBand && observation.PriceUSD > 0 && observation.Confidence >= .5
		health = append(health, entry)
		if entry.Healthy {
			accepted = append(accepted, observation.PriceUSD)
			confidence += observation.Confidence
			out.Sources = append(out.Sources, strings.ToLower(source))
			if observation.Timestamp > out.UpdatedAt {
				out.UpdatedAt = observation.Timestamp
			}
		} else {
			out.RejectCount++
		}
	}
	sort.Slice(health, func(i, j int) bool { return health[i].Source < health[j].Source })
	sort.Strings(out.Sources)
	if len(accepted) >= minimum {
		out.PriceUSD = medianFloat(accepted)
		out.Confidence = confidence / float64(len(accepted))
		out.Valid = out.Confidence >= .5
	}
	out.Stale = len(eligible) == 0 && len(observations) > 0
	return out, health
}

// ResilientOraclePriceForAsset fails over only when the primary tier cannot
// form its own quorum. Primary and fallback observations are never mixed into
// one median, preventing one weak tier from silently lowering the threshold.
func (bc *Blockchain_struct) ResilientOraclePriceForAsset(asset string, policy OracleSourcePolicy, now int64) ResilientOraclePrice {
	out := ResilientOraclePrice{Tier: "unavailable"}
	if bc == nil {
		return out
	}
	bc.EnsureRuntimeState()
	observations := bc.OracleObservations[strings.ToUpper(strings.TrimSpace(asset))]
	primary, primaryHealth := oracleTierPrice(asset, observations, normalizedSourceSet(policy.PrimarySources), policy.MinimumPrimary, now, policy.MaxAgeSeconds, policy.MaxDeviationBPS)
	if primary.Valid {
		out.MedianOraclePrice, out.Tier, out.SourceHealth = primary, "primary", primaryHealth
		return out
	}
	fallback, fallbackHealth := oracleTierPrice(asset, observations, normalizedSourceSet(policy.FallbackSources), policy.MinimumFallback, now, policy.MaxAgeSeconds, policy.MaxDeviationBPS)
	out.MedianOraclePrice, out.SourceHealth, out.PrimaryFailed = fallback, append(primaryHealth, fallbackHealth...), true
	if fallback.Valid {
		out.Tier, out.FallbackUsed = "fallback", true
	}
	return out
}
