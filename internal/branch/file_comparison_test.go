// BDD Scenario: 8.2.2 - Show side-by-side text
// Requirements: M8-R05, M8-R07
// Test purpose: File comparison rejects paths outside the current comparison.

package branch_test

import (
	"context"
	"errors"
	"testing"

	"storywork/internal/branch"
	"storywork/internal/mutation"
)

// Test: path must belong to current comparison inventory.
// Requirements: M8-R07.
func TestIndexChangedFilesRejectsUnknownPath(t *testing.T) {
	t.Parallel()
	index := branch.IndexChangedFiles([]branch.ChangedFile{{Path: "outline.yaml", Status: branch.StatusModified}})
	if _, ok := index["scenes/scn_00000000000000000001.md"]; ok {
		t.Fatal("unexpected path in comparison index")
	}
	if _, err := branch.ValidateProjectPath("../outline.yaml"); !errors.Is(err, branch.ErrInvalidProjectPath) {
		t.Fatalf("err = %v", err)
	}
}

// Test: added and deleted comparisons return an explicit absent side and exact
// text from the present Git blob without checkout.
// Requirements: M8-R05, M8-R07.
func TestLoadFileComparisonRepresentsAddedAndDeletedSides(t *testing.T) {
	t.Parallel()
	mainHead := branch.CommitID("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	experimentHead := branch.CommitID("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	files := []branch.ChangedFile{{Path: "scenes/scn_00000000000000000001.md", Status: branch.StatusAdded}, {Path: "scenes/scn_00000000000000000002.md", Status: branch.StatusDeleted}}
	repo := &fakeRepo{
		experiments: []branch.ExperimentRef{{ID: "brn_0123456789abcdef0123", BranchName: "branch/test-exp-0123456789abcdef0123", Head: experimentHead}},
		mainHead:    mainHead, compareFiles: files,
		blobSides: map[string]branch.TextSide{
			string(experimentHead) + "|scenes/scn_00000000000000000001.md": {Exists: true, Text: "added\n"},
			string(mainHead) + "|scenes/scn_00000000000000000002.md":       {Exists: true, Text: "deleted\n"},
		},
	}
	service := branch.NewService(repo, &fakeIndex{}, mutation.NewCoordinator(), branch.SessionAdapter{PathFn: func() (string, bool) { return "/tmp/project", true }}, nil, nil, &staticIDs{id: "brn_0123456789abcdef0123"})
	added, err := service.LoadFileComparison(context.Background(), "brn_0123456789abcdef0123", "scenes/scn_00000000000000000001.md")
	if err != nil {
		t.Fatal(err)
	}
	if added.Canon.Exists || !added.Experiment.Exists || added.Experiment.Text != "added\n" {
		t.Fatalf("added=%#v", added)
	}
	deleted, err := service.LoadFileComparison(context.Background(), "brn_0123456789abcdef0123", "scenes/scn_00000000000000000002.md")
	if err != nil {
		t.Fatal(err)
	}
	if !deleted.Canon.Exists || deleted.Experiment.Exists || deleted.Canon.Text != "deleted\n" {
		t.Fatalf("deleted=%#v", deleted)
	}
}
