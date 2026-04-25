#!/bin/sh
set -eu

CACHE_ROOT="${LQD_GO_CACHE_ROOT:-$PWD/.lqd-go-cache}"
export GOCACHE="${GOCACHE:-$CACHE_ROOT/build}"
export GOMODCACHE="${GOMODCACHE:-$CACHE_ROOT/mod}"

PORT_TO_USE="${PORT:-9000}"
CHAIN_URL_VALUE="${CHAIN_URL:-http://127.0.0.1:6500}"
WALLET_URL_VALUE="${WALLET_URL:-http://127.0.0.1:8080}"
AGGREGATOR_NODES_VALUE="${AGGREGATOR_NODES:-auto}"

mkdir -p "$GOCACHE" "$GOMODCACHE"

set -- ./bin/lqd aggregate \
  -port "$PORT_TO_USE" \
  -canonical "$CHAIN_URL_VALUE" \
  -wallet "$WALLET_URL_VALUE"

if [ "$AGGREGATOR_NODES_VALUE" != "auto" ] && [ -n "$AGGREGATOR_NODES_VALUE" ]; then
  set -- "$@" -nodes "$AGGREGATOR_NODES_VALUE"
fi

exec "$@"
