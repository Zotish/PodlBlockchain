//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"math/big"
	"strings"

	blockchaincomponent "github.com/Zotish/DefenceProject/BlockchainComponent"
)

type DAOTreasury struct{}

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

func (d *DAOTreasury) Init(ctx *blockchaincomponent.Context, name string) {
	if name == "" {
		name = "DAO Treasury"
	}
	ctx.Set("name", name)
	ctx.Set("treasury", "0")
	ctx.Set("proposal:count", "0")
	ctx.Emit("Init", map[string]interface{}{"name": name})
}

func (d *DAOTreasury) Name(ctx *blockchaincomponent.Context) {
	ctx.Set("output", ctx.Get("name"))
}

func (d *DAOTreasury) Deposit(ctx *blockchaincomponent.Context, amount string) {
	amt := parseBig(amount)
	if amt.Sign() == 0 {
		ctx.Revert("invalid deposit amount")
	}
	cur := parseBig(ctx.Get("treasury"))
	ctx.Set("treasury", new(big.Int).Add(cur, amt).String())
	ctx.Emit("Deposit", map[string]interface{}{
		"from":   ctx.CallerAddr,
		"amount": amt.String(),
	})
}

func (d *DAOTreasury) Withdraw(ctx *blockchaincomponent.Context, to string, amount string) {
	if ctx.CallerAddr != ctx.OwnerAddr {
		ctx.Revert("only owner can withdraw")
	}
	amt := parseBig(amount)
	if amt.Sign() == 0 {
		ctx.Revert("invalid withdraw amount")
	}
	cur := parseBig(ctx.Get("treasury"))
	if cur.Cmp(amt) < 0 {
		ctx.Revert("insufficient treasury")
	}
	ctx.Set("treasury", new(big.Int).Sub(cur, amt).String())
	ctx.Emit("Withdraw", map[string]interface{}{
		"to":     to,
		"amount": amt.String(),
	})
}

func (d *DAOTreasury) Treasury(ctx *blockchaincomponent.Context) {
	ctx.Set("output", ctx.Get("treasury"))
}

// CreateProposal creates a new governance proposal.
// args[0] = description, args[1] = target address, args[2] = amount
func (d *DAOTreasury) CreateProposal(ctx *blockchaincomponent.Context, description string, target string, amount string) {
	if description == "" {
		ctx.Revert("description required")
	}
	if target == "" {
		ctx.Revert("target address required")
	}
	amt := parseBig(amount)
	if amt.Sign() == 0 {
		ctx.Revert("invalid proposal amount")
	}

	// Increment proposal counter
	countStr := ctx.Get("proposal:count")
	if countStr == "" {
		countStr = "0"
	}
	count := parseBig(countStr)
	id := new(big.Int).Add(count, big.NewInt(1))
	idStr := id.String()
	ctx.Set("proposal:count", idStr)

	ctx.Set("proposal:"+idStr+":desc", description)
	ctx.Set("proposal:"+idStr+":target", target)
	ctx.Set("proposal:"+idStr+":amount", amt.String())
	ctx.Set("proposal:"+idStr+":yes", "0")
	ctx.Set("proposal:"+idStr+":no", "0")
	ctx.Set("proposal:"+idStr+":created", fmt.Sprintf("%d", ctx.BlockTime))
	ctx.Set("proposal:"+idStr+":executed", "false")
	ctx.Set("proposal:"+idStr+":proposer", ctx.CallerAddr)

	ctx.Set("output", idStr)
	ctx.Emit("ProposalCreated", map[string]interface{}{
		"id":          idStr,
		"proposer":    ctx.CallerAddr,
		"description": description,
		"target":      target,
		"amount":      amt.String(),
		"created":     ctx.BlockTime,
	})
}

// Vote casts a vote on a proposal.
// args[0] = proposalId, args[1] = "yes" or "no"
func (d *DAOTreasury) Vote(ctx *blockchaincomponent.Context, proposalId string, vote string) {
	proposalId = strings.TrimSpace(proposalId)
	vote = strings.TrimSpace(strings.ToLower(vote))

	if proposalId == "" {
		ctx.Revert("proposalId required")
	}
	if vote != "yes" && vote != "no" {
		ctx.Revert("vote must be 'yes' or 'no'")
	}

	// Check proposal exists
	desc := ctx.Get("proposal:" + proposalId + ":desc")
	if desc == "" {
		ctx.Revert("proposal does not exist")
	}

	// Check proposal has not been executed
	if ctx.Get("proposal:"+proposalId+":executed") == "true" {
		ctx.Revert("proposal already executed")
	}

	// Enforce 3-day voting window (259200 seconds)
	var createdAt int64
	fmt.Sscanf(ctx.Get("proposal:"+proposalId+":created"), "%d", &createdAt)
	votingPeriod := int64(3 * 24 * 3600) // 3 days
	if ctx.BlockTime > createdAt+votingPeriod {
		ctx.Revert("voting period has ended")
	}

	// Prevent double voting
	voteKey := "vote:" + proposalId + ":" + ctx.CallerAddr
	if ctx.Get(voteKey) != "" {
		ctx.Revert("already voted")
	}
	ctx.Set(voteKey, vote)

	// Tally the vote
	if vote == "yes" {
		yesCount := parseBig(ctx.Get("proposal:" + proposalId + ":yes"))
		ctx.Set("proposal:"+proposalId+":yes", new(big.Int).Add(yesCount, big.NewInt(1)).String())
	} else {
		noCount := parseBig(ctx.Get("proposal:" + proposalId + ":no"))
		ctx.Set("proposal:"+proposalId+":no", new(big.Int).Add(noCount, big.NewInt(1)).String())
	}

	ctx.Emit("VoteCast", map[string]interface{}{
		"proposalId": proposalId,
		"voter":      ctx.CallerAddr,
		"vote":       vote,
	})
}

// ExecuteProposal executes a passed proposal after the voting period ends.
// A proposal passes if it has >50% yes votes and the 3-day voting period has elapsed.
// args[0] = proposalId
func (d *DAOTreasury) ExecuteProposal(ctx *blockchaincomponent.Context, proposalId string) {
	proposalId = strings.TrimSpace(proposalId)
	if proposalId == "" {
		ctx.Revert("proposalId required")
	}

	// Check proposal exists
	target := ctx.Get("proposal:" + proposalId + ":target")
	if target == "" {
		ctx.Revert("proposal does not exist")
	}

	// Must not be already executed
	if ctx.Get("proposal:"+proposalId+":executed") == "true" {
		ctx.Revert("proposal already executed")
	}

	// Voting period must have ended
	var createdAt int64
	fmt.Sscanf(ctx.Get("proposal:"+proposalId+":created"), "%d", &createdAt)
	votingPeriod := int64(3 * 24 * 3600)
	if ctx.BlockTime < createdAt+votingPeriod {
		ctx.Revert("voting period not yet ended")
	}

	// Count votes and check >50% yes
	yesVotes := parseBig(ctx.Get("proposal:" + proposalId + ":yes"))
	noVotes := parseBig(ctx.Get("proposal:" + proposalId + ":no"))
	totalVotes := new(big.Int).Add(yesVotes, noVotes)

	if totalVotes.Sign() == 0 {
		ctx.Revert("no votes cast")
	}

	// yes must be strictly more than 50% of total
	// yes * 100 > total * 50  →  yes * 2 > total
	doubleYes := new(big.Int).Mul(yesVotes, big.NewInt(2))
	if doubleYes.Cmp(totalVotes) <= 0 {
		ctx.Revert("proposal did not pass (insufficient yes votes)")
	}

	// Check treasury has sufficient funds
	amount := parseBig(ctx.Get("proposal:" + proposalId + ":amount"))
	treasury := parseBig(ctx.Get("treasury"))
	if treasury.Cmp(amount) < 0 {
		ctx.Revert("insufficient treasury funds")
	}

	// Deduct from treasury and mark executed
	ctx.Set("treasury", new(big.Int).Sub(treasury, amount).String())
	ctx.Set("proposal:"+proposalId+":executed", "true")

	desc := ctx.Get("proposal:" + proposalId + ":desc")
	ctx.Set("output", fmt.Sprintf("executed proposal %s: %s -> %s amount=%s", proposalId, desc, target, amount.String()))
	ctx.Emit("ProposalExecuted", map[string]interface{}{
		"proposalId": proposalId,
		"target":     target,
		"amount":     amount.String(),
		"executor":   ctx.CallerAddr,
	})
}

// GetProposal returns proposal details for a given proposalId.
// args[0] = proposalId
// Output is set as a comma-separated string: desc,target,amount,yes,no,created,executed
func (d *DAOTreasury) GetProposal(ctx *blockchaincomponent.Context, proposalId string) {
	proposalId = strings.TrimSpace(proposalId)
	if proposalId == "" {
		ctx.Revert("proposalId required")
	}

	desc := ctx.Get("proposal:" + proposalId + ":desc")
	if desc == "" {
		ctx.Revert("proposal does not exist")
	}

	target := ctx.Get("proposal:" + proposalId + ":target")
	amount := ctx.Get("proposal:" + proposalId + ":amount")
	yes := ctx.Get("proposal:" + proposalId + ":yes")
	no := ctx.Get("proposal:" + proposalId + ":no")
	created := ctx.Get("proposal:" + proposalId + ":created")
	executed := ctx.Get("proposal:" + proposalId + ":executed")
	proposer := ctx.Get("proposal:" + proposalId + ":proposer")

	output := fmt.Sprintf(
		"id=%s desc=%s target=%s amount=%s yes=%s no=%s created=%s executed=%s proposer=%s",
		proposalId, desc, target, amount, yes, no, created, executed, proposer,
	)
	ctx.Set("output", output)
}

// GetVoteCount returns the yes and no vote counts for a proposal.
// args[0] = proposalId
// Output is set as: "yes=N no=M total=T"
func (d *DAOTreasury) GetVoteCount(ctx *blockchaincomponent.Context, proposalId string) {
	proposalId = strings.TrimSpace(proposalId)
	if proposalId == "" {
		ctx.Revert("proposalId required")
	}

	if ctx.Get("proposal:"+proposalId+":desc") == "" {
		ctx.Revert("proposal does not exist")
	}

	yes := ctx.Get("proposal:" + proposalId + ":yes")
	no := ctx.Get("proposal:" + proposalId + ":no")
	if yes == "" {
		yes = "0"
	}
	if no == "" {
		no = "0"
	}

	yesN := parseBig(yes)
	noN := parseBig(no)
	total := new(big.Int).Add(yesN, noN)

	ctx.Set("output", fmt.Sprintf("yes=%s no=%s total=%s", yes, no, total.String()))
}

// REQUIRED EXPORT
var Contract = &DAOTreasury{}
