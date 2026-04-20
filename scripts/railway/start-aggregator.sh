#!/bin/sh
set -eu

PORT_TO_USE="${PORT:-9000}"
CHAIN_URL_VALUE="${CHAIN_URL:-http://127.0.0.1:6500}"
WALLET_URL_VALUE="${WALLET_URL:-http://127.0.0.1:8080}"
AGGREGATOR_NODES_VALUE="${AGGREGATOR_NODES:-auto}"

set -- ./bin/lqd aggregate \
  -port "$PORT_TO_USE" \
  -canonical "$CHAIN_URL_VALUE" \
  -wallet "$WALLET_URL_VALUE"

if [ "$AGGREGATOR_NODES_VALUE" != "auto" ] && [ -n "$AGGREGATOR_NODES_VALUE" ]; then
  set -- "$@" -nodes "$AGGREGATOR_NODES_VALUE"
fi

exec "$@"
