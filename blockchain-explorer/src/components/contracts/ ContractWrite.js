import React, { useState } from "react";

const API = "http://127.0.0.1:9000";

export default function ContractWrite({ address, walletAddress, abi, privateKey }) {
  const writes = abi.filter((fn) => fn.type === "function" && fn.stateMutability !== "view");

  const [inputs, setInputs] = useState({});
  const [selectedFn, setSelectedFn] = useState(null);

  const handleInput = (name, value) =>
    setInputs((prev) => ({ ...prev, [name]: value }));

  const sendWrite = async () => {
    if (!privateKey) {
      alert("Private key is required to send contract transactions");
      return;
    }
    const body = {
      address: walletAddress,
      contract_address: address,
      function: selectedFn.name,
      args: selectedFn.inputs.map((i) => inputs[i.name] || ""),
      private_key: privateKey || "",
    };

    const res = await fetch(`${API}/wallet/contract-template`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });

    alert(await res.text());
  };

  return (
    <div>
      <h3>✍️ Write Functions</h3>

      {writes.map((fn) => (
        <button
          key={fn.name}
          className="fn-btn"
          onClick={() => {
            setSelectedFn(fn);
            setInputs({});
          }}
        >
          {fn.name}
        </button>
      ))}

      {selectedFn && (
        <div>
          <h4>{selectedFn.name}</h4>

          {selectedFn.inputs.map((input) => (
            <input
              key={input.name}
              placeholder={`${input.name} (${input.type})`}
              onChange={(e) => handleInput(input.name, e.target.value)}
            />
          ))}

          <button className="fn-btn" onClick={sendWrite}>
            Execute
          </button>
        </div>
      )}
    </div>
  );
}
