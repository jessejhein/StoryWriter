// BDD Scenario: 3.4.2 - Validate revisions
// Requirements: M3-R17
// Test purpose: Exported revision helpers enforce exact SHA-256 token shapes and hashes.
package codex

import (
	"errors"
	"testing"
)

func TestRevisionHelpersValidateAndComputeCanonicalTokens(t *testing.T) {
	t.Parallel()

	// Test: revision validation accepts only sha256 plus 64 lowercase hex characters, and revision computation matches a known literal digest.
	// Requirements: M3-R17
	const want = "sha256:ebf9f5fc52186339448fda2cea1737fbd9398f880116d184c05cef488e638ecb"
	if err := ValidateRevision(want); err != nil {
		t.Fatalf("ValidateRevision(valid) error = %v", err)
	}
	for _, invalid := range []string{"", "sha256:ABC", "sha256:14d07fc2b2769447b8050b95dcfa4ce3f875c9e67c8fe23c4c0d0d09b9f1a31", "sha512:14d07fc2b2769447b8050b95dcfa4ce3f875c9e67c8fe23c4c0d0d09b9f1a31c"} {
		if err := ValidateRevision(invalid); !errors.Is(err, ErrInvalidRevision) {
			t.Fatalf("ValidateRevision(%q) error = %v, want %v", invalid, err, ErrInvalidRevision)
		}
	}
	if got := ComputeRevision([]byte("canonical fixture")); got != want {
		t.Fatalf("ComputeRevision() = %q, want %q", got, want)
	}
	if changed := ComputeRevision([]byte("canonical fixture.")); changed == want {
		t.Fatalf("ComputeRevision() should change for different bytes")
	}
}
