import React, { useCallback, useEffect, useState } from "react";
import { ExplorerPageHero, MetricStrip } from "../components/ExplorerPage";
import { fetchJSON } from "../utils/api";

const validAddress = (value) => !value || /^0x[0-9a-fA-F]{40}$/.test(value);
const shortValue = (value, size = 10) => value ? `${String(value).slice(0, size)}…` : "—";

const BridgePage = () => {
  const [address, setAddress] = useState("");
  const [mode, setMode] = useState("public");
  const [requests, setRequests] = useState([]);
  const [tokenMappings, setTokenMappings] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const loadRequests = useCallback(async () => {
    const normalizedAddress = address.trim();
    if (!validAddress(normalizedAddress)) {
      setError("Enter a valid 20-byte public address or leave the filter empty.");
      return;
    }

    setLoading(true);
    setError("");
    try {
      const params = [];
      if (normalizedAddress) params.push(`address=${encodeURIComponent(normalizedAddress)}`);
      params.push(`mode=${encodeURIComponent(mode)}`);
      const data = await fetchJSON(`/bridge/requests?${params.join("&")}`);
      setRequests(Array.isArray(data) ? data : []);
    } catch (loadError) {
      setRequests([]);
      setError(loadError?.message || "Bridge activity is currently unavailable.");
    } finally {
      setLoading(false);
    }
  }, [address, mode]);

  useEffect(() => {
    let active = true;
    fetchJSON("/bridge/tokens")
      .then((data) => {
        if (active) setTokenMappings(Array.isArray(data) ? data : []);
      })
      .catch(() => {
        if (active) setTokenMappings([]);
      });
    return () => {
      active = false;
    };
  }, []);

  useEffect(() => {
    const timer = window.setTimeout(loadRequests, 150);
    return () => window.clearTimeout(timer);
  }, [loadRequests]);

  return (
    <main className="bridge-page premium-route-page">
      <ExplorerPageHero
        eyebrow="Cross-chain evidence"
        title="Bridge activity"
        description="Inspect registered mappings and public bridge requests. Transaction signing is intentionally unavailable in the explorer."
        metaLabel="Security mode"
        metaValue="Read-only"
      />
      <MetricStrip items={[
        { label: "Request class", value: mode, note: "public ledger filter" },
        { label: "Token mappings", value: tokenMappings.length.toLocaleString(), note: "registered assets" },
        { label: "Visible requests", value: requests.length.toLocaleString(), note: "matching operations" },
        { label: "Environment", value: "Testnet", note: "no real funds" },
      ]} />

      <div className="bridge-workbench">
        <section className="card bridge-mode-card">
          <div className="section-heading-row">
            <div>
              <span className="eyebrow">Public query</span>
              <h3>Filter bridge ledger</h3>
            </div>
            <span className="status-pill">Watch only</span>
          </div>
          <div className="template-wrap" role="group" aria-label="Bridge request class">
            <button type="button" className={mode === "public" ? "chip active" : "chip"} onClick={() => setMode("public")}>Public</button>
            <button type="button" className={mode === "private" ? "chip active" : "chip"} onClick={() => setMode("private")}>Private class</button>
          </div>
          <div className="form-row">
            <label htmlFor="bridge-address-filter">Public account address</label>
            <input
              id="bridge-address-filter"
              value={address}
              onChange={(event) => setAddress(event.target.value)}
              placeholder="Optional: 0x…"
              autoComplete="off"
              spellCheck="false"
            />
          </div>
          <button type="button" className="btn-secondary" onClick={loadRequests}>Refresh public data</button>
          {error && <div className="notice" role="alert">{error}</div>}
        </section>

        <section className="card bridge-action-card">
          <span className="eyebrow">Trust boundary</span>
          <h3>Signing disabled</h3>
          <p>
            This public explorer never requests, stores or transmits a private key or recovery
            phrase. Bridge execution remains unavailable until the relayer, proof path and local
            signer pass independent security review.
          </p>
          <div className="notice">Testnet observation only. Do not send real assets to bridge addresses.</div>
        </section>

        <section className="card bridge-action-card">
          <span className="eyebrow">Registered assets</span>
          <h3>Token mappings</h3>
          {tokenMappings.length === 0 ? (
            <p>No bridge token mappings are currently published.</p>
          ) : (
            <div className="bridge-mapping-list">
              {tokenMappings.map((token, index) => (
                <div key={token.lqd_token || token.external_token || index} className="balance-item">
                  <span>{token.symbol || "Asset"}</span>
                  <strong title={token.lqd_token || token.external_token || ""}>
                    {shortValue(token.lqd_token || token.external_token)}
                  </strong>
                </div>
              ))}
            </div>
          )}
        </section>

        <section className="card bridge-request-card">
          <div className="section-heading-row">
            <div>
              <span className="eyebrow">On-chain records</span>
              <h3>Bridge requests</h3>
            </div>
            <span className="status-pill">{loading ? "Loading" : `${requests.length} records`}</span>
          </div>
          <div className="table-scroll" tabIndex="0" aria-label="Bridge request table">
            <table className="table">
              <thead>
                <tr>
                  <th>ID</th>
                  <th>From</th>
                  <th>To</th>
                  <th>Amount</th>
                  <th>Status</th>
                  <th>Token</th>
                  <th>LQD Tx</th>
                  <th>External Tx</th>
                </tr>
              </thead>
              <tbody>
                {!loading && requests.length === 0 ? (
                  <tr><td colSpan="8">No matching bridge requests.</td></tr>
                ) : requests.map((request) => (
                  <tr key={request.id || `${request.lqd_tx_hash}-${request.bsc_tx_hash}`}>
                    <td title={request.id}>{shortValue(request.id)}</td>
                    <td title={request.from}>{shortValue(request.from)}</td>
                    <td title={request.to}>{shortValue(request.to)}</td>
                    <td>{request.amount ?? "—"}</td>
                    <td>{request.status || "unknown"}</td>
                    <td title={request.token}>{request.token ? shortValue(request.token) : "LQD"}</td>
                    <td title={request.lqd_tx_hash}>{shortValue(request.lqd_tx_hash)}</td>
                    <td title={request.bsc_tx_hash || request.external_tx_hash}>{shortValue(request.bsc_tx_hash || request.external_tx_hash)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      </div>
    </main>
  );
};

export default BridgePage;
