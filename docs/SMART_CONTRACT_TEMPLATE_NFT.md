# PoDL NFT Smart Contract Starter Template

This is a starter template for an NFT collection contract on PoDL.

Use it when you want to build a custom NFT, collection, or membership contract.

## Starter NFT template

```go
package main

import (
	"math/big"
	"strings"

	lqdctx "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/lqd-sdk-compat/context"
)

type NFTCollection struct{}

func normAddr(addr string) string {
	return strings.ToLower(strings.TrimSpace(addr))
}

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

func (n *NFTCollection) Init(ctx *lqdctx.Context, name string, symbol string) {
	if name == "" {
		name = "NFT Collection"
	}
	if symbol == "" {
		symbol = "NFT"
	}
	ctx.Set("name", name)
	ctx.Set("symbol", symbol)
	ctx.Set("totalSupply", "0")
}

func (n *NFTCollection) Name(ctx *lqdctx.Context) {
	ctx.Set("output", ctx.Get("name"))
}

func (n *NFTCollection) Symbol(ctx *lqdctx.Context) {
	ctx.Set("output", ctx.Get("symbol"))
}

func (n *NFTCollection) TotalSupply(ctx *lqdctx.Context) {
	ctx.Set("output", ctx.Get("totalSupply"))
}

func (n *NFTCollection) Mint(ctx *lqdctx.Context, to string, tokenId string) {
	to = normAddr(to)
	tokenId = strings.TrimSpace(tokenId)
	if to == "" || tokenId == "" {
		ctx.Revert("invalid mint params")
	}
	key := "owner:" + tokenId
	if ctx.Get(key) != "" {
		ctx.Revert("token already minted")
	}
	ctx.Set(key, to)
	total := parseBig(ctx.Get("totalSupply"))
	ctx.Set("totalSupply", new(big.Int).Add(total, big.NewInt(1)).String())
	ctx.Emit("Mint", map[string]any{
		"to":      to,
		"tokenId": tokenId,
	})
}

func (n *NFTCollection) OwnerOf(ctx *lqdctx.Context, tokenId string) {
	tokenId = strings.TrimSpace(tokenId)
	owner := ctx.Get("owner:" + tokenId)
	if owner == "" {
		owner = "0x0000000000000000000000000000000000000000"
	}
	ctx.Set("output", owner)
}

func (n *NFTCollection) Approve(ctx *lqdctx.Context, spender string, tokenId string) {
	spender = normAddr(spender)
	tokenId = strings.TrimSpace(tokenId)
	if spender == "" || tokenId == "" {
		ctx.Revert("invalid approve params")
	}
	owner := ctx.Get("owner:" + tokenId)
	if owner == "" {
		ctx.Revert("token does not exist")
	}
	if owner != ctx.CallerAddr && ctx.Get("approvalAll:"+owner+":"+ctx.CallerAddr) != "true" {
		ctx.Revert("not owner or operator")
	}
	ctx.Set("approved:"+tokenId, spender)
	ctx.Emit("Approval", map[string]any{
		"owner":   owner,
		"spender": spender,
		"tokenId": tokenId,
	})
}

func (n *NFTCollection) GetApproved(ctx *lqdctx.Context, tokenId string) {
	tokenId = strings.TrimSpace(tokenId)
	if tokenId == "" {
		ctx.Revert("tokenId required")
	}
	approved := ctx.Get("approved:" + tokenId)
	if approved == "" {
		approved = "0x0000000000000000000000000000000000000000"
	}
	ctx.Set("output", approved)
}

func (n *NFTCollection) SetApprovalForAll(ctx *lqdctx.Context, operator string, approved string) {
	operator = normAddr(operator)
	approved = strings.TrimSpace(strings.ToLower(approved))
	if operator == "" {
		ctx.Revert("operator required")
	}
	if approved != "true" && approved != "false" {
		ctx.Revert("approved must be true or false")
	}
	ctx.Set("approvalAll:"+ctx.CallerAddr+":"+operator, approved)
}

func (n *NFTCollection) IsApprovedForAll(ctx *lqdctx.Context, owner string, operator string) {
	val := ctx.Get("approvalAll:" + normAddr(owner) + ":" + normAddr(operator))
	if val == "" {
		val = "false"
	}
	ctx.Set("output", val)
}

func (n *NFTCollection) Transfer(ctx *lqdctx.Context, to string, tokenId string) {
	to = normAddr(to)
	tokenId = strings.TrimSpace(tokenId)
	if to == "" || tokenId == "" {
		ctx.Revert("invalid transfer params")
	}
	key := "owner:" + tokenId
	owner := ctx.Get(key)
	if owner == "" || owner != ctx.CallerAddr {
		ctx.Revert("not token owner")
	}
	ctx.Set(key, to)
	ctx.Set("approved:"+tokenId, "")
	ctx.Emit("Transfer", map[string]any{
		"from":    ctx.CallerAddr,
		"to":      to,
		"tokenId": tokenId,
	})
}

func (n *NFTCollection) TransferFrom(ctx *lqdctx.Context, from string, to string, tokenId string) {
	from = normAddr(from)
	to = normAddr(to)
	tokenId = strings.TrimSpace(tokenId)
	if from == "" || to == "" || tokenId == "" {
		ctx.Revert("invalid transferFrom params")
	}
	owner := ctx.Get("owner:" + tokenId)
	if owner != from {
		ctx.Revert("from is not token owner")
	}
	approved := ctx.Get("approved:" + tokenId)
	operator := ctx.Get("approvalAll:" + from + ":" + ctx.CallerAddr)
	if ctx.CallerAddr != from && ctx.CallerAddr != approved && operator != "true" {
		ctx.Revert("not authorised to transfer")
	}
	ctx.Set("owner:"+tokenId, to)
	ctx.Set("approved:"+tokenId, "")
	ctx.Emit("Transfer", map[string]any{
		"from":    from,
		"to":      to,
		"tokenId": tokenId,
	})
}

func (n *NFTCollection) Burn(ctx *lqdctx.Context, tokenId string) {
	tokenId = strings.TrimSpace(tokenId)
	if tokenId == "" {
		ctx.Revert("tokenId required")
	}
	key := "owner:" + tokenId
	owner := ctx.Get(key)
	if owner == "" {
		ctx.Revert("token does not exist")
	}
	approved := ctx.Get("approved:" + tokenId)
	operator := ctx.Get("approvalAll:" + owner + ":" + ctx.CallerAddr)
	if ctx.CallerAddr != owner && ctx.CallerAddr != approved && operator != "true" {
		ctx.Revert("not authorised to burn")
	}
	ctx.Set(key, "")
	ctx.Set("approved:"+tokenId, "")
	total := parseBig(ctx.Get("totalSupply"))
	if total.Sign() > 0 {
		ctx.Set("totalSupply", new(big.Int).Sub(total, big.NewInt(1)).String())
	}
	ctx.Emit("Burn", map[string]any{
		"from":    owner,
		"tokenId": tokenId,
	})
}

var Contract = &NFTCollection{}
```

## How to use it

1. Paste the template into the explorer compiler.
2. Compile as `Go Plugin (.so)`.
3. Deploy the collection.
4. Mint NFTs to users.
5. Use `Approve`, `SetApprovalForAll`, and `TransferFrom` for marketplace flows.

## Notes

- This is a minimal starter template.
- Add metadata URI logic if you want full marketplace support.
- Add royalty logic if your project needs secondary-sale fees.
