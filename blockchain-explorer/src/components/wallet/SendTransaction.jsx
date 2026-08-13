// // src/components/SendTransaction.jsx
// import React, { useState } from 'react';

// const SendTransaction = ({ fromAddress, privateKey }) => {
//   const [toAddress, setToAddress] = useState('');
//   const [amount, setAmount] = useState('');
//   const [gasPrice, setGasPrice] = useState('50');
//   const [gasLimit, setGasLimit] = useState('21000');
//   const [loading, setLoading] = useState(false);
//   const [error, setError] = useState('');
//   const [success, setSuccess] = useState('');

//   const handleSendTransaction = async (e) => {
//     e.preventDefault();
    
//     if (!toAddress || !amount || !privateKey) {
//       setError('Please fill all fields');
//       return;
//     }

//     setLoading(true);
//     setError('');
//     setSuccess('');

//     try {
//       const transactionData = {
//         from: fromAddress,
//         to: toAddress,
//         value: parseInt(amount),
//         gas: parseInt(gasLimit),
//         gas_price: parseInt(gasPrice),
//         private_key: privateKey
//       };

//       const response = await fetch('http://127.0.0.1:8080/wallet/send', {
//         method: 'POST',
//         headers: {
//           'Content-Type': 'application/json',
//         },
//         body: JSON.stringify(transactionData),
//       });

//       const result = await response.json();

//       if (!response.ok) {
//         throw new Error(result.error || 'Transaction failed');
//       }

//       setSuccess(`Transaction sent successfully! Hash: ${result.tx_hash}`);
//       setToAddress('');
//       setAmount('');
      
//     } catch (err) {
//       setError(err.message);
//     } finally {
//       setLoading(false);
//     }
//   };

//   return (
//     <div className="send-transaction">
//       <h3>Send LQD Coins</h3>
      
//       <form onSubmit={handleSendTransaction} className="transaction-form">
//         <div className="form-group">
//           <label>From Address:</label>
//           <input 
//             type="text" 
//             value={fromAddress} 
//             readOnly 
//             className="readonly"
//           />
//         </div>

//         <div className="form-group">
//           <label>To Address:</label>
//           <input
//             type="text"
//             value={toAddress}
//             onChange={(e) => setToAddress(e.target.value)}
//             placeholder="Enter recipient address (0x...)"
//             required
//           />
//         </div>

//         <div className="form-group">
//           <label>Amount (LQD):</label>
//           <input
//             type="number"
//             value={amount}
//             onChange={(e) => setAmount(e.target.value)}
//             placeholder="Enter amount"
//             min="1"
//             required
//           />
//         </div>

//         <div className="gas-settings">
//           <h4>Gas Settings</h4>
//           <div className="form-row">
//             <div className="form-group">
//               <label>Gas Price:</label>
//               <input
//                 type="number"
//                 value={gasPrice}
//                 onChange={(e) => setGasPrice(e.target.value)}
//                 min="1"
//               />
//             </div>
//             <div className="form-group">
//               <label>Gas Limit:</label>
//               <input
//                 type="number"
//                 value={gasLimit}
//                 onChange={(e) => setGasLimit(e.target.value)}
//                 min="21000"
//               />
//             </div>
//           </div>
//         </div>

//         {error && <div className="error-message">{error}</div>}
//         {success && <div className="success-message">{success}</div>}

//         <button 
//           type="submit" 
//           disabled={loading}
//           className="btn-primary btn-send"
//         >
//           {loading ? 'Sending...' : 'Send Transaction'}
//         </button>
//       </form>

//       <div className="transaction-info">
//         <h4>Transaction Information</h4>
//         <ul>
//           <li>Standard gas limit for simple transfers: 21,000</li>
//           <li>Gas price determines transaction priority</li>
//           <li>Transactions typically confirm in 6-12 seconds</li>
//           <li>Network fee = Gas Limit × Gas Price</li>
//         </ul>
//       </div>
//     </div>
//   );
// };

// export default SendTransaction;


/* global BigInt */
// src/components/SendTransaction.jsx
import React, { useState, useEffect } from 'react';
import { parseLQD, formatLQD, LQD_DECIMALS } from "./lqdUnits";
import { API_BASE, apiUrl, waitForTx } from "../../utils/api";

const NODE_URL = API_BASE;

const SendTransaction = ({ fromAddress, privateKey }) => {
  const [toAddress, setToAddress] = useState('');
  const [amount, setAmount] = useState('');
  const [gasPrice, setGasPrice] = useState('1');
  const [gasLimit, setGasLimit] = useState('21000');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');
  // ── live balance for validation & MAX button
  const [rawBalance, setRawBalance] = useState('0');

  useEffect(() => {
    if (!fromAddress) return;
    fetch(apiUrl(NODE_URL, `/balance?address=${encodeURIComponent(fromAddress)}`))
      .then(r => r.json())
      .then(d => setRawBalance(d.balance || d.Balance || "0"))
      .catch(() => {});
  }, [fromAddress]);

  const humanBalance = formatLQD(rawBalance);  // e.g. "10000000"

  const handleMax = () => {
    // max = balance − fee;  keep a small buffer for gas
    try {
      const gp = BigInt(parseInt(gasPrice, 10) || 1);
      const gl = BigInt(parseInt(gasLimit, 10) || 21000);
      const fee = gp * gl;
      const bal = BigInt(rawBalance || "0");
      const maxRaw = bal > fee ? bal - fee : 0n;
      // convert raw → human (divide by 10^8)
      const intPart = maxRaw / BigInt(10 ** LQD_DECIMALS);
      const fracPart = (maxRaw % BigInt(10 ** LQD_DECIMALS)).toString().padStart(LQD_DECIMALS, "0").replace(/0+$/, "");
      setAmount(fracPart ? `${intPart}.${fracPart}` : String(intPart));
    } catch { /* ignore */ }
  };

  // Input validation helper
  const validateAmount = (val) => {
    if (!val || val.trim() === "") return "Amount is required";
    const parts = val.split(".");
    if (parts.length > 2) return "Invalid amount format";
    const intPart = parts[0].replace(/^-/, "");
    if (intPart.length > 18) return "Amount too large (max 18 digits)";
    if (parts[1] && parts[1].length > 8) return "Max 8 decimal places for LQD";
    if (parseFloat(val) <= 0) return "Amount must be greater than 0";
    // balance check (human units)
    try {
      const rawSend = BigInt(parseLQD(val));
      const gp = BigInt(parseInt(gasPrice, 10) || 1);
      const gl = BigInt(parseInt(gasLimit, 10) || 21000);
      const fee = gp * gl;
      const bal = BigInt(rawBalance || "0");
      if (rawSend + fee > bal) {
        return `Insufficient balance. You have ${humanBalance} LQD, sending ${val} LQD + fee.`;
      }
    } catch { /* ignore bigint errors */ }
    return null;
  };

  const handleSendTransaction = async (e) => {
    e.preventDefault();

    if (!toAddress || !amount || !privateKey) {
      setError('Please fill all fields');
      return;
    }

    if (!toAddress.startsWith("0x")) {
      setError("Invalid address. Must start with 0x.");
      return;
    }

    const amountErr = validateAmount(amount);
    if (amountErr) { setError(amountErr); return; }

    const gasPriceNum = parseInt(gasPrice, 10);
    const gasLimitNum = parseInt(gasLimit, 10);
    if (!gasPriceNum || gasPriceNum < 1 || gasPriceNum > 10_000_000) {
      setError("Gas price must be between 1 and 10,000,000"); return;
    }
    if (!gasLimitNum || gasLimitNum < 21000 || gasLimitNum > 10_000_000) {
      setError("Gas limit must be between 21,000 and 10,000,000"); return;
    }

    setLoading(true);
    setError('');
    setSuccess('');

    try {
      let rawValueStr;
      try {
        rawValueStr = parseLQD(amount);
      } catch (parseErr) {
        setError("Invalid amount: " + parseErr.message);
        setLoading(false);
        return;
      }
      console.log("amount=", amount, "rawValueStr=", rawValueStr);
      const transactionData = {
        from: fromAddress,
        to: toAddress,
        value: rawValueStr, // send as string to support large values
        gas: parseInt(gasLimit, 10) || 21000,
        gas_price: parseInt(gasPrice, 10) || 1,
        private_key: privateKey
      };

      const response = await fetch(apiUrl(NODE_URL, "/wallet/send"), {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(transactionData),
      });

      const result = await response.json();

      if (!response.ok) {
        throw new Error(result.error || 'Transaction failed');
      }

      // 🔥 backend might return tx_hash or TxHash → handle both
      const hash = result.tx_hash || result.TxHash || result.hash;

      setSuccess(`Transaction sent! Hash: ${hash}`);
      setToAddress('');
      setAmount('');
      if (hash) {
        await waitForTx(hash, 5000).catch(() => null);
      }
      // refresh live balance display after confirmation
      fetch(apiUrl(NODE_URL, `/balance?address=${encodeURIComponent(fromAddress)}`))
        .then(r => r.json())
        .then(d => {
          setRawBalance(d.balance || d.Balance || "0");
          try {
            window.dispatchEvent(new CustomEvent("lqd:wallet-updated", { detail: { address: fromAddress, txHash: hash } }));
          } catch {}
        })
        .catch(() => {});
      
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="send-transaction">
      <h3>Send LQD Coins</h3>
      
      <form onSubmit={handleSendTransaction} className="transaction-form">
        <div className="form-group">
          <label>From Address:</label>
          <input 
            type="text" 
            value={fromAddress} 
            readOnly 
            className="readonly"
          />
        </div>

        <div className="form-group">
          <label>To Address:</label>
          <input
            type="text"
            value={toAddress}
            onChange={(e) => setToAddress(e.target.value)}
            placeholder="Enter recipient address (0x...)"
            required
          />
        </div>

        <div className="form-group">
          <label style={{ display: "flex", justifyContent: "space-between" }}>
            <span>Amount (LQD):</span>
            <span style={{ fontWeight: "normal", opacity: 0.7 }}>
              Balance: <strong>{humanBalance} LQD</strong>
              <button
                type="button"
                onClick={handleMax}
                style={{ marginLeft: 8, fontSize: 11, padding: "1px 6px", cursor: "pointer" }}
              >MAX</button>
            </span>
          </label>
          <input
            type="number"
            value={amount}
            onChange={(e) => setAmount(e.target.value)}
            placeholder="e.g. 100.5"
            min="0"
            step="0.00000001"
            required
          />
        </div>

        <div className="gas-settings">
          <h4>Gas Settings</h4>
          <div className="form-row">
            <div className="form-group">
              <label>Gas Price:</label>
              <input
                type="number"
                value={gasPrice}
                onChange={(e) => setGasPrice(e.target.value)}
                min="1"
                max="10000000"
              />
            </div>
            <div className="form-group">
              <label>Gas Limit:</label>
              <input
                type="number"
                value={gasLimit}
                onChange={(e) => setGasLimit(e.target.value)}
                min="21000"
                max="10000000"
              />
            </div>
          </div>
        </div>

        {error && <div className="error-message">{error}</div>}
        {success && <div className="success-message">{success}</div>}

        <button 
          type="submit" 
          disabled={loading}
          className="btn-primary btn-send"
        >
          {loading ? 'Sending...' : 'Send Transaction'}
        </button>
      </form>

      <div className="transaction-info">
        <h4>Transaction Information</h4>
        <ul>
          <li>Standard gas limit for simple transfers: 21,000</li>
          <li>Gas price determines transaction priority</li>
          <li>Transactions typically confirm in 6-12 seconds</li>
          <li>Network fee = Gas Limit × Gas Price</li>
        </ul>
      </div>
    </div>
  );
};

export default SendTransaction;
