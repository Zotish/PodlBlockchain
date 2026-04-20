#!/bin/sh
set -eu

PORT_TO_USE="${PORT:-8080}"
CHAIN_URL_VALUE="${CHAIN_URL:-http://127.0.0.1:6500}"

exec ./bin/lqd wallet \
  -port "$PORT_TO_USE" \
  -node_address "$CHAIN_URL_VALUE"
