package blockchaincomponent

import (
	"errors"
	"testing"
)

func TestIsPrunedHistoryError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "pruned history", err: errors.New("History has been pruned for this block"), want: true},
		{name: "missing trie", err: errors.New("missing trie node"), want: true},
		{name: "other", err: errors.New("temporary rpc timeout"), want: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := isPrunedHistoryError(tc.err); got != tc.want {
				t.Fatalf("isPrunedHistoryError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestFastForwardRelayerCheckpointPreservesOtherCursor(t *testing.T) {
	t.Parallel()

	var saved *BridgeRelayerCheckpoint
	lastBurn := uint64(10)
	ok := fastForwardRelayerCheckpoint("test", 11, 20, &lastBurn, 33, "rpc-a", func(cp *BridgeRelayerCheckpoint) error {
		copy := *cp
		saved = &copy
		return nil
	})
	if !ok {
		t.Fatal("expected fast-forward to succeed")
	}
	if lastBurn != 20 {
		t.Fatalf("expected burn cursor to advance to 20, got %d", lastBurn)
	}
	if saved == nil || saved.LastBurnChecked != 20 || saved.LastLockChecked != 33 {
		t.Fatalf("unexpected saved checkpoint: %+v", saved)
	}
}

func TestFastForwardRelayerLockCheckpointPreservesOtherCursor(t *testing.T) {
	t.Parallel()

	var saved *BridgeRelayerCheckpoint
	lastLock := uint64(15)
	ok := fastForwardRelayerLockCheckpoint("test", 16, 25, 42, &lastLock, "rpc-b", func(cp *BridgeRelayerCheckpoint) error {
		copy := *cp
		saved = &copy
		return nil
	})
	if !ok {
		t.Fatal("expected fast-forward to succeed")
	}
	if lastLock != 25 {
		t.Fatalf("expected lock cursor to advance to 25, got %d", lastLock)
	}
	if saved == nil || saved.LastBurnChecked != 42 || saved.LastLockChecked != 25 {
		t.Fatalf("unexpected saved checkpoint: %+v", saved)
	}
}
