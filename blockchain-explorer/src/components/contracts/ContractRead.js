import React, { useState } from "react";

const API = "http://127.0.0.1:9000";

export default function ContractRead({ address, abi }) {
  const reads = abi.filter((fn) => fn.type === "function" && fn.stateMutability === "view");

  const callRead = async (fn) => {
    const res = await fetch(`${API}/contract/call?address=${address}&fn=${fn.name}`);
    const data = await res.json();
    alert(JSON.stringify(data, null, 2));
  };

  return (
    <div>
      <h3>📘 Read Functions</h3>
      {reads.map((fn) => (
        <button key={fn.name} className="fn-btn" onClick={() => callRead(fn)}>
          {fn.name}
        </button>
      ))}
    </div>
  );
}
