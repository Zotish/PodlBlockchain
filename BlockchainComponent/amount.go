package blockchaincomponent

import (
	"fmt"
	"math/big"
	"strings"
)

func NewAmountFromUint64(v uint64) *big.Int {
	return new(big.Int).SetUint64(v)
}

func NewAmountFromString(s string) (*big.Int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return big.NewInt(0), nil
	}
	z := new(big.Int)
	if _, ok := z.SetString(s, 10); !ok {
		return nil, fmt.Errorf("invalid amount: %q", s)
	}
	return z, nil
}

func NewAmountFromStringOrZero(s string) *big.Int {
	amt, err := NewAmountFromString(s)
	if err != nil {
		return big.NewInt(0)
	}
	return amt
}

func CopyAmount(a *big.Int) *big.Int {
	if a == nil {
		return big.NewInt(0)
	}
	return new(big.Int).Set(a)
}

func AmountString(a *big.Int) string {
	if a == nil {
		return "0"
	}
	return a.String()
}

// AmountToFloat64 converts a big.Int to float64 for weighting.
// Note: precision may be lost for very large values.
func AmountToFloat64(a *big.Int) float64 {
	if a == nil {
		return 0
	}
	f, _ := new(big.Float).SetInt(a).Float64()
	return f
}
