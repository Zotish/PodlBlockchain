import React, { useCallback, useEffect, useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import {
  fetchChainJSON,
  fetchJSON,
  fetchRecentBlocks,
  fetchRecentTransactions,
  firstNodeResult,
  mergeArrayResults,
} from "../utils/api";
import { formatLQD } from "../utils/lqdUnits";

const number = (value, fraction = 0) => {
  const parsed = Number(value);
  return Number.isFinite(parsed)
    ? parsed.toLocaleString("en-US", { maximumFractionDigits: fraction })
    : "—";
};

const short = (value, head = 9, tail = 6) => {
  const text = String(value || "");
  if (!text) return "—";
  return text.length > head + tail + 1 ? `${text.slice(0, head)}…${text.slice(-tail)}` : text;
};

const timeAgo = (timestamp) => {
  const parsed = Number(timestamp);
  if (!Number.isFinite(parsed)) return "—";
  const elapsed = Math.max(0, Math.floor(Date.now() / 1000) - parsed);
  if (elapsed < 60) return `${elapsed}s ago`;
  if (elapsed < 3600) return `${Math.floor(elapsed / 60)}m ago`;
  if (elapsed < 86400) return `${Math.floor(elapsed / 3600)}h ago`;
  return `${Math.floor(elapsed / 86400)}d ago`;
};

const txHash = (transaction) =>
  transaction?.tx_hash || transaction?.txHash || transaction?.TxHash || transaction?.hash || "";

const txType = (transaction) => {
  const raw = String(transaction?.tx_type || transaction?.type || transaction?.kind || "").toLowerCase();
  const method = String(transaction?.function || transaction?.method || "").toLowerCase();
  if (raw.includes("reward") || method === "blockreward") return "Reward";
  if (raw.includes("contract") || method) return method || "Contract";
  return "Transfer";
};

const Dashboard = () => {
  const navigate = useNavigate();
  const [query, setQuery] = useState("");
  const [state, setState] = useState({
    health: null,
    index: null,
    baseFee: null,
    network: null,
    blockTime: null,
    blocks: [],
    transactions: [],
    validators: [],
  });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [updatedAt, setUpdatedAt] = useState(null);

  const load = useCallback(async () => {
    const results = await Promise.allSettled([
      fetchChainJSON("/health", { cacheTtlMs: 2500, timeoutMs: 7000 }),
      fetchChainJSON("/v2/index/status", { cacheTtlMs: 2500, timeoutMs: 7000 }),
      fetchChainJSON("/basefee", { cacheTtlMs: 2500, timeoutMs: 7000 }),
      fetchChainJSON("/network", { cacheTtlMs: 3500, timeoutMs: 7000 }),
      fetchChainJSON("/blocktime/latest", { cacheTtlMs: 2500, timeoutMs: 7000 }),
      fetchRecentBlocks(8, { timeoutMs: 8500 }),
      fetchRecentTransactions(8, { timeoutMs: 8500 }),
      fetchJSON("/validators", { cacheTtlMs: 4000, timeoutMs: 8000 }),
    ]);

    const fulfilled = results.filter((result) => result.status === "fulfilled").length;
    if (!fulfilled) setError("Live network data is temporarily unavailable.");
    else setError("");

    setState((current) => ({
      health: results[0].status === "fulfilled" ? results[0].value : current.health,
      index: results[1].status === "fulfilled" ? results[1].value : current.index,
      baseFee: results[2].status === "fulfilled" ? results[2].value : current.baseFee,
      network: results[3].status === "fulfilled" ? firstNodeResult(results[3].value) : current.network,
      blockTime: results[4].status === "fulfilled" ? firstNodeResult(results[4].value) : current.blockTime,
      blocks: results[5].status === "fulfilled" ? results[5].value : current.blocks,
      transactions: results[6].status === "fulfilled" ? results[6].value : current.transactions,
      validators: results[7].status === "fulfilled"
        ? mergeArrayResults(results[7].value, "address")
        : current.validators,
    }));
    if (fulfilled) setUpdatedAt(new Date());
    setLoading(false);
  }, []);

  useEffect(() => {
    load();
    const timer = window.setInterval(load, 8000);
    return () => window.clearInterval(timer);
  }, [load]);

  const latestBlock = state.blocks[0] || null;
  const latestTimestamp = Number(latestBlock?.timestamp ?? latestBlock?.TimeStamp);
  const latestAge = Number.isFinite(latestTimestamp)
    ? Math.max(0, Math.floor(Date.now() / 1000) - latestTimestamp)
    : Number.POSITIVE_INFINITY;
  const operational = state.health?.status === "ok" && latestAge <= 90;
  const height = state.health?.height ?? latestBlock?.block_number;
  const indexedTransactions = state.index?.indexed_transactions ?? state.index?.transactions;
  const averageBlockTime = Number(state.network?.average_block_time || state.blockTime?.interval_seconds || 0);
  const activeValidators = useMemo(
    () => state.validators.filter((validator) => validator.voting_eligible !== false).length,
    [state.validators]
  );

  const metrics = [
    { label: "Finalized blocks", value: number(height), note: operational ? "Producing normally" : "Finality delayed" },
    { label: "Average block time", value: averageBlockTime ? `${number(averageBlockTime, 2)}s` : "—", note: "Recent network average" },
    { label: "Indexed transactions", value: number(indexedTransactions), note: state.index?.lag_blocks === 0 ? "Index fully synced" : `Index lag ${number(state.index?.lag_blocks)}` },
    { label: "Validators", value: number(state.validators.length), note: `${number(activeValidators)} voting eligible` },
  ];

  const submitSearch = async (event) => {
    event.preventDefault();
    const value = query.trim();
    if (!value) return;
    try {
      const result = await fetchChainJSON(`/v2/index/search?q=${encodeURIComponent(value)}`, { timeoutMs: 8000 });
      if (result?.type === "address") return navigate(`/address/${result.query || value}`);
      if (result?.type === "transaction") return navigate(`/tx/${result.transaction?.hash || value}`);
      if (result?.type === "block") return navigate(`/blocks/${result.block?.hash || result.block?.number || value}`);
    } catch {
      // Deterministic routing below supports nodes without the index search endpoint.
    }
    if (/^\d+$/.test(value)) navigate(`/blocks/${value}`);
    else if (/^0x[a-fA-F0-9]{40}$/.test(value)) navigate(`/address/${value}`);
    else navigate(`/tx/${value}`);
  };

  if (loading && !state.health && !state.blocks.length) {
    return <main className="clean-dashboard"><div className="clean-loading"><span /> Loading live network data…</div></main>;
  }

  return (
    <main className="clean-dashboard">
      <section className="clean-overview-head">
        <div>
          <div className={`clean-status-pill ${operational ? "live" : "delayed"}`}>
            <i aria-hidden="true" /> {operational ? "Network operational" : "Finality delayed"}
          </div>
          <h1>Network overview</h1>
          <p>Blocks, transactions and validators on the PoDL public testnet.</p>
        </div>
        <span className="clean-updated">Updated {updatedAt ? updatedAt.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" }) : "—"}</span>
      </section>

      {!operational && !loading && (
        <div className="clean-alert" role="alert">
          <strong>Block production is delayed.</strong>
          <span>The API is reachable, but the latest finalized block is {Number.isFinite(latestAge) ? timeAgo(latestTimestamp) : "not available"}.</span>
        </div>
      )}
      {error && <div className="clean-alert" role="alert"><strong>Data unavailable.</strong><span>{error}</span></div>}

      <form className="clean-hero-search" onSubmit={submitSearch} role="search">
        <span className="clean-search-icon" aria-hidden="true" />
        <label className="sr-only" htmlFor="overview-search">Search the PoDL blockchain</label>
        <input
          id="overview-search"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder="Search by block number, transaction hash or address"
          autoComplete="off"
        />
        <button type="submit">Search</button>
      </form>

      <section className="clean-metric-grid" aria-label="Network metrics">
        {metrics.map((metric) => (
          <article key={metric.label}>
            <span>{metric.label}</span>
            <strong>{metric.value}</strong>
            <small>{metric.note}</small>
          </article>
        ))}
      </section>

      <section className="clean-ledger-grid">
        <article className="clean-panel">
          <header><div><h2>Latest blocks</h2><p>Recently finalized blocks</p></div><Link to="/blocks">View all</Link></header>
          <div className="clean-block-rows">
            {state.blocks.slice(0, 6).map((block) => {
              const blockNumber = block.block_number ?? block.BlockNumber;
              const transactions = block.transactions ?? block.Transactions ?? [];
              const hash = block.current_hash ?? block.CurrentHash;
              return (
                <button key={blockNumber} type="button" onClick={() => navigate(`/blocks/${blockNumber}`)}>
                  <span className="clean-block-icon" aria-hidden="true" />
                  <span><strong>#{number(blockNumber)}</strong><small>{timeAgo(block.timestamp ?? block.TimeStamp)}</small></span>
                  <code>{short(hash)}</code>
                  <span className="clean-count"><strong>{transactions.length}</strong><small>txns</small></span>
                </button>
              );
            })}
            {!state.blocks.length && <div className="clean-empty">No blocks available.</div>}
          </div>
        </article>

        <article className="clean-panel">
          <header><div><h2>Latest transactions</h2><p>Recent on-chain activity</p></div><Link to="/transactions">View all</Link></header>
          <div className="clean-tx-rows">
            {state.transactions.slice(0, 6).map((transaction, index) => {
              const hash = txHash(transaction);
              const timestamp = transaction.timestamp ?? transaction.Timestamp;
              return (
                <button key={hash || index} type="button" onClick={() => hash && navigate(`/tx/${hash}`)}>
                  <span className="clean-tx-icon" aria-hidden="true">↗</span>
                  <span><strong>{short(hash)}</strong><small>{timeAgo(timestamp)}</small></span>
                  <span className="clean-type">{txType(transaction)}</span>
                  <span className="clean-value">{formatLQD(transaction.value ?? transaction.Value ?? 0)} LQD</span>
                </button>
              );
            })}
            {!state.transactions.length && <div className="clean-empty">No transactions available.</div>}
          </div>
        </article>
      </section>

      <section className="clean-network-details">
        <div><h2>Network details</h2><p>Current public chain configuration and index state.</p></div>
        <dl>
          <div><dt>Network</dt><dd>PoDL public testnet</dd></div>
          <div><dt>Protocol</dt><dd>{latestBlock?.protocol_version ? `PoDL v${latestBlock.protocol_version}` : "PoDL v2"}</dd></div>
          <div><dt>Base fee</dt><dd>{state.baseFee?.base_fee ?? state.baseFee?.baseFee ?? "—"} LQD</dd></div>
          <div><dt>Mempool</dt><dd>{number(state.network?.transaction_pool ?? 0)} pending</dd></div>
          <div><dt>Index</dt><dd>{state.index?.lag_blocks === 0 ? "Synced" : `Lag ${number(state.index?.lag_blocks)}`}</dd></div>
          <div><dt>Consensus</dt><dd>{operational ? "Finalizing" : "Delayed"}</dd></div>
        </dl>
        <div className="clean-quick-links">
          <Link to="/developers/api">API reference</Link>
          <Link to="/stats">Network statistics</Link>
          <Link to="/investor">Protocol evidence</Link>
          <Link to="/developers/broadcast">Broadcast transaction</Link>
        </div>
      </section>
    </main>
  );
};

export default Dashboard;
