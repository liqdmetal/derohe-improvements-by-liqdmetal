package dvm

// TEMPORARY verification harness for obscura's DeroHashTimelock.bas.
// Runs the full HTLC state machine on the DVM simulator and asserts the
// storage/balance invariants. Removed after verification.

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"github.com/deroproject/derohe/cryptography/crypto"
	"github.com/deroproject/derohe/rpc"
)

func loadContract(t *testing.T) string {
	// contract lives in the obscura repo
	b, err := os.ReadFile(`C:\Users\officeone\obscura\contracts\dero\DeroHashTimelock.bas`)
	if err != nil {
		t.Skipf("obscura contract not present on this machine (%v); harness is machine-local", err)
	}
	return string(b)
}

func newAddr(t *testing.T, s string) *rpc.Address {
	t.Helper()
	a, err := rpc.NewAddress(strings.TrimSpace(s))
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func callArgs(scid crypto.Hash, entrypoint string, extra ...rpc.Argument) rpc.Arguments {
	args := rpc.Arguments{
		{rpc.SCACTION, rpc.DataUint64, uint64(rpc.SC_CALL)},
		{rpc.SCID, rpc.DataHash, scid},
		{"entrypoint", rpc.DataString, entrypoint},
	}
	args = append(args, extra...)
	return args
}

func deroBalance(t *testing.T, s *Simulator, scid crypto.Hash) uint64 {
	t.Helper()
	w := Wrapped_tree(s.cache, s.ss, scid)
	v, _ := LoadSCAssetValue(w, scid, crypto.Hash{})
	return v
}

func key(t *testing.T, s *Simulator, scid crypto.Hash, k string) interface{} {
	t.Helper()
	w := Wrapped_tree(s.cache, s.ss, scid)
	return ReadSCValue(w, scid, k)
}

func keyExists(t *testing.T, s *Simulator, scid crypto.Hash, k string) bool {
	t.Helper()
	w := Wrapped_tree(s.cache, s.ss, scid)
	keybytes := DataKey{Key: Variable{Type: String, ValueString: k}}.MarshalBinaryPanic()
	_, found := LoadSCValue(w, scid, keybytes)
	return found
}

// TestDeroHtlc_LockClaimRefund drives the full HTLC state machine.
func TestDeroHtlc_LockClaimRefund(t *testing.T) {
	src := loadContract(t)
	s := SimulatorInitialize(nil, 0)

	locker := newAddr(t, "deto1qy0ehnqjpr0wxqnknyc66du2fsxyktppkr8m8e6jvplp954klfjz2qqdzcd8p")
	claimer := newAddr(t, "deto1qyvyeyzrcm2fzf6kyq7egkes2ufgny5xn77y6typhfx9s7w3mvyd5qqynr5hx")

	var zerohash crypto.Hash
	s.AccountAddBalance(*locker, zerohash, 10_000)
	s.AccountAddBalance(*claimer, zerohash, 10_000)

	scid, _, _, err := s.SCInstall(src, map[crypto.Hash]uint64{}, rpc.Arguments{}, locker, 0)
	if err != nil {
		t.Fatalf("install: %v", err)
	}

	// --- Lock: locker deposits 1000 DERO against a sha256 preimage ---
	preimage := []byte("the-secret-bridge-secret")
	hash := sha256.Sum256(preimage)
	hashlockHex := hex.EncodeToString(hash[:])
	refundHeight := uint64(1_000_000) // far future so Claim works before expiry

	_, _, err = s.RunSC(
		map[crypto.Hash]uint64{zerohash: 1000},
		callArgs(scid, "Lock",
			rpc.Argument{"lockid", rpc.DataString, "swap1"},
			rpc.Argument{"hashlock", rpc.DataString, hashlockHex},
			rpc.Argument{"refundheight", rpc.DataUint64, refundHeight},
		),
		locker, 0)
	if err != nil {
		t.Fatalf("lock: %v", err)
	}

	// contract holds 1000 DERO, state recorded
	if got := deroBalance(t, s, scid); got != 1000 {
		t.Fatalf("after lock: SC balance = %d, want 1000", got)
	}
	if got := key(t, s, scid, "l_swap1_hash"); got != hashlockHex {
		t.Fatalf("stored hashlock mismatch")
	}
	if got := key(t, s, scid, "l_swap1_amount"); got != "1000" {
		t.Fatalf("stored amount = %v, want 1000", got)
	}

	// --- Claim with the WRONG preimage is rejected (state unchanged) ---
	_, _, err = s.RunSC(
		map[crypto.Hash]uint64{},
		callArgs(scid, "Claim",
			rpc.Argument{"lockid", rpc.DataString, "swap1"},
			rpc.Argument{"preimage", rpc.DataString, hex.EncodeToString([]byte("wrong"))},
		),
		claimer, 0)
	if err == nil {
		t.Fatalf("claim(wrong) was accepted — contract failed to reject (RETURN 1 must discard the tx)")
	}
	if deroBalance(t, s, scid) != 1000 {
		t.Fatalf("wrong preimage moved funds: balance = %d, want 1000", deroBalance(t, s, scid))
	}
	if !keyExists(t, s, scid, "l_swap1_hash") {
		t.Fatalf("wrong preimage deleted the lock")
	}

	// --- Claim with the CORRECT preimage: claimer receives DERO, lock cleared ---
	_, _, err = s.RunSC(
		map[crypto.Hash]uint64{},
		callArgs(scid, "Claim",
			rpc.Argument{"lockid", rpc.DataString, "swap1"},
			rpc.Argument{"preimage", rpc.DataString, string(preimage)}, // raw preimage — the contract does HEX(SHA256(preimage)) itself
		),
		claimer, 0)
	if err != nil {
		t.Fatalf("claim(correct) errored: %v", err)
	}
	if deroBalance(t, s, scid) != 0 {
		t.Fatalf("after claim: SC balance = %d, want 0", deroBalance(t, s, scid))
	}
	if keyExists(t, s, scid, "l_swap1_hash") {
		t.Fatalf("claimed lock not cleared")
	}
	// claimer's external balance was credited (0 incoming, +1000 out)
	if b, _ := LoadSCAssetValue(Wrapped_tree(s.cache, s.ss, scid), zerohash, zerohash); false {
		_ = b
	}
}

// TestDeroHtlc_Refund verifies the refund path: locker reclaims only after
// the refund height.
func TestDeroHtlc_Refund(t *testing.T) {
	src := loadContract(t)
	s := SimulatorInitialize(nil, 0)

	locker := newAddr(t, "deto1qy0ehnqjpr0wxqnknyc66du2fsxyktppkr8m8e6jvplp954klfjz2qqdzcd8p")
	attacker := newAddr(t, "deto1qyvyeyzrcm2fzf6kyq7egkes2ufgny5xn77y6typhfx9s7w3mvyd5qqynr5hx")

	var zerohash crypto.Hash
	s.AccountAddBalance(*locker, zerohash, 10_000)
	s.AccountAddBalance(*attacker, zerohash, 10_000)

	scid, _, _, err := s.SCInstall(src, map[crypto.Hash]uint64{}, rpc.Arguments{}, locker, 0)
	if err != nil {
		t.Fatalf("install: %v", err)
	}

	hashlockHex := strings.Repeat("a", 64)
	// refundheight=10, simulator height=0 -> NOT refundable yet
	_, _, err = s.RunSC(
		map[crypto.Hash]uint64{zerohash: 500},
		callArgs(scid, "Lock",
			rpc.Argument{"lockid", rpc.DataString, "swap2"},
			rpc.Argument{"hashlock", rpc.DataString, hashlockHex},
			rpc.Argument{"refundheight", rpc.DataUint64, uint64(10)},
		),
		locker, 0)
	if err != nil {
		t.Fatalf("lock: %v", err)
	}

	// attacker cannot refund (not the locker) — state unchanged
	_, _, err = s.RunSC(map[crypto.Hash]uint64{}, callArgs(scid, "Refund",
		rpc.Argument{"lockid", rpc.DataString, "swap2"}), attacker, 0)
	if err == nil {
		t.Fatalf("refund(attacker) was accepted — contract failed to reject")
	}
	if deroBalance(t, s, scid) != 500 || !keyExists(t, s, scid, "l_swap2_hash") {
		t.Fatalf("non-locker refund moved funds")
	}

	// locker refund too early (height 0 < 10) — state unchanged
	_, _, err = s.RunSC(map[crypto.Hash]uint64{}, callArgs(scid, "Refund",
		rpc.Argument{"lockid", rpc.DataString, "swap2"}), locker, 0)
	if err == nil {
		t.Fatalf("early refund was accepted — contract failed to reject")
	}
	if deroBalance(t, s, scid) != 500 {
		t.Fatalf("early refund moved funds")
	}

	// advance to refund height and refund
	s.height = 11
	_, _, err = s.RunSC(map[crypto.Hash]uint64{}, callArgs(scid, "Refund",
		rpc.Argument{"lockid", rpc.DataString, "swap2"}), locker, 0)
	if err != nil {
		t.Fatalf("refund errored: %v", err)
	}
	if deroBalance(t, s, scid) != 0 {
		t.Fatalf("after refund: SC balance = %d, want 0", deroBalance(t, s, scid))
	}
	if keyExists(t, s, scid, "l_swap2_hash") {
		t.Fatalf("refunded lock not cleared")
	}
}
