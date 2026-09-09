## Summary

**One reviewable PR replacing seven stacked PRs (#101, #103, #107, #109, #111,
#115, #117).** The complete DVM v10 language: structured BASIC.

## What's in it

| Item | Syntax | What it gives |
|------|--------|---------------|
| L1 | `FOR x = a TO b [STEP z]` / `NEXT`, `WHILE`/`WEND`, block `IF/ELSE/ENDIF` | structured loops replace GOTO spaghetti |
| L2 | `GOSUB <line>` / `RETURN` | shared helper reuse inside one contract |
| L3 | `DIM a(n) AS type`, `a[i]`, `arrlen(a)` | list semantics + loops over arrays |
| L4 | `mapkeys()` | enumerate SC state keys (batch/paged contracts) |
| L6 | `DIM b AS Bool`, `TRUE`/`FALSE`, `&&`/`||`/`!` | real boolean type |
| L7 | `CONST name = value` | immutable named constants |
| L8 | enforced version gate | new syntax hard-requires `version("10.0.0")` |
| L9 | `DIM i AS Int`, negative literals, signed arithmetic | signed 64-bit ints (deltas, negatives) |

## Design notes (why it's safe)

- All gated `>=10.0.0` — pre-fork contracts can never accidentally parse new
  syntax (`gateControlFlow` rejects versions without `version()`).
- Arrays (`*[]Variable`), Bool, Int are **Locals-only** — never serialized to
  `RamStore`/storage, so the change is consensus-neutral on the state tree.
- `Variable.Array` is a pointer so `Variable` stays comparable (it's a map key).
- Block-IF coexists with the existing single-line `IF THEN GOTO` (untouched).
- `RETURN` pops the `CallStack` when non-empty (subroutine return), otherwise
  the existing function-return semantics are unchanged.

## Tests

14 files, 8 control-flow test suites (L1–L9) covering: FOR sum/step/nesting,
WHILE countdown, IF/ELSE both branches, nested GOSUB, array fill via loop +
get/set + arrlen, Bool logic + params, CONST immutability, Int negative
arithmetic, and version-gate rejection. Plus a wargame test.

## Supersedes

Closes #101, #103, #107, #109, #111, #115, #117 in favour of this single
package.
