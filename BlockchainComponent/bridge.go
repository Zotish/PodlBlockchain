package blockchaincomponent

import (
	"math/big"
	"strings"
	"time"

	constantset "github.com/Zotish/DefenceProject/ConstantSet"
)

const (
	BridgeStatusLocked  = "locked"
	BridgeStatusMinted  = "minted"
	BridgeStatusBurned  = "burned"
	BridgeStatusUnlock  = "unlocked"
	BridgeStatusFailed  = "failed"
)

type BridgeRequest struct {
	ID           string `json:"id"`
	From         string `json:"from"`
	To           string `json:"to"`
	Amount       string `json:"amount"`
	Token        string `json:"token,omitempty"`
	SourceChain  string `json:"source_chain"`
	TargetChain  string `json:"target_chain"`
	Status       string `json:"status"`
	LqdTxHash    string `json:"lqd_tx_hash"`
	BscTxHash    string `json:"bsc_tx_hash,omitempty"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
}

func (bc *Blockchain_struct) AddBridgeRequest(tx *Transaction, toBSC string) {
	if tx == nil || tx.TxHash == "" {
		return
	}
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()

	if bc.BridgeRequests == nil {
		bc.BridgeRequests = make(map[string]*BridgeRequest)
	}
	id := strings.ToLower(tx.TxHash)
	if _, ok := bc.BridgeRequests[id]; ok {
		return
	}
	now := time.Now().Unix()
	bc.BridgeRequests[id] = &BridgeRequest{
		ID:          id,
		From:        tx.From,
		To:          toBSC,
		Amount:      AmountString(tx.Value),
		Token:       "LQD",
		SourceChain: "LQD",
		TargetChain: "BSC",
		Status:      BridgeStatusLocked,
		LqdTxHash:   tx.TxHash,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// AddBridgeRequestBSC records a BSC->LQD lock (token on BSC, mint on LQD).
func (bc *Blockchain_struct) AddBridgeRequestBSC(bscTx string, token string, from string, toLqd string, amount *big.Int) {
	if bscTx == "" || token == "" || toLqd == "" || amount == nil || amount.Sign() <= 0 {
		return
	}
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	if bc.BridgeRequests == nil {
		bc.BridgeRequests = make(map[string]*BridgeRequest)
	}
	id := strings.ToLower(bscTx)
	if _, ok := bc.BridgeRequests[id]; ok {
		return
	}
	now := time.Now().Unix()
	bc.BridgeRequests[id] = &BridgeRequest{
		ID:          id,
		From:        from,
		To:          toLqd,
		Amount:      AmountString(amount),
		Token:       strings.ToLower(token),
		SourceChain: "BSC",
		TargetChain: "LQD",
		Status:      BridgeStatusLocked,
		BscTxHash:   bscTx,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// AddBridgeRequestBurn records an LQD burn request for release on BSC.
func (bc *Blockchain_struct) AddBridgeRequestBurn(lqdTx string, token string, from string, toBsc string, amount *big.Int) {
	if lqdTx == "" || token == "" || toBsc == "" || amount == nil || amount.Sign() <= 0 {
		return
	}
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	if bc.BridgeRequests == nil {
		bc.BridgeRequests = make(map[string]*BridgeRequest)
	}
	id := strings.ToLower(lqdTx)
	if _, ok := bc.BridgeRequests[id]; ok {
		return
	}
	now := time.Now().Unix()
	bc.BridgeRequests[id] = &BridgeRequest{
		ID:          id,
		From:        from,
		To:          toBsc,
		Amount:      AmountString(amount),
		Token:       strings.ToLower(token),
		SourceChain: "LQD",
		TargetChain: "BSC",
		Status:      BridgeStatusBurned,
		LqdTxHash:   lqdTx,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func (bc *Blockchain_struct) MarkBridgeMinted(id string, bscTx string, lqdTx string) {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	if bc.BridgeRequests == nil {
		return
	}
	req, ok := bc.BridgeRequests[strings.ToLower(id)]
	if !ok {
		return
	}
	req.Status = BridgeStatusMinted
	if bscTx != "" {
		req.BscTxHash = bscTx
	}
	if lqdTx != "" {
		req.LqdTxHash = lqdTx
	}
	req.UpdatedAt = time.Now().Unix()
}

func (bc *Blockchain_struct) MarkBridgeUnlocked(id string) {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	if bc.BridgeRequests == nil {
		return
	}
	req, ok := bc.BridgeRequests[strings.ToLower(id)]
	if !ok {
		return
	}
	req.Status = BridgeStatusUnlock
	req.UpdatedAt = time.Now().Unix()
}

func (bc *Blockchain_struct) ListBridgeRequests(address string) []*BridgeRequest {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	out := make([]*BridgeRequest, 0)
	for _, r := range bc.BridgeRequests {
		if address == "" || strings.EqualFold(r.From, address) || strings.EqualFold(r.To, address) {
			out = append(out, r)
		}
	}
	return out
}

func BridgeEscrowAddress() string {
	return constantset.BridgeEscrowAddress
}
