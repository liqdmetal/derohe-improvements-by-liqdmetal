## Summary

**L7 + L8 from the DVM-BASIC language agenda (P2):** named immutable constants (`CONST`) and the version auto-gate enforced as the default.

## L7: `CONST`

```
CONST BASE = 100          ' uint64
CONST SCALE = 3
CONST DOMAIN = "relayos"  ' string
```

- Declared inside a function, scoped to the interpreter run (Locals-only, **consensus-neutral**)
- The evaluator resolves identifiers against `Constants` first
- **Immutability enforced**: `LET` on a constant is rejected

## L8: version auto-gate

Every new syntax keyword (L1 control flow, L2 subroutines, L3 arrays, L4 mapkeys, L7 CONST) **requires `version("10.0.0")`**. A contract with no `version()` call (defaults to 0.0.0) or an old one (1.2.3) gets a hard rejection. Pre-fork contracts cannot accidentally use new syntax — the gate is now the enforced default, not an afterthought.

## Tests (`dvm/control_flow_l7_test.go`, 5 cases)

| Test | Verifies |
|---|---|
| `TestL7_ConstUint` | CONST arithmetic: BASE·SCALE+1 = 301 |
| `TestL7_ConstString` | CONST string concat + compare |
| `TestL7_ConstImmutable` | LET on a const → rejected |
| `TestL8_VersionGate` | CONST at 1.2.3 → rejected |
| `TestL8_NoVersion` | CONST with no version() → rejected |

Full dvm suite green (25 control-flow tests including L1–L4 regression).

## Relationship

- **Stacks on L4** (PR #109) — same `>= 10.0.0` gate
- The L-series is now 6/10 delivered (L1–L4, L7, L8); remaining: L5 (multi-value returns), L6 (boolean type), L9 (signed ints), L10 (block scoping) — all P2/P3

---

*Branch: `feature/dvm-l7-l8-const-version` in the fork `liqdmetal/derohe-improvements-by-liqdmetal` (stacked on `feature/dvm-l4-mapkeys`).*
