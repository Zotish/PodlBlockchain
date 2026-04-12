const TOKENS_KEY = "lqd.swap.tokens";

// Native LQD coin — always present, cannot be removed
export const NATIVE_LQD = {
  address: "lqd",
  name: "LQD Coin",
  symbol: "LQD",
  decimals: "8",
  native: true,
};

export function loadTokens() {
  try {
    const raw = localStorage.getItem(TOKENS_KEY);
    const stored = raw ? JSON.parse(raw) : [];
    const list = Array.isArray(stored) ? stored : [];
    // Always ensure native LQD is first
    const withoutNative = list.filter(t => t.address !== "lqd");
    return [NATIVE_LQD, ...withoutNative];
  } catch {
    return [NATIVE_LQD];
  }
}

export function saveTokens(tokens) {
  localStorage.setItem(TOKENS_KEY, JSON.stringify(tokens));
}

export function upsertToken(token) {
  // Never overwrite native LQD entry
  if (token.address === "lqd") return loadTokens();
  const raw = (() => { try { const r = localStorage.getItem(TOKENS_KEY); return r ? JSON.parse(r) : []; } catch { return []; } })();
  const list = Array.isArray(raw) ? raw.filter(t => t.address !== "lqd") : [];
  const idx = list.findIndex((t) => t.address.toLowerCase() === token.address.toLowerCase());
  if (idx >= 0) list[idx] = { ...list[idx], ...token };
  else list.push(token);
  saveTokens(list);
  return [NATIVE_LQD, ...list];
}
