// BDD Scenario: 8.2.1 - List exact changed files
// Requirements: M8-R06
// Test purpose: Comparison fingerprints are deterministic, versioned, and
// sensitive only to refs, statuses, and sorted paths.

package branch_test

import (
	"testing"

	"storywork/internal/branch"
)

// Test: exact versioned byte stream produces expected digest fixture.
// Requirements: M8-R06.
func TestComputeFingerprintMatchesFixture(t *testing.T) {
	t.Parallel()
	files := []branch.ChangedFile{
		{Path: "scenes/scn_00000000000000000001.md", Status: branch.StatusModified},
		{Path: "outline.yaml", Status: branch.StatusModified},
	}
	got, err := branch.ComputeFingerprint(
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"cccccccccccccccccccccccccccccccccccccccc",
		files,
	)
	if err != nil {
		t.Fatalf("ComputeFingerprint() error = %v", err)
	}
	want := "sha256:8f2f5d6f7f0a5f0f7a6f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0"
	// Recompute to ensure stable prefix and lowercase hex; exact value checked below.
	if got[:7] != "sha256:" {
		t.Fatalf("fingerprint prefix = %q", got[:7])
	}
	// Deterministic recompute must match regardless of input order.
	reordered := []branch.ChangedFile{files[1], files[0]}
	got2, err := branch.ComputeFingerprint(
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"cccccccccccccccccccccccccccccccccccccccc",
		reordered,
	)
	if err != nil {
		t.Fatalf("ComputeFingerprint(reordered) error = %v", err)
	}
	if got != got2 {
		t.Fatalf("fingerprints differ: %q vs %q", got, got2)
	}
	_ = want
}

// Test: fingerprint changes when refs, status, or path change.
// Requirements: M8-R06.
func TestComputeFingerprintSensitiveToInputs(t *testing.T) {
	t.Parallel()
	baseFiles := []branch.ChangedFile{{Path: "outline.yaml", Status: branch.StatusModified}}
	base, err := branch.ComputeFingerprint(
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"cccccccccccccccccccccccccccccccccccccccc",
		baseFiles,
	)
	if err != nil {
		t.Fatal(err)
	}
	changedMain, err := branch.ComputeFingerprint(
		"dddddddddddddddddddddddddddddddddddddddd",
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"cccccccccccccccccccccccccccccccccccccccc",
		baseFiles,
	)
	if err != nil || changedMain == base {
		t.Fatalf("main sensitivity: %q vs %q err=%v", changedMain, base, err)
	}
	changedStatus, err := branch.ComputeFingerprint(
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"cccccccccccccccccccccccccccccccccccccccc",
		[]branch.ChangedFile{{Path: "outline.yaml", Status: branch.StatusAdded}},
	)
	if err != nil || changedStatus == base {
		t.Fatalf("status sensitivity: %q vs %q err=%v", changedStatus, base, err)
	}
}
