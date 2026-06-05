package blockchaincomponent

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"sync"
	"time"
)

var persistRuntimeState = func(bc Blockchain_struct) error {
	return PutIntoDB(bc)
}

func (bc *Blockchain_struct) StrategyVaultDeposit(owner, pool, tokenA, tokenB string, amountA, amountB *big.Int) (*StrategyVaultPosition, error) {
	if bc == nil {
		return nil, fmt.Errorf("nil blockchain")
	}
	owner = strings.TrimSpace(owner)
	pool = strings.TrimSpace(pool)
	tokenA = strings.TrimSpace(tokenA)
	tokenB = strings.TrimSpace(tokenB)
	if !ValidateAddress(owner) {
		return nil, fmt.Errorf("valid owner address is required")
	}
	if pool == "" || tokenA == "" || tokenB == "" {
		return nil, fmt.Errorf("pool and token pair are required")
	}
	if amountA == nil || amountB == nil || amountA.Sign() <= 0 || amountB.Sign() <= 0 {
		return nil, fmt.Errorf("positive token amounts are required")
	}

	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	bc.EnsureRuntimeState()

	now := time.Now().Unix()
	shares := new(big.Int).Add(amountA, amountB)
	id := strategyVaultID(owner, pool, tokenA, tokenB, now, shares)
	pos := &StrategyVaultPosition{
		ID:          id,
		Owner:       owner,
		CurrentPool: pool,
		TokenA:      tokenA,
		TokenB:      tokenB,
		AmountA:     CopyAmount(amountA),
		AmountB:     CopyAmount(amountB),
		Shares:      shares,
		Status:      "active",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	bc.StrategyVaults[id] = pos
	bc.persistRuntimeStateLocked()
	return pos, nil
}

func (bc *Blockchain_struct) StrategyVaultRebalance(vaultID, targetPool string, minOutBps int, reason string) (*StrategyVaultMovement, error) {
	if bc == nil {
		return nil, fmt.Errorf("nil blockchain")
	}
	vaultID = strings.TrimSpace(vaultID)
	targetPool = strings.TrimSpace(targetPool)
	reason = strings.TrimSpace(reason)
	if vaultID == "" || targetPool == "" {
		return nil, fmt.Errorf("vault id and target pool are required")
	}
	if minOutBps == 0 {
		minOutBps = 9900
	}
	if minOutBps < 9000 || minOutBps > 10000 {
		return nil, fmt.Errorf("min_out_bps must be between 9000 and 10000")
	}

	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	bc.EnsureRuntimeState()

	pos, ok := bc.StrategyVaults[vaultID]
	if !ok || pos == nil {
		return nil, fmt.Errorf("vault position not found")
	}
	if pos.Status != "active" {
		return nil, fmt.Errorf("vault position is not active")
	}
	if strings.EqualFold(pos.CurrentPool, targetPool) {
		return nil, fmt.Errorf("target pool is already current")
	}

	now := time.Now().Unix()
	move := &StrategyVaultMovement{
		ID:         strategyMovementID(vaultID, pos.CurrentPool, targetPool, now),
		VaultID:    vaultID,
		FromPool:   pos.CurrentPool,
		ToPool:     targetPool,
		Reason:     reason,
		Status:     "executed",
		MinOutBps:  minOutBps,
		AmountA:    CopyAmount(pos.AmountA),
		AmountB:    CopyAmount(pos.AmountB),
		Shares:     CopyAmount(pos.Shares),
		ExecutedAt: now,
	}
	pos.CurrentPool = targetPool
	pos.UpdatedAt = now
	pos.LastMove = move
	bc.StrategyVaultMoves = append(bc.StrategyVaultMoves, *move)
	if len(bc.StrategyVaultMoves) > 1000 {
		bc.StrategyVaultMoves = bc.StrategyVaultMoves[len(bc.StrategyVaultMoves)-1000:]
	}
	bc.persistRuntimeStateLocked()
	return move, nil
}

func (bc *Blockchain_struct) StrategyVaultWithdraw(vaultID string) (*StrategyVaultPosition, error) {
	if bc == nil {
		return nil, fmt.Errorf("nil blockchain")
	}
	vaultID = strings.TrimSpace(vaultID)
	if vaultID == "" {
		return nil, fmt.Errorf("vault id is required")
	}

	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	bc.EnsureRuntimeState()

	pos, ok := bc.StrategyVaults[vaultID]
	if !ok || pos == nil {
		return nil, fmt.Errorf("vault position not found")
	}
	if pos.Status == "withdrawn" {
		return pos, nil
	}
	pos.Status = "withdrawn"
	pos.UpdatedAt = time.Now().Unix()
	bc.persistRuntimeStateLocked()
	return pos, nil
}

func (bc *Blockchain_struct) StrategyVaultStatus(owner string) []*StrategyVaultPosition {
	if bc == nil {
		return nil
	}
	owner = strings.TrimSpace(owner)
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	bc.EnsureRuntimeState()

	positions := make([]*StrategyVaultPosition, 0, len(bc.StrategyVaults))
	for _, pos := range bc.StrategyVaults {
		if pos == nil {
			continue
		}
		if owner != "" && !strings.EqualFold(pos.Owner, owner) {
			continue
		}
		copyPos := *pos
		copyPos.AmountA = CopyAmount(pos.AmountA)
		copyPos.AmountB = CopyAmount(pos.AmountB)
		copyPos.Shares = CopyAmount(pos.Shares)
		if pos.LastMove != nil {
			move := *pos.LastMove
			move.AmountA = CopyAmount(pos.LastMove.AmountA)
			move.AmountB = CopyAmount(pos.LastMove.AmountB)
			move.Shares = CopyAmount(pos.LastMove.Shares)
			copyPos.LastMove = &move
		}
		positions = append(positions, &copyPos)
	}
	sort.SliceStable(positions, func(i, j int) bool {
		return positions[i].UpdatedAt > positions[j].UpdatedAt
	})
	return positions
}

func (bc *Blockchain_struct) persistRuntimeStateLocked() {
	dbCopy := *bc
	dbCopy.Mutex = sync.Mutex{}
	_ = persistRuntimeState(dbCopy)
}

func strategyVaultID(parts ...interface{}) string {
	return hashedID("vault", parts...)
}

func strategyMovementID(parts ...interface{}) string {
	return hashedID("move", parts...)
}

func hashedID(prefix string, parts ...interface{}) string {
	h := sha256.New()
	h.Write([]byte(prefix))
	for _, part := range parts {
		h.Write([]byte("|"))
		h.Write([]byte(fmt.Sprint(part)))
	}
	return prefix + "_" + hex.EncodeToString(h.Sum(nil))[:20]
}
