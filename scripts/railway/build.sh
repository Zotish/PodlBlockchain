#!/bin/sh
set -eu

CACHE_ROOT="${LQD_GO_CACHE_ROOT:-$PWD/.lqd-go-cache}"
export GOCACHE="${GOCACHE:-$CACHE_ROOT/build}"
export GOMODCACHE="${GOMODCACHE:-$CACHE_ROOT/mod}"

mkdir -p bin "$GOCACHE" "$GOMODCACHE"
CGO_ENABLED=1 go build -o bin/lqd .
CGO_ENABLED=1 go run ./scripts/railway/precompile_builtins.go
