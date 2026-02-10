import React, { useState, useEffect } from 'react';
import { useParams } from 'react-router-dom';
import { fetchJSON, firstNodeResult } from '../utils/api';

const ValidatorPage = () => {
  const { address } = useParams();
  const [validator, setValidator] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
    const fetchValidator = async () => {
      try {
        const data = await fetchJSON(`/validator/${address}`);
        const result = firstNodeResult(data);
        if (!result) {
          throw new Error('Validator not found');
        }
        setValidator(result);
      } catch (err) {
        setError(err.message);
      } finally {
        setLoading(false);
      }
    };

    fetchValidator();
  }, [address]);

  if (loading) return <div className="loading">Loading validator details...</div>;
  if (error) return <div className="error">Error: {error}</div>;

  const addr = validator.Address || validator.address || '';
  const stake = validator.LPStakeAmount ?? validator.lp_stake_amount ?? validator.stake ?? 0;
  const power = validator.LiquidityPower ?? validator.liquidity_power ?? 0;
  const penalty = validator.PenaltyScore ?? validator.penalty_score ?? 0;
  const proposed = validator.BlocksProposed ?? validator.blocks_proposed ?? 0;
  const included = validator.BlocksIncluded ?? validator.blocks_included ?? 0;
  const lastActive = validator.LastActive ?? validator.last_active ?? '';
  const lockTime = validator.LockTime ?? validator.lock_time ?? '';

  return (
    <div className="validator-page">
      <h2>Validator Details</h2>
      <div className="validator-address">{addr}</div>
      
      <div className="validator-stats">
        <div className="stat-card">
          <h3>Stake Amount</h3>
          <p>{Number(stake).toFixed(2)} LQD</p>
        </div>
        <div className="stat-card">
          <h3>Liquidity Power</h3>
          <p>{Number(power).toFixed(2)}</p>
        </div>
        <div className="stat-card">
          <h3>Penalty Score</h3>
          <p className={Number(penalty) > 0.5 ? 'penalty-high' : 'penalty-low'}>
            {Number(penalty).toFixed(2)}
          </p>
        </div>
      </div>

      <div className="validator-details">
        <div className="detail-row">
          <span className="detail-label">Blocks Proposed:</span>
          <span className="detail-value">{proposed}</span>
        </div>
        <div className="detail-row">
          <span className="detail-label">Blocks Included:</span>
          <span className="detail-value">{included}</span>
        </div>
        <div className="detail-row">
          <span className="detail-label">Success Rate:</span>
          <span className="detail-value">
            {proposed > 0 
              ? `${((included / proposed) * 100).toFixed(2)}%`
              : 'N/A'}
          </span>
        </div>
        <div className="detail-row">
          <span className="detail-label">Last Active:</span>
          <span className="detail-value">
            {lastActive ? new Date(lastActive).toLocaleString() : '—'}
          </span>
        </div>
        <div className="detail-row">
          <span className="detail-label">Lock Time:</span>
          <span className="detail-value">
            {lockTime ? new Date(lockTime).toLocaleString() : '—'}
          </span>
        </div>
      </div>

      <h3>Recent Activity</h3>
      <div className="activity-list">
        {/* This would be populated with validator's recent blocks */}
        <p>Activity history would be displayed here</p>
      </div>
    </div>
  );
};

export default ValidatorPage;
