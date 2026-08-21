import React, { useMemo, useState } from "react";
import { API_BASE, CHAIN_BASE, DEX_REGISTRY_BASE, WALLET_BASE } from "../utils/api";

const SERVICES = {
  all: { label: "All services" },
  chain: { label: "Chain API", base: CHAIN_BASE, note: "Ledger, consensus and protocol data" },
  gateway: { label: "Explorer API", base: API_BASE, note: "Public indexed data gateway" },
  wallet: { label: "Wallet API", base: WALLET_BASE, note: "Wallet transaction gateway" },
  dex: { label: "DEX registry", base: DEX_REGISTRY_BASE, note: "Public token and pool registry" },
};

const endpoint = (service, group, method, path, description, access = "Public") => ({
  service, group, method, path, description, access,
});

const ENDPOINTS = [
  endpoint("chain", "Network", "GET", "/", "Chain service information"),
  endpoint("chain", "Network", "GET", "/health", "Node health and canonical height"),
  endpoint("chain", "Network", "GET", "/readiness/mainnet", "Mainnet-readiness checks and evidence"),
  endpoint("chain", "Network", "GET", "/getheight", "Current canonical block height"),
  endpoint("chain", "Network", "GET", "/network", "Network activity and validator summary"),
  endpoint("chain", "Network", "GET", "/peers", "Connected peer information"),
  endpoint("chain", "Network", "GET", "/basefee", "Current protocol base fee"),
  endpoint("chain", "Network", "GET", "/blocktime/latest", "Latest block production timing"),
  endpoint("chain", "Network", "GET", "/metrics", "Prometheus-compatible node metrics"),
  endpoint("chain", "Network", "GET", "/mempool", "Pending transaction pool summary"),
  endpoint("chain", "Network", "GET", "/treasury", "Public protocol treasury state"),
  endpoint("chain", "Network", "GET", "/chain/summary", "Compact chain summary"),
  endpoint("chain", "Network", "GET", "/chain/global", "Aggregated public chain state"),
  endpoint("chain", "Network", "GET", "/chain/export", "Canonical chain export", "Large response"),
  endpoint("chain", "Index", "GET", "/v2/index/status", "Persistent index height, transaction count and lag"),
  endpoint("chain", "Index", "GET", "/v2/index/search?q={query}", "Search a block, transaction or address"),
  endpoint("chain", "Index", "GET", "/v2/protocol/status", "Protocol and state version status"),
  endpoint("chain", "Index", "POST", "/v2/product/suitability", "Evaluate disclosed product suitability", "Validated input"),

  endpoint("chain", "Blocks", "GET", "/fetch_last_n_block?page={page}&size={size}", "Paginated canonical blocks"),
  endpoint("chain", "Blocks", "GET", "/block/{id}", "Block by number or hash"),

  endpoint("chain", "Transactions", "GET", "/transactions?page={page}&size={size}", "Paginated finalized transactions"),
  endpoint("chain", "Transactions", "GET", "/transactions/recent", "Most recent finalized transactions"),
  endpoint("chain", "Transactions", "GET", "/transactions/pending", "Pending transactions"),
  endpoint("chain", "Transactions", "GET", "/transactions/internal", "Indexed internal transactions"),
  endpoint("chain", "Transactions", "GET", "/tx/{hash}", "Transaction receipt and execution details"),
  endpoint("chain", "Transactions", "POST", "/send_tx", "Broadcast one signed transaction", "Signed payload"),
  endpoint("chain", "Transactions", "POST", "/send_tx/batch", "Broadcast a signed transaction batch", "Signed payload"),
  endpoint("chain", "Transactions", "POST", "/rpc", "JSON-RPC compatible request gateway", "Validated input"),
  endpoint("chain", "Transactions", "POST", "/faucet", "Request public-testnet LQD", "Testnet only"),

  endpoint("chain", "Accounts", "GET", "/balance?address={address}", "Native LQD balance"),
  endpoint("chain", "Accounts", "GET", "/account/{address}/nonce", "Next account nonce"),
  endpoint("chain", "Accounts", "GET", "/address/{address}/overview", "Account balance and activity summary"),
  endpoint("chain", "Accounts", "GET", "/address/{address}/transactions", "Transactions for an account"),

  endpoint("chain", "Validators", "GET", "/validators", "Complete validator registry"),
  endpoint("chain", "Validators", "GET", "/validators/active", "Active validator set"),
  endpoint("chain", "Validators", "GET", "/validators/sync", "Validator synchronization state"),
  endpoint("chain", "Validators", "GET", "/validator/{address}", "Validator state by address"),
  endpoint("chain", "Validators", "GET", "/validator/onboarding", "Public validator onboarding requirements"),

  endpoint("chain", "Rewards", "GET", "/rewards/recent", "Recent validator and liquidity rewards"),
  endpoint("chain", "Rewards", "GET", "/rewards/table", "Paginated reward ledger"),
  endpoint("chain", "Rewards", "GET", "/rewards/latest", "Latest reward distribution"),
  endpoint("chain", "Rewards", "GET", "/rewards/summary", "Aggregate reward totals"),
  endpoint("chain", "Rewards", "GET", "/rewards/address/{address}", "Rewards attributed to an address"),

  endpoint("chain", "Liquidity", "GET", "/liquidity", "Liquidity protocol summary"),
  endpoint("chain", "Liquidity", "GET", "/liquidity/info?address={address}", "Liquidity position for an address"),
  endpoint("chain", "Liquidity", "GET", "/liquidity/all", "All indexed liquidity positions"),
  endpoint("chain", "Liquidity", "GET", "/liquidity/pools", "Registered pool state and routing data"),
  endpoint("chain", "Liquidity", "GET", "/liquidity/vault/status", "Strategy vault state and accounting"),
  endpoint("chain", "Liquidity", "GET", "/liquidity/dynamic/status", "Dynamic-liquidity engine status"),
  endpoint("chain", "Liquidity", "POST", "/liquidity/lock", "Lock liquidity for protocol credit", "Signed action"),
  endpoint("chain", "Liquidity", "POST", "/liquidity/unlock", "Begin liquidity unlock", "Signed action"),
  endpoint("chain", "Liquidity", "POST", "/liquidity/provide", "Provide protocol liquidity", "Signed action"),
  endpoint("chain", "Liquidity", "POST", "/liquidity/unstake", "Request liquidity unstaking", "Signed action"),
  endpoint("chain", "Liquidity", "POST", "/liquidity/claim", "Claim available liquidity rewards", "Signed action"),
  endpoint("chain", "Liquidity", "POST", "/liquidity/vault/deposit", "Deposit into the strategy vault", "Signed action"),
  endpoint("chain", "Liquidity", "POST", "/liquidity/vault/withdraw", "Request a strategy-vault withdrawal", "Signed action"),

  endpoint("chain", "Contracts", "GET", "/contract/list", "Indexed contract deployments"),
  endpoint("chain", "Contracts", "GET", "/contract/con1?address={address}", "Legacy contract state view"),
  endpoint("chain", "Contracts", "GET", "/contract/con2?address={address}", "Legacy contract event view"),
  endpoint("chain", "Contracts", "GET", "/contract/getAbi?address={address}", "Verified contract ABI"),
  endpoint("chain", "Contracts", "GET", "/contract/storage?address={address}", "Public contract storage"),
  endpoint("chain", "Contracts", "GET", "/contract/code?address={address}", "Deployed contract bytecode"),
  endpoint("chain", "Contracts", "GET", "/contract/events?address={address}", "Indexed contract events"),
  endpoint("chain", "Contracts", "GET", "/contract/verification?address={address}", "Source verification status"),
  endpoint("chain", "Contracts", "GET", "/token/{address}/holders", "Token holder registry"),
  endpoint("chain", "Contracts", "GET", "/dex/current", "Current protocol DEX configuration"),
  endpoint("chain", "Contracts", "POST", "/contract/call", "Submit a signed contract call", "Signed action"),
  endpoint("chain", "Contracts", "POST", "/contract/deploy", "Deploy a signed contract", "Signed action"),
  endpoint("chain", "Contracts", "POST", "/contract/compile", "Compile supported contract source", "Validated input"),
  endpoint("chain", "Contracts", "POST", "/contract/verify", "Submit contract source verification", "Validated input"),

  endpoint("chain", "Bridge", "GET", "/bridge/requests", "Public bridge request ledger"),
  endpoint("chain", "Bridge", "GET", "/bridge/families", "Supported bridge chain families"),
  endpoint("chain", "Bridge", "GET", "/bridge/tokens", "Supported bridge assets"),
  endpoint("chain", "Bridge", "GET", "/bridge/chains", "Configured public bridge chains"),
  endpoint("chain", "Bridge", "POST", "/bridge/lock_bsc", "Submit a public BSC lock request", "Signed action"),
  endpoint("chain", "Bridge", "POST", "/bridge/burn_lqd", "Submit a public LQD burn request", "Signed action"),
  endpoint("chain", "Bridge", "POST", "/bridge/lock_chain", "Submit a public external-chain lock", "Signed action"),
  endpoint("chain", "Bridge", "POST", "/bridge/burn_chain", "Submit a public external-chain burn", "Signed action"),

  endpoint("gateway", "Gateway", "GET", "/health", "Explorer gateway health"),
  endpoint("gateway", "Gateway", "GET", "/chain/global", "Consolidated public explorer data"),
  endpoint("gateway", "Gateway", "GET", "/", "Gateway service and upstream status"),

  endpoint("wallet", "Wallet", "GET", "/health", "Wallet gateway health"),
  endpoint("wallet", "Wallet", "POST", "/wallet/new", "Create a new testnet wallet", "Sensitive response"),
  endpoint("wallet", "Wallet", "POST", "/wallet/import/mnemonic", "Restore a wallet from a recovery phrase", "Sensitive input"),
  endpoint("wallet", "Wallet", "POST", "/wallet/import/private-key", "Restore a wallet from a private key", "Sensitive input"),
  endpoint("wallet", "Wallet", "GET", "/wallet/balance?address={address}", "Wallet balance summary"),
  endpoint("wallet", "Wallet", "GET", "/wallet/token-balance?address={address}&token={token}", "Token balance for a wallet"),
  endpoint("wallet", "Wallet", "POST", "/wallet/contract-template", "Build a supported contract transaction template", "Validated input"),
  endpoint("wallet", "Wallet", "POST", "/wallet/send", "Create and broadcast a wallet transfer", "Sensitive input"),
  endpoint("wallet", "Wallet", "POST", "/wallet/send_batch", "Create and broadcast a transfer batch", "Sensitive input"),
  endpoint("wallet", "Bridge", "POST", "/wallet/bridge/lock", "Wallet bridge lock transaction", "Sensitive input"),
  endpoint("wallet", "Bridge", "POST", "/wallet/bridge/burn", "Wallet bridge burn transaction", "Sensitive input"),
  endpoint("wallet", "Bridge", "POST", "/wallet/bridge/lock_bsc_token", "Build and submit a BSC token lock", "Sensitive input"),
  endpoint("wallet", "Bridge", "POST", "/wallet/bridge/burn_lqd_token", "Build and submit an LQD bridge burn", "Sensitive input"),
  endpoint("wallet", "Bridge", "POST", "/wallet/bridge/bsc_lock_tx", "Build a BSC lock transaction", "Sensitive input"),

  endpoint("dex", "Registry", "GET", "/health", "DEX registry health"),
  endpoint("dex", "Registry", "GET", "/config", "Public DEX registry configuration"),
  endpoint("dex", "Registry", "GET", "/tokens", "Registered public token list"),
  endpoint("dex", "Registry", "GET", "/pools", "Registered public liquidity pools"),
];

const ApiDocsPage = () => {
  const [service, setService] = useState("all");
  const [method, setMethod] = useState("ALL");
  const [query, setQuery] = useState("");
  const [copied, setCopied] = useState("");

  const filtered = useMemo(() => {
    const needle = query.trim().toLowerCase();
    return ENDPOINTS.filter((item) => {
      if (service !== "all" && item.service !== service) return false;
      if (method !== "ALL" && item.method !== method) return false;
      if (!needle) return true;
      return `${item.path} ${item.description} ${item.group}`.toLowerCase().includes(needle);
    });
  }, [method, query, service]);

  const groups = useMemo(() => {
    const result = [];
    filtered.forEach((item) => {
      const key = `${item.service}:${item.group}`;
      let group = result.find((entry) => entry.key === key);
      if (!group) {
        group = { key, service: item.service, label: item.group, endpoints: [] };
        result.push(group);
      }
      group.endpoints.push(item);
    });
    return result;
  }, [filtered]);

  const copy = async (item) => {
    const value = `${SERVICES[item.service].base}${item.path}`;
    await navigator.clipboard.writeText(value);
    setCopied(value);
    window.setTimeout(() => setCopied(""), 1200);
  };

  return (
    <main className="clean-api-page">
      <header className="clean-api-head">
        <span>Developer reference</span>
        <h1>Public API</h1>
        <p>Public PoDL endpoints, grouped by service with the exact HTTP method required.</p>
      </header>

      <section className="clean-service-grid" aria-label="Public service URLs">
        {Object.entries(SERVICES).filter(([key]) => key !== "all").map(([key, item]) => (
          <article key={key}>
            <div><i className={key} aria-hidden="true" /><strong>{item.label}</strong></div>
            <p>{item.note}</p>
            <code>{item.base}</code>
            <button type="button" onClick={() => navigator.clipboard.writeText(item.base)}>Copy</button>
          </article>
        ))}
      </section>

      <section className="clean-api-catalog">
        <div className="clean-api-tools">
          <div className="clean-api-search">
            <span aria-hidden="true" />
            <label className="sr-only" htmlFor="api-search">Search endpoints</label>
            <input id="api-search" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search endpoint or description" />
          </div>
          <select value={method} onChange={(event) => setMethod(event.target.value)} aria-label="Filter by HTTP method">
            <option value="ALL">All methods</option>
            <option value="GET">GET</option>
            <option value="POST">POST</option>
            <option value="PUT">PUT</option>
            <option value="DELETE">DELETE</option>
          </select>
        </div>

        <div className="clean-service-tabs" role="tablist" aria-label="Filter by service">
          {Object.entries(SERVICES).map(([key, item]) => (
            <button
              key={key}
              type="button"
              className={service === key ? "active" : ""}
              onClick={() => setService(key)}
              role="tab"
              aria-selected={service === key}
            >
              {item.label}
            </button>
          ))}
        </div>

        <div className="clean-api-count"><strong>{filtered.length}</strong> public endpoints</div>

        <div className="clean-endpoint-groups">
          {groups.map((group) => (
            <section key={group.key}>
              <header><div><span>{SERVICES[group.service].label}</span><h2>{group.label}</h2></div><code>{SERVICES[group.service].base}</code></header>
              <div className="clean-endpoint-list">
                {group.endpoints.map((item) => {
                  const url = `${SERVICES[item.service].base}${item.path}`;
                  return (
                    <article key={`${item.service}-${item.method}-${item.path}`}>
                      <span className={`clean-method ${item.method.toLowerCase()}`}>{item.method}</span>
                      <code>{item.path}</code>
                      <p>{item.description}</p>
                      <span className="clean-access">{item.access}</span>
                      <button type="button" onClick={() => copy(item)} aria-label={`Copy ${item.method} ${item.path}`}>
                        {copied === url ? "Copied" : "Copy"}
                      </button>
                    </article>
                  );
                })}
              </div>
            </section>
          ))}
          {!groups.length && <div className="clean-api-empty">No endpoints match this filter.</div>}
        </div>
      </section>

      <p className="clean-api-note">
        Administrative recovery, peer mutation, oracle injection, registry mutation and private bridge routes are intentionally excluded from this public reference.
      </p>
    </main>
  );
};

export default ApiDocsPage;
