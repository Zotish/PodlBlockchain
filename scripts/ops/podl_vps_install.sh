#!/usr/bin/env sh
set -eu

PODL_ROOT="${PODL_ROOT:-/opt/podl}"
APP_ROOT="${APP_ROOT:-$PODL_ROOT/app}"
REPO_URL="${REPO_URL:-https://github.com/Zotish/PodlBlockchain.git}"
BRANCH="${BRANCH:-main}"

if [ "$(id -u)" -ne 0 ]; then
  echo "Run as root: sudo PODL_ROOT=/opt/podl sh scripts/ops/podl_vps_install.sh" >&2
  exit 1
fi

apt-get update
apt-get install -y ca-certificates curl git docker.io docker-compose-plugin ufw
systemctl enable --now docker

mkdir -p "$PODL_ROOT" "$PODL_ROOT/data" "$PODL_ROOT/backups" "$PODL_ROOT/logs"

if [ ! -d "$APP_ROOT/.git" ]; then
  git clone --branch "$BRANCH" "$REPO_URL" "$APP_ROOT"
else
  git -C "$APP_ROOT" fetch origin "$BRANCH"
  git -C "$APP_ROOT" checkout "$BRANCH"
  git -C "$APP_ROOT" pull --ff-only origin "$BRANCH"
fi

cp "$APP_ROOT/deploy/vps/docker-compose.yml" "$PODL_ROOT/docker-compose.yml"
cp "$APP_ROOT/deploy/vps/Caddyfile" "$PODL_ROOT/Caddyfile"
if [ ! -f "$PODL_ROOT/.env" ]; then
  cp "$APP_ROOT/deploy/vps/.env.example" "$PODL_ROOT/.env"
  echo "Created $PODL_ROOT/.env. Edit validator address, domains, and secrets before starting."
fi

cp "$APP_ROOT/deploy/vps/systemd/"*.service /etc/systemd/system/
cp "$APP_ROOT/deploy/vps/systemd/"*.timer /etc/systemd/system/
systemctl daemon-reload
systemctl enable podl-stack.service podl-backup.timer podl-monitor.timer

echo "Install complete."
echo "Next:"
echo "  nano $PODL_ROOT/.env"
echo "  systemctl start podl-stack"
echo "  systemctl start podl-backup.timer podl-monitor.timer"
