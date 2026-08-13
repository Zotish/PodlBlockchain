#!/usr/bin/env sh
set -eu

PODL_ROOT="${PODL_ROOT:-/opt/podl}"
COMPOSE="${COMPOSE:-docker compose}"
ACTION="${1:-start}"

cd "$PODL_ROOT"

case "$ACTION" in
  start)
    $COMPOSE up -d --remove-orphans
    enable_caddy="$(sed -n 's/^ENABLE_CADDY=//p' "$PODL_ROOT/.env" | tail -1 | tr '[:upper:]' '[:lower:]')"
    if [ "$enable_caddy" = "true" ]; then
      $COMPOSE --profile proxy up -d caddy
    else
      $COMPOSE rm -sf caddy >/dev/null 2>&1 || true
    fi
    ;;
  stop)
    $COMPOSE --profile proxy stop
    ;;
  *)
    echo "Usage: $0 [start|stop]" >&2
    exit 2
    ;;
esac
