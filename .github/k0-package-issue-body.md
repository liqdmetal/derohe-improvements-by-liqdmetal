## Summary

**The complete K0 privacy package, consolidated into one reviewable PR.**

RingSize-2 transactions expose the signer on-chain by design — the ring *is*
sender + receiver, and parity selects the sender (`Extract_signer`). **56.8% of
mainnet txs use ringsize 2.** This package eliminates the loophole in three
stages, sequenced so each is independently reviewable and the non-consensus
half ships immediately.

## The fixes

### Fix A — wallet warning (no hard fork, ships now)
The wallet/API half. When a transfer resolves to ringsize 2, the wallet sets a
`PrivacyWarning` surfaced through the RPC result. No consensus change.

### Fix B1 — consensus min-ring-4 floor (hard fork)
From `K0_MIN_RING4_HEIGHT`, NORMAL/BURN txs with ringsize < 4 are rejected.
The floor of 4 = sender + receiver + 2 decoys is the smallest meaningful
anonymity set. Hardened against two bypasses found during wargaming:
- **fork-boundary dodge** — the floor keys off the chain tip, not the
  attacker-settable `tx.Height` (a tx can reference a block 11 back)
- **SC_TX type-lie** — a ringsize-2 NORMAL transfer stamped `SC_TX` (a no-op
  execution path) no longer dodges the gate

### Fix B2 — uses_signer registry (hard fork)
RingSize-2 SC calls expose the signer the same way. The **NoSigner bit** (high
bit of the `SC_META_DATA` Type byte — 33-byte wire format *unchanged*) marks
contracts that never call `SIGNER()`, auto-detected at install via AST scan
(`ContractUsesSigner`). Existing contracts default to `uses_signer=true`
(behavior preserved). Then:
- `SC_TX` + ringsize 2 + NoSigner → rejected
- `SC_INSTALL` at ringsize < 4 → rejected

After B1+B2, **ringsize 2 has no legitimate remaining use**.

## Why this is one PR now

Previously split across four PRs (#80, #82, #91, #98), each carrying a
4,400-file vendored-dependency diff that made review impractical. This is the
same code consolidated into **16 files / ~1,000 lines**, with no vendor noise,
plus the build-manifest fix (upstream's `go.mod` is 3 lines with no
`vendor/modules.txt`, so fresh clones don't build).

## Tests (22, all green)

- `blockchain`: k0_ringsize (floor reject / not-active / pre-fork), wargame
  bypass + SC_TX type-lie, k0_b2 consensus
- `dvm`: ContractUsesSigner (detect / no-false-positive / case-insensitive),
  SC_META NoSigner bit, Private+NoSigner coexist
- `walletapi`: ringsize-2 warning set, ring-4 no-warning, Transfer result field

## Sequencing note

The fourth stage — Fix C, owner auth via `verify_sig` so gated contracts can
run at ringsize ≥ 4 — depends on the DVM v9 `verify_sig` intrinsic and remains
a separate PR (submitted separately against the intrinsics).

## Supersedes

Closes the fragmented #80 (Fix A), #82 (Fix B1), #91 (Fix B2) in favour of
this single package. (#98 / Fix C stays open as it targets a different base.)

---

*Branch `fix/k0-privacy-package`, based on `community-dev`.*
