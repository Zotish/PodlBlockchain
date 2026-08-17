#!/bin/sh
set -eu

CACHE_ROOT="${LQD_GO_CACHE_ROOT:-$PWD/.lqd-go-cache}"
export GOCACHE="${GOCACHE:-$CACHE_ROOT/build}"
export GOMODCACHE="${GOMODCACHE:-$CACHE_ROOT/mod}"

PORT_TO_USE="${PORT:-6500}"
P2P_PORT_TO_USE="${P2P_PORT:-6100}"
DATA_DIR="${LQD_DATA_DIR:-${RAILWAY_VOLUME_MOUNT_PATH:-/app/data}}"
BRIDGE_DIR="${LQD_BRIDGE_DATA_DIR:-$DATA_DIR}"
CHAIN_DB_PATH="${CHAIN_DB_PATH:-$DATA_DIR/chain/evodb}"

mkdir -p "$GOCACHE" "$GOMODCACHE" "$DATA_DIR" "$BRIDGE_DIR" "$(dirname "$CHAIN_DB_PATH")"

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
  -mining="${MINING_ENABLED:-true}" \
  -require_signed_bft="${LQD_REQUIRE_SIGNED_BFT:-false}"

if [ -n "${REMOTE_NODE:-}" ]; then
  set -- "$@" -remote_node "$REMOTE_NODE"
fi

if [ -z "${LQD_VALIDATOR_SIGNER_URL:-}" ] && [ -z "${LQD_VALIDATOR_KEY_FILE:-}" ] && [ -n "${VALIDATOR_PRIVATE_KEY:-}" ]; then
  set -- "$@" -validator_private_key "$VALIDATOR_PRIVATE_KEY"
fi

if [ -n "${LQD_VALIDATOR_VRF_PRIVATE_KEY:-}" ]; then
  set -- "$@" -validator_vrf_private_key "$LQD_VALIDATOR_VRF_PRIVATE_KEY"
fi

if [ -n "${LQD_VALIDATOR_KEY_FILE:-}" ]; then
  set -- "$@" -validator_key_file "$LQD_VALIDATOR_KEY_FILE"
fi

if [ -n "${LQD_VALIDATOR_SIGNER_URL:-}" ]; then
  set -- "$@" \
    -validator_signer_url "$LQD_VALIDATOR_SIGNER_URL" \
    -validator_signer_ca "${LQD_VALIDATOR_SIGNER_CA:?LQD_VALIDATOR_SIGNER_CA is required}" \
    -validator_signer_cert "${LQD_VALIDATOR_SIGNER_CERT:?LQD_VALIDATOR_SIGNER_CERT is required}" \
    -validator_signer_key "${LQD_VALIDATOR_SIGNER_KEY:?LQD_VALIDATOR_SIGNER_KEY is required}"
  if [ -n "${LQD_VALIDATOR_SIGNER_NAME:-}" ]; then
    set -- "$@" -validator_signer_name "$LQD_VALIDATOR_SIGNER_NAME"
  fi
fi

if [ -n "${LQD_VALIDATOR_SLASHING_DB:-}" ]; then
  set -- "$@" -validator_slashing_db "$LQD_VALIDATOR_SLASHING_DB"
fi

DEX_ADDRESS_TO_USE="${DEX_ADDRESS:-${VALIDATOR_DEX_ADDRESS:-}}"
LP_TOKEN_AMOUNT_TO_USE="${LP_TOKEN_AMOUNT:-${VALIDATOR_LP_TOKEN_AMOUNT:-}}"
if [ -n "$DEX_ADDRESS_TO_USE" ] && [ -n "$LP_TOKEN_AMOUNT_TO_USE" ]; then
  set -- "$@" -dex_address "$DEX_ADDRESS_TO_USE" -lp_token_amount "$LP_TOKEN_AMOUNT_TO_USE"
fi

exec "$@"
