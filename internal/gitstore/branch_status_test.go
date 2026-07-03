// BDD Scenario: 8.1.1 - Create from current canon
// Requirements: M8-R01, M8-R02
// Test purpose: Git adapter reports active branch, cleanliness, and main head.

package gitstore_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"storywork/internal/gitstore"
)

func initTestRepo(t *testing.T) (context.Context, string, *gitstore.Store) {
	t.Helper()
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
	return ctx, dir, store
}

// Test: malformed refs inside the reserved namespace fail closed instead of
// disappearing from the managed experiment list.
// Requirements: M8-R01, M8-R06.
func TestListExperimentsRejectsMalformedReservedRef(t *testing.T) {
	t.Parallel()
	ctx, dir, store := initTestRepo(t)
	if output, err := exec.Command("git", "-C", dir, "branch", "branch/not-managed", "main").CombinedOutput(); err != nil {
		t.Fatalf("git branch: %v: %s", err, output)
	}
	if _, err := store.ListExperiments(ctx, dir); err == nil {
		t.Fatal("ListExperiments() error = nil")
	}
}

// Test: active main, managed experiment, and cleanliness detection.
// Requirements: M8-R01.
func TestStatusReportsActiveBranchAndCleanliness(t *testing.T) {
	t.Parallel()
	ctx, dir, store := initTestRepo(t)
	status, err := store.Status(ctx, dir)
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status.ActiveBranch != "main" || !status.IsClean || status.IsDetached {
		t.Fatalf("status = %#v", status)
	}
	if err := store.CreateAndSwitch(ctx, dir, "branch/test-exp-0123456789abcdef0123", status.MainHead); err != nil {
		t.Fatalf("CreateAndSwitch() error = %v", err)
	}
	status, err = store.Status(ctx, dir)
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status.ActiveBranch != "branch/test-exp-0123456789abcdef0123" {
		t.Fatalf("active = %q", status.ActiveBranch)
	}
}
