// src/utils/api.test.js
// Tests for the API utility functions (pure / synchronous functions only)

import { apiUrl, nodeResults, firstNodeResult, mergeArrayResults } from './api';

// ─────────────────────────────────────────────────────────────────────────────
// apiUrl
// ─────────────────────────────────────────────────────────────────────────────

describe('apiUrl', () => {
  test('prepends / to path that lacks it', () => {
    expect(apiUrl('http://127.0.0.1:9000', 'blocks')).toBe('http://127.0.0.1:9000/blocks');
  });

  test('does not double-slash when path already starts with /', () => {
    expect(apiUrl('http://127.0.0.1:9000', '/blocks')).toBe('http://127.0.0.1:9000/blocks');
  });

  test('handles sub-paths correctly', () => {
    expect(apiUrl('http://127.0.0.1:9000', '/tx/0xabc')).toBe('http://127.0.0.1:9000/tx/0xabc');
  });

  test('empty path → base + "/"', () => {
    expect(apiUrl('http://127.0.0.1:9000', '')).toBe('http://127.0.0.1:9000/');
  });
});

// ─────────────────────────────────────────────────────────────────────────────
// nodeResults — extracts results from aggregator envelope or raw array
// ─────────────────────────────────────────────────────────────────────────────

describe('nodeResults', () => {
  test('returns array as-is', () => {
    const arr = [{ tx_hash: '0x1' }, { tx_hash: '0x2' }];
    expect(nodeResults(arr)).toEqual(arr);
  });

  test('extracts .result from each node', () => {
    const data = { nodes: [{ result: { a: 1 } }, { result: { b: 2 } }] };
    expect(nodeResults(data)).toEqual([{ a: 1 }, { b: 2 }]);
  });

  test('falls back to .summary if no .result', () => {
    const data = { nodes: [{ summary: { block_height: 10 } }] };
    expect(nodeResults(data)).toEqual([{ block_height: 10 }]);
  });

  test('filters out null/undefined nodes', () => {
    const data = { nodes: [{ result: null }, { result: { ok: true } }, { summary: undefined }] };
    expect(nodeResults(data)).toEqual([{ ok: true }]);
  });

  test('returns [] for null input', () => {
    expect(nodeResults(null)).toEqual([]);
  });

  test('returns [] for empty node list', () => {
    expect(nodeResults({ nodes: [] })).toEqual([]);
  });

  test('returns [] for non-matching object', () => {
    expect(nodeResults({ foo: 'bar' })).toEqual([]);
  });
});

// ─────────────────────────────────────────────────────────────────────────────
// firstNodeResult — returns first non-null result from aggregator
// ─────────────────────────────────────────────────────────────────────────────

describe('firstNodeResult', () => {
  test('returns raw array as-is', () => {
    const arr = [1, 2, 3];
    expect(firstNodeResult(arr)).toEqual(arr);
  });

  test('returns .result from first matching node', () => {
    const data = { nodes: [{ result: { block_height: 5 } }] };
    expect(firstNodeResult(data)).toEqual({ block_height: 5 });
  });

  test('falls back to .summary', () => {
    const data = { nodes: [{ summary: { validators: [] } }] };
    expect(firstNodeResult(data)).toEqual({ validators: [] });
  });

  test('returns null for nodes with no result', () => {
    const data = { nodes: [{ noResult: true }] };
    expect(firstNodeResult(data)).toBeNull();
  });

  test('returns raw data if not array and no nodes property', () => {
    const data = { block_height: 42 };
    expect(firstNodeResult(data)).toEqual({ block_height: 42 });
  });

  test('returns null for null input', () => {
    expect(firstNodeResult(null)).toBeNull();
  });
});

// ─────────────────────────────────────────────────────────────────────────────
// mergeArrayResults — flattens and deduplicates results by key
// ─────────────────────────────────────────────────────────────────────────────

describe('mergeArrayResults', () => {
  test('flattens nested array of results', () => {
    const data = [
      [{ tx_hash: '0x1' }, { tx_hash: '0x2' }],
      [{ tx_hash: '0x3' }],
    ];
    const result = mergeArrayResults(data);
    expect(result).toHaveLength(3);
  });

  test('deduplicates by key', () => {
    const data = [
      [{ tx_hash: '0x1', v: 'a' }],
      [{ tx_hash: '0x1', v: 'b' }], // duplicate key → last write wins
    ];
    const result = mergeArrayResults(data, 'tx_hash');
    expect(result).toHaveLength(1);
  });

  test('handles nodes envelope format', () => {
    const data = {
      nodes: [
        { result: [{ tx_hash: '0xA' }, { tx_hash: '0xB' }] },
        { result: [{ tx_hash: '0xC' }] },
      ],
    };
    const result = mergeArrayResults(data);
    expect(result).toHaveLength(3);
  });

  test('handles transactions sub-array in result', () => {
    const data = {
      nodes: [
        { result: { transactions: [{ tx_hash: '0x1' }] } },
      ],
    };
    const result = mergeArrayResults(data);
    expect(result).toHaveLength(1);
    expect(result[0].tx_hash).toBe('0x1');
  });

  test('returns [] for empty data', () => {
    expect(mergeArrayResults([])).toEqual([]);
  });

  test('returns [] for null data', () => {
    expect(mergeArrayResults(null)).toEqual([]);
  });

  test('deduplicates by block_number', () => {
    const data = [
      [{ block_number: 5, hash: 'a' }],
      [{ block_number: 5, hash: 'b' }],
    ];
    const result = mergeArrayResults(data, 'block_number');
    expect(result).toHaveLength(1);
  });
});
