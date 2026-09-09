## Summary

**Non-consensus fix** (wallet↔daemon protocol only, ships in a normal release): kills the two decoy-selection privacy leaks from the transaction-relation analysis.

## The two leaks

**K1 — active-account narrowing.** `DERO.GetRandomAddress` samples the balance tree but skips any account whose ciphertext changed in the **last 5 blocks**. Since decoys are guaranteed untouched for 5 blocks, any ring member that *was* touched is, by construction, sender or receiver. An observer computing per-block balance-tree diffs can read this signal off public state.

**K2 — daemon-ring-leak.** The wallet fetches the encrypted balance of every decoy candidate via `DERO.GetEncryptedBalance` with the address **in plaintext**. A daemon sees: the wallet's own address, the receiver, and every decoy re-queried milliseconds after serving it — and can reconstruct the ring, often inferring sender/receiver from query order and timing.

## The fix

**Node side = raw material only.** New RPC `DERO.GetRandomAddressBatch` returns up to 512 **real registered accounts WITH their encrypted balances** in one response, sampled from the balance tree:
- the **5-block filter is removed** (weak floor: current block's touched set only, optional)
- ghost/zero-balance accounts are rejected server-side

**Wallet side = selection.** The wallet verifies the batch (registered + balance present + valid address), then picks the final ring **client-side with its own CSPRNG** (Fisher-Yates draw, `crypto/rand`). The daemon's posterior over the true ring after serving a batch of size B for a ring of size R is **1/C(B,R)** — its information advantage is destroyed.

## Changes

| File | Change |
|---|---|
| `rpc/daemon_rpc.go` | `GetRandomAddressBatch_Params/Result` + `Candidate` structs |
| `cmd/derod/rpc/rpc_dero_getrandomaddress.go` | `GetRandomAddressBatch` handler (cap 512, optional pinned state version) |
| `cmd/derod/rpc/websocket_server.go` | register `getrandomaddressbatch` (both name forms) |
| `walletapi/daemon_communication.go` | `Random_ring_members_batch()` + `filterBatchCandidates()` (fail-closed verification) |
| `walletapi/wallet_transfer.go` | ring assembly uses batch + CSPRNG selection; falls back to legacy path if the daemon lacks the RPC |
| `walletapi/decoy_batch_test.go` | filter test: self/ghost/invalid/unregistered all rejected |

## Security argument

1. **No active-account signal (K1 fixed)** — decoys drawn from the full registered set including recently-active accounts.
2. **Daemon cannot reconstruct ring (K2 fixed)** — one batch request, offline selection.
3. **No ghost injection (K3 fixed)** — wallet verifies registered + balance for every candidate (fail-closed).
4. **No consensus impact** — decoy selection is not consensus-enforced; ships without a fork.

## Honest limits

- Uniform sampling fixes sender-selection statistics, not *activity-distribution matching* (a monthly-active receiver among daily-active decoys is still distinguishable). That's the OSPEAD-style research step, deliberately not in this PR.
- Timing/network metadata (first-seen, IP) is orthogonal.
- Bounded ring (max 128) unchanged.

---

*Branch: `feature/decoy-batch-rpc` in the fork `liqdmetal/derohe-improvements-by-liqdmetal`. Carries the build fixes: re-vendored modules (`vendor/modules.txt` present) and the `KickReader` removal, so the tree builds from a fresh clone.*
