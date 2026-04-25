#!/bin/sh
set -eu

CACHE_ROOT="${LQD_GO_CACHE_ROOT:-$PWD/.lqd-go-cache}"
export GOCACHE="${GOCACHE:-$CACHE_ROOT/build}"
export GOMODCACHE="${GOMODCACHE:-$CACHE_ROOT/mod}"

PORT_TO_USE="${PORT:-8080}"
CHAIN_URL_VALUE="${CHAIN_URL:-http://127.0.0.1:6500}"

mkdir -p "$GOCACHE" "$GOMODCACHE"

exec ./bin/lqd wallet \
  -port "$PORT_TO_USE" \
  -node_address "$CHAIN_URL_VALUE"
