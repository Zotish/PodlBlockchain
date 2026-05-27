import React from 'react';
import { BrowserRouter as Router, Route, Routes } from 'react-router-dom';
import Navbar from './components/Navbar';
import Footer from './components/Footer';
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
import RewardsPage from './pages/RewardsPage';

import './styles.css';
import LiquidityPage from './pages/LiquidityPage';
import PoolsPage from './pages/PoolsPage';
import BridgePage from './pages/BridgePage';
import {
  TokenTrackerPage,
  PoolTrackerPage,
  LPTrackerPage,
  ContractTrackerPage,
  PendingTransactionsPage,
  InternalTransactionsPage,
} from './pages/ExplorerTrackers';

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
            <Route path="/transactions/pending" element={<PendingTransactionsPage />} />
            <Route path="/transactions/internal" element={<InternalTransactionsPage />} />
            <Route path="/tx/:hash" element={<TransactionPage />} />
            <Route path="/validators" element={<ValidatorsPage />} />
            <Route path="/validator/:address" element={<ValidatorPage />} />
            <Route path="/address/:address" element={<AddressPage />} />
            <Route path="/wallet" element={<WalletPage />} />
            <Route path="/rewards" element={<RewardsPage />} />
            <Route path="/liquidity" element={<LiquidityPage />} />
            <Route path="/pools" element={<PoolsPage />} />
            <Route path="/tokens" element={<TokenTrackerPage />} />
            <Route path="/pools/tracker" element={<PoolTrackerPage />} />
            <Route path="/lp-tracker" element={<LPTrackerPage />} />
            <Route path="/liquidity/providers" element={<LPTrackerPage />} />
            <Route path="/contracts" element={<ContractTrackerPage />} />
            <Route path="/bridge" element={<BridgePage />} />

          </Routes>
        </div>
        <Footer />
      </div>
    </Router>
  );
}

export default App;
