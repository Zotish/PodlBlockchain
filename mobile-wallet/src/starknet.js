/**
 * Starknet — Pedersen hash, correct ArgentX address, STARK ECDSA, invoke transaction.
 * Pure JS, zero external SDK required. Uses only crypto-js (already in project).
 *
 * Exports:
 *   starkPedersen(a, b)                         → BigInt
 *   starkHashArray(values)                       → BigInt
 *   starknetArgentAddress(publicKeyHex)          → "0x..." (correct address)
 *   starkSign(msgHashHex, privKeyHex)            → { r, s } hex strings
 *   getStarknetNonce(nodeUrl, accountAddress)    → BigInt
 *   getStarknetChainId(nodeUrl)                  → BigInt
 *   broadcastStarknetTransfer(params, privKeyHex) → txHash string
 *   broadcastStarknetTokenTransfer(params, privKeyHex) → txHash string
 */

import CryptoJS from "crypto-js";

// ─── Utils ────────────────────────────────────────────────────────────────────

function _hexToBytes(hex) {
  const h = hex.replace(/^0x/, "");
  const out = new Uint8Array(h.length / 2);
  for (let i = 0; i < out.length; i++) out[i] = parseInt(h.slice(i * 2, i * 2 + 2), 16);
  return out;
}

function _bytesToHex(b) {
  return Array.from(b).map(x => x.toString(16).padStart(2, "0")).join("");
}

function _sha256(data) {
  const wa = CryptoJS.lib.WordArray.create(data);
  const h = CryptoJS.SHA256(wa);
  const out = new Uint8Array(h.sigBytes);
  for (let i = 0; i < h.sigBytes; i++) out[i] = (h.words[i >>> 2] >>> (24 - (i % 4) * 8)) & 0xff;
  return out;
}

function _keccak256(data) {
  const wa = CryptoJS.lib.WordArray.create(data);
  const h = CryptoJS.SHA3(wa, { outputLength: 256 });
  const out = new Uint8Array(h.sigBytes);
  for (let i = 0; i < h.sigBytes; i++) out[i] = (h.words[i >>> 2] >>> (24 - (i % 4) * 8)) & 0xff;
  return out;
}

// ─── STARK curve (duplicated from crypto.js for module isolation) ─────────────

const STARK_P     = BigInt("0x0800000000000011000000000000000000000000000000000000000000000001");
const STARK_ORDER = BigInt("0x0800000000000010ffffffffffffffffb781126dcae7b2321e66a241adc64d2f");
const STARK_GX    = BigInt("0x01ef15c18599971b7beced415a40f0c7deacfd9b0d1819e03d723d8bc943cfca");
const STARK_GY    = BigInt("0x005668060aa49730b7be4801df46ec62de53ecd11abe43a32873000c36e8dc1f");
const STARK_A     = 1n;

function _smod(a, m) { return ((a % m) + m) % m; }

function _sinv(a, m) {
  let [r, nr, s, ns] = [m, _smod(a, m), 0n, 1n];
  while (nr !== 0n) {
    const q = r / nr;
    [r, nr] = [nr, r - q * nr];
    [s, ns] = [ns, s - q * ns];
  }
  return _smod(s, m);
}

function _sadd(p1, p2) {
  if (!p1) return p2;
  if (!p2) return p1;
  const [x1, y1] = p1, [x2, y2] = p2;
  if (x1 === x2) {
    if (y1 !== y2) return null;
    const lam = _smod(3n * x1 * x1 % STARK_P + STARK_A, STARK_P) * _sinv(2n * y1, STARK_P) % STARK_P;
    const x3 = _smod(lam * lam - 2n * x1, STARK_P);
    return [x3, _smod(lam * (x1 - x3) - y1, STARK_P)];
  }
  const lam = _smod((y2 - y1) * _sinv(x2 - x1, STARK_P), STARK_P);
  const x3 = _smod(lam * lam - x1 - x2, STARK_P);
  return [x3, _smod(lam * (x1 - x3) - y1, STARK_P)];
}

function _smul(k, px = STARK_GX, py = STARK_GY) {
  let res = null, cur = [px, py];
  for (; k > 0n; k >>= 1n) {
    if (k & 1n) res = _sadd(res, cur);
    cur = _sadd(cur, cur);
  }
  return res;
}

// ─── Pedersen hash constants (from Starkware spec) ───────────────────────────
// https://docs.starkware.co/starkex/crypto/pedersen-hash-function.html

const P0 = [
  BigInt("0x049ee3eba8c1600700ee1b87eb599f16716b0b1022947733551fde4050ca6804"),
  BigInt("0x03ca0cfe4b3bc6ddf346d49d06ea0ed34e621062c0e056c1d0405d266e10268a"),
];
const P1 = [
  BigInt("0x0234287dcbaffe7f969c748655fca9e58fa8120b6d56eb0c1080d17957ebe47b"),
  BigInt("0x03b056f100f96fb21e889527d41f4e39940135dd7a6c94cc6ed0268ee89e5615"),
];
const P2 = [
  BigInt("0x04fa56f376c83db33f9dab2656558f3399099ec1de5e3018b7a6932dba8aa378"),
  BigInt("0x03fa0984c931c9e38113e0c0e47e4401562761f92a7a23b45168f4e80ff5b54d"),
];
const P3 = [
  BigInt("0x04ba4cc166be8dec764910f75b45f74b40c690c74709e90f3aa372f0bd2d6997"),
  BigInt("0x0040301cf5c1751f4b971e46c4ede85fcac5c59a5ce5ae7c48151f27b24b219c"),
];
const P4 = [
  BigInt("0x054302dcb0e6cc1c6e44cca8f61a63bb2ca65048d53fb325d36b12d77532867"),
  BigInt("0x01b77b3e37d13504b348046268d8ae25ce98ad783c25561a879dcc77e99c2426"),
];

/**
 * Starkware Pedersen hash of two field elements.
 * pedersen(a, b) = P0 + a_low*P1 + a_high*P2 + b_low*P3 + b_high*P4
 * where a_low = a mod 2^248, a_high = a >> 248.
 */
export function starkPedersen(a, b) {
  const LOW = (1n << 248n) - 1n;
  let acc = [...P0];
  const aLo = a & LOW, aHi = a >> 248n;
  const bLo = b & LOW, bHi = b >> 248n;
  if (aLo > 0n) acc = _sadd(acc, _smul(aLo, P1[0], P1[1]));
  if (aHi > 0n) acc = _sadd(acc, _smul(aHi, P2[0], P2[1]));
  if (bLo > 0n) acc = _sadd(acc, _smul(bLo, P3[0], P3[1]));
  if (bHi > 0n) acc = _sadd(acc, _smul(bHi, P4[0], P4[1]));
  return acc ? acc[0] : 0n;
}

/**
 * Pedersen chain hash of an array of field elements.
 * h = 0; for each v: h = pedersen(h, v);  return pedersen(h, len)
 */
export function starkHashArray(values) {
  let h = 0n;
  for (const v of values) h = starkPedersen(h, BigInt(v));
  return starkPedersen(h, BigInt(values.length));
}

// ─── Address derivation ───────────────────────────────────────────────────────

// ArgentX v0 class hash (deployed on Goerli / Starknet testnet)
const ARGENT_CLASS_HASH = BigInt("0x025ec026985a3bf9d0cc1fe17326b245dfdc3ff89b8fde106542a3ea56c5a918");

// "STARKNET_CONTRACT_ADDRESS" string encoded as a felt
const CONTRACT_ADDR_PREFIX = BigInt("0x535441524b4e45545f434f4e54524143545f41444452455353");

/**
 * Compute the correct ArgentX account address from a STARK public key.
 * Uses Pedersen hash — this replaces the sha256 approximation in crypto.js.
 *
 * Formula:
 *   calldata_hash = starkHashArray([pubkey, guardian=0])
 *   address       = starkHashArray([prefix, deployer=0, salt=pubkey, class_hash, calldata_hash])
 *
 * @param {string} publicKeyHex  "0x..." STARK public key
 * @returns {string} "0x..." Starknet account address (64 hex chars)
 */
export function starknetArgentAddress(publicKeyHex) {
  try {
    const pub = BigInt(publicKeyHex.startsWith("0x") ? publicKeyHex : "0x" + publicKeyHex);
    const calldataHash = starkHashArray([pub, 0n]);
    const addr = starkHashArray([
      CONTRACT_ADDR_PREFIX,
      0n,               // deployer = 0 (CREATE2-style)
      pub,              // salt = public key
      ARGENT_CLASS_HASH,
      calldataHash,
    ]);
    return "0x" + addr.toString(16).padStart(64, "0");
  } catch {
    return "0x" + "0".repeat(64);
  }
}

// ─── STARK ECDSA ─────────────────────────────────────────────────────────────

/**
 * Deterministic k for STARK ECDSA (RFC-6979-inspired with SHA256).
 * Hashes privKey || msgHash until result < STARK_ORDER.
 */
function _starkDeterministicK(privKeyHex, msgHashHex) {
  let data = new Uint8Array([
    ..._hexToBytes(privKeyHex.replace(/^0x/, "").padStart(64, "0")),
    ..._hexToBytes(msgHashHex.replace(/^0x/, "").padStart(64, "0")),
  ]);
  for (let i = 0; i < 1000; i++) {
    const candidate = BigInt("0x" + _bytesToHex(_sha256(data)));
    if (candidate > 0n && candidate < STARK_ORDER) return candidate;
    data = _sha256(new Uint8Array([...data, i & 0xff]));
  }
  throw new Error("STARK: failed to generate valid k");
}

/**
 * Sign a message hash with STARK ECDSA.
 * @param {string} msgHashHex  32-byte hex (no 0x prefix)
 * @param {string} privKeyHex  STARK private key hex
 * @returns {{ r: string, s: string }}  both "0x..." padded to 64 hex chars
 */
export function starkSign(msgHashHex, privKeyHex) {
  const priv = BigInt("0x" + privKeyHex.replace(/^0x/, ""));
  const msg  = BigInt("0x" + msgHashHex.replace(/^0x/, ""));
  const k    = _starkDeterministicK(privKeyHex, msgHashHex);
  const R    = _smul(k);
  if (!R) throw new Error("STARK: R point is null");
  const r    = R[0] % STARK_ORDER;
  if (r === 0n) throw new Error("STARK: r == 0");
  const s    = _smod(_sinv(k, STARK_ORDER) * (msg + r * priv), STARK_ORDER);
  if (s === 0n) throw new Error("STARK: s == 0");
  return {
    r: "0x" + r.toString(16).padStart(64, "0"),
    s: "0x" + s.toString(16).padStart(64, "0"),
  };
}

// ─── Starknet selectors ───────────────────────────────────────────────────────

/**
 * Starknet function selector = keccak256(name_bytes) as BigInt, mod 2^250.
 */
function _selector(name) {
  const enc = new TextEncoder().encode(name);
  return BigInt("0x" + _bytesToHex(_keccak256(enc))) % (1n << 250n);
}

// Cache selectors (computed once)
const SEL_TRANSFER = _selector("transfer");

// ─── Transaction hash ────────────────────────────────────────────────────────

/**
 * Compute invoke v1 transaction hash.
 * Uses Pedersen chain hash over: ["invoke", 1, sender, 0, calldata_hash, max_fee, chain_id, nonce]
 */
function _invokeV1Hash({ senderAddress, calldata, maxFee, chainId, nonce }) {
  const INVOKE_PREFIX = BigInt("0x696e766f6b65"); // "invoke" as felt
  const calldataHash  = starkHashArray(calldata);
  return starkHashArray([
    INVOKE_PREFIX,
    1n,            // version
    senderAddress,
    0n,            // entry_point_selector (unused in v1)
    calldataHash,
    maxFee,
    chainId,
    nonce,
  ]);
}

// ─── Node queries ─────────────────────────────────────────────────────────────

/**
 * Get nonce for a Starknet account.
 */
export async function getStarknetNonce(nodeUrl, accountAddress) {
  try {
    // Try JSON-RPC first
    const rpc = await fetch(nodeUrl, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        jsonrpc: "2.0", id: 1,
        method: "starknet_getNonce",
        params: ["latest", accountAddress],
      }),
    }).then(r => r.json()).catch(() => null);
    if (rpc?.result != null) return BigInt(rpc.result);

    // Fallback: feeder_gateway
    const fg = await fetch(`${nodeUrl}/feeder_gateway/get_nonce?contractAddress=${accountAddress}`)
      .then(r => r.json()).catch(() => null);
    return BigInt(fg ?? "0x0");
  } catch { return 0n; }
}

/**
 * Get chain ID from a Starknet node.
 */
export async function getStarknetChainId(nodeUrl) {
  try {
    const res = await fetch(nodeUrl, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ jsonrpc: "2.0", id: 1, method: "starknet_chainId", params: [] }),
    }).then(r => r.json()).catch(() => null);
    return BigInt(res?.result ?? "0x534e5f474f45524c49"); // default: SN_GOERLI
  } catch {
    return BigInt("0x534e5f474f45524c49");
  }
}

// ─── Broadcast helpers ────────────────────────────────────────────────────────

async function _broadcast(nodeUrl, senderAddress, calldata, privKeyHex, maxFee = "100000000000000") {
  const sender  = BigInt(senderAddress);
  const chainId = await getStarknetChainId(nodeUrl);
  const nonce   = await getStarknetNonce(nodeUrl, senderAddress);
  const maxFeeN = BigInt(maxFee);

  const txHash = _invokeV1Hash({ senderAddress: sender, calldata, maxFee: maxFeeN, chainId, nonce });
  const hashHex = txHash.toString(16).padStart(64, "0");
  const { r, s } = starkSign(hashHex, privKeyHex.replace(/^0x/, ""));

  const body = {
    jsonrpc: "2.0", id: 1,
    method: "starknet_addInvokeTransaction",
    params: [{
      type: "INVOKE",
      max_fee: "0x" + maxFeeN.toString(16),
      version: "0x1",
      signature: [r, s],
      nonce: "0x" + nonce.toString(16),
      sender_address: senderAddress,
      calldata: calldata.map(v => "0x" + BigInt(v).toString(16)),
    }],
  };

  const res = await fetch(nodeUrl, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  }).then(r => r.json()).catch(() => null);

  if (res?.result?.transaction_hash) return res.result.transaction_hash;
  throw new Error(res?.error?.message || "Starknet broadcast failed");
}

// ETH contract address on Starknet (both mainnet and testnet)
const ETH_CONTRACT = BigInt("0x049d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7");

/**
 * Sign and broadcast a native ETH transfer on Starknet (invoke v1).
 *
 * @param {object} params
 *   senderAddress  "0x..." account address
 *   toAddress      "0x..." recipient
 *   amountWei      amount in wei (string)
 *   maxFee         optional max fee in wei string (default "100000000000000" = 0.0001 ETH)
 *   nodeUrl        Starknet node URL
 * @param {string} privKeyHex  STARK private key hex
 * @returns {string} transaction hash
 */
export async function broadcastStarknetTransfer({ senderAddress, toAddress, amountWei, maxFee, nodeUrl }, privKeyHex) {
  const to     = BigInt(toAddress);
  const amtLow = BigInt(amountWei) & ((1n << 128n) - 1n);
  const amtHigh = BigInt(amountWei) >> 128n;

  // __execute__ v1 calldata: [call_count, contract, selector, calldata_offset, calldata_len, ...args]
  const calldata = [
    1n,              // 1 call
    ETH_CONTRACT,    // call ETH contract
    SEL_TRANSFER,    // transfer(to, amount)
    0n,              // calldata offset
    3n,              // calldata length
    to,              // arg: recipient
    amtLow,          // arg: amount uint256 low
    amtHigh,         // arg: amount uint256 high
  ];

  return _broadcast(nodeUrl, senderAddress, calldata, privKeyHex, maxFee);
}

/**
 * Sign and broadcast a Starknet ERC-20 token transfer (invoke v1).
 * Works for any ERC-20 token on Starknet (USDC, DAI, etc.).
 *
 * @param {object} params
 *   senderAddress   "0x..." account address
 *   toAddress       "0x..." recipient
 *   tokenContract   "0x..." ERC-20 contract address
 *   amountWei       amount in token base units (string)
 *   maxFee          optional
 *   nodeUrl         Starknet node URL
 * @param {string} privKeyHex
 * @returns {string} transaction hash
 */
export async function broadcastStarknetTokenTransfer({ senderAddress, toAddress, tokenContract, amountWei, maxFee, nodeUrl }, privKeyHex) {
  const to       = BigInt(toAddress);
  const contract = BigInt(tokenContract);
  const amtLow   = BigInt(amountWei) & ((1n << 128n) - 1n);
  const amtHigh  = BigInt(amountWei) >> 128n;

  const calldata = [
    1n, contract, SEL_TRANSFER, 0n, 3n,
    to, amtLow, amtHigh,
  ];

  return _broadcast(nodeUrl, senderAddress, calldata, privKeyHex, maxFee);
}
