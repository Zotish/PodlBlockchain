#!/bin/sh
set -eu

LISTEN_TO_USE="${LQD_SIGNER_LISTEN:-0.0.0.0:${PORT:-9100}}"
SLASHING_DB_TO_USE="${LQD_VALIDATOR_SLASHING_DB:-${RAILWAY_VOLUME_MOUNT_PATH:-/app/data}/signer/slashing-protection.json}"

if [ -n "${LQD_VALIDATOR_KEY_PASSPHRASE_FILE:-}" ]; then
  if [ ! -r "$LQD_VALIDATOR_KEY_PASSPHRASE_FILE" ]; then
    echo "LQD_VALIDATOR_KEY_PASSPHRASE_FILE is not readable" >&2
    exit 1
  fi
  LQD_VALIDATOR_KEY_PASSPHRASE=$(sed -n '1p' "$LQD_VALIDATOR_KEY_PASSPHRASE_FILE")
  export LQD_VALIDATOR_KEY_PASSPHRASE
fi

set -- ./bin/lqd signer \
  -listen "$LISTEN_TO_USE" \
  -tls_cert "${LQD_SIGNER_TLS_CERT:?LQD_SIGNER_TLS_CERT is required}" \
  -tls_key "${LQD_SIGNER_TLS_KEY:?LQD_SIGNER_TLS_KEY is required}" \
  -client_ca "${LQD_SIGNER_CLIENT_CA:?LQD_SIGNER_CLIENT_CA is required}" \
  -slashing_db "$SLASHING_DB_TO_USE"

if [ -n "${LQD_PKCS11_MODULE:-}" ]; then
  set -- "$@" \
    -vrf_private_key "${LQD_VALIDATOR_VRF_PRIVATE_KEY:?LQD_VALIDATOR_VRF_PRIVATE_KEY is required for PKCS11 mode}" \
    -pkcs11_module "$LQD_PKCS11_MODULE" \
    -pkcs11_token "${LQD_PKCS11_TOKEN_LABEL:-}" \
    -pkcs11_key "${LQD_PKCS11_KEY_LABEL:?LQD_PKCS11_KEY_LABEL is required}" \
    -pkcs11_public_key "${LQD_PKCS11_PUBLIC_KEY:?LQD_PKCS11_PUBLIC_KEY is required}"
  if [ -n "${LQD_PKCS11_SLOT:-}" ]; then
    set -- "$@" -pkcs11_slot "$LQD_PKCS11_SLOT"
  fi
elif [ -n "${LQD_VALIDATOR_KEY_FILE:-}" ]; then
  set -- "$@" -key_file "$LQD_VALIDATOR_KEY_FILE"
else
  echo "LQD_PKCS11_MODULE or LQD_VALIDATOR_KEY_FILE is required" >&2
  exit 1
fi

exec "$@"
