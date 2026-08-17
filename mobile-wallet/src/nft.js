import { postJson } from "./api";
import { isLikelyAddressForFamily } from "./utils";

const EVM_ADDRESS_RE = /^0x[a-fA-F0-9]{40}$/;
const UINT256_RE = /^(?:0x[0-9a-fA-F]+|[0-9]+)$/;

export function nftKey(nft) {
  return [
    String(nft?.networkId || "").toLowerCase(),
    String(nft?.address || "").toLowerCase(),
    String(nft?.tokenId ?? "").toLowerCase(),
  ].join(":");
}

export function normalizeNftUri(value) {
  const raw = String(value || "").trim();
  if (!raw) return "";
  if (raw.startsWith("ipfs://ipfs/")) return `https://ipfs.io/ipfs/${raw.slice(12)}`;
  if (raw.startsWith("ipfs://")) return `https://ipfs.io/ipfs/${raw.slice(7)}`;
  if (raw.startsWith("ar://")) return `https://arweave.net/${raw.slice(5)}`;
  return raw;
}

export function validateNftIdentifier(address, tokenId, family) {
  const fam = String(family || "evm").toLowerCase();
  const id = String(tokenId ?? "").trim();
  if (!id || id.length > 160) return false;
  if (["evm", "harmony"].includes(fam)) {
    return EVM_ADDRESS_RE.test(String(address || "").trim()) && UINT256_RE.test(id);
  }
  if (fam === "tron") {
    return isLikelyAddressForFamily(address, "tron") && UINT256_RE.test(id);
  }
  if (fam === "sei" && EVM_ADDRESS_RE.test(String(address || "").trim())) {
    return UINT256_RE.test(id);
  }
  if (["utxo", "litecoin"].includes(fam)) return false;
  return isLikelyAddressForFamily(address, fam);
}

function encodeUint256(value) {
  const number = BigInt(String(value || "0"));
  if (number < 0n || number >= (1n << 256n)) throw new Error("NFT token ID is outside uint256 range");
  return number.toString(16).padStart(64, "0");
}

function hexToUtf8(hex) {
  const clean = String(hex || "").replace(/^0x/, "");
  let encoded = "";
  for (let i = 0; i + 1 < clean.length; i += 2) {
    const byte = parseInt(clean.slice(i, i + 2), 16);
    if (byte) encoded += `%${byte.toString(16).padStart(2, "0")}`;
  }
  try {
    return decodeURIComponent(encoded);
  } catch {
    let output = "";
    for (let i = 0; i + 1 < clean.length; i += 2) {
      const byte = parseInt(clean.slice(i, i + 2), 16);
      if (byte) output += String.fromCharCode(byte);
    }
    return output;
  }
}

export function decodeAbiString(value) {
  const clean = String(value || "").replace(/^0x/, "");
  if (!clean || clean.length < 64) return "";
  try {
    if (clean.length === 64) return hexToUtf8(clean).replace(/\0+$/g, "");
    const offset = Number(BigInt(`0x${clean.slice(0, 64)}`)) * 2;
    if (!Number.isSafeInteger(offset) || offset < 0 || offset + 64 > clean.length) return "";
    const length = Number(BigInt(`0x${clean.slice(offset, offset + 64)}`));
    if (!Number.isSafeInteger(length) || length < 0 || length > 1024 * 1024) return "";
    return hexToUtf8(clean.slice(offset + 64, offset + 64 + (length * 2)));
  } catch {
    return "";
  }
}

function decodeAbiAddress(value) {
  const clean = String(value || "").replace(/^0x/, "");
  if (clean.length < 64) return "";
  return `0x${clean.slice(-40)}`.toLowerCase();
}

export async function resolveNftUriMetadata(uri, tokenId = "") {
  const normalized = normalizeNftUri(uri);
  if (!normalized) return null;
  if (normalized.startsWith("data:application/json;base64,")) {
    try {
      return JSON.parse(atob(normalized.slice("data:application/json;base64,".length)));
    } catch {
      return null;
    }
  }
  if (normalized.startsWith("data:application/json,")) {
    try {
      return JSON.parse(decodeURIComponent(normalized.slice("data:application/json,".length)));
    } catch {
      return null;
    }
  }
  if (!/^https:\/\//i.test(normalized)) return null;
  const controller = typeof AbortController !== "undefined" ? new AbortController() : null;
  const timer = controller ? setTimeout(() => controller.abort(), 8000) : null;
  try {
    const response = await fetch(normalized, {
      headers: { Accept: "application/json" },
      signal: controller?.signal,
    });
    if (!response.ok) return null;
    const contentLength = Number(response.headers?.get?.("content-length") || 0);
    if (contentLength > 2_000_000) return null;
    const body = await response.text();
    if (body.length > 2_000_000) return null;
    return JSON.parse(body);
  } catch {
    return null;
  } finally {
    if (timer) clearTimeout(timer);
  }
}

export async function resolveEvmNftMetadata({ rpcUrl, contract, tokenId, ownerAddress = "" }) {
  const address = String(contract || "").trim();
  if (!EVM_ADDRESS_RE.test(address)) throw new Error("EVM NFT contract address is invalid");
  const idWord = encodeUint256(tokenId);
  const rpc = async (method, params) => {
    const response = await postJson(rpcUrl, { jsonrpc: "2.0", id: 1, method, params });
    if (response?.error) throw new Error(response.error.message || `${method} failed`);
    return response?.result;
  };

  const code = await rpc("eth_getCode", [address, "latest"]);
  if (!code || code === "0x" || code === "0x0") throw new Error("No NFT contract is deployed at this address");

  const call = (data) => rpc("eth_call", [{ to: address, data }, "latest"]);
  const ownerWord = String(ownerAddress || "").replace(/^0x/, "").padStart(64, "0");
  const [ownerResult, tokenUriResult, erc1155UriResult, erc1155BalanceResult, nameResult, symbolResult] = await Promise.all([
    call(`0x6352211e${idWord}`).catch(() => ""),
    call(`0xc87b56dd${idWord}`).catch(() => ""),
    call(`0x0e89341c${idWord}`).catch(() => ""),
    EVM_ADDRESS_RE.test(ownerAddress)
      ? call(`0x00fdd58e${ownerWord}${idWord}`).catch(() => "")
      : Promise.resolve(""),
    call("0x06fdde03").catch(() => ""),
    call("0x95d89b41").catch(() => ""),
  ]);

  const owner = decodeAbiAddress(ownerResult);
  let erc1155Balance = 0n;
  try {
    if (erc1155BalanceResult && erc1155BalanceResult !== "0x") erc1155Balance = BigInt(erc1155BalanceResult);
  } catch { /* malformed RPC result is treated as no verified balance */ }
  const rawTokenUri = decodeAbiString(tokenUriResult) || decodeAbiString(erc1155UriResult);
  const tokenUri = rawTokenUri.replace(/\{id\}/gi, idWord.toLowerCase());
  const metadata = await resolveNftUriMetadata(tokenUri, tokenId);
  const image = normalizeNftUri(metadata?.image || metadata?.image_url || "");
  const expectedOwner = String(ownerAddress || "").toLowerCase();
  return {
    name: String(metadata?.name || decodeAbiString(nameResult) || `NFT #${tokenId}`),
    symbol: decodeAbiString(symbolResult),
    description: String(metadata?.description || ""),
    image,
    tokenUri: normalizeNftUri(tokenUri),
    owner,
    ownershipVerified: Boolean(
      (owner && expectedOwner && owner === expectedOwner) ||
      (!owner && expectedOwner && erc1155Balance > 0n)
    ),
    ownershipMismatch: Boolean(owner && expectedOwner && owner !== expectedOwner),
    standard: owner ? "ERC-721" : erc1155UriResult ? "ERC-1155" : "EVM NFT",
    contractVerified: true,
  };
}
