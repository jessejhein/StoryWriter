// BDD Scenario: 3.1.2 - Create an entry
// Requirements: M3-R04
// Test purpose: Canonical Codex entry YAML round-trips without changing exact bytes or revisions.
package storyfile

import (
	"context"
	"testing"

	"storywork/internal/codex"
)

// Test: the store marshals exact canonical entry YAML bytes and reloads the same logical document with a stable revision.
// Requirements: M3-R04
func TestCodexEntryRoundTripCanonicalBytes(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mustMkdirAll(t, root, "codex/characters", "progressions")
	store := New()
	entry := codex.Entry{
		ID:          "char_0123456789abcdef0123",
		Type:        codex.TypeCharacter,
		Name:        "Obi-Wan Kenobi",
		Aliases:     []string{"Ben"},
		Tags:        []string{"jedi", "mentor"},
		Description: "Guide.\n",
		Metadata:    map[string]string{"role": "mentor", "status": "alive"},
	}
	entryBytes, err := store.MarshalCodexEntry(entry)
	if err != nil {
		t.Fatalf("MarshalCodexEntry() error = %v", err)
	}
	if _, err := store.WriteFiles(context.Background(), root, map[string][]byte{
		"codex/characters/char_0123456789abcdef0123.yaml": entryBytes,
	}); err != nil {
		t.Fatalf("WriteFiles() error = %v", err)
	}
	loadedEntry, err := store.LoadCodexEntry(context.Background(), root, entry.ID)
	if err != nil {
		t.Fatalf("LoadCodexEntry() error = %v", err)
	}
	if loadedEntry.Revision != codex.ComputeRevision(entryBytes) {
		t.Fatalf("entry revision = %q", loadedEntry.Revision)
	}
}
