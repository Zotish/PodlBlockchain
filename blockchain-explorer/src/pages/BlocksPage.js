import React, { useState, useEffect } from 'react';
import BlockList from '../components/BlockList';
import { fetchJSON, mergeArrayResults } from '../utils/api';

const BlocksPage = () => {
  const [blocks, setBlocks] = useState([]);
  const [loading, setLoading] = useState(true);
  const [page, setPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);

  useEffect(() => {
    const fetchBlocks = async () => {
      try {

        const data = await fetchJSON('/fetch_last_n_block');
        const merged = mergeArrayResults(data, 'block_number');
        merged.sort((a, b) => (b.block_number ?? 0) - (a.block_number ?? 0));
        setBlocks(merged.slice(0, 10));
        setTotalPages(1);
      } catch (err) {
        console.error('Error fetching blocks:', err);
      } finally {
        setLoading(false);
      }
    };

    fetchBlocks();
    const id = setInterval(fetchBlocks, 1000);
    return () => clearInterval(id);
  }, [page]);

  if (loading) return <div className="loading">Loading blocks...</div>;

  return (
    <div className="blocks-page">
      <h2>Blocks</h2>
      
      <div className="blocks-controls">
        <div className="pagination">
          <button 
            onClick={() => setPage(p => Math.max(1, p - 1))} 
            disabled={page === 1}
          >
            Previous
          </button>
          <span>Page {page} of {totalPages}</span>
          <button 
            onClick={() => setPage(p => Math.min(totalPages, p + 1))} 
            disabled={page === totalPages}
          >
            Next
          </button>
        </div>
      </div>

      <BlockList blocks={blocks} />
    </div>
  );
};

export default BlocksPage;



// src/pages/BlockPage.js
// import React, { useState, useEffect } from "react";
// import { useParams, Link } from "react-router-dom";
// import TransactionList from "../components/TransactionList";

// const NODE = "http://127.0.0.1:5000";

// const BlockPage = () => {
//   const { id } = useParams();
//   const [block, setBlock] = useState(null);
//   const [loading, setLoading] = useState(true);
//   const [error, setError] = useState("");

//   const fetchBlock = async () => {
//     try {
//       setError("");
//       setLoading(true);

//       const res = await fetch(`${NODE}/block/${id}`);
//       if (!res.ok) {
//         if (res.status === 404) {
//           throw new Error("Block not found");
//         }
//         throw new Error(`HTTP ${res.status}`);
//       }

//       const data = await res.json();
//       setBlock(data);
//     } catch (err) {
//       console.error("Error fetching block:", err);
//       setError(err.message || String(err));
//       setBlock(null);
//     } finally {
//       setLoading(false);
//     }
//   };

//   useEffect(() => {
//     fetchBlock();
//     // If you want live updates, you can poll here:
//     // const idInt = setInterval(fetchBlock, 5000);
//     // return () => clearInterval(idInt);
//   }, [id]);

//   if (loading) {
//     return <div className="loading">Loading block…</div>;
//   }

//   if (error || !block) {
//     return (
//       <div className="block-page">
//         <h2>Block Details</h2>
//         <p className="error">Error: {error || "Block not found"}</p>
//         <Link to="/blocks" style={{ color: "#2563eb" }}>
//           ← Back to Blocks
//         </Link>
//       </div>
//     );
//   }

//   const blockNumber = block.block_number;
//   const currentHash = block.current_hash;
//   const previousHash = block.previous_hash;
//   const timestamp = block.timestamp
//     ? new Date(block.timestamp * 1000).toLocaleString()
//     : "—";
//   const gasUsed = block.gas_used ?? 0;
//   const gasLimit = block.gas_limit ?? 0;
//   const baseFee = block.base_fee ?? 0;
//   const txs = Array.isArray(block.transactions) ? block.transactions : [];

//   return (
//     <div className="block-page">
//       <h2>Block #{blockNumber}</h2>

//       <div className="block-details">
//         <div className="detail-row">
//           <span className="detail-label">Block Number:</span>
//           <span className="detail-value">{blockNumber}</span>
//         </div>
//         <div className="detail-row">
//           <span className="detail-label">Hash:</span>
//           <span className="detail-value" style={{ wordBreak: "break-all" }}>
//             {currentHash}
//           </span>
//         </div>
//         <div className="detail-row">
//           <span className="detail-label">Previous Hash:</span>
//           <span className="detail-value" style={{ wordBreak: "break-all" }}>
//             {previousHash}
//           </span>
//         </div>
//         <div className="detail-row">
//           <span className="detail-label">Timestamp:</span>
//           <span className="detail-value">{timestamp}</span>
//         </div>
//         <div className="detail-row">
//           <span className="detail-label">Gas Used:</span>
//           <span className="detail-value">
//             {gasUsed} / {gasLimit}
//           </span>
//         </div>
//         <div className="detail-row">
//           <span className="detail-label">Base Fee:</span>
//           <span className="detail-value">{baseFee} (per gas unit)</span>
//         </div>
//       </div>

//       <h3>Transactions ({txs.length})</h3>
//       {txs.length > 0 ? (
//         <TransactionList transactions={txs} />
//       ) : (
//         <p>No transactions in this block</p>
//       )}
//     </div>
//   );
// };

// export default BlockPage;
