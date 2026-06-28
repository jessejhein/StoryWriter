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
