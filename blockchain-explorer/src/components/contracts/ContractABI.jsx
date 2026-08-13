import React, { useEffect, useState } from "react";
import { API_BASE, CHAIN_BASE, WALLET_BASE } from "../../utils/api";

const NODE = CHAIN_BASE;

export default function ContractABI({ address }) {
  const [entries, setEntries] = useState(null);
  const [copied, setCopied]   = useState("");
  const [view,   setView]     = useState("json"); // "json" | "js" | "table"

  useEffect(() => {
    fetch(`${NODE}/contract/getAbi?address=${address}`)
      .then((r) => r.json())
      .then((raw) => {
        const list = Array.isArray(raw) ? raw : (raw.entries || []);
        setEntries(list);
      })
      .catch(() => setEntries([]));
  }, [address]);

  if (entries === null) return <p>Loading ABI…</p>;
  if (!entries.length)  return <p style={{ color: "#6b7280" }}>No ABI found for this contract.</p>;

  /* ─── JSON (copy-paste ready) ───────────────────────────────────── */
  const jsonText = JSON.stringify(entries, null, 2);

  /* ─── JavaScript DApp helper ────────────────────────────────────── */
  const jsText =
`// ── LQD Contract Client ──────────────────────────────────────────
// Contract: ${address}
// Copy this file into your DApp project.

const CONTRACT_ADDRESS = "${address}";
const NODE_URL   = "${CHAIN_BASE}"; // chain node
const WALLET_URL = "${WALLET_BASE}"; // wallet server

// ABI (full list of functions)
export const ABI = ${JSON.stringify(entries, null, 2)};

// ── Read (no transaction needed) ────────────────────────────────
// Usage: const name = await callRead("Name", []);
//        const bal  = await callRead("BalanceOf", ["0xYourAddress"]);
export async function callRead(fn, args = [], callerAddress = "") {
  const res = await fetch(\`\${NODE_URL}/contract/call\`, {
    method:  "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      address: CONTRACT_ADDRESS,
      fn,
      args,
      caller: callerAddress,
    }),
  });
  if (!res.ok) throw new Error(await res.text());
  const data = await res.json();
  if (!data.success) throw new Error(data.error || "call failed");
  return data.output; // string
}

// ── Write (sends a transaction) ─────────────────────────────────
// Usage: await callWrite("Transfer", ["0xTo", "1000000"], "0xYour", "yourPrivKey");
export async function callWrite(fn, args = [], fromAddress, privateKey) {
  const res = await fetch(\`\${WALLET_URL}/wallet/contract-template\`, {
    method:  "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      address:          fromAddress,
      private_key:      privateKey,
      contract_address: CONTRACT_ADDRESS,
      function:         fn,
      args,
    }),
  });
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

// ── Generated wrappers ───────────────────────────────────────────
${entries.map((fn) => {
  const params  = (fn.inputs || []).map((t, i) => `arg${i + 1}`).join(", ");
  const hasArgs = (fn.inputs || []).length > 0;
  const isWrite = hasArgs; // heuristic: functions with args are writes
  if (!isWrite) {
    return `export const ${fn.name} = (caller = "") => callRead("${fn.name}", [], caller);`;
  }
  return `export const ${fn.name} = (${params}, from, pk) => callWrite("${fn.name}", [${params}], from, pk);`;
}).join("\n")}
`;

  const copy = (text, label) => {
    navigator.clipboard.writeText(text).catch(() => {
      // fallback for non-HTTPS
      const ta = document.createElement("textarea");
      ta.value = text;
      document.body.appendChild(ta);
      ta.select();
      document.execCommand("copy");
      document.body.removeChild(ta);
    });
    setCopied(label);
    setTimeout(() => setCopied(""), 2500);
  };

  /* ─── styles ────────────────────────────────────────────────────── */
  const tabBtn = (id) => ({
    padding: "5px 16px", borderRadius: 6, cursor: "pointer", fontSize: 13,
    fontWeight: 500, border: "1px solid #d1d5db",
    background: view === id ? "#2563eb" : "#f9fafb",
    color:      view === id ? "#fff"    : "#374151",
  });

  const code = {
    background: "#1e1e2e", color: "#cdd6f4",
    fontFamily: "monospace", fontSize: 12,
    padding: 16, borderRadius: 8,
    whiteSpace: "pre", overflowX: "auto",
    maxHeight: 500, overflowY: "auto",
    lineHeight: 1.65,
  };

  const copyBtn = (label, text) => (
    <button
      onClick={() => copy(text, label)}
      style={{
        padding: "5px 14px", borderRadius: 6, border: "1px solid #d1d5db",
        background: copied === label ? "#dcfce7" : "#f9fafb",
        color: copied === label ? "#166534" : "#374151",
        cursor: "pointer", fontSize: 12, fontWeight: 500,
      }}
    >
      {copied === label ? "✅ Copied!" : "📋 Copy"}
    </button>
  );

  return (
    <div>
      {/* Header */}
      <div style={{ display:"flex", alignItems:"center", justifyContent:"space-between", marginBottom:14 }}>
        <h3 style={{ margin:0 }}>📋 Contract ABI</h3>
        <span style={{ fontSize:12, color:"#6b7280", fontFamily:"monospace" }}>
          {entries.length} function{entries.length !== 1 ? "s" : ""}
        </span>
      </div>

      {/* Tab switcher */}
      <div style={{ display:"flex", gap:6, marginBottom:14 }}>
        <button style={tabBtn("json")}  onClick={() => setView("json")}>JSON ABI</button>
        <button style={tabBtn("js")}    onClick={() => setView("js")}>JS Client</button>
        <button style={tabBtn("table")} onClick={() => setView("table")}>Functions</button>
      </div>

      {/* ── JSON ABI ── */}
      {view === "json" && (
        <>
          <div style={{ display:"flex", justifyContent:"flex-end", marginBottom:6 }}>
            {copyBtn("json", jsonText)}
          </div>
          <div style={code}>{jsonText}</div>
          <div style={{ marginTop:8, padding:"10px 14px", background:"#eff6ff",
              border:"1px solid #bfdbfe", borderRadius:8, fontSize:12, color:"#1e40af" }}>
            <strong>Usage in your DApp:</strong><br/>
            <code>POST {API_BASE}/contract/call</code><br/>
            <code>{`{ "address": "${address}", "fn": "FunctionName", "args": [...], "caller": "0x..." }`}</code>
          </div>
        </>
      )}

      {/* ── JavaScript Client ── */}
      {view === "js" && (
        <>
          <div style={{ display:"flex", justifyContent:"space-between", alignItems:"center", marginBottom:6 }}>
            <span style={{ fontSize:12, color:"#6b7280" }}>
              Ready-to-use JS helper — paste into your React / Next.js / Node project
            </span>
            {copyBtn("js", jsText)}
          </div>
          <div style={code}>{jsText}</div>
        </>
      )}

      {/* ── Function Table ── */}
      {view === "table" && (
        <table style={{ width:"100%", borderCollapse:"collapse", fontSize:13 }}>
          <thead>
            <tr style={{ background:"#f3f4f6" }}>
              <th style={{ padding:"8px 12px", textAlign:"left" }}>Function</th>
              <th style={{ padding:"8px 12px", textAlign:"left" }}>Parameters</th>
              <th style={{ padding:"8px 12px", textAlign:"left" }}>Kind</th>
            </tr>
          </thead>
          <tbody>
            {entries.map((fn, i) => (
              <tr key={fn.name+i} style={{ borderBottom:"1px solid #f3f4f6" }}>
                <td style={{ padding:"8px 12px", fontWeight:600, color:"#2563eb", fontFamily:"monospace" }}>
                  {fn.name}
                </td>
                <td style={{ padding:"8px 12px", fontFamily:"monospace", color:"#6b7280" }}>
                  {(fn.inputs||[]).length === 0
                    ? <em style={{ color:"#d1d5db" }}>none</em>
                    : (fn.inputs||[]).map((t,j) => (
                        <span key={j}>
                          <span style={{ color:"#a78bfa" }}>{t}</span>
                          {j < fn.inputs.length-1 ? ", " : ""}
                        </span>
                      ))}
                </td>
                <td style={{ padding:"8px 12px" }}>
                  <span style={{
                    padding:"2px 8px", borderRadius:4, fontSize:11,
                    background: (fn.inputs||[]).length===0 ? "#f0fdf4" : "#eff6ff",
                    color:      (fn.inputs||[]).length===0 ? "#166534"  : "#1e40af",
                  }}>
                    {(fn.inputs||[]).length===0 ? "read" : "write"}
                  </span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
