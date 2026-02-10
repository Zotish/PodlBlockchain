// src/pages/WalletPage.jsx
import React from 'react';
import WalletDashboard from '../components/wallet/WalletDashboard';

const WalletPage = () => {
  return (
    <div className="wallet-page">
      <div className="page-header">
        <h1>LiquidityChain Wallet</h1>
        <p>Manage your LQD coins, send and receive transactions</p>
      </div>
      <WalletDashboard />
    </div>
  );
};

export default WalletPage;