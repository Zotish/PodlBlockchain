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

if [ -d "$APP_ROOT/.git" ]; then
  cd "$APP_ROOT"
  git pull --ff-only origin "$BRANCH"
fi

cp "$APP_ROOT/deploy/vps/docker-compose.yml" "$PODL_ROOT/docker-compose.yml"
cp "$APP_ROOT/deploy/vps/Caddyfile" "$PODL_ROOT/Caddyfile"

cd "$PODL_ROOT"
docker build -t "${LQD_IMAGE:-podl-blockchain:local}" "$APP_ROOT" >/dev/null
$COMPOSE up -d --no-deps dex-api wallet aggregator caddy >/dev/null
$COMPOSE up -d --no-deps chain >/dev/null
$COMPOSE up -d --remove-orphans >/dev/null

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
  echo "Deploy failed: chain health did not recover. Backup: $backup_file" >&2
  if [ "$RESTORE_ON_FAIL" = "true" ]; then
    PODL_ROOT="$PODL_ROOT" sh "$SCRIPT_DIR/podl_restore.sh" "$backup_file"
  fi
  exit 1
fi

post_height="$(must_height)"
if [ "$post_height" -lt "$pre_height" ]; then
  echo "Deploy failed: chain height regressed from $pre_height to $post_height. Backup: $backup_file" >&2
  if [ "$RESTORE_ON_FAIL" = "true" ]; then
    PODL_ROOT="$PODL_ROOT" sh "$SCRIPT_DIR/podl_restore.sh" "$backup_file"
  fi
  exit 1
fi

readiness="$(curl -fsS "$CHAIN_URL/readiness/mainnet" 2>/dev/null || true)"
case "$readiness" in
  *'"status":"action_required"'*)
    echo "Deploy warning: readiness endpoint reports action_required." >&2
    echo "$readiness" >&2
    ;;
esac

echo "Safe deploy completed."
echo "Backup: $backup_file"
echo "Height: $pre_height -> $post_height"
