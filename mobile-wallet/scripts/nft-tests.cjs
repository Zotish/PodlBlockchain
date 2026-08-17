const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");

const sourcePath = path.join(__dirname, "..", "src", "nft.js");
let source = fs.readFileSync(sourcePath, "utf8")
  .replace(/^import .*;\s*$/gm, "")
  .replace(/\bexport\s+(?=(?:async\s+)?function\b|const\b|let\b|class\b)/g, "");
source += "\nglobalThis.__nftExports = { nftKey, normalizeNftUri, validateNftIdentifier, decodeAbiString, resolveEvmNftMetadata };";

const owner = "0x1111111111111111111111111111111111111111";
const abiString = (value) => {
  const body = Buffer.from(value, "utf8").toString("hex");
  const padded = body.padEnd(Math.ceil(body.length / 64) * 64, "0");
  return `0x${"20".padStart(64, "0")}${(body.length / 2).toString(16).padStart(64, "0")}${padded}`;
};

const postJson = async (_url, body) => {
  if (body.method === "eth_getCode") return { result: "0x60016000" };
  const data = body.params?.[0]?.data || "";
  if (data.startsWith("0x6352211e")) return { result: `0x${owner.slice(2).padStart(64, "0")}` };
  if (data.startsWith("0xc87b56dd")) {
    return { result: abiString(`data:application/json,${encodeURIComponent(JSON.stringify({ name: "Test NFT", image: "ipfs://image-cid" }))}`) };
  }
  if (data === "0x06fdde03") return { result: abiString("Test Collection") };
  if (data === "0x95d89b41") return { result: abiString("TNFT") };
  return { result: "0x" };
};

const context = {
  postJson,
  isLikelyAddressForFamily: (address, family) => {
    if (family === "solana") return /^[1-9A-HJ-NP-Za-km-z]{32,44}$/.test(address);
    if (family === "tron") return /^T[A-Za-z1-9]{33}$/.test(address);
    return String(address || "").length >= 10;
  },
  fetch,
  AbortController,
  atob,
  setTimeout,
  clearTimeout,
  console,
  BigInt,
  JSON,
  decodeURIComponent,
  encodeURIComponent,
};
vm.createContext(context);
vm.runInContext(source, context, { filename: sourcePath });
const nft = context.__nftExports;

assert.equal(nft.nftKey({ networkId: "ETH", address: "0xABC", tokenId: 7 }), "eth:0xabc:7");
assert.equal(nft.normalizeNftUri("ipfs://abc/metadata.json"), "https://ipfs.io/ipfs/abc/metadata.json");
assert.equal(nft.normalizeNftUri("ar://abc"), "https://arweave.net/abc");
assert.equal(nft.validateNftIdentifier("0x1234567890123456789012345678901234567890", "42", "evm"), true);
assert.equal(nft.validateNftIdentifier("0x1234567890123456789012345678901234567890", "", "evm"), false);
assert.equal(nft.validateNftIdentifier("tb1qexample", "1", "utxo"), false);
assert.equal(nft.decodeAbiString(abiString("Collection")), "Collection");

(async () => {
  const result = await nft.resolveEvmNftMetadata({
    rpcUrl: "https://rpc.example",
    contract: "0x2222222222222222222222222222222222222222",
    tokenId: "7",
    ownerAddress: owner,
  });
  assert.equal(result.name, "Test NFT");
  assert.equal(result.symbol, "TNFT");
  assert.equal(result.image, "https://ipfs.io/ipfs/image-cid");
  assert.equal(result.ownershipVerified, true);
  assert.equal(result.standard, "ERC-721");
  console.log("PASS NFT utility, URI normalization, ABI decoding and mocked ERC-721 verification");
})().catch((error) => {
  console.error(error);
  process.exit(1);
});
