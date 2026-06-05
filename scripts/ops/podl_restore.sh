#!/usr/bin/env sh
set -eu

if [ "${1:-}" = "" ]; then
  echo "Usage: PODL_ROOT=/opt/podl sh scripts/ops/podl_restore.sh /opt/podl/backups/podl-data-YYYYMMDD-HHMMSS.tar.gz" >&2
  exit 1
fi

PODL_ROOT="${PODL_ROOT:-/opt/podl}"
DATA_DIR="${DATA_DIR:-$PODL_ROOT/data}"
COMPOSE="${COMPOSE:-docker compose}"
BACKUP_FILE="$1"
STAMP="$(date -u +%Y%m%d-%H%M%S)"

if [ ! -f "$BACKUP_FILE" ]; then
  echo "Backup file not found: $BACKUP_FILE" >&2
  exit 1
fi

cd "$PODL_ROOT"
$COMPOSE down >/dev/null 2>&1 || true

if [ -d "$DATA_DIR" ]; then
  mv "$DATA_DIR" "$DATA_DIR.broken-$STAMP"
fi

tar -C "$PODL_ROOT" -xzf "$BACKUP_FILE"
$COMPOSE up -d --remove-orphans >/dev/null

echo "Restored $BACKUP_FILE"
echo "Previous data kept at $DATA_DIR.broken-$STAMP if it existed."
