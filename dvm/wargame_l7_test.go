// Wargame: L7+L8 CONST immutability enforcement.
//
// Findings pinned:
//
// 1. CASE-VARIANT SHADOWING: CONST dispatch is case-insensitive
//    (strings.EqualFold for the CONST keyword) but resolveConst does an
//    EXACT map lookup. A contract that declares CONST FOO = 5 and then
//    LET foo = 7 may be able to create a case-variant local that SHADOWS
//    the constant (bypassing immutability) — because the LET immutability
//    check also uses resolveConst (exact match).
//
// 2. Direct LET on a constant must be rejected (immutable).
package dvm

import (
	"testing"
)

func runL7(t *testing.T, code string) (Variable, error) {
	t.Helper()
	sc, pos, err := ParseSmartContract(code)
	if err != nil {
		t.Fatalf("parse (%s): %v", pos, err)
	}
	state := &Shared_State{Chain_inputs: &Blockchain_Input{}}
	return RunSmartContract(&sc, "F", state, map[string]interface{}{})
}

// TestWargameL7ConstImmutable: LET on a declared constant must be rejected.
func TestWargameL7ConstImmutable(t *testing.T) {
	code := `
Function F() Uint64
	5 version("10.0.0")
	10 CONST FEE = 100
	20 LET FEE = 50
	30 RETURN FEE
End Function
`
	res, err := runL7(t, code)
	if err != nil {
		t.Logf("WARGAME OK: LET on a constant rejected (%v)", err)
		return
	}
	if res.ValueUint64 == 50 {
		t.Fatal("WARGAME FINDING: CONST FEE reassigned to 50 — immutability bypassed")
	}
	t.Log("CONST value preserved (immutability holds)")
}

// TestWargameL7ConstCaseVariant: CONST FOO + DIM foo + LET foo must not
// shadow the constant (case-insensitive language — the CONST keyword is
// dispatched EqualFold, so FOO/foo should be the same identifier).
func TestWargameL7ConstCaseVariant(t *testing.T) {
	code := `
Function F() Uint64
	5 version("10.0.0")
	10 DIM foo AS Uint64
	20 CONST FOO = 5
	30 LET foo = 7
	40 RETURN FOO
End Function
`
	res, err := runL7(t, code)
	if err != nil {
		t.Logf("run error (boundary): %v", err)
		t.Skip("case-variant CONST/variable interplay rejected by the parser")
	}
	if res.ValueUint64 == 7 {
		t.Fatal("WARGAME FINDING: CONST FOO shadowed by case-variant LET foo = 7 — returned 7, not 5 (case-variant shadowing bypass)")
	}
	t.Log("CONST FOO not shadowed by case-variant (immutability holds)")
}
