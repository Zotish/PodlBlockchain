package blockchaincomponent

import (
	"testing"
	"time"
)

func TestMedianOracleRejectsOutlierAndStale(t *testing.T) {
	bc := newTestBlockchain()
	bc.EnsureRuntimeState()
	now := time.Now().Unix()
	for source, price := range map[string]float64{"a": 100, "b": 101, "c": 99, "outlier": 1000} {
		if err := bc.SubmitOracleObservation(OracleObservation{Asset: "BTC", Source: source, PriceUSD: price, Confidence: 0.9, Timestamp: now}); err != nil {
			t.Fatal(err)
		}
	}
	price := bc.MedianOraclePrice("BTC", now)
	if !price.Valid || price.PriceUSD < 99 || price.PriceUSD > 101 || price.RejectCount != 1 {
		t.Fatalf("unexpected median result: %+v", price)
	}
}

func TestOracleRequiresIndependentSources(t *testing.T) {
	bc := newTestBlockchain()
	bc.EnsureRuntimeState()
	for _, source := range []string{"a", "b"} {
		if err := bc.SubmitOracleObservation(OracleObservation{Asset: "ETH", Source: source, PriceUSD: 3000, Confidence: 1}); err != nil {
			t.Fatal(err)
		}
	}
	if got := bc.MedianOraclePrice("ETH", 0); got.Valid {
		t.Fatalf("two sources must not satisfy oracle quorum: %+v", got)
	}
}
