import { loadTokens, saveTokens, upsertToken, NATIVE_LQD } from './storage';

beforeEach(() => {
  localStorage.clear();
});

test('loadTokens always includes native LQD first', () => {
  expect(loadTokens()).toEqual([NATIVE_LQD]);

  saveTokens([{ address: '0xabc', symbol: 'ABC', decimals: '8' }]);
  expect(loadTokens()).toEqual([
    NATIVE_LQD,
    { address: '0xabc', symbol: 'ABC', decimals: '8' },
  ]);
});

test('upsertToken adds or updates non-native tokens without removing native LQD', () => {
  const inserted = upsertToken({ address: '0xabc', symbol: 'ABC', decimals: '8' });
  expect(inserted[0]).toEqual(NATIVE_LQD);
  expect(inserted[1]).toEqual({ address: '0xabc', symbol: 'ABC', decimals: '8' });

  const updated = upsertToken({ address: '0xabc', symbol: 'ABX', decimals: '18' });
  expect(updated).toEqual([
    NATIVE_LQD,
    { address: '0xabc', symbol: 'ABX', decimals: '18' },
  ]);
});
