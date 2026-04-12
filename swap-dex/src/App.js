/* global BigInt */
import React, { useCallback, useEffect, useRef, useState } from "react";
import "./styles.css";
import { DEX_ABI, DEX_CONTRACT_ADDRESS, NODE_URL, WALLET_URL, WEB_WALLET_URL } from "./config";
import {
  callContract,
  getBaseFee,
  getContractStorage,
  getNativeBalance,
  getTokenAllowance,
  getTokenBalance,
  getTokenMeta,
  sendContractTx,
} from "./api";
import { loadTokens, saveTokens, upsertToken } from "./storage";

// ─── Constants ────────────────────────────────────────────────────────────────
const SECS_PER_DAY = 86400;
const TABS = ["Swap", "Pool", "Validate", "Settings"];
const SLIPPAGE_PRESETS = ["0.1", "0.5", "1.0"];
const ZERO_ADDR = "0x0000000000000000000000000000000000000000";

// ─── Helpers ──────────────────────────────────────────────────────────────────
function safeBig(v) {
  try { return (!v || v === "") ? 0n : BigInt(v); }
  catch { return 0n; }
}

// Uniswap v2 AMM quote (mirrors dex_swap.go): 0.3% fee
function quoteOut(amtIn, resIn, resOut) {
  if (amtIn <= 0n || resIn <= 0n || resOut <= 0n) return 0n;
  const fee = amtIn * 997n;
  return (fee * resOut) / (resIn * 1000n + fee);
}

// Price impact in basis-points
function priceImpactBps(amtIn, resIn) {
  if (resIn <= 0n || amtIn <= 0n) return 0;
  return Number((amtIn * 10000n) / resIn);
}

function fmtBps(bps) {
  return (bps / 100).toFixed(2);
}

// Format raw base-units → human string  e.g. "10000000000" (8 dec) → "100"
function fmtAmount(raw, decimals = 8) {
  if (!raw || raw === "0") return "0";
  try {
    const big = BigInt(raw);
    const d = BigInt(10 ** decimals);
    const whole = big / d;
    const frac = big % d;
    const fracStr = frac.toString().padStart(decimals, "0").replace(/0+$/, "");
    return fracStr ? `${whole}.${fracStr}` : whole.toString();
  } catch { return raw; }
}

// Parse human string → raw base-unit string  e.g. "100" (8 dec) → "10000000000"
// Pure string arithmetic — no Number() precision loss, safe for any size.
function parseHuman(humanStr, decimals = 8) {
  if (!humanStr && humanStr !== 0) return "0";
  const s = String(humanStr).trim();
  if (!s || s === "0") return "0";
  const dotIdx = s.indexOf(".");
  let intS, fracS;
  if (dotIdx === -1) { intS = s; fracS = ""; }
  else { intS = s.slice(0, dotIdx); fracS = s.slice(dotIdx + 1); }
  const frac = fracS.slice(0, decimals).padEnd(decimals, "0");
  const full = (intS.replace(/^0+/, "") || "0") + frac;
  return full.replace(/^0+/, "") || "0";
}

// Shorten address
function shortAddr(a) {
  if (!a || a.length < 10) return a || "";
  return `${a.slice(0, 6)}…${a.slice(-4)}`;
}

function extractHash(res) {
  return res?.tx_hash || res?.TxHash || res?.hash || "";
}

// ─── Token icon letter ─────────────────────────────────────────────────────
function TokenIcon({ symbol, size = 22 }) {
  const letter = (symbol || "?")[0].toUpperCase();
  const hue = ((symbol || "?").charCodeAt(0) * 47 + 120) % 360;
  return (
    <div
      className="token-icon"
      style={{
        width: size, height: size, fontSize: size * 0.45,
        background: `hsl(${hue},60%,45%)`,
      }}
    >
      {letter}
    </div>
  );
}

// ─── App ──────────────────────────────────────────────────────────────────────
export default function App() {
  const [tab, setTab] = useState("Swap");

  // Wallet
  const [wallet, setWallet] = useState({ address: "", privateKey: "" });
  const [usingExt, setUsingExt] = useState(false);

  // Tokens
  const [tokens, setTokens] = useState(loadTokens);
  const [tokenA, setTokenA] = useState("");
  const [tokenB, setTokenB] = useState("");

  // DEX config
  const [dexAddr, setDexAddr] = useState(() => localStorage.getItem("lqd_dex_address") || DEX_CONTRACT_ADDRESS);
  const [nodeUrl, setNodeUrl]   = useState(() => localStorage.getItem("lqd_node_url") || NODE_URL);
  const [walletUrl, setWalletUrl] = useState(() => localStorage.getItem("lqd_wallet_url") || WALLET_URL);
  const [pairAddr, setPairAddr] = useState("");

  // Pool state
  const [pool, setPool] = useState({ reserveA: "0", reserveB: "0", totalLP: "0", lpBalance: "0", tokenA: "", tokenB: "" });
  const [balances, setBalances] = useState({ a: "", b: "" });
  const [allowances, setAllowances] = useState({ a: "0", b: "0" });
  const [baseFee, setBaseFee] = useState(0);

  // Swap state
  const [amtIn,   setAmtIn]   = useState("");
  const [amtOut,  setAmtOut]  = useState("");
  const [impact,  setImpact]  = useState(0); // basis-points
  const [swapDir, setSwapDir] = useState("AtoB");

  // Settings (slippage)
  const [slippage, setSlippage]       = useState("0.5");
  const [customSlip, setCustomSlip]   = useState("");
  const [showSettings, setShowSettings] = useState(false);

  // Liquidity UI state (sub-screen: null | "add" | "remove")
  const [liqScreen, setLiqScreen] = useState(null);
  const [liqA, setLiqA] = useState("");
  const [liqB, setLiqB] = useState("");
  const [lpBurn, setLpBurn] = useState("");

  // Validator
  const [valDays,     setValDays]     = useState("30");
  const [valLPAmt,    setValLPAmt]    = useState("");
  const [valInfo,     setValInfo]     = useState(null);

  // Wallet dropdown
  const [showWalletMenu, setShowWalletMenu] = useState(false);
  const walletMenuRef = useRef(null);

  // Tracks optimistic allowance floor — interval polling never goes below this
  const minAllowanceRef = useRef({ a: "0", b: "0" });

  // Token selector modal
  const [modalOpen, setModalOpen] = useState(false);
  const [modalTarget, setModalTarget] = useState("A");
  const [modalSearch, setModalSearch] = useState("");
  const [importAddr, setImportAddr] = useState("");

  // Loading / status
  const [loading, setLoading] = useState(false);
  const [toast, setToast] = useState({ msg: "", type: "" });
  const toastTimer = useRef(null);

  const popupRef = useRef(null);
  const settingsRef = useRef(null);

  // Derived
  const symA  = tokens.find(t => t.address === tokenA)?.symbol || "–";
  const symB  = tokens.find(t => t.address === tokenB)?.symbol || "–";
  const decA  = parseInt(tokens.find(t => t.address === tokenA)?.decimals || "8", 10) || 8;
  const decB  = parseInt(tokens.find(t => t.address === tokenB)?.decimals || "8", 10) || 8;
  const outDec = swapDir === "AtoB" ? decB : decA;
  const activeSlip = customSlip || slippage;
  // amtOut is already human-readable; convert → raw for slippage maths
  const minReceived = amtOut && activeSlip
    ? fmtAmount(
        ((safeBig(parseHuman(amtOut, outDec)) * BigInt(Math.floor((1 - parseFloat(activeSlip) / 100) * 10000))) / 10000n).toString(),
        outDec
      )
    : "";

  // Wallet is considered connected if address is set + can sign (pk or extension)
  const walletConnected = !!(wallet.address && (wallet.privateKey || (typeof window !== "undefined" && window.lqd)));
  const canSend = !!(walletConnected && dexAddr);
  // Pool is inited if reserves exist (r0 > 0) OR if pair was created (totalLP key exists)
  const poolInited = !!(tokenA && tokenB && (safeBig(pool.reserveA) > 0n || safeBig(pool.reserveB) > 0n || pool.totalLP !== "0"));

  // ── Toast helper ─────────────────────────────────────────────────────────
  function showToast(msg, type = "info") {
    setToast({ msg, type });
    clearTimeout(toastTimer.current);
    toastTimer.current = setTimeout(() => setToast({ msg: "", type: "" }), 5000);
  }

  // ── Quote ─────────────────────────────────────────────────────────────────
  // amtIn is human-readable; convert to raw units before AMM math
  useEffect(() => {
    const inDec  = swapDir === "AtoB" ? decA : decB;
    const _outDec = swapDir === "AtoB" ? decB : decA;
    const rawIn  = safeBig(parseHuman(amtIn, inDec));
    const resA   = safeBig(pool.reserveA);
    const resB   = safeBig(pool.reserveB);

    if (swapDir === "AtoB") {
      const rawOut = quoteOut(rawIn, resA, resB);
      setAmtOut(rawOut > 0n ? fmtAmount(rawOut.toString(), _outDec) : "");
      setImpact(priceImpactBps(rawIn, resA));
    } else {
      const rawOut = quoteOut(rawIn, resB, resA);
      setAmtOut(rawOut > 0n ? fmtAmount(rawOut.toString(), _outDec) : "");
      setImpact(priceImpactBps(rawIn, resB));
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [amtIn, pool.reserveA, pool.reserveB, swapDir, decA, decB]);

  // ── Extension auto-connect + accountsChanged reactive listener ──────────
  useEffect(() => {
    // Apply an accounts array to React state
    function applyAccounts(accs) {
      const a = Array.isArray(accs) ? accs[0] : "";
      if (a) {
        setWallet(w => w.address === a ? w : { address: a, privateKey: "" });
        setUsingExt(true);
      } else {
        setUsingExt(false);
      }
    }

    // 1. Background pushes LQD_ACCOUNTS on connect (handled by accountsChanged)
    // 2. Also poll lqd_accounts once as fallback for timing edge-cases
    async function tryConnect() {
      if (typeof window === "undefined" || !window.lqd) return;
      try {
        const accs = await window.lqd.request({ method: "lqd_accounts" });
        applyAccounts(accs);
      } catch {}
    }

    // Wait a tick so injected.js + content.js port are fully ready
    const t = setTimeout(tryConnect, 150);

    // React to all account changes (push from background, approval, lock/unlock)
    function onAccountsChanged(accs) { applyAccounts(accs); }

    function setup() {
      if (!window.lqd) return;
      window.lqd.on("accountsChanged", onAccountsChanged);
    }

    // lqd might not be ready synchronously — listen for the init event too
    setup();
    window.addEventListener("lqd#initialized", setup, { once: true });

    return () => {
      clearTimeout(t);
      if (window.lqd) window.lqd.removeListener("accountsChanged", onAccountsChanged);
      window.removeEventListener("lqd#initialized", setup);
    };
  }, []);

  // ── Postmessage wallet connect ────────────────────────────────────────────
  useEffect(() => {
    function onMsg(e) {
      if (!e.data || typeof e.data !== "object") return;
      if (e.data.type === "LQD_WALLET_CONNECT") {
        const { address, privateKey } = e.data;
        if (address && privateKey) {
          setWallet({ address, privateKey });
          setUsingExt(false);
          showToast("Web wallet connected", "success");
        }
      }
    }
    window.addEventListener("message", onMsg);
    return () => window.removeEventListener("message", onMsg);
  }, []);

  // ── Pool polling — resolves pair address from factory, then reads pair storage ──
  const refreshPool = useCallback(async () => {
    if (!dexAddr || !tokenA || !tokenB) return;
    try {
      const factoryData = await getContractStorage(dexAddr);
      const factoryStorage = factoryData?.State?.storage ?? factoryData?.State ?? {};
      const normA = tokenA.toLowerCase();
      const normB = tokenB.toLowerCase();
      const pk = normA < normB ? `${normA}:${normB}` : `${normB}:${normA}`;
      const nextPairAddr = factoryStorage[`pairAddr:${pk}`] || "";
      setPairAddr(nextPairAddr);
      if (!nextPairAddr) {
        setPool({ reserveA: "0", reserveB: "0", totalLP: "0", lpBalance: "0", tokenA: "", tokenB: "" });
        return;
      }

      const pairData = await getContractStorage(nextPairAddr);
      const s = pairData?.State?.storage ?? pairData?.State ?? {};
      const r0 = s.reserve0 || "0";
      const r1 = s.reserve1 || "0";
      const totalLP = s.totalLP || "0";
      const t0 = s.token0 || (normA < normB ? normA : normB);
      const t1 = s.token1 || (normA < normB ? normB : normA);

      // Align reserves to tokenA/tokenB order
      let reserveA, reserveB;
      if (normA === t0) { reserveA = r0; reserveB = r1; }
      else               { reserveA = r1; reserveB = r0; }

      // LP balance
      let lpBal = "0";
      if (wallet.address) {
        const lpKey = `lp:${wallet.address.toLowerCase()}`;
        const key = Object.keys(s).find(k => k.toLowerCase() === lpKey.toLowerCase());
        lpBal = (key && s[key]) ? s[key] : "0";
      }

      setPool({ reserveA, reserveB, totalLP, lpBalance: lpBal, tokenA: t0, tokenB: t1 });

      // auto-fetch token meta for non-native tokens
      const fetchMeta = async (addr) => {
        if (!addr || addr === "lqd" || !wallet.address) return;
        try {
          const m = await getTokenMeta(addr, wallet.address);
          upsertToken({ address: addr, name: m.name || "Token", symbol: m.symbol || addr.slice(2, 6).toUpperCase(), decimals: m.decimals || "8" });
          setTokens(loadTokens());
        } catch {}
      };
      fetchMeta(t0); fetchMeta(t1);
    } catch {
      setPairAddr("");
    }
  }, [dexAddr, tokenA, tokenB, wallet.address]);

  useEffect(() => {
    refreshPool();
    const id = setInterval(refreshPool, 3000);
    return () => clearInterval(id);
  }, [refreshPool]);

  // ── Refresh allowances (declared before useEffect that uses it) ───────────
  const refreshAllowances = useCallback(async () => {
    if (!wallet.address || !dexAddr) return;
    const next = { a: "0", b: "0" };
    try {
      const spender = pairAddr || "";
      // Native LQD needs no approval — treat allowance as max
      if (tokenA === "lqd") next.a = "999999999999999999";
      else if (spender && tokenA?.startsWith("0x") && tokenA.length === 42) next.a = await getTokenAllowance(tokenA, wallet.address, spender);
      if (tokenB === "lqd") next.b = "999999999999999999";
      else if (spender && tokenB?.startsWith("0x") && tokenB.length === 42) next.b = await getTokenAllowance(tokenB, wallet.address, spender);
    } catch {}
    // Always take the max of fetched and the optimistic floor (ref persists across renders)
    const floorA = minAllowanceRef.current.a;
    const floorB = minAllowanceRef.current.b;
    const resolvedA = safeBig(next.a) >= safeBig(floorA) ? next.a : floorA;
    const resolvedB = safeBig(next.b) >= safeBig(floorB) ? next.b : floorB;
    // Once on-chain confirms (fetched >= floor), clear the floor
    if (safeBig(next.a) >= safeBig(floorA)) minAllowanceRef.current.a = "0";
    if (safeBig(next.b) >= safeBig(floorB)) minAllowanceRef.current.b = "0";
    setAllowances({ a: resolvedA, b: resolvedB });
  }, [wallet.address, tokenA, tokenB, dexAddr, pairAddr]);

  // ── Balances ──────────────────────────────────────────────────────────────
  const refreshBalances = useCallback(async () => {
    if (!wallet.address) { setBalances({ a: "", b: "" }); return; }
    const next = { a: "", b: "" };
    try {
      if (tokenA) next.a = await getTokenBalance(tokenA, wallet.address);
      if (tokenB) next.b = await getTokenBalance(tokenB, wallet.address);
    } catch {}
    setBalances(next);
  }, [wallet.address, tokenA, tokenB]);

  useEffect(() => {
    refreshBalances();
    const id = setInterval(refreshBalances, 4000);
    return () => clearInterval(id);
  }, [refreshBalances]);

  // Poll allowances every 3s so they stay fresh after Approve TXs
  useEffect(() => {
    refreshAllowances();
    const id = setInterval(refreshAllowances, 3000);
    return () => clearInterval(id);
  }, [refreshAllowances]);

  // ── Base fee ──────────────────────────────────────────────────────────────
  useEffect(() => {
    const load = async () => { try { setBaseFee((await getBaseFee()) || 0); } catch {} };
    load();
    const id = setInterval(load, 5000);
    return () => clearInterval(id);
  }, []);

  // ── Validator info ────────────────────────────────────────────────────────
  useEffect(() => {
    if (tab !== "Validate" || !wallet.address || !dexAddr || !tokenA || !tokenB) return;
    callContract({ address: dexAddr, caller: wallet.address, fn: "GetValidatorLP", args: [tokenA, tokenB, wallet.address], value: "0" })
      .then(r => setValInfo(r))
      .catch(() => setValInfo(null));
  }, [tab, wallet.address, dexAddr, tokenA, tokenB]);

  // ── Close menus on outside click ─────────────────────────────────────────
  useEffect(() => {
    function handler(e) {
      if (settingsRef.current && !settingsRef.current.contains(e.target)) setShowSettings(false);
      if (walletMenuRef.current && !walletMenuRef.current.contains(e.target)) setShowWalletMenu(false);
    }
    if (showSettings || showWalletMenu) document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [showSettings]);

  // ── Actions ───────────────────────────────────────────────────────────────
  async function sendTx(contractAddress, fn, args, successMsg, nativeValue = "0") {
    setLoading(true);
    try {
      const res = await sendContractTx({
        address: wallet.address, privateKey: wallet.privateKey,
        contractAddress, fn, args, value: nativeValue,
        gasPrice: baseFee ? baseFee + 1 : 0,
        onPending: () => showToast("🔔 Open the LQD Wallet extension popup to approve this transaction", "info")
      });
      const hash = extractHash(res);
      showToast(`${successMsg}${hash ? " · " + shortAddr(hash) : ""}`, "success");
      // Refresh balances + pool after TX mines
      setTimeout(() => { refreshPool(); refreshAllowances(); refreshBalances(); }, 2000);
      setTimeout(() => { refreshPool(); refreshAllowances(); refreshBalances(); }, 5000);
      return true;
    } catch (err) {
      showToast(err.message || "Transaction failed", "error");
      return false;
    } finally {
      setLoading(false);
    }
  }

  // dec = token decimals so we convert human → raw before approving
  async function doApprove(tokenAddr, humanAmt, dec = 8) {
    if (!canSend || !tokenAddr) return;
    if (!pairAddr) { showToast("Create the pair first so approvals target the pool contract", "error"); return; }
    const rawAmt = parseHuman(humanAmt, dec) || "0";
    const ok = await sendTx(tokenAddr, "Approve", [pairAddr, rawAmt], "Approved");
    if (ok) {
      // Set optimistic floor in ref — survives re-renders until on-chain confirms
      if (tokenAddr === tokenA) minAllowanceRef.current.a = rawAmt;
      if (tokenAddr === tokenB) minAllowanceRef.current.b = rawAmt;
      // Optimistic UI update
      setAllowances(prev => ({
        ...prev,
        ...(tokenAddr === tokenA ? { a: rawAmt } : {}),
        ...(tokenAddr === tokenB ? { b: rawAmt } : {})
      }));
    }
  }

  async function doInitPool() {
    if (!dexAddr) { showToast("Set DEX contract address in Settings first", "error"); return false; }
    if (!tokenA || !tokenB) { showToast("Select both tokens first", "error"); return false; }
    // Factory uses CreatePair(tokenA, tokenB)
    const ok = await sendTx(dexAddr, "CreatePair", [tokenA, tokenB], "Pair created — waiting for confirmation…");
    return ok;
  }

  async function doSwap() {
    if (!canSend) { showToast("Connect wallet first", "error"); return; }
    if (!poolInited) { showToast("Pool not initialized — click Init Pool in Pool tab", "error"); return; }
    if (!amtIn || parseFloat(amtIn) <= 0) { showToast("Enter an amount", "error"); return; }

    // Convert human-readable input → raw base units for contract
    const inDec = swapDir === "AtoB" ? decA : decB;
    const rawIn = parseHuman(amtIn, inDec);

    const slipBps = parseFloat(activeSlip) * 100;
    if (impact > slipBps * 100) {
      const ok = window.confirm(`Price impact (${fmtBps(impact)}%) exceeds your slippage tolerance (${activeSlip}%). Continue?`);
      if (!ok) return;
    }

    const tokenIn  = swapDir === "AtoB" ? tokenA : tokenB;
    const tokenOut = swapDir === "AtoB" ? tokenB : tokenA;
    const isNativeIn = tokenIn === "lqd";

    // Native LQD doesn't need approval
    if (!isNativeIn) {
      const needApprove = swapDir === "AtoB"
        ? safeBig(allowances.a) < safeBig(rawIn)
        : safeBig(allowances.b) < safeBig(rawIn);
      if (needApprove) {
        showToast(`Approve ${swapDir === "AtoB" ? symA : symB} first`, "error");
        return;
      }
    }

    // minAmountOut with slippage (raw units)
    const outDecNow = swapDir === "AtoB" ? decB : decA;
    const rawOutBig = safeBig(parseHuman(amtOut, outDecNow));
    const slip = parseFloat(activeSlip) / 100;
    const minOut = ((rawOutBig * BigInt(Math.floor((1 - slip) * 10000))) / 10000n).toString();

    const nativeValue = isNativeIn ? rawIn : "0";
    const ok = await sendTx(
      dexAddr, "SwapExactTokensForTokens",
      [rawIn, minOut, tokenIn, tokenOut],
      "Swap submitted",
      nativeValue
    );
    if (ok) { setAmtIn(""); setAmtOut(""); }
  }

  async function doAddLiquidity() {
    if (!canSend) { showToast("Connect wallet first", "error"); return; }
    if (!tokenA || !tokenB) { showToast("Select both tokens first", "error"); return; }
    if (!liqA || !liqB) { showToast("Enter both amounts", "error"); return; }
    const rawA = parseHuman(liqA, decA);
    const rawB = parseHuman(liqB, decB);

    // Check allowances — skip for native LQD (no approval needed)
    await refreshAllowances();
    if (tokenA !== "lqd") {
      const freshA = await getTokenAllowance(tokenA, wallet.address, pairAddr).catch(() => "0");
      if (safeBig(freshA) < safeBig(rawA)) { showToast(`Approve ${symA} first`, "error"); return; }
    }
    if (tokenB !== "lqd") {
      const freshB = await getTokenAllowance(tokenB, wallet.address, pairAddr).catch(() => "0");
      if (safeBig(freshB) < safeBig(rawB)) { showToast(`Approve ${symB} first`, "error"); return; }
    }

    // If tokenA or tokenB is native LQD, pass its amount as tx value
    let nativeValue = "0";
    if (tokenA === "lqd") nativeValue = rawA;
    else if (tokenB === "lqd") nativeValue = rawB;

    // Factory: AddLiquidity(tokenA, tokenB, amountA, amountB)
    const ok = await sendTx(dexAddr, "AddLiquidity", [tokenA, tokenB, rawA, rawB], "Liquidity added", nativeValue);
    if (ok) { setLiqA(""); setLiqB(""); setLiqScreen(null); }
  }

  async function doRemoveLiquidity() {
    if (!canSend) { showToast("Connect wallet first", "error"); return; }
    if (!lpBurn) { showToast("Enter LP amount to burn", "error"); return; }
    const rawLp = parseHuman(lpBurn, 8);  // LP tokens always 8 decimals
    // Factory: RemoveLiquidity(tokenA, tokenB, lpAmount)
    const ok = await sendTx(dexAddr, "RemoveLiquidity", [tokenA, tokenB, rawLp], "Liquidity removed");
    if (ok) { setLpBurn(""); setLiqScreen(null); }
  }

  async function doLockLP() {
    if (!canSend) { showToast("Connect wallet first", "error"); return; }
    if (!tokenA || !tokenB) { showToast("Select the pool tokens first", "error"); return; }
    if (!valLPAmt) { showToast("Enter LP amount", "error"); return; }
    const days = parseInt(valDays, 10);
    if (!days || days <= 0) { showToast("Invalid lock duration", "error"); return; }
    const rawLP = parseHuman(valLPAmt, 8);  // LP tokens always 8 decimals
    await sendTx(dexAddr, "LockLPForValidation", [tokenA, tokenB, rawLP, (days * SECS_PER_DAY).toString()], "LP locked for validation");
  }

  async function doUnlockLP() {
    if (!canSend) return;
    if (!tokenA || !tokenB) { showToast("Select the pool tokens first", "error"); return; }
    await sendTx(dexAddr, "UnlockValidatorLP", [tokenA, tokenB], "LP unlocked");
  }

  async function doImportToken() {
    if (!importAddr || !wallet.address) { showToast("Enter token address", "error"); return; }
    try {
      const m = await getTokenMeta(importAddr, wallet.address);
      const list = upsertToken({ address: importAddr, name: m.name || "Token", symbol: m.symbol || importAddr.slice(2, 6).toUpperCase(), decimals: m.decimals || "8" });
      setTokens(list);
      setImportAddr("");
      showToast("Token imported", "success");
    } catch (err) { showToast(err.message || "Import failed", "error"); }
  }

  async function connectExtension() {
    if (!window.lqd) { showToast("LQD Wallet extension not found. Install it first.", "error"); return; }
    try {
      showToast("Waiting for approval in LQD Wallet…", "info");
      const accs = await window.lqd.request({
        method: "lqd_connect",
        // onPending fires when background queues the approval request
        onPending: () => showToast("Open the LQD Wallet extension popup to approve", "info"),
      });
      const a = Array.isArray(accs) ? accs[0] : "";
      if (a) {
        setWallet({ address: a, privateKey: "" });
        setUsingExt(true);
        showToast("Wallet connected: " + shortAddr(a), "success");
      } else {
        showToast("No account returned from extension", "error");
      }
    } catch (err) {
      const msg = err?.message || "Connect failed";
      if (msg.toLowerCase().includes("rejected")) {
        showToast("Connection rejected by user", "error");
      } else {
        showToast(msg, "error");
      }
    }
  }

  function connectWebWallet() {
    popupRef.current = window.open(WEB_WALLET_URL, "lqd_wallet", "width=420,height=720");
  }

  function disconnectWallet() {
    setWallet({ address: "", privateKey: "" });
    setUsingExt(false);
    setBalances({ a: "", b: "" });
    setAllowances({ a: "0", b: "0" });
    setShowWalletMenu(false);
    showToast("Wallet disconnected", "info");
  }

  function swapSides() {
    setTokenA(tokenB); setTokenB(tokenA);
    setSwapDir(d => d === "AtoB" ? "BtoA" : "AtoB");
    setAmtIn(amtOut); setAmtOut(amtIn);
  }

  function openTokenModal(target) { setModalTarget(target); setModalSearch(""); setModalOpen(true); }
  function selectToken(addr) {
    if (modalTarget === "A") setTokenA(addr);
    else setTokenB(addr);
    setModalOpen(false);
  }

  // ── Filtered token list ───────────────────────────────────────────────────
  const filteredTokens = tokens.filter(t => {
    const q = modalSearch.toLowerCase();
    return t.address.toLowerCase().includes(q) || t.symbol.toLowerCase().includes(q) || t.name.toLowerCase().includes(q);
  });

  // ── Pool share pct ─────────────────────────────────────────────────────────
  const totalLPBig = safeBig(pool.totalLP);
  const lpBalBig   = safeBig(pool.lpBalance);
  const sharePct   = totalLPBig > 0n && lpBalBig > 0n
    ? Number((lpBalBig * 10000n) / totalLPBig) / 100
    : 0;

  // ── Impact colour ─────────────────────────────────────────────────────────
  const impactClass = impact < 100 ? "impact-low" : impact < 500 ? "impact-mid" : "impact-high";

  // ── Spot price display ────────────────────────────────────────────────────
  const resA = safeBig(pool.reserveA);
  const resB = safeBig(pool.reserveB);
  const spotPrice = resA > 0n
    ? (Number(resB * 10000n / resA) / 10000).toFixed(4)
    : null;

  // ════════════════════════════════════════════════════════════════════════════
  return (
    <div className="app">

      {/* ── Header ─────────────────────────────────────────────────────────── */}
      <header className="header">
        <div className="brand">
          <div className="brand-logo">L</div>
          <div>
            <span className="brand-name">LQD Swap</span>
            <span className="brand-sub">Proof of Dynamic Liquidity</span>
          </div>
        </div>

        <nav className="nav-tabs">
          {TABS.map(t => (
            <button key={t} className={tab === t ? "active" : ""} onClick={() => setTab(t)}>{t}</button>
          ))}
        </nav>

        <div className="header-right">
          {wallet.address ? (
            <div style={{ position: "relative" }} ref={walletMenuRef}>
              <button
                className="wallet-btn"
                onClick={() => setShowWalletMenu(m => !m)}
                title="Wallet options"
              >
                <span className="dot" style={{ background: walletConnected ? "var(--green)" : "var(--yellow)" }} />
                {shortAddr(wallet.address)}
                <span style={{ fontSize: 10, marginLeft: 2, color: "var(--muted)" }}>▾</span>
              </button>

              {showWalletMenu && (
                <div className="wallet-dropdown">
                  <div className="wallet-dropdown-addr">
                    <span className="dot" style={{ width: 8, height: 8, borderRadius: "50%", background: "var(--green)", display: "inline-block", marginRight: 6 }} />
                    {wallet.address}
                  </div>
                  <button className="wallet-dropdown-item" onClick={() => { navigator.clipboard.writeText(wallet.address); showToast("Address copied!", "success"); setShowWalletMenu(false); }}>
                    📋 Copy Address
                  </button>
                  <button className="wallet-dropdown-item" onClick={() => { setTab("Settings"); setShowWalletMenu(false); }}>
                    ⚙ Settings
                  </button>
                  <div className="wallet-dropdown-divider" />
                  <button className="wallet-dropdown-item disconnect" onClick={disconnectWallet}>
                    🔌 Disconnect
                  </button>
                </div>
              )}
            </div>
          ) : (
            <button className="wallet-btn" onClick={connectExtension}>
              <span className="dot red" />
              Connect Wallet
            </button>
          )}
        </div>
      </header>

      {/* ── Mobile bottom tabs ───────────────────────────────────────────── */}
      <nav className="mobile-tabs">
        {TABS.map(t => (
          <button key={t} className={tab === t ? "active" : ""} onClick={() => setTab(t)}>{t}</button>
        ))}
      </nav>

      {/* ══════════════════ SWAP TAB ══════════════════════════════════════ */}
      {tab === "Swap" && (
        <main className="page">
          <div style={{ width: "100%", maxWidth: 480 }}>
            <div className="card">
              {/* Card header */}
              <div className="card-header">
                <span className="card-title">Swap</span>
                <div style={{ position: "relative" }} ref={settingsRef}>
                  <button className="icon-btn" onClick={() => setShowSettings(s => !s)} title="Settings">⚙</button>
                  {showSettings && (
                    <div className="settings-popover">
                      <h3>Slippage Tolerance</h3>
                      <div className="slippage-btns">
                        {SLIPPAGE_PRESETS.map(p => (
                          <button key={p} className={slippage === p && !customSlip ? "active" : ""}
                            onClick={() => { setSlippage(p); setCustomSlip(""); }}
                          >{p}%</button>
                        ))}
                      </div>
                      <input className="settings-input" placeholder="Custom %" type="number" step="0.1"
                        value={customSlip} onChange={e => setCustomSlip(e.target.value)} />
                      <div className="settings-row">
                        <span>Active: {activeSlip}%</span>
                        {parseFloat(activeSlip) > 5 && <span style={{ color: "var(--yellow)" }}>⚠ High</span>}
                      </div>
                    </div>
                  )}
                </div>
              </div>

              {/* You Pay */}
              <div className="token-box">
                <div className="token-box-label">You Pay</div>
                <div className="token-box-row">
                  <input
                    className="token-amount-input"
                    type="number"
                    placeholder="0"
                    value={amtIn}
                    onChange={e => setAmtIn(e.target.value)}
                  />
                  <button
                    className={`token-select-btn${!tokenA ? " unset" : ""}`}
                    onClick={() => openTokenModal("A")}
                  >
                    {tokenA ? <><TokenIcon symbol={symA} />{symA}</> : "Select token"}
                    <span className="arrow">▾</span>
                  </button>
                </div>
                <div className="token-balance">
                  <span>Balance: {balances.a ? fmtAmount(balances.a, decA) : "–"}</span>
                  {balances.a && balances.a !== "0" && (
                    <span className="balance-max" onClick={() => setAmtIn(fmtAmount(balances.a, decA))}>MAX</span>
                  )}
                </div>
              </div>

              {/* Swap direction arrow */}
              <div className="swap-arrow-wrap">
                <button className="swap-arrow-btn" onClick={swapSides}>↕</button>
              </div>

              {/* You Receive */}
              <div className="token-box">
                <div className="token-box-label">You Receive</div>
                <div className="token-box-row">
                  <input
                    className="token-amount-input"
                    type="number"
                    placeholder="0"
                    value={amtOut}
                    readOnly
                  />
                  <button
                    className={`token-select-btn${!tokenB ? " unset" : ""}`}
                    onClick={() => openTokenModal("B")}
                  >
                    {tokenB ? <><TokenIcon symbol={symB} />{symB}</> : "Select token"}
                    <span className="arrow">▾</span>
                  </button>
                </div>
                <div className="token-balance">
                  <span>Balance: {balances.b ? fmtAmount(balances.b, decB) : "–"}</span>
                </div>
              </div>

              {/* Price info strip */}
              {spotPrice && amtIn && amtOut && (
                <div className="price-strip">
                  <div className="price-row">
                    <span className="price-label">Rate</span>
                    <span>1 {symA} = {spotPrice} {symB}</span>
                  </div>
                  <div className="price-row">
                    <span className="price-label">Price Impact</span>
                    <span className={impactClass}>{fmtBps(impact)}%</span>
                  </div>
                  <div className="price-row">
                    <span className="price-label">Fee (0.3%)</span>
                    <span>{fmtAmount((safeBig(parseHuman(amtIn, swapDir === "AtoB" ? decA : decB)) * 3n / 1000n).toString(), swapDir === "AtoB" ? decA : decB)} {swapDir === "AtoB" ? symA : symB}</span>
                  </div>
                  <div className="price-row">
                    <span className="price-label">Min Received ({activeSlip}% slippage)</span>
                    <span>{fmtAmount(minReceived)} {symB}</span>
                  </div>
                  <div className="price-row">
                    <span className="price-label">Route</span>
                    <span>{symA} → {symB}</span>
                  </div>
                </div>
              )}

              {/* Approve buttons (only shown if needed) */}
              {swapDir === "AtoB" && tokenA && safeBig(allowances.a) < safeBig(parseHuman(amtIn, decA)) && amtIn && (
                <button className="action-btn secondary" onClick={() => doApprove(tokenA, amtIn, decA)} disabled={loading}>
                  Approve {symA}
                </button>
              )}
              {swapDir === "BtoA" && tokenB && safeBig(allowances.b) < safeBig(parseHuman(amtIn, decB)) && amtIn && (
                <button className="action-btn secondary" onClick={() => doApprove(tokenB, amtIn, decB)} disabled={loading}>
                  Approve {symB}
                </button>
              )}

              {/* Main swap button — label matches the actual blocking reason */}
              <button
                className={`action-btn${impact > 1500 ? " warn" : ""}`}
                onClick={!walletConnected ? connectExtension : doSwap}
                disabled={loading || (walletConnected && (!dexAddr || !amtIn))}
              >
                {loading
                  ? <span className="spinner" />
                  : !walletConnected
                    ? "Connect Wallet"
                    : !dexAddr
                      ? "Set DEX Address in Settings ⚙"
                      : !tokenA || !tokenB
                        ? "Select Tokens"
                        : !amtIn
                          ? "Enter Amount"
                          : "Swap"}
              </button>
            </div>

            {/* Pool info below swap */}
            {poolInited && (
              <div className="notice" style={{ marginTop: 12, fontSize: 12 }}>
                <strong>Pool:</strong> Reserve {symA}: {fmtAmount(pool.reserveA)} · Reserve {symB}: {fmtAmount(pool.reserveB)} · Total LP: {fmtAmount(pool.totalLP)}
              </div>
            )}
          </div>
        </main>
      )}

      {/* ══════════════════ POOL TAB ══════════════════════════════════════ */}
      {tab === "Pool" && (
        <main className="page">
          <div className="page-wide">

            {/* ── Add liquidity sub-screen ─────────────────────────────────── */}
            {liqScreen === "add" && (
              <div className="liq-panel">
                <div className="liq-panel-header">
                  <button className="back-btn" onClick={() => setLiqScreen(null)}>←</button>
                  <span className="card-title">Add Liquidity</span>
                </div>

                <div className="liq-section">
                  <div className="liq-label">Token {symA !== "–" ? symA : "A"}</div>
                  <div className="token-box" style={{ marginBottom: 10 }}>
                    <div className="token-box-row">
                      <input className="token-amount-input" type="number" placeholder="0"
                        value={liqA} onChange={e => setLiqA(e.target.value)} />
                      <button className="token-select-btn" onClick={() => openTokenModal("A")}>
                        {tokenA ? <><TokenIcon symbol={symA} />{symA}</> : "Select"}
                        <span className="arrow">▾</span>
                      </button>
                    </div>
                    <div className="token-balance">Balance: {balances.a ? fmtAmount(balances.a, decA) : "–"}</div>
                  </div>

                  <div style={{ textAlign: "center", marginBottom: 10, color: "var(--muted)" }}>+</div>

                  <div className="liq-label">Token {symB !== "–" ? symB : "B"}</div>
                  <div className="token-box" style={{ marginBottom: 16 }}>
                    <div className="token-box-row">
                      <input className="token-amount-input" type="number" placeholder="0"
                        value={liqB} onChange={e => setLiqB(e.target.value)} />
                      <button className="token-select-btn" onClick={() => openTokenModal("B")}>
                        {tokenB ? <><TokenIcon symbol={symB} />{symB}</> : "Select"}
                        <span className="arrow">▾</span>
                      </button>
                    </div>
                    <div className="token-balance">Balance: {balances.b ? fmtAmount(balances.b, decB) : "–"}</div>
                  </div>

                  {/* Approve if needed */}
                  <div style={{ display: "flex", gap: 8, marginBottom: 10 }}>
                    {safeBig(allowances.a) < safeBig(parseHuman(liqA, decA)) && liqA && (
                      <button className="action-btn secondary" style={{ margin: 0 }} onClick={() => doApprove(tokenA, liqA, decA)} disabled={loading}>
                        Approve {symA}
                      </button>
                    )}
                    {safeBig(allowances.b) < safeBig(parseHuman(liqB, decB)) && liqB && (
                      <button className="action-btn secondary" style={{ margin: 0 }} onClick={() => doApprove(tokenB, liqB, decB)} disabled={loading}>
                        Approve {symB}
                      </button>
                    )}
                  </div>

                  {!poolInited && (
                    <button className="action-btn secondary" style={{ margin: "0 0 10px" }} onClick={doInitPool} disabled={loading}>
                      Init Pool First
                    </button>
                  )}

                  <button className="action-btn" style={{ margin: 0 }} onClick={doAddLiquidity} disabled={loading || !canSend}>
                    {loading ? <span className="spinner" /> : "Add Liquidity"}
                  </button>
                </div>
              </div>
            )}

            {/* ── Remove liquidity sub-screen ──────────────────────────────── */}
            {liqScreen === "remove" && (
              <div className="liq-panel">
                <div className="liq-panel-header">
                  <button className="back-btn" onClick={() => setLiqScreen(null)}>←</button>
                  <span className="card-title">Remove Liquidity</span>
                </div>
                <div className="liq-section">
                  <div className="pool-stat" style={{ marginBottom: 16 }}>
                    <div className="pool-stat-label">Your LP Balance</div>
                    <div className="pool-stat-value">{fmtAmount(pool.lpBalance)}</div>
                  </div>
                  {sharePct > 0 && (
                    <>
                      <div className="liq-label">Your Pool Share: {sharePct.toFixed(2)}%</div>
                      <div className="liq-share-bar">
                        <div className="liq-share-fill" style={{ width: Math.min(sharePct, 100) + "%" }} />
                      </div>
                    </>
                  )}
                  <div className="field" style={{ marginTop: 16 }}>
                    <label>LP Amount to Burn</label>
                    <input type="number" placeholder="0" value={lpBurn} onChange={e => setLpBurn(e.target.value)} />
                  </div>
                  {lpBurn && pool.totalLP !== "0" && (
                    <div className="notice" style={{ marginBottom: 12 }}>
                      You will receive approx{" "}
                      {fmtAmount((safeBig(parseHuman(lpBurn, 8)) * safeBig(pool.reserveA) / safeBig(pool.totalLP)).toString(), decA)} {symA}
                      {" + "}
                      {fmtAmount((safeBig(parseHuman(lpBurn, 8)) * safeBig(pool.reserveB) / safeBig(pool.totalLP)).toString(), decB)} {symB}
                    </div>
                  )}
                  <button className="action-btn" style={{ margin: 0 }} onClick={doRemoveLiquidity} disabled={loading || !canSend}>
                    {loading ? <span className="spinner" /> : "Remove Liquidity"}
                  </button>
                </div>
              </div>
            )}

            {/* ── Default pool view ────────────────────────────────────────── */}
            {!liqScreen && (
              <>
                <div className="pool-header">
                  <h2>Liquidity</h2>
                  <button className="pool-btn primary" style={{ width: "auto", padding: "10px 20px" }}
                    onClick={() => setLiqScreen("add")}>
                    + New Position
                  </button>
                </div>

                {poolInited ? (
                  <div className="pool-card">
                    <div className="pool-pair">
                      <div className="pool-icons">
                        <TokenIcon symbol={symA} size={28} />
                        <TokenIcon symbol={symB} size={28} />
                      </div>
                      <div>
                        <div className="pool-pair-name">{symA} / {symB}</div>
                        <div className="pool-pair-sub">0.3% fee · Uniswap v2 AMM</div>
                      </div>
                    </div>

                    <div className="pool-stats">
                      <div className="pool-stat">
                        <div className="pool-stat-label">Reserve {symA}</div>
                        <div className="pool-stat-value">{fmtAmount(pool.reserveA)}</div>
                      </div>
                      <div className="pool-stat">
                        <div className="pool-stat-label">Reserve {symB}</div>
                        <div className="pool-stat-value">{fmtAmount(pool.reserveB)}</div>
                      </div>
                      <div className="pool-stat">
                        <div className="pool-stat-label">Total LP</div>
                        <div className="pool-stat-value">{fmtAmount(pool.totalLP)}</div>
                      </div>
                      <div className="pool-stat">
                        <div className="pool-stat-label">Your LP</div>
                        <div className="pool-stat-value">{fmtAmount(pool.lpBalance)}</div>
                      </div>
                    </div>

                    {sharePct > 0 && (
                      <div className="notice" style={{ marginBottom: 12 }}>
                        Your share: <strong>{sharePct.toFixed(2)}%</strong>
                        {" · "}{fmtAmount((lpBalBig * resA / totalLPBig).toString())} {symA}
                        {" + "}{fmtAmount((lpBalBig * resB / totalLPBig).toString())} {symB}
                      </div>
                    )}

                    <div className="pool-actions">
                      <button className="pool-btn primary" onClick={() => setLiqScreen("add")}>Add</button>
                      <button className="pool-btn secondary" onClick={() => setLiqScreen("remove")}>Remove</button>
                    </div>
                  </div>
                ) : (
                  <div className="pool-card" style={{ textAlign: "center", padding: "40px 20px" }}>
                    <div style={{ fontSize: 48, marginBottom: 12 }}>💧</div>
                    <div style={{ fontSize: 18, fontWeight: 700, marginBottom: 8 }}>No pool yet</div>
                    <div style={{ color: "var(--text2)", marginBottom: 20 }}>
                      Create a new liquidity pool by adding your first position.
                    </div>
                    <button className="pool-btn primary" style={{ maxWidth: 200, margin: "0 auto" }}
                      onClick={() => setLiqScreen("add")}>
                      Create Pool
                    </button>
                  </div>
                )}
              </>
            )}
          </div>
        </main>
      )}

      {/* ══════════════════ VALIDATE TAB ══════════════════════════════════ */}
      {tab === "Validate" && (
        <main className="page">
          <div className="validate-panel">

            <div className="validate-card">
              <h3>⚡ Become a PoDL Validator</h3>
              <p>
                Lock your DEX LP tokens as validator stake. Your consensus power is derived
                from <strong>real market liquidity</strong>, not single-asset staking.
                The longer you lock, the more LiquidityPower you earn.
              </p>

              {valInfo?.output && (
                <div className="validator-info-box">
                  <div className="info-row"><span>Locked LP</span><span className="info-val">{fmtAmount(valInfo.output.lockedLP || "0")}</span></div>
                  <div className="info-row"><span>Pool Backing</span><span className="info-val">{fmtAmount(valInfo.output.poolBacking || "0")}</span></div>
                  <div className="info-row">
                    <span>Lock Until</span>
                    <span className="info-val">
                      {valInfo.output.lockUntil ? new Date(valInfo.output.lockUntil * 1000).toLocaleString() : "—"}
                    </span>
                  </div>
                  <div className="info-row"><span>Active</span><span className="info-val">{valInfo.output.isActive ? "✓ Yes" : "✗ No"}</span></div>
                </div>
              )}

              <div className="field">
                <label>LP Amount to Lock</label>
                <input type="number" placeholder="e.g. 1000000000"
                  value={valLPAmt} onChange={e => setValLPAmt(e.target.value)} />
                <div className="notice" style={{ marginTop: 6, fontSize: 12 }}>Your LP: {fmtAmount(pool.lpBalance)}</div>
              </div>

              <div className="field">
                <label>Lock Duration (days)</label>
                <input type="number" placeholder="30" value={valDays} onChange={e => setValDays(e.target.value)} />
              </div>

              <div style={{ display: "flex", gap: 8 }}>
                <button className="action-btn" style={{ margin: 0 }} onClick={doLockLP} disabled={loading || !canSend}>
                  {loading ? <span className="spinner" /> : "Lock LP"}
                </button>
                <button className="action-btn secondary" style={{ margin: 0 }} onClick={doUnlockLP} disabled={loading || !canSend}>
                  Unlock LP
                </button>
              </div>
            </div>

            <div className="validate-card">
              <h3>📖 How PoDL Works</h3>
              <div className="notice">
                <strong>1. Add Liquidity</strong> in the Pool tab<br />
                <strong>2. Lock LP tokens</strong> here for your chosen duration<br />
                <strong>3. Start your node</strong> with:<br />
                <code style={{ display: "block", marginTop: 8, wordBreak: "break-all" }}>
                  go run main.go chain -validator {wallet.address || "0x..."} -dex_address {dexAddr} -lp_token_amount {valLPAmt || "AMOUNT"}
                </code><br />
                <strong>Power Formula:</strong><br />
                <code>Power = (lockedLP / totalLP) × (resA + resB) × (1 + lockYears)</code>
              </div>
            </div>
          </div>
        </main>
      )}

      {/* ══════════════════ SETTINGS TAB ══════════════════════════════════ */}
      {tab === "Settings" && (
        <main className="page">
          <div style={{ width: "100%", maxWidth: 480 }}>

            {/* Wallet */}
            <div className="settings-section">
              <h3>Wallet</h3>
              <div className="field">
                <label>Address</label>
                <input value={wallet.address} onChange={e => setWallet(w => ({ ...w, address: e.target.value }))} placeholder="0x..." />
              </div>
              {!usingExt && (
                <div className="field">
                  <label>Private Key</label>
                  <input type="password" value={wallet.privateKey}
                    onChange={e => setWallet(w => ({ ...w, privateKey: e.target.value }))}
                    placeholder="0x... (never share)" />
                </div>
              )}
              <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
                <button className="action-btn secondary" style={{ margin: 0 }} onClick={connectWebWallet}>Web Wallet</button>
                <button className="action-btn secondary" style={{ margin: 0 }} onClick={connectExtension}>LQD Extension</button>
                {wallet.address && (
                  <button
                    className="action-btn secondary"
                    style={{ margin: 0, borderColor: "var(--red)", color: "var(--red)" }}
                    onClick={disconnectWallet}
                  >
                    🔌 Disconnect
                  </button>
                )}
              </div>
              {usingExt && <div className="notice success" style={{ marginTop: 10 }}>✓ Using LQD Wallet extension</div>}
              {wallet.address && (
                <div className="notice" style={{ marginTop: 10, fontSize: 12, wordBreak: "break-all" }}>
                  Connected: {wallet.address}
                  <span
                    style={{ marginLeft: 8, color: "var(--accent2)", cursor: "pointer" }}
                    onClick={() => { navigator.clipboard.writeText(wallet.address); showToast("Copied!", "success"); }}
                  >📋 copy</span>
                </div>
              )}
            </div>

            {/* Endpoints */}
            <div className="settings-section">
              <h3>API Endpoints</h3>
              <div className="field">
                <label>Node URL (aggregator / chain)</label>
                <input value={nodeUrl} onChange={e => setNodeUrl(e.target.value)} />
              </div>
              <div className="field">
                <label>Wallet Server URL</label>
                <input value={walletUrl} onChange={e => setWalletUrl(e.target.value)} />
              </div>
              <div className="settings-row-item">
                <span className="settings-row-label">Base Fee</span>
                <span>{baseFee}</span>
              </div>
              <button className="action-btn secondary" style={{ margin: "12px 0 0" }} onClick={() => {
                localStorage.setItem("lqd_node_url", nodeUrl);
                localStorage.setItem("lqd_wallet_url", walletUrl);
                showToast("Endpoints saved", "success");
              }}>
                Save Endpoints
              </button>
            </div>

            {/* DEX contract */}
            <div className="settings-section">
              <h3>DEX Contract</h3>
              <div className="field">
                <label>DEX Address</label>
                <input
                  value={dexAddr}
                  onChange={e => { setDexAddr(e.target.value); localStorage.setItem("lqd_dex_address", e.target.value); }}
                  placeholder="0x... (deploy DEX contract first)"
                  style={{ borderColor: !dexAddr ? "var(--red)" : undefined }}
                />
              </div>
              {!dexAddr
                ? <div className="notice error">⚠ No DEX address set. Deploy the DEX contract and paste its address here to enable swapping.</div>
                : <div className="notice">✓ DEX contract configured.</div>
              }
            </div>

            {/* Token import */}
            <div className="settings-section">
              <h3>Token Import</h3>
              <div className="modal-import-row" style={{ margin: "0 0 12px" }}>
                <input value={importAddr} onChange={e => setImportAddr(e.target.value)} placeholder="Token contract address 0x..." />
                <button onClick={doImportToken}>Import</button>
              </div>
              <div className="token-list" style={{ maxHeight: 200 }}>
                {tokens.map(t => (
                  <div key={t.address} className="token-row">
                    <TokenIcon symbol={t.symbol} />
                    <div className="token-row-info">
                      <div className="token-row-sym">{t.symbol}</div>
                      <div className="token-row-name">{t.name} · {t.address}</div>
                    </div>
                    {t.native
                      ? <div className="token-row-bal" style={{ color: "var(--accent2)", fontSize: 11 }}>native</div>
                      : <button
                          style={{ background: "none", border: "none", color: "var(--red)", cursor: "pointer", fontSize: 14, padding: "0 4px" }}
                          onClick={() => {
                            const list = tokens.filter(x => x.address !== t.address);
                            saveTokens(list.filter(x => !x.native));
                            setTokens(loadTokens());
                            if (tokenA === t.address) setTokenA("");
                            if (tokenB === t.address) setTokenB("");
                          }}
                          title="Remove token"
                        >✕</button>
                    }
                  </div>
                ))}
              </div>
              {tokens.filter(t => !t.native).length > 0 && (
                <button
                  className="action-btn secondary"
                  style={{ margin: "10px 0 0", borderColor: "var(--red)", color: "var(--red)" }}
                  onClick={() => {
                    saveTokens([]);
                    setTokens(loadTokens());
                    setTokenA(""); setTokenB("");
                    showToast("Token list cleared", "success");
                  }}
                >
                  Clear All Tokens
                </button>
              )}
            </div>
          </div>
        </main>
      )}

      {/* ── Token selector modal ──────────────────────────────────────────── */}
      {modalOpen && (
        <div className="modal-overlay" onClick={() => setModalOpen(false)}>
          <div className="modal-box" onClick={e => e.stopPropagation()}>
            <div className="modal-top">
              <h2>Select a Token</h2>
              <button className="icon-btn" onClick={() => setModalOpen(false)}>✕</button>
            </div>
            <input className="modal-search" placeholder="Search name or address"
              value={modalSearch} onChange={e => setModalSearch(e.target.value)} autoFocus />
            <div className="modal-import-row">
              <input value={importAddr} onChange={e => setImportAddr(e.target.value)} placeholder="Import by address 0x..." />
              <button onClick={doImportToken}>Import</button>
            </div>
            <div className="token-list">
              {filteredTokens.length === 0 && (
                <div style={{ padding: "20px", textAlign: "center", color: "var(--muted)" }}>No tokens found</div>
              )}
              {filteredTokens.map(t => (
                <div key={t.address} className={`token-row${(modalTarget === "A" ? tokenA : tokenB) === t.address ? " selected" : ""}`}
                  onClick={() => selectToken(t.address)}>
                  <TokenIcon symbol={t.symbol} />
                  <div className="token-row-info">
                    <div className="token-row-sym">{t.symbol}</div>
                    <div className="token-row-name">{t.name}</div>
                  </div>
                  <div className="token-row-bal" style={{ fontSize: 11, color: "var(--muted)" }}>
                    {t.address.slice(0, 8)}…
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}

      {/* ── Toast ────────────────────────────────────────────────────────── */}
      {toast.msg && (
        <div className={`toast-bar ${toast.type}`} onClick={() => setToast({ msg: "", type: "" })}>
          {toast.type === "success" && "✓ "}
          {toast.type === "error" && "✗ "}
          {toast.msg}
        </div>
      )}
    </div>
  );
}
