// src/components/contract/ContractABI.jsx
import React, { useState } from "react";
import { fetchJSON } from "../../utils/api";

const ContractABI = ({ setSelectedFn }) => {
  const [address, setAddress] = useState("");
  const [abi, setABI] = useState(null);
  const [log, setLog] = useState("");

  const loadABI = async () => {
    try {
      const json = await fetchJSON(`/contract/getAbi?address=${address}`);
      const entries = Array.isArray(json) ? json : json?.entries;
      if (!Array.isArray(entries) || entries.length === 0) {
        throw new Error("Invalid ABI or contract not found");
      }
      setABI(entries);
      setLog("Loaded ABI");
    } catch (err) {
      setLog("Error: " + err.message);
    }
  };

  return (
    <div className="contract-panel">
      <h2>📄 Contract ABI</h2>

      <input
        value={address}
        onChange={(e) => setAddress(e.target.value)}
        placeholder="Contract address"
      />

      <button className="btn-primary" onClick={loadABI}>
        Load ABI
      </button>

      <pre className="console-output">{log}</pre>

      {Array.isArray(abi) && (
        <div className="abi-section">
          <h3>Functions</h3>
          {abi.map((fn) => (
            <button
              key={fn.name}
              className="fn-btn"
              onClick={() => setSelectedFn({ fn, address })}
            >
              {fn.name}({(fn.inputs || []).map((i) => i.name || i.type || i).join(", ")})
            </button>
          ))}

          <h3>Raw ABI JSON</h3>
          <pre className="abi-json">{JSON.stringify(abi, null, 2)}</pre>
        </div>
      )}
    </div>
  );
};

export default ContractABI;
