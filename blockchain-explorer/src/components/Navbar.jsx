import React, { useEffect, useState } from "react";
import { Link, useLocation } from "react-router-dom";
import { fetchChainJSON } from "../utils/api";

const PRIMARY_LINKS = [
  { to: "/", label: "Overview" },
  { to: "/blocks", label: "Blocks" },
  { to: "/transactions", label: "Transactions" },
  { to: "/validators", label: "Validators" },
  { to: "/liquidity", label: "Liquidity" },
];

const EXPLORE_GROUPS = [
  {
    label: "Network",
    items: [
      { to: "/stats", label: "Network analytics", description: "Chain activity and performance" },
      { to: "/rewards", label: "Reward analytics", description: "Validator and liquidity rewards" },
      { to: "/accounts", label: "Top accounts", description: "Public account activity" },
      { to: "/investor", label: "Investor evidence", description: "Readiness and protocol evidence" },
    ],
  },
  {
    label: "Assets",
    items: [
      { to: "/tokens", label: "Tokens", description: "Verified asset registry" },
      { to: "/pools", label: "Liquidity pools", description: "Pool state and routing" },
      { to: "/liquidity/providers", label: "LP tracker", description: "Provider positions and rewards" },
      { to: "/nfts", label: "NFT activity", description: "Mints, trades and transfers" },
    ],
  },
  {
    label: "Builders",
    items: [
      { to: "/contracts", label: "Contracts", description: "Inspect verified deployments" },
      { to: "/developers/api", label: "API reference", description: "Public endpoints and examples" },
      { to: "/developers/broadcast", label: "Broadcast", description: "Submit signed transactions" },
      { to: "/bridge", label: "Bridge", description: "Cross-chain operations" },
    ],
  },
];

const formatHeight = (value) => {
  const number = Number(value);
  return Number.isFinite(number) ? number.toLocaleString() : "—";
};

const Navbar = () => {
  const location = useLocation();
  const [open, setOpen] = useState(false);
  const [network, setNetwork] = useState({ status: "checking", height: null, baseFee: null });

  useEffect(() => {
    let active = true;

    const loadNetwork = async () => {
      const [healthResult, feeResult] = await Promise.allSettled([
        fetchChainJSON("/health", { cacheTtlMs: 3500, timeoutMs: 6000 }),
        fetchChainJSON("/basefee", { cacheTtlMs: 3500, timeoutMs: 6000 }),
      ]);
      if (!active) return;

      const health = healthResult.status === "fulfilled" ? healthResult.value : null;
      const fee = feeResult.status === "fulfilled" ? feeResult.value : null;
      setNetwork({
        status: health?.status === "ok" ? "live" : "degraded",
        height: health?.height ?? null,
        baseFee: fee?.base_fee ?? null,
      });
    };

    loadNetwork();
    const timer = window.setInterval(loadNetwork, 10000);
    return () => {
      active = false;
      window.clearInterval(timer);
    };
  }, []);

  useEffect(() => setOpen(false), [location.pathname]);

  const isActive = (to) =>
    to === "/" ? location.pathname === "/" : location.pathname.startsWith(to);

  return (
    <header className="explorer-header premium-header">
      <div className="network-ribbon" aria-label="Live network summary">
        <div className="network-ribbon-inner">
          <div className="ribbon-status">
            <span className={`live-indicator ${network.status}`} aria-hidden="true" />
            <span>PoDL public testnet</span>
            <strong>{network.status === "live" ? "Operational" : "Connecting"}</strong>
          </div>
          <div className="ribbon-metrics">
            <span>Height <strong>#{formatHeight(network.height)}</strong></span>
            <span>Base fee <strong>{network.baseFee ?? "—"}</strong></span>
            <span>Protocol <strong>PoDL v2</strong></span>
          </div>
        </div>
      </div>

      <nav className="navbar premium-navbar" aria-label="Primary navigation">
        <Link className="premium-brand" to="/" aria-label="PoDL Explorer home">
          <span className="podl-mark" aria-hidden="true">
            <span />
          </span>
          <span className="premium-brand-copy">
            <strong>PoDL</strong>
            <small>Network Explorer</small>
          </span>
        </Link>

        <div className={`premium-nav-links${open ? " open" : ""}`}>
          {PRIMARY_LINKS.map(({ to, label }) => (
            <Link key={to} to={to} className={isActive(to) ? "active" : ""}>
              {label}
            </Link>
          ))}

          <div className="explore-menu">
            <button className="explore-trigger" type="button" aria-haspopup="true">
              Explore <span aria-hidden="true">⌄</span>
            </button>
            <div className="explore-panel">
              {EXPLORE_GROUPS.map((group) => (
                <section key={group.label}>
                  <p>{group.label}</p>
                  {group.items.map((item) => (
                    <Link key={item.to} to={item.to}>
                      <strong>{item.label}</strong>
                      <small>{item.description}</small>
                    </Link>
                  ))}
                </section>
              ))}
            </div>
          </div>

          <Link className="wallet-launch" to="/wallet">
            Open wallet <span aria-hidden="true">↗</span>
          </Link>
        </div>

        <button
          className="premium-menu-button"
          type="button"
          onClick={() => setOpen((value) => !value)}
          aria-label="Toggle navigation"
          aria-expanded={open}
        >
          <span />
          <span />
        </button>
      </nav>
    </header>
  );
};

export default Navbar;
