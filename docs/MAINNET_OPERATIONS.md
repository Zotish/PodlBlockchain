# Mainnet Operations Runbook

This runbook is for production-style testnet operation before mainnet. It does not replace a real audit, but it makes unsafe deploys, DB regressions, and block stalls visible before users are affected.

## Safe Deploy

Run this from the VPS after a GitHub push:

```bash
cd /opt/podl
sh app/scripts/ops/podl_safe_deploy.sh
```

What it does:
- reads the current chain height
- creates a compressed backup of `/opt/podl/data`
- pulls the latest `main` branch from `/opt/podl/app`
- recreates Docker services
- checks `/health`, `/getheight`, `/readiness/mainnet`, and `/rewards/summary`
- fails if height regresses

If you want automatic restore on a failed deploy:

```bash
cd /opt/podl
RESTORE_ON_FAIL=true sh app/scripts/ops/podl_safe_deploy.sh
```

## Manual Snapshot

```bash
cd /opt/podl
sh app/scripts/ops/podl_snapshot.sh
```

The command prints the backup path, for example:

```text
/opt/podl/backups/podl-data-20260605-120000.tar.gz
```

## Manual Restore

```bash
cd /opt/podl
sh app/scripts/ops/podl_restore.sh /opt/podl/backups/podl-data-20260605-120000.tar.gz
```

The old data directory is preserved as `/opt/podl/data.broken-YYYYMMDD-HHMMSS`.

## Block-Stuck Soak Test

Run this after deploys and before public announcements:

```bash
cd /opt/podl
DURATION_SEC=3600 INTERVAL_SEC=30 sh app/scripts/ops/podl_soak_check.sh
```

For a 30-60 day testnet, run it repeatedly with a process manager or cron and save logs:

```bash
cd /opt/podl
DURATION_SEC=86400 INTERVAL_SEC=30 LOG_FILE=/opt/podl/logs/soak-$(date -u +%Y%m%d).csv sh app/scripts/ops/podl_soak_check.sh
```

## Mainnet Evidence Checklist

Before calling the system mainnet-ready, keep evidence for:
- zero DB height regression across upgrades
- snapshot and restore tested on a staging node
- no block stall during repeated 24-hour soak tests
- validator peer list shows only verified, near-tip voting nodes as eligible
- offline validators do not receive voting rewards
- wallet balance, explorer balance, and reward claim state match for the same address
- DEX quote, slippage, deadline, and multi-hop routes pass live pair tests
- strategy vault movement has rollback/minOut protection verified in live tests

## Important Boundary

Code can make the system safer, but "100% mainnet complete" also requires time-based proof: long-running testnet logs, external audit, incident drills, and upgrade rehearsals. Treat this runbook as the operational gate that turns those requirements into repeatable checks.
