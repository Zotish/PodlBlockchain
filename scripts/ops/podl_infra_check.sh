#!/usr/bin/env sh
set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
fail=0

check_file() {
  if [ -f "$ROOT/$1" ]; then
    echo "PASS file $1"
  else
    echo "FAIL missing $1" >&2
    fail=1
  fi
}

check_grep() {
  name="$1"
  pattern="$2"
  file="$3"
  if grep -Eq "$pattern" "$ROOT/$file"; then
    echo "PASS $name"
  else
    echo "FAIL $name" >&2
    fail=1
  fi
}

check_file deploy/vps/docker-compose.yml
check_file deploy/vps/Caddyfile
check_file deploy/vps/systemd/podl-stack.service
check_file deploy/vps/systemd/podl-backup.timer
check_file deploy/vps/systemd/podl-monitor.timer
check_file scripts/ops/podl_snapshot.sh
check_file scripts/ops/podl_restore.sh
check_file scripts/ops/podl_monitor.sh
check_file scripts/ops/podl_firewall.sh
check_file scripts/ops/podl_safe_deploy.sh

check_grep "compose healthchecks" "healthcheck:" deploy/vps/docker-compose.yml
check_grep "dex api service" "dex-api:" deploy/vps/docker-compose.yml
check_grep "caddy dex route" "DEX_API_DOMAIN" deploy/vps/Caddyfile
check_grep "backup retention" "BACKUP_RETENTION_DAYS|RETENTION_DAYS" scripts/ops/podl_snapshot.sh
check_grep "backup checksum" "sha256|shasum" scripts/ops/podl_snapshot.sh
check_grep "restore checksum" "sha256sum -c|checksum mismatch" scripts/ops/podl_restore.sh
check_grep "monitor restart" "MONITOR_RESTART_UNHEALTHY" scripts/ops/podl_monitor.sh
check_grep "firewall denies public APIs" "deny 6500/tcp" scripts/ops/podl_firewall.sh
check_grep "safe deploy height guard" "height regressed" scripts/ops/podl_safe_deploy.sh

exit "$fail"
