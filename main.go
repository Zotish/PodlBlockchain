package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"time"

	aggregatorserver "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/AggregatorServer"
	blockchaincomponent "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/BlockchainComponent"
	blockchainserver "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/BlockchainServer"
	constantset "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/ConstantSet"
	walletserver "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/WalletServer"
)

func init() {
	log.SetPrefix("Blockchain: ")
}

const canonicalGenesisTimestamp = uint64(1700000000)

const defaultBlockProductionInterval = 2 * time.Second

func configuredBlockProductionInterval(bc *blockchaincomponent.Blockchain_struct) time.Duration {
	if bc == nil || bc.ChainSpec.BlockTimeMS == 0 {
		return defaultBlockProductionInterval
	}
	return time.Duration(bc.ChainSpec.BlockTimeMS) * time.Millisecond
}

// remainingBlockProductionDelay keeps the configured interval measured from
// the start of a production attempt. Sleeping a fixed interval after mining
// makes the actual block interval equal to work time + target time and causes
// block time to grow as state execution becomes more expensive.
func remainingBlockProductionDelay(target, elapsed time.Duration) time.Duration {
	if target <= 0 {
		target = defaultBlockProductionInterval
	}
	if elapsed >= target {
		return 0
	}
	return target - elapsed
}

func main() {
	loadEnvFile(".env")
	chainCmdSet := flag.NewFlagSet("chain", flag.ExitOnError)
	walletCmdSet := flag.NewFlagSet("wallet", flag.ExitOnError)
	signerCmdSet := flag.NewFlagSet("signer", flag.ExitOnError)

	chainPort := chainCmdSet.Uint("port", 5000, "HTTP port to launch our blockchain server")
	p2pPort := chainCmdSet.Uint("p2p_port", 0, "P2P TCP port for validator sync (default: port+1000)")
	validatorAddress := chainCmdSet.String("validator", "", "Validator address to receive staking rewards")
	validatorPrivateKey := chainCmdSet.String("validator_private_key", "", "Development-only local validator key; prefer remote signer or encrypted key file")
	validatorVRFPrivateKey := chainCmdSet.String("validator_vrf_private_key", os.Getenv("LQD_VALIDATOR_VRF_PRIVATE_KEY"), "P-256 VRF private scalar")
	validatorKeyFile := chainCmdSet.String("validator_key_file", os.Getenv("LQD_VALIDATOR_KEY_FILE"), "Encrypted local validator key file")
	validatorSignerURL := chainCmdSet.String("validator_signer_url", os.Getenv("LQD_VALIDATOR_SIGNER_URL"), "Remote mTLS validator signer URL")
	validatorSignerCA := chainCmdSet.String("validator_signer_ca", os.Getenv("LQD_VALIDATOR_SIGNER_CA"), "Remote signer CA certificate")
	validatorSignerCert := chainCmdSet.String("validator_signer_cert", os.Getenv("LQD_VALIDATOR_SIGNER_CERT"), "Remote signer client certificate")
	validatorSignerKey := chainCmdSet.String("validator_signer_key", os.Getenv("LQD_VALIDATOR_SIGNER_KEY"), "Remote signer client key")
	validatorSignerName := chainCmdSet.String("validator_signer_name", os.Getenv("LQD_VALIDATOR_SIGNER_NAME"), "Remote signer TLS server name")
	validatorSlashingDB := chainCmdSet.String("validator_slashing_db", os.Getenv("LQD_VALIDATOR_SLASHING_DB"), "Durable anti-double-sign database")
	remoteNode := chainCmdSet.String("remote_node", "", "Remote P2P node (host:port) to sync from")
	minStake := chainCmdSet.Float64("min_stake", 100000, "Minimum stake amount to become a validator")
	stakeAmount := chainCmdSet.Float64("stake_amount", 2000000, "Amount being staked by the validator (legacy PoS)")
	miningEnabled := chainCmdSet.Bool("mining", true, "Enable mining on this node")
	requireSignedBFT := chainCmdSet.Bool("require_signed_bft", strings.EqualFold(strings.TrimSpace(os.Getenv("LQD_REQUIRE_SIGNED_BFT")), "true"), "Reject legacy unsigned finality")
	dbPath := chainCmdSet.String("db_path", "", "Path to LevelDB for this node")
	// True PosDL: register via DEX LP position instead of single-asset stake.
	// When -dex_address is provided the node uses AddDEXValidator; otherwise
	// it falls back to the legacy AddNewValidators (single-asset PoS).
	dexAddress := chainCmdSet.String("dex_address", "", "DEX contract address for PosDL LP-based validation")
	lpTokenAmount := chainCmdSet.String("lp_token_amount", "", "LP token amount (decimal) to lock for PosDL validation")

	walletPort := walletCmdSet.Uint("port", 8080, "HTTP port to launch our wallet server")
	blockchainNodeAddress := walletCmdSet.String("node_address", "http://127.0.0.1:5000", "Blockchain node address for the wallet gateway")

	signerListen := signerCmdSet.String("listen", "127.0.0.1:9100", "Remote signer HTTPS listen address")
	signerTLSCert := signerCmdSet.String("tls_cert", os.Getenv("LQD_SIGNER_TLS_CERT"), "Signer TLS certificate")
	signerTLSKey := signerCmdSet.String("tls_key", os.Getenv("LQD_SIGNER_TLS_KEY"), "Signer TLS private key")
	signerClientCA := signerCmdSet.String("client_ca", os.Getenv("LQD_SIGNER_CLIENT_CA"), "Signer trusted client CA")
	signerLocalKeyFile := signerCmdSet.String("key_file", os.Getenv("LQD_VALIDATOR_KEY_FILE"), "Encrypted local validator key file")
	signerCreateKeyFile := signerCmdSet.String("create_key_file", "", "Create an encrypted validator key file and exit")
	signerLocalPrivateKey := signerCmdSet.String("validator_private_key", os.Getenv("LQD_VALIDATOR_PRIVATE_KEY"), "Development-only raw validator key")
	signerVRFPrivateKey := signerCmdSet.String("vrf_private_key", os.Getenv("LQD_VALIDATOR_VRF_PRIVATE_KEY"), "P-256 VRF private scalar")
	signerSlashingDB := signerCmdSet.String("slashing_db", os.Getenv("LQD_VALIDATOR_SLASHING_DB"), "Durable anti-double-sign database")
	pkcs11Module := signerCmdSet.String("pkcs11_module", os.Getenv("LQD_PKCS11_MODULE"), "PKCS#11 module path")
	pkcs11Token := signerCmdSet.String("pkcs11_token", os.Getenv("LQD_PKCS11_TOKEN_LABEL"), "PKCS#11 token label")
	pkcs11KeyLabel := signerCmdSet.String("pkcs11_key", os.Getenv("LQD_PKCS11_KEY_LABEL"), "PKCS#11 validator key label")
	pkcs11PublicKey := signerCmdSet.String("pkcs11_public_key", os.Getenv("LQD_PKCS11_PUBLIC_KEY"), "Pinned uncompressed secp256k1 public key")
	pkcs11Slot := signerCmdSet.String("pkcs11_slot", "", "Optional PKCS#11 slot ID, including slot 0")

	if len(os.Args) < 2 {
		fmt.Println("Usage:")
		fmt.Println("  chain -port PORT -validator ADDRESS -stake_amount AMOUNT [-remote_node URL] [-min_stake AMOUNT]")
		fmt.Println("  wallet -port PORT -node_address URL")
		fmt.Println("  signer -listen HOST:PORT -tls_cert FILE -tls_key FILE -client_ca FILE")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "chain":
		chainCmdSet.Parse(os.Args[2:])

		if chainCmdSet.Parsed() {
			posDLMode := strings.TrimSpace(*dexAddress) != "" && strings.TrimSpace(*lpTokenAmount) != ""
			if *validatorAddress == "" || (!posDLMode && *stakeAmount <= *minStake) {
				fmt.Println("Error: validator address is required; legacy mode also requires stake_amount > min_stake")
				chainCmdSet.PrintDefaults()
				os.Exit(1)
			}
			if !strings.HasPrefix(*validatorAddress, "0x") || len(*validatorAddress) != 42 {
				log.Fatal("Validator address must be a valid Ethereum-style address (0x...)")
			}
			if *dbPath != "" {
				constantset.BLOCKCHAIN_DB_PATH = *dbPath
			}
			genesisBlock := canonicalGenesisBlock()
			bc := blockchaincomponent.NewBlockchain(genesisBlock)
			if bc == nil {
				log.Fatal("Failed to initialize blockchain")
			}
			bc.EnsureRuntimeState()
			bc.ChainSpec.AllowLegacyFinality = !*requireSignedBFT
			bc.InitLiquiditySystem()
			bc.MinStake = *minStake
			bc.BaseMinStake = *minStake

			if *p2pPort == 0 {
				*p2pPort = *chainPort + 1000
			}
			bc.Network.HTTPPort = int(*chainPort)
			slashingPath := strings.TrimSpace(*validatorSlashingDB)
			if slashingPath == "" {
				base := strings.TrimSpace(*dbPath)
				if base == "" {
					base = filepath.Join("data", "chain")
				}
				slashingPath = filepath.Join(base, "validator-slashing-protection.json")
			}
			var validatorSigner blockchaincomponent.ValidatorSigner
			var signerErr error
			switch {
			case strings.TrimSpace(*validatorSignerURL) != "":
				validatorSigner, signerErr = blockchaincomponent.NewRemoteValidatorSigner(context.Background(), blockchaincomponent.RemoteValidatorSignerConfig{
					URL: *validatorSignerURL, CAFile: *validatorSignerCA, ClientCertificateFile: *validatorSignerCert,
					ClientKeyFile: *validatorSignerKey, ServerName: *validatorSignerName, Timeout: 5 * time.Second,
				})
			case strings.TrimSpace(*validatorKeyFile) != "":
				validatorSigner, signerErr = blockchaincomponent.LoadEncryptedLocalValidatorSigner(*validatorKeyFile, os.Getenv("LQD_VALIDATOR_KEY_PASSPHRASE"), slashingPath)
			case strings.TrimSpace(*validatorPrivateKey) != "":
				validatorSigner, signerErr = blockchaincomponent.NewLocalValidatorSigner(*validatorPrivateKey, *validatorVRFPrivateKey, slashingPath)
			}
			if signerErr != nil {
				log.Fatalf("Validator signer configuration failed: %v", signerErr)
			}
			if validatorSigner != nil {
				if err := bc.Network.SetValidatorSigner(*validatorAddress, validatorSigner); err != nil {
					log.Fatalf("Validator signer rejected: %v", err)
				}
			} else {
				bc.Network.SetValidatorIdentity(*validatorAddress, "")
				if *requireSignedBFT {
					log.Fatal("Signed BFT requires a remote, encrypted-file, or local development validator signer")
				}
			}
			if err := bc.Network.Start(strconv.FormatUint(uint64(*p2pPort), 10)); err != nil {
				log.Fatalf("Failed to start P2P network: %v", err)
			}

			bcs := blockchainserver.NewBlockchainServer(uint(*chainPort), bc)
			go bcs.Start()
			blockchaincomponent.StartBridgeRelayer(bc)

			// Start peer health check (ping every 30s, remove unresponsive peers)
			go bc.Network.StartHealthCheck()

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

			if *remoteNode != "" {
				for !bc.Network.HasHealthyRemotePeer() {
					if err := bc.Network.SyncChain(); err != nil {
						log.Printf("Initial sync error: %v", err)
					}
					bc.Network.SyncAllValidators()
					if bc.Network.HasHealthyRemotePeer() {
						break
					}
					log.Printf("Waiting for remote peer %s before validator registration", *remoteNode)
					time.Sleep(2 * time.Second)
				}
			} else {
				if err := bc.Network.SyncChain(); err != nil {
					log.Printf("Initial sync error: %v", err)
				}
			}

			// ── Validator Registration ──────────────────────────────────────────
			// PosDL mode: lock DEX LP tokens (multi-asset, true liquidity stake).
			// Legacy mode: stake single LQD amount (backward compatible).
			if *dexAddress != "" && *lpTokenAmount != "" {
				log.Printf("Registering PosDL validator via DEX LP: dex=%s lp=%s", *dexAddress, *lpTokenAmount)
				if err := bc.AddDEXValidator(*validatorAddress, *dexAddress, *lpTokenAmount, time.Hour*24*30); err != nil {
					log.Fatalf("Failed to add PosDL validator: %v", err)
				}
			} else {
				err := bc.AddNewValidators(*validatorAddress, *stakeAmount, time.Hour*24*30)
				if err != nil {
					log.Fatalf("Failed to add legacy validator: %v", err)
				}
			}
			bc.LocalValidator = *validatorAddress

			for _, v := range bc.Validators {
				bc.Network.BroadcastValidator(v)
			}
			bc.Network.SyncAllValidators()

			lastValidatorsSync := time.Time{}
			stalledFinalizeRounds := 0
			for {
				productionAttemptStarted := time.Now()
				bc.CleanStaleTransactions()

				// Startup/recovery hydrates a larger DB tail. Bound it immediately;
				// checking only exact multiples allowed 1,000+ blocks to remain in
				// memory and made every speculative replay progressively slower.
				if len(bc.Blocks) > 100 {
					bc.TrimInMemoryBlocks(100)
				}

				if len(bc.Blocks)%10 == 0 {
					bc.CleanTransactionPool()
				}

				if err := bc.Network.SyncChain(); err != nil {
					// Solo node: sync error is normal — don't block mining
					log.Printf("Sync error (solo node): %v", err)
				}
				if time.Since(lastValidatorsSync) > 5*time.Second {
					bc.Network.SyncAllValidators()
					lastValidatorsSync = time.Now()
				}

				if *miningEnabled {
					if *remoteNode != "" && !bc.Network.HasHealthyRemotePeer() {
						log.Printf("Remote peer unavailable, pausing mining until sync recovers")
						time.Sleep(2 * time.Second)
						continue
					}

					nextHeight := bc.LatestBlockNumber() + 1
					round := bc.CurrentConsensusRound(nextHeight)
					nowUnix := time.Now().Unix()
					if bc.ConsensusRoundTimedOut(nextHeight, nowUnix) {
						if tc, timeoutErr := bc.CastLocalConsensusTimeout(nextHeight, round); timeoutErr != nil {
							log.Printf("Consensus timeout vote unavailable: height=%d round=%d err=%v", nextHeight, round, timeoutErr)
						} else if tc != nil {
							log.Printf("Consensus timeout certificate ready: height=%d round=%d hash=%s", nextHeight, round, tc.Hash)
						}
					}
					if advancedRound, advanced := bc.AdvanceConsensusRound(nextHeight, nowUnix); advanced {
						round = advancedRound
						log.Printf("Consensus view change: height=%d round=%d", nextHeight, round)
					}
					validator, err := bc.SelectBlockProposer(nextHeight, round)
					if err != nil {
						log.Printf("Validator selection error: %v", err)
						time.Sleep(2 * time.Second)
						continue
					}

					if strings.EqualFold(validator.Address, bc.LocalValidator) {
						beforeHeight := bc.LatestBlockNumber()
						newBlock := bc.MineNewBlock()
						afterHeight := bc.LatestBlockNumber()
						if newBlock != nil {
							stalledFinalizeRounds = 0
							log.Printf("Mined block #%d", newBlock.BlockNumber)

							if err := bc.Network.BroadcastBlock(newBlock); err != nil {
								log.Printf("Failed to broadcast block: %v", err)
							}
						} else if afterHeight <= beforeHeight {
							stalledFinalizeRounds++
							if stalledFinalizeRounds >= 3 {
								recovered, err := bc.RecoverInMemoryTipFromDB(1024)
								if err != nil {
									log.Printf("Block finalization recovery failed: %v", err)
								} else {
									log.Printf("Block finalization recovery ran: recovered=%v tip=%d", recovered, bc.LatestBlockNumber())
								}
								stalledFinalizeRounds = 0
							}
						} else {
							stalledFinalizeRounds = 0
						}
					} else {
						stalledFinalizeRounds = 0
						log.Printf("Selected validator is remote: %s (local=%s) — waiting for peer block", validator.Address, bc.LocalValidator)
					}

					bc.ProcessUnstakeReleases()

					log.Printf("Selected validator: %s", validator.Address)

					bc.MonitorValidators()
				}

				// Target cadence is defined by ChainSpec. Account for time already
				// spent syncing, executing and finalizing instead of adding another
				// fixed sleep after that work.
				target := configuredBlockProductionInterval(bc)
				if delay := remainingBlockProductionDelay(target, time.Since(productionAttemptStarted)); delay > 0 {
					time.Sleep(delay)
				}
			}
		}

	case "signer":
		signerCmdSet.Parse(os.Args[2:])
		if signerCmdSet.Parsed() {
			if strings.TrimSpace(*signerCreateKeyFile) != "" {
				if strings.TrimSpace(*signerLocalPrivateKey) == "" {
					log.Fatal("-create_key_file requires -validator_private_key")
				}
				if err := blockchaincomponent.WriteEncryptedValidatorKeyFile(*signerCreateKeyFile, *signerLocalPrivateKey, *signerVRFPrivateKey, os.Getenv("LQD_VALIDATOR_KEY_PASSPHRASE")); err != nil {
					log.Fatalf("Encrypted validator key creation failed: %v", err)
				}
				log.Printf("Encrypted validator key written to %s", *signerCreateKeyFile)
				return
			}
			if strings.TrimSpace(*signerSlashingDB) == "" {
				log.Fatal("Remote signer requires -slashing_db for durable anti-double-sign protection")
			}
			var signer blockchaincomponent.ValidatorSigner
			var err error
			if strings.TrimSpace(*pkcs11Module) != "" {
				var slotID *uint
				if strings.TrimSpace(*pkcs11Slot) != "" {
					parsed, parseErr := strconv.ParseUint(strings.TrimSpace(*pkcs11Slot), 10, 32)
					if parseErr != nil {
						log.Fatalf("Invalid PKCS#11 slot ID: %v", parseErr)
					}
					value := uint(parsed)
					slotID = &value
				}
				signer, err = blockchaincomponent.NewPKCS11ValidatorSigner(blockchaincomponent.PKCS11ValidatorSignerConfig{
					ModulePath: *pkcs11Module, TokenLabel: *pkcs11Token, PIN: os.Getenv("LQD_PKCS11_PIN"),
					KeyLabel: *pkcs11KeyLabel, PublicKeyHex: *pkcs11PublicKey, SlotID: slotID,
				}, *signerVRFPrivateKey, *signerSlashingDB)
			} else if strings.TrimSpace(*signerLocalKeyFile) != "" {
				signer, err = blockchaincomponent.LoadEncryptedLocalValidatorSigner(*signerLocalKeyFile, os.Getenv("LQD_VALIDATOR_KEY_PASSPHRASE"), *signerSlashingDB)
			} else if strings.TrimSpace(*signerLocalPrivateKey) != "" {
				signer, err = blockchaincomponent.NewLocalValidatorSigner(*signerLocalPrivateKey, *signerVRFPrivateKey, *signerSlashingDB)
			} else {
				err = fmt.Errorf("configure PKCS#11, encrypted key file, or development local key")
			}
			if err != nil {
				log.Fatalf("Signer backend initialization failed: %v", err)
			}
			defer signer.Close()
			tlsConfig, err := blockchaincomponent.ValidatorSignerServerTLSConfig(blockchaincomponent.ValidatorSignerTLSFiles{
				CertificateFile: *signerTLSCert, KeyFile: *signerTLSKey, ClientCAFile: *signerClientCA,
			})
			if err != nil {
				log.Fatalf("Signer mTLS configuration failed: %v", err)
			}
			server := &http.Server{
				Addr:              *signerListen,
				Handler:           blockchaincomponent.NewValidatorSignerHandler(signer, true),
				TLSConfig:         tlsConfig,
				ReadHeaderTimeout: 5 * time.Second,
				ReadTimeout:       10 * time.Second,
				WriteTimeout:      10 * time.Second,
				IdleTimeout:       30 * time.Second,
			}
			log.Printf("Remote validator signer listening on %s address=%s backend=%s", *signerListen, signer.Address(), signer.Status(context.Background()).Backend)
			if err := server.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
				log.Fatalf("Remote signer stopped: %v", err)
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
		fmt.Println("Expected 'chain', 'signer', 'wallet', or 'aggregate' subcommands")
		os.Exit(1)
	}
}

func canonicalGenesisBlock() blockchaincomponent.Block {
	genesis := blockchaincomponent.NewBlock(0, "0x_Genesis")
	genesis.TimeStamp = canonicalGenesisTimestamp
	return genesis
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
