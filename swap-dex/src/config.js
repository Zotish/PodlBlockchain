export const NODE_URL    = "http://127.0.0.1:9000"; // aggregator (or 5000 for single node)
export const WALLET_URL  = "http://127.0.0.1:8080"; // wallet server
export const WEB_WALLET_URL = "http://127.0.0.1:3000"; // optional web wallet UI

export const DEX_CONTRACT_ADDRESS = ""; // set after fresh deployment

// Full Uniswap v2-style ABI for the LQD DEX contract
export const DEX_ABI = [
  // ── Initialization ──────────────────────────────────────────────────────
  { name: "Init",               inputs: ["string", "string"], outputs: [],        type: "function" },

  // ── Liquidity ───────────────────────────────────────────────────────────
  { name: "AddLiquidity",       inputs: ["string", "string"], outputs: [],        type: "function" },
  { name: "RemoveLiquidity",    inputs: ["string"],           outputs: [],        type: "function" },

  // ── Swaps ───────────────────────────────────────────────────────────────
  { name: "SwapAtoB",           inputs: ["string"],           outputs: [],        type: "function" },
  { name: "SwapBtoA",           inputs: ["string"],           outputs: [],        type: "function" },

  // ── View helpers ────────────────────────────────────────────────────────
  { name: "GetAmountOut",       inputs: ["string"],           outputs: [],        type: "function" },
  { name: "GetAmountIn",        inputs: ["string"],           outputs: [],        type: "function" },
  { name: "GetPoolInfo",        inputs: [],                   outputs: [],        type: "function" },
  { name: "GetLPBalance",       inputs: ["string"],           outputs: [],        type: "function" },
  { name: "GetLPValue",         inputs: ["string"],           outputs: [],        type: "function" },

  // ── Proof of Dynamic Liquidity — validator LP locking ───────────────────
  { name: "LockLPForValidation", inputs: ["string", "string"], outputs: [],       type: "function" },
  { name: "UnlockValidatorLP",   inputs: [],                   outputs: [],       type: "function" },
  { name: "GetValidatorLP",      inputs: ["string"],           outputs: [],       type: "function" },
];
