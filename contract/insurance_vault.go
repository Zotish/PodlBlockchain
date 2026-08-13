//go:build ignore
// +build ignore

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"strings"

	bc "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/BlockchainComponent"
)

type InsuranceVault struct{}

func ivBig(raw string) *big.Int {
	z := new(big.Int)
	if _, ok := z.SetString(strings.TrimSpace(raw), 10); !ok {
		return big.NewInt(0)
	}
	return z
}
func ivNorm(raw string) string { return strings.ToLower(strings.TrimSpace(raw)) }

func (v *InsuranceVault) Init(ctx *bc.Context, governance string, minimumReserve string, maxClaimBps string) {
	if ctx.Get("initialized") == "true" {
		ctx.Revert("already initialized")
	}
	governance = ivNorm(governance)
	minimum, maxClaim := ivBig(minimumReserve), ivBig(maxClaimBps)
	if governance == "" || minimum.Sign() < 0 || !maxClaim.IsInt64() || maxClaim.Int64() < 1 || maxClaim.Int64() > 2500 {
		ctx.Revert("invalid insurance policy")
	}
	ctx.Set("initialized", "true")
	ctx.Set("governance", governance)
	ctx.Set("minimum_reserve", minimum.String())
	ctx.Set("max_claim_bps", maxClaim.String())
	ctx.Set("reserve", "0")
	ctx.Set("reserved_claims", "0")
	ctx.Set("covered_liability", "0")
	ctx.Set("claims_paid", "0")
	ctx.Emit("InsuranceInitialized", map[string]interface{}{"governance": governance, "minimumReserve": minimum.String(), "maxClaimBps": maxClaim.String()})
}

func (v *InsuranceVault) DepositRevenue(ctx *bc.Context, source string, reference string) {
	amount := ctx.MsgValue()
	if amount.Sign() <= 0 || strings.TrimSpace(source) == "" || strings.TrimSpace(reference) == "" {
		ctx.Revert("custodied revenue and reference required")
	}
	ctx.ReceiveNative(amount)
	ctx.Set("reserve", new(big.Int).Add(ivBig(ctx.Get("reserve")), amount).String())
	ctx.Emit("InsuranceRevenueDeposited", map[string]interface{}{"source": source, "reference": reference, "amount": amount.String()})
}

func (v *InsuranceVault) OpenClaim(ctx *bc.Context, beneficiary string, amount string, lossProofHash string, challengeUntil string) {
	if !strings.EqualFold(ctx.CallerAddr, ctx.Get("governance")) {
		ctx.Revert("governance only")
	}
	beneficiary = ivNorm(beneficiary)
	amt, deadline := ivBig(amount), ivBig(challengeUntil)
	reserve := ivBig(ctx.Get("reserve"))
	reserved := ivBig(ctx.Get("reserved_claims"))
	available := new(big.Int).Sub(reserve, reserved)
	cap := new(big.Int).Div(new(big.Int).Mul(reserve, ivBig(ctx.Get("max_claim_bps"))), big.NewInt(10000))
	if beneficiary == "" || amt.Sign() <= 0 || amt.Cmp(cap) > 0 || strings.TrimSpace(lossProofHash) == "" || !deadline.IsInt64() || deadline.Int64() <= ctx.BlockTime {
		ctx.Revert("invalid or over-cap insurance claim")
	}
	if available.Cmp(amt) < 0 || new(big.Int).Sub(available, amt).Cmp(ivBig(ctx.Get("minimum_reserve"))) < 0 {
		ctx.Revert("claim reservation breaches available reserve floor")
	}
	sum := sha256.Sum256([]byte(beneficiary + "|" + amt.String() + "|" + lossProofHash))
	id := "claim_" + hex.EncodeToString(sum[:12])
	ctx.Set("claim:"+id+":beneficiary", beneficiary)
	ctx.Set("claim:"+id+":amount", amt.String())
	ctx.Set("claim:"+id+":proof", lossProofHash)
	ctx.Set("claim:"+id+":challenge_until", deadline.String())
	ctx.Set("claim:"+id+":status", "challengeable")
	ctx.Set("reserved_claims", new(big.Int).Add(reserved, amt).String())
	ctx.Set("output", id)
	ctx.Emit("InsuranceClaimOpened", map[string]interface{}{"id": id, "beneficiary": beneficiary, "amount": amt.String(), "challengeUntil": deadline.String()})
}

func (v *InsuranceVault) ChallengeClaim(ctx *bc.Context, id string, evidenceHash string) {
	if ctx.Get("claim:"+id+":status") != "challengeable" || ctx.BlockTime > ivBig(ctx.Get("claim:"+id+":challenge_until")).Int64() || strings.TrimSpace(evidenceHash) == "" {
		ctx.Revert("claim not challengeable")
	}
	ctx.Set("claim:"+id+":status", "challenged")
	ctx.Set("claim:"+id+":challenge", evidenceHash)
	ctx.Emit("InsuranceClaimChallenged", map[string]interface{}{"id": id, "challenger": ctx.CallerAddr, "evidence": evidenceHash})
}

func (v *InsuranceVault) ResolveClaim(ctx *bc.Context, id string, approve string, governanceProposal string) {
	if !strings.EqualFold(ctx.CallerAddr, ctx.Get("governance")) || strings.TrimSpace(governanceProposal) == "" {
		ctx.Revert("governance proposal required")
	}
	status := ctx.Get("claim:" + id + ":status")
	if status != "challengeable" && status != "challenged" {
		ctx.Revert("claim not pending")
	}
	if status == "challengeable" && ctx.BlockTime <= ivBig(ctx.Get("claim:"+id+":challenge_until")).Int64() {
		ctx.Revert("challenge window open")
	}
	if strings.EqualFold(approve, "true") {
		amt := ivBig(ctx.Get("claim:" + id + ":amount"))
		reserve := ivBig(ctx.Get("reserve"))
		minimum := ivBig(ctx.Get("minimum_reserve"))
		if new(big.Int).Sub(reserve, amt).Cmp(minimum) < 0 {
			ctx.Revert("insurance floor breach")
		}
		ctx.Set("reserve", new(big.Int).Sub(reserve, amt).String())
		ctx.Set("claims_paid", new(big.Int).Add(ivBig(ctx.Get("claims_paid")), amt).String())
		ctx.SendNative(ctx.Get("claim:"+id+":beneficiary"), amt)
		ctx.Set("claim:"+id+":status", "paid")
	} else {
		ctx.Set("claim:"+id+":status", "rejected")
	}
	ctx.Set("reserved_claims", new(big.Int).Sub(ivBig(ctx.Get("reserved_claims")), ivBig(ctx.Get("claim:"+id+":amount"))).String())
	ctx.Set("claim:"+id+":governance", governanceProposal)
	ctx.Emit("InsuranceClaimResolved", map[string]interface{}{"id": id, "approved": strings.EqualFold(approve, "true"), "governance": governanceProposal})
}

func (v *InsuranceVault) Reserve(ctx *bc.Context) {
	ctx.Set("output", ivBig(ctx.Get("reserve")).String())
}

func (v *InsuranceVault) SetCoveredLiability(ctx *bc.Context, amount string, governanceProposal string) {
	if !strings.EqualFold(ctx.CallerAddr, ctx.Get("governance")) || strings.TrimSpace(governanceProposal) == "" {
		ctx.Revert("governance proposal required")
	}
	liability := ivBig(amount)
	if liability.Sign() < 0 {
		ctx.Revert("invalid covered liability")
	}
	ctx.Set("covered_liability", liability.String())
	ctx.Emit("InsuranceLiabilityUpdated", map[string]interface{}{"liability": liability.String(), "governance": governanceProposal})
}
func (v *InsuranceVault) CoverageRatioBPS(ctx *bc.Context) {
	liability := ivBig(ctx.Get("covered_liability"))
	if liability.Sign() == 0 {
		ctx.Set("output", "0")
		return
	}
	ctx.Set("output", new(big.Int).Div(new(big.Int).Mul(ivBig(ctx.Get("reserve")), big.NewInt(10000)), liability).String())
}
func (v *InsuranceVault) AvailableReserve(ctx *bc.Context) {
	ctx.Set("output", new(big.Int).Sub(ivBig(ctx.Get("reserve")), ivBig(ctx.Get("reserved_claims"))).String())
}

var Contract = &InsuranceVault{}
