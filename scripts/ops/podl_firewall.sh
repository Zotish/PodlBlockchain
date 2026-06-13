#!/usr/bin/env sh
set -eu

SSH_PORT="${SSH_PORT:-22}"
P2P_PORT="${P2P_PORT:-6100}"
ALLOW_CADDY="${ALLOW_CADDY:-true}"

if [ "$(id -u)" -ne 0 ]; then
  echo "Run as root: sudo sh scripts/ops/podl_firewall.sh" >&2
  exit 1
fi

if ! command -v ufw >/dev/null 2>&1; then
  apt-get update
  apt-get install -y ufw
fi

ufw --force reset
ufw default deny incoming
ufw default allow outgoing

ufw allow "$SSH_PORT"/tcp comment "SSH"
ufw allow "$P2P_PORT"/tcp comment "PODL P2P"

if [ "$ALLOW_CADDY" = "true" ]; then
  ufw allow 80/tcp comment "HTTP ACME"
  ufw allow 443/tcp comment "HTTPS"
fi

# Chain, wallet, aggregator, and DEX API should bind to 127.0.0.1 behind Caddy.
ufw deny 6500/tcp comment "Block public chain HTTP"
ufw deny 8080/tcp comment "Block public wallet HTTP"
ufw deny 9000/tcp comment "Block public aggregator HTTP"
ufw deny 9100/tcp comment "Block public dex-api HTTP"

ufw --force enable
ufw status verbose
