import { NODE_URL, WALLET_URL } from "./config";

function getNodeUrl() {
  return localStorage.getItem("lqd_node_url") || NODE_URL;
}

function getWalletUrl() {
  return localStorage.getItem("lqd_wallet_url") || WALLET_URL;
}

export async function callContract({ address, caller, fn, args = [], value = 0 }) {
  const res = await fetch(`${getNodeUrl()}/contract/call`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ address, caller, fn, args, value })
  });
  const text = await res.text();
  let data;
  try {
    data = JSON.parse(text);
  } catch {
    data = { raw: text };
  }
  if (!res.ok) throw new Error(data.error || text || "Contract call failed");
  return data;
}

export async function getContractStorage(address) {
  const res = await fetch(`${getNodeUrl()}/contract/storage?address=${encodeURIComponent(address)}`);
  const text = await res.text();
  let data;
  try {
    data = JSON.parse(text);
  } catch {
    data = { raw: text };
  }
  if (!res.ok) throw new Error(data.error || text || "Storage fetch failed");
  return data;
}

export async function sendContractTx({
  address,
  privateKey,
  contractAddress,
  fn,
  args = [],
  value = "0",
  gas = 0,
  gasPrice = 0
}) {
  // Prefer extension provider if available and no private key supplied
  if ((!privateKey || privateKey === "") && typeof window !== "undefined" && window.lqd) {
    const res = await window.lqd.request({
      method: "lqd_contractTx",
      params: [{
        contract_address: contractAddress,
        function: fn,
        args,
        value,
        gas,
        gas_price: gasPrice
      }]
    });
    return res;
  }

  const res = await fetch(`${getWalletUrl()}/wallet/contract-template`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      address,
      private_key: privateKey,
      contract_address: contractAddress,
      function: fn,
      args,
      value,
      gas,
      gas_price: gasPrice
    })
  });
  const text = await res.text();
  let data;
  try {
    data = JSON.parse(text);
  } catch {
    data = { raw: text };
  }
  if (!res.ok) throw new Error(data.error || text || "Transaction failed");
  return data;
}

export async function getBaseFee() {
  const res = await fetch(`${getNodeUrl()}/basefee`);
  const text = await res.text();
  let data;
  try { data = JSON.parse(text); } catch { data = null; }
  if (!res.ok) throw new Error((data && data.error) || text || "Failed to fetch basefee");
  return (data && data.base_fee) ? data.base_fee : 0;
}

export async function getTokenMeta(token, caller) {
  const name = await tryFn(token, caller, "Name");
  const symbol = await tryFn(token, caller, "Symbol");
  const decimals = await tryFn(token, caller, "Decimals");
  return { name, symbol, decimals };
}

export async function getTokenBalance(token, holder) {
  const res = await callContract({
    address: token,
    caller: holder,
    fn: "BalanceOf",
    args: [holder]
  });
  return res.output || "0";
}

export async function getTokenAllowance(token, owner, spender) {
  try {
    const res = await callContract({
      address: token,
      caller: owner,
      fn: "Allowance",
      args: [owner, spender]
    });
    return res.output || "0";
  } catch {
    return "N/A";
  }
}

async function tryFn(token, caller, fn) {
  try {
    const res = await callContract({ address: token, caller, fn, args: [] });
    return res.output || "";
  } catch {
    return "";
  }
}
