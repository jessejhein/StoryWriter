// BDD Scenario: 3.1.4 - Reject invalid entry data
// Requirements: M3-R02, M3-R03, M3-R04
// Test purpose: Entry validation rejects invalid names, aliases, tags, descriptions, metadata, and type-specific IDs.
package codex

import (
	"errors"
	"testing"
)

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
