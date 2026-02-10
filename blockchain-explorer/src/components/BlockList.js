


/* global BigInt */
import { formatUnits } from "ethers";
import { formatLQD, parseLQD } from "../utils/lqdUnits";

import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { fetchJSON, mergeArrayResults } from '../utils/api';

const BlockList = ({ blocks: propBlocks, showTxHash = true }) => {
  const [blocks, setBlocks] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const navigate = useNavigate();

  // Format “time ago”
  const formatTimeAgo = (timestamp) => {
    if (!timestamp) return 'N/A';
    const now = Date.now();
    const diff = Math.floor((now - timestamp * 1000) / 1000);

    if (diff < 60) return `${diff}s ago`;
    if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
    if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
    return `${Math.floor(diff / 86400)}d ago`;
  };

  // Fetch blocks
  const fetchBlocks = async () => {
    try {
      const data = await fetchJSON('/fetch_last_n_block');
      const merged = mergeArrayResults(data, 'block_number');
      const sorted = merged.sort((a, b) => (b.block_number ?? 0) - (a.block_number ?? 0));
      setBlocks(sorted);
    } catch (err) {
      console.error('Error fetching blocks:', err);
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (Array.isArray(propBlocks)) {
      setBlocks(propBlocks);
      setLoading(false);
      return;
    }
    fetchBlocks();
    const interval = setInterval(fetchBlocks, 1000);
    return () => clearInterval(interval);
  }, [propBlocks]);

  if (loading) return <div>Loading blocks...</div>;
  if (error) return <div>Error: {error}</div>;
  if (!blocks || blocks.length === 0) return <div>No blocks found</div>;
  const lpMap = blocks?.reward_breakdown?.liquidity_rewards || {};
  const totalLPRewardBase = Object.values(lpMap).reduce(
    (acc, v) => acc + BigInt(v || 0),
    0n
  );
  return (
    <div style={{ padding: '20px' }}>
      <table style={{ width: '100%', borderCollapse: 'collapse', textAlign: 'left' }}>
        <thead>
          <tr style={{ background: '#f0f0f0' }}>
            <th style={{ padding: '8px' }}>Block</th>
            <th style={{ padding: '8px' }}>Hash</th>
            <th style={{ padding: '8px' }}>Transactions</th>
            <th style={{ padding: '8px' }}>Time</th>

            {/* NEW: Reward Columns */}
            <th style={{ padding: '8px' }}>Total Reward</th>
            <th style={{ padding: '8px' }}>Validator</th>
            <th style={{ padding: '8px' }}>LP</th>
            <th style={{ padding: '8px' }}>Participant</th>
            {showTxHash && <th style={{ padding: '8px' }}>Tx Hash</th>}
          </tr>
        </thead>

        <tbody>
          {blocks.map((block) => {
            const rb = block.reward_breakdown || block.RewardBreakdown || null;

            const validatorReward = rb?.validator_reward || 0;
            const lpTotal = rb
              ? Object.values(rb.liquidity_rewards || {}).reduce((a, b) => a + b, 0)
              : 0;

            const participantTotal = rb
              ? Object.values(rb.participant_rewards || rb.ParticipantRewards || {}).reduce(
                  (a, b) => a + b,
                  0
                )
              : 0;

            const totalReward = validatorReward + lpTotal + participantTotal;

            const blockNum = block.block_number ?? block.BlockNumber ?? 0;
            const hash = block.current_hash || block.CurrentHash || '';
            const txs = block.transactions || block.Transactions || [];
            const firstTx = txs[0] || null;
            const txHash = firstTx?.tx_hash || firstTx?.txHash || firstTx?.TxHash || '';
            return (
              <tr key={block.block_number} style={{ borderBottom: '1px solid #ddd' }}>
                <td style={{ padding: '8px' }}>
                  <button
                    type="button"
                    className="link-button"
                    onClick={() => navigate(`/blocks/${blockNum}`)}
                  >
                    {blockNum}
                  </button>
                </td>

                <td style={{ padding: '8px' }}>
                  <button
                    type="button"
                    className="link-button"
                    onClick={() => navigate(`/blocks/${blockNum}`)}
                  >
                    {hash ? `${hash.substring(0, 16)}…` : '—'}
                  </button>
                </td>

                <td style={{ padding: '8px' }}>
                  {block.transactions?.length || 0}
                </td>

                <td style={{ padding: '8px' }}>{formatTimeAgo(block.timestamp)}</td>

                {/* NEW: Reward Columns */}
                <td style={{ padding: '8px' }}>
                {formatLQD(totalReward)} LQD
                </td>

                <td style={{ padding: '8px' }}>
                  {formatLQD(validatorReward)} LQD
                </td>

                <td style={{ padding: '8px' }}>
                  {formatLQD(lpTotal)} LQD
                </td>

                <td style={{ padding: '8px' }}>
                  {formatLQD(participantTotal)} LQD
                </td>

                {showTxHash && (
                  <td style={{ padding: '8px' }}>
                    {txHash ? (
                      <button
                        type="button"
                        className="link-button"
                        onClick={() => navigate(`/blocks/${blockNum}`)}
                      >
                        {txHash.substring(0, 12)}…
                      </button>
                    ) : (
                      '—'
                    )}
                  </td>
                )}
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
};

export default BlockList;
