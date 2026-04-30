const ext = typeof chrome !== "undefined" ? chrome : browser;

const PROD_CHAIN_URL = "https://dazzling-peace-production-3529.up.railway.app";
const PROD_WALLET_URL = "https://enchanting-hope-production-1c63.up.railway.app";
const PROD_AGGREGATOR_URL = "https://keen-enjoyment-production-0440.up.railway.app";
const PROD_EXPLORER_URL = "https://warm-dragon-34d6ff.netlify.app";
const PROD_DEX_URL = "https://bright-crisp-91fe94.netlify.app";
// ── Default networks ──────────────────────────────────────────────────────────
const DEFAULT_NETWORKS = {
  "0x8b": {
    chainId: "0x8b",
    name: "LQD Mainnet",
    nodeUrl: PROD_CHAIN_URL,
    walletUrl: PROD_WALLET_URL,
    symbol: "LQD",
    blockExplorer: PROD_EXPLORER_URL
  },
  "0x8c": {
    chainId: "0x8c",
    name: "LQD Aggregator",
    nodeUrl: PROD_AGGREGATOR_URL,
    walletUrl: PROD_WALLET_URL,
    symbol: "LQD",
    blockExplorer: PROD_EXPLORER_URL
  }
};

let session = {
  unlocked: false,
  address: "",
  privateKey: "",
  chainId: "0x8b",
  nodeUrl: PROD_CHAIN_URL,
  walletUrl: PROD_WALLET_URL
};

const portsByTab = new Map();
const pending = new Map();
const AUTO_LOCK_MINUTES = 15;

// ── Auto lock ─────────────────────────────────────────────────────────────────
function resetAutoLock() {
  try {
    ext.alarms.clear("autoLock");
    ext.alarms.create("autoLock", { delayInMinutes: AUTO_LOCK_MINUTES });
    ext.storage.local.set({ lastActive: Date.now() });
  } catch { }
}

// ── Badge ─────────────────────────────────────────────────────────────────────
function updateBadge(count) {
  try {
    if (count > 0) {
      ext.action.setBadgeText({ text: String(count) });
      ext.action.setBadgeBackgroundColor({ color: "#f6851b" });
    } else {
      ext.action.setBadgeText({ text: "" });
    }
  } catch { }
}

// ── Notifications ─────────────────────────────────────────────────────────────
function notifyApproval(method, origin) {
  try {
    ext.notifications.create("lqd_approval_" + Date.now(), {
      type: "basic",
      iconUrl: "icons/icon48.png",
      title: "LQD Wallet — Action Required",
      message: `${origin || "A dApp"} requests: ${method}. Open LQD Wallet to approve.`
    });
  } catch { }
}

// ── Allowlist ─────────────────────────────────────────────────────────────────

// Previously auto-trusted localhost origins — purge them from storage so
// they go through the normal approval popup instead of auto-approving.
const FORMERLY_TRUSTED_ORIGINS = [
  "http://localhost:3000", "http://localhost:3001", "http://localhost:5173",
  "http://127.0.0.1:3000", "http://127.0.0.1:3001", "http://127.0.0.1:5173"
];
ext.storage.local.get(["allowlist"]).then((data) => {
  const al = data.allowlist || {};
  let changed = false;
  for (const origin of FORMERLY_TRUSTED_ORIGINS) {
    if (al[origin]) { delete al[origin]; changed = true; }
  }
  if (changed) ext.storage.local.set({ allowlist: al });
}).catch(() => { });

async function getAllowlist() {
  const data = await ext.storage.local.get(["allowlist"]);
  return data.allowlist || {};
}
async function setAllowlist(al) {
  await ext.storage.local.set({ allowlist: al });
}

// ── Network management ────────────────────────────────────────────────────────
async function getNetworks() {
  const data = await ext.storage.local.get(["networks", "currentNetwork"]);
  const networks = data.networks || DEFAULT_NETWORKS;
  let changed = false;
  for (const [chainId, net] of Object.entries(networks)) {
    if (net?.nodeUrl && isLocalEndpoint(net.nodeUrl)) {
      net.nodeUrl = chainId === "0x8c" ? PROD_AGGREGATOR_URL : PROD_CHAIN_URL;
      changed = true;
    }
    if (net?.walletUrl && isLocalEndpoint(net.walletUrl)) {
      net.walletUrl = PROD_WALLET_URL;
      changed = true;
    }
    if (net?.blockExplorer && isLocalEndpoint(net.blockExplorer)) {
      net.blockExplorer = PROD_EXPLORER_URL;
      changed = true;
    }
  }
  if (changed) await saveNetworks(networks);
  const currentNetwork = data.currentNetwork || "0x8b";
  return { networks, currentNetwork };
}
async function saveNetworks(networks) {
  await ext.storage.local.set({ networks });
}
async function switchNetwork(chainId) {
  const { networks } = await getNetworks();
  const net = networks[chainId];
  if (!net) throw new Error(`Network ${chainId} not found`);
  session.chainId = chainId;
  session.nodeUrl = net.nodeUrl;
  session.walletUrl = net.walletUrl;
  await ext.storage.local.set({ currentNetwork: chainId });
  pushChainId();
  return net;
}
async function addNetwork(net) {
  if (!net.chainId || !net.nodeUrl || !net.name) throw new Error("Missing required fields: chainId, nodeUrl, name");
  const { networks } = await getNetworks();
  networks[net.chainId] = net;
  await saveNetworks(networks);
  return net;
}

// ── Config load ───────────────────────────────────────────────────────────────
async function loadConfig() {
  const { networks, currentNetwork } = await getNetworks();
  const net = networks[currentNetwork];
  if (net) {
    session.chainId = currentNetwork;
    session.nodeUrl = net.nodeUrl;
    session.walletUrl = net.walletUrl;
  }
  if (session.nodeUrl && isLocalEndpoint(session.nodeUrl)) {
    session.nodeUrl = currentNetwork === "0x8c" ? PROD_AGGREGATOR_URL : PROD_CHAIN_URL;
    await ext.storage.local.set({ nodeUrl: session.nodeUrl });
  }
  if (session.walletUrl && isLocalEndpoint(session.walletUrl)) {
    session.walletUrl = PROD_WALLET_URL;
    await ext.storage.local.set({ walletUrl: session.walletUrl });
  }
  const data = await ext.storage.local.get(["address"]);
  if (data.address) session.address = data.address;
}

function isLocalEndpoint(url = "") {
  return /^(https?:\/\/)?(localhost|127\.0\.0\.1)(:\d+)?/i.test(String(url).trim());
}

// ── Crypto helpers ────────────────────────────────────────────────────────────
function bufToHex(buffer) {
  return Array.from(new Uint8Array(buffer))
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");
}
function hexToBuf(hex) {
  const clean = hex.startsWith("0x") ? hex.slice(2) : hex;
  const bytes = new Uint8Array(clean.length / 2);
  for (let i = 0; i < bytes.length; i++)
    bytes[i] = parseInt(clean.substr(i * 2, 2), 16);
  return bytes.buffer;
}
async function deriveKey(password, saltHex) {
  const enc = new TextEncoder();
  const baseKey = await crypto.subtle.importKey(
    "raw", enc.encode(password), { name: "PBKDF2" }, false, ["deriveKey"]
  );
  return crypto.subtle.deriveKey(
    { name: "PBKDF2", salt: hexToBuf(saltHex), iterations: 100000, hash: "SHA-256" },
    baseKey,
    { name: "AES-GCM", length: 256 },
    false,
    ["encrypt", "decrypt"]
  );
}
async function encryptPrivateKey(privateKey, password) {
  const iv = crypto.getRandomValues(new Uint8Array(12));
  const salt = crypto.getRandomValues(new Uint8Array(16));
  const key = await deriveKey(password, bufToHex(salt));
  const cipher = await crypto.subtle.encrypt(
    { name: "AES-GCM", iv },
    key,
    new TextEncoder().encode(privateKey)
  );
  return { cipher: bufToHex(cipher), iv: bufToHex(iv), salt: bufToHex(salt) };
}
async function decryptPrivateKey(cipherHex, ivHex, saltHex, password) {
  const key = await deriveKey(password, saltHex);
  const plain = await crypto.subtle.decrypt(
    { name: "AES-GCM", iv: new Uint8Array(hexToBuf(ivHex)) },
    key,
    hexToBuf(cipherHex)
  );
  return new TextDecoder().decode(plain);
}

// ── Wallet storage ────────────────────────────────────────────────────────────
async function saveWallet({ address, privateKey, password }) {
  const enc = await encryptPrivateKey(privateKey, password);
  await ext.storage.local.set({
    address,
    walletCipher: enc.cipher, walletIv: enc.iv, walletSalt: enc.salt,
    locked: true
  });
  session.address = address;
}
async function saveMnemonic(mnemonic, password) {
  const enc = await encryptPrivateKey(mnemonic, password);
  await ext.storage.local.set({
    mnemonicCipher: enc.cipher, mnemonicIv: enc.iv, mnemonicSalt: enc.salt
  });
}
async function unlock(password) {
  const data = await ext.storage.local.get(["walletCipher", "walletIv", "walletSalt", "address"]);
  if (!data.walletCipher || !data.address) throw new Error("No wallet stored");
  const pk = await decryptPrivateKey(data.walletCipher, data.walletIv, data.walletSalt, password);
  session.privateKey = pk;
  session.address = data.address;
  session.unlocked = true;
  await ext.storage.local.set({ locked: false });
  // ── Persist session so it survives MV3 service-worker restarts ──────────
  // chrome.storage.session is in-memory, survives SW restarts within the same
  // Chrome session, and is automatically cleared when the browser closes.
  try {
    if (ext.storage.session) {
      await ext.storage.session.set({
        sw_unlocked: true,
        sw_address: data.address,
        sw_pk: pk
      });
    }
  } catch { }
  resetAutoLock();
  return session.address;
}
function lock() {
  session.privateKey = "";
  session.unlocked = false;
  ext.storage.local.set({ locked: true });
  // Clear persisted session on explicit lock
  try {
    if (ext.storage.session) {
      ext.storage.session.remove(["sw_unlocked", "sw_address", "sw_pk"]).catch(() => { });
    }
  } catch { }
}

// ── Session restore after SW restart ─────────────────────────────────────────
// Called before every handleRequest so the first request after a SW wake-up
// will see the correct unlocked state.
let _sessionRestored = false;
async function restoreSessionIfNeeded() {
  if (_sessionRestored || session.unlocked) { _sessionRestored = true; return; }
  try {
    if (!ext.storage.session) return;
    const d = await ext.storage.session.get(["sw_unlocked", "sw_address", "sw_pk"]);
    if (d.sw_unlocked && d.sw_address && d.sw_pk) {
      session.unlocked = true;
      session.address = d.sw_address;
      session.privateKey = d.sw_pk;
    }
  } catch { }
  _sessionRestored = true;
}

// ── HTTP helpers ──────────────────────────────────────────────────────────────
function extractErr(data, fallback) {
  if (!data) return fallback;
  return data.error || data.Error || data.message || data.msg || data.reason || fallback;
}
async function callNode(path, body) {
  let res;
  try {
    res = await fetch(session.nodeUrl + path, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body)
    });
  } catch (e) {
    throw new Error(`Cannot reach node at ${session.nodeUrl}: ${e.message}`);
  }
  const text = await res.text();
  let data; try { data = JSON.parse(text); } catch { data = { raw: text }; }
  if (!res.ok) throw new Error(extractErr(data, text || `Node error ${res.status}`));
  return data;
}
async function callNodeGet(path) {
  let res;
  try { res = await fetch(session.nodeUrl + path); }
  catch (e) { throw new Error(`Cannot reach node at ${session.nodeUrl}: ${e.message}`); }
  const text = await res.text();
  let data; try { data = JSON.parse(text); } catch { data = { raw: text }; }
  if (!res.ok) throw new Error(extractErr(data, text || `Node error ${res.status}`));
  return data;
}
async function callWallet(path, body) {
  let res;
  try {
    res = await fetch(session.walletUrl + path, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body)
    });
  } catch (e) {
    throw new Error(`Cannot reach wallet at ${session.walletUrl}: ${e.message}`);
  }
  const text = await res.text();
  let data; try { data = JSON.parse(text); } catch { data = { raw: text }; }
  if (!res.ok) throw new Error(extractErr(data, text || `Wallet error ${res.status}`));
  return data;
}
async function getBaseFee() {
  try {
    const res = await callNodeGet("/basefee");
    return Number(res.base_fee || res.baseFee || res.BaseFee || 10) || 10;
  } catch { return 10; }
}

// ── Activity log (200 entries) ────────────────────────────────────────────────
async function recordActivity(entry) {
  const data = await ext.storage.local.get(["txActivity"]);
  const list = Array.isArray(data.txActivity) ? data.txActivity : [];
  list.unshift({ ...entry, chainId: session.chainId, time: Date.now() });
  await ext.storage.local.set({ txActivity: list.slice(0, 200) });
}

// ── Push helpers ──────────────────────────────────────────────────────────────
function pushAccounts() {
  const accounts = session.unlocked && session.address ? [session.address] : [];
  for (const [, port] of portsByTab.entries()) {
    try { port.postMessage({ type: "LQD_PUSH", subtype: "LQD_ACCOUNTS", payload: accounts }); } catch { }
  }
}
function pushChainId() {
  for (const [, port] of portsByTab.entries()) {
    try { port.postMessage({ type: "LQD_PUSH", subtype: "LQD_CHAIN_ID", payload: session.chainId }); } catch { }
  }
}
async function storePendingList() {
  const list = Array.from(pending.values()).map((p) => ({
    id: p.id, method: p.method, params: p.params,
    tabId: p.tabId, createdAt: p.createdAt, origin: p.origin
  }));
  await ext.storage.local.set({ pendingRequests: list });
  updateBadge(list.length);
}

// ── Approval helpers ──────────────────────────────────────────────────────────
function needsApproval(method) {
  return ["lqd_connect", "lqd_requestAccounts", "lqd_sendTransaction", "lqd_contractTx"].includes(method);
}
function methodScope(method) {
  if (method === "lqd_connect" || method === "lqd_requestAccounts") return "connect";
  if (method === "lqd_sendTransaction") return "send";
  if (method === "lqd_contractTx") return "contract";
  return "other";
}

// ── Request handler ───────────────────────────────────────────────────────────
async function handleRequest({ id, method, params }) {
  await loadConfig();
  await restoreSessionIfNeeded(); // restore after MV3 service-worker restart
  switch (method) {

    case "lqd_requestAccounts":
    case "lqd_connect":
      if (!session.unlocked) throw new Error("Wallet locked. Please unlock LQD Wallet.");
      return { result: [session.address] };

    case "lqd_accounts":
      return { result: session.unlocked && session.address ? [session.address] : [] };

    case "lqd_chainId":
      return { result: session.chainId };

    case "lqd_networkVersion":
      return { result: parseInt(session.chainId, 16).toString() };

    case "lqd_getBalance": {
      const addr = (params && params[0]) || session.address;
      const res = await callNodeGet(`/balance?address=${encodeURIComponent(addr)}`);
      return { result: res.balance ?? res };
    }

    case "lqd_estimateGas": {
      const baseFee = await getBaseFee();
      const [payload] = params || [{}];
      const gasUnits = payload && payload.data ? 50000 : 21000;
      return { result: { gas: gasUnits, gas_price: baseFee, fee: gasUnits * baseFee } };
    }

    case "lqd_getTokenBalance": {
      const [token, addr] = params || [];
      const res = await callNode("/contract/call", {
        address: token, caller: addr || session.address,
        fn: "BalanceOf", args: [addr || session.address], value: 0
      });
      return { result: res.output || "0" };
    }

    case "lqd_contractCall": {
      const [payload] = params || [];
      const res = await callNode("/contract/call", payload);
      return { result: res };
    }

    case "lqd_getTransactionReceipt": {
      const [hash] = params || [];
      try {
        const res = await callNodeGet(`/tx/${encodeURIComponent(hash)}`);
        return { result: res };
      } catch { return { result: null }; }
    }

    case "lqd_getPrivateKey": {
      if (!session.unlocked || !session.privateKey) {
        throw new Error("Wallet locked");
      }
      return { result: session.privateKey };
    }

    case "lqd_deployBuiltin": {
      if (!session.unlocked || !session.privateKey) throw new Error("Wallet locked");
      const [payload] = params || [];
      if (!payload?.template) throw new Error("Missing template");
      const gas = payload.gas || 500000;
      const owner = payload.owner || session.address;
      const initArgs = Array.isArray(payload.init_args) ? payload.init_args : [];
      const res = await callNode("/contract/deploy-builtin", {
        template: payload.template,
        owner,
        private_key: session.privateKey,
        gas,
        init_args: initArgs
      });
      await recordActivity({
        type: "contract",
        tx_hash: res.tx_hash || res.TxHash || res.hash || "",
        to: res.address || res.contract_address || res.result?.address || "",
        function: `deploy:${payload.template}`,
        args: initArgs,
        value: "0"
      });
      return { result: res };
    }

    case "lqd_getNetworks": {
      const { networks, currentNetwork } = await getNetworks();
      return { result: { networks, currentNetwork } };
    }

    case "lqd_addNetwork": {
      const [net] = params || [];
      const added = await addNetwork(net);
      return { result: added };
    }

    case "lqd_switchNetwork": {
      const [chainId] = params || [];
      const net = await switchNetwork(chainId);
      return { result: net };
    }

    case "lqd_contractTx": {
      if (!session.unlocked) throw new Error("Wallet locked");
      const [payload] = params || [];
      const gp = payload.gas_price || await getBaseFee();
      const g = payload.gas || 50000;
      const res = await callWallet("/wallet/contract-template", {
        address: session.address,
        private_key: session.privateKey,
        contract_address: payload.contract_address,
        function: payload.function,
        args: payload.args || [],
        value: payload.value || "0",
        gas: g, gas_price: gp
      });
      const isTransfer = (payload.function || "").toLowerCase() === "transfer";
      await recordActivity({
        type: isTransfer ? "token" : "contract",
        tx_hash: res.tx_hash || res.TxHash || res.hash || "",
        to: isTransfer ? (payload.args || [])[0] : payload.contract_address,
        function: payload.function,
        args: payload.args || [],
        value: payload.value || "0",
        amount: isTransfer ? (payload.args || [])[1] : undefined
      });
      return { result: res };
    }

    case "lqd_sendTransaction": {
      if (!session.unlocked) throw new Error("Wallet locked");
      const [payload] = params || [];
      const gp = payload.gas_price || await getBaseFee();
      const g = payload.gas || 21000;
      const res = await callWallet("/wallet/send", {
        from: session.address,
        to: payload.to,
        value: String(payload.value ?? "0"),   // ← must be "value" (wallet server field name)
        private_key: session.privateKey,
        gas: g, gas_price: gp
      });
      await recordActivity({
        type: "send",
        tx_hash: res.tx_hash || res.TxHash || res.hash || "",
        to: payload.to,
        value: payload.value || "0",
        gas: g, gas_price: gp
      });
      return { result: res };
    }

    default:
      throw new Error(`Method not supported: ${method}`);
  }
}

// ── Port (dApp) connection ────────────────────────────────────────────────────
ext.runtime.onConnect.addListener((port) => {
  const tabId = port.sender?.tab?.id ?? null;
  const url = port.sender?.tab?.url || "";
  let origin = "";
  try { origin = url ? new URL(url).origin : ""; } catch { }
  if (tabId == null) return;
  portsByTab.set(tabId, port);
  port.onDisconnect.addListener(() => portsByTab.delete(tabId));

  // Push current state to the newly-connected tab (handles page reload).
  // restoreSessionIfNeeded() ensures the session is hydrated before we push.
  restoreSessionIfNeeded().then(async () => {
    const accounts = session.unlocked && session.address ? [session.address] : [];
    try { port.postMessage({ type: "LQD_PUSH", subtype: "LQD_ACCOUNTS", payload: accounts }); } catch { }
    try { port.postMessage({ type: "LQD_PUSH", subtype: "LQD_CHAIN_ID", payload: session.chainId }); } catch { }

    // Flush any pending results that were saved when this tab's port was gone (SW restart)
    try {
      if (ext.storage.session) {
        const d = await ext.storage.session.get(["pendingResults"]);
        const pr = d.pendingResults || {};
        const remaining = {};
        for (const [id, msg] of Object.entries(pr)) {
          // Only forward results for requests that came from this tab
          const savedReq = (await ext.storage.local.get(["pendingRequests"])).pendingRequests || [];
          const matchedReq = savedReq.find((r) => r.id === id && r.tabId === tabId);
          // Also try: if this is the reconnecting tab, forward any result for it
          try { port.postMessage(msg); } catch { remaining[id] = msg; }
        }
        if (Object.keys(remaining).length === 0) {
          await ext.storage.session.remove(["pendingResults"]);
        } else {
          await ext.storage.session.set({ pendingResults: remaining });
        }
      }
    } catch { }
  });

  port.onMessage.addListener((message) => {
    if (!message || message.type !== "LQD_REQUEST") return;
    resetAutoLock();
    const payload = message.payload || {};

    if (needsApproval(payload.method)) {
      getAllowlist().then((al) => {
        const scope = methodScope(payload.method);
        if (origin && al[origin]?.[scope]) {
          handleRequest(payload)
            .then((res) => port.postMessage({ type: "LQD_RESPONSE", id: payload.id, result: res.result }))
            .catch((err) => port.postMessage({ type: "LQD_RESPONSE", id: payload.id, error: err.message }));
          return;
        }
        const req = { id: payload.id, method: payload.method, params: payload.params, tabId, createdAt: Date.now(), origin };
        pending.set(payload.id, req);
        storePendingList();
        notifyApproval(payload.method, origin);
        // Do NOT send an immediate error — the Promise in the dApp keeps waiting.
        // The response is sent only when the user Approves or Denies in the popup.
        // Send a non-resolving "pending" push so the dApp can show a waiting state.
        try { port.postMessage({ type: "LQD_PUSH", subtype: "LQD_APPROVAL_PENDING", payload: { id: payload.id, method: payload.method } }); } catch { }
      });
      return;
    }

    handleRequest(payload)
      .then((res) => port.postMessage({ type: "LQD_RESPONSE", id: payload.id, result: res.result }))
      .catch((err) => port.postMessage({ type: "LQD_RESPONSE", id: payload.id, error: err.message }));
  });
});

// ── Internal (popup) messages ─────────────────────────────────────────────────
ext.runtime.onMessage.addListener((message, sender, sendResponse) => {
  if (!message) return;

  if (message.type === "LQD_REQUEST") {
    Promise.resolve()
      .then(() => handleRequest(message.payload || {}))
      .then((res) => sendResponse({ result: res.result }))
      .catch((err) => sendResponse({ error: err?.message || String(err) }));
    return true;
  }
  if (message.type === "LQD_UNLOCK") {
    unlock(message.password)
      .then((addr) => { sendResponse({ ok: true, address: addr }); pushAccounts(); })
      .catch((err) => sendResponse({ ok: false, error: err.message }));
    return true;
  }
  if (message.type === "LQD_LOCK") {
    lock(); sendResponse({ ok: true }); pushAccounts();
    return false;
  }
  if (message.type === "LQD_IMPORT") {
    saveWallet(message.payload)
      .then(() => sendResponse({ ok: true }))
      .catch((err) => sendResponse({ ok: false, error: err.message }));
    return true;
  }
  if (message.type === "LQD_STORE_MNEMONIC") {
    saveMnemonic(message.mnemonic, message.password)
      .then(() => sendResponse({ ok: true }))
      .catch((err) => sendResponse({ ok: false, error: err.message }));
    return true;
  }
  if (message.type === "LQD_REVEAL_SECRET") {
    const kind = message.kind;
    const keys = kind === "mnemonic"
      ? ["mnemonicCipher", "mnemonicIv", "mnemonicSalt"]
      : ["walletCipher", "walletIv", "walletSalt"];
    ext.storage.local.get(keys).then(async (data) => {
      try {
        const c = kind === "mnemonic" ? data.mnemonicCipher : data.walletCipher;
        const iv = kind === "mnemonic" ? data.mnemonicIv : data.walletIv;
        const s = kind === "mnemonic" ? data.mnemonicSalt : data.walletSalt;
        if (!c || !iv || !s) throw new Error("Secret not stored");
        const secret = await decryptPrivateKey(c, iv, s, message.password);
        sendResponse({ ok: true, secret });
      } catch (e) { sendResponse({ ok: false, error: e.message }); }
    });
    return true;
  }
  if (message.type === "LQD_GET_ACTIVITY") {
    ext.storage.local.get(["txActivity"]).then((data) => {
      sendResponse({ ok: true, list: data.txActivity || [] });
    });
    return true;
  }
  if (message.type === "LQD_GET_PENDING") {
    ext.storage.local.get(["pendingRequests"]).then((data) => {
      sendResponse({ ok: true, list: data.pendingRequests || [] });
    });
    return true;
  }
  if (message.type === "LQD_APPROVE") {
    // Restore from storage if SW was restarted and pending Map was cleared
    Promise.resolve().then(async () => {
      let req = pending.get(message.id);
      if (!req) {
        const stored = await ext.storage.local.get(["pendingRequests"]);
        const list = stored.pendingRequests || [];
        req = list.find((p) => p.id === message.id);
      }
      if (!req) { sendResponse({ ok: false, error: "Request not found" }); return; }
      pending.delete(message.id);
      storePendingList();
      const port = portsByTab.get(req.tabId);

      // Helper: send result to DApp via port; if port is gone, save to session for reconnect
      async function deliverResult(resMsg) {
        if (port) {
          try { port.postMessage(resMsg); return; } catch { }
        }
        // Port gone (SW was restarted) — save result to session storage so content.js
        // can pick it up on its next reconnect/check.
        try {
          if (ext.storage.session) {
            const d = await ext.storage.session.get(["pendingResults"]);
            const pr = d.pendingResults || {};
            pr[resMsg.id] = resMsg;
            await ext.storage.session.set({ pendingResults: pr });
          }
        } catch { }
      }

      if (!message.allow) {
        await deliverResult({ type: "LQD_RESPONSE", id: req.id, error: "User rejected" });
        sendResponse({ ok: true });
        return;
      }
      if (message.remember && req.origin) {
        getAllowlist().then((al) => {
          const scope = methodScope(req.method);
          al[req.origin] = al[req.origin] || {};
          al[req.origin][scope] = true;
          return setAllowlist(al);
        });
      }
      resetAutoLock();
      handleRequest({ id: req.id, method: req.method, params: req.params })
        .then(async (res) => {
          await deliverResult({ type: "LQD_RESPONSE", id: req.id, result: res.result });
          if (req.method === "lqd_connect" || req.method === "lqd_requestAccounts") {
            pushAccounts();
          }
          sendResponse({ ok: true });
        })
        .catch(async (err) => {
          await deliverResult({ type: "LQD_RESPONSE", id: req.id, error: err.message });
          sendResponse({ ok: false, error: err.message });
        });
    });
    return true;
  }
  if (message.type === "LQD_GET_ALLOWLIST") {
    getAllowlist().then((al) => sendResponse({ ok: true, allowlist: al }));
    return true;
  }
  if (message.type === "LQD_REMOVE_ALLOW") {
    getAllowlist().then((al) => {
      if (al[message.origin]) {
        delete al[message.origin][message.scope];
        if (Object.keys(al[message.origin]).length === 0) delete al[message.origin];
      }
      return setAllowlist(al).then(() => sendResponse({ ok: true }));
    });
    return true;
  }
  if (message.type === "LQD_GET_NETWORKS") {
    getNetworks().then(({ networks, currentNetwork }) =>
      sendResponse({ ok: true, networks, currentNetwork })
    );
    return true;
  }
  if (message.type === "LQD_SWITCH_NETWORK") {
    switchNetwork(message.chainId)
      .then((net) => { pushChainId(); sendResponse({ ok: true, network: net, chainId: message.chainId }); })
      .catch((err) => sendResponse({ ok: false, error: err.message }));
    return true;
  }
  if (message.type === "LQD_ADD_NETWORK") {
    addNetwork(message.network)
      .then((net) => sendResponse({ ok: true, network: net }))
      .catch((err) => sendResponse({ ok: false, error: err.message }));
    return true;
  }
  if (message.type === "LQD_REMOVE_NETWORK") {
    getNetworks().then(async ({ networks, currentNetwork }) => {
      if (!networks[message.chainId]) {
        return sendResponse({ ok: false, error: "Network not found" });
      }
      if (message.chainId === currentNetwork) {
        return sendResponse({ ok: false, error: "Cannot remove current network" });
      }
      if (DEFAULT_NETWORKS[message.chainId]) {
        return sendResponse({ ok: false, error: "Cannot remove default network" });
      }
      delete networks[message.chainId];
      await saveNetworks(networks);
      sendResponse({ ok: true });
    });
    return true;
  }
  if (message.type === "LQD_SET_ENDPOINTS") {
    loadConfig().then(async () => {
      const { networks, currentNetwork } = await getNetworks();
      networks[currentNetwork] = {
        ...networks[currentNetwork],
        nodeUrl: message.nodeUrl,
        walletUrl: message.walletUrl
      };
      await saveNetworks(networks);
      session.nodeUrl = message.nodeUrl;
      session.walletUrl = message.walletUrl;
      resetAutoLock();
      sendResponse({ ok: true });
    });
    return true;
  }
});

// ── Alarms ────────────────────────────────────────────────────────────────────
ext.alarms.onAlarm.addListener((alarm) => {
  if (alarm.name === "autoLock") { lock(); pushAccounts(); }
});

// ── Init ──────────────────────────────────────────────────────────────────────
loadConfig();
