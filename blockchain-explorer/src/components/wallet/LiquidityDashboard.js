
import React, { useEffect, useState } from "react";
import './Wallet.css'
import { formatLQD, parseLQD } from "./lqdUnits";
import { fetchJSON, firstNodeResult } from "../../utils/api";

const API = "http://127.0.0.1:9000";

export default function LiquidityDashboard({ address }) {
  const [lp, setLP] = useState(null);
  const [amount, setAmount] = useState("");
  const [lockDays, setLockDays] = useState(30);
  const [loading, setLoading] = useState(false);
  const [msg, setMsg] = useState("");

  const fetchInfo = async () => {
    try {
      const data = await fetchJSON(`/liquidity/info?address=${address}`);
      const result = firstNodeResult(data);
      if (!result || result.exists === false) {
        setLP(null);
      } else {
        setLP(result);
      }
    } catch (e) {
      console.error("liquidity/info error", e);
    }
  };

  useEffect(() => {
    if (!address) return;
    fetchInfo();
  }, [address]);

  const provideLiquidity = async () => {

    const rawValueStr = parseLQD(amount); // base units as string
    console.log("amount=", amount, "rawValueStr=", rawValueStr);
    setLoading(true);
    setMsg("");
    const body = {
      address,
      amount: rawValueStr,
      lock_days: Number(lockDays),
    };

    try {
      const res = await fetch(`${API}/liquidity/provide`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });

      const text = await res.text();
      setMsg(text);
    } catch (e) {
      setMsg("Error: " + e.message);
    } finally {
      setLoading(false);
      fetchInfo();
    }
  };

  const unstake = async () => {
    setLoading(true);
    setMsg("");
    try {
      const res = await fetch(`${API}/liquidity/unstake`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ address }),
      });
      const text = await res.text();
      setMsg(text);
    } catch (e) {
      setMsg("Error: " + e.message);
    } finally {
      setLoading(false);
      fetchInfo();
    }
  };

  // ---- Derived UI values (no structure change, just cosmetics) ----
  const hasLP = !!lp;
  const isUnstaking = lp?.is_unstaking;
  const totalStake = lp?.stake_amount || 0;
  const unstakeAmount = lp?.unstake_amount || 0;
  const released = lp?.released_so_far || 0;
  const pendingRewards = lp?.pending_rewards || 0;
  const totalRewards = lp?.total_rewards || 0;

  const unstakePercent =
    isUnstaking && unstakeAmount > 0
      ? Math.min(100, Math.round((released / unstakeAmount) * 100))
      : 0;

  const readableLockTime =
    lp?.lock_time ? new Date(lp.lock_time * 1000).toLocaleString() : "—";

  return (
    <div className="liquidity-dashboard">
      <div className="liquidity-header">
        <div>
          <h3>Liquidity Provider</h3>
          <p className="liquidity-subtitle">
            Auto-claiming <strong>rewards</strong> &amp;{" "}
            <strong>unstaked balance</strong> directly to your wallet.
          </p>
        </div>
        <div className="liquidity-address-chip">
          <span>Address:</span>
          <code>{address}</code>
        </div>
      </div>

      {hasLP ? (
        <>
          <div className="lp-card">
            <div className="lp-row">
              <div className="lp-stat-block">
                <h4>Current Stake</h4>
                <div className="lp-value">
                  {formatLQD(totalStake)} <span className="lp-unit">LQD</span>
                </div>
                <div className="lp-label">Actively locked for liquidity</div>
              </div>

              <div className="lp-stat-block">
                <h4>Liquidity Power</h4>
                <div className="lp-value">
                  {formatLQD(lp.liquidity_power)}
                </div>
                <div className="lp-label">Influences validator selection</div>
              </div>

              <div className="lp-stat-block">
                <h4>Rewards</h4>
                <div className="lp-reward-values">
                  <span>
                    Total: <strong>{formatLQD(totalRewards)}</strong> LQD
                  </span>
                  <span>
                    Pending: <strong>{formatLQD(pendingRewards)}</strong> LQD
                  </span>
                </div>
                <div className="lp-label">
                  Auto-distributed into your wallet by the chain.
                </div>
              </div>
            </div>

            <div className="lp-row lp-row-secondary">
              <div className="lp-meta">
                <span className="lp-meta-label">Lock until</span>
                <span className="lp-meta-value">{readableLockTime}</span>
              </div>
              <div className="lp-meta">
                <span className="lp-meta-label">Unstaking mode</span>
                <span className={`lp-badge ${isUnstaking ? "lp-badge-on" : "lp-badge-off"}`}>
                  {isUnstaking ? "ACTIVE (1% / day unlock)" : "INACTIVE"}
                </span>
              </div>
            </div>

            {isUnstaking && (
              <div className="lp-unstake">
                <div className="lp-unstake-header">
                  <h4>Unstaking Progress</h4>
                  <span className="lp-unstake-summary">
                    {formatLQD(released)} / {formatLQD(unstakeAmount)} LQD unlocked
                  </span>
                </div>

                <div className="lp-progress-bar">
                  <div
                    className="lp-progress-fill"
                    style={{ width: `${unstakePercent}%` }}
                  />
                </div>

                <div className="lp-progress-meta">
                  <span>{unstakePercent}% released</span>
                  <span>
                    Remaining: {unstakeAmount > released ? unstakeAmount - released : 0} LQD
                  </span>
                </div>

                <p className="lp-unstake-note">
                  Unlocked balance is automatically credited to your wallet
                  every day, no manual claim needed.
                </p>
              </div>
            )}

            {!isUnstaking && (
              <div className="lp-actions">
                <button
                  className="btn-secondary"
                  onClick={unstake}
                  disabled={loading}
                >
                  {loading ? "Processing..." : "Start Unstake"}
                </button>
                <span className="lp-hint">
                  Once you start unstaking, your liquidity stops earning power
                  and unlocks gradually at <strong>~1% per day</strong>.
                </span>
              </div>
            )}
          </div>
        </>
      ) : (
        <div className="lp-card">
          <h4>Provide Liquidity</h4>
          <p className="lp-intro">
            Stake your LQD to become a liquidity provider. You&apos;ll earn
            rewards and influence validator selection.
          </p>

          <div className="lp-form">
            <div className="lp-form-group">
              <label>Amount (LQD)</label>
              <input
                type="number"
                placeholder="Amount LQD"
                value={amount}
                onChange={(e) => setAmount(e.target.value)}
                min="0"
              />
            </div>

            <div className="lp-form-group">
              <label>Lock Period (days)</label>
              <input
                type="number"
                placeholder="Lock Days"
                value={lockDays}
                onChange={(e) => setLockDays(e.target.value)}
                min="0"
              />
            </div>

            <button
              className="btn-primary"
              onClick={provideLiquidity}
              disabled={loading || !amount}
            >
              {loading ? "Staking..." : "Stake Liquidity"}
            </button>
          </div>

          <p className="lp-hint">
            After lock time ends, you can start unstaking and your stake will be
            auto-released back to your wallet over time.
          </p>
        </div>
      )}

      {msg && (
        <div className="lp-message">
          <pre>{msg}</pre>
        </div>
      )}
    </div>
  );
}
