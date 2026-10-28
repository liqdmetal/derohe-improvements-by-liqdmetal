# DERO.GetInfo forensic supply record

This implements Decision 2 in `DERO-STATEMENT_2608.md`. It reports a forensic floor beside the emission-schedule `total_supply`; it does **not** modify balances, issuance, or consensus state and is not a circulating-supply estimate.

## Gate

`created_supply_floor` is returned only when the daemon is configured for `mainnet`, is not running as a simulator, and the current height is at or beyond `MAJOR_HF3_HEIGHT`. Testnet, simulator, and pre-HF3 responses omit the field. The record is read-only and compiled into the daemon; changing it requires a code commit with the supporting evidence and a new record version.

The `GetInfo` `spendable_supply` field forecasts the supply for the block after the current tip: the schedule is evaluated at `tip_height + 1`, and the net-created floor is included when that next block reaches the mainnet HF3 activation height. This intentionally exposes the floor one block before HF3 in the tip-height view. `total_supply` remains the emission schedule at the current tip. Testnet and simulator forecasts never include the forensic floor.

## Current record

All integer amounts in the record are atomic DERO units (100,000 per DERO). The net floor is computed and validated as gross created floor minus the chain-verified burn. The existing `GetInfo` `total_supply` and `spendable_supply` fields are whole-DERO counts; fractional atomic units are truncated there, while this record preserves exact amounts.

| Field | Atomic units | DERO | Meaning |
|---|---:|---:|---|
| `gross_created_floor_atomic_units` | 2,388,651,730,000 | 23,886,517.3 | Forensically established created coin; undecoded affected transactions excluded |
| `verified_burn_atomic_units` | 500,000,000,000 | 5,000,000 | Chain-verified burn from the same era; attribution to the exploit wallets is unproven |
| `net_created_floor_atomic_units` | 1,888,651,730,000 | 18,886,517.3 | Gross floor less the verified burn |

The object also carries a record version, metric label, unit scale, scope note, evidence references, and `burn_attribution_proven: false`. The net arithmetic is **not** evidence that the burn came from exploit wallets or a count of affected coins still outstanding. Neither `total_supply` nor `spendable_supply` is a balance census; the latter is a height-based estimate using this forensic floor only after the mainnet HF3 gate.

## Scope boundary

This is a telemetry/assertion record, not gated reissuance, compensation, clawback, or a native-balance migration. A separate consensus invariant caps the height-based emission schedule (`CalcSupply`) at 21M DERO and checks coinbase addition overflow; it does not cap the current forensic/spendable estimate or change balances. A smart contract cannot change the base-layer DERO supply; any future native balance change would require a separately authorized consensus migration with its own evidence, proof rules, and adversarial tests.
