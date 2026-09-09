## Summary

**PoC implementation** of the P1-1 `verify_proof` intrinsic — ZK proof verification inside the DVM, as a native hook into the audited Go `Proof.Verify` (NOT VM-interpreted group arithmetic). Follows the design in issue #92.

## The intrinsic

```
verify_proof(tx_hex String, scid_index Uint64, ctx_hex String) -> Uint64
```

- **`tx_hex`** — a fully-serialized transaction (carries the statement's C/D/pointers/roothash + the proof)
- **`ctx_hex`** — the expanded statement material DERO's serialization deliberately omits (ring/CLn/CRn are "expanded from graviton store"): per ring member `[ring key (33B) | CLn (33B) | CRn (33B)]` concatenated, 99·N bytes
- The VM splices the context into the deserialized statement and runs the **same `Proof.Verify` nodes run on every tx** (`transaction_verify.go:482`)

## Known limitation — chain-binding gap (wargame finding, pinned by test)

The intrinsic verifies a bulletproof against a **fully caller-supplied context**: the tx (with its Roothash/Fees/BurnValue), the payload index, and the expanded ring. It performs **zero chain-state lookups**:

- The on-chain verifier requires the statement's `Roothash` to equal the chain's actual merkle root at the referenced snapshot (`transaction_verify.go:424`) and expands the ring/balances from the balance tree (`transaction_verify.go:429+`). The intrinsic has no chain handle, so it can do neither.
- **Result:** `verify_proof` proves only that *a bulletproof exists that is internally consistent with the caller-supplied statement*. A fabricated-but-self-consistent proof — random roothash that has never been a chain root, decoy ring members with made-up encrypted balances — verifies as **1**.
- **Pinned by `TestVerifyProofChainBindingGap`** (`dvm/wargame_verify_proof_test.go`): a never-mined tx with fabricated state verifies as 1; a genuinely tampered proof still returns 0 (crypto binding intact; it is the *chain* binding that is missing).

**Fix direction (required before this is production-grade):** the intrinsic must be a daemon-side hook that resolves `tx.BLID`'s snapshot, checks the merkle root against the actual chain state, and expands the ring from the balance tree — i.e. exactly the binding the on-chain verifier applies. As specified here it is a tautology machine, and a contract gating a payout on `verify_proof()==1` could be triggered by a fabricated tx. This PoC establishes the VM plumbing, serialization format, and gas shape; the chain-binding hook is the follow-up.

## Why this serialization format

DERO's tx serialization strips the expanded statement (ring, CLn, CRn) to truncated pointers + C/D — the verifier needs the real points, which nodes normally reconstruct from the graviton balance tree. The context blob is the contract's way of supplying exactly what it expects. These are public values; nothing secret enters SCDATA. (Note: this caller-supplied expansion is precisely why the chain-binding hook above is mandatory — without it the "context" is attacker-chosen.)

## Safety

- **Panic-safe**: any internal failure (malformed input reaching a panicking decode path) returns 0 — a consensus primitive never panics out of the VM.
- Malformed tx/context/index → 0.
- Version-gated `>= 10.0.0` (new DVM major), gas 2,000,000 (benchmarked shape — the Rust reference implementation in derohe-rs can calibrate this).

## Tests (`dvm/verify_proof_test.go` + `dvm/wargame_verify_proof_test.go`)

`TestVerifyProof` builds a valid NORMAL tx (aggregate bulletproof, ring 16) and asserts:
- valid tx + correct context → **1**
- wrong context → 0 · context size mismatch → 0 · tampered tx → 0 · malformed hex → 0 · bad index → 0
- version gate: hidden from < 10.0.0 contracts

`TestVerifyProofChainBindingGap` pins the known limitation above (fabricated-state tx accepted, tampered rejected, crypto self-consistency shown).

## Honest scope

- Verifies **DERO's own proof system** (aggregate Bulletproofs over the statement relation), not arbitrary zk-SNARK/STARK circuits.
- **No proof generation in-VM** — proving happens off-chain (the prover holds the witness).
- The end-state K0 fix: "prove you're the owner without saying which ring member you are" — owner-gated contracts at ringsize ≥ 4 with true ZK authorization, no credential disclosed.
- **Not yet safe to gate value on**: the chain-binding hook (above) must land first.

## Relationship

- Design: issue #92 (updated with the settled serialization format)
- Builds on the Rust differential harness gate (2,539 vectors incl. proof_generate/proof_verify + the K0 rules, byte-identical with Go)
- Composes with the K0 package (Fix A/B1/B2) and DVM v9 — ringsize 2 dies completely

---

*Branch: `feature/verify-proof-poc` in the fork `liqdmetal/derohe-improvements-by-liqdmetal`. Carries the build fixes: re-vendored modules (`vendor/modules.txt` present) and the `KickReader` removal.*
