// Package context provides the smart contract execution context for LQD Plugin VM.
// This compat stub re-exports blockchaincomponent.Context as a type alias so that
// plugin contracts compiled against lqd-sdk/context are type-compatible with the
// Plugin VM (which passes *blockchaincomponent.Context via reflection).
package context

import blockchaincomponent "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/BlockchainComponent"

// Context is a type alias for blockchaincomponent.Context.
// Using = (alias) instead of a new type means *context.Context and
// *blockchaincomponent.Context are the SAME type at runtime, eliminating the
// "reflect: Call using *blockchaincomponent.Context as type *context.Context" panic.
type Context = blockchaincomponent.Context

// Event is a type alias for blockchaincomponent.ContractEvent.
type Event = blockchaincomponent.ContractEvent
