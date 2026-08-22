// Standard-size contract template v0 — MAINNET-v151-COMPATIBLE.
// spec/standard-size-contracts.md — the settlement template that runs on
// DERO mainnet Release 151 AS IS: only the 36 v151 intrinsics, DVM
// version 1.2.3, within the 10M compute-gas / 11k-eval budget.
//
// This is the HTLC-flavored T1 template:
//   - Terms committed at issue via keccak256 (hash commitment — v151 has
//     no group ops, so Pedersen is impossible in-language; a hash
//     commitment over (tier, nonce, blind) is the honest v151 primitive)
//   - Redemption authorized by PREIMAGE of a stored hash (the atomic-swap
//     secret) — the classic HTLC pattern, fully v151-compatible, no
//     signature verification needed in the VM (and no SIGNER()/ringsize-2)
//   - Refund path opens after a block_height deadline (timelock) — v151
//     has block_height; no oracle needed
//   - One-shot state machine: OPEN -> REDEEMED | REFUNDED
//
// Gas accounting (v151 costs): keccak256 = 25k, store/load = 5-10k,
// block_height = 2k. This contract: ~4 intrinsics per call ≈ < 100k gas —
// far inside the 10M budget. Eval count ≈ 20 lines — far inside 11k.
// ⚠️ DRAFT — research template, NOT deployed. Not audited. ⚠️
package dvm

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/deroproject/derohe/cryptography/crypto"
	"github.com/deroproject/derohe/rpc"
)

const stdSizeV151Code = `
/* Standard Size Contract v0 — v151-compatible (T1 HTLC traversal)
   State:
     "issuer_addr"   String  — issuer DERO address (refund payout)
     "counter_addr"  String  — counterparty DERO address (redeem payout)
     "tier"          Uint64  — standard denomination (e.g. 500 = $500)
     "terms_commit"  String  — keccak256(tier:nonce:blind) hash commitment
     "preimage_hash" String  — keccak256(secret) — the atomic-swap secret
     "deadline"      Uint64  — block height after which refund opens
     "state"         Uint64  — 0=OPEN 1=REDEEMED 2=REFUNDED
*/
Function Initialize(issuer_addr String, counter_addr String, tier Uint64, nonce String, blind String, secret String, deadline Uint64, amount Uint64) Uint64
	5   version("1.2.3")          // v151 as-is
	10  STORE("issuer_addr", HEXDECODE(issuer_addr))  // hex -> raw compressed key
	20  STORE("counter_addr", HEXDECODE(counter_addr))
	30  STORE("tier", tier)
	40  STORE("deadline", deadline)
	50  STORE("state", 0)
	60  STORE("amount", amount)   // the locked deposit; paid out on redeem/refund
	70  STORE("terms_commit", KECCAK256(ITOA(tier) + ":" + nonce + ":" + blind))
	80  STORE("preimage_hash", KECCAK256(secret))
	90  RETURN 0
End Function

/* Redeem: counterparty reveals the secret; contract pays the stored
   amount to counter_addr. Pure v151: keccak256 comparison, no signature,
   no group ops. DERO convention: RETURN 0 = success, nonzero = error. */
Function Redeem(secret String) Uint64
	10  IF LOAD("state") != 0 THEN GOTO 900   // only OPEN
	20  IF KECCAK256(secret) == LOAD("preimage_hash") THEN GOTO 100
	30  GOTO 900                              // wrong preimage
	100 STORE("state", 1)                     // REDEEMED
	110 SEND_DERO_TO_ADDRESS(LOAD("counter_addr"), LOAD("amount"))
	120 RETURN 0                             // success — commits state+transfer
	900 RETURN 1                             // error
End Function

/* Refund: after deadline, ANYONE can trigger the refund — funds go to
   the stored issuer address. No SIGNER() (no ringsize-2 requirement,
   K0-safe), no authorization needed for a timelock payout. */
Function Refund() Uint64
	10  IF LOAD("state") != 0 THEN GOTO 900   // only OPEN
	20  IF BLOCK_HEIGHT() < LOAD("deadline") THEN GOTO 900  // not expired yet
	100 STORE("state", 2)                     // REFUNDED
	110 SEND_DERO_TO_ADDRESS(LOAD("issuer_addr"), LOAD("amount"))
	120 RETURN 0                             // success
	900 RETURN 1                             // error
End Function

/* TermsReveal: counterparty reveals (tier, nonce, blind) to prove the
   terms commitment; contract recomputes keccak and compares.
   RETURN 0 = terms match, nonzero = mismatch. */
Function TermsReveal(tier Uint64, nonce String, blind String) Uint64
	10  IF tier != LOAD("tier") THEN GOTO 900
	20  IF KECCAK256(ITOA(tier) + ":" + nonce + ":" + blind) == LOAD("terms_commit") THEN GOTO 100
	30  GOTO 900
	100 RETURN 0                             // match
	900 RETURN 1                             // mismatch
End Function
`

// TestStdSizeV151_EndToEnd: full HTLC flow on v151 intrinsics — install,
// wrong-preimage redeem -> 0, correct preimage -> state=1, double-redeem
// blocked, terms reveal verified.
func TestStdSizeV151_EndToEnd(t *testing.T) {
	s := SimulatorInitialize(nil, 0)
	addr, err := rpc.NewAddress(strings.TrimSpace("deto1qy0ehnqjpr0wxqnknyc66du2fsxyktppkr8m8e6jvplp954klfjz2qqdzcd8p"))
	if err != nil {
		t.Fatal(err)
	}
	var zerohash crypto.Hash
	s.AccountAddBalance(*addr, zerohash, 5000)

	counterAddr, _ := rpc.NewAddress(strings.TrimSpace("deto1qyke2rsfgu9e6skfq6sf7t6rdzdd88srnp2khhjpy9mfvmeu0hsqyqgmpa6c8"))
	s.AccountAddBalance(*counterAddr, zerohash, 0)  // register payout account
	tier := uint64(500)
	nonce := "n-7f3a"
	blind := "b-c0ffee42"
	secret := "the-atomic-swap-secret-9f8e7d6c"

	scid, _, _, err := s.SCInstall(stdSizeV151Code, map[crypto.Hash]uint64{}, rpc.Arguments{
		rpc.Argument{Name: "issuer_addr", DataType: rpc.DataString, Value: hex.EncodeToString(addr.Compressed())},
		rpc.Argument{Name: "counter_addr", DataType: rpc.DataString, Value: hex.EncodeToString(counterAddr.Compressed())},
		rpc.Argument{Name: "tier", DataType: rpc.DataUint64, Value: tier},
		rpc.Argument{Name: "nonce", DataType: rpc.DataString, Value: nonce},
		rpc.Argument{Name: "blind", DataType: rpc.DataString, Value: blind},
		rpc.Argument{Name: "secret", DataType: rpc.DataString, Value: secret},
		rpc.Argument{Name: "deadline", DataType: rpc.DataUint64, Value: uint64(100000)},
		rpc.Argument{Name: "amount", DataType: rpc.DataUint64, Value: uint64(500)},
	}, addr, 0)
	if err != nil {
		t.Fatalf("install: %v", err)
	}

	// deposit tier value via a SUCCESSFUL call (wrong-secret redeem would
	// roll back and discard the deposit — return nonzero = error)
	_, _, err = s.RunSC(map[crypto.Hash]uint64{zerohash: tier}, rpc.Arguments{
		rpc.Argument{Name: rpc.SCACTION, DataType: rpc.DataUint64, Value: uint64(rpc.SC_CALL)},
		rpc.Argument{Name: rpc.SCID, DataType: rpc.DataHash, Value: scid},
		rpc.Argument{Name: "entrypoint", DataType: rpc.DataString, Value: "TermsReveal"},
		rpc.Argument{Name: "tier", DataType: rpc.DataUint64, Value: tier},
		rpc.Argument{Name: "nonce", DataType: rpc.DataString, Value: nonce},
		rpc.Argument{Name: "blind", DataType: rpc.DataString, Value: blind},
	}, addr, 0)
	if err != nil {
		t.Fatalf("deposit (terms reveal) errored: %v", err)
	}
	// wrong preimage -> RETURN 1 -> SC error, state untouched
	_, _, err = s.RunSC(map[crypto.Hash]uint64{}, rpc.Arguments{
		rpc.Argument{Name: rpc.SCACTION, DataType: rpc.DataUint64, Value: uint64(rpc.SC_CALL)},
		rpc.Argument{Name: rpc.SCID, DataType: rpc.DataHash, Value: scid},
		rpc.Argument{Name: "entrypoint", DataType: rpc.DataString, Value: "Redeem"},
		rpc.Argument{Name: "secret", DataType: rpc.DataString, Value: "wrong-secret"},
	}, addr, 0)
	if err == nil {
		t.Fatalf("wrong-preimage redeem should error (nonzero return), got nil")
	}
	// state must still be OPEN (0)
	if st := readSCState(t, s, scid); st != 0 {
		t.Fatalf("wrong preimage changed state to %d (should stay 0)", st)
	}

	// correct preimage -> RETURN 0 -> commits
	_, _, err = s.RunSC(map[crypto.Hash]uint64{}, rpc.Arguments{
		rpc.Argument{Name: rpc.SCACTION, DataType: rpc.DataUint64, Value: uint64(rpc.SC_CALL)},
		rpc.Argument{Name: rpc.SCID, DataType: rpc.DataHash, Value: scid},
		rpc.Argument{Name: "entrypoint", DataType: rpc.DataString, Value: "Redeem"},
		rpc.Argument{Name: "secret", DataType: rpc.DataString, Value: secret},
	}, addr, 0)
	if err != nil {
		t.Fatalf("correct preimage redeem errored: %v", err)
	}
	if st := readSCState(t, s, scid); st != 1 {
		t.Fatalf("expected state=1 (REDEEMED) after correct preimage, got %d", st)
	}

	// double-redeem blocked (state != OPEN -> RETURN 1 -> error)
	_, _, err = s.RunSC(map[crypto.Hash]uint64{}, rpc.Arguments{
		rpc.Argument{Name: rpc.SCACTION, DataType: rpc.DataUint64, Value: uint64(rpc.SC_CALL)},
		rpc.Argument{Name: rpc.SCID, DataType: rpc.DataHash, Value: scid},
		rpc.Argument{Name: "entrypoint", DataType: rpc.DataString, Value: "Redeem"},
		rpc.Argument{Name: "secret", DataType: rpc.DataString, Value: secret},
	}, addr, 0)
	if err == nil {
		t.Fatalf("double-redeem should error (state already REDEEMED), got nil")
	}
	if st := readSCState(t, s, scid); st != 1 {
		t.Fatalf("double-redeem changed state to %d (should stay 1)", st)
	}

	// terms reveal (correct) -> RETURN 0 -> nil err
	_, _, err = s.RunSC(map[crypto.Hash]uint64{}, rpc.Arguments{
		rpc.Argument{Name: rpc.SCACTION, DataType: rpc.DataUint64, Value: uint64(rpc.SC_CALL)},
		rpc.Argument{Name: rpc.SCID, DataType: rpc.DataHash, Value: scid},
		rpc.Argument{Name: "entrypoint", DataType: rpc.DataString, Value: "TermsReveal"},
		rpc.Argument{Name: "tier", DataType: rpc.DataUint64, Value: tier},
		rpc.Argument{Name: "nonce", DataType: rpc.DataString, Value: nonce},
		rpc.Argument{Name: "blind", DataType: rpc.DataString, Value: blind},
	}, addr, 0)
	if err != nil {
		t.Fatalf("terms reveal errored: %v", err)
	}
	t.Logf("v151 HTLC flow OK: install → wrong-preimage-reject → preimage-redeem → state=1 → double-redeem-blocked → terms-reveal")
}

// TestStdSizeV151_RefundBeforeDeadline: refund must FAIL before the
// deadline (timelock holds).
func TestStdSizeV151_RefundBeforeDeadline(t *testing.T) {
	s := SimulatorInitialize(nil, 0)
	addr, _ := rpc.NewAddress(strings.TrimSpace("deto1qy0ehnqjpr0wxqnknyc66du2fsxyktppkr8m8e6jvplp954klfjz2qqdzcd8p"))
	var zerohash crypto.Hash
	s.AccountAddBalance(*addr, zerohash, 5000)
	counterAddr, _ := rpc.NewAddress(strings.TrimSpace("deto1qyke2rsfgu9e6skfq6sf7t6rdzdd88srnp2khhjpy9mfvmeu0hsqyqgmpa6c8"))

	scid, _, _, err := s.SCInstall(stdSizeV151Code, map[crypto.Hash]uint64{}, rpc.Arguments{
		rpc.Argument{Name: "issuer_addr", DataType: rpc.DataString, Value: hex.EncodeToString(addr.Compressed())},
		rpc.Argument{Name: "counter_addr", DataType: rpc.DataString, Value: hex.EncodeToString(counterAddr.Compressed())},
		rpc.Argument{Name: "tier", DataType: rpc.DataUint64, Value: uint64(250)},
		rpc.Argument{Name: "nonce", DataType: rpc.DataString, Value: "n-1"},
		rpc.Argument{Name: "blind", DataType: rpc.DataString, Value: "b-2"},
		rpc.Argument{Name: "secret", DataType: rpc.DataString, Value: "s-3"},
		rpc.Argument{Name: "deadline", DataType: rpc.DataUint64, Value: uint64(100000)},
		rpc.Argument{Name: "amount", DataType: rpc.DataUint64, Value: uint64(500)},
	}, addr, 0)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	s.RunSC(map[crypto.Hash]uint64{zerohash: 250}, rpc.Arguments{
		rpc.Argument{Name: rpc.SCACTION, DataType: rpc.DataUint64, Value: uint64(rpc.SC_CALL)},
		rpc.Argument{Name: rpc.SCID, DataType: rpc.DataHash, Value: scid},
		rpc.Argument{Name: "entrypoint", DataType: rpc.DataString, Value: "Redeem"},
		rpc.Argument{Name: "secret", DataType: rpc.DataString, Value: "x"},
	}, addr, 0)

	// refund before deadline (simulator height is 0 < 100000) must NOT redeem
	// (returns nonzero = error; state stays OPEN)
	_, _, err = s.RunSC(map[crypto.Hash]uint64{}, rpc.Arguments{
		rpc.Argument{Name: rpc.SCACTION, DataType: rpc.DataUint64, Value: uint64(rpc.SC_CALL)},
		rpc.Argument{Name: rpc.SCID, DataType: rpc.DataHash, Value: scid},
		rpc.Argument{Name: "entrypoint", DataType: rpc.DataString, Value: "Refund"},
	}, addr, 0)
	if err == nil {
		t.Fatalf("refund before deadline should error (timelock holds), got nil")
	}
	if st := readSCState(t, s, scid); st != 0 {
		t.Fatalf("refund before deadline changed state to %d (should stay OPEN=0)", st)
	}
	t.Logf("v151 timelock OK: refund before deadline rejected (state stays OPEN)")
}

func readSCState(t *testing.T, s *Simulator, scid crypto.Hash) uint64 {
	t.Helper()
	dt := Wrapped_tree(s.cache, s.ss, scid)
	v := ReadSCValue(dt, scid, "state")
	st, ok := v.(uint64)
	if !ok {
		t.Fatalf("state read failed: %v", v)
	}
	return st
}

// keep imports used
var _ = sha256.New
var _ = hex.EncodeToString
var _ = strings.TrimSpace
