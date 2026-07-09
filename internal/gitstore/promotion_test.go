// BDD Scenario: 8.4.1 - Promote selected files to main
// Requirements: M8-R12, M8-R13, M8-R14
// Test purpose: Selected-path application, staging, and promotion commit work.

package gitstore_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"storywork/internal/gitstore"
)

// Test: apply selected experiment blob onto main and commit once.
// Requirements: M8-R14.
func TestApplyPathsAndCommitPromotion(t *testing.T) {
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
	changed := "version: 2\nroot:\n  arcs: []\n"
	if err := os.WriteFile(filepath.Join(dir, "outline.yaml"), []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.CommitAll(ctx, dir, "experiment"); err != nil {
		t.Fatal(err)
	}
	experimentHead, err := store.ResolveCommit(ctx, dir, ref)
	if err != nil {
		t.Fatal(err)
	}
	changes, err := store.CompareTrees(ctx, dir, mainHead, experimentHead)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Switch(ctx, dir, "main"); err != nil {
		t.Fatal(err)
	}
	if err := store.ApplyPaths(ctx, dir, experimentHead, changes, []string{"outline.yaml"}); err != nil {
		t.Fatalf("ApplyPaths() error = %v", err)
	}
	staged, err := exec.Command("git", "-C", dir, "diff", "--cached", "--name-only").Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(staged)) != "" {
		t.Fatalf("ApplyPaths staged files before StagePaths: %q", staged)
	}
	if err := store.StagePaths(ctx, dir, []string{"outline.yaml"}); err != nil {
		t.Fatal(err)
	}
	newHead, err := store.CommitPromotion(ctx, dir, gitstore.PromotionCommitInput{
		ExperimentID:     "brn_0123456789abcdef0123",
		SourceCommit:     experimentHead,
		BaseCommit:       mainHead,
		ExpectedMainHead: mainHead,
		Paths:            []string{"outline.yaml"},
	})
	if err != nil {
		t.Fatalf("CommitPromotion() error = %v", err)
	}
	if newHead == mainHead {
		t.Fatalf("newHead = mainHead = %q", mainHead)
	}
}

// Test: whole-file application handles one addition and one deletion without
// staging until the explicit StagePaths step.
// Requirements: M8-R13, M8-R15.
func TestApplyPathsHandlesAdditionsAndDeletions(t *testing.T) {
	t.Parallel()
	ctx, dir, store := initTestRepo(t)
	if err := os.MkdirAll(filepath.Join(dir, "scenes"), 0o755); err != nil {
		t.Fatal(err)
	}
	deleted := "scenes/scn_00000000000000000001.md"
	added := "scenes/scn_00000000000000000002.md"
	if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(deleted)), []byte("deleted from experiment\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.CommitAll(ctx, dir, "add main scene"); err != nil {
		t.Fatal(err)
	}
	mainHead, err := store.ResolveCommit(ctx, dir, "main")
	if err != nil {
		t.Fatal(err)
	}
	ref := "branch/add-delete-0123456789abcdef0123"
	if err := store.CreateAndSwitch(ctx, dir, ref, mainHead, mainHead); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, filepath.FromSlash(deleted))); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(added)), []byte("added on experiment\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.CommitAll(ctx, dir, "add and delete"); err != nil {
		t.Fatal(err)
	}
	experimentHead, err := store.ResolveCommit(ctx, dir, ref)
	if err != nil {
		t.Fatal(err)
	}
	changes, err := store.CompareTrees(ctx, dir, mainHead, experimentHead)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Switch(ctx, dir, "main"); err != nil {
		t.Fatal(err)
	}
	if err := store.ApplyPaths(ctx, dir, experimentHead, changes, []string{added, deleted}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(deleted))); !os.IsNotExist(err) {
		t.Fatalf("deleted path stat error = %v", err)
	}
	if body, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(added))); err != nil || string(body) != "added on experiment\n" {
		t.Fatalf("added body=%q err=%v", body, err)
	}
	staged, err := exec.Command("git", "-C", dir, "diff", "--cached", "--name-only").Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(staged)) != "" {
		t.Fatalf("staged=%q", staged)
	}
}

// Test: promotion publication rejects unrelated staged index content and keeps
// main at the expected head.
// Requirements: M8-R13, M8-R15.
func TestCommitPromotionRejectsUnexpectedStagedPaths(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(dir, "outline.yaml"), []byte("version: 2\nroot:\n  arcs: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.CommitAll(ctx, dir, "experiment"); err != nil {
		t.Fatal(err)
	}
	experimentHead, err := store.ResolveCommit(ctx, dir, ref)
	if err != nil {
		t.Fatal(err)
	}
	changes, err := store.CompareTrees(ctx, dir, mainHead, experimentHead)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Switch(ctx, dir, "main"); err != nil {
		t.Fatal(err)
	}
	if err := store.ApplyPaths(ctx, dir, experimentHead, changes, []string{"outline.yaml"}); err != nil {
		t.Fatal(err)
	}
	if err := store.StagePaths(ctx, dir, []string{"outline.yaml"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rogue.txt"), []byte("unexpected\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("git", "-C", dir, "add", "rogue.txt").CombinedOutput(); err != nil {
		t.Fatalf("git add rogue.txt: %v: %s", err, output)
	}
	_, err = store.CommitPromotion(ctx, dir, gitstore.PromotionCommitInput{
		ExperimentID:     "brn_0123456789abcdef0123",
		SourceCommit:     experimentHead,
		BaseCommit:       mainHead,
		ExpectedMainHead: mainHead,
		Paths:            []string{"outline.yaml"},
	})
	if err == nil {
		t.Fatal("CommitPromotion() error = nil")
	}
	currentMain, err := store.ResolveCommit(ctx, dir, "main")
	if err != nil {
		t.Fatal(err)
	}
	if currentMain != mainHead {
		t.Fatalf("main head changed: %q -> %q", mainHead, currentMain)
	}
}

// Test: a publication verification failure rolls main back to the expected
// head instead of leaving the promotion commit published.
// Requirements: M8-R14, M8-R15.
func TestCommitPromotionRevertsPublishedMainOnVerificationFailure(t *testing.T) {
	t.Parallel()
	ctx, dir, _ := initTestRepo(t)
	mainHead, err := gitstore.New("git").ResolveCommit(ctx, dir, "main")
	if err != nil {
		t.Fatal(err)
	}
	ref := "branch/test-exp-0123456789abcdef0123"
	baseStore := gitstore.New("git")
	if err := baseStore.CreateAndSwitch(ctx, dir, ref, mainHead, mainHead); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "outline.yaml"), []byte("version: 2\nroot:\n  arcs: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := baseStore.CommitAll(ctx, dir, "experiment"); err != nil {
		t.Fatal(err)
	}
	experimentHead, err := baseStore.ResolveCommit(ctx, dir, ref)
	if err != nil {
		t.Fatal(err)
	}
	changes, err := baseStore.CompareTrees(ctx, dir, mainHead, experimentHead)
	if err != nil {
		t.Fatal(err)
	}
	if err := baseStore.Switch(ctx, dir, "main"); err != nil {
		t.Fatal(err)
	}
	if err := baseStore.ApplyPaths(ctx, dir, experimentHead, changes, []string{"outline.yaml"}); err != nil {
		t.Fatal(err)
	}
	if err := baseStore.StagePaths(ctx, dir, []string{"outline.yaml"}); err != nil {
		t.Fatal(err)
	}

	wrapper := filepath.Join(t.TempDir(), "git-wrapper.sh")
	script := "#!/bin/sh\n" +
		"set -eu\n" +
		"repo=\n" +
		"if [ \"${1:-}\" = \"-C\" ]; then repo=$2; fi\n" +
		"if [ \"${1:-}\" = \"-C\" ] && [ \"${3:-}\" = \"update-ref\" ] && [ \"${4:-}\" = \"refs/heads/main\" ] && [ -n \"$repo\" ]; then\n" +
		"  /usr/bin/env git \"$@\"\n" +
		"  printf '\\n' >> \"$repo/outline.yaml\"\n" +
		"  exit 0\n" +
		"fi\n" +
		"exec /usr/bin/env git \"$@\"\n"
	if err := os.WriteFile(wrapper, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	store := gitstore.New(wrapper)
	_, err = store.CommitPromotion(ctx, dir, gitstore.PromotionCommitInput{
		ExperimentID:     "brn_0123456789abcdef0123",
		SourceCommit:     experimentHead,
		BaseCommit:       mainHead,
		ExpectedMainHead: mainHead,
		Paths:            []string{"outline.yaml"},
	})
	if err == nil {
		t.Fatal("CommitPromotion() error = nil")
	}
	if !strings.Contains(err.Error(), "verify promotion publication") {
		t.Fatalf("CommitPromotion() error = %v", err)
	}
	currentMain, err := baseStore.ResolveCommit(ctx, dir, "main")
	if err != nil {
		t.Fatal(err)
	}
	if currentMain != mainHead {
		t.Fatalf("main head changed: %q -> %q", mainHead, currentMain)
	}
}
