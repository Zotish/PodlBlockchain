package blockchaincomponent

import "testing"

func TestBoundedBFTModelFourToOneHundredValidators(t *testing.T) {
	report := CheckBoundedBFTModel(4, 100)
	if !report.Valid() {
		t.Fatalf("bounded BFT model failed: %#v", report)
	}
	if report.Configurations != 97 || report.StatesExplored < 100_000 {
		t.Fatalf("unexpected model coverage: %#v", report)
	}
}

func TestStrictQuorumDoesNotAcceptExactlyTwoThirds(t *testing.T) {
	for n := 3; n <= 100; n += 3 {
		if integerStrictQuorum(n) != 2*n/3+1 {
			t.Fatalf("n=%d did not require strictly greater than two thirds", n)
		}
	}
}
