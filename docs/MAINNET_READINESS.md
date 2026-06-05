# Mainnet Readiness Checklist

This project is configured for a public testnet hardening phase first. Do not treat a green checklist as an audit replacement. Before mainnet, run 30-60 days of public testnet, validator onboarding, backup restore drills, stress tests, and an external security review.

## Live Readiness Endpoints

Run these on the VPS after each deploy:

```bash
curl -sS https://chain.178-105-133-94.sslip.io/health
curl -sS https://chain.178-105-133-94.sslip.io/readiness/mainnet
curl -sS https://chain.178-105-133-94.sslip.io/rewards/summary
curl -sS https://chain.178-105-133-94.sslip.io/validators
curl -sS https://chain.178-105-133-94.sslip.io/peers
```

`/readiness/mainnet` returns critical and warning checks for chain tip, DB persistence, validator capacity, peer handshakes, reward history, mempool pressure, and base-fee floor.

## 30-60 Day Testnet Gates

- Core node: block height must only increase, restarts must keep the same DB, and no deploy should reset contract or account state.
- Snapshot/restore: restore from a backup on a clean VPS and confirm height, balances, contracts, validators, and rewards match.
- Validator onboarding: at least 3 independent validator nodes should sync near-tip, verify handshakes, and become voting eligible.
- Reward consistency: explorer balance, extension balance, mobile balance, reward history, and claim output must match for the same address.
- DEX: pair creation, liquidity add/remove, slippage, deadline, quote, multi-hop routing, and failed transaction handling must pass repeated live tests.
- Dynamic liquidity: vault moves must enforce minOut, slippage, whitelist, cooldown, and rollback behavior on failed routes.
- Explorer performance: dashboard, blocks, txs, address, token, pool, rewards, and contract pages should load quickly on a normal mobile connection.
- Security: private keys, admin keys, relayer keys, registry writes, bridge endpoints, and validator registration must be rate-limited and auditable.
- Docs: public users need guides for adding liquidity, becoming a validator, claiming rewards, deploying tokens, using the bridge, and restoring wallets.

## Mainnet Exit Criteria

- No unexplained block stall for 14 consecutive days.
- No DB reset or block-height rollback during repeated deploys and restarts.
- Successful backup restore drill from the latest production snapshot.
- Validator peer list correctly marks offline, syncing, active, and voting nodes.
- Offline validators receive no voting reward.
- LP reward, validator participant reward, treasury reward, and tx participant reward reconcile per block.
- DEX registry and router/factory config survive frontend/backend redeploys.
- All admin endpoints require a strong secret and produce logs.
- External audit findings are fixed or formally accepted with documented risk.
