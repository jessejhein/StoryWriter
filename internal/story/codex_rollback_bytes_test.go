// BDD Scenario: 3.4.4 - Roll back failures
// Requirements: M3-R15, M3-R16
// Test purpose: With real filesystem, Git, and index adapters, a checkpoint failure on a new Codex file removes the file and restores the prior index/worktree, while a checkpoint failure on an existing file restores its exact previous canonical bytes.
package story_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"storywork/internal/codex"
	"storywork/internal/gitstore"
	"storywork/internal/index"
	"storywork/internal/project"
	"storywork/internal/story"
	"storywork/internal/storyfile"
	"storywork/internal/workspace"
)

// failingGitStore wraps a real gitstore adapter but fails CommitAll after the
// first successful commit, so the index has already been rebuilt by the time
// the checkpoint fails. This exercises the real rollback path.
type failingGitStore struct {
	delegate    *gitstore.Store
	commitErr   error
	commitCalls int
	failAfter   int
}

func (f *failingGitStore) IsClean(ctx context.Context, path string) (bool, error) {
	return f.delegate.IsClean(ctx, path)
}
func (f *failingGitStore) CommitAll(ctx context.Context, path, message string) error {
	f.commitCalls++
	if f.commitErr != nil && f.commitCalls > f.failAfter {
		return f.commitErr
	}
	return f.delegate.CommitAll(ctx, path, message)
}
func (f *failingGitStore) UnstageAll(ctx context.Context, path string) error {
	return f.delegate.UnstageAll(ctx, path)
}

func TestRollbackRemovesNewCodexFileWhenCheckpointFails(t *testing.T) {
	t.Parallel()

	// Test: a checkpoint failure on a new Codex entry removes the new file, leaves the worktree clean, and creates no commit (acceptance: new-file rollback removes the file).
	// Requirements: M3-R16
	ctx := context.Background()
	projectPath := filepath.Join(t.TempDir(), "rollback-new")
	realGit := gitstore.New("git")
	disposableIndex := index.New()
	projectService := project.NewService(realGit, disposableIndex, func() time.Time { return time.Date(2026, time.June, 28, 12, 0, 0, 0, time.UTC) })
	created, err := projectService.Create(ctx, project.CreateRequest{Name: "Rollback New", Path: projectPath})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	session := workspace.NewSession()
	session.Set(created)
	failingGit := &failingGitStore{delegate: realGit, commitErr: errCheckpointFailed, failAfter: 0}
	service := story.NewService(session, storyfile.New(), failingGit, disposableIndex, &staticIDGenerator{ids: []string{"char_0123456789abcdef0123"}})

	_, err = service.CreateCodexEntry(ctx, codex.SaveEntryRequest{
		Type:        codex.TypeCharacter,
		Name:        "Ben",
		Aliases:     []string{},
		Tags:        []string{},
		Description: "Guide.",
		Metadata:    map[string]string{},
	})
	if err == nil {
		t.Fatal("CreateCodexEntry() error = nil, want checkpoint failure")
	}
	entryPath := filepath.Join(projectPath, "codex", "characters", "char_0123456789abcdef0123.yaml")
	if _, err := os.Stat(entryPath); !os.IsNotExist(err) {
		t.Fatalf("new entry file should not exist after rollback; Stat err = %v", err)
	}
	clean, err := realGit.IsClean(ctx, projectPath)
	if err != nil {
		t.Fatalf("IsClean() error = %v", err)
	}
	if !clean {
		t.Fatal("worktree dirty after new-file rollback, want clean")
	}
	if got := gitCommitCount(t, ctx, projectPath); got != "1" {
		t.Fatalf("commit count = %s, want 1 (only the project-create commit)", got)
	}
}

func TestRollbackRestoresExistingCodexFileBytesWhenCheckpointFails(t *testing.T) {
	t.Parallel()

	// Test: a checkpoint failure on an edited (existing) Codex entry restores its exact previous canonical bytes, leaves the worktree clean, and creates no extra commit (acceptance: existing-file rollback restores exact previous bytes).
	// Requirements: M3-R16
	ctx := context.Background()
	projectPath := filepath.Join(t.TempDir(), "rollback-existing")
	realGit := gitstore.New("git")
	disposableIndex := index.New()
	projectService := project.NewService(realGit, disposableIndex, func() time.Time { return time.Date(2026, time.June, 28, 12, 0, 0, 0, time.UTC) })
	created, err := projectService.Create(ctx, project.CreateRequest{Name: "Rollback Existing", Path: projectPath})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	session := workspace.NewSession()
	session.Set(created)
	realService := story.NewService(session, storyfile.New(), realGit, disposableIndex, &staticIDGenerator{ids: []string{"char_0123456789abcdef0123"}})
	createdEntry, err := realService.CreateCodexEntry(ctx, codex.SaveEntryRequest{
		Type:        codex.TypeCharacter,
		Name:        "Ben",
		Aliases:     []string{},
		Tags:        []string{},
		Description: "Original guide.",
		Metadata:    map[string]string{},
	})
	if err != nil {
		t.Fatalf("CreateCodexEntry() error = %v", err)
	}
	entryPath := filepath.Join(projectPath, "codex", "characters", createdEntry.ID+".yaml")
	originalBytes, err := os.ReadFile(entryPath)
	if err != nil {
		t.Fatalf("ReadFile(original) error = %v", err)
	}
	// Now wire a failing git that fails the first CommitAll it sees (the edit),
	// exercising the real rollback path against the already-committed canon.
	failingGit := &failingGitStore{delegate: realGit, commitErr: errCheckpointFailed, failAfter: 0}
	failingService := story.NewService(session, storyfile.New(), failingGit, disposableIndex, &staticIDGenerator{ids: []string{"unused"}})
	_, err = failingService.UpdateCodexEntry(ctx, createdEntry.ID, codex.SaveEntryRequest{
		Name:             "Ben Kenobi",
		Aliases:          []string{},
		Tags:             []string{},
		Description:      "Changed description that must not persist.",
		Metadata:         map[string]string{},
		ExpectedRevision: createdEntry.Revision,
	})
	if err == nil {
		t.Fatal("UpdateCodexEntry() error = nil, want checkpoint failure")
	}
	restoredBytes, err := os.ReadFile(entryPath)
	if err != nil {
		t.Fatalf("ReadFile(restored) error = %v", err)
	}
	if !bytesEqual(restoredBytes, originalBytes) {
		t.Fatalf("existing entry bytes not restored after rollback:\nwant:\n%s\ngot:\n%s", originalBytes, restoredBytes)
	}
	clean, err := realGit.IsClean(ctx, projectPath)
	if err != nil {
		t.Fatalf("IsClean() error = %v", err)
	}
	if !clean {
		t.Fatal("worktree dirty after existing-file rollback, want clean")
	}
	if got := gitCommitCount(t, ctx, projectPath); got != "2" {
		t.Fatalf("commit count = %s, want 2 (project-create + entry-create)", got)
	}
}

var errCheckpointFailed = errCheckpoint("checkpoint failed")

type errCheckpoint string

func (e errCheckpoint) Error() string { return string(e) }
