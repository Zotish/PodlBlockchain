function normalizeBaseUrl(value, fallback) {
  const raw = (value || fallback || "").trim();
  return raw.replace(/\/+$/, "");
}

export const API_BASE = normalizeBaseUrl(
	import.meta.env.REACT_APP_API_BASE || import.meta.env.VITE_API_BASE,
  "https://api.178-105-133-94.sslip.io"
);

export const CHAIN_BASE = normalizeBaseUrl(
	import.meta.env.REACT_APP_CHAIN_BASE || import.meta.env.VITE_CHAIN_BASE,
  "https://chain.178-105-133-94.sslip.io"
);

export const WALLET_BASE = normalizeBaseUrl(
	import.meta.env.REACT_APP_WALLET_BASE || import.meta.env.VITE_WALLET_BASE,
  "https://wallet.178-105-133-94.sslip.io"
);

export const WEB_WALLET_BASE = normalizeBaseUrl(
	import.meta.env.REACT_APP_WEB_WALLET_BASE || import.meta.env.VITE_WEB_WALLET_BASE,
  "http://127.0.0.1:3000"
);

export const DEX_REGISTRY_BASE = normalizeBaseUrl(
	import.meta.env.REACT_APP_DEX_REGISTRY_API || import.meta.env.VITE_DEX_REGISTRY_API,
  "https://dex-api.178-105-133-94.sslip.io"
);

export function apiUrl(base, path) {
  const normalizedPath = path.startsWith("/") ? path : `/${path}`;
  return `${base}${normalizedPath}`;
}

const requestCache = new Map();
const inFlightRequests = new Map();
const MAX_CACHE_ENTRIES = 128;
const MAX_CACHE_BYTES = 8 * 1024 * 1024;
const MAX_RESPONSE_BYTES = 4 * 1024 * 1024;
let cacheBytes = 0;
let expiryTimer;

function evictCache(key) {
  const entry = requestCache.get(key);
  if (entry) cacheBytes -= entry.bytes;
  requestCache.delete(key);
}

function expireCache() {
  clearTimeout(expiryTimer);
  const now = Date.now();
  let next = Infinity;
  for (const [key, entry] of requestCache) {
    if (entry.expiresAt <= now) evictCache(key);
    else next = Math.min(next, entry.expiresAt);
  }
  if (Number.isFinite(next)) {
    expiryTimer = setTimeout(expireCache, Math.max(1, next - now));
    expiryTimer.unref?.();
  }
}

export function clearRequestCache() {
  clearTimeout(expiryTimer);
  requestCache.clear();
  cacheBytes = 0;
}

export function requestCacheStats() {
  expireCache();
  return { entries: requestCache.size, bytes: cacheBytes };
}

async function responseJSON(response, signal) {
  const declared = Number(response.headers?.get('content-length'));
  if (declared > MAX_RESPONSE_BYTES) throw new Error('Response exceeds the explorer safety limit');
  if (!response.body?.getReader) return response.json();
  const reader = response.body.getReader();
  const cancel = () => { void reader.cancel().catch(() => {}); };
  signal.addEventListener('abort', cancel, { once: true });
  const decoder = new TextDecoder();
  let bytes = 0;
  let body = '';
  try {
    while (true) {
      if (signal.aborted) throw signal.reason;
      const part = await reader.read();
      if (part.done) break;
      bytes += part.value.byteLength;
      if (bytes > MAX_RESPONSE_BYTES) {
        cancel();
        throw new Error('Response exceeds the explorer safety limit');
      }
      body += decoder.decode(part.value, { stream: true });
    }
    return JSON.parse(body + decoder.decode());
  } finally {
    signal.removeEventListener('abort', cancel);
    reader.releaseLock();
  }
}

function requestKey(base, path, options = {}) {
  const method = String(options.method || "GET").toUpperCase();
  const body = options.body ? String(options.body) : "";
  return `${method}:${apiUrl(base, path)}:${body}:${options.timeoutMs || 12000}:${options.cacheTtlMs || 0}`;
}

export async function fetchJSON(path, options = {}) {
  const {
    base = API_BASE,
    cacheTtlMs = 0,
    timeoutMs = 12000,
    ...fetchOptions
  } = options || {};
  const method = String(fetchOptions.method || "GET").toUpperCase();
  const key = requestKey(base, path, { ...fetchOptions, timeoutMs, cacheTtlMs });
  const headers = new Headers(fetchOptions.headers);
  // Never share authenticated requests or cancellation between independent callers.
  const shareable = method === 'GET' && !fetchOptions.signal && [...headers].length === 0 && fetchOptions.credentials !== 'include';
  if (fetchOptions.signal?.aborted) throw fetchOptions.signal.reason || new DOMException('Cancelled', 'AbortError');
  expireCache();

  if (shareable && cacheTtlMs > 0) {
    const cached = requestCache.get(key);
    if (cached) {
      requestCache.delete(key);
      requestCache.set(key, cached);
      return structuredClone(cached.data);
    }
  }

  if (shareable && inFlightRequests.has(key)) {
    return structuredClone(await inFlightRequests.get(key));
  }

  const controller = new AbortController();
  const cancel = () => controller.abort(fetchOptions.signal.reason || new DOMException('Cancelled', 'AbortError'));
  fetchOptions.signal?.addEventListener('abort', cancel, { once: true });
  const deadline = Number.isFinite(timeoutMs) ? Math.max(1, Math.min(60000, timeoutMs)) : 12000;
  const timer = setTimeout(() => controller.abort(new DOMException('Request timed out', 'TimeoutError')), deadline);
  let rejectAborted;
  const aborted = new Promise((_, reject) => { rejectAborted = () => reject(controller.signal.reason); });
  controller.signal.addEventListener('abort', rejectAborted, { once: true });
  const work = async () => {
    const res = await fetch(apiUrl(base, path), { ...fetchOptions, signal: controller.signal });
    const data = await responseJSON(res, controller.signal);
    if (controller.signal.aborted) throw controller.signal.reason;
    if (!res.ok) throw new Error(data?.error || `Request failed (${res.status})`);
    if (shareable && cacheTtlMs > 0) {
      const bytes = new TextEncoder().encode(JSON.stringify(data)).byteLength;
      if (bytes <= MAX_RESPONSE_BYTES) {
        evictCache(key);
        requestCache.set(key, { data: structuredClone(data), bytes, expiresAt: Date.now() + Math.min(cacheTtlMs, 300000) });
        cacheBytes += bytes;
        while (requestCache.size > MAX_CACHE_ENTRIES || cacheBytes > MAX_CACHE_BYTES) evictCache(requestCache.keys().next().value);
        expireCache();
      }
    }
    return data;
  };
  const promise = Promise.race([work(), aborted]).finally(() => {
    clearTimeout(timer);
    fetchOptions.signal?.removeEventListener('abort', cancel);
    controller.signal.removeEventListener('abort', rejectAborted);
    if (shareable && inFlightRequests.get(key) === promise) inFlightRequests.delete(key);
  });

  if (shareable) inFlightRequests.set(key, promise);
  return shareable ? structuredClone(await promise) : promise;
}

export async function fetchChainJSON(path, options = {}) {
  return fetchJSON(path, { ...options, base: CHAIN_BASE });
}

export async function fetchWalletJSON(path, options = {}) {
  return fetchJSON(path, { ...options, base: WALLET_BASE });
}

export async function fetchDexRegistryJSON(path, options = {}) {
  return fetchJSON(path, { ...options, base: DEX_REGISTRY_BASE });
}

export async function fetchDexRegistryTokens(options = {}) {
  const data = await fetchDexRegistryJSON("/tokens", { cacheTtlMs: 10000, timeoutMs: 10000, ...options });
  return Array.isArray(data) ? data : [];
}

export async function fetchDexRegistryPools(options = {}) {
  const data = await fetchDexRegistryJSON("/pools", { cacheTtlMs: 10000, timeoutMs: 10000, ...options });
  return Array.isArray(data) ? data : [];
}

export function extractBlocks(data) {
  const primary = firstNodeResult(data);
  if (Array.isArray(primary)) return primary;
  if (primary && Array.isArray(primary.blocks)) return primary.blocks;
  if (data && Array.isArray(data.blocks)) return data.blocks;
  if (Array.isArray(data)) return data;
  return mergeArrayResults(data, "block_number");
}

export function transactionsFromBlocks(blocks, limit = 20) {
  const seen = new Map();
  (blocks || []).forEach((block) => {
    const blockNumber = block.block_number ?? block.BlockNumber;
    const blockHash = block.current_hash ?? block.CurrentHash;
    const txs = block.transactions ?? block.Transactions ?? [];
    if (!Array.isArray(txs)) return;
    txs.forEach((tx, index) => {
      const hash = tx.tx_hash || tx.txHash || tx.TxHash || tx.hash;
      if (!hash) return;
      seen.set(hash, {
        ...tx,
        block_number: tx.block_number ?? blockNumber,
        block_hash: tx.block_hash ?? blockHash,
        tx_index: tx.tx_index ?? index,
        timestamp: tx.timestamp ?? block.timestamp ?? block.TimeStamp,
      });
    });
  });
  return Array.from(seen.values())
    .sort((a, b) => (b.timestamp ?? 0) - (a.timestamp ?? 0))
    .slice(0, limit);
}

export async function fetchRecentBlocks(count = 14, options = {}) {
  const data = await fetchJSON(`/fetch_last_n_block?n=${count}`, {
    cacheTtlMs: 1500,
    ...options,
  });
  return extractBlocks(data)
    .sort((a, b) => (b.block_number ?? b.BlockNumber ?? 0) - (a.block_number ?? a.BlockNumber ?? 0))
    .slice(0, count);
}

export async function fetchRecentTransactions(limit = 20, options = {}) {
  try {
    const data = await fetchJSON(`/transactions?page=1&size=${limit}`, {
      cacheTtlMs: 1500,
      timeoutMs: 10000,
      ...options,
    });
    const primary = firstNodeResult(data);
    const txs = primary?.transactions || data?.transactions || [];
    const hasTransactionPage =
      Array.isArray(primary?.transactions) ||
      Array.isArray(data?.transactions) ||
      primary?.total === 0 ||
      data?.total === 0;
    if (hasTransactionPage && Array.isArray(txs)) return txs.slice(0, limit);
  } catch {}
  const blocks = await fetchRecentBlocks(Math.max(limit, 20), options);
  return transactionsFromBlocks(blocks, limit);
}

export async function fetchAllHistoricalTransactions(options = {}) {
  const pageSize = options.pageSize || 200;
  const firstPage = await fetchJSON(`/fetch_last_n_block?page=1&size=${pageSize}`, {
    cacheTtlMs: 3000,
    timeoutMs: 15000,
    ...options,
  });
  const primary = firstNodeResult(firstPage);
  const firstBlocks = extractBlocks(primary || firstPage);
  const totalPages = Math.max(
    1,
    Number(primary?.total_pages || firstPage?.total_pages || 1)
  );

  if (totalPages === 1) {
    return transactionsFromBlocks(firstBlocks, Number.POSITIVE_INFINITY);
  }

  const pages = [];
  for (let page = 2; page <= totalPages; page += 1) pages.push(page);

  const remainingBlocks = [];
  const concurrency = 4;
  for (let i = 0; i < pages.length; i += concurrency) {
    const chunk = pages.slice(i, i + concurrency);
    const results = await Promise.all(
      chunk.map((page) =>
        fetchJSON(`/fetch_last_n_block?page=${page}&size=${pageSize}`, {
          cacheTtlMs: 3000,
          timeoutMs: 15000,
          ...options,
        }).catch(() => null)
      )
    );
    results.forEach((data) => {
      if (!data) return;
      const result = firstNodeResult(data);
      remainingBlocks.push(...extractBlocks(result || data));
    });
  }

  return transactionsFromBlocks(
    [...firstBlocks, ...remainingBlocks],
    Number.POSITIVE_INFINITY
  );
}

export async function fetchHistoricalTransactionPage(page = 1, pageSize = 10, options = {}) {
  try {
    const data = await fetchJSON(`/transactions?page=${page}&size=${pageSize}`, {
      cacheTtlMs: 1500,
      timeoutMs: 10000,
      ...options,
    });
    const primary = firstNodeResult(data);
    const transactions = primary?.transactions || data?.transactions;
    const hasTransactionPage =
      Array.isArray(transactions) ||
      primary?.total === 0 ||
      data?.total === 0;
    if (hasTransactionPage) {
      return {
        transactions: Array.isArray(transactions) ? transactions : [],
        total: Number(primary?.total || data?.total || 0),
        totalPages: Math.max(1, Number(primary?.total_pages || data?.total_pages || 1)),
      };
    }
  } catch {}

  const fallback = await fetchJSON(`/fetch_last_n_block?page=${page}&size=${pageSize}`, {
    cacheTtlMs: 1500,
    timeoutMs: 10000,
    ...options,
  });
  const primary = firstNodeResult(fallback);
  const blocks = extractBlocks(primary || fallback);
  return {
    transactions: transactionsFromBlocks(blocks, pageSize),
    total: Number(primary?.total || fallback?.total || blocks.length || 0),
    totalPages: Math.max(1, Number(primary?.total_pages || fallback?.total_pages || 1)),
  };
}

/*
export async function fetchJSON(path, options) {
  const res = await fetch(apiUrl(API_BASE, path), options);
  if (!res.ok) {
    // Try to read the JSON error body for a friendly message
    try {
      const data = await res.json();
      if (data && data.error) throw new Error(data.error);
    } catch (inner) {
      if (inner.message && inner.message !== 'Failed to fetch') throw inner;
    }
    throw new Error(`Request failed (${res.status})`);
  }
  return res.json();
}
*/

export function nodeResults(data) {
  if (Array.isArray(data)) {
    return data;
  }
  if (data && Array.isArray(data.nodes)) {
    return data.nodes
      .map((n) => n.result || n.summary)
      .filter((n) => n !== undefined && n !== null);
  }
  return [];
}

export function firstNodeResult(data) {
  if (Array.isArray(data)) {
    return data;
  }
  if (data && Array.isArray(data.nodes) && data.nodes.length > 0) {
    const found = data.nodes.find(
      (n) => n && (n.result !== undefined || n.summary !== undefined)
    );
    if (found) {
      return found.result || found.summary || null;
    }
    return null;
  }
  return data || null;
}

export function mergeArrayResults(data, key) {
  const results = nodeResults(data);
  const flat = [];
  results.forEach((entry) => {
    if (Array.isArray(entry)) {
      flat.push(...entry);
      return;
    }
    if (entry && Array.isArray(entry.transactions)) {
      flat.push(...entry.transactions);
      return;
    }
    if (entry && Array.isArray(entry.blocks)) {
      flat.push(...entry.blocks);
    }
  });

  if (!key) {
    return flat;
  }

  const seen = new Map();
  flat.forEach((item) => {
    const k =
      item?.[key] ||
      item?.[key.toLowerCase()] ||
      item?.address ||
      item?.tx_hash ||
      item?.txHash ||
      item?.block_number;
    if (k === undefined || k === null) {
      seen.set(Math.random().toString(36), item);
      return;
    }
    seen.set(k, item);
  });
  return Array.from(seen.values());
}

export async function waitForTx(txHash, timeoutMs = 20000) {
  if (!txHash) return null;
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    await new Promise((r) => setTimeout(r, 1200));
    try {
      const res = await fetchJSON(`/tx/${encodeURIComponent(txHash)}`);
      if (res && (res.tx_hash || res.TxHash || res.hash)) return res;
    } catch {}
  }
  return null;
}
