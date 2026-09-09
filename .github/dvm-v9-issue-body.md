## Summary

**Hard-fork proposal:** five new DVM-BASIC intrinsics gated to a new DVM version (semver >= 9.0.0), turning DERO smart contracts from "oracle-dependent commitments + ringsize-2-only authorization" into "self-contained confidential settlement." Existing contracts are unaffected — the func_table `Range` mechanism hides new functions from old DVM versions.

- `verify_sig(pubkey_hex, message, sig_hex) -> Uint64` — Ed25519 in-VM signature verification
- `hash_to_point(input) -> String` — deterministic hash-to-curve (protocol generator derivation)
- `pedersen_commit(value, blind_hex) -> String` + `verify_commit(value, blind_hex, commit_hex) -> Uint64` — Pedersen commitments with 256-bit-hiding blind
- `asset_balance(asset_hex) -> Uint64` — read the SC's own stored balance for any asset
- `ec_add(p1_hex, p2_hex) -> String` — homomorphic point addition

## Why these, in order

### 1. `verify_sig` — the missing half of the K0 fix
The only in-VM authorization primitive today is `SIGNER()`, which requires ringsize 2 (`blockchain/transaction_execute.go`) and therefore exposes the sender on-chain. A contract storing a public key and checking `verify_sig(pubkey, msg, sig)` in SCDATA lets callers prove key ownership **anonymously at ringsize >= 4**. This is the "Fix C" path for owner-gated contracts (TransferOwnership, UpdateCode, escrow redemption) — authorization without the anonymity cost.

### 2. `hash_to_point` + `pedersen_commit`/`verify_commit` — oracle-free commitments
The DVM currently expects Pedersen commitments to arrive as external oracles (`dvm_functions.go` — the comment documents this as "needs more investigation"). These make commitments a first-class primitive: commit on-chain, reveal off-chain, SC verifies — no trust in the caller's commitment. Generator derivation matches the protocol (`HashToPoint(HashtoNumber(PROTOCOL_CONSTANT+"H"))`), so commitments are compatible with the existing proof system's generators.

### 3. `asset_balance` — the stablecoin/settlement primitive
Verified gap: `derovalue()`/`assetvalue()` only report the value arriving *in the current tx* (`dvm.State.Assets`). The chain persists SC asset balances (`LoadSCAssetValue`/`StoreSCValue`), but **no intrinsic can read them** — a contract can't know its own DERO or asset holding before deciding to pay out. This blocks asset-denominated settlement (stablecoins, tokens) entirely.

### 4. `ec_add` — homomorphic accumulation
Pedersen commitments are homomorphic, but DVM-BASIC has no point arithmetic — you can't add two compressed points. `ec_add` enables updating a stored commitment by a delta without revealing it (the exact pattern for confidential AMM reserves, batched settlement): `ec_add(c1, c2) == pedersen_commit(v1+v2, b1+b2)`, verified by test.

## Gas & versioning

| Intrinsic | ComputeCost | Rationale |
|---|---|---|
| `verify_sig` | 250,000 | 2 scalar-mults + hashing (conservative; tune after bench) |
| `hash_to_point` | 30,000 | one G1 scalar-mult |
| `pedersen_commit`/`verify_commit` | 45,000 | 2 scalar-mults + compare |
| `asset_balance` | 2,000 | tree read |
| `ec_add` | 15,000 | one G1 add |

All gated `semver >= 9.0.0` — a new DVM version; existing contracts see no change.

## Tests (dvm/verify_sig_test.go, 6 functions + Fix C wallet half)

- `verify_sig`: valid→1, tampered sig/message→0, wrong key→0, malformed→0 (no panic), version gate
- `hash_to_point`: determinism, distinct-input separation, valid 33-byte point
- `pedersen_commit`/`verify_commit`: determinism, correct reveal→1, wrong value/blind/commit→0, hiding, binding
- `asset_balance`: reads SC's own balance via BalanceLoader(scid, asset), version gate
- `ec_add`: **homomorphic property** (ec_add(c1,c2)==c3), commutativity, valid point, version gate
- Fix C wallet half: `walletapi/sc_auth.go` (SCAuthKey helper) + `dvm/fixc_auth_test.go` — end-to-end owner auth via `verify_sig`: attacker key rejected, owner authorized, tampered signature rejected

## Security notes

- `verify_sig` must be non-malleable: contract binds the signed message to the call context (`domain || txid || args`), never signs bare txids. Secret keys never enter the VM.
- `verify_sig` deliberately works on **public** keys the contract stores — nothing that reveals a caller's key.
- Commitments only hide if the blind is high-entropy (32-byte CSPRNG); documented as a production rule, not enforced by the VM.
- **Strict point decoding (wargame hardening):** all caller-supplied compressed points (`ec_add`, `verify_commit`) are validated with `strictDecodeG1` — x must be < p (canonical field encoding). Go's lenient `DecodeCompressed` accepts x ≥ p encodings (computing y from x mod p); a strict decoder (the clean-room Rust port) rejects them. Without the strict check, a contract could feed an encoding one implementation accepts and the other rejects → chain-split class bug. The boundary is pinned in the derohe-rs differential harness (`strict_point_decode`, 8 vectors). The I3 PR applies the same fix to `ec_mul`.

## Consensus implications

New intrinsics change VM state output → **hard fork** (DVM version bump to 9.x). Ship as part of the next scheduled HF; the version gate means old contracts keep running byte-identical.

## Not included (deliberately)

- `verify_proof` (ZK verification in-VM) — a much larger undertaking; the native-hook design is documented but deferred.
- `block_hash`, `verify_merkle`, `FOR` loops, cross-contract calls — separate proposals.
- Anti-proposals explicitly out of scope: floats/big-int (determinism), `balance_of(address)` (leaks other accounts' encrypted balances), WASM/bytecode VM.

---

*Branch: `feature/dvm-v9-intrinsics` in the fork `liqdmetal/derohe-improvements-by-liqdmetal`. Carries the build fixes: re-vendored modules (`vendor/modules.txt` present) and the `KickReader` removal, so the tree builds from a fresh clone.*
