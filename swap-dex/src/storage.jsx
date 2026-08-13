const TOKENS_KEY = "lqd.swap.tokens";
let sessionTokens = [];

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
    localStorage.removeItem(TOKENS_KEY);
    const withoutNative = sessionTokens.filter(t => t.address !== "lqd");
    return [NATIVE_LQD, ...withoutNative];
  } catch {
    return [NATIVE_LQD, ...sessionTokens.filter(t => t.address !== "lqd")];
  }
}

export function saveTokens(tokens) {
  sessionTokens = (Array.isArray(tokens) ? tokens : []).filter(t => t?.address && t.address !== "lqd" && !t.registry);
  try { localStorage.removeItem(TOKENS_KEY); } catch {}
}

export function mergeTokens(...groups) {
  const byAddress = new Map();
  for (const group of groups) {
    for (const token of Array.isArray(group) ? group : []) {
      if (!token?.address) continue;
      const key = String(token.address).toLowerCase();
      byAddress.set(key, { ...(byAddress.get(key) || {}), ...token, address: key === "lqd" ? "lqd" : token.address });
    }
  }
  byAddress.delete("lqd");
  return [NATIVE_LQD, ...Array.from(byAddress.values())];
}

export function upsertToken(token) {
  // Never overwrite native LQD entry
  if (token.address === "lqd") return loadTokens();
  const list = sessionTokens.filter(t => t.address !== "lqd" && !t.registry);
  const idx = list.findIndex((t) => t.address.toLowerCase() === token.address.toLowerCase());
  if (idx >= 0) list[idx] = { ...list[idx], ...token };
  else list.push(token);
  saveTokens(list);
  return [NATIVE_LQD, ...list];
}
