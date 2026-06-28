package story_test

import (
	"testing"

	"storywork/internal/story"
)

// BDD trace:
//   - Requirement: Milestone 1, Story 1.1, view the outline.
//   - Scenario: given a valid project outline, when I request the outline, then
//     I receive ordered arcs, chapters, and scenes with stable IDs, titles, and
//     derived display labels.
//   - Test purpose: verify the in-memory outline model derives labels from
//     current nesting order without changing stored IDs or titles.
func TestOutlineDerivesDisplayLabelsFromOrder(t *testing.T) {
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
	outline, err = story.AddScene(outline, "ch_00000000000000000001", "scn_00000000000000000001", "The Station")
	if err != nil {
		t.Fatalf("AddScene() error = %v", err)
	}

	arc := outline.Arcs[0]
	if arc.DisplayLabel != "Arc 1" {
		t.Fatalf("arc label = %q, want %q", arc.DisplayLabel, "Arc 1")
	}
	chapter := arc.Chapters[0]
	if chapter.DisplayLabel != "Chapter 1.1" {
		t.Fatalf("chapter label = %q, want %q", chapter.DisplayLabel, "Chapter 1.1")
	}
	scene := chapter.Scenes[0]
	if scene.DisplayLabel != "Scene 1.1.1" {
		t.Fatalf("scene label = %q, want %q", scene.DisplayLabel, "Scene 1.1.1")
	}
	if arc.ID != "arc_00000000000000000001" || chapter.ID != "ch_00000000000000000001" || scene.ID != "scn_00000000000000000001" {
		t.Fatalf("IDs changed unexpectedly: %#v", outline)
	}
}
