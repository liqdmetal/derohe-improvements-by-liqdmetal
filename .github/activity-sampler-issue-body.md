## Summary

**D5 from the decoy-activity-distribution spec:** decoy sampling that matches the **real participant activity distribution**, so an observer cannot distinguish real sender/receiver from decoys by on-chain activity alone — the OSPEAD analog for DERO's account model. Pure client logic, no consensus impact.

## The leak it fixes

The current decoy pool is "accounts not seen in any ring for the last 5 blocks" — guaranteed-dormant. Real participants (settlement wallets, AMMs, recurring users) appear in rings far more often. An observer computing per-account ring-appearance history scores each ring member by "how unlike a decoy is this account's activity?" — the posterior skews away from uniform, and effective anonymity is smaller than ring size.

Measured (D2, 3.6k-block scan): participants skew heavily recent (13.6% in 0–5 blocks, 24.7% in 5–10) while the decoy pool is guaranteed-dormant — **p/d = ∞ in bin 0–5**.

**The posterior is uniform iff decoys are drawn from the same distribution as real participants** (spec §3.1). This module implements that match.

## The implementation

`walletapi/decoy_sampler.go`:
- **`DecoyModel`** — a compact published bin table (`blocks-since-last-appearance` × weight), sourced from the D2 direct estimator (participant density p(x) recovered from **ringsize-2 members** — the only rings where both members are real, so no deconvolution noise; the naive mixture estimator was rejected by D4 validation)
- **`candidateRecency`** — extracts blocks-since-last from the candidate's embedded NonceBalance (uvarint NonceHeight + ElGamal)
- **`SelectDecoys`** — weighted draws without replacement, probability ∝ bin weight; zero-weight candidates never drawn

Plus a **decode fix** in the batch path: the tree value is NonceBalance (varint NonceHeight + 66B ElGamal), not a bare ElGamal; and `NonceBalance.Unmarshal` panics on malformed input, so it must be recover-guarded — a wallet must never panic on a daemon's malformed batch.

## Tests (`walletapi/decoy_sampler_test.go`)

- `TestCandidateRecency` — extraction + malformed → 0
- `TestSelectDecoysMatchesModel` — sampled distribution matches model weights (recent share ~0.67 vs expected 0.714 on a 10:1 model, within finite-population tolerance); zero-weight candidates excluded

## Model publication (D6, follow-up)

The model file is a published dataset (quantile/bin table, kilobytes) re-published per epoch from the D2 estimator. Model drift (spec §6.3) is handled by the publication cadence; a stale model re-creates the leak, so this is an operational commitment, not a code change.

## Honest limits

- Matches **marginal activity** (recency). Higher-order features (pairwise co-appearance, periodicity) are the spec's open research questions — v2 work.
- Amounts are invisible on-chain (ElGamal), so amount-correlation attacks are structurally impossible — a DERO advantage.
- Uniform-over-batch (PR #89) strictly improves the current state and ships independently; this PR is the next step.

## Relationship

- **Depends on PR #89** (decoy batch RPC) — this branch stacks on it (the batch carries each candidate's NonceHeight embedded in the balance, which is the activity feature)
- Spec: `decoy-activity-distribution.md` (D5), companion to `decoy-selection-batch-rpc.md`

---

*Branch: `feature/activity-matched-sampler` in the fork `liqdmetal/derohe-improvements-by-liqdmetal` (stacked on `feature/decoy-batch-rpc`). Carries the build fixes: re-vendored modules (`vendor/modules.txt` present) and the `KickReader` removal, so the tree builds from a fresh clone.*
