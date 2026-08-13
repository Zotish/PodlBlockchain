# PoDL v2 Deterministic State-Transition Specification

Status: executable repository specification, 2026-08-13. This is not an independent audit.

## Block pre-state and header

For candidate `B(h)`, every node starts from finalized state after `B(h-1)`. The header commits protocol version, height, previous hash, parent state root, block timestamp, gas policy, transaction list, reward breakdown, post-state root, and a deterministic weighted proposer certificate for `(chain spec, validator set, height, round)`. P2P peers must match chain/network IDs, genesis, protocol/state versions, and spec hash.

## Transaction validity

Transactions execute in block order and pass address, chain, timestamp, nonce, gas/fee, signature, and balance checks. Oracle/governance transactions use the domain `PODL-CONTROL-TX-V2`; their signature commits sender, recipient, value, gas, gas price, chain ID, timestamp, nonce, type, and complete extra data. Exactly one zero-address reward transaction must pay the elected proposer the deterministic reward.

## Isolated execution

Incoming blocks execute against a deep-copied full consensus state plus a copy-on-write contract-storage overlay. Block timestamp is the time source. Verification returns a staged `BlockReplayTransition`; finalization installs that exact verified snapshot rather than recomputing or copying a smaller subset.

For each user transaction: calculate gas with overflow checks; atomically execute a contract or typed control action; debit value plus fee; credit a non-contract recipient (contract value is handled by the atomic pipeline); reject duplicate sender nonces or failed transitions. After user transactions, recompute rewards, run any scheduled DLE epoch, and reconcile trusted built-in DEX custody counters. Non-LQD fees remain asset-denominated until governed realization.

## Canonical state root

SHA-256 is calculated over canonical JSON with sorted map-derived collections. State version 4 commits:

- accounts; validator bonds, power, penalties and jail state;
- base fee, minimum stake/liquidity stake, reward parameters and slashing pool;
- liquidity locks/totals, fee pools, pool/unallocated balances and complete provider unstake state;
- strategy-vault positions, token identities, movement log and safety policy;
- dynamic oracle signals, rolling price history, observations, publishers and every nonce, including retired-source nonces;
- all deployed contract storage, pair risk, congestion and consensus parameters;
- economic policy/buckets/revenue/checkpoints/assets/burn/emission, arb auctions/keeper bonds/unbond heights;
- governance proposals/delegation/guardian/council/audit state and pauses;
- bridge proof/replay/cap state, requests and token map; B2B agreements and treasury deployments.

Contract-storage read or serialization failure returns an empty root and therefore fails verification. Ephemeral vote pools, peers, process locks, caches and wall-clock process state are excluded.

`reference_state.go` is a mechanically independent canonicalizer with separate types and serialization. Tests populate every extended state category, compare its root with the production canonicalizer and verify that each mutation changes the root.

The replay root must equal `B.StateRoot`; otherwise the overlay is discarded.

## Finality and commit

Signed prevote/precommit certificates require strictly more than two thirds of active weighted power. During set transition, old and new sets must independently pass quorum. Finalization installs the staged state, rechecks the root, flushes the contract overlay, fsyncs the block, then advances memory and prunes included/pending transactions.

View change requires a signed timeout certificate and retains the lock. Conflicting signed votes open a challengeable slashing case; after the bounded appeal period an independently configured threshold council submits signed case-specific decisions and reason hashes.

## Safety invariants

- Forged parent/post root, reward, signature, or contract transition cannot finalize.
- Byzantine power below one third cannot alone make a QC or timeout certificate.
- Old-only/new-only votes cannot bypass joint quorum.
- Failed typed governance/contract actions roll back completely.
- Routing weights never fabricate reserves; only vault custody moves assets.
- DEX/vault balances use integer arithmetic and conservative rounding.
- Withdrawals are pro-rata and never promise original fiat principal.
- Bridge events are consumed once; restore cannot regress consumed state.

## Upgrade procedure

Change protocol/state version and spec hash; schedule epoch-boundary activation; snapshot durable DB; rehearse old/new handshake rejection; migrate; verify the root. Rollback uses the pre-activation snapshot. See `scripts/table1/upgrade_drill`.

## Formal-model boundary

`formal/PODLBFT.tla` models honest non-equivocation, locks, strict quorum, decisions, the `<1/3` fault bound and agreement. `CheckBoundedBFTModel` exhaustively explores equal-power 4–100-validator honest A/B/abstain distributions with Byzantine double voting and old/new joint-quorum combinations. The repository test passes more than 100,000 states.

The TLA+ source and configuration are present, but the official TLC JAR is an external tool dependency and was not available in the local environment on 2026-08-13. A published model-check log, proof review, independently authored client, external audit and public multi-operator fault campaign remain external mainnet evidence gates.
