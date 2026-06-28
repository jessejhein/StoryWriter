package story_test

import (
	"errors"
	"strings"
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

// BDD trace:
//   - Requirement: Milestone 1 mutation rules and stable-ID design.
//   - Scenario: titles are trimmed and limited to 200 Unicode code points, and
//     every entity and parent ID must have its exact opaque prefix and shape.
//   - Test purpose: verify all title boundaries and all ID kinds independently.
func TestCreateStructureValidatesEveryTitleAndIDBoundary(t *testing.T) {
	t.Parallel()

	validArcID := "arc_00000000000000000001"
	validChapterID := "ch_00000000000000000001"
	tests := []struct {
		name string
		run  func() error
		want error
	}{
		{name: "empty title", run: func() error { _, err := story.AddArc(story.NewOutline(), validArcID, " \n\t "); return err }, want: story.ErrInvalidTitle},
		{name: "201 Unicode code points", run: func() error {
			_, err := story.AddArc(story.NewOutline(), validArcID, strings.Repeat("界", 201))
			return err
		}, want: story.ErrInvalidTitle},
		{name: "invalid arc ID", run: func() error {
			_, err := story.AddArc(story.NewOutline(), "arc_ABCDEF0123456789abcd", "Act")
			return err
		}, want: story.ErrInvalidID},
		{name: "invalid chapter parent ID", run: func() error {
			_, err := story.AddChapter(story.NewOutline(), "arc_bad", validChapterID, "Chapter")
			return err
		}, want: story.ErrInvalidID},
		{name: "invalid chapter ID", run: func() error {
			_, err := story.AddChapter(story.NewOutline(), validArcID, "ch_bad", "Chapter")
			return err
		}, want: story.ErrInvalidID},
		{name: "invalid scene parent ID", run: func() error {
			_, err := story.AddScene(story.NewOutline(), "ch_bad", "scn_00000000000000000001", "Scene")
			return err
		}, want: story.ErrInvalidID},
		{name: "invalid scene ID", run: func() error {
			_, err := story.AddScene(story.NewOutline(), validChapterID, "scn_bad", "Scene")
			return err
		}, want: story.ErrInvalidID},
	}

	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if err := testCase.run(); !errors.Is(err, testCase.want) {
				t.Fatalf("error = %v, want %v", err, testCase.want)
			}
		})
	}

	if _, err := story.AddArc(story.NewOutline(), validArcID, strings.Repeat("界", 200)); err != nil {
		t.Fatalf("200-code-point title error = %v", err)
	}
}
