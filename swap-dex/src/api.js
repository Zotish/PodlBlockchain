import { NODE_URL, WALLET_URL } from "./config";

function getNodeUrl() {
  return localStorage.getItem("lqd_node_url") || NODE_URL;
}

function getWalletUrl() {
  return localStorage.getItem("lqd_wallet_url") || WALLET_URL;
}

// Poll until TX appears in a block (confirmed), up to timeoutMs
export async function waitForTx(txHash, timeoutMs = 20000) {
  if (!txHash) return null;
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    await new Promise(r => setTimeout(r, 1200));
    try {
      const res = await fetch(`${getNodeUrl()}/tx/${encodeURIComponent(txHash)}`);
      if (res.ok) {
        const data = await res.json();
        if (data && (data.tx_hash || data.TxHash || data.hash)) return data;
      }
    } catch {}
  }
  return null;
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
  // Unwrap aggregator envelope if present
  return unwrapAggregator(data);
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
  return unwrapAggregator(data);
}

export async function sendContractTx({
  address,
  privateKey,
  contractAddress,
  fn,
  args = [],
  value = "0",
  gas = 0,
  gasPrice = 0,
  onPending = null
}) {
  // Always prefer extension provider when available — ensures popup approval
  if (typeof window !== "undefined" && window.lqd) {
    const res = await window.lqd.request({
      method: "lqd_contractTx",
      params: [{
        contract_address: contractAddress,
        function: fn,
        args,
        value,
        gas,
        gas_price: gasPrice
      }],
      onPending: onPending || (() => {
        // Default: browser notification that approval is needed
        try {
          const evt = new CustomEvent("lqd_approval_needed", { detail: { fn } });
          window.dispatchEvent(evt);
        } catch {}
      })
    });
    // Extension wraps result in { result: ... }, unwrap it
    return res?.result ?? res;
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

export async function getCurrentDexFactory() {
  const res = await fetch(`${getNodeUrl()}/dex/current`);
  const text = await res.text();
  let data;
  try { data = JSON.parse(text); } catch { data = { raw: text }; }
  if (!res.ok) throw new Error(data.error || text || "Failed to fetch current DEX factory");
  return data?.address || "";
}

export async function getContractAbi(address) {
  const res = await fetch(`${getNodeUrl()}/contract/getAbi?address=${encodeURIComponent(address)}`);
  const text = await res.text();
  let data;
  try { data = JSON.parse(text); } catch { data = { raw: text }; }
  if (!res.ok) throw new Error(data.error || text || "ABI fetch failed");
  const inner = unwrapAggregator(data);
  if (Array.isArray(inner)) return inner;
  if (Array.isArray(inner?.entries)) return inner.entries;
  if (Array.isArray(inner?.abi)) return inner.abi;
  if (Array.isArray(data?.entries)) return data.entries;
  if (Array.isArray(data?.abi)) return data.abi;
  return [];
}

export async function getTokenMeta(token, caller) {
  const name = await tryFn(token, caller, "Name");
  const symbol = await tryFn(token, caller, "Symbol");
  const decimals = await tryFn(token, caller, "Decimals");
  return { name, symbol, decimals };
}

// Unwrap aggregator envelope: { nodes: [{ result: {...} }] } → inner result
function unwrapAggregator(data) {
  if (data?.nodes?.[0]?.result !== undefined) return data.nodes[0].result;
  return data;
}

export async function getNativeBalance(address) {
  const res = await fetch(`${getNodeUrl()}/balance?address=${encodeURIComponent(address)}`);
  const text = await res.text();
  let data;
  try { data = JSON.parse(text); } catch { data = null; }
  if (!res.ok) return "0";
  const inner = unwrapAggregator(data);
  return inner?.balance ?? inner?.Balance ?? "0";
}

export async function getTokenBalance(token, holder) {
  // Native LQD uses blockchain balance endpoint, not contract
  if (token === "lqd") return getNativeBalance(holder);
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
