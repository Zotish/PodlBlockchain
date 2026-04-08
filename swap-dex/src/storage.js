const TOKENS_KEY = "lqd.swap.tokens";

export function loadTokens() {
  try {
    const raw = localStorage.getItem(TOKENS_KEY);
    if (!raw) return [];
    const data = JSON.parse(raw);
    return Array.isArray(data) ? data : [];
  } catch {
    return [];
  }
}

export function saveTokens(tokens) {
  localStorage.setItem(TOKENS_KEY, JSON.stringify(tokens));
}

export function upsertToken(token) {
  const list = loadTokens();
  const idx = list.findIndex((t) => t.address.toLowerCase() === token.address.toLowerCase());
  if (idx >= 0) list[idx] = { ...list[idx], ...token };
  else list.push(token);
  saveTokens(list);
  return list;
}
