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


// src/components/SendTransaction.jsx
import React, { useState } from 'react';
import { parseLQD } from "./lqdUnits";
import { parseUnits } from 'ethers';

const SendTransaction = ({ fromAddress, privateKey }) => {
  const [toAddress, setToAddress] = useState('');
  const [amount, setAmount] = useState('');
  const [gasPrice, setGasPrice] = useState('1');        // 🔥 changed (backend uses 1 by default)
  const [gasLimit, setGasLimit] = useState('21000');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');

  const handleSendTransaction = async (e) => {
    e.preventDefault();
    
    if (!toAddress || !amount || !privateKey) {
      setError('Please fill all fields');
      return;
    }

    // 🔥 ensure proper address format
    if (!toAddress.startsWith("0x")) {
      setError("Invalid address. Must start with 0x.");
      return;
    }

    setLoading(true);
    setError('');
    setSuccess('');



    try {

      const rawValueStr = parseLQD(amount); // base units as string
      console.log("amount=", amount, "rawValueStr=", rawValueStr);
      const transactionData = {
        from: fromAddress,
        to: toAddress,
        value: rawValueStr, // send as string to support large values
        gas: parseInt(gasLimit, 10) || 21000,
        gas_price: parseInt(gasPrice, 10) || 1,
        private_key: privateKey
      };

      // 🔥 FIXED: correct backend endpoint
      const response = await fetch('http://127.0.0.1:9000/wallet/send', {
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
          <label>Amount (LQD):</label>
          <input
            type="number"
            value={amount}
            onChange={(e) => setAmount(e.target.value)}
            placeholder="Enter amount"
            min="0"
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
              />
            </div>
            <div className="form-group">
              <label>Gas Limit:</label>
              <input
                type="number"
                value={gasLimit}
                onChange={(e) => setGasLimit(e.target.value)}
                min="21000"
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
