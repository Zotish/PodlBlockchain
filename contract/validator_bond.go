//go:build ignore
// +build ignore

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"strconv"
	"strings"

	bc "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/BlockchainComponent"
)

// ValidatorBond is the native-LQD custody boundary for PoDL validators. Bonded
// principal cannot leave without the unbond delay and a pending evidence case
// can freeze withdrawal until governance adjudicates it.
type ValidatorBond struct{}

const validatorBondRevenueEscrow = "0x0000000000000000000000000000000000000e01"

func vbBig(raw string) *big.Int {
	z := new(big.Int)
	if _, ok := z.SetString(strings.TrimSpace(raw), 10); !ok {
		return big.NewInt(0)
	}
	return z
}

func vbAddr(raw string) string { return strings.ToLower(strings.TrimSpace(raw)) }

func (v *ValidatorBond) Init(ctx *bc.Context, unbondDelaySeconds string, slashChallengeSeconds string) {
	if ctx.Get("initialized") == "true" {
		ctx.Revert("already initialized")
	}
	unbond, err := strconv.ParseInt(unbondDelaySeconds, 10, 64)
	if err != nil || unbond < 86400 {
		ctx.Revert("unbond delay must be at least one day")
	}
	challenge, err := strconv.ParseInt(slashChallengeSeconds, 10, 64)
	if err != nil || challenge < 3600 {
		ctx.Revert("slash challenge must be at least one hour")
	}
	ctx.Set("initialized", "true")
	ctx.Set("unbond_delay", strconv.FormatInt(unbond, 10))
	ctx.Set("slash_challenge", strconv.FormatInt(challenge, 10))
	ctx.Set("total_bonded", "0")
	ctx.Emit("ValidatorBondInitialized", map[string]interface{}{"unbondDelay": unbond, "slashChallenge": challenge})
}

func (v *ValidatorBond) Bond(ctx *bc.Context) {
	amount := ctx.MsgValue()
	if amount.Sign() <= 0 {
		ctx.Revert("native bond amount required")
	}
	ctx.ReceiveNative(amount)
	validator := vbAddr(ctx.CallerAddr)
	ctx.Set("bond:"+validator, new(big.Int).Add(vbBig(ctx.Get("bond:"+validator)), amount).String())
	ctx.Set("total_bonded", new(big.Int).Add(vbBig(ctx.Get("total_bonded")), amount).String())
	ctx.Emit("ValidatorBonded", map[string]interface{}{"validator": validator, "amount": amount.String()})
}

func (v *ValidatorBond) RequestUnbond(ctx *bc.Context, amount string) {
	validator := vbAddr(ctx.CallerAddr)
	amt := vbBig(amount)
	if amt.Sign() <= 0 || vbBig(ctx.Get("bond:"+validator)).Cmp(amt) < 0 {
		ctx.Revert("invalid unbond amount")
	}
	if ctx.Get("pending_slash:"+validator) != "" {
		ctx.Revert("pending slash freezes unbonding")
	}
	unlock := ctx.BlockTime + vbBig(ctx.Get("unbond_delay")).Int64()
	ctx.Set("unbond_amount:"+validator, amt.String())
	ctx.Set("unbond_at:"+validator, strconv.FormatInt(unlock, 10))
	ctx.Emit("ValidatorUnbondRequested", map[string]interface{}{"validator": validator, "amount": amt.String(), "unlockAt": unlock})
}

func (v *ValidatorBond) CancelUnbond(ctx *bc.Context) {
	validator := vbAddr(ctx.CallerAddr)
	ctx.Set("unbond_amount:"+validator, "0")
	ctx.Set("unbond_at:"+validator, "0")
	ctx.Emit("ValidatorUnbondCancelled", map[string]interface{}{"validator": validator})
}

func (v *ValidatorBond) WithdrawUnbonded(ctx *bc.Context) {
	validator := vbAddr(ctx.CallerAddr)
	amt := vbBig(ctx.Get("unbond_amount:" + validator))
	unlock := vbBig(ctx.Get("unbond_at:" + validator)).Int64()
	if amt.Sign() <= 0 || ctx.BlockTime < unlock || ctx.Get("pending_slash:"+validator) != "" {
		ctx.Revert("unbond is not withdrawable")
	}
	bond := vbBig(ctx.Get("bond:" + validator))
	if bond.Cmp(amt) < 0 {
		ctx.Revert("bond changed below request")
	}
	ctx.Set("bond:"+validator, new(big.Int).Sub(bond, amt).String())
	ctx.Set("total_bonded", new(big.Int).Sub(vbBig(ctx.Get("total_bonded")), amt).String())
	ctx.Set("unbond_amount:"+validator, "0")
	ctx.Set("unbond_at:"+validator, "0")
	ctx.SendNative(validator, amt)
	ctx.Emit("ValidatorUnbonded", map[string]interface{}{"validator": validator, "amount": amt.String()})
}

func (v *ValidatorBond) OpenSlash(ctx *bc.Context, validator string, amount string, evidenceHash string) {
	if !strings.EqualFold(ctx.CallerAddr, ctx.OwnerAddr) {
		ctx.Revert("only governance manager")
	}
	validator, evidenceHash = vbAddr(validator), strings.TrimSpace(evidenceHash)
	amt := vbBig(amount)
	if amt.Sign() <= 0 || evidenceHash == "" || vbBig(ctx.Get("bond:"+validator)).Cmp(amt) < 0 {
		ctx.Revert("invalid slash case")
	}
	sum := sha256.Sum256([]byte(validator + "|" + evidenceHash))
	id := "slash_" + hex.EncodeToString(sum[:12])
	deadline := ctx.BlockTime + vbBig(ctx.Get("slash_challenge")).Int64()
	ctx.Set("pending_slash:"+validator, id)
	ctx.Set("slash:"+id+":validator", validator)
	ctx.Set("slash:"+id+":amount", amt.String())
	ctx.Set("slash:"+id+":evidence", evidenceHash)
	ctx.Set("slash:"+id+":deadline", strconv.FormatInt(deadline, 10))
	ctx.Set("slash:"+id+":status", "challengeable")
	ctx.Emit("ValidatorSlashOpened", map[string]interface{}{"id": id, "validator": validator, "amount": amt.String(), "challengeUntil": deadline})
}

func (v *ValidatorBond) AppealSlash(ctx *bc.Context, caseID string, appealHash string) {
	validator := vbAddr(ctx.CallerAddr)
	if ctx.Get("slash:"+caseID+":validator") != validator || ctx.Get("slash:"+caseID+":status") != "challengeable" || ctx.BlockTime > vbBig(ctx.Get("slash:"+caseID+":deadline")).Int64() || strings.TrimSpace(appealHash) == "" {
		ctx.Revert("case is not appealable")
	}
	ctx.Set("slash:"+caseID+":appeal", appealHash)
	ctx.Set("slash:"+caseID+":status", "appealed")
	ctx.Emit("ValidatorSlashAppealed", map[string]interface{}{"id": caseID, "validator": validator, "appeal": appealHash})
}

func (v *ValidatorBond) ResolveSlash(ctx *bc.Context, caseID string, uphold string, governanceProposal string) {
	if !strings.EqualFold(ctx.CallerAddr, ctx.OwnerAddr) || strings.TrimSpace(governanceProposal) == "" {
		ctx.Revert("governance authorization required")
	}
	status := ctx.Get("slash:" + caseID + ":status")
	if status != "challengeable" && status != "appealed" {
		ctx.Revert("slash case not pending")
	}
	if status == "challengeable" && ctx.BlockTime <= vbBig(ctx.Get("slash:"+caseID+":deadline")).Int64() {
		ctx.Revert("challenge window open")
	}
	validator := ctx.Get("slash:" + caseID + ":validator")
	if strings.EqualFold(uphold, "true") {
		amt := vbBig(ctx.Get("slash:" + caseID + ":amount"))
		bond := vbBig(ctx.Get("bond:" + validator))
		if bond.Cmp(amt) < 0 {
			amt = bond
		}
		ctx.Set("bond:"+validator, new(big.Int).Sub(bond, amt).String())
		ctx.Set("total_bonded", new(big.Int).Sub(vbBig(ctx.Get("total_bonded")), amt).String())
		ctx.Set("protocol_slash_total", new(big.Int).Add(vbBig(ctx.Get("protocol_slash_total")), amt).String())
		ctx.SendNative(validatorBondRevenueEscrow, amt)
		ctx.Set("slash:"+caseID+":status", "upheld")
	} else {
		ctx.Set("slash:"+caseID+":status", "dismissed")
	}
	ctx.Set("slash:"+caseID+":governance", governanceProposal)
	ctx.Set("pending_slash:"+validator, "")
	ctx.Emit("ValidatorSlashResolved", map[string]interface{}{"id": caseID, "validator": validator, "uphold": strings.EqualFold(uphold, "true"), "governance": governanceProposal})
}

func (v *ValidatorBond) BondOf(ctx *bc.Context, validator string) {
	ctx.Set("output", vbBig(ctx.Get("bond:"+vbAddr(validator))).String())
}
func (v *ValidatorBond) TotalBonded(ctx *bc.Context) {
	ctx.Set("output", vbBig(ctx.Get("total_bonded")).String())
}

var Contract = &ValidatorBond{}
