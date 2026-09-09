## Summary

**L4 from the DVM-BASIC language agenda (P1):** `mapkeys() -> String` — map enumeration for SC state. Batch/paged contracts can iterate their stored keys instead of hand-rolling key sets.

## The intrinsic

```
LET ks = mapkeys()     ' "alpha,beta,gamma" — sorted, comma-separated
```

The effective key set is the union of:
- **`RamStore`** — keys loaded from disk (via DiskLoader) during this call
- **`RawKeys`** — keys written during this call (`TX_Storage.RawKeys`)

Keys are the String/Uint64 values of the stored Variables, unmarshaled, deduped, sorted, and joined. The comma-separated String is the DVM-friendly return (intrinsics return String/Uint64); contracts split it for iteration, and L3 arrays give the next natural return type.

## Why it matters

Today a contract that batches/pages state must hand-roll its key set (store a counter, maintain a registry). With `mapkeys`, the keys themselves are enumerable — combined with L1's `FOR` and L3's arrays, a contract can:

```
FOR i = 0 TO <n>
    LET k = <split(ks, i)>
    LET v = LOAD(k)
    ... process ...
NEXT i
```

## Consensus safety

- Gated `>= 10.0.0` like L1/L2/L3 — pre-fork contracts can't use it
- Read-only: enumerates keys, changes no state

## Tests (`dvm/control_flow_l4_test.go`, 3 cases)

| Test | Verifies |
|---|---|
| `TestL4_MapkeysBasic` | STORE alpha/beta/gamma → `"alpha,beta,gamma"` (sorted) |
| `TestL4_MapkeysIterate` | batch iteration pattern |
| `TestL4_MapkeysVersionGate` | rejected at `1.2.3` |

Full dvm suite green.

## Relationship

- **Stacks on L3** (PR #107) — same `>= 10.0.0` gate; L3 arrays give the next natural return type
- Completes the L-series P1 items (L3 arrays + L4 enumeration) on top of the P0 items (L1 control flow + L2 subroutines)

---

*Branch: `feature/dvm-l4-mapkeys` in the fork `liqdmetal/derohe-improvements-by-liqdmetal` (stacked on `feature/dvm-l3-arrays`).*
