import { normalizeUrl } from "./api";

const FINAL_STATUSES = new Set(["confirmed", "failed", "dropped"]);
const TRACKED_TX_LIMIT = 100;
const MAX_PENDING_AGE_MS = 24 * 60 * 60 * 1000;
const MIN_RECHECK_INTERVAL_MS = 20 * 1000;

export function createTrackedTransaction({ hash, networkId, family, type, symbol, to }) {
  const cleanHash = String(hash || "").trim();
  if (!cleanHash) return null;
  return {
    id: `${networkId || "unknown"}:${cleanHash.toLowerCase()}`,
    hash: cleanHash,
    networkId: networkId || "unknown",
    family: String(family || "evm").toLowerCase(),
    type: type || "transaction",
    symbol: symbol || "",
    to: to || "",
    status: "pending",
    createdAt: Date.now(),
    updatedAt: Date.now(),
    lastCheckedAt: 0,
    attempts: 0,
    expiresAt: Date.now() + MAX_PENDING_AGE_MS,
    history: [{ status: "pending", at: Date.now() }],
  };
}

export function mergeTrackedTransaction(list, tx) {
  if (!tx?.id) return Array.isArray(list) ? list : [];
  const current = Array.isArray(list) ? list : [];
  const existing = current.find((item) => item?.id === tx.id);
  const merged = existing ? { ...existing, ...tx, history: mergeHistory(existing.history, tx.history, tx.status) } : tx;
  const without = current.filter((item) => item?.id !== tx.id);
  return [merged, ...without].slice(0, TRACKED_TX_LIMIT);
}

function isFinal(item) {
  return FINAL_STATUSES.has(String(item?.status || "").toLowerCase());
}

function mergeHistory(oldHistory = [], newHistory = [], status = "") {
  const base = Array.isArray(oldHistory) ? oldHistory : [];
  const next = Array.isArray(newHistory) ? newHistory : [];
  const merged = [...base, ...next];
  if (status && !merged.some((item) => item.status === status)) {
    merged.push({ status, at: Date.now() });
  }
  return merged
    .filter((item) => item?.status)
    .sort((a, b) => Number(a.at || 0) - Number(b.at || 0))
    .slice(-12);
}

function withStatus(tx, status, extra = {}) {
  const previous = String(tx?.status || "");
  const history = Array.isArray(tx?.history) ? [...tx.history] : [];
  if (status && previous !== status) history.push({ status, at: Date.now() });
  return {
    ...tx,
    ...extra,
    status: status || previous || "pending",
    updatedAt: Date.now(),
    lastCheckedAt: Date.now(),
    attempts: Number(tx?.attempts || 0) + 1,
    history: history.slice(-12),
  };
}

async function evmReceipt(tx, helpers) {
  const res = await helpers.postJson(tx.nodeUrl, {
    jsonrpc: "2.0",
    id: 1,
    method: "eth_getTransactionReceipt",
    params: [tx.hash],
  });
  const receipt = res?.result;
  if (!receipt) return null;
  return receipt.status === "0x0" ? "failed" : "confirmed";
}

export async function checkTrackedTransaction(tx, helpers) {
  if (!tx?.hash || isFinal(tx)) return tx;
  const now = Date.now();
  if (Number(tx.lastCheckedAt || 0) && now - Number(tx.lastCheckedAt || 0) < MIN_RECHECK_INTERVAL_MS) {
    return tx;
  }
  if (Number(tx.expiresAt || 0) && now > Number(tx.expiresAt || 0)) {
    return withStatus(tx, "dropped", { lastError: "Transaction was not confirmed before the tracking window expired." });
  }
  const family = String(tx.family || "evm").toLowerCase();
  let status = "";

  try {
    if (family === "lqd") {
      const res = await helpers.getJson(`${normalizeUrl(tx.nodeUrl)}/tx/${encodeURIComponent(tx.hash)}`);
      if (res?.transaction || res?.block_number || res?.status === "confirmed") status = "confirmed";
    } else if (family === "evm" || family === "harmony" || family === "tron" || family === "sei") {
      status = await evmReceipt(tx, helpers);
    } else if (family === "solana") {
      const res = await helpers.postJson(tx.nodeUrl, {
        jsonrpc: "2.0",
        id: 1,
        method: "getSignatureStatuses",
        params: [[tx.hash], { searchTransactionHistory: true }],
      });
      const value = res?.result?.value?.[0];
      if (value?.confirmationStatus === "finalized") status = value?.err ? "failed" : "confirmed";
    } else if (family === "aptos") {
      const res = await helpers.getJson(`${normalizeUrl(tx.nodeUrl)}/transactions/by_hash/${tx.hash}`);
      if (res?.success === true) status = "confirmed";
      if (res?.success === false) status = "failed";
    } else if (family === "sui") {
      const res = await helpers.postJson(tx.nodeUrl, {
        jsonrpc: "2.0",
        id: 1,
        method: "sui_getTransactionBlock",
        params: [tx.hash, { showEffects: true }],
      });
      const txStatus = res?.result?.effects?.status?.status;
      if (txStatus === "success") status = "confirmed";
      if (txStatus === "failure") status = "failed";
    } else if (family === "starknet") {
      const res = await helpers.postJson(tx.nodeUrl, {
        jsonrpc: "2.0",
        id: 1,
        method: "starknet_getTransactionReceipt",
        params: [tx.hash],
      });
      const finality = String(res?.result?.finality_status || "").toLowerCase();
      const execution = String(res?.result?.execution_status || "").toLowerCase();
      if (finality.includes("accepted") && !execution.includes("reverted")) status = "confirmed";
      if (execution.includes("reverted")) status = "failed";
    }
  } catch (error) {
    return withStatus(tx, tx.status || "pending", { lastError: error?.message || "status check failed" });
  }

  return status ? withStatus(tx, status, { lastError: "" }) : withStatus(tx, tx.status || "pending");
}

export async function refreshTrackedTransactions(transactions, helpers) {
  const list = Array.isArray(transactions) ? transactions : [];
  const open = list.filter((tx) => !isFinal(tx));
  const closed = list.filter(isFinal);
  const checked = await Promise.all(open.map((tx) => checkTrackedTransaction(tx, helpers)));
  return [...checked, ...closed]
    .sort((a, b) => Number(b.updatedAt || b.createdAt || 0) - Number(a.updatedAt || a.createdAt || 0))
    .slice(0, TRACKED_TX_LIMIT);
}
