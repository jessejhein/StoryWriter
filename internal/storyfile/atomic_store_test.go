package storyfile

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// BDD trace:
//   - Requirement: Milestone 1, Story 1.4, preserve checkpoint integrity.
//   - Scenario: when canonical file replacement fails during a structural
//     mutation, the previous file contents remain intact instead of being partly
//     replaced.
//   - Test purpose: verify atomic write failure preserves the old canonical file.
func TestWriteFilesLeavesOldContentsIntactWhenReplacementFails(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	root := t.TempDir()
	store := New()
	mustMkdirAll(t, root)

	path := filepath.Join(root, "outline.yaml")
	original := []byte("version: 1\nroot:\n  arcs: []\n")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatalf("WriteFile(outline.yaml) error = %v", err)
	}

	store.rename = func(_, _ string) error {
		return errors.New("rename failed")
	}

	_, err := store.WriteFiles(ctx, root, map[string][]byte{
		"outline.yaml": []byte("version: 1\nroot:\n  arcs:\n    - id: arc_00000000000000000001\n      chapters: []\n"),
	})
	if err == nil {
		t.Fatal("WriteFiles() error = nil, want failure")
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(outline.yaml) error = %v", err)
	}
	if string(contents) != string(original) {
		t.Fatalf("outline.yaml = %q, want original %q", string(contents), string(original))
	}
}

// BDD trace:
//   - Requirement: Milestone 1, Story 1.4, preserve checkpoint integrity.
//   - Scenario: when checkpoint creation fails after canonical writes, replaced
//     files are restored and newly created entity files are removed.
//   - Test purpose: verify the snapshot rollback returned by a successful atomic
//     write restores every affected canonical path.
func TestWriteFilesRollbackRestoresReplacedAndRemovesCreatedFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := New()
	original := []byte("version: 1\nroot:\n  arcs: []\n")
	if err := os.WriteFile(filepath.Join(root, "outline.yaml"), original, 0o644); err != nil {
		t.Fatalf("WriteFile(outline.yaml) error = %v", err)
	}

	rollback, err := store.WriteFiles(context.Background(), root, map[string][]byte{
		"outline.yaml":                       []byte("changed\n"),
		"arcs/arc_00000000000000000001.yaml": []byte("new arc\n"),
	})
	if err != nil {
		t.Fatalf("WriteFiles() error = %v", err)
	}
	if err := rollback(); err != nil {
		t.Fatalf("rollback() error = %v", err)
	}

	contents, err := os.ReadFile(filepath.Join(root, "outline.yaml"))
	if err != nil {
		t.Fatalf("ReadFile(outline.yaml) error = %v", err)
	}
	if string(contents) != string(original) {
		t.Fatalf("restored outline = %q, want %q", contents, original)
	}
	if _, err := os.Stat(filepath.Join(root, "arcs", "arc_00000000000000000001.yaml")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("new arc Stat() error = %v, want not exist", err)
	}
}
