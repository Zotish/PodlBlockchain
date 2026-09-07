# Explorer security review

This branch contains Explorer source changes only. A GitHub push is not proof
that either the Explorer site or the blockchain node has been deployed. Do not
merge solely to preview the UI: the repository's `main` push workflow can deploy
the backend to the VPS.

## Visible changes

- Account access accepts a public address or an extension-shared address. The
  public Explorer does not request a private key, seed phrase or vault password.
- The account dashboard shows public balances, receiving information and
  activity. Native transfers require a separately enabled, pinned V4 test
  network, preflight review and explicit local-wallet approval. Signing is off
  by default; the existing V2 node cannot satisfy that gate.
- The bridge page avoids the previous secret-bearing signing workflow and
  exposes public status and operational limitations.
- The investor page verifies signed evidence in the browser against configured
  trust roots. Missing/mismatched/stale evidence is not labelled verified.
  Business revenue and slashing recovery are displayed separately when the
  backend supplies that distinction; missing data is not invented.
- API documentation reflects the safer public-access boundaries.

## Reliability and release changes

- Request timeouts include body parsing and compose with caller cancellation.
- API caching is bounded with expiry eviction.
- The Sites worker adds a restricted CSP, frame denial and other security
  headers. Build output includes asset integrity and release hashes.
- The existing responsive layout is retained. This is a security/behavior
  update, not a new visual redesign or an independent security audit.

## Local validation

With a compatible Node.js runtime and the lockfile dependencies installed:

```sh
cd blockchain-explorer
npm test
npm run build
npm test -- scripts/security-policy.test.mjs
node scripts/verify-sites-release.mjs
```

Validation for this release: 107 Explorer tests passed, the production build
passed, and the worker security/release-integrity checks passed. No dependency
advisory audit or live deployment is implied by these results.

For local preview, run `npm run dev` and open the exact URL printed by Vite.
The public endpoint defaults are documented in `.env.example`. Never put keys,
passwords or other secrets in browser-exposed `VITE_*` variables.
