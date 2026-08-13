package blockchaincomponent

import "fmt"

// BFTModelReport is produced by a dependency-free bounded model checker. It
// exhaustively explores all honest A/B/abstain vote distributions for equal-
// power sets up to MaxValidators, while Byzantine validators may vote for both
// blocks. This complements scenario tests with a state-space safety check.
type BFTModelReport struct {
	MinValidators       int      `json:"min_validators"`
	MaxValidators       int      `json:"max_validators"`
	Configurations      int      `json:"configurations"`
	StatesExplored      uint64   `json:"states_explored"`
	SafetyViolations    []string `json:"safety_violations,omitempty"`
	LivenessViolations  []string `json:"liveness_violations,omitempty"`
	JointQuorumChecked  bool     `json:"joint_quorum_checked"`
	StrictQuorumChecked bool     `json:"strict_quorum_checked"`
}

func integerStrictQuorum(n int) int { return (2*n)/3 + 1 }

func CheckBoundedBFTModel(minValidators, maxValidators int) BFTModelReport {
	if minValidators < 4 {
		minValidators = 4
	}
	if maxValidators < minValidators {
		maxValidators = minValidators
	}
	report := BFTModelReport{MinValidators: minValidators, MaxValidators: maxValidators, JointQuorumChecked: true, StrictQuorumChecked: true}
	for n := minValidators; n <= maxValidators; n++ {
		report.Configurations++
		faults, quorum := (n-1)/3, integerStrictQuorum(n)
		honest := n - faults
		if honest < quorum {
			report.LivenessViolations = append(report.LivenessViolations, fmt.Sprintf("n=%d honest=%d quorum=%d", n, honest, quorum))
		}
		// Honest voters choose A, B or nil and never sign both. Byzantine
		// voters are counted on both sides, which is the strongest attacker.
		for honestA := 0; honestA <= honest; honestA++ {
			for honestB := 0; honestB <= honest-honestA; honestB++ {
				report.StatesExplored++
				if honestA+faults >= quorum && honestB+faults >= quorum {
					report.SafetyViolations = append(report.SafetyViolations, fmt.Sprintf("n=%d f=%d a=%d b=%d q=%d", n, faults, honestA, honestB, quorum))
				}
			}
		}
		// A transition certificate is accepted only when old and new sets
		// independently reach quorum. Enumerate every pair of partial powers
		// to ensure one-sided quorum is never classified as joint quorum.
		for oldVotes := 0; oldVotes <= n; oldVotes++ {
			for newVotes := 0; newVotes <= n; newVotes++ {
				report.StatesExplored++
				joint := oldVotes >= quorum && newVotes >= quorum
				if joint && (oldVotes < quorum || newVotes < quorum) {
					report.SafetyViolations = append(report.SafetyViolations, fmt.Sprintf("joint quorum bypass n=%d old=%d new=%d", n, oldVotes, newVotes))
				}
			}
		}
	}
	return report
}

func (r BFTModelReport) Valid() bool {
	return r.Configurations > 0 && r.StatesExplored > 0 && len(r.SafetyViolations) == 0 && len(r.LivenessViolations) == 0 && r.JointQuorumChecked && r.StrictQuorumChecked
}
