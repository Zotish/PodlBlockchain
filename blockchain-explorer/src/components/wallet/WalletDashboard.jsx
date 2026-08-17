import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import WalletLogin from "./WalletLogin";
import WalletBalance from "./WalletBalance";
import SendTransaction from "./SendTransaction";
import ReceiveSection from "./ReceiveSection";
import TransactionHistory from "./TransactionHistory";
import ContractManager from "../contracts/ContractManager";
import LiquidityDashboard from "./LiquidityDashboard";
import BridgePanel from "./BridgePanel";
import "./Wallet.css";

const STORAGE_KEY = "liquidityChainWallet";
const TOKENS_STORAGE_KEY = "liquidity_tokens_v1";
const INACTIVITY_LIMIT = 60 * 1000;
const encoder = new TextEncoder();
const decoder = new TextDecoder();

const TABS = [
  { id: "balance", label: "Portfolio", hint: "Balances & assets" },
  { id: "send", label: "Send", hint: "Transfer LQD" },
  { id: "receive", label: "Receive", hint: "Address & QR" },
  { id: "history", label: "Activity", hint: "Transaction history" },
  { id: "contracts", label: "Contracts", hint: "Deploy & interact" },
  { id: "liquidity", label: "Liquidity", hint: "PoDL positions" },
  { id: "bridge", label: "Bridge", hint: "Cross-chain tools" },
  { id: "settings", label: "Security", hint: "Backup & vault" },
];

const shortAddress = (value) => (value ? `${value.slice(0, 10)}…${value.slice(-8)}` : "—");

async function deriveKey(password, saltBytes) {
  const keyMaterial = await window.crypto.subtle.importKey(
    "raw",
    encoder.encode(password),
    "PBKDF2",
    false,
    ["deriveKey"]
  );

  return window.crypto.subtle.deriveKey(
    { name: "PBKDF2", salt: saltBytes, iterations: 250000, hash: "SHA-256" },
    keyMaterial,
    { name: "AES-GCM", length: 256 },
    false,
    ["encrypt", "decrypt"]
  );
}

function encodeBase64(bytes) {
  return btoa(String.fromCharCode(...new Uint8Array(bytes)));
}

function decodeBase64(value) {
  return Uint8Array.from(atob(value), (character) => character.charCodeAt(0));
}

async function encryptPrivateKey(password, privateKey) {
  const salt = window.crypto.getRandomValues(new Uint8Array(16));
  const iv = window.crypto.getRandomValues(new Uint8Array(12));
  const key = await deriveKey(password, salt);
  const ciphertext = await window.crypto.subtle.encrypt(
    { name: "AES-GCM", iv },
    key,
    encoder.encode(privateKey)
  );

  return {
    encryptedPrivateKey: encodeBase64(ciphertext),
    salt: encodeBase64(salt),
    iv: encodeBase64(iv),
  };
}

async function decryptPrivateKey(password, encryptedBundle) {
  const key = await deriveKey(password, decodeBase64(encryptedBundle.salt));
  const plaintext = await window.crypto.subtle.decrypt(
    { name: "AES-GCM", iv: decodeBase64(encryptedBundle.iv) },
    key,
    decodeBase64(encryptedBundle.encryptedPrivateKey)
  );
  return decoder.decode(plaintext);
}

export function getTrustedWalletConnectOrigin(referrer = "", currentOrigin = "") {
  const configured = String(
    import.meta.env.REACT_APP_DEX_APP_ORIGIN || import.meta.env.VITE_DEX_APP_ORIGIN || ""
  )
    .split(",")
    .map((value) => value.trim().replace(/\/+$/, ""))
    .filter(Boolean);

  if (/^https?:\/\/(localhost|127\.0\.0\.1)(:\d+)?$/.test(currentOrigin)) {
    configured.push("http://localhost:3000", "http://127.0.0.1:3000");
  }

  let requestingOrigin = "";
  try {
    requestingOrigin = referrer ? new URL(referrer).origin : "";
  } catch {
    requestingOrigin = "";
  }

  return configured.includes(requestingOrigin) ? requestingOrigin : "";
}

const WalletDashboard = () => {
  const navigate = useNavigate();
  const inactivityRef = useRef(null);
  const backupInputRef = useRef(null);
  const [activeTab, setActiveTab] = useState("balance");
  const [walletAddress, setWalletAddress] = useState("");
  const [privateKey, setPrivateKey] = useState("");
  const [isWalletLoaded, setIsWalletLoaded] = useState(false);
  const [savedWalletMeta, setSavedWalletMeta] = useState(null);
  const [unlockPassword, setUnlockPassword] = useState("");
  const [unlockError, setUnlockError] = useState("");
  const [backupMessage, setBackupMessage] = useState("");
  const [connectionMessage, setConnectionMessage] = useState("");
  const [recoveryPhrase, setRecoveryPhrase] = useState("");
  const [recoveryAcknowledged, setRecoveryAcknowledged] = useState(false);
  const [copied, setCopied] = useState(false);

  const requestingOrigin = useMemo(
    () =>
      typeof window === "undefined"
        ? ""
        : getTrustedWalletConnectOrigin(document.referrer, window.location.origin),
    []
  );

  const lockWallet = useCallback(() => {
    setPrivateKey("");
    setIsWalletLoaded(false);
    setUnlockPassword("");
    setUnlockError("");
    setConnectionMessage("");
  }, []);

  const forgetWallet = useCallback(() => {
    setWalletAddress("");
    setPrivateKey("");
    setIsWalletLoaded(false);
    setSavedWalletMeta(null);
    setUnlockPassword("");
    setUnlockError("");
    setRecoveryPhrase("");
    setRecoveryAcknowledged(false);
    localStorage.removeItem(STORAGE_KEY);
  }, []);

  const resetInactivityTimer = useCallback(() => {
    if (!isWalletLoaded) return;
    if (inactivityRef.current) window.clearTimeout(inactivityRef.current);
    inactivityRef.current = window.setTimeout(lockWallet, INACTIVITY_LIMIT);
  }, [isWalletLoaded, lockWallet]);

  useEffect(() => {
    if (!isWalletLoaded) return undefined;
    resetInactivityTimer();
    const events = ["mousemove", "keydown", "click", "touchstart"];
    events.forEach((event) => window.addEventListener(event, resetInactivityTimer, { passive: true }));
    return () => {
      if (inactivityRef.current) window.clearTimeout(inactivityRef.current);
      events.forEach((event) => window.removeEventListener(event, resetInactivityTimer));
    };
  }, [isWalletLoaded, resetInactivityTimer]);

  useEffect(() => {
    const saved = localStorage.getItem(STORAGE_KEY);
    if (!saved) return;
    try {
      const parsed = JSON.parse(saved);
      if (parsed.privateKey && !parsed.encryptedPrivateKey) {
        localStorage.removeItem(STORAGE_KEY);
        return;
      }
      if (!parsed.address || !parsed.encryptedPrivateKey || !parsed.salt || !parsed.iv) {
        localStorage.removeItem(STORAGE_KEY);
        return;
      }
      setSavedWalletMeta(parsed);
      setWalletAddress(parsed.address);
    } catch {
      localStorage.removeItem(STORAGE_KEY);
    }
  }, []);

  const persistEncryptedWallet = useCallback((address, encryptedBundle) => {
    const metadata = { address, ...encryptedBundle, vaultVersion: 2 };
    localStorage.setItem(STORAGE_KEY, JSON.stringify(metadata));
    setSavedWalletMeta(metadata);
  }, []);

  const handleWalletCreate = async (walletData, password) => {
    try {
      const bundle = await encryptPrivateKey(password, walletData.private_key);
      persistEncryptedWallet(walletData.address, bundle);
      setWalletAddress(walletData.address);
      setPrivateKey(walletData.private_key);
      setRecoveryPhrase(String(walletData.mnemonic || "").trim());
      setRecoveryAcknowledged(false);
      setIsWalletLoaded(true);
      setUnlockError("");
    } catch {
      setUnlockError("The new wallet could not be encrypted in this browser.");
    }
  };

  const handleWalletImport = async (walletData, password) => {
    try {
      const bundle = await encryptPrivateKey(password, walletData.private_key);
      persistEncryptedWallet(walletData.address, bundle);
      setWalletAddress(walletData.address);
      setPrivateKey(walletData.private_key);
      setRecoveryPhrase("");
      setRecoveryAcknowledged(false);
      setIsWalletLoaded(true);
      setUnlockError("");
    } catch {
      setUnlockError("The imported wallet could not be encrypted in this browser.");
    }
  };

  const handleUnlock = async (event) => {
    event.preventDefault();
    if (!savedWalletMeta) return;
    setUnlockError("");
    try {
      const unlockedPrivateKey = await decryptPrivateKey(unlockPassword, savedWalletMeta);
      setPrivateKey(unlockedPrivateKey);
      setWalletAddress(savedWalletMeta.address);
      setIsWalletLoaded(true);
      setUnlockPassword("");
    } catch {
      setUnlockError("Incorrect password or damaged encrypted wallet data.");
    }
  };

  const copyAddress = async () => {
    await navigator.clipboard.writeText(walletAddress);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1200);
  };

  const connectTrustedDex = useCallback(() => {
    if (!walletAddress || !privateKey) return;
    if (!requestingOrigin) {
      setConnectionMessage("Open this wallet from the configured PoDL DEX before connecting a session.");
      return;
    }

    const payload = { type: "LQD_WALLET_CONNECT", address: walletAddress, privateKey };
    let delivered = false;
    if (window.opener && !window.opener.closed) {
      window.opener.postMessage(payload, requestingOrigin);
      delivered = true;
    }
    if (window.parent && window.parent !== window) {
      window.parent.postMessage(payload, requestingOrigin);
      delivered = true;
    }
    setConnectionMessage(
      delivered
        ? `Wallet session connected to ${requestingOrigin}`
        : "No trusted DEX request window is available."
    );
  }, [privateKey, requestingOrigin, walletAddress]);

  const exportBackup = useCallback(() => {
    if (!walletAddress) return;
    const walletRaw = localStorage.getItem(STORAGE_KEY);
    const tokensRaw = localStorage.getItem(`${TOKENS_STORAGE_KEY}_${walletAddress.toLowerCase()}`) || "[]";
    let wallet = { address: walletAddress };
    let tokens = [];
    try { wallet = walletRaw ? JSON.parse(walletRaw) : wallet; } catch { /* keep safe fallback */ }
    try { tokens = JSON.parse(tokensRaw); } catch { /* keep empty list */ }

    const backup = {
      exportedAt: new Date().toISOString(),
      wallet,
      tokens,
      app: "podl-explorer-wallet",
      version: 2,
    };
    const url = URL.createObjectURL(new Blob([JSON.stringify(backup, null, 2)], { type: "application/json" }));
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = `podl-wallet-backup-${walletAddress.slice(2, 8)}.json`;
    anchor.click();
    URL.revokeObjectURL(url);
    setBackupMessage("Encrypted backup exported successfully.");
  }, [walletAddress]);

  const importBackupFile = useCallback(async (file) => {
    if (!file) return;
    const parsed = JSON.parse(await file.text());
    const wallet = parsed.wallet || parsed;
    if (!wallet?.address || !wallet?.encryptedPrivateKey || !wallet?.salt || !wallet?.iv) {
      throw new Error("This is not a valid encrypted PoDL wallet backup.");
    }
    localStorage.setItem(STORAGE_KEY, JSON.stringify(wallet));
    if (Array.isArray(parsed.tokens)) {
      localStorage.setItem(`${TOKENS_STORAGE_KEY}_${wallet.address.toLowerCase()}`, JSON.stringify(parsed.tokens));
    }
    setSavedWalletMeta(wallet);
    setWalletAddress(wallet.address);
    setIsWalletLoaded(false);
    setPrivateKey("");
    setUnlockPassword("");
    setBackupMessage("Backup imported. Unlock it with the original vault password.");
  }, []);

  if (!savedWalletMeta && !isWalletLoaded) {
    return <WalletLogin onWalletCreate={handleWalletCreate} onWalletImport={handleWalletImport} />;
  }

  if (savedWalletMeta && !isWalletLoaded) {
    return (
      <div className="wallet-unlock-layout">
        <section className="unlock-visual">
          <span className="vault-emblem" aria-hidden="true"><i /></span>
          <p>Encrypted local vault</p>
          <h1>Welcome back.</h1>
          <span>Your key material stays encrypted until this session is unlocked.</span>
        </section>
        <form className="wallet-unlock-card" onSubmit={handleUnlock}>
          <div className="wallet-access-heading">
            <span>Existing secure vault</span>
            <h2>Unlock PoDL Wallet</h2>
            <p>Enter the local password created for this browser vault.</p>
          </div>
          <div className="saved-account-chip">
            <span className="account-identicon" aria-hidden="true" />
            <div><strong>{shortAddress(savedWalletMeta.address)}</strong><small>PoDL public testnet</small></div>
          </div>
          {unlockError && <div className="wallet-form-alert" role="alert">{unlockError}</div>}
          <label className="premium-field">
            <span>Vault password</span>
            <input
              type="password"
              value={unlockPassword}
              onChange={(event) => setUnlockPassword(event.target.value)}
              placeholder="Enter your password"
              autoComplete="current-password"
              autoFocus
            />
          </label>
          <button className="wallet-primary-action" type="submit">
            Unlock wallet <span aria-hidden="true">→</span>
          </button>
          <button className="wallet-text-action danger" type="button" onClick={forgetWallet}>
            Forget this wallet on this device
          </button>
        </form>
      </div>
    );
  }

  if (recoveryPhrase) {
    const words = recoveryPhrase.split(/\s+/).filter(Boolean);
    return (
      <section className="recovery-checkpoint">
        <div className="recovery-heading">
          <span>Required security checkpoint</span>
          <h1>Back up your recovery phrase.</h1>
          <p>This is the only recovery path if the encrypted browser vault or its password is lost.</p>
        </div>
        <div className="recovery-phrase-grid">
          {words.map((word, index) => (
            <div key={`${word}-${index}`}><span>{index + 1}</span><strong>{word}</strong></div>
          ))}
        </div>
        <button className="wallet-secondary-action" type="button" onClick={() => navigator.clipboard.writeText(recoveryPhrase)}>
          Copy recovery phrase
        </button>
        <label className="recovery-confirmation">
          <input type="checkbox" checked={recoveryAcknowledged} onChange={(event) => setRecoveryAcknowledged(event.target.checked)} />
          <span>I stored this phrase offline and understand PoDL cannot recover it.</span>
        </label>
        <button
          className="wallet-primary-action"
          type="button"
          disabled={!recoveryAcknowledged}
          onClick={() => {
            setRecoveryPhrase("");
            setRecoveryAcknowledged(false);
          }}
        >
          Continue to wallet <span aria-hidden="true">→</span>
        </button>
      </section>
    );
  }

  const activeDefinition = TABS.find((tab) => tab.id === activeTab) || TABS[0];

  return (
    <div className="wallet-shell">
      <section className="wallet-portfolio-header">
        <div className="wallet-header-copy">
          <span className="wallet-network-label"><i /> PoDL public testnet</span>
          <h1>Wallet command center</h1>
          <p>Assets, transfers, liquidity and contracts—secured by your encrypted local vault.</p>
        </div>
        <div className="wallet-account-card">
          <span className="account-identicon large" aria-hidden="true" />
          <div>
            <small>Active account</small>
            <strong>{shortAddress(walletAddress)}</strong>
          </div>
          <button type="button" onClick={copyAddress}>{copied ? "Copied" : "Copy"}</button>
        </div>
      </section>

      <div className="wallet-security-strip">
        <div><span className="security-dot" /><strong>Vault unlocked</strong><small>AES-256-GCM · auto-lock in 60 seconds of inactivity</small></div>
        <div className="wallet-header-actions">
          <button type="button" onClick={() => navigate(`/address/${walletAddress}`)}>View on explorer</button>
          {requestingOrigin && <button type="button" className="connect-dex" onClick={connectTrustedDex}>Connect trusted DEX</button>}
          <button type="button" className="lock-action" onClick={lockWallet}>Lock wallet</button>
        </div>
      </div>
      {connectionMessage && <div className="wallet-inline-message">{connectionMessage}</div>}

      <div className="wallet-workspace">
        <aside className="wallet-sidebar" aria-label="Wallet navigation">
          <div className="wallet-sidebar-heading">Workspace</div>
          {TABS.map((tab, index) => (
            <button
              key={tab.id}
              type="button"
              className={activeTab === tab.id ? "active" : ""}
              onClick={() => setActiveTab(tab.id)}
            >
              <span>{String(index + 1).padStart(2, "0")}</span>
              <div><strong>{tab.label}</strong><small>{tab.hint}</small></div>
              <i aria-hidden="true">→</i>
            </button>
          ))}
          <div className="wallet-sidebar-footer">
            <span>Local-first security</span>
            <p>Private keys are decrypted only for this active browser session.</p>
          </div>
        </aside>

        <main className="wallet-content premium-wallet-content">
          <div className="wallet-section-heading">
            <div><span>Wallet / {activeDefinition.label}</span><h2>{activeDefinition.label}</h2></div>
            <small>{activeDefinition.hint}</small>
          </div>

          {activeTab === "balance" && <WalletBalance address={walletAddress} privateKey={privateKey} />}
          {activeTab === "send" && <SendTransaction fromAddress={walletAddress} privateKey={privateKey} />}
          {activeTab === "receive" && <ReceiveSection address={walletAddress} />}
          {activeTab === "history" && <TransactionHistory address={walletAddress} />}
          {activeTab === "contracts" && <ContractManager address={walletAddress} privateKey={privateKey} />}
          {activeTab === "liquidity" && <LiquidityDashboard address={walletAddress} />}
          {activeTab === "bridge" && <BridgePanel lqdAddress={walletAddress} lqdPrivateKey={privateKey} />}
          {activeTab === "settings" && (
            <div className="wallet-settings-grid">
              <section className="wallet-settings-card">
                <span className="settings-kicker">Recovery</span>
                <h3>Encrypted wallet backup</h3>
                <p>Export encrypted vault metadata and your local token watchlist. The original password is still required.</p>
                <div className="settings-actions">
                  <button className="wallet-primary-action compact" type="button" onClick={exportBackup}>Export backup</button>
                  <button className="wallet-secondary-action" type="button" onClick={() => backupInputRef.current?.click()}>Import backup</button>
                </div>
                <input
                  ref={backupInputRef}
                  type="file"
                  accept="application/json"
                  hidden
                  onChange={async (event) => {
                    const file = event.target.files?.[0];
                    try {
                      await importBackupFile(file);
                    } catch (importError) {
                      setBackupMessage(importError.message || "Backup import failed.");
                    } finally {
                      event.target.value = "";
                    }
                  }}
                />
                {backupMessage && <div className="wallet-inline-message">{backupMessage}</div>}
              </section>

              <section className="wallet-settings-card">
                <span className="settings-kicker">Account</span>
                <h3>Public account details</h3>
                <div className="wallet-detail-list">
                  <div><span>Address</span><strong>{walletAddress}</strong></div>
                  <div><span>Vault format</span><strong>AES-256-GCM / PBKDF2</strong></div>
                  <div><span>Key derivation</span><strong>250,000 iterations</strong></div>
                  <div><span>Session policy</span><strong>60-second inactivity lock</strong></div>
                </div>
                <button className="wallet-secondary-action" type="button" onClick={copyAddress}>Copy public address</button>
              </section>

              <section className="wallet-settings-card danger-zone">
                <span className="settings-kicker">Device access</span>
                <h3>Forget local wallet</h3>
                <p>Removes the encrypted vault from this browser. Export a backup before continuing.</p>
                <button className="wallet-danger-action" type="button" onClick={forgetWallet}>Forget this wallet</button>
              </section>
            </div>
          )}
        </main>
      </div>
    </div>
  );
};

export default WalletDashboard;
