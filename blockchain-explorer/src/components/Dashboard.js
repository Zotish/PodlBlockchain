



// src/components/Dashboard.jsx
import React, { useState, useEffect, useMemo } from "react";
import BlockList from "./BlockList";
import ValidatorList from "./ValidatorList";
import DebugComponent from "./DebugComponent";
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
      setRecentBlocks(sortedBlocks.slice(0, 10));

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
    const id = setInterval(fetchData,1000);
    return () => clearInterval(id);
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

    const from = (tx.from || tx.From || "").toLowerCase();
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
    const color =
      s === "succsess" ? "#16a34a" : s === "failed" ? "#dc2626" : "#ca8a04";
    const text =
      s === "succsess" ? "confirmed" : s === "failed" ? "failed" : "pending";

    return (
      <span
        style={{
          background: color,
          color: "#fff",
          padding: "2px 6px",
          borderRadius: 6,
          fontSize: 11,
        }}
      >
        {text}
      </span>
    );
  };

  const TypeBadge = ({ type }) => {
    const t = (type || "").toLowerCase();
    const map = {
      reward_validator: ["#0ea5e9", "validator reward"],
      reward_lp: ["#8b5cf6", "lp reward"],
      reward_contributor: ["#14b8a6", "contributor reward"],
      reward: ["#0ea5e9", "reward"],
      contract_create: ["#a855f7", "contract create"],
      contract_call: ["#22c55e", "contract call"],
      token_transfer: ["#f97316", "token transfer"],
      transfer: ["#4b5563", "transfer"],
    };

    const [bg, label] = map[t] || map.transfer;

    return (
      <span
        style={{
          background: bg,
          color: "#fff",
          padding: "2px 6px",
          borderRadius: 6,
          fontSize: 11,
        }}
      >
        {label}
      </span>
    );
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

  const latestBlock =
    recentBlocks && recentBlocks.length > 0 ? recentBlocks[0] : null;

  /* ===========================================================
                         RENDER START
     =========================================================== */
  return (
    <div className="dashboard">
      {/* -------- Global Search -------- */}
      <form
        onSubmit={handleGlobalSearchSubmit}
        style={{
          marginBottom: 20,
          display: "flex",
          gap: 10,
          justifyContent: "space-between",
        }}
      >
        <input
          value={globalSearch}
          onChange={(e) => setGlobalSearch(e.target.value)}
          placeholder="Search: tx hash / address / block number"
          style={{
            flex: 1,
            padding: "10px 14px",
            border: "1px solid #ccc",
            borderRadius: 10,
          }}
        />
        <button
          type="submit"
          className="btn-primary"

          style={{ padding: "10px 10px" ,height: "42px",width: "80px"}}
        >
          Search
        </button>
      </form>

      {error && <div className="error-message">{error}</div>}

      {/* -------- Network Stats -------- */}
      <div className="network-stats">
        <h3>Network Statistics</h3>
        <div className="stats-grid">
        <div className="stat-card">
  <h4>Block Height and Block Producing Time</h4>
  <p>
    {blockTime && typeof blockTime.mining_time_sec === "number"
      ? `#${blockTime.block_number} • ${blockTime.mining_time_sec.toFixed(5)} s`
      : (networkStats?.height ?? "N/A")}
  </p>
</div>

          <div className="stat-card">
            <h4>Validators</h4>
            <p>{validators.length}</p>
          </div>
          <div className="stat-card">
            <h4>Avg Block Time</h4>
            <p>{networkStats?.average_block_time
                ? `${networkStats.average_block_time.toFixed(2)} s`: "N/A"} </p>
          </div>
        </div>
      </div>

      {/* -------- Recent Blocks + TX Story -------- */}
      <div className="recent-data">
        <div className="recent-blocks">
          <h3>Recent Blocks ({recentBlocks.length})</h3>
          <BlockList blocks={recentBlocks} showTxHash={false} />
        </div>

        {/* -------- Transaction Story -------- */}
        <div className="recent-transactions">
          <h3>Recent Transactions</h3>

          {/* Search controls */}
          <div
            style={{
              marginBottom: 10,
              display: "flex",
              gap: 8,
              flexWrap: "wrap",
            }}
          >
            <input
              type="text"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Search recent transactions…"
              style={{
                padding: "10px 14px",
                width: "60%",
                border: "1px solid #ccc",
                borderRadius: 10,
              }}
            />

            <select
              value={searchField}
              onChange={(e) => setSearchField(e.target.value)}
              style={{
                padding: "10px 16px",
                borderRadius: 10,
                background: "#2563eb",
                color: "#fff",
                border: "1px solid #2563eb",
              }}
            >
              <option value="all">All fields</option>
              <option value="hash">Tx hash</option>
              <option value="address">Address</option>
              <option value="status">Status</option>
              <option value="type">Tx type</option>
            </select>

            <span style={{ marginLeft: "auto", fontSize: 12, color: "#666" }}>
              Showing {visibleTxs.length} result
              {visibleTxs.length === 1 ? "" : "s"}
            </span>
          </div>

          {/* Tabs */}
          <div style={{ display: "flex", gap: 6, marginBottom: 12 }}>
            {["all", "pending", "confirmed", "failed"].map((tab) => (
              <button
                key={tab}
                onClick={() => setActiveTab(tab)}
                style={{
                  fontSize: 12,
                  padding: "6px 10px",
                  borderRadius: 8,
                  border:
                    activeTab === tab ? "1px solid #2563eb" : "1px solid #bbb",
                  background: activeTab === tab ? "#eff6ff" : "#fff",
                }}
              >
                {tab[0].toUpperCase() + tab.slice(1)}
              </button>
            ))}
          </div>

          {/* TX List */}
          {visibleTxs.length === 0 ? (
            <div className="no-transactions">No matching transactions.</div>
          ) : (
            visibleTxs.map((tx, i) => {
              const h = tx.tx_hash || tx.txHash || `idx-${i}`;
              const from = tx.from || tx.From || "";
              const to = tx.to || tx.To || "";
              const gas = tx.gas || tx.Gas || 0;
              const gasPrice = tx.gas_price || tx.GasPrice || 0;
              const fee = gas * gasPrice;

              return (
                <div
                  key={h}
                  className="tx-item"
                  style={{
                    border: "1px solid #e5e7eb",
                    borderRadius: 10,
                    padding: 10,
                    marginBottom: 8,
                    background: "#f9fafb",
                    cursor: "pointer",
                  }}
                  onClick={() => navigate(`/tx/${h}`)}
                >
                  <div
                    style={{
                      display: "flex",
                      justifyContent: "space-between",
                      alignItems: "center",
                      gap: 8,
                    }}
                  >
                    <div>
                      <div style={{ fontSize: 13 }}>
                        <strong>Tx:</strong>{" "}
                        <span style={{ fontFamily: "monospace" }}>
                          {h.slice(0, 18)}…
                        </span>
                      </div>
                      <div style={{ fontSize: 11, color: "#666" }}>
                        {timeAgo(tx.timestamp || tx.Timestamp)}
                      </div>
                    </div>

                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        copyHash(h);
                      }}
                      className="btn-copy-small"
                    >
                      {copiedHash === h ? "Copied" : "Copy"}
                    </button>

                    <div style={{ display: "flex", gap: 6 }}>
                      <TypeBadge type={tx.__txType} />
                      <StatusBadge status={tx.status || tx.Status} />
                    </div>
                  </div>

                  <div style={{ marginTop: 6, fontSize: 13 }}>
                    <strong>From:</strong> {from.slice(0, 18)}… →{" "}
                    <strong>To:</strong> {to.slice(0, 18)}…
                  </div>

                  <div style={{ marginTop: 6, fontSize: 12, color: "#555" }}>
                    <span style={{ marginRight: 10 }}>
                      <strong>Value:</strong> {formatLQD(tx.value || 0)}
                    </span>
                    <span style={{ marginRight: 10 }}>
                      <strong>Gas:</strong> {gas}
                    </span>
                    <span style={{ marginRight: 10 }}>
                      <strong>GasPrice:</strong> {formatLQD(gasPrice)}
                    </span>
                    {!!fee && (
                      <span>
                        <strong>Fee:</strong> {formatLQD(fee)}
                      </span>
                    )}
                  </div>
                </div>
              );
            })
          )}
        </div>
      </div>

      {/* -------- Validators -------- */}
      <div className="validators-section">
        <h3>Validators</h3>
        {validators.length > 0 ? (
          <ValidatorList validators={validators} />
        ) : (
          <div className="no-validators">
            <p>No validators found.</p>
            {/* <DebugComponent /> */}
          </div>
        )}
      </div>
    </div>
  );
};

export default Dashboard;
