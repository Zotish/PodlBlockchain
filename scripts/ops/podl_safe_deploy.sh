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
IMAGE_NAME="${LQD_IMAGE:-podl-blockchain:local}"
ROLLBACK_IMAGE="${IMAGE_NAME}-rollback"

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
    $COMPOSE up -d --force-recreate --remove-orphans >/dev/null 2>&1 || true
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
if ! $COMPOSE up -d --no-deps dex-api wallet aggregator >/dev/null; then
  rollback_deploy "supporting services failed to start"
fi
if ! $COMPOSE up -d --no-deps chain >/dev/null; then
  rollback_deploy "chain failed to start"
fi
if ! $COMPOSE up -d --remove-orphans >/dev/null; then
  rollback_deploy "stack reconciliation failed"
fi

enable_caddy="$(sed -n 's/^ENABLE_CADDY=//p' "$PODL_ROOT/.env" | tail -1 | tr '[:upper:]' '[:lower:]')"
if [ "$enable_caddy" = "true" ]; then
  if ! $COMPOSE --profile proxy up -d caddy >/dev/null; then
    rollback_deploy "Caddy failed to start"
  fi
else
  $COMPOSE rm -sf caddy >/dev/null 2>&1 || true
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
