#!/bin/sh
set -eu

PORT_TO_USE="${PORT:-6500}"
P2P_PORT_TO_USE="${P2P_PORT:-6100}"
DATA_DIR="${LQD_DATA_DIR:-${RAILWAY_VOLUME_MOUNT_PATH:-/app/data}}"
BRIDGE_DIR="${LQD_BRIDGE_DATA_DIR:-$DATA_DIR}"
CHAIN_DB_PATH="${CHAIN_DB_PATH:-$DATA_DIR/chain/evodb}"

mkdir -p "$DATA_DIR" "$BRIDGE_DIR" "$(dirname "$CHAIN_DB_PATH")"

export LQD_DATA_DIR="$DATA_DIR"
export LQD_BRIDGE_DATA_DIR="$BRIDGE_DIR"
export BRIDGE_STATE_FILE="${BRIDGE_STATE_FILE:-$DATA_DIR/bridge_relayer_state.json}"

set -- ./bin/lqd chain \
  -port "$PORT_TO_USE" \
  -p2p_port "$P2P_PORT_TO_USE" \
  -db_path "$CHAIN_DB_PATH" \
  -validator "${VALIDATOR_ADDRESS:?VALIDATOR_ADDRESS is required}" \
  -stake_amount "${STAKE_AMOUNT:-3000000}" \
  -min_stake "${MIN_STAKE:-100000}" \
  -mining="${MINING_ENABLED:-true}"

if [ -n "${REMOTE_NODE:-}" ]; then
  set -- "$@" -remote_node "$REMOTE_NODE"
fi

if [ -n "${DEX_ADDRESS:-}" ] && [ -n "${LP_TOKEN_AMOUNT:-}" ]; then
  set -- "$@" -dex_address "$DEX_ADDRESS" -lp_token_amount "$LP_TOKEN_AMOUNT"
fi

exec "$@"
