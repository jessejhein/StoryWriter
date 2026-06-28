package story

import (
	"context"
	"testing"

	"storywork/internal/project"
)

// BDD trace:
//   - Requirement: Milestone 1, Story 1.3, reorder structure.
//   - Scenario: given a parent with ordered children, when I reorder them by
//     stable IDs, then only outline.yaml changes, IDs stay stable, and exactly
//     one checkpoint is created for the reorder.
//   - Test purpose: verify reorder orchestration writes only the outline and
//     commits with the exact chapter/scene reorder messages.
func TestReorderWritesOnlyOutlineAndCommitsOnce(t *testing.T) {
	t.Parallel()

	initial := NewOutline()
	var err error
	initial, err = AddArc(initial, "arc_00000000000000000001", "Act One")
	if err != nil {
		t.Fatalf("AddArc() error = %v", err)
	}
	initial, err = AddChapter(initial, "arc_00000000000000000001", "ch_00000000000000000001", "Arrival")
	if err != nil {
		t.Fatalf("AddChapter() error = %v", err)
	}
	initial, err = AddChapter(initial, "arc_00000000000000000001", "ch_00000000000000000002", "Departure")
	if err != nil {
		t.Fatalf("AddChapter() error = %v", err)
	}

	files := &fakeFileStore{loadOutline: initial, exists: map[string]bool{}}
	git := &fakeGitStore{clean: true}
	service := NewService(
		&fakeSession{current: project.Project{Path: "/tmp/story"}, ok: true},
		files,
		git,
		&fakeIndexStore{},
		&fakeIDGenerator{},
	)

	result, err := service.Reorder(context.Background(), ReorderRequest{
		ParentType:      "arc",
		ParentID:        "arc_00000000000000000001",
		OrderedChildIDs: []string{"ch_00000000000000000002", "ch_00000000000000000001"},
	})
	if err != nil {
		t.Fatalf("Reorder() error = %v", err)
	}
	if result.ChangedID != "" {
		t.Fatalf("Reorder() changed_id = %q, want empty", result.ChangedID)
	}
	if len(files.writtenFiles) != 1 {
		t.Fatalf("written files = %d, want 1", len(files.writtenFiles))
	}
	if _, ok := files.writtenFiles["outline.yaml"]; !ok {
		t.Fatal("Reorder() did not write outline.yaml")
	}
	if git.commitCalls != 1 {
		t.Fatalf("commit calls = %d, want 1", git.commitCalls)
	}
	if git.commitMessages[0] != "Reorder chapters in arc_00000000000000000001" {
		t.Fatalf("commit message = %q", git.commitMessages[0])
	}
	if result.Outline.Arcs[0].Chapters[0].ID != "ch_00000000000000000002" {
		t.Fatalf("reordered first chapter ID = %q", result.Outline.Arcs[0].Chapters[0].ID)
	}
}
