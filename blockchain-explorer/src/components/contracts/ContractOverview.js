// src/components/contracts/ContractOverview.js
import React, { useEffect, useState } from "react";

const API = "http://127.0.0.1:9000";

export default function ContractOverview({ address, setABI }) {
  const [info, setInfo] = useState(null);

  useEffect(() => {
    fetch(`${API}/contract/getAbi?address=${address}`)
      .then((r) => r.json())
      .then((abi) => {
        setABI(abi);
        setInfo({ abi, type: "smart-contract" });
      });
  }, [address]);

  if (!info) return <p>Loading...</p>;

  return (
    <div>
      <h3>📜 Contract Overview</h3>

      <div><strong>Address:</strong> {address}</div>
      <div><strong>Type:</strong> {info.type}</div>
      <div><strong>ABI Functions:</strong> {info.abi.length}</div>
    </div>
  );
}
