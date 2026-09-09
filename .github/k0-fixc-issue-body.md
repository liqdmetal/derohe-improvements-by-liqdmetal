## Summary

**The last piece of the K0 sequence:** once `verify_sig` (DVM v9, PR #84) lets a contract verify Ed25519 signatures in encrypted SCDATA, owner-gated entrypoints no longer need ringsize 2 — the signer is authorized by *signature*, not by ringsize-2 ring exposure. This PR is the wallet-side half plus the end-to-end demo.

## Why it completes K0

The ringsize-2 ring exposes the signer by design. The K0 package removes every legitimate use:

| PR | Fix | Effect |
|---|---|---|
| #80 | A | wallet warns on ringsize 2 (non-consensus) |
| #82 | B1 | consensus min-ring-4 for NORMAL/BURN |
| #91 | B2 | ringsize-2 SC_TX only for contracts that declare SIGNER() |
| #84 | C (intrinsic) | `verify_sig` — signature verification in the VM |
| **this** | **C (wallet+demo)** | **owner auth by signature at ringsize ≥ 4** |

After this, **ringsize 2 has no legitimate remaining use** — the last reason it existed (owner-gated contracts needing `SIGNER()` at ringsize 2) is gone.

## The implementation

**`walletapi/sc_auth.go`** — the wallet-side helper (implemented + unit-tested):
- `SCAuthKey`: app-level Ed25519 keypair. DERO spend keys are bn256 scalars, so SC-auth is a separate key, persisted by the wallet.
- `SignSCData`: builds + signs the call message `domain:scid:entrypoint:args`, bound to the call context so a signature can't be replayed onto another entrypoint or contract.
- `VerifySCData`: client-side mirror of the contract's `verify_sig` check (self-check before broadcast).

**`dvm/fixc_auth_test.go`** — the full Fix C flow in the simulator (passing):
- install an owner-gated contract with the owner's Ed25519 pubkey
- wallet signs the call message
- the contract's `verify_sig` authorizes at ringsize ≥ 4
- attacker key rejected · tampered signature rejected · state transition verified

Two real bugs were found and fixed while getting the test green:
1. **State-read path** — the simulator has no `State` field; the test now reads the per-SCID graviton tree exactly as the interpreter's diskloader does (`Wrapped_tree` + `DataKey{SCID, Key}.MarshalBinary()`).
2. **Message-encoding convention** — DVM's `SCID()` intrinsic returns the **raw 32 bytes** as a String (not hex), so the wallet's `SignSCData` must build the signed message with the same encoding or `verify_sig` fails. The test now matches the DVM's actual semantics.

**`walletapi/sc_auth_test.go`** — direct unit tests for the helper (round-trip, tamper/wrong-key/malformed rejection, message convention + domain binding) that the dvm simulator test cannot reach due to the dvm↔walletapi import cycle.

## How a contract uses it

```
Function OwnerAction(nonce String, pubkey String, sig String) Uint64
	5 version("9.0.0")
	10 LET msg = "relayos:" + SCID() + ":OwnerAction:" + nonce
	20 IF verify_sig(pubkey, msg, sig) != 1 THEN GOTO 900
	30 IF pubkey != LOAD("owner") THEN GOTO 900
	40 STORE("authorized", 1)
	50 RETURN 0
	900 RETURN 1
End Function
```

The caller signs `domain:scid:entrypoint:nonce` with the owner's Ed25519 key at **ringsize ≥ 4** — no SIGNER(), no ringsize-2 exposure, and the caller stays anonymous (only the *signature* proves authorization).

## Relationship

- **Depends on PR #84** (DVM v9 `verify_sig`) — this branch stacks on it
- Completes the K0 package: A (#80) + B1 (#82) + B2 (#91) + C (this PR)
- The ringsize-2 era ends here; combined with the decoy batch RPC (#89) and activity-matched sampling (#96), the privacy story is coherent

---

*Branch: `feature/k0-fix-c` in the fork `liqdmetal/derohe-improvements-by-liqdmetal` (stacked on `feature/dvm-v9-intrinsics`). Carries the build fixes: re-vendored modules (`vendor/modules.txt` present) and the `KickReader` removal, so the tree builds from a fresh clone.*
