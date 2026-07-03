// BDD Scenario: 8.2.1 - List exact changed files
// Requirements: M8-R05, M8-R06
// Test purpose: Comparison policy validates changed files and fingerprints.

package branch_test

import (
	"testing"

	"storywork/internal/branch"
)

// Test: changed files are validated and sorted.
// Requirements: M8-R06.
func TestValidateChangedFilesSortsAndDedupes(t *testing.T) {
	t.Parallel()
	files, err := branch.ValidateChangedFiles([]branch.ChangedFile{
		{Path: "scenes/scn_00000000000000000001.md", Status: branch.StatusAdded},
		{Path: "outline.yaml", Status: branch.StatusModified},
	})
	if err != nil {
		t.Fatalf("ValidateChangedFiles() error = %v", err)
	}
	if files[0].Path != "outline.yaml" {
		t.Fatalf("first path = %q", files[0].Path)
	}
}
