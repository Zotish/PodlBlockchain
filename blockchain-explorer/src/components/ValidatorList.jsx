// src/components/ValidatorList.jsx
import React from 'react';
import { Link } from 'react-router-dom';

const compactNumber = (value, digits = 2) => {
  const number = Number(value);
  if (!Number.isFinite(number)) return '—';
  return new Intl.NumberFormat('en-US', { maximumFractionDigits: digits }).format(number);
};

const ValidatorList = ({ validators, premium = false }) => {
  if (!validators || validators.length === 0) {
    return <p>No validators found</p>;
  }

  if (premium) {
    return (
      <div className="premium-validator-list">
        {validators.map((validator, index) => {
          const address = validator.address || '';
          const penalty = Math.max(0, Math.min(1, Number(validator.penalty_score || 0)));
          const lastActive = Date.parse(validator.last_active || '');
          const stale = !Number.isFinite(lastActive) || Date.now() - lastActive > 120000;
          const jailed = validator.node_status === 'jailed' || Boolean(validator.jailed_until && Date.parse(validator.jailed_until) > Date.now());
          const voting = validator.voting_eligible !== false && !jailed && !stale;
          const status = jailed ? 'Jailed' : stale ? 'Stalled' : voting ? 'Voting' : 'Unavailable';
          return (
            <div className="premium-validator-row" key={address || index}>
              <span className="validator-rank">{String(index + 1).padStart(2, '0')}</span>
              <div className="validator-identity">
                <Link to={address ? `/validator/${address}` : '/validators'}>
                  {address ? `${address.slice(0, 11)}…${address.slice(-7)}` : 'Unknown validator'}
                </Link>
                <span className={voting ? 'validator-online' : 'validator-offline'}>
                  <i /> {status}
                </span>
              </div>
              <div className="validator-stake">
                <span>Native stake</span>
                <strong>{compactNumber(validator.stake)} LQD</strong>
              </div>
              <div className="validator-power">
                <span>Hybrid power</span>
                <strong>{compactNumber(validator.liquidity_power)}</strong>
              </div>
              <div className="validator-blocks">
                <span>Proposed / included</span>
                <strong>{compactNumber(validator.blocks_proposed, 0)} / {compactNumber(validator.blocks_included, 0)}</strong>
              </div>
              <div className="validator-penalty">
                <span>Penalty score</span>
                <strong className={penalty >= 0.95 ? 'critical' : penalty >= 0.5 ? 'warning' : ''}>{compactNumber(penalty * 100, 1)}%</strong>
                <span className="validator-penalty-meter" aria-hidden="true"><i style={{ width: `${penalty * 100}%` }} /></span>
              </div>
              <Link className="validator-row-link" to={address ? `/validator/${address}` : '/validators'} aria-label="Open validator">→</Link>
            </div>
          );
        })}
      </div>
    );
  }

  return (
    <div className="validator-list">
      {validators.map((v, i) => (
        <div key={v.address || i} className="validator-item">
          <div className="validator-address">
            <strong>Address:</strong> {v.address?.slice(0, 42) || 'N/A'}
          </div>

          <div className="validator-stats">
            <span>Stake: {Number.isFinite(Number(v.stake)) ? Number(v.stake).toFixed(2) : '0.00'} LQD</span>
            <span>Power: {Number.isFinite(Number(v.liquidity_power)) ? Number(v.liquidity_power).toFixed(2) : '0.00'}</span>
            <span>Penalty: {Number.isFinite(v.penalty_score) ? (v.penalty_score * 100).toFixed(1) : '0.0'}%</span>
          </div>

          <div className="validator-activity">
            Blocks: {v.blocks_proposed || 0} proposed, {v.blocks_included || 0} included
          </div>

          {(v.last_active || v.lock_time) && (
            <div className="validator-times">
              {v.last_active && <span>Last Active: {v.last_active}</span>}
              {v.lock_time && <span>Lock Until: {v.lock_time}</span>}
            </div>
          )}
        </div>
      ))}
    </div>
  );
};

export default ValidatorList;
