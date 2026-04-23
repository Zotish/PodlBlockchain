# Mobile Wallet Architecture

This is the recommended architecture for a MetaMask-like PoDL mobile wallet.

## Suggested stack

- Expo React Native
- TypeScript
- React Navigation
- secure storage
- QR scanner
- deep linking
- background sync service

## Core layers

### 1. UI layer
- onboarding
- home
- send
- receive
- tokens
- activity
- dapps / approvals
- contracts
- bridge
- networks
- settings

### 2. State layer
- wallet session
- encrypted key material
- selected network
- active address
- token list
- pending approvals
- activity cache

### 3. Chain layer
- node RPC
- wallet server RPC
- aggregator RPC
- contract call API
- transaction submit API

### 4. Security layer
- secure key storage
- biometric re-auth
- auto-lock
- per-origin approval tracking
- backup encryption

## Suggested data model

### Wallet session
- address
- locked / unlocked
- selected network
- last active timestamp

### Token entry
- address
- name
- symbol
- decimals
- balance

### Approval entry
- request id
- origin
- method
- contract / function
- status

### Activity entry
- tx hash
- type
- from
- to
- contract
- value
- status
- timestamp

## API mapping

The mobile wallet should talk to the same backend APIs used by the extension:
- wallet create/import endpoints
- wallet send endpoint
- wallet contract-template endpoint
- chain balance and tx history endpoints
- contract ABI and storage endpoints
- bridge endpoints
- network management endpoints

## Sync strategy

- refresh native balance on app open
- refresh token balances on app focus
- poll activity every few seconds
- refresh pending approvals immediately
- refresh after every successful action

## Security recommendations

- store private keys only in secure storage
- never log private keys
- require biometrics or PIN for sensitive actions
- keep backup export encrypted
- support auto-lock on backgrounding

## Mobile wallet parity plan

Phase 1:
- wallet create/import/unlock
- send / receive
- native balance and token watchlist

Phase 2:
- token send
- DApp approvals
- networks
- activity

Phase 3:
- contracts
- bridge
- backup / restore

Phase 4:
- polished notifications
- biometrics
- deep links
- hardware-wallet integration if needed
