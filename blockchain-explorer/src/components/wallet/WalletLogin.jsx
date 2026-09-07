import React, { useState } from "react";

// Kept as a public helper for existing security tests and consumers migrating
// from the old vault UI. The public explorer no longer accepts passwords.
export function validatePasswordStrength(password) {
  if (!password || password.length < 12) return "Use at least 12 characters";
  if (!/[a-z]/.test(password)) return "Add at least one lowercase letter";
  if (!/[A-Z]/.test(password)) return "Add at least one uppercase letter";
  if (!/[0-9]/.test(password)) return "Add at least one number";
  if (!/[!@#$%^&*()_\-+=[\]{};:"\\|,.<>/?]/.test(password)) return "Add at least one special character";
  return "";
}

export function validateWatchAddress(address) {
  return /^0x[0-9a-fA-F]{40}$/.test(String(address || "").trim());
}

const WalletLogin = ({ onWatchAddress, onConnectExtension }) => {
  const [address, setAddress] = useState("");
  const [error, setError] = useState("");

  const submit = (event) => {
    event.preventDefault();
    const next = address.trim();
    if (!validateWatchAddress(next)) {
      setError("Enter a valid 20-byte PoDL public address.");
      return;
    }
    setError("");
    onWatchAddress(next);
  };

  return (
    <div className="wallet-onboarding">
      <section className="wallet-trust-panel">
        <div className="wallet-product-badge">PoDL Public Account</div>
        <h1>Inspect an account without exposing its keys.</h1>
        <p>
          View LQD balances, tokens, receiving details and finalized activity. This public
          explorer never asks for a seed phrase or private key.
        </p>
        <div className="wallet-trust-list">
          <div><span>01</span><strong>No key custody</strong><small>No secret enters this website or its APIs.</small></div>
          <div><span>02</span><strong>Public-chain verification</strong><small>Portfolio data resolves from public PoDL endpoints.</small></div>
          <div><span>03</span><strong>Fail-closed signing</strong><small>Transfers remain disabled until audited local signing ships.</small></div>
        </div>
        <div className="wallet-local-note">
          <span className="security-orbit" aria-hidden="true"><i /></span>
          <div><strong>Safe for public browsing</strong><small>Use only an address you are comfortable displaying publicly.</small></div>
        </div>
      </section>

      <section className="wallet-access-card">
        <div className="wallet-access-heading">
          <span>Watch-only session</span>
          <h2>Open PoDL account</h2>
          <p>Paste a public address or connect the PoDL extension to share only your address.</p>
        </div>

        <form className="wallet-access-form" onSubmit={submit}>
          {error && <div className="wallet-form-alert" role="alert">{error}</div>}
          <label className="premium-field">
            <span>Public address</span>
            <input
              type="text"
              value={address}
              onChange={(event) => setAddress(event.target.value)}
              placeholder="0x…"
              autoComplete="off"
              spellCheck="false"
            />
            <small>Never enter a private key or recovery phrase here.</small>
          </label>
          <button className="wallet-primary-action" type="submit">
            Open watch-only account <span aria-hidden="true">→</span>
          </button>
          <button className="wallet-secondary-action" type="button" onClick={onConnectExtension}>
            Connect PoDL extension
          </button>
          <p className="wallet-consent-copy">
            Transaction signing is intentionally unavailable in the public explorer until the
            local signer and transaction codec complete independent security review.
          </p>
        </form>
      </section>
    </div>
  );
};

export default WalletLogin;
