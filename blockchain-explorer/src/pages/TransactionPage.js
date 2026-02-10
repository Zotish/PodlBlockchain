



import React, { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import { formatLQD, toBigIntSafe } from '../utils/lqdUnits';
import { fetchJSON, firstNodeResult } from '../utils/api';

const TransactionPage = () => {
  const { hash } = useParams();
  const [tx, setTx] = useState(null);
  const [meta, setMeta] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  // Fetch transaction details
  const fetchTx = async () => {
    try {
      setError('');
      const data = await fetchJSON(`/tx/${hash}`);
      const result = firstNodeResult(data);
      if (!result || !result.transaction) throw new Error(result?.error || 'Transaction not found');

      setTx(result.transaction);
      setMeta({
        source: result.source,
        blockHash: result.block_hash,
        blockNumber: result.block_number,
        txIndex: result.tx_index,
      });
    } catch (e) {
      setError(e.message);
      setTx(null);
      setMeta(null);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchTx();
    const interval = setInterval(fetchTx, 1000);
    return () => clearInterval(interval);
  }, [hash]);

  if (loading) return <div className="loading">Loading transaction…</div>;

  if (error || !tx) {
    return (
      <div style={{ maxWidth: 960, margin: '0 auto', padding: 16 }}>
        <h2>Transaction Details</h2>
        <p style={{ color: '#b91c1c' }}>{error || 'No transaction found.'}</p>
        <Link to="/transactions" style={{ color: '#2563eb' }}>← Back to Transactions</Link>
      </div>
    );
  }

  const value = (tx.value ?? 0);
const gas = (tx.gas ?? tx.Gas ?? 0);
const gasPrice = (tx.gas_price ?? tx.GasPrice ?? 0);
  const fee = gas * gasPrice;
  const timestamp = tx.timestamp ? new Date(tx.timestamp * 1000).toLocaleString() : '—';

  // ------------------------------
  // REWARD BREAKDOWN HANDLING
  // ------------------------------
  const rb = tx.reward_breakdown || tx.RewardBreakdown || null;

//   const validatorReward = toBigIntSafe(rb?.validator_reward || 0);
// const participantReward = toBigIntSafe(rb?.participant_rewards?.[tx.tx_hash] || 0);
// const totalLP = rb ? Object.values(rb.liquidity_rewards || {}).reduce((a,b)=> a + toBigIntSafe(b), 0n) : 0n;
// const totalReward = validatorReward + participantReward + totalLP;
  const validatorReward = rb?.validator_reward || 0;

  const participantReward = rb?.participant_rewards?.[tx.tx_hash] || 0;

  const totalLP = rb
    ? Object.values(rb.liquidity_rewards || {}).reduce((a, b) => a + b, 0)
    : 0;

  const totalReward = validatorReward + participantReward + totalLP;

  return (
    <div style={{ maxWidth: 960, margin: '0 auto', padding: 16 }}>
      <div style={{ marginBottom: 16 }}>
        <h2 style={{ marginBottom: 4 }}>Transaction Details</h2>
        <Link to="/transactions" style={{ fontSize: 13, color: '#2563eb', textDecoration: 'none' }}>
          ← Back to Transactions
        </Link>
      </div>

      {/* ======================== */}
      {/* OVERVIEW CARD            */}
      {/* ======================== */}
      <div style={{ border: '1px solid #e5e7eb', borderRadius: 12, padding: 16, background: '#fff', marginBottom: 16 }}>
        <div style={{ marginBottom: 10 }}>
          <div style={{ fontSize: 13, color: '#6b7280' }}>Transaction Hash:</div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
            <span style={{ fontFamily: 'monospace' }}>{tx.tx_hash}</span>
            <div style={{
              fontSize: 14,
              background: tx.status === 'success' || tx.status === "succsess" ? '#16a34a'
                : tx.status === 'failed' ? '#dc2626' : '#ca8a04',
              color: '#fff', padding: '3px 10px',
              borderRadius: 999,
              fontWeight: 500
            }}>
              {tx.status ? tx.status.toUpperCase() : 'PENDING'}
            </div>
          </div>
        </div>

        <div style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit,minmax(180px,1fr))',
          gap: 16,
        }}>
          <div>
            <div style={{ fontSize: 13, color: '#6b7280' }}>Block:</div>
            <div style={{ fontSize: 14 }}>
              {meta?.blockNumber != null ? (
                <>
                  <Link to={`/blocks/${meta.blockNumber}`} style={{ color: '#2563eb' }}>
                    #{meta.blockNumber}
                  </Link>
                  {typeof meta.txIndex === 'number' && (
                    <span style={{ fontSize: 12, color: '#6b7280' }}>
                      {' '} (Position {meta.txIndex})
                    </span>
                  )}
                </>
              ) : 'Pending'}
            </div>
          </div>

          <div>
            <div style={{ fontSize: 13, color: '#6b7280' }}>Timestamp:</div>
            <div style={{ fontSize: 14 }}>{timestamp}</div>
          </div>
        </div>
      </div>

      {/* ======================== */}
      {/* FROM / TO                */}
      {/* ======================== */}
      <div style={{ border: '1px solid #e5e7eb', padding: 16, borderRadius: 12, background: '#fff', marginBottom: 16 }}>
        <div style={{ marginBottom: 12 }}>
          <strong>From</strong>
          <div style={{ fontFamily: 'monospace' }}>{tx.from}</div>
        </div>

        <div>
          <strong>To</strong>
          <div style={{ fontFamily: 'monospace' }}>
            {tx.to}
            {tx.is_contract && (
              <span style={{ marginLeft: 8, fontSize: 12, color: '#6b7280' }}>
                (Contract)
              </span>
            )}
          </div>
        </div>
      </div>

      {/* ======================== */}
      {/* VALUE + GAS              */}
      {/* ======================== */}
      <div style={{
        border: '1px solid #e5e7eb',
        borderRadius: 12,
        padding: 16,
        background: '#fff',
        marginBottom: 16
      }}>
        <h3 style={{ marginBottom: 12 }}>Value & Gas</h3>

        <div style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit,minmax(180px,1fr))',
          gap: 16,
        }}>
          <div><strong>Value:</strong> {formatLQD(value)} LQD</div>
          <div><strong>Fee:</strong> {formatLQD(fee)} LQD</div>
          <div><strong>Gas Price:</strong> {formatLQD(gasPrice)}</div>
          <div><strong>Gas:</strong> {gas}</div>
          <div><strong>Nonce:</strong> {tx.nonce}</div>
          <div><strong>Chain ID:</strong> {tx.chain_id}</div>
        </div>
      </div>

      {/* ======================== */}
      {/* ⭐ REWARD BREAKDOWN      */}
      {/* ======================== */}
      {rb && (
        <div style={{
          border: '1px solid #e5e7eb',
          borderRadius: 12,
          padding: 16,
          background: '#fff',
          marginBottom: 16
        }}>
          <h3 style={{ marginTop: 0 }}>Reward Breakdown</h3>

          <p><strong>Total Reward:</strong> {formatLQD(totalReward)} LQD</p>
          <p><strong>Validator Reward:</strong> {formatLQD(validatorReward)} LQD</p>
          <p><strong>Participant Reward:</strong> {formatLQD(participantReward)} LQD</p>

          <h4>Liquidity Provider Rewards</h4>
          <ul>
            {Object.entries(rb.liquidity_rewards || {}).map(([addr, reward]) => (
              <li key={addr}>
                {addr}: {formatLQD(reward)} LQD
              </li>
            ))}
          </ul>
        </div>
      )}

      {/* ======================== */}
      {/* INPUT DATA               */}
      {/* ======================== */}
      <div style={{
        border: '1px solid #e5e7eb',
        padding: 16,
        borderRadius: 12,
        background: '#fff'
      }}>
        <h3>Input Data</h3>

        {tx.function ? (
          <div>
            <strong>Function:</strong> {tx.function}
            {tx.args && (
              <ul>
                {tx.args.map((a, i) => (
                  <li key={i}>{a}</li>
                ))}
              </ul>
            )}
          </div>
        ) : (
          <div style={{ color: '#6b7280' }}>No decoded function data.</div>
        )}
      </div>
    </div>
  );
};

export default TransactionPage;
