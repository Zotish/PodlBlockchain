
import React, { useState } from 'react';

// Simple password strength validator
function validatePasswordStrength(pw) {
  if (!pw || pw.length < 10) {
    return 'Password must be at least 10 characters long';
  }
  if (!/[a-z]/.test(pw)) {
    return 'Password must contain at least one lowercase letter';
  }
  if (!/[A-Z]/.test(pw)) {
    return 'Password must contain at least one uppercase letter';
  }
  if (!/[0-9]/.test(pw)) {
    return 'Password must contain at least one number';
  }
  if (!/[!@#$%^&*()_\-+=\[\]{};:"\\|,.<>/?]/.test(pw)) {
    return 'Password must contain at least one special character';
  }
  return '';
}

const WalletLogin = ({ onWalletCreate, onWalletImport }) => {
  const [activeTab, setActiveTab] = useState('create');
  const [password, setPassword] = useState('');
  const [mnemonic, setMnemonic] = useState('');
  const [privateKey, setPrivateKey] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const handleCreateWallet = async () => {
    const pwError = validatePasswordStrength(password);
    if (pwError) {
      setError(pwError);
      return;
    }

    setLoading(true);
    setError('');

    try {
      // 🔥 changed (8080 → 5000)
      const response = await fetch('http://127.0.0.1:9000/wallet/new', { 
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ password }),
      });

      if (!response.ok) {
        throw new Error('Failed to create wallet');
      }

      const walletData = await response.json();
      onWalletCreate(walletData, password);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const handleImportFromMnemonic = async () => {
    if (!mnemonic) {
      setError('Mnemonic is required');
      return;
    }
    const pwError = validatePasswordStrength(password);
    if (pwError) {
      setError(pwError);
      return;
    }

    setLoading(true);
    setError('');

    try {
      // 🔥 changed (8080 → 5000)
      const response = await fetch('http://127.0.0.1:9000/wallet/import/mnemonic', { 
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ mnemonic, password }),
      });

      if (!response.ok) {
        throw new Error('Failed to import wallet from mnemonic');
      }

      const walletData = await response.json();
      onWalletImport(walletData, password);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const handleImportFromPrivateKey = async () => {
    if (!privateKey) {
      setError('Private key is required');
      return;
    }
    const pwError = validatePasswordStrength(password);
    if (pwError) {
      setError(pwError);
      return;
    }

    setLoading(true);
    setError('');

    try {
      // 🔥 changed (8080 → 5000)
      const response = await fetch('http://127.0.0.1:9000/wallet/import/private-key', { 
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ private_key: privateKey }),
      });

      if (!response.ok) {
        throw new Error('Failed to import wallet from private key');
      }

      const walletData = await response.json();
      onWalletImport(walletData, password);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="wallet-login">
      <div className="login-container">
        <h2>LiquidityChain Wallet</h2>
        
        <div className="login-tabs">
           <button 
            className={`tab ${activeTab === 'create' ? 'active' : ''}`}
            onClick={() => setActiveTab('create')}
          >
            Create Wallet
          </button>
          <button 
            className={`tab ${activeTab === 'import-mnemonic' ? 'active' : ''}`}
            onClick={() => setActiveTab('import-mnemonic')}
          >
            Import from Mnemonic
          </button>
          <button 
            className={`tab ${activeTab === 'import-privatekey' ? 'active' : ''}`}
            onClick={() => setActiveTab('import-privatekey')}
          >
            Import from Private Key
          </button>
        </div>

        <div className="login-content">
          {error && <div className="error-message">{error}</div>}

          {activeTab === 'create' && (
            <div className="create-wallet">
              <h3>Create New Wallet</h3>
              <p>Create a new wallet with a secure password</p>
              <div className="form-group">
                <label>Password:</label>
                <input
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder="Enter strong password"
                />
                <small>
                  Min 10 chars, with upper, lower, number &amp; symbol
                </small>
              </div>
              <button 
                onClick={handleCreateWallet}
                disabled={loading}
                className="btn-primary"
              >
                {loading ? 'Creating...' : 'Create Wallet'}
              </button>
            </div>
          )}

          {activeTab === 'import-mnemonic' && (
            <div className="import-mnemonic">
              <h3>Import from Mnemonic</h3>
              <div className="form-group">
                <label>Mnemonic Phrase:</label>
                <textarea
                  value={mnemonic}
                  onChange={(e) => setMnemonic(e.target.value)}
                  placeholder="Enter your 25-word mnemonic phrase"
                  rows="3"
                />
              </div>
              <div className="form-group">
                <label>Password:</label>
                <input
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder="Enter strong password"
                />
                <small>
                  This password will encrypt your wallet locally.
                </small>
              </div>
              <button 
                onClick={handleImportFromMnemonic}
                disabled={loading}
                className="btn-primary"
              >
                {loading ? 'Importing...' : 'Import Wallet'}
              </button>
            </div>
          )}

          {activeTab === 'import-privatekey' && (
            <div className="import-privatekey">
              <h3>Import from Private Key</h3>
              <div className="form-group">
                <label>Private Key:</label>
                <input
                  type="text"
                  value={privateKey}
                  onChange={(e) => setPrivateKey(e.target.value)}
                  placeholder="Enter your private key"
                />
              </div>
              <div className="form-group">
                <label>Encryption Password:</label>
                <input
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder="Set strong password for this wallet"
                />
                <small>
                  This password encrypts your key in the browser (like MetaMask vault).
                </small>
              </div>
              <button 
                onClick={handleImportFromPrivateKey}
                disabled={loading}
                className="btn-primary"
              >
                {loading ? 'Importing...' : 'Import Wallet'}
              </button>
            </div>
          )}
        </div>

      </div>
    </div>
  );
};

export default WalletLogin;
