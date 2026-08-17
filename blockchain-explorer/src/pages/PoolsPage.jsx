import React, { useEffect, useState } from "react";
import { DataSurface, ExplorerPageHero, MetricStrip } from "../components/ExplorerPage";
import { fetchDexRegistryPools, fetchJSON, firstNodeResult } from "../utils/api";
import { formatLQD } from "../utils/lqdUnits";

const REFRESH_MS = 5000;

export default function PoolsPage() {
  const [pools, setPools] = useState({});
  const [total, setTotal] = useState(0);
  const [target, setTarget] = useState(0);
  const [unallocated, setUnallocated] = useState(0);
  const [registryPools, setRegistryPools] = useState([]);
  const [error, setError] = useState("");

  const loadPools = async () => {
    try {
      setError("");
      const [data, registry] = await Promise.all([
        fetchJSON("/liquidity/pools"),
        fetchDexRegistryPools().catch(() => []),
      ]);
      const payload = firstNodeResult(data) || {};
      setPools(payload.pools || {});
      setRegistryPools(registry);
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
    <main className="pools-page premium-route-page">
      <ExplorerPageHero
        eyebrow="Dynamic liquidity fabric"
        title="Liquidity, routed by measurable demand."
        description="Inspect pool reserves, registry policy and capital still available for allocation across the PoDL liquidity network."
        metaLabel="Routing model"
        metaValue="Dynamic pool allocation"
      />
      {error && <div className="error">{error}</div>}

      <MetricStrip items={[
        { label: 'Total liquidity', value: `${formatLQD(total)} LQD`, note: 'tracked reserves' },
        { label: 'Allocation target', value: `${formatLQD(target)} LQD`, note: 'current target share' },
        { label: 'Unallocated', value: `${formatLQD(unallocated)} LQD`, note: 'available capital' },
        { label: 'Tracked pools', value: Math.max(entries.length, registryPools.length).toLocaleString(), note: 'live and registered' },
      ]} />

      {registryPools.length > 0 && (
        <DataSurface title="Verified registry" description="Governed pool metadata and active routing weights.">
          <div className="pool-registry-grid">{registryPools.map((pool) => (
            <article key={pool.address || `${pool.token_a}-${pool.token_b}`}>
              <span>{pool.tier || "Tier 3"}</span>
              <strong>{pool.token_a || "LQD"} / {pool.token_b || pool.symbol || "Token"}</strong>
              <small>Routing weight {pool.weight || "0.35x"}</small>
            </article>
          ))}</div>
        </DataSurface>
      )}

      <DataSurface title="On-chain pool reserves" description="Current contract-level liquidity reported by the public API.">
        <div className="premium-table-scroll"><table className="table">
          <thead>
            <tr><th>Pool contract</th><th>Liquidity</th><th>State</th></tr>
          </thead>
          <tbody>
            {entries.length === 0 ? (
              <tr><td colSpan="3" className="tracker-empty">No pools found</td></tr>
            ) : entries.map(([addr, amount]) => (
              <tr key={addr}>
                <td><code>{addr}</code></td>
                <td><strong>{formatLQD(amount)} LQD</strong></td>
                <td><span className="badge badge-green">Active</span></td>
              </tr>
            ))}
          </tbody>
        </table></div>
      </DataSurface>
    </main>
  );
}
