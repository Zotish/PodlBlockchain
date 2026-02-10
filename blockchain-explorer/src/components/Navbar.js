// src/components/Navbar.jsx
import React from 'react';
import { Link, useLocation } from 'react-router-dom';
const Navbar = () => {
  const location = useLocation();

  return (
    <nav className="navbar">
      <div className="navbar-brand">
        <Link to="/">Chain Explorer</Link>
      </div>
      <div className="navbar-links">
        <Link 
          to="/" 
          className={location.pathname === '/' ? 'active' : ''}
        >
          Dashboard
        </Link>
        <Link 
          to="/blocks" 
          className={location.pathname === '/blocks' ? 'active' : ''}
        >
          Blocks
        </Link>
        <Link 
          to="/transactions" 
          className={location.pathname === '/transactions' ? 'active' : ''}
        >
          Transactions
        </Link>
        <Link 
          to="/validators" 
          className={location.pathname === '/validators' ? 'active' : ''}
        >
          Validators
        </Link>
        {/* Add Wallet Link */}
        <Link 
          to="/wallet" 
          className={location.pathname === '/wallet' ? 'active' : ''}
        >
          Wallet
        </Link>
        <Link 
           to="/liquidity"
           className={location.pathname === '/liquidity' ? 'active' : ''}>
            Liquidity
       </Link>
        <Link
          to="/pools"
          className={location.pathname === '/pools' ? 'active' : ''}
        >
          Pools
        </Link>
      </div>
    </nav>
  );
};

export default Navbar;
