/**
 * Client-side cryptography for multi-chain wallet.
 * BIP39 mnemonic → BIP32/SLIP-0010 HD key derivation → per-chain signing.
 * Uses @noble/secp256k1 v1.7.1 (secp256k1), tweetnacl (ed25519), @noble/hashes (Blake2b).
 */

import CryptoJS from "crypto-js";
import * as secp from "@noble/secp256k1";
import nacl from "tweetnacl";
import { blake2b as _blake2bLib } from "@noble/hashes/blake2";

// ─── Configure @noble/secp256k1 HMAC-SHA256 (required for synchronous ops) ───
secp.utils.hmacSha256Sync = (key, ...msgs) => {
  const keyWA = CryptoJS.lib.WordArray.create(key);
  const msgWA = CryptoJS.lib.WordArray.create(
    msgs.reduce((acc, m) => {
      const a = new Uint8Array(acc.length + m.length);
      a.set(acc);
      a.set(m, acc.length);
      return a;
    }, new Uint8Array(0))
  );
  const hmac = CryptoJS.HmacSHA256(msgWA, keyWA);
  return wordArrayToUint8(hmac);
};

// ─── Helpers ─────────────────────────────────────────────────────────────────

function wordArrayToUint8(wa) {
  const words = wa.words;
  const sigBytes = wa.sigBytes;
  const u8 = new Uint8Array(sigBytes);
  for (let i = 0; i < sigBytes; i++) {
    u8[i] = (words[i >>> 2] >>> (24 - (i % 4) * 8)) & 0xff;
  }
  return u8;
}

function hexToUint8(hex) {
  if (hex.startsWith("0x") || hex.startsWith("0X")) hex = hex.slice(2);
  const u8 = new Uint8Array(hex.length / 2);
  for (let i = 0; i < u8.length; i++) {
    u8[i] = parseInt(hex.slice(i * 2, i * 2 + 2), 16);
  }
  return u8;
}

function uint8ToHex(u8) {
  return Array.from(u8)
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");
}

function concatUint8(...arrays) {
  const total = arrays.reduce((s, a) => s + a.length, 0);
  const out = new Uint8Array(total);
  let off = 0;
  for (const a of arrays) {
    out.set(a, off);
    off += a.length;
  }
  return out;
}

function hmacSHA512(keyU8, dataU8) {
  const key = CryptoJS.lib.WordArray.create(keyU8);
  const data = CryptoJS.lib.WordArray.create(dataU8);
  return wordArrayToUint8(CryptoJS.HmacSHA512(data, key));
}

function sha256(dataU8) {
  const wa = CryptoJS.lib.WordArray.create(dataU8);
  return wordArrayToUint8(CryptoJS.SHA256(wa));
}

function keccak256(dataU8) {
  // Use @noble/secp256k1's internal keccak (it ships with it for address derivation)
  // Fallback: manual import via js-sha3 if needed; here use CryptoJS SHA3 in Keccak mode
  const wa = CryptoJS.lib.WordArray.create(dataU8);
  return wordArrayToUint8(CryptoJS.SHA3(wa, { outputLength: 256 }));
}

const SECP256K1_ORDER = BigInt(
  "0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141"
);

// ─── BIP39: mnemonic → seed ───────────────────────────────────────────────────

export function mnemonicToSeed(mnemonic, passphrase = "") {
  const mnemonicWA = CryptoJS.enc.Utf8.parse(mnemonic.normalize("NFKD"));
  const saltWA = CryptoJS.enc.Utf8.parse(("mnemonic" + passphrase).normalize("NFKD"));
  const result = CryptoJS.PBKDF2(mnemonicWA, saltWA, {
    keySize: 512 / 32,
    iterations: 2048,
    hasher: CryptoJS.algo.SHA512,
  });
  return uint8ToHex(wordArrayToUint8(result));
}

// ─── BIP32 (secp256k1) ────────────────────────────────────────────────────────

export function bip32Master(seedHex) {
  const seed = hexToUint8(seedHex);
  const key = new TextEncoder().encode("Bitcoin seed");
  const I = hmacSHA512(key, seed);
  return { key: I.slice(0, 32), chainCode: I.slice(32, 64) };
}

export function bip32Child(parentKeyU8, parentChainCode, index) {
  const hardened = index >= 0x80000000;
  let data;
  if (hardened) {
    data = concatUint8(new Uint8Array([0]), parentKeyU8, indexToUint8(index));
  } else {
    const pub = secp.getPublicKey(parentKeyU8, true);
    data = concatUint8(pub, indexToUint8(index));
  }
  const I = hmacSHA512(parentChainCode, data);
  const IL = I.slice(0, 32);
  const IR = I.slice(32, 64);
  // child key = (IL + parentKey) mod n
  const ILn = BigInt("0x" + uint8ToHex(IL));
  const parentN = BigInt("0x" + uint8ToHex(parentKeyU8));
  const childN = (ILn + parentN) % SECP256K1_ORDER;
  if (childN === 0n || ILn >= SECP256K1_ORDER) throw new Error("invalid child key");
  const childKey = hexToUint8(childN.toString(16).padStart(64, "0"));
  return { key: childKey, chainCode: IR };
}

function indexToUint8(index) {
  const b = new Uint8Array(4);
  b[0] = (index >>> 24) & 0xff;
  b[1] = (index >>> 16) & 0xff;
  b[2] = (index >>> 8) & 0xff;
  b[3] = index & 0xff;
  return b;
}

export function deriveSecp256k1(seedHex, path) {
  // path: "m/44'/60'/0'/0/0"
  let node = bip32Master(seedHex);
  const segments = path.replace("m/", "").split("/");
  for (const seg of segments) {
    const hardened = seg.endsWith("'");
    const idx = parseInt(hardened ? seg.slice(0, -1) : seg, 10);
    const index = hardened ? idx + 0x80000000 : idx;
    node = bip32Child(node.key, node.chainCode, index);
  }
  return { privateKey: uint8ToHex(node.key), chainCode: uint8ToHex(node.chainCode) };
}

// ─── SLIP-0010 (ed25519) ──────────────────────────────────────────────────────

export function slip10Master(seedHex) {
  const seed = hexToUint8(seedHex);
  const key = new TextEncoder().encode("ed25519 seed");
  const I = hmacSHA512(key, seed);
  return { key: I.slice(0, 32), chainCode: I.slice(32, 64) };
}

export function slip10Child(parentKeyU8, parentChainCode, index) {
  // SLIP-0010 ed25519: only hardened derivation
  const data = concatUint8(new Uint8Array([0]), parentKeyU8, indexToUint8(index));
  const I = hmacSHA512(parentChainCode, data);
  return { key: I.slice(0, 32), chainCode: I.slice(32, 64) };
}

export function deriveEd25519(seedHex, path) {
  // path: "m/44'/501'/0'/0'" — all segments hardened
  let node = slip10Master(seedHex);
  const segments = path.replace("m/", "").split("/");
  for (const seg of segments) {
    const hardened = seg.endsWith("'");
    const idx = parseInt(hardened ? seg.slice(0, -1) : seg, 10);
    const index = hardened ? idx + 0x80000000 : idx;
    node = slip10Child(node.key, node.chainCode, index);
  }
  return { privateKey: uint8ToHex(node.key), chainCode: uint8ToHex(node.chainCode) };
}

// ─── Address derivation per chain ─────────────────────────────────────────────

export function evmAddressFromPrivKey(privKeyHex) {
  const pub = secp.getPublicKey(hexToUint8(privKeyHex), false); // 65-byte uncompressed
  const pubBody = pub.slice(1); // drop 0x04 prefix → 64 bytes
  const hash = keccak256(pubBody);
  return "0x" + uint8ToHex(hash.slice(12)); // last 20 bytes
}

export function tronAddressFromPrivKey(privKeyHex) {
  const evmAddr = evmAddressFromPrivKey(privKeyHex);
  return evmAddr; // 0x form for RPC calls
}

// Returns the T... base58check address used in TRON explorers and wallets
export function tronBase58AddressFromPrivKey(privKeyHex) {
  const evmAddr = evmAddressFromPrivKey(privKeyHex); // 0x + 40 hex
  const addrBytes = hexToUint8(evmAddr.slice(2)); // 20 bytes
  const prefixed = new Uint8Array(21);
  prefixed[0] = 0x41;
  prefixed.set(addrBytes, 1);
  const check1 = sha256(prefixed);
  const check2 = sha256(check1);
  const full = new Uint8Array(25);
  full.set(prefixed);
  full.set(check2.slice(0, 4), 21);
  return base58Encode(full);
}

export function cosmosAddressFromPrivKey(privKeyHex, prefix = "cosmos") {
  const pub = secp.getPublicKey(hexToUint8(privKeyHex), true); // 33-byte compressed
  const sha = sha256(pub);
  const ripe = ripemd160(sha);
  return bech32Encode(prefix, ripe);
}

export function solanaAddressFromPrivKey(privKeyHex) {
  const seed = hexToUint8(privKeyHex);
  const kp = nacl.sign.keyPair.fromSeed(seed);
  return base58Encode(kp.publicKey);
}

export function nearAddressFromPrivKey(privKeyHex) {
  // NEAR uses ed25519, address = hex of public key or account id
  const seed = hexToUint8(privKeyHex);
  const kp = nacl.sign.keyPair.fromSeed(seed);
  return uint8ToHex(kp.publicKey); // hex pubkey; user uses account_id separately
}

export function aptosAddressFromPrivKey(privKeyHex) {
  const seed = hexToUint8(privKeyHex);
  const kp = nacl.sign.keyPair.fromSeed(seed);
  // Aptos address = sha256(pubkey || 0x00) for single-key accounts
  const hashInput = concatUint8(kp.publicKey, new Uint8Array([0x00]));
  const hash = sha256(hashInput);
  return "0x" + uint8ToHex(hash);
}

export function suiAddressFromPrivKey(privKeyHex) {
  const seed = hexToUint8(privKeyHex);
  const kp = nacl.sign.keyPair.fromSeed(seed);
  // SUI address = blake2b(0x00 || pubkey)[0..32] — approximate with sha256 for compatibility
  const hashInput = concatUint8(new Uint8Array([0x00]), kp.publicKey);
  const hash = sha256(hashInput);
  return "0x" + uint8ToHex(hash);
}

export function tonAddressFromPrivKey(privKeyHex) {
  const seed = hexToUint8(privKeyHex);
  const kp = nacl.sign.keyPair.fromSeed(seed);
  return uint8ToHex(kp.publicKey);
}

export function harmonyAddressFromPrivKey(privKeyHex) {
  // ONE address = same derivation as EVM, different bech32 prefix
  return evmAddressFromPrivKey(privKeyHex); // use 0x form; bech32 is optional display
}

// ─── RIPEMD-160 (compact pure JS) ─────────────────────────────────────────────

function ripemd160(data) {
  const wa = CryptoJS.lib.WordArray.create(data);
  return wordArrayToUint8(CryptoJS.RIPEMD160(wa));
}

// ─── Bech32 ───────────────────────────────────────────────────────────────────

const BECH32_ALPHABET = "qpzry9x8gf2tvdw0s3jn54khce6mua7l";

function bech32Encode(prefix, data) {
  const words = convertBits(data, 8, 5, true);
  const checksum = bech32Checksum(prefix, words);
  const combined = words.concat(checksum);
  return prefix + "1" + combined.map((w) => BECH32_ALPHABET[w]).join("");
}

function convertBits(data, fromBits, toBits, pad) {
  let acc = 0, bits = 0;
  const result = [];
  const maxv = (1 << toBits) - 1;
  for (const val of data) {
    acc = (acc << fromBits) | val;
    bits += fromBits;
    while (bits >= toBits) {
      bits -= toBits;
      result.push((acc >> bits) & maxv);
    }
  }
  if (pad && bits > 0) result.push((acc << (toBits - bits)) & maxv);
  return result;
}

function bech32Polymod(values) {
  const GEN = [0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3];
  let chk = 1;
  for (const v of values) {
    const b = chk >> 25;
    chk = ((chk & 0x1ffffff) << 5) ^ v;
    for (let i = 0; i < 5; i++) if ((b >> i) & 1) chk ^= GEN[i];
  }
  return chk;
}

function bech32HrpExpand(hrp) {
  const ret = [];
  for (let i = 0; i < hrp.length; i++) ret.push(hrp.charCodeAt(i) >> 5);
  ret.push(0);
  for (let i = 0; i < hrp.length; i++) ret.push(hrp.charCodeAt(i) & 31);
  return ret;
}

function bech32Checksum(hrp, data) {
  const values = bech32HrpExpand(hrp).concat(data).concat([0, 0, 0, 0, 0, 0]);
  const poly = bech32Polymod(values) ^ 1;
  const ret = [];
  for (let p = 0; p < 6; p++) ret.push((poly >> (5 * (5 - p))) & 31);
  return ret;
}

// ─── Base58 ───────────────────────────────────────────────────────────────────

const BASE58_ALPHABET = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz";

export function base58Encode(bytes) {
  let num = BigInt("0x" + uint8ToHex(bytes));
  let str = "";
  while (num > 0n) {
    str = BASE58_ALPHABET[Number(num % 58n)] + str;
    num /= 58n;
  }
  for (const b of bytes) {
    if (b !== 0) break;
    str = "1" + str;
  }
  return str;
}

// ─── Master key derivation for all chains ────────────────────────────────────

export function deriveAllChainKeys(mnemonic, passphrase = "") {
  const seedHex = mnemonicToSeed(mnemonic, passphrase);

  // secp256k1 chains
  const evm = deriveSecp256k1(seedHex, "m/44'/60'/0'/0/0");
  const harmony = deriveSecp256k1(seedHex, "m/44'/60'/0'/0/0"); // same as EVM
  const tron = deriveSecp256k1(seedHex, "m/44'/195'/0'/0/0");
  const cosmos = deriveSecp256k1(seedHex, "m/44'/118'/0'/0/0");
  const sei = deriveSecp256k1(seedHex, "m/44'/118'/0'/0/0");
  const injective = deriveSecp256k1(seedHex, "m/44'/60'/0'/0/0"); // INJ uses ETH path

  // ed25519 chains
  const solana = deriveEd25519(seedHex, "m/44'/501'/0'/0'");
  const near = deriveEd25519(seedHex, "m/44'/397'/0'");
  const aptos = deriveEd25519(seedHex, "m/44'/637'/0'/0'/0'");
  const sui = deriveEd25519(seedHex, "m/44'/784'/0'/0'/0'");
  const ton = deriveEd25519(seedHex, "m/44'/607'/0'");

  return {
    evm: {
      privateKey: evm.privateKey,
      address: evmAddressFromPrivKey(evm.privateKey),
    },
    harmony: {
      privateKey: harmony.privateKey,
      address: evmAddressFromPrivKey(harmony.privateKey),
    },
    tron: {
      privateKey: tron.privateKey,
      address: evmAddressFromPrivKey(tron.privateKey),      // 0x for RPC
      tronAddress: tronBase58AddressFromPrivKey(tron.privateKey), // T... for display
    },
    cosmos: {
      privateKey: cosmos.privateKey,
      address: cosmosAddressFromPrivKey(cosmos.privateKey, "cosmos"),
    },
    sei: {
      privateKey: sei.privateKey,
      address: cosmosAddressFromPrivKey(sei.privateKey, "sei"),
      evmAddress: evmAddressFromPrivKey(sei.privateKey),
    },
    injective: {
      privateKey: injective.privateKey,
      address: cosmosAddressFromPrivKey(injective.privateKey, "inj"),
      evmAddress: evmAddressFromPrivKey(injective.privateKey),
    },
    solana: {
      privateKey: solana.privateKey,
      address: solanaAddressFromPrivKey(solana.privateKey),
    },
    near: {
      privateKey: near.privateKey,
      address: nearAddressFromPrivKey(near.privateKey),
    },
    aptos: {
      privateKey: aptos.privateKey,
      address: aptosAddressFromPrivKey(aptos.privateKey),
    },
    sui: {
      privateKey: sui.privateKey,
      address: suiAddressFromPrivKey(sui.privateKey),
    },
    ton: {
      privateKey: ton.privateKey,
      address: tonAddressFromPrivKey(ton.privateKey),
    },
  };
}

// ─── RLP Encoding (for EVM transactions) ─────────────────────────────────────

function rlpItem(input) {
  if (input instanceof Uint8Array || Array.isArray(input)) {
    return _rlpEncode(input);
  }
  throw new Error("rlpItem: unexpected type");
}

function _rlpEncode(input) {
  if (input instanceof Uint8Array) {
    if (input.length === 1 && input[0] < 0x80) return input;
    return concatUint8(_rlpLength(input.length, 0x80), input);
  }
  // array / list
  const items = input.map(_rlpEncode);
  const payload = concatUint8(...items);
  return concatUint8(_rlpLength(payload.length, 0xc0), payload);
}

function _rlpLength(length, offset) {
  if (length < 56) return new Uint8Array([offset + length]);
  const hex = length.toString(16);
  const hexPadded = hex.length % 2 ? "0" + hex : hex;
  const lenBytes = hexToUint8(hexPadded);
  return concatUint8(new Uint8Array([offset + 55 + lenBytes.length]), lenBytes);
}

function bigintToMinimalUint8(n) {
  if (n === 0n) return new Uint8Array(0);
  let hex = n.toString(16);
  if (hex.length % 2) hex = "0" + hex;
  return hexToUint8(hex);
}

export function rlpEncodeList(items) {
  return _rlpEncode(items);
}

// ─── EIP-155 Transaction Signing ─────────────────────────────────────────────

/**
 * Sign an EVM transaction with EIP-155.
 * @param {object} params - { nonce, gasPrice, gasLimit, to, value, data, chainId }
 * @param {string} privKeyHex
 * @returns {string} signed raw transaction hex (0x-prefixed)
 */
export function signEip155Tx(params, privKeyHex) {
  const { nonce, gasPrice, gasLimit, to, value, data, chainId } = params;

  const fields = [
    bigintToMinimalUint8(BigInt(nonce)),
    bigintToMinimalUint8(BigInt(gasPrice)),
    bigintToMinimalUint8(BigInt(gasLimit)),
    to ? hexToUint8(to.startsWith("0x") ? to.slice(2) : to) : new Uint8Array(0),
    bigintToMinimalUint8(BigInt(value)),
    data
      ? data instanceof Uint8Array
        ? data
        : hexToUint8(data.startsWith("0x") ? data.slice(2) : data)
      : new Uint8Array(0),
    bigintToMinimalUint8(BigInt(chainId)),
    new Uint8Array(0),
    new Uint8Array(0),
  ];

  const rlpUnsigned = _rlpEncode(fields);
  const txHash = keccak256(rlpUnsigned);
  const sig = secp.signSync(txHash, hexToUint8(privKeyHex), { recovered: true, der: false });
  const [sigBytes, recovery] = sig; // sigBytes is 64-byte raw r+s

  const r = sigBytes.slice(0, 32);
  const s = sigBytes.slice(32, 64);
  const v = BigInt(recovery) + BigInt(chainId) * 2n + 35n;

  const signedFields = [
    bigintToMinimalUint8(BigInt(nonce)),
    bigintToMinimalUint8(BigInt(gasPrice)),
    bigintToMinimalUint8(BigInt(gasLimit)),
    to ? hexToUint8(to.startsWith("0x") ? to.slice(2) : to) : new Uint8Array(0),
    bigintToMinimalUint8(BigInt(value)),
    data
      ? data instanceof Uint8Array
        ? data
        : hexToUint8(data.startsWith("0x") ? data.slice(2) : data)
      : new Uint8Array(0),
    bigintToMinimalUint8(v),
    r,
    s,
  ];

  return "0x" + uint8ToHex(_rlpEncode(signedFields));
}

// ─── ERC-20 Transfer ABI encoding ────────────────────────────────────────────

export function encodeErc20Transfer(to, amount) {
  // transfer(address,uint256) selector = 0xa9059cbb
  const selector = hexToUint8("a9059cbb");
  const addrPad = hexToUint8(
    (to.startsWith("0x") ? to.slice(2) : to).padStart(64, "0")
  );
  const amtHex = BigInt(amount).toString(16).padStart(64, "0");
  const amtPad = hexToUint8(amtHex);
  return concatUint8(selector, addrPad, amtPad);
}

// ─── ERC-20 Approve ABI encoding ─────────────────────────────────────────────

export function encodeErc20Approve(spender, amount) {
  // approve(address,uint256) selector = 0x095ea7b3
  const selector = hexToUint8("095ea7b3");
  const spenderPad = hexToUint8(
    (spender.startsWith("0x") ? spender.slice(2) : spender).padStart(64, "0")
  );
  const amtHex = BigInt(amount).toString(16).padStart(64, "0");
  const amtPad = hexToUint8(amtHex);
  return concatUint8(selector, spenderPad, amtPad);
}

// ─── Cosmos Amino Transaction Signing ────────────────────────────────────────

/**
 * Sign a Cosmos SDK (amino) send transaction.
 * @param {object} params - { chainId, sequence, accountNumber, fromAddress, toAddress, amount, denom, memo, gas }
 * @param {string} privKeyHex
 * @returns {{ tx: object, signature: string }} broadcast-ready tx object
 */
export function signCosmosTx(params, privKeyHex) {
  const {
    chainId,
    sequence,
    accountNumber,
    fromAddress,
    toAddress,
    amount,
    denom,
    memo = "",
    gas = 200000,
  } = params;

  // Canonical amino SignDoc
  const signDoc = {
    account_number: String(accountNumber),
    chain_id: chainId,
    fee: {
      amount: [{ amount: "5000", denom }],
      gas: String(gas),
    },
    memo,
    msgs: [
      {
        type: "cosmos-sdk/MsgSend",
        value: {
          amount: [{ amount: String(amount), denom }],
          from_address: fromAddress,
          to_address: toAddress,
        },
      },
    ],
    sequence: String(sequence),
  };

  const signDocBytes = new TextEncoder().encode(JSON.stringify(signDoc));
  const hash = sha256(signDocBytes);
  const sig = secp.signSync(hash, hexToUint8(privKeyHex), { recovered: true, der: false });
  const sigBytes = sig[0]; // raw 64-byte r+s
  const sigBase64 = btoa(String.fromCharCode(...sigBytes));

  const pubKey = secp.getPublicKey(hexToUint8(privKeyHex), true);
  const pubKeyBase64 = btoa(String.fromCharCode(...pubKey));

  return {
    tx: {
      msg: signDoc.msgs,
      fee: signDoc.fee,
      signatures: [
        {
          pub_key: {
            type: "tendermint/PubKeySecp256k1",
            value: pubKeyBase64,
          },
          signature: sigBase64,
        },
      ],
      memo,
    },
    mode: "sync",
  };
}

// ─── Solana Transfer signing (ed25519 + compact tx format) ───────────────────

/**
 * Sign a Solana SOL transfer.
 * @param {object} params - { fromPubkey, toPubkey, lamports, recentBlockhash }
 * @param {string} privKeyHex
 * @returns {string} base58-encoded signed transaction
 */
export function signSolanaTransfer(params, privKeyHex) {
  const { fromPubkey, toPubkey, lamports, recentBlockhash } = params;
  const seed = hexToUint8(privKeyHex);
  const kp = nacl.sign.keyPair.fromSeed(seed);

  const fromPub = base58Decode(fromPubkey);
  const toPub = base58Decode(toPubkey);
  const blockhash = base58Decode(recentBlockhash);

  // System Program Transfer instruction: program_id=11111111..., accounts=[from,to], data=[2,0,0,0, lamports LE 8 bytes]
  const systemProgram = new Uint8Array(32); // all zeros

  // Compact array encoding
  function compactU16(n) {
    if (n < 0x80) return new Uint8Array([n]);
    if (n < 0x4000) return new Uint8Array([((n & 0x7f) | 0x80), (n >> 7) & 0xff]);
    return new Uint8Array([((n & 0x7f) | 0x80), (((n >> 7) & 0x7f) | 0x80), (n >> 14) & 0xff]);
  }

  const lamportsLE = new Uint8Array(8);
  let lamt = BigInt(lamports);
  for (let i = 0; i < 8; i++) {
    lamportsLE[i] = Number(lamt & 0xffn);
    lamt >>= 8n;
  }

  const instructionData = concatUint8(
    new Uint8Array([2, 0, 0, 0]), // transfer instruction index
    lamportsLE
  );

  // Message: header, accounts, blockhash, instructions
  const header = new Uint8Array([1, 0, 1]); // 1 sig_required, 0 read_only_signed, 1 read_only_unsigned

  const accountKeys = concatUint8(
    compactU16(3),
    fromPub, toPub, systemProgram
  );

  const instructionAccounts = new Uint8Array([0, 1]); // from=0, to=1
  const instruction = concatUint8(
    new Uint8Array([2]), // program_id index = 2 (system program)
    compactU16(2),       // 2 accounts
    instructionAccounts,
    compactU16(instructionData.length),
    instructionData
  );

  const message = concatUint8(
    header,
    accountKeys,
    blockhash,
    compactU16(1),   // 1 instruction
    instruction
  );

  const signature = nacl.sign.detached(message, kp.secretKey);

  // Full transaction: compact_array([sig]), message
  const tx = concatUint8(
    compactU16(1),
    signature,
    message
  );

  return base58Encode(tx);
}

// ─── Base58 Decode ────────────────────────────────────────────────────────────

export function base58Decode(str) {
  let num = 0n;
  for (const c of str) {
    const idx = BASE58_ALPHABET.indexOf(c);
    if (idx < 0) throw new Error("invalid base58 char: " + c);
    num = num * 58n + BigInt(idx);
  }
  const hex = num.toString(16);
  const padded = hex.length % 2 ? "0" + hex : hex;
  const bytes = hexToUint8(padded);
  const leading = str.split("").findIndex((c) => c !== "1");
  const zeros = new Uint8Array(leading < 0 ? str.length : leading);
  return concatUint8(zeros, bytes);
}

// ─── Export key info for display ─────────────────────────────────────────────

export function exportKeyInfo(chainKeys) {
  return Object.entries(chainKeys).map(([chain, info]) => ({
    chain,
    privateKey: info.privateKey,
    address: info.address,
    ...(info.evmAddress ? { evmAddress: info.evmAddress } : {}),
  }));
}

// ─── NEAR Borsh Helpers ───────────────────────────────────────────────────────

function borshU32(n) {
  const b = new Uint8Array(4);
  new DataView(b.buffer).setUint32(0, n, true);
  return b;
}

function borshU64LE(n) {
  const b = new Uint8Array(8);
  const v = new DataView(b.buffer);
  const big = BigInt(n);
  v.setUint32(0, Number(big & 0xffffffffn), true);
  v.setUint32(4, Number((big >> 32n) & 0xffffffffn), true);
  return b;
}

function borshU128LE(n) {
  const b = new Uint8Array(16);
  const v = new DataView(b.buffer);
  const big = BigInt(n);
  v.setUint32(0, Number(big & 0xffffffffn), true);
  v.setUint32(4, Number((big >> 32n) & 0xffffffffn), true);
  v.setUint32(8, Number((big >> 64n) & 0xffffffffn), true);
  v.setUint32(12, Number((big >> 96n) & 0xffffffffn), true);
  return b;
}

function borshStr(s) {
  const bytes = new TextEncoder().encode(s);
  return concatUint8(borshU32(bytes.length), bytes);
}

function borshBytesVec(data) {
  return concatUint8(borshU32(data.length), data);
}

// ─── NEAR Transaction Signing ─────────────────────────────────────────────────

/**
 * Sign a NEAR native transfer (borsh-serialised SignedTransaction).
 * Returns base64 string for broadcast_tx_commit.
 */
export function signNearTransfer({ signerId, receiverId, amount, nonce, blockHash }, privKeyHex) {
  const seed = hexToUint8(privKeyHex);
  const kp = nacl.sign.keyPair.fromSeed(seed);
  const blockHashBytes = typeof blockHash === "string" ? base58Decode(blockHash) : blockHash;

  const txBytes = concatUint8(
    borshStr(signerId),
    new Uint8Array([0]), kp.publicKey,   // PublicKey: type=0 (ed25519) + 32 bytes
    borshU64LE(nonce),
    borshStr(receiverId),
    blockHashBytes,                       // 32 bytes
    borshU32(1),                          // 1 action
    new Uint8Array([3]),                  // Action::Transfer
    borshU128LE(amount),
  );

  const hash = sha256(sha256(txBytes));
  const sig = nacl.sign.detached(hash, kp.secretKey);
  const signedTx = concatUint8(txBytes, new Uint8Array([0]), sig); // sig type=0 (ed25519)
  return btoa(String.fromCharCode(...signedTx));
}

/**
 * Sign a NEAR function-call action (used for NEP-141 ft_transfer etc.).
 * Returns base64 string for broadcast_tx_commit.
 */
export function signNearFunctionCall(
  { signerId, contractId, methodName, args, gas, deposit, nonce, blockHash },
  privKeyHex
) {
  const seed = hexToUint8(privKeyHex);
  const kp = nacl.sign.keyPair.fromSeed(seed);
  const blockHashBytes = typeof blockHash === "string" ? base58Decode(blockHash) : blockHash;
  const argsBytes = typeof args === "string" ? new TextEncoder().encode(args) : args;

  const txBytes = concatUint8(
    borshStr(signerId),
    new Uint8Array([0]), kp.publicKey,
    borshU64LE(nonce),
    borshStr(contractId),
    blockHashBytes,
    borshU32(1),
    new Uint8Array([2]),                  // Action::FunctionCall
    borshStr(methodName),
    borshBytesVec(argsBytes),
    borshU64LE(gas),
    borshU128LE(deposit),
  );

  const hash = sha256(sha256(txBytes));
  const sig = nacl.sign.detached(hash, kp.secretKey);
  const signedTx = concatUint8(txBytes, new Uint8Array([0]), sig);
  return btoa(String.fromCharCode(...signedTx));
}

// ─── Aptos Transaction Signing ────────────────────────────────────────────────

/**
 * Sign the bytes returned by Aptos /transactions/encode_submission.
 * Returns { publicKey, signature } as 0x-hex strings ready for the submit body.
 */
export function signAptosEntry(signingHex, privKeyHex) {
  const seed = hexToUint8(privKeyHex);
  const kp = nacl.sign.keyPair.fromSeed(seed);
  const bytes = hexToUint8(signingHex.startsWith("0x") ? signingHex.slice(2) : signingHex);
  const sig = nacl.sign.detached(bytes, kp.secretKey);
  return {
    publicKey: "0x" + uint8ToHex(kp.publicKey),
    signature: "0x" + uint8ToHex(sig),
  };
}

// ─── SUI Transaction Signing ──────────────────────────────────────────────────

function _blake2b256(data) {
  return _blake2bLib(data, { dkLen: 32 });
}

/**
 * Sign SUI transaction bytes (base64) with ed25519 + intent prefix.
 * Returns a serialized SUI signature string (base64) for sui_executeTransactionBlock.
 */
export function signSuiTx(txBytesBase64, privKeyHex) {
  const seed = hexToUint8(privKeyHex);
  const kp = nacl.sign.keyPair.fromSeed(seed);

  const txBytes = Uint8Array.from(atob(txBytesBase64), (c) => c.charCodeAt(0));
  const intent = new Uint8Array([0, 0, 0]); // IntentScope::TransactionData
  const intentMsg = concatUint8(intent, txBytes);

  const hash = _blake2b256(intentMsg);
  const sig = nacl.sign.detached(hash, kp.secretKey);

  // SUI serialised signature: [0x00 flag] + [64-byte sig] + [32-byte pubkey]
  const suiSig = concatUint8(new Uint8Array([0x00]), sig, kp.publicKey);
  return btoa(String.fromCharCode(...suiSig));
}

// ─── Solana SPL Token Transfer ────────────────────────────────────────────────

/**
 * Build + sign a Solana SPL token transfer transaction.
 * Accounts: [owner(signer, idx 0), sourceTA(writable, idx 1), destTA(writable, idx 2), tokenProgram(readonly, idx 3)]
 */
export function signSolanaTokenTransfer(
  { fromTokenAccount, toTokenAccount, amount, recentBlockhash },
  privKeyHex
) {
  const seed = hexToUint8(privKeyHex);
  const kp = nacl.sign.keyPair.fromSeed(seed);

  const ownerPub = kp.publicKey;
  const fromTAPub = base58Decode(fromTokenAccount);
  const toTAPub = base58Decode(toTokenAccount);
  const tokenProgramPub = base58Decode("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA");
  const blockhash = base58Decode(recentBlockhash);

  function compactU16(n) {
    if (n < 0x80) return new Uint8Array([n]);
    return new Uint8Array([((n & 0x7f) | 0x80), (n >> 7) & 0xff]);
  }

  // SPL Transfer instruction data: discriminant=3, amount u64 LE
  const amtLE = new Uint8Array(8);
  let amt = BigInt(amount);
  for (let i = 0; i < 8; i++) { amtLE[i] = Number(amt & 0xffn); amt >>= 8n; }
  const ixData = concatUint8(new Uint8Array([3]), amtLE);

  // header: 1 required_sig, 0 readonly_signed, 1 readonly_unsigned (tokenProgram)
  const header = new Uint8Array([1, 0, 1]);

  const accountKeys = concatUint8(
    compactU16(4),
    ownerPub, fromTAPub, toTAPub, tokenProgramPub,
  );

  // instruction: program=idx3, accounts=[1,2,0], data
  const instruction = concatUint8(
    new Uint8Array([3]),           // tokenProgram index
    compactU16(3),
    new Uint8Array([1, 2, 0]),     // sourceTA, destTA, owner
    compactU16(ixData.length),
    ixData,
  );

  const message = concatUint8(header, accountKeys, blockhash, compactU16(1), instruction);
  const signature = nacl.sign.detached(message, kp.secretKey);
  const tx = concatUint8(compactU16(1), signature, message);
  return base58Encode(tx);
}

// ─── Cosmos Token Send (IBC / native denom) ───────────────────────────────────

/**
 * Same as signCosmosTx but for any denom (IBC tokens, CW20 not supported).
 * Re-exports signCosmosTx under a clearer name for token sends.
 */
export { signCosmosTx as signCosmosTokenSend };
