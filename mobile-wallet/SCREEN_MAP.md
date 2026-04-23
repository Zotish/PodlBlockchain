# Mobile Wallet Screen Map

This map converts the current extension wallet UI into a mobile app UI.

## Compact flow

Equivalent to extension popup:
- splash / unlock
- wallet home
- token list
- send modal
- receive modal
- approvals
- network picker
- activity list

## Full flow

Equivalent to extension fullpage:
- onboarding
- create wallet
- import wallet
- unlock / lock
- dashboard
- send LQD
- send token
- receive
- token watchlist
- activity feed
- contract studio
- contract deploy
- contract call
- contract storage
- bridge panel
- network manager
- settings

## Screen breakdown

### 1. Onboarding
- create wallet
- import mnemonic
- import private key
- restore backup

### 2. Unlock
- password / PIN input
- biometric unlock
- auto-lock notice

### 3. Home
- wallet address
- LQD balance
- token cards
- recent transactions
- network badge

### 4. Send
- native send
- token send
- QR scan support
- max button
- fee preview

### 5. Tokens
- watchlist
- add token
- remove token
- refresh balances
- send token action

### 6. Approvals
- pending dApp requests
- contract interaction approval
- allow / reject
- remember origin

### 7. Networks
- active network
- switch network
- add custom network
- remove custom network

### 8. Contracts
- deploy contract
- compile source
- inspect ABI
- read storage
- write function call

### 9. Bridge
- BSC lock flow
- LQD burn flow
- history
- token mapping

### 10. Settings
- endpoints
- backup / export
- reveal secret
- lock wallet
- security controls

## Navigation suggestion

Bottom tabs:
- Home
- Tokens
- Activity
- DApps
- Settings

Advanced screens:
- Contracts
- Bridge
- Networks
