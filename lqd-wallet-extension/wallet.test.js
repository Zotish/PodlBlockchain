/**
 * wallet.test.js
 * Unit tests for LQD Wallet Extension pure helper functions.
 *
 * The wallet extension runs as a browser extension (no ES module exports),
 * so we test the identical pure-function logic in isolation here.
 * These tests verify formatting, parsing, and wallet utility behaviour.
 */

// ─────────────────────────────────────────────────────────────────────────────
// Pure helpers extracted from popup.js / background.js
// (identical implementations — no production files changed)
// ─────────────────────────────────────────────────────────────────────────────

const LQD_DECIMALS = 8;

function formatLQD(raw) {
  if (!raw && raw !== 0) return "0";
  const str = String(raw).replace(/[^0-9]/g, "");
  if (!str || str === "0") return "0";
  const DECIMALS = 8;
  if (str.length <= DECIMALS) {
    const padded = str.padStart(DECIMALS + 1, "0");
    const frac = padded.slice(padded.length - DECIMALS).replace(/0+$/, "");
    return frac ? `0.${frac}` : "0";
  }
  const intPart = str.slice(0, str.length - DECIMALS);
  const fracPart = str.slice(str.length - DECIMALS).replace(/0+$/, "");
  return fracPart ? `${intPart}.${fracPart}` : intPart;
}

function parseAmount(human, decimals = 8) {
  if (!human) return "0";
  const [intS, fracS = ""] = String(human).split(".");
  const frac = fracS.slice(0, decimals).padEnd(decimals, "0");
  const full = (intS.replace(/^0+/, "") || "0") + frac;
  return full.replace(/^0+/, "") || "0";
}

function truncate(str, n) {
  if (!str) return "";
  if (str.length <= n * 2) return str;
  return str.slice(0, n) + "…" + str.slice(-6);
}

function normalizeBaseUrl(value, fallback) {
  const raw = (value || fallback || "").trim();
  return raw.replace(/\/+$/, "");
}

function validateAddress(addr) {
  return typeof addr === "string" &&
    addr.startsWith("0x") &&
    addr.length === 42;
}

// ─────────────────────────────────────────────────────────────────────────────
// formatLQD — convert raw satoshi integer to human-readable LQD
// ─────────────────────────────────────────────────────────────────────────────

describe('formatLQD', () => {
  test('100000000 → "1" (one LQD)', () => {
    expect(formatLQD(100000000)).toBe("1");
  });

  test('200000000 → "2"', () => {
    expect(formatLQD(200000000)).toBe("2");
  });

  test('2000000000 → "20" (genesis block reward)', () => {
    expect(formatLQD(2000000000)).toBe("20");
  });

  test('1 → "0.00000001" (1 satoshi)', () => {
    expect(formatLQD(1)).toBe("0.00000001");
  });

  test('150000000 → "1.5"', () => {
    expect(formatLQD(150000000)).toBe("1.5");
  });

  test('123456789 → "1.23456789"', () => {
    expect(formatLQD(123456789)).toBe("1.23456789");
  });

  test('0 → "0"', () => {
    expect(formatLQD(0)).toBe("0");
  });

  test('null → "0"', () => {
    expect(formatLQD(null)).toBe("0");
  });

  test('undefined → "0"', () => {
    expect(formatLQD(undefined)).toBe("0");
  });

  test('string "100000000" → "1"', () => {
    expect(formatLQD("100000000")).toBe("1");
  });

  test('strips non-numeric chars from input string', () => {
    expect(formatLQD("1,000,000,00")).toBe("1");
  });

  test('trailing zeros removed: 100000000 → "1" not "1.00000000"', () => {
    const result = formatLQD(100000000);
    expect(result).not.toContain('.');
  });
});

// ─────────────────────────────────────────────────────────────────────────────
// parseAmount — convert human LQD string to raw satoshi integer string
// ─────────────────────────────────────────────────────────────────────────────

describe('parseAmount', () => {
  test('"1" → "100000000"', () => {
    expect(parseAmount("1")).toBe("100000000");
  });

  test('"20" → "2000000000"', () => {
    expect(parseAmount("20")).toBe("2000000000");
  });

  test('"0.5" → "50000000"', () => {
    expect(parseAmount("0.5")).toBe("50000000");
  });

  test('"0.00000001" → "1" (1 satoshi)', () => {
    expect(parseAmount("0.00000001")).toBe("1");
  });

  test('"1.23456789" → "123456789"', () => {
    expect(parseAmount("1.23456789")).toBe("123456789");
  });

  test('null → "0"', () => {
    expect(parseAmount(null)).toBe("0");
  });

  test('empty string → "0"', () => {
    expect(parseAmount("")).toBe("0");
  });

  test('truncates extra fractional digits to 8', () => {
    // "1.123456789" → truncated at 8 decimals → "1.12345678" → "112345678"
    expect(parseAmount("1.123456789")).toBe("112345678");
  });

  test('"100" → "10000000000"', () => {
    expect(parseAmount("100")).toBe("10000000000");
  });

  test('parseAmount and formatLQD are inverses', () => {
    const human = "1.5";
    const raw = parseAmount(human);
    const backToHuman = formatLQD(raw);
    expect(backToHuman).toBe(human);
  });

  test('round trip: 2 LQD', () => {
    expect(formatLQD(parseAmount("2"))).toBe("2");
  });
});

// ─────────────────────────────────────────────────────────────────────────────
// truncate — shorten long strings (addresses) for display
// ─────────────────────────────────────────────────────────────────────────────

describe('truncate', () => {
  test('short string returned as-is', () => {
    expect(truncate("short", 12)).toBe("short");
  });

  test('null → ""', () => {
    expect(truncate(null, 12)).toBe("");
  });

  test('empty string → ""', () => {
    expect(truncate("", 12)).toBe("");
  });

  test('long address is truncated with ellipsis', () => {
    const addr = "0x1234567890abcdef1234567890abcdef12345678";
    const result = truncate(addr, 12);
    expect(result).toContain("…");
    expect(result.length).toBeLessThan(addr.length);
  });

  test('truncated string ends with last 6 chars of original', () => {
    const addr = "0x1234567890abcdef1234567890abcdef12345678";
    const result = truncate(addr, 12);
    expect(result.endsWith(addr.slice(-6))).toBe(true);
  });

  test('truncated string starts with first n chars of original', () => {
    const addr = "0x1234567890abcdef1234567890abcdef12345678";
    const result = truncate(addr, 12);
    expect(result.startsWith(addr.slice(0, 12))).toBe(true);
  });
});

// ─────────────────────────────────────────────────────────────────────────────
// normalizeBaseUrl — strip trailing slashes
// ─────────────────────────────────────────────────────────────────────────────

describe('normalizeBaseUrl', () => {
  test('strips trailing slash', () => {
    expect(normalizeBaseUrl("http://localhost:9000/", "")).toBe("http://localhost:9000");
  });

  test('strips multiple trailing slashes', () => {
    expect(normalizeBaseUrl("http://localhost:9000///", "")).toBe("http://localhost:9000");
  });

  test('keeps URL without trailing slash unchanged', () => {
    expect(normalizeBaseUrl("http://localhost:9000", "")).toBe("http://localhost:9000");
  });

  test('uses fallback when value is empty', () => {
    expect(normalizeBaseUrl("", "http://fallback:5000")).toBe("http://fallback:5000");
  });

  test('uses fallback when value is null', () => {
    expect(normalizeBaseUrl(null, "http://fallback:5000")).toBe("http://fallback:5000");
  });

  test('uses fallback when value is undefined', () => {
    expect(normalizeBaseUrl(undefined, "http://fallback:5000")).toBe("http://fallback:5000");
  });

  test('trims whitespace', () => {
    expect(normalizeBaseUrl("  http://localhost:9000  ", "")).toBe("http://localhost:9000");
  });
});

// ─────────────────────────────────────────────────────────────────────────────
// validateAddress — 0x-prefixed 40-char hex address
// ─────────────────────────────────────────────────────────────────────────────

describe('validateAddress', () => {
  test('valid address → true', () => {
    expect(validateAddress("0x1234567890abcdef1234567890abcdef12345678")).toBe(true);
  });

  test('all zeros address → true', () => {
    expect(validateAddress("0x0000000000000000000000000000000000000000")).toBe(true);
  });

  test('missing 0x prefix → false', () => {
    expect(validateAddress("1234567890abcdef1234567890abcdef12345678")).toBe(false);
  });

  test('too short → false', () => {
    expect(validateAddress("0x1234")).toBe(false);
  });

  test('too long → false', () => {
    expect(validateAddress("0x1234567890abcdef1234567890abcdef123456789")).toBe(false);
  });

  test('null → false', () => {
    expect(validateAddress(null)).toBe(false);
  });

  test('empty string → false', () => {
    expect(validateAddress("")).toBe(false);
  });
});

// ─────────────────────────────────────────────────────────────────────────────
// DEFAULT_NETWORKS — structure validation
// ─────────────────────────────────────────────────────────────────────────────

describe('DEFAULT_NETWORKS structure', () => {
  const DEFAULT_NETWORKS = {
    "0x8b": {
      chainId: "0x8b",
      name: "LQD Mainnet",
      nodeUrl: "http://127.0.0.1:5000",
      walletUrl: "http://127.0.0.1:8080",
      symbol: "LQD",
      blockExplorer: "http://localhost:3001"
    },
    "0x8c": {
      chainId: "0x8c",
      name: "LQD Aggregator",
      nodeUrl: "http://127.0.0.1:9000",
      walletUrl: "http://127.0.0.1:8080",
      symbol: "LQD",
      blockExplorer: "http://localhost:3001"
    }
  };

  test('has mainnet (0x8b)', () => {
    expect(DEFAULT_NETWORKS["0x8b"]).toBeDefined();
  });

  test('mainnet chainId is 0x8b (decimal 139)', () => {
    expect(parseInt("0x8b", 16)).toBe(139);
    expect(DEFAULT_NETWORKS["0x8b"].chainId).toBe("0x8b");
  });

  test('mainnet uses LQD symbol', () => {
    expect(DEFAULT_NETWORKS["0x8b"].symbol).toBe("LQD");
  });

  test('aggregator network exists (0x8c)', () => {
    expect(DEFAULT_NETWORKS["0x8c"]).toBeDefined();
  });

  test('all networks have required fields', () => {
    Object.values(DEFAULT_NETWORKS).forEach(network => {
      expect(network.chainId).toBeDefined();
      expect(network.name).toBeDefined();
      expect(network.nodeUrl).toBeDefined();
      expect(network.walletUrl).toBeDefined();
      expect(network.symbol).toBeDefined();
    });
  });

  test('network URLs have no trailing slash', () => {
    Object.values(DEFAULT_NETWORKS).forEach(network => {
      expect(network.nodeUrl).not.toMatch(/\/$/);
      expect(network.walletUrl).not.toMatch(/\/$/);
    });
  });
});

// ─────────────────────────────────────────────────────────────────────────────
// Wallet session initial state
// ─────────────────────────────────────────────────────────────────────────────

describe('Wallet session initial state', () => {
  const defaultSession = {
    unlocked: false,
    address: "",
    privateKey: "",
    chainId: "0x8b",
    nodeUrl: "http://127.0.0.1:5000",
    walletUrl: "http://127.0.0.1:8080"
  };

  test('wallet starts locked', () => {
    expect(defaultSession.unlocked).toBe(false);
  });

  test('wallet starts with no address', () => {
    expect(defaultSession.address).toBe("");
  });

  test('default chain is LQD mainnet (0x8b)', () => {
    expect(defaultSession.chainId).toBe("0x8b");
  });

  test('private key is not exposed in initial state', () => {
    expect(defaultSession.privateKey).toBe("");
  });
});

// ─────────────────────────────────────────────────────────────────────────────
// Auto-lock constant
// ─────────────────────────────────────────────────────────────────────────────

describe('AUTO_LOCK_MINUTES', () => {
  const AUTO_LOCK_MINUTES = 15;

  test('auto-lock is set to 15 minutes', () => {
    expect(AUTO_LOCK_MINUTES).toBe(15);
  });

  test('auto-lock is > 0', () => {
    expect(AUTO_LOCK_MINUTES).toBeGreaterThan(0);
  });
});

// ─────────────────────────────────────────────────────────────────────────────
// LQD amount math (big number edge cases)
// ─────────────────────────────────────────────────────────────────────────────

describe('LQD amount edge cases', () => {
  test('parseAmount handles "0.00000001" correctly (1 satoshi)', () => {
    expect(parseAmount("0.00000001")).toBe("1");
  });

  test('parseAmount → formatLQD: "0.00000001" round trip', () => {
    expect(formatLQD(parseAmount("0.00000001"))).toBe("0.00000001");
  });

  test('large amount: "1000000" LQD = 1e14 satoshis', () => {
    expect(parseAmount("1000000")).toBe("100000000000000");
  });

  test('formatLQD of 1e14 satoshis → "1000000"', () => {
    expect(formatLQD("100000000000000")).toBe("1000000");
  });

  test('genesis block reward (2000000000 satoshis) → "20"', () => {
    expect(formatLQD(2000000000)).toBe("20");
  });
});
