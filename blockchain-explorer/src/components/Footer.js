// src/components/Footer.js
import React from 'react';
import { Link } from 'react-router-dom';
import { API_BASE, CHAIN_BASE, WALLET_BASE } from '../utils/api';

const FOOTER_LINKS = [
  { to: '/', label: 'Dashboard' },
  { to: '/blocks', label: 'Blocks' },
  { to: '/transactions', label: 'Transactions' },
  { to: '/validators', label: 'Validators' },
  { to: '/liquidity', label: 'Liquidity' },
  { to: '/wallet', label: 'Wallet' },
];

const ENDPOINTS = [
  { label: 'Chain', value: CHAIN_BASE },
  { label: 'API', value: API_BASE },
  { label: 'Wallet', value: WALLET_BASE },
];

const shortHost = (url) => {
  try {
    return new URL(url).host;
  } catch {
    return url;
  }
};

const Footer = () => (
  <footer className="site-footer">
    <div className="site-footer-glow" />
    <div className="site-footer-inner">
      <div className="footer-brand">
        <Link to="/" className="footer-logo">
          <span className="navbar-logo-icon">⬡</span>
          <span>
            <strong>LQD Explorer</strong>
            <small>Proof of Dynamic Liquidity</small>
          </span>
        </Link>
        <p>
          A production-grade explorer for monitoring blocks, transactions,
          validators, liquidity and wallet activity across the LQD ecosystem.
        </p>
        <div className="footer-badges">
          <span>Live Network</span>
          <span>HTTPS API</span>
          <span>Real-time Blocks</span>
        </div>
      </div>

      <div className="footer-section">
        <h4>Explore</h4>
        <div className="footer-links">
          {FOOTER_LINKS.map((item) => (
            <Link key={item.to} to={item.to}>{item.label}</Link>
          ))}
        </div>
      </div>

      <div className="footer-section footer-endpoints">
        <h4>Backend Status</h4>
        {ENDPOINTS.map((endpoint) => (
          <a
            key={endpoint.label}
            href={endpoint.value}
            target="_blank"
            rel="noreferrer"
            className="footer-endpoint"
            title={endpoint.value}
          >
            <span className="footer-endpoint-dot" />
            <span>{endpoint.label}</span>
            <code>{shortHost(endpoint.value)}</code>
          </a>
        ))}
      </div>
    </div>

    <div className="site-footer-bottom">
      <span>© {new Date().getFullYear()} LQD Network. All systems are for public chain visibility.</span>
      <span>Built for transparent liquidity, validator rewards and transaction tracking.</span>
    </div>
  </footer>
);

export default Footer;
