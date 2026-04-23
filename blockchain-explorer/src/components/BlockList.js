import { formatLQD, toBigIntSafe } from "../utils/lqdUnits";
import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { fetchJSON, mergeArrayResults } from '../utils/api';

const BlockList = ({ blocks: propBlocks, showTxHash = true }) => {
  const [blocks,  setBlocks]  = useState([]);
  const [loading, setLoading] = useState(true);
  const [error,   setError]   = useState(null);
  const navigate = useNavigate();

  const formatTimeAgo = (timestamp) => {
    if (!timestamp) return 'N/A';
    const diff = Math.floor((Date.now() - timestamp * 1000) / 1000);
    if (diff < 60)    return `${diff}s ago`;
    if (diff < 3600)  return `${Math.floor(diff / 60)}m ago`;
    if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
    return `${Math.floor(diff / 86400)}d ago`;
  };

  const fetchBlocks = async () => {
    try {
      const data   = await fetchJSON('/fetch_last_n_block');
      const merged = mergeArrayResults(data, 'block_number');
      merged.sort((a, b) => (b.block_number ?? 0) - (a.block_number ?? 0));
      setBlocks(merged);
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

  if (loading) return <div className="loading" style={{ padding: '20px' }}>Loading blocks...</div>;
  if (error)   return <div className="error">Error: {error}</div>;
  if (!blocks || blocks.length === 0) return <div style={{ color: 'var(--text-muted)', padding: '16px 0' }}>No blocks found</div>;

  return (
    <div style={{ overflowX: 'auto', WebkitOverflowScrolling: 'touch' }}>
      <table style={{ width: '100%', borderCollapse: 'collapse', minWidth: 600 }}>
        <thead>
          <tr>
            <th>Block</th>
            <th>Hash</th>
            <th>Txns</th>
            <th>Time</th>
            <th>Total Reward</th>
            <th>Validator</th>
            <th>LP</th>
            <th>Participant</th>
            {showTxHash && <th>Tx Hash</th>}
          </tr>
        </thead>

        <tbody>
          {blocks.map((block) => {
            const rb = block.reward_breakdown || block.RewardBreakdown || null;

            const validatorReward = toBigIntSafe(rb?.validator_reward || 0);
            const lpTotal = rb
              ? Object.values(rb.liquidity_rewards || {}).reduce(
                  (a, b) => a + toBigIntSafe(b), 0n)
              : 0n;
            const participantTotal = rb
              ? Object.values(rb.participant_rewards || rb.ParticipantRewards || {}).reduce(
                  (a, b) => a + toBigIntSafe(b), 0n)
              : 0n;
            const totalReward = validatorReward + lpTotal + participantTotal;

            const blockNum = block.block_number ?? block.BlockNumber ?? 0;
            const hash     = block.current_hash  || block.CurrentHash  || '';
            const txs      = block.transactions  || block.Transactions  || [];
            const firstTx  = txs[0] || null;
            const txHash   = firstTx?.tx_hash || firstTx?.txHash || firstTx?.TxHash || '';

            return (
              <tr key={blockNum}>
                <td>
                  <button className="link-button" type="button"
                    onClick={() => navigate(`/blocks/${blockNum}`)}>
                    {blockNum}
                  </button>
                </td>

                <td className="hash-cell">
                  <button className="link-button" type="button"
                    onClick={() => navigate(`/blocks/${blockNum}`)}>
                    {hash ? `${hash.substring(0, 16)}…` : '—'}
                  </button>
                </td>

                <td>{txs.length}</td>

                <td style={{ whiteSpace: 'nowrap' }}>{formatTimeAgo(block.timestamp)}</td>

                <td style={{ whiteSpace: 'nowrap' }}>{formatLQD(totalReward)} LQD</td>
                <td style={{ whiteSpace: 'nowrap' }}>{formatLQD(validatorReward)} LQD</td>
                <td style={{ whiteSpace: 'nowrap' }}>{formatLQD(lpTotal)} LQD</td>
                <td style={{ whiteSpace: 'nowrap' }}>{formatLQD(participantTotal)} LQD</td>

                {showTxHash && (
                  <td className="hash-cell">
                    {txHash ? (
                      <button className="link-button" type="button"
                        onClick={() => navigate(`/blocks/${blockNum}`)}>
                        {txHash.substring(0, 12)}…
                      </button>
                    ) : '—'}
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
