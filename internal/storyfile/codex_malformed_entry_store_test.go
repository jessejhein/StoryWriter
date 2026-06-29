// BDD Scenario: 3.1.5 - Reject missing or malformed canonical entries
// Requirements: M3-R01, M3-R05, M3-R18
// Test purpose: Plain-English description of the strict YAML parser behavior for unknown fields and malformed canonical Codex files.
package storyfile

import (
	"context"
	"os"
	"path/filepath"
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
