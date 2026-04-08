#!/bin/bash
# reset-chain.sh — wipe all chain data for a fresh start
# Usage: ./reset-chain.sh

set -e
ROOT="$(cd "$(dirname "$0")" && pwd)"

echo "⚠️  This will DELETE all blockchain data. Are you sure? (y/N)"
read -r CONFIRM
if [ "$CONFIRM" != "y" ] && [ "$CONFIRM" != "Y" ]; then
  echo "Aborted."
  exit 0
fi

echo "🗑  Deleting chain databases..."
rm -rf "$ROOT/data/contract_events_db"
rm -rf "$ROOT/data/contracts_db"
rm -rf "$ROOT/data/contracts"
rm -rf "$ROOT/data/evodb"
rm -rf "$ROOT/5000/evodb"
rm -rf "$ROOT/5001/evodb"
rm -rf "$ROOT/5002/evodb"

# Recreate empty directories
mkdir -p "$ROOT/data/contract_events_db"
mkdir -p "$ROOT/data/contracts_db"
mkdir -p "$ROOT/data/contracts"
mkdir -p "$ROOT/5000/evodb"

echo ""
echo "✅  Chain data wiped. Fresh start ready."
echo ""
echo "Next steps:"
echo "  1. Start the blockchain node:"
echo "     go run main.go chain -port 5000 -p2p_port 6000 -db_path 5000/evodb \\"
echo "       -validator YOUR_ADDRESS -stake_amount 3000000 -mining=true"
echo ""
echo "  2. Deploy a fresh LQD token:"
echo "     POST /contract/deploy { type: lqd20, args: [\"LQD\",\"LQD\",\"1000000000000000\"] }"
echo ""
echo "  3. Deploy a fresh DEX contract:"
echo "     POST /contract/deploy { type: dex_swap }"
echo ""
echo "  4. Call DEX.Init(tokenA_address, tokenB_address)"
echo ""
echo "  5. Update swap-dex/src/config.js with the new DEX_CONTRACT_ADDRESS"
