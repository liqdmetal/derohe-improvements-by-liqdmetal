# Seamless HF4 Tokenomics and Late-Migration Rollout

**Status:** Historical initial plan; superseded by the exact-value design and verification record in `docs/seamless-migration.md`. No mainnet activation, transaction, or state change has occurred; HF4 remains unscheduled.

> **Current decision:** Each call carries one non-zero native-DERO input and forwards exactly `DEROVALUE()`. No net-new credit, global budget/cap, claim pool, cumulative-credit counter, or nullifier is used. Budget/cap items and commands below are archived, not current instructions; use the design note for verified status. Keep total test/build concurrency at or below 12.

## Historical outcome draft

HF4 should not force every eligible holder to migrate in the activation block. A migration entrypoint remains available after activation, and every migration is bounded by a separately authorized budget. State and counters must survive the fork, remain deterministic through reorgs, and fail closed when the budget is exhausted.

This is a rollout design, not authorization to mint, claw back, burn, or compensate any DERO.

## Current facts and constraints

- Mainnet HF4 is currently fail-closed and unscheduled (`HeightUnscheduled`, `config/config.go:127`). Keep it that way until the accounting model, tests, review, and activation decision are approved.
- `blockchain.Add_Complete_Block` loads the parent snapshot's balance and `SC_META` trees, loads contract data trees, then commits the updated trees with the block (`blockchain/blockchain.go:1002–1126`). This supports state continuity, but is **not** an end-to-end proof that a migration contract remains callable across HF4.
- `GetInfo`'s `total_supply` is emission-schedule based; `spendable_supply` is a height-based estimate that adds the pinned forensic floor after HF3. Neither is a balance census. The record reports a net-created floor of 18,886,517.3 DERO, but the burn attribution is unproven and the amount is not proof of coins still outstanding (`docs/forensics/DERO-GETINFO-SUPPLY.md`). Do not use this floor as an eligible migration pool or cap.
- The DVM can receive DERO and transfer from a contract's balance; a smart contract cannot mint base-layer DERO. If migration produces net-new native DERO credit, consensus code—not a contract-only counter—must authorize and enforce it.
- Current DVM tests embed contract source in Go tests (`dvm/simulator_test.go`). The DVM simulator tests contract execution but does not itself mine across a fork. `cmd/simulator` has a local chain/mining harness suitable for the boundary test.
- The working tree on branch `2027/foundation` already has broad uncommitted changes (28 tracked paths plus untracked forensic-supply work). Preserve them. Do not reset, stash, stage, commit, or overwrite them as part of this plan.
- Execution remains local-only: no fee-paying mainnet transaction, no external network calls, no secrets or personal data in fixtures or presentation material. Use the approved explicit Go binary, `MSYS2_ARG_CONV_EXCL='*'`, `GOFLAGS=-mod=mod`, and no more than 12 build/test threads.

## Accounting rule: decide before coding

Name and keep separate these values:

- `E(H)`: consensus emission schedule at consensus height `H`, from the reviewed emission function (`CalcSupply`).
- `F`: evidence-bounded forensic created-supply floor. It is telemetry, not a current-balance census.
- `B`: migration budget authorized by evidence and the fork rules.
- `P(H)`: cumulative **net-new native DERO credit** caused by migration by height `H`. Pure movement of already-existing coins is not net-new credit.

The release must select and document one of these modes:

1. **Escrow/pass-through only:** each payout is backed by DERO actually received and locked/consumed as the source of that payout. Enforce one-time source use and value conservation. The contract cannot make the original coins remain spendable elsewhere and also create an equivalent payout.
2. **Net-new native credit:** consensus defines a deterministic remaining allowance and atomically reduces it for each accepted migration. The accounting must show how the allowance relates to `E(H)` and the already-recognized ledger supply. Do not infer that value from `GetInfo.spendable_supply`, `F`, or a contract's view of its own balance.

If the chain cannot compute or enforce the required global allowance deterministically, **disable net-new credit**; only a verified source-backed pass-through may proceed. `P(H) <= E(H)` by itself is insufficient because scheduled supply already includes coins represented elsewhere on the chain.

## Historical rollout stages (superseded; do not execute as written)

### Stage 0 — Preserve and establish the baseline

1. Record the current branch and complete dirty-tree inventory; separate existing work from migration-specific work without discarding anything.
2. Review the existing diff in the HF schedule, supply accounting, transaction/SC execution, and RPC reporting before making additional edits.
3. Establish focused local test results for the untouched baseline; identify tests with network or filesystem side effects before running them. Stop if dependencies would require an unapproved network fetch.

**Gate 0:** No destructive git operation; existing forensic-supply work is understood and preserved.

### Stage 1 — Approve migration semantics and budget

1. Decide whether this is escrow/pass-through or net-new native credit. Identify the source value and proof for every eligible claim.
2. Write the invariant, in atomic units, including the exact height convention and when a migration is counted. Specify how source coins become unavailable for duplicate use.
3. Set `B` only from separately reviewed evidence and a consensus-safe calculation. The currently reported forensic floor is not enough to set `B`.
4. Decide the registration/privacy and anti-Sybil policy. Do not assume a public wallet register or a 1,000-DERO per-account burn: a burn does not prove eligibility and could exclude legitimate holders. If Spore bulletins are used, keep them as signed, versioned communications—not a consensus dependency or a place to publish claim secrets, personal data, or wallet-to-identity mappings.
5. Decide whether the contract is pre-installed before HF4 or installed by a deterministic HF4 mechanism, and define any emergency pause authority. No general owner-controlled code upgrade or drain path should bypass the approved cap.

**Gate 1:** Written accounting model, evidence boundary, claim proof, budget, privacy policy, deployment method, and emergency behavior are approved. No budget means no net-new credit.

### Stage 2 — Build the contract and consensus guard

1. Add a DVM test contract and tests, following the existing embedded-contract convention (`dvm/migration_contract_test.go`). Keep the migration entrypoint open after HF4; do not impose an activation-day-only deadline. Claims stop only when the approved budget is depleted or a pre-approved safety pause is invoked.
2. Track cumulative migrated value and a one-time nullifier for each eligible source claim. Check `amount <= remaining` before mutating state or scheduling transfers; use checked arithmetic. Make failed, duplicate, malformed, or over-budget calls leave both contract state and balances unchanged/refunded according to DVM semantics.
3. Audit the DVM incoming-value path (`dvm/sc.go:245–250`) and the SC transfer/revert path for checked accumulation, atomicity, and exact refunds. Verify the relevant bounds rather than assuming the contract's own arithmetic is the only accounting boundary.
4. For net-new native credit, implement a consensus-side allowance check in the same state transition that accepts the migration. Validate it again during block execution; a mempool check is not authoritative. Ensure ordinary miner rewards and migration credits share a documented supply invariant.
5. Make the contract identity/code hash and state schema explicit. Prevent an alternate contract deployment or update path from multiplying or bypassing the global cap.

**Gate 2:** Unit tests show source/value conservation, one-time claims, checked cap behavior, and rollback on every failed execution path.

### Stage 3 — Prove fork continuity and reorg behavior

Add an integration test using `cmd/simulator`'s local chain harness, preferably in `cmd/simulator/migration_fork_test.go`:

1. Install the migration contract and set non-zero state before the fork boundary; verify its SCID, code, metadata, stored counter, and escrow balance.
2. Mine the block immediately before HF4, the activation block, and blocks after activation. Read the same state from the canonical snapshot and call the contract at `HF4+1` and at a later height.
3. Test migration at the exact remaining budget, then one unit over; verify the latter cannot change the counter, escrow, or recipient balance.
4. Reorg across the activation boundary and across a successful migration. The winning branch must restore the correct counter and balances from its snapshot; no process-local counter may survive incorrectly.
5. Compare deterministic state roots/results across independent local simulator runs.

Also test pre-/at-/post-activation emission boundaries and the mainnet unscheduled case. The DVM simulator alone is not sufficient evidence for fork persistence.

**Gate 3:** The same contract remains callable after HF4, late calls work, all accounting is deterministic, and reorgs restore the exact pre-branch state.

### Stage 4 — Adversarial review and release candidate

Run a focused adversarial matrix:

- exact budget, zero value, maximum valid value, over-budget, counter boundary, integer overflow, malformed arguments, and wrong asset;
- duplicate nullifier, replayed claim, multiple contracts attempting the same global budget, and any legacy transaction replay path;
- contract execution failure after value receipt, failed send, and repeated retries; verify atomic refund/no double credit;
- side-chain/reorg, activation-height off-by-one, unscheduled HF4, simulator/mainnet gating, and clean restart from persisted snapshots;
- code-update/owner/pause authority cannot raise the cap or drain user escrow.

Update `docs/forensics/DERO-GETINFO-SUPPLY.md` only if behavior changes; keep telemetry clearly distinguished from consensus and migration accounting. Add a separate community-facing design note, e.g. `docs/forensics/DERO-MIGRATION-ROLLOUT.md`, with the invariant, evidence scope, claim rules, limitations, and rollback policy. Use synthetic fixtures and redact all credentials and personal data.

Run focused tests first, then the broader suite after checking for network-dependent tests. At execution time use the approved explicit Go executable:

```sh
MSYS2_ARG_CONV_EXCL='*' GOMAXPROCS=12 GOFLAGS=-mod=mod "$GO_EXE" test -p 12 -parallel 12 ./dvm ./blockchain ./forensicsupply ./rpc ./cmd/simulator
MSYS2_ARG_CONV_EXCL='*' GOMAXPROCS=12 GOFLAGS=-mod=mod "$GO_EXE" test -p 12 -parallel 12 ./...
```

Review `git diff --check`, `git status --short`, and `git diff -- go.mod go.sum` after test/build commands. Do not stage, commit, or push without a separate explicit instruction.

**Gate 4:** Tests pass on the reviewed source tree; reviewers can reproduce the results locally; release notes make no claim that the forensic floor is a live-balance census.

### Stage 5 — Activation and long-tail migration

1. Keep HF4 unscheduled until consensus approval, release review, and operator readiness are documented. Choose an activation height only in that separate approval step.
2. Publish signed, versioned user instructions before activation. Proposed operations cadence: check chain/version/snapshot invariants every block in automated local monitoring; during the first 24 hours, review health every 15 minutes; daily for the first 30 days; weekly thereafter. Publish a bulletin only when there is a material status or instruction change. This cadence is a proposal, not a protocol rule.
3. Keep migration callable after activation without a short cutoff. At each accepted claim, record the unique claim/nullifier, amount, remaining authorized budget, and resulting state root. At exhaustion, reject/refund new claims without changing prior valid migrations.
4. Post-activation rollback is **forward-only**: do not downgrade nodes onto a pre-HF consensus after HF4 blocks are accepted. Before activation, cancel the release and leave HF4 unscheduled. After activation, pause only through the pre-approved mechanism and ship a compatible forward fix; never silently reverse settled claims.
5. Report schedule supply, forensic floor, and migration totals as separate fields/metrics. Do not label an RPC estimate as actual circulating supply.

**Gate 5:** Activation and any later budget change require explicit consensus/community approval. This plan does not set an activation date or authorize a mainnet transaction.

## Definition of done

- No forced same-block migration; a valid claim can be submitted later while the authorized budget remains.
- The hardfork preserves the contract's SCID, code, metadata, and state in canonical snapshots; tested at the boundary and after reorg.
- Every migration is source-backed or consumes an explicit consensus budget; no duplicate claim, overflow, or cap bypass is possible.
- Failed claims are atomic and do not strand or double-credit value.
- Mainnet HF4 remains unscheduled until the separate approval gate; local-only evidence is documented; no mainnet transaction is part of this rollout plan.
