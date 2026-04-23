// src/components/Navbar.js
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

const Navbar = () => {
  const location  = useLocation();
  const [open, setOpen] = useState(false);

  const isActive = (to) =>
    to === '/' ? location.pathname === '/' : location.pathname.startsWith(to);

  const close = () => setOpen(false);

  return (
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
      </div>

      <style>{`
        @keyframes navpulse {
          0%, 100% { opacity: 1; }
          50%       { opacity: 0.35; }
        }
      `}</style>
    </nav>
  );
};

export default Navbar;
