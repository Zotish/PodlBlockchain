#!/usr/bin/env sh
set -eu

CHAIN_URL="${CHAIN_URL:-http://127.0.0.1:6500}"
DURATION_SEC="${DURATION_SEC:-3600}"
INTERVAL_SEC="${INTERVAL_SEC:-30}"
STALL_LIMIT="${STALL_LIMIT:-3}"
LOG_FILE="${LOG_FILE:-/tmp/podl-soak-$(date -u +%Y%m%d-%H%M%S).log}"

height() {
  curl -fsS "$CHAIN_URL/getheight" 2>/dev/null | sed -n 's/.*"height":[[:space:]]*\([0-9][0-9]*\).*/\1/p'
}

end_at=$(( $(date +%s) + DURATION_SEC ))
previous=""
stalls=0

echo "timestamp,height,stalls" > "$LOG_FILE"

while [ "$(date +%s)" -lt "$end_at" ]; do
  current="$(height || true)"
  if [ "$current" = "" ]; then
    echo "Health failure: cannot read height from $CHAIN_URL/getheight" >&2
    exit 1
  fi

  if [ "$previous" != "" ] && [ "$current" -le "$previous" ]; then
    stalls=$((stalls + 1))
  else
    stalls=0
  fi

  echo "$(date -u +%Y-%m-%dT%H:%M:%SZ),$current,$stalls" >> "$LOG_FILE"

  if [ "$stalls" -ge "$STALL_LIMIT" ]; then
    echo "Block production stalled at height $current for $stalls checks. Log: $LOG_FILE" >&2
    exit 1
  fi

  previous="$current"
  sleep "$INTERVAL_SEC"
done

curl -fsS "$CHAIN_URL/readiness/mainnet" >/dev/null 2>&1 || {
  echo "Readiness endpoint failed after soak. Log: $LOG_FILE" >&2
  exit 1
}

curl -fsS "$CHAIN_URL/rewards/summary" >/dev/null 2>&1 || {
  echo "Reward summary endpoint failed after soak. Log: $LOG_FILE" >&2
  exit 1
}

echo "Soak check passed. Log: $LOG_FILE"
