#!/usr/bin/env sh
set -eu

PODL_ROOT="${PODL_ROOT:-/opt/podl}"
CHAIN_URL="${CHAIN_URL:-http://127.0.0.1:6500}"
WALLET_URL="${WALLET_URL:-http://127.0.0.1:8080}"
AGG_URL="${AGG_URL:-http://127.0.0.1:9000}"
DEX_API_URL="${DEX_API_URL:-http://127.0.0.1:9100}"
COMPOSE="${COMPOSE:-docker compose}"
WAIT_ATTEMPTS="${WAIT_ATTEMPTS:-60}"
VERIFY_HEIGHT_ADVANCE="${VERIFY_HEIGHT_ADVANCE:-true}"
NEW_IMAGE="${LQD_IMAGE:?LQD_IMAGE is required}"
SKIP_IMAGE_PULL="${SKIP_IMAGE_PULL:-false}"
SNAPSHOT_SCRIPT="${SNAPSHOT_SCRIPT:-$PODL_ROOT/podl_snapshot.sh}"
ENV_FILE="$PODL_ROOT/.env"
PENDING_FILE="$PODL_ROOT/signer-migration.pending"
NEXT_ENV="$PODL_ROOT/signer-migration.next.env"

cd "$PODL_ROOT"
if [ ! -f .env ]; then
  echo "$PODL_ROOT/.env is required" >&2
  exit 1
fi
if [ ! -f docker-compose.yml ]; then
  echo "$PODL_ROOT/docker-compose.yml is required" >&2
  exit 1
fi

env_value() {
  setting="$1"
  source_file="${2:-$ENV_FILE}"
  sed -n "s/^$setting=//p" "$source_file" | tail -1
}

# Persist the selected runtime image in Compose's source of truth. Without this,
# a later systemd restart or scheduled snapshot reads the stale image from .env
# and silently rolls a successful deployment back.
persist_env_value() {
  setting="$1"
  value="$2"
  target_file="${3:-$ENV_FILE}"
  temp_file="$(mktemp "$PODL_ROOT/.env.tmp.XXXXXX")"
  if ! awk -v setting="$setting" -v value="$value" '
    BEGIN { written = 0 }
    index($0, setting "=") == 1 {
      if (!written) print setting "=" value
      written = 1
      next
    }
    { print }
    END { if (!written) print setting "=" value }
  ' "$target_file" > "$temp_file"; then
    rm -f "$temp_file"
    return 1
  fi
  chmod 600 "$temp_file"
  mv "$temp_file" "$target_file"
}

configure_profiles() {
  profile_env="$1"
  SIGNER_ENABLED="$(env_value ENABLE_VALIDATOR_SIGNER "$profile_env" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')"
  CADDY_ENABLED="$(env_value ENABLE_CADDY "$profile_env" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')"
  COMPOSE_PROFILES=""
  if [ "$SIGNER_ENABLED" = "true" ]; then COMPOSE_PROFILES="signer"; fi
  if [ "$CADDY_ENABLED" = "true" ]; then
    if [ -n "$COMPOSE_PROFILES" ]; then COMPOSE_PROFILES="$COMPOSE_PROFILES,proxy"; else COMPOSE_PROFILES="proxy"; fi
  fi
  export COMPOSE_PROFILES
}

TARGET_ENV="$ENV_FILE"
MIGRATION_PENDING=false
MIGRATION_BACKUP=""
if [ -s "$PENDING_FILE" ]; then
  if ! command -v openssl >/dev/null 2>&1; then
    echo "openssl is required for atomic signer-migration rollback" >&2
    exit 1
  fi
  marker_next="$(env_value NEXT_ENV "$PENDING_FILE")"
  MIGRATION_BACKUP="$(env_value ENV_BACKUP "$PENDING_FILE")"
  if [ "$marker_next" != "$NEXT_ENV" ] || [ ! -s "$NEXT_ENV" ] || [ ! -s "$MIGRATION_BACKUP" ]; then
    echo "Signer migration marker is invalid or incomplete" >&2
    exit 1
  fi
  TARGET_ENV="$NEXT_ENV"
  MIGRATION_PENDING=true
fi
configure_profiles "$TARGET_ENV"
previous_configured_image="$(env_value LQD_IMAGE "$ENV_FILE")"
if [ -z "$previous_configured_image" ]; then
  previous_configured_image="podl-blockchain:local"
fi

compose_run() {
  $COMPOSE -f docker-compose.yml "$@"
}

height() {
  curl -fsS "$CHAIN_URL/getheight" 2>/dev/null | sed -n 's/.*"height":[[:space:]]*\([0-9][0-9]*\).*/\1/p'
}

require_height() {
  current="$(height || true)"
  if [ -z "$current" ]; then
    echo "Unable to read chain height from $CHAIN_URL/getheight" >&2
    exit 1
  fi
  echo "$current"
}

pre_height="$(require_height)"
chain_container="$(compose_run ps -q chain 2>/dev/null || true)"
if [ -z "$chain_container" ]; then
  echo "Existing chain container was not found; refusing an unguarded production deploy" >&2
  exit 1
fi
previous_image="$(docker inspect -f '{{.Image}}' "$chain_container")"

if [ ! -x "$SNAPSHOT_SCRIPT" ] && [ ! -f "$SNAPSHOT_SCRIPT" ]; then
  echo "Snapshot script not found: $SNAPSHOT_SCRIPT" >&2
  exit 1
fi

if [ "$SIGNER_ENABLED" = "true" ]; then
  if [ "$(env_value LQD_REQUIRE_SIGNED_BFT "$TARGET_ENV" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')" != "true" ]; then
    echo "LQD_REQUIRE_SIGNED_BFT=true is required when the production signer is enabled" >&2
    exit 1
  fi
  for required_file in ca.crt signer.crt signer.key chain-client.crt chain-client.key validator-key.json key-passphrase; do
    if [ ! -s "$PODL_ROOT/signer-secrets/$required_file" ]; then
      echo "Missing signer secret file: signer-secrets/$required_file" >&2
      exit 1
    fi
  done
  for required_setting in LQD_VALIDATOR_SIGNER_URL LQD_VALIDATOR_SIGNER_CA LQD_VALIDATOR_SIGNER_CERT LQD_VALIDATOR_SIGNER_KEY; do
    if [ -z "$(env_value "$required_setting" "$TARGET_ENV")" ]; then
      echo "Missing signer setting in .env: $required_setting" >&2
      exit 1
    fi
  done
fi

backup_file="not-created"
signer_config_activated=false

rollback() {
  reason="$1"
  echo "Deploy failed: $reason" >&2
  echo "Rolling back to image $previous_image; data backup: $backup_file" >&2
  if [ "$signer_config_activated" = "true" ]; then
    rollback_env="$PODL_ROOT/.env.rollback"
    if ! openssl enc -d -aes-256-cbc -pbkdf2 \
      -pass file:"$PODL_ROOT/signer-secrets/key-passphrase" \
      -in "$MIGRATION_BACKUP" -out "$rollback_env"; then
      echo "CRITICAL: unable to decrypt the pre-signer environment for rollback" >&2
      exit 1
    fi
    chmod 600 "$rollback_env"
    mv "$rollback_env" "$ENV_FILE"
    rm -f "$PENDING_FILE"
    configure_profiles "$ENV_FILE"
  fi
  persist_env_value LQD_IMAGE "$previous_configured_image" "$ENV_FILE" || true
  export LQD_IMAGE="$previous_image"
  if [ "$SIGNER_ENABLED" = "true" ]; then
    compose_run up -d --no-deps signer >/dev/null 2>&1 || true
  fi
  compose_run up -d --force-recreate --remove-orphans >/dev/null 2>&1 || true
  exit 1
}

if ! backup_file="$(LQD_IMAGE="$previous_image" PODL_ROOT="$PODL_ROOT" sh "$SNAPSHOT_SCRIPT")"; then
  rollback "state snapshot failed"
fi

if [ "$MIGRATION_PENDING" = "true" ]; then
  mv "$NEXT_ENV" "$ENV_FILE"
  signer_config_activated=true
  configure_profiles "$ENV_FILE"
fi

export LQD_IMAGE="$NEW_IMAGE"
if [ "$SKIP_IMAGE_PULL" = "true" ]; then
  if ! docker image inspect "$NEW_IMAGE" >/dev/null 2>&1; then rollback "local image is unavailable"; fi
else
  if ! compose_run pull; then rollback "image pull failed"; fi
fi
required_scripts="/app/scripts/railway/start-chain.sh"
if [ "$SIGNER_ENABLED" = "true" ]; then
  required_scripts="$required_scripts /app/scripts/railway/start-signer.sh"
fi
if ! docker run --rm --entrypoint sh "$NEW_IMAGE" -c "for file in $required_scripts; do test -r \"\$file\" || exit 1; done"; then
  rollback "runtime image is missing a readable chain/signer startup script"
fi
# Switch Compose's durable source of truth only after the image passes
# availability/preflight, but before containers are reconciled. This closes the
# window where a concurrent systemd monitor or snapshot could restore the old
# .env image during deployment.
if ! persist_env_value LQD_IMAGE "$NEW_IMAGE" "$ENV_FILE"; then
  rollback "unable to persist the deployed image in $ENV_FILE"
fi

if [ "$SIGNER_ENABLED" = "true" ]; then
  if ! compose_run up -d --no-deps signer; then rollback "signer failed to start"; fi
  signer_ready=false
  attempt=1
  while [ "$attempt" -le 45 ]; do
    signer_container="$(compose_run ps -q signer 2>/dev/null || true)"
    signer_health=""
    if [ -n "$signer_container" ]; then
      signer_health="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "$signer_container" 2>/dev/null || true)"
    fi
    if [ "$signer_health" = "healthy" ]; then signer_ready=true; break; fi
    sleep 2
    attempt=$((attempt + 1))
  done
  if [ "$signer_ready" != "true" ]; then rollback "signer health check failed"; fi
fi

if ! compose_run up -d --no-deps chain; then rollback "chain failed to start"; fi
if ! compose_run up -d wallet aggregator dex-api; then rollback "supporting services failed to start"; fi
if ! compose_run up -d --remove-orphans; then rollback "stack reconciliation failed"; fi

ready=false
attempt=1
while [ "$attempt" -le "$WAIT_ATTEMPTS" ]; do
  if curl -fsS "$CHAIN_URL/health" >/dev/null 2>&1 \
    && curl -fsS "$WALLET_URL/health" >/dev/null 2>&1 \
    && curl -fsS "$AGG_URL/health" >/dev/null 2>&1 \
    && curl -fsS "$DEX_API_URL/health" >/dev/null 2>&1; then
    ready=true
    break
  fi
  sleep 2
  attempt=$((attempt + 1))
done
if [ "$ready" != "true" ]; then rollback "service health checks did not recover"; fi

post_height="$(height || true)"
if [ -z "$post_height" ]; then rollback "chain height is unavailable after deploy"; fi
if [ "$post_height" -lt "$pre_height" ]; then rollback "chain height regressed from $pre_height to $post_height"; fi
mining_enabled="$(env_value MINING_ENABLED "$ENV_FILE" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')"
if [ -z "$mining_enabled" ]; then mining_enabled=true; fi
if [ "$VERIFY_HEIGHT_ADVANCE" = "true" ] && [ "$mining_enabled" = "true" ]; then
  advance_attempt=1
  while [ "$advance_attempt" -le 15 ] && [ "$post_height" -le "$pre_height" ]; do
    sleep 2
    post_height="$(height || true)"
    advance_attempt=$((advance_attempt + 1))
  done
  if [ -z "$post_height" ] || [ "$post_height" -le "$pre_height" ]; then
    rollback "chain process is healthy but finality did not advance beyond $pre_height"
  fi
fi

readiness="$(curl -fsS "$CHAIN_URL/readiness/mainnet" 2>/dev/null || true)"
deployed_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
umask 077
{
  echo "DEPLOYED_AT=$deployed_at"
  echo "LQD_IMAGE=$NEW_IMAGE"
  echo "BACKUP_FILE=$backup_file"
  echo "PRE_HEIGHT=$pre_height"
  echo "POST_HEIGHT=$post_height"
  echo "SIGNER_ENABLED=$SIGNER_ENABLED"
} > "$PODL_ROOT/last-deploy.env"

if [ "$MIGRATION_PENDING" = "true" ]; then rm -f "$PENDING_FILE"; fi

docker image prune -f >/dev/null 2>&1 || true
echo "Safe image deploy completed."
echo "Image: $NEW_IMAGE"
echo "Backup: $backup_file"
echo "Height: $pre_height -> $post_height"
if [ -n "$readiness" ]; then echo "Readiness: $readiness"; fi
