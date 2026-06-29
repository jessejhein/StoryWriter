// BDD Scenario: 3.2.3 - Report malformed canonical progressions
// Requirements: M3-R05, M3-R18
// Test purpose: Plain-English description of strict progression-document validation for route and filename consistency during canonical reads.
package storyfile

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadProgressionsRejectsEntryIDMismatch(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mustMkdirAll(t, root, "progressions")
	path := filepath.Join(root, "progressions", "char_0123456789abcdef0123.yaml")
	if err := os.WriteFile(path, []byte("version: 1\nentry_id: \"char_99999999999999999999\"\nprogressions: []\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// Test: a progression file whose entry_id disagrees with its filename is rejected as malformed canonical state.
	// Requirements: M3-R18
	if _, err := New().LoadProgressions(context.Background(), root, "char_0123456789abcdef0123"); err == nil {
		t.Fatal("LoadProgressions() error = nil")
	}
}
