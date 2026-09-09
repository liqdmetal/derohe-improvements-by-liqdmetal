## Summary

**L3 from the DVM-BASIC language agenda (P1):** first-class RAM arrays — `DIM a(n) AS type`, `a[i]` get/set, `arrlen`. Gives contracts list semantics instead of hand-rolled scalar keys, and composes with L1's `FOR` loops for iteration.

## The syntax

```
DIM a(10) AS Uint64      ' zero-filled array, indices 0..10 (cap 1024)
LET a[0] = 10
LET a[1] = 20
LET a[2] = a[0] + a[1]   ' expression index works too (loop vars)
RETURN arrlen("a")       ' 11
```

With L1's FOR:
```
FOR i = 0 TO 10
    LET a[i] = i * i
NEXT i
```

## Implementation

- `Variable` gains `Array *[]Variable` — a **pointer** so Variable stays comparable (it's used as a map key in `RamStore`/SC state; a raw slice would break compilation).
- **`DIM a(n) AS Uint64|String`** — detects the `( n )` form in `interpret_DIM`, allocates `n+1` zero-filled elements, cap 1024.
- **`a[i]` read** — new `*ast.IndexExpr` case in the evaluator (Go's parser already tokenizes `a [ i ]`).
- **`LET a[i] = expr`** — new branch in `interpret_LET`; the index is **evaluated** (literal or expression like a loop variable), with bounds checking.
- **`arrlen("a")`** — intrinsic returning the length, gated `>= 10.0.0`.

## Consensus safety

- **Arrays are Locals-only**: contracts cannot `STORE` an array (STORE takes scalars), so arrays never touch the serialized SC state — **consensus-neutral** (no state-format change).
- Gated `>= 10.0.0` like L1/L2 — pre-fork contracts can't use it.

## Tests (`dvm/control_flow_l3_test.go`, 4 cases)

| Test | Verifies |
|---|---|
| `TestL3_ArrayBasics` | set/read/compute + `arrlen` |
| `TestL3_ArrayForLoop` | L1+L3 compose: sum of squares 0..10 = 385 |
| `TestL3_StringArray` | string arrays |
| `TestL3_VersionGate` | `DIM a(n)` at `1.2.3` rejected |

Full dvm suite green.

## Relationship

- **Stacks on L2** (PR #103) — same `>= 10.0.0` gate and interpreter machinery
- Enables the next agenda item **L4 (map enumeration)** — arrays give `mapkeys` a natural return type

---

*Branch: `feature/dvm-l3-arrays` in the fork `liqdmetal/derohe-improvements-by-liqdmetal` (stacked on `feature/dvm-l2-subroutines`).*
