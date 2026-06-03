import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import {
  API_BASE,
  CHAIN_BASE,
  DEX_REGISTRY_BASE,
  fetchHistoricalTransactionPage,
  fetchDexRegistryPools,
  fetchDexRegistryTokens,
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
  const [registryTokens, setRegistryTokens] = useState([]);
  const [registryError, setRegistryError] = useState('');
  const tokens = useMemo(() => {
    const byAddress = new Map();
    registryTokens.forEach((token) => {
      const address = token.address || token.contract || '';
      if (!address) return;
      byAddress.set(String(address).toLowerCase(), {
        address,
        type: token.native ? 'native' : 'registry_token',
        owner: token.owner || '',
        name: token.name || token.symbol || 'Registry Token',
        symbol: token.symbol || '',
        decimals: token.decimals || '8',
        totalSupply: token.total_supply || token.totalSupply || '',
        registry: true,
        verified: token.verified !== false,
      });
    });
    contracts.filter(isTokenContract).forEach((token) => {
      if (!token.address) return;
      byAddress.set(String(token.address).toLowerCase(), {
        ...(byAddress.get(String(token.address).toLowerCase()) || {}),
        ...token,
      });
    });
    return Array.from(byAddress.values());
  }, [contracts, registryTokens]);

  useEffect(() => {
    let alive = true;
    fetchDexRegistryTokens()
      .then((items) => { if (alive) setRegistryTokens(items); })
      .catch((err) => { if (alive) setRegistryError(err.message || 'DEX registry unavailable'); });
    return () => { alive = false; };
  }, []);

  return (
    <TrackerShell
      title="Token Tracker"
      subtitle="All detected LQD token contracts from deployed contract registry."
      count={tokens.length}
      loading={loading}
      error={error}
    >
      {registryError && <div className="tracker-warning">DEX registry: {registryError}</div>}
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
                <td><span className={`badge ${token.registry ? 'badge-teal' : 'badge-blue'}`}>{token.registry ? 'verified registry' : token.type}</span></td>
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
  const [registryPools, setRegistryPools] = useState([]);
  const [summary, setSummary] = useState({ total: 0, target: 0, unallocated: 0 });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    try {
      setError('');
      const [data, registry] = await Promise.all([
        fetchJSON('/liquidity/pools', { cacheTtlMs: 4000, timeoutMs: 10000 }),
        fetchDexRegistryPools().catch(() => []),
      ]);
      const payload = firstNodeResult(data) || {};
      const entries = Object.entries(payload.pools || {}).map(([address, liquidity]) => ({
        address,
        liquidity,
      }));
      setPools(entries);
      setRegistryPools(registry);
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
      {registryPools.length > 0 && (
        <div className="tracker-stat-grid">
          {registryPools.map((pool) => (
            <div key={pool.address || `${pool.token_a}-${pool.token_b}`}>
              {pool.token_a || 'LQD'} / {pool.token_b || pool.symbol || 'Token'}
              <strong>{pool.tier || 'Tier 3'} · {pool.weight || '0.35x'}</strong>
            </div>
          ))}
        </div>
      )}
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
      try {
        const data = await fetchJSON('/transactions/pending?page=1&size=80', { cacheTtlMs: 1000, timeoutMs: 8000 });
        const list = Array.isArray(data) ? data : data.transactions || mergeArrayResults(data, 'tx_hash');
        setTxs(list.map(normalizeTx));
        return;
      } catch {
        // Older nodes may not expose the dedicated endpoint yet; fall back to live mempool + recent merge.
      }
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
      try {
        const data = await fetchJSON('/transactions/internal?page=1&size=80', { cacheTtlMs: 4000, timeoutMs: 10000 });
        const list = Array.isArray(data) ? data : data.transactions || mergeArrayResults(data, 'tx_hash');
        setTxs(list.map(normalizeTx));
        return;
      } catch {
        // Keep explorer compatible with older backend builds.
      }
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

const isTokenTransferTx = (tx = {}) => {
  const item = normalizeTx(tx);
  const type = String(item.type || '').toLowerCase();
  const fn = String(item.functionName || '').toLowerCase();
  return type.includes('token') || fn.includes('transfer') || fn.includes('approve') || item.isContract;
};

const isNftTx = (tx = {}, mode = '') => {
  const item = normalizeTx(tx);
  const haystack = `${item.type} ${item.functionName}`.toLowerCase();
  if (!haystack.includes('nft') && !haystack.includes('mint') && !haystack.includes('trade')) return false;
  if (mode === 'mints') return haystack.includes('mint');
  if (mode === 'trades') return haystack.includes('trade') || haystack.includes('sale');
  if (mode === 'transfers') return haystack.includes('transfer') || haystack.includes('nft');
  return true;
};

const InfoCard = ({ title, value, children }) => (
  <div className="tracker-info-card">
    <span>{title}</span>
    {value && <strong>{value}</strong>}
    {children}
  </div>
);

export const TokenTransfersPage = () => {
  const [txs, setTxs] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    try {
      setError('');
      const data = await fetchJSON('/transactions?page=1&size=150&include_pending=false', { cacheTtlMs: 3000, timeoutMs: 12000 });
      const list = Array.isArray(data) ? data : data.transactions || mergeArrayResults(data, 'tx_hash');
      setTxs(list.filter(isTokenTransferTx).map(normalizeTx));
    } catch (err) {
      setError(err.message || 'Failed to load token transfers');
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
      title="Token Transfers"
      subtitle="Token contract transfer, approve, and contract-call activity from the live chain."
      txs={txs}
      loading={loading}
      error={error}
      empty="No token transfers found in the latest indexed range"
    />
  );
};

export const TokenFlowPage = () => {
  const [tokens, setTokens] = useState([]);
  const [pools, setPools] = useState([]);
  const [txs, setTxs] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    try {
      setError('');
      const [registryTokens, registryPools, recent] = await Promise.all([
        fetchDexRegistryTokens().catch(() => []),
        fetchDexRegistryPools().catch(() => []),
        fetchJSON('/transactions/recent', { cacheTtlMs: 2000, timeoutMs: 10000 }).catch(() => []),
      ]);
      const recentList = Array.isArray(recent) ? recent : mergeArrayResults(recent, 'tx_hash');
      setTokens(registryTokens);
      setPools(registryPools);
      setTxs(recentList.filter(isTokenTransferTx).map(normalizeTx).slice(0, 20));
    } catch (err) {
      setError(err.message || 'Failed to load token flow');
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
    <TrackerShell title="Token Flow Visualizer" subtitle="Registry tokens, pool coverage, and recent token movement." count={tokens.length} loading={loading} error={error}>
      <div className="tracker-card-grid">
        <InfoCard title="Registry Tokens" value={tokens.length.toLocaleString()} />
        <InfoCard title="Approved Pools" value={pools.filter((p) => p.approved !== false).length.toLocaleString()} />
        <InfoCard title="Recent Token Events" value={txs.length.toLocaleString()} />
      </div>
      <TransactionTracker title="Recent Token Flow" subtitle="Latest token-like transactions" txs={txs} loading={false} error="" empty="No token flow found yet" />
    </TrackerShell>
  );
};

export const NFTTrackerPage = () => {
  const { contracts, loading, error } = useContracts();
  const nfts = contracts.filter((contract) => String(contract.type || '').toLowerCase().includes('nft'));

  return (
    <TrackerShell title="NFT Tracker" subtitle="NFT collection contracts discovered on LQD." count={nfts.length} loading={loading} error={error}>
      <div className="tracker-table-wrap">
        <table className="tracker-table">
          <thead>
            <tr><th>#</th><th>Collection</th><th>Symbol</th><th>Contract</th><th>Owner</th></tr>
          </thead>
          <tbody>
            {nfts.length === 0 ? <EmptyRow colSpan={5} label="No NFT collections found yet" /> : nfts.map((nft, index) => (
              <tr key={nft.address || index}>
                <td>{index + 1}</td>
                <td>{nft.name || 'NFT Collection'}</td>
                <td>{nft.symbol || '-'}</td>
                <td><Link to={`/address/${nft.address}`}>{shortHash(nft.address)}</Link></td>
                <td>{nft.owner ? shortHash(nft.owner) : '-'}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </TrackerShell>
  );
};

export const NFTActivityPage = ({ mode = 'activity' }) => {
  const [txs, setTxs] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    try {
      setError('');
      const data = await fetchJSON('/transactions?page=1&size=150&include_pending=false', { cacheTtlMs: 3000, timeoutMs: 12000 });
      const list = Array.isArray(data) ? data : data.transactions || mergeArrayResults(data, 'tx_hash');
      setTxs(list.filter((tx) => isNftTx(tx, mode)).map(normalizeTx));
    } catch (err) {
      setError(err.message || 'Failed to load NFT activity');
      setTxs([]);
    } finally {
      setLoading(false);
    }
  }, [mode]);

  useEffect(() => {
    load();
    const id = setInterval(load, 15000);
    return () => clearInterval(id);
  }, [load]);

  const title = mode === 'mints' ? 'NFT Mints' : mode === 'trades' ? 'NFT Trades' : mode === 'transfers' ? 'NFT Transfers' : 'NFT Activity';
  return <TransactionTracker title={title} subtitle="NFT-related transactions from indexed chain data." txs={txs} loading={loading} error={error} empty="No NFT activity found yet" />;
};

export const BridgeTransactionsPage = () => {
  const [requests, setRequests] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    try {
      setError('');
      const data = await fetchJSON('/bridge/requests', { cacheTtlMs: 4000, timeoutMs: 10000 });
      setRequests(Array.isArray(data) ? data : mergeArrayResults(data, 'request_id'));
    } catch (err) {
      setError(err.message || 'Failed to load bridge transactions');
      setRequests([]);
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
    <TrackerShell title="Cross-Chain Transactions" subtitle="Bridge request queue, status, source, and target chain metadata." count={requests.length} loading={loading} error={error}>
      <div className="tracker-table-wrap">
        <table className="tracker-table">
          <thead>
            <tr><th>#</th><th>Request</th><th>Direction</th><th>Token</th><th>Amount</th><th>Status</th></tr>
          </thead>
          <tbody>
            {requests.length === 0 ? <EmptyRow colSpan={6} label="No bridge requests found yet" /> : requests.map((req, index) => (
              <tr key={req.request_id || req.id || index}>
                <td>{index + 1}</td>
                <td>{shortHash(req.request_id || req.id || req.tx_hash)}</td>
                <td>{req.direction || `${req.source_chain || 'LQD'} → ${req.target_chain || 'External'}`}</td>
                <td>{req.token || req.symbol || '-'}</td>
                <td>{req.amount || '-'}</td>
                <td><span className="badge badge-yellow">{req.status || 'queued'}</span></td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </TrackerShell>
  );
};

export const TopAccountsPage = () => {
  const [accounts, setAccounts] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    try {
      setError('');
      const [validators, lps] = await Promise.all([
        fetchJSON('/validators', { cacheTtlMs: 4000, timeoutMs: 10000 }).catch(() => []),
        fetchJSON('/liquidity/all', { cacheTtlMs: 4000, timeoutMs: 10000 }).catch(() => []),
      ]);
      const byAddress = new Map();
      (Array.isArray(validators) ? validators : []).forEach((v) => {
        const address = v.address || '';
        if (!address) return;
        byAddress.set(address.toLowerCase(), { address, role: 'Validator', score: v.liquidity_power || v.stake || 0, status: v.node_status || 'registered' });
      });
      (Array.isArray(lps) ? lps : mergeArrayResults(lps, 'address')).forEach((lp) => {
        const address = lp.address || lp.Address || '';
        if (!address) return;
        const existing = byAddress.get(address.toLowerCase()) || { address, role: 'LP Provider', score: 0, status: 'active' };
        existing.role = existing.role === 'Validator' ? 'Validator + LP' : 'LP Provider';
        existing.score = Math.max(Number(existing.score || 0), Number(lp.liquidity_power || lp.LiquidityPower || 0));
        byAddress.set(address.toLowerCase(), existing);
      });
      setAccounts(Array.from(byAddress.values()).sort((a, b) => Number(b.score || 0) - Number(a.score || 0)));
    } catch (err) {
      setError(err.message || 'Failed to load accounts');
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
    <TrackerShell title="Top Accounts" subtitle="Validator and LP provider accounts ranked by available protocol power." count={accounts.length} loading={loading} error={error}>
      <div className="tracker-table-wrap">
        <table className="tracker-table">
          <thead>
            <tr><th>#</th><th>Address</th><th>Role</th><th>Power</th><th>Status</th></tr>
          </thead>
          <tbody>
            {accounts.length === 0 ? <EmptyRow colSpan={5} label="No ranked accounts found yet" /> : accounts.map((account, index) => (
              <tr key={account.address}>
                <td>{index + 1}</td>
                <td><Link to={`/address/${account.address}`}>{account.address}</Link></td>
                <td>{account.role}</td>
                <td>{formatLQD(account.score)}</td>
                <td><span className="badge badge-teal">{account.status}</span></td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </TrackerShell>
  );
};

export const ChartsStatsPage = () => {
  const [stats, setStats] = useState({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    try {
      setError('');
      const [network, summary, basefee, blocktime] = await Promise.allSettled([
        fetchJSON('/network', { cacheTtlMs: 4000, timeoutMs: 10000 }),
        fetchJSON('/chain/summary', { cacheTtlMs: 4000, timeoutMs: 10000 }),
        fetchJSON('/basefee', { cacheTtlMs: 4000, timeoutMs: 10000 }),
        fetchJSON('/blocktime/latest', { cacheTtlMs: 4000, timeoutMs: 10000 }),
      ]);
      setStats({
        network: network.status === 'fulfilled' ? firstNodeResult(network.value) || network.value : {},
        summary: summary.status === 'fulfilled' ? firstNodeResult(summary.value) || summary.value : {},
        basefee: basefee.status === 'fulfilled' ? firstNodeResult(basefee.value) || basefee.value : {},
        blocktime: blocktime.status === 'fulfilled' ? firstNodeResult(blocktime.value) || blocktime.value : {},
      });
    } catch (err) {
      setError(err.message || 'Failed to load charts and stats');
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
    <TrackerShell title="Charts & Stats" subtitle="Live chain health, block timing, fees, and network summary." loading={loading} error={error}>
      <div className="tracker-card-grid">
        <InfoCard title="Height" value={String(stats.summary?.height || stats.network?.height || '-')} />
        <InfoCard title="Peers" value={String(stats.network?.peers || stats.network?.peer_count || 0)} />
        <InfoCard title="Base Fee" value={String(stats.basefee?.base_fee || stats.basefee?.baseFee || '-')} />
        <InfoCard title="Block Time" value={String(stats.blocktime?.block_time || stats.blocktime?.average_block_time || '-')} />
      </div>
    </TrackerShell>
  );
};

export const ApiDocsPage = () => (
  <TrackerShell title="API Documentation" subtitle="Production endpoints available to explorer, wallets, DEX, and integrations." loading={false} error="">
    <div className="tracker-card-grid">
      <InfoCard title="Chain API" value={CHAIN_BASE}>
        <code>/health</code><code>/transactions</code><code>/fetch_last_n_block</code><code>/contract/call</code>
      </InfoCard>
      <InfoCard title="Aggregator API" value={API_BASE}>
        <code>/network</code><code>/transactions/recent</code><code>/liquidity/all</code><code>/rewards/latest</code>
      </InfoCard>
      <InfoCard title="DEX Registry API" value={DEX_REGISTRY_BASE}>
        <code>/tokens</code><code>/pools</code><code>/config</code>
      </InfoCard>
    </div>
  </TrackerShell>
);

export const BroadcastTransactionPage = () => (
  <TrackerShell title="Broadcast Transaction" subtitle="Wallet-signed transaction submission endpoints for LQD network." loading={false} error="">
    <div className="tracker-card-grid">
      <InfoCard title="Single Transaction" value={`${CHAIN_BASE}/send_tx`} />
      <InfoCard title="Batch Transactions" value={`${CHAIN_BASE}/send_tx/batch`} />
      <InfoCard title="JSON-RPC" value={`${CHAIN_BASE}/rpc`} />
    </div>
    <div className="tracker-warning">For safety, raw broadcast should be done from LQD wallet, extension, mobile wallet, or trusted backend tooling.</div>
  </TrackerShell>
);

export const DeveloperContractToolsPage = ({ mode = 'search' }) => (
  <TrackerShell
    title={mode === 'verify' ? 'Verify Contract' : 'Smart Contract Search'}
    subtitle={mode === 'verify' ? 'Compiled/deployed contracts can be inspected through ABI, code, storage, and events APIs.' : 'Search deployed contracts from the registry and inspect runtime metadata.'}
    loading={false}
    error=""
  >
    <div className="tracker-card-grid">
      <InfoCard title="Contract Registry" value="/contract/list" />
      <InfoCard title="ABI Lookup" value="/contract/getAbi?address=0x..." />
      <InfoCard title="Storage" value="/contract/storage?address=0x..." />
      <InfoCard title="Code" value="/contract/code?address=0x..." />
    </div>
    <ContractTrackerPage />
  </TrackerShell>
);

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
