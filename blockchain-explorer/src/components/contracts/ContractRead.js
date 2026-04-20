import React, { useState } from "react";
import { API_BASE, apiUrl } from "../../utils/api";

const API = API_BASE;

export default function ContractRead({ address, abi }) {
  // Show all non-payable functions (our ABI has no stateMutability; payable:false = read/write)
  // Treat functions with 0 inputs as pure reads; those with inputs allow arg entry
  const [inputs, setInputs] = useState({});
  const [results, setResults] = useState({});
  const [selectedFn, setSelectedFn] = useState(null);

  const reads = Array.isArray(abi)
    ? abi.filter((fn) => fn.type === "function")
    : [];

  const callRead = async (fn, argValues) => {
    const args = (fn.inputs || []).map((_, i) => argValues[i] || "");
    try {
      const res = await fetch(apiUrl(API, "/contract/call"), {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ address, fn: fn.name, args, caller: "" }),
      });
      const data = await res.json();
      setResults((prev) => ({ ...prev, [fn.name]: data.output ?? JSON.stringify(data) }));
    } catch (e) {
      setResults((prev) => ({ ...prev, [fn.name]: "Error: " + e.message }));
    }
  };

  return (
    <div>
      <h3>📘 Read Functions</h3>
      {reads.map((fn) => (
        <div key={fn.name} style={{ marginBottom: 12 }}>
          <div style={{ fontWeight: 600, marginBottom: 4 }}>{fn.name}({(fn.inputs || []).join(", ")})</div>

          {(fn.inputs || []).map((type, i) => (
            <input
              key={i}
              placeholder={`arg${i + 1} (${type})`}
              style={{ marginRight: 6, padding: "4px 8px", borderRadius: 4, border: "1px solid #d1d5db" }}
              onChange={(e) => setInputs((prev) => ({ ...prev, [`${fn.name}_${i}`]: e.target.value }))}
            />
          ))}

          <button
            className="fn-btn"
            onClick={() => callRead(fn, (fn.inputs || []).map((_, i) => inputs[`${fn.name}_${i}`] || ""))}
          >
            Call
          </button>

          {results[fn.name] !== undefined && (
            <div style={{ marginTop: 4, padding: "6px 10px", background: "#f0fdf4", borderRadius: 4, fontFamily: "monospace", fontSize: 13 }}>
              → {String(results[fn.name])}
            </div>
          )}
        </div>
      ))}
      {reads.length === 0 && <p style={{ color: "#6b7280" }}>No functions found. Load a contract first.</p>}
    </div>
  );
}
