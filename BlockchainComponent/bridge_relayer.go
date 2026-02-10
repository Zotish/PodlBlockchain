package blockchaincomponent

import (
	"bufio"
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"os"
	"strconv"
	"strings"
	"time"

	constantset "github.com/Zotish/DefenceProject/ConstantSet"
	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

const bscBridgeABI = `[
  {"inputs":[{"internalType":"address","name":"to","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"},{"internalType":"bytes32","name":"id","type":"bytes32"}],"name":"mint","outputs":[],"stateMutability":"nonpayable","type":"function"},
  {"inputs":[{"internalType":"uint256","name":"amount","type":"uint256"},{"internalType":"string","name":"toLqd","type":"string"}],"name":"burn","outputs":[],"stateMutability":"nonpayable","type":"function"},
  {"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"from","type":"address"},{"indexed":false,"internalType":"uint256","name":"amount","type":"uint256"},{"indexed":false,"internalType":"bytes32","name":"id","type":"bytes32"},{"indexed":false,"internalType":"string","name":"toLqd","type":"string"}],"name":"Burn","type":"event"}
]`

const bscTokenLockABI = `[
  {"inputs":[{"internalType":"address","name":"token","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"},{"internalType":"string","name":"toLqd","type":"string"}],"name":"lock","outputs":[{"internalType":"bytes32","name":"","type":"bytes32"}],"stateMutability":"nonpayable","type":"function"},
  {"inputs":[{"internalType":"address","name":"token","type":"address"},{"internalType":"address","name":"to","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"},{"internalType":"bytes32","name":"id","type":"bytes32"}],"name":"release","outputs":[],"stateMutability":"nonpayable","type":"function"},
  {"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"token","type":"address"},{"indexed":true,"internalType":"address","name":"from","type":"address"},{"indexed":false,"internalType":"uint256","name":"amount","type":"uint256"},{"indexed":false,"internalType":"bytes32","name":"id","type":"bytes32"},{"indexed":false,"internalType":"string","name":"toLqd","type":"string"}],"name":"Locked","type":"event"}
]`

const erc20MetaABI = `[
  {"inputs":[],"name":"name","outputs":[{"internalType":"string","name":"","type":"string"}],"stateMutability":"view","type":"function"},
  {"inputs":[],"name":"symbol","outputs":[{"internalType":"string","name":"","type":"string"}],"stateMutability":"view","type":"function"},
  {"inputs":[],"name":"decimals","outputs":[{"internalType":"uint8","name":"","type":"uint8"}],"stateMutability":"view","type":"function"}
]`

func StartBridgeRelayer(bc *Blockchain_struct) {
	rpc := os.Getenv("BSC_TESTNET_RPC")
	pk := os.Getenv("BSC_TESTNET_PRIVATE_KEY")
	bridgeAddr := os.Getenv("BSC_BRIDGE_ADDRESS")
	lockAddr := os.Getenv("BSC_LOCK_ADDRESS")
	if lockAddr == "" || strings.EqualFold(lockAddr, bridgeAddr) {
		if v := readEnvKey(".env", "BSC_LOCK_ADDRESS"); v != "" {
			lockAddr = v
			_ = os.Setenv("BSC_LOCK_ADDRESS", v)
		}
	}
	if rpc == "" || pk == "" || bridgeAddr == "" {
		log.Println("Bridge relayer disabled: missing BSC_TESTNET_RPC or BSC_TESTNET_PRIVATE_KEY or BSC_BRIDGE_ADDRESS")
		return
	}
	log.Printf("Bridge relayer: rpc=%s bridge=%s lock=%s", rpc, bridgeAddr, lockAddr)

	interval := 5 * time.Second
	if v := os.Getenv("BRIDGE_POLL_INTERVAL_SEC"); v != "" {
		if sec, err := strconv.Atoi(v); err == nil && sec > 0 {
			interval = time.Duration(sec) * time.Second
		}
	}
	backfill := uint64(200)
	if v := os.Getenv("BRIDGE_BACKFILL_BLOCKS"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil && n > 0 {
			backfill = n
		}
	}

	chainID := int64(97)
	if v := os.Getenv("BSC_CHAIN_ID"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil && id > 0 {
			chainID = id
		}
	}

	go func() {
		client, err := ethclient.Dial(rpc)
		if err != nil {
			log.Printf("Bridge relayer: cannot connect to BSC RPC: %v", err)
			return
		}
		parsedABI, err := abi.JSON(strings.NewReader(bscBridgeABI))
		if err != nil {
			log.Printf("Bridge relayer: ABI parse error: %v", err)
			return
		}
		parsedLockABI, err := abi.JSON(strings.NewReader(bscTokenLockABI))
		if err != nil {
			log.Printf("Bridge relayer: lock ABI parse error: %v", err)
			return
		}
		parsedErc20ABI, err := abi.JSON(strings.NewReader(erc20MetaABI))
		if err != nil {
			log.Printf("Bridge relayer: ERC20 ABI parse error: %v", err)
			return
		}
		bridge := common.HexToAddress(bridgeAddr)
		lock := common.Address{}
		if lockAddr != "" {
			lock = common.HexToAddress(lockAddr)
		}
		if lockAddr == "" || strings.EqualFold(lock.Hex(), bridge.Hex()) {
			log.Printf("Bridge relayer: lock address missing or equals bridge (lock=%s bridge=%s)", lock.Hex(), bridge.Hex())
		}

		var lastChecked uint64
		var lastLockChecked uint64

		for {
			latest, err := client.BlockNumber(context.Background())
			if err != nil {
				log.Printf("Bridge relayer: failed to fetch latest block: %v", err)
				time.Sleep(interval)
				continue
			}
			if latest == 0 {
				time.Sleep(interval)
				continue
			}

			maxRange := uint64(40000)
			if v := os.Getenv("BRIDGE_MAX_RANGE"); v != "" {
				if n, err := strconv.ParseUint(v, 10, 64); err == nil && n > 0 {
					maxRange = n
				}
			}

			startBurn := lastChecked + 1
			if lastChecked == 0 {
				if latest > backfill {
					startBurn = latest - backfill
				} else {
					startBurn = 1
				}
			}

			startLock := lastLockChecked + 1
			if lastLockChecked == 0 {
				if latest > backfill {
					startLock = latest - backfill
				} else {
					startLock = 1
				}
			}
			if startBurn < 1 {
				startBurn = 1
			}
			if startLock < 1 {
				startLock = 1
			}
			log.Printf("Bridge relayer: latest=%d startBurn=%d startLock=%d", latest, startBurn, startLock)

			// Scan burns in chunks
			for from := startBurn; from <= latest; {
				to := from + maxRange
				if to > latest {
					to = latest
				}
				if from > to {
					break
				}
				log.Printf("Bridge relayer: scanning burn events %d -> %d", from, to)
				if err := handleBurnEvents(client, parsedABI, bridge, from, to, bc); err != nil {
					log.Printf("Bridge relayer: burn event scan error: %v", err)
					break
				}
				lastChecked = to
				if to == latest {
					break
				}
				from = to + 1
			}

			// Scan locks in chunks
			if lockAddr != "" {
				for from := startLock; from <= latest; {
					to := from + maxRange
					if to > latest {
						to = latest
					}
					if from > to {
						break
					}
					log.Printf("Bridge relayer: scanning lock events %d -> %d", from, to)
					if err := handleBscTokenLocks(client, parsedLockABI, parsedErc20ABI, lock, from, to, bc); err != nil {
						log.Printf("Bridge relayer: bsc lock scan error: %v", err)
						break
					}
					lastLockChecked = to
					if to == latest {
						break
					}
					from = to + 1
				}
			}

			// 1) Mint for locked requests
			reqs := bc.ListBridgeRequests("")
			for _, r := range reqs {
				if r.Status != BridgeStatusLocked {
					continue
				}
				if r.To == "" || strings.EqualFold(r.SourceChain, "BSC") {
					continue
				}
				if err := sendMint(client, parsedABI, bridge, pk, chainID, r, bc); err != nil {
					log.Printf("Bridge relayer: mint failed for %s: %v", r.ID, err)
					continue
				}
			}

			// 1b) Fallback mint for BSC->LQD requests (if RPC log scan missed)
			reqs = bc.ListBridgeRequests("")
			for _, r := range reqs {
				if r.Status != BridgeStatusLocked {
					continue
				}
				if !strings.EqualFold(r.SourceChain, "BSC") || !strings.EqualFold(r.TargetChain, "LQD") {
					continue
				}
				if r.Token == "" || r.To == "" {
					continue
				}
				if err := mintFromBscRequest(client, parsedErc20ABI, r, bc); err != nil {
					log.Printf("Bridge relayer: mint from BSC request failed for %s: %v", r.ID, err)
					continue
				}
			}

			// 4) Process LQD burns -> release on BSC
			for _, r := range bc.ListBridgeRequests("") {
				if r.Status != BridgeStatusBurned || !strings.EqualFold(r.TargetChain, "BSC") {
					continue
				}
				if r.Token == "" || r.To == "" {
					continue
				}
				if lockAddr == "" {
					continue
				}
				if err := sendRelease(client, parsedLockABI, lock, pk, chainID, r); err != nil {
					log.Printf("Bridge relayer: release failed for %s: %v", r.ID, err)
					continue
				}
				bc.MarkBridgeUnlocked(r.ID)
			}

			time.Sleep(interval)
		}
	}()
}

func readEnvKey(path, key string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		k := strings.TrimSpace(parts[0])
		v := strings.TrimSpace(parts[1])
		v = strings.Trim(v, `"'`)
		if k == key {
			return v
		}
	}
	return ""
}

func sendMint(client *ethclient.Client, parsedABI abi.ABI, bridge common.Address, privKeyHex string, chainID int64, r *BridgeRequest, bc *Blockchain_struct) error {
	key, err := crypto.HexToECDSA(strings.TrimPrefix(privKeyHex, "0x"))
	if err != nil {
		return err
	}
	auth, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(chainID))
	if err != nil {
		return err
	}
	auth.Context = context.Background()

	id := common.HexToHash(r.LqdTxHash)
	to := common.HexToAddress(r.To)
	amount, err := NewAmountFromString(r.Amount)
	if err != nil {
		return err
	}

	contract := bind.NewBoundContract(bridge, parsedABI, client, client, client)
	tx, err := contract.Transact(auth, "mint", to, amount, id)
	if err != nil {
		return err
	}
	log.Printf("Bridge relayer: mint tx sent %s for %s", tx.Hash().Hex(), r.ID)
	bc.MarkBridgeMinted(r.ID, tx.Hash().Hex(), "")
	return nil
}

func handleBurnEvents(client *ethclient.Client, parsedABI abi.ABI, bridge common.Address, fromBlock, toBlock uint64, bc *Blockchain_struct) error {
	ctx := context.Background()
	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(int64(fromBlock)),
		ToBlock:   big.NewInt(int64(toBlock)),
		Addresses: []common.Address{bridge},
		Topics:    [][]common.Hash{{parsedABI.Events["Burn"].ID}},
	}
	logs, err := client.FilterLogs(ctx, query)
	if err != nil {
		return err
	}
	if len(logs) == 0 {
		log.Printf("Bridge relayer: no lock events for %s in %d -> %d", bridge.Hex(), fromBlock, toBlock)
		if os.Getenv("BRIDGE_DEBUG_LOCK") == "1" {
			debugQuery := ethereum.FilterQuery{
				FromBlock: big.NewInt(int64(fromBlock)),
				ToBlock:   big.NewInt(int64(toBlock)),
				Addresses: []common.Address{bridge},
			}
			debugLogs, derr := client.FilterLogs(ctx, debugQuery)
			if derr != nil {
				log.Printf("Bridge relayer: debug lock logs error: %v", derr)
			} else {
				log.Printf("Bridge relayer: debug lock logs count=%d", len(debugLogs))
				max := 5
				if len(debugLogs) < max {
					max = len(debugLogs)
				}
				for i := 0; i < max; i++ {
					if len(debugLogs[i].Topics) > 0 {
						log.Printf("Bridge relayer: debug lock topic0=%s tx=%s", debugLogs[i].Topics[0].Hex(), debugLogs[i].TxHash.Hex())
					}
				}
			}
		}
	}
	if len(logs) == 0 {
		log.Printf("Bridge relayer: no burn events for %s in %d -> %d", bridge.Hex(), fromBlock, toBlock)
	}
	if len(logs) == 0 {
		log.Printf("Bridge relayer: no burn events for %s in %d -> %d", bridge.Hex(), fromBlock, toBlock)
	}

	for _, vLog := range logs {
		var event struct {
			From   common.Address
			Amount *big.Int
			ID     [32]byte
			ToLqd  string
		}
		if err := parsedABI.UnpackIntoInterface(&event, "Burn", vLog.Data); err != nil {
			continue
		}
		toLqd := event.ToLqd
		if !ValidateAddress(toLqd) {
			continue
		}
		amount := new(big.Int).Set(event.Amount)
		if amount.Sign() <= 0 {
			continue
		}
		// create system unlock tx on LQD
		tx := bc.NewSystemTx("bridge_unlock", constantset.BridgeEscrowAddress, toLqd, amount)
		tx.ExtraData = []byte("bsc_burn:" + hex.EncodeToString(vLog.TxHash.Bytes()))
		bc.AddNewTxToTheTransaction_pool(tx)
	}
	return nil
}

func handleBscTokenLocks(client *ethclient.Client, parsedABI abi.ABI, erc20ABI abi.ABI, lock common.Address, fromBlock, toBlock uint64, bc *Blockchain_struct) error {
	ctx := context.Background()
	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(int64(fromBlock)),
		ToBlock:   big.NewInt(int64(toBlock)),
		Addresses: []common.Address{lock},
		Topics:    [][]common.Hash{{parsedABI.Events["Locked"].ID}},
	}
	logs, err := client.FilterLogs(ctx, query)
	if err != nil {
		return err
	}

	for _, vLog := range logs {
		var event struct {
			Amount *big.Int
			ID     [32]byte
			ToLqd  string
		}
		if err := parsedABI.UnpackIntoInterface(&event, "Locked", vLog.Data); err != nil {
			continue
		}
		if len(vLog.Topics) < 3 {
			continue
		}
		token := common.HexToAddress(vLog.Topics[1].Hex())
		from := common.HexToAddress(vLog.Topics[2].Hex())
		toLqd := event.ToLqd
		if !ValidateAddress(toLqd) {
			continue
		}
		amount := new(big.Int).Set(event.Amount)
		if amount.Sign() <= 0 {
			continue
		}

		bscTx := vLog.TxHash.Hex()
		bc.AddBridgeRequestBSC(bscTx, token.Hex(), from.Hex(), toLqd, amount)

		info := bc.GetBridgeTokenMapping(token.Hex())
		if info == nil {
			meta, err := fetchBscTokenMeta(client, erc20ABI, token)
			if err != nil {
				log.Printf("Bridge relayer: failed to fetch token meta: %v", err)
				continue
			}
			lqdAddr, err := deployBridgeToken(bc, meta.Name, meta.Symbol, meta.Decimals, token.Hex())
			if err != nil {
				log.Printf("Bridge relayer: deploy token failed: %v", err)
				continue
			}
			info = &BridgeTokenInfo{
				BscToken: token.Hex(),
				LqdToken: lqdAddr,
				Name:     meta.Name,
				Symbol:   meta.Symbol,
				Decimals: meta.Decimals,
			}
			bc.SetBridgeTokenMapping(token.Hex(), info)
		}

		// Mint on LQD via system contract call
		lqdTx := bc.NewSystemTx("contract_call", constantset.BridgeEscrowAddress, info.LqdToken, NewAmountFromUint64(0))
		lqdTx.Function = "Mint"
		lqdTx.Args = []string{toLqd, event.Amount.String()}
		lqdTx.IsContract = true
		lqdTx.Gas = uint64(constantset.ContractCallGas)
		lqdTx.TxHash = CalculateTransactionHash(*lqdTx)
		bc.AddNewTxToTheTransaction_pool(lqdTx)
		bc.RecordRecentTx(lqdTx)
		bc.MarkBridgeMinted(bscTx, "", lqdTx.TxHash)
	}
	return nil
}

type tokenMeta struct {
	Name     string
	Symbol   string
	Decimals string
}

func fetchBscTokenMeta(client *ethclient.Client, parsedABI abi.ABI, token common.Address) (*tokenMeta, error) {
	contract := bind.NewBoundContract(token, parsedABI, client, client, client)
	var name string
	if err := callString(contract, "name", &name); err != nil {
		name = "Token"
	}
	var symbol string
	if err := callString(contract, "symbol", &symbol); err != nil {
		symbol = "TOKEN"
	}
	var decimals uint8
	_ = callUint8(contract, "decimals", &decimals)
	return &tokenMeta{
		Name:     name,
		Symbol:   symbol,
		Decimals: strconv.Itoa(int(decimals)),
	}, nil
}

func callString(contract *bind.BoundContract, method string, out *string) error {
	var res []interface{}
	if err := contract.Call(&bind.CallOpts{Context: context.Background()}, &res, method); err != nil {
		return err
	}
	if len(res) == 0 {
		return fmt.Errorf("empty result")
	}
	if v, ok := res[0].(string); ok {
		*out = v
		return nil
	}
	if b, ok := res[0].([]byte); ok {
		*out = string(b)
		return nil
	}
	return fmt.Errorf("unexpected type")
}

func callUint8(contract *bind.BoundContract, method string, out *uint8) error {
	var res []interface{}
	if err := contract.Call(&bind.CallOpts{Context: context.Background()}, &res, method); err != nil {
		return err
	}
	if len(res) == 0 {
		return fmt.Errorf("empty result")
	}
	switch v := res[0].(type) {
	case uint8:
		*out = v
	case *big.Int:
		*out = uint8(v.Uint64())
	default:
		return fmt.Errorf("unexpected type")
	}
	return nil
}

func deployBridgeToken(bc *Blockchain_struct, name, symbol, decimals, bscToken string) (string, error) {
	return bc.DeployBridgeToken(name, symbol, decimals, bscToken)
}

func sendRelease(client *ethclient.Client, parsedABI abi.ABI, lock common.Address, privKeyHex string, chainID int64, r *BridgeRequest) error {
	key, err := crypto.HexToECDSA(strings.TrimPrefix(privKeyHex, "0x"))
	if err != nil {
		return err
	}
	auth, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(chainID))
	if err != nil {
		return err
	}
	auth.Context = context.Background()

	id := common.HexToHash(r.LqdTxHash)
	token := common.HexToAddress(r.Token)
	to := common.HexToAddress(r.To)
	amount, err := NewAmountFromString(r.Amount)
	if err != nil {
		return err
	}

	contract := bind.NewBoundContract(lock, parsedABI, client, client, client)
	tx, err := contract.Transact(auth, "release", token, to, amount, id)
	if err != nil {
		return err
	}
	log.Printf("Bridge relayer: release tx sent %s for %s", tx.Hash().Hex(), r.ID)
	r.BscTxHash = tx.Hash().Hex()
	return nil
}

func mintFromBscRequest(client *ethclient.Client, erc20ABI abi.ABI, r *BridgeRequest, bc *Blockchain_struct) error {
	tokenAddr := common.HexToAddress(r.Token)
	info := bc.GetBridgeTokenMapping(tokenAddr.Hex())
	if info == nil {
		meta, err := fetchBscTokenMeta(client, erc20ABI, tokenAddr)
		if err != nil {
			return err
		}
		lqdAddr, err := deployBridgeToken(bc, meta.Name, meta.Symbol, meta.Decimals, tokenAddr.Hex())
		if err != nil {
			return err
		}
		info = &BridgeTokenInfo{
			BscToken: tokenAddr.Hex(),
			LqdToken: lqdAddr,
			Name:     meta.Name,
			Symbol:   meta.Symbol,
			Decimals: meta.Decimals,
		}
		bc.SetBridgeTokenMapping(tokenAddr.Hex(), info)
	}

	// Execute mint immediately to guarantee balance update.
	if bc.ContractEngine == nil || bc.ContractEngine.Pipeline == nil {
		return fmt.Errorf("contract engine not initialized")
	}
	_, err := bc.ContractEngine.Pipeline.Execute(
		info.LqdToken,
		constantset.BridgeEscrowAddress,
		"Mint",
		[]string{r.To, r.Amount},
		5_000_000,
	)
	if err != nil {
		return err
	}

	// Record a synthetic system tx for UI/history.
	lqdTx := bc.NewSystemTx("contract_call", constantset.BridgeEscrowAddress, info.LqdToken, NewAmountFromUint64(0))
	lqdTx.Function = "Mint"
	lqdTx.Args = []string{r.To, r.Amount}
	lqdTx.IsContract = true
	lqdTx.Gas = uint64(constantset.ContractCallGas)
	lqdTx.Status = constantset.StatusSuccess
	lqdTx.TxHash = CalculateTransactionHash(*lqdTx)
	bc.RecordRecentTx(lqdTx)

	bc.MarkBridgeMinted(r.ID, r.BscTxHash, lqdTx.TxHash)
	return nil
}
