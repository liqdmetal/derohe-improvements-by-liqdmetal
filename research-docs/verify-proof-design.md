# verify_proof — ZK Verification in-VM (Native Hook Design)

**STATUS: ⚠️ DRAFT — DESIGN PROPOSAL. The Rust differential harness gate
is now OPEN (1,823 vectors, incl. proof_generate/proof_verify, byte-identical
with the Go reference). The Go-side native hook is designed here; PoC
implementation is the next step after community review. ⚠️**

> The end-state DVM primitive: a contract verifies a zero-knowledge proof
> inside the VM — "I know sk for a ring member without saying which" —
> the direction DERO gestures at with SIGNER() at ringsize 2. This design
> follows the P1-1 recommendation: a **native hook**, not VM-interpreted
> group arithmetic.

---

## 1. The gate is open (what changed)

The original defer ("blocked on the Rust harness") is resolved. The
clean-room Rust port (derohe-rs) now differentially verifies the aggregate
bulletproof against the Go reference:

```
PASS: all 1823 vectors match the Go reference
  proof_generate  15   proof_verify_trace  ...
  tx_build_valid   4   tx_serialize 5  tx_deserialize 41
```

`generate_proof`/`verify`/serialization are byte-identical in Rust. That
means: (a) the proof system's behavior is pinned by an independent
implementation, and (b) the same differential harness can test the Go-side
native hook before it's consensus-locked.

---

## 2. The Go verifier (what the hook wraps)

```go
// cryptography/crypto/proof_verify.go:98
func (proof *Proof) Verify(scid crypto.Hash, scid_index int, s *Statement,
                           txid crypto.Hash, extra_value uint64) bool
```

The proof is the full aggregate Bulletproofs statement proof — balance
conservation, range, membership — bound to the chain state via `Statement`
(ring, roothash, fees). This is consensus machinery: the verifier is
already what every node runs on every tx.

---

## 3. The intrinsic: `verify_proof(statement_hex String, proof_hex String) -> Uint64`

### 3.1 What a contract would do

A contract stores a *public statement* (e.g. an owner's membership claim)
and a *proof* (generated off-chain by the prover holding the witness).
Any caller can then prove "I satisfy this statement" without revealing
which ring member they are:

```
Function Prove(statement_hex String, proof_hex String) Uint64
    10  dim ok as Uint64
    20  LET ok = verify_proof(statement_hex, proof_hex)
    30  IF ok == 1 THEN GOTO 100
    40  RETURN 1              // proof invalid
    100 STORE("proved", 1)    // grant access / release funds
    110 RETURN 0
End Function
```

### 3.2 Design decisions

| Decision | Choice | Rationale |
|---|---|---|
| **Hook, not VM arithmetic** | native Go call into `Proof.Verify` | VM-interpreted group arithmetic would need a gas model for pairing; a native hook is version-locked like every intrinsic and reuses the audited verifier |
| **Inputs** | `statement_hex`, `proof_hex` (both serialized, hex) | The statement is public; the proof is public. No witness material enters the VM (nothing secret is ever in SCDATA) |
| **Binding** | contract must bind the statement to its own context | Same rule as verify_sig: the contract builds `message = domain || scid || args` and the statement must be anchored to the SC's own state (e.g. a stored commitment). Never verify a bare caller-supplied statement without binding |
| **Gas** | ~2,000,000 ComputeCost | Full Fiat–Shamir verification with pairings; must be benched (the Rust harness gives us a reference implementation to bench against) |
| **Version** | `>= 10.0.0` | A new DVM major — this adds pairing-level verification to the VM surface |

### 3.3 What the hook does NOT do (honest limits)

- **No arbitrary-circuit ZK** — this verifies *DERO's own* proof system
  (aggregate Bulletproofs over the statement relation), not general
  SNARK/STARK circuits. A contract can verify "this proof satisfies this
  DERO statement," not "this zk-SNARK is valid."
- **No proof generation** — only verification. Proving happens off-chain
  (the prover holds the witness).
- **Statement reconstruction** — the `Statement` struct has chain-bound
  fields (roothash, ring). The serialized statement the contract verifies
  must be *complete and self-consistent*; a malformed statement must
  return 0, never panic.

---

## 4. Changes required (diff-level)

| File | Change |
|---|---|
| `dvm/dvm_functions.go` | `func_table["verify_proof"]` (>= 10.0.0, ComputeCost ~2M), `dvm_verify_proof` handler |
| `dvm/sc.go` / `transaction_execute.go` | version-lock + hard-fork height |
| `cryptography/crypto/proof_verify.go` | expose a serialized-statement entry (or the handler deserializes) |
| `dvm/simulator.go` | simulator support (tests) |
| `tests/vectors/` | conformance: valid proof→1, tampered→0, malformed→0 (no panic), statement-binding |
| `derohe-rs harness` | differential-test the Go hook against the Rust verifier (the gate, now open) |

---

## 5. The use case that makes it worth a hard fork

The end-state K0 fix: **"prove you're the owner without saying which ring
member you are."** Today `SIGNER()` requires ringsize 2 and exposes the
sender. `verify_sig` (DVM v9) lets an owner authorize by Ed25519 signature
in encrypted payload — anonymous but *credential-based*. `verify_proof`
goes further: a statement like "the sender of this tx holds the secret key
of the owner address" verified in-VM, so an owner-gated contract can be
called **at ringsize ≥ 4 with true ZK authorization** — no signature
disclosed, no credential revealed, membership proven.

Combined with the K0 package (Fix A/B1/B2) and DVM v9, the ringsize-2
legacy dies completely.

---

## 6. Sequencing

1. **This design** (community review) — the statement/proof serialization
   format is the one open protocol question.
2. **PoC in the simulator** — `verify_proof` handler + a test that generates
   a proof with the Go prover and verifies it in-VM.
3. **Differential gate** — run the same vectors through the Rust verifier;
   both must agree.
4. **Gas bench** — the Rust reference gives us honest numbers for the ~2M
   estimate before consensus-lock.

*⚠️ DRAFT — design for review. The serialization format for
(statement, proof) into SCDATA hex is the key open question; the PoC will
settle it. ⚠️*
