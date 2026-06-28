package story_test

import (
	"errors"
	"testing"

	"storywork/internal/story"
)

// BDD trace:
//   - Requirement: Milestone 1, Story 1.3, reorder structure.
//   - Scenario: given an existing parent and children, when I reorder chapters or
//     scenes with a complete stable-ID permutation, then order changes, display
//     labels update, and IDs remain unchanged; invalid permutations are rejected.
//   - Test purpose: verify pure reorder decisions before any file, Git, or HTTP
//     integration is involved.
func TestReorderValidatesPermutationsAndPreservesIDs(t *testing.T) {
	t.Parallel()

	outline := story.NewOutline()
	var err error
	outline, err = story.AddArc(outline, "arc_00000000000000000001", "Act One")
	if err != nil {
		t.Fatalf("AddArc() error = %v", err)
	}
	outline, err = story.AddChapter(outline, "arc_00000000000000000001", "ch_00000000000000000001", "Arrival")
	if err != nil {
		t.Fatalf("AddChapter() error = %v", err)
	}
	outline, err = story.AddChapter(outline, "arc_00000000000000000001", "ch_00000000000000000002", "Departure")
	if err != nil {
		t.Fatalf("AddChapter() error = %v", err)
	}
	outline, err = story.AddScene(outline, "ch_00000000000000000001", "scn_00000000000000000001", "A")
	if err != nil {
		t.Fatalf("AddScene() error = %v", err)
	}
	outline, err = story.AddScene(outline, "ch_00000000000000000001", "scn_00000000000000000002", "B")
	if err != nil {
		t.Fatalf("AddScene() error = %v", err)
	}

	reordered, err := story.Reorder(outline, story.ReorderRequest{
		ParentType:      "arc",
		ParentID:        "arc_00000000000000000001",
		OrderedChildIDs: []string{"ch_00000000000000000002", "ch_00000000000000000001"},
	})
	if err != nil {
		t.Fatalf("Reorder(chapters) error = %v", err)
	}
	if got := reordered.Arcs[0].Chapters[0].ID; got != "ch_00000000000000000002" {
		t.Fatalf("first chapter ID = %q, want %q", got, "ch_00000000000000000002")
	}
	if got := reordered.Arcs[0].Chapters[0].DisplayLabel; got != "Chapter 1.1" {
		t.Fatalf("first chapter label = %q, want %q", got, "Chapter 1.1")
	}
	if got := reordered.Arcs[0].Chapters[1].DisplayLabel; got != "Chapter 1.2" {
		t.Fatalf("second chapter label = %q, want %q", got, "Chapter 1.2")
	}

	reordered, err = story.Reorder(outline, story.ReorderRequest{
		ParentType:      "chapter",
		ParentID:        "ch_00000000000000000001",
		OrderedChildIDs: []string{"scn_00000000000000000002", "scn_00000000000000000001"},
	})
	if err != nil {
		t.Fatalf("Reorder(scenes) error = %v", err)
	}
	if got := reordered.Arcs[0].Chapters[0].Scenes[0].ID; got != "scn_00000000000000000002" {
		t.Fatalf("first scene ID = %q, want %q", got, "scn_00000000000000000002")
	}
	if got := reordered.Arcs[0].Chapters[0].Scenes[0].DisplayLabel; got != "Scene 1.1.1" {
		t.Fatalf("first scene label = %q, want %q", got, "Scene 1.1.1")
	}

	_, err = story.Reorder(outline, story.ReorderRequest{
		ParentType:      "chapter",
		ParentID:        "ch_00000000000000000001",
		OrderedChildIDs: []string{"scn_00000000000000000001"},
	})
	if !errors.Is(err, story.ErrInvalidReorder) {
		t.Fatalf("Reorder() error = %v, want ErrInvalidReorder", err)
	}

	_, err = story.Reorder(outline, story.ReorderRequest{
		ParentType:      "chapter",
		ParentID:        "ch_00000000000000000001",
		OrderedChildIDs: []string{"scn_00000000000000000001", "scn_00000000000000000001"},
	})
	if !errors.Is(err, story.ErrInvalidReorder) {
		t.Fatalf("Reorder() error = %v, want ErrInvalidReorder for duplicates", err)
	}
}
