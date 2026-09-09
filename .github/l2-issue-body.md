## Summary

**L2 from the DVM-BASIC language agenda (P0):** subroutines via `GOSUB`/`RETURN` — internal code reuse so entrypoints stop re-implementing helpers. Pairs with L1 structured control flow (PR #101): loops for repetition, subroutines for reuse.

## The syntax

```
Function Settle(x Uint64) Uint64
	5 version("10.0.0")
	10 DIM r AS Uint64
	20 GOSUB 100        ' call the helper
	30 RETURN r
	100 LET r = compute(x)   ' helper body (shared Locals)
	110 RETURN          ' back to line 30
End Function
```

## Implementation

- **`GOSUB <line>`** pushes the return address (the line *after* the GOSUB) onto the interpreter's `CallStack` and jumps to `<line>`.
- **`RETURN`** pops the `CallStack` when non-empty (subroutine return — the value is ignored; subroutines communicate via shared `Locals`) and jumps back; when the stack is empty it behaves **exactly as today** (function return with value).
- Fully **backward-compatible**: existing contracts never push a `CallStack`, so their `RETURN` semantics are untouched.
- Nests correctly: GOSUB-in-GOSUB and GOSUB-in-FOR both verified.

## Consensus safety

Gated on DVM version `>= 10.0.0` like L1 — the contract must call `version("10.0.0")`. Pre-fork contracts cannot accidentally use it.

## Tests (`dvm/control_flow_l2_test.go`, 5 cases)

| Test | Verifies |
|---|---|
| `TestL2_Gosub` | shared-Locals helper (x·2) |
| `TestL2_NestedGosub` | nested calls → 1 + 10 = 11 |
| `TestL2_GosubInFor` | helper inside a FOR body → 1+2+3 = 6 |
| `TestL2_FunctionReturnStillWorks` | plain function RETURN unaffected |
| `TestL2_VersionGate` | GOSUB at `1.2.3` rejected |

Full dvm suite green; `dvm` package builds clean.

## Relationship

- **Stacks on L1** (PR #101) — same `>=10.0.0` gate, same interpreter mechanism
- Both P0 language items now delivered; L3 (arrays) is the natural next step

---

*Branch: `feature/dvm-l2-subroutines` in the fork `liqdmetal/derohe-improvements-by-liqdmetal` (stacked on `feature/dvm-l1-control-flow-pr`). Carries the build fixes: re-vendored modules (`vendor/modules.txt` present) and the `KickReader` removal, so the tree builds from a fresh clone.*
