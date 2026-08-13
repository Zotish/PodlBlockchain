import React, { useState } from "react";
import { parseLQD, isAmountParam } from "../../utils/lqdUnits";
import { API_BASE, apiUrl } from "../../utils/api";

const API = API_BASE;

export default function ContractWrite({ address, walletAddress, abi, privateKey }) {
  // Show functions that have inputs (state-changing functions)
  const writes = Array.isArray(abi)
    ? abi.filter((fn) => fn.type === "function" && (fn.inputs || []).length > 0)
    : [];

  const [inputs, setInputs] = useState({});
  const [selectedFn, setSelectedFn] = useState(null);
  const [status, setStatus] = useState("");

  // ABI inputs are ["string","string"] type-string arrays, not {name,type} objects
  const handleInput = (idx, value) =>
    setInputs((prev) => ({ ...prev, [idx]: value }));

  const sendWrite = async () => {
    if (!privateKey) {
      alert("Private key is required to send contract transactions");
      return;
    }
    const args = (selectedFn.inputs || []).map((type, i) => {
      const v = inputs[i] || "";
      // Auto-convert decimal amounts for string params that look like amounts
      if (type === "string" && isAmountParam({ name: `arg${i}`, type }) && v.includes(".")) {
        try { return parseLQD(v); } catch { return v; }
      }
      return v;
    });

    const body = {
      address: walletAddress,
      contract_address: address,
      function: selectedFn.name,
      args,
      private_key: privateKey || "",
    };

    setStatus("⏳ Sending...");
    try {
      const res = await fetch(apiUrl(API, "/wallet/contract-template"), {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
      const text = await res.text();
      setStatus(res.ok ? "✅ " + text : "❌ " + text);
    } catch (e) {
      setStatus("❌ " + e.message);
    }
  };

  return (
    <div>
      <h3>✍️ Write Functions</h3>

      <div style={{ display: "flex", flexWrap: "wrap", gap: 6, marginBottom: 12 }}>
        {writes.map((fn) => (
          <button
            key={fn.name}
            className="fn-btn"
            style={{ background: selectedFn?.name === fn.name ? "#2563eb" : undefined, color: selectedFn?.name === fn.name ? "#fff" : undefined }}
            onClick={() => { setSelectedFn(fn); setInputs({}); setStatus(""); }}
          >
            {fn.name}
          </button>
        ))}
      </div>

      {selectedFn && (
        <div style={{ background: "#f9fafb", border: "1px solid #e5e7eb", borderRadius: 8, padding: 16 }}>
          <h4 style={{ marginTop: 0 }}>{selectedFn.name}({(selectedFn.inputs || []).join(", ")})</h4>

          {(selectedFn.inputs || []).map((type, i) => (
            <div key={i} style={{ marginBottom: 8 }}>
              <input
                placeholder={`arg${i + 1} (${type})`}
                style={{ width: "100%", padding: "6px 10px", borderRadius: 6, border: "1px solid #d1d5db", boxSizing: "border-box" }}
                onChange={(e) => handleInput(i, e.target.value)}
              />
            </div>
          ))}

          <button className="fn-btn" onClick={sendWrite} style={{ background: "#16a34a", color: "#fff" }}>
            Execute
          </button>

          {status && (
            <div style={{ marginTop: 8, padding: "6px 10px", background: status.startsWith("✅") ? "#f0fdf4" : "#fef2f2", borderRadius: 4, fontSize: 13 }}>
              {status}
            </div>
          )}
        </div>
      )}

      {writes.length === 0 && <p style={{ color: "#6b7280" }}>No write functions found. Load a contract first.</p>}
    </div>
  );
}
