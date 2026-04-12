//go:build ignore
// +build ignore

package main

import (
	"math/big"
	"strings"

	blockchaincomponent "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/BlockchainComponent"
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
	// Clear any existing approval on transfer
	ctx.Set("approved:"+tokenId, "")
	ctx.Emit("Transfer", map[string]interface{}{
		"from":    ctx.CallerAddr,
		"to":      to,
		"tokenId": tokenId,
	})
}

// Approve approves spender to transfer a specific tokenId.
// args[0] = spender address, args[1] = tokenId
func (n *NFTCollection) Approve(ctx *blockchaincomponent.Context, spender string, tokenId string) {
	if spender == "" || tokenId == "" {
		ctx.Revert("invalid approve params")
	}
	owner := ctx.Get("owner:" + tokenId)
	if owner == "" {
		ctx.Revert("token does not exist")
	}
	// Caller must be owner or approved-for-all operator
	isOperator := ctx.Get("approvalAll:"+owner+":"+ctx.CallerAddr)
	if owner != ctx.CallerAddr && isOperator != "true" {
		ctx.Revert("not owner or operator")
	}
	ctx.Set("approved:"+tokenId, spender)
	ctx.Emit("Approval", map[string]interface{}{
		"owner":   owner,
		"spender": spender,
		"tokenId": tokenId,
	})
}

// GetApproved returns the approved address for a tokenId.
// args[0] = tokenId
func (n *NFTCollection) GetApproved(ctx *blockchaincomponent.Context, tokenId string) {
	if tokenId == "" {
		ctx.Revert("tokenId required")
	}
	approved := ctx.Get("approved:" + tokenId)
	if approved == "" {
		approved = "0x0000000000000000000000000000000000000000"
	}
	ctx.Set("output", approved)
}

// SetApprovalForAll approves or revokes an operator for all tokens owned by the caller.
// args[0] = operator address, args[1] = "true" or "false"
func (n *NFTCollection) SetApprovalForAll(ctx *blockchaincomponent.Context, operator string, approved string) {
	if operator == "" {
		ctx.Revert("operator address required")
	}
	approved = strings.TrimSpace(strings.ToLower(approved))
	if approved != "true" && approved != "false" {
		ctx.Revert("approved must be 'true' or 'false'")
	}
	if operator == ctx.CallerAddr {
		ctx.Revert("cannot approve self")
	}
	ctx.Set("approvalAll:"+ctx.CallerAddr+":"+operator, approved)
	ctx.Emit("ApprovalForAll", map[string]interface{}{
		"owner":    ctx.CallerAddr,
		"operator": operator,
		"approved": approved,
	})
}

// IsApprovedForAll returns whether operator is approved for all tokens of owner.
// args[0] = owner address, args[1] = operator address
func (n *NFTCollection) IsApprovedForAll(ctx *blockchaincomponent.Context, owner string, operator string) {
	if owner == "" || operator == "" {
		ctx.Revert("owner and operator required")
	}
	result := ctx.Get("approvalAll:" + owner + ":" + operator)
	if result == "" {
		result = "false"
	}
	ctx.Set("output", result)
}

// TransferFrom transfers a token from one address to another using an approval.
// The caller must be the owner, an approved address for the token, or an approved-for-all operator.
// args[0] = from, args[1] = to, args[2] = tokenId
func (n *NFTCollection) TransferFrom(ctx *blockchaincomponent.Context, from string, to string, tokenId string) {
	if from == "" || to == "" || tokenId == "" {
		ctx.Revert("invalid transferFrom params")
	}
	key := "owner:" + tokenId
	owner := ctx.Get(key)
	if owner == "" {
		ctx.Revert("token does not exist")
	}
	if owner != from {
		ctx.Revert("from is not token owner")
	}

	caller := ctx.CallerAddr
	approved := ctx.Get("approved:" + tokenId)
	isOperator := ctx.Get("approvalAll:" + from + ":" + caller)

	if caller != from && caller != approved && isOperator != "true" {
		ctx.Revert("not authorised to transfer")
	}

	ctx.Set(key, to)
	// Clear token-level approval on transfer
	ctx.Set("approved:"+tokenId, "")
	ctx.Emit("Transfer", map[string]interface{}{
		"from":    from,
		"to":      to,
		"tokenId": tokenId,
	})
}

// Burn destroys a token. Caller must be the owner, approved, or an approved-for-all operator.
// args[0] = tokenId
func (n *NFTCollection) Burn(ctx *blockchaincomponent.Context, tokenId string) {
	if tokenId == "" {
		ctx.Revert("tokenId required")
	}
	key := "owner:" + tokenId
	owner := ctx.Get(key)
	if owner == "" {
		ctx.Revert("token does not exist")
	}

	caller := ctx.CallerAddr
	approved := ctx.Get("approved:" + tokenId)
	isOperator := ctx.Get("approvalAll:" + owner + ":" + caller)

	if caller != owner && caller != approved && isOperator != "true" {
		ctx.Revert("not authorised to burn")
	}

	// Nullify ownership and approval
	ctx.Set(key, "")
	ctx.Set("approved:"+tokenId, "")
	// Clear any individual token URI
	ctx.Set("uri:"+tokenId, "")

	total := ctx.Get("totalSupply")
	if total == "" {
		total = "0"
	}
	supply := parseBig(total)
	if supply.Sign() > 0 {
		ctx.Set("totalSupply", new(big.Int).Sub(supply, big.NewInt(1)).String())
	}

	ctx.Emit("Burn", map[string]interface{}{
		"from":    owner,
		"tokenId": tokenId,
	})
}

// TokenURI returns the URI for a given tokenId.
// If a per-token URI is set it is returned; otherwise the base URI + tokenId is returned.
// args[0] = tokenId
func (n *NFTCollection) TokenURI(ctx *blockchaincomponent.Context, tokenId string) {
	if tokenId == "" {
		ctx.Revert("tokenId required")
	}
	if ctx.Get("owner:"+tokenId) == "" {
		ctx.Revert("token does not exist")
	}
	// Check per-token URI first
	uri := ctx.Get("uri:" + tokenId)
	if uri == "" {
		// Fall back to baseURI + tokenId
		base := ctx.Get("uri:base")
		uri = base + tokenId
	}
	ctx.Set("output", uri)
}

// SetBaseURI sets the base URI for all tokens. Only the contract owner may call this.
// args[0] = uri
func (n *NFTCollection) SetBaseURI(ctx *blockchaincomponent.Context, uri string) {
	if ctx.CallerAddr != ctx.OwnerAddr {
		ctx.Revert("only owner can set base URI")
	}
	ctx.Set("uri:base", uri)
	ctx.Emit("BaseURISet", map[string]interface{}{
		"uri": uri,
	})
}

// REQUIRED EXPORT
var Contract = &NFTCollection{}
