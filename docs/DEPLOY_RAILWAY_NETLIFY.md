# Railway + Netlify Deployment Guide

This guide prepares the project for:

- Railway backend services:
  - `chain`
  - `wallet`
  - `aggregator`
- Netlify frontend sites:
  - `blockchain-explorer`
  - `swap-dex`
  - `bridge-admin-ui`

Extension and mobile wallet are intentionally left for a later phase.

## 1. Before You Start

You need:

- a GitHub repo connected to Railway and Netlify
- a Railway project
- three Netlify sites
- a strong `LQD_API_KEY`

Official docs used for this setup:

- Railway monorepo deploy: <https://docs.railway.com/guides/monorepo>
- Railway start commands: <https://docs.railway.com/guides/start-command>
- Railway volumes: <https://docs.railway.com/deploy/volumes>
- Railway variables and reference variables: <https://docs.railway.com/variables>
- Railway public/private domains: <https://docs.railway.com/networking/domains/working-with-domains>
- Netlify monorepos: <https://docs.netlify.com/build/configure-builds/monorepos/>
- Netlify environment variables: <https://docs.netlify.com/build/environment-variables/overview>
- Netlify SPA redirects/rewrites: <https://docs.netlify.com/routing/redirects/>

## 2. GitHub Push Flow

From the repo root:

```bash
git checkout -b codex/railway-netlify-deploy
git add \
  .env.example \
  docs/DEPLOY_RAILWAY_NETLIFY.md \
  scripts/railway \
  BlockchainComponent/data_paths.go \
  BlockchainComponent/bridge_tokens.go \
  BlockchainComponent/bridge_relayer_state.go \
  BlockchainComponent/lqd_contract_engine.go \
  BlockchainComponent/bridge_deploy.go \
  BlockchainServer/blockchain_server.go \
  blockchain-explorer/netlify.toml \
  blockchain-explorer/public/_redirects \
  blockchain-explorer/src/utils/api.js \
  blockchain-explorer/src/components/DebugComponent.js \
  blockchain-explorer/src/components/useValidators.js \
  blockchain-explorer/src/components/contracts/ContractABI.js \
  blockchain-explorer/src/components/contracts/ContractCode.js \
  blockchain-explorer/src/components/contracts/ContractCompiler.js \
  blockchain-explorer/src/components/contracts/ContractEvents.js \
  blockchain-explorer/src/components/contracts/ContractOverview.js \
  blockchain-explorer/src/components/contracts/ContractRead.js \
  blockchain-explorer/src/components/contracts/ContractStorage.js \
  blockchain-explorer/src/components/contracts/ContractWrite.js \
  blockchain-explorer/src/components/contracts/CallContract.js \
  blockchain-explorer/src/components/contracts/DeployContract.js \
  blockchain-explorer/src/components/wallet/LiquidityDashboard.js \
  blockchain-explorer/src/components/wallet/SendTransaction.js \
  blockchain-explorer/src/components/wallet/WalletBalance.js \
  blockchain-explorer/src/components/wallet/WalletLogin.js \
  swap-dex/netlify.toml \
  swap-dex/public/_redirects \
  swap-dex/src/config.js \
  bridge-admin-ui/app.js \
  bridge-admin-ui/build.mjs \
  bridge-admin-ui/index.html \
  bridge-admin-ui/netlify.toml \
  bridge-admin-ui/package.json \
  bridge-admin-ui/runtime-config.js

git commit -m "Add Railway and Netlify deployment setup"
git push -u origin codex/railway-netlify-deploy
```

After you are happy, merge this branch into your deployment branch or `main`.

## 3. Railway Setup

### 3.1 Create Services

Create a Railway project from this GitHub repo, then create these 3 services from the same repo:

1. `chain`
2. `wallet`
3. `aggregator`

Because this is a monorepo-style repo, each Railway service should use the same repository but a different **start command**.

### 3.2 Root Directory

Set the Root Directory to `/` for all three Railway services.

### 3.3 Build Command

Use this for all three Railway services:

```bash
sh scripts/railway/build.sh
```

### 3.4 Start Commands

For the `chain` service:

```bash
sh scripts/railway/start-chain.sh
```

For the `wallet` service:

```bash
sh scripts/railway/start-wallet.sh
```

For the `aggregator` service:

```bash
sh scripts/railway/start-aggregator.sh
```

### 3.5 Attach a Volume

Attach a Railway Volume to the `chain` service.

Recommended mount path:

```text
/app/data
```

This matches Railway’s volume guide for apps that write to a relative `./data` path.

### 3.6 Service Naming

Name the Railway services exactly:

- `chain`
- `wallet`
- `aggregator`

That makes Railway reference variables easier to use.

### 3.7 Shared Variables

In Railway Project Settings -> Shared Variables, set:

```text
LQD_API_KEY=your-secret
LQD_ALLOWED_ORIGINS=https://your-explorer.netlify.app,https://your-swap.netlify.app,https://your-bridge-admin.netlify.app
BSC_TESTNET_RPC=...
BSC_TESTNET_RPCS=...
BSC_TESTNET_PRIVATE_KEY=...
BSC_BRIDGE_ADDRESS=...
BSC_LOCK_ADDRESS=...
BSC_CHAIN_ID=97
BRIDGE_POLL_INTERVAL_SEC=5
BRIDGE_BACKFILL_BLOCKS=200
BRIDGE_MAX_RANGE=40000
BRIDGE_PRIVATE_BATCH_SIZE=3
BRIDGE_PRIVATE_BATCH_MAX_SIZE=8
BRIDGE_PRIVATE_BATCH_WAIT_SEC=45
```

### 3.8 Chain Service Variables

On the `chain` service set:

```text
VALIDATOR_ADDRESS=0xYourValidatorAddress
STAKE_AMOUNT=3000000
MIN_STAKE=100000
MINING_ENABLED=true
P2P_PORT=6100
CHAIN_DB_PATH=/app/data/chain/evodb
LQD_DATA_DIR=/app/data
LQD_BRIDGE_DATA_DIR=/app/data
BRIDGE_STATE_FILE=/app/data/bridge_relayer_state.json
```

If you want PosDL validator registration:

```text
DEX_ADDRESS=0xYourDexAddress
LP_TOKEN_AMOUNT=12345
```

### 3.9 Wallet Service Variables

On the `wallet` service set:

```text
CHAIN_URL=http://${{chain.RAILWAY_PRIVATE_DOMAIN}}:${{chain.PORT}}
```

### 3.10 Aggregator Service Variables

On the `aggregator` service set:

```text
CHAIN_URL=http://${{chain.RAILWAY_PRIVATE_DOMAIN}}:${{chain.PORT}}
WALLET_URL=http://${{wallet.RAILWAY_PRIVATE_DOMAIN}}:${{wallet.PORT}}
AGGREGATOR_NODES=auto
```

### 3.11 Generate Public Domains

Generate public domains for:

- `chain`
- `wallet`
- `aggregator`

You will use:

- `aggregator` public domain in the Explorer and Swap DEX frontends
- `chain` public domain in the Bridge Admin UI
- `chain` and `wallet` public domains in the Explorer for direct chain/wallet calls

## 4. Netlify Setup

Create 3 separate Netlify sites from the same GitHub repo.

### 4.1 Explorer

Site config:

- Base directory: `blockchain-explorer`
- Build command: `npm run build`
- Publish directory: `build`

Environment variables:

```text
REACT_APP_API_BASE=https://your-aggregator-public-domain
REACT_APP_CHAIN_BASE=https://your-chain-public-domain
REACT_APP_WALLET_BASE=https://your-wallet-public-domain
REACT_APP_WEB_WALLET_BASE=https://your-explorer-domain
```

### 4.2 Swap DEX

Site config:

- Base directory: `swap-dex`
- Build command: `npm run build`
- Publish directory: `build`

Environment variables:

```text
REACT_APP_NODE_URL=https://your-aggregator-public-domain
REACT_APP_WALLET_URL=https://your-wallet-public-domain
REACT_APP_WEB_WALLET_URL=https://your-explorer-domain
REACT_APP_DEX_CONTRACT_ADDRESS=0xYourDexContractAddress
```

### 4.3 Bridge Admin Console

Site config:

- Base directory: `bridge-admin-ui`
- Build command: `npm run build`
- Publish directory: `dist`

Environment variables:

```text
BRIDGE_ADMIN_NODE_URL=https://your-chain-public-domain
```

## 5. Deployment Order

Deploy in this order:

1. Push branch to GitHub
2. Deploy Railway `chain`
3. Deploy Railway `wallet`
4. Deploy Railway `aggregator`
5. Generate Railway public domains
6. Add/update Netlify environment variables
7. Deploy Netlify Explorer
8. Deploy Netlify Swap DEX
9. Deploy Netlify Bridge Admin

## 6. Post-Deploy Health Checks

### Backend

Check:

```bash
curl https://your-chain-public-domain/getheight
curl https://your-chain-public-domain/network
curl https://your-chain-public-domain/validators
curl https://your-chain-public-domain/bridge/chains
curl https://your-chain-public-domain/bridge/families
curl https://your-chain-public-domain/bridge/tokens

curl https://your-aggregator-public-domain/network
curl https://your-wallet-public-domain/wallet/new -X POST -H 'Content-Type: application/json' -d '{"password":"ExamplePass123!"}'
```

### Frontend

Verify:

- Explorer loads and wallet create/import works
- Swap DEX loads and uses the aggregator URL
- Bridge admin console loads and can read bridge families/chains

## 7. Notes

- Railway frontends should use **public** domains, not private Railway domains.
- Railway service-to-service backend communication should use **private** domains through reference variables.
- For this first testnet setup, Railway is acceptable.
- For a later multi-validator or stricter production rollout, a dedicated VPS/bare metal setup will be more stable for validator/P2P traffic.
