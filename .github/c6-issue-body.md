## Summary

**C6 from the improvement agenda — cross-contract calls: `call_sc`.**
The single biggest DeFi gap in DERO's DVM is closed: a contract can now
call another contract. No more monolithic dApps.

## The primitive

```
LET ok = call_sc(TARGET_SCID, "Deposit", "amount", ITOA(amount), "blind", blind)
IF ok != 0 THEN GOTO fail
```

- `call_sc(scid_hex String, entrypoint String, name1, val1, name2, val2, ...) -> Uint64`
- Runs the target's `entrypoint` in a **nested DVM invocation** sharing the
  same tx: one gas budget, one commit point, deferred transfers intact
- Returns the callee's RETURN value (0 = success, nonzero = error/rollback)

## Execution semantics (the safe minimal version)

| Concern | Design |
|---|---|
| **Storage isolation** | Each SC lives in its own per-SCID graviton tree; the nested call switches `SCIDSELF` + `Chain_inputs.SCID` to the target and resolves/executes against the target's own tree |
| **Atomicity** | Nested writes commit to the target's tree directly, safe under the snapshot — a failing top-level tx rolls back the whole chain. Nonzero RETURN rolls back the callee's writes; parent sees it |
| **Reentrancy** | Caller's writes are uncommitted until the whole chain succeeds, so a reentrant call sees pre-commit state (checks-effects-interactions, enforced structurally) |
| **Recursion** | `Monitor_recursion` cap of **8** (the codebase's own comment says 64 is already excessive); exceeding it fails the call cleanly, never panics the parent |
| **Auth** | Signer propagated unchanged; callee must do its own auth via `verify_sig` (K0 Fix C) — never trusts "who called me" |
| **Gas** | `call_sc` base cost 10,000 + the callee's execution on the shared budget |

## Deliberately NOT in v1

- No dynamic dispatch (call-by-SCID only — a registry/interface standard layers on top later)
- No cross-SC value *return* (funds flow one direction per call)
- No caller-identity to the callee beyond the shared signer

## Changes

- `dvm/dvm_functions.go` — `dvm_call_sc` handler + func_table entry (`>= 10.0.0`)
- `dvm/dvm.go` — `Shared_State` gains `TreeCache` + `Snapshot` (to reach any SC's tree)
- `dvm/sc.go` — `Execute_sc_function` carries the tree cache/snapshot through
- `dvm/simulator.go`, `blockchain/transaction_execute.go` — wire the tree cache/snapshot

## Tests (`dvm/call_sc_test.go`)

| Test | Verifies |
|---|---|
| `TestC6_NestedSuccess` | caller → counter's `Inc(7)` → count persisted = 7 |
| `TestC6_FailureRollback` | failing callee's write (`poison`) discarded |
| `TestC6_RecursionCap` | self-call chain exceeds depth, fails cleanly |

Full dvm suite green; whole tree builds.

## Why this matters

Router, escrow, and token can now be **separate audited contracts** calling
each other instead of one giant contract where a bug in any piece
compromises all of it. Standard interfaces become possible. Upgrade
layering (swap an oracle behind a stable interface) becomes possible.
This is the composability primitive that a settlement rail — atomic swaps,
DEX, confidential AMM, perpDEX — is built on.

## Relationship

- Implements `research-docs/cross-contract-calls.md` (issue #87)
- Stacks on the DVM v9 intrinsics (PR #84); sequencing per design: verify_sig first → call_sc → registry/interface standard (next)

---

*Branch: `feature/dvm-c6-callsc` in the fork `liqdmetal/derohe-improvements-by-liqdmetal` (stacked on `feature/dvm-v9-intrinsics`).*
