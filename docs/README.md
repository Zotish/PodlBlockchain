# DefenceProject Blockchain — Operator & Validator Guide

## Overview
This document explains how to run the chain, add validators, and upgrade safely.

## Requirements
- Go 1.21+
- Linux/macOS
- Open ports: HTTP (5000), P2P (6000) per node

## Quick Start (Single Miner)
```bash
go run main.go chain -port 5000 -p2p_port 6000 \
  -db_path 5000/evodb \
  -validator 0xYOUR_VALIDATOR \
  -stake_amount 3000000 \
  -mining=true
```

## Add Validator Node (Sync‑only)
```bash
go run main.go chain -port 5001 -p2p_port 6001 \
  -db_path 5001/evodb \
  -remote_node 127.0.0.1:6000 \
  -validator 0xYOUR_VALIDATOR \
  -stake_amount 4000000 \
  -mining=false
```

## Add Peer Manually (if needed)
```bash
curl -X POST http://127.0.0.1:5000/peers/add \
  -H "Content-Type: application/json" \
  -d '{"address":"127.0.0.1","port":6001}'
```

## Verify Validator Sync
```bash
curl http://127.0.0.1:5000/validators
```

## Wallet Server
```bash
go run main.go wallet -port 8080 -node_address http://127.0.0.1:5000
```

## Aggregator (Global API)
```bash
go run main.go aggregate -port 9000 \
  -nodes 127.0.0.1:5000,127.0.0.1:5001 \
  -canonical http://127.0.0.1:5000 \
  -wallet http://127.0.0.1:8080
```

## Chain Upgrade (Patch)
If you fix a bug or update fees, keep the same DB path and re‑start the node. It will resume from the current block.

```bash
cd DefenceProject
git pull
go build -o chain main.go
./chain chain -port 5000 -p2p_port 6000 -db_path 5000/evodb \
  -validator 0xYOUR_VALIDATOR -stake_amount 3000000 -mining=true
```

## Bridge (BSC Testnet)
Set env vars before starting the chain:
```bash
export BSC_TESTNET_RPC="https://bsc-testnet.publicnode.com"
export BSC_TESTNET_PRIVATE_KEY="<bsc private key>"
export BSC_BRIDGE_ADDRESS="<bsc bridge contract>"
export BSC_LOCK_ADDRESS="<bsc lock contract>"
export BRIDGE_BACKFILL_BLOCKS=2000
export BRIDGE_MAX_RANGE=40000
```

## Notes
- Using the same `-db_path` means the chain **continues** from the current height.
- Use **one miner** for stable consensus; others sync only.
- For production, use a dedicated server with NVMe SSD.

## Validator Requirements & Command

### Requirements
- Public IP (or reachable LAN)
- Open ports: HTTP (node port, e.g. 5001) and P2P (e.g. 6001)
- Go 1.21+
- Unique DB path per validator
- Stake amount (any positive integer)

### Validator (Sync‑only) Command
```bash
go run main.go chain -port 5001 -p2p_port 6001 \
  -db_path 5001/evodb \
  -remote_node <GENESIS_IP>:6000 \
  -validator 0xYOUR_VALIDATOR \
  -stake_amount 4000000 \
  -mining=false
```

### Notes
- Use **mining=false** for validators (non‑miner).
- Ensure the genesis node (miner) has your validator in its list; if not, call:
```bash
curl -X POST http://<GENESIS_IP>:5000/peers/add \
  -H "Content-Type: application/json" \
  -d '{"address":"<VALIDATOR_IP>","port":6001}'
```

Then ask the genesis node to sync validators:
```bash
curl -X POST http://<GENESIS_IP>:9000/validators/sync
```

## Validator Onboarding Checklist

1. **Network**
   - Open HTTP port (e.g. 5001) and P2P port (e.g. 6001)
   - Ensure outbound access to the genesis node P2P port

2. **Node setup**
   - Install Go 1.21+
   - Clone repo and run sync‑only command

3. **Staking**
   - Provide `-stake_amount` at startup (integer)

4. **Sync**
   - Ensure validator appears on genesis node `/validators`
   - If missing, add peer + run `/validators/sync`

5. **Monitoring**
   - Check `/validators`, `/getheight`, `/network`
   - Watch logs for validator selection and reward distribution

## Staking Minimum
Currently **no enforced minimum stake** in code. The validator stake is whatever you pass as `-stake_amount`. If you want a minimum, I can add a config constant (e.g., `MIN_VALIDATOR_STAKE`) and reject below it.
