export class PoDLClient {
  constructor(baseUrl, { apiKey = "", fetchImpl = globalThis.fetch } = {}) {
    if (!baseUrl || typeof fetchImpl !== "function") throw new Error("baseUrl and fetch are required");
    this.baseUrl = baseUrl.replace(/\/$/, "");
    this.apiKey = apiKey;
    this.fetch = fetchImpl;
  }

  async request(path, options = {}) {
    const headers = { Accept: "application/json", ...(options.body ? { "Content-Type": "application/json" } : {}), ...options.headers };
    if (this.apiKey) headers["X-API-Key"] = this.apiKey;
    const response = await this.fetch(`${this.baseUrl}${path}`, { ...options, headers });
    const text = await response.text();
    let body;
    try { body = text ? JSON.parse(text) : null; } catch { body = { raw: text }; }
    if (!response.ok) throw new PoDLError(response.status, body?.error || body?.message || text || "request failed", body);
    return body;
  }

  protocolStatus() { return this.request("/v2/protocol/status"); }
  mainnetReadiness() { return this.request("/readiness/mainnet"); }
  suitability(answers) { return this.request("/v2/product/suitability", { method: "POST", body: JSON.stringify(answers) }); }
  faucet(address) { return this.request("/faucet", { method: "POST", body: JSON.stringify({ address }) }); }
  balance(address) { return this.request(`/balance?address=${encodeURIComponent(address)}`); }
  transaction(hash) { return this.request(`/tx/${encodeURIComponent(hash)}`); }
  swapQuote({ amountIn, tokenIn, tokenOut, factory }) {
    return this.contractCall({ address: factory, function: "GetSwapQuote", args: [String(amountIn), tokenIn, tokenOut], readOnly: true });
  }
  bestRoute({ router, amountIn, tokenIn, tokenOut }) {
    return this.contractCall({ address: router, function: "GetBestRouteForAmount", args: [String(amountIn), tokenIn, tokenOut], readOnly: true });
  }
  vaultAccounting(vault) { return this.contractCall({ address: vault, function: "AccountingStatus", readOnly: true }); }
  vaultWithdrawalReceipt(vault, id) { return this.contractCall({ address: vault, function: "GetWithdrawalReceipt", args: [String(id)], readOnly: true }); }
  concentratedPosition(pool, id) { return this.contractCall({ address: pool, function: "PositionInfo", args: [String(id)], readOnly: true }); }
  mintConcentratedPosition({ pool, from, lowerSqrtX18, upperSqrtX18, amount0, amount1 }) {
    return this.contractCall({ address: pool, from, function: "MintConcentratedPosition", args: [String(lowerSqrtX18), String(upperSqrtX18), String(amount0), String(amount1)] });
  }
  transferConcentratedPosition({ pool, from, id, to }) { return this.contractCall({ address: pool, from, function: "TransferPosition", args: [String(id), to] }); }
  collectConcentratedPositionFees({ pool, from, id }) { return this.contractCall({ address: pool, from, function: "CollectPositionFees", args: [String(id)] }); }
  burnConcentratedPosition({ pool, from, id }) { return this.contractCall({ address: pool, from, function: "BurnConcentratedPosition", args: [String(id)] }); }
  submitOracleUpdate(signedTransaction) { return this.sendSignedTransaction(signedTransaction); }
  submitGovernanceAction(signedTransaction) { return this.sendSignedTransaction(signedTransaction); }
  contractCall({ address, function: fn, args = [], from = "", value = "0", readOnly = false }) {
    return this.request("/contract/call", { method: "POST", body: JSON.stringify({ contract_address: address, function: fn, args, from, value, read_only: readOnly }) });
  }
  sendSignedTransaction(transaction) { return this.request("/send_tx", { method: "POST", body: JSON.stringify(transaction) }); }
}

export class PoDLError extends Error {
  constructor(status, message, details) {
    super(message);
    this.name = "PoDLError";
    this.status = status;
    this.details = details;
  }
}

export function controlSigningPayload(transaction) {
  if (!transaction || !["oracle_update", "governance_action"].includes(transaction.type)) throw new Error("protocol control transaction required");
  const extra = transaction.extra_data instanceof Uint8Array
    ? [...transaction.extra_data].map((byte) => byte.toString(16).padStart(2, "0")).join("")
    : String(transaction.extra_data_hex || "");
  return JSON.stringify({
    domain: "PODL-CONTROL-TX-V2",
    from: String(transaction.from).toLowerCase(),
    to: String(transaction.to).toLowerCase(),
    value: String(transaction.value || "0"),
    gas: Number(transaction.gas),
    gas_price: Number(transaction.gas_price),
    chain_id: Number(transaction.chain_id),
    timestamp: Number(transaction.timestamp),
    nonce: Number(transaction.nonce),
    type: transaction.type,
    extra_data_hex: extra,
  });
}

export async function controlTransactionDigest(transaction) {
  const bytes = new TextEncoder().encode(controlSigningPayload(transaction));
  return new Uint8Array(await globalThis.crypto.subtle.digest("SHA-256", bytes));
}
