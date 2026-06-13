const fs = require("fs");
const path = require("path");

const root = path.resolve(__dirname, "..");
const app = fs.readFileSync(path.join(root, "App.js"), "utf8");
const crypto = fs.readFileSync(path.join(root, "src", "crypto.js"), "utf8");
const ton = fs.readFileSync(path.join(root, "src", "ton.js"), "utf8");
const starknet = fs.readFileSync(path.join(root, "src", "starknet.js"), "utf8");

const checks = [
  ["cosmos native signing", app, /family === "cosmos"[\s\S]*signCosmosTx/],
  ["injective native signing", app, /family === "cosmos" \|\| family === "cosmos-testnet" \|\| family === "injective"[\s\S]*signCosmosTx/],
  ["solana native signing", app, /family === "solana"[\s\S]*signSolanaTransfer/],
  ["near native signing", app, /family === "near"[\s\S]*signNearTransfer/],
  ["aptos native signing", app, /family === "aptos"[\s\S]*signAptosEntry/],
  ["sui native signing", app, /family === "sui"[\s\S]*signSuiTx/],
  ["ton native BOC signing", app, /family === "ton"[\s\S]*buildTonTransferBoc/],
  ["starknet native signing", app, /family === "starknet"[\s\S]*broadcastStarknetTransfer/],
  ["solana token signing", app, /family === "solana"[\s\S]*signSolanaTokenTransfer/],
  ["near token function call signing", app, /family === "near"[\s\S]*signNearFunctionCall/],
  ["aptos token signing", app, /family === "aptos"[\s\S]*signAptosEntry/],
  ["sui token signing", app, /family === "sui"[\s\S]*signSuiTx/],
  ["ton jetton BOC signing", app, /family === "ton"[\s\S]*buildJettonTransferBoc/],
  ["starknet token signing", app, /family === "starknet"[\s\S]*broadcastStarknetTokenTransfer/],
  ["crypto solana transfer builder", crypto, /export function signSolanaTransfer/],
  ["crypto near transfer builder", crypto, /export function signNearTransfer/],
  ["crypto aptos entry signer", crypto, /export function signAptosEntry/],
  ["crypto sui signer", crypto, /export function signSuiTx/],
  ["TON BOC builder", ton, /export function buildTonTransferBoc/],
  ["Starknet broadcaster", starknet, /export async function broadcastStarknetTransfer/],
  ["WebView origin gate", app, /isAllowedBrowserOrigin/],
  ["MetaMask-style dApp tx approval", app, /approveDappTransaction/],
  ["MetaMask-style dApp sign approval", app, /approveDappSign/],
  ["non-EVM dApp transaction approval", app, /approveDappNonEvmTransaction/],
  ["Solana browser provider", app, /window\.solana/],
  ["Phantom Solana alias", app, /window\.phantom\.solana = window\.solana/],
  ["Aptos browser provider", app, /window\.aptos/],
  ["Petra Aptos alias", app, /window\.petra = window\.aptos/],
  ["Sui browser provider", app, /window\.suiWallet/],
  ["Sui wallet alias", app, /window\.sui = window\.suiWallet/],
  ["TON browser provider", app, /window\.ton/],
  ["Tonkeeper alias", app, /window\.tonkeeper = window\.ton/],
  ["Starknet browser provider", app, /window\.starknet/],
  ["ArgentX Starknet alias", app, /window\.starknet_argentX = window\.starknet/],
  ["Keplr browser provider", app, /window\.keplr/],
  ["Keplr amino signature shape", crypto, /export function signCosmosAminoDoc/],
  ["No direct dApp signing restriction", app, (source) => !/Direct dApp signing is restricted/.test(source)],
  ["persistent tx tracking", app, /addTrackedTx/],
];

let failed = 0;
for (const [name, source, pattern] of checks) {
  const ok = typeof pattern === "function" ? pattern(source) : pattern.test(source);
  console.log(`${ok ? "PASS" : "FAIL"} ${name}`);
  if (!ok) failed += 1;
}

if (failed) {
  console.error(`\n${failed} non-EVM mock coverage checks failed.`);
  process.exit(1);
}

console.log("\nAll non-EVM send/signing mock coverage checks passed.");
