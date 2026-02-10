// // //  ///* eslint-disable no-undef */
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

export const formatLQD = (v) =>
  formatUnits(v?.toString() || "0", LQD_DECIMALS);

export const parseLQD = (v) =>
  parseUnits(String(v ?? "0"), LQD_DECIMALS).toString();
