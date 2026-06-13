import { STORAGE_KEYS, loadJSON, saveJSON } from "./storage";

export const CURRENT_SCHEMA_VERSION = 3;

export async function runWalletMigrations() {
  const previous = Number(await loadJSON(STORAGE_KEYS.schemaVersion, 0));
  if (previous >= CURRENT_SCHEMA_VERSION) {
    return { previous, current: CURRENT_SCHEMA_VERSION, changed: false };
  }

  if (previous < 1) {
    const settings = await loadJSON(STORAGE_KEYS.settings, {});
    await saveJSON(STORAGE_KEYS.settings, {
      autoRefresh: settings?.autoRefresh !== false,
      biometricEnabled: settings?.biometricEnabled !== false,
      telemetryEnabled: Boolean(settings?.telemetryEnabled),
      txTrackingEnabled: settings?.txTrackingEnabled !== false,
    });
  }

  if (previous < 2) {
    const pending = await loadJSON(STORAGE_KEYS.pendingTransactions, []);
    await saveJSON(
      STORAGE_KEYS.pendingTransactions,
      Array.isArray(pending)
        ? pending.filter((tx) => tx && tx.hash && tx.status !== "confirmed").slice(0, 50)
        : []
    );
  }

  if (previous < 3) {
    const settings = await loadJSON(STORAGE_KEYS.settings, {});
    await saveJSON(STORAGE_KEYS.settings, {
      autoRefresh: settings?.autoRefresh !== false,
      biometricEnabled: settings?.biometricEnabled !== false,
      telemetryEnabled: Boolean(settings?.telemetryEnabled),
      txTrackingEnabled: settings?.txTrackingEnabled !== false,
      backgroundTxTrackingEnabled: settings?.backgroundTxTrackingEnabled !== false,
      slowNetworkMode: settings?.slowNetworkMode !== false,
      storageAuditEnabled: settings?.storageAuditEnabled !== false,
      nonEvmSigningProof: settings?.nonEvmSigningProof !== false,
    });
  }

  await saveJSON(STORAGE_KEYS.schemaVersion, CURRENT_SCHEMA_VERSION);
  return { previous, current: CURRENT_SCHEMA_VERSION, changed: true };
}
