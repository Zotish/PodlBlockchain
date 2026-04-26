# LQD Wallet Extension (MV3)

## What it is
A browser extension wallet for the LQD chain. It exposes a `window.lqd` provider to dapps and supports:
- connect / accounts
- chain id
- balance + token balance
- contract calls
- contract tx (via wallet server)
- native send (via wallet server)
- dapp approvals (connect + tx require user approval)
- token watchlist in popup
- network presets (production chain / aggregator)
- dapp allowlist with per-scope permissions
- tx preview in approval list
- backup export (encrypted JSON)

## Load in browser
### Chrome / Edge
1. Open `chrome://extensions`
2. Enable **Developer mode**
3. Click **Load unpacked**
4. Select the `lqd-wallet-extension/` folder

### Firefox (MV3 support is still evolving)
- Use `about:debugging` → This Firefox → Load Temporary Add‑on
- Select `lqd-wallet-extension/manifest.json`

## Endpoints
Default endpoints (editable in popup):
- Chain: `https://dazzling-peace-production-3529.up.railway.app`
- Wallet: `https://enchanting-hope-production-1c63.up.railway.app`
- Aggregator: `https://keen-enjoyment-production-0440.up.railway.app`

## Dapp API
Injected provider:
```js
window.lqd.request({ method: "lqd_connect" })
window.lqd.request({ method: "lqd_accounts" })
window.lqd.request({ method: "lqd_chainId" })
window.lqd.request({ method: "lqd_getBalance", params: [address] })
window.lqd.request({ method: "lqd_getTokenBalance", params: [token, address] })
window.lqd.request({ method: "lqd_contractCall", params: [{ address, caller, fn, args, value }] })
window.lqd.request({ method: "lqd_contractTx", params: [{ contract_address, function, args, value, gas, gas_price }] })
window.lqd.request({ method: "lqd_sendTransaction", params: [{ to, value, gas, gas_price }] })
```

## Security
Private keys are encrypted with a password (AES‑GCM + PBKDF2) in extension storage. The key is only decrypted in memory after unlock.
