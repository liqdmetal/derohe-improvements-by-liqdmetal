## Summary

**I3 from the intrinsic agenda:** `ec_mul(point_hex String, scalar Uint64) -> String` — point scalar multiplication for the DVM, gated `>= 9.0.0`, 30k gas.

## What it does

Multiplies a compressed bn256 G1 point (DERO's 33-byte encoding) by a uint64 scalar, returning the compressed result. The **homomorphic counterpart to `ec_add` (I2, already in DVM v9 PR #84)**:

| Op | Property |
|---|---|
| `ec_add(c1, c2)` | c1 + c2 (commitment accumulation) |
| **`ec_mul(c, k)`** | k·c (point derivation, key blinding, commitment scaling) |

Together they give a contract **complete in-VM group arithmetic** on commitments — point derivation for key blinding, commitment scaling, and accumulation — without oracle dependence.

## Tests (`TestEcMul`)

- `ec_mul(c, 2) == ec_add(c, c)` — the homomorphic pair
- scalar composition: `ec_mul(ec_mul(c, 3), 2) == ec_mul(c, 6)`
- `ec_mul(c, 1) == c` (identity)
- `ec_mul(c, 0)` produces a valid point encoding
- output is always a 33-byte compressed point (66 hex)
- version gate: invisible to `1.2.3` contracts

## Hardening (wargame finding, now in the code)

**Strict point decoding — `strictDecodeG1`.** `ec_mul` (and its siblings `ec_add`, `verify_commit` in PR #84) decode the caller-supplied compressed point with `DecodeCompressed` + `err == nil`. The conformance PR (#86) documented that Go's decoder **accepts x ≥ p encodings** — it computes y from x mod p but stores the raw x. So a contract could feed an off-curve (x ≥ p) point and the intrinsic would compute on it, while a strict decoder (the clean-room Rust port) **rejects the encoding** → the two implementations diverge on contract behavior → **chain-split class bug**.

`strictDecodeG1` now validates `x < p` (canonical field encoding) before `DecodeCompressed`, applied to `ec_mul`, `ec_add`, and `verify_commit`. Non-canonical input panics (recovered → deterministic tx failure), matching the strict decoder. Canonical points are unaffected.

Pinned by `TestWargameEcMulAcceptsOffCurvePoint` (proves the fix: off-curve input now rejected, canonical input still accepted) and in the derohe-rs differential harness (`strict_point_decode`, 8 vectors).

## Relationship

- **Stacks on PR #84** (DVM v9 intrinsics) — same `>= 9.0.0` gate, same group-arithmetic family
- Completes the `ec_add`/`ec_mul` pair; the AMM-reserve/confidential-settlement use case is now fully expressible in-VM

---

*Branch: `feature/dvm-i3-ecmul` in the fork `liqdmetal/derohe-improvements-by-liqdmetal` (stacked on `feature/dvm-v9-intrinsics`).*
