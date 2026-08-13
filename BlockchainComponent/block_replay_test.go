package blockchaincomponent

import (
	"fmt"
	"math/big"
	"reflect"
	"testing"
	"time"

	constantset "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/ConstantSet"
)

func buildReplayFixture(t *testing.T) (*Blockchain_struct, *Block) {
	t.Helper()
	bc := newTestBlockchain()
	validator := &Validator{Address: "0x1111111111111111111111111111111111111111", NativeBond: 1e12, LPStakeAmount: 1e12, LockTime: time.Now().Add(time.Hour)}
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
	block.StateRoot = shadow.ComputeDeterministicStateRootAt(block.BlockNumber)
	block.CurrentHash = CalculateHash(&block)
	return bc, &block
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
	bc := newTestBlockchain()
	for i := 1; i <= 4; i++ {
		bc.Validators = append(bc.Validators, &Validator{Address: fmt.Sprintf("0x%040x", i), NativeBond: 1e12, LPStakeAmount: 1e12, LockTime: time.Now().Add(time.Hour)})
	}
	bc.EnsureRuntimeState()
	bc.ChainSpec.AllowLegacyFinality = false
	bc.PrepareValidatorSetTransition(1)
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
