# PODL VPS Mainnet Runbook

This directory contains the Docker Compose, Caddy, systemd, backup, monitoring,
and firewall assets for a single-node PODL mainnet VPS.

## First Install

```sh
sudo REPO_URL=https://github.com/Zotish/PodlBlockchain.git \
  PODL_ROOT=/opt/podl \
  sh /opt/podl/app/scripts/ops/podl_vps_install.sh
```

Edit `/opt/podl/.env`:

- `VALIDATOR_ADDRESS`
- `LQD_API_KEY`
- `DEX_REGISTRY_ADMIN_KEY`
- `CHAIN_DOMAIN`, `WALLET_DOMAIN`, `AGG_DOMAIN`, `DEX_API_DOMAIN`
- `LQD_ALLOWED_ORIGINS`

Then start:

```sh
sudo systemctl start podl-stack
sudo systemctl start podl-backup.timer podl-monitor.timer
sudo systemctl status podl-stack
sudo docker compose -f /opt/podl/docker-compose.yml ps
```

## Safe Update

```sh
cd /opt/podl/app
sudo RESTORE_ON_FAIL=true PODL_ROOT=/opt/podl sh scripts/ops/podl_safe_deploy.sh
```

The safe deploy script creates a backup, rebuilds the local image, starts
services, waits for health endpoints, and fails if chain height regresses.

## Backups

Manual backup:

```sh
sudo PODL_ROOT=/opt/podl sh /opt/podl/app/scripts/ops/podl_snapshot.sh
```

Restore:

```sh
sudo PODL_ROOT=/opt/podl sh /opt/podl/app/scripts/ops/podl_restore.sh /opt/podl/backups/podl-data-YYYYMMDD-HHMMSS.tar.gz
```

The systemd timer `podl-backup.timer` runs every 6 hours and keeps
`BACKUP_RETENTION_DAYS` days.

## Monitoring

```sh
sudo systemctl status podl-monitor.timer
sudo journalctl -u podl-monitor.service -n 100 --no-pager
sudo tail -f /opt/podl/logs/podl-monitor.log
```

Set `MONITOR_WEBHOOK_URL` in `/opt/podl/.env` to receive alerts.

## Firewall

```sh
sudo SSH_PORT=22 P2P_PORT=6100 sh /opt/podl/app/scripts/ops/podl_firewall.sh
```

The firewall allows SSH, 80/443, P2P, and blocks public access to raw API
ports `6500`, `8080`, `9000`, and `9100`. Public API access should go through
Caddy HTTPS domains.
