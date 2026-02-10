package main

import (
	"math/big"
	"strings"

	blockchaincomponent "github.com/Zotish/DefenceProject/BlockchainComponent"
)

type NFTCollection struct{}

func parseBig(v string) *big.Int {
	v = strings.TrimSpace(v)
	if v == "" {
		return big.NewInt(0)
	}
	z := new(big.Int)
	if _, ok := z.SetString(v, 10); !ok {
		return big.NewInt(0)
	}
	return z
}

func (n *NFTCollection) Init(ctx *blockchaincomponent.Context, name string, symbol string) {
	if name == "" {
		name = "NFT Collection"
	}
	if symbol == "" {
		symbol = "NFT"
	}
	ctx.Set("name", name)
	ctx.Set("symbol", symbol)
	ctx.Set("totalSupply", "0")
	ctx.Emit("Init", map[string]interface{}{
		"name":   name,
		"symbol": symbol,
	})
}

func (n *NFTCollection) Name(ctx *blockchaincomponent.Context) {
	ctx.Set("output", ctx.Get("name"))
}

func (n *NFTCollection) Symbol(ctx *blockchaincomponent.Context) {
	ctx.Set("output", ctx.Get("symbol"))
}

func (n *NFTCollection) TotalSupply(ctx *blockchaincomponent.Context) {
	ctx.Set("output", ctx.Get("totalSupply"))
}

func (n *NFTCollection) Mint(ctx *blockchaincomponent.Context, to string, tokenId string) {
	if to == "" || tokenId == "" {
		ctx.Revert("invalid mint params")
	}
	key := "owner:" + tokenId
	if ctx.Get(key) != "" {
		ctx.Revert("token already minted")
	}
	ctx.Set(key, to)
	total := ctx.Get("totalSupply")
	if total == "" {
		total = "0"
	}
	ctx.Set("totalSupply", new(big.Int).Add(parseBig(total), big.NewInt(1)).String())
	ctx.Emit("Mint", map[string]interface{}{
		"to":      to,
		"tokenId": tokenId,
	})
}

func (n *NFTCollection) OwnerOf(ctx *blockchaincomponent.Context, tokenId string) {
	owner := ctx.Get("owner:" + tokenId)
	if owner == "" {
		owner = "0x0000000000000000000000000000000000000000"
	}
	ctx.Set("output", owner)
}

func (n *NFTCollection) Transfer(ctx *blockchaincomponent.Context, to string, tokenId string) {
	if to == "" || tokenId == "" {
		ctx.Revert("invalid transfer params")
	}
	key := "owner:" + tokenId
	owner := ctx.Get(key)
	if owner == "" || owner != ctx.CallerAddr {
		ctx.Revert("not token owner")
	}
	ctx.Set(key, to)
	ctx.Emit("Transfer", map[string]interface{}{
		"from":    ctx.CallerAddr,
		"to":      to,
		"tokenId": tokenId,
	})
}

// REQUIRED EXPORT
var Contract = &NFTCollection{}
