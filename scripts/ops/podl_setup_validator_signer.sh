#!/usr/bin/env sh
set -eu

PODL_ROOT="${PODL_ROOT:-/opt/podl}"
LQD_IMAGE="${LQD_IMAGE:?LQD_IMAGE is required}"
SKIP_IMAGE_PULL="${SKIP_IMAGE_PULL:-false}"
ENV_FILE="$PODL_ROOT/.env"
SECRETS_DIR="$PODL_ROOT/signer-secrets"
CA_BACKUP_DIR="$PODL_ROOT/signer-ca-backup"
DATA_DIR="$PODL_ROOT/data"
NEXT_ENV="$PODL_ROOT/signer-migration.next.env"
PENDING_FILE="$PODL_ROOT/signer-migration.pending"

cd "$PODL_ROOT"
for command_name in docker openssl awk sed; do
  if ! command -v "$command_name" >/dev/null 2>&1; then
    echo "$command_name is required to provision the validator signer" >&2
    exit 1
  fi
done
if [ ! -f "$ENV_FILE" ]; then echo "$ENV_FILE is required" >&2; exit 1; fi

read_env() {
  sed -n "s/^$1=//p" "$ENV_FILE" | tail -1
}

expected_address="$(read_env VALIDATOR_ADDRESS)"
validator_private_key="$(read_env VALIDATOR_PRIVATE_KEY)"
if [ -z "$expected_address" ]; then echo "VALIDATOR_ADDRESS is missing from .env" >&2; exit 1; fi

if [ "$(read_env ENABLE_VALIDATOR_SIGNER | tr '[:upper:]' '[:lower:]')" = "true" ]; then
  echo "Validator signer is already enabled; no migration was performed."
  exit 0
fi
if [ -s "$PENDING_FILE" ] && [ -s "$NEXT_ENV" ]; then
  echo "A verified validator signer migration is already staged; no migration was performed."
  exit 0
fi
if [ -z "$validator_private_key" ]; then
  echo "VALIDATOR_PRIVATE_KEY is required for the one-time encrypted-key migration" >&2
  exit 1
fi

umask 077
mkdir -p "$SECRETS_DIR" "$CA_BACKUP_DIR" "$DATA_DIR/signer"
chmod 700 "$SECRETS_DIR" "$CA_BACKUP_DIR" "$DATA_DIR/signer"

cleanup_files=""
cleanup_container=""
cleanup() {
  if [ -n "$cleanup_container" ]; then docker rm -f "$cleanup_container" >/dev/null 2>&1 || true; fi
  for cleanup_file in $cleanup_files; do rm -f "$cleanup_file"; done
}
trap cleanup EXIT HUP INT TERM

passphrase_file="$SECRETS_DIR/key-passphrase"
if [ ! -s "$passphrase_file" ]; then openssl rand -base64 48 > "$passphrase_file"; fi
chmod 600 "$passphrase_file"

if [ ! -s "$SECRETS_DIR/ca.crt" ] || [ ! -s "$SECRETS_DIR/signer.crt" ] || [ ! -s "$SECRETS_DIR/signer.key" ] || [ ! -s "$SECRETS_DIR/chain-client.crt" ] || [ ! -s "$SECRETS_DIR/chain-client.key" ]; then
  rm -f "$SECRETS_DIR/ca.crt" "$SECRETS_DIR/signer.crt" "$SECRETS_DIR/signer.key" "$SECRETS_DIR/chain-client.crt" "$SECRETS_DIR/chain-client.key"
  openssl req -x509 -newkey rsa:3072 -nodes -sha256 -days 3650 \
    -subj "/CN=PoDL Validator Signer CA" \
    -keyout "$CA_BACKUP_DIR/ca.key" -out "$SECRETS_DIR/ca.crt" >/dev/null 2>&1

  openssl req -newkey rsa:2048 -nodes -sha256 -subj "/CN=signer" \
    -keyout "$SECRETS_DIR/signer.key" -out "$SECRETS_DIR/signer.csr" >/dev/null 2>&1
  printf '%s\n' 'subjectAltName=DNS:signer,DNS:localhost,IP:127.0.0.1' 'extendedKeyUsage=serverAuth' > "$SECRETS_DIR/signer.ext"
  openssl x509 -req -sha256 -days 825 -in "$SECRETS_DIR/signer.csr" \
    -CA "$SECRETS_DIR/ca.crt" -CAkey "$CA_BACKUP_DIR/ca.key" -CAcreateserial \
    -extfile "$SECRETS_DIR/signer.ext" -out "$SECRETS_DIR/signer.crt" >/dev/null 2>&1

  openssl req -newkey rsa:2048 -nodes -sha256 -subj "/CN=podl-chain" \
    -keyout "$SECRETS_DIR/chain-client.key" -out "$SECRETS_DIR/chain-client.csr" >/dev/null 2>&1
  printf '%s\n' 'extendedKeyUsage=clientAuth' > "$SECRETS_DIR/chain-client.ext"
  openssl x509 -req -sha256 -days 825 -in "$SECRETS_DIR/chain-client.csr" \
    -CA "$SECRETS_DIR/ca.crt" -CAkey "$CA_BACKUP_DIR/ca.key" -CAcreateserial \
    -extfile "$SECRETS_DIR/chain-client.ext" -out "$SECRETS_DIR/chain-client.crt" >/dev/null 2>&1

  rm -f "$SECRETS_DIR/signer.csr" "$SECRETS_DIR/signer.ext" "$SECRETS_DIR/chain-client.csr" "$SECRETS_DIR/chain-client.ext" "$SECRETS_DIR/ca.srl"
fi
chmod 600 "$SECRETS_DIR"/* "$CA_BACKUP_DIR"/*

create_env="$SECRETS_DIR/.create-key.env"
cleanup_files="$cleanup_files $create_env"
{
  echo "LQD_VALIDATOR_PRIVATE_KEY=$validator_private_key"
  echo "LQD_VALIDATOR_KEY_PASSPHRASE=$(sed -n '1p' "$passphrase_file")"
} > "$create_env"
chmod 600 "$create_env"

if [ "$SKIP_IMAGE_PULL" = "true" ]; then
  docker image inspect "$LQD_IMAGE" >/dev/null
else
  docker pull "$LQD_IMAGE" >/dev/null
fi
rm -f "$SECRETS_DIR/validator-key.json"
docker run --rm --env-file "$create_env" \
  -v "$SECRETS_DIR:/secure" \
  "$LQD_IMAGE" ./bin/lqd signer -create_key_file /secure/validator-key.json >/dev/null
chmod 600 "$SECRETS_DIR/validator-key.json"
rm -f "$create_env"
cleanup_files=""

preflight_env="$SECRETS_DIR/.preflight.env"
cleanup_files="$preflight_env"
echo "LQD_VALIDATOR_KEY_PASSPHRASE=$(sed -n '1p' "$passphrase_file")" > "$preflight_env"
chmod 600 "$preflight_env"
cleanup_container="podl-signer-preflight-$$"
docker run -d --name "$cleanup_container" --env-file "$preflight_env" \
  -e LQD_VALIDATOR_KEY_FILE=/secure/validator-key.json \
  -e LQD_SIGNER_TLS_CERT=/secure/signer.crt \
  -e LQD_SIGNER_TLS_KEY=/secure/signer.key \
  -e LQD_SIGNER_CLIENT_CA=/secure/ca.crt \
  -e LQD_VALIDATOR_SLASHING_DB=/data/slashing-protection.json \
  -v "$SECRETS_DIR:/secure:ro" -v "$DATA_DIR/signer:/data" \
  "$LQD_IMAGE" ./bin/lqd signer -listen 127.0.0.1:9101 \
    -tls_cert /secure/signer.crt -tls_key /secure/signer.key -client_ca /secure/ca.crt \
    -key_file /secure/validator-key.json -slashing_db /data/slashing-protection.json >/dev/null

signer_status=""
attempt=1
while [ "$attempt" -le 30 ]; do
  signer_status="$(docker exec "$cleanup_container" curl -fsS --cacert /secure/ca.crt \
    --cert /secure/chain-client.crt --key /secure/chain-client.key \
    https://127.0.0.1:9101/v1/status 2>/dev/null || true)"
  if [ -n "$signer_status" ]; then break; fi
  sleep 1
  attempt=$((attempt + 1))
done
actual_address="$(printf '%s' "$signer_status" | sed -n 's/.*"address"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')"
if [ -z "$actual_address" ] || [ "$(printf '%s' "$actual_address" | tr '[:upper:]' '[:lower:]')" != "$(printf '%s' "$expected_address" | tr '[:upper:]' '[:lower:]')" ]; then
  echo "Encrypted signer key does not match VALIDATOR_ADDRESS; .env was not changed" >&2
  exit 1
fi
docker rm -f "$cleanup_container" >/dev/null
cleanup_container=""
rm -f "$preflight_env"
cleanup_files=""

stamp="$(date -u +%Y%m%d-%H%M%S)"
env_backup="$CA_BACKUP_DIR/env-before-signer-$stamp.enc"
openssl enc -aes-256-cbc -pbkdf2 -salt -pass file:"$passphrase_file" -in "$ENV_FILE" -out "$env_backup"
chmod 600 "$env_backup"
next_env="$NEXT_ENV.tmp"
cleanup_files="$next_env"
awk -F= '
  $1 != "VALIDATOR_PRIVATE_KEY" &&
  $1 != "LQD_VALIDATOR_VRF_PRIVATE_KEY" &&
  $1 != "ENABLE_VALIDATOR_SIGNER" &&
  $1 != "LQD_REQUIRE_SIGNED_BFT" &&
  $1 != "LQD_VALIDATOR_SIGNER_URL" &&
  $1 != "LQD_VALIDATOR_SIGNER_CA" &&
  $1 != "LQD_VALIDATOR_SIGNER_CERT" &&
  $1 != "LQD_VALIDATOR_SIGNER_KEY" &&
  $1 != "LQD_VALIDATOR_SIGNER_NAME" &&
  $1 != "LQD_VALIDATOR_SLASHING_DB" { print }
' "$ENV_FILE" > "$next_env"
{
  echo "ENABLE_VALIDATOR_SIGNER=true"
  echo "LQD_REQUIRE_SIGNED_BFT=true"
  echo "LQD_VALIDATOR_SIGNER_URL=https://signer:9101"
  echo "LQD_VALIDATOR_SIGNER_CA=/run/podl-signer/ca.crt"
  echo "LQD_VALIDATOR_SIGNER_CERT=/run/podl-signer/chain-client.crt"
  echo "LQD_VALIDATOR_SIGNER_KEY=/run/podl-signer/chain-client.key"
  echo "LQD_VALIDATOR_SIGNER_NAME=signer"
  echo "LQD_VALIDATOR_SLASHING_DB=/app/data/signer/slashing-protection.json"
} >> "$next_env"
chmod 600 "$next_env"
mv "$next_env" "$NEXT_ENV"

pending_tmp="$PENDING_FILE.tmp"
cleanup_files="$pending_tmp"
{
  echo "ENV_BACKUP=$env_backup"
  echo "NEXT_ENV=$NEXT_ENV"
} > "$pending_tmp"
chmod 600 "$pending_tmp"
mv "$pending_tmp" "$PENDING_FILE"
cleanup_files=""

validator_private_key=""
unset validator_private_key
echo "Validator signer provisioned, address-verified and staged for atomic deployment."
echo "The active .env is unchanged until deployment; the staged config excludes the raw key."
echo "AES-256 encrypted rollback backup retained at $env_backup with mode 0600."
