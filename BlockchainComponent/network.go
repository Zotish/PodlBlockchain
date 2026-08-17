package blockchaincomponent

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	constantset "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/ConstantSet"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	protocolVersion       = int(CurrentProtocolVersion)
	defaultPort           = "5000"
	PingInterval          = 30 * time.Second
	defaultNetworkID      = "mainnet"
	PeerDiscoveryInterval = 5 * time.Minute
	MaxPeers              = 50
	HandshakeTimeout      = 10 * time.Second

	SyncBatchSize          = 100
	MaxSyncAttempts        = 3
	PeerResponseThreshold  = 5 * time.Second
	PeerReputationDecay    = 0.9
	MinReputationThreshold = 0.3
	MaxVotingPeerHeightLag = 2
)

type Peer struct {
	Address           string    `json:"address"`
	Port              int       `json:"port"`
	HTTPPort          int       `json:"http_port"`
	LastSeen          time.Time `json:"last_seen"`
	Protocol          int       `json:"protocol"`
	IsActive          bool      `json:"is_active"`
	Reputation        float64   `json:"reputation"`
	LastUpdated       time.Time `json:"last_updated"`
	Height            int       `json:"height"`
	ValidatorAddress  string    `json:"validator_address,omitempty"`
	ValidatorVerified bool      `json:"validator_verified"`
	SyncStatus        string    `json:"sync_status,omitempty"`
	HeightLag         int       `json:"height_lag"`
	SuccessCount      uint64    `json:"success_count,omitempty"`
	FailureCount      uint64    `json:"failure_count,omitempty"`
	LastFailure       time.Time `json:"last_failure,omitempty"`
}

type NetworkService struct {
	Peers               map[string]*Peer   `json:"peers"`
	Blockchain          *Blockchain_struct `json:"blockchain"`
	Listener            net.Listener       `json:"-"`
	ListenPort          string             `json:"-"`
	HTTPPort            int                `json:"-"`
	ValidatorAddress    string             `json:"-"`
	ValidatorPrivateKey string             `json:"-"`
	ValidatorSigner     ValidatorSigner    `json:"-"`
	Mutex               sync.Mutex         `json:"-"`
	PeerEvents          chan PeerEvent     `json:"-"`
	Wg                  sync.WaitGroup     `json:"-"`
}
type PeerEvent struct {
	Type string `json:"type"`
	Peer *Peer  `json:"peer"`
	Data []byte `json:"data"`
}

func NewNetworkService(bc *Blockchain_struct) *NetworkService {
	newService := new(NetworkService)
	newService.Peers = make(map[string]*Peer)
	newService.Blockchain = bc
	newService.PeerEvents = make(chan PeerEvent, 100)
	return newService
}

func defaultPeerReputation() float64 {
	return 0.6
}

func promotePeerReputation(current, delta float64) float64 {
	if current <= 0 {
		current = defaultPeerReputation()
	}
	current += delta
	if current > 1 {
		return 1
	}
	return current
}

func decayPeerReputation(current float64) float64 {
	if current <= 0 {
		current = defaultPeerReputation()
	}
	current *= PeerReputationDecay
	if current < 0 {
		return 0
	}
	return current
}

func peerMinReputationThreshold() float64 {
	if v := parseEnvFloat("LQD_PEER_MIN_REPUTATION"); v > 0 && v < 1 {
		return v
	}
	return MinReputationThreshold
}

func peerMaxVotingHeightLag() int {
	return parseEnvInt("LQD_MAX_VOTING_PEER_HEIGHT_LAG", MaxVotingPeerHeightLag)
}

func peerFailurePenalty() float64 {
	if v := parseEnvFloat("LQD_PEER_FAILURE_PENALTY"); v > 0 && v < 1 {
		return v
	}
	return 0.15
}

func recordPeerSuccess(peer *Peer, delta float64) {
	if peer == nil {
		return
	}
	peer.SuccessCount++
	peer.Reputation = promotePeerReputation(peer.Reputation, delta)
	peer.IsActive = true
	peer.LastSeen = time.Now()
	peer.LastUpdated = time.Now()
}

func recordPeerFailure(peer *Peer, reason string) {
	if peer == nil {
		return
	}
	peer.FailureCount++
	peer.LastFailure = time.Now()
	peer.LastUpdated = time.Now()
	penalty := peerFailurePenalty()
	if peer.Reputation <= 0 {
		peer.Reputation = defaultPeerReputation()
	}
	peer.Reputation -= penalty
	if peer.Reputation < 0 {
		peer.Reputation = 0
	}
	if peer.Reputation < peerMinReputationThreshold()/2 {
		peer.IsActive = false
	}
	if reason != "" {
		log.Printf("Peer %s:%d reputation penalty (%s): %.2f", peer.Address, peer.Port, reason, peer.Reputation)
	}
}

func (ns *NetworkService) SetValidatorIdentity(address, privateKey string) {
	address = strings.TrimSpace(address)
	privateKey = strings.TrimSpace(privateKey)
	var signer ValidatorSigner
	if privateKey != "" {
		local, err := NewLocalValidatorSigner(privateKey, "", "")
		if err != nil {
			log.Printf("validator signer configuration rejected: %v", err)
		} else if address != "" && !strings.EqualFold(address, local.Address()) {
			log.Printf("validator signer configuration rejected: key does not match %s", address)
			_ = local.Close()
		} else {
			signer = local
			address = local.Address()
		}
	}
	ns.Mutex.Lock()
	defer ns.Mutex.Unlock()
	ns.ValidatorAddress = address
	// Private keys are no longer retained by NetworkService. This field remains
	// for state/ABI compatibility and is deliberately cleared.
	ns.ValidatorPrivateKey = ""
	ns.ValidatorSigner = signer
}

func (ns *NetworkService) SetValidatorSigner(address string, signer ValidatorSigner) error {
	if ns == nil || signer == nil || !ValidateAddress(signer.Address()) {
		return fmt.Errorf("healthy validator signer required")
	}
	if strings.TrimSpace(address) != "" && !strings.EqualFold(address, signer.Address()) {
		return fmt.Errorf("validator signer address %s does not match configured address %s", signer.Address(), address)
	}
	status := signer.Status(context.Background())
	if !status.Healthy {
		return fmt.Errorf("validator signer is unhealthy: %s", status.Detail)
	}
	ns.Mutex.Lock()
	defer ns.Mutex.Unlock()
	if ns.ValidatorSigner != nil && ns.ValidatorSigner != signer {
		_ = ns.ValidatorSigner.Close()
	}
	ns.ValidatorAddress = strings.ToLower(signer.Address())
	ns.ValidatorPrivateKey = ""
	ns.ValidatorSigner = signer
	return nil
}

func (ns *NetworkService) ValidatorIdentitySnapshot() (string, string) {
	if ns == nil {
		return "", ""
	}
	ns.Mutex.Lock()
	defer ns.Mutex.Unlock()
	return strings.TrimSpace(ns.ValidatorAddress), strings.TrimSpace(ns.ValidatorPrivateKey)
}

func (ns *NetworkService) ValidatorSignerSnapshot() (string, ValidatorSigner) {
	if ns == nil {
		return "", nil
	}
	ns.Mutex.Lock()
	defer ns.Mutex.Unlock()
	return strings.TrimSpace(ns.ValidatorAddress), ns.ValidatorSigner
}

func (ns *NetworkService) syncWithPeer(peer *Peer, ourHeight int) error {
	conn, err := net.DialTimeout("tcp",
		fmt.Sprintf("%s:%d", peer.Address, peer.Port),
		10*time.Second)
	if err != nil {
		return fmt.Errorf("dial failed: %v", err)
	}
	defer conn.Close()

	decoder := json.NewDecoder(conn)
	if peerVersion, err := ns.sendVersionHandshake(conn, decoder); err != nil {
		return err
	} else {
		ns.applyPeerVersion(peer, peerVersion)
	}

	for start := ourHeight + 1; start <= peer.Height; start += SyncBatchSize {
		end := start + SyncBatchSize - 1
		if end > peer.Height {
			end = peer.Height
		}

		request := map[string]interface{}{
			"type":        "sync",
			"start_block": start,
			"end_block":   end,
		}

		if err := json.NewEncoder(conn).Encode(request); err != nil {
			return fmt.Errorf("encode failed: %v", err)
		}

		blocks := make([]*Block, 0, end-start+1)
		for height := start; height <= end; height++ {
			_ = conn.SetReadDeadline(time.Now().Add(PeerResponseThreshold))
			var block Block
			if err := decoder.Decode(&block); err != nil {
				return fmt.Errorf("decode failed at height %d: %v", height, err)
			}
			_ = conn.SetReadDeadline(time.Time{})
			blocks = append(blocks, &block)
		}

		if err := ns.applySyncedBlocks(blocks); err != nil {
			return err
		}
	}

	return nil
}

func (ns *NetworkService) applySyncedBlocks(blocks []*Block) error {
	if len(blocks) == 0 {
		return nil
	}

	ns.Blockchain.Mutex.Lock()
	defer ns.Blockchain.Mutex.Unlock()

	for _, block := range blocks {
		if len(ns.Blockchain.Blocks) > 0 && block.BlockNumber <= ns.Blockchain.Blocks[len(ns.Blockchain.Blocks)-1].BlockNumber {
			continue
		}
		if err := ns.verifySyncedBlock(ns.Blockchain.Blocks, block); err != nil {
			return err
		}
		ns.Blockchain.Blocks = append(ns.Blockchain.Blocks, block)
		if err := SaveBlockToDB(block); err != nil {
			log.Printf("sync: SaveBlockToDB error at height %d: %v", block.BlockNumber, err)
		}
	}

	ns.Blockchain.Transaction_pool = []*Transaction{}
	return nil
}

func (ns *NetworkService) verifySyncedBlock(chain []*Block, block *Block) error {
	if block == nil {
		return fmt.Errorf("nil block in sync stream")
	}
	if len(chain) == 0 {
		return fmt.Errorf("cannot sync onto empty chain")
	}

	prev := chain[len(chain)-1]
	if block.BlockNumber != prev.BlockNumber+1 {
		return fmt.Errorf("unexpected block number %d after %d", block.BlockNumber, prev.BlockNumber)
	}
	if block.PreviousHash != prev.CurrentHash {
		return fmt.Errorf("previous hash mismatch at height %d", block.BlockNumber)
	}

	tempHash := block.CurrentHash
	block.CurrentHash = ""
	calculatedHash := CalculateHash(block)
	block.CurrentHash = tempHash
	if calculatedHash != tempHash {
		return fmt.Errorf("invalid hash at height %d", block.BlockNumber)
	}

	if block.TimeStamp < prev.TimeStamp {
		return fmt.Errorf("non-monotonic timestamp at height %d", block.BlockNumber)
	}
	now := uint64(time.Now().Unix())
	if block.TimeStamp > now+30 {
		return fmt.Errorf("future timestamp at height %d", block.BlockNumber)
	}

	totalGas := uint64(0)
	for _, tx := range block.Transactions {
		totalGas += tx.Gas * tx.GasPrice
		if totalGas > uint64(constantset.MaxBlockGas) {
			return fmt.Errorf("gas overflow at height %d", block.BlockNumber)
		}
	}

	return nil
}

func (ns *NetworkService) Start(listenPort string) error {
	if listenPort == "" {
		listenPort = defaultPort
	}
	ns.ListenPort = listenPort
	listener, err := net.Listen("tcp", ":"+listenPort)
	if err != nil {
		return err
	}
	ns.Listener = listener
	ns.Wg.Add(3)
	go ns.acceptConnections()
	go ns.maintainPeerConnections()
	go ns.processPeerEvents()

	defaultp, err := strconv.Atoi(listenPort)
	if err != nil {
		log.Printf("Error converting default port: %v", err)
	}
	// Add a local bootstrap node (configure more peers via -remote_node).
	ns.AddPeer("localhost", defaultp, true)
	log.Printf("Network service started on port %s", listenPort)
	return nil

}

func (ns *NetworkService) sendVersionHandshake(conn net.Conn, decoder *json.Decoder) (map[string]interface{}, error) {
	if err := json.NewEncoder(conn).Encode(ns.buildVersionPayload()); err != nil {
		return nil, fmt.Errorf("handshake send failed: %v", err)
	}

	var peerVersion map[string]interface{}
	if err := decoder.Decode(&peerVersion); err != nil {
		return nil, fmt.Errorf("handshake read failed: %v", err)
	}

	peerProtocol, ok := peerVersion["protocol"].(float64)
	if !ok || int(peerProtocol) != protocolVersion {
		return nil, fmt.Errorf("handshake protocol mismatch")
	}
	if err := ns.validatePeerChainSpec(peerVersion); err != nil {
		return nil, err
	}

	return peerVersion, nil
}

func (ns *NetworkService) buildVersionPayload() map[string]interface{} {
	now := time.Now().Unix()
	payload := map[string]interface{}{
		"protocol":        protocolVersion,
		"best_height":     ns.bestHeight(),
		"timestamp":       now,
		"listen_port":     toIntPort(ns.ListenPort),
		"http_port":       ns.HTTPPort,
		"network_id":      ns.Blockchain.ChainSpec.NetworkID,
		"chain_id":        ns.Blockchain.ChainSpec.ChainID,
		"genesis_hash":    ns.Blockchain.ChainSpec.GenesisHash,
		"chain_spec_hash": ns.Blockchain.ChainSpec.Hash(),
	}

	validatorAddress, signer := ns.ValidatorSignerSnapshot()

	if validatorAddress != "" {
		message := validatorHandshakeMessage(validatorAddress, toIntPort(ns.ListenPort), ns.HTTPPort, now)
		payload["validator_address"] = validatorAddress
		payload["validator_message"] = message
		if signer != nil {
			signature, err := signer.SignMessage(context.Background(), SignerDomainP2PHandshake, []byte(message), "")
			if err != nil {
				log.Printf("validator handshake signing failed: %v", err)
			} else {
				payload["validator_signature"] = signature
			}
		}
	}
	return payload
}

func (ns *NetworkService) validatePeerChainSpec(payload map[string]interface{}) error {
	if ns == nil || ns.Blockchain == nil {
		return fmt.Errorf("local chain spec unavailable")
	}
	spec := ns.Blockchain.ChainSpec
	networkID, _ := payload["network_id"].(string)
	genesisHash, _ := payload["genesis_hash"].(string)
	specHash, _ := payload["chain_spec_hash"].(string)
	chainID, chainIDOK := payload["chain_id"].(float64)
	if !chainIDOK || uint(chainID) != spec.ChainID || networkID != spec.NetworkID || !strings.EqualFold(genesisHash, spec.GenesisHash) || !strings.EqualFold(specHash, spec.Hash()) {
		return fmt.Errorf("handshake chain specification mismatch")
	}
	return nil
}

func validatorHandshakeMessage(address string, listenPort, httpPort int, timestamp int64) string {
	return fmt.Sprintf("PODL-P2P:%s:%d:%d:%d", strings.ToLower(strings.TrimSpace(address)), listenPort, httpPort, timestamp)
}

func signValidatorHandshake(privateKeyHex, message string) (string, error) {
	keyHex := strings.TrimPrefix(strings.TrimSpace(privateKeyHex), "0x")
	privateKey, err := crypto.HexToECDSA(keyHex)
	if err != nil {
		return "", err
	}
	signature, err := crypto.Sign(accounts.TextHash([]byte(message)), privateKey)
	if err != nil {
		return "", err
	}
	return "0x" + hex.EncodeToString(signature), nil
}

func verifyValidatorHandshake(address, message, signature string) bool {
	address = strings.TrimSpace(address)
	signature = strings.TrimPrefix(strings.TrimSpace(signature), "0x")
	if address == "" || message == "" || signature == "" {
		return false
	}
	parts := strings.Split(message, ":")
	if len(parts) != 5 || parts[0] != "PODL-P2P" || !strings.EqualFold(parts[1], address) {
		return false
	}
	timestamp, err := strconv.ParseInt(parts[4], 10, 64)
	if err != nil {
		return false
	}
	// Keep validator identity proofs fresh enough to avoid simple replay.
	if math.Abs(float64(time.Now().Unix()-timestamp)) > float64(10*time.Minute/time.Second) {
		return false
	}
	raw, err := hex.DecodeString(signature)
	if err != nil {
		return false
	}
	pub, err := crypto.SigToPub(accounts.TextHash([]byte(message)), raw)
	if err != nil {
		return false
	}
	recovered := crypto.PubkeyToAddress(*pub).Hex()
	return strings.EqualFold(recovered, address)
}

func (ns *NetworkService) bestHeight() int {
	if ns == nil || ns.Blockchain == nil {
		return 0
	}
	height := 0
	ns.Blockchain.Mutex.Lock()
	if len(ns.Blockchain.Blocks) > 0 {
		height = int(ns.Blockchain.Blocks[len(ns.Blockchain.Blocks)-1].BlockNumber)
	}
	ns.Blockchain.Mutex.Unlock()
	latest, err := GetLatestBlockNumberFromDB()
	if err != nil {
		return height
	}
	if int(latest) > height {
		height = int(latest)
	}
	return height
}

func (ns *NetworkService) applyPeerVersion(peer *Peer, peerVersion map[string]interface{}) {
	if peer == nil || peerVersion == nil {
		return
	}
	if height, ok := peerVersion["best_height"].(float64); ok {
		peer.Height = int(height)
	}
	if httpPort, ok := peerVersion["http_port"].(float64); ok {
		peer.HTTPPort = int(httpPort)
	}
	if listenPort, ok := peerVersion["listen_port"].(float64); ok {
		peer.Port = int(listenPort)
	}
	if validatorAddress, ok := peerVersion["validator_address"].(string); ok {
		peer.ValidatorAddress = strings.TrimSpace(validatorAddress)
		message, _ := peerVersion["validator_message"].(string)
		signature, _ := peerVersion["validator_signature"].(string)
		peer.ValidatorVerified = verifyValidatorHandshake(peer.ValidatorAddress, message, signature)
	}
	if peer.Reputation <= 0 {
		peer.Reputation = defaultPeerReputation()
	}
	if peer.ValidatorVerified {
		peer.Reputation = promotePeerReputation(peer.Reputation, 0.10)
	} else {
		peer.Reputation = promotePeerReputation(peer.Reputation, 0.02)
	}
	peer.IsActive = true
	peer.LastSeen = time.Now()
	peer.LastUpdated = time.Now()
	ns.updatePeerSyncStatus(peer, ns.bestHeight())
}

func (ns *NetworkService) blockByNumber(number uint64) (*Block, error) {
	if ns == nil || ns.Blockchain == nil {
		return nil, fmt.Errorf("network/blockchain not initialized")
	}
	ns.Blockchain.Mutex.Lock()
	for _, block := range ns.Blockchain.Blocks {
		if block != nil && block.BlockNumber == number {
			ns.Blockchain.Mutex.Unlock()
			return block, nil
		}
	}
	ns.Blockchain.Mutex.Unlock()
	return GetBlockFromDB(number)
}

func (ns *NetworkService) updatePeerSyncStatus(peer *Peer, localHeight int) {
	if peer == nil {
		return
	}
	lag := localHeight - peer.Height
	if lag < 0 {
		lag = 0
	}
	peer.HeightLag = lag
	switch {
	case !peer.IsActive:
		peer.SyncStatus = "offline"
	case localHeight > 0 && peer.Height+peerMaxVotingHeightLag() < localHeight:
		peer.SyncStatus = "syncing"
	case peer.ValidatorVerified:
		peer.SyncStatus = "active"
	default:
		peer.SyncStatus = "connected_unverified"
	}
}

func (ns *NetworkService) peerVotingEligibleLocked(peer *Peer, localHeight int) bool {
	if peer == nil || ns.isSelfPeer(peer) {
		return false
	}
	if !peer.IsActive || peer.HTTPPort == 0 {
		return false
	}
	if !peer.ValidatorVerified || strings.TrimSpace(peer.ValidatorAddress) == "" {
		return false
	}
	if peer.Reputation < peerMinReputationThreshold() {
		return false
	}
	if localHeight > 0 && peer.Height+peerMaxVotingHeightLag() < localHeight {
		return false
	}
	return time.Since(peer.LastSeen) < 2*PingInterval
}

func (ns *NetworkService) HasVotingPeerForValidator(address string, localHeight int) bool {
	if ns == nil {
		return false
	}
	address = strings.TrimSpace(address)
	if address == "" {
		return false
	}
	ns.Mutex.Lock()
	defer ns.Mutex.Unlock()
	for _, peer := range ns.Peers {
		if peer == nil || !strings.EqualFold(peer.ValidatorAddress, address) {
			continue
		}
		ns.updatePeerSyncStatus(peer, localHeight)
		if ns.peerVotingEligibleLocked(peer, localHeight) {
			return true
		}
	}
	return false
}

func (ns *NetworkService) PeerStatusSnapshot() []map[string]interface{} {
	if ns == nil {
		return []map[string]interface{}{}
	}
	localHeight := ns.bestHeight()
	ns.Mutex.Lock()
	defer ns.Mutex.Unlock()

	out := make([]map[string]interface{}, 0, len(ns.Peers))
	for _, p := range ns.Peers {
		if p == nil {
			continue
		}
		ns.updatePeerSyncStatus(p, localHeight)
		out = append(out, map[string]interface{}{
			"address":            p.Address,
			"port":               p.Port,
			"http_port":          p.HTTPPort,
			"last_seen":          p.LastSeen,
			"reputation":         p.Reputation,
			"height":             p.Height,
			"height_lag":         p.HeightLag,
			"is_active":          p.IsActive,
			"sync_status":        p.SyncStatus,
			"validator_address":  p.ValidatorAddress,
			"validator_verified": p.ValidatorVerified,
			"voting_eligible":    ns.peerVotingEligibleLocked(p, localHeight),
			"success_count":      p.SuccessCount,
			"failure_count":      p.FailureCount,
			"last_failure":       p.LastFailure,
		})
	}
	return out
}

func (ns *NetworkService) fetchPeerHeight(peer *Peer) error {
	conn, err := net.DialTimeout("tcp",
		fmt.Sprintf("%s:%d", peer.Address, peer.Port),
		5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()

	decoder := json.NewDecoder(conn)
	peerVersion, err := ns.sendVersionHandshake(conn, decoder)
	if err != nil {
		return err
	}

	ns.applyPeerVersion(peer, peerVersion)
	return nil
}
func (ns *NetworkService) processPeerEvents() {
	for event := range ns.PeerEvents {
		localHeight := ns.bestHeight()
		ns.Mutex.Lock()
		peer, exists := ns.Peers[peerKey(event.Peer)]
		if exists && peer != nil {
			switch event.Type {
			case "block":
				peer.LastSeen = time.Now()
				peer.IsActive = true
				peer.Reputation = promotePeerReputation(peer.Reputation, 0.05)

			case "transaction":
				peer.LastSeen = time.Now()
				peer.IsActive = true
				peer.Reputation = promotePeerReputation(peer.Reputation, 0.01)

			case "invalid_block":
				recordPeerFailure(peer, "invalid block")
			}
			ns.updatePeerSyncStatus(peer, localHeight)
		}
		ns.Mutex.Unlock()
	}
}
func (ns *NetworkService) acceptConnections() {
	for {
		conn, err := ns.Listener.Accept()
		if err != nil {
			log.Printf("Error accepting connection: %v", err)
			continue
		}

		go ns.handleConnection(conn)
	}
}
func (ns *NetworkService) maintainPeerConnections() {
	ticker := time.NewTicker(PingInterval)
	defer ticker.Stop()

	for range ticker.C {
		localHeight := ns.bestHeight()
		ns.Mutex.Lock()
		peers := make(map[string]*Peer, len(ns.Peers))
		for key, peer := range ns.Peers {
			if peer == nil {
				delete(ns.Peers, key)
				continue
			}
			if ns.isSelfPeer(peer) {
				delete(ns.Peers, key)
				continue
			}
			if time.Since(peer.LastSeen) > 4*PingInterval {
				delete(ns.Peers, key)
				log.Printf("Removed inactive peer: %s:%d", peer.Address, peer.Port)
				continue
			}
			peers[key] = peer
		}
		ns.Mutex.Unlock()

		for key, peer := range peers {
			go func(key string, p *Peer) {
				success := ns.sendData(p, []byte(`{"type":"ping"}`))
				ns.Mutex.Lock()
				defer ns.Mutex.Unlock()

				current := ns.Peers[key]
				if current == nil {
					return
				}
				if success {
					recordPeerSuccess(current, 0.05)
				} else {
					recordPeerFailure(current, "ping failed")
				}
				ns.updatePeerSyncStatus(current, localHeight)
			}(key, peer)
		}
	}
}

func (ns *NetworkService) sendData(peer *Peer, data []byte) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf(

		"%s:%d",

		peer.Address, peer.Port), 5*time.Second)
	if err != nil {
		log.Printf("Error connecting to peer %s:%d: %v", peer.Address, peer.Port, err)
		return false
	}
	defer conn.Close()

	decoder := json.NewDecoder(conn)
	if peerVersion, err := ns.sendVersionHandshake(conn, decoder); err != nil {
		log.Printf("Handshake with %s:%d failed: %v", peer.Address, peer.Port, err)
		return false
	} else {
		ns.applyPeerVersion(peer, peerVersion)
	}

	// Set write deadline
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))

	_, err = conn.Write(data)
	if err != nil {
		log.Printf("Error sending data to peer %s:%d: %v", peer.Address, peer.Port, err)
		return false
	}

	// For ping messages, wait for pong response
	if string(data) == `{"type":"ping"}` {
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))

		var response map[string]interface{}
		decoder := json.NewDecoder(conn)
		if err := decoder.Decode(&response); err != nil {
			log.Printf("Error reading pong from %s:%d: %v", peer.Address, peer.Port, err)
			return false
		}

		if response["type"] != "pong" {
			log.Printf("Invalid response from %s:%d: expected pong", peer.Address, peer.Port)
			return false
		}
	}

	return true
}
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func hostFromAddr(addr net.Addr) string {
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		return addr.String()
	}
	return host
}

func portFromAddr(addr net.Addr) int {
	_, portStr, err := net.SplitHostPort(addr.String())
	if err != nil {
		return 0
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0
	}
	return port
}

func toIntPort(port string) int {
	p, err := strconv.Atoi(port)
	if err != nil {
		return 0
	}
	return p
}

func peerKey(peer *Peer) string {
	if peer == nil {
		return ""
	}
	return fmt.Sprintf("%s:%d", peer.Address, peer.Port)
}

func isLocalAddress(address string) bool {
	return address == "localhost" || address == "127.0.0.1" || address == "::1"
}
func (ns *NetworkService) handleConnection(conn net.Conn) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(HandshakeTimeout))

	// Create the decoder once at the start of the connection
	decoder := json.NewDecoder(conn)

	// 1. Read first message (version or direct message)
	var firstMsg map[string]interface{}
	if err := decoder.Decode(&firstMsg); err != nil {
		log.Printf("Handshake or message read failed: %v", err)
		return
	}

	peer := &Peer{
		Address:  hostFromAddr(conn.RemoteAddr()),
		Port:     portFromAddr(conn.RemoteAddr()),
		LastSeen: time.Now(),
		Protocol: protocolVersion,
		IsActive: false,
	}

	handledHandshake := false
	if proto, ok := firstMsg["protocol"].(float64); ok {
		if int(proto) != protocolVersion {
			log.Printf("Incompatible protocol: %v (we use %v)", proto, protocolVersion)
			return
		}
		if err := ns.validatePeerChainSpec(firstMsg); err != nil {
			log.Printf("Rejected incompatible peer chain: %v", err)
			return
		}

		ns.applyPeerVersion(peer, firstMsg)

		// Send our version information
		if err := json.NewEncoder(conn).Encode(ns.buildVersionPayload()); err != nil {
			log.Printf("Error sending our version: %v", err)
			return
		}
		handledHandshake = true
	} else {
		// No handshake provided; treat first message as regular message.
	}

	// Handshake complete, reset deadline
	conn.SetDeadline(time.Time{})
	peer.LastUpdated = time.Now()

	// Add peer to our list
	ns.Mutex.Lock()
	ns.Peers[peerKey(peer)] = peer
	ns.Mutex.Unlock()

	// Handle incoming messages
	handleMsg := func(msg map[string]interface{}) bool {
		if msgType, ok := msg["type"].(string); ok {
			switch msgType {
			case "ping":
				if err := json.NewEncoder(conn).Encode(map[string]string{"type": "pong"}); err != nil {
					log.Printf("Error sending pong: %v", err)
					return false
				}
				return true
			case "getpeers":
				ns.Mutex.Lock()
				peersToSend := make([]map[string]interface{}, 0, len(ns.Peers))
				for _, p := range ns.Peers {
					peersToSend = append(peersToSend, map[string]interface{}{
						"address": p.Address,
						"port":    p.Port,
					})
				}
				ns.Mutex.Unlock()

				if err := json.NewEncoder(conn).Encode(map[string]interface{}{
					"type":  "peers",
					"peers": peersToSend,
				}); err != nil {
					log.Printf("Error sending peer list: %v", err)
					return false
				}
				return true
			case "get_validators":
				ns.Blockchain.Mutex.Lock()
				validators := ns.Blockchain.Validators
				ns.Blockchain.Mutex.Unlock()
				if err := json.NewEncoder(conn).Encode(map[string]interface{}{
					"validators": validators,
				}); err != nil {
					log.Printf("Error sending validators: %v", err)
					return false
				}
				return true
			case "sync":
				start, ok1 := msg["start_block"].(float64)
				end, ok2 := msg["end_block"].(float64)
				if !ok1 || !ok2 {
					log.Printf("Invalid sync request from %s", peer.Address)
					return false
				}
				s := int(start)
				e := int(end)
				if s < 0 {
					s = 0
				}
				bestHeight := ns.bestHeight()
				if e > bestHeight {
					e = bestHeight
				}
				if e < s {
					return true
				}
				for height := s; height <= e; height++ {
					block, err := ns.blockByNumber(uint64(height))
					if err != nil {
						log.Printf("Error loading block %d for sync: %v", height, err)
						return false
					}
					if err := json.NewEncoder(conn).Encode(block); err != nil {
						log.Printf("Error sending block %d: %v", height, err)
						return false
					}
				}
				return true
			}
		}

		ns.handleMessage(peer, msg)
		return true
	}

	if !handledHandshake {
		if !handleMsg(firstMsg) {
			return
		}
	}

	for {
		var msg map[string]interface{}
		if err := decoder.Decode(&msg); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			log.Printf("Error decoding message from %s: %v", peer.Address, err)
			break
		}
		if !handleMsg(msg) {
			break
		}
	}

	// Keep the peer record for future reconnect / gossip. Health checks and
	// reputation decay will prune dead peers later; dropping here breaks
	// proposer->follower broadcasts because most exchanges are short-lived.
	ns.Mutex.Lock()
	if existing, ok := ns.Peers[peerKey(peer)]; ok && existing != nil {
		existing.IsActive = true
		existing.LastUpdated = time.Now()
	}
	ns.Mutex.Unlock()
}
func (ns *NetworkService) handleMessage(peer *Peer, msg map[string]interface{}) {
	if peer != nil {
		peer.LastSeen = time.Now()
		peer.LastUpdated = time.Now()
	}

	msgType, ok := msg["type"].(string)
	if !ok {
		log.Printf("Message from %s missing type field", peer.Address)
		return
	}

	switch msgType {
	case "block":
		blockData, ok := msg["data"].(map[string]interface{})
		if !ok {
			log.Printf("Invalid block data from %s", peer.Address)
			return
		}

		jsonData, err := json.Marshal(blockData)
		if err != nil {
			log.Printf("Error marshaling block data: %v", err)
			return
		}

		var block Block
		if err := json.Unmarshal(jsonData, &block); err != nil {
			log.Printf("Error decoding block: %v", err)
			return
		}

		ns.Blockchain.Mutex.Lock()
		localHeight := uint64(0)
		if len(ns.Blockchain.Blocks) > 0 {
			localHeight = ns.Blockchain.Blocks[len(ns.Blockchain.Blocks)-1].BlockNumber
		}
		ns.Blockchain.Mutex.Unlock()
		if block.BlockNumber <= localHeight {
			log.Printf("Stale block received from %s (height %d <= %d)", peer.Address, block.BlockNumber, localHeight)
			return
		}
		if block.BlockNumber > localHeight+1 {
			if peer != nil && int(block.BlockNumber)+1 > peer.Height {
				peer.Height = int(block.BlockNumber) + 1
			}
			log.Printf("Future block received from %s (height %d > %d+1), triggering sync", peer.Address, block.BlockNumber, localHeight)
			go func() {
				if err := ns.SyncChain(); err != nil {
					log.Printf("SyncChain after future block failed: %v", err)
				}
			}()
			return
		}

		if block.RewardBreakdown.Validator != "" {
			ns.Blockchain.Mutex.Lock()
			known := false
			for _, v := range ns.Blockchain.Validators {
				if strings.EqualFold(v.Address, block.RewardBreakdown.Validator) {
					known = true
					break
				}
			}
			ns.Blockchain.Mutex.Unlock()
			if !known {
				if err := ns.SyncValidators(peer); err != nil {
					log.Printf("SyncValidators failed from %s: %v", peer.Address, err)
				}
			}
		}

		// Verify block before processing
		if !ns.Blockchain.VerifySingleBlock(&block) {
			log.Printf("Invalid block received from %s", peer.Address)
			recordPeerFailure(peer, "invalid block")
			return
		}

		// Add to pending; non-proposer validators cast a vote and broadcast it
		ns.Blockchain.Mutex.Lock()
		ns.Blockchain.AddPendingBlock(&block)
		localVal := ns.Blockchain.LocalValidator
		isProposer := strings.EqualFold(localVal, block.RewardBreakdown.Validator)
		if localVal != "" && !isProposer {
			ns.Blockchain.AddBlockVote(block.CurrentHash, localVal)
		}
		finalized := ns.Blockchain.TryFinalizePending(block.CurrentHash, 0.67)
		ns.Blockchain.Mutex.Unlock()

		if localVal != "" && !isProposer {
			ns.BroadcastVote(block.CurrentHash, localVal)
		}
		if finalized {
			log.Printf("✅ Block #%d finalized on receipt (peer block)", block.BlockNumber)
		}

		ns.PeerEvents <- PeerEvent{
			Type: "block",
			Peer: peer,
			Data: jsonData,
		}

	case "validator":
		validatorData, ok := msg["data"].(map[string]interface{})
		if !ok {
			log.Printf("Invalid validator data from %s", peer.Address)
			return
		}

		jsonData, err := json.Marshal(validatorData)
		if err != nil {
			log.Printf("Error marshaling validator data: %v", err)
			return
		}

		var validator Validator
		if err := json.Unmarshal(jsonData, &validator); err != nil {
			log.Printf("Error decoding validator: %v", err)
			return
		}

		// Add validator to local blockchain
		ns.Blockchain.Mutex.Lock()
		found := false
		for _, v := range ns.Blockchain.Validators {
			if v.Address == validator.Address {
				found = true
				break
			}
		}
		if !found {
			newValidator := &Validator{
				Address:             validator.Address,
				DEXAddress:          validator.DEXAddress,
				DEXFactoryAddress:   validator.DEXFactoryAddress,
				PairKey:             validator.PairKey,
				Token0:              validator.Token0,
				Token1:              validator.Token1,
				LPTokenAmount:       validator.LPTokenAmount,
				LockedLiquidityUSD:  validator.LockedLiquidityUSD,
				ValidatorPairWeight: validator.ValidatorPairWeight,
				LPStakeAmount:       validator.LPStakeAmount,
				NativeBond:          validator.NativeBond,
				LockTime:            validator.LockTime,
				LiquidityPower:      validator.LiquidityPower,
				PenaltyScore:        validator.PenaltyScore,
				LastActive:          validator.LastActive,
			}
			ns.Blockchain.Validators = append(ns.Blockchain.Validators, newValidator)
			log.Printf("Added new validator from network: %s", validator.Address)
		}
		ns.Blockchain.Mutex.Unlock()

	case "transaction":
		txData, ok := msg["data"].(map[string]interface{})
		if !ok {
			log.Printf("Invalid transaction data from %s", peer.Address)
			return
		}

		jsonData, err := json.Marshal(txData)
		if err != nil {
			log.Printf("Error marshaling transaction data: %v", err)
			return
		}

		var tx Transaction
		if err := json.Unmarshal(jsonData, &tx); err != nil {
			log.Printf("Error decoding transaction: %v", err)
			return
		}

		// Verify transaction before processing
		if !ns.Blockchain.VerifyTransaction(&tx) {
			log.Printf("Invalid transaction received from %s", peer.Address)
			return
		}

		ns.PeerEvents <- PeerEvent{
			Type: "transaction",
			Peer: peer,
			Data: jsonData,
		}

	case "vote":
		hash, _ := msg["hash"].(string)
		validator, _ := msg["validator"].(string)
		if hash == "" || validator == "" {
			return
		}
		ns.Blockchain.Mutex.Lock()
		if !ns.Blockchain.ValidatorCanVote(validator) {
			ns.Blockchain.Mutex.Unlock()
			log.Printf("Rejected vote from ineligible validator %s", validator)
			recordPeerFailure(peer, "ineligible vote")
			return
		}
		alreadySeen := false
		if ns.Blockchain.BlockVotes != nil {
			if voters, ok := ns.Blockchain.BlockVotes[hash]; ok && voters != nil {
				alreadySeen = voters[validator]
			}
		}
		ns.Blockchain.AddBlockVote(hash, validator)
		finalized := ns.Blockchain.TryFinalizePending(hash, 0.67)
		ns.Blockchain.Mutex.Unlock()
		if !alreadySeen {
			go ns.BroadcastVote(hash, validator)
		}
		if finalized {
			log.Printf("✅ Block finalized via vote from %s", validator)
		}

	case "consensus_vote":
		voteData, ok := msg["data"].(map[string]interface{})
		if !ok {
			recordPeerFailure(peer, "invalid consensus vote payload")
			return
		}
		raw, err := json.Marshal(voteData)
		if err != nil {
			return
		}
		var vote ConsensusVote
		if err := json.Unmarshal(raw, &vote); err != nil {
			recordPeerFailure(peer, "invalid consensus vote encoding")
			return
		}
		if peer != nil && peer.ValidatorVerified && !strings.EqualFold(peer.ValidatorAddress, vote.Validator) {
			recordPeerFailure(peer, "consensus vote identity mismatch")
			return
		}
		finalized, err := ns.Blockchain.ProcessConsensusVote(vote)
		if err != nil {
			recordPeerFailure(peer, "rejected consensus vote")
			log.Printf("Rejected signed consensus vote: %v", err)
			return
		}
		if finalized {
			log.Printf("✅ Block #%d finalized with signed precommit QC", vote.Height)
		}

	case "peers":
		peersData, ok := msg["peers"].([]interface{})
		if !ok {
			log.Printf("Invalid peers data from %s", peer.Address)
			return
		}

		for _, p := range peersData {
			peerInfo, ok := p.(map[string]interface{})
			if !ok {
				continue
			}

			address, ok1 := peerInfo["address"].(string)
			port, ok2 := peerInfo["port"].(float64)
			if ok1 && ok2 {
				ns.AddPeer(address, int(port), false)
			}
		}

	default:
		log.Printf("Unknown message type '%s' from %s", msgType, peer.Address)
	}
}

func (ns *NetworkService) SyncValidators(peer *Peer) error {
	conn, err := net.DialTimeout("tcp",
		fmt.Sprintf("%s:%d", peer.Address, peer.Port),
		10*time.Second)
	if err != nil {
		return fmt.Errorf("dial failed: %v", err)
	}
	defer conn.Close()

	decoder := json.NewDecoder(conn)
	if peerVersion, err := ns.sendVersionHandshake(conn, decoder); err != nil {
		return err
	} else {
		ns.applyPeerVersion(peer, peerVersion)
	}

	// Request validators
	request := map[string]interface{}{
		"type": "get_validators",
	}

	if err := json.NewEncoder(conn).Encode(request); err != nil {
		return fmt.Errorf("encode failed: %v", err)
	}

	var response struct {
		Validators []*Validator `json:"validators"`
	}

	if err := decoder.Decode(&response); err != nil {
		return fmt.Errorf("decode failed: %v", err)
	}

	ns.Blockchain.Mutex.Lock()
	defer ns.Blockchain.Mutex.Unlock()

	// Merge validators
	for _, remoteValidator := range response.Validators {
		found := false
		for _, localValidator := range ns.Blockchain.Validators {
			if localValidator.Address == remoteValidator.Address {
				found = true
				break
			}
		}
		if !found {
			ns.Blockchain.Validators = append(ns.Blockchain.Validators, remoteValidator)
			log.Printf("Synced validator from peer: %s", remoteValidator.Address)
		}
	}

	return nil
}

func (ns *NetworkService) SyncAllValidators() {
	if ns == nil {
		return
	}
	ns.Mutex.Lock()
	peers := make([]*Peer, 0, len(ns.Peers))
	for _, p := range ns.Peers {
		if p != nil {
			peers = append(peers, p)
		}
	}
	ns.Mutex.Unlock()

	for _, p := range peers {
		_ = ns.SyncValidators(p)
	}
}
func (ns *NetworkService) AddPeer(address string, port int, isBootstrap bool) {
	peerKey := fmt.Sprintf("%s:%d", address, port)

	ns.Mutex.Lock()
	defer ns.Mutex.Unlock()

	if ns.ListenPort != "" && port == toIntPort(ns.ListenPort) && isLocalAddress(address) {
		return
	}

	_, exists := ns.Peers[peerKey]
	if !exists {
		ns.Peers[peerKey] = &Peer{
			Address:  address,
			Port:     port,
			LastSeen: time.Now(),
			Protocol: protocolVersion,
			IsActive: isBootstrap,
		}
	}
}

func (ns *NetworkService) BroadcastBlock(block *Block) error {

	ns.Mutex.Lock()
	newPool := make([]*Transaction, 0, len(ns.Blockchain.Transaction_pool))
	includedTxs := make(map[string]bool)

	for _, tx := range block.Transactions {
		includedTxs[tx.TxHash] = true
	}

	for _, tx := range ns.Blockchain.Transaction_pool {
		if !includedTxs[tx.TxHash] {
			newPool = append(newPool, tx)
		}
	}

	ns.Blockchain.Transaction_pool = newPool
	ns.Mutex.Unlock()

	data, err := json.Marshal(map[string]interface{}{
		"type": "block",
		"data": block,
		"ttl":  7, // Time-to-live for gossip
	})
	if err != nil {
		return err
	}

	ns.Mutex.Lock()
	defer ns.Mutex.Unlock()

	// Select random peers to gossip to
	targets := make([]*Peer, 0, 3)
	for _, p := range ns.Peers {
		if len(targets) >= 3 { // Fan-out of 3
			break
		}
		if p != nil && !ns.isSelfPeer(p) && time.Since(p.LastSeen) < time.Minute {
			targets = append(targets, p)
		}
	}

	for _, peer := range targets {
		go func(p *Peer) {
			ns.sendData(p, data)
		}(peer)
	}

	return nil
}

func (ns *NetworkService) BroadcastTransaction(tx *Transaction) error {
	data, err := json.Marshal(map[string]interface{}{
		"type": "transaction",
		"data": tx,
	})
	if err != nil {
		return err
	}

	ns.Mutex.Lock()
	defer ns.Mutex.Unlock()

	for _, peer := range ns.Peers {
		if peer == nil || ns.isSelfPeer(peer) {
			continue
		}
		go ns.sendData(peer, data)
	}

	return nil
}

func (ns *NetworkService) BroadcastVote(blockHash string, validator string) {
	if blockHash == "" || validator == "" {
		return
	}
	data, err := json.Marshal(map[string]interface{}{
		"type":      "vote",
		"hash":      blockHash,
		"validator": validator,
	})
	if err != nil {
		return
	}
	ns.Mutex.Lock()
	defer ns.Mutex.Unlock()
	for _, peer := range ns.Peers {
		if peer == nil || ns.isSelfPeer(peer) {
			continue
		}
		go ns.sendData(peer, data)
	}
}

func (ns *NetworkService) BroadcastConsensusVote(vote ConsensusVote) {
	if ns == nil || !VerifyConsensusVote(vote) {
		return
	}
	data, err := json.Marshal(map[string]interface{}{"type": "consensus_vote", "data": vote})
	if err != nil {
		return
	}
	ns.Mutex.Lock()
	defer ns.Mutex.Unlock()
	for _, peer := range ns.Peers {
		if peer == nil || ns.isSelfPeer(peer) {
			continue
		}
		go ns.sendData(peer, data)
	}
}

func (ns *NetworkService) SyncChain() error {

	startTime := time.Now()
	defer func() {
		log.Printf("Sync completed in %v", time.Since(startTime))
	}()

	// Apply reputation decay
	now := time.Now()
	ns.Mutex.Lock()
	peers := make([]*Peer, 0, len(ns.Peers))
	for _, peer := range ns.Peers {
		if peer.Reputation <= 0 || peer.LastUpdated.IsZero() {
			peer.Reputation = 1.0 // Initial reputation
		} else {
			hours := now.Sub(peer.LastUpdated).Hours()
			if hours < 0 {
				hours = 0
			}
			peer.Reputation *= math.Pow(PeerReputationDecay, hours)
			if peer.Reputation < 0.1 {
				peer.Reputation = 0.1 // Minimum reputation
			}
		}
		peer.LastUpdated = now
		peers = append(peers, peer)
	}
	ns.Mutex.Unlock()

	ourHeight := ns.bestHeight()
	var bestPeer *Peer
	bestScore := 0.0

	for _, peer := range peers {
		if err := ns.fetchPeerHeight(peer); err != nil {
			log.Printf("Failed to fetch peer height from %s:%d: %v", peer.Address, peer.Port, err)
			continue
		}
		// Skip peers with low reputation
		if peer.Reputation < peerMinReputationThreshold() {
			continue
		}

		// Calculate peer score (height difference * reputation)
		heightDiff := peer.Height - ourHeight
		if heightDiff <= 0 {
			continue
		}

		score := float64(heightDiff) * peer.Reputation
		if score > bestScore {
			bestScore = score
			bestPeer = peer
		}
	}

	if bestPeer == nil {
		return nil // No better peer found
	}

	log.Printf("Syncing with peer %s (height: %d, reputation: %.2f)",
		bestPeer.Address, bestPeer.Height, bestPeer.Reputation)

	// Implement incremental sync with retries
	var lastErr error
	for attempt := 1; attempt <= MaxSyncAttempts; attempt++ {
		if err := ns.syncWithPeer(bestPeer, ourHeight); err != nil {
			lastErr = err
			// Penalize peer for failed sync
			ns.Mutex.Lock()
			recordPeerFailure(bestPeer, "sync failed")
			ns.Mutex.Unlock()
			log.Printf("Sync attempt %d failed: %v (peer reputation now: %.2f)",
				attempt, err, bestPeer.Reputation)
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}
		// Reward peer for successful sync
		ns.Mutex.Lock()
		recordPeerSuccess(bestPeer, 0.10)
		ns.Mutex.Unlock()
		return nil
	}

	return fmt.Errorf("failed to sync after %d attempts: %v", MaxSyncAttempts, lastErr)
}
func (ns *NetworkService) BroadcastValidator(v *Validator) {
	if ns == nil || v == nil {
		return
	}

	// Marshal once
	payload, err := json.Marshal(v)
	if err != nil {
		log.Printf("BroadcastValidator: marshal error: %v", err)
		return
	}

	ns.Mutex.Lock()
	defer ns.Mutex.Unlock()

	for peerKey, peer := range ns.Peers {
		if peer == nil || peer.HTTPPort == 0 || ns.isSelfPeer(peer) {
			continue
		}
		url := fmt.Sprintf("http://%s:%d/validator/new", peer.Address, peer.HTTPPort)

		go func(k string, p *Peer, u string, body []byte) {
			req, _ := http.NewRequest(http.MethodPost, u, bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			client := &http.Client{Timeout: 3 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				log.Printf("BroadcastValidator -> %s:%d failed: %v", p.Address, p.Port, err)
				return
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				p.LastSeen = time.Now()
				p.LastUpdated = time.Now()
				log.Printf("BroadcastValidator -> %s:%d OK", p.Address, p.Port)
			} else {
				log.Printf("BroadcastValidator -> %s:%d HTTP %d", p.Address, p.Port, resp.StatusCode)
			}
		}(peerKey, peer, url, payload)
	}
}

// ── Bootstrap ────────────────────────────────────────────────────────────────

// DefaultBootstrapPeers lists well-known seed nodes used for multi-node testing.
var DefaultBootstrapPeers = []string{
	"127.0.0.1:6001",
	"127.0.0.1:6002",
}

// Bootstrap connects to known seed nodes on startup so the node can discover
// its peers without manual configuration.
func (ns *NetworkService) Bootstrap(seedPeers []string) {
	for _, addr := range seedPeers {
		parts := strings.Split(addr, ":")
		if len(parts) != 2 {
			continue
		}
		port, err := strconv.Atoi(parts[1])
		if err != nil {
			continue
		}
		peer := &Peer{
			Address: parts[0],
			Port:    port,
		}
		ns.Mutex.Lock()
		key := fmt.Sprintf("%s:%d", peer.Address, peer.Port)
		if _, exists := ns.Peers[key]; !exists {
			ns.Peers[key] = peer
			log.Printf("[Network] Bootstrap peer added: %s", key)
		}
		ns.Mutex.Unlock()
	}
}

// ── HTTP-based mempool broadcast ─────────────────────────────────────────────

// BroadcastTransactionHTTP sends a transaction to all known peers via HTTP.
// This complements the TCP-based BroadcastTransaction for peers that only
// expose an HTTP endpoint.
func (ns *NetworkService) BroadcastTransactionHTTP(tx interface{}) {
	ns.Mutex.Lock()
	peers := make([]*Peer, 0, len(ns.Peers))
	for _, p := range ns.Peers {
		peers = append(peers, p)
	}
	ns.Mutex.Unlock()

	txJSON, err := json.Marshal(tx)
	if err != nil {
		return
	}

	for _, peer := range peers {
		if peer.HTTPPort == 0 {
			continue
		}
		go func(p *Peer) {
			url := fmt.Sprintf("http://%s:%d/send_tx", p.Address, p.HTTPPort)
			client := &http.Client{Timeout: 3 * time.Second}
			resp, err := client.Post(url, "application/json", bytes.NewReader(txJSON))
			if err != nil {
				p.Reputation *= 0.99 // slight reputation decay on failure
				return
			}
			resp.Body.Close()
		}(peer)
	}
}

// ── Retry helper ─────────────────────────────────────────────────────────────

// postToPeerWithRetry sends a POST request to a peer with exponential-backoff
// retries.  It skips peers that have no HTTP port.
func (ns *NetworkService) postToPeerWithRetry(peer *Peer, path string, body []byte, maxRetries int) error {
	if peer.HTTPPort == 0 {
		return fmt.Errorf("peer %s has no HTTP port", peer.Address)
	}
	url := fmt.Sprintf("http://%s:%d%s", peer.Address, peer.HTTPPort, path)
	client := &http.Client{Timeout: 5 * time.Second}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * 500 * time.Millisecond
			if backoff > 10*time.Second {
				backoff = 10 * time.Second
			}
			time.Sleep(backoff)
		}
		resp, err := client.Post(url, "application/json", bytes.NewReader(body))
		if err != nil {
			lastErr = err
			continue
		}
		resp.Body.Close()
		if resp.StatusCode < 500 {
			return nil // success or client error (don't retry 4xx)
		}
		lastErr = fmt.Errorf("server error %d", resp.StatusCode)
	}
	return lastErr
}

// ── Peer health check ─────────────────────────────────────────────────────────

// StartHealthCheck periodically GETs /health on each peer and removes peers
// that have accumulated too many failures (reputation drops below zero).
func (ns *NetworkService) StartHealthCheck() {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			ns.Mutex.Lock()
			snapshot := make(map[string]*Peer, len(ns.Peers))
			for k, v := range ns.Peers {
				snapshot[k] = v
			}
			ns.Mutex.Unlock()

			for key, peer := range snapshot {
				if peer.HTTPPort == 0 {
					continue
				}
				go func(k string, p *Peer) {
					url := fmt.Sprintf("http://%s:%d/health", p.Address, p.HTTPPort)
					client := &http.Client{Timeout: 3 * time.Second}
					resp, err := client.Get(url)
					if err != nil {
						p.Reputation -= 0.1
						if p.Reputation < 0 {
							ns.Mutex.Lock()
							delete(ns.Peers, k)
							ns.Mutex.Unlock()
							log.Printf("[Network] Removed unresponsive peer: %s", k)
						}
						return
					}
					resp.Body.Close()
					if p.Reputation < 1.0 {
						p.Reputation = min(p.Reputation+0.1, 1.0)
					}
				}(key, peer)
			}
		}
	}()
}

func (ns *NetworkService) isSelfPeer(peer *Peer) bool {
	if peer == nil {
		return false
	}
	return isLocalAddress(peer.Address) && peer.Port == toIntPort(ns.ListenPort)
}

func (ns *NetworkService) HasHealthyRemotePeer() bool {
	ns.Mutex.Lock()
	defer ns.Mutex.Unlock()

	for _, peer := range ns.Peers {
		if peer == nil || ns.isSelfPeer(peer) {
			continue
		}
		if peer.IsActive && peer.HTTPPort != 0 && time.Since(peer.LastSeen) < 2*PingInterval {
			return true
		}
	}
	return false
}

func (ns *NetworkService) HealthyRemotePeerCount() int {
	return ns.HealthyRemotePeerCountNearHeight(0)
}

func (ns *NetworkService) HealthyRemotePeerCountNearHeight(localHeight int) int {
	ns.Mutex.Lock()
	defer ns.Mutex.Unlock()

	count := 0
	for _, peer := range ns.Peers {
		if peer == nil || ns.isSelfPeer(peer) {
			continue
		}
		ns.updatePeerSyncStatus(peer, localHeight)
		if ns.peerVotingEligibleLocked(peer, localHeight) {
			count++
		}
	}
	return count
}
