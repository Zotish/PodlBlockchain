// src/components/contracts/ContractOverview.js
import React, { useEffect, useState } from "react";
import { API_BASE, apiUrl } from "../../utils/api";

const API = API_BASE;

export default function ContractOverview({ address, setABI }) {
  const [info, setInfo] = useState(null);

  useEffect(() => {
    fetch(apiUrl(API, `/contract/getAbi?address=${address}`))
      .then((r) => r.json())
      .then((raw) => {
        // Server returns { entries: [...] } — extract the array
        const entries = Array.isArray(raw) ? raw : (raw.entries || []);
        setABI(entries);
        setInfo({ abi: entries, type: "smart-contract" });
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
