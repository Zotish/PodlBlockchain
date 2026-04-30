const ext = typeof chrome !== "undefined" ? chrome : browser;
const PROD_CHAIN_URL = "https://dazzling-peace-production-3529.up.railway.app";
const PROD_WALLET_URL = "https://enchanting-hope-production-1c63.up.railway.app";
const PROD_AGGREGATOR_URL = "https://keen-enjoyment-production-0440.up.railway.app";
const PROD_BRIDGE_ADMIN_URL = "https://delightful-churros-767ded.netlify.app";
const PROD_EXPLORER_URL = "https://warm-dragon-34d6ff.netlify.app";
const PROD_DEX_URL = "https://bright-crisp-91fe94.netlify.app";
const OFFICIAL_DAPPS = [
  {
    name: "LQD DEX",
    description: "Swap & provide liquidity on PosDL chain",
    icon: "⇄",
    iconBg: "linear-gradient(135deg,#0f172a,#7c3aed)",
    url: PROD_DEX_URL,
    category: "DeFi",
  },
  {
    name: "Block Explorer",
    description: "Browse blocks, transactions & addresses",
    icon: "🔍",
    iconBg: "linear-gradient(135deg,#0c4a6e,#0369a1)",
    url: PROD_EXPLORER_URL,
    category: "Tools",
  },
  {
    name: "LQD Bridge",
    description: "Bridge assets between LQD and BSC",
    icon: "⬡",
    iconBg: "linear-gradient(135deg,#064e3b,#059669)",
    url: PROD_BRIDGE_ADMIN_URL,
    category: "Bridge",
  },
  {
    name: "Liquidity Pools",
    description: "Manage LP positions & earn rewards",
    icon: "💧",
    iconBg: "linear-gradient(135deg,#1e3a5f,#2563eb)",
    url: `${PROD_DEX_URL}/pools`,
    category: "DeFi",
  },
  {
    name: "Validators",
    description: "View validators & staking statistics",
    icon: "✦",
    iconBg: "linear-gradient(135deg,#3b0764,#7e22ce)",
    url: `${PROD_EXPLORER_URL}/validators`,
    category: "Staking",
  },
];
// ── Screens & DOM refs ────────────────────────────────────────────────────────
const screens = {
  onboarding: document.getElementById("onboarding"),
  create: document.getElementById("create"),
  import: document.getElementById("import"),
  main: document.getElementById("main"),
  settings: document.getElementById("settings"),
  networks: document.getElementById("networks"),
  locked: document.getElementById("locked")
};

function showScreen(name) {
  Object.values(screens).forEach((el) => {
    el.classList.add("hidden");
    el.classList.remove("active");
  });
  const s = screens[name];
  if (s) { s.classList.remove("hidden"); s.classList.add("active"); }
}

// ── Helpers ───────────────────────────────────────────────────────────────────
function on(id, event, handler) {
  const el = typeof id === "string" ? document.getElementById(id) : id;
  if (el) el.addEventListener(event, handler);
}

function setError(msg) {
  const el = document.getElementById("error");
  if (!el) return;
  if (msg) {
    el.textContent = msg;
    el.classList.remove("hidden");
    setTimeout(() => el.classList.add("hidden"), 5000);
  } else {
    el.classList.add("hidden");
  }
}

function setStatus(locked, address) {
  const statusEl = document.getElementById("status");
  const addrEl = document.getElementById("address");
  if (statusEl) {
    statusEl.textContent = locked ? "Locked" : "Unlocked";
    statusEl.className = locked ? "status" : "status ok";
  }
  if (addrEl) addrEl.textContent = address ? truncate(address, 12) : "";
}

function truncate(str, n) {
  if (!str) return "";
  if (str.length <= n * 2) return str;
  return str.slice(0, n) + "…" + str.slice(-6);
}

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

function isLocalEndpoint(url = "") {
  return /^(https?:\/\/)?(localhost|127\.0\.0\.1)(:\d+)?/i.test(String(url).trim());
}

async function getNodeUrl() {
  const data = await ext.storage.local.get(["nodeUrl"]);
  let url = (data.nodeUrl || PROD_CHAIN_URL).replace(/\/$/, "");
  if (isLocalEndpoint(url)) {
    url = PROD_CHAIN_URL;
    await ext.storage.local.set({ nodeUrl: url });
  }
  return url;
}

async function getWalletUrl() {
  const data = await ext.storage.local.get(["walletUrl"]);
  let url = (data.walletUrl || PROD_WALLET_URL).replace(/\/$/, "");
  if (
    url.includes(":8080") ||
    url.includes("127.0.0.1") ||
    url.includes("localhost")
  ) {
    url = PROD_WALLET_URL;
    await ext.storage.local.set({ walletUrl: url });
  }
  return url;
}

// ── Confirmation modal ────────────────────────────────────────────────────────
let confirmResolve = null;

function showConfirmModal({ title, details, fee, origin }) {
  return new Promise((resolve) => {
    confirmResolve = resolve;
    const titleEl = document.getElementById("confirmTitle");
    const detailsEl = document.getElementById("confirmDetails");
    const feeEl = document.getElementById("confirmFeeEl");
    const originEl = document.getElementById("confirmOriginEl");
    if (titleEl) titleEl.textContent = title || "Confirm Transaction";
    if (detailsEl) detailsEl.innerHTML = details || "";
    if (feeEl) feeEl.textContent = fee ? `Estimated fee: ${fee} LQD` : "";
    if (originEl) originEl.textContent = origin || "";
    const rememberEl = document.getElementById("confirmRemember");
    if (rememberEl) rememberEl.checked = false;
    document.getElementById("confirmModal").classList.remove("hidden");
  });
}

on("confirmApproveBtn", "click", () => {
  if (!confirmResolve) return;
  const remember = document.getElementById("confirmRemember")?.checked || false;
  confirmResolve({ confirmed: true, remember });
  confirmResolve = null;
  document.getElementById("confirmModal").classList.add("hidden");
});
on("confirmRejectBtn", "click", () => {
  if (!confirmResolve) return;
  confirmResolve({ confirmed: false });
  confirmResolve = null;
  document.getElementById("confirmModal").classList.add("hidden");
});

// ── Balance ───────────────────────────────────────────────────────────────────
async function refreshBalance() {
  const data = await ext.storage.local.get(["address", "nodeUrl"]);
  if (!data.address) return;
  try {
    let url = (data.nodeUrl || PROD_CHAIN_URL).replace(/\/$/, "");
    if (isLocalEndpoint(url)) url = PROD_CHAIN_URL;
    const res = await fetch(`${url}/balance?address=${encodeURIComponent(data.address)}`);
    const json = await res.json();
    const raw = json.balance || json.Balance || "0";
    const el = document.getElementById("lqdBalance");
    if (el) el.textContent = `${formatLQD(raw)} LQD`;
  } catch (e) {
    const el = document.getElementById("lqdBalance");
    if (el) el.textContent = "Error";
  }
}

// ── Gas / fee estimation ──────────────────────────────────────────────────────
async function fetchBaseFee() {
  try {
    const url = await getNodeUrl();
    const res = await fetch(`${url}/basefee`);
    if (!res.ok) return 10;
    const json = await res.json();
    return Number(json.base_fee || json.baseFee || json.BaseFee || 10) || 10;
  } catch { return 10; }
}

async function estimateFee(gasUnits) {
  const bf = await fetchBaseFee();
  return gasUnits * bf;
}

function parseErr(err) {
  if (!err) return "Unknown error";
  let msg = typeof err === "string" ? err : (err.error || err.message || err.Error || JSON.stringify(err));
  // Convert raw-unit blockchain error to human-readable LQD values
  msg = msg.replace(/balance=(\d+)/g, (_, n) => `balance=${formatLQD(n)} LQD`);
  msg = msg.replace(/required=(\d+)/g, (_, n) => `required=${formatLQD(n)} LQD`);
  msg = msg.replace(/value (\d+)/g, (_, n) => `value ${formatLQD(n)} LQD`);
  msg = msg.replace(/fee (\d+)/g, (_, n) => `fee ${formatLQD(n)} LQD`);
  return msg;
}

// ── Network selector ──────────────────────────────────────────────────────────
async function loadNetworkName() {
  ext.runtime.sendMessage({ type: "LQD_GET_NETWORKS" }, (res) => {
    if (!res || !res.ok) return;
    const net = res.networks[res.currentNetwork];
    const el = document.getElementById("networkName");
    if (el) el.textContent = net ? net.name : res.currentNetwork;
  });
}

on("networkBtn", "click", () => showScreen("networks"));

// ── Network management screen ─────────────────────────────────────────────────
function renderNetworkList(networks, currentNetwork) {
  const container = document.getElementById("networkList");
  if (!container) return;
  container.innerHTML = "";
  for (const [chainId, net] of Object.entries(networks)) {
    const div = document.createElement("div");
    div.className = "item";
    const isCurrent = chainId === currentNetwork;
    div.innerHTML = `
      <div class="net-row">
        <div>
          <strong>${net.name}</strong>${isCurrent ? ' <span class="badge-active">Active</span>' : ""}
          <div class="mono">${net.nodeUrl}</div>
          <div class="mono">Chain: ${chainId} | Symbol: ${net.symbol || "LQD"}</div>
        </div>
      </div>`;
    if (!isCurrent) {
      const row = document.createElement("div");
      row.className = "row";
      const switchBtn = document.createElement("button");
      switchBtn.className = "primary";
      switchBtn.textContent = "Switch";
      switchBtn.onclick = () => {
        ext.runtime.sendMessage({ type: "LQD_SWITCH_NETWORK", chainId }, (res) => {
          if (!res || !res.ok) return setError(res?.error || "Switch failed");
          loadNetworkName();
          loadNetworksScreen();
          refreshBalance();
        });
      };
      row.appendChild(switchBtn);
      // Only allow remove for custom networks (not DEFAULT_NETWORKS)
      const isDefault = chainId === "0x8b" || chainId === "0x8c";
      if (!isDefault) {
        const removeBtn = document.createElement("button");
        removeBtn.className = "secondary";
        removeBtn.textContent = "Remove";
        removeBtn.onclick = () => {
          ext.runtime.sendMessage({ type: "LQD_REMOVE_NETWORK", chainId }, (res) => {
            if (!res || !res.ok) return setError(res?.error || "Remove failed");
            loadNetworksScreen();
          });
        };
        row.appendChild(removeBtn);
      }
      div.appendChild(row);
    }
    container.appendChild(div);
  }
}

function loadNetworksScreen() {
  ext.runtime.sendMessage({ type: "LQD_GET_NETWORKS" }, (res) => {
    if (!res || !res.ok) return;
    renderNetworkList(res.networks, res.currentNetwork);
  });
}

on("backFromNetworks", "click", () => showScreen("settings"));

on("addNetworkBtn", "click", () => {
  const name = document.getElementById("netName")?.value.trim();
  const chainId = document.getElementById("netChainId")?.value.trim();
  const nodeUrl = document.getElementById("netNodeUrl")?.value.trim();
  const walletUrl = document.getElementById("netWalletUrl")?.value.trim() || PROD_WALLET_URL;
  const symbol = document.getElementById("netSymbol")?.value.trim() || "LQD";
  const statusEl = document.getElementById("addNetworkStatus");

  if (!name || !chainId || !nodeUrl) {
    if (statusEl) statusEl.textContent = "Name, Chain ID and Node URL are required";
    return;
  }
  ext.runtime.sendMessage({ type: "LQD_ADD_NETWORK", network: { chainId, name, nodeUrl, walletUrl, symbol } }, (res) => {
    if (!res || !res.ok) {
      if (statusEl) statusEl.textContent = res?.error || "Failed to add network";
      return;
    }
    if (statusEl) statusEl.textContent = "Network added!";
    ["netName", "netChainId", "netNodeUrl", "netWalletUrl", "netSymbol"].forEach((id) => {
      const el = document.getElementById(id);
      if (el) el.value = "";
    });
    loadNetworksScreen();
  });
});

// ── Init / load ───────────────────────────────────────────────────────────────
async function load() {
  const data = await ext.storage.local.get(["address", "nodeUrl", "walletUrl", "tokenWatchlist", "locked", "txActivity"]);
  if (data.nodeUrl && isLocalEndpoint(data.nodeUrl)) {
    data.nodeUrl = PROD_CHAIN_URL;
    await ext.storage.local.set({ nodeUrl: data.nodeUrl });
  }
  if (data.walletUrl && isLocalEndpoint(data.walletUrl)) {
    data.walletUrl = PROD_WALLET_URL;
    await ext.storage.local.set({ walletUrl: data.walletUrl });
  }
  const nodeUrlEl = document.getElementById("nodeUrl");
  const walletUrlEl = document.getElementById("walletUrl");
  if (nodeUrlEl) nodeUrlEl.value = data.nodeUrl || PROD_CHAIN_URL;
  if (walletUrlEl) walletUrlEl.value = data.walletUrl || PROD_WALLET_URL;

  if (data.address) {
    const locked = data.locked !== false;
    setStatus(locked, data.address);
    showScreen(locked ? "locked" : "main");
    if (!locked) {
      refreshBalance();
      loadNetworkName();
    }
  } else {
    showScreen("onboarding");
  }

  renderTokens(data.tokenWatchlist || []);
  renderActivity(data.txActivity || []);

  // ── Auto-show dApps tab if there are pending approvals ──────────────────
  ext.runtime.sendMessage({ type: "LQD_GET_PENDING" }, (res) => {
    const list = res?.list || [];
    if (list.length > 0 && !data.locked) {
      // Jump straight to the dApps/approvals tab so user sees the pending request
      switchTab({ tab: "tabDapps", pane: "dappsPane" });
      renderPending(list);
      refreshAllowlist();
    } else {
      refreshPending();
      refreshAllowlist();
    }
  });
}

// ── Onboarding ────────────────────────────────────────────────────────────────
on("startCreate", "click", () => showScreen("create"));
on("startImport", "click", () => showScreen("import"));
on("backFromCreate", "click", () => showScreen("onboarding"));
on("backFromImport", "click", () => showScreen("onboarding"));

on("createBtn", "click", async () => {
  const pass = document.getElementById("createPass")?.value || "";
  const pass2 = document.getElementById("createPass2")?.value || "";
  const out = document.getElementById("createOut");
  if (pass.length < 8) { if (out) out.textContent = "Password must be at least 8 characters"; return; }
  if (pass !== pass2) { if (out) out.textContent = "Passwords do not match"; return; }
  if (out) out.textContent = "Creating…";

  const data = await ext.storage.local.get(["walletUrl"]);
  let walletServer = (data.walletUrl || PROD_WALLET_URL).replace(/\/$/, "");
  if (isLocalEndpoint(walletServer)) walletServer = PROD_WALLET_URL;
  try {
    const res = await fetch(`${walletServer}/wallet/new`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ password: pass })
    });
    const json = await res.json();
    if (!res.ok) throw new Error(json.error || "Create failed");
    ext.runtime.sendMessage({
      type: "LQD_IMPORT",
      payload: { address: json.address, privateKey: json.private_key, password: pass }
    }, (result) => {
      if (!result?.ok) { if (out) out.textContent = result?.error || "Import failed"; return; }
      if (json.mnemonic) {
        ext.runtime.sendMessage({ type: "LQD_STORE_MNEMONIC", mnemonic: json.mnemonic, password: pass }, () => { });
      }
      // Auto-unlock immediately after create — password is already known
      ext.runtime.sendMessage({ type: "LQD_UNLOCK", password: pass }, (unlockRes) => {
        if (unlockRes?.ok) {
          setStatus(false, json.address);
        } else {
          setStatus(true, json.address);
        }
        if (out) out.textContent = `✓ Wallet created!\nAddress: ${json.address}\n\nSave your seed phrase in Settings.`;
        setTimeout(() => {
          showScreen("main");
          refreshBalance();
          loadNetworkName();
        }, 1200);
      });
    });
  } catch (e) {
    if (out) out.textContent = e.message;
  }
});

on("importBtn", "click", () => {
  const addr = document.getElementById("importAddress")?.value.trim();
  const pk = document.getElementById("importKey")?.value.trim();
  const pass = document.getElementById("importPass")?.value;
  const out = document.getElementById("importOut");
  if (!addr || !pk || !pass) { if (out) out.textContent = "All fields required"; return; }
  ext.runtime.sendMessage({ type: "LQD_IMPORT", payload: { address: addr, privateKey: pk, password: pass } }, (res) => {
    if (!res?.ok) { if (out) out.textContent = res?.error || "Import failed"; return; }
    if (out) out.textContent = "✓ Wallet imported!";
    // Auto-unlock immediately after import — password is already known
    ext.runtime.sendMessage({ type: "LQD_UNLOCK", password: pass }, (unlockRes) => {
      if (unlockRes?.ok) {
        setStatus(false, addr);
      } else {
        setStatus(true, addr);
      }
      setTimeout(() => {
        showScreen("main");
        refreshBalance();
        loadNetworkName();
      }, 800);
    });
  });
});

// ── Unlock ────────────────────────────────────────────────────────────────────
on("unlockBtn", "click", () => {
  const pass = document.getElementById("unlockPass")?.value || "";
  const errEl = document.getElementById("unlockErr");
  ext.runtime.sendMessage({ type: "LQD_UNLOCK", password: pass }, (res) => {
    if (!res?.ok) {
      if (errEl) errEl.textContent = res?.error || "Wrong password";
      return;
    }
    setStatus(false, res.address);
    showScreen("main");
    refreshBalance();
    loadNetworkName();
  });
});

// ── Settings ──────────────────────────────────────────────────────────────────
on("openSettings", "click", () => showScreen("settings"));
on("openFullPage", "click", () => {
  ext.tabs.create({ url: ext.runtime.getURL("fullpage.html") });
});
on("backFromSettings", "click", () => showScreen("main"));
on("openNetworks", "click", () => { loadNetworksScreen(); showScreen("networks"); });

on("lockBtn", "click", () => {
  ext.runtime.sendMessage({ type: "LQD_LOCK" });
  setStatus(true, "");
  showScreen("locked");
});

on("saveEndpoints", "click", () => {
  const nodeUrl = document.getElementById("nodeUrl")?.value.trim();
  const walletUrl = document.getElementById("walletUrl")?.value.trim();
  const statusEl = document.getElementById("endpointStatus");
  ext.runtime.sendMessage({ type: "LQD_SET_ENDPOINTS", nodeUrl, walletUrl }, (res) => {
    if (statusEl) statusEl.textContent = res?.ok ? "✓ Saved" : (res?.error || "Failed");
  });
});

on("presetLocal5000", "click", () => {
  const n = document.getElementById("nodeUrl");
  const w = document.getElementById("walletUrl");
  if (n) n.value = PROD_CHAIN_URL;
  if (w) w.value = PROD_WALLET_URL;
});
on("presetLocal9000", "click", () => {
  const n = document.getElementById("nodeUrl");
  const w = document.getElementById("walletUrl");
  if (n) n.value = PROD_AGGREGATOR_URL;
  if (w) w.value = PROD_WALLET_URL;
});

on("revealKey", "click", () => {
  const pass = document.getElementById("revealPass")?.value;
  const out = document.getElementById("revealOut");
  ext.runtime.sendMessage({ type: "LQD_REVEAL_SECRET", kind: "privateKey", password: pass }, (res) => {
    if (!res?.ok) { if (out) out.textContent = res?.error || "Failed"; return; }
    if (out) { out.textContent = res.secret; out.classList.add("revealed"); }
    setTimeout(() => { if (out) { out.textContent = ""; out.classList.remove("revealed"); } }, 15000);
  });
});

on("revealMnemonic", "click", () => {
  const pass = document.getElementById("revealPass")?.value;
  const out = document.getElementById("revealOut");
  ext.runtime.sendMessage({ type: "LQD_REVEAL_SECRET", kind: "mnemonic", password: pass }, (res) => {
    if (!res?.ok) { if (out) out.textContent = res?.error || "Failed"; return; }
    if (out) { out.textContent = res.secret; out.classList.add("revealed"); }
    setTimeout(() => { if (out) { out.textContent = ""; out.classList.remove("revealed"); } }, 15000);
  });
});

on("exportBackup", "click", async () => {
  const data = await ext.storage.local.get([
    "address", "walletCipher", "walletIv", "walletSalt",
    "mnemonicCipher", "mnemonicIv", "mnemonicSalt",
    "nodeUrl", "walletUrl", "tokenWatchlist", "allowlist", "networks", "currentNetwork"
  ]);
  const blob = new Blob([JSON.stringify(data, null, 2)], { type: "application/json" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url; a.download = "lqd-wallet-backup.json"; a.click();
  URL.revokeObjectURL(url);
});

// ── Balance & copy address ────────────────────────────────────────────────────
on("refreshBalance", "click", refreshBalance);

on("copyAddrBtn", "click", async () => {
  const data = await ext.storage.local.get(["address"]);
  if (data.address) {
    try { await navigator.clipboard.writeText(data.address); } catch { }
  }
});

// ── Receive modal ─────────────────────────────────────────────────────────────
on("openReceive", "click", async () => {
  const data = await ext.storage.local.get(["address"]);
  const el = document.getElementById("receiveAddress");
  if (el) el.textContent = data.address || "";
  document.getElementById("receiveModal").classList.remove("hidden");
});
on("receiveClose", "click", () => document.getElementById("receiveModal").classList.add("hidden"));
on("copyReceiveAddress", "click", async () => {
  const data = await ext.storage.local.get(["address"]);
  if (data.address) {
    try { await navigator.clipboard.writeText(data.address); } catch { }
  }
});

// ── Send LQD modal ────────────────────────────────────────────────────────────
async function updateSendFeePreview() {
  const feeEl = document.getElementById("sendFeePreview");
  if (!feeEl) return;
  const fee = await estimateFee(21000);
  feeEl.textContent = `Est. fee: ${formatLQD(fee)} LQD (gas=21000)`;
}

// Keep raw balance in memory for MAX button (refreshed when modal opens)
let _sendRawBalance = "0";

on("openSend", "click", async () => {
  document.getElementById("sendModal").classList.remove("hidden");
  updateSendFeePreview();
  // Fetch fresh balance and show it
  const data = await ext.storage.local.get(["address", "nodeUrl"]);
  if (data.address) {
    let url = (data.nodeUrl || PROD_CHAIN_URL).replace(/\/$/, "");
    if (isLocalEndpoint(url)) url = PROD_CHAIN_URL;
    try {
      const res = await fetch(`${url}/balance?address=${encodeURIComponent(data.address)}`);
      const json = await res.json();
      _sendRawBalance = json.balance || json.Balance || "0";
      const el = document.getElementById("sendBalanceDisplay");
      if (el) el.textContent = `Balance: ${formatLQD(_sendRawBalance)} LQD`;
    } catch { }
  }
});

on("sendMaxBtn", "click", async () => {
  const fee = await estimateFee(21000);
  try {
    const bal = BigInt(_sendRawBalance || "0");
    const f = BigInt(fee || 21000);
    const maxRaw = bal > f ? bal - f : 0n;
    const DECIMALS = 8;
    const intPart = maxRaw / BigInt(10 ** DECIMALS);
    const fracRaw = maxRaw % BigInt(10 ** DECIMALS);
    const fracPart = fracRaw.toString().padStart(DECIMALS, "0").replace(/0+$/, "");
    const human = fracPart ? `${intPart}.${fracPart}` : String(intPart);
    const el = document.getElementById("sendAmount");
    if (el) el.value = human;
  } catch { }
});
on("sendCancel", "click", () => {
  document.getElementById("sendModal").classList.add("hidden");
  document.getElementById("sendStatus").textContent = "";
});

on("sendSubmit", "click", async () => {
  const statusEl = document.getElementById("sendStatus");
  const toAddr = document.getElementById("sendTo")?.value.trim();
  const amountStr = document.getElementById("sendAmount")?.value.trim();
  if (!toAddr || !amountStr) { if (statusEl) statusEl.textContent = "Fill all fields"; return; }
  if (!toAddr.startsWith("0x")) { if (statusEl) statusEl.textContent = "Invalid address"; return; }

  const GAS = 21000;
  const fee = await estimateFee(GAS);
  const baseFee = await fetchBaseFee();
  const rawAmount = parseAmount(amountStr, 8);

  const result = await showConfirmModal({
    title: "Send LQD",
    details: `<div><strong>To:</strong> ${toAddr}</div><div><strong>Amount:</strong> ${amountStr} LQD</div>`,
    fee: formatLQD(fee),
    origin: "LQD Wallet"
  });

  if (!result.confirmed) { if (statusEl) statusEl.textContent = "Cancelled"; return; }

  if (statusEl) statusEl.textContent = "Sending…";
  const submitBtn = document.getElementById("sendSubmit");
  if (submitBtn) submitBtn.disabled = true;

  ext.runtime.sendMessage({
    type: "LQD_REQUEST",
    payload: {
      id: Math.random().toString(16).slice(2),
      method: "lqd_sendTransaction",
      params: [{ to: toAddr, value: rawAmount, gas: GAS, gas_price: baseFee }]
    }
  }, (res) => {
    if (submitBtn) submitBtn.disabled = false;
    if (res?.error) {
      if (statusEl) statusEl.textContent = parseErr(res.error);
    } else {
      const hash = res?.result?.tx_hash || res?.result?.TxHash || res?.result?.hash || "";
      if (statusEl) statusEl.textContent = hash ? `✓ Sent! Tx: ${hash.slice(0, 18)}…` : "✓ Submitted";
      document.getElementById("sendTo").value = "";
      document.getElementById("sendAmount").value = "";
      refreshBalance();
      refreshActivity();
      try {
        window.dispatchEvent(new CustomEvent("lqd:wallet-updated", { detail: { address: toAddr } }));
      } catch { }
    }
  });
});

// ── Token send modal ──────────────────────────────────────────────────────────
let activeTokenForSend = { address: "", symbol: "", decimals: 8 };

function showTokenSendModal(token) {
  activeTokenForSend = token;
  const nameEl = document.getElementById("tokenSendName");
  if (nameEl) nameEl.textContent = `${token.symbol || token.address} (${token.address})`;
  document.getElementById("tokenSendModal").classList.remove("hidden");
  document.getElementById("tokenSendStatus").textContent = "";
  estimateFee(50000).then((fee) => {
    const feeEl = document.getElementById("tokenSendFee");
    if (feeEl) feeEl.textContent = `Est. fee: ${formatLQD(fee)} LQD (gas=50000)`;
  });
}

on("tokenSendCancel", "click", () => document.getElementById("tokenSendModal").classList.add("hidden"));
on("tokenSendSubmit", "click", async () => {
  const statusEl = document.getElementById("tokenSendStatus");
  const toAddr = document.getElementById("tokenSendTo")?.value.trim();
  const amountStr = document.getElementById("tokenSendAmount")?.value.trim();
  if (!toAddr || !amountStr) { if (statusEl) statusEl.textContent = "Fill all fields"; return; }

  const GAS = 50000;
  const fee = await estimateFee(GAS);
  const baseFee = await fetchBaseFee();
  const rawAmount = parseAmount(amountStr, activeTokenForSend.decimals || 8);

  const result = await showConfirmModal({
    title: `Send ${activeTokenForSend.symbol || "Token"}`,
    details: `<div><strong>Contract:</strong> ${truncate(activeTokenForSend.address, 10)}</div><div><strong>To:</strong> ${toAddr}</div><div><strong>Amount:</strong> ${amountStr}</div>`,
    fee: formatLQD(fee),
    origin: "LQD Wallet"
  });

  if (!result.confirmed) { if (statusEl) statusEl.textContent = "Cancelled"; return; }
  if (statusEl) statusEl.textContent = "Sending…";

  const submitBtn = document.getElementById("tokenSendSubmit");
  if (submitBtn) submitBtn.disabled = true;

  ext.runtime.sendMessage({
    type: "LQD_REQUEST",
    payload: {
      id: Math.random().toString(16).slice(2),
      method: "lqd_contractTx",
      params: [{
        contract_address: activeTokenForSend.address,
        function: "Transfer",
        args: [toAddr, rawAmount],
        value: "0",
        gas: GAS,
        gas_price: baseFee
      }]
    }
  }, (res) => {
    if (submitBtn) submitBtn.disabled = false;
    if (res?.error) {
      if (statusEl) statusEl.textContent = parseErr(res.error);
    } else {
      const hash = res?.result?.tx_hash || res?.result?.TxHash || "";
      if (statusEl) statusEl.textContent = hash ? `✓ Sent! Tx: ${hash.slice(0, 18)}…` : "✓ Submitted";
      refreshActivity();
      refreshBalance();
      ext.storage.local.get(["tokenWatchlist"]).then((d) => renderTokens(d.tokenWatchlist || [])).catch(() => { });
      try {
        window.dispatchEvent(new CustomEvent("lqd:wallet-updated", { detail: { address: toAddr, token: activeTokenForSend.address } }));
      } catch { }
    }
  });
});

// ── Tabs ──────────────────────────────────────────────────────────────────────
function switchTab(active) {
  ["tabTokens", "tabActivity", "tabDapps"].forEach((id) => {
    document.getElementById(id)?.classList.remove("active");
  });
  ["tokensPane", "activityPane", "dappsPane"].forEach((id) => {
    document.getElementById(id)?.classList.add("hidden");
  });
  document.getElementById(active.tab)?.classList.add("active");
  document.getElementById(active.pane)?.classList.remove("hidden");
}

on("tabTokens", "click", () => switchTab({ tab: "tabTokens", pane: "tokensPane" }));
on("tabActivity", "click", () => switchTab({ tab: "tabActivity", pane: "activityPane" }));
on("tabDapps", "click", () => { switchTab({ tab: "tabDapps", pane: "dappsPane" }); renderDappStore(); refreshPending(); refreshAllowlist(); });

// ── Tokens ────────────────────────────────────────────────────────────────────
async function fetchTokenMeta(contractAddr, nodeUrl) {
  let base = (nodeUrl || PROD_CHAIN_URL).replace(/\/$/, "");
  if (isLocalEndpoint(base)) base = PROD_CHAIN_URL;
  const call = async (fn) => {
    try {
      const res = await fetch(`${base}/contract/call`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ address: contractAddr, fn, args: [], caller: contractAddr, value: 0 })
      });
      const json = await res.json();
      return json.output || "";
    } catch { return ""; }
  };
  const [symbol, name, decimalsStr] = await Promise.all([
    call("Symbol").then((v) => v || call("symbol")),
    call("Name").then((v) => v || call("name")),
    call("Decimals").then((v) => v || call("decimals")),
  ]);
  return {
    symbol: symbol || "TOKEN",
    name: name || symbol || "Token",
    decimals: parseInt(decimalsStr, 10) || 8
  };
}

async function fetchTokenBalance(contractAddr, walletAddr, nodeUrl) {
  let base = (nodeUrl || PROD_CHAIN_URL).replace(/\/$/, "");
  if (isLocalEndpoint(base)) base = PROD_CHAIN_URL;
  const call = async (fn) => {
    try {
      const res = await fetch(`${base}/contract/call`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ address: contractAddr, fn, args: [walletAddr], caller: walletAddr, value: 0 })
      });
      const json = await res.json();
      return json.output || "0";
    } catch { return "0"; }
  };
  try {
    const upper = await call("BalanceOf");
    return upper || await call("balanceOf");
  } catch { return "0"; }
}

async function renderTokens(watchlist) {
  const container = document.getElementById("tokenList");
  if (!container) return;
  if (!watchlist || watchlist.length === 0) {
    container.innerHTML = '<div class="empty-state">No tokens added yet.</div>';
    return;
  }
  container.innerHTML = '<div class="loading-text">Loading tokens…</div>';

  const data = await ext.storage.local.get(["address", "nodeUrl"]);
  const addr = data.address;
  const nodeUrl = isLocalEndpoint(data.nodeUrl) ? PROD_CHAIN_URL : (data.nodeUrl || PROD_CHAIN_URL);

  container.innerHTML = "";
  for (const token of watchlist) {
    const contractAddr = typeof token === "string" ? token : token.contract;
    const savedMeta = typeof token === "object" ? token : {};

    const div = document.createElement("div");
    div.className = "item token-item";
    div.innerHTML = `<div class="token-loading">Loading ${truncate(contractAddr, 8)}…</div>`;
    container.appendChild(div);

    // fetch meta and balance in parallel
    const [meta, rawBal] = await Promise.all([
      savedMeta.symbol ? Promise.resolve(savedMeta) : fetchTokenMeta(contractAddr, nodeUrl),
      addr ? fetchTokenBalance(contractAddr, addr, nodeUrl) : Promise.resolve("0")
    ]);

    const fmtBal = formatLQD(rawBal);
    div.innerHTML = `
      <div class="token-row">
        <div class="token-icon">${(meta.symbol || "?")[0]}</div>
        <div class="token-info">
          <div class="token-name">${meta.name || meta.symbol}</div>
          <div class="mono">${truncate(contractAddr, 10)}</div>
        </div>
        <div class="token-balance">${fmtBal} <span class="token-symbol">${meta.symbol}</span></div>
      </div>`;

    const row = document.createElement("div");
    row.className = "row";
    const sendBtn = document.createElement("button");
    sendBtn.className = "secondary mini";
    sendBtn.textContent = "Send";
    sendBtn.onclick = () => showTokenSendModal({ address: contractAddr, symbol: meta.symbol, decimals: meta.decimals });
    const removeBtn = document.createElement("button");
    removeBtn.className = "ghost mini";
    removeBtn.textContent = "✕";
    removeBtn.onclick = async () => {
      const d = await ext.storage.local.get(["tokenWatchlist"]);
      const list = (d.tokenWatchlist || []).filter((t) =>
        (typeof t === "string" ? t : t.contract) !== contractAddr
      );
      await ext.storage.local.set({ tokenWatchlist: list });
      renderTokens(list);
    };
    row.appendChild(sendBtn);
    row.appendChild(removeBtn);
    div.appendChild(row);
  }
}

on("addToken", "click", async () => {
  const input = document.getElementById("tokenAddress");
  const addr = input?.value.trim();
  if (!addr) return;
  const data = await ext.storage.local.get(["tokenWatchlist"]);
  const list = data.tokenWatchlist || [];
  const exists = list.some((t) => (typeof t === "string" ? t : t.contract).toLowerCase() === addr.toLowerCase());
  if (exists) { setError("Token already added"); return; }
  list.push(addr);
  await ext.storage.local.set({ tokenWatchlist: list });
  if (input) input.value = "";
  renderTokens(list);
});

// ── Activity ──────────────────────────────────────────────────────────────────
async function refreshActivity() {
  const data = await ext.storage.local.get(["txActivity"]);
  renderActivity(data.txActivity || []);
}

function renderActivity(list) {
  const container = document.getElementById("activityList");
  if (!container) return;
  if (!list.length) { container.innerHTML = '<div class="empty-state">No activity yet.</div>'; return; }
  container.innerHTML = "";
  list.slice(0, 50).forEach((tx) => {
    const div = document.createElement("div");
    div.className = "item activity-item";
    const time = tx.time ? new Date(tx.time).toLocaleString() : "";
    const typeLabel = tx.type === "token" ? "Token Transfer" : tx.type === "send" ? "Send LQD" : "Contract Call";
    const icon = tx.type === "send" ? "↑" : tx.type === "token" ? "⬡" : "⚙";
    div.innerHTML = `
      <div class="activity-row">
        <div class="activity-icon">${icon}</div>
        <div class="activity-info">
          <div class="activity-type">${typeLabel}</div>
          <div class="mono">To: ${truncate(tx.to || "-", 8)}</div>
          <div class="mono">${time}</div>
        </div>
        <div class="activity-amount">${formatLQD(tx.amount || tx.value || "0")}</div>
      </div>`;
    div.onclick = () => openActivityModal(tx);
    container.appendChild(div);
  });
}

function openActivityModal(tx) {
  const titleEl = document.getElementById("activityTitle");
  const detailsEl = document.getElementById("activityDetails");
  if (titleEl) titleEl.textContent = tx.type === "token" ? "Token Transfer" : tx.type === "send" ? "Send LQD" : "Contract Call";
  if (detailsEl) detailsEl.textContent = [
    `Hash:   ${tx.tx_hash || "-"}`,
    `To:     ${tx.to || "-"}`,
    `Amount: ${tx.amount || tx.value || "0"}`,
    tx.function ? `Fn:     ${tx.function}` : "",
    tx.chainId ? `Chain:  ${tx.chainId}` : "",
    tx.time ? `Time:   ${new Date(tx.time).toLocaleString()}` : ""
  ].filter(Boolean).join("\n");
  document.getElementById("activityModal").classList.remove("hidden");
}

on("activityClose", "click", () => document.getElementById("activityModal").classList.add("hidden"));
on("activityCopy", "click", async () => {
  const detailsEl = document.getElementById("activityDetails");
  const lines = (detailsEl?.textContent || "").split("\n");
  const hashLine = lines.find((l) => l.startsWith("Hash:"));
  if (hashLine) {
    const hash = hashLine.replace("Hash:", "").trim();
    if (hash && hash !== "-") try { await navigator.clipboard.writeText(hash); } catch { }
  }
});

// Live activity updates
if (ext.storage?.onChanged) {
  ext.storage.onChanged.addListener((changes, area) => {
    if (area !== "local") return;
    if (changes.txActivity) renderActivity(changes.txActivity.newValue || []);
  });
}

// ── Official dApp Registry ────────────────────────────────────────────────────
// To add/remove dApps, edit this list. url can be absolute or relative.


function renderDappStore() {
  const container = document.getElementById("dappStoreList");
  if (!container) return;
  container.innerHTML = "";
  for (const dapp of OFFICIAL_DAPPS) {
    const card = document.createElement("div");
    card.className = "dapp-card";
    card.innerHTML = `
      <div class="dapp-icon" style="background:${dapp.iconBg}">${dapp.icon}</div>
      <div class="dapp-info">
        <div class="dapp-name">${dapp.name}</div>
        <div class="dapp-desc">${dapp.description}</div>
      </div>
      <span class="dapp-category">${dapp.category}</span>`;

    const launch = document.createElement("button");
    launch.className = "dapp-launch";
    launch.textContent = "Open ↗";
    launch.onclick = (e) => {
      e.stopPropagation();
      ext.tabs.create({ url: dapp.url });
    };
    card.onclick = () => ext.tabs.create({ url: dapp.url });
    card.appendChild(launch);
    container.appendChild(card);
  }
}

// ── Pending approvals ─────────────────────────────────────────────────────────
function refreshPending() {
  ext.runtime.sendMessage({ type: "LQD_GET_PENDING" }, (res) => {
    if (!res?.ok) return;
    renderPending(res.list || []);
  });
}

function renderPending(list) {
  const container = document.getElementById("pendingList");
  const section = document.getElementById("pendingSection");
  const badge = document.getElementById("pendingCount");
  if (!container) return;

  // Show/hide the whole pending block based on whether requests exist
  if (!list.length) {
    if (section) section.classList.add("hidden");
    return;
  }
  if (section) section.classList.remove("hidden");
  if (badge) badge.textContent = String(list.length);
  container.innerHTML = "";
  for (const item of list) {
    const div = document.createElement("div");
    div.className = "item pending-item";
    div.innerHTML = `
      <div><strong>${item.method}</strong></div>
      ${item.origin ? `<div class="mono origin-text">${item.origin}</div>` : ""}
      <div class="mono">${summarizeRequest(item)}</div>`;
    const row = document.createElement("div");
    row.className = "row";
    const remember = document.createElement("input");
    remember.type = "checkbox";
    remember.title = "Remember this site";
    const label = document.createElement("label");
    label.style.cssText = "display:flex;align-items:center;gap:4px;font-size:11px;flex:1";
    label.appendChild(remember);
    label.appendChild(Object.assign(document.createElement("span"), { textContent: "Remember" }));
    const approve = document.createElement("button");
    approve.className = "primary";
    approve.textContent = "Approve";
    approve.onclick = () => handleApproval(item.id, true, remember.checked);
    const deny = document.createElement("button");
    deny.className = "secondary";
    deny.textContent = "Deny";
    deny.onclick = () => handleApproval(item.id, false, false);
    row.appendChild(label);
    row.appendChild(deny);
    row.appendChild(approve);
    div.appendChild(row);
    container.appendChild(div);
  }
}

function handleApproval(id, allow, remember) {
  ext.runtime.sendMessage({ type: "LQD_APPROVE", id, allow, remember }, () => {
    refreshPending();
    refreshAllowlist();
  });
}

function summarizeRequest(item) {
  const params = item.params || [];
  if (item.method === "lqd_connect" || item.method === "lqd_requestAccounts") return "Request account access";
  if (item.method === "lqd_sendTransaction") {
    const p = params[0] || {};
    return `To: ${truncate(p.to || "-", 8)} | Value: ${p.value || "0"}`;
  }
  if (item.method === "lqd_contractTx") {
    const p = params[0] || {};
    return `Contract: ${truncate(p.contract_address || "-", 8)} | Fn: ${p.function || "-"}`;
  }
  return JSON.stringify(params).slice(0, 60);
}

// ── Connected sites (allowlist) ───────────────────────────────────────────────
function refreshAllowlist() {
  ext.runtime.sendMessage({ type: "LQD_GET_ALLOWLIST" }, (res) => {
    if (!res?.ok) return;
    renderAllowlist(res.allowlist || {});
  });
}

function renderAllowlist(allowlist) {
  const container = document.getElementById("allowList");
  if (!container) return;
  const origins = Object.keys(allowlist || {});
  if (!origins.length) { container.innerHTML = '<div class="empty-state">No connected sites.</div>'; return; }
  container.innerHTML = "";
  for (const origin of origins) {
    const scopes = Object.keys(allowlist[origin] || {});
    const div = document.createElement("div");
    div.className = "item";
    div.innerHTML = `<strong>${origin}</strong><div class="mono">Permissions: ${scopes.join(", ")}</div>`;
    const row = document.createElement("div");
    row.className = "row";
    for (const scope of scopes) {
      const btn = document.createElement("button");
      btn.textContent = `Revoke ${scope}`;
      btn.className = "ghost mini";
      btn.onclick = () => ext.runtime.sendMessage({ type: "LQD_REMOVE_ALLOW", origin, scope }, refreshAllowlist);
      row.appendChild(btn);
    }
    div.appendChild(row);
    container.appendChild(div);
  }
}

// ── Auto-refresh balance every 30s ────────────────────────────────────────────
let autoRefreshInterval = null;
function startAutoRefresh() {
  if (autoRefreshInterval) clearInterval(autoRefreshInterval);
  autoRefreshInterval = setInterval(refreshBalance, 30000);
}

// ── Boot ──────────────────────────────────────────────────────────────────────
load().then(startAutoRefresh);
