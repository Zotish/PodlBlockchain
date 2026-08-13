package blockchaincomponent

import (
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
)

func consensusFixture(t *testing.T, n int) (*Blockchain_struct, []string) {
	t.Helper()
	bc := newTestBlockchain()
	keys := make([]string, n)
	for i := 0; i < n; i++ {
		key, err := crypto.GenerateKey()
		if err != nil {
			t.Fatal(err)
		}
		keys[i] = hex.EncodeToString(crypto.FromECDSA(key))
		bc.Validators = append(bc.Validators, &Validator{Address: crypto.PubkeyToAddress(key.PublicKey).Hex(), NativeBond: 1e12, LPStakeAmount: 1e12, LockTime: time.Now().Add(time.Hour)})
	}
	bc.EnsureRuntimeState()
	bc.PrepareValidatorSetTransition(1)
	return bc, keys
}

func TestBFTChaosSafetyAndLivenessFourToTwentyNodes(t *testing.T) {
	for n := 4; n <= 20; n++ {
		t.Run(fmt.Sprintf("validators_%d", n), func(t *testing.T) {
			bc, keys := consensusFixture(t, n)
			faults := (n - 1) / 3
			for i := 0; i < faults; i++ {
				if qc, _, err := bc.AddConsensusVote(makeSignedVote(t, keys[i], 1, 0, StepPrecommit, "0xevil")); err != nil || qc != nil {
					t.Fatalf("Byzantine minority formed QC: qc=%v err=%v", qc, err)
				}
			}
			var qc *QuorumCertificate
			for i := faults; i < n; i++ {
				var err error
				qc, _, err = bc.AddConsensusVote(makeSignedVote(t, keys[i], 1, 0, StepPrevote, "0xgood"))
				if err != nil {
					t.Fatal(err)
				}
			}
			if qc == nil {
				t.Fatal("honest >2/3 did not form QC")
			}
			quorum := (2*n)/3 + 1
			for i := 0; i < quorum-1; i++ {
				vote := ConsensusTimeoutVote{Height: 2, Round: 0}
				if err := SignConsensusTimeoutVote(&vote, keys[i]); err != nil {
					t.Fatal(err)
				}
				tc, err := bc.AddConsensusTimeoutVote(vote)
				if err != nil {
					t.Fatal(err)
				}
				if tc != nil {
					t.Fatal("timeout certificate formed without >2/3")
				}
			}
			vote := ConsensusTimeoutVote{Height: 2, Round: 0}
			if err := SignConsensusTimeoutVote(&vote, keys[quorum-1]); err != nil {
				t.Fatal(err)
			}
			tc, err := bc.AddConsensusTimeoutVote(vote)
			if err != nil || tc == nil {
				t.Fatalf("timeout liveness failed: %v", err)
			}
		})
	}
}

func TestJointQuorumRejectsLargeValidatorChurnPartition(t *testing.T) {
	bc, oldKeys := consensusFixture(t, 10)
	bc.ConsensusV2.EpochLength = 10
	newKeys := make([]string, 10)
	bc.Validators = nil
	for i := range newKeys {
		key, err := crypto.GenerateKey()
		if err != nil {
			t.Fatal(err)
		}
		newKeys[i] = hex.EncodeToString(crypto.FromECDSA(key))
		bc.Validators = append(bc.Validators, &Validator{Address: crypto.PubkeyToAddress(key.PublicKey).Hex(), NativeBond: 1e12, LPStakeAmount: 1e12, LockTime: time.Now().Add(time.Hour)})
	}
	bc.PrepareValidatorSetTransition(10)
	for i := 0; i < 7; i++ {
		qc, _, err := bc.AddConsensusVote(makeSignedVote(t, oldKeys[i], 10, 0, StepPrevote, "0xjoint"))
		if err != nil {
			t.Fatal(err)
		}
		if qc != nil {
			t.Fatal("old set alone bypassed joint quorum")
		}
	}
	var qc *QuorumCertificate
	for i := 0; i < 7; i++ {
		var err error
		qc, _, err = bc.AddConsensusVote(makeSignedVote(t, newKeys[i], 10, 0, StepPrevote, "0xjoint"))
		if err != nil {
			t.Fatal(err)
		}
	}
	if qc == nil {
		t.Fatal("joint old/new >2/3 quorum not formed")
	}
}
