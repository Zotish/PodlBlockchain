// TransactionsPage.js
import React, { useState, useEffect, useCallback } from 'react';
import TransactionList from '../components/TransactionList';
import { DataSurface, ExplorerPageHero, MetricStrip, PremiumPagination } from '../components/ExplorerPage';
import { fetchHistoricalTransactionPage } from '../utils/api';

const PAGE_SIZE = 10;

const TransactionsPage = () => {
  const [transactions, setTransactions] = useState([]);
  const [loading, setLoading]           = useState(true);
  const [page, setPage]                 = useState(1);
  const [totalPages, setTotalPages]     = useState(1);
  const [total, setTotal]               = useState(0);
  const [error, setError]               = useState('');

  const fetchTransactions = useCallback(async () => {
    try {
      setError('');
      const result = await fetchHistoricalTransactionPage(page, PAGE_SIZE, { timeoutMs: 10000 });
      setTransactions(result.transactions);
      setTotal(result.total);
      setTotalPages(result.totalPages);
    } catch (err) {
      console.error('Error fetching transactions:', err);
      setError(String(err.message || err));
      setTransactions([]);
    } finally {
      setLoading(false);
    }
  }, [page]);

  useEffect(() => {
    setLoading(true);
    fetchTransactions();
    const id = setInterval(fetchTransactions, 15000);
    return () => clearInterval(id);
  }, [fetchTransactions]);

  // ── client-side pagination ──────────────────────────────────────────
  const safePage    = Math.min(page, totalPages);          // clamp if data shrinks
  const startIdx    = (safePage - 1) * PAGE_SIZE;
  const pageTxs     = transactions;

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
    <main className="transactions-page premium-route-page">
      <ExplorerPageHero
        eyebrow="Transaction intelligence"
        title="Every state change, clearly resolved."
        description="Follow transfers, contract calls, fees and settlement status across the live PoDL public ledger."
        metaLabel="Index cadence"
        metaValue="Refreshes every 15 seconds"
      />

      <MetricStrip items={[
        { label: 'Indexed transactions', value: total.toLocaleString(), note: 'historical ledger' },
        { label: 'Current page', value: `${safePage} / ${Math.max(totalPages, 1)}`, note: `${PAGE_SIZE} rows per page` },
        { label: 'Settlement', value: 'Finalized', note: 'status-aware records' },
        { label: 'Data mode', value: 'Live', note: 'automatic refresh' },
      ]} />

      {error && (
        <div className="error" style={{ marginBottom: 16 }}>
          Failed to load transactions: {error}
        </div>
      )}

      <DataSurface title="Transaction stream" description="Decoded movement, counterparties, value, gas and final execution state.">
        <PremiumPagination
          page={safePage}
          totalPages={totalPages}
          pageNumbers={pageNumbers()}
          start={startIdx + 1}
          end={Math.min(startIdx + PAGE_SIZE, total)}
          total={total}
          label="transactions"
          goTo={goTo}
        />
        <TransactionList transactions={pageTxs} />
        {totalPages > 1 && (
          <PremiumPagination
            page={safePage}
            totalPages={totalPages}
            pageNumbers={pageNumbers()}
            start={startIdx + 1}
            end={Math.min(startIdx + PAGE_SIZE, total)}
            total={total}
            label="transactions"
            goTo={goTo}
          />
        )}
      </DataSurface>
    </main>
  );
};

export default TransactionsPage;
