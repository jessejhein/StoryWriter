package story

import (
	"context"
	"errors"
	"testing"

	"storywork/internal/project"
)

// BDD trace:
//   - Requirement: Milestone 1, Story 1.1, view the outline.
//   - Scenario: given a valid active project, when I request the outline, then I
//     receive the loaded hierarchy; given no active project, the request fails
//     with the active-project conflict error.
//   - Test purpose: verify outline reads go through the active session and return
//     the loaded outline without mutating state.
func TestOutlineRequiresActiveProjectAndReturnsLoadedTree(t *testing.T) {
	t.Parallel()

	service := NewService(&fakeSession{}, &fakeFileStore{}, &fakeGitStore{}, &fakeIndexStore{}, &fakeIDGenerator{})
	_, err := service.Outline(context.Background())
	if !errors.Is(err, ErrNoActiveProject) {
		t.Fatalf("Outline() error = %v, want ErrNoActiveProject", err)
	}

	expected := NewOutline()
	expected, err = AddArc(expected, "arc_00000000000000000001", "Act One")
	if err != nil {
		t.Fatalf("AddArc() error = %v", err)
	}
	service = NewService(
		&fakeSession{current: project.Project{Path: "/tmp/story"}, ok: true},
		&fakeFileStore{loadOutline: expected},
		&fakeGitStore{clean: true},
		&fakeIndexStore{},
		&fakeIDGenerator{},
	)
	loaded, err := service.Outline(context.Background())
	if err != nil {
		t.Fatalf("Outline() error = %v", err)
	}
	if got := loaded.Arcs[0].Title; got != "Act One" {
		t.Fatalf("arc title = %q, want %q", got, "Act One")
	}
}
