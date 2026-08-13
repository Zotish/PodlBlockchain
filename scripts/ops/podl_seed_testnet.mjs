#!/usr/bin/env node

import crypto from "node:crypto";
import fs from "node:fs";
import path from "node:path";
import { execFileSync } from "node:child_process";

const root = process.env.PODL_ROOT || "/opt/podl";
const envPath = path.join(root, ".env");
const secretDir = path.join(root, "secrets");
const ownerPath = process.env.PODL_TESTNET_OWNER_FILE || path.join(secretDir, "testnet-owner.json");
const chain = process.env.CHAIN_URL || "http://127.0.0.1:6500";
const aggregator = process.env.AGG_URL || "http://127.0.0.1:9000";
const dex = process.env.DEX_API_URL || "http://127.0.0.1:9100";

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

function readEnv() {
  const values = {};
  const raw = fs.readFileSync(envPath, "utf8");
  for (const line of raw.split(/\r?\n/)) {
    const match = line.match(/^([A-Za-z_][A-Za-z0-9_]*)=(.*)$/);
    if (match) values[match[1]] = match[2];
  }
  return values;
}

function upsertEnv(key, value) {
  const raw = fs.readFileSync(envPath, "utf8");
  const lines = raw.split(/\r?\n/).filter((line) => !line.startsWith(`${key}=`));
  while (lines.length && lines.at(-1) === "") lines.pop();
  lines.push(`${key}=${value}`, "");
  const temp = `${envPath}.seed.tmp`;
  const mode = fs.statSync(envPath).mode & 0o777;
  fs.writeFileSync(temp, lines.join("\n"), { mode });
  fs.renameSync(temp, envPath);
}

async function jsonRequest(url, options = {}) {
  const response = await fetch(url, options);
  const text = await response.text();
  let body = {};
  try {
    body = text ? JSON.parse(text) : {};
  } catch {
    body = { raw: text };
  }
  if (!response.ok) {
    throw new Error(`${options.method || "GET"} ${url} failed (${response.status}): ${text.slice(0, 240)}`);
  }
  return body;
}

async function waitFor(label, fn, attempts = 90, intervalMs = 1000) {
  let lastError;
  for (let i = 0; i < attempts; i += 1) {
    try {
      const result = await fn();
      if (result) return result;
    } catch (error) {
      lastError = error;
    }
    await sleep(intervalMs);
  }
  throw new Error(`${label} timed out${lastError ? `: ${lastError.message}` : ""}`);
}

function apiHeaders(env, extra = {}) {
  const headers = { "Content-Type": "application/json", ...extra };
  if (env.LQD_API_KEY) headers["X-API-Key"] = env.LQD_API_KEY;
  return headers;
}

let env = readEnv();
let adminKey = env.DEX_REGISTRY_ADMIN_KEY || "";
if (!adminKey) {
  adminKey = crypto.randomBytes(32).toString("hex");
  upsertEnv("DEX_REGISTRY_ADMIN_KEY", adminKey);
  execFileSync("docker", ["compose", "up", "-d", "--force-recreate", "dex-api"], {
    cwd: root,
    stdio: "inherit",
  });
  await waitFor("DEX API restart", async () => {
    const health = await jsonRequest(`${dex}/health`);
    return health.status === "ok";
  });
  env = readEnv();
}

fs.mkdirSync(secretDir, { recursive: true, mode: 0o700 });
let owner;
if (fs.existsSync(ownerPath)) {
  owner = JSON.parse(fs.readFileSync(ownerPath, "utf8"));
} else {
  owner = await jsonRequest(`${aggregator}/wallet/new`, {
    method: "POST",
    headers: apiHeaders(env),
    body: JSON.stringify({ password: `podl-testnet-${Date.now()}` }),
  });
  fs.writeFileSync(ownerPath, `${JSON.stringify(owner, null, 2)}\n`, { mode: 0o600 });
}

if (!owner.address || !owner.private_key) {
  throw new Error(`testnet owner file is missing address/private_key: ${ownerPath}`);
}

await jsonRequest(`${aggregator}/faucet`, {
  method: "POST",
  headers: apiHeaders(env),
  body: JSON.stringify({ address: owner.address }),
}).catch((error) => {
  if (!/already|limit|claimed/i.test(error.message)) throw error;
});

await waitFor("owner faucet balance", async () => {
  const balance = await jsonRequest(`${chain}/balance?address=${encodeURIComponent(owner.address)}`);
  const amount = String(balance.balance ?? balance.Balance ?? balance.amount ?? "0");
  return /^\d+$/.test(amount) && BigInt(amount) > 0n;
});

const startHeightResponse = await jsonRequest(`${chain}/getheight`);
const startHeight = Number(startHeightResponse.height || 0);
let current = await jsonRequest(`${chain}/dex/current`);
let factory = String(current.address || "").trim();
let txHash = "";

if (!factory) {
  const deployed = await jsonRequest(`${aggregator}/contract/deploy-builtin`, {
    method: "POST",
    headers: apiHeaders(env),
    body: JSON.stringify({
      template: "dex_factory",
      owner: owner.address,
      private_key: owner.private_key,
      gas: 1_200_000,
      init_args: [],
    }),
  });
  if (deployed.error) throw new Error(`factory deployment failed: ${deployed.error}`);
  factory = String(deployed.address || "").trim();
  txHash = String(deployed.tx_hash || "").trim();
  if (!factory) throw new Error("factory deployment returned no address");

  await waitFor("factory discovery", async () => {
    current = await jsonRequest(`${chain}/dex/current`);
    return String(current.address || "").toLowerCase() === factory.toLowerCase();
  });

  if (txHash) {
    await waitFor("factory transaction finality", async () => {
      const tx = await jsonRequest(`${chain}/tx/${encodeURIComponent(txHash)}`);
      const status = String(tx.transaction?.status || tx.status || "").toLowerCase();
      const source = String(tx.source || "").toLowerCase();
      if (status === "failed") throw new Error("factory transaction failed");
      return source === "block" || source === "recent" || status === "success" || status === "succsess";
    });
  }
}

await jsonRequest(`${dex}/config`, {
  method: "PUT",
  headers: { "Content-Type": "application/json", "X-Admin-Key": adminKey },
  body: JSON.stringify({ factory_address: factory }),
});
upsertEnv("DEX_FACTORY_ADDRESS", factory);

const endHeightResponse = await jsonRequest(`${chain}/getheight`);
const config = await jsonRequest(`${dex}/config`);
if (String(config.factory_address || "").toLowerCase() !== factory.toLowerCase()) {
  throw new Error("DEX registry factory does not match the on-chain factory");
}

process.stdout.write(`${JSON.stringify({
  status: "ok",
  owner_address: owner.address,
  owner_file: ownerPath,
  factory_address: factory,
  deployment_tx: txHash,
  start_height: startHeight,
  end_height: Number(endHeightResponse.height || 0),
})}\n`);
