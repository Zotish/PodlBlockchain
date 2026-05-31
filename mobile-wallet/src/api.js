import { tronAddressToEvm } from "./utils";

const DEFAULT_TIMEOUT_MS = 30000;
const DEFAULT_GET_TIMEOUT_MS = 12000;
export const DEX_REGISTRY_URL = "https://dex-api.178-105-133-94.sslip.io";
const requestCache = new Map();
const inFlightRequests = new Map();

export function normalizeUrl(value) {
  return String(value || "").trim().replace(/\/+$/, "");
}

function normalizeUrlList(value) {
  if (Array.isArray(value)) {
    return [...new Set(value.map(normalizeUrl).filter(Boolean))];
  }
  const single = normalizeUrl(value);
  return single ? [single] : [];
}

function isLqdNodeUrl(urlOrUrls) {
  const text = normalizeUrlList(urlOrUrls).join(" ").toLowerCase();
  return (
    text.includes("lqd") ||
    text.includes("podl") ||
    text.includes("railway") ||
    text.includes("dazzling-peace") ||
    text.includes("178.105.133.94") ||
    text.includes("178-105-133-94.sslip.io")
  );
}

async function tryUrls(urlOrUrls, runner, accept = (result) => result !== null && result !== undefined) {
  const urls = normalizeUrlList(urlOrUrls);
  let lastError = null;
  for (const url of urls) {
    try {
      const result = await runner(url);
      if (accept(result)) return result;
    } catch (error) {
      lastError = error;
    }
  }
  if (lastError) throw lastError;
  return null;
}

function requestKey(method, url, body) {
  return `${method}:${url}:${body && typeof body === "string" ? body : ""}`;
}

async function requestJson(url, options = {}) {
  const method = options.method || "GET";
  const cacheTtlMs = method === "GET" ? Number(options.cacheTtlMs || 0) : 0;
  const cacheKey = cacheTtlMs > 0 ? requestKey(method, url, "") : "";
  if (cacheKey) {
    const cached = requestCache.get(cacheKey);
    if (cached && Date.now() - cached.at < cacheTtlMs) return cached.value;
  }

  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), options.timeoutMs || (method === "GET" ? DEFAULT_GET_TIMEOUT_MS : DEFAULT_TIMEOUT_MS));
  try {
    const headers = { ...(options.headers || {}) };
    let body = options.body;

    const isFormData = typeof FormData !== "undefined" && body instanceof FormData;
    const isBlob = typeof Blob !== "undefined" && body instanceof Blob;
    if (body && typeof body === "object" && !isFormData && !isBlob) {
      headers["Content-Type"] = headers["Content-Type"] || "application/json";
      body = JSON.stringify(body);
    }

    const key = requestKey(method, url, body);
    if (inFlightRequests.has(key)) return inFlightRequests.get(key);

    const requestPromise = fetch(url, {
      method,
      headers,
      body,
      signal: controller.signal,
    }).then(async (response) => {
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

      if (cacheKey) requestCache.set(cacheKey, { at: Date.now(), value: data });
      return data;
    }).finally(() => {
      inFlightRequests.delete(key);
    });
    inFlightRequests.set(key, requestPromise);
    return await requestPromise;
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

export async function nodeCompile(nodeUrl, type, source) {
  return postJson(`${normalizeUrl(nodeUrl)}/contract/compile`, { type, source }, { timeoutMs: 60000 });
}

export async function nodeContractAbi(nodeUrl, address) {
  return getJson(`${normalizeUrl(nodeUrl)}/contract/getAbi?address=${encodeURIComponent(address)}`, { cacheTtlMs: 30000 });
}

export async function nodeContractStorage(nodeUrl, address) {
  return getJson(`${normalizeUrl(nodeUrl)}/contract/storage?address=${encodeURIComponent(address)}`, { cacheTtlMs: 5000 });
}

export async function nodeCurrentFactory(nodeUrl) {
  return getJson(`${normalizeUrl(nodeUrl)}/dex/current`, { cacheTtlMs: 30000 });
}

export async function nodeLiquidityPools(nodeUrl) {
  return getJson(`${normalizeUrl(nodeUrl)}/liquidity/pools`, { cacheTtlMs: 5000 });
}

export async function dexRegistryConfig(registryUrl = DEX_REGISTRY_URL) {
  return getJson(`${normalizeUrl(registryUrl)}/config`, { cacheTtlMs: 30000, timeoutMs: 8000 });
}

export async function dexRegistryTokens(registryUrl = DEX_REGISTRY_URL) {
  const data = await getJson(`${normalizeUrl(registryUrl)}/tokens`, { cacheTtlMs: 30000, timeoutMs: 8000 });
  return Array.isArray(data) ? data : [];
}

export async function dexRegistryPools(registryUrl = DEX_REGISTRY_URL) {
  const data = await getJson(`${normalizeUrl(registryUrl)}/pools`, { cacheTtlMs: 30000, timeoutMs: 8000 });
  return Array.isArray(data) ? data : [];
}

export async function nodeEstimateGas(nodeUrl, payload) {
  return postJson(`${normalizeUrl(nodeUrl)}/contract/estimate_gas`, payload);
}

export async function nodeStatus(nodeUrl) {
  try {
    const res = await getJson(`${normalizeUrl(nodeUrl)}/blockchain`, { cacheTtlMs: 5000, timeoutMs: 6000 });
    return { online: !!(res?.status === "ok" || res?.version || res?.height), ...res };
  } catch {
    return { online: false };
  }
}

export async function nodeRecentTransactions(nodeUrl, limit = 30) {
  return getJson(`${normalizeUrl(nodeUrl)}/transactions/recent?limit=${limit}`, { cacheTtlMs: 3000 });
}

export async function nodeBridgeRequests(nodeUrl, mode = '') {
  const qs = mode ? `?mode=${encodeURIComponent(mode)}` : '';
  return getJson(`${normalizeUrl(nodeUrl)}/bridge/requests${qs}`, { cacheTtlMs: 5000 });
}

export async function nodeBridgeFamilies(nodeUrl) {
  return getJson(`${normalizeUrl(nodeUrl)}/bridge/families`, { cacheTtlMs: 60000 });
}

export async function nodeBridgeChains(nodeUrl) {
  return getJson(`${normalizeUrl(nodeUrl)}/bridge/chains`, { cacheTtlMs: 30000 });
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
  return getJson(`${normalizeUrl(nodeUrl)}/bridge/tokens`, { cacheTtlMs: 30000 });
}

export async function nodeBaseFee(nodeUrl) {
  const data = await getJson(`${normalizeUrl(nodeUrl)}/basefee`, { cacheTtlMs: 3000 });
  const base = data.base_fee ?? data.BaseFee ?? data.baseFee ?? 0;
  return Number(base || 0);
}

// Decode an ABI-encoded string or bytes32 return value from eth_call.
// Handles standard ABI string encoding, bytes32 pattern (MKR, WBTC), and
// non-standard short returns from some testnets/chains (Tron, Fantom, etc.).
function _decodeEvmString(hex) {
  if (!hex || hex === "0x") return "";
  try {
    // Always lowercase so comparison works regardless of RPC casing (Tron, Harmony etc.)
    const s = hex.replace(/^0x/i, "").toLowerCase();
    if (s.length < 64) {
      // Some chains return very short non-zero hex for bytes32 strings
      const bytes = (s.match(/.{1,2}/g) || []).map(b => parseInt(b, 16)).filter(b => b > 31 && b < 127);
      return String.fromCharCode(...bytes).trim();
    }
    const firstWord = s.slice(0, 64);
    if (firstWord === "0000000000000000000000000000000000000000000000000000000000000020") {
      // Standard ABI dynamic string: offset(32) + length(32) + data
      const len = parseInt(s.slice(64, 128), 16);
      if (len === 0 || len > 512) return "";
      const bytes = (s.slice(128, 128 + len * 2).match(/.{1,2}/g) || []).map(b => parseInt(b, 16));
      return String.fromCharCode(...bytes).replace(/[\x00-\x1F\x7F-\x9F]/g, "").trim();
    }
    // bytes32: left-aligned ASCII text, right-padded with zeros (MKR, WBTC, etc.)
    const bytes = (firstWord.match(/.{1,2}/g) || []).map(b => parseInt(b, 16)).filter(b => b > 31 && b < 127);
    return String.fromCharCode(...bytes).replace(/[\x00-\x1F\x7F-\x9F]/g, "").trim();
  } catch { return ""; }
}

export async function resolveTokenMeta(nodeUrl, contract, holder) {
  const urls = normalizeUrlList(nodeUrl);
  const isLqd = isLqdNodeUrl(urls);

  if (!isLqd) {
    const verified = await tryUrls(urls, async (url) => {
      const addr = contract.toLowerCase().startsWith("0x") ? contract : "0x" + contract;
      const callRpc = async (data, method = "eth_call") => {
        const res = await postJson(url, {
          jsonrpc: "2.0", id: Date.now(), method,
          params: method === "eth_call" ? [{ to: addr, data }, "latest"] : [addr, "latest"],
        }).catch(() => null);
        return res?.result;
      };

      const codeHex = await callRpc(null, "eth_getCode");
      if (!codeHex || codeHex === "0x" || codeHex === "0x0") {
        throw new Error("Token contract not found on this RPC");
      }

      const [nameHex, symbolHex, decimalsHex] = await Promise.all([
        callRpc("0x06fdde03"),
        callRpc("0x95d89b41"),
        callRpc("0x313ce567"),
      ]);

      const symbol = _decodeEvmString(symbolHex);
      const name = _decodeEvmString(nameHex);
      let decimals = 18;
      if (decimalsHex && decimalsHex !== "0x") {
        try {
          const parsed = Number(BigInt(decimalsHex));
          if (Number.isFinite(parsed) && parsed >= 0 && parsed <= 255) decimals = parsed;
        } catch {
          decimals = 18;
        }
      }

      if (!symbol && !name) {
        return null;
      }

      return {
        address: contract,
        symbol: symbol || "",
        name: name || symbol || "",
        decimals,
        verified: true,
      };
    }, (result) => !!result?.verified).catch(() => null);
    if (verified) return verified;
    return { address: contract, symbol: "", name: "", decimals: 18, verified: false };
  }

  const verified = await tryUrls(urls, async (url) => {
    const calls = async (fn) => {
      try {
        const res = await nodeCallContract(url, { address: contract, caller: holder, fn, args: [], value: 0 });
        return res?.output || res?.Output || "";
      } catch {
        return "";
      }
    };

    const [symbol, name, decimalsStr] = await Promise.all([
      calls("Symbol").then(v => v || calls("symbol")),
      calls("Name").then(v => v || calls("name")),
      calls("Decimals").then(v => v || calls("decimals")),
    ]);

    const dec = String(decimalsStr || "").startsWith("0x")
      ? parseInt(decimalsStr, 16)
      : parseInt(decimalsStr || "8", 10);

    return {
      address: contract,
      symbol: symbol || "",
      name: name || symbol || "",
      decimals: dec || 8,
      verified: !!(symbol || name),
    };
  }, (result) => !!result?.verified).catch(() => null);
  if (verified) return verified;
  return { address: contract, symbol: "", name: "", decimals: 8, verified: false };
}

export async function resolveTokenBalance(nodeUrl, walletUrl, contract, holder) {
  const urls = normalizeUrlList(nodeUrl);
  const isLqd = isLqdNodeUrl(urls);

  if (!isLqd) {
    return tryUrls(urls, async (url) => {
      const holderHex = holder.replace(/^0x/, '').padStart(64, '0');
      const data = "0x70a08231" + holderHex;
      const res = await postJson(url, {
        jsonrpc: "2.0", id: Date.now(), method: "eth_call",
        params: [{ to: contract, data }, "latest"]
      }).catch(() => null);
      return res?.result ? BigInt(res.result).toString() : null;
    }, (value) => value !== null);
  }

  const defaultWalletUrl = "http://192.168.45.167:8080";
  const tryFns = ["BalanceOf", "balanceOf"];
  const nodeResult = await tryUrls(urls, async (url) => {
    for (const fn of tryFns) {
      try {
        const res = await nodeCallContract(url, {
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
    return null;
  }, (value) => value !== null).catch(() => null);

  if (nodeResult !== null && nodeResult !== undefined) return nodeResult;

  try {
    const res = await walletTokenBalance(walletUrl || defaultWalletUrl, contract, holder);
    return String(res?.output || res?.Output || "0");
  } catch {
    return "0";
  }
}

// ─── Multichain token metadata ───────────────────────────────────────────────

// Well-known Cosmos native denom registry: strips micro-prefix and maps to readable symbol
const _COSMOS_DENOMS = {
  uatom: { symbol: "ATOM",  name: "Cosmos Hub",  decimals: 6 },
  uosmo: { symbol: "OSMO",  name: "Osmosis",     decimals: 6 },
  usei:  { symbol: "SEI",   name: "SEI Network", decimals: 6 },
  inj:   { symbol: "INJ",   name: "Injective",   decimals: 18 },
  uinj:  { symbol: "INJ",   name: "Injective",   decimals: 18 },
  ujuno: { symbol: "JUNO",  name: "Juno",        decimals: 6 },
  ustars:{ symbol: "STARS", name: "Stargaze",    decimals: 6 },
  uakt:  { symbol: "AKT",   name: "Akash",       decimals: 6 },
  uion:  { symbol: "ION",   name: "Ion",         decimals: 6 },
  uluna: { symbol: "LUNA",  name: "Terra",       decimals: 6 },
  uusd:  { symbol: "UST",   name: "TerraUSD",    decimals: 6 },
};

function _resolveNativeDenom(denom, nodeUrl) {
  const known = _COSMOS_DENOMS[denom];
  if (known) return known;
  // Generic micro-denom: uXXX → XXX
  if (denom.startsWith("u") && !denom.startsWith("ibc/") && denom.length > 1) {
    return { symbol: denom.slice(1).toUpperCase(), name: denom.slice(1).toUpperCase(), decimals: 6 };
  }
  return null;
}

const _VERIFIED_TESTNET_TOKENS = [
  { family: "evm", chainHints: ["railway", "podl", "dazzling-peace"], address: "0x654fefacf72c022b08146128fcc5dafabe0eeb0e", symbol: "helo", name: "hi", decimals: 8 },
  { family: "evm", chainHints: ["sepolia"], address: "0x1c7D4B196Cb0C7B01d743Fbc6116a902379C7238", symbol: "USDC", name: "USDC", decimals: 6 },
  { family: "evm", chainHints: ["bsc"], address: "0x109656Aba6F175c634c63C9874f29CeAAAB8E606", symbol: "USYC", name: "US Yield Coin", decimals: 6 },
  { family: "solana", address: "4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU", symbol: "USDC", name: "USD Coin", decimals: 6 },
  { family: "evm", chainHints: ["polygon", "amoy"], address: "0x41E94Eb019C0762f9Bfcf9Fb1E58725BfB0e7582", symbol: "USDC", name: "USDC", decimals: 6 },
  { family: "evm", chainHints: ["arbitrum"], address: "0x75faf114eafb1BDbe2F0316DF893fd58CE46AA4d", symbol: "USDC", name: "USD Coin", decimals: 6 },
  { family: "evm", chainHints: ["optimism"], address: "0x5fd84259d66Cd46123540766Be93DFE6D43130D7", symbol: "USDC", name: "USDC", decimals: 6 },
  { family: "evm", chainHints: ["base"], address: "0x036CbD53842c5426634e7929541eC2318f3dCF7e", symbol: "USDC", name: "USDC", decimals: 6 },
  { family: "evm", chainHints: ["avalanche", "avax"], address: "0x5425890298aed601595a70AB815c96711a31Bc65", symbol: "USDC", name: "USD Coin", decimals: 6 },
  { family: "evm", chainHints: ["linea"], address: "0xFEce4462D57bD51A6A552365A011b95f0E16d9B7", symbol: "USDC", name: "USD//C", decimals: 6 },
  { family: "evm", chainHints: ["scroll"], address: "0x5300000000000000000000000000000000000004", symbol: "WETH", name: "Wrapped Ether", decimals: 18 },
  { family: "evm", chainHints: ["berachain", "bepolia"], address: "0xFCBD14DC51f0A4d49d5E53C2E0950e0bC26d0Dce", symbol: "HONEY", name: "Honey", decimals: 18 },
  { family: "evm", chainHints: ["fantom"], address: "0xfaFedb041c0DD4fA2Dc0d87a6B0979Ee6FA7af5F", symbol: "LINK", name: "ChainLink Token", decimals: 18 },
  { family: "evm", chainHints: ["blast"], address: "0x4200000000000000000000000000000000000022", symbol: "USDB", name: "Rebasing USD", decimals: 18 },
  { family: "evm", chainHints: ["zksync"], address: "0xAe045DE5638162fa134807Cb558E15A3F5A7F853", symbol: "USDC", name: "USDC", decimals: 6 },
  { family: "evm", chainHints: ["monad"], address: "0x534b2f3A21130d7a60830c2Df862319e593943A3", symbol: "USDC", name: "USDC", decimals: 6 },
  { family: "near", address: "3e2210e1184b45b64c8a434c0a7e7b23cc04ea7eb7a6c3c32520d03d4afcb8af", symbol: "USDC", name: "USDC", decimals: 6 },
  { family: "aptos", address: "0x69091fbab5f7d635ee7ac5098cf0c1efbe31d68fec0f2cd565e8d168daf52832", symbol: "USDC", name: "USDC", decimals: 6 },
  { family: "tron", address: "TG3XXyExBkPp9nzdajDZsozEu4BkaSJozs", symbol: "USDT", name: "TetherToken", decimals: 6 },
  { family: "evm", chainHints: ["celo"], address: "0x01C5C0122039549AD1493B8220cABEdD739BC44E", symbol: "USDC", name: "USDC", decimals: 6 },
  { family: "sui", address: "0xa1ec7fc00a6f40db9693ad1415d0c193ad3906494428cf252621037bd7117e29::usdc::USDC", symbol: "USDC", name: "USDC", decimals: 6 },
  { family: "evm", chainHints: ["mantle"], address: "0xDeadDeAddeAddEAddeadDEaDDEAdDeaDDeAD0000", symbol: "MNT", name: "Mantle Token", decimals: 18 },
  { family: "evm", chainHints: ["cronos"], address: "0xa85d35eb8E439078a1810Ec3738997E61d157f0d", symbol: "WCRO", name: "Wrapped CRO", decimals: 18 },
  { family: "evm", chainHints: ["metis"], address: "0xDeadDeAddeAddEAddeadDEaDDEAdDeaDDeAD0000", symbol: "Metis", name: "Metis Token", decimals: 18 },
  { family: "evm", chainHints: ["moonbase", "moonbeam"], address: "0x0000000000000000000000000000000000000802", symbol: "DEV", name: "DEV token", decimals: 18 },
  { family: "harmony", address: "0xBDE1E4D62D7ee1e88d02d623741d7ef9F549dD43", symbol: "AIUSD", name: "AIUSD Crypto-Rewards", decimals: 18 },
  { family: "ton", address: "EQDnRHbK5vJBLQyAnS6V8XNoRerCebnn9A2FlVlHtFVLFGZ", symbol: "USDT", name: "Tether USD", decimals: 6 },
  { family: "sei", address: "0x4fCF1784B31630811181f670Aea7A7bEF803eaED", symbol: "USDC", name: "USDC", decimals: 6 },
  { family: "evm", chainHints: ["hyperliquid"], address: "0x2B3370eE501B4a559b57D449569354196457D8Ab", symbol: "USDC", name: "USDC", decimals: 6 },
  { family: "evm", chainHints: ["story"], address: "0x1514000000000000000000000000000000000000", symbol: "WIP", name: "Wrapped IP", decimals: 18 },
  { family: "evm", chainHints: ["sonic"], address: "0x0BA304580ee7c9a980CF72e55f5Ed2E9fd30Bc51", symbol: "USDC", name: "USDC", decimals: 6 },
  { family: "starknet", address: "0x0512feAc6339Ff7889822cb5aA2a86C848e9D392bB0E3E237C008674feeD8343", symbol: "USDC", name: "USDC", decimals: 6 },
];

function _tokenRegistryKey(value) {
  return String(value || "").trim().toLowerCase();
}

function _verifiedRegistryMeta(family, urls, contract) {
  const fam = String(family || "evm").toLowerCase();
  const key = _tokenRegistryKey(contract);
  const urlText = normalizeUrlList(urls).join(" ").toLowerCase();
  const found = _VERIFIED_TESTNET_TOKENS.find((item) => {
    if (item.family !== fam || _tokenRegistryKey(item.address) !== key) return false;
    if (!item.chainHints?.length) return true;
    return item.chainHints.some((hint) => urlText.includes(String(hint).toLowerCase()));
  });
  return found ? {
    address: contract,
    symbol: found.symbol,
    name: found.name,
    decimals: found.decimals,
    verified: true,
    verifiedSource: "testnet-registry",
  } : null;
}

export async function resolveTokenMetaMultichain(nodeUrl, contract, holder, family) {
  const fam = String(family || "evm").toLowerCase();
  const urls = normalizeUrlList(nodeUrl);
  const registryMeta = _verifiedRegistryMeta(fam, urls, contract);
  if (registryMeta) return registryMeta;

  if (fam === "evm") return resolveTokenMeta(urls, contract, holder);
  if (fam === "harmony") {
    const rpcMeta = await resolveTokenMeta(urls, contract, holder).catch(() => null);
    if (rpcMeta?.verified) return rpcMeta;
    const explorerMeta = await getJson(
      `https://explorer.testnet.harmony.one/api/v2/tokens/${encodeURIComponent(contract)}`,
      { timeoutMs: 8000 }
    ).catch(() => null);
    if (explorerMeta?.symbol && explorerMeta?.name) {
      return {
        address: contract,
        symbol: explorerMeta.symbol,
        name: explorerMeta.name,
        decimals: Number(explorerMeta.decimals ?? 18),
        verified: true,
        verifiedSource: "harmony-testnet-explorer",
      };
    }
    return rpcMeta || { address: contract, symbol: "", name: "", decimals: 18, verified: false };
  }
  if ((fam === "sei" || fam === "injective") && String(contract || "").startsWith("0x")) {
    return resolveTokenMeta(urls, contract, holder);
  }

  // ── Solana SPL token ─────────────────────────────────────────────────────────
  if (fam === "solana") {
    const mintRes = await tryUrls(urls, (url) => postJson(url, {
      jsonrpc: "2.0", id: 1, method: "getAccountInfo",
      params: [contract, { encoding: "jsonParsed" }],
    }).catch(() => null)).catch(() => null);
    const mintInfo = mintRes?.result?.value?.data?.parsed?.info;
    if (!mintRes?.result?.value || !mintInfo) {
      throw new Error("Mint contract not found");
    }
    const decimals = mintInfo?.decimals ?? 9;

    const knownSolanaTokens = {
      "4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU": { symbol: "USDC", name: "USD Coin", decimals: 6 },
    };
    if (knownSolanaTokens[contract]) {
      return { address: contract, ...knownSolanaTokens[contract], verified: true };
    }

    // 1) Try Jupiter strict list (verified tokens)
    try {
      const strict = await getJson("https://token.jup.ag/strict", { timeoutMs: 8000 });
      const found = (strict || []).find(t => t.address === contract);
      if (found) return { address: contract, symbol: found.symbol, name: found.name, decimals: found.decimals ?? decimals, verified: !!(found.symbol && found.name) };
    } catch { /* fall through */ }

    // 2) Try Jupiter all-tokens list (includes unverified)
    try {
      const all = await getJson("https://token.jup.ag/all", { timeoutMs: 10000 });
      const found = (all || []).find(t => t.address === contract);
      if (found) return { address: contract, symbol: found.symbol, name: found.name, decimals: found.decimals ?? decimals, verified: !!(found.symbol && found.name) };
    } catch { /* fall through */ }

    // 3) Try Solana token registry (covers devnet tokens registered there)
    try {
      const reg = await getJson(
        `https://raw.githubusercontent.com/solana-labs/token-list/main/src/tokens/solana.tokenlist.json`,
        { timeoutMs: 8000 }
      );
      const found = (reg?.tokens || []).find(t => t.address === contract);
      if (found) return { address: contract, symbol: found.symbol, name: found.name, decimals: found.decimals ?? decimals, verified: !!(found.symbol && found.name) };
    } catch { /* fall through */ }

    // 4) Try fetching on-chain Metaplex metadata via DAS API (devnet)
    try {
      const das = await tryUrls(urls, (url) => postJson(url, {
        jsonrpc: "2.0", id: 1, method: "getAsset", params: { id: contract },
      }).catch(() => null)).catch(() => null);
      const content = das?.result?.content?.metadata;
      if (content?.symbol || content?.name) {
        return {
          address: contract,
          symbol: content.symbol || contract.slice(0, 6).toUpperCase(),
          name: content.name || content.symbol || "SPL Token",
          decimals,
          verified: true,
        };
      }
    } catch { /* fall through */ }

    return { address: contract, symbol: "", name: "", decimals, verified: false };
  }

  // ── NEAR NEP-141 ─────────────────────────────────────────────────────────────
  if (fam === "near") {
    const args = btoa(JSON.stringify({}));
    const res = await tryUrls(urls, (url) => postJson(url, {
      jsonrpc: "2.0", id: 1, method: "query",
      params: { request_type: "call_function", finality: "final", account_id: contract, method_name: "ft_metadata", args_base64: args },
    }).catch(() => null)).catch(() => null);
    if (res?.result?.result) {
      try {
        const meta = JSON.parse(String.fromCharCode(...res.result.result));
        return { address: contract, symbol: meta.symbol || "", name: meta.name || meta.symbol || "", decimals: Number(meta.decimals ?? 24), verified: true };
      } catch { /* fall through */ }
    }
    return { address: contract, symbol: "", name: "", decimals: 24, verified: false };
  }

  // ── Aptos coin ───────────────────────────────────────────────────────────────
  if (fam === "aptos") {
    const aptosBase = (url) => {
      const normalized = normalizeUrl(url);
      return normalized.endsWith("/v1") ? normalized : `${normalized}/v1`;
    };
    const faMeta = await tryUrls(urls, (url) => getJson(
      `${aptosBase(url)}/accounts/${encodeURIComponent(contract)}/resource/0x1::fungible_asset::Metadata`
    ).catch(() => null)).catch(() => null);
    if (faMeta?.data) {
      return {
        address: contract,
        symbol: faMeta.data.symbol || "",
        name: faMeta.data.name || faMeta.data.symbol || "",
        decimals: Number(faMeta.data.decimals ?? 8),
        verified: !!(faMeta.data.symbol && faMeta.data.name),
      };
    }

    const moduleAddr = contract.split("::")[0];
    const res = await tryUrls(urls, (url) => getJson(
      `${aptosBase(url)}/accounts/${encodeURIComponent(moduleAddr)}/resource/0x1::coin::CoinInfo<${contract}>`
    ).catch(() => null)).catch(() => null);
    if (res?.data) {
      return {
        address: contract,
        symbol:   res.data.symbol   || contract.split("::")[2] || "APT",
        name:     res.data.name     || "Aptos Token",
        decimals: Number(res.data.decimals ?? 8),
        verified: true,
      };
    }
    return { address: contract, symbol: "", name: "", decimals: 8, verified: false };
  }

  // ── SUI coin ─────────────────────────────────────────────────────────────────
  if (fam === "sui") {
    const res = await tryUrls(urls, (url) => postJson(url, {
      jsonrpc: "2.0", id: 1, method: "suix_getCoinMetadata", params: [contract],
    }).catch(() => null)).catch(() => null);
    if (res?.result) {
      return {
        address: contract,
        symbol:   res.result.symbol   || contract.split("::")[2] || "SUI",
        name:     res.result.name     || "SUI Token",
        decimals: Number(res.result.decimals ?? 9),
        verified: true,
      };
    }
    return { address: contract, symbol: "", name: "", decimals: 9, verified: false };
  }

  // ── Cosmos / SEI / Injective ─────────────────────────────────────────────────
  if (fam === "cosmos" || fam === "cosmos-testnet" || fam === "sei" || fam === "injective") {
    // Known native denom?
    const native = _resolveNativeDenom(contract, nodeUrl);
    if (native) return { address: contract, ...native, verified: true };

    // IBC denom — try denom_trace to get the original path
    if (contract.startsWith("ibc/")) {
      const hash = contract.slice(4);
      try {
        const trace = await tryUrls(urls, (url) => getJson(
          `${normalizeUrl(url)}/ibc/apps/transfer/v1/denom_traces/${hash}`
        ).catch(() => null)).catch(() => null);
        const path = trace?.denom_trace?.base_denom || "";
        if (path) {
          const nativeFromPath = _resolveNativeDenom(path, nodeUrl);
          if (nativeFromPath) return { address: contract, ...nativeFromPath, verified: true };
          return {
            address: contract,
            symbol: path.toUpperCase().slice(0, 12),
            name: `IBC ${path.toUpperCase()}`,
            decimals: 6,
            verified: true,
          };
        }
      } catch { /* fall through */ }
      return { address: contract, symbol: "", name: "", decimals: 6, verified: false };
    }

    // CW20 contract — query token_info
    const cwPrefixes = ["cosmos1", "osmo1", "sei1", "inj1", "stars1", "juno1", "neutron1", "migaloo1", "kujira1"];
    if (cwPrefixes.some(p => contract.startsWith(p))) {
      // Use Buffer.from for reliable base64 in React Native
      const queryJson = JSON.stringify({ token_info: {} });
      const queryB64 = typeof Buffer !== "undefined"
        ? Buffer.from(queryJson).toString("base64")
        : btoa(queryJson);
      try {
        const res = await tryUrls(urls, (url) => getJson(
          `${normalizeUrl(url)}/cosmwasm/wasm/v1/contract/${contract}/smart/${encodeURIComponent(queryB64)}`
        ).catch(() => null)).catch(() => null);
        if (res?.data?.symbol) {
          return {
            address: contract,
            symbol:   res.data.symbol,
            name:     res.data.name     || res.data.symbol,
            decimals: Number(res.data.decimals ?? 6),
            verified: true,
          };
        }
      } catch { /* fall through */ }

      // Try alternate REST path (some nodes use /wasm/v1beta1/)
      try {
        const res = await tryUrls(urls, (url) => getJson(
          `${normalizeUrl(url)}/wasm/v1beta1/contract/${contract}/smart/${encodeURIComponent(queryB64)}`
        ).catch(() => null)).catch(() => null);
        if (res?.data?.symbol) {
          return {
            address: contract,
            symbol:   res.data.symbol,
            name:     res.data.name     || res.data.symbol,
            decimals: Number(res.data.decimals ?? 6),
            verified: true,
          };
        }
      } catch { /* fall through */ }
    }

    return { address: contract, symbol: "", name: "", decimals: 6, verified: false };
  }

  // ── TRON TRC-20 ──────────────────────────────────────────────────────────────
  if (fam === "tron") {
    // TRON has EVM-compatible JSON-RPC; contract may be T... or 0x format
    const evmContract = tronAddressToEvm(contract);
    return tryUrls(urls, async (url) => {
      const callTron = async (selector) => {
        const res = await postJson(url, {
          jsonrpc: "2.0", id: Date.now(), method: "eth_call",
          params: [{ to: evmContract, data: selector }, "latest"],
        }).catch(() => null);
        return res?.result;
      };
      const [nameHex, symHex, decHex] = await Promise.all([
        callTron("0x06fdde03"),
        callTron("0x95d89b41"),
        callTron("0x313ce567"),
      ]);
      let sym = _decodeEvmString(symHex);
      if (!sym) {
        // Shasta can occasionally return an empty parallel eth_call for symbol().
        sym = _decodeEvmString(await callTron("0x95d89b41"));
      }
      const nam = _decodeEvmString(nameHex);
      let dec = 6;
      if (decHex && decHex !== "0x") {
        try { dec = Number(BigInt(decHex)) || 6; } catch { dec = 6; }
      }
      const fallbackSymbol = nam ? nam.replace(/\s+/g, "").slice(0, 12).toUpperCase() : "";
      return { address: contract, symbol: sym || fallbackSymbol, name: nam || sym || "", decimals: dec, verified: !!(sym || nam) };
    });
  }

  // ── TON Jetton ───────────────────────────────────────────────────────────────
  if (fam === "ton") {
    const primaryUrl = urls[0] || "";
    const isTestnet = primaryUrl.includes("testnet");
    const v3Base = isTestnet
      ? "https://testnet.toncenter.com/api/v3"
      : "https://toncenter.com/api/v3";

    // Helper: extract decimals safely from string or number
    const _parseTonDecimals = (v) => {
      if (v === undefined || v === null) return 9;
      const n = Number(String(v).trim());
      return Number.isFinite(n) && n >= 0 ? n : 9;
    };

    // 1) TonCenter v3 /jetton/masters — most complete data
    try {
      const res = await getJson(
        `${v3Base}/jetton/masters?address=${encodeURIComponent(contract)}&limit=1`,
        { timeoutMs: 8000 }
      ).catch(() => null);
      const master = res?.jetton_masters?.[0];
      if (master) {
        // jetton_content is the primary metadata object in v3
        const jc = master?.jetton_content || master?.content || {};
        // Also check metadata object keyed by raw address
        const rawAddr = Object.keys(res?.metadata || {})[0];
        const info = rawAddr ? res.metadata[rawAddr]?.token_info?.[0] : null;
        const sym = info?.symbol || jc?.symbol || jc?.name?.slice(0, 8) || "";
        const nam = info?.name  || jc?.name   || sym;
        const dec = _parseTonDecimals(info?.decimals ?? jc?.decimals);
        if (sym) return { address: contract, symbol: sym, name: nam || sym, decimals: dec, verified: true };
      }
    } catch { /* fall through */ }

    // 2) TonCenter v3 /jetton/masters with direct address param (alternate endpoint)
    try {
      const res = await getJson(
        `${v3Base}/jetton/masters?jetton_master_address=${encodeURIComponent(contract)}&limit=1`,
        { timeoutMs: 8000 }
      ).catch(() => null);
      const jc = res?.jetton_masters?.[0]?.jetton_content || {};
      const sym = jc?.symbol || "";
      if (sym) {
        return { address: contract, symbol: sym, name: jc?.name || sym,
                 decimals: _parseTonDecimals(jc?.decimals), verified: true };
      }
    } catch { /* fall through */ }

    // 3) TonCenter v2 getTokenData fallback
    try {
      const v2Base = isTestnet
        ? "https://testnet.toncenter.com/api/v2"
        : "https://toncenter.com/api/v2";
      const res = await getJson(
        `${v2Base}/getTokenData?address=${encodeURIComponent(contract)}`,
        { timeoutMs: 8000 }
      ).catch(() => null);
      // v2 returns result.jetton_content or result.jetton_content.data
      const jc = res?.result?.jetton_content?.data || res?.result?.jetton_content || res?.result || {};
      const sym = jc?.symbol || jc?.name || "";
      const nam = jc?.name   || sym;
      const dec = _parseTonDecimals(jc?.decimals);
      if (sym) return { address: contract, symbol: sym, name: nam || sym, decimals: dec, verified: true };
    } catch { /* fall through */ }

    return { address: contract, symbol: "", name: "", decimals: 9, verified: false };
  }

  // ── Starknet ERC-20 ──────────────────────────────────────────────────────────
  if (fam === "starknet") {
    // Starknet ERC-20 uses felt252 strings for name/symbol; read via starknet_call
    return tryUrls(urls, async (url) => {
      const callStark = async (selector) => {
        const res = await postJson(url, {
          jsonrpc: "2.0", id: 1, method: "starknet_call",
          params: [{ contract_address: contract, entry_point_selector: selector, calldata: [] }, { block_tag: "latest" }],
        }).catch(() => null);
        if (res?.result !== undefined) return res.result;
        const res2 = await postJson(url, {
          jsonrpc: "2.0", id: 1, method: "starknet_call",
          params: [{ contract_address: contract, entry_point_selector: selector, calldata: [] }, "latest"],
        }).catch(() => null);
        return res2?.result;
      };
      // name()  = 0x361458367e696363fbcc70777d07ebbd2394e89fd0adcaf147faccd1d294d60
      // symbol()= 0x216b05c387bab9ac31918a3e61672f4618601f3c598a2f3f2710f37053e1ea4
      // decimals()=0x4c4fb1ab068f6039d5780c68dd0fa2f8742cceb3426d19667778ca7f3518a9
      const [nameRes, symRes, decRes] = await Promise.all([
        callStark("0x361458367e696363fbcc70777d07ebbd2394e89fd0adcaf147faccd1d294d60"),
        callStark("0x216b05c387bab9ac31918a3e61672f4618601f3c598a2f3f2710f37053e1ea4"),
        callStark("0x4c4fb1ab068f6039d5780c68dd0fa2f8742cceb3426d19667778ca7f3518a9"),
      ]);
      // Decode a Starknet return value — handles BOTH:
      //   felt252  (Cairo 0 / OZ v0.x): single felt as ASCII bytes → arr = ["0x55534443"] for "USDC"
      //   ByteArray (Cairo 1 / OZ v0.8+): [data_len, ...words, pending_word, pending_word_len]
      //   For short strings (< 31 bytes) ByteArray is: [0, pending_word, pending_word_len]
      const decodeFelt = (arr) => {
        if (!arr?.length) return "";
        try {
          // ByteArray: arr[0] is data_len (number of full 31-byte words)
          // For short strings data_len=0 → arr=[0, pending_word, pending_word_len]
          if (arr.length >= 3) {
            const dataLen = Number(BigInt(arr[0] || "0x0"));
            if (dataLen === 0) {
              // No full 31-byte words — entire string is in pending_word
              const pendingWord = BigInt(arr[1] || "0x0");
              const pendingLen = Number(BigInt(arr[2] || "0x0"));
              if (pendingLen > 0 && pendingLen <= 31 && pendingWord > 0n) {
                const bytes = [];
                let tmp = pendingWord;
                while (tmp > 0n) { bytes.unshift(Number(tmp & 0xffn)); tmp >>= 8n; }
                // Ensure exactly pendingLen bytes (left-pad if needed)
                while (bytes.length < pendingLen) bytes.unshift(0);
                return String.fromCharCode(...bytes.slice(-pendingLen).filter(b => b > 31 && b < 127)).trim();
              }
            } else {
              // Has full 31-byte words + optional pending
              // Decode all full words then pending
              let result = "";
              for (let i = 1; i <= dataLen; i++) {
                const w = BigInt(arr[i] || "0x0");
                const wBytes = [];
                let tmp = w;
                while (tmp > 0n) { wBytes.unshift(Number(tmp & 0xffn)); tmp >>= 8n; }
                while (wBytes.length < 31) wBytes.unshift(0);
                result += String.fromCharCode(...wBytes.filter(b => b > 31 && b < 127));
              }
              const pendingWord = BigInt(arr[dataLen + 1] || "0x0");
              const pendingLen = Number(BigInt(arr[dataLen + 2] || "0x0"));
              if (pendingLen > 0 && pendingWord > 0n) {
                const pBytes = [];
                let tmp = pendingWord;
                while (tmp > 0n) { pBytes.unshift(Number(tmp & 0xffn)); tmp >>= 8n; }
                while (pBytes.length < pendingLen) pBytes.unshift(0);
                result += String.fromCharCode(...pBytes.slice(-pendingLen).filter(b => b > 31 && b < 127));
              }
              if (result.trim()) return result.trim();
            }
          }
          // Fallback: felt252 single value (Cairo 0 style)
          const n = BigInt(arr[0] || "0x0");
          if (n === 0n) return "";
          const bytes = [];
          let tmp = n;
          while (tmp > 0n) { bytes.unshift(Number(tmp & 0xffn)); tmp >>= 8n; }
          return String.fromCharCode(...bytes.filter(b => b > 31 && b < 127)).trim();
        } catch { return ""; }
      };
      const sym = decodeFelt(symRes);
      const nam = decodeFelt(nameRes);
      const dec = decRes?.length ? Number(BigInt(decRes[0])) : 18;
      return { address: contract, symbol: sym || "", name: nam || sym || "", decimals: dec, verified: !!sym };
    });
  }

  return { address: contract, symbol: "", name: "", decimals: 18, verified: false };
}

// ─── Multichain token balance ─────────────────────────────────────────────────

export async function resolveTokenBalanceMultichain(nodeUrl, walletUrl, contract, holder, family) {
  const fam = String(family || "evm").toLowerCase();
  const urls = normalizeUrlList(nodeUrl);

  if (fam === "evm" || fam === "harmony") {
    return resolveTokenBalance(urls, walletUrl, contract, holder);
  }
  if ((fam === "sei" || fam === "injective") && String(contract || "").startsWith("0x")) {
    return resolveTokenBalance(urls, walletUrl, contract, holder);
  }

  if (fam === "solana") {
    const res = await tryUrls(urls, (url) => postJson(url, {
      jsonrpc: "2.0", id: 1, method: "getTokenAccountsByOwner",
      params: [holder, { mint: contract }, { encoding: "jsonParsed" }],
    }).catch(() => null)).catch(() => null);
    let total = 0n;
    for (const acc of (res?.result?.value || [])) {
      const amt = acc?.account?.data?.parsed?.info?.tokenAmount?.amount;
      if (amt) total += BigInt(amt);
    }
    return total.toString();
  }

  if (fam === "cosmos" || fam === "cosmos-testnet" || fam === "sei" || fam === "injective") {
    const prefix = fam === "sei" ? "sei1" : fam === "injective" ? "inj1" : "cosmos1";
    if (contract.startsWith(prefix) || contract.startsWith("osmo1") || contract.startsWith("stars1")) {
      // CW20 balance query
      try {
        const query = btoa(JSON.stringify({ balance: { address: holder } }));
        const res = await tryUrls(urls, (url) => getJson(`${normalizeUrl(url)}/cosmwasm/wasm/v1/contract/${contract}/smart/${encodeURIComponent(query)}`).catch(() => null)).catch(() => null);
        return res?.data?.balance || "0";
      } catch { /* fall through */ }
    }
    // IBC / native denom balance via bank module
    const res = await tryUrls(urls, (url) => getJson(`${normalizeUrl(url)}/cosmos/bank/v1beta1/balances/${holder}`).catch(() => null)).catch(() => null);
    const bal = (res?.balances || []).find(b => b.denom === contract);
    return bal?.amount || "0";
  }

  if (fam === "near") {
    const args = btoa(JSON.stringify({ account_id: holder }));
    const res = await tryUrls(urls, (url) => postJson(url, {
      jsonrpc: "2.0", id: 1, method: "query",
      params: { request_type: "call_function", finality: "final", account_id: contract, method_name: "ft_balance_of", args_base64: args },
    }).catch(() => null)).catch(() => null);
    if (res?.result?.result) {
      try { return JSON.parse(String.fromCharCode(...res.result.result)) || "0"; } catch { }
    }
    return "0";
  }

  if (fam === "aptos") {
    const aptosBase = (url) => {
      const normalized = normalizeUrl(url);
      return normalized.endsWith("/v1") ? normalized : `${normalized}/v1`;
    };
    const res = await tryUrls(urls, (url) => getJson(`${aptosBase(url)}/accounts/${encodeURIComponent(holder)}/resources`).catch(() => null)).catch(() => null);
    const resource = (Array.isArray(res) ? res : []).find(r => r.type === `0x1::coin::CoinStore<${contract}>`);
    return resource?.data?.coin?.value || "0";
  }

  if (fam === "sui") {
    const res = await tryUrls(urls, (url) => postJson(url, {
      jsonrpc: "2.0", id: 1, method: "suix_getBalance", params: [holder, contract],
    }).catch(() => null)).catch(() => null);
    return res?.result?.totalBalance || "0";
  }

  if (fam === "tron") {
    // TRC20: standard eth_call works on Tron JSON-RPC (needs 0x EVM address, not T... base58)
    const evmHolder = tronAddressToEvm(holder);
    const holderHex = evmHolder.replace(/^0x/, "").padStart(64, "0");
    const data = "0x70a08231" + holderHex;
    const res = await tryUrls(urls, (url) => postJson(url, {
      jsonrpc: "2.0", id: Date.now(), method: "eth_call",
      params: [{ to: contract, data }, "latest"],
    }).catch(() => null)).catch(() => null);
    return res?.result ? BigInt(res.result).toString() : "0";
  }

  if (fam === "ton") {
    if (!holder) return "0";
    const primaryUrl = urls[0] || "";
    const isTestnet = primaryUrl.includes("testnet");
    const v2Base = isTestnet ? "https://testnet.toncenter.com/api/v2" : "https://toncenter.com/api/v2";
    try {
      // Get Jetton wallet address for this holder from master contract
      const jwRes = await getJson(
        `${v2Base}/runGetMethod?address=${encodeURIComponent(contract)}&method=get_wallet_address&stack=[["tvm.Slice","${holder}"]]`,
        { timeoutMs: 8000 }
      ).catch(() => null);
      const stackItem = jwRes?.result?.stack?.[0];
      // stack item is ["cell", { object: { data: { b64: "..." } } }] or ["num", "0x..."]
      const rawAddr = stackItem?.[1]?.object?.data?.b64 || stackItem?.[1];
      if (rawAddr && typeof rawAddr === "string" && rawAddr.length > 10) {
        const balRes = await getJson(
          `${v2Base}/runGetMethod?address=${encodeURIComponent(rawAddr)}&method=get_wallet_data&stack=[]`,
          { timeoutMs: 8000 }
        ).catch(() => null);
        const balStack = balRes?.result?.stack?.[0];
        if (balStack) {
          const raw = balStack?.[1];
          if (typeof raw === "string") return BigInt(raw).toString();
          if (typeof raw === "number") return String(raw);
        }
      }
    } catch { }
    return "0";
  }

  if (fam === "starknet") {
    const BALANCE_OF = "0x2e4263afad30923c891518314c3c95dbe830a16874e8abc5777a9a20b54c76e";
    const res = await tryUrls(urls, async (url) => {
      const v1 = await postJson(url, {
        jsonrpc: "2.0", id: 1, method: "starknet_call",
        params: [{ contract_address: contract, entry_point_selector: BALANCE_OF, calldata: [holder] }, { block_tag: "latest" }],
      }).catch(() => null);
      if (v1?.result) return v1;
      return postJson(url, {
        jsonrpc: "2.0", id: 1, method: "starknet_call",
        params: [{ contract_address: contract, entry_point_selector: BALANCE_OF, calldata: [holder] }, "latest"],
      }).catch(() => null);
    }).catch(() => null);
    return res?.result?.[0] ? BigInt(res.result[0]).toString() : "0";
  }

  return "0";
}

// ─── Chain-specific token ownership discovery ─────────────────────────────────

/**
 * Solana: returns all SPL token accounts owned by holder.
 * Result: [{address: mintAddress, balance: rawAmountString}]
 */
export async function discoverSolanaTokens(nodeUrl, holder) {
  const SPL_TOKEN_PROGRAM = "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA";
  const res = await postJson(normalizeUrl(nodeUrl), {
    jsonrpc: "2.0", id: 1, method: "getTokenAccountsByOwner",
    params: [holder, { programId: SPL_TOKEN_PROGRAM }, { encoding: "jsonParsed" }],
  }).catch(() => null);
  const out = [];
  for (const acc of (res?.result?.value || [])) {
    const info = acc?.account?.data?.parsed?.info;
    if (!info?.mint) continue;
    const amount = info?.tokenAmount?.amount || "0";
    out.push({ address: info.mint, balance: amount });
  }
  return out;
}

/**
 * Cosmos / SEI / INJ: returns all bank module balances (native + IBC denoms).
 * Also discovers CW20 contracts held via a query of known contracts is not needed —
 * the bank module covers native and IBC; CW20 requires an indexer.
 * Result: [{address: denom, balance: amountString}]
 */
export async function discoverCosmosTokens(nodeUrl, holder) {
  const res = await getJson(
    `${normalizeUrl(nodeUrl)}/cosmos/bank/v1beta1/balances/${holder}?pagination.limit=200`
  ).catch(() => null);
  return (res?.balances || [])
    .filter(b => b?.denom && b.denom !== "ulqd") // skip the chain's native denom (shown separately)
    .map(b => ({ address: b.denom, balance: b.amount || "0" }));
}

/**
 * Aptos: reads all CoinStore<T> resources from the account.
 * Result: [{address: coinType (e.g. "0x1::aptos_coin::AptosCoin"), balance: valueString}]
 */
export async function discoverAptosCoins(nodeUrl, holder) {
  const res = await getJson(
    `${normalizeUrl(nodeUrl)}/accounts/${encodeURIComponent(holder)}/resources?limit=100`
  ).catch(() => null);
  const NATIVE = "0x1::aptos_coin::AptosCoin";
  return (Array.isArray(res) ? res : [])
    .filter(r => r?.type?.startsWith("0x1::coin::CoinStore<") && !r.type.includes(NATIVE))
    .map(r => {
      const coinType = r.type.replace("0x1::coin::CoinStore<", "").replace(/>$/, "");
      return { address: coinType, balance: r?.data?.coin?.value || "0" };
    });
}

/**
 * SUI: returns all coin balances via suix_getAllBalances.
 * Result: [{address: coinType, balance: totalBalanceString}]
 */
export async function discoverSuiCoins(nodeUrl, holder) {
  const res = await postJson(normalizeUrl(nodeUrl), {
    jsonrpc: "2.0", id: 1, method: "suix_getAllBalances", params: [holder],
  }).catch(() => null);
  const NATIVE = "0x2::sui::SUI";
  return (res?.result || [])
    .filter(c => c?.coinType && c.coinType !== NATIVE)
    .map(c => ({ address: c.coinType, balance: c.totalBalance || "0" }));
}

/**
 * NEAR: checks a curated list of popular NEP-141 token contracts for a non-zero balance.
 * (NEAR has no on-chain "list all tokens for account" without an indexer.)
 * Result: [{address: contractId, balance: rawAmountString}]
 */
export async function discoverNearTokens(nodeUrl, holder) {
  const POPULAR = [
    "usdc.fakes.testnet",
    "usdt.fakes.testnet",
    "wrap.testnet",
    "ref.fakes.testnet",
    "aurora",
    "meta-token.near",
    "token.v2.ref-finance.near",
    "token.sweat",
    "ftv2.nekotoken.near",
    "usn",
  ];
  const results = [];
  await Promise.all(POPULAR.map(async (contract) => {
    try {
      const args = btoa(JSON.stringify({ account_id: holder }));
      const res = await postJson(normalizeUrl(nodeUrl), {
        jsonrpc: "2.0", id: 1, method: "query",
        params: { request_type: "call_function", finality: "final", account_id: contract, method_name: "ft_balance_of", args_base64: args },
      }).catch(() => null);
      if (!res?.result?.result) return;
      const balance = JSON.parse(String.fromCharCode(...res.result.result));
      if (balance && balance !== "0") results.push({ address: contract, balance: String(balance) });
    } catch { /* skip */ }
  }));
  return results;
}
