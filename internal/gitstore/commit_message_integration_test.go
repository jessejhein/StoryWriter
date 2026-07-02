package gitstore_test

// BDD Scenario: 7.5.1 - Record causal and dependency trailers
// Requirements: M7-R13
// Test purpose: Verify Git writes exact commit bodies for operation metadata.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"storywork/internal/gitstore"
)

func initRepo(t *testing.T, dir string) *gitstore.Store {
	t.Helper()
	ctx := context.Background()
	store := gitstore.New("git")
	if err := store.Init(ctx, dir); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	if err := store.CommitAll(ctx, dir, "Initialize story project"); err != nil {
		t.Fatalf("initial CommitAll() error = %v", err)
	}
	return store
}

func commitBody(t *testing.T, dir string) string {
	t.Helper()
	command := exec.CommandContext(context.Background(), "git", "-C", dir, "log", "-1", "--format=%B")
	output, err := command.Output()
	if err != nil {
		t.Fatalf("git log body: %v", err)
	}
	return string(output)
}

// Test: CommitAllMessage writes exact subject and trailer bytes.
// Requirements: M7-R13.
func TestCommitAllMessageWritesExactSubjectAndTrailers(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "scene.md"), []byte("Alpha\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := initRepo(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "scene.md"), []byte("Beta\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := store.CommitAllMessage(ctx, dir, gitstore.CommitMessage{
		Subject:     "Accept AI patch run_aaaaaaaaaaaaaaaaaaaa",
		OperationID: "run_aaaaaaaaaaaaaaaaaaaa",
		Scope:       "selection:scn_0123456789abcdef0123",
	}); err != nil {
		t.Fatalf("CommitAllMessage() error = %v", err)
	}
	want := strings.Join([]string{
		"Accept AI patch run_aaaaaaaaaaaaaaaaaaaa",
		"",
		"Storywork-Operation-ID: run_aaaaaaaaaaaaaaaaaaaa",
		"Storywork-Scope: selection:scn_0123456789abcdef0123",
		"",
	}, "\n")
	if got, wantBody := strings.TrimRight(commitBody(t, dir), "\n"), strings.TrimRight(want, "\n"); got != wantBody {
		t.Fatalf("commit body = %q, want %q", got, wantBody)
	}
}

// Test: CommitAll preserves existing subject-only commit behavior.
// Requirements: M7-R19.
func TestCommitAllSubjectPreservesExistingCommitBehavior(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "scene.md"), []byte("Alpha\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := initRepo(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "scene.md"), []byte("Beta\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := store.CommitAll(ctx, dir, "Save scene scn_0123456789abcdef0123"); err != nil {
		t.Fatalf("CommitAll() error = %v", err)
	}
	if got := strings.TrimRight(commitBody(t, dir), "\n"); got != "Save scene scn_0123456789abcdef0123" {
		t.Fatalf("commit body = %q", got)
	}
}

// Test: CommitAllMessage failure leaves the repository recoverable.
// Requirements: M7-R15.
func TestCommitAllMessageFailureLeavesRepositoryRecoverable(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "scene.md"), []byte("Alpha\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := initRepo(t, dir)
	ctx := context.Background()
	if err := store.CommitAllMessage(ctx, dir, gitstore.CommitMessage{
		Subject:     "Accept AI patch run_bad",
		OperationID: "run_bad",
		Scope:       "selection:scn_0123456789abcdef0123",
	}); err == nil {
		t.Fatal("CommitAllMessage() error = nil, want validation failure")
	}
	clean, err := store.IsClean(ctx, dir)
	if err != nil {
		t.Fatalf("IsClean() error = %v", err)
	}
	if !clean {
		t.Fatal("repository is dirty after failed commit message")
	}
}
