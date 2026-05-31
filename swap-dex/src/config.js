function normalizeBaseUrl(value, fallback) {
  const raw = (value || fallback || "").trim();
  return raw.replace(/\/+$/, "");
}

export const NODE_URL = normalizeBaseUrl(
  process.env.REACT_APP_API_BASE || process.env.REACT_APP_NODE_URL,
  "https://api.178-105-133-94.sslip.io"
); // aggregator (or 5000 for single node)

export const WALLET_URL = normalizeBaseUrl(
  process.env.REACT_APP_WALLET_BASE || process.env.REACT_APP_WALLET_URL,
  "https://wallet.178-105-133-94.sslip.io"
); // wallet server

export const WEB_WALLET_URL = normalizeBaseUrl(
  process.env.REACT_APP_WEB_WALLET_URL,
  "http://127.0.0.1:3000"
); // optional web wallet UI

export const DEX_REGISTRY_URL = normalizeBaseUrl(
  process.env.REACT_APP_DEX_REGISTRY_API,
  "https://dex-api.178-105-133-94.sslip.io"
); // optional SQLite registry API for universal DEX config/tokens/pools

export const DEX_CONTRACT_ADDRESS =
  (process.env.REACT_APP_DEX_CONTRACT_ADDRESS || "0x51d85e8fea15bc1523e83f9fc919c11605abc4ae").trim(); // deployed Factory contract

export const DEX_ROUTER_ADDRESS =
  (process.env.REACT_APP_DEX_ROUTER_ADDRESS || "").trim(); // optional Router contract initialized with factory address

// LQD DEX Factory / Router ABI
// Factory creates pools. Router can handle swap/liquidity/LP-lock when configured.
export const DEX_ABI = [
  // ── Factory ─────────────────────────────────────────────────────────────
  { name: "CreatePair",                   inputs: ["string","string"],                        type: "function" },
  { name: "GetPair",                      inputs: ["string","string"],                        type: "function" },
  { name: "AllPairsLength",               inputs: [],                                         type: "function" },
  { name: "AllPairs",                     inputs: ["string"],                                 type: "function" },

  // ── Liquidity ────────────────────────────────────────────────────────────
  { name: "AddLiquidity",                 inputs: ["string","string","string","string"],      type: "function" },
  { name: "RemoveLiquidity",              inputs: ["string","string","string"],               type: "function" },

  // ── Swaps ────────────────────────────────────────────────────────────────
  { name: "SwapExactTokensForTokens",     inputs: ["string","string","string","string"],      type: "function" },

  // ── View helpers ─────────────────────────────────────────────────────────
  { name: "GetBestRoute",                 inputs: ["string","string"],                        type: "function" },
  { name: "GetAmountOut",                 inputs: ["string","string","string"],               type: "function" },
  { name: "GetAmountIn",                  inputs: ["string","string","string"],               type: "function" },
  { name: "GetPoolInfo",                  inputs: ["string","string"],                        type: "function" },
  { name: "GetLPBalance",                 inputs: ["string","string","string"],               type: "function" },
  { name: "GetLPValue",                   inputs: ["string","string","string"],               type: "function" },

  // ── Proof of Dynamic Liquidity — validator LP locking ────────────────────
  { name: "LockLPForValidation",          inputs: ["string","string","string","string"],      type: "function" },
  { name: "UnlockValidatorLP",            inputs: ["string","string"],                        type: "function" },
  { name: "GetValidatorLP",               inputs: ["string","string","string"],               type: "function" },
];
