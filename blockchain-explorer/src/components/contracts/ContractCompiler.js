import React, { useState } from "react";

const API = "http://127.0.0.1:9000";

export default function ContractCompiler({ onCompiled }) {
  const [source, setSource] = useState("");
  const [type, setType] = useState("solidity");
  const [output, setOutput] = useState(null);

  const compile = async () => {
    const res = await fetch(`${API}/contract/compile`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ type, source }),
    });

    const out = await res.json();
    setOutput(out);
    onCompiled && onCompiled(out);
  };

  return (
    <div className="contract-panel">
      <h3>🛠 Smart Contract Compiler</h3>

      <select value={type} onChange={(e) => setType(e.target.value)}>
        <option value="solidity">Solidity</option>
        <option value="gocode">Go-Contract</option>
        <option value="dsl">DSL</option>
      </select>

      <textarea
        className="code-input"
        placeholder="Write your smart contract code here..."
        value={source}
        onChange={(e) => setSource(e.target.value)}
      />

      <button onClick={compile} className="fn-btn">Compile</button>

      {output && (
        <div className="code-block">
          <pre>{JSON.stringify(output, null, 2)}</pre>
        </div>
      )}
    </div>
  );
}
