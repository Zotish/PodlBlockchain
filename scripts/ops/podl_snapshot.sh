#!/usr/bin/env sh
set -eu

PODL_ROOT="${PODL_ROOT:-/opt/podl}"
DATA_DIR="${DATA_DIR:-$PODL_ROOT/data}"
BACKUP_ROOT="${BACKUP_ROOT:-$PODL_ROOT/backups}"
COMPOSE="${COMPOSE:-docker compose}"
STOP_SERVICES="${PODL_SNAPSHOT_STOP:-true}"

STAMP="$(date -u +%Y%m%d-%H%M%S)"
BACKUP_FILE="${1:-$BACKUP_ROOT/podl-data-$STAMP.tar.gz}"

if [ ! -d "$DATA_DIR" ]; then
  echo "Data directory not found: $DATA_DIR" >&2
  exit 1
fi

mkdir -p "$BACKUP_ROOT"
cd "$PODL_ROOT"

stopped=false
if [ "$STOP_SERVICES" = "true" ]; then
  $COMPOSE stop chain wallet aggregator >/dev/null 2>&1 || true
  stopped=true
fi

tar -C "$PODL_ROOT" -czf "$BACKUP_FILE" "$(basename "$DATA_DIR")"

if [ "$stopped" = "true" ]; then
  $COMPOSE up -d chain wallet aggregator >/dev/null
fi

echo "$BACKUP_FILE"
