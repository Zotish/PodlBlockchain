package blockchaincomponent

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"sort"
	"strings"
)

type BatchSwapOrder struct {
	ID        string   `json:"id"`
	Owner     string   `json:"owner"`
	TokenIn   string   `json:"token_in"`
	TokenOut  string   `json:"token_out"`
	AmountIn  *big.Int `json:"amount_in"`
	MinOut    *big.Int `json:"min_out"`
	ValidFrom uint64   `json:"valid_from"`
	ExpiresAt uint64   `json:"expires_at"`
	Nonce     uint64   `json:"nonce"`
}

type BatchSwapFill struct {
	OrderID   string   `json:"order_id"`
	Owner     string   `json:"owner"`
	AmountIn  *big.Int `json:"amount_in"`
	AmountOut *big.Int `json:"amount_out"`
}

type BatchAuctionSettlement struct {
	Pair          string          `json:"pair"`
	Height        uint64          `json:"height"`
	ClearingNum   *big.Int        `json:"clearing_numerator"`
	ClearingDen   *big.Int        `json:"clearing_denominator"`
	MatchedToken0 *big.Int        `json:"matched_token0"`
	MatchedToken1 *big.Int        `json:"matched_token1"`
	Fills         []BatchSwapFill `json:"fills"`
	Unfilled      []string        `json:"unfilled"`
}

func batchOrderID(order BatchSwapOrder) string {
	material := fmt.Sprintf("PODL-BATCH-ORDER-V1:%s:%s:%s:%s:%s:%d:%d:%d", strings.ToLower(order.Owner), strings.ToLower(order.TokenIn), strings.ToLower(order.TokenOut), amountOrZero(order.AmountIn), amountOrZero(order.MinOut), order.ValidFrom, order.ExpiresAt, order.Nonce)
	sum := sha256.Sum256([]byte(material))
	return "order_" + hex.EncodeToString(sum[:12])
}

func BatchOrderID(order BatchSwapOrder) string { return batchOrderID(order) }

// ClearUniformBatch matches opposite-direction intents at one reserve-derived
// clearing price. Every included order receives the same price, eliminating
// within-batch transaction-order advantage. Unmatched inventory is returned to
// the caller for normal AMM routing; this pure function never fabricates fills.
func ClearUniformBatch(token0, token1 string, reserve0, reserve1 *big.Int, height uint64, orders []BatchSwapOrder) (BatchAuctionSettlement, error) {
	token0, token1 = strings.ToLower(strings.TrimSpace(token0)), strings.ToLower(strings.TrimSpace(token1))
	if token0 == "" || token1 == "" || token0 == token1 || reserve0 == nil || reserve1 == nil || reserve0.Sign() <= 0 || reserve1.Sign() <= 0 || height == 0 {
		return BatchAuctionSettlement{}, fmt.Errorf("valid pair reserves and height required")
	}
	settlement := BatchAuctionSettlement{Pair: token0 + ":" + token1, Height: height, ClearingNum: new(big.Int).Set(reserve1), ClearingDen: new(big.Int).Set(reserve0), MatchedToken0: big.NewInt(0), MatchedToken1: big.NewInt(0)}
	valid0, valid1 := []BatchSwapOrder{}, []BatchSwapOrder{}
	seen := map[string]bool{}
	for _, order := range orders {
		order.Owner, order.TokenIn, order.TokenOut = strings.ToLower(strings.TrimSpace(order.Owner)), strings.ToLower(strings.TrimSpace(order.TokenIn)), strings.ToLower(strings.TrimSpace(order.TokenOut))
		if order.ID == "" {
			order.ID = batchOrderID(order)
		}
		if seen[order.ID] || !ValidateAddress(order.Owner) || order.AmountIn == nil || order.MinOut == nil || order.AmountIn.Sign() <= 0 || order.MinOut.Sign() < 0 || height < order.ValidFrom || height > order.ExpiresAt {
			settlement.Unfilled = append(settlement.Unfilled, order.ID)
			continue
		}
		seen[order.ID] = true
		if order.TokenIn == token0 && order.TokenOut == token1 {
			quote := new(big.Int).Div(new(big.Int).Mul(order.AmountIn, reserve1), reserve0)
			if quote.Cmp(order.MinOut) >= 0 {
				valid0 = append(valid0, order)
			} else {
				settlement.Unfilled = append(settlement.Unfilled, order.ID)
			}
		} else if order.TokenIn == token1 && order.TokenOut == token0 {
			quote := new(big.Int).Div(new(big.Int).Mul(order.AmountIn, reserve0), reserve1)
			if quote.Cmp(order.MinOut) >= 0 {
				valid1 = append(valid1, order)
			} else {
				settlement.Unfilled = append(settlement.Unfilled, order.ID)
			}
		} else {
			settlement.Unfilled = append(settlement.Unfilled, order.ID)
		}
	}
	sort.Slice(valid0, func(i, j int) bool { return valid0[i].ID < valid0[j].ID })
	sort.Slice(valid1, func(i, j int) bool { return valid1[i].ID < valid1[j].ID })
	total0, total1 := big.NewInt(0), big.NewInt(0)
	for _, order := range valid0 {
		total0.Add(total0, order.AmountIn)
	}
	for _, order := range valid1 {
		total1.Add(total1, order.AmountIn)
	}
	available0From1 := new(big.Int).Div(new(big.Int).Mul(total1, reserve0), reserve1)
	matched0 := new(big.Int).Set(total0)
	if available0From1.Cmp(matched0) < 0 {
		matched0.Set(available0From1)
	}
	matched1 := new(big.Int).Div(new(big.Int).Mul(matched0, reserve1), reserve0)
	settlement.MatchedToken0, settlement.MatchedToken1 = matched0, matched1
	fillSide := func(side []BatchSwapOrder, matched, total, outNum, outDen *big.Int) {
		remaining := new(big.Int).Set(matched)
		for i, order := range side {
			fillIn := new(big.Int)
			if i == len(side)-1 {
				fillIn.Set(remaining)
			} else if total.Sign() > 0 {
				fillIn.Div(new(big.Int).Mul(matched, order.AmountIn), total)
				remaining.Sub(remaining, fillIn)
			}
			if fillIn.Sign() <= 0 {
				settlement.Unfilled = append(settlement.Unfilled, order.ID)
				continue
			}
			fillOut := new(big.Int).Div(new(big.Int).Mul(fillIn, outNum), outDen)
			// A partially filled order carries a pro-rata minimum output. Round
			// that minimum upward so partial settlement can never weaken the
			// user's limit price by integer truncation.
			minNumerator := new(big.Int).Mul(order.MinOut, fillIn)
			minimumForFill := new(big.Int).Div(minNumerator, order.AmountIn)
			if new(big.Int).Mod(minNumerator, order.AmountIn).Sign() > 0 {
				minimumForFill.Add(minimumForFill, big.NewInt(1))
			}
			if fillOut.Cmp(minimumForFill) < 0 {
				settlement.Unfilled = append(settlement.Unfilled, order.ID)
				continue
			}
			settlement.Fills = append(settlement.Fills, BatchSwapFill{order.ID, order.Owner, fillIn, fillOut})
			if fillIn.Cmp(order.AmountIn) < 0 {
				settlement.Unfilled = append(settlement.Unfilled, order.ID)
			}
		}
	}
	if matched0.Sign() > 0 && matched1.Sign() > 0 {
		fillSide(valid0, matched0, total0, reserve1, reserve0)
		fillSide(valid1, matched1, total1, reserve0, reserve1)
	} else {
		for _, order := range append(valid0, valid1...) {
			settlement.Unfilled = append(settlement.Unfilled, order.ID)
		}
	}
	sort.Slice(settlement.Fills, func(i, j int) bool { return settlement.Fills[i].OrderID < settlement.Fills[j].OrderID })
	sort.Strings(settlement.Unfilled)
	return settlement, nil
}
