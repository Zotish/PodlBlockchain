package blockchaincomponent

import "testing"

func TestResilientOracleUsesIndependentFallbackQuorum(t *testing.T) {
	bc := newTestBlockchain()
	bc.EnsureRuntimeState()
	now := int64(10_000)
	bc.OracleObservations["USD"] = map[string]OracleObservation{
		"p1": {Asset: "USD", Source: "p1", PriceUSD: 1, Confidence: .9, Timestamp: now - 10},
		"p2": {Asset: "USD", Source: "p2", PriceUSD: 1, Confidence: .9, Timestamp: now - 10},
		"p3": {Asset: "USD", Source: "p3", PriceUSD: 20, Confidence: .9, Timestamp: now - 10},
		"f1": {Asset: "USD", Source: "f1", PriceUSD: 1.00, Confidence: .8, Timestamp: now - 5},
		"f2": {Asset: "USD", Source: "f2", PriceUSD: 1.01, Confidence: .8, Timestamp: now - 5},
		"f3": {Asset: "USD", Source: "f3", PriceUSD: .99, Confidence: .8, Timestamp: now - 5},
	}
	policy := OracleSourcePolicy{PrimarySources: []string{"p1", "p2", "p3"}, FallbackSources: []string{"f1", "f2", "f3"}, MinimumPrimary: 3, MinimumFallback: 3, MaxAgeSeconds: 60, MaxDeviationBPS: 500}
	price := bc.ResilientOraclePriceForAsset("USD", policy, now)
	if !price.Valid || !price.FallbackUsed || !price.PrimaryFailed || price.Tier != "fallback" || price.PriceUSD != 1 {
		t.Fatalf("fallback policy failed: %#v", price)
	}
}

func TestResilientOracleNeverMixesIncompleteTiers(t *testing.T) {
	bc := newTestBlockchain()
	bc.EnsureRuntimeState()
	now := int64(100)
	bc.OracleObservations["X"] = map[string]OracleObservation{
		"p1": {Asset: "X", Source: "p1", PriceUSD: 10, Confidence: 1, Timestamp: now},
		"p2": {Asset: "X", Source: "p2", PriceUSD: 10, Confidence: 1, Timestamp: now},
		"f1": {Asset: "X", Source: "f1", PriceUSD: 10, Confidence: 1, Timestamp: now},
	}
	price := bc.ResilientOraclePriceForAsset("X", OracleSourcePolicy{PrimarySources: []string{"p1", "p2"}, FallbackSources: []string{"f1"}, MinimumPrimary: 3, MinimumFallback: 3, MaxAgeSeconds: 60}, now)
	if price.Valid {
		t.Fatalf("mixed primary/fallback sources formed an invalid quorum: %#v", price)
	}
}
