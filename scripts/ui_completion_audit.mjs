import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const uiRoots = ["blockchain-explorer/src", "swap-dex/src", "bridge-admin-ui", "lqd-wallet-extension", "mobile-wallet"];
const ignoredDirs = new Set(["node_modules", "dist", "coverage", ".expo", ".idea"]);
const ignoredFiles = new Set(["package-lock.json", "check.txt"]);
const sourceExt = new Set([".js", ".jsx", ".mjs", ".cjs", ".html"]);
const forbidden = [
  ["empty click/press handler", /on(?:Click|Press)=\{\(\)\s*=>\s*\{\s*\}\}/],
  ["coming-soon placeholder", /coming soon/i],
  ["not-yet-supported placeholder", /not yet supported|not supported in browser yet/i],
  ["dead hash link", /href=["']#["']/],
];

function walk(directory, output = []) {
  if (!fs.existsSync(directory)) return output;
  for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
    if (entry.isDirectory() && ignoredDirs.has(entry.name)) continue;
    const target = path.join(directory, entry.name);
    if (entry.isDirectory()) walk(target, output);
    else if (!ignoredFiles.has(entry.name) && sourceExt.has(path.extname(entry.name))) output.push(target);
  }
  return output;
}

const files = uiRoots.flatMap((directory) => walk(path.join(root, directory)));
const failures = [];
for (const file of files) {
  const source = fs.readFileSync(file, "utf8");
  for (const [name, pattern] of forbidden) {
    if (pattern.test(source)) failures.push(`${name}: ${path.relative(root, file)}`);
  }
}

const app = fs.readFileSync(path.join(root, "mobile-wallet", "App.js"), "utf8");
const storage = fs.readFileSync(path.join(root, "mobile-wallet", "src", "storage.js"), "utf8");
const crypto = fs.readFileSync(path.join(root, "mobile-wallet", "src", "crypto.js"), "utf8");
const required = [
  ["NFT import action", /function importNftAction/],
  ["NFT portfolio tab", /homeSubTab === "nfts"/],
  ["NFT persistence key", /nfts: "lqd_mobile_nfts_v1"/],
  ["CW20 transfer signer", /signCosmosWasmExecuteTx/],
  ["all native signer families exposed", /supportedNativeSendFamilies/],
];
for (const [name, pattern] of required) {
  const source = name === "NFT persistence key" ? storage : app;
  if (!pattern.test(source)) failures.push(`missing required UI flow: ${name}`);
}

const cryptoFailures = [
  ["Sui address must use BLAKE2b-256", /suiAddressFromPrivKey[\s\S]*_blake2b256\(hashInput\)/],
  ["TON user-friendly address must include workchain and 36-byte CRC layout", /body\[1\] = workchain[\s\S]*new Uint8Array\(36\)[\s\S]*full\[35\]/],
  ["NEAR transaction must use a single SHA-256", (source) => !/sha256\(sha256\(txBytes\)\)/.test(source)],
];
for (const [name, check] of cryptoFailures) {
  const passed = typeof check === "function" ? check(crypto) : check.test(crypto);
  if (!passed) failures.push(`invalid native wallet implementation: ${name}`);
}

if (failures.length) {
  console.error(failures.map((failure) => `FAIL ${failure}`).join("\n"));
  process.exit(1);
}

console.log(`PASS audited ${files.length} UI source files with no unfinished interaction placeholders`);
console.log("PASS NFT portfolio/import, CW20 transfer and all native signer families are wired");
console.log("PASS Sui, TON and NEAR address/signature invariants are implemented without approximation placeholders");
