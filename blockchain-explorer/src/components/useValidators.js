// src/hooks/useValidators.js
import { useEffect, useMemo, useRef, useState } from 'react';
import { mergeArrayResults } from '../utils/api';

function normalizeValidator(v) {
  // Accept both styles returned by your APIs
  const address = v.address ?? v.Address ?? '';
  const stake =
    v.stake ??
    v.lp_stake_amount ??
    v.LPStakeAmount ??
    0;
  const liquidity_power = v.liquidity_power ?? v.LiquidityPower ?? 0;
  const penalty_score = v.penalty_score ?? v.PenaltyScore ?? 0;
  const blocks_proposed = v.blocks_proposed ?? v.BlocksProposed ?? 0;
  const blocks_included = v.blocks_included ?? v.BlocksIncluded ?? 0;
  const last_active = v.last_active ?? v.LastActive ?? '';
  const lock_time = v.lock_time ?? v.LockTime ?? '';

  return {
    address,
    stake: Number(stake),
    liquidity_power: Number(liquidity_power),
    penalty_score: Number(penalty_score),
    blocks_proposed: Number(blocks_proposed),
    blocks_included: Number(blocks_included),
    last_active,
    lock_time,
  };
}

export default function useValidators(nodeUrls = ['http://127.0.0.1:9000'], { intervalMs = 3000 } = {}) {
  const [validators, setValidators] = useState([]);
  const [loading, setLoading] = useState(true);
  const [errors, setErrors] = useState([]);
  const timerRef = useRef(null);

  const fetchAll = async () => {
    setErrors([]);
    try {
      const results = await Promise.allSettled(
        nodeUrls.map(async (url) => {
          const res = await fetch(`${url}/validators`, { method: 'GET' });
          if (!res.ok) throw new Error(`${url} /validators -> ${res.status}`);
          return res.json();
        })
      );

      const merged = new Map(); // address -> normalized validator
      const errs = [];

      results.forEach((r, idx) => {
        if (r.status === 'fulfilled') {
          const arr = mergeArrayResults(r.value, 'address');
          arr.forEach((raw) => {
            const n = normalizeValidator(raw);
            if (!n.address) return;
            const prev = merged.get(n.address);
            if (!prev || n.stake > prev.stake || n.liquidity_power > prev.liquidity_power) {
              merged.set(n.address, n);
            }
          });
        } else {
          errs.push(`Node ${nodeUrls[idx]}: ${r.reason}`);
        }
      });

      setValidators(Array.from(merged.values()).sort((a, b) => b.liquidity_power - a.liquidity_power));
      setErrors(errs);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    setLoading(true);
    fetchAll();
    if (intervalMs > 0) {
      timerRef.current = setInterval(fetchAll, intervalMs);
      return () => clearInterval(timerRef.current);
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [JSON.stringify(nodeUrls), intervalMs]);

  const summary = useMemo(() => {
    const totalStake = validators.reduce((s, v) => s + (v.stake || 0), 0);
    const active = validators.length;
    return { totalStake, active };
  }, [validators]);

  return { validators, loading, errors, summary, refresh: fetchAll };
}
