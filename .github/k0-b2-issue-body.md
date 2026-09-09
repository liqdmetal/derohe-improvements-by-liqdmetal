## Summary

**Hard-fork proposal:** the consensus half of closing the ringsize-2 loophole for SC calls. A ringsize-2 SC_TX exposes the signer by design (the ring IS sender+receiver; the proof's parity bit identifies the sender). This makes ringsize 2 **structurally impossible for contracts that don't need it** — without requiring any contract-author changes.

## The mechanism

1. **`SC_META_DATA` gains a `NoSigner` bit** — the high bit of the existing `Type` byte. The 33-byte wire format is **unchanged**, so existing metadata stays valid and existing contracts default to `uses_signer=true` (preserving behavior, exactly as the K0 design specifies).
2. **Auto-detection at install** — the parsed contract AST is scanned for `SIGNER()` calls (`ContractUsesSigner`, case-insensitive to match the DVM's dispatch). If none are found, the `NoSigner` bit is set. No contract author changes needed — a contract that never calls `SIGNER()` is automatically eligible for ringsize ≥ 4.
3. **Consensus enforcement** — SC_TX + ringsize 2 + `NoSigner`-marked contract → rejected with a clear error. Contracts that genuinely call `SIGNER()` keep ringsize 2 (owner-gated entrypoints) until the `verify_sig` migration (K0 Fix C) removes that need.
4. **`IsPrivate()` masks the bit** — the private-SC check (`meta.Type == 1`) would have broken for a private contract that is also NoSigner (0x81 ≠ 1). `IsPrivate()` masks the B2 bit so private + NoSigner coexist in one Type byte, and `InitializePrivate` still dispatches correctly.

## Why this matters

The ringsize-2 SC call is the *last legitimate* reason the protocol needs ringsize 2. Once `verify_sig` (DVM v9, PR #84) lands, owner-gated contracts can authorize at ringsize ≥ 4 via signature-in-payload, and ringsize 2 becomes fully extinct.

## Changes

| File | Change |
|---|---|
| `dvm/sc.go` | `SC_META_DATA` NoSigner bit (high bit of Type byte) + `ContractUsesSigner` AST scan + `IsPrivate()` mask |
| `blockchain/transaction_execute.go` | set NoSigner bit at install when no SIGNER() usage detected (**height-gated**, see below) |
| `blockchain/transaction_verify.go` | SC_TX + ringsize 2 + NoSigner → reject (reads SC meta tree); **SC_INSTALL ring-2 check** parses the code in SCDATA |
| `dvm/k0_b2_test.go` | 7 tests: detection, bit round-trip, 33-byte legacy compat, private+NoSigner coexistence |
| `blockchain/k0_b2_consensus_test.go` | 4 consensus-rule tests for the SC_INSTALL path |

## Hardening (wargame findings, now in the code)

1. **SC_INSTALL ring-2 hole (closed).** The original check only covered SC_CALL (SCID non-zero). An install of a no-SIGNER contract at ringsize 2 was unchecked — exposing the deployer for no legitimate reason. `k0SCInstallRing2Reject` now parses the SCDATA code, scans for `SIGNER()`, and rejects a ring-2 install of a no-SIGNER contract.
2. **Chain-split gating (critical).** The NoSigner bit changes the SC_META tree bytes, which are committed into the **chain state root** (`blockchain.go` `sc_change_cache` → tree hash). If a B2 node set the bit (or rejected a ring-2 install) before the fork while a legacy node did not, the two node types would produce **different state roots / disagree on block validity → instant chain split**. Both the install-path `SetNoSigner` and the verify-path SC_INSTALL rejection are therefore **gated on the hard-fork height** (`K0_MIN_RING4_HEIGHT` on this branch — same activation height as the B1 floor). Pre-fork: byte-identical behavior, zero divergence. Post-fork: all nodes run the same rule. SC_CALL is naturally safe because the bit can only appear post-fork (install is gated), so pre-fork contracts always read `NoSigner=false`.

## Tests

- `TestContractUsesSigner` — detects `SIGNER()` (incl. lowercase), no false positive on SIGNER-free contracts
- `TestSCMetaNoSignerBit` — bit set/clear, 33-byte serialization round-trip
- `TestSCMetaPrivateAndNoSignerCoexist` — private + NoSigner in one byte
- `TestSCMeta33ByteCompatibility` — legacy meta still decodes
- `TestK0SCInstallRing2Reject_*` — no-SIGNER install rejected at ring 2, SIGNER contract allowed, no-SCDATA allowed, case-insensitive

## Relationship to the K0 package

| PR | Fix | Effect |
|---|---|---|
| #80 | Fix A | wallet warns (non-consensus, ships now) |
| #82 | Fix B1 | consensus min-ring-4 for NORMAL/BURN |
| **this** | **Fix B2** | **consensus: ringsize-2 SC_TX only for contracts that declare SIGNER()** |
| #84 | Fix C (DVM v9) | `verify_sig` removes the last ringsize-2 need |

## Open questions (for review)

1. **Auto-detection vs explicit declaration**: AST scanning is automatic but can be fooled by indirect calls (e.g. a function that passes SIGNER() as a parameter — not possible in DVM-BASIC today, but worth stating). An explicit `OPTION NOSIGNER` pragma could be added later. The scan matches the DVM's own dispatch (case-insensitive, whole-line `REM`/`;` comment handling — a Rust reimplementation of the parser was differentially tested against this rule and had a mid-line-comment bug fixed as a result).
2. **HF scheduling + activation-height alignment (RESOLVED).** This is a storage-metadata + consensus check change; needs a hard fork. **Activation height is aligned with the B1 floor (PR #82): both gate on `K0_MIN_RING4_HEIGHT` (mainnet 7,600,000, testnet 0).** This branch carries its own `K0_MIN_RING4_HEIGHT` config field (mainnet 7,600,000 / testnet 0) and both the install-path `SetNoSigner` and the verify-path SC_INSTALL rejection key off it — so B2's rejection and B1's floor turn on **together** at one height. (Earlier drafts gated on `MAJOR_HF3_HEIGHT` 7,504,640, which would have activated B2 ~95k blocks before B1 — that window is closed.)
3. **Differential coverage**: the SIGNER-detection rule is pinned Go↔Rust in the derohe-rs harness (`dvm_uses_signer`, 12 vectors); the floor rule is pinned too (`k0_floor`, 159 vectors). The NoSigner meta-bit round-trip would benefit from the same treatment when the fork is green-lit.

---

*Branch: `feature/k0-fix-b2-uses-signer` in the fork `liqdmetal/derohe-improvements-by-liqdmetal`. Carries the build fixes: re-vendored modules (`vendor/modules.txt` present) and the `KickReader` removal.*
