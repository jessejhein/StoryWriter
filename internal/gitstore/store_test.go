package gitstore_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"storywork/internal/gitstore"
)

// BDD trace:
//   - Requirement: Milestone 0, Story 0.1, create project folder.
//   - Scenario: given an empty directory path, when I create a project, then the
//     app initializes a Git repository and records a first commit.
//   - Test purpose: verify the Git adapter can initialize a repository, commit
//     staged project files, and report the repository as valid.
func TestStoreInitializesAndCommitsRepository(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "project.yaml"), []byte("name: Test Novel\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := gitstore.New("git")
	if err := store.Init(ctx, dir); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	if err := store.CommitAll(ctx, dir, "Initialize story project"); err != nil {
		t.Fatalf("CommitAll() error = %v", err)
	}

	isRepo, err := store.IsRepo(ctx, dir)
	if err != nil {
		t.Fatalf("IsRepo() error = %v", err)
	}
	if !isRepo {
		t.Fatal("IsRepo() = false, want true")
	}

	command := exec.CommandContext(ctx, "git", "-C", dir, "log", "-1", "--pretty=%s")
	output, err := command.Output()
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	if got := strings.TrimSpace(string(output)); got != "Initialize story project" {
		t.Fatalf("commit subject = %q, want %q", got, "Initialize story project")
	}
}
