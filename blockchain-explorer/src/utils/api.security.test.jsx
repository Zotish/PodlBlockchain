import { afterEach, expect, test, vi } from 'vitest';
import { clearRequestCache, fetchJSON, requestCacheStats } from './api';

afterEach(() => { clearRequestCache(); vi.useRealTimers(); vi.unstubAllGlobals(); });
const response = (data = {}) => ({ ok: true, json: async () => data });

test('deadline cancels fetch even with a caller signal', async () => {
  vi.useFakeTimers();
  let signal;
  vi.stubGlobal('fetch', vi.fn((_, opts) => { signal = opts.signal; return new Promise(() => {}); }));
  const caller = new AbortController();
  const pending = fetchJSON('/deadline', { signal: caller.signal, timeoutMs: 5 });
  const assertion = expect(pending).rejects.toThrow('timed out');
  await vi.advanceTimersByTimeAsync(10);
  await assertion;
  expect(signal.aborted).toBe(true);
  expect(caller.signal.aborted).toBe(false);
});

test('deadline covers response body and caller cancellation settles promptly', async () => {
  vi.useFakeTimers();
  vi.stubGlobal('fetch', vi.fn(async () => ({ ok: true, json: () => new Promise(() => {}) })));
  const pending = fetchJSON('/body', { timeoutMs: 5 });
  const assertion = expect(pending).rejects.toThrow('timed out');
  await vi.advanceTimersByTimeAsync(10);
  await assertion;
  const caller = new AbortController();
  const cancelled = fetchJSON('/cancel', { signal: caller.signal });
  const cancellation = expect(cancelled).rejects.toThrow();
  caller.abort();
  await cancellation;
});

test('LRU and idle expiry bound cached history', async () => {
  vi.useFakeTimers();
  vi.stubGlobal('fetch', vi.fn(async () => response({ value: 1 })));
  for (let i = 0; i < 140; i++) await fetchJSON(`/block/${i}`, { cacheTtlMs: 50 });
  expect(requestCacheStats().entries).toBe(128);
  await vi.advanceTimersByTimeAsync(60);
  expect(requestCacheStats()).toEqual({ entries: 0, bytes: 0 });
});

test('cache cannot leak responses across authorization or expose mutable cached objects', async () => {
  vi.stubGlobal('fetch', vi.fn(async () => response({ value: 1 })));
  const first = await fetchJSON('/cached', { cacheTtlMs: 50 });
  first.value = 99;
  expect((await fetchJSON('/cached', { cacheTtlMs: 50 })).value).toBe(1);
  await fetchJSON('/cached', { cacheTtlMs: 50, headers: { Authorization: 'one' } });
  await fetchJSON('/cached', { cacheTtlMs: 50, headers: { Authorization: 'two' } });
  expect(fetch).toHaveBeenCalledTimes(3);
});

test('oversize response rejected and aborted calls do not start fetch', async () => {
  vi.stubGlobal('fetch', vi.fn(async () => ({ ...response(), headers: new Headers({ 'content-length': '99999999' }) })));
  await expect(fetchJSON('/large')).rejects.toThrow('safety limit');
  const caller = new AbortController(); caller.abort();
  await expect(fetchJSON('/abort', { signal: caller.signal })).rejects.toThrow();
  expect(fetch).toHaveBeenCalledTimes(1);
});

test('deduplicated callers receive detached JSON objects', async () => {
  let finish;
  vi.stubGlobal('fetch',vi.fn(()=>new Promise(resolve=>{finish=resolve;})));
  const a=fetchJSON('/shared'),b=fetchJSON('/shared');
  finish(response({nested:{value:1}}));
  const [one,two]=await Promise.all([a,b]);one.nested.value=99;
  expect(two.nested.value).toBe(1);expect(fetch).toHaveBeenCalledTimes(1);
});
