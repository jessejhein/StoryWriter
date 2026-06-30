// BDD Scenario: 3.1.4 - Reject invalid entry data
// Requirements: M3-R02, M3-R03, M3-R04, M3-R09
// Test purpose: Exported entry identity helpers accept only canonical types, IDs, and directories.
package codex

import (
	"errors"
	"testing"
)

func TestEntryIdentityHelpersValidateCanonicalTypesAndIDs(t *testing.T) {
	t.Parallel()

	// Test: exported entry identity helpers accept the four canonical types and exact stable ID shapes while rejecting malformed values.
	// Requirements: M3-R02, M3-R03, M3-R09
	validTypes := []EntryType{TypeCharacter, TypeLocation, TypeLore, TypeCustom}
	validIDs := map[EntryType]string{
		TypeCharacter: "char_0123456789abcdef0123",
		TypeLocation:  "loc_0123456789abcdef0123",
		TypeLore:      "lore_0123456789abcdef0123",
		TypeCustom:    "custom_0123456789abcdef0123",
	}
	for _, entryType := range validTypes {
		if got, err := ValidateEntryType(entryType); err != nil || got != entryType {
			t.Fatalf("ValidateEntryType(%q) = %q, %v", entryType, got, err)
		}
		if err := ValidateEntryID(validIDs[entryType]); err != nil {
			t.Fatalf("ValidateEntryID(%q) error = %v", validIDs[entryType], err)
		}
		if err := ValidateEntryIDForType(entryType, validIDs[entryType]); err != nil {
			t.Fatalf("ValidateEntryIDForType(%q, %q) error = %v", entryType, validIDs[entryType], err)
		}
		if got, err := TypeForID(validIDs[entryType]); err != nil || got != entryType {
			t.Fatalf("TypeForID(%q) = %q, %v", validIDs[entryType], got, err)
		}
	}

	for _, invalidType := range []EntryType{"", "vehicle"} {
		if _, err := ValidateEntryType(invalidType); !errors.Is(err, ErrInvalidType) {
			t.Fatalf("ValidateEntryType(%q) error = %v, want %v", invalidType, err, ErrInvalidType)
		}
	}

	for _, invalidID := range []string{
		"CHAR_0123456789abcdef0123",
		"char_0123456789abcdef012",
		"char_0123456789abcdef01234",
		"ship_0123456789abcdef0123",
		"char_0123456789ABCDEF0123",
		"char_/etc/passwd012345678",
		"char_0123456789abcdef/../",
	} {
		if err := ValidateEntryID(invalidID); !errors.Is(err, ErrInvalidID) {
			t.Fatalf("ValidateEntryID(%q) error = %v, want %v", invalidID, err, ErrInvalidID)
		}
		if _, err := TypeForID(invalidID); !errors.Is(err, ErrInvalidID) {
			t.Fatalf("TypeForID(%q) error = %v, want %v", invalidID, err, ErrInvalidID)
		}
	}

	for _, mismatch := range []struct {
		entryType EntryType
		id        string
	}{
		{entryType: TypeCharacter, id: validIDs[TypeLocation]},
		{entryType: TypeLocation, id: validIDs[TypeLore]},
		{entryType: TypeLore, id: validIDs[TypeCustom]},
		{entryType: TypeCustom, id: validIDs[TypeCharacter]},
		{entryType: "vehicle", id: validIDs[TypeCharacter]},
	} {
		if err := ValidateEntryIDForType(mismatch.entryType, mismatch.id); !errors.Is(err, ErrInvalidID) && !errors.Is(err, ErrInvalidType) {
			t.Fatalf("ValidateEntryIDForType(%q, %q) error = %v", mismatch.entryType, mismatch.id, err)
		}
	}
}

func TestDirectoryForTypeReturnsCanonicalDirectories(t *testing.T) {
	t.Parallel()

	// Test: the exported directory helper returns the exact canonical subdirectory names and rejects unsupported types.
	// Requirements: M3-R09
	cases := map[EntryType]string{
		TypeCharacter: "characters",
		TypeLocation:  "locations",
		TypeLore:      "lore",
		TypeCustom:    "custom",
	}
	for entryType, want := range cases {
		got, err := DirectoryForType(entryType)
		if err != nil || got != want {
			t.Fatalf("DirectoryForType(%q) = %q, %v", entryType, got, err)
		}
	}
	if _, err := DirectoryForType("vehicle"); !errors.Is(err, ErrInvalidType) {
		t.Fatalf("DirectoryForType(invalid) error = %v, want %v", err, ErrInvalidType)
	}
}
