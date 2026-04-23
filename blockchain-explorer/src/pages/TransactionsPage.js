// TransactionsPage.js
import React, { useState, useEffect } from 'react';
import TransactionList from '../components/TransactionList';
import { fetchJSON, mergeArrayResults } from '../utils/api';

const PAGE_SIZE = 10;

const TransactionsPage = () => {
  const [transactions, setTransactions] = useState([]);
  const [loading, setLoading]           = useState(true);
  const [page, setPage]                 = useState(1);
  const [error, setError]               = useState('');

  const fetchTransactions = async () => {
    try {
      setError('');
      const data = await fetchJSON('/transactions/recent');
      const txs  = mergeArrayResults(data, 'tx_hash');
      setTransactions(txs);
    } catch (err) {
      console.error('Error fetching transactions:', err);
      setError(String(err.message || err));
      setTransactions([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchTransactions();
    const id = setInterval(fetchTransactions, 1000);
    return () => clearInterval(id);
  }, []);

  // ── client-side pagination ──────────────────────────────────────────
  const totalPages  = Math.max(1, Math.ceil(transactions.length / PAGE_SIZE));
  const safePage    = Math.min(page, totalPages);          // clamp if data shrinks
  const startIdx    = (safePage - 1) * PAGE_SIZE;
  const pageTxs     = transactions.slice(startIdx, startIdx + PAGE_SIZE);

  const goTo = (p) => setPage(Math.max(1, Math.min(totalPages, p)));

  // build page number window: always show first, last, current ±1
  const pageNumbers = () => {
    const pages = new Set([1, totalPages, safePage, safePage - 1, safePage + 1]);
    return [...pages]
      .filter(p => p >= 1 && p <= totalPages)
      .sort((a, b) => a - b);
  };

  if (loading) return <div className="loading">Loading transactions...</div>;

  return (
    <div className="transactions-page" style={{ maxWidth: 1200 }}>
      <h2 style={{
        fontSize: '1.35rem', fontWeight: 700,
        color: 'var(--text-primary)', margin: '0 0 20px',
        letterSpacing: '-0.3px'
      }}>
        Transactions
        {transactions.length > 0 && (
          <span style={{
            marginLeft: 12, fontSize: '0.8rem', fontWeight: 500,
            color: 'var(--text-muted)'
          }}>
            {transactions.length.toLocaleString()} total
          </span>
        )}
      </h2>

      {error && (
        <div className="error" style={{ marginBottom: 16 }}>
          Failed to load transactions: {error}
        </div>
      )}

      {/* ── Pagination top ── */}
      <PaginationBar
        page={safePage}
        totalPages={totalPages}
        pageNumbers={pageNumbers()}
        startIdx={startIdx}
        endIdx={Math.min(startIdx + PAGE_SIZE, transactions.length)}
        total={transactions.length}
        goTo={goTo}
      />

      <TransactionList transactions={pageTxs} />

      {/* ── Pagination bottom ── */}
      {totalPages > 1 && (
        <PaginationBar
          page={safePage}
          totalPages={totalPages}
          pageNumbers={pageNumbers()}
          startIdx={startIdx}
          endIdx={Math.min(startIdx + PAGE_SIZE, transactions.length)}
          total={transactions.length}
          goTo={goTo}
        />
      )}
    </div>
  );
};

/* ══════════════════════════════════════════════
   Reusable pagination bar
══════════════════════════════════════════════ */
const PaginationBar = ({ page, totalPages, pageNumbers, startIdx, endIdx, total, goTo }) => (
  <div style={{
    display: 'flex', alignItems: 'center', gap: 6,
    margin: '12px 0 16px', flexWrap: 'wrap',
  }}>
    {/* showing info */}
    <span style={{ fontSize: '0.8rem', color: 'var(--text-muted)', marginRight: 8 }}>
      Showing <strong style={{ color: 'var(--text-secondary)' }}>{startIdx + 1}–{endIdx}</strong> of{' '}
      <strong style={{ color: 'var(--text-secondary)' }}>{total}</strong> transactions
    </span>

    {/* ← First */}
    <button
      className="btn-secondary"
      style={{ padding: '5px 10px', fontSize: '0.78rem' }}
      onClick={() => goTo(1)}
      disabled={page === 1}
    >
      «
    </button>

    {/* ← Prev */}
    <button
      className="btn-secondary"
      style={{ padding: '5px 12px', fontSize: '0.78rem' }}
      onClick={() => goTo(page - 1)}
      disabled={page === 1}
    >
      ‹ Prev
    </button>

    {/* page number pills */}
    {pageNumbers.map((p, i, arr) => (
      <React.Fragment key={p}>
        {/* ellipsis gap */}
        {i > 0 && arr[i - 1] !== p - 1 && (
          <span style={{ color: 'var(--text-muted)', padding: '0 2px', fontSize: '0.8rem' }}>…</span>
        )}
        <button
          onClick={() => goTo(p)}
          style={{
            padding: '5px 11px',
            fontSize: '0.8rem',
            fontWeight: p === page ? 700 : 400,
            borderRadius: 6,
            border: p === page ? '1px solid var(--primary)' : '1px solid var(--border)',
            background: p === page ? 'var(--primary-subtle)' : 'var(--bg-badge)',
            color: p === page ? 'var(--primary-light)' : 'var(--text-secondary)',
            cursor: p === page ? 'default' : 'pointer',
            transition: 'all 0.15s',
            minWidth: 34,
          }}
        >
          {p}
        </button>
      </React.Fragment>
    ))}

    {/* Next → */}
    <button
      className="btn-secondary"
      style={{ padding: '5px 12px', fontSize: '0.78rem' }}
      onClick={() => goTo(page + 1)}
      disabled={page === totalPages}
    >
      Next ›
    </button>

    {/* Last → */}
    <button
      className="btn-secondary"
      style={{ padding: '5px 10px', fontSize: '0.78rem' }}
      onClick={() => goTo(totalPages)}
      disabled={page === totalPages}
    >
      »
    </button>
  </div>
);

export default TransactionsPage;
