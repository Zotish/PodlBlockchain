# @podl/sdk (testnet)

Zero-dependency JavaScript client for the versioned PoDL chain API. It never
stores private keys; sign transactions in a wallet, hardware wallet or secure
keystore and submit only the signed payload.

```js
import { PoDLClient } from "@podl/sdk";

const podl = new PoDLClient("http://127.0.0.1:6500");
console.log(await podl.protocolStatus());
console.log(await podl.swapQuote({ amountIn: "1000000", tokenIn: "lqd", tokenOut: token, factory }));
```

The typed API also exposes suitability screening, faucet requests, bounded 1–8-hop route
discovery, vault accounting/withdrawal receipts, and signed oracle/governance
transaction submission. Run `npm test` before publishing. Private keys remain
outside the SDK by design.

Concentrated-liquidity helpers cover the transferable position lifecycle:
mint, read, transfer, fee collection and burn. The explorer's generic contract
interface can invoke the same methods without a protocol-specific backend.
