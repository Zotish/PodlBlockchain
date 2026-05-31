import express from "express";
import cors from "cors";
import Database from "better-sqlite3";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const PORT = Number(process.env.PORT || 9100);
const DB_PATH = process.env.DEX_REGISTRY_DB || path.join(__dirname, "data", "dex-registry.sqlite");
const ADMIN_KEY = process.env.DEX_REGISTRY_ADMIN_KEY || "";
const DEFAULT_FACTORY = (process.env.DEX_FACTORY_ADDRESS || "0x51d85e8fea15bc1523e83f9fc919c11605abc4ae").trim();
const DEFAULT_ROUTER = (process.env.DEX_ROUTER_ADDRESS || "").trim();
const DEFAULT_NODE_URL = (process.env.LQD_NODE_URL || "https://api.178-105-133-94.sslip.io").replace(/\/+$/, "");
const DEFAULT_WALLET_URL = (process.env.LQD_WALLET_URL || "https://wallet.178-105-133-94.sslip.io").replace(/\/+$/, "");
const ALLOWED_ORIGINS = (process.env.ALLOWED_ORIGINS || "*")
  .split(",")
  .map((origin) => origin.trim())
  .filter(Boolean);

const app = express();
const db = new Database(DB_PATH);

app.use(express.json({ limit: "256kb" }));
app.use(cors({
  origin(origin, cb) {
    if (!origin || ALLOWED_ORIGINS.includes("*") || ALLOWED_ORIGINS.includes(origin)) return cb(null, true);
    return cb(new Error(`Origin not allowed: ${origin}`));
  }
}));

initDB();
seedDefaults();

app.get("/health", (_req, res) => {
  res.json({ status: "ok", db: "sqlite", timestamp: Math.floor(Date.now() / 1000) });
});

app.get("/config", (_req, res) => {
  const rows = db.prepare("SELECT key, value FROM dex_config").all();
  const config = Object.fromEntries(rows.map((row) => [row.key, row.value]));
  res.json(config);
});

app.put("/config", requireAdmin, (req, res) => {
  const allowed = ["factory_address", "router_address", "node_url", "wallet_url"];
  const upsert = db.prepare(`
    INSERT INTO dex_config (key, value, updated_at)
    VALUES (?, ?, ?)
    ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
  `);
  const now = Math.floor(Date.now() / 1000);
  for (const key of allowed) {
    if (req.body[key] === undefined) continue;
    upsert.run(key, String(req.body[key] || "").trim(), now);
  }
  res.json({ status: "ok" });
});

app.get("/tokens", (_req, res) => {
  const tokens = db.prepare(`
    SELECT address, name, symbol, decimals, logo_url, verified, native, sort_order, created_at, updated_at
    FROM tokens
    WHERE enabled = 1
    ORDER BY native DESC, sort_order ASC, symbol COLLATE NOCASE ASC
  `).all();
  res.json(tokens.map(mapToken));
});

app.post("/tokens", requireAdmin, (req, res) => {
  const token = normalizeToken(req.body);
  if (!token.address || !token.symbol) return res.status(400).json({ error: "address and symbol are required" });
  const now = Math.floor(Date.now() / 1000);
  db.prepare(`
    INSERT INTO tokens (address, name, symbol, decimals, logo_url, verified, native, enabled, sort_order, created_at, updated_at)
    VALUES (@address, @name, @symbol, @decimals, @logo_url, @verified, @native, 1, @sort_order, @now, @now)
    ON CONFLICT(address) DO UPDATE SET
      name = excluded.name,
      symbol = excluded.symbol,
      decimals = excluded.decimals,
      logo_url = excluded.logo_url,
      verified = excluded.verified,
      native = excluded.native,
      enabled = 1,
      sort_order = excluded.sort_order,
      updated_at = excluded.updated_at
  `).run({ ...token, now });
  res.json({ status: "ok", token: mapToken(token) });
});

app.delete("/tokens/:address", requireAdmin, (req, res) => {
  db.prepare("UPDATE tokens SET enabled = 0, updated_at = ? WHERE lower(address) = lower(?)")
    .run(Math.floor(Date.now() / 1000), req.params.address);
  res.json({ status: "ok" });
});

app.get("/pools", (_req, res) => {
  const pools = db.prepare(`
    SELECT address, token_a, token_b, pair_key, tier, weight, approved, sort_order, created_at, updated_at
    FROM pools
    WHERE enabled = 1
    ORDER BY tier ASC, sort_order ASC, pair_key COLLATE NOCASE ASC
  `).all();
  res.json(pools.map(mapPool));
});

app.post("/pools", requireAdmin, (req, res) => {
  const pool = normalizePool(req.body);
  if (!pool.address || !pool.token_a || !pool.token_b) {
    return res.status(400).json({ error: "address, token_a, and token_b are required" });
  }
  const now = Math.floor(Date.now() / 1000);
  db.prepare(`
    INSERT INTO pools (address, token_a, token_b, pair_key, tier, weight, approved, enabled, sort_order, created_at, updated_at)
    VALUES (@address, @token_a, @token_b, @pair_key, @tier, @weight, @approved, 1, @sort_order, @now, @now)
    ON CONFLICT(address) DO UPDATE SET
      token_a = excluded.token_a,
      token_b = excluded.token_b,
      pair_key = excluded.pair_key,
      tier = excluded.tier,
      weight = excluded.weight,
      approved = excluded.approved,
      enabled = 1,
      sort_order = excluded.sort_order,
      updated_at = excluded.updated_at
  `).run({ ...pool, now });
  res.json({ status: "ok", pool: mapPool(pool) });
});

app.delete("/pools/:address", requireAdmin, (req, res) => {
  db.prepare("UPDATE pools SET enabled = 0, updated_at = ? WHERE lower(address) = lower(?)")
    .run(Math.floor(Date.now() / 1000), req.params.address);
  res.json({ status: "ok" });
});

app.use((err, _req, res, _next) => {
  res.status(500).json({ error: err.message || "registry error" });
});

app.listen(PORT, () => {
  console.log(`LQD DEX registry listening on :${PORT}`);
});

function requireAdmin(req, res, next) {
  if (!ADMIN_KEY) return res.status(500).json({ error: "DEX_REGISTRY_ADMIN_KEY is not configured" });
  if (req.get("X-Admin-Key") !== ADMIN_KEY) return res.status(401).json({ error: "unauthorized" });
  next();
}

function initDB() {
  db.exec(`
    PRAGMA journal_mode = WAL;
    CREATE TABLE IF NOT EXISTS dex_config (
      key TEXT PRIMARY KEY,
      value TEXT NOT NULL DEFAULT '',
      updated_at INTEGER NOT NULL
    );
    CREATE TABLE IF NOT EXISTS tokens (
      address TEXT PRIMARY KEY,
      name TEXT NOT NULL,
      symbol TEXT NOT NULL,
      decimals INTEGER NOT NULL DEFAULT 8,
      logo_url TEXT NOT NULL DEFAULT '',
      verified INTEGER NOT NULL DEFAULT 1,
      native INTEGER NOT NULL DEFAULT 0,
      enabled INTEGER NOT NULL DEFAULT 1,
      sort_order INTEGER NOT NULL DEFAULT 1000,
      created_at INTEGER NOT NULL,
      updated_at INTEGER NOT NULL
    );
    CREATE TABLE IF NOT EXISTS pools (
      address TEXT PRIMARY KEY,
      token_a TEXT NOT NULL,
      token_b TEXT NOT NULL,
      pair_key TEXT NOT NULL,
      tier INTEGER NOT NULL DEFAULT 3,
      weight REAL NOT NULL DEFAULT 0.35,
      approved INTEGER NOT NULL DEFAULT 1,
      enabled INTEGER NOT NULL DEFAULT 1,
      sort_order INTEGER NOT NULL DEFAULT 1000,
      created_at INTEGER NOT NULL,
      updated_at INTEGER NOT NULL
    );
  `);
}

function seedDefaults() {
  const now = Math.floor(Date.now() / 1000);
  const cfg = db.prepare(`
    INSERT INTO dex_config (key, value, updated_at)
    VALUES (?, ?, ?)
    ON CONFLICT(key) DO NOTHING
  `);
  cfg.run("factory_address", DEFAULT_FACTORY, now);
  cfg.run("router_address", DEFAULT_ROUTER, now);
  cfg.run("node_url", DEFAULT_NODE_URL, now);
  cfg.run("wallet_url", DEFAULT_WALLET_URL, now);

  db.prepare(`
    INSERT INTO tokens (address, name, symbol, decimals, verified, native, enabled, sort_order, created_at, updated_at)
    VALUES ('lqd', 'LQD Coin', 'LQD', 8, 1, 1, 1, 0, ?, ?)
    ON CONFLICT(address) DO NOTHING
  `).run(now, now);
}

function normalizeToken(input = {}) {
  const address = String(input.address || "").trim().toLowerCase();
  return {
    address,
    name: String(input.name || input.symbol || "Token").trim(),
    symbol: String(input.symbol || "").trim().toUpperCase(),
    decimals: Math.max(0, Number.parseInt(input.decimals ?? 8, 10) || 8),
    logo_url: String(input.logo_url || input.logoUrl || "").trim(),
    verified: boolInt(input.verified ?? true),
    native: boolInt(input.native ?? address === "lqd"),
    sort_order: Number.parseInt(input.sort_order ?? input.sortOrder ?? 1000, 10) || 1000
  };
}

function normalizePool(input = {}) {
  const tokenA = String(input.token_a || input.tokenA || "").trim().toLowerCase();
  const tokenB = String(input.token_b || input.tokenB || "").trim().toLowerCase();
  const pairKey = String(input.pair_key || input.pairKey || makePairKey(tokenA, tokenB)).trim().toUpperCase();
  return {
    address: String(input.address || "").trim().toLowerCase(),
    token_a: tokenA,
    token_b: tokenB,
    pair_key: pairKey,
    tier: Number.parseInt(input.tier ?? 3, 10) || 3,
    weight: Number.parseFloat(input.weight ?? 0.35) || 0.35,
    approved: boolInt(input.approved ?? true),
    sort_order: Number.parseInt(input.sort_order ?? input.sortOrder ?? 1000, 10) || 1000
  };
}

function makePairKey(tokenA, tokenB) {
  const a = tokenA === "lqd" ? "LQD" : tokenA;
  const b = tokenB === "lqd" ? "LQD" : tokenB;
  return `${a}/${b}`;
}

function boolInt(value) {
  return value === true || value === 1 || value === "1" || value === "true" ? 1 : 0;
}

function mapToken(row) {
  return {
    address: row.address,
    name: row.name,
    symbol: row.symbol,
    decimals: String(row.decimals),
    logoUrl: row.logo_url || "",
    verified: !!row.verified,
    native: !!row.native,
    sortOrder: row.sort_order
  };
}

function mapPool(row) {
  return {
    address: row.address,
    tokenA: row.token_a,
    tokenB: row.token_b,
    pairKey: row.pair_key,
    tier: row.tier,
    weight: row.weight,
    approved: !!row.approved,
    sortOrder: row.sort_order
  };
}
