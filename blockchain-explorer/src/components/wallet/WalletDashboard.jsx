import React, { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import WalletLogin, { validateWatchAddress } from "./WalletLogin";
import WalletBalance from "./WalletBalance";
import ReceiveSection from "./ReceiveSection";
import TransactionHistory from "./TransactionHistory";
import LocalTransfer from './LocalTransfer';
import "./Wallet.css";

const STORAGE_KEY = "podl_watch_only_address";
const TABS = [
  { id: "balance", label: "Portfolio", hint: "Balances & public assets" },
  { id: "receive", label: "Receive", hint: "Address & QR" },
  { id: "send", label: "Send", hint: "Local testnet signer" },
  { id: "history", label: "Activity", hint: "Finalized transaction history" },
  { id: "security", label: "Security", hint: "Session and signing status" },
];

const shortAddress = (value) => (value ? `${value.slice(0, 10)}…${value.slice(-8)}` : "—");

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
  try {
    const requestingOrigin = referrer ? new URL(referrer).origin : "";
    return configured.includes(requestingOrigin) ? requestingOrigin : "";
  } catch {
    return "";
  }
}

const WalletDashboard = () => {
  const navigate = useNavigate();
  const [activeTab, setActiveTab] = useState("balance");
  const [walletAddress, setWalletAddress] = useState("");
  const [message, setMessage] = useState("");
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    const saved = localStorage.getItem(STORAGE_KEY) || "";
    if (validateWatchAddress(saved)) setWalletAddress(saved);
    else localStorage.removeItem(STORAGE_KEY);
  }, []);

  const openAddress = (address) => {
    const normalized = String(address || "").trim();
    if (!validateWatchAddress(normalized)) {
      setMessage("The wallet did not return a valid PoDL address.");
      return;
    }
    localStorage.setItem(STORAGE_KEY, normalized);
    setWalletAddress(normalized);
    setMessage("Watch-only account opened. No secret was shared.");
  };

  const connectExtension = async () => {
    try {
      if (!window.lqd?.request) throw new Error("PoDL extension was not detected.");
      const accounts = await window.lqd.request({ method: "lqd_connect" });
      openAddress(Array.isArray(accounts) ? accounts[0] : "");
    } catch (error) {
      setMessage(error?.message || "Extension connection failed.");
    }
  };

  const forgetAddress = () => {
    localStorage.removeItem(STORAGE_KEY);
    setWalletAddress("");
    setMessage("");
  };

  const copyAddress = async () => {
    await navigator.clipboard.writeText(walletAddress);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1200);
  };

  const activeDefinition = useMemo(
    () => TABS.find((tab) => tab.id === activeTab) || TABS[0],
    [activeTab]
  );

  if (!walletAddress) {
    return <WalletLogin onWatchAddress={openAddress} onConnectExtension={connectExtension} />;
  }

  return (
    <div className="wallet-shell">
      <section className="wallet-portfolio-header">
        <div className="wallet-header-copy">
          <span className="wallet-network-label"><i /> PoDL public network</span>
          <h1>Public account view</h1>
          <p>Portfolio, receiving details and finalized activity—with zero private-key custody.</p>
        </div>
        <div className="wallet-account-card">
          <span className="account-identicon large" aria-hidden="true" />
          <div><small>Watched account</small><strong>{shortAddress(walletAddress)}</strong></div>
          <button type="button" onClick={copyAddress}>{copied ? "Copied" : "Copy"}</button>
        </div>
      </section>

      <div className="wallet-security-strip">
        <div><span className="security-dot" /><strong>Watch-only mode</strong><small>No seed phrase or private key is stored, decrypted or transmitted.</small></div>
        <div className="wallet-header-actions">
          <button type="button" onClick={() => navigate(`/address/${walletAddress}`)}>View on explorer</button>
          <button type="button" className="lock-action" onClick={forgetAddress}>Close account</button>
        </div>
      </div>
      {message && <div className="wallet-inline-message" role="status">{message}</div>}

      <div className="wallet-workspace">
        <aside className="wallet-sidebar" aria-label="Wallet navigation">
          <div className="wallet-sidebar-heading">Workspace</div>
          {TABS.map((tab, index) => (
            <button key={tab.id} type="button" className={activeTab === tab.id ? "active" : ""} onClick={() => setActiveTab(tab.id)}>
              <span>{String(index + 1).padStart(2, "0")}</span>
              <div><strong>{tab.label}</strong><small>{tab.hint}</small></div>
              <i aria-hidden="true">→</i>
            </button>
          ))}
          <div className="wallet-sidebar-footer">
            <span>Public-data only</span>
            <p>{import.meta.env.VITE_ENABLE_TESTNET_SIGNING === 'true' ? 'Testnet transfers require approval in your local wallet. No keys enter this explorer.' : 'Signing is disabled until the local signer passes independent review.'}</p>
          </div>
        </aside>

        <main className="wallet-content premium-wallet-content">
          <div className="wallet-section-heading">
            <div><span>Account / {activeDefinition.label}</span><h2>{activeDefinition.label}</h2></div>
            <small>{activeDefinition.hint}</small>
          </div>

          {activeTab === "balance" && <WalletBalance address={walletAddress} />}
          {activeTab === "receive" && <ReceiveSection address={walletAddress} />}
          {activeTab === "send" && <LocalTransfer key={walletAddress} address={walletAddress} />}
          {activeTab === "history" && <TransactionHistory address={walletAddress} />}
          {activeTab === "security" && (
            <div className="wallet-settings-grid">
              <section className="wallet-settings-card">
                <span className="settings-kicker">Trust boundary</span>
                <h3>No secrets in the explorer</h3>
                <p>The explorer stores only this public address. A page reload cannot reveal or recover any signing key.</p>
                <div className="wallet-detail-list">
                  <div><span>Address</span><strong>{walletAddress}</strong></div>
                  <div><span>Custody</span><strong>None</strong></div>
                  <div><span>Signing</span><strong>Fail-closed</strong></div>
                  <div><span>Stored locally</span><strong>Public address only</strong></div>
                </div>
                <button className="wallet-secondary-action" type="button" onClick={copyAddress}>Copy public address</button>
              </section>
              <section className="wallet-settings-card danger-zone">
                <span className="settings-kicker">Real-fund warning</span>
                <h3>Do not paste keys into websites</h3>
                <p>{import.meta.env.VITE_ENABLE_TESTNET_SIGNING === 'true' ? 'Use test funds only. The local-provider integration is experimental, not independently audited or approved for real funds.' : 'Use test funds only. Native PoDL signing remains unavailable here until an audited local wallet implementation is released.'}</p>
                <button className="wallet-danger-action" type="button" onClick={forgetAddress}>Close watch-only session</button>
              </section>
            </div>
          )}
        </main>
      </div>
    </div>
  );
};

export default WalletDashboard;
