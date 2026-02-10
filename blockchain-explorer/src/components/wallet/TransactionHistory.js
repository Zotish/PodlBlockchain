// src/components/TransactionHistory.jsx
import React, { useState, useEffect } from 'react';
import { formatLQD } from "./lqdUnits";
import { fetchJSON, mergeArrayResults } from "../../utils/api";

const TransactionHistory = ({ address }) => {
  const [transactions, setTransactions] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const fetchTransactionHistory = async () => {
    try {
      setError('');
      
      // Preferred: direct address history (does not disappear)
      let allTransactions = [];
      try {
        const data = await fetchJSON(`/address/${address}/transactions`);
        const items = mergeArrayResults(data, 'tx_hash');
        if (Array.isArray(items) && items.length > 0) {
          allTransactions = items.map((tx) => ({
            ...tx,
            blockNumber: tx.block_number ?? tx.blockNumber,
            timestamp: tx.timestamp ?? tx.time,
          }));
        }
      } catch {
        // Fallback to recent blocks if endpoint not available
        const data = await fetchJSON('/fetch_last_n_block');
        const blocks = mergeArrayResults(data, 'block_number');
        blocks.forEach(block => {
          if (block.transactions) {
            block.transactions.forEach(tx => {
              if (tx.from === address || tx.to === address) {
                allTransactions.push({
                  ...tx,
                  blockNumber: block.block_number,
                  timestamp: block.timestamp
                });
              }
            });
          }
        });
      }

      // Sort by timestamp (newest first)
      allTransactions.sort((a, b) => b.timestamp - a.timestamp);
      setTransactions(allTransactions);
      
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchTransactionHistory();
  }, [address]);

  const formatTime = (timestamp) => {
    return new Date(timestamp * 1000).toLocaleString();
  };

  if (loading) return <div className="loading">Loading transaction history...</div>;

  return (
    <div className="transaction-history">
      <div className="history-header">
        <h3>Transaction History</h3>
        <button className="btn-secondary" onClick={fetchTransactionHistory}>
          Refresh
        </button>
      </div>

      {error && <div className="error-message">{error}</div>}

      {transactions.length === 0 ? (
        <div className="no-transactions">
          <p>No transactions found for this address</p>
          <p>Transactions will appear here once you send or receive coins</p>
        </div>
      ) : (
        <div className="transactions-list">
          {transactions.map((tx, index) => (
            <div key={tx.tx_hash || index} className="transaction-item">
              <div className="tx-header">
                <div className="tx-hash">
                  <strong>Hash:</strong> {tx.tx_hash?.substring(0, 20)}...
                </div>
                <div className={`tx-status ${tx.status}`}>
                  {tx.status}
                </div>
              </div>
              
              <div className="tx-details">
                <div className="tx-addresses">
                  <div>
                    <strong>From:</strong> {tx.from?.substring(0, 16)}...
                  </div>
                  <div>
                    <strong>To:</strong> {tx.to?.substring(0, 16)}...
                  </div>
                </div>
                
                <div className="tx-amount">
                  <strong>Amount:</strong> {formatLQD(tx.value)} LQD
                </div>
                
                <div className="tx-meta">
                  <div>
                    <strong>Block:</strong> {tx.blockNumber}
                  </div>
                  <div>
                    <strong>Time:</strong> {formatTime(tx.timestamp)}
                  </div>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
};

export default TransactionHistory;
