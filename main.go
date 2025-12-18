package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"

	"time"

	blockchaincomponent "github.com/Zotish/DefenceProject/BlockchainComponent"
	blockchainserver "github.com/Zotish/DefenceProject/BlockchainServer"
	walletserver "github.com/Zotish/DefenceProject/WalletServer"
)

func init() {
	log.SetPrefix("Blockchain: ")
}
func main() {
	chainCmdSet := flag.NewFlagSet("chain", flag.ExitOnError)
	walletCmdSet := flag.NewFlagSet("wallet", flag.ExitOnError)

	chainPort := chainCmdSet.Uint("port", 5000, "HTTP port to launch our blockchain server")
	validatorAddress := chainCmdSet.String("validator", "", "Validator address to receive staking rewards")
	remoteNode := chainCmdSet.String("remote_node", "", "Remote Node from where the blockchain will be synced")
	minStake := chainCmdSet.Float64("min_stake", 100000, "Minimum stake amount to become a validator")
	stakeAmount := chainCmdSet.Float64("stake_amount", 2000000, "Amount being staked by the validator")

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
			genesisBlock := blockchaincomponent.NewBlock(0, "0x_Genesis")
			bc := blockchaincomponent.NewBlockchain(genesisBlock)
			bc.InitLiquiditySystem()
			bc.MinStake = *minStake

			bcs := blockchainserver.NewBlockchainServer(uint(*chainPort), bc)
			go bcs.Start()

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

			err := bc.AddNewValidators(*validatorAddress, *stakeAmount, time.Hour*24*30)
			if err != nil {
				log.Fatalf("Failed to add validator: %v", err)
			}

			for _, v := range bc.Validators {
				bc.Network.BroadcastValidator(v)
			}

			for {
				bc.CleanStaleTransactions()

				if len(bc.Blocks)%100 == 0 {
					bc.TrimInMemoryBlocks(100)
				}

				if len(bc.Blocks)%10 == 0 {
					bc.CleanTransactionPool()
				}

				bc.UpdateMinStake(float64(len(bc.Transaction_pool)))

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

				if err := bc.Network.SyncChain(); err != nil {
					log.Printf("Sync error: %v", err)
				}

				interval := 1 * time.Second
				if len(bc.Transaction_pool) > 100 {
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

	default:
		fmt.Println("Expected 'chain' or 'wallet' subcommands")
		os.Exit(1)
	}
}
