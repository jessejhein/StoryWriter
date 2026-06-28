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

// BDD trace:
//   - Requirement: Milestone 1, Story 1.4, preserve checkpoint integrity.
//   - Scenario: given a clean active project, when writing, indexing, or
//     checkpointing fails, canonical state is recovered, Git is left unstaged,
//     the derived index is rebuilt, and no successful checkpoint is reported.
//   - Test purpose: verify every failure stage after the clean check performs
//     the required recovery actions and preserves the original failure.
func TestMutationFailureStagesRecoverCanonicalAndDerivedState(t *testing.T) {
	t.Parallel()

	cause := errors.New("adapter failed")
	tests := []struct {
		name              string
		configure         func(*fakeFileStore, *fakeGitStore, *fakeIndexStore)
		wantRollbackCalls int
		wantUnstageCalls  int
		wantRebuildCalls  int
		wantCommitCalls   int
	}{
		{
			name: "canonical write",
			configure: func(files *fakeFileStore, _ *fakeGitStore, _ *fakeIndexStore) {
				files.writeErr = cause
			},
			wantUnstageCalls: 1,
			wantRebuildCalls: 1,
		},
		{
			name: "canonical reload",
			configure: func(files *fakeFileStore, _ *fakeGitStore, _ *fakeIndexStore) {
				files.reloadErr = cause
			},
			wantRollbackCalls: 1,
			wantUnstageCalls:  1,
			wantRebuildCalls:  1,
		},
		{
			name: "index rebuild",
			configure: func(_ *fakeFileStore, _ *fakeGitStore, index *fakeIndexStore) {
				index.rebuildErr = cause
			},
			wantRollbackCalls: 1,
			wantUnstageCalls:  1,
			wantRebuildCalls:  2,
		},
		{
			name: "Git checkpoint",
			configure: func(_ *fakeFileStore, git *fakeGitStore, _ *fakeIndexStore) {
				git.commitErr = cause
			},
			wantRollbackCalls: 1,
			wantUnstageCalls:  1,
			wantRebuildCalls:  2,
			wantCommitCalls:   1,
		},
	}

	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			files := &fakeFileStore{exists: map[string]bool{}}
			git := &fakeGitStore{clean: true}
			index := &fakeIndexStore{}
			testCase.configure(files, git, index)
			service := NewService(
				&fakeSession{current: project.Project{Path: "/tmp/story"}, ok: true},
				files,
				git,
				index,
				&fakeIDGenerator{ids: []string{"arc_00000000000000000001"}},
			)

			_, err := service.CreateArc(context.Background(), "Act One")
			if !errors.Is(err, cause) {
				t.Fatalf("CreateArc() error = %v, want adapter failure", err)
			}
			if files.rollbackCalls != testCase.wantRollbackCalls {
				t.Fatalf("rollback calls = %d, want %d", files.rollbackCalls, testCase.wantRollbackCalls)
			}
			if git.unstageCalls != testCase.wantUnstageCalls {
				t.Fatalf("unstage calls = %d, want %d", git.unstageCalls, testCase.wantUnstageCalls)
			}
			if index.rebuildCalls != testCase.wantRebuildCalls {
				t.Fatalf("index rebuild calls = %d, want %d", index.rebuildCalls, testCase.wantRebuildCalls)
			}
			if git.commitCalls != testCase.wantCommitCalls {
				t.Fatalf("commit calls = %d, want %d", git.commitCalls, testCase.wantCommitCalls)
			}
		})
	}
}

// BDD trace:
//   - Requirement: Milestone 1 fixed design decision, stable IDs.
//   - Scenario: generated IDs never overwrite an existing canonical entity;
//     collisions retry at most five times and invalid generated IDs are rejected.
//   - Test purpose: verify collision retry, exhaustion, and generated-ID shape
//     validation happen before canonical writes or Git checkpoints.
func TestCreateRetriesAndRejectsUnsafeGeneratedIDs(t *testing.T) {
	t.Parallel()

	collisionID := "arc_00000000000000000001"
	availableID := "arc_00000000000000000002"
	files := &fakeFileStore{exists: map[string]bool{"arcs/" + collisionID + ".yaml": true}}
	git := &fakeGitStore{clean: true}
	service := NewService(
		&fakeSession{current: project.Project{Path: "/tmp/story"}, ok: true},
		files,
		git,
		&fakeIndexStore{},
		&fakeIDGenerator{ids: []string{collisionID, availableID}},
	)

	result, err := service.CreateArc(context.Background(), "Act One")
	if err != nil {
		t.Fatalf("CreateArc() error = %v", err)
	}
	if result.ChangedID != availableID {
		t.Fatalf("changed ID = %q, want %q", result.ChangedID, availableID)
	}

	files = &fakeFileStore{exists: map[string]bool{"arcs/" + collisionID + ".yaml": true}}
	git = &fakeGitStore{clean: true}
	service = NewService(
		&fakeSession{current: project.Project{Path: "/tmp/story"}, ok: true},
		files,
		git,
		&fakeIndexStore{},
		&fakeIDGenerator{ids: []string{collisionID, collisionID, collisionID, collisionID, collisionID}},
	)
	if _, err := service.CreateArc(context.Background(), "Act One"); err == nil {
		t.Fatal("CreateArc() collision exhaustion error = nil")
	}
	if files.writeCalls != 0 || git.commitCalls != 0 {
		t.Fatalf("collision exhaustion wrote %d times and committed %d times", files.writeCalls, git.commitCalls)
	}

	files = &fakeFileStore{exists: map[string]bool{}}
	git = &fakeGitStore{clean: true}
	service = NewService(
		&fakeSession{current: project.Project{Path: "/tmp/story"}, ok: true},
		files,
		git,
		&fakeIndexStore{},
		&fakeIDGenerator{ids: []string{"../../unsafe"}},
	)
	_, err = service.CreateArc(context.Background(), "Act One")
	if !errors.Is(err, ErrInvalidID) {
		t.Fatalf("CreateArc() unsafe ID error = %v, want ErrInvalidID", err)
	}
	if files.writeCalls != 0 || git.commitCalls != 0 {
		t.Fatalf("unsafe ID wrote %d times and committed %d times", files.writeCalls, git.commitCalls)
	}
}
