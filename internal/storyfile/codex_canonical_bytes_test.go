// BDD Scenario: 3.1.2 - Create an entry
// Requirements: M3-R04, M3-R05, M3-R18
// Test purpose: Prove the canonical YAML byte shape for Codex entries and progression documents: two-space indentation, exactly one terminal newline, sorted metadata keys, and [] / {} for empty aliases, tags, and metadata.
package storyfile

import (
	"strings"
	"testing"

	"storywork/internal/codex"
)

func TestMarshalCodexEntryProducesExactCanonicalBytes(t *testing.T) {
	t.Parallel()

	entry := codex.Entry{
		ID:          "char_0123456789abcdef0123",
		Type:        codex.TypeCharacter,
		Name:        "Obi-Wan Kenobi",
		Aliases:     []string{"Ben", "Old Ben"},
		Tags:        []string{"jedi", "mentor"},
		Description: "A former Jedi acting as Luke's guide.",
		Metadata:    map[string]string{"role": "mentor", "status": "alive"},
	}
	store := New()
	contents, err := store.MarshalCodexEntry(entry)
	if err != nil {
		t.Fatalf("MarshalCodexEntry() error = %v", err)
	}

	// Test: a populated entry serializes to the exact canonical YAML byte shape documented in the contract: unquoted safe scalars, two-space indentation, sorted metadata keys, and one terminal newline.
	// Requirements: M3-R04
	want := strings.Join([]string{
		`version: 1`,
		`id: char_0123456789abcdef0123`,
		`type: character`,
		`name: Obi-Wan Kenobi`,
		`aliases:`,
		`  - Ben`,
		`  - Old Ben`,
		`tags:`,
		`  - jedi`,
		`  - mentor`,
		`description: A former Jedi acting as Luke's guide.`,
		`metadata:`,
		`  role: mentor`,
		`  status: alive`,
		``,
	}, "\n")
	if string(contents) != want {
		t.Fatalf("canonical entry bytes mismatch:\nwant:\n%s\ngot:\n%s", want, contents)
	}
}

func TestMarshalCodexEntryQuotesDescriptionsWithNewlines(t *testing.T) {
	t.Parallel()

	// Descriptions that contain newlines must be double-quoted so the newline
	// serializes as the documented \n escape rather than as a block scalar.
	entry := codex.Entry{
		ID:          "char_0123456789abcdef0123",
		Type:        codex.TypeCharacter,
		Name:        "Ben",
		Aliases:     []string{},
		Tags:        []string{},
		Description: "Line one.\nLine two.",
		Metadata:    map[string]string{},
	}
	store := New()
	contents, err := store.MarshalCodexEntry(entry)
	if err != nil {
		t.Fatalf("MarshalCodexEntry() error = %v", err)
	}

	// Test: a description containing a newline is double-quoted with the \n escape, preserving canonical stability.
	// Requirements: M3-R04
	want := strings.Join([]string{
		`version: 1`,
		`id: char_0123456789abcdef0123`,
		`type: character`,
		`name: Ben`,
		`aliases: []`,
		`tags: []`,
		`description: "Line one.\nLine two."`,
		`metadata: {}`,
		``,
	}, "\n")
	if string(contents) != want {
		t.Fatalf("canonical newline-description bytes mismatch:\nwant:\n%s\ngot:\n%s", want, contents)
	}
}

func TestMarshalCodexEntryEmptyCollectionsUseCanonicalEmptyShape(t *testing.T) {
	t.Parallel()

	entry := codex.Entry{
		ID:          "loc_0123456789abcdef0123",
		Type:        codex.TypeLocation,
		Name:        "Tatooine",
		Aliases:     []string{},
		Tags:        []string{},
		Description: "Desert.",
		Metadata:    map[string]string{},
	}
	store := New()
	contents, err := store.MarshalCodexEntry(entry)
	if err != nil {
		t.Fatalf("MarshalCodexEntry() error = %v", err)
	}

	// Test: empty aliases, tags, and metadata serialize as [] / [] / {}, not null or omitted, with one terminal newline.
	// Requirements: M3-R04
	want := strings.Join([]string{
		`version: 1`,
		`id: loc_0123456789abcdef0123`,
		`type: location`,
		`name: Tatooine`,
		`aliases: []`,
		`tags: []`,
		`description: Desert.`,
		`metadata: {}`,
		``,
	}, "\n")
	if string(contents) != want {
		t.Fatalf("canonical empty-collection bytes mismatch:\nwant:\n%s\ngot:\n%s", want, contents)
	}
}

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

	// Test: a populated progression document serializes to the exact canonical YAML bytes documented in the contract with unquoted safe scalars and two-space indentation.
	// Requirements: M3-R05
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

func TestMarshalProgressionsEmptyDocumentUsesCanonicalEmptyShape(t *testing.T) {
	t.Parallel()

	// Saving an empty list over an existing file writes a canonical empty document; it does not delete the file.
	document := codex.ProgressionDocument{
		EntryID:      "char_0123456789abcdef0123",
		Progressions: []codex.Progression{},
	}
	store := New()
	contents, err := store.MarshalProgressions(document)
	if err != nil {
		t.Fatalf("MarshalProgressions() error = %v", err)
	}

	// Test: an empty progression list serializes as progressions: [] with one terminal newline, not null or omitted.
	// Requirements: M3-R05
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
