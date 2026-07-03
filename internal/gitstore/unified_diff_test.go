// BDD Scenario: 8.3.1 - Bounded real unified diff at Git boundary
// Requirements: M8-R09, M8-R20
// Test purpose: Git adapter produces real unified diff output with bounds.

package gitstore_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"storywork/internal/gitstore"
)

// Test: modified file produces a real unified diff with context lines.
func TestUnifiedDiffModifiedFileHasContextLines(t *testing.T) {
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
	modified := "version: 2\nroot:\n  arcs: []\n"
	if err := os.WriteFile(filepath.Join(dir, "outline.yaml"), []byte(modified), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.CommitAll(ctx, dir, "experiment"); err != nil {
		t.Fatal(err)
	}
	experimentHead, err := store.ResolveCommit(ctx, dir, ref)
	if err != nil {
		t.Fatal(err)
	}
	diff, err := store.UnifiedDiff(ctx, dir, mainHead, experimentHead, []string{"outline.yaml"}, 64*1024)
	if err != nil {
		t.Fatalf("UnifiedDiff() error = %v", err)
	}
	if !strings.Contains(diff, " root:\n") {
		t.Fatalf("diff missing context line ' root:':\n%s", diff)
	}
	if !strings.Contains(diff, "-version: 1\n") {
		t.Fatalf("diff missing removed line '-version: 1':\n%s", diff)
	}
	if !strings.Contains(diff, "+version: 2\n") {
		t.Fatalf("diff missing added line '+version: 2':\n%s", diff)
	}
	if !strings.Contains(diff, "  arcs: []\n") {
		t.Fatalf("diff missing context line '  arcs: []':\n%s", diff)
	}
}

// Test: added file diff shows all lines as additions.
func TestUnifiedDiffAddedFile(t *testing.T) {
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
	os.MkdirAll(filepath.Join(dir, "scenes"), 0o755)
	if err := os.WriteFile(filepath.Join(dir, "scenes/scn_001.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.CommitAll(ctx, dir, "add scene"); err != nil {
		t.Fatal(err)
	}
	experimentHead, err := store.ResolveCommit(ctx, dir, ref)
	if err != nil {
		t.Fatal(err)
	}
	diff, err := store.UnifiedDiff(ctx, dir, mainHead, experimentHead, []string{"scenes/scn_001.md"}, 64*1024)
	if err != nil {
		t.Fatalf("UnifiedDiff() error = %v", err)
	}
	if !strings.Contains(diff, "+hello\n") {
		t.Fatalf("diff missing '+hello':\n%s", diff)
	}
	if strings.Contains(diff, "-hello\n") {
		t.Fatalf("diff should not contain '-hello':\n%s", diff)
	}
}

// Test: deleted file diff shows all lines as removals.
func TestUnifiedDiffDeletedFile(t *testing.T) {
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
	if err := os.Remove(filepath.Join(dir, "outline.yaml")); err != nil {
		t.Fatal(err)
	}
	if err := store.CommitAll(ctx, dir, "delete outline"); err != nil {
		t.Fatal(err)
	}
	experimentHead, err := store.ResolveCommit(ctx, dir, ref)
	if err != nil {
		t.Fatal(err)
	}
	diff, err := store.UnifiedDiff(ctx, dir, mainHead, experimentHead, []string{"outline.yaml"}, 64*1024)
	if err != nil {
		t.Fatalf("UnifiedDiff() error = %v", err)
	}
	if !strings.Contains(diff, "-version: 1\n") {
		t.Fatalf("diff missing '-version: 1':\n%s", diff)
	}
}

// Test: multi-path diff is sorted by path.
func TestUnifiedDiffMultiPathSorted(t *testing.T) {
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
	os.MkdirAll(filepath.Join(dir, "scenes"), 0o755)
	if err := os.WriteFile(filepath.Join(dir, "scenes/z.md"), []byte("z\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "outline.yaml"), []byte("version: 2\nroot:\n  arcs: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.CommitAll(ctx, dir, "multi"); err != nil {
		t.Fatal(err)
	}
	experimentHead, err := store.ResolveCommit(ctx, dir, ref)
	if err != nil {
		t.Fatal(err)
	}
	diff, err := store.UnifiedDiff(ctx, dir, mainHead, experimentHead, []string{"scenes/z.md", "outline.yaml"}, 64*1024)
	if err != nil {
		t.Fatalf("UnifiedDiff() error = %v", err)
	}
	outlineIdx := strings.Index(diff, "outline.yaml")
	scenesIdx := strings.Index(diff, "scenes/z.md")
	if outlineIdx < 0 || scenesIdx < 0 {
		t.Fatalf("diff missing one path:\n%s", diff)
	}
	if scenesIdx < outlineIdx {
		t.Fatalf("diff not sorted: scenes before outline:\n%s", diff)
	}
}

// Test: diff exceeding maxBytes fails closed.
func TestUnifiedDiffExceedsMaxBytes(t *testing.T) {
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
	big := strings.Repeat("x\n", 200)
	if err := os.WriteFile(filepath.Join(dir, "outline.yaml"), []byte(big), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.CommitAll(ctx, dir, "big"); err != nil {
		t.Fatal(err)
	}
	experimentHead, err := store.ResolveCommit(ctx, dir, ref)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.UnifiedDiff(ctx, dir, mainHead, experimentHead, []string{"outline.yaml"}, 10)
	if err == nil {
		t.Fatal("UnifiedDiff() = nil, want error for exceeding maxBytes")
	}
	if !errors.Is(err, gitstore.ErrDiffTooLarge) {
		t.Fatalf("UnifiedDiff() err = %v, want gitstore.ErrDiffTooLarge", err)
	}
}
