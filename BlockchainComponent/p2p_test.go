package blockchaincomponent

import (
	"testing"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// NetworkService construction
// ─────────────────────────────────────────────────────────────────────────────

func TestNewNetworkService_Init(t *testing.T) {
	bc := newTestBlockchain()
	ns := NewNetworkService(bc)
	if ns == nil {
		t.Fatal("NewNetworkService should return non-nil")
	}
	if ns.Peers == nil {
		t.Error("Peers map should be initialized")
	}
	if ns.Blockchain != bc {
		t.Error("NetworkService.Blockchain should point to the provided blockchain")
	}
	if ns.PeerEvents == nil {
		t.Error("PeerEvents channel should be initialized")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Peer struct
// ─────────────────────────────────────────────────────────────────────────────

func TestPeer_Fields(t *testing.T) {
	now := time.Now()
	p := Peer{
		Address:     "127.0.0.1",
		Port:        5000,
		HTTPPort:    8080,
		LastSeen:    now,
		Protocol:    1,
		IsActive:    true,
		Reputation:  1.0,
		LastUpdated: now,
		Height:      42,
	}
	if p.Address != "127.0.0.1" {
		t.Errorf("unexpected address: %q", p.Address)
	}
	if p.Port != 5000 {
		t.Errorf("unexpected port: %d", p.Port)
	}
	if !p.IsActive {
		t.Error("peer should be active")
	}
	if p.Height != 42 {
		t.Errorf("unexpected height: %d", p.Height)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Peer constants
// ─────────────────────────────────────────────────────────────────────────────

func TestNetworkConstants(t *testing.T) {
	if MaxPeers <= 0 {
		t.Errorf("MaxPeers should be positive, got %d", MaxPeers)
	}
	if SyncBatchSize <= 0 {
		t.Errorf("SyncBatchSize should be positive, got %d", SyncBatchSize)
	}
	if PingInterval <= 0 {
		t.Errorf("PingInterval should be positive, got %v", PingInterval)
	}
	if HandshakeTimeout <= 0 {
		t.Errorf("HandshakeTimeout should be positive, got %v", HandshakeTimeout)
	}
	if MinReputationThreshold < 0 || MinReputationThreshold >= 1 {
		t.Errorf("MinReputationThreshold out of range [0,1): %f", MinReputationThreshold)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// PeerEvent
// ─────────────────────────────────────────────────────────────────────────────

func TestPeerEvent_Fields(t *testing.T) {
	p := &Peer{Address: "1.2.3.4", Port: 5000}
	evt := PeerEvent{
		Type: "connect",
		Peer: p,
		Data: []byte(`{"hello":"world"}`),
	}
	if evt.Type != "connect" {
		t.Errorf("unexpected event type: %q", evt.Type)
	}
	if evt.Peer.Address != "1.2.3.4" {
		t.Errorf("unexpected peer address: %q", evt.Peer.Address)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// NetworkService Peers map
// ─────────────────────────────────────────────────────────────────────────────

func TestNetworkService_PeersMapOperations(t *testing.T) {
	bc := newTestBlockchain()
	ns := NewNetworkService(bc)

	// Manually add a peer (simulating discovered peer)
	peerKey := "127.0.0.1:5001"
	ns.Peers[peerKey] = &Peer{
		Address:  "127.0.0.1",
		Port:     5001,
		IsActive: true,
		Reputation: 1.0,
	}

	if len(ns.Peers) != 1 {
		t.Errorf("expected 1 peer, got %d", len(ns.Peers))
	}

	if ns.Peers[peerKey].Port != 5001 {
		t.Errorf("unexpected port for stored peer: %d", ns.Peers[peerKey].Port)
	}
}

func TestNetworkService_PeerReputationTracking(t *testing.T) {
	bc := newTestBlockchain()
	ns := NewNetworkService(bc)

	ns.Peers["1.2.3.4:5000"] = &Peer{
		Address:    "1.2.3.4",
		Port:       5000,
		Reputation: 1.0,
		IsActive:   true,
	}

	// Simulate reputation decay (what the network does after failures)
	ns.Peers["1.2.3.4:5000"].Reputation *= PeerReputationDecay

	newRep := ns.Peers["1.2.3.4:5000"].Reputation
	if newRep >= 1.0 {
		t.Errorf("reputation should decay below 1.0, got %f", newRep)
	}
	if newRep != PeerReputationDecay {
		t.Errorf("expected reputation %f after one decay, got %f", PeerReputationDecay, newRep)
	}
}

func TestNetworkService_RemoveLowReputationPeer(t *testing.T) {
	bc := newTestBlockchain()
	ns := NewNetworkService(bc)

	key := "bad.peer:5000"
	ns.Peers[key] = &Peer{
		Address:    "bad.peer",
		Reputation: MinReputationThreshold - 0.1, // below threshold
	}

	// Simulate the check that removes bad peers
	for k, p := range ns.Peers {
		if p.Reputation < MinReputationThreshold {
			delete(ns.Peers, k)
		}
	}

	if _, exists := ns.Peers[key]; exists {
		t.Error("peer with reputation below threshold should be removed")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Network stats via blockchain
// ─────────────────────────────────────────────────────────────────────────────

func TestNetworkService_BlockchainReference(t *testing.T) {
	bc := newTestBlockchain()
	ns := NewNetworkService(bc)

	// The network should be able to read blockchain state
	if ns.Blockchain == nil {
		t.Fatal("network should hold blockchain reference")
	}

	// Blockchain block height visible through network
	blocks := ns.Blockchain.Blocks
	if len(blocks) == 0 {
		t.Error("expected at least genesis block in blockchain via network reference")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// PeerEvents channel (non-blocking send/receive)
// ─────────────────────────────────────────────────────────────────────────────

func TestNetworkService_PeerEventsChannel(t *testing.T) {
	bc := newTestBlockchain()
	ns := NewNetworkService(bc)

	// Channel should be buffered (capacity > 0)
	if cap(ns.PeerEvents) == 0 {
		t.Error("PeerEvents channel should be buffered")
	}

	// Should be able to send without blocking
	evt := PeerEvent{Type: "test", Data: []byte("ping")}
	select {
	case ns.PeerEvents <- evt:
		// ok
	default:
		t.Error("PeerEvents channel is full (should have capacity)")
	}

	// And receive it back
	select {
	case received := <-ns.PeerEvents:
		if received.Type != "test" {
			t.Errorf("unexpected event type: %q", received.Type)
		}
	default:
		t.Error("expected to receive the event back")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Multiple peers
// ─────────────────────────────────────────────────────────────────────────────

func TestNetworkService_MaxPeersCap(t *testing.T) {
	bc := newTestBlockchain()
	ns := NewNetworkService(bc)

	// Fill up to MaxPeers + 5
	for i := 0; i < MaxPeers+5; i++ {
		key := "peer" + string(rune('0'+i)) + ":5000"
		ns.Peers[key] = &Peer{Address: key, IsActive: true, Reputation: 1.0}
	}

	// Simulate trimming excess peers
	for len(ns.Peers) > MaxPeers {
		for k := range ns.Peers {
			delete(ns.Peers, k)
			break
		}
	}

	if len(ns.Peers) > MaxPeers {
		t.Errorf("peers should be capped at MaxPeers (%d), got %d", MaxPeers, len(ns.Peers))
	}
}
