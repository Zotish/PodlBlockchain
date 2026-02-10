export const API_BASE = "http://127.0.0.1:9000";

export async function fetchJSON(path, options) {
  const res = await fetch(`${API_BASE}${path}`, options);
  if (!res.ok) {
    throw new Error(`${path} -> ${res.status}`);
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
