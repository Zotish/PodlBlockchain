



import { formatLQD } from "../utils/lqdUnits";

import React, { useMemo } from "react";
import { useNavigate } from "react-router-dom";
import { detectTxType, TxTypeBadge } from "../utils/txType";

const TransactionList = ({ transactions = [], validators = [] }) => {
  const navigate = useNavigate();

  // Build validator set for reward detection
  const validatorSet = useMemo(
    () =>
      new Set(
        (validators || []).map(
          (v) => (v.Address || v.address || "").toLowerCase()
        )
      ),
    [validators]
  );

  const timeAgo = (unix) => {
    if (!unix) return "N/A";
    const now = Math.floor(Date.now() / 1000);
    let d = now - unix;
    if (d < 0) d = 0;
    if (d < 60) return `${d}s ago`;
    const m = Math.floor(d / 60);
    if (m < 60) return `${m}m ago`;
    const h = Math.floor(m / 60);
    if (h < 24) return `${h}h ago`;
    const days = Math.floor(h / 24);
    return `${days}d ago`;
  };

  return (
    <div className="transaction-list">
      {transactions.length === 0 ? (
        <div className="no-transactions">No transactions found</div>
      ) : (
        transactions.map((tx, i) => {
          const hash = tx.txHash || tx.tx_hash || `tx_${i}`;
          const from = tx.from || tx.From || "";
          const to = tx.to || tx.To || "";
          const status = (tx.status || tx.Status || "").toLowerCase();
          const type = detectTxType(tx, validatorSet);

          const gas = tx.gas || tx.Gas || 0;
          const gasPrice = tx.gasPrice || tx.GasPrice || 0;
          const fee = gas * gasPrice;

          return (
            <div
              key={hash}
              className="transaction-item"
              onClick={() => navigate(`/tx/${hash}`)}
            >
              <div className="tx-header">
                <div className="tx-hash">{hash.slice(0, 12)}…</div>

                {/* NEW: Tx Type Badge */}
                <TxTypeBadge type={type} />

                <span className={`tx-status ${status}`}>
                  {status === "succsess"
                    ? "Success"
                    : status === "failed"
                    ? "Failed"
                    : "Pending"}
                </span>
              </div>

              <div className="tx-details">
                <div className="tx-addresses">
                  <strong>From:</strong> {from.slice(0, 18)}…  
                  &nbsp;→&nbsp;
                  <strong>To:</strong> {to.slice(0, 18)}…
                </div>

                <div>
                  <strong>Value:</strong> {formatLQD(tx.value || tx.Value || 0)}
                </div>

                <div>
                  <strong>Gas:</strong> {gas}
                </div>

                <div>
                  <strong>Fee:</strong> {formatLQD(fee)}
                </div>

                <div>
                  <strong>Time:</strong>{" "}
                  {timeAgo(tx.timestamp || tx.Timestamp)}
                </div>
              </div>
            </div>
          );
        })
      )}
    </div>
  );
};

export default TransactionList;



// import React, { useMemo } from "react";
// import { useNavigate } from "react-router-dom";
// import { detectTxType, TxTypeBadge } from "../utils/txType";

// const TransactionList = ({ transactions = [], validators = [] }) => {
//   const navigate = useNavigate();

//   const validatorSet = useMemo(
//     () =>
//       new Set(
//         (validators || []).map(
//           (v) => (v.Address || v.address || "").toLowerCase()
//         )
//       ),
//     [validators]
//   );

//   const timeAgo = (unix) => {
//     if (!unix) return "N/A";
//     const now = Math.floor(Date.now() / 1000);
//     let d = now - unix;
//     if (d < 0) d = 0;
//     if (d < 60) return `${d}s ago`;
//     const m = Math.floor(d / 60);
//     if (m < 60) return `${m}m ago`;
//     const h = Math.floor(m / 60);
//     if (h < 24) return `${h}h ago`;
//     return `${Math.floor(h / 24)}d ago`;
//   };

//   return (
//     <div className="transaction-list">
//       {transactions.length === 0 ? (
//         <div className="no-transactions">No transactions found</div>
//       ) : (
//         transactions.map((tx, i) => {
//           const hash =
//             tx.TxHash ||
//             tx.txHash ||
//             tx.tx_hash ||
//             tx.hash ||
//             `tx_${i}`;

//           const from = tx.from || tx.From || "";
//           const to = tx.to || tx.To || "";

//           // FIXED: Your backend uses "Succsess"
//           const status = (tx.status || tx.Status || "").toLowerCase().trim();

//           const type = detectTxType(tx, validatorSet);

//           const gas = tx.gas || tx.Gas || 0;
//           const gasPrice = tx.gasPrice || tx.GasPrice || 0;
//           const fee = gas * gasPrice;

//           // Reward breakdown
//           const rb = tx.reward_breakdown || tx.RewardBreakdown || null;

//           const totalLP = rb
//             ? Object.values(rb.liquidity_rewards || {}).reduce(
//                 (a, b) => a + b,
//                 0
//               )
//             : 0;

//           const participantReward = rb
//             ? rb.participant_rewards?.[hash] || 0
//             : 0;

//           const validatorReward = rb ? rb.validator_reward || 0 : 0;

//           const totalReward =
//             validatorReward + participantReward + totalLP;

//           return (
//             <div
//               key={hash}
//               className="transaction-item"
//               onClick={() => navigate(`/tx/${hash}`)}
//             >
//               {/* HEADER */}
//               <div className="tx-header">
//                 <div className="tx-hash">{hash.slice(0, 12)}…</div>

//                 <TxTypeBadge type={type} />

//                 <span className={`tx-status ${status}`}>
//                   {status === "succsess"
//                     ? "Success"
//                     : status === "failed"
//                     ? "Failed"
//                     : "Pending"}
//                 </span>
//               </div>

//               {/* DETAILS */}
//               <div className="tx-details">
//                 <div className="tx-addresses">
//                   <strong>From:</strong> {from.slice(0, 18)}… →{" "}
//                   <strong>To:</strong> {to.slice(0, 18)}…
//                 </div>

//                 <div>
//                   <strong>Value:</strong> {tx.value || tx.Value || 0} LQD
//                 </div>

//                 <div>
//                   <strong>Gas:</strong> {gas}
//                 </div>

//                 <div>
//                   <strong>Fee:</strong> {fee}
//                 </div>

//                 <div>
//                   <strong>Time:</strong> {timeAgo(tx.timestamp || tx.Timestamp)}
//                 </div>

//                 {/* Rewards */}
//                 <div className="tx-rewards">
//                   <strong>Rewards:</strong>
//                   {rb ? (
//                     <div>
//                       <div><strong>Total:</strong> {totalReward} LQD</div>
//                       <div><strong>Validator:</strong> {validatorReward} LQD</div>
//                       <div><strong>Participant:</strong> {participantReward} LQD</div>
//                       <div><strong>LP:</strong> {totalLP} LQD</div>
//                     </div>
//                   ) : (
//                     "-"
//                   )}
//                 </div>
//               </div>
//             </div>
//           );
//         })
//       )}
//     </div>
//   );
// };

// export default TransactionList;




// import React, { useMemo } from "react";
// import { useNavigate } from "react-router-dom";
// import { detectTxType, TxTypeBadge } from "../utils/txType";

// const TransactionList = ({ transactions = [], validators = [] }) => {
//   const navigate = useNavigate();

//   // Build validator set
//   const validatorSet = useMemo(
//     () =>
//       new Set(
//         (validators || []).map(
//           (v) => (v.Address || v.address || "").toLowerCase()
//         )
//       ),
//     [validators]
//   );

//   // Time helper
//   const timeAgo = (unix) => {
//     if (!unix) return "N/A";
//     const now = Math.floor(Date.now() / 1000);
//     let d = now - unix;
//     if (d < 0) d = 0;
//     if (d < 60) return `${d}s ago`;
//     const m = Math.floor(d / 60);
//     if (m < 60) return `${m}m ago`;
//     const h = Math.floor(m / 60);
//     if (h < 24) return `${h}h ago`;
//     return `${Math.floor(h / 24)}d ago`;
//   };

//   return (
//     <div className="transaction-list">
//       {transactions.length === 0 ? (
//         <div className="no-transactions">No transactions found</div>
//       ) : (
//         transactions.map((tx, index) => {
//           const hash =
//             tx.tx_hash ||
//             tx.TxHash ||
//             tx.hash ||
//             tx.txHash ||
//             `tx_${index}`;

//           const from = tx.from || tx.From || "";
//           const to = tx.to || tx.To || "";

//           // FIX: backend typo "Succsess"
//           const status = (tx.status || tx.Status || "").toLowerCase();

//           const type = detectTxType(tx, validatorSet);

//           const gas = tx.gas || tx.Gas || 0;
//           const gasPrice = tx.gas_price || tx.GasPrice || 0;
//           const fee = gas * gasPrice;

//           // Reward breakdown
//           const rb = tx.reward_breakdown || tx.RewardBreakdown || null;

//           const validatorReward = rb?.validator_reward || 0;
//           const participantReward =
//             rb?.participant_rewards?.[hash] || 0;
//           const totalLP = rb
//             ? Object.values(rb.liquidity_rewards || {}).reduce(
//                 (a, b) => a + b,
//                 0
//               )
//             : 0;

//           const totalReward =
//             validatorReward + participantReward + totalLP;

//           return (
//             <div
//               key={hash}
//               className="transaction-item"
//               onClick={() => navigate(`/tx/${hash}`)}
//             >
//               {/* ------------ HEADER ------------ */}
//               <div className="tx-header">
//                 <div className="tx-hash">{hash.slice(0, 14)}…</div>

//                 <TxTypeBadge type={type} />

//                 <span className={`tx-status ${status}`}>
//                   {status === "succsess"
//                     ? "Success"
//                     : status === "failed"
//                     ? "Failed"
//                     : "Pending"}
//                 </span>
//               </div>

//               {/* ------------ MAIN DETAILS ------------ */}
//               <div className="tx-details">
//                 <div className="tx-addresses">
//                   <strong>From:</strong> {from.slice(0, 18)}… →{" "}
//                   <strong>To:</strong> {to.slice(0, 18)}…
//                 </div>

//                 <div>
//                   <strong>Value:</strong> {tx.value || 0} LQD
//                 </div>

//                 <div>
//                   <strong>Gas:</strong> {gas}
//                 </div>

//                 <div>
//                   <strong>Fee:</strong> {fee}
//                 </div>

//                 <div>
//                   <strong>Time:</strong> {timeAgo(tx.timestamp)}
//                 </div>

//                 {/* ------------ REWARD SECTION ------------ */}
//                 {rb && (
//                   <div className="tx-rewards">
//                     <strong>Rewards:</strong>
//                     <div><strong>Total:</strong> {totalReward} LQD</div>
//                     <div><strong>Validator:</strong> {validatorReward} LQD</div>
//                     <div><strong>Participant:</strong> {participantReward} LQD</div>
//                     <div><strong>LP Providers:</strong> {totalLP} LQD</div>
//                   </div>
//                 )}
//               </div>
//             </div>
//           );
//         })
//       )}
//     </div>
//   );
// };

// export default TransactionList;
