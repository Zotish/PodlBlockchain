import React, { useEffect, useState } from "react";
import { formatLQD } from "../utils/lqdUnits";
import { fetchJSON, mergeArrayResults } from "../utils/api";

export default function LiquidityPage() {
  const [providers, setProviders] = useState([]);

  useEffect(() => {
    const fetchProviders = () => {
      fetchJSON("/liquidity/all")
        .then((d) =>
          setProviders(
            mergeArrayResults(d, "address").map((lp) => ({
              address: lp.address ?? lp.Address ?? "",
              stake_amount: lp.stake_amount ?? lp.StakeAmount ?? 0,
              liquidity_power: lp.liquidity_power ?? lp.LiquidityPower ?? 0,
              total_rewards: lp.total_rewards ?? lp.TotalRewards ?? 0,
              pending_rewards: lp.pending_rewards ?? lp.PendingRewards ?? 0,
            }))
          )
        )
        .catch(() => setProviders([]));
    };
    fetchProviders();
    const id = setInterval(fetchProviders, 1000);
    return () => clearInterval(id);
  }, []);

  return (
    <div className="validators-page">
      <h2>Liquidity Providers</h2>
      <table>
        <thead>
          <tr>
            <th>Address</th>
            <th>Stake</th>
            <th>Liquidity Power</th>
            <th>Total Rewards</th>
            <th>Pending Rewards</th>
          </tr>
        </thead>
        <tbody>
          {providers.map((lp) => (
            <tr key={lp.address}>
              <td>{lp.address}</td>
              <td>{formatLQD(lp.stake_amount)}</td>
              <td>{formatLQD(lp.liquidity_power)}</td>
              <td>{formatLQD(lp.total_rewards)}</td>
              <td>{formatLQD(lp.pending_rewards)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
