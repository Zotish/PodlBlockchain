import "react-native-get-random-values";
import React, { useEffect, useMemo, useRef, useState } from "react";
import {
  Alert,
  FlatList,
  Keyboard,
  Linking,
  KeyboardAvoidingView,
  Modal,
  Platform,
  Pressable,
  SafeAreaView,
  ScrollView,
  Share,
  StyleSheet,
  Text,
  TextInput,
  View,
} from "react-native";
import { StatusBar } from "expo-status-bar";
import * as Clipboard from "expo-clipboard";
import * as LocalAuthentication from "expo-local-authentication";
import * as FileSystem from "expo-file-system";
import { CameraView, useCameraPermissions } from "expo-camera";
import CryptoJS from "crypto-js";
import QRCode from "react-native-qrcode-svg";
import { WebView } from "react-native-webview";

import {
  getJson,
  nodeBaseFee,
  nodeBridgeRequests,
  nodeBridgeFamilies,
  nodeBridgeChainRemove,
  nodeBridgeChainUpsert,
  nodeBridgeChains,
  nodeBridgeTokenRemove,
  nodeBridgeTokenUpsert,
  nodeBridgeTokens,
  nodeCallContract,
  nodeCompilePlugin,
  nodeContractAbi,
  nodeContractStorage,
  nodeCurrentFactory,
  nodeDeployBuiltin,
  nodeDeployContract,
  nodeRecentTransactions,
  normalizeUrl,
  postJson,
  resolveTokenBalance,
  resolveTokenMeta,
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
import { STORAGE_KEYS, loadJSON, loadString, removeItem, saveJSON, saveString } from "./src/storage";
import {
  formatDate,
  formatUnits,
  isLikelyAddress,
  mergeUniqueByKey,
  parseUnits,
  shortAddress,
  txTouchesAddress,
} from "./src/utils";

const PROD_CHAIN_URL = "https://dazzling-peace-production-3529.up.railway.app";
const PROD_WALLET_URL = "https://enchanting-hope-production-1c63.up.railway.app";
const PROD_AGGREGATOR_URL = "https://keen-enjoyment-production-0440.up.railway.app";
const PROD_EXPLORER_URL = "https://warm-dragon-34d6ff.netlify.app";
const DEFAULT_BROWSER_URL = PROD_EXPLORER_URL;

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

const BUILTIN_TEMPLATES = [
  { value: "dex_factory", label: "DEX Factory" },
  { value: "dex_swap", label: "DEX Swap / Router" },
  { value: "lqd20", label: "LQD20 Token" },
  { value: "dao_treasury", label: "DAO Treasury" },
  { value: "nft_collection", label: "NFT Collection" },
  { value: "bridge_token", label: "Bridge Token" },
  { value: "lending_pool", label: "Lending Pool" },
];

const TABS = [
  { id: "home", label: "Home", icon: "⌂" },
  { id: "tokens", label: "Tokens", icon: "◫" },
  { id: "browser", label: "Browser", icon: "◌" },
  { id: "activity", label: "Activity", icon: "▤" },
  { id: "settings", label: "Settings", icon: "⚙" },
];

const ADVANCED_TABS = [
  { id: "contracts", label: "Contracts" },
  { id: "bridge", label: "Bridge" },
  { id: "approvals", label: "Approvals" },
  { id: "networks", label: "Networks" },
];

const EMPTY_VAULT = {
  address: "",
  privateKey: "",
  mnemonic: "",
};

const DEFAULT_CUSTOM_SOURCE = `package main

import (
  "fmt"
  lqdc "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/lqd-sdk-compat/context"
)

type ContractTemplate struct{}

var Contract = &ContractTemplate{}

func (c *ContractTemplate) Init(ctx *lqdc.Context, args []string) error {
  ctx.Set("hello", "world")
  return nil
}

func (c *ContractTemplate) Ping(ctx *lqdc.Context, args []string) (string, error) {
  return fmt.Sprintf("pong:%s", ctx.CallerAddr), nil
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
  return (
    <View style={styles.fieldWrap}>
      <View style={styles.fieldLabelRow}>
        <Text style={styles.fieldLabel}>{label}</Text>
        {right}
      </View>
      <TextInput
        value={value}
        onChangeText={onChangeText}
        placeholder={placeholder}
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

function TokenRow({ item, onSend, onRefresh, onRemove }) {
  return (
    <View style={styles.rowCard}>
      <View style={{ flex: 1 }}>
        <Text style={styles.rowTitle}>{item.symbol} · {shortAddress(item.address)}</Text>
        <Text style={styles.rowSub}>{item.name}</Text>
        <Text style={styles.tokenBalance}>{formatUnits(item.balance || "0", item.decimals || 8)} {item.symbol}</Text>
      </View>
      <View style={styles.rowActions}>
        <Button label="Send" onPress={onSend} compact />
        <Button label="Refresh" onPress={onRefresh} compact secondary />
        <Button label="Remove" onPress={onRemove} compact danger />
      </View>
    </View>
  );
}

function ActivityRow({ item }) {
  const hash = item.TxHash || item.tx_hash || item.hash || "";
  const type = item.Type || item.type || "tx";
  const status = item.Status || item.status || "";
  return (
    <View style={styles.rowCard}>
      <View style={{ flex: 1 }}>
        <Text style={styles.rowTitle}>{type.toUpperCase()} · {status || "unknown"}</Text>
        <Text style={styles.rowSub}>{shortAddress(hash, 8, 6)}</Text>
        <Text style={styles.rowSub}>
          {shortAddress(item.From || item.from)} → {shortAddress(item.To || item.to)}
        </Text>
        <Text style={styles.rowSub}>Time: {formatDate(item.Timestamp || item.timestamp)}</Text>
      </View>
      <View style={styles.rowActions}>
        <Button
          label="Copy"
          onPress={() => Clipboard.setStringAsync(hash)}
          compact
          secondary
        />
      </View>
    </View>
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
};

const initialTokenSendForm = {
  to: "",
  amount: "",
};

const initialDeployForm = {
  template: "dex_factory",
  gas: "500000",
  gasPrice: "",
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
  const [tab, setTab] = useState("home");

  const [vaultRecord, setVaultRecord] = useState(null);
  const [wallet, setWallet] = useState(null);
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
  const [selectedTokenForSend, setSelectedTokenForSend] = useState(null);
  const [tokenSendForm, setTokenSendForm] = useState(initialTokenSendForm);

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
  const [backupText, setBackupText] = useState("");
  const [settingsAutoRefresh, setSettingsAutoRefresh] = useState(true);
  const [walletVisible, setWalletVisible] = useState(false);
  const [scannerVisible, setScannerVisible] = useState(false);
  const [scannerTarget, setScannerTarget] = useState("");
  const [receiveVisible, setReceiveVisible] = useState(false);
  const [deepLinkHint, setDeepLinkHint] = useState("");
  const [cameraPermission, requestCameraPermission] = useCameraPermissions();
  const [biometricEnabled, setBiometricEnabled] = useState(true);
  const [biometricAvailable, setBiometricAvailable] = useState(false);
  const [browserInput, setBrowserInput] = useState(DEFAULT_BROWSER_URL);
  const [browserUrl, setBrowserUrl] = useState(DEFAULT_BROWSER_URL);
  const [browserLoading, setBrowserLoading] = useState(false);
  const [browserCanGoBack, setBrowserCanGoBack] = useState(false);
  const [browserCanGoForward, setBrowserCanGoForward] = useState(false);

  const nodeUrl = useMemo(() => {
    const current = networks.find((item) => item.id === activeNetworkId) || networks[0] || DEFAULT_NETWORKS[0];
    return normalizeUrl(current?.nodeUrl || endpoints.nodeUrl);
  }, [networks, activeNetworkId, endpoints.nodeUrl]);

  const walletUrl = useMemo(() => {
    const current = networks.find((item) => item.id === activeNetworkId) || networks[0] || DEFAULT_NETWORKS[0];
    return normalizeUrl(current?.walletUrl || endpoints.walletUrl);
  }, [networks, activeNetworkId, endpoints.walletUrl]);

  const aggregatorUrl = useMemo(() => normalizeUrl(endpoints.aggregatorUrl || DEFAULT_ENDPOINTS.aggregatorUrl), [endpoints.aggregatorUrl]);
  const explorerUrl = useMemo(() => normalizeUrl(endpoints.explorerUrl || DEFAULT_ENDPOINTS.explorerUrl), [endpoints.explorerUrl]);

  const currentNetwork = useMemo(
    () => networks.find((item) => item.id === activeNetworkId) || networks[0] || DEFAULT_NETWORKS[0],
    [networks, activeNetworkId]
  );

  const currentBridgeChain = useMemo(() => {
    const normalized = String(bridgeChainId || "").toLowerCase();
    return bridgeChains.find((item) => String(item.id || "").toLowerCase() === normalized)
      || bridgeChains.find((item) => String(item.chain_id || "").toLowerCase() === normalized)
      || null;
  }, [bridgeChains, bridgeChainId]);
  const currentBridgeFamily = String(currentBridgeChain?.family || "evm").toLowerCase();
  const isExternalBridgeFamily = currentBridgeFamily === "cosmos" || currentBridgeFamily === "utxo" || currentBridgeFamily === "cardano" || currentBridgeFamily === "solana" || currentBridgeFamily === "substrate" || currentBridgeFamily === "xrpl" || currentBridgeFamily === "ton" || currentBridgeFamily === "near" || currentBridgeFamily === "aptos";

  const unlockInProgress = useRef(false);
  const scanHandlerRef = useRef(() => {});
  const browserRef = useRef(null);

  useEffect(() => {
    let alive = true;
    (async () => {
      try {
        const [vault, savedNetworks, savedNetworkId, savedEndpoints, savedWatchlist, savedActivity, savedFactory, savedBridgeChainId, savedSettings, savedApprovals, savedTrustedOrigins] = await Promise.all([
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
        ]);

        if (!alive) return;
        if (savedNetworks?.length) setNetworks(savedNetworks);
        if (savedNetworkId) setActiveNetworkId(savedNetworkId);
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
        }
        if (Array.isArray(savedApprovals)) setPendingApprovals(savedApprovals);
        if (Array.isArray(savedTrustedOrigins)) setTrustedOrigins(savedTrustedOrigins);
        setVaultRecord(vault || null);
      } catch (e) {
        setStatus(e.message || "Failed to load wallet state");
      } finally {
        if (alive) setBooting(false);
      }
    })();
    return () => { alive = false; };
  }, []);

  useEffect(() => {
    saveJSON(STORAGE_KEYS.networks, networks).catch(() => {});
  }, [networks]);

  useEffect(() => {
    saveJSON(STORAGE_KEYS.activeNetworkId, activeNetworkId).catch(() => {});
  }, [activeNetworkId]);

  useEffect(() => {
    saveJSON(STORAGE_KEYS.endpoints, endpoints).catch(() => {});
  }, [endpoints]);

  useEffect(() => {
    saveJSON(STORAGE_KEYS.watchlist, watchlist).catch(() => {});
  }, [watchlist]);

  useEffect(() => {
    saveJSON(STORAGE_KEYS.activity, activity).catch(() => {});
  }, [activity]);

  useEffect(() => {
    saveJSON(STORAGE_KEYS.factory, factoryAddress).catch(() => {});
  }, [factoryAddress]);

  useEffect(() => {
    saveJSON(STORAGE_KEYS.bridgeChainId, bridgeChainId).catch(() => {});
  }, [bridgeChainId]);

  useEffect(() => {
    saveJSON(STORAGE_KEYS.settings, { autoRefresh: settingsAutoRefresh, biometricEnabled }).catch(() => {});
  }, [settingsAutoRefresh, biometricEnabled]);

  useEffect(() => {
    saveJSON(STORAGE_KEYS.approvals, pendingApprovals).catch(() => {});
  }, [pendingApprovals]);

  useEffect(() => {
    saveJSON(STORAGE_KEYS.trustedOrigins, trustedOrigins).catch(() => {});
  }, [trustedOrigins]);

  useEffect(() => {
    if (!wallet?.address) return;
    if (biometricEnabled) {
      saveString(STORAGE_KEYS.biometricVault, JSON.stringify(wallet), { requireAuthentication: true }).catch(() => {});
    } else {
      removeItem(STORAGE_KEYS.biometricVault).catch(() => {});
    }
  }, [biometricEnabled, wallet?.address]);

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
    scanHandlerRef.current = openFromScan;
  }, [scannerTarget, currentNetwork.chainId, wallet?.address]);

  useEffect(() => {
    const handleUrl = ({ url }) => {
      if (url && typeof scanHandlerRef.current === "function") {
        scanHandlerRef.current(url);
      }
    };
    const sub = Linking.addEventListener("url", handleUrl);
    Linking.getInitialURL().then((url) => {
      if (url) handleUrl({ url });
    }).catch(() => {});
    return () => {
      sub?.remove?.();
    };
  }, []);

  useEffect(() => {
    if (!wallet?.address || !settingsAutoRefresh) return undefined;
    let cancelled = false;
    const tick = () => {
      refreshWalletSnapshot().catch((e) => {
        if (!cancelled) setStatus(e.message || "Refresh failed");
      });
    };
    tick();
    const timer = setInterval(tick, 25000);
    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  }, [wallet?.address, settingsAutoRefresh, nodeUrl, walletUrl, activeNetworkId]);

  async function persistWalletVault(vault, password) {
    const cipher = encryptVault(vault, password);
    const record = { address: vault.address, cipher, createdAt: Date.now() };
    await saveJSON(STORAGE_KEYS.vault, record);
    if (biometricEnabled) {
      try {
        await saveString(STORAGE_KEYS.biometricVault, JSON.stringify(vault), { requireAuthentication: true });
      } catch {
        // Do not block wallet creation if biometric-secured storage is unavailable on a device.
      }
    } else {
      await removeItem(STORAGE_KEYS.biometricVault);
    }
    setVaultRecord(record);
  }

  async function refreshWalletSnapshot() {
    if (!wallet?.address) return;
    const [native, factory, recent, requests, tokensResp] = await Promise.all([
      walletBalance(nodeUrl, wallet.address).catch(() => null),
      nodeCurrentFactory(nodeUrl).catch(() => null),
      nodeRecentTransactions(nodeUrl).catch(() => []),
      nodeBridgeRequests(nodeUrl).catch(() => []),
      nodeBridgeTokens(nodeUrl).catch(() => []),
    ]);

    const balanceValue = native?.balance || native?.Balance || native?.amount || "0";
    setNativeBalance(String(balanceValue));
    if (factory?.address) {
      setFactoryAddress(factory.address);
    }
    if (Array.isArray(recent)) {
      setRecentTxs(recent);
      const local = [...activity];
      const merged = mergeUniqueByKey(local, recent, "TxHash");
      setActivity(merged.slice(0, 100));
    }
    if (Array.isArray(requests)) {
      setBridgeRequests(requests);
    }
    if (Array.isArray(tokensResp)) {
      setBridgeTokens(tokensResp);
    }

    await refreshTokenBalances();
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
      const resp = await nodeBridgeFamilies(nodeUrl);
      const list = Array.isArray(resp) ? resp.filter(Boolean) : [];
      setBridgeFamilies(list);
      return list;
    } catch {
      setBridgeFamilies([]);
      return [];
    }
  }

  function applyBridgeChainSelection(cfg) {
    const chainId = String(cfg?.id || cfg?.chain_id || "").trim();
    if (!chainId) return;
    const nextFamily = String(cfg?.family || "evm").toLowerCase();
    setBridgeChainId(chainId);
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
  }

  async function refreshTokenBalances(nextWatchlist = watchlist) {
    if (!wallet?.address) return;
    const updated = [];
    for (const token of nextWatchlist) {
      try {
        const contract = token.address || token.contract;
        const [meta, balance] = await Promise.all([
          token.name && token.symbol ? Promise.resolve(token) : resolveTokenMeta(nodeUrl, contract, wallet.address),
          resolveTokenBalance(nodeUrl, walletUrl, contract, wallet.address),
        ]);
        updated.push({
          address: contract,
          name: meta.name || token.name || "Token",
          symbol: meta.symbol || token.symbol || "TOKEN",
          decimals: Number(meta.decimals || token.decimals || 8),
          balance: String(balance || "0"),
        });
      } catch {
        updated.push(token);
      }
    }
    setWatchlist(updated);
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
    if (/^lqdwallet:\/\//i.test(raw)) {
      try {
        const url = new URL(raw);
        const action = (url.hostname || url.pathname || "").replace(/^\//, "").toLowerCase();
        const params = Object.fromEntries(url.searchParams.entries());
        if (action === "send") {
          if (params.to) setSendForm((prev) => ({ ...prev, to: params.to, amount: params.amount || prev.amount }));
          setTab("home");
          setStatus("Send form populated from QR / deep link");
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
      } catch {
        // fallback to address handling below
      }
    }
    if (isLikelyAddress(raw)) {
      if (scannerTarget === "native") {
        setSendForm((prev) => ({ ...prev, to: raw }));
      } else if (scannerTarget === "token") {
        setTokenSendForm((prev) => ({ ...prev, to: raw }));
      } else if (scannerTarget === "bridge") {
        setBridgeForm((prev) => ({ ...prev, toBsc: raw, toLqd: raw }));
      } else if (scannerTarget === "import") {
        setTokenImportForm({ address: raw });
        setTab("tokens");
      } else {
        setSendForm((prev) => ({ ...prev, to: raw }));
      }
      setStatus("Address scanned");
      return;
    }
    setStatus("QR scanned but format was not recognized");
  }

  async function approveRequest(item) {
    setPendingApprovals((prev) => prev.map((x) => (x.id === item.id ? { ...x, status: "approved", approvedAt: Date.now() } : x)));
    setTrustedOrigins((prev) => (prev.includes(item.origin) ? prev : [...prev, item.origin]));
    setStatus(`Approved ${item.name}`);
    if (item.callback) {
      const callback = String(item.callback).trim();
      if (callback) {
        const url = callback.includes("?") ? `${callback}&approved=1&address=${encodeURIComponent(wallet.address)}&chainId=${encodeURIComponent(currentNetwork.chainId)}` : `${callback}?approved=1&address=${encodeURIComponent(wallet.address)}&chainId=${encodeURIComponent(currentNetwork.chainId)}`;
        try {
          await Linking.openURL(url);
        } catch {
          // keep approval in wallet even if callback cannot open
        }
      }
    }
  }

  function openBrowserTarget(value) {
    const next = coerceBrowserUrl(value);
    setBrowserInput(next);
    setBrowserUrl(next);
    setTab("browser");
  }

  function rejectRequest(item) {
    setPendingApprovals((prev) => prev.map((x) => (x.id === item.id ? { ...x, status: "rejected", rejectedAt: Date.now() } : x)));
    setStatus(`Rejected ${item.name}`);
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
    try {
      const vault = decryptVault(vaultRecord.cipher, unlockPassword);
      setWallet(vault);
      setWalletVisible(true);
      setStatus(`Unlocked ${shortAddress(vault.address)}`);
      await refreshWalletSnapshot();
    } catch (e) {
      setStatus(e.message || "Failed to unlock");
    } finally {
      unlockInProgress.current = false;
      setBusy(false);
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
    const auth = await LocalAuthentication.authenticateAsync({
      promptMessage: "Unlock LQD Mobile Wallet",
      fallbackLabel: "Use password",
      cancelLabel: "Cancel",
    });
    if (!auth.success) {
      setStatus("Biometric unlock cancelled");
      return;
    }
    try {
      const biometricRaw = await loadString(STORAGE_KEYS.biometricVault, "", { requireAuthentication: true });
      const vault = biometricRaw ? JSON.parse(biometricRaw) : decryptVault(vaultRecord.cipher, unlockPassword || "");
      setWallet(vault);
      setWalletVisible(true);
      setStatus(`Unlocked ${shortAddress(vault.address)} with biometrics`);
      await refreshWalletSnapshot();
    } catch (e) {
      setStatus(e.message || "Biometric unlock failed");
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
      await refreshWalletSnapshot();
    } catch (e) {
      const message = e.message || "Failed to create wallet";
      setStatus(message);
      Alert.alert("Create wallet failed", message);
    } finally {
      setBusy(false);
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
      await refreshWalletSnapshot();
    } catch (e) {
      const message = e.message || "Failed to import mnemonic";
      setStatus(message);
      Alert.alert("Import failed", message);
    } finally {
      setBusy(false);
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
      await refreshWalletSnapshot();
    } catch (e) {
      const message = e.message || "Failed to import private key";
      setStatus(message);
      Alert.alert("Import failed", message);
    } finally {
      setBusy(false);
    }
  }

  async function lockWalletAction() {
    setWallet(null);
    setWalletVisible(false);
    setUnlockPassword("");
    setStatus("Wallet locked");
  }

  async function copyAddress() {
    if (!wallet?.address) return;
    await Clipboard.setStringAsync(wallet.address);
    setStatus("Address copied");
  }

  async function pasteClipboardTo(setter) {
    const value = await Clipboard.getStringAsync();
    setter(value || "");
  }

  async function refreshNativeOnly() {
    if (!wallet?.address) return;
    const native = await walletBalance(nodeUrl, wallet.address);
    setNativeBalance(String(native?.balance || native?.Balance || native?.amount || "0"));
  }

  async function sendNativeAction() {
    if (!wallet?.address || !wallet?.privateKey) {
      setStatus("Unlock wallet first");
      return;
    }
    if (!isLikelyAddress(sendForm.to)) {
      setStatus("Enter a valid recipient address");
      return;
    }
    const amount = parseUnits(sendForm.amount, 8);
    if (BigInt(amount) <= 0n) {
      setStatus("Enter a valid amount");
      return;
    }

    setBusy(true);
    try {
      const baseFee = await nodeBaseFee(nodeUrl).catch(() => 10);
      const res = await walletSend(walletUrl, {
        from: wallet.address,
        to: sendForm.to.trim(),
        value: amount,
        data: "",
        gas: 21000,
        gas_price: baseFee || 10,
        private_key: wallet.privateKey,
      });
      const hash = res?.tx_hash || res?.TxHash || res?.hash || "";
      setStatus(hash ? `Native sent: ${shortAddress(hash, 8, 6)}` : "Native sent");
      rememberActivity({
        type: "send",
        From: wallet.address,
        To: sendForm.to.trim(),
        TxHash: hash,
        Timestamp: Math.floor(Date.now() / 1000),
        Status: "success",
      });
      setSendForm(initialSendForm);
      await refreshNativeOnly();
      await refreshWalletSnapshot();
    } catch (e) {
      setStatus(e.message || "Send failed");
    } finally {
      setBusy(false);
    }
  }

  async function addTokenAction() {
    if (!wallet?.address) {
      setStatus("Unlock wallet first");
      return;
    }
    const address = tokenImportForm.address.trim();
    if (!isLikelyAddress(address)) {
      setStatus("Enter a valid token address");
      return;
    }
    setBusy(true);
    try {
      const meta = await resolveTokenMeta(nodeUrl, address, wallet.address);
      const balance = await resolveTokenBalance(nodeUrl, walletUrl, address, wallet.address);
      const next = mergeUniqueByKey(watchlist, [{
        address,
        name: meta.name,
        symbol: meta.symbol,
        decimals: meta.decimals,
        balance,
      }], "address");
      setWatchlist(next);
      setTokenImportForm(initialTokenImportForm);
      setStatus(`Imported token ${meta.symbol}`);
    } catch (e) {
      setStatus(e.message || "Token import failed");
    } finally {
      setBusy(false);
    }
  }

  async function refreshSingleToken(address) {
    if (!wallet?.address) return;
    try {
      const meta = await resolveTokenMeta(nodeUrl, address, wallet.address);
      const balance = await resolveTokenBalance(nodeUrl, walletUrl, address, wallet.address);
      setWatchlist((prev) => mergeUniqueByKey(prev, [{ ...meta, balance }], "address"));
    } catch (e) {
      setStatus(e.message || "Token refresh failed");
    }
  }

  async function removeToken(address) {
    setWatchlist((prev) => prev.filter((item) => String(item.address).toLowerCase() !== String(address).toLowerCase()));
  }

  async function sendTokenAction(token) {
    if (!wallet?.address || !wallet?.privateKey) {
      setStatus("Unlock wallet first");
      return;
    }
    if (!isLikelyAddress(tokenSendForm.to)) {
      setStatus("Enter a valid destination address");
      return;
    }
    const amount = parseUnits(tokenSendForm.amount, token.decimals || 8);
    if (BigInt(amount) <= 0n) {
      setStatus("Enter a valid token amount");
      return;
    }
    setBusy(true);
    try {
      const baseFee = await nodeBaseFee(nodeUrl).catch(() => 10);
      const res = await walletContractTx(walletUrl, {
        address: wallet.address,
        contract_address: token.address,
        function: "Transfer",
        args: [tokenSendForm.to.trim(), amount],
        value: "0",
        gas: 50000,
        gas_price: baseFee || 10,
        private_key: wallet.privateKey,
      });
      const hash = res?.tx_hash || res?.TxHash || res?.hash || "";
      setStatus(hash ? `Token sent: ${shortAddress(hash, 8, 6)}` : "Token sent");
      rememberActivity({
        type: "token",
        From: wallet.address,
        To: tokenSendForm.to.trim(),
        Contract: token.address,
        TxHash: hash,
        Timestamp: Math.floor(Date.now() / 1000),
        Status: "success",
      });
      setTokenSendForm(initialTokenSendForm);
      await refreshSingleToken(token.address);
      await refreshNativeOnly();
    } catch (e) {
      setStatus(e.message || "Token send failed");
    } finally {
      setBusy(false);
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
    try {
      await nodeBridgeChainUpsert(nodeUrl, payload, apiKey);
      await loadBridgeChains();
      setStatus(`Bridge chain saved: ${payload.name}`);
    } catch (e) {
      setStatus(e.message || "Bridge chain save failed");
    } finally {
      setBusy(false);
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
    try {
      await nodeBridgeChainRemove(nodeUrl, { id: chainId }, apiKey);
      await loadBridgeChains();
      setStatus(`Bridge chain removed: ${chainId}`);
    } catch (e) {
      setStatus(e.message || "Bridge chain remove failed");
    } finally {
      setBusy(false);
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
    setBusy(true);
    try {
      const res = await nodeDeployBuiltin(nodeUrl, {
        template: deployForm.template,
        owner: wallet.address,
        private_key: wallet.privateKey,
        gas: Number(deployForm.gas || 500000),
      });
      if (deployForm.template === "dex_factory" && res?.address) {
        setFactoryAddress(res.address);
      }
      rememberActivity({
        type: "deploy",
        From: wallet.address,
        To: res?.address,
        TxHash: res?.tx_hash,
        Timestamp: Math.floor(Date.now() / 1000),
        Status: "success",
      });
      setStatus(`Deployed ${deployForm.template}: ${shortAddress(res?.address || "")}`);
    } catch (e) {
      setStatus(e.message || "Builtin deploy failed");
    } finally {
      setBusy(false);
    }
  }

  async function compileCustomPluginAction() {
    setBusy(true);
    try {
      const res = await nodeCompilePlugin(nodeUrl, customSource);
      if (!res?.success) {
        throw new Error(res?.error || "Plugin compilation failed");
      }
      const uri = `${FileSystem.cacheDirectory || ""}lqd-mobile-plugin.so`;
      await FileSystem.writeAsStringAsync(uri, res.binary, {
        encoding: FileSystem.EncodingType.Base64,
      });
      setCompiledPlugin(res);
      setCompiledPluginUri(uri);
      setCompiledPluginSize(Number(res.size || 0));
      setStatus(`Plugin compiled (${res.size || 0} bytes)`);
    } catch (e) {
      setStatus(e.message || "Compile failed");
    } finally {
      setBusy(false);
    }
  }

  async function deployCustomPluginAction() {
    if (!wallet?.address || !wallet?.privateKey) {
      setStatus("Unlock wallet first");
      return;
    }
    if (!compiledPluginUri) {
      setStatus("Compile a plugin first");
      return;
    }
    setBusy(true);
    try {
      const formData = new FormData();
      formData.append("type", "plugin");
      formData.append("owner", wallet.address);
      formData.append("private_key", wallet.privateKey);
      formData.append("gas", "500000");
      formData.append("gas_price", "0");
      formData.append("contract_file", {
        uri: compiledPluginUri,
        name: "contract.so",
        type: "application/octet-stream",
      });
      const res = await nodeDeployContract(nodeUrl, formData);
      rememberActivity({
        type: "deploy",
        From: wallet.address,
        To: res?.address,
        TxHash: res?.tx_hash,
        Timestamp: Math.floor(Date.now() / 1000),
        Status: "success",
      });
      setStatus(`Custom plugin deployed: ${shortAddress(res?.address || "")}`);
    } catch (e) {
      setStatus(e.message || "Deploy failed");
    } finally {
      setBusy(false);
    }
  }

  async function inspectContractAction() {
    if (!isLikelyAddress(inspectForm.address)) {
      setStatus("Enter a valid contract address");
      return;
    }
    setBusy(true);
    try {
      const [abi, storage] = await Promise.all([
        nodeContractAbi(nodeUrl, inspectForm.address),
        nodeContractStorage(nodeUrl, inspectForm.address),
      ]);
      setInspectData({ abi, storage });
      setStatus(`Loaded contract ${shortAddress(inspectForm.address)}`);
    } catch (e) {
      setStatus(e.message || "Contract inspect failed");
    } finally {
      setBusy(false);
    }
  }

  async function callContractAction() {
    if (!wallet?.address || !wallet?.privateKey) {
      setStatus("Unlock wallet first");
      return;
    }
    if (!isLikelyAddress(callForm.contract)) {
      setStatus("Enter a valid contract address");
      return;
    }
    const args = callForm.args
      .split(",")
      .map((s) => s.trim())
      .filter(Boolean);
    setBusy(true);
    try {
      const res = await walletContractTx(walletUrl, {
        address: wallet.address,
        contract_address: callForm.contract.trim(),
        function: callForm.functionName.trim(),
        args,
        value: callForm.value || "0",
        gas: Number(callForm.gas || 200000),
        gas_price: Number(callForm.gasPrice || 0),
        private_key: wallet.privateKey,
      });
      const hash = res?.tx_hash || res?.TxHash || res?.hash || "";
      setStatus(hash ? `Contract call submitted: ${shortAddress(hash, 8, 6)}` : "Contract call submitted");
      rememberActivity({
        type: "contract",
        From: wallet.address,
        To: callForm.contract.trim(),
        TxHash: hash,
        Timestamp: Math.floor(Date.now() / 1000),
        Status: "success",
      });
    } catch (e) {
      setStatus(e.message || "Contract call failed");
    } finally {
      setBusy(false);
    }
  }

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
        refreshWalletSnapshot().catch(() => {});
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
    refreshWalletSnapshot().catch(() => {});
  }, [wallet?.address, activeNetworkId]);

  useEffect(() => {
    if (!wallet?.address) return;
    loadBridgeChains().catch(() => {});
    loadBridgeFamilies().catch(() => {});
  }, [wallet?.address, nodeUrl]);

  const currentTokens = watchlist || [];

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
            <Button label={busy ? "Creating…" : "Create Wallet"} onPress={createWalletAction} disabled={busy} />
            <Text style={styles.helperText}>The wallet server returns the address, private key and mnemonic. The mobile vault encrypts them locally with your password.</Text>
          </Card>

          <Card title="Import from mnemonic" subtitle="Restore an existing wallet phrase.">
            <Field label="Mnemonic" value={importMnemonicForm.mnemonic} onChangeText={(v) => setImportMnemonicForm((p) => ({ ...p, mnemonic: v }))} multiline numberOfLines={4} placeholder="twelve or twenty-four words…" />
            <Field label="Password" value={importMnemonicForm.password} onChangeText={(v) => setImportMnemonicForm((p) => ({ ...p, password: v }))} secureTextEntry placeholder="Password for local vault" />
            <Button label={busy ? "Importing…" : "Import Mnemonic"} onPress={importMnemonicAction} disabled={busy} />
          </Card>

          <Card title="Import private key" subtitle="Paste a raw private key if you already have one.">
            <Field label="Private key" value={importPkForm.privateKey} onChangeText={(v) => setImportPkForm((p) => ({ ...p, privateKey: v }))} placeholder="0x..." />
            <Field label="Password" value={importPkForm.password} onChangeText={(v) => setImportPkForm((p) => ({ ...p, password: v }))} secureTextEntry placeholder="Password for local vault" />
            <Button label={busy ? "Importing…" : "Import Private Key"} onPress={importPrivateKeyAction} disabled={busy} />
          </Card>

          <Text style={styles.statusText}>{status}</Text>
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
              <Button label={busy ? "Unlocking…" : "Unlock Wallet"} onPress={unlockWallet} disabled={busy} />
              <Button label="Biometric Unlock" onPress={unlockWithBiometrics} secondary disabled={!biometricAvailable || !biometricEnabled} />
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
      <KeyboardAvoidingView behavior={Platform.OS === "ios" ? "padding" : undefined} style={{ flex: 1 }}>
        <Modal visible={receiveVisible} transparent animationType="fade" onRequestClose={() => setReceiveVisible(false)}>
          <View style={styles.modalBackdrop}>
            <View style={styles.modalCard}>
              <Text style={styles.cardTitle}>Receive LQD</Text>
              <Text style={styles.cardSubtitle}>Share this address or display the QR code for a sender.</Text>
              <View style={styles.qrWrap}>
                <QRCode value={wallet.address} size={210} color="#f4f7ff" backgroundColor="#151b31" />
              </View>
              <Text style={styles.inspectBox}>{wallet.address}</Text>
              <View style={styles.inlineButtons}>
                <Button
                  label="Copy Address"
                  onPress={async () => {
                    await Clipboard.setStringAsync(wallet.address);
                    setStatus("Address copied");
                  }}
                  compact
                />
                <Button
                  label="Share"
                  onPress={async () => {
                    await Share.share({ message: wallet.address });
                  }}
                  compact
                  secondary
                />
                <Button label="Close" onPress={() => setReceiveVisible(false)} compact danger />
              </View>
            </View>
          </View>
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
                    setTimeout(() => openFromScan(data), 250);
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

        <View style={styles.topBar}>
          <View style={styles.topActions}>
            <Pressable style={styles.walletPill} onPress={() => setReceiveVisible(true)}>
              <Text style={styles.walletPillText}>Receive</Text>
            </Pressable>
            <Pressable style={styles.walletPill} onPress={() => scanWithCamera("native")}>
              <Text style={styles.walletPillText}>Scan</Text>
            </Pressable>
            <View style={[styles.walletPill, styles.walletPillState]}>
              <Text style={styles.walletPillText}>{walletVisible ? "Unlocked" : "Locked"}</Text>
            </View>
          </View>
          <View style={styles.topIdentity}>
            <Text style={styles.topAddress}>{shortAddress(wallet.address)}</Text>
            <Text style={styles.topNetwork}>{currentNetwork.name}</Text>
          </View>
        </View>

        <ScrollView style={{ flex: 1 }} contentContainerStyle={styles.mainScroll}>
          <View style={styles.summaryGrid}>
            <Stat label="Native Balance" value={formatUnits(nativeBalance, 8, 6)} subvalue="LQD" />
            <Stat label="Network" value={currentNetwork.symbol || "LQD"} subvalue={currentNetwork.name} />
            <Stat label="Token Count" value={String(currentTokens.length)} subvalue="Watchlist" />
          </View>

          {tab === "home" && (
            <View style={styles.sectionGap}>
              <Card title="Quick actions" subtitle="Core wallet actions only. DEX sites can be opened from the Browser tab.">
                <View style={styles.actionGrid}>
                  <Button label="Send" onPress={() => setTab("home")} />
                  <Button label="Receive" onPress={() => setReceiveVisible(true)} secondary />
                  <Button label="Open Browser" onPress={() => setTab("browser")} secondary />
                  <Button label="Activity" onPress={() => setTab("activity")} secondary />
                </View>
              </Card>

              <Card title="Send LQD" subtitle="Native coin transfer from your wallet.">
                <Field label="To" value={sendForm.to} onChangeText={(v) => setSendForm((p) => ({ ...p, to: v }))} placeholder="0x..." />
                <Field label="Amount" value={sendForm.amount} onChangeText={(v) => setSendForm((p) => ({ ...p, amount: v }))} keyboardType="decimal-pad" placeholder="0.0" />
                <View style={styles.inlineButtons}>
                  <Button label="Scan Recipient" onPress={() => scanWithCamera("native")} compact secondary />
                  <Button label="Paste" onPress={() => pasteClipboardTo((value) => setSendForm((p) => ({ ...p, to: value } )))} compact />
                </View>
                <Button label={busy ? "Sending…" : "Send LQD"} onPress={sendNativeAction} disabled={busy} />
              </Card>

              <Card title="Receive" subtitle="Your current address.">
                <Text style={styles.largeCode}>{wallet.address}</Text>
                <View style={styles.inlineButtons}>
                  <Button label="Copy Address" onPress={copyAddress} compact />
                  <Button label="Show QR" onPress={() => setReceiveVisible(true)} compact secondary />
                  <Button label="Refresh" onPress={refreshNativeOnly} compact secondary />
                  <Button label="Lock" onPress={lockWalletAction} compact danger />
                </View>
              </Card>
            </View>
          )}

          {tab === "tokens" && (
            <View style={styles.sectionGap}>
              <Card title="Import token" subtitle="Add a token by contract address.">
                <Field label="Token address" value={tokenImportForm.address} onChangeText={(v) => setTokenImportForm({ address: v })} placeholder="0x..." />
                <View style={styles.inlineButtons}>
                  <Button label="Scan Token" onPress={() => scanWithCamera("import")} compact secondary />
                  <Button label="Paste" onPress={() => pasteClipboardTo((value) => setTokenImportForm({ address: value }))} compact />
                </View>
                <Button label={busy ? "Importing…" : "Import Token"} onPress={addTokenAction} disabled={busy} />
              </Card>

              <Card title="Watchlist" subtitle="Balances auto-refresh on unlock and after actions.">
                {!currentTokens.length ? (
                  <Text style={styles.helperText}>No tokens imported yet.</Text>
                ) : (
                  currentTokens.map((token) => (
                    <TokenRow
                      key={token.address}
                      item={token}
                      onSend={() => {
                        setSelectedTokenForSend(token);
                        setTokenSendForm(initialTokenSendForm);
                        setTab("tokens");
                      }}
                      onRefresh={() => refreshSingleToken(token.address)}
                      onRemove={() => removeToken(token.address)}
                    />
                  ))
                )}
              </Card>

              {selectedTokenForSend ? (
                <Card title={`Send ${selectedTokenForSend.symbol}`} subtitle={selectedTokenForSend.address}>
                  <Field label="To" value={tokenSendForm.to} onChangeText={(v) => setTokenSendForm((p) => ({ ...p, to: v }))} placeholder="0x..." />
                  <Field label="Amount" value={tokenSendForm.amount} onChangeText={(v) => setTokenSendForm((p) => ({ ...p, amount: v }))} keyboardType="decimal-pad" placeholder="0.0" />
                  <View style={styles.inlineButtons}>
                    <Button label="Scan Recipient" onPress={() => scanWithCamera("token")} compact secondary />
                    <Button label="Paste" onPress={() => pasteClipboardTo((value) => setTokenSendForm((p) => ({ ...p, to: value } )))} compact />
                  </View>
                  <Button label={busy ? "Sending…" : "Send Token"} onPress={() => sendTokenAction(selectedTokenForSend)} disabled={busy} />
                  <Button label="Close" onPress={() => setSelectedTokenForSend(null)} secondary />
                </Card>
              ) : null}
            </View>
          )}

          {tab === "contracts" && (
            <View style={styles.sectionGap}>
              <Card title="Deploy builtin contract" subtitle="Use one of the chain templates.">
                <View style={styles.templateWrap}>
                  {BUILTIN_TEMPLATES.map((item) => (
                    <Chip key={item.value} label={item.label} active={deployForm.template === item.value} onPress={() => setDeployForm((p) => ({ ...p, template: item.value }))} />
                  ))}
                </View>
                <Field label="Gas" value={deployForm.gas} onChangeText={(v) => setDeployForm((p) => ({ ...p, gas: v }))} keyboardType="numeric" placeholder="500000" />
                <Button label={busy ? "Deploying…" : "Deploy Builtin"} onPress={deployBuiltinAction} disabled={busy} />
                <Button label="Refresh Factory" onPress={refreshFactory} secondary />
                <Text style={styles.helperText}>If the selected template is `dex_factory`, the deployed address becomes the canonical shared DEX factory for all users.</Text>
              </Card>

              <Card title="Custom plugin deploy" subtitle="Compile Go plugin source and deploy it from mobile.">
                <Field label="Source code" value={customSource} onChangeText={setCustomSource} multiline numberOfLines={10} placeholder="Go plugin source…" />
                <View style={styles.inlineButtons}>
                  <Button label={busy ? "Compiling…" : "Compile Plugin"} onPress={compileCustomPluginAction} disabled={busy} />
                  <Button label="Deploy Plugin" onPress={deployCustomPluginAction} secondary disabled={busy || !compiledPluginUri} />
                </View>
                <Text style={styles.helperText}>
                  {compiledPlugin ? `Compiled: ${compiledPluginSize} bytes ready for upload.` : "Compile source first, then deploy the generated .so file."}
                </Text>
              </Card>

              <Card title="Call contract" subtitle="Read or write via wallet signed calls.">
                <Field label="Contract address" value={callForm.contract} onChangeText={(v) => setCallForm((p) => ({ ...p, contract: v }))} placeholder="0x..." />
                <Field label="Function" value={callForm.functionName} onChangeText={(v) => setCallForm((p) => ({ ...p, functionName: v }))} placeholder="Transfer / BalanceOf / Ping" />
                <Field label="Args (comma separated)" value={callForm.args} onChangeText={(v) => setCallForm((p) => ({ ...p, args: v }))} placeholder="addr, 1000" />
                <Field label="Value" value={callForm.value} onChangeText={(v) => setCallForm((p) => ({ ...p, value: v }))} keyboardType="decimal-pad" placeholder="0" />
                <Field label="Gas" value={callForm.gas} onChangeText={(v) => setCallForm((p) => ({ ...p, gas: v }))} keyboardType="numeric" placeholder="200000" />
                <Button label={busy ? "Submitting…" : "Submit Call"} onPress={callContractAction} disabled={busy} />
              </Card>

              <Card title="Inspect contract" subtitle="Read ABI and contract storage from the node.">
                <Field label="Contract address" value={inspectForm.address} onChangeText={(v) => setInspectForm({ address: v })} placeholder="0x..." />
                <Button label={busy ? "Loading…" : "Load ABI / Storage"} onPress={inspectContractAction} disabled={busy} />
                <Text style={styles.inspectTitle}>ABI</Text>
                <Text style={styles.inspectBox}>{inspectData.abi ? JSON.stringify(inspectData.abi, null, 2) : "No ABI loaded yet."}</Text>
                <Text style={styles.inspectTitle}>Storage</Text>
                <Text style={styles.inspectBox}>{inspectData.storage ? JSON.stringify(inspectData.storage, null, 2) : "No storage loaded yet."}</Text>
              </Card>
            </View>
          )}

          {tab === "browser" && (
            <View style={styles.sectionGap}>
              <Card title="Wallet Browser" subtitle="Paste any trusted DEX or dApp link here.">
                <Field
                  label="Website URL"
                  value={browserInput}
                  onChangeText={setBrowserInput}
                  placeholder="https://example-dapp.com"
                  autoCapitalize="none"
                />
                <View style={styles.inlineButtons}>
                  <Button label="Go" onPress={() => openBrowserTarget(browserInput)} compact />
                  <Button label="Paste Link" onPress={() => pasteClipboardTo((value) => setBrowserInput(coerceBrowserUrl(value)))} compact secondary />
                  <Button label="Home" onPress={() => openBrowserTarget(DEFAULT_BROWSER_URL)} compact secondary />
                </View>
                <View style={styles.browserSurface}>
                  <View style={styles.browserToolbar}>
                    <Button label="←" onPress={() => browserRef.current?.goBack()} compact secondary disabled={!browserCanGoBack} />
                    <Button label="→" onPress={() => browserRef.current?.goForward()} compact secondary disabled={!browserCanGoForward} />
                    <Button label="Reload" onPress={() => browserRef.current?.reload()} compact secondary />
                    <Button label="Open External" onPress={() => Linking.openURL(browserUrl)} compact secondary />
                  </View>
                  {browserLoading ? <Text style={styles.browserHint}>Loading…</Text> : null}
                  <WebView
                    ref={browserRef}
                    source={{ uri: browserUrl }}
                    style={styles.browserFrame}
                    onLoadStart={() => setBrowserLoading(true)}
                    onLoadEnd={() => setBrowserLoading(false)}
                    onNavigationStateChange={(navState) => {
                      setBrowserCanGoBack(navState.canGoBack);
                      setBrowserCanGoForward(navState.canGoForward);
                      if (navState.url) {
                        setBrowserUrl(navState.url);
                        setBrowserInput(navState.url);
                      }
                    }}
                    setSupportMultipleWindows={false}
                    javaScriptEnabled
                    domStorageEnabled
                    startInLoadingState
                  />
                </View>
                <Text style={styles.helperText}>
                  DEX is treated as an external site. Open it here and interact from inside the wallet browser.
                </Text>
              </Card>
            </View>
          )}

          {tab === "bridge" && (
            <View style={styles.sectionGap}>
              <Card title="Bridge family" subtitle="Select the chain family / adapter model.">
                <View style={styles.templateWrap}>
                  {(bridgeFamilies.length ? bridgeFamilies : [
                    { id: "evm", name: "EVM" },
                    { id: "utxo", name: "UTXO" },
                    { id: "cosmos", name: "Cosmos" },
                    { id: "substrate", name: "Substrate" },
                    { id: "solana", name: "Solana" },
                    { id: "xrpl", name: "XRPL" },
                    { id: "ton", name: "TON" },
                    { id: "cardano", name: "Cardano" },
                    { id: "aptos", name: "Aptos" },
                    { id: "sui", name: "Sui" },
                    { id: "near", name: "NEAR" },
                    { id: "icp", name: "ICP" },
                  ]).map((family) => {
                    const familyId = String(family.id || "").toLowerCase();
                    const active = String(currentBridgeChain?.family || "evm").toLowerCase() === familyId;
                    return (
                      <Chip
                        key={`fam-${family.id}`}
                        label={family.name || family.id}
                        active={active}
                        onPress={() => {
                          setBridgeChainForm((p) => ({ ...p, family: familyId, adapter: familyId }));
                          setBridgeTokenAdminForm((p) => ({ ...p, family: familyId }));
                        }}
                      />
                    );
                  })}
                </View>
                <Text style={styles.helperText}>
                  Current family: {currentBridgeChain?.family || "evm"} · {currentBridgeChain?.adapter || currentBridgeChain?.family || "evm"}
                </Text>
                <Button label="Refresh Families" onPress={loadBridgeFamilies} secondary />
              </Card>

              <Card title="Bridge chain" subtitle="Select a configured chain from the registry.">
                <View style={styles.templateWrap}>
                  {(bridgeChains.length ? bridgeChains : [{ id: bridgeChainId, chain_id: bridgeChainId, name: "BSC Testnet" }]).map((cfg) => {
                    const cfgId = String(cfg.id || cfg.chain_id || "").toLowerCase();
                    const active = String(bridgeChainId || "").toLowerCase() === cfgId;
                    const disabled = cfg?.enabled === false;
                    return (
                      <Chip
                        key={`${cfg.id || cfg.chain_id || cfg.name || "chain"}`}
                        label={`${cfg.name || cfg.id || cfg.chain_id}${disabled ? " (disabled)" : ""}`}
                        active={active}
                        onPress={() => applyBridgeChainSelection(cfg)}
                      />
                    );
                  })}
                </View>
                <Text style={styles.helperText}>
                  Selected: {currentBridgeChain?.name || bridgeChainId} · {currentBridgeChain?.chain_id || currentBridgeChain?.chainId || "unknown"} · {currentBridgeChain?.family || "evm"}{currentBridgeChain?.enabled === false ? " · disabled" : ""}
                </Text>
                <Button label="Refresh Bridge Chains" onPress={loadBridgeChains} secondary />
              </Card>

              {isExternalBridgeFamily ? (
                <Card title="External source metadata" subtitle={`${currentBridgeFamily.toUpperCase()} bridge requests need source tx details.`}>
                  <Field label="Source token / asset" value={bridgeForm.token} onChangeText={(v) => setBridgeForm((p) => ({ ...p, token: v }))} placeholder="denom / tx token / asset id" />
                  <Field label="Source tx hash" value={bridgeForm.sourceTxHash} onChangeText={(v) => setBridgeForm((p) => ({ ...p, sourceTxHash: v }))} placeholder="tx hash / transaction id" />
                  <Field label="Source address" value={bridgeForm.sourceAddress} onChangeText={(v) => setBridgeForm((p) => ({ ...p, sourceAddress: v }))} placeholder="sender address" />
                  <Field label="Source memo / note" value={bridgeForm.sourceMemo} onChangeText={(v) => setBridgeForm((p) => ({ ...p, sourceMemo: v }))} placeholder="memo / note" />
                  <Field label={(currentBridgeFamily === "solana" || currentBridgeFamily === "substrate" || currentBridgeFamily === "xrpl" || currentBridgeFamily === "ton" || currentBridgeFamily === "near" || currentBridgeFamily === "aptos") ? "Recent blockhash / sequence" : "Source sequence / nonce"} value={bridgeForm.sourceSequence} onChangeText={(v) => setBridgeForm((p) => ({ ...p, sourceSequence: v }))} placeholder={(currentBridgeFamily === "solana" || currentBridgeFamily === "substrate" || currentBridgeFamily === "xrpl" || currentBridgeFamily === "ton" || currentBridgeFamily === "near" || currentBridgeFamily === "aptos") ? "recent blockhash / slot" : "sequence / nonce"} />
                  <Field label="Source output index" value={bridgeForm.sourceOutput} onChangeText={(v) => setBridgeForm((p) => ({ ...p, sourceOutput: v }))} placeholder="UTXO output index" />
                  <Text style={styles.helperText}>
                    Cosmos requires source tx hash, source address and memo. UTXO/Cardano require source tx hash, source address and output index. Solana/Substrate/XRPL/TON/NEAR/Aptos require source tx hash, source address and recent blockhash/sequence.
                  </Text>
                </Card>
              ) : null}

              <Card title={isExternalBridgeFamily ? "Register external lock" : "LQD → BSC lock"} subtitle={isExternalBridgeFamily ? "Register a lock proof from Cosmos/UTXO/Cardano/Solana/Substrate/XRPL/TON/NEAR/Aptos and mint on LQD." : "Lock native LQD for release on the selected chain."}>
                <View style={styles.templateWrap}>
                  <Chip label="Public" active={bridgeMode === "public"} onPress={() => setBridgeMode("public")} />
                  <Chip label="Private" active={bridgeMode === "private"} onPress={() => setBridgeMode("private")} />
                </View>
                <Field label={isExternalBridgeFamily ? "LQD recipient" : "BSC recipient"} value={isExternalBridgeFamily ? bridgeForm.toLqd : bridgeForm.toBsc} onChangeText={(v) => setBridgeForm((p) => (isExternalBridgeFamily ? { ...p, toLqd: v } : { ...p, toBsc: v }))} placeholder="0x..." />
                <Field label="Amount" value={bridgeForm.amount} onChangeText={(v) => setBridgeForm((p) => ({ ...p, amount: v }))} keyboardType="decimal-pad" placeholder="0.0" />
                <View style={styles.inlineButtons}>
                  <Button label="Scan Recipient" onPress={() => scanWithCamera("bridge")} compact secondary />
                  <Button label="Paste" onPress={() => pasteClipboardTo((value) => setBridgeForm((p) => (isExternalBridgeFamily ? { ...p, toLqd: value } : { ...p, toBsc: value } )))} compact />
                </View>
                <Button
                  label={busy ? "Locking…" : (isExternalBridgeFamily ? "Register Lock" : "Lock on LQD")}
                  onPress={async () => {
                    setBusy(true);
                    try {
                      if (isExternalBridgeFamily) {
                        const sourceTxHash = bridgeForm.sourceTxHash.trim();
                        const sourceAddress = bridgeForm.sourceAddress.trim();
                        if (!sourceTxHash || !sourceAddress) {
                          setStatus("Enter source tx hash and source address");
                          return;
                        }
                        if (currentBridgeFamily === "cosmos" && !bridgeForm.sourceMemo.trim()) {
                          setStatus("Cosmos bridge needs a memo/note");
                          return;
                        }
                        if (currentBridgeFamily === "utxo" && !bridgeForm.sourceOutput.trim()) {
                          setStatus("UTXO bridge needs a source output index");
                          return;
                        }
                        if (currentBridgeFamily === "cardano" && !bridgeForm.sourceOutput.trim()) {
                          setStatus("Cardano bridge needs a source output index");
                          return;
                        }
                        if ((currentBridgeFamily === "solana" || currentBridgeFamily === "substrate" || currentBridgeFamily === "xrpl" || currentBridgeFamily === "ton" || currentBridgeFamily === "near" || currentBridgeFamily === "aptos") && !bridgeForm.sourceSequence.trim()) {
                          setStatus("Solana/Substrate/XRPL/TON/NEAR/Aptos bridge needs a recent blockhash / sequence");
                          return;
                        }
                        const lockPayload = {
                          chain_id: bridgeChainId,
                          family: currentBridgeFamily,
                          adapter: currentBridgeFamily,
                          tx_hash: sourceTxHash,
                          source_tx_hash: sourceTxHash,
                          source_address: sourceAddress,
                          source_memo: bridgeForm.sourceMemo.trim(),
                          source_sequence: bridgeForm.sourceSequence.trim(),
                          source_output: bridgeForm.sourceOutput.trim(),
                          to_lqd: bridgeForm.toLqd.trim() || wallet?.address || "",
                          token: bridgeForm.token.trim(),
                          amount: parseUnits(bridgeForm.amount, 8),
                          mode: bridgeMode,
                        };
                        const res = await postJson(`${nodeUrl}/bridge/lock_chain`, lockPayload);
                        setStatus(`External lock registered: ${shortAddress(res?.status || "ok", 12, 4)}`);
                      } else {
                        if (!wallet?.address || !wallet?.privateKey) return setStatus("Unlock wallet first");
                        const lockPayload = {
                          from: wallet.address,
                          to_bsc: bridgeForm.toBsc.trim(),
                          amount: parseUnits(bridgeForm.amount, 8),
                          chain_id: bridgeChainId,
                          family: currentBridgeChain?.family || "evm",
                          gas: 200000,
                          gas_price: await nodeBaseFee(nodeUrl).catch(() => 10),
                          private_key: wallet.privateKey,
                          mode: bridgeMode,
                        };
                        const res = bridgeMode === "private"
                          ? await walletBridgePrivateLock(walletUrl, lockPayload)
                          : await walletBridgeLock(walletUrl, lockPayload);
                        setStatus(`Bridge lock tx: ${shortAddress(res?.tx_hash || "", 8, 6)}`);
                      }
                    } catch (e) {
                      setStatus(e.message || "Bridge lock failed");
                    } finally {
                      setBusy(false);
                    }
                  }}
                  disabled={busy}
                />
              </Card>

              <Card title="BSC token lock" subtitle="Lock a token on the selected chain and mint on LQD.">
                <Field label="Token address" value={bridgeForm.token} onChangeText={(v) => setBridgeForm((p) => ({ ...p, token: v }))} placeholder="0x..." />
                <Field label="Amount" value={bridgeForm.amount} onChangeText={(v) => setBridgeForm((p) => ({ ...p, amount: v }))} keyboardType="decimal-pad" placeholder="0.0" />
                <Field label="LQD recipient" value={bridgeForm.toLqd} onChangeText={(v) => setBridgeForm((p) => ({ ...p, toLqd: v }))} placeholder="0x..." />
                {isExternalBridgeFamily ? (
                  <Text style={styles.helperText}>For Cosmos/UTXO/Cardano/Solana/Substrate/XRPL/TON/NEAR/Aptos, use the metadata card above and then register the external source lock.</Text>
                ) : null}
                <View style={styles.inlineButtons}>
                  <Button label="Scan Recipient" onPress={() => scanWithCamera("bridge")} compact secondary />
                  <Button label="Paste Token" onPress={() => pasteClipboardTo((value) => setBridgeForm((p) => ({ ...p, token: value } )))} compact />
                </View>
                <Button
                  label={busy ? "Locking…" : "Lock BSC Token"}
                  onPress={async () => {
                    if (!wallet?.address || !wallet?.privateKey) return setStatus("Unlock wallet first");
                    setBusy(true);
                    try {
                      const res = bridgeMode === "private"
                        ? await walletBridgePrivateLockBscToken(walletUrl, {
                          private_key: wallet.privateKey,
                          token: bridgeForm.token.trim(),
                          amount: parseUnits(bridgeForm.amount, 18),
                          to_lqd: bridgeForm.toLqd.trim(),
                          chain_id: bridgeChainId,
                          family: currentBridgeChain?.family || "evm",
                          mode: bridgeMode,
                        })
                        : await walletBridgeLockBscToken(walletUrl, {
                        private_key: wallet.privateKey,
                        token: bridgeForm.token.trim(),
                        amount: parseUnits(bridgeForm.amount, 18),
                        to_lqd: bridgeForm.toLqd.trim(),
                        chain_id: bridgeChainId,
                        family: currentBridgeChain?.family || "evm",
                        mode: bridgeMode,
                      });
                      setStatus(`BSC lock tx: ${shortAddress(res?.tx_hash || "", 8, 6)}`);
                    } catch (e) {
                      setStatus(e.message || "BSC token lock failed");
                    } finally {
                      setBusy(false);
                    }
                  }}
                  disabled={busy}
                />
              </Card>

              <Card title="Burn LQD bridge token" subtitle="Burn wrapped LQD token to release on the selected chain.">
                <Field label="Token address" value={bridgeForm.token} onChangeText={(v) => setBridgeForm((p) => ({ ...p, token: v }))} placeholder="0x..." />
                <Field label="Amount" value={bridgeForm.amount} onChangeText={(v) => setBridgeForm((p) => ({ ...p, amount: v }))} keyboardType="decimal-pad" placeholder="0.0" />
                <Field label="BSC recipient" value={bridgeForm.toBsc} onChangeText={(v) => setBridgeForm((p) => ({ ...p, toBsc: v }))} placeholder="0x..." />
                <View style={styles.inlineButtons}>
                  <Button label="Scan Recipient" onPress={() => scanWithCamera("bridge")} compact secondary />
                  <Button label="Paste Token" onPress={() => pasteClipboardTo((value) => setBridgeForm((p) => ({ ...p, token: value } )))} compact />
                </View>
                <Button
                  label={busy ? "Burning…" : "Burn LQD"}
                  onPress={async () => {
                    if (!wallet?.address || !wallet?.privateKey) return setStatus("Unlock wallet first");
                    setBusy(true);
                    try {
                      const burnMeta = {
                        source_tx_hash: bridgeForm.sourceTxHash.trim(),
                        source_address: bridgeForm.sourceAddress.trim(),
                        source_memo: bridgeForm.sourceMemo.trim(),
                        source_sequence: bridgeForm.sourceSequence.trim(),
                        source_output: bridgeForm.sourceOutput.trim(),
                      };
                      const res = bridgeMode === "private"
                        ? await walletBridgePrivateBurnLqdToken(walletUrl, {
                          private_key: wallet.privateKey,
                          token: bridgeForm.token.trim(),
                          amount: parseUnits(bridgeForm.amount, 18),
                          to_bsc: bridgeForm.toBsc.trim(),
                          chain_id: bridgeChainId,
                          family: currentBridgeChain?.family || "evm",
                          mode: bridgeMode,
                          ...burnMeta,
                        })
                        : await walletBridgeBurnLqdToken(walletUrl, {
                        private_key: wallet.privateKey,
                        token: bridgeForm.token.trim(),
                        amount: parseUnits(bridgeForm.amount, 18),
                        to_bsc: bridgeForm.toBsc.trim(),
                        chain_id: bridgeChainId,
                        family: currentBridgeChain?.family || "evm",
                        mode: bridgeMode,
                        ...burnMeta,
                      });
                      setStatus(`Burn tx: ${shortAddress(res?.tx_hash || "", 8, 6)}`);
                    } catch (e) {
                      setStatus(e.message || "Burn failed");
                    } finally {
                      setBusy(false);
                    }
                  }}
                  disabled={busy}
                />
              </Card>

              <Card title="Bridge state" subtitle="Recent requests and token mappings from the node.">
                <Text style={styles.inspectTitle}>Requests</Text>
                <Text style={styles.inspectBox}>{JSON.stringify(bridgeRequests, null, 2)}</Text>
                <Text style={styles.inspectTitle}>Chains</Text>
                <Text style={styles.inspectBox}>{JSON.stringify(bridgeChains, null, 2)}</Text>
                <Text style={styles.inspectTitle}>Families</Text>
                <Text style={styles.inspectBox}>{JSON.stringify(bridgeFamilies, null, 2)}</Text>
                <Text style={styles.inspectTitle}>Tokens</Text>
                <Text style={styles.inspectBox}>{JSON.stringify(bridgeTokens, null, 2)}</Text>
              </Card>
            </View>
          )}

          {tab === "approvals" && (
            <View style={styles.sectionGap}>
              <Card title="DApp approvals" subtitle="Connect requests and trusted origins.">
                <Text style={styles.helperText}>{deepLinkHint || "Open a lqdwallet://connect QR or deep link to simulate a dApp connection request."}</Text>
                <View style={styles.inlineButtons}>
                  <Button label="Open Receive" onPress={() => setReceiveVisible(true)} compact secondary />
                  <Button label="Simulate Request" onPress={() => queueApprovalRequest({ origin: "https://example-dapp.local", name: "Example DApp", callback: "" })} compact />
                </View>
              </Card>

              <Card title="Pending requests" subtitle="Approve or reject incoming connection requests.">
                {!pendingApprovals.length ? (
                  <Text style={styles.helperText}>No pending approvals right now.</Text>
                ) : (
                  pendingApprovals.map((item) => (
                    <View key={item.id} style={styles.rowCard}>
                      <View style={{ flex: 1 }}>
                        <Text style={styles.rowTitle}>{item.name}</Text>
                        <Text style={styles.rowSub}>{item.origin}</Text>
                        <Text style={styles.rowSub}>Status: {item.status}</Text>
                        {item.callback ? <Text style={styles.rowSub}>Callback: {shortAddress(item.callback, 18, 6)}</Text> : null}
                      </View>
                      <View style={styles.rowActions}>
                        <Button label="Approve" onPress={() => approveRequest(item)} compact />
                        <Button label="Reject" onPress={() => rejectRequest(item)} compact danger />
                      </View>
                    </View>
                  ))
                )}
              </Card>

              <Card title="Trusted origins" subtitle="Approved dApps can reconnect without asking again.">
                {!trustedOrigins.length ? (
                  <Text style={styles.helperText}>No trusted origins saved yet.</Text>
                ) : (
                  trustedOrigins.map((origin) => (
                    <View key={origin} style={styles.rowCard}>
                      <Text style={styles.rowTitle}>{origin}</Text>
                    </View>
                  ))
                )}
              </Card>
            </View>
          )}

          {tab === "activity" && (
            <View style={styles.sectionGap}>
              <Card title="Activity" subtitle="Recent on-chain and local wallet actions.">
                <View style={styles.inlineButtons}>
                  <Button label="Refresh" onPress={refreshWalletSnapshot} compact secondary />
                  <Button label="Open Explorer" onPress={() => Share.share({ message: explorerUrl || "Explorer URL not configured" })} compact />
                </View>
                {!activity.length ? (
                  <Text style={styles.helperText}>No activity recorded yet.</Text>
                ) : (
                  activity
                    .filter((item) => txTouchesAddress(item, wallet.address) || String(item.type || "").length > 0)
                    .map((item, idx) => <ActivityRow key={`${item.TxHash || item.tx_hash || idx}`} item={item} />)
                )}
              </Card>
            </View>
          )}

          {tab === "networks" && (
            <View style={styles.sectionGap}>
              <Card title="Current network" subtitle={currentNetwork.name}>
                <Text style={styles.helperText}>Chain ID: {currentNetwork.chainId}</Text>
                <Text style={styles.helperText}>Node: {currentNetwork.nodeUrl}</Text>
                <Text style={styles.helperText}>Wallet: {currentNetwork.walletUrl}</Text>
              </Card>

              <Card title="Add network" subtitle="Create a new custom chain entry.">
                <Field label="Name" value={networkForm.name} onChangeText={(v) => setNetworkForm((p) => ({ ...p, name: v }))} placeholder="My Network" />
                <Field label="Chain ID" value={networkForm.chainId} onChangeText={(v) => setNetworkForm((p) => ({ ...p, chainId: v }))} placeholder="0x8b" />
                <Field label="Node URL" value={networkForm.nodeUrl} onChangeText={(v) => setNetworkForm((p) => ({ ...p, nodeUrl: v }))} placeholder="http://127.0.0.1:6500" />
                <Field label="Wallet URL" value={networkForm.walletUrl} onChangeText={(v) => setNetworkForm((p) => ({ ...p, walletUrl: v }))} placeholder="http://127.0.0.1:8080" />
                <Field label="Explorer URL" value={networkForm.explorerUrl} onChangeText={(v) => setNetworkForm((p) => ({ ...p, explorerUrl: v }))} placeholder="http://localhost:3001" />
                <Field label="Symbol" value={networkForm.symbol} onChangeText={(v) => setNetworkForm((p) => ({ ...p, symbol: v }))} placeholder="LQD" />
                <Button label="Add Network" onPress={addNetworkAction} />
              </Card>

              <Card title="Switch / remove" subtitle="Your saved networks.">
                {networks.map((net) => (
                  <View key={net.id} style={styles.rowCard}>
                    <View style={{ flex: 1 }}>
                      <Text style={styles.rowTitle}>{net.name}</Text>
                      <Text style={styles.rowSub}>{net.chainId}</Text>
                      <Text style={styles.rowSub}>{shortAddress(net.nodeUrl, 12, 4)}</Text>
                    </View>
                    <View style={styles.rowActions}>
                      <Button label={activeNetworkId === net.id ? "Active" : "Switch"} onPress={() => switchNetworkAction(net.id)} compact secondary />
                      {DEFAULT_NETWORKS.some((d) => d.id === net.id) ? null : <Button label="Remove" onPress={() => removeNetworkAction(net.id)} compact danger />}
                    </View>
                  </View>
                ))}
              </Card>
            </View>
          )}

          {tab === "settings" && (
            <View style={styles.sectionGap}>
              <Card title="Endpoints" subtitle="Adjust the live services used by the app.">
                <Field label="Node URL" value={endpoints.nodeUrl} onChangeText={(v) => setEndpoints((p) => ({ ...p, nodeUrl: v }))} placeholder="http://127.0.0.1:6500" />
                <Field label="Wallet URL" value={endpoints.walletUrl} onChangeText={(v) => setEndpoints((p) => ({ ...p, walletUrl: v }))} placeholder="http://127.0.0.1:8080" />
                <Field label="Aggregator URL" value={endpoints.aggregatorUrl} onChangeText={(v) => setEndpoints((p) => ({ ...p, aggregatorUrl: v }))} placeholder="http://127.0.0.1:9000" />
                <Field label="Explorer URL" value={endpoints.explorerUrl} onChangeText={(v) => setEndpoints((p) => ({ ...p, explorerUrl: v }))} placeholder="http://localhost:3001" />
              </Card>

              <Card title="Wallet actions" subtitle="Security and backup.">
                <View style={styles.inlineButtons}>
                  <Button label={settingsAutoRefresh ? "Auto-refresh: On" : "Auto-refresh: Off"} onPress={() => setSettingsAutoRefresh((p) => !p)} compact secondary />
                  <Button label={biometricEnabled ? "Biometric: On" : "Biometric: Off"} onPress={() => setBiometricEnabled((p) => !p)} compact secondary />
                  <Button label="Copy Backup" onPress={saveBackupToClipboard} compact />
                  <Button label="Restore Backup" onPress={restoreBackupFromText} compact secondary />
                </View>
                <Field label="Backup JSON" value={backupText} onChangeText={setBackupText} multiline numberOfLines={6} placeholder="Paste a previously exported backup JSON here…" />
                <View style={styles.inlineButtons}>
                  <Button label="Lock Wallet" onPress={lockWalletAction} compact danger />
                  <Button label="Clear Local Vault" onPress={clearLocalWallet} compact danger />
                </View>
                <Text style={styles.helperText}>The vault itself is encrypted with your password. Backup exports include the encrypted vault, networks, watchlist and settings only.</Text>
              </Card>

              <Card title="Advanced tools" subtitle="Keep heavy developer tools out of the main wallet navigation.">
                <View style={styles.templateWrap}>
                  {ADVANCED_TABS.map((item) => (
                    <Chip key={item.id} label={item.label} active={tab === item.id} onPress={() => setTab(item.id)} />
                  ))}
                </View>
                <Text style={styles.helperText}>
                  Contracts, bridge and dApp approval tools are still available here, but the main wallet stays focused on balance, tokens and browser.
                </Text>
              </Card>
            </View>
          )}

          <Text style={styles.statusText}>{status}</Text>
        </ScrollView>
        <View style={styles.bottomNav}>
          {TABS.map((item) => (
            <NavItem key={item.id} icon={item.icon} label={item.label} active={tab === item.id} onPress={() => setTab(item.id)} />
          ))}
        </View>
      </KeyboardAvoidingView>
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
    padding: 24,
  },
  heroTitle: {
    color: "#f4f7ff",
    fontSize: 30,
    fontWeight: "800",
    marginBottom: 8,
  },
  heroText: {
    color: "#9ea8cc",
    fontSize: 15,
    textAlign: "center",
    lineHeight: 22,
  },
  scrollPad: {
    padding: 18,
    paddingBottom: 36,
  },
  topActions: {
    flexDirection: "row",
    gap: 8,
    alignItems: "center",
    justifyContent: "flex-end",
  },
  topBar: {
    paddingHorizontal: 18,
    paddingTop: Platform.select({ ios: 14, android: 26, default: 18 }),
    paddingBottom: 12,
    gap: 12,
  },
  topIdentity: {
    alignItems: "center",
  },
  topAddress: {
    color: "#dce4ff",
    fontSize: 16,
    fontWeight: "800",
    letterSpacing: 0.2,
  },
  topNetwork: {
    color: "#9ea8cc",
    fontSize: 13,
    marginTop: 3,
    fontWeight: "600",
  },
  walletPill: {
    backgroundColor: "#16203a",
    borderColor: "#273152",
    borderWidth: 1,
    borderRadius: 999,
    paddingVertical: 10,
    paddingHorizontal: 16,
    minWidth: 92,
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
    padding: 16,
    paddingBottom: 28,
  },
  tabRow: {
    flexDirection: "row",
    flexWrap: "wrap",
    gap: 8,
    marginBottom: 14,
  },
  chip: {
    paddingVertical: 9,
    paddingHorizontal: 12,
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
    fontSize: 12,
    fontWeight: "700",
  },
  chipTextActive: {
    color: "#f6f3ff",
  },
  summaryGrid: {
    flexDirection: "row",
    gap: 10,
    flexWrap: "wrap",
  },
  actionGrid: {
    flexDirection: "row",
    flexWrap: "wrap",
    gap: 10,
  },
  stat: {
    flexGrow: 1,
    flexBasis: "31%",
    minWidth: 96,
    backgroundColor: "#151b31",
    borderColor: "#273152",
    borderWidth: 1,
    borderRadius: 18,
    padding: 14,
  },
  statLabel: {
    color: "#8b94b7",
    fontSize: 12,
    marginBottom: 6,
    fontWeight: "700",
  },
  statValue: {
    color: "#f4f7ff",
    fontSize: 18,
    fontWeight: "800",
  },
  statSub: {
    color: "#91f7bf",
    fontSize: 12,
    marginTop: 4,
    fontWeight: "700",
  },
  sectionGap: {
    gap: 12,
    marginTop: 12,
  },
  card: {
    backgroundColor: "#151b31",
    borderColor: "#273152",
    borderWidth: 1,
    borderRadius: 22,
    padding: 15,
    gap: 10,
  },
  cardHeader: {
    marginBottom: 2,
  },
  cardTitle: {
    color: "#f4f7ff",
    fontSize: 18,
    fontWeight: "800",
  },
  cardSubtitle: {
    color: "#9ca7ca",
    fontSize: 13,
    marginTop: 4,
    lineHeight: 19,
  },
  fieldWrap: {
    gap: 6,
  },
  fieldLabelRow: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
  },
  fieldLabel: {
    color: "#c6cee8",
    fontSize: 13,
    fontWeight: "700",
  },
  input: {
    backgroundColor: "#1b2342",
    borderColor: "#2f3960",
    borderWidth: 1,
    borderRadius: 16,
    color: "#f4f7ff",
    paddingHorizontal: 14,
    paddingVertical: 12,
    minHeight: 46,
    fontSize: 14,
  },
  inputMultiline: {
    minHeight: 110,
    textAlignVertical: "top",
  },
  inputReadonly: {
    opacity: 0.8,
  },
  button: {
    backgroundColor: "#8a78ff",
    borderRadius: 16,
    paddingVertical: 13,
    paddingHorizontal: 16,
    alignItems: "center",
    justifyContent: "center",
    minHeight: 48,
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
    paddingVertical: 10,
    paddingHorizontal: 12,
    minHeight: 38,
    borderRadius: 12,
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
    fontSize: 15,
  },
  buttonTextSecondary: {
    color: "#b6c0e6",
  },
  buttonTextDanger: {
    color: "#ffb4c0",
  },
  helperText: {
    color: "#8f9bc1",
    fontSize: 12,
    lineHeight: 18,
  },
  bottomNav: {
    flexDirection: "row",
    gap: 8,
    paddingHorizontal: 12,
    paddingTop: 10,
    paddingBottom: 14,
    borderTopWidth: 1,
    borderTopColor: "#242d4e",
    backgroundColor: "#0b1020",
    alignItems: "center",
    justifyContent: "space-between",
  },
  navItem: {
    flex: 1,
    minWidth: 0,
    alignItems: "center",
    justifyContent: "center",
    paddingVertical: 9,
    paddingHorizontal: 6,
    borderRadius: 16,
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
    fontSize: 14,
    fontWeight: "800",
    marginBottom: 3,
  },
  navIconActive: {
    color: "#f4f7ff",
  },
  navLabel: {
    color: "#9aa5ca",
    fontSize: 9,
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
    padding: 18,
  },
  modalCard: {
    backgroundColor: "#151b31",
    borderColor: "#3a4670",
    borderWidth: 1,
    borderRadius: 24,
    padding: 18,
    gap: 12,
  },
  qrWrap: {
    alignItems: "center",
    justifyContent: "center",
    paddingVertical: 10,
    backgroundColor: "#0f152a",
    borderRadius: 20,
    borderWidth: 1,
    borderColor: "#2a3558",
  },
  scannerBackdrop: {
    flex: 1,
    backgroundColor: "#050814",
    paddingTop: 40,
    paddingHorizontal: 16,
    paddingBottom: 16,
  },
  scannerHeader: {
    gap: 10,
    marginBottom: 12,
  },
  scannerBody: {
    flex: 1,
    borderRadius: 24,
    overflow: "hidden",
    borderWidth: 1,
    borderColor: "#2b3557",
    backgroundColor: "#0d1326",
  },
  cameraFallback: {
    flex: 1,
    alignItems: "center",
    justifyContent: "center",
    padding: 18,
    gap: 14,
  },
  statusText: {
    color: "#c6d2ff",
    fontSize: 13,
    marginTop: 14,
    lineHeight: 20,
  },
  largeCode: {
    color: "#dce4ff",
    fontSize: 13,
    lineHeight: 19,
    fontFamily: Platform.select({ ios: "Courier", android: "monospace", default: "monospace" }),
  },
  inlineButtons: {
    flexDirection: "row",
    gap: 10,
    flexWrap: "wrap",
  },
  templateWrap: {
    flexDirection: "row",
    flexWrap: "wrap",
    gap: 8,
  },
  browserSurface: {
    backgroundColor: "#0f1428",
    borderColor: "#273152",
    borderWidth: 1,
    borderRadius: 18,
    overflow: "hidden",
  },
  browserToolbar: {
    flexDirection: "row",
    gap: 8,
    padding: 12,
    borderBottomColor: "#273152",
    borderBottomWidth: 1,
    flexWrap: "wrap",
  },
  browserFrame: {
    height: 520,
    backgroundColor: "#ffffff",
  },
  browserHint: {
    color: "#9aa5ca",
    fontSize: 12,
    paddingHorizontal: 12,
    paddingTop: 8,
  },
  rowCard: {
    backgroundColor: "#10162c",
    borderColor: "#273152",
    borderWidth: 1,
    borderRadius: 18,
    padding: 14,
    flexDirection: "row",
    alignItems: "flex-start",
    gap: 12,
  },
  rowTitle: {
    color: "#f4f7ff",
    fontSize: 15,
    fontWeight: "800",
  },
  rowSub: {
    color: "#9aa5ca",
    fontSize: 12,
    marginTop: 4,
  },
  tokenBalance: {
    color: "#91f7bf",
    fontSize: 13,
    fontWeight: "700",
    marginTop: 6,
  },
  rowActions: {
    gap: 8,
    alignItems: "flex-end",
  },
  inspectTitle: {
    color: "#b9c4e9",
    fontSize: 12,
    fontWeight: "700",
    marginTop: 8,
  },
  inspectBox: {
    color: "#dce4ff",
    backgroundColor: "#0f1428",
    borderRadius: 14,
    borderColor: "#273152",
    borderWidth: 1,
    padding: 12,
    fontSize: 12,
    lineHeight: 18,
    marginTop: 8,
  },
});

export default App;
