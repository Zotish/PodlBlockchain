import React, { useState, useEffect } from 'react';
import { fetchJSON, mergeArrayResults } from '../../utils/api';

const ContractList = ({ address }) => {
  const [contracts, setContracts] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const fetchContracts = async () => {
    try {
      setLoading(true);
      setError('');
      const data = await fetchJSON('/contract/list');
      const list = Array.isArray(data) ? data : mergeArrayResults(data, 'address');
      const filtered = address
        ? list.filter(
            (c) =>
              c &&
              c.owner &&
              c.owner.toLowerCase() === address.toLowerCase()
          )
        : list;
      setContracts(filtered || []);
    } catch (err) {
      setError(err.message || 'Failed to load contracts');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchContracts();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [address]);

  if (loading) {
    return (
      <div className="loading" style={{ padding: 24, textAlign: 'center' }}>
        <div style={{ fontSize: 28, marginBottom: 8 }}>⏳</div>
        Loading contracts...
      </div>
    );
  }

  if (error) {
    return (
      <div style={{
        background: '#fef2f2', border: '1px solid #fecaca',
        borderRadius: 10, padding: 20, margin: 16, textAlign: 'center'
      }}>
        <div style={{ fontSize: 24, marginBottom: 8 }}>⚠️</div>
        <div style={{ fontWeight: 600, color: '#b91c1c', marginBottom: 8 }}>
          Failed to load contracts
        </div>
        <div style={{ fontSize: 13, color: '#6b7280', marginBottom: 12 }}>{error}</div>
        <button
          onClick={fetchContracts}
          style={{
            padding: '8px 16px', borderRadius: 8,
            background: '#2563eb', color: '#fff', border: 'none', cursor: 'pointer'
          }}
        >
          Retry
        </button>
      </div>
    );
  }

  if (contracts.length === 0) {
    return (
      <div style={{
        textAlign: 'center', padding: 40, color: '#6b7280'
      }}>
        <div style={{ fontSize: 40, marginBottom: 12 }}>📄</div>
        <div style={{ fontWeight: 600, fontSize: 16, marginBottom: 6 }}>
          No contracts deployed yet
        </div>
        <div style={{ fontSize: 13 }}>
          Deploy a smart contract to see it listed here.
        </div>
        <button
          className="btn-secondary"
          onClick={fetchContracts}
          style={{ marginTop: 16 }}
        >
          Refresh
        </button>
      </div>
    );
  }

  return (
    <div className="contract-list">
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
        <h3 style={{ margin: 0 }}>Deployed Contracts ({contracts.length})</h3>
        <button className="btn-secondary" onClick={fetchContracts}>
          Refresh
        </button>
      </div>

      <div className="contracts-grid">
        {contracts.map((contract, idx) => (
          <div key={contract.address || idx} className="contract-card" style={{
            background: '#f9fafb', border: '1px solid #e5e7eb',
            borderRadius: 10, padding: 16, marginBottom: 12
          }}>
            <div style={{ fontWeight: 600, marginBottom: 6, wordBreak: 'break-all' }}>
              {contract.address}
            </div>
            {contract.name && (
              <div style={{ fontSize: 13, color: '#6b7280', marginBottom: 4 }}>
                <strong>Name:</strong> {contract.name}
              </div>
            )}
            {contract.owner && (
              <div style={{ fontSize: 13, color: '#6b7280', marginBottom: 4, wordBreak: 'break-all' }}>
                <strong>Owner:</strong> {contract.owner}
              </div>
            )}
            {contract.block_number != null && (
              <div style={{ fontSize: 13, color: '#6b7280' }}>
                <strong>Block:</strong> {contract.block_number}
              </div>
            )}
          </div>
        ))}
      </div>
    </div>
  );
};

export default ContractList;
