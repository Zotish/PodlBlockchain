# PoDL Emergency Governance Constitution

Status: repository-enforced policy boundary, 2026-08-13. It is not a legal constitution.

## Normal governance

Protocol changes use signed `governance_action` transactions, an epoch validator-power snapshot, for/against/abstain accounting, quorum, approval threshold and an execution timelock. Delegation is prospective and cycle-bounded; it cannot rewrite an existing proposal snapshot. Execution is restricted to typed module/parameter actions and rolls back on any failed action.

## Emergency guardian boundary

The guardian is an expiring threshold multisig configured by governance. It may only:

- temporarily pause `bridge`, `vault` or `router`;
- veto a proposal while it is timelocked, with a public reason hash.

It cannot transfer assets, mint tokens, change contract code, alter validator power, change economic percentages, resolve slashing, or unpause a module. Recovery/unpause requires normal timelocked governance. Every signed operation is added to the governance audit trail.

Mainnet readiness requires at least three guardian members, threshold at least two, threshold no greater than membership, and an expiry later than the chain tip.

## Slashing adjudication boundary

Evidence opens a challengeable case with a bounded appeal window. An expiring slashing council is configured independently from the legacy guardian field. After the challenge window, signed `slash_decide` transactions require a case-specific decision and public reason hash. The configured council threshold must agree before a case is upheld or dismissed.

An upheld penalty debits native bond custody and classifies the recovered amount insurance-first. Slashing is never presented as recurring operating income or LP yield.

## Operational invariants

- emergency authority expires automatically;
- one signer cannot satisfy a production threshold;
- pause and veto approvals are action/proposal-specific;
- guardian recovery cannot bypass timelock;
- governance and slashing state are included in the deterministic state root and incoming-block replay;
- signer diversity is an external operational requirement, not something an address list alone can prove.
