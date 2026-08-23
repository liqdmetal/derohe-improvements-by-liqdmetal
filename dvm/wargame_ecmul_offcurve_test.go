// Wargame: ec_mul / ec_add point-input validation gap.
//
// The v9/I3 point intrinsics (ec_mul, ec_add, verify_commit) decode their
// compressed-point inputs with bn256.DecodeCompressed and only check
// err == nil. But the conformance PR documented that Go's decoder
// (bn256/changes.go) DISCARDS the on-curve error when x >= p — an
// off-curve point decodes with err == nil (pinned by
// TestConformance_MalformedPointDecode). So a contract can feed an
// off-curve point to ec_mul/ec_add and get back a computed result instead
// of a rejection.
//
// Consensus relevance: ScalarMult on an invalid point is deterministic
// GARBAGE — all Go nodes agree (no Go-vs-Go split), but a strict
// implementation (the clean-room Rust port, which rejects at decode)
// produces a different result -> state-root divergence -> chain-split
// class bug, exactly what the conformance PR warns about. The intrinsics
// that accept compressed points must validate on-curve AFTER decode.
package dvm

import (
	"encoding/hex"
	"go/ast"
	"go/token"
	"strconv"
	"testing"

	"github.com/blang/semver/v4"
	"github.com/deroproject/derohe/cryptography/bn256"
)

// x > p (p = bn256 base field modulus), from the conformance vectors.
const xGtP = "30644e72e131a029b85045b68181585d97816a916871ca8d3c208c16d87cfd48" + "00"

func TestWargameEcMulAcceptsOffCurvePoint(t *testing.T) {
	// sanity: the decoder really does accept it (mirrors conformance vector)
	var pt bn256.G1
	if err := pt.DecodeCompressed(mustHex(t, xGtP)); err != nil {
		t.Fatalf("precondition failed: DecodeCompressed(x_gt_p) should return nil error (documented Go behavior), got %v", err)
	}

	dvm := &DVM_Interpreter{Version: semver.MustParse("9.0.0"), State: &Shared_State{}}
	mkHex := func(s string) ast.Expr { return &ast.BasicLit{Kind: token.STRING, Value: "\"" + s + "\""} }
	mkUint := func(v uint64) ast.Expr { return &ast.BasicLit{Kind: token.INT, Value: strconv.FormatUint(v, 10)} }

	expr := &ast.CallExpr{Fun: &ast.Ident{Name: "ec_mul"},
		Args: []ast.Expr{mkHex(xGtP), mkUint(2)}}

	// FIXED: the intrinsic now uses strictDecodeG1 (x < p validation), so
	// the off-curve encoding is REJECTED with a panic (recovered by the
	// Execute_sc_function wrapper -> deterministic tx failure), matching the
	// strict decoder. No more deterministic-garbage computation.
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("ec_mul accepted an x>=p point after strict-decode fix — gap still open")
			}
			t.Logf("WARGAME FIXED: ec_mul now rejects the off-curve (x>p) encoding via strictDecodeG1")
		}()
		dvm_ec_mul(dvm, expr)
	}()

	// Sanity: a canonical point still works (strict decode does not break
	// valid inputs).
	canonical := "000000000000000000000000000000000000000000000000000000000000000100" // x=1, y-even flag
	if _, res := dvm_ec_mul(dvm, &ast.CallExpr{Fun: &ast.Ident{Name: "ec_mul"},
		Args: []ast.Expr{mkHex(canonical), mkUint(2)}}); res == "" {
		t.Fatal("strict decode broke canonical point input")
	}
	t.Logf("canonical point still accepted — strict validation only rejects non-canonical encodings")
}

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("bad hex: %v", err)
	}
	return b
}
