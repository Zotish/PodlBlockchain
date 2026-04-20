#!/bin/sh
set -eu

mkdir -p bin
CGO_ENABLED=1 go build -o bin/lqd .
