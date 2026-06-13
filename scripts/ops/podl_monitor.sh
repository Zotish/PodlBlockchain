#!/usr/bin/env sh
set -eu

PODL_ROOT="${PODL_ROOT:-/opt/podl}"
COMPOSE="${COMPOSE:-docker compose}"
CHAIN_URL="${CHAIN_URL:-http://127.0.0.1:6500}"
WALLET_URL="${WALLET_URL:-http://127.0.0.1:8080}"
AGG_URL="${AGG_URL:-http://127.0.0.1:9000}"
DEX_API_URL="${DEX_API_URL:-http://127.0.0.1:9100}"
MONITOR_RESTART_UNHEALTHY="${MONITOR_RESTART_UNHEALTHY:-true}"
MONITOR_WEBHOOK_URL="${MONITOR_WEBHOOK_URL:-}"
LOG_FILE="${LOG_FILE:-$PODL_ROOT/logs/podl-monitor.log}"

mkdir -p "$(dirname "$LOG_FILE")"

log() {
  line="$(date -u +%Y-%m-%dT%H:%M:%SZ) $*"
  echo "$line" | tee -a "$LOG_FILE"
}

alert() {
  msg="$*"
  log "$msg"
  if [ -n "$MONITOR_WEBHOOK_URL" ]; then
    curl -fsS -X POST -H 'Content-Type: application/json' \
      --data "{\"text\":\"$msg\"}" "$MONITOR_WEBHOOK_URL" >/dev/null 2>&1 || true
  fi
}

check_url() {
  name="$1"
  url="$2"
  if curl -fsS "$url" >/dev/null 2>&1; then
    log "OK $name $url"
    return 0
  fi
  alert "FAIL $name $url"
  return 1
}

cd "$PODL_ROOT"

failed=0
check_url chain "$CHAIN_URL/health" || failed=1
check_url wallet "$WALLET_URL/health" || failed=1
check_url aggregator "$AGG_URL/health" || failed=1
check_url dex-api "$DEX_API_URL/health" || failed=1

unhealthy="$($COMPOSE ps --format '{{.Service}} {{.State}} {{.Health}}' 2>/dev/null | awk '$3=="unhealthy"{print $1}' || true)"
if [ -n "$unhealthy" ]; then
  alert "Docker reports unhealthy services: $unhealthy"
  failed=1
fi

if [ "$failed" -ne 0 ] && [ "$MONITOR_RESTART_UNHEALTHY" = "true" ]; then
  alert "Restarting unhealthy PODL stack services"
  $COMPOSE up -d --remove-orphans >/dev/null
  sleep 8
  $COMPOSE ps | tee -a "$LOG_FILE"
fi

exit "$failed"
