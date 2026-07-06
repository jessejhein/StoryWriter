// BDD Scenario: 8.3.1 - Reject non-text diff output at branch boundary
// Requirements: M8-R09, M8-R20
// Test purpose: branch.GitRepository.UnifiedDiff rejects invalid UTF-8 and NUL
// bytes in diff output before returning to the analysis service.

package branch_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"storywork/internal/branch"
	"storywork/internal/gitstore"
)

func initRealRepoForBranch(t *testing.T) (context.Context, string, *branch.GitRepository, string, string) {
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
	mainHead, err := store.ResolveCommit(ctx, dir, "main")
	if err != nil {
		t.Fatal(err)
	}
	ref := "branch/test-exp-0123456789abcdef0123"
	if err := store.CreateAndSwitch(ctx, dir, ref, mainHead, mainHead); err != nil {
		t.Fatal(err)
	}
	repo := &branch.GitRepository{Store: store}
	return ctx, dir, repo, mainHead, ref
}

// Test: invalid UTF-8 in a changed file's diff is rejected at the branch
// adapter boundary.
func TestGitRepositoryUnifiedDiffRejectsInvalidUTF8(t *testing.T) {
	t.Parallel()
	ctx, dir, repo, mainHead, ref := initRealRepoForBranch(t)

	// Write a file containing invalid UTF-8 bytes.
	invalid := []byte("version: 2\nroot:\n  arcs: []\n\xff\xfe\n")
	if err := os.WriteFile(filepath.Join(dir, "outline.yaml"), invalid, 0o644); err != nil {
		t.Fatal(err)
	}
	store := gitstore.New("git")
	if err := store.CommitAll(ctx, dir, "bad utf8"); err != nil {
		t.Fatal(err)
	}
	experimentHead, err := store.ResolveCommit(ctx, dir, ref)
	if err != nil {
		t.Fatal(err)
	}

	_, err = repo.UnifiedDiff(ctx, dir, branch.CommitID(mainHead), branch.CommitID(experimentHead), []branch.ProjectPath{"outline.yaml"}, 64*1024)
	if !errors.Is(err, branch.ErrInvalidAnalysis) {
		t.Fatalf("UnifiedDiff() err = %v, want errors.Is branch.ErrInvalidAnalysis", err)
	}
}

// Test: a NUL byte in a changed file's diff is rejected at the branch
// adapter boundary.
func TestGitRepositoryUnifiedDiffRejectsNULByte(t *testing.T) {
	t.Parallel()
	ctx, dir, repo, mainHead, ref := initRealRepoForBranch(t)

	// Write a file containing a NUL byte.
	nulContent := []byte("version: 2\nroot:\n  arcs: []\n\x00\n")
	if err := os.WriteFile(filepath.Join(dir, "outline.yaml"), nulContent, 0o644); err != nil {
		t.Fatal(err)
	}
	store := gitstore.New("git")
	if err := store.CommitAll(ctx, dir, "nul byte"); err != nil {
		t.Fatal(err)
	}
	experimentHead, err := store.ResolveCommit(ctx, dir, ref)
	if err != nil {
		t.Fatal(err)
	}

	_, err = repo.UnifiedDiff(ctx, dir, branch.CommitID(mainHead), branch.CommitID(experimentHead), []branch.ProjectPath{"outline.yaml"}, 64*1024)
	if !errors.Is(err, branch.ErrInvalidAnalysis) {
		t.Fatalf("UnifiedDiff() err = %v, want errors.Is branch.ErrInvalidAnalysis", err)
	}
}
