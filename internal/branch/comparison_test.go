// BDD Scenario: 8.2.1 - List exact changed files
// Requirements: M8-R05, M8-R06
// Test purpose: Comparison policy validates changed files and fingerprints.

package branch_test

import (
	"context"
	"errors"
	"testing"

	"storywork/internal/branch"
	"storywork/internal/mutation"
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

// Test: malformed repository comparison rows fail closed before a comparison is returned.
// Requirements: M8-R06, M8-R07.
func TestLoadComparisonRejectsInvalidRepositoryComparisonRows(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{
		status: branch.RepositoryStatus{
			ActiveBranch:   "branch/test-exp-0123456789abcdef0123",
			IsManaged:      true,
			IsClean:        true,
			MainHead:       "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			ExperimentID:   "brn_0123456789abcdef0123",
			ExperimentHead: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		},
		experiments: []branch.ExperimentRef{{
			ID:         "brn_0123456789abcdef0123",
			BranchName: "branch/test-exp-0123456789abcdef0123",
			Head:       "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			BaseHead:   "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		}},
		mainHead: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		compareFiles: []branch.ChangedFile{{
			Path:   "/abs",
			Status: branch.StatusModified,
		}},
	}
	service := branch.NewService(repo, &fakeIndex{}, mutation.NewCoordinator(), branch.SessionAdapter{
		PathFn: func() (string, bool) { return "/tmp/project", true },
	}, nil, nil, &staticIDs{id: "brn_0123456789abcdef0123"})
	_, err := service.LoadComparison(context.Background(), "brn_0123456789abcdef0123")
	if !errors.Is(err, branch.ErrInvalidProjectPath) {
		t.Fatalf("LoadComparison() err = %v, want ErrInvalidProjectPath", err)
	}
}
