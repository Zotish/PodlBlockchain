import React, { Suspense, lazy } from "react";
import { BrowserRouter as Router, Route, Routes } from "react-router-dom";
import Navbar from "./components/Navbar";
import Footer from "./components/Footer";
import HomePage from "./pages/HomePage";
import "./styles.css";
import "./premium.css";
import "./explorer-clean.css";

const BlocksPage = lazy(() => import("./pages/BlocksPage"));
const TransactionsPage = lazy(() => import("./pages/TransactionsPage"));
const ValidatorsPage = lazy(() => import("./pages/ValidatorsPage"));
const BlockPage = lazy(() => import("./pages/BlockPage"));
const BlockRewardsPage = lazy(() => import("./pages/BlockRewardsPage"));
const TransactionPage = lazy(() => import("./pages/TransactionPage"));
const ValidatorPage = lazy(() => import("./pages/ValidatorPage"));
const AddressPage = lazy(() => import("./pages/AddressPage"));
const TokenPage = lazy(() => import("./pages/TokenPage"));
const WalletPage = lazy(() => import("./pages/WalletPage"));
const RewardsPage = lazy(() => import("./pages/RewardsPage"));
const InvestorPage = lazy(() => import("./pages/InvestorPage"));
const LiquidityPage = lazy(() => import("./pages/LiquidityPage"));
const PoolsPage = lazy(() => import("./pages/PoolsPage"));
const BridgePage = lazy(() => import("./pages/BridgePage"));
const ApiDocsPage = lazy(() => import("./pages/ApiDocsPage"));

const loadTracker = (name) =>
  lazy(() => import("./pages/ExplorerTrackers").then((module) => ({ default: module[name] })));

const TokenTrackerPage = loadTracker("TokenTrackerPage");
const PoolTrackerPage = loadTracker("PoolTrackerPage");
const LPTrackerPage = loadTracker("LPTrackerPage");
const ContractTrackerPage = loadTracker("ContractTrackerPage");
const PendingTransactionsPage = loadTracker("PendingTransactionsPage");
const InternalTransactionsPage = loadTracker("InternalTransactionsPage");
const TokenTransfersPage = loadTracker("TokenTransfersPage");
const TokenFlowPage = loadTracker("TokenFlowPage");
const NFTTrackerPage = loadTracker("NFTTrackerPage");
const NFTActivityPage = loadTracker("NFTActivityPage");
const BridgeTransactionsPage = loadTracker("BridgeTransactionsPage");
const TopAccountsPage = loadTracker("TopAccountsPage");
const ChartsStatsPage = loadTracker("ChartsStatsPage");
const BroadcastTransactionPage = loadTracker("BroadcastTransactionPage");
const DeveloperContractToolsPage = loadTracker("DeveloperContractToolsPage");

const RouteLoading = () => (
  <div className="route-loading" role="status" aria-live="polite">
    <span />
    <p>Loading network module…</p>
  </div>
);

function App() {
  return (
    <Router>
      <div className="App">
        <Navbar />
        <div className="container">
          <Suspense fallback={<RouteLoading />}>
            <Routes>
              <Route path="/" element={<HomePage />} />
              <Route path="/blocks" element={<BlocksPage />} />
              <Route path="/blocks/:id" element={<BlockPage />} />
              <Route path="/blocks/:id/rewards" element={<BlockRewardsPage />} />
              <Route path="/transactions" element={<TransactionsPage />} />
              <Route path="/transactions/pending" element={<PendingTransactionsPage />} />
              <Route path="/transactions/internal" element={<InternalTransactionsPage />} />
              <Route path="/transactions/token-transfers" element={<TokenTransfersPage />} />
              <Route path="/tx/:hash" element={<TransactionPage />} />
              <Route path="/validators" element={<ValidatorsPage />} />
              <Route path="/validator/:address" element={<ValidatorPage />} />
              <Route path="/accounts" element={<TopAccountsPage />} />
              <Route path="/address/:address" element={<AddressPage />} />
              <Route path="/wallet" element={<WalletPage />} />
              <Route path="/rewards" element={<RewardsPage />} />
              <Route path="/stats" element={<ChartsStatsPage />} />
              <Route path="/investor" element={<InvestorPage />} />
              <Route path="/liquidity" element={<LiquidityPage />} />
              <Route path="/pools" element={<PoolsPage />} />
              <Route path="/tokens" element={<TokenTrackerPage />} />
              <Route path="/token/:address" element={<TokenPage />} />
              <Route path="/tokens/:address" element={<TokenPage />} />
              <Route path="/tokens/flow" element={<TokenFlowPage />} />
              <Route path="/nfts" element={<NFTTrackerPage />} />
              <Route path="/nfts/mints" element={<NFTActivityPage mode="mints" />} />
              <Route path="/nfts/trades" element={<NFTActivityPage mode="trades" />} />
              <Route path="/nfts/transfers" element={<NFTActivityPage mode="transfers" />} />
              <Route path="/nfts/latest-mints" element={<NFTActivityPage mode="mints" />} />
              <Route path="/pools/tracker" element={<PoolTrackerPage />} />
              <Route path="/lp-tracker" element={<LPTrackerPage />} />
              <Route path="/liquidity/providers" element={<LPTrackerPage />} />
              <Route path="/contracts" element={<ContractTrackerPage />} />
              <Route path="/bridge" element={<BridgePage />} />
              <Route path="/bridge/transactions" element={<BridgeTransactionsPage />} />
              <Route path="/developers/api" element={<ApiDocsPage />} />
              <Route path="/developers/verify-contract" element={<DeveloperContractToolsPage mode="verify" />} />
              <Route path="/developers/contracts/search" element={<DeveloperContractToolsPage mode="search" />} />
              <Route path="/developers/broadcast" element={<BroadcastTransactionPage />} />
            </Routes>
          </Suspense>
        </div>
        <Footer />
      </div>
    </Router>
  );
}

export default App;
