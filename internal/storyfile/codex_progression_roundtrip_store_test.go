// BDD Scenario: 3.2.1 - Save ordered progressions
// Requirements: M3-R05
// Test purpose: Canonical progression YAML round-trips without changing exact bytes or revisions.
package storyfile

import (
	"context"
	"testing"

	"storywork/internal/codex"
)

// Test: the store marshals exact canonical progression YAML bytes and reloads the same logical document with a stable revision.
// Requirements: M3-R05
func TestProgressionRoundTripCanonicalBytes(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mustMkdirAll(t, root, "progressions")
	store := New()
	description := "Gone.\n"
	document := codex.ProgressionDocument{
		EntryID: "char_0123456789abcdef0123",
		Progressions: []codex.Progression{{
			ID:      "prog_0123456789abcdef0123",
			Anchor:  codex.ProgressionAnchor{Type: "scene", ID: "scn_0123456789abcdef0123", Timing: "after"},
			Changes: codex.ProgressionChange{Description: &description, Metadata: map[string]string{"status": "deceased"}},
		}},
	}

	progressionBytes, err := store.MarshalProgressions(document)
	if err != nil {
		t.Fatalf("MarshalProgressions() error = %v", err)
	}
	if _, err := store.WriteFiles(context.Background(), root, map[string][]byte{
		"progressions/char_0123456789abcdef0123.yaml": progressionBytes,
	}); err != nil {
		t.Fatalf("WriteFiles() error = %v", err)
	}

	loadedDocument, err := store.LoadProgressions(context.Background(), root, document.EntryID)
	if err != nil {
		t.Fatalf("LoadProgressions() error = %v", err)
	}
	if loadedDocument.Revision == nil || *loadedDocument.Revision != codex.ComputeRevision(progressionBytes) {
		t.Fatalf("progression revision = %#v", loadedDocument.Revision)
	}
}
