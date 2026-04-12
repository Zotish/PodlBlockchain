//go:build ignore
// +build ignore

package main

import (
	"math/big"
	"strings"

	blockchaincomponent "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/BlockchainComponent"
)

// BridgeToken is a mint/burn token used for bridged assets.
// Owner (bridge escrow) can mint. Anyone can burn their balance to bridge back.
type BridgeToken struct{}

func parseBig(s string) *big.Int {
	s = strings.TrimSpace(s)
	if s == "" {
		return big.NewInt(0)
	}
	z := new(big.Int)
	if _, ok := z.SetString(s, 10); !ok {
		return big.NewInt(0)
	}
	return z
}

func normAddr(addr string) string {
	return strings.ToLower(strings.TrimSpace(addr))
}

func (c *BridgeToken) ensureInit(ctx *blockchaincomponent.Context) {
	if ctx.Get("name") != "" {
		return
	}
	ctx.Set("name", "Bridged Token")
	ctx.Set("symbol", "BRG")
	ctx.Set("decimals", "18")
	ctx.Set("totalSupply", "0")
	ctx.Set("bsc_token", "")
}

func (c *BridgeToken) Init(ctx *blockchaincomponent.Context, name string, symbol string, decimals string, bscToken string) {
	if name == "" || symbol == "" || decimals == "" {
		c.ensureInit(ctx)
		return
	}
	ctx.Set("name", name)
	ctx.Set("symbol", symbol)
	ctx.Set("decimals", decimals)
	ctx.Set("bsc_token", bscToken)
	if ctx.Get("totalSupply") == "" {
		ctx.Set("totalSupply", "0")
	}
	ctx.Emit("Init", map[string]interface{}{
		"name":     name,
		"symbol":   symbol,
		"decimals": decimals,
		"bsc":      bscToken,
	})
}

func (c *BridgeToken) Name(ctx *blockchaincomponent.Context) {
	c.ensureInit(ctx)
	ctx.Set("output", ctx.Get("name"))
}

func (c *BridgeToken) Symbol(ctx *blockchaincomponent.Context) {
	c.ensureInit(ctx)
	ctx.Set("output", ctx.Get("symbol"))
}

func (c *BridgeToken) Decimals(ctx *blockchaincomponent.Context) {
	c.ensureInit(ctx)
	ctx.Set("output", ctx.Get("decimals"))
}

func (c *BridgeToken) TotalSupply(ctx *blockchaincomponent.Context) {
	c.ensureInit(ctx)
	ctx.Set("output", ctx.Get("totalSupply"))
}

func (c *BridgeToken) BalanceOf(ctx *blockchaincomponent.Context, addr string) {
	c.ensureInit(ctx)
	bal := ctx.Get("bal:" + normAddr(addr))
	if bal == "" {
		bal = "0"
	}
	ctx.Set("output", bal)
}

func (c *BridgeToken) Transfer(ctx *blockchaincomponent.Context, to string, amount string) {
	c.ensureInit(ctx)
	from := normAddr(ctx.CallerAddr)
	to = normAddr(to)
	fromKey := "bal:" + from
	toKey := "bal:" + to
	fromBal := parseBig(ctx.Get(fromKey))
	amt := parseBig(amount)
	if fromBal.Cmp(amt) < 0 {
		ctx.Revert("insufficient balance")
	}
	ctx.Set(fromKey, new(big.Int).Sub(fromBal, amt).String())
	ctx.Set(toKey, new(big.Int).Add(parseBig(ctx.Get(toKey)), amt).String())
	ctx.Emit("Transfer", map[string]any{
		"from":   from,
		"to":     to,
		"amount": amount,
	})
}

// Approve allows a spender to transfer tokens on behalf of the caller.
func (c *BridgeToken) Approve(ctx *blockchaincomponent.Context, spender string, amount string) {
	c.ensureInit(ctx)
	owner := normAddr(ctx.CallerAddr)
	spender = normAddr(spender)
	key := "allow:" + owner + ":" + spender
	ctx.Set(key, amount)
	ctx.Emit("Approve", map[string]any{
		"owner":   owner,
		"spender": spender,
		"amount":  amount,
	})
}

// Allowance returns approved amount for spender.
func (c *BridgeToken) Allowance(ctx *blockchaincomponent.Context, owner string, spender string) {
	c.ensureInit(ctx)
	owner = normAddr(owner)
	spender = normAddr(spender)
	key := "allow:" + owner + ":" + spender
	val := ctx.Get(key)
	if val == "" {
		val = "0"
	}
	ctx.Set("output", val)
}

// TransferFrom moves tokens from an owner using the caller as spender.
func (c *BridgeToken) TransferFrom(ctx *blockchaincomponent.Context, from string, to string, amount string) {
	c.ensureInit(ctx)
	spender := normAddr(ctx.CallerAddr)
	from = normAddr(from)
	to = normAddr(to)
	allowKey := "allow:" + from + ":" + spender
	allow := parseBig(ctx.Get(allowKey))
	amt := parseBig(amount)
	if allow.Cmp(amt) < 0 {
		ctx.Revert("allowance too low")
	}

	fromKey := "bal:" + from
	toKey := "bal:" + to

	fromBal := parseBig(ctx.Get(fromKey))
	if fromBal.Cmp(amt) < 0 {
		ctx.Revert("insufficient balance")
	}

	ctx.Set(fromKey, new(big.Int).Sub(fromBal, amt).String())
	ctx.Set(toKey, new(big.Int).Add(parseBig(ctx.Get(toKey)), amt).String())
	ctx.Set(allowKey, new(big.Int).Sub(allow, amt).String())

	ctx.Emit("TransferFrom", map[string]any{
		"spender": spender,
		"from":    from,
		"to":      to,
		"amount":  amount,
	})
}

// Mint can only be called by contract owner (bridge escrow).
func (c *BridgeToken) Mint(ctx *blockchaincomponent.Context, to string, amount string) {
	c.ensureInit(ctx)
	if normAddr(ctx.CallerAddr) != normAddr(ctx.OwnerAddr) {
		ctx.Revert("only owner can mint")
	}
	to = normAddr(to)
	toKey := "bal:" + to
	amt := parseBig(amount)
	total := parseBig(ctx.Get("totalSupply"))
	ctx.Set("totalSupply", new(big.Int).Add(total, amt).String())
	ctx.Set(toKey, new(big.Int).Add(parseBig(ctx.Get(toKey)), amt).String())
	ctx.Emit("Mint", map[string]any{
		"to":     to,
		"amount": amount,
	})
}

// Burn reduces caller balance and emits a bridge event with destination BSC address.
func (c *BridgeToken) Burn(ctx *blockchaincomponent.Context, amount string, toBsc string) {
	c.ensureInit(ctx)
	from := normAddr(ctx.CallerAddr)
	fromKey := "bal:" + from
	amt := parseBig(amount)
	fromBal := parseBig(ctx.Get(fromKey))
	if fromBal.Cmp(amt) < 0 {
		ctx.Revert("insufficient balance")
	}
	total := parseBig(ctx.Get("totalSupply"))
	ctx.Set(fromKey, new(big.Int).Sub(fromBal, amt).String())
	if total.Cmp(amt) >= 0 {
		ctx.Set("totalSupply", new(big.Int).Sub(total, amt).String())
	}
	ctx.Emit("Burn", map[string]any{
		"from":     from,
		"amount":   amount,
		"to_bsc":   toBsc,
		"bsc":      ctx.Get("bsc_token"),
		"symbol":   ctx.Get("symbol"),
		"decimals": ctx.Get("decimals"),
	})
}

// REQUIRED EXPORT
var Contract = &BridgeToken{}
