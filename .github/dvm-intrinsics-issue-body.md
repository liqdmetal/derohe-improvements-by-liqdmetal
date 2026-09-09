## Summary

**One reviewable PR replacing three stacked PRs (#84, #105, #113).** The
consolidated DVM intrinsics: group arithmetic + verification primitives.

## What's in it

| Intrinsic | Gas | Gate | What it does |
|-----------|-----|------|--------------|
| `verify_sig` | 250k | ≥9.0.0 | Ed25519 signature auth in encrypted SCDATA (no SIGNER/ringsize-2) |
| `hash_to_point` | 30k | ≥9.0.0 | map a value to a bn256 G1 point (self-contained Pedersen) |
| `pedersen_commit` | 45k | ≥9.0.0 | homomorphic commitment (confidential settlement) |
| `verify_commit` | 45k | ≥9.0.0 | verify a commitment against revealed value + blind |
| `asset_balance` | 2k | ≥9.0.0 | SC reads its **own** stored balance (any asset) |
| `ec_add` | 15k | ≥9.0.0 | bn256 G1 point addition (homomorphic accumulation) |
| `ec_mul` | 30k | ≥9.0.0 | point scalar multiplication (homomorphic pair of ec_add) |
| `verify_adaptor` | 250k | ≥10.0.0 | Schnorr adaptor-signature verification (cross-chain atomic) |

## Design notes (why it's safe)

- Every intrinsic gated `>=9.0.0` / `>=10.0.0` — pre-fork contracts can't call
  them.
- `ec_mul` and `verify_adaptor` use **strict x<p point decode** — rejects
  off-curve encodings that `DecodeCompressed` silently accepts (would be a
  chain-split vector).
- `verify_adaptor` enforces **low-s** (non-malleable: rejects s ≥ group order).
- `verify_adaptor` uses the existing bn256 + `crypto.ReducedHash` — zero new
  dependencies (Go stdlib has no secp256k1).

## Tests

verify_sig (unit + version gate), ec_mul (homomorphic pair, composition,
identity), verify_adaptor (valid / wrong-key / tamper / malformed), pedersen,
hash_to_point, asset_balance, ec_add, + 2 wargame tests (scalar malleability,
non-canonical point).

## Supersedes

Closes #84, #105, #113 in favour of this single package.
