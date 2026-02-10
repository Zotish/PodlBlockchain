// src/components/contract/ContractDeploy.jsx
import React, { useState } from "react";
import { parseLQD } from "../../utils/lqdUnits";

const NODE = "http://127.0.0.1:9000";

const ContractDeploy = ({ walletAddress, privateKey }) => {
  const [contractType, setContractType] = useState("plugin");
  const [file, setFile] = useState(null);
  const [owner, setOwner] = useState(walletAddress);
  const [pk, setPk] = useState(privateKey || "");
  const [gasPriceLQD, setGasPriceLQD] = useState("");
  const [gasLimit, setGasLimit] = useState("50000");
  const [deploying, setDeploying] = useState(false);
  const [log, setLog] = useState("");

  const deployContract = async () => {
    if (!file) return alert("Select a contract file");
    if (!pk) return alert("Private key is required to pay deploy fees");

    const formData = new FormData();
    formData.append("contract_file", file);
    formData.append("type", contractType);
    formData.append("owner", owner);
    formData.append("private_key", pk);
    if (gasPriceLQD.trim() !== "") {
      formData.append("gas_price", parseLQD(gasPriceLQD));
    }
    formData.append("gas", gasLimit);

    setDeploying(true);
    setLog("📦 Uploading...");

    try {
      const res = await fetch(`${NODE}/contract/deploy`, {
        method: "POST",
        body: formData,
      });

      const text = await res.text();
      let data = null;
      try {
        data = JSON.parse(text);
      } catch {
        data = null;
      }
      if (!res.ok) {
        const msg = data?.error || text || "deploy failed";
        throw new Error(msg);
      }

      setLog(
        `✅ DEPLOYED\nAddress: ${data.address}\nType: ${data.type}\nOwner: ${data.owner}`
      );
    } catch (err) {
      setLog("❌ ERROR: " + err.message);
    } finally {
      setDeploying(false);
    }
  };

  return (
    <div className="contract-panel">
      <h2>📦 Deploy Contract</h2>

      <label>Contract Type</label>
      <select
        value={contractType}
        onChange={(e) => setContractType(e.target.value)}
      >
        <option value="plugin">Go Plugin (.so)</option>
        <option value="gocode">Go-Bytecode (.lqd)</option>
        <option value="dsl">DSL Script (.dsl)</option>
      </select>

      <label>Contract File</label>
      <input
        type="file"
        onChange={(e) => setFile(e.target.files[0])}
        accept=".so,.dsl,.lqd"
      />

      <label>Owner Address</label>
      <input
        type="text"
        value={owner}
        onChange={(e) => setOwner(e.target.value)}
        placeholder="0x..."
      />
      <label>Owner Private Key</label>
      <input
        type="password"
        value={pk}
        onChange={(e) => setPk(e.target.value)}
        placeholder="0x..."
      />
      <label>Gas Price (LQD)</label>
      <input
        type="text"
        value={gasPriceLQD}
        onChange={(e) => setGasPriceLQD(e.target.value)}
        placeholder="auto (basefee+1)"
      />
      <label>Gas Limit</label>
      <input
        type="number"
        value={gasLimit}
        onChange={(e) => setGasLimit(e.target.value)}
        placeholder="50000"
      />

      <button className="btn-primary" onClick={deployContract} disabled={deploying}>
        {deploying ? "Deploying..." : "Deploy Contract"}
      </button>

      <pre className="console-output">{log}</pre>
    </div>
  );
};

export default ContractDeploy;
