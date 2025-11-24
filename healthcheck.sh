#!/usr/bin/env bash
set -euo pipefail

NODE_URL="http://127.0.0.1:5000"
WALLET_URL="http://127.0.0.1:8080"

WALLET_COUNT=500
PARALLEL_LIMIT=500  # Reduced concurrency
TEST_PASS="Test@123"
TEST_VALUE=100
GAS=21000
BASEFEE_FALLBACK=10  # Adjusted fallback base fee
LOG_FILE="node.log"
POLL_PER_TX_SECONDS=60  # Reduced polling timeout

TMP_DIR=$(mktemp -d)

# -------------------------
# Utilities
# -------------------------
extract_txhash(){ jq -r '.tx_hash // .TxHash // .hash // .Hash // empty'; }
banner(){ echo -e "\n==== $* ===="; }

# -------------------------
# Node & wallet health
# -------------------------
banner "Checking node + wallet"
curl -sf "$NODE_URL/getheight" >/dev/null || { echo "❌ Node not reachable"; exit 1; }
echo "✅ Node responsive"

curl -sf "$WALLET_URL/wallet/new" -X OPTIONS >/dev/null || { echo "❌ Wallet server not reachable"; exit 1; }
echo "✅ Wallet server responsive"

# -------------------------
# Base fee
# -------------------------
BASEFEE=$(curl -s "$NODE_URL/rewards/recent" | jq -r 'last.BaseFee // .[-1].BaseFee // empty')
[[ -z "$BASEFEE" || "$BASEFEE" == "null" ]] && BASEFEE=$BASEFEE_FALLBACK
GAS_PRICE=$((BASEFEE + 1))
echo "BaseFee=$BASEFEE → Using gas_price=$GAS_PRICE"

# -------------------------
# Worker (runs in parallel)
# -------------------------
run_wallet_test() {
  IDX=$1
  PASS="$TEST_PASS$IDX"
  VALUE=$((TEST_VALUE + IDX))

  # 1) Create wallet
  CREATE=$(curl -s -X POST "$WALLET_URL/wallet/new" \
    -H "Content-Type: application/json" \
    -d "{\"password\":\"$PASS\"}")
  ADDR=$(echo "$CREATE" | jq -r .address)
  PRIV=$(echo "$CREATE" | jq -r .private_key)
  if [[ -z "$ADDR" || "$ADDR" == "null" ]]; then
    echo "❌ Wallet[$IDX] create failed: $CREATE" >> "$TMP_DIR/results.log"
    echo "Response: $CREATE" >> "$TMP_DIR/results.log"
    echo "$IDX failed" >> "$TMP_DIR/done.log"
    return 1
  fi

  # 2) Faucet
  curl -s -X POST "$NODE_URL/faucet" -H "Content-Type: application/json" \
    -d "{\"address\":\"$ADDR\"}" >/dev/null
  sleep 2  # Increased delay to ensure faucet processing

  # 3) Send self-tx
  SEND=$(curl -s -X POST "$WALLET_URL/wallet/send" \
    -H "Content-Type: application/json" \
    -d "{
      \"from\": \"$ADDR\",
      \"to\": \"$ADDR\",
      \"value\": $VALUE,
      \"gas\": $GAS,
      \"gas_price\": $GAS_PRICE,
      \"private_key\": \"$PRIV\"
    }")

  TXH=$(echo "$SEND" | extract_txhash)
  STATUS=$(echo "$SEND" | jq -r .status)
  printf "Wallet[%02d] %s  TX=%s  status=%s\n" "$IDX" "$ADDR" "$TXH" "$STATUS" >> "$TMP_DIR/results.log"

  # 4) Poll status
  waited=0
  while [[ $waited -lt $POLL_PER_TX_SECONDS ]]; do
    RECENT=$(curl -s "$NODE_URL/transactions/recent")
    ST=$(echo "$RECENT" | jq -r --arg H "$TXH" '
      map(select((.tx_hash//.TxHash//.hash//.Hash) == $H)) | .[0].status // empty
    ')
    if [[ "$ST" == "succsess" ]]; then
      echo "Wallet[$IDX] ✅ succsess" >> "$TMP_DIR/done.log"
      return 0
    fi
    if [[ "$ST" == "failed" ]]; then
      echo "Wallet[$IDX] ❌ failed" >> "$TMP_DIR/done.log"
      return 1
    fi
    sleep 2; waited=$((waited+2))
  done
  echo "Wallet[$IDX] ⏱ timeout" >> "$TMP_DIR/done.log"
  return 1
}

export -f run_wallet_test extract_txhash
export NODE_URL WALLET_URL TEST_PASS TEST_VALUE GAS GAS_PRICE TMP_DIR POLL_PER_TX_SECONDS

# -------------------------
# Launch parallel workers
# -------------------------
banner "Starting $WALLET_COUNT parallel transactions (concurrency=$PARALLEL_LIMIT)"
seq 1 "$WALLET_COUNT" | xargs -n1 -P "$PARALLEL_LIMIT" bash -c 'run_wallet_test "$@"' _

# -------------------------
# Summary
# -------------------------
banner "Summary"
echo "Results log: $TMP_DIR/results.log"
echo "Done log:    $TMP_DIR/done.log"

TOTAL=$(wc -l < "$TMP_DIR/results.log" 2>/dev/null || echo 0)
SUCCESS=$(grep -c "succsess" "$TMP_DIR/done.log" 2>/dev/null || echo 0)
FAILED=$(grep -c "failed" "$TMP_DIR/done.log" 2>/dev/null || echo 0)
TIMEOUTS=$(grep -c "timeout" "$TMP_DIR/done.log" 2>/dev/null || echo 0)

# Make sure counters are numeric
TOTAL=${TOTAL:-0}
SUCCESS=${SUCCESS:-0}
FAILED=${FAILED:-0}
TIMEOUTS=${TIMEOUTS:-0}

echo ""
echo "Total tx attempted: $TOTAL"
echo "✅ Success:  $SUCCESS / $TOTAL"
echo "❌ Failed:   $FAILED / $TOTAL"
echo "⏱ Timeout:  $TIMEOUTS / $TOTAL"

# Optional: also write this summary into the node log for later inspection
if [[ -n "${LOG_FILE:-}" ]]; then
  {
    echo ""
    echo "==== Load Test Summary ===="
    echo "Total tx attempted: $TOTAL"
    echo "Success:  $SUCCESS / $TOTAL"
    echo "Failed:   $FAILED / $TOTAL"
    echo "Timeout:  $TIMEOUTS / $TOTAL"
    echo "==========================="
  } >> "$LOG_FILE"
fi

if [[ -n "$LOG_FILE" && -f "$LOG_FILE" ]]; then
  banner "Recent mined blocks"
  tail -n 50 "$LOG_FILE" | grep -E "Mined block #|⏱️ Block #" || true
fi
