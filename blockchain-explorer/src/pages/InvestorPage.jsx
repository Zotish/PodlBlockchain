import React, { useEffect, useState } from 'react';
import { DataSurface, ExplorerPageHero, MetricStrip } from '../components/ExplorerPage';
import { fetchChainJSON } from '../utils/api';
import { formatLQD } from '../utils/lqdUnits';
import { verifyInvestorEvidence } from '../utils/investorEvidence';

const value = (input, fallback = '—') => input === undefined || input === null || input === '' ? fallback : input;
const percent = (input) => input !== null && input !== undefined && input !== '' && Number.isFinite(Number(input)) ? `${(Number(input) * 100).toFixed(2)}%` : '—';
const nativeAmount = (input) => typeof input === 'string' && /^\d+$/.test(input) ? `${formatLQD(input)} LQD` : '—';

export default function InvestorPage() {
  const [status, setStatus] = useState(null);
  const [readiness, setReadiness] = useState(null);
  const [indexStatus, setIndexStatus] = useState(null);
  const [error, setError] = useState('');
  const [verificationTime, setVerificationTime] = useState(Date.now());

  useEffect(() => {
    let active = true;
    const load = async () => {
      if (active) setVerificationTime(Date.now());
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
  const hasSeparatedRevenue = metrics.slashing_counts_as_revenue === false && metrics.realized_business_revenue !== undefined;
  const economics = status?.economics || {};
  const signedReport = status?.signed_investor_report || {};
  const evidence = verifyInvestorEvidence(signedReport, metrics, undefined, verificationTime / 1000);
  const history = Array.isArray(status?.economic_history) ? status.economic_history : [];
  const activeHistory = hasSeparatedRevenue ? history.filter((point) => point.security_recovery !== undefined).slice(-30) : [];
  const hasReadiness = Array.isArray(readiness?.checks) && readiness.checks.length > 0;
  const checks = hasReadiness ? readiness.checks : [];
  const blockers = checks.filter((check) => check.critical && !check.ok);

  return (
    <main className="investor-page premium-route-page">
      <ExplorerPageHero
        eyebrow="Institutional evidence room"
        title="Protocol evidence"
        description="Live consensus, economics, concentration and launch-readiness evidence."
        metaLabel="Evidence posture"
        metaValue={evidence.verified ? 'Signature verified' : 'Unauthenticated metrics'}
      />
      {error && <div className="error-message">{error}</div>}
      <MetricStrip items={[
        { label: 'Finalized height', value: value(evidence.verified ? evidence.payload.height : status?.height), note: 'reported checkpoint' },
        { label: 'Validator set', value: value(metrics.validator_count), note: `${percent(metrics.largest_validator_power_share)} largest share` },
        { label: 'Business revenue', value: hasSeparatedRevenue ? nativeAmount(metrics.realized_business_revenue) : '—', note: hasSeparatedRevenue ? 'realized; excludes slashing' : 'separated revenue unavailable' },
        { label: 'Mainnet blockers', value: hasReadiness ? blockers.length : '—', note: hasReadiness ? 'reported automated checks' : 'readiness data unavailable' },
      ]} />
      <section className="investor-detail-grid">
      <DataSurface title="Signed evidence checkpoint" description="Browser-verified validator attestation; not an audit or a light-client finality proof.">
        {evidence.verified ? (
          <p>Trusted signer <code>{evidence.signer}</code> attested validator metrics, business revenue and checkpoint height {evidence.payload.height}. State root: <code>{evidence.payload.state_root}</code>. Readiness, policy and revenue history are separate, unauthenticated API data.</p>
        ) : (
          <p>{evidence.reason}. These API figures are unauthenticated and must not be presented as verified evidence.</p>
        )}
      </DataSurface>
      <DataSurface title="Economic controls" description="Current observable protocol policy—never projected performance.">
        <dl className="investor-control-list">
          <div><dt>Recorded pilot agreements</dt><dd>{value(metrics.business_pilot_count)}</dd></div>
          <div><dt>Buyback</dt><dd>{typeof economics?.policy?.buyback_enabled === 'boolean' ? (economics.policy.buyback_enabled ? 'Enabled' : 'Disabled') : 'Unavailable'}</dd></div>
          <div><dt>Explorer index lag</dt><dd>{value(indexStatus?.lag_blocks, '—')} blocks</dd></div>
          <div><dt>Evidence signature</dt><dd>{evidence.verified ? 'Verified in browser' : 'Not verified'}</dd></div>
        </dl>
      </DataSurface>
      </section>
      <DataSurface title="Realized business-revenue history" description="Daily custody-backed business revenue. Slashing recovery is disclosed separately and never treated as sales; this is historical evidence, not a forecast.">
        <div className="premium-table-scroll">
          <table className="table">
            <thead><tr><th>Date (UTC)</th><th>Business revenue</th><th>Security recovery</th><th>Sources</th><th>Insurance allocation</th></tr></thead>
            <tbody>{activeHistory.length === 0 ? <tr><td colSpan="5" className="tracker-empty">No separated business-revenue history is available.</td></tr> : activeHistory.map((point) => (
              <tr key={point.date}><td>{point.date}</td><td>{nativeAmount(point.revenue)}</td><td>{nativeAmount(point.security_recovery)}</td><td>{Object.keys(point.by_source || {}).join(', ') || '—'}</td><td>{nativeAmount(point.allocations?.insurance_reserve)}</td></tr>
            ))}</tbody>
          </table>
        </div>
      </DataSurface>
      <section className="investor-detail-grid">
      <DataSurface title="Mainnet evidence gate" description="Launch requires every critical protocol check to pass.">
        <p>{hasReadiness ? <><strong>{blockers.length}</strong> reported automated blocker(s).</> : 'Readiness data is unavailable; launch status cannot be assessed.'} Passing automated checks does not replace independent audits, operator verification or legal review.</p>
        <ul className="investor-blocker-list">{blockers.map((check) => <li key={check.name}><strong>{check.name}</strong><span>{check.message}</span></li>)}</ul>
      </DataSurface>
      <DataSurface title="Disclosure" description="Material limitations shown alongside the evidence.">
        <p>These metrics are unaudited until an independent audit is published. LP withdrawals return a proportional basket or market-value output; original fiat value is not guaranteed.</p>
      </DataSurface>
      </section>
    </main>
  );
}
