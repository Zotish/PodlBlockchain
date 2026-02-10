package blockchaincomponent

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	protocolVersion       = 1
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
)

type Peer struct {
	Address     string    `json:"address"`
	Port        int       `json:"port"`
	HTTPPort    int       `json:"http_port"`
	LastSeen    time.Time `json:"last_seen"`
	Protocol    int       `json:"protocol"`
	IsActive    bool      `json:"is_active"`
	Reputation  float64   `json:"reputation"`
	LastUpdated time.Time `json:"last_updated"`
	Height      int       `json:"height"`
}

type NetworkService struct {
	Peers      map[string]*Peer   `json:"peers"`
	Blockchain *Blockchain_struct `json:"blockchain"`
	Listener   net.Listener       `json:"-"`
	ListenPort string             `json:"-"`
	HTTPPort   int                `json:"-"`
	Mutex      sync.Mutex         `json:"-"`
	PeerEvents chan PeerEvent     `json:"-"`
	Wg         sync.WaitGroup     `json:"-"`
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
	} else if httpPort, ok := peerVersion["http_port"].(float64); ok {
		peer.HTTPPort = int(httpPort)
	}

	// Pipeline stages
	type batch struct {
		start, end int
		blocks     []*Block
		err        error
	}

	batchChan := make(chan batch)
	resultChan := make(chan batch)

	// Start verifier workers
	var wg sync.WaitGroup
	for i := 0; i < runtime.NumCPU(); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for b := range batchChan {
				for _, block := range b.blocks {
					if !ns.Blockchain.VerifySingleBlock(block) {
						b.err = fmt.Errorf("invalid block at height %d", block.BlockNumber)
						break
					}
				}
				resultChan <- b
			}
		}()
	}

	// Start batched requests
	go func() {
		defer close(batchChan)
		for start := ourHeight; start < peer.Height; start += SyncBatchSize {
			end := start + SyncBatchSize
			if end > peer.Height {
				end = peer.Height
			}

			request := map[string]interface{}{
				"type":        "sync",
				"start_block": start,
				"end_block":   end,
			}

			if err := json.NewEncoder(conn).Encode(request); err != nil {
				batchChan <- batch{err: fmt.Errorf("encode failed: %v", err)}
				return
			}

			var blocks []*Block
			for i := start; i < end; i++ {
				var block Block
				if err := decoder.Decode(&block); err != nil {
					batchChan <- batch{err: fmt.Errorf("decode failed at height %d: %v", i, err)}
					return
				}
				blocks = append(blocks, &block)
			}

			batchChan <- batch{start: start, end: end, blocks: blocks}
		}
	}()

	// Close resultChan after all workers exit.
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Process results
	for b := range resultChan {
		if b.err != nil {
			return b.err
		}

		ns.Blockchain.Mutex.Lock()
		ns.Blockchain.Blocks = append(ns.Blockchain.Blocks[:b.start], b.blocks...)
		ns.Blockchain.Transaction_pool = []*Transaction{}
		ns.Blockchain.Mutex.Unlock()
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
	ourVersion := map[string]interface{}{
		"protocol":    protocolVersion,
		"best_height": len(ns.Blockchain.Blocks),
		"timestamp":   time.Now().Unix(),
		"listen_port": toIntPort(ns.ListenPort),
		"http_port":   ns.HTTPPort,
	}

	if err := json.NewEncoder(conn).Encode(ourVersion); err != nil {
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

	return peerVersion, nil
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

	if height, ok := peerVersion["best_height"].(float64); ok {
		peer.Height = int(height)
	}
	if httpPort, ok := peerVersion["http_port"].(float64); ok {
		peer.HTTPPort = int(httpPort)
	}
	return nil
}
func (ns *NetworkService) processPeerEvents() {
	for event := range ns.PeerEvents {

		ns.Mutex.Lock()

	peer, exists := ns.Peers[peerKey(event.Peer)]
		if exists {
			switch event.Type {
			case "block":
				peer.LastSeen = time.Now()
				// Reward for valid blocks
				peer.Reputation = min(1.0, peer.Reputation*1.05)

			case "transaction":
				peer.LastSeen = time.Now()
				// Small reward for transactions
				peer.Reputation = min(1.0, peer.Reputation*1.01)

			case "invalid_block":
				// Penalize for invalid blocks
				peer.Reputation *= 0.7
				log.Printf("Penalized peer %s for invalid block (new reputation: %.2f)",
					peer.Address, peer.Reputation)
			}
		}

		ns.Mutex.Unlock()

		// this part is commented because it was past code

		// ns.Mutex.Lock()
		// peer, exists := ns.Peers[event.Peer.Address]
		// switch event.Type {
		// case "block":
		// 	var block Block
		// 	if err := json.Unmarshal(event.Data, &block); err != nil {
		// 		log.Printf("Error decoding block: %v", err)
		// 		continue
		// 	}

		// 	// Verify and add block to blockchain
		// 	if ns.Blockchain.VerifySingleBlock(&block) {
		// 		ns.Blockchain.Blocks = append(ns.Blockchain.Blocks, &block)
		// 		ns.Blockchain.Transaction_pool = []*Transaction{} // Clear transaction pool after block is added
		// 	}

		// case "transaction":
		// 	var tx Transaction
		// 	if err := json.Unmarshal(event.Data, &tx); err != nil {
		// 		log.Printf("Error decoding transaction: %v", err)
		// 		continue
		// 	}

		// 	// Verify and add transaction to pool
		// 	if ns.Blockchain.VerifyTransaction(&tx) {
		// 		ns.Blockchain.AddNewTxToTheTransaction_pool(&tx)
		// 	}
		// }
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
		ns.Mutex.Lock()
		for _, peer := range ns.Peers {
			if time.Since(peer.LastSeen) > 2*PingInterval {
				delete(ns.Peers, peer.Address)
				log.Printf("Removed inactive peer: %s:%d", peer.Address, peer.Port)
				continue
			}

			// Send ping
			go func(p *Peer) {
				success := ns.sendData(p, []byte(`{"type":"ping"}`))
				if success {
					ns.Mutex.Lock()
					p.LastSeen = time.Now()
					ns.Mutex.Unlock()
				}
			}(peer)
		}
		ns.Mutex.Unlock()
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
	} else if httpPort, ok := peerVersion["http_port"].(float64); ok {
		peer.HTTPPort = int(httpPort)
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

		if lp, ok := firstMsg["listen_port"].(float64); ok {
			peer.Port = int(lp)
		}
		if hp, ok := firstMsg["http_port"].(float64); ok {
			peer.HTTPPort = int(hp)
		}

		if height, ok := firstMsg["best_height"].(float64); ok {
			peer.Height = int(height)
		}

		// Send our version information
		ourVersion := map[string]interface{}{
			"protocol":    protocolVersion,
			"best_height": len(ns.Blockchain.Blocks),
			"timestamp":   time.Now().Unix(),
			"listen_port": toIntPort(ns.ListenPort),
		}

		if err := json.NewEncoder(conn).Encode(ourVersion); err != nil {
			log.Printf("Error sending our version: %v", err)
			return
		}
		handledHandshake = true
	} else {
		// No handshake provided; treat first message as regular message.
	}

	// 4. Exchange peer lists if this is a bootstrap node
	if peer.IsActive {
		// Request peer list
		if err := json.NewEncoder(conn).Encode(map[string]string{"type": "getpeers"}); err != nil {
			log.Printf("Error requesting peer list: %v", err)
			return
		}

		// Receive peer list
		var peerList struct {
			Peers []struct {
				Address string `json:"address"`
				Port    int    `json:"port"`
			} `json:"peers"`
		}

		if err := decoder.Decode(&peerList); err != nil {
			log.Printf("Error reading peer list: %v", err)
			return
		}

		// Add new peers
		for _, p := range peerList.Peers {
			ns.AddPeer(p.Address, p.Port, false)
		}
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
				ns.Blockchain.Mutex.Lock()
				if s < 0 {
					s = 0
				}
				if e > len(ns.Blockchain.Blocks) {
					e = len(ns.Blockchain.Blocks)
				}
				for i := s; i < e; i++ {
					if err := json.NewEncoder(conn).Encode(ns.Blockchain.Blocks[i]); err != nil {
						ns.Blockchain.Mutex.Unlock()
						log.Printf("Error sending block %d: %v", i, err)
						return false
					}
				}
				ns.Blockchain.Mutex.Unlock()
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

	// Clean up disconnected peer
	ns.Mutex.Lock()
	delete(ns.Peers, peerKey(peer))
	ns.Mutex.Unlock()
}
func (ns *NetworkService) handleMessage(peer *Peer, msg map[string]interface{}) {

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
		localHeight := len(ns.Blockchain.Blocks) - 1
		ns.Blockchain.Mutex.Unlock()
		if int(block.BlockNumber) <= localHeight {
			log.Printf("Stale block received from %s (height %d <= %d)", peer.Address, block.BlockNumber, localHeight)
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
			return
		}

		if ns.Blockchain.LocalValidator != "" && !strings.EqualFold(ns.Blockchain.LocalValidator, block.RewardBreakdown.Validator) {
			ns.Blockchain.Mutex.Lock()
			ns.Blockchain.AddBlockVote(block.CurrentHash, ns.Blockchain.LocalValidator)
			ns.Blockchain.Mutex.Unlock()
			ns.BroadcastVote(block.CurrentHash, ns.Blockchain.LocalValidator)
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
				Address:        validator.Address,
				LPStakeAmount:  validator.LPStakeAmount,
				LockTime:       validator.LockTime,
				LiquidityPower: validator.LiquidityPower,
				PenaltyScore:   validator.PenaltyScore,
				LastActive:     validator.LastActive,
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
		ns.Blockchain.AddBlockVote(hash, validator)
		ns.Blockchain.Mutex.Unlock()

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
	} else if httpPort, ok := peerVersion["http_port"].(float64); ok {
		peer.HTTPPort = int(httpPort)
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
		if p.IsActive && time.Since(p.LastSeen) < time.Minute {
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
		if peer == nil {
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
	ns.Mutex.Lock()
	defer ns.Mutex.Unlock()

	// Apply reputation decay
	now := time.Now()
	for _, peer := range ns.Peers {
		if peer.LastUpdated.IsZero() {
			peer.Reputation = 1.0 // Initial reputation
		} else {
			hours := now.Sub(peer.LastUpdated).Hours()
			peer.Reputation *= PeerReputationDecay * hours
			if peer.Reputation < 0.1 {
				peer.Reputation = 0.1 // Minimum reputation
			}
		}
		peer.LastUpdated = now
	}

	ourHeight := len(ns.Blockchain.Blocks)
	var bestPeer *Peer
	bestScore := 0.0

	for _, peer := range ns.Peers {
		if peer.Height == 0 {
			if err := ns.fetchPeerHeight(peer); err != nil {
				log.Printf("Failed to fetch peer height from %s:%d: %v", peer.Address, peer.Port, err)
			}
		}
		// Skip peers with low reputation
		if peer.Reputation < MinReputationThreshold {
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
			bestPeer.Reputation *= 0.8
			log.Printf("Sync attempt %d failed: %v (peer reputation now: %.2f)",
				attempt, err, bestPeer.Reputation)
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}
		// Reward peer for successful sync
		bestPeer.Reputation = min(1.0, bestPeer.Reputation*1.1)
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
		// Skip dead/inactive peers if you track that
		if peer == nil || !peer.IsActive {
			continue
		}

		url := fmt.Sprintf("http://%s:%d/validator/new", peer.Address, peer.Port)

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
