import React, { useState } from "react";
import { API_BASE, CHAIN_BASE } from "../../utils/api";

const NODE_CANDIDATES = [CHAIN_BASE, API_BASE];

const PLACEHOLDER = `package main

import (
    lqdctx "github.com/Zotish/lqd-sdk/context"
)

// Example: simple token contract
type MyToken struct{}

func (t *MyToken) Name(ctx *lqdctx.Context) {
    ctx.Set("output", "MyToken")
}

func (t *MyToken) Symbol(ctx *lqdctx.Context) {
    ctx.Set("output", "MTK")
}

func (t *MyToken) TotalSupply(ctx *lqdctx.Context) {
    ctx.Set("output", "1000000000000000")
}

func (t *MyToken) Decimals(ctx *lqdctx.Context) {
    ctx.Set("output", "8")
}

var Contract = &MyToken{}
`;

export default function ContractCompiler({ walletAddress, privateKey, onDeployed }) {
  const [source, setSource]         = useState("");
  const [type, setType]             = useState("goplugin");
  const [status, setStatus]         = useState("");
  const [isError, setIsError]       = useState(false);
  const [compiling, setCompiling]   = useState(false);
  const [deploying, setDeploying]   = useState(false);
  const [compiledBlob, setCompiledBlob] = useState(null);
  const [compiledType, setCompiledType] = useState("plugin");
  const [deployedAddr, setDeployedAddr] = useState("");

  const show = (msg, err = false) => { setStatus(msg); setIsError(err); };

  const postJson = async (path, payload, { timeoutMs = 120000 } = {}) => {
    let lastErr = null;
    for (const base of NODE_CANDIDATES) {
      const controller = new AbortController();
      const timer = setTimeout(() => controller.abort(), timeoutMs);
      try {
        const res = await fetch(`${base}${path}`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(payload),
          signal: controller.signal,
        });
        clearTimeout(timer);
        const data = await res.json().catch(() => ({}));
        if (!res.ok) {
          lastErr = new Error(data.error || `Request failed (${res.status})`);
          continue;
        }
        return data;
      } catch (e) {
        clearTimeout(timer);
        lastErr = e;
      }
    }
    throw lastErr || new Error("Request failed");
  };

  const buildDeployForm = () => {
    const form = new FormData();
    const fileName = compiledType === "plugin" ? "contract.so" : "contract.lqd";
    form.append("contract_file", compiledBlob, fileName);
    form.append("owner", walletAddress);
    form.append("private_key", privateKey);
    form.append("type", compiledType);
    form.append("gas", "500000");
    return form;
  };

  const compile = async () => {
    if (!source.trim()) { show("⚠️ Enter source code first", true); return; }
    setCompiling(true);
    setCompiledBlob(null);
    setDeployedAddr("");
    show("");

    try {
      if (type === "goplugin") {
        show("⏳ Building Go plugin… (may take ~10s)");
        const data = await postJson("/contract/compile-plugin", { source }, { timeoutMs: 180000 });
        if (!data.success) {
          show("❌ Compile error:\n\n" + (data.error || "unknown"), true);
          return;
        }
        // Decode base64 → Blob
        const binStr = atob(data.binary);
        const bytes  = new Uint8Array(binStr.length);
        for (let i = 0; i < binStr.length; i++) bytes[i] = binStr.charCodeAt(i);
        const blob = new Blob([bytes], { type: "application/octet-stream" });
        setCompiledBlob(blob);
        setCompiledType("plugin");
        show(`✅ Go Plugin compiled! Size: ${(data.size / 1024).toFixed(1)} KB — Ready to deploy`);
      } else {
        const data = await postJson("/contract/compile", { type, source });
        if (data.error) { show("❌ " + data.error, true); return; }
        const raw = JSON.stringify(data);
        const blob = new Blob([raw], { type: "application/octet-stream" });
        setCompiledBlob(blob);
        setCompiledType(type);
        show(`✅ Compiled! Type: ${type} — Ready to deploy`);
      }
    } catch (e) {
      show("❌ Error: " + e.message, true);
    } finally {
      setCompiling(false);
    }
  };

  const deploy = async () => {
    if (!compiledBlob) { show("⚠️ Compile first", true); return; }
    if (!walletAddress) { show("⚠️ Wallet not connected", true); return; }
    if (!privateKey)    { show("⚠️ Private key required", true); return; }

    setDeploying(true);
    show("📦 Deploying to chain…");
    try {
      let data = null;
      let lastErr = null;
      for (const base of NODE_CANDIDATES) {
        try {
          const res = await fetch(`${base}/contract/deploy`, { method: "POST", body: buildDeployForm() });
          data = await res.json().catch(() => ({}));
          if (!res.ok) throw new Error(data.error || "Deploy failed");
          lastErr = null;
          break;
        } catch (e) {
          lastErr = e;
        }
      }
      if (lastErr) throw lastErr;

      setDeployedAddr(data.address || "");
      show(`🎉 Deployed!\nAddress: ${data.address}\nType: ${compiledType}`);
      setCompiledBlob(null);
      onDeployed && onDeployed(data.address);
    } catch (e) {
      show("❌ Deploy failed: " + e.message, true);
    } finally {
      setDeploying(false);
    }
  };

  return (
    <div className="contract-panel">
      <h3>🛠 Smart Contract Compiler</h3>

      {/* Language selector */}
      <div style={{ marginBottom: 12 }}>
        <label style={{ fontSize: 13, fontWeight: 600, display: "block", marginBottom: 4 }}>
          Language
        </label>
        <select
          value={type}
          onChange={(e) => { setType(e.target.value); setCompiledBlob(null); show(""); }}
          style={{ width: "100%", padding: "8px 10px", borderRadius: 8, border: "1px solid #d1d5db", fontSize: 14 }}
        >
          <option value="goplugin">⭐ Go Plugin (.so) — Full DApp Power</option>
          <option value="gocode">Go Bytecode (limited opcodes)</option>
          <option value="dsl">DSL Script (simple key-value)</option>
          <option value="solidity">Solidity (needs solc installed)</option>
        </select>
      </div>

      {/* Hint for goplugin */}
      {type === "goplugin" && (
        <div style={{ background: "#eff6ff", border: "1px solid #bfdbfe", borderRadius: 8, padding: "10px 14px", fontSize: 13, color: "#1e40af", marginBottom: 10 }}>
          💡 Write any Go contract — token, DEX, NFT, staking, etc. Must start with <code>package main</code> and export <code>var Contract</code>.
        </div>
      )}

      {/* Code editor */}
      <div style={{ marginBottom: 12 }}>
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 4 }}>
          <label style={{ fontSize: 13, fontWeight: 600 }}>Source Code</label>
          {!source && (
            <button
              onClick={() => setSource(PLACEHOLDER)}
              style={{ fontSize: 11, padding: "2px 8px", borderRadius: 4, border: "1px solid #d1d5db", background: "#f9fafb", cursor: "pointer" }}
            >
              Load Example
            </button>
          )}
        </div>
        <textarea
          value={source}
          onChange={(e) => setSource(e.target.value)}
          placeholder={type === "goplugin" ? PLACEHOLDER : "Paste your contract source code here..."}
          rows={18}
          style={{
            width: "100%", fontFamily: "monospace", fontSize: 12,
            padding: 12, borderRadius: 8, border: "1px solid #d1d5db",
            background: "#1e1e2e", color: "#cdd6f4",
            resize: "vertical", boxSizing: "border-box"
          }}
        />
      </div>

      {/* Compile button */}
      <button
        onClick={compile}
        disabled={compiling}
        className="btn-primary"
        style={{ width: "100%", marginBottom: 8 }}
      >
        {compiling ? "⏳ Compiling…" : "⚙️ Compile"}
      </button>

      {/* Status */}
      {status && (
        <pre style={{
          background: isError ? "#fef2f2" : "#f0fdf4",
          border: `1px solid ${isError ? "#fecaca" : "#bbf7d0"}`,
          color: isError ? "#b91c1c" : "#166534",
          borderRadius: 8, padding: 12, fontSize: 12,
          whiteSpace: "pre-wrap", wordBreak: "break-word",
          marginBottom: compiledBlob ? 10 : 0
        }}>
          {status}
        </pre>
      )}

      {/* Deploy button — shown after successful compile */}
      {compiledBlob && !deployedAddr && (
        <button
          onClick={deploy}
          disabled={deploying}
          className="btn-primary"
          style={{ width: "100%", background: "#16a34a", marginTop: 4 }}
        >
          {deploying ? "📦 Deploying…" : "🚀 Deploy to Chain"}
        </button>
      )}

      {/* Deployed address */}
      {deployedAddr && (
        <div style={{ background: "#f0fdf4", border: "1px solid #bbf7d0", borderRadius: 8, padding: 12, marginTop: 8 }}>
          <div style={{ fontWeight: 600, color: "#166534", marginBottom: 4 }}>🎉 Contract Live!</div>
          <div style={{ fontFamily: "monospace", fontSize: 13, wordBreak: "break-all" }}>{deployedAddr}</div>
        </div>
      )}
    </div>
  );
}
