// Copyright 2017-2025 DERO Project. All rights reserved.
// Use of this source code in any form is governed by RESEARCH license.
// license can be found in the LICENSE file.
//
// Security enhancement: payload proof amount validation.

package proof

import "fmt"

const (
	// Maximum safe value for int64 conversion. This prevents uint64-to-int64
	// wraparound in proof code paths that use SetInt64.
	maxInt64Safe = 9223372036854775807

	// For display and calculations: 1 DERO = 100,000 atomic units.
	atomicUnitsPerDero = 100_000
)

// ValidatePayloadProofAmount checks only signed-range conversion safety. It
// does not prove that an amount is backed by a balance, nor does it enforce a
// protocol supply cap. Callers must validate the cryptographic proof itself.
func ValidatePayloadProofAmount(amount uint64) error {
	if amount > maxInt64Safe {
		return fmt.Errorf("amount %d exceeds maximum safe integer (%d) - possible signed wraparound",
			amount, maxInt64Safe)
	}
	return nil
}

// DetectSuspiciousProofPatterns returns advisory warnings for amounts that
// merit additional scrutiny. These warnings are not proof-verification results.
func DetectSuspiciousProofPatterns(amount uint64) []string {
	var warnings []string

	// Warning 1: Near int64 boundary (possible wraparound)
	boundaryThreshold := uint64(maxInt64Safe * 9 / 10)
	if amount > boundaryThreshold {
		warnings = append(warnings, "amount near int64 maximum - possible wraparound attempt")
	}

	// Warning 2: Very large amount (> 1M DERO) - notable, but not proof of invalidity.
	largeThreshold := uint64(1_000_000 * atomicUnitsPerDero)
	if amount > largeThreshold {
		amountDero := amount / atomicUnitsPerDero
		warnings = append(warnings, fmt.Sprintf("large transfer amount: %d DERO; verify the proof and forensic context", amountDero))
	}

	// Warning 3: Suspiciously round number (exact multiple of 1M DERO).
	roundThreshold := uint64(1_000_000 * atomicUnitsPerDero)
	if amount >= roundThreshold && amount%roundThreshold == 0 {
		amountDero := amount / atomicUnitsPerDero
		warnings = append(warnings, fmt.Sprintf("suspiciously round number (%d DERO exactly)", amountDero))
	}

	return warnings
}
