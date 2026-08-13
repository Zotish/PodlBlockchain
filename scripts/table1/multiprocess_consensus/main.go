package main

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	bc "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/BlockchainComponent"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	votePrefix   = "PODL_VALIDATOR_VOTE="
	tokenAddress = "0x7000000000000000000000000000000000000007"
	claimAmount  = "1000000000000000000"
)

type report struct {
	ValidatorsMin      int     `json:"validators_min"`
	ValidatorsMax      int     `json:"validators_max"`
	ValidatorSets      int     `json:"validator_sets_tested"`
	FaucetClaims       int     `json:"test_liquidity_claims"`
	TotalSupply        string  `json:"test_liquidity_total_supply"`
	IndependentVotes   int     `json:"independent_process_votes"`
	FinalizedQCs       int     `json:"precommit_qcs"`
	ByzantineRejected  bool    `json:"byzantine_minority_rejected"`
	ElapsedSeconds     float64 `json:"elapsed_seconds"`
	SignedVotesPerSec  float64 `json:"signed_votes_per_second"`
	FinalizedQCsPerSec float64 `json:"finalized_qcs_per_second"`
}

func fatal(err error) {
	if err != nil {
		panic(err)
	}
}

func childSigner() bool {
	if os.Getenv("PODL_VALIDATOR_HELPER") != "1" {
		return false
	}
	height, err := strconv.ParseUint(os.Getenv("PODL_VOTE_HEIGHT"), 10, 64)
	fatal(err)
	round, err := strconv.ParseUint(os.Getenv("PODL_VOTE_ROUND"), 10, 32)
	fatal(err)
	vote := bc.ConsensusVote{Height: height, Round: uint32(round), Step: bc.ConsensusStep(os.Getenv("PODL_VOTE_STEP")), BlockHash: os.Getenv("PODL_VOTE_HASH")}
	fatal(bc.SignConsensusVote(&vote, os.Getenv("PODL_VALIDATOR_KEY")))
	raw, err := json.Marshal(vote)
	fatal(err)
	fmt.Println(votePrefix + string(raw))
	return true
}

func processVote(key string, height uint64, round uint32, step bc.ConsensusStep, blockHash string) (bc.ConsensusVote, error) {
	executable, err := os.Executable()
	if err != nil {
		return bc.ConsensusVote{}, err
	}
	cmd := exec.Command(executable)
	cmd.Env = append(os.Environ(),
		"PODL_VALIDATOR_HELPER=1",
		"PODL_VALIDATOR_KEY="+key,
		"PODL_VOTE_HEIGHT="+strconv.FormatUint(height, 10),
		"PODL_VOTE_ROUND="+strconv.FormatUint(uint64(round), 10),
		"PODL_VOTE_STEP="+string(step),
		"PODL_VOTE_HASH="+blockHash,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return bc.ConsensusVote{}, fmt.Errorf("signer process: %w: %s", err, out)
	}
	var vote bc.ConsensusVote
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), votePrefix) {
			if err := json.Unmarshal([]byte(strings.TrimPrefix(scanner.Text(), votePrefix)), &vote); err != nil {
				return vote, err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return vote, err
	}
	if !bc.VerifyConsensusVote(vote) {
		return vote, fmt.Errorf("child returned invalid signed vote")
	}
	return vote, nil
}

func main() {
	if childSigner() {
		return
	}
	started := time.Now()
	root, err := os.Getwd()
	fatal(err)
	dataDir, err := os.MkdirTemp("", "podl-process-consensus-")
	fatal(err)
	defer os.RemoveAll(dataDir)
	fatal(os.Setenv("LQD_DATA_DIR", dataDir))

	engine, err := bc.NewLQDContractEngine()
	fatal(err)
	engine.Registry.Blockchain = &bc.Blockchain_struct{Accounts: map[string]*big.Int{}}
	engine.Registry.Blockchain.EnsureRuntimeState()
	pluginPath := filepath.Join(root, "bin", "builtins", "test_liquidity_token.so")
	fatal(engine.Registry.DeployClone(tokenAddress, pluginPath, tokenAddress))
	call := func(caller, method string, args ...string) *bc.ContractExecutionResult {
		result, err := engine.Pipeline.ExecuteAtomic(tokenAddress, caller, method, args, 5_000_000, big.NewInt(0))
		fatal(err)
		return result
	}
	call(tokenAddress, "Init", "20000000000000000000", claimAmount)

	keys, addresses := make([]string, 20), make([]string, 20)
	for i := range keys {
		key, err := crypto.GenerateKey()
		fatal(err)
		keys[i] = hex.EncodeToString(crypto.FromECDSA(key))
		addresses[i] = crypto.PubkeyToAddress(key.PublicKey).Hex()
		call(addresses[i], "Claim")
		if balance := strings.TrimSpace(call(addresses[i], "BalanceOf", addresses[i]).Output); balance != claimAmount {
			panic(fmt.Sprintf("validator %d faucet balance mismatch: %s", i, balance))
		}
	}
	supply := strings.TrimSpace(call(tokenAddress, "TotalSupply").Output)
	if supply != "20000000000000000000" {
		panic("test-liquidity total supply mismatch: " + supply)
	}

	result := report{ValidatorsMin: 4, ValidatorsMax: 20, ValidatorSets: 17, FaucetClaims: 20, TotalSupply: supply, ByzantineRejected: true}
	for n := 4; n <= 20; n++ {
		chain := &bc.Blockchain_struct{Accounts: map[string]*big.Int{}}
		for i := 0; i < n; i++ {
			chain.Validators = append(chain.Validators, &bc.Validator{Address: addresses[i], NativeBond: 1e12, LPStakeAmount: 1e18, LiquidityPower: 1e6, LockTime: time.Now().Add(24 * time.Hour)})
		}
		chain.EnsureRuntimeState()
		chain.PrepareValidatorSetTransition(1)
		faults := (n - 1) / 3
		for i := 0; i < faults; i++ {
			vote, err := processVote(keys[i], 1, 0, bc.StepPrevote, "0xpartition")
			fatal(err)
			qc, _, err := chain.AddConsensusVote(vote)
			fatal(err)
			result.IndependentVotes++
			if qc != nil {
				panic(fmt.Sprintf("%d-validator Byzantine minority formed QC", n))
			}
		}
		var qc *bc.QuorumCertificate
		for i := faults; i < n; i++ {
			vote, err := processVote(keys[i], 1, 0, bc.StepPrevote, "0xfinal")
			fatal(err)
			qc, _, err = chain.AddConsensusVote(vote)
			fatal(err)
			result.IndependentVotes++
		}
		if qc == nil || qc.Step != bc.StepPrevote {
			panic(fmt.Sprintf("%d-validator prevote QC missing", n))
		}
		for i := faults; i < n; i++ {
			vote, err := processVote(keys[i], 1, 0, bc.StepPrecommit, "0xfinal")
			fatal(err)
			qc, _, err = chain.AddConsensusVote(vote)
			fatal(err)
			result.IndependentVotes++
		}
		if qc == nil || qc.Step != bc.StepPrecommit || qc.Randomness == "" || chain.ConsensusV2.LastQC == nil {
			panic(fmt.Sprintf("%d-validator finality QC missing", n))
		}
		result.FinalizedQCs++
	}
	result.ElapsedSeconds = time.Since(started).Seconds()
	if result.ElapsedSeconds > 0 {
		result.SignedVotesPerSec = float64(result.IndependentVotes) / result.ElapsedSeconds
		result.FinalizedQCsPerSec = float64(result.FinalizedQCs) / result.ElapsedSeconds
	}
	raw, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(raw))
}
