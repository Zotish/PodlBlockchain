/* global BigInt */
import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { fetchJSON, firstNodeResult, mergeArrayResults } from '../utils/api';
import { buildSignedClaimPayload, connectExtensionWallet, shortWalletAddress } from '../utils/claimWallet';
import { formatLQD } from '../utils/lqdUnits';

const SECONDS_PER_YEAR = 365 * 24 * 60 * 60;
const TARGET_BLOCK_SECONDS = 2;

const toBig = (value) => {
  try {
    if (value === undefined || value === null || value === '') return 0n;
    return BigInt(String(value));
  } catch {
    return 0n;
  }
};

const shortAddress = (value = '') => {
  const text = String(value || '');
  if (text.length <= 18) return text || '-';
  return `${text.slice(0, 10)}...${text.slice(-8)}`;
};

const sumMap = (map = {}) =>
  Object.values(map || {}).reduce((acc, value) => acc + toBig(value), 0n);

const normalizeProvider = (lp = {}) => ({
  address: lp.address || lp.Address || '',
  stake: lp.stake_amount ?? lp.StakeAmount ?? 0,
  power: lp.liquidity_power ?? lp.LiquidityPower ?? 0,
  totalRewards: lp.total_rewards ?? lp.TotalRewards ?? 0,
  pendingRewards: lp.pending_rewards ?? lp.PendingRewards ?? 0,
  lockTime: lp.lock_time ?? lp.LockTime ?? 0,
  lockDays: lp.lock_days ?? lp.LockDays ?? 0,
  isUnstaking: Boolean(lp.is_unstaking ?? lp.IsUnstaking),
});

const metric = (label, value, hint) => (
  <div className="reward-metric-card">
    <span>{label}</span>
    <strong>{value}</strong>
    {hint && <small>{hint}</small>}
  </div>
);

export default function RewardsPage() {
  const [latest, setLatest] = useState(null);
  const [history, setHistory] = useState([]);
  const [providers, setProviders] = useState([]);
  const [claimAddress, setClaimAddress] = useState('');
  const [claimStatus, setClaimStatus] = useState('');
  const [connectedWallet, setConnectedWallet] = useState('');
  const [loading, setLoading] = useState(true);
  const [claiming, setClaiming] = useState(false);
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    try {
      setError('');
      const [latestResult, historyResult, providersResult] = await Promise.allSettled([
        fetchJSON('/rewards/latest', { cacheTtlMs: 1500, timeoutMs: 10000 }),
        fetchJSON('/rewards/recent', { cacheTtlMs: 5000, timeoutMs: 10000 }),
        fetchJSON('/liquidity/all', { cacheTtlMs: 4000, timeoutMs: 10000 }),
      ]);

      if (latestResult.status === 'fulfilled') {
        setLatest(firstNodeResult(latestResult.value) || latestResult.value);
      }
      if (historyResult.status === 'fulfilled') {
        const payload = firstNodeResult(historyResult.value) || historyResult.value;
        setHistory(Array.isArray(payload) ? payload : []);
      }
      if (providersResult.status === 'fulfilled') {
        const list = Array.isArray(providersResult.value)
          ? providersResult.value
          : mergeArrayResults(providersResult.value, 'address');
        setProviders(list.map(normalizeProvider));
      }
    } catch (err) {
      setError(err.message || 'Failed to load reward analytics');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
    const id = setInterval(load, 10000);
    return () => clearInterval(id);
  }, [load]);

  const analytics = useMemo(() => {
    const validatorReward = toBig(latest?.validator_reward);
    const validatorPartRewards = sumMap(latest?.validator_part_rewards);
    const liquidityRewards = sumMap(latest?.liquidity_rewards);
    const participantRewards = sumMap(latest?.participant_rewards);
    const totalStaked = providers.reduce((acc, lp) => acc + toBig(lp.stake), 0n);
    const pendingLP = providers.reduce((acc, lp) => acc + toBig(lp.pendingRewards), 0n);
    const totalLPRewards = providers.reduce((acc, lp) => acc + toBig(lp.totalRewards), 0n);
    const latestTotal = validatorReward + validatorPartRewards + liquidityRewards + participantRewards;
    const annualBlocks = Math.floor(SECONDS_PER_YEAR / TARGET_BLOCK_SECONDS);
    const lpRewardPerBlock = liquidityRewards > 0n ? liquidityRewards : pendingLP;
    const apr =
      totalStaked > 0n && lpRewardPerBlock > 0n
        ? Number((lpRewardPerBlock * BigInt(annualBlocks) * 10000n) / totalStaked) / 100
        : 0;
    const apy = apr > 0 ? (Math.pow(1 + apr / 100 / 365, 365) - 1) * 100 : 0;

    return {
      validatorReward,
      validatorPartRewards,
      liquidityRewards,
      participantRewards,
      totalStaked,
      pendingLP,
      totalLPRewards,
      latestTotal,
      apr,
      apy,
    };
  }, [latest, providers]);

  const topProviders = useMemo(
    () =>
      [...providers].sort((a, b) => {
        const left = toBig(a.pendingRewards) + toBig(a.totalRewards);
        const right = toBig(b.pendingRewards) + toBig(b.totalRewards);
        return right > left ? 1 : right < left ? -1 : 0;
      }).slice(0, 10),
    [providers]
  );

  const connectWallet = async () => {
    try {
      const account = await connectExtensionWallet();
      setConnectedWallet(account);
      if (!claimAddress.trim()) setClaimAddress(account);
      setClaimStatus(`Connected ${shortWalletAddress(account)} with LQD Wallet. Claim will require extension approval.`);
    } catch (err) {
      setClaimStatus(err.message || 'Wallet connection failed.');
    }
  };

  const claimRewards = async () => {
    const target = claimAddress.trim() || connectedWallet;
    if (!target) {
      setClaimStatus('Enter an LP address first.');
      return;
    }
    setClaiming(true);
    setClaimStatus('Waiting for LQD Wallet approval...');
    try {
      const payload = await buildSignedClaimPayload(target);
      setConnectedWallet(payload.wallet_address);
      setClaimAddress(payload.address);
      setClaimStatus('LQD Wallet approved. Submitting claim request...');
      const res = await fetchJSON('/liquidity/claim', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
        timeoutMs: 10000,
      });
      setClaimStatus(`Claim submitted: ${JSON.stringify(res)}`);
    } catch (err) {
      const message = err.message || '';
      setClaimStatus(message.includes('Request failed')
        ? 'Claim endpoint is not active on this node yet. Rewards remain tracked until the node is upgraded.'
        : message);
    } finally {
      setClaiming(false);
      load();
    }
  };

  if (loading) return <div className="loading">Loading reward analytics...</div>;

  return (
    <div className="rewards-page">
      <div className="tracker-header reward-hero">
        <p className="section-title">Rewards Analytics</p>
        <h2 className="page-title">
          Reward Center
          {latest?.block_number && <span>latest block #{latest.block_number}</span>}
        </h2>
        <p>
          Validator, LP, and participant rewards are separated here so users can
          understand who earned what and why.
        </p>
      </div>

      {error && <div className="error">{error}</div>}

      <div className="reward-metric-grid">
        {metric('Latest Validator Reward', `${formatLQD(analytics.validatorReward)} LQD`, shortAddress(latest?.validator))}
        {metric('Latest LP Rewards', `${formatLQD(analytics.liquidityRewards)} LQD`, 'distributed to active liquidity providers')}
        {metric('Participant Rewards', `${formatLQD(analytics.participantRewards + analytics.validatorPartRewards)} LQD`, 'validator participants + transaction participants')}
        {metric('Total LP Rewards', `${formatLQD(analytics.totalLPRewards)} LQD`, 'lifetime reward accounting')}
        {metric('Pending LP Rewards', `${formatLQD(analytics.pendingLP)} LQD`, 'available/sync pending by provider')}
        {metric('Estimated LP APR / APY', `${analytics.apr.toFixed(2)}% / ${analytics.apy.toFixed(2)}%`, 'based on current latest LP reward pace')}
      </div>

      <div className="reward-layout">
        <section className="tracker-table-wrap reward-panel">
          <div className="reward-panel-head">
            <div>
              <h3>Reward Timeline</h3>
              <p>Recent reward snapshots emitted by the chain.</p>
            </div>
            <Link to="/blocks" className="btn-secondary">View blocks</Link>
          </div>
          <table className="tracker-table">
            <thead>
              <tr>
                <th>Block</th>
                <th>Base Fee</th>
                <th>Gas Used</th>
                <th>Distribution</th>
              </tr>
            </thead>
            <tbody>
              {history.length === 0 ? (
                <tr><td colSpan={4} className="tracker-empty">No reward snapshots found yet</td></tr>
              ) : history.slice().reverse().map((item, index) => (
                <tr key={`${item.block_number}-${index}`}>
                  <td><Link to={`/blocks/${item.block_number}`}>#{item.block_number}</Link></td>
                  <td>{item.base_fee ?? '-'}</td>
                  <td>{item.gas_used ?? '-'}</td>
                  <td>
                    {Object.entries(item.dist || {}).length === 0 ? '-' : (
                      <div className="reward-chip-list">
                        {Object.entries(item.dist || {}).slice(0, 4).map(([addr, amount]) => (
                          <span key={addr}>{shortAddress(addr)}: {formatLQD(amount)} LQD</span>
                        ))}
                      </div>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </section>

        <aside className="reward-claim-card">
          <h3>Claim Center</h3>
          <p>
            Connect the LP wallet before claiming. The connected address must
            match the LP address, then the extension signs a claim proof.
          </p>
          <button className="btn-secondary" type="button" onClick={connectWallet}>
            {connectedWallet ? `Connected ${shortWalletAddress(connectedWallet)}` : 'Connect Extension Wallet'}
          </button>
          <input
            type="text"
            value={claimAddress}
            onChange={(event) => setClaimAddress(event.target.value)}
            placeholder="0x... connected LP provider address"
          />
          <button className="btn-primary" onClick={claimRewards} disabled={claiming}>
            {claiming ? 'Syncing...' : 'Claim / Sync Rewards'}
          </button>
          {claimStatus && <div className="reward-claim-status">{claimStatus}</div>}
        </aside>
      </div>

      <section className="tracker-table-wrap reward-panel">
        <div className="reward-panel-head">
          <div>
            <h3>Top LP Reward Accounts</h3>
            <p>Providers ranked by total plus pending reward accounting.</p>
          </div>
          <Link to="/liquidity/providers" className="btn-secondary">Open LP tracker</Link>
        </div>
        <table className="tracker-table">
          <thead>
            <tr>
              <th>#</th>
              <th>Provider</th>
              <th>Stake</th>
              <th>Power</th>
              <th>Total Rewards</th>
              <th>Pending</th>
            </tr>
          </thead>
          <tbody>
            {topProviders.length === 0 ? (
              <tr><td colSpan={6} className="tracker-empty">No LP reward accounts found yet</td></tr>
            ) : topProviders.map((lp, index) => (
              <tr key={lp.address || index}>
                <td>{index + 1}</td>
                <td><Link to={`/address/${lp.address}`}>{shortAddress(lp.address)}</Link></td>
                <td>{formatLQD(lp.stake)} LQD</td>
                <td>{Number(lp.power || 0).toFixed(2)}</td>
                <td>{formatLQD(lp.totalRewards)} LQD</td>
                <td>{formatLQD(lp.pendingRewards)} LQD</td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>
    </div>
  );
}
