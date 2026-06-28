package codex

import (
	"errors"
	"reflect"
	"testing"
)

// BDD Scenario: 3.1.2 - Create an entry
// Requirements: M3-R02, M3-R04
// Test purpose: Plain-English description of the normalization and stable-type rules that canonical Codex entry creation must enforce.
func TestNormalizeEntryCanonicalizesFields(t *testing.T) {
	t.Parallel()

	// Test: trims names and aliases, sorts tags, normalizes line endings, and trims metadata keys while preserving alias order.
	// Requirements: M3-R04
	entry, err := NormalizeEntry(Entry{
		ID:          "char_0123456789abcdef0123",
		Type:        TypeCharacter,
		Name:        "  Obi-Wan Kenobi  ",
		Aliases:     []string{"  Ben  ", "Old Ben"},
		Tags:        []string{"mentor", "jedi"},
		Description: "Guide.\r\nStill watching.\r",
		Metadata: map[string]string{
			" status ": "alive\r\n",
			"role":     "mentor",
		},
	})
	if err != nil {
		t.Fatalf("NormalizeEntry() error = %v", err)
	}
	if entry.Name != "Obi-Wan Kenobi" {
		t.Fatalf("name = %q", entry.Name)
	}
	if !reflect.DeepEqual(entry.Aliases, []string{"Ben", "Old Ben"}) {
		t.Fatalf("aliases = %#v", entry.Aliases)
	}
	if !reflect.DeepEqual(entry.Tags, []string{"jedi", "mentor"}) {
		t.Fatalf("tags = %#v", entry.Tags)
	}
	if entry.Description != "Guide.\nStill watching.\n" {
		t.Fatalf("description = %q", entry.Description)
	}
	if !reflect.DeepEqual(entry.Metadata, map[string]string{"role": "mentor", "status": "alive\n"}) {
		t.Fatalf("metadata = %#v", entry.Metadata)
	}
}

// BDD Scenario: 3.1.4 - Reject invalid entry data
// Requirements: M3-R02, M3-R03, M3-R04
// Test purpose: Plain-English description of the invalid create and edit payload rules for names, aliases, tags, descriptions, metadata, and type-specific IDs.
func TestNormalizeEntryRejectsInvalidFields(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		entry Entry
		want  error
	}{
		{
			name:  "unsupported type",
			entry: Entry{Type: EntryType("vehicle"), Name: "Falcon"},
			want:  ErrInvalidType,
		},
		{
			name:  "alias duplicates name",
			entry: Entry{Type: TypeCharacter, Name: "Ben", Aliases: []string{"Ben"}},
			want:  ErrInvalidAlias,
		},
		{
			name:  "duplicate tag",
			entry: Entry{Type: TypeCharacter, Name: "Ben", Tags: []string{"mentor", "mentor"}},
			want:  ErrInvalidTag,
		},
		{
			name:  "invalid tag shape",
			entry: Entry{Type: TypeCharacter, Name: "Ben", Tags: []string{"Bad Tag"}},
			want:  ErrInvalidTag,
		},
		{
			name:  "description contains NUL",
			entry: Entry{Type: TypeCharacter, Name: "Ben", Description: "bad\x00text"},
			want:  ErrInvalidDescription,
		},
		{
			name:  "metadata empty key after trim",
			entry: Entry{Type: TypeCharacter, Name: "Ben", Metadata: map[string]string{"  ": "x"}},
			want:  ErrInvalidMetadata,
		},
		{
			name:  "entry ID wrong prefix for type",
			entry: Entry{ID: "loc_0123456789abcdef0123", Type: TypeCharacter, Name: "Ben"},
			want:  ErrInvalidID,
		},
	}

	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			// Test: rejects each invalid field class with the typed domain error required by the contract.
			// Requirements: M3-R02
			_, err := NormalizeEntry(testCase.entry)
			if !errors.Is(err, testCase.want) {
				t.Fatalf("NormalizeEntry() error = %v, want %v", err, testCase.want)
			}
		})
	}
}
