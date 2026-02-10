

/* global BigInt */

import { formatLQD } from "../utils/lqdUnits";


import React, { useState, useEffect } from 'react';
import { Link, useParams } from 'react-router-dom';
import TransactionList from '../components/TransactionList';
import { fetchJSON, firstNodeResult } from '../utils/api';

const BlockPage = () => {
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

  if (loading) return <div className="loading">Loading block details...</div>;
  if (error) return <div className="error">Error: {error}</div>;
  

  const blockNumber = block.BlockNumber ?? block.block_number;
  const currentHash = block.CurrentHash ?? block.current_hash;
  const previousHash = block.PreviousHash ?? block.previous_hash;
  const timeStamp = block.TimeStamp ?? block.timestamp;
  const gasUsed = block.GasUsed ?? block.gas_used ?? 0;
  const gasLimit = block.GasLimit ?? block.gas_limit ?? 0;
  const rb = block.RewardBreakdown ?? block.reward_breakdown ?? {};
  const validator = rb.Validator ?? rb.validator ?? '';
  const txs = Array.isArray(block.Transactions) ? block.Transactions : block.transactions || [];

  return (
    <div className="block-page">
      <h2>Block #{blockNumber}</h2>
      
      <div className="block-details">
        <div className="detail-row">
          <span className="detail-label">Hash:</span>
          <span className="detail-value">{currentHash}</span>
        </div>

        <div className="detail-row">
          <span className="detail-label">Previous Hash:</span>
          <span className="detail-value">{previousHash}</span>
        </div>

        <div className="detail-row">
          <span className="detail-label">Timestamp:</span>
          <span className="detail-value">
            {timeStamp ? new Date(timeStamp * 1000).toLocaleString() : '—'}
          </span>
        </div>

        <div className="detail-row">
          <span className="detail-label">Validator:</span>
          <span className="detail-value validator-address">
            {validator || '—'}
          </span>
        </div>

        <div className="detail-row">
          <span className="detail-label">Gas Used:</span>
          <span className="detail-value">{gasUsed}</span>
        </div>

        <div className="detail-row">
          <span className="detail-label">Gas Limit:</span>
          <span className="detail-value">{gasLimit}</span>
        </div>

        <div className="detail-row">
          <span className="detail-label">Base Fee:</span>
          <span className="detail-value">{formatLQD(block.BaseFee)} LQD</span>
        </div>
      </div>

      <h3>Reward Breakdown</h3>
      <div className="reward-box">
        <p>Reward details are available on this page.</p>
        <Link className="link-button" to={`/blocks/${blockNumber}/rewards`}>
          Check all rewards
        </Link>
      </div>

      <h3>Transactions ({txs.length})</h3>
      {txs.length > 0 ? (
        <TransactionList transactions={txs} />
      ) : (
        <p>No transactions in this block</p>
      )}
    </div>
  );
};

export default BlockPage;
