// BDD Scenario: 8.2.1 - List exact changed files
// Requirements: M8-R05, M8-R06
// Test purpose: Tree comparison uses endpoint diff with rename detection disabled.

package gitstore_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// Test: add, modify, and delete paths are reported.
// Requirements: M8-R05.
func TestCompareTreesReportsAddedModifiedDeleted(t *testing.T) {
	t.Parallel()
	ctx, dir, store := initTestRepo(t)
	if err := os.MkdirAll(filepath.Join(dir, "chapters"), 0o755); err != nil {
		t.Fatal(err)
	}
	deletedPath := "chapters/ch_00000000000000000001.yaml"
	if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(deletedPath)), []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.CommitAll(ctx, dir, "add deletion fixture"); err != nil {
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
	scenePath := filepath.Join(dir, "scenes")
	if err := os.MkdirAll(scenePath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scenePath, "scn_00000000000000000001.md"), []byte("---\nid: scn_00000000000000000001\n---\n\nnew\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "outline.yaml"), []byte("version: 1\nroot:\n  arcs: []\n# modified\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, filepath.FromSlash(deletedPath))); err != nil {
		t.Fatal(err)
	}
	if err := store.CommitAll(ctx, dir, "experiment edit"); err != nil {
		t.Fatal(err)
	}
	experimentHead, err := store.ResolveCommit(ctx, dir, ref)
	if err != nil {
		t.Fatal(err)
	}
	changes, err := store.CompareTrees(ctx, dir, mainHead, experimentHead)
	if err != nil {
		t.Fatalf("CompareTrees() error = %v", err)
	}
	got := map[string]byte{}
	for _, change := range changes {
		got[change.Path] = change.Status
	}
	want := map[string]byte{"outline.yaml": 'M', "scenes/scn_00000000000000000001.md": 'A', deletedPath: 'D'}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("changes=%v want=%v", got, want)
	}
}

// Test: comparison rejects symlink entries instead of reporting them as
// promotable text files.
// Requirements: M8-R06, M8-R07.
func TestCompareTreesRejectsSymlinkChanges(t *testing.T) {
	t.Parallel()
	ctx, dir, store := initTestRepo(t)
	mainHead, err := store.ResolveCommit(ctx, dir, "main")
	if err != nil {
		t.Fatal(err)
	}
	ref := "branch/symlink-0123456789abcdef0123"
	if err := store.CreateAndSwitch(ctx, dir, ref, mainHead, mainHead); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "scenes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("target", filepath.Join(dir, "scenes", "scn_00000000000000000001.md")); err != nil {
		t.Fatal(err)
	}
	if err := store.CommitAll(ctx, dir, "symlink"); err != nil {
		t.Fatal(err)
	}
	experimentHead, err := store.ResolveCommit(ctx, dir, ref)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CompareTrees(ctx, dir, mainHead, experimentHead); err == nil {
		t.Fatal("CompareTrees() error = nil")
	}
}
