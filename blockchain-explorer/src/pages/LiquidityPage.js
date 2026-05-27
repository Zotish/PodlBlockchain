/* global BigInt */
import React, { useCallback, useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { formatLQD } from "../utils/lqdUnits";
import { buildSignedClaimPayload, connectExtensionWallet, shortWalletAddress } from "../utils/claimWallet";
import { fetchJSON, firstNodeResult, mergeArrayResults } from "../utils/api";

const SECONDS_PER_YEAR = 365 * 24 * 60 * 60;
const TARGET_BLOCK_SECONDS = 2;

const toBig = (value) => {
  try {
    if (value === undefined || value === null || value === "") return 0n;
    return BigInt(String(value));
  } catch {
    return 0n;
  }
};

const shortAddress = (value = "") => {
  const text = String(value || "");
  if (text.length <= 18) return text || "-";
  return `${text.slice(0, 10)}...${text.slice(-8)}`;
};

const normalizeProvider = (lp = {}) => ({
  address: lp.address ?? lp.Address ?? "",
  stakeAmount: lp.stake_amount ?? lp.StakeAmount ?? 0,
  liquidityPower: lp.liquidity_power ?? lp.LiquidityPower ?? 0,
  totalRewards: lp.total_rewards ?? lp.TotalRewards ?? 0,
  pendingRewards: lp.pending_rewards ?? lp.PendingRewards ?? 0,
  lockTime: lp.lock_time ?? lp.LockTime ?? 0,
  lockDays: lp.lock_days ?? lp.LockDays ?? 0,
  isUnstaking: Boolean(lp.is_unstaking ?? lp.IsUnstaking),
});

const sumMap = (map = {}) =>
  Object.values(map || {}).reduce((acc, value) => acc + toBig(value), 0n);

export default function LiquidityPage() {
  const [providers, setProviders] = useState([]);
  const [latestRewards, setLatestRewards] = useState(null);
  const [rewardHistory, setRewardHistory] = useState([]);
  const [claimAddress, setClaimAddress] = useState("");
  const [connectedWallet, setConnectedWallet] = useState("");
  const [claimMessage, setClaimMessage] = useState("");
  const [claiming, setClaiming] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const fetchProviders = useCallback(async () => {
    try {
      setError("");
      const [providerResult, latestResult, historyResult] = await Promise.allSettled([
        fetchJSON("/liquidity/all", { cacheTtlMs: 4000, timeoutMs: 10000 }),
        fetchJSON("/rewards/latest", { cacheTtlMs: 2000, timeoutMs: 10000 }),
        fetchJSON("/rewards/recent", { cacheTtlMs: 5000, timeoutMs: 10000 }),
      ]);

      if (providerResult.status === "fulfilled") {
        const list = Array.isArray(providerResult.value)
          ? providerResult.value
          : mergeArrayResults(providerResult.value, "address");
        setProviders(list.map(normalizeProvider));
      } else {
        setProviders([]);
        setError(providerResult.reason?.message || "Failed to load liquidity providers");
      }

      if (latestResult.status === "fulfilled") {
        setLatestRewards(firstNodeResult(latestResult.value) || latestResult.value);
      }

      if (historyResult.status === "fulfilled") {
        const payload = firstNodeResult(historyResult.value) || historyResult.value;
        setRewardHistory(Array.isArray(payload) ? payload : []);
      }
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchProviders();
    const id = setInterval(fetchProviders, 10000);
    return () => clearInterval(id);
  }, [fetchProviders]);

  const analytics = useMemo(() => {
    const totalStake = providers.reduce((acc, lp) => acc + toBig(lp.stakeAmount), 0n);
    const totalPower = providers.reduce((acc, lp) => acc + BigInt(Math.round(Number(lp.liquidityPower || 0) * 100)), 0n);
    const totalRewards = providers.reduce((acc, lp) => acc + toBig(lp.totalRewards), 0n);
    const pendingRewards = providers.reduce((acc, lp) => acc + toBig(lp.pendingRewards), 0n);
    const latestLPReward = sumMap(latestRewards?.liquidity_rewards);
    const rewardBasis = latestLPReward > 0n ? latestLPReward : pendingRewards;
    const annualBlocks = Math.floor(SECONDS_PER_YEAR / TARGET_BLOCK_SECONDS);
    const apr =
      totalStake > 0n && rewardBasis > 0n
        ? Number((rewardBasis * BigInt(annualBlocks) * 10000n) / totalStake) / 100
        : 0;
    const apy = apr > 0 ? (Math.pow(1 + apr / 100 / 365, 365) - 1) * 100 : 0;
    return { totalStake, totalPower, totalRewards, pendingRewards, latestLPReward, apr, apy };
  }, [providers, latestRewards]);

  const connectWallet = async () => {
    try {
      const account = await connectExtensionWallet();
      setConnectedWallet(account);
      if (!claimAddress.trim()) setClaimAddress(account);
      setClaimMessage(`Connected ${shortWalletAddress(account)}. Claim will require a wallet signature.`);
    } catch (err) {
      setClaimMessage(err.message || "Wallet connection failed.");
    }
  };

  const claimRewards = async (address) => {
    const target = String(address || claimAddress || connectedWallet).trim();
    if (!target) {
      setClaimMessage("Enter or select a liquidity provider address first.");
      return;
    }
    setClaiming(true);
    setClaimAddress(target);
    setClaimMessage("Waiting for wallet signature...");
    try {
      const payload = await buildSignedClaimPayload(target);
      setConnectedWallet(payload.wallet_address);
      setClaimAddress(payload.address);
      setClaimMessage("Signature accepted. Submitting signed claim request...");
      const result = await fetchJSON("/liquidity/claim", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
        timeoutMs: 10000,
      });
      setClaimMessage(`Claim submitted: ${JSON.stringify(result)}`);
    } catch (err) {
      const message = err.message || "";
      setClaimMessage(message.includes("Request failed")
        ? "Signed claim proof was created, but manual claim endpoint is not active on this node. Pending LP rewards remain visible and auto-sync during reward settlement/unstake flows."
        : message);
    } finally {
      setClaiming(false);
      fetchProviders();
    }
  };

  if (loading) return <div className="loading">Loading liquidity dashboard...</div>;

  return (
    <div className="liquidity-page">
      <div className="tracker-header">
        <p className="section-title">Liquidity Provider Dashboard</p>
        <h2 className="page-title">
          Liquidity Providers
          <span>{providers.length.toLocaleString()} total</span>
        </h2>
        <p>
          Track user liquidity, LP reward accrual, estimated APR/APY, and reward
          claim/sync status in one production dashboard.
        </p>
      </div>

      {error && <div className="error">{error}</div>}

      <div className="reward-metric-grid">
        <div className="reward-metric-card">
          <span>Total Staked Liquidity</span>
          <strong>{formatLQD(analytics.totalStake)} LQD</strong>
          <small>locked by active providers</small>
        </div>
        <div className="reward-metric-card">
          <span>Total LP Rewards</span>
          <strong>{formatLQD(analytics.totalRewards)} LQD</strong>
          <small>lifetime reward accounting</small>
        </div>
        <div className="reward-metric-card">
          <span>Pending LP Rewards</span>
          <strong>{formatLQD(analytics.pendingRewards)} LQD</strong>
          <small>claim/sync queue</small>
        </div>
        <div className="reward-metric-card">
          <span>Estimated APR / APY</span>
          <strong>{analytics.apr.toFixed(2)}% / {analytics.apy.toFixed(2)}%</strong>
          <small>based on current reward pace</small>
        </div>
      </div>

      <div className="reward-layout">
        <section className="tracker-table-wrap reward-panel">
          <div className="reward-panel-head">
            <div>
              <h3>LP Reward Timeline</h3>
              <p>Recent reward snapshots for reward transparency.</p>
            </div>
            <Link to="/rewards" className="btn-secondary">Open reward center</Link>
          </div>
          <table className="tracker-table">
            <thead>
              <tr>
                <th>Block</th>
                <th>Base Fee</th>
                <th>Gas Used</th>
                <th>Reward Distribution</th>
              </tr>
            </thead>
            <tbody>
              {rewardHistory.length === 0 ? (
                <tr><td colSpan={4} className="tracker-empty">No reward timeline found yet</td></tr>
              ) : rewardHistory.slice().reverse().map((item, index) => (
                <tr key={`${item.block_number}-${index}`}>
                  <td><Link to={`/blocks/${item.block_number}`}>#{item.block_number}</Link></td>
                  <td>{item.base_fee ?? "-"}</td>
                  <td>{item.gas_used ?? "-"}</td>
                  <td>
                    <div className="reward-chip-list">
                      {Object.entries(item.dist || {}).slice(0, 4).map(([addr, reward]) => (
                        <span key={addr}>{shortAddress(addr)}: {formatLQD(reward)} LQD</span>
                      ))}
                      {Object.keys(item.dist || {}).length === 0 && <span>No distribution data</span>}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </section>

        <aside className="reward-claim-card">
          <h3>LP Claim / Sync</h3>
          <p>
            Connect the LP wallet first. The connected wallet must match the LP
            address and sign a claim proof before submission.
          </p>
          <button className="btn-secondary" type="button" onClick={connectWallet}>
            {connectedWallet ? `Connected ${shortWalletAddress(connectedWallet)}` : "Connect Extension Wallet"}
          </button>
          <input
            type="text"
            value={claimAddress}
            onChange={(event) => setClaimAddress(event.target.value)}
            placeholder="0x... connected provider address"
          />
          <button className="btn-primary" disabled={claiming} onClick={() => claimRewards()}>
            {claiming ? "Syncing..." : "Claim / Sync"}
          </button>
          {claimMessage && <div className="reward-claim-status">{claimMessage}</div>}
        </aside>
      </div>

      <div className="tracker-table-wrap">
        <table className="tracker-table">
          <thead>
            <tr>
              <th>Provider</th>
              <th>Stake</th>
              <th>Liquidity Power</th>
              <th>APR Estimate</th>
              <th>APY Estimate</th>
              <th>Total Rewards</th>
              <th>Pending Rewards</th>
              <th>Status</th>
              <th>Claim</th>
            </tr>
          </thead>
          <tbody>
            {providers.length === 0 ? (
              <tr><td colSpan={9} className="tracker-empty">No liquidity providers found yet</td></tr>
            ) : providers.map((lp) => {
              const stake = toBig(lp.stakeAmount);
              const share = analytics.totalStake > 0n && stake > 0n
                ? Number((stake * 10000n) / analytics.totalStake) / 100
                : 0;
              return (
                <tr key={lp.address}>
                  <td><Link to={`/address/${lp.address}`}>{shortAddress(lp.address)}</Link></td>
                  <td>{formatLQD(lp.stakeAmount)} LQD</td>
                  <td>{Number(lp.liquidityPower || 0).toFixed(2)}</td>
                  <td>{analytics.apr.toFixed(2)}%</td>
                  <td>{analytics.apy.toFixed(2)}%</td>
                  <td>{formatLQD(lp.totalRewards)} LQD</td>
                  <td>{formatLQD(lp.pendingRewards)} LQD</td>
                  <td>
                    <span className={`badge ${lp.isUnstaking ? "badge-yellow" : "badge-green"}`}>
                      {lp.isUnstaking ? "Unstaking" : `Active - ${share.toFixed(2)}% share`}
                    </span>
                  </td>
                  <td>
                    <button className="btn-secondary compact" onClick={() => claimRewards(lp.address)} disabled={claiming}>
                      Claim
                    </button>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}
