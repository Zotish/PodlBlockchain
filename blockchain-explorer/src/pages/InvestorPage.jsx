import React, { useEffect, useState } from 'react';
import { fetchChainJSON } from '../utils/api';

const value = (input, fallback = '—') => input === undefined || input === null || input === '' ? fallback : input;
const percent = (input) => Number.isFinite(Number(input)) ? `${(Number(input) * 100).toFixed(2)}%` : '—';

export default function InvestorPage() {
  const [status, setStatus] = useState(null);
  const [readiness, setReadiness] = useState(null);
  const [indexStatus, setIndexStatus] = useState(null);
  const [error, setError] = useState('');

  useEffect(() => {
    let active = true;
    const load = async () => {
      try {
        const [nextStatus, nextReadiness, nextIndexStatus] = await Promise.all([
          fetchChainJSON('/v2/protocol/status', { cacheTtlMs: 5000 }),
          fetchChainJSON('/readiness/mainnet', { cacheTtlMs: 5000 }),
          fetchChainJSON('/v2/index/status', { cacheTtlMs: 5000 }),
        ]);
        if (active) { setStatus(nextStatus); setReadiness(nextReadiness); setIndexStatus(nextIndexStatus); setError(''); }
      } catch (err) {
        if (active) setError(err.message || 'Metrics unavailable');
      }
    };
    load();
    const timer = setInterval(load, 15000);
    return () => { active = false; clearInterval(timer); };
  }, []);

  const metrics = status?.investor_metrics || {};
  const economics = status?.economics || {};
  const signedReport = status?.signed_investor_report || {};
  const history = Array.isArray(status?.economic_history) ? status.economic_history : [];
  const activeHistory = history.filter((point) => point?.revenue && point.revenue !== '0').slice(-30);
  const checks = Array.isArray(readiness?.checks) ? readiness.checks : [];
  const blockers = checks.filter((check) => check.critical && !check.ok);

  return (
    <main className="investor-page">
      <h1>PoDL Investor Evidence Dashboard</h1>
      <p>Live protocol evidence only. No projected revenue, guaranteed APY, or guaranteed principal is shown.</p>
      {error && <div className="error-message">{error}</div>}
      <section className="dashboard-grid">
        <article className="stat-card"><span>Finalized height</span><strong>{value(status?.height)}</strong></article>
        <article className="stat-card"><span>Validators</span><strong>{value(metrics.validator_count, 0)}</strong></article>
        <article className="stat-card"><span>Largest power share</span><strong>{percent(metrics.largest_validator_power_share)}</strong></article>
        <article className="stat-card"><span>Realized protocol revenue</span><strong>{value(metrics.realized_protocol_revenue, '0')} LQD</strong></article>
        <article className="stat-card"><span>Business pilots</span><strong>{value(metrics.business_pilot_count, 0)}</strong></article>
        <article className="stat-card"><span>Buyback enabled</span><strong>{economics?.policy?.buyback_enabled ? 'Yes' : 'No'}</strong></article>
        <article className="stat-card"><span>Explorer index lag</span><strong>{value(indexStatus?.lag_blocks, '—')} blocks</strong></article>
        <article className="stat-card"><span>Evidence signature</span><strong>{signedReport?.verified ? 'Verified' : 'Unavailable'}</strong></article>
      </section>
      <section className="dashboard-card">
        <h2>Signed evidence checkpoint</h2>
        {signedReport?.verified ? (
          <p>Validator <code>{signedReport.signer}</code> signed state root <code>{signedReport.state_root}</code> at height {signedReport.height}. Payload hash: <code>{signedReport.payload_hash}</code>.</p>
        ) : (
          <p>No validator-signed checkpoint is available. Unsigned figures must not be presented as authenticated protocol evidence.</p>
        )}
      </section>
      <section className="dashboard-card">
        <h2>Realized revenue history</h2>
        <p>Daily custody-backed ledger totals. Zero-revenue dates are preserved by the API; this is historical evidence, not a forecast.</p>
        <div style={{ overflowX: 'auto' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse' }}>
            <thead><tr><th>Date (UTC)</th><th>Realized LQD</th><th>Sources</th><th>Insurance allocation</th></tr></thead>
            <tbody>{activeHistory.length === 0 ? <tr><td colSpan="4">No realized revenue has been recorded.</td></tr> : activeHistory.map((point) => (
              <tr key={point.date}><td>{point.date}</td><td>{value(point.revenue, '0')}</td><td>{Object.keys(point.by_source || {}).join(', ') || '—'}</td><td>{value(point.allocations?.insurance_reserve, '0')}</td></tr>
            ))}</tbody>
          </table>
        </div>
      </section>
      <section className="dashboard-card">
        <h2>Mainnet evidence gate</h2>
        <p><strong>{blockers.length}</strong> required blocker(s) remain. Launch is allowed only when the node reports every required check passing.</p>
        <ul>{blockers.map((check) => <li key={check.name}><strong>{check.name}</strong>: {check.message}</li>)}</ul>
      </section>
      <section className="dashboard-card">
        <h2>Disclosure</h2>
        <p>These metrics are unaudited until an independent audit is published. LP withdrawals return a proportional basket or market-value output; original fiat value is not guaranteed.</p>
      </section>
    </main>
  );
}
