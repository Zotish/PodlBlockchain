// src/components/Navbar.js
import React from 'react';
import { Link, useLocation } from 'react-router-dom';

const NAV_ITEMS = [
  { to: '/',             label: 'Dashboard'    },
  { to: '/blocks',       label: 'Blocks'       },
  { to: '/transactions', label: 'Txns'         },
  { to: '/validators',   label: 'Validators'   },
  { to: '/liquidity',    label: 'Liquidity'    },
  { to: '/pools',        label: 'Pools'        },
  { to: '/wallet',       label: 'Wallet'       },
];

const Navbar = () => {
  const location = useLocation();

  const isActive = (to) =>
    to === '/'
      ? location.pathname === '/'
      : location.pathname.startsWith(to);

  return (
    <nav className="navbar">
      {/* ── Brand ── */}
      <div className="navbar-brand">
        <Link to="/">
          <span className="navbar-logo-icon">⬡</span>
          LQD Explorer
        </Link>
      </div>

      {/* ── Desktop Links ── */}
      <div className="navbar-links" style={{ display: 'flex' }}>
        {NAV_ITEMS.map(({ to, label }) => (
          <Link
            key={to}
            to={to}
            className={isActive(to) ? 'active' : ''}
          >
            {label}
          </Link>
        ))}

        {/* Chain indicator pill */}
        <span style={{
          marginLeft: 12,
          padding: '4px 10px',
          background: 'rgba(16, 185, 129, 0.1)',
          border: '1px solid rgba(16, 185, 129, 0.25)',
          borderRadius: 20,
          fontSize: '0.72rem',
          fontWeight: 600,
          color: '#10b981',
          display: 'flex',
          alignItems: 'center',
          gap: 5,
          whiteSpace: 'nowrap',
        }}>
          <span style={{
            width: 6, height: 6,
            borderRadius: '50%',
            background: '#10b981',
            boxShadow: '0 0 6px #10b981',
            display: 'inline-block',
            animation: 'pulse 2s infinite',
          }} />
          Mainnet
        </span>
      </div>

      {/* ── Pulse animation ── */}
      <style>{`
        @keyframes pulse {
          0%, 100% { opacity: 1; }
          50%       { opacity: 0.4; }
        }
      `}</style>
    </nav>
  );
};

export default Navbar;
