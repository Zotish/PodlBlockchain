import * as SecureStore from "expo-secure-store";
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
};

export async function loadJSON(key, fallback = null, options = {}) {
  const raw = await SecureStore.getItemAsync(key, options);
  const parsed = safeJsonParse(raw, undefined);
  return parsed === undefined ? fallback : parsed;
}

export async function saveJSON(key, value, options = {}) {
  await SecureStore.setItemAsync(key, JSON.stringify(value), options);
}

export async function removeItem(key) {
  await SecureStore.deleteItemAsync(key);
}

export async function loadString(key, fallback = "", options = {}) {
  const raw = await SecureStore.getItemAsync(key, options);
  return raw ?? fallback;
}

export async function saveString(key, value, options = {}) {
  await SecureStore.setItemAsync(key, String(value), options);
}
