// BDD Scenario: 3.1.2 - Create an entry
// Requirements: M3-R02, M3-R04
// Test purpose: Entry creation normalizes mutable fields while preserving the required canonical type rules.
package codex

import (
	"reflect"
	"testing"
)

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
