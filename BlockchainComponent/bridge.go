package blockchaincomponent

import (
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"strings"
	"sync"
	"time"

	constantset "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/ConstantSet"
)

const (
	BridgeStatusLocked     = "locked"
	BridgeStatusQueued     = "queued"
	BridgeStatusProcessing = "processing"
	BridgeStatusMinted     = "minted"
	BridgeStatusBurned     = "burned"
	BridgeStatusUnlock     = "unlocked"
	BridgeStatusFailed     = "failed"
)

type BridgeRequest struct {
	ID                string `json:"id"`
	Mode              string `json:"mode,omitempty"`
	Family            string `json:"family,omitempty"`
	Adapter           string `json:"adapter,omitempty"`
	From              string `json:"from"`
	To                string `json:"to"`
	Amount            string `json:"amount"`
	Token             string `json:"token,omitempty"`
	SourceChain       string `json:"source_chain"`
	TargetChain       string `json:"target_chain"`
	SourceChainID     string `json:"source_chain_id,omitempty"`
	TargetChainID     string `json:"target_chain_id,omitempty"`
	Status            string `json:"status"`
	LqdTxHash         string `json:"lqd_tx_hash"`
	BscTxHash         string `json:"bsc_tx_hash,omitempty"`
	PrivacyCommitment string `json:"privacy_commitment,omitempty"`
	ShieldedKind      string `json:"shielded_kind,omitempty"`
	ShieldedNote      string `json:"shielded_note,omitempty"`
	ShieldedProof     string `json:"shielded_proof,omitempty"`
	ShieldedNullifier string `json:"shielded_nullifier,omitempty"`
	ShieldedRoot      string `json:"shielded_root,omitempty"`
	SourceTxHash      string `json:"source_tx_hash,omitempty"`
	SourceAddress     string `json:"source_address,omitempty"`
	SourceMemo        string `json:"source_memo,omitempty"`
	SourceSequence    string `json:"source_sequence,omitempty"`
	SourceOutput      string `json:"source_output,omitempty"`
	BatchID           string `json:"batch_id,omitempty"`
	QueuedAt          int64  `json:"queued_at,omitempty"`
	ProcessedAt       int64  `json:"processed_at,omitempty"`
	CreatedAt         int64  `json:"created_at"`
	UpdatedAt         int64  `json:"updated_at"`
}

func bridgeFamilyAndAdapterForChainIDs(chainIDs ...string) (string, string) {
	for _, chainID := range chainIDs {
		chainID = strings.TrimSpace(chainID)
		if chainID == "" {
			continue
		}
		if reg, err := LoadBridgeChainRegistry(); err == nil && reg != nil {
			if cfg := reg.ChainByID(chainID); cfg != nil {
				family := NormalizeBridgeFamilyID(cfg.Family)
				if family == "" {
					family = "evm"
				}
				adapter := NormalizeBridgeFamilyID(cfg.Adapter)
				if adapter == "" {
					adapter = family
				}
				return family, adapter
			}
			if cfg := reg.ChainByName(chainID); cfg != nil {
				family := NormalizeBridgeFamilyID(cfg.Family)
				if family == "" {
					family = "evm"
				}
				adapter := NormalizeBridgeFamilyID(cfg.Adapter)
				if adapter == "" {
					adapter = family
				}
				return family, adapter
			}
		}
	}
	return "evm", "evm"
}

var shieldedBridgeState = struct {
	sync.Mutex
	SpentNullifiers map[string]bool
	Commitments     []string
	Root            string
}{
	SpentNullifiers: make(map[string]bool),
}

func (bc *Blockchain_struct) AddBridgeRequest(tx *Transaction, toBSC string) {
	bc.addBridgeRequestMode(tx, toBSC, BridgeStatusLocked, "public", "", "", "", "", "", "LQD", "BSC", "", "")
}

func (bc *Blockchain_struct) AddPrivateBridgeRequest(tx *Transaction, toBSC string) {
	note, proof, nullifier, root, err := buildShieldedBridgeMaterial("lqd-lock", tx.From, toBSC, AmountString(tx.Value), "LQD", "BSC", tx.TxHash)
	if err != nil {
		return
	}
	bc.addBridgeRequestMode(tx, toBSC, BridgeStatusQueued, "private", "lqd-lock", note, proof, nullifier, root, "LQD", "BSC", "", "")
}

func (bc *Blockchain_struct) AddBridgeRequestWithRoute(tx *Transaction, toAddr string, status string, mode string, shieldKind string, note string, proof string, nullifier string, root string, sourceChainID string, targetChainID string, sourceChain string, targetChain string) {
	bc.addBridgeRequestMode(tx, toAddr, status, mode, shieldKind, note, proof, nullifier, root, sourceChainID, targetChainID, sourceChain, targetChain)
}

func (bc *Blockchain_struct) addBridgeRequestMode(tx *Transaction, toBSC string, status string, mode string, shieldKind string, note string, proof string, nullifier string, root string, sourceChainID string, targetChainID string, sourceChain string, targetChain string) {
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
	family, adapter := bridgeFamilyAndAdapterForChainIDs(sourceChainID, targetChainID)
	bc.BridgeRequests[id] = &BridgeRequest{
		ID:                id,
		Mode:              mode,
		Family:            family,
		Adapter:           adapter,
		From:              tx.From,
		To:                toBSC,
		Amount:            AmountString(tx.Value),
		Token:             "LQD",
		SourceChain:       sourceChain,
		TargetChain:       targetChain,
		SourceChainID:     sourceChainID,
		TargetChainID:     targetChainID,
		Status:            status,
		LqdTxHash:         tx.TxHash,
		ShieldedKind:      shieldKind,
		PrivacyCommitment: note,
		ShieldedNote:      note,
		ShieldedProof:     proof,
		ShieldedNullifier: nullifier,
		ShieldedRoot:      root,
		QueuedAt:          now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

// AddBridgeRequestBSC records a BSC->LQD lock (token on BSC, mint on LQD).
func (bc *Blockchain_struct) AddBridgeRequestBSC(bscTx string, token string, from string, toLqd string, amount *big.Int) {
	bc.addBridgeRequestBSCMode(bscTx, token, from, toLqd, amount, "public", "BSC", "LQD", "", "", "", "", "")
}

func (bc *Blockchain_struct) AddPrivateBridgeRequestBSC(bscTx string, token string, from string, toLqd string, amount *big.Int) {
	bc.addBridgeRequestBSCMode(bscTx, token, from, toLqd, amount, "private", "BSC", "LQD", "", "", "", "", "")
}

func (bc *Blockchain_struct) AddBridgeRequestChain(chainID string, txHash string, token string, from string, toLqd string, amount *big.Int) {
	bc.addBridgeRequestBSCMode(txHash, token, from, toLqd, amount, "public", chainID, "LQD", "", "", "", "", "")
}

func (bc *Blockchain_struct) AddPrivateBridgeRequestChain(chainID string, txHash string, token string, from string, toLqd string, amount *big.Int) {
	bc.addBridgeRequestBSCMode(txHash, token, from, toLqd, amount, "private", chainID, "LQD", "", "", "", "", "")
}

func (bc *Blockchain_struct) AddBridgeRequestChainWithMetadata(chainID string, txHash string, token string, from string, toLqd string, amount *big.Int, sourceAddress string, sourceMemo string, sourceSequence string, sourceOutput string) {
	bc.addBridgeRequestBSCMode(txHash, token, from, toLqd, amount, "public", chainID, "LQD", sourceAddress, sourceMemo, sourceSequence, sourceOutput, "")
}

func (bc *Blockchain_struct) AddPrivateBridgeRequestChainWithMetadata(chainID string, txHash string, token string, from string, toLqd string, amount *big.Int, sourceAddress string, sourceMemo string, sourceSequence string, sourceOutput string) {
	bc.addBridgeRequestBSCMode(txHash, token, from, toLqd, amount, "private", chainID, "LQD", sourceAddress, sourceMemo, sourceSequence, sourceOutput, "")
}

func (bc *Blockchain_struct) addBridgeRequestBSCMode(bscTx string, token string, from string, toLqd string, amount *big.Int, mode string, sourceChainID string, targetChainID string, sourceAddress string, sourceMemo string, sourceSequence string, sourceOutput string, shieldKind string) {
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
	note, proof, nullifier, root, err := buildShieldedBridgeMaterial("bsc-lock", from, toLqd, AmountString(amount), token, "BSC", bscTx)
	if err != nil {
		return
	}
	family, adapter := bridgeFamilyAndAdapterForChainIDs(sourceChainID, targetChainID)
	bc.BridgeRequests[id] = &BridgeRequest{
		ID:                id,
		Mode:              mode,
		Family:            family,
		Adapter:           adapter,
		From:              from,
		To:                toLqd,
		Amount:            AmountString(amount),
		Token:             strings.ToLower(token),
		SourceChain:       sourceChainID,
		TargetChain:       targetChainID,
		SourceChainID:     sourceChainID,
		TargetChainID:     targetChainID,
		Status:            BridgeStatusQueued,
		BscTxHash:         bscTx,
		ShieldedKind:      "bsc-lock",
		PrivacyCommitment: note,
		ShieldedNote:      note,
		ShieldedProof:     proof,
		ShieldedNullifier: nullifier,
		ShieldedRoot:      root,
		SourceTxHash:      bscTx,
		SourceAddress:     sourceAddress,
		SourceMemo:        sourceMemo,
		SourceSequence:    sourceSequence,
		SourceOutput:      sourceOutput,
		QueuedAt:          now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

// AddBridgeRequestBurn records an LQD burn request for release on BSC.
func (bc *Blockchain_struct) AddBridgeRequestBurn(lqdTx string, token string, from string, toBsc string, amount *big.Int) {
	bc.addBridgeRequestBurnMode(lqdTx, token, from, toBsc, amount, "public", "LQD", "BSC", "", "", "", "", "")
}

func (bc *Blockchain_struct) AddPrivateBridgeRequestBurn(lqdTx string, token string, from string, toBsc string, amount *big.Int) {
	bc.addBridgeRequestBurnMode(lqdTx, token, from, toBsc, amount, "private", "LQD", "BSC", "", "", "", "", "")
}

func (bc *Blockchain_struct) AddBridgeRequestBurnToChain(chainID string, lqdTx string, token string, from string, toAddr string, amount *big.Int) {
	bc.addBridgeRequestBurnMode(lqdTx, token, from, toAddr, amount, "public", "LQD", chainID, "", "", "", "", "")
}

func (bc *Blockchain_struct) AddPrivateBridgeRequestBurnToChain(chainID string, lqdTx string, token string, from string, toAddr string, amount *big.Int) {
	bc.addBridgeRequestBurnMode(lqdTx, token, from, toAddr, amount, "private", "LQD", chainID, "", "", "", "", "")
}

func (bc *Blockchain_struct) AddBridgeRequestBurnToChainWithMetadata(chainID string, lqdTx string, token string, from string, toAddr string, amount *big.Int, sourceAddress string, sourceMemo string, sourceSequence string, sourceOutput string) {
	bc.addBridgeRequestBurnMode(lqdTx, token, from, toAddr, amount, "public", "LQD", chainID, sourceAddress, sourceMemo, sourceSequence, sourceOutput, "")
}

func (bc *Blockchain_struct) AddPrivateBridgeRequestBurnToChainWithMetadata(chainID string, lqdTx string, token string, from string, toAddr string, amount *big.Int, sourceAddress string, sourceMemo string, sourceSequence string, sourceOutput string) {
	bc.addBridgeRequestBurnMode(lqdTx, token, from, toAddr, amount, "private", "LQD", chainID, sourceAddress, sourceMemo, sourceSequence, sourceOutput, "")
}

func (bc *Blockchain_struct) addBridgeRequestBurnMode(lqdTx string, token string, from string, toBsc string, amount *big.Int, mode string, sourceChainID string, targetChainID string, sourceAddress string, sourceMemo string, sourceSequence string, sourceOutput string, shieldKind string) {
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
	note, proof, nullifier, root, err := buildShieldedBridgeMaterial("lqd-burn", from, toBsc, AmountString(amount), token, "LQD", lqdTx)
	if err != nil {
		return
	}
	family, adapter := bridgeFamilyAndAdapterForChainIDs(sourceChainID, targetChainID)
	bc.BridgeRequests[id] = &BridgeRequest{
		ID:                id,
		Mode:              mode,
		Family:            family,
		Adapter:           adapter,
		From:              from,
		To:                toBsc,
		Amount:            AmountString(amount),
		Token:             strings.ToLower(token),
		SourceChain:       sourceChainID,
		TargetChain:       targetChainID,
		SourceChainID:     sourceChainID,
		TargetChainID:     targetChainID,
		Status:            BridgeStatusQueued,
		LqdTxHash:         lqdTx,
		ShieldedKind:      "lqd-burn",
		PrivacyCommitment: note,
		ShieldedNote:      note,
		ShieldedProof:     proof,
		ShieldedNullifier: nullifier,
		ShieldedRoot:      root,
		SourceTxHash:      lqdTx,
		SourceAddress:     sourceAddress,
		SourceMemo:        sourceMemo,
		SourceSequence:    sourceSequence,
		SourceOutput:      sourceOutput,
		QueuedAt:          now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

func (bc *Blockchain_struct) MarkBridgeMinted(id string, bscTx string, lqdTx string) {
	bc.updateBridgeRequest(id, func(req *BridgeRequest) {
		req.Status = BridgeStatusMinted
		if bscTx != "" {
			req.BscTxHash = bscTx
		}
		if lqdTx != "" {
			req.LqdTxHash = lqdTx
		}
		req.ProcessedAt = time.Now().Unix()
	})
}

func (bc *Blockchain_struct) MarkBridgeUnlocked(id string) {
	bc.updateBridgeRequest(id, func(req *BridgeRequest) {
		req.Status = BridgeStatusUnlock
		req.ProcessedAt = time.Now().Unix()
	})
}

func (bc *Blockchain_struct) MarkBridgeProcessing(id string, bscTx string, lqdTx string) {
	bc.updateBridgeRequest(id, func(req *BridgeRequest) {
		req.Status = BridgeStatusProcessing
		if bscTx != "" {
			req.BscTxHash = bscTx
		}
		if lqdTx != "" {
			req.LqdTxHash = lqdTx
		}
	})
}

func (bc *Blockchain_struct) MarkBridgeBatchProcessing(id string, batchID string, bscTx string, lqdTx string) {
	bc.updateBridgeRequest(id, func(req *BridgeRequest) {
		req.Status = BridgeStatusProcessing
		if batchID != "" {
			req.BatchID = batchID
		}
		if bscTx != "" {
			req.BscTxHash = bscTx
		}
		if lqdTx != "" {
			req.LqdTxHash = lqdTx
		}
	})
}

func (bc *Blockchain_struct) MarkBridgeFailed(id string) {
	bc.updateBridgeRequest(id, func(req *BridgeRequest) {
		req.Status = BridgeStatusFailed
	})
}

func (bc *Blockchain_struct) updateBridgeRequest(id string, fn func(*BridgeRequest)) {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	if bc.BridgeRequests == nil {
		return
	}
	req, ok := bc.BridgeRequests[strings.ToLower(id)]
	if !ok || req == nil {
		return
	}
	fn(req)
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

func (bc *Blockchain_struct) ListBridgeRequestsView(address string) []*BridgeRequest {
	raw := bc.ListBridgeRequests(address)
	out := make([]*BridgeRequest, 0, len(raw))
	for _, r := range raw {
		out = append(out, redactBridgeRequest(r))
	}
	return out
}

func redactBridgeRequest(r *BridgeRequest) *BridgeRequest {
	if r == nil {
		return nil
	}
	cp := *r
	if strings.EqualFold(cp.Mode, "private") {
		cp.ID = "private"
		cp.From = "private"
		cp.To = "private"
		cp.Amount = "private"
		cp.Token = "private"
		cp.LqdTxHash = "private"
		cp.BscTxHash = "private"
		cp.SourceTxHash = "private"
		cp.SourceAddress = "private"
		cp.SourceMemo = "private"
		cp.SourceSequence = "private"
		cp.SourceOutput = "private"
		cp.ShieldedKind = "private"
		cp.PrivacyCommitment = "private"
		cp.ShieldedNote = "private"
		cp.ShieldedProof = "private"
		cp.ShieldedNullifier = "private"
		cp.ShieldedRoot = "private"
		cp.BatchID = "private"
		cp.CreatedAt = 0
		cp.UpdatedAt = 0
		cp.QueuedAt = 0
		cp.ProcessedAt = 0
	}
	return &cp
}

func bridgeShieldTxHash(r *BridgeRequest) string {
	if r == nil {
		return ""
	}
	if r.LqdTxHash != "" {
		return r.LqdTxHash
	}
	return r.BscTxHash
}

func markShieldedNullifierSpent(nullifier string) {
	if nullifier == "" {
		return
	}
	shieldedBridgeState.Lock()
	defer shieldedBridgeState.Unlock()
	if shieldedBridgeState.SpentNullifiers == nil {
		shieldedBridgeState.SpentNullifiers = make(map[string]bool)
	}
	shieldedBridgeState.SpentNullifiers[strings.ToLower(nullifier)] = true
}

func registerShieldedBridgeCommitment(commitment string, root string) {
	if commitment == "" {
		return
	}
	shieldedBridgeState.Lock()
	defer shieldedBridgeState.Unlock()
	shieldedBridgeState.Commitments = append(shieldedBridgeState.Commitments, strings.ToLower(commitment))
	if root != "" {
		shieldedBridgeState.Root = strings.ToLower(root)
	} else {
		shieldedBridgeState.Root = privacyCommitment("shield-accumulator", shieldedBridgeState.Root, commitment)
	}
}

func (bc *Blockchain_struct) MarkBridgeQueued(id string, batchID string) {
	bc.updateBridgeRequest(id, func(req *BridgeRequest) {
		req.Status = BridgeStatusQueued
		req.BatchID = batchID
	})
}

func privacyCommitment(parts ...string) string {
	h := sha256.Sum256([]byte(strings.ToLower(strings.Join(parts, "|"))))
	return "0x" + hex.EncodeToString(h[:])
}

func BridgeEscrowAddress() string {
	return constantset.BridgeEscrowAddress
}
