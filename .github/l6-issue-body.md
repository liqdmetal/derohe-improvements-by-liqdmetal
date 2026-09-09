## Summary

**L6 from the DVM-BASIC language agenda (P2):** a real boolean type — `DIM b AS Bool`, built-in `TRUE`/`FALSE` constants, boolean assignment from comparisons, logical `&&` / `||` / `!`, and Bool function parameters.

## The syntax

```
DIM b AS Bool
LET b = TRUE
LET b = FALSE
LET b = (x > 3)          ' comparison -> 0/1
IF b && (y < 10) THEN ...
IF !(b == FALSE) THEN ...
Function Pass(flag Bool) Uint64 ...
```

## Implementation

- **`Bool` = Vtype 0x6**, stored internally as `uint64` 0/1 — fully compatible with the existing comparison/IF semantics (which already return 0/1), so no breaking change
- `check_valid_type("bool")` + every type-switch site extended (DIM, LET, eval Ident, arrays, function params, CONST resolution)
- **`TRUE`/`FALSE`** seeded as Bool-typed constants per interpreter run — they resolve through the L7 CONST path (which gained a Bool case)
- Gated `>= 10.0.0` like the rest of the L-series

## Tests (`dvm/control_flow_l6_test.go`, 4 cases)

| Test | Verifies |
|---|---|
| `TestL6_BoolBasics` | TRUE/FALSE constants + comparison assignment |
| `TestL6_BoolLogic` | `&&` / `||` / `!` |
| `TestL6_BoolParam` | Bool function parameter |
| `TestL6_VersionGate` | Bool at 1.2.3 rejected |

Full dvm suite green (29 control-flow tests).

## Relationship

- **Stacks on L7+L8** (PR #111) — uses the CONST resolution path for TRUE/FALSE
- L-series now 7/10 delivered (L1–L4, L6, L7, L8); remaining: L5 (multi-value returns), L9 (signed ints), L10 (block scoping)

---

*Branch: `feature/dvm-l6-bool` in the fork `liqdmetal/derohe-improvements-by-liqdmetal` (stacked on `feature/dvm-l7-l8-const-version`).*
