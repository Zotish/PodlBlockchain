# LQD Swap DEX Registry

Small SQLite-backed registry API for universal DEX config, verified tokens, and approved pools.

This service does not store blockchain state. Blocks, balances, contract storage, LP state, and rewards remain in the chain backend.

## Environment

```env
PORT=9100
DEX_REGISTRY_DB=/opt/podl/swap-dex-registry.sqlite
DEX_REGISTRY_ADMIN_KEY=change-this-long-random-secret
DEX_FACTORY_ADDRESS=0x51d85e8fea15bc1523e83f9fc919c11605abc4ae
DEX_ROUTER_ADDRESS=
LQD_NODE_URL=https://api.178-105-133-94.sslip.io
LQD_WALLET_URL=https://wallet.178-105-133-94.sslip.io
ALLOWED_ORIGINS=https://bright-crisp-91fe94.netlify.app
```

## Run

```bash
npm install
npm start
```

Public endpoints:

```txt
GET /health
GET /config
GET /tokens
GET /pools
```

Admin endpoints require `X-Admin-Key`:

```txt
PUT /config
POST /tokens
DELETE /tokens/:address
POST /pools
DELETE /pools/:address
```
