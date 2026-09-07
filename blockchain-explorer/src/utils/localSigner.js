import { BrowserProvider, TypedDataEncoder, verifyTypedData } from 'ethers';
import { transactionTypedDataV4, signedTransactionV4 } from './transactionV4';

export function localIntentDigest(tx) {
  const typed = transactionTypedDataV4(tx);
  return TypedDataEncoder.hash(typed.domain, typed.types, typed.message);
}
export function verifyPreflight(tx, result, nowSeconds = Date.now()/1000) {
  if (!result?.success || result.kind !== 'native-transfer-admission-preflight' || result.broadcast !== false || result.intent_digest !== localIntentDigest(tx) || result.spec_hash !== tx.signature_domain || !Number.isSafeInteger(result.expires_at) || result.expires_at < nowSeconds || result.expires_at > nowSeconds + 35) throw new Error('Preflight does not match this transaction or has expired');
  const debit = BigInt(tx.value) + BigInt(tx.gas) * BigInt(tx.gas_price);
  if (String(result.maximum_debit) !== String(debit)) throw new Error('Preflight debit mismatch');
  return result;
}
export async function signNativeIntent(tx, preflight, injectedProvider, approved) {
  if (approved !== true) throw new Error('Explicit transaction approval required');
  if (!injectedProvider?.request) throw new Error('Install a local EIP-712 wallet to sign test transactions');
  verifyPreflight(tx, preflight);
  const provider = new BrowserProvider(injectedProvider);
  const chainId = await injectedProvider.request({ method: 'eth_chainId' });
  if (BigInt(chainId) !== BigInt(tx.chain_id)) throw new Error('Select the PoDL test network in your local wallet first');
  const accounts = await injectedProvider.request({ method: 'eth_requestAccounts' });
  if (!accounts?.some((a) => a.toLowerCase() === tx.from.toLowerCase())) throw new Error('Local wallet does not control the watched account');
  const signer = await provider.getSigner(tx.from);
  const typed = transactionTypedDataV4(tx);
  const signature = await signer.signTypedData(typed.domain, typed.types, typed.message);
  if (verifyTypedData(typed.domain, typed.types, typed.message, signature).toLowerCase() !== tx.from.toLowerCase()) throw new Error('Wallet signature does not match the approved intent');
  verifyPreflight(tx, preflight);
  return signedTransactionV4(tx, signature);
}
