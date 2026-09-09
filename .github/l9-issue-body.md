## Summary

**L9 from the DVM-BASIC language agenda (P2):** signed 64-bit integers — `DIM i AS Int`, negative literals, signed arithmetic and comparisons, Int function parameters and returns. Ends the two's-complement tricks contracts currently use for delta accounting.

## The syntax

```
DIM balance AS Int
DIM fee AS Int
LET balance = 100
LET fee = -30
LET balance = balance + fee      ' 70
IF balance - 100 == -30 THEN ... ' signed comparison
Function Diff(a Int, b Int) Int  ' negative returns
    RETURN a - b
End Function
```

## Implementation

- **`Int` = Vtype 0x7**, stored in `ValueInt64` (Go int64)
- **Negative literals**: `UnaryExpr{SUB}` handled in the evaluator
- **Signed arithmetic**: `evalBinaryExpr` gains an int64 path (ADD/SUB/MUL/QUO/REM + all comparisons); int64-vs-uint64 mixed operands reinterpret two's-complement so `IntVar == 70` works naturally
- Every type-switch site extended (DIM, LET, arrays, params, returns, CONST resolution)
- **Consensus-neutral**: Locals-only — contracts can't STORE an Int (STORE takes the uint64 scalar path), so serialized SC state is untouched
- Gated `>= 10.0.0` like the rest of the L-series

## Tests (`dvm/control_flow_l9_test.go`, 4 cases)

| Test | Verifies |
|---|---|
| `TestL9_IntBasics` | signed delta: 100 + (−30) = 70, balance−100 == −30 |
| `TestL9_NegativeLiteral` | −5, −x, x·−1 |
| `TestL9_IntParam` | Int param, negative return |
| `TestL9_VersionGate` | Int at 1.2.3 rejected |

Full dvm suite green (33 control-flow tests).

## Relationship

- **Stacks on L6** (PR #115) — same type-system extension pattern
- L-series now **8/10 delivered** (L1–L4, L6–L9); remaining: L5 (multi-value returns) and L10 (block scoping), both structural

---

*Branch: `feature/dvm-l9-int` in the fork `liqdmetal/derohe-improvements-by-liqdmetal` (stacked on `feature/dvm-l6-bool`).*
