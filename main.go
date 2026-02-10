package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"

	"time"

	aggregatorserver "github.com/Zotish/DefenceProject/AggregatorServer"
	blockchaincomponent "github.com/Zotish/DefenceProject/BlockchainComponent"
	blockchainserver "github.com/Zotish/DefenceProject/BlockchainServer"
	constantset "github.com/Zotish/DefenceProject/ConstantSet"
	walletserver "github.com/Zotish/DefenceProject/WalletServer"
)

func init() {
	log.SetPrefix("Blockchain: ")
}
func main() {
	loadEnvFile(".env")
	chainCmdSet := flag.NewFlagSet("chain", flag.ExitOnError)
	walletCmdSet := flag.NewFlagSet("wallet", flag.ExitOnError)

	chainPort := chainCmdSet.Uint("port", 5000, "HTTP port to launch our blockchain server")
	p2pPort := chainCmdSet.Uint("p2p_port", 0, "P2P TCP port for validator sync (default: port+1000)")
	validatorAddress := chainCmdSet.String("validator", "", "Validator address to receive staking rewards")
	remoteNode := chainCmdSet.String("remote_node", "", "Remote P2P node (host:port) to sync from")
	minStake := chainCmdSet.Float64("min_stake", 100000, "Minimum stake amount to become a validator")
	stakeAmount := chainCmdSet.Float64("stake_amount", 2000000, "Amount being staked by the validator")
	miningEnabled := chainCmdSet.Bool("mining", true, "Enable mining on this node")
	dbPath := chainCmdSet.String("db_path", "", "Path to LevelDB for this node")

	walletPort := walletCmdSet.Uint("port", 8080, "HTTP port to launch our wallet server")
	blockchainNodeAddress := walletCmdSet.String("node_address", "http://127.0.0.1:5000", "Blockchain node address for the wallet gateway")

	if len(os.Args) < 2 {
		fmt.Println("Usage:")
		fmt.Println("  chain -port PORT -validator ADDRESS -stake_amount AMOUNT [-remote_node URL] [-min_stake AMOUNT]")
		fmt.Println("  wallet -port PORT -node_address URL")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "chain":
		chainCmdSet.Parse(os.Args[2:])

		if chainCmdSet.Parsed() {
			if *validatorAddress == "" || *stakeAmount <= *minStake {
				fmt.Println("Error: Validator address and stake amount (> min_stake) are required")
				chainCmdSet.PrintDefaults()
				os.Exit(1)
			}
			if !strings.HasPrefix(*validatorAddress, "0x") || len(*validatorAddress) != 42 {
				log.Fatal("Validator address must be a valid Ethereum-style address (0x...)")
			}
			if *dbPath != "" {
				constantset.BLOCKCHAIN_DB_PATH = *dbPath
			}
			genesisBlock := blockchaincomponent.NewBlock(0, "0x_Genesis")
			bc := blockchaincomponent.NewBlockchain(genesisBlock)
			bc.InitLiquiditySystem()
			bc.MinStake = *minStake

			if *p2pPort == 0 {
				*p2pPort = *chainPort + 1000
			}
			bc.Network.HTTPPort = int(*chainPort)
			if err := bc.Network.Start(strconv.FormatUint(uint64(*p2pPort), 10)); err != nil {
				log.Fatalf("Failed to start P2P network: %v", err)
			}

			bcs := blockchainserver.NewBlockchainServer(uint(*chainPort), bc)
			go bcs.Start()
			blockchaincomponent.StartBridgeRelayer(bc)

			if *remoteNode != "" {
				host, portStr, err := net.SplitHostPort(*remoteNode)
				if err != nil {
					log.Fatalf("Invalid remote node address: %v", err)
				}
				if host == "localhost" {
					host = "127.0.0.1"
				}
				port, err := strconv.Atoi(portStr)
				if err != nil {
					log.Fatalf("Invalid remote node port: %v", err)
				}
				bc.Network.AddPeer(host, port, true)
			}

			if err := bc.Network.SyncChain(); err != nil {
				log.Printf("Initial sync error: %v", err)
			}

			err := bc.AddNewValidators(*validatorAddress, *stakeAmount, time.Hour*24*30)
			if err != nil {
				log.Fatalf("Failed to add validator: %v", err)
			}
			bc.LocalValidator = *validatorAddress

			for _, v := range bc.Validators {
				bc.Network.BroadcastValidator(v)
			}

			lastValidatorsSync := time.Time{}
			for {
				bc.CleanStaleTransactions()

				if len(bc.Blocks)%100 == 0 {
					bc.TrimInMemoryBlocks(100)
				}

				if len(bc.Blocks)%10 == 0 {
					bc.CleanTransactionPool()
				}

				bc.UpdateMinStake(float64(len(bc.Transaction_pool)))

				if err := bc.Network.SyncChain(); err != nil {
					log.Printf("Sync error: %v", err)
					time.Sleep(1 * time.Second)
					continue
				}
				if time.Since(lastValidatorsSync) > 5*time.Second {
					bc.Network.SyncAllValidators()
					lastValidatorsSync = time.Now()
				}

				if *miningEnabled {
					validator, err := bc.SelectValidator()
					if err != nil {
						log.Printf("Validator selection error: %v", err)
						time.Sleep(0 * time.Second)
						continue
					}

					newBlock := bc.MineNewBlock()
					if newBlock != nil {
						log.Printf("Mined block #%d", newBlock.BlockNumber)

						if err := bc.Network.BroadcastBlock(newBlock); err != nil {
							log.Printf("Failed to broadcast block: %v", err)
						}
					}

					bc.ProcessUnstakeReleases()

					log.Printf("Selected validator: %s", validator.Address)

					bc.MonitorValidators()
				}

				interval := 1 * time.Second
				if len(bc.Transaction_pool) > 200 {
					interval = 2 * time.Second
				}
				time.Sleep(interval)
			}
		}

	case "wallet":
		walletCmdSet.Parse(os.Args[2:])
		if walletCmdSet.Parsed() {
			ws := walletserver.NewWalletServer(uint64(*walletPort), *blockchainNodeAddress)
			ws.Start()
		}

	case "aggregate":
		aggCmdSet := flag.NewFlagSet("aggregate", flag.ExitOnError)
		aggPort := aggCmdSet.Uint("port", 9000, "HTTP port to launch aggregator server")
		aggNodes := aggCmdSet.String("nodes", "auto", "Comma-separated node list or 'auto' to discover from canonical")
		aggCanonical := aggCmdSet.String("canonical", "http://127.0.0.1:5000", "Canonical node base URL")
		aggWallet := aggCmdSet.String("wallet", "http://127.0.0.1:8080", "Wallet server base URL")
		aggCmdSet.Parse(os.Args[2:])
		if aggCmdSet.Parsed() {
			var nodes []string
			if strings.TrimSpace(strings.ToLower(*aggNodes)) != "auto" && strings.TrimSpace(*aggNodes) != "" {
				for _, n := range strings.Split(*aggNodes, ",") {
					n = strings.TrimSpace(n)
					if n == "" {
						continue
					}
					nodes = append(nodes, n)
				}
			}
			as := aggregatorserver.NewAggregatorServer(uint(*aggPort), nodes, *aggCanonical, *aggWallet)
			as.Start()
		}

	default:
		fmt.Println("Expected 'chain' or 'wallet' subcommands")
		os.Exit(1)
	}
}

// loadEnvFile reads simple export-based .env files and sets env vars if not already set.
func loadEnvFile(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
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
		} else {
			continue
		}
		if strings.HasPrefix(line, "go ") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		val = strings.Trim(val, `"'`)
		if key == "" {
			continue
		}
		_ = os.Setenv(key, val)
	}
}
