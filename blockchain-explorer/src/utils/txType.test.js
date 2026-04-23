// src/utils/txType.test.js
// Tests for transaction type detection and labelling

import { detectTxType, getTxTypeLabel } from './txType';

// ─────────────────────────────────────────────────────────────────────────────
// detectTxType
// ─────────────────────────────────────────────────────────────────────────────

describe('detectTxType', () => {
  // ── null / undefined ──────────────────────────────────────────────────────
  test('null tx → "transfer"', () => {
    expect(detectTxType(null)).toBe('transfer');
  });

  test('undefined tx → "transfer"', () => {
    expect(detectTxType(undefined)).toBe('transfer');
  });

  // ── Native transfer ───────────────────────────────────────────────────────
  test('plain LQD transfer → "transfer"', () => {
    const tx = {
      from: '0x1111111111111111111111111111111111111111',
      to:   '0x2222222222222222222222222222222222222222',
      value: '100000000',
    };
    expect(detectTxType(tx)).toBe('transfer');
  });

  // ── Reward types ──────────────────────────────────────────────────────────
  test('type "reward" → "reward"', () => {
    expect(detectTxType({ type: 'reward', to: '0xabc' })).toBe('reward');
  });

  test('validator reward via type field → "reward_validator"', () => {
    expect(detectTxType({ type: 'validator_reward' })).toBe('reward_validator');
  });

  test('lp reward via type field → "reward_lp"', () => {
    expect(detectTxType({ type: 'lp_reward' })).toBe('reward_lp');
  });

  test('validator reward function → "reward_validator"', () => {
    expect(detectTxType({ function: 'validatorreward' })).toBe('reward_validator');
  });

  test('lp reward function → "reward_lp"', () => {
    expect(detectTxType({ function: 'lpreward' })).toBe('reward_lp');
  });

  test('blockreward function (validator address) → "reward_validator"', () => {
    const validatorAddr = '0x1111111111111111111111111111111111111111';
    const validatorSet = new Set([validatorAddr]);
    expect(detectTxType({ function: 'blockreward', to: validatorAddr }, validatorSet))
      .toBe('reward_validator');
  });

  test('blockreward function (non-validator) → "reward"', () => {
    expect(detectTxType({ function: 'blockreward', to: '0xregular' }))
      .toBe('reward');
  });

  // ── Contract types ────────────────────────────────────────────────────────
  test('contract_create type → "contract_create"', () => {
    expect(detectTxType({ type: 'contract_create' })).toBe('contract_create');
  });

  test('deploycontract function → "contract_create"', () => {
    expect(detectTxType({ function: 'deploycontract' })).toBe('contract_create');
  });

  test('contract_address field present → "contract_create"', () => {
    expect(detectTxType({ contract_address: '0xnewcontract' })).toBe('contract_create');
  });

  test('is_contract_creation flag → "contract_create"', () => {
    expect(detectTxType({ is_contract_creation: true })).toBe('contract_create');
  });

  test('contract call with transfer function → "token_transfer"', () => {
    expect(detectTxType({ is_contract: true, function: 'transfer' })).toBe('token_transfer');
  });

  test('contract call with arbitrary function → "contract_call"', () => {
    expect(detectTxType({ is_contract: true, function: 'swap' })).toBe('contract_call');
  });

  test('contract call with addLiquidity function → "contract_call"', () => {
    expect(detectTxType({ is_contract: true, function: 'addliquidity' })).toBe('contract_call');
  });

  test('contract call with approve function → "contract_call"', () => {
    expect(detectTxType({ is_contract: true, function: 'approve' })).toBe('contract_call');
  });

  // ── Edge cases ────────────────────────────────────────────────────────────
  test('empty tx object → "transfer"', () => {
    expect(detectTxType({})).toBe('transfer');
  });

  test('tx with only value → "transfer"', () => {
    expect(detectTxType({ value: '1000' })).toBe('transfer');
  });

  test('case insensitive function matching', () => {
    expect(detectTxType({ function: 'ValidatorReward' })).toBe('reward_validator');
  });
});

// ─────────────────────────────────────────────────────────────────────────────
// getTxTypeLabel
// ─────────────────────────────────────────────────────────────────────────────

describe('getTxTypeLabel', () => {
  test('"transfer" → "Transfer"', () => {
    expect(getTxTypeLabel('transfer')).toBe('Transfer');
  });

  test('"reward_validator" → "Validator Reward"', () => {
    expect(getTxTypeLabel('reward_validator')).toBe('Validator Reward');
  });

  test('"reward_lp" → "Liquidity Provider Reward"', () => {
    expect(getTxTypeLabel('reward_lp')).toBe('Liquidity Provider Reward');
  });

  test('"reward" → "Reward"', () => {
    expect(getTxTypeLabel('reward')).toBe('Reward');
  });

  test('"contract_create" → "Contract Creation"', () => {
    expect(getTxTypeLabel('contract_create')).toBe('Contract Creation');
  });

  test('"contract_call" → "Contract Call"', () => {
    expect(getTxTypeLabel('contract_call')).toBe('Contract Call');
  });

  test('"token_transfer" → "Token Transfer"', () => {
    expect(getTxTypeLabel('token_transfer')).toBe('Token Transfer');
  });

  test('unknown type → "Transfer" (default)', () => {
    expect(getTxTypeLabel('unknown_type')).toBe('Transfer');
  });

  test('empty string → "Transfer" (default)', () => {
    expect(getTxTypeLabel('')).toBe('Transfer');
  });

  test('undefined → "Transfer" (default)', () => {
    expect(getTxTypeLabel(undefined)).toBe('Transfer');
  });

  test('uppercase input is handled case-insensitively', () => {
    expect(getTxTypeLabel('TRANSFER')).toBe('Transfer');
  });
});
