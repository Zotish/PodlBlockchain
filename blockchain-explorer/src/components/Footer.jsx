import React from "react";
import { Link } from "react-router-dom";
import { CHAIN_BASE } from "../utils/api";

const Footer = () => (
  <footer className="clean-footer">
    <div>
      <Link className="clean-footer-brand" to="/">
        <span className="clean-brand-mark" aria-hidden="true"><i /></span>
        <strong>PoDL Explorer</strong>
      </Link>
      <nav aria-label="Footer navigation">
        <Link to="/blocks">Blocks</Link>
        <Link to="/transactions">Transactions</Link>
        <Link to="/validators">Validators</Link>
        <Link to="/developers/api">API</Link>
        <Link to="/wallet">Wallet</Link>
      </nav>
      <span>Public testnet · <a href={`${CHAIN_BASE}/health`} target="_blank" rel="noreferrer">Network status</a></span>
    </div>
    <div><span>© {new Date().getFullYear()} PoDL Network</span><span>Proof of Dynamic Liquidity</span></div>
  </footer>
);

export default Footer;
