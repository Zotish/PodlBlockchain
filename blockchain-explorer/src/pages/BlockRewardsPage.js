/* global BigInt */

import React, { useEffect, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { fetchJSON, firstNodeResult } from '../utils/api';
import { formatLQD } from '../utils/lqdUnits';

const BlockRewardsPage = () => {
  const { id } = useParams();
  const [block, setBlock] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
    const fetchBlock = async () => {
      try {
        const data = await fetchJSON(`/block/${id}`);
        const result = firstNodeResult(data);
        if (!result) {
          throw new Error('Block not found');
        }
        setBlock(result);
      } catch (err) {
        setError(err.message);
      } finally {
        setLoading(false);
      }
    };

    fetchBlock();
  }, [id]);

  if (loading) return <div className="loading">Loading reward details...</div>;
  if (error) return <div className="error">Error: {error}</div>;

  const blockNumber = block.BlockNumber ?? block.block_number;
  const rb = block.RewardBreakdown ?? block.reward_breakdown ?? {};
  const validator = rb.Validator ?? rb.validator ?? '';
  const validatorReward = rb.ValidatorReward ?? rb.validator_reward ?? 0;
  const liquidityRewards = rb.LiquidityRewards ?? rb.liquidity_rewards ?? {};
  const participantRewards = rb.ParticipantRewards ?? rb.participant_rewards ?? {};
  const validatorPartRewards = rb.ValidatorPartRewards ?? rb.validator_part_rewards ?? {};

  return (
    <div className="block-page">
      <h2>Block #{blockNumber} Rewards</h2>
      <div className="reward-box">
        <p>
          <strong>Validator:</strong> {validator || '—'}
        </p>
        <p>
          <strong>Validator Reward:</strong> {formatLQD(validatorReward)} LQD
        </p>

        <h4>Liquidity Provider Rewards</h4>
        <ul>
          {Object.keys(liquidityRewards).length > 0 ? (
            Object.entries(liquidityRewards).map(([addr, reward]) => (
              <li key={addr}>
                {addr}: {formatLQD(reward)} LQD
              </li>
            ))
          ) : (
            <li>No LP rewards</li>
          )}
        </ul>

        <h4>Validator Participant Rewards</h4>
        <ul>
          {Object.keys(validatorPartRewards).length > 0 ? (
            Object.entries(validatorPartRewards).map(([addr, reward]) => (
              <li key={addr}>
                {addr}: {formatLQD(reward)} LQD
              </li>
            ))
          ) : (
            <li>No validator participant rewards</li>
          )}
        </ul>

        <h4>Tx Participant Rewards</h4>
        <ul>
          {Object.keys(participantRewards).length > 0 ? (
            Object.entries(participantRewards).map(([hash, reward]) => (
              <li key={hash}>
                Tx {hash.slice(0, 12)}...: {formatLQD(reward)} LQD
              </li>
            ))
          ) : (
            <li>No participant rewards</li>
          )}
        </ul>
      </div>

  
    <Link className="link-button1" to={`/blocks/${blockNumber}`}>
      <h1>Back to block</h1>
    </Link>
  
</div>
    
  );
};

export default BlockRewardsPage;
