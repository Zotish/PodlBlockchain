#!/bin/sh
set -eu

CACHE_ROOT="${LQD_GO_CACHE_ROOT:-$PWD/.lqd-go-cache}"
export GOCACHE="${GOCACHE:-$CACHE_ROOT/build}"
export GOMODCACHE="${GOMODCACHE:-$CACHE_ROOT/mod}"
mkdir -p "$GOCACHE" "$GOMODCACHE"

go run scripts/railway/precompile_builtins.go
go test ./...
go vet ./...
go test -race -timeout 5m ./BlockchainComponent ./BlockchainServer ./WalletServer
go test ./BlockchainComponent -run 'TestBFTChaosSafetyAndLivenessFourToTwentyNodes|TestJointQuorumRejectsLargeValidatorChurnPartition|TestIncomingBlockReplay|TestLocalProposalDoesNotCommitSpeculativeStateBeforeQC'
if [ -n "${TLA2TOOLS_JAR:-}" ] && [ -r "$TLA2TOOLS_JAR" ]; then
	java -cp "$TLA2TOOLS_JAR" tlc2.TLC -config formal/PODLBFT.cfg formal/PODLBFT.tla
else
	echo 'TLC_MODEL_CHECK=SKIP (external TLA2TOOLS_JAR unavailable; bounded 4-100 Go model remains mandatory)'
fi
go test ./BlockchainComponent -run '^TestBFTFourToTwentyIndependentSignerProcesses$' -count=1
go build -o "$CACHE_ROOT/multiprocess_consensus" ./scripts/table1/multiprocess_consensus
"$CACHE_ROOT/multiprocess_consensus"
go run ./scripts/table1/network_fault_lab
go test ./BlockchainComponent -run '^$' -fuzz FuzzAMMOutputPreservesConstantProduct -fuzztime 3s
go test ./BlockchainComponent -run '^$' -fuzz FuzzBridgeReplaySnapshotRejectsTamper -fuzztime 3s
go test ./BlockchainComponent -run '^$' -fuzz FuzzStableSwapInvariant -fuzztime 3s
go test ./BlockchainComponent -run '^$' -fuzz FuzzConcentratedSwapAtomicAndNonNegative -fuzztime 3s
go test ./BlockchainComponent -run '^$' -fuzz FuzzHardenedVMCompileAndExecuteIsBounded -fuzztime 3s
# Build E2E runners with the same Go build cache as plugins. Go's plugin ABI
# requires an identical package build identity; direct go run can select a
# stale cached runner and correctly fail closed as incompatible.
go build -o "$CACHE_ROOT/advanced_pool_e2e" ./scripts/table1/advanced_pool_e2e
"$CACHE_ROOT/advanced_pool_e2e"
go build -o "$CACHE_ROOT/lending_e2e" ./scripts/table1/lending_e2e
"$CACHE_ROOT/lending_e2e"
go run ./scripts/table1/liquidity_sim
go run ./scripts/table1/bank_run_sim
go run ./scripts/table1/upgrade_drill
go run ./scripts/table1/gas_calibration
go run ./scripts/table1/economics_sim
node --check sdk/javascript/src/index.js
(cd sdk/javascript && npm test && npm_config_cache=/tmp/podl-npm-cache npm pack --dry-run)
if [ -x blockchain-explorer/node_modules/.bin/vite ]; then
	(cd blockchain-explorer && npm run test && npm run build)
else
	echo 'EXPLORER_BUILD=SKIP (dependencies absent; run npm install in blockchain-explorer to enable)'
fi
if [ -x swap-dex/node_modules/.bin/vite ]; then
	(cd swap-dex && npm run test && npm run build)
else
	echo 'SWAP_DEX_BUILD=SKIP (dependencies absent; run npm install in swap-dex to enable)'
fi
(cd bridge-admin-ui && npm run build)
sh -n scripts/ops/podl_monitor.sh

echo 'TABLE1_INTERNAL_VALIDATION=PASS'
