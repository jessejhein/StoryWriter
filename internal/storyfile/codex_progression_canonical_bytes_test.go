// BDD Scenario: 3.2.1 - Save ordered progressions
// Requirements: M3-R05
// Test purpose: Prove the canonical YAML byte shape for progression documents, including populated and empty ordered lists.
package storyfile

import (
	"strings"
	"testing"

	"storywork/internal/codex"
)

// Test: a populated progression document serializes to the exact canonical YAML bytes documented in the contract with unquoted safe scalars and two-space indentation.
// Requirements: M3-R05
func TestMarshalProgressionsProducesExactCanonicalBytes(t *testing.T) {
	t.Parallel()

	description := "No longer physically present, but still influential."
	document := codex.ProgressionDocument{
		EntryID: "char_0123456789abcdef0123",
		Progressions: []codex.Progression{{
			ID:      "prog_0123456789abcdef0123",
			Anchor:  codex.ProgressionAnchor{Type: "scene", ID: "scn_0123456789abcdef0123", Timing: "after"},
			Changes: codex.ProgressionChange{Description: &description, Metadata: map[string]string{"status": "deceased"}},
		}},
	}
	store := New()
	contents, err := store.MarshalProgressions(document)
	if err != nil {
		t.Fatalf("MarshalProgressions() error = %v", err)
	}

	want := strings.Join([]string{
		`version: 1`,
		`entry_id: char_0123456789abcdef0123`,
		`progressions:`,
		`  - id: prog_0123456789abcdef0123`,
		`    anchor:`,
		`      type: scene`,
		`      id: scn_0123456789abcdef0123`,
		`      timing: after`,
		`    changes:`,
		`      description: No longer physically present, but still influential.`,
		`      metadata:`,
		`        status: deceased`,
		``,
	}, "\n")
	if string(contents) != want {
		t.Fatalf("canonical progression bytes mismatch:\nwant:\n%s\ngot:\n%s", want, contents)
	}
}

// Test: an empty progression list serializes as progressions: [] with one terminal newline, not null or omitted.
// Requirements: M3-R05
func TestMarshalProgressionsEmptyDocumentUsesCanonicalEmptyShape(t *testing.T) {
	t.Parallel()

	document := codex.ProgressionDocument{
		EntryID:      "char_0123456789abcdef0123",
		Progressions: []codex.Progression{},
	}
	store := New()
	contents, err := store.MarshalProgressions(document)
	if err != nil {
		t.Fatalf("MarshalProgressions() error = %v", err)
	}

	want := strings.Join([]string{
		`version: 1`,
		`entry_id: char_0123456789abcdef0123`,
		`progressions: []`,
		``,
	}, "\n")
	if string(contents) != want {
		t.Fatalf("canonical empty-progression bytes mismatch:\nwant:\n%s\ngot:\n%s", want, contents)
	}
}
