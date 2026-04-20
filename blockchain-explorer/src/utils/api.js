function normalizeBaseUrl(value, fallback) {
  const raw = (value || fallback || "").trim();
  return raw.replace(/\/+$/, "");
}

export const API_BASE = normalizeBaseUrl(
  process.env.REACT_APP_API_BASE,
  "http://127.0.0.1:9000"
);

export const CHAIN_BASE = normalizeBaseUrl(
  process.env.REACT_APP_CHAIN_BASE,
  "http://127.0.0.1:6500"
);

export const WALLET_BASE = normalizeBaseUrl(
  process.env.REACT_APP_WALLET_BASE,
  "http://127.0.0.1:8080"
);

export const WEB_WALLET_BASE = normalizeBaseUrl(
  process.env.REACT_APP_WEB_WALLET_BASE,
  "http://127.0.0.1:3000"
);

export function apiUrl(base, path) {
  const normalizedPath = path.startsWith("/") ? path : `/${path}`;
  return `${base}${normalizedPath}`;
}

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
