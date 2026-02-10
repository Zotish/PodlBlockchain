import React, { useEffect, useState } from "react";
import { fetchJSON, firstNodeResult } from "../utils/api";
import { formatLQD } from "../utils/lqdUnits";

const REFRESH_MS = 5000;

export default function PoolsPage() {
  const [pools, setPools] = useState({});
  const [total, setTotal] = useState(0);
  const [target, setTarget] = useState(0);
  const [unallocated, setUnallocated] = useState(0);
  const [error, setError] = useState("");

  const loadPools = async () => {
    try {
      setError("");
      const data = await fetchJSON("/liquidity/pools");
      const payload = firstNodeResult(data) || {};
      setPools(payload.pools || {});
      setTotal(payload.total || 0);
      setTarget(payload.target_equal || 0);
      setUnallocated(payload.unallocated || 0);
    } catch (err) {
      setError(err.message || "Failed to load pools");
    }
  };

  useEffect(() => {
    loadPools();
    const id = setInterval(loadPools, REFRESH_MS);
    return () => clearInterval(id);
  }, []);

  const entries = Object.entries(pools);

  return (
    <div>
      <h2>Liquidity Pools</h2>
      <p>Dynamic routing keeps pools balanced to the target equal share.</p>
      {error && <div className="error">{error}</div>}

      <div className="stats-card">
        <div>Total Liquidity: {formatLQD(total)}</div>
        <div>Target Equal: {formatLQD(target)}</div>
        <div>Unallocated: {formatLQD(unallocated)}</div>
      </div>

      <table className="table">
        <thead>
          <tr>
            <th>Pool (Contract)</th>
            <th>Liquidity</th>
          </tr>
        </thead>
        <tbody>
          {entries.length === 0 ? (
            <tr>
              <td colSpan="2">No pools found</td>
            </tr>
          ) : (
            entries.map(([addr, amount]) => (
              <tr key={addr}>
                <td>{addr}</td>
                <td>{formatLQD(amount)}</td>
              </tr>
            ))
          )}
        </tbody>
      </table>
    </div>
  );
}
