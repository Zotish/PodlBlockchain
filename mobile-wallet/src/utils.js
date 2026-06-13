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

export function isLikelyAddressForFamily(value, family) {
  const v = String(value || "").trim();
  if (!v) return false;
  switch (String(family || "").toLowerCase()) {
    case "evm":       return /^0x[a-fA-F0-9]{40}$/.test(v);
    case "solana":    return /^[1-9A-HJ-NP-Za-km-z]{32,44}$/.test(v);
    case "cosmos":
    case "cosmos-testnet": return v.startsWith("cosmos1") || v.startsWith("ibc/") || v.length >= 10;
    case "sei":       return v.startsWith("sei1") || v.length >= 10;
    case "injective": return v.startsWith("inj1") || v.length >= 10;
    case "harmony":   return v.startsWith("one1") || /^0x[a-fA-F0-9]{40}$/.test(v);
    case "near":      return v.length >= 2 && v.length <= 64 && /^[\w.\-]+$/.test(v);
    case "tron":      return /^T[A-Za-z1-9]{33}$/.test(v);
    case "ton":       return (v.startsWith("EQ") || v.startsWith("UQ")) && v.length >= 48;
    case "aptos":
    case "sui":
    case "starknet":  return v.startsWith("0x") && v.length >= 10;
    case "utxo":
    case "litecoin":  return false;
    default:          return v.length >= 10;
  }
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
    const idx = out.findIndex((x) => {
      const matchKey = String(x?.[key] || "").toLowerCase() === String(item?.[key] || "").toLowerCase();
      // If networkId is present, both must match
      if (item.networkId && x.networkId) {
        return matchKey && x.networkId === item.networkId;
      }
      return matchKey;
    });
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

// ─── TRON address conversion ──────────────────────────────────────────────────

/**
 * Convert a TRON T... base58 address to its 0x EVM-format equivalent.
 * TRON and Ethereum share the same secp256k1 key; the 20-byte address is identical,
 * just encoded differently (base58check with 0x41 prefix vs 0x hex).
 */
export function tronAddressToEvm(addr) {
  const s = String(addr || "").trim();
  if (!s.startsWith("T") || s.length !== 34) return s; // already 0x or unknown
  const B58 = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz";
  let num = 0n;
  for (const c of s) {
    const i = B58.indexOf(c);
    if (i < 0) return s;
    num = num * 58n + BigInt(i);
  }
  // 25 bytes: version(1=0x41) + address(20) + checksum(4)
  const hex = num.toString(16).padStart(50, "0");
  return "0x" + hex.slice(2, 42); // skip first byte (0x41), take 20 address bytes
}

// ─── Cryptographic address derivation ────────────────────────────────────────

function _hexToBytes(hex) {
  const h = hex.replace(/^0x/, "");
  const out = new Uint8Array(h.length / 2);
  for (let i = 0; i < out.length; i++) out[i] = parseInt(h.slice(i * 2, i * 2 + 2), 16);
  return out;
}

function _bytesToHex(bytes) {
  return Array.from(bytes).map(b => b.toString(16).padStart(2, "0")).join("");
}

// secp256k1 constants
const _P = 0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEFFFFFC2Fn;
const _GX = 0x79BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798n;
const _GY = 0x483ADA7726A3C4655DA4FBFC0E1108A8FD17B448A68554199C47D08FFB10D4B8n;

function _modpow(b, e, m) {
  let r = 1n; b %= m;
  for (; e > 0n; e >>= 1n) { if (e & 1n) r = r * b % m; b = b * b % m; }
  return r;
}

function _pointAdd(p1, p2) {
  if (!p1) return p2;
  if (!p2) return p1;
  const [x1, y1] = p1, [x2, y2] = p2;
  if (x1 === x2) {
    if (y1 !== y2) return null;
    const lam = 3n * x1 * x1 % _P * _modpow(2n * y1 % _P, _P - 2n, _P) % _P;
    const x3 = (lam * lam % _P - 2n * x1 + _P * 2n) % _P;
    return [x3, (lam * ((x1 - x3 + _P) % _P) % _P - y1 + _P) % _P];
  }
  const lam = (y2 - y1 + _P) % _P * _modpow((x2 - x1 + _P) % _P, _P - 2n, _P) % _P;
  const x3 = (lam * lam % _P - x1 - x2 + _P * 2n) % _P;
  return [x3, (lam * ((x1 - x3 + _P) % _P) % _P - y1 + _P) % _P];
}

function _pointMul(k) {
  let res = null, cur = [_GX, _GY];
  for (; k > 0n; k >>= 1n) { if (k & 1n) res = _pointAdd(res, cur); cur = _pointAdd(cur, cur); }
  return res;
}

function _secp256k1Pubkey(privKeyHex, compressed) {
  const k = BigInt("0x" + privKeyHex.replace(/^0x/, ""));
  const [x, y] = _pointMul(k);
  const xh = x.toString(16).padStart(64, "0");
  if (compressed) return _hexToBytes((y % 2n === 0n ? "02" : "03") + xh);
  return _hexToBytes("04" + xh + y.toString(16).padStart(64, "0"));
}

// bech32 encoder
const _B32 = "qpzry9x8gf2tvdw0s3jn54khce6mua7l";
const _B32GEN = [0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3];

function _b32Polymod(v) {
  let c = 1;
  for (const d of v) {
    const b = c >> 25; c = ((c & 0x1ffffff) << 5) ^ d;
    for (let i = 0; i < 5; i++) if ((b >> i) & 1) c ^= _B32GEN[i];
  }
  return c;
}

function _b32HrpExpand(hrp) {
  const r = [];
  for (const ch of hrp) r.push(ch.charCodeAt(0) >> 5);
  r.push(0);
  for (const ch of hrp) r.push(ch.charCodeAt(0) & 31);
  return r;
}

function _convertBits(data, from, to) {
  let acc = 0, bits = 0; const maxv = (1 << to) - 1, out = [];
  for (const v of data) {
    acc = ((acc << from) | v) >>> 0; bits += from;
    while (bits >= to) { bits -= to; out.push((acc >> bits) & maxv); }
  }
  if (bits > 0) out.push((acc << (to - bits)) & maxv);
  return out;
}

function _bech32Encode(hrp, bytes) {
  const words = _convertBits(Array.from(bytes), 8, 5);
  const expand = _b32HrpExpand(hrp);
  const poly = _b32Polymod([...expand, ...words, 0, 0, 0, 0, 0, 0]) ^ 1;
  const chk = Array.from({ length: 6 }, (_, i) => (poly >> (5 * (5 - i))) & 31);
  return hrp + "1" + [...words, ...chk].map(d => _B32[d]).join("");
}

// base58check for TRON
const _B58 = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz";
function _base58Check(bytes, CryptoJS) {
  const hex = _bytesToHex(bytes);
  const sha1 = CryptoJS.SHA256(CryptoJS.enc.Hex.parse(hex)).toString();
  const sha2 = CryptoJS.SHA256(CryptoJS.enc.Hex.parse(sha1)).toString();
  const full = new Uint8Array([...bytes, ..._hexToBytes(sha2.slice(0, 8))]);
  let num = BigInt("0x" + _bytesToHex(full));
  let res = "";
  while (num > 0n) { res = _B58[Number(num % 58n)] + res; num /= 58n; }
  for (const b of full) { if (b !== 0) break; res = "1" + res; }
  return res;
}

/**
 * Derive a Cosmos-family bech32 address (cosmos1, sei1, inj1 …) from a secp256k1 private key.
 * Uses the same key as EVM — just a different address encoding standard.
 */
export function deriveCosmosLikeAddress(privKeyHex, hrp, CryptoJS) {
  try {
    const pub = _secp256k1Pubkey(privKeyHex, true);
    const sha = CryptoJS.SHA256(CryptoJS.enc.Hex.parse(_bytesToHex(pub))).toString();
    const rip = CryptoJS.RIPEMD160(CryptoJS.enc.Hex.parse(sha)).toString();
    return _bech32Encode(hrp, _hexToBytes(rip));
  } catch { return ""; }
}

/**
 * Harmony ONE address = same 20-byte EVM address, re-encoded as bech32 with prefix "one".
 */
export function deriveHarmonyAddress(evmAddress) {
  try {
    const bytes = _hexToBytes(String(evmAddress).replace(/^0x/, "").slice(-40));
    return _bech32Encode("one", bytes);
  } catch { return ""; }
}

/**
 * TRON address = keccak256(uncompressed pubkey 64 bytes)[-20:] with 0x41 prefix, base58check.
 */
export function deriveTronAddress(privKeyHex, CryptoJS) {
  try {
    const pub = _secp256k1Pubkey(privKeyHex, false).slice(1); // drop 04 prefix
    const keccak = CryptoJS.SHA3(CryptoJS.enc.Hex.parse(_bytesToHex(pub)), { outputLength: 256 }).toString();
    const addr = new Uint8Array([0x41, ..._hexToBytes(keccak.slice(-40))]);
    return _base58Check(addr, CryptoJS);
  } catch { return ""; }
}

