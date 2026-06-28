package story

import (
	"context"
	"errors"
	"testing"

	"storywork/internal/project"
)

// BDD trace:
//   - Requirement: Milestone 1, Story 1.4, preserve checkpoint integrity.
//   - Scenario: dirty worktrees reject structural mutation before writes, and a
//     checkpoint failure restores files and leaves the repository unstaged.
//   - Test purpose: verify the service rollback path and precondition checks for
//     failed structural mutations.
func TestMutationIntegrityChecksDirtyStateAndRollback(t *testing.T) {
	t.Parallel()

	session := &fakeSession{current: project.Project{Path: "/tmp/story"}, ok: true}
	files := &fakeFileStore{exists: map[string]bool{}}
	git := &fakeGitStore{clean: false}
	index := &fakeIndexStore{}
	service := NewService(session, files, git, index, &fakeIDGenerator{ids: []string{"arc_00000000000000000001"}})

	_, err := service.CreateArc(context.Background(), "Act One")
	if !errors.Is(err, ErrDirtyWorktree) {
		t.Fatalf("CreateArc() error = %v, want ErrDirtyWorktree", err)
	}
	if files.writeCalls != 0 {
		t.Fatalf("write calls = %d, want 0 for dirty worktree", files.writeCalls)
	}
	if git.commitCalls != 0 {
		t.Fatalf("commit calls = %d, want 0 for dirty worktree", git.commitCalls)
	}

	files = &fakeFileStore{exists: map[string]bool{}}
	git = &fakeGitStore{clean: true, commitErr: errors.New("commit failed")}
	index = &fakeIndexStore{}
	service = NewService(session, files, git, index, &fakeIDGenerator{ids: []string{"arc_00000000000000000001"}})

	_, err = service.CreateArc(context.Background(), "Act One")
	if err == nil {
		t.Fatal("CreateArc() error = nil, want commit failure")
	}
	if files.rollbackCalls != 1 {
		t.Fatalf("rollback calls = %d, want 1", files.rollbackCalls)
	}
	if git.unstageCalls != 1 {
		t.Fatalf("unstage calls = %d, want 1", git.unstageCalls)
	}
	if index.rebuildCalls != 2 {
		t.Fatalf("index rebuild calls = %d, want 2", index.rebuildCalls)
	}
}
