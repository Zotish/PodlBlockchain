package blockchaincomponent

import (
	"fmt"
	"math"
	"math/big"
	"sort"
)

var ammScaleX18 = new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)

// StableSwapInvariant implements the two-asset amplified invariant used for
// low-slippage correlated-asset pools. All arithmetic is deterministic integer
// arithmetic and converges within 255 iterations.
func StableSwapInvariant(x, y *big.Int, amplification int64) (*big.Int, error) {
	if x == nil || y == nil || x.Sign() <= 0 || y.Sign() <= 0 || amplification < 1 || amplification > 1_000_000 {
		return nil, fmt.Errorf("positive balances and amplification 1..1000000 required")
	}
	sum := new(big.Int).Add(x, y)
	d := new(big.Int).Set(sum)
	ann := big.NewInt(amplification * 2)
	for i := 0; i < 255; i++ {
		previous := new(big.Int).Set(d)
		dp := new(big.Int).Div(new(big.Int).Mul(d, d), new(big.Int).Mul(x, big.NewInt(2)))
		dp.Div(new(big.Int).Mul(dp, d), new(big.Int).Mul(y, big.NewInt(2)))
		numerator := new(big.Int).Mul(new(big.Int).Add(new(big.Int).Mul(ann, sum), new(big.Int).Mul(dp, big.NewInt(2))), d)
		denominator := new(big.Int).Add(new(big.Int).Mul(new(big.Int).Sub(ann, big.NewInt(1)), d), new(big.Int).Mul(dp, big.NewInt(3)))
		if denominator.Sign() == 0 {
			return nil, fmt.Errorf("stable invariant denominator is zero")
		}
		d.Div(numerator, denominator)
		delta := new(big.Int).Sub(d, previous)
		if delta.Sign() < 0 {
			delta.Neg(delta)
		}
		if delta.Cmp(big.NewInt(1)) <= 0 {
			return d, nil
		}
	}
	return nil, fmt.Errorf("stable invariant did not converge")
}

func stableGetY(x, d *big.Int, amplification int64) (*big.Int, error) {
	ann := big.NewInt(amplification * 2)
	c := new(big.Int).Div(new(big.Int).Mul(d, d), new(big.Int).Mul(x, big.NewInt(2)))
	c.Div(new(big.Int).Mul(c, d), new(big.Int).Mul(ann, big.NewInt(2)))
	b := new(big.Int).Add(x, new(big.Int).Div(d, ann))
	y := new(big.Int).Set(d)
	for i := 0; i < 255; i++ {
		previous := new(big.Int).Set(y)
		numerator := new(big.Int).Add(new(big.Int).Mul(y, y), c)
		denominator := new(big.Int).Add(new(big.Int).Mul(y, big.NewInt(2)), new(big.Int).Sub(b, d))
		if denominator.Sign() <= 0 {
			return nil, fmt.Errorf("stable quote denominator is invalid")
		}
		y.Div(numerator, denominator)
		delta := new(big.Int).Sub(y, previous)
		if delta.Sign() < 0 {
			delta.Neg(delta)
		}
		if delta.Cmp(big.NewInt(1)) <= 0 {
			return y, nil
		}
	}
	return nil, fmt.Errorf("stable quote did not converge")
}

func StableSwapAmountOut(amountIn, reserveIn, reserveOut *big.Int, amplification, feeBPS int64) (*big.Int, error) {
	if amountIn == nil || amountIn.Sign() <= 0 || feeBPS < 0 || feeBPS > 1000 {
		return nil, fmt.Errorf("invalid stable swap input or fee")
	}
	d, err := StableSwapInvariant(reserveIn, reserveOut, amplification)
	if err != nil {
		return nil, err
	}
	net := new(big.Int).Div(new(big.Int).Mul(amountIn, big.NewInt(10000-feeBPS)), big.NewInt(10000))
	y, err := stableGetY(new(big.Int).Add(reserveIn, net), d, amplification)
	if err != nil {
		return nil, err
	}
	out := new(big.Int).Sub(reserveOut, y)
	if out.Sign() > 0 {
		out.Sub(out, big.NewInt(1)) // conservative rounding protects the pool
	}
	if out.Sign() <= 0 || out.Cmp(reserveOut) >= 0 {
		return nil, fmt.Errorf("stable swap has no safe output")
	}
	return out, nil
}

type ConcentratedTick struct {
	SqrtPriceX18 *big.Int `json:"sqrt_price_x18"`
	LiquidityNet *big.Int `json:"liquidity_net"`
}

type ConcentratedPosition struct {
	ID           string   `json:"id"`
	Owner        string   `json:"owner"`
	LowerSqrtX18 *big.Int `json:"lower_sqrt_x18"`
	UpperSqrtX18 *big.Int `json:"upper_sqrt_x18"`
	Liquidity    *big.Int `json:"liquidity"`
	TokensOwed0  *big.Int `json:"tokens_owed0"`
	TokensOwed1  *big.Int `json:"tokens_owed1"`
}

// ConcentratedPool is a deterministic tick-crossing engine. LiquidityNet is
// added when price rises across a tick and subtracted when price falls.
type ConcentratedPool struct {
	SqrtPriceX18  *big.Int                         `json:"sqrt_price_x18"`
	Liquidity     *big.Int                         `json:"liquidity"`
	FeeBPS        int64                            `json:"fee_bps"`
	Ticks         []ConcentratedTick               `json:"ticks"`
	Positions     map[string]*ConcentratedPosition `json:"positions,omitempty"`
	FeeRemainder0 *big.Int                         `json:"fee_remainder0,omitempty"`
	FeeRemainder1 *big.Int                         `json:"fee_remainder1,omitempty"`
}

func (p *ConcentratedPool) ensurePositions() {
	if p.Positions == nil {
		p.Positions = make(map[string]*ConcentratedPosition)
	}
	if p.FeeRemainder0 == nil {
		p.FeeRemainder0 = big.NewInt(0)
	}
	if p.FeeRemainder1 == nil {
		p.FeeRemainder1 = big.NewInt(0)
	}
}

func (p *ConcentratedPool) AddPosition(id, owner string, lowerSqrtX18, upperSqrtX18, liquidity *big.Int) error {
	p.ensurePositions()
	if id == "" || owner == "" || p.Positions[id] != nil {
		return fmt.Errorf("unique position and owner required")
	}
	if err := p.AddRange(lowerSqrtX18, upperSqrtX18, liquidity); err != nil {
		return err
	}
	p.Positions[id] = &ConcentratedPosition{ID: id, Owner: owner, LowerSqrtX18: new(big.Int).Set(lowerSqrtX18), UpperSqrtX18: new(big.Int).Set(upperSqrtX18), Liquidity: new(big.Int).Set(liquidity), TokensOwed0: big.NewInt(0), TokensOwed1: big.NewInt(0)}
	return nil
}

func (p *ConcentratedPool) RemovePosition(id, owner string, liquidity *big.Int) (*ConcentratedPosition, error) {
	p.ensurePositions()
	position := p.Positions[id]
	if position == nil || position.Owner != owner || liquidity == nil || liquidity.Sign() <= 0 || position.Liquidity.Cmp(liquidity) < 0 {
		return nil, fmt.Errorf("position ownership or liquidity invalid")
	}
	active := p.SqrtPriceX18.Cmp(position.LowerSqrtX18) >= 0 && p.SqrtPriceX18.Cmp(position.UpperSqrtX18) < 0
	if active && (p.Liquidity == nil || p.Liquidity.Cmp(liquidity) < 0) {
		return nil, fmt.Errorf("active liquidity underflow")
	}
	negative := new(big.Int).Neg(new(big.Int).Set(liquidity))
	p.addTick(position.LowerSqrtX18, negative)
	p.addTick(position.UpperSqrtX18, liquidity)
	if active {
		p.Liquidity.Sub(p.Liquidity, liquidity)
	}
	position.Liquidity.Sub(position.Liquidity, liquidity)
	copyPosition := *position
	copyPosition.Liquidity = new(big.Int).Set(position.Liquidity)
	copyPosition.TokensOwed0 = new(big.Int).Set(position.TokensOwed0)
	copyPosition.TokensOwed1 = new(big.Int).Set(position.TokensOwed1)
	if position.Liquidity.Sign() == 0 && position.TokensOwed0.Sign() == 0 && position.TokensOwed1.Sign() == 0 {
		delete(p.Positions, id)
	}
	return &copyPosition, nil
}

func (p *ConcentratedPool) TransferPosition(id, from, to string) error {
	p.ensurePositions()
	position := p.Positions[id]
	if position == nil || position.Owner != from || to == "" {
		return fmt.Errorf("position transfer unauthorized")
	}
	position.Owner = to
	return nil
}

func (p *ConcentratedPool) CollectPositionFees(id, owner string) (*big.Int, *big.Int, error) {
	p.ensurePositions()
	position := p.Positions[id]
	if position == nil || position.Owner != owner {
		return nil, nil, fmt.Errorf("position fee collection unauthorized")
	}
	owed0, owed1 := new(big.Int).Set(position.TokensOwed0), new(big.Int).Set(position.TokensOwed1)
	position.TokensOwed0.SetInt64(0)
	position.TokensOwed1.SetInt64(0)
	if position.Liquidity.Sign() == 0 {
		delete(p.Positions, id)
	}
	return owed0, owed1, nil
}

func (p *ConcentratedPool) accruePositionFee(fee *big.Int, token0 bool) {
	if fee == nil || fee.Sign() <= 0 {
		return
	}
	p.ensurePositions()
	active := make([]*ConcentratedPosition, 0, len(p.Positions))
	total := big.NewInt(0)
	for _, position := range p.Positions {
		if position != nil && position.Liquidity.Sign() > 0 && p.SqrtPriceX18.Cmp(position.LowerSqrtX18) >= 0 && p.SqrtPriceX18.Cmp(position.UpperSqrtX18) < 0 {
			active = append(active, position)
			total.Add(total, position.Liquidity)
		}
	}
	if total.Sign() == 0 {
		if token0 {
			p.FeeRemainder0.Add(p.FeeRemainder0, fee)
		} else {
			p.FeeRemainder1.Add(p.FeeRemainder1, fee)
		}
		return
	}
	sort.Slice(active, func(i, j int) bool { return active[i].ID < active[j].ID })
	distributed := big.NewInt(0)
	for _, position := range active {
		share := new(big.Int).Div(new(big.Int).Mul(fee, position.Liquidity), total)
		distributed.Add(distributed, share)
		if token0 {
			position.TokensOwed0.Add(position.TokensOwed0, share)
		} else {
			position.TokensOwed1.Add(position.TokensOwed1, share)
		}
	}
	remainder := new(big.Int).Sub(fee, distributed)
	if token0 {
		p.FeeRemainder0.Add(p.FeeRemainder0, remainder)
	} else {
		p.FeeRemainder1.Add(p.FeeRemainder1, remainder)
	}
}

func (p *ConcentratedPool) AddRange(lowerSqrtX18, upperSqrtX18, liquidity *big.Int) error {
	if p == nil || p.SqrtPriceX18 == nil || lowerSqrtX18 == nil || upperSqrtX18 == nil || liquidity == nil || liquidity.Sign() <= 0 || lowerSqrtX18.Cmp(upperSqrtX18) >= 0 {
		return fmt.Errorf("invalid concentrated range")
	}
	p.addTick(lowerSqrtX18, liquidity)
	p.addTick(upperSqrtX18, new(big.Int).Neg(liquidity))
	if p.SqrtPriceX18.Cmp(lowerSqrtX18) >= 0 && p.SqrtPriceX18.Cmp(upperSqrtX18) < 0 {
		if p.Liquidity == nil {
			p.Liquidity = big.NewInt(0)
		}
		p.Liquidity.Add(p.Liquidity, liquidity)
	}
	return nil
}

func (p *ConcentratedPool) addTick(price, delta *big.Int) {
	for i := range p.Ticks {
		if p.Ticks[i].SqrtPriceX18.Cmp(price) == 0 {
			p.Ticks[i].LiquidityNet.Add(p.Ticks[i].LiquidityNet, delta)
			return
		}
	}
	p.Ticks = append(p.Ticks, ConcentratedTick{SqrtPriceX18: new(big.Int).Set(price), LiquidityNet: new(big.Int).Set(delta)})
	sort.Slice(p.Ticks, func(i, j int) bool { return p.Ticks[i].SqrtPriceX18.Cmp(p.Ticks[j].SqrtPriceX18) < 0 })
}

func mulDiv(a, b, denominator *big.Int) *big.Int {
	if denominator == nil || denominator.Sign() == 0 {
		return big.NewInt(0)
	}
	return new(big.Int).Div(new(big.Int).Mul(a, b), denominator)
}

func (p *ConcentratedPool) nextTick(zeroForOne bool) (*ConcentratedTick, bool) {
	if zeroForOne {
		for i := len(p.Ticks) - 1; i >= 0; i-- {
			if p.Ticks[i].SqrtPriceX18.Cmp(p.SqrtPriceX18) < 0 {
				return &p.Ticks[i], true
			}
		}
	} else {
		for i := range p.Ticks {
			if p.Ticks[i].SqrtPriceX18.Cmp(p.SqrtPriceX18) > 0 {
				return &p.Ticks[i], true
			}
		}
	}
	return nil, false
}

// Swap executes across as many initialized ranges as needed and fails rather
// than silently crossing into a zero-liquidity region. zeroForOne means token0
// enters and token1 exits.
func (p *ConcentratedPool) Swap(amountIn *big.Int, zeroForOne bool) (*big.Int, error) {
	if p == nil || amountIn == nil || amountIn.Sign() <= 0 || p.SqrtPriceX18 == nil || p.SqrtPriceX18.Sign() <= 0 || p.Liquidity == nil || p.Liquidity.Sign() <= 0 || p.FeeBPS < 0 || p.FeeBPS > 1000 {
		return nil, fmt.Errorf("invalid concentrated pool or swap")
	}
	// Work on a deep copy so any failure (for example crossing into an
	// uninitialized/zero-liquidity range) cannot partially mutate the pool.
	working := p.deepCopy()
	output, err := working.swapInPlace(amountIn, zeroForOne)
	if err != nil {
		return nil, err
	}
	*p = *working
	return output, nil
}

func (p *ConcentratedPool) deepCopy() *ConcentratedPool {
	copyPool := &ConcentratedPool{FeeBPS: p.FeeBPS, Positions: make(map[string]*ConcentratedPosition)}
	if p.SqrtPriceX18 != nil {
		copyPool.SqrtPriceX18 = new(big.Int).Set(p.SqrtPriceX18)
	}
	if p.Liquidity != nil {
		copyPool.Liquidity = new(big.Int).Set(p.Liquidity)
	}
	if p.FeeRemainder0 != nil {
		copyPool.FeeRemainder0 = new(big.Int).Set(p.FeeRemainder0)
	}
	if p.FeeRemainder1 != nil {
		copyPool.FeeRemainder1 = new(big.Int).Set(p.FeeRemainder1)
	}
	for _, tick := range p.Ticks {
		copyPool.Ticks = append(copyPool.Ticks, ConcentratedTick{SqrtPriceX18: new(big.Int).Set(tick.SqrtPriceX18), LiquidityNet: new(big.Int).Set(tick.LiquidityNet)})
	}
	for id, position := range p.Positions {
		if position == nil {
			continue
		}
		copyPool.Positions[id] = &ConcentratedPosition{ID: position.ID, Owner: position.Owner, LowerSqrtX18: new(big.Int).Set(position.LowerSqrtX18), UpperSqrtX18: new(big.Int).Set(position.UpperSqrtX18), Liquidity: new(big.Int).Set(position.Liquidity), TokensOwed0: new(big.Int).Set(position.TokensOwed0), TokensOwed1: new(big.Int).Set(position.TokensOwed1)}
	}
	copyPool.ensurePositions()
	return copyPool
}

func (p *ConcentratedPool) swapInPlace(amountIn *big.Int, zeroForOne bool) (*big.Int, error) {
	netInput := mulDiv(amountIn, big.NewInt(10000-p.FeeBPS), big.NewInt(10000))
	fee := new(big.Int).Sub(new(big.Int).Set(amountIn), netInput)
	remaining := new(big.Int).Set(netInput)
	output := big.NewInt(0)
	for remaining.Sign() > 0 {
		if p.Liquidity.Sign() <= 0 {
			return nil, fmt.Errorf("concentrated swap reached zero liquidity")
		}
		tick, hasTick := p.nextTick(zeroForOne)
		var target *big.Int
		if hasTick {
			target = new(big.Int).Set(tick.SqrtPriceX18)
		}
		if zeroForOne {
			// dx to target = L * (P-target) * 1e18 / (P*target)
			if !hasTick {
				return nil, fmt.Errorf("no lower initialized tick")
			}
			dxTarget := mulDiv(new(big.Int).Mul(p.Liquidity, new(big.Int).Sub(p.SqrtPriceX18, target)), ammScaleX18, new(big.Int).Mul(p.SqrtPriceX18, target))
			if remaining.Cmp(dxTarget) < 0 {
				den := new(big.Int).Add(new(big.Int).Mul(p.Liquidity, ammScaleX18), new(big.Int).Mul(remaining, p.SqrtPriceX18))
				newP := mulDiv(new(big.Int).Mul(p.Liquidity, p.SqrtPriceX18), ammScaleX18, den)
				output.Add(output, mulDiv(p.Liquidity, new(big.Int).Sub(p.SqrtPriceX18, newP), ammScaleX18))
				p.SqrtPriceX18 = newP
				remaining.SetInt64(0)
				continue
			}
			output.Add(output, mulDiv(p.Liquidity, new(big.Int).Sub(p.SqrtPriceX18, target), ammScaleX18))
			remaining.Sub(remaining, dxTarget)
			p.SqrtPriceX18 = target
			p.Liquidity.Sub(p.Liquidity, tick.LiquidityNet)
		} else {
			if !hasTick {
				return nil, fmt.Errorf("no upper initialized tick")
			}
			dyTarget := mulDiv(p.Liquidity, new(big.Int).Sub(target, p.SqrtPriceX18), ammScaleX18)
			if remaining.Cmp(dyTarget) < 0 {
				newP := new(big.Int).Add(p.SqrtPriceX18, mulDiv(remaining, ammScaleX18, p.Liquidity))
				numerator := new(big.Int).Mul(new(big.Int).Mul(p.Liquidity, new(big.Int).Sub(newP, p.SqrtPriceX18)), ammScaleX18)
				output.Add(output, new(big.Int).Div(numerator, new(big.Int).Mul(p.SqrtPriceX18, newP)))
				p.SqrtPriceX18 = newP
				remaining.SetInt64(0)
				continue
			}
			numerator := new(big.Int).Mul(new(big.Int).Mul(p.Liquidity, new(big.Int).Sub(target, p.SqrtPriceX18)), ammScaleX18)
			output.Add(output, new(big.Int).Div(numerator, new(big.Int).Mul(p.SqrtPriceX18, target)))
			remaining.Sub(remaining, dyTarget)
			p.SqrtPriceX18 = target
			p.Liquidity.Add(p.Liquidity, tick.LiquidityNet)
		}
	}
	if output.Sign() <= 0 {
		return nil, fmt.Errorf("concentrated swap rounded to zero")
	}
	p.accruePositionFee(fee, zeroForOne)
	return output, nil
}

type StablePoolRiskPolicy struct{ BaseFeeBPS, DepegSurchargeBPS, WarningDeviationBPS, EmergencyDeviationBPS, EmergencyMaxSwapBPS int64 }

func (p StablePoolRiskPolicy) EffectiveFeeAndCap(price0, price1 float64) (int64, int64, bool) {
	if p.BaseFeeBPS < 0 {
		p.BaseFeeBPS = 0
	}
	if p.WarningDeviationBPS <= 0 {
		p.WarningDeviationBPS = 100
	}
	if p.EmergencyDeviationBPS <= p.WarningDeviationBPS {
		p.EmergencyDeviationBPS = 1000
	}
	if p.EmergencyMaxSwapBPS <= 0 || p.EmergencyMaxSwapBPS > 10000 {
		p.EmergencyMaxSwapBPS = 100
	}
	if price0 <= 0 || price1 <= 0 {
		return p.BaseFeeBPS + p.DepegSurchargeBPS, p.EmergencyMaxSwapBPS, true
	}
	deviation := int64(math.Round(math.Abs(price0-price1) / math.Max(price0, price1) * 10000))
	fee, capBPS, emergency := p.BaseFeeBPS, int64(10000), false
	if deviation >= p.WarningDeviationBPS {
		fee += p.DepegSurchargeBPS
	}
	if deviation >= p.EmergencyDeviationBPS {
		emergency = true
		capBPS = p.EmergencyMaxSwapBPS
	}
	if fee > 1000 {
		fee = 1000
	}
	return fee, capBPS, emergency
}
