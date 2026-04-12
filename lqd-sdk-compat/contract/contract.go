// Package contract provides helpers for writing LQD smart contracts.
package contract

import (
	"errors"
	"math/big"
	"strings"
)

// RequireArg checks that args[idx] exists and returns it.
func RequireArg(args []string, idx int, name string) (string, error) {
	if idx >= len(args) || strings.TrimSpace(args[idx]) == "" {
		return "", errors.New("missing argument: " + name)
	}
	return strings.TrimSpace(args[idx]), nil
}

// ParseAmount converts a human-readable string "1.5" to raw base units
// e.g. "1.5" with decimals=8 → "150000000"
func ParseAmount(humanStr string, decimals int) string {
	s := strings.TrimSpace(humanStr)
	if s == "" || s == "0" {
		return "0"
	}
	dotIdx := strings.Index(s, ".")
	var intS, fracS string
	if dotIdx == -1 {
		intS = s
		fracS = ""
	} else {
		intS = s[:dotIdx]
		fracS = s[dotIdx+1:]
	}
	if len(fracS) > decimals {
		fracS = fracS[:decimals]
	}
	for len(fracS) < decimals {
		fracS += "0"
	}
	intS = strings.TrimLeft(intS, "0")
	if intS == "" {
		intS = "0"
	}
	result := strings.TrimLeft(intS+fracS, "0")
	if result == "" {
		return "0"
	}
	return result
}

// FormatAmount converts raw base units to human-readable string
// e.g. "150000000" with decimals=8 → "1.5"
func FormatAmount(raw string, decimals int) string {
	raw = strings.TrimLeft(raw, "0")
	if raw == "" {
		return "0"
	}
	for len(raw) <= decimals {
		raw = "0" + raw
	}
	intPart := raw[:len(raw)-decimals]
	fracPart := strings.TrimRight(raw[len(raw)-decimals:], "0")
	if fracPart == "" {
		return intPart
	}
	return intPart + "." + fracPart
}

// AddBig adds two big.Int strings and returns the result string.
func AddBig(a, b string) string {
	x, _ := new(big.Int).SetString(a, 10)
	y, _ := new(big.Int).SetString(b, 10)
	if x == nil {
		x = new(big.Int)
	}
	if y == nil {
		y = new(big.Int)
	}
	return new(big.Int).Add(x, y).String()
}

// SubBig subtracts b from a. Returns error if result would be negative.
func SubBig(a, b string) (string, error) {
	x, _ := new(big.Int).SetString(a, 10)
	y, _ := new(big.Int).SetString(b, 10)
	if x == nil {
		x = new(big.Int)
	}
	if y == nil {
		y = new(big.Int)
	}
	if x.Cmp(y) < 0 {
		return "0", errors.New("insufficient balance")
	}
	return new(big.Int).Sub(x, y).String(), nil
}

// CmpBig compares a and b. Returns -1, 0, or 1.
func CmpBig(a, b string) int {
	x, _ := new(big.Int).SetString(a, 10)
	y, _ := new(big.Int).SetString(b, 10)
	if x == nil {
		x = new(big.Int)
	}
	if y == nil {
		y = new(big.Int)
	}
	return x.Cmp(y)
}

// NormAddr normalizes an Ethereum-style address to lowercase.
func NormAddr(addr string) string {
	return strings.ToLower(strings.TrimSpace(addr))
}
