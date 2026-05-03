export function shortAddress(address, left = 6, right = 4) {
  if (!address) return "";
  const clean = String(address).trim();
  if (clean.length <= left + right + 3) return clean;
  return `${clean.slice(0, left)}…${clean.slice(-right)}`;
}

export function safeJsonParse(value, fallback = null) {
  if (typeof value !== "string" || !value.trim()) return fallback;
  try {
    return JSON.parse(value);
  } catch {
    return fallback;
  }
}

export function formatDate(value) {
  const raw = Number(value || 0);
  if (!raw) return "Just now";
  const ms = raw < 1e12 ? raw * 1000 : raw;
  const diff = Date.now() - ms;
  if (diff < 60000) return `${Math.max(1, Math.floor(diff / 1000))}s ago`;
  if (diff < 3600000) return `${Math.floor(diff / 60000)}m ago`;
  if (diff < 86400000) return `${Math.floor(diff / 3600000)}h ago`;
  return new Date(ms).toLocaleString();
}

export function normalizeHexAddress(value) {
  const clean = String(value || "").trim();
  return clean.length ? clean : "";
}

export function isLikelyAddress(value) {
  return /^0x[a-fA-F0-9]{40}$/.test(String(value || "").trim());
}

export function parseUnits(input, decimals = 8) {
  const raw = String(input ?? "").trim();
  if (!raw || raw === ".") return "0";
  if (!Number.isFinite(Number(raw))) return "0";

  const sign = raw.startsWith("-") ? -1n : 1n;
  const clean = raw.replace(/^-/, "");
  const [wholePart, fracPart = ""] = clean.split(".");
  const whole = wholePart.replace(/[^\d]/g, "") || "0";
  const frac = fracPart.replace(/[^\d]/g, "").slice(0, Math.max(0, decimals)).padEnd(Math.max(0, decimals), "0");
  const base = 10n ** BigInt(Math.max(0, decimals));
  const value = (BigInt(whole) * base) + BigInt(frac || "0");
  return (value * sign).toString();
}

export function formatUnits(raw, decimals = 8, maxFractionDigits = 6) {
  try {
    const value = BigInt(String(raw || "0"));
    const sign = value < 0n ? "-" : "";
    const abs = value < 0n ? -value : value;
    const base = 10n ** BigInt(Math.max(0, decimals));
    const whole = abs / base;
    const frac = abs % base;
    if (decimals <= 0) return `${sign}${whole.toString()}`;
    const fracStr = frac.toString().padStart(decimals, "0").slice(0, Math.max(0, maxFractionDigits)).replace(/0+$/, "");
    return fracStr ? `${sign}${whole.toString()}.${fracStr}` : `${sign}${whole.toString()}`;
  } catch {
    return String(raw ?? "0");
  }
}

export function mergeUniqueByKey(list, next, key = "address") {
  const out = [...list];
  for (const item of next) {
    const idx = out.findIndex((x) => String(x?.[key] || "").toLowerCase() === String(item?.[key] || "").toLowerCase());
    if (idx >= 0) out[idx] = { ...out[idx], ...item };
    else out.push(item);
  }
  return out;
}

export function txTouchesAddress(tx, address) {
  if (!tx || !address) return false;
  const a = String(address).trim().toLowerCase();
  const from = String(tx.From || tx.from || tx.sender || "").trim().toLowerCase();
  const to = String(tx.To || tx.to || tx.recipient || "").trim().toLowerCase();
  const contract = String(tx.Contract || tx.contract || "").trim().toLowerCase();
  return from === a || to === a || contract === a;
}

