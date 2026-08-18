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
MONITOR_MAX_STALLED_SECONDS="${MONITOR_MAX_STALLED_SECONDS:-120}"
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

compose_status() {
  signer_enabled="$(sed -n 's/^ENABLE_VALIDATOR_SIGNER=//p' "$PODL_ROOT/.env" 2>/dev/null | tail -1 | tr '[:upper:]' '[:lower:]')"
  if [ "$signer_enabled" = "true" ]; then
    $COMPOSE --profile signer ps --format '{{.Service}} {{.State}} {{.Health}}'
  else
    $COMPOSE ps --format '{{.Service}} {{.State}} {{.Health}}'
  fi
}

chain_height() {
  curl -fsS "$CHAIN_URL/getheight" 2>/dev/null | sed -n 's/.*"height":[[:space:]]*\([0-9][0-9]*\).*/\1/p'
}

cd "$PODL_ROOT"

failed=0
check_url chain "$CHAIN_URL/health" || failed=1
check_url explorer-index "$CHAIN_URL/v2/index/status" || failed=1
check_url wallet "$WALLET_URL/health" || failed=1
check_url aggregator "$AGG_URL/health" || failed=1
check_url dex-api "$DEX_API_URL/health" || failed=1

service_status="$(compose_status 2>/dev/null || true)"
unhealthy="$(printf '%s\n' "$service_status" | awk '$3=="unhealthy"{print $1}' || true)"
if [ -n "$unhealthy" ]; then
  alert "Docker reports unhealthy services: $unhealthy"
  failed=1
fi
not_running="$(printf '%s\n' "$service_status" | awk '$2=="restarting" || $2=="exited" || $2=="dead"{print $1 ":" $2}' || true)"
if [ -n "$not_running" ]; then
  alert "Docker reports non-running services: $not_running"
  failed=1
fi

streak=0
previous_height=""
last_height_change=""
if [ -f "$STATE_FILE" ]; then
  streak="$(sed -n 's/^failure_streak=//p' "$STATE_FILE" | head -1)"
  previous_height="$(sed -n 's/^last_height=//p' "$STATE_FILE" | head -1)"
  last_height_change="$(sed -n 's/^last_height_change=//p' "$STATE_FILE" | head -1)"
fi
case "$streak" in ''|*[!0-9]*) streak=0 ;; esac
case "$previous_height" in ''|*[!0-9]*) previous_height="" ;; esac
case "$last_height_change" in ''|*[!0-9]*) last_height_change="" ;; esac

current_height="$(chain_height || true)"
now_epoch="$(date +%s)"
case "$current_height" in
  ''|*[!0-9]*)
    alert "Unable to read canonical chain height"
    failed=1
    current_height="${previous_height:-0}"
    ;;
esac
if [ -z "$last_height_change" ] || [ "$current_height" != "$previous_height" ]; then
  last_height_change="$now_epoch"
  if [ -n "$previous_height" ]; then
    log "PROGRESS chain height=$previous_height->$current_height"
  fi
else
  stalled_seconds=$((now_epoch - last_height_change))
  if [ "$stalled_seconds" -gt "$MONITOR_MAX_STALLED_SECONDS" ]; then
    alert "FAIL chain finality stalled height=$current_height stalled_seconds=$stalled_seconds threshold=$MONITOR_MAX_STALLED_SECONDS"
    failed=1
  fi
fi

if [ "$failed" -ne 0 ]; then streak=$((streak + 1)); else streak=0; fi
printf 'failure_streak=%s\nlast_check=%s\nlast_height=%s\nlast_height_change=%s\n' \
  "$streak" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$current_height" "$last_height_change" > "$STATE_FILE"

if [ "$failed" -ne 0 ] && [ "$MONITOR_RESTART_UNHEALTHY" = "true" ] && [ "$streak" -ge "$MONITOR_FAILURE_THRESHOLD" ]; then
	alert "Restarting PODL stack after $streak consecutive failed SLO checks"
	if [ "$(sed -n 's/^ENABLE_VALIDATOR_SIGNER=//p' "$PODL_ROOT/.env" 2>/dev/null | tail -1 | tr '[:upper:]' '[:lower:]')" = "true" ]; then
		$COMPOSE --profile signer up -d --remove-orphans >/dev/null
	else
		$COMPOSE up -d --remove-orphans >/dev/null
	fi
	sleep 8
	$COMPOSE ps | tee -a "$LOG_FILE"
fi

exit "$failed"
