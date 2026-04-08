#!/bin/bash
# run-all.sh — Start all LQD blockchain services
# Usage: ./run-all.sh [validator_address]

ROOT="$(cd "$(dirname "$0")" && pwd)"
VALIDATOR="${1:-0xFd220d301291dc32D491A9Fe5b872b8aB5F028D5}"
STAKE=3000000
LOGS="$ROOT/.logs"
mkdir -p "$LOGS"

# ── Kill existing processes on these ports ─────────────────────────────────
echo "Freeing ports 6500 8080 9000 3000 3001 ..."
for PORT in 6500 8080 9000 3000 3001; do
  PID=$(lsof -ti tcp:"$PORT" 2>/dev/null)
  if [ -n "$PID" ]; then
    echo "  Killing PID $PID on port $PORT"
    kill -9 $PID 2>/dev/null || true
  fi
done
sleep 1

# ── Write launcher scripts for each service ───────────────────────────────
write_launcher() {
  local FILE="$1"
  local TITLE="$2"
  local WORKDIR="$3"
  local CMD="$4"
  cat >"$FILE" <<EOF
#!/bin/bash
printf '\033]0;${TITLE}\007'
cd "${WORKDIR}"
${CMD}
echo ""
echo "--- ${TITLE} exited (press Enter to close) ---"
read -r
EOF
  chmod +x "$FILE"
}

CHAIN_SH="$LOGS/run_chain.sh"
WALLET_SH="$LOGS/run_wallet.sh"
AGG_SH="$LOGS/run_agg.sh"
EXPLORER_SH="$LOGS/run_explorer.sh"
DEX_SH="$LOGS/run_dex.sh"

write_launcher "$CHAIN_SH" "Chain :6500" "$ROOT" \
  "go run main.go chain -port 6500 -p2p_port 6000 -db_path 5000/evodb -validator $VALIDATOR -stake_amount $STAKE -mining=true 2>&1 | tee '$LOGS/chain.log'"

write_launcher "$WALLET_SH" "Wallet :8080" "$ROOT" \
  "go run main.go wallet -port 8080 -node_address http://127.0.0.1:6500 2>&1 | tee '$LOGS/wallet.log'"

write_launcher "$AGG_SH" "Aggregator :9000" "$ROOT" \
  "go run main.go aggregate -port 9000 -canonical http://127.0.0.1:6500 -wallet http://127.0.0.1:8080 2>&1 | tee '$LOGS/aggregator.log'"

write_launcher "$EXPLORER_SH" "Explorer :3001" "$ROOT/blockchain-explorer" \
  "PORT=3001 npm start 2>&1 | tee '$LOGS/explorer.log'"

write_launcher "$DEX_SH" "SwapDEX :3000" "$ROOT/swap-dex" \
  "PORT=3000 npm start 2>&1 | tee '$LOGS/swapdex.log'"

# ── Open each in a new Terminal window ────────────────────────────────────
open_in_terminal() {
  local SCRIPT="$1"
  # Use single quotes around the path inside the AppleScript string to handle spaces
  osascript -e "tell application \"Terminal\" to do script \"bash '${SCRIPT}'\""
  sleep 1
}

echo ""
echo "Starting LQD blockchain stack..."
echo "  Validator : $VALIDATOR"
echo ""

open_in_terminal "$CHAIN_SH"
sleep 3
open_in_terminal "$WALLET_SH"
sleep 2
open_in_terminal "$AGG_SH"
sleep 2
open_in_terminal "$EXPLORER_SH"
sleep 1
open_in_terminal "$DEX_SH"

echo ""
echo "All services launched in Terminal windows."
echo "Logs: $LOGS/"
echo ""
echo "  Chain node  → http://127.0.0.1:6500"
echo "  Wallet srv  → http://127.0.0.1:8080"
echo "  Aggregator  → http://127.0.0.1:9000"
echo "  Explorer    → http://localhost:3001"
echo "  Swap DEX    → http://localhost:3000"
echo ""
echo "Extension: chrome://extensions → Developer mode → Load unpacked"
echo "           → $(basename "$ROOT")/lqd-wallet-extension/"
