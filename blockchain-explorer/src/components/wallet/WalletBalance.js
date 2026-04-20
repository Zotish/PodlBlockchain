
import React, { useState, useEffect } from "react";
import { formatLQD } from "./lqdUnits";
import { API_BASE, apiUrl, fetchJSON, firstNodeResult, waitForTx } from "../../utils/api";

const NODE_URL = API_BASE;
const TOKENS_STORAGE_KEY = "liquidity_tokens_v1";

// Helper: build a per-address storage key
const tokensKeyForAddress = (address) =>
  `${TOKENS_STORAGE_KEY}_${(address || "").toLowerCase()}`;

// Helper component for one token card
const TokenCard = ({ token, address, privateKey, onRefresh, onRemove }) => {
  const [to, setTo] = useState("");
  const [amount, setAmount] = useState("");
  const [status, setStatus] = useState("");

  // Convert "1.23" -> raw integer string using decimals
  const toRawTokenAmount = (amountStr, decimals) => {
    if (!amountStr) return null;
    const trimmed = amountStr.trim();
    if (trimmed === "") return null;

    // Overflow guard: max 20 integer digits + decimals
    const [intPartRaw, fracPartRaw] = trimmed.split(".");
    if ((intPartRaw || "").replace(/^0+/, "").length > 20) return null;
    if ((fracPartRaw || "").length > 18) return null;

    const intPart = (intPartRaw || "0").replace(/^0+/, "") || "0";
    let fracPart = fracPartRaw || "";

    if (fracPart.length > decimals) {
      fracPart = fracPart.slice(0, decimals);
    } else {
      fracPart = fracPart.padEnd(decimals, "0");
    }

    const full = (intPart + fracPart).replace(/^0+/, "") || "0";
    if (!/^[0-9]+$/.test(full)) return null;
    // Final length guard: max 39 digits (safe for uint256)
    if (full.length > 39) return null;
    return full;
  };

  const handleSendToken = async () => {
    if (!token.contract) {
      setStatus("No token contract.");
      return;
    }
    if (!to.trim()) {
      setStatus("Enter recipient address.");
      return;
    }
    if (!amount.trim()) {
      setStatus("Enter amount.");
      return;
    }

    const rawAmount = toRawTokenAmount(amount, token.decimals);
    if (!rawAmount) {
      setStatus("Invalid amount.");
      return;
    }

    try {
      setStatus("Sending token...");
      const body = {
        address,
        contract_address: token.contract,
        function: "Transfer",
        args: [to.trim(), rawAmount],
        value: 0,
        private_key: privateKey || "",
      };

      const res = await fetch(apiUrl(NODE_URL, "/wallet/contract-template"), {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
      const text = await res.text();
      let data = null;
      try {
        data = JSON.parse(text);
      } catch {
        data = null;
      }
      if (!res.ok || (data && data.success === false)) {
        throw new Error(data?.error || data?.output || text || "Token transfer failed");
      }

      const hash = data?.tx_hash || data?.TxHash || data?.hash || "";
      if (hash) {
        await waitForTx(hash, 5000).catch(() => null);
      }

      setStatus(`Success: sent ${amount} ${token.symbol} to ${to.trim()}`);
      setAmount("");
      setTo("");
      try {
        window.dispatchEvent(new CustomEvent("lqd:wallet-updated", { detail: { address, token: token.contract } }));
      } catch {}

      await onRefresh(token.contract);
    } catch (err) {
      console.error("token transfer error", err);
      setStatus(`Error: ${err.message}`);
    }
  };

  return (
    <div className="balance-card" style={{ marginTop: 20 }}>
      <div
        style={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
        }}
      >
        <h3>{token.symbol} {token.type === "dapp" ? "Contract" : "Token"}</h3>
        <button
          className="btn-secondary"
          style={{ fontSize: 12 }}
          onClick={() => onRemove(token.contract)}
        >
          Remove
        </button>
      </div>

      <div className="balance-details" style={{ marginTop: 8 }}>
        <div className="balance-item">
          <span>Contract:</span>
          <span style={{ wordBreak: "break-all" }}>{token.contract}</span>
        </div>
        <div className="balance-item">
          <span>Name:</span>
          <span>{token.name}</span>
        </div>
        <div className="balance-item">
          <span>Symbol:</span>
          <span>{token.symbol}</span>
        </div>
        <div className="balance-item">
          <span>Decimals:</span>
          <span>{token.decimals}</span>
        </div>
        <div className="balance-item">
          <span>Type:</span>
          <span>{token.type || "token"}</span>
        </div>
      </div>

      <div className="balance-amount" style={{ fontSize: 28, marginTop: 12 }}>
        {token.type === "dapp" ? "N/A" : `${token.balanceFormatted} ${token.symbol}`}
      </div>

      <div className="balance-details">
        <div className="balance-item">
          <span>Raw:</span>
          <span>{token.balanceRaw}</span>
        </div>
      </div>

      <div className="balance-actions" style={{ marginTop: 10 }}>
        {token.type !== "dapp" && (
          <button
            className="btn-secondary"
            onClick={() => onRefresh(token.contract)}
          >
            Refresh {token.symbol}
          </button>
        )}
      </div>

      {token.type !== "dapp" && (
        <div style={{ marginTop: 20, textAlign: "left" }}>
          <h4>Send {token.symbol}</h4>
          <div className="form-group">
            <label>Recipient Address</label>
            <input
              type="text"
              value={to}
              onChange={(e) => setTo(e.target.value)}
              placeholder="0xRecipient..."
            />
          </div>
          <div className="form-group">
            <label>Amount ({token.symbol})</label>
            <input
              type="number"
              min="0"
              step="0.000000000000000001"
              value={amount}
              onChange={(e) => setAmount(e.target.value)}
              placeholder="e.g. 10.5"
            />
          </div>
          <button className="btn-primary" onClick={handleSendToken}>
            Send {token.symbol}
          </button>

          {status && <div style={{ marginTop: 8 }}>{status}</div>}
        </div>
      )}
    </div>
  );
};

const WalletBalance = ({ address, privateKey }) => {
  const [balance, setBalance] = useState("0");
  const [confirmedBalance, setConfirmedBalance] = useState("0");
  const [pendingChange, setPendingChange] = useState("0");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  // multiple tokens
  const [tokens, setTokens] = useState([]); // {contract, symbol, decimals, balanceRaw, balanceFormatted, type}
  const [tokenInputAddress, setTokenInputAddress] = useState("");
  const [tokenError, setTokenError] = useState("");
  const [tokenLoading, setTokenLoading] = useState(false);
  const [importMode, setImportMode] = useState("auto");
  const [autoImporting, setAutoImporting] = useState(false);
  const [autoStatus, setAutoStatus] = useState("");

  const fetchBalance = async () => {
    try {
      setError("");
      const data = await fetchJSON(`/balance?address=${address}`);
      const result = firstNodeResult(data) || {};
      setBalance(result.balance ?? "0");
      setConfirmedBalance(result.confirmed_balance ?? result.balance ?? "0");
      setPendingChange(result.pending_balance_change ?? "0");
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const formatTokenAmount = (raw, decimals) => {
    const rawStr = (raw || "0").toString().replace(/^0+/, "") || "0";
    const dec = Number.isFinite(decimals) ? decimals : 0;

    if (dec === 0) return rawStr;

    if (rawStr.length <= dec) {
      const padded = rawStr.padStart(dec + 1, "0");
      const intPart = padded.slice(0, padded.length - dec);
      const fracPart = padded.slice(padded.length - dec).replace(/0+$/, "");
      return fracPart ? `${intPart}.${fracPart}` : intPart;
    }

    const intPart = rawStr.slice(0, rawStr.length - dec);
    const fracPart = rawStr.slice(rawStr.length - dec).replace(/0+$/, "");
    return fracPart ? `${intPart}.${fracPart}` : intPart;
  };

  const callContract = async (contractAddr, fn, args = []) => {
    const body = {
      address: contractAddr,
      fn,
      args,
      value: 0,
      caller: address,
    };
    const res = await fetch(apiUrl(NODE_URL, "/contract/call"), {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    const text = await res.text();
    let data = null;
    try {
      data = JSON.parse(text);
    } catch {
      data = null;
    }
    if (!res.ok) {
      throw new Error(data?.error || text || `Failed ${fn}()`);
    }
    if (data && data.success === false) {
      throw new Error(data.error || data.output || `Failed ${fn}()`);
    }
    return data || {};
  };

  const fetchContractABI = async (contractAddr) => {
    const res = await fetch(apiUrl(NODE_URL, `/contract/getAbi?address=${contractAddr}`));
    const text = await res.text();
    let data = null;
    try {
      data = JSON.parse(text);
    } catch {
      data = null;
    }
    if (!res.ok) {
      throw new Error(data?.error || text || "Failed to load ABI");
    }
    return Array.isArray(data) ? data : data?.entries || [];
  };

  const isTokenABI = (abi) => {
    if (!Array.isArray(abi)) return false;
    const names = abi
      .map((fn) => (fn?.name || "").toLowerCase())
      .filter(Boolean);
    return (
      names.includes("symbol") &&
      names.includes("decimals") &&
      names.includes("balanceof") &&
      names.includes("transfer")
    );
  };

  const detectDappType = (abi) => {
    const names = Array.isArray(abi)
      ? abi.map((fn) => (fn?.name || "").toLowerCase()).filter(Boolean)
      : [];
    const has = (k) => names.includes(k);
    if (has("addliquidity") || has("removeliquidity") || has("swapatob") || has("swapbtoa")) {
      return { name: "DEX", symbol: "DEX", type: "dex" };
    }
    if (has("tokenuri") || has("ownerof") || has("safetransferfrom")) {
      return { name: "NFT", symbol: "NFT", type: "nft" };
    }
    if (has("borrow") || has("repay") || has("deposit") || has("withdraw") || has("interest")) {
      return { name: "Lending", symbol: "LEND", type: "lending" };
    }
    if (has("proposal") || has("vote") || has("delegate")) {
      return { name: "DAO", symbol: "DAO", type: "dao" };
    }
    return null;
  };

  const classifyDapp = (abi) => {
    return detectDappType(abi) || { name: "Dapp Contract", symbol: "DAPP", type: "dapp" };
  };

  const fetchTokenMetadata = async (contractAddr) => {
    const symbolRes = await callContract(contractAddr, "Symbol", []).catch(() => callContract(contractAddr, "symbol", []));
    const decimalsRes = await callContract(contractAddr, "Decimals", []).catch(() => callContract(contractAddr, "decimals", []));
    const nameRes = await callContract(contractAddr, "Name", []).catch(() => callContract(contractAddr, "name", []));

    const sym = symbolRes.output || "TOKEN";
    const decimalsStr =
      decimalsRes.output ||
      (decimalsRes.storage && decimalsRes.storage.decimals) ||
      "18";

    const dec = parseInt(decimalsStr, 10) || 0;

    return {
      symbol: sym,
      decimals: dec,
      name: nameRes.output || symbolRes.storage?.name || sym,
    };
  };

  const fetchTokenBalance = async (contractAddr, symbol, decimals) => {
    const data = await callContract(contractAddr, "BalanceOf", [address]).catch(() => callContract(contractAddr, "balanceOf", [address]));
    const raw = data.output || "0";
    const formatted = formatTokenAmount(raw, decimals);
    return { raw, formatted };
  };

  // Save tokens (without balances) per address
  const saveTokensForAddress = (addr, tokenList) => {
    const key = tokensKeyForAddress(addr);
    const minimal = tokenList.map((t) => ({
      contract: t.contract,
      name: t.name || t.symbol || "Token",
      symbol: t.symbol,
      decimals: t.decimals,
      type: t.type || "token",
    }));
    try {
      localStorage.setItem(key, JSON.stringify(minimal));
    } catch (e) {
      console.warn("Failed to save tokens to localStorage", e);
    }
  };

  const handleImportToken = async () => {
    const contractAddr = tokenInputAddress.trim();
    if (!contractAddr) {
      setTokenError("Please enter a token contract address");
      return;
    }

    // prevent duplicates
    if (
      tokens.some(
        (t) => t.contract.toLowerCase() === contractAddr.toLowerCase()
      )
    ) {
      setTokenError("Token already imported");
      return;
    }

    try {
      setTokenLoading(true);
      setTokenError("");

      // 1) read symbol & decimals
      let newToken = null;
      try {
        const abi = await fetchContractABI(contractAddr);
        const dapp = detectDappType(abi);
        if (dapp) {
          newToken = {
            contract: contractAddr,
            name: dapp.name,
            symbol: dapp.symbol,
            decimals: 0,
            balanceRaw: "0",
            balanceFormatted: "0",
            type: dapp.type,
          };
        } else if (isTokenABI(abi)) {
          const meta = await fetchTokenMetadata(contractAddr);
          const bal = await fetchTokenBalance(
            contractAddr,
            meta.symbol,
            meta.decimals
          );
          newToken = {
            contract: contractAddr,
            name: meta.name || meta.symbol || "Token",
            symbol: meta.symbol,
            decimals: meta.decimals,
            balanceRaw: bal.raw,
            balanceFormatted: bal.formatted,
            type: "lqd20",
          };
        } else {
          const d = classifyDapp(abi);
          newToken = {
            contract: contractAddr,
            name: d.name,
            symbol: d.symbol,
            decimals: 0,
            balanceRaw: "0",
            balanceFormatted: "0",
            type: d.type,
          };
        }
      } catch (err) {
        throw err;
      }

      setTokens((prev) => {
        const updated = [...prev, newToken];
        saveTokensForAddress(address, updated);
        return updated;
      });
    } catch (err) {
      console.error("import token error", err);
      setTokenError(err.message);
    } finally {
      setTokenLoading(false);
    }
  };

  const refreshSingleToken = async (contractAddr) => {
    try {
      const token = tokens.find(
        (t) => t.contract.toLowerCase() === contractAddr.toLowerCase()
      );
      if (!token) return;
      if (token.type === "dapp") return;

      const bal = await fetchTokenBalance(
        contractAddr,
        token.symbol,
        token.decimals
      );

      setTokens((prev) => {
        const updated = prev.map((t) =>
          t.contract.toLowerCase() === contractAddr.toLowerCase()
            ? {
                ...t,
                balanceRaw: bal.raw,
                balanceFormatted: bal.formatted,
              }
            : t
        );
        saveTokensForAddress(address, updated);
        return updated;
      });
    } catch (err) {
      console.error("refresh token error", err);
      setTokenError(err.message);
    }
  };

  const handleRemoveToken = (contractAddr) => {
    setTokens((prev) => {
      const updated = prev.filter(
        (t) => t.contract.toLowerCase() !== contractAddr.toLowerCase()
      );
      saveTokensForAddress(address, updated);
      return updated;
    });
  };

  const handleFaucet = async () => {
    try {
      setLoading(true);
      setError("");

      const response = await fetch(apiUrl(NODE_URL, "/faucet"), {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ address }),
      });

      const result = await response.json();

      if (!response.ok) {
        throw new Error(
          result.error || result.message || "Faucet request failed"
        );
      }

      await fetchBalance();

      alert(`Received ${formatLQD(result.credited || "0")} LQD from faucet!`);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  // On mount / when address changes: load saved tokens for this address and fetch their balances
  useEffect(() => {
    const key = tokensKeyForAddress(address);
    let stored = [];
    try {
      const raw = localStorage.getItem(key);
      if (raw) stored = JSON.parse(raw);
    } catch {
      stored = [];
    }

    if (!stored || stored.length === 0) {
      setTokens([]);
      return;
    }

    // stored: [{contract, symbol, decimals, type}]
    const initialTokens = stored.map((t) => ({
      contract: t.contract,
      name: t.name || t.symbol || "Token",
      symbol: t.symbol || "TOKEN",
      decimals: t.decimals ?? 18,
      balanceRaw: "0",
      balanceFormatted: "0",
      type: t.type || "token",
    }));
    setTokens(initialTokens);

    // Fetch balances for each saved token
    (async () => {
      for (const tok of initialTokens) {
        try {
          if (tok.type === "dapp") {
            continue;
          }
          const bal = await fetchTokenBalance(
            tok.contract,
            tok.symbol,
            tok.decimals
          );
          setTokens((prev) => {
            const updated = prev.map((p) =>
              p.contract.toLowerCase() === tok.contract.toLowerCase()
                ? {
                    ...p,
                    balanceRaw: bal.raw,
                    balanceFormatted: bal.formatted,
                  }
                : p
            );
            saveTokensForAddress(address, updated);
            return updated;
          });
        } catch (e) {
          console.error("error refreshing saved token", tok.contract, e);
        }
      }
    })();
  }, [address]);

  useEffect(() => {
    const refreshAll = () => {
      fetchBalance();
      tokens.forEach((tok) => {
        if (tok.type !== "dapp") {
          refreshSingleToken(tok.contract);
        }
      });
    };
    window.addEventListener("lqd:wallet-updated", refreshAll);
    return () => window.removeEventListener("lqd:wallet-updated", refreshAll);
  }, [tokens]);

  const autoImport = async () => {
    setAutoImporting(true);
    setAutoStatus("Scanning recent deployments...");
    setTokenError("");
    try {
      const listRes = await fetchJSON("/contract/list");
      const list = Array.isArray(listRes) ? listRes : listRes?.nodes || [];
      const contracts = list
        .map((c) => c?.result || c)
        .flat()
        .filter(Boolean)
        .filter(
          (c) =>
            c.owner &&
            c.owner.toLowerCase() === address.toLowerCase() &&
            c.address
        );

      if (contracts.length === 0) {
        setAutoStatus("No deployed contracts found for this wallet.");
        return;
      }

      let added = 0;
      for (const c of contracts) {
        const contractAddr = c.address;
        if (
          tokens.some(
            (t) => t.contract.toLowerCase() === contractAddr.toLowerCase()
          )
        ) {
          continue;
        }

        try {
          const abi = await fetchContractABI(contractAddr);
          const dapp = detectDappType(abi);
          if (dapp) {
            const dappToken = {
              contract: contractAddr,
              name: dapp.name,
              symbol: dapp.symbol,
              decimals: 0,
              balanceRaw: "0",
              balanceFormatted: "0",
              type: dapp.type,
            };
            added += 1;
            setTokens((prev) => {
              const updated = [...prev, dappToken];
              saveTokensForAddress(address, updated);
              return updated;
            });
          } else if (isTokenABI(abi)) {
            const meta = await fetchTokenMetadata(contractAddr);
            const bal = await fetchTokenBalance(
              contractAddr,
              meta.symbol,
              meta.decimals
            );
            const newToken = {
              contract: contractAddr,
              name: meta.name || meta.symbol || "Token",
              symbol: meta.symbol,
              decimals: meta.decimals,
              balanceRaw: bal.raw,
              balanceFormatted: bal.formatted,
              type: "lqd20",
            };
            added += 1;
            setTokens((prev) => {
              const updated = [...prev, newToken];
              saveTokensForAddress(address, updated);
              return updated;
            });
          } else {
            const d = classifyDapp(abi);
            const dappToken = {
              contract: contractAddr,
              name: d.name,
              symbol: d.symbol,
              decimals: 0,
              balanceRaw: "0",
              balanceFormatted: "0",
              type: d.type,
            };
            added += 1;
            setTokens((prev) => {
              const updated = [...prev, dappToken];
              saveTokensForAddress(address, updated);
              return updated;
            });
          }
        } catch {
          // ignore bad contract entry
        }
      }
      setAutoStatus(`Auto import complete. Added ${added} contract(s).`);
    } catch (err) {
      setTokenError(err.message);
    } finally {
      setAutoImporting(false);
    }
  };

  // Fetch LQD on mount/address change, then auto-refresh every 5 seconds
  useEffect(() => {
    fetchBalance();
    const interval = setInterval(fetchBalance, 5000);
    return () => clearInterval(interval);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [address]);

  if (loading && balance === "0")
    return <div className="loading">Loading balance...</div>;

  return (
    <div className="wallet-balance">
      {/* Native LQD balance */}
      <div className="balance-card">
        <h3>Your Balance</h3>
        <div className="balance-amount">{formatLQD(balance)} LQD</div>

        {pendingChange !== 0 && (
          <div className="pending-balance">
            <small>
              {pendingChange > 0 ? "+" : ""}
              {formatLQD(pendingChange)} LQD pending
            </small>
          </div>
        )}

        <div className="balance-details">
          <div className="balance-item">
            <span>Confirmed:</span>
            <span>{formatLQD(confirmedBalance)} LQD</span>
          </div>
          {pendingChange !== 0 && (
            <div className="balance-item">
              <span>Pending:</span>
              <span
                className={pendingChange > 0 ? "positive" : "negative"}
              >
                {pendingChange > 0 ? "+" : ""}
                {formatLQD(pendingChange)} LQD
              </span>
            </div>
          )}
        </div>

        <div className="balance-actions">
          <button
            className="btn-primary"
            onClick={handleFaucet}
            disabled={loading}
          >
            {loading ? "Processing..." : "Get Test Coins (Faucet)"}
          </button>
          <button className="btn-secondary" onClick={fetchBalance}>
            Refresh Balance
          </button>
        </div>
      </div>

      {/* Token import */}
      <div className="balance-card" style={{ marginTop: 20 }}>
        <h3>Import Token (LQD20-style)</h3>

        <div className="wallet-tabs" style={{ marginBottom: 12 }}>
          <button
            className={`tab ${importMode === "auto" ? "active" : ""}`}
            onClick={() => setImportMode("auto")}
          >
            Auto Import
          </button>
          <button
            className={`tab ${importMode === "manual" ? "active" : ""}`}
            onClick={() => setImportMode("manual")}
          >
            Manual Import
          </button>
        </div>

        {importMode === "auto" && (
          <div className="form-group">
            <button
              className="btn-secondary"
              onClick={autoImport}
              disabled={autoImporting}
            >
              {autoImporting ? "Scanning..." : "Auto Import My Deployed Tokens"}
            </button>
            {autoStatus && (
              <p style={{ marginTop: 8 }}>{autoStatus}</p>
            )}
          </div>
        )}

        {importMode === "manual" && (
        <div className="form-group">
          <label>Token Contract Address</label>
          <input
            type="text"
            value={tokenInputAddress}
            onChange={(e) => setTokenInputAddress(e.target.value)}
            placeholder="0x..."
          />
          <button
            className="btn-secondary"
            style={{ marginTop: 8 }}
            onClick={handleImportToken}
            disabled={tokenLoading || !tokenInputAddress.trim()}
          >
            {tokenLoading ? "Loading token..." : "Import Token"}
          </button>
        </div>
        )}

        {tokenError && (
          <div className="error-message" style={{ marginTop: 8 }}>
            {tokenError}
          </div>
        )}

        {tokens.length === 0 && (
          <p style={{ marginTop: 8 }}>
            No tokens imported yet. Paste a contract address above to add a
            token.
          </p>
        )}
      </div>

      {/* Tokens list */}
      {tokens.map((token) => (
        <TokenCard
          key={token.contract}
          token={token}
          address={address}
          privateKey={privateKey}
          onRefresh={refreshSingleToken}
          onRemove={handleRemoveToken}
        />
      ))}

      {error && <div className="error-message">{error}</div>}

      {/* Wallet info */}
      <div className="wallet-info">
        <h4>Wallet Information</h4>
        <div className="info-item">
          <strong>Address:</strong> {address}
        </div>
        <div className="info-item">
          <strong>Private Key:</strong>
          <span className="private-key-masked">
            ••••••••••••••••••••
            <button
              className="btn-copy-small"
              onClick={() => {
                const confirmReveal = window.confirm(
                  "Showing or copying your private key gives full control of your funds.\n\nDo you really want to copy it to clipboard?"
                );
                if (confirmReveal) {
                  navigator.clipboard.writeText(privateKey);
                }
              }}
            >
              Reveal & Copy
            </button>
          </span>
        </div>
        <div className="warning">
          ⚠️ Keep your private key secure and never share it with anyone.
          This key is only stored encrypted on this device and decrypted in memory when you unlock.
        </div>
      </div>
    </div>
  );
};

export default WalletBalance;
