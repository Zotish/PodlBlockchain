const ext = typeof chrome !== "undefined" ? chrome : browser;

(function inject() {
  const s = document.createElement("script");
  s.src = ext.runtime.getURL("injected.js");
  s.onload = () => s.remove();
  (document.head || document.documentElement).appendChild(s);
})();

let port = null;

function ensurePort() {
  if (port) return port;
  port = ext.runtime.connect({ name: "lqd" });
  port.onDisconnect.addListener(() => {
    port = null;
  });
  port.onMessage.addListener((message) => {
    if (!message) return;
    if (message.type === "LQD_RESPONSE") {
      window.postMessage({ __LQD_EXT__: true, type: "LQD_RESPONSE", id: message.id, result: message.result, error: message.error }, "*");
    }
    if (message.type === "LQD_PUSH") {
      window.postMessage({ __LQD_EXT__: true, type: message.subtype, payload: message.payload }, "*");
    }
  });
  return port;
}

window.addEventListener("message", (event) => {
  const msg = event.data;
  if (!msg || msg.__LQD_EXT__ !== true) return;
  if (msg.type !== "LQD_REQUEST") return;
  const p = ensurePort();
  p.postMessage({ type: "LQD_REQUEST", payload: msg.payload });
});
