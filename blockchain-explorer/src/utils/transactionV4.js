const address = (value) => {
  if (!/^0x[0-9a-fA-F]{40}$/.test(value || '')) throw new Error('Valid 20-byte address required');
  return value.toLowerCase();
};
const uint = (value, bits = 64) => {
  if (typeof value === 'number' && !Number.isSafeInteger(value)) throw new Error('Use a decimal string for large integers');
  const text = String(value ?? '0');
  if (!/^(0|[1-9][0-9]*)$/.test(text) || BigInt(text) >= (1n << BigInt(bits))) throw new Error(`Invalid uint${bits}`);
  return text;
};
const bytesHex = (base64) => {
  if (!base64) return '0x';
  if (typeof base64 !== 'string' || base64.length > 350000 || !/^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$/.test(base64)) throw new Error('Canonical base64 bytes required');
  return '0x' + [...atob(base64)].map((s) => s.charCodeAt(0).toString(16).padStart(2, '0')).join('');
};

// This module constructs intent only. Key handling/signing is delegated to an
// independently maintained local EIP-712 wallet, never an HTTP signing service.
export function transactionTypedDataV4(tx) {
  if (tx?.signature_version !== 4 || tx.is_system !== false) throw new Error('Ordinary signature-v4 transaction required');
  if (!/^(0x)?[a-fA-F0-9]{64}$/.test(tx.signature_domain || '')) throw new Error('Pinned ChainSpec hash required');
  if (!Array.isArray(tx.args) || tx.args.length > 256 || tx.args.some((a) => typeof a !== 'string')) throw new Error('String arguments required');
  if (typeof tx.is_contract !== 'boolean' || typeof tx.function !== 'string' || typeof tx.type !== 'string') throw new Error('Explicit transaction flags/type/function required');
  const chainId = uint(tx.chain_id);
  if (BigInt(chainId) > 0x7fffffffffffffffn) throw new Error('Unsupported chain ID');
  return {
    domain: { name: 'PoDL Transaction', version: '4', chainId, salt: '0x' + tx.signature_domain.replace(/^0x/, '').toLowerCase() },
    types: {
      Transaction: [
        ['from','address'], ['to','address'], ['value','uint256'], ['data','bytes'], ['gas','uint64'], ['gasPrice','uint64'], ['nonce','uint64'], ['chainId','uint64'], ['timestamp','uint64'], ['priorityFee','uint64'], ['isContract','bool'], ['function','string'], ['args','string[]'], ['txType','string'], ['extraData','bytes'], ['isSystem','bool'],
      ].map(([name,type]) => ({ name, type })),
    },
    primaryType: 'Transaction',
    message: { from: address(tx.from), to: address(tx.to), value: uint(tx.value,256), data: bytesHex(tx.data), gas: uint(tx.gas), gasPrice: uint(tx.gas_price), nonce: uint(tx.nonce), chainId, timestamp: uint(tx.timestamp), priorityFee: uint(tx.priority_fee), isContract: tx.is_contract, function: tx.function, args: [...tx.args], txType: tx.type, extraData: bytesHex(tx.extra_data), isSystem: false },
  };
}

export function signedTransactionV4(tx, signatureHex) {
  transactionTypedDataV4(tx);
  if (!/^0x[a-fA-F0-9]{130}$/.test(signatureHex || '')) throw new Error('Recoverable signature required');
  const raw = signatureHex.slice(2).match(/../g).map((s) => parseInt(s,16));
  if (raw[64] === 27 || raw[64] === 28) raw[64] -= 27;
  if (raw[64] !== 0 && raw[64] !== 1) throw new Error('Invalid signature recovery ID');
  return { ...tx, signature_domain: '0x' + tx.signature_domain.replace(/^0x/, '').toLowerCase(), sig: btoa(String.fromCharCode(...raw)) };
}
