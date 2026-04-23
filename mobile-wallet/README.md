# LQD Mobile Wallet

This folder now contains a real Expo/React Native mobile wallet scaffold for the PoDL ecosystem.

It is designed to behave like a MetaMask-style mobile wallet, not the browser-explorer web wallet.

## What is implemented

- Create wallet
- Import wallet by mnemonic
- Import wallet by private key
- Password-locked vault using encrypted local storage
- Unlock / lock session
- Biometric unlock on supported devices
- Native LQD send
- Receive address and copy-to-clipboard
- QR receive display
- QR scanner for addresses and deep links
- Token watchlist and token import
- Token send
- Network management and switching
- Bottom navigation with icon labels
- Builtin contract deploy
- Custom Go plugin compile + deploy flow
- Generic contract call
- Contract ABI / storage inspection
- Bridge lock / burn actions
- DApp connect / approvals inbox
- Activity feed
- Backup export / restore

## Stack

- Expo React Native
- JavaScript
- `expo-secure-store`
- `expo-file-system`
- `expo-clipboard`
- `crypto-js`

## Run

From this folder:

```bash
npm install
npm start
```

Then open the project in Expo Go, Android emulator, or iOS simulator.

## Build an installable app

To generate a real installable build:

```bash
npm install -g eas-cli
eas login
eas build:configure
eas build -p android --profile preview
eas build -p ios --profile preview
```

For production releases:

```bash
eas build -p android --profile production
eas build -p ios --profile production
```

## Backend expectation

The app talks to the same local services used by the extension and DEX:

- Blockchain node: `http://127.0.0.1:6500`
- Wallet server: `http://127.0.0.1:8080`
- Aggregator: `http://127.0.0.1:9000`
- Explorer: `http://localhost:3001`

## Notes

- The wallet vault is encrypted locally with your password.
- The mobile app loads the canonical DEX factory from the chain when available.
- Builtin templates can be deployed directly from the app.
- Custom plugin deploy is supported through compile + upload.
