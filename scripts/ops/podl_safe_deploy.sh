#!/usr/bin/env sh
set -eu

PODL_ROOT="${PODL_ROOT:-/opt/podl}"
APP_ROOT="${APP_ROOT:-$PODL_ROOT/app}"
CHAIN_URL="${CHAIN_URL:-http://127.0.0.1:6500}"
WALLET_URL="${WALLET_URL:-http://127.0.0.1:8080}"
AGG_URL="${AGG_URL:-http://127.0.0.1:9000}"
DEX_API_URL="${DEX_API_URL:-http://127.0.0.1:9100}"
BRANCH="${BRANCH:-main}"
COMPOSE="${COMPOSE:-docker compose}"
WAIT_ATTEMPTS="${WAIT_ATTEMPTS:-45}"
RESTORE_ON_FAIL="${RESTORE_ON_FAIL:-false}"
RUN_LIVE_E2E_SMOKE="${RUN_LIVE_E2E_SMOKE:-false}"
VERIFY_HEIGHT_ADVANCE="${VERIFY_HEIGHT_ADVANCE:-true}"
IMAGE_NAME="${LQD_IMAGE:-podl-blockchain:local}"
ROLLBACK_IMAGE="${IMAGE_NAME}-rollback"
SIGNER_ENABLED="${ENABLE_VALIDATOR_SIGNER:-}"

if [ -z "$SIGNER_ENABLED" ] && [ -f "$PODL_ROOT/.env" ]; then
  SIGNER_ENABLED="$(sed -n 's/^ENABLE_VALIDATOR_SIGNER=//p' "$PODL_ROOT/.env" | tail -1 | tr '[:upper:]' '[:lower:]')"
fi

compose_run() {
  if [ "$SIGNER_ENABLED" = "true" ]; then
    $COMPOSE --profile signer "$@"
  else
    $COMPOSE "$@"
  fi
}

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"

height() {
  curl -fsS "$CHAIN_URL/getheight" 2>/dev/null | sed -n 's/.*"height":[[:space:]]*\([0-9][0-9]*\).*/\1/p'
}

must_height() {
  h="$(height || true)"
  if [ "$h" = "" ]; then
    echo "Unable to read chain height from $CHAIN_URL/getheight" >&2
    exit 1
  fi
  echo "$h"
}

pre_height="$(must_height)"
backup_file="$(PODL_ROOT="$PODL_ROOT" sh "$SCRIPT_DIR/podl_snapshot.sh")"

previous_image="$(docker image inspect "$IMAGE_NAME" --format '{{.Id}}' 2>/dev/null || true)"
if [ -n "$previous_image" ]; then
  docker tag "$previous_image" "$ROLLBACK_IMAGE"
fi

rollback_deploy() {
  reason="$1"
  echo "Deploy failed: $reason. Backup: $backup_file" >&2
  if [ -n "$previous_image" ]; then
    docker tag "$ROLLBACK_IMAGE" "$IMAGE_NAME"
  fi
  if [ "$RESTORE_ON_FAIL" = "true" ]; then
    PODL_ROOT="$PODL_ROOT" sh "$SCRIPT_DIR/podl_restore.sh" "$backup_file" || true
  elif [ -n "$previous_image" ]; then
    compose_run up -d --force-recreate --remove-orphans >/dev/null 2>&1 || true
  fi
  exit 1
}

if [ -d "$APP_ROOT/.git" ]; then
  cd "$APP_ROOT"
  git pull --ff-only origin "$BRANCH"
fi

cp "$APP_ROOT/deploy/vps/docker-compose.yml" "$PODL_ROOT/docker-compose.yml"
cp "$APP_ROOT/deploy/vps/Caddyfile" "$PODL_ROOT/Caddyfile"

cd "$PODL_ROOT"
if ! docker build -t "$IMAGE_NAME" "$APP_ROOT" >/dev/null; then
  rollback_deploy "image build failed"
fi
required_scripts="/app/scripts/railway/start-chain.sh"
if [ "$SIGNER_ENABLED" = "true" ]; then
  required_scripts="$required_scripts /app/scripts/railway/start-signer.sh"
fi
if ! docker run --rm --entrypoint sh "$IMAGE_NAME" -c "for file in $required_scripts; do test -x \"\$file\" || exit 1; done"; then
  rollback_deploy "runtime image is missing an executable chain/signer startup script"
fi
if [ "$SIGNER_ENABLED" = "true" ]; then
  if ! compose_run up -d --no-deps signer >/dev/null; then
    rollback_deploy "validator signer failed to start"
  fi
  signer_ready=false
  signer_attempt=1
  while [ "$signer_attempt" -le 30 ]; do
    signer_container="$(compose_run ps -q signer 2>/dev/null || true)"
    if [ -n "$signer_container" ] && [ "$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "$signer_container" 2>/dev/null || true)" = "healthy" ]; then
      signer_ready=true
      break
    fi
    sleep 2
    signer_attempt=$((signer_attempt + 1))
  done
  if [ "$signer_ready" != "true" ]; then
    rollback_deploy "validator signer health did not recover"
  fi
fi
if ! compose_run up -d --no-deps dex-api wallet aggregator >/dev/null; then
  rollback_deploy "supporting services failed to start"
fi
if ! compose_run up -d --no-deps chain >/dev/null; then
  rollback_deploy "chain failed to start"
fi
if ! compose_run up -d --remove-orphans >/dev/null; then
  rollback_deploy "stack reconciliation failed"
fi

enable_caddy="$(sed -n 's/^ENABLE_CADDY=//p' "$PODL_ROOT/.env" | tail -1 | tr '[:upper:]' '[:lower:]')"
if [ "$enable_caddy" = "true" ]; then
  if ! compose_run --profile proxy up -d caddy >/dev/null; then
    rollback_deploy "Caddy failed to start"
  fi
else
  compose_run rm -sf caddy >/dev/null 2>&1 || true
fi

ready=false
i=1
while [ "$i" -le "$WAIT_ATTEMPTS" ]; do
  if curl -fsS "$CHAIN_URL/health" >/dev/null 2>&1 \
    && curl -fsS "$WALLET_URL/health" >/dev/null 2>&1 \
    && curl -fsS "$AGG_URL/health" >/dev/null 2>&1 \
    && curl -fsS "$DEX_API_URL/health" >/dev/null 2>&1; then
    ready=true
    break
  fi
  sleep 2
  i=$((i + 1))
done

if [ "$ready" != "true" ]; then
  rollback_deploy "service health did not recover"
fi

post_height="$(must_height)"
if [ "$post_height" -lt "$pre_height" ]; then
  rollback_deploy "chain height regressed from $pre_height to $post_height"
fi
mining_enabled="$(sed -n 's/^MINING_ENABLED=//p' "$PODL_ROOT/.env" | tail -1 | tr '[:upper:]' '[:lower:]')"
if [ -z "$mining_enabled" ]; then mining_enabled=true; fi
if [ "$VERIFY_HEIGHT_ADVANCE" = "true" ] && [ "$mining_enabled" = "true" ]; then
  advance_attempt=1
  while [ "$advance_attempt" -le 15 ] && [ "$post_height" -le "$pre_height" ]; do
    sleep 2
    post_height="$(must_height)"
    advance_attempt=$((advance_attempt + 1))
  done
  if [ "$post_height" -le "$pre_height" ]; then
    rollback_deploy "chain process is healthy but finality did not advance beyond $pre_height"
  fi
fi

readiness="$(curl -fsS "$CHAIN_URL/readiness/mainnet" 2>/dev/null || true)"
case "$readiness" in
  *'"status":"action_required"'*)
    echo "Deploy warning: readiness endpoint reports action_required." >&2
    echo "$readiness" >&2
    ;;
esac

if [ "$RUN_LIVE_E2E_SMOKE" = "true" ]; then
  if ! command -v bash >/dev/null 2>&1; then
    echo "Deploy failed: bash is required for scripts/live_e2e_curl.sh. Backup: $backup_file" >&2
    exit 1
  fi
  if ! command -v jq >/dev/null 2>&1; then
    echo "Deploy failed: jq is required for scripts/live_e2e_curl.sh. Backup: $backup_file" >&2
    exit 1
  fi
  CHAIN="$CHAIN_URL" \
    WALLET="$WALLET_URL" \
    API="$AGG_URL" \
    DEXAPI="$DEX_API_URL" \
    bash "$APP_ROOT/scripts/live_e2e_curl.sh"
fi

echo "Safe deploy completed."
echo "Backup: $backup_file"
echo "Height: $pre_height -> $post_height"
