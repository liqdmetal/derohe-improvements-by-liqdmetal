# DERO DVM / derohe / DVM-BASIC — Improvement Agenda

**STATUS: ⚠️ DRAFT — consolidated from source-verified analysis of
derohe-Release151. Companion to `dvm-basic-improvements.md`,
`k0-fix-design.md`, `decoy-selection-batch-rpc.md`,
`dero-submissions-package.md`. To be pushed to the derohe fork repo on
the user's go-ahead. ⚠️**

> Every claim below is grounded in the derohe-Release151 tree with
> file:line references. Items marked ✅ are implemented and tested in the
> working tree; others are designed, ranked, and ready to build.
>
> This is a DERO-ecosystem agenda (the product framing was scrubbed).

---

## 0. Corrections locked in from source verification

| # | Correction | Evidence |
|---|---|---|
| 1 | `dero()` returns the **zero-hash / DERO asset ID**, NOT the contract's balance | `dvm_functions.go:291-295` |
| 2 | **No intrinsic reads the SC's own stored balance** — `derovalue()`/`assetvalue()` return only the value arriving *in the current tx* (`dvm.State.Assets`), never the persisted holding | `dvm_functions.go:441-461`; stored balances persist via `LoadSCAssetValue`/`StoreSCValue` (`sc.go:208-212,340`) but are unreadable in-VM |
| 3 | `update_sc_code` is **ungated** — source comment literally says `// TODO verify code authenticity how` | `dvm_functions.go:319-330` |
| 4 | Parser **does** support `IF … THEN GOTO x ELSE GOTO y` (richer than earlier notes) | `dvm.go:731,746` |
| 5 | `GetGasEstimate` RPC **already exists** — not a gap | `cmd/derod/rpc/rpc_dero_estimategas.go:39` |
| 6 | **No cross-contract calls**, **no map/key iteration**, **no historical block hash** — all absent | greps of `dvm/`, `blockchain/` |
| 7 | The `SCIDSELF` field comment anticipates cross-SC calls: *"this separation is necessary, if we enable cross SC calls"* | `dvm/dvm.go:410` |

---

## 1. Language-level improvements (DVM-BASIC grammar)

| # | Improvement | Gap it fills | Priority |
|---|---|---|---|
| L1 | **Structured control flow**: `FOR`/`NEXT`, `WHILE`, block `IF/ELSE/ENDIF` | GOTO-only loops today; error-prone, every expression 800 gas | **P0 ✅ IMPLEMENTED** (PR #98+; gated `>=10.0.0`) |
| L2 | **Subroutines / internal functions** (`GOSUB`/`RETURN` or named blocks) | No code reuse inside a contract; every entrypoint re-implements helpers | **P0** |
| L3 | **Arrays / indexed maps first-class** | `mapstore` works but keys are caller scalars; no list semantics | P1 |
| L4 | **Map enumeration** (`mapkeys(scid) -> list`) | No way to iterate stored keys — batch/paged contracts hand-roll key sets | **P1** |
| L5 | **Multi-value returns** | Entrypoints return Uint64 only | P2 |
| L6 | **Real boolean type** | Comparisons return uint64; explicit Bool aids audit | P2 |
| L7 | **Constants / `CONST`** | Magic numbers everywhere | P2 |
| L8 | **`version()` pragma auto-gate** | Manual `version("x.y.z")` call today | P2 |
| L9 | **Signed integers** | Uint64 only — delta accounting forces two's-complement tricks | P2 |
| L10 | **Block-scoped variables** | Storage is global via STORE/LOAD; no local state | P3 |

**Biggest wins: L1 + L2** — halve contract size (cuts the `len(scdata)*1.5`
code-size fee term), make contracts auditable, and reduce GOTO spaghetti.

---

## 2. New intrinsics

### 2a. High-value, missing today (verified)

| # | Intrinsic | Signature | Gap filled | Est. gas | Priority |
|---|---|---|---|---|---|
| I1 | **`asset_balance`** | `asset_balance(asset String) -> Uint64` | SC can't read its own stored balance in any asset (verified gap #2) | 2,000 | **P0 ✅ IMPLEMENTED** |
| I2 | **`ec_add`** | `ec_add(p1 String, p2 String) -> String` | No point arithmetic -> no homomorphic accumulation of commitments | 15,000 | **P0 ✅ IMPLEMENTED** |
| I3 | **`ec_mul`** | `ec_mul(point String, scalar Uint64) -> String` | Scalar mult for point derivation, key blinding | ~30k | P1 |
| I4 | **`verify_adaptor`** | `verify_adaptor(pubkey, adaptor, tweak) -> Uint64` | Cross-chain atomic via adaptor completion in-VM | ~250k | **P0 (design)** |
| I5 | **`verify_merkle`** | `verify_merkle(root, leaf, proof) -> Uint64` | Provenance, model binding | ~20k | P1 |
| I6 | **`block_hash`** | `block_hash(height) -> String` | No historical block data; timelocks, cross-chain anchors | ~20k | P1 |
| I7 | **`hmac`** | `hmac(key, msg) -> String` | Keyed commitment, authenticated hashing | ~25k | P1 |
| I8 | **`hkdf`** | `hkdf(secret, info, len) -> String` | In-contract key derivation | ~30k | P1 |
| I9 | **`tx_fees`** | `tx_fees() -> Uint64` | Fees are plaintext in the statement but not exposed to SC | ~1k | P2 |
| I10 | **`verify_proof`** | native hook `-> Uint64` | ZK verification in-VM; groundwork = Rust proof_verify differential harness | ~2M | P1 |

### 2b. Designed earlier, refined
- `verify_sig` (P0-1) — Ed25519 first; **spec the bn256-Schnorr variant now** so reviewed curve code can back it later.
- `hash_to_point` (P0-2) — **pin to `algebra_pedersen.go:36-37`** derivation (`HashToPoint(HashtoNumber("DERO"+"G"/"H"))`) to match protocol generators.
- `pedersen_commit`/`verify_commit` (P0-3) — same generator pinning.

### 2c. Anti-proposals (do NOT add)
- **`balance_of(address)`** — leaks another account's encrypted balance; breaks the privacy model.
- **`random` with caller seed** — breaks determinism.
- **Floats / big-int in-VM** — consensus determinism non-negotiable.
- **WASM/bytecode VM** — intrinsic table + structured flow covers the surface; JIT of the same AST is the safe speed path.
- **Anything returning a private key** — destroys anonymity.

---

## 3. Consensus / protocol-level improvements

| # | Improvement | Gap | Fork? | Priority |
|---|---|---|---|---|
| C1 | K0 Fix A (wallet ringsize-2 warning + `PrivacyWarning` RPC) | 56.8% of txs at ringsize 2 expose sender | no | ✅ done, ship |
| C2 | K0 Fix B1 (min-ring-4 floor NORMAL/BURN) | ringsize-2 dominance is protocol flaw | yes | ✅ done |
| C3 | K0 Fix B2 (contract registry `uses_signer`) | ringsize-2 SC calls only when needed | yes | designed |
| C4 | K0 Fix C (verify_sig owner auth) | owner-gated contracts shouldn't cost anonymity | yes | ✅ done (I-verify_sig) |
| C5 | Decoy batch RPC + client-side selection | daemon learns ring + 5-block narrowing | no | designed |
| C6 | **Cross-contract calls** | No composability — biggest DeFi gap (verified absent) | yes | **P0 (design in `cross-contract-calls.md`)** |
| C7 | **`update_sc_code` ownership gating** | Ungated in source (`// TODO verify authenticity how`) | yes | **P0** |
| C8 | Contract registry / interface standard | No discoverability, no standard interfaces | yes | P1 (with C6) |
| C9 | Dandelion++ / propagation privacy | First-seen/IP timing metadata leaks | no | P2 |
| C10 | Fee privacy | Fees are plaintext | yes | P2 |

---

## 4. Privacy / analysis follow-through (K-series)

| # | Improvement | Gap | Status |
|---|---|---|---|
| P1 | Activity-matched decoy sampling | Uniform sampling != real activity distribution (OSPEAD analog) | research |
| P2 | Ghost-account / zero-balance decoy rejection | Unregistered accounts silently fabricated (`daemon_communication.go:419-427`) | fold into batch RPC |
| P3 | **Malformed-point conformance vectors** | Rust harness found `x >= p` decode is UB in Go, Rust rejects — chain-split class bug if a contract stores a malformed point | **do now** |
| P4 | Spec gaps G1–G10 | transcript byte-order, anon-set statement, HF3 whitelist | in progress |

---

## 5. The Rust port (derohe-rs) — what it unlocks

2,890 differential vectors pass on keccak/bn256/hash-to-point/ElGamal;
**1 consensus-relevant divergence found** (`x >= p` compressed-decode).
Phase 2+ targets `algebra_pedersen`, proof verify, `balance_serdes`, the
DVM interpreter.

**Strategic reframe: every DVM improvement is a spec the Go AND Rust
implementations must both satisfy.**
- Conformance suite becomes the Rust DVM's test oracle.
- `verify_proof` (I10) is blocked on the Rust harness.
- Bench intrinsics in Rust to calibrate gas before consensus-lock.
- Pin the DVM-BASIC grammar end-state now (L1–L3) so the Rust interpreter
  targets the final language, not today's GOTO-only subset.

---

## 6. Tooling / dev-experience

| # | Tool | Why |
|---|---|---|
| T1 | DVM-BASIC language server | Dev experience unlock for contract authoring |
| T2 | Static analyzer / linter | Catches classic SC bugs pre-deploy (unused vars, unbounded GOTO, missing version gate, reentrancy) |
| T3 | Higher-level compiler -> DVM-BASIC | Safer authoring surface, emits auditable BASIC |
| T4 | Fuzzing harness for the DVM | Random code/args; find panics (malformed-point class) |
| T5 | Formal grammar spec | The Rust DVM needs it; Go side has none written down |

---

## 7. Implementation status (working tree, derohe-Release151)

| Item | Status |
|---|---|
| `verify_sig`, `hash_to_point`, `pedersen_commit`, `verify_commit` | ✅ implemented + tested |
| **`asset_balance`, `ec_add` (I1, I2)** | ✅ **implemented + tested this pass** |
| K0 Fix A (wallet/RPC/CLI) | ✅ 4 tests |
| K0 Fix B1 (min-ring-4) | ✅ 3 tests |
| Conformance suite | ✅ 8 tests |
| std-size v151 HTLC template + HTLC v4 | ✅ tested |
| Cross-contract calls (C6) | design in `cross-contract-calls.md` |

*⚠️ DRAFT — all `file:line` references verified against
derohe-Release151 at write time. ⚠️*
