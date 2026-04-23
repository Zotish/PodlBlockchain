import React, { useState, useEffect, useCallback } from 'react';
import BlockList from '../components/BlockList';
import { fetchJSON } from '../utils/api';

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
      const data = await fetchJSON(`/fetch_last_n_block?page=${page}&size=${PAGE_SIZE}`);

      // paginated response: { blocks: [...], total: N, total_pages: M }
      if (data && data.blocks) {
        setBlocks(Array.isArray(data.blocks) ? data.blocks : []);
        setTotal(data.total       ?? 0);
        setTotalPages(data.total_pages ?? 1);
      } else {
        // fallback: legacy array response
        const arr = Array.isArray(data) ? data : [];
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
    const id = setInterval(fetchBlocks, 3000);   // slower poll — page changes trigger re-fetch
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
    <div className="blocks-page" style={{ maxWidth: 1200 }}>
      <h2 style={{
        fontSize: '1.35rem', fontWeight: 700,
        color: 'var(--text-primary)', margin: '0 0 20px',
        letterSpacing: '-0.3px',
      }}>
        Blocks
        {total > 0 && (
          <span style={{ marginLeft: 12, fontSize: '0.8rem',
            fontWeight: 500, color: 'var(--text-muted)' }}>
            {total.toLocaleString()} total
          </span>
        )}
      </h2>

      {error && <div className="error" style={{ marginBottom: 16 }}>{error}</div>}

      <PaginationBar
        page={page} totalPages={totalPages}
        pageNumbers={pageNumbers()}
        startIdx={startIdx} endIdx={endIdx} total={total}
        label="blocks" goTo={goTo}
      />

      {/* BlockList receives the already-paginated slice */}
      <BlockList blocks={blocks} />

      {totalPages > 1 && (
        <PaginationBar
          page={page} totalPages={totalPages}
          pageNumbers={pageNumbers()}
          startIdx={startIdx} endIdx={endIdx} total={total}
          label="blocks" goTo={goTo}
        />
      )}
    </div>
  );
};

/* ── Shared pagination bar ───────────────────────────────────────────── */
const PaginationBar = ({ page, totalPages, pageNumbers, startIdx, endIdx, total, label, goTo }) => (
  <div style={{ display: 'flex', alignItems: 'center', gap: 6,
    margin: '12px 0 16px', flexWrap: 'wrap' }}>

    <span style={{ fontSize: '0.8rem', color: 'var(--text-muted)', marginRight: 8 }}>
      Showing{' '}
      <strong style={{ color: 'var(--text-secondary)' }}>{startIdx}–{endIdx}</strong>
      {' '}of{' '}
      <strong style={{ color: 'var(--text-secondary)' }}>{total.toLocaleString()}</strong>
      {' '}{label}
    </span>

    <button className="btn-secondary" style={{ padding: '5px 10px', fontSize: '0.78rem' }}
      onClick={() => goTo(1)} disabled={page === 1}>«</button>

    <button className="btn-secondary" style={{ padding: '5px 12px', fontSize: '0.78rem' }}
      onClick={() => goTo(page - 1)} disabled={page === 1}>‹ Prev</button>

    {pageNumbers.map((p, i, arr) => (
      <React.Fragment key={p}>
        {i > 0 && arr[i - 1] !== p - 1 && (
          <span style={{ color: 'var(--text-muted)', fontSize: '0.8rem', padding: '0 2px' }}>…</span>
        )}
        <button onClick={() => goTo(p)} style={{
          padding: '5px 11px', fontSize: '0.8rem', minWidth: 34,
          fontWeight: p === page ? 700 : 400, borderRadius: 6,
          border:      p === page ? '1px solid var(--primary)'       : '1px solid var(--border)',
          background:  p === page ? 'var(--primary-subtle)'          : 'var(--bg-badge)',
          color:       p === page ? 'var(--primary-light)'           : 'var(--text-secondary)',
          cursor:      p === page ? 'default' : 'pointer', transition: 'all 0.15s',
        }}>
          {p}
        </button>
      </React.Fragment>
    ))}

    <button className="btn-secondary" style={{ padding: '5px 12px', fontSize: '0.78rem' }}
      onClick={() => goTo(page + 1)} disabled={page === totalPages}>Next ›</button>

    <button className="btn-secondary" style={{ padding: '5px 10px', fontSize: '0.78rem' }}
      onClick={() => goTo(totalPages)} disabled={page === totalPages}>»</button>
  </div>
);

export default BlocksPage;
