// src/utils/lqdUnits.test.js
// Tests for LQD unit conversion utilities
// Uses Jest (react-scripts test environment)

import { parseHuman, isAmountParam } from './lqdUnits';

// ─────────────────────────────────────────────────────────────────────────────
// parseHuman — pure string version, no ethers dependency
// ─────────────────────────────────────────────────────────────────────────────

describe('parseHuman', () => {
  test('integer "1" → "100000000" (8 decimals)', () => {
    expect(parseHuman('1')).toBe('100000000');
  });

  test('integer "100" → "10000000000"', () => {
    expect(parseHuman('100')).toBe('10000000000');
  });

  test('"1.5" → "150000000"', () => {
    expect(parseHuman('1.5')).toBe('150000000');
  });

  test('"0.00000001" → "1" (1 satoshi)', () => {
    expect(parseHuman('0.00000001')).toBe('1');
  });

  test('"0" → "0"', () => {
    expect(parseHuman('0')).toBe('0');
  });

  test('empty string → "0"', () => {
    expect(parseHuman('')).toBe('0');
  });

  test('null → "0"', () => {
    expect(parseHuman(null)).toBe('0');
  });

  test('undefined → "0"', () => {
    expect(parseHuman(undefined)).toBe('0');
  });

  test('truncates fractional digits beyond decimals', () => {
    // "1.123456789" with 8 decimals → "112345678" (truncated, not rounded)
    expect(parseHuman('1.123456789')).toBe('112345678');
  });

  test('handles numeric input', () => {
    expect(parseHuman(5)).toBe('500000000');
  });

  test('"20" → "2000000000" (genesis block reward)', () => {
    expect(parseHuman('20')).toBe('2000000000');
  });

  test('"0.5" → "50000000"', () => {
    expect(parseHuman('0.5')).toBe('50000000');
  });

  test('"1000000" → "100000000000000" (1 million LQD)', () => {
    expect(parseHuman('1000000')).toBe('100000000000000');
  });
});

// ─────────────────────────────────────────────────────────────────────────────
// isAmountParam — detect token amount ABI parameters
// ─────────────────────────────────────────────────────────────────────────────

describe('isAmountParam', () => {
  test('amount parameter of type uint256 → true', () => {
    expect(isAmountParam({ name: 'amount', type: 'uint256' })).toBe(true);
  });

  test('value parameter of type uint128 → true', () => {
    expect(isAmountParam({ name: 'value', type: 'uint128' })).toBe(true);
  });

  test('supply parameter of type uint256 → true', () => {
    expect(isAmountParam({ name: 'totalSupply', type: 'uint256' })).toBe(true);
  });

  test('balance parameter → true', () => {
    expect(isAmountParam({ name: 'balance', type: 'uint256' })).toBe(true);
  });

  test('qty parameter → true', () => {
    expect(isAmountParam({ name: 'qty', type: 'uint256' })).toBe(true);
  });

  test('address parameter → false', () => {
    expect(isAmountParam({ name: 'to', type: 'address' })).toBe(false);
  });

  test('string parameter → false', () => {
    expect(isAmountParam({ name: 'name', type: 'string' })).toBe(false);
  });

  test('bool parameter → false', () => {
    expect(isAmountParam({ name: 'active', type: 'bool' })).toBe(false);
  });

  test('non-amount uint parameter → false', () => {
    expect(isAmountParam({ name: 'tokenId', type: 'uint256' })).toBe(false);
  });

  test('null input → false', () => {
    expect(isAmountParam(null)).toBe(false);
  });

  test('undefined input → false', () => {
    expect(isAmountParam(undefined)).toBe(false);
  });

  test('int256 amount → true (signed also matches)', () => {
    expect(isAmountParam({ name: 'amount', type: 'int256' })).toBe(true);
  });
});
