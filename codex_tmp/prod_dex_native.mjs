const CHAIN = "https://dazzling-peace-production-3529.up.railway.app";
const WALLET = "https://enchanting-hope-production-1c63.up.railway.app";

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

async function req(method, url, body) {
  const opts = { method };
  if (body !== undefined) {
    opts.body = JSON.stringify(body);
    opts.headers = { "Content-Type": "application/json" };
  }
  const res = await fetch(url, opts);
  const parsed = await readResponse(res);
  if (!res.ok) throw new Error(`${method} ${url} -> ${res.status}: ${JSON.stringify(parsed)}`);
  return parsed;
}

const get = (url) => req("GET", url);
const post = (url, body) => req("POST", url, body);

function output(v) {
  if (typeof v === "string") return v;
  if (v?.output != null) return String(v.output);
  if (v?.Output != null) return String(v.Output);
  return "";
}

async function call(address, caller, fn, args = []) {
  return post(`${CHAIN}/contract/call`, { address, caller, fn, args });
}

async function tx(wallet, contract, fn, args = [], value = "0") {
  const res = await post(`${WALLET}/wallet/contract-template`, {
    address: wallet.address,
    private_key: wallet.private_key,
    contract_address: contract,
    function: fn,
    args,
    value,
    gas: 900000,
    gas_price: 0,
  });
  await sleep(2500);
  return res;
}

async function deployBuiltin(wallet, template, initArgs) {
  const res = await post(`${CHAIN}/contract/deploy-builtin`, {
    template,
    owner: wallet.address,
    private_key: wallet.private_key,
    gas: 900000,
    init_args: initArgs,
  });
  await sleep(2500);
  if (!res.address) throw new Error(`no address: ${JSON.stringify(res)}`);
  return res;
}

async function deployWorkingToken(wallet) {
  for (let i = 0; i < 6; i += 1) {
    const name = `DEX Native Smoke ${Date.now()} ${i}`;
    const symbol = `DN${String(Date.now()).slice(-4)}${i}`;
    const deployed = await deployBuiltin(wallet, "bridge_token", [name, symbol, "8", "0x0000000000000000000000000000000000000000"]);
    try {
      await tx(wallet, deployed.address, "Mint", [wallet.address, "1000000000000"]);
      const meta = {
        name: output(await call(deployed.address, wallet.address, "Name")),
        symbol: output(await call(deployed.address, wallet.address, "Symbol")),
        decimals: output(await call(deployed.address, wallet.address, "Decimals")),
        totalSupply: output(await call(deployed.address, wallet.address, "TotalSupply")),
        balance: output(await call(deployed.address, wallet.address, "BalanceOf", [wallet.address])),
      };
      if (meta.name === name && meta.symbol === symbol && meta.decimals === "8") {
        return { ...deployed, ...meta };
      }
    } catch (err) {
      console.log(`token attempt ${i + 1} failed: ${err.message}`);
    }
  }
  throw new Error("could not deploy a working bridge_token after retries");
}

async function main() {
  const wallet = await post(`${WALLET}/wallet/new`, { password: `dex-${Date.now()}` });
  await post(`${CHAIN}/faucet`, { address: wallet.address });
  await sleep(2500);
  const token = await deployWorkingToken(wallet);
  const dex = await get(`${CHAIN}/dex/current`);
  const factory = dex.address;
  if (!factory) throw new Error(`missing factory: ${JSON.stringify(dex)}`);

  let pair = output(await call(factory, wallet.address, "GetPair", ["lqd", token.address]));
  if (!pair) {
    await tx(wallet, factory, "CreatePair", ["lqd", token.address]);
    pair = output(await call(factory, wallet.address, "GetPair", ["lqd", token.address]));
  }
  if (!pair) throw new Error("pair not created");

  await tx(wallet, token.address, "Approve", [pair, "1000000000000"]);
  await tx(wallet, factory, "AddLiquidity", ["lqd", token.address, "10000000000", "20000000000"], "10000000000");
  const pool = output(await call(factory, wallet.address, "GetPoolInfo", ["lqd", token.address]));
  const quote = output(await call(factory, wallet.address, "GetAmountOut", ["100000000", "lqd", token.address]));
  if (!quote || quote === "0") throw new Error(`bad quote ${quote}, pool ${pool}`);
  const before = output(await call(token.address, wallet.address, "BalanceOf", [wallet.address]));
  const swap = await tx(wallet, factory, "SwapExactTokensForTokens", ["100000000", "1", "lqd", token.address], "100000000");
  const after = output(await call(token.address, wallet.address, "BalanceOf", [wallet.address]));
  if (BigInt(after) <= BigInt(before)) throw new Error(`swap did not increase token balance: ${before} -> ${after}`);

  console.log(JSON.stringify({
    wallet: wallet.address,
    token,
    factory,
    pair,
    pool,
    quote,
    swap,
    tokenBalanceBefore: before,
    tokenBalanceAfter: after,
  }, null, 2));
}

main().catch((err) => {
  console.error(err);
  process.exitCode = 1;
});
