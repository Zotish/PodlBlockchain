// @vitest-environment node
import { test } from 'vitest';
import assert from 'node:assert/strict';
import { securityHeaders } from './security-policy.mjs';
test('strict script and frame policy with exact API origins', () => {
  const headers = securityHeaders({ VITE_API_BASE: 'https://example.org/api' });
  assert.match(headers['Content-Security-Policy'], /script-src 'self';/);
  assert.match(headers['Content-Security-Policy'], /frame-ancestors 'none'/);
  assert.match(headers['Content-Security-Policy'], /https:\/\/example.org(?: |;)/);
  assert.doesNotMatch(headers['Content-Security-Policy'], /unsafe-eval|script-src[^;]*unsafe-inline|connect-src[^;]*\*/);
  assert.equal(headers['X-Frame-Options'], 'DENY');
});
test('rejects insecure or credential-bearing build endpoints', () => {
  for (const endpoint of ['http://example.org', 'https://user:password@example.org', 'javascript:alert(1)']) assert.throws(() => securityHeaders({ VITE_CHAIN_BASE: endpoint }));
});
