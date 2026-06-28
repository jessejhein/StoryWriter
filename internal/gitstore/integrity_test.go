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
//   - Requirement: Milestone 1, Story 1.4, preserve checkpoint integrity.
//   - Scenario: a clean project can be detected before mutation, and if a staged
//     mutation must be rolled back, the repository is left unstaged afterward.
//   - Test purpose: verify the real Git adapter reports clean versus dirty state
//     and can remove staged changes without touching the working copy.
func TestStoreDetectsDirtyWorktreesAndUnstagesFiles(t *testing.T) {
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

	clean, err := store.IsClean(ctx, dir)
	if err != nil {
		t.Fatalf("IsClean(clean repo) error = %v", err)
	}
	if !clean {
		t.Fatal("IsClean(clean repo) = false, want true")
	}

	if err := os.WriteFile(filepath.Join(dir, "project.yaml"), []byte("name: Revised Novel\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	clean, err = store.IsClean(ctx, dir)
	if err != nil {
		t.Fatalf("IsClean(dirty repo) error = %v", err)
	}
	if clean {
		t.Fatal("IsClean(dirty repo) = true, want false")
	}

	stage := exec.CommandContext(ctx, "git", "-C", dir, "add", "project.yaml")
	if output, err := stage.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v: %s", err, strings.TrimSpace(string(output)))
	}
	if err := store.UnstageAll(ctx, dir); err != nil {
		t.Fatalf("UnstageAll() error = %v", err)
	}

	diffCached := exec.CommandContext(ctx, "git", "-C", dir, "diff", "--cached", "--name-only")
	output, err := diffCached.Output()
	if err != nil {
		t.Fatalf("git diff --cached: %v", err)
	}
	if strings.TrimSpace(string(output)) != "" {
		t.Fatalf("cached diff = %q, want empty", string(output))
	}
}
