//go:build ignore
// +build ignore

package main

import (
	"math/big"
	"strings"

	blockchaincomponent "github.com/Zotish/DefenceProject/BlockchainComponent"
)

// Example minimal token
type LQDToken struct{}

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

func (c *LQDToken) ensureInit(ctx *blockchaincomponent.Context) {
	if ctx.Get("name") != "" {
		return
	}
	name := "test token"
	symbol := "test"
	decimals := "8"
	// 10,000,000 * 10^8 = 1,000,000,000,000,000
	totalSupply := "1000000000000000"

	ctx.Set("name", name)
	ctx.Set("symbol", symbol)
	ctx.Set("decimals", decimals)
	ctx.Set("totalSupply", totalSupply)
	ctx.Set("bal:"+normAddr(ctx.OwnerAddr), totalSupply)

	ctx.Emit("Init", map[string]interface{}{
		"name":        name,
		"symbol":      symbol,
		"decimals":    decimals,
		"totalSupply": totalSupply,
	})
}

func (c *LQDToken) Init(ctx *blockchaincomponent.Context, name string, symbol string, supply string) {
	if name == "" || symbol == "" || supply == "" {
		c.ensureInit(ctx)
		return
	}
	ctx.Set("name", name)
	ctx.Set("symbol", symbol)
	ctx.Set("totalSupply", supply)
	ctx.Set("decimals", "8")

	ctx.Set("bal:"+normAddr(ctx.OwnerAddr), supply)

	ctx.Emit("Init", map[string]interface{}{
		"name":   name,
		"symbol": symbol,
		"supply": supply,
	})
}

func (c *LQDToken) BalanceOf(ctx *blockchaincomponent.Context, addr string) {
	c.ensureInit(ctx)
	bal := ctx.Get("bal:" + normAddr(addr))
	if bal == "" {
		bal = "0"
	}
	ctx.Set("output", bal)
}

func (c *LQDToken) Transfer(ctx *blockchaincomponent.Context, to string, amount string) {
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
func (c *LQDToken) Approve(ctx *blockchaincomponent.Context, spender string, amount string) {
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
func (c *LQDToken) Allowance(ctx *blockchaincomponent.Context, owner string, spender string) {
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
func (c *LQDToken) TransferFrom(ctx *blockchaincomponent.Context, from string, to string, amount string) {
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

func (c *LQDToken) Name(ctx *blockchaincomponent.Context) {
	c.ensureInit(ctx)
	ctx.Set("output", ctx.Get("name"))
}

func (c *LQDToken) Symbol(ctx *blockchaincomponent.Context) {
	c.ensureInit(ctx)
	ctx.Set("output", ctx.Get("symbol"))
}

// Decimals returns the token decimal precision (always 8 for LQD).
func (c *LQDToken) Decimals(ctx *blockchaincomponent.Context) {
	c.ensureInit(ctx)
	ctx.Set("output", "8")
}

func (c *LQDToken) TotalSupply(ctx *blockchaincomponent.Context) {
	c.ensureInit(ctx)
	ctx.Set("output", ctx.Get("totalSupply"))
}

// Burn destroys tokens from the caller's balance.
// args[0] = amount to burn
func (c *LQDToken) Burn(ctx *blockchaincomponent.Context, amount string) {
	c.ensureInit(ctx)
	from := normAddr(ctx.CallerAddr)
	amt := parseBig(amount)
	if amt.Sign() == 0 {
		ctx.Revert("invalid burn amount")
	}

	balKey := "bal:" + from
	bal := parseBig(ctx.Get(balKey))
	if bal.Cmp(amt) < 0 {
		ctx.Revert("insufficient balance to burn")
	}

	newBal := new(big.Int).Sub(bal, amt)
	ctx.Set(balKey, newBal.String())

	supply := parseBig(ctx.Get("totalSupply"))
	ctx.Set("totalSupply", new(big.Int).Sub(supply, amt).String())

	ctx.Emit("Burn", map[string]any{
		"from":   from,
		"amount": amt.String(),
	})
}

// Mint creates new tokens and assigns them to a recipient. Only the contract owner may mint.
// args[0] = recipient address, args[1] = amount to mint
func (c *LQDToken) Mint(ctx *blockchaincomponent.Context, to string, amount string) {
	c.ensureInit(ctx)
	if normAddr(ctx.CallerAddr) != normAddr(ctx.OwnerAddr) {
		ctx.Revert("only owner can mint")
	}
	to = normAddr(to)
	amt := parseBig(amount)
	if amt.Sign() == 0 {
		ctx.Revert("invalid mint amount")
	}

	toKey := "bal:" + to
	ctx.Set(toKey, new(big.Int).Add(parseBig(ctx.Get(toKey)), amt).String())

	supply := parseBig(ctx.Get("totalSupply"))
	ctx.Set("totalSupply", new(big.Int).Add(supply, amt).String())

	ctx.Emit("Mint", map[string]any{
		"to":     to,
		"amount": amt.String(),
	})
}

// IncreaseAllowance increases the spender's allowance by an additional amount.
// args[0] = spender address, args[1] = amount to add
func (c *LQDToken) IncreaseAllowance(ctx *blockchaincomponent.Context, spender string, addedValue string) {
	c.ensureInit(ctx)
	owner := normAddr(ctx.CallerAddr)
	spender = normAddr(spender)
	added := parseBig(addedValue)
	if added.Sign() == 0 {
		ctx.Revert("invalid increase amount")
	}

	key := "allow:" + owner + ":" + spender
	current := parseBig(ctx.Get(key))
	newAllowance := new(big.Int).Add(current, added)
	ctx.Set(key, newAllowance.String())

	ctx.Emit("Approval", map[string]any{
		"owner":   owner,
		"spender": spender,
		"amount":  newAllowance.String(),
	})
}

// DecreaseAllowance decreases the spender's allowance by a given amount.
// Reverts if the subtraction would underflow below zero.
// args[0] = spender address, args[1] = amount to subtract
func (c *LQDToken) DecreaseAllowance(ctx *blockchaincomponent.Context, spender string, subtractedValue string) {
	c.ensureInit(ctx)
	owner := normAddr(ctx.CallerAddr)
	spender = normAddr(spender)
	sub := parseBig(subtractedValue)
	if sub.Sign() == 0 {
		ctx.Revert("invalid decrease amount")
	}

	key := "allow:" + owner + ":" + spender
	current := parseBig(ctx.Get(key))
	if current.Cmp(sub) < 0 {
		ctx.Revert("decreased allowance below zero")
	}

	newAllowance := new(big.Int).Sub(current, sub)
	ctx.Set(key, newAllowance.String())

	ctx.Emit("Approval", map[string]any{
		"owner":   owner,
		"spender": spender,
		"amount":  newAllowance.String(),
	})
}

// REQUIRED EXPORT
var Contract = &LQDToken{}
