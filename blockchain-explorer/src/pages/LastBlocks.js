
import React, { useState, useEffect } from "react";
import { Link } from "react-router-dom";
import { fetchJSON, mergeArrayResults } from "../utils/api";

const LastBlocks = () => {
  const [blocks, setBlocks] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const fetchLastBlocks = async () => {
    try {
      setError("");
      const data = await fetchJSON("/fetch_last_n_block");
      const arr = mergeArrayResults(data, "block_number");
      // Sort newest first by block_number
      arr.sort((a, b) => (b.block_number ?? 0) - (a.block_number ?? 0));

      setBlocks(arr);
    } catch (err) {
      console.error("Error fetching last blocks:", err);
      setError(err.message || String(err));
      setBlocks([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchLastBlocks();
    const id = setInterval(fetchLastBlocks, 1000);
    return () => clearInterval(id);
  }, []);

  if (loading) return <div className="loading">Loading last blocks...</div>;
  if (error)
    return <div className="error">Error loading last blocks: {error}</div>;
  if (!blocks.length)
    return <div className="no-blocks">No recent blocks found</div>;

  return (
    <div className="last-blocks">
      <h3>Latest Blocks</h3>
      <div className="table-wrapper">
        <table>
          <thead>
            <tr>
              <th>Block</th>
              <th>Hash</th>
              <th>Transactions</th>
              <th>Time</th>
            </tr>
          </thead>
          <tbody>
            {blocks.map((block) => (
              <tr key={block.block_number}>
                <td>
                  <Link to={`/blocks/${block.block_number}`}>
                    {block.block_number}
                  </Link>
                </td>
                <td>
                  <Link to={`/blocks/${block.block_number}`}>
                    {block.current_hash
                      ? block.current_hash.substring(0, 16) + "..."
                      : "—"}
                  </Link>
                </td>
                <td>{Array.isArray(block.transactions) ? block.transactions.length : 0}</td>
                <td>
                  {block.timestamp
                    ? new Date(block.timestamp * 1000).toLocaleString()
                    : "—"}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
};

export default LastBlocks;
