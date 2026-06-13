/**
 * TON wallet v3R2 — pure JS BOC builder, send, Jetton transfer + balance.
 * No external SDK required. Uses crypto-js + tweetnacl (already in project).
 *
 * Exports:
 *   buildTonTransferBoc(params, privKeyHex)  → base64 BOC string
 *   buildJettonTransferBoc(params, privKeyHex) → base64 BOC string
 *   getTonSeqno(nodeUrl, walletAddress)      → number
 *   broadcastTonBoc(nodeUrl, bocBase64)      → txHash string
 *   getTonJettonBalance(nodeUrl, walletAddr, jettonMaster) → balance string
 *   decodeTonAddress(addr)                   → { workchain, hash }
 */

import CryptoJS from "crypto-js";
import nacl from "tweetnacl";

// ─── Low-level utils ──────────────────────────────────────────────────────────

function _hexToBytes(hex) {
  const h = hex.replace(/^0x/, "");
  const out = new Uint8Array(h.length / 2);
  for (let i = 0; i < out.length; i++) out[i] = parseInt(h.slice(i * 2, i * 2 + 2), 16);
  return out;
}

function _bytesToHex(b) {
  return Array.from(b).map(x => x.toString(16).padStart(2, "0")).join("");
}

function _concat(...arrs) {
  const n = arrs.reduce((s, a) => s + a.length, 0);
  const o = new Uint8Array(n);
  let p = 0;
  for (const a of arrs) { o.set(a, p); p += a.length; }
  return o;
}

function _sha256(data) {
  const wa = CryptoJS.lib.WordArray.create(data);
  const h = CryptoJS.SHA256(wa);
  const out = new Uint8Array(h.sigBytes);
  for (let i = 0; i < h.sigBytes; i++) out[i] = (h.words[i >>> 2] >>> (24 - (i % 4) * 8)) & 0xff;
  return out;
}

// ─── Bit Builder ─────────────────────────────────────────────────────────────

class BB {
  constructor() { this._b = []; }

  /** Write unsigned integer n in bitCount bits (MSB first). */
  u(n, bitCount) {
    const bn = BigInt(n);
    for (let i = bitCount - 1; i >= 0; i--) this._b.push(Number((bn >> BigInt(i)) & 1n));
    return this;
  }

  /** Write signed integer n in bitCount bits (two's-complement). */
  i(n, bitCount) {
    let bn = BigInt(n);
    if (bn < 0n) bn = (1n << BigInt(bitCount)) + bn;
    return this.u(bn, bitCount);
  }

  /** Write raw bytes (8 bits each). */
  bytes(arr) {
    for (const b of arr) this.u(b, 8);
    return this;
  }

  get len() { return this._b.length; }

  /**
   * Serialize to bytes with TVM completion tag:
   * if bits not byte-aligned, append 1 then zeros to fill the byte.
   * Returns { bytes: Uint8Array, bitLen: number }
   */
  toDataBytes() {
    const bits = [...this._b];
    const bitLen = bits.length;
    if (bitLen % 8 !== 0) {
      bits.push(1);
      while (bits.length % 8 !== 0) bits.push(0);
    }
    const out = new Uint8Array(bits.length / 8);
    for (let i = 0; i < out.length; i++) {
      let v = 0;
      for (let j = 0; j < 8; j++) v = (v << 1) | bits[i * 8 + j];
      out[i] = v;
    }
    return { bytes: out, bitLen };
  }
}

// ─── Cell ────────────────────────────────────────────────────────────────────

class Cell {
  constructor() {
    this.bb = new BB();
    this.refs = []; // Cell[]
  }

  /**
   * Cell hash = SHA256(d1 || d2 || data_bytes || ref_hash_0 || ref_hash_1 ...)
   * d1 = refs_count  (ordinary cell, level 0, not exotic)
   * d2 = floor(bitLen/8)*2 + (bitLen%8 != 0 ? 1 : 0)
   */
  hash() {
    const { bytes, bitLen } = this.bb.toDataBytes();
    const d1 = this.refs.length;
    const d2 = Math.floor(bitLen / 8) * 2 + (bitLen % 8 !== 0 ? 1 : 0);
    const refHashes = this.refs.map(r => r.hash());
    return _sha256(_concat(new Uint8Array([d1, d2]), bytes, ...refHashes));
  }

  /** Serialize cell descriptors + data bytes (refs are stored as indices in BOC). */
  serialize() {
    const { bytes, bitLen } = this.bb.toDataBytes();
    const d1 = this.refs.length;
    const d2 = Math.floor(bitLen / 8) * 2 + (bitLen % 8 !== 0 ? 1 : 0);
    return { d1, d2, bytes };
  }
}

// ─── TL-B encoders ───────────────────────────────────────────────────────────

/**
 * Decode a TON user-friendly address (EQ.../UQ...) to { workchain, hash(32 bytes) }.
 */
export function decodeTonAddress(addr) {
  const b64 = addr.replace(/-/g, "+").replace(/_/g, "/");
  const bin = atob(b64);
  const bytes = Uint8Array.from(bin, c => c.charCodeAt(0));
  // Layout: [flags(1), workchain(1), hash(32), crc(2)] = 36 bytes
  const workchain = bytes[1] === 0xff ? -1 : bytes[1];
  return { workchain, hash: bytes.slice(2, 34) };
}

/**
 * Encode { workchain, hash32 } back to EQ... user-friendly address.
 */
export function encodeTonAddress(workchain, hash32, bounceable = true) {
  const data = new Uint8Array(36);
  data[0] = bounceable ? 0x11 : 0x51;
  data[1] = workchain === -1 ? 0xff : workchain;
  data.set(hash32, 2);
  // CRC-16 CCITT over first 34 bytes
  let crc = 0;
  for (let i = 0; i < 34; i++) {
    for (let j = 0; j < 8; j++) {
      const bit = (data[i] >> (7 - j)) & 1;
      const c15 = (crc >> 15) & 1;
      crc = (crc << 1) & 0xffff;
      if (c15 ^ bit) crc ^= 0x1021;
    }
  }
  data[34] = (crc >> 8) & 0xff;
  data[35] = crc & 0xff;
  let bin = "";
  for (const b of data) bin += String.fromCharCode(b);
  return btoa(bin).replace(/\+/g, "-").replace(/\//g, "_");
}

function _writeAddr(bb, workchain, hash32) {
  bb.u(0b10, 2); // addr_std$10
  bb.u(0, 1);    // no anycast
  bb.i(workchain, 8);
  bb.bytes(hash32);
}

/** Variable-length Grams encoding. */
function _writeGrams(bb, nanoTon) {
  const bn = BigInt(nanoTon);
  if (bn === 0n) { bb.u(0, 4); return; }
  let hex = bn.toString(16);
  if (hex.length % 2) hex = "0" + hex;
  bb.u(hex.length / 2, 4);
  for (let i = 0; i < hex.length; i += 2) bb.u(parseInt(hex.slice(i, i + 2), 16), 8);
}

// ─── Message cell builders ────────────────────────────────────────────────────

/**
 * Build an internal message cell (TON coin transfer, optional text comment).
 */
function _buildInternal({ destWorkchain, destHash, nanoTon, bounce = true, comment = "" }) {
  const c = new Cell();
  const b = c.bb;
  b.u(0, 1);              // int_msg_info$0
  b.u(1, 1);              // ihr_disabled = true
  b.u(bounce ? 1 : 0, 1); // bounce
  b.u(0, 1);              // bounced = false
  b.u(0, 2);              // src: addr_none$00
  _writeAddr(b, destWorkchain, destHash);
  _writeGrams(b, nanoTon);
  _writeGrams(b, 0); _writeGrams(b, 0); // ihr_fee, fwd_fee
  b.u(0, 64); b.u(0, 32); // created_lt, created_at
  b.u(0, 1);              // no stateInit
  b.u(0, 1);              // body inline
  if (comment) {
    b.u(0, 32);           // op = 0 → simple text comment
    b.bytes(new TextEncoder().encode(comment));
  }
  return c;
}

/**
 * Build a Jetton transfer internal message.
 * Sends to the *user's own Jetton wallet*, which forwards to the recipient.
 * jettonWalletWorkchain/Hash = the sender's jetton wallet address (not the master)
 */
function _buildJettonInternal({
  jettonWalletWorkchain, jettonWalletHash,
  destWorkchain, destHash,
  responseWorkchain, responseHash,
  jettonAmount, forwardTon = 1n,
}) {
  const c = new Cell();
  const b = c.bb;
  // int_msg_info to sender's Jetton wallet
  b.u(0, 1); b.u(1, 1); b.u(1, 1); b.u(0, 1); // int_msg_info, ihr_disabled, bounce=true, bounced=false
  b.u(0, 2); // src: addr_none
  _writeAddr(b, jettonWalletWorkchain, jettonWalletHash);
  _writeGrams(b, 50000000n); // 0.05 TON for gas
  _writeGrams(b, 0); _writeGrams(b, 0);
  b.u(0, 64); b.u(0, 32);
  b.u(0, 1); // no stateInit
  b.u(0, 1); // inline body
  // Jetton transfer op: 0x0f8a7ea5
  b.u(0x0f8a7ea5, 32);
  b.u(0, 64);  // query_id = 0
  _writeGrams(b, jettonAmount);
  _writeAddr(b, destWorkchain, destHash);           // destination
  _writeAddr(b, responseWorkchain, responseHash);   // response_destination (excess gas back)
  b.u(0, 1);  // custom_payload: none
  _writeGrams(b, forwardTon);
  b.u(0, 1);  // forward_payload: inline empty
  return c;
}

/** Build unsigned v3R2 body cell. */
function _buildBody({ subwalletId, validUntil, seqno, mode, intMsg }) {
  const c = new Cell();
  c.bb.u(subwalletId, 32).u(validUntil, 32).u(seqno, 32).u(mode, 8);
  c.refs.push(intMsg);
  return c;
}

/** Prepend signature to body cell, copying all bits and refs. */
function _buildSignedBody(sig64, bodyCell) {
  const c = new Cell();
  c.bb.bytes(sig64);
  for (const bit of bodyCell.bb._b) c.bb._b.push(bit);
  for (const ref of bodyCell.refs) c.refs.push(ref);
  return c;
}

/** Build ext_in_msg_info wrapper. */
function _buildExtMsg({ walletWorkchain, walletHash, bodyCell }) {
  const c = new Cell();
  const b = c.bb;
  b.u(0b10, 2); // ext_in_msg_info$10
  b.u(0, 2);    // src: addr_none
  _writeAddr(b, walletWorkchain, walletHash);
  _writeGrams(b, 0); // import_fee = 0
  b.u(0, 1);    // no stateInit
  b.u(1, 1);    // body is in ref
  c.refs.push(bodyCell);
  return c;
}

// ─── BOC serialization ────────────────────────────────────────────────────────

/** Collect all cells via BFS (root first). */
function _collectCells(root) {
  const cells = [], seen = new Set(), q = [root];
  while (q.length) {
    const c = q.shift();
    if (seen.has(c)) continue;
    seen.add(c); cells.push(c);
    for (const r of c.refs) q.push(r);
  }
  return cells;
}

/**
 * Serialize a cell tree to standard BOC (base64 string).
 * Single root, no CRC32, no index.
 */
export function toBoc(rootCell) {
  const cells = _collectCells(rootCell);
  const idx = new Map(cells.map((c, i) => [c, i]));
  const refSize = cells.length <= 255 ? 1 : 2;

  function uintB(n, size) {
    const b = new Uint8Array(size);
    for (let i = size - 1; i >= 0; i--) { b[i] = n & 0xff; n >>= 8; }
    return b;
  }

  const cellData = _concat(...cells.map(cell => {
    const { d1, d2, bytes } = cell.serialize();
    const refs = _concat(...cell.refs.map(r => {
      const i = idx.get(r);
      return refSize === 1 ? new Uint8Array([i]) : new Uint8Array([(i >> 8) & 0xff, i & 0xff]);
    }));
    return _concat(new Uint8Array([d1, d2]), bytes, refs);
  }));

  const offSz = 4;
  const boc = _concat(
    new Uint8Array([0xB5, 0xEE, 0x9C, 0x72]), // magic
    new Uint8Array([refSize]),                  // flags byte (has_idx=0, has_crc32=0, ref_size=refSize)
    new Uint8Array([offSz]),                    // offset_size
    uintB(cells.length, refSize),              // cell_count
    uintB(1, refSize),                          // roots_count
    uintB(0, refSize),                          // absent
    uintB(cellData.length, offSz),             // tot_cells_size
    uintB(0, refSize),                          // root_list[0] = 0
    cellData,
  );

  let bin = "";
  for (const b of boc) bin += String.fromCharCode(b);
  return btoa(bin);
}

// ─── Minimal BOC decoder (for Jetton wallet address extraction) ───────────────

/**
 * Decode the first cell of a BOC that contains an addr_std.
 * Returns EQ... string or null.
 */
function _decodeAddrCellFromBoc(bocB64) {
  try {
    const bin = atob(bocB64.replace(/-/g, "+").replace(/_/g, "/"));
    const bytes = Uint8Array.from(bin, c => c.charCodeAt(0));
    if (bytes[0] !== 0xB5 || bytes[1] !== 0xEE || bytes[2] !== 0x9C || bytes[3] !== 0x72) return null;
    const refSize = bytes[4] & 0x07;
    const offSize = bytes[5];
    let p = 6 + refSize * 3 + offSize; // skip cell_count, roots_count, absent, tot_cells_size
    p += refSize; // skip root_list[0]
    // First cell: d1, d2, then data
    const d2 = bytes[p + 1]; p += 2;
    const dataByteCount = Math.ceil(d2 / 2);
    const cellData = bytes.slice(p, p + dataByteCount);
    // addr_std: 2 bits tag(10) + 1 bit anycast(0) + 8 bits workchain + 256 bits hash
    // starts at bit 0. Read workchain at bit 3, hash at bit 11.
    let wc = 0;
    for (let i = 0; i < 8; i++) {
      const bp = 3 + i;
      wc = (wc << 1) | ((cellData[bp >> 3] >> (7 - (bp & 7))) & 1);
    }
    if (wc >= 128) wc -= 256;
    const hash = new Uint8Array(32);
    for (let i = 0; i < 256; i++) {
      const bp = 11 + i;
      const bit = (cellData[bp >> 3] >> (7 - (bp & 7))) & 1;
      hash[i >> 3] |= (bit << (7 - (i & 7)));
    }
    return encodeTonAddress(wc, hash, true);
  } catch { return null; }
}

// ─── Public: sign & build BOC ─────────────────────────────────────────────────

/**
 * Build and sign a native TON transfer.
 *
 * @param {object} params
 *   walletAddress     EQ... sender wallet address
 *   toAddress         EQ... destination
 *   nanoTon           amount in nanoTON (string or number)
 *   seqno             current wallet seqno (fetch with getTonSeqno)
 *   memo              optional text comment
 *   bounce            default true
 *   validUntilOffset  seconds from now, default 60
 *   subwalletId       default 698983191
 * @param {string} privKeyHex  32-byte ed25519 private key hex
 * @returns {string} base64 BOC ready for broadcastTonBoc
 */
export function buildTonTransferBoc(params, privKeyHex) {
  const {
    walletAddress,
    toAddress,
    nanoTon,
    seqno = 0,
    memo = "",
    bounce = true,
    validUntilOffset = 60,
    subwalletId = 698983191,
  } = params;

  const validUntil = Math.floor(Date.now() / 1000) + validUntilOffset;
  const { workchain: dWC, hash: dHash } = decodeTonAddress(toAddress);
  const { workchain: sWC, hash: sHash } = decodeTonAddress(walletAddress);

  const intMsg = _buildInternal({
    destWorkchain: dWC, destHash: dHash,
    nanoTon: BigInt(nanoTon),
    bounce, comment: memo,
  });

  const body = _buildBody({ subwalletId, validUntil, seqno, mode: 3, intMsg });

  // ed25519 sign of cell hash
  const kp = nacl.sign.keyPair.fromSeed(_hexToBytes(privKeyHex));
  const sig = nacl.sign.detached(body.hash(), kp.secretKey);

  const signedBody = _buildSignedBody(sig, body);
  const extMsg = _buildExtMsg({ walletWorkchain: sWC, walletHash: sHash, bodyCell: signedBody });
  return toBoc(extMsg);
}

/**
 * Build and sign a Jetton (token) transfer.
 * Requires the sender's Jetton wallet address (obtained via getTonJettonWalletAddress).
 *
 * @param {object} params
 *   walletAddress          EQ... sender wallet address
 *   jettonWalletAddress    EQ... sender's Jetton wallet (not master)
 *   toAddress              EQ... recipient
 *   jettonAmount           amount in Jetton base units (string)
 *   seqno                  current wallet seqno
 *   subwalletId            default 698983191
 * @param {string} privKeyHex
 * @returns {string} base64 BOC
 */
export function buildJettonTransferBoc(params, privKeyHex) {
  const {
    walletAddress,
    jettonWalletAddress,
    toAddress,
    jettonAmount,
    seqno = 0,
    validUntilOffset = 60,
    subwalletId = 698983191,
  } = params;

  const validUntil = Math.floor(Date.now() / 1000) + validUntilOffset;
  const { workchain: sWC, hash: sHash } = decodeTonAddress(walletAddress);
  const { workchain: jwWC, hash: jwHash } = decodeTonAddress(jettonWalletAddress);
  const { workchain: dWC, hash: dHash } = decodeTonAddress(toAddress);

  const intMsg = _buildJettonInternal({
    jettonWalletWorkchain: jwWC, jettonWalletHash: jwHash,
    destWorkchain: dWC, destHash: dHash,
    responseWorkchain: sWC, responseHash: sHash,
    jettonAmount: BigInt(jettonAmount),
    forwardTon: 1n,
  });

  const body = _buildBody({ subwalletId, validUntil, seqno, mode: 3, intMsg });
  const kp = nacl.sign.keyPair.fromSeed(_hexToBytes(privKeyHex));
  const sig = nacl.sign.detached(body.hash(), kp.secretKey);
  const signedBody = _buildSignedBody(sig, body);
  const extMsg = _buildExtMsg({ walletWorkchain: sWC, walletHash: sHash, bodyCell: signedBody });
  return toBoc(extMsg);
}

// ─── Public: network calls ────────────────────────────────────────────────────

function _tonBase(nodeUrl) {
  return nodeUrl.replace(/\/jsonRPC$/, "");
}

/**
 * Fetch current wallet seqno from TonCenter v2.
 * Returns 0 if wallet not yet deployed (new wallet).
 */
export async function getTonSeqno(nodeUrl, walletAddress) {
  try {
    const res = await fetch(`${_tonBase(nodeUrl)}/runGetMethod`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ address: walletAddress, method: "seqno", stack: [] }),
    }).then(r => r.json()).catch(() => null);
    const item = res?.result?.stack?.[0];
    if (!item) return 0;
    const val = Array.isArray(item) ? item[1] : item;
    return parseInt(String(val).replace(/^0x/, ""), 16) || 0;
  } catch { return 0; }
}

/**
 * Get the user's personal Jetton wallet address from the Jetton master.
 * Returns EQ... string or null.
 */
export async function getTonJettonWalletAddress(nodeUrl, ownerAddress, jettonMaster) {
  try {
    const res = await fetch(`${_tonBase(nodeUrl)}/runGetMethod`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        address: jettonMaster,
        method: "get_wallet_address",
        stack: [["tvm.Slice", ownerAddress]],
      }),
    }).then(r => r.json()).catch(() => null);
    const item = res?.result?.stack?.[0];
    if (!item) return null;
    // TonCenter returns ["cell", {"bytes":"base64boc"}] or ["cell", "base64boc"]
    const val = Array.isArray(item) ? item[1] : item;
    const bocB64 = typeof val === "object" ? val?.bytes : val;
    if (!bocB64) return null;
    return _decodeAddrCellFromBoc(bocB64);
  } catch { return null; }
}

/**
 * Fetch user's Jetton token balance.
 * Tries TonCenter v3 REST first (clean JSON), falls back to v2 runGetMethod.
 * Returns balance as a string in base units.
 */
export async function getTonJettonBalance(nodeUrl, walletAddress, jettonMasterAddress) {
  try {
    const isTestnet = nodeUrl.includes("testnet");
    const v3Base = isTestnet
      ? "https://testnet.toncenter.com/api/v3"
      : "https://toncenter.com/api/v3";

    // Try v3 endpoint (direct Jetton wallet lookup by owner + master)
    const v3 = await fetch(
      `${v3Base}/jetton/wallets?owner_address=${encodeURIComponent(walletAddress)}&jetton_address=${encodeURIComponent(jettonMasterAddress)}&limit=1`
    ).then(r => r.json()).catch(() => null);

    const jwV3 = v3?.jetton_wallets?.[0];
    if (jwV3?.balance != null) return String(jwV3.balance);

    // Fallback: v2 — get_wallet_address → get_wallet_data
    const jwAddr = await getTonJettonWalletAddress(nodeUrl, walletAddress, jettonMasterAddress);
    if (!jwAddr) return "0";

    const dataRes = await fetch(`${_tonBase(nodeUrl)}/runGetMethod`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ address: jwAddr, method: "get_wallet_data", stack: [] }),
    }).then(r => r.json()).catch(() => null);

    const balItem = dataRes?.result?.stack?.[0];
    if (!balItem) return "0";
    const v = Array.isArray(balItem) ? balItem[1] : balItem;
    return String(parseInt(String(v).replace(/^0x/, ""), 16) || 0);
  } catch { return "0"; }
}

/**
 * Broadcast a signed BOC to TonCenter.
 * Throws on failure.
 */
export async function broadcastTonBoc(nodeUrl, bocBase64) {
  const res = await fetch(`${_tonBase(nodeUrl)}/sendBoc`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ boc: bocBase64 }),
  }).then(r => r.json()).catch(() => null);

  if (res?.ok) return res?.result?.hash || "submitted";
  throw new Error(res?.error || "TON broadcast failed");
}
