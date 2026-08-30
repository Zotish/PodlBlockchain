package blockchaincomponent

import (
	"encoding/json"
	"math"
	"math/big"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	constantset "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/ConstantSet"
	"github.com/ethereum/go-ethereum/crypto"
)

func largeReplayHistoryFixture() *Blockchain_struct {
	bc := newTestBlockchain()
	bc.EnsureRuntimeState()
	for i := 1; i <= 1024; i++ {
		block := &Block{
			BlockNumber:  uint64(i),
			PreviousHash: bc.Blocks[len(bc.Blocks)-1].CurrentHash,
			CurrentHash:  "0x" + strings.Repeat("a", 63) + string(rune('a'+i%26)),
			TimeStamp:    bc.Blocks[len(bc.Blocks)-1].TimeStamp + 2,
			Transactions: []*Transaction{},
		}
		bc.Blocks = append(bc.Blocks, block)
	}
	bc.RewardHistory = make([]RewardSnapshot, 10000)
	bc.RewardLedger = make([]RewardLedgerEntry, 100000)
	bc.RecentTxs = make([]*Transaction, 50000)
	for i := range bc.RewardLedger {
		bc.RewardLedger[i] = RewardLedgerEntry{ID: "ledger-" + string(rune(i%127)), Address: "0x1111111111111111111111111111111111111111", Amount: "1"}
	}
	for i := range bc.RecentTxs {
		bc.RecentTxs[i] = &Transaction{TxHash: "tx-" + string(rune(i%127)), Type: "transfer"}
	}
	return bc
}

func TestCloneForBlockReplayBoundsOperationalHistory(t *testing.T) {
	bc := largeReplayHistoryFixture()
	beforeRoot := bc.ComputeDeterministicStateRootAt(bc.LatestBlockNumber())
	beforeBlocks := len(bc.Blocks)
	beforeLedger := len(bc.RewardLedger)

	shadow, _, err := bc.cloneForBlockReplay()
	if err != nil {
		t.Fatal(err)
	}
	if len(shadow.Blocks) != replayRecentBlockWindow {
		t.Fatalf("replay blocks = %d, want %d", len(shadow.Blocks), replayRecentBlockWindow)
	}
	if shadow.LatestBlockNumber() != bc.LatestBlockNumber() {
		t.Fatalf("replay tip = %d, want %d", shadow.LatestBlockNumber(), bc.LatestBlockNumber())
	}
	if len(shadow.RewardHistory) != 0 || len(shadow.RewardLedger) != 0 || len(shadow.RecentTxs) != 0 {
		t.Fatalf("operational history leaked into replay: history=%d ledger=%d recent=%d", len(shadow.RewardHistory), len(shadow.RewardLedger), len(shadow.RecentTxs))
	}
	if len(shadow.BlockVotes) != 0 || len(shadow.PendingBlocks) != 0 || len(shadow.PendingBlockSeenAt) != 0 {
		t.Fatal("local pending consensus caches leaked into replay")
	}
	if got := shadow.ComputeDeterministicStateRootAt(shadow.LatestBlockNumber()); got != beforeRoot {
		t.Fatalf("bounded replay changed consensus root: got %s want %s", got, beforeRoot)
	}
	if len(bc.Blocks) != beforeBlocks || len(bc.RewardLedger) != beforeLedger {
		t.Fatal("building replay snapshot mutated canonical history")
	}
}

func BenchmarkReplayCloneLargeOperationalHistory(b *testing.B) {
	bc := largeReplayHistoryFixture()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, err := bc.cloneForBlockReplay(); err != nil {
			b.Fatal(err)
		}
	}
}

// This benchmark models the pre-fix full-struct JSON replay boundary so the
// age-independent clone can be compared against the former behavior.
func BenchmarkLegacyFullReplayMarshalLargeOperationalHistory(b *testing.B) {
	bc := largeReplayHistoryFixture()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := json.Marshal(bc); err != nil {
			b.Fatal(err)
		}
	}
}

func buildReplayFixtureWithPenalty(t *testing.T, penalty float64, reason string) (*Blockchain_struct, *Block) {
	t.Helper()
	bc := newTestBlockchain()
	validator := &Validator{Address: "0x1111111111111111111111111111111111111111", NativeBond: 1e12, LPStakeAmount: 1e12, LockTime: time.Now().Add(time.Hour), PenaltyScore: penalty, SlashReason: reason}
	bc.Validators = append(bc.Validators, validator)
	bc.EnsureRuntimeState()
	parent := bc.Blocks[0]
	block := NewBlock(parent.BlockNumber, parent.CurrentHash)
	block.TimeStamp = parent.TimeStamp
	block.BaseFee = bc.CalculateBaseFee()
	block.ParentStateRoot = bc.ComputeDeterministicStateRootAt(parent.BlockNumber)
	proof, err := bc.BuildProposerCertificate(block.BlockNumber, block.ConsensusRound)
	if err != nil {
		t.Fatal(err)
	}
	block.ProposerProof = proof
	shadow, _, err := bc.cloneForBlockReplay()
	if err != nil {
		t.Fatal(err)
	}
	block.RewardBreakdown = shadow.CalculateBlockRewards(validator.Address, nil, 0, block.BlockNumber)
	reward := &Transaction{From: "0x0000000000000000000000000000000000000000", To: validator.Address, Value: NewAmountFromStringOrZero(block.RewardBreakdown.ValidatorReward), Type: "reward", IsSystem: true, Status: constantset.StatusSuccess, Timestamp: block.TimeStamp, GasPrice: 0}
	reward.TxHash = CalculateTransactionHash(*reward)
	block.Transactions = []*Transaction{reward}
	shadow.applyValidatorCleanUptimeRecovery(validator.Address)
	block.StateRoot = shadow.ComputeDeterministicStateRootAt(block.BlockNumber)
	block.CurrentHash = CalculateHash(&block)
	return bc, &block
}

func buildReplayFixture(t *testing.T) (*Blockchain_struct, *Block) {
	t.Helper()
	return buildReplayFixtureWithPenalty(t, 0, "")
}

func TestIncomingBlockReplayMatchesAndStagesPostState(t *testing.T) {
	bc, block := buildReplayFixture(t)
	if !bc.VerifySingleBlock(block) {
		t.Fatal("valid replayed block rejected")
	}
	transition := bc.PendingReplayTransitions[block.CurrentHash]
	if transition == nil {
		t.Fatal("verified post-state was not staged")
	}
	if transition.ReferenceStateRoot == "" || transition.ReferenceStateRoot != transition.PostStateRoot {
		t.Fatal("independent reference replay was not compared before staging")
	}
}

func TestIncomingBlockReplayRecoversOnlyLivenessPenalty(t *testing.T) {
	bc, block := buildReplayFixtureWithPenalty(t, 0.95, "inactivity")
	if !bc.VerifySingleBlock(block) {
		t.Fatal("valid liveness-recovery block rejected")
	}
	transition := bc.PendingReplayTransitions[block.CurrentHash]
	if transition == nil || len(transition.Validators) != 1 {
		t.Fatal("liveness-recovery transition was not staged")
	}
	if got, want := transition.Validators[0].PenaltyScore, 0.94; math.Abs(got-want) > 0.000000001 {
		t.Fatalf("unexpected recovered penalty: got %.8f want %.8f", got, want)
	}
	if !transition.Validators[0].JailedUntil.IsZero() {
		t.Fatal("successful finalized participation did not release liveness jail")
	}

	bc, block = buildReplayFixtureWithPenalty(t, 0.2, "double signing")
	if !bc.VerifySingleBlock(block) {
		t.Fatal("valid safety-penalty block rejected")
	}
	transition = bc.PendingReplayTransitions[block.CurrentHash]
	if got := transition.Validators[0].PenaltyScore; got != 0.2 {
		t.Fatalf("safety penalty was incorrectly recovered: got %.8f", got)
	}
}

func TestIncomingBlockReplayRejectsForgedStateRootAndReward(t *testing.T) {
	bc, block := buildReplayFixture(t)
	block.StateRoot = "0xdeadbeef"
	block.CurrentHash = ""
	block.CurrentHash = CalculateHash(block)
	if bc.VerifySingleBlock(block) {
		t.Fatal("forged state root accepted")
	}

	bc, block = buildReplayFixture(t)
	block.Transactions[0].Value = new(big.Int).Add(block.Transactions[0].Value, big.NewInt(1))
	block.Transactions[0].TxHash = CalculateTransactionHash(*block.Transactions[0])
	block.CurrentHash = ""
	block.CurrentHash = CalculateHash(block)
	if bc.VerifySingleBlock(block) {
		t.Fatal("forged reward transaction accepted")
	}
}

func TestLocalProposalDoesNotCommitSpeculativeStateBeforeQC(t *testing.T) {
	bc, keys := consensusFixture(t, 4)
	bc.ChainSpec.AllowLegacyFinality = false
	proposer, err := bc.SelectBlockProposer(1, 0)
	if err != nil {
		t.Fatal(err)
	}
	var proposerKey string
	for _, keyHex := range keys {
		key, keyErr := crypto.HexToECDSA(keyHex)
		if keyErr == nil && strings.EqualFold(crypto.PubkeyToAddress(key.PublicKey).Hex(), proposer.Address) {
			proposerKey = keyHex
			break
		}
	}
	if proposerKey == "" {
		t.Fatal("selected proposer key missing")
	}
	signer, err := NewLocalValidatorSigner(proposerKey, testP256VRFSecret, filepath.Join(t.TempDir(), "slashing.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer signer.Close()
	bc.Network = NewNetworkService(bc)
	if err := bc.Network.SetValidatorSigner(proposer.Address, signer); err != nil {
		t.Fatal(err)
	}
	bc.LocalValidator = proposer.Address
	beforeRoot := bc.ComputeDeterministicStateRootAt(0)
	beforeEmission := CopyAmount(bc.CumulativeEmission)
	if block := bc.MineNewBlock(); block != nil {
		t.Fatal("strict BFT proposal finalized without signed quorum")
	}
	if got := bc.ComputeDeterministicStateRootAt(0); got != beforeRoot {
		t.Fatalf("pending local proposal mutated finalized state: before=%s after=%s", beforeRoot, got)
	}
	if bc.CumulativeEmission.Cmp(beforeEmission) != 0 {
		t.Fatal("pending local proposal consumed issuance")
	}
	if len(bc.PendingBlocks) != 1 || len(bc.PendingReplayTransitions) != 1 {
		t.Fatal("isolated proposal or staged state was not retained for later QC")
	}
	var retainedHash string
	for hash := range bc.PendingBlocks {
		retainedHash = hash
	}
	if block := bc.MineNewBlock(); block != nil {
		t.Fatal("retained proposal finalized without signed quorum")
	}
	if len(bc.PendingBlocks) != 1 || bc.PendingBlocks[retainedHash] == nil {
		t.Fatal("retry rebuilt or discarded the retained height/round proposal")
	}
}

func TestBlockReplayReusesProcessGlobalPluginVM(t *testing.T) {
	bc := newTestBlockchain()
	parentDB := NewOverlayContractDB(nil)
	registry := NewContractRegistry(parentDB, nil)
	address := "0x1111111111111111111111111111111111111111"
	contract := &PluginContract{Instance: struct{}{}, Methods: map[string]reflect.Method{}}
	registry.PluginVM.mu.Lock()
	registry.PluginVM.plugins[address] = contract
	registry.PluginVM.mu.Unlock()
	bc.ContractEngine = &LQDContractEngine{
		DB:       parentDB,
		Registry: registry,
		Pipeline: NewExecutionPipeline(registry),
	}

	shadow, _, err := bc.cloneForBlockReplay()
	if err != nil {
		t.Fatal(err)
	}
	if shadow.ContractEngine.Registry.PluginVM != registry.PluginVM {
		t.Fatal("replay created a fresh process-global plugin cache")
	}
	if got := shadow.ContractEngine.Registry.PluginVM.GetPlugin(address); got != contract {
		t.Fatal("replay could not reuse the already-loaded plugin contract")
	}
}

func TestFinalizedLocalReplayEvictsRejectedTransactions(t *testing.T) {
	bc := newTestBlockchain()
	rejected := &Transaction{
		From:          "0x1111111111111111111111111111111111111111",
		To:            "0x2222222222222222222222222222222222222222",
		TxHash:        "0xrejected",
		Status:        constantset.StatusPending,
		FailureReason: "",
	}
	remaining := &Transaction{
		From:   rejected.From,
		To:     rejected.To,
		TxHash: "0xremaining",
		Status: constantset.StatusPending,
	}
	bc.Transaction_pool = []*Transaction{rejected, remaining}

	shadow, overlay, err := bc.cloneForBlockReplay()
	if err != nil {
		t.Fatal(err)
	}
	failedCopy := *rejected
	failedCopy.Status = constantset.StatusFailed
	failedCopy.FailureReason = "slippage protection"
	shadow.RecentTxs = []*Transaction{&failedCopy}

	const blockHash = "0xlocal-finalized"
	postRoot := shadow.ComputeDeterministicStateRootAt(1)
	transition := captureReplayTransition(blockHash, postRoot, shadow, overlay)
	if len(transition.RejectedTransactions) != 1 || transition.RejectedTransactions[0].TxHash != rejected.TxHash {
		t.Fatalf("failed proposal transaction was not captured: %#v", transition.RejectedTransactions)
	}
	bc.evictRejectedTransactions(transition.RejectedTransactions)

	if len(bc.Transaction_pool) != 1 || bc.Transaction_pool[0].TxHash != remaining.TxHash {
		t.Fatalf("rejected transaction remained in canonical mempool: %#v", bc.Transaction_pool)
	}
	if len(bc.RecentTxs) != 1 || bc.RecentTxs[0].TxHash != rejected.TxHash || bc.RecentTxs[0].Status != constantset.StatusFailed {
		t.Fatalf("rejected transaction was not retained as failed history: %#v", bc.RecentTxs)
	}
}
