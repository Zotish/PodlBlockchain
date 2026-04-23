



// src/components/Dashboard.jsx
import React, { useState, useEffect, useMemo } from "react";
import BlockList from "./BlockList";
import ValidatorList from "./ValidatorList";
import { useNavigate } from "react-router-dom";
import { formatLQD } from "../utils/lqdUnits";
import { fetchJSON, mergeArrayResults, firstNodeResult } from "../utils/api";

const Dashboard = () => {
  const navigate = useNavigate();

  const [networkStats, setNetworkStats] = useState(null);
  const [recentBlocks, setRecentBlocks] = useState([]);
  const [recentTxs, setRecentTxs] = useState([]);
  const [validators, setValidators] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [blockTime, setBlockTime] = useState(null); 


  /* -----------------------------
      LOCAL STORY SEARCH / FILTER
  ----------------------------- */
  const [searchQuery, setSearchQuery] = useState("");
  const [searchField, setSearchField] = useState("all");
  const [activeTab, setActiveTab] = useState("all");
  const [copiedHash, setCopiedHash] = useState("");

  /* -----------------------------
      GLOBAL HYBRID SEARCH BAR
  ----------------------------- */
  const [globalSearch, setGlobalSearch] = useState("");

  // Debounce (local story search)
  const [debouncedQuery, setDebouncedQuery] = useState("");
  useEffect(() => {
    const id = setTimeout(() => setDebouncedQuery(searchQuery.trim()), 300);
    return () => clearTimeout(id);
  }, [searchQuery]);

  /* -----------------------------
      FETCH DASHBOARD DATA
  ----------------------------- */
  const fetchData = async () => {
    try {
      setError(null);

      const stats = await fetchJSON("/network");
      setNetworkStats(firstNodeResult(stats));

      const blocks = await fetchJSON("/fetch_last_n_block");
      const mergedBlocks = mergeArrayResults(blocks, "block_number");
      const sortedBlocks = mergedBlocks.sort(
        (a, b) => (b.block_number ?? 0) - (a.block_number ?? 0)
      );
      setRecentBlocks(sortedBlocks.slice(0, 15));

      try {
        const bt = await fetchJSON(`/blocktime/latest`);
        const btResult = firstNodeResult(bt);
        if (btResult && !btResult.error) {
          setBlockTime(btResult);
        } else {
          setBlockTime(null);
        }
      } catch (e) {
        setBlockTime(null);
      }

      // Validators
      let val = [];
      try {
        const v = await fetchJSON(`/validators`);
        val = mergeArrayResults(v, "address");
      } catch {}
      setValidators(val);

      // Recent TXs
      const txRes = await fetchJSON(`/transactions/recent`);
      const arr = mergeArrayResults(txRes, "tx_hash");

      // Attach type tags
      const withType = arr.map((t) => ({
        ...t,
        __txType: detectTxType(t),
      }));

      setRecentTxs(withType);
    } catch (err) {
      setError("Failed to load dashboard data");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
    const id = setInterval(fetchData, 1000);
    return () => clearInterval(id);
    // fetchData intentionally re-created with latest dashboard state.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  /* -----------------------------
      HELPERS: TYPE DETECTION
  ----------------------------- */
  const validatorSet = useMemo(
    () =>
      new Set(
        validators.map((v) => (v.Address || v.address || "").toLowerCase())
      ),
    [validators]
  );

  const detectTxType = (tx) => {
    if (!tx) return "transfer";

    const raw =
      tx.tx_type ||
      tx.type ||
      tx.category ||
      tx.reward_type ||
      tx.kind ||
      "";
    const fn =
      (tx.function ||
        tx.method ||
        tx.function_name ||
        tx.method_name ||
        "") + "";

    const to = (tx.to || tx.To || "").toLowerCase();

    const l = raw.toLowerCase();
    const f = fn.toLowerCase();

    // Rewards
    if (l.includes("validator") && l.includes("reward")) return "reward_validator";
    if (l.includes("lp")) return "reward_lp";
    if (l.includes("contributor")) return "reward_contributor";
    if (f === "blockreward") {
      if (validatorSet.has(to)) return "reward_validator";
      return "reward";
    }

    // Contract interactions
    if (
      l.includes("contract_create") ||
      f === "deploycontract" ||
      to === "0x0000000000000000000000000000000000000000"
    )
      return "contract_create";

    if (f === "transfer" && tx.is_contract) return "token_transfer";

    if (tx.is_contract || f.length > 0) return "contract_call";

    return "transfer";
  };

  const timeAgo = (ts) => {
    if (!ts) return "N/A";
    const now = Math.floor(Date.now() / 1000);
    let d = now - ts;
    if (d < 0) d = 0;
    if (d < 60) return `${d}s ago`;
    const m = Math.floor(d / 60);
    if (m < 60) return `${m}m ago`;
    const h = Math.floor(m / 60);
    if (h < 24) return `${h}h ago`;
    return `${Math.floor(h / 24)}d ago`;
  };

  /* -----------------------------
      PROFESSIONAL HYBRID SEARCH
  ----------------------------- */
  const isHash = (q) =>
    q.startsWith("0x") && (q.length === 66 || q.length === 64 || q.length > 40);

  const isAddress = (q) =>
    q.startsWith("0x") && q.length === 42;

  const isBlockNumber = (q) => /^\d+$/.test(q);

  const handleGlobalSearchSubmit = (e) => {
    e.preventDefault();
    const q = globalSearch.trim();

    if (!q) return;

    // TX hash
    // Address FIRST (so it doesn't get mistaken for a hash)
if (isAddress(q)) {
  return navigate(`/address/${q}`);
}

// Then tx hash
if (isHash(q)) {
  return navigate(`/tx/${q}`);
}

// Block number (allow `123` or `#123`)
if (isBlockNumber(q)) {
  const blockNum = q.replace(/^#/, "").trim();
  return navigate(`/blocks/${blockNum}`);
}


    // Otherwise fallback to local dashboard story search
    setSearchQuery(q);
  };

  /* -----------------------------
      LOCAL RECENT TX FILTERING
  ----------------------------- */
  const visibleTxs = useMemo(() => {
    let arr = [...recentTxs];

    const tab = activeTab.toLowerCase();
    const q = debouncedQuery.toLowerCase();

    // Tabs
    if (tab !== "all") {
      arr = arr.filter((t) => {
        const s = (t.status || "").toLowerCase();
        if (tab === "confirmed") return s === "succsess";
        if (tab === "pending") return s !== "succsess" && s !== "failed";
        if (tab === "failed") return s === "failed";
        return true;
      });
    }

    // Search filters
    if (q) {
      arr = arr.filter((t) => {
        const h = (t.tx_hash || t.txHash || "").toLowerCase();
        const from = (t.from || "").toLowerCase();
        const to = (t.to || "").toLowerCase();
        const st = (t.status || "").toLowerCase();
        const tp = (t.__txType || "").toLowerCase();

        if (searchField === "hash") return h.includes(q);
        if (searchField === "address") return from.includes(q) || to.includes(q);
        if (searchField === "status") return st.includes(q);
        if (searchField === "type") return tp.includes(q);

        // All fields
        return (
          h.includes(q) ||
          from.includes(q) ||
          to.includes(q) ||
          st.includes(q) ||
          tp.includes(q)
        );
      });
    }

    return arr.slice(0, 4); // show only top
  }, [recentTxs, debouncedQuery, searchField, activeTab]);

  /* -----------------------------
      BADGES
  ----------------------------- */
  const StatusBadge = ({ status }) => {
    const s = (status || "").toLowerCase();
    const cls =
      s === "succsess" ? "badge badge-green"
      : s === "failed" ? "badge badge-red"
      : "badge badge-yellow";
    const text =
      s === "succsess" ? "Confirmed"
      : s === "failed"  ? "Failed"
      : "Pending";
    return <span className={cls}>{text}</span>;
  };

  const TypeBadge = ({ type }) => {
    const t = (type || "").toLowerCase();
    const map = {
      reward_validator: ["badge badge-cyan",   "Validator Reward"],
      reward_lp:        ["badge badge-purple", "LP Reward"],
      reward_contributor:["badge badge-teal",  "Contributor"],
      reward:           ["badge badge-cyan",   "Reward"],
      contract_create:  ["badge badge-purple", "Deploy"],
      contract_call:    ["badge badge-green",  "Contract Call"],
      token_transfer:   ["badge badge-orange", "Token Transfer"],
      transfer:         ["badge badge-gray",   "Transfer"],
    };
    const [cls, label] = map[t] || map.transfer;
    return <span className={cls}>{label}</span>;
  };

  /* -----------------------------
      COPY HELPER
  ----------------------------- */
  const copyHash = async (h) => {
    try {
      await navigator.clipboard.writeText(h);
      setCopiedHash(h);
      setTimeout(() => setCopiedHash(""), 1200);
    } catch {}
  };

  if (loading && !networkStats)
    return <div className="loading">Loading dashboard…</div>;

  /* ===========================================================
                         RENDER START
     =========================================================== */
  return (
    <div className="dashboard">

      {/* ══════════ HERO SEARCH ══════════ */}
      <div className="search-hero">
        <p className="search-hero-title">LQD Blockchain Explorer</p>
        <p className="search-hero-sub">
          Search by transaction hash, address, or block number
        </p>
        <form className="search-form" onSubmit={handleGlobalSearchSubmit}>
          <input
            className="search-input"
            value={globalSearch}
            onChange={(e) => setGlobalSearch(e.target.value)}
            placeholder="0x… tx hash  /  0x… address  /  block number"
          />
          <button type="submit" className="btn-primary" style={{ flexShrink: 0 }}>
            Search
          </button>
        </form>
      </div>

      {error && <div className="error-message">{error}</div>}

      {/* ══════════ NETWORK STATS ══════════ */}
      <div className="network-stats">
        <div className="stats-grid">
          <div className="stat-card">
            <h4>Block Height</h4>
            <p>
              {blockTime && typeof blockTime.mining_time_sec === "number"
                ? `#${blockTime.block_number}`
                : (networkStats?.block_height ?? "—")}
            </p>
          </div>
          <div className="stat-card">
            <h4>Block Time</h4>
            <p>
              {blockTime && typeof blockTime.mining_time_sec === "number"
                ? `${blockTime.mining_time_sec.toFixed(3)} s`
                : "—"}
            </p>
          </div>
          <div className="stat-card">
            <h4>Validators</h4>
            <p>{validators.length || "—"}</p>
          </div>
          <div className="stat-card">
            <h4>Avg Block Time</h4>
            <p>
              {networkStats?.average_block_time
                ? `${networkStats.average_block_time.toFixed(2)} s`
                : "—"}
            </p>
          </div>
        </div>
      </div>

      {/* ══════════ RECENT BLOCKS + TXS ══════════ */}
      <div className="recent-data">

        {/* ── Recent Blocks ── */}
        <div className="recent-blocks">
          <h3>Recent Blocks
            <span style={{ marginLeft: "auto", fontSize: "0.72rem",
              color: "var(--text-muted)", fontWeight: 400 }}>
              {recentBlocks.length} blocks
            </span>
          </h3>
          <BlockList blocks={recentBlocks} showTxHash={false} />
        </div>

        {/* ── Recent Transactions ── */}
        <div className="recent-transactions">
          <h3>Recent Transactions</h3>

          {/* Search + filter row */}
          <div style={{ display: "flex", gap: 8, marginBottom: 10, flexWrap: "wrap" }}>
            <input
              type="text"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Filter transactions…"
              style={{ flex: 1, minWidth: 120 }}
            />
            <select
              value={searchField}
              onChange={(e) => setSearchField(e.target.value)}
              style={{ width: "auto" }}
            >
              <option value="all">All fields</option>
              <option value="hash">Tx hash</option>
              <option value="address">Address</option>
              <option value="status">Status</option>
              <option value="type">Tx type</option>
            </select>
          </div>

          {/* Status tabs */}
          <div className="tx-tabs" style={{ marginBottom: 10 }}>
            {["all", "pending", "confirmed", "failed"].map((tab) => (
              <button
                key={tab}
                className={`tx-tab${activeTab === tab ? " active" : ""}`}
                onClick={() => setActiveTab(tab)}
              >
                {tab[0].toUpperCase() + tab.slice(1)}
              </button>
            ))}
          </div>

          {/* TX count */}
          <div style={{ fontSize: "0.72rem", color: "var(--text-muted)",
            marginBottom: 8, textAlign: "right" }}>
            {visibleTxs.length} result{visibleTxs.length !== 1 ? "s" : ""}
          </div>

          {/* TX List */}
          {visibleTxs.length === 0 ? (
            <div className="no-transactions">No matching transactions.</div>
          ) : (
            visibleTxs.map((tx, i) => {
              const h       = tx.tx_hash || tx.txHash || `idx-${i}`;
              const from    = tx.from || tx.From || "";
              const to      = tx.to || tx.To || "";
              const gas     = tx.gas || tx.Gas || 0;
              const gasPrice = tx.gas_price || tx.GasPrice || 0;
              const fee     = gas * gasPrice;

              return (
                <div
                  key={h}
                  className="tx-item"
                  onClick={() => navigate(`/tx/${h}`)}
                >
                  {/* top row */}
                  <div style={{ display: "flex", justifyContent: "space-between",
                    alignItems: "flex-start", gap: 8 }}>
                    <div style={{ minWidth: 0 }}>
                      <div style={{ fontSize: "0.8rem", color: "var(--text-link)",
                        fontFamily: "var(--font-mono)", marginBottom: 2 }}>
                        {h.slice(0, 20)}…
                      </div>
                      <div style={{ fontSize: "0.72rem", color: "var(--text-muted)" }}>
                        {timeAgo(tx.timestamp || tx.Timestamp)}
                      </div>
                    </div>

                    <div style={{ display: "flex", gap: 5, alignItems: "center",
                      flexShrink: 0 }}>
                      <button
                        className="btn-copy-small"
                        onClick={(e) => { e.stopPropagation(); copyHash(h); }}
                      >
                        {copiedHash === h ? "✓" : "Copy"}
                      </button>
                      <TypeBadge type={tx.__txType} />
                      <StatusBadge status={tx.status || tx.Status} />
                    </div>
                  </div>

                  {/* address row */}
                  <div style={{ marginTop: 8, fontSize: "0.78rem",
                    color: "var(--text-secondary)", display: "flex",
                    alignItems: "center", gap: 6 }}>
                    <span style={{ color: "var(--text-muted)", fontSize: "0.7rem" }}>FROM</span>
                    <span style={{ fontFamily: "var(--font-mono)", color: "var(--text-link)" }}>
                      {from.slice(0, 14)}…
                    </span>
                    <span style={{ color: "var(--text-muted)" }}>→</span>
                    <span style={{ color: "var(--text-muted)", fontSize: "0.7rem" }}>TO</span>
                    <span style={{ fontFamily: "var(--font-mono)", color: "var(--text-link)" }}>
                      {to.slice(0, 14)}…
                    </span>
                  </div>

                  {/* value / gas row */}
                  <div style={{ marginTop: 6, display: "flex", gap: 14,
                    fontSize: "0.75rem", color: "var(--text-muted)", flexWrap: "wrap" }}>
                    <span><span style={{ color: "var(--text-secondary)" }}>Value</span>{" "}
                      <strong style={{ color: "var(--text-primary)" }}>{formatLQD(tx.value || 0)}</strong> LQD
                    </span>
                    <span><span style={{ color: "var(--text-secondary)" }}>Gas</span>{" "}
                      <strong style={{ color: "var(--text-primary)" }}>{gas}</strong>
                    </span>
                    {!!fee && (
                      <span><span style={{ color: "var(--text-secondary)" }}>Fee</span>{" "}
                        <strong style={{ color: "var(--text-primary)" }}>{formatLQD(fee)}</strong>
                      </span>
                    )}
                  </div>
                </div>
              );
            })
          )}
        </div>
      </div>

      {/* ══════════ VALIDATORS ══════════ */}
      <div className="validators-section">
        <h3>Active Validators</h3>
        {validators.length > 0 ? (
          <ValidatorList validators={validators} />
        ) : (
          <div className="no-validators">No validators found.</div>
        )}
      </div>
    </div>
  );
};

export default Dashboard;
