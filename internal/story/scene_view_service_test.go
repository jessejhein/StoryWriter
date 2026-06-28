package story

import (
	"context"
	"errors"
	"testing"

	"storywork/internal/project"
)

// BDD trace:
//   - Requirement: M2-R01, M2-R02.
//   - Scenario: 2.1.3 — Invalid or unknown scene.
//   - Test purpose: verify scene loads reject malformed IDs, unknown stable IDs,
//     and missing active projects before any editor payload is returned.
func TestLoadSceneValidatesActiveProjectAndStableID(t *testing.T) {
	t.Parallel()

	service := NewService(&fakeSession{}, &fakeFileStore{}, &fakeGitStore{}, &fakeIndexStore{}, &fakeIDGenerator{})
	if _, err := service.LoadScene(context.Background(), "bad"); !errors.Is(err, ErrNoActiveProject) {
		t.Fatalf("LoadScene(no project) error = %v, want ErrNoActiveProject", err)
	}

	service = NewService(
		&fakeSession{current: project.Project{Path: "/tmp/story"}, ok: true},
		&fakeFileStore{loadOutline: NewOutline()},
		&fakeGitStore{},
		&fakeIndexStore{},
		&fakeIDGenerator{},
	)
	if _, err := service.LoadScene(context.Background(), "bad"); !errors.Is(err, ErrInvalidID) {
		t.Fatalf("LoadScene(invalid id) error = %v, want ErrInvalidID", err)
	}
	if _, err := service.LoadScene(context.Background(), "scn_00000000000000000001"); !errors.Is(err, ErrSceneNotFound) {
		t.Fatalf("LoadScene(unknown) error = %v, want ErrSceneNotFound", err)
	}
}
