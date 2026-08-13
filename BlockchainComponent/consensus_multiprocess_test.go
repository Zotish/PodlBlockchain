package blockchaincomponent

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
)

const validatorProcessPrefix = "PODL_VALIDATOR_VOTE="

// TestValidatorSignerHelperProcess runs only in a child OS process. Keeping
// signing out of the coordinator catches serialization, key-domain and
// process-boundary failures that an in-memory chaos model cannot detect.
func TestValidatorSignerHelperProcess(t *testing.T) {
	if os.Getenv("PODL_VALIDATOR_HELPER") != "1" {
		return
	}
	height, err := strconv.ParseUint(os.Getenv("PODL_VOTE_HEIGHT"), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	round, err := strconv.ParseUint(os.Getenv("PODL_VOTE_ROUND"), 10, 32)
	if err != nil {
		t.Fatal(err)
	}
	vote := ConsensusVote{Height: height, Round: uint32(round), Step: ConsensusStep(os.Getenv("PODL_VOTE_STEP")), BlockHash: os.Getenv("PODL_VOTE_HASH")}
	if err := SignConsensusVote(&vote, os.Getenv("PODL_VALIDATOR_KEY")); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(vote)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(validatorProcessPrefix + string(raw))
}

func processSignedVote(t *testing.T, key string, height uint64, round uint32, step ConsensusStep, blockHash string) ConsensusVote {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestValidatorSignerHelperProcess$")
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
		t.Fatalf("validator subprocess failed: %v\n%s", err, out)
	}
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	var vote ConsensusVote
	found := false
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, validatorProcessPrefix) {
			continue
		}
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, validatorProcessPrefix)), &vote); err != nil {
			t.Fatal(err)
		}
		found = true
	}
	if !found || !VerifyConsensusVote(vote) {
		t.Fatalf("subprocess did not return a valid vote: %s", out)
	}
	return vote
}

func TestBFTFourToTwentyIndependentSignerProcesses(t *testing.T) {
	if raceDetectorEnabled {
		t.Skip("process isolation has no shared-memory race surface; the release gate runs this harness separately without -race")
	}
	for n := 4; n <= 20; n++ {
		t.Run(fmt.Sprintf("validators_%d", n), func(t *testing.T) {
			bc := newTestBlockchain()
			keys := make([]string, n)
			const testLiquidityBalance = float64(1_000_000_000_000_000_000)
			for i := 0; i < n; i++ {
				key, err := crypto.GenerateKey()
				if err != nil {
					t.Fatal(err)
				}
				keys[i] = hex.EncodeToString(crypto.FromECDSA(key))
				bc.Validators = append(bc.Validators, &Validator{Address: crypto.PubkeyToAddress(key.PublicKey).Hex(), NativeBond: 1e12, LPStakeAmount: testLiquidityBalance, LiquidityPower: 1e6, LockTime: time.Now().Add(24 * time.Hour)})
			}
			bc.EnsureRuntimeState()
			bc.PrepareValidatorSetTransition(1)

			faults := (n - 1) / 3
			for i := 0; i < faults; i++ {
				qc, _, err := bc.AddConsensusVote(processSignedVote(t, keys[i], 1, 0, StepPrevote, "0xpartition"))
				if err != nil || qc != nil {
					t.Fatalf("Byzantine partition formed QC: qc=%v err=%v", qc, err)
				}
			}
			var qc *QuorumCertificate
			for i := faults; i < n; i++ {
				var err error
				qc, _, err = bc.AddConsensusVote(processSignedVote(t, keys[i], 1, 0, StepPrevote, "0xfinal"))
				if err != nil {
					t.Fatal(err)
				}
			}
			if qc == nil || qc.BlockHash != "0xfinal" || qc.VotingPower < qc.RequiredPower {
				t.Fatalf("independent honest processes failed to prevote: %#v", qc)
			}
			for i := faults; i < n; i++ {
				var err error
				qc, _, err = bc.AddConsensusVote(processSignedVote(t, keys[i], 1, 0, StepPrecommit, "0xfinal"))
				if err != nil {
					t.Fatal(err)
				}
			}
			if qc == nil || qc.Step != StepPrecommit || qc.Randomness == "" || bc.ConsensusV2.LastQC == nil {
				t.Fatalf("independent honest processes failed to finalize precommit QC: %#v", qc)
			}
		})
	}
}
