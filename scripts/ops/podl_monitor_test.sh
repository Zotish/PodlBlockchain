#!/usr/bin/env sh
set -eu

REPO_ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
TEST_ROOT="$(mktemp -d)"
trap 'rm -rf "$TEST_ROOT"' EXIT HUP INT TERM
MOCK_BIN="$TEST_ROOT/bin"
PODL_ROOT="$TEST_ROOT/podl"
mkdir -p "$MOCK_BIN" "$PODL_ROOT/logs"

printf '%s\n' \
  '#!/bin/sh' \
  'url=""' \
  'for arg in "$@"; do url="$arg"; done' \
  'case "$url" in' \
  '  */getheight) printf "{\"height\":%s}\\n" "${MOCK_HEIGHT:-10}" ;;' \
  '  *)' \
  '    case " $* " in *" -w "*) printf "200 0.001" ;; *) printf "{\"status\":\"ok\"}\\n" ;; esac' \
  '    ;;' \
  'esac' > "$MOCK_BIN/curl"

printf '%s\n' \
  '#!/bin/sh' \
  'if [ "${MOCK_SERVICE_STATE:-running}" = "restarting" ]; then' \
  '  printf "signer restarting \\nchain running healthy\\n"' \
  'else' \
  '  printf "signer running healthy\\nchain running healthy\\n"' \
  'fi' > "$MOCK_BIN/docker"

chmod 700 "$MOCK_BIN/curl" "$MOCK_BIN/docker"
printf '%s\n' 'ENABLE_VALIDATOR_SIGNER=true' > "$PODL_ROOT/.env"

PATH="$MOCK_BIN:$PATH" PODL_ROOT="$PODL_ROOT" MONITOR_RESTART_UNHEALTHY=false MOCK_HEIGHT=10 \
  sh "$REPO_ROOT/scripts/ops/podl_monitor.sh" >/dev/null

grep -q '^last_height=10$' "$PODL_ROOT/logs/podl-monitor.state"

old_epoch=$(( $(date +%s) - 600 ))
printf 'failure_streak=0\nlast_height=10\nlast_height_change=%s\n' "$old_epoch" > "$PODL_ROOT/logs/podl-monitor.state"
if PATH="$MOCK_BIN:$PATH" PODL_ROOT="$PODL_ROOT" MONITOR_RESTART_UNHEALTHY=false MONITOR_MAX_STALLED_SECONDS=120 MOCK_HEIGHT=10 \
  sh "$REPO_ROOT/scripts/ops/podl_monitor.sh" >/dev/null 2>&1; then
  echo "stalled finality should fail monitoring" >&2
  exit 1
fi
grep -q 'FAIL chain finality stalled' "$PODL_ROOT/logs/podl-monitor.log"

PATH="$MOCK_BIN:$PATH" PODL_ROOT="$PODL_ROOT" MONITOR_RESTART_UNHEALTHY=false MOCK_HEIGHT=11 \
  sh "$REPO_ROOT/scripts/ops/podl_monitor.sh" >/dev/null
grep -q '^last_height=11$' "$PODL_ROOT/logs/podl-monitor.state"

if PATH="$MOCK_BIN:$PATH" PODL_ROOT="$PODL_ROOT" MONITOR_RESTART_UNHEALTHY=false MOCK_HEIGHT=12 MOCK_SERVICE_STATE=restarting \
  sh "$REPO_ROOT/scripts/ops/podl_monitor.sh" >/dev/null 2>&1; then
  echo "restarting signer should fail monitoring" >&2
  exit 1
fi
grep -q 'Docker reports non-running services: signer:restarting' "$PODL_ROOT/logs/podl-monitor.log"

echo "PASS monitor detects stalled finality, resumed height and signer crash-loop"
