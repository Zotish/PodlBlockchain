import React, { useState, useEffect } from 'react';
import { fetchJSON, API_BASE } from '../utils/api';

const BridgePage = () => {
  const [from, setFrom] = useState('');
  const [privateKey, setPrivateKey] = useState('');
  const [toBsc, setToBsc] = useState('');
  const [amount, setAmount] = useState('');
  const [gasPrice, setGasPrice] = useState('10');
  const [requests, setRequests] = useState([]);
  const [status, setStatus] = useState('');
  const [burnKey, setBurnKey] = useState('');
  const [burnAmount, setBurnAmount] = useState('');
  const [burnToLqd, setBurnToLqd] = useState('');
  const [burnStatus, setBurnStatus] = useState('');
  const [bscToken, setBscToken] = useState('');
  const [bscKey, setBscKey] = useState('');
  const [bscToLqd, setBscToLqd] = useState('');
  const [bscAmount, setBscAmount] = useState('');
  const [bscStatus, setBscStatus] = useState('');
  const [lqdToken, setLqdToken] = useState('');
  const [lqdKey, setLqdKey] = useState('');
  const [lqdToBsc, setLqdToBsc] = useState('');
  const [lqdAmount, setLqdAmount] = useState('');
  const [lqdStatus, setLqdStatus] = useState('');
  const [tokenMappings, setTokenMappings] = useState([]);

  const defaultTokens = [
    { symbol: 'USDT', address: '0xfd086bc7cd5c481dcc9c85ebe478a1c0b69fcbb9' },
    { symbol: 'USDC', address: '0x64544969ed7EBf5f083679233325356EbE738930' },
    { symbol: 'BUSD', address: '0xed24fc36d5ee211ea25a80239fb8c4cfd80f12ee' },
  ];

  const loadRequests = async () => {
    try {
      const q = from ? `?address=${encodeURIComponent(from)}` : '';
      const data = await fetchJSON(`/bridge/requests${q}`);
      setRequests(Array.isArray(data) ? data : []);
    } catch (e) {
      setRequests([]);
    }
  };

  useEffect(() => {
    if (from) {
      loadRequests();
    }
  }, [from]);

  const loadTokenMappings = async () => {
    try {
      const data = await fetchJSON('/bridge/tokens');
      setTokenMappings(Array.isArray(data) ? data : []);
    } catch (e) {
      setTokenMappings([]);
    }
  };

  useEffect(() => {
    loadTokenMappings();
  }, []);

  const submitBridge = async () => {
    setStatus('');
    if (!from || !privateKey || !toBsc || !amount) {
      setStatus('Please fill all fields');
      return;
    }
    try {
      const body = {
        from,
        private_key: privateKey,
        to_bsc: toBsc,
        amount: amount,
        gas_price: Number(gasPrice || 0),
        gas: 50000,
      };
      const res = await fetch(`${API_BASE}/wallet/bridge/lock`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });
      if (!res.ok) {
        const text = await res.text();
        throw new Error(text || `HTTP ${res.status}`);
      }
      const json = await res.json();
      setStatus(`Bridge lock submitted: ${json?.tx_hash || json?.TxHash || 'ok'}`);
      loadRequests();
    } catch (e) {
      setStatus(e.message || 'Bridge lock failed');
    }
  };

  const submitBurn = async () => {
    setBurnStatus('');
    if (!burnKey || !burnAmount || !burnToLqd) {
      setBurnStatus('Please fill all fields');
      return;
    }
    try {
      const body = {
        private_key: burnKey,
        amount: burnAmount,
        to_lqd: burnToLqd,
      };
      const res = await fetch(`${API_BASE}/wallet/bridge/burn`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });
      if (!res.ok) {
        const text = await res.text();
        throw new Error(text || `HTTP ${res.status}`);
      }
      const json = await res.json();
      setBurnStatus(`Burn submitted: ${json?.tx_hash || 'ok'}`);
    } catch (e) {
      setBurnStatus(e.message || 'Burn failed');
    }
  };

  const submitBscLock = async () => {
    setBscStatus('');
    if (!bscKey || !bscToken || !bscToLqd || !bscAmount) {
      setBscStatus('Please fill all fields');
      return;
    }
    try {
      const body = {
        private_key: bscKey,
        token: bscToken,
        to_lqd: bscToLqd,
        amount: bscAmount,
      };
      const res = await fetch(`${API_BASE}/wallet/bridge/lock_bsc_token`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });
      if (!res.ok) {
        const text = await res.text();
        throw new Error(text || `HTTP ${res.status}`);
      }
      const json = await res.json();
      setBscStatus(`BSC lock submitted: ${json?.tx_hash || 'ok'}`);
      loadRequests();
      loadTokenMappings();
    } catch (e) {
      setBscStatus(e.message || 'Lock failed');
    }
  };

  const submitLqdBurn = async () => {
    setLqdStatus('');
    if (!lqdKey || !lqdToken || !lqdToBsc || !lqdAmount) {
      setLqdStatus('Please fill all fields');
      return;
    }
    try {
      const body = {
        private_key: lqdKey,
        token: lqdToken,
        to_bsc: lqdToBsc,
        amount: lqdAmount,
      };
      const res = await fetch(`${API_BASE}/wallet/bridge/burn_lqd_token`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });
      if (!res.ok) {
        const text = await res.text();
        throw new Error(text || `HTTP ${res.status}`);
      }
      const json = await res.json();
      setLqdStatus(`Burn submitted: ${json?.tx_hash || json?.TxHash || 'ok'}`);
      loadRequests();
    } catch (e) {
      setLqdStatus(e.message || 'Burn failed');
    }
  };

  return (
    <div className="page">
      <h2>Bridge (LQD ↔ BSC Testnet)</h2>
      <div className="card">
        <h3>Lock BEP20 on BSC → Mint on LQD</h3>
        <div className="form-row">
          <label>BSC Token</label>
          <select value={bscToken} onChange={(e) => setBscToken(e.target.value)}>
            <option value="">Select token</option>
            {defaultTokens.map((t) => (
              <option key={t.address} value={t.address}>{t.symbol} ({t.address.slice(0, 8)}…)</option>
            ))}
          </select>
          <input
            value={bscToken}
            onChange={(e) => setBscToken(e.target.value)}
            placeholder="Or paste token address"
          />
        </div>
        <div className="form-row">
          <label>BSC Private Key</label>
          <input value={bscKey} onChange={(e) => setBscKey(e.target.value)} placeholder="private key" />
          <small>Raw hex, may include 0x prefix</small>
        </div>
        <div className="form-row">
          <label>To (LQD address)</label>
          <input value={bscToLqd} onChange={(e) => setBscToLqd(e.target.value)} placeholder="0x..." />
        </div>
        <div className="form-row">
          <label>Amount</label>
          <input value={bscAmount} onChange={(e) => setBscAmount(e.target.value)} placeholder="1000" />
        </div>
        <button className="btn-primary" onClick={submitBscLock}>Bridge Now</button>
        {bscStatus && <div className="notice">{bscStatus}</div>}
      </div>

      <div className="card">
        <h3>Burn LQD Token → Release on BSC</h3>
        <div className="form-row">
          <label>LQD Token Contract</label>
          <select value={lqdToken} onChange={(e) => setLqdToken(e.target.value)}>
            <option value="">Select token</option>
            {tokenMappings.map((t) => (
              <option key={t.lqd_token} value={t.lqd_token}>
                {t.symbol} ({t.lqd_token.slice(0, 8)}…)
              </option>
            ))}
          </select>
          <input
            value={lqdToken}
            onChange={(e) => setLqdToken(e.target.value)}
            placeholder="Or paste LQD token address"
          />
        </div>
        <div className="form-row">
          <label>LQD Private Key</label>
          <input value={lqdKey} onChange={(e) => setLqdKey(e.target.value)} placeholder="private key" />
        </div>
        <div className="form-row">
          <label>To (BSC address)</label>
          <input value={lqdToBsc} onChange={(e) => setLqdToBsc(e.target.value)} placeholder="0x..." />
        </div>
        <div className="form-row">
          <label>Amount</label>
          <input value={lqdAmount} onChange={(e) => setLqdAmount(e.target.value)} placeholder="1000" />
        </div>
        <button className="btn-primary" onClick={submitLqdBurn}>Burn on LQD</button>
        {lqdStatus && <div className="notice">{lqdStatus}</div>}
      </div>

      <div className="card">
        <h3>Lock LQD → Mint on BSC</h3>
        <div className="form-row">
          <label>From (LQD address)</label>
          <input value={from} onChange={(e) => setFrom(e.target.value)} placeholder="0x..." />
        </div>
        <div className="form-row">
          <label>Private Key</label>
          <input value={privateKey} onChange={(e) => setPrivateKey(e.target.value)} placeholder="private key" />
          <small>Use raw hex (no 0x prefix)</small>
        </div>
        <div className="form-row">
          <label>To (BSC address)</label>
          <input value={toBsc} onChange={(e) => setToBsc(e.target.value)} placeholder="0x..." />
        </div>
        <div className="form-row">
          <label>Amount (LQD)</label>
          <input value={amount} onChange={(e) => setAmount(e.target.value)} placeholder="100" />
        </div>
        <div className="form-row">
          <label>Gas Price</label>
          <input value={gasPrice} onChange={(e) => setGasPrice(e.target.value)} />
        </div>
        <button className="btn-primary" onClick={submitBridge}>Bridge Now</button>
        {status && <div className="notice">{status}</div>}
      </div>

      <div className="card">
        <h3>Bridge Requests</h3>
        <button className="btn-secondary" onClick={loadRequests}>Refresh</button>
        <table className="table">
          <thead>
            <tr>
              <th>ID</th>
              <th>From</th>
              <th>To</th>
              <th>Amount</th>
              <th>Status</th>
              <th>Token</th>
              <th>LQD Tx</th>
              <th>BSC Tx</th>
            </tr>
          </thead>
          <tbody>
            {requests.length === 0 ? (
              <tr><td colSpan="8">No bridge requests</td></tr>
            ) : (
              requests.map((r) => (
                <tr key={r.id}>
                  <td>{r.id?.slice(0, 10)}…</td>
                  <td>{r.from?.slice(0, 10)}…</td>
                  <td>{r.to?.slice(0, 10)}…</td>
                  <td>{r.amount}</td>
                  <td>{r.status}</td>
                  <td>{r.token ? r.token.slice(0, 10) + '…' : 'LQD'}</td>
                  <td>{r.lqd_tx_hash?.slice(0, 10)}…</td>
                  <td>{r.bsc_tx_hash ? `${r.bsc_tx_hash.slice(0, 10)}…` : '—'}</td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      <div className="card">
        <h3>Burn on BSC → Unlock on LQD</h3>
        <div className="form-row">
          <label>BSC Private Key</label>
          <input value={burnKey} onChange={(e) => setBurnKey(e.target.value)} placeholder="private key" />
          <small>Use raw hex (no 0x prefix)</small>
        </div>
        <div className="form-row">
          <label>Amount (LQD)</label>
          <input value={burnAmount} onChange={(e) => setBurnAmount(e.target.value)} placeholder="100" />
        </div>
        <div className="form-row">
          <label>To (LQD address)</label>
          <input value={burnToLqd} onChange={(e) => setBurnToLqd(e.target.value)} placeholder="0x..." />
        </div>
        <button className="btn-primary" onClick={submitBurn}>Burn on BSC</button>
        {burnStatus && <div className="notice">{burnStatus}</div>}
      </div>
    </div>
  );
};

export default BridgePage;
