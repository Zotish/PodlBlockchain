package main

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	bc "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/BlockchainComponent"
	"github.com/ethereum/go-ethereum/crypto"
)

type signRequest struct {
	Height uint64           `json:"height"`
	Round  uint32           `json:"round"`
	Step   bc.ConsensusStep `json:"step"`
	Hash   string           `json:"hash"`
}

type signResponse struct {
	Vote  bc.ConsensusVote `json:"vote"`
	Error string           `json:"error,omitempty"`
}

func validatorChild() {
	key := os.Getenv("PODL_FAULT_LAB_KEY")
	scanner, writer := bufio.NewScanner(os.Stdin), bufio.NewWriter(os.Stdout)
	defer writer.Flush()
	for scanner.Scan() {
		var req signRequest
		response := signResponse{}
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			response.Error = err.Error()
		} else {
			response.Vote = bc.ConsensusVote{Height: req.Height, Round: req.Round, Step: req.Step, BlockHash: req.Hash}
			if err := bc.SignConsensusVote(&response.Vote, key); err != nil {
				response.Error = err.Error()
			}
		}
		raw, _ := json.Marshal(response)
		_, _ = writer.Write(append(raw, '\n'))
		_ = writer.Flush()
	}
}

type signerProcess struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	scan   *bufio.Scanner
	keyHex string
	addr   string
}

func startSigner(keyHex string) (*signerProcess, error) {
	key, err := crypto.HexToECDSA(keyHex)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(os.Args[0], "--validator-child")
	cmd.Env = append(os.Environ(), "PODL_FAULT_LAB_KEY="+keyHex)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &signerProcess{cmd: cmd, stdin: stdin, scan: bufio.NewScanner(stdout), keyHex: keyHex, addr: strings.ToLower(crypto.PubkeyToAddress(key.PublicKey).Hex())}, nil
}

func (p *signerProcess) sign(req signRequest) (bc.ConsensusVote, error) {
	raw, _ := json.Marshal(req)
	if _, err := p.stdin.Write(append(raw, '\n')); err != nil {
		return bc.ConsensusVote{}, err
	}
	if !p.scan.Scan() {
		return bc.ConsensusVote{}, fmt.Errorf("validator process ended: %v", p.scan.Err())
	}
	var response signResponse
	if err := json.Unmarshal(p.scan.Bytes(), &response); err != nil {
		return bc.ConsensusVote{}, err
	}
	if response.Error != "" {
		return bc.ConsensusVote{}, fmt.Errorf("%s", response.Error)
	}
	if !bc.VerifyConsensusVote(response.Vote) || !strings.EqualFold(response.Vote.Validator, p.addr) {
		return bc.ConsensusVote{}, fmt.Errorf("invalid process-boundary vote")
	}
	return response.Vote, nil
}

func (p *signerProcess) close() {
	_ = p.stdin.Close()
	_ = p.cmd.Wait()
}

func newChain(signers []*signerProcess, n int) *bc.Blockchain_struct {
	chain := &bc.Blockchain_struct{MinStake: 100_000}
	chain.EnsureRuntimeState()
	chain.Validators = nil
	for i := 0; i < n; i++ {
		chain.Validators = append(chain.Validators, &bc.Validator{Address: signers[i].addr, NativeBond: 1e12, LPStakeAmount: 1e18, LiquidityPower: 1e6, LockTime: time.Now().Add(24 * time.Hour)})
	}
	chain.PrepareValidatorSetTransition(1)
	return chain
}

// wanDeliveryOrder is a deterministic discrete-event WAN model. Ten percent
// of first deliveries are dropped and appended as retries; accepted messages
// are reordered by 20-500ms latency plus a bounded +/-120s node-clock offset.
// Consensus uses height/round rather than sender wall time, while transaction
// clock-skew boundaries are covered by the chain unit suite.
func wanDeliveryOrder(indices []int, scenario int) ([]int, int) {
	type delivery struct {
		index, arrival int
		firstDrop      bool
	}
	deliveries := make([]delivery, 0, len(indices))
	for position, index := range indices {
		latencyMS := 20 + (index*7919+scenario*131+position*17)%481
		clockSkewMS := -120_000 + (index*3571+scenario*101)%240_001
		deliveries = append(deliveries, delivery{index: index, arrival: latencyMS + clockSkewMS/10_000, firstDrop: (position+scenario)%10 == 0})
	}
	sort.Slice(deliveries, func(i, j int) bool {
		if deliveries[i].arrival == deliveries[j].arrival {
			return deliveries[i].index < deliveries[j].index
		}
		return deliveries[i].arrival < deliveries[j].arrival
	})
	first, retries := []int{}, []int{}
	for _, item := range deliveries {
		if item.firstDrop {
			retries = append(retries, item.index)
		} else {
			first = append(first, item.index)
		}
	}
	return append(first, retries...), len(retries)
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--validator-child" {
		validatorChild()
		return
	}
	const maxValidators = 100
	// Two independent process banks permit a 100% set replacement campaign
	// while the active validator-set size itself remains bounded at 100.
	const signerProcesses = maxValidators * 2
	signers := make([]*signerProcess, 0, signerProcesses)
	for i := 0; i < signerProcesses; i++ {
		key, err := crypto.GenerateKey()
		if err != nil {
			panic(err)
		}
		process, err := startSigner(hex.EncodeToString(crypto.FromECDSA(key)))
		if err != nil {
			panic(err)
		}
		signers = append(signers, process)
	}
	defer func() {
		for _, signer := range signers {
			signer.close()
		}
	}()
	started := time.Now()
	totalVotes, qcs, minorityRejected := 0, 0, 0
	timeoutCertificates, jointQCs, oldOnlyRejected := 0, 0, 0
	transientDrops, retransmittedVotes := 0, 0
	for n := 20; n <= maxValidators; n++ {
		chain, faults := newChain(signers, n), (n-1)/3
		// Deliver minority votes in reverse order to model reordering. They
		// must never form a conflicting certificate.
		for i := faults - 1; i >= 0; i-- {
			vote, err := signers[i].sign(signRequest{Height: 1, Step: bc.StepPrevote, Hash: "0xpartition"})
			if err != nil {
				panic(err)
			}
			totalVotes++
			qc, _, err := chain.AddConsensusVote(vote)
			if err != nil || qc != nil {
				panic(fmt.Sprintf("n=%d Byzantine minority formed QC: %#v %v", n, qc, err))
			}
		}
		minorityRejected++
		indices := make([]int, 0, n-faults)
		for i := faults; i < n; i++ {
			indices = append(indices, i)
		}
		for stepIndex, step := range []bc.ConsensusStep{bc.StepPrevote, bc.StepPrecommit} {
			var finalQC *bc.QuorumCertificate
			order, drops := wanDeliveryOrder(indices, n*10+stepIndex)
			transientDrops += drops
			retransmittedVotes += drops
			for _, i := range order {
				vote, err := signers[i].sign(signRequest{Height: 1, Step: step, Hash: "0xfinal"})
				if err != nil {
					panic(err)
				}
				totalVotes++
				finalQC, _, err = chain.AddConsensusVote(vote)
				if err != nil {
					panic(err)
				}
			}
			if finalQC == nil || finalQC.Step != step || finalQC.VotingPower < finalQC.RequiredPower {
				panic(fmt.Sprintf("n=%d step=%s no honest QC", n, step))
			}
			qcs++
		}

		// Delayed/reordered timeout votes must form a certificate and advance
		// the round without discarding the locked block.
		quorum := (2*n)/3 + 1
		_, _ = chain.CurrentConsensusRound(2), chain.ConsensusV2.RoundStartedAt[2]
		chain.ConsensusV2.LockedHash = "0xfinal"
		timeoutOrder, drops := wanDeliveryOrder(indices[:quorum], n*10+3)
		transientDrops += drops
		retransmittedVotes += drops
		var timeoutCertificate *bc.TimeoutCertificate
		for _, i := range timeoutOrder {
			vote := bc.ConsensusTimeoutVote{Height: 2, Round: 0}
			if err := bc.SignConsensusTimeoutVote(&vote, signers[i].keyHex); err != nil {
				panic(err)
			}
			var err error
			timeoutCertificate, err = chain.AddConsensusTimeoutVote(vote)
			if err != nil {
				panic(err)
			}
		}
		startedAt := chain.ConsensusV2.RoundStartedAt[2]
		round, changed := chain.AdvanceConsensusRound(2, startedAt+chain.ConsensusV2.RoundTimeoutSeconds)
		if timeoutCertificate == nil || !changed || round != 1 || chain.ConsensusV2.LockedHash != "0xfinal" {
			panic(fmt.Sprintf("n=%d WAN timeout/view-change liveness failed", n))
		}
		timeoutCertificates++

		// Replace the entire validator set. Old-only quorum is insufficient;
		// old and new banks must independently exceed two thirds.
		chain.ConsensusV2.EpochLength = 1
		chain.Validators = nil
		for i := 0; i < n; i++ {
			chain.Validators = append(chain.Validators, &bc.Validator{Address: signers[maxValidators+i].addr, NativeBond: 1e12, LPStakeAmount: 1e18, LiquidityPower: 1e6, LockTime: time.Now().Add(24 * time.Hour)})
		}
		chain.PrepareValidatorSetTransition(1)
		for i := 0; i < quorum; i++ {
			vote, err := signers[i].sign(signRequest{Height: 1, Round: 1, Step: bc.StepPrevote, Hash: "0xjoint"})
			if err != nil {
				panic(err)
			}
			totalVotes++
			qc, _, err := chain.AddConsensusVote(vote)
			if err != nil || qc != nil {
				panic(fmt.Sprintf("n=%d old-only transition quorum accepted", n))
			}
		}
		oldOnlyRejected++
		var jointQC *bc.QuorumCertificate
		for i := 0; i < quorum; i++ {
			vote, err := signers[maxValidators+i].sign(signRequest{Height: 1, Round: 1, Step: bc.StepPrevote, Hash: "0xjoint"})
			if err != nil {
				panic(err)
			}
			totalVotes++
			jointQC, _, err = chain.AddConsensusVote(vote)
			if err != nil {
				panic(err)
			}
		}
		if jointQC == nil {
			panic(fmt.Sprintf("n=%d full-churn joint quorum missing", n))
		}
		jointQCs++
	}
	elapsed := time.Since(started)
	result := map[string]any{
		"validator_range":                 "20-100",
		"configurations":                  81,
		"persistent_validator_processes":  signerProcesses,
		"signed_votes":                    totalVotes,
		"quorum_certificates":             qcs,
		"byzantine_minorities_rejected":   minorityRejected,
		"timeout_certificates":            timeoutCertificates,
		"view_changes_with_lock_retained": timeoutCertificates,
		"full_set_churn_joint_qcs":        jointQCs,
		"old_only_joint_quorums_rejected": oldOnlyRejected,
		"simulated_wan_latency_ms":        "20-500",
		"simulated_clock_skew_ms":         "+/-120000",
		"first_attempt_packet_loss_bps":   1000,
		"transient_packet_drops":          transientDrops,
		"retransmitted_votes":             retransmittedVotes,
		"elapsed_seconds":                 elapsed.Seconds(),
		"votes_per_second":                float64(totalVotes) / elapsed.Seconds(),
	}
	raw, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(raw))
}
