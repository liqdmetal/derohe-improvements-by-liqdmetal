## Summary

**I4 from the intrinsic agenda (P0):** `verify_adaptor(pubkey_hex, message_hex, adaptor_sig_hex) -> Uint64` — Schnorr adaptor-signature verification in the VM, on **DERO's own bn256 curve** (no new dependencies). This is the cross-chain atomic settlement primitive (PTLC-style).

## What it enables

A Schnorr adaptor signature proves *"the holder of x (P = x·G) can produce a valid signature on m once a tweak t is revealed"* — **without revealing t**. Two chains share a pre-signed adaptor; whichever party reveals the tweak completes **both** transactions atomically. This is the settlement rail primitive for cross-chain atomic swaps:

- DERO contract holds an adaptor-signed commitment
- Counterparty (Bitcoin/Ethereum side) reveals the tweak to claim
- The tweak simultaneously completes the DERO leg — no oracle, no trusted third party, no HTLC timeout race

## Math

```
e = ReducedHash(R' || P || m)          (scalar mod bn256 order)
valid iff  s'·G == R' + e·P
```

where adaptor_sig = (s', R'): s' = r + e·x + t, R' = (r+t)·G. When the prover reveals t, the counterparty extracts the real signature: s = s' − t.

## Serialization

- `pubkey_hex` — 33-byte compressed G1
- `message_hex` — raw message bytes
- `adaptor_sig_hex` — 97 bytes = 64-byte big-endian s' + 33-byte compressed R'

## Consensus safety

- **Pure verification**: no witness material, no key recovery, no tweak disclosure enters the VM
- Gated `>= 10.0.0` (same HF window as verify_proof, L-series)
- Malformed input → 0, never panics
- 250k gas (verified: bn256 scalar mult + add + hash)

## Tests (`TestVerifyAdaptor`)

Builds a real Schnorr adaptor signature (same construction the intrinsic verifies) and asserts:
- valid adaptor → **1**
- wrong pubkey / wrong message / tampered s' → 0
- malformed (bad lengths, invalid points) → 0, no panic
- version gate: hidden from 9.0.0 contracts

Full dvm suite green.

## Relationship

- **Stacks on PR #84** (DVM v9 intrinsics) — same family, `>= 10.0.0` gate shared with verify_proof (#94)
- Complements `verify_sig` (Ed25519 auth) and `verify_proof` (ZK) — the three-verification toolkit for settlement contracts
- The DERO half of cross-chain atomic settlement: Fiat→DERO (Haveno/Bisq-style) and DERO↔BTC/ETH atomic swaps without oracles

---

*Branch: `feature/dvm-i4-verify-adaptor` in the fork `liqdmetal/derohe-improvements-by-liqdmetal` (stacked on `feature/dvm-v9-intrinsics`).*
