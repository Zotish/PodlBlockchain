//go:build ignore
// +build ignore

package main

import (
	"math/big"
	"strings"

	bc "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/BlockchainComponent"
)

// LendingPool is one isolated collateral/debt market. Supplier and borrower
// positions use global indices so interest accrual is O(1), deterministic and
// independent of the number of users.
type LendingPool struct{}

var lendingScale = new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)

func lpBig(raw string) *big.Int {
	z := new(big.Int)
	if _, ok := z.SetString(strings.TrimSpace(raw), 10); !ok {
		return big.NewInt(0)
	}
	return z
}
func lpNorm(raw string) string { return strings.ToLower(strings.TrimSpace(raw)) }
func lpCeilDiv(n, d *big.Int) *big.Int {
	if n == nil || d == nil || d.Sign() <= 0 {
		return big.NewInt(0)
	}
	return new(big.Int).Div(new(big.Int).Add(n, new(big.Int).Sub(d, big.NewInt(1))), d)
}
func lpActor(ctx *bc.Context) string {
	if actor := lpNorm(ctx.OriginAddr); actor != "" {
		return actor
	}
	return lpNorm(ctx.CallerAddr)
}
func (l *LendingPool) manager(ctx *bc.Context) string {
	if manager := lpNorm(ctx.Get("manager")); manager != "" {
		return manager
	}
	return lpNorm(ctx.OwnerAddr)
}
func (l *LendingPool) requireManager(ctx *bc.Context) {
	actor := lpActor(ctx)
	if actor != l.manager(ctx) && lpNorm(ctx.CallerAddr) != l.manager(ctx) {
		ctx.Revert("market manager only")
	}
}
func (l *LendingPool) pull(ctx *bc.Context, token, from string, amount *big.Int) {
	if amount.Sign() <= 0 {
		ctx.Revert("positive token amount required")
	}
	result, err := ctx.Call(lpNorm(token), "TransferFrom", []string{lpNorm(from), ctx.ContractAddr, amount.String()})
	if err != nil || result == nil || !result.Success {
		ctx.Revert("token transfer into lending market failed")
	}
}
func (l *LendingPool) push(ctx *bc.Context, token, to string, amount *big.Int) {
	if amount.Sign() <= 0 {
		return
	}
	result, err := ctx.Call(lpNorm(token), "Transfer", []string{lpNorm(to), amount.String()})
	if err != nil || result == nil || !result.Success {
		ctx.Revert("token transfer from lending market failed")
	}
}

func (l *LendingPool) Init(ctx *bc.Context, token string) {
	if ctx.Get("initialized") == "true" {
		ctx.Revert("already initialized")
	}
	token = lpNorm(token)
	if token == "" {
		ctx.Revert("market token required")
	}
	ctx.Set("initialized", "true")
	ctx.Set("manager", lpNorm(ctx.OwnerAddr))
	ctx.Set("debt_token", token)
	ctx.Set("collateral_token", token)
	ctx.Set("ltv_bps", "5000")
	ctx.Set("liquidation_threshold_bps", "7500")
	ctx.Set("liquidation_bonus_bps", "500")
	ctx.Set("close_factor_bps", "5000")
	ctx.Set("reserve_factor_bps", "1000")
	ctx.Set("base_rate_bps", "200")
	ctx.Set("slope1_bps", "1000")
	ctx.Set("slope2_bps", "10000")
	ctx.Set("kink_bps", "8000")
	ctx.Set("oracle_max_age", "900")
	ctx.Set("borrow_index_x18", lendingScale.String())
	ctx.Set("supply_index_x18", lendingScale.String())
	ctx.Set("last_accrual", big.NewInt(ctx.BlockTime).String())
	ctx.Set("total_scaled_supply", "0")
	ctx.Set("total_scaled_debt", "0")
	ctx.Set("total_collateral", "0")
	ctx.Set("protocol_reserves", "0")
	ctx.Set("bad_debt", "0")
	ctx.Emit("LendingMarketInitialized", map[string]interface{}{"debtToken": token, "collateralToken": token})
}

func (l *LendingPool) ConfigureMarket(ctx *bc.Context, collateralToken string, debtToken string, ltvBPS string, liquidationThresholdBPS string, liquidationBonusBPS string, reserveFactorBPS string) {
	l.requireManager(ctx)
	if lpBig(ctx.Get("total_scaled_supply")).Sign() > 0 || lpBig(ctx.Get("total_scaled_debt")).Sign() > 0 || lpBig(ctx.Get("total_collateral")).Sign() > 0 {
		ctx.Revert("active market cannot be reconfigured")
	}
	collateralToken, debtToken = lpNorm(collateralToken), lpNorm(debtToken)
	ltv, threshold, bonus, reserve := lpBig(ltvBPS), lpBig(liquidationThresholdBPS), lpBig(liquidationBonusBPS), lpBig(reserveFactorBPS)
	if collateralToken == "" || debtToken == "" || !ltv.IsInt64() || !threshold.IsInt64() || !bonus.IsInt64() || !reserve.IsInt64() || ltv.Int64() < 1000 || ltv.Int64() > 8500 || threshold.Int64() <= ltv.Int64() || threshold.Int64() > 9500 || bonus.Int64() < 0 || bonus.Int64() > 1500 || reserve.Int64() < 0 || reserve.Int64() > 5000 {
		ctx.Revert("invalid isolated market risk policy")
	}
	ctx.Set("collateral_token", collateralToken)
	ctx.Set("debt_token", debtToken)
	ctx.Set("ltv_bps", ltv.String())
	ctx.Set("liquidation_threshold_bps", threshold.String())
	ctx.Set("liquidation_bonus_bps", bonus.String())
	ctx.Set("reserve_factor_bps", reserve.String())
	ctx.Emit("LendingMarketConfigured", map[string]interface{}{"collateralToken": collateralToken, "debtToken": debtToken, "ltvBPS": ltv.String(), "liquidationThresholdBPS": threshold.String()})
}

func (l *LendingPool) SetInterestModel(ctx *bc.Context, baseBPS string, slope1BPS string, slope2BPS string, kinkBPS string) {
	l.requireManager(ctx)
	l.accrue(ctx)
	base, s1, s2, kink := lpBig(baseBPS), lpBig(slope1BPS), lpBig(slope2BPS), lpBig(kinkBPS)
	if !base.IsInt64() || !s1.IsInt64() || !s2.IsInt64() || !kink.IsInt64() || base.Int64() < 0 || base.Int64() > 2000 || s1.Int64() < 0 || s1.Int64() > 5000 || s2.Int64() < 0 || s2.Int64() > 30000 || kink.Int64() < 5000 || kink.Int64() > 9500 {
		ctx.Revert("invalid interest model")
	}
	ctx.Set("base_rate_bps", base.String())
	ctx.Set("slope1_bps", s1.String())
	ctx.Set("slope2_bps", s2.String())
	ctx.Set("kink_bps", kink.String())
	ctx.Emit("InterestModelUpdated", map[string]interface{}{"baseBPS": base.String(), "slope1BPS": s1.String(), "slope2BPS": s2.String(), "kinkBPS": kink.String()})
}

func (l *LendingPool) SetOraclePrice(ctx *bc.Context, token string, priceUSD18 string, timestamp string) {
	l.requireManager(ctx)
	token, price, ts := lpNorm(token), lpBig(priceUSD18), lpBig(timestamp)
	if token == "" || price.Sign() <= 0 || !ts.IsInt64() || ts.Int64() > ctx.BlockTime+30 || ctx.BlockTime-ts.Int64() > lpBig(ctx.Get("oracle_max_age")).Int64() {
		ctx.Revert("invalid or stale oracle update")
	}
	ctx.Set("oracle_price18:"+token, price.String())
	ctx.Set("oracle_updated_at:"+token, ts.String())
	ctx.Emit("LendingOracleUpdated", map[string]interface{}{"token": token, "priceUSD18": price.String(), "timestamp": ts.String()})
}
func (l *LendingPool) price(ctx *bc.Context, token string) *big.Int {
	ts := lpBig(ctx.Get("oracle_updated_at:" + lpNorm(token)))
	if !ts.IsInt64() || ts.Int64() > ctx.BlockTime+30 || ctx.BlockTime-ts.Int64() > lpBig(ctx.Get("oracle_max_age")).Int64() {
		return big.NewInt(0)
	}
	return lpBig(ctx.Get("oracle_price18:" + lpNorm(token)))
}

func (l *LendingPool) totalSupply(ctx *bc.Context) *big.Int {
	return new(big.Int).Div(new(big.Int).Mul(lpBig(ctx.Get("total_scaled_supply")), lpBig(ctx.Get("supply_index_x18"))), lendingScale)
}
func (l *LendingPool) totalDebt(ctx *bc.Context) *big.Int {
	return new(big.Int).Div(new(big.Int).Mul(lpBig(ctx.Get("total_scaled_debt")), lpBig(ctx.Get("borrow_index_x18"))), lendingScale)
}
func (l *LendingPool) supplyOf(ctx *bc.Context, user string) *big.Int {
	return new(big.Int).Div(new(big.Int).Mul(lpBig(ctx.Get("scaled_supply:"+lpNorm(user))), lpBig(ctx.Get("supply_index_x18"))), lendingScale)
}
func (l *LendingPool) debtOf(ctx *bc.Context, user string) *big.Int {
	return lpCeilDiv(new(big.Int).Mul(lpBig(ctx.Get("scaled_debt:"+lpNorm(user))), lpBig(ctx.Get("borrow_index_x18"))), lendingScale)
}
func (l *LendingPool) utilizationBPS(ctx *bc.Context) int64 {
	supply, debt := l.totalSupply(ctx), l.totalDebt(ctx)
	if supply.Sign() <= 0 {
		return 0
	}
	u := new(big.Int).Div(new(big.Int).Mul(debt, big.NewInt(10000)), supply)
	if u.Cmp(big.NewInt(10000)) > 0 {
		return 10000
	}
	return u.Int64()
}
func (l *LendingPool) borrowRateBPS(ctx *bc.Context) int64 {
	u, base, s1, s2, kink := l.utilizationBPS(ctx), lpBig(ctx.Get("base_rate_bps")).Int64(), lpBig(ctx.Get("slope1_bps")).Int64(), lpBig(ctx.Get("slope2_bps")).Int64(), lpBig(ctx.Get("kink_bps")).Int64()
	if kink <= 0 {
		kink = 8000
	}
	if u <= kink {
		return base + s1*u/kink
	}
	return base + s1 + s2*(u-kink)/(10000-kink)
}
func (l *LendingPool) accrue(ctx *bc.Context) {
	last := lpBig(ctx.Get("last_accrual"))
	if !last.IsInt64() || ctx.BlockTime <= last.Int64() {
		return
	}
	elapsed, scaledDebt := ctx.BlockTime-last.Int64(), lpBig(ctx.Get("total_scaled_debt"))
	if scaledDebt.Sign() == 0 {
		ctx.Set("last_accrual", big.NewInt(ctx.BlockTime).String())
		return
	}
	borrowIndex, supplyIndex := lpBig(ctx.Get("borrow_index_x18")), lpBig(ctx.Get("supply_index_x18"))
	oldDebt := new(big.Int).Div(new(big.Int).Mul(scaledDebt, borrowIndex), lendingScale)
	annualDenom := big.NewInt(10000 * 365 * 24 * 3600)
	interestFactor := new(big.Int).Div(new(big.Int).Mul(big.NewInt(l.borrowRateBPS(ctx)), big.NewInt(elapsed)), annualDenom)
	// Preserve sub-wei rate precision by applying the rational directly.
	interest := new(big.Int).Div(new(big.Int).Mul(new(big.Int).Mul(oldDebt, big.NewInt(l.borrowRateBPS(ctx))), big.NewInt(elapsed)), annualDenom)
	_ = interestFactor
	if interest.Sign() > 0 {
		borrowIndex.Add(borrowIndex, new(big.Int).Div(new(big.Int).Mul(interest, lendingScale), oldDebt))
		reserve := new(big.Int).Div(new(big.Int).Mul(interest, lpBig(ctx.Get("reserve_factor_bps"))), big.NewInt(10000))
		supplierInterest := new(big.Int).Sub(interest, reserve)
		if scaledSupply := lpBig(ctx.Get("total_scaled_supply")); scaledSupply.Sign() > 0 {
			supplyIndex.Add(supplyIndex, new(big.Int).Div(new(big.Int).Mul(supplierInterest, lendingScale), scaledSupply))
		}
		ctx.Set("protocol_reserves", new(big.Int).Add(lpBig(ctx.Get("protocol_reserves")), reserve).String())
	}
	ctx.Set("borrow_index_x18", borrowIndex.String())
	ctx.Set("supply_index_x18", supplyIndex.String())
	ctx.Set("last_accrual", big.NewInt(ctx.BlockTime).String())
}

func (l *LendingPool) Deposit(ctx *bc.Context, amount string) { l.Supply(ctx, amount, lpActor(ctx)) }
func (l *LendingPool) Supply(ctx *bc.Context, amount string, receiver string) {
	l.accrue(ctx)
	actor, receiver, amt := lpActor(ctx), lpNorm(receiver), lpBig(amount)
	if receiver == "" || amt.Sign() <= 0 {
		ctx.Revert("invalid supply")
	}
	index := lpBig(ctx.Get("supply_index_x18"))
	scaled := new(big.Int).Div(new(big.Int).Mul(amt, lendingScale), index)
	if scaled.Sign() <= 0 {
		ctx.Revert("supply rounds to zero")
	}
	l.pull(ctx, ctx.Get("debt_token"), actor, amt)
	ctx.Set("scaled_supply:"+receiver, new(big.Int).Add(lpBig(ctx.Get("scaled_supply:"+receiver)), scaled).String())
	ctx.Set("total_scaled_supply", new(big.Int).Add(lpBig(ctx.Get("total_scaled_supply")), scaled).String())
	ctx.Emit("Supply", map[string]interface{}{"sender": actor, "receiver": receiver, "assets": amt.String(), "scaledShares": scaled.String()})
}
func (l *LendingPool) Withdraw(ctx *bc.Context, amount string) {
	l.WithdrawSupply(ctx, amount, lpActor(ctx))
}
func (l *LendingPool) WithdrawSupply(ctx *bc.Context, amount string, receiver string) {
	l.accrue(ctx)
	actor, receiver, amt := lpActor(ctx), lpNorm(receiver), lpBig(amount)
	if receiver == "" || amt.Sign() <= 0 || l.supplyOf(ctx, actor).Cmp(amt) < 0 {
		ctx.Revert("insufficient supply")
	}
	available := new(big.Int).Sub(l.totalSupply(ctx), l.totalDebt(ctx))
	available.Sub(available, lpBig(ctx.Get("protocol_reserves")))
	if available.Cmp(amt) < 0 {
		ctx.Revert("insufficient market liquidity")
	}
	index := lpBig(ctx.Get("supply_index_x18"))
	scaled := lpCeilDiv(new(big.Int).Mul(amt, lendingScale), index)
	owned := lpBig(ctx.Get("scaled_supply:" + actor))
	if scaled.Cmp(owned) > 0 {
		scaled.Set(owned)
	}
	ctx.Set("scaled_supply:"+actor, new(big.Int).Sub(owned, scaled).String())
	ctx.Set("total_scaled_supply", new(big.Int).Sub(lpBig(ctx.Get("total_scaled_supply")), scaled).String())
	l.push(ctx, ctx.Get("debt_token"), receiver, amt)
	ctx.Emit("WithdrawSupply", map[string]interface{}{"owner": actor, "receiver": receiver, "assets": amt.String(), "scaledShares": scaled.String()})
}

func (l *LendingPool) DepositCollateral(ctx *bc.Context, amount string, receiver string) {
	actor, receiver, amt := lpActor(ctx), lpNorm(receiver), lpBig(amount)
	if receiver == "" || amt.Sign() <= 0 {
		ctx.Revert("invalid collateral deposit")
	}
	l.pull(ctx, ctx.Get("collateral_token"), actor, amt)
	ctx.Set("collateral:"+receiver, new(big.Int).Add(lpBig(ctx.Get("collateral:"+receiver)), amt).String())
	ctx.Set("total_collateral", new(big.Int).Add(lpBig(ctx.Get("total_collateral")), amt).String())
	ctx.Emit("CollateralDeposited", map[string]interface{}{"sender": actor, "owner": receiver, "amount": amt.String()})
}
func (l *LendingPool) collateralValue(ctx *bc.Context, amount *big.Int) *big.Int {
	return new(big.Int).Div(new(big.Int).Mul(amount, l.price(ctx, ctx.Get("collateral_token"))), lendingScale)
}
func (l *LendingPool) debtValue(ctx *bc.Context, amount *big.Int) *big.Int {
	return new(big.Int).Div(new(big.Int).Mul(amount, l.price(ctx, ctx.Get("debt_token"))), lendingScale)
}
func (l *LendingPool) healthFactorBPS(ctx *bc.Context, user string) *big.Int {
	debt := l.debtOf(ctx, user)
	if debt.Sign() == 0 {
		return big.NewInt(1_000_000_000)
	}
	collateralPrice, debtPrice := l.price(ctx, ctx.Get("collateral_token")), l.price(ctx, ctx.Get("debt_token"))
	if collateralPrice.Sign() <= 0 || debtPrice.Sign() <= 0 {
		return big.NewInt(0)
	}
	adjustedCollateral := new(big.Int).Div(new(big.Int).Mul(l.collateralValue(ctx, lpBig(ctx.Get("collateral:"+lpNorm(user)))), lpBig(ctx.Get("liquidation_threshold_bps"))), big.NewInt(10000))
	return new(big.Int).Div(new(big.Int).Mul(adjustedCollateral, big.NewInt(10000)), l.debtValue(ctx, debt))
}
func (l *LendingPool) WithdrawCollateral(ctx *bc.Context, amount string, receiver string) {
	l.accrue(ctx)
	actor, receiver, amt := lpActor(ctx), lpNorm(receiver), lpBig(amount)
	collateral := lpBig(ctx.Get("collateral:" + actor))
	if receiver == "" || amt.Sign() <= 0 || collateral.Cmp(amt) < 0 {
		ctx.Revert("insufficient collateral")
	}
	ctx.Set("collateral:"+actor, new(big.Int).Sub(collateral, amt).String())
	if l.healthFactorBPS(ctx, actor).Cmp(big.NewInt(10000)) < 0 {
		ctx.Revert("withdrawal would make position liquidatable")
	}
	ctx.Set("total_collateral", new(big.Int).Sub(lpBig(ctx.Get("total_collateral")), amt).String())
	l.push(ctx, ctx.Get("collateral_token"), receiver, amt)
	ctx.Emit("CollateralWithdrawn", map[string]interface{}{"owner": actor, "receiver": receiver, "amount": amt.String()})
}

func (l *LendingPool) Borrow(ctx *bc.Context, amount string) {
	l.accrue(ctx)
	borrower, amt := lpActor(ctx), lpBig(amount)
	if amt.Sign() <= 0 {
		ctx.Revert("invalid borrow")
	}
	if l.price(ctx, ctx.Get("collateral_token")).Sign() <= 0 || l.price(ctx, ctx.Get("debt_token")).Sign() <= 0 {
		ctx.Revert("fresh collateral and debt oracle prices required")
	}
	newDebt := new(big.Int).Add(l.debtOf(ctx, borrower), amt)
	maxDebtValue := new(big.Int).Div(new(big.Int).Mul(l.collateralValue(ctx, lpBig(ctx.Get("collateral:"+borrower))), lpBig(ctx.Get("ltv_bps"))), big.NewInt(10000))
	if l.debtValue(ctx, newDebt).Cmp(maxDebtValue) > 0 {
		ctx.Revert("borrow limit exceeded")
	}
	available := new(big.Int).Sub(l.totalSupply(ctx), l.totalDebt(ctx))
	available.Sub(available, lpBig(ctx.Get("protocol_reserves")))
	if available.Cmp(amt) < 0 {
		ctx.Revert("insufficient market liquidity")
	}
	shares := lpCeilDiv(new(big.Int).Mul(amt, lendingScale), lpBig(ctx.Get("borrow_index_x18")))
	ctx.Set("scaled_debt:"+borrower, new(big.Int).Add(lpBig(ctx.Get("scaled_debt:"+borrower)), shares).String())
	ctx.Set("total_scaled_debt", new(big.Int).Add(lpBig(ctx.Get("total_scaled_debt")), shares).String())
	l.push(ctx, ctx.Get("debt_token"), borrower, amt)
	ctx.Emit("Borrow", map[string]interface{}{"borrower": borrower, "amount": amt.String(), "debtShares": shares.String()})
}
func (l *LendingPool) repayFor(ctx *bc.Context, payer, borrower string, amount *big.Int) *big.Int {
	l.accrue(ctx)
	debt, shares := l.debtOf(ctx, borrower), lpBig(ctx.Get("scaled_debt:"+borrower))
	if debt.Sign() <= 0 {
		ctx.Revert("no debt")
	}
	if amount.Cmp(debt) > 0 {
		amount = debt
	}
	l.pull(ctx, ctx.Get("debt_token"), payer, amount)
	remove := new(big.Int).Div(new(big.Int).Mul(amount, lendingScale), lpBig(ctx.Get("borrow_index_x18")))
	if amount.Cmp(debt) == 0 || remove.Cmp(shares) > 0 {
		remove.Set(shares)
	}
	if remove.Sign() <= 0 {
		ctx.Revert("repayment rounds to zero")
	}
	ctx.Set("scaled_debt:"+borrower, new(big.Int).Sub(shares, remove).String())
	ctx.Set("total_scaled_debt", new(big.Int).Sub(lpBig(ctx.Get("total_scaled_debt")), remove).String())
	return amount
}
func (l *LendingPool) Repay(ctx *bc.Context, amount string) {
	borrower := lpActor(ctx)
	paid := l.repayFor(ctx, borrower, borrower, lpBig(amount))
	ctx.Emit("Repay", map[string]interface{}{"payer": borrower, "borrower": borrower, "amount": paid.String()})
}
func (l *LendingPool) RepayFor(ctx *bc.Context, borrower string, amount string) {
	payer := lpActor(ctx)
	paid := l.repayFor(ctx, payer, lpNorm(borrower), lpBig(amount))
	ctx.Emit("Repay", map[string]interface{}{"payer": payer, "borrower": lpNorm(borrower), "amount": paid.String()})
}

func (l *LendingPool) LiquidatePartial(ctx *bc.Context, borrower string, repayAmount string, minCollateralOut string) {
	l.accrue(ctx)
	liquidator, borrower := lpActor(ctx), lpNorm(borrower)
	requested, minimum := lpBig(repayAmount), lpBig(minCollateralOut)
	if borrower == "" || borrower == liquidator || requested.Sign() <= 0 || l.healthFactorBPS(ctx, borrower).Cmp(big.NewInt(10000)) >= 0 {
		ctx.Revert("position is not liquidatable")
	}
	debt := l.debtOf(ctx, borrower)
	maxClose := new(big.Int).Div(new(big.Int).Mul(debt, lpBig(ctx.Get("close_factor_bps"))), big.NewInt(10000))
	if requested.Cmp(maxClose) > 0 {
		requested.Set(maxClose)
	}
	debtPrice, collateralPrice := l.price(ctx, ctx.Get("debt_token")), l.price(ctx, ctx.Get("collateral_token"))
	if debtPrice.Sign() <= 0 || collateralPrice.Sign() <= 0 {
		ctx.Revert("fresh oracle required")
	}
	seize := lpCeilDiv(new(big.Int).Mul(new(big.Int).Mul(requested, debtPrice), new(big.Int).Add(big.NewInt(10000), lpBig(ctx.Get("liquidation_bonus_bps")))), new(big.Int).Mul(collateralPrice, big.NewInt(10000)))
	collateral := lpBig(ctx.Get("collateral:" + borrower))
	if seize.Cmp(collateral) > 0 {
		seize.Set(collateral)
		requested.Div(new(big.Int).Mul(new(big.Int).Mul(seize, collateralPrice), big.NewInt(10000)), new(big.Int).Mul(debtPrice, new(big.Int).Add(big.NewInt(10000), lpBig(ctx.Get("liquidation_bonus_bps")))))
	}
	if seize.Sign() <= 0 || seize.Cmp(minimum) < 0 {
		ctx.Revert("liquidation collateral output below minimum")
	}
	paid := l.repayFor(ctx, liquidator, borrower, requested)
	ctx.Set("collateral:"+borrower, new(big.Int).Sub(collateral, seize).String())
	ctx.Set("total_collateral", new(big.Int).Sub(lpBig(ctx.Get("total_collateral")), seize).String())
	l.push(ctx, ctx.Get("collateral_token"), liquidator, seize)
	remainingDebt := l.debtOf(ctx, borrower)
	if new(big.Int).Sub(collateral, seize).Sign() == 0 && remainingDebt.Sign() > 0 {
		badShares := lpBig(ctx.Get("scaled_debt:" + borrower))
		ctx.Set("scaled_debt:"+borrower, "0")
		ctx.Set("total_scaled_debt", new(big.Int).Sub(lpBig(ctx.Get("total_scaled_debt")), badShares).String())
		ctx.Set("bad_debt", new(big.Int).Add(lpBig(ctx.Get("bad_debt")), remainingDebt).String())
	}
	ctx.Emit("Liquidation", map[string]interface{}{"liquidator": liquidator, "borrower": borrower, "repaid": paid.String(), "collateralSeized": seize.String(), "badDebt": ctx.Get("bad_debt")})
}
func (l *LendingPool) Liquidate(ctx *bc.Context, borrower string) {
	debt := l.debtOf(ctx, lpNorm(borrower))
	amount := new(big.Int).Div(new(big.Int).Mul(debt, lpBig(ctx.Get("close_factor_bps"))), big.NewInt(10000))
	l.LiquidatePartial(ctx, borrower, amount.String(), "0")
}

func (l *LendingPool) CoverBadDebt(ctx *bc.Context, amount string) {
	l.requireManager(ctx)
	amt, bad := lpBig(amount), lpBig(ctx.Get("bad_debt"))
	if amt.Sign() <= 0 || amt.Cmp(bad) > 0 {
		ctx.Revert("invalid bad-debt coverage")
	}
	l.pull(ctx, ctx.Get("debt_token"), lpActor(ctx), amt)
	ctx.Set("bad_debt", new(big.Int).Sub(bad, amt).String())
	ctx.Emit("BadDebtCovered", map[string]interface{}{"amount": amt.String(), "remaining": ctx.Get("bad_debt")})
}
func (l *LendingPool) WithdrawProtocolReserves(ctx *bc.Context, amount string, recipient string) {
	l.requireManager(ctx)
	l.accrue(ctx)
	amt, reserves := lpBig(amount), lpBig(ctx.Get("protocol_reserves"))
	if lpNorm(recipient) == "" || amt.Sign() <= 0 || reserves.Cmp(amt) < 0 {
		ctx.Revert("invalid reserve withdrawal")
	}
	available := new(big.Int).Sub(l.totalSupply(ctx), l.totalDebt(ctx))
	if available.Cmp(amt) < 0 {
		ctx.Revert("reserve withdrawal exceeds cash")
	}
	ctx.Set("protocol_reserves", new(big.Int).Sub(reserves, amt).String())
	l.push(ctx, ctx.Get("debt_token"), recipient, amt)
	ctx.Emit("ProtocolReservesWithdrawn", map[string]interface{}{"recipient": lpNorm(recipient), "amount": amt.String()})
}

func (l *LendingPool) AccrueInterest(ctx *bc.Context, addr string) {
	before := l.debtOf(ctx, addr)
	l.accrue(ctx)
	after := l.debtOf(ctx, addr)
	ctx.Set("output", new(big.Int).Sub(after, before).String())
}
func (l *LendingPool) BalanceOf(ctx *bc.Context, addr string) {
	l.accrue(ctx)
	ctx.Set("output", l.supplyOf(ctx, addr).String())
}
func (l *LendingPool) DebtOf(ctx *bc.Context, addr string) {
	l.accrue(ctx)
	ctx.Set("output", l.debtOf(ctx, addr).String())
}
func (l *LendingPool) GetDebt(ctx *bc.Context, addr string) { l.DebtOf(ctx, addr) }
func (l *LendingPool) CollateralOf(ctx *bc.Context, addr string) {
	ctx.Set("output", lpBig(ctx.Get("collateral:"+lpNorm(addr))).String())
}
func (l *LendingPool) HealthFactor(ctx *bc.Context, addr string) {
	l.accrue(ctx)
	ctx.Set("output", l.healthFactorBPS(ctx, lpNorm(addr)).String())
}
func (l *LendingPool) TotalDeposits(ctx *bc.Context) {
	l.accrue(ctx)
	ctx.Set("output", l.totalSupply(ctx).String())
}
func (l *LendingPool) TotalBorrows(ctx *bc.Context) {
	l.accrue(ctx)
	ctx.Set("output", l.totalDebt(ctx).String())
}
func (l *LendingPool) Utilization(ctx *bc.Context) {
	l.accrue(ctx)
	ctx.Set("output", big.NewInt(l.utilizationBPS(ctx)).String())
}
func (l *LendingPool) BorrowRate(ctx *bc.Context) {
	l.accrue(ctx)
	ctx.Set("output", big.NewInt(l.borrowRateBPS(ctx)).String())
}

var Contract = &LendingPool{}
