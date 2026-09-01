import React, { useEffect, useState } from "react";
import { Link, useLocation, useNavigate } from "react-router-dom";
import { fetchChainJSON } from "../utils/api";

const PRIMARY_LINKS = [
  { to: "/blocks", label: "Blocks" },
  { to: "/transactions", label: "Transactions" },
  { to: "/validators", label: "Validators" },
  { to: "/tokens", label: "Tokens" },
  { to: "/developers/api", label: "API" },
];

const MORE_LINKS = [
  { to: "/pools", label: "Liquidity pools" },
  { to: "/liquidity/providers", label: "Liquidity providers" },
  { to: "/accounts", label: "Accounts" },
  { to: "/contracts", label: "Contracts" },
  { to: "/rewards", label: "Rewards" },
  { to: "/nfts", label: "NFTs" },
  { to: "/bridge", label: "Bridge" },
  { to: "/stats", label: "Network statistics" },
  { to: "/investor", label: "Protocol evidence" },
  { to: "/developers/broadcast", label: "Broadcast transaction" },
];

const formatNumber = (value) => {
  const number = Number(value);
  return Number.isFinite(number) ? number.toLocaleString() : "—";
};

const formatAge = (seconds) => {
  if (!Number.isFinite(seconds) || seconds < 0) return "—";
  if (seconds < 60) return `${Math.floor(seconds)}s`;
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m`;
  return `${Math.floor(seconds / 3600)}h`;
};

const Navbar = () => {
  const location = useLocation();
  const navigate = useNavigate();
  const [mobileOpen, setMobileOpen] = useState(false);
  const [moreOpen, setMoreOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [network, setNetwork] = useState({ status: "checking", height: null, baseFee: null, blockAge: null });

  useEffect(() => {
    let active = true;
    const load = async () => {
      const [healthResult, feeResult, blockResult] = await Promise.allSettled([
        fetchChainJSON("/health", { cacheTtlMs: 3000, timeoutMs: 6000 }),
        fetchChainJSON("/basefee", { cacheTtlMs: 3000, timeoutMs: 6000 }),
        fetchChainJSON("/fetch_last_n_block?page=1&size=1", { cacheTtlMs: 3000, timeoutMs: 6000 }),
      ]);
      if (!active) return;
      const health = healthResult.status === "fulfilled" ? healthResult.value : null;
      const fee = feeResult.status === "fulfilled" ? feeResult.value : null;
      const payload = blockResult.status === "fulfilled" ? blockResult.value : null;
      const latest = payload?.blocks?.[0] || payload?.data?.blocks?.[0] || null;
      const timestamp = Number(latest?.timestamp ?? latest?.TimeStamp);
      const blockAge = Number.isFinite(timestamp) ? Math.max(0, Math.floor(Date.now() / 1000) - timestamp) : null;
      setNetwork({
        status: health?.status === "ok" && Number.isFinite(blockAge) && blockAge <= 90 ? "live" : "degraded",
        height: health?.height ?? latest?.block_number ?? null,
        baseFee: fee?.base_fee ?? fee?.baseFee ?? null,
        blockAge,
      });
    };
    load();
    const timer = window.setInterval(load, 10000);
    return () => {
      active = false;
      window.clearInterval(timer);
    };
  }, []);

  useEffect(() => {
    setMobileOpen(false);
    setMoreOpen(false);
  }, [location.pathname]);

  useEffect(() => {
    if (!mobileOpen) return undefined;

    const closeMenu = (event) => {
      if (event.key === "Escape") {
        setMobileOpen(false);
        setMoreOpen(false);
      }
    };

    window.addEventListener("keydown", closeMenu);
    return () => window.removeEventListener("keydown", closeMenu);
  }, [mobileOpen]);

  const isActive = (to) => to === "/" ? location.pathname === "/" : location.pathname.startsWith(to);

  const routeSearch = async (event) => {
    event.preventDefault();
    const value = query.trim();
    if (!value) return;
    try {
      const result = await fetchChainJSON(`/v2/index/search?q=${encodeURIComponent(value)}`, { timeoutMs: 7000 });
      if (result?.type === "address") navigate(`/address/${result.query || value}`);
      else if (result?.type === "transaction") navigate(`/tx/${result.transaction?.hash || value}`);
      else if (result?.type === "block") navigate(`/blocks/${result.block?.hash || result.block?.number || value}`);
      else if (/^\d+$/.test(value)) navigate(`/blocks/${value}`);
      else if (/^0x[a-fA-F0-9]{40}$/.test(value)) navigate(`/address/${value}`);
      else navigate(`/tx/${value}`);
    } catch {
      if (/^\d+$/.test(value)) navigate(`/blocks/${value}`);
      else if (/^0x[a-fA-F0-9]{40}$/.test(value)) navigate(`/address/${value}`);
      else navigate(`/tx/${value}`);
    }
    setQuery("");
  };

  return (
    <header className="clean-header">
      <div className="clean-network-bar" aria-label="Network status">
        <div className="clean-network-inner">
          <span className={`clean-network-state ${network.status}`}>
            <i aria-hidden="true" />
            {network.status === "live" ? "PoDL testnet operational" : "Finality delayed"}
          </span>
          <div className="clean-network-metrics">
            <span>Height <strong>#{formatNumber(network.height)}</strong></span>
            <span>Last block <strong>{formatAge(network.blockAge)} ago</strong></span>
            <span>Base fee <strong>{network.baseFee ?? "—"}</strong></span>
            <span>PoDL v2</span>
          </div>
        </div>
      </div>

      <nav className="clean-nav" aria-label="Primary navigation">
        <Link className="clean-brand" to="/" aria-label="PoDL Explorer home">
          <span className="clean-brand-mark" aria-hidden="true"><i /></span>
          <span><strong>PoDL</strong><small>Explorer</small></span>
        </Link>

        <form className="clean-nav-search" onSubmit={routeSearch} role="search">
          <span aria-hidden="true" />
          <label className="sr-only" htmlFor="header-chain-search">Search the blockchain</label>
          <input
            id="header-chain-search"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Search block, transaction or address"
            autoComplete="off"
          />
          <kbd>/</kbd>
        </form>

        <div id="primary-navigation" className={`clean-nav-links${mobileOpen ? " open" : ""}`}>
          {PRIMARY_LINKS.map((item) => (
            <Link key={item.to} to={item.to} className={isActive(item.to) ? "active" : ""}>{item.label}</Link>
          ))}
          <div className={`clean-more${moreOpen ? " open" : ""}`}>
            <button
              type="button"
              className="clean-more-trigger"
              onClick={() => setMoreOpen((value) => !value)}
              aria-haspopup="true"
              aria-expanded={moreOpen}
            >
              More <span aria-hidden="true">⌄</span>
            </button>
            <div className="clean-more-menu">
              {MORE_LINKS.map((item) => <Link key={item.to} to={item.to}>{item.label}</Link>)}
            </div>
          </div>
          <Link className="clean-wallet-link" to="/wallet">Wallet <span aria-hidden="true">↗</span></Link>
        </div>

        <button
          className={`clean-menu-button${mobileOpen ? " open" : ""}`}
          type="button"
          onClick={() => {
            setMobileOpen((value) => !value);
            if (mobileOpen) setMoreOpen(false);
          }}
          aria-label={mobileOpen ? "Close navigation" : "Open navigation"}
          aria-expanded={mobileOpen}
          aria-controls="primary-navigation"
        >
          <span /><span />
        </button>
      </nav>
    </header>
  );
};

export default Navbar;
