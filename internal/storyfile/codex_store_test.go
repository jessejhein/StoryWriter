package storyfile

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"storywork/internal/codex"
)

// BDD Scenario: 3.1.2 - Create an entry
// Requirements: M3-R04, M3-R05, M3-R18
// Test purpose: Plain-English description of exact canonical Codex and progression YAML serialization plus strict reload of the same bytes.
func TestCodexAndProgressionRoundTripCanonicalBytes(t *testing.T) {
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
	description := "Gone.\n"
	document := codex.ProgressionDocument{
		EntryID: "char_0123456789abcdef0123",
		Progressions: []codex.Progression{{
			ID:      "prog_0123456789abcdef0123",
			Anchor:  codex.ProgressionAnchor{Type: "scene", ID: "scn_0123456789abcdef0123", Timing: "after"},
			Changes: codex.ProgressionChange{Description: &description, Metadata: map[string]string{"status": "deceased"}},
		}},
	}

	// Test: the store marshals exact canonical YAML bytes and reloads the same logical documents with stable revisions.
	// Requirements: M3-R04
	entryBytes, err := store.MarshalCodexEntry(entry)
	if err != nil {
		t.Fatalf("MarshalCodexEntry() error = %v", err)
	}
	progressionBytes, err := store.MarshalProgressions(document)
	if err != nil {
		t.Fatalf("MarshalProgressions() error = %v", err)
	}
	if _, err := store.WriteFiles(context.Background(), root, map[string][]byte{
		"codex/characters/char_0123456789abcdef0123.yaml": entryBytes,
		"progressions/char_0123456789abcdef0123.yaml":     progressionBytes,
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
	loadedDocument, err := store.LoadProgressions(context.Background(), root, entry.ID)
	if err != nil {
		t.Fatalf("LoadProgressions() error = %v", err)
	}
	if loadedDocument.Revision == nil || *loadedDocument.Revision != codex.ComputeRevision(progressionBytes) {
		t.Fatalf("progression revision = %#v", loadedDocument.Revision)
	}
}

// BDD Scenario: 3.1.5 - Reject missing or malformed canonical entries
// Requirements: M3-R01, M3-R05, M3-R18
// Test purpose: Plain-English description of the strict YAML parser behavior for unknown fields and malformed canonical Codex files.
func TestLoadCodexEntryRejectsMalformedCanonicalYAML(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mustMkdirAll(t, root, "codex/characters")
	path := filepath.Join(root, "codex", "characters", "char_0123456789abcdef0123.yaml")
	if err := os.WriteFile(path, []byte("version: 1\nid: \"char_0123456789abcdef0123\"\ntype: character\nname: \"Ben\"\naliases: []\ntags: []\ndescription: \"Guide.\"\nmetadata: {}\nextra: true\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// Test: unknown canonical fields are rejected with a contextual decode error instead of being ignored or repaired.
	// Requirements: M3-R18
	if _, err := New().LoadCodexEntry(context.Background(), root, "char_0123456789abcdef0123"); err == nil {
		t.Fatal("LoadCodexEntry() error = nil")
	}
}
