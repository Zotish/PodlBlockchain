# LQD Smart Contract Development Guide

Welcome to the **Proof of Dynamic Liquidity (PoDL)** blockchain smart contract development guide. This document explains how to write, compile, and deploy smart contracts on the LQD engine using Go-based plugins.

---

## 1. Overview
PoDL uses **Go Plugins** for high-performance smart contracts. Unlike EVM (Solidity), LQD contracts are written in Go and compiled into dynamic libraries (`.so` files) that run natively on the blockchain nodes.

### Key Benefits:
- **High Performance**: Native execution speed.
- **Full Go Power**: Use standard Go libraries (`math/big`, `strings`, `strconv`, etc.).
- **Atomic State**: Built-in state management with rollback on failure.

---

## 2. Prerequisites
- **Go Version**: 1.24.2 (recommended)
- **CGO**: Must be enabled (`CGO_ENABLED=1`) as plugins require CGO.
- **Wallet**: LQD Wallet Extension for Chrome/Brave.

---

## 3. Anatomy of a Contract

Every contract must follow this structure:

### A. Build Tags & Package
You must include `//go:build ignore` so that the main blockchain binary doesn't try to compile your contract as part of the core engine.
```go
//go:build ignore
// +build ignore

package main
```

### B. Imports
You must import the `BlockchainComponent` to access the execution context.
```go
import (
    "math/big"
    blockchaincomponent "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/BlockchainComponent"
)
```

### C. The Contract Struct
Define a struct that represents your contract. By convention, name it something descriptive.
```go
type MyContract struct{}
```

### D. Exported Methods
Any method starting with a **Capital Letter** is "Exported" and can be called by users or other contracts via the blockchain.
```go
func (c *MyContract) MyMethod(ctx *blockchaincomponent.Context, arg1 string) {
    // Logic here
}
```

---

## 4. The `Context` (`ctx`) Object
The `ctx` object is your interface to the blockchain.

| Method | Description |
| :--- | :--- |
| `ctx.Get(key)` | Read a string value from the contract's persistent storage. |
| `ctx.Set(key, val)` | Write a string value to storage. |
| `ctx.Emit(event, data)`| Trigger a blockchain event (visible to explorers/frontends). |
| `ctx.Call(addr, fn, args)`| Call another smart contract (Synchronous/Atomic). |
| `ctx.MsgValue()` | Get the amount of native LQD sent with the transaction. |
| `ctx.CallerAddr` | The address of the immediate caller. |
| `ctx.OriginAddr` | The address of the EOA (user) who started the transaction. |
| `ctx.Revert(reason)` | Stop execution and rollback all state changes. |

---

## 5. Step-by-Step: Writing a Token (LQD20)

### Step 1: Initialize Storage
Use an `Init` method to set up initial state (like `totalSupply` and `name`).
```go
func (c *LQDToken) Init(ctx *blockchaincomponent.Context, name, symbol, supply string) {
    if ctx.Get("initialized") == "true" {
        ctx.Revert("already initialized")
    }
    ctx.Set("name", name)
    ctx.Set("symbol", symbol)
    ctx.Set("totalSupply", supply)
    ctx.Set("bal:" + ctx.OwnerAddr, supply) // Give supply to owner
    ctx.Set("initialized", "true")
}
```

### Step 2: Implement Logic (Transfer)
Always validate balances before moving funds.
```go
func (c *LQDToken) Transfer(ctx *blockchaincomponent.Context, to string, amount string) {
    from := ctx.CallerAddr
    amt := parseBig(amount)
    
    fromBal := parseBig(ctx.Get("bal:" + from))
    if fromBal.Cmp(amt) < 0 {
        ctx.Revert("insufficient balance")
    }
    
    // Update balances
    ctx.Set("bal:" + from, new(big.Int).Sub(fromBal, amt).String())
    
    toBal := parseBig(ctx.Get("bal:" + to))
    ctx.Set("bal:" + to, new(big.Int).Add(toBal, amt).String())
    
    ctx.Emit("Transfer", map[string]interface{}{"from": from, "to": to, "value": amount})
}
```

---

## 6. Step-by-Step: Writing a DApp Hook (Interaction)
You can write contracts that interact with other contracts (like a Swap or a DAO).

```go
func (c *MyHook) SwapAndDonate(ctx *blockchaincomponent.Context, dexAddr, tokenAddr, amount string) {
    // 1. Call DEX to swap tokens
    _, err := ctx.Call(dexAddr, "Swap", []string{tokenAddr, amount})
    if err != nil {
        ctx.Revert("DEX call failed: " + err.Error())
    }
    
    // 2. Emit success event
    ctx.Emit("HookExecuted", map[string]interface{}{"user": ctx.OriginAddr})
}
```

---

## 7. Compilation & Deployment

### Using LQD Wallet Extension:
1. Open the **LQD Extension** and go to the **DApp Store** or **Developer Tab**.
2. Paste your Go code into the **Compiler** section.
3. Select **Type: Go Plugin**.
4. Click **Compile**.
5. Once compiled, click **Deploy**.
6. Sign the transaction. Your contract is now live!

---

## 8. Best Practices
1. **Always use `math/big`**: For any currency calculations, never use `float64`.
2. **Revert Early**: Check permissions and balances at the start of a function.
3. **Normalize Addresses**: Use `strings.ToLower(strings.TrimSpace(addr))` to ensure address comparisons match.
4. **Gas Awareness**: Complex loops consume gas. Ensure your logic is efficient.
5. **State Naming**: Use prefixes for storage keys to avoid collisions (e.g., `bal:0x...` for balances, `allow:0x...:0x...` for approvals).

---

## Happy Coding on PoDL! 🚀
