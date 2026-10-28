package proof

import (
	"strings"
	"testing"
)

func TestValidatePayloadProofAmount(t *testing.T) {
	tests := []struct {
		name        string
		amount      uint64
		shouldError bool
	}{
		{name: "normal amount (100 DERO)", amount: 10_000_000},
		{name: "large amount (1M DERO)", amount: 100_000_000_000},
		{name: "schedule-scale amount (16.5M DERO)", amount: 1_650_000_000_000},
		{name: "21M DERO is not a proof-validation cap", amount: 2_100_000_000_000},
		{name: "23M DERO forensic-scale amount is not rejected by a false cap", amount: 2_300_000_000_000},
		{name: "50M DERO is not rejected by an unproven supply cap", amount: 5_000_000_000_000},
		{name: "maximum safe int64", amount: 9_223_372_036_854_775_807},
		{name: "just above int64 max", amount: 9_223_372_036_854_775_808, shouldError: true},
		{name: "uint64 wraparound amount", amount: 18_446_744_073_707_351_616, shouldError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidatePayloadProofAmount(test.amount)
			if test.shouldError && err == nil {
				t.Fatalf("expected amount %d to be rejected", test.amount)
			}
			if !test.shouldError && err != nil {
				t.Fatalf("expected amount %d to be accepted: %v", test.amount, err)
			}
		})
	}
}

func TestRejectAmountsOutsideSignedRange(t *testing.T) {
	amounts := []uint64{
		18_446_743_673_709_551_434,
		18_446_743_853_709_551_434,
	}
	for _, amount := range amounts {
		t.Run("signed-range-rejection", func(t *testing.T) {
			if err := ValidatePayloadProofAmount(amount); err == nil {
				t.Fatalf("expected amount %d to be rejected as unsafe for int64 conversion", amount)
			}
		})
	}
}

func TestSuspiciousProofWarningsDoNotClaimLargeSupplyAmountsAreImpossible(t *testing.T) {
	warnings := DetectSuspiciousProofPatterns(2_300_000_000_000)
	if len(warnings) == 0 {
		t.Fatal("expected advisory warnings for a very large amount")
	}
	for _, warning := range warnings {
		lower := strings.ToLower(warning)
		if strings.Contains(lower, "impossible") || strings.Contains(lower, "fabricat") || strings.Contains(lower, "hard cap") {
			t.Fatalf("warning overstates proof: %q", warning)
		}
	}
}
