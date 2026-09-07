import { sha256, toUtf8Bytes, verifyMessage } from 'ethers';

const keys = ['domain', 'report_version', 'chain_id', 'network_id', 'spec_hash', 'height', 'latest_block_hash', 'state_root', 'reported_at', 'metrics'];
const hash = (value) => typeof value === 'string' && /^(0x)?[a-fA-F0-9]{64}$/.test(value);
const normalizeHash = (value) => String(value).replace(/^0x/, '').toLowerCase();
const canonical = (value) => {
  if (Array.isArray(value)) return `[${value.map(canonical).join(',')}]`;
  if (value && typeof value === 'object') return `{${Object.keys(value).sort().map((k) => `${JSON.stringify(k)}:${canonical(value[k])}`).join(',')}}`;
  if (typeof value === 'number' && (!Number.isFinite(value) || (Number.isInteger(value) && !Number.isSafeInteger(value)))) throw new Error('Unsafe numeric evidence');
  return JSON.stringify(value);
};

// Trust roots are release configuration, never values fetched from the same API
// whose statement is being checked. Missing roots deliberately fail closed.
export function configuredEvidenceTrust() {
  return {
    chainId: Number(import.meta.env.VITE_TRUSTED_CHAIN_ID),
    networkId: import.meta.env.VITE_TRUSTED_NETWORK_ID,
    specHash: import.meta.env.VITE_TRUSTED_SPEC_HASH,
    validators: (import.meta.env.VITE_TRUSTED_VALIDATORS || '').split(',').map((s) => s.trim().toLowerCase()).filter(Boolean),
  };
}

export function verifyInvestorEvidence(report, displayedMetrics, trust = configuredEvidenceTrust(), nowSeconds = Date.now() / 1000) {
  try {
    if (!Number.isSafeInteger(trust.chainId) || trust.chainId <= 0 || !trust.networkId || !hash(trust.specHash) || !trust.validators?.length) throw new Error('Trusted chain and validator configuration is required');
    if (!report || typeof report.payload_json !== 'string' || report.payload_json.length > 262144) throw new Error('Signed payload is unavailable');
    const payload = JSON.parse(report.payload_json);
    if (canonical(Object.keys(payload).sort()) !== canonical([...keys].sort())) throw new Error('Unexpected evidence fields');
    if (payload.domain !== 'PODL-INVESTOR-EVIDENCE-V1' || payload.report_version !== 1) throw new Error('Unsupported evidence domain');
    if (payload.chain_id !== trust.chainId || payload.network_id !== trust.networkId || normalizeHash(payload.spec_hash) !== normalizeHash(trust.specHash)) throw new Error('Evidence belongs to another chain');
    if (!Number.isSafeInteger(payload.height) || payload.height < 0 || !hash(payload.latest_block_hash) || !hash(payload.state_root)) throw new Error('Invalid checkpoint');
    if (!Number.isSafeInteger(payload.reported_at) || nowSeconds - payload.reported_at > 90 || payload.reported_at - nowSeconds > 30) throw new Error('Evidence is stale or future-dated');
    for (const key of keys) if (canonical(payload[key]) !== canonical(report[key])) throw new Error('Signed payload does not match report');
    if (canonical(payload.metrics) !== canonical(displayedMetrics)) throw new Error('Displayed metrics do not match signed metrics');
    const bytes = toUtf8Bytes(report.payload_json);
    if (sha256(bytes).toLowerCase() !== String(report.payload_hash).toLowerCase()) throw new Error('Evidence hash mismatch');
    const signer = verifyMessage(bytes, report.signature).toLowerCase();
    if (signer !== String(report.signer).toLowerCase() || !trust.validators.includes(signer)) throw new Error('Signer is not a trusted validator');
    return { verified: true, payload, signer, reason: 'Signature verified against release-configured trust roots' };
  } catch (error) {
    return { verified: false, reason: error.message || 'Evidence verification failed' };
  }
}
