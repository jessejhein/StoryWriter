// BDD Scenario: 8.1.2 - Reject unsafe branch state
// Requirements: M8-R01, M8-R03, M8-R04
// Test purpose: Create and switch primitives enforce clean worktrees and validated refs.

package gitstore_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"storywork/internal/gitstore"
)

// Test: create at main while another branch is active and switch back.
// Requirements: M8-R02.
func TestCreateAndSwitchFromMain(t *testing.T) {
	t.Parallel()
	ctx, dir, store := initTestRepo(t)
	mainHead, err := store.ResolveCommit(ctx, dir, "main")
	if err != nil {
		t.Fatal(err)
	}
	ref := "branch/test-exp-0123456789abcdef0123"
	if err := store.CreateAndSwitch(ctx, dir, ref, mainHead); err != nil {
		t.Fatalf("CreateAndSwitch() error = %v", err)
	}
	head, err := store.ResolveCommit(ctx, dir, ref)
	if err != nil || head != mainHead {
		t.Fatalf("experiment head = %q main = %q err=%v", head, mainHead, err)
	}
	if err := store.Switch(ctx, dir, "main"); err != nil {
		t.Fatalf("Switch(main) error = %v", err)
	}
}

// Test: a checkout failure restores the prior branch and removes the newly
// created experiment ref.
// Requirements: M8-R02, M8-R04.
func TestCreateAndSwitchCheckoutFailureLeavesNoExperiment(t *testing.T) {
	t.Parallel()
	ctx, dir, store := initTestRepo(t)
	hook := filepath.Join(dir, ".git", "hooks", "post-checkout")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	mainHead, err := store.ResolveCommit(ctx, dir, "main")
	if err != nil {
		t.Fatal(err)
	}
	err = store.CreateAndSwitch(ctx, dir, "branch/failing-0123456789abcdef0123", mainHead)
	if err == nil {
		t.Fatal("CreateAndSwitch() error = nil")
	}
	active, commandErr := exec.Command("git", "-C", dir, "symbolic-ref", "--short", "HEAD").Output()
	if commandErr != nil {
		t.Fatal(commandErr)
	}
	if strings.TrimSpace(string(active)) != "main" {
		t.Fatalf("active branch = %q", active)
	}
	refs, commandErr := exec.Command("git", "-C", dir, "for-each-ref", "--format=%(refname:short)", "refs/heads/branch/").Output()
	if commandErr != nil {
		t.Fatal(commandErr)
	}
	if strings.TrimSpace(string(refs)) != "" {
		t.Fatalf("experiment refs = %q", refs)
	}
}

// Test: dirty worktree refuses create and switch.
// Requirements: M8-R03.
func TestCreateAndSwitchRejectsDirtyWorktree(t *testing.T) {
	t.Parallel()
	ctx, dir, store := initTestRepo(t)
	mainHead, err := store.ResolveCommit(ctx, dir, "main")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "outline.yaml"), []byte("version: 1\nroot:\n  arcs: []\nchanged: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateAndSwitch(ctx, dir, "branch/test-exp-0123456789abcdef0123", mainHead); err == nil {
		t.Fatal("CreateAndSwitch() on dirty worktree = nil, want error")
	}
}

// Test: switching to an experiment with a stale expected head fails before
// changing the active branch.
// Requirements: M8-R03, M8-R17.
func TestSwitchExperimentRejectsStaleExpectedHead(t *testing.T) {
	t.Parallel()
	ctx, dir, store := initTestRepo(t)
	mainHead, err := store.ResolveCommit(ctx, dir, "main")
	if err != nil {
		t.Fatal(err)
	}
	ref := "branch/test-exp-0123456789abcdef0123"
	if err := store.CreateAndSwitch(ctx, dir, ref, mainHead); err != nil {
		t.Fatal(err)
	}
	experimentHead, err := store.ResolveCommit(ctx, dir, ref)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Switch(ctx, dir, "main"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "outline.yaml"), []byte("version: 9\nroot:\n  arcs: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.CommitAll(ctx, dir, "advance main"); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("git", "-C", dir, "branch", "-f", ref, "HEAD").CombinedOutput(); err != nil {
		t.Fatalf("git branch -f: %v: %s", err, output)
	}
	err = store.SwitchExperiment(ctx, dir, ref, experimentHead)
	if !errors.Is(err, gitstore.ErrStaleExperimentHead) {
		t.Fatalf("SwitchExperiment() err = %v", err)
	}
	active, commandErr := exec.Command("git", "-C", dir, "symbolic-ref", "--short", "HEAD").Output()
	if commandErr != nil {
		t.Fatal(commandErr)
	}
	if strings.TrimSpace(string(active)) != "main" {
		t.Fatalf("active branch = %q, want main", active)
	}
}
