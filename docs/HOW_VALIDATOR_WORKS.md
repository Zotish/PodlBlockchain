# How Validator Works

Validators produce blocks, secure the network, and receive rewards for successful validation.

## Validator Requirements

- A valid LQD address.
- Minimum stake configured by the chain.
- Node online and synced.
- Validator process running with mining enabled.

## Validator Lifecycle

1. Validator address is registered.
2. Validator stake is checked.
3. Node joins the network.
4. Validator selection runs for each block.
5. Selected validator produces or finalizes the block.
6. Reward breakdown records validator reward and participant reward.

## Validator Selection Signals

Validator selection may consider:

- validator stake
- liquidity power
- network participation
- local chain rules
- validator availability

## Where to Check Validator Data

- `Validators`: validator list and liquidity power.
- `Validator Details`: single validator status.
- `Block Rewards`: selected validator and reward amount.
- `Reward Center`: latest validator reward analytics.
- `Health`: node status and block height.

## Best Practice for Operators

- Keep the node online.
- Monitor `/health`.
- Monitor block height.
- Keep validator key safe.
- Do not reuse production private keys in public demos.
- Keep persistent chain data mounted.

## Common Issues

- Validator not selected: stake, liquidity power, or node state may be too low.
- Rewards not visible: wait for a finalized block and check block reward breakdown.
- Chain restarts from old state: verify persistent data path and volume mount.
