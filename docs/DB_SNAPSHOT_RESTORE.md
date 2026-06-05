# DB Snapshot And Restore Runbook

The source of truth on the VPS is `/opt/podl/data`. Keep this directory mounted into the containers at `/app/data`. Backend code updates should never delete this directory unless you intentionally want a fresh testnet reset.

## Backup Before A Risky Deploy

```bash
cd /opt/podl
mkdir -p /opt/podl/backups
docker compose stop chain wallet aggregator
tar -C /opt/podl -czf /opt/podl/backups/podl-data-$(date +%Y%m%d-%H%M%S).tar.gz data
docker compose up -d
curl -sS http://127.0.0.1:6500/health
```

## Safe Deploy Flow

```bash
cd /opt/podl/app
git pull --ff-only origin main
cd /opt/podl
docker compose up -d --build --remove-orphans
curl -sS http://127.0.0.1:6500/readiness/mainnet
curl -sS http://127.0.0.1:6500/rewards/summary
```

If height decreases, contracts disappear, or balances change unexpectedly, stop and restore the previous backup.

## Restore From Backup

Replace `YYYYMMDD-HHMMSS` with the backup timestamp.

```bash
cd /opt/podl
docker compose down
mv data data.broken-$(date +%Y%m%d-%H%M%S)
tar -C /opt/podl -xzf /opt/podl/backups/podl-data-YYYYMMDD-HHMMSS.tar.gz
docker compose up -d
curl -sS http://127.0.0.1:6500/health
curl -sS http://127.0.0.1:6500/readiness/mainnet
```

## Fresh Testnet Reset

Only use this when you intentionally want to erase all blocks, balances, contracts, validators, bridge state, and reward history.

```bash
cd /opt/podl
docker compose down
mv data data.reset-$(date +%Y%m%d-%H%M%S)
mkdir -p data
docker compose up -d --build --remove-orphans
curl -sS http://127.0.0.1:6500/health
```

## Quick DB Safety Checks

```bash
cd /opt/podl
docker inspect podl-chain-1 --format '{{json .Mounts}}'
grep -n "CHAIN_DB_PATH\|LQD_DATA_DIR\|LQD_BRIDGE_DATA_DIR" /opt/podl/.env
du -sh /opt/podl/data /opt/podl/data/chain /opt/podl/data/chain/evodb 2>/dev/null
```

Expected values:

- `CHAIN_DB_PATH=/app/data/chain/evodb`
- `LQD_DATA_DIR=/app/data`
- `LQD_BRIDGE_DATA_DIR=/app/data`
- Docker mount source should be `/opt/podl/data` and destination should be `/app/data`.
