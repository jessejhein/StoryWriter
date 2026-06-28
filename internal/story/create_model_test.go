package story_test

import (
	"errors"
	"testing"

	"storywork/internal/story"
)

// BDD trace:
//   - Requirement: Milestone 1, Story 1.2, create structure.
//   - Scenario: given a clean active project, when I create an arc, a chapter in
//     that arc, and a scene in that chapter, then the hierarchy is appended in
//     creation order and each ID keeps the correct stable prefix.
//   - Test purpose: verify pure outline mutations append valid nodes and reject
//     invalid titles, invalid IDs, and unknown parents before persistence runs.
func TestCreateStructureAppendsNodesAndValidatesInputs(t *testing.T) {
	t.Parallel()

	outline := story.NewOutline()
	var err error
	outline, err = story.AddArc(outline, "arc_00000000000000000001", "  Act One  ")
	if err != nil {
		t.Fatalf("AddArc() error = %v", err)
	}
	outline, err = story.AddChapter(outline, "arc_00000000000000000001", "ch_00000000000000000001", "Arrival")
	if err != nil {
		t.Fatalf("AddChapter() error = %v", err)
	}
	outline, err = story.AddScene(outline, "ch_00000000000000000001", "scn_00000000000000000001", "The Station")
	if err != nil {
		t.Fatalf("AddScene() error = %v", err)
	}

	if got := outline.Arcs[0].Title; got != "Act One" {
		t.Fatalf("trimmed title = %q, want %q", got, "Act One")
	}
	if got := outline.Arcs[0].Chapters[0].Title; got != "Arrival" {
		t.Fatalf("chapter title = %q, want %q", got, "Arrival")
	}
	if got := outline.Arcs[0].Chapters[0].Scenes[0].Title; got != "The Station" {
		t.Fatalf("scene title = %q, want %q", got, "The Station")
	}

	_, err = story.AddChapter(outline, "arc_00000000000000000009", "ch_00000000000000000002", "Missing")
	if !errors.Is(err, story.ErrParentNotFound) {
		t.Fatalf("AddChapter() error = %v, want ErrParentNotFound", err)
	}

	_, err = story.AddArc(outline, "arc_invalid", "Another Arc")
	if !errors.Is(err, story.ErrInvalidID) {
		t.Fatalf("AddArc() error = %v, want ErrInvalidID", err)
	}

	_, err = story.AddScene(outline, "ch_00000000000000000001", "scn_00000000000000000002", "")
	if !errors.Is(err, story.ErrInvalidTitle) {
		t.Fatalf("AddScene() error = %v, want ErrInvalidTitle", err)
	}
}
