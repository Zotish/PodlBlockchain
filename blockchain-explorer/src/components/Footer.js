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

const SOCIAL_LINKS = [
  {
    label: 'GitHub',
    href: 'https://github.com/Zotish/PodlBlockchain',
    path: 'M12 .5C5.65.5.5 5.65.5 12c0 5.08 3.29 9.39 7.86 10.92.58.11.79-.25.79-.56v-2.08c-3.2.7-3.88-1.37-3.88-1.37-.52-1.32-1.27-1.67-1.27-1.67-1.04-.71.08-.69.08-.69 1.15.08 1.75 1.18 1.75 1.18 1.02 1.74 2.67 1.24 3.32.95.1-.74.4-1.24.72-1.52-2.55-.29-5.23-1.28-5.23-5.69 0-1.26.45-2.28 1.18-3.08-.12-.29-.51-1.46.11-3.04 0 0 .96-.31 3.15 1.18.91-.25 1.89-.38 2.86-.38.97 0 1.95.13 2.86.38 2.19-1.49 3.15-1.18 3.15-1.18.62 1.58.23 2.75.11 3.04.74.8 1.18 1.82 1.18 3.08 0 4.42-2.69 5.4-5.25 5.68.41.35.77 1.04.77 2.1v3.11c0 .31.21.68.79.56A11.51 11.51 0 0 0 23.5 12C23.5 5.65 18.35.5 12 .5Z',
  },
  {
    label: 'X',
    href: 'https://x.com/',
    path: 'M18.9 2h3.2l-7 8 8.2 10.8h-6.4l-5-6.5-5.7 6.5H3l7.5-8.6L2.6 2h6.6l4.5 6 5.2-6Zm-1.1 17h1.8L8.2 3.7H6.3L17.8 19Z',
  },
  {
    label: 'Telegram',
    href: 'https://t.me/',
    path: 'M21.9 4.1 18.7 20c-.24 1.13-.88 1.4-1.78.87l-4.92-3.63-2.37 2.28c-.26.26-.48.48-.98.48l.35-5 9.1-8.22c.4-.35-.08-.55-.62-.2L6.24 13.65 1.4 12.13c-1.05-.33-1.07-1.05.22-1.55L20.55 3.3c.88-.33 1.65.2 1.35.8Z',
  },
  {
    label: 'Discord',
    href: 'https://discord.com/',
    path: 'M19.5 5.2A16.2 16.2 0 0 0 15.5 4l-.2.4c1.43.35 2.1.85 2.1.85a13.4 13.4 0 0 0-6.8 0s.7-.53 2.18-.87L12.6 4a16.2 16.2 0 0 0-4.02 1.22C6.04 9.02 5.35 12.72 5.7 16.37A16.3 16.3 0 0 0 10.62 19l.6-.82a10.5 10.5 0 0 1-1.55-.74l.37-.28a11.56 11.56 0 0 0 9.93 0l.37.28c-.5.3-1.02.55-1.56.74l.6.82a16.3 16.3 0 0 0 4.92-2.63c.42-4.23-.72-7.9-4.8-11.17ZM10.3 14.05c-.95 0-1.73-.86-1.73-1.92s.76-1.92 1.73-1.92c.96 0 1.75.86 1.73 1.92 0 1.06-.77 1.92-1.73 1.92Zm6.2 0c-.95 0-1.73-.86-1.73-1.92s.76-1.92 1.73-1.92c.96 0 1.75.86 1.73 1.92 0 1.06-.77 1.92-1.73 1.92Z',
  },
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
        <div className="footer-socials" aria-label="LQD social links">
          {SOCIAL_LINKS.map((social) => (
            <a
              key={social.label}
              href={social.href}
              target="_blank"
              rel="noreferrer"
              aria-label={social.label}
              title={social.label}
            >
              <svg viewBox="0 0 24 24" role="img" aria-hidden="true">
                <path d={social.path} />
              </svg>
            </a>
          ))}
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
