import React, { useState, useEffect } from 'react';
import ValidatorList from '../components/ValidatorList';
import { fetchJSON, mergeArrayResults } from '../utils/api';

const ValidatorsPage = () => {
  const [validators, setValidators] = useState([]);
  const [loading, setLoading] = useState(true);
  const [sortBy, setSortBy] = useState('liquidityPower');
  const [sortOrder, setSortOrder] = useState('desc');

  useEffect(() => {
    const fetchValidators = async () => {
      try {
        const data = await fetchJSON('/validators');
        const merged = mergeArrayResults(data, 'address').map((v) => ({
          address: v.address ?? v.Address ?? '',
          stake: v.stake ?? v.lp_stake_amount ?? v.LPStakeAmount ?? 0,
          liquidity_power: v.liquidity_power ?? v.LiquidityPower ?? 0,
          penalty_score: v.penalty_score ?? v.PenaltyScore ?? 0,
          blocks_proposed: v.blocks_proposed ?? v.BlocksProposed ?? 0,
          blocks_included: v.blocks_included ?? v.BlocksIncluded ?? 0,
          last_active: v.last_active ?? v.LastActive ?? '',
          lock_time: v.lock_time ?? v.LockTime ?? '',
        }));
        setValidators(merged);
      } catch (err) {
        console.error('Error fetching validators:', err);
      } finally {
        setLoading(false);
      }
    };

    fetchValidators();
  }, [sortBy, sortOrder]);

  const handleSort = (field) => {
    if (sortBy === field) {
      setSortOrder(sortOrder === 'asc' ? 'desc' : 'asc');
    } else {
      setSortBy(field);
      setSortOrder('desc');
    }
  };

  if (loading) return <div className="loading">Loading validators...</div>;

  return (
    <div className="validators-page">
      <h2>Validators</h2>
      
      <div className="validators-controls">
        <div className="sort-options">
          <span>Sort by: </span>
          <button 
            className={sortBy === 'liquidityPower' ? 'active' : ''}
            onClick={() => handleSort('liquidityPower')}
          >
            Liquidity Power {sortBy === 'liquidityPower' && (sortOrder === 'asc' ? '↑' : '↓')}
          </button>
          <button 
            className={sortBy === 'stake' ? 'active' : ''}
            onClick={() => handleSort('stake')}
          >
            Stake Amount {sortBy === 'stake' && (sortOrder === 'asc' ? '↑' : '↓')}
          </button>
          <button 
            className={sortBy === 'blocksProposed' ? 'active' : ''}
            onClick={() => handleSort('blocksProposed')}
          >
            Blocks Proposed {sortBy === 'blocksProposed' && (sortOrder === 'asc' ? '↑' : '↓')}
          </button>
        </div>
      </div>

      <ValidatorList validators={validators} />
    </div>
  );
};

export default ValidatorsPage;
