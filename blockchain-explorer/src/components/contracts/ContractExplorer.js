// src/components/contracts/ContractExplorer.js
import React, { useState, useEffect } from "react";
import ContractOverview from "./ContractOverview";
import ContractRead from "./ContractRead";
import ContractWrite from "./ContractWrite";
import ContractEvents from "./ContractEvents";
import ContractStorage from "./ContractStorage";
import ContractABI from "./ContractABI";
import ContractTxHistory from "./ContractTxHistory";
import ContractCode from "./ContractCode";
import "./contract.css";

const tabs = ["overview", "read", "write", "events", "storage", "abi", "code", "tx"];

export default function ContractExplorer({ address, walletAddress, privateKey }) {
  const [active, setActive] = useState("overview");
  const [abi, setABI] = useState([]);

  return (
    <div className="contract-explorer">
      <h2>🔍 Contract Explorer</h2>
      <h3>{address}</h3>

      <div className="explorer-tabs">
        {tabs.map((t) => (
          <button
            key={t}
            className={`tab ${active === t ? "active" : ""}`}
            onClick={() => setActive(t)}
          >
            {t.toUpperCase()}
          </button>
        ))}
      </div>

      {active === "overview" && <ContractOverview address={address} setABI={setABI} />}
      {active === "read" && <ContractRead address={address} abi={abi} />}
      {active === "write" && <ContractWrite address={address} walletAddress={walletAddress} abi={abi} privateKey={privateKey} />}
      {active === "events" && <ContractEvents address={address} />}
      {active === "storage" && <ContractStorage address={address} />}
      {active === "code" && <ContractCode address={address} />}

      {active === "abi" && <ContractABI address={address} />}
      {active === "tx" && <ContractTxHistory address={address} />}
    </div>
  );
}
