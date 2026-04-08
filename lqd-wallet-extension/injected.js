(() => {
  if (window.lqd) return;

  const eventListeners = {};
  let accounts = [];
  let chainId = "0x8b";

  // Track pending requests so we can reject them if the port disconnects
  const pendingResolvers = new Map(); // id → { resolve, reject, onPending }

  function send(type, payload) {
    window.postMessage({ __LQD_EXT__: true, type, payload }, "*");
  }

  function emit(event, data) {
    const handlers = eventListeners[event];
    if (!handlers) return;
    handlers.forEach((fn) => { try { fn(data); } catch {} });
  }

  window.addEventListener("message", (event) => {
    const msg = event.data;
    if (!msg || msg.__LQD_EXT__ !== true) return;

    // ── Final response (approve / deny / direct result) ────────────────────
    if (msg.type === "LQD_RESPONSE") {
      const entry = pendingResolvers.get(msg.id);
      if (!entry) return;
      pendingResolvers.delete(msg.id);
      if (msg.error) entry.reject(new Error(msg.error));
      else entry.resolve(msg.result);
      return;
    }

    // ── Approval pending — keep the Promise waiting, notify dApp ──────────
    if (msg.type === "LQD_APPROVAL_PENDING") {
      const entry = pendingResolvers.get(msg.payload?.id);
      if (entry && entry.onPending) {
        try { entry.onPending(); } catch {}
      }
      // Emit an event so dApps can show "waiting for approval" UI
      emit("approvalPending", { id: msg.payload?.id, method: msg.payload?.method });
      return;
    }

    // ── Push events from background ────────────────────────────────────────
    if (msg.type === "LQD_ACCOUNTS") {
      const next = Array.isArray(msg.payload) ? msg.payload : [];
      if (JSON.stringify(next) !== JSON.stringify(accounts)) {
        accounts = next;
        emit("accountsChanged", accounts);
      }
    }
    if (msg.type === "LQD_CHAIN_ID") {
      const next = msg.payload || chainId;
      if (next !== chainId) {
        chainId = next;
        emit("chainChanged", chainId);
        emit("networkChanged", parseInt(chainId, 16).toString());
      }
    }
  });

  function genId() {
    try { return crypto.randomUUID(); } catch {}
    return Math.random().toString(16).slice(2) + Date.now().toString(16);
  }

  const provider = {
    isLQD: true,

    isConnected() { return accounts.length > 0; },

    /**
     * request({ method, params, onPending })
     *
     * onPending is an optional callback fired when approval is required but
     * the user hasn't acted yet — useful for showing "Waiting for wallet…" UI.
     * The Promise stays open until the user approves or denies in the popup.
     */
    request({ method, params, onPending } = {}) {
      return new Promise((resolve, reject) => {
        const id = genId();
        pendingResolvers.set(id, { resolve, reject, onPending: onPending || null });
        send("LQD_REQUEST", { id, method, params });
      });
    },

    on(event, handler) {
      if (!eventListeners[event]) eventListeners[event] = new Set();
      eventListeners[event].add(handler);
    },

    removeListener(event, handler) {
      if (eventListeners[event]) eventListeners[event].delete(handler);
    },

    off(event, handler) {
      if (eventListeners[event]) eventListeners[event].delete(handler);
    },

    once(event, handler) {
      const wrap = (data) => { handler(data); provider.removeListener(event, wrap); };
      provider.on(event, wrap);
    },

    get accounts()         { return [...accounts]; },
    get chainId()          { return chainId; },
    get selectedAddress()  { return accounts[0] || null; },
    get networkVersion()   { return parseInt(chainId, 16).toString(); },
  };

  window.lqd = provider;
  window.dispatchEvent(new CustomEvent("lqd#initialized", { detail: provider }));
})();
