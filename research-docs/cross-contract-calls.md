# Cross-Contract Calls — DVM Design Proposal (C6)

**STATUS: ⚠️ DRAFT — DESIGN PROPOSAL, NOT IMPLEMENTED. Grounded in
derohe-Release151 source. Hard-fork feature (VM execution-model change).
Companion to `dero-improvements-agenda.md`. ⚠️**

> The single biggest DeFi gap in DERO's DVM today (verified absent):
> a contract cannot call another contract. Every "composable" DERO dApp
> is therefore a monolithic contract. This document designs the minimal,
> safe version of cross-SC calls — respecting the existing architecture
> rather than redesigning it.

---

## 1. Why it matters (the case)

Without cross-contract calls:
- **No composability** — a router, an escrow, and a token cannot be
  separate audited contracts; they must be one giant contract, so a bug
  in any piece compromises all of it, and reuse is impossible.
- **No standard interfaces** — there is no way to call "the standard
  token" or "the standard escrow" by interface; every dApp re-implements.
- **No upgrade layering** — you cannot swap out a dependency contract
  (e.g. an oracle) behind a stable interface.

The DERO codebase *already anticipates this*: `Shared_State` has
`SCIDSELF` with the comment *"points to SELF SCID, this separation is
necessary, if we enable cross SC calls — but note they bring all sorts
of mess, bugs"* (`dvm/dvm.go:410-411`). And `Monitor_recursion` exists
to bound call depth ("64 calls are more than necessary"). So the
architecture was built with the door open — this design walks through it.

---

## 2. The execution model today (what we must not break)

1. One SC tx executes exactly **one** entrypoint: `Execute_sc_function`
   (`sc.go:122`) builds a `Shared_State`, wires a fresh `TX_Storage`,
   runs `runSmartContract_internal`, then (on `Monitor_recursion == 0`)
   persists state and processes `Assets_Transfer` / `Transfers`.
2. **Transfers are deferred until the contract terminates successfully**
   (`dvm.go:374-390`; `Assets_Transfer` comment: *"transfers are only
   processed after the contract has terminated successfully"*). This is
   the existing atomicity boundary — we build on it, not around it.
3. **Recursion is already guarded**: `state.Monitor_recursion` starts 0,
   must return to 0 at end (`dvm.go:385`), and bounds nesting.
4. **SC storage is per-SCID in a shared data tree** (`LoadSCValue` /
   `LoadSCAssetValue` keyed by SCID), and the SC's own SCID is
   `SCIDSELF` — the separation needed to address *another* SC is already
   in the state.

---

## 3. Design: `CALL_SC(scid, entrypoint, args...)` intrinsic

### 3.1 The primitive

```go
// CALL_SC(scid_hex String, entrypoint String, args String...) -> Uint64
// Executes entrypoint on the target SC in a nested DVM invocation,
// sharing the same tx's state tree and gas budget. Returns the target's
// RETURN value (0 = success, nonzero = error/rollback, per SC convention).
```

A contract calls:
```
LET ok = CALL_SC(TARGET_SCID, "Deposit", ITOA(amount), blind)
IF ok != 0 THEN GOTO fail
```

### 3.2 Execution semantics (minimal, safe)

1. **Nested `Execute_sc_function`** with:
   - `scid` = target; the `Shared_State` is **re-used** (same
     `GasComputeLimit`, same `RND`, same `Store` tree wrapper), so the
     call is *in-transaction*, not a separate tx.
   - `SCIDSELF` switched to the target for the duration of the nested
     call, restored on return (the field exists precisely for this).
   - `signer` propagated unchanged (the caller's signer context is not
     the nested contract's concern).
   - `incoming_value` = empty unless the parent explicitly forwards
     value (see §3.3).
2. **Recursion guard**: `Monitor_recursion++` on entry, `--` on exit.
   Hard cap (e.g. 8 — far below the 64 the comment allows) to bound
   worst-case gas. Exceeding → nested call fails with error, parent
   continues (sandbox, not panic).
3. **Atomicity**: the nested call writes to the *same* `TX_Storage`
   (`RawKeys` + `Transfers` + `Assets_Transfer`), which is only
   persisted if **every** level in the chain returns success. A failing
   nested call must **roll back its own writes** — the parent sees the
   failure code and can decide. Two options:
   - **A (snapshot/rollback)**: snapshot `RawKeys`/`Transfers` before the
     nested call, restore on failure. Simple, no callee discipline.
     Slightly more state to carry (a keys-diff). **Recommended.**
   - **B (callee-cleanup contract)**: require the callee to not commit
     on error. Cheaper but puts correctness on the callee — weak.
4. **Return value**: the nested entrypoint's Uint64 return is surfaced
   to the parent (0 = success per DERO convention). Errors internal to
   the callee (panic caught by `Execute_sc_function`'s recover) surface
   as a failure code (e.g. 2), never as a panic in the parent.
5. **Value forwarding**: a parent may forward DERO/assets to the callee
   via `incoming_value` for that call (e.g. "fund this escrow"), or the
   callee can use its own `derovalue()`. Deliberately no *return* of
   value to parent in v1 (avoid reentrancy); funds flow one direction
   per call.

### 3.3 What we deliberately do NOT do in v1

- **No dynamic dispatch** (call by interface/name registry) — v1 is
  call-by-SCID only; a registry (C8) layers on top later.
- **No caller-identity to callee** beyond the shared signer — the callee
  must use its own auth (verify_sig, K0 Fix C) rather than trusting
  "who called me."
- **No cross-SC value *return*** — funds only flow into a callee, and
  the parent's deferred-transfer model is preserved.
- **No gas sub-metering by callee** — one shared budget; a malicious or
  buggy callee can consume the parent's gas, but that's *bounded and
  visible* (the parent chose to call it), and the recursion cap bounds
  the blast.

---

## 4. Reentrancy & safety analysis (the "all sorts of mess" the codebase warns about)

| Risk | Mitigation |
|---|---|
| **Reentrancy** (callee calls back into caller mid-call) | The caller's state writes are uncommitted until the whole chain succeeds (deferred `Transfers`, snapshot rollback). A reentrant call sees the *pre-commit* state — it cannot observe or spend intermediate writes. This is the classic "checks-effects-interactions" invariant, enforced structurally. |
| **Infinite recursion** | `Monitor_recursion` cap (8) — hard bound on gas + depth. |
| **Gas exhaustion mid-chain** | Shared budget; a nested call that would exceed the limit fails cleanly (rollback option A), parent handles the code. |
| **Callee consumes parent's storage** | Shared tree; bounded by gas/storage limits already enforced. The parent chose the callee — trust is explicit. |
| **Signer spoofing** | Signer propagated unchanged; callee must do its own auth via `verify_sig` (never trust a "caller" the way `SIGNER()` would at ringsize 2). |
| **State visibility of a reentrant call** | Nested call reads the *same* tree but the parent's uncommitted `RawKeys` are visible to it — document that a callee can read in-progress parent writes; acceptable (same trust domain as "callee sees everything anyway" post-commit). |

**The core safety principle:** a cross-contract chain is *one logical
tx* with one gas budget, one commit point, and the deferred-transfer +
snapshot-rollback machinery making failure atomic. The callee cannot
steal or double-spend because nothing it sees is committed until the
whole chain returns success.

---

## 5. Changes required (diff-level, mapped to the tree)

| File | Change |
|---|---|
| `dvm/dvm_functions.go` | `func_table["call_sc"]` (>= 10.0.0), `dvm_call_sc` handler |
| `dvm/sc.go` | Nested-execution helper `Execute_sc_function_nested` reusing the state; snapshot/restore of `RawKeys` + `Transfers` + `Assets_Transfer` |
| `dvm/dvm.go` | `Monitor_recursion` cap (8); `SCIDSELF` push/pop |
| `dvm/sc.go` / `transaction_execute.go` | Version-lock: `call_sc` gated on DVM version; hard-fork height |
| `dvm/simulator.go` | Simulator support for nested calls (tests) |
| `tests/vectors/` | Conformance: nested success, nested failure rollback, reentrancy rejection, recursion-cap, gas-exhaustion |

---

## 6. Gas & versioning

- `call_sc` base ComputeCost: **10,000** (dispatch overhead) + the
  callee's own execution gas (shared budget).
- Recursion cap: 8 (a deliberate, low ceiling — the codebase's own
  comment says 64 is already excessive).
- Version gate: `>= 10.0.0` (a new DVM major — this changes the
  execution model, not just the table).

---

## 7. Sequencing & relation to other improvements

| Order | Item | Why |
|---|---|---|
| 1 | **verify_sig / K0 Fix C** | Callees need independent auth; they can't trust "the caller" — this must exist first |
| 2 | **CALL_SC (this design)** | The composability primitive |
| 3 | **Contract registry / interface standard (C8)** | Discoverability + standard interfaces on top of CALL_SC |
| 4 | **Structured control flow (L1/L2)** | Nested-call error handling becomes readable (no GOTO spaghetti for call-failure branches) |

---

## 8. Open questions (before implementation)

1. **Rollback granularity**: option A (snapshot+restore) vs B
   (callee-cleanup). Recommend A — correctness must not depend on the
   callee's discipline.
2. **Depth vs breadth**: is a cap of 8 nested calls enough for real
   dApps (router → pool → token is depth 3)? Probably yes; revisit with
   real contracts.
3. **Can the callee see the parent's uncommitted writes?** Recommend
   *yes* (shared tree) but document it as the reentrancy boundary —
   a callee that reads parent in-progress state must be treated as
   same-trust.
4. **Value forwarding API**: explicit `CALL_SC(scid, ep, value, asset, args)`
   vs letting the callee pull via `derovalue()`? Recommend explicit
   forwarding — clearer and safer than a pull model.

*⚠️ DRAFT — design for review. The recursion guard and SCIDSELF
separation already exist in the codebase, which lowers the risk profile
considerably, but this is still a consensus-relevant execution-model
change requiring a hard fork and careful conformance coverage. ⚠️*
