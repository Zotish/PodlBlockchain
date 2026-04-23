# Mobile Wallet Security Notes

This mobile wallet should be treated as a high-risk application because it handles signing keys and approvals.

## Must-haves

- encrypt private keys at rest
- use secure device storage
- require re-authentication for sensitive actions
- support biometric unlock
- auto-lock on inactivity
- never persist raw private keys in plain text

## Recommended controls

- separate session key from encrypted wallet key
- clear clipboard after sensitive copy events if possible
- show clear signing details before every approval
- verify contract address, function, and value before signing
- keep network switching explicit

## DApp approval safety

- show origin and request type
- show contract address for tx signing
- show function name and arguments when available
- require user confirmation for all signing actions

## Backup safety

- encrypt backups with a user password
- display a clear warning before export
- provide restore/import only after password verification

## Operational safety

- if a user loses their device, the app should be recoverable from seed phrase or private key import
- support app lock after background or app switch
- do not auto-connect to untrusted networks

## Future hardening ideas

- hardware wallet support
- transaction simulation before signing
- risk warnings for approvals and unlimited allowances
- phishing detection for fake contract addresses
