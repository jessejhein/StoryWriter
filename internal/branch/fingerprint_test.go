// BDD Scenario: 8.2.1 - List exact changed files
// Requirements: M8-R06
// Test purpose: Comparison fingerprints are deterministic, versioned, and
// sensitive only to refs, statuses, and sorted paths.

package branch_test

import (
	"errors"
	"fmt"
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
	want := "sha256:6ecf098443c151ec088d046f2060f3c3394d7109f8961a7a349efcb2162cc878"
	if got != want {
		t.Fatalf("fingerprint = %q, want %q", got, want)
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

// Test: fingerprinting rejects invalid changed-file inputs instead of hashing
// malformed comparison state.
// Requirements: M8-R06, M8-R07.
func TestComputeFingerprintRejectsInvalidChangedFiles(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		files   []branch.ChangedFile
		wantErr error
	}{
		{
			name:    "invalid path",
			files:   []branch.ChangedFile{{Path: "/abs", Status: branch.StatusModified}},
			wantErr: branch.ErrInvalidProjectPath,
		},
		{
			name:    "invalid status",
			files:   []branch.ChangedFile{{Path: "outline.yaml", Status: branch.ChangedStatus("renamed")}},
			wantErr: branch.ErrInvalidChangedStatus,
		},
		{
			name: "duplicate path",
			files: []branch.ChangedFile{
				{Path: "outline.yaml", Status: branch.StatusModified},
				{Path: "outline.yaml", Status: branch.StatusAdded},
			},
			wantErr: branch.ErrInvalidChangedStatus,
		},
		{
			name: "too many paths",
			files: func() []branch.ChangedFile {
				files := make([]branch.ChangedFile, branch.MaxChangedPaths+1)
				for index := range files {
					files[index] = branch.ChangedFile{
						Path:   branch.ProjectPath(fmt.Sprintf("imports/raw/a/part-%03d.md", index)),
						Status: branch.StatusModified,
					}
				}
				return files
			}(),
			wantErr: branch.ErrTooManyChangedPaths,
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := branch.ComputeFingerprint(
				"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				"cccccccccccccccccccccccccccccccccccccccc",
				testCase.files,
			)
			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("ComputeFingerprint() err = %v, want errors.Is(%v)", err, testCase.wantErr)
			}
		})
	}
}
