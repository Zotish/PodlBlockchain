# How Bridge Works

The bridge moves value between LQD and external chains through lock, burn, mint, and release flows.

## Bridge Concepts

- Source chain: where the user starts the bridge.
- Target chain: where the user receives assets.
- Lock contract: holds source assets.
- Vault contract: controls locked liquidity.
- Bridged token: representation of the asset on the target chain.
- Relayer: watches bridge requests and finalizes them.
- Token mapping: links source token and target token.
- Chain registry: stores each supported chain configuration.

## LQD to External Chain

1. User selects source `LQD`.
2. User selects target chain.
3. User enters recipient address.
4. LQD is locked or burned on the source side.
5. Bridge request is created.
6. Relayer verifies the request.
7. Target chain mint or release is executed.
8. User receives bridged token or released token.

## External Chain to LQD

1. User locks or burns the external token.
2. External transaction proof is submitted.
3. Relayer verifies source transaction.
4. LQD-side bridge request is finalized.
5. User receives mapped LQD asset.

## What Must Be Configured Per Chain

- RPC URL.
- Chain ID or native chain identifier.
- Explorer URL.
- Bridge/lock/vault contract address.
- Relayer wallet address.
- Token mapping.
- Liquidity source.
- Confirmation depth.
- Fee settings.

## Bridge Status Values

- Queued: request was created.
- Pending: relayer has not finalized yet.
- Completed: target release/mint is done.
- Failed: verification or execution failed.

## Safety Notes

- Never bridge to an address format that does not belong to the target chain.
- Use testnet first.
- Confirm token mapping before sending.
- Relayer wallet must have gas on every target chain.
- Production bridge needs replay protection, nonce checks, confirmation rules, and contract audit.
