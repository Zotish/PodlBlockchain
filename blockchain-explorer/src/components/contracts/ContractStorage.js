import React, { useEffect, useState } from "react";

const API = "http://127.0.0.1:9000";

export default function ContractStorage({ address }) {
  const [storage, setStorage] = useState(null);

  useEffect(() => {
    fetch(`${API}/contract/storage?address=${address}`)
      .then((res) => res.json())
      .then((data) => setStorage(data));
  }, [address]);

  if (!storage) return <p>Loading storage...</p>;

  return (
    <div className="contract-panel">
      <h3>🧠 Contract Storage</h3>

      {Object.keys(storage).length === 0 && (
        <p style={{ color: "#9ca3af" }}>This contract has no storage yet.</p>
      )}

      {Object.entries(storage).map(([key, val]) => (
        <div key={key} className="contract-entry">
          <strong>{key}</strong>
          <div>{JSON.stringify(val)}</div>
        </div>
      ))}
    </div>
  );
}
