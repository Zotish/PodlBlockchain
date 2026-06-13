import * as SecureStore from "expo-secure-store";
import AsyncStorage from "@react-native-async-storage/async-storage";
import { Platform } from "react-native";
import { safeJsonParse } from "./utils";

export const STORAGE_KEYS = {
  vault: "lqd_mobile_vault_v1",
  networks: "lqd_mobile_networks_v1",
  activeNetworkId: "lqd_mobile_active_network_v1",
  endpoints: "lqd_mobile_endpoints_v1",
  watchlist: "lqd_mobile_watchlist_v1",
  activity: "lqd_mobile_activity_v1",
  settings: "lqd_mobile_settings_v1",
  factory: "lqd_mobile_factory_v1",
  bridgeChainId: "lqd_mobile_bridge_chain_v1",
  approvals: "lqd_mobile_approvals_v1",
  trustedOrigins: "lqd_mobile_trusted_origins_v1",
  biometricVault: "lqd_mobile_biometric_vault_v1",
  backup: "lqd_mobile_backup_v1",
  watchAddresses: "lqd_mobile_watch_addresses_v1",
  hiddenTokens: "lqd_mobile_hidden_tokens_v1",
  removedTokens: "lqd_mobile_removed_tokens_v1",
  legalRiskAccepted: "lqd_mobile_legal_risk_accepted_v1",
  pendingTransactions: "lqd_mobile_pending_transactions_v1",
  telemetryEvents: "lqd_mobile_telemetry_events_v1",
  securityAudit: "lqd_mobile_security_audit_v1",
  schemaVersion: "lqd_mobile_schema_version_v1",
};

// Sensitive keys that MUST stay in SecureStore
const SENSITIVE_KEYS = [STORAGE_KEYS.vault, STORAGE_KEYS.biometricVault, STORAGE_KEYS.backup];
const SENSITIVE_PREFIXES = ["pk_"];

function secureStoreOptions(options = {}) {
  if (Platform.OS === "ios" && SecureStore.AFTER_FIRST_UNLOCK_THIS_DEVICE_ONLY && !options.keychainAccessible) {
    return { ...options, keychainAccessible: SecureStore.AFTER_FIRST_UNLOCK_THIS_DEVICE_ONLY };
  }
  return options;
}

function hasWebStorage() {
  return Platform.OS === "web" && typeof globalThis !== "undefined" && Boolean(globalThis.localStorage);
}

async function getItem(key, options = {}) {
  if (hasWebStorage()) {
    return globalThis.localStorage.getItem(key);
  }

  // Use SecureStore only for sensitive keys
  if (SENSITIVE_KEYS.includes(key)) {
    return SecureStore.getItemAsync(key, secureStoreOptions(options));
  }

  // Use AsyncStorage for everything else (large data)
  return AsyncStorage.getItem(key);
}

async function setItem(key, value, options = {}) {
  if (hasWebStorage()) {
    globalThis.localStorage.setItem(key, value);
    return;
  }

  if (SENSITIVE_KEYS.includes(key)) {
    await SecureStore.setItemAsync(key, value, secureStoreOptions(options));
    return;
  }

  await AsyncStorage.setItem(key, value);
}

async function deleteItem(key) {
  if (hasWebStorage()) {
    globalThis.localStorage.removeItem(key);
    return;
  }

  if (SENSITIVE_KEYS.includes(key)) {
    await SecureStore.deleteItemAsync(key);
    return;
  }

  await AsyncStorage.removeItem(key);
}

export async function loadJSON(key, fallback = null, options = {}) {
  const raw = await getItem(key, options);
  const parsed = safeJsonParse(raw, undefined);
  return parsed === undefined ? fallback : parsed;
}

export async function saveJSON(key, value, options = {}) {
  await setItem(key, JSON.stringify(value), options);
}

export async function removeItem(key) {
  await deleteItem(key);
}

export async function loadString(key, fallback = "", options = {}) {
  const raw = await getItem(key, options);
  return raw ?? fallback;
}

export async function saveString(key, value, options = {}) {
  await setItem(key, String(value), options);
}

function isSensitiveStorageKey(key) {
  return SENSITIVE_KEYS.includes(key) || SENSITIVE_PREFIXES.some((prefix) => String(key || "").startsWith(prefix));
}

export async function runSecureStorageAudit({ repair = true } = {}) {
  const startedAt = Date.now();
  const result = {
    ok: true,
    platform: Platform.OS,
    secureStoreAvailable: false,
    repairedKeys: [],
    leakedKeys: [],
    warnings: [],
    checkedAt: startedAt,
  };

  if (hasWebStorage()) {
    result.secureStoreAvailable = false;
    result.warnings.push("Web build uses localStorage fallback; do not use it for production key custody.");
    result.ok = false;
    globalThis.localStorage.setItem(STORAGE_KEYS.securityAudit, JSON.stringify(result));
    return result;
  }

  try {
    result.secureStoreAvailable = typeof SecureStore.isAvailableAsync === "function"
      ? await SecureStore.isAvailableAsync()
      : true;
  } catch {
    result.secureStoreAvailable = false;
  }

  if (!result.secureStoreAvailable) {
    result.ok = false;
    result.warnings.push("SecureStore is not available on this device.");
  }

  let asyncKeys = [];
  try {
    asyncKeys = await AsyncStorage.getAllKeys();
  } catch (error) {
    result.ok = false;
    result.warnings.push(`AsyncStorage audit failed: ${error?.message || "unknown error"}`);
  }

  for (const key of asyncKeys.filter(isSensitiveStorageKey)) {
    result.leakedKeys.push(key);
    result.ok = false;
    if (!repair || !result.secureStoreAvailable) continue;
    try {
      const leakedValue = await AsyncStorage.getItem(key);
      if (leakedValue !== null && leakedValue !== undefined) {
        const secureValue = await SecureStore.getItemAsync(key, secureStoreOptions());
        if (!secureValue) {
          await SecureStore.setItemAsync(key, leakedValue, secureStoreOptions());
        }
        await AsyncStorage.removeItem(key);
        result.repairedKeys.push(key);
      }
    } catch (error) {
      result.warnings.push(`Could not repair ${key}: ${error?.message || "unknown error"}`);
    }
  }

  if (result.repairedKeys.length === result.leakedKeys.length) {
    result.ok = result.warnings.length === 0 && result.secureStoreAvailable;
  }

  await AsyncStorage.setItem(STORAGE_KEYS.securityAudit, JSON.stringify(result));
  return result;
}
