const CHAIN = process.env.CHAIN_URL || "https://dazzling-peace-production-3529.up.railway.app";
const WALLET = process.env.WALLET_URL || "https://enchanting-hope-production-1c63.up.railway.app";
const AGG = process.env.AGG_URL || "https://keen-enjoyment-production-0440.up.railway.app";

const report = [];
const artifacts = {
  chain: CHAIN,
  wallet: WALLET,
  aggregator: AGG,
  wallets: {},
  tokens: [],
  contracts: {},
  dex: {},
  bridge: {},
};

function ok(name, data) {
  report.push({ status: "PASS", name, data });
  console.log(`PASS ${name}`);
}

function fail(name, err) {
  const message = err?.stack || err?.message || String(err);
  report.push({ status: "FAIL", name, error: message });
  console.log(`FAIL ${name}: ${message}`);
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function readResponse(res) {
  const text = await res.text();
  try {
    return JSON.parse(text);
  } catch {
    return text;
  }
}

async function req(method, url, body, headers = {}) {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), Number(process.env.SMOKE_TIMEOUT_MS || 180000));
  try {
    const opts = { method, headers, signal: controller.signal };
    if (body !== undefined) {
      if (body instanceof FormData) {
        opts.body = body;
      } else {
        opts.body = JSON.stringify(body);
        opts.headers = { "Content-Type": "application/json", ...headers };
      }
    }
    const res = await fetch(url, opts);
    const parsed = await readResponse(res);
    if (!res.ok) {
      throw new Error(`${method} ${url} -> ${res.status}: ${JSON.stringify(parsed)}`);
    }
    return parsed;
  } finally {
    clearTimeout(timeout);
  }
}

const get = (path) => req("GET", path);
const post = (path, body) => req("POST", path, body);

async function callContract(address, caller, fn, args = []) {
  const out = await post(`${CHAIN}/contract/call`, { address, caller, fn, args });
  if (out && out.success === false) {
    throw new Error(`${fn} failed: ${JSON.stringify(out)}`);
  }
  return out;
}

async function sendContractTx(wallet, contractAddress, fn, args = [], value = "0") {
  const out = await post(`${WALLET}/wallet/contract-template`, {
    address: wallet.address,
    private_key: wallet.private_key,
    contract_address: contractAddress,
    function: fn,
    args,
    value,
    gas: 900000,
    gas_price: 0,
  });
  await sleep(2500);
  return out;
}

async function deployBuiltin(wallet, template, initArgs = []) {
  const out = await post(`${CHAIN}/contract/deploy-builtin`, {
    template,
    owner: wallet.address,
    private_key: wallet.private_key,
    gas: 900000,
    init_args: initArgs,
  });
  if (!out.address) throw new Error(`deploy-builtin ${template} returned no address: ${JSON.stringify(out)}`);
  await sleep(2500);
  return out;
}

function unwrapOutput(v) {
  if (v == null) return "";
  if (typeof v === "string") return v;
  if (typeof v.output === "string") return v.output;
  if (typeof v.Output === "string") return v.Output;
  return JSON.stringify(v);
}

async function tokenMeta(address, caller) {
  const [name, symbol, decimals, totalSupply, balance] = await Promise.all([
    callContract(address, caller, "Name").then(unwrapOutput),
    callContract(address, caller, "Symbol").then(unwrapOutput),
    callContract(address, caller, "Decimals").then(unwrapOutput),
    callContract(address, caller, "TotalSupply").then(unwrapOutput),
    callContract(address, caller, "BalanceOf", [caller]).then(unwrapOutput),
  ]);
  return { address, name, symbol, decimals, totalSupply, balance };
}

async function main() {
  await step("chain health", async () => {
    const health = await get(`${CHAIN}/health`);
    if (health.status !== "ok") throw new Error(JSON.stringify(health));
    artifacts.chainHealth = health;
    return health;
  });

  await step("aggregator network", async () => {
    const network = await get(`${AGG}/network`);
    if (!Array.isArray(network.nodes) || network.nodes.length === 0) throw new Error(JSON.stringify(network));
    artifacts.aggregatorNetwork = network;
    return network;
  });

  let walletA;
  const walletPass = `podl-${Date.now()}`;
  await step("wallet create", async () => {
    walletA = await post(`${WALLET}/wallet/new`, { password: walletPass });
    if (!walletA.address || !walletA.private_key || !walletA.mnemonic) throw new Error(JSON.stringify(walletA));
    artifacts.wallets.created = { address: walletA.address, mnemonicWords: walletA.mnemonic.split(/\s+/).length };
    return artifacts.wallets.created;
  });

  await step("wallet import mnemonic", async () => {
    const imported = await post(`${WALLET}/wallet/import/mnemonic`, { mnemonic: walletA.mnemonic, password: walletPass });
    if (!imported.address || imported.address.toLowerCase() !== walletA.address.toLowerCase()) {
      throw new Error(`import address mismatch: ${JSON.stringify(imported)}`);
    }
    artifacts.wallets.importMnemonic = { address: imported.address };
    return artifacts.wallets.importMnemonic;
  });

  await step("wallet import private key", async () => {
    const imported = await post(`${WALLET}/wallet/import/private-key`, { private_key: walletA.private_key });
    if (!imported.address || imported.address.toLowerCase() !== walletA.address.toLowerCase()) {
      throw new Error(`private key import mismatch: ${JSON.stringify(imported)}`);
    }
    artifacts.wallets.importPrivateKey = { address: imported.address };
    return artifacts.wallets.importPrivateKey;
  });

  await step("faucet claim", async () => {
    const faucet = await post(`${CHAIN}/faucet`, { address: walletA.address });
    await sleep(2500);
    const balance = await get(`${CHAIN}/balance?address=${encodeURIComponent(walletA.address)}`);
    artifacts.faucet = { faucet, balance };
    return artifacts.faucet;
  });

  await step("lqd20 built-in sanity check", async () => {
    const deploy = await deployBuiltin(walletA, "lqd20", ["Podl LQD20 Smoke", "PLQD", "1000000000000"]);
    const abi = await get(`${CHAIN}/contract/getAbi?address=${deploy.address}`);
    const names = JSON.stringify(abi);
    artifacts.contracts.lqd20 = { address: deploy.address, tx_hash: deploy.tx_hash, abi };
    if (!names.includes("Name") || !names.includes("Symbol") || !names.includes("TotalSupply")) {
      throw new Error(`lqd20 ABI missing token functions: ${names.slice(0, 500)}`);
    }
    const meta = await tokenMeta(deploy.address, walletA.address);
    return meta;
  });

  await step("deploy 5 bridge-token built-ins and verify metadata", async () => {
    for (let i = 1; i <= 5; i += 1) {
      const symbol = `T${Date.now().toString().slice(-5)}${i}`;
      const name = `Podl Smoke Token ${i}`;
      const supply = "1000000000000";
      const deploy = await deployBuiltin(walletA, "bridge_token", [name, symbol, "8", "0x0000000000000000000000000000000000000000"]);
      await sendContractTx(walletA, deploy.address, "Mint", [walletA.address, supply]);
      const meta = await tokenMeta(deploy.address, walletA.address);
      if (meta.name !== name || meta.symbol !== symbol || meta.decimals !== "8" || meta.totalSupply !== supply) {
        throw new Error(`metadata mismatch for ${deploy.address}: ${JSON.stringify(meta)}`);
      }
      artifacts.tokens.push({ ...meta, tx_hash: deploy.tx_hash });
    }
    return artifacts.tokens;
  });

  await step("custom source compile and deploy", async () => {
    const source = `package main

import bc "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/BlockchainComponent"

type ContractTemplate struct{}

var Contract = &ContractTemplate{}

func (c *ContractTemplate) Init(ctx *bc.Context) {
  ctx.Set("hello", "world")
}

func (c *ContractTemplate) Ping(ctx *bc.Context) {
  ctx.Set("output", "pong:"+ctx.CallerAddr)
}`;
    const compiled = await post(`${CHAIN}/contract/compile-plugin`, { source });
    if (!compiled.success || !compiled.binary) throw new Error(`compile failed: ${JSON.stringify(compiled).slice(0, 500)}`);
    const bytes = Uint8Array.from(Buffer.from(compiled.binary, "base64"));
    const form = new FormData();
    form.append("type", "plugin");
    form.append("owner", walletA.address);
    form.append("private_key", walletA.private_key);
    form.append("gas", "900000");
    form.append("contract_file", new Blob([bytes], { type: "application/octet-stream" }), "custom.so");
    const deployed = await req("POST", `${CHAIN}/contract/deploy`, form);
    if (!deployed.address) throw new Error(`custom deploy missing address: ${JSON.stringify(deployed)}`);
    await sleep(2500);
    const ping = await callContract(deployed.address, walletA.address, "Ping", []);
    const pingOutput = unwrapOutput(ping);
    if (!pingOutput.includes("pong:")) throw new Error(`bad ping output: ${JSON.stringify(ping)}`);
    artifacts.contracts.custom = { address: deployed.address, ping: pingOutput, compileSize: compiled.size };
    return artifacts.contracts.custom;
  });

  await step("dapp-style builtins deploy and callable ABI", async () => {
    const dao = await deployBuiltin(walletA, "dao_treasury", []);
    const nft = await deployBuiltin(walletA, "nft_collection", []);
    const daoAbi = await get(`${CHAIN}/contract/getAbi?address=${dao.address}`);
    const nftAbi = await get(`${CHAIN}/contract/getAbi?address=${nft.address}`);
    artifacts.contracts.dao = { address: dao.address, abiEntries: Array.isArray(daoAbi) ? daoAbi.length : null };
    artifacts.contracts.nft = { address: nft.address, abiEntries: Array.isArray(nftAbi) ? nftAbi.length : null };
    return { dao: artifacts.contracts.dao, nft: artifacts.contracts.nft };
  });

  await step("bridge lock request", async () => {
    const before = await get(`${CHAIN}/bridge/requests`).catch((err) => ({ error: err.message }));
    const bridge = await post(`${WALLET}/wallet/bridge/lock`, {
      from: walletA.address,
      to_bsc: "0x1111111111111111111111111111111111111111",
      chain_id: "bsc-test",
      amount: "1000",
      gas: 900000,
      gas_price: 0,
      private_key: walletA.private_key,
    });
    await sleep(4000);
    const after = await get(`${CHAIN}/bridge/requests`).catch((err) => ({ error: err.message }));
    artifacts.bridge = { before, bridge, after };
    return artifacts.bridge;
  });

  await step("DEX create pair, liquidity, quote, swap", async () => {
    let dexCurrent = await get(`${CHAIN}/dex/current`);
    let factory = dexCurrent.address;
    if (!factory) {
      const deployedFactory = await deployBuiltin(walletA, "dex_factory", []);
      factory = deployedFactory.address;
      dexCurrent = await get(`${CHAIN}/dex/current`);
      factory = dexCurrent.address || factory;
    }
    if (!factory) throw new Error(`no factory: ${JSON.stringify(dexCurrent)}`);
    const [tokenA, tokenB] = artifacts.tokens;
    const amountA = "10000000000";
    const amountB = "20000000000";
    const swapIn = "100000000";

    await sendContractTx(walletA, tokenA.address, "Approve", [factory, "999999999999"]);
    await sendContractTx(walletA, tokenB.address, "Approve", [factory, "999999999999"]);

    const pairBefore = unwrapOutput(await callContract(factory, walletA.address, "GetPair", [tokenA.address, tokenB.address]));
    if (!pairBefore) {
      await sendContractTx(walletA, factory, "CreatePair", [tokenA.address, tokenB.address]);
    }
    const pair = unwrapOutput(await callContract(factory, walletA.address, "GetPair", [tokenA.address, tokenB.address]));
    if (!pair) throw new Error("pair was not created");

    await sendContractTx(walletA, tokenA.address, "Approve", [pair, "999999999999"]);
    await sendContractTx(walletA, tokenB.address, "Approve", [pair, "999999999999"]);
    await sendContractTx(walletA, factory, "AddLiquidity", [tokenA.address, tokenB.address, amountA, amountB]);
    const pool = unwrapOutput(await callContract(factory, walletA.address, "GetPoolInfo", [tokenA.address, tokenB.address]));
    const quote = unwrapOutput(await callContract(factory, walletA.address, "GetAmountOut", [swapIn, tokenA.address, tokenB.address]));
    if (!quote || quote === "0") throw new Error(`bad quote: ${quote}, pool=${pool}`);
    await sendContractTx(walletA, tokenA.address, "Approve", [pair, "999999999999"]);
    const balBefore = unwrapOutput(await callContract(tokenB.address, walletA.address, "BalanceOf", [walletA.address]));
    const swap = await sendContractTx(walletA, factory, "SwapExactTokensForTokens", [swapIn, "1", tokenA.address, tokenB.address]);
    const balAfter = unwrapOutput(await callContract(tokenB.address, walletA.address, "BalanceOf", [walletA.address]));
    if (BigInt(balAfter) <= BigInt(balBefore)) {
      throw new Error(`swap did not increase tokenB balance: before=${balBefore} after=${balAfter}`);
    }
    artifacts.dex = { factory, pair, pool, quote, swap, tokenBBefore: balBefore, tokenBAfter: balAfter };
    return artifacts.dex;
  });

  console.log("\nJSON_REPORT_START");
  console.log(JSON.stringify({ report, artifacts }, null, 2));
  console.log("JSON_REPORT_END");
}

async function step(name, fn) {
  try {
    const data = await fn();
    ok(name, data);
  } catch (err) {
    fail(name, err);
  }
}

main().catch((err) => {
  console.error(err);
  process.exitCode = 1;
});
