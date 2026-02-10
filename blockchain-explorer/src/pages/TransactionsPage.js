


// TransactionsPage.js
import React, { useState, useEffect } from 'react';
import TransactionList from '../components/TransactionList';
import { fetchJSON, mergeArrayResults } from '../utils/api';

const TransactionsPage = () => {
  const [transactions, setTransactions] = useState([]);
  const [loading, setLoading] = useState(true);
  const [page, setPage] = useState(1);        // kept for future pagination
  const [totalPages, setTotalPages] = useState(1);
  const [error, setError] = useState('');

  // const fetchTransactions = async () => {
  //   try {
  //     setError('');
  //     const response = await fetch(`${NODE}/transactions/recent`);
  //     if (!response.ok) throw new Error(`HTTP ${response.status}`);
  //     const data = await response.json();

  //     // accept both shapes:
  //     //  - plain array: [ ...txs ]
  //     //  - object: { transactions: [...], totalPages: N }
  //     const txs = Array.isArray(data) ? data : (data.transactions || []);
  //     setTransactions(Array.isArray(txs) ? txs : []);
  //     setTotalPages(Number(data.totalPages || 1));
  //   } catch (err) {
  //     console.error('Error fetching transactions:', err);
  //     setError(String(err.message || err));
  //     setTransactions([]);
  //     setTotalPages(1);
  //   } finally {
  //     setLoading(false);
  //   }
  // };


  const fetchTransactions = async () => {
    try {
      setError('');
      const data = await fetchJSON('/transactions/recent');
      const txs = mergeArrayResults(data, 'tx_hash');
      setTransactions(txs);
      setTotalPages(1);
    } catch (err) {
      console.error('Error fetching transactions:', err);
      setError(String(err.message || err));
      setTransactions([]);
      setTotalPages(1);
    } finally {
      setLoading(false);
    }
  };
  
  useEffect(() => {
    fetchTransactions();
    const id = setInterval(fetchTransactions, 1000); // refresh to surface pending→confirmed
    return () => clearInterval(id);
  }, [page]); // page is here in case you add server-side pagination later

  if (loading) return <div className="loading">Loading transactions...</div>;

  return (
    <div className="transactions-page">
      <h2>Transactions</h2>

      {error && (
        <div className="error" style={{marginBottom:12}}>
          Failed to load transactions: {error}
        </div>
      )}

      <div className="transactions-controls">
        <div className="pagination">
          <button onClick={() => setPage(p => Math.max(1, p - 1))} disabled={page === 1}>
            Previous
          </button>
          <span>Page {page} of {totalPages}</span>
          <button onClick={() => setPage(p => Math.min(totalPages, p + 1))} disabled={page === totalPages}>
            Next
          </button>
        </div>
      </div>

      <TransactionList transactions={transactions} />
    </div>
  );
};

export default TransactionsPage;
