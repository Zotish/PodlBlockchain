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
MONITOR_MAX_LATENCY_MS="${MONITOR_MAX_LATENCY_MS:-2000}"
MONITOR_FAILURE_THRESHOLD="${MONITOR_FAILURE_THRESHOLD:-3}"
LOG_FILE="${LOG_FILE:-$PODL_ROOT/logs/podl-monitor.log}"
STATE_FILE="${STATE_FILE:-$PODL_ROOT/logs/podl-monitor.state}"

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
	result="$(curl -fsS -o /dev/null -w '%{http_code} %{time_total}' "$url" 2>/dev/null || true)"
	code="${result%% *}"
	seconds="${result#* }"
	latency_ms="$(awk -v seconds="$seconds" 'BEGIN { printf "%d", seconds * 1000 }' 2>/dev/null || echo 999999)"
	if [ "$code" = "200" ] && [ "$latency_ms" -le "$MONITOR_MAX_LATENCY_MS" ]; then
		log "OK $name status=$code latency_ms=$latency_ms url=$url"
		return 0
	fi
	alert "FAIL $name status=${code:-unreachable} latency_ms=$latency_ms slo_ms=$MONITOR_MAX_LATENCY_MS url=$url"
  return 1
}

cd "$PODL_ROOT"

failed=0
check_url chain "$CHAIN_URL/health" || failed=1
check_url explorer-index "$CHAIN_URL/v2/index/status" || failed=1
check_url wallet "$WALLET_URL/health" || failed=1
check_url aggregator "$AGG_URL/health" || failed=1
check_url dex-api "$DEX_API_URL/health" || failed=1

unhealthy="$($COMPOSE ps --format '{{.Service}} {{.State}} {{.Health}}' 2>/dev/null | awk '$3=="unhealthy"{print $1}' || true)"
if [ -n "$unhealthy" ]; then
  alert "Docker reports unhealthy services: $unhealthy"
  failed=1
fi

streak=0
if [ -f "$STATE_FILE" ]; then streak="$(sed -n 's/^failure_streak=//p' "$STATE_FILE" | head -1)"; fi
case "$streak" in ''|*[!0-9]*) streak=0 ;; esac
if [ "$failed" -ne 0 ]; then streak=$((streak + 1)); else streak=0; fi
printf 'failure_streak=%s\nlast_check=%s\n' "$streak" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" > "$STATE_FILE"

if [ "$failed" -ne 0 ] && [ "$MONITOR_RESTART_UNHEALTHY" = "true" ] && [ "$streak" -ge "$MONITOR_FAILURE_THRESHOLD" ]; then
	alert "Restarting PODL stack after $streak consecutive failed SLO checks"
	$COMPOSE up -d --remove-orphans >/dev/null
	sleep 8
	$COMPOSE ps | tee -a "$LOG_FILE"
fi

exit "$failed"
