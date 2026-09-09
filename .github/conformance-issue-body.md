## Summary

Executable conformance suite for the DERO transaction relation + CI + malformed-point decode vectors. Tests-only (no consensus change, no production code touched).

## What's in it

| Test | What it verifies |
|---|---|
| `TestConformance_ValidProof` | deterministic NORMAL ring-16 tx, proof verifies end-to-end |
| `TestConformance_MutationsRejected` | 11 byte-mutations across header/statement/proof all rejected |
| `TestConformance_RingSizeMatrix` | build+verify+round-trip across ringsizes 2..128 |
| `TestG2P0_SameIndexAttack` | adversarial self-send (same ring index) rejection |
| `TestG2P0_FakeReceiverMutation` | fake-receiver statement-region flips rejected |
| `TestK0_RingSize2IsIdentifiable` | regression marker for the ringsize-2 signer-exposure finding |
| `TestConformance_Determinism` | statement/txid byte-determinism |
| `TestConformance_MalformedPointDecode` | **NEW** — compressed-point decode behavior for x≥p, non-residue, bad-flag inputs |

## The malformed-point vectors (why they matter)

A Rust differential harness (clean-room reimplementation, differentially tested against this codebase) found that **`bn256/changes.go` discards the on-curve error when `x >= p`** — Go "succeeds" with an invalid point while a strict decoder rejects. This is a **chain-split class bug** if a contract ever stores a malformed point (commitments, point-arithmetic outputs). The vectors pin both implementations: the assertion is determinism-only (no panic, same result per call) so Go and a strict Rust decoder can converge on the exact accept/reject rule.

Confirmed live by the vectors: `x_gt_p` and `bad_flag` inputs currently decode with `err = nil` in Go.

## CI

`.github/workflows/build-and-test.yml` — builds the tree + runs the conformance, DVM-intrinsic, and K0 suites on Go 1.25. The branches carry the re-vendored modules (`vendor/modules.txt` present) and the `KickReader` removal, so the tree builds from a fresh clone (no `-mod=mod` needed).

## Relationship to the other PRs

This is the "Submission C" conformance piece from the K0/DVM package. It strengthens all three:
- **#80** (Fix A, wallet warning) — the K0 regression marker
- **#82** (Fix B1, min-ring-4) — the ringsize-matrix + K0 vectors
- **#84** (DVM v9 intrinsics) — malformed-point vectors cover the commitment/point inputs the new intrinsics accept

---

*Branch: `feature/conformance-suite` in the fork `liqdmetal/derohe-improvements-by-liqdmetal`.*
