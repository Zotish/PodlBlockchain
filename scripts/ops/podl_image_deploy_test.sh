#!/usr/bin/env sh
set -eu

REPO_ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
TEST_ROOT="$(mktemp -d)"
trap 'rm -rf "$TEST_ROOT"' EXIT HUP INT TERM
MOCK_BIN="$TEST_ROOT/bin"
PODL_ROOT="$TEST_ROOT/podl"
mkdir -p "$MOCK_BIN" "$PODL_ROOT/backups"

printf '%s\n' \
  '#!/bin/sh' \
  'if [ "$1" = "compose" ]; then' \
  '  shift' \
  '  if [ "${1:-}" = "-f" ]; then shift 2; fi' \
  '  if [ "${1:-}" = "ps" ] && [ "${2:-}" = "-q" ] && [ "${3:-}" = "chain" ]; then echo chain-container; fi' \
  '  if [ "${1:-}" = "ps" ] && [ "${2:-}" = "-q" ] && [ "${3:-}" = "signer" ]; then echo signer-container; fi' \
  '  if [ "${1:-}" = "pull" ] && [ "${MOCK_PULL_FAIL:-false}" = "true" ]; then exit 1; fi' \
  '  exit 0' \
  'fi' \
  'if [ "$1" = "inspect" ]; then' \
  '  case "$*" in *State.Health*) echo healthy ;; *) echo sha256:old-image ;; esac' \
  '  exit 0' \
  'fi' \
  'exit 0' > "$MOCK_BIN/docker"

printf '%s\n' \
  '#!/bin/sh' \
  'url=""' \
  'for arg in "$@"; do url="$arg"; done' \
  'case "$url" in' \
  '  */getheight) echo "{\"height\":110908}" ;;' \
  '  */readiness/mainnet) echo "{\"status\":\"action_required\"}" ;;' \
  '  *) echo "{\"ok\":true}" ;;' \
  'esac' > "$MOCK_BIN/curl"

printf '%s\n' \
  '#!/bin/sh' \
  'mkdir -p "$PODL_ROOT/backups"' \
  'touch "$PODL_ROOT/backups/mock.tar.gz"' \
  'echo "$PODL_ROOT/backups/mock.tar.gz"' > "$PODL_ROOT/podl_snapshot.sh"

chmod 700 "$MOCK_BIN/docker" "$MOCK_BIN/curl" "$PODL_ROOT/podl_snapshot.sh"
printf '%s\n' 'ENABLE_VALIDATOR_SIGNER=false' 'ENABLE_CADDY=false' > "$PODL_ROOT/.env"
printf '%s\n' 'services: {}' > "$PODL_ROOT/docker-compose.yml"

PATH="$MOCK_BIN:$PATH" PODL_ROOT="$PODL_ROOT" LQD_IMAGE="registry.example/podl:test" \
  sh "$REPO_ROOT/scripts/ops/podl_image_deploy.sh" >/dev/null

test -s "$PODL_ROOT/last-deploy.env"
grep -q '^LQD_IMAGE=registry.example/podl:test$' "$PODL_ROOT/last-deploy.env"
grep -q '^PRE_HEIGHT=110908$' "$PODL_ROOT/last-deploy.env"
grep -q '^POST_HEIGHT=110908$' "$PODL_ROOT/last-deploy.env"

mkdir -p "$PODL_ROOT/signer-secrets" "$PODL_ROOT/signer-ca-backup"
for signer_file in ca.crt signer.crt signer.key chain-client.crt chain-client.key validator-key.json; do
  printf '%s\n' test > "$PODL_ROOT/signer-secrets/$signer_file"
done
printf '%s\n' 'test-passphrase' > "$PODL_ROOT/signer-secrets/key-passphrase"
openssl enc -aes-256-cbc -pbkdf2 -salt -pass file:"$PODL_ROOT/signer-secrets/key-passphrase" \
  -in "$PODL_ROOT/.env" -out "$PODL_ROOT/signer-ca-backup/env-before-signer.enc"
printf '%s\n' \
  'ENABLE_VALIDATOR_SIGNER=true' \
  'LQD_REQUIRE_SIGNED_BFT=true' \
  'LQD_VALIDATOR_SIGNER_URL=https://signer:9101' \
  'LQD_VALIDATOR_SIGNER_CA=/run/podl-signer/ca.crt' \
  'LQD_VALIDATOR_SIGNER_CERT=/run/podl-signer/chain-client.crt' \
  'LQD_VALIDATOR_SIGNER_KEY=/run/podl-signer/chain-client.key' > "$PODL_ROOT/signer-migration.next.env"
printf '%s\n' \
  "ENV_BACKUP=$PODL_ROOT/signer-ca-backup/env-before-signer.enc" \
  "NEXT_ENV=$PODL_ROOT/signer-migration.next.env" > "$PODL_ROOT/signer-migration.pending"

PATH="$MOCK_BIN:$PATH" PODL_ROOT="$PODL_ROOT" LQD_IMAGE="registry.example/podl:signed" \
  sh "$REPO_ROOT/scripts/ops/podl_image_deploy.sh" >/dev/null

grep -q '^ENABLE_VALIDATOR_SIGNER=true$' "$PODL_ROOT/.env"
grep -q '^LQD_REQUIRE_SIGNED_BFT=true$' "$PODL_ROOT/.env"
test ! -e "$PODL_ROOT/signer-migration.pending"
grep -q '^SIGNER_ENABLED=true$' "$PODL_ROOT/last-deploy.env"

printf '%s\n' \
  'VALIDATOR_ADDRESS=0x1111111111111111111111111111111111111111' \
  'VALIDATOR_PRIVATE_KEY=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa' \
  'ENABLE_VALIDATOR_SIGNER=false' \
  'LQD_REQUIRE_SIGNED_BFT=false' > "$PODL_ROOT/.env"
openssl enc -aes-256-cbc -pbkdf2 -salt -pass file:"$PODL_ROOT/signer-secrets/key-passphrase" \
  -in "$PODL_ROOT/.env" -out "$PODL_ROOT/signer-ca-backup/env-before-failed-signer.enc"
printf '%s\n' \
  'VALIDATOR_ADDRESS=0x1111111111111111111111111111111111111111' \
  'ENABLE_VALIDATOR_SIGNER=true' \
  'LQD_REQUIRE_SIGNED_BFT=true' \
  'LQD_VALIDATOR_SIGNER_URL=https://signer:9101' \
  'LQD_VALIDATOR_SIGNER_CA=/run/podl-signer/ca.crt' \
  'LQD_VALIDATOR_SIGNER_CERT=/run/podl-signer/chain-client.crt' \
  'LQD_VALIDATOR_SIGNER_KEY=/run/podl-signer/chain-client.key' > "$PODL_ROOT/signer-migration.next.env"
printf '%s\n' \
  "ENV_BACKUP=$PODL_ROOT/signer-ca-backup/env-before-failed-signer.enc" \
  "NEXT_ENV=$PODL_ROOT/signer-migration.next.env" > "$PODL_ROOT/signer-migration.pending"

if PATH="$MOCK_BIN:$PATH" MOCK_PULL_FAIL=true PODL_ROOT="$PODL_ROOT" LQD_IMAGE="registry.example/podl:broken" \
  sh "$REPO_ROOT/scripts/ops/podl_image_deploy.sh" >/dev/null 2>&1; then
  echo "Broken image pull should have triggered rollback" >&2
  exit 1
fi
grep -q '^ENABLE_VALIDATOR_SIGNER=false$' "$PODL_ROOT/.env"
grep -q '^VALIDATOR_PRIVATE_KEY=' "$PODL_ROOT/.env"
test ! -e "$PODL_ROOT/signer-migration.pending"

printf '%s\n' 'ENABLE_VALIDATOR_SIGNER=true' 'LQD_REQUIRE_SIGNED_BFT=false' 'ENABLE_CADDY=false' > "$PODL_ROOT/.env"
if PATH="$MOCK_BIN:$PATH" PODL_ROOT="$PODL_ROOT" LQD_IMAGE="registry.example/podl:test" \
  sh "$REPO_ROOT/scripts/ops/podl_image_deploy.sh" >/dev/null 2>&1; then
  echo "Signer preflight should fail when signed BFT is disabled" >&2
  exit 1
fi

echo "PASS image deploy health/height marker, atomic signer activation/rollback and fail-closed preflight"
