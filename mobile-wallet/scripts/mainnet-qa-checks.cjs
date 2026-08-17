const fs = require("fs");
const path = require("path");

const root = path.resolve(__dirname, "..");
const read = (file) => fs.readFileSync(path.join(root, file), "utf8");

const files = {
  app: read("App.js"),
  api: read("src/api.js"),
  storage: read("src/storage.js"),
  tracker: read("src/txTracker.js"),
  migrations: read("src/migrations.js"),
  nft: read("src/nft.js"),
  pkg: read("package.json"),
};

const checks = [
  ["foreground AppState tx refresh", files.app, /AppState\.addEventListener\("change"[\s\S]*state === "active"[\s\S]*refresh\(\)/],
  ["background tx tracking setting", files.app, /settingsBackgroundTxTrackingEnabled/],
  ["secure storage audit on boot", files.app, /runSecureStorageAudit\(\{ repair: true \}\)/],
  ["secure storage audit UI", files.app, /Run Storage Audit/],
  ["sensitive key leakage repair", files.storage, /SENSITIVE_KEYS[\s\S]*runSecureStorageAudit[\s\S]*AsyncStorage\.removeItem/],
  ["device-only iOS keychain option", files.storage, /AFTER_FIRST_UNLOCK_THIS_DEVICE_ONLY/],
  ["transient network retry", files.api, /fetchWithRetry[\s\S]*retryableStatus/],
  ["pending tx expiry", files.tracker, /MAX_PENDING_AGE_MS[\s\S]*withStatus\(tx, "dropped"/],
  ["tx recheck throttling", files.tracker, /MIN_RECHECK_INTERVAL_MS/],
  ["schema v4 NFT migration", files.migrations, /CURRENT_SCHEMA_VERSION = 4[\s\S]*STORAGE_KEYS\.nfts/],
  ["persistent NFT portfolio", files.app, /loadJSON\(STORAGE_KEYS\.nfts[\s\S]*saveJSON\(STORAGE_KEYS\.nfts/],
  ["NFT on-chain metadata resolver", files.nft, /eth_getCode[\s\S]*0xc87b56dd[\s\S]*ownershipVerified/],
  ["NFT import is wired", files.app, /function importNftAction[\s\S]*setNfts[\s\S]*homeSubTab === "nfts"/],
  ["BTC and LTC address validation", read("src/utils.js"), /case "utxo"[\s\S]*case "litecoin"[\s\S]*tltc/],
  ["non-EVM proof script wired", files.pkg, /"test:non-evm"/],
];

let failed = 0;
for (const [name, source, pattern] of checks) {
  const ok = pattern.test(source);
  console.log(`${ok ? "PASS" : "FAIL"} ${name}`);
  if (!ok) failed += 1;
}

if (failed) {
  console.error(`\n${failed} mobile mainnet QA checks failed.`);
  process.exit(1);
}

console.log("\nAll mobile mainnet QA checks passed.");
