// @vitest-environment node
import { expect, test } from 'vitest';
import { Wallet, TypedDataEncoder } from 'ethers';
import { readFileSync } from 'node:fs';
import { localIntentDigest, signNativeIntent, verifyPreflight } from './localSigner';
import { transactionTypedDataV4 } from './transactionV4';

const wallet = new Wallet('0x' + '1'.padStart(64,'0'));
const fixture = () => ({ from: wallet.address, to: '0x'+'2'.repeat(40), value:'5', data:'', gas:'21000', gas_price:'10', nonce:'0', chain_id:'139', timestamp:String(Math.floor(Date.now()/1000)), priority_fee:'0', is_contract:false, function:'', args:[], type:'transfer', extra_data:'', is_system:false, signature_version:4, signature_domain:'a'.repeat(64) });
const preflight = (tx) => ({ success:true, kind:'native-transfer-admission-preflight', broadcast:false, intent_digest:localIntentDigest(tx), spec_hash:tx.signature_domain, maximum_debit:String(BigInt(tx.value)+BigInt(tx.gas)*BigInt(tx.gas_price)), expires_at:Math.floor(Date.now()/1000)+30 });
test('matches the go-ethereum V4 golden digest, including uint256 and Unicode',()=>{
  const tx={...fixture(),value:'9007199254740993',data:'AQI=',gas:'21032',nonce:'42',timestamp:'1000',priority_fee:'1',is_contract:true,function:'Call',args:['α','42'],type:'contract_call',extra_data:'Aw=='};
  const p=transactionTypedDataV4(tx);
  expect(TypedDataEncoder.hash(p.domain,p.types,p.message)).toBe('0xbd2a2b68ab4d0dd199f66d5a975316f9ab7c3ff72415818c8ed65e577e2beeab');
});
test('signs exactly the approved intent locally; never calls remote signing or sends value', async()=>{
  const tx=fixture(), methods=[];
  const provider={request:async({method,params})=>{
    methods.push(method);
    if(method==='eth_chainId')return '0x8b';
    if(method==='eth_accounts'||method==='eth_requestAccounts')return [wallet.address];
    if(method==='eth_signTypedData_v4'){const t=JSON.parse(params[1]);delete t.types.EIP712Domain;return wallet.signTypedData(t.domain,t.types,t.message);}
    throw new Error(`Unexpected provider method ${method}`);
  }};
  const signed=await signNativeIntent(tx,preflight(tx),provider,true);
  expect(Buffer.from(signed.sig,'base64').length).toBe(65);
  expect(methods).toContain('eth_signTypedData_v4');
  expect(methods).not.toContain('eth_sendTransaction');
  expect(tx.sig).toBeUndefined();
});
test('requires explicit approval, correct network, fresh simulation and exact debit', async()=>{
  const tx=fixture(), p=preflight(tx);let touched=false;
  await expect(signNativeIntent(tx,p,{request:async()=>{touched=true;}},false)).rejects.toThrow('approval');expect(touched).toBe(false);
  await expect(signNativeIntent(tx,p,{request:async()=> '0x1'},true)).rejects.toThrow('network');
  for(const patch of [{intent_digest:'0x'+'0'.repeat(64)},{maximum_debit:'1'},{expires_at:1},{broadcast:true}])expect(()=>verifyPreflight(tx,{...p,...patch})).toThrow();
});
test('rejects lossy numeric input and keeps explorer/SDK codecs byte-identical',()=>{
  expect(()=>transactionTypedDataV4({...fixture(),value:Number.MAX_SAFE_INTEGER+1})).toThrow('decimal string');
  expect(readFileSync(new URL('./transactionV4.js',import.meta.url),'utf8')).toBe(readFileSync(new URL('../../../sdk/javascript/src/transaction-v4.js',import.meta.url),'utf8'));
});
