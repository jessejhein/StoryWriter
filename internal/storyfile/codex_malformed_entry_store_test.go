// BDD Scenario: 3.1.5 - Reject missing or malformed canonical entries
// Requirements: M3-R01, M3-R09, M3-R18
// Test purpose: Strict Codex reads reject malformed files and keep host filesystem paths out of errors.
package storyfile

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadCodexEntryRejectsMalformedCanonicalYAML(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mustMkdirAll(t, root, "codex/characters")
	path := filepath.Join(root, "codex", "characters", "char_0123456789abcdef0123.yaml")
	if err := os.WriteFile(path, []byte("id: char_0123456789abcdef0123\ntype: character\nname: Ben\nunknown: value\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// Test: malformed canonical Codex YAML fails loudly instead of being repaired or skipped.
	// Requirements: M3-R18
	if _, err := New().LoadCodexEntry(context.Background(), root, "char_0123456789abcdef0123"); err == nil {
		t.Fatal("LoadCodexEntry() error = nil")
	}
}

func TestLoadCodexEntryMasksHostFilesystemPath(t *testing.T) {
	t.Parallel()

	store := New()
	store.readFile = func(string) ([]byte, error) {
		return nil, &os.PathError{Op: "open", Path: "/private/story/codex/characters/char_0123456789abcdef0123.yaml", Err: os.ErrPermission}
	}

	// Test: adapter errors retain the canonical relative path but do not expose the host project root.
	// Requirements: M3-R09, M3-R18
	_, err := store.LoadCodexEntry(context.Background(), "/private/story", "char_0123456789abcdef0123")
	if err == nil {
		t.Fatal("LoadCodexEntry() error = nil")
	}
	if strings.Contains(err.Error(), "/private/story") {
		t.Fatalf("LoadCodexEntry() leaked host path: %v", err)
	}
	if !strings.Contains(err.Error(), "codex/characters/char_0123456789abcdef0123.yaml") {
		t.Fatalf("LoadCodexEntry() error lacks canonical context: %v", err)
	}
}

// Test: YAML numbers, booleans, and timestamps are rejected where the entry schema requires strings.
// Requirements: M3-R01, M3-R18
func TestLoadCodexEntryRejectsNonStringScalars(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		contents string
	}{
		{
			name:     "numeric name",
			contents: "version: 1\nid: char_0123456789abcdef0123\ntype: character\nname: 123\naliases: []\ntags: []\ndescription: Guide.\nmetadata: {}\n",
		},
		{
			name:     "boolean alias",
			contents: "version: 1\nid: char_0123456789abcdef0123\ntype: character\nname: Ben\naliases: [true]\ntags: []\ndescription: Guide.\nmetadata: {}\n",
		},
		{
			name:     "numeric metadata value",
			contents: "version: 1\nid: char_0123456789abcdef0123\ntype: character\nname: Ben\naliases: []\ntags: []\ndescription: Guide.\nmetadata:\n  rank: 3\n",
		},
	}

	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			mustMkdirAll(t, root, "codex/characters")
			path := filepath.Join(root, "codex", "characters", "char_0123456789abcdef0123.yaml")
			if err := os.WriteFile(path, []byte(testCase.contents), 0o644); err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}
			if _, err := New().LoadCodexEntry(context.Background(), root, "char_0123456789abcdef0123"); err == nil {
				t.Fatal("LoadCodexEntry() error = nil, want non-string scalar rejection")
			}
		})
	}
}
