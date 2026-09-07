import React, { useEffect, useRef, useState } from 'react';
import { parseUnits } from 'ethers';
import { fetchChainJSON } from '../../utils/api';
import { configuredEvidenceTrust } from '../../utils/investorEvidence';
import { signNativeIntent, verifyPreflight } from '../../utils/localSigner';
import { formatLQD } from '../../utils/lqdUnits';

export default function LocalTransfer({ address }) {
  const [to, setTo] = useState('');
  const [amount, setAmount] = useState('');
  const [review, setReview] = useState(null);
  const [approved, setApproved] = useState(false);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState('');
  const epoch = useRef(0);
  const mounted = useRef(true);
  useEffect(()=>{mounted.current=true;return()=>{mounted.current=false;epoch.current++;};},[]);
  const enabled = import.meta.env.VITE_ENABLE_TESTNET_SIGNING === 'true';
  const invalidate = () => { epoch.current++; setReview(null); setApproved(false); setMessage(''); };
  const simulate = async (event) => {
    event.preventDefault(); invalidate(); const requestEpoch = epoch.current; setBusy(true);
    try {
      if (!/^0x[a-fA-F0-9]{40}$/.test(to) || !/^\d+(\.\d{1,8})?$/.test(amount) || parseUnits(amount,8) <= 0n) throw new Error('Enter a valid recipient and positive LQD amount (up to 8 decimals)');
      const trust = configuredEvidenceTrust();
      if (!trust.specHash || !trust.networkId || !Number.isSafeInteger(trust.chainId)) throw new Error('Release-configured chain identity is required');
      const [status, nonce] = await Promise.all([fetchChainJSON('/v2/protocol/status'), fetchChainJSON(`/account/${address}/nonce`)]);
      const spec = status?.protocol?.chain_spec;
      if (spec?.protocol_version !== 4 || spec?.chain_id !== trust.chainId || spec?.network_id !== trust.networkId || status?.protocol?.chain_spec_hash !== trust.specHash) throw new Error('Node does not match the pinned security-v4 test network');
      const gasPrice = status.base_fee;
      if (!Number.isSafeInteger(gasPrice) || gasPrice <= 0 || !Number.isSafeInteger(nonce.nonce)) throw new Error('Exact fee or nonce unavailable');
      const tx = Object.freeze({ from: address, to, value: parseUnits(amount,8).toString(), data: '', gas: '21000', gas_price: String(gasPrice), nonce: String(nonce.nonce), chain_id: String(trust.chainId), timestamp: String(Math.floor(Date.now()/1000)), priority_fee: '0', is_contract: false, function: '', args: Object.freeze([]), type: 'transfer', extra_data: '', is_system: false, signature_version: 4, signature_domain: trust.specHash });
      const result = await fetchChainJSON('/v4/transactions/simulate', { method: 'POST', body: JSON.stringify(tx), headers: { 'Content-Type': 'application/json' } });
      verifyPreflight(tx,result);
      if (requestEpoch === epoch.current) setReview({ tx, preflight: result });
    } catch (error) { if (requestEpoch === epoch.current) setMessage(error.message); }
    finally { setBusy(false); }
  };
  const confirm = async () => {
    if (!approved || !review || busy) return;
    setBusy(true);
    const requestEpoch=epoch.current;
    try {
      if (review.tx.from.toLowerCase() !== address.toLowerCase()) throw new Error('Account changed; simulate again');
      const signed = await signNativeIntent(review.tx, review.preflight, window.ethereum, approved);
      if (!mounted.current || requestEpoch!==epoch.current) return;
      const result = await fetchChainJSON('/send_tx', { method:'POST', headers:{'Content-Type':'application/json'}, body: JSON.stringify(signed) });
      setMessage(`Submitted: ${result.tx_hash || 'pending'}. Submission is not finality.`);setReview(null);setApproved(false);
    } catch (error) { setMessage(error.message); setReview(null); setApproved(false); }
    finally { setBusy(false); }
  };
  if (!enabled) return <section className="wallet-settings-card"><h3>Local signing release gate</h3><p>Native-transfer signing is implemented for a pinned security-v4 test network. It remains disabled in this release until the operator verifies chain configuration and completes the local-wallet review. Public account viewing remains available.</p></section>;
  return <section className="wallet-settings-card"><h3>Send test LQD</h3><p>Local EIP-712 wallet approval only. Test funds only; native transfers only.</p>
    <form className="wallet-access-form" onSubmit={simulate}>
      <label className="premium-field"><span>Recipient</span><input value={to} disabled={busy} onChange={(e)=>{invalidate();setTo(e.target.value.trim());}} autoComplete="off" spellCheck="false" required /></label>
      <label className="premium-field"><span>Amount (LQD)</span><input value={amount} disabled={busy} onChange={(e)=>{invalidate();setAmount(e.target.value);}} inputMode="decimal" required /></label>
      <button type="submit" disabled={busy}>{busy ? 'Working…' : 'Simulate & review'}</button>
    </form>
    {review && <div className="wallet-detail-list"><p>To: <code>{review.tx.to}</code></p><p>Amount: {formatLQD(review.tx.value)} LQD · maximum debit including gas: {formatLQD(review.preflight.maximum_debit)} LQD</p><p>Nonce {review.tx.nonce} · chain {review.tx.chain_id}. Fees and state may change before inclusion.</p><label><input type="checkbox" checked={approved} disabled={busy} onChange={(e)=>setApproved(e.target.checked)} /> I checked the recipient, amount, network and maximum fee.</label><button type="button" disabled={!approved || busy} onClick={confirm}>Approve in local wallet & submit</button></div>}
    {message && <p role="status">{message}</p>}
  </section>;
}
