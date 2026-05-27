const normalizeAddress = (value = '') => String(value || '').trim().toLowerCase();

export const shortWalletAddress = (value = '') => {
  const text = String(value || '');
  if (text.length <= 14) return text || 'Not connected';
  return `${text.slice(0, 6)}...${text.slice(-4)}`;
};

const getLQDProvider = () => {
  if (typeof window === 'undefined' || !window.lqd) {
    throw new Error('LQD extension wallet is not installed or not available in this browser.');
  }
  return window.lqd;
};

const extractAccount = (result) => {
  if (Array.isArray(result)) return result[0] || '';
  if (Array.isArray(result?.accounts)) return result.accounts[0] || '';
  if (Array.isArray(result?.result)) return result.result[0] || '';
  if (typeof result?.address === 'string') return result.address;
  if (typeof result === 'string') return result;
  return '';
};

export async function connectExtensionWallet() {
  const provider = getLQDProvider();
  const accounts = await provider.request({ method: 'lqd_connect' });
  const account = extractAccount(accounts);
  if (!account) throw new Error('No wallet account returned by extension.');
  return account;
}

export async function buildSignedClaimPayload(address) {
  const target = String(address || '').trim();
  if (!target) throw new Error('Select a liquidity provider address first.');

  const walletAddress = await connectExtensionWallet();
  if (normalizeAddress(walletAddress) !== normalizeAddress(target)) {
    throw new Error('Connected wallet must match the LP address before claiming rewards.');
  }

  const issuedAt = new Date().toISOString();
  const nonce = `${Date.now()}-${Math.random().toString(16).slice(2)}`;
  const message = [
    'PODL Liquidity Reward Claim',
    `Address: ${target}`,
    `Issued At: ${issuedAt}`,
    `Nonce: ${nonce}`,
    'Purpose: Claim or sync liquidity provider rewards',
  ].join('\n');

  return {
    address: target,
    wallet_address: walletAddress,
    message,
    signature: '',
    issued_at: issuedAt,
    nonce,
    provider: 'window.lqd',
  };
}
