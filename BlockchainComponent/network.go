// package blockchaincomponent

// import (
// 	"bytes"
// 	"encoding/json"
// 	"fmt"
// 	"io"
// 	"log"
// 	"net"
// 	"net/http"
// 	"runtime"
// 	"strconv"
// 	"sync"
// 	"time"
// )

// const (
// 	protocolVersion       = 1
// 	defaultPort           = "5000" // Default P2P port
// 	PingInterval          = 30 * time.Second
// 	defaultNetworkID      = "mainnet"
// 	PeerDiscoveryInterval = 5 * time.Minute
// 	MaxPeers              = 50
// 	HandshakeTimeout      = 10 * time.Second

// 	SyncBatchSize          = 100
// 	MaxSyncAttempts        = 3
// 	PeerResponseThreshold  = 5 * time.Second
// 	PeerReputationDecay    = 0.9 // 10% decay per hour
// 	MinReputationThreshold = 0.3
// )

// //	type Peer struct {
// //		Address  string    `json:"address"`
// //		Port     int       `json:"port"`
// //		LastSeen time.Time `json:"last_seen"`
// //		Protocol int       `json:"protocol"`
// //		IsActive bool      `json:"is_active"`
// //	}
// type Peer struct {
// 	Address     string    `json:"address"`
// 	Port        int       `json:"port"`
// 	LastSeen    time.Time `json:"last_seen"`
// 	Protocol    int       `json:"protocol"`
// 	IsActive    bool      `json:"is_active"`
// 	Reputation  float64   `json:"reputation"`
// 	LastUpdated time.Time `json:"last_updated"`
// 	Height      int       `json:"height"`
// }

// type NetworkService struct {
// 	Peers      map[string]*Peer   `json:"peers"`
// 	Blockchain *Blockchain_struct `json:"blockchain"`
// 	Listener   net.Listener       `json:"-"`
// 	Mutex      sync.Mutex         `json:"-"`
// 	PeerEvents chan PeerEvent     `json:"-"`
// 	Wg         sync.WaitGroup     `json:"-"`
// }
// type PeerEvent struct {
// 	Type string `json:"type"` // "connect", "Disconnect", "MsgReceived", etc.
// 	Peer *Peer  `json:"peer"` // The peer involved in the event
// 	Data []byte `json:"data"` // Additional data related to the event
// }

// func NewNetworkService(bc *Blockchain_struct) *NetworkService {
// 	newService := new(NetworkService)
// 	newService.Peers = make(map[string]*Peer)
// 	newService.Blockchain = bc
// 	newService.PeerEvents = make(chan PeerEvent, 100)
// 	return newService
// }

// //	func NewNetworkService(bc *Blockchain_struct) *NetworkService {
// //		newService := new(NetworkService)
// //		newService.Peers = make(map[string]*Peer)
// //		newService.Blockchain = bc
// //		newService.PeerEvents = make(chan PeerEvent, 100)
// //		return newService
// //	}

// func (ns *NetworkService) syncWithPeer(peer *Peer, ourHeight int) error {
// 	conn, err := net.DialTimeout("tcp",
// 		fmt.Sprintf("%s:%d", peer.Address, peer.Port),
// 		10*time.Second)
// 	if err != nil {
// 		return fmt.Errorf("dial failed: %v", err)
// 	}
// 	defer conn.Close()

// 	// Pipeline stages
// 	type batch struct {
// 		start, end int
// 		blocks     []*Block
// 		err        error
// 	}

// 	batchChan := make(chan batch)
// 	resultChan := make(chan batch)
// 	defer close(resultChan)

// 	// Start verifier workers
// 	var wg sync.WaitGroup
// 	for i := 0; i < runtime.NumCPU(); i++ {
// 		wg.Add(1)
// 		go func() {
// 			defer wg.Done()
// 			for b := range batchChan {
// 				for _, block := range b.blocks {
// 					if !ns.Blockchain.VerifySingleBlock(block) {
// 						b.err = fmt.Errorf("invalid block at height %d", block.BlockNumber)
// 						break
// 					}
// 				}
// 				resultChan <- b
// 			}
// 		}()
// 	}

// 	// Start batched requests
// 	go func() {
// 		defer close(batchChan)
// 		for start := ourHeight; start < peer.Height; start += SyncBatchSize {
// 			end := start + SyncBatchSize
// 			if end > peer.Height {
// 				end = peer.Height
// 			}

// 			request := map[string]interface{}{
// 				"type":        "sync",
// 				"start_block": start,
// 				"end_block":   end,
// 			}

// 			if err := json.NewEncoder(conn).Encode(request); err != nil {
// 				batchChan <- batch{err: fmt.Errorf("encode failed: %v", err)}
// 				return
// 			}

// 			var blocks []*Block
// 			decoder := json.NewDecoder(conn)
// 			for i := start; i < end; i++ {
// 				var block Block
// 				if err := decoder.Decode(&block); err != nil {
// 					batchChan <- batch{err: fmt.Errorf("decode failed at height %d: %v", i, err)}
// 					return
// 				}
// 				blocks = append(blocks, &block)
// 			}

// 			batchChan <- batch{start: start, end: end, blocks: blocks}
// 		}
// 	}()

// 	// Process results
// 	for b := range resultChan {
// 		if b.err != nil {
// 			return b.err
// 		}

// 		ns.Blockchain.Mutex.Lock()
// 		ns.Blockchain.Blocks = append(ns.Blockchain.Blocks[:b.start], b.blocks...)
// 		ns.Blockchain.Transaction_pool = []*Transaction{}
// 		ns.Blockchain.Mutex.Unlock()
// 	}

// 	wg.Wait()
// 	return nil
// }

// func (ns *NetworkService) Start() error {
// 	listener, err := net.Listen("tcp", ":"+defaultPort)
// 	if err != nil {
// 		return err
// 	}
// 	ns.Listener = listener
// 	ns.Wg.Add(3)
// 	go ns.acceptConnections()
// 	go ns.maintainPeerConnections()
// 	go ns.processPeerEvents()

// 	defaultp, err := strconv.Atoi(defaultPort)
// 	if err != nil {
// 		log.Printf("Error converting default port: %v", err)
// 	}
// 	// Add some bootstrap nodes (in production, these would be configurable)
// 	ns.AddPeer("localhost", defaultp, true)
// 	ns.AddPeer("bootstrap.node.address", 8080, true)         // Replace with actual bootstrap node addresses
// 	ns.AddPeer("another.bootstrap.node.address", 8081, true) // Replace with actual bootstrap node addresses
// 	log.Printf("Network service started on port %s", defaultPort)
// 	return nil

// }
// func (ns *NetworkService) processPeerEvents() {
// 	for event := range ns.PeerEvents {

// 		ns.Mutex.Lock()

// 		peer, exists := ns.Peers[event.Peer.Address]
// 		if exists {
// 			switch event.Type {
// 			case "block":
// 				peer.LastSeen = time.Now()
// 				// Reward for valid blocks
// 				peer.Reputation = min(1.0, peer.Reputation*1.05)

// 			case "transaction":
// 				peer.LastSeen = time.Now()
// 				// Small reward for transactions
// 				peer.Reputation = min(1.0, peer.Reputation*1.01)

// 			case "invalid_block":
// 				// Penalize for invalid blocks
// 				peer.Reputation *= 0.7
// 				log.Printf("Penalized peer %s for invalid block (new reputation: %.2f)",
// 					peer.Address, peer.Reputation)
// 			}
// 		}

// 		ns.Mutex.Unlock()

// 		// this part is commented because it was past code

// 		// ns.Mutex.Lock()
// 		// peer, exists := ns.Peers[event.Peer.Address]
// 		// switch event.Type {
// 		// case "block":
// 		// 	var block Block
// 		// 	if err := json.Unmarshal(event.Data, &block); err != nil {
// 		// 		log.Printf("Error decoding block: %v", err)
// 		// 		continue
// 		// 	}

// 		// 	// Verify and add block to blockchain
// 		// 	if ns.Blockchain.VerifySingleBlock(&block) {
// 		// 		ns.Blockchain.Blocks = append(ns.Blockchain.Blocks, &block)
// 		// 		ns.Blockchain.Transaction_pool = []*Transaction{} // Clear transaction pool after block is added
// 		// 	}

// 		// case "transaction":
// 		// 	var tx Transaction
// 		// 	if err := json.Unmarshal(event.Data, &tx); err != nil {
// 		// 		log.Printf("Error decoding transaction: %v", err)
// 		// 		continue
// 		// 	}

// 		// 	// Verify and add transaction to pool
// 		// 	if ns.Blockchain.VerifyTransaction(&tx) {
// 		// 		ns.Blockchain.AddNewTxToTheTransaction_pool(&tx)
// 		// 	}
// 		// }
// 	}
// }
// func (ns *NetworkService) acceptConnections() {
// 	for {
// 		conn, err := ns.Listener.Accept()
// 		if err != nil {
// 			log.Printf("Error accepting connection: %v", err)
// 			continue
// 		}

// 		go ns.handleConnection(conn)
// 	}
// }
// func (ns *NetworkService) maintainPeerConnections() {
// 	ticker := time.NewTicker(PingInterval)
// 	defer ticker.Stop()

// 	for range ticker.C {
// 		ns.Mutex.Lock()
// 		for _, peer := range ns.Peers {
// 			if time.Since(peer.LastSeen) > 2*PingInterval {
// 				delete(ns.Peers, peer.Address)
// 				log.Printf("Removed inactive peer: %s:%d", peer.Address, peer.Port)
// 				continue
// 			}

// 			// Send ping
// 			go func(p *Peer) {
// 				success := ns.sendData(p, []byte(`{"type":"ping"}`))
// 				if success {
// 					ns.Mutex.Lock()
// 					p.LastSeen = time.Now()
// 					ns.Mutex.Unlock()
// 				}
// 			}(peer)
// 		}
// 		ns.Mutex.Unlock()
// 	}
// }

// func (ns *NetworkService) sendData(peer *Peer, data []byte) bool {
// 	conn, err := net.DialTimeout("tcp", fmt.Sprintf(

// 		"%s:%d",

// 		peer.Address, peer.Port), 5*time.Second)
// 	if err != nil {
// 		log.Printf("Error connecting to peer %s:%d: %v", peer.Address, peer.Port, err)
// 		return false
// 	}
// 	defer conn.Close()

// 	// Set write deadline
// 	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))

// 	_, err = conn.Write(data)
// 	if err != nil {
// 		log.Printf("Error sending data to peer %s:%d: %v", peer.Address, peer.Port, err)
// 		return false
// 	}

// 	// For ping messages, wait for pong response
// 	if string(data) == `{"type":"ping"}` {
// 		conn.SetReadDeadline(time.Now().Add(5 * time.Second))

// 		var response map[string]interface{}
// 		decoder := json.NewDecoder(conn)
// 		if err := decoder.Decode(&response); err != nil {
// 			log.Printf("Error reading pong from %s:%d: %v", peer.Address, peer.Port, err)
// 			return false
// 		}

// 		if response["type"] != "pong" {
// 			log.Printf("Invalid response from %s:%d: expected pong", peer.Address, peer.Port)
// 			return false
// 		}
// 	}

// 	return true
// }
// func min(a, b float64) float64 {
// 	if a < b {
// 		return a
// 	}
// 	return b
// }
// func (ns *NetworkService) handleConnection(conn net.Conn) {
// 	defer conn.Close()
// 	conn.SetDeadline(time.Now().Add(HandshakeTimeout))

// 	// Create the decoder once at the start of the connection
// 	decoder := json.NewDecoder(conn)

// 	// 1. Read peer version
// 	var peerVersion map[string]interface{}
// 	if err := decoder.Decode(&peerVersion); err != nil {
// 		log.Printf("Version handshake failed: %v", err)
// 		return
// 	}

// 	// 2. Verify protocol version exists and is compatible
// 	peerProtocol, ok := peerVersion["protocol"].(float64)
// 	if !ok {
// 		log.Printf("Missing protocol version in handshake")
// 		return
// 	}
// 	if int(peerProtocol) != protocolVersion {
// 		log.Printf("Incompatible protocol: %v (we use %v)", peerProtocol, protocolVersion)
// 		return
// 	}

// 	// 2. Send our version information
// 	ourVersion := map[string]interface{}{
// 		"protocol": protocolVersion,
// 		//"node_id":     ns.Blockchain.NodeID, // You'll need to add NodeID to Blockchain_struct
// 		"best_height": len(ns.Blockchain.Blocks),
// 		"timestamp":   time.Now().Unix(),
// 	}

// 	if err := json.NewEncoder(conn).Encode(ourVersion); err != nil {
// 		log.Printf("Error sending our version: %v", err)
// 		return
// 	}

// 	// 3. Verify peer's blockchain height and capabilities
// 	//peerHeight,ok := peerVersion["best_height"].(float64)
// 	if !ok {
// 		log.Printf("Invalid peer height: %v", peerVersion)
// 		return
// 	}

// 	// Create peer object
// 	peer := &Peer{
// 		Address:  conn.RemoteAddr().String(),
// 		Port:     int(peerVersion["listen_port"].(float64)),
// 		LastSeen: time.Now(),
// 		Protocol: int(peerProtocol),
// 		//Height:   int(peerHeight),
// 		IsActive: false, // Only bootstrap nodes are manually added
// 	}

// 	// 4. Exchange peer lists if this is a bootstrap node
// 	if peer.IsActive {
// 		// Request peer list
// 		if err := json.NewEncoder(conn).Encode(map[string]string{"type": "getpeers"}); err != nil {
// 			log.Printf("Error requesting peer list: %v", err)
// 			return
// 		}

// 		// Receive peer list
// 		var peerList struct {
// 			Peers []struct {
// 				Address string `json:"address"`
// 				Port    int    `json:"port"`
// 			} `json:"peers"`
// 		}

// 		if err := decoder.Decode(&peerList); err != nil {
// 			log.Printf("Error reading peer list: %v", err)
// 			return
// 		}

// 		// Add new peers
// 		for _, p := range peerList.Peers {
// 			ns.AddPeer(p.Address, p.Port, false)
// 		}
// 	}

// 	// Handshake complete, reset deadline
// 	conn.SetDeadline(time.Time{})
// 	// this two line added later
// 	peer.Height = int(peerVersion["best_height"].(float64))
// 	peer.LastUpdated = time.Now()

// 	// Add peer to our list
// 	ns.Mutex.Lock()
// 	ns.Peers[peer.Address] = peer
// 	ns.Mutex.Unlock()

// 	// Handle incoming messages
// 	for {
// 		var msg map[string]interface{}
// 		if err := decoder.Decode(&msg); err != nil {
// 			log.Printf("Error decoding message from %s: %v", peer.Address, err)
// 			break
// 		}

// 		// Handle special control messages
// 		if msgType, ok := msg["type"].(string); ok {
// 			switch msgType {
// 			case "ping":
// 				// Respond to ping
// 				if err := json.NewEncoder(conn).Encode(map[string]string{"type": "pong"}); err != nil {
// 					log.Printf("Error sending pong: %v", err)
// 					break
// 				}
// 				continue
// 			case "getpeers":
// 				// Send our peer list
// 				ns.Mutex.Lock()
// 				peersToSend := make([]map[string]interface{}, 0, len(ns.Peers))
// 				for _, p := range ns.Peers {
// 					peersToSend = append(peersToSend, map[string]interface{}{
// 						"address": p.Address,
// 						"port":    p.Port,
// 					})
// 				}
// 				ns.Mutex.Unlock()

// 				if err := json.NewEncoder(conn).Encode(map[string]interface{}{
// 					"type":  "peers",
// 					"peers": peersToSend,
// 				}); err != nil {
// 					log.Printf("Error sending peer list: %v", err)
// 				}
// 				continue
// 			}
// 		}

// 		ns.handleMessage(peer, msg)
// 	}
// 	for {
// 		var msg map[string]interface{}
// 		if err := decoder.Decode(&msg); err != nil {
// 			log.Printf("Error decoding message from %s: %v", peer.Address, err)
// 			break
// 		}
// 		ns.handleMessage(peer, msg)
// 	}

// 	// Clean up disconnected peer
// 	ns.Mutex.Lock()
// 	delete(ns.Peers, peer.Address)
// 	ns.Mutex.Unlock()
// }
// func (ns *NetworkService) handleMessage(peer *Peer, msg map[string]interface{}) {

// 	msgType, ok := msg["type"].(string)
// 	if !ok {
// 		log.Printf("Message from %s missing type field", peer.Address)
// 		return
// 	}

// 	switch msgType {
// 	case "block":
// 		blockData, ok := msg["data"].(map[string]interface{})
// 		if !ok {
// 			log.Printf("Invalid block data from %s", peer.Address)
// 			return
// 		}

// 		jsonData, err := json.Marshal(blockData)
// 		if err != nil {
// 			log.Printf("Error marshaling block data: %v", err)
// 			return
// 		}

// 		var block Block
// 		if err := json.Unmarshal(jsonData, &block); err != nil {
// 			log.Printf("Error decoding block: %v", err)
// 			return
// 		}

// 		// Verify block before processing
// 		if !ns.Blockchain.VerifySingleBlock(&block) {
// 			log.Printf("Invalid block received from %s", peer.Address)
// 			return
// 		}

// 		ns.PeerEvents <- PeerEvent{
// 			Type: "block",
// 			Peer: peer,
// 			Data: jsonData,
// 		}

// 	case "validator":
// 		validatorData, ok := msg["data"].(map[string]interface{})
// 		if !ok {
// 			log.Printf("Invalid validator data from %s", peer.Address)
// 			return
// 		}

// 		jsonData, err := json.Marshal(validatorData)
// 		if err != nil {
// 			log.Printf("Error marshaling validator data: %v", err)
// 			return
// 		}

// 		var validator Validator
// 		if err := json.Unmarshal(jsonData, &validator); err != nil {
// 			log.Printf("Error decoding validator: %v", err)
// 			return
// 		}

// 		// Add validator to local blockchain
// 		ns.Blockchain.Mutex.Lock()
// 		found := false
// 		for _, v := range ns.Blockchain.Validators {
// 			if v.Address == validator.Address {
// 				found = true
// 				break
// 			}
// 		}
// 		if !found {
// 			newValidator := &Validator{
// 				Address:        validator.Address,
// 				LPStakeAmount:  validator.LPStakeAmount,
// 				LockTime:       validator.LockTime,
// 				LiquidityPower: validator.LiquidityPower,
// 				PenaltyScore:   validator.PenaltyScore,
// 				LastActive:     validator.LastActive,
// 			}
// 			ns.Blockchain.Validators = append(ns.Blockchain.Validators, newValidator)
// 			log.Printf("Added new validator from network: %s", validator.Address)
// 		}
// 		ns.Blockchain.Mutex.Unlock()

// 	case "transaction":
// 		txData, ok := msg["data"].(map[string]interface{})
// 		if !ok {
// 			log.Printf("Invalid transaction data from %s", peer.Address)
// 			return
// 		}

// 		jsonData, err := json.Marshal(txData)
// 		if err != nil {
// 			log.Printf("Error marshaling transaction data: %v", err)
// 			return
// 		}

// 		var tx Transaction
// 		if err := json.Unmarshal(jsonData, &tx); err != nil {
// 			log.Printf("Error decoding transaction: %v", err)
// 			return
// 		}

// 		// Verify transaction before processing
// 		if !ns.Blockchain.VerifyTransaction(&tx) {
// 			log.Printf("Invalid transaction received from %s", peer.Address)
// 			return
// 		}

// 		ns.PeerEvents <- PeerEvent{
// 			Type: "transaction",
// 			Peer: peer,
// 			Data: jsonData,
// 		}

// 	case "peers":
// 		peersData, ok := msg["peers"].([]interface{})
// 		if !ok {
// 			log.Printf("Invalid peers data from %s", peer.Address)
// 			return
// 		}

// 		for _, p := range peersData {
// 			peerInfo, ok := p.(map[string]interface{})
// 			if !ok {
// 				continue
// 			}

// 			address, ok1 := peerInfo["address"].(string)
// 			port, ok2 := peerInfo["port"].(float64)
// 			if ok1 && ok2 {
// 				ns.AddPeer(address, int(port), false)
// 			}
// 		}

// 	default:
// 		log.Printf("Unknown message type '%s' from %s", msgType, peer.Address)
// 	}
// }

// // Add to network.go
// func (ns *NetworkService) SyncValidators(peer *Peer) error {
// 	conn, err := net.DialTimeout("tcp",
// 		fmt.Sprintf("%s:%d", peer.Address, peer.Port),
// 		10*time.Second)
// 	if err != nil {
// 		return fmt.Errorf("dial failed: %v", err)
// 	}
// 	defer conn.Close()

// 	// Request validators
// 	request := map[string]interface{}{
// 		"type": "get_validators",
// 	}

// 	if err := json.NewEncoder(conn).Encode(request); err != nil {
// 		return fmt.Errorf("encode failed: %v", err)
// 	}

// 	var response struct {
// 		Validators []*Validator `json:"validators"`
// 	}

// 	decoder := json.NewDecoder(conn)
// 	if err := decoder.Decode(&response); err != nil {
// 		return fmt.Errorf("decode failed: %v", err)
// 	}

// 	ns.Blockchain.Mutex.Lock()
// 	defer ns.Blockchain.Mutex.Unlock()

// 	// Merge validators
// 	for _, remoteValidator := range response.Validators {
// 		found := false
// 		for _, localValidator := range ns.Blockchain.Validators {
// 			if localValidator.Address == remoteValidator.Address {
// 				found = true
// 				break
// 			}
// 		}
// 		if !found {
// 			ns.Blockchain.Validators = append(ns.Blockchain.Validators, remoteValidator)
// 			log.Printf("Synced validator from peer: %s", remoteValidator.Address)
// 		}
// 	}

// 	return nil
// }
// func (ns *NetworkService) AddPeer(address string, port int, isBootstrap bool) {
// 	peerKey := fmt.Sprintf("%s:%d", address, port)

// 	ns.Mutex.Lock()
// 	defer ns.Mutex.Unlock()

// 	_, exists := ns.Peers[peerKey]
// 	if !exists {
// 		ns.Peers[peerKey] = &Peer{
// 			Address:  address,
// 			Port:     port,
// 			LastSeen: time.Now(),
// 			Protocol: protocolVersion,
// 			IsActive: isBootstrap,
// 		}
// 	}
// }

// func (ns *NetworkService) BroadcastBlock(block *Block) error {

// 	ns.Mutex.Lock()
// 	newPool := make([]*Transaction, 0, len(ns.Blockchain.Transaction_pool))
// 	includedTxs := make(map[string]bool)

// 	for _, tx := range block.Transactions {
// 		includedTxs[tx.TxHash] = true
// 	}

// 	for _, tx := range ns.Blockchain.Transaction_pool {
// 		if !includedTxs[tx.TxHash] {
// 			newPool = append(newPool, tx)
// 		}
// 	}

// 	ns.Blockchain.Transaction_pool = newPool
// 	ns.Mutex.Unlock()

// 	data, err := json.Marshal(map[string]interface{}{
// 		"type": "block",
// 		"data": block,
// 		"ttl":  7, // Time-to-live for gossip
// 	})
// 	if err != nil {
// 		return err
// 	}

// 	ns.Mutex.Lock()
// 	defer ns.Mutex.Unlock()

// 	// Select random peers to gossip to
// 	targets := make([]*Peer, 0, 3)
// 	for _, p := range ns.Peers {
// 		if len(targets) >= 3 { // Fan-out of 3
// 			break
// 		}
// 		if p.IsActive && time.Since(p.LastSeen) < time.Minute {
// 			targets = append(targets, p)
// 		}
// 	}

// 	for _, peer := range targets {
// 		go func(p *Peer) {
// 			ns.sendData(p, data)
// 		}(peer)
// 	}

// 	return nil
// }

// // func (ns *NetworkService) BroadcastBlock(block *Block) error {
// // 	data, err := json.Marshal(map[string]interface{}{
// // 		"type": "block",
// // 		"data": block,
// // 	})
// // 	if err != nil {
// // 		return err
// // 	}

// // 	ns.Mutex.Lock()
// // 	defer ns.Mutex.Unlock()

// // 	for _, peer := range ns.Peers {
// // 		go ns.sendData(peer, data)
// // 	}

// //		return nil
// //	}
// func (ns *NetworkService) BroadcastTransaction(tx *Transaction) error {
// 	data, err := json.Marshal(map[string]interface{}{
// 		"type": "transaction",
// 		"data": tx,
// 	})
// 	if err != nil {
// 		return err
// 	}

// 	ns.Mutex.Lock()
// 	defer ns.Mutex.Unlock()

// 	for _, peer := range ns.Peers {
// 		go ns.sendData(peer, data)
// 	}

// 	return nil
// }

// func (ns *NetworkService) SyncChain() error {

// 	startTime := time.Now()
// 	defer func() {
// 		log.Printf("Sync completed in %v", time.Since(startTime))
// 	}()
// 	ns.Mutex.Lock()
// 	defer ns.Mutex.Unlock()

// 	// Apply reputation decay
// 	now := time.Now()
// 	for _, peer := range ns.Peers {
// 		if peer.LastUpdated.IsZero() {
// 			peer.Reputation = 1.0 // Initial reputation
// 		} else {
// 			hours := now.Sub(peer.LastUpdated).Hours()
// 			peer.Reputation *= PeerReputationDecay * hours
// 			if peer.Reputation < 0.1 {
// 				peer.Reputation = 0.1 // Minimum reputation
// 			}
// 		}
// 		peer.LastUpdated = now
// 	}

// 	ourHeight := len(ns.Blockchain.Blocks)
// 	var bestPeer *Peer
// 	bestScore := 0.0

// 	for _, peer := range ns.Peers {
// 		// Skip peers with low reputation
// 		if peer.Reputation < MinReputationThreshold {
// 			continue
// 		}

// 		// Calculate peer score (height difference * reputation)
// 		heightDiff := peer.Height - ourHeight
// 		if heightDiff <= 0 {
// 			continue
// 		}

// 		score := float64(heightDiff) * peer.Reputation
// 		if score > bestScore {
// 			bestScore = score
// 			bestPeer = peer
// 		}
// 	}

// 	if bestPeer == nil {
// 		return nil // No better peer found
// 	}

// 	log.Printf("Syncing with peer %s (height: %d, reputation: %.2f)",
// 		bestPeer.Address, bestPeer.Height, bestPeer.Reputation)

// 	// Implement incremental sync with retries
// 	var lastErr error
// 	for attempt := 1; attempt <= MaxSyncAttempts; attempt++ {
// 		if err := ns.syncWithPeer(bestPeer, ourHeight); err != nil {
// 			lastErr = err
// 			// Penalize peer for failed sync
// 			bestPeer.Reputation *= 0.8
// 			log.Printf("Sync attempt %d failed: %v (peer reputation now: %.2f)",
// 				attempt, err, bestPeer.Reputation)
// 			time.Sleep(time.Duration(attempt) * time.Second)
// 			continue
// 		}
// 		// Reward peer for successful sync
// 		bestPeer.Reputation = min(1.0, bestPeer.Reputation*1.1)
// 		return nil
// 	}

// 	return fmt.Errorf("failed to sync after %d attempts: %v", MaxSyncAttempts, lastErr)
// }

// // syncWithPeer connects to the given peer and requests missing blocks starting from startHeight.
// // this is the last one func (ns *NetworkService) syncWithPeer(peer *Peer, ourHeight int) error {
// // 	conn, err := net.DialTimeout("tcp",
// // 		fmt.Sprintf("%s:%d", peer.Address, peer.Port),
// // 		10*time.Second)
// // 	if err != nil {
// // 		return fmt.Errorf("dial failed: %v", err)
// // 	}
// // 	defer conn.Close()

// // 	// Set deadlines for the connection
// // 	conn.SetDeadline(time.Now().Add(30 * time.Second))

// // 	// Request blocks in batches
// // 	for start := ourHeight; start < peer.Height; start += SyncBatchSize {
// // 		end := start + SyncBatchSize
// // 		if end > peer.Height {
// // 			end = peer.Height
// // 		}

// // 		request := map[string]interface{}{
// // 			"type":        "sync",
// // 			"start_block": start,
// // 			"end_block":   end,
// // 		}

// // 		if err := json.NewEncoder(conn).Encode(request); err != nil {
// // 			return fmt.Errorf("encode failed: %v", err)
// // 		}

// // 		decoder := json.NewDecoder(conn)
// // 		for i := start; i < end; i++ {
// // 			var block Block
// // 			if err := decoder.Decode(&block); err != nil {
// // 				return fmt.Errorf("decode failed at height %d: %v", i, err)
// // 			}

// // 			if !ns.Blockchain.VerifySingleBlock(&block) {
// // 				return fmt.Errorf("invalid block at height %d", i)
// // 			}

// // 			// Add block to chain
// // 			ns.Blockchain.Blocks = append(ns.Blockchain.Blocks, &block)
// // 			ns.Blockchain.Transaction_pool = []*Transaction{}
// // 		}
// // 	}

// // 	log.Printf("Successfully synced from height %d to %d with peer %s",
// // 		ourHeight, peer.Height, peer.Address)
// // 	return nil
// // }

// //this is the 2nd last one  func (ns *NetworkService) SyncChain() error {
// // 	ns.Mutex.Lock()
// // 	defer ns.Mutex.Unlock()

// // 	// Get our current height
// // 	ourHeight := len(ns.Blockchain.Blocks)

// // 	// Find best peer (considering both height and reputation)
// // 	var bestPeer *Peer
// // 	maxHeight := ourHeight
// // 	bestReputation := 0.0

// // 	for _, peer := range ns.Peers {
// // 		// In a real implementation, you'd get this from peer handshake
// // 		peerHeight := 0 // Placeholder - implement peer height tracking

// // 		// Calculate reputation score (1 - penaltyScore)
// // 		reputation := 1.0
// // 		if peerHeight > maxHeight ||
// // 			(peerHeight == maxHeight && reputation > bestReputation) {
// // 			maxHeight = peerHeight
// // 			bestReputation = reputation
// // 			bestPeer = peer
// // 		}
// // 	}

// // 	if bestPeer == nil || maxHeight <= ourHeight {
// // 		return nil // We have the best chain
// // 	}

// // 	// Implement incremental sync with retries
// // 	maxAttempts := 3
// // 	for attempt := 1; attempt <= maxAttempts; attempt++ {
// // 		conn, err := net.DialTimeout("tcp",
// // 			fmt.Sprintf("%s:%d", bestPeer.Address, bestPeer.Port),
// // 			10*time.Second)
// // 		if err != nil {
// // 			if attempt == maxAttempts {
// // 				return fmt.Errorf("failed to connect after %d attempts: %v", maxAttempts, err)
// // 			}
// // 			time.Sleep(time.Duration(attempt) * time.Second)
// // 			continue
// // 		}
// // 		defer conn.Close()

// // 		// Request blocks in batches
// // 		batchSize := 100
// // 		for start := ourHeight; start < maxHeight; start += batchSize {
// // 			end := start + batchSize
// // 			if end > maxHeight {
// // 				end = maxHeight
// // 			}

// // 			request := map[string]interface{}{
// // 				"type":        "sync",
// // 				"start_block": start,
// // 				"end_block":   end,
// // 			}

// // 			if err := json.NewEncoder(conn).Encode(request); err != nil {
// // 				return err
// // 			}

// // 			decoder := json.NewDecoder(conn)
// // 			for i := start; i < end; i++ {
// // 				var block Block
// // 				if err := decoder.Decode(&block); err != nil {
// // 					return err
// // 				}

// // 				if !ns.Blockchain.VerifySingleBlock(&block) {
// // 					return fmt.Errorf("invalid block received at height %d", i)
// // 				}

// // 				// Add block to chain
// // 				ns.Blockchain.Blocks = append(ns.Blockchain.Blocks, &block)
// // 				ns.Blockchain.Transaction_pool = []*Transaction{}
// // 			}
// // 		}
// // 		break
// // 	}

// // 	return nil
// // }

// // this one was the last one func (ns *NetworkService) SyncChain() error {
// // 	ns.Mutex.Lock()
// // 	defer ns.Mutex.Unlock()

// // 	// Find peer with longest chain
// // 	var bestPeer *Peer
// // 	maxHeight := len(ns.Blockchain.Blocks)

// // 	for _, peer := range ns.Peers {
// // 		// In a real implementation, you'd need to get the peer's height first
// // 		// This is simplified for demonstration
// // 		peerHeight := 0 // You need to implement peer height tracking
// // 		if peerHeight > maxHeight {
// // 			maxHeight = peerHeight
// // 			bestPeer = peer
// // 		}
// // 	}

// // 	if bestPeer == nil {
// // 		return nil // We have the longest chain
// // 	}

// // 	conn, err := net.DialTimeout("tcp",
// // 		fmt.Sprintf("%s:%d", bestPeer.Address, bestPeer.Port),
// // 		10*time.Second)
// // 	if err != nil {
// // 		return err
// // 	}
// // 	defer conn.Close()

// // 	// Send sync request
// // 	request := map[string]interface{}{
// // 		"type":        "sync",
// // 		"start_block": len(ns.Blockchain.Blocks),
// // 	}

// // 	if err := json.NewEncoder(conn).Encode(request); err != nil {
// // 		return err
// // 	}

// // 	// Receive blocks
// // 	decoder := json.NewDecoder(conn)
// // 	for {
// // 		var block Block
// // 		if err := decoder.Decode(&block); err != nil {
// // 			if err == io.EOF {
// // 				break
// // 			}
// // 			return err
// // 		}

// // 		if ns.Blockchain.VerifySingleBlock(&block) {
// // 			ns.Blockchain.Blocks = append(ns.Blockchain.Blocks, &block)
// // 			ns.Blockchain.Transaction_pool = []*Transaction{} // Clear pool
// // 		}
// // 	}

// // 	return nil
// // }

// // func (ns *NetworkService) SyncChain() error {
// // 	ns.Mutex.Lock()
// // 	defer ns.Mutex.Unlock()

// // 	// Find peer with longest chain
// // 	var bestPeer *Peer
// // 	maxHeight := len(ns.Blockchain.Blocks)

// // 	for _, peer := range ns.Peers {
// // 		if peer.height > maxHeight {
// // 			maxHeight = peer.Height
// // 			bestPeer = peer
// // 		}
// // 	}

// // 	if bestPeer == nil {
// // 		return nil // We have the longest chain
// // 	}

// // 	conn, err := net.DialTimeout("tcp",
// // 		fmt.Sprintf("%s:%d", bestPeer.Address, bestPeer.Port),
// // 		10*time.Second)
// // 	if err != nil {
// // 		return err
// // 	}
// // 	defer conn.Close()

// // 	// Create the decoder after establishing connection
// // 	decoder := json.NewDecoder(conn)

// // 	// Send sync request
// // 	request := map[string]interface{}{
// // 		"type":        "sync",
// // 		"start_block": len(ns.Blockchain.Blocks),
// // 	}

// // 	if err := json.NewEncoder(conn).Encode(request); err != nil {
// // 		return err
// // 	}

// // 	// Receive blocks
// // 	for {
// // 		var block Block
// // 		if err := decoder.Decode(&block); err != nil {
// // 			if err == io.EOF {
// // 				break
// // 			}
// // 			return err
// // 		}

// // 		if ns.Blockchain.VerifySingleBlock(&block) {
// // 			ns.Blockchain.Blocks = append(ns.Blockchain.Blocks, &block)
// // 			ns.Blockchain.Transaction_pool = []*Transaction{}
// // 		}
// // 	}

// // 	return nil
// // }

// // In network.go
// // func (ns *NetworkService) SyncChain() error {
// // 	ns.Mutex.Lock()
// // 	defer ns.Mutex.Unlock()

// // 	// Find peer with longest chain
// // 	var bestPeer *Peer
// // 	//maxHeight := len(ns.Blockchain.Blocks)

// // 	// for _, peer := range ns.Peers {
// // 	// 	if peer.Height > maxHeight {
// // 	// 		maxHeight = peer.Height
// // 	// 		bestPeer = peer
// // 	// 	}
// // 	// }

// // 	if bestPeer == nil {
// // 		return nil // We have the longest chain
// // 	}

// // 	// Request blocks we're missing
// // 	conn, err := net.DialTimeout("tcp",
// // 		fmt.Sprintf("%s:%d", bestPeer.Address, bestPeer.Port),
// // 		10*time.Second)
// // 	if err != nil {
// // 		return err
// // 	}
// // 	defer conn.Close()

// // 	// Send sync request
// // 	request := map[string]interface{}{
// // 		"type":        "sync",
// // 		"start_block": len(ns.Blockchain.Blocks),
// // 	}

// // 	if err := json.NewEncoder(conn).Encode(request); err != nil {
// // 		return err
// // 	}

// // 	// Receive blocks
// // 	decoder := json.NewDecoder(conn)
// // 	for {
// // 		var block Block
// // 		if err := decoder.Decode(&block); err != nil {
// // 			if err == io.EOF {
// // 				break
// // 			}
// // 			return err
// // 		}

// // 		// Verify and add block
// // 		if ns.Blockchain.VerifySingleBlock(&block) {
// // 			ns.Blockchain.Blocks = append(ns.Blockchain.Blocks, &block)
// // 			ns.Blockchain.Transaction_pool = []*Transaction{} // Clear pool
// // 		}
// // 	}

// // 	return nil
// // }

// func (ns *NetworkService) BroadcastValidator(v *Validator) {
// 	if ns == nil || v == nil {
// 		return
// 	}

// 	// Marshal once
// 	payload, err := json.Marshal(v)
// 	if err != nil {
// 		log.Printf("BroadcastValidator: marshal error: %v", err)
// 		return
// 	}

// 	ns.Mutex.Lock()
// 	defer ns.Mutex.Unlock()

// 	for peerKey, peer := range ns.Peers {
// 		// Skip dead/inactive peers if you track that
// 		if peer == nil || !peer.IsActive {
// 			continue
// 		}

// 		url := fmt.Sprintf("http://%s:%d/validator/new", peer.Address, peer.Port)

// 		go func(k string, p *Peer, u string, body []byte) {
// 			req, _ := http.NewRequest(http.MethodPost, u, bytes.NewBuffer(body))
// 			req.Header.Set("Content-Type", "application/json")

// 			client := &http.Client{Timeout: 3 * time.Second}
// 			resp, err := client.Do(req)
// 			if err != nil {
// 				log.Printf("BroadcastValidator -> %s:%d failed: %v", p.Address, p.Port, err)
// 				return
// 			}
// 			io.Copy(io.Discard, resp.Body)
// 			resp.Body.Close()

// 			if resp.StatusCode == http.StatusOK {
// 				p.LastSeen = time.Now()
// 				p.LastUpdated = time.Now()
// 				log.Printf("BroadcastValidator -> %s:%d OK", p.Address, p.Port)
// 			} else {
// 				log.Printf("BroadcastValidator -> %s:%d HTTP %d", p.Address, p.Port, resp.StatusCode)
// 			}
// 		}(peerKey, peer, url, payload)
// 	}
// }

// // func (ns *NetworkService) BroadcastValidator(validator *Validator) error {
// // 	data, err := json.Marshal(map[string]interface{}{
// // 		"type": "validator",
// // 		"data": validator,
// // 	})
// // 	if err != nil {
// // 		return err
// // 	}

// // 	ns.Mutex.Lock()
// // 	defer ns.Mutex.Unlock()

// // 	for _, peer := range ns.Peers {
// // 		go ns.sendData(peer, data)
// // 	}
// // 	return nil
// // }

package blockchaincomponent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"runtime"
	"strconv"
	"sync"
	"time"
)

const (
	protocolVersion       = 1
	defaultPort           = "5000" // Default P2P port
	PingInterval          = 30 * time.Second
	defaultNetworkID      = "mainnet"
	PeerDiscoveryInterval = 5 * time.Minute
	MaxPeers              = 50
	HandshakeTimeout      = 10 * time.Second

	SyncBatchSize          = 100
	MaxSyncAttempts        = 3
	PeerResponseThreshold  = 5 * time.Second
	PeerReputationDecay    = 0.9 // 10% decay per hour
	MinReputationThreshold = 0.3
)

//	type Peer struct {
//		Address  string    `json:"address"`
//		Port     int       `json:"port"`
//		LastSeen time.Time `json:"last_seen"`
//		Protocol int       `json:"protocol"`
//		IsActive bool      `json:"is_active"`
//	}
type Peer struct {
	Address     string    `json:"address"`
	Port        int       `json:"port"`
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
	Mutex      sync.Mutex         `json:"-"`
	PeerEvents chan PeerEvent     `json:"-"`
	Wg         sync.WaitGroup     `json:"-"`
}
type PeerEvent struct {
	Type string `json:"type"` // "connect", "Disconnect", "MsgReceived", etc.
	Peer *Peer  `json:"peer"` // The peer involved in the event
	Data []byte `json:"data"` // Additional data related to the event
}

func NewNetworkService(bc *Blockchain_struct) *NetworkService {
	newService := new(NetworkService)
	newService.Peers = make(map[string]*Peer)
	newService.Blockchain = bc
	newService.PeerEvents = make(chan PeerEvent, 100)
	return newService
}

//	func NewNetworkService(bc *Blockchain_struct) *NetworkService {
//		newService := new(NetworkService)
//		newService.Peers = make(map[string]*Peer)
//		newService.Blockchain = bc
//		newService.PeerEvents = make(chan PeerEvent, 100)
//		return newService
//	}

func (ns *NetworkService) syncWithPeer(peer *Peer, ourHeight int) error {
	conn, err := net.DialTimeout("tcp",
		fmt.Sprintf("%s:%d", peer.Address, peer.Port),
		10*time.Second)
	if err != nil {
		return fmt.Errorf("dial failed: %v", err)
	}
	defer conn.Close()

	// Pipeline stages
	type batch struct {
		start, end int
		blocks     []*Block
		err        error
	}

	batchChan := make(chan batch)
	resultChan := make(chan batch)
	defer close(resultChan)

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
			decoder := json.NewDecoder(conn)
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

	wg.Wait()
	return nil
}

func (ns *NetworkService) Start() error {
	listener, err := net.Listen("tcp", ":"+defaultPort)
	if err != nil {
		return err
	}
	ns.Listener = listener
	ns.Wg.Add(3)
	go ns.acceptConnections()
	go ns.maintainPeerConnections()
	go ns.processPeerEvents()

	defaultp, err := strconv.Atoi(defaultPort)
	if err != nil {
		log.Printf("Error converting default port: %v", err)
	}
	// Add some bootstrap nodes (in production, these would be configurable)
	ns.AddPeer("localhost", defaultp, true)
	ns.AddPeer("bootstrap.node.address", 8080, true)         // Replace with actual bootstrap node addresses
	ns.AddPeer("another.bootstrap.node.address", 8081, true) // Replace with actual bootstrap node addresses
	log.Printf("Network service started on port %s", defaultPort)
	return nil

}
func (ns *NetworkService) processPeerEvents() {
	for event := range ns.PeerEvents {

		ns.Mutex.Lock()

		peer, exists := ns.Peers[event.Peer.Address]
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
func (ns *NetworkService) handleConnection(conn net.Conn) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(HandshakeTimeout))

	// Create the decoder once at the start of the connection
	decoder := json.NewDecoder(conn)

	// 1. Read peer version
	var peerVersion map[string]interface{}
	if err := decoder.Decode(&peerVersion); err != nil {
		log.Printf("Version handshake failed: %v", err)
		return
	}

	// 2. Verify protocol version exists and is compatible
	peerProtocol, ok := peerVersion["protocol"].(float64)
	if !ok {
		log.Printf("Missing protocol version in handshake")
		return
	}
	if int(peerProtocol) != protocolVersion {
		log.Printf("Incompatible protocol: %v (we use %v)", peerProtocol, protocolVersion)
		return
	}

	// 2. Send our version information
	ourVersion := map[string]interface{}{
		"protocol": protocolVersion,
		//"node_id":     ns.Blockchain.NodeID, // You'll need to add NodeID to Blockchain_struct
		"best_height": len(ns.Blockchain.Blocks),
		"timestamp":   time.Now().Unix(),
	}

	if err := json.NewEncoder(conn).Encode(ourVersion); err != nil {
		log.Printf("Error sending our version: %v", err)
		return
	}

	// 3. Verify peer's blockchain height and capabilities
	//peerHeight,ok := peerVersion["best_height"].(float64)
	if !ok {
		log.Printf("Invalid peer height: %v", peerVersion)
		return
	}

	// Create peer object
	peer := &Peer{
		Address:  conn.RemoteAddr().String(),
		Port:     int(peerVersion["listen_port"].(float64)),
		LastSeen: time.Now(),
		Protocol: int(peerProtocol),
		//Height:   int(peerHeight),
		IsActive: false, // Only bootstrap nodes are manually added
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
	// this two line added later
	peer.Height = int(peerVersion["best_height"].(float64))
	peer.LastUpdated = time.Now()

	// Add peer to our list
	ns.Mutex.Lock()
	ns.Peers[peer.Address] = peer
	ns.Mutex.Unlock()

	// Handle incoming messages
	for {
		var msg map[string]interface{}
		if err := decoder.Decode(&msg); err != nil {
			log.Printf("Error decoding message from %s: %v", peer.Address, err)
			break
		}

		// Handle special control messages
		if msgType, ok := msg["type"].(string); ok {
			switch msgType {
			case "ping":
				// Respond to ping
				if err := json.NewEncoder(conn).Encode(map[string]string{"type": "pong"}); err != nil {
					log.Printf("Error sending pong: %v", err)
					break
				}
				continue
			case "getpeers":
				// Send our peer list
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
				}
				continue
			}
		}

		ns.handleMessage(peer, msg)
	}
	for {
		var msg map[string]interface{}
		if err := decoder.Decode(&msg); err != nil {
			log.Printf("Error decoding message from %s: %v", peer.Address, err)
			break
		}
		ns.handleMessage(peer, msg)
	}

	// Clean up disconnected peer
	ns.Mutex.Lock()
	delete(ns.Peers, peer.Address)
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

		// Verify block before processing
		if !ns.Blockchain.VerifySingleBlock(&block) {
			log.Printf("Invalid block received from %s", peer.Address)
			return
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

// Add to network.go
func (ns *NetworkService) SyncValidators(peer *Peer) error {
	conn, err := net.DialTimeout("tcp",
		fmt.Sprintf("%s:%d", peer.Address, peer.Port),
		10*time.Second)
	if err != nil {
		return fmt.Errorf("dial failed: %v", err)
	}
	defer conn.Close()

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

	decoder := json.NewDecoder(conn)
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
func (ns *NetworkService) AddPeer(address string, port int, isBootstrap bool) {
	peerKey := fmt.Sprintf("%s:%d", address, port)

	ns.Mutex.Lock()
	defer ns.Mutex.Unlock()

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

// func (ns *NetworkService) BroadcastBlock(block *Block) error {
// 	data, err := json.Marshal(map[string]interface{}{
// 		"type": "block",
// 		"data": block,
// 	})
// 	if err != nil {
// 		return err
// 	}

// 	ns.Mutex.Lock()
// 	defer ns.Mutex.Unlock()

// 	for _, peer := range ns.Peers {
// 		go ns.sendData(peer, data)
// 	}

//		return nil
//	}
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

// syncWithPeer connects to the given peer and requests missing blocks starting from startHeight.
// this is the last one func (ns *NetworkService) syncWithPeer(peer *Peer, ourHeight int) error {
// 	conn, err := net.DialTimeout("tcp",
// 		fmt.Sprintf("%s:%d", peer.Address, peer.Port),
// 		10*time.Second)
// 	if err != nil {
// 		return fmt.Errorf("dial failed: %v", err)
// 	}
// 	defer conn.Close()

// 	// Set deadlines for the connection
// 	conn.SetDeadline(time.Now().Add(30 * time.Second))

// 	// Request blocks in batches
// 	for start := ourHeight; start < peer.Height; start += SyncBatchSize {
// 		end := start + SyncBatchSize
// 		if end > peer.Height {
// 			end = peer.Height
// 		}

// 		request := map[string]interface{}{
// 			"type":        "sync",
// 			"start_block": start,
// 			"end_block":   end,
// 		}

// 		if err := json.NewEncoder(conn).Encode(request); err != nil {
// 			return fmt.Errorf("encode failed: %v", err)
// 		}

// 		decoder := json.NewDecoder(conn)
// 		for i := start; i < end; i++ {
// 			var block Block
// 			if err := decoder.Decode(&block); err != nil {
// 				return fmt.Errorf("decode failed at height %d: %v", i, err)
// 			}

// 			if !ns.Blockchain.VerifySingleBlock(&block) {
// 				return fmt.Errorf("invalid block at height %d", i)
// 			}

// 			// Add block to chain
// 			ns.Blockchain.Blocks = append(ns.Blockchain.Blocks, &block)
// 			ns.Blockchain.Transaction_pool = []*Transaction{}
// 		}
// 	}

// 	log.Printf("Successfully synced from height %d to %d with peer %s",
// 		ourHeight, peer.Height, peer.Address)
// 	return nil
// }

//this is the 2nd last one  func (ns *NetworkService) SyncChain() error {
// 	ns.Mutex.Lock()
// 	defer ns.Mutex.Unlock()

// 	// Get our current height
// 	ourHeight := len(ns.Blockchain.Blocks)

// 	// Find best peer (considering both height and reputation)
// 	var bestPeer *Peer
// 	maxHeight := ourHeight
// 	bestReputation := 0.0

// 	for _, peer := range ns.Peers {
// 		// In a real implementation, you'd get this from peer handshake
// 		peerHeight := 0 // Placeholder - implement peer height tracking

// 		// Calculate reputation score (1 - penaltyScore)
// 		reputation := 1.0
// 		if peerHeight > maxHeight ||
// 			(peerHeight == maxHeight && reputation > bestReputation) {
// 			maxHeight = peerHeight
// 			bestReputation = reputation
// 			bestPeer = peer
// 		}
// 	}

// 	if bestPeer == nil || maxHeight <= ourHeight {
// 		return nil // We have the best chain
// 	}

// 	// Implement incremental sync with retries
// 	maxAttempts := 3
// 	for attempt := 1; attempt <= maxAttempts; attempt++ {
// 		conn, err := net.DialTimeout("tcp",
// 			fmt.Sprintf("%s:%d", bestPeer.Address, bestPeer.Port),
// 			10*time.Second)
// 		if err != nil {
// 			if attempt == maxAttempts {
// 				return fmt.Errorf("failed to connect after %d attempts: %v", maxAttempts, err)
// 			}
// 			time.Sleep(time.Duration(attempt) * time.Second)
// 			continue
// 		}
// 		defer conn.Close()

// 		// Request blocks in batches
// 		batchSize := 100
// 		for start := ourHeight; start < maxHeight; start += batchSize {
// 			end := start + batchSize
// 			if end > maxHeight {
// 				end = maxHeight
// 			}

// 			request := map[string]interface{}{
// 				"type":        "sync",
// 				"start_block": start,
// 				"end_block":   end,
// 			}

// 			if err := json.NewEncoder(conn).Encode(request); err != nil {
// 				return err
// 			}

// 			decoder := json.NewDecoder(conn)
// 			for i := start; i < end; i++ {
// 				var block Block
// 				if err := decoder.Decode(&block); err != nil {
// 					return err
// 				}

// 				if !ns.Blockchain.VerifySingleBlock(&block) {
// 					return fmt.Errorf("invalid block received at height %d", i)
// 				}

// 				// Add block to chain
// 				ns.Blockchain.Blocks = append(ns.Blockchain.Blocks, &block)
// 				ns.Blockchain.Transaction_pool = []*Transaction{}
// 			}
// 		}
// 		break
// 	}

// 	return nil
// }

// this one was the last one func (ns *NetworkService) SyncChain() error {
// 	ns.Mutex.Lock()
// 	defer ns.Mutex.Unlock()

// 	// Find peer with longest chain
// 	var bestPeer *Peer
// 	maxHeight := len(ns.Blockchain.Blocks)

// 	for _, peer := range ns.Peers {
// 		// In a real implementation, you'd need to get the peer's height first
// 		// This is simplified for demonstration
// 		peerHeight := 0 // You need to implement peer height tracking
// 		if peerHeight > maxHeight {
// 			maxHeight = peerHeight
// 			bestPeer = peer
// 		}
// 	}

// 	if bestPeer == nil {
// 		return nil // We have the longest chain
// 	}

// 	conn, err := net.DialTimeout("tcp",
// 		fmt.Sprintf("%s:%d", bestPeer.Address, bestPeer.Port),
// 		10*time.Second)
// 	if err != nil {
// 		return err
// 	}
// 	defer conn.Close()

// 	// Send sync request
// 	request := map[string]interface{}{
// 		"type":        "sync",
// 		"start_block": len(ns.Blockchain.Blocks),
// 	}

// 	if err := json.NewEncoder(conn).Encode(request); err != nil {
// 		return err
// 	}

// 	// Receive blocks
// 	decoder := json.NewDecoder(conn)
// 	for {
// 		var block Block
// 		if err := decoder.Decode(&block); err != nil {
// 			if err == io.EOF {
// 				break
// 			}
// 			return err
// 		}

// 		if ns.Blockchain.VerifySingleBlock(&block) {
// 			ns.Blockchain.Blocks = append(ns.Blockchain.Blocks, &block)
// 			ns.Blockchain.Transaction_pool = []*Transaction{} // Clear pool
// 		}
// 	}

// 	return nil
// }

// func (ns *NetworkService) SyncChain() error {
// 	ns.Mutex.Lock()
// 	defer ns.Mutex.Unlock()

// 	// Find peer with longest chain
// 	var bestPeer *Peer
// 	maxHeight := len(ns.Blockchain.Blocks)

// 	for _, peer := range ns.Peers {
// 		if peer.height > maxHeight {
// 			maxHeight = peer.Height
// 			bestPeer = peer
// 		}
// 	}

// 	if bestPeer == nil {
// 		return nil // We have the longest chain
// 	}

// 	conn, err := net.DialTimeout("tcp",
// 		fmt.Sprintf("%s:%d", bestPeer.Address, bestPeer.Port),
// 		10*time.Second)
// 	if err != nil {
// 		return err
// 	}
// 	defer conn.Close()

// 	// Create the decoder after establishing connection
// 	decoder := json.NewDecoder(conn)

// 	// Send sync request
// 	request := map[string]interface{}{
// 		"type":        "sync",
// 		"start_block": len(ns.Blockchain.Blocks),
// 	}

// 	if err := json.NewEncoder(conn).Encode(request); err != nil {
// 		return err
// 	}

// 	// Receive blocks
// 	for {
// 		var block Block
// 		if err := decoder.Decode(&block); err != nil {
// 			if err == io.EOF {
// 				break
// 			}
// 			return err
// 		}

// 		if ns.Blockchain.VerifySingleBlock(&block) {
// 			ns.Blockchain.Blocks = append(ns.Blockchain.Blocks, &block)
// 			ns.Blockchain.Transaction_pool = []*Transaction{}
// 		}
// 	}

// 	return nil
// }

// In network.go
// func (ns *NetworkService) SyncChain() error {
// 	ns.Mutex.Lock()
// 	defer ns.Mutex.Unlock()

// 	// Find peer with longest chain
// 	var bestPeer *Peer
// 	//maxHeight := len(ns.Blockchain.Blocks)

// 	// for _, peer := range ns.Peers {
// 	// 	if peer.Height > maxHeight {
// 	// 		maxHeight = peer.Height
// 	// 		bestPeer = peer
// 	// 	}
// 	// }

// 	if bestPeer == nil {
// 		return nil // We have the longest chain
// 	}

// 	// Request blocks we're missing
// 	conn, err := net.DialTimeout("tcp",
// 		fmt.Sprintf("%s:%d", bestPeer.Address, bestPeer.Port),
// 		10*time.Second)
// 	if err != nil {
// 		return err
// 	}
// 	defer conn.Close()

// 	// Send sync request
// 	request := map[string]interface{}{
// 		"type":        "sync",
// 		"start_block": len(ns.Blockchain.Blocks),
// 	}

// 	if err := json.NewEncoder(conn).Encode(request); err != nil {
// 		return err
// 	}

// 	// Receive blocks
// 	decoder := json.NewDecoder(conn)
// 	for {
// 		var block Block
// 		if err := decoder.Decode(&block); err != nil {
// 			if err == io.EOF {
// 				break
// 			}
// 			return err
// 		}

// 		// Verify and add block
// 		if ns.Blockchain.VerifySingleBlock(&block) {
// 			ns.Blockchain.Blocks = append(ns.Blockchain.Blocks, &block)
// 			ns.Blockchain.Transaction_pool = []*Transaction{} // Clear pool
// 		}
// 	}

// 	return nil
// }

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

// func (ns *NetworkService) BroadcastValidator(validator *Validator) error {
// 	data, err := json.Marshal(map[string]interface{}{
// 		"type": "validator",
// 		"data": validator,
// 	})
// 	if err != nil {
// 		return err
// 	}

// 	ns.Mutex.Lock()
// 	defer ns.Mutex.Unlock()

// 	for _, peer := range ns.Peers {
// 		go ns.sendData(peer, data)
// 	}
// 	return nil
// }
