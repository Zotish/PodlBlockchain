# Attachment Proposed-Improvements Closure

Date: 2026-08-13

Meaning of `repository complete`: the improvement has an executable implementation and is included in the local release gates. It does **not** mean audited, economically calibrated, publicly battle-tested or safe for real capital.

| # | Attachment feature | Repository completion | External evidence still required |
|---:|---|---|---|
| 1 | Custom Layer-1 | State version 4, independent canonical state implementation, production/reference root comparison during local proposal and incoming replay | Independently authored full node/client and third-party equivalence review |
| 2 | State transition | Full-state isolated replay/commit, TLA+ safety model, 4–100 bounded model checker | Official TLC log and formal-methods peer review; TLC JAR was unavailable locally |
| 3 | BFT finality | Signed strict `>2/3` prevote/precommit, 4–20 chaos, 20–100 process campaign | 30–60 day public multi-region campaign |
| 4 | View change | Signed timeout certificate, retained lock; 81 simulated WAN-loss/skew/latency view changes | Real WAN/clock/network evidence |
| 5 | Validator-set transition | Delayed activation/joint quorum; 81 full-set replacement campaigns for 20–100 validators | Real operator churn/exit drill |
| 6 | Proposer randomness | All-active-validator verifiable signature beacon; non-final subsets cannot bias election; prior-QC fallback preserves liveness | Independent cryptographic review; replace with standardized EC-VRF if review requires it |
| 7 | Native bond | Custody/unbond/slash plus configurable economic attack-cost estimator/gate | LQD market price/liquidity and reviewed security-budget target |
| 8 | Liquidity credit | CSV backtest loader, weight optimizer and adversarial-capital/oracle/fee model | Trustworthy historical/live market dataset and economic calibration |
| 9 | Liquidity quality | Address clustering, unique actors, circular wash detection and fee-paid flow cost | Identity/market data and adversarial field evidence |
| 10 | Slashing | Bounded appeal plus separate expiring threshold council and signed case decisions | Diverse organizations and governance drill |
| 11 | Test liquidity/faucet | Capped token; persistent address/IP/budget state; hashed IPs; corrupt/write failure fail-closed; Prometheus counters | Public endpoint, real abuse traffic and operational tuning |
| 12 | Oracle | Primary/fallback quorums kept separate, confidence/deviation/staleness/nonce/signature enforcement | 3–7 organizationally independent publishers, feeds and SLA history |
| 13 | Rolling TWAP | Low-depth/flash-manipulation cost tests and circuit breaking | Live venue liquidity/calibration |
| 14 | Dynamic routing | Deterministic amount-aware routing and replayable quality/backtest inputs | Real order history and measured live slippage |
| 15 | Router | Candidate cap/pruning, topology TTL cache, requote and gas-aware scoring, bounded 1–8 hops | Production gas/slippage observations |
| 16 | MEV | Commit/reveal execution plus deterministic order-independent uniform batch clearing engine | Public keeper/order-flow deployment and adversarial MEV measurements |
| 17 | Constant-product AMM | Atomic custody, dynamic fee, invariant tests/fuzz | Independent invariant/economic/security audit and real depth |
| 18 | Stable AMM | Amplified curve, oracle depeg policy, emergency fee/swap cap and depeg tests | Asset-specific depeg calibration/audit |
| 19 | Concentrated liquidity | Tick/property fuzz; transferable position ID lifecycle; SDK mint/read/transfer/collect/burn helpers | Independent accounting audit and real-user UX testing |
| 20 | Strategy vault | Physical LP move, multi-source valuation and bonded keeper assignment/fallback/slashing | Custody audit and multiple independent keeper operators |
| 21 | ERC-4626 | Correct deposit/mint/withdraw/redeem returns, previews/max methods, donation defense and E2E conformance | Independent ERC-4626 integration/audit |
| 22 | NAV accounting | Three-source quorum, confidence/deviation/freshness filters and settlement delay | Independent live valuation feeds |
| 23 | Withdrawal queue | FIFO receipts/cancel/buffer/emergency exit; immutable jobs and permissionless keeper fallback; 10,000-user model | Public funded withdrawal surge drill |
| 24 | Original capital | UI prominently states IL/loss and no principal guarantee | Legal/compliance review and real-user comprehension study |
| 25 | Protocol arbitrage | Sealed bids, opportunity binding, keeper bonds, deadlines, fallback promotion and insurance-first missed-duty slash | Competitive keepers/private execution and live opportunity history |
| 26 | Arb concurrency | Sorted per-market locks plus short serialized custody/revenue commit and atomic storage batch | Production contention/latency measurements |
| 27 | Trading revenue | Custody-backed fee reconciliation and source ledger | Market makers, organic volume and revenue history |
| 28 | B2B liquidity | Governed protocol-capital agreements and idempotent invoices | Customers, signed agreements, capital and pricing evidence |
| 29 | Bridge revenue | Existing-request-bound, custody-backed, idempotent capture | Audited production bridge and actual fees |
| 30 | Slashing revenue | 100% insurance-first classification | Funded reserve; slashing must not be marketed as recurring revenue |
| 31 | Revenue waterfall | Deterministic 1–10 year seeded scenario engine, 5,000-path stress script and governed parameters | Reviewed assumptions and governance calibration |
| 32 | Insurance reserve | Native custody vault, pending-claim reservation, cap/floor, challenge, payout, liabilities and coverage ratio E2E | Real seeded capital, claims policy/legal review and audit |
| 33 | Token emissions | Issuance cap plus 5–10 year supply/revenue-replacement simulation | Demand/revenue history and economic review |
| 34 | Buyback/burn | Insurance-floor guard, custody debit, burn address and public ledger receipt | Audited real surplus and governance authorization |
| 35 | Governance | Signed lifecycle, delegation, typed rollback, threshold veto and constitution | Turnout/distribution history and public drill |
| 36 | Guardian | Expiring threshold pause/veto only, audit log and governance-only recovery | Diverse signers and expiry/incident drill |
| 37 | Bridge | Enforced threshold mode or weighted light-client header/checkpoint/Merkle proof mode, caps/replay | Chain-specific trusted bootstrap/deployment, independent signers or audited source-chain proof adapters |
| 38 | SDK | Versioned zero-dependency JS client, typings, tests, examples and concentrated-position lifecycle | npm credentials/publication, adopters and maintenance history |
| 39 | Explorer | Persistent LevelDB block/tx/address index, reorg rebuild, lag metrics/search, Vite UI | Hosted SLA and production history |
| 40 | Swap UI | Simulation/route/min-out flow and prominent slippage/IL/loss warnings | Mobile device/accessibility/user testing |
| 41 | Investor dashboard | Zero-filled historical revenue, index lag, blockers and validator-signed evidence checkpoint | Actual history/users and audited financial reports |
| 42 | Monitoring | Latency SLO, persistent failure streak, webhook, bounded restart, index monitoring, on-call runbook/readiness gate | Real pager receiver, named rotation, failover and drill evidence |
| 43 | Testing | Unit/race/vet/fuzz/E2E, 4–100 consensus, bank-run/manipulation/upgrade/economic campaigns | External audit, bug bounty and public testnet evidence |
| 44 | Contract runtime | Hardened deterministic VM validation/metering/step limit/fail-closed ops plus fuzz; plugin ABI mismatch fail-closed | Independent sandbox/runtime audit; Go-plugin portability remains a deployment constraint |
| 45 | Compliance | Product/UX risk disclosures and protocol-capital pilot boundary | Entity, jurisdiction, token/yield/bridge/KYC/AML legal opinions |
| 46 | Business model | Isolated custody-backed lending market plus trading/vault/B2B/bridge revenue plumbing | Borrowers, customers, retention, capital and realized revenue |

## Local evidence snapshot

- Go test suite: pass.
- Go vet: pass.
- Go race suite: pass; the OS-process signer campaign runs separately because it has no shared-memory race surface.
- 20–100 process fault campaign: 81 configurations, 200 persistent signer processes, 14,742 signed votes, 162 vote QCs, 81 timeout certificates, 81 full-set churn QCs, 964 simulated first-attempt drops recovered, all 81 Byzantine minorities and old-only transition quorums rejected.
- Fuzz: AMM, bridge replay, stable AMM, concentrated positions and hardened VM.
- Contract E2E: four-hop exact-output routing, stable/concentrated pools, ERC-4626/donation defense, multi-source keeper vault, insurance claim custody, validator slashing and isolated lending.
- Frontends: Explorer 83 tests, Swap 21 tests, Explorer/Swap/Bridge Admin production builds.
- SDK: four tests and package dry-run.

The authoritative command is `./scripts/validate_table1.sh`. It validates repository-controlled behavior only and cannot produce the external evidence in the last column.
