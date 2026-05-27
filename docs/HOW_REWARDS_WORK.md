# How Rewards Work

PODL separates rewards into clear categories so users can see exactly why a reward was paid.

## Reward Categories

- Validator reward: paid to the block validator.
- Validator participant reward: paid to validator participants that helped validation.
- Liquidity provider reward: paid to LPs that provide liquidity power.
- Transaction participant reward: paid when transaction-level reward logic applies.

## Where Rewards Are Visible

- `Reward Center`: overall reward analytics, latest reward split, LP APR/APY estimate, top LP reward accounts.
- `Block Rewards`: reward breakdown for a specific block.
- `Transaction Details`: reward-related transaction data.
- `LP Tracker`: total and pending LP rewards per provider.
- `Liquidity Dashboard`: provider-level stake, power, rewards, APR/APY, and claim/sync status.

## Reward Flow

1. A new block is produced.
2. The chain calculates block reward distribution.
3. Validator reward is assigned.
4. LP reward allocation is calculated from active liquidity providers.
5. Participant rewards are calculated when applicable.
6. Reward history stores the distribution.
7. Explorer displays the split by category.

## LP APR/APY

APR/APY shown in the explorer is an estimate. It is calculated from the current reward pace and active stake data. It is useful for understanding direction, but real yield changes when:

- block rewards change
- total liquidity changes
- LP participation changes
- pool activity changes
- validator participation changes

## Claiming Rewards

The explorer includes a `Claim / Sync` action. If the node exposes a manual claim endpoint, the button submits a claim. If the current node uses auto-claim logic, the dashboard refreshes reward status and shows that rewards are auto-synced by the chain.

## Why Pending Rewards May Stay Visible

Pending rewards can remain visible when they are tracked but not yet settled to the wallet balance. They may settle during reward processing, unstake flow, or a future manual claim endpoint depending on node configuration.
