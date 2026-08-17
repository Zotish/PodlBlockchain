import React, { useState, useEffect, useCallback } from 'react';
import BlockList from '../components/BlockList';
import { DataSurface, ExplorerPageHero, MetricStrip, PremiumPagination } from '../components/ExplorerPage';
import { fetchJSON, firstNodeResult, mergeArrayResults } from '../utils/api';

const PAGE_SIZE = 10;

const BlocksPage = () => {
  const [blocks,     setBlocks]     = useState([]);
  const [loading,    setLoading]    = useState(true);
  const [page,       setPage]       = useState(1);
  const [totalPages, setTotalPages] = useState(1);
  const [total,      setTotal]      = useState(0);
  const [error,      setError]      = useState('');

  const fetchBlocks = useCallback(async () => {
    try {
      setError('');
      const data = await fetchJSON(`/fetch_last_n_block?page=${page}&size=${PAGE_SIZE}`, {
        cacheTtlMs: 1500,
        timeoutMs: 8000,
      });
      const primary = firstNodeResult(data);

      // direct/paginated response: { blocks: [...], total: N, total_pages: M }
      if (primary && primary.blocks) {
        setBlocks(Array.isArray(primary.blocks) ? primary.blocks : []);
        setTotal(primary.total ?? 0);
        setTotalPages(primary.total_pages ?? 1);
      } else if (data && data.blocks) {
        setBlocks(Array.isArray(data.blocks) ? data.blocks : []);
        setTotal(data.total ?? 0);
        setTotalPages(data.total_pages ?? 1);
      } else {
        // fallback: legacy array response or aggregated node wrapper
        const arr = Array.isArray(data)
          ? data
          : mergeArrayResults(data, 'block_number');
        arr.sort((a, b) => (b.block_number ?? 0) - (a.block_number ?? 0));
        setBlocks(arr);
        setTotal(arr.length);
        setTotalPages(1);
      }
    } catch (err) {
      console.error('Error fetching blocks:', err);
      setError(String(err.message || err));
    } finally {
      setLoading(false);
    }
  }, [page]);

  useEffect(() => {
    setLoading(true);
    fetchBlocks();
    const id = setInterval(fetchBlocks, 5000);   // page changes trigger re-fetch
    return () => clearInterval(id);
  }, [fetchBlocks]);

  const goTo = (p) => setPage(Math.max(1, Math.min(totalPages, p)));

  const pageNumbers = () => {
    const s = new Set([1, totalPages, page, page - 1, page + 1]);
    return [...s].filter(p => p >= 1 && p <= totalPages).sort((a, b) => a - b);
  };

  const startIdx = (page - 1) * PAGE_SIZE + 1;
  const endIdx   = Math.min(page * PAGE_SIZE, total);

  if (loading && blocks.length === 0)
    return <div className="loading">Loading blocks...</div>;

  return (
    <main className="blocks-page premium-route-page">
      <ExplorerPageHero
        eyebrow="Consensus ledger"
        title="Finalized blocks, without the noise."
        description="Inspect canonical PoDL blocks, proposer output and reward distribution from one continuously refreshed ledger view."
        metaLabel="Index cadence"
        metaValue="Refreshes every 5 seconds"
      />

      <MetricStrip items={[
        { label: 'Indexed blocks', value: total.toLocaleString(), note: 'canonical records' },
        { label: 'Current page', value: `${page} / ${Math.max(totalPages, 1)}`, note: `${PAGE_SIZE} rows per page` },
        { label: 'Finality view', value: 'Canonical', note: 'public chain index' },
        { label: 'Data mode', value: 'Live', note: 'automatic refresh' },
      ]} />

      {error && <div className="error" style={{ marginBottom: 16 }}>{error}</div>}

      <DataSurface title="Block ledger" description="Canonical height, hash, transaction count and protocol reward allocation.">
        <PremiumPagination
          page={page} totalPages={totalPages}
          pageNumbers={pageNumbers()}
          start={startIdx} end={endIdx} total={total}
          label="blocks" goTo={goTo}
        />
        <div className="premium-ledger-table"><BlockList blocks={blocks} /></div>
        {totalPages > 1 && (
          <PremiumPagination
            page={page} totalPages={totalPages}
            pageNumbers={pageNumbers()}
            start={startIdx} end={endIdx} total={total}
            label="blocks" goTo={goTo}
          />
        )}
      </DataSurface>
    </main>
  );
};

export default BlocksPage;
