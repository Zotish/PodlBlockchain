// src/utils/txType.js

// MAIN CLASSIFIER: returns internal type string
export function detectTxType(tx, validatorSet = new Set()) {
    if (!tx) return "transfer";
  
    const fn =
      (tx.function ||
        tx.func ||
        tx.method ||
        tx.method_name ||
        "").toLowerCase();
  
    const rawType =
      (tx.tx_type || tx.type || tx.kind || tx.category || "").toLowerCase();
  
    const isContract = !!tx.is_contract;
    const to = (tx.to || tx.To || "").toLowerCase();
  
    // ---------------- REWARD TYPES ----------------
  
    // Backend may tag type directly:
    if (rawType.includes("validator") && rawType.includes("reward"))
      return "reward_validator";
    if (
      rawType.includes("lp") ||
      rawType.includes("liquidity_provider") ||
      rawType.includes("liquidity provider")
    )
      return "reward_lp";
    if (
      rawType.includes("validation") &&
      rawType.includes("contributor")
    )
      return "reward_contributor";
  
    // Function-based reward hints:
    if (fn === "validatorreward") return "reward_validator";
    if (fn === "lpreward") return "reward_lp";
    if (fn === "validationcontributorreward")
      return "reward_contributor";
  
    // Generic “blockReward”
    if (fn === "blockreward" || rawType === "reward") {
      if (validatorSet.has(to)) return "reward_validator";
      return "reward";
    }
  
    // ---------------- CONTRACT CREATION ----------------
  
    if (
      fn === "deploycontract" ||
      rawType.includes("contract_create") ||
      tx.contract_address ||
      tx.is_contract_creation === true ||
      (to === "0x0000000000000000000000000000000000000000" &&
        !tx.value) // fallback heuristic
    ) {
      return "contract_create";
    }
  
    // ---------------- TOKEN TRANSFER ----------------
    if (isContract && fn === "transfer") {
      return "token_transfer";
    }
  
    // ---------------- OTHER CONTRACT CALL ----------------
    if (isContract && fn) {
      return "contract_call";
    }
  
    // ---------------- DEFAULT: NATIVE TRANSFER ----------------
    return "transfer";
  }
  
  // HUMAN LABEL for UI
  export function getTxTypeLabel(type = "") {
    const t = type.toLowerCase();
    if (t === "reward_validator") return "Validator Reward";
    if (t === "reward_lp") return "Liquidity Provider Reward";
    if (t === "reward_contributor")
      return "Validation Contributor Reward";
    if (t === "reward") return "Reward";
    if (t === "contract_create") return "Contract Creation";
    if (t === "contract_call") return "Contract Call";
    if (t === "token_transfer") return "Token Transfer";
    if (t === "transfer") return "Transfer";
    return "Transfer";
  }
  
  // BADGE COMPONENT
  export const TxTypeBadge = ({ type }) => {
    const t = (type || "").toLowerCase();
  
    const map = {
      reward_validator: {
        color: "#0ea5e9",
        label: "Validator Reward",
      },
      reward_lp: {
        color: "#8b5cf6",
        label: "LP Provider Reward",
      },
      reward_contributor: {
        color: "#14b8a6",
        label: "Validation Contributor Reward",
      },
      reward: {
        color: "#0ea5e9",
        label: "Reward",
      },
      contract_create: {
        color: "#a855f7",
        label: "Contract Creation",
      },
      contract_call: {
        color: "#22c55e",
        label: "Contract Call",
      },
      token_transfer: {
        color: "#f97316",
        label: "Token Transfer",
      },
      transfer: {
        color: "#4b5563",
        label: "Transfer",
      },
    };
  
    const item = map[t] || map.transfer;
  
    return (
      <span
        style={{
          background: item.color,
          color: "white",
          padding: "2px 8px",
          borderRadius: 6,
          fontSize: 12,
        }}
      >
        {item.label}
      </span>
    );
  };
  