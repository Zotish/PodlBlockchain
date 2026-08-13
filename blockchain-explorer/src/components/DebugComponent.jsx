// DebugComponent.jsx
import React, { useState, useEffect } from 'react';
import { API_BASE, apiUrl } from '../utils/api';

const DebugComponent = () => {
  const [apiData, setApiData] = useState({});

  useEffect(() => {
    const fetchDebugData = async () => {
      const endpoints = [
        apiUrl(API_BASE, '/network'),
        apiUrl(API_BASE, '/validators'),
        apiUrl(API_BASE, '/fetch_last_n_block')
      ];

      const results = {};
      
      for (const endpoint of endpoints) {
        try {
          const response = await fetch(endpoint);
          const data = await response.json();
          results[endpoint] = data;
        } catch (error) {
          results[endpoint] = { error: error.message };
        }
      }
      
      setApiData(results);
    };

    fetchDebugData();
  }, []);

  return (
    <div style={{ padding: '20px', background: '#f5f5f5' }}>
      <h3>API Debug Information</h3>
      {Object.entries(apiData).map(([endpoint, data]) => (
        <div key={endpoint} style={{ marginBottom: '20px', padding: '10px', background: 'white', border: '1px solid #ccc' }}>
          <h4>Endpoint: {endpoint}</h4>
          <pre>{JSON.stringify(data, null, 2)}</pre>
        </div>
      ))}
    </div>
  );
};

export default DebugComponent;
