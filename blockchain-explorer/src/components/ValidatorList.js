// src/components/ValidatorList.jsx
import React from 'react';

const ValidatorList = ({ validators }) => {
  if (!validators || validators.length === 0) {
    return <p>No validators found</p>;
  }

  return (
    <div className="validator-list">
      {validators.map((v, i) => (
        <div key={v.address || i} className="validator-item">
          <div className="validator-address">
            <strong>Address:</strong> {v.address?.slice(0, 42) || 'N/A'}
          </div>

          <div className="validator-stats">
            <span>Stake: {Number.isFinite(v.stake) ? v.stake.toFixed(2) : '0.00'}</span>
            <span>Power: {Number.isFinite(v.liquidity_power) ? v.liquidity_power.toFixed(2) : '0.00'}</span>
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
