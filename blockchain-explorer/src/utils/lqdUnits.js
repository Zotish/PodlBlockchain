/* global BigInt */
// // import { formatUnits, parseUnits } from "ethers";

// // export const LQD_DECIMALS = 8;

// // export function formatLQD(v) {
// //   if (v == null) return "0";
// //   return formatUnits(BigInt(v), LQD_DECIMALS);
// // }

// // export function parseLQD(v) {
// //   return parseUnits(String(v || "0"), LQD_DECIMALS).toString();
// // }


// import { formatUnits, parseUnits  } from "ethers";

// export const LQD_DECIMALS = 8;

// // Accept number/string/bigint and safely convert
// export function toBigIntSafe(v) {
//   if (v == null) return 0n;
//   if (typeof v === "bigint") return v;
//   if (typeof v === "number") return BigInt(Math.trunc(v)); // only safe for integers
//   if (typeof v === "string") {
//     if (v.trim() === "") return 0n;
//     // if backend ever returns numeric strings
//     if (/^\d+$/.test(v.trim())) return BigInt(v.trim());
//   }
//   // fallback
//   try { return BigInt(v); } catch { return 0n; }
// }

// export function formatLQD(v, decimals = LQD_DECIMALS) {
//   return formatUnits(toBigIntSafe(v), decimals);
// }

// export function parseLQD(human, decimals = LQD_DECIMALS) {
//   // human = "1.25" -> bigint base units
//   return parseUnits(String(human ?? "0"), decimals);
// }
import { formatUnits, parseUnits } from "ethers";

export const LQD_DECIMALS = 8;

// Accept number/string/bigint and safely convert to bigint base units
export function toBigIntSafe(v, decimals = LQD_DECIMALS) {
  if (v == null) return 0n;
  if (typeof v === "bigint") return v;
  if (typeof v === "number") {
    if (!Number.isFinite(v)) return 0n;
    if (Number.isInteger(v)) return BigInt(v);
    return parseUnits(String(v), decimals);
  }
  if (typeof v === "string") {
    const s = v.trim();
    if (s === "") return 0n;
    if (s.includes(".")) {
      return parseUnits(s, decimals);
    }
    if (/^\d+$/.test(s)) return BigInt(s);
    try {
      return BigInt(s);
    } catch {
      return 0n;
    }
  }
  return 0n;
}

export function formatLQD(v, decimals = LQD_DECIMALS) {
  return formatUnits(toBigIntSafe(v, decimals), decimals);
}

export function parseLQD(v, decimals = LQD_DECIMALS) {
  return parseUnits(String(v ?? "0"), decimals).toString();
}

// Pure string version — no ethers dependency, safe for any environment.
// "1.5" with 8 decimals → "150000000"
// "100"  with 8 decimals → "10000000000"
export function parseHuman(humanStr, decimals = LQD_DECIMALS) {
  if (!humanStr && humanStr !== 0) return "0";
  const s = String(humanStr).trim();
  if (!s || s === "0") return "0";
  const dotIdx = s.indexOf(".");
  let intS = dotIdx === -1 ? s : s.slice(0, dotIdx);
  let fracS = dotIdx === -1 ? "" : s.slice(dotIdx + 1);
  const frac = fracS.slice(0, decimals).padEnd(decimals, "0");
  const full = (intS.replace(/^0+/, "") || "0") + frac;
  return full.replace(/^0+/, "") || "0";
}

// Returns true when a uint ABI parameter is a token amount field (by name heuristic)
export function isAmountParam(inputDef) {
  const n = (inputDef?.name || "").toLowerCase();
  const t = (inputDef?.type || "").toLowerCase();
  if (!t.startsWith("uint") && !t.startsWith("int")) return false;
  return (
    n.includes("amount") || n.includes("value") ||
    n.includes("supply") || n.includes("qty") ||
    n.includes("quantity") || n.includes("balance")
  );
}
