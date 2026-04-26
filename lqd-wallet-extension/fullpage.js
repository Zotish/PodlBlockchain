"use strict";
const ext = typeof chrome !== "undefined" ? chrome : browser;
const PROD_CHAIN_URL = "https://dazzling-peace-production-3529.up.railway.app";
const PROD_WALLET_URL = "https://enchanting-hope-production-1c63.up.railway.app";
const PROD_AGGREGATOR_URL = "https://keen-enjoyment-production-0440.up.railway.app";
const PROD_EXPLORER_URL = "https://warm-dragon-34d6ff.netlify.app";

// ─── State ────────────────────────────────────────────────────────────────────
let state = {
  address: "",
  nodeUrl:   PROD_CHAIN_URL,
  walletUrl: PROD_WALLET_URL,
  tokens: [],          // [{ address, symbol, name, decimals }]
  compiledBinary: null // last compiler output
};
let bridgeMode = "public";
let bridgeChainId = "bsc-testnet";
let bridgeFamilyId = "evm";
let bridgeChains = [];
let bridgeFamilies = [];

// ─── Helpers ──────────────────────────────────────────────────────────────────
function $(id) { return document.getElementById(id); }

function msg(type, payload = {}) {
  return new Promise((resolve) => ext.runtime.sendMessage({ type, ...payload }, resolve));
}

let toastTimer;
function toast(text, kind = "info") {
  const el = $("toast");
  el.textContent = text;
  el.className = `toast show ${kind}`;
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => { el.className = "toast"; }, 4000);
}

function showResult(elId, text, isErr = false) {
  const el = $(elId);
  if (!el) return;
  el.style.display = "block";
  el.style.color = isErr ? "var(--red)" : "var(--green)";
  el.textContent = text;
}

function fmtAmount(raw, dec = 8) {
  try {
    if (!raw || raw === "0") return "0";
    const b = BigInt(raw);
    const d = BigInt(10 ** dec);
    const w = b / d, f = b % d;
    const fs = f.toString().padStart(dec, "0").replace(/0+$/, "");
    return fs ? `${w}.${fs}` : w.toString();
  } catch { return raw || "0"; }
}

// Convert human-readable "1.5" → raw base units "150000000" (pure string, no precision loss)
function parseHuman(humanStr, decimals = 8) {
  if (!humanStr && humanStr !== 0) return "0";
  const s = String(humanStr).trim();
  const dotIdx = s.indexOf(".");
  let intS = dotIdx === -1 ? s : s.slice(0, dotIdx);
  let fracS = dotIdx === -1 ? "" : s.slice(dotIdx + 1);
  const frac = fracS.slice(0, decimals).padEnd(decimals, "0");
  const full = (intS.replace(/^0+/, "") || "0") + frac;
  return full.replace(/^0+/, "") || "0";
}

function shortAddr(a) {
  if (!a || a.length < 10) return a || "";
  return `${a.slice(0, 6)}…${a.slice(-4)}`;
}

function tokenColor(sym) {
  const hue = ((sym || "?").charCodeAt(0) * 47 + 120) % 360;
  return `hsl(${hue},55%,42%)`;
}

// ─── API calls ────────────────────────────────────────────────────────────────
async function nodeGet(path) {
  const r = await fetch(state.nodeUrl + path);
  if (!r.ok) throw new Error(await r.text());
  return r.json();
}

async function waitForTx(txHash, timeoutMs = 30000) {
  if (!txHash) return null;
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    await new Promise(r => setTimeout(r, 1200));
    try {
      const res = await nodeGet(`/tx/${encodeURIComponent(txHash)}`);
      if (res && (res.tx_hash || res.TxHash || res.hash)) return res;
    } catch {}
  }
  return null;
}

async function nodePost(path, body) {
  const r = await fetch(state.nodeUrl + path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body)
  });
  const text = await r.text();
  let data; try { data = JSON.parse(text); } catch { data = { raw: text }; }
  if (!r.ok) throw new Error(data.error || text);
  return data;
}

async function walletPost(path, body) {
  const r = await fetch(state.walletUrl + path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body)
  });
  const text = await r.text();
  let data; try { data = JSON.parse(text); } catch { data = { raw: text }; }
  if (!r.ok) throw new Error(data.error || text);
  return data;
}

async function contractCall(contractAddr, fn, args = []) {
  return nodePost("/contract/call", {
    address: contractAddr, caller: state.address,
    fn, args, value: 0
  });
}

async function contractTx(contractAddr, fn, args = [], value = "0") {
  return walletPost("/wallet/contract-template", {
    address: state.address,
    private_key: await getPrivateKey(),
    contract_address: contractAddr,
    function: fn, args, value
  });
}

async function getPrivateKey() {
  // background holds private key in session while unlocked
  const res = await msg("LQD_REQUEST", { payload: { id: "fp-pk", method: "lqd_getPrivateKey" } });
  if (res?.result) return res.result;
  // fallback: prompt (extension session stores it)
  throw new Error("Private key not available — wallet may be locked");
}

// ─── Init ─────────────────────────────────────────────────────────────────────
async function init() {
  // Load config from background
  const cfgRes = await msg("LQD_GET_NETWORKS");
  if (cfgRes?.ok) {
    const nets = cfgRes.networks || {};
    const cur  = cfgRes.currentNetwork;
    const net  = nets[cur];
    if (net) { state.nodeUrl = net.nodeUrl; state.walletUrl = net.walletUrl; }
  }

  // Load saved endpoints override
  const stored = await ext.storage.local.get(["nodeUrl", "walletUrl", "address"]);
  if (stored.nodeUrl)   state.nodeUrl   = stored.nodeUrl;
  if (stored.walletUrl) state.walletUrl = stored.walletUrl;

  // Auto-migrate legacy local defaults to production endpoints.
  if (
    state.nodeUrl &&
    (state.nodeUrl.includes(":5000") ||
      state.nodeUrl.includes(":6500") ||
      state.nodeUrl.includes(":9000") ||
      state.nodeUrl.includes("127.0.0.1") ||
      state.nodeUrl.includes("localhost"))
  ) {
    state.nodeUrl = PROD_CHAIN_URL;
    await ext.storage.local.set({ nodeUrl: state.nodeUrl });
  }
  if (
    state.walletUrl &&
    (state.walletUrl.includes(":8080") ||
      state.walletUrl.includes("127.0.0.1") ||
      state.walletUrl.includes("localhost"))
  ) {
    state.walletUrl = PROD_WALLET_URL;
    await ext.storage.local.set({ walletUrl: state.walletUrl });
  }

  // Populate settings inputs
  $("settingsNodeUrl").value  = state.nodeUrl;
  $("settingsWalletUrl").value = state.walletUrl;
  await loadBridgeFamilies();
  await loadBridgeTokens();
  await loadBridgeChains();

  // Check lock state
  const lockData = await ext.storage.local.get(["address", "locked"]);
  if (!lockData.address) {
    // No wallet created — redirect to popup
    document.body.innerHTML = `
      <div style="display:flex;height:100vh;align-items:center;justify-content:center;flex-direction:column;gap:16px;font-family:Inter,sans-serif;background:#0D0F14;color:#eee;">
        <div style="font-size:48px">🔐</div>
        <div style="font-size:20px;font-weight:700">No wallet found</div>
        <div style="color:#636E8A">Please create or import a wallet first.</div>
        <button onclick="window.close()" style="margin-top:8px;padding:10px 24px;background:#7B61FF;color:#fff;border:0;border-radius:8px;cursor:pointer;font-size:14px;font-weight:600;">Close</button>
      </div>`;
    return;
  }

  if (lockData.locked !== false) {
    showLockScreen();
  } else {
    state.address = lockData.address;
    showApp();
  }
}

// ─── Lock screen ──────────────────────────────────────────────────────────────
function showLockScreen() {
  $("lockScreen").classList.add("show");
  $("appShell").style.display = "none";
}

function showApp() {
  $("lockScreen").classList.remove("show");
  $("appShell").style.display = "flex";
  $("chipAddr").textContent = state.address;
  $("chipLabel").textContent = shortAddr(state.address);
  $("receiveAddr").textContent = state.address;
  _bscAccount = state.address;
  loadBridgeFamilies().catch(() => {});
  loadBridgeTokens().catch(() => {});
  loadBridgeChains().catch(() => {});
  useExtensionWalletSigner().catch(() => {});
  loadTokens();
  refreshBalance();
  loadActivity();
  loadNetworkList();
}

$("lockUnlockBtn").addEventListener("click", async () => {
  const pass = $("lockPass").value;
  if (!pass) return;
  const res = await msg("LQD_UNLOCK", { password: pass });
  if (res?.ok) {
    const d = await ext.storage.local.get(["address"]);
    state.address = d.address || "";
    $("lockErr").style.display = "none";
    showApp();
  } else {
    $("lockErr").textContent = res?.error || "Wrong password";
    $("lockErr").style.display = "block";
  }
});

$("lockPass").addEventListener("keydown", (e) => { if (e.key === "Enter") $("lockUnlockBtn").click(); });

// ─── Navigation ───────────────────────────────────────────────────────────────
document.querySelectorAll(".nav-item").forEach(btn => {
  btn.addEventListener("click", () => {
    document.querySelectorAll(".nav-item").forEach(b => b.classList.remove("active"));
    document.querySelectorAll(".page").forEach(p => p.classList.remove("active"));
    btn.classList.add("active");
    const page = btn.dataset.page;
    $(`page-${page}`)?.classList.add("active");
    if (page === "activity") loadActivity();
    if (page === "bridge")   loadBridgeHistory();
    if (page === "settings") loadNetworkList();
  });
});

// ─── Subtab helper ────────────────────────────────────────────────────────────
function initSubtabs(containerId, prefix) {
  document.querySelectorAll(`#${containerId} .subtab`).forEach(btn => {
    btn.addEventListener("click", () => {
      document.querySelectorAll(`#${containerId} .subtab`).forEach(b => b.classList.remove("active"));
      btn.classList.add("active");
      document.querySelectorAll(`[id^="${prefix}-"]`).forEach(p => p.style.display = "none");
      $(`${prefix}-${btn.dataset.sub || btn.dataset.exp}`)?.removeAttribute("style");
    });
  });
}
initSubtabs("contractSubtabs", "sub");

// ══════════════════════════════════════════════════════════════════
// WALLET PAGE
// ══════════════════════════════════════════════════════════════════
async function refreshBalance() {
  try {
    const data = await nodeGet(`/balance?address=${state.address}`);
    const bal = data.balance ?? data.confirmed ?? "0";
    const pend = data.pending ?? "0";
    $("heroBalance").textContent = fmtAmount(bal);
    $("statConfirmed").textContent = fmtAmount(bal) + " LQD";
    $("statPending").textContent   = fmtAmount(pend) + " LQD";
  } catch (e) { toast("Balance load failed: " + e.message, "error"); }
}

$("refreshBalBtn").addEventListener("click", refreshBalance);

$("faucetBtn").addEventListener("click", async () => {
  try {
    await nodePost("/faucet", { address: state.address });
    toast("Faucet request sent!", "success");
    setTimeout(refreshBalance, 2000);
  } catch (e) { toast("Faucet failed: " + e.message, "error"); }
});

// Send LQD
$("openSendBtn").addEventListener("click", () => {
  $("sendCard").style.display = "block";
  $("receiveCard").style.display = "none";
});
$("cancelSendBtn").addEventListener("click", () => { $("sendCard").style.display = "none"; });

$("doSendBtn").addEventListener("click", async () => {
  const to  = $("sendTo").value.trim();
  const amt = $("sendAmt").value.trim();
  const gp  = parseInt($("sendGasPrice").value) || 1;
  const gl  = parseInt($("sendGasLimit").value) || 21000;
  if (!to || !amt) { toast("Fill all fields", "error"); return; }

  // Convert human LQD (e.g. "1.5") → raw base units (e.g. "150000000")
  const rawValue = parseHuman(amt, 8);

  try {
    $("doSendBtn").disabled = true;
    const pk = await getPrivateKey();
    const res = await walletPost("/wallet/send", {
      from: state.address, to,
      value: rawValue, private_key: pk,
      gas: gl, gas_price: gp
    });
    const hash = res.tx_hash || res.TxHash || res.hash || "";
    if (hash) { await waitForTx(hash, 5000).catch(() => null); }
    showResult("sendResult", `✓ Sent ${amt} LQD! Tx: ${hash}`);
    toast(`Sent ${amt} LQD!`, "success");
    await recordLocalActivity({ type: "send", to, value: rawValue, tx_hash: hash });
    $("sendTo").value = ""; $("sendAmt").value = "";
    await refreshBalance();
    try { window.dispatchEvent(new CustomEvent("lqd:wallet-updated", { detail: { address: state.address } })); } catch {}
  } catch (e) { showResult("sendResult", "✗ " + e.message, true); toast(e.message, "error"); }
  finally { $("doSendBtn").disabled = false; }
});

// Receive
$("openReceiveBtn").addEventListener("click", () => {
  $("receiveCard").style.display = "block";
  $("sendCard").style.display = "none";
});
$("closeReceiveBtn").addEventListener("click", () => { $("receiveCard").style.display = "none"; });
$("copyAddrBtn").addEventListener("click", () => {
  navigator.clipboard.writeText(state.address);
  toast("Address copied!", "success");
});

// ══════════════════════════════════════════════════════════════════
// TOKENS PAGE
// ══════════════════════════════════════════════════════════════════
function loadTokens() {
  const raw = localStorage.getItem(`lqd_tokens_fp_${state.address}`);
  try { state.tokens = raw ? JSON.parse(raw) : []; } catch { state.tokens = []; }
  renderTokenList();
}

function saveTokens() {
  localStorage.setItem(`lqd_tokens_fp_${state.address}`, JSON.stringify(state.tokens));
}

function upsertToken(t) {
  const idx = state.tokens.findIndex(x => x.address.toLowerCase() === t.address.toLowerCase());
  if (idx >= 0) state.tokens[idx] = { ...state.tokens[idx], ...t };
  else state.tokens.push(t);
  saveTokens();
  renderTokenList();
}

async function fetchTokenMeta(addr) {
  const tryCall = async (fn) => {
    try { const r = await contractCall(addr, fn); return r.output || ""; } catch { return ""; }
  };
  const [name, symbol, decimals, balance] = await Promise.all([
    tryCall("Name"), tryCall("Symbol"), tryCall("Decimals"),
    tryCall(`BalanceOf`).catch(() => "0")
  ]);
  // BalanceOf needs arg
  let bal = "0";
  try { const r = await contractCall(addr, "BalanceOf", [state.address]); bal = r.output || "0"; } catch {}
  return { name: name || "Unknown", symbol: symbol || addr.slice(2,6).toUpperCase(), decimals: parseInt(decimals) || 8, balance: bal };
}

function renderTokenList() {
  const container = $("tokenList");
  if (!state.tokens.length) {
    container.innerHTML = '<div class="notice">No tokens imported yet.</div>';
    return;
  }
  container.innerHTML = "";
  state.tokens.forEach(t => {
    const row = document.createElement("div");
    row.className = "token-row";
    row.innerHTML = `
      <div class="token-icon-sm" style="background:${tokenColor(t.symbol)}">${(t.symbol||"?")[0]}</div>
      <div class="token-info">
        <div class="token-sym">${t.symbol}</div>
        <div class="token-name">${t.name} · ${t.address.slice(0,10)}…</div>
      </div>
      <div style="text-align:right;">
        <div class="token-bal">${fmtAmount(t.balance || "0", parseInt(t.decimals)||8)}</div>
        <div style="display:flex;gap:6px;margin-top:4px;justify-content:flex-end;">
          <button class="btn btn-secondary btn-sm sendTokBtn" data-addr="${t.address}" data-sym="${t.symbol}">Send</button>
          <button class="btn btn-secondary btn-sm refreshTokBtn" data-addr="${t.address}">⟳</button>
        </div>
      </div>`;
    container.appendChild(row);
  });

  container.querySelectorAll(".sendTokBtn").forEach(b => {
    b.addEventListener("click", () => openSendToken(b.dataset.addr, b.dataset.sym));
  });
  container.querySelectorAll(".refreshTokBtn").forEach(b => {
    b.addEventListener("click", () => refreshTokenBal(b.dataset.addr));
  });
}

async function refreshTokenBal(addr) {
  try {
    const r = await contractCall(addr, "BalanceOf", [state.address]);
    const idx = state.tokens.findIndex(t => t.address.toLowerCase() === addr.toLowerCase());
    if (idx >= 0) { state.tokens[idx].balance = r.output || "0"; saveTokens(); renderTokenList(); }
  } catch {}
}

$("doImportTokenBtn").addEventListener("click", async () => {
  const addr = $("importTokenAddr").value.trim();
  if (!addr) { toast("Enter token address", "error"); return; }
  try {
    $("doImportTokenBtn").disabled = true;
    const meta = await fetchTokenMeta(addr);
    upsertToken({ address: addr, ...meta });
    $("importTokenAddr").value = "";
    toast(`Token ${meta.symbol} imported!`, "success");
  } catch (e) { toast("Import failed: " + e.message, "error"); }
  finally { $("doImportTokenBtn").disabled = false; }
});

$("autoImportBtn").addEventListener("click", async () => {
  toast("Scanning for tokens…", "info");
  try {
    // Fetch recent contract interactions from activity
    const acts = JSON.parse(localStorage.getItem(`lqd_activity_${state.address}`) || "[]");
    const addrs = [...new Set(acts.filter(a => a.contract).map(a => a.contract))];
    let found = 0;
    for (const addr of addrs.slice(0, 20)) {
      try {
        const name = await contractCall(addr, "Name").then(r => r.output).catch(() => null);
        if (name) {
          const meta = await fetchTokenMeta(addr);
          upsertToken({ address: addr, ...meta });
          found++;
        }
      } catch {}
    }
    toast(`Auto-scan done. Found ${found} token(s).`, "success");
  } catch (e) { toast("Scan failed: " + e.message, "error"); }
});

// Send token
let _sendTokenAddr = "";
let _sendTokenDec = 8;
function openSendToken(addr, sym) {
  _sendTokenAddr = addr;
  // Look up token decimals from state
  const tok = state.tokens.find(t => t.address === addr);
  _sendTokenDec = tok ? (parseInt(tok.decimals) || 8) : 8;
  $("sendTokenSym").textContent = sym;
  $("sendTokenCard").style.display = "block";
  $("sendTokenTo").value = "";
  $("sendTokenAmt").value = "";
  // Navigate to tokens page if not already there
}
$("cancelSendTokenBtn").addEventListener("click", () => { $("sendTokenCard").style.display = "none"; });

$("doSendTokenBtn").addEventListener("click", async () => {
  const to  = $("sendTokenTo").value.trim();
  const amt = $("sendTokenAmt").value.trim();
  if (!to || !amt) { toast("Fill all fields", "error"); return; }
  // Convert human token amount → raw base units using token's own decimals
  const rawAmt = parseHuman(amt, _sendTokenDec);
  try {
    $("doSendTokenBtn").disabled = true;
    const res = await contractTx(_sendTokenAddr, "Transfer", [to, rawAmt]);
    const hash = res.tx_hash || res.TxHash || "";
    if (hash) { await waitForTx(hash, 5000).catch(() => null); }
    showResult("sendTokenResult", `✓ Sent ${amt} tokens! Tx: ${hash}`);
    toast("Token sent!", "success");
    await recordLocalActivity({ type: "token", to, contract: _sendTokenAddr, value: rawAmt, tx_hash: hash });
    await refreshTokenBal(_sendTokenAddr);
    await refreshBalance();
    try { window.dispatchEvent(new CustomEvent("lqd:wallet-updated", { detail: { address: state.address, token: _sendTokenAddr } })); } catch {}
  } catch (e) { showResult("sendTokenResult", "✗ " + e.message, true); }
  finally { $("doSendTokenBtn").disabled = false; }
});

// ══════════════════════════════════════════════════════════════════
// CONTRACTS PAGE
// ══════════════════════════════════════════════════════════════════

// ── Quick Deploy args ──────────────────────────────────────────────
const QUICK_ARGS = {
  lqd20:         [{ id:"q_name",    label:"Token Name",        ph:"My Token"            },
                   { id:"q_sym",     label:"Symbol",            ph:"MTK"                 },
                   { id:"q_supply",  label:"Initial Supply",    ph:"1000000000000000"    }],
  dex_swap:      [],   // no init args at deploy time
  bridge_token:  [{ id:"q_bname",   label:"Token Name",        ph:"Wrapped BNB"         },
                   { id:"q_bsym",    label:"Symbol",            ph:"wBNB"                },
                   { id:"q_bdec",    label:"Decimals",          ph:"18"                  },
                   { id:"q_bbsc",    label:"BSC Token Address", ph:"0x..."               }],
  lending_pool:  [],
  nft_collection:[{ id:"q_nname",   label:"Collection Name",   ph:"My NFT"              },
                   { id:"q_nsym",    label:"Symbol",            ph:"MNFT"                }],
  dao_treasury:  [],
};

$("quickDeployType").addEventListener("change", renderQuickArgs);
function renderQuickArgs() {
  const type = $("quickDeployType").value;
  const fields = QUICK_ARGS[type] || [];
  $("quickDeployArgs").innerHTML = fields.map(f => `
    <div class="field">
      <label>${f.label}</label>
      <input id="${f.id}" placeholder="${f.ph}" />
    </div>`).join("");
}
renderQuickArgs();

$("quickDeployBtn").addEventListener("click", async () => {
  const type = $("quickDeployType").value;
  const fields = QUICK_ARGS[type] || [];
  const args = fields.map(f => $(f.id)?.value?.trim() || "");
  if (args.some(a => a === "") && fields.length) {
    toast("Fill all required fields", "error"); return;
  }
  if (!state.address) { toast("⚠️ Wallet not connected — please unlock your wallet first", "error"); return; }
  try {
    $("quickDeployBtn").disabled = true;
    $("quickDeployBtn").textContent = "Compiling & Deploying… (~15s)";
    const pk = await getPrivateKey();
    // Quick deploy uses builtin endpoint — compiles+deploys the template automatically
    const res = await nodePost("/contract/deploy-builtin", {
      template: type,
      owner: state.address,
      private_key: pk,
      gas: 500000,
      init_args: args   // name, symbol, supply etc. — passed to contract's Init()
    });
    const addr = res.address || res.contract_address || "";
    if (!addr) {
      throw new Error(`Deploy returned no contract address. Response: ${JSON.stringify(res)}`);
    }
    showResult("quickDeployResult",
      `✓ Deployed!\nAddress: ${addr}\nType: ${type}${res.tx_hash ? "\nTx: " + res.tx_hash : ""}${args.length ? "\nArgs: " + args.join(", ") : ""}`);
    toast("Contract deployed: " + shortAddr(addr), "success");
    await recordLocalActivity({ type: "deploy", contract: addr, contractType: type, tx_hash: res.tx_hash || "" });
  } catch (e) {
    showResult("quickDeployResult", "✗ " + e.message, true);
    toast("Deploy failed: " + e.message, "error");
  } finally {
    $("quickDeployBtn").disabled = false;
    $("quickDeployBtn").textContent = "🚀 Deploy Contract";
  }
});

// ── File Deploy ────────────────────────────────────────────────────
const fileDrop = $("fileDrop");
const fileInput = $("contractFile");

fileDrop.addEventListener("click", () => fileInput.click());
fileDrop.addEventListener("dragover", e => { e.preventDefault(); fileDrop.classList.add("drag"); });
fileDrop.addEventListener("dragleave", () => fileDrop.classList.remove("drag"));
fileDrop.addEventListener("drop", e => {
  e.preventDefault(); fileDrop.classList.remove("drag");
  if (e.dataTransfer.files[0]) setFile(e.dataTransfer.files[0]);
});
fileInput.addEventListener("change", () => { if (fileInput.files[0]) setFile(fileInput.files[0]); });

let _contractFile = null;
function setFile(f) {
  _contractFile = f;
  $("fileDropName").textContent = `📄 ${f.name} (${(f.size/1024).toFixed(1)} KB)`;
}

$("fileDeployBtn").addEventListener("click", async () => {
  if (!_contractFile) { toast("Select a contract file first", "error"); return; }
  if (!state.address) { toast("⚠️ Wallet not connected — please unlock your wallet first", "error"); return; }
  try {
    $("fileDeployBtn").disabled = true;
    $("fileDeployBtn").textContent = "Deploying…";
    const pk = await getPrivateKey();
    const form = new FormData();
    form.append("contract_file", _contractFile);
    form.append("owner", state.address);
    form.append("private_key", pk);
    form.append("type", $("fileDeployType").value);
    form.append("gas", $("fileDeployGas").value || "50000");

    const r = await fetch(`${state.nodeUrl}/contract/deploy`, { method: "POST", body: form });
    const data = await r.json();
    if (!r.ok) throw new Error(data.error || "Deploy failed");
    const addr = data.address || "";
    showResult("fileDeployResult", `✓ Deployed!\nAddress: ${addr}\nType: ${data.type || ""}`);
    toast("Contract deployed!", "success");
    await recordLocalActivity({ type: "deploy", contract: addr, tx_hash: data.tx_hash || "" });
  } catch (e) {
    showResult("fileDeployResult", "✗ " + e.message, true);
    toast("Deploy failed: " + e.message, "error");
  } finally {
    $("fileDeployBtn").disabled = false;
    $("fileDeployBtn").textContent = "🚀 Deploy File";
  }
});

// ── Contract Call ──────────────────────────────────────────────────
let _callAbi = [];

$("loadAbiBtn").addEventListener("click", async () => {
  const addr = $("callContractAddr").value.trim();
  if (!addr) { toast("Enter contract address", "error"); return; }
  try {
    $("loadAbiBtn").disabled = true;
    const data = await nodeGet(`/contract/getAbi?address=${encodeURIComponent(addr)}`);
    _callAbi = Array.isArray(data) ? data : (data.entries || data.abi || data.functions || []);
    renderCallFnSelect();
    $("callFnSection").style.display = "block";
    toast(`ABI loaded: ${_callAbi.length} function(s)`, "success");
  } catch {
    // No ABI — allow manual function entry
    _callAbi = [];
    renderManualCallSection();
    $("callFnSection").style.display = "block";
    toast("No ABI found — using manual mode", "info");
  } finally { $("loadAbiBtn").disabled = false; }
});

function renderCallFnSelect() {
  const sel = $("callFnSelect");
  sel.innerHTML = '<option value="">— select function —</option>';
  _callAbi.forEach((fn, i) => {
    const opt = document.createElement("option");
    opt.value = i;
    opt.textContent = `${fn.name}(${(fn.inputs||[]).map(inp => inp.type || inp).join(", ")})`;
    sel.appendChild(opt);
  });
  $("callArgsSection").innerHTML = "";
}

function renderManualCallSection() {
  const sel = $("callFnSelect");
  sel.innerHTML = "";
  const opt = document.createElement("option");
  opt.value = "__manual__"; opt.textContent = "Manual — type function name";
  sel.appendChild(opt);
  $("callArgsSection").innerHTML = `
    <div class="field"><label>Function Name</label><input id="manualFnName" placeholder="e.g. Transfer" /></div>
    <div class="field"><label>Arguments (comma-separated)</label><input id="manualFnArgs" placeholder='e.g. 0x123..., 1000' /></div>`;
}

$("callFnSelect").addEventListener("change", () => {
  const idx = parseInt($("callFnSelect").value);
  if (isNaN(idx)) { $("callArgsSection").innerHTML = ""; return; }
  const fn = _callAbi[idx];
  if (!fn) return;
  const inputs = fn.inputs || [];
  $("callArgsSection").innerHTML = inputs.length
    ? inputs.map((inp, i) => `
        <div class="field">
          <label>${inp.name || "arg" + i} (${inp.type || "string"})</label>
          <input class="call-arg" data-idx="${i}" placeholder="${inp.type || "value"}" />
        </div>`).join("")
    : '<div class="notice" style="margin-bottom:12px;">No arguments required.</div>';
});

$("doCallBtn").addEventListener("click",  () => executeCall(false));
$("doWriteBtn").addEventListener("click", () => executeCall(true));

async function executeCall(isWrite) {
  const addr = $("callContractAddr").value.trim();
  const selVal = $("callFnSelect").value;
  let fn, args;

  if (selVal === "__manual__") {
    fn   = $("manualFnName")?.value?.trim() || "";
    args = ($("manualFnArgs")?.value || "").split(",").map(s => s.trim()).filter(Boolean);
  } else {
    const idx = parseInt(selVal);
    if (!addr || isNaN(idx)) { toast("Select a function", "error"); return; }
    const fnDef = _callAbi[idx];
    fn = fnDef.name;
    args = [...document.querySelectorAll(".call-arg")].map(el => el.value.trim());
  }

  if (!fn) { toast("Enter function name", "error"); return; }

  try {
    $("doCallBtn").disabled = $("doWriteBtn").disabled = true;
    let res;
    if (isWrite) {
      const value = $("callValue").value || "0";
      res = await contractTx(addr, fn, args, value);
      showResult("callResult", `✓ Tx submitted!\n${JSON.stringify(res, null, 2)}`);
      toast("Transaction sent!", "success");
      await recordLocalActivity({ type: "contract", contract: addr, function: fn, args, tx_hash: res.tx_hash || "" });
    } else {
      res = await contractCall(addr, fn, args);
      const out = res.output ?? res.result ?? JSON.stringify(res, null, 2);
      showResult("callResult", `Output: ${out}`);
    }
  } catch (e) {
    showResult("callResult", "✗ " + e.message, true);
    toast(e.message, "error");
  } finally { $("doCallBtn").disabled = $("doWriteBtn").disabled = false; }
}

// ── Compiler ───────────────────────────────────────────────────────
// Show/hide hint when compile type changes
$("compileType").addEventListener("change", () => {
  const t = $("compileType").value;
  $("gopluginHint").style.display = t === "goplugin" ? "block" : "none";
});

// Store compiled type alongside binary for deploy
let _compiledType = "gocode";

$("doCompileBtn").addEventListener("click", async () => {
  const type   = $("compileType").value;
  const source = $("compileSource").value.trim();
  if (!source) { toast("Enter source code", "error"); return; }

  $("doCompileBtn").disabled = true;
  $("doCompileBtn").textContent = "Compiling…";
  $("compileDeploySection").style.display = "none";
  $("compileResult").style.display = "none";
  state.compiledBinary = null;

  try {
    if (type === "goplugin") {
      // ── Go Plugin: compile via /contract/compile-plugin ──────────
      // This can take a few seconds (go build)
      $("doCompileBtn").textContent = "Building .so… (may take ~10s)";
      const res = await nodePost("/contract/compile-plugin", { source });

      if (!res.success) {
        // Compiler error — show formatted error
        showResult("compileResult",
          "✗ Compile error:\n\n" + (res.error || "unknown error"), true);
        toast("Compile failed — check errors", "error");
        return;
      }

      // Decode base64 → Uint8Array → store as Blob
      const binStr = atob(res.binary);
      const bytes  = new Uint8Array(binStr.length);
      for (let i = 0; i < binStr.length; i++) bytes[i] = binStr.charCodeAt(i);
      state.compiledBinary = new Blob([bytes], { type: "application/octet-stream" });
      _compiledType = "plugin";

      const kb = (res.size / 1024).toFixed(1);
      if ($("compiledSizeInfo")) $("compiledSizeInfo").textContent = `Plugin size: ${kb} KB`;
      showResult("compileResult",
        `✓ Go Plugin compiled!\nSize: ${kb} KB\nReady to deploy as .so plugin.`);
      $("compileDeploySection").style.display = "block";
      toast("Plugin compiled! Click Deploy to go live.", "success");

    } else {
      // ── gocode / dsl / solidity ───────────────────────────────────
      const res = await nodePost("/contract/compile", { type, source });
      state.compiledBinary = res.binary || res.bytecode || null;
      _compiledType = type;
      showResult("compileResult",
        `✓ Compiled!\nType: ${type}\nSize: ${res.size || "?"} bytes`);
      if (state.compiledBinary) $("compileDeploySection").style.display = "block";
      toast("Compiled!", "success");
    }

  } catch (e) {
    showResult("compileResult", "✗ Error:\n" + e.message, true);
    toast("Compile failed", "error");
  } finally {
    $("doCompileBtn").disabled = false;
    $("doCompileBtn").textContent = "⚙️ Compile";
  }
});

$("deployCompiledBtn").addEventListener("click", async () => {
  if (!state.compiledBinary) { toast("Nothing compiled yet", "error"); return; }
  if (!state.address) { toast("⚠️ Wallet not connected — please unlock your wallet first", "error"); return; }
  try {
    $("deployCompiledBtn").disabled = true;
    $("deployCompiledBtn").textContent = "Deploying…";
    const pk = await getPrivateKey();

    const form = new FormData();
    // For goplugin, compiledBinary is a Blob; for others it may be a string
    const fileBlob = state.compiledBinary instanceof Blob
      ? state.compiledBinary
      : new Blob([state.compiledBinary], { type: "application/octet-stream" });
    const fileName = _compiledType === "plugin" ? "contract.so" : "contract.lqd";
    form.append("contract_file", fileBlob, fileName);
    form.append("owner", state.address);
    form.append("private_key", pk);
    form.append("type", _compiledType);
    form.append("gas", "500000");

    const r = await fetch(`${state.nodeUrl}/contract/deploy`, { method: "POST", body: form });
    const data = await r.json();
    if (!r.ok) throw new Error(data.error || "Deploy failed");

    const addr = data.address || "";
    if (!addr) throw new Error(`Deploy returned no contract address. Response: ${JSON.stringify(data)}`);
    showResult("compileResult",
      `✓ Contract Deployed!\nAddress: ${addr}\nType: ${_compiledType}${data.tx_hash ? "\nTx: " + data.tx_hash : ""}`);
    toast("🎉 Contract live: " + shortAddr(addr), "success");
    await recordLocalActivity({ type: "deploy", contract: addr, contractType: _compiledType, tx_hash: data.tx_hash || "" });
    $("compileDeploySection").style.display = "none";
    state.compiledBinary = null;

  } catch (e) {
    showResult("compileResult", "✗ Deploy failed:\n" + e.message, true);
    toast("Deploy failed: " + e.message, "error");
  } finally {
    $("deployCompiledBtn").disabled = false;
    $("deployCompiledBtn").textContent = "🚀 Deploy to Chain";
  }
});

// ── Explorer ───────────────────────────────────────────────────────
initSubtabs("explorerSubtabs", "exp");

$("exploreBtn").addEventListener("click", async () => {
  const addr = $("explorerAddr").value.trim();
  if (!addr) { toast("Enter contract address", "error"); return; }
  try {
    $("exploreBtn").disabled = true;
    $("explorerResult").style.display = "none";

    const [abiData, storageData, eventsData] = await Promise.all([
      nodeGet(`/contract/getAbi?address=${encodeURIComponent(addr)}`).catch(() => null),
      nodeGet(`/contract/storage?address=${encodeURIComponent(addr)}`).catch(() => null),
      nodeGet(`/contract/events?address=${encodeURIComponent(addr)}`).catch(() => null),
    ]);

    const abi = Array.isArray(abiData) ? abiData : (abiData?.entries || abiData?.abi || abiData?.functions || []);
    const storage = storageData?.State?.storage ?? storageData?.State ?? storageData ?? {};
    const events  = Array.isArray(eventsData) ? eventsData : (eventsData?.events || []);

    // Overview
    $("exp-overview").innerHTML = `
      <div class="stat-box" style="margin-bottom:10px;">
        <div class="stat-label">Address</div>
        <div class="mono" style="font-size:13px;margin-top:4px;word-break:break-all;">${addr}</div>
      </div>
      <div class="grid-2" style="margin-bottom:10px;">
        <div class="stat-box"><div class="stat-label">Functions</div><div class="stat-value">${abi.length}</div></div>
        <div class="stat-box"><div class="stat-label">Events</div><div class="stat-value">${events.length}</div></div>
      </div>
      <div class="notice">Owner: ${storage.owner || storage.Owner || "—"}</div>`;

    // Storage
    const storageEntries = Object.entries(storage);
    $("exp-storage").innerHTML = storageEntries.length
      ? `<table style="width:100%;border-collapse:collapse;font-size:13px;">
           <tr style="color:var(--muted)"><th style="text-align:left;padding:6px 0;">Key</th><th style="text-align:left;padding:6px 0;">Value</th></tr>
           ${storageEntries.map(([k,v]) => `<tr style="border-top:1px solid var(--border)">
             <td style="padding:8px 0;font-family:monospace;color:var(--accent2);max-width:200px;overflow:hidden;text-overflow:ellipsis;">${k}</td>
             <td style="padding:8px 0 8px 12px;word-break:break-all;max-width:300px;">${v}</td></tr>`).join("")}
         </table>`
      : '<div class="notice">No storage state.</div>';

    // Events
    $("exp-events").innerHTML = events.length
      ? `<div class="activity-list">${events.slice(0,50).map(ev => `
          <div class="activity-row">
            <div class="act-icon">📡</div>
            <div class="act-info">
              <div class="act-type">${ev.event || ev.name || "Event"}</div>
              <div class="act-hash">${JSON.stringify(ev.data || ev.payload || {})}</div>
            </div>
          </div>`).join("")}</div>`
      : '<div class="notice">No events emitted.</div>';

    // ABI — full DApp-ready view with JSON + JS client + copy buttons
    if (!abi.length) {
      $("exp-abi").innerHTML = '<div class="notice">No ABI available.</div>';
    } else {
      const jsonAbi = JSON.stringify(abi, null, 2);
      const wrappers = abi.map(fn => {
        const params  = (fn.inputs||[]).map((_,i) => `arg${i+1}`).join(", ");
        const hasArgs = (fn.inputs||[]).length > 0;
        return hasArgs
          ? `export const ${fn.name} = (${params}, from, pk) => callWrite("${fn.name}", [${params}], from, pk);`
          : `export const ${fn.name} = (caller="") => callRead("${fn.name}", [], caller);`;
      }).join("\n");

      const jsClient = [
        `// LQD Contract Client — ${addr}`,
        `const CONTRACT_ADDRESS = "${addr}";`,
        `const NODE_URL   = "${PROD_CHAIN_URL}";`,
        `const WALLET_URL = "${PROD_WALLET_URL}";`,
        ``,
        `export const ABI = ${jsonAbi};`,
        ``,
        `export async function callRead(fn, args = [], caller = "") {`,
        `  const res = await fetch(NODE_URL + "/contract/call", {`,
        `    method: "POST", headers: { "Content-Type": "application/json" },`,
        `    body: JSON.stringify({ address: CONTRACT_ADDRESS, fn, args, caller }),`,
        `  });`,
        `  return (await res.json()).output;`,
        `}`,
        ``,
        `export async function callWrite(fn, args = [], fromAddress, privateKey) {`,
        `  const res = await fetch(WALLET_URL + "/wallet/contract-template", {`,
        `    method: "POST", headers: { "Content-Type": "application/json" },`,
        `    body: JSON.stringify({ address: fromAddress, private_key: privateKey,`,
        `      contract_address: CONTRACT_ADDRESS, function: fn, args }),`,
        `  });`,
        `  return res.json();`,
        `}`,
        ``,
        `// Generated wrappers`,
        wrappers,
      ].join("\n");

      // Store in window so onclick can access without escaping hell
      window._lqdAbiJson = jsonAbi;
      window._lqdAbiJs   = jsClient;

      const fnRows = abi.map(fn => {
        const isRead = (fn.inputs||[]).length === 0;
        const badge  = isRead
          ? `style="padding:2px 8px;border-radius:4px;font-size:11px;background:rgba(22,163,74,0.15);color:#4ade80;"`
          : `style="padding:2px 8px;border-radius:4px;font-size:11px;background:rgba(37,99,235,0.15);color:#93c5fd;"`;
        return `<div class="stat-box" style="display:flex;justify-content:space-between;align-items:center;padding:8px 12px;">
          <div>
            <div style="font-weight:700;font-size:13px;font-family:monospace;">${fn.name}</div>
            <div style="font-size:11px;color:var(--muted);margin-top:2px;">${(fn.inputs||[]).join(", ") || "no params"}</div>
          </div>
          <span ${badge}>${isRead ? "read" : "write"}</span>
        </div>`;
      }).join("");

      // Build HTML with NO inline event handlers (CSP blocks onclick="...")
      $("exp-abi").innerHTML = `
        <div style="display:flex;gap:6px;margin-bottom:12px;flex-wrap:wrap;">
          <button id="abiTabJson" style="padding:5px 14px;border-radius:6px;border:1px solid var(--border);background:var(--accent);color:#fff;cursor:pointer;font-size:12px;font-weight:600;">JSON ABI</button>
          <button id="abiTabJs"   style="padding:5px 14px;border-radius:6px;border:1px solid var(--border);background:var(--surface2);color:var(--text);cursor:pointer;font-size:12px;">JS Client</button>
          <button id="abiTabTable" style="padding:5px 14px;border-radius:6px;border:1px solid var(--border);background:var(--surface2);color:var(--text);cursor:pointer;font-size:12px;">Functions</button>
        </div>

        <div id="abiPanelJson">
          <div style="display:flex;justify-content:flex-end;margin-bottom:6px;">
            <button id="copyJsonBtn" style="padding:4px 12px;border-radius:6px;border:1px solid var(--border);background:var(--surface2);color:var(--text);cursor:pointer;font-size:12px;">📋 Copy JSON</button>
          </div>
          <pre id="abiJsonPre" style="background:#1e1e2e;color:#cdd6f4;font-family:monospace;font-size:11px;padding:12px;border-radius:8px;overflow:auto;max-height:420px;white-space:pre;margin:0;"></pre>
          <div style="margin-top:8px;padding:10px;background:rgba(37,99,235,0.1);border:1px solid rgba(37,99,235,0.3);border-radius:8px;font-size:12px;color:#93c5fd;">
            <b>Call from DApp:</b> POST ${PROD_CHAIN_URL}/contract/call<br>
            Body: { "address":"${addr}", "fn":"Name", "args":[], "caller":"0x..." }
          </div>
        </div>

        <div id="abiPanelJs" style="display:none;">
          <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:6px;">
            <span style="font-size:12px;color:var(--muted);">Paste into React / Next.js / Node</span>
            <button id="copyJsBtn" style="padding:4px 12px;border-radius:6px;border:1px solid var(--border);background:var(--surface2);color:var(--text);cursor:pointer;font-size:12px;">📋 Copy JS</button>
          </div>
          <pre id="abiJsPre" style="background:#1e1e2e;color:#cdd6f4;font-family:monospace;font-size:11px;padding:12px;border-radius:8px;overflow:auto;max-height:420px;white-space:pre;margin:0;"></pre>
        </div>

        <div id="abiPanelTable" style="display:none;">
          <div style="display:flex;flex-direction:column;gap:6px;">${fnRows}</div>
        </div>
      `;

      // Populate <pre> elements via textContent — safe, no escaping needed
      document.getElementById("abiJsonPre").textContent = jsonAbi;
      document.getElementById("abiJsPre").textContent   = jsClient;

      // ── Attach all event listeners here (CSP-safe, no inline onclick) ──
      const doAbiCopy = (text, btnId) => {
        navigator.clipboard.writeText(text).catch(() => {
          const ta = document.createElement("textarea");
          ta.value = text; document.body.appendChild(ta);
          ta.select(); document.execCommand("copy");
          document.body.removeChild(ta);
        });
        const btn = document.getElementById(btnId);
        if (!btn) return;
        const orig = btn.textContent;
        btn.textContent = "✅ Copied!";
        setTimeout(() => { btn.textContent = orig; }, 2000);
      };

      const showAbiPanel = (active) => {
        const panels = { json:"abiPanelJson", js:"abiPanelJs", table:"abiPanelTable" };
        const tabs   = { json:"abiTabJson",   js:"abiTabJs",   table:"abiTabTable"   };
        Object.keys(panels).forEach(k => {
          const panel = document.getElementById(panels[k]);
          const tab   = document.getElementById(tabs[k]);
          if (panel) panel.style.display = (k === active) ? "block" : "none";
          if (tab) {
            tab.style.background = (k === active) ? "var(--accent)" : "var(--surface2)";
            tab.style.color      = (k === active) ? "#fff"          : "var(--text)";
          }
        });
      };

      document.getElementById("abiTabJson") .addEventListener("click", () => showAbiPanel("json"));
      document.getElementById("abiTabJs")   .addEventListener("click", () => showAbiPanel("js"));
      document.getElementById("abiTabTable").addEventListener("click", () => showAbiPanel("table"));
      document.getElementById("copyJsonBtn").addEventListener("click", () => doAbiCopy(jsonAbi,   "copyJsonBtn"));
      document.getElementById("copyJsBtn")  .addEventListener("click", () => doAbiCopy(jsClient,  "copyJsBtn"));
    }

    $("explorerResult").style.display = "block";
    // Reset to overview tab
    document.querySelectorAll("#explorerSubtabs .subtab").forEach(b => b.classList.remove("active"));
    document.querySelectorAll("[id^='exp-']").forEach(p => p.style.display = "none");
    document.querySelector("#explorerSubtabs .subtab[data-exp='overview']").classList.add("active");
    $("exp-overview").removeAttribute("style");
    toast("Contract loaded!", "success");
  } catch (e) { toast("Explorer failed: " + e.message, "error"); }
  finally { $("exploreBtn").disabled = false; }
});

// ══════════════════════════════════════════════════════════════════
// BRIDGE PAGE
// ══════════════════════════════════════════════════════════════════
let _bscAccount = "";
let _bridgeTokens = [];

async function loadBridgeTokens() {
  try {
    const data = await nodeGet("/bridge/tokens");
    _bridgeTokens = Array.isArray(data) ? data : [];
  } catch {
    _bridgeTokens = [];
  }
}

function syncBridgeFamilyToUI(familyId) {
  const nextFamily = String(familyId || "evm").toLowerCase();
  bridgeFamilyId = nextFamily;
  const familySelect = $("bridgeFamilySelect");
  const burnFamilySelect = $("burnFamilySelect");
  const adminFamilySelect = $("bridgeAdminFamily");
  const tokenFamilySelect = $("bridgeTokenAdminFamily");
  if (familySelect && familySelect.value !== nextFamily) familySelect.value = nextFamily;
  if (burnFamilySelect && burnFamilySelect.value !== nextFamily) burnFamilySelect.value = nextFamily;
  if (adminFamilySelect && adminFamilySelect.value !== nextFamily) adminFamilySelect.value = nextFamily;
  if (tokenFamilySelect && tokenFamilySelect.value !== nextFamily) tokenFamilySelect.value = nextFamily;
  updateBridgeExternalFieldVisibility(nextFamily);
}

function resolvedBridgeChainFamily(chainId = bridgeChainId) {
  const normalized = String(chainId || "").toLowerCase();
  const selected = bridgeChains.find((cfg) => String(cfg.id || cfg.chain_id || "").toLowerCase() === normalized);
  const family = String(selected?.family || "evm").toLowerCase();
  return family || "evm";
}

function updateBridgeExternalFieldVisibility(familyId) {
  const family = String(familyId || resolvedBridgeChainFamily() || "evm").toLowerCase();
  const isExternal = isExternalBridgeFamily(family);
  const metaCard = $("bridgeExternalMetaCard");
  const evmHint = $("bridgeEvmHint");
  const burnMetaCard = $("burnExternalMetaCard");
  if (metaCard) metaCard.style.display = isExternal ? "block" : "none";
  if (burnMetaCard) burnMetaCard.style.display = isExternal ? "block" : "none";
  if (evmHint) evmHint.style.display = isExternal ? "none" : "block";
  if (!isExternal) {
    ["bridgeSourceTxHash", "bridgeSourceAddress", "bridgeSourceMemo", "bridgeSourceSequence", "bridgeSourceOutput", "burnSourceTxHash", "burnSourceAddress", "burnSourceMemo", "burnSourceSequence", "burnSourceOutput"].forEach((id) => {
      const el = $(id);
      if (el) el.value = "";
    });
  }
}

async function loadBridgeFamilies() {
  try {
    const data = await nodeGet("/bridge/families");
    bridgeFamilies = Array.isArray(data) ? data : [];
  } catch {
    bridgeFamilies = [];
  }
  const familySelect = $("bridgeFamilySelect");
  const burnFamilySelect = $("burnFamilySelect");
  const adminFamilySelect = $("bridgeAdminFamily");
  const tokenFamilySelect = $("bridgeTokenAdminFamily");
  const current = bridgeFamilies.length ? bridgeFamilies : [
    { id: "evm", name: "EVM" },
    { id: "utxo", name: "UTXO" },
    { id: "cosmos", name: "Cosmos" },
    { id: "substrate", name: "Substrate" },
    { id: "solana", name: "Solana" },
    { id: "xrpl", name: "XRPL" },
    { id: "ton", name: "TON" },
    { id: "cardano", name: "Cardano" },
    { id: "aptos", name: "Aptos" },
    { id: "sui", name: "Sui" },
    { id: "near", name: "NEAR" },
    { id: "icp", name: "ICP" },
  ];
  const options = current.map((cfg) => {
    const value = cfg.id || cfg.family || "";
    const label = cfg.name || cfg.id || "Family";
    return `<option value="${value}">${label}</option>`;
  }).join("");
  if (familySelect) {
    familySelect.innerHTML = options || '<option value="evm">EVM</option>';
    if (![...familySelect.options].some((opt) => opt.value === bridgeFamilyId)) {
      bridgeFamilyId = familySelect.options[0]?.value || "evm";
    }
    familySelect.value = bridgeFamilyId;
  }
  if (burnFamilySelect) {
    burnFamilySelect.innerHTML = options || '<option value="evm">EVM</option>';
    if (![...burnFamilySelect.options].some((opt) => opt.value === bridgeFamilyId)) {
      burnFamilySelect.value = burnFamilySelect.options[0]?.value || "evm";
    } else {
      burnFamilySelect.value = bridgeFamilyId;
    }
  }
  if (adminFamilySelect) {
    adminFamilySelect.innerHTML = options || '<option value="evm">EVM</option>';
    adminFamilySelect.value = bridgeFamilyId;
  }
  if (tokenFamilySelect) {
    tokenFamilySelect.innerHTML = options || '<option value="evm">EVM</option>';
    tokenFamilySelect.value = bridgeFamilyId;
  }
}

async function loadBridgeChains() {
  try {
    const data = await nodeGet("/bridge/chains");
    bridgeChains = Array.isArray(data) ? data : [];
  } catch {
    bridgeChains = [];
  }
  const chainSelect = $("bridgeChainSelect");
  const burnSelect = $("burnChainSelect");
  const tokenAdminSelect = $("bridgeTokenAdminChainSelect");
  const current = bridgeChains.length ? bridgeChains : [{ id: "bsc-testnet", name: "BSC Testnet", chain_id: "97", family: "evm" }];
  const familyFilter = String(bridgeFamilyId || $("bridgeFamilySelect")?.value || "evm").toLowerCase();
  const filtered = current.filter((cfg) => !familyFilter || String(cfg.family || "evm").toLowerCase() === String(familyFilter).toLowerCase());
  const visible = filtered.length ? filtered : (familyFilter === "evm" ? current : []);
  const options = current.map((cfg) => {
    const value = cfg.id || cfg.chain_id || "";
    const label = cfg.name || cfg.id || cfg.chain_id || "Chain";
    const fam = cfg.family || "evm";
    const extra = fam ? ` (${fam.toUpperCase()})` : "";
    return `<option value="${value}">${label}${extra}</option>`;
  }).join("");
  if (chainSelect) {
    const chainOptions = visible.length ? visible.map((cfg) => {
      const value = cfg.id || cfg.chain_id || "";
      const label = cfg.name || cfg.id || cfg.chain_id || "Chain";
      const fam = cfg.family || "evm";
      return `<option value="${value}">${label} (${fam.toUpperCase()})</option>`;
    }).join("") : '<option value="">No chains configured for this family</option>';
    chainSelect.innerHTML = chainOptions;
    if (!visible.length) {
      bridgeChainId = "";
    } else if (![...chainSelect.options].some((opt) => opt.value === bridgeChainId)) {
      const optionList = Array.from(chainSelect.options || []);
      bridgeChainId = optionList.find((opt) => opt.value === "bsc-testnet")?.value
        || optionList.find((opt) => /EVM/i.test(opt.textContent || ""))?.value
        || optionList[0]?.value
        || "";
    }
    chainSelect.value = bridgeChainId;
  }
  if (burnSelect) {
    const chainOptions = visible.length ? visible.map((cfg) => {
      const value = cfg.id || cfg.chain_id || "";
      const label = cfg.name || cfg.id || cfg.chain_id || "Chain";
      const fam = cfg.family || "evm";
      return `<option value="${value}">${label} (${fam.toUpperCase()})</option>`;
    }).join("") : '<option value="">No chains configured for this family</option>';
    burnSelect.innerHTML = chainOptions;
    if (!visible.length) {
      bridgeChainId = "";
    } else if (![...burnSelect.options].some((opt) => opt.value === bridgeChainId)) {
      const optionList = Array.from(burnSelect.options || []);
      burnSelect.value = optionList.find((opt) => opt.value === "bsc-testnet")?.value
        || optionList.find((opt) => /EVM/i.test(opt.textContent || ""))?.value
        || optionList[0]?.value
        || "";
    } else {
      burnSelect.value = bridgeChainId;
    }
  }
  if (tokenAdminSelect) {
    const chainOptions = visible.length ? visible.map((cfg) => {
      const value = cfg.id || cfg.chain_id || "";
      const label = cfg.name || cfg.id || cfg.chain_id || "Chain";
      const fam = cfg.family || "evm";
      return `<option value="${value}">${label} (${fam.toUpperCase()})</option>`;
    }).join("") : '<option value="">No chains configured for this family</option>';
    tokenAdminSelect.innerHTML = chainOptions;
    tokenAdminSelect.value = bridgeChainId;
  }
  updateBridgeExternalFieldVisibility(bridgeFamilyId || resolvedBridgeChainFamily());
  renderBridgeChainList();
}

function renderBridgeChainList() {
  const el = $("bridgeChainList");
  if (!el) return;
  if (!bridgeChains.length) {
    el.innerHTML = "<div class='notice'>No bridge chains configured yet.</div>";
    return;
  }
  el.innerHTML = bridgeChains.map((cfg) => {
    const id = cfg.id || cfg.chain_id || "";
    const name = cfg.name || id || "Chain";
    const family = cfg.family || "evm";
    const adapter = cfg.adapter || family;
    return `<div style="padding:8px 0;border-bottom:1px solid var(--surface2);">
      <strong>${name}</strong> <span style="opacity:.7">${id}</span><br/>
      <span style="opacity:.7">Family:</span> ${family.toUpperCase()} · <span style="opacity:.7">Adapter:</span> ${adapter}<br/>
      <span style="opacity:.7">RPC:</span> ${cfg.rpc || (cfg.rpcs || []).join(", ") || "—"}<br/>
      <span style="opacity:.7">Bridge:</span> ${cfg.bridge_address || "—"}<br/>
      <span style="opacity:.7">Lock:</span> ${cfg.lock_address || "—"}<br/>
      <span style="opacity:.7">Public/Private:</span> ${cfg.supports_public ? "public " : ""}${cfg.supports_private ? "private" : ""}
    </div>`;
  }).join("");
}

function bridgeTokenAddressForSelection() {
  const symbol = $("bridgeTokenSelect")?.value?.trim();
  const custom = $("bridgeTokenCustomAddr")?.value?.trim();
  if (custom && /^0x[a-fA-F0-9]{40}$/.test(custom)) return custom;
  const mapped = _bridgeTokens.find((t) => {
    const chainMatch = !bridgeChainId || !t.chain_id || String(t.chain_id).toLowerCase() === String(bridgeChainId).toLowerCase();
    const symbolMatch = String(t.symbol || "").toUpperCase() === String(symbol || "").toUpperCase();
    return chainMatch && symbolMatch;
  }) || _bridgeTokens.find((t) => String(t.symbol || "").toUpperCase() === String(symbol || "").toUpperCase());
  if (mapped?.bsc_token && /^0x[a-fA-F0-9]{40}$/.test(mapped.bsc_token)) return mapped.bsc_token;
  if (mapped?.source_token && /^0x[a-fA-F0-9]{40}$/.test(mapped.source_token)) return mapped.source_token;
  return "";
}

function bridgeMetadataFromUI(prefix = "bridge") {
  const get = (id) => $(id)?.value?.trim() || "";
  return {
    source_tx_hash: get(`${prefix}SourceTxHash`),
    source_address: get(`${prefix}SourceAddress`),
    source_memo: get(`${prefix}SourceMemo`),
    source_sequence: get(`${prefix}SourceSequence`),
    source_output: get(`${prefix}SourceOutput`),
  };
}

function isExternalBridgeFamily(familyId) {
  const family = String(familyId || bridgeFamilyId || "evm").toLowerCase();
  return family === "cosmos" || family === "utxo" || family === "cardano" || family === "solana" || family === "substrate" || family === "xrpl" || family === "ton" || family === "near" || family === "aptos";
}

async function useExtensionWalletSigner() {
  if (!state.address) {
    $("bscStatus").textContent = "Unlock your LQD wallet first";
    $("bscStatus").className = "notice warn";
    toast("Unlock wallet first", "error");
    return;
  }
  _bscAccount = state.address;
  $("bscStatus").textContent = `✓ Extension wallet signer ready: ${shortAddr(_bscAccount)}`;
  $("bscStatus").className = "notice success";
  toast("Extension wallet signer ready", "success");
}

$("useWalletSignerBtn").addEventListener("click", async () => {
  await useExtensionWalletSigner();
});

$("doBridgeLockBtn").addEventListener("click", async () => {
  const family = String(bridgeFamilyId || $("bridgeFamilySelect")?.value || "evm").toLowerCase();
  const token = bridgeTokenAddressForSelection();
  const amt    = $("bridgeLockAmt").value.trim();
  const lqdRecipient = $("bridgeLqdRecipient").value.trim() || state.address;
  bridgeMode = $("bridgeModeSelect")?.value || "public";
  bridgeChainId = $("bridgeChainSelect")?.value || bridgeChainId;
  syncBridgeFamilyToUI(family);
  if (!token) { toast("Select a mapped token or enter a custom BSC token address", "error"); return; }
  if (!amt) { toast("Enter amount", "error"); return; }
  try {
    if (isExternalBridgeFamily(family)) {
      const meta = bridgeMetadataFromUI("bridge");
      if (!meta.source_tx_hash || !meta.source_address) {
        toast("Enter source tx hash and source address for this non-EVM bridge", "error");
        return;
      }
      if (family === "cosmos" && !meta.source_memo) {
        toast("Cosmos bridge needs a memo/note", "error");
        return;
      }
      if (family === "utxo" && !meta.source_output) {
        toast("UTXO bridge needs a source output index", "error");
        return;
      }
      if (family === "cardano" && !meta.source_output) {
        toast("Cardano bridge needs a source output index", "error");
        return;
      }
      if (family === "solana" && !meta.source_sequence) {
        toast("Solana bridge needs a recent blockhash / sequence", "error");
        return;
      }
      if (family === "substrate" && !meta.source_sequence) {
        toast("Substrate bridge needs a nonce / runtime sequence", "error");
        return;
      }
      if (family === "xrpl" && !meta.source_sequence) {
        toast("XRPL bridge needs a ledger sequence / delivery sequence", "error");
        return;
      }
      if (family === "ton" && !meta.source_sequence) {
        toast("TON bridge needs a message sequence / logical time", "error");
        return;
      }
      if (family === "near" && !meta.source_sequence) {
        toast("NEAR bridge needs an access key nonce / sequence", "error");
        return;
      }
      if (family === "aptos" && !meta.source_sequence) {
        toast("Aptos bridge needs a sequence number", "error");
        return;
      }
      const res = await nodePost("/bridge/lock_chain", {
        chain_id: bridgeChainId,
        family,
        adapter: family,
        tx_hash: meta.source_tx_hash,
        source_tx_hash: meta.source_tx_hash,
        source_address: meta.source_address,
        source_memo: meta.source_memo,
        source_sequence: meta.source_sequence,
        source_output: meta.source_output,
        lqd_recipient: lqdRecipient,
        token,
        amount: amt,
        mode: bridgeMode,
      });
      showResult("bridgeLockResult", `✓ External lock registered\nSource Tx: ${meta.source_tx_hash}\nStatus: ${res?.status || "ok"}`);
      toast("External source lock registered!", "success");
    } else {
      if (!state.address) { toast("Unlock your LQD wallet first", "error"); return; }
      const res = await walletPost(bridgeMode === "private" ? "/wallet/bridge/private/lock_bsc_token" : "/wallet/bridge/lock_bsc_token", {
        address: state.address,
        private_key: await getPrivateKey(),
        token,
        amount: amt,
        to_lqd: lqdRecipient,
        chain_id: bridgeChainId,
        family,
        mode: bridgeMode
      });
      const txHash = res?.tx_hash || res?.hash || "";
      showResult("bridgeLockResult", `✓ BSC Tx: ${txHash}\nMinting will happen after confirmation.`);
      toast("Bridge lock initiated using extension wallet!", "success");
      await nodePost("/bridge/lock_chain", {
        chain_id: bridgeChainId,
        family,
        adapter: family,
        tx_hash: txHash, lqd_recipient: lqdRecipient, token, amount: amt, mode: bridgeMode
      });
    }
    setTimeout(loadBridgeHistory, 5000);
  } catch (e) { showResult("bridgeLockResult", "✗ " + e.message, true); }
});

$("doBurnBtn").addEventListener("click", async () => {
  bridgeMode = $("bridgeModeSelect")?.value || "public";
  bridgeChainId = $("burnChainSelect")?.value || bridgeChainId;
  syncBridgeFamilyToUI(bridgeFamilyId);
  const tokenAddr = $("burnTokenAddr").value.trim();
  const amt       = $("burnAmt").value.trim();
  const bscRecipient = $("burnBscRecipient").value.trim();
  if (!tokenAddr || !amt || !bscRecipient) { toast("Fill all fields", "error"); return; }
  try {
    const meta = bridgeMetadataFromUI("burn");
    const res = await walletPost(bridgeMode === "private" ? "/wallet/bridge/private/burn_lqd_token" : "/wallet/bridge/burn_lqd_token", {
      address: state.address, private_key: await getPrivateKey(),
      token_address: tokenAddr, amount: amt, bsc_recipient: bscRecipient, chain_id: bridgeChainId, family: bridgeFamilyId, mode: bridgeMode,
      source_tx_hash: meta.source_tx_hash,
      source_address: meta.source_address,
      source_memo: meta.source_memo,
      source_sequence: meta.source_sequence,
      source_output: meta.source_output,
    });
    showResult("burnResult", `✓ Burn Tx: ${res.tx_hash || ""}\nTokens will be unlocked on BSC.`);
    toast("Bridge burn initiated!", "success");
    setTimeout(loadBridgeHistory, 5000);
  } catch (e) { showResult("burnResult", "✗ " + e.message, true); }
});

$("refreshBridgeBtn").addEventListener("click", loadBridgeHistory);
async function loadBridgeHistory() {
  try {
    bridgeMode = $("bridgeModeSelect")?.value || bridgeMode;
    const data = await nodeGet(`/bridge/requests?mode=${encodeURIComponent(bridgeMode)}`);
    const list = Array.isArray(data) ? data : (data.requests || []);
    if (!list.length) { $("bridgeHistory").innerHTML = '<div class="notice">No bridge requests found.</div>'; return; }
  $("bridgeHistory").innerHTML = `
              <div class="activity-list">
        ${list.slice(0,20).map(r => `
          <div class="activity-row">
            <div class="act-icon">${r.direction === "bsc_to_lqd" ? "→" : "←"}</div>
            <div class="act-info">
              <div class="act-type">${r.direction === "bsc_to_lqd" ? "BSC → LQD" : "LQD → BSC"} · ${r.token || ""} · ${r.mode || bridgeMode} · ${(r.family || "evm").toUpperCase()}</div>
              <div class="act-hash">Amount: ${r.amount || "?"} · Status: ${r.status || "pending"}${r.source_tx_hash ? ` · SourceTx: ${r.source_tx_hash}` : ""}</div>
            </div>
          </div>`).join("")}
      </div>`;
  } catch { $("bridgeHistory").innerHTML = '<div class="notice">Could not load bridge history.</div>'; }
}

async function refreshBridgeChainsAdmin() {
  await loadBridgeChains();
}

async function saveBridgeChainAdmin() {
  const apiKey = $("bridgeAdminApiKey")?.value?.trim() || "";
  const id = $("bridgeAdminChainId")?.value?.trim() || "";
  const name = $("bridgeAdminChainName")?.value?.trim() || id;
  const family = $("bridgeAdminFamily")?.value?.trim() || "evm";
  const adapter = $("bridgeAdminAdapter")?.value?.trim() || family;
  const rpc = $("bridgeAdminChainRpc")?.value?.trim() || "";
  const bridgeAddress = $("bridgeAdminBridgeAddr")?.value?.trim() || "";
  const lockAddress = $("bridgeAdminLockAddr")?.value?.trim() || "";
  const chainId = $("bridgeAdminChainIdNumber")?.value?.trim() || "";
  if (!id || !name || !family || !adapter) {
    toast("Fill chain id, name, family and adapter", "error");
    return;
  }
  if (family === "evm" && (!rpc || !bridgeAddress)) {
    toast("EVM chains need rpc and bridge address", "error");
    return;
  }
  try {
    const res = await fetch(`${state.nodeUrl}/bridge/chain`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...(apiKey ? { "X-API-Key": apiKey } : {}),
      },
      body: JSON.stringify({
        id,
        name,
        chain_id: chainId || id,
        family,
        adapter,
        rpc,
        bridge_address: bridgeAddress,
        lock_address: lockAddress || bridgeAddress,
        native_symbol: "LQD",
        enabled: true,
        supports_public: true,
        supports_private: true,
      }),
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
    toast(`Bridge chain saved: ${name}`, "success");
    await loadBridgeChains();
  } catch (err) {
    toast(err?.message || "Failed to save chain", "error");
  }
}

async function saveBridgeTokenAdmin() {
  const apiKey = $("bridgeTokenAdminApiKey")?.value?.trim() || "";
  const chainId = $("bridgeTokenAdminChainSelect")?.value?.trim() || bridgeChainId;
  const family = $("bridgeTokenAdminFamily")?.value?.trim() || bridgeFamilyId || "evm";
  const sourceToken = $("bridgeTokenAdminSource")?.value?.trim() || "";
  const lqdToken = $("bridgeTokenAdminLqd")?.value?.trim() || "";
  const name = $("bridgeTokenAdminName")?.value?.trim() || "";
  const symbol = $("bridgeTokenAdminSymbol")?.value?.trim() || "";
  const decimals = $("bridgeTokenAdminDecimals")?.value?.trim() || "";
  if (!chainId || !sourceToken || !lqdToken) {
    toast("Fill chain, source token and LQD token", "error");
    return;
  }
  try {
    const res = await fetch(`${state.nodeUrl}/bridge/token`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...(apiKey ? { "X-API-Key": apiKey } : {}),
      },
      body: JSON.stringify({
        chain_id: chainId,
        family,
        chain_name: chainId,
        source_token: sourceToken,
        target_chain_id: "lqd",
        target_chain_name: "LQD",
        target_token: lqdToken,
        name,
        symbol,
        decimals,
        bsc_token: sourceToken,
        lqd_token: lqdToken,
      }),
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
    toast("Bridge token saved", "success");
    await loadBridgeTokens();
  } catch (err) {
    toast(err?.message || "Failed to save token", "error");
  }
}

async function removeBridgeTokenAdmin() {
  const apiKey = $("bridgeTokenAdminApiKey")?.value?.trim() || "";
  const chainId = $("bridgeTokenAdminChainSelect")?.value?.trim() || bridgeChainId;
  const sourceToken = $("bridgeTokenAdminSource")?.value?.trim() || "";
  const lqdToken = $("bridgeTokenAdminLqd")?.value?.trim() || "";
  if (!chainId || (!sourceToken && !lqdToken)) {
    toast("Fill chain and source or LQD token", "error");
    return;
  }
  try {
    const res = await fetch(`${state.nodeUrl}/bridge/token/remove`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...(apiKey ? { "X-API-Key": apiKey } : {}),
      },
      body: JSON.stringify({
        chain_id: chainId,
        source_token: sourceToken,
        lqd_token: lqdToken,
      }),
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
    toast("Bridge token removed", "success");
    await loadBridgeTokens();
  } catch (err) {
    toast(err?.message || "Failed to remove token", "error");
  }
}

async function removeBridgeChainAdmin() {
  const apiKey = $("bridgeAdminApiKey")?.value?.trim() || "";
  const id = $("bridgeAdminChainId")?.value?.trim() || "";
  if (!id) {
    toast("Enter chain id", "error");
    return;
  }
  try {
    const res = await fetch(`${state.nodeUrl}/bridge/chain/remove`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...(apiKey ? { "X-API-Key": apiKey } : {}),
      },
      body: JSON.stringify({ id }),
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
    toast(`Bridge chain removed: ${id}`, "success");
    await loadBridgeChains();
  } catch (err) {
    toast(err?.message || "Failed to remove chain", "error");
  }
}

const bridgeModeSelect = $("bridgeModeSelect");
if (bridgeModeSelect) {
  bridgeModeSelect.addEventListener("change", () => {
    bridgeMode = bridgeModeSelect.value || "public";
    loadBridgeHistory();
  });
}

const bridgeFamilySelect = $("bridgeFamilySelect");
if (bridgeFamilySelect) {
  bridgeFamilySelect.addEventListener("change", async () => {
    syncBridgeFamilyToUI(bridgeFamilySelect.value || "evm");
    await loadBridgeChains();
  });
}

const bridgeChainSelect = $("bridgeChainSelect");
if (bridgeChainSelect) {
  bridgeChainSelect.addEventListener("change", () => {
    bridgeChainId = bridgeChainSelect.value || bridgeChainId;
    const selected = bridgeChains.find((cfg) => String(cfg.id || cfg.chain_id || "").toLowerCase() === String(bridgeChainId || "").toLowerCase());
    syncBridgeFamilyToUI(selected?.family || "evm");
  });
}

const burnChainSelect = $("burnChainSelect");
if (burnChainSelect) {
  burnChainSelect.addEventListener("change", () => {
    bridgeChainId = burnChainSelect.value || bridgeChainId;
    const selected = bridgeChains.find((cfg) => String(cfg.id || cfg.chain_id || "").toLowerCase() === String(bridgeChainId || "").toLowerCase());
    syncBridgeFamilyToUI(selected?.family || "evm");
  });
}

const burnFamilySelect = $("burnFamilySelect");
if (burnFamilySelect) {
  burnFamilySelect.addEventListener("change", async () => {
    syncBridgeFamilyToUI(burnFamilySelect.value || "evm");
    await loadBridgeChains();
  });
}

const bridgeAdminFamilySelect = $("bridgeAdminFamily");
if (bridgeAdminFamilySelect) {
  bridgeAdminFamilySelect.addEventListener("change", () => {
    syncBridgeFamilyToUI(bridgeAdminFamilySelect.value || "evm");
  });
}

const bridgeTokenAdminFamilySelect = $("bridgeTokenAdminFamily");
if (bridgeTokenAdminFamilySelect) {
  bridgeTokenAdminFamilySelect.addEventListener("change", async () => {
    syncBridgeFamilyToUI(bridgeTokenAdminFamilySelect.value || "evm");
    await loadBridgeChains();
  });
}

const bridgeAdminSaveBtn = $("bridgeAdminSaveBtn");
if (bridgeAdminSaveBtn) bridgeAdminSaveBtn.addEventListener("click", saveBridgeChainAdmin);
const bridgeAdminRemoveBtn = $("bridgeAdminRemoveBtn");
if (bridgeAdminRemoveBtn) bridgeAdminRemoveBtn.addEventListener("click", removeBridgeChainAdmin);
const bridgeAdminRefreshBtn = $("bridgeAdminRefreshBtn");
if (bridgeAdminRefreshBtn) bridgeAdminRefreshBtn.addEventListener("click", refreshBridgeChainsAdmin);
const bridgeTokenAdminSaveBtn = $("bridgeTokenAdminSaveBtn");
if (bridgeTokenAdminSaveBtn) bridgeTokenAdminSaveBtn.addEventListener("click", saveBridgeTokenAdmin);
const bridgeTokenAdminRemoveBtn = $("bridgeTokenAdminRemoveBtn");
if (bridgeTokenAdminRemoveBtn) bridgeTokenAdminRemoveBtn.addEventListener("click", removeBridgeTokenAdmin);

// ══════════════════════════════════════════════════════════════════
// ACTIVITY PAGE
// ══════════════════════════════════════════════════════════════════
async function recordLocalActivity(entry) {
  const key = `lqd_activity_${state.address}`;
  let acts = [];
  try { acts = JSON.parse(localStorage.getItem(key) || "[]"); } catch {}
  acts.unshift({ ...entry, time: Date.now() });
  if (acts.length > 200) acts = acts.slice(0, 200);
  localStorage.setItem(key, JSON.stringify(acts));
}

function loadActivity() {
  const key = `lqd_activity_${state.address}`;
  let acts = [];
  try { acts = JSON.parse(localStorage.getItem(key) || "[]"); } catch {}

  // Also pull from extension background activity
  msg("LQD_GET_ACTIVITY").then(res => {
    const bgActs = res?.list || [];
    const all = [...bgActs, ...acts].sort((a,b) => (b.time||0) - (a.time||0)).slice(0, 200);
    renderActivity(all);
  }).catch(() => renderActivity(acts));
}

function renderActivity(acts) {
  const container = $("activityList");
  if (!acts.length) { container.innerHTML = '<div class="notice">No activity yet.</div>'; return; }
  const icons = { send:"↑", receive:"↓", token:"🪙", contract:"📜", deploy:"🚀", bridge:"🌉" };
  container.innerHTML = `<div class="activity-list">
    ${acts.map(a => `
      <div class="activity-row">
        <div class="act-icon">${icons[a.type] || "•"}</div>
        <div class="act-info">
          <div class="act-type">${(a.type||"tx").toUpperCase()}${a.function ? " · " + a.function : ""}${a.contractType ? " · " + a.contractType : ""}</div>
          <div class="act-hash">${a.tx_hash ? "Tx: " + a.tx_hash : (a.contract ? "Contract: " + a.contract : (a.to ? "To: " + a.to : ""))}</div>
          <div class="act-time">${a.time ? new Date(a.time).toLocaleString() : ""}</div>
        </div>
        ${a.value ? `<div class="act-amount">${fmtAmount(a.value)} LQD</div>` : ""}
      </div>`).join("")}
  </div>`;
}

$("refreshActivityBtn").addEventListener("click", loadActivity);

// ══════════════════════════════════════════════════════════════════
// SETTINGS PAGE
// ══════════════════════════════════════════════════════════════════
$("saveEndpointsBtn").addEventListener("click", async () => {
  state.nodeUrl   = $("settingsNodeUrl").value.trim()  || state.nodeUrl;
  state.walletUrl = $("settingsWalletUrl").value.trim() || state.walletUrl;
  await ext.storage.local.set({ nodeUrl: state.nodeUrl, walletUrl: state.walletUrl });
  toast("Endpoints saved!", "success");
});

// Reveal secrets
$("revealKeyBtn").addEventListener("click", () => revealSecret("key"));
$("revealSeedBtn").addEventListener("click", () => revealSecret("mnemonic"));
async function revealSecret(kind) {
  const pass = $("revealPass").value;
  if (!pass) { toast("Enter your password first", "error"); return; }
  const res = await msg("LQD_REVEAL_SECRET", { kind, password: pass });
  if (res?.ok) {
    $("secretBox").style.display = "block";
    $("secretBox").textContent = res.secret;
    setTimeout(() => { $("secretBox").style.display = "none"; $("secretBox").textContent = ""; }, 30000);
    toast("Shown for 30 seconds — keep it safe!", "info");
  } else {
    toast(res?.error || "Wrong password", "error");
  }
}

// Lock wallet
$("lockWalletBtn").addEventListener("click", async () => {
  await msg("LQD_LOCK");
  showLockScreen();
  toast("Wallet locked", "info");
});

// Network list
async function loadNetworkList() {
  const res = await msg("LQD_GET_NETWORKS");
  if (!res?.ok) return;
  const { networks, currentNetwork } = res;
  const container = $("networkList");
  container.innerHTML = Object.entries(networks).map(([chainId, net]) => `
    <div style="display:flex;align-items:center;justify-content:space-between;padding:10px;background:var(--card2);border:1px solid ${chainId===currentNetwork?"var(--accent)":"var(--border)"};border-radius:var(--rs);margin-bottom:8px;">
      <div>
        <div style="font-weight:600;">${net.name} ${chainId===currentNetwork?"<span style='color:var(--green);font-size:11px;'>● Active</span>":""}</div>
        <div style="font-size:11px;color:var(--muted);">Chain: ${chainId} · ${net.nodeUrl||""}</div>
      </div>
      ${chainId!==currentNetwork ? `<button class="btn btn-secondary btn-sm switchNetBtn" data-chain="${chainId}">Switch</button>` : ""}
    </div>`).join("");

  container.querySelectorAll(".switchNetBtn").forEach(b => {
    b.addEventListener("click", async () => {
      await msg("LQD_SWITCH_NETWORK", { chainId: b.dataset.chain });
      const nets = (await msg("LQD_GET_NETWORKS"))?.networks || {};
      const net = nets[b.dataset.chain];
      if (net) { state.nodeUrl = net.nodeUrl; state.walletUrl = net.walletUrl; }
      loadNetworkList();
      toast("Network switched!", "success");
    });
  });
}

$("addNetworkBtn").addEventListener("click", async () => {
  const net = {
    name:      $("newNetName").value.trim(),
    chainId:   $("newNetChain").value.trim(),
    nodeUrl:   $("newNetNode").value.trim(),
    walletUrl: $("newNetWallet").value.trim(),
  };
  if (!net.name || !net.chainId || !net.nodeUrl) { toast("Fill required fields", "error"); return; }
  const res = await msg("LQD_ADD_NETWORK", { network: net });
  if (res?.ok) { loadNetworkList(); toast("Network added!", "success"); }
  else toast(res?.error || "Failed", "error");
});

// ── Handle background getPrivateKey ──────────────────────────────
// background.js doesn't have lqd_getPrivateKey — we pull from session
// Override getPrivateKey to use stored session directly
(async () => {
  // Intercept: background keeps private key in memory while unlocked
  // We send a custom message to get it
  const origGetPK = getPrivateKey;
  window._getPrivateKey = async function() {
    const res = await msg("LQD_REQUEST", { payload: { id: "fp-pk-" + Date.now(), method: "lqd_getSessionKey" } });
    if (res?.result) return res.result;
    // If not supported, ask user inline
    const pass = prompt("Enter wallet password to authorize transaction:");
    if (!pass) throw new Error("Transaction cancelled");
    const unlockRes = await msg("LQD_UNLOCK", { password: pass });
    if (!unlockRes?.ok) throw new Error("Wrong password");
    const res2 = await msg("LQD_REQUEST", { payload: { id: "fp-pk2-" + Date.now(), method: "lqd_getSessionKey" } });
    return res2?.result || "";
  };
})();

// Patch getPrivateKey to try background session key
async function getPrivateKey() {
  try {
    // Try to get from background session
    const res = await msg("LQD_REQUEST", { payload: { id: "fp-pk-" + Date.now(), method: "lqd_accounts" } });
    // If session is unlocked, get key via reveal secret approach
    // Actually: ask user for password then decrypt
    const data = await ext.storage.local.get(["walletCipher","walletIv","walletSalt"]);
    if (!data.walletCipher) throw new Error("No wallet stored");
    // Use the background reveal mechanism
    const pass = prompt("Enter your wallet password to sign this transaction:");
    if (!pass) throw new Error("Transaction cancelled by user");
    const revRes = await msg("LQD_REVEAL_SECRET", { kind: "key", password: pass });
    if (!revRes?.ok) throw new Error(revRes?.error || "Wrong password");
    return revRes.secret;
  } catch (e) {
    throw e;
  }
}

// ── Boot ──────────────────────────────────────────────────────────
init();
