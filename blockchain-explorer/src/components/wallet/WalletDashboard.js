




import React, { useState, useEffect, useCallback, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import WalletLogin from './WalletLogin';
import WalletBalance from './WalletBalance';
import SendTransaction from './SendTransaction';
import ReceiveSection from './ReceiveSection';
import TransactionHistory from './TransactionHistory';
import ContractManager from '../contracts/ContractManager';

import './Wallet.css';
import LiquidityDashboard from './LiquidityDashboard';
import BridgePanel from './BridgePanel';

const STORAGE_KEY = 'liquidityChainWallet';
const TOKENS_STORAGE_KEY = 'liquidity_tokens_v1';
const INACTIVITY_LIMIT = 60 * 1000; // 1 minute

// Helpers for encrypting/decrypting private key using Web Crypto
const enc = new TextEncoder();
const dec = new TextDecoder();

async function deriveKey(password, saltBytes) {
  const keyMaterial = await window.crypto.subtle.importKey(
    'raw',
    enc.encode(password),
    'PBKDF2',
    false,
    ['deriveKey']
  );

  return window.crypto.subtle.deriveKey(
    {
      name: 'PBKDF2',
      salt: saltBytes,
      iterations: 250000,
      hash: 'SHA-256',
    },
    keyMaterial,
    { name: 'AES-GCM', length: 256 },
    false,
    ['encrypt', 'decrypt']
  );
}

function b64encode(bytes) {
  return btoa(String.fromCharCode(...new Uint8Array(bytes)));
}

function b64decode(str) {
  return Uint8Array.from(atob(str), c => c.charCodeAt(0));
}

async function encryptPrivateKey(password, privateKey) {
  const salt = window.crypto.getRandomValues(new Uint8Array(16));
  const iv   = window.crypto.getRandomValues(new Uint8Array(12));

  const key = await deriveKey(password, salt);
  const ciphertext = await window.crypto.subtle.encrypt(
    { name: 'AES-GCM', iv },
    key,
    enc.encode(privateKey)
  );

  return {
    encryptedPrivateKey: b64encode(ciphertext),
    salt: b64encode(salt),
    iv: b64encode(iv),
  };
}

async function decryptPrivateKey(password, encryptedBundle) {
  const { encryptedPrivateKey, salt, iv } = encryptedBundle;
  const saltBytes = b64decode(salt);
  const ivBytes   = b64decode(iv);
  const cipherBytes = b64decode(encryptedPrivateKey);

  const key = await deriveKey(password, saltBytes);
  const plain = await window.crypto.subtle.decrypt(
    { name: 'AES-GCM', iv: ivBytes },
    key,
    cipherBytes
  );

  return dec.decode(plain);
}

const WalletDashboard = () => {
  const [activeTab, setActiveTab] = useState('balance');
  const [walletAddress, setWalletAddress] = useState('');
  const [privateKey, setPrivateKey] = useState('');
  const [isWalletLoaded, setIsWalletLoaded] = useState(false);
  const [savedWalletMeta, setSavedWalletMeta] = useState(null);
  const [unlockPassword, setUnlockPassword] = useState('');
  const [unlockError, setUnlockError] = useState('');
  const [backupMessage, setBackupMessage] = useState('');
  const navigate = useNavigate();
  const sentConnectRef = useRef(false);
  const backupInputRef = useRef(null);

  // --------------------------------------------------
  // 🔐 AUTO-LOCK HOOKS (always at top, non-conditional)
  // --------------------------------------------------
  const inactivityRef = useRef(null);
  const lockWallet = useCallback(() => {
    setPrivateKey('');
    setIsWalletLoaded(false);
    setUnlockPassword('');
    setUnlockError('');
  }, []);
  const disconnectWallet = useCallback(() => {
    setWalletAddress('');
    setPrivateKey('');
    setIsWalletLoaded(false);
    setSavedWalletMeta(null);
    setUnlockPassword('');
    setUnlockError('');
    localStorage.removeItem(STORAGE_KEY);
    sentConnectRef.current = false;
  }, []);

  const exportBackup = useCallback(() => {
    if (!walletAddress) return;
    const walletRaw = localStorage.getItem(STORAGE_KEY);
    const tokensRaw = localStorage.getItem(`${TOKENS_STORAGE_KEY}_${walletAddress.toLowerCase()}`) || '[]';
    let wallet = { address: walletAddress };
    try {
      wallet = walletRaw ? JSON.parse(walletRaw) : wallet;
    } catch {}
    let tokens = [];
    try {
      tokens = JSON.parse(tokensRaw);
    } catch {}
    const backup = {
      exportedAt: new Date().toISOString(),
      wallet,
      tokens,
      app: 'blockchain-explorer-wallet',
      version: 1,
    };
    const blob = new Blob([JSON.stringify(backup, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `lqd-wallet-backup-${walletAddress.slice(2, 8)}.json`;
    a.click();
    URL.revokeObjectURL(url);
    setBackupMessage('Backup exported.');
  }, [walletAddress]);

  const importBackupFile = useCallback(async (file) => {
    if (!file) return;
    const parsed = JSON.parse(await file.text());
    const wallet = parsed.wallet || parsed;
    if (!wallet?.address) {
      throw new Error('Backup file missing wallet address');
    }
    localStorage.setItem(STORAGE_KEY, JSON.stringify(wallet));
    if (Array.isArray(parsed.tokens)) {
      localStorage.setItem(`${TOKENS_STORAGE_KEY}_${wallet.address.toLowerCase()}`, JSON.stringify(parsed.tokens));
    }
    setSavedWalletMeta(wallet);
    setWalletAddress(wallet.address);
    setIsWalletLoaded(false);
    setPrivateKey('');
    setUnlockPassword('');
    setBackupMessage('Backup imported. Unlock the wallet to continue.');
  }, []);

  const resetInactivityTimer = useCallback(() => {
   if (!isWalletLoaded) return; // only run when wallet unlocked

    if (inactivityRef.current) clearTimeout(inactivityRef.current);

    inactivityRef.current = setTimeout(() => {
      console.log("Auto-lock: wallet locked due to inactivity.");
      lockWallet();
    }, INACTIVITY_LIMIT);
  }, [isWalletLoaded, lockWallet]);

  useEffect(() => {
    if (!isWalletLoaded) return; // only active when unlocked

    resetInactivityTimer();

    const events = ["mousemove", "keydown", "click", "touchstart"];
    const handler = () => resetInactivityTimer();

    events.forEach(evt => window.addEventListener(evt, handler));

    return () => {
      if (inactivityRef.current) clearTimeout(inactivityRef.current);
      events.forEach(evt => window.removeEventListener(evt, handler));
    };
  }, [isWalletLoaded, resetInactivityTimer]);
  // --------------------------------------------------

  // Load encrypted wallet metadata from localStorage on mount
  useEffect(() => {
    const saved = localStorage.getItem(STORAGE_KEY);
    if (saved) {
      try {
        const parsed = JSON.parse(saved);

        // Reject old insecure format
        if (parsed.privateKey && !parsed.encryptedPrivateKey) {
          console.warn('Found legacy insecure wallet; forcing reset.');
          localStorage.removeItem(STORAGE_KEY);
        } else {
          setSavedWalletMeta(parsed);
          setWalletAddress(parsed.address || '');
        }
      } catch (e) {
        console.error('Failed to parse saved wallet:', e);
        localStorage.removeItem(STORAGE_KEY);
      }
    }
  }, []);

  const persistEncryptedWallet = (address, encryptedBundle) => {
    const toStore = {
      address,
      encryptedPrivateKey: encryptedBundle.encryptedPrivateKey,
      salt: encryptedBundle.salt,
      iv: encryptedBundle.iv,
    };
    localStorage.setItem(STORAGE_KEY, JSON.stringify(toStore));
    setSavedWalletMeta(toStore);
  };

  const handleWalletCreate = async (walletData, password) => {
    try {
      const bundle = await encryptPrivateKey(password, walletData.private_key);
      persistEncryptedWallet(walletData.address, bundle);

      setWalletAddress(walletData.address);
      setPrivateKey(walletData.private_key);
      setIsWalletLoaded(true);
      setUnlockPassword('');
      setUnlockError('');
    } catch {
      setUnlockError('Failed to securely store wallet.');
    }
  };

  const handleWalletImport = async (walletData, password) => {
    try {
      const bundle = await encryptPrivateKey(password, walletData.private_key);
      persistEncryptedWallet(walletData.address, bundle);

      setWalletAddress(walletData.address);
      setPrivateKey(walletData.private_key);
      setIsWalletLoaded(true);
      setUnlockPassword('');
      setUnlockError('');
    } catch {
      setUnlockError('Failed to securely import wallet.');
    }
  };

  const handleUnlock = async () => {
    if (!savedWalletMeta) return;
    setUnlockError('');

    try {
      const pk = await decryptPrivateKey(unlockPassword, savedWalletMeta);
      setPrivateKey(pk);
      setWalletAddress(savedWalletMeta.address);
      setIsWalletLoaded(true);
      setUnlockPassword('');
    } catch {
      setUnlockError('Incorrect password or corrupted wallet data.');
    }
  };

  const viewOnExplorer = () => {
    if (walletAddress) navigate(`/address/${walletAddress}`);
  };

  const sendWalletToOpener = useCallback(() => {
    if (!walletAddress || !privateKey) return;
    const payload = {
      type: 'LQD_WALLET_CONNECT',
      address: walletAddress,
      privateKey: privateKey,
    };
    if (window.opener && !window.opener.closed) {
      window.opener.postMessage(payload, '*');
    }
    if (window.parent && window.parent !== window) {
      window.parent.postMessage(payload, '*');
    }
  }, [walletAddress, privateKey]);

  useEffect(() => {
    if (!isWalletLoaded) return;
    if (sentConnectRef.current) return;
    sendWalletToOpener();
    sentConnectRef.current = true;
  }, [isWalletLoaded, sendWalletToOpener]);

  // --------------------------------------------------
  // 1) No saved wallet → Show create/import page
  // --------------------------------------------------
  if (!savedWalletMeta  && !isWalletLoaded) {
    return (
      <WalletLogin 
        onWalletCreate={handleWalletCreate}
        onWalletImport={handleWalletImport}
      />
    );
  }

  // --------------------------------------------------
  // 2) Wallet saved but locked → Unlock screen
  // --------------------------------------------------
  if (savedWalletMeta && !isWalletLoaded) {
    return (
      <div className="wallet-login">
        <div className="login-container">
          <h2>Unlock Wallet</h2>
          <p>Enter your password to decrypt your wallet.</p>

          {unlockError && <div className="error-message">{unlockError}</div>}

          <div className="form-group">
            <label>Address:</label>
            <input type="text" value={savedWalletMeta.address} readOnly className="readonly" />
          </div>

          <div className="form-group">
            <label>Password:</label>
            <input
              type="password"
              value={unlockPassword}
              onChange={(e) => setUnlockPassword(e.target.value)}
              placeholder="Enter your wallet password"
            />
          </div>

          <button className="btn-primary" onClick={handleUnlock}>
            Unlock
          </button>

          <button className="btn-secondary" onClick={disconnectWallet} style={{ marginTop: 10 }}>
            Forget Wallet
          </button>
        </div>
      </div>
    );
  }

  // --------------------------------------------------
  // 3) Wallet unlocked → Dashboard UI
  // --------------------------------------------------
  return (
    <div className="wallet-dashboard">
      <div className="wallet-header">
        <div className="wallet-info">
          <h2>My Wallet</h2>
          <div className="wallet-address">
            <strong>Address:</strong>
            <span className="address">{walletAddress}</span>
            <button className="btn-copy" onClick={() => navigator.clipboard.writeText(walletAddress)}>
              Copy
            </button>
            <button className="btn-secondary" onClick={viewOnExplorer}>
              View on Explorer
            </button>
          </div>
        </div>

        <div style={{ display: 'flex', gap: 10 }}>
          <button className="btn-secondary" onClick={sendWalletToOpener}>
            Connect to DEX
          </button>
          <button className="btn-disconnect" onClick={disconnectWallet}>
            Disconnect
          </button>
        </div>
      </div>

      <div className="wallet-tabs">
        <button className={`tab ${activeTab === 'balance' ? 'active' : ''}`} onClick={() => setActiveTab('balance')}>
          Balance
        </button>
        <button className={`tab ${activeTab === 'send' ? 'active' : ''}`} onClick={() => setActiveTab('send')}>
          Send
        </button>
        <button className={`tab ${activeTab === 'receive' ? 'active' : ''}`} onClick={() => setActiveTab('receive')}>
          Receive
        </button>
        <button className={`tab ${activeTab === 'history' ? 'active' : ''}`} onClick={() => setActiveTab('history')}>
          History
        </button>
        <button 
       className={`tab ${activeTab === 'contracts' ? 'active' : ''}`}
        onClick={() => setActiveTab('contracts')}
        >
        Contracts
      </button> 
      <button
       className={`tab ${activeTab === 'liquidity' ? 'active' : ''}`}
        onClick={() => setActiveTab('liquidity')}
        >
        Liquidity
      </button>
      <button
       className={`tab ${activeTab === 'bridge' ? 'active' : ''}`}
        onClick={() => setActiveTab('bridge')}
        >
        Bridge
      </button>
      <button
       className={`tab ${activeTab === 'settings' ? 'active' : ''}`}
        onClick={() => setActiveTab('settings')}
        >
        Settings
      </button>
      </div>


      <div className="wallet-content">
        {activeTab === 'balance' && (
          <WalletBalance address={walletAddress} privateKey={privateKey} />
        )}
        {activeTab === 'send' && (
          <SendTransaction fromAddress={walletAddress} privateKey={privateKey} />
        )}
        {activeTab === 'receive' && <ReceiveSection address={walletAddress} />}
        {activeTab === 'history' && <TransactionHistory address={walletAddress} />}
  
        {activeTab === 'contracts' && (<ContractManager address={walletAddress} privateKey={privateKey} />)}
        {activeTab === 'liquidity' && (
       <LiquidityDashboard address={walletAddress} />)}
        {activeTab === 'bridge' && (
          <BridgePanel lqdAddress={walletAddress} lqdPrivateKey={privateKey} />
        )}
        {activeTab === 'settings' && (
          <div style={{ display: 'grid', gap: 16 }}>
            <div className="balance-card">
              <h3>Wallet Backup</h3>
              <p style={{ color: '#6b7280', marginTop: 0 }}>
                Export or import encrypted wallet metadata and the local token watchlist.
              </p>
              <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap' }}>
                <button className="btn-primary" style={{ width: 'auto' }} onClick={exportBackup}>
                  Export Backup
                </button>
                <button className="btn-secondary" style={{ width: 'auto' }} onClick={() => backupInputRef.current?.click()}>
                  Import Backup
                </button>
                <button className="btn-secondary" style={{ width: 'auto' }} onClick={() => navigator.clipboard.writeText(walletAddress)}>
                  Copy Address
                </button>
              </div>
              <input
                ref={backupInputRef}
                type="file"
                accept="application/json"
                style={{ display: 'none' }}
                onChange={async (e) => {
                  const file = e.target.files?.[0];
                  try {
                    await importBackupFile(file);
                  } catch (err) {
                    setBackupMessage(err.message || 'Backup import failed');
                  } finally {
                    e.target.value = '';
                  }
                }}
              />
              {backupMessage && <div style={{ marginTop: 12, color: '#166534' }}>{backupMessage}</div>}
            </div>

            <div className="balance-card">
              <h3>Wallet Details</h3>
              <div className="balance-details" style={{ marginTop: 8 }}>
                <div className="balance-item">
                  <span>Address</span>
                  <span style={{ wordBreak: 'break-all' }}>{walletAddress}</span>
                </div>
                <div className="balance-item">
                  <span>Token watchlist key</span>
                  <span style={{ wordBreak: 'break-all' }}>{`${TOKENS_STORAGE_KEY}_${walletAddress.toLowerCase()}`}</span>
                </div>
                <div className="balance-item">
                  <span>Recovery</span>
                  <span>Encrypted local backup</span>
                </div>
              </div>
            </div>
          </div>
        )}

      </div>
    </div>
  );
};

export default WalletDashboard;



// import React, { useState, useEffect, useCallback, useRef } from 'react';
// import { useNavigate } from 'react-router-dom';
// import WalletLogin from './WalletLogin';
// import WalletBalance from './WalletBalance';
// import SendTransaction from './SendTransaction';
// import ReceiveSection from './ReceiveSection';
// import TransactionHistory from './TransactionHistory';
// import './Wallet.css';

// const STORAGE_KEY = 'liquidityChainWallet';
// const SESSION_KEY = 'liquidityChainWalletSession'; // 🔹 per-tab unlocked session

// // Helpers for encrypting/decrypting private key using Web Crypto
// const enc = new TextEncoder();
// const dec = new TextDecoder();

// async function deriveKey(password, saltBytes) {
//   const keyMaterial = await window.crypto.subtle.importKey(
//     'raw',
//     enc.encode(password),
//     'PBKDF2',
//     false,
//     ['deriveKey']
//   );

//   return window.crypto.subtle.deriveKey(
//     {
//       name: 'PBKDF2',
//       salt: saltBytes,
//       iterations: 250000,
//       hash: 'SHA-256',
//     },
//     keyMaterial,
//     { name: 'AES-GCM', length: 256 },
//     false,
//     ['encrypt', 'decrypt']
//   );
// }

// function b64encode(bytes) {
//   return btoa(String.fromCharCode(...new Uint8Array(bytes)));
// }

// function b64decode(str) {
//   return Uint8Array.from(atob(str), c => c.charCodeAt(0));
// }

// async function encryptPrivateKey(password, privateKey) {
//   const salt = window.crypto.getRandomValues(new Uint8Array(16));
//   const iv   = window.crypto.getRandomValues(new Uint8Array(12));

//   const key = await deriveKey(password, salt);
//   const ciphertext = await window.crypto.subtle.encrypt(
//     { name: 'AES-GCM', iv },
//     key,
//     enc.encode(privateKey)
//   );

//   return {
//     encryptedPrivateKey: b64encode(ciphertext),
//     salt: b64encode(salt),
//     iv: b64encode(iv),
//   };
// }

// async function decryptPrivateKey(password, encryptedBundle) {
//   const { encryptedPrivateKey, salt, iv } = encryptedBundle;
//   const saltBytes = b64decode(salt);
//   const ivBytes   = b64decode(iv);
//   const cipherBytes = b64decode(encryptedPrivateKey);

//   const key = await deriveKey(password, saltBytes);
//   const plain = await window.crypto.subtle.decrypt(
//     { name: 'AES-GCM', iv: ivBytes },
//     key,
//     cipherBytes
//   );

//   return dec.decode(plain);
// }

// const WalletDashboard = () => {
//   const [activeTab, setActiveTab] = useState('balance');
//   const [walletAddress, setWalletAddress] = useState('');
//   const [privateKey, setPrivateKey] = useState('');
//   const [isWalletLoaded, setIsWalletLoaded] = useState(false);
//   const [savedWalletMeta, setSavedWalletMeta] = useState(null);
//   const [unlockPassword, setUnlockPassword] = useState('');
//   const [unlockError, setUnlockError] = useState('');
//   const navigate = useNavigate();

//   // --------------------------------------------------
//   // 🔐 AUTO-LOCK HOOKS (always at top, non-conditional)
//   // --------------------------------------------------
//   const INACTIVITY_LIMIT = 60 * 1000; // 1 minute
//   const inactivityRef = useRef(null);

//   // 🔹 Lock only this session (keep encrypted wallet)
//   const lockWallet = useCallback(() => {
//     setPrivateKey('');
//     setIsWalletLoaded(false);
//     setUnlockPassword('');
//     setUnlockError('');
//     sessionStorage.removeItem(SESSION_KEY);
//   }, []);

//   // 🔹 Full disconnect + forget wallet (manual)
//   const disconnectWallet = useCallback(() => {
//     setWalletAddress('');
//     setPrivateKey('');
//     setIsWalletLoaded(false);
//     setSavedWalletMeta(null);
//     setUnlockPassword('');
//     setUnlockError('');
//     localStorage.removeItem(STORAGE_KEY);
//     sessionStorage.removeItem(SESSION_KEY);
//   }, []);

//   const resetInactivityTimer = useCallback(() => {
//     if (!isWalletLoaded) return; // only run when wallet unlocked

//     if (inactivityRef.current) clearTimeout(inactivityRef.current);

//     inactivityRef.current = setTimeout(() => {
//       console.log("Auto-lock: wallet locked due to inactivity.");
//       lockWallet();
//     }, INACTIVITY_LIMIT);
//   }, [isWalletLoaded, lockWallet]);


//   useEffect(() => {
//     if (!isWalletLoaded) return; // only active when unlocked

//     resetInactivityTimer();

//     const events = ["mousemove", "keydown", "click", "touchstart"];
//     const handler = () => resetInactivityTimer();

//     events.forEach(evt => window.addEventListener(evt, handler));

//     return () => {
//       if (inactivityRef.current) clearTimeout(inactivityRef.current);
//       events.forEach(evt => window.removeEventListener(evt, handler));
//     };
//   }, [isWalletLoaded, resetInactivityTimer]);
//   // --------------------------------------------------

//   // Load encrypted wallet metadata OR active session from storage on mount
//   useEffect(() => {
//     // 1) Check unlocked session first (so reload keeps wallet unlocked)
//     const session = sessionStorage.getItem(SESSION_KEY);
//     if (session) {
//       try {
//         const parsed = JSON.parse(session);
//         if (parsed.address && parsed.privateKey) {
//           setWalletAddress(parsed.address);
//           setPrivateKey(parsed.privateKey);
//           setIsWalletLoaded(true);
//           return; // already fully unlocked, skip to dashboard
//         }
//       } catch (e) {
//         console.error('Failed to parse session wallet:', e);
//         sessionStorage.removeItem(SESSION_KEY);
//       }
//     }

//     // 2) Fallback: load encrypted wallet (locked) from localStorage
//     const saved = localStorage.getItem(STORAGE_KEY);
//     if (saved) {
//       try {
//         const parsed = JSON.parse(saved);

//         // Reject old insecure format
//         if (parsed.privateKey && !parsed.encryptedPrivateKey) {
//           console.warn('Found legacy insecure wallet; forcing reset.');
//           localStorage.removeItem(STORAGE_KEY);
//         } else {
//           setSavedWalletMeta(parsed);
//           setWalletAddress(parsed.address || '');
//         }
//       } catch (e) {
//         console.error('Failed to parse saved wallet:', e);
//         localStorage.removeItem(STORAGE_KEY);
//       }
//     }
//   }, []);

//   const persistEncryptedWallet = (address, encryptedBundle) => {
//     const toStore = {
//       address,
//       encryptedPrivateKey: encryptedBundle.encryptedPrivateKey,
//       salt: encryptedBundle.salt,
//       iv: encryptedBundle.iv,
//     };
//     localStorage.setItem(STORAGE_KEY, JSON.stringify(toStore));
//     setSavedWalletMeta(toStore);
//   };

//   const handleWalletCreate = async (walletData, password) => {
//     try {
//       const bundle = await encryptPrivateKey(password, walletData.private_key);
//       persistEncryptedWallet(walletData.address, bundle);

//       setWalletAddress(walletData.address);
//       setPrivateKey(walletData.private_key);
//       setIsWalletLoaded(true);
//       setUnlockPassword('');
//       setUnlockError('');

//       // 🔹 also store unlocked session so reload stays logged in
//       sessionStorage.setItem(
//         SESSION_KEY,
//         JSON.stringify({ address: walletData.address, privateKey: walletData.private_key })
//       );
//     } catch {
//       setUnlockError('Failed to securely store wallet.');
//     }
//   };

//   const handleWalletImport = async (walletData, password) => {
//     try {
//       const bundle = await encryptPrivateKey(password, walletData.private_key);
//       persistEncryptedWallet(walletData.address, bundle);

//       setWalletAddress(walletData.address);
//       setPrivateKey(walletData.private_key);
//       setIsWalletLoaded(true);
//       setUnlockPassword('');
//       setUnlockError('');

//       // 🔹 unlocked session
//       sessionStorage.setItem(
//         SESSION_KEY,
//         JSON.stringify({ address: walletData.address, privateKey: walletData.private_key })
//       );
//     } catch {
//       setUnlockError('Failed to securely import wallet.');
//     }
//   };

//   const handleUnlock = async () => {
//     if (!savedWalletMeta) return;
//     setUnlockError('');

//     try {
//       const pk = await decryptPrivateKey(unlockPassword, savedWalletMeta);
//       setPrivateKey(pk);
//       setWalletAddress(savedWalletMeta.address);
//       setIsWalletLoaded(true);
//       setUnlockPassword('');

//       // 🔹 unlocked session persists across reloads
//       sessionStorage.setItem(
//         SESSION_KEY,
//         JSON.stringify({ address: savedWalletMeta.address, privateKey: pk })
//       );
//     } catch {
//       setUnlockError('Incorrect password or corrupted wallet data.');
//     }
//   };

//   const viewOnExplorer = () => {
//     if (walletAddress) navigate(`/address/${walletAddress}`);
//   };

//   // --------------------------------------------------
//   // 1) No saved wallet → Show create/import page
//   // --------------------------------------------------
//   if (!savedWalletMeta && !isWalletLoaded) {
//     return (
//       <WalletLogin 
//         onWalletCreate={handleWalletCreate}
//         onWalletImport={handleWalletImport}
//       />
//     );
//   }

//   // --------------------------------------------------
//   // 2) Wallet saved but locked → Unlock screen
//   // --------------------------------------------------
//   if (savedWalletMeta && !isWalletLoaded) {
//     return (
//       <div className="wallet-login">
//         <div className="login-container">
//           <h2>Unlock Wallet</h2>
//           <p>Enter your password to decrypt your wallet.</p>

//           {unlockError && <div className="error-message">{unlockError}</div>}

//           <div className="form-group">
//             <label>Address:</label>
//             <input type="text" value={savedWalletMeta.address} readOnly className="readonly" />
//           </div>

//           <div className="form-group">
//             <label>Password:</label>
//             <input
//               type="password"
//               value={unlockPassword}
//               onChange={(e) => setUnlockPassword(e.target.value)}
//               placeholder="Enter your wallet password"
//             />
//           </div>

//           <button className="btn-primary" onClick={handleUnlock}>
//             Unlock
//           </button>

//           <button className="btn-secondary" onClick={disconnectWallet} style={{ marginTop: 10 }}>
//             Forget Wallet
//           </button>
//         </div>
//       </div>
//     );
//   }

//   // --------------------------------------------------
//   // 3) Wallet unlocked → Dashboard UI
//   // --------------------------------------------------
//   return (
//     <div className="wallet-dashboard">
//       <div className="wallet-header">
//         <div className="wallet-info">
//           <h2>My Wallet</h2>
//           <div className="wallet-address">
//             <strong>Address:</strong>
//             <span className="address">{walletAddress}</span>
//             <button className="btn-copy" onClick={() => navigator.clipboard.writeText(walletAddress)}>
//               Copy
//             </button>
//             <button className="btn-secondary" onClick={viewOnExplorer}>
//               View on Explorer
//             </button>
//           </div>
//         </div>

//         <button className="btn-disconnect" onClick={disconnectWallet}>
//           Disconnect
//         </button>
//       </div>

//       <div className="wallet-tabs">
//         <button className={`tab ${activeTab === 'balance' ? 'active' : ''}`} onClick={() => setActiveTab('balance')}>
//           Balance
//         </button>
//         <button className={`tab ${activeTab === 'send' ? 'active' : ''}`} onClick={() => setActiveTab('send')}>
//           Send
//         </button>
//         <button className={`tab ${activeTab === 'receive' ? 'active' : ''}`} onClick={() => setActiveTab('receive')}>
//           Receive
//         </button>
//         <button className={`tab ${activeTab === 'history' ? 'active' : ''}`} onClick={() => setActiveTab('history')}>
//           History
//         </button>
//       </div>

//       <div className="wallet-content">
//         {activeTab === 'balance' && (
//           <WalletBalance address={walletAddress} privateKey={privateKey} />
//         )}
//         {activeTab === 'send' && (
//           <SendTransaction fromAddress={walletAddress} privateKey={privateKey} />
//         )}
//         {activeTab === 'receive' && <ReceiveSection address={walletAddress} />}
//         {activeTab === 'history' && <TransactionHistory address={walletAddress} />}
//       </div>
//     </div>
//   );
// };

// export default WalletDashboard;
