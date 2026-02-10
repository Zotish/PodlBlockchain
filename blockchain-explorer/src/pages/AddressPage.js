import React, { useState, useEffect } from 'react';
import { useParams } from 'react-router-dom';
import TransactionList from '../components/TransactionList';
import { formatLQD } from "../utils/lqdUnits";
import { fetchJSON, mergeArrayResults, firstNodeResult } from "../utils/api";

const AddressPage = () => {
  const { address } = useParams();
  const [addressData, setAddressData] = useState(null);
  const [transactions, setTransactions] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
    const fetchAddressData = async () => {
      try {
        // Fetch address balance and basic info
        const balanceData = await fetchJSON(`/balance?address=${address}`);
        const balanceResult = firstNodeResult(balanceData);
        if (!balanceResult) {
          throw new Error('Address not found');
        }
        
        // Fetch transactions for this address
        const txsData = await fetchJSON(`/address/${address}/transactions`);
        const mergedTxs = mergeArrayResults(txsData, "tx_hash");
        
        setAddressData({
          address,
          balance: balanceResult.balance,
          isValidator: balanceResult.isValidator || false
        });
        setTransactions(mergedTxs);
      } catch (err) {
        setError(err.message);
      } finally {
        setLoading(false);
      }
    };

    fetchAddressData();
  }, [address]);

  if (loading) return <div className="loading">Loading address details...</div>;
  if (error) return <div className="error">Error: {error}</div>;

  return (
    <div className="address-page">
      <h2>Address Details</h2>
      <div className="address-hash">{address}</div>
      
      <div className="address-summary">
        <div className="balance-card">
          <h3>Balance</h3>
          <p>{formatLQD(addressData.balance)} LQD</p>
        </div>
        
        {addressData.isValidator && (
          <div className="validator-badge">
            <span>VALIDATOR</span>
            <a href={`/validator/${address}`}>View Validator Details</a>
          </div>
        )}
      </div>

      <h3>Transactions</h3>
      {transactions.length > 0 ? (
        <TransactionList transactions={transactions} />
      ) : (
        <p>No transactions found for this address</p>
      )}
    </div>
  );
};

export default AddressPage;
