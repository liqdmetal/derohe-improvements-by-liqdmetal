## Summary

Mainnet measurement (50k-block scan) shows **~57% of transactions ship at ringsize 2**, where the ring IS the two participants (sender+receiver, zero decoys) and the signer is recoverable on-chain by design. This is the wallet/API half of the fix: **warn loudly and surface it to callers** when a tx would expose the sender. No consensus change — ships immediately.

## Why ringsize 2 exposes the signer

At ringsize 2 the ring is [sender, receiver]; the proof parity bit selects one of the two positions and, because sender/receiver are placed at opposite parity, the parity bit identifies the sender uniquely. `Extract_signer` (blockchain/transaction_execute.go) recovers it.

## This PR (Fix A)

- `walletapi/wallet_memory.go` — `LastRingSizeWarning` field (not persisted)
- `walletapi/wallet_transfer.go` — `TransferPayload0` sets the warning when ringsize resolves to 2 (or < 4)
- `rpc/wallet_rpc.go` — `Transfer_Result.PrivacyWarning` surfaces it to programmatic callers
- `walletapi/rpcserver/rpc_transfer.go` — wire `PrivacyWarning` into the result
- `cmd/dero-wallet-cli/prompt.go` — burn path prints the warning and requires explicit confirmation
- `walletapi/k0_guard_test.go` — 5 tests

**Warn, do not force:** SC entrypoints calling `SIGNER()` still require ringsize 2; forcing a higher ring would break owner-gated contracts until a signature-check intrinsic (K0 Fix C) lands. Consensus enforcement (Fix B: min-ring-4 floor) is a separate hard fork and intentionally NOT in this PR.

## Why it matters

- ringsize-2 txs are a silent privacy footgun for every wallet user and programmatic caller
- no migration burden, no fork, no wallet-file change

## Follow-ups (not this PR)

- **Fix B1**: consensus min-ring-4 floor for NORMAL/BURN txs (hard fork) — now hardened against two wargame bypasses (chain-tip keying, SC_TX type-lie restamp)
- **Fix C**: `verify_sig` intrinsic so owner-gated contracts can authorize at ringsize >= 4

---

*Branch: `fix/k0-ringsize-warning` in the fork `liqdmetal/derohe-improvements-by-liqdmetal`. Carries the build fixes: re-vendored modules (`vendor/modules.txt` present) and the `KickReader` removal, so the tree builds from a fresh clone.*
