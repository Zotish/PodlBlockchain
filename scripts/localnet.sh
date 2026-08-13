#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
RUN_DIR="$ROOT/.podl-localnet"
ACTION="${1:-status}"

mkdir -p "$RUN_DIR"

start_service() {
  name="$1"
  shift
  pid_file="$RUN_DIR/$name.pid"
  if [ -f "$pid_file" ] && kill -0 "$(sed -n '1p' "$pid_file")" 2>/dev/null; then
    return
  fi
  (cd "$ROOT" && nohup "$@" >"$RUN_DIR/$name.log" 2>&1 & echo $! >"$pid_file")
}

stop_service() {
  name="$1"
  pid_file="$RUN_DIR/$name.pid"
  if [ -f "$pid_file" ]; then
    pid=$(sed -n '1p' "$pid_file")
    case "$pid" in (*[!0-9]*|'') return 1;; esac
    kill "$pid" 2>/dev/null || true
    rm -f "$pid_file"
  fi
}

case "$ACTION" in
  up)
    start_service chain env PORT=16500 P2P_PORT=16100 LQD_DATA_DIR="$RUN_DIR/data" MINING_ENABLED=true sh scripts/railway/start-chain.sh
    start_service wallet env PORT=18080 CHAIN_URL=http://127.0.0.1:16500 sh scripts/railway/start-wallet.sh
    start_service aggregator env PORT=19000 CHAIN_URL=http://127.0.0.1:16500 WALLET_URL=http://127.0.0.1:18080 sh scripts/railway/start-aggregator.sh
    printf '%s\n' 'PoDL localnet starting: chain=16500 wallet=18080 aggregator=19000'
    ;;
  down)
    stop_service aggregator
    stop_service wallet
    stop_service chain
    printf '%s\n' 'PoDL localnet stopped'
    ;;
  status)
    for name in chain wallet aggregator; do
      pid_file="$RUN_DIR/$name.pid"
      if [ -f "$pid_file" ] && kill -0 "$(sed -n '1p' "$pid_file")" 2>/dev/null; then
        printf '%s\n' "$name=running"
      else
        printf '%s\n' "$name=stopped"
      fi
    done
    ;;
  *)
    printf '%s\n' 'usage: scripts/localnet.sh up|down|status' >&2
    exit 2
    ;;
esac
