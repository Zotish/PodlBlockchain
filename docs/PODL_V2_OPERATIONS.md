# PoDL v2 Restricted-Testnet Operations

Do not use real public capital with this configuration.

## Validator launch

Every validator needs a unique data directory, address, matching private key and ports. Use at least four independently controlled nodes.

```sh
export VALIDATOR_ADDRESS=0x...
export VALIDATOR_PRIVATE_KEY=...
export LQD_REQUIRE_SIGNED_BFT=true
export STAKE_AMOUNT=3000000
export MIN_STAKE=100000
export MINING_ENABLED=true
export LQD_DATA_DIR=/absolute/unique/node-data
export PORT=6500
export P2P_PORT=6100
sh scripts/railway/start-chain.sh
```

Never reuse a validator key or data directory.

## Operational endpoints

| Endpoint | Purpose |
|---|---|
| `GET /health` | Liveness and tip |
| `GET /v2/protocol/status` | Consolidated protocol/consensus/liquidity/economics/governance/bridge/product status |
| `GET /readiness/mainnet` | Fail-closed launch gates, not a certificate |
| `GET /metrics` | Prometheus metrics |
| `GET /v2/index/status` | Persistent explorer checkpoint and chain-tip lag |
| `GET /v2/index/search?q=...` | Indexed block/hash/address/transaction search |
| `GET /liquidity/dynamic/status` | Pool routing and quality |
| `POST /contract/call` | Contract view/execution gateway |

## Release checks

```sh
env GOCACHE=/tmp/podl-test-gocache GOMODCACHE=/tmp/podl-test-gomodcache go test ./...
env GOCACHE=/tmp/podl-race-gocache GOMODCACHE=/tmp/podl-test-gomodcache go test -race ./...
env GOCACHE=/tmp/podl-test-gocache GOMODCACHE=/tmp/podl-test-gomodcache go run scripts/railway/precompile_builtins.go
node --check sdk/javascript/src/index.js
./scripts/validate_table1.sh
```

`go vet ./...` is now a release gate. Persistence is pointer-based and speculative block replay uses a serialized deep-copy boundary, so live mutexes are not copied.

## Oracle rules

- Minimum three organizationally independent publishers.
- Observation age <= 900 seconds.
- No organization should control an oracle threshold and validator threshold.
- Every production pool needs valid confidence and no active circuit break.
- Stale data must stop weight increases and vault actions, never silently become trusted.

## Vault rules

- Deploy the `strategy_vault` builtin and set its router.
- Publish fresh `priceUSD18`, decimals and timestamp before any deposit.
- Default maximum movement is 25%; default liquid buffer is 15%.
- Rebalance with min-out, slippage cap and deadline.
- Exit through `RequestWithdrawal` then FIFO `ProcessWithdrawal` for a proportional LP basket.
- Chain-level `/liquidity/vault/*` is a legacy metadata compatibility path, not the custody product.

## Bridge rules

Keep bridge paused or tiny-capped during early testnet. Before a mainnet candidate:

- enable attestation enforcement;
- use at least 3-of-5 independent signers;
- configure per-transaction and UTC daily cap per chain;
- configure source finality/confirmations;
- test replay DB backup/restore;
- allow guardian pause, but governance-only recovery.

## Economic rules

Record only realized external revenue. Never label emissions, treasury transfers or token appreciation as revenue. Keep buyback disabled until insurance is actually funded.

## Mandatory drills

1. proposer offline and view change;
2. conflicting signed vote/evidence/slashing;
3. one-third validator outage;
4. partition and recovery;
5. stale/outlier oracle circuit break;
6. vault slippage rollback and withdrawal burst;
7. bridge replay and cap breach;
8. DB snapshot/loss/restore;
9. API overload/indexer failover.

Only set `LQD_TESTNET_SOAK_COMPLETE=true`, `LQD_RESTORE_DRILL_COMPLETE=true` or `LQD_EXTERNAL_AUDIT_REPORT=...` after real public evidence exists.

## Alerts and on-call

Run `scripts/ops/podl_monitor.sh` at least once per minute from infrastructure independent of the node. It checks chain, persistent index, wallet, aggregator and DEX API latency, stores a consecutive-failure counter, sends a webhook, and restarts only after the configured threshold.

Required deployment variables:

```sh
export MONITOR_WEBHOOK_URL=https://your-pager-receiver.example/...
export MONITOR_MAX_LATENCY_MS=2000
export MONITOR_FAILURE_THRESHOLD=3
export MONITOR_RESTART_UNHEALTHY=true
```

Before marking the mainnet readiness check, separately configure `LQD_ONCALL_ALERT_ENDPOINT` and set `LQD_ONCALL_RUNBOOK_ACK=true` only after a named rotation has acknowledged the escalation path. Run a quarterly drill: stop one service, confirm three consecutive alerts, confirm page receipt, recover, verify index lag returns to zero, and attach timestamps/logs to the incident record.

Do not expose webhook secrets in logs or repository configuration. A configured URL is not proof that a human rotation exists; the acknowledgement and drill evidence remain operational evidence.
