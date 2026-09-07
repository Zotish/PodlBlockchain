import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const dist=resolve(dirname(fileURLToPath(import.meta.url)),'../dist');
const code=readFileSync(resolve(dist,'server/index.js'),'utf8');
const {default:worker}=await import('data:text/javascript;base64,'+Buffer.from(code).toString('base64'));
assert.equal(typeof worker.fetch,'function');
const fetchAsset=async(request)=>new Response(request.url.endsWith('/index.html')?'app':'missing',{status:request.url.endsWith('/index.html')?200:404,headers:{'content-type':request.url.endsWith('/index.html')?'text/html':'text/plain'}});
for(const path of ['/investor','/wallet','/not-found.js']){
  const res=await worker.fetch(new Request('https://explorer.invalid'+path,{headers:{accept:path.endsWith('.js')?'application/javascript':'text/html'}}),{ASSETS:{fetch:fetchAsset}});
  assert.equal(res.status,path.endsWith('.js')?404:200);
  assert.match(res.headers.get('content-security-policy'),/script-src 'self';/);
  assert.equal(res.headers.get('x-frame-options'),'DENY');
  if(res.status===200)assert.equal(res.headers.get('cache-control'),'no-store');
}
assert.equal((await worker.fetch(new Request('https://explorer.invalid'),{})).status,503);
assert.equal((await worker.fetch(new Request('https://explorer.invalid/wallet',{method:'POST',headers:{accept:'text/html'}}),{ASSETS:{fetch:fetchAsset}})).status,404);
const html=readFileSync(resolve(dist,'index.html'),'utf8');
let imports=0;
for(const match of html.matchAll(/<(?:script|link)\b[^>]*(?:src|href)="(\/assets\/[^"?#]+)"[^>]*>/g)){
  const sri=createHash('sha384').update(readFileSync(resolve(dist,'.'+match[1]))).digest('base64');
  assert.ok(match[0].includes(`integrity="sha384-${sri}"`));imports++;
}
assert.ok(imports>=2,'script and style require integrity hashes');
const manifest=JSON.parse(readFileSync(resolve(dist,'release-integrity.json')));
assert.ok(manifest.sha256['server/index.js']);
for(const [path,hash] of Object.entries(manifest.sha256))assert.equal(createHash('sha256').update(readFileSync(resolve(dist,path))).digest('hex'),hash);
process.stdout.write(`PASS: generated Worker routes, CSP, frame denial, ${imports} SRI imports and ${Object.keys(manifest.sha256).length} release hashes\n`);
