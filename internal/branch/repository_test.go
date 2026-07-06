// BDD Scenario: 8.1.1 - Create from current canon
// Requirements: M8-R01, M8-R06
// Test purpose: The branch repository adapter fails closed on malformed
// managed refs instead of skipping them.

package branch_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"storywork/internal/branch"
	"storywork/internal/gitstore"
)

// Test: a malformed reserved ref is returned as a repository-state error
// rather than disappearing from the adapter output.
// Requirements: M8-R01, M8-R06.
func TestGitRepositoryListExperimentsReturnsMalformedRefError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "outline.yaml"), []byte("version: 1\nroot:\n  arcs: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := gitstore.New("git")
	if err := store.Init(ctx, dir); err != nil {
		t.Fatal(err)
	}
	if err := store.CommitAll(ctx, dir, "init"); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.CommandContext(ctx, "git", "-C", dir, "branch", "branch/main-0123456789abcdef0123", "main").CombinedOutput(); err != nil {
		t.Fatalf("git branch: %v: %s", err, output)
	}
	repo := &branch.GitRepository{Store: store}
	if _, err := repo.ListExperiments(ctx, dir); !errors.Is(err, branch.ErrRepositoryState) {
		t.Fatalf("ListExperiments() err = %v, want ErrRepositoryState", err)
	}
}
