const DEFAULT_TIMEOUT_MS = 15000;

export function normalizeUrl(value) {
  return String(value || "").trim().replace(/\/+$/, "");
}

async function requestJson(url, options = {}) {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), options.timeoutMs || DEFAULT_TIMEOUT_MS);
  try {
    const headers = { ...(options.headers || {}) };
    let body = options.body;

    const isFormData = typeof FormData !== "undefined" && body instanceof FormData;
    const isBlob = typeof Blob !== "undefined" && body instanceof Blob;
    if (body && typeof body === "object" && !isFormData && !isBlob) {
      headers["Content-Type"] = headers["Content-Type"] || "application/json";
      body = JSON.stringify(body);
    }

    const response = await fetch(url, {
      method: options.method || "GET",
      headers,
      body,
      signal: controller.signal,
    });

    const text = await response.text();
    let data = null;
    try {
      data = text ? JSON.parse(text) : {};
    } catch {
      data = { raw: text };
    }

    if (!response.ok) {
      const message = (data && (data.error || data.message)) || text || `HTTP ${response.status}`;
      const error = new Error(message);
      error.status = response.status;
      error.data = data;
      throw error;
    }

    return data;
  } finally {
    clearTimeout(timeout);
  }
}

export async function getJson(url, options = {}) {
  return requestJson(url, { ...options, method: "GET" });
}

export async function postJson(url, body, options = {}) {
  return requestJson(url, { ...options, method: "POST", body });
}

export async function walletCreate(walletUrl, password) {
  return postJson(`${normalizeUrl(walletUrl)}/wallet/new`, { password });
}

export async function walletImportMnemonic(walletUrl, mnemonic, password) {
  return postJson(`${normalizeUrl(walletUrl)}/wallet/import/mnemonic`, { mnemonic, password });
}

export async function walletImportPrivateKey(walletUrl, privateKey) {
  return postJson(`${normalizeUrl(walletUrl)}/wallet/import/private-key`, { private_key: privateKey });
}

export async function walletBalance(nodeUrl, address) {
  return getJson(`${normalizeUrl(nodeUrl)}/balance?address=${encodeURIComponent(address)}`);
}

export async function nodeFaucet(nodeUrl, address) {
  return postJson(`${normalizeUrl(nodeUrl)}/faucet`, { address });
}

export async function walletSend(walletUrl, payload) {
  return postJson(`${normalizeUrl(walletUrl)}/wallet/send`, payload);
}

export async function walletSendBatch(walletUrl, payload) {
  return postJson(`${normalizeUrl(walletUrl)}/wallet/send_batch`, payload);
}

export async function walletContractTx(walletUrl, payload) {
  return postJson(`${normalizeUrl(walletUrl)}/wallet/contract-template`, payload);
}

export async function walletTokenBalance(walletUrl, contract, holder) {
  return getJson(`${normalizeUrl(walletUrl)}/wallet/token-balance?contract=${encodeURIComponent(contract)}&holder=${encodeURIComponent(holder)}`);
}

export async function walletBridgeLock(walletUrl, payload) {
  return postJson(`${normalizeUrl(walletUrl)}/wallet/bridge/lock`, payload);
}

export async function walletBridgePrivateLock(walletUrl, payload) {
  return postJson(`${normalizeUrl(walletUrl)}/wallet/bridge/private/lock`, payload);
}

export async function walletBridgeBurn(walletUrl, payload) {
  return postJson(`${normalizeUrl(walletUrl)}/wallet/bridge/burn`, payload);
}

export async function walletBridgePrivateBurn(walletUrl, payload) {
  return postJson(`${normalizeUrl(walletUrl)}/wallet/bridge/private/burn`, payload);
}

export async function walletBridgeLockBscToken(walletUrl, payload) {
  return postJson(`${normalizeUrl(walletUrl)}/wallet/bridge/lock_bsc_token`, payload);
}

export async function walletBridgePrivateLockBscToken(walletUrl, payload) {
  return postJson(`${normalizeUrl(walletUrl)}/wallet/bridge/private/lock_bsc_token`, payload);
}

export async function walletBridgeBurnLqdToken(walletUrl, payload) {
  return postJson(`${normalizeUrl(walletUrl)}/wallet/bridge/burn_lqd_token`, payload);
}

export async function walletBridgePrivateBurnLqdToken(walletUrl, payload) {
  return postJson(`${normalizeUrl(walletUrl)}/wallet/bridge/private/burn_lqd_token`, payload);
}

export async function nodeCallContract(nodeUrl, payload) {
  return postJson(`${normalizeUrl(nodeUrl)}/contract/call`, payload);
}

export async function nodeDeployBuiltin(nodeUrl, payload) {
  return postJson(`${normalizeUrl(nodeUrl)}/contract/deploy-builtin`, payload);
}

export async function nodeDeployContract(nodeUrl, formData) {
  return requestJson(`${normalizeUrl(nodeUrl)}/contract/deploy`, {
    method: "POST",
    body: formData,
    timeoutMs: 180000,
  });
}

export async function nodeCompilePlugin(nodeUrl, source) {
  return postJson(`${normalizeUrl(nodeUrl)}/contract/compile-plugin`, { source }, { timeoutMs: 180000 });
}

export async function nodeContractAbi(nodeUrl, address) {
  return getJson(`${normalizeUrl(nodeUrl)}/contract/getAbi?address=${encodeURIComponent(address)}`);
}

export async function nodeContractStorage(nodeUrl, address) {
  return getJson(`${normalizeUrl(nodeUrl)}/contract/storage?address=${encodeURIComponent(address)}`);
}

export async function nodeCurrentFactory(nodeUrl) {
  return getJson(`${normalizeUrl(nodeUrl)}/dex/current`);
}

export async function nodeLiquidityPools(nodeUrl) {
  return getJson(`${normalizeUrl(nodeUrl)}/liquidity/pools`);
}

export async function nodeRecentTransactions(nodeUrl) {
  return getJson(`${normalizeUrl(nodeUrl)}/transactions/recent`);
}

export async function nodeBridgeRequests(nodeUrl, mode = '') {
  const qs = mode ? `?mode=${encodeURIComponent(mode)}` : '';
  return getJson(`${normalizeUrl(nodeUrl)}/bridge/requests${qs}`);
}

export async function nodeBridgeFamilies(nodeUrl) {
  return getJson(`${normalizeUrl(nodeUrl)}/bridge/families`);
}

export async function nodeBridgeChains(nodeUrl) {
  return getJson(`${normalizeUrl(nodeUrl)}/bridge/chains`);
}

export async function nodeBridgeChainUpsert(nodeUrl, payload, apiKey = '') {
  const headers = apiKey ? { 'X-API-Key': apiKey } : {};
  return postJson(`${normalizeUrl(nodeUrl)}/bridge/chain`, payload, { headers });
}

export async function nodeBridgeChainRemove(nodeUrl, payload, apiKey = '') {
  const headers = apiKey ? { 'X-API-Key': apiKey } : {};
  return postJson(`${normalizeUrl(nodeUrl)}/bridge/chain/remove`, payload, { headers });
}

export async function nodeBridgeTokenUpsert(nodeUrl, payload, apiKey = '') {
  const headers = apiKey ? { 'X-API-Key': apiKey } : {};
  return postJson(`${normalizeUrl(nodeUrl)}/bridge/token`, payload, { headers });
}

export async function nodeBridgeTokenRemove(nodeUrl, payload, apiKey = '') {
  const headers = apiKey ? { 'X-API-Key': apiKey } : {};
  return postJson(`${normalizeUrl(nodeUrl)}/bridge/token/remove`, payload, { headers });
}

export async function nodeBridgeTokens(nodeUrl) {
  return getJson(`${normalizeUrl(nodeUrl)}/bridge/tokens`);
}

export async function nodeBaseFee(nodeUrl) {
  const data = await getJson(`${normalizeUrl(nodeUrl)}/basefee`);
  const base = data.base_fee ?? data.BaseFee ?? data.baseFee ?? 0;
  return Number(base || 0);
}

export async function resolveTokenMeta(nodeUrl, contract, holder) {
  const calls = async (fn) => {
    try {
      const res = await nodeCallContract(nodeUrl, {
        address: contract,
        caller: holder,
        fn,
        args: [],
        value: 0,
      });
      return res?.output || res?.Output || "";
    } catch {
      return "";
    }
  };

  const [symbol, name, decimalsStr] = await Promise.all([
    calls("Symbol").then((v) => v || calls("symbol")),
    calls("Name").then((v) => v || calls("name")),
    calls("Decimals").then((v) => v || calls("decimals")),
  ]);

  return {
    address: contract,
    symbol: symbol || "TOKEN",
    name: name || symbol || "Token",
    decimals: Number.parseInt(decimalsStr || "8", 10) || 8,
  };
}

export async function resolveTokenBalance(nodeUrl, walletUrl, contract, holder) {
  const defaultWalletUrl = "http://192.168.45.167:8080";
  const tryFns = ["BalanceOf", "balanceOf"];
  for (const fn of tryFns) {
    try {
      const res = await nodeCallContract(nodeUrl, {
        address: contract,
        caller: holder,
        fn,
        args: [holder],
        value: 0,
      });
      const out = res?.output || res?.Output || res?.result || "";
      if (out !== "" && out != null) return String(out);
    } catch {
      // continue
    }
  }

  try {
    const res = await walletTokenBalance(walletUrl || defaultWalletUrl, contract, holder);
    return String(res?.output || res?.Output || "0");
  } catch {
    return "0";
  }
}
