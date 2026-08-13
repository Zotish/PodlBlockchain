import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import {
  fetchDexRegistryPools,
  fetchDexRegistryTokens,
  fetchHistoricalTransactionPage,
  fetchJSON,
  firstNodeResult,
  mergeArrayResults,
} from '../utils/api';
import { formatLQD, toBigIntSafe } from '../utils/lqdUnits';

const ZERO_ADDRESS = '0x0000000000000000000000000000000000000000';

const lower = (value = '') => String(value || '').toLowerCase();

const sameAddress = (a = '', b = '') => lower(a) === lower(b);

const hasAddress = (value = '') => /^0x[a-fA-F0-9]{40}$/.test(String(value || '').trim());

const shortHash = (value = '') => {
  const text = String(value || '');
  if (!text) return '-';
  if (text.length <= 18) return text;
  return `${text.slice(0, 10)}...${text.slice(-8)}`;
};

const compactNumber = (value) => {
  const num = Number(value);
  if (!Number.isFinite(num)) return '-';
  return new Intl.NumberFormat('en-US', { maximumFractionDigits: 4 }).format(num);
};

const safeText = (value, fallback = '-') => {
  const text = String(value ?? '').trim();
  return text || fallback;
};

const normalizeContract = (contract = {}) => ({
  address: contract.address || contract.Address || '',
  type: contract.type || contract.Type || 'custom',
  owner: contract.owner || contract.Owner || '',
  name: contract.name || contract.Name || '',
  symbol: contract.symbol || contract.Symbol || '',
  decimals: contract.decimals ?? contract.Decimals ?? '',
  totalSupply: contract.totalSupply || contract.total_supply || contract.TotalSupply || '',
  verified: contract.verified ?? contract.Verified,
});

const normalizeRegistryToken = (token = {}) => ({
  address: token.address || token.contract || '',
  name: token.name || token.symbol || '',
  symbol: token.symbol || '',
  decimals: token.decimals ?? 8,
  totalSupply: token.total_supply || token.totalSupply || token.supply || '',
  logoUrl: token.logo_url || token.logoUrl || token.icon || '',
  verified: token.verified !== false,
  native: Boolean(token.native),
  priceUsd: token.price_usd || token.priceUsd || token.price || '',
  priceChange24h: token.price_change_24h || token.priceChange24h || token.change_24h || '',
});

const normalizeTx = (tx = {}) => ({
  hash: tx.tx_hash || tx.txHash || tx.TxHash || tx.hash || '',
  from: tx.from || tx.From || '',
  to: tx.to || tx.To || '',
  status: tx.status || tx.Status || 'pending',
  type: tx.type || tx.Type || tx.tx_type || tx.function || tx.Function || 'transfer',
  value: tx.value ?? tx.Value ?? 0,
  gas: tx.gas ?? tx.Gas ?? 0,
  gasPrice: tx.gas_price ?? tx.gasPrice ?? tx.GasPrice ?? 0,
  block: tx.block_number ?? tx.BlockNumber ?? tx.block ?? '-',
  timestamp: tx.timestamp ?? tx.Timestamp ?? 0,
  functionName: tx.function || tx.Function || '',
});

const extractStorage = (data) => {
  const primary = firstNodeResult(data);
  if (!primary) return {};
  return primary.storage || primary.Storage || primary.state || primary.State || primary;
};

const readStorageValue = (storage = {}, keys = []) => {
  for (const key of keys) {
    if (storage[key] !== undefined && storage[key] !== null) return storage[key];
  }
  const found = Object.entries(storage).find(([key]) => keys.some((name) => lower(key) === lower(name)));
  return found ? found[1] : undefined;
};

const formatTokenAmount = (value, decimals = 8, fallback = '-') => {
  if (value === undefined || value === null || value === '') return fallback;
  try {
    return formatLQD(value, Number(decimals) || 8);
  } catch {
    return String(value);
  }
};

const extractHolderValue = (value) => {
  if (value && typeof value === 'object') {
    return value.balance ?? value.Balance ?? value.amount ?? value.Amount ?? value.value ?? value.Value ?? 0;
  }
  return value;
};

const parseHolders = (storage = {}, decimals = 8) => {
  const balances = new Map();
  const addBalance = (address, value) => {
    if (!hasAddress(address)) return;
    const amount = toBigIntSafe(extractHolderValue(value), Number(decimals) || 8);
    if (amount <= 0n) return;
    const key = lower(address);
    const current = balances.get(key) || { address, amount: 0n };
    current.amount += amount;
    balances.set(key, current);
  };

  const nested = [
    storage.balances,
    storage.Balances,
    storage.balanceOf,
    storage.BalanceOf,
    storage.holders,
    storage.Holders,
  ].filter(Boolean);

  nested.forEach((entry) => {
    if (!entry || typeof entry !== 'object') return;
    Object.entries(entry).forEach(([address, value]) => addBalance(address, value));
  });

  Object.entries(storage || {}).forEach(([key, value]) => {
    if (hasAddress(key)) {
      addBalance(key, value);
      return;
    }
    const match = String(key).match(/0x[a-fA-F0-9]{40}/);
    if (match && /bal|holder|account|owner/i.test(key)) addBalance(match[0], value);
  });

  return Array.from(balances.values())
    .sort((a, b) => (a.amount === b.amount ? 0 : a.amount > b.amount ? -1 : 1))
    .map((holder, index) => ({
      rank: index + 1,
      address: holder.address,
      amount: holder.amount.toString(),
      formatted: formatTokenAmount(holder.amount, decimals, '0'),
    }));
};

const txMatchesToken = (tx, tokenAddress) => {
  const target = lower(tokenAddress);
  if (!target) return false;
  const item = normalizeTx(tx);
  if (sameAddress(item.from, target) || sameAddress(item.to, target)) return true;
  const contract = tx.contract || tx.contract_address || tx.ContractAddress || tx.token || tx.token_address;
  if (sameAddress(contract, target)) return true;
  return lower(JSON.stringify(tx || {})).includes(target);
};

const statusLabel = (status = '') => {
  const value = lower(status);
  if (value === 'succsess' || value === 'success') return 'Success';
  if (value === 'failed') return 'Failed';
  return 'Pending';
};

const TokenMetric = ({ label, value, detail }) => (
  <div className="token-metric-card">
    <span>{label}</span>
    <strong>{value}</strong>
    {detail && <small>{detail}</small>}
  </div>
);

const TokenPage = () => {
  const { address = '' } = useParams();
  const [contracts, setContracts] = useState([]);
  const [registryTokens, setRegistryTokens] = useState([]);
  const [registryPools, setRegistryPools] = useState([]);
  const [storage, setStorage] = useState({});
  const [indexedHolders, setIndexedHolders] = useState([]);
  const [verification, setVerification] = useState(null);
  const [verifySource, setVerifySource] = useState('');
  const [verifyMessage, setVerifyMessage] = useState('');
  const [transactions, setTransactions] = useState([]);
  const [activeTab, setActiveTab] = useState('transfers');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    if (!address) return;
    try {
      setLoading(true);
      setError('');
      const [contractData, registryTokenData, registryPoolData, storageData, txPage, holderData, verificationData] = await Promise.all([
        fetchJSON('/contract/list', { cacheTtlMs: 5000, timeoutMs: 10000 }).catch(() => []),
        fetchDexRegistryTokens().catch(() => []),
        fetchDexRegistryPools().catch(() => []),
        fetchJSON(`/contract/storage?address=${address}`, { cacheTtlMs: 5000, timeoutMs: 10000 }).catch(() => ({})),
        fetchHistoricalTransactionPage(1, 250).catch(() => ({ transactions: [] })),
        fetchJSON(`/token/${address}/holders`, { cacheTtlMs: 5000, timeoutMs: 10000 }).catch(() => null),
        fetchJSON(`/contract/verification?address=${address}`, { cacheTtlMs: 5000, timeoutMs: 10000 }).catch(() => null),
      ]);
      setContracts((Array.isArray(contractData) ? contractData : mergeArrayResults(contractData, 'address')).map(normalizeContract));
      setRegistryTokens(Array.isArray(registryTokenData) ? registryTokenData.map(normalizeRegistryToken) : []);
      setRegistryPools(Array.isArray(registryPoolData) ? registryPoolData : []);
      setStorage(extractStorage(storageData));
      setIndexedHolders(Array.isArray(holderData?.holders) ? holderData.holders : []);
      setVerification(verificationData?.contract || null);
      setTransactions((txPage.transactions || []).filter((tx) => txMatchesToken(tx, address)));
    } catch (err) {
      setError(err.message || 'Failed to load token details');
    } finally {
      setLoading(false);
    }
  }, [address]);

  useEffect(() => {
    load();
  }, [load]);

  const token = useMemo(() => {
    const contract = contracts.find((item) => sameAddress(item.address, address)) || {};
    const registry = registryTokens.find((item) => sameAddress(item.address, address)) || {};
    const storageName = readStorageValue(storage, ['name', 'Name', 'token_name', 'TokenName']);
    const storageSymbol = readStorageValue(storage, ['symbol', 'Symbol', 'ticker', 'Ticker']);
    const storageDecimals = readStorageValue(storage, ['decimals', 'Decimals']);
    const storageSupply = readStorageValue(storage, ['totalSupply', 'total_supply', 'TotalSupply', 'supply', 'Supply', 'maxSupply']);
    return {
      ...contract,
      ...registry,
      address,
      name: registry.name || contract.name || storageName || 'LQD Token',
      symbol: registry.symbol || contract.symbol || storageSymbol || 'TOKEN',
      decimals: registry.decimals || contract.decimals || storageDecimals || 8,
      totalSupply: registry.totalSupply || contract.totalSupply || storageSupply || '',
      type: contract.type || (registry.native ? 'native' : 'token'),
      verified: registry.verified || contract.verified === true,
      logoUrl: registry.logoUrl || '',
      priceUsd: registry.priceUsd || '',
      priceChange24h: registry.priceChange24h || '',
    };
  }, [address, contracts, registryTokens, storage]);

  const holders = useMemo(() => {
    if (indexedHolders.length) {
      return indexedHolders.map((holder, index) => ({
        rank: holder.rank || index + 1,
        address: holder.address,
        amount: holder.amount || '0',
        formatted: formatTokenAmount(holder.amount || '0', token.decimals, '0'),
      }));
    }
    return parseHolders(storage, token.decimals);
  }, [indexedHolders, storage, token.decimals]);

  const submitVerification = async () => {
    setVerifyMessage('');
    if (!verifySource.trim()) {
      setVerifyMessage('Source code required');
      return;
    }
    try {
      const data = await fetchJSON('/contract/verify', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          address,
          source_code: verifySource,
          compiler_version: 'lqd-go-plugin',
          optimization: false,
        }),
        timeoutMs: 15000,
      });
      setVerifyMessage(`Verified: ${data.source_hash || 'source matched'}`);
      await load();
    } catch (err) {
      setVerifyMessage(err.message || 'Verification failed');
    }
  };

  const pools = useMemo(() => {
    const target = lower(address);
    return registryPools.filter((pool) => {
      const tokenA = pool.token_a || pool.tokenA || pool.token0 || pool.TokenA || '';
      const tokenB = pool.token_b || pool.tokenB || pool.token1 || pool.TokenB || '';
      const poolAddress = pool.address || pool.pool_address || '';
      return lower(tokenA) === target || lower(tokenB) === target || lower(poolAddress) === target;
    });
  }, [address, registryPools]);

  const formattedSupply = formatTokenAmount(token.totalSupply, token.decimals);
  const priceText = token.priceUsd ? `$${compactNumber(token.priceUsd)}` : 'Price unavailable';
  const changeText = token.priceChange24h !== '' ? `${Number(token.priceChange24h) >= 0 ? '+' : ''}${compactNumber(token.priceChange24h)}%` : '24h --';
  const tabs = [
    { id: 'transfers', label: 'Transfers' },
    { id: 'holders', label: 'Holders' },
    { id: 'contract', label: 'Contract' },
    { id: 'pools', label: 'DEX Pools' },
  ];

  if (loading) {
    return <div className="loading">Loading token details...</div>;
  }

  return (
    <div className="token-detail-page">
      {error && <div className="error">{error}</div>}

      <section className="token-hero-card">
        <div className="token-identity">
          <div className="token-logo-xl">
            {token.logoUrl ? <img src={token.logoUrl} alt={`${token.symbol} logo`} /> : <span>{safeText(token.symbol, 'T').slice(0, 1)}</span>}
          </div>
          <div>
            <p className="section-title">Token Tracker</p>
            <h1>
              {safeText(token.name)}
              <span>{safeText(token.symbol)}</span>
              {token.verified && <em>Verified</em>}
            </h1>
            <div className="token-contract-line">
              <span>Contract</span>
              <code>{address}</code>
              <Link to={`/address/${address}`}>Address view</Link>
            </div>
          </div>
        </div>
        <div className="token-price-panel">
          <span>Token Price</span>
          <strong>{priceText}</strong>
          <small className={Number(token.priceChange24h) >= 0 ? 'positive' : 'negative'}>{changeText}</small>
        </div>
      </section>

      <section className="token-overview-grid">
        <TokenMetric label="Total Supply" value={formattedSupply} detail={token.symbol} />
        <TokenMetric label="Decimals" value={safeText(token.decimals)} />
        <TokenMetric label="Holders" value={holders.length.toLocaleString()} detail="detected from contract storage" />
        <TokenMetric label="Transfers" value={transactions.length.toLocaleString()} detail="recent indexed activity" />
        <TokenMetric label="DEX Pools" value={pools.length.toLocaleString()} detail="registry matched" />
        <TokenMetric label="Contract Type" value={safeText(token.type)} />
      </section>

      <div className="token-tabs">
        {tabs.map((tab) => (
          <button
            key={tab.id}
            type="button"
            className={activeTab === tab.id ? 'active' : ''}
            onClick={() => setActiveTab(tab.id)}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {activeTab === 'transfers' && (
        <section className="token-panel">
          <div className="token-panel-heading">
            <h2>Token Transfers</h2>
            <p>Recent transactions that reference this token contract.</p>
          </div>
          <div className="tracker-table-wrap">
            <table className="tracker-table">
              <thead>
                <tr>
                  <th>Txn Hash</th>
                  <th>Method</th>
                  <th>Block</th>
                  <th>From</th>
                  <th>To</th>
                  <th>Value</th>
                  <th>Status</th>
                </tr>
              </thead>
              <tbody>
                {transactions.length === 0 ? (
                  <tr><td colSpan={7} className="tracker-empty">No token transfers found yet</td></tr>
                ) : transactions.map((tx, index) => {
                  const item = normalizeTx(tx);
                  return (
                    <tr key={item.hash || index}>
                      <td><Link to={`/tx/${item.hash}`}>{shortHash(item.hash)}</Link></td>
                      <td><span className="badge badge-blue">{safeText(item.functionName || item.type)}</span></td>
                      <td>{item.block}</td>
                      <td>{sameAddress(item.from, ZERO_ADDRESS) ? 'Mint' : <Link to={`/address/${item.from}`}>{shortHash(item.from)}</Link>}</td>
                      <td><Link to={`/address/${item.to}`}>{shortHash(item.to)}</Link></td>
                      <td>{formatTokenAmount(item.value, token.decimals, '0')} {token.symbol}</td>
                      <td><span className={`badge ${statusLabel(item.status) === 'Success' ? 'badge-teal' : 'badge-yellow'}`}>{statusLabel(item.status)}</span></td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </section>
      )}

      {activeTab === 'holders' && (
        <section className="token-panel">
          <div className="token-panel-heading">
            <h2>Holders</h2>
            <p>Detected balances from token storage. Older contracts may expose partial holder data.</p>
          </div>
          <div className="tracker-table-wrap">
            <table className="tracker-table">
              <thead>
                <tr>
                  <th>#</th>
                  <th>Address</th>
                  <th>Quantity</th>
                  <th>Percentage</th>
                </tr>
              </thead>
              <tbody>
                {holders.length === 0 ? (
                  <tr><td colSpan={4} className="tracker-empty">No holder balances available from storage</td></tr>
                ) : holders.slice(0, 100).map((holder) => {
                  const total = toBigIntSafe(token.totalSupply, Number(token.decimals) || 8);
                  const pct = total > 0n ? `${((Number(holder.amount) / Number(total)) * 100).toFixed(4)}%` : '-';
                  return (
                    <tr key={holder.address}>
                      <td>{holder.rank}</td>
                      <td><Link to={`/address/${holder.address}`}>{holder.address}</Link></td>
                      <td>{holder.formatted} {token.symbol}</td>
                      <td>{pct}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </section>
      )}

      {activeTab === 'contract' && (
        <section className="token-panel">
          <div className="token-panel-heading">
            <h2>Contract</h2>
            <p>Contract metadata, registry status, and raw storage preview.</p>
          </div>
          <div className="token-contract-grid">
            <TokenMetric label="Owner" value={token.owner ? shortHash(token.owner) : '-'} />
            <TokenMetric label="Verified Registry" value={(verification?.verified || token.verified) ? 'Yes' : 'No'} />
            <TokenMetric label="Contract Address" value={shortHash(address)} />
            <TokenMetric label="Storage Keys" value={(verification?.storage_keys ?? Object.keys(storage || {}).length).toLocaleString()} />
            <TokenMetric label="ABI Methods" value={verification?.abi_count ?? '-'} />
            <TokenMetric label="Code Hash" value={verification?.code_hash ? shortHash(verification.code_hash) : '-'} />
          </div>
          <div className="contract-verify-box">
            <h3>Contract Verification</h3>
            <textarea
              value={verifySource}
              onChange={(e) => setVerifySource(e.target.value)}
              placeholder="Paste deployed Go/DSL source code to verify against stored code hash"
            />
            <button type="button" onClick={submitVerification}>Verify Source</button>
            {verifyMessage && <p>{verifyMessage}</p>}
          </div>
          <pre className="token-storage-preview">{JSON.stringify(storage || {}, null, 2).slice(0, 6000)}</pre>
        </section>
      )}

      {activeTab === 'pools' && (
        <section className="token-panel">
          <div className="token-panel-heading">
            <h2>DEX Pools</h2>
            <p>Pool registry entries where this token is paired.</p>
          </div>
          <div className="token-pool-grid">
            {pools.length === 0 ? (
              <div className="tracker-empty">No registered pools found for this token</div>
            ) : pools.map((pool, index) => (
              <div className="token-pool-card" key={pool.address || index}>
                <span>Tier {pool.tier || '-'}</span>
                <strong>{pool.pair_key || pool.pairKey || `${safeText(pool.token_a || pool.tokenA)} / ${safeText(pool.token_b || pool.tokenB)}`}</strong>
                <code>{pool.address || pool.pool_address || '-'}</code>
                <small>Weight {pool.weight || '1.0'}x · {pool.approved === false ? 'Pending' : 'Approved'}</small>
              </div>
            ))}
          </div>
        </section>
      )}
    </div>
  );
};

export default TokenPage;
