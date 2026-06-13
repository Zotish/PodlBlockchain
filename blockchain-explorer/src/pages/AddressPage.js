import React, { useEffect, useMemo, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import TransactionList from '../components/TransactionList';
import { formatLQD } from "../utils/lqdUnits";
import {
  fetchDexRegistryTokens,
  fetchJSON,
  firstNodeResult,
  mergeArrayResults,
} from "../utils/api";

const REFRESH_MS = 5000;
const ZERO_ADDRESS = "0x0000000000000000000000000000000000000000";

const lower = (value = "") => String(value || "").trim().toLowerCase();
const sameAddress = (a, b) => lower(a) === lower(b);
const hasAddress = (value = "") => /^0x[a-fA-F0-9]{40}$/.test(String(value || "").trim());
const hasPositiveBalance = (value = "0") => {
  const text = String(value ?? "0").trim();
  if (!text) return false;
  const numeric = text.startsWith("0x") ? text.slice(2) : text.replace(/[^0-9]/g, "");
  return /[1-9]/.test(numeric);
};

const shortHash = (value = "", start = 10, end = 6) => {
  const text = String(value || "");
  if (text.length <= start + end + 3) return text || "-";
  return `${text.slice(0, start)}...${text.slice(-end)}`;
};

const asArrayPayload = (data, key = "") => {
  if (Array.isArray(data)) return data;
  const primary = firstNodeResult(data);
  if (Array.isArray(primary)) return primary;
  if (key && Array.isArray(primary?.[key])) return primary[key];
  if (key && Array.isArray(data?.[key])) return data[key];
  if (Array.isArray(primary?.contracts)) return primary.contracts;
  if (Array.isArray(data?.contracts)) return data.contracts;
  return mergeArrayResults(data, "address");
};

const normalizeContract = (contract = {}) => ({
  address: contract.address || contract.Address || contract.contract || '',
  type: contract.type || contract.Type || contract.template || contract.Template || 'custom',
  owner: contract.owner || contract.Owner || '',
  name: contract.name || contract.Name || '',
  symbol: contract.symbol || contract.Symbol || '',
  decimals: contract.decimals || contract.Decimals || '',
  totalSupply: contract.totalSupply || contract.total_supply || contract.TotalSupply || '',
});

const normalizeRegistryToken = (token = {}) => ({
  address: token.address || token.contract || token.contract_address || '',
  type: 'token',
  owner: token.owner || '',
  name: token.name || token.symbol || 'Token',
  symbol: token.symbol || '',
  decimals: token.decimals || 8,
  totalSupply: token.total_supply || token.totalSupply || '',
  logoUrl: token.logo_url || token.logoUrl || '',
  verified: token.verified !== false,
  official: true,
});

const normalizeTx = (tx = {}) => ({
  ...tx,
  hash: tx.tx_hash || tx.txHash || tx.TxHash || tx.hash || '',
  from: tx.from || tx.From || '',
  to: tx.to || tx.To || '',
  type: tx.type || tx.Type || tx.tx_type || tx.function || tx.Function || 'transfer',
  functionName: tx.function || tx.Function || '',
  timestamp: tx.timestamp || tx.Timestamp || 0,
  block: tx.block_number ?? tx.BlockNumber ?? tx.block ?? '',
});

const extractStorage = (data = {}) => (
  data?.State?.storage ||
  data?.State ||
  data?.state?.storage ||
  data?.state ||
  data?.storage ||
  data ||
  {}
);

const readStorageValue = (storage = {}, keys = []) => {
  for (const key of keys) {
    if (storage[key] !== undefined) return storage[key];
  }
  const entries = Object.entries(storage);
  for (const key of keys) {
    const found = entries.find(([candidate]) => lower(candidate) === lower(key));
    if (found) return found[1];
  }
  return undefined;
};

const balanceFromStorage = (storage = {}, owner = "") => {
  const original = String(owner || "").trim();
  const addr = lower(original);
  const nestedBalances = storage.balances || storage.Balances || storage.balanceOf || storage.BalanceOf;
  if (nestedBalances && typeof nestedBalances === "object") {
    const nested =
      nestedBalances[original] ??
      nestedBalances[addr] ??
      Object.entries(nestedBalances).find(([key]) => lower(key) === addr)?.[1];
    if (nested !== undefined) return String(nested);
  }

  const keys = [
    `balance:${original}`,
    `balance:${addr}`,
    `balances:${original}`,
    `balances:${addr}`,
    `balance_${original}`,
    `balance_${addr}`,
    `bal:${original}`,
    `bal:${addr}`,
    `__bal:${original}`,
    `__bal:${addr}`,
    original,
    addr,
  ];
  const direct = readStorageValue(storage, keys);
  if (direct !== undefined) return String(direct);

  const hit = Object.entries(storage).find(([key]) => {
    const k = lower(key);
    return k.endsWith(addr) && (k.includes("balance") || k.includes("balances") || k.includes("bal:"));
  });
  return hit ? String(hit[1]) : "0";
};

const nftIdsFromStorage = (storage = {}, owner = "") => {
  const addr = lower(owner);
  return Object.entries(storage)
    .filter(([key, value]) => {
      const k = lower(key);
      return (k.startsWith("owner:") || k.startsWith("ownerof:")) && lower(value) === addr;
    })
    .map(([key]) => String(key).split(":").slice(1).join(":"))
    .filter(Boolean);
};

const isTokenContract = (contract = {}) => {
  const type = lower(contract.type);
  return type.includes("token") && !type.includes("nft");
};

const isNftContract = (contract = {}) => {
  const type = lower(contract.type);
  return type.includes("nft") || type.includes("collection");
};

const txMatchesAddress = (tx, address) => {
  const item = normalizeTx(tx);
  return sameAddress(item.from, address) || sameAddress(item.to, address);
};

const isTokenTx = (tx = {}) => {
  const item = normalizeTx(tx);
  const type = lower(item.type);
  const fn = lower(item.functionName);
  return type.includes("token") || fn.includes("transfer") || fn.includes("approve");
};

const isInternalTx = (tx = {}) => {
  const item = normalizeTx(tx);
  const type = lower(item.type);
  const fn = lower(item.functionName);
  return (
    sameAddress(item.from, ZERO_ADDRESS) ||
    type.includes("reward") ||
    type.includes("internal") ||
    type.includes("contract") ||
    fn === "constructor"
  );
};

const formatTokenAmount = (raw, decimals = 8) => {
  try {
    return formatLQD(raw || "0", Number(decimals) || 8);
  } catch {
    return "0";
  }
};

async function fetchCombinedBalance(address) {
  const original = String(address || '').trim();
  if (!original) return null;
  const primary = firstNodeResult(await fetchJSON(`/balance?address=${encodeURIComponent(original)}`));
  return primary;
}

const AddressPage = () => {
  const { address } = useParams();
  const [addressData, setAddressData] = useState(null);
  const [transactions, setTransactions] = useState([]);
  const [tokenHoldings, setTokenHoldings] = useState([]);
  const [nftHoldings, setNftHoldings] = useState([]);
  const [contractInfo, setContractInfo] = useState(null);
  const [analytics, setAnalytics] = useState(null);
  const [activeTab, setActiveTab] = useState("transactions");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
    let cancelled = false;

    const fetchAddressData = async () => {
      try {
        setError(null);
        const indexed = await fetchJSON(`/address/${address}/overview`, { cacheTtlMs: 2500, timeoutMs: 10000 }).catch(() => null);
        if (indexed && indexed.address) {
          if (cancelled) return;
          const allTxs = Array.isArray(indexed.transactions) ? indexed.transactions : [];
          setAddressData({
            address,
            balance: indexed.balance ?? "0",
            confirmedBalance: indexed.confirmed_balance ?? indexed.balance ?? "0",
            pendingBalance: indexed.pending_balance_change ?? "0",
            isValidator: indexed.is_validator || indexed.isValidator || false
          });
          setTransactions(allTxs.map(normalizeTx));
          setContractInfo(indexed.contract || null);
          setTokenHoldings((indexed.token_balances || []).filter((token) => hasPositiveBalance(token.balance || "0")).map((token) => ({
            ...token,
            decimals: Number(token.decimals || 8) || 8,
            balanceFormatted: formatTokenAmount(token.balance, token.decimals || 8),
          })));
          setNftHoldings((indexed.nft_balances || []).map((nft) => ({
            ...nft,
            tokenIds: nft.token_ids || nft.tokenIds || [],
            count: nft.count || (nft.token_ids || []).length,
          })));
          setAnalytics(indexed.analytics || null);
          setLoading(false);
          return;
        }
        const [balanceResult, txsData, contractData, registryTokens] = await Promise.all([
          fetchCombinedBalance(address),
          fetchJSON(`/address/${address}/transactions`, { cacheTtlMs: 2500 }).catch(() => []),
          fetchJSON('/contract/list', { cacheTtlMs: 10000, timeoutMs: 10000 }).catch(() => []),
          fetchDexRegistryTokens().catch(() => []),
        ]);

        if (!balanceResult) {
          throw new Error('Address not found');
        }

        const mergedTxs = mergeArrayResults(txsData, "tx_hash")
          .filter((tx) => txMatchesAddress(tx, address))
          .sort((a, b) => (normalizeTx(b).timestamp || 0) - (normalizeTx(a).timestamp || 0));

        const contracts = asArrayPayload(contractData)
          .map(normalizeContract)
          .filter((contract) => contract.address);
        const currentContract = contracts.find((contract) => sameAddress(contract.address, address)) || null;

        const tokenMap = new Map();
        registryTokens.map(normalizeRegistryToken).forEach((token) => {
          if (!hasAddress(token.address)) return;
          tokenMap.set(lower(token.address), token);
        });
        contracts.filter(isTokenContract).forEach((contract) => {
          const key = lower(contract.address);
          tokenMap.set(key, { ...(tokenMap.get(key) || {}), ...contract });
        });

        const tokenCandidates = Array.from(tokenMap.values()).slice(0, 80);
        const tokenResults = await Promise.all(
          tokenCandidates.map(async (token) => {
            try {
              const storageData = await fetchJSON(`/contract/storage?address=${encodeURIComponent(token.address)}`, {
                cacheTtlMs: 5000,
                timeoutMs: 8000,
              });
              const storage = extractStorage(storageData);
              const balance = balanceFromStorage(storage, address);
              return {
                ...token,
                decimals: Number(token.decimals || storage.decimals || storage.Decimals || 8) || 8,
                balance,
                balanceFormatted: formatTokenAmount(balance, token.decimals || storage.decimals || storage.Decimals || 8),
              };
            } catch {
              return { ...token, balance: "0", balanceFormatted: "0" };
            }
          })
        );

        const nftCandidates = contracts.filter(isNftContract).slice(0, 40);
        const nftResults = await Promise.all(
          nftCandidates.map(async (contract) => {
            try {
              const storageData = await fetchJSON(`/contract/storage?address=${encodeURIComponent(contract.address)}`, {
                cacheTtlMs: 10000,
                timeoutMs: 8000,
              });
              const tokenIds = nftIdsFromStorage(extractStorage(storageData), address);
              return { ...contract, tokenIds, count: tokenIds.length };
            } catch {
              return { ...contract, tokenIds: [], count: 0 };
            }
          })
        );

        if (cancelled) return;

        setAddressData({
          address,
          balance: balanceResult.balance ?? balanceResult.confirmed_balance ?? "0",
          confirmedBalance: balanceResult.confirmed_balance ?? balanceResult.balance ?? "0",
          pendingBalance: balanceResult.pending_balance_change ?? balanceResult.pending ?? "0",
          isValidator: balanceResult.isValidator || false
        });
        setTransactions(mergedTxs);
        setContractInfo(currentContract);
        setTokenHoldings(tokenResults.filter((token) => hasPositiveBalance(token.balance || "0")));
        setNftHoldings(nftResults.filter((nft) => nft.count > 0));
        setAnalytics({
          tx_count: mergedTxs.length,
          token_tx_count: mergedTxs.filter(isTokenTx).length,
          internal_count: mergedTxs.filter(isInternalTx).length,
          pending_count: mergedTxs.filter((tx) => lower(tx.status || tx.Status) === "pending").length,
          daily: [],
        });
      } catch (err) {
        if (!cancelled) setError(err.message || "Failed to load address details");
      } finally {
        if (!cancelled) setLoading(false);
      }
    };

    fetchAddressData();
    const id = setInterval(fetchAddressData, REFRESH_MS);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, [address]);

  const tokenTransactions = useMemo(
    () => transactions.filter(isTokenTx),
    [transactions]
  );
  const internalTransactions = useMemo(
    () => transactions.filter(isInternalTx),
    [transactions]
  );
  const nftCount = useMemo(
    () => nftHoldings.reduce((sum, nft) => sum + nft.count, 0),
    [nftHoldings]
  );

  if (loading) return <div className="loading">Loading address details...</div>;
  if (error) return <div className="error">Error: {error}</div>;

  const tabs = [
    ["transactions", `Transactions (${transactions.length})`],
    ["tokens", `Token Transfers (${tokenTransactions.length})`],
    ["internal", `Internal Txns (${internalTransactions.length})`],
    ["holdings", `Tokens (${tokenHoldings.length})`],
    ["nfts", `NFTs (${nftCount})`],
    ["analytics", "Analytics"],
    ["contract", contractInfo ? "Contract" : "Contract Info"],
  ];

  return (
    <div className="address-page address-portfolio">
      <div className="address-hero">
        <div>
          <p className="eyebrow">Address</p>
          <h2>Address Details</h2>
          <div className="address-hash">{address}</div>
        </div>
        <div className="address-badges">
          {addressData.isValidator && <span className="status-pill success">Validator</span>}
          {contractInfo && <span className="status-pill info">Contract</span>}
        </div>
      </div>

      <div className="address-summary-grid">
        <div className="address-metric primary">
          <span>LQD Balance</span>
          <strong>{formatLQD(addressData.balance)} LQD</strong>
          <small>Confirmed: {formatLQD(addressData.confirmedBalance)} LQD</small>
        </div>
        <div className="address-metric">
          <span>Token Holdings</span>
          <strong>{tokenHoldings.length}</strong>
          <small>Non-zero token balances detected</small>
        </div>
        <div className="address-metric">
          <span>NFT Holdings</span>
          <strong>{nftCount}</strong>
          <small>{nftHoldings.length} collection{nftHoldings.length === 1 ? "" : "s"}</small>
        </div>
        <div className="address-metric">
          <span>Transactions</span>
          <strong>{transactions.length}</strong>
          <small>Native, token, reward, and contract activity</small>
        </div>
      </div>

      <div className="address-tabs">
        {tabs.map(([key, label]) => (
          <button
            key={key}
            className={`address-tab ${activeTab === key ? "active" : ""}`}
            onClick={() => setActiveTab(key)}
            type="button"
          >
            {label}
          </button>
        ))}
      </div>

      {activeTab === "transactions" && (
        <section className="address-panel">
          <h3>Transactions</h3>
          <TransactionList transactions={transactions} />
        </section>
      )}

      {activeTab === "tokens" && (
        <section className="address-panel">
          <h3>Token Transfers</h3>
          <TransactionList transactions={tokenTransactions} />
        </section>
      )}

      {activeTab === "internal" && (
        <section className="address-panel">
          <h3>Internal Transactions & Rewards</h3>
          <TransactionList transactions={internalTransactions} />
        </section>
      )}

      {activeTab === "holdings" && (
        <section className="address-panel">
          <h3>Token Holdings</h3>
          {tokenHoldings.length ? (
            <div className="address-token-table">
              {tokenHoldings.map((token) => (
                <div className="address-token-row" key={token.address}>
                  <div className="asset-cell">
                    {token.logoUrl ? (
                      <img src={token.logoUrl} alt={token.symbol || token.name} />
                    ) : (
                      <span className="asset-fallback">{(token.symbol || token.name || "?").slice(0, 1)}</span>
                    )}
                    <div>
                      <strong>{token.symbol || shortHash(token.address, 6, 4)}</strong>
                      <small>{token.name || "Token"} · {shortHash(token.address)}</small>
                    </div>
                  </div>
                  <div>
                    <span className="muted-label">Balance</span>
                    <strong>{token.balanceFormatted}</strong>
                  </div>
                  <div>
                    <span className="muted-label">Decimals</span>
                    <strong>{token.decimals || 8}</strong>
                  </div>
                  <div>
                    {token.verified || token.official ? (
                      <span className="status-pill success">Official</span>
                    ) : (
                      <span className="status-pill muted">Custom</span>
                    )}
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <div className="empty-state">No token balances found for this address.</div>
          )}
        </section>
      )}

      {activeTab === "nfts" && (
        <section className="address-panel">
          <h3>NFT Holdings</h3>
          {nftHoldings.length ? (
            <div className="address-nft-grid">
              {nftHoldings.map((nft) => (
                <div className="address-nft-card" key={nft.address}>
                  <span className="nft-icon">NFT</span>
                  <strong>{nft.name || nft.symbol || shortHash(nft.address)}</strong>
                  <small>{shortHash(nft.address)}</small>
                  <p>{nft.count} owned</p>
                  <div className="nft-token-ids">
                    {nft.tokenIds.slice(0, 8).map((id) => <span key={id}>#{id}</span>)}
                    {nft.tokenIds.length > 8 && <span>+{nft.tokenIds.length - 8}</span>}
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <div className="empty-state">No NFT holdings found for this address.</div>
          )}
        </section>
      )}

      {activeTab === "analytics" && (
        <section className="address-panel">
          <h3>Address Analytics</h3>
          <div className="address-summary-grid compact">
            <div className="address-metric"><span>Total Txns</span><strong>{analytics?.tx_count ?? transactions.length}</strong></div>
            <div className="address-metric"><span>Pending</span><strong>{analytics?.pending_count ?? 0}</strong></div>
            <div className="address-metric"><span>Internal</span><strong>{analytics?.internal_count ?? internalTransactions.length}</strong></div>
            <div className="address-metric"><span>Token Txns</span><strong>{analytics?.token_tx_count ?? tokenTransactions.length}</strong></div>
          </div>
          <div className="address-chart">
            {(analytics?.daily || []).length ? analytics.daily.map((row) => {
              const max = Math.max(...analytics.daily.map((d) => Number(d.count || 0)), 1);
              const height = Math.max(8, Math.round((Number(row.count || 0) / max) * 120));
              return (
                <div className="address-chart-bar" key={row.date}>
                  <span style={{ height }} title={`${row.date}: ${row.count} tx`} />
                  <small>{String(row.date || "").slice(5)}</small>
                </div>
              );
            }) : <div className="empty-state">Analytics will appear after indexed confirmed activity.</div>}
          </div>
        </section>
      )}

      {activeTab === "contract" && (
        <section className="address-panel">
          <h3>Contract Information</h3>
          {contractInfo ? (
            <div className="contract-info-grid">
              <div><span>Type</span><strong>{contractInfo.type}</strong></div>
              <div><span>Name</span><strong>{contractInfo.name || "-"}</strong></div>
              <div><span>Symbol</span><strong>{contractInfo.symbol || "-"}</strong></div>
              <div><span>Owner</span><strong>{contractInfo.owner ? shortHash(contractInfo.owner) : "-"}</strong></div>
              <div><span>Total Supply</span><strong>{contractInfo.totalSupply || "-"}</strong></div>
              <div><span>Decimals</span><strong>{contractInfo.decimals || "-"}</strong></div>
              <Link className="outline-link" to={`/contracts`}>Open Contract Tracker</Link>
            </div>
          ) : (
            <div className="empty-state">This address is not detected as a deployed contract.</div>
          )}
        </section>
      )}
    </div>
  );
};

export default AddressPage;
