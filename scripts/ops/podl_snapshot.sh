#!/usr/bin/env sh
set -eu

PODL_ROOT="${PODL_ROOT:-/opt/podl}"
DATA_DIR="${DATA_DIR:-$PODL_ROOT/data}"
BACKUP_ROOT="${BACKUP_ROOT:-$PODL_ROOT/backups}"
COMPOSE="${COMPOSE:-docker compose}"
STOP_SERVICES="${PODL_SNAPSHOT_STOP:-true}"
RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-14}"

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
  $COMPOSE stop chain wallet aggregator dex-api >/dev/null 2>&1 || true
  stopped=true
fi

tar -C "$PODL_ROOT" -czf "$BACKUP_FILE" "$(basename "$DATA_DIR")"
sha256sum "$BACKUP_FILE" > "$BACKUP_FILE.sha256" 2>/dev/null || shasum -a 256 "$BACKUP_FILE" > "$BACKUP_FILE.sha256"

if [ "$stopped" = "true" ]; then
  $COMPOSE up -d chain wallet aggregator dex-api >/dev/null
fi

if [ "$RETENTION_DAYS" -gt 0 ] 2>/dev/null; then
  find "$BACKUP_ROOT" -type f \( -name 'podl-data-*.tar.gz' -o -name 'podl-data-*.tar.gz.sha256' \) -mtime +"$RETENTION_DAYS" -delete 2>/dev/null || true
fi

echo "$BACKUP_FILE"
