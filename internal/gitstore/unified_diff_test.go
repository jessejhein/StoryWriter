// BDD Scenario: 8.3.1 - Bounded real unified diff at Git boundary
// Requirements: M8-R09, M8-R20
// Test purpose: Git adapter produces real unified diff output with bounds.

package gitstore_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

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

// Test: the bounded reader kills the child process after maxBytes and returns
// ErrDiffTooLarge with no partial output. A marker file proves the process
// was stopped before writing the marker.
func TestUnifiedDiffStopsReadingAtByteLimit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script test is not supported on Windows")
	}
	t.Parallel()

	// Build a shell script that writes more than maxBytes to stdout, then
	// sleeps, and then creates a marker file. If the process is killed
	// after the bounded read, the marker will not exist.
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "fake-git.sh")
	markerPath := filepath.Join(tmpDir, "marker.txt")
	overflow := strings.Repeat("A", 200)
	script := "#!/bin/sh\nprintf '" + overflow + "'\nsleep 10\ntouch '" + markerPath + "'\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	store := gitstore.New(scriptPath)
	ctx := context.Background()

	// Use valid 40-char commit IDs and a valid project path so argument
	// validation passes. The script ignores all arguments.
	left := strings.Repeat("a", 40)
	right := strings.Repeat("b", 40)
	repoPath := t.TempDir()

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, err := store.UnifiedDiff(ctx, repoPath, left, right, []string{"outline.yaml"}, 100)
		if !errors.Is(err, gitstore.ErrDiffTooLarge) {
			t.Errorf("UnifiedDiff() err = %v, want gitstore.ErrDiffTooLarge", err)
		}
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("UnifiedDiff() did not return within 5 seconds; child process may not have been killed")
	}

	// The marker must not exist because the process was killed before
	// reaching the touch command.
	if _, err := os.Stat(markerPath); !os.IsNotExist(err) {
		t.Fatalf("marker file exists; child process was not killed after bounded read: %v", err)
	}
}

// Test: maxBytes <= 0 is rejected before starting any process.
func TestUnifiedDiffRejectsNonPositiveMaxBytes(t *testing.T) {
	t.Parallel()
	ctx, dir, store := initTestRepo(t)
	mainHead, err := store.ResolveCommit(ctx, dir, "main")
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.UnifiedDiff(ctx, dir, mainHead, mainHead, []string{"outline.yaml"}, 0)
	if !errors.Is(err, gitstore.ErrDiffTooLarge) {
		t.Fatalf("UnifiedDiff(maxBytes=0) err = %v, want gitstore.ErrDiffTooLarge", err)
	}
	_, err = store.UnifiedDiff(ctx, dir, mainHead, mainHead, []string{"outline.yaml"}, -1)
	if !errors.Is(err, gitstore.ErrDiffTooLarge) {
		t.Fatalf("UnifiedDiff(maxBytes=-1) err = %v, want gitstore.ErrDiffTooLarge", err)
	}
}
