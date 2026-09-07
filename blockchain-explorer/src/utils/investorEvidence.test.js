// @vitest-environment node
import { expect, test } from 'vitest';
import { Wallet, sha256, toUtf8Bytes } from 'ethers';
import { verifyInvestorEvidence } from './investorEvidence';

const wallet = new Wallet('0x' + '1'.padStart(64, '0'));
const trust = { chainId: 139, networkId: 'test-security', specHash: 'a'.repeat(64), validators: [wallet.address.toLowerCase()] };
async function fixture() {
  const payload = { domain: 'PODL-INVESTOR-EVIDENCE-V1', report_version: 1, chain_id: trust.chainId, network_id: trust.networkId, spec_hash: trust.specHash, height: 10, latest_block_hash: 'b'.repeat(64), state_root: 'c'.repeat(64), reported_at: 1000, metrics: { realized_business_revenue: '12345678901234567890123456789', unaudited_metrics: true } };
  const raw = JSON.stringify(payload);
  return { ...payload, payload_json: raw, payload_hash: sha256(toUtf8Bytes(raw)), signature: await wallet.signMessage(toUtf8Bytes(raw)), signer: wallet.address, verified: false };
}
test('verifies locally even when the server verified flag is false', async () => {
  const r = await fixture();
  expect(verifyInvestorEvidence(r, r.metrics, trust, 1010).verified).toBe(true);
});
test('never trusts a server flag, unknown validator or missing trust roots', async () => {
  const r = await fixture(); r.verified = true;
  expect(verifyInvestorEvidence(r, r.metrics, { ...trust, validators: [] }, 1010).verified).toBe(false);
  expect(verifyInvestorEvidence(r, r.metrics, { ...trust, validators: ['0x' + '2'.repeat(40)] }, 1010).verified).toBe(false);
  expect(verifyInvestorEvidence(r, r.metrics, {}, 1010).verified).toBe(false);
});
test('rejects replay, wrong chain/spec, stale and future reports', async () => {
  const r = await fixture();
  for (const t of [{ ...trust, chainId: 1 }, { ...trust, specHash: 'd'.repeat(64) }, { ...trust, networkId: 'other' }]) expect(verifyInvestorEvidence(r, r.metrics, t, 1010).verified).toBe(false);
  for (const now of [1091, 969]) expect(verifyInvestorEvidence(r, r.metrics, trust, now).verified).toBe(false);
});
test('rejects altered displayed metrics, envelope, payload and signature', async () => {
  const r = await fixture();
  expect(verifyInvestorEvidence(r, { ...r.metrics, unaudited_metrics: false }, trust, 1010).verified).toBe(false);
  for (const patch of [{ height: 11 }, { payload_json: r.payload_json.replace('1000', '1001') }, { signature: '0x' + '0'.repeat(130) }, { payload_hash: '0x' + '0'.repeat(64) }]) expect(verifyInvestorEvidence({ ...r, ...patch }, r.metrics, trust, 1010).verified).toBe(false);
});
