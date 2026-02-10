import React from 'react';
import { BrowserRouter as Router, Route, Routes } from 'react-router-dom';
import Navbar from './components/Navbar';
import HomePage from './pages/HomePage';
import BlocksPage from './pages/BlocksPage';
import TransactionsPage from './pages/TransactionsPage';
import ValidatorsPage from './pages/ValidatorsPage';
import BlockPage from './pages/BlockPage';
import BlockRewardsPage from './pages/BlockRewardsPage';
import TransactionPage from './pages/TransactionPage';
import ValidatorPage from './pages/ValidatorPage';
import AddressPage from './pages/AddressPage';
import WalletPage from './pages/WalletPage'; // Add this import

import './styles.css';
import SmartContractStudio from './components/contracts/SmartContractStudio';
import LiquidityPage from './pages/LiquidityPage';
import PoolsPage from './pages/PoolsPage';
import BridgePage from './pages/BridgePage';

function App() {
  return (
    <Router>
      <div className="App">
        <Navbar />
        <div className="container">
          <Routes>
            <Route path="/" element={<HomePage />} />
            <Route path="/blocks" element={<BlocksPage />} />
            <Route path="/blocks/:id" element={<BlockPage />} />
            <Route path="/blocks/:id/rewards" element={<BlockRewardsPage />} />
            <Route path="/transactions" element={<TransactionsPage />} />
            <Route path="/tx/:hash" element={<TransactionPage />} />
            <Route path="/validators" element={<ValidatorsPage />} />
            <Route path="/validator/:address" element={<ValidatorPage />} />
            <Route path="/address/:address" element={<AddressPage />} />
            <Route path="/wallet" element={<WalletPage />} />
            <Route path="/liquidity" element={<LiquidityPage />} />
            <Route path="/pools" element={<PoolsPage />} />
            <Route path="/bridge" element={<BridgePage />} />

          </Routes>
        </div>
      </div>
    </Router>
  );
}

export default App;
