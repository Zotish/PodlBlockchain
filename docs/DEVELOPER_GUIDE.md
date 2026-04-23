# PoDL Developer Guide

This guide explains how to write smart contracts, compile them, deploy them, and test them on PoDL.

## 1) What a developer needs

### Required
- Go 1.21+
- Linux or macOS
- Access to the running chain node
- A wallet address and private key for deployment
- The repo checked out locally

### Recommended
- `swap-dex` and `blockchain-explorer` running locally
- the LQD wallet extension installed
- a fresh test wallet for experiments
- a unique token or contract namespace for each test

### Important platform note
Go plugin contracts compile as `.so` files. That means:
- use the same Go toolchain version as the node whenever possible
- prefer Linux/macOS for plugin development
- do not expect Go plugin contracts to be portable like plain source code

## 2) Contract types supported by PoDL

The current chain supports:
- Go plugin contracts (`.so`)
- Go-subset bytecode contracts
- a small DSL contract format
- builtin/native contracts deployed by the node

For most serious DApp work, use the Go plugin path.

## 3) How to write a Go plugin contract

### Minimum rules
- file must start with `package main`
- contract must export a global `var Contract = &YourType{}`
- methods must accept the execution context as the first parameter
- function names become callable contract methods
- storage is string-based, so encode numbers as decimal strings

### Example skeleton

```go
package main

import lqdctx "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/lqd-sdk-compat/context"

type MyToken struct{}

func (t *MyToken) Init(ctx *lqdctx.Context, name string, symbol string, supply string) {
    ctx.Set("name", name)
    ctx.Set("symbol", symbol)
    ctx.Set("totalSupply", supply)
    ctx.Set("balance:"+ctx.OwnerAddr, supply)
}

func (t *MyToken) Name(ctx *lqdctx.Context) {
    ctx.Set("output", ctx.Get("name"))
}

var Contract = &MyToken{}
```

### Context methods you can use

From the runtime, contracts can use:
- `ctx.Set(key, value)`
- `ctx.Get(key)`
- `ctx.Emit(eventName, data)`
- `ctx.Revert(reason)`
- `ctx.Commit()`
- `ctx.Call(target, method, args)`
- `ctx.DeployContract(newAddr, pluginPath)`
- `ctx.MsgValue()`
- `ctx.ReceiveNative(amount)`
- `ctx.SendNative(to, amount)`

### Contract patterns used in this repo

Check these as working examples:
- `contract/lqd20.go`
- `contract/dex_factory.go`
- `contract/dex_pair.go`
- `contract/dao_treasury.go`
- `contract/nft_collection.go`

They show:
- token init and transfers
- approvals and allowances
- AMM reserve accounting
- LP mint/burn
- cross-contract calls
- event emission

## 4) How to compile a contract

### Option A: Explorer UI
Use the smart contract compiler in the explorer:
- open `blockchain-explorer`
- go to the contracts studio/compiler
- paste Go source
- choose `Go Plugin (.so)`
- compile
- deploy with your wallet

The compiler expects:
- `package main`
- exported `var Contract`
- plugin-safe imports

### Option B: Node API
The chain exposes plugin compilation and deployment endpoints.

Common flow:
1. compile source
2. deploy compiled artifact
3. inspect ABI and storage

## 5) How to deploy a contract

Deployment requires:
- wallet address
- private key or extension wallet approval
- node endpoint
- compiled artifact

General deployment flow:
1. compile the contract
2. sign/deploy from wallet or extension
3. save the returned contract address
4. inspect ABI and storage
5. call contract methods through the explorer or DEX UI

## 6) How to test a contract

### Recommended testing checklist
- compile the contract locally
- deploy it on the test chain
- read `Name`, `Symbol`, `TotalSupply`, or equivalent getters
- perform a state-changing call
- verify updated storage through the explorer
- inspect emitted events

### Token contract checklist
For token contracts, verify:
- deploy
- `Name`
- `Symbol`
- `Decimals`
- `TotalSupply`
- `Transfer`
- `Approve`
- `Allowance`
- `TransferFrom`

### DEX contract checklist
For DEX contracts, verify:
- pair creation
- initial liquidity
- add liquidity to existing pool
- swap exact tokens
- remove liquidity
- LP lock / unlock for validation

## 7) Smart contract coding rules

### Use string storage
On-chain contract storage is string-based. Keep integers as decimal strings.

### Emit events
Emit events for:
- init
- transfer
- approve
- mint
- burn
- liquidity add/remove
- pair creation

### Revert early
Reject invalid inputs as early as possible using `ctx.Revert(...)`.

### Handle native LQD carefully
Use `"lqd"` as the native token sentinel in DEX contracts.

### Keep methods deterministic
Contracts should not depend on external APIs or random data.

### Avoid hidden side effects
Cross-contract calls should be intentional and minimal.

## 8) How approvals work in the DEX

For ERC-20 style tokens:
- user approves the pair contract or factory-approved target
- DEX then pulls tokens with `TransferFrom`

For native LQD:
- no approval is needed
- the transaction carries value directly

## 9) Common developer mistakes

### Missing `var Contract`
If the plugin does not export `var Contract`, deployment will fail.

### Wrong package name
Plugin contracts should be in `package main`.

### Wrong method signatures
Methods should use the runtime context as the first argument and string inputs after that.

### Not using decimal strings
Do not use floating-point math for token balances.

### Accidentally using pool address as factory
For DEX work:
- factory handles pair creation
- pair handles liquidity state

### Forgetting to compile with the right Go version
Go plugin `.so` files are sensitive to build environment mismatches.

## 10) Example developer flow

1. Write a token contract in Go.
2. Export `var Contract`.
3. Compile it through the explorer compiler.
4. Deploy it with a wallet.
5. Import its address into `swap-dex`.
6. Create a pair against the canonical factory.
7. Add liquidity.
8. Swap.
9. Inspect balances and storage in the explorer.

## 11) For frontend developers

If you are integrating with the UI:
- `swap-dex` is the trading and liquidity app
- `blockchain-explorer` is the read/write console for chain, wallet, and contracts
- `lqd-wallet-extension` is the approval/signing layer

Key endpoints:
- `GET /dex/current`
- `GET /contract/getAbi?address=0x...`
- `GET /contract/storage?address=0x...`
- `POST /contract/call`
- `POST /contract/deploy`
- `POST /contract/deploy-builtin`

## 12) Production advice

- keep one canonical DEX factory per network
- do not redeploy the shared factory for every user
- test all contracts on a reset chain before production deployment
- treat plugin contracts as powerful code that needs review and auditing

## 13) Best practice contract template

```go
package main

import lqdctx "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/lqd-sdk-compat/context"

type ContractName struct{}

func (c *ContractName) Init(ctx *lqdctx.Context, a string, b string) {
    ctx.Set("a", a)
    ctx.Set("b", b)
}

func (c *ContractName) GetA(ctx *lqdctx.Context) {
    ctx.Set("output", ctx.Get("a"))
}

var Contract = &ContractName{}
```

This is the safest starting point for new plugin contracts in PoDL.
