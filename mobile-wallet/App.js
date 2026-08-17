import "react-native-get-random-values";
if (typeof global.crypto === 'undefined') {
  global.crypto = {
    getRandomValues: (byteArray) => {
      for (let i = 0; i < byteArray.length; i++) {
        byteArray[i] = Math.floor(Math.random() * 256);
      }
      return byteArray;
    },
  };
}
import React, { useEffect, useMemo, useRef, useState } from "react";
import {
  ActivityIndicator,
  Animated,
  Alert,
  AppState,
  Dimensions,
  FlatList,
  Image,
  Keyboard,
  Linking,
  KeyboardAvoidingView,
  Modal,
  PixelRatio,
  Platform,
  Pressable,
  RefreshControl,
  SafeAreaView,
  ScrollView,
  Share,
  StyleSheet,
  Switch,
  Text,
  TextInput,
  TouchableOpacity,
  View,
} from "react-native";
import { StatusBar } from "expo-status-bar";
import * as Clipboard from "expo-clipboard";
import * as LocalAuthentication from "expo-local-authentication";
import * as FileSystem from "expo-file-system/legacy";
import { CameraView, useCameraPermissions } from "expo-camera";
import * as SecureStore from "expo-secure-store";
import CryptoJS from "crypto-js";
import QRCode from "react-native-qrcode-svg";
import { WebView } from "react-native-webview";

const { width: SCREEN_WIDTH, height: SCREEN_HEIGHT } = Dimensions.get("window");
const scale = (size) => (SCREEN_WIDTH / 375) * size;

import {
  getJson,
  dexRegistryConfig,
  dexRegistryPools,
  dexRegistryTokens,
  nodeBaseFee,
  nodeBridgeRequests,
  nodeBridgeFamilies,
  nodeBridgeChainRemove,
  nodeBridgeChainUpsert,
  nodeBridgeChains,
  nodeBridgeTokenRemove,
  nodeBridgeTokenUpsert,
  nodeBridgeTokens,
  discoverAptosCoins,
  discoverCosmosTokens,
  discoverNearTokens,
  discoverSolanaTokens,
  discoverSuiCoins,
  nodeCallContract,
  nodeCompilePlugin,
  nodeContractAbi,
  nodeContractStorage,
  nodeCurrentFactory,
  nodeDeployBuiltin,
  nodeDeployContract,
  nodeEstimateGas,
  nodeFaucet,
  nodeLiquidityPools,
  nodeRecentTransactions,
  nodeStatus,
  normalizeUrl,
  postJson,
  resolveTokenBalanceMultichain,
  resolveTokenMetaMultichain,
  walletBalance,
  walletBridgeBurn,
  walletBridgeBurnLqdToken,
  walletBridgePrivateBurn,
  walletBridgePrivateBurnLqdToken,
  walletBridgeLock,
  walletBridgeLockBscToken,
  walletBridgePrivateLock,
  walletBridgePrivateLockBscToken,
  walletCreate,
  walletContractTx,
  walletImportMnemonic,
  walletImportPrivateKey,
  walletSend,
} from "./src/api";
import { STORAGE_KEYS, loadJSON, loadString, removeItem, runSecureStorageAudit, saveJSON, saveString } from "./src/storage";
import { runWalletMigrations } from "./src/migrations";
import { installGlobalErrorReporter, recordError, recordTelemetry } from "./src/telemetry";
import { createTrackedTransaction, mergeTrackedTransaction, refreshTrackedTransactions } from "./src/txTracker";
import {
  deriveCosmosLikeAddress,
  deriveHarmonyAddress,
  deriveTronAddress,
  formatDate,
  formatUnits,
  isLikelyAddress,
  isLikelyAddressForFamily,
  mergeUniqueByKey,
  parseUnits,
  shortAddress,
  tronAddressToEvm,
  txTouchesAddress,
} from "./src/utils";
import {
  deriveAllChainKeys,
  signEip155Tx,
  signPersonalMessage,
  signEd25519Message,
  signSecp256k1Message,
  signCosmosAminoDoc,
  ed25519PublicKey,
  secp256k1CompressedPublicKey,
  encodeErc20Transfer,
  signCosmosTx,
  signCosmosWasmExecuteTx,
  signSolanaTransfer,
  signSolanaTokenTransfer,
  signNearTransfer,
  signNearFunctionCall,
  signAptosEntry,
  signSuiTx,
  signBtcP2WPKHTx,
  exportKeyInfo,
  base58Encode as cryptoBase58Encode,
} from "./src/crypto";
import { nftKey, normalizeNftUri, resolveEvmNftMetadata, resolveNftUriMetadata, validateNftIdentifier } from "./src/nft";
import {
  buildTonTransferBoc,
  buildJettonTransferBoc,
  getTonSeqno,
  getTonJettonWalletAddress,
  getTonJettonBalance,
  broadcastTonBoc,
} from "./src/ton";
import {
  starknetArgentAddress,
  broadcastStarknetTransfer,
  broadcastStarknetTokenTransfer,
  starkSign,
} from "./src/starknet";

// Convert hex string to Uint8Array (used for NEAR pubkey base58 conversion)
const hexToBytes = (hex) => {
  const h = hex.startsWith("0x") ? hex.slice(2) : hex;
  const u8 = new Uint8Array(h.length / 2);
  for (let i = 0; i < u8.length; i++) u8[i] = parseInt(h.slice(i * 2, i * 2 + 2), 16);
  return u8;
};

const PROD_CHAIN_URL = "https://chain.178-105-133-94.sslip.io";
const PROD_WALLET_URL = "https://wallet.178-105-133-94.sslip.io";

const BASE58_ALPHABET = '123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz';
const encodeBase58 = (hex) => {
  let num = BigInt('0x' + hex);
  let res = '';
  while (num > 0n) {
    res = BASE58_ALPHABET[Number(num % 58n)] + res;
    num /= 58n;
  }
  return res;
};

// Returns a cryptographically valid chain address from the EVM private key,
// or "" for chains that need a different key type (ed25519).
const deriveFamilyAddress = (privKey, family) => {
  if (!privKey || !family) return "";
  const fam = String(family).toLowerCase();
  if (fam === "evm") return ""; // caller should use wallet.address directly
  try {
    if (fam === "cosmos" || fam === "cosmos-testnet")
      return deriveCosmosLikeAddress(privKey, "cosmos", CryptoJS);
    if (fam === "sei")
      return deriveCosmosLikeAddress(privKey, "sei", CryptoJS);
    if (fam === "injective")
      return deriveCosmosLikeAddress(privKey, "inj", CryptoJS);
    if (fam === "tron")
      return deriveTronAddress(privKey, CryptoJS);
    // Remaining families use the mnemonic-derived keys from deriveAllChainKeys.
    return "";
  } catch { return ""; }
};
const PROD_AGGREGATOR_URL = "https://api.178-105-133-94.sslip.io";
const PROD_EXPLORER_URL = "https://warm-dragon-34d6ff.netlify.app";
const DEFAULT_BROWSER_URL = PROD_EXPLORER_URL;
const DEFAULT_TIMEOUT_MS = 30000;

const DEFAULT_NETWORKS = [
  {
    id: "lqd-mainnet",
    chainId: "0x8b",
    name: "LQD Mainnet",
    symbol: "LQD",
    nodeUrl: PROD_CHAIN_URL,
    walletUrl: PROD_WALLET_URL,
    explorerUrl: PROD_EXPLORER_URL,
    aggregatorUrl: PROD_AGGREGATOR_URL,
  },
  {
    id: "lqd-agg",
    chainId: "0x8c",
    name: "LQD Aggregator",
    symbol: "LQD",
    nodeUrl: PROD_AGGREGATOR_URL,
    walletUrl: PROD_WALLET_URL,
    explorerUrl: PROD_EXPLORER_URL,
    aggregatorUrl: PROD_AGGREGATOR_URL,
  },
];

const DEFAULT_ENDPOINTS = {
  nodeUrl: PROD_CHAIN_URL,
  walletUrl: PROD_WALLET_URL,
  aggregatorUrl: PROD_AGGREGATOR_URL,
  explorerUrl: PROD_EXPLORER_URL,
};

const LP_REWARD_TIERS = [
  {
    tier: "Tier 1",
    title: "Stable Core",
    pairs: "LQD/USDT · LQD/USDC",
    detail: "Highest stable-liquidity reward for deep market pairs.",
    weight: "1.25x",
    accent: "#10b981",
  },
  {
    tier: "Tier 2",
    title: "Blue-Chip",
    pairs: "LQD/ETH · LQD/BNB · LQD/BTC",
    detail: "Medium/high reward for volatile canonical liquidity.",
    weight: "0.90x-1.00x",
    accent: "#60a5fa",
  },
  {
    tier: "Tier 3",
    title: "Community",
    pairs: "Admin approved pools",
    detail: "Community pools earn only after governance/admin approval.",
    weight: "0.35x default",
    accent: "#f59e0b",
  },
];

function migrateLocalEndpoint(value, fallback) {
  const current = String(value || "").trim();
  if (!current) return fallback;
  if (current.includes("127.0.0.1") || current.includes("localhost") || current.includes("0.0.0.0")) {
    if (current.includes(":8080")) return PROD_WALLET_URL;
    if (current.includes(":9000")) return PROD_AGGREGATOR_URL;
    if (current.includes(":3001")) return PROD_EXPLORER_URL;
    return PROD_CHAIN_URL;
  }
  return current;
}

function migrateNetworkId(value) {
  const id = String(value || "").trim();
  const renamed = {
    "berachain-artio": "berachain-bepolia",
    "celo-alfajores": "celo-sepolia",
    "starknet-testnet": "starknet-sepolia",
  };
  return renamed[id] || id;
}

const BUILTIN_TEMPLATES = [
  { value: "lqd20", label: "LQD20 Token" },
  { value: "dex_swap", label: "DEX Pair" },
  { value: "dex_factory", label: "DEX Factory" },
  { value: "dex_router", label: "DEX Router" },
  { value: "bridge_token", label: "Bridge Token" },
  { value: "lending_pool", label: "Lending Pool" },
  { value: "nft_collection", label: "NFT Collection" },
  { value: "dao_treasury", label: "DAO Treasury" },
];

const QUICK_ARGS = {
  lqd20: [
    { key: "q_name", label: "Token Name", ph: "My Token" },
    { key: "q_sym", label: "Symbol", ph: "MTK" },
    { key: "q_supply", label: "Initial Supply", ph: "1000000000000000" }
  ],
  dex_swap: [
    { key: "tokenA", label: "Token A Address", ph: "0x..." },
    { key: "tokenB", label: "Token B Address", ph: "0x..." }
  ],
  bridge_token: [
    { key: "q_bname", label: "Token Name", ph: "Wrapped BNB" },
    { key: "q_bsym", label: "Symbol", ph: "wBNB" },
    { key: "q_bdec", label: "Decimals", ph: "18" },
    { key: "q_bbsc", label: "BSC Token Address", ph: "0x..." }
  ],
  nft_collection: [
    { key: "q_nname", label: "Collection Name", ph: "My NFT" },
    { key: "q_nsym", label: "Symbol", ph: "MNFT" }
  ],
  dao_treasury: [
    { key: "daoName", label: "DAO Name", ph: "My DAO" }
  ],
  lending_pool: [
    { key: "lendingToken", label: "Lending Asset", ph: "LQD or token address" }
  ],
  dex_factory: [],
  dex_router: []
};

const TABS = [
  { id: "home", label: "Wallet", icon: "👛" },
  { id: "bridge", label: "Bridge", icon: "🌉" },
  { id: "browser", label: "Browser", icon: "🌐" },
  { id: "contracts", label: "Contract", icon: "📝" },
  { id: "liquidity", label: "Liquidity", icon: "💧" },
  { id: "settings", label: "Settings", icon: "⚙️" },
];

const LP_CANONICAL_POOL_SLOTS = [
  { id: "lqd-usdt", token: "USDT", label: "LQD / USDT", tier: "Tier 1", weight: "1.25x" },
  { id: "lqd-usdc", token: "USDC", label: "LQD / USDC", tier: "Tier 1", weight: "1.25x" },
  { id: "lqd-eth", token: "ETH", label: "LQD / ETH", tier: "Tier 2", weight: "1.00x" },
  { id: "lqd-bnb", token: "BNB", label: "LQD / BNB", tier: "Tier 2", weight: "0.90x" },
  { id: "lqd-btc", token: "BTC", label: "LQD / BTC", tier: "Tier 2", weight: "1.00x" },
];

const ADVANCED_TABS = [
  { id: "approvals", label: "Approvals" },
  { id: "networks", label: "Networks" },
  { id: "faucet", label: "Faucet" },
];

const EMPTY_VAULT = {
  address: "",
  privateKey: "",
  mnemonic: "",
};

const DEFAULT_CUSTOM_SOURCE = `package main

import (
  "fmt"
  bc "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/BlockchainComponent"
)

type ContractTemplate struct{}

var Contract = &ContractTemplate{}

func (c *ContractTemplate) Init(ctx *bc.Context) {
  ctx.Set("hello", "world")
}

func (c *ContractTemplate) Ping(ctx *bc.Context) {
  ctx.Set("output", fmt.Sprintf("pong:%s", ctx.CallerAddr))
}`;

function encryptVault(vault, password) {
  return CryptoJS.AES.encrypt(JSON.stringify(vault), password).toString();
}

function decryptVault(cipher, password) {
  const raw = CryptoJS.AES.decrypt(cipher, password).toString(CryptoJS.enc.Utf8);
  if (!raw) {
    throw new Error("Wrong password or damaged vault");
  }
  return JSON.parse(raw);
}

function coerceBrowserUrl(value) {
  const raw = String(value || "").trim();
  if (!raw) return DEFAULT_BROWSER_URL;
  if (/^https?:\/\//i.test(raw)) return raw;
  return `https://${raw}`;
}

function isDefaultBrowserUrl(value) {
  const clean = String(value || "").trim().replace(/\/+$/, "");
  const home = String(DEFAULT_BROWSER_URL || "").trim().replace(/\/+$/, "");
  return clean === home;
}

function normalizeAddress(value) {
  const raw = String(value || "").trim();
  return isLikelyAddress(raw) ? raw.toLowerCase() : "";
}

function tokenCandidateFromValue(value) {
  if (Array.isArray(value)) {
    return value.flatMap(tokenCandidateFromValue);
  }
  if (!value || typeof value !== "object") {
    return normalizeAddress(value) ? [normalizeAddress(value)] : [];
  }

  const directKeys = [
    "contract",
    "Contract",
    "contract_address",
    "contractAddress",
    "token",
    "Token",
    "token_address",
    "tokenAddress",
    "lqd_token",
    "lqdToken",
    "target_token",
    "targetToken",
    "address",
    "Address",
  ];
  const pairKeys = ["tokenA", "token_a", "TokenA", "tokenB", "token_b", "TokenB"];
  const candidates = [];

  for (const key of [...directKeys, ...pairKeys]) {
    const candidate = normalizeAddress(value[key]);
    if (candidate) candidates.push(candidate);
  }

  if (value.args || value.Args) candidates.push(...tokenCandidateFromValue(value.args || value.Args));
  if (value.extra_data || value.ExtraData) candidates.push(...tokenCandidateFromValue(value.extra_data || value.ExtraData));
  return candidates;
}

function Card({ title, subtitle, children, style }) {
  return (
    <View style={[styles.card, style]}>
      {(title || subtitle) ? (
        <View style={styles.cardHeader}>
          <View style={{ flex: 1 }}>
            {title ? <Text style={styles.cardTitle}>{title}</Text> : null}
            {subtitle ? <Text style={styles.cardSubtitle}>{subtitle}</Text> : null}
          </View>
        </View>
      ) : null}
      {children}
    </View>
  );
}

function Field({ label, value, onChangeText, placeholder, secureTextEntry, multiline, autoCapitalize = "none", keyboardType, right, editable = true, numberOfLines = 1 }) {
  let finalPlaceholder = placeholder;
  if (!finalPlaceholder && label) {
    if (label.toLowerCase().includes("address")) finalPlaceholder = "0x...";
    else if (label.toLowerCase().includes("amount")) finalPlaceholder = "0.0";
    else if (label.toLowerCase().includes("password")) finalPlaceholder = "••••••••";
    else if (label.toLowerCase().includes("token")) finalPlaceholder = "0x...";
  }

  return (
    <View style={styles.fieldWrap}>
      <View style={styles.fieldLabelRow}>
        <Text style={styles.fieldLabel}>{label}</Text>
        {right}
      </View>
      <TextInput
        value={value}
        onChangeText={onChangeText}
        placeholder={finalPlaceholder}
        placeholderTextColor="#7680a8"
        secureTextEntry={secureTextEntry}
        multiline={multiline}
        autoCapitalize={autoCapitalize}
        keyboardType={keyboardType}
        editable={editable}
        numberOfLines={numberOfLines}
        style={[styles.input, multiline && styles.inputMultiline, !editable && styles.inputReadonly]}
      />
    </View>
  );
}

function Button({ label, onPress, secondary, disabled, compact, danger }) {
  return (
    <Pressable
      onPress={onPress}
      disabled={disabled}
      style={({ pressed }) => [
        styles.button,
        secondary && styles.buttonSecondary,
        danger && styles.buttonDanger,
        compact && styles.buttonCompact,
        disabled && styles.buttonDisabled,
        pressed && !disabled && styles.buttonPressed,
      ]}
    >
      <Text style={[styles.buttonText, secondary && styles.buttonTextSecondary, danger && styles.buttonTextDanger]}>
        {label}
      </Text>
    </Pressable>
  );
}

function Chip({ label, active, onPress }) {
  return (
    <Pressable onPress={onPress} style={[styles.chip, active && styles.chipActive]}>
      <Text style={[styles.chipText, active && styles.chipTextActive]}>{label}</Text>
    </Pressable>
  );
}

function NavItem({ icon, label, active, onPress }) {
  return (
    <Pressable onPress={onPress} style={({ pressed }) => [styles.navItem, active && styles.navItemActive, pressed && styles.buttonPressed]}>
      <Text style={[styles.navIcon, active && styles.navIconActive]}>{icon}</Text>
      <Text style={[styles.navLabel, active && styles.navLabelActive]} numberOfLines={1}>
        {label}
      </Text>
    </Pressable>
  );
}

function Stat({ label, value, subvalue }) {
  return (
    <View style={styles.stat}>
      <Text style={styles.statLabel}>{label}</Text>
      <Text style={styles.statValue}>{value}</Text>
      {subvalue ? <Text style={styles.statSub}>{subvalue}</Text> : null}
    </View>
  );
}

function LPRewardTierGuide() {
  return (
    <Card
      title="LP Reward Tiers"
      subtitle="Only approved LQD pairs receive block LP rewards. Add liquidity in DEX, then track rewards from your wallet."
    >
      <View style={styles.lpTierList}>
        {LP_REWARD_TIERS.map((item) => (
          <View key={item.tier} style={[styles.lpTierRow, { borderColor: `${item.accent}55` }]}>
            <View style={[styles.lpTierDot, { backgroundColor: item.accent }]} />
            <View style={{ flex: 1 }}>
              <Text style={styles.lpTierKicker}>{item.tier} · {item.title}</Text>
              <Text style={styles.lpTierPairs}>{item.pairs}</Text>
              <Text style={styles.lpTierDetail}>{item.detail}</Text>
            </View>
            <Text style={[styles.lpTierWeight, { color: item.accent }]}>{item.weight}</Text>
          </View>
        ))}
      </View>
    </Card>
  );
}

function safeBig(raw) {
  try {
    if (raw === null || raw === undefined || raw === "") return 0n;
    if (typeof raw === "number") return BigInt(Math.trunc(raw));
    return BigInt(String(raw));
  } catch {
    return 0n;
  }
}

function normalizeContractStorage(data) {
  return data?.State?.storage || data?.State || data?.storage || data || {};
}

function findStorageValue(storage, key, fallback = "") {
  if (!storage || !key) return fallback;
  if (storage[key] !== undefined) return storage[key];
  const lower = String(key).toLowerCase();
  const found = Object.keys(storage).find((item) => String(item).toLowerCase() === lower);
  return found ? storage[found] : fallback;
}

function normalizePoolMap(poolsPayload) {
  const raw = poolsPayload?.pools || poolsPayload?.Pools || {};
  return raw && typeof raw === "object" && !Array.isArray(raw) ? raw : {};
}

function tokenLabel(value) {
  const token = String(value || "").trim();
  if (!token) return "Token";
  if (token.toLowerCase() === "lqd") return "LQD";
  if (token.startsWith("0x")) return shortAddress(token);
  return token.toUpperCase();
}

function normalizePoolAsset(value) {
  const token = String(value || "").trim();
  if (!token) return "";
  if (token.startsWith("0x")) return token.toLowerCase();
  return token.toUpperCase();
}

function canonicalSlotForForm(form, registryEntry = {}) {
  const pairKey = String(registryEntry?.pairKey || registryEntry?.pair_key || "")
    .trim()
    .toUpperCase()
    .replace(/\s+/g, "");
  if (pairKey) {
    const normalizedPair = pairKey.replace("-", "/");
    const byPair = LP_CANONICAL_POOL_SLOTS.find(
      (slot) => slot.label.toUpperCase().replace(/\s+/g, "") === normalizedPair
    );
    if (byPair) return byPair;
  }
  const a = normalizePoolAsset(form?.tokenA);
  const b = normalizePoolAsset(form?.tokenB);
  if (a !== "LQD" && b !== "LQD") return null;
  const other = a === "LQD" ? b : a;
  return LP_CANONICAL_POOL_SLOTS.find((slot) => slot.token === other) || null;
}

function inferPoolForm(address, storage = {}, saved = {}) {
  const tokenA = saved.tokenA || findStorageValue(storage, "token0", findStorageValue(storage, "tokenA", "lqd")) || "lqd";
  const tokenB = saved.tokenB || findStorageValue(storage, "token1", findStorageValue(storage, "tokenB", "")) || "";
  return {
    pool: address,
    tokenA,
    tokenB,
    decimalsA: parseInt(saved.decimalsA || findStorageValue(storage, "decimals0", findStorageValue(storage, "decimalsA", "8")), 10) || 8,
    decimalsB: parseInt(saved.decimalsB || findStorageValue(storage, "decimals1", findStorageValue(storage, "decimalsB", "8")), 10) || 8,
  };
}

function resolveDexStorage(storage, form, ownerAddress) {
  const normA = String(form?.tokenA || "lqd").toLowerCase();
  const token0 = String(findStorageValue(storage, "token0", findStorageValue(storage, "tokenA", "")) || "").toLowerCase();
  const reserve0 = findStorageValue(storage, "reserve0", findStorageValue(storage, "reserveA", "0"));
  const reserve1 = findStorageValue(storage, "reserve1", findStorageValue(storage, "reserveB", "0"));
  const owner = String(ownerAddress || "").toLowerCase();
  return {
    reserveA: token0 && normA !== token0 ? reserve1 : reserve0,
    reserveB: token0 && normA !== token0 ? reserve0 : reserve1,
    totalLP: findStorageValue(storage, "totalLP", "0"),
    lpBalance: owner ? findStorageValue(storage, `lp:${owner}`, "0") : "0",
    lockedLP: owner ? findStorageValue(storage, `vlp:${owner}`, "0") : "0",
    lockUntil: owner ? findStorageValue(storage, `vlu:${owner}`, "0") : "0",
  };
}

function sortedPairKey(tokenA, tokenB) {
  const a = String(tokenA || "").trim().toLowerCase();
  const b = String(tokenB || "").trim().toLowerCase();
  return a < b ? `${a}:${b}` : `${b}:${a}`;
}

function getPairAddressForLPAction(form, storage = {}) {
  const key = sortedPairKey(form?.tokenA, form?.tokenB);
  return findStorageValue(storage, `pairAddr:${key}`, "") || form?.pool || "";
}

function isFactoryPoolForm(form, storage = {}) {
  const key = sortedPairKey(form?.tokenA, form?.tokenB);
  return !!findStorageValue(storage, `pairAddr:${key}`, "");
}

function formatPercentFromParts(numerator, denominator, decimals = 2) {
  const n = safeBig(numerator);
  const d = safeBig(denominator);
  if (d <= 0n) return "0.00%";
  const scaleValue = 10n ** BigInt(decimals);
  const value = (n * 100n * scaleValue) / d;
  const whole = value / scaleValue;
  const frac = (value % scaleValue).toString().padStart(decimals, "0");
  return `${whole}.${frac}%`;
}

function formatUnlockTime(raw) {
  const ts = Number(raw || 0);
  if (!Number.isFinite(ts) || ts <= 0) return "Not locked";
  const remainingMs = (ts * 1000) - Date.now();
  const date = new Date(ts * 1000).toLocaleString();
  if (remainingMs <= 0) return `Ready · ${date}`;
  return `${Math.ceil(remainingMs / 86400000)}d · ${date}`;
}

function formatLockDays(raw) {
  const ts = Number(raw || 0);
  if (!Number.isFinite(ts) || ts <= 0) return "365 default";
  const remainingMs = (ts * 1000) - Date.now();
  if (remainingMs <= 0) return "Ready";
  return `${Math.ceil(remainingMs / 86400000)} days left`;
}

function extractTxHash(res) {
  return res?.tx_hash || res?.TxHash || res?.hash || res?.transaction_hash || res?.transactionHash ||
    res?.result?.tx_hash || res?.result?.TxHash || res?.result?.hash || res?.result?.transaction_hash || res?.result?.transactionHash || "";
}

const TokenRow = ({ item, onSend, onRefresh, onRemove }) => (
  <View style={styles.rowCard}>
    <View style={styles.rowIcon}>
      <Text style={styles.rowIconText}>{String(item.symbol || "?").substring(0, 1).toUpperCase()}</Text>
    </View>
    <View style={{ flex: 1 }}>
      <Text style={styles.rowTitle}>{item.name || "Token"}</Text>
      <Text style={styles.rowSub}>{item.symbol} · {shortAddress(item.address)}</Text>
      <Text style={styles.rowValue}>{item.balance} {item.symbol}</Text>
    </View>
    <View style={styles.rowActions}>
      <Button label="Send" onPress={onSend} compact />
      <Button label="↻" onPress={onRefresh} compact secondary />
      <Button label="✕" onPress={onRemove} compact danger />
    </View>
  </View>
);

const BridgeRow = ({ item }) => {
  const direction = String(item.direction || item.Direction || item.type || item.Type || "").toLowerCase();
  const isInbound = direction.includes("external") || direction.includes("bsc_to_lqd") || direction.includes("lock");
  const s = String(item.status || "pending").toLowerCase();
  let statusColor = "#fbbf24"; // yellow
  if (s.includes("complete") || s.includes("success") || s.includes("confirmed")) statusColor = "#4ade80";
  if (s.includes("fail") || s.includes("error") || s.includes("reject")) statusColor = "#f87171";

  return (
    <View style={[styles.rowCard, { borderLeftWidth: 4, borderLeftColor: statusColor }]}>
      <View style={[styles.rowIcon, { backgroundColor: statusColor + "20" }]}>
        <Text style={{ color: statusColor, fontSize: scale(16) }}>{isInbound ? "📥" : "📤"}</Text>
      </View>
      <View style={{ flex: 1 }}>
        <View style={{ flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center' }}>
          <Text style={styles.rowTitle}>{isInbound ? "External → LQD" : "LQD → External"}</Text>
          <View style={{ backgroundColor: statusColor + "15", paddingHorizontal: 6, paddingVertical: 2, borderRadius: 4 }}>
            <Text style={{ color: statusColor, fontSize: scale(10), fontWeight: "800" }}>{s.toUpperCase()}</Text>
          </View>
        </View>
        <Text style={styles.rowSub}>
          <Text style={{ fontWeight: "700", color: "#f4f7ff" }}>{item.amount || "0"} {item.token || "LQD"}</Text>
          {" · "}{item.mode || "public"}{" · "}{(item.family || "evm").toUpperCase()}
        </Text>
        <Text style={styles.rowSub} numberOfLines={1}>
          {item.source_tx_hash ? `Source: ${shortAddress(item.source_tx_hash)}` : ""}
          {item.tx_hash && item.tx_hash !== item.source_tx_hash ? ` · LQD: ${shortAddress(item.tx_hash)}` : ""}
        </Text>
      </View>
    </View>
  );
};




const MMActionBtn = ({ label, icon, onPress, disabled }) => (
  <TouchableOpacity
    onPress={onPress}
    disabled={disabled}
    style={[styles.mmActionBtn, disabled && { opacity: 0.5 }]}
  >
    <View style={styles.mmActionIconWrap}>
      <Text style={styles.mmActionIconText}>{icon}</Text>
    </View>
    <Text style={styles.mmActionLabel}>{label}</Text>
  </TouchableOpacity>
);

function ActivityRow({ item, onPress }) {
  const hash = item.TxHash || item.tx_hash || item.hash || "";
  const type = item.Type || item.type || "tx";
  const typeLabels = {
    lp_lock: "Stake / Lock LP",
    lp_unlock: "Unstake / Unlock LP",
    lp_claim: "Claim LP Rewards",
    validator_register: "Validator Register",
  };
  const rawStatus = item.Status || item.status || "success";
  const s = String(rawStatus).toLowerCase();

  let statusColor = "#fbbf24"; // yellow/pending
  if (s.includes("complete") || s.includes("success") || s.includes("confirmed")) statusColor = "#4ade80";
  if (s.includes("fail") || s.includes("error") || s.includes("reject")) statusColor = "#f87171";

  const from = item.From || item.from || "";
  const to = item.To || item.to || "";
  const val = item.Value || item.amount || "";
  const sym = item.Symbol || (type === "send" ? "LQD" : "");

  return (
    <TouchableOpacity onPress={() => onPress && onPress(item)} style={[styles.rowCard, { borderLeftWidth: 4, borderLeftColor: statusColor }]}>
      <View style={{ flex: 1 }}>
        <View style={{ flexDirection: 'row', alignItems: 'center', marginBottom: scale(4) }}>
          <View style={{ backgroundColor: statusColor + "20", paddingHorizontal: 6, paddingVertical: 2, borderRadius: 4, marginRight: scale(8) }}>
            <Text style={{ color: statusColor, fontSize: scale(10), fontWeight: "800" }}>{s.toUpperCase()}</Text>
          </View>
          <Text style={styles.rowTitle}>{typeLabels[type] || type.toUpperCase()}</Text>
        </View>
        <Text style={styles.rowSub}>{hash ? `Hash: ${shortAddress(hash, 8, 6)}` : "Submitted to node"}</Text>
        <Text style={styles.rowSub}>
          {shortAddress(from)} → {shortAddress(to)}
        </Text>
        {!!val && <Text style={{ color: '#fff', fontSize: scale(13), fontWeight: 'bold', marginTop: scale(4) }}>{val} {sym}</Text>}
      </View>
      <View style={styles.rowActions}>
        <Text style={{ color: '#717da4', fontSize: scale(10) }}>{formatDate(item.Timestamp || item.timestamp)}</Text>
        <Text style={{ color: '#8a78ff', fontSize: scale(11), fontWeight: 'bold', marginTop: scale(4) }}>Details ›</Text>
      </View>
    </TouchableOpacity>
  );
}

const initialCreateForm = {
  password: "",
  confirm: "",
};

const initialImportMnemonicForm = {
  mnemonic: "",
  password: "",
};

const initialImportPkForm = {
  privateKey: "",
  password: "",
};

const initialSendForm = {
  to: "",
  amount: "",
};

const initialTokenImportForm = {
  address: "",
  symbol: "",
  name: "",
  decimals: "",
};

const initialNftImportForm = {
  address: "",
  tokenId: "",
  name: "",
  image: "",
};

const FAMILY_TOKEN_UI = {
  evm: { label: "Contract Address", placeholder: "0x...", hint: "ERC-20 token contract address", autoFetch: true, supported: true },
  solana: { label: "Mint Address", placeholder: "EPjFWdd5…", hint: "SPL token mint address (base58)", autoFetch: false, supported: true },
  cosmos: { label: "IBC Denom / CW20", placeholder: "ibc/... or cosmos1...", hint: "IBC denom or CosmWasm CW20 contract", autoFetch: false, supported: true },
  "cosmos-testnet": { label: "IBC Denom / CW20", placeholder: "ibc/... or cosmos1...", hint: "IBC denom or CosmWasm CW20 contract", autoFetch: false, supported: true },
  sei: { label: "Contract / Denom", placeholder: "sei1... or factory/...", hint: "SEI CW20 or native denom", autoFetch: false, supported: true },
  injective: { label: "Contract / Denom", placeholder: "inj1... or factory/...", hint: "INJ CW20 or native denom", autoFetch: false, supported: true },
  near: { label: "Contract Account", placeholder: "token.near", hint: "NEP-141 fungible token contract ID", autoFetch: false, supported: true },
  tron: { label: "TRC-20 Contract", placeholder: "T...", hint: "TRC-20 token contract address", autoFetch: false, supported: true },
  ton: { label: "Jetton Address", placeholder: "EQ...", hint: "TON Jetton master contract address", autoFetch: false, supported: true },
  aptos: { label: "Coin Type", placeholder: "0x1::module::Coin", hint: "Aptos coin type identifier", autoFetch: false, supported: true },
  sui: { label: "Coin Type", placeholder: "0x2::sui::SUI", hint: "SUI coin type address", autoFetch: false, supported: true },
  starknet: { label: "Contract Address", placeholder: "0x0...", hint: "Starknet ERC-20 contract", autoFetch: false, supported: true },
  harmony: { label: "Contract Address", placeholder: "one1... or 0x...", hint: "HRC-20 token contract address", autoFetch: false, supported: true },
  utxo: { supported: false, unsupportedMsg: "Bitcoin does not support token contracts. UTXO chains use only the native coin." },
  litecoin: { supported: false, unsupportedMsg: "Litecoin does not support token contracts. UTXO chains use only the native coin." },
};
const DEFAULT_FAMILY_UI = { label: "Token Address", placeholder: "...", hint: "Enter token identifier for this network", autoFetch: false, supported: true };

function getDefaultDecimalsForFamily(family) {
  const map = { solana: 9, cosmos: 6, "cosmos-testnet": 6, sei: 6, injective: 18, near: 24, ton: 9, tron: 6, aptos: 8, sui: 9, starknet: 18, harmony: 18 };
  return map[String(family || "").toLowerCase()] ?? 18;
}

const SEND_ADDR_PLACEHOLDER = {
  evm: "0x...",
  harmony: "one1... or 0x...",
  solana: "e.g. EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
  cosmos: "cosmos1...",
  "cosmos-testnet": "cosmos1...",
  sei: "sei1...",
  injective: "inj1...",
  near: "account.testnet",
  tron: "T...",
  ton: "EQ...",
  aptos: "0x...",
  sui: "0x...",
  starknet: "0x0...",
  utxo: "tb1q...",
  litecoin: "tltc1q...",
};

const initialTokenSendForm = {
  to: "",
  amount: "",
};

const initialFeeInfo = {
  display: "Estimating...",
  raw: "0",
  gasLimit: "",
  gasPrice: "",
  loading: false,
  error: "",
};

const initialDeployForm = {
  template: "dex_factory",
  gas: "500000",
  gasPrice: "",
  tokenName: "My Token",
  tokenSymbol: "MTK",
  tokenSupply: "1000000000000000",
  tokenA: "",
  tokenB: "",
  daoName: "DAO Treasury",
  nftName: "NFT Collection",
  nftSymbol: "NFT",
  bridgeName: "Bridged Token",
  bridgeSymbol: "BRG",
  bridgeDecimals: "18",
  bridgeSourceToken: "",
  lendingToken: "LQD",
};

const initialCallForm = {
  contract: "",
  functionName: "Ping",
  args: "",
  value: "0",
  gas: "200000",
  gasPrice: "",
};

const initialBridgeForm = {
  chainId: "bsc-testnet",
  toBsc: "",
  toLqd: "",
  token: "",
  amount: "",
  sourceTxHash: "",
  sourceAddress: "",
  sourceMemo: "",
  sourceSequence: "",
  sourceOutput: "",
};

const initialBridgeChainForm = {
  id: "",
  name: "",
  chainId: "",
  family: "evm",
  adapter: "evm",
  rpc: "",
  bridgeAddress: "",
  lockAddress: "",
  explorerUrl: "",
  nativeSymbol: "BNB",
  enabled: true,
  supportsPublic: true,
  supportsPrivate: true,
};

const initialBridgeTokenAdminForm = {
  chainId: "bsc-testnet",
  family: "evm",
  sourceToken: "",
  lqdToken: "",
  name: "",
  symbol: "",
  decimals: "8",
};

const initialNetworkForm = {
  name: "",
  chainId: "",
  nodeUrl: "",
  walletUrl: "",
  explorerUrl: "",
  symbol: "LQD",
};

const NETWORKS = [
  { id: 'lqd', name: 'LQD Testnet', symbol: 'LQD', family: 'evm', decimals: 8, chainId: 139, nodeUrl: DEFAULT_ENDPOINTS.nodeUrl, walletUrl: DEFAULT_ENDPOINTS.walletUrl, explorer: DEFAULT_ENDPOINTS.explorerUrl, icon: '💎', color: '#8a78ff' },
  { id: 'eth-sepolia', name: 'Ethereum Sepolia', symbol: 'ETH', family: 'evm', decimals: 18, chainId: 11155111, nodeUrl: 'https://ethereum-sepolia-rpc.publicnode.com', rpcUrls: ['https://rpc.sepolia.org'], walletUrl: DEFAULT_ENDPOINTS.walletUrl, explorer: 'https://sepolia.etherscan.io', icon: '🔷', color: '#627EEA' },
  { id: 'bsc-testnet', name: 'BSC Testnet', symbol: 'tBNB', family: 'evm', decimals: 18, chainId: 97, nodeUrl: 'https://bsc-testnet-rpc.publicnode.com', rpcUrls: ['https://data-seed-prebsc-1-s1.binance.org:8545'], walletUrl: DEFAULT_ENDPOINTS.walletUrl, explorer: 'https://testnet.bscscan.com', icon: '🟡', color: '#F3BA2F' },
  { id: 'solana-devnet', name: 'Solana Devnet', symbol: 'SOL', family: 'solana', decimals: 9, nodeUrl: 'https://api.devnet.solana.com', walletUrl: DEFAULT_ENDPOINTS.walletUrl, explorer: 'https://explorer.solana.com/?cluster=devnet', icon: '☀️', color: '#14F195' },
  { id: 'polygon-amoy', name: 'Polygon Amoy', symbol: 'POL', family: 'evm', decimals: 18, chainId: 80002, nodeUrl: 'https://polygon-amoy-bor-rpc.publicnode.com', rpcUrls: ['https://rpc-amoy.polygon.technology'], walletUrl: DEFAULT_ENDPOINTS.walletUrl, explorer: 'https://amoy.polygonscan.com', icon: '💜', color: '#8247E5' },
  { id: 'arb-sepolia', name: 'Arbitrum Sepolia', symbol: 'ETH', family: 'evm', decimals: 18, chainId: 421614, nodeUrl: 'https://arbitrum-sepolia-rpc.publicnode.com', rpcUrls: ['https://sepolia-rollup.arbitrum.io/rpc'], walletUrl: DEFAULT_ENDPOINTS.walletUrl, explorer: 'https://sepolia.arbiscan.io', icon: '🔵', color: '#28A0F0' },
  { id: 'op-sepolia', name: 'Optimism Sepolia', symbol: 'ETH', family: 'evm', decimals: 18, chainId: 11155420, nodeUrl: 'https://optimism-sepolia.drpc.org', rpcUrls: ['https://sepolia.optimism.io'], walletUrl: DEFAULT_ENDPOINTS.walletUrl, explorer: 'https://sepolia-optimistic.etherscan.io', icon: '🔴', color: '#FF0420' },
  { id: 'base-sepolia', name: 'Base Sepolia', symbol: 'ETH', family: 'evm', decimals: 18, chainId: 84532, nodeUrl: 'https://base-sepolia-rpc.publicnode.com', rpcUrls: ['https://sepolia.base.org'], walletUrl: DEFAULT_ENDPOINTS.walletUrl, explorer: 'https://sepolia.basescan.org', icon: '🔵', color: '#0052FF' },
  { id: 'avax-fuji', name: 'Avalanche Fuji', symbol: 'AVAX', family: 'evm', decimals: 18, chainId: 43113, nodeUrl: 'https://avalanche-fuji-c-chain-rpc.publicnode.com', rpcUrls: ['https://api.avax-test.network/ext/bc/C/rpc'], walletUrl: DEFAULT_ENDPOINTS.walletUrl, explorer: 'https://testnet.snowtrace.io', icon: '🔺', color: '#E84142' },
  { id: 'linea-sepolia', name: 'Linea Sepolia', symbol: 'ETH', family: 'evm', decimals: 18, chainId: 59141, nodeUrl: 'https://linea-sepolia-rpc.publicnode.com', rpcUrls: ['https://rpc.sepolia.linea.build'], walletUrl: DEFAULT_ENDPOINTS.walletUrl, explorer: 'https://sepolia.lineascan.build', icon: '🧬', color: '#121212' },
  { id: 'scroll-sepolia', name: 'Scroll Sepolia', symbol: 'ETH', family: 'evm', decimals: 18, chainId: 534351, nodeUrl: 'https://scroll-sepolia-rpc.publicnode.com', rpcUrls: ['https://sepolia-rpc.scroll.io'], walletUrl: DEFAULT_ENDPOINTS.walletUrl, explorer: 'https://sepolia.scrollscan.com', icon: '📜', color: '#FFDEC1' },
  { id: 'berachain-bepolia', name: 'Berachain Bepolia', symbol: 'BERA', family: 'evm', decimals: 18, chainId: 80069, nodeUrl: 'https://bepolia.rpc.berachain.com', walletUrl: DEFAULT_ENDPOINTS.walletUrl, explorer: 'https://bepolia.beratrail.io', icon: '🐻', color: '#FFB237' },
  { id: 'fantom-testnet', name: 'Fantom Testnet', symbol: 'FTM', family: 'evm', decimals: 18, chainId: 4002, nodeUrl: 'https://fantom-testnet.rpc.thirdweb.com', walletUrl: DEFAULT_ENDPOINTS.walletUrl, explorer: 'https://testnet.ftmscan.com', icon: '👻', color: '#1969FF' },
  { id: 'bitcoin-testnet', name: 'Bitcoin Testnet', symbol: 'tBTC', family: 'utxo', decimals: 8, nodeUrl: 'https://blockstream.info/testnet/api', walletUrl: DEFAULT_ENDPOINTS.walletUrl, explorer: 'https://blockstream.info/testnet', icon: '₿', color: '#F7931A' },
  { id: 'litecoin-testnet', name: 'Litecoin Testnet', symbol: 'tLTC', family: 'litecoin', decimals: 8, nodeUrl: 'https://api.blockcypher.com/v1/ltc/test3', walletUrl: DEFAULT_ENDPOINTS.walletUrl, explorer: 'https://live.blockcypher.com/ltc-testnet', icon: 'Ł', color: '#345D9D' },
  { id: 'cosmos-testnet', name: 'Cosmos Hub Testnet', symbol: 'ATOM', family: 'cosmos', decimals: 6, cosmosChainId: 'theta-testnet-001', nodeUrl: 'https://cosmos-testnet-api.polkachu.com', cosmosRestUrls: ['https://rest.sentry-01.theta-testnet.polypore.xyz'], walletUrl: DEFAULT_ENDPOINTS.walletUrl, explorer: 'https://explorer.theta-testnet.polypore.xyz', icon: '⚛️', color: '#2E3148' },
  { id: 'blast-sepolia', name: 'Blast Sepolia', symbol: 'ETH', family: 'evm', decimals: 18, chainId: 168587773, nodeUrl: 'https://sepolia.blast.io', walletUrl: DEFAULT_ENDPOINTS.walletUrl, explorer: 'https://testnet.blastscan.io', icon: '💥', color: '#FCFC03' },
  { id: 'zksync-sepolia', name: 'zkSync Sepolia', symbol: 'ETH', family: 'evm', decimals: 18, chainId: 300, nodeUrl: 'https://sepolia.era.zksync.dev', walletUrl: DEFAULT_ENDPOINTS.walletUrl, explorer: 'https://sepolia.explorer.zksync.io', icon: '🔗', color: '#3333FF' },
  { id: 'monad-testnet', name: 'Monad Testnet', symbol: 'MON', family: 'evm', decimals: 18, chainId: 10143, nodeUrl: 'https://testnet-rpc.monad.xyz', rpcUrls: ['https://rpc-testnet.monadinfra.com'], walletUrl: DEFAULT_ENDPOINTS.walletUrl, explorer: 'https://explorer.monad.xyz', icon: '🟣', color: '#836EF9' },
  { id: 'near-testnet', name: 'NEAR Testnet', symbol: 'NEAR', family: 'near', decimals: 24, nodeUrl: 'https://rpc.testnet.near.org', walletUrl: DEFAULT_ENDPOINTS.walletUrl, explorer: 'https://explorer.testnet.near.org', icon: 'Ⓝ', color: '#000000' },
  { id: 'aptos-testnet', name: 'Aptos Testnet', symbol: 'APT', family: 'aptos', decimals: 8, nodeUrl: 'https://fullnode.testnet.aptoslabs.com', walletUrl: DEFAULT_ENDPOINTS.walletUrl, explorer: 'https://explorer.aptoslabs.com/?network=testnet', icon: '🌀', color: '#EDF2F7' },
  { id: 'tron-shasta', name: 'Tron Shasta', symbol: 'TRX', family: 'tron', decimals: 6, chainId: 2494104990, nodeUrl: 'https://api.shasta.trongrid.io/jsonrpc', walletUrl: DEFAULT_ENDPOINTS.walletUrl, explorer: 'https://shasta.tronscan.org', icon: '🔴', color: '#FF0013' },
  { id: 'celo-sepolia', name: 'Celo Sepolia', symbol: 'CELO', family: 'evm', decimals: 18, chainId: 11142220, nodeUrl: 'https://forno.celo-sepolia.celo-testnet.org', walletUrl: DEFAULT_ENDPOINTS.walletUrl, explorer: 'https://celo-sepolia.blockscout.com', icon: '🌿', color: '#35D07F' },
  { id: 'sui-testnet', name: 'Sui Testnet', symbol: 'SUI', family: 'sui', decimals: 9, nodeUrl: 'https://fullnode.testnet.sui.io', walletUrl: DEFAULT_ENDPOINTS.walletUrl, explorer: 'https://explorer.sui.io/?network=testnet', icon: '💧', color: '#6FBCF0' },
  { id: 'mantle-sepolia', name: 'Mantle Sepolia', symbol: 'MNT', family: 'evm', decimals: 18, chainId: 5003, nodeUrl: 'https://rpc.sepolia.mantle.xyz', walletUrl: DEFAULT_ENDPOINTS.walletUrl, explorer: 'https://explorer.sepolia.mantle.xyz', icon: '🧤', color: '#000000' },
  { id: 'cronos-testnet', name: 'Cronos Testnet', symbol: 'TCRO', family: 'evm', decimals: 18, chainId: 338, nodeUrl: 'https://cronos-testnet.drpc.org', walletUrl: DEFAULT_ENDPOINTS.walletUrl, explorer: 'https://cronos.org/explorer/testnet3', icon: '🔵', color: '#002D74' },
  { id: 'metis-sepolia', name: 'Metis Sepolia', symbol: 'METIS', family: 'evm', decimals: 18, chainId: 59902, nodeUrl: 'https://sepolia.metisdevops.link', rpcUrls: ['https://59902.rpc.thirdweb.com'], walletUrl: DEFAULT_ENDPOINTS.walletUrl, explorer: 'https://sepolia.explorer.metis.io', icon: '🌿', color: '#00D2FF' },
  { id: 'moonriver-test', name: 'Moonbase Alpha', symbol: 'DEV', family: 'evm', decimals: 18, chainId: 1287, nodeUrl: 'https://rpc.api.moonbase.moonbeam.network', walletUrl: DEFAULT_ENDPOINTS.walletUrl, explorer: 'https://moonbase.moonscan.io', icon: '🌙', color: '#53CBC9' },
  { id: 'harmony-test', name: 'Harmony Testnet', symbol: 'ONE', family: 'harmony', decimals: 18, chainId: 1666700000, nodeUrl: 'https://api.s0.b.hmny.io', walletUrl: DEFAULT_ENDPOINTS.walletUrl, explorer: 'https://explorer.testnet.harmony.one', icon: '💠', color: '#00AEEF' },
  { id: 'ton-testnet', name: 'TON Testnet', symbol: 'TON', family: 'ton', decimals: 9, nodeUrl: 'https://testnet.toncenter.com/api/v2/jsonRPC', walletUrl: DEFAULT_ENDPOINTS.walletUrl, explorer: 'https://testnet.tonscan.org', icon: '💎', color: '#0088CC' },
  { id: 'sei-testnet', name: 'Sei Atlantic', symbol: 'SEI', family: 'sei', decimals: 6, chainId: 1328, cosmosChainId: 'atlantic-2', nodeUrl: 'https://evm-rpc-testnet.sei-apis.com', rpcUrls: ['https://evm-rpc.atlantic-2.seinetwork.io'], cosmosRestUrl: 'https://rest-testnet.sei-apis.com', cosmosRestUrls: ['https://rest.atlantic-2.seinetwork.io'], walletUrl: DEFAULT_ENDPOINTS.walletUrl, explorer: 'https://atlantic-2.seiscan.app', icon: '🔴', color: '#FF0000' },
  { id: 'hyperliquid-test', name: 'Hyperliquid Test', symbol: 'HYPE', family: 'evm', decimals: 18, chainId: 998, nodeUrl: 'https://rpc.hyperliquid-testnet.xyz/evm', walletUrl: DEFAULT_ENDPOINTS.walletUrl, explorer: 'https://app.hyperliquid.xyz/explorer', icon: '📈', color: '#27C19F' },
  { id: 'story-testnet', name: 'Story Aeneid', symbol: 'IP', family: 'evm', decimals: 18, chainId: 1315, nodeUrl: 'https://aeneid.storyrpc.io', walletUrl: DEFAULT_ENDPOINTS.walletUrl, explorer: 'https://aeneid.storyscan.xyz', icon: '📖', color: '#000000' },
  { id: 'injective-testnet', name: 'Injective Testnet', symbol: 'INJ', family: 'injective', decimals: 18, cosmosChainId: 'injective-888', nodeUrl: 'https://testnet.sentry.lcd.injective.network', walletUrl: DEFAULT_ENDPOINTS.walletUrl, explorer: 'https://testnet.explorer.injective.network', icon: '🌀', color: '#00A3FF' },
  { id: 'sonic-testnet', name: 'Sonic Testnet', symbol: 'S', family: 'evm', decimals: 18, chainId: 14601, nodeUrl: 'https://rpc.testnet.soniclabs.com', walletUrl: DEFAULT_ENDPOINTS.walletUrl, explorer: 'https://testnet.sonicscan.org', icon: '⚡', color: '#FFFFFF' },
  { id: 'starknet-sepolia', name: 'Starknet Sepolia', symbol: 'ETH', family: 'starknet', decimals: 18, nodeUrl: 'https://starknet-sepolia.drpc.org', walletUrl: DEFAULT_ENDPOINTS.walletUrl, explorer: 'https://sepolia.starkscan.co', icon: '✨', color: '#0C0C4F' }
];

const initialEndpointsForm = {
  nodeUrl: DEFAULT_ENDPOINTS.nodeUrl,
  walletUrl: DEFAULT_ENDPOINTS.walletUrl,
  aggregatorUrl: DEFAULT_ENDPOINTS.aggregatorUrl,
  explorerUrl: DEFAULT_ENDPOINTS.explorerUrl,
};

function App() {
  const [booting, setBooting] = useState(true);
  const [status, setStatus] = useState("");
  const [busy, setBusy] = useState(false);
  const [busyAction, setBusyAction] = useState("");
  const [tab, setTab] = useState("home");
  const [homeSubTab, setHomeSubTab] = useState("tokens");
  const [networkModalVisible, setNetworkModalVisible] = useState(false);
  const [sendVisible, setSendVisible] = useState(false);
  const [tokenImportVisible, setTokenImportVisible] = useState(false);
  const [tokenPreviewMeta, setTokenPreviewMeta] = useState(null);
  const [tokenPreviewLoading, setTokenPreviewLoading] = useState(false);
  const [nftImportVisible, setNftImportVisible] = useState(false);
  const [faucetVisible, setFaucetVisible] = useState(false);
  const [isMainMenuVisible, setIsMainMenuVisible] = useState(false);

  const [vaultRecord, setVaultRecord] = useState(null);
  const [wallet, setWallet] = useState(null);
  const [multiWallets, setMultiWallets] = useState({}); // Stores addresses for different families { evm: { address, pk }, solana: { address, pk } }
  const [chainKeys, setChainKeys] = useState(null); // { evm, harmony, tron, cosmos, sei, injective, solana, near, aptos, sui, ton }
  const [keyExportVisible, setKeyExportVisible] = useState(false);
  const [unlockPassword, setUnlockPassword] = useState("");
  const [createForm, setCreateForm] = useState(initialCreateForm);
  const [importMnemonicForm, setImportMnemonicForm] = useState(initialImportMnemonicForm);
  const [importPkForm, setImportPkForm] = useState(initialImportPkForm);
  const [showMnemonic, setShowMnemonic] = useState(false);

  const [networks, setNetworks] = useState(DEFAULT_NETWORKS);
  const [activeNetworkId, setActiveNetworkId] = useState(DEFAULT_NETWORKS[0].id);
  const [endpoints, setEndpoints] = useState(initialEndpointsForm);
  const [networkForm, setNetworkForm] = useState(initialNetworkForm);

  const [nativeBalance, setNativeBalance] = useState("0");
  const [factoryAddress, setFactoryAddress] = useState("");
  const [liquidityDashboard, setLiquidityDashboard] = useState({
    protocol: null,
    providers: [],
    latestRewards: null,
    recentRewards: [],
    poolCards: [],
  });
  const [liquidityLoading, setLiquidityLoading] = useState(false);
  const [liquidityAction, setLiquidityAction] = useState("");
  const [liquidityPoolInputs, setLiquidityPoolInputs] = useState({});
  const [liquidityPoolResults, setLiquidityPoolResults] = useState({});
  const [recentTxs, setRecentTxs] = useState([]);
  const [activity, setActivity] = useState([]);
  const [watchlist, setWatchlist] = useState([]);
  const [bridgeRequests, setBridgeRequests] = useState([]);
  const [bridgeTokens, setBridgeTokens] = useState([]);
  const [bridgeFamilies, setBridgeFamilies] = useState([]);
  const [bridgeChains, setBridgeChains] = useState([]);
  const [bridgeChainId, setBridgeChainId] = useState("bsc-testnet");
  const [bridgeAdminApiKey, setBridgeAdminApiKey] = useState("");
  const [bridgeChainForm, setBridgeChainForm] = useState(initialBridgeChainForm);
  const [bridgeTokenAdminApiKey, setBridgeTokenAdminApiKey] = useState("");
  const [bridgeTokenAdminForm, setBridgeTokenAdminForm] = useState(initialBridgeTokenAdminForm);
  const [pendingApprovals, setPendingApprovals] = useState([]);
  const [trustedOrigins, setTrustedOrigins] = useState([]);

  const [sendForm, setSendForm] = useState(initialSendForm);
  const [tokenImportForm, setTokenImportForm] = useState(initialTokenImportForm);
  const [nftImportForm, setNftImportForm] = useState(initialNftImportForm);
  const [nfts, setNfts] = useState([]);
  const [selectedTokenForSend, setSelectedTokenForSend] = useState(null);
  const [tokenSendForm, setTokenSendForm] = useState(initialTokenSendForm);
  const [hiddenTokens, setHiddenTokens] = useState([]); // [{address, networkId}]
  const [removedTokens, setRemovedTokens] = useState([]); // [{address, networkId}] — persisted blacklist
  const [tokenActionTarget, setTokenActionTarget] = useState(null); // token being actioned
  const [showHiddenTokens, setShowHiddenTokens] = useState(false);

  const [deployForm, setDeployForm] = useState(initialDeployForm);
  const [customSource, setCustomSource] = useState(DEFAULT_CUSTOM_SOURCE);
  const [compiledPlugin, setCompiledPlugin] = useState(null);
  const [compiledPluginUri, setCompiledPluginUri] = useState("");
  const [compiledPluginSize, setCompiledPluginSize] = useState(0);
  const [inspectForm, setInspectForm] = useState({ address: "" });
  const [inspectData, setInspectData] = useState({ abi: null, storage: null });
  const [callForm, setCallForm] = useState(initialCallForm);

  const [bridgeForm, setBridgeForm] = useState(initialBridgeForm);
  const [bridgeMode, setBridgeMode] = useState("public");

  const [callAbi, setCallAbi] = useState([]);
  const [callSelectedFnIdx, setCallSelectedFnIdx] = useState(null);
  const [callArgs, setCallArgs] = useState({});
  const [explorerTab, setExplorerTab] = useState("overview");
  const [explorerEvents, setExplorerEvents] = useState([]);
  const [compileType, setCompileType] = useState("goplugin");
  const [compiledBinary, setCompiledBinary] = useState(null);
  const [bridgeBaseFee, setBridgeBaseFee] = useState(10);
  const [isNodeOnline, setIsNodeOnline] = useState(true);
  const [networkHealth, setNetworkHealth] = useState({ state: "checking", message: "Checking node", checkedAt: 0 });
  const [balanceLoadError, setBalanceLoadError] = useState("");
  const [lastWalletRefresh, setLastWalletRefresh] = useState(0);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const refreshInFlightRef = useRef(null);
  const lastRefreshRequestRef = useRef({ key: "", at: 0 });
  const [processingMessage, setProcessingMessage] = useState("");
  const [toast, setToast] = useState({ visible: false, message: "", type: "info" });

  // ── Premium spinner animation ────────────────────────────────────────────────
  const _spinAnim = useRef(new Animated.Value(0)).current;
  const _spinAnim2 = useRef(new Animated.Value(0)).current;
  useEffect(() => {
    if (!processingMessage) { _spinAnim.setValue(0); _spinAnim2.setValue(0); return; }
    const loop1 = Animated.loop(Animated.timing(_spinAnim,  { toValue: 1, duration: 900,  useNativeDriver: true }));
    const loop2 = Animated.loop(Animated.timing(_spinAnim2, { toValue: 1, duration: 1500, useNativeDriver: true }));
    loop1.start(); loop2.start();
    return () => { loop1.stop(); loop2.stop(); };
  }, [processingMessage]);
  const _rotateDeg  = _spinAnim.interpolate({ inputRange: [0, 1], outputRange: ['0deg', '360deg'] });
  const _rotateDeg2 = _spinAnim2.interpolate({ inputRange: [0, 1], outputRange: ['360deg', '0deg'] });
  const [bridgeDirection, setBridgeDirection] = useState("lqd_to_external"); // lqd_to_external or external_to_lqd
  const [bridgeSelectedToken, setBridgeSelectedToken] = useState(null);
  const [bridgeChainModalVisible, setBridgeChainModalVisible] = useState(false);
  const [bridgeTokenModalVisible, setBridgeTokenModalVisible] = useState(false);
  const [bridgeTargetSide, setBridgeTargetSide] = useState("target"); // "source" or "target"

  function showToast(message, type = "info") {
    setToast({ visible: true, message, type });
    setTimeout(() => setToast((p) => ({ ...p, visible: false })), 4000);
  }

  function addTrackedTx({ hash, type, symbol, to, network = currentNetwork, family = currentNetwork?.family }) {
    if (!settingsTxTrackingEnabled || !hash) return;
    const normalizedFamily = network?.id?.startsWith("lqd") || ["0x8b", "0x8c"].includes(String(network?.chainId || "").toLowerCase())
      ? "lqd"
      : (family || network?.family || "evm");
    const tx = createTrackedTransaction({
      hash,
      type,
      symbol,
      to,
      networkId: network?.id || activeNetworkId,
      family: normalizedFamily,
    });
    if (!tx) return;
    setTrackedTxs((prev) => mergeTrackedTransaction(prev, { ...tx, nodeUrl: getRpcCandidatesForFamily(network, normalizedFamily)[0] || nodeUrl }));
  }

  function isAllowedBrowserOrigin(origin) {
    const value = String(origin || "").trim();
    if (!value || value === "null") return false;
    try {
      const parsed = new URL(value);
      if (parsed.protocol === "https:") return true;
      if (parsed.hostname === "127.0.0.1" || parsed.hostname === "localhost") return true;
      return false;
    } catch {
      return false;
    }
  }

  function isLqdNetworkUrl(value = nodeUrl) {
    const text = `${currentNetwork?.id || ""} ${currentNetwork?.name || ""} ${value || ""}`.toLowerCase();
    return text.includes("lqd") || text.includes("podl") || text.includes("railway") || text.includes("dazzling-peace") || text.includes("178.105.133.94") || text.includes("178-105-133-94.sslip.io");
  }

  function tokenKey(token, fallbackNetworkId = activeNetworkId) {
    return `${String(token?.networkId || fallbackNetworkId)}:${String(token?.address || token?.contract || "").toLowerCase()}`;
  }

  function isGenericTokenMeta(token) {
    const symbol = String(token?.symbol || "").trim().toUpperCase();
    const name = String(token?.name || "").trim().toUpperCase();
    return !symbol || symbol === "TOKEN" || symbol === "UNKNOWN" || name === "TOKEN" || name === "UNKNOWN TOKEN";
  }

  function tokenLogoUri(token = {}) {
    return token.logoUrl || token.logo_url || token.logo || "";
  }

  function tokenMarketNumber(value) {
    if (value === null || value === undefined || value === "") return null;
    const n = Number(String(value).replace("%", "").trim());
    return Number.isFinite(n) ? n : null;
  }

  function tokenPriceLabel(token = {}) {
    const price = tokenMarketNumber(token.priceUsd ?? token.price_usd ?? token.price ?? token.usd_price);
    if (price === null) return "Price unavailable";
    const digits = price >= 1 ? 2 : 6;
    return `$${price.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: digits })}`;
  }

  function tokenChangeLabel(token = {}) {
    const change = tokenMarketNumber(token.priceChange24h ?? token.price_change_24h ?? token.change24h ?? token.change_24h);
    if (change === null) return "24h --";
    const sign = change > 0 ? "+" : "";
    return `${sign}${change.toFixed(2)}%`;
  }

  function tokenChangeStyle(token = {}) {
    const change = tokenMarketNumber(token.priceChange24h ?? token.price_change_24h ?? token.change24h ?? token.change_24h);
    if (change === null) return styles.walletTokenChangeNeutral;
    if (change > 0) return styles.walletTokenChangePositive;
    if (change < 0) return styles.walletTokenChangeNegative;
    return styles.walletTokenChangeNeutral;
  }

  function isOfficialToken(token = {}) {
    return Boolean(token.official || token.registry || token.detectedFrom === "dex-registry");
  }

  function healthDotColor() {
    if (isRefreshing || networkHealth.state === "checking") return "#f59e0b";
    if (networkHealth.state === "online") return "#10b981";
    if (networkHealth.state === "slow") return "#f59e0b";
    return "#ef4444";
  }


  const [backupText, setBackupText] = useState("");
  const [settingsAutoRefresh, setSettingsAutoRefresh] = useState(true);
  const [settingsTelemetryEnabled, setSettingsTelemetryEnabled] = useState(false);
  const [settingsTxTrackingEnabled, setSettingsTxTrackingEnabled] = useState(true);
  const [settingsBackgroundTxTrackingEnabled, setSettingsBackgroundTxTrackingEnabled] = useState(true);
  const [settingsStorageAuditEnabled, setSettingsStorageAuditEnabled] = useState(true);
  const [securityAudit, setSecurityAudit] = useState(null);
  const [legalRiskAccepted, setLegalRiskAccepted] = useState(false);
  const [trackedTxs, setTrackedTxs] = useState([]);
  const [walletVisible, setWalletVisible] = useState(false);
  const [scannerVisible, setScannerVisible] = useState(false);
  const [scannerTarget, setScannerTarget] = useState("");
  const [scanChoiceVisible, setScanChoiceVisible] = useState(false);
  const [estimatedFee, setEstimatedFee] = useState("0.00001");
  const [nativeFeeInfo, setNativeFeeInfo] = useState(initialFeeInfo);
  const [tokenFeeInfo, setTokenFeeInfo] = useState(initialFeeInfo);
  const [receiveVisible, setReceiveVisible] = useState(false);
  const [watchAddresses, setWatchAddresses] = useState({}); // { networkId: externalAddress } for ed25519 chains
  const [watchAddrInput, setWatchAddrInput] = useState("");
  const [statusModal, setStatusModal] = useState({ visible: false, title: "", message: "", type: "success", hash: "", copyLabel: "" });
  const [selectedTxStory, setSelectedTxStory] = useState(null);
  const [activeBridgeTx, setActiveBridgeTx] = useState(null); // Tracking current bridge
  const [approvalModalVisible, setApprovalModalVisible] = useState(false);
  const [deepLinkHint, setDeepLinkHint] = useState("");
  const [cameraPermission, requestCameraPermission] = useCameraPermissions();
  const [biometricEnabled, setBiometricEnabled] = useState(true);
  const [biometricAvailable, setBiometricAvailable] = useState(false);
  const [browserInput, setBrowserInput] = useState("");
  const [browserUrl, setBrowserUrl] = useState("");
  const [browserHistory, setBrowserHistory] = useState([]);
  const [browserLoading, setBrowserLoading] = useState(false);
  const [browserCanGoBack, setBrowserCanGoBack] = useState(false);
  const [browserCanGoForward, setBrowserCanGoForward] = useState(false);
  const [browserVisible, setBrowserVisible] = useState(false);

  const currentNetwork = useMemo(() => {
    return NETWORKS.find(n => n.id === activeNetworkId) || NETWORKS[0];
  }, [activeNetworkId]);

  const nodeUrl = useMemo(() => {
    return normalizeUrl(currentNetwork?.nodeUrl || endpoints.nodeUrl);
  }, [currentNetwork, endpoints.nodeUrl]);

  const walletUrl = useMemo(() => {
    return normalizeUrl(currentNetwork?.walletUrl || endpoints.walletUrl);
  }, [currentNetwork, endpoints.walletUrl]);

  const aggregatorUrl = useMemo(() => normalizeUrl(endpoints.aggregatorUrl || DEFAULT_ENDPOINTS.aggregatorUrl), [endpoints.aggregatorUrl]);
  const explorerUrl = useMemo(() => normalizeUrl(endpoints.explorerUrl || DEFAULT_ENDPOINTS.explorerUrl), [endpoints.explorerUrl]);

  function dedupeUrls(values = []) {
    return [...new Set((values || []).map((value) => normalizeUrl(value)).filter(Boolean))];
  }

  function getNetworkNodeCandidates(network = currentNetwork) {
    const list = [network?.nodeUrl, ...(network?.rpcUrls || [])];
    if (String(network?.id || "") === "lqd" && endpoints.nodeUrl) {
      list.unshift(endpoints.nodeUrl);
    }
    return dedupeUrls(list);
  }

  function getNetworkRestCandidates(network = currentNetwork) {
    return dedupeUrls([network?.cosmosRestUrl, ...(network?.cosmosRestUrls || [])]);
  }

  function getRpcCandidatesForFamily(network = currentNetwork, familyOverride = "") {
    const family = String(familyOverride || network?.family || "evm").toLowerCase();
    if (family === "sei" || family === "injective" || family === "cosmos-testnet" || family === "cosmos") {
      const rest = getNetworkRestCandidates(network);
      return rest.length ? rest : getNetworkNodeCandidates(network);
    }
    return getNetworkNodeCandidates(network);
  }

  async function tryRpcCandidates(candidates, runner, accept = (result) => result !== null && result !== undefined) {
    let lastError = null;
    for (const candidate of dedupeUrls(candidates)) {
      try {
        const result = await runner(candidate);
        if (accept(result)) return { result, url: candidate };
      } catch (error) {
        lastError = error;
      }
    }
    if (lastError) throw lastError;
    return { result: null, url: "" };
  }

  function hexQuantity(value) {
    try {
      return "0x" + BigInt(value || 0).toString(16);
    } catch {
      return "0x0";
    }
  }

  function feeInfo(raw, decimals, symbol, extra = {}) {
    const value = String(raw || "0");
    return {
      ...initialFeeInfo,
      raw: value,
      display: `~${formatUnits(value, decimals || 18, 8)} ${symbol || currentNetwork.symbol}`,
      ...extra,
    };
  }

  function evmFeeSigner(family) {
    if (family === "harmony") return chainKeys?.harmony?.address || chainKeys?.evm?.address || wallet?.address || "";
    if (family === "tron") return chainKeys?.tron?.address || wallet?.address || "";
    if (family === "sei") return chainKeys?.sei?.evmAddress || chainKeys?.evm?.address || wallet?.address || "";
    return chainKeys?.evm?.address || wallet?.address || "";
  }

  function isZeroFeeNetwork(network, family) {
    return network?.id === "monad-testnet" || family === "sei";
  }

  async function estimateWalletFee({ token = null } = {}) {
    const network = token
      ? (NETWORKS.find((n) => n.id === (token.networkId || activeNetworkId)) || currentNetwork)
      : currentNetwork;
    const family = String(token?.family || network.family || "evm").toLowerCase();
    const symbol = network.symbol || currentNetwork.symbol;
    const decimals = network.decimals || currentNetwork.decimals || 18;
    const candidates = getRpcCandidatesForFamily(network, family);
    const primary = candidates[0] || nodeUrl;
    const amountText = token ? tokenSendForm.amount : sendForm.amount;
    const recipient = (token ? tokenSendForm.to : sendForm.to).trim();
    const amountRaw = amountText ? parseUnits(amountText, token?.decimals || decimals) : "0";

    if (network.id === "lqd") {
      const gasLimit = token ? 50000 : 21000;
      const base = await nodeBaseFee(primary).catch(() => 10);
      const gasPrice = String(base?.base_fee || base?.BaseFee || base?.baseFee || base || 10);
      return feeInfo(BigInt(gasPrice) * BigInt(gasLimit), 8, "LQD", {
        gasLimit: String(gasLimit),
        gasPrice,
      });
    }

    if (isZeroFeeNetwork(network, family)) {
      return feeInfo("0", decimals, symbol, {
        display: "No network fee",
        gasLimit: String(token ? 80000 : 21000),
        gasPrice: "0",
      });
    }

    if (family === "tron") {
      const tronBaseUrl = normalizeUrl(String(primary).replace(/\/jsonrpc\/?$/, ""));
      const params = await postJson(`${tronBaseUrl}/wallet/getchainparameters`, {}).catch(() => null);
      const chainParams = Array.isArray(params?.chainParameter) ? params.chainParameter : [];
      const getParam = (key, fallback) => {
        const item = chainParams.find((entry) => entry?.key === key);
        return Number(item?.value || fallback);
      };
      const transactionFeePerByte = getParam("getTransactionFee", 1000);
      const energyFee = getParam("getEnergyFee", 420);
      const rawFee = token
        ? Math.ceil(energyFee * 65000)
        : Math.ceil(transactionFeePerByte * 300);
      return feeInfo(String(rawFee), 6, "TRX", {
        gasLimit: token ? "65000" : "300",
        gasPrice: String(token ? energyFee : transactionFeePerByte),
      });
    }

    if (family === "evm" || family === "harmony" || family === "sei") {
      const fromAddr = evmFeeSigner(family);
      const safeTo = token
        ? token.address
        : (recipient && isLikelyAddressForFamily(recipient, family) ? recipient : fromAddr);
      const data = token && recipient && isLikelyAddressForFamily(recipient, family)
        ? encodeErc20Transfer(recipient, amountRaw)
        : "";
      const tx = {
        from: fromAddr,
        to: safeTo,
        value: token ? "0x0" : hexQuantity(amountRaw),
      };
      if (data) tx.data = data;
      const [gasPriceRes, gasRes] = await Promise.all([
        tryRpcCandidates(candidates, (url) => postJson(url, {
          jsonrpc: "2.0", id: 1, method: "eth_gasPrice", params: [],
        }).catch(() => null), (r) => !!r?.result).then(({ result }) => result).catch(() => null),
        fromAddr && safeTo ? tryRpcCandidates(candidates, (url) => postJson(url, {
          jsonrpc: "2.0", id: 2, method: "eth_estimateGas", params: [tx],
        }).catch(() => null), (r) => !!r?.result).then(({ result }) => result).catch(() => null) : Promise.resolve(null),
      ]);
      const gasPrice = gasPriceRes?.result ? BigInt(gasPriceRes.result).toString() : String(10_000_000_000n);
      const gasLimit = gasRes?.result ? BigInt(gasRes.result).toString() : String(token ? 80000 : 21000);
      return feeInfo(BigInt(gasPrice) * BigInt(gasLimit), decimals, symbol, { gasLimit, gasPrice });
    }

    if (family === "cosmos" || family === "cosmos-testnet" || family === "injective") {
      return feeInfo("5000", decimals, symbol, { gasLimit: "200000", gasPrice: "0.025" });
    }

    if (family === "solana") {
      const recentFees = await postJson(primary, {
        jsonrpc: "2.0", id: 1, method: "getRecentPrioritizationFees", params: [],
      }).catch(() => null);
      const priority = Array.isArray(recentFees?.result)
        ? Math.max(0, ...recentFees.result.slice(-20).map((x) => Number(x?.prioritizationFee || 0)))
        : 0;
      return feeInfo(5000n + BigInt(priority || 0), 9, "SOL", { gasLimit: "1", gasPrice: String(priority || 5000) });
    }

    if (family === "near") {
      const gasPriceRes = await postJson(primary, {
        jsonrpc: "2.0", id: 1, method: "gas_price", params: [null],
      }).catch(() => null);
      const gasPrice = BigInt(gasPriceRes?.result?.gas_price || "100000000");
      const gasLimit = token ? 30000000000000n : 1000000000000n;
      return feeInfo(gasPrice * gasLimit, 24, "NEAR", { gasLimit: gasLimit.toString(), gasPrice: gasPrice.toString() });
    }

    if (family === "aptos") {
      const gasRes = await getJson(`${normalizeUrl(primary)}/estimate_gas_price`).catch(() => null);
      const gasPrice = BigInt(gasRes?.gas_estimate || gasRes?.prioritized_gas_estimate || "100");
      const gasLimit = 2000n;
      return feeInfo(gasPrice * gasLimit, 8, "APT", { gasLimit: gasLimit.toString(), gasPrice: gasPrice.toString() });
    }

    if (family === "sui") {
      const gasRes = await postJson(primary, {
        jsonrpc: "2.0", id: 1, method: "suix_getReferenceGasPrice", params: [],
      }).catch(() => null);
      const gasPrice = BigInt(gasRes?.result || "1000");
      const gasBudget = token ? 5000000n : 2000000n;
      return feeInfo(gasBudget, 9, "SUI", { gasLimit: gasBudget.toString(), gasPrice: gasPrice.toString() });
    }

    if (family === "utxo") {
      const fees = await getJson(`${normalizeUrl(primary)}/fee-estimates`).catch(() => null);
      const satsPerVb = Math.ceil(Number(fees?.["3"] || fees?.["6"] || 2));
      const vbytes = token ? 180 : 140;
      return feeInfo(String(satsPerVb * vbytes), 8, "tBTC", { gasLimit: String(vbytes), gasPrice: String(satsPerVb) });
    }

    if (family === "litecoin") {
      const fees = await getJson(`${normalizeUrl(primary)}`).catch(() => null);
      const perKb = Number(fees?.high_fee_per_kb || fees?.medium_fee_per_kb || 100000);
      const vbytes = token ? 180 : 140;
      return feeInfo(String(Math.ceil((perKb / 1000) * vbytes)), 8, "tLTC", { gasLimit: String(vbytes), gasPrice: String(perKb) });
    }

    if (family === "ton") {
      return feeInfo("50000000", 9, "TON", { gasLimit: "1", gasPrice: "50000000" });
    }

    if (family === "starknet") {
      return feeInfo("100000000000000", 18, "ETH", { gasLimit: "1", gasPrice: "100000000000000" });
    }

    return { ...initialFeeInfo, display: "Fee unavailable", error: "Unsupported fee estimator" };
  }

  const activeNodeCandidates = useMemo(() => getNetworkNodeCandidates(currentNetwork), [currentNetwork, endpoints.nodeUrl]);
  const activeRestCandidates = useMemo(() => getRpcCandidatesForFamily(currentNetwork), [currentNetwork, endpoints.nodeUrl]);


  const currentBridgeChain = useMemo(() => {

    const normalized = String(bridgeChainId || "").toLowerCase();
    return bridgeChains.find((item) => String(item.id || "").toLowerCase() === normalized)
      || bridgeChains.find((item) => String(item.chain_id || "").toLowerCase() === normalized)
      || null;
  }, [bridgeChains, bridgeChainId]);

  const bridgeNativeSymbol = useMemo(() => (
    String(currentBridgeChain?.native_symbol || currentBridgeChain?.nativeSymbol || "LQD").trim() || "LQD"
  ), [currentBridgeChain]);

  const bridgeExternalFee = useMemo(() => {
    const value = Number(
      currentBridgeChain?.fee_estimate
      ?? currentBridgeChain?.feeEstimate
      ?? currentBridgeChain?.native_fee
      ?? currentBridgeChain?.nativeFee
      ?? currentBridgeChain?.relayer_fee
      ?? currentBridgeChain?.relayerFee
      ?? 0
    );
    return Number.isFinite(value) && value >= 0 ? value : 0;
  }, [currentBridgeChain]);

  const bridgeSourceFee = useMemo(() => (
    bridgeDirection === "lqd_to_external" ? Number(bridgeBaseFee || 0) : bridgeExternalFee
  ), [bridgeDirection, bridgeBaseFee, bridgeExternalFee]);

  const bridgeTargetFee = useMemo(() => (
    bridgeDirection === "lqd_to_external" ? bridgeExternalFee : Number(bridgeBaseFee || 0)
  ), [bridgeDirection, bridgeBaseFee, bridgeExternalFee]);

  const bridgeTotalFee = useMemo(() => {
    const amount = parseFloat(bridgeForm.amount) || 0;
    const platformFee = amount * 0.005; // 0.5%
    return (Number(bridgeSourceFee || 0) + Number(bridgeTargetFee || 0) + platformFee).toFixed(4);
  }, [bridgeForm.amount, bridgeSourceFee, bridgeTargetFee]);

  const bridgeHistoryItems = useMemo(() => {
    const fromRequests = (Array.isArray(bridgeRequests) ? bridgeRequests : []).map((item) => ({
      ...item,
      type: "bridge",
        tx_hash: bridgeRequestHash(item),
        amount: item.amount || item.Amount || "0",
        token: item.token || item.Token || "LQD",
        mode: item.mode || item.Mode || "public",
        family: item.family || item.Family || currentBridgeChain?.family || "evm",
        direction: item.direction || item.Direction || item.bridge_direction || item.BridgeDirection || "",
        status: item.status || item.Status || "pending",
        timestamp: item.updated_at || item.UpdatedAt || item.created_at || item.CreatedAt || 0,
      }));
    const fromActivity = (activity || [])
      .filter((item) => (item.Type || item.type) === "bridge" || (item.Type || item.type) === "lock")
      .map((item) => ({
        ...item,
        type: "bridge",
        tx_hash: item.TxHash || item.tx_hash || item.hash || "",
        amount: item.Value || item.amount || "0",
        token: item.token || item.Token || "LQD",
        mode: item.mode || item.Mode || "public",
        family: item.family || item.Family || currentBridgeChain?.family || "evm",
        direction: item.direction || item.Direction || "",
        status: item.Status || item.status || "pending",
        timestamp: item.Timestamp || item.timestamp || 0,
      }));
    const seen = new Set();
    return [...fromRequests, ...fromActivity]
      .filter((item) => {
        const key = String(item.tx_hash || item.id || item.ID || `${item.timestamp}:${item.amount}:${item.to || item.To}`).toLowerCase();
        if (!key || seen.has(key)) return false;
        seen.add(key);
        return true;
      })
      .sort((a, b) => Number(b.timestamp || 0) - Number(a.timestamp || 0))
      .slice(0, 10);
  }, [bridgeRequests, activity, currentBridgeChain]);


  const activeAddress = useMemo(() => {
    if (!wallet) return "";
    const family = String(currentNetwork.family || "evm").toLowerCase();
    if (currentNetwork.id === "lqd") return wallet.address;
    // If chainKeys are available (derived from mnemonic), use them for all chains
    if (chainKeys) {
      if (family === "evm") return chainKeys.evm.address;
      if (family === "harmony") return chainKeys.harmony.address;
      if (family === "tron") return chainKeys.tron.tronAddress || chainKeys.tron.address;
      if (family === "cosmos" || family === "cosmos-testnet") return chainKeys.cosmos.address;
      if (family === "sei") return chainKeys.sei.address;
      if (family === "injective") return chainKeys.injective.address;
      if (family === "solana") return chainKeys.solana.address;
      if (family === "near") return chainKeys.near.address;
      if (family === "aptos") return chainKeys.aptos.address;
      if (family === "sui") return chainKeys.sui.address;
      if (family === "ton") return chainKeys.ton?.address || chainKeys.ton?.rawPubkey || "";
      if (family === "utxo") return chainKeys.btc?.address || "";
      if (family === "litecoin") return chainKeys.ltc?.address || "";
      if (family === "starknet") {
        // Use Pedersen-based correct ArgentX address (starknet.js)
        const pub = chainKeys.starknet?.publicKey;
        return pub ? starknetArgentAddress(pub) : (chainKeys.starknet?.address || "");
      }
    }
    // Fallback: legacy derivation (EVM key only)
    if (family === "evm") return wallet.address;
    if (family === "harmony") return deriveHarmonyAddress(wallet.address);
    const derived = deriveFamilyAddress(wallet.privateKey, family);
    if (derived) return derived;
    return watchAddresses[activeNetworkId] || "";
  }, [wallet, chainKeys, currentNetwork, activeNetworkId, watchAddresses]);

  useEffect(() => {
    if (!sendVisible || !wallet?.address) return;
    let alive = true;
    setNativeFeeInfo((prev) => ({ ...prev, loading: true, error: "" }));
    estimateWalletFee()
      .then((info) => { if (alive) setNativeFeeInfo(info); })
      .catch((error) => {
        if (alive) setNativeFeeInfo({ ...initialFeeInfo, display: "Fee unavailable", error: error?.message || "Fee estimate failed" });
      });
    return () => { alive = false; };
  }, [sendVisible, sendForm.to, sendForm.amount, activeNetworkId, activeAddress, nodeUrl, wallet?.address]);

  useEffect(() => {
    if (!selectedTokenForSend || !wallet?.address) return;
    let alive = true;
    setTokenFeeInfo((prev) => ({ ...prev, loading: true, error: "" }));
    estimateWalletFee({ token: selectedTokenForSend })
      .then((info) => { if (alive) setTokenFeeInfo(info); })
      .catch((error) => {
        if (alive) setTokenFeeInfo({ ...initialFeeInfo, display: "Fee unavailable", error: error?.message || "Token fee estimate failed" });
      });
    return () => { alive = false; };
  }, [selectedTokenForSend, tokenSendForm.to, tokenSendForm.amount, activeNetworkId, activeAddress, nodeUrl, wallet?.address]);

  function liquidityInputKey(card) {
    return card?.address ? String(card.address).toLowerCase() : `slot:${card?.slotId || card?.label || "unknown"}`;
  }

  function updateLiquidityPoolInput(card, patch) {
    const key = liquidityInputKey(card);
    setLiquidityPoolInputs((prev) => ({
      ...prev,
      [key]: { lockAmount: "", lockDays: "365", ...(prev[key] || {}), ...patch },
    }));
  }

  function setLiquidityPoolResult(card, message, type = "info") {
    const key = liquidityInputKey(card);
    setLiquidityPoolResults((prev) => ({ ...prev, [key]: { message, type } }));
  }

  async function buildMobilePoolCards(poolsPayload) {
    const registryPools = await dexRegistryPools().catch(() => []);
    const poolMap = normalizePoolMap(poolsPayload);
    const entries = (registryPools || [])
      .map((pool) => ({
        ...pool,
        address: normalizeAddress(pool.address || pool.pool || ""),
      }))
      .filter((pool) => pool.address)
      .slice(0, 40);
    const cards = [];
    for (const entry of entries) {
      const address = entry.address;
      let storage = {};
      try {
        storage = normalizeContractStorage(await nodeContractStorage(nodeUrl, address));
      } catch {
        storage = {};
      }
      const form = inferPoolForm(address, storage, {
        tokenA: entry.tokenA || entry.token_a || "lqd",
        tokenB: entry.tokenB || entry.token_b || "",
        decimalsA: entry.decimalsA || entry.decimals_a || 8,
        decimalsB: entry.decimalsB || entry.decimals_b || 8,
      });
      const slot = canonicalSlotForForm(form, entry);
      const dex = resolveDexStorage(storage, form, activeAddress || wallet?.address);
      cards.push({
        address,
        form,
        storage,
        dex,
        registryLiquidity: poolMap[address] || poolMap[String(address).toLowerCase()] || "0",
        pair: entry.pairKey || entry.pair_key || `${tokenLabel(form.tokenA)} / ${tokenLabel(form.tokenB)}`,
        slotId: slot?.id || "",
        tierLabel: slot?.tier || (entry.tier ? `Tier ${entry.tier}` : "Tier 3"),
        weightLabel: slot?.weight || (entry.weight ? `${entry.weight}x` : "0.35x"),
      });
    }
    return cards;
  }

  async function loadLiquidityDashboard() {
    if (!wallet?.address) return;
    setLiquidityLoading(true);
    try {
      const [protocol, providers, latestRewards, recentRewards, poolsPayload] = await Promise.all([
        getJson(`${normalizeUrl(nodeUrl)}/liquidity/info?address=${encodeURIComponent(String(wallet.address || "").toLowerCase())}`).catch(() => null),
        getJson(`${normalizeUrl(nodeUrl)}/liquidity/all`).catch(() => []),
        getJson(`${normalizeUrl(nodeUrl)}/rewards/latest?address=${encodeURIComponent(wallet.address)}`).catch(() => null),
        getJson(`${normalizeUrl(nodeUrl)}/rewards/recent?limit=10`).catch(() => []),
        nodeLiquidityPools(nodeUrl).catch(() => null),
      ]);
      await syncDexRegistryIntoWallet().catch(() => null);
      const poolCards = await buildMobilePoolCards(poolsPayload || {});
      setLiquidityDashboard({
        protocol,
        providers: Array.isArray(providers) ? providers : [],
        latestRewards,
        recentRewards: Array.isArray(recentRewards) ? recentRewards : [],
        poolCards,
      });
    } catch (error) {
      showToast(error?.message || "Liquidity dashboard failed", "error");
    } finally {
      setLiquidityLoading(false);
    }
  }

  async function syncLiquidityPools() {
    setLiquidityAction("sync");
    try {
      await postJson(`${normalizeUrl(nodeUrl)}/liquidity/pools/sync`, {});
      await loadLiquidityDashboard();
      showToast("Liquidity pools synced", "success");
    } catch (error) {
      showToast(error?.message || "Pool sync failed", "error");
    } finally {
      setLiquidityAction("");
    }
  }

  async function claimLiquidityRewards() {
    if (!wallet?.address) return showToast("Unlock wallet first", "error");
    setLiquidityAction("claim");
    setProcessingMessage("Claiming LP rewards...");
    try {
      const res = await postJson(`${normalizeUrl(nodeUrl)}/liquidity/claim`, { address: wallet.address });
      const hash = extractTxHash(res);
      showToast("LP reward claim submitted", "success");
      rememberActivity({
        type: "lp_claim",
        TxHash: hash,
        From: wallet.address,
        To: "LP Rewards",
        Status: hash ? "pending" : "submitted",
        Timestamp: Math.floor(Date.now() / 1000),
      });
      await loadLiquidityDashboard();
      setTimeout(() => refreshWalletSnapshot(), 2500);
    } catch (error) {
      showToast(error?.message || "Claim failed", "error");
    } finally {
      setLiquidityAction("");
      setProcessingMessage("");
    }
  }

  async function submitPoolLockAction(card) {
    if (!wallet?.address || !wallet?.privateKey) return showToast("Unlock wallet first", "error");
    if (!card?.address) return showToast("Configure this pool first", "error");
    const input = liquidityPoolInputs[liquidityInputKey(card)] || {};
    const amount = String(input.lockAmount || "").trim();
    const days = Math.max(1, parseInt(input.lockDays || "365", 10) || 365);
    if (!amount) return showToast("Enter LP amount", "error");
    setLiquidityAction(`lock:${liquidityInputKey(card)}`);
    setProcessingMessage("Broadcasting LP lock transaction...");
    try {
      const rawLP = parseUnits(amount, 8);
      const durationSecs = String(days * 86400);
      const baseFee = await nodeBaseFee(nodeUrl).catch(() => 10);
      const baseFeeValue = baseFee?.base_fee || baseFee?.BaseFee || baseFee?.baseFee || baseFee || 10;
      const useFactory = isFactoryPoolForm(card.form, card.storage);
      const args = useFactory ? [card.form.tokenA, card.form.tokenB, rawLP, durationSecs] : [rawLP, durationSecs];
      const res = await walletContractTx(walletUrl, {
        address: wallet.address,
        contract_address: card.form.pool,
        function: "LockLPForValidation",
        args,
        value: "0",
        gas: 500000,
        gas_price: Number(baseFeeValue || 10),
        private_key: wallet.privateKey,
      });
      const hash = extractTxHash(res);
      setLiquidityPoolResult(card, `LP lock submitted${hash ? `: ${shortAddress(hash, 8, 6)}` : ""}`, "success");
      updateLiquidityPoolInput(card, { lockAmount: "" });
      rememberActivity({ type: "lp_lock", TxHash: hash, From: wallet.address, To: card.address, Contract: card.address, Value: rawLP, Status: hash ? "pending" : "submitted", Timestamp: Math.floor(Date.now() / 1000) });
      showToast("LP lock submitted", "success");
      setTimeout(() => loadLiquidityDashboard(), 3000);
    } catch (error) {
      setLiquidityPoolResult(card, error?.message || "LP lock failed", "error");
      showToast(error?.message || "LP lock failed", "error");
    } finally {
      setLiquidityAction("");
      setProcessingMessage("");
    }
  }

  async function submitPoolUnlockAction(card) {
    if (!wallet?.address || !wallet?.privateKey) return showToast("Unlock wallet first", "error");
    if (!card?.address) return showToast("Configure this pool first", "error");
    setLiquidityAction(`unlock:${liquidityInputKey(card)}`);
    setProcessingMessage("Broadcasting LP unlock transaction...");
    try {
      const baseFee = await nodeBaseFee(nodeUrl).catch(() => 10);
      const baseFeeValue = baseFee?.base_fee || baseFee?.BaseFee || baseFee?.baseFee || baseFee || 10;
      const useFactory = isFactoryPoolForm(card.form, card.storage);
      const args = useFactory ? [card.form.tokenA, card.form.tokenB] : [];
      const res = await walletContractTx(walletUrl, {
        address: wallet.address,
        contract_address: card.form.pool,
        function: "UnlockValidatorLP",
        args,
        value: "0",
        gas: 500000,
        gas_price: Number(baseFeeValue || 10),
        private_key: wallet.privateKey,
      });
      const hash = extractTxHash(res);
      setLiquidityPoolResult(card, `LP unlock submitted${hash ? `: ${shortAddress(hash, 8, 6)}` : ""}`, "success");
      rememberActivity({ type: "lp_unlock", TxHash: hash, From: wallet.address, To: card.address, Contract: card.address, Status: hash ? "pending" : "submitted", Timestamp: Math.floor(Date.now() / 1000) });
      showToast("LP unlock submitted", "success");
      setTimeout(() => loadLiquidityDashboard(), 3000);
    } catch (error) {
      setLiquidityPoolResult(card, error?.message || "LP unlock failed", "error");
      showToast(error?.message || "LP unlock failed", "error");
    } finally {
      setLiquidityAction("");
      setProcessingMessage("");
    }
  }

  async function registerPoolValidator(card) {
    if (!wallet?.address) return showToast("Unlock wallet first", "error");
    if (!card?.address) return showToast("Configure this pool first", "error");
    const pairAddress = getPairAddressForLPAction(card.form, card.storage);
    if (!pairAddress) return showToast("Pool pair address not found", "error");
    setLiquidityAction(`register:${liquidityInputKey(card)}`);
    try {
      const res = await postJson(`${normalizeUrl(nodeUrl)}/validator/register-dex`, {
        address: wallet.address,
        pair_address: pairAddress,
      });
      const hash = extractTxHash(res);
      rememberActivity({
        type: "validator_register",
        TxHash: hash,
        From: wallet.address,
        To: pairAddress,
        Contract: pairAddress,
        Status: hash ? "pending" : "submitted",
        Timestamp: Math.floor(Date.now() / 1000),
      });
      setLiquidityPoolResult(card, `Validator registration submitted: ${JSON.stringify(res)}`, "success");
      showToast("Validator registration submitted", "success");
      await loadLiquidityDashboard();
    } catch (error) {
      setLiquidityPoolResult(card, error?.message || "Validator registration failed", "error");
      showToast(error?.message || "Validator registration failed", "error");
    } finally {
      setLiquidityAction("");
    }
  }

  useEffect(() => {
    if (tab !== "liquidity" || !wallet?.address) return;
    loadLiquidityDashboard();
  }, [tab, wallet?.address, activeAddress, nodeUrl]);


  const currentBridgeFamily = String(currentBridgeChain?.family || "evm").toLowerCase();
  const bridgeWalletSignedFamilies = ["evm", "harmony", "sei", "monad"];
  const isExternalBridgeFamily = ["aptos", "bitcoin", "btc", "cardano", "cosmos", "cosmos-testnet", "injective", "litecoin", "near", "solana", "starknet", "substrate", "sui", "ton", "tron", "utxo", "xrpl"].includes(currentBridgeFamily);

  const unlockInProgress = useRef(false);
  const scanHandlerRef = useRef(() => { });
  const browserRef = useRef(null);

  function rememberBrowserUrl(url) {
    const next = String(url || "").trim();
    if (!next) return;
    setBrowserHistory((items) => [next, ...items.filter((item) => item !== next)].slice(0, 8));
  }

  function openBrowserUrl(value) {
    const raw = String(value || "").trim();
    if (!raw) {
      showToast("Enter a website URL first", "info");
      return;
    }
    const next = coerceBrowserUrl(raw);
    setBrowserUrl(next);
    setBrowserInput("");
    rememberBrowserUrl(next);
    setBrowserVisible(false);
  }

  const lqdProviderScript = useMemo(() => {
    let currentOrigin = "";
    try {
      currentOrigin = browserUrl ? new URL(browserUrl).origin : "";
    } catch {
      currentOrigin = "";
    }
    const isTrusted = trustedOrigins.includes(currentOrigin);
    const selectedAddress = isTrusted ? (activeAddress || wallet?.address || "") : "";
    const chainId = currentNetwork?.chainId || "0x8b";
    const networkVersion = parseInt(chainId, 16).toString();

    return `
    (function() {
      if (window.lqd && window.lqd._initialized) return;

      var requestId = 0;
      var pending = {};
      var eventListeners = {};

      function emit(event, data) {
        if (eventListeners[event]) {
          eventListeners[event].forEach(function(cb) { try { cb(data); } catch(e) {} });
        }
      }

      function lqdRequest(method, params) {
        return provider.request({ method: method, params: params || [] });
      }

      var provider = {
        isLQD: true,
        isMetaMask: true, // Compatibility for dApps that only look for MetaMask
        _initialized: true,
        selectedAddress: ${JSON.stringify(selectedAddress)},
        chainId: ${JSON.stringify(chainId)},
        networkVersion: ${JSON.stringify(networkVersion)},

        request: function(args) {
          var payload = args || {};
          if (typeof args === 'string') payload = { method: args, params: [] };
          
          var id = String(requestId++);
          return new Promise(function(resolve, reject) {
            pending[id] = { resolve: resolve, reject: reject };
            window.ReactNativeWebView.postMessage(JSON.stringify({
              source: "lqd-mobile-provider",
              id: id,
              method: payload.method,
              params: payload.params || [],
              origin: window.location.origin,
              name: document.title || window.location.host || "dApp"
            }));
          });
        },

        send: function(method, params) {
          if (typeof method === 'string') return this.request({ method: method, params: params });
          return this.request(method);
        },

        sendAsync: function(payload, callback) {
          this.request(payload).then(function(res) { 
            callback(null, { id: payload.id, jsonrpc: "2.0", result: res });
          }).catch(function(err) { callback(err); });
        },

        on: function(event, cb) {
          eventListeners[event] = eventListeners[event] || [];
          eventListeners[event].push(cb);
          return this;
        },

        removeListener: function(event, cb) {
          if (!eventListeners[event]) return this;
          eventListeners[event] = eventListeners[event].filter(function(i) { return i !== cb; });
          return this;
        },

        off: function(event, cb) { return this.removeListener(event, cb); },

        once: function(event, cb) {
          var self = this;
          var wrap = function(data) { self.removeListener(event, wrap); cb(data); };
          return this.on(event, wrap);
        },

        isConnected: function() { return !!this.selectedAddress; }
      };

      // Define standard getters/setters for address
      var _currentAddr = ${JSON.stringify(selectedAddress)};
      Object.defineProperty(provider, 'selectedAddress', { 
        get: function() { return _currentAddr; },
        set: function(v) { _currentAddr = v; }
      });

      window.lqd = provider;
      window.ethereum = provider;
      window.solana = {
        isPhantom: true,
        publicKey: _currentAddr ? { toString: function() { return _currentAddr; } } : null,
        connect: function() { return lqdRequest("solana_connect", []).then(function(res) {
          var addr = res && (res.publicKey || res.address || (Array.isArray(res) ? res[0] : ""));
          if (addr) { _currentAddr = addr; this.publicKey = { toString: function() { return addr; } }; }
          return { publicKey: this.publicKey };
        }.bind(this)); },
        disconnect: function() { _currentAddr = ""; this.publicKey = null; emit("disconnect"); return Promise.resolve(); },
        signMessage: function(message) { return lqdRequest("solana_signMessage", [message]); },
        signAndSendTransaction: function(transaction) { return lqdRequest("solana_signAndSendTransaction", [transaction]); }
      };
      window.phantom = window.phantom || {};
      window.phantom.solana = window.solana;
      window.aptos = {
        connect: function() { return lqdRequest("aptos_connect", []); },
        account: function() { return lqdRequest("aptos_account", []); },
        signMessage: function(payload) { return lqdRequest("aptos_signMessage", [payload]); },
        signAndSubmitTransaction: function(payload) { return lqdRequest("aptos_signAndSubmitTransaction", [payload]); }
      };
      window.petra = window.aptos;
      window.suiWallet = {
        requestPermissions: function() { return lqdRequest("sui_requestPermissions", []); },
        getAccounts: function() { return lqdRequest("sui_getAccounts", []); },
        signPersonalMessage: function(payload) { return lqdRequest("sui_signPersonalMessage", [payload]); },
        signAndExecuteTransactionBlock: function(payload) { return lqdRequest("sui_signAndExecuteTransactionBlock", [payload]); }
      };
      window.sui = window.suiWallet;
      window.keplr = {
        enable: function(chainId) { return lqdRequest("keplr_enable", [chainId]); },
        getKey: function(chainId) { return lqdRequest("keplr_getKey", [chainId]); },
        signAmino: function(chainId, signer, signDoc) { return lqdRequest("keplr_signAmino", [chainId, signer, signDoc]); },
        sendTx: function(chainId, tx, mode) { return lqdRequest("keplr_sendTx", [chainId, tx, mode]); }
      };
      window.ton = {
        connect: function() { return lqdRequest("ton_connect", []); },
        sendTransaction: function(payload) { return lqdRequest("ton_sendTransaction", [payload]); },
        signData: function(payload) { return lqdRequest("ton_signData", [payload]); }
      };
      window.tonkeeper = window.ton;
      window.starknet = {
        isConnected: false,
        selectedAddress: _currentAddr || "",
        enable: function() { return lqdRequest("starknet_enable", []).then(function(accounts) {
          var addr = Array.isArray(accounts) ? accounts[0] : (accounts && accounts.address);
          if (addr) { this.selectedAddress = addr; this.isConnected = true; }
          return Array.isArray(accounts) ? accounts : [addr].filter(Boolean);
        }.bind(this)); },
        request: function(payload) { return lqdRequest((payload || {}).method, (payload || {}).params || []); },
        execute: function(calls) { return lqdRequest("starknet_execute", [calls]); },
        signMessage: function(message) { return lqdRequest("starknet_signMessage", [message]); }
      };
      window.starknet_argentX = window.starknet;

      window.__LQD_MOBILE_PROVIDER_RESPONSE__ = function(message) {
        var req = pending[String(message.id)];
        if (!req) return;
        delete pending[String(message.id)];
        if (message.ok) {
          if (message.method === "eth_requestAccounts" || message.method === "lqd_requestAccounts" || message.method === "lqd_connect" || message.method === "wallet_requestPermissions") {
            _currentAddr = Array.isArray(message.result) ? message.result[0] : (message.result && (message.result.address || message.result.publicKey)) || _currentAddr;
            emit("accountsChanged", [_currentAddr]);
          }
          req.resolve(message.result);
        } else {
          req.reject(new Error(message.error || "Request rejected"));
        }
      };

      window.__LQD_MOBILE_SET_ACCOUNT__ = function(address, chainId) {
        _currentAddr = address || "";
        if (chainId) provider.chainId = chainId;
        emit("accountsChanged", [_currentAddr].filter(Boolean));
        if (chainId) emit("chainChanged", chainId);
      };

      // Dispatch initialization events
      setTimeout(function() {
        window.dispatchEvent(new CustomEvent("lqd#initialized", { detail: provider }));
        window.dispatchEvent(new CustomEvent("ethereum#initialized", { detail: provider }));
      }, 50);
      
      return true;
    })();
    `;
  }, [wallet?.address, activeAddress, currentNetwork?.chainId, trustedOrigins]);

  // Always keep ref fresh — no deps so every render updates it.
  // Camera onBarcodeScanned uses this ref to avoid stale closures.
  useEffect(() => {
    scanHandlerRef.current = openFromScan;
  });

  // Auto-fetch token metadata when address is pasted in import modal (Trust Wallet style)
  useEffect(() => {
    if (!tokenImportVisible) { setTokenPreviewMeta(null); return; }
    const addr = tokenImportForm.address.trim();
    const family = currentNetwork?.family || "evm";
    if (!addr || !isLikelyAddressForFamily(addr, family)) { setTokenPreviewMeta(null); return; }
    let cancelled = false;
    setTokenPreviewLoading(true);
    setTokenPreviewMeta(null);
    const effectiveNodeUrls = getRpcCandidatesForFamily(currentNetwork, family);
    const holderAddr = activeAddress || wallet?.address || "";
    const timer = setTimeout(() => {
      resolveTokenMetaMultichain(effectiveNodeUrls, addr, holderAddr, family)
        .then(meta => { if (!cancelled) setTokenPreviewMeta(meta || null); })
        .catch(() => { if (!cancelled) setTokenPreviewMeta(null); })
        .finally(() => { if (!cancelled) setTokenPreviewLoading(false); });
    }, 350);
    return () => { cancelled = true; clearTimeout(timer); };
  }, [tokenImportForm.address, tokenImportVisible, currentNetwork, activeAddress, wallet?.address]);

  useEffect(() => {
    const handleUrl = ({ url }) => {
      if (url && typeof scanHandlerRef.current === "function") {
        scanHandlerRef.current(url);
      }
    };
    const sub = Linking.addEventListener("url", handleUrl);
    Linking.getInitialURL().then((url) => {
      if (url) handleUrl({ url });
    }).catch(() => { });
    return () => {
      sub?.remove?.();
    };
  }, []);

  useEffect(() => {
    loadBridgeChains().then(() => {
      setTimeout(() => loadBridgeTokens(), 500);
    });
    loadBridgeFamilies();
  }, []);

  useEffect(() => {
    if (!bridgeChainId) return;
    loadBridgeTokens().catch(() => null);
  }, [bridgeChainId]);

  useEffect(() => {
    if (!activeBridgeTx) return undefined;
    let cancelled = false;
    const tick = async () => {
      if (cancelled) return;
      await refreshBridgeTracking(activeBridgeTx).catch(() => null);
    };
    tick();
    const timer = setInterval(tick, 10000);
    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  }, [activeBridgeTx?.tx_hash, activeBridgeTx?.hash, nodeUrl]);

  useEffect(() => {
    if (!wallet?.address || !settingsAutoRefresh) return undefined;
    let cancelled = false;
    const tick = () => {
      refreshWalletSnapshot("", { full: false, refreshTokens: false }).catch((e) => {
        if (!cancelled) setStatus(e.message || "Refresh failed");
      });
    };
    tick();
    const timer = setInterval(tick, 10000);
    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  }, [wallet?.address, settingsAutoRefresh, nodeUrl, walletUrl, activeNetworkId]);

  // Derive all chain keys from mnemonic in background; update chainKeys + activeAddress
  async function applyChainKeys(mnemonic) {
    if (!mnemonic) return;
    try {
      const keys = deriveAllChainKeys(mnemonic);
      setChainKeys(keys);
      // Also update multiWallets for backwards-compat display
      setMultiWallets({
        evm: { address: keys.evm.address, pk: keys.evm.privateKey },
        harmony: { address: keys.harmony.address, pk: keys.harmony.privateKey },
        cosmos: { address: keys.cosmos.address, pk: keys.cosmos.privateKey },
        sei: { address: keys.sei.address, pk: keys.sei.privateKey },
        injective: { address: keys.injective.address, pk: keys.injective.privateKey },
        tron: { address: keys.tron.tronAddress || keys.tron.address, pk: keys.tron.privateKey },
        solana: { address: keys.solana.address, pk: keys.solana.privateKey },
        near: { address: keys.near.address, pk: keys.near.privateKey },
        aptos: { address: keys.aptos.address, pk: keys.aptos.privateKey },
        sui: { address: keys.sui.address, pk: keys.sui.privateKey },
        ton: { address: keys.ton.address || keys.ton.rawPubkey, pk: keys.ton.privateKey },
        btc: { address: keys.btc?.address, pk: keys.btc?.privateKey },
        ltc: { address: keys.ltc?.address, pk: keys.ltc?.privateKey },
        starknet: { address: keys.starknet?.address, pk: keys.starknet?.privateKey },
      });
    } catch (e) {
      console.warn("deriveAllChainKeys failed:", e.message);
    }
  }

  async function persistWalletVault(vault, password) {
    const cipher = encryptVault(vault, password);
    const record = { address: vault.address, cipher, createdAt: Date.now() };
    await saveJSON(STORAGE_KEYS.vault, record);

    // Security Hardening: Try to store raw private key in hardware-backed SecureStore if available
    if (SecureStore && typeof SecureStore.setItemAsync === "function") {
      try {
        const options = Platform.OS === "ios" && SecureStore.AFTER_FIRST_UNLOCK_THIS_DEVICE_ONLY
          ? { keychainAccessible: SecureStore.AFTER_FIRST_UNLOCK_THIS_DEVICE_ONLY }
          : {};
        await SecureStore.setItemAsync(`pk_${vault.address}`, vault.privateKey, options);
      } catch (e) {
        console.warn("SecureStore set failed:", e.message);
      }
    }

    if (biometricEnabled) {
      try {
        await saveString(STORAGE_KEYS.biometricVault, JSON.stringify(vault), { requireAuthentication: true });
      } catch {
      }
    } else {
      await removeItem(STORAGE_KEYS.biometricVault);
    }
    setVaultRecord(record);
  }

  async function refreshWalletSnapshot(addressOverride = "", refreshOptions = {}) {
    const targetAddr = (addressOverride || activeAddress || "").trim();
    if (!targetAddr) return;
    const forceRefresh = !!refreshOptions.force;
    const fullRefresh = refreshOptions.full !== false;
    const refreshTokensEnabled = refreshOptions.refreshTokens !== false;
    const autoDiscoverEnabled = refreshOptions.autoDiscover === true;
    const refreshKey = `${activeNetworkId}:${targetAddr}`;
    const now = Date.now();
    if (!forceRefresh && refreshInFlightRef.current?.key === refreshKey) {
      return refreshInFlightRef.current.promise;
    }
    if (!forceRefresh && lastRefreshRequestRef.current.key === refreshKey && now - lastRefreshRequestRef.current.at < 1200) {
      return;
    }
    if (isRefreshing && !forceRefresh) return;

    setIsRefreshing(true);
    let completeRefresh = () => {};
    const refreshPromise = new Promise((resolve) => { completeRefresh = resolve; });
    refreshInFlightRef.current = { key: refreshKey, promise: refreshPromise };
    setBalanceLoadError("");
    setNetworkHealth((prev) => ({ ...prev, state: "checking", message: "Checking node" }));
    try {
      const family = currentNetwork.family || 'evm';
      const nodeCandidates = getNetworkNodeCandidates(currentNetwork);
      const rpcCandidates = getRpcCandidatesForFamily(currentNetwork, family);
      const primaryNodeUrl = nodeCandidates[0] || nodeUrl;
      const primaryRpcUrl = rpcCandidates[0] || primaryNodeUrl;
      const refreshStartedAt = Date.now();
      const rpcPromises = [
        // Handle Status — per-family health check
        (async () => {
          try {
            if (family === 'evm') {
              const isLqd = primaryNodeUrl.includes('lqd') || primaryNodeUrl.includes('192.168') || primaryNodeUrl.includes('railway');
              if (isLqd) return await tryRpcCandidates(nodeCandidates, (url) => nodeStatus(url), (res) => !!res).then(({ result }) => result || { online: false }).catch(() => ({ online: false }));
              // External EVM (Sepolia, BSC, etc.): /blockchain doesn't exist, use eth_blockNumber
              const { result: res } = await tryRpcCandidates(nodeCandidates, (url) => postJson(url, { jsonrpc: "2.0", id: 1, method: "eth_blockNumber", params: [] }).catch(() => null), (r) => !!r?.result).catch(() => ({ result: null }));
              return { online: !!res?.result };
            }
            if (family === 'harmony' || family === 'tron') {
              const { result: res } = await tryRpcCandidates(nodeCandidates, (url) => postJson(url, { jsonrpc: "2.0", id: 1, method: "eth_blockNumber", params: [] }).catch(() => null), (r) => !!r?.result).catch(() => ({ result: null }));
              return { online: !!res?.result };
            }
            if (family === 'solana') {
              const { result: res } = await tryRpcCandidates(nodeCandidates, (url) => postJson(url, { jsonrpc: "2.0", id: 1, method: "getHealth", params: [] }).catch(() => null), (r) => !!(r?.result === 'ok' || r?.result)).catch(() => ({ result: null }));
              return { online: !!(res?.result === 'ok' || res?.result) };
            }
            if (family === 'near') {
              const { result: res } = await tryRpcCandidates(nodeCandidates, (url) => postJson(url, { jsonrpc: "2.0", id: 1, method: "status", params: [] }).catch(() => null), (r) => !!r?.result).catch(() => ({ result: null }));
              return { online: !!res?.result };
            }
            if (family === 'aptos') {
              const { result: res } = await tryRpcCandidates(nodeCandidates, (url) => getJson(`${url}/`).catch(() => null), (r) => !!r?.chain_id).catch(() => ({ result: null }));
              return { online: !!res?.chain_id };
            }
            if (family === 'sui') {
              const { result: res } = await tryRpcCandidates(nodeCandidates, (url) => postJson(url, { jsonrpc: "2.0", id: 1, method: "sui_getChainIdentifier", params: [] }).catch(() => null), (r) => !!r?.result).catch(() => ({ result: null }));
              return { online: !!res?.result };
            }
            if (family === 'ton') {
              const { result: res } = await tryRpcCandidates(nodeCandidates, (url) => {
                const tonBase = url.replace(/\/jsonRPC$/, '').replace(/\/api\/v\d\/jsonRPC$/, '');
                return getJson(`${tonBase}/getMasterchainInfo`).catch(() => null);
              }, (r) => !!r?.result).catch(() => ({ result: null }));
              return { online: !!res?.result };
            }
            if (family === 'cosmos' || family === 'cosmos-testnet') {
              const { result: res } = await tryRpcCandidates(rpcCandidates, async (url) =>
                await getJson(`${url}/cosmos/base/tendermint/v1beta1/syncing`, { timeoutMs: 8000 }).catch(() => null)
                || await getJson(`${url}/cosmos/base/tendermint/v1beta1/node_info`, { timeoutMs: 8000 }).catch(() => null)
              ).catch(() => ({ result: null }));
              return { online: res !== null };
            }
            if (family === 'sei') {
              const { result: res } = await tryRpcCandidates(rpcCandidates, async (url) =>
                await getJson(`${url}/cosmos/base/tendermint/v1beta1/syncing`, { timeoutMs: 8000 }).catch(() => null)
                || await getJson(`${url}/cosmos/base/tendermint/v1beta1/node_info`, { timeoutMs: 8000 }).catch(() => null)
              ).catch(() => ({ result: null }));
              return { online: res !== null };
            }
            if (family === 'injective') {
              const { result: res } = await tryRpcCandidates(rpcCandidates, async (url) =>
                await getJson(`${url}/cosmos/base/tendermint/v1beta1/syncing`, { timeoutMs: 8000 }).catch(() => null)
                || await getJson(`${url}/cosmos/base/tendermint/v1beta1/node_info`, { timeoutMs: 8000 }).catch(() => null)
              ).catch(() => ({ result: null }));
              return { online: res !== null };
            }
            if (family === 'starknet') {
              const { result: res } = await tryRpcCandidates(nodeCandidates, (url) => postJson(url, { jsonrpc: "2.0", id: 1, method: "starknet_blockNumber", params: [] }).catch(() => null), (r) => r?.result !== undefined && r?.result !== null).catch(() => ({ result: null }));
              return { online: res?.result !== undefined && res?.result !== null };
            }
            return { online: true };
          } catch { return { online: true }; }
        })(),
        // Handle Balance
        (async () => {
          if (family === 'evm') {
            if (currentNetwork.id === 'lqd') {
              return await tryRpcCandidates(
                nodeCandidates,
                (url) => walletBalance(url, targetAddr).catch(() => null),
                (r) => r && (r.balance !== undefined || r.Balance !== undefined || r.amount !== undefined)
              ).then(({ result }) => result).catch(() => null);
            }

            // Try standard JSON-RPC first as it's more universal for Sepolia/BSC/etc.
            try {
              const { result: res } = await tryRpcCandidates(nodeCandidates, (url) => postJson(url, {
                jsonrpc: "2.0",
                id: 1,
                method: "eth_getBalance",
                params: [targetAddr, "latest"]
              }).catch(() => null), (r) => !!r?.result).catch(() => ({ result: null }));

              if (res && res.result) {
                // Convert hex to decimal string
                const dec = BigInt(res.result).toString();
                return { balance: dec };
              }
            } catch (e) { /* ignore and try custom */ }

            // Fallback to custom LQD API
            return await tryRpcCandidates(nodeCandidates, (url) => walletBalance(url, targetAddr).catch(() => null), (r) => !!r).then(({ result }) => result).catch(() => null);
          }
          // Bug fix #1: Harmony uses EVM-compatible JSON-RPC — same as EVM path
          if (family === 'harmony') {
            const { result: res } = await tryRpcCandidates(nodeCandidates, (url) => postJson(url, {
              jsonrpc: "2.0", id: 1, method: "eth_getBalance", params: [targetAddr, "latest"]
            }).catch(() => null), (r) => !!r?.result).catch(() => ({ result: null }));
            if (res?.result) return { balance: BigInt(res.result).toString() };
            return await tryRpcCandidates(nodeCandidates, (url) => walletBalance(url, targetAddr).catch(() => null), (r) => !!r).then(({ result }) => result).catch(() => null);
          }
          if (family === 'solana') {
            const { result: res } = await tryRpcCandidates(nodeCandidates, (url) => postJson(url, { jsonrpc: "2.0", id: 1, method: "getBalance", params: [targetAddr] }).catch(() => null), (r) => r?.result?.value !== undefined).catch(() => ({ result: null }));
            return res ? { balance: res?.result?.value || 0 } : null;
          }
          if (family === 'cosmos') {
            const { result: res } = await tryRpcCandidates(rpcCandidates, (url) => getJson(`${url}/cosmos/bank/v1beta1/balances/${targetAddr}`).catch(() => null), (r) => !!r?.balances).catch(() => ({ result: null }));
            const bal = res?.balances?.find(b => b.denom === 'uatom' || b.denom === 'stake' || b.denom === 'atom');
            return bal ? { balance: bal.amount } : { balance: 0 };
          }
          if (family === 'utxo') {
            // Blockstream API
            const res = await getJson(`${nodeUrl}/address/${targetAddr}`).catch(() => null);
            const balance = (res?.chain_stats?.funded_txo_sum || 0) - (res?.chain_stats?.spent_txo_sum || 0);
            return { balance };
          }
          if (family === 'litecoin') {
            // BlockCypher API: /addrs/{addr}/balance
            const res = await getJson(`${nodeUrl}/addrs/${targetAddr}/balance`).catch(() => null);
            return { balance: res?.balance ?? 0 };
          }
          // Bug fix #2: TRON JSON-RPC endpoint uses eth_getBalance, not a REST body
          if (family === 'tron') {
            const evmAddr = tronAddressToEvm(targetAddr) || targetAddr;
            const { result: res } = await tryRpcCandidates(nodeCandidates, (url) => postJson(url, {
              jsonrpc: "2.0", id: 1, method: "eth_getBalance", params: [evmAddr, "latest"]
            }).catch(() => null), (r) => !!r?.result).catch(() => ({ result: null }));
            if (res?.result) return { balance: BigInt(res.result).toString() };
            return { balance: 0 };
          }
          if (family === 'ton') {
            if (!targetAddr) return { balance: 0 };
            // targetAddr is now EQ.../UQ... (v3R2 wallet address from tonV3R2Address)
            // TonCenter jsonRPC: getAddressBalance or getAddressInformation
            const { result: res } = await tryRpcCandidates(nodeCandidates, (url) => {
              const tonBase = url.includes('jsonRPC') ? url.replace(/\/jsonRPC$/, '') : url;
              return getJson(`${tonBase}/getAddressInformation?address=${encodeURIComponent(targetAddr)}`).catch(() => null);
            }, (r) => !!r?.result).catch(() => ({ result: null }));
            return { balance: res?.result?.balance || 0 };
          }
          // Bug fix #6b: NEAR — activeAddress is "" for ed25519; use targetAddr param
          if (family === 'near') {
            if (!targetAddr) return { balance: 0 };
            const { result: res } = await tryRpcCandidates(nodeCandidates, (url) => postJson(url, {
              jsonrpc: "2.0", id: 1, method: "query",
              params: { request_type: "view_account", finality: "final", account_id: targetAddr }
            }).catch(() => null), (r) => !!r?.result?.amount).catch(() => ({ result: null }));
            return { balance: res?.result?.amount || 0 };
          }
          // Bug fix #3: Aptos — fetch real APT balance from CoinStore resource
          if (family === 'aptos') {
            if (!targetAddr) return { balance: 0 };
            const { result: res } = await tryRpcCandidates(nodeCandidates, (url) => getJson(`${url}/accounts/${encodeURIComponent(targetAddr)}/resources`).catch(() => null), (r) => Array.isArray(r)).catch(() => ({ result: null }));
            const store = (Array.isArray(res) ? res : []).find(r =>
              r.type === "0x1::coin::CoinStore<0x1::aptos_coin::AptosCoin>"
            );
            return { balance: store?.data?.coin?.value || 0 };
          }
          // Bug fix #5: SUI — use targetAddr, not stale activeAddress state
          if (family === 'sui') {
            if (!targetAddr) return { balance: 0 };
            const { result: res } = await tryRpcCandidates(nodeCandidates, (url) => postJson(url, {
              jsonrpc: "2.0", id: 1, method: "suix_getBalance",
              params: [targetAddr, "0x2::sui::SUI"],
            }).catch(() => null), (r) => r?.result?.totalBalance !== undefined).catch(() => ({ result: null }));
            return { balance: res?.result?.totalBalance || 0 };
          }
          // SEI/Injective: try Cosmos bank REST, then EVM fallback with correct key
          if (family === 'sei' || family === 'injective') {
            if (!targetAddr) return { balance: 0 };
            const denom = family === 'sei' ? 'usei' : 'inj';
            // SEI nodeUrl is EVM RPC; use cosmosRestUrl for Cosmos bank queries
            if (rpcCandidates.length) {
              const { result: cosmosRes } = await tryRpcCandidates(rpcCandidates, (url) => getJson(`${url}/cosmos/bank/v1beta1/balances/${targetAddr}`).catch(() => null), (r) => !!r?.balances).catch(() => ({ result: null }));
              const cosmosbal = cosmosRes?.balances?.find(b => b.denom === denom);
              if (cosmosbal?.amount) return { balance: cosmosbal.amount };
            }
            // EVM fallback using chain-specific 0x address
            const evmAddr = family === 'sei'
              ? (chainKeys?.sei?.evmAddress || chainKeys?.evm?.address)
              : (chainKeys?.injective?.evmAddress || chainKeys?.evm?.address);
            if (evmAddr) {
              const { result: evmRes } = await tryRpcCandidates(nodeCandidates, (url) => postJson(url, {
                jsonrpc: "2.0", id: 1, method: "eth_getBalance", params: [evmAddr, "latest"],
              }).catch(() => null), (r) => !!r?.result).catch(() => ({ result: null }));
              if (evmRes?.result && evmRes.result !== "0x0" && evmRes.result !== "0x") {
                return { balance: BigInt(evmRes.result).toString() };
              }
            }
            return { balance: 0 };
          }
          if (family === 'starknet') {
            if (!targetAddr) return { balance: 0 };
            // ETH balance on Starknet via starknet_call to ETH token contract
            const ETH_CONTRACT = "0x049d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7";
            const BALANCE_OF_SELECTOR = "0x2e4263afad30923c891518314c3c95dbe830a16874e8abc5777a9a20b54c76e";
            try {
              const { result: res } = await tryRpcCandidates(nodeCandidates, (url) => postJson(url, {
                jsonrpc: "2.0", id: 1, method: "starknet_call",
                params: [{ contract_address: ETH_CONTRACT, entry_point_selector: BALANCE_OF_SELECTOR, calldata: [targetAddr] }, "latest"],
              }).catch(() => null), (r) => Array.isArray(r?.result) && r.result.length >= 2).catch(() => ({ result: null }));
              // Returns [low, high] uint256
              if (res?.result?.length >= 2) {
                const low = BigInt(res.result[0] || "0x0");
                const high = BigInt(res.result[1] || "0x0");
                return { balance: (high * (1n << 128n) + low).toString() };
              }
            } catch { /* fall through */ }
            return { balance: 0 };
          }
          return null;
        })()
      ];

      const [status, native, factory, recent, requests, tokensResp, poolsResp, feeResp, dexRegistry] = await Promise.all([
        rpcPromises[0],
        rpcPromises[1],
        family === 'evm' && fullRefresh ? nodeCurrentFactory(nodeUrl).catch(() => null) : Promise.resolve(null),
        family === 'evm' && fullRefresh ? nodeRecentTransactions(nodeUrl).catch(() => []) : Promise.resolve([]),
        family === 'evm' && fullRefresh ? nodeBridgeRequests(nodeUrl).catch(() => []) : Promise.resolve([]),
        family === 'evm' && fullRefresh ? nodeBridgeTokens(nodeUrl).catch(() => []) : Promise.resolve([]),
        family === 'evm' && fullRefresh ? nodeLiquidityPools(nodeUrl).catch(() => null) : Promise.resolve(null),
        family === 'evm' && fullRefresh ? nodeBaseFee(nodeUrl).catch(() => 10) : Promise.resolve(10),
        family === 'evm' && fullRefresh ? syncDexRegistryIntoWallet().catch(() => null) : Promise.resolve(null),
      ]);

      const online = !!status?.online || !!status?.version;
      const latencyMs = Date.now() - refreshStartedAt;
      setIsNodeOnline(online);
      setNetworkHealth({
        state: online ? (latencyMs > 2500 ? "slow" : "online") : "offline",
        message: online ? (latencyMs > 2500 ? "Node is slow" : "Node online") : "Node unavailable",
        latencyMs,
        checkedAt: Date.now(),
      });

      if (family === 'evm' && feeResp) {
        setBridgeBaseFee(Number(feeResp?.base_fee || feeResp || 10));
        const base = Number(feeResp?.base_fee || feeResp || 0);
        const total = (base * 21000) / 100000000;
        setEstimatedFee(total.toFixed(8));
      }

      if (native) {
        let val = "0";
        if (native.balance !== undefined && native.balance !== null) val = String(native.balance);
        else if (native.Balance !== undefined && native.Balance !== null) val = String(native.Balance);
        else if (native.amount !== undefined && native.amount !== null) val = String(native.amount);
        setNativeBalance(val);
      } else {
        setBalanceLoadError("Balance refresh failed. Keeping the last known balance.");
      }
      if (factory?.address) {
        setFactoryAddress(factory.address);
      }
      if (Array.isArray(recent)) {
        setRecentTxs(recent);
        setActivity((prev) => {
          const merged = mergeUniqueByKey(prev, recent, "TxHash");
          // Sort by timestamp desc
          merged.sort((a, b) => Number(b.Timestamp || b.timestamp || 0) - Number(a.Timestamp || a.timestamp || 0));
          return merged.slice(0, 100);
        });
      }
      if (Array.isArray(requests)) {
        setBridgeRequests(requests);
      }
      if (Array.isArray(tokensResp)) {
        setBridgeTokens(tokensResp);
      }

      setLastWalletRefresh(Date.now());
      if (refreshTokensEnabled || autoDiscoverEnabled) {
        const backgroundTasks = [];
        if (refreshTokensEnabled) backgroundTasks.push(refreshTokenBalances(watchlist, targetAddr));
        if (autoDiscoverEnabled) {
          backgroundTasks.push(autoDiscoverTokens({
            recent: Array.isArray(recent) ? recent : [],
            bridgeTokens: Array.isArray(tokensResp) ? tokensResp : [],
            factory,
            pools: poolsResp,
            registryTokens: dexRegistry?.tokens || [],
          }, targetAddr));
        }
        Promise.allSettled(backgroundTasks).catch((error) => {
          recordError("wallet_background_refresh", error, { network: activeNetworkId }).catch(() => {});
        });
      }
    } catch (e) {
      console.warn("Refresh failed:", e.message);
      setBalanceLoadError(e.message || "Refresh failed");
      setNetworkHealth({
        state: "offline",
        message: e.message || "Node unavailable",
        checkedAt: Date.now(),
      });
    } finally {
      lastRefreshRequestRef.current = { key: refreshKey, at: Date.now() };
      if (refreshInFlightRef.current?.key === refreshKey) refreshInFlightRef.current = null;
      completeRefresh();
      setIsRefreshing(false);
    }
  }

  async function loadBridgeChains() {
    try {
      const resp = await nodeBridgeChains(nodeUrl);
      const list = Array.isArray(resp) ? resp.filter(Boolean) : [];
      setBridgeChains(list);
      const current = String(bridgeChainId || "").toLowerCase();
      const next =
        list.find((item) => String(item.id || "").toLowerCase() === current)
        || list.find((item) => String(item.chain_id || "").toLowerCase() === current)
        || list.find((item) => String(item.id || item.chain_id || "").toLowerCase() === "bsc-testnet")
        || list.find((item) => String(item.family || "evm").toLowerCase() === "evm")
        || list[0]
        || null;
      const nextId = String(next?.id || next?.chain_id || bridgeChainId || "bsc-testnet");
      if (nextId && nextId !== bridgeChainId) {
        setBridgeChainId(nextId);
      }
      const nextFamily = String(next?.family || "evm").toLowerCase();
      setBridgeForm((prev) => ({
        ...prev,
        chainId: nextId,
        ...(nextFamily === "evm" ? {
          sourceTxHash: "",
          sourceAddress: "",
          sourceMemo: "",
          sourceSequence: "",
          sourceOutput: "",
        } : {}),
      }));
      setBridgeTokenAdminForm((prev) => ({
        ...prev,
        chainId: nextId,
        family: nextFamily || String(prev.family || "evm"),
      }));
      setBridgeChainForm((prev) => ({
        ...prev,
        id: String(next?.id || nextId),
        family: nextFamily || String(prev.family || "evm"),
        adapter: String(next?.adapter || prev.adapter || nextFamily || "evm"),
      }));
      return list;
    } catch (e) {
      setBridgeChains([]);
      return [];
    }
  }

  async function loadBridgeFamilies() {
    try {
      const lqdNode = DEFAULT_ENDPOINTS.nodeUrl;
      const resp = await nodeBridgeFamilies(lqdNode);
      const list = Array.isArray(resp) ? resp.filter(Boolean) : [];
      setBridgeFamilies(list);

      // Also update bridgeChains for the flat list
      const chains = list.flatMap(f => f.Chains || f.chains || []);
      if (chains.length > 0) setBridgeChains(chains);

      return list;
    } catch {
      setBridgeFamilies([]);
      return [];
    }
  }

  async function loadBridgeTokens() {
    if (!bridgeChainId) return [];
    try {
      const lqdNode = DEFAULT_ENDPOINTS.nodeUrl;
      const tokens = await nodeBridgeTokens(lqdNode, bridgeChainId);
      const list = Array.isArray(tokens) ? tokens.filter(Boolean) : [];
      setBridgeTokens(list);
      if (list.length > 0) {
        setBridgeSelectedToken(list[0]);
        setBridgeForm(p => ({ ...p, token: list[0].symbol }));
      } else {
        setBridgeSelectedToken(null);
        setBridgeForm(p => ({ ...p, token: "" }));
      }
      return list;
    } catch {
      setBridgeTokens([]);
      return [];
    }
  }

  function applyBridgeChainSelection(cfg) {
    const chainId = String(cfg?.id || cfg?.chain_id || "").trim();
    if (!chainId) return;
    const nextFamily = String(cfg?.family || "evm").toLowerCase();
    setBridgeChainId(chainId);

    // Switch direction based on which side was clicked
    if (bridgeTargetSide === "source") {
      setBridgeDirection("external_to_lqd");
    } else {
      setBridgeDirection("lqd_to_external");
    }
    setBridgeForm((prev) => ({
      ...prev,
      chainId,
      ...(nextFamily === "evm" ? {
        sourceTxHash: "",
        sourceAddress: "",
        sourceMemo: "",
        sourceSequence: "",
        sourceOutput: "",
      } : {}),
    }));
    setBridgeTokenAdminForm((prev) => ({ ...prev, chainId, family: String(cfg?.family || prev.family || "evm") }));
    setBridgeChainForm((prev) => ({
      ...prev,
      id: String(cfg?.id || chainId),
      name: String(cfg?.name || prev.name || "").trim(),
      chainId: String(cfg?.chain_id || cfg?.chainId || chainId),
      family: nextFamily || String(prev.family || "evm"),
      adapter: String(cfg?.adapter || prev.adapter || nextFamily || "evm"),
      rpc: String(cfg?.rpc || prev.rpc || "").trim(),
      bridgeAddress: String(cfg?.bridge_address || prev.bridgeAddress || "").trim(),
      lockAddress: String(cfg?.lock_address || prev.lockAddress || "").trim(),
      explorerUrl: String(cfg?.explorer_url || prev.explorerUrl || "").trim(),
      nativeSymbol: String(cfg?.native_symbol || prev.nativeSymbol || "BNB"),
      enabled: cfg?.enabled ?? prev.enabled,
      supportsPublic: cfg?.supports_public ?? prev.supportsPublic,
      supportsPrivate: cfg?.supports_private ?? prev.supportsPrivate,
    }));
    setTimeout(() => loadBridgeTokens(), 100);
  }

  async function importDetectedTokens(candidates, addressOverride = "", source = "activity") {
    const activeAddress = addressOverride || wallet?.address;
    if (!activeAddress) return 0;
    const family = String(currentNetwork.family || "evm").toLowerCase();
    const existing = new Set((watchlist || []).map((item) => tokenKey(item)).filter(Boolean));
    const unique = [...new Set((candidates || []).map(normalizeAddress).filter(Boolean))]
      .filter((address) => !existing.has(`${activeNetworkId}:${address}`));
    if (!unique.length) return 0;

    const detected = [];
    const rpcUrls = getRpcCandidatesForFamily(currentNetwork, family);
    for (const address of unique) {
      try {
        const meta = await resolveTokenMetaMultichain(rpcUrls, address, activeAddress, family);
        const hasRealMeta = !isGenericTokenMeta(meta);
        if (!hasRealMeta) continue;
        const balance = await resolveTokenBalanceMultichain(rpcUrls, walletUrl, address, activeAddress, family);
        detected.push({ ...meta, address, balance, detectedFrom: source, networkId: activeNetworkId, family, holderAddress: activeAddress });
      } catch {
        // Ignore contracts that are not token-like.
      }
    }
    if (!detected.length) return 0;
    setWatchlist((prev) => mergeUniqueByKey(prev, detected, "address"));
    return detected.length;
  }

  function normalizeRegistryToken(token = {}) {
    const address = normalizeAddress(token.address || token.contract || "");
    if (!address || address === "lqd") return null;
    return {
      address,
      name: token.name || token.symbol || "Registry Token",
      symbol: token.symbol || address.slice(2, 6).toUpperCase(),
      decimals: String(token.decimals || "8"),
      balance: "0",
      detectedFrom: "dex-registry",
      verified: token.verified !== false,
      registry: true,
      official: true,
      networkId: "lqd",
      family: "evm",
      holderAddress: wallet?.address || "",
      logoUrl: token.logo_url || token.logoUrl || "",
      priceUsd: token.price_usd ?? token.priceUsd ?? token.price ?? token.usd_price ?? "",
      priceChange24h: token.price_change_24h ?? token.priceChange24h ?? token.change_24h ?? token.change24h ?? "",
    };
  }

  async function syncDexRegistryIntoWallet() {
    const [cfg, tokens] = await Promise.all([
      dexRegistryConfig().catch(() => null),
      dexRegistryTokens().catch(() => []),
    ]);
    if (cfg?.factory_address) setFactoryAddress(String(cfg.factory_address));
    let registryTokens = (Array.isArray(tokens) ? tokens : []).map(normalizeRegistryToken).filter(Boolean);
    if (registryTokens.length && wallet?.address) {
      const lqdNetwork = NETWORKS.find(n => n.id === "lqd") || currentNetwork;
      const lqdUrls = getRpcCandidatesForFamily(lqdNetwork, "evm");
      registryTokens = await Promise.all(registryTokens.map(async (token) => {
        const balance = await resolveTokenBalanceMultichain(lqdUrls, walletUrl, token.address, wallet.address, "evm").catch(() => token.balance || "0");
        return { ...token, holderAddress: wallet.address, balance };
      }));
    }
    if (registryTokens.length) {
      setWatchlist((prev) => mergeUniqueByKey(prev, registryTokens, "address"));
    }
    return { cfg, tokens: registryTokens };
  }

  async function autoDiscoverTokens(snapshot = {}, addressOverride = "") {
    const activeAddress = addressOverride || wallet?.address;
    if (!activeAddress) return 0;
    const currentAddress = normalizeAddress(activeAddress);
    const recent = Array.isArray(snapshot.recent) ? snapshot.recent : recentTxs;
    const bridgeTokenList = Array.isArray(snapshot.bridgeTokens) ? snapshot.bridgeTokens : bridgeTokens;
    const registryTokenList = Array.isArray(snapshot.registryTokens) ? snapshot.registryTokens : [];
    const factory = snapshot.factory || null;
    const poolTokenCandidates = await discoverPoolTokenCandidates(snapshot.pools);

    const relatedTxs = (recent || []).filter((tx) => {
      const from = normalizeAddress(tx.From || tx.from || tx.sender || "");
      const to = normalizeAddress(tx.To || tx.to || tx.recipient || "");
      const contract = normalizeAddress(tx.Contract || tx.contract || "");
      return from === currentAddress || to === currentAddress || contract === currentAddress;
    });

    const candidates = new Set();
    relatedTxs.forEach((tx) => {
      const to = normalizeAddress(tx.To || tx.to || "");
      const from = normalizeAddress(tx.From || tx.from || "");
      const contract = normalizeAddress(tx.Contract || tx.contract || "");
      if (to && to !== currentAddress) candidates.add(to);
      if (from && from !== currentAddress) candidates.add(from);
      if (contract && contract !== currentAddress) candidates.add(contract);
    });

    const extra = [
      ...tokenCandidateFromValue(bridgeTokenList),
      ...tokenCandidateFromValue(registryTokenList),
      ...tokenCandidateFromValue(factory),
      ...poolTokenCandidates,
      ...tokenCandidateFromValue(deployForm.tokenA),
      ...tokenCandidateFromValue(deployForm.tokenB),
    ].filter((address) => address !== currentAddress);
    extra.forEach(c => candidates.add(c));

    return importDetectedTokens(Array.from(candidates), activeAddress, "auto");
  }

  async function discoverPoolTokenCandidates(poolsSnapshot) {
    const source = poolsSnapshot?.pools || poolsSnapshot;
    const poolAddresses = Array.isArray(source)
      ? source.map((item) => item?.address || item?.pool || item?.pair || item?.pairAddr || item)
      : Object.keys(source || {});
    const candidates = [];
    for (const poolAddress of poolAddresses.slice(0, 50)) {
      const address = normalizeAddress(poolAddress);
      if (!address) continue;
      try {
        const storage = await nodeContractStorage(nodeUrl, address);
        candidates.push(storage?.token0, storage?.token1, storage?.Token0, storage?.Token1);
      } catch {
        // Some liquidity entries may not be DEX pairs; skip silently.
      }
    }
    return tokenCandidateFromValue(candidates);
  }

  function rememberActivity(entry) {
    setActivity((prev) => {
      const combined = [entry, ...prev];
      const seen = new Set();
      return combined.filter((item) => {
        const key = String(item.TxHash || item.tx_hash || item.hash || `${item.type}:${item.Timestamp || item.timestamp || 0}`);
        if (seen.has(key)) return false;
        seen.add(key);
        return true;
      }).slice(0, 100);
    });
  }

  function queueApprovalRequest(request) {
    const item = {
      id: request.id || `${Date.now()}_${Math.random().toString(16).slice(2)}`,
      origin: request.origin || "unknown",
      name: request.name || request.origin || "dApp",
      callback: request.callback || "",
      method: request.method || "wallet_connect",
      status: "pending",
      createdAt: Date.now(),
      type: request.type,
      data: request.data
    };
    setPendingApprovals((prev) => {
      if (prev.some((x) => x.id === item.id)) return prev;
      return [item, ...prev].slice(0, 50);
    });
    setDeepLinkHint(`Request from ${item.name}`);
    setTab("approvals");
  }

  function openFromScan(data) {
    const raw = String(data || "").trim();
    if (!raw) return;

    // ── lqdwallet:// deep link ────────────────────────────────────────────────
    if (/^lqdwallet:\/\//i.test(raw)) {
      try {
        const url = new URL(raw);
        const action = (url.hostname || url.pathname || "").replace(/^\//, "").toLowerCase();
        const params = Object.fromEntries(url.searchParams.entries());
        if (action === "send") {
          if (params.to) setSendForm((prev) => ({ ...prev, to: params.to, amount: params.amount || prev.amount }));
          setSendVisible(true);
          setTab("home");
          setStatus("Send form populated from QR");
          return;
        }
        if (action === "connect") {
          queueApprovalRequest({
            origin: params.origin || url.host || "unknown",
            name: params.name || "dApp",
            callback: params.callback || "",
            method: "wallet_connect",
          });
          return;
        }
        if (action === "token") {
          if (params.address) setTokenImportForm({ address: params.address });
          setTab("tokens");
          return;
        }
        if (action === "receive") {
          setReceiveVisible(true);
          return;
        }
      } catch { /* fallback below */ }
    }

    // ── Payment URI parsing (BIP21, EIP-681, TON, Solana Pay, etc.) ──────────
    // Extracts the address from: bitcoin:addr?amount=x  ethereum:addr  ton://transfer/addr  solana:addr
    let extractedAddr = raw;
    let extractedAmount = "";
    try {
      const uriMatch = raw.match(
        /^(?:bitcoin|litecoin|ethereum|ton|solana|tron|cosmos|near|aptos|sui|starknet):(?:\/\/(?:transfer\/)?)?([^\/?&#]+)/i
      );
      if (uriMatch) {
        extractedAddr = uriMatch[1].trim();
        const qIdx = raw.indexOf("?");
        if (qIdx !== -1) {
          const qs = new URLSearchParams(raw.slice(qIdx + 1));
          extractedAmount = qs.get("amount") || qs.get("value") || "";
        }
      }
    } catch { /* use raw as-is */ }

    // ── Address detection: use current chain family, not just EVM ────────────
    const family = currentNetwork?.family || "evm";
    const isAddr = isLikelyAddressForFamily(extractedAddr, family)
      || isLikelyAddress(extractedAddr); // EVM fallback for universal QRs

    if (isAddr) {
      if (scannerTarget === "native") {
        setSendForm((prev) => ({
          ...prev,
          to: extractedAddr,
          amount: extractedAmount || prev.amount,
        }));
        setTab("home");
        setTimeout(() => setSendVisible(true), 100);
      } else if (scannerTarget === "token") {
        setTokenSendForm((prev) => ({
          ...prev,
          to: extractedAddr,
          amount: extractedAmount || prev.amount,
        }));
      } else if (scannerTarget === "bridge") {
        setBridgeForm((prev) => ({ ...prev, toBsc: extractedAddr, toLqd: extractedAddr }));
      } else {
        // Scanned from standalone scanner tab — open send modal on home tab
        setSendForm((prev) => ({ ...prev, to: extractedAddr, amount: extractedAmount || prev.amount }));
        setTab("home");
        // Small delay so tab navigation settles before modal opens
        setTimeout(() => setSendVisible(true), 100);
      }
      setStatus(`✓ Address scanned`);
      return;
    }

    setStatus("QR scanned but no valid address found for " + (currentNetwork?.name || "this chain"));
  }

  async function respondBrowser(id, ok, result, error = "", method = "") {
    const payload = JSON.stringify({ id, ok, result, error, method }).replace(/\\/g, "\\\\").replace(/'/g, "\\'");
    browserRef.current?.injectJavaScript(`
      if (window.__LQD_MOBILE_PROVIDER_RESPONSE__) {
        window.__LQD_MOBILE_PROVIDER_RESPONSE__(JSON.parse('${payload}'));
      }
      true;
    `);
  }

  function parseDappQuantity(value, fallback = "0") {
    if (value === undefined || value === null || value === "") return String(fallback);
    const raw = String(value).trim();
    if (!raw) return String(fallback);
    if (/^0x[0-9a-fA-F]+$/.test(raw)) return BigInt(raw).toString();
    if (/^\d+$/.test(raw)) return raw;
    throw new Error(`Invalid numeric value: ${raw}`);
  }

  function dappActiveEvmKey() {
    const family = currentNetwork.family || "evm";
    if (family === "harmony") {
      return {
        privateKey: chainKeys?.harmony?.privateKey || chainKeys?.evm?.privateKey || wallet?.privateKey,
        address: chainKeys?.harmony?.address || chainKeys?.evm?.address || wallet?.address,
      };
    }
    if (family === "tron") {
      return {
        privateKey: chainKeys?.tron?.privateKey || wallet?.privateKey,
        address: chainKeys?.tron?.address || wallet?.address,
      };
    }
    if (family === "sei") {
      return {
        privateKey: chainKeys?.sei?.privateKey || wallet?.privateKey,
        address: chainKeys?.sei?.evmAddress || chainKeys?.evm?.address || wallet?.address,
      };
    }
    return {
      privateKey: chainKeys?.evm?.privateKey || wallet?.privateKey,
      address: chainKeys?.evm?.address || wallet?.address,
    };
  }

  function dappActiveChainKey(family = currentNetwork.family || "evm") {
    if (family === "solana") return { privateKey: chainKeys?.solana?.privateKey, address: chainKeys?.solana?.address || activeAddress };
    if (family === "near") return { privateKey: chainKeys?.near?.privateKey, address: watchAddresses[activeNetworkId] || activeAddress, publicKey: chainKeys?.near?.address };
    if (family === "aptos") return { privateKey: chainKeys?.aptos?.privateKey, address: chainKeys?.aptos?.address || activeAddress };
    if (family === "sui") return { privateKey: chainKeys?.sui?.privateKey, address: chainKeys?.sui?.address || activeAddress };
    if (family === "ton") return { privateKey: chainKeys?.ton?.privateKey, address: chainKeys?.ton?.address || activeAddress, rawPubkey: chainKeys?.ton?.rawPubkey };
    if (family === "starknet") return { privateKey: chainKeys?.starknet?.privateKey, address: starknetArgentAddress(chainKeys?.starknet?.publicKey || ""), publicKey: chainKeys?.starknet?.publicKey };
    if (family === "cosmos" || family === "cosmos-testnet") return { privateKey: chainKeys?.cosmos?.privateKey, address: chainKeys?.cosmos?.address || activeAddress };
    if (family === "injective") return { privateKey: chainKeys?.injective?.privateKey, address: chainKeys?.injective?.address || activeAddress };
    if (family === "utxo") return { privateKey: chainKeys?.btc?.privateKey, address: chainKeys?.btc?.address || activeAddress };
    if (family === "litecoin") return { privateKey: chainKeys?.ltc?.privateKey, address: chainKeys?.ltc?.address || activeAddress };
    return dappActiveEvmKey();
  }

  function dappAmountRaw(tx, decimals) {
    const value = tx?.lamports ?? tx?.nanoTon ?? tx?.amountRaw ?? tx?.value;
    if (value !== undefined && value !== null && value !== "") return parseDappQuantity(value, "0");
    if (tx?.amount !== undefined && tx?.amount !== null && tx.amount !== "") {
      return parseUnits(String(tx.amount), decimals || currentNetwork.decimals || 8);
    }
    return "0";
  }

  function dappRecipient(tx) {
    return String(
      tx?.to || tx?.recipient || tx?.receiver || tx?.receiverId || tx?.toAddress ||
      tx?.messages?.[0]?.address || tx?.messages?.[0]?.to || ""
    ).trim();
  }

  function normaliseDappMessage(value) {
    if (value === undefined || value === null) return "";
    if (typeof value === "string") return value;
    if (Array.isArray(value)) return "0x" + value.map((n) => Number(n).toString(16).padStart(2, "0")).join("");
    if (typeof value === "object") {
      if (Array.isArray(value.data)) return normaliseDappMessage(value.data);
      if (Array.isArray(value.message)) return normaliseDappMessage(value.message);
      if (value.message !== undefined) return normaliseDappMessage(value.message);
      if (value.bytes !== undefined) return normaliseDappMessage(value.bytes);
      return JSON.stringify(value);
    }
    return String(value);
  }

  function aptosFullMessage(payload) {
    const safePayload = payload && typeof payload === "object" ? payload : { message: payload };
    const lines = ["APTOS"];
    if (safePayload.address) lines.push(`address: ${safePayload.address}`);
    if (safePayload.application) lines.push(`application: ${safePayload.application}`);
    if (safePayload.chainId) lines.push(`chainId: ${safePayload.chainId}`);
    lines.push(`message: ${normaliseDappMessage(safePayload.message)}`);
    lines.push(`nonce: ${safePayload.nonce || ""}`);
    return { payload: safePayload, fullMessage: lines.join("\n") };
  }

  function assertDappFromMatches(tx, expectedAddress) {
    const from = String(tx?.from || "").trim();
    if (!from || !expectedAddress) return;
    if (from.toLowerCase() !== String(expectedAddress).toLowerCase()) {
      throw new Error(`Request account mismatch. dApp used ${shortAddress(from)}, active wallet is ${shortAddress(expectedAddress)}.`);
    }
  }

  async function approveDappTransaction(item) {
    if (!wallet?.address || !wallet?.privateKey) throw new Error("Unlock wallet first");
    const tx = item.data || {};
    const method = item.method || "";

    if (currentNetwork.id === "lqd" || method === "lqd_contractTx") {
      assertDappFromMatches(tx, wallet.address);
      const baseFee = await nodeBaseFee(nodeUrl).catch(() => 10);
      const baseFeeValue = baseFee?.base_fee || baseFee?.BaseFee || baseFee?.baseFee || baseFee || 10;

      if (method === "lqd_contractTx") {
        const contractAddress = tx.contract_address || tx.contractAddress || tx.address || tx.to;
        const functionName = tx.function || tx.functionName || tx.methodName;
        if (!contractAddress || !functionName) throw new Error("Contract address and function are required");
        const res = await walletContractTx(walletUrl, {
          address: wallet.address,
          contract_address: contractAddress,
          function: functionName,
          args: Array.isArray(tx.args) ? tx.args : [],
          value: parseDappQuantity(tx.value, "0"),
          gas: Number(parseDappQuantity(tx.gas || tx.gasLimit, "200000")),
          gas_price: Number(parseDappQuantity(tx.gas_price || tx.gasPrice, baseFeeValue)),
          private_key: wallet.privateKey,
        });
        const hash = res?.tx_hash || res?.hash || "";
        if (!hash) throw new Error(res?.error || "Contract transaction failed");
        return hash;
      }

      if (!tx.to) throw new Error("Recipient address is required");
      const res = await walletSend(walletUrl, {
        from: wallet.address,
        to: tx.to,
        value: parseDappQuantity(tx.value, "0"),
        data: tx.data || "",
        gas: Number(parseDappQuantity(tx.gas || tx.gasLimit, "21000")),
        gas_price: Number(parseDappQuantity(tx.gas_price || tx.gasPrice, baseFeeValue)),
        private_key: wallet.privateKey,
        node_url: nodeUrl,
      });
      const hash = res?.tx_hash || res?.hash || "";
      if (!hash) throw new Error(res?.error || "Transaction failed");
      return hash;
    }

    const family = currentNetwork.family || "evm";
    if (!(family === "evm" || family === "harmony" || family === "tron" || family === "sei")) {
      return approveDappNonEvmTransaction(item, family);
    }
    const { privateKey, address } = dappActiveEvmKey();
    if (!privateKey || !address) throw new Error("Unlock EVM-compatible wallet key first");
    assertDappFromMatches(tx, address);
    const chainId = currentNetwork.chainId;
    if (!chainId) throw new Error(`chainId not configured for ${currentNetwork.name}`);

    const value = parseDappQuantity(tx.value, "0");
    const data = tx.data || "0x";
    const to = tx.to || "";
    const [nonceRes, gasPriceRes, gasEstimateRes] = await Promise.all([
      postJson(nodeUrl, { jsonrpc: "2.0", id: 1, method: "eth_getTransactionCount", params: [address, "pending"] }).catch(() => null),
      postJson(nodeUrl, { jsonrpc: "2.0", id: 1, method: "eth_gasPrice", params: [] }).catch(() => null),
      tx.gas || tx.gasLimit
        ? Promise.resolve(null)
        : postJson(nodeUrl, {
            jsonrpc: "2.0",
            id: 1,
            method: "eth_estimateGas",
            params: [{ from: address, to: to || undefined, value: "0x" + BigInt(value).toString(16), data }],
          }).catch(() => null),
    ]);
    const nonce = nonceRes?.result ? parseInt(nonceRes.result, 16) : 0;
    const gasPrice = isZeroFeeNetwork(currentNetwork, family)
      ? 0n
      : BigInt(parseDappQuantity(tx.gasPrice || tx.gas_price || gasPriceRes?.result, gasPriceRes?.result ? BigInt(gasPriceRes.result).toString() : String(10e9)));
    const gasLimit = Number(parseDappQuantity(tx.gas || tx.gasLimit || gasEstimateRes?.result, gasEstimateRes?.result ? BigInt(gasEstimateRes.result).toString() : (data && data !== "0x" ? "150000" : "21000")));
    const signedTx = signEip155Tx({
      nonce,
      gasPrice: gasPrice.toString(),
      gasLimit,
      to,
      value,
      data,
      chainId,
    }, privateKey);
    const sendRes = await postJson(nodeUrl, { jsonrpc: "2.0", id: 1, method: "eth_sendRawTransaction", params: [signedTx] });
    const hash = sendRes?.result || "";
    if (!hash || hash === "0x") throw new Error(sendRes?.error?.message || "Broadcast failed");
    return hash;
  }

  async function approveDappNonEvmTransaction(item, family = currentNetwork.family || "evm") {
    const tx = item.data || {};
    const { privateKey, address, publicKey, rawPubkey } = dappActiveChainKey(family);
    if (!privateKey || !address) throw new Error(`Unlock ${currentNetwork.name} wallet key first`);
    assertDappFromMatches(tx, address);
    const recipient = dappRecipient(tx);
    const decimals = currentNetwork.decimals || 8;
    const amount = dappAmountRaw(tx, decimals);

    if (family === "cosmos" || family === "cosmos-testnet" || family === "injective") {
      if (!recipient) throw new Error("Recipient address is required");
      const restBase = (currentNetwork.cosmosRestUrl || getRpcCandidatesForFamily(currentNetwork, family)[0] || nodeUrl).replace(/\/$/, "");
      const accRes = await getJson(`${restBase}/cosmos/auth/v1beta1/accounts/${address}`).catch(() => null);
      const accInfo = accRes?.account;
      const sequence = parseInt(accInfo?.sequence || "0", 10);
      const accountNumber = parseInt(accInfo?.account_number || "0", 10);
      const denom = tx.denom || (family === "injective" ? "inj" : "uatom");
      const txBody = signCosmosTx({
        chainId: currentNetwork.cosmosChainId || currentNetwork.id,
        sequence,
        accountNumber,
        fromAddress: address,
        toAddress: recipient,
        amount,
        denom,
        memo: tx.memo || "",
        gas: Number(parseDappQuantity(tx.gas, "200000")),
      }, privateKey);
      const broadcastRes = await postJson(`${restBase}/cosmos/tx/v1beta1/txs`, { tx: txBody.tx, mode: "BROADCAST_MODE_SYNC" }).catch(() => null);
      const hash = broadcastRes?.tx_response?.txhash || broadcastRes?.txhash || "";
      if (!hash) throw new Error(broadcastRes?.tx_response?.raw_log || "Cosmos broadcast failed");
      return hash;
    }

    if (family === "solana") {
      if (!recipient) throw new Error("Recipient public key is required");
      const blockRes = await postJson(nodeUrl, { jsonrpc: "2.0", id: 1, method: "getLatestBlockhash", params: [{ commitment: "finalized" }] });
      const blockhash = blockRes?.result?.value?.blockhash;
      if (!blockhash) throw new Error("Could not get Solana blockhash");
      const signedTx = signSolanaTransfer({ fromPubkey: address, toPubkey: recipient, lamports: amount, recentBlockhash: blockhash }, privateKey);
      const sendRes = await postJson(nodeUrl, { jsonrpc: "2.0", id: 1, method: "sendTransaction", params: [signedTx, { encoding: "base58" }] });
      const hash = sendRes?.result || "";
      if (!hash) throw new Error(sendRes?.error?.message || "Solana broadcast failed");
      return hash;
    }

    if (family === "near") {
      if (!recipient) throw new Error("Receiver account is required");
      const pubBase58 = cryptoBase58Encode(hexToBytes(publicKey || ""));
      const keyRes = await postJson(nodeUrl, {
        jsonrpc: "2.0", id: 1, method: "query",
        params: { request_type: "view_access_key", finality: "final", account_id: address, public_key: `ed25519:${pubBase58}` },
      }).catch(() => null);
      const nonce = (keyRes?.result?.nonce || 0) + 1;
      const blockRes = await postJson(nodeUrl, { jsonrpc: "2.0", id: 1, method: "block", params: { finality: "final" } }).catch(() => null);
      const blockHash = blockRes?.result?.header?.hash;
      if (!blockHash) throw new Error("Could not fetch NEAR block hash");
      const signedTx = signNearTransfer({ signerId: address, receiverId: recipient, amount, nonce, blockHash }, privateKey);
      const sendRes = await postJson(nodeUrl, { jsonrpc: "2.0", id: 1, method: "broadcast_tx_commit", params: [signedTx] });
      const hash = sendRes?.result?.transaction?.hash || sendRes?.result?.transaction_outcome?.id || "";
      if (!hash && sendRes?.error) throw new Error(sendRes.error.data || sendRes.error.message || "NEAR broadcast failed");
      return hash || "pending";
    }

    if (family === "aptos") {
      const payload = tx.payload || {
        type: "entry_function_payload",
        function: tx.function || "0x1::aptos_coin::transfer",
        type_arguments: tx.type_arguments || [],
        arguments: tx.arguments || [recipient, String(amount)],
      };
      const accRes = await getJson(`${nodeUrl}/accounts/${address}`).catch(() => null);
      const encBody = {
        sender: address,
        sequence_number: tx.sequence_number || accRes?.sequence_number || "0",
        max_gas_amount: String(tx.max_gas_amount || tx.maxGasAmount || "2000"),
        gas_unit_price: String(tx.gas_unit_price || tx.gasUnitPrice || "100"),
        expiration_timestamp_secs: String(tx.expiration_timestamp_secs || Math.floor(Date.now() / 1000) + 120),
        payload,
      };
      const signingHex = await postJson(`${nodeUrl}/transactions/encode_submission`, encBody);
      if (!signingHex || typeof signingHex !== "string") throw new Error("Aptos encode_submission failed");
      const { publicKey: aptPub, signature: aptSig } = signAptosEntry(signingHex, privateKey);
      const submitRes = await postJson(`${nodeUrl}/transactions`, { ...encBody, signature: { type: "ed25519_signature", public_key: aptPub, signature: aptSig } });
      const hash = submitRes?.hash || "";
      if (!hash) throw new Error(submitRes?.message || submitRes?.error_code || "Aptos transaction failed");
      return hash;
    }

    if (family === "sui") {
      if (tx.txBytes || tx.transactionBlock) {
        const txBytes = tx.txBytes || tx.transactionBlock?.txBytes || tx.transactionBlock;
        if (typeof txBytes !== "string") throw new Error("SUI raw transactionBlock must be serialized txBytes");
        const suiSig = signSuiTx(txBytes, privateKey);
        const execRes = await postJson(nodeUrl, {
          jsonrpc: "2.0", id: 1, method: "sui_executeTransactionBlock",
          params: [txBytes, [suiSig], { showEffects: true }, "WaitForLocalExecution"],
        });
        const hash = execRes?.result?.digest || "";
        if (!hash) throw new Error(execRes?.error?.message || "SUI execution failed");
        return hash;
      }
      if (!recipient) throw new Error("Recipient SUI address is required");
      const coinsRes = await postJson(nodeUrl, { jsonrpc: "2.0", id: 1, method: "suix_getCoins", params: [address, "0x2::sui::SUI", null, 1] }).catch(() => null);
      const coinObjectId = coinsRes?.result?.data?.[0]?.coinObjectId;
      if (!coinObjectId) throw new Error("No SUI coin object found in wallet");
      const buildRes = await postJson(nodeUrl, {
        jsonrpc: "2.0", id: 1, method: "unsafe_transferSui",
        params: [address, coinObjectId, String(tx.gasBudget || "10000000"), recipient, String(amount)],
      }).catch(() => null);
      const txBytes = buildRes?.result?.txBytes;
      if (!txBytes) throw new Error(buildRes?.error?.message || "SUI tx build failed");
      const suiSig = signSuiTx(txBytes, privateKey);
      const execRes = await postJson(nodeUrl, { jsonrpc: "2.0", id: 1, method: "sui_executeTransactionBlock", params: [txBytes, [suiSig], { showEffects: true }, "WaitForLocalExecution"] });
      const hash = execRes?.result?.digest || "";
      if (!hash) throw new Error(execRes?.error?.message || "SUI execution failed");
      return hash;
    }

    if (family === "ton") {
      const msg = tx.messages?.[0] || tx;
      const toAddress = dappRecipient(msg);
      if (!toAddress) throw new Error("TON recipient is required");
      const walletAddress = address || rawPubkey;
      if (!walletAddress || !walletAddress.startsWith("EQ")) throw new Error("TON wallet address not available");
      const seqno = await getTonSeqno(nodeUrl, walletAddress);
      const nanoTon = dappAmountRaw({ value: msg.amount ?? msg.value ?? tx.value, amount: msg.displayAmount || tx.amount }, 9);
      const bocBase64 = buildTonTransferBoc({ walletAddress, toAddress, nanoTon, seqno, memo: msg.payload || tx.memo || "", bounce: msg.bounce !== false }, privateKey);
      return broadcastTonBoc(nodeUrl, bocBase64);
    }

    if (family === "starknet") {
      if (!recipient) throw new Error("Starknet recipient is required");
      return broadcastStarknetTransfer({ senderAddress: address, toAddress: recipient, amountWei: String(amount), maxFee: tx.maxFee, nodeUrl }, privateKey);
    }

    throw new Error(`${currentNetwork.name} does not expose a safe browser transaction adapter for this request`);
  }

  function browserConnectResult(method) {
    const address = activeAddress || wallet?.address || "";
    if (method === "solana_connect") {
      const key = dappActiveChainKey("solana");
      return { publicKey: key.address || address };
    }
    if (method === "aptos_connect" || method === "aptos_account") {
      const key = dappActiveChainKey("aptos");
      return { address: key.address || address, publicKey: key.privateKey ? ed25519PublicKey(key.privateKey) : "" };
    }
    if (method === "sui_requestPermissions") return [{ parentCapability: "viewAccount" }, { parentCapability: "suggestTransactions" }];
    if (method === "sui_getAccounts") return [dappActiveChainKey("sui").address || address];
    if (method === "ton_connect" || method === "ton_account") {
      const key = dappActiveChainKey("ton");
      return { address: key.address || address, network: currentNetwork.id, publicKey: key.privateKey ? ed25519PublicKey(key.privateKey, "hex").replace(/^0x/, "") : (key.rawPubkey || "") };
    }
    if (method === "starknet_enable") return [dappActiveChainKey("starknet").address || address];
    if (method === "starknet_account") return { address: dappActiveChainKey("starknet").address || address };
    if (method === "keplr_getKey") {
      const key = dappActiveChainKey(currentNetwork.family);
      return {
        name: "LQD Mobile",
        algo: "secp256k1",
        bech32Address: key.address || address,
        address: key.address || address,
        pubKey: key.privateKey ? secp256k1CompressedPublicKey(key.privateKey, "base64") : "",
        isNanoLedger: false,
      };
    }
    return [address];
  }

  async function approveDappSign(item) {
    if (!wallet?.privateKey) throw new Error("Unlock wallet first");
    const params = Array.isArray(item.data?.params) ? item.data.params : [];
    const method = item.method || "";
    const first = typeof params[0] === "string" ? params[0] : "";
    const second = String(params[1] || "");
    const firstIsAddress = /^0x[0-9a-fA-F]{40}$/.test(first);
    const secondIsAddress = /^0x[0-9a-fA-F]{40}$/.test(second);
    const message = item.data?.message ?? (firstIsAddress && second ? second : params[0]);
    const requestedAddress = secondIsAddress ? second : (firstIsAddress ? first : "");
    const { privateKey, address } = dappActiveEvmKey();
    if (requestedAddress && /^0x[0-9a-fA-F]{40}$/.test(requestedAddress)) {
      assertDappFromMatches({ from: requestedAddress }, address || wallet.address);
    }
    const family = currentNetwork.family || "evm";
    if (method === "keplr_signAmino") {
      const key = dappActiveChainKey(family);
      if (!key.privateKey) throw new Error(`Unlock ${currentNetwork.name} wallet key first`);
      return signCosmosAminoDoc(params[2] || {}, key.privateKey);
    }
    if (method === "aptos_signMessage") {
      const key = dappActiveChainKey("aptos");
      if (!key.privateKey) throw new Error("Unlock Aptos wallet key first");
      const { payload, fullMessage } = aptosFullMessage(params[0] || {});
      const sig = signEd25519Message(fullMessage, key.privateKey);
      return {
        address: key.address,
        application: payload.application || item.origin || "",
        chainId: payload.chainId || currentNetwork.id,
        fullMessage,
        message: normaliseDappMessage(payload.message),
        nonce: payload.nonce || "",
        prefix: "APTOS",
        signature: sig.signature,
      };
    }
    if (method === "sui_signPersonalMessage") {
      const key = dappActiveChainKey("sui");
      if (!key.privateKey) throw new Error("Unlock SUI wallet key first");
      const payload = params[0] || {};
      const bytes = normaliseDappMessage(payload.message || payload.bytes || message);
      const sig = signEd25519Message(bytes, key.privateKey);
      return { bytes, signature: sig.signature, signatureScheme: "ED25519" };
    }
    if (method === "solana_signMessage") {
      const key = dappActiveChainKey("solana");
      if (!key.privateKey) throw new Error("Unlock Solana wallet key first");
      const sig = signEd25519Message(normaliseDappMessage(message), key.privateKey);
      return {
        publicKey: key.address,
        signature: sig.signature,
        signatureBase58: cryptoBase58Encode(hexToBytes(sig.signature)),
      };
    }
    if (method === "ton_signData") {
      const key = dappActiveChainKey("ton");
      if (!key.privateKey) throw new Error("Unlock TON wallet key first");
      const sig = signEd25519Message(normaliseDappMessage(message), key.privateKey);
      return {
        address: key.address,
        publicKey: key.privateKey ? ed25519PublicKey(key.privateKey, "hex").replace(/^0x/, "") : (key.rawPubkey || ""),
        signature: sig.signature,
      };
    }
    if (method === "starknet_signMessage") {
      const key = dappActiveChainKey("starknet");
      if (!key.privateKey) throw new Error("Unlock Starknet wallet key first");
      const digest = CryptoJS.SHA256(normaliseDappMessage(message)).toString();
      return starkSign(digest, key.privateKey);
    }
    if (family === "near") {
      const key = dappActiveChainKey(family);
      if (!key.privateKey) throw new Error(`Unlock ${currentNetwork.name} wallet key first`);
      return signEd25519Message(normaliseDappMessage(message), key.privateKey);
    }
    if (family === "cosmos" || family === "cosmos-testnet" || family === "injective" || family === "utxo" || family === "litecoin") {
      const key = dappActiveChainKey(family);
      if (!key.privateKey) throw new Error(`Unlock ${currentNetwork.name} wallet key first`);
      return signSecp256k1Message(normaliseDappMessage(message), key.privateKey);
    }
    return signPersonalMessage(normaliseDappMessage(message), privateKey || wallet.privateKey);
  }

  async function handleBrowserRequest(req) {
    if (!wallet) return respondBrowser(req.id, false, null, "Wallet locked", req.method);
    const { method, params, origin, name } = req;
    try {
      if (!isAllowedBrowserOrigin(origin)) {
        return respondBrowser(req.id, false, null, "Untrusted or insecure dApp origin", method);
      }

      if (method === "eth_chainId" || method === "lqd_chainId") {
        return respondBrowser(req.id, true, currentNetwork?.chainId || "0x8b", "", method);
      }

      if (method === "net_version") {
        const parsed = parseInt(currentNetwork?.chainId || "0x8b", 16);
        return respondBrowser(req.id, true, Number.isFinite(parsed) ? String(parsed) : "139", "", method);
      }

      if (method === "eth_accounts" || method === "lqd_accounts") {
        return respondBrowser(req.id, true, trustedOrigins.includes(origin) ? [activeAddress || wallet.address] : [], "", method);
      }

      if (method === "aptos_account" || method === "sui_getAccounts" || method === "ton_account" || method === "starknet_account" || method === "keplr_getKey") {
        return respondBrowser(req.id, true, trustedOrigins.includes(origin) ? browserConnectResult(method) : (method === "sui_getAccounts" ? [] : null), "", method);
      }

      if (method === "wallet_getPermissions") {
        return respondBrowser(req.id, true, trustedOrigins.includes(origin) ? [{ parentCapability: "eth_accounts" }] : [], "", method);
      }

      if (method === "lqd_supportedChains") {
        return respondBrowser(req.id, true, NETWORKS.map((n) => ({ id: n.id, name: n.name, family: n.family, chainId: n.chainId || n.cosmosChainId || n.id })), "", method);
      }

      const connectMethods = [
        "lqd_requestAccounts", "eth_requestAccounts", "lqd_connect", "wallet_requestPermissions",
        "solana_connect", "aptos_connect", "sui_requestPermissions", "ton_connect", "starknet_enable", "keplr_enable",
      ];
      if (connectMethods.includes(method)) {
        if (trustedOrigins.includes(origin)) {
          return respondBrowser(req.id, true, browserConnectResult(method), "", method);
        }
        queueApprovalRequest({
          id: req.id,
          type: "connect",
          origin,
          name,
          method,
          data: { message: "This dApp wants to see your wallet address and activity." }
        });
        setTab("approvals");
        return;
      }

      const transactionMethods = [
        "lqd_sendTransaction", "eth_sendTransaction", "lqd_contractTx",
        "solana_signAndSendTransaction", "aptos_signAndSubmitTransaction", "sui_signAndExecuteTransactionBlock",
        "ton_sendTransaction", "starknet_execute", "keplr_sendTx",
      ];
      if (transactionMethods.includes(method)) {
        const tx = params[0] || {};
        queueApprovalRequest({
          id: req.id,
          type: "transaction",
          origin,
          name,
          method,
          data: tx
        });
        setTab("approvals");
        return;
      }

      const signMethods = [
        "lqd_sign", "personal_sign", "solana_signMessage", "aptos_signMessage", "sui_signPersonalMessage",
        "ton_signData", "starknet_signMessage", "keplr_signAmino",
      ];
      if (signMethods.includes(method)) {
        queueApprovalRequest({
          id: req.id,
          type: "sign",
          origin,
          name,
          method,
          data: { message: params[0] || params[1] || "", params }
        });
        setTab("approvals");
        return;
      }

      // Default: Method not supported
      respondBrowser(req.id, false, null, `Method not supported: ${method}`);
    } catch (e) {
      respondBrowser(req.id, false, null, e.message || "Internal error");
      setStatusModal({
        visible: true,
        title: "Browser Request Error",
        message: `Error handling dApp request: ${e.message}\nMethod: ${method}`,
        type: "error",
        hash: ""
      });
    }
  }

  useEffect(() => {
    let alive = true;
    (async () => {
      try {
        await runWalletMigrations();
        const audit = await runSecureStorageAudit({ repair: true }).catch((error) => ({
          ok: false,
          warnings: [error?.message || "Secure storage audit failed"],
          checkedAt: Date.now(),
        }));
        if (alive) setSecurityAudit(audit);
        if (audit && audit.ok === false) {
          recordError("secure_storage_audit", new Error((audit.warnings || []).join("; ") || "Secure storage audit failed"), {
            repairedKeys: audit.repairedKeys || [],
            leakedKeys: audit.leakedKeys || [],
          }).catch(() => {});
        }
        const [vault, savedNetworks, savedNetworkId, savedEndpoints, savedWatchlist, savedActivity, savedFactory, savedBridgeChainId, savedSettings, savedApprovals, savedTrustedOrigins, savedWatchAddresses, savedHiddenTokens, savedRemovedTokens, savedLegalRiskAccepted, savedTrackedTxs, savedNfts] = await Promise.all([
          loadJSON(STORAGE_KEYS.vault, null),
          loadJSON(STORAGE_KEYS.networks, null),
          loadJSON(STORAGE_KEYS.activeNetworkId, null),
          loadJSON(STORAGE_KEYS.endpoints, null),
          loadJSON(STORAGE_KEYS.watchlist, []),
          loadJSON(STORAGE_KEYS.activity, []),
          loadJSON(STORAGE_KEYS.factory, ""),
          loadJSON(STORAGE_KEYS.bridgeChainId, "bsc-testnet"),
          loadJSON(STORAGE_KEYS.settings, {}),
          loadJSON(STORAGE_KEYS.approvals, []),
          loadJSON(STORAGE_KEYS.trustedOrigins, []),
          loadJSON(STORAGE_KEYS.watchAddresses, {}),
          loadJSON(STORAGE_KEYS.hiddenTokens, []),
          loadJSON(STORAGE_KEYS.removedTokens, []),
          loadJSON(STORAGE_KEYS.legalRiskAccepted, false),
          loadJSON(STORAGE_KEYS.pendingTransactions, []),
          loadJSON(STORAGE_KEYS.nfts, []),
        ]);

        if (!alive) return;
        if (savedNetworks?.length) setNetworks(savedNetworks);
        if (savedNetworkId) setActiveNetworkId(migrateNetworkId(savedNetworkId));
        if (savedEndpoints) {
          setEndpoints((prev) => ({
            ...prev,
            nodeUrl: migrateLocalEndpoint(savedEndpoints.nodeUrl, prev.nodeUrl),
            walletUrl: migrateLocalEndpoint(savedEndpoints.walletUrl, prev.walletUrl),
            aggregatorUrl: migrateLocalEndpoint(savedEndpoints.aggregatorUrl, prev.aggregatorUrl),
            explorerUrl: migrateLocalEndpoint(savedEndpoints.explorerUrl, prev.explorerUrl),
          }));
        }
        if (savedWatchlist) setWatchlist(savedWatchlist);
        if (savedActivity) setActivity(savedActivity);
        if (savedFactory) setFactoryAddress(savedFactory);
        if (savedBridgeChainId) setBridgeChainId(String(savedBridgeChainId));
        if (savedSettings && typeof savedSettings === "object") {
          setSettingsAutoRefresh(savedSettings.autoRefresh !== false);
          setBiometricEnabled(savedSettings.biometricEnabled !== false);
          setSettingsTelemetryEnabled(Boolean(savedSettings.telemetryEnabled));
          setSettingsTxTrackingEnabled(savedSettings.txTrackingEnabled !== false);
          setSettingsBackgroundTxTrackingEnabled(savedSettings.backgroundTxTrackingEnabled !== false);
          setSettingsStorageAuditEnabled(savedSettings.storageAuditEnabled !== false);
        }
        if (Array.isArray(savedApprovals)) setPendingApprovals(savedApprovals);
        if (Array.isArray(savedTrustedOrigins)) setTrustedOrigins(savedTrustedOrigins);
        if (savedWatchAddresses && typeof savedWatchAddresses === "object") setWatchAddresses(savedWatchAddresses);
        if (Array.isArray(savedHiddenTokens)) setHiddenTokens(savedHiddenTokens);
        if (Array.isArray(savedRemovedTokens)) setRemovedTokens(savedRemovedTokens);
        setLegalRiskAccepted(Boolean(savedLegalRiskAccepted));
        if (Array.isArray(savedTrackedTxs)) setTrackedTxs(savedTrackedTxs);
        if (Array.isArray(savedNfts)) setNfts(savedNfts);
        setVaultRecord(vault || null);
      } catch (e) {
        recordError("boot", e).catch(() => {});
        setStatus(e.message || "Failed to load wallet state");
      } finally {
        if (alive) setBooting(false);
      }
    })();
    return () => { alive = false; };
  }, []);

  useEffect(() => {
    saveJSON(STORAGE_KEYS.networks, networks).catch(() => { });
  }, [networks]);

  useEffect(() => {
    saveJSON(STORAGE_KEYS.activeNetworkId, activeNetworkId).catch(() => { });
  }, [activeNetworkId]);

  useEffect(() => {
    if (wallet && activeAddress) {
      refreshWalletSnapshot(activeAddress);
    }
  }, [activeNetworkId, activeAddress]);

  useEffect(() => {
    saveJSON(STORAGE_KEYS.endpoints, endpoints).catch(() => { });
  }, [endpoints]);

  useEffect(() => {
    saveJSON(STORAGE_KEYS.watchlist, watchlist).catch(() => { });
  }, [watchlist]);

  useEffect(() => {
    saveJSON(STORAGE_KEYS.hiddenTokens, hiddenTokens).catch(() => { });
  }, [hiddenTokens]);

  useEffect(() => {
    saveJSON(STORAGE_KEYS.removedTokens, removedTokens).catch(() => { });
  }, [removedTokens]);

  useEffect(() => {
    saveJSON(STORAGE_KEYS.nfts, nfts).catch(() => { });
  }, [nfts]);

  useEffect(() => {
    saveJSON(STORAGE_KEYS.activity, activity).catch(() => { });
  }, [activity]);

  useEffect(() => {
    saveJSON(STORAGE_KEYS.factory, factoryAddress).catch(() => { });
  }, [factoryAddress]);

  useEffect(() => {
    saveJSON(STORAGE_KEYS.bridgeChainId, bridgeChainId).catch(() => { });
  }, [bridgeChainId]);

  useEffect(() => {
    saveJSON(STORAGE_KEYS.settings, {
      autoRefresh: settingsAutoRefresh,
      biometricEnabled,
      telemetryEnabled: settingsTelemetryEnabled,
      txTrackingEnabled: settingsTxTrackingEnabled,
      backgroundTxTrackingEnabled: settingsBackgroundTxTrackingEnabled,
      storageAuditEnabled: settingsStorageAuditEnabled,
    }).catch(() => { });
  }, [settingsAutoRefresh, biometricEnabled, settingsTelemetryEnabled, settingsTxTrackingEnabled, settingsBackgroundTxTrackingEnabled, settingsStorageAuditEnabled]);

  useEffect(() => {
    saveJSON(STORAGE_KEYS.legalRiskAccepted, legalRiskAccepted).catch(() => { });
  }, [legalRiskAccepted]);

  useEffect(() => {
    saveJSON(STORAGE_KEYS.pendingTransactions, trackedTxs).catch(() => { });
  }, [trackedTxs]);

  useEffect(() => {
    saveJSON(STORAGE_KEYS.approvals, pendingApprovals).catch(() => { });
  }, [pendingApprovals]);

  useEffect(() => {
    saveJSON(STORAGE_KEYS.trustedOrigins, trustedOrigins).catch(() => { });
  }, [trustedOrigins]);

  useEffect(() => {
    saveJSON(STORAGE_KEYS.watchAddresses, watchAddresses).catch(() => { });
  }, [watchAddresses]);

  useEffect(() => {
    let mounted = true;
    (async () => {
      try {
        const hasHardware = await LocalAuthentication.hasHardwareAsync();
        const enrolled = hasHardware ? await LocalAuthentication.isEnrolledAsync() : false;
        if (mounted) setBiometricAvailable(Boolean(hasHardware && enrolled));
      } catch {
        if (mounted) setBiometricAvailable(false);
      }
    })();
    return () => {
      mounted = false;
    };
  }, []);

  useEffect(() => {
    return installGlobalErrorReporter({ app: "lqd-mobile-wallet", version: "1.2.0" });
  }, []);

  useEffect(() => {
    if (!settingsTxTrackingEnabled || !trackedTxs.some((tx) => tx.status === "pending")) return;
    let cancelled = false;
    let appState = AppState.currentState || "active";

    const refresh = async () => {
      if (!settingsBackgroundTxTrackingEnabled && appState !== "active") return;
      try {
        const enriched = trackedTxs.map((tx) => {
          if (tx.nodeUrl) return tx;
          const network = NETWORKS.find((n) => n.id === tx.networkId) || currentNetwork;
          const family = tx.family || network?.family || "evm";
          return { ...tx, nodeUrl: getRpcCandidatesForFamily(network, family)[0] || nodeUrl };
        });
        const next = await refreshTrackedTransactions(enriched, { getJson, postJson });
        if (cancelled) return;
        const confirmed = next.find((tx) => tx.status === "confirmed" && trackedTxs.find((old) => old.id === tx.id && old.status === "pending"));
        const failed = next.find((tx) => tx.status === "failed" && trackedTxs.find((old) => old.id === tx.id && old.status === "pending"));
        const dropped = next.find((tx) => tx.status === "dropped" && trackedTxs.find((old) => old.id === tx.id && old.status === "pending"));
        setTrackedTxs(next);
        if (confirmed) showToast(`${confirmed.symbol || "Transaction"} confirmed`, "success");
        if (failed) showToast(`${failed.symbol || "Transaction"} failed`, "error");
        if (dropped) showToast(`${dropped.symbol || "Transaction"} tracking expired`, "error");
      } catch (error) {
        recordError("tx_tracking", error, { network: activeNetworkId }).catch(() => {});
      }
    };

    const subscription = AppState.addEventListener("change", (state) => {
      appState = state;
      if (state === "active") refresh();
    });
    const timer = setInterval(refresh, appState === "active" ? 30000 : 90000);
    refresh();
    return () => {
      cancelled = true;
      clearInterval(timer);
      subscription?.remove?.();
    };
  }, [settingsTxTrackingEnabled, settingsBackgroundTxTrackingEnabled, trackedTxs, activeNetworkId, currentNetwork, nodeUrl]);

  async function approveRequest(item) {
    setPendingApprovals((prev) => prev.filter((x) => x.id !== item.id));
    try {
      if (item.type === "connect") {
        setTrustedOrigins((prev) => (prev.includes(item.origin) ? prev : [...prev, item.origin]));
        setStatus(`Approved ${item.name}`);
        if (item.origin) {
          respondBrowser(item.id, true, browserConnectResult(item.method), "", item.method);
        }
        recordTelemetry("dapp_connect_approved", { origin: item.origin, network: activeNetworkId }).catch(() => {});
      } else if (item.type === "transaction") {
        setBusy(true);
        setBusyAction("dappTransaction");
        setProcessingMessage("Review approved. Broadcasting dApp transaction...");
        setTrustedOrigins((prev) => (prev.includes(item.origin) ? prev : [...prev, item.origin]));
        const hash = await approveDappTransaction(item);
        respondBrowser(item.id, true, hash, "", item.method);
        setStatus(`Transaction approved for ${item.name}`);
        showToast(`dApp transaction sent: ${shortAddress(hash, 8, 6)}`, "success");
        addTrackedTx({
          hash,
          type: "dapp_transaction",
          symbol: currentNetwork.symbol,
          to: item.data?.to || item.data?.contract_address || item.data?.contractAddress || "",
          family: currentNetwork.id === "lqd" ? "lqd" : currentNetwork.family,
        });
        rememberActivity({
          type: "dapp",
          From: activeAddress,
          To: item.data?.to || item.data?.contract_address || item.data?.contractAddress || "",
          TxHash: hash,
          Timestamp: Math.floor(Date.now() / 1000),
          Status: "success",
          Value: parseDappQuantity(item.data?.value, "0"),
        });
        recordTelemetry("dapp_transaction_approved", { origin: item.origin, method: item.method, network: activeNetworkId, hash }).catch(() => {});
      } else if (item.type === "sign") {
        setBusy(true);
        setBusyAction("dappSign");
        setProcessingMessage("Signing dApp message...");
        setTrustedOrigins((prev) => (prev.includes(item.origin) ? prev : [...prev, item.origin]));
        const signature = await approveDappSign(item);
        respondBrowser(item.id, true, signature, "", item.method);
        setStatus(`Signature approved for ${item.name}`);
        showToast("dApp message signed", "success");
        recordTelemetry("dapp_sign_approved", { origin: item.origin, method: item.method, network: activeNetworkId }).catch(() => {});
      }
    } catch (error) {
      respondBrowser(item.id, false, null, error.message || "Request failed", item.method);
      setStatusModal({
        visible: true,
        title: "dApp Request Failed",
        message: error.message || "Request failed",
        type: "error",
        hash: "",
      });
      recordError("dapp_approval", error, { origin: item.origin, type: item.type, method: item.method, network: activeNetworkId }).catch(() => {});
    } finally {
      setBusy(false);
      setBusyAction("");
      setProcessingMessage("");
      setTab("browser");
    }
  }

  function builtinInitArgs() {
    const fields = QUICK_ARGS[deployForm.template] || [];
    return fields.map((f) => String(deployForm[f.key] || "").trim());
  }

  function validateBuiltinDeployForm(form = deployForm) {
    if (form.template === "dex_swap" && (!isLikelyAddress(form.tokenA) || !isLikelyAddress(form.tokenB))) {
      return "DEX Pair / Pool needs valid Token A and Token B contract addresses";
    }
    if (form.template === "lqd20" && (!/^\d+$/.test(form.q_supply.trim()) || BigInt(form.q_supply.trim() || "0") <= 0n)) {
      return "LQD20 supply must be greater than 0";
    }
    if (form.template === "bridge_token" && Number(form.q_bdec || 0) < 0) {
      return "Bridge token decimals must be valid";
    }
    return "";
  }

  function renderBuiltinTemplateFields() {
    const fields = QUICK_ARGS[deployForm.template] || [];
    if (!fields.length) {
      return <Text style={styles.helperText}>This template does not require initial arguments.</Text>;
    }
    return (
      <View style={styles.sectionGapSmall}>
        {fields.map((f) => (
          <Field
            key={f.key}
            label={f.label}
            value={deployForm[f.key] || ""}
            onChangeText={(v) => setDeployForm((p) => ({ ...p, [f.key]: v }))}
            placeholder={f.ph}
          />
        ))}
      </View>
    );
  }

  async function handleBrowserProviderMessage(event) {
    let request = null;
    try {
      request = JSON.parse(event?.nativeEvent?.data || "{}");
    } catch {
      return;
    }
    if (request?.source !== "lqd-mobile-provider" || !request.id) return;
    handleBrowserRequest(request);
  }

  function rejectRequest(item) {
    setPendingApprovals((prev) => prev.filter((x) => x.id !== item.id));
    setStatus(`Rejected ${item.name}`);
    if (item.origin) {
      respondBrowser(item.id, false, null, "User rejected request", item.method);
    }
    recordTelemetry("dapp_request_rejected", { origin: item.origin, type: item.type, network: activeNetworkId }).catch(() => {});
    setTab("browser");
  }

  async function scanWithCamera(target) {
    setScannerTarget(target);
    if (!cameraPermission?.granted) {
      const perm = await requestCameraPermission();
      if (!perm.granted) {
        setStatus("Camera permission is required for QR scan");
        return;
      }
    }
    setScannerVisible(true);
  }

  async function unlockWallet() {
    if (!vaultRecord?.cipher) {
      setStatus("No wallet vault found");
      return;
    }
    if (!unlockPassword) {
      setStatus("Enter your wallet password");
      return;
    }
    if (unlockInProgress.current) return;
    unlockInProgress.current = true;
    setBusy(true);
    setBusyAction("unlockPassword");
    try {
      const vault = decryptVault(vaultRecord.cipher, unlockPassword);
      // Try to recover private key from hardware store
      if (SecureStore && typeof SecureStore.getItemAsync === "function") {
        try {
          const pk = await SecureStore.getItemAsync(`pk_${vault.address}`);
          if (pk) vault.privateKey = pk;
        } catch { }
      }
      setWallet(vault);
      setWalletVisible(true);
      setStatus(`Unlocked ${shortAddress(vault.address)}`);
      applyChainKeys(vault.mnemonic).catch(() => { });
      await refreshWalletSnapshot(vault.address);
    } catch (e) {
      setStatus(e.message || "Failed to unlock");
    } finally {
      unlockInProgress.current = false;
      setBusy(false);
      setBusyAction("");
    }
  }

  async function unlockWithBiometrics() {
    if (!vaultRecord?.cipher) {
      setStatus("No wallet vault found");
      return;
    }
    if (!biometricAvailable) {
      setStatus("Biometrics not available on this device");
      return;
    }
    if (unlockInProgress.current) return;
    unlockInProgress.current = true;
    setBusy(true);
    setBusyAction("unlockBiometric");
    try {
      const biometricRaw = await loadString(STORAGE_KEYS.biometricVault, "", { requireAuthentication: true });
      if (!biometricRaw) {
        throw new Error("Biometric vault is not saved yet. Unlock with password once, then enable biometrics.");
      }
      const vault = JSON.parse(biometricRaw);
      // Try to recover private key from hardware store
      if (SecureStore && typeof SecureStore.getItemAsync === "function") {
        try {
          const pk = await SecureStore.getItemAsync(`pk_${vault.address}`);
          if (pk) vault.privateKey = pk;
        } catch { }
      }
      setWallet(vault);
      setWalletVisible(true);
      setStatus(`Unlocked ${shortAddress(vault.address)} with biometrics`);
      applyChainKeys(vault.mnemonic).catch(() => { });
      await refreshWalletSnapshot(vault.address);
    } catch (e) {
      setStatus(e.message || "Biometric unlock failed");
    } finally {
      unlockInProgress.current = false;
      setBusy(false);
      setBusyAction("");
    }
  }

  async function createWalletAction() {
    const password = createForm.password.trim();
    if (!password || password !== createForm.confirm.trim()) {
      setStatus("Passwords do not match");
      Alert.alert("Create wallet failed", "Passwords do not match.");
      return;
    }
    Keyboard.dismiss();
    setBusy(true);
    setBusyAction("createWallet");
    try {
      const res = await walletCreate(walletUrl, password);
      const vault = {
        address: res.address,
        privateKey: res.private_key,
        mnemonic: res.mnemonic || "",
      };
      await persistWalletVault(vault, password);
      setWallet(vault);
      setWalletVisible(true);
      setShowMnemonic(true);
      setCreateForm(initialCreateForm);
      setStatus(`Created wallet ${shortAddress(vault.address)}`);
      Alert.alert("Wallet created", `Address: ${vault.address}`);
      applyChainKeys(vault.mnemonic).catch(() => { });
      await refreshWalletSnapshot(vault.address);
    } catch (e) {
      const message = e.message || "Failed to create wallet";
      setStatus(message);
      Alert.alert("Create wallet failed", message);
    } finally {
      setBusy(false);
      setBusyAction("");
    }
  }

  async function importMnemonicAction() {
    const mnemonic = importMnemonicForm.mnemonic.trim();
    const password = importMnemonicForm.password.trim();
    if (!mnemonic || !password) {
      setStatus("Fill mnemonic and password");
      Alert.alert("Import failed", "Fill mnemonic and password.");
      return;
    }
    Keyboard.dismiss();
    setBusy(true);
    setBusyAction("importMnemonic");
    try {
      const res = await walletImportMnemonic(walletUrl, mnemonic, password);
      const vault = {
        address: res.address,
        privateKey: res.private_key,
        mnemonic,
      };
      await persistWalletVault(vault, password);
      setWallet(vault);
      setWalletVisible(true);
      setShowMnemonic(true);
      setImportMnemonicForm(initialImportMnemonicForm);
      setStatus(`Imported wallet ${shortAddress(vault.address)}`);
      Alert.alert("Wallet imported", `Address: ${vault.address}`);
      applyChainKeys(vault.mnemonic).catch(() => { });
      await refreshWalletSnapshot(vault.address);
    } catch (e) {
      const message = e.message || "Failed to import mnemonic";
      setStatus(message);
      Alert.alert("Import failed", message);
    } finally {
      setBusy(false);
      setBusyAction("");
    }
  }

  async function importPrivateKeyAction() {
    const privateKey = importPkForm.privateKey.trim();
    const password = importPkForm.password.trim();
    if (!privateKey || !password) {
      setStatus("Fill private key and password");
      Alert.alert("Import failed", "Fill private key and password.");
      return;
    }
    Keyboard.dismiss();
    setBusy(true);
    setBusyAction("importPrivateKey");
    try {
      const res = await walletImportPrivateKey(walletUrl, privateKey);
      const vault = {
        address: res.address,
        privateKey,
        mnemonic: "",
      };
      await persistWalletVault(vault, password);
      setWallet(vault);
      setWalletVisible(true);
      setImportPkForm(initialImportPkForm);
      setStatus(`Imported private key wallet ${shortAddress(vault.address)}`);
      Alert.alert("Wallet imported", `Address: ${vault.address}`);
      // No mnemonic for private key import — only EVM-family chains work
      await refreshWalletSnapshot(vault.address);
    } catch (e) {
      const message = e.message || "Failed to import private key";
      setStatus(message);
      Alert.alert("Import failed", message);
    } finally {
      setBusy(false);
      setBusyAction("");
    }
  }

  async function lockWalletAction() {
    setWallet(null);
    setChainKeys(null);
    setMultiWallets({});
    setWalletVisible(false);
    setUnlockPassword("");
    setStatus("Wallet locked");
  }

  async function copyAddress() {
    if (!activeAddress) return;
    await Clipboard.setStringAsync(activeAddress);
    setStatus("Address copied");
  }

  async function pasteClipboardTo(setter) {
    const value = await Clipboard.getStringAsync();
    setter(value || "");
  }

  async function refreshNativeOnly(addressOverride = "") {
    const rawAddr = addressOverride || wallet?.address;
    if (!rawAddr) return;
    const activeAddress = rawAddr.trim();
    const native = await walletBalance(nodeUrl, activeAddress).catch(() => null);
    if (native) {
      let val = "0";
      if (native.balance !== undefined && native.balance !== null) val = String(native.balance);
      else if (native.Balance !== undefined && native.Balance !== null) val = String(native.Balance);
      else if (native.amount !== undefined && native.amount !== null) val = String(native.amount);
      setNativeBalance(val);
    }
  }

  function walletHasGasBalance() {
    try {
      return BigInt(String(nativeBalance || "0")) > 0n;
    } catch {
      return false;
    }
  }

  async function claimFaucetAction() {
    if (!wallet?.address) {
      setStatus("Unlock wallet first");
      return;
    }
    setBusy(true);
    setBusyAction("faucet");
    try {
      const targetAddress = activeAddress || wallet.address;
      const res = await nodeFaucet(nodeUrl, targetAddress);
      const credited = res?.credited || res?.amount || "";
      setStatus(credited ? `Faucet credited ${formatUnits(credited, 8, 6)} LQD` : "Faucet credited");

      // Directly fetch balance — bypasses isRefreshing guard so we always get fresh value
      const fetchBalance = async () => {
        try {
          const native = await walletBalance(nodeUrl, targetAddress);
          if (native) {
            const val = String(native.balance ?? native.Balance ?? native.amount ?? "0");
            if (val !== "0") setNativeBalance(val);
          }
        } catch { /* ignore */ }
      };
      await fetchBalance();
      setTimeout(fetchBalance, 2000);
      setTimeout(() => refreshWalletSnapshot(), 3000);
    } catch (e) {
      setStatus(e.message || "Faucet claim failed");
    } finally {
      setBusy(false);
      setBusyAction("");
    }
  }


  async function refreshTokenBalances(list, address) {
    if (!address || !list?.length) return;
    try {
      const results = await Promise.all(
        list.map(async (t) => {
          try {
            const holder = t.holderAddress || address;
            const fam = t.family || "evm";
            const tokenNet = NETWORKS.find(n => n.id === (t.networkId || activeNetworkId)) || currentNetwork;
            const tokenUrls = getRpcCandidatesForFamily(tokenNet, fam);
            const tokenUrl = tokenUrls[0] || nodeUrl;

            // TON Jetton: query per-user balance instead of total supply
            if (fam === "ton") {
              const tonAddr = chainKeys?.ton?.address || chainKeys?.ton?.rawPubkey || holder;
              const balance = await getTonJettonBalance(tokenUrl, tonAddr, t.address);
              return { ...t, balance };
            }

            // Starknet: use correct address
            if (fam === "starknet" && chainKeys?.starknet?.publicKey) {
              const sAddr = starknetArgentAddress(chainKeys.starknet.publicKey);
              const balance = await resolveTokenBalanceMultichain(tokenUrls, walletUrl, t.address, sAddr, fam);
              return { ...t, balance };
            }

            const balance = await resolveTokenBalanceMultichain(tokenUrls, walletUrl, t.address, holder, fam);
            return { ...t, balance };
          } catch {
            return t;
          }
        })
      );
      // Merge balances into existing state — never replace, to avoid losing tokens added concurrently
      setWatchlist(prev => {
        const balMap = new Map(results.map(r => [tokenKey(r), r.balance]));
        return prev.map(t => {
          const key = tokenKey(t);
          return balMap.has(key) ? { ...t, balance: balMap.get(key) } : t;
        });
      });
    } catch { }
  }

  async function sendAction() {
    if (!wallet?.address || !wallet?.privateKey) {
      showToast("Unlock wallet first", "error"); return;
    }
    const family = currentNetwork.family || "evm";
    const recipient = sendForm.to.trim();

    if (!isLikelyAddressForFamily(recipient, family)) {
      showToast(`Enter a valid ${currentNetwork.name} address (e.g. ${SEND_ADDR_PLACEHOLDER[family] || "..."})`, "error");
      return;
    }
    const decimals = currentNetwork.decimals || 8;
    const amount = parseUnits(sendForm.amount, decimals);
    if (BigInt(amount) <= 0n) {
      showToast("Enter a valid amount", "error"); return;
    }

    setBusy(true);
    setBusyAction("sendNative");
    setProcessingMessage("Broadcasting Transaction...");
    try {
      let hash = "";

      // ── LQD: use WalletServer (custom LQD signing) ────────────────────────
      if (currentNetwork.id === "lqd") {
        const baseFee = await nodeBaseFee(nodeUrl).catch(() => 10);
        const baseFeeValue = baseFee?.base_fee || baseFee?.BaseFee || baseFee?.baseFee || baseFee || 10;
        const gasLimit = Number(nativeFeeInfo.gasLimit || 21000);
        const gasPrice = Number(nativeFeeInfo.gasPrice || baseFeeValue || 10);
        const res = await walletSend(walletUrl, {
          from: wallet.address,
          to: recipient,
          value: amount,
          data: "",
          gas: gasLimit,
          gas_price: gasPrice,
          private_key: wallet.privateKey,
          node_url: nodeUrl,
        });
        hash = res?.tx_hash || res?.hash || "";
        if (!hash) throw new Error(res?.error || "Transaction failed");

        // ── EVM / Harmony / Tron / SEI-EVM: client-side EIP-155 signing ───────
      } else if (family === "evm" || family === "harmony" || family === "tron" || family === "sei") {
        let privKey, fromAddr;
        if (family === "harmony") {
          privKey = chainKeys?.harmony?.privateKey || chainKeys?.evm?.privateKey || wallet.privateKey;
          fromAddr = chainKeys?.harmony?.address || chainKeys?.evm?.address || wallet.address;
        } else if (family === "tron") {
          privKey = chainKeys?.tron?.privateKey || wallet.privateKey;
          fromAddr = chainKeys?.tron?.address || wallet.address; // 0x for RPC
        } else if (family === "sei") {
          privKey = chainKeys?.sei?.privateKey || wallet.privateKey;
          fromAddr = chainKeys?.sei?.evmAddress || chainKeys?.evm?.address || wallet.address;
        } else {
          privKey = chainKeys?.evm?.privateKey || wallet.privateKey;
          fromAddr = chainKeys?.evm?.address || wallet.address;
        }
        const chainId = currentNetwork.chainId;
        if (!chainId) throw new Error(`chainId not configured for ${currentNetwork.name}`);

        const [noncRes, gasPriceRes] = await Promise.all([
          postJson(nodeUrl, { jsonrpc: "2.0", id: 1, method: "eth_getTransactionCount", params: [fromAddr, "pending"] }).catch(() => null),
          postJson(nodeUrl, { jsonrpc: "2.0", id: 1, method: "eth_gasPrice", params: [] }).catch(() => null),
        ]);
        const nonce = noncRes?.result ? parseInt(noncRes.result, 16) : 0;
        const gasPrice = isZeroFeeNetwork(currentNetwork, family)
          ? 0n
          : (nativeFeeInfo.gasPrice ? BigInt(nativeFeeInfo.gasPrice) : (gasPriceRes?.result ? BigInt(gasPriceRes.result) : BigInt(10e9)));
        const gasLimit = nativeFeeInfo.gasLimit ? Number(nativeFeeInfo.gasLimit) : 21000;

        const signedTx = signEip155Tx({
          nonce,
          gasPrice: gasPrice.toString(),
          gasLimit,
          to: recipient,
          value: amount,
          data: "",
          chainId,
        }, privKey);

        const sendRes = await postJson(nodeUrl, {
          jsonrpc: "2.0", id: 1, method: "eth_sendRawTransaction", params: [signedTx],
        });
        hash = sendRes?.result || "";
        if (!hash || hash === "0x") throw new Error(sendRes?.error?.message || "Broadcast failed");

        // ── Cosmos / Injective: amino + REST broadcast ─────────────────────────
      } else if (family === "cosmos" || family === "injective") {
        const privKey = family === "cosmos"
          ? (chainKeys?.cosmos?.privateKey || wallet.privateKey)
          : (chainKeys?.injective?.privateKey || wallet.privateKey);
        const fromAddr = family === "cosmos"
          ? (chainKeys?.cosmos?.address || activeAddress)
          : (chainKeys?.injective?.address || activeAddress);
        const cosmosChainId = currentNetwork.cosmosChainId || currentNetwork.id;
        const denom = family === "cosmos" ? "uatom" : "inj";

        // Fetch account sequence + number (SEI: use cosmosRestUrl; injective: nodeUrl is already LCD)
        const restBase = (currentNetwork.cosmosRestUrl || nodeUrl).replace(/\/$/, "");
        const accRes = await getJson(`${restBase}/cosmos/auth/v1beta1/accounts/${fromAddr}`).catch(() => null);
        const accInfo = accRes?.account;
        const sequence = parseInt(accInfo?.sequence || "0", 10);
        const accountNumber = parseInt(accInfo?.account_number || "0", 10);

        const txBody = signCosmosTx({
          chainId: cosmosChainId,
          sequence,
          accountNumber,
          fromAddress: fromAddr,
          toAddress: recipient,
          amount,
          denom,
          memo: "",
          gas: 200000,
        }, privKey);

        const broadcastRes = await postJson(`${restBase}/cosmos/tx/v1beta1/txs`, {
          tx: txBody.tx,
          mode: "BROADCAST_MODE_SYNC",
        }).catch(() => null);
        hash = broadcastRes?.tx_response?.txhash || broadcastRes?.txhash || "";
        if (!hash) throw new Error(broadcastRes?.tx_response?.raw_log || "Cosmos broadcast failed");

        // ── Solana: client-side ed25519 signing ───────────────────────────────
      } else if (family === "solana") {
        if (!chainKeys?.solana) throw new Error("Unlock wallet with mnemonic to sign Solana transactions");
        const privKey = chainKeys.solana.privateKey;
        const fromPubkey = chainKeys.solana.address;

        const bhRes = await postJson(nodeUrl, {
          jsonrpc: "2.0", id: 1, method: "getLatestBlockhash", params: [{ commitment: "finalized" }],
        });
        const blockhash = bhRes?.result?.value?.blockhash;
        if (!blockhash) throw new Error("Could not get Solana blockhash");

        const lamports = amount; // amount is already in smallest unit
        const signedTx = signSolanaTransfer({
          fromPubkey,
          toPubkey: recipient,
          lamports,
          recentBlockhash: blockhash,
        }, privKey);

        const sendRes = await postJson(nodeUrl, {
          jsonrpc: "2.0", id: 1, method: "sendTransaction",
          params: [signedTx, { encoding: "base58" }],
        });
        hash = sendRes?.result || "";
        if (!hash) throw new Error(sendRes?.error?.message || "Solana broadcast failed");

        // ── NEAR: borsh-encode + ed25519 sign ─────────────────────────────────
      } else if (family === "near") {
        if (!chainKeys?.near) throw new Error("Unlock wallet with mnemonic to sign NEAR transactions");
        const privKey = chainKeys.near.privateKey;
        // chainKeys.near.address = hex pubkey; base58 encode for access key lookup
        const pubBase58 = cryptoBase58Encode(hexToBytes(chainKeys.near.address));
        const signerId = watchAddresses[activeNetworkId] || activeAddress;

        // Access key nonce for this public key
        const keyRes = await postJson(nodeUrl, {
          jsonrpc: "2.0", id: 1, method: "query",
          params: { request_type: "view_access_key", finality: "final", account_id: signerId, public_key: `ed25519:${pubBase58}` },
        }).catch(() => null);
        const nonce = (keyRes?.result?.nonce || 0) + 1;

        // Latest block hash
        const blockRes = await postJson(nodeUrl, {
          jsonrpc: "2.0", id: 1, method: "block", params: { finality: "final" },
        }).catch(() => null);
        const blockHash = blockRes?.result?.header?.hash;
        if (!blockHash) throw new Error("Could not fetch NEAR block hash");

        const signedTx = signNearTransfer({ signerId, receiverId: recipient, amount, nonce, blockHash }, privKey);
        const sendRes = await postJson(nodeUrl, {
          jsonrpc: "2.0", id: 1, method: "broadcast_tx_commit", params: [signedTx],
        });
        hash = sendRes?.result?.transaction?.hash || sendRes?.result?.transaction_outcome?.id || "";
        if (!hash && sendRes?.error) throw new Error(sendRes.error.data || sendRes.error.message || "NEAR broadcast failed");
        if (!hash) hash = sendRes?.result?.transaction?.hash || "pending";

        // ── Aptos: encode_submission → ed25519 sign → submit ──────────────────
      } else if (family === "aptos") {
        if (!chainKeys?.aptos) throw new Error("Unlock wallet with mnemonic to sign Aptos transactions");
        const privKey = chainKeys.aptos.privateKey;
        const fromAddr = chainKeys.aptos.address;

        const accRes = await getJson(`${nodeUrl}/accounts/${fromAddr}`).catch(() => null);
        const seqNum = accRes?.sequence_number || "0";
        const expTime = Math.floor(Date.now() / 1000) + 120;

        const encBody = {
          sender: fromAddr,
          sequence_number: seqNum,
          max_gas_amount: "2000",
          gas_unit_price: "100",
          expiration_timestamp_secs: String(expTime),
          payload: {
            type: "entry_function_payload",
            function: "0x1::aptos_coin::transfer",
            type_arguments: [],
            arguments: [recipient, String(amount)],
          },
        };
        const signingHex = await postJson(`${nodeUrl}/transactions/encode_submission`, encBody);
        if (!signingHex || typeof signingHex !== "string") throw new Error("Aptos encode_submission failed");

        const { publicKey: aptPub, signature: aptSig } = signAptosEntry(signingHex, privKey);
        const submitRes = await postJson(`${nodeUrl}/transactions`, {
          ...encBody,
          signature: { type: "ed25519_signature", public_key: aptPub, signature: aptSig },
        });
        hash = submitRes?.hash || "";
        if (!hash) throw new Error(submitRes?.message || submitRes?.error_code || "Aptos transaction failed");

        // ── SUI: unsafe_transferSui → blake2b + ed25519 sign → execute ────────
      } else if (family === "sui") {
        if (!chainKeys?.sui) throw new Error("Unlock wallet with mnemonic to sign SUI transactions");
        const privKey = chainKeys.sui.privateKey;
        const fromAddr = chainKeys.sui.address;

        // Get SUI coin objects
        const coinsRes = await postJson(nodeUrl, {
          jsonrpc: "2.0", id: 1, method: "suix_getCoins",
          params: [fromAddr, "0x2::sui::SUI", null, 1],
        }).catch(() => null);
        const coinObjectId = coinsRes?.result?.data?.[0]?.coinObjectId;
        if (!coinObjectId) throw new Error("No SUI coin object found in wallet");

        // Build unsigned tx via node
        const buildRes = await postJson(nodeUrl, {
          jsonrpc: "2.0", id: 1, method: "unsafe_transferSui",
          params: [fromAddr, coinObjectId, "10000000", recipient, String(amount)],
        }).catch(() => null);
        const txBytes = buildRes?.result?.txBytes;
        if (!txBytes) throw new Error(buildRes?.error?.message || "SUI tx build failed");

        const suiSig = signSuiTx(txBytes, privKey);
        const execRes = await postJson(nodeUrl, {
          jsonrpc: "2.0", id: 1, method: "sui_executeTransactionBlock",
          params: [txBytes, [suiSig], { showEffects: true }, "WaitForLocalExecution"],
        });
        hash = execRes?.result?.digest || "";
        if (!hash) throw new Error(execRes?.error?.message || "SUI execution failed");

        // ── BTC (P2WPKH, SegWit, BIP143) ─────────────────────────────────────
      } else if (family === "utxo") {
        if (!chainKeys?.btc) throw new Error("Unlock wallet with mnemonic to sign BTC transactions");
        const privKey = chainKeys.btc.privateKey;
        const fromAddr = chainKeys.btc.address; // tb1...

        // Fetch UTXOs from Blockstream
        const utxoRes = await getJson(`${nodeUrl}/address/${fromAddr}/utxo`).catch(() => []);
        const utxos = (Array.isArray(utxoRes) ? utxoRes : []).map(u => ({
          txid: u.txid, vout: u.vout, value: u.value,
        }));
        if (!utxos.length) throw new Error("No UTXOs found. Fund your testnet address first.");

        const amountSats = Number(amount); // amount is already in satoshis (smallest unit)
        const feeSats = 300;
        const totalIn = utxos.reduce((s, u) => s + u.value, 0);
        if (totalIn < amountSats + feeSats) throw new Error(`Insufficient BTC balance. Need ${amountSats + feeSats} sats, have ${totalIn} sats.`);

        const signedHex = signBtcP2WPKHTx({
          utxos,
          recipients: [{ address: recipient, value: amountSats }],
          changeAddress: fromAddr,
          feeSatoshis: feeSats,
        }, privKey);

        // Broadcast via Blockstream
        const broadcastRes = await fetch(`${nodeUrl}/tx`, {
          method: "POST",
          headers: { "Content-Type": "text/plain" },
          body: signedHex,
        });
        const txid = await broadcastRes.text();
        if (!broadcastRes.ok) throw new Error(`BTC broadcast failed: ${txid}`);
        hash = txid.trim();

        // ── LTC (P2WPKH, BlockCypher API) ─────────────────────────────────────
      } else if (family === "litecoin") {
        if (!chainKeys?.ltc) throw new Error("Unlock wallet with mnemonic to sign LTC transactions");
        const privKey = chainKeys.ltc.privateKey;
        const fromAddr = chainKeys.ltc.address; // tltc1...

        // Fetch UTXOs via BlockCypher
        const utxoRes = await getJson(`${nodeUrl}/addrs/${fromAddr}?unspentOnly=true`).catch(() => null);
        const utxos = (utxoRes?.txrefs || []).map(u => ({
          txid: u.tx_hash, vout: u.tx_output_n, value: u.value,
        }));
        if (!utxos.length) throw new Error("No UTXOs found. Fund your LTC testnet address first.");

        const amountSats = Number(amount);
        const feeSats = 500;
        const totalIn = utxos.reduce((s, u) => s + u.value, 0);
        if (totalIn < amountSats + feeSats) throw new Error(`Insufficient LTC. Need ${amountSats + feeSats} sats, have ${totalIn} sats.`);

        const signedHex = signBtcP2WPKHTx({
          utxos,
          recipients: [{ address: recipient, value: amountSats }],
          changeAddress: fromAddr,
          feeSatoshis: feeSats,
        }, privKey);

        // Broadcast via BlockCypher
        const broadRes = await postJson(`${nodeUrl}/txs/push`, { tx: signedHex });
        hash = broadRes?.tx?.hash || "";
        if (!hash) throw new Error(broadRes?.error || "LTC broadcast failed");

        // ── TON: wallet v3R2 BOC send ────────────────────────────────────────
      } else if (family === "ton") {
        if (!chainKeys?.ton) throw new Error("Unlock wallet with mnemonic to sign TON transactions");
        const tonWalletAddr = chainKeys.ton.address || chainKeys.ton.rawPubkey || "";
        if (!tonWalletAddr || !tonWalletAddr.startsWith("EQ")) {
          throw new Error("TON wallet address not available. Re-import your mnemonic.");
        }
        setProcessingMessage("Fetching TON seqno...");
        const seqno = await getTonSeqno(nodeUrl, tonWalletAddr);
        const nanoTon = String(parseUnits(sendForm.amount, 9));
        setProcessingMessage("Building & signing TON BOC...");
        const bocBase64 = buildTonTransferBoc(
          {
            walletAddress: tonWalletAddr,
            toAddress: recipient,
            nanoTon,
            seqno,
            memo: sendForm.memo || "",
            bounce: true,
          },
          chainKeys.ton.privateKey
        );
        setProcessingMessage("Broadcasting to TON network...");
        hash = await broadcastTonBoc(nodeUrl, bocBase64);

        // ── Starknet: invoke v1 with STARK ECDSA + Pedersen hash ─────────────
      } else if (family === "starknet") {
        if (!chainKeys?.starknet) throw new Error("Unlock wallet with mnemonic to sign Starknet transactions");
        const starkAddr = starknetArgentAddress(chainKeys.starknet.publicKey);
        const amtWei = String(parseUnits(sendForm.amount, 18));
        setProcessingMessage("Signing Starknet invoke transaction...");
        hash = await broadcastStarknetTransfer(
          { senderAddress: starkAddr, toAddress: recipient, amountWei: amtWei, nodeUrl },
          chainKeys.starknet.privateKey
        );

      } else {
        throw new Error(`No native-send adapter is registered for ${currentNetwork.name} (${family})`);
      }

      setProcessingMessage("");
      setStatusModal({
        visible: true,
        title: "Success",
        message: `${currentNetwork.symbol} Sent Successfully!`,
        type: "success",
        hash,
      });
      addTrackedTx({ hash, type: "native_send", symbol: currentNetwork.symbol, to: recipient, family });
      recordTelemetry("native_send_success", { network: activeNetworkId, family, hash }).catch(() => {});
      rememberActivity({
        type: "send",
        From: activeAddress,
        To: recipient,
        TxHash: hash,
        Timestamp: Math.floor(Date.now() / 1000),
        Status: "success",
        Value: sendForm.amount,
      });
      setSendForm(initialSendForm);
      setSendVisible(false);
      refreshWalletSnapshot();
      setTimeout(() => refreshWalletSnapshot(), 2000);
      setTimeout(() => refreshWalletSnapshot(), 5000);
      setTimeout(() => refreshWalletSnapshot(), 10000);
    } catch (e) {
      setProcessingMessage("");
      recordError("native_send", e, { network: activeNetworkId, family }).catch(() => {});
      setStatusModal({
        visible: true,
        title: "Failed",
        message: e.message || "Transaction failed",
        type: "error",
        hash: "",
      });
    } finally {
      setBusy(false);
      setBusyAction("");
      setProcessingMessage("");
    }
  }

  function saveWatchAddress() {
    const addr = watchAddrInput.trim();
    const family = currentNetwork.family || "evm";
    if (!isLikelyAddressForFamily(addr, family)) {
      showToast(`Invalid ${currentNetwork.name} address format`, "error");
      return;
    }
    setWatchAddresses(prev => ({ ...prev, [activeNetworkId]: addr }));
    setWatchAddrInput("");
    showToast(`${currentNetwork.name} address saved`, "success");
  }

  function clearWatchAddress() {
    setWatchAddresses(prev => {
      const next = { ...prev };
      delete next[activeNetworkId];
      return next;
    });
    showToast("Watch address cleared", "info");
  }

  async function addTokenAction() {
    if (!wallet?.address) {
      setStatus("Unlock wallet first");
      return;
    }
    const address = tokenImportForm.address.trim();
    const family = currentNetwork.family || "evm";
    const familyUi = FAMILY_TOKEN_UI[family] || DEFAULT_FAMILY_UI;

    if (!familyUi.supported) {
      showToast(familyUi.unsupportedMsg || "Token import not supported on this chain", "error");
      return;
    }
    if (!isLikelyAddressForFamily(address, family)) {
      showToast(`Enter a valid ${family.toUpperCase()} address (e.g. ${familyUi.placeholder})`, "error");
      return;
    }
    const hasManualMeta = !!(tokenImportForm.symbol?.trim() && tokenImportForm.name?.trim() && tokenImportForm.decimals?.trim());
    const hasVerifiedPreview = !!(tokenPreviewMeta?.verified && tokenPreviewMeta?.symbol && tokenPreviewMeta?.name);
    if (!hasVerifiedPreview && !hasManualMeta) {
      showToast("Token metadata could not be verified. Fill symbol, name and decimals manually before import.", "error");
      return;
    }
    setBusy(true);
    setBusyAction("addToken");
    setProcessingMessage("Importing Token...");
    // Use the chain-specific active address as the holder (e.g. cosmos1... for Cosmos, 0x... for EVM)
    const holderAddr = activeAddress || wallet.address;
    const effectiveNodeUrls = getRpcCandidatesForFamily(currentNetwork, family);
    try {
      // Use already-fetched preview meta if available; otherwise fetch now
      let meta = tokenPreviewMeta;
      if (!meta) {
        try {
          setProcessingMessage("Fetching token metadata…");
          meta = await resolveTokenMetaMultichain(effectiveNodeUrls, address, holderAddr, family);
        } catch { meta = null; }
      }

      // Always prefer user-provided override values over auto-fetched
      const userSymbol = tokenImportForm.symbol?.trim();
      const userName = tokenImportForm.name?.trim();
      const userDec = tokenImportForm.decimals?.trim();
      const FALLBACK_SYMBOLS = new Set(["TOKEN", "SPL", "FT", "Jetton", "TRC20", "CW20", "IBC"]);
      const fetchedSymbolIsReal = meta?.symbol && !FALLBACK_SYMBOLS.has(meta.symbol);
      meta = {
        address,
        symbol: userSymbol || (fetchedSymbolIsReal ? meta?.symbol : null) || meta?.symbol || "TOKEN",
        name: userName || (fetchedSymbolIsReal ? meta?.name : null) || meta?.name || userSymbol || "Token",
        decimals: userDec ? parseInt(userDec, 10) : (meta?.decimals ?? getDefaultDecimalsForFamily(family)),
        verified: meta?.verified || !!(userSymbol && userName),
      };

      setProcessingMessage("Fetching balance…");
      const balance = await resolveTokenBalanceMultichain(effectiveNodeUrls, walletUrl, address, holderAddr, family).catch(() => "0");

      // If this token was previously removed, clear it from the blacklist so it shows again
      const addrKey = String(address).toLowerCase();
      setRemovedTokens(prev => prev.filter(r => !(r.address && String(r.address).toLowerCase() === addrKey && r.networkId === activeNetworkId)));
      setHiddenTokens(prev => prev.filter(h => !(h.address && String(h.address).toLowerCase() === addrKey && h.networkId === activeNetworkId)));

      const importedToken = {
        address,
        name: meta.name,
        symbol: meta.symbol,
        decimals: meta.decimals,
        balance,
        networkId: activeNetworkId,
        family,
        holderAddress: holderAddr,
      };
      setWatchlist(prev => {
        const nextKey = tokenKey(importedToken);
        const withoutOld = (prev || []).filter(item => tokenKey(item) !== nextKey);
        return [importedToken, ...withoutOld];
      });
      setTokenImportForm(initialTokenImportForm);
      setTokenPreviewMeta(null);
      setTokenImportVisible(false);
      showToast(`Imported ${meta.symbol}`, "success");
    } catch (e) {
      showToast(e.message || "Token import failed", "error");
    } finally {
      setBusy(false);
      setBusyAction("");
      setProcessingMessage("");
    }
  }

  async function importNftAction() {
    if (!wallet?.address) {
      showToast("Unlock wallet first", "error");
      return;
    }
    const family = String(currentNetwork.family || "evm").toLowerCase();
    const address = nftImportForm.address.trim();
    const tokenId = nftImportForm.tokenId.trim();
    if (!validateNftIdentifier(address, tokenId, family)) {
      const message = ["utxo", "litecoin"].includes(family)
        ? `${currentNetwork.name} has no account-based NFT contract standard`
        : `Enter a valid ${currentNetwork.name} NFT identifier and token ID`;
      showToast(message, "error");
      return;
    }

    setBusy(true);
    setBusyAction("addNft");
    setProcessingMessage("Verifying NFT...");
    try {
      let metadata = {};
      if (currentNetwork.id === "lqd") {
        const [abi, storage] = await Promise.all([
          nodeContractAbi(nodeUrl, address),
          nodeContractStorage(nodeUrl, address).catch(() => null),
        ]);
        if (!abi) throw new Error("No LQD contract is deployed at this address");
        const storageData = storage?.State?.storage || storage?.State || storage?.state?.storage || storage?.state || storage?.storage || storage || {};
        const owner = String(storageData[`owner:${tokenId}`] || storageData[`ownerOf:${tokenId}`] || "");
        if (!owner) throw new Error(`LQD NFT token #${tokenId} does not exist in this contract`);
        const baseUri = String(storageData["uri:base"] || "");
        const tokenUri = String(storageData[`uri:${tokenId}`] || (baseUri ? `${baseUri}${tokenId}` : ""));
        const uriMetadata = await resolveNftUriMetadata(tokenUri, tokenId).catch(() => null);
        const expectedOwner = String(activeAddress || wallet.address).toLowerCase();
        metadata = {
          name: uriMetadata?.name || storageData.name || storageData.Name || "",
          symbol: storageData.symbol || storageData.Symbol || "",
          description: uriMetadata?.description || "",
          image: normalizeNftUri(uriMetadata?.image || storageData.image || storageData.Image || ""),
          tokenUri: normalizeNftUri(tokenUri),
          owner,
          ownershipVerified: owner.toLowerCase() === expectedOwner,
          ownershipMismatch: owner.toLowerCase() !== expectedOwner,
          contractVerified: true,
        };
      } else if (["evm", "harmony"].includes(family) || (family === "sei" && /^0x[a-fA-F0-9]{40}$/.test(address))) {
        const ownerAddress = family === "sei"
          ? (chainKeys?.sei?.evmAddress || chainKeys?.evm?.address || wallet.address)
          : (chainKeys?.evm?.address || wallet.address);
        let lastError = null;
        for (const rpcUrl of getRpcCandidatesForFamily(currentNetwork, family)) {
          try {
            metadata = await resolveEvmNftMetadata({ rpcUrl, contract: address, tokenId, ownerAddress });
            lastError = null;
            break;
          } catch (error) {
            lastError = error;
          }
        }
        if (lastError) throw lastError;
      }

      const imported = {
        address,
        tokenId,
        name: nftImportForm.name.trim() || metadata.name || `NFT #${tokenId}`,
        symbol: metadata.symbol || "NFT",
        description: metadata.description || "",
        image: normalizeNftUri(nftImportForm.image.trim() || metadata.image || ""),
        tokenUri: metadata.tokenUri || "",
        owner: metadata.owner || "",
        ownershipVerified: Boolean(metadata.ownershipVerified),
        ownershipMismatch: Boolean(metadata.ownershipMismatch),
        contractVerified: Boolean(metadata.contractVerified),
        networkId: activeNetworkId,
        family,
        importedAt: Date.now(),
      };
      setNfts((previous) => [imported, ...previous.filter((item) => nftKey(item) !== nftKey(imported))]);
      setNftImportForm(initialNftImportForm);
      setNftImportVisible(false);
      setHomeSubTab("nfts");
      if (imported.ownershipMismatch) {
        showToast("NFT imported as watch-only; active address is not the on-chain owner", "info");
      } else {
        showToast(`Imported ${imported.name}`, "success");
      }
    } catch (error) {
      showToast(error?.message || "NFT import failed", "error");
    } finally {
      setBusy(false);
      setBusyAction("");
      setProcessingMessage("");
    }
  }

  function removeNftAction(nft) {
    Alert.alert(
      "Remove NFT",
      `Remove ${nft?.name || `NFT #${nft?.tokenId || ""}`} from this wallet view? This does not affect on-chain ownership.`,
      [
        { text: "Cancel", style: "cancel" },
        {
          text: "Remove",
          style: "destructive",
          onPress: () => {
            setNfts((previous) => previous.filter((item) => nftKey(item) !== nftKey(nft)));
            showToast("NFT removed from wallet view", "info");
          },
        },
      ]
    );
  }

  // Import a list of pre-discovered {address, balance} items for a non-EVM chain.
  // Fetches metadata, skips already-watchlisted tokens, and stores holderAddress.
  async function importMultichainDiscovery(items, holderAddress, family) {
    if (!items?.length) return 0;
    const existingSet = new Set((watchlist || []).map(t => tokenKey(t)).filter(Boolean));
    const novel = items.filter(item => item?.address && !existingSet.has(`${activeNetworkId}:${String(item.address).toLowerCase()}`));
    if (!novel.length) return 0;

    const discoveryNodeUrls = getRpcCandidatesForFamily(currentNetwork, family);
    const resolved = [];
    for (const item of novel) {
      try {
        const meta = await resolveTokenMetaMultichain(discoveryNodeUrls, item.address, holderAddress, family)
          .catch(() => null);
        // Skip tokens with no real metadata and zero balance
        if (!meta && !item.balance) continue;
        const symbol = meta?.symbol || item.address.split("::").pop() || "TOKEN";
        if (symbol === "TOKEN" && (!item.balance || item.balance === "0")) continue;
        resolved.push({
          address: item.address,
          symbol,
          name: meta?.name || symbol,
          decimals: meta?.decimals ?? 6,
          balance: item.balance || "0",
          family,
          networkId: activeNetworkId,
          holderAddress,
        });
      } catch { /* skip unresolvable entries */ }
    }
    if (!resolved.length) return 0;
    setWatchlist(prev => {
      const incoming = new Set(resolved.map(token => tokenKey(token)));
      return [...resolved, ...(prev || []).filter(token => !incoming.has(tokenKey(token)))];
    });
    return resolved.length;
  }

  async function autoDiscoverTokensAction() {
    if (!wallet?.address) { showToast("Unlock wallet first", "error"); return; }
    setBusy(true);
    setBusyAction("autoDiscoverTokens");
    try {
      const family = String(currentNetwork.family || "evm").toLowerCase();
      const holder = activeAddress || wallet.address;
      const isLqdNode = isLqdNetworkUrl(nodeUrl);

      // ── EVM (LQD or external) ──
      if (family === "evm") {
        if (!isLqdNode) {
          const count = await autoDiscoverErc20(holder);
          showToast(count ? `Auto imported ${count} token(s)` : "No new token activity found", count ? "success" : "info");
          return;
        }
        // LQD mainnet: bridge tokens + pool tokens + recent activity
        const recent = await nodeRecentTransactions(nodeUrl).catch(() => []);
        const bridgeTokenList = await nodeBridgeTokens(nodeUrl).catch(() => []);
        const factory = await nodeCurrentFactory(nodeUrl).catch(() => null);
        const pools = await nodeLiquidityPools(nodeUrl).catch(() => null);
        const count = await autoDiscoverTokens({ recent, bridgeTokens: bridgeTokenList, factory, pools }, holder);
        if (!count) await refreshTokenBalances(watchlist, holder);
        showToast(count ? `Auto imported ${count} token(s)` : "No new tokens found", count ? "success" : "info");
        return;
      }

      // ── Harmony: EVM-compatible, use eth_getLogs ──
      if (family === "harmony") {
        const count = await autoDiscoverErc20(holder);
        showToast(count ? `Auto imported ${count} token(s)` : "No new token activity found", count ? "success" : "info");
        return;
      }

      // ── TRON: EVM JSON-RPC compatible; convert T... base58 → 0x for eth_getLogs ──
      if (family === "tron") {
        const evmHolder = tronAddressToEvm(holder) || wallet.address;
        const count = await autoDiscoverErc20(evmHolder);
        showToast(count ? `Auto imported ${count} token(s)` : "No new token activity found", count ? "success" : "info");
        return;
      }

      // ── Solana: check all SPL token accounts owned ──
      if (family === "solana") {
        const items = await discoverSolanaTokens(nodeUrl, holder).catch(() => []);
        const count = await importMultichainDiscovery(items, holder, "solana");
        showToast(count ? `Auto imported ${count} token(s)` : "No SPL tokens found in wallet", count ? "success" : "info");
        return;
      }

      // ── Cosmos / SEI / Injective: bank module balances ──
      if (family === "cosmos" || family === "cosmos-testnet" || family === "sei" || family === "injective") {
        // SEI nodeUrl is EVM RPC; use dedicated Cosmos REST URL
        const cosmosUrl = (family === "sei" || family === "injective")
          ? (currentNetwork.cosmosRestUrl || nodeUrl)
          : nodeUrl;
        const items = await discoverCosmosTokens(cosmosUrl, holder).catch(() => []);
        const count = await importMultichainDiscovery(items, holder, family);
        showToast(count ? `Auto imported ${count} token(s)` : "No extra tokens found in wallet", count ? "success" : "info");
        return;
      }

      // ── Aptos: CoinStore resources ──
      if (family === "aptos") {
        const items = await discoverAptosCoins(nodeUrl, holder).catch(() => []);
        const count = await importMultichainDiscovery(items, holder, "aptos");
        showToast(count ? `Auto imported ${count} coin(s)` : "No extra coins found in wallet", count ? "success" : "info");
        return;
      }

      // ── SUI: suix_getAllBalances ──
      if (family === "sui") {
        const items = await discoverSuiCoins(nodeUrl, holder).catch(() => []);
        const count = await importMultichainDiscovery(items, holder, "sui");
        showToast(count ? `Auto imported ${count} coin(s)` : "No extra coins found in wallet", count ? "success" : "info");
        return;
      }

      // ── NEAR: check popular NEP-141 token contracts ──
      if (family === "near") {
        if (!holder) { showToast("Save your NEAR address first (tap Receive)", "info"); return; }
        const items = await discoverNearTokens(nodeUrl, holder).catch(() => []);
        const count = await importMultichainDiscovery(items, holder, "near");
        showToast(count ? `Auto imported ${count} token(s)` : "No NEP-141 tokens found in wallet", count ? "success" : "info");
        return;
      }

      showToast(`${currentNetwork.name} requires an external token indexer for automatic discovery. Use Import Token for manual on-chain lookup.`, "info");
    } catch (e) {
      showToast(e.message || "Auto token discovery failed", "error");
    } finally {
      setBusy(false);
      setBusyAction("");
    }
  }

  async function autoDiscoverErc20(address) {
    const ERC20_TRANSFER = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef";
    const padded = "0x000000000000000000000000" + address.replace("0x", "").toLowerCase();
    const rpcUrls = getNetworkNodeCandidates(currentNetwork);
    const [sentRes, recvRes] = await Promise.all([
      tryRpcCandidates(rpcUrls, (url) => postJson(url, {
        jsonrpc: "2.0", id: 1, method: "eth_getLogs",
        params: [{ topics: [ERC20_TRANSFER, padded], fromBlock: "earliest", toBlock: "latest" }]
      }).catch(() => null), (r) => Array.isArray(r?.result)).then(({ result }) => result).catch(() => null),
      tryRpcCandidates(rpcUrls, (url) => postJson(url, {
        jsonrpc: "2.0", id: 2, method: "eth_getLogs",
        params: [{ topics: [ERC20_TRANSFER, null, padded], fromBlock: "earliest", toBlock: "latest" }]
      }).catch(() => null), (r) => Array.isArray(r?.result)).then(({ result }) => result).catch(() => null),
    ]);
    const contracts = new Set([
      ...((sentRes?.result || []).map(l => String(l.address || "").toLowerCase())),
      ...((recvRes?.result || []).map(l => String(l.address || "").toLowerCase())),
    ].filter(Boolean));
    if (!contracts.size) return 0;
    return importDetectedTokens(Array.from(contracts), address, "auto");
  }

  function removeToken(token) {
    if (!token?.address) return;
    const netId = token.networkId || activeNetworkId;
    const key = String(token.address).toLowerCase();
    setRemovedTokens(prev => {
      const alreadyIn = prev.some(r => r.address && String(r.address).toLowerCase() === key && r.networkId === netId);
      if (alreadyIn) return prev;
      return [...prev, { address: token.address, networkId: netId }];
    });
    setHiddenTokens(prev => prev.filter(h => !(h.address && String(h.address).toLowerCase() === key && h.networkId === netId)));
    setTokenActionTarget(null);
    showToast(`${token.symbol} removed`, "info");
  }

  function hideToken(token) {
    if (!token?.address) return;
    const netId = token.networkId || activeNetworkId;
    const key = String(token.address).toLowerCase();
    setHiddenTokens(prev => {
      const already = prev.some(h => h.address && String(h.address).toLowerCase() === key && h.networkId === netId);
      if (already) return prev;
      return [...prev, { address: token.address, networkId: netId }];
    });
    setTokenActionTarget(null);
    showToast(`${token.symbol} hidden`, "info");
  }

  function unhideToken(token) {
    if (!token?.address) return;
    const netId = token.networkId || activeNetworkId;
    const key = String(token.address).toLowerCase();
    setHiddenTokens(prev => prev.filter(h => !(h.address && String(h.address).toLowerCase() === key && h.networkId === netId)));
    showToast(`${token.symbol} visible again`, "success");
  }

  async function refreshSingleToken(address) {
    if (!wallet?.address) return;
    try {
      const existing = watchlist.find(t => String(t.address).toLowerCase() === String(address).toLowerCase());
      const fam = existing?.family || "evm";
      const holder = existing?.holderAddress || activeAddress || wallet.address;
      const tokenNetwork = NETWORKS.find(n => n.id === (existing?.networkId || activeNetworkId)) || currentNetwork;
      const tokenNodeUrls = getRpcCandidatesForFamily(tokenNetwork, fam);
      const tokenNodeUrl = tokenNodeUrls[0] || nodeUrl;

      const meta = await resolveTokenMetaMultichain(tokenNodeUrls, address, holder, fam).catch(() => existing || { address });

      let balance = existing?.balance || "0";
      if (fam === "ton") {
        const tonAddr = chainKeys?.ton?.address || chainKeys?.ton?.rawPubkey || holder;
        balance = await getTonJettonBalance(tokenNodeUrl, tonAddr, address).catch(() => "0");
      } else if (fam === "starknet" && chainKeys?.starknet?.publicKey) {
        const sAddr = starknetArgentAddress(chainKeys.starknet.publicKey);
        balance = await resolveTokenBalanceMultichain(tokenNodeUrls, walletUrl, address, sAddr, fam).catch(() => "0");
      } else {
        balance = await resolveTokenBalanceMultichain(tokenNodeUrls, walletUrl, address, holder, fam).catch(() => existing?.balance || "0");
      }

      setWatchlist((prev) => mergeUniqueByKey(prev, [{ ...(existing || {}), ...meta, balance }], "address"));
    } catch (e) {
      setStatus(e.message || "Token refresh failed");
    }
  }

  function bridgeTokenExternalAddress(token = bridgeSelectedToken) {
    return String(token?.source_token || token?.SourceToken || token?.bsc_token || token?.BscToken || token?.address || "").trim();
  }

  function bridgeTokenLqdAddress(token = bridgeSelectedToken) {
    return String(token?.lqd_token || token?.LqdToken || token?.target_token || token?.TargetToken || token?.address || "").trim();
  }

  function bridgeTokenDecimals(token = bridgeSelectedToken) {
    const parsed = Number(token?.decimals || token?.Decimals || 8);
    return Number.isFinite(parsed) && parsed >= 0 ? parsed : 8;
  }

  function bridgeFamilySupportsWalletSigning(family) {
    return bridgeWalletSignedFamilies.includes(String(family || "evm").toLowerCase());
  }

  function bridgeNeedsExternalProof(family = currentBridgeFamily) {
    return bridgeDirection === "external_to_lqd" && !bridgeFamilySupportsWalletSigning(family);
  }

  function bridgeProofLabel(family = currentBridgeFamily) {
    const fam = String(family || "").toLowerCase();
    if (fam === "solana") return "Solana signature";
    if (fam === "ton") return "TON transaction hash";
    if (fam === "near") return "NEAR receipt / transaction hash";
    if (fam === "aptos") return "Aptos version / transaction hash";
    if (fam === "sui") return "Sui digest";
    if (fam === "starknet") return "Starknet transaction hash";
    if (fam === "tron") return "Tron transaction hash";
    if (fam === "cosmos" || fam === "cosmos-testnet" || fam === "sei" || fam === "injective") return "Cosmos tx hash";
    if (fam === "utxo" || fam === "bitcoin" || fam === "btc" || fam === "litecoin") return "UTXO txid";
    return "Source transaction hash";
  }

  function bridgeProofAddressLabel(family = currentBridgeFamily) {
    const fam = String(family || "").toLowerCase();
    if (fam === "utxo" || fam === "bitcoin" || fam === "btc" || fam === "litecoin") return "Sender / input address";
    return "Source sender address";
  }

  function bridgeProofReady(family = currentBridgeFamily) {
    if (!bridgeNeedsExternalProof(family)) return true;
    const fam = String(family || "").toLowerCase();
    if (!(bridgeForm.sourceTxHash.trim() && bridgeForm.sourceAddress.trim())) return false;
    if (["cosmos", "cosmos-testnet", "sei", "injective"].includes(fam)) {
      return !!bridgeForm.sourceMemo.trim();
    }
    if (["utxo", "bitcoin", "btc", "litecoin", "dogecoin", "cardano"].includes(fam)) {
      return !!bridgeForm.sourceOutput.trim();
    }
    if (["solana", "substrate", "xrpl", "ton", "near", "aptos", "sui", "starknet", "tron"].includes(fam)) {
      return !!bridgeForm.sourceSequence.trim();
    }
    return true;
  }

  function bridgeChainIsEnabled(chain = currentBridgeChain) {
    if (!chain) return true;
    return chain.enabled !== false && chain.Enabled !== false;
  }

  function bridgeExternalPrivateKey(family) {
    const fam = String(family || "evm").toLowerCase();
    if (fam === "harmony") return chainKeys?.harmony?.privateKey || chainKeys?.evm?.privateKey || wallet?.privateKey || "";
    if (fam === "tron") return chainKeys?.tron?.privateKey || wallet?.privateKey || "";
    if (fam === "sei") return chainKeys?.sei?.privateKey || chainKeys?.evm?.privateKey || wallet?.privateKey || "";
    return chainKeys?.evm?.privateKey || wallet?.privateKey || "";
  }

  function bridgeRequestHash(item = {}) {
    return String(item.tx_hash || item.TxHash || item.lqd_tx_hash || item.LqdTxHash || item.bsc_tx_hash || item.BscTxHash || item.source_tx_hash || item.SourceTxHash || "").trim();
  }

  function bridgeRequestMatches(item, tx) {
    const needle = String(tx?.tx_hash || tx?.hash || "").toLowerCase();
    if (!needle) return false;
    return [
      item?.tx_hash,
      item?.TxHash,
      item?.lqd_tx_hash,
      item?.LqdTxHash,
      item?.bsc_tx_hash,
      item?.BscTxHash,
      item?.source_tx_hash,
      item?.SourceTxHash,
      item?.id,
      item?.ID,
    ].some((value) => String(value || "").toLowerCase() === needle);
  }

  async function refreshBridgeTracking(tx = activeBridgeTx) {
    if (!tx) return null;
    const requests = await nodeBridgeRequests(nodeUrl).catch(() => []);
    if (Array.isArray(requests)) {
      setBridgeRequests(requests);
      const matched = requests.find((item) => bridgeRequestMatches(item, tx));
      if (matched) {
        const status = String(matched.status || matched.Status || tx.status || "pending").toLowerCase();
        setActiveBridgeTx((prev) => prev ? { ...prev, ...matched, tx_hash: bridgeRequestHash(matched) || prev.tx_hash, status } : prev);
        return matched;
      }
    }
    return null;
  }

  async function submitBridgeAction() {
    if (!wallet?.address || !wallet?.privateKey) return showToast("Unlock wallet first", "error");
    if (!bridgeForm.amount) return showToast("Enter amount", "error");

    const chain = currentBridgeChain || {};
    const family = String(chain.family || "evm").toLowerCase();
    const adapter = String(chain.adapter || family || "evm").toLowerCase();
    const recipient = (bridgeDirection === "lqd_to_external" ? bridgeForm.toBsc : bridgeForm.toLqd).trim();
    if (!recipient) return showToast("Enter recipient address", "error");
    if (!bridgeChainIsEnabled(chain)) return showToast(`${chain.name || bridgeChainId} bridge route is not active in registry yet`, "error");

    const hasBridgeToken = !!bridgeSelectedToken;
    const tokenAddress = bridgeDirection === "lqd_to_external"
      ? bridgeTokenLqdAddress()
      : bridgeTokenExternalAddress();
    if (hasBridgeToken && !tokenAddress) return showToast("Selected bridge token is missing mapping", "error");

    const decimals = hasBridgeToken ? bridgeTokenDecimals() : (bridgeDirection === "lqd_to_external" ? 8 : Number(chain.decimals || chain.native_decimals || 8));
    const amount = parseUnits(bridgeForm.amount, decimals || 8);
    if (BigInt(amount) <= 0n) return showToast("Enter a valid amount", "error");

    const metadata = {
      chain_id: bridgeChainId,
      family,
      adapter,
      source_tx_hash: bridgeForm.sourceTxHash.trim(),
      source_address: bridgeForm.sourceAddress.trim(),
      source_memo: bridgeForm.sourceMemo.trim(),
      source_sequence: bridgeForm.sourceSequence.trim(),
      source_output: bridgeForm.sourceOutput.trim(),
      mode: bridgeMode,
    };
    const needsExternalProof = bridgeNeedsExternalProof(family);
    if (needsExternalProof && !bridgeProofReady(family)) {
      return showToast(`${bridgeProofLabel(family)} and source address are required for ${String(chain.name || family).toUpperCase()} bridge proof`, "error");
    }

    setBusy(true);
    setBusyAction("bridge");
    setProcessingMessage(bridgeDirection === "lqd_to_external" ? "Creating LQD bridge request..." : "Creating external bridge request...");
    try {
      let res;
      if (bridgeDirection === "lqd_to_external") {
        if (hasBridgeToken) {
          const payload = {
            ...metadata,
            private_key: wallet.privateKey,
            token: tokenAddress,
            amount,
            to_bsc: recipient,
          };
          res = bridgeMode === "private"
            ? await walletBridgePrivateBurnLqdToken(walletUrl, payload)
            : await walletBridgeBurnLqdToken(walletUrl, payload);
        } else {
          const baseFee = await nodeBaseFee(nodeUrl).catch(() => bridgeBaseFee || 10);
          const payload = {
            from: wallet.address,
            to_bsc: recipient,
            amount,
            chain_id: bridgeChainId,
            gas: 200000,
            gas_price: Number(baseFee?.base_fee || baseFee?.BaseFee || baseFee?.baseFee || baseFee || bridgeBaseFee || 10),
            private_key: wallet.privateKey,
            mode: bridgeMode,
          };
          res = bridgeMode === "private"
            ? await walletBridgePrivateLock(walletUrl, payload)
            : await walletBridgeLock(walletUrl, payload);
        }
      } else {
        if (needsExternalProof) {
          const payload = {
            ...metadata,
            tx_hash: metadata.source_tx_hash,
            token: tokenAddress || `native:${bridgeChainId}`,
            from: metadata.source_address,
            to_lqd: recipient,
            amount,
          };
          res = await postJson(`${normalizeUrl(nodeUrl)}/bridge/lock_chain`, payload);
        } else if (hasBridgeToken) {
          const payload = {
            ...metadata,
            private_key: bridgeExternalPrivateKey(family),
            token: tokenAddress,
            amount,
            to_lqd: recipient,
          };
          if (!payload.private_key) throw new Error(`Unlock ${String(family).toUpperCase()} key before bridging from external chain`);
          res = bridgeMode === "private"
            ? await walletBridgePrivateLockBscToken(walletUrl, payload)
            : await walletBridgeLockBscToken(walletUrl, payload);
        } else {
          const payload = {
            private_key: bridgeExternalPrivateKey(family),
            amount,
            to_lqd: recipient,
            chain_id: bridgeChainId,
            mode: bridgeMode,
          };
          if (!payload.private_key) throw new Error(`Unlock ${String(family).toUpperCase()} key before bridging from external chain`);
          res = bridgeMode === "private"
            ? await walletBridgePrivateBurn(walletUrl, payload)
            : await walletBridgeBurn(walletUrl, payload);
        }
      }

      const txHash = res?.tx_hash || res?.hash || res?.TxHash || metadata.source_tx_hash || "";
      const newTx = {
        tx_hash: txHash,
        type: "bridge",
        status: String(res?.status || "pending").toLowerCase(),
        mode: bridgeMode,
        family,
        direction: bridgeDirection,
        from: bridgeDirection === "lqd_to_external" ? wallet.address : (metadata.source_address || chainKeys?.evm?.address || wallet.address),
        to: recipient,
        amount: bridgeForm.amount,
        token: hasBridgeToken ? (bridgeSelectedToken?.symbol || "TOKEN") : "LQD",
        timestamp: Date.now() / 1000,
      };
      setActiveBridgeTx(newTx);
      rememberActivity({
        type: "bridge",
        TxHash: txHash,
        From: newTx.from,
        To: recipient,
        Status: newTx.status,
        Value: bridgeForm.amount,
        token: newTx.token,
        direction: bridgeDirection,
        Timestamp: Math.floor(Date.now() / 1000),
      });
      setBridgeForm((prev) => ({ ...prev, amount: "" }));
      showToast(`Bridge initiated: ${shortAddress(txHash || "pending", 8, 6)}`, "success");
      setTimeout(() => refreshBridgeTracking(newTx), 2500);
      setTimeout(() => refreshWalletSnapshot(), 3000);
    } catch (e) {
      showToast(e.message || "Bridge failed", "error");
    } finally {
      setBusy(false);
      setBusyAction("");
      setProcessingMessage("");
    }
  }

  async function sendTokenAction(token) {
    if (!wallet?.address || !wallet?.privateKey) {
      showToast("Unlock wallet first", "error"); return;
    }
    const family = token.family || currentNetwork.family || "evm";
    const recipient = tokenSendForm.to.trim();

    if (!isLikelyAddressForFamily(recipient, family)) {
      showToast(`Enter a valid ${currentNetwork.name} address (e.g. ${SEND_ADDR_PLACEHOLDER[family] || "..."})`, "error");
      return;
    }
    const amount = parseUnits(tokenSendForm.amount, token.decimals || 8);
    if (BigInt(amount) <= 0n) {
      setStatus("Enter a valid token amount");
      return;
    }
    setBusy(true);
    setBusyAction("sendToken");
    setProcessingMessage(`Broadcasting ${token.symbol} transfer...`);
    try {
      let hash = "";

      // ── LQD: WalletServer ────────────────────────────────────────────────
      if (currentNetwork.id === "lqd") {
        const baseFee = await nodeBaseFee(nodeUrl).catch(() => 10);
        const baseFeeValue = baseFee?.base_fee || baseFee?.BaseFee || baseFee?.baseFee || baseFee || 10;
        const res = await walletContractTx(walletUrl, {
          address: wallet.address,
          contract_address: token.address,
          function: "Transfer",
          args: [recipient, amount],
          value: "0",
          gas: Number(tokenFeeInfo.gasLimit || 50000),
          gas_price: Number(tokenFeeInfo.gasPrice || baseFeeValue || 10),
          private_key: wallet.privateKey,
        });
        hash = res?.tx_hash || res?.hash || "";

        // ── EVM / Harmony / Tron / SEI: ERC-20 EIP-155 ──────────────────────
      } else if (family === "evm" || family === "harmony" || family === "tron" || (family === "sei" && String(token.address || "").startsWith("0x"))) {
        let privKey, fromAddr;
        if (family === "harmony") {
          privKey = chainKeys?.harmony?.privateKey || chainKeys?.evm?.privateKey || wallet.privateKey;
          fromAddr = chainKeys?.harmony?.address || chainKeys?.evm?.address || wallet.address;
        } else if (family === "tron") {
          privKey = chainKeys?.tron?.privateKey || wallet.privateKey;
          fromAddr = chainKeys?.tron?.address || wallet.address; // 0x for RPC
        } else if (family === "sei") {
          privKey = chainKeys?.sei?.privateKey || wallet.privateKey;
          fromAddr = chainKeys?.sei?.evmAddress || chainKeys?.evm?.address || wallet.address;
        } else {
          privKey = chainKeys?.evm?.privateKey || wallet.privateKey;
          fromAddr = chainKeys?.evm?.address || wallet.address;
        }
        const chainId = currentNetwork.chainId;
        if (!chainId) throw new Error(`chainId not configured for ${currentNetwork.name}`);

        const [noncRes, gasPriceRes] = await Promise.all([
          postJson(nodeUrl, { jsonrpc: "2.0", id: 1, method: "eth_getTransactionCount", params: [fromAddr, "pending"] }).catch(() => null),
          postJson(nodeUrl, { jsonrpc: "2.0", id: 1, method: "eth_gasPrice", params: [] }).catch(() => null),
        ]);
        const nonce = noncRes?.result ? parseInt(noncRes.result, 16) : 0;
        const gasPrice = isZeroFeeNetwork(currentNetwork, family)
          ? 0n
          : (tokenFeeInfo.gasPrice ? BigInt(tokenFeeInfo.gasPrice) : (gasPriceRes?.result ? BigInt(gasPriceRes.result) : BigInt(10e9)));
        const gasLimit = tokenFeeInfo.gasLimit ? Number(tokenFeeInfo.gasLimit) : 80000;

        const callData = encodeErc20Transfer(recipient, amount);
        const signedTx = signEip155Tx({ nonce, gasPrice: gasPrice.toString(), gasLimit, to: token.address, value: "0", data: callData, chainId }, privKey);
        const sendRes = await postJson(nodeUrl, { jsonrpc: "2.0", id: 1, method: "eth_sendRawTransaction", params: [signedTx] });
        hash = sendRes?.result || "";
        if (!hash || hash === "0x") throw new Error(sendRes?.error?.message || "Broadcast failed");

        // ── Cosmos / SEI / Injective: amino MsgSend with token denom ────────────
      } else if (family === "cosmos" || family === "cosmos-testnet" || family === "sei" || family === "injective") {
        if (!chainKeys) throw new Error("Unlock wallet with mnemonic to sign Cosmos token transfers");
        let privKey, fromAddr;
        if (family === "sei") {
          privKey = chainKeys.sei.privateKey;
          fromAddr = chainKeys.sei.address; // sei1... bech32
        } else if (family === "injective") {
          privKey = chainKeys.injective.privateKey;
          fromAddr = chainKeys.injective.address;
        } else {
          privKey = chainKeys.cosmos.privateKey;
          fromAddr = chainKeys.cosmos.address;
        }
        const cosmosChainId = currentNetwork.cosmosChainId || currentNetwork.id;
        const denom = token.address;

        // SEI nodeUrl is EVM RPC; use cosmosRestUrl for amino broadcast
        const restBase = (currentNetwork.cosmosRestUrl || nodeUrl).replace(/\/$/, "");
        const accRes = await getJson(`${restBase}/cosmos/auth/v1beta1/accounts/${fromAddr}`).catch(() => null);
        const accInfo = accRes?.account;
        const sequence = parseInt(accInfo?.sequence || "0", 10);
        const accountNumber = parseInt(accInfo?.account_number || "0", 10);

        const isCw20 = /^(?:cosmos|osmo|sei|inj|stars|juno|neutron|migaloo|kujira)1[0-9a-z]+$/.test(denom);
        const feeDenom = family === "sei" ? "usei" : family === "injective" ? "inj" : "uatom";
        const txBody = isCw20
          ? signCosmosWasmExecuteTx({
              chainId: cosmosChainId,
              sequence,
              accountNumber,
              sender: fromAddr,
              contract: denom,
              executeMsg: { transfer: { recipient, amount: String(amount) } },
              feeDenom,
              memo: "",
              gas: 300000,
            }, privKey)
          : signCosmosTx({ chainId: cosmosChainId, sequence, accountNumber, fromAddress: fromAddr, toAddress: recipient, amount, denom, memo: "", gas: 200000 }, privKey);
        const broadcastRes = await postJson(`${restBase}/cosmos/tx/v1beta1/txs`, { tx: txBody.tx, mode: "BROADCAST_MODE_SYNC" }).catch(() => null);
        hash = broadcastRes?.tx_response?.txhash || "";
        if (!hash) throw new Error(broadcastRes?.tx_response?.raw_log || "Cosmos token broadcast failed");

        // ── NEAR: NEP-141 ft_transfer function call ───────────────────────────
      } else if (family === "near") {
        if (!chainKeys?.near) throw new Error("Unlock wallet with mnemonic to sign NEAR token transfers");
        const privKey = chainKeys.near.privateKey;
        const pubBase58 = cryptoBase58Encode(hexToBytes(chainKeys.near.address));
        const signerId = watchAddresses[activeNetworkId] || activeAddress;

        const keyRes = await postJson(nodeUrl, { jsonrpc: "2.0", id: 1, method: "query", params: { request_type: "view_access_key", finality: "final", account_id: signerId, public_key: `ed25519:${pubBase58}` } }).catch(() => null);
        const nonce = (keyRes?.result?.nonce || 0) + 1;
        const blockRes = await postJson(nodeUrl, { jsonrpc: "2.0", id: 1, method: "block", params: { finality: "final" } }).catch(() => null);
        const blockHash = blockRes?.result?.header?.hash;
        if (!blockHash) throw new Error("Could not fetch NEAR block hash");

        const ftArgs = JSON.stringify({ receiver_id: recipient, amount: String(amount), memo: "" });
        const signedTx = signNearFunctionCall({ signerId, contractId: token.address, methodName: "ft_transfer", args: ftArgs, gas: 30000000000000, deposit: 1, nonce, blockHash }, privKey);
        const sendRes = await postJson(nodeUrl, { jsonrpc: "2.0", id: 1, method: "broadcast_tx_commit", params: [signedTx] });
        hash = sendRes?.result?.transaction?.hash || "";
        if (!hash && sendRes?.error) throw new Error(sendRes.error.data || sendRes.error.message || "NEAR ft_transfer failed");
        if (!hash) hash = sendRes?.result?.transaction?.hash || "pending";

        // ── Aptos: coin::transfer entry function ─────────────────────────────
      } else if (family === "aptos") {
        if (!chainKeys?.aptos) throw new Error("Unlock wallet with mnemonic to sign Aptos token transfers");
        const privKey = chainKeys.aptos.privateKey;
        const fromAddr = chainKeys.aptos.address;

        const accRes = await getJson(`${nodeUrl}/accounts/${fromAddr}`).catch(() => null);
        const seqNum = accRes?.sequence_number || "0";
        const expTime = Math.floor(Date.now() / 1000) + 120;
        // token.address is the coin type e.g. "0x1::aptos_coin::AptosCoin" or a FA address
        const coinType = token.address.includes("::") ? token.address : `${token.address}::coin::T`;

        const encBody = {
          sender: fromAddr, sequence_number: seqNum, max_gas_amount: "2000", gas_unit_price: "100",
          expiration_timestamp_secs: String(expTime),
          payload: { type: "entry_function_payload", function: "0x1::coin::transfer", type_arguments: [coinType], arguments: [recipient, String(amount)] },
        };
        const signingHex = await postJson(`${nodeUrl}/transactions/encode_submission`, encBody);
        if (!signingHex || typeof signingHex !== "string") throw new Error("Aptos encode_submission failed");

        const { publicKey: aptPub, signature: aptSig } = signAptosEntry(signingHex, privKey);
        const submitRes = await postJson(`${nodeUrl}/transactions`, { ...encBody, signature: { type: "ed25519_signature", public_key: aptPub, signature: aptSig } });
        hash = submitRes?.hash || "";
        if (!hash) throw new Error(submitRes?.message || "Aptos token transfer failed");

        // ── SUI: coin::transfer via unsafe_pay ───────────────────────────────
      } else if (family === "sui") {
        if (!chainKeys?.sui) throw new Error("Unlock wallet with mnemonic to sign SUI token transfers");
        const privKey = chainKeys.sui.privateKey;
        const fromAddr = chainKeys.sui.address;

        // Get coins of this token type
        const coinsRes = await postJson(nodeUrl, { jsonrpc: "2.0", id: 1, method: "suix_getCoins", params: [fromAddr, token.address, null, 5] }).catch(() => null);
        const coins = coinsRes?.result?.data || [];
        if (!coins.length) throw new Error(`No ${token.symbol} coin objects found in wallet`);

        const inputCoins = coins.map((c) => c.coinObjectId);
        const buildRes = await postJson(nodeUrl, {
          jsonrpc: "2.0", id: 1, method: "unsafe_pay",
          params: [fromAddr, inputCoins, [recipient], [String(amount)], inputCoins[0], "10000000"],
        }).catch(() => null);
        const txBytes = buildRes?.result?.txBytes;
        if (!txBytes) throw new Error(buildRes?.error?.message || "SUI coin tx build failed");

        const suiSig = signSuiTx(txBytes, privKey);
        const execRes = await postJson(nodeUrl, {
          jsonrpc: "2.0", id: 1, method: "sui_executeTransactionBlock",
          params: [txBytes, [suiSig], { showEffects: true }, "WaitForLocalExecution"],
        });
        hash = execRes?.result?.digest || "";
        if (!hash) throw new Error(execRes?.error?.message || "SUI token transfer failed");

        // ── Solana: SPL token transfer via Token Program ─────────────────────
      } else if (family === "solana") {
        if (!chainKeys?.solana) throw new Error("Unlock wallet with mnemonic to sign Solana token transfers");
        const privKey = chainKeys.solana.privateKey;
        const fromPubkey = chainKeys.solana.address;

        // Find source token account owned by sender
        const srcRes = await postJson(nodeUrl, { jsonrpc: "2.0", id: 1, method: "getTokenAccountsByOwner", params: [fromPubkey, { mint: token.address }, { encoding: "jsonParsed" }] }).catch(() => null);
        const fromTokenAccount = srcRes?.result?.value?.[0]?.pubkey;
        if (!fromTokenAccount) throw new Error(`No ${token.symbol} token account found for your wallet`);

        // Find destination token account owned by recipient
        const dstRes = await postJson(nodeUrl, { jsonrpc: "2.0", id: 1, method: "getTokenAccountsByOwner", params: [recipient, { mint: token.address }, { encoding: "jsonParsed" }] }).catch(() => null);
        const toTokenAccount = dstRes?.result?.value?.[0]?.pubkey;
        if (!toTokenAccount) throw new Error(`Recipient has no ${token.symbol} token account. They need to create one first (e.g. via Phantom wallet).`);

        const bhRes = await postJson(nodeUrl, { jsonrpc: "2.0", id: 1, method: "getLatestBlockhash", params: [{ commitment: "finalized" }] });
        const blockhash = bhRes?.result?.value?.blockhash;
        if (!blockhash) throw new Error("Could not get Solana blockhash");

        const signedTx = signSolanaTokenTransfer({ fromTokenAccount, toTokenAccount, amount, recentBlockhash: blockhash }, privKey);
        const sendRes = await postJson(nodeUrl, { jsonrpc: "2.0", id: 1, method: "sendTransaction", params: [signedTx, { encoding: "base58" }] });
        hash = sendRes?.result || "";
        if (!hash) throw new Error(sendRes?.error?.message || "Solana SPL broadcast failed");

        // ── TON: Jetton transfer via wallet v3R2 BOC ─────────────────────────
      } else if (family === "ton") {
        if (!chainKeys?.ton) throw new Error("Unlock wallet with mnemonic to sign TON Jetton transfers");
        const tonWalletAddr = chainKeys.ton.address || chainKeys.ton.rawPubkey || "";
        if (!tonWalletAddr || !tonWalletAddr.startsWith("EQ")) {
          throw new Error("TON wallet address not available. Re-import your mnemonic.");
        }
        setProcessingMessage("Fetching Jetton wallet address...");
        const jettonWalletAddr = await getTonJettonWalletAddress(nodeUrl, tonWalletAddr, token.address);
        if (!jettonWalletAddr) throw new Error("Could not find your Jetton wallet. Ensure you hold this token.");
        setProcessingMessage("Fetching TON seqno...");
        const seqno = await getTonSeqno(nodeUrl, tonWalletAddr);
        const jettonAmount = String(parseUnits(tokenSendForm.amount, token.decimals || 9));
        setProcessingMessage("Building & signing Jetton transfer BOC...");
        const bocBase64 = buildJettonTransferBoc(
          {
            walletAddress: tonWalletAddr,
            jettonWalletAddress: jettonWalletAddr,
            toAddress: tokenSendForm.to.trim(),
            jettonAmount,
            seqno,
          },
          chainKeys.ton.privateKey
        );
        setProcessingMessage("Broadcasting Jetton transfer...");
        hash = await broadcastTonBoc(nodeUrl, bocBase64);

        // ── Starknet: ERC-20 invoke via STARK ECDSA ────────────────────────────
      } else if (family === "starknet") {
        if (!chainKeys?.starknet) throw new Error("Unlock wallet with mnemonic to sign Starknet token transfers");
        const starkAddr = starknetArgentAddress(chainKeys.starknet.publicKey);
        const amtWei = String(parseUnits(tokenSendForm.amount, token.decimals || 18));
        setProcessingMessage("Signing Starknet token transfer...");
        hash = await broadcastStarknetTokenTransfer(
          {
            senderAddress: starkAddr,
            toAddress: tokenSendForm.to.trim(),
            tokenContract: token.address,
            amountWei: amtWei,
            nodeUrl,
          },
          chainKeys.starknet.privateKey
        );

      } else {
        throw new Error(`No token-send adapter is registered for ${currentNetwork.name} (${family})`);
      }

      setProcessingMessage("");

      setStatusModal({
        visible: true,
        title: "Success",
        message: `${token.symbol} Sent Successfully!`,
        type: "success",
        hash: hash
      });

      addTrackedTx({ hash, type: "token_send", symbol: token.symbol, to: tokenSendForm.to.trim(), network: NETWORKS.find((n) => n.id === (token.networkId || activeNetworkId)) || currentNetwork, family });
      recordTelemetry("token_send_success", { network: token.networkId || activeNetworkId, family, hash, token: token.address }).catch(() => {});
      showToast(`${token.symbol} Sent Successfully`, "success");
      rememberActivity({
        type: "token_send",
        From: wallet.address,
        To: tokenSendForm.to.trim(),
        Contract: token.address,
        TxHash: hash,
        Timestamp: Math.floor(Date.now() / 1000),
        Status: "success",
        Value: tokenSendForm.amount,
        Symbol: token.symbol
      });
      setTokenSendForm(initialTokenSendForm);
      setSelectedTokenForSend(null);
      refreshSingleToken(token.address); // Immediate targeted refresh
      refreshWalletSnapshot(); // Immediate full refresh
      setTimeout(() => refreshWalletSnapshot(), 2000);
      setTimeout(() => refreshWalletSnapshot(), 5000);
      setTimeout(() => refreshWalletSnapshot(), 10000);
    } catch (e) {
      setProcessingMessage("");
      recordError("token_send", e, { network: token.networkId || activeNetworkId, family, token: token.address }).catch(() => {});
      setStatusModal({
        visible: true,
        title: "Failed",
        message: e.message || "Token transfer failed",
        type: "error",
        hash: ""
      });
    } finally {
      setBusy(false);
      setBusyAction("");
      setSelectedTokenForSend(null);
    }
  }

  async function refreshFactory() {
    try {
      const factory = await nodeCurrentFactory(nodeUrl);
      if (factory?.address) {
        setFactoryAddress(factory.address);
      } else {
        setFactoryAddress("");
      }
      setStatus(factory?.address ? `Factory: ${shortAddress(factory.address)}` : "No canonical factory found");
    } catch (e) {
      setStatus(e.message || "Factory lookup failed");
    }
  }

  async function saveBridgeChainAdmin() {
    const apiKey = bridgeAdminApiKey.trim();
    const payload = {
      id: bridgeChainForm.id.trim() || bridgeChainForm.chainId.trim(),
      name: bridgeChainForm.name.trim(),
      chain_id: bridgeChainForm.chainId.trim(),
      family: bridgeChainForm.family.trim() || "evm",
      adapter: bridgeChainForm.adapter.trim() || bridgeChainForm.family.trim() || "evm",
      rpc: bridgeChainForm.rpc.trim(),
      bridge_address: bridgeChainForm.bridgeAddress.trim(),
      lock_address: bridgeChainForm.lockAddress.trim(),
      explorer_url: bridgeChainForm.explorerUrl.trim(),
      native_symbol: bridgeChainForm.nativeSymbol.trim() || "BNB",
      enabled: Boolean(bridgeChainForm.enabled),
      supports_public: Boolean(bridgeChainForm.supportsPublic),
      supports_private: Boolean(bridgeChainForm.supportsPrivate),
    };
    if (!payload.chain_id || !payload.name || !payload.family || !payload.adapter) {
      setStatus("Fill chain id, name, family and adapter");
      return;
    }
    if (payload.family === "evm" && (!payload.rpc || !payload.bridge_address || !payload.lock_address)) {
      setStatus("EVM chains need rpc, bridge address and lock address");
      return;
    }
    setBusy(true);
    setBusyAction("bridgeChainSave");
    try {
      await nodeBridgeChainUpsert(nodeUrl, payload, apiKey);
      await loadBridgeChains();
      setStatus(`Bridge chain saved: ${payload.name}`);
    } catch (e) {
      setStatus(e.message || "Bridge chain save failed");
    } finally {
      setBusy(false);
      setBusyAction("");
    }
  }

  async function removeBridgeChainAdmin() {
    const apiKey = bridgeAdminApiKey.trim();
    const chainId = bridgeChainForm.id.trim() || bridgeChainForm.chainId.trim() || bridgeChainId;
    if (!chainId) {
      setStatus("Enter a chain id to remove");
      return;
    }
    setBusy(true);
    setBusyAction("bridgeChainRemove");
    try {
      await nodeBridgeChainRemove(nodeUrl, { id: chainId }, apiKey);
      await loadBridgeChains();
      setStatus(`Bridge chain removed: ${chainId}`);
    } catch (e) {
      setStatus(e.message || "Bridge chain remove failed");
    } finally {
      setBusy(false);
      setBusyAction("");
    }
  }

  async function saveBridgeTokenAdmin() {
    const apiKey = bridgeTokenAdminApiKey.trim();
    const chainId = bridgeTokenAdminForm.chainId.trim() || bridgeChainId;
    const sourceToken = bridgeTokenAdminForm.sourceToken.trim();
    const lqdToken = bridgeTokenAdminForm.lqdToken.trim();
    if (!chainId || !sourceToken || !lqdToken) {
      setStatus("Fill chain id, source token and LQD token");
      return;
    }
    setBusy(true);
    setBusyAction("bridgeTokenSave");
    try {
      const chain = bridgeChains.find((item) => String(item.id || "").toLowerCase() === String(chainId).toLowerCase())
        || bridgeChains.find((item) => String(item.chain_id || "").toLowerCase() === String(chainId).toLowerCase());
      await nodeBridgeTokenUpsert(nodeUrl, {
        chain_id: chainId,
        family: bridgeTokenAdminForm.family.trim() || chain?.family || "evm",
        chain_name: chain?.name || "",
        source_token: sourceToken,
        target_chain_id: "lqd",
        target_chain_name: "LQD",
        target_token: lqdToken,
        bsc_token: sourceToken,
        lqd_token: lqdToken,
        name: bridgeTokenAdminForm.name.trim(),
        symbol: bridgeTokenAdminForm.symbol.trim(),
        decimals: bridgeTokenAdminForm.decimals.trim(),
      }, apiKey);
      setStatus(`Bridge token saved on ${chainId}`);
      await refreshWalletSnapshot();
    } catch (e) {
      setStatus(e.message || "Bridge token save failed");
    } finally {
      setBusy(false);
      setBusyAction("");
    }
  }

  async function removeBridgeTokenAdmin() {
    const apiKey = bridgeTokenAdminApiKey.trim();
    const chainId = bridgeTokenAdminForm.chainId.trim() || bridgeChainId;
    const sourceToken = bridgeTokenAdminForm.sourceToken.trim();
    const lqdToken = bridgeTokenAdminForm.lqdToken.trim();
    if (!chainId || (!sourceToken && !lqdToken)) {
      setStatus("Fill chain id and either source token or LQD token");
      return;
    }
    setBusy(true);
    try {
      await nodeBridgeTokenRemove(nodeUrl, {
        chain_id: chainId,
        source_token: sourceToken,
        lqd_token: lqdToken,
      }, apiKey);
      setStatus(`Bridge token removed on ${chainId}`);
      await refreshWalletSnapshot();
    } catch (e) {
      setStatus(e.message || "Bridge token remove failed");
    } finally {
      setBusy(false);
    }
  }

  async function deployBuiltinAction() {
    if (!wallet?.address || !wallet?.privateKey) {
      setStatus("Unlock wallet first");
      return;
    }
    if (!walletHasGasBalance()) {
      setStatus("Claim faucet first: this wallet has 0 LQD for gas");
      return;
    }
    const validationError = validateBuiltinDeployForm();
    if (validationError) {
      setStatus(validationError);
      return;
    }
    setBusy(true);
    setBusyAction("deployBuiltin");
    setProcessingMessage("Deploying Builtin Contract...");
    try {
      const res = await nodeDeployBuiltin(nodeUrl, {
        template: deployForm.template,
        owner: wallet.address,
        private_key: wallet.privateKey,
        gas: Number(deployForm.gas || 500000),
        init_args: builtinInitArgs(),
      });
      if (deployForm.template === "dex_factory" && res?.address) {
        setFactoryAddress(res.address);
      }
      const contractAddr = res?.address || "";
      if (contractAddr) {
        setCallForm((prev) => ({ ...prev, contract: contractAddr }));
        setInspectForm({ address: contractAddr });
        const tokenLikeTemplates = ["lqd20", "bridge_token"];
        const candidates = tokenLikeTemplates.includes(deployForm.template)
          ? [contractAddr]
          : deployForm.template === "dex_swap"
            ? [deployForm.tokenA, deployForm.tokenB]
            : [];
        await importDetectedTokens(candidates, wallet.address, "deploy");
        setProcessingMessage("");
        setStatusModal({
          visible: true,
          title: "Contract Deployed",
          message: `Your contract is live!\nAddress: ${contractAddr}`,
          type: "success",
          hash: contractAddr,
          copyLabel: "Copy Contract"
        });
      }
      rememberActivity({
        type: "deploy",
        From: wallet.address,
        To: contractAddr || "New Contract",
        TxHash: res?.tx_hash || "",
        Timestamp: Math.floor(Date.now() / 1000),
        Status: "success",
      });
      setStatus(`Deployed ${deployForm.template}: ${shortAddress(contractAddr)}`);
    } catch (e) {
      setProcessingMessage("");
      setStatusModal({
        visible: true,
        title: "Deployment Failed",
        message: e.message || "Template deployment failed",
        type: "error",
        hash: ""
      });
      setStatus(e.message || "Builtin deploy failed");
    } finally {
      setBusy(false);
      setBusyAction("");
    }
  }

  async function compileAction() {
    if (!customSource.trim()) {
      setStatus("Enter source code");
      return;
    }
    setBusy(true);
    setBusyAction("compile");
    setProcessingMessage("Compiling Contract...");
    try {
      // Normalize line endings and remove non-ASCII hidden characters that cause Go syntax errors
      const normalizedSource = customSource
        .replace(/\r\n/g, "\n")
        .replace(/\r/g, "\n")
        .replace(/[^\x00-\x7F]/g, "");
      let res;
      if (compileType === "goplugin") {
        res = await nodeCompilePlugin(nodeUrl, normalizedSource);
        if (!res?.success) throw new Error(res?.error || "Plugin compilation failed");

        const uri = `${FileSystem.cacheDirectory || ""}lqd-mobile-plugin.so`;
        await FileSystem.writeAsStringAsync(uri, res.binary, { encoding: "base64" });
        setCompiledPluginUri(uri);
        setCompiledBinary(res.binary);
        setCompiledPluginSize(Number(res.size || 0));
        setStatusModal({
          visible: true,
          title: "Plugin Compiled",
          message: `Go Plugin compiled successfully (${res.size || 0} bytes). Ready to deploy.`,
          type: "success",
          hash: ""
        });
      } else {
        res = await nodeCompile(nodeUrl, {
          type: compileType,
          source: normalizedSource,
        });
        if (!res?.success && !res?.binary && !res?.bytecode) throw new Error(res?.error || "Compilation failed");
        setCompiledBinary(res.binary || res.bytecode || null);
        setCompiledPluginSize(Number(res.size || 0));
        setStatusModal({
          visible: true,
          title: "Compile Success",
          message: `Contract compiled successfully (${res.size || 0} bytes). You can now deploy it.`,
          type: "success",
          hash: ""
        });
      }
    } catch (e) {
      setStatus(e.message || "Compile failed");
      setStatusModal({
        visible: true,
        title: "Compile Failed",
        message: e.message || "Compilation failed. Please check your source code syntax.",
        type: "error",
        hash: ""
      });
    } finally {
      setBusy(false);
      setBusyAction("");
      setProcessingMessage("");
    }
  }

  async function deployCompiledAction() {
    if (!wallet?.address || !wallet?.privateKey) {
      setStatus("Unlock wallet first");
      return;
    }
    if (!compiledBinary) {
      setStatus("Compile source first");
      return;
    }
    if (!walletHasGasBalance()) {
      setStatus("Claim faucet first: this wallet has 0 LQD for gas");
      return;
    }
    setBusy(true);
    setBusyAction("deployCompiled");
    setProcessingMessage("Deploying Compiled Contract...");
    try {
      const formData = new FormData();
      const type = compileType === "goplugin" ? "plugin" : compileType;
      formData.append("type", type);
      formData.append("owner", wallet.address);
      formData.append("private_key", wallet.privateKey);
      formData.append("gas", "500000");

      if (compileType === "goplugin") {
        formData.append("contract_file", {
          uri: compiledPluginUri,
          name: "contract.so",
          type: "application/octet-stream",
        });
      } else {
        // For other types, create a temporary file from bytecode/binary
        const uri = `${FileSystem.cacheDirectory || ""}contract.lqd`;
        await FileSystem.writeAsStringAsync(uri, compiledBinary, { encoding: "base64" });
        formData.append("contract_file", {
          uri,
          name: "contract.lqd",
          type: "application/octet-stream",
        });
      }

      const res = await nodeDeployContract(nodeUrl, formData);
      const contractAddr = res?.contract_address || res?.ContractAddress || res?.address || "";
      setProcessingMessage("");
      if (contractAddr) {
        setCallForm((prev) => ({ ...prev, contract: contractAddr }));
        setInspectForm({ address: contractAddr });
        setStatusModal({
          visible: true,
          title: "Contract Deployed",
          message: `Your contract is live!\nAddress: ${contractAddr}`,
          type: "success",
          hash: contractAddr,
          copyLabel: "Copy Contract"
        });
      }
      rememberActivity({
        type: "deploy",
        From: wallet.address,
        To: contractAddr || "New Contract",
        TxHash: res?.tx_hash || "",
        Timestamp: Math.floor(Date.now() / 1000),
        Status: "success",
      });
      setStatus(`Contract deployed: ${shortAddress(contractAddr || "")}`);
      setCompiledBinary(null);
    } catch (e) {
      setProcessingMessage("");
      setStatusModal({
        visible: true,
        title: "Deployment Failed",
        message: e.message || "Custom deployment failed",
        type: "error",
        hash: ""
      });
      setStatus(e.message || "Deploy failed");
    } finally {
      setBusy(false);
      setBusyAction("");
    }
  }

  async function loadAbiAction() {
    const addr = callForm.contract.trim();
    if (!isLikelyAddress(addr)) {
      setStatus("Enter a valid contract address");
      return;
    }
    setBusy(true);
    setBusyAction("loadAbi");
    try {
      const data = await nodeContractAbi(nodeUrl, addr);
      const abi = Array.isArray(data) ? data : (data.entries || data.abi || data.functions || []);
      setCallAbi(abi);
      setStatus(`ABI loaded: ${abi.length} function(s)`);
    } catch (e) {
      setCallAbi([]);
      setStatus("No ABI found — using manual mode");
    } finally {
      setBusy(false);
      setBusyAction("");
    }
  }

  async function callContractAction(isWrite) {
    if (!wallet?.address || !wallet?.privateKey) {
      setStatus("Unlock wallet first");
      return;
    }
    const addr = callForm.contract.trim();
    if (!isLikelyAddress(addr)) {
      setStatus("Enter a valid contract address");
      return;
    }

    let fnName = "";
    let args = [];

    if (callSelectedFnIdx != null && callAbi[callSelectedFnIdx]) {
      const fnDef = callAbi[callSelectedFnIdx];
      fnName = fnDef.name;
      args = (fnDef.inputs || []).map((input, i) => callArgs[input.name] || callArgs[`arg${i}`] || "");
    } else {
      fnName = callForm.functionName.trim();
      args = callForm.args.split(",").map((s) => s.trim()).filter(Boolean);
    }

    if (!fnName) {
      setStatus("Select a function or enter name manually");
      return;
    }

    if (isWrite && !walletHasGasBalance()) {
      setStatus("Claim faucet first: this wallet has 0 LQD for gas");
      return;
    }

    setBusy(true);
    setBusyAction(isWrite ? "callContractWrite" : "callContractRead");
    setProcessingMessage(isWrite ? `Executing ${fnName}...` : `Reading ${fnName}...`);
    try {
      let res;
      if (isWrite) {
        let estimatedGas = 2000000;
        try {
          const gasRes = await nodeEstimateGas(nodeUrl, {
            address: addr,
            fn: fnName,
            args,
            caller: wallet.address,
            value: callForm.value || "0"
          });
          if (gasRes?.gas_limit) {
            estimatedGas = Math.ceil(Number(gasRes.gas_limit) * 1.2);
          }
        } catch { /* fallback */ }

        res = await walletContractTx(walletUrl, {
          address: wallet.address,
          contract_address: addr,
          function: fnName,
          args,
          value: callForm.value || "0",
          gas: Number(callForm.gas) || estimatedGas,
          gas_price: Number(callForm.gasPrice || bridgeBaseFee || 10),
          private_key: wallet.privateKey,
        });
        const hash = res?.tx_hash || res?.TxHash || res?.hash || "";
        if (!hash) throw new Error(res?.error || "Transaction failed");

        setStatusModal({
          visible: true,
          title: "Success",
          message: `${fnName} Submitted Successfully!`,
          type: "success",
          hash: hash
        });
        rememberActivity({
          type: "contract",
          From: wallet.address,
          To: addr,
          TxHash: hash,
          Timestamp: Math.floor(Date.now() / 1000),
          Status: "success",
          Data: fnName
        });
        setTimeout(() => refreshWalletSnapshot(), 1000);
        setTimeout(() => refreshWalletSnapshot(), 6000);
      } else {
        res = await nodeCallContract(nodeUrl, { address: addr, fn: fnName, args, caller: wallet.address });
        const output = res?.result ?? res?.output ?? res?.data ?? res?.val ?? JSON.stringify(res);
        setStatusModal({
          visible: true,
          title: "Call Result",
          message: `Result:\n${output}`,
          type: "success",
          hash: ""
        });
      }
    } catch (e) {
      setStatusModal({
        visible: true,
        title: "Call Failed",
        message: e.message || "Contract call failed",
        type: "error",
        hash: ""
      });
    } finally {
      setBusy(false);
      setBusyAction("");
      setProcessingMessage("");
    }
  }

  async function inspectContractAction() {
    const addr = inspectForm.address.trim();
    if (!isLikelyAddress(addr)) {
      setStatus("Enter a valid contract address");
      return;
    }
    setBusy(true);
    setBusyAction("inspectContract");
    try {
      const [abiData, storageData, eventsData] = await Promise.all([
        nodeContractAbi(nodeUrl, addr).catch(() => null),
        nodeContractStorage(nodeUrl, addr).catch(() => null),
        getJson(`${nodeUrl}/contract/events?address=${encodeURIComponent(addr)}`).catch(() => null),
      ]);

      const abi = Array.isArray(abiData) ? abiData : (abiData?.entries || abiData?.abi || abiData?.functions || []);
      const storage = storageData?.State?.storage ?? storageData?.State ?? storageData ?? {};
      const events = Array.isArray(eventsData) ? eventsData : (eventsData?.events || []);

      setInspectData({ abi, storage });
      setExplorerEvents(events);
      setStatus(`Loaded contract ${shortAddress(addr)}`);
    } catch (e) {
      setStatus(e.message || "Contract inspect failed");
    } finally {
      setBusy(false);
      setBusyAction("");
    }
  }

  // NOTE: openFromScan is defined earlier (single definition) — handles send, token, bridge, import flows.

  async function addNetworkAction() {
    if (!networkForm.name.trim() || !networkForm.chainId.trim() || !networkForm.nodeUrl.trim() || !networkForm.walletUrl.trim()) {
      setStatus("Fill network name, chainId, nodeUrl and walletUrl");
      return;
    }
    const net = {
      id: networkForm.chainId.trim(),
      chainId: networkForm.chainId.trim(),
      name: networkForm.name.trim(),
      symbol: networkForm.symbol.trim() || "LQD",
      nodeUrl: normalizeUrl(networkForm.nodeUrl),
      walletUrl: normalizeUrl(networkForm.walletUrl),
      explorerUrl: normalizeUrl(networkForm.explorerUrl || explorerUrl),
      aggregatorUrl,
    };
    setNetworks((prev) => mergeUniqueByKey(prev, [net], "id"));
    setActiveNetworkId(net.id);
    setNetworkForm(initialNetworkForm);
    setStatus(`Added network ${net.name}`);
  }

  async function switchNetworkAction(id) {
    setActiveNetworkId(id);
    setStatus(`Network switched`);
    if (wallet?.address) {
      setTimeout(() => {
        refreshWalletSnapshot().catch(() => { });
      }, 250);
    }
  }

  async function removeNetworkAction(id) {
    if (id === DEFAULT_NETWORKS[0].id) {
      setStatus("Cannot remove default network");
      return;
    }
    setNetworks((prev) => prev.filter((item) => item.id !== id));
    if (activeNetworkId === id) {
      setActiveNetworkId(DEFAULT_NETWORKS[0].id);
    }
  }

  async function deployFreshFactoryIfMissing() {
    if (!wallet?.address || !wallet?.privateKey) {
      setStatus("Unlock wallet first");
      return;
    }
    if (factoryAddress) {
      setStatus("Factory already configured");
      return;
    }
    setDeployForm((prev) => ({ ...prev, template: "dex_factory" }));
    await deployBuiltinAction();
  }

  async function saveBackupToClipboard() {
    const backup = {
      vault: vaultRecord,
      networks,
      activeNetworkId,
      endpoints,
      watchlist,
      activity,
      factoryAddress,
      bridgeChainId,
      createdAt: new Date().toISOString(),
      version: 1,
    };
    const json = JSON.stringify(backup, null, 2);
    await Clipboard.setStringAsync(json);
    setBackupText(json);
    setStatus("Backup copied to clipboard");
  }

  async function restoreBackupFromText() {
    const parsed = (() => {
      try {
        return JSON.parse(backupText || "{}");
      } catch {
        return null;
      }
    })();
    if (!parsed) {
      setStatus("Invalid backup JSON");
      return;
    }
    if (parsed.vault) {
      await saveJSON(STORAGE_KEYS.vault, parsed.vault);
      setVaultRecord(parsed.vault);
    }
    if (Array.isArray(parsed.networks)) {
      setNetworks(parsed.networks);
    }
    if (parsed.activeNetworkId) {
      setActiveNetworkId(parsed.activeNetworkId);
    }
    if (parsed.endpoints) {
      setEndpoints((prev) => ({ ...prev, ...parsed.endpoints }));
    }
    if (Array.isArray(parsed.watchlist)) {
      setWatchlist(parsed.watchlist);
    }
    if (Array.isArray(parsed.activity)) {
      setActivity(parsed.activity);
    }
    if (parsed.factoryAddress) {
      setFactoryAddress(parsed.factoryAddress);
    }
    if (parsed.bridgeChainId) {
      setBridgeChainId(String(parsed.bridgeChainId));
    }
    setStatus("Backup restored");
  }

  async function clearLocalWallet() {
    await removeItem(STORAGE_KEYS.vault);
    await removeItem(STORAGE_KEYS.biometricVault);
    setVaultRecord(null);
    setWallet(null);
    setUnlockPassword("");
    setStatus("Local wallet cleared");
  }

  useEffect(() => {
    if (!wallet?.address) return;
    setNetworkHealth((prev) => ({ ...prev, state: "checking", message: `Checking ${currentNetwork.name}` }));
    refreshWalletSnapshot().catch(() => { });
  }, [wallet?.address, activeNetworkId, currentNetwork.name]);

  useEffect(() => {
    if (!wallet?.address) return;
    loadBridgeChains().catch(() => { });
    loadBridgeFamilies().catch(() => { });
  }, [wallet?.address, nodeUrl]);

  const currentTokens = useMemo(() => {
    const base = (watchlist || []).filter(t => t.networkId === activeNetworkId);
    if (!removedTokens.length) return base;
    const removedSet = new Set(
      removedTokens
        .filter(r => r.networkId === activeNetworkId && r.address)
        .map(r => String(r.address).toLowerCase())
    );
    return base.filter(t => !t.address || !removedSet.has(String(t.address).toLowerCase()));
  }, [watchlist, activeNetworkId, removedTokens]);

  const visibleTokens = useMemo(() => {
    if (showHiddenTokens) return currentTokens;
    if (!hiddenTokens.length) return currentTokens;
    const hiddenSet = new Set(
      hiddenTokens
        .filter(h => h.networkId === activeNetworkId && h.address)
        .map(h => String(h.address).toLowerCase())
    );
    return currentTokens.filter(t => !t.address || !hiddenSet.has(String(t.address).toLowerCase()));
  }, [currentTokens, hiddenTokens, showHiddenTokens, activeNetworkId]);

  const hiddenCurrentTokens = useMemo(() => {
    if (!hiddenTokens.length) return [];
    const hiddenSet = new Set(
      hiddenTokens
        .filter(h => h.networkId === activeNetworkId && h.address)
        .map(h => String(h.address).toLowerCase())
    );
    return currentTokens.filter(t => t.address && hiddenSet.has(String(t.address).toLowerCase()));
  }, [currentTokens, hiddenTokens, activeNetworkId]);

  const currentNfts = useMemo(
    () => (nfts || []).filter((nft) => nft?.networkId === activeNetworkId),
    [nfts, activeNetworkId]
  );

  const currentActivity = useMemo(() => {
    const holder = activeAddress || wallet?.address || "";
    if (!holder) return [];
    return (activity || []).filter((item) => txTouchesAddress(item, holder));
  }, [activity, activeAddress, wallet?.address]);

  const lastRefreshLabel = useMemo(() => {
    if (!lastWalletRefresh) return "Not refreshed yet";
    const seconds = Math.max(1, Math.floor((Date.now() - lastWalletRefresh) / 1000));
    if (seconds < 60) return `Updated ${seconds}s ago`;
    const minutes = Math.floor(seconds / 60);
    return `Updated ${minutes}m ago`;
  }, [lastWalletRefresh, isRefreshing]);

  if (booting) {
    return (
      <SafeAreaView style={styles.safe}>
        <StatusBar style="light" />
        <View style={styles.centerScreen}>
          <Text style={styles.heroTitle}>LQD Mobile Wallet</Text>
          <Text style={styles.heroText}>Loading vault, networks and on-chain state…</Text>
        </View>
      </SafeAreaView>
    );
  }

  if (!vaultRecord) {
    return (
      <SafeAreaView style={styles.safe}>
        <StatusBar style="light" />
        <ScrollView contentContainerStyle={styles.scrollPad}>
          <Text style={styles.heroTitle}>LQD Mobile Wallet</Text>
          <Text style={styles.heroText}>MetaMask-style mobile wallet for the PoDL ecosystem.</Text>
          <Card title="Create wallet" subtitle="Generate a fresh vault and save it locally with your password.">
            <Field label="Password" value={createForm.password} onChangeText={(v) => setCreateForm((p) => ({ ...p, password: v }))} secureTextEntry placeholder="Set a strong password" />
            <Field label="Confirm password" value={createForm.confirm} onChangeText={(v) => setCreateForm((p) => ({ ...p, confirm: v }))} secureTextEntry placeholder="Repeat password" />
            <Button label={busyAction === "createWallet" ? "Creating…" : "Create Wallet"} onPress={createWalletAction} disabled={busy} />
            <Text style={styles.helperText}>The wallet server returns the address, private key and mnemonic. The mobile vault encrypts them locally with your password.</Text>
          </Card>

          <Card title="Import from mnemonic" subtitle="Restore an existing wallet phrase.">
            <Field label="Mnemonic" value={importMnemonicForm.mnemonic} onChangeText={(v) => setImportMnemonicForm((p) => ({ ...p, mnemonic: v }))} multiline numberOfLines={4} placeholder="twelve or twenty-four words…" />
            <Field label="Password" value={importMnemonicForm.password} onChangeText={(v) => setImportMnemonicForm((p) => ({ ...p, password: v }))} secureTextEntry placeholder="Password for local vault" />
            <Button label={busyAction === "importMnemonic" ? "Importing…" : "Import Mnemonic"} onPress={importMnemonicAction} disabled={busy} />
          </Card>

          <Card title="Import private key" subtitle="Paste a raw private key if you already have one.">
            <Field label="Private key" value={importPkForm.privateKey} onChangeText={(v) => setImportPkForm((p) => ({ ...p, privateKey: v }))} placeholder="0x..." />
            <Field label="Password" value={importPkForm.password} onChangeText={(v) => setImportPkForm((p) => ({ ...p, password: v }))} secureTextEntry placeholder="Password for local vault" />
            <Button label={busyAction === "importPrivateKey" ? "Importing…" : "Import Private Key"} onPress={importPrivateKeyAction} disabled={busy} />
          </Card>

          {tab !== "browser" ? <Text style={styles.statusText}>{status}</Text> : null}
        </ScrollView>
      </SafeAreaView>
    );
  }

  if (!walletVisible || !wallet) {
    return (
      <SafeAreaView style={styles.safe}>
        <StatusBar style="light" />
        <ScrollView contentContainerStyle={styles.scrollPad}>
          <Text style={styles.heroTitle}>Wallet Locked</Text>
          <Text style={styles.heroText}>Unlock the vault to access native send, token send, contracts and bridge flows.</Text>
          <Card title="Unlock" subtitle={shortAddress(vaultRecord.address)}>
            <Field label="Password" value={unlockPassword} onChangeText={setUnlockPassword} secureTextEntry placeholder="Enter vault password" />
            <View style={styles.inlineButtons}>
              <Button label={busyAction === "unlockPassword" ? "Unlocking…" : "Unlock Wallet"} onPress={unlockWallet} disabled={busy} />
              <Button label={busyAction === "unlockBiometric" ? "Unlocking…" : "Biometric Unlock"} onPress={unlockWithBiometrics} secondary disabled={busy || !biometricAvailable || !biometricEnabled} />
            </View>
            <Text style={styles.helperText}>{biometricEnabled && biometricAvailable ? "Biometric unlock is available on this device." : "Biometric unlock is not enabled or not available."}</Text>
          </Card>
          <Text style={styles.statusText}>{status}</Text>
        </ScrollView>
      </SafeAreaView>
    );
  }

  return (
    <SafeAreaView style={styles.safe}>
      <StatusBar style="light" />
      {statusModal.visible && (
        <Modal transparent animationType="fade" visible={statusModal.visible} onRequestClose={() => setStatusModal({ ...statusModal, visible: false })}>
          <View style={{ flex: 1, backgroundColor: 'rgba(7, 10, 21, 0.95)', justifyContent: 'center', alignItems: 'center', padding: scale(20) }}>
            <View style={{ width: '100%', backgroundColor: '#161b33', borderRadius: scale(28), padding: scale(30), alignItems: 'center', borderWidth: 1, borderColor: statusModal.type === 'success' ? 'rgba(16, 185, 129, 0.2)' : 'rgba(239, 68, 68, 0.2)', shadowColor: "#000", shadowOffset: { width: 0, height: 20 }, shadowOpacity: 0.5, shadowRadius: 30, elevation: 20 }}>
              <View style={{ width: scale(80), height: scale(80), borderRadius: scale(40), backgroundColor: statusModal.type === 'success' ? 'rgba(16, 185, 129, 0.1)' : 'rgba(239, 68, 68, 0.1)', justifyContent: 'center', alignItems: 'center', marginBottom: scale(24) }}>
                {statusModal.type === 'success' ? (
                  <View style={{ width: scale(36), height: scale(18), borderLeftWidth: 5, borderBottomWidth: 5, borderColor: '#10b981', transform: [{ rotate: '-45deg' }], marginTop: -8 }} />
                ) : (
                  <Text style={{ color: '#ef4444', fontSize: scale(48), fontWeight: '300' }}>×</Text>
                )}
              </View>
              <Text style={{ color: '#f4f7ff', fontSize: scale(24), fontWeight: '800', marginBottom: scale(12), textAlign: 'center' }}>{statusModal.title}</Text>
              <Text style={{ color: '#9aa5ca', fontSize: scale(16), textAlign: 'center', marginBottom: scale(30), lineHeight: scale(24) }}>{statusModal.message}</Text>

              <View style={{ width: '100%', gap: scale(14) }}>
                {!!statusModal.hash && (
                  <Button
                    label={statusModal.copyLabel || (statusModal.hash.startsWith("0x") ? "Copy Hash" : "Copy Result")}
                    onPress={() => {
                      Clipboard.setStringAsync(statusModal.hash);
                      showToast(`${statusModal.copyLabel || "Result"} copied`, "success");
                    }}
                    primary
                  />
                )}
                <Button label="Close" onPress={() => setStatusModal({ ...statusModal, visible: false })} secondary />
              </View>
            </View>
          </View>
        </Modal>
      )}

      {!legalRiskAccepted && (
        <Modal transparent animationType="fade" visible={!legalRiskAccepted}>
          <View style={{ flex: 1, backgroundColor: 'rgba(7, 10, 21, 0.94)', justifyContent: 'center', alignItems: 'center', padding: scale(20) }}>
            <View style={{ width: '100%', backgroundColor: '#161b33', borderRadius: scale(26), padding: scale(24), borderWidth: 1, borderColor: '#273152' }}>
              <Text style={{ color: '#f4f7ff', fontSize: scale(22), fontWeight: '900', marginBottom: scale(10) }}>Testnet Wallet Notice</Text>
              <Text style={{ color: '#a8b3d8', fontSize: scale(14), lineHeight: scale(21), marginBottom: scale(16) }}>
                This app is for testnet use. Do not store mainnet funds until final legal review, public security audit, app-store review, and mainnet release notes are complete.
              </Text>
              <Text style={{ color: '#fbbf24', fontSize: scale(13), lineHeight: scale(19), marginBottom: scale(18) }}>
                Always verify addresses, token contracts, dApp origins, and transaction details before approving.
              </Text>
              <Button label="I Understand" onPress={() => setLegalRiskAccepted(true)} />
            </View>
          </View>
        </Modal>
      )}

      <KeyboardAvoidingView behavior={Platform.OS === "ios" ? "padding" : undefined} style={{ flex: 1 }}>
        <Modal visible={receiveVisible} transparent animationType="fade" onRequestClose={() => setReceiveVisible(false)}>
          <View style={styles.modalBackdrop}>
            <View style={styles.modalCard}>
              <Text style={styles.cardTitle}>Receive {currentNetwork.symbol}</Text>
              {activeAddress ? (
                <>
                  <Text style={styles.cardSubtitle}>Share this address or scan the QR code.</Text>
                  {!!watchAddresses[activeNetworkId] && (
                    <Text style={{ color: '#f59e0b', fontSize: scale(11), textAlign: 'center', marginBottom: scale(6) }}>
                      ⚠️ Watch-only — not derived from your key
                    </Text>
                  )}
                  <View style={styles.qrWrap}>
                    <QRCode value={activeAddress} size={210} color="#f4f7ff" backgroundColor="#151b31" />
                  </View>
                  <Text style={styles.inspectBox}>{activeAddress}</Text>
                  <View style={styles.inlineButtons}>
                    <Button label="Copy" onPress={async () => { await Clipboard.setStringAsync(activeAddress); showToast("Address copied", "success"); }} compact />
                    <Button label="Share" onPress={async () => { await Share.share({ message: activeAddress }); }} compact secondary />
                    {!!watchAddresses[activeNetworkId] && (
                      <Button label="Clear" onPress={() => { clearWatchAddress(); }} compact danger />
                    )}
                    <Button label="Close" onPress={() => setReceiveVisible(false)} compact danger />
                  </View>
                </>
              ) : (
                <>
                  <Text style={[styles.cardSubtitle, { color: '#f59e0b', marginBottom: scale(10) }]}>
                    Address not derivable — paste your {currentNetwork.name} address to track balance and tokens.
                  </Text>
                  <Text style={{ color: '#717da4', fontSize: scale(12), textAlign: 'center', lineHeight: scale(18), marginBottom: scale(14) }}>
                    {currentNetwork.name} uses Ed25519 keys which cannot be derived from your EVM key. Import your mnemonic into a native wallet (e.g. Phantom for Solana, NEAR Wallet for NEAR) to get your address, then paste it below.
                  </Text>
                  <TextInput
                    style={[styles.input, { marginBottom: scale(10) }]}
                    placeholder={`Paste ${currentNetwork.name} address (${SEND_ADDR_PLACEHOLDER[currentNetwork.family] || "..."})`}
                    placeholderTextColor="#717da4"
                    value={watchAddrInput}
                    onChangeText={setWatchAddrInput}
                    autoCapitalize="none"
                    autoCorrect={false}
                  />
                  <View style={styles.inlineButtons}>
                    <Button label="Save & Use" onPress={saveWatchAddress} compact primary />
                    <Button label="Close" onPress={() => { setWatchAddrInput(""); setReceiveVisible(false); }} compact danger />
                  </View>
                </>
              )}
            </View>
          </View>
        </Modal>

        {/* Scan Choice Modal — pick what to scan for */}
        <Modal visible={scanChoiceVisible} transparent animationType="slide" onRequestClose={() => setScanChoiceVisible(false)}>
          <TouchableOpacity activeOpacity={1} onPress={() => setScanChoiceVisible(false)} style={{ flex: 1, backgroundColor: 'rgba(7,10,21,0.9)', justifyContent: 'flex-end' }}>
            <View style={{ backgroundColor: '#161b33', borderTopLeftRadius: scale(32), borderTopRightRadius: scale(32), padding: scale(24), paddingBottom: scale(40), borderWidth: 1, borderColor: '#273152' }}>
              <View style={{ width: scale(40), height: scale(4), backgroundColor: '#273152', borderRadius: 2, alignSelf: 'center', marginBottom: scale(20) }} />
              <Text style={{ color: '#f4f7ff', fontSize: scale(18), fontWeight: '800', textAlign: 'center', marginBottom: scale(6) }}>Scan QR Code</Text>
              <Text style={{ color: '#8899cc', fontSize: scale(13), textAlign: 'center', marginBottom: scale(24) }}>What would you like to scan?</Text>
              {[
                { label: `Send ${currentNetwork?.symbol || 'Coin'}`, sub: 'Fill the native send address', target: 'native', icon: '📤' },
                { label: 'Send Token', sub: 'Fill the token send address', target: 'token', icon: '💎' },
              ].map(({ label, sub, target, icon }) => (
                <TouchableOpacity
                  key={target}
                  onPress={() => { setScanChoiceVisible(false); setTimeout(() => scanWithCamera(target), 200); }}
                  style={{ flexDirection: 'row', alignItems: 'center', backgroundColor: '#1e2540', borderRadius: scale(14), padding: scale(14), marginBottom: scale(10), borderWidth: 1, borderColor: '#273152' }}
                >
                  <Text style={{ fontSize: scale(22), marginRight: scale(12) }}>{icon}</Text>
                  <View style={{ flex: 1 }}>
                    <Text style={{ color: '#f4f7ff', fontSize: scale(15), fontWeight: '700' }}>{label}</Text>
                    <Text style={{ color: '#8899cc', fontSize: scale(12), marginTop: scale(2) }}>{sub}</Text>
                  </View>
                  <Text style={{ color: '#8a78ff', fontSize: scale(18) }}>›</Text>
                </TouchableOpacity>
              ))}
              <TouchableOpacity onPress={() => setScanChoiceVisible(false)} style={{ marginTop: scale(8), padding: scale(12), alignItems: 'center' }}>
                <Text style={{ color: '#8899cc', fontSize: scale(14) }}>Cancel</Text>
              </TouchableOpacity>
            </View>
          </TouchableOpacity>
        </Modal>

        <Modal visible={scannerVisible} transparent animationType="slide" onRequestClose={() => setScannerVisible(false)}>
          <View style={styles.scannerBackdrop}>
            <View style={styles.scannerHeader}>
              <Text style={styles.cardTitle}>Scan QR</Text>
              <Text style={styles.cardSubtitle}>Scan an address, payment QR, or lqdwallet deep link.</Text>
              <View style={styles.inlineButtons}>
                <Button label="Close" onPress={() => setScannerVisible(false)} compact danger />
              </View>
            </View>
            <View style={styles.scannerBody}>
              {cameraPermission?.granted ? (
                <CameraView
                  style={StyleSheet.absoluteFill}
                  facing="back"
                  barcodeScannerSettings={{ barcodeTypes: ["qr"] }}
                  onBarcodeScanned={({ data }) => {
                    if (!data) return;
                    setScannerVisible(false);
                    // Use ref (not closure) so we always call the freshest openFromScan
                    const handler = scanHandlerRef.current;
                    setTimeout(() => handler?.(data), 300);
                  }}
                />
              ) : (
                <View style={styles.cameraFallback}>
                  <Text style={styles.heroText}>Camera permission is required for scanning QR codes.</Text>
                  <Button label="Grant Permission" onPress={requestCameraPermission} />
                </View>
              )}
            </View>
          </View>
        </Modal>

        {/* Key Export Modal */}
        <Modal visible={keyExportVisible && Boolean(chainKeys)} transparent animationType="fade" onRequestClose={() => setKeyExportVisible(false)}>
          <View style={styles.modalBackdrop}>
            <View style={[styles.modalCard, { maxHeight: '90%' }]}>
              <Text style={styles.cardTitle}>All Chain Keys</Text>
              <Text style={[styles.cardSubtitle, { color: '#ef4444', marginBottom: 8 }]}>⚠️ Keep private keys secret. Never share them.</Text>
              <ScrollView style={{ maxHeight: 380 }}>
                {chainKeys && exportKeyInfo(chainKeys).map((entry) => (
                  <View key={entry.chain} style={{ marginBottom: 12, borderWidth: 1, borderColor: '#273152', borderRadius: 8, padding: 10 }}>
                    <Text style={{ color: '#8a78ff', fontWeight: '700', marginBottom: 4 }}>{entry.chain.toUpperCase()}</Text>
                    <Text style={{ color: '#9aa5ca', fontSize: 11, marginBottom: 2 }}>Address:</Text>
                    <Text selectable style={{ color: '#e2e8f0', fontSize: 11, fontFamily: 'monospace' }}>{entry.address}</Text>
                    {entry.evmAddress && (
                      <>
                        <Text style={{ color: '#9aa5ca', fontSize: 11, marginTop: 4, marginBottom: 2 }}>EVM Address:</Text>
                        <Text selectable style={{ color: '#e2e8f0', fontSize: 11, fontFamily: 'monospace' }}>{entry.evmAddress}</Text>
                      </>
                    )}
                    <Text style={{ color: '#9aa5ca', fontSize: 11, marginTop: 4, marginBottom: 2 }}>Private Key:</Text>
                    <Text selectable style={{ color: '#fbbf24', fontSize: 11, fontFamily: 'monospace' }}>{entry.privateKey}</Text>
                    <TouchableOpacity
                      onPress={async () => { await Clipboard.setStringAsync(entry.privateKey); showToast(`${entry.chain} key copied`, "success"); }}
                      style={{ marginTop: 4, alignSelf: 'flex-start', backgroundColor: '#1e293b', borderRadius: 4, paddingHorizontal: 8, paddingVertical: 4 }}
                    >
                      <Text style={{ color: '#8a78ff', fontSize: 11 }}>Copy Key</Text>
                    </TouchableOpacity>
                  </View>
                ))}
              </ScrollView>
              <Button label="Close" onPress={() => setKeyExportVisible(false)} secondary />
            </View>
          </View>
        </Modal>

        <Modal visible={showMnemonic && Boolean(wallet?.mnemonic)} transparent animationType="fade" onRequestClose={() => setShowMnemonic(false)}>
          <View style={styles.modalBackdrop}>
            <View style={styles.modalCard}>
              <Text style={styles.cardTitle}>Save your mnemonic</Text>
              <Text style={styles.cardSubtitle}>Keep this offline. Anyone with it can control the wallet.</Text>
              <Text style={styles.inspectBox}>{wallet?.mnemonic || "No mnemonic available."}</Text>
              <View style={styles.inlineButtons}>
                <Button
                  label="Copy Mnemonic"
                  onPress={async () => {
                    if (wallet?.mnemonic) {
                      await Clipboard.setStringAsync(wallet.mnemonic);
                      setStatus("Mnemonic copied");
                    }
                  }}
                  compact
                />
                <Button label="Close" onPress={() => setShowMnemonic(false)} compact secondary />
              </View>
            </View>
          </View>
        </Modal>

        {tab !== "browser" && (
          <View style={[styles.topBar, { paddingTop: scale(40) }]}>
            <View style={{ flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', paddingHorizontal: scale(16), paddingBottom: scale(6) }}>
              <View style={{ flexDirection: 'row', gap: scale(8), flex: 1 }}>
                <TouchableOpacity onPress={() => setReceiveVisible(true)} style={styles.topActionPill}>
                  <Text style={styles.topActionPillText}>Receive</Text>
                </TouchableOpacity>
                <TouchableOpacity onPress={() => setScanChoiceVisible(true)} style={styles.topActionPill}>
                  <Text style={styles.topActionPillText}>Scan</Text>
                </TouchableOpacity>
              </View>

              <View style={{ flexDirection: 'row', alignItems: 'center', gap: scale(12), flex: 1, justifyContent: 'flex-end' }}>
                <TouchableOpacity onPress={() => refreshWalletSnapshot()} style={[styles.topRefreshBtn, isRefreshing && { opacity: 0.7 }]} disabled={isRefreshing}>
                  <Text style={{ color: '#fff', fontSize: scale(12), fontWeight: '700' }}>{isRefreshing ? "Refreshing" : "Refresh"}</Text>
                </TouchableOpacity>
              </View>
            </View>
          </View>
        )}

        {/* Metamask-style Network Selector Modal */}
        <Modal visible={networkModalVisible} transparent animationType="slide" onRequestClose={() => setNetworkModalVisible(false)}>
          <TouchableOpacity activeOpacity={1} onPress={() => setNetworkModalVisible(false)} style={{ flex: 1, backgroundColor: 'rgba(7, 10, 21, 0.9)', justifyContent: 'flex-end' }}>
            <View style={{ backgroundColor: '#161b33', borderTopLeftRadius: scale(32), borderTopRightRadius: scale(32), padding: scale(24), paddingBottom: scale(40), borderWidth: 1, borderColor: '#273152' }}>
              <View style={{ width: scale(40), height: scale(4), backgroundColor: '#273152', borderRadius: 2, alignSelf: 'center', marginBottom: scale(20) }} />
              <Text style={{ color: '#f4f7ff', fontSize: scale(18), fontWeight: '800', textAlign: 'center', marginBottom: scale(24) }}>Select Network</Text>

              <ScrollView style={{ maxHeight: scale(400) }}>
                {NETWORKS.map((n) => (
                  <TouchableOpacity
                    key={n.id}
                    onPress={() => { setActiveNetworkId(n.id); setNetworkModalVisible(false); }}
                    style={{
                      flexDirection: 'row',
                      alignItems: 'center',
                      padding: scale(16),
                      backgroundColor: activeNetworkId === n.id ? 'rgba(138, 120, 255, 0.1)' : '#0f152a',
                      borderRadius: scale(16),
                      marginBottom: scale(10),
                      borderWidth: 1,
                      borderColor: activeNetworkId === n.id ? '#8a78ff' : '#1b2342'
                    }}
                  >
                    <View style={{ width: scale(36), height: scale(36), borderRadius: 18, backgroundColor: n.color + '20', justifyContent: 'center', alignItems: 'center', marginRight: scale(14) }}>
                      <Text style={{ fontSize: scale(18) }}>{n.icon}</Text>
                    </View>
                    <View style={{ flex: 1 }}>
                      <Text style={{ color: '#fff', fontWeight: 'bold', fontSize: scale(14) }}>{n.name}</Text>
                      <Text style={{ color: '#717da4', fontSize: scale(11) }}>{n.symbol} Chain</Text>
                    </View>
                    {activeNetworkId === n.id && (
                      <View style={{ width: 8, height: 8, borderRadius: 4, backgroundColor: '#10b981' }} />
                    )}
                  </TouchableOpacity>
                ))}
              </ScrollView>

              <Button label="Close" onPress={() => setNetworkModalVisible(false)} secondary style={{ marginTop: scale(16) }} />
            </View>
          </TouchableOpacity>
        </Modal>

        {/* Global Action Menu */}
        <Modal visible={isMainMenuVisible} transparent animationType="fade" onRequestClose={() => setIsMainMenuVisible(false)}>
          <TouchableOpacity activeOpacity={1} onPress={() => setIsMainMenuVisible(false)} style={{ flex: 1, backgroundColor: 'rgba(7, 10, 21, 0.8)' }}>
            <View style={{ position: 'absolute', top: scale(300), right: scale(16), backgroundColor: '#161b33', borderRadius: scale(16), width: scale(200), padding: scale(8), borderWidth: 1, borderColor: '#273152', shadowColor: "#000", shadowOffset: { width: 0, height: 10 }, shadowOpacity: 0.3, shadowRadius: 20, elevation: 10 }}>
              <TouchableOpacity
                onPress={() => { if (currentNetwork.id?.startsWith("lqd")) { setIsMainMenuVisible(false); setFaucetVisible(true); } }}
                style={[styles.menuItem, !currentNetwork.id?.startsWith("lqd") && { opacity: 0.35 }]}
                disabled={!currentNetwork.id?.startsWith("lqd")}
              >
                <Text style={styles.menuItemIcon}>🚰</Text>
                <Text style={styles.menuItemText}>Faucet{!currentNetwork.id?.startsWith("lqd") ? " (LQD only)" : ""}</Text>
              </TouchableOpacity>
              <TouchableOpacity onPress={() => { setIsMainMenuVisible(false); setTokenImportVisible(true); }} style={styles.menuItem}>
                <Text style={styles.menuItemIcon}>➕</Text>
                <Text style={styles.menuItemText}>Import Token</Text>
              </TouchableOpacity>
              <TouchableOpacity onPress={() => { setIsMainMenuVisible(false); setNftImportVisible(true); }} style={styles.menuItem}>
                <Text style={styles.menuItemIcon}>🖼️</Text>
                <Text style={styles.menuItemText}>Import NFT</Text>
              </TouchableOpacity>
              <TouchableOpacity onPress={() => { setIsMainMenuVisible(false); autoDiscoverTokensAction(); }} style={styles.menuItem}>
                <Text style={styles.menuItemIcon}>🔄</Text>
                <Text style={styles.menuItemText}>Auto Detect</Text>
              </TouchableOpacity>
              <TouchableOpacity onPress={() => { setIsMainMenuVisible(false); setTab("liquidity"); }} style={[styles.menuItem, { borderBottomWidth: 0 }]}>
                <Text style={styles.menuItemIcon}>💧</Text>
                <Text style={styles.menuItemText}>Liquidity</Text>
              </TouchableOpacity>
            </View>
          </TouchableOpacity>
        </Modal>

        <ScrollView
          style={{ flex: tab === "browser" ? 0 : 1, display: tab === "browser" ? "none" : "flex" }}
          scrollEnabled={tab !== "browser"}
          keyboardShouldPersistTaps="handled"
          contentContainerStyle={[styles.mainScroll]}
        >
          {tab === "home" && (
            <View style={styles.mmHome}>
              {/* Network Selector Moved Here */}
              <TouchableOpacity
                onPress={() => setNetworkModalVisible(true)}
                style={styles.walletNetworkPill}
              >
                <View style={[styles.walletHealthDot, { backgroundColor: healthDotColor() }]} />
                <Text style={{ color: '#fff', fontSize: scale(15), fontWeight: '800' }}>{currentNetwork.name}</Text>
                <Text style={{ color: '#8a78ff', marginLeft: scale(8), fontSize: scale(12) }}>▼</Text>
              </TouchableOpacity>
              <Text style={styles.walletHealthText}>
                {isRefreshing ? "Refreshing wallet..." : networkHealth.message}
                {networkHealth.latencyMs ? ` · ${networkHealth.latencyMs}ms` : ""}
              </Text>

              {/* Account Overview */}
              <View style={styles.mmAccountCard}>
                <View style={styles.mmAccountHeader}>
                  <View style={{ width: scale(32), height: scale(32), borderRadius: 16, backgroundColor: '#3b2f72', justifyContent: 'center', alignItems: 'center' }}>
                    <Text style={{ color: '#fff', fontSize: scale(14), fontWeight: 'bold' }}>L</Text>
                  </View>
                  <TouchableOpacity onPress={copyAddress} style={styles.mmAddressPill}>
                    <Text style={styles.mmAddressText}>{shortAddress(activeAddress)}</Text>
                    <Text style={{ color: '#8a78ff', marginLeft: scale(6), fontSize: scale(10) }}>▼</Text>
                  </TouchableOpacity>
                  <TouchableOpacity onPress={() => setScanChoiceVisible(true)} style={styles.mmHeaderIcon}>
                    <Text style={{ fontSize: scale(18) }}>📷</Text>
                  </TouchableOpacity>
                </View>

                <View style={styles.mmBalanceContainer}>
                  <Text style={styles.mmBalanceValue}>{formatUnits(nativeBalance, currentNetwork.decimals || 8, 4)} {currentNetwork.symbol}</Text>
                  <Text style={styles.mmBalanceFiat}>Price unavailable</Text>
                  <Text style={styles.walletRefreshMeta}>{lastRefreshLabel}</Text>
                  {!!balanceLoadError && <Text style={styles.walletWarningText}>{balanceLoadError}</Text>}
                </View>

                <View style={styles.mmActionRow}>
                  <MMActionBtn label="Receive" icon="📥" onPress={() => setReceiveVisible(true)} />
                  <MMActionBtn label="Send" icon="📤" onPress={() => setSendVisible(true)} />
                  <MMActionBtn label="Refresh" icon="↻" onPress={() => refreshWalletSnapshot()} />
                  <MMActionBtn label="Tokens" icon="💎" onPress={() => setTokenImportVisible(true)} />
                </View>
              </View>

              {/* Sub Tabs: Tokens / NFTs / Activity */}
              <View style={[styles.mmSubTabRow, { alignItems: 'center' }]}>
                <TouchableOpacity onPress={() => setHomeSubTab("tokens")} style={[styles.mmSubTab, homeSubTab === "tokens" && styles.mmSubTabActive]}>
                  <Text style={[styles.mmSubTabText, homeSubTab === "tokens" && styles.mmSubTabTextActive]}>ASSETS</Text>
                </TouchableOpacity>
                <TouchableOpacity onPress={() => setHomeSubTab("nfts")} style={[styles.mmSubTab, homeSubTab === "nfts" && styles.mmSubTabActive]}>
                  <Text style={[styles.mmSubTabText, homeSubTab === "nfts" && styles.mmSubTabTextActive]}>NFTS</Text>
                </TouchableOpacity>
                <TouchableOpacity onPress={() => setHomeSubTab("activity")} style={[styles.mmSubTab, homeSubTab === "activity" && styles.mmSubTabActive]}>
                  <Text style={[styles.mmSubTabText, homeSubTab === "activity" && styles.mmSubTabTextActive]}>ACTIVITY</Text>
                </TouchableOpacity>
                <TouchableOpacity onPress={() => setIsMainMenuVisible(true)} style={[styles.hamburgerBtn, { width: scale(50), height: scale(50), marginRight: 0 }]}>
                  <View style={styles.hamburgerLine} />
                  <View style={[styles.hamburgerLine, { width: scale(14) }]} />
                  <View style={styles.hamburgerLine} />
                </TouchableOpacity>
              </View>

              <View style={styles.mmListContainer}>
                {homeSubTab === "tokens" ? (
                  <View style={styles.tokenList}>
                    {/* Native Asset */}
                    <View style={styles.mmTokenRow}>
                      <View style={styles.mmTokenIcon}>
                        <Text style={{ fontSize: scale(16) }}>🪙</Text>
                      </View>
                      <View style={{ flex: 1 }}>
                        <Text style={styles.mmTokenName}>{currentNetwork.name}</Text>
                        <Text style={styles.mmTokenSymbol}>{formatUnits(nativeBalance, currentNetwork.decimals || 8, 4)} {currentNetwork.symbol}</Text>
                      </View>
                      <View style={{ alignItems: 'flex-end' }}>
                        <Text style={styles.walletTokenBadge}>Native</Text>
                      </View>
                    </View>
                    {/* Custom Tokens */}
                    {visibleTokens.map((token, i) => {
                      const logoUri = tokenLogoUri(token);
                      return (
                        <TouchableOpacity
                          key={i}
                          onPress={() => setSelectedTokenForSend(token)}
                          onLongPress={() => setTokenActionTarget(token)}
                          delayLongPress={400}
                          style={styles.mmTokenRow}
                        >
                          <View style={styles.mmTokenIcon}>
                            {logoUri ? (
                              <Image source={{ uri: logoUri }} style={styles.mmTokenLogo} resizeMode="cover" />
                            ) : (
                              <Text style={{ fontSize: scale(16) }}>{isGenericTokenMeta(token) ? "?" : "💎"}</Text>
                            )}
                          </View>
                          <View style={{ flex: 1 }}>
                            <View style={styles.mmTokenTitleRow}>
                              <Text style={styles.mmTokenName} numberOfLines={1}>{isGenericTokenMeta(token) ? "Unknown Token" : token.name}</Text>
                              {isOfficialToken(token) && <Text style={styles.walletOfficialBadge}>Official</Text>}
                            </View>
                            <Text style={styles.mmTokenSymbol}>{formatUnits(token.balance, token.decimals || 8, 2)} {token.symbol}</Text>
                            <View style={styles.walletTokenMarketRow}>
                              <Text style={styles.walletTokenPrice}>{tokenPriceLabel(token)}</Text>
                              <Text style={[styles.walletTokenChange, tokenChangeStyle(token)]}>{tokenChangeLabel(token)}</Text>
                            </View>
                            <Text style={styles.walletTokenMeta}>
                              {shortAddress(token.address)} · {token.networkId === activeNetworkId ? currentNetwork.name : token.networkId}
                            </Text>
                            {isGenericTokenMeta(token) && (
                              <Text style={styles.walletWarningText}>Metadata incomplete. Re-import with symbol/decimals if needed.</Text>
                            )}
                          </View>
                          <Text style={{ color: '#8a78ff', fontSize: scale(18), paddingHorizontal: scale(4) }}>⋮</Text>
                        </TouchableOpacity>
                      );
                    })}

                    {/* Hidden tokens toggle */}
                    {hiddenCurrentTokens.length > 0 && (
                      <TouchableOpacity
                        onPress={() => setShowHiddenTokens(v => !v)}
                        style={{ flexDirection: 'row', alignItems: 'center', paddingVertical: scale(10), paddingHorizontal: scale(4) }}
                      >
                        <Text style={{ color: '#717da4', fontSize: scale(12) }}>
                          {showHiddenTokens ? '▲ Hide hidden tokens' : `▼ Show ${hiddenCurrentTokens.length} hidden token${hiddenCurrentTokens.length > 1 ? 's' : ''}`}
                        </Text>
                      </TouchableOpacity>
                    )}
                  </View>
                ) : homeSubTab === "nfts" ? (
                  <View style={styles.nftGrid}>
                    {!currentNfts.length ? (
                      <View style={styles.nftEmptyState}>
                        <Text style={styles.nftEmptyIcon}>🖼️</Text>
                        <Text style={styles.mmTokenName}>No NFTs imported on {currentNetwork.name}</Text>
                        <Text style={styles.mmEmptyText}>Import a collection address and token ID. EVM ownership and metadata are verified from the active RPC.</Text>
                        <Button label="Import NFT" onPress={() => setNftImportVisible(true)} compact />
                      </View>
                    ) : currentNfts.map((nft) => (
                      <View key={nftKey(nft)} style={styles.nftCard}>
                        {nft.image ? (
                          <Image source={{ uri: nft.image }} style={styles.nftImage} resizeMode="cover" />
                        ) : (
                          <View style={[styles.nftImage, styles.nftImageFallback]}>
                            <Text style={styles.nftEmptyIcon}>🖼️</Text>
                          </View>
                        )}
                        <View style={styles.nftCardBody}>
                          <Text style={styles.mmTokenName} numberOfLines={1}>{nft.name || `NFT #${nft.tokenId}`}</Text>
                          <Text style={styles.mmTokenSymbol}>#{nft.tokenId} · {nft.symbol || "NFT"}</Text>
                          <Text style={styles.walletTokenMeta}>{shortAddress(nft.address)} · {nft.contractVerified ? "Contract verified" : "Manual metadata"}</Text>
                          {nft.ownershipVerified && <Text style={styles.nftOwnedBadge}>✓ Ownership verified</Text>}
                          {nft.ownershipMismatch && <Text style={styles.walletWarningText}>Watch-only · another address owns this token</Text>}
                          <TouchableOpacity onPress={() => removeNftAction(nft)} style={styles.nftRemoveButton}>
                            <Text style={styles.nftRemoveText}>Remove from wallet view</Text>
                          </TouchableOpacity>
                        </View>
                      </View>
                    ))}
                    {!!currentNfts.length && <Button label="Import another NFT" onPress={() => setNftImportVisible(true)} compact secondary />}
                  </View>
                ) : (
                  <View style={styles.activityList}>
                    {!currentActivity.length ? (
                      <Text style={styles.mmEmptyText}>No transactions yet</Text>
                    ) : (
                      currentActivity.map((item, idx) => <ActivityRow key={idx} item={item} onPress={setSelectedTxStory} />)
                    )}
                  </View>
                )}
              </View>
            </View>
          )}

          {/* Native Send Modal — network-aware */}
          <Modal visible={sendVisible} transparent animationType="slide" onRequestClose={() => setSendVisible(false)}>
            {(() => {
              const sendFam = currentNetwork.id === "lqd" ? "lqd" : (currentNetwork.family || "evm");
              const isLqdSend = sendFam === "lqd";
              const isEvmSend = !isLqdSend && (sendFam === "evm" || sendFam === "harmony");
              const supportedNativeSendFamilies = new Set(["lqd", "evm", "harmony", "tron", "sei", "cosmos", "cosmos-testnet", "injective", "solana", "near", "aptos", "sui", "utxo", "litecoin", "ton", "starknet"]);
              const canSignSend = supportedNativeSendFamilies.has(sendFam);
              const addrPlaceholder = SEND_ADDR_PLACEHOLDER[sendFam] || "...";
              return (
                <View style={{ flex: 1, backgroundColor: 'rgba(7, 10, 21, 0.95)', justifyContent: 'flex-end' }}>
                  <View style={{ backgroundColor: '#161b33', borderTopLeftRadius: scale(32), borderTopRightRadius: scale(32), padding: scale(24), paddingBottom: scale(40) }}>
                    <View style={{ width: scale(40), height: scale(4), backgroundColor: '#273152', borderRadius: 2, alignSelf: 'center', marginBottom: scale(20) }} />

                    {/* Header with network badge */}
                    <View style={{ flexDirection: 'row', alignItems: 'center', marginBottom: scale(6) }}>
                      <Text style={{ color: '#f4f7ff', fontSize: scale(20), fontWeight: '800', flex: 1 }}>Send {currentNetwork.symbol}</Text>
                      <View style={{ backgroundColor: canSignSend ? 'rgba(16,185,129,0.1)' : 'rgba(251,191,36,0.1)', borderRadius: scale(10), paddingHorizontal: scale(8), paddingVertical: scale(3), borderWidth: 1, borderColor: canSignSend ? 'rgba(16,185,129,0.3)' : 'rgba(251,191,36,0.3)' }}>
                        <Text style={{ color: canSignSend ? '#10b981' : '#fbbf24', fontSize: scale(10), fontWeight: '800' }}>{isLqdSend ? '✓ LQD' : (isEvmSend ? '✓ EVM' : `${canSignSend ? '✓ ' : ''}${String(sendFam).toUpperCase()}`)}</Text>
                      </View>
                    </View>
                    <Text style={{ color: '#717da4', fontSize: scale(13), marginBottom: scale(16) }}>{currentNetwork.name} · {currentNetwork.symbol}</Text>

                    {!canSignSend && (
                      <View style={{ backgroundColor: 'rgba(251,191,36,0.08)', borderRadius: scale(12), padding: scale(12), marginBottom: scale(16), borderWidth: 1, borderColor: 'rgba(251,191,36,0.25)' }}>
                        <Text style={{ color: '#fbbf24', fontSize: scale(12), fontWeight: '700', marginBottom: scale(4) }}>⚠️ Unknown signing family</Text>
                        <Text style={{ color: '#9aa5ca', fontSize: scale(11), lineHeight: scale(17) }}>
                          This custom network does not declare a wallet signing adapter. Add a supported family before sending funds.
                        </Text>
                      </View>
                    )}

                    <Field
                      label={`Recipient Address (${currentNetwork.name})`}
                      value={sendForm.to}
                      onChangeText={(v) => setSendForm(p => ({ ...p, to: v }))}
                      placeholder={addrPlaceholder}
                    />
                    <Field
                      label={`Amount (${currentNetwork.symbol})`}
                      value={sendForm.amount}
                      onChangeText={(v) => setSendForm(p => ({ ...p, amount: v }))}
                      keyboardType="decimal-pad"
                      placeholder="0.0"
                    />

                    <View style={styles.inlineButtons}>
                      <Button label="Scan QR" onPress={() => scanWithCamera("native")} compact secondary />
                      <Button label="Paste" onPress={() => pasteClipboardTo((v) => setSendForm(p => ({ ...p, to: v })))} compact secondary />
                    </View>

                    <View style={{ marginVertical: scale(16), padding: scale(14), backgroundColor: '#0f152a', borderRadius: scale(14), gap: scale(8) }}>
                      <View style={{ flexDirection: 'row', justifyContent: 'space-between' }}>
                        <Text style={{ color: '#717da4', fontSize: scale(12) }}>Balance</Text>
                        <Text style={{ color: '#f4f7ff', fontSize: scale(12), fontWeight: '700' }}>{formatUnits(nativeBalance, currentNetwork.decimals || 8, 4)} {currentNetwork.symbol}</Text>
                      </View>
                      <View style={{ flexDirection: 'row', justifyContent: 'space-between' }}>
                        <Text style={{ color: '#717da4', fontSize: scale(12) }}>Network Fee</Text>
                        <Text style={{ color: nativeFeeInfo.error ? '#f59e0b' : '#10b981', fontSize: scale(12), fontWeight: '700' }}>
                          {nativeFeeInfo.loading ? "Estimating..." : nativeFeeInfo.display}
                        </Text>
                      </View>
                    </View>

                    <Button
                      label={busyAction === "sendNative" ? "Sending..." : (canSignSend ? "Confirm Send" : "Validate Address")}
                      onPress={sendAction}
                      disabled={busy}
                    />
                    <View style={{ height: scale(14) }} />
                    <Button label="Cancel" onPress={() => { setSendVisible(false); setSendForm(initialSendForm); }} secondary />
                  </View>
                </View>
              );
            })()}
          </Modal>

          {/* Token Import Modal — network-aware */}
          <Modal visible={tokenImportVisible} transparent animationType="slide" onRequestClose={() => { setTokenImportVisible(false); setTokenImportForm(initialTokenImportForm); setTokenPreviewMeta(null); }}>
            {(() => {
              const fam = currentNetwork.family || "evm";
              const ui = FAMILY_TOKEN_UI[fam] || DEFAULT_FAMILY_UI;
              const isEvm = fam === "evm";
              const FAMILY_ICON = { evm: "🔷", solana: "☀️", cosmos: "⚛️", "cosmos-testnet": "⚛️", sei: "🔴", injective: "🌀", near: "Ⓝ", tron: "🔴", ton: "💎", aptos: "🌀", sui: "💧", starknet: "✨", harmony: "💠", utxo: "₿", litecoin: "Ł" };
              const icon = FAMILY_ICON[fam] || "🪙";
              return (
                <View style={{ flex: 1, backgroundColor: 'rgba(7, 10, 21, 0.95)', justifyContent: 'center', padding: scale(20) }}>
                  <View style={{ backgroundColor: '#161b33', borderRadius: scale(24), padding: scale(20), borderWidth: 1, borderColor: '#273152' }}>
                    {/* Header */}
                    <View style={{ flexDirection: 'row', alignItems: 'center', marginBottom: scale(6) }}>
                      <Text style={{ fontSize: scale(22), marginRight: scale(10) }}>{icon}</Text>
                      <View style={{ flex: 1 }}>
                        <Text style={{ color: '#f4f7ff', fontSize: scale(18), fontWeight: '800' }}>Import Token</Text>
                        <View style={{ flexDirection: 'row', alignItems: 'center', marginTop: scale(2) }}>
                          <View style={{ width: 6, height: 6, borderRadius: 3, backgroundColor: isNodeOnline ? '#10b981' : '#ef4444', marginRight: scale(6) }} />
                          <Text style={{ color: '#8a78ff', fontSize: scale(12), fontWeight: '700' }}>{currentNetwork.name}</Text>
                          <Text style={{ color: '#717da4', fontSize: scale(11), marginLeft: scale(6) }}>· {String(fam).toUpperCase()}</Text>
                        </View>
                      </View>
                    </View>

                    {!ui.supported ? (
                      /* Unsupported chain notice */
                      <View style={{ backgroundColor: 'rgba(239,68,68,0.08)', borderRadius: scale(14), padding: scale(16), marginTop: scale(12), borderWidth: 1, borderColor: 'rgba(239,68,68,0.2)' }}>
                        <Text style={{ color: '#ef4444', fontSize: scale(14), fontWeight: '700', marginBottom: scale(6) }}>⛔ Not Supported</Text>
                        <Text style={{ color: '#9aa5ca', fontSize: scale(13), lineHeight: scale(20) }}>{ui.unsupportedMsg}</Text>
                      </View>
                    ) : (
                      <View style={{ marginTop: scale(12) }}>
                        {/* Step 1: Contract address input */}
                        <View style={{ backgroundColor: 'rgba(138,120,255,0.06)', borderRadius: scale(10), paddingHorizontal: scale(12), paddingVertical: scale(7), marginBottom: scale(12), borderWidth: 1, borderColor: 'rgba(138,120,255,0.12)' }}>
                          <Text style={{ color: '#9aa5ca', fontSize: scale(12) }}>📋  Paste the token contract address — name & symbol will auto-detect.</Text>
                        </View>

                        <Field
                          label={ui.label}
                          value={tokenImportForm.address}
                          onChangeText={(v) => { setTokenImportForm(p => ({ ...p, address: v })); setTokenPreviewMeta(null); }}
                          placeholder={ui.placeholder}
                        />

                        {/* Loading indicator while fetching */}
                        {tokenPreviewLoading && (
                          <View style={{ flexDirection: 'row', alignItems: 'center', paddingVertical: scale(10), paddingHorizontal: scale(4) }}>
                            <ActivityIndicator size="small" color="#8a78ff" />
                            <Text style={{ color: '#8a78ff', fontSize: scale(13), marginLeft: scale(8) }}>Fetching token info…</Text>
                          </View>
                        )}

                        {/* Step 2: Token preview card (Trust Wallet style) */}
                        {tokenPreviewMeta && !tokenPreviewLoading && (() => {
                          const FALLBACK_SYMBOLS = new Set(["TOKEN", "SPL", "FT", "Jetton", "TRC20", "CW20", "IBC"]);
                          const isRealSymbol = tokenPreviewMeta.symbol && !FALLBACK_SYMBOLS.has(tokenPreviewMeta.symbol);
                          const previewVerified = !!(tokenPreviewMeta.verified && isRealSymbol && tokenPreviewMeta.name);
                          return (
                            <View style={{ backgroundColor: previewVerified ? 'rgba(16,185,129,0.07)' : 'rgba(245,158,11,0.08)', borderRadius: scale(16), padding: scale(16), marginBottom: scale(12), borderWidth: 1, borderColor: previewVerified ? 'rgba(16,185,129,0.2)' : 'rgba(245,158,11,0.22)' }}>
                              <Text style={{ color: previewVerified ? '#10b981' : '#f59e0b', fontSize: scale(12), fontWeight: '700', marginBottom: scale(10), letterSpacing: 0.5 }}>
                                {previewVerified ? '✓  TOKEN VERIFIED' : '⚠  TOKEN DETECTED, VERIFY DETAILS'}
                              </Text>
                              <View style={{ flexDirection: 'row', alignItems: 'center', marginBottom: scale(8) }}>
                                <View style={{ width: scale(44), height: scale(44), borderRadius: scale(22), backgroundColor: previewVerified ? 'rgba(138,120,255,0.15)' : 'rgba(245,158,11,0.12)', alignItems: 'center', justifyContent: 'center', marginRight: scale(14) }}>
                                  <Text style={{ color: '#8a78ff', fontSize: scale(18), fontWeight: '800' }}>{isRealSymbol ? tokenPreviewMeta.symbol.slice(0, 2).toUpperCase() : icon}</Text>
                                </View>
                                <View style={{ flex: 1 }}>
                                  <Text style={{ color: '#f4f7ff', fontSize: scale(18), fontWeight: '800' }}>{isRealSymbol ? tokenPreviewMeta.symbol : 'Manual check needed'}</Text>
                                  <Text style={{ color: '#9aa5ca', fontSize: scale(13), marginTop: scale(2) }}>{tokenPreviewMeta.name || 'Metadata incomplete on this RPC'}</Text>
                                </View>
                              </View>
                              <View style={{ flexDirection: 'row', gap: scale(8) }}>
                                <View style={{ flex: 1, backgroundColor: 'rgba(7,10,21,0.4)', borderRadius: scale(10), padding: scale(10) }}>
                                  <Text style={{ color: '#717da4', fontSize: scale(11), marginBottom: scale(2) }}>DECIMALS</Text>
                                  <Text style={{ color: '#f4f7ff', fontSize: scale(15), fontWeight: '700' }}>{tokenPreviewMeta.decimals ?? '—'}</Text>
                                </View>
                                <View style={{ flex: 1, backgroundColor: 'rgba(7,10,21,0.4)', borderRadius: scale(10), padding: scale(10) }}>
                                  <Text style={{ color: '#717da4', fontSize: scale(11), marginBottom: scale(2) }}>NETWORK</Text>
                                  <Text style={{ color: '#f4f7ff', fontSize: scale(15), fontWeight: '700' }}>{String(fam).toUpperCase()}</Text>
                                </View>
                              </View>
                              {!previewVerified && (
                                <Text style={{ color: '#f5c26b', fontSize: scale(12), marginTop: scale(10), lineHeight: scale(18) }}>
                                  Auto-preview could not verify full metadata from the active RPC set. Review and fill the fields below before importing.
                                </Text>
                              )}
                              {(previewVerified ? !isEvm : true) && (
                                <View style={{ marginTop: scale(10) }}>
                                  <Text style={{ color: '#717da4', fontSize: scale(11), marginBottom: scale(6) }}>Override (optional):</Text>
                                  <Field label="Symbol" value={tokenImportForm.symbol} onChangeText={(v) => setTokenImportForm(p => ({ ...p, symbol: v }))} placeholder={tokenPreviewMeta.symbol || "e.g. USDT"} />
                                  <Field label="Token Name" value={tokenImportForm.name} onChangeText={(v) => setTokenImportForm(p => ({ ...p, name: v }))} placeholder={tokenPreviewMeta.name || "e.g. Tether USD"} />
                                  <Field label="Decimals" value={tokenImportForm.decimals} onChangeText={(v) => setTokenImportForm(p => ({ ...p, decimals: v }))} keyboardType="numeric" placeholder={String(tokenPreviewMeta.decimals ?? getDefaultDecimalsForFamily(fam))} />
                                </View>
                              )}
                            </View>
                          );
                        })()}

                        {/* If preview could not be verified, allow manual metadata entry */}
                        {!tokenPreviewMeta && !tokenPreviewLoading && tokenImportForm.address.trim() !== "" && (
                          <>
                            <Field label="Symbol" value={tokenImportForm.symbol} onChangeText={(v) => setTokenImportForm(p => ({ ...p, symbol: v }))} placeholder="e.g. USDT" />
                            <Field label="Token Name" value={tokenImportForm.name} onChangeText={(v) => setTokenImportForm(p => ({ ...p, name: v }))} placeholder="e.g. Tether USD" />
                            <Field label={`Decimals (default: ${getDefaultDecimalsForFamily(fam)})`} value={tokenImportForm.decimals} onChangeText={(v) => setTokenImportForm(p => ({ ...p, decimals: v }))} keyboardType="numeric" placeholder={String(getDefaultDecimalsForFamily(fam))} />
                          </>
                        )}

                        <View style={{ flexDirection: 'row', gap: scale(10), marginTop: scale(4) }}>
                          <View style={{ flex: 1 }}>
                            <Button
                              label={busyAction === "addToken" ? "Importing…" : "Import Token"}
                              onPress={addTokenAction}
                              disabled={busy || tokenPreviewLoading || !tokenImportForm.address.trim()}
                            />
                          </View>
                          <View style={{ flex: 1 }}>
                            <Button label="Cancel" onPress={() => { setTokenImportVisible(false); setTokenImportForm(initialTokenImportForm); setTokenPreviewMeta(null); }} secondary />
                          </View>
                        </View>
                      </View>
                    )}

                    {!ui.supported && (
                      <View style={{ marginTop: scale(14) }}>
                        <Button label="Close" onPress={() => setTokenImportVisible(false)} secondary />
                      </View>
                    )}
                  </View>
                </View>
              );
            })()}
          </Modal>

          {/* Token Action Sheet — long-press on token */}
          <Modal visible={!!tokenActionTarget} transparent animationType="slide" onRequestClose={() => setTokenActionTarget(null)}>
            <View style={{ flex: 1, backgroundColor: 'rgba(7,10,21,0.7)', justifyContent: 'flex-end' }}>
              {/* Backdrop tap closes sheet */}
              <TouchableOpacity
                style={{ position: 'absolute', top: 0, left: 0, right: 0, bottom: 0 }}
                activeOpacity={1}
                onPress={() => setTokenActionTarget(null)}
              />
              <View style={{ backgroundColor: '#161b33', borderTopLeftRadius: scale(24), borderTopRightRadius: scale(24), padding: scale(20), borderWidth: 1, borderColor: '#273152' }}>
                {tokenActionTarget && (
                  <>
                    <View style={{ flexDirection: 'row', alignItems: 'center', marginBottom: scale(20) }}>
                      <View style={{ width: scale(44), height: scale(44), borderRadius: scale(22), backgroundColor: 'rgba(138,120,255,0.15)', alignItems: 'center', justifyContent: 'center', marginRight: scale(14) }}>
                        <Text style={{ color: '#8a78ff', fontSize: scale(18), fontWeight: '800' }}>{String(tokenActionTarget.symbol || '?').slice(0, 2).toUpperCase()}</Text>
                      </View>
                      <View style={{ flex: 1 }}>
                        <Text style={{ color: '#f4f7ff', fontSize: scale(17), fontWeight: '800' }}>{tokenActionTarget.symbol}</Text>
                        <Text style={{ color: '#9aa5ca', fontSize: scale(13) }}>{tokenActionTarget.name}</Text>
                      </View>
                    </View>

                    {/* Hide / Unhide */}
                    {hiddenTokens.some(h => h?.address && tokenActionTarget?.address && String(h.address).toLowerCase() === String(tokenActionTarget.address).toLowerCase() && h.networkId === tokenActionTarget.networkId) ? (
                      <TouchableOpacity
                        onPress={() => unhideToken(tokenActionTarget)}
                        style={{ flexDirection: 'row', alignItems: 'center', paddingVertical: scale(14), borderTopWidth: 1, borderTopColor: '#273152' }}
                      >
                        <Text style={{ fontSize: scale(20), marginRight: scale(14) }}>👁️</Text>
                        <View>
                          <Text style={{ color: '#f4f7ff', fontSize: scale(15), fontWeight: '600' }}>Unhide Token</Text>
                          <Text style={{ color: '#717da4', fontSize: scale(12) }}>Show this token in your list again</Text>
                        </View>
                      </TouchableOpacity>
                    ) : (
                      <TouchableOpacity
                        onPress={() => hideToken(tokenActionTarget)}
                        style={{ flexDirection: 'row', alignItems: 'center', paddingVertical: scale(14), borderTopWidth: 1, borderTopColor: '#273152' }}
                      >
                        <Text style={{ fontSize: scale(20), marginRight: scale(14) }}>🙈</Text>
                        <View>
                          <Text style={{ color: '#f4f7ff', fontSize: scale(15), fontWeight: '600' }}>Hide Token</Text>
                          <Text style={{ color: '#717da4', fontSize: scale(12) }}>Keep in wallet but hide from list</Text>
                        </View>
                      </TouchableOpacity>
                    )}

                    {/* Remove */}
                    <TouchableOpacity
                      onPress={() => removeToken(tokenActionTarget)}
                      style={{ flexDirection: 'row', alignItems: 'center', paddingVertical: scale(14), borderTopWidth: 1, borderTopColor: '#273152' }}
                    >
                      <Text style={{ fontSize: scale(20), marginRight: scale(14) }}>🗑️</Text>
                      <View>
                        <Text style={{ color: '#ef4444', fontSize: scale(15), fontWeight: '600' }}>Remove Token</Text>
                        <Text style={{ color: '#717da4', fontSize: scale(12) }}>Permanently delete from your wallet</Text>
                      </View>
                    </TouchableOpacity>

                    {/* Cancel */}
                    <TouchableOpacity
                      onPress={() => setTokenActionTarget(null)}
                      style={{ marginTop: scale(8), paddingVertical: scale(14), alignItems: 'center', borderTopWidth: 1, borderTopColor: '#273152' }}
                    >
                      <Text style={{ color: '#9aa5ca', fontSize: scale(15), fontWeight: '600' }}>Cancel</Text>
                    </TouchableOpacity>
                  </>
                )}
              </View>
            </View>
          </Modal>

          {/* NFT Import Modal */}
          <Modal visible={nftImportVisible} transparent animationType="slide" onRequestClose={() => { setNftImportVisible(false); setNftImportForm(initialNftImportForm); }}>
            <View style={{ flex: 1, backgroundColor: 'rgba(7, 10, 21, 0.95)', justifyContent: 'center', padding: scale(20) }}>
              <Card title="Import NFT" subtitle={`Add an NFT on ${currentNetwork.name}. EVM metadata and ownership are checked on-chain.`}>
                {["utxo", "litecoin"].includes(String(currentNetwork.family || "").toLowerCase()) ? (
                  <View style={styles.nftUnsupportedNotice}>
                    <Text style={styles.walletWarningText}>{currentNetwork.name} has no account-based NFT contract standard, so NFT import is unavailable on this network.</Text>
                  </View>
                ) : (
                  <>
                    <Field
                      label="Collection / Contract Address"
                      value={nftImportForm.address}
                      onChangeText={(value) => setNftImportForm((previous) => ({ ...previous, address: value }))}
                      placeholder={(FAMILY_TOKEN_UI[currentNetwork.family] || DEFAULT_FAMILY_UI).placeholder || "Contract or collection address"}
                    />
                    <Field label="Token ID" value={nftImportForm.tokenId} onChangeText={(value) => setNftImportForm((previous) => ({ ...previous, tokenId: value }))} placeholder="e.g. 1" />
                    <Field label="Display Name (optional)" value={nftImportForm.name} onChangeText={(value) => setNftImportForm((previous) => ({ ...previous, name: value }))} placeholder="Fetched automatically when supported" />
                    <Field label="Image URL / IPFS URI (optional)" value={nftImportForm.image} onChangeText={(value) => setNftImportForm((previous) => ({ ...previous, image: value }))} placeholder="ipfs://... or https://..." />
                  </>
                )}
                <View style={styles.inlineButtons}>
                  <Button
                    label={busyAction === "addNft" ? "Verifying..." : "Import NFT"}
                    onPress={importNftAction}
                    disabled={busy || ["utxo", "litecoin"].includes(String(currentNetwork.family || "").toLowerCase())}
                  />
                  <Button label="Cancel" onPress={() => { setNftImportVisible(false); setNftImportForm(initialNftImportForm); }} secondary />
                </View>
              </Card>
            </View>
          </Modal>

          {/* Faucet Modal */}
          <Modal visible={faucetVisible} transparent animationType="slide" onRequestClose={() => setFaucetVisible(false)}>
            <View style={{ flex: 1, backgroundColor: 'rgba(7, 10, 21, 0.95)', justifyContent: 'center', padding: scale(20) }}>
              <Card title={`${currentNetwork.symbol} Faucet`} subtitle={`Claim testnet ${currentNetwork.symbol} for gas and testing.`}>
                <View style={{ alignItems: 'center', marginVertical: scale(20) }}>
                  <Text style={{ color: '#8a78ff', fontSize: scale(48), marginBottom: scale(10) }}>🚰</Text>
                  <Text style={{ color: '#f4f7ff', fontSize: scale(14), textAlign: 'center' }}>You will receive a small amount of native {currentNetwork.symbol} to your address: {shortAddress(activeAddress)}</Text>
                </View>
                <Button label={busyAction === "faucet" ? "Claiming..." : `Claim ${currentNetwork.symbol}`} onPress={async () => { await claimFaucetAction(); setFaucetVisible(false); }} disabled={busy} />
                <Button label="Close" onPress={() => setFaucetVisible(false)} secondary style={{ marginTop: scale(12) }} />
              </Card>
            </View>
          </Modal>

          {selectedTokenForSend ? (() => {
            const tkFam = selectedTokenForSend.family || currentNetwork.family || "evm";
            const isEvmTk = tkFam === "evm" || tkFam === "harmony";
            const tkPlaceholder = SEND_ADDR_PLACEHOLDER[tkFam] || "...";
            return (
              <Card title={`Send ${selectedTokenForSend.symbol}`} subtitle={`${currentNetwork.name} · ${shortAddress(selectedTokenForSend.address)}`}>
                {!isEvmTk && (
                  <View style={{ backgroundColor: 'rgba(251,191,36,0.08)', borderRadius: scale(10), padding: scale(10), marginBottom: scale(10), borderWidth: 1, borderColor: 'rgba(251,191,36,0.2)' }}>
                    <Text style={{ color: '#fbbf24', fontSize: scale(11) }}>🔐 {String(tkFam).toUpperCase()} transfer is signed locally with the network-specific key and broadcast to the active RPC. Confirm the recipient and network before sending.</Text>
                  </View>
                )}
                <Field
                  label={`Recipient (${currentNetwork.name})`}
                  value={tokenSendForm.to}
                  onChangeText={(v) => setTokenSendForm((p) => ({ ...p, to: v }))}
                  placeholder={tkPlaceholder}
                />
                <Field label="Amount" value={tokenSendForm.amount} onChangeText={(v) => setTokenSendForm((p) => ({ ...p, amount: v }))} keyboardType="decimal-pad" placeholder="0.0" />
                <View style={styles.inlineButtons}>
                  <Button label="Scan Recipient" onPress={() => scanWithCamera("token")} compact secondary />
                  <Button label="Paste" onPress={() => pasteClipboardTo((value) => setTokenSendForm((p) => ({ ...p, to: value })))} compact />
                </View>
                <View style={{ marginVertical: scale(12), padding: scale(12), backgroundColor: '#0f152a', borderRadius: scale(12), flexDirection: 'row', justifyContent: 'space-between' }}>
                  <Text style={{ color: '#717da4', fontSize: scale(12) }}>Network Fee</Text>
                  <Text style={{ color: tokenFeeInfo.error ? '#f59e0b' : '#10b981', fontSize: scale(12), fontWeight: '700' }}>
                    {tokenFeeInfo.loading ? "Estimating..." : tokenFeeInfo.display}
                  </Text>
                </View>
                <Button label={busyAction === "sendToken" ? "Sending…" : "Send Token"} onPress={() => sendTokenAction(selectedTokenForSend)} disabled={busy} />
                <Button label="Close" onPress={() => setSelectedTokenForSend(null)} secondary />
              </Card>
            );
          })() : null}

          {tab === "contracts" && (
            <View style={styles.mmHome}>
              <View style={[styles.mmAccountCard, { backgroundColor: '#1e293b' }]}>
                <Text style={{ color: '#f4f7ff', fontSize: scale(20), fontWeight: '800', marginBottom: scale(8) }}>Contract Studio</Text>
                <Text style={{ color: '#94a3b8', fontSize: scale(13), textAlign: 'center' }}>Deploy builtin templates or compile custom source code.</Text>
              </View>

              <Card title="Deploy builtin contract" subtitle="Use one of the chain templates.">
                <View style={styles.templateWrap}>
                  {BUILTIN_TEMPLATES.map((item) => (
                    <Chip key={item.value} label={item.label} active={deployForm.template === item.value} onPress={() => setDeployForm((p) => ({ ...p, template: item.value }))} />
                  ))}
                </View>
                {renderBuiltinTemplateFields()}
                <Field label="Gas" value={deployForm.gas} onChangeText={(v) => setDeployForm((p) => ({ ...p, gas: v }))} keyboardType="numeric" placeholder="500000" />
                <Button label={busyAction === "deployBuiltin" ? "Deploying…" : "Deploy Builtin"} onPress={deployBuiltinAction} disabled={busy} />
                <Button label="Refresh Factory" onPress={refreshFactory} secondary />
              </Card>

              <Card title="Custom Compiler" subtitle="Compile and deploy contract source.">
                <Text style={styles.inspectTitle}>Language / Type</Text>
                <View style={styles.templateWrap}>
                  {['goplugin', 'gocode', 'dsl', 'solidity'].map(t => (
                    <Chip key={t} label={t.toUpperCase()} active={compileType === t} onPress={() => setCompileType(t)} />
                  ))}
                </View>
                <Field label="Source code" value={customSource} onChangeText={setCustomSource} multiline numberOfLines={10} placeholder="Enter source code here..." />
                <View style={styles.inlineButtons}>
                  <Button label={busyAction === "compile" ? "Compiling…" : "Compile Source"} onPress={compileAction} disabled={busy} />
                  <Button label={busyAction === "deployCompiled" ? "Deploying…" : "Deploy Compiled"} onPress={deployCompiledAction} secondary disabled={busy || !compiledBinary} />
                </View>
              </Card>

              <Card title="Call contract" subtitle="Read or write via wallet signed calls.">
                <Field label="Contract address" value={callForm.contract} onChangeText={(v) => { setCallForm((p) => ({ ...p, contract: v })); setCallAbi([]); }} placeholder="0x..." />
                <Button label={busyAction === "loadAbi" ? "Load ABI" : "Load ABI"} onPress={loadAbiAction} disabled={busy} secondary />
                {callAbi.length > 0 && (
                  <View style={styles.sectionGapSmall}>
                    <Text style={styles.inspectTitle}>Select Function</Text>
                    <View style={styles.templateWrap}>
                      {callAbi.map((fn, idx) => (
                        <Chip key={`${fn.name}-${idx}`} label={`${fn.name}(${(fn.inputs || []).length})`} active={callSelectedFnIdx === idx} onPress={() => { setCallSelectedFnIdx(idx); setCallArgs({}); }} />
                      ))}
                    </View>
                    {callSelectedFnIdx !== null && callAbi[callSelectedFnIdx] && (
                      <View style={{ marginTop: scale(20), borderTopWidth: 1, borderTopColor: '#273152', paddingTop: scale(20) }}>
                        <Text style={[styles.inspectTitle, { color: '#8a78ff' }]}>
                          {callAbi[callSelectedFnIdx].name} ({callAbi[callSelectedFnIdx].stateMutability || 'non-payable'})
                        </Text>
                        {(callAbi[callSelectedFnIdx].inputs || []).map((input, i) => {
                          const inpName = input.name || input.Name || `Arg ${i}`;
                          const inpType = input.type || input.Type || "string";
                          const lowerName = inpName.toLowerCase();
                          const lowerType = inpType.toLowerCase();
                          // Smart detection: check if name or type suggests an address
                          const isAddr = lowerType.includes('address') ||
                            lowerName.includes('addr') ||
                            lowerName === 'to' ||
                            lowerName === 'from' ||
                            lowerName === 'owner';

                          return (
                            <View key={`${inpName}-${i}`} style={{ marginBottom: scale(16) }}>
                              <Text style={{ color: '#8a78ff', fontSize: scale(12), fontWeight: 'bold', marginBottom: scale(4) }}>
                                {inpName} ({inpType})
                              </Text>
                              <Field
                                value={callArgs[inpName] || callArgs[`arg${i}`] || ""}
                                onChangeText={(v) => setCallArgs(p => ({ ...p, [inpName]: v, [`arg${i}`]: v }))}
                                placeholder={`e.g. ${isAddr ? '0x...' : '100'}`}
                              />
                            </View>
                          );
                        })}
                        <View style={styles.inlineButtons}>
                          {(() => {
                            const fn = callAbi[callSelectedFnIdx];
                            const name = (fn.name || "").toLowerCase();
                            // Robust Read detection: Standard flags OR common naming patterns
                            const isRead = fn.stateMutability === 'view' ||
                              fn.stateMutability === 'pure' ||
                              fn.constant === true ||
                              name.startsWith('get') ||
                              name.includes('balance') ||
                              name === 'name' ||
                              name === 'symbol' ||
                              name === 'decimals' ||
                              name.includes('total');

                            return (
                              <>
                                <Button
                                  label={busyAction === "call" && !isRead ? "Reading..." : "Read Function"}
                                  onPress={() => callContractAction(false)}
                                  disabled={busy || !isRead}
                                  style={{ flex: 1, opacity: isRead ? 1 : 0.3 }}
                                />
                                <Button
                                  label={busyAction === "call" && isRead ? "Writing..." : "Write Function"}
                                  onPress={() => callContractAction(true)}
                                  disabled={busy || isRead}
                                  secondary
                                  style={{ flex: 1, opacity: !isRead ? 1 : 0.3 }}
                                />
                              </>
                            );
                          })()}
                        </View>
                      </View>
                    )}
                  </View>
                )}
              </Card>

              {/* Contract Inspector / Explorer */}
              <Card title="Contract Inspector" subtitle="Explore ABI, Storage, and Events for any contract.">
                <Field label="Contract address" value={inspectForm.address} onChangeText={(v) => setInspectForm(p => ({ ...p, address: v }))} placeholder="0x..." />
                <Button label={busyAction === "inspectContract" ? "Inspecting..." : "Inspect Contract"} onPress={inspectContractAction} disabled={busy} />

                {inspectData.abi && (
                  <View style={styles.sectionGapSmall}>
                    <View style={{ marginBottom: scale(10) }}>
                      <ScrollView horizontal showsHorizontalScrollIndicator={false} contentContainerStyle={{ gap: scale(8) }}>
                        {['ABI', 'Functions', 'JS Client', 'Storage', 'Events', 'Overview'].map(t => (
                          <Chip key={t} label={t} active={explorerTab === t.toLowerCase()} onPress={() => setExplorerTab(t.toLowerCase())} />
                        ))}
                      </ScrollView>
                    </View>

                    <View style={{ marginTop: scale(8), backgroundColor: '#0b1020', borderRadius: scale(12), padding: scale(12), borderWidth: 1, borderColor: '#161b33' }}>
                      {explorerTab === 'abi' && (
                        <View>
                          <View style={{ flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center', marginBottom: scale(8) }}>
                            <Text style={{ color: '#8a78ff', fontWeight: 'bold' }}>JSON ABI</Text>
                            <TouchableOpacity onPress={() => { Clipboard.setString(JSON.stringify(inspectData.abi, null, 2)); showToast("ABI Copied", "success"); }}>
                              <Text style={{ color: '#10b981', fontSize: scale(12) }}>📋 Copy JSON</Text>
                            </TouchableOpacity>
                          </View>
                          <ScrollView style={{ maxHeight: scale(300) }} nestedScrollEnabled={true}>
                            <Text style={{ color: '#9aa5ca', fontSize: scale(11), fontFamily: Platform.OS === 'ios' ? 'Courier' : 'monospace' }}>
                              {JSON.stringify(inspectData.abi, null, 2)}
                            </Text>
                          </ScrollView>
                        </View>
                      )}

                      {explorerTab === 'functions' && (
                        <View>
                          <Text style={{ color: '#8a78ff', fontWeight: 'bold', marginBottom: scale(10) }}>Contract Functions</Text>
                          {inspectData.abi.map((fn, i) => {
                            const name = (fn.name || "").toLowerCase();
                            const isRead = fn.stateMutability === 'view' ||
                              fn.stateMutability === 'pure' ||
                              fn.constant === true ||
                              name.startsWith('get') ||
                              name.includes('balance') ||
                              name === 'name' ||
                              name === 'symbol' ||
                              name === 'decimals' ||
                              name.includes('total');

                            return (
                              <View key={i} style={{ flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center', paddingVertical: scale(8), borderBottomWidth: 1, borderBottomColor: '#161b33' }}>
                                <View style={{ flex: 1 }}>
                                  <Text style={{ color: '#f4f7ff', fontSize: scale(13), fontWeight: '700' }}>{fn.name}</Text>
                                  <Text style={{ color: '#717da4', fontSize: scale(11) }}>{(fn.inputs || []).map(inp => `${inp.name || 'arg'}:${inp.type}`).join(', ') || 'no args'}</Text>
                                </View>
                                <View style={{ backgroundColor: isRead ? 'rgba(16, 185, 129, 0.15)' : 'rgba(59, 130, 246, 0.15)', paddingHorizontal: scale(8), paddingVertical: scale(2), borderRadius: scale(4) }}>
                                  <Text style={{ color: isRead ? '#10b981' : '#60a5fa', fontSize: scale(10), fontWeight: 'bold' }}>{isRead ? 'READ' : 'WRITE'}</Text>
                                </View>
                              </View>
                            );
                          })}
                        </View>
                      )}

                      {explorerTab === 'js client' && (
                        <View>
                          <View style={{ flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center', marginBottom: scale(8) }}>
                            <Text style={{ color: '#8a78ff', fontWeight: 'bold' }}>JS Client Code</Text>
                            <TouchableOpacity onPress={() => {
                              const code = `// LQD Contract Client\nconst ADDR = "${inspectForm.address}";\n// ... implementation ...`;
                              Clipboard.setString(code);
                              showToast("Client Code Copied", "success");
                            }}>
                              <Text style={{ color: '#10b981', fontSize: scale(12) }}>📋 Copy Code</Text>
                            </TouchableOpacity>
                          </View>
                          <ScrollView style={{ maxHeight: scale(300) }} nestedScrollEnabled={true}>
                            <Text style={{ color: '#f4f7ff', fontSize: scale(11), fontFamily: Platform.OS === 'ios' ? 'Courier' : 'monospace' }}>
                              {`// LQD Contract Client — ${inspectForm.address}\nconst ADDR = "${inspectForm.address}";\n\n` +
                                inspectData.abi.map(fn => {
                                  const params = (fn.inputs || []).map((_, i) => "arg" + (i + 1)).join(", ");
                                  return "export const " + fn.name + " = (" + params + ") => callContract(\"" + fn.name + "\", [" + params + "]);";
                                }).join("\n")}
                            </Text>
                          </ScrollView>
                        </View>
                      )}

                      {explorerTab === 'storage' && (
                        <View>
                          <Text style={{ color: '#8a78ff', fontWeight: 'bold', marginBottom: scale(8) }}>Global Storage</Text>
                          {Object.keys(inspectData.storage || {}).length === 0 ? (
                            <Text style={{ color: '#717da4', fontSize: scale(12) }}>No public storage state found.</Text>
                          ) : (
                            Object.entries(inspectData.storage).map(([k, v], i) => (
                              <View key={i} style={{ paddingVertical: scale(6), borderBottomWidth: 1, borderBottomColor: '#161b33' }}>
                                <Text style={{ color: '#60a5fa', fontSize: scale(11) }}>{k}</Text>
                                <Text style={{ color: '#f4f7ff', fontSize: scale(12), fontWeight: '600' }}>{JSON.stringify(v)}</Text>
                              </View>
                            ))
                          )}
                        </View>
                      )}

                      {explorerTab === 'events' && (
                        <View>
                          <Text style={{ color: '#8a78ff', fontWeight: 'bold', marginBottom: scale(8) }}>Contract Events</Text>
                          {!explorerEvents.length ? (
                            <Text style={{ color: '#717da4', fontSize: scale(12) }}>No recent events found.</Text>
                          ) : (
                            explorerEvents.map((ev, i) => (
                              <View key={i} style={{ paddingVertical: scale(8), borderBottomWidth: 1, borderBottomColor: '#161b33' }}>
                                <Text style={{ color: '#10b981', fontSize: scale(11), fontWeight: 'bold' }}>{ev.event || 'Unknown Event'}</Text>
                                <Text style={{ color: '#9aa5ca', fontSize: scale(10) }}>TX: {shortAddress(ev.transactionHash || ev.tx_hash)}</Text>
                              </View>
                            ))
                          )}
                        </View>
                      )}

                      {explorerTab === 'overview' && (
                        <View>
                          <Text style={{ color: '#8a78ff', fontWeight: 'bold', marginBottom: scale(8) }}>Contract Overview</Text>
                          <Text style={{ color: '#9aa5ca', fontSize: scale(12) }}>Address: {inspectForm.address}</Text>
                          <Text style={{ color: '#9aa5ca', fontSize: scale(12) }}>Functions: {inspectData.abi?.length || 0}</Text>
                          <Text style={{ color: '#9aa5ca', fontSize: scale(12) }}>Balance: {formatUnits(nativeBalance, currentNetwork.decimals || 8, 4)} LQD</Text>
                        </View>
                      )}
                    </View>
                  </View>
                )}
              </Card>
            </View>
          )}




          {tab === "bridge" && (
            <View style={styles.mmHome}>
              <View style={[styles.mmAccountCard, { backgroundColor: '#1e293b', marginBottom: scale(20) }]}>
                <Text style={{ color: '#f4f7ff', fontSize: scale(20), fontWeight: '800', marginBottom: scale(4) }}>Bridge & Swap</Text>
                <Text style={{ color: '#94a3b8', fontSize: scale(13) }}>Transfer assets across chains securely</Text>
              </View>

              <View style={{ paddingHorizontal: scale(16) }}>
                {/* Mode Selector (Public/Private) */}
                <View style={{ flexDirection: 'row', backgroundColor: '#161b33', borderRadius: scale(12), padding: scale(4), marginBottom: scale(20), borderWidth: 1, borderColor: '#273152' }}>
                  <TouchableOpacity
                    onPress={() => setBridgeMode("public")}
                    style={{ flex: 1, paddingVertical: scale(8), flexDirection: 'row', justifyContent: 'center', alignItems: 'center', borderRadius: scale(10), backgroundColor: bridgeMode === 'public' ? '#273152' : 'transparent' }}
                  >
                    <Text style={{ color: bridgeMode === 'public' ? '#fff' : '#717da4', fontWeight: 'bold', fontSize: scale(13) }}>🔓 Public</Text>
                  </TouchableOpacity>
                  <TouchableOpacity
                    onPress={() => setBridgeMode("private")}
                    style={{ flex: 1, paddingVertical: scale(8), flexDirection: 'row', justifyContent: 'center', alignItems: 'center', borderRadius: scale(10), backgroundColor: bridgeMode === 'private' ? '#273152' : 'transparent' }}
                  >
                    <Text style={{ color: bridgeMode === 'private' ? '#fff' : '#717da4', fontWeight: 'bold', fontSize: scale(13) }}>🛡️ Private</Text>
                  </TouchableOpacity>
                </View>

                {/* Swap Container */}
                <View style={{ backgroundColor: '#161b33', borderRadius: scale(24), padding: scale(16), borderWidth: 1, borderColor: '#273152' }}>

                  {/* FROM Section */}
                  <View style={{ backgroundColor: '#0f152a', borderRadius: scale(20), padding: scale(16), marginBottom: scale(4) }}>
                    <View style={{ flexDirection: 'row', justifyContent: 'space-between', marginBottom: scale(12) }}>
                      <Text style={{ color: '#717da4', fontSize: scale(11), fontWeight: '700' }}>SOURCE CHAIN</Text>
                      <TouchableOpacity onPress={() => { setBridgeTargetSide("source"); setBridgeChainModalVisible(true); }}>
                        <Text style={{ color: '#8a78ff', fontSize: scale(11), fontWeight: 'bold' }}>{bridgeDirection === 'lqd_to_external' ? 'LQD Mainnet' : (currentBridgeChain?.name || 'Select Source')} ▾</Text>
                      </TouchableOpacity>
                    </View>

                    <View style={{ flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center' }}>
                      <TextInput
                        style={{ flex: 1, color: '#fff', fontSize: scale(24), fontWeight: 'bold', padding: 0 }}
                        value={bridgeForm.amount}
                        onChangeText={(v) => setBridgeForm(p => ({ ...p, amount: v }))}
                        placeholder="0"
                        placeholderTextColor="#273152"
                        keyboardType="decimal-pad"
                      />
                      <TouchableOpacity
                        onPress={() => setBridgeTokenModalVisible(true)}
                        style={{ flexDirection: 'row', alignItems: 'center', backgroundColor: '#1e293b', paddingHorizontal: scale(12), paddingVertical: scale(8), borderRadius: scale(20), borderWidth: 1, borderColor: '#273152' }}
                      >
                        <Text style={{ fontSize: scale(18), marginRight: scale(6) }}>💎</Text>
                        <Text style={{ color: '#fff', fontWeight: 'bold', fontSize: scale(14) }}>{bridgeSelectedToken?.symbol || 'LQD'}</Text>
                        <Text style={{ color: '#717da4', marginLeft: scale(4), fontSize: scale(10) }}>▼</Text>
                      </TouchableOpacity>
                    </View>
                    <Text style={{ color: '#717da4', fontSize: scale(11), marginTop: scale(8) }}>Balance: {bridgeDirection === 'lqd_to_external' ? formatUnits(nativeBalance, currentNetwork.decimals || 8, 4) : '0.00'} LQD</Text>
                  </View>

                  {/* Arrow Divider */}
                  <View style={{ alignItems: 'center', zIndex: 10, marginVertical: scale(-12) }}>
                    <TouchableOpacity
                      onPress={() => setBridgeDirection(prev => prev === 'lqd_to_external' ? 'external_to_lqd' : 'lqd_to_external')}
                      style={{ width: scale(36), height: scale(36), backgroundColor: '#161b33', borderRadius: scale(18), justifyContent: 'center', alignItems: 'center', borderWidth: 4, borderColor: '#070a15' }}
                    >
                      <Text style={{ fontSize: scale(16), color: '#8a78ff' }}>🔄</Text>
                    </TouchableOpacity>
                  </View>

                  {/* TO Section */}
                  <View style={{ backgroundColor: '#0f152a', borderRadius: scale(20), padding: scale(16), marginTop: scale(4) }}>
                    <View style={{ flexDirection: 'row', justifyContent: 'space-between', marginBottom: scale(12) }}>
                      <Text style={{ color: '#717da4', fontSize: scale(11), fontWeight: '700' }}>DESTINATION CHAIN</Text>
                      <TouchableOpacity onPress={() => { setBridgeTargetSide("target"); setBridgeChainModalVisible(true); }}>
                        <Text style={{ color: '#10b981', fontSize: scale(11), fontWeight: 'bold' }}>{bridgeDirection === 'lqd_to_external' ? (currentBridgeChain?.name || 'Select Target') : 'LQD Mainnet'} ▾</Text>
                      </TouchableOpacity>
                    </View>
                    <View style={{ flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center' }}>
                      <Text style={{ flex: 1, color: '#f4f7ff', fontSize: scale(24), fontWeight: 'bold' }}>
                        {bridgeForm.amount || '0'}
                      </Text>
                      <TouchableOpacity
                        onPress={() => setBridgeTokenModalVisible(true)}
                        style={{ flexDirection: 'row', alignItems: 'center', backgroundColor: '#1e293b', paddingHorizontal: scale(12), paddingVertical: scale(8), borderRadius: scale(20), borderWidth: 1, borderColor: '#8a78ff' }}
                      >
                        <Text style={{ color: '#fff', fontWeight: 'bold', fontSize: scale(14) }}>{bridgeDirection === 'lqd_to_external' ? (bridgeSelectedToken?.symbol || 'LQD') : 'LQD'}</Text>
                        <Text style={{ color: '#717da4', marginLeft: scale(4), fontSize: scale(10) }}>▼</Text>
                      </TouchableOpacity>
                    </View>

                    {/* Selectable Target Label */}
                    <TouchableOpacity
                      onPress={() => setBridgeChainModalVisible(true)}
                      style={{ marginTop: scale(8), alignSelf: 'flex-start' }}
                    >
                      <Text style={{ color: '#10b981', fontSize: scale(12), fontWeight: '600' }}>
                        Target: <Text style={{ textDecorationLine: 'underline' }}>{bridgeDirection === 'lqd_to_external' ? (currentBridgeChain?.name || 'External Network') : 'LQD Mainnet'}</Text>
                      </Text>
                    </TouchableOpacity>
                  </View>

                  {/* Recipient Field */}
                  <View style={{ marginTop: scale(20) }}>
                    <Text style={{ color: '#717da4', fontSize: scale(11), marginBottom: scale(8), marginLeft: scale(4), fontWeight: '700' }}>
                      RECIPIENT ({bridgeDirection === 'lqd_to_external' ? (currentBridgeChain?.name || 'External Network') : 'LQD'})
                    </Text>
                    <TextInput
                      style={{ backgroundColor: '#0f152a', color: '#fff', borderRadius: scale(12), padding: scale(14), borderWidth: 1, borderColor: '#1b2342', fontSize: scale(13) }}
                      value={bridgeDirection === 'lqd_to_external' ? bridgeForm.toBsc : bridgeForm.toLqd}
                      onChangeText={(v) => setBridgeForm((p) => bridgeDirection === 'lqd_to_external' ? { ...p, toBsc: v } : { ...p, toLqd: v })}
                      placeholder="Enter recipient address..."
                      placeholderTextColor="#273152"
                    />
                  </View>

                  {bridgeNeedsExternalProof() && (
                    <View style={{ marginTop: scale(16), padding: scale(14), backgroundColor: '#0f152a', borderRadius: scale(16), borderWidth: 1, borderColor: bridgeProofReady() ? 'rgba(16, 185, 129, 0.35)' : 'rgba(245, 158, 11, 0.35)' }}>
                      <Text style={{ color: '#f4f7ff', fontSize: scale(13), fontWeight: '800', marginBottom: scale(4) }}>External chain proof</Text>
                      <Text style={{ color: '#94a3b8', fontSize: scale(11), lineHeight: scale(16), marginBottom: scale(12) }}>
                        Complete the source-chain transfer first, then paste the proof so the LQD relayer can verify and mint/release on LQD.
                      </Text>

                      <Text style={{ color: '#717da4', fontSize: scale(11), marginBottom: scale(8), fontWeight: '700' }}>{bridgeProofLabel().toUpperCase()}</Text>
                      <TextInput
                        style={{ backgroundColor: '#111936', color: '#fff', borderRadius: scale(12), padding: scale(14), borderWidth: 1, borderColor: '#1b2342', fontSize: scale(12), marginBottom: scale(12) }}
                        value={bridgeForm.sourceTxHash}
                        onChangeText={(v) => setBridgeForm((p) => ({ ...p, sourceTxHash: v }))}
                        placeholder="Paste source transaction hash..."
                        placeholderTextColor="#3b4568"
                        autoCapitalize="none"
                      />

                      <Text style={{ color: '#717da4', fontSize: scale(11), marginBottom: scale(8), fontWeight: '700' }}>{bridgeProofAddressLabel().toUpperCase()}</Text>
                      <TextInput
                        style={{ backgroundColor: '#111936', color: '#fff', borderRadius: scale(12), padding: scale(14), borderWidth: 1, borderColor: '#1b2342', fontSize: scale(12), marginBottom: scale(12) }}
                        value={bridgeForm.sourceAddress}
                        onChangeText={(v) => setBridgeForm((p) => ({ ...p, sourceAddress: v }))}
                        placeholder="Paste source sender address..."
                        placeholderTextColor="#3b4568"
                        autoCapitalize="none"
                      />

                      <View style={{ flexDirection: 'row', gap: scale(8) }}>
                        <TextInput
                          style={{ flex: 1, backgroundColor: '#111936', color: '#fff', borderRadius: scale(12), padding: scale(12), borderWidth: 1, borderColor: '#1b2342', fontSize: scale(11) }}
                          value={bridgeForm.sourceMemo}
                          onChangeText={(v) => setBridgeForm((p) => ({ ...p, sourceMemo: v }))}
                          placeholder="Memo optional"
                          placeholderTextColor="#3b4568"
                          autoCapitalize="none"
                        />
                        <TextInput
                          style={{ flex: 1, backgroundColor: '#111936', color: '#fff', borderRadius: scale(12), padding: scale(12), borderWidth: 1, borderColor: '#1b2342', fontSize: scale(11) }}
                          value={bridgeForm.sourceSequence}
                          onChangeText={(v) => setBridgeForm((p) => ({ ...p, sourceSequence: v }))}
                          placeholder="Seq / block optional"
                          placeholderTextColor="#3b4568"
                          autoCapitalize="none"
                        />
                      </View>

                      <View style={{ flexDirection: 'row', gap: scale(8), marginTop: scale(8) }}>
                        <TextInput
                          style={{ flex: 1, backgroundColor: '#111936', color: '#fff', borderRadius: scale(12), padding: scale(12), borderWidth: 1, borderColor: '#1b2342', fontSize: scale(11) }}
                          value={bridgeForm.sourceOutput}
                          onChangeText={(v) => setBridgeForm((p) => ({ ...p, sourceOutput: v }))}
                          placeholder="Output optional"
                          placeholderTextColor="#3b4568"
                          autoCapitalize="none"
                        />
                        <View style={{ flex: 1 }} />
                      </View>
                    </View>
                  )}

                  {!bridgeChainIsEnabled() && (
                    <View style={{ marginTop: scale(16), padding: scale(12), backgroundColor: 'rgba(245, 158, 11, 0.08)', borderRadius: scale(14), borderWidth: 1, borderColor: 'rgba(245, 158, 11, 0.25)' }}>
                      <Text style={{ color: '#fbbf24', fontSize: scale(12), fontWeight: '800' }}>Route inactive</Text>
                      <Text style={{ color: '#94a3b8', fontSize: scale(11), marginTop: scale(4), lineHeight: scale(16) }}>
                        This chain is listed in the bridge registry but not enabled yet. Enable its registry config and token mapping before production bridging.
                      </Text>
                    </View>
                  )}

                  <View style={{ gap: scale(8), marginVertical: scale(20), padding: scale(16), backgroundColor: 'rgba(138, 120, 255, 0.05)', borderRadius: scale(16), borderWidth: 1, borderColor: 'rgba(138, 120, 255, 0.1)' }}>
                    <View style={{ flexDirection: 'row', justifyContent: 'space-between' }}>
                      <Text style={{ color: '#717da4', fontSize: scale(12) }}>Source Chain Fee</Text>
                      <Text style={{ color: '#f4f7ff', fontSize: scale(12) }}>{bridgeSourceFee.toFixed(4)} {bridgeDirection === 'lqd_to_external' ? 'LQD' : bridgeNativeSymbol}</Text>
                    </View>
                    <View style={{ flexDirection: 'row', justifyContent: 'space-between' }}>
                      <Text style={{ color: '#717da4', fontSize: scale(12) }}>Target Chain Fee</Text>
                      <Text style={{ color: '#f4f7ff', fontSize: scale(12) }}>{bridgeTargetFee.toFixed(4)} {bridgeDirection === 'lqd_to_external' ? bridgeNativeSymbol : 'LQD'}</Text>
                    </View>
                    <View style={{ flexDirection: 'row', justifyContent: 'space-between' }}>
                      <Text style={{ color: '#717da4', fontSize: scale(12) }}>Platform Fee (0.5%)</Text>
                      <Text style={{ color: '#f4f7ff', fontSize: scale(12) }}>{(parseFloat(bridgeForm.amount || 0) * 0.005).toFixed(4)} LQD</Text>
                    </View>
                    <View style={{ height: 1, backgroundColor: '#1b2342', marginVertical: scale(4) }} />
                    <View style={{ flexDirection: 'row', justifyContent: 'space-between' }}>
                      <Text style={{ color: '#fff', fontSize: scale(13), fontWeight: 'bold' }}>Total Bridge Fee</Text>
                      <Text style={{ color: '#8a78ff', fontSize: scale(13), fontWeight: 'bold' }}>{bridgeTotalFee} LQD</Text>
                    </View>
                    <View style={{ flexDirection: 'row', justifyContent: 'space-between', marginTop: scale(4) }}>
                      <Text style={{ color: '#717da4', fontSize: scale(12) }}>Privacy Mode</Text>
                      <Text style={{ color: bridgeMode === 'private' ? '#8a78ff' : '#10b981', fontSize: scale(12), fontWeight: 'bold' }}>{bridgeMode.toUpperCase()}</Text>
                    </View>
                  </View>

                  <Button
                    label={busy ? "Processing…" : (bridgeNeedsExternalProof() ? "Submit Verified Proof" : (bridgeMode === 'private' ? `🔒 Private Bridge` : "Initiate Bridge Request"))}
                    onPress={submitBridgeAction}
                    disabled={busy || !bridgeProofReady() || !bridgeChainIsEnabled()}
                    primary
                    style={{ borderRadius: scale(16), height: scale(56) }}
                  />
                </View>

                {/* Bridge Chain Selection Modal */}
                <Modal visible={bridgeChainModalVisible} transparent animationType="fade">
                  <View style={{ flex: 1, backgroundColor: 'rgba(7, 10, 21, 0.8)', justifyContent: 'center', alignItems: 'center' }}>
                    <View style={{ backgroundColor: '#161b33', width: '85%', borderRadius: scale(24), padding: scale(20), borderWidth: 1, borderColor: '#273152' }}>
                      <View style={{ flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center', marginBottom: scale(20) }}>
                        <Text style={{ color: '#fff', fontSize: scale(18), fontWeight: 'bold' }}>Select Network</Text>
                        <TouchableOpacity onPress={() => setBridgeChainModalVisible(false)}>
                          <Text style={{ color: '#717da4', fontSize: scale(20) }}>✕</Text>
                        </TouchableOpacity>
                      </View>

                      <ScrollView style={{ maxHeight: scale(300) }}>
                        {(bridgeChains.length > 0 ? bridgeChains : NETWORKS).map((cfg) => (
                          <TouchableOpacity
                            key={cfg.id}
                            onPress={() => { applyBridgeChainSelection(cfg); setBridgeChainModalVisible(false); }}
                            style={{
                              padding: scale(16),
                              backgroundColor: bridgeChainId === cfg.id ? 'rgba(138, 120, 255, 0.1)' : '#0f152a',
                              borderRadius: scale(16),
                              marginBottom: scale(8),
                              borderWidth: 1,
                              borderColor: bridgeChainId === cfg.id ? '#8a78ff' : '#1b2342'
                            }}
                          >
                            <View style={{ flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center' }}>
                              <View>
                                <Text style={{ color: '#fff', fontWeight: 'bold', fontSize: scale(14) }}>{cfg.name}</Text>
                                <Text style={{ color: '#717da4', fontSize: scale(11) }}>{(cfg.family || 'evm').toUpperCase()} · {cfg.symbol || cfg.nativeSymbol}</Text>
                              </View>
                              <Text style={{ color: bridgeChainIsEnabled(cfg) ? (bridgeChainId === cfg.id ? '#8a78ff' : '#10b981') : '#f59e0b', fontSize: scale(12), fontWeight: '800' }}>
                                {bridgeChainId === cfg.id ? '✓ ' : ''}{bridgeChainIsEnabled(cfg) ? 'Active' : 'Inactive'}
                              </Text>
                            </View>
                          </TouchableOpacity>
                        ))}
                      </ScrollView>

                      <Button label="Close" onPress={() => setBridgeChainModalVisible(false)} secondary style={{ marginTop: scale(10) }} />
                    </View>
                  </View>
                </Modal>

                {/* Bridge Token Selection Modal */}
                <Modal visible={bridgeTokenModalVisible} transparent animationType="fade">
                  <View style={{ flex: 1, backgroundColor: 'rgba(7, 10, 21, 0.8)', justifyContent: 'center', alignItems: 'center' }}>
                    <View style={{ backgroundColor: '#161b33', width: '85%', borderRadius: scale(24), padding: scale(20), borderWidth: 1, borderColor: '#273152' }}>
                      <View style={{ flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center', marginBottom: scale(20) }}>
                        <Text style={{ color: '#fff', fontSize: scale(18), fontWeight: 'bold' }}>Select Asset</Text>
                        <TouchableOpacity onPress={() => setBridgeTokenModalVisible(false)}>
                          <Text style={{ color: '#717da4', fontSize: scale(20) }}>✕</Text>
                        </TouchableOpacity>
                      </View>

                      <ScrollView style={{ maxHeight: scale(300) }}>
                        {bridgeTokens.length === 0 ? (
                          <Text style={{ color: '#717da4', textAlign: 'center', padding: scale(20) }}>No tokens found for this chain.</Text>
                        ) : (
                          bridgeTokens.map((t, i) => (
                            <TouchableOpacity
                              key={i}
                              onPress={() => { setBridgeSelectedToken(t); setBridgeForm(p => ({ ...p, token: t.symbol })); setBridgeTokenModalVisible(false); }}
                              style={{
                                padding: scale(16),
                                backgroundColor: bridgeSelectedToken?.symbol === t.symbol ? 'rgba(138, 120, 255, 0.1)' : '#0f152a',
                                borderRadius: scale(16),
                                marginBottom: scale(8),
                                borderWidth: 1,
                                borderColor: bridgeSelectedToken?.symbol === t.symbol ? '#8a78ff' : '#1b2342'
                              }}
                            >
                              <View style={{ flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center' }}>
                                <View style={{ flexDirection: 'row', alignItems: 'center' }}>
                                  <Text style={{ fontSize: scale(20), marginRight: scale(12) }}>💎</Text>
                                  <View>
                                    <Text style={{ color: '#fff', fontWeight: 'bold', fontSize: scale(14) }}>{t.symbol}</Text>
                                    <Text style={{ color: '#717da4', fontSize: scale(11) }}>{t.name || 'LQD Token'}</Text>
                                  </View>
                                </View>
                                {bridgeSelectedToken?.symbol === t.symbol && (
                                  <Text style={{ color: '#8a78ff', fontSize: scale(16) }}>✓</Text>
                                )}
                              </View>
                            </TouchableOpacity>
                          ))
                        )}
                      </ScrollView>

                      <Button label="Close" onPress={() => setBridgeTokenModalVisible(false)} secondary style={{ marginTop: scale(10) }} />
                    </View>
                  </View>
                </Modal>


                {/* Bridge Activity/History Section */}
                <View style={{ marginTop: scale(30), borderTopWidth: 1, borderTopColor: '#1b2342', paddingTop: scale(20) }}>
                  <Text style={{ color: '#fff', fontSize: scale(16), fontWeight: 'bold', marginBottom: scale(12) }}>Bridge History</Text>
                  <View style={{ gap: scale(12) }}>
                    {bridgeHistoryItems.length === 0 ? (
                      <View style={{ padding: scale(30), backgroundColor: '#0f152a', borderRadius: scale(16), alignItems: 'center' }}>
                        <Text style={{ color: '#717da4', fontSize: scale(12) }}>No bridge activity found.</Text>
                      </View>
                    ) : (
                      bridgeHistoryItems.map((item, idx) => (
                        <BridgeRow key={`${item.tx_hash || item.id || idx}`} item={item} />
                      ))
                    )}
                  </View>
                </View>

              </View>
            </View>
          )}




          {tab === "liquidity" && (
            <View style={styles.mmHome}>
              <View style={[styles.mmAccountCard, { backgroundColor: '#1e293b', marginBottom: scale(18) }]}>
                <Text style={{ color: '#f4f7ff', fontSize: scale(20), fontWeight: '800', marginBottom: scale(6) }}>Liquidity</Text>
                <Text style={{ color: '#94a3b8', fontSize: scale(13), textAlign: 'center' }}>
                  Track DEX LP positions, lock LP for validation, register validator eligibility, and claim rewards.
                </Text>
                <View style={[styles.inlineButtons, { marginTop: scale(14) }]}>
                  <Button label={liquidityLoading ? "Loading…" : "Refresh"} onPress={loadLiquidityDashboard} disabled={liquidityLoading} />
                  <Button label={liquidityAction === "sync" ? "Syncing…" : "Sync Pools"} onPress={syncLiquidityPools} disabled={!!liquidityAction} secondary />
                </View>
              </View>

              <LPRewardTierGuide />

              <Card title="Reward Dashboard" subtitle="LP rewards are weighted by approved pool tier and your locked LP position.">
                <View style={styles.statsGrid}>
                  <Stat label="Pending Rewards" value={`${formatUnits(liquidityDashboard.protocol?.pending_reward || liquidityDashboard.latestRewards?.pending_reward || "0", 8, 4)} LQD`} />
                  <Stat label="Total Earned" value={`${formatUnits(liquidityDashboard.protocol?.total_reward || liquidityDashboard.latestRewards?.total_reward || "0", 8, 4)} LQD`} />
                  <Stat label="APR/APY" value={liquidityDashboard.latestRewards?.apr ? `${liquidityDashboard.latestRewards.apr}%` : "Dynamic"} subvalue="Pool-weighted" />
                </View>
                <Button label={liquidityAction === "claim" ? "Claiming…" : "Claim Rewards"} onPress={claimLiquidityRewards} disabled={!!liquidityAction} />
              </Card>

              <Card title="Multi-pool LP Positions" subtitle="Add/remove liquidity in DEX. This section locks LP, unlocks LP, and registers validators against each pool.">
                {liquidityLoading ? (
                  <View style={{ padding: scale(24), alignItems: 'center' }}>
                    <ActivityIndicator color="#8a78ff" />
                    <Text style={{ color: '#94a3b8', marginTop: scale(10) }}>Loading liquidity pools...</Text>
                  </View>
                ) : (() => {
                  const used = new Set();
                  const allCards = liquidityDashboard.poolCards || [];
                  const findSlotCard = (slot) => {
                    const idx = allCards.findIndex((card) => card.slotId === slot.id && !used.has(String(card.address).toLowerCase()));
                    if (idx === -1) return null;
                    used.add(String(allCards[idx].address).toLowerCase());
                    return allCards[idx];
                  };
                  const communityCards = allCards.filter((card) => !card.slotId && !used.has(String(card.address).toLowerCase()));
                  const renderPoolCard = (slot, card) => {
                    const configured = !!card?.address;
                    const view = card || {
                      slotId: slot.id,
                      pair: slot.label,
                      tierLabel: slot.tier,
                      weightLabel: slot.weight,
                      dex: { lpBalance: "0", lockedLP: "0", totalLP: "0", lockUntil: "0" },
                    };
                    const key = liquidityInputKey(view);
                    const input = liquidityPoolInputs[key] || { lockAmount: "", lockDays: "365" };
                    const result = liquidityPoolResults[key];
                    const actionKey = liquidityAction.split(":")[0];
                    const isActiveAction = liquidityAction.endsWith(key);
                    return (
                      <View key={key} style={{ backgroundColor: configured ? '#151b31' : '#11172a', borderWidth: 1, borderColor: configured ? '#273152' : '#27315288', borderRadius: scale(18), padding: scale(14), marginBottom: scale(12), opacity: configured ? 1 : 0.72 }}>
                        <View style={{ flexDirection: 'row', justifyContent: 'space-between', alignItems: 'flex-start', gap: scale(10) }}>
                          <View style={{ flex: 1 }}>
                            <Text style={{ color: '#f4f7ff', fontSize: scale(17), fontWeight: '900' }}>{view.pair || slot.label}</Text>
                            <Text style={{ color: '#94a3b8', fontSize: scale(11), marginTop: scale(3) }}>
                              {view.tierLabel || slot.tier} · Weight {view.weightLabel || slot.weight}{configured ? ` · ${shortAddress(view.address)}` : ' · Configure in DEX/admin first'}
                            </Text>
                          </View>
                          <View style={{ paddingHorizontal: scale(10), paddingVertical: scale(5), borderRadius: scale(999), backgroundColor: configured ? 'rgba(16, 185, 129, 0.12)' : 'rgba(148, 163, 184, 0.10)', borderWidth: 1, borderColor: configured ? 'rgba(16, 185, 129, 0.35)' : '#334070' }}>
                            <Text style={{ color: configured ? '#86efac' : '#94a3b8', fontSize: scale(11), fontWeight: '800' }}>{configured ? 'Configured' : 'Not configured'}</Text>
                          </View>
                        </View>

                        <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: scale(8), marginTop: scale(14) }}>
                          <View style={styles.lpMiniStat}><Text style={styles.lpMiniLabel}>Unlocked LP</Text><Text style={styles.lpMiniValue}>{formatUnits(view.dex.lpBalance, 8, 4)} LP</Text></View>
                          <View style={styles.lpMiniStat}><Text style={styles.lpMiniLabel}>Locked LP</Text><Text style={styles.lpMiniValue}>{formatUnits(view.dex.lockedLP, 8, 4)} LP</Text></View>
                          <View style={styles.lpMiniStat}><Text style={styles.lpMiniLabel}>Pool Share</Text><Text style={styles.lpMiniValue}>{formatPercentFromParts(view.dex.lpBalance, view.dex.totalLP)}</Text></View>
                          <View style={styles.lpMiniStat}><Text style={styles.lpMiniLabel}>Unlock Time</Text><Text style={styles.lpMiniValue}>{formatUnlockTime(view.dex.lockUntil)}</Text></View>
                          <View style={styles.lpMiniStat}><Text style={styles.lpMiniLabel}>Lock Days</Text><Text style={styles.lpMiniValue}>{formatLockDays(view.dex.lockUntil)}</Text></View>
                        </View>

                        <View style={{ marginTop: scale(14) }}>
                          <Field label="LP Amount" value={input.lockAmount || ""} onChangeText={(v) => updateLiquidityPoolInput(view, { lockAmount: v })} placeholder="0.0" keyboardType="decimal-pad" editable={configured && !liquidityAction} />
                          <Field label="Lock Days" value={String(input.lockDays || "365")} onChangeText={(v) => updateLiquidityPoolInput(view, { lockDays: v.replace(/[^\d]/g, "") })} placeholder="365" keyboardType="numeric" editable={configured && !liquidityAction} />
                        </View>

                        <View style={styles.inlineButtons}>
                          <Button label={isActiveAction && actionKey === "lock" ? "Locking…" : "Stake / Lock LP"} onPress={() => submitPoolLockAction(view)} disabled={!configured || !!liquidityAction} compact />
                          <Button label={isActiveAction && actionKey === "register" ? "Registering…" : "Register Validator"} onPress={() => registerPoolValidator(view)} disabled={!configured || !!liquidityAction} compact secondary />
                        </View>
                        <Button label={isActiveAction && actionKey === "unlock" ? "Unlocking…" : "Unstake / Unlock LP"} onPress={() => submitPoolUnlockAction(view)} disabled={!configured || !!liquidityAction} danger />

                        {result?.message ? (
                          <View style={[styles.resultBox, { marginTop: scale(12), borderColor: result.type === 'error' ? '#7f1d1d' : '#14532d', backgroundColor: result.type === 'error' ? 'rgba(127, 29, 29, 0.18)' : 'rgba(20, 83, 45, 0.18)' }]}>
                            <Text style={{ color: result.type === 'error' ? '#fca5a5' : '#86efac', fontSize: scale(12), lineHeight: scale(18) }}>{result.message}</Text>
                          </View>
                        ) : null}
                      </View>
                    );
                  };
                  return (
                    <View>
                      <Text style={styles.inspectTitle}>Core reward pools</Text>
                      {LP_CANONICAL_POOL_SLOTS.map((slot) => renderPoolCard(slot, findSlotCard(slot)))}
                      <Text style={[styles.inspectTitle, { marginTop: scale(10) }]}>Tier 3 community pools</Text>
                      {communityCards.length ? communityCards.map((card) => renderPoolCard({ id: card.address, label: card.pair, tier: "Tier 3", weight: "0.35x" }, card)) : (
                        <View style={[styles.resultBox, { marginTop: scale(8), borderColor: '#334070' }]}>
                          <Text style={{ color: '#94a3b8' }}>No community pools yet. Admin-approved pools will appear here automatically.</Text>
                        </View>
                      )}
                    </View>
                  );
                })()}
              </Card>
            </View>
          )}

          {tab === "settings" && (
            <View style={styles.sectionGap}>
              <Card title="Wallet Settings" subtitle="Configure your experience and endpoints.">
                <View style={styles.settingsGroup}>
                  <Text style={styles.settingsLabel}>Network Configuration</Text>
                  <Field label="Node RPC" value={endpoints.nodeUrl} onChangeText={(v) => setEndpoints(p => ({ ...p, nodeUrl: v }))} placeholder="https://..." />
                  <Field label="Explorer" value={endpoints.explorerUrl} onChangeText={(v) => setEndpoints(p => ({ ...p, explorerUrl: v }))} placeholder="https://..." />
                </View>

                <View style={styles.settingsDivider} />

                <View style={styles.settingsGroup}>
                  <Text style={styles.settingsLabel}>Security & Privacy</Text>
                  <View style={styles.settingRow}>
                    <Text style={styles.settingName}>Auto Refresh Balance</Text>
                    <Switch value={settingsAutoRefresh} onValueChange={setSettingsAutoRefresh} trackColor={{ false: "#1b2342", true: "#8a78ff" }} />
                  </View>
                  <View style={styles.settingRow}>
                    <Text style={styles.settingName}>Biometric Unlock</Text>
                    <Switch value={biometricEnabled} onValueChange={setBiometricEnabled} trackColor={{ false: "#1b2342", true: "#8a78ff" }} />
                  </View>
                  <View style={styles.settingRow}>
                    <Text style={styles.settingName}>Transaction Tracking</Text>
                    <Switch value={settingsTxTrackingEnabled} onValueChange={setSettingsTxTrackingEnabled} trackColor={{ false: "#1b2342", true: "#8a78ff" }} />
                  </View>
                  <View style={styles.settingRow}>
                    <Text style={styles.settingName}>Background Tx Tracking</Text>
                    <Switch value={settingsBackgroundTxTrackingEnabled} onValueChange={setSettingsBackgroundTxTrackingEnabled} trackColor={{ false: "#1b2342", true: "#8a78ff" }} />
                  </View>
                  <View style={styles.settingRow}>
                    <Text style={styles.settingName}>Secure Storage Audit</Text>
                    <Switch value={settingsStorageAuditEnabled} onValueChange={setSettingsStorageAuditEnabled} trackColor={{ false: "#1b2342", true: "#8a78ff" }} />
                  </View>
                  <View style={styles.settingRow}>
                    <Text style={styles.settingName}>Local Error Reporting</Text>
                    <Switch value={settingsTelemetryEnabled} onValueChange={setSettingsTelemetryEnabled} trackColor={{ false: "#1b2342", true: "#8a78ff" }} />
                  </View>
                  <Text style={styles.helperText}>
                    Storage audit: {securityAudit?.checkedAt ? `${securityAudit.ok ? "OK" : "Needs review"} · ${new Date(securityAudit.checkedAt).toLocaleString()}` : "not run yet"}
                  </Text>
                  <Button
                    label="Run Storage Audit"
                    onPress={async () => {
                      const audit = await runSecureStorageAudit({ repair: true });
                      setSecurityAudit(audit);
                      showToast(audit.ok ? "Secure storage audit passed" : "Secure storage audit repaired or reported issues", audit.ok ? "success" : "info");
                    }}
                    compact
                    secondary
                  />
                  <Text style={styles.helperText}>Diagnostics stay on this device unless you export logs manually.</Text>
                </View>

                <View style={styles.settingsDivider} />

                <View style={styles.settingsGroup}>
                  <Text style={styles.settingsLabel}>Account Operations</Text>
                  <View style={styles.inlineButtons}>
                    <Button label="Backup" onPress={saveBackupToClipboard} compact secondary />
                    <Button label="Restore" onPress={restoreBackupFromText} compact secondary />
                    <Button label="Show Mnemonic" onPress={() => setShowMnemonic(true)} compact secondary />
                  </View>
                  <Button label="Export All Chain Keys" onPress={() => { if (!wallet) { showToast("Unlock wallet first", "error"); return; } if (!chainKeys) { showToast("No mnemonic wallet — private key import only supports EVM", "info"); return; } setKeyExportVisible(true); }} secondary />
                  <Button label="Lock Wallet Now" onPress={lockWalletAction} danger />
                </View>
              </Card>

              <Card title="Advanced tools" subtitle="Developer utilities and debug.">
                <View style={styles.templateWrap}>
                  {ADVANCED_TABS.map((item) => (
                    <Chip key={item.id} label={item.label} active={tab === item.id} onPress={() => setTab(item.id)} />
                  ))}
                </View>
              </Card>

              <Text style={styles.versionText}>LQD Mobile Wallet v1.2.0 • Stable</Text>
            </View>
          )}
        </ScrollView>

        {/* Persistent Browser (Hidden when not active to prevent unmounting) */}
        <View style={{
          flex: tab === 'browser' ? 1 : 0,
          backgroundColor: '#070a15',
          display: tab === 'browser' ? 'flex' : 'none',
          paddingTop: Platform.OS === 'ios' ? 0 : scale(30)
        }}>
          <View style={styles.topBar}>
            <View style={{ flexDirection: 'row', alignItems: 'center', paddingHorizontal: scale(12), marginBottom: scale(8) }}>
              <TouchableOpacity onPress={() => setBrowserVisible(true)} style={{ flex: 1, height: scale(38), backgroundColor: '#161b33', borderRadius: 10, flexDirection: 'row', alignItems: 'center', paddingHorizontal: scale(12), borderWidth: 1, borderColor: '#273152' }}>
                <View style={{ width: scale(8), height: scale(8), borderRadius: 4, backgroundColor: '#10b981', marginRight: scale(8) }} />
                <Text numberOfLines={1} style={{ color: '#9aa5ca', fontSize: scale(12), flex: 1 }}>{browserUrl && !isDefaultBrowserUrl(browserUrl) ? browserUrl : "Search or enter URL"}</Text>
                <Text style={{ color: '#8a78ff', fontSize: scale(16), marginLeft: scale(8) }}>🌐</Text>
              </TouchableOpacity>
            </View>
            <View style={{ flexDirection: 'row', alignItems: 'center', justifyContent: 'space-around', paddingHorizontal: scale(10) }}>
              <TouchableOpacity onPress={() => browserRef.current?.goBack()} disabled={!browserUrl || !browserCanGoBack} style={{ width: scale(40), height: scale(34), borderRadius: 8, backgroundColor: '#161b33', justifyContent: 'center', alignItems: 'center', opacity: browserUrl && browserCanGoBack ? 1 : 0.3 }}>
                <Text style={{ color: '#fff', fontSize: scale(18), fontWeight: 'bold' }}>‹</Text>
              </TouchableOpacity>
              <TouchableOpacity onPress={() => browserRef.current?.goForward()} disabled={!browserUrl || !browserCanGoForward} style={{ width: scale(40), height: scale(34), borderRadius: 8, backgroundColor: '#161b33', justifyContent: 'center', alignItems: 'center', opacity: browserUrl && browserCanGoForward ? 1 : 0.3 }}>
                <Text style={{ color: '#fff', fontSize: scale(18), fontWeight: 'bold' }}>›</Text>
              </TouchableOpacity>
              <TouchableOpacity onPress={() => browserUrl ? browserRef.current?.reload() : setBrowserVisible(true)} style={{ width: scale(40), height: scale(34), borderRadius: 8, backgroundColor: '#161b33', justifyContent: 'center', alignItems: 'center', opacity: browserUrl ? 1 : 0.45 }}>
                <Text style={{ color: '#fff', fontSize: scale(16) }}>↻</Text>
              </TouchableOpacity>
              <TouchableOpacity onPress={() => { setBrowserUrl(""); setBrowserInput(""); }} style={{ width: scale(40), height: scale(34), borderRadius: 8, backgroundColor: '#161b33', justifyContent: 'center', alignItems: 'center' }}>
                <Text style={{ color: '#fff', fontSize: scale(14) }}>🏠</Text>
              </TouchableOpacity>
              <TouchableOpacity onPress={async () => { if (!browserUrl) { setBrowserVisible(true); return; } const canOpen = await Linking.canOpenURL(browserUrl); if (canOpen) Linking.openURL(browserUrl); else showToast("Cannot open this URL externally", "error"); }} style={{ paddingHorizontal: scale(12), height: scale(34), borderRadius: 8, backgroundColor: '#1e293b', justifyContent: 'center', alignItems: 'center', borderWidth: 1, borderColor: '#273152', opacity: browserUrl ? 1 : 0.6 }}>
                <Text style={{ color: '#9aa5ca', fontSize: scale(11), fontWeight: '700' }}>EXTERNAL</Text>
              </TouchableOpacity>
            </View>
            {browserHistory.length > 0 && (
              <ScrollView horizontal showsHorizontalScrollIndicator={false} style={{ marginTop: scale(8) }} contentContainerStyle={{ paddingHorizontal: scale(12), gap: scale(8) }}>
                {browserHistory.slice(0, 5).map((item) => (
                  <TouchableOpacity
                    key={item}
                    onPress={() => {
                      setBrowserUrl(item);
                      setBrowserInput("");
                      rememberBrowserUrl(item);
                    }}
                    style={{ maxWidth: scale(180), paddingHorizontal: scale(10), height: scale(30), borderRadius: 999, backgroundColor: '#161b33', borderWidth: 1, borderColor: '#273152', justifyContent: 'center' }}
                  >
                    <Text numberOfLines={1} style={{ color: '#9aa5ca', fontSize: scale(11), fontWeight: '700' }}>{item}</Text>
                  </TouchableOpacity>
                ))}
              </ScrollView>
            )}
          </View>

          {browserUrl ? (
            <View style={{ flex: 1 }}>
              <WebView
                ref={browserRef}
                source={{ uri: browserUrl }}
                injectedJavaScript={lqdProviderScript}
                onMessage={handleBrowserProviderMessage}
                originWhitelist={["https://*", "http://127.0.0.1:*", "http://localhost:*"]}
                mixedContentMode="never"
                javaScriptCanOpenWindowsAutomatically={false}
                allowFileAccess={false}
                allowUniversalAccessFromFileURLs={false}
                onError={(e) => setStatusModal({ visible: true, title: "Browser Error", message: `Failed to load page: ${e.nativeEvent.description || "Unknown error"}`, type: "error", hash: "" })}
                onNavigationStateChange={(navState) => {
                  setBrowserUrl(navState.url);
                  setBrowserCanGoBack(navState.canGoBack);
                  setBrowserCanGoForward(navState.canGoForward);
                  setBrowserLoading(navState.loading);
                }}
                style={{ flex: 1 }}
                backgroundColor="#070a15"
                startInLoadingState={true}
                renderLoading={() => <View style={[StyleSheet.absoluteFill, { backgroundColor: '#070a15', justifyContent: 'center', alignItems: 'center' }]}><ActivityIndicator color="#8a78ff" /></View>}
              />
            </View>
          ) : (
            <ScrollView style={{ flex: 1 }} contentContainerStyle={{ padding: scale(20), paddingBottom: scale(120) }}>
              <Card title="Browser history" subtitle="Open a trusted dApp or website from your recent searches.">
                <Button label="Search or enter URL" onPress={() => setBrowserVisible(true)} primary />
                {browserHistory.length > 0 ? (
                  <View style={{ marginTop: scale(16) }}>
                    {browserHistory.map((item) => (
                      <TouchableOpacity
                        key={item}
                        onPress={() => {
                          setBrowserUrl(item);
                          setBrowserInput("");
                          rememberBrowserUrl(item);
                        }}
                        style={{ paddingVertical: scale(12), borderTopWidth: 1, borderTopColor: '#273152' }}
                      >
                        <Text numberOfLines={1} style={{ color: '#d8def8', fontSize: scale(14), fontWeight: '700' }}>{item}</Text>
                      </TouchableOpacity>
                    ))}
                    <Button label="Clear History" onPress={() => setBrowserHistory([])} secondary />
                  </View>
                ) : (
                  <Text style={{ color: '#9aa5ca', fontSize: scale(14), lineHeight: scale(22), marginTop: scale(16) }}>
                    No browser history yet. Tap the search box above and enter a dApp link.
                  </Text>
                )}
              </Card>
            </ScrollView>
          )}

          <Modal visible={browserVisible} transparent animationType="fade" onRequestClose={() => setBrowserVisible(false)}>
            <View style={{ flex: 1, backgroundColor: 'rgba(7, 10, 21, 0.9)', justifyContent: 'center', alignItems: 'center', padding: scale(20) }}>
              <View style={{ width: '100%', maxWidth: scale(340) }}>
                <Card title="Browser" subtitle="Enter a dApp or Website URL.">
                  <Field value={browserInput} onChangeText={setBrowserInput} placeholder="https://..." autoCapitalize="none" autoFocus />
                  {browserHistory.length > 0 && (
                    <View style={{ marginTop: scale(12), marginBottom: scale(4) }}>
                      <View style={{ flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', marginBottom: scale(8) }}>
                        <Text style={{ color: '#9aa5ca', fontSize: scale(12), fontWeight: '800', letterSpacing: 0.6 }}>RECENT SEARCHES</Text>
                        <TouchableOpacity onPress={() => setBrowserHistory([])}>
                          <Text style={{ color: '#8a78ff', fontSize: scale(12), fontWeight: '700' }}>Clear</Text>
                        </TouchableOpacity>
                      </View>
                      {browserHistory.map((item) => (
                        <TouchableOpacity
                          key={item}
                          onPress={() => {
                            setBrowserUrl(item);
                            setBrowserInput("");
                            rememberBrowserUrl(item);
                            setBrowserVisible(false);
                          }}
                          style={{ paddingVertical: scale(9), borderTopWidth: 1, borderTopColor: '#273152' }}
                        >
                          <Text numberOfLines={1} style={{ color: '#d8def8', fontSize: scale(13) }}>{item}</Text>
                        </TouchableOpacity>
                      ))}
                    </View>
                  )}
                  <View style={styles.inlineButtons}>
                    <Button label="Go" onPress={() => openBrowserUrl(browserInput)} primary />
                    <Button label="Home" onPress={() => { setBrowserUrl(""); setBrowserInput(""); setBrowserVisible(false); }} secondary />
                    <Button label="Cancel" onPress={() => setBrowserVisible(false)} secondary />
                  </View>
                </Card>
              </View>
            </View>
          </Modal>
        </View>

        {/* Approvals Tab (Centered Popup Design) */}
        {tab === "approvals" && (
          <View style={[StyleSheet.absoluteFill, { backgroundColor: '#070a15', justifyContent: 'center', alignItems: 'center', padding: scale(20), bottom: scale(80), zIndex: 500 }]}>
            {pendingApprovals.length === 0 ? (
              <View style={{ alignItems: 'center' }}>
                <Text style={{ color: '#9aa5ca', fontSize: scale(18), marginBottom: scale(20) }}>No Pending Requests</Text>
                <Button label="Go to Browser" onPress={() => setTab("browser")} primary />
              </View>
            ) : (
              (() => {
                const req = pendingApprovals[0];
                let displayVal = "0";
                try {
                  const rawVal = req.data?.value || "0";
                  displayVal = rawVal.startsWith("0x") ? BigInt(rawVal).toString() : rawVal;
                } catch { displayVal = "0"; }
                const requestTitle = req.type === 'connect'
                  ? 'Connect Wallet'
                  : req.type === 'transaction'
                    ? 'Confirm Transaction'
                    : 'Sign Message';
                const requestIcon = req.type === 'connect' ? '🔗' : req.type === 'transaction' ? '🧾' : '✍️';
                const requestDescription = req.type === 'connect'
                  ? 'This dApp wants to view your wallet address and request transactions.'
                  : req.type === 'transaction'
                    ? 'This dApp wants to submit a transaction. Review every field before approving.'
                    : 'This dApp wants your wallet signature. Only sign messages from sites you trust.';
                const messagePreview = String(req.data?.message || req.data?.params?.[0] || "").slice(0, 160);

                return (
                  <View style={{ width: '100%', maxWidth: scale(340), backgroundColor: '#161b33', borderRadius: scale(24), padding: scale(24), borderWidth: 1, borderColor: '#273152', shadowColor: "#000", shadowOffset: { width: 0, height: 20 }, shadowOpacity: 0.5, shadowRadius: 30, elevation: 20 }}>
                    <View style={{ alignItems: 'center', marginBottom: scale(20) }}>
                      <View style={{ width: scale(60), height: scale(60), borderRadius: scale(30), backgroundColor: '#1e293b', justifyContent: 'center', alignItems: 'center', marginBottom: scale(12) }}>
                        <Text style={{ fontSize: scale(30) }}>{requestIcon}</Text>
                      </View>
                      <Text style={{ color: '#fff', fontSize: scale(20), fontWeight: 'bold', textAlign: 'center' }}>{requestTitle}</Text>
                      <Text style={{ color: '#8a78ff', fontSize: scale(13), marginTop: scale(4), textAlign: 'center' }}>{req.name || 'dApp'} ({req.origin})</Text>
                    </View>

                    <View style={{ padding: scale(16), backgroundColor: '#0f152a', borderRadius: scale(16), marginBottom: scale(24) }}>
                      <Text style={{ color: '#f4f7ff', fontSize: scale(14), lineHeight: scale(20), textAlign: 'center' }}>
                        {requestDescription}
                      </Text>
                      <Text style={{ color: '#717da4', fontSize: scale(11), marginTop: scale(10), textAlign: 'center' }}>Method: {req.method || 'wallet_request'}</Text>
                      {req.type === 'transaction' && (
                        <View style={{ marginTop: scale(12), borderTopWidth: 1, borderTopColor: '#1b2342', paddingTop: scale(12) }}>
                          <Text style={{ color: '#9aa5ca', fontSize: scale(12) }}>To: <Text style={{ color: '#8a78ff' }}>{shortAddress(req.data?.to || req.data?.contract_address || req.data?.contractAddress || "")}</Text></Text>
                          <Text style={{ color: '#9aa5ca', fontSize: scale(12), marginTop: scale(4) }}>Value: <Text style={{ color: '#fff', fontWeight: 'bold' }}>{formatUnits(displayVal, currentNetwork.decimals || 8, 4)} {currentNetwork.symbol}</Text></Text>
                          {!!req.data?.data && req.data.data !== "0x" && (
                            <Text style={{ color: '#9aa5ca', fontSize: scale(12), marginTop: scale(4) }}>Data: <Text style={{ color: '#f59e0b' }}>{String(req.data.data).slice(0, 34)}...</Text></Text>
                          )}
                        </View>
                      )}
                      {req.type === 'sign' && !!messagePreview && (
                        <View style={{ marginTop: scale(12), borderTopWidth: 1, borderTopColor: '#1b2342', paddingTop: scale(12) }}>
                          <Text style={{ color: '#9aa5ca', fontSize: scale(12) }}>Message:</Text>
                          <Text style={{ color: '#f4f7ff', fontSize: scale(12), marginTop: scale(4) }}>{messagePreview}{messagePreview.length >= 160 ? "..." : ""}</Text>
                        </View>
                      )}
                    </View>

                    <View style={styles.inlineButtons}>
                      <Button label="Approve" onPress={() => approveRequest(req)} primary />
                      <Button label="Reject" onPress={() => rejectRequest(req)} danger />
                    </View>

                    <TouchableOpacity onPress={() => setTab("browser")} style={{ marginTop: scale(16), alignItems: 'center' }}>
                      <Text style={{ color: '#717da4', fontSize: scale(13) }}>Cancel & Return</Text>
                    </TouchableOpacity>
                  </View>
                );
              })()
            )}
          </View>
        )}

        <View style={styles.bottomNav}>
          {TABS.map((item) => (
            <NavItem key={item.id} icon={item.icon} label={item.label} active={tab === item.id} onPress={() => setTab(item.id)} />
          ))}
        </View>
      </KeyboardAvoidingView>

      {/* ── Premium Toast Notification ────────────────────────────────────────── */}
      {toast.visible && (
        <View pointerEvents="none" style={{
          position: 'absolute', bottom: scale(90), left: scale(16), right: scale(16),
          zIndex: 9999999, alignItems: 'center',
        }}>
          <View style={{
            flexDirection: 'row', alignItems: 'center',
            paddingVertical: scale(13), paddingHorizontal: scale(18),
            borderRadius: scale(16), maxWidth: scale(340),
            backgroundColor: toast.type === 'success' ? '#0d2318'
              : toast.type === 'error' ? '#1f0e0e'
              : '#0e1528',
            borderWidth: 1,
            borderColor: toast.type === 'success' ? '#16a34a'
              : toast.type === 'error' ? '#dc2626'
              : '#334070',
            shadowColor: toast.type === 'success' ? '#16a34a'
              : toast.type === 'error' ? '#dc2626'
              : '#8a78ff',
            shadowOffset: { width: 0, height: 4 }, shadowOpacity: 0.35, shadowRadius: 16, elevation: 16,
          }}>
            {/* Icon dot */}
            <View style={{
              width: scale(8), height: scale(8), borderRadius: scale(4), marginRight: scale(12),
              backgroundColor: toast.type === 'success' ? '#22c55e'
                : toast.type === 'error' ? '#ef4444'
                : '#8a78ff',
            }} />
            <Text style={{
              flex: 1, fontSize: scale(13), fontWeight: '700', lineHeight: scale(18),
              color: toast.type === 'success' ? '#bbf7d0'
                : toast.type === 'error' ? '#fecaca'
                : '#c7d2fe',
              letterSpacing: 0.2,
            }}>
              {toast.message}
            </Text>
          </View>
        </View>
      )}

      {!!processingMessage && (
        <View style={[StyleSheet.absoluteFill, { backgroundColor: 'rgba(5, 8, 18, 0.96)', zIndex: 999999, justifyContent: 'center', alignItems: 'center', padding: scale(40) }]}>
          <View style={{ backgroundColor: '#0f1428', padding: scale(36), borderRadius: scale(28), alignItems: 'center', borderWidth: 1, borderColor: '#1e2d52', shadowColor: '#8a78ff', shadowOffset: { width: 0, height: 0 }, shadowOpacity: 0.25, shadowRadius: 40, elevation: 24, minWidth: scale(240) }}>

            {/* Premium triple-ring spinner */}
            <View style={{ width: scale(88), height: scale(88), justifyContent: 'center', alignItems: 'center', marginBottom: scale(28) }}>
              {/* Outer static track */}
              <View style={{ position: 'absolute', width: scale(88), height: scale(88), borderRadius: scale(44), borderWidth: 2, borderColor: '#1e2d52' }} />
              {/* Outer spinning arc — purple */}
              <Animated.View style={{ position: 'absolute', width: scale(88), height: scale(88), borderRadius: scale(44), borderWidth: 2.5, borderTopColor: '#8a78ff', borderRightColor: '#6d5ce6', borderBottomColor: 'transparent', borderLeftColor: 'transparent', transform: [{ rotate: _rotateDeg }] }} />
              {/* Middle static track */}
              <View style={{ position: 'absolute', width: scale(68), height: scale(68), borderRadius: scale(34), borderWidth: 1.5, borderColor: '#1a2545' }} />
              {/* Middle spinning arc — indigo counter-clockwise */}
              <Animated.View style={{ position: 'absolute', width: scale(68), height: scale(68), borderRadius: scale(34), borderWidth: 2, borderTopColor: 'transparent', borderRightColor: 'transparent', borderBottomColor: '#5b8af5', borderLeftColor: '#3b6fd4', transform: [{ rotate: _rotateDeg2 }] }} />
              {/* Inner glowing core */}
              <View style={{ width: scale(44), height: scale(44), borderRadius: scale(22), backgroundColor: '#141c38', borderWidth: 1, borderColor: '#2a3a64', justifyContent: 'center', alignItems: 'center' }}>
                <View style={{ width: scale(16), height: scale(16), borderRadius: scale(8), backgroundColor: '#8a78ff', opacity: 0.9 }} />
              </View>
            </View>

            {/* Message */}
            <Text style={{ color: '#f0f4ff', fontSize: scale(16), fontWeight: '800', textAlign: 'center', letterSpacing: 0.3, marginBottom: scale(8) }}>
              {processingMessage}
            </Text>
            <Text style={{ color: '#5a6a94', fontSize: scale(12), textAlign: 'center', lineHeight: scale(17), letterSpacing: 0.2 }}>
              Please wait, do not close the app
            </Text>

            {/* Bottom shimmer bar */}
            <View style={{ marginTop: scale(24), width: '70%', height: 2, borderRadius: 1, backgroundColor: '#1e2d52', overflow: 'hidden' }}>
              <Animated.View style={{ height: '100%', width: '50%', borderRadius: 1, backgroundColor: '#8a78ff', opacity: 0.6, transform: [{ translateX: _spinAnim.interpolate({ inputRange: [0, 1], outputRange: [scale(-60), scale(120)] }) }] }} />
            </View>
          </View>
        </View>
      )}

      {/* Transaction Story Modal */}
      <Modal visible={!!selectedTxStory} transparent animationType="fade" onRequestClose={() => setSelectedTxStory(null)}>
        <View style={{ flex: 1, backgroundColor: 'rgba(7, 10, 21, 0.9)', justifyContent: 'center', alignItems: 'center', padding: scale(20) }}>
          <View style={{ width: '100%', maxWidth: scale(360), backgroundColor: '#161b33', borderRadius: scale(28), padding: scale(24), borderWidth: 1, borderColor: '#273152' }}>
            <View style={{ flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center', marginBottom: scale(20) }}>
              <Text style={{ color: '#fff', fontSize: scale(18), fontWeight: 'bold' }}>Transaction Story</Text>
              <TouchableOpacity onPress={() => setSelectedTxStory(null)}>
                <Text style={{ color: '#717da4', fontSize: scale(20) }}>✕</Text>
              </TouchableOpacity>
            </View>

            {selectedTxStory && (() => {
              const s = String(selectedTxStory.Status || selectedTxStory.status || "pending").toLowerCase();
              const isSuccess = s.includes("complete") || s.includes("success") || s.includes("confirmed");
              const isFailed = s.includes("fail") || s.includes("error") || s.includes("reject");
              const color = isSuccess ? "#4ade80" : (isFailed ? "#f87171" : "#fbbf24");

              return (
                <View>
                  <View style={{ alignItems: 'center', marginBottom: scale(24) }}>
                    <View style={{ width: scale(64), height: scale(64), borderRadius: 32, backgroundColor: color + '20', justifyContent: 'center', alignItems: 'center', marginBottom: scale(12), borderWidth: 1, borderColor: color }}>
                      <Text style={{ fontSize: scale(28) }}>{isSuccess ? "✅" : (isFailed ? "❌" : "⏳")}</Text>
                    </View>
                    <Text style={{ color: color, fontSize: scale(16), fontWeight: 'bold' }}>{s.toUpperCase()}</Text>
                    <Text style={{ color: '#717da4', fontSize: scale(12), marginTop: scale(4) }}>{formatDate(selectedTxStory.Timestamp || selectedTxStory.timestamp)}</Text>
                  </View>

                  <View style={{ gap: scale(16) }}>
                    <View style={{ flexDirection: 'row' }}>
                      <View style={{ alignItems: 'center', marginRight: scale(12) }}>
                        <View style={{ width: scale(12), height: scale(12), borderRadius: 6, backgroundColor: '#8a78ff' }} />
                        <View style={{ width: 2, flex: 1, backgroundColor: '#273152', marginVertical: 4 }} />
                      </View>
                      <View style={{ flex: 1 }}>
                        <Text style={{ color: '#fff', fontWeight: 'bold', fontSize: scale(14) }}>Initiated</Text>
                        <Text style={{ color: '#717da4', fontSize: scale(12) }}>Transaction broadcasted from {shortAddress(selectedTxStory.From || selectedTxStory.from)}</Text>
                      </View>
                    </View>

                    <View style={{ flexDirection: 'row' }}>
                      <View style={{ alignItems: 'center', marginRight: scale(12) }}>
                        <View style={{ width: scale(12), height: scale(12), borderRadius: 6, backgroundColor: isSuccess || isFailed ? '#8a78ff' : '#273152' }} />
                        <View style={{ width: 2, flex: 1, backgroundColor: '#273152', marginVertical: 4 }} />
                      </View>
                      <View style={{ flex: 1 }}>
                        <Text style={{ color: isSuccess || isFailed ? '#fff' : '#717da4', fontWeight: 'bold', fontSize: scale(14) }}>Validation</Text>
                        <Text style={{ color: '#717da4', fontSize: scale(12) }}>Nodes are verifying the cryptographic signatures</Text>
                      </View>
                    </View>

                    <View style={{ flexDirection: 'row' }}>
                      <View style={{ alignItems: 'center', marginRight: scale(12) }}>
                        <View style={{ width: scale(12), height: scale(12), borderRadius: 6, backgroundColor: isSuccess ? '#4ade80' : (isFailed ? '#f87171' : '#273152') }} />
                      </View>
                      <View style={{ flex: 1 }}>
                        <Text style={{ color: isSuccess ? '#4ade80' : (isFailed ? '#f87171' : '#717da4'), fontWeight: 'bold', fontSize: scale(14) }}>Result</Text>
                        <Text style={{ color: '#717da4', fontSize: scale(12) }}>{isSuccess ? "Transaction confirmed on-chain" : (isFailed ? "Transaction was rejected by the network" : "Waiting for finality...")}</Text>
                      </View>
                    </View>
                  </View>

                  <View style={{ marginTop: scale(24), padding: scale(16), backgroundColor: '#0f152a', borderRadius: scale(16), gap: scale(8) }}>
                    <View style={{ flexDirection: 'row', justifyContent: 'space-between' }}>
                      <Text style={{ color: '#717da4', fontSize: scale(12) }}>Type</Text>
                      <Text style={{ color: '#fff', fontSize: scale(12), fontWeight: 'bold' }}>{(selectedTxStory.Type || selectedTxStory.type || "TX").toUpperCase()}</Text>
                    </View>
                    <View style={{ flexDirection: 'row', justifyContent: 'space-between' }}>
                      <Text style={{ color: '#717da4', fontSize: scale(12) }}>Value</Text>
                      <Text style={{ color: '#fff', fontSize: scale(12), fontWeight: 'bold' }}>{selectedTxStory.Value || selectedTxStory.amount || "0"} {selectedTxStory.Symbol || "LQD"}</Text>
                    </View>
                    <TouchableOpacity onPress={() => Clipboard.setStringAsync(selectedTxStory.TxHash || selectedTxStory.tx_hash || "")} style={{ flexDirection: 'row', justifyContent: 'space-between' }}>
                      <Text style={{ color: '#717da4', fontSize: scale(12) }}>TX Hash</Text>
                      <Text style={{ color: '#8a78ff', fontSize: scale(12) }}>{shortAddress(selectedTxStory.TxHash || selectedTxStory.tx_hash || "", 6, 4)} 📋</Text>
                    </TouchableOpacity>
                  </View>

                  <Button label="Close" onPress={() => setSelectedTxStory(null)} secondary style={{ marginTop: scale(20) }} />
                </View>
              );
            })()}
          </View>
        </View>
      </Modal>

      {/* Bridge Tracking Popup (Real-time tracking) */}
      <Modal visible={!!activeBridgeTx} transparent animationType="slide" onRequestClose={() => setActiveBridgeTx(null)}>
        <View style={{ flex: 1, backgroundColor: 'rgba(7, 10, 21, 0.95)', justifyContent: 'flex-end' }}>
          <View style={{ backgroundColor: '#161b33', borderTopLeftRadius: scale(32), borderTopRightRadius: scale(32), padding: scale(24), paddingBottom: scale(40), borderWidth: 1, borderColor: '#273152' }}>
            <View style={{ width: scale(40), height: scale(4), backgroundColor: '#273152', borderRadius: 2, alignSelf: 'center', marginBottom: scale(20) }} />

            <View style={{ flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center', marginBottom: scale(24) }}>
              <View>
                <Text style={{ color: '#f4f7ff', fontSize: scale(20), fontWeight: '800' }}>Bridge Tracking</Text>
                <Text style={{ color: '#8a78ff', fontSize: scale(12) }}>ID: {shortAddress(activeBridgeTx?.tx_hash || "", 8, 4)}</Text>
              </View>
              <TouchableOpacity onPress={() => setActiveBridgeTx(null)} style={{ padding: scale(8) }}>
                <Text style={{ color: '#717da4', fontSize: scale(18) }}>✕</Text>
              </TouchableOpacity>
            </View>

            <View style={{ marginBottom: scale(30) }}>
              {/* Timeline Steps */}
              {(() => {
                const status = String(activeBridgeTx?.status || "pending").toLowerCase();
                const failed = status.includes("fail") || status.includes("error") || status.includes("reject");
                const complete = status.includes("mint") || status.includes("unlock") || status.includes("complete") || status.includes("success") || status.includes("confirmed");
                const processing = !failed && !complete && (status.includes("process") || status.includes("queue") || status.includes("lock") || status.includes("burn") || status.includes("pending"));
                return [
                  { label: "Initiated", desc: "Bridge request accepted", status: failed ? "completed" : "completed" },
                  { label: "Validation", desc: "Relayer validating cross-chain state", status: failed ? "failed" : (complete ? "completed" : (processing ? "processing" : "pending")) },
                  { label: "Completion", desc: "Assets finalizing on target chain", status: failed ? "failed" : (complete ? "completed" : "pending") }
                ];
              })().map((step, i) => (
                <View key={i} style={{ flexDirection: 'row', marginBottom: scale(20) }}>
                  <View style={{ alignItems: 'center', marginRight: scale(16) }}>
                    <View style={{
                      width: scale(24),
                      height: scale(24),
                      borderRadius: 12,
                      backgroundColor: step.status === 'completed' ? '#4ade80' : (step.status === 'processing' ? '#8a78ff' : (step.status === 'failed' ? '#f87171' : '#0f152a')),
                      justifyContent: 'center',
                      alignItems: 'center',
                      borderWidth: 2,
                      borderColor: step.status === 'pending' ? '#273152' : 'transparent'
                    }}>
                      {step.status === 'completed' ? <Text style={{ fontSize: scale(12) }}>✓</Text> : (step.status === 'processing' ? <ActivityIndicator size="small" color="#fff" /> : (step.status === 'failed' ? <Text style={{ color: "#fff", fontSize: scale(12) }}>!</Text> : null))}
                    </View>
                    {i < 2 && <View style={{ width: 2, height: scale(20), backgroundColor: '#273152', marginTop: 4 }} />}
                  </View>
                  <View style={{ flex: 1 }}>
                    <Text style={{ color: step.status === 'pending' ? '#717da4' : '#fff', fontWeight: 'bold', fontSize: scale(14) }}>{step.label}</Text>
                    <Text style={{ color: '#717da4', fontSize: scale(11) }}>{step.desc}</Text>
                  </View>
                </View>
              ))}
            </View>

            <View style={{ padding: scale(16), backgroundColor: '#0f152a', borderRadius: scale(20), marginBottom: scale(24) }}>
              <View style={{ flexDirection: 'row', justifyContent: 'space-between', marginBottom: scale(8) }}>
                <Text style={{ color: '#717da4', fontSize: scale(12) }}>Amount</Text>
                <Text style={{ color: '#fff', fontSize: scale(12), fontWeight: 'bold' }}>{activeBridgeTx?.amount} LQD</Text>
              </View>
              <View style={{ flexDirection: 'row', justifyContent: 'space-between' }}>
                <Text style={{ color: '#717da4', fontSize: scale(12) }}>Estimated Time</Text>
                <Text style={{ color: '#fbbf24', fontSize: scale(12), fontWeight: 'bold' }}>~5-10 Minutes</Text>
              </View>
            </View>

            <Button label="Close & Track in Background" onPress={() => setActiveBridgeTx(null)} primary />
            <Text style={{ color: '#717da4', fontSize: scale(11), textAlign: 'center', marginTop: scale(16) }}>You will receive a notification once the bridge is complete.</Text>
          </View>
        </View>
      </Modal>

    </SafeAreaView>
  );

}

const styles = StyleSheet.create({
  safe: {
    flex: 1,
    backgroundColor: "#0b1020",
  },
  centerScreen: {
    flex: 1,
    justifyContent: "center",
    alignItems: "center",
    padding: scale(24),
  },
  heroTitle: {
    color: "#f4f7ff",
    fontSize: scale(30),
    fontWeight: "800",
    marginBottom: scale(8),
  },
  heroText: {
    color: "#9ea8cc",
    fontSize: scale(15),
    textAlign: "center",
    lineHeight: scale(22),
  },
  scrollPad: {
    padding: scale(18),
    paddingBottom: scale(36),
  },
  topActions: {
    flexDirection: "row",
    gap: scale(8),
    alignItems: "center",
    justifyContent: "flex-end",
  },
  topBar: {
    backgroundColor: '#070a15',
    borderBottomWidth: 1,
    borderBottomColor: '#161b33',
  },
  topActionPill: {
    paddingHorizontal: scale(14),
    paddingVertical: scale(6),
    borderRadius: scale(20),
    backgroundColor: '#161b33',
    borderWidth: 1,
    borderColor: '#273152',
    justifyContent: 'center',
    alignItems: 'center',
  },
  topActionPillText: {
    color: '#9aa5ca',
    fontSize: scale(11),
    fontWeight: '700',
  },
  topRefreshBtn: {
    paddingHorizontal: scale(12),
    paddingVertical: scale(6),
    backgroundColor: 'rgba(138, 120, 255, 0.15)',
    borderRadius: scale(8),
    borderWidth: 1,
    borderColor: 'rgba(138, 120, 255, 0.3)',
  },
  hamburgerBtn: {
    width: scale(34),
    height: scale(34),
    justifyContent: 'center',
    alignItems: 'flex-end',
    gap: scale(4),
  },
  hamburgerLine: {
    width: scale(20),
    height: 2,
    backgroundColor: '#8a78ff',
    borderRadius: 1,
  },
  menuItem: {
    flexDirection: 'row',
    alignItems: 'center',
    padding: scale(14),
    borderBottomWidth: 1,
    borderBottomColor: '#273152',
  },
  menuItemIcon: {
    fontSize: scale(18),
    marginRight: scale(12),
  },
  menuItemText: {
    color: '#f4f7ff',
    fontSize: scale(14),
    fontWeight: '600',
  },
  topBarBrowser: {
    paddingBottom: scale(8),
  },
  topIdentity: {
    alignItems: "center",
  },
  topAddress: {
    color: "#dce4ff",
    fontSize: scale(16),
    fontWeight: "800",
    letterSpacing: 0.2,
  },
  topNetwork: {
    color: "#9ea8cc",
    fontSize: scale(13),
    marginTop: scale(3),
    fontWeight: "600",
  },
  walletPill: {
    backgroundColor: "#16203a",
    borderColor: "#273152",
    borderWidth: 1,
    borderRadius: 999,
    paddingVertical: scale(10),
    paddingHorizontal: scale(16),
    minWidth: scale(92),
    alignItems: "center",
  },
  walletPillState: {
    backgroundColor: "#18252e",
    borderColor: "#24413d",
  },
  walletPillText: {
    color: "#91f7bf",
    fontWeight: "700",
  },
  mainScroll: {
    padding: scale(16),
    paddingBottom: scale(100),
    flexGrow: 1,
  },
  mainScrollBrowser: {
    paddingHorizontal: 0,
    paddingTop: 0,
    paddingBottom: 0,
  },
  tabRow: {
    flexDirection: "row",
    flexWrap: "wrap",
    gap: scale(8),
    marginBottom: scale(14),
  },
  chip: {
    paddingVertical: scale(9),
    paddingHorizontal: scale(12),
    borderRadius: 999,
    backgroundColor: "#151b31",
    borderWidth: 1,
    borderColor: "#273152",
  },
  chipActive: {
    backgroundColor: "#3a2f72",
    borderColor: "#9c86ff",
  },
  chipText: {
    color: "#a4afcf",
    fontSize: scale(12),
    fontWeight: "700",
  },
  chipTextActive: {
    color: "#f6f3ff",
  },
  summaryGrid: {
    flexDirection: "row",
    gap: scale(10),
    flexWrap: "wrap",
  },
  statsGrid: {
    flexDirection: "row",
    flexWrap: "wrap",
    gap: scale(10),
  },
  actionGrid: {
    flexDirection: "row",
    flexWrap: "wrap",
    gap: scale(10),
  },
  stat: {
    flexGrow: 1,
    flexBasis: "30%",
    minWidth: scale(90),
    backgroundColor: "#151b31",
    borderColor: "#273152",
    borderWidth: 1,
    borderRadius: scale(18),
    padding: scale(14),
  },
  statLabel: {
    color: "#8b94b7",
    fontSize: scale(12),
    marginBottom: scale(6),
    fontWeight: "700",
  },
  statValue: {
    color: "#f4f7ff",
    fontSize: scale(18),
    fontWeight: "800",
  },
  statSub: {
    color: "#91f7bf",
    fontSize: scale(12),
    marginTop: scale(4),
    fontWeight: "700",
  },
  lpMiniStat: {
    flexGrow: 1,
    flexBasis: "46%",
    minWidth: scale(126),
    backgroundColor: "#10172a",
    borderColor: "#273152",
    borderWidth: 1,
    borderRadius: scale(14),
    padding: scale(11),
  },
  lpMiniLabel: {
    color: "#7d89b2",
    fontSize: scale(10),
    fontWeight: "800",
    textTransform: "uppercase",
    marginBottom: scale(4),
  },
  lpMiniValue: {
    color: "#f4f7ff",
    fontSize: scale(13),
    fontWeight: "900",
  },
  resultBox: {
    backgroundColor: "#10172a",
    borderColor: "#273152",
    borderWidth: 1,
    borderRadius: scale(14),
    padding: scale(12),
  },
  sectionGap: {
    gap: scale(12),
    marginTop: scale(12),
  },
  sectionGapSmall: {
    gap: scale(10),
    marginTop: scale(10),
  },
  browserSection: {
    flex: 1,
    gap: 0,
    marginTop: 0,
    minHeight: 0,
  },
  card: {
    backgroundColor: "#151b31",
    borderColor: "#273152",
    borderWidth: 1,
    borderRadius: scale(22),
    padding: scale(15),
    gap: scale(10),
  },
  browserCard: {
    flex: 1,
    borderRadius: 0,
    borderLeftWidth: 0,
    borderRightWidth: 0,
    borderTopWidth: 1,
    paddingHorizontal: scale(8),
    paddingTop: scale(8),
    paddingBottom: scale(6),
    gap: scale(8),
    minHeight: 0,
  },
  cardHeader: {
    marginBottom: scale(2),
  },
  cardTitle: {
    color: "#f4f7ff",
    fontSize: scale(18),
    fontWeight: "800",
  },
  cardSubtitle: {
    color: "#9ca7ca",
    fontSize: scale(13),
    marginTop: scale(4),
    lineHeight: scale(19),
  },
  fieldWrap: {
    gap: scale(6),
  },
  fieldLabelRow: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
  },
  fieldLabel: {
    color: "#c6cee8",
    fontSize: scale(13),
    fontWeight: "700",
  },
  input: {
    backgroundColor: "#1b2342",
    borderColor: "#2f3960",
    borderWidth: 1,
    borderRadius: scale(16),
    color: "#f4f7ff",
    paddingHorizontal: scale(14),
    paddingVertical: scale(12),
    minHeight: scale(46),
    fontSize: scale(14),
  },
  inputMultiline: {
    minHeight: scale(110),
    textAlignVertical: "top",
  },
  inputReadonly: {
    opacity: 0.8,
  },
  button: {
    backgroundColor: "#8a78ff",
    borderRadius: scale(16),
    paddingVertical: scale(13),
    paddingHorizontal: scale(16),
    alignItems: "center",
    justifyContent: "center",
    minHeight: scale(48),
  },
  buttonSecondary: {
    backgroundColor: "#1d2542",
    borderColor: "#3b4670",
    borderWidth: 1,
  },
  buttonDanger: {
    backgroundColor: "#311822",
    borderColor: "#6d2a38",
    borderWidth: 1,
  },
  buttonCompact: {
    paddingVertical: scale(10),
    paddingHorizontal: scale(12),
    minHeight: scale(38),
    borderRadius: scale(12),
  },
  buttonDisabled: {
    opacity: 0.55,
  },
  buttonPressed: {
    transform: [{ scale: 0.985 }],
  },
  buttonText: {
    color: "#fff",
    fontWeight: "800",
    fontSize: scale(15),
  },
  buttonTextSecondary: {
    color: "#b6c0e6",
  },
  buttonTextDanger: {
    color: "#ffb4c0",
  },
  helperText: {
    color: "#8f9bc1",
    fontSize: scale(12),
    lineHeight: scale(18),
  },
  bottomNav: {
    flexDirection: "row",
    gap: scale(8),
    paddingHorizontal: scale(12),
    paddingTop: scale(10),
    paddingBottom: scale(34),
    backgroundColor: '#070a15',
    borderTopWidth: 1,
    borderTopColor: '#161b33',
  },
  navItem: {
    flex: 1,
    minWidth: 0,
    alignItems: "center",
    justifyContent: "center",
    paddingVertical: scale(9),
    paddingHorizontal: scale(6),
    borderRadius: scale(16),
    backgroundColor: "#11182e",
    borderWidth: 1,
    borderColor: "#243056",
  },
  navItemActive: {
    backgroundColor: "#2c2557",
    borderColor: "#8a78ff",
  },
  navIcon: {
    color: "#7f8bb6",
    fontSize: scale(14),
    fontWeight: "800",
    marginBottom: scale(3),
  },
  navIconActive: {
    color: "#f4f7ff",
  },
  navLabel: {
    color: "#9aa5ca",
    fontSize: scale(9),
    fontWeight: "700",
    textAlign: "center",
  },
  navLabelActive: {
    color: "#f4f7ff",
  },
  modalBackdrop: {
    flex: 1,
    backgroundColor: "rgba(3, 6, 12, 0.82)",
    justifyContent: "center",
    padding: scale(18),
  },
  modalCard: {
    backgroundColor: "#151b31",
    borderColor: "#3a4670",
    borderWidth: 1,
    borderRadius: scale(24),
    padding: scale(18),
    gap: scale(12),
  },
  qrWrap: {
    alignItems: "center",
    justifyContent: "center",
    paddingVertical: scale(10),
    backgroundColor: "#0f152a",
    borderRadius: scale(20),
    borderWidth: 1,
    borderColor: "#2a3558",
  },
  scannerBackdrop: {
    flex: 1,
    backgroundColor: "#050814",
    paddingTop: scale(40),
    paddingHorizontal: scale(16),
    paddingBottom: scale(16),
  },
  scannerHeader: {
    gap: scale(10),
    marginBottom: scale(12),
  },
  scannerBody: {
    flex: 1,
    borderRadius: scale(24),
    overflow: "hidden",
    borderWidth: 1,
    borderColor: "#2b3557",
    backgroundColor: "#0d1326",
  },
  cameraFallback: {
    flex: 1,
    alignItems: "center",
    justifyContent: "center",
    padding: scale(18),
    gap: scale(14),
  },
  statusText: {
    color: "#c6d2ff",
    fontSize: scale(13),
    marginTop: scale(14),
    lineHeight: scale(20),
  },
  largeCode: {
    color: "#dce4ff",
    fontSize: scale(13),
    lineHeight: scale(19),
    fontFamily: Platform.select({ ios: "Courier", android: "monospace", default: "monospace" }),
  },
  inlineButtons: {
    flexDirection: "row",
    gap: scale(10),
    flexWrap: "wrap",
  },
  templateWrap: {
    flexDirection: "row",
    flexWrap: "wrap",
    gap: scale(8),
  },
  browserSurface: {
    flex: 1,
    minHeight: 0,
    backgroundColor: "#0f1428",
    borderColor: "#273152",
    borderWidth: 1,
    borderRadius: scale(12),
    overflow: "hidden",
  },
  browserToolbar: {
    flexDirection: "row",
    gap: scale(8),
    padding: scale(12),
    borderBottomColor: "#273152",
    borderBottomWidth: 1,
    flexWrap: "wrap",
  },
  browserFrame: {
    flex: 1,
    minHeight: 0,
    backgroundColor: "#ffffff",
  },
  browserHint: {
    color: "#9aa5ca",
    fontSize: scale(12),
    paddingHorizontal: scale(12),
    paddingTop: scale(8),
  },
  rowCard: {
    backgroundColor: "#10162c",
    borderColor: "#273152",
    borderWidth: 1,
    borderRadius: scale(18),
    padding: scale(14),
    flexDirection: "row",
    alignItems: "flex-start",
    gap: scale(12),
  },
  rowTitle: {
    color: "#f4f7ff",
    fontSize: scale(15),
    fontWeight: "800",
  },
  rowSub: {
    color: "#9aa5ca",
    fontSize: scale(12),
    marginTop: scale(4),
  },
  tokenBalance: {
    color: "#91f7bf",
    fontSize: scale(13),
    fontWeight: "700",
    marginTop: scale(6),
  },
  rowActions: {
    gap: scale(8),
    alignItems: "flex-end",
  },
  inspectTitle: {
    color: "#b9c4e9",
    fontSize: scale(12),
    fontWeight: "700",
    marginTop: scale(8),
  },
  inspectBox: {
    color: "#dce4ff",
    backgroundColor: "#0f1428",
    borderRadius: scale(14),
    borderColor: "#273152",
    borderWidth: 1,
    padding: scale(12),
    fontSize: scale(12),
    lineHeight: scale(18),
    marginTop: scale(8),
  },
  subtabHeader: {
    flexDirection: "row",
    gap: scale(6),
    marginTop: scale(10),
    marginBottom: scale(8),
    flexWrap: "wrap",
  },
  subtabItem: {
    paddingVertical: scale(6),
    paddingHorizontal: scale(12),
    borderRadius: scale(10),
    backgroundColor: "#1b2342",
    borderWidth: 1,
    borderColor: "#303a5e",
  },
  subtabItemActive: {
    backgroundColor: "#2c2557",
    borderColor: "#8a78ff",
  },
  subtabText: {
    color: "#9aa5ca",
    fontSize: scale(12),
    fontWeight: "700",
  },
  subtabTextActive: {
    color: "#f4f7ff",
  },
  inspectContent: {
    marginTop: scale(6),
    gap: scale(6),
  },
  inspectScroll: {
    maxHeight: scale(250),
    marginTop: scale(6),
  },
  eventRow: {
    paddingVertical: scale(8),
    borderBottomWidth: 1,
    borderBottomColor: "#273152",
  },
  eventTitle: {
    color: "#8a78ff",
    fontSize: scale(13),
    fontWeight: "800",
  },
  eventBody: {
    color: "#c6cee8",
    fontSize: scale(11),
    lineHeight: scale(16),
    marginTop: scale(2),
  },
  toast: {
    position: 'absolute',
    top: scale(50),
    left: scale(20),
    right: scale(20),
    padding: scale(16),
    borderRadius: scale(12),
    zIndex: 9999,
    flexDirection: 'row',
    alignItems: 'center',
    shadowColor: "#000",
    shadowOffset: { width: 0, height: 2 },
    shadowOpacity: 0.25,
    shadowRadius: 3.84,
    elevation: 5,
  },
  toastText: {
    color: '#fff',
    fontSize: scale(14),
    fontWeight: '700',
    textAlign: 'center',
    flex: 1,
  },
  dashboardHeader: {
    marginBottom: scale(15),
  },
  actionBtn: {
    flex: 1,
    minWidth: scale(75),
    backgroundColor: '#1b2342',
    borderRadius: scale(18),
    padding: scale(12),
    alignItems: 'center',
    justifyContent: 'center',
    borderWidth: 1,
    borderColor: '#303a5e',
    gap: scale(4),
  },
  actionIcon: {
    fontSize: scale(20),
  },
  actionLabel: {
    color: '#c6cee8',
    fontSize: scale(11),
    fontWeight: '700',
  },
  qrContainer: {
    alignItems: 'center',
    gap: scale(12),
  },
  addressText: {
    color: '#91f7bf',
    fontSize: scale(12),
    backgroundColor: '#0f152a',
    padding: scale(10),
    borderRadius: scale(10),
    width: '100%',
    textAlign: 'center',
    fontFamily: Platform.select({ ios: 'Courier', android: 'monospace' }),
  },
  activityHeader: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: scale(10),
  },
  activityStats: {
    color: '#9aa5ca',
    fontSize: scale(12),
    fontWeight: '600',
  },
  emptyState: {
    alignItems: 'center',
    paddingVertical: scale(40),
    gap: scale(10),
  },
  emptyIcon: {
    fontSize: scale(40),
    opacity: 0.4,
  },
  emptyText: {
    color: '#717da4',
    fontSize: scale(14),
    fontWeight: '600',
  },
  settingsGroup: {
    gap: scale(12),
  },
  settingsLabel: {
    color: '#8a78ff',
    fontSize: scale(12),
    fontWeight: '800',
    textTransform: 'uppercase',
    letterSpacing: 1,
    marginBottom: scale(4),
  },
  settingsDivider: {
    height: 1,
    backgroundColor: '#273152',
    marginVertical: scale(8),
  },
  versionText: {
    color: '#5b648e',
    fontSize: scale(11),
    textAlign: 'center',
    marginTop: scale(10),
    fontWeight: '600',
  },
  // MetaMask Style Extensions
  mmHome: {
    flex: 1,
  },
  walletNetworkPill: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: 'rgba(39, 49, 82, 0.55)',
    paddingHorizontal: scale(16),
    paddingVertical: scale(10),
    borderRadius: scale(24),
    borderWidth: 1,
    borderColor: 'rgba(138, 120, 255, 0.35)',
    marginBottom: scale(6),
    alignSelf: 'center',
  },
  walletHealthDot: {
    width: scale(8),
    height: scale(8),
    borderRadius: scale(4),
    marginRight: scale(10),
  },
  walletHealthText: {
    color: '#8792bb',
    fontSize: scale(11),
    textAlign: 'center',
    marginBottom: scale(14),
    fontWeight: '700',
  },
  walletRefreshMeta: {
    color: '#657096',
    fontSize: scale(11),
    marginTop: scale(6),
    fontWeight: '700',
  },
  walletWarningText: {
    color: '#f59e0b',
    fontSize: scale(10),
    marginTop: scale(4),
    lineHeight: scale(14),
    fontWeight: '700',
  },
  walletTokenBadge: {
    color: '#91f7bf',
    fontSize: scale(11),
    fontWeight: '800',
    backgroundColor: 'rgba(16,185,129,0.12)',
    borderRadius: scale(10),
    paddingHorizontal: scale(8),
    paddingVertical: scale(4),
    overflow: 'hidden',
  },
  walletOfficialBadge: {
    color: '#91f7bf',
    fontSize: scale(9),
    fontWeight: '900',
    backgroundColor: 'rgba(16,185,129,0.12)',
    borderRadius: scale(8),
    paddingHorizontal: scale(6),
    paddingVertical: scale(2),
    overflow: 'hidden',
  },
  walletTokenMarketRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: scale(8),
    marginTop: scale(3),
  },
  walletTokenPrice: {
    color: '#aeb8d8',
    fontSize: scale(11),
    fontWeight: '800',
  },
  walletTokenChange: {
    fontSize: scale(11),
    fontWeight: '900',
  },
  walletTokenChangePositive: {
    color: '#91f7bf',
  },
  walletTokenChangeNegative: {
    color: '#ff7b7b',
  },
  walletTokenChangeNeutral: {
    color: '#717da4',
  },
  walletTokenMeta: {
    color: '#5f6a91',
    fontSize: scale(10),
    marginTop: scale(3),
    fontWeight: '700',
  },
  walletEmptyCard: {
    borderWidth: 1,
    borderColor: '#273152',
    backgroundColor: 'rgba(27,35,66,0.45)',
    borderRadius: scale(18),
    padding: scale(16),
    marginTop: scale(12),
    gap: scale(10),
  },
  walletEmptyTitle: {
    color: '#f4f7ff',
    fontSize: scale(15),
    fontWeight: '800',
  },
  walletEmptyBody: {
    color: '#8f9bc1',
    fontSize: scale(12),
    lineHeight: scale(18),
  },
  lpTierList: {
    gap: scale(10),
  },
  lpTierRow: {
    flexDirection: 'row',
    alignItems: 'flex-start',
    gap: scale(10),
    backgroundColor: 'rgba(15,21,42,0.68)',
    borderWidth: 1,
    borderRadius: scale(16),
    padding: scale(12),
  },
  lpTierDot: {
    width: scale(9),
    height: scale(9),
    borderRadius: scale(5),
    marginTop: scale(4),
  },
  lpTierKicker: {
    color: '#9aa5ca',
    fontSize: scale(10),
    fontWeight: '900',
    letterSpacing: 0.4,
    textTransform: 'uppercase',
  },
  lpTierPairs: {
    color: '#f4f7ff',
    fontSize: scale(13),
    fontWeight: '800',
    marginTop: scale(3),
  },
  lpTierDetail: {
    color: '#8f9bc1',
    fontSize: scale(11),
    lineHeight: scale(16),
    marginTop: scale(3),
  },
  lpTierWeight: {
    fontSize: scale(11),
    fontWeight: '900',
    paddingHorizontal: scale(8),
    paddingVertical: scale(4),
    backgroundColor: 'rgba(7,10,21,0.62)',
    borderRadius: scale(999),
    overflow: 'hidden',
  },
  mmAccountCard: {
    backgroundColor: '#1b2342',
    borderRadius: scale(28),
    padding: scale(16),
    marginBottom: scale(20),
    borderWidth: 1,
    borderColor: '#303a5e',
    alignItems: 'center',
  },
  mmAccountHeader: {
    flexDirection: 'row',
    width: '100%',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: scale(18),
  },
  mmAddressPill: {
    backgroundColor: '#273152',
    paddingHorizontal: scale(12),
    paddingVertical: scale(6),
    borderRadius: 999,
    flexDirection: 'row',
    alignItems: 'center',
  },
  mmAddressText: {
    color: '#dce4ff',
    fontSize: scale(12),
    fontWeight: '700',
  },
  mmHeaderIcon: {
    width: scale(36),
    height: scale(36),
    borderRadius: 18,
    backgroundColor: '#273152',
    justifyContent: 'center',
    alignItems: 'center',
  },
  mmBalanceContainer: {
    alignItems: 'center',
    marginBottom: scale(24),
  },
  mmBalanceValue: {
    color: '#fff',
    fontSize: scale(32),
    fontWeight: '800',
  },
  mmBalanceFiat: {
    color: '#8f9bc1',
    fontSize: scale(16),
    marginTop: scale(4),
    fontWeight: '600',
  },
  mmActionRow: {
    flexDirection: 'row',
    justifyContent: 'space-around',
    width: '100%',
    paddingHorizontal: scale(10),
  },
  mmActionBtn: {
    alignItems: 'center',
    gap: scale(8),
  },
  mmActionIconWrap: {
    width: scale(48),
    height: scale(48),
    borderRadius: 24,
    backgroundColor: '#8a78ff',
    justifyContent: 'center',
    alignItems: 'center',
    shadowColor: '#8a78ff',
    shadowOffset: { width: 0, height: 4 },
    shadowOpacity: 0.3,
    shadowRadius: 8,
    elevation: 6,
  },
  mmActionIconText: {
    fontSize: scale(20),
    color: '#fff',
  },
  mmActionLabel: {
    color: '#8a78ff',
    fontSize: scale(12),
    fontWeight: '700',
  },
  mmSubTabRow: {
    flexDirection: 'row',
    borderBottomWidth: 1,
    borderBottomColor: '#242d4e',
    marginBottom: scale(10),
  },
  mmSubTab: {
    flex: 1,
    paddingVertical: scale(14),
    alignItems: 'center',
    borderBottomWidth: 2,
    borderBottomColor: 'transparent',
  },
  mmSubTabActive: {
    borderBottomColor: '#8a78ff',
  },
  mmSubTabText: {
    color: '#717da4',
    fontSize: scale(13),
    fontWeight: '800',
    letterSpacing: 1,
  },
  mmSubTabTextActive: {
    color: '#8a78ff',
  },
  mmListContainer: {
    flex: 1,
  },
  mmTokenRow: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingVertical: scale(14),
    borderBottomWidth: 1,
    borderBottomColor: '#1b2342',
  },
  mmTokenIcon: {
    width: scale(40),
    height: scale(40),
    borderRadius: 20,
    backgroundColor: '#273152',
    justifyContent: 'center',
    alignItems: 'center',
    marginRight: scale(12),
    overflow: 'hidden',
  },
  mmTokenLogo: {
    width: '100%',
    height: '100%',
    borderRadius: 20,
  },
  mmTokenTitleRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: scale(6),
    minWidth: 0,
  },
  mmTokenName: {
    color: '#f4f7ff',
    fontSize: scale(15),
    fontWeight: '700',
  },
  mmTokenSymbol: {
    color: '#717da4',
    fontSize: scale(13),
    marginTop: scale(2),
  },
  mmTokenFiat: {
    color: '#f4f7ff',
    fontSize: scale(14),
    fontWeight: '700',
  },
  nftGrid: {
    gap: scale(12),
    paddingVertical: scale(8),
  },
  nftCard: {
    backgroundColor: '#11182e',
    borderColor: '#273152',
    borderWidth: 1,
    borderRadius: scale(16),
    overflow: 'hidden',
  },
  nftImage: {
    width: '100%',
    height: scale(180),
    backgroundColor: '#0f152a',
  },
  nftImageFallback: {
    alignItems: 'center',
    justifyContent: 'center',
  },
  nftCardBody: {
    padding: scale(14),
    gap: scale(4),
  },
  nftOwnedBadge: {
    color: '#10b981',
    fontSize: scale(11),
    fontWeight: '700',
    marginTop: scale(4),
  },
  nftRemoveButton: {
    marginTop: scale(10),
    paddingVertical: scale(9),
    borderTopWidth: 1,
    borderTopColor: '#273152',
  },
  nftRemoveText: {
    color: '#f87171',
    fontSize: scale(12),
    fontWeight: '700',
    textAlign: 'center',
  },
  nftEmptyState: {
    alignItems: 'center',
    gap: scale(10),
    padding: scale(24),
  },
  nftEmptyIcon: {
    fontSize: scale(36),
  },
  nftUnsupportedNotice: {
    backgroundColor: 'rgba(245,158,11,0.08)',
    borderColor: 'rgba(245,158,11,0.25)',
    borderWidth: 1,
    borderRadius: scale(12),
    padding: scale(12),
    marginBottom: scale(12),
  },
  mmEmptyText: {
    color: '#717da4',
    textAlign: 'center',
    marginTop: scale(40),
    fontSize: scale(14),
  }
});

export default App;
