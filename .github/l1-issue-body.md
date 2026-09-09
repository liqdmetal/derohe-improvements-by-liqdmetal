## Summary

**L1 from the DVM-BASIC language agenda (P0):** structured control flow — `FOR`/`NEXT`, `WHILE`/`WEND`, and block `IF`/`ELSE`/`ENDIF` — replacing GOTO-only spaghetti. Halves contract size (cuts the `len(scdata)*1.5` code-size fee term), makes contracts auditable, and reduces GOTO bugs.

## The syntax

```
FOR i = 1 TO 10 [STEP 2]
    ... body ...
NEXT i

WHILE LOAD("x") < 10
    ... body ...
WEND

IF x > 3 THEN
    ... then-block ...
ELSE
    ... else-block ...
ENDIF
```

## Implementation

New `dvm/control_flow.go` + a `LoopFrame` stack on `DVM_Interpreter`:
- **`FOR var = start TO end [STEP step]` / `NEXT var`** — counter loop, step defaults to 1, counter lives in `Locals` so it's visible to the body
- **`WHILE expr` / `WEND`** — pre-tested loop; `WEND` jumps back to the `WHILE` line which re-evaluates
- **block `IF expr THEN ... [ELSE ...] ENDIF`** — detected as an `IF` line ending in `THEN` without `GOTO`; the existing single-line `IF expr THEN GOTO x [ELSE GOTO y]` form is untouched
- `findMatchingLine` scans forward for the matching `WEND`/`ENDIF` respecting nesting (verified with FOR-in-WHILE)

**No parser changes** — the line-token interpreter dispatches the new keywords natively.

## Consensus safety

All new keywords are **gated on the contract's declared DVM version** (`>= 10.0.0` — the contract must call `version("10.0.0")`). Pre-fork contracts cannot accidentally use the new syntax; a contract at `1.2.3` or with no `version()` call gets a clear rejection. This matches the existing intrinsic-versioning model.

## Tests (`dvm/control_flow_test.go`, 8 cases)

| Test | Verifies |
|---|---|
| `TestL1_ForNext` | `FOR 1..5` sums to 15 |
| `TestL1_ForStep` | `STEP 2` → 0+2+4 = 6 |
| `TestL1_WhileWend` | WHILE countdown runs 10 iterations |
| `TestL1_BlockIfElse` | both branches of IF/ELSE |
| `TestL1_BlockIfNoElse` | false falls through past ENDIF |
| `TestL1_Nested` | FOR inside WHILE, nested frames → 9 |
| `TestL1_VersionGate` | `version("1.2.3")` + FOR → rejected |
| `TestL1_NoVersion` | no `version()` + FOR → rejected |

Full dvm suite green; `dvm` package builds clean on the community-dev base.

## Relationship

- First of the language agenda items (L2 subroutines next)
- Independent of the DVM v9 intrinsics — no stacking dependency
- Version-gated `>= 10.0.0`, so it slots into the same HF window as `verify_proof` (PR #94)

---

*Branch: `feature/dvm-l1-control-flow-pr` in the fork `liqdmetal/derohe-improvements-by-liqdmetal`. Carries the build fixes: re-vendored modules (`vendor/modules.txt` present) and the `KickReader` removal, so the tree builds from a fresh clone.*
