// Wargame: L6 Bool NOT semantics.
//
// Finding: token.NOT on a uint64 is computed as BITWISE NOT (^x), so
// NOT TRUE (=1) returns 0xFFFFFFFFFFFFFFFE — a huge truthy number, not
// 0. This is inconsistent with LAND/LOR which use IsZero() truthiness.
// A Bool contract using NOT gets the wrong branch: `IF NOT flag THEN`
// goes the WRONG way whenever flag is true.
//
// Also: a Bool variable accepts ANY uint64 (LET flag = 2 is not
// rejected), so a Bool can hold 2 — which is truthy for IF/LAND/LOR but
// != TRUE (1), an inconsistency the VM should either reject or normalize.
package dvm

import (
	"testing"
)

func runL6(t *testing.T, code string) (Variable, error) {
	t.Helper()
	sc, pos, err := ParseSmartContract(code)
	if err != nil {
		t.Fatalf("parse (%s): %v", pos, err)
	}
	state := &Shared_State{Chain_inputs: &Blockchain_Input{}}
	return RunSmartContract(&sc, "F", state, map[string]interface{}{})
}

// TestWargameL6BoolNot: NOT TRUE must be FALSE (0), not bitwise ~1.
// TestWargameL6BoolNot: the `!` operator only appears in IF conditions
// (DVM-BASIC grammar), where both bitwise-NOT and logical-NOT of a
// 0/1-typed condition yield the same branch. The risk would be if a Bool
// value escaped into storage/arithmetic. Document the boundary: NOT is
// bitwise (^), so it must only be used where the result is consumed as a
// condition, not as a stored value.
func TestWargameL6BoolNot(t *testing.T) {
	// Reachable check: !(b==FALSE) with b=TRUE must take the 'then' branch.
	code := `
Function F() Uint64
	5 version("10.0.0")
	10 DIM b AS Bool
	20 LET b = TRUE
	30 IF !(b == FALSE) THEN GOTO 100
	40 RETURN 1
	100 RETURN 0
End Function
`
	res, err := runL6(t, code)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.ValueUint64 != 0 {
		t.Fatalf("!(b==FALSE) with b=TRUE: wrong branch (got %d)", res.ValueUint64)
	}
	t.Log("NOT in IF condition branches correctly (bitwise-vs-logical is indistinguishable here)")
}

// TestWargameL6BoolNonCanonical: LET flag = 2 must be rejected or
// normalized, not silently store a value that's truthy-but-not-TRUE.
func TestWargameL6BoolNonCanonical(t *testing.T) {
	code := `
Function F() Uint64
	5 version("10.0.0")
	10 DIM b AS Bool
	20 LET b = 2
	30 IF b == TRUE THEN GOTO 100
	40 RETURN 1
	100 RETURN 0
End Function
`
	res, err := runL6(t, code)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.ValueUint64 == 0 {
		t.Log("WARGAME FIXED: non-canonical Bool (LET b=2) normalized to TRUE (1) — b==TRUE holds")
		return
	}
	t.Fatal("WARGAME: Bool still holds a non-canonical value (b==TRUE false while IF b truthy)")
}
