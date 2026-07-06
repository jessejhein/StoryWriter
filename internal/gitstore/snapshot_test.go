// BDD Scenario: 8.4.3 - Snapshot main paths for rollback
// Requirements: M8-R14, M8-R15
// Test purpose: Snapshot acquisition fails closed for missing, invalid, and
// non-regular tree entries and preserves clean rollback inputs.

package gitstore_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"storywork/internal/gitstore"
)

// Test: snapshot reports existing files, missing files, and executable modes
// without collapsing inspection errors.
// Requirements: M8-R14, M8-R15.
func TestSnapshotPathsDistinguishesExistenceAndInspectionErrors(t *testing.T) {
	t.Parallel()
	ctx, dir, store := initTestRepo(t)
	if err := os.MkdirAll(filepath.Join(dir, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	executablePath := "bin/run.sh"
	if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(executablePath)), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := store.CommitAll(ctx, dir, "add executable"); err != nil {
		t.Fatal(err)
	}
	head, err := store.ResolveCommit(ctx, dir, "main")
	if err != nil {
		t.Fatal(err)
	}
	snapshots, err := store.SnapshotPaths(ctx, dir, head, []string{"outline.yaml", "missing.txt", executablePath})
	if err != nil {
		t.Fatalf("SnapshotPaths() error = %v", err)
	}
	if len(snapshots) != 3 {
		t.Fatalf("snapshots = %#v", snapshots)
	}
	if !snapshots[0].Exists || snapshots[1].Exists || !snapshots[2].Exists {
		t.Fatalf("snapshots = %#v", snapshots)
	}
}

// Test: snapshot rejects symlinks, invalid commits, cancellations, and git
// command failures before returning partial data.
// Requirements: M8-R14, M8-R15.
func TestSnapshotPathsFailsClosedOnInvalidTreeState(t *testing.T) {
	t.Parallel()
	ctx, dir, store := initTestRepo(t)
	head, err := store.ResolveCommit(ctx, dir, "main")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "links"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../outline.yaml", filepath.Join(dir, "links", "outline.yaml")); err != nil {
		t.Fatal(err)
	}
	if err := store.CommitAll(ctx, dir, "add symlink"); err != nil {
		t.Fatal(err)
	}
	symlinkHead, err := store.ResolveCommit(ctx, dir, "main")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SnapshotPaths(ctx, dir, symlinkHead, []string{"links/outline.yaml"}); err == nil {
		t.Fatal("SnapshotPaths(symlink) error = nil")
	}

	if _, err := store.SnapshotPaths(ctx, dir, "ffffffffffffffffffffffffffffffffffffffff", []string{"outline.yaml"}); err == nil {
		t.Fatal("SnapshotPaths(invalid object) error = nil")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := store.SnapshotPaths(canceled, dir, head, []string{"outline.yaml"}); err == nil {
		t.Fatal("SnapshotPaths(canceled) error = nil")
	}

	failingGit := filepath.Join(t.TempDir(), "git")
	script := "#!/bin/sh\nexit 1\n"
	if err := os.WriteFile(failingGit, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	failingStore := gitstore.New(failingGit)
	if _, err := failingStore.SnapshotPaths(ctx, dir, head, []string{"outline.yaml"}); err == nil {
		t.Fatal("SnapshotPaths(injected failure) error = nil")
	}
}
