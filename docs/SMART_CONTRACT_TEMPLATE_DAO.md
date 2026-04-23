# PoDL DAO Smart Contract Starter Template

This is a minimal starter template for a DAO / treasury governance contract on PoDL.

It matches the contract style used by the chain:
- `package main`
- exported `var Contract = &YourType{}`
- `*Context` as the first argument
- string-based storage

## What this template covers

- treasury balance
- proposal creation
- yes/no voting
- execution after a voting window
- proposal query helpers

## Starter DAO template

```go
package main

import (
	"fmt"
	"math/big"
	"strings"

	lqdctx "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/lqd-sdk-compat/context"
)

type StarterDAO struct{}

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

func (d *StarterDAO) Init(ctx *lqdctx.Context, name string) {
	if name == "" {
		name = "Starter DAO"
	}
	ctx.Set("name", name)
	ctx.Set("treasury", "0")
	ctx.Set("proposal:count", "0")
}

func (d *StarterDAO) Name(ctx *lqdctx.Context) {
	ctx.Set("output", ctx.Get("name"))
}

func (d *StarterDAO) Deposit(ctx *lqdctx.Context, amount string) {
	amt := parseBig(amount)
	if amt.Sign() <= 0 {
		ctx.Revert("invalid deposit amount")
	}
	cur := parseBig(ctx.Get("treasury"))
	ctx.Set("treasury", new(big.Int).Add(cur, amt).String())
	ctx.Emit("Deposit", map[string]any{
		"from":   ctx.CallerAddr,
		"amount": amt.String(),
	})
}

func (d *StarterDAO) Treasury(ctx *lqdctx.Context) {
	ctx.Set("output", ctx.Get("treasury"))
}

func (d *StarterDAO) CreateProposal(ctx *lqdctx.Context, description string, target string, amount string) {
	if description == "" {
		ctx.Revert("description required")
	}
	if target == "" {
		ctx.Revert("target required")
	}

	amt := parseBig(amount)
	if amt.Sign() <= 0 {
		ctx.Revert("invalid amount")
	}

	count := parseBig(ctx.Get("proposal:count"))
	id := new(big.Int).Add(count, big.NewInt(1)).String()

	ctx.Set("proposal:count", id)
	ctx.Set("proposal:"+id+":desc", description)
	ctx.Set("proposal:"+id+":target", target)
	ctx.Set("proposal:"+id+":amount", amt.String())
	ctx.Set("proposal:"+id+":yes", "0")
	ctx.Set("proposal:"+id+":no", "0")
	ctx.Set("proposal:"+id+":executed", "false")
	ctx.Set("proposal:"+id+":created", fmt.Sprintf("%d", ctx.BlockTime))
	ctx.Set("proposal:"+id+":proposer", ctx.CallerAddr)
	ctx.Set("output", id)
}

func (d *StarterDAO) Vote(ctx *lqdctx.Context, proposalID string, vote string) {
	proposalID = strings.TrimSpace(proposalID)
	vote = strings.ToLower(strings.TrimSpace(vote))
	if proposalID == "" {
		ctx.Revert("proposalID required")
	}
	if vote != "yes" && vote != "no" {
		ctx.Revert("vote must be yes or no")
	}
	if ctx.Get("proposal:"+proposalID+":desc") == "" {
		ctx.Revert("proposal does not exist")
	}
	if ctx.Get("proposal:"+proposalID+":executed") == "true" {
		ctx.Revert("proposal already executed")
	}

	voteKey := "vote:" + proposalID + ":" + ctx.CallerAddr
	if ctx.Get(voteKey) != "" {
		ctx.Revert("already voted")
	}
	ctx.Set(voteKey, vote)

	if vote == "yes" {
		yes := parseBig(ctx.Get("proposal:" + proposalID + ":yes"))
		ctx.Set("proposal:"+proposalID+":yes", new(big.Int).Add(yes, big.NewInt(1)).String())
	} else {
		no := parseBig(ctx.Get("proposal:" + proposalID + ":no"))
		ctx.Set("proposal:"+proposalID+":no", new(big.Int).Add(no, big.NewInt(1)).String())
	}
}

func (d *StarterDAO) ExecuteProposal(ctx *lqdctx.Context, proposalID string) {
	proposalID = strings.TrimSpace(proposalID)
	if proposalID == "" {
		ctx.Revert("proposalID required")
	}

	if ctx.Get("proposal:"+proposalID+":desc") == "" {
		ctx.Revert("proposal does not exist")
	}
	if ctx.Get("proposal:"+proposalID+":executed") == "true" {
		ctx.Revert("proposal already executed")
	}

	created := parseBig(ctx.Get("proposal:" + proposalID + ":created"))
	if ctx.BlockTime < created.Int64()+3*24*3600 {
		ctx.Revert("voting period not ended")
	}

	yes := parseBig(ctx.Get("proposal:" + proposalID + ":yes"))
	no := parseBig(ctx.Get("proposal:" + proposalID + ":no"))
	total := new(big.Int).Add(yes, no)
	if total.Sign() == 0 {
		ctx.Revert("no votes cast")
	}
	if new(big.Int).Mul(yes, big.NewInt(2)).Cmp(total) <= 0 {
		ctx.Revert("proposal did not pass")
	}

	amount := parseBig(ctx.Get("proposal:" + proposalID + ":amount"))
	treasury := parseBig(ctx.Get("treasury"))
	if treasury.Cmp(amount) < 0 {
		ctx.Revert("insufficient treasury")
	}

	ctx.Set("treasury", new(big.Int).Sub(treasury, amount).String())
	ctx.Set("proposal:"+proposalID+":executed", "true")
	ctx.Emit("ProposalExecuted", map[string]any{
		"proposalId": proposalID,
		"target":     ctx.Get("proposal:" + proposalID + ":target"),
		"amount":     amount.String(),
	})
}

func (d *StarterDAO) GetProposal(ctx *lqdctx.Context, proposalID string) {
	proposalID = strings.TrimSpace(proposalID)
	if proposalID == "" {
		ctx.Revert("proposalID required")
	}
	if ctx.Get("proposal:"+proposalID+":desc") == "" {
		ctx.Revert("proposal does not exist")
	}

	output := fmt.Sprintf(
		"id=%s desc=%s target=%s amount=%s yes=%s no=%s executed=%s created=%s proposer=%s",
		proposalID,
		ctx.Get("proposal:"+proposalID+":desc"),
		ctx.Get("proposal:"+proposalID+":target"),
		ctx.Get("proposal:"+proposalID+":amount"),
		ctx.Get("proposal:"+proposalID+":yes"),
		ctx.Get("proposal:"+proposalID+":no"),
		ctx.Get("proposal:"+proposalID+":executed"),
		ctx.Get("proposal:"+proposalID+":created"),
		ctx.Get("proposal:"+proposalID+":proposer"),
	)
	ctx.Set("output", output)
}

var Contract = &StarterDAO{}
```

## Usage notes

- Keep treasury math in raw integer strings.
- Make sure voting windows and quorum rules match your governance policy.
- If you want token-weighted voting, add a balance snapshot or staking lookup before `Vote`.
- If you want on-chain execution of proposals, wire `ExecuteProposal` to the target contract call flow you want.

## Recommended next steps

- Add proposal cancellation or expiration.
- Add token-weighted or LP-weighted voting.
- Add quorum threshold and execution delay settings.
- Add timelock support for sensitive actions.
- Add emergency pause / guardian controls if needed.
