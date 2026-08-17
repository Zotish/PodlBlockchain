# Validator signer and RFC 9381 ECVRF

Date: 2026-08-17

This closes repository coding gaps 1 and 3: validator keys no longer need to be retained by the chain process, and proposer randomness uses the standardized `ECVRF-P256-SHA256-TAI` ciphersuite from RFC 9381. It is engineering closure, not an HSM vendor certification or an external security audit.

## Security model

The chain talks only to the `ValidatorSigner` interface. The signer supports:

- consensus prevote/precommit and timeout signatures;
- P2P validator handshakes and signed investor evidence;
- RFC 9381 P-256 VRF proofs, bound to the validator's secp256k1 identity signature;
- durable write-before-sign slashing protection;
- a local encrypted key-file fallback;
- a TLS 1.3 mutual-TLS remote signer;
- a PKCS#11 secp256k1 identity-key backend with pinned public-key verification.

The remote client verifies every returned identity signature and VRF proof locally. An unavailable, unhealthy, wrong-address or wrong-suite signer fails closed. The signer service accepts only allowlisted, domain-separated payloads, rejects stale/replayed request envelopes and serializes HSM access.

## Canonical VRF flow

1. The current weighted proposer executes block `H` and obtains its state root.
2. Its signer proves a seed bound to the chain spec, block height, parent hash, state root and target height `H+1`.
3. The ECVRF proof, output, P-256 public key and validator identity-key binding are placed in block `H` before its hash is calculated.
4. Receivers verify the block proposer, seed, identity binding and RFC 9381 proof.
5. The canonical proof output becomes proposer entropy for height `H+1`; the consensus round still changes the final selection seed.
6. If a compatibility block has no canonical proof, proposer selection uses prior signed-QC randomness. Signed-only production mode rejects incoming blocks without the proof.

This avoids using an arrival-order-dependent off-chain proof subset for consensus. The older all-contributor beacon remains diagnostic/experimental state but cannot change canonical proposer selection.

## Durable slashing protection

Before accessing a key, the signer atomically persists `(domain slot, message digest)` with file mode `0600`, file `fsync`, atomic rename and directory `fsync`. Repeating the same digest is idempotent; a different digest for the same vote, timeout or VRF slot is rejected before signing. A corrupt or unsupported database fails startup.

## Encrypted fallback key

Use an environment variable so the raw keys do not appear in the command arguments:

```bash
export LQD_VALIDATOR_PRIVATE_KEY='<64-hex-secp256k1-key>'
export LQD_VALIDATOR_VRF_PRIVATE_KEY='<64-hex-P256-scalar>'
export LQD_VALIDATOR_KEY_PASSPHRASE='<strong-passphrase>'
./bin/lqd signer -create_key_file /secure/validator-key.json
unset LQD_VALIDATOR_PRIVATE_KEY LQD_VALIDATOR_VRF_PRIVATE_KEY
```

The file uses scrypt plus AES-256-GCM and mode `0600`. This is a fallback for testnet/small operators; production should prefer a separately isolated remote signer and HSM identity key.

## Remote signer

The server requires a TLS certificate, key, trusted client CA and persistent slashing database. With an encrypted signer key:

```bash
export LQD_VALIDATOR_KEY_FILE=/secure/validator-key.json
export LQD_VALIDATOR_KEY_PASSPHRASE='<strong-passphrase>'
export LQD_VALIDATOR_SLASHING_DB=/secure/slashing-protection.json
export LQD_SIGNER_TLS_CERT=/secure/signer.crt
export LQD_SIGNER_TLS_KEY=/secure/signer.key
export LQD_SIGNER_CLIENT_CA=/secure/client-ca.crt
./scripts/railway/start-signer.sh
```

Configure the chain process with `LQD_VALIDATOR_SIGNER_URL`, `LQD_VALIDATOR_SIGNER_CA`, `LQD_VALIDATOR_SIGNER_CERT`, `LQD_VALIDATOR_SIGNER_KEY` and optionally `LQD_VALIDATOR_SIGNER_NAME`. `scripts/railway/start-chain.sh` forwards them and does not pass a configured raw key when remote/encrypted signing is selected.

### VPS Compose activation

The VPS compose file contains an opt-in `signer` profile. Put the encrypted key, passphrase and certificates under `/opt/podl/signer-secrets` with directory mode `0700` and file mode `0600`. Then add these non-secret paths/settings to `/opt/podl/.env`:

```env
ENABLE_VALIDATOR_SIGNER=true
LQD_VALIDATOR_SIGNER_URL=https://signer:9101
LQD_VALIDATOR_SIGNER_CA=/run/podl-signer/ca.crt
LQD_VALIDATOR_SIGNER_CERT=/run/podl-signer/chain-client.crt
LQD_VALIDATOR_SIGNER_KEY=/run/podl-signer/chain-client.key
LQD_VALIDATOR_SIGNER_NAME=signer
LQD_VALIDATOR_SLASHING_DB=/app/data/signer/slashing-protection.json
```

The safe deploy script starts and health-checks the signer before recreating the chain. Do not enable the profile unless the encrypted secp256k1 key derives exactly to `VALIDATOR_ADDRESS`; the chain deliberately fails closed on an address mismatch.

For an existing VPS that still has `VALIDATOR_PRIVATE_KEY` in `/opt/podl/.env`, the one-time migration script creates the private CA, signer/client mTLS certificates, encrypted key file and passphrase with restrictive permissions. It starts an isolated preflight signer, verifies its reported address against `VALIDATOR_ADDRESS`, encrypts the old environment as a rollback artifact and stages a raw-key-free signed-BFT environment:

```bash
cd /opt/podl
PODL_ROOT=/opt/podl \
LQD_IMAGE=ghcr.io/zotish/podlblockchain:<immutable-commit-sha> \
sh ./podl_setup_validator_signer.sh
```

The GitHub `Deploy Backend To VPS` workflow exposes the same operation as the manual `configure_validator_signer` input. Normal push deployments never perform key migration implicitly. `podl_image_deploy.sh` checks the staged signer files/settings, takes a state snapshot while the old environment is still active, atomically activates the staged environment, waits for signer health before starting the chain, verifies all local services and rejects any height regression. A failed deploy decrypts the protected pre-migration environment, recreates the previous image and reports the snapshot path. Only a successful rollout removes the pending marker and leaves the active environment without the raw validator key.

## PKCS#11 HSM mode

Configure `LQD_PKCS11_MODULE`, `LQD_PKCS11_TOKEN_LABEL`, `LQD_PKCS11_KEY_LABEL`, `LQD_PKCS11_PUBLIC_KEY`, `LQD_PKCS11_PIN` and an explicit distinct `LQD_VALIDATOR_VRF_PRIVATE_KEY`. The PKCS#11 backend selects exactly one signing object, requests `CKM_ECDSA`, accepts raw or DER vendor signature encoding, normalizes low-S and reconstructs the recovery byte only when the result matches the pinned public key.

The real HSM, vendor PKCS#11 library, token provisioning and vendor conformance/security certification are external deployment evidence. The repository verifies the adapter and signature conversion paths without claiming access to a physical HSM.

## Verification

```bash
go test ./...
go test -race ./BlockchainComponent ./BlockchainServer
go vet ./...
sh -n scripts/railway/start-chain.sh scripts/railway/start-signer.sh
```

Automated tests cover the RFC 9381 published vector, proof/output tampering, canonical block entropy, consensus and timeout signatures, domain separation, duplicate/conflicting slots, restart durability, corrupt database fail-closed behavior, encrypted-key round trip/wrong password, raw and DER PKCS#11 signature conversion, TLS 1.3 mTLS, missing-client-certificate rejection and remote response verification.
