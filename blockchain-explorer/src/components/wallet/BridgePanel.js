import React, { useMemo, useState, useEffect } from 'react';
import { BrowserProvider, Contract, parseUnits } from 'ethers';
import { fetchJSON } from '../../utils/api';
import { formatLQD } from './lqdUnits';

const BSC_CHAIN_ID = '0x61';

const DEFAULT_TOKENS = [
  { symbol: 'TST', name: 'Test Token', address: '0xede08551e0de19794359652b112a3b1f99e4c9f0', decimals: 8 },
  { symbol: 'USDT', name: 'USDT', address: '0xfd086bc7cd5c481dcc9c85ebe478a1c0b69fcbb9', decimals: 18 },
  { symbol: 'USDC', name: 'USDC', address: '0x64544969ed7EBf5f083679233325356EbE738930', decimals: 18 },
  { symbol: 'BUSD', name: 'BUSD', address: '0xed24fc36d5ee211ea25a80239fb8c4cfd80f12ee', decimals: 18 },
];

const ERC20_ABI = [
  'function allowance(address owner, address spender) view returns (uint256)',
  'function approve(address spender, uint256 amount) returns (bool)',
];

function toRawAmount(amountStr, decimals) {
  if (!amountStr) return '0';
  const [whole, frac = ''] = amountStr.split('.');
  const cleanFrac = frac.padEnd(decimals, '0').slice(0, decimals);
  const raw = `${whole || '0'}${cleanFrac}`.replace(/^0+/, '') || '0';
  return raw;
}

function normalizeHex(input) {
  if (input === null || input === undefined) return '';
  const s = String(input).trim();
  if (s === '') return '';
  if (s.startsWith('0x') || s.startsWith('0X')) return s;
  return `0x${s}`;
}

const BridgePanel = ({ lqdAddress, lqdPrivateKey }) => {
  const [bscAccount, setBscAccount] = useState('');
  const [bscStatus, setBscStatus] = useState('');
  const [lockStatus, setLockStatus] = useState('');
  const [burnStatus, setBurnStatus] = useState('');
  const [amountLock, setAmountLock] = useState('0');
  const [amountBurn, setAmountBurn] = useState('0');
  const [toLqd, setToLqd] = useState(lqdAddress || '');
  const [toBsc, setToBsc] = useState('');
  const [tokenAddr, setTokenAddr] = useState(DEFAULT_TOKENS[0].address);
  const [customToken, setCustomToken] = useState('');
  const [customDecimals, setCustomDecimals] = useState('18');
  const [useCustomToken, setUseCustomToken] = useState(false);
  const [useCustomBurn, setUseCustomBurn] = useState(false);
  const [customBurnToken, setCustomBurnToken] = useState('');
  const [customBurnDecimals, setCustomBurnDecimals] = useState('18');
  const [requests, setRequests] = useState([]);
  const [reqStatus, setReqStatus] = useState('');
  const [bridgeTokens, setBridgeTokens] = useState([]);

  const selectedToken = useMemo(() => {
    if (useCustomToken) {
      return { address: customToken, decimals: Number(customDecimals || 0), symbol: 'CUSTOM' };
    }
    return DEFAULT_TOKENS.find((t) => t.address.toLowerCase() === tokenAddr.toLowerCase()) || DEFAULT_TOKENS[0];
  }, [useCustomToken, customToken, customDecimals, tokenAddr]);

  const connectBsc = async () => {
    setBscStatus('');
    try {
      if (!window.ethereum) {
        setBscStatus('Metamask not found');
        return;
      }
      const chainId = await window.ethereum.request({ method: 'eth_chainId' });
      if (chainId !== BSC_CHAIN_ID) {
        await window.ethereum.request({
          method: 'wallet_switchEthereumChain',
          params: [{ chainId: BSC_CHAIN_ID }],
        });
      }
      const accounts = await window.ethereum.request({ method: 'eth_requestAccounts' });
      setBscAccount(accounts?.[0] || '');
      if (accounts?.[0]) {
        setToBsc(accounts[0]);
      }
      setBscStatus('Connected');
    } catch (err) {
      setBscStatus(err?.message || 'Failed to connect');
    }
  };

  const loadRequests = async () => {
    try {
      const data = await fetchJSON('/bridge/requests');
      const list = Array.isArray(data) ? data : [];
      setRequests(list);
      setReqStatus('');
    } catch (err) {
      setReqStatus(err?.message || 'Failed to load bridge requests');
    }
  };

  useEffect(() => {
    loadRequests();
  }, []);
  useEffect(() => {
    const loadBridgeTokens = async () => {
      try {
        const data = await fetchJSON('/bridge/tokens');
        setBridgeTokens(Array.isArray(data) ? data : []);
      } catch {
        setBridgeTokens([]);
      }
    };
    loadBridgeTokens();
  }, []);

  useEffect(() => {
    if (lqdAddress) setToLqd(lqdAddress);
  }, [lqdAddress]);

  useEffect(() => {
    if (bscAccount) setToBsc(bscAccount);
  }, [bscAccount]);

  const mappedToken = bridgeTokens.find(
    (t) => t?.bsc_token?.toLowerCase() === selectedToken.address?.toLowerCase()
  );
  const lqdTokenAddr = mappedToken?.lqd_token || '';
  const lqdTokenDecimals = Number(mappedToken?.decimals || selectedToken.decimals || 0);
  const burnTokenAddr = useCustomBurn ? customBurnToken : lqdTokenAddr;
  const burnTokenDecimals = useCustomBurn ? Number(customBurnDecimals || 0) : lqdTokenDecimals;

  const submitBscLock = async () => {
    setLockStatus('');
    if (!window.ethereum) {
      setLockStatus('Metamask not found');
      return;
    }
    if (!bscAccount) {
      setLockStatus('Connect BSC wallet first');
      return;
    }
    const token = selectedToken.address;
    if (!token) {
      setLockStatus('Token address required');
      return;
    }
    const rawAmount = toRawAmount(amountLock, selectedToken.decimals || 0);
    if (rawAmount === '0') {
      setLockStatus('Amount required');
      return;
    }
    const payload = {
      token,
      amount: rawAmount,
      to_lqd: toLqd || lqdAddress,
    };
    try {
      const txdata = await fetchJSON('/wallet/bridge/bsc_lock_tx', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      });

      const lockAddr = normalizeHex(txdata.to);
      const dataHex = normalizeHex(txdata.data);
      if (!lockAddr || !dataHex.startsWith('0x')) {
        throw new Error('Invalid lock tx data from server');
      }
      const fromAddr = normalizeHex(bscAccount);
      const toAddr = normalizeHex(lockAddr);
      const dataStr = normalizeHex(dataHex);
      if (!fromAddr || !toAddr || !dataStr.startsWith('0x')) {
        setLockStatus('Invalid tx params for MetaMask');
        return;
      }
      const provider = new BrowserProvider(window.ethereum);
      const signer = await provider.getSigner();

      // Auto-approve if allowance < amount
      try {
        const tokenContract = new Contract(normalizeHex(token), ERC20_ABI, signer);
        const needAmount = parseUnits(String(amountLock), selectedToken.decimals || 0);
        const allowance = await tokenContract.allowance(fromAddr, toAddr);
        if (allowance < needAmount) {
          setLockStatus('Approving token spend…');
          const approveTx = await tokenContract.approve(toAddr, needAmount);
          await approveTx.wait();
        }
      } catch (approveErr) {
        setLockStatus(approveErr?.message || 'Approval failed');
        return;
      }
      const txResp = await signer.sendTransaction({
        to: toAddr,
        data: dataStr,
        value: 0,
      });
      const txHash = txResp?.hash || '';
      // Register on LQD (fallback in case log scan misses)
      await fetchJSON('/bridge/lock_bsc', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          bsc_tx: txHash,
          token,
          from: bscAccount,
          to_lqd: toLqd || lqdAddress,
          amount: rawAmount,
        }),
      });
      setLockStatus(`BSC lock submitted: ${txHash}`);
      loadRequests();
    } catch (err) {
      setLockStatus(err?.message || 'BSC lock failed');
    }
  };

  const submitLqdBurn = async () => {
    setBurnStatus('');
    if (!lqdPrivateKey) {
      setBurnStatus('Unlock LQD wallet first');
      return;
    }
    if (!burnTokenAddr) {
      setBurnStatus('Select token');
      return;
    }
    const rawAmount = toRawAmount(amountBurn, burnTokenDecimals);
    if (rawAmount === '0') {
      setBurnStatus('Amount required');
      return;
    }
    if (!toBsc) {
      setBurnStatus('BSC destination required');
      return;
    }
    try {
      const res = await fetchJSON('/wallet/bridge/burn_lqd_token', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          private_key: lqdPrivateKey,
          token: burnTokenAddr,
          amount: rawAmount,
          to_bsc: toBsc,
        }),
      });
      const hash = res?.tx_hash || res?.TxHash || '';
      setBurnStatus(hash ? `Burn submitted: ${hash}` : 'Burn submitted');
      loadRequests();
    } catch (err) {
      setBurnStatus(err?.message || 'Burn failed');
    }
  };

  const walletFiltered = requests.filter((r) => {
    const lqdMatch =
      lqdAddress &&
      (r?.from?.toLowerCase() === lqdAddress.toLowerCase() ||
        r?.to?.toLowerCase() === lqdAddress.toLowerCase());
    const bscMatch =
      bscAccount &&
      (r?.from?.toLowerCase() === bscAccount.toLowerCase() ||
        r?.to?.toLowerCase() === bscAccount.toLowerCase());
    return lqdMatch || bscMatch;
  });

  return (
    <div className="contract-section">
      <h3>Bridge (LQD ↔ BSC Testnet)</h3>

      <div className="contract-card">
        <h4>Connect BSC Wallet</h4>
        <button className="btn-primary" onClick={connectBsc}>Connect Metamask</button>
        {bscAccount && <p><strong>BSC:</strong> {bscAccount}</p>}
        {bscStatus && <p>{bscStatus}</p>}
      </div>

      <div className="contract-card">
        <h4>BSC → LQD (Lock on BSC, Mint on LQD)</h4>
        <label>Token</label>
        <select value={useCustomToken ? 'custom' : tokenAddr} onChange={(e) => {
          if (e.target.value === 'custom') {
            setUseCustomToken(true);
          } else {
            setUseCustomToken(false);
            setTokenAddr(e.target.value);
          }
        }}>
          {DEFAULT_TOKENS.map((t) => (
            <option key={t.address} value={t.address}>{t.symbol} ({t.address.slice(0, 6)}…)</option>
          ))}
          <option value="custom">Custom Token</option>
        </select>

        {useCustomToken && (
          <>
            <label>Custom Token Address</label>
            <input value={customToken} onChange={(e) => setCustomToken(e.target.value)} />
            <label>Decimals</label>
            <input value={customDecimals} onChange={(e) => setCustomDecimals(e.target.value)} />
          </>
        )}

        <label>To (LQD address)</label>
        <input value={toLqd} onChange={(e) => setToLqd(e.target.value)} placeholder={lqdAddress} />

        <label>Amount</label>
        <input value={amountLock} onChange={(e) => setAmountLock(e.target.value)} />

        <button className="btn-primary" onClick={submitBscLock}>Lock on BSC</button>
        {lockStatus && <p>{lockStatus}</p>}
      </div>

      <div className="contract-card">
        <h4>LQD → BSC (Burn on LQD, Unlock on BSC)</h4>
        <label>BSC Destination</label>
        <input value={toBsc} onChange={(e) => setToBsc(e.target.value)} placeholder={bscAccount || '0x...'} />

        <label>Token</label>
        <select value={useCustomBurn ? 'custom' : selectedToken.address} onChange={(e) => {
          if (e.target.value === 'custom') {
            setUseCustomBurn(true);
          } else {
            setUseCustomBurn(false);
            setTokenAddr(e.target.value);
          }
        }}>
          {DEFAULT_TOKENS.map((t) => (
            <option key={t.address} value={t.address}>{t.symbol} ({t.address.slice(0, 6)}…)</option>
          ))}
          <option value="custom">Custom Token</option>
        </select>
        {useCustomBurn && (
          <>
            <label>LQD Token Address</label>
            <input value={customBurnToken} onChange={(e) => setCustomBurnToken(e.target.value)} />
            <label>Decimals</label>
            <input value={customBurnDecimals} onChange={(e) => setCustomBurnDecimals(e.target.value)} />
          </>
        )}
        {!useCustomBurn && !lqdTokenAddr && (
          <p>Token not mapped on LQD yet. Do a BSC→LQD lock once to create mapping.</p>
        )}

        <label>Amount</label>
        <input value={amountBurn} onChange={(e) => setAmountBurn(e.target.value)} />

        <button className="btn-primary" onClick={submitLqdBurn}>Burn on LQD</button>
        {burnStatus && <p>{burnStatus}</p>}
      </div>

      <div className="contract-card">
        <h4>Bridge History (This Wallet)</h4>
        <button className="btn-secondary" onClick={loadRequests}>Refresh</button>
        {reqStatus && <p>{reqStatus}</p>}
        <p><strong>Total Requests:</strong> {walletFiltered.length}</p>
        <div style={{ overflowX: 'auto' }}>
          <table className="tx-table">
            <thead>
              <tr>
                <th>ID</th>
                <th>From</th>
                <th>To</th>
                <th>Amount</th>
                <th>Status</th>
                <th>LQD Tx</th>
                <th>BSC Tx</th>
              </tr>
            </thead>
            <tbody>
              {walletFiltered.length === 0 && (
                <tr><td colSpan={7}>No bridge requests for this wallet</td></tr>
              )}
              {walletFiltered.map((r) => (
                <tr key={r.id}>
                  <td>{String(r.id).slice(0, 10)}…</td>
                  <td>{String(r.from || '').slice(0, 10)}…</td>
                  <td>{String(r.to || '').slice(0, 10)}…</td>
                  <td>{formatLQD(r.amount)}</td>
                  <td>{r.status}</td>
                  <td>
                    {r.lqd_tx_hash ? (
                      <a href={`/tx/${r.lqd_tx_hash}`} className="link-button">
                        {r.lqd_tx_hash.slice(0, 10)}…
                      </a>
                    ) : '—'}
                  </td>
                  <td>
                    {r.bsc_tx_hash ? (
                      <a
                        href={`https://testnet.bscscan.com/tx/${r.bsc_tx_hash}`}
                        className="link-button"
                        target="_blank"
                        rel="noreferrer"
                      >
                        {r.bsc_tx_hash.slice(0, 10)}…
                      </a>
                    ) : '—'}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
};

export default BridgePanel;
