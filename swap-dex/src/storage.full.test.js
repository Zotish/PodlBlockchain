// swap-dex/src/storage.full.test.js
// Full coverage for DEX token storage utilities

import { loadTokens, saveTokens, upsertToken, NATIVE_LQD } from './storage';

beforeEach(() => {
  localStorage.clear();
});

// ─────────────────────────────────────────────────────────────────────────────
// NATIVE_LQD constant
// ─────────────────────────────────────────────────────────────────────────────

describe('NATIVE_LQD', () => {
  test('has address "lqd"', () => {
    expect(NATIVE_LQD.address).toBe('lqd');
  });

  test('has name "LQD Coin"', () => {
    expect(NATIVE_LQD.name).toBe('LQD Coin');
  });

  test('has symbol "LQD"', () => {
    expect(NATIVE_LQD.symbol).toBe('LQD');
  });

  test('has 8 decimals', () => {
    expect(NATIVE_LQD.decimals).toBe('8');
  });

  test('is marked native', () => {
    expect(NATIVE_LQD.native).toBe(true);
  });
});

// ─────────────────────────────────────────────────────────────────────────────
// loadTokens
// ─────────────────────────────────────────────────────────────────────────────

describe('loadTokens', () => {
  test('returns [NATIVE_LQD] when localStorage is empty', () => {
    expect(loadTokens()).toEqual([NATIVE_LQD]);
  });

  test('native LQD is always first even with stored tokens', () => {
    saveTokens([{ address: '0xabc', symbol: 'ABC', decimals: '8' }]);
    const tokens = loadTokens();
    expect(tokens[0]).toEqual(NATIVE_LQD);
  });

  test('preserves stored tokens after native LQD', () => {
    saveTokens([{ address: '0xabc', symbol: 'ABC', decimals: '8' }]);
    const tokens = loadTokens();
    expect(tokens).toHaveLength(2);
    expect(tokens[1].symbol).toBe('ABC');
  });

  test('does not duplicate native LQD if stored list contains "lqd"', () => {
    saveTokens([NATIVE_LQD, { address: '0xabc', symbol: 'ABC', decimals: '8' }]);
    const tokens = loadTokens();
    const nativeLQDs = tokens.filter(t => t.address === 'lqd');
    expect(nativeLQDs).toHaveLength(1);
  });

  test('handles corrupted localStorage gracefully', () => {
    localStorage.setItem('lqd.swap.tokens', '{invalid json}');
    expect(loadTokens()).toEqual([NATIVE_LQD]);
  });

  test('handles non-array stored value gracefully', () => {
    localStorage.setItem('lqd.swap.tokens', '"string value"');
    expect(loadTokens()).toEqual([NATIVE_LQD]);
  });
});

// ─────────────────────────────────────────────────────────────────────────────
// saveTokens
// ─────────────────────────────────────────────────────────────────────────────

describe('saveTokens', () => {
  test('persists tokens to localStorage', () => {
    const tokens = [{ address: '0xabc', symbol: 'ABC', decimals: '8' }];
    saveTokens(tokens);
    const raw = localStorage.getItem('lqd.swap.tokens');
    expect(JSON.parse(raw)).toEqual(tokens);
  });

  test('overwrites previous data', () => {
    saveTokens([{ address: '0xold', symbol: 'OLD', decimals: '8' }]);
    saveTokens([{ address: '0xnew', symbol: 'NEW', decimals: '8' }]);
    const tokens = loadTokens();
    expect(tokens.find(t => t.symbol === 'OLD')).toBeUndefined();
    expect(tokens.find(t => t.symbol === 'NEW')).toBeDefined();
  });
});

// ─────────────────────────────────────────────────────────────────────────────
// upsertToken
// ─────────────────────────────────────────────────────────────────────────────

describe('upsertToken', () => {
  test('adds a new token', () => {
    const result = upsertToken({ address: '0xabc', symbol: 'ABC', decimals: '8' });
    expect(result).toHaveLength(2);
    expect(result[1].symbol).toBe('ABC');
  });

  test('native LQD is always first after upsert', () => {
    const result = upsertToken({ address: '0xabc', symbol: 'ABC', decimals: '8' });
    expect(result[0]).toEqual(NATIVE_LQD);
  });

  test('updates existing token by address (case-insensitive)', () => {
    upsertToken({ address: '0xabc', symbol: 'ABC', decimals: '8' });
    const result = upsertToken({ address: '0xABC', symbol: 'ABCV2', decimals: '18' });
    const abcTokens = result.filter(t => t.address.toLowerCase() === '0xabc');
    expect(abcTokens).toHaveLength(1);
    expect(abcTokens[0].symbol).toBe('ABCV2');
  });

  test('does not overwrite native LQD entry', () => {
    const result = upsertToken({ address: 'lqd', symbol: 'FAKE', decimals: '0' });
    expect(result[0]).toEqual(NATIVE_LQD);
    expect(result[0].symbol).toBe('LQD');
  });

  test('adding multiple tokens accumulates them', () => {
    upsertToken({ address: '0xaaa', symbol: 'AAA', decimals: '8' });
    upsertToken({ address: '0xbbb', symbol: 'BBB', decimals: '8' });
    const result = upsertToken({ address: '0xccc', symbol: 'CCC', decimals: '8' });
    expect(result).toHaveLength(4); // LQD + AAA + BBB + CCC
  });

  test('merge preserves extra fields on update', () => {
    upsertToken({ address: '0xabc', symbol: 'ABC', decimals: '8', name: 'Alpha' });
    const result = upsertToken({ address: '0xabc', symbol: 'ABCV2' });
    const tok = result.find(t => t.address === '0xabc');
    expect(tok.name).toBe('Alpha'); // preserved from first upsert
    expect(tok.symbol).toBe('ABCV2'); // updated
  });
});
