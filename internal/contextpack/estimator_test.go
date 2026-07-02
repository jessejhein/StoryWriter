package contextpack

// BDD Scenario: 7.2.2 - Exclude irrelevant Codex entries
// Requirements: M7-R08
// Test purpose: Estimated tokens use one conservative byte-based definition across budgeting.

import (
	"errors"
	"testing"
)

// Test: byte estimator counts ASCII and UTF-8 bytes.
// Requirements: M7-R08.
func TestByteEstimatorCountsASCIIAndUTF8Bytes(t *testing.T) {
	t.Parallel()

	estimator := ByteEstimator{}
	if got := estimator.Estimate("hello"); got != 5 {
		t.Fatalf("Estimate(ascii) = %d, want 5", got)
	}
	if got := estimator.Estimate("café"); got != 5 {
		t.Fatalf("Estimate(utf8) = %d, want 5", got)
	}
}

// Test: byte estimator handles empty values and stable sums.
// Requirements: M7-R08.
func TestByteEstimatorHandlesEmptyValuesAndStableSums(t *testing.T) {
	t.Parallel()

	estimator := ByteEstimator{}
	if got := estimator.Estimate(""); got != 0 {
		t.Fatalf("Estimate(empty) = %d, want 0", got)
	}
	if got := estimator.Sum([]string{"ab", "", "cde"}); got != 5 {
		t.Fatalf("Sum() = %d, want 5", got)
	}
	if got := estimator.Sum([]string{"ab", "cde"}); got != estimator.Estimate("ab")+estimator.Estimate("cde") {
		t.Fatalf("Sum() = %d, want additive total", got)
	}
}

// Test: byte estimator rejects integer overflow.
// Requirements: M7-R08.
func TestByteEstimatorRejectsIntegerOverflow(t *testing.T) {
	t.Parallel()

	_, err := addEstimatedTokenLength(int64(int(^uint(0)>>1)), 1)
	if !errors.Is(err, ErrEstimatorOverflow) {
		t.Fatalf("addEstimatedTokenLength() error = %v, want %v", err, ErrEstimatorOverflow)
	}
}
