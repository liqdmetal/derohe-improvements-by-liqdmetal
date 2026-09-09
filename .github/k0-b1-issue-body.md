## Summary

**Hard-fork proposal:** from a fixed height, reject NORMAL/BURN transactions with ringsize < 4 at consensus. Ringsize-2 txs expose the signer on-chain by design — the ring is just [sender, receiver] and the proof's parity bit identifies the sender uniquely (`Extract_signer`). Mainnet baseline: **~57% of txs ship at ringsize 2**.

The privacy floor of 4 (sender + receiver + 2 decoys) is the smallest meaningful anonymity set.

## This PR (Fix B1)

- `config/config.go` — `K0_MIN_RING4_HEIGHT` (mainnet 7,600,000 post-HF3; testnet 0 = apply at genesis)
- `blockchain/transaction_verify.go` — `K0RingSizeFloorReject()`: NORMAL/BURN ringsize < 4 rejected post-gate, **keyed off the current chain tip at verification time**
- `blockchain/k0_ringsize_test.go` — 3 rule tests (post-gate reject, SC_TX exemption, pre-fork unaffected)
- `blockchain/wargame_k0_bypass_test.go` + `wargame_k0_sctxtype_test.go` — 4 wargame regression tests pinning the two hardening fixes below

**Exemptions:** SC_TX / coinbase / registration / premine. SC owner-gated calls still require ringsize 2 today because they use `SIGNER()` — forcing a higher ring would break them until the `verify_sig` intrinsic (K0 Fix C) lands. This is explicit in the code comments.

## Hardening (wargame findings, now in the code)

Two bypasses were found and closed before this PR shipped:

1. **Fork-boundary bypass (closed).** The original rule keyed off `tx.Height`, which is **attacker-settable** (`walletapi/transaction_build.go:37`). Since `TX_VALIDITY_HEIGHT = 11` lets a tx reference a block up to 11 back, a ring-2 NORMAL tx pinned to a pre-fork block would dodge the gate for the first ~11 blocks after activation. The floor now keys off **`chain.Get_Height()` at verification time** — the window is closed.
2. **SC_TX type-lie bypass (closed).** `TransactionType` is an attacker-set header field (`transaction.go:316`), and `process_transaction_sc` treats an SC_TX with no `SCACTION` in SCDATA as a no-op (`transaction_execute.go:267,291`). A ring-2 NORMAL transfer stamped `SC_TX` used to dodge the SC_TX exemption for free. Now any SC_TX that does not carry `SCACTION` is re-stamped NORMAL and meets the floor.

Both are pinned by wargame regression tests (`TestWargame_K0FloorNoPreForkDodge`, `TestWargame_K0FloorSC_TXTypeLieBypass`, plus the execute-path no-op case).

## Design notes

- **Dust threshold:** recommend an outright ban for all NORMAL/BURN (the fee floor already prices micro-txs); no `value > dust` carve-out.
- **HF3 whitelist interaction:** the affected-tx whitelist (`hardfork_fixes.go`) covers pre-fork txs only, so no conflict — verified.
- **Fee impact:** ringsize-16 txs are larger (fee = `FEE_PER_KB × (len/16 + mult)`); privacy has a price, documented.
- **No state-tree write:** this is a pure rejection rule (no graviton/SC_META mutation), so there is no chain-split vector — all nodes above the fork height run the same rule, below it nothing changes.

## Relationship to the other K0 PRs

- **Fix A** (wallet warning, non-consensus, ships immediately) — separate PR, the wallet/API half.
- **Fix B2** (contract registry `uses_signer`) — follow-up: ringsize 2 only for genuinely owner-gated SC entrypoints.
- **Fix C** (`verify_sig` intrinsic) — lets owner-gated contracts authorize at ringsize ≥ 4, removing the last ringsize-2 need.

This PR is deliberately scoped to the consensus floor only.

---

*Branch: `fix/k0-min-ring4` in the fork `liqdmetal/derohe-improvements-by-liqdmetal`. Carries the build fixes: re-vendored modules (`vendor/modules.txt` present) and the `KickReader` removal, so the tree builds from a fresh clone.*
