#!/usr/bin/env bash
set +e

CHAIN="${CHAIN:-https://chain.178-105-133-94.sslip.io}"
API="${API:-https://api.178-105-133-94.sslip.io}"
WALLET="${WALLET:-https://wallet.178-105-133-94.sslip.io}"
DEXAPI="${DEXAPI:-https://dex-api.178-105-133-94.sslip.io}"
EXPLORER="${EXPLORER:-https://warm-dragon-34d6ff.netlify.app}"
DEXUI="${DEXUI:-https://bright-crisp-91fe94.netlify.app}"

TS="$(date +%s)"
PASS=0
FAIL=0
RESULTS=""
FAILURES=""

log_pass() {
  PASS=$((PASS + 1))
  printf 'PASS %s\n' "$1"
  RESULTS="${RESULTS}
PASS|$1|$2"
}

log_fail() {
  FAIL=$((FAIL + 1))
  printf 'FAIL %s: %s\n' "$1" "$2"
  RESULTS="${RESULTS}
FAIL|$1|$2"
  FAILURES="${FAILURES}
$1: $2"
}

curl_do() {
  local method="$1" base="$2" path="$3" body="$4" timeout="${5:-30}"
  local retry_args=(--retry 3 --retry-all-errors --retry-delay 1)
  if [ -n "$body" ]; then
    OUT="$(curl -sS "${retry_args[@]}" --max-time "$timeout" -w '\n__HTTP_CODE__:%{http_code}' -X "$method" "$base$path" -H 'Content-Type: application/json' --data "$body" 2>&1)"
  else
    OUT="$(curl -sS "${retry_args[@]}" --max-time "$timeout" -w '\n__HTTP_CODE__:%{http_code}' -X "$method" "$base$path" 2>&1)"
  fi
  RC=$?
  if [ $RC -ne 0 ]; then
    HTTP_CODE="CURL$RC"
    HTTP_BODY="$OUT"
    return 1
  fi
  HTTP_CODE="${OUT##*__HTTP_CODE__:}"
  HTTP_BODY="${OUT%$'\n'__HTTP_CODE__:*}"
  case "$HTTP_CODE" in
    2*|3*) return 0 ;;
    *) return 1 ;;
  esac
}

json_val() {
  jq -r "$1 // empty" 2>/dev/null <<<"$2"
}

step_curl() {
  local name="$1" method="$2" base="$3" path="$4" body="$5" timeout="${6:-30}"
  curl_do "$method" "$base" "$path" "$body" "$timeout"
  if [ $? -eq 0 ]; then
    log_pass "$name" "$HTTP_CODE"
    return 0
  fi
  log_fail "$name" "$HTTP_CODE $(printf '%s' "$HTTP_BODY" | head -c 220)"
  return 1
}

cors_check() {
  local name="$1" base="$2" path="$3" origin="$4" method="$5"
  HEADERS="$(curl -sS --retry 3 --retry-all-errors --retry-delay 1 --max-time 20 -D - -o /dev/null -X OPTIONS "$base$path" -H "Origin: $origin" -H "Access-Control-Request-Method: $method" -H 'Access-Control-Request-Headers: content-type' 2>&1)"
  RC=$?
  if [ $RC -ne 0 ]; then
    log_fail "$name" "curl error $RC $(printf '%s' "$HEADERS" | head -c 160)"
    return 1
  fi
  STATUS="$(printf '%s\n' "$HEADERS" | awk 'toupper($0) ~ /^HTTP/ {code=$2} END {print code}')"
  ALLOW_ORIGIN="$(printf '%s\n' "$HEADERS" | awk 'BEGIN{IGNORECASE=1} /^access-control-allow-origin:/ {sub(/^[^:]+:[[:space:]]*/, ""); print; exit}' | tr -d '\r')"
  ALLOW_METHODS="$(printf '%s\n' "$HEADERS" | awk 'BEGIN{IGNORECASE=1} /^access-control-allow-methods:/ {sub(/^[^:]+:[[:space:]]*/, ""); print; exit}' | tr -d '\r')"
  if [ "$STATUS" != "200" ] && [ "$STATUS" != "204" ]; then
    log_fail "$name" "preflight status $STATUS"
    return 1
  fi
  if [ "$ALLOW_ORIGIN" != "*" ] && [ "$ALLOW_ORIGIN" != "$origin" ]; then
    log_fail "$name" "allow-origin mismatch: $ALLOW_ORIGIN"
    return 1
  fi
  printf '%s' "$ALLOW_METHODS" | tr '[:lower:]' '[:upper:]' | grep -q "$method"
  if [ $? -ne 0 ]; then
    log_fail "$name" "missing method $method: $ALLOW_METHODS"
    return 1
  fi
  log_pass "$name" "origin=$ALLOW_ORIGIN methods=$ALLOW_METHODS"
}

extract_hash() {
  jq -r '.tx_hash // .TxHash // .hash // .transaction.tx_hash // .transaction.TxHash // empty' 2>/dev/null <<<"$1"
}

tx_status() {
  jq -r '.transaction.status // .transaction.Status // .status // .Status // empty' 2>/dev/null <<<"$1"
}

tx_source() {
  jq -r '.source // empty' 2>/dev/null <<<"$1"
}

wait_for_tx() {
  local hash="$1" timeout="${2:-90}" allow_failed="${3:-0}"
  local deadline=$(( $(date +%s) + timeout ))
  LAST_TX=""
  while [ "$(date +%s)" -lt "$deadline" ]; do
    sleep 2
    curl_do GET "$CHAIN" "/tx/$hash" "" 15
    if [ $? -eq 0 ]; then
      LAST_TX="$HTTP_BODY"
      STATUS="$(tx_status "$LAST_TX" | tr '[:upper:]' '[:lower:]')"
      SOURCE="$(tx_source "$LAST_TX" | tr '[:upper:]' '[:lower:]')"
      if [ "$STATUS" = "failed" ]; then
        [ "$allow_failed" = "1" ] && return 0 || return 2
      fi
      if [ "$SOURCE" = "block" ] || [ "$SOURCE" = "recent" ]; then
        return 0
      fi
    fi
  done
  return 1
}

call_contract() {
  local address="$1" caller="$2" fn="$3" args_json="$4"
  BODY="$(jq -cn --arg address "$address" --arg caller "$caller" --arg fn "$fn" --argjson args "$args_json" '{address:$address, caller:$caller, fn:$fn, args:$args}')"
  curl_do POST "$API" "/contract/call" "$BODY" 30
  return $?
}

contract_tx() {
  local wallet_addr="$1" pk="$2" contract="$3" fn="$4" args_json="$5" value="$6" gas="$7" allow_failed="${8:-0}"
  BODY="$(jq -cn --arg address "$wallet_addr" --arg pk "$pk" --arg contract "$contract" --arg fn "$fn" --arg value "$value" --argjson args "$args_json" --argjson gas "$gas" '{address:$address, private_key:$pk, contract_address:$contract, function:$fn, args:$args, value:$value, gas:$gas, gas_price:0}')"
  curl_do POST "$WALLET" "/wallet/contract-template" "$BODY" 40
  if [ $? -ne 0 ]; then
    TX_HASH=""
    return 1
  fi
  TX_HASH="$(extract_hash "$HTTP_BODY")"
  [ -z "$TX_HASH" ] && return 3
  wait_for_tx "$TX_HASH" 100 "$allow_failed"
  return $?
}

deploy_builtin() {
  local wallet_addr="$1" pk="$2" template="$3" init_args_json="$4" gas="$5"
  BODY="$(jq -cn --arg template "$template" --arg owner "$wallet_addr" --arg pk "$pk" --argjson init_args "$init_args_json" --argjson gas "$gas" '{template:$template, owner:$owner, private_key:$pk, gas:$gas, init_args:$init_args}')"
  curl_do POST "$API" "/contract/deploy-builtin" "$BODY" 120
  if [ $? -ne 0 ]; then
    DEPLOY_ADDR=""
    DEPLOY_HASH=""
    return 1
  fi
  DEPLOY_ADDR="$(json_val '.address' "$HTTP_BODY")"
  DEPLOY_HASH="$(json_val '.tx_hash' "$HTTP_BODY")"
  [ -n "$DEPLOY_HASH" ] && wait_for_tx "$DEPLOY_HASH" 100 0 >/dev/null 2>&1
  [ -n "$DEPLOY_ADDR" ]
  return $?
}

cors_check "CORS API /wallet/new from explorer" "$API" "/wallet/new" "$EXPLORER" "POST"
cors_check "CORS wallet direct /wallet/send from DEX" "$WALLET" "/wallet/send" "$DEXUI" "POST"
cors_check "CORS API /contract/deploy-builtin from DEX" "$API" "/contract/deploy-builtin" "$DEXUI" "POST"
cors_check "CORS chain /contract/call from explorer" "$CHAIN" "/contract/call" "$EXPLORER" "POST"
cors_check "CORS dex-api /config from DEX" "$DEXAPI" "/config" "$DEXUI" "GET"

step_curl "Chain health" GET "$CHAIN" "/health" "" 30
step_curl "Aggregator health" GET "$API" "/health" "" 30
step_curl "Wallet direct health route" GET "$WALLET" "/health" "" 30
step_curl "DEX registry health" GET "$DEXAPI" "/health" "" 30
step_curl "Chain current DEX factory" GET "$CHAIN" "/dex/current" "" 30
DEX_CURRENT="$HTTP_BODY"
step_curl "DEX registry config" GET "$DEXAPI" "/config" "" 30
DEX_CONFIG="$HTTP_BODY"
CHAIN_FACTORY="$(json_val '.address' "$DEX_CURRENT")"
REG_FACTORY="$(json_val '.factory_address' "$DEX_CONFIG")"

OWNER_BODY="$(jq -cn --arg p "codex-owner-$TS" '{password:$p}')"
step_curl "Wallet create via API aggregator" POST "$API" "/wallet/new" "$OWNER_BODY" 30
OWNER_JSON="$HTTP_BODY"
OWNER_ADDR="$(json_val '.address' "$OWNER_JSON")"
OWNER_PK="$(json_val '.private_key' "$OWNER_JSON")"
OWNER_MNEMONIC="$(json_val '.mnemonic' "$OWNER_JSON")"

RECIP_BODY="$(jq -cn --arg p "codex-recipient-$TS" '{password:$p}')"
step_curl "Wallet create via wallet service" POST "$WALLET" "/wallet/new" "$RECIP_BODY" 30
RECIP_JSON="$HTTP_BODY"
RECIP_ADDR="$(json_val '.address' "$RECIP_JSON")"

if [ -n "$OWNER_PK" ]; then
  BODY="$(jq -cn --arg pk "$OWNER_PK" '{private_key:$pk}')"
  step_curl "Wallet import private key" POST "$WALLET" "/wallet/import/private-key" "$BODY" 30
fi

if [ -n "$OWNER_MNEMONIC" ]; then
  BODY="$(jq -cn --arg m "$OWNER_MNEMONIC" --arg p "codex-import-$TS" '{mnemonic:$m,password:$p}')"
  step_curl "Wallet import mnemonic via API" POST "$API" "/wallet/import/mnemonic" "$BODY" 30
fi

if [ -n "$OWNER_ADDR" ]; then
  BODY="$(jq -cn --arg a "$OWNER_ADDR" '{address:$a}')"
  step_curl "Faucet claim owner via API" POST "$API" "/faucet" "$BODY" 30
  step_curl "Owner balance after faucet via chain" GET "$CHAIN" "/balance?address=$OWNER_ADDR" "" 30
fi

if [ -n "$OWNER_ADDR" ] && [ -n "$RECIP_ADDR" ] && [ -n "$OWNER_PK" ]; then
  BODY="$(jq -cn --arg from "$OWNER_ADDR" --arg to "$RECIP_ADDR" --arg pk "$OWNER_PK" --arg data "codex-live-native-$TS" '{from:$from,to:$to,value:"100000000",data:$data,gas:21000,gas_price:1,private_key:$pk}')"
  curl_do POST "$WALLET" "/wallet/send" "$BODY" 40
  if [ $? -eq 0 ]; then
    NATIVE_HASH="$(extract_hash "$HTTP_BODY")"
    wait_for_tx "$NATIVE_HASH" 100 0
    W=$?
    [ $W -eq 0 ] && log_pass "Signed native LQD send wallet -> recipient" "hash=$NATIVE_HASH" || log_fail "Signed native LQD send wallet -> recipient" "wait code=$W hash=$NATIVE_HASH"
  else
    log_fail "Signed native LQD send wallet -> recipient" "$HTTP_CODE $(printf '%s' "$HTTP_BODY" | head -c 200)"
  fi
  step_curl "Recipient balance after native send" GET "$CHAIN" "/balance?address=$RECIP_ADDR" "" 30
fi

TOKEN_A=""
TOKEN_B=""
TOKEN_C=""
GENERIC=""
if [ -n "$OWNER_ADDR" ] && [ -n "$OWNER_PK" ]; then
  INIT_A="$(jq -cn --arg n "Codex Live A $TS" --arg s "CLA${TS: -4}" '[ $n, $s, "1000000000000000" ]')"
  deploy_builtin "$OWNER_ADDR" "$OWNER_PK" "lqd20" "$INIT_A" 900000
  [ $? -eq 0 ] && { TOKEN_A="$DEPLOY_ADDR"; log_pass "Deploy token A built-in LQD20" "addr=$TOKEN_A hash=$DEPLOY_HASH"; } || log_fail "Deploy token A built-in LQD20" "$HTTP_CODE $(printf '%s' "$HTTP_BODY" | head -c 200)"

  INIT_B="$(jq -cn --arg n "Codex Live B $TS" --arg s "CLB${TS: -4}" '[ $n, $s, "1000000000000000" ]')"
  deploy_builtin "$OWNER_ADDR" "$OWNER_PK" "lqd20" "$INIT_B" 900000
  [ $? -eq 0 ] && { TOKEN_B="$DEPLOY_ADDR"; log_pass "Deploy token B built-in LQD20" "addr=$TOKEN_B hash=$DEPLOY_HASH"; } || log_fail "Deploy token B built-in LQD20" "$HTTP_CODE $(printf '%s' "$HTTP_BODY" | head -c 200)"

  INIT_C="$(jq -cn --arg n "Codex Live C $TS" --arg s "CLC${TS: -4}" '[ $n, $s, "1000000000000000" ]')"
  deploy_builtin "$OWNER_ADDR" "$OWNER_PK" "lqd20" "$INIT_C" 900000
  [ $? -eq 0 ] && { TOKEN_C="$DEPLOY_ADDR"; log_pass "Deploy token C built-in LQD20" "addr=$TOKEN_C hash=$DEPLOY_HASH"; } || log_fail "Deploy token C built-in LQD20" "$HTTP_CODE $(printf '%s' "$HTTP_BODY" | head -c 200)"

  INIT_DAO="$(jq -cn --arg n "Codex DAO $TS" '[ $n ]')"
  deploy_builtin "$OWNER_ADDR" "$OWNER_PK" "dao_treasury" "$INIT_DAO" 700000
  [ $? -eq 0 ] && { GENERIC="$DEPLOY_ADDR"; log_pass "Deploy generic DAO treasury contract" "addr=$GENERIC hash=$DEPLOY_HASH"; } || log_fail "Deploy generic DAO treasury contract" "$HTTP_CODE $(printf '%s' "$HTTP_BODY" | head -c 200)"
fi

if [ -n "$TOKEN_A" ]; then
  META_OK=1
  call_contract "$TOKEN_A" "$OWNER_ADDR" "Name" '[]'; [ $? -eq 0 ] || META_OK=0
  call_contract "$TOKEN_A" "$OWNER_ADDR" "Symbol" '[]'; [ $? -eq 0 ] || META_OK=0
  call_contract "$TOKEN_A" "$OWNER_ADDR" "Decimals" '[]'; [ $? -eq 0 ] || META_OK=0
  call_contract "$TOKEN_A" "$OWNER_ADDR" "TotalSupply" '[]'; [ $? -eq 0 ] || META_OK=0
  [ $META_OK -eq 1 ] && log_pass "Token metadata Name/Symbol/Decimals" "ok" || log_fail "Token metadata Name/Symbol/Decimals" "metadata call failed"

  ARGS="$(jq -cn --arg to "$RECIP_ADDR" '[ $to, "1000000000" ]')"
  contract_tx "$OWNER_ADDR" "$OWNER_PK" "$TOKEN_A" "Transfer" "$ARGS" "0" 500000 0
  W=$?
  [ $W -eq 0 ] && log_pass "Signed token transfer owner -> recipient" "hash=$TX_HASH" || log_fail "Signed token transfer owner -> recipient" "code=$W hash=$TX_HASH http=$HTTP_CODE $(printf '%s' "$HTTP_BODY" | head -c 200)"

  ARGS="$(jq -cn --arg h "$RECIP_ADDR" '[ $h ]')"
  call_contract "$TOKEN_A" "$RECIP_ADDR" "BalanceOf" "$ARGS"
  [ $? -eq 0 ] && log_pass "Recipient token balance via contract call" "$(json_val '.output' "$HTTP_BODY")" || log_fail "Recipient token balance via contract call" "$HTTP_CODE $(printf '%s' "$HTTP_BODY" | head -c 200)"
  step_curl "Recipient token balance via wallet token-balance" GET "$WALLET" "/wallet/token-balance?contract=$TOKEN_A&holder=$RECIP_ADDR" "" 30
  step_curl "Explorer token holders list" GET "$CHAIN" "/token/$TOKEN_A/holders" "" 30
fi

if [ -n "$GENERIC" ]; then
  call_contract "$GENERIC" "$OWNER_ADDR" "Name" '[]'
  [ $? -eq 0 ] && log_pass "Generic contract read call" "$(json_val '.output' "$HTTP_BODY")" || log_fail "Generic contract read call" "$HTTP_CODE $(printf '%s' "$HTTP_BODY" | head -c 200)"
  step_curl "Contract ABI endpoint" GET "$API" "/contract/getAbi?address=$GENERIC" "" 30
  step_curl "Contract verification status endpoint" GET "$CHAIN" "/contract/verification?address=$GENERIC" "" 30
fi

FACTORY="$CHAIN_FACTORY"
[ -z "$FACTORY" ] && FACTORY="$REG_FACTORY"
PAIR_AB=""
PAIR_BC=""
QUOTE=""
if [ -n "$OWNER_ADDR" ] && [ -n "$TOKEN_A" ] && [ -n "$TOKEN_B" ] && [ -n "$TOKEN_C" ] && [ -n "$FACTORY" ]; then
  step_curl "DEX factory ABI endpoint" GET "$API" "/contract/getAbi?address=$FACTORY" "" 30

  ARGS="$(jq -cn --arg a "$TOKEN_A" --arg b "$TOKEN_B" '[ $a, $b ]')"
  contract_tx "$OWNER_ADDR" "$OWNER_PK" "$FACTORY" "CreatePair" "$ARGS" "0" 1200000 0
  W=$?
  [ $W -eq 0 ] && log_pass "DEX create pair A-B signed" "hash=$TX_HASH" || log_fail "DEX create pair A-B signed" "code=$W hash=$TX_HASH http=$HTTP_CODE $(printf '%s' "$HTTP_BODY" | head -c 200)"
  call_contract "$FACTORY" "$OWNER_ADDR" "GetPair" "$ARGS"
  [ $? -eq 0 ] && { PAIR_AB="$(json_val '.output' "$HTTP_BODY")"; log_pass "DEX GetPair A-B" "$PAIR_AB"; } || log_fail "DEX GetPair A-B" "$HTTP_CODE $(printf '%s' "$HTTP_BODY" | head -c 200)"

  ARGS="$(jq -cn --arg b "$TOKEN_B" --arg c "$TOKEN_C" '[ $b, $c ]')"
  contract_tx "$OWNER_ADDR" "$OWNER_PK" "$FACTORY" "CreatePair" "$ARGS" "0" 1200000 0
  W=$?
  [ $W -eq 0 ] && log_pass "DEX create pair B-C signed" "hash=$TX_HASH" || log_fail "DEX create pair B-C signed" "code=$W hash=$TX_HASH http=$HTTP_CODE $(printf '%s' "$HTTP_BODY" | head -c 200)"
  call_contract "$FACTORY" "$OWNER_ADDR" "GetPair" "$ARGS"
  [ $? -eq 0 ] && { PAIR_BC="$(json_val '.output' "$HTTP_BODY")"; log_pass "DEX GetPair B-C" "$PAIR_BC"; } || log_fail "DEX GetPair B-C" "$HTTP_CODE $(printf '%s' "$HTTP_BODY" | head -c 200)"
fi

if [ -n "$PAIR_AB" ] && [ -n "$PAIR_BC" ]; then
  AMT_A="100000000000"
  AMT_B="100000000000"
  AMT_C="100000000000"
  SWAP_IN="1000000000"

  ARGS="$(jq -cn --arg pair "$PAIR_AB" --arg amt "$AMT_A" '[ $pair, $amt ]')"
  contract_tx "$OWNER_ADDR" "$OWNER_PK" "$TOKEN_A" "Approve" "$ARGS" "0" 350000 0
  [ $? -eq 0 ] && log_pass "DEX approve token A -> pairAB" "hash=$TX_HASH" || log_fail "DEX approve token A -> pairAB" "code=$?"
  ARGS="$(jq -cn --arg pair "$PAIR_AB" --arg amt "$AMT_B" '[ $pair, $amt ]')"
  contract_tx "$OWNER_ADDR" "$OWNER_PK" "$TOKEN_B" "Approve" "$ARGS" "0" 350000 0
  [ $? -eq 0 ] && log_pass "DEX approve token B -> pairAB" "hash=$TX_HASH" || log_fail "DEX approve token B -> pairAB" "code=$?"
  ARGS="$(jq -cn --arg a "$TOKEN_A" --arg b "$TOKEN_B" --arg aa "$AMT_A" --arg bb "$AMT_B" '[ $a, $b, $aa, $bb ]')"
  contract_tx "$OWNER_ADDR" "$OWNER_PK" "$FACTORY" "AddLiquidity" "$ARGS" "0" 900000 0
  W=$?
  [ $W -eq 0 ] && log_pass "DEX add liquidity A-B signed" "hash=$TX_HASH" || log_fail "DEX add liquidity A-B signed" "code=$W hash=$TX_HASH http=$HTTP_CODE $(printf '%s' "$HTTP_BODY" | head -c 200)"
  call_contract "$PAIR_AB" "$OWNER_ADDR" "GetReserves" '[]'
  [ $? -eq 0 ] && { RES_AB="$(json_val '.output' "$HTTP_BODY")"; log_pass "DEX pairAB reserves after liquidity" "$RES_AB"; } || log_fail "DEX pairAB reserves after liquidity" "$HTTP_CODE $(printf '%s' "$HTTP_BODY" | head -c 200)"

  ARGS="$(jq -cn --arg pair "$PAIR_BC" --arg amt "$AMT_B" '[ $pair, $amt ]')"
  contract_tx "$OWNER_ADDR" "$OWNER_PK" "$TOKEN_B" "Approve" "$ARGS" "0" 350000 0
  [ $? -eq 0 ] && log_pass "DEX approve token B -> pairBC" "hash=$TX_HASH" || log_fail "DEX approve token B -> pairBC" "code=$?"
  ARGS="$(jq -cn --arg pair "$PAIR_BC" --arg amt "$AMT_C" '[ $pair, $amt ]')"
  contract_tx "$OWNER_ADDR" "$OWNER_PK" "$TOKEN_C" "Approve" "$ARGS" "0" 350000 0
  [ $? -eq 0 ] && log_pass "DEX approve token C -> pairBC" "hash=$TX_HASH" || log_fail "DEX approve token C -> pairBC" "code=$?"
  ARGS="$(jq -cn --arg b "$TOKEN_B" --arg c "$TOKEN_C" --arg bb "$AMT_B" --arg cc "$AMT_C" '[ $b, $c, $bb, $cc ]')"
  contract_tx "$OWNER_ADDR" "$OWNER_PK" "$FACTORY" "AddLiquidity" "$ARGS" "0" 900000 0
  W=$?
  [ $W -eq 0 ] && log_pass "DEX add liquidity B-C signed" "hash=$TX_HASH" || log_fail "DEX add liquidity B-C signed" "code=$W hash=$TX_HASH http=$HTTP_CODE $(printf '%s' "$HTTP_BODY" | head -c 200)"
  call_contract "$PAIR_BC" "$OWNER_ADDR" "GetReserves" '[]'
  [ $? -eq 0 ] && { RES_BC="$(json_val '.output' "$HTTP_BODY")"; log_pass "DEX pairBC reserves after liquidity" "$RES_BC"; } || log_fail "DEX pairBC reserves after liquidity" "$HTTP_CODE $(printf '%s' "$HTTP_BODY" | head -c 200)"

  ARGS="$(jq -cn --arg pair "$PAIR_AB" --arg amt "$SWAP_IN" '[ $pair, $amt ]')"
  contract_tx "$OWNER_ADDR" "$OWNER_PK" "$TOKEN_A" "Approve" "$ARGS" "0" 350000 0
  [ $? -eq 0 ] && log_pass "DEX approve token A -> pairAB for swap" "hash=$TX_HASH" || log_fail "DEX approve token A -> pairAB for swap" "code=$?"

  QARGS="$(jq -cn --arg a "$SWAP_IN" --arg tin "$TOKEN_A" --arg tout "$TOKEN_C" '[ $a, $tin, $tout ]')"
  call_contract "$FACTORY" "$OWNER_ADDR" "GetSwapQuote" "$QARGS"
  [ $? -eq 0 ] && { QUOTE="$(json_val '.output' "$HTTP_BODY")"; log_pass "DEX multi-hop quote A -> C" "$QUOTE"; } || log_fail "DEX multi-hop quote A -> C" "$HTTP_CODE $(printf '%s' "$HTTP_BODY" | head -c 200)"

  if [ -n "$QUOTE" ]; then
    Q_TYPE="$(printf '%s' "$QUOTE" | awk -F'|' '{print $1}')"
    Q_OUT="$(printf '%s' "$QUOTE" | awk -F'|' '{print $2}')"
    Q_HOP1="$(printf '%s' "$QUOTE" | awk -F'|' '{print $6}')"
    PARGS="$(jq -cn --arg a "$SWAP_IN" --arg tin "$TOKEN_A" '[ $a, $tin ]')"
    call_contract "$PAIR_AB" "$OWNER_ADDR" "GetQuote" "$PARGS"
    PQ1="$(json_val '.output' "$HTTP_BODY")"
    HOP1="$(printf '%s' "$PQ1" | awk -F',' '{print $1}')"
    PARGS="$(jq -cn --arg a "$HOP1" --arg tin "$TOKEN_B" '[ $a, $tin ]')"
    call_contract "$PAIR_BC" "$OWNER_ADDR" "GetQuote" "$PARGS"
    PQ2="$(json_val '.output' "$HTTP_BODY")"
    HOP2="$(printf '%s' "$PQ2" | awk -F',' '{print $1}')"
    [ "$Q_TYPE" = "2hop" ] && [ "$HOP1" = "$Q_HOP1" ] && [ "$HOP2" = "$Q_OUT" ] && log_pass "DEX quote math matches pair quotes" "hop1=$HOP1 hop2=$HOP2" || log_fail "DEX quote math matches pair quotes" "quote=$QUOTE pairQuote1=$PQ1 pairQuote2=$PQ2"

    call_contract "$PAIR_AB" "$OWNER_ADDR" "GetReserves" '[]'; BEFORE_AB="$(json_val '.output' "$HTTP_BODY")"
    call_contract "$PAIR_BC" "$OWNER_ADDR" "GetReserves" '[]'; BEFORE_BC="$(json_val '.output' "$HTTP_BODY")"
    HUGE_MIN="1000000000000000000000000000000"
    DEADLINE=$(( $(date +%s) + 3600 ))
    ARGS="$(jq -cn --arg amount "$SWAP_IN" --arg min "$HUGE_MIN" --arg tin "$TOKEN_A" --arg tout "$TOKEN_C" --arg dl "$DEADLINE" '[ $amount, $min, $tin, $tout, $dl ]')"
    contract_tx "$OWNER_ADDR" "$OWNER_PK" "$FACTORY" "SwapExactTokensForTokensWithDeadline" "$ARGS" "0" 1200000 1
    SLIP_CODE=$?
    sleep 3
    call_contract "$PAIR_AB" "$OWNER_ADDR" "GetReserves" '[]'; AFTER_AB="$(json_val '.output' "$HTTP_BODY")"
    call_contract "$PAIR_BC" "$OWNER_ADDR" "GetReserves" '[]'; AFTER_BC="$(json_val '.output' "$HTTP_BODY")"
    [ "$BEFORE_AB" = "$AFTER_AB" ] && [ "$BEFORE_BC" = "$AFTER_BC" ] && log_pass "DEX failed slippage protection rollback" "tx_code=$SLIP_CODE" || log_fail "DEX failed slippage protection rollback" "$BEFORE_AB/$BEFORE_BC -> $AFTER_AB/$AFTER_BC"

    call_contract "$PAIR_AB" "$OWNER_ADDR" "GetReserves" '[]'; BEFORE_AB="$(json_val '.output' "$HTTP_BODY")"
    call_contract "$PAIR_BC" "$OWNER_ADDR" "GetReserves" '[]'; BEFORE_BC="$(json_val '.output' "$HTTP_BODY")"
    ARGS="$(jq -cn --arg amount "$SWAP_IN" --arg min "1" --arg tin "$TOKEN_A" --arg tout "$TOKEN_C" '[ $amount, $min, $tin, $tout, "1" ]')"
    contract_tx "$OWNER_ADDR" "$OWNER_PK" "$FACTORY" "SwapExactTokensForTokensWithDeadline" "$ARGS" "0" 1200000 1
    DEAD_CODE=$?
    sleep 3
    call_contract "$PAIR_AB" "$OWNER_ADDR" "GetReserves" '[]'; AFTER_AB="$(json_val '.output' "$HTTP_BODY")"
    call_contract "$PAIR_BC" "$OWNER_ADDR" "GetReserves" '[]'; AFTER_BC="$(json_val '.output' "$HTTP_BODY")"
    [ "$BEFORE_AB" = "$AFTER_AB" ] && [ "$BEFORE_BC" = "$AFTER_BC" ] && log_pass "DEX failed deadline protection rollback" "tx_code=$DEAD_CODE" || log_fail "DEX failed deadline protection rollback" "$BEFORE_AB/$BEFORE_BC -> $AFTER_AB/$AFTER_BC"

    call_contract "$FACTORY" "$OWNER_ADDR" "GetSwapQuote" "$QARGS"
    GOOD_QUOTE="$(json_val '.output' "$HTTP_BODY")"
    GOOD_OUT="$(printf '%s' "$GOOD_QUOTE" | awk -F'|' '{print $2}')"
    MIN_OK="$(jq -nr --arg out "$GOOD_OUT" '(($out|tonumber)*0.995|floor|tostring)')"
    DEADLINE=$(( $(date +%s) + 3600 ))
    ARGS="$(jq -cn --arg amount "$SWAP_IN" --arg min "$MIN_OK" --arg tin "$TOKEN_A" --arg tout "$TOKEN_C" --arg dl "$DEADLINE" '[ $amount, $min, $tin, $tout, $dl ]')"
    call_contract "$PAIR_AB" "$OWNER_ADDR" "GetReserves" '[]'; BEFORE_AB="$(json_val '.output' "$HTTP_BODY")"
    call_contract "$PAIR_BC" "$OWNER_ADDR" "GetReserves" '[]'; BEFORE_BC="$(json_val '.output' "$HTTP_BODY")"
    contract_tx "$OWNER_ADDR" "$OWNER_PK" "$FACTORY" "SwapExactTokensForTokensWithDeadline" "$ARGS" "0" 1200000 0
    GOOD_CODE=$?
    call_contract "$PAIR_AB" "$OWNER_ADDR" "GetReserves" '[]'; AFTER_AB="$(json_val '.output' "$HTTP_BODY")"
    call_contract "$PAIR_BC" "$OWNER_ADDR" "GetReserves" '[]'; AFTER_BC="$(json_val '.output' "$HTTP_BODY")"
    [ $GOOD_CODE -eq 0 ] && { [ "$BEFORE_AB" != "$AFTER_AB" ] || [ "$BEFORE_BC" != "$AFTER_BC" ]; } && log_pass "DEX successful multi-hop swap A -> C signed" "hash=$TX_HASH before=$BEFORE_AB/$BEFORE_BC after=$AFTER_AB/$AFTER_BC" || log_fail "DEX successful multi-hop swap A -> C signed" "code=$GOOD_CODE before=$BEFORE_AB/$BEFORE_BC after=$AFTER_AB/$AFTER_BC"
  fi
fi

if [ -n "$OWNER_ADDR" ]; then
  step_curl "Explorer address overview owner" GET "$CHAIN" "/address/$OWNER_ADDR/overview" "" 30
  step_curl "Explorer address transactions owner" GET "$CHAIN" "/address/$OWNER_ADDR/transactions?page=1&page_size=20" "" 30
fi
step_curl "Explorer blocks fetch_last_n_block" GET "$CHAIN" "/fetch_last_n_block?n=5" "" 30
step_curl "Explorer transactions recent" GET "$CHAIN" "/transactions?page=1&size=20" "" 30
step_curl "Explorer pending transactions" GET "$CHAIN" "/transactions/pending" "" 30
step_curl "Explorer internal transactions" GET "$CHAIN" "/transactions/internal" "" 30
step_curl "Explorer rewards summary" GET "$CHAIN" "/rewards/summary" "" 30
step_curl "Explorer validators" GET "$CHAIN" "/validators" "" 30
step_curl "Explorer contract list" GET "$API" "/contract/list" "" 30
step_curl "DEX registry tokens" GET "$DEXAPI" "/tokens" "" 30
step_curl "DEX registry pools" GET "$DEXAPI" "/pools" "" 30
step_curl "Explorer UI root" GET "$EXPLORER" "/" "" 30
step_curl "DEX UI root" GET "$DEXUI" "/" "" 30

printf '\n=== LIVE_E2E_SUMMARY ===\n'
printf 'pass=%s fail=%s total=%s\n' "$PASS" "$FAIL" "$((PASS + FAIL))"
printf 'owner=%s recipient=%s\n' "$OWNER_ADDR" "$RECIP_ADDR"
printf 'tokens=%s,%s,%s generic=%s\n' "$TOKEN_A" "$TOKEN_B" "$TOKEN_C" "$GENERIC"
printf 'factory_chain=%s factory_registry=%s factory_used=%s\n' "$CHAIN_FACTORY" "$REG_FACTORY" "$FACTORY"
printf 'pairs=%s,%s\n' "$PAIR_AB" "$PAIR_BC"
printf 'quote=%s\n' "$QUOTE"
printf '\nFAILURES:%s\n' "$FAILURES"
printf '\nRESULTS:%s\n' "$RESULTS"
