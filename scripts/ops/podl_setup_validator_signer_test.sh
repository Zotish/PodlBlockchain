#!/usr/bin/env sh
set -eu

REPO_ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
TEST_ROOT="$(mktemp -d)"
trap 'rm -rf "$TEST_ROOT"' EXIT HUP INT TERM
MOCK_BIN="$TEST_ROOT/bin"
PODL_ROOT="$TEST_ROOT/podl"
EXPECTED_ADDRESS="0x1111111111111111111111111111111111111111"
mkdir -p "$MOCK_BIN" "$PODL_ROOT/data"

printf '%s\n' \
  '#!/bin/sh' \
  'case "$1" in' \
  '  pull) exit 0 ;;' \
  '  run)' \
  '    secret_dir=""' \
  '    create=false' \
  '    previous=""' \
  '    for arg in "$@"; do' \
  '      if [ "$previous" = "-v" ]; then case "$arg" in *:/secure|*:/secure:rw) secret_dir="${arg%%:*}" ;; esac; fi' \
  '      if [ "$arg" = "-create_key_file" ]; then create=true; fi' \
  '      previous="$arg"' \
  '    done' \
  '    if [ "$create" = "true" ]; then printf "%s\n" "{\"version\":1}" > "$secret_dir/validator-key.json"; exit 0; fi' \
  '    echo mock-container-id; exit 0 ;;' \
  '  exec) echo "{\"healthy\":true,\"address\":\"0x1111111111111111111111111111111111111111\",\"vrf_suite\":\"ECVRF-P256-SHA256-TAI\"}"; exit 0 ;;' \
  '  rm) exit 0 ;;' \
  'esac' \
  'exit 0' > "$MOCK_BIN/docker"
chmod 700 "$MOCK_BIN/docker"

printf '%s\n' \
  "VALIDATOR_ADDRESS=$EXPECTED_ADDRESS" \
  'VALIDATOR_PRIVATE_KEY=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa' \
  'ENABLE_VALIDATOR_SIGNER=false' \
  'LQD_REQUIRE_SIGNED_BFT=false' > "$PODL_ROOT/.env"
chmod 600 "$PODL_ROOT/.env"

PATH="$MOCK_BIN:$PATH" PODL_ROOT="$PODL_ROOT" LQD_IMAGE="registry.example/podl:test" SKIP_IMAGE_PULL=true \
  sh "$REPO_ROOT/scripts/ops/podl_setup_validator_signer.sh" >/dev/null

grep -q '^ENABLE_VALIDATOR_SIGNER=false$' "$PODL_ROOT/.env"
grep -q '^VALIDATOR_PRIVATE_KEY=' "$PODL_ROOT/.env"
test -s "$PODL_ROOT/signer-migration.pending"
test -s "$PODL_ROOT/signer-migration.next.env"
grep -q '^ENABLE_VALIDATOR_SIGNER=true$' "$PODL_ROOT/signer-migration.next.env"
grep -q '^LQD_REQUIRE_SIGNED_BFT=true$' "$PODL_ROOT/signer-migration.next.env"
if grep -q '^VALIDATOR_PRIVATE_KEY=' "$PODL_ROOT/signer-migration.next.env"; then
  echo "Raw validator key remained in the staged signer environment" >&2
  exit 1
fi
test -s "$PODL_ROOT/signer-secrets/validator-key.json"
test -s "$PODL_ROOT/signer-secrets/signer.crt"
test -s "$PODL_ROOT/signer-secrets/chain-client.crt"
encrypted_backup="$(find "$PODL_ROOT/signer-ca-backup" -name 'env-before-signer-*.enc' -type f | head -1)"
test -s "$encrypted_backup"

PATH="$MOCK_BIN:$PATH" PODL_ROOT="$PODL_ROOT" LQD_IMAGE="registry.example/podl:test" SKIP_IMAGE_PULL=true \
  sh "$REPO_ROOT/scripts/ops/podl_setup_validator_signer.sh" >/dev/null

echo "PASS signer provisioning creates mTLS material, verifies address, stages raw-key-free config and is idempotent"
