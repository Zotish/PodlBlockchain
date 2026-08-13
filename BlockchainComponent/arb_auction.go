package blockchaincomponent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"sync"

	constantset "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/ConstantSet"
)

type ProtocolArbPolicy struct {
	Enabled          bool   `json:"enabled"`
	RequireAuction   bool   `json:"require_auction"`
	AuthorizedKeeper string `json:"authorized_keeper,omitempty"`
	MaxCapitalBPS    int64  `json:"max_capital_bps"`
	MinProfitBPS     int64  `json:"min_profit_bps"`
	MinKeeperBond    string `json:"min_keeper_bond"`
	ExecutionTimeout uint64 `json:"execution_timeout_blocks"`
	MissedSlashBPS   int64  `json:"missed_execution_slash_bps"`
	UnbondDelay      uint64 `json:"unbond_delay_blocks"`
}

func DefaultProtocolArbPolicy() ProtocolArbPolicy {
	return ProtocolArbPolicy{Enabled: false, RequireAuction: true, MaxCapitalBPS: 1000, MinProfitBPS: 50, MinKeeperBond: "100000", ExecutionTimeout: 20, MissedSlashBPS: 1000, UnbondDelay: 1000}
}

const ProtocolArbKeeperBondEscrow = "0x0000000000000000000000000000000000000e03"

type ArbBid struct {
	Keeper     string `json:"keeper"`
	Commitment string `json:"commitment"`
	FeeBPS     int64  `json:"fee_bps,omitempty"`
	Salt       string `json:"salt,omitempty"`
	Revealed   bool   `json:"revealed"`
}
type ArbAuction struct {
	ID                string            `json:"id"`
	OpportunityHash   string            `json:"opportunity_hash"`
	CommitEnd         uint64            `json:"commit_end"`
	RevealEnd         uint64            `json:"reveal_end"`
	Status            string            `json:"status"`
	Winner            string            `json:"winner,omitempty"`
	WinningFeeBPS     int64             `json:"winning_fee_bps,omitempty"`
	Executed          bool              `json:"executed"`
	ExecutionHeight   uint64            `json:"execution_height,omitempty"`
	RealizedProfit    string            `json:"realized_profit,omitempty"`
	Bids              map[string]ArbBid `json:"bids"`
	Candidates        []string          `json:"candidates,omitempty"`
	FailedWinners     []string          `json:"failed_winners,omitempty"`
	ExecutionDeadline uint64            `json:"execution_deadline,omitempty"`
}

type arbMarketLock struct {
	mu   sync.Mutex
	refs int
}

// lockArbMarkets serializes only operations whose pool sets overlap. Calls on
// independent markets can perform discovery and reserve validation in
// parallel; final custody/accounting still commits under the consensus-state
// lock so ledger effects remain deterministic.
func (bc *Blockchain_struct) lockArbMarkets(metrics []PoolMetrics) func() {
	keys, seen := make([]string, 0, len(metrics)), map[string]bool{}
	for _, metric := range metrics {
		key := strings.ToLower(strings.TrimSpace(metric.PairAddress))
		if key != "" && !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	bc.ArbMarketLockMu.Lock()
	if bc.ArbMarketLocks == nil {
		bc.ArbMarketLocks = make(map[string]*arbMarketLock)
	}
	locks := make([]*arbMarketLock, 0, len(keys))
	for _, key := range keys {
		lock := bc.ArbMarketLocks[key]
		if lock == nil {
			lock = &arbMarketLock{}
			bc.ArbMarketLocks[key] = lock
		}
		lock.refs++
		locks = append(locks, lock)
	}
	bc.ArbMarketLockMu.Unlock()
	for _, lock := range locks {
		lock.mu.Lock()
	}
	return func() {
		for i := len(locks) - 1; i >= 0; i-- {
			locks[i].mu.Unlock()
		}
		bc.ArbMarketLockMu.Lock()
		for i, key := range keys {
			locks[i].refs--
			if locks[i].refs == 0 && bc.ArbMarketLocks[key] == locks[i] {
				delete(bc.ArbMarketLocks, key)
			}
		}
		bc.ArbMarketLockMu.Unlock()
	}
}

func (bc *Blockchain_struct) prepareArbPaths(metrics []PoolMetrics) (*ProtocolArb, []ArbPath, error) {
	bc.Mutex.Lock()
	bc.EnsureRuntimeState()
	policy, engine := bc.ArbPolicy, bc.ContractEngine
	bc.Mutex.Unlock()
	if engine == nil || !policy.Enabled || len(metrics) < 2 {
		return nil, nil, fmt.Errorf("arbitrage engine is disabled or unavailable")
	}
	treasury := bc.AccountBalanceAmount(constantset.LiquidityPoolAddress)
	maxCapitalBPS := policy.MaxCapitalBPS
	if maxCapitalBPS <= 0 || maxCapitalBPS > MaxCapitalBps {
		maxCapitalBPS = MaxCapitalBps
	}
	maxCapital := new(big.Int).Div(new(big.Int).Mul(treasury, big.NewInt(maxCapitalBPS)), big.NewInt(10000))
	if maxCapital.Sign() <= 0 {
		return nil, nil, fmt.Errorf("arbitrage treasury has no deployable capital")
	}
	runner := NewProtocolArb()
	paths := runner.findTriangularPaths(metrics, maxCapital)
	if len(paths) == 0 {
		return nil, nil, fmt.Errorf("auction opportunity is no longer profitable")
	}
	return runner, paths, nil
}

func (bc *Blockchain_struct) BondArbKeeper(keeper string, amount *big.Int) error {
	keeper = strings.ToLower(strings.TrimSpace(keeper))
	if bc == nil || !ValidateAddress(keeper) || amount == nil || amount.Sign() <= 0 {
		return fmt.Errorf("valid keeper and positive bond required")
	}
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	bc.EnsureRuntimeState()
	if !bc.subAccountBalance(keeper, amount) {
		return fmt.Errorf("insufficient keeper balance")
	}
	bc.addAccountBalance(ProtocolArbKeeperBondEscrow, amount)
	if bc.ArbKeeperBonds[keeper] == nil {
		bc.ArbKeeperBonds[keeper] = big.NewInt(0)
	}
	bc.ArbKeeperBonds[keeper].Add(bc.ArbKeeperBonds[keeper], amount)
	delete(bc.ArbKeeperUnbondAt, keeper)
	bc.persistRuntimeStateLocked()
	return nil
}

func (bc *Blockchain_struct) RequestArbKeeperUnbond(keeper string, height uint64) error {
	keeper = strings.ToLower(strings.TrimSpace(keeper))
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	bc.EnsureRuntimeState()
	if bc.ArbKeeperBonds[keeper] == nil || bc.ArbKeeperBonds[keeper].Sign() <= 0 {
		return fmt.Errorf("keeper has no bond")
	}
	for _, auction := range bc.ArbAuctions {
		if auction != nil && auction.Status == "finalized" && strings.EqualFold(auction.Winner, keeper) {
			return fmt.Errorf("keeper has active auction duty")
		}
	}
	bc.ArbKeeperUnbondAt[keeper] = height + bc.ArbPolicy.UnbondDelay
	bc.persistRuntimeStateLocked()
	return nil
}

func (bc *Blockchain_struct) WithdrawArbKeeperBond(keeper string, height uint64) (*big.Int, error) {
	keeper = strings.ToLower(strings.TrimSpace(keeper))
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	bc.EnsureRuntimeState()
	unlock, ok := bc.ArbKeeperUnbondAt[keeper]
	bond := bc.ArbKeeperBonds[keeper]
	if !ok || height < unlock || bond == nil || bond.Sign() <= 0 {
		return nil, fmt.Errorf("keeper bond remains locked")
	}
	amount := new(big.Int).Set(bond)
	if !bc.subAccountBalance(ProtocolArbKeeperBondEscrow, amount) {
		return nil, fmt.Errorf("keeper bond escrow insolvent")
	}
	bc.addAccountBalance(keeper, amount)
	bc.ArbKeeperBonds[keeper].SetInt64(0)
	delete(bc.ArbKeeperUnbondAt, keeper)
	bc.persistRuntimeStateLocked()
	return amount, nil
}

// ArbOpportunityHash commits the auction to exact pool addresses and reserve
// snapshots so a winning bid cannot be reused for a different opportunity.
func ArbOpportunityHash(metrics []PoolMetrics) string {
	copyMetrics := append([]PoolMetrics(nil), metrics...)
	sort.Slice(copyMetrics, func(i, j int) bool {
		return strings.ToLower(copyMetrics[i].PairAddress) < strings.ToLower(copyMetrics[j].PairAddress)
	})
	type committedPool struct {
		Pair, Token0, Token1, Reserve0, Reserve1 string
	}
	rows := make([]committedPool, 0, len(copyMetrics))
	for _, m := range copyMetrics {
		rows = append(rows, committedPool{strings.ToLower(m.PairAddress), strings.ToLower(m.Token0), strings.ToLower(m.Token1), AmountString(m.Reserve0), AmountString(m.Reserve1)})
	}
	raw, _ := json.Marshal(rows)
	sum := sha256.Sum256(append([]byte("PODL-ARB-OPPORTUNITY-V1:"), raw...))
	return "0x" + hex.EncodeToString(sum[:])
}

func arbBidCommitment(auctionID, keeper string, fee int64, salt string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(auctionID) + "|" + strings.ToLower(keeper) + "|" + strconv.FormatInt(fee, 10) + "|" + salt))
	return "0x" + hex.EncodeToString(sum[:])
}

func (bc *Blockchain_struct) OpenArbAuction(opportunityHash string, currentHeight, commitBlocks, revealBlocks uint64) (*ArbAuction, error) {
	if bc == nil || strings.TrimSpace(opportunityHash) == "" || commitBlocks == 0 || revealBlocks == 0 {
		return nil, fmt.Errorf("valid opportunity and auction windows required")
	}
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	bc.EnsureRuntimeState()
	sum := sha256.Sum256([]byte(opportunityHash + "|" + strconv.FormatUint(currentHeight, 10)))
	id := "arb_" + hex.EncodeToString(sum[:12])
	a := &ArbAuction{ID: id, OpportunityHash: opportunityHash, CommitEnd: currentHeight + commitBlocks, RevealEnd: currentHeight + commitBlocks + revealBlocks, Status: "commit", Bids: map[string]ArbBid{}}
	bc.ArbAuctions[id] = a
	bc.persistRuntimeStateLocked()
	return a, nil
}
func (bc *Blockchain_struct) CommitArbBid(id, keeper, commitment string, height uint64) error {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	bc.EnsureRuntimeState()
	a := bc.ArbAuctions[id]
	keeper = strings.ToLower(strings.TrimSpace(keeper))
	if a == nil || a.Status != "commit" || height > a.CommitEnd || !ValidateAddress(keeper) || len(strings.TrimSpace(commitment)) != 66 {
		return fmt.Errorf("auction is not accepting commitments")
	}
	minimum := NewAmountFromStringOrZero(bc.ArbPolicy.MinKeeperBond)
	if bc.ArbKeeperBonds[keeper] == nil || bc.ArbKeeperBonds[keeper].Cmp(minimum) < 0 {
		return fmt.Errorf("keeper bond below auction minimum")
	}
	a.Bids[keeper] = ArbBid{Keeper: keeper, Commitment: strings.ToLower(commitment)}
	bc.persistRuntimeStateLocked()
	return nil
}
func (bc *Blockchain_struct) RevealArbBid(id, keeper string, feeBPS int64, salt string, height uint64) error {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	bc.EnsureRuntimeState()
	a := bc.ArbAuctions[id]
	keeper = strings.ToLower(strings.TrimSpace(keeper))
	if a == nil || height <= a.CommitEnd || height > a.RevealEnd || feeBPS < 0 || feeBPS > 2000 {
		return fmt.Errorf("auction is not in reveal window")
	}
	bid, ok := a.Bids[keeper]
	if !ok || bid.Commitment != arbBidCommitment(id, keeper, feeBPS, salt) {
		return fmt.Errorf("bid reveal does not match")
	}
	a.Status = "reveal"
	bid.FeeBPS, bid.Salt, bid.Revealed = feeBPS, salt, true
	a.Bids[keeper] = bid
	bc.persistRuntimeStateLocked()
	return nil
}
func (bc *Blockchain_struct) FinalizeArbAuction(id string, height uint64) (*ArbAuction, error) {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	bc.EnsureRuntimeState()
	a := bc.ArbAuctions[id]
	if a == nil || height <= a.RevealEnd || (a.Status != "commit" && a.Status != "reveal") {
		return nil, fmt.Errorf("auction not finalizable")
	}
	bids := []ArbBid{}
	for _, bid := range a.Bids {
		if bid.Revealed {
			bids = append(bids, bid)
		}
	}
	sort.Slice(bids, func(i, j int) bool {
		if bids[i].FeeBPS == bids[j].FeeBPS {
			return bids[i].Keeper < bids[j].Keeper
		}
		return bids[i].FeeBPS < bids[j].FeeBPS
	})
	if len(bids) == 0 {
		a.Status = "failed"
		return a, nil
	}
	for _, bid := range bids {
		a.Candidates = append(a.Candidates, bid.Keeper)
	}
	a.Status, a.Winner, a.WinningFeeBPS = "finalized", bids[0].Keeper, bids[0].FeeBPS
	a.ExecutionDeadline = height + bc.ArbPolicy.ExecutionTimeout
	bc.ArbPolicy.AuthorizedKeeper = a.Winner
	bc.persistRuntimeStateLocked()
	return a, nil
}

// ExecuteArbAuction binds the finalized winner to the committed market
// snapshot and settles at most once. Execution still re-reads live reserves;
// if profitability disappeared, no state/revenue is claimed.
func (bc *Blockchain_struct) ExecuteArbAuction(id, keeper string, height uint64, metrics []PoolMetrics) (*ArbAuction, error) {
	if bc == nil || bc.ContractEngine == nil {
		return nil, fmt.Errorf("chain contract engine required")
	}
	keeper = strings.ToLower(strings.TrimSpace(keeper))
	// Reject unauthorized/stale work before doing graph discovery. The same
	// checks are repeated at commit time to close the check/use window.
	bc.Mutex.Lock()
	bc.EnsureRuntimeState()
	preflightAuction := bc.ArbAuctions[id]
	if preflightAuction == nil || preflightAuction.Status != "finalized" || preflightAuction.Executed || !strings.EqualFold(preflightAuction.Winner, keeper) || height <= preflightAuction.RevealEnd || (preflightAuction.ExecutionDeadline > 0 && height > preflightAuction.ExecutionDeadline) || preflightAuction.OpportunityHash != ArbOpportunityHash(metrics) {
		bc.Mutex.Unlock()
		return nil, fmt.Errorf("auction is not executable for this keeper and snapshot")
	}
	bc.Mutex.Unlock()
	unlockMarkets := bc.lockArbMarkets(metrics)
	defer unlockMarkets()
	runner, preparedPaths, err := bc.prepareArbPaths(metrics)
	if err != nil {
		return nil, err
	}
	// Winner validation, reserve mutation, custody transfer, revenue allocation
	// and the executed flag form one short chain-state commit. Expensive path
	// discovery above is not inside this global critical section.
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	bc.EnsureRuntimeState()
	a := bc.ArbAuctions[id]
	if a == nil || a.Status != "finalized" || a.Executed || !strings.EqualFold(a.Winner, keeper) || height <= a.RevealEnd || (a.ExecutionDeadline > 0 && height > a.ExecutionDeadline) {
		return nil, fmt.Errorf("auction is not executable by keeper")
	}
	if a.OpportunityHash != ArbOpportunityHash(metrics) {
		return nil, fmt.Errorf("opportunity snapshot mismatch")
	}
	before := bc.AccountBalanceAmount(constantset.LiquidityPoolAddress)
	runner.executePreparedPaths(bc, preparedPaths)
	after := bc.AccountBalanceAmount(constantset.LiquidityPoolAddress)
	profit := new(big.Int).Sub(after, before)
	if profit.Sign() <= 0 {
		return nil, fmt.Errorf("auction opportunity is no longer profitable")
	}
	// Move only realized profit (never principal) into the protocol revenue
	// escrow before allocating the waterfall.
	if !bc.subAccountBalance(constantset.LiquidityPoolAddress, profit) {
		return nil, fmt.Errorf("realized arb profit custody transfer failed")
	}
	bc.addAccountBalance(ProtocolRevenueEscrowAddress, profit)
	timestamp := int64(0)
	if len(bc.Blocks) > 0 && bc.Blocks[len(bc.Blocks)-1] != nil {
		timestamp = int64(bc.Blocks[len(bc.Blocks)-1].TimeStamp)
	}
	if _, err := bc.recordProtocolRevenueConsensus("protocol_arb", profit, "auction:"+id, timestamp); err != nil {
		_ = bc.subAccountBalance(ProtocolRevenueEscrowAddress, profit)
		bc.addAccountBalance(constantset.LiquidityPoolAddress, profit)
		return nil, err
	}
	a.Executed, a.ExecutionHeight, a.RealizedProfit = true, height, profit.String()
	a.Status = "executed"
	bc.persistRuntimeStateLocked()
	return a, nil
}

// ExpireArbAuction penalizes a winner that missed its execution deadline and
// deterministically promotes the next revealed bidder. Anyone may call it.
func (bc *Blockchain_struct) ExpireArbAuction(id string, height uint64) (*ArbAuction, error) {
	if bc == nil {
		return nil, fmt.Errorf("nil chain")
	}
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	bc.EnsureRuntimeState()
	a := bc.ArbAuctions[id]
	if a == nil || a.Status != "finalized" || a.Executed || a.ExecutionDeadline == 0 || height <= a.ExecutionDeadline {
		return nil, fmt.Errorf("auction execution deadline has not expired")
	}
	failed := strings.ToLower(a.Winner)
	bond := bc.ArbKeeperBonds[failed]
	if bond != nil && bond.Sign() > 0 {
		slash := new(big.Int).Div(new(big.Int).Mul(bond, big.NewInt(bc.ArbPolicy.MissedSlashBPS)), big.NewInt(10000))
		if slash.Sign() == 0 {
			slash.SetInt64(1)
		}
		if slash.Cmp(bond) > 0 {
			slash.Set(bond)
		}
		bond.Sub(bond, slash)
		if !bc.subAccountBalance(ProtocolArbKeeperBondEscrow, slash) {
			return nil, fmt.Errorf("keeper bond escrow insolvent")
		}
		bc.addAccountBalance(ProtocolRevenueEscrowAddress, slash)
		if _, err := bc.recordProtocolRevenueConsensus("slashing", slash, "missed-arb:"+id+":"+failed, int64(height)); err != nil {
			return nil, err
		}
	}
	a.FailedWinners = append(a.FailedWinners, failed)
	next := ""
	for _, candidate := range a.Candidates {
		candidate = strings.ToLower(candidate)
		alreadyFailed := false
		for _, prior := range a.FailedWinners {
			if prior == candidate {
				alreadyFailed = true
				break
			}
		}
		if !alreadyFailed && bc.ArbKeeperBonds[candidate] != nil && bc.ArbKeeperBonds[candidate].Cmp(NewAmountFromStringOrZero(bc.ArbPolicy.MinKeeperBond)) >= 0 {
			next = candidate
			break
		}
	}
	if next == "" {
		a.Status, bc.ArbPolicy.AuthorizedKeeper = "failed", ""
	} else {
		a.Winner, a.WinningFeeBPS, a.ExecutionDeadline = next, a.Bids[next].FeeBPS, height+bc.ArbPolicy.ExecutionTimeout
		bc.ArbPolicy.AuthorizedKeeper = next
	}
	bc.persistRuntimeStateLocked()
	return a, nil
}
