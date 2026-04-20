const $ = (id) => document.getElementById(id);
const runtimeConfig = window.__BRIDGE_ADMIN_CONFIG__ || {};
const DEFAULT_NODE_URL = (runtimeConfig.defaultNodeUrl || "http://127.0.0.1:6500").trim();

const STORAGE_KEYS = {
  passwordHash: "lqd_bridge_admin_password_hash",
  nodeUrl: "lqd_bridge_admin_node_url",
  apiKey: "lqd_bridge_admin_api_key",
};

const state = {
  unlocked: false,
  chains: [],
  tokens: [],
  families: [],
};

function toJson(v) {
  try {
    return JSON.stringify(v, null, 2);
  } catch {
    return "{}";
  }
}

async function sha256(text) {
  const data = new TextEncoder().encode(text);
  const digest = await crypto.subtle.digest("SHA-256", data);
  return Array.from(new Uint8Array(digest)).map((b) => b.toString(16).padStart(2, "0")).join("");
}

function setStatus(text, kind = "") {
  const el = $("status");
  if (!el) return;
  el.textContent = text;
  el.dataset.kind = kind;
}

function getNodeUrl() {
  return ($("nodeUrl")?.value || DEFAULT_NODE_URL).trim();
}

function getApiKey() {
  return ($("apiKey")?.value || "").trim();
}

function authHeaders() {
  const apiKey = getApiKey();
  return apiKey ? { "X-API-Key": apiKey } : {};
}

async function apiGet(path) {
  const res = await fetch(getNodeUrl() + path);
  const text = await res.text();
  let data;
  try { data = JSON.parse(text); } catch { data = { raw: text }; }
  if (!res.ok) throw new Error(data.error || text || `HTTP ${res.status}`);
  return data;
}

async function apiPost(path, body) {
  const res = await fetch(getNodeUrl() + path, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      ...authHeaders(),
    },
    body: JSON.stringify(body),
  });
  const text = await res.text();
  let data;
  try { data = JSON.parse(text); } catch { data = { raw: text }; }
  if (!res.ok) throw new Error(data.error || text || `HTTP ${res.status}`);
  return data;
}

function renderFamilies() {
  const el = $("familiesList");
  if (!el) return;
  if (!state.families.length) {
    el.innerHTML = `<div class="item"><strong>No families loaded.</strong><div class="muted">Refresh the bridge registry first.</div></div>`;
    return;
  }
  el.innerHTML = state.families.map((fam) => `
    <div class="family-card">
      <div class="k">Family</div>
      <div class="v">${fam.name || fam.id || "Unknown"}</div>
      <div class="muted">id: ${fam.id || "—"}</div>
      <div class="muted">public: ${fam.supports_public ? "yes" : "no"} · private: ${fam.supports_private ? "yes" : "no"}</div>
      <div class="muted">${fam.description || ""}</div>
    </div>
  `).join("");
}

function familyDisplayName(familyId) {
  const id = String(familyId || "unknown").toLowerCase();
  const known = state.families.find((fam) => String(fam.id || "").toLowerCase() === id);
  return known?.name || id.toUpperCase();
}

function familyOrderKey(familyId) {
  const order = ["evm", "utxo", "cosmos", "substrate", "solana", "xrpl", "ton", "cardano", "aptos", "sui", "near", "icp"];
  const idx = order.indexOf(String(familyId || "").toLowerCase());
  return idx === -1 ? 999 : idx;
}

function renderChains() {
  const el = $("chainsList");
  if (!el) return;
  if (!state.chains.length) {
    el.innerHTML = `<div class="item"><strong>No chains loaded.</strong><div class="muted">Refresh or check your node URL.</div></div>`;
    return;
  }
  const grouped = state.chains.reduce((acc, cfg) => {
    const fam = String(cfg.family || "evm").toLowerCase();
    if (!acc[fam]) acc[fam] = [];
    acc[fam].push(cfg);
    return acc;
  }, {});
  const families = Object.keys(grouped).sort((a, b) => familyOrderKey(a) - familyOrderKey(b) || a.localeCompare(b));
  el.innerHTML = families.map((fam) => {
    const items = grouped[fam].sort((a, b) => String(a.name || a.id || "").localeCompare(String(b.name || b.id || "")));
    return `
      <div class="family-section">
        <div class="family-section-header">
          <div>
            <div class="family-section-title">${familyDisplayName(fam)}</div>
            <div class="family-section-sub">${items.length} chain${items.length === 1 ? "" : "s"} · ${fam.toUpperCase()}</div>
          </div>
        </div>
        <div class="family-chain-list">
          ${items.map((cfg) => `
            <div class="item family-chain-item" data-chain-id="${cfg.id || cfg.chain_id || ""}">
              <strong>${cfg.name || cfg.id || "Chain"}${cfg.enabled === false ? " (disabled)" : ""}</strong>
              <div class="muted">id: ${cfg.id || "—"} · chain_id: ${cfg.chain_id || "—"} · family: ${(cfg.family || "evm").toUpperCase()} · adapter: ${cfg.adapter || cfg.family || "evm"}</div>
              <div class="muted">rpc: ${cfg.rpc || (cfg.rpcs || []).join(", ") || "—"}</div>
              <div class="muted">bridge: ${cfg.bridge_address || "—"} · lock: ${cfg.lock_address || "—"}</div>
              <div class="muted">public/private: ${cfg.supports_public ? "public " : ""}${cfg.supports_private ? "private" : ""}</div>
              <div class="row">
                <button class="secondary" data-action="select-chain" data-id="${cfg.id || cfg.chain_id || ""}">Select</button>
              </div>
            </div>
          `).join("")}
        </div>
      </div>
    `;
  }).join("");
}

function renderTokens() {
  const el = $("tokensList");
  if (!el) return;
  if (!state.tokens.length) {
    el.innerHTML = `<div class="item"><strong>No tokens loaded.</strong><div class="muted">Add a mapping to get started.</div></div>`;
    return;
  }
  el.innerHTML = state.tokens.map((tok) => `
    <div class="item" data-token-chain="${tok.chain_id || ""}">
      <strong>${tok.name || tok.symbol || "Token"} · ${(tok.chain_id || "unknown").toUpperCase()}</strong>
      <div class="muted">source: ${tok.source_token || tok.bsc_token || "—"}</div>
      <div class="muted">lqd: ${tok.lqd_token || tok.target_token || "—"}</div>
      <div class="muted">family: ${(tok.family || "evm").toUpperCase()}</div>
      <div class="row">
        <button class="secondary" data-action="select-token" data-chain="${tok.chain_id || ""}">Select</button>
      </div>
    </div>
  `).join("");
}

function selectChain(cfg) {
  if (!cfg) return;
  $("chainId").value = cfg.id || cfg.chain_id || "";
  $("chainName").value = cfg.name || "";
  $("chainNumber").value = cfg.chain_id || "";
  $("chainFamily").value = cfg.family || "evm";
  $("chainAdapter").value = cfg.adapter || cfg.family || "evm";
  $("chainRpc").value = cfg.rpc || "";
  $("chainBridge").value = cfg.bridge_address || "";
  $("chainLock").value = cfg.lock_address || "";
  $("chainExplorer").value = cfg.explorer_url || "";
  $("chainSymbol").value = cfg.native_symbol || "";
  $("chainEnabled").checked = cfg.enabled !== false;
  $("chainPublic").checked = cfg.supports_public !== false;
  $("chainPrivate").checked = cfg.supports_private !== false;
  $("selectedChainJson").textContent = toJson(cfg);
}

function selectToken(tok) {
  if (!tok) return;
  $("tokenChainId").value = tok.chain_id || "";
  $("tokenSource").value = tok.source_token || tok.bsc_token || "";
  $("tokenLqd").value = tok.lqd_token || tok.target_token || "";
  $("tokenName").value = tok.name || "";
  $("tokenSymbol").value = tok.symbol || "";
  $("tokenDecimals").value = tok.decimals || "";
  $("selectedTokenJson").textContent = toJson(tok);
}

async function refreshAll() {
  setStatus("Loading registry...");
  const [families, chains, tokens] = await Promise.all([
    apiGet("/bridge/families"),
    apiGet("/bridge/chains"),
    apiGet("/bridge/tokens"),
  ]);
  state.families = Array.isArray(families) ? families : [];
  state.chains = Array.isArray(chains) ? chains : [];
  state.tokens = Array.isArray(tokens) ? tokens : [];
  renderFamilies();
  renderChains();
  renderTokens();
  setStatus(`Loaded ${state.chains.length} chains and ${state.tokens.length} token mappings.`, "ok");
}

function wireTabs() {
  document.querySelectorAll(".nav").forEach((btn) => {
    btn.addEventListener("click", () => {
      document.querySelectorAll(".nav").forEach((x) => x.classList.remove("active"));
      document.querySelectorAll(".tab").forEach((x) => x.classList.remove("active"));
      btn.classList.add("active");
      const tab = btn.dataset.tab;
      const panel = $("tab-" + tab);
      if (panel) panel.classList.add("active");
    });
  });
}

async function upsertChain() {
  const payload = {
    id: $("chainId").value.trim(),
    name: $("chainName").value.trim(),
    chain_id: $("chainNumber").value.trim(),
    family: $("chainFamily").value.trim() || "evm",
    adapter: $("chainAdapter").value.trim() || $("chainFamily").value.trim() || "evm",
    rpc: $("chainRpc").value.trim(),
    bridge_address: $("chainBridge").value.trim(),
    lock_address: $("chainLock").value.trim(),
    explorer_url: $("chainExplorer").value.trim(),
    native_symbol: $("chainSymbol").value.trim(),
    enabled: $("chainEnabled").checked,
    supports_public: $("chainPublic").checked,
    supports_private: $("chainPrivate").checked,
  };
  if (!payload.id || !payload.name || !payload.chain_id || !payload.family || !payload.adapter) {
    throw new Error("Fill id, name, chain id, family and adapter.");
  }
  if (payload.family === "evm" && (!payload.rpc || !payload.bridge_address)) {
    throw new Error("EVM chain needs rpc and bridge address.");
  }
  const res = await apiPost("/bridge/chain", payload);
  setStatus(`Saved chain: ${res?.chain?.name || payload.name}`, "ok");
  await refreshAll();
}

async function removeChain() {
  const id = $("chainId").value.trim();
  if (!id) throw new Error("Enter chain id.");
  await apiPost("/bridge/chain/remove", { id });
  setStatus(`Removed chain: ${id}`, "ok");
  await refreshAll();
}

async function upsertToken() {
  const payload = {
    chain_id: $("tokenChainId").value.trim(),
    source_token: $("tokenSource").value.trim(),
    lqd_token: $("tokenLqd").value.trim(),
    name: $("tokenName").value.trim(),
    symbol: $("tokenSymbol").value.trim(),
    decimals: $("tokenDecimals").value.trim(),
    target_chain_id: "lqd",
    target_chain_name: "LQD",
    target_token: $("tokenLqd").value.trim(),
    bsc_token: $("tokenSource").value.trim(),
  };
  if (!payload.chain_id || !payload.source_token || !payload.lqd_token) {
    throw new Error("Fill chain id, source token and LQD token.");
  }
  const res = await apiPost("/bridge/token", payload);
  setStatus(`Saved token mapping: ${(res?.token?.symbol || payload.symbol || "token")}`, "ok");
  await refreshAll();
}

async function removeToken() {
  const payload = {
    chain_id: $("tokenChainId").value.trim(),
    source_token: $("tokenSource").value.trim(),
    lqd_token: $("tokenLqd").value.trim(),
  };
  if (!payload.chain_id || (!payload.source_token && !payload.lqd_token)) {
    throw new Error("Fill chain and source or LQD token.");
  }
  await apiPost("/bridge/token/remove", payload);
  setStatus("Removed token mapping.", "ok");
  await refreshAll();
}

async function unlockUI() {
  const password = $("adminPassword").value.trim();
  if (!password) {
    setStatus("Enter the admin password first.", "warn");
    return;
  }
  const savedHash = localStorage.getItem(STORAGE_KEYS.passwordHash);
  const hash = await sha256(password);
  if (savedHash && savedHash !== hash) {
    setStatus("Wrong admin password.", "error");
    return;
  }
  localStorage.setItem(STORAGE_KEYS.passwordHash, hash);
  localStorage.setItem(STORAGE_KEYS.nodeUrl, getNodeUrl());
  localStorage.setItem(STORAGE_KEYS.apiKey, getApiKey());
  $("gate").classList.add("hidden");
  $("app").classList.remove("hidden");
  state.unlocked = true;
  await refreshAll();
}

function restoreLocalSettings() {
  const nodeUrl = localStorage.getItem(STORAGE_KEYS.nodeUrl);
  const apiKey = localStorage.getItem(STORAGE_KEYS.apiKey);
  const normalized = nodeUrl
    ? (nodeUrl.includes(":9000") || nodeUrl.includes(":8080") ? DEFAULT_NODE_URL : nodeUrl)
    : DEFAULT_NODE_URL;
  $("nodeUrl").value = normalized;
  if (nodeUrl && normalized !== nodeUrl) localStorage.setItem(STORAGE_KEYS.nodeUrl, normalized);
  if (apiKey) $("apiKey").value = apiKey;
}

function wireEvents() {
  $("unlockBtn").addEventListener("click", unlockUI);
  $("saveLocalBtn").addEventListener("click", async () => {
    localStorage.setItem(STORAGE_KEYS.nodeUrl, getNodeUrl());
    localStorage.setItem(STORAGE_KEYS.apiKey, getApiKey());
    const pwd = $("password").value.trim();
    if (pwd) localStorage.setItem(STORAGE_KEYS.passwordHash, await sha256(pwd));
    setStatus("UI settings saved locally.", "ok");
  });
  $("refreshBtn").addEventListener("click", () => refreshAll().catch((e) => setStatus(e.message, "error")));
  $("saveChainBtn").addEventListener("click", () => upsertChain().catch((e) => setStatus(e.message, "error")));
  $("removeChainBtn").addEventListener("click", () => removeChain().catch((e) => setStatus(e.message, "error")));
  $("saveTokenBtn").addEventListener("click", () => upsertToken().catch((e) => setStatus(e.message, "error")));
  $("removeTokenBtn").addEventListener("click", () => removeToken().catch((e) => setStatus(e.message, "error")));

  document.addEventListener("click", (ev) => {
    const action = ev.target?.dataset?.action;
    if (action === "select-chain") {
      const id = ev.target.dataset.id;
      const chain = state.chains.find((item) => String(item.id || item.chain_id || "") === String(id || ""));
      selectChain(chain);
    }
    if (action === "select-token") {
      const chainId = ev.target.dataset.chain;
      const token = state.tokens.find((item) => String(item.chain_id || "") === String(chainId || ""));
      selectToken(token);
    }
  });
}

async function boot() {
  restoreLocalSettings();
  wireTabs();
  wireEvents();
  setStatus("Enter the local admin password to unlock the console.");
}

boot().catch((err) => setStatus(err.message || "Failed to start.", "error"));
