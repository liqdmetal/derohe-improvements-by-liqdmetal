// K0 Fix B2 — uses_signer contract registry tests.
// ⚠️ DRAFT — research tooling, NOT part of DERO release code.
//
// Tests:
//   - ContractUsesSigner: AST-scan detects SIGNER() calls (case-insensitive)
//   - SC_META_DATA NoSigner bit: set/clear + serialization round-trip
//     (the bit lives in the Type byte's high bit, so old 33-byte metadata
//     stays valid and existing contracts default to uses_signer=true)
package dvm

import (
	"strings"
	"testing"
)

func TestContractUsesSigner(t *testing.T) {
	// a contract that calls SIGNER() — must be detected
	usesSigner := `Function OwnerAction() Uint64
	10 IF SIGNER() == LOAD("owner") THEN GOTO 100
	20 RETURN 1
	100 RETURN 0
End Function`
	sc, _, err := ParseSmartContract(usesSigner)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !ContractUsesSigner(sc) {
		t.Fatal("ContractUsesSigner failed to detect SIGNER()")
	}

	// a contract that never calls SIGNER() — must NOT be flagged
	noSigner := `Function Redeem(secret String) Uint64
	10 IF KECCAK256(secret) == LOAD("ph") THEN GOTO 100
	20 RETURN 1
	100 STORE("st",1)
	110 RETURN 0
End Function`
	sc2, _, err := ParseSmartContract(noSigner)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if ContractUsesSigner(sc2) {
		t.Fatal("ContractUsesSigner false-positive on contract with no SIGNER()")
	}
	t.Log("ContractUsesSigner OK: detects SIGNER(), no false positive")
}

func TestSCMetaNoSignerBit(t *testing.T) {
	// default meta (existing contract): Type=0, bit unset -> NoSigner false
	meta := SC_META_DATA{}
	if meta.NoSigner() {
		t.Fatal("default meta should not be NoSigner (preserves existing behavior)")
	}

	// set the bit -> NoSigner true, low bits preserved
	meta.SetNoSigner(true)
	if !meta.NoSigner() {
		t.Fatal("SetNoSigner(true) not reflected")
	}
	if meta.Type&0x7F != 0 {
		t.Fatalf("SetNoSigner clobbered low Type bits: %#x", meta.Type)
	}

	// serialization round-trip (the 33-byte format must stay intact)
	buf := meta.MarshalBinary()
	if len(buf) != 33 {
		t.Fatalf("MarshalBinary len=%d, want 33", len(buf))
	}
	var meta2 SC_META_DATA
	if err := meta2.UnmarshalBinary(buf); err != nil {
		t.Fatalf("UnmarshalBinary: %v", err)
	}
	if !meta2.NoSigner() {
		t.Fatal("NoSigner bit lost in serialization round-trip")
	}

	// private SC (Type=1) + NoSigner coexist
	meta3 := SC_META_DATA{Type: 1}
	meta3.SetNoSigner(true)
	if meta3.Type != 1|SC_META_NOSIGNER_BIT {
		t.Fatalf("private+nosigner Type=%#x want %#x", meta3.Type, byte(1)|SC_META_NOSIGNER_BIT)
	}
	if !meta3.NoSigner() {
		t.Fatal("private+nosigner NoSigner() false")
	}

	// clear the bit
	meta3.SetNoSigner(false)
	if meta3.NoSigner() || meta3.Type != 1 {
		t.Fatalf("SetNoSigner(false) failed: Type=%#x", meta3.Type)
	}
	t.Log(strings.TrimSpace("SC_META_DATA NoSigner bit OK: round-trip + private coexistence"))
}
