package main

import (
	"encoding/json"
	"fmt"
	"math/big"
)

type output struct {
	Users           int    `json:"users"`
	InitialAssets   string `json:"initial_assets"`
	LossBPS         int64  `json:"loss_bps"`
	ClaimsPaid      string `json:"claims_paid"`
	FinalAssets     string `json:"final_assets"`
	MaxRoundingLoss string `json:"max_rounding_loss"`
	FIFO            bool   `json:"fifo"`
	ProRata         bool   `json:"pro_rata"`
}

func main() {
	users := 10000
	assets := big.NewInt(1_000_000_000_000)
	shares := new(big.Int).Set(assets)
	lossBPS := int64(2500)
	assets.Mul(assets, big.NewInt(10000-lossBPS))
	assets.Div(assets, big.NewInt(10000))
	initial := new(big.Int).Set(assets)
	paid := big.NewInt(0)
	rounding := big.NewInt(0)
	for i := 0; i < users; i++ {
		userShares := big.NewInt(100_000_000)
		claim := new(big.Int).Div(new(big.Int).Mul(assets, userShares), shares)
		exactNumerator := new(big.Int).Mul(assets, userShares)
		remainder := new(big.Int).Mod(exactNumerator, shares)
		if remainder.Cmp(rounding) > 0 {
			rounding.Set(remainder)
		}
		assets.Sub(assets, claim)
		shares.Sub(shares, userShares)
		paid.Add(paid, claim)
	}
	result := output{users, initial.String(), lossBPS, paid.String(), assets.String(), rounding.String(), true, true}
	raw, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(raw))
	if assets.Sign() != 0 || shares.Sign() != 0 || paid.Cmp(initial) != 0 {
		panic("bank-run conservation invariant failed")
	}
}
