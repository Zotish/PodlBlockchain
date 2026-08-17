import React from "react";
import { Link } from "react-router-dom";
import { API_BASE, CHAIN_BASE, WALLET_BASE } from "../utils/api";

const FOOTER_LINKS = [
  { to: "/blocks", label: "Blocks" },
  { to: "/transactions", label: "Transactions" },
  { to: "/validators", label: "Validators" },
  { to: "/liquidity", label: "Liquidity" },
  { to: "/investor", label: "Investor evidence" },
  { to: "/developers/api", label: "API reference" },
];

const ENDPOINTS = [
  { label: "Chain RPC", value: CHAIN_BASE },
  { label: "Explorer API", value: API_BASE },
  { label: "Wallet service", value: WALLET_BASE },
];

const shortHost = (url) => {
  try { return new URL(url).host; } catch { return url; }
};

const Footer = () => (
  <footer className="site-footer premium-footer">
    <div className="site-footer-inner">
      <div className="footer-brand premium-footer-brand">
        <Link to="/" className="footer-logo">
          <span className="podl-mark" aria-hidden="true"><span /></span>
          <span><strong>PoDL Network</strong><small>Proof of Dynamic Liquidity</small></span>
        </Link>
        <p>
          Public infrastructure for inspecting finalized state, validator consensus,
          dynamic liquidity and protocol-readiness evidence.
        </p>
        <a
          className="footer-github"
          href="https://github.com/Zotish/PodlBlockchain"
          target="_blank"
          rel="noreferrer"
        >
          <span>GH</span> View source on GitHub <i aria-hidden="true">↗</i>
        </a>
      </div>

      <div className="footer-section">
        <h4>Explore</h4>
        <div className="footer-links">
          {FOOTER_LINKS.map((item) => <Link key={item.to} to={item.to}>{item.label}</Link>)}
        </div>
      </div>

      <div className="footer-section footer-endpoints">
        <h4>Public infrastructure</h4>
        {ENDPOINTS.map((endpoint) => (
          <a key={endpoint.label} href={endpoint.value} target="_blank" rel="noreferrer" className="footer-endpoint">
            <span className="footer-endpoint-dot" />
            <span>{endpoint.label}</span>
            <code>{shortHost(endpoint.value)}</code>
          </a>
        ))}
      </div>
    </div>
    <div className="site-footer-bottom">
      <span>© {new Date().getFullYear()} PoDL Network</span>
      <span>Public testnet · Live API data · Self-custodial wallet</span>
    </div>
  </footer>
);

export default Footer;
