# Validator Node Setup

This guide covers the two production cases for PoDL mainnet/testnet.

## Case 1: Main VPS as bootstrap/canonical node

The main VPS keeps the canonical chain online and acts as the first P2P bootstrap peer.

Required ports:

- `6500/tcp` for chain HTTP API
- `6100/tcp` for validator P2P
- `8080/tcp` for wallet API
- `9000/tcp` for aggregator API

Main VPS `.env` example:

```env
VALIDATOR_ADDRESS=0x84c4b2D364153AB012dcE3eaaBD002839b825fA3
STAKE_AMOUNT=3000000
MIN_STAKE=100000
MINING_ENABLED=true
PORT=6500
P2P_PORT=6100
CHAIN_DB_PATH=/app/data/chain/evodb
LQD_DATA_DIR=/app/data
LQD_BRIDGE_DATA_DIR=/app/data
```

In this mode the bootstrap node can finalize blocks even when no remote validators are connected yet. Registered validators that are offline do not block the chain.

## Case 2: User validator on own VPS/PC

A validator becomes eligible after registration, but it contributes to block verification only after running a node and connecting to the bootstrap P2P node.

Validator node `.env` example:

```env
VALIDATOR_ADDRESS=<USER_REGISTERED_VALIDATOR_ADDRESS>
STAKE_AMOUNT=0
MIN_STAKE=100000
MINING_ENABLED=true
PORT=6500
P2P_PORT=6100
REMOTE_NODE=178.105.133.94:6100
CHAIN_DB_PATH=/app/data/chain/evodb
LQD_DATA_DIR=/app/data
LQD_BRIDGE_DATA_DIR=/app/data
```

Validator operator steps:

1. Register as validator from the wallet after locking eligible LP.
2. Run the validator node with the same `VALIDATOR_ADDRESS`.
3. Open firewall ports `6100/tcp` and `6500/tcp`.
4. Set `REMOTE_NODE` to the bootstrap node P2P address.
5. Start the node and verify `/health` shows the node is online.
6. On the bootstrap node, check `/peers` and `/validators` to confirm the validator is active.

Once the remote validator is connected and healthy, it becomes part of the active voting set. Blocks then require votes from active validator nodes, and participant validator rewards can be distributed to voters.

## Production Notes

- Registered but offline validators do not count toward block-finalization quorum.
- Active P2P validators do count toward quorum.
- If a validator disconnects, the active voting set shrinks so the chain does not stall.
- For public mainnet, every validator should run its own node instead of sharing the bootstrap VPS.
