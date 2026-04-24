#!/bin/sh
set -eu

mkdir -p bin
CGO_ENABLED=1 go build -o bin/lqd .
CGO_ENABLED=1 go run ./scripts/railway/precompile_builtins.go
