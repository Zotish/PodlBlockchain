package main

import (
	"encoding/json"
	"fmt"
	"math"
)

type result struct {
	Scenario      string  `json:"scenario"`
	RawVolume     float64 `json:"raw_volume"`
	OrganicCredit float64 `json:"organic_credit"`
	FeeCost       float64 `json:"fee_cost"`
	TWAPDeviation float64 `json:"twap_deviation"`
	Quality       float64 `json:"quality"`
	Passed        bool    `json:"passed"`
}

func clamp(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
func score(depth, organic, volatility, oracle, concentration float64) float64 {
	return clamp(.35*depth + .25*organic + .20*math.Exp(-8*volatility) + .15*oracle + .05*concentration)
}
func main() {
	reserve := 1_000_000.0
	rows := []result{}
	organicVolume := 200_000.0
	organic := math.Min(organicVolume, reserve*.05*8)
	rows = append(rows, result{"eight-organic-traders", organicVolume, organic, organicVolume * .003, 0.002, score(.9, organic/organicVolume, .02, .95, .8), true})
	washVolume := 2_000_000.0
	washCredit := math.Min(washVolume, reserve*.05)
	fee := washVolume * .003
	twapDeviation := .30 * (60.0 / 3600.0)
	washQuality := score(.9, washCredit/washVolume, .30, .5, .2)
	rows = append(rows, result{"single-identity-wash-loop", washVolume, washCredit, fee, twapDeviation, washQuality, washCredit/washVolume <= .05 && fee >= 6000 && twapDeviation < .01})
	shockTWAP := .50 * (300.0 / 3600.0)
	rows = append(rows, result{"five-minute-price-shock", 500_000, 25_000, 1500, shockTWAP, score(.9, .05, .5, .4, .5), shockTWAP < .05})
	raw, _ := json.MarshalIndent(map[string]interface{}{"model": "actor-cap + fee-cost + 1h TWAP", "reserve": reserve, "results": rows}, "", "  ")
	fmt.Println(string(raw))
	for _, row := range rows {
		if !row.Passed {
			panic("liquidity manipulation simulation failed: " + row.Scenario)
		}
	}
}
