// BDD Scenario: 8.1.2 - Reject unsafe branch state via inspectable sentinels
// Requirements: M8-R01, M8-R03, M8-R17
// Test purpose: dirty create/switch and stale delete errors expose exported sentinels.

package gitstore_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"storywork/internal/gitstore"
)

// Test: dirty worktree on create returns ErrDirtyWorktree inspectably.
// Requirements: M8-R03.
func TestCreateAndSwitchDirtyWorktreeErrorIsSentinel(t *testing.T) {
	t.Parallel()
	ctx, dir, store := initTestRepo(t)
	mainHead, err := store.ResolveCommit(ctx, dir, "main")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "outline.yaml"), []byte("version: 1\nroot:\n  arcs: []\ndirty: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err = store.CreateAndSwitch(ctx, dir, "branch/test-exp-0123456789abcdef0123", mainHead, mainHead)
	if err == nil {
		t.Fatal("CreateAndSwitch() = nil, want dirty error")
	}
	if !errors.Is(err, gitstore.ErrDirtyWorktree) {
		t.Fatalf("CreateAndSwitch() err = %v, want errors.Is gitstore.ErrDirtyWorktree", err)
	}
}

// Test: dirty worktree on switch returns ErrDirtyWorktree inspectably.
// Requirements: M8-R03.
func TestSwitchDirtyWorktreeErrorIsSentinel(t *testing.T) {
	t.Parallel()
	ctx, dir, store := initTestRepo(t)
	mainHead, err := store.ResolveCommit(ctx, dir, "main")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateAndSwitch(ctx, dir, "branch/test-exp-0123456789abcdef0123", mainHead, mainHead); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "outline.yaml"), []byte("version: 1\nroot:\n  arcs: []\ndirty: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err = store.Switch(ctx, dir, "main")
	if err == nil {
		t.Fatal("Switch() = nil, want dirty error")
	}
	if !errors.Is(err, gitstore.ErrDirtyWorktree) {
		t.Fatalf("Switch() err = %v, want errors.Is gitstore.ErrDirtyWorktree", err)
	}
}

// Test: stale head on delete returns ErrStaleExperimentHead inspectably.
// Requirements: M8-R17.
func TestDeleteExperimentStaleHeadErrorIsSentinel(t *testing.T) {
	t.Parallel()
	ctx, dir, store := initTestRepo(t)
	mainHead, err := store.ResolveCommit(ctx, dir, "main")
	if err != nil {
		t.Fatal(err)
	}
	ref := "branch/test-exp-0123456789abcdef0123"
	if err := store.CreateAndSwitch(ctx, dir, ref, mainHead, mainHead); err != nil {
		t.Fatal(err)
	}
	if err := store.Switch(ctx, dir, "main"); err != nil {
		t.Fatal(err)
	}
	err = store.DeleteExperiment(ctx, dir, ref, "ffffffffffffffffffffffffffffffffffffffff", mainHead)
	if err == nil {
		t.Fatal("DeleteExperiment() = nil, want stale error")
	}
	if !errors.Is(err, gitstore.ErrStaleExperimentHead) {
		t.Fatalf("DeleteExperiment() err = %v, want errors.Is gitstore.ErrStaleExperimentHead", err)
	}
}

// Test: sentinel errors are exported and distinct from generic errors.
func TestGitstoreSentinelErrorsAreExported(t *testing.T) {
	t.Parallel()
	if errors.Is(gitstore.ErrDirtyWorktree, gitstore.ErrStaleExperimentHead) {
		t.Fatal("ErrDirtyWorktree and ErrStaleExperimentHead must be distinct")
	}
	if gitstore.ErrDirtyWorktree == nil || gitstore.ErrStaleExperimentHead == nil {
		t.Fatal("sentinels must be non-nil")
	}
}

// Compile-time sentinel-presence guard.
var _ = gitstore.ErrDirtyWorktree
var _ = gitstore.ErrStaleExperimentHead
var _ error = gitstore.ErrDirtyWorktree
var _ error = gitstore.ErrStaleExperimentHead
