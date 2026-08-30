package blockchaincomponent

import "testing"

func TestCommittedParentStateRootUsesFinalizedCommitment(t *testing.T) {
	bc := newTestBlockchain()
	parent := bc.Blocks[len(bc.Blocks)-1]
	parent.StateRoot = " 0xcommitted-parent-root "
	if got := committedParentStateRoot(bc, parent); got != "0xcommitted-parent-root" {
		t.Fatalf("parent root = %q, want finalized commitment", got)
	}
}

func TestCommittedParentStateRootSupportsLegacyFallback(t *testing.T) {
	bc := newTestBlockchain()
	parent := bc.Blocks[len(bc.Blocks)-1]
	parent.StateRoot = ""
	want := bc.ComputeDeterministicStateRootAt(parent.BlockNumber)
	if got := committedParentStateRoot(bc, parent); got != want || got == "" {
		t.Fatalf("legacy parent root = %q, want %q", got, want)
	}
}
