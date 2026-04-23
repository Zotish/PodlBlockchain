# Mobile Wallet Feature Parity

This document maps the browser extension wallet features into the mobile wallet.

## 1) Wallet Core

### Available in extension
- create wallet
- import by mnemonic
- import by private key
- unlock with password
- lock wallet
- export / backup
- reveal private key / mnemonic

### Mobile wallet target
- create wallet
- import by mnemonic
- import by private key
- unlock with PIN / password / biometrics
- lock automatically on background
- encrypted local backup
- recovery phrase view with confirmation

## 2) Native LQD

### Available in extension
- send LQD
- receive QR/address
- balance view
- transaction history

### Mobile wallet target
- send LQD
- receive by QR code
- balance dashboard
- transaction history
- pending / confirmed / failed status

## 3) Token Management

### Available in extension
- token watchlist
- import token by address
- show token metadata
- show token balance
- send token
- remove token from list
- auto-refresh token balances

### Mobile wallet target
- watchlist import by address
- token metadata fetch
- token send
- token receive display
- balance refresh
- token search and pinning

## 4) Network Management

### Available in extension
- view active network
- add network
- switch network
- remove custom network
- display node URL / wallet URL / explorer URL

### Mobile wallet target
- active network dashboard
- add custom network
- switch chain
- remove custom network
- show RPC / explorer endpoints
- support canonical PoDL network discovery

## 5) DApp Permissions and Approvals

### Available in extension
- connect / approve requests
- pending request queue
- allowlist management
- auto-lock and session restore

### Mobile wallet target
- DApp connect modal
- approval queue
- allowlist controls
- session timeout
- biometric re-auth for sensitive actions

## 6) Contracts

### Available in extension
- compile Go plugin
- deploy contract
- call contract
- inspect ABI
- inspect storage
- inspect events

### Mobile wallet target
- deploy from saved templates
- view contract detail
- read-only call forms
- write transaction forms
- ABI browser
- event timeline

## 7) Bridge

### Available in extension
- bridge lock flow
- bridge burn flow
- bridge request history
- token mapping display

### Mobile wallet target
- BSC lock request
- LQD burn request
- bridge status page
- request timeline
- mapping and claim tracking

## 8) UX Modes

### Compact mode
Equivalent to the extension popup:
- quick balance
- quick send
- token list
- pending approvals
- network switch

### Full mode
Equivalent to the extension fullpage app:
- wallet dashboard
- tokens
- activity
- contracts
- bridge
- settings
- network manager
- advanced tools

## 9) Missing in mobile-only form

These should be redesigned for touch UX:
- browser-tab-specific allowlist prompts
- injected web page permissions
- popup/extension background-worker session constraints

## 10) Recommended mobile screens

- Onboarding
- Create / Import Wallet
- Unlock Wallet
- Home
- Send
- Receive
- Tokens
- Activity
- Networks
- DApps / Approvals
- Contracts
- Bridge
- Settings

## 11) Feature parity verdict

The mobile wallet should aim to cover:
- 100% of the user-facing wallet features
- 100% of the token management features
- 100% of the network management features
- 100% of the bridge UI features
- a redesigned version of DApp approval and contract interaction flows

The only major difference is that browser-specific injection and tab-handling logic will be replaced by mobile deep links and in-app permissions.
