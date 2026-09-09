# DERO DVM / derohe Improvements — Condensed Change Notes

Everything shipped in this program, one line per change, with the effect.
Written for a coder who has never seen this work. Full detail lives in each
PR's body; this is the map.

**Fork:** `liqdmetal/derohe-improvements-by-liqdmetal` → PRs to `DEROFDN/derohe`
**Base:** `community-dev` (the active dev branch; `main` is release-only).
**License:** DERO Research License v1.1.2 (non-commercial) travels with all
Go code in this fork.

---

## Master table

| # | Package | Kind | Hard fork? | Effect (one line) |
|---|---------|------|-----------|-------------------|
| 1 | K0 privacy (A+B1+B2) | consensus | **yes** | ringsize-2 can no longer expose the signer on-chain |
| 2 | DVM intrinsics + SC-auth | consensus | **yes** | group arithmetic, signature/commitment verification, self-balance, adaptor sigs, owner auth |
| 3 | `verify_proof` (PoC) | consensus | **yes** | true ZK tx verification in-VM (native hook into `Proof.Verify`) |
| 4 | DVM language L1–L9 | consensus | **yes** | structured BASIC: loops, subroutines, arrays, mapkeys, bool, const, signed ints |
| 5 | Cross-contract calls (C6) | consensus | **yes** | a contract can call another contract (composability) |
| 6 | Supply hard-cap invariant | consensus | **yes** | assert 21M-DERO ceiling at coinbase; catches emission-curve regressions |
| 7 | Decoy selection (batch + sampler) | wallet/daemon | no | client-side ring selection; daemon posterior collapses to 1/C(B,R) |
| 8 | Conformance suite + CI | tests | no | reproducible correctness oracle + first CI gate |

**Totals:** 8 packages, 6 consensus (hard-fork) + 2 non-consensus.

---

## 1. K0 privacy — eliminate ringsize-2

**Problem:** a ringsize-2 transaction *is* sender + receiver. Parity selects the
sender (`Extract_signer`), so ringsize-2 exposes the signer on-chain by design.
56.8% of mainnet txs used ringsize 2.

- **Fix A** — `walletapi`: warn when a transfer resolves to ringsize 2, surface
  `PrivacyWarning` over RPC. *No fork, ships now.*
- **Fix B1** — `blockchain/transaction_verify.go`: from `K0_MIN_RING4_HEIGHT`,
  NORMAL/BURN txs with ringsize < 4 are rejected. Floor of 4 = sender + receiver
  + 2 decoys. Hardened against fork-boundary dodge and SC_TX type-lie bypass.
- **Fix B2** — `dvm/sc.go`: `NoSigner` bit in `SC_META_DATA` (33-byte wire
  format *unchanged*), auto-detected at install via AST scan
  (`ContractUsesSigner`). ringsize-2 SC calls + SC_INSTALL at ring < 4 rejected.
  Existing contracts default `uses_signer=true` — behavior preserved.

**Effect:** ringsize-2 has no legitimate remaining use. The 56.8% leak is
closed at consensus. One PR (#121), 17 files, ~1,000 lines, no vendor noise.

## 2. K0 Fix C — SC-auth via `verify_sig`

`walletapi/sc_auth.go`: Ed25519 SC-auth keypair + `SignSCData`
(domain:scid:entrypoint:args) + `VerifySCData`. Gated contracts authorize the
caller with a signature, so they can run at ringsize ≥ 4.

**Effect:** removes the last legitimate use of ringsize 2 / `SIGNER()`.
Depends on the v9 `verify_sig` intrinsic (package 3), so it's a separate PR.

## 3. DVM v9 intrinsics (6)

| Intrinsic | Gas | Effect |
|-----------|-----|--------|
| `verify_sig` | 250k | Ed25519 auth in encrypted SCDATA (no SIGNER/ringsize-2) |
| `hash_to_point` | 30k | map a value to a bn256 G1 point (self-contained Pedersen) |
| `pedersen_commit` | 45k | homomorphic commitment (confidential settlement) |
| `verify_commit` | 45k | verify a commitment against a revealed value+blind |
| `asset_balance` | 2k | SC reads its **own** stored balance (any asset) |
| `ec_add` | 15k | bn256 G1 point addition (homomorphic accumulation) |

**Effect:** the building blocks of confidential settlement and auth, all gated
`>=9.0.0` so pre-fork contracts can't accidentally use them.

## 4. `ec_mul` (I3)

`ec_mul(point_hex, scalar) -> point_hex` — bn256 G1 scalar multiplication,
30k gas, `>=9.0.0`. Uses a strict `x < p` point decoder (rejects off-curve
encodings that `DecodeCompressed` silently accepts).

**Effect:** homomorphic counterpart to `ec_add` (`ec_mul(c,2) == ec_add(c,c)`);
enables amount×rate computation for settlement and range proofs.

## 5. `verify_adaptor` (I4)

`verify_adaptor(pubkey_hex, message_hex, adaptor_sig_hex) -> 0/1` — Schnorr
adaptor-signature verification on bn256, 250k gas, `>=10.0.0`. Proves "the
holder of x can sign m once tweak t is revealed" **without revealing t**.
Strict point decode + low-s scalar (non-malleable).

**Effect:** the cross-chain atomic primitive — two chains share a pre-signed
adaptor; whoever reveals the tweak completes both. No new deps (uses the
existing bn256 + `crypto.ReducedHash`).

## 6. `verify_proof` (PoC)

`verify_proof(tx_hex, scid_index, ctx_hex) -> 0/1` — native hook into DERO's
`Proof.Verify`, ~2M gas, `>=10.0.0`. Panic-safe (`recover → 0`); ctx_hex carries
per-member `[ring point || CLn || CRn]` because tx `Serialize` strips the ring
and drops CLn/CRn.

**Effect:** true ZK tx verification in-VM — the end-state of K0 (ZK owner-auth
at ringsize ≥ 4 without leaking a key). **PoC only:** roothash binding and gas
metering are still open design questions before adoption.

## 7. DVM language L1–L9 (`>=10.0.0`)

| Item | Syntax | Effect |
|------|--------|--------|
| L1 | `FOR x = a TO b [STEP z]` / `NEXT`, `WHILE`/`WEND`, block `IF/ELSE/ENDIF` | structured loops replace GOTO spaghetti |
| L2 | `GOSUB <line>` / `RETURN` | shared helper reuse inside one contract |
| L3 | `DIM a(n) AS type`, `a[i]`, `arrlen(a)` | list semantics + loops over arrays |
| L4 | `mapkeys()` | enumerate SC state keys (batch/paged contracts) |
| L6 | `DIM b AS Bool`, `TRUE`/`FALSE`, `&&`/`||`/`!` | real boolean type |
| L7 | `CONST name = value` | immutable named constants |
| L8 | version gate | new syntax hard-requires `version("10.0.0")` |
| L9 | `DIM i AS Int`, negative literals, signed arithmetic | signed 64-bit ints (deltas, negative balances) |

**Effect:** one coherent "structured BASIC" upgrade. Arrays/Bool/Int are
Locals-only (never serialized → consensus-neutral). The gate means pre-fork
contracts can never accidentally parse new syntax. One PR, 14 files.

## 8. Cross-contract calls (C6)

`call_sc(scid_hex, entrypoint, arg1, val1, ...) -> 0/1` — nested DVM invocation
sharing the same tx (one gas budget, one commit point). Snapshot/rollback on
callee failure, recursion cap at 8, `SCIDSELF` + `Chain_inputs.SCID` switched
for the nested run (so LOAD/STORE hit the target's tree).

**Effect:** router/escrow/token become separate audited contracts that call each
other — no more monolith where one bug compromises everything. The composability
primitive the whole settlement stack needs.

## 9. Supply hard-cap invariant

`config.MAX_SUPPLY = 21M DERO` + `Verify_Transaction_Coinbase` asserts
`CalcSupply(height) <= MAX_SUPPLY` + overflow guard on the coinbase mint.

**Effect:** DERO's terminal supply is ~20.89M (0.51% under the cap). The assert
never trips on the real curve — it catches any regression in
`CalcBlockReward`/`BaseReward`/`PREMINE` that would mint past the cap. Pure
defensive invariant.

## 10. Decoy selection (batch RPC + sampler)

- **Batch RPC** — daemon `GetRandomAddressBatch` (≤512 real accounts with
  encrypted balances, one call, 5-block filter removed). Wallet draws the ring
  client-side with CSPRNG Fisher-Yates. **Effect:** no per-decoy round-trips;
  daemon's posterior collapses to 1/C(B,R).
- **Activity sampler** — `walletapi/decoy_sampler.go`: quantile model of
  observed mainnet activity; `SelectDecoys` weighted draw without replacement.
  **Effect:** decoy distribution tracks real activity (test tolerance ~±5%).

## 11. Conformance suite + CI

Test vectors + GitHub Actions + malformed-point (`x>=p`) vector.

**Effect:** reproducible correctness oracle + the first CI gate (upstream has
zero CI today).

---

## Build manifest (every PR carries this)

Upstream's `go.mod` is 3 lines with no `vendor/modules.txt`, so a fresh clone
doesn't build. The manifest (R151 `go.mod`/`go.sum`, jrpc2 v0.35.4) fixes it.
Build with: `GOFLAGS=-mod=mod GOTOOLCHAIN=local go build ./...`.

Also: `cmd/dero-wallet-cli/prompt.go` calls `KickReader()`, which no published
`readline` implements — removed (UI shim, not consensus).

## Gas table (all intrinsics)

| Intrinsic | Gas | Gate |
|-----------|-----|------|
| verify_sig | 250,000 | ≥9.0.0 |
| hash_to_point | 30,000 | ≥9.0.0 |
| pedersen_commit / verify_commit | 45,000 | ≥9.0.0 |
| asset_balance | 2,000 | ≥9.0.0 |
| ec_add | 15,000 | ≥9.0.0 |
| ec_mul | 30,000 | ≥9.0.0 |
| verify_adaptor | 250,000 | ≥10.0.0 |
| verify_proof | 2,000,000 | ≥10.0.0 |

## How to read a PR

Every consensus PR follows the same shape: code change → `>=X.0.0` semver gate →
test (incl. a wargame test that attacks the boundary) → the R151 build manifest.
Non-consensus PRs (decoy, conformance) have no gate and no manifest dependency.
