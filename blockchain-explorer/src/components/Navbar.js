import React, { useState } from 'react';
import { Link, useLocation } from 'react-router-dom';

const NAV_ITEMS = [
  { to: '/',             label: 'Dashboard'  },
  { to: '/blocks',       label: 'Blocks'     },
  { to: '/transactions', label: 'Txns'       },
  { to: '/validators',   label: 'Validators' },
  { to: '/liquidity',    label: 'Liquidity'  },
  { to: '/pools',        label: 'Pools'      },
  { to: '/wallet',       label: 'Wallet'     },
];

const NAV_GROUPS = [
  {
    label: 'Blockchain',
    items: [
      { to: '/transactions', label: 'Transactions' },
      { to: '/transactions', label: 'Pending Transactions' },
      { to: '/transactions', label: 'Contract Internal Transactions' },
      { to: '/bridge', label: 'Cross-Chain Transactions' },
      { to: '/blocks', label: 'View Blocks' },
      { to: '/validators', label: 'Top Accounts' },
      { to: '/wallet', label: 'Verified Contracts' },
    ],
  },
  {
    label: 'Tokens',
    items: [
      { to: '/wallet', label: 'Top Tokens' },
      { to: '/transactions', label: 'Token Transfers' },
      { to: '/liquidity', label: 'Token Flow Visualizer' },
    ],
  },
  {
    label: 'NFTs',
    items: [
      { to: '/transactions', label: 'Top NFTs' },
      { to: '/transactions', label: 'Top Mints' },
      { to: '/transactions', label: 'Latest Trades' },
      { to: '/transactions', label: 'Latest Transfers' },
      { to: '/transactions', label: 'Latest Mints' },
    ],
  },
  {
    label: 'Resources',
    items: [
      { to: '/', label: 'Charts & Stats' },
      { to: '/validators', label: 'Validators' },
      { to: '/liquidity', label: 'Liquidity' },
      { to: '/pools', label: 'Pools' },
    ],
  },
  {
    label: 'Developers',
    items: [
      { to: '/wallet', label: 'API Documentation' },
      { to: '/wallet', label: 'Verify Contract' },
      { to: '/wallet', label: 'Smart Contract Search' },
      { to: '/wallet', label: 'Broadcast Transaction' },
    ],
  },
  {
    label: 'More',
    items: [
      { to: '/bridge', label: 'Bridge' },
      { to: '/pools', label: 'Pools' },
      { to: '/liquidity', label: 'Liquidity Mining' },
      { to: '/wallet', label: 'Wallet Tools' },
    ],
  },
];

const Navbar = () => {
  const location  = useLocation();
  const [open, setOpen] = useState(false);

  const isActive = (to) =>
    to === '/' ? location.pathname === '/' : location.pathname.startsWith(to);

  const close = () => setOpen(false);

  return (
    <header className="explorer-header">
      <div className="explorer-topbar">
        <div className="explorer-topbar-stats">
          <span>LQD Price: <strong>Live Testnet</strong></span>
          <span>Gas: <strong>Dynamic</strong></span>
        </div>
        <div className="explorer-topbar-actions" aria-label="Explorer tools">
          <span title="API">/</span>
          <span title="Settings">⚙</span>
          <span title="Theme">☼</span>
        </div>
      </div>

      <nav className="navbar">
        {/* ── Brand ── */}
        <div className="navbar-brand">
          <Link to="/" onClick={close}>
            <span className="navbar-logo-icon">⬡</span>
            LQD Explorer
          </Link>
        </div>

        {/* ── Mainnet pill (always visible) ── */}
        <span className="navbar-mainnet-pill">
          <span className="navbar-dot" />
          Mainnet
        </span>

        {/* ── Hamburger (mobile only) ── */}
        <button
          className="navbar-hamburger"
          onClick={() => setOpen(o => !o)}
          aria-label="Toggle menu"
        >
          {open ? '✕' : '☰'}
        </button>

        {/* ── Nav links ── */}
        <div className={`navbar-links${open ? ' open' : ''}`}>
          {NAV_ITEMS.map(({ to, label }) => (
            <Link
              key={to}
              to={to}
              className={isActive(to) ? 'active' : ''}
              onClick={close}
            >
              {label}
            </Link>
          ))}
          <div className="navbar-mega-groups">
            {NAV_GROUPS.map((group) => (
              <div className="navbar-dropdown" key={group.label}>
                <button type="button" className="navbar-dropdown-trigger">
                  {group.label} <span>⌄</span>
                </button>
                <div className="navbar-dropdown-menu">
                  {group.items.map((item) => (
                    <Link key={`${group.label}-${item.label}`} to={item.to} onClick={close}>
                      {item.label}
                    </Link>
                  ))}
                </div>
              </div>
            ))}
          </div>
        </div>

        <style>{`
          @keyframes navpulse {
            0%, 100% { opacity: 1; }
            50%       { opacity: 0.35; }
          }
        `}</style>
      </nav>
    </header>
  );
};

export default Navbar;
