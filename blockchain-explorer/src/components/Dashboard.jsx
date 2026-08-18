import React, { useCallback, useEffect, useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import BlockList from "./BlockList";
import ValidatorList from "./ValidatorList";
import { formatLQD } from "../utils/lqdUnits";
import {
  fetchChainJSON,
  fetchJSON,
  fetchRecentBlocks,
  firstNodeResult,
  mergeArrayResults,
  transactionsFromBlocks,
} from "../utils/api";

const SECURITY_CHECKS = [
  ["signed_bft_finality_observed", "Signed BFT finality"],
  ["standard_ecvrf", "RFC 9381 ECVRF"],
  ["durable_slashing_protection", "Durable slashing protection"],
  ["validator_signer_healthy", "Remote validator signer"],
  ["persistent_explorer_index", "Persistent explorer index"],
  ["deterministic_state_commitment", "Deterministic state roots"],
];

const compactNumber = (value, maximumFractionDigits = 2) => {
  const number = Number(value);
  if (!Number.isFinite(number)) return "—";
  return new Intl.NumberFormat("en-US", { maximumFractionDigits }).format(number);
};

const shortHash = (value, start = 10, end = 6) => {
  if (!value) return "—";
  if (value.length <= start + end + 2) return value;
  return `${value.slice(0, start)}…${value.slice(-end)}`;
};

const timeAgo = (timestamp) => {
  const seconds = Number(timestamp);
  if (!Number.isFinite(seconds)) return "—";
  const elapsed = Math.max(0, Math.floor(Date.now() / 1000) - seconds);
  if (elapsed < 60) return `${elapsed}s ago`;
  if (elapsed < 3600) return `${Math.floor(elapsed / 60)}m ago`;
  if (elapsed < 86400) return `${Math.floor(elapsed / 3600)}h ago`;
  return `${Math.floor(elapsed / 86400)}d ago`;
};

const detectTransactionType = (transaction, validatorAddresses) => {
  const rawType = String(
    transaction?.tx_type || transaction?.type || transaction?.category || transaction?.kind || ""
  ).toLowerCase();
  const fn = String(
    transaction?.function || transaction?.method || transaction?.function_name || ""
  ).toLowerCase();
  const recipient = String(transaction?.to || transaction?.To || "").toLowerCase();

  if (rawType.includes("reward") || fn === "blockreward") {
    return validatorAddresses.has(recipient) ? "Validator reward" : "Protocol reward";
  }
  if (rawType.includes("lp")) return "Liquidity reward";
  if (rawType.includes("contract_create") || fn === "deploycontract") return "Contract deploy";
  if (fn === "transfer" && transaction?.is_contract) return "Token transfer";
  if (transaction?.is_contract || fn) return "Contract call";
  return "Transfer";
};

const StatCard = ({ label, value, detail, accent = "blue", eyebrow }) => (
  <article className={`network-stat premium-stat accent-${accent}`}>
    <div className="premium-stat-topline">
      <span>{label}</span>
      {eyebrow && <small>{eyebrow}</small>}
    </div>
    <strong>{value}</strong>
    <p>{detail}</p>
  </article>
);

const Dashboard = () => {
  const navigate = useNavigate();
  const [data, setData] = useState({
    health: null,
    readiness: null,
    network: null,
    blockTime: null,
    baseFee: null,
    blocks: [],
    validators: [],
  });
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState("");
  const [lastUpdated, setLastUpdated] = useState(null);
  const [globalSearch, setGlobalSearch] = useState("");
  const [transactionFilter, setTransactionFilter] = useState("all");
  const [copiedHash, setCopiedHash] = useState("");

  const fetchData = useCallback(async ({ background = false } = {}) => {
    if (background) setRefreshing(true);
    setError("");

    const results = await Promise.allSettled([
      fetchChainJSON("/health", { cacheTtlMs: 2500, timeoutMs: 6500 }),
      fetchChainJSON("/readiness/mainnet", { cacheTtlMs: 5000, timeoutMs: 8000 }),
      fetchChainJSON("/basefee", { cacheTtlMs: 2500, timeoutMs: 6500 }),
      fetchJSON("/network", { cacheTtlMs: 3000, timeoutMs: 7500 }),
      fetchJSON("/blocktime/latest", { cacheTtlMs: 2500, timeoutMs: 6500 }),
      fetchRecentBlocks(14, { timeoutMs: 8000 }),
      fetchJSON("/validators", { cacheTtlMs: 5000, timeoutMs: 8000 }),
    ]);

    const fulfilled = results.filter((result) => result.status === "fulfilled").length;
    if (fulfilled === 0) {
      setError("The public network APIs are temporarily unreachable. Live data will retry automatically.");
    }

    setData((current) => ({
      health: results[0].status === "fulfilled" ? results[0].value : current.health,
      readiness: results[1].status === "fulfilled" ? results[1].value : current.readiness,
      baseFee: results[2].status === "fulfilled" ? results[2].value : current.baseFee,
      network: results[3].status === "fulfilled" ? firstNodeResult(results[3].value) : current.network,
      blockTime: results[4].status === "fulfilled" ? firstNodeResult(results[4].value) : current.blockTime,
      blocks: results[5].status === "fulfilled" && results[5].value.length ? results[5].value : current.blocks,
      validators:
        results[6].status === "fulfilled"
          ? mergeArrayResults(results[6].value, "address")
          : current.validators,
    }));
    if (fulfilled > 0) setLastUpdated(new Date());
    setLoading(false);
    setRefreshing(false);
  }, []);

  useEffect(() => {
    fetchData();
    const timer = window.setInterval(() => fetchData({ background: true }), 7000);
    return () => window.clearInterval(timer);
  }, [fetchData]);

  const validatorAddresses = useMemo(
    () => new Set(data.validators.map((validator) => String(validator.address || "").toLowerCase())),
    [data.validators]
  );

  const recentTransactions = useMemo(
    () =>
      transactionsFromBlocks(data.blocks, 16).map((transaction) => ({
        ...transaction,
        displayType: detectTransactionType(transaction, validatorAddresses),
      })),
    [data.blocks, validatorAddresses]
  );

  const visibleTransactions = useMemo(() => {
    if (transactionFilter === "all") return recentTransactions.slice(0, 6);
    return recentTransactions
      .filter((transaction) =>
        transactionFilter === "rewards"
          ? transaction.displayType.toLowerCase().includes("reward")
          : !transaction.displayType.toLowerCase().includes("reward")
      )
      .slice(0, 6);
  }, [recentTransactions, transactionFilter]);

  const blockIntervals = useMemo(() => {
    return data.blocks.slice(0, 10).map((block, index, blocks) => {
      const previous = blocks[index + 1];
      const seconds = previous
        ? Math.max(0, Number(block.timestamp || 0) - Number(previous.timestamp || 0))
        : Number(data.network?.average_block_time || 0);
      return { number: block.block_number, seconds };
    });
  }, [data.blocks, data.network]);

  const averageBlockTime = useMemo(() => {
    const valid = blockIntervals.map((item) => item.seconds).filter((seconds) => seconds > 0);
    if (valid.length) return valid.reduce((sum, seconds) => sum + seconds, 0) / valid.length;
    return Number(data.network?.average_block_time || 0);
  }, [blockIntervals, data.network]);

  const securityChecks = useMemo(() => {
    const checks = new Map((data.readiness?.checks || []).map((check) => [check.name, check]));
    return SECURITY_CHECKS.map(([name, label]) => ({ name, label, ...(checks.get(name) || {}) }));
  }, [data.readiness]);

  const pendingMainnetGates = useMemo(
    () => (data.readiness?.checks || []).filter((check) => check.critical && !check.ok),
    [data.readiness]
  );

  const readinessScore = Number(data.readiness?.completion_score_percent);
  const latestBlock = data.blocks[0] || null;
  const latestBlockAge = latestBlock?.timestamp
    ? Math.max(0, Math.floor(Date.now() / 1000) - Number(latestBlock.timestamp))
    : Number.POSITIVE_INFINITY;
  const finalityLive = data.health?.status === "ok" && latestBlockAge <= 90;
  const chainHeight = data.health?.height ?? data.readiness?.height ?? latestBlock?.block_number;
  const protocolVersion = latestBlock?.protocol_version ?? "—";
  const bftObserved = securityChecks.find((check) => check.name === "signed_bft_finality_observed")?.ok;
  const vrfObserved = securityChecks.find((check) => check.name === "standard_ecvrf")?.ok;

  const handleSearch = async (event) => {
    event.preventDefault();
    const query = globalSearch.trim();
    if (!query) return;

    try {
      const result = await fetchChainJSON(`/v2/index/search?q=${encodeURIComponent(query)}`, {
        timeoutMs: 8000,
      });
      if (result?.type === "address") return navigate(`/address/${result.query || query}`);
      if (result?.type === "transaction") return navigate(`/tx/${result.transaction.hash}`);
      if (result?.type === "block") {
        return navigate(`/blocks/${result.block.hash || result.block.number}`);
      }
    } catch {
      // Fall through to deterministic route matching for older nodes.
    }

    if (/^\d+$/.test(query)) return navigate(`/blocks/${query}`);
    if (/^0x[a-fA-F0-9]{40}$/.test(query)) return navigate(`/address/${query}`);
    if (/^(0x)?[a-fA-F0-9]{64}$/.test(query)) return navigate(`/tx/${query}`);
    setError("No exact block, transaction, or address match was found for that query.");
  };

  const copyHash = async (hash) => {
    try {
      await navigator.clipboard.writeText(hash);
      setCopiedHash(hash);
      window.setTimeout(() => setCopiedHash(""), 1200);
    } catch {
      setError("Clipboard access is not available in this browser.");
    }
  };

  if (loading && !data.health && data.blocks.length === 0) {
    return (
      <div className="premium-loading" aria-live="polite">
        <span />
        <p>Connecting to the PoDL public network…</p>
      </div>
    );
  }

  return (
    <main className="dashboard premium-dashboard">
      <section className="intelligence-hero">
        <div className="hero-copy">
          <div className="hero-eyebrow">
            <span className={`live-indicator ${finalityLive ? "live" : "degraded"}`} />
            {finalityLive ? "Live public network intelligence" : "Public network finality alert"}
          </div>
          <h1>Every PoDL block.<br />One transparent view.</h1>
          <p>
            Inspect finalized state, validator activity, dynamic-liquidity infrastructure,
            transactions and protocol readiness directly from public backend APIs.
          </p>

          <form className="premium-search" onSubmit={handleSearch} role="search">
            <span className="search-symbol" aria-hidden="true" />
            <label className="sr-only" htmlFor="global-chain-search">Search the PoDL chain</label>
            <input
              id="global-chain-search"
              value={globalSearch}
              onChange={(event) => setGlobalSearch(event.target.value)}
              placeholder="Search address, transaction hash, or block height"
              autoComplete="off"
            />
            <button type="submit">Search network</button>
          </form>

          <div className="hero-evidence-row" aria-label="Protocol evidence">
            <span className={bftObserved ? "verified" : "pending"}>Signed BFT {bftObserved ? "observed" : "checking"}</span>
            <span className={vrfObserved ? "verified" : "pending"}>ECVRF {vrfObserved ? "verified" : "checking"}</span>
            <span>Protocol v{protocolVersion}</span>
          </div>
        </div>

        <aside className="chain-pulse-card" aria-label="Live chain pulse">
          <div className="pulse-card-header">
            <div>
              <span>Chain pulse</span>
              <strong>Public testnet</strong>
            </div>
            <button
              className={refreshing ? "refresh-button refreshing" : "refresh-button"}
              type="button"
              onClick={() => fetchData({ background: true })}
              disabled={refreshing}
              aria-label="Refresh live network data"
              title="Refresh live data"
            >
              ↻
            </button>
          </div>
          <div className="pulse-height">
            <span>Finalized height</span>
            <strong>#{compactNumber(chainHeight, 0)}</strong>
            <small>{latestBlock ? timeAgo(latestBlock.timestamp) : "Waiting for latest block"}</small>
          </div>
          <div className="cadence-chart" aria-label="Recent block interval chart">
            {blockIntervals.map((item, index) => {
              const height = Math.min(100, Math.max(26, (item.seconds / Math.max(averageBlockTime, 1)) * 48));
              return (
                <span
                  key={`${item.number}-${index}`}
                  style={{ height: `${height}%` }}
                  title={`Block ${item.number}: ${item.seconds.toFixed(1)} seconds`}
                />
              );
            })}
          </div>
          <div className="pulse-footer">
            <span>Average cadence <strong>{averageBlockTime ? `${averageBlockTime.toFixed(2)}s` : "—"}</strong></span>
            <span>Latest hash <strong>{shortHash(data.health?.latest_block_hash, 8, 5)}</strong></span>
          </div>
        </aside>
      </section>

      {!finalityLive && latestBlock && (
        <div className="premium-alert critical" role="alert">
          Block production is delayed: the latest finalized block is {timeAgo(latestBlock.timestamp)}. Node HTTP availability is not treated as consensus health.
        </div>
      )}
      {error && <div className="premium-alert" role="status">{error}</div>}

      <section className="network-stat-grid" aria-label="Live network metrics">
        <StatCard label="Block height" value={`#${compactNumber(chainHeight, 0)}`} detail="Persistent index synchronized" accent="cyan" eyebrow="Live" />
        <StatCard label="Block cadence" value={averageBlockTime ? `${averageBlockTime.toFixed(2)}s` : "—"} detail={`${data.blockTime?.mining_time_ms ? compactNumber(data.blockTime.mining_time_ms, 0) : "—"} ms latest mining time`} accent="blue" />
        <StatCard label="Base fee" value={compactNumber(data.baseFee?.base_fee ?? data.readiness?.base_fee, 0)} detail="Network-native gas unit" accent="violet" />
        <StatCard label="Active validators" value={compactNumber(data.validators.length, 0)} detail={`${data.health?.peers ?? 0} connected public peers`} accent="green" />
        <StatCard label="Mempool" value={compactNumber(data.readiness?.mempool ?? data.network?.transaction_pool, 0)} detail="Pending public transactions" accent="amber" />
        <StatCard
          label="Readiness evidence"
          value={Number.isFinite(readinessScore) ? `${readinessScore}%` : "—"}
          detail={data.readiness?.testnet_recommended ? "Public testnet recommended" : "Readiness under evaluation"}
          accent="indigo"
          eyebrow="Evidence"
        />
      </section>

      <section className="dashboard-intelligence-grid">
        <article className="premium-panel activity-panel">
          <div className="panel-heading">
            <div>
              <span className="panel-kicker">Finalized ledger</span>
              <h2>Latest blocks</h2>
            </div>
            <Link to="/blocks">View all blocks <span aria-hidden="true">→</span></Link>
          </div>
          <BlockList blocks={data.blocks.slice(0, 7)} showTxHash={false} compact />
        </article>

        <article className="premium-panel readiness-panel">
          <div className="panel-heading">
            <div>
              <span className="panel-kicker">Verifiable infrastructure</span>
              <h2>Security posture</h2>
            </div>
            <Link to="/investor">Full evidence <span aria-hidden="true">→</span></Link>
          </div>

          <div className="readiness-score-row">
            <div
              className="readiness-ring"
              style={{ "--readiness": Number.isFinite(readinessScore) ? readinessScore : 0 }}
              aria-label={`${Number.isFinite(readinessScore) ? readinessScore : 0}% readiness evidence complete`}
            >
              <strong>{Number.isFinite(readinessScore) ? `${readinessScore}%` : "—"}</strong>
              <span>evidence</span>
            </div>
            <div>
              <strong>Testnet operational</strong>
              <p>{pendingMainnetGates.length} critical mainnet gates remain openly tracked.</p>
            </div>
          </div>

          <div className="security-check-list">
            {securityChecks.map((check) => (
              <div key={check.name}>
                <span className={check.ok ? "check-ok" : "check-wait"} aria-hidden="true">
                  {check.ok ? "✓" : "·"}
                </span>
                <span>{check.label}</span>
                <small>{check.ok ? "Verified" : "Pending"}</small>
              </div>
            ))}
          </div>

          {pendingMainnetGates.length > 0 && (
            <div className="mainnet-gates">
              <span>Mainnet gates</span>
              <div>
                {pendingMainnetGates.slice(0, 3).map((check) => (
                  <small key={check.name}>{check.name.replaceAll("_", " ")}</small>
                ))}
              </div>
            </div>
          )}
        </article>
      </section>

      <section className="dashboard-intelligence-grid lower-grid">
        <article className="premium-panel transaction-stream">
          <div className="panel-heading transaction-heading">
            <div>
              <span className="panel-kicker">On-chain activity</span>
              <h2>Latest transactions</h2>
            </div>
            <div className="segment-control" aria-label="Filter recent transactions">
              {["all", "rewards", "user"].map((filter) => (
                <button
                  key={filter}
                  type="button"
                  className={transactionFilter === filter ? "active" : ""}
                  onClick={() => setTransactionFilter(filter)}
                >
                  {filter === "user" ? "User activity" : filter[0].toUpperCase() + filter.slice(1)}
                </button>
              ))}
            </div>
          </div>

          <div className="premium-transaction-list">
            {visibleTransactions.length === 0 ? (
              <div className="empty-state">No matching finalized transactions in the recent block window.</div>
            ) : (
              visibleTransactions.map((transaction, index) => {
                const hash = transaction.tx_hash || transaction.txHash || `transaction-${index}`;
                const status = String(transaction.status || "pending").toLowerCase();
                const confirmed = status === "succsess" || status === "success" || status === "confirmed";
                return (
                  <div className="premium-transaction-row" key={hash}>
                    <button className="transaction-type-mark" type="button" onClick={() => navigate(`/tx/${hash}`)} aria-label={`Open ${transaction.displayType}`}>
                      {transaction.displayType.includes("reward") ? "R" : "T"}
                    </button>
                    <div className="transaction-primary">
                      <button type="button" onClick={() => navigate(`/tx/${hash}`)}>{shortHash(hash, 12, 7)}</button>
                      <span>{transaction.displayType} · {timeAgo(transaction.timestamp)}</span>
                    </div>
                    <div className="transaction-route">
                      <span>{shortHash(transaction.from, 7, 4)}</span>
                      <i aria-hidden="true">→</i>
                      <span>{shortHash(transaction.to, 7, 4)}</span>
                    </div>
                    <div className="transaction-value">
                      <strong>{formatLQD(transaction.value || 0)} LQD</strong>
                      <span className={confirmed ? "confirmed" : "pending"}>{confirmed ? "Finalized" : "Pending"}</span>
                    </div>
                    <button className="copy-row-button" type="button" onClick={() => copyHash(hash)}>
                      {copiedHash === hash ? "Copied" : "Copy"}
                    </button>
                  </div>
                );
              })
            )}
          </div>
          <Link className="panel-footer-link" to="/transactions">Explore transaction history <span aria-hidden="true">→</span></Link>
        </article>

        <article className="premium-panel validator-panel">
          <div className="panel-heading">
            <div>
              <span className="panel-kicker">Consensus participants</span>
              <h2>Validator set</h2>
            </div>
            <Link to="/validators">All validators <span aria-hidden="true">→</span></Link>
          </div>
          <ValidatorList validators={data.validators.slice(0, 4)} premium />
        </article>
      </section>

      <div className="dashboard-refresh-note">
        <span className={`live-indicator ${finalityLive ? "live" : "degraded"}`} />
        Public data refreshes every 7 seconds
        {lastUpdated && <time dateTime={lastUpdated.toISOString()}> · updated {lastUpdated.toLocaleTimeString()}</time>}
      </div>
    </main>
  );
};

export default Dashboard;
