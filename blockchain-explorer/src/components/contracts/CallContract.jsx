// // src/components/contract/ContractList.jsx
// import React, { useEffect, useState } from "react";

// const NODE = "http://127.0.0.1:5000";

// const ContractList = ({ onSelect }) => {
//   const [contracts, setContracts] = useState([]);

//   useEffect(() => {
//     fetch(`${NODE}/contract/list`)
//       .then((r) => r.json())
//       .then((d) => setContracts(d));
//   }, []);

//   return (
//     <div className="contract-panel">
//       <h2>📚 Deployed Contracts</h2>

//       {contracts.map((c) => (
//         <div
//           key={c.address}
//           className="contract-entry"
//           onClick={() => onSelect(c.address)}
//         >
//           <strong>{c.address}</strong>
//           <div>Type: {c.type}</div>
//         </div>
//       ))}
//     </div>
//   );
// };

// export default ContractList;



// src/components/contract/CallContract.js
import React, { useState } from "react";
import { parseLQD, isAmountParam } from "../../utils/lqdUnits";
import { API_BASE, apiUrl } from "../../utils/api";

const NODE = API_BASE;

export default function CallContract({ walletAddress, privateKey }) {
  const [contractAddress, setContractAddress] = useState("");
  const [abi, setABI] = useState([]);
  const [selectedFn, setSelectedFn] = useState(null);
  const [fnInputs, setFnInputs] = useState({});
  const [result, setResult] = useState(null);
  const [txHash, setTxHash] = useState(null);
  const [events, setEvents] = useState([]);

  // Load ABI from backend
  const loadABI = async () => {
    const res = await fetch(apiUrl(NODE, `/contract/getAbi?address=${contractAddress}`));
    const data = await res.json();
    const entries = Array.isArray(data) ? data : data?.entries;
    if (Array.isArray(entries)) {
      setABI(entries);
    } else {
      alert("Invalid ABI or contract not found");
    }
  };

  const handleSelectFn = (fn) => {
    setSelectedFn(fn);
    setFnInputs({});
    setResult(null);
    setTxHash(null);
    setEvents([]);
  };

  const handleInputChange = (key, value, inputType = "") => {
    // Guard: numeric types — max 39 digits
    if (inputType && (inputType.startsWith("uint") || inputType.startsWith("int"))) {
      const digits = value.replace(/^-/, "").replace(/^0+/, "") || "0";
      if (digits.length > 39) return; // silently block overflow
    }
    setFnInputs((prev) => ({ ...prev, [key]: value }));
  };

  const isReadOnlyFn = (fn) => {
    if (!fn) return false;
    if (fn.stateMutability) {
      const m = fn.stateMutability.toLowerCase();
      return m === "view" || m === "pure";
    }
    // If no mutability info, treat common getters as read-only
    const n = (fn.name || "").toLowerCase();
    return (
      n === "balanceof" ||
      n === "totalsupply" ||
      n === "symbol" ||
      n === "decimals" ||
      n === "name"
    );
  };

  const callFunction = async () => {
    if (!selectedFn) return;
    const readOnly = isReadOnlyFn(selectedFn);
    if (!readOnly && !privateKey) {
      alert("Private key is required to send contract transactions");
      return;
    }

    const args = (selectedFn.inputs || []).map((i, idx) => {
      const key = i.name || `${selectedFn.name}_${idx}`;
      const val = fnInputs[key] || "";
      // If it's an amount-type uint field AND contains a decimal point → treat as human-readable
      if (isAmountParam(i) && val.includes(".")) {
        try { return parseLQD(val); } catch { return val; }
      }
      return val;
    });
    let res;
    if (readOnly) {
      const body = {
        address: contractAddress,
        fn: selectedFn.name,
        args,
        caller: walletAddress,
        value: 0,
      };
      res = await fetch(apiUrl(NODE, "/contract/call"), {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
    } else {
      const body = {
        address: walletAddress,
        contract_address: contractAddress,
        function: selectedFn.name,
        args,
        private_key: privateKey || "",
      };
      res = await fetch(apiUrl(NODE, "/wallet/contract-template"), {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
    }

    const txt = await res.text();
    let out;

    try {
      out = JSON.parse(txt);
    } catch {
      out = txt;
    }

    // read-only call returns output immediately; write returns tx hash
    const display =
      out?.output !== undefined ? out.output : out?.result !== undefined ? out.result : out;
    setResult(display);
    setTxHash(out.txHash || out.tx_hash || null);

    if (out.events) setEvents(out.events);
  };

  return (
    <div className="contract-panel">
      <h2>🔧 Call Contract</h2>

      <input
        type="text"
        placeholder="Contract Address"
        value={contractAddress}
        onChange={(e) => setContractAddress(e.target.value)}
      />

      <button className="fn-btn" onClick={loadABI}>
        Load ABI
      </button>

      {abi.length > 0 && (
        <div className="abi-functions">
          <h3>Select Function</h3>

          {abi
            .filter((fn) => fn.type === "function")
            .map((fn) => (
              <button
                key={fn.name}
                className="fn-btn"
                onClick={() => handleSelectFn(fn)}
              >
                {fn.name}
              </button>
            ))}
        </div>
      )}

      {selectedFn && (
        <div className="fn-call-box">
          <h3>Function: {selectedFn.name}</h3>

          {selectedFn.inputs.map((input, idx) => {
            const key = input.name || `${selectedFn.name}_${idx}`;
            const isNumeric = input.type && (input.type.startsWith("uint") || input.type.startsWith("int"));
            return (
              <input
                key={key}
                type={isNumeric ? "number" : "text"}
                placeholder={`${input.name} (${input.type})${isAmountParam(input) ? " — e.g. 1.5 or raw" : ""}`}
                value={fnInputs[key] || ""}
                min={isNumeric ? "0" : undefined}
                onChange={(e) => handleInputChange(key, e.target.value, input.type)}
              />
            );
          })}

          <button className="fn-btn" onClick={callFunction}>
            Call Function
          </button>
        </div>
      )}

      {txHash && (
        <div className="fn-output">
          <h3>Transaction Hash</h3>
          <code>{txHash}</code>
        </div>
      )}

      {result && (
        <div className="fn-output">
          <h3>Result</h3>
          <pre>{JSON.stringify(result, null, 2)}</pre>
        </div>
      )}

      {events.length > 0 && (
        <div className="fn-output">
          <h3>Events</h3>
          <pre>{JSON.stringify(events, null, 2)}</pre>
        </div>
      )}
    </div>
  );
}
