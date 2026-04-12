export const NODE_URL    = "http://127.0.0.1:9000"; // aggregator (or 5000 for single node)
export const WALLET_URL  = "http://127.0.0.1:8080"; // wallet server
export const WEB_WALLET_URL = "http://127.0.0.1:3000"; // optional web wallet UI

export const DEX_CONTRACT_ADDRESS = ""; // set after deploying fresh Factory contract

// LQD DEX Factory + Router ABI
// Single contract manages ALL pairs — Uniswap v2 Factory + Router combined
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
