import React, { useMemo, useState, useEffect } from 'react';
import ValidatorList from '../components/ValidatorList';
import { DataSurface, ExplorerPageHero, MetricStrip } from '../components/ExplorerPage';
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

  const sortedValidators = useMemo(() => {
    const keyBySort = {
      liquidityPower: 'liquidity_power',
      stake: 'stake',
      blocksProposed: 'blocks_proposed',
    };
    const key = keyBySort[sortBy] || 'liquidity_power';
    return [...validators].sort((left, right) => {
      const delta = Number(left[key] || 0) - Number(right[key] || 0);
      return sortOrder === 'asc' ? delta : -delta;
    });
  }, [sortBy, sortOrder, validators]);

  const totalStake = validators.reduce((sum, validator) => sum + Number(validator.stake || 0), 0);
  const totalPower = validators.reduce((sum, validator) => sum + Number(validator.liquidity_power || 0), 0);
  const healthy = validators.filter((validator) => Number(validator.penalty_score || 0) < 1).length;

  if (loading) return <div className="loading">Loading validators...</div>;

  return (
    <main className="validators-page premium-route-page">
      <ExplorerPageHero
        eyebrow="Consensus operators"
        title="Validator power, made accountable."
        description="Compare hybrid liquidity power, native stake, block participation and penalty posture across the active validator set."
        metaLabel="Consensus view"
        metaValue="Deterministic validator index"
      />

      <MetricStrip items={[
        { label: 'Validator set', value: validators.length.toLocaleString(), note: 'indexed operators' },
        { label: 'Aggregate stake', value: totalStake.toLocaleString(undefined, { maximumFractionDigits: 2 }), note: 'LQD bonded' },
        { label: 'Hybrid power', value: totalPower.toLocaleString(undefined, { maximumFractionDigits: 2 }), note: 'network total' },
        { label: 'Healthy records', value: `${healthy}/${validators.length}`, note: 'below penalty threshold' },
      ]} />

      <DataSurface
        title="Validator registry"
        description="Rank and inspect the operators securing PoDL consensus."
        action={<div className="validators-controls"><div className="sort-options"><span>Sort</span>
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
        </div></div>}
      >
        <ValidatorList validators={sortedValidators} premium />
      </DataSurface>
    </main>
  );
};

export default ValidatorsPage;
