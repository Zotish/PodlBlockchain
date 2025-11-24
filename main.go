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

	// Chain command flags
	chainPort := chainCmdSet.Uint("port", 5000, "HTTP port to launch our blockchain server")
	validatorAddress := chainCmdSet.String("validator", "", "Validator address to receive staking rewards")
	remoteNode := chainCmdSet.String("remote_node", "", "Remote Node from where the blockchain will be synced")
	minStake := chainCmdSet.Float64("min_stake", 100000, "Minimum stake amount to become a validator")
	stakeAmount := chainCmdSet.Float64("stake_amount", 2000000, "Amount being staked by the validator")

	// Wallet command flags
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
			bc.MinStake = *minStake

			// Start blockchain server
			bcs := blockchainserver.NewBlockchainServer(uint(*chainPort), bc)
			go bcs.Start()

			// Start network service
			// if *remoteNode != "" {
			// 	// Parse remote node address and port
			// 	host, portStr, err := net.SplitHostPort(*remoteNode)
			// 	if err != nil {
			// 		log.Fatalf("Invalid remote node address: %v", err)
			// 	}
			// 	port, err := strconv.Atoi(portStr)
			// 	if err != nil {
			// 		log.Fatalf("Invalid remote node port: %v", err)
			// 	}
			// 	bc.Network.AddPeer(host, port, true)

			// }

			if *remoteNode != "" {
				host, portStr, err := net.SplitHostPort(*remoteNode)
				if err != nil {
					log.Fatalf("Invalid remote node address: %v", err)
				}
				// normalize “localhost” -> 127.0.0.1 to avoid IPv6 ::1 issues
				if host == "localhost" {
					host = "127.0.0.1"
				}
				port, err := strconv.Atoi(portStr)
				if err != nil {
					log.Fatalf("Invalid remote node port: %v", err)
				}
				bc.Network.AddPeer(host, port, true)
			}

			// Add the validator
			err := bc.AddNewValidators(*validatorAddress, *stakeAmount, time.Hour*24*30)
			if err != nil {
				log.Fatalf("Failed to add validator: %v", err)
			}

			for _, v := range bc.Validators {
				bc.Network.BroadcastValidator(v)
			}

			// Start mining loop
			for {
				bc.CleanStaleTransactions()

				// Trim memory every 100 blocks
				if len(bc.Blocks)%100 == 0 {
					bc.TrimInMemoryBlocks(100) // Keep only last 100 blocks in memory
				}

				// Clean pool every 10 blocks
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

					// Broadcast the new block
					if err := bc.Network.BroadcastBlock(newBlock); err != nil {
						log.Printf("Failed to broadcast block: %v", err)
					}
				}
				log.Printf("Selected validator: %s", validator.Address)

				// Monitor validators
				bc.MonitorValidators()

				// Sync with network periodically
				if err := bc.Network.SyncChain(); err != nil {
					log.Printf("Sync error: %v", err)
				}

				interval := 1 * time.Second
				if len(bc.Transaction_pool) > 100 {
					interval = 2 * time.Second
				}
				time.Sleep(interval)
			}
			// for {
			// 	bc.CleanStaleTransactions()
			// 	bc.UpdateMinStake(float64(len(bc.Transaction_pool)))

			// 	// Select validator
			// 	validator, err := bc.SelectValidator()
			// 	if err != nil {
			// 		log.Printf("Validator selection error: %v", err)
			// 		time.Sleep(2 * time.Second)
			// 		continue
			// 	}
			// 	log.Printf("Selected validator: %s", validator.Address)

			// 	// Mine new block
			// 	newBlock := bc.MineNewBlock()
			// 	if newBlock != nil {
			// 		log.Printf("Mined block #%d", newBlock.BlockNumber)

			// 		// Broadcast the new block
			// 		if err := bc.Network.BroadcastBlock(newBlock); err != nil {
			// 			log.Printf("Failed to broadcast block: %v", err)
			// 		}
			// 	}

			// 	// Monitor validators
			// 	bc.MonitorValidators()

			// 	// Sync with network periodically
			// 	if err := bc.Network.SyncChain(); err != nil {
			// 		log.Printf("Sync error: %v", err)
			// 	}

			// 	interval := 6 * time.Second
			// 	if len(bc.Transaction_pool) > 100 {
			// 		interval = 4 * time.Second // Faster blocks when busy
			// 	}
			// 	time.Sleep(interval)
			// }
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

// func main() {
// 	chainCmdSet := flag.NewFlagSet("chain", flag.ExitOnError)
// 	walletCmdSet := flag.NewFlagSet("wallet", flag.ExitOnError)

// 	// Chain command flags
// 	chainPort := chainCmdSet.Uint("port", 5000, "HTTP port to launch our blockchain server")
// 	validatorAddress := chainCmdSet.String("validator", "", "Validator address to receive staking rewards")
// 	remoteNode := chainCmdSet.String("remote_node", "", "Remote Node from where the blockchain will be synced")
// 	minStake := chainCmdSet.Float64("min_stake", 100000, "Minimum stake amount to become a validator")
// 	stakeAmount := chainCmdSet.Float64("stake_amount", 2000000, "Amount being staked by the validator")

// 	// Wallet command flags
// 	walletPort := walletCmdSet.Uint("port", 8080, "HTTP port to launch our wallet server")
// 	blockchainNodeAddress := walletCmdSet.String("node_address", "http://127.0.0.1:5000", "Blockchain node address for the wallet gateway")

// 	if len(os.Args) < 2 {
// 		fmt.Println("Usage:")
// 		fmt.Println("  chain -port PORT -validator ADDRESS -stake_amount AMOUNT [-remote_node URL] [-min_stake AMOUNT]")
// 		fmt.Println("  wallet -port PORT -node_address URL")
// 		os.Exit(1)
// 	}

// 	switch os.Args[1] {
// 	case "chain":
// 		var wg sync.WaitGroup

// 		chainCmdSet.Parse(os.Args[2:])
// 		if chainCmdSet.Parsed() {
// 			if *validatorAddress == "" || *stakeAmount <= *minStake {
// 				fmt.Println("Error: Validator address and stake amount (> min_stake) are required")
// 				chainCmdSet.PrintDefaults()
// 				os.Exit(1)
// 			}

// 			if *remoteNode == "" {
// 				genesisBlock := blockchain.NewBlock(0, "0x_Genesis")
// 				bc := blockchain.NewBlockchain(genesisBlock)
// 				bc.MinStake = *minStake

// 				// Only add local peer for testing
// 				bc.Network.AddPeer("localhost", 5001, true)

// 				// Add the validator with dynamic stake
// 				// Replace the validator addition with:
// 				if *stakeAmount < *minStake { // Explicit check before adding validator
// 					log.Fatalf("Stake amount (%f) must be >= min_stake (%f)", *stakeAmount, *minStake)
// 				}
// 				err := bc.AddNewValidators(*validatorAddress, *stakeAmount, time.Hour*24*30)
// 				if err != nil {
// 					log.Fatalf("Failed to add validator: %v", err)
// 				}

// 				// Update liquidity power based on current stake
// 				bc.UpdateLiquidityPower()

// 				bcs := blockserver.NewBlockchainServer(uint(*chainPort), bc)

// 				wg.Add(3)
// 				go bcs.Start()
// 				go bc.MonitorValidators()
// 				go bc.Network.SyncChain()
// 				wg.Wait()
// 			} else {
// 				log.Println("Syncing with remote node is not yet implemented")
// 				os.Exit(1)
// 			}
// 		}

// 	case "wallet":
// 		walletCmdSet.Parse(os.Args[2:])
// 		if walletCmdSet.Parsed() {
// 			if walletCmdSet.NFlag() == 0 {
// 				walletCmdSet.PrintDefaults()
// 				os.Exit(1)
// 			}

// 			ws := walletserver.NewWalletServer(uint64(*walletPort), *blockchainNodeAddress)
// 			ws.Start()
// 		}

// 	default:
// 		fmt.Println("Expected 'chain' or 'wallet' subcommands")
// 		os.Exit(1)
// 	}
// }

// package main

// import (
// 	"fmt"
// 	"log"
// 	blockchain "project/Blockchain"
// 	wallet "project/Wallet"
// 	"time"
// )

// func main() {

// 	wallet1, err := wallet.NewWallet("iLIKEYOU@1112")
// 	if err != nil {
// 		log.Println("error occure")
// 	}
// 	log.Println("Address\n", wallet1.Address)
// 	log.Println("PrivateKey\n", wallet1.GetPrivateKeyHex())
// 	log.Println("25 word backup phrase \n", wallet1.Mnemonic)
// 	wallet2, err := wallet.NewWallet("iLIKEYOU@1112")
// 	if err != nil {
// 		log.Println("error occure")
// 	}
// 	log.Println("Address\n", wallet2.Address)
// 	log.Println("PrivateKey\n", wallet2.GetPrivateKeyHex())
// 	log.Println("25 word backup phrase \n", wallet2.Mnemonic)

// 	// restoredFromMnemonic, _ := wallet.ImportFromMnemonic(wallet.Mnemonic, "iLIKEYOU@1112")
// 	// fmt.Println("From Mnemonic:", restoredFromMnemonic.Address)

// 	// //Restore from private key
// 	// restoredFromPK, _ := wallet.ImportFromPrivateKey(wallet.GetPrivateKeyHex())
// 	//fmt.Println("From Private Key:", restoredFromPK.Address)
// 	genesisBlock := blockchain.NewBlock(0, "0x_Genesis")
// 	bc := blockchain.NewBlockchain(genesisBlock)

// 	err = bc.AddNewValidators("validator1", 1000000.0, time.Hour*24*1) // $1.5M locked for 1 year
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	err = bc.AddNewValidators("validator2", 2000000.0, time.Hour*24*2) // $2M locked for 6 months
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	err = bc.AddNewValidators("validator3", 3000000.0, time.Hour*24*3) // $2M locked for 6 months
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	// log.Printf("Mined block #%d\n", genesisBlock.BlockNumber)
// 	// log.Println("this is the inside Gen Prevhash", genesisBlock.PreviousHash)
// 	// log.Println("this is the inside Gen Currenthash", genesisBlock.CurrentHash)
// 	// Get current nonce for account

// 	// set initial balances for the two wallets
// 	bc.Accounts[wallet1.Address] = 1000000 // Starting balance
// 	bc.Accounts[wallet2.Address] = 1000000 // Starting balance

// 	for {
// 		bc.UpdateMinStake(float64(len(bc.Transaction_pool)))
// 		validator, err := bc.SelectValidator()
// 		if err != nil {
// 			log.Println("Validator selection error:", err)
// 			time.Sleep(2 * time.Second)
// 			continue
// 		}

// 		log.Printf("Selected validator: %s (LP: %.2f)\n", validator.Address, validator.LiquidityPower)
// 		balance1 := bc.CheckBalance(wallet1.Address)
// 		balance2 := bc.CheckBalance(wallet2.Address)
// 		nonce1 := bc.GetAccountNonce(wallet1.Address) + 1
// 		nonce2 := bc.GetAccountNonce(wallet2.Address) + 1
// 		log.Println("This is before tx creation")
// 		log.Println("first User : ", bc.CheckBalance(wallet1.Address))
// 		log.Println("2nd user : ", bc.CheckBalance(wallet1.Address))
// 		// 2. Create and add transactions
// 		txAmount1 := uint64(44) // Reduced from 222 to 50
// 		if txAmount1 > balance1 {
// 			log.Printf("Insufficient funds: 0x1 needs %d but has %d\n", txAmount1, balance1)
// 			continue
// 		}

// 		tx1 := blockchain.NewTransaction(wallet1.Address, wallet2.Address, txAmount1, []byte{}, nonce1)
// 		wallet1.SignTransaction(tx1)
// 		if !bc.VerifyTransaction(tx1) {
// 			log.Println("Transaction 1 verification failed:", tx1.Status)
// 			continue
// 		}

// 		if err := bc.AddNewTxToTheTransaction_pool(tx1); err != nil {
// 			log.Println("Failed to add tx1 to pool:", err)
// 			continue
// 		}
// 		log.Println("Transaction 1 added to pool with status:", tx1.Status)

// 		// Second transaction
// 		txAmount2 := uint64(1)
// 		if txAmount2 > balance2 {
// 			log.Printf("Insufficient funds: 0x2 needs %d but has %d\n", txAmount2, balance2)
// 			continue
// 		}

// 		tx2 := blockchain.NewTransaction(wallet2.Address, wallet1.Address, txAmount2, []byte{}, nonce2)
// 		wallet2.SignTransaction(tx2)

// 		if !bc.VerifyTransaction(tx2) {
// 			log.Println("Transaction 2 verification failed:", tx2.Status)
// 			continue
// 		}

// 		if err := bc.AddNewTxToTheTransaction_pool(tx2); err != nil {
// 			log.Println("Failed to add tx2 to pool:", err)
// 			continue
// 		}
// 		log.Println("Transaction 2 added to pool with status:", tx2.Status)
// 		newBlock := bc.MineNewBlock()
// 		for _, tx := range newBlock.Transactions {
// 			log.Printf("Transaction %s final status: %s (in block %d)\n",
// 				tx.TxHash, tx.Status, newBlock.BlockNumber)
// 		}
// 		if bc.VerifyBlock(bc) {
// 			fmt.Println("✅ Block  verification success\n", newBlock.BlockNumber)

// 		} else {
// 			log.Println("here is block validation is failed ", newBlock.BlockNumber)
// 			log.Println("❌ Block verification failed")
// 		}
// 		log.Println("This is after tx creation")

// 		log.Println("first User : ", bc.CheckBalance(wallet1.Address))
// 		log.Println("2nd user : ", bc.CheckBalance(wallet2.Address))
// 		fmt.Println("Current chain state:")
// 		//fmt.Println(bc.ToJsonChain())

// 		time.Sleep(10 * time.Second)
// 		//log.Println("All Blocks", bc.Blocks)
// 	}
// }

// func init() {
// 	log.SetPrefix("Blockchain ")
// }
