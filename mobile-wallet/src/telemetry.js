import { STORAGE_KEYS, loadJSON, saveJSON } from "./storage";

const MAX_EVENTS = 120;

function clean(value) {
  if (value === null || value === undefined) return "";
  const text = String(value);
  return text.length > 500 ? `${text.slice(0, 500)}...` : text;
}

async function telemetryEnabled() {
  const settings = await loadJSON(STORAGE_KEYS.settings, {});
  return Boolean(settings?.telemetryEnabled);
}

export async function recordTelemetry(event, payload = {}, force = false) {
  if (!force && !(await telemetryEnabled())) return;
  const current = await loadJSON(STORAGE_KEYS.telemetryEvents, []);
  const entry = {
    event: clean(event),
    payload: Object.fromEntries(
      Object.entries(payload || {}).map(([key, value]) => [key, clean(value)])
    ),
    timestamp: Date.now(),
  };
  await saveJSON(STORAGE_KEYS.telemetryEvents, [entry, ...(Array.isArray(current) ? current : [])].slice(0, MAX_EVENTS));
}

export async function recordError(scope, error, context = {}) {
  await recordTelemetry(
    "error",
    {
      scope,
      message: error?.message || error,
      stack: error?.stack || "",
      ...context,
    },
    true
  );
}

export function installGlobalErrorReporter(context = {}) {
  const globalHandler = globalThis?.ErrorUtils;
  if (!globalHandler || typeof globalHandler.getGlobalHandler !== "function") return () => {};
  const previous = globalHandler.getGlobalHandler();
  globalHandler.setGlobalHandler((error, isFatal) => {
    recordError("global", error, { ...context, fatal: Boolean(isFatal) }).catch(() => {});
    if (typeof previous === "function") previous(error, isFatal);
  });
  return () => {
    if (typeof previous === "function") globalHandler.setGlobalHandler(previous);
  };
}
