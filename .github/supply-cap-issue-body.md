## Summary

**Adds a consensus supply hard-cap invariant** asserting DERO's 21M-DERO ceiling at coinbase verification.

## The finding

`Verify_Transaction_Coinbase` was an `IsCoinbase()`-only no-op. The 21M
hard cap (the Bitcoin-parallel supply ceiling) was **not enforced anywhere in
consensus** — the `deroHardCapAtomic` constant in `proof/proof_validation.go`
only sanity-checks individual proof amounts, never chain supply. The coinbase
mint (`full_reward := base_reward + fees`) had no overflow guard.

DERO's emission curve is a pure function of height (`CalcSupply`) that
converges to **~20.89M DERO**, so the real curve never approaches the cap —
but there is no invariant that would catch a *regression* in
`CalcBlockReward` / `BaseReward` / `PREMINE` (a bad `>>` shift, an accidental
reward inflation, uint64 wraparound) that would silently mint past the cap.

## The change

1. **`config.MAX_SUPPLY = 21_000_000 * 100_000`** — the hard cap (2.1e12 atomic).
2. **`Verify_Transaction_Coinbase` asserts `CalcSupply(height) <= MAX_SUPPLY`**
   — every node recomputes the supply identically, so this is deterministic
   and only trips on a curve regression. Purely a defensive invariant on the
   real curve.
3. **Coinbase emission guards `full_reward`** against uint64 wraparound and
   the cap (panic on overflow — can only fire on a curve regression).

## Tests

- `TestCalcSupplyUnderHardCap` — supply under the cap at every sampled height
  (genesis → 160 years), and monotonic.
- `TestCalcBlockRewardConvergence` — reward non-increasing per epoch,
  converges to zero without underflow.

Terminal supply **20.89M** vs cap **21.00M** = **0.51% margin** — the assert
is meaningful (it would catch an inflation bug) but never false-positives on
the real curve.

## Consensus status

Hard-fork (consensus) change, same category as K0 Fix B1/B2. On the real
curve it changes nothing — it only adds rejection for a state that can only
arise from an emission bug.

---

*Branch `fix/supply-hardcap`, based on `community-dev`.*
