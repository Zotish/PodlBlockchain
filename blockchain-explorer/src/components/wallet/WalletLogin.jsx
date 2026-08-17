import React, { useState } from "react";
import { API_BASE, apiUrl } from "../../utils/api";

export function validatePasswordStrength(password) {
  if (!password || password.length < 10) return "Use at least 10 characters";
  if (!/[a-z]/.test(password)) return "Add at least one lowercase letter";
  if (!/[A-Z]/.test(password)) return "Add at least one uppercase letter";
  if (!/[0-9]/.test(password)) return "Add at least one number";
  if (!/[!@#$%^&*()_\-+=[\]{};:"\\|,.<>/?]/.test(password)) return "Add at least one special character";
  return "";
}

const MODES = [
  { id: "create", label: "Create" },
  { id: "import-mnemonic", label: "Recovery phrase" },
  { id: "import-privatekey", label: "Private key" },
];

const WalletLogin = ({ onWalletCreate, onWalletImport }) => {
  const [activeTab, setActiveTab] = useState("create");
  const [password, setPassword] = useState("");
  const [mnemonic, setMnemonic] = useState("");
  const [privateKey, setPrivateKey] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [showPrivateKey, setShowPrivateKey] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const requestWallet = async (path, body) => {
    const response = await fetch(apiUrl(API_BASE, path), {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    const payload = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(payload.error || "The wallet service could not complete this request");
    return payload;
  };

  const submit = async (event) => {
    event.preventDefault();
    setError("");

    const passwordError = validatePasswordStrength(password);
    if (passwordError) {
      setError(passwordError);
      return;
    }

    if (activeTab === "import-mnemonic" && !mnemonic.trim()) {
      setError("Enter your recovery phrase");
      return;
    }
    if (activeTab === "import-privatekey" && !privateKey.trim()) {
      setError("Enter your private key");
      return;
    }

    setLoading(true);
    try {
      if (activeTab === "create") {
        const wallet = await requestWallet("/wallet/new", { password });
        await onWalletCreate(wallet, password);
      } else if (activeTab === "import-mnemonic") {
        const wallet = await requestWallet("/wallet/import/mnemonic", {
          mnemonic: mnemonic.trim().replace(/\s+/g, " "),
          password,
        });
        await onWalletImport(wallet, password);
      } else {
        const wallet = await requestWallet("/wallet/import/private-key", {
          private_key: privateKey.trim(),
        });
        await onWalletImport(wallet, password);
      }
    } catch (requestError) {
      setError(requestError.message || "Wallet request failed");
    } finally {
      setLoading(false);
    }
  };

  const passwordScore = [
    password.length >= 10,
    /[a-z]/.test(password) && /[A-Z]/.test(password),
    /[0-9]/.test(password),
    /[^a-zA-Z0-9]/.test(password),
  ].filter(Boolean).length;

  return (
    <div className="wallet-onboarding">
      <section className="wallet-trust-panel">
        <div className="wallet-product-badge">PoDL Secure Vault</div>
        <h1>Your gateway to the dynamic-liquidity network.</h1>
        <p>
          Manage LQD, tokens, contracts, liquidity positions and bridge operations from one
          encrypted browser vault.
        </p>
        <div className="wallet-trust-list">
          <div><span>01</span><strong>AES-256-GCM vault</strong><small>Private keys are encrypted before browser storage.</small></div>
          <div><span>02</span><strong>Automatic session lock</strong><small>Unlocked key material is cleared after inactivity.</small></div>
          <div><span>03</span><strong>Public-chain verification</strong><small>Balances and activity resolve from PoDL public APIs.</small></div>
        </div>
        <div className="wallet-local-note">
          <span className="security-orbit" aria-hidden="true"><i /></span>
          <div><strong>Non-custodial browser vault</strong><small>PoDL cannot recover a lost password or recovery phrase.</small></div>
        </div>
      </section>

      <section className="wallet-access-card">
        <div className="wallet-access-heading">
          <span>New wallet session</span>
          <h2>Access PoDL Wallet</h2>
          <p>Create a vault or restore an account you already control.</p>
        </div>

        <div className="wallet-mode-switch" role="tablist" aria-label="Wallet access method">
          {MODES.map((mode) => (
            <button
              key={mode.id}
              type="button"
              role="tab"
              aria-selected={activeTab === mode.id}
              className={activeTab === mode.id ? "active" : ""}
              onClick={() => {
                setActiveTab(mode.id);
                setError("");
              }}
            >
              {mode.label}
            </button>
          ))}
        </div>

        <form className="wallet-access-form" onSubmit={submit}>
          {error && <div className="wallet-form-alert" role="alert">{error}</div>}

          {activeTab === "create" && (
            <div className="wallet-method-intro">
              <span className="method-index">01</span>
              <div><strong>Create a fresh PoDL account</strong><p>A unique address and recovery phrase will be generated for you.</p></div>
            </div>
          )}

          {activeTab === "import-mnemonic" && (
            <label className="premium-field">
              <span>Recovery phrase</span>
              <textarea
                value={mnemonic}
                onChange={(event) => setMnemonic(event.target.value)}
                placeholder="Enter every word in the original order"
                rows={4}
                spellCheck="false"
                autoComplete="off"
              />
              <small>Spaces and line breaks will be normalized before import.</small>
            </label>
          )}

          {activeTab === "import-privatekey" && (
            <label className="premium-field">
              <span>Private key</span>
              <div className="secret-field">
                <input
                  type={showPrivateKey ? "text" : "password"}
                  value={privateKey}
                  onChange={(event) => setPrivateKey(event.target.value)}
                  placeholder="Enter your account private key"
                  spellCheck="false"
                  autoComplete="off"
                />
                <button type="button" onClick={() => setShowPrivateKey((visible) => !visible)}>
                  {showPrivateKey ? "Hide" : "Show"}
                </button>
              </div>
              <small>Only import keys on a device you trust.</small>
            </label>
          )}

          <label className="premium-field">
            <span>{activeTab === "create" ? "Create vault password" : "New local vault password"}</span>
            <div className="secret-field">
              <input
                type={showPassword ? "text" : "password"}
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                placeholder="10+ characters with mixed character types"
                autoComplete="new-password"
              />
              <button type="button" onClick={() => setShowPassword((visible) => !visible)}>
                {showPassword ? "Hide" : "Show"}
              </button>
            </div>
          </label>

          <div className="password-meter" aria-label={`Password requirements: ${passwordScore} of 4 met`}>
            <div>{[0, 1, 2, 3].map((index) => <span key={index} className={index < passwordScore ? "filled" : ""} />)}</div>
            <small>{password ? `${passwordScore}/4 security requirements met` : "Use uppercase, lowercase, a number and a symbol"}</small>
          </div>

          <button className="wallet-primary-action" type="submit" disabled={loading}>
            {loading
              ? "Securing vault…"
              : activeTab === "create"
                ? "Create secure wallet"
                : "Import into secure vault"}
            {!loading && <span aria-hidden="true">→</span>}
          </button>

          <p className="wallet-consent-copy">
            By continuing, you acknowledge that this public testnet wallet is self-custodial
            and experimental software.
          </p>
        </form>
      </section>
    </div>
  );
};

export default WalletLogin;
