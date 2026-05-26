import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import {
  fetchHistoricalTransactionPage,
  fetchJSON,
  mergeArrayResults,
  firstNodeResult,
} from '../utils/api';
import { formatLQD } from '../utils/lqdUnits';

const ZERO_ADDRESS = '0x0000000000000000000000000000000000000000';

const shortHash = (value = '') => {
  const text = String(value || '');
  if (text.length <= 18) return text || '-';
  return `${text.slice(0, 10)}...${text.slice(-8)}`;
};

const normalizeContract = (contract = {}) => ({
  address: contract.address || contract.Address || '',
  type: contract.type || contract.Type || 'custom',
  owner: contract.owner || contract.Owner || '',
  name: contract.name || contract.Name || '',
  symbol: contract.symbol || contract.Symbol || '',
  decimals: contract.decimals || contract.Decimals || '',
  totalSupply: contract.totalSupply || contract.total_supply || contract.TotalSupply || '',
});

const normalizeTx = (tx = {}) => ({
  hash: tx.tx_hash || tx.txHash || tx.TxHash || tx.hash || '',
  from: tx.from || tx.From || '',
  to: tx.to || tx.To || '',
  type: tx.type || tx.Type || tx.tx_type || tx.function || tx.Function || 'transfer',
  status: tx.status || tx.Status || 'pending',
  value: tx.value ?? tx.Value ?? 0,
  block: tx.block_number ?? tx.BlockNumber ?? tx.block ?? '-',
  timestamp: tx.timestamp ?? tx.Timestamp ?? 0,
  isSystem: Boolean(tx.is_system || tx.IsSystem),
  isContract: Boolean(tx.is_contract || tx.IsContract),
  functionName: tx.function || tx.Function || '',
});

const isSuccess = (status = '') => String(status).toLowerCase() === 'succsess' || String(status).toLowerCase() === 'success';
const isFailed = (status = '') => String(status).toLowerCase() === 'failed';
const isPending = (tx = {}) => !isSuccess(normalizeTx(tx).status) && !isFailed(normalizeTx(tx).status);

const isTokenContract = (contract = {}) => {
  const type = String(contract.type || '').toLowerCase();
  return type.includes('token') && !type.includes('nft');
};

const isInternalTx = (tx = {}) => {
  const item = normalizeTx(tx);
  const type = String(item.type || '').toLowerCase();
  const fn = String(item.functionName || '').toLowerCase();
  return (
    item.isSystem ||
    item.from.toLowerCase() === ZERO_ADDRESS ||
    type.includes('reward') ||
    type.includes('internal') ||
    type.includes('contract') ||
    fn === 'constructor'
  );
};

const useContracts = () => {
  const [contracts, setContracts] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    try {
      setError('');
      const data = await fetchJSON('/contract/list', { cacheTtlMs: 5000, timeoutMs: 10000 });
      setContracts((Array.isArray(data) ? data : mergeArrayResults(data, 'address')).map(normalizeContract));
    } catch (err) {
      setError(err.message || 'Failed to load contracts');
      setContracts([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
    const id = setInterval(load, 15000);
    return () => clearInterval(id);
  }, [load]);

  return { contracts, loading, error, reload: load };
};

const PageHeader = ({ title, subtitle, count }) => (
  <div className="tracker-header">
    <div>
      <p className="section-title">Explorer Tracker</p>
      <h2 className="page-title">
        {title}
        {typeof count === 'number' && <span>{count.toLocaleString()} total</span>}
      </h2>
      {subtitle && <p>{subtitle}</p>}
    </div>
  </div>
);

const TrackerShell = ({ title, subtitle, count, loading, error, children }) => (
  <div className="tracker-page">
    <PageHeader title={title} subtitle={subtitle} count={count} />
    {error && <div className="error">{error}</div>}
    {loading ? <div className="loading">Loading tracker data...</div> : children}
  </div>
);

const EmptyRow = ({ colSpan, label }) => (
  <tr>
    <td colSpan={colSpan} className="tracker-empty">{label}</td>
  </tr>
);

export const TokenTrackerPage = () => {
  const { contracts, loading, error } = useContracts();
  const tokens = useMemo(() => contracts.filter(isTokenContract), [contracts]);

  return (
    <TrackerShell
      title="Token Tracker"
      subtitle="All detected LQD token contracts from deployed contract registry."
      count={tokens.length}
      loading={loading}
      error={error}
    >
      <div className="tracker-table-wrap">
        <table className="tracker-table">
          <thead>
            <tr>
              <th>#</th>
              <th>Token</th>
              <th>Type</th>
              <th>Decimals</th>
              <th>Total Supply</th>
              <th>Contract</th>
              <th>Owner</th>
            </tr>
          </thead>
          <tbody>
            {tokens.length === 0 ? (
              <EmptyRow colSpan={7} label="No token contracts found yet" />
            ) : tokens.map((token, index) => (
              <tr key={token.address || index}>
                <td>{index + 1}</td>
                <td>
                  <strong>{token.name || token.symbol || 'LQD Token'}</strong>
                  <small>{token.symbol || 'metadata pending'}</small>
                </td>
                <td><span className="badge badge-blue">{token.type}</span></td>
                <td>{token.decimals || '-'}</td>
                <td>{token.totalSupply || '-'}</td>
                <td><Link to={`/address/${token.address}`}>{shortHash(token.address)}</Link></td>
                <td>{token.owner ? shortHash(token.owner) : '-'}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </TrackerShell>
  );
};

export const PoolTrackerPage = () => {
  const [pools, setPools] = useState([]);
  const [summary, setSummary] = useState({ total: 0, target: 0, unallocated: 0 });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    try {
      setError('');
      const data = await fetchJSON('/liquidity/pools', { cacheTtlMs: 4000, timeoutMs: 10000 });
      const payload = firstNodeResult(data) || {};
      const entries = Object.entries(payload.pools || {}).map(([address, liquidity]) => ({
        address,
        liquidity,
      }));
      setPools(entries);
      setSummary({
        total: payload.total || 0,
        target: payload.target_equal || 0,
        unallocated: payload.unallocated || 0,
      });
    } catch (err) {
      setError(err.message || 'Failed to load pools');
      setPools([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
    const id = setInterval(load, 10000);
    return () => clearInterval(id);
  }, [load]);

  return (
    <TrackerShell
      title="Pool Tracker"
      subtitle="Registered liquidity pools with dynamic liquidity balance status."
      count={pools.length}
      loading={loading}
      error={error}
    >
      <div className="tracker-stat-grid">
        <div>Total Liquidity <strong>{formatLQD(summary.total)}</strong></div>
        <div>Target Equal <strong>{formatLQD(summary.target)}</strong></div>
        <div>Unallocated <strong>{formatLQD(summary.unallocated)}</strong></div>
      </div>
      <div className="tracker-table-wrap">
        <table className="tracker-table">
          <thead>
            <tr>
              <th>#</th>
              <th>Pool Contract</th>
              <th>Liquidity</th>
              <th>Balance Status</th>
            </tr>
          </thead>
          <tbody>
            {pools.length === 0 ? (
              <EmptyRow colSpan={4} label="No pools found yet" />
            ) : pools.map((pool, index) => (
              <tr key={pool.address}>
                <td>{index + 1}</td>
                <td><Link to={`/address/${pool.address}`}>{pool.address}</Link></td>
                <td>{formatLQD(pool.liquidity)}</td>
                <td><span className="badge badge-teal">Tracked</span></td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </TrackerShell>
  );
};

export const LPTrackerPage = () => {
  const [providers, setProviders] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    try {
      setError('');
      const data = await fetchJSON('/liquidity/all', { cacheTtlMs: 4000, timeoutMs: 10000 });
      const list = Array.isArray(data) ? data : mergeArrayResults(data, 'address');
      setProviders(list.map((lp) => ({
        address: lp.address || lp.Address || '',
        stake: lp.stake_amount ?? lp.StakeAmount ?? 0,
        power: lp.liquidity_power ?? lp.LiquidityPower ?? 0,
        totalRewards: lp.total_rewards ?? lp.TotalRewards ?? 0,
        pendingRewards: lp.pending_rewards ?? lp.PendingRewards ?? 0,
      })));
    } catch (err) {
      setError(err.message || 'Failed to load LP providers');
      setProviders([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
    const id = setInterval(load, 10000);
    return () => clearInterval(id);
  }, [load]);

  return (
    <TrackerShell
      title="LP Tracker"
      subtitle="Liquidity providers, liquidity power, and reward accounting."
      count={providers.length}
      loading={loading}
      error={error}
    >
      <div className="tracker-table-wrap">
        <table className="tracker-table">
          <thead>
            <tr>
              <th>#</th>
              <th>Provider</th>
              <th>Stake</th>
              <th>Liquidity Power</th>
              <th>Total Rewards</th>
              <th>Pending Rewards</th>
            </tr>
          </thead>
          <tbody>
            {providers.length === 0 ? (
              <EmptyRow colSpan={6} label="No liquidity providers found yet" />
            ) : providers.map((lp, index) => (
              <tr key={lp.address || index}>
                <td>{index + 1}</td>
                <td><Link to={`/address/${lp.address}`}>{shortHash(lp.address)}</Link></td>
                <td>{formatLQD(lp.stake)}</td>
                <td>{formatLQD(lp.power)}</td>
                <td>{formatLQD(lp.totalRewards)}</td>
                <td>{formatLQD(lp.pendingRewards)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </TrackerShell>
  );
};

export const ContractTrackerPage = () => {
  const { contracts, loading, error } = useContracts();

  return (
    <TrackerShell
      title="Contract Tracker"
      subtitle="Deployed smart contracts indexed by the LQD contract registry."
      count={contracts.length}
      loading={loading}
      error={error}
    >
      <div className="tracker-table-wrap">
        <table className="tracker-table">
          <thead>
            <tr>
              <th>#</th>
              <th>Contract</th>
              <th>Type</th>
              <th>Owner</th>
              <th>Status</th>
            </tr>
          </thead>
          <tbody>
            {contracts.length === 0 ? (
              <EmptyRow colSpan={5} label="No contracts deployed yet" />
            ) : contracts.map((contract, index) => (
              <tr key={contract.address || index}>
                <td>{index + 1}</td>
                <td><Link to={`/address/${contract.address}`}>{contract.address}</Link></td>
                <td><span className="badge badge-purple">{contract.type}</span></td>
                <td>{contract.owner ? shortHash(contract.owner) : '-'}</td>
                <td><span className="badge badge-green">Verified Runtime</span></td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </TrackerShell>
  );
};

export const PendingTransactionsPage = () => {
  const [txs, setTxs] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    try {
      setError('');
      const [mempool, recent] = await Promise.allSettled([
        fetchJSON('/mempool', { cacheTtlMs: 1000, timeoutMs: 8000 }),
        fetchJSON('/transactions/recent', { cacheTtlMs: 1000, timeoutMs: 8000 }),
      ]);
      const pending = [];
      if (mempool.status === 'fulfilled') pending.push(...(mempool.value.transactions || []));
      if (recent.status === 'fulfilled') {
        const recentList = Array.isArray(recent.value) ? recent.value : mergeArrayResults(recent.value, 'tx_hash');
        pending.push(...recentList.filter(isPending));
      }
      const seen = new Map();
      pending.map(normalizeTx).forEach((tx) => {
        if (tx.hash) seen.set(tx.hash, tx);
      });
      setTxs(Array.from(seen.values()).sort((a, b) => (b.timestamp || 0) - (a.timestamp || 0)));
    } catch (err) {
      setError(err.message || 'Failed to load pending transactions');
      setTxs([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
    const id = setInterval(load, 5000);
    return () => clearInterval(id);
  }, [load]);

  return (
    <TransactionTracker
      title="Pending Transactions"
      subtitle="Live mempool and pending transaction queue."
      txs={txs}
      loading={loading}
      error={error}
      empty="No pending transactions right now"
    />
  );
};

export const InternalTransactionsPage = () => {
  const [txs, setTxs] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    try {
      setError('');
      const result = await fetchHistoricalTransactionPage(1, 80, { timeoutMs: 12000 });
      setTxs(result.transactions.filter(isInternalTx).map(normalizeTx));
    } catch (err) {
      setError(err.message || 'Failed to load internal transactions');
      setTxs([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
    const id = setInterval(load, 15000);
    return () => clearInterval(id);
  }, [load]);

  return (
    <TransactionTracker
      title="Internal Transactions"
      subtitle="System, reward, contract-create, and internal bookkeeping transactions."
      txs={txs}
      loading={loading}
      error={error}
      empty="No internal transactions found in the latest block range"
    />
  );
};

const TransactionTracker = ({ title, subtitle, txs, loading, error, empty }) => (
  <TrackerShell title={title} subtitle={subtitle} count={txs.length} loading={loading} error={error}>
    <div className="tracker-table-wrap">
      <table className="tracker-table">
        <thead>
          <tr>
            <th>Txn Hash</th>
            <th>Block</th>
            <th>Type</th>
            <th>From</th>
            <th>To</th>
            <th>Value</th>
            <th>Status</th>
          </tr>
        </thead>
        <tbody>
          {txs.length === 0 ? (
            <EmptyRow colSpan={7} label={empty} />
          ) : txs.map((tx, index) => (
            <tr key={tx.hash || index}>
              <td><Link to={`/tx/${tx.hash}`}>{shortHash(tx.hash)}</Link></td>
              <td>{tx.block !== '-' ? <Link to={`/blocks/${tx.block}`}>#{tx.block}</Link> : '-'}</td>
              <td><span className="badge badge-cyan">{tx.type}</span></td>
              <td>{tx.from ? shortHash(tx.from) : '-'}</td>
              <td>{tx.to ? shortHash(tx.to) : '-'}</td>
              <td>{formatLQD(tx.value)}</td>
              <td><span className={`badge ${isSuccess(tx.status) ? 'badge-green' : isFailed(tx.status) ? 'badge-red' : 'badge-yellow'}`}>{tx.status}</span></td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  </TrackerShell>
);
