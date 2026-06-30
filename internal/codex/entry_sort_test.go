// BDD Scenario: 3.1.1 - List entries
// Requirements: M3-R01
// Test purpose: Exported entry sorting uses the canonical type, name, and ID ordering without altering entry fields.
package codex

import (
	"reflect"
	"testing"
)

func TestSortEntriesUsesCanonicalTypeNameAndIDOrder(t *testing.T) {
	t.Parallel()

	// Test: sorting orders character, location, lore, and custom entries by case-sensitive name and then stable ID without changing their contents.
	// Requirements: M3-R01
	entries := []Entry{
		{ID: "custom_0123456789abcdef0124", Type: TypeCustom, Name: "Artifacts", Metadata: map[string]string{"slot": "4"}},
		{ID: "char_0123456789abcdef0124", Type: TypeCharacter, Name: "Ben", Metadata: map[string]string{"slot": "1"}},
		{ID: "lore_0123456789abcdef0124", Type: TypeLore, Name: "Force", Metadata: map[string]string{"slot": "3"}},
		{ID: "loc_0123456789abcdef0124", Type: TypeLocation, Name: "Alderaan", Metadata: map[string]string{"slot": "2"}},
		{ID: "char_0123456789abcdef0123", Type: TypeCharacter, Name: "Ben", Metadata: map[string]string{"slot": "0"}},
		{ID: "char_0123456789abcdef0125", Type: TypeCharacter, Name: "ben", Metadata: map[string]string{"slot": "5"}},
	}
	before := append([]Entry(nil), entries...)
	SortEntries(entries)
	gotIDs := []string{entries[0].ID, entries[1].ID, entries[2].ID, entries[3].ID, entries[4].ID, entries[5].ID}
	wantIDs := []string{
		"char_0123456789abcdef0123",
		"char_0123456789abcdef0124",
		"char_0123456789abcdef0125",
		"loc_0123456789abcdef0124",
		"lore_0123456789abcdef0124",
		"custom_0123456789abcdef0124",
	}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("sorted IDs = %#v, want %#v", gotIDs, wantIDs)
	}
	for _, entry := range entries {
		var original Entry
		for _, candidate := range before {
			if candidate.ID == entry.ID {
				original = candidate
				break
			}
		}
		if !reflect.DeepEqual(entry, original) {
			t.Fatalf("entry mutated for %q: %#v want %#v", entry.ID, entry, original)
		}
	}
}
