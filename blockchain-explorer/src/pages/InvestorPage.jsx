import React, { useEffect, useState } from 'react';
import { DataSurface, ExplorerPageHero, MetricStrip } from '../components/ExplorerPage';
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
    <main className="investor-page premium-route-page">
      <ExplorerPageHero
        eyebrow="Institutional evidence room"
        title="Protocol claims, backed by live evidence."
        description="A diligence-first view of PoDL consensus, realized economics, concentration and launch readiness—without forecasts or guaranteed returns."
        metaLabel="Evidence posture"
        metaValue={signedReport?.verified ? 'Validator-signed' : 'Verification pending'}
      />
      {error && <div className="error-message">{error}</div>}
      <MetricStrip items={[
        { label: 'Finalized height', value: value(status?.height), note: 'canonical checkpoint' },
        { label: 'Validator set', value: value(metrics.validator_count, 0), note: `${percent(metrics.largest_validator_power_share)} largest share` },
        { label: 'Realized revenue', value: `${value(metrics.realized_protocol_revenue, '0')} LQD`, note: 'no projections' },
        { label: 'Mainnet blockers', value: blockers.length, note: 'critical evidence gates' },
      ]} />
      <section className="investor-detail-grid">
      <DataSurface title="Signed evidence checkpoint" description="Cryptographic authentication for the displayed protocol state.">
        {signedReport?.verified ? (
          <p>Validator <code>{signedReport.signer}</code> signed state root <code>{signedReport.state_root}</code> at height {signedReport.height}. Payload hash: <code>{signedReport.payload_hash}</code>.</p>
        ) : (
          <p>No validator-signed checkpoint is available. Unsigned figures must not be presented as authenticated protocol evidence.</p>
        )}
      </DataSurface>
      <DataSurface title="Economic controls" description="Current observable protocol policy—never projected performance.">
        <dl className="investor-control-list">
          <div><dt>Business pilots</dt><dd>{value(metrics.business_pilot_count, 0)}</dd></div>
          <div><dt>Buyback</dt><dd>{economics?.policy?.buyback_enabled ? 'Enabled' : 'Disabled'}</dd></div>
          <div><dt>Explorer index lag</dt><dd>{value(indexStatus?.lag_blocks, '—')} blocks</dd></div>
          <div><dt>Evidence signature</dt><dd>{signedReport?.verified ? 'Verified' : 'Unavailable'}</dd></div>
        </dl>
      </DataSurface>
      </section>
      <DataSurface title="Realized revenue history" description="Daily custody-backed ledger totals. Zero-revenue dates are preserved; this is historical evidence, not a forecast.">
        <div className="premium-table-scroll">
          <table className="table">
            <thead><tr><th>Date (UTC)</th><th>Realized LQD</th><th>Sources</th><th>Insurance allocation</th></tr></thead>
            <tbody>{activeHistory.length === 0 ? <tr><td colSpan="4" className="tracker-empty">No realized revenue has been recorded.</td></tr> : activeHistory.map((point) => (
              <tr key={point.date}><td>{point.date}</td><td>{value(point.revenue, '0')}</td><td>{Object.keys(point.by_source || {}).join(', ') || '—'}</td><td>{value(point.allocations?.insurance_reserve, '0')}</td></tr>
            ))}</tbody>
          </table>
        </div>
      </DataSurface>
      <section className="investor-detail-grid">
      <DataSurface title="Mainnet evidence gate" description="Launch requires every critical protocol check to pass.">
        <p><strong>{blockers.length}</strong> required blocker(s) remain. Launch is allowed only when the node reports every required check passing.</p>
        <ul className="investor-blocker-list">{blockers.map((check) => <li key={check.name}><strong>{check.name}</strong><span>{check.message}</span></li>)}</ul>
      </DataSurface>
      <DataSurface title="Disclosure" description="Material limitations shown alongside the evidence.">
        <p>These metrics are unaudited until an independent audit is published. LP withdrawals return a proportional basket or market-value output; original fiat value is not guaranteed.</p>
      </DataSurface>
      </section>
    </main>
  );
}
