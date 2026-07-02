package story

import (
	"context"
	"testing"
	"time"

	"storywork/internal/mutation"
	"storywork/internal/project"
)

// TestStoryMutationsUseInjectedApplicationCoordinator proves import/review
// transactions can exclude ordinary canonical story writes.
func TestStoryMutationsUseInjectedApplicationCoordinator(t *testing.T) {
	t.Parallel()

	coordinator := mutation.NewCoordinator()
	service := NewService(
		&fakeSession{current: project.Project{Path: t.TempDir()}, ok: true},
		&fakeFileStore{loadOutline: Outline{Version: OutlineVersion, Arcs: []Arc{}}, exists: map[string]bool{}},
		&fakeGitStore{clean: true},
		&fakeIndexStore{},
		&fakeIDGenerator{ids: []string{"arc_00000000000000000001"}},
	).WithMutationCoordinator(coordinator)

	coordinator.Lock()
	result := make(chan error, 1)
	go func() {
		_, err := service.CreateArc(context.Background(), "Blocked Arc")
		result <- err
	}()

	select {
	case err := <-result:
		t.Fatalf("CreateArc() completed while shared transaction was held: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	coordinator.Unlock()
	if err := <-result; err != nil {
		t.Fatalf("CreateArc() error = %v", err)
	}
}
