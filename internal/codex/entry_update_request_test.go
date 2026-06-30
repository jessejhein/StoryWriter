// BDD Scenario: 3.1.3 - Update an entry
// Requirements: M3-R03, M3-R04, M3-R17
// Test purpose: Exported update-request normalization preserves immutable identity while canonicalizing mutable fields.
package codex

import (
	"errors"
	"reflect"
	"testing"
)

func TestNormalizeUpdateRequestPreservesIdentityAndClonesMutableFields(t *testing.T) {
	t.Parallel()

	// Test: update normalization validates route ID and revision shape, preserves immutable ID and type, and clones request collections into the result.
	// Requirements: M3-R03, M3-R04, M3-R17
	current := Entry{
		ID:          "char_0123456789abcdef0123",
		Type:        TypeCharacter,
		Name:        "Ben",
		Aliases:     []string{"General"},
		Tags:        []string{"mentor"},
		Description: "Guide.",
		Metadata:    map[string]string{"status": "alive"},
	}
	request := SaveEntryRequest{
		Name:             "  Obi-Wan Kenobi  ",
		Aliases:          []string{"  Ben  ", "Old Ben"},
		Tags:             []string{"mentor", "jedi"},
		Description:      "Guide.\r\nStill watching.\r",
		Metadata:         map[string]string{" status ": "alive\r\n", "role": "mentor"},
		ExpectedRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}

	got, err := NormalizeUpdateRequest(current.ID, current, request)
	if err != nil {
		t.Fatalf("NormalizeUpdateRequest() error = %v", err)
	}
	if got.ID != current.ID || got.Type != current.Type {
		t.Fatalf("identity changed = %#v", got)
	}
	if got.Name != "Obi-Wan Kenobi" || got.Description != "Guide.\nStill watching.\n" {
		t.Fatalf("normalized fields = %#v", got)
	}
	if !reflect.DeepEqual(got.Aliases, []string{"Ben", "Old Ben"}) || !reflect.DeepEqual(got.Tags, []string{"jedi", "mentor"}) {
		t.Fatalf("normalized aliases/tags = %#v", got)
	}
	if !reflect.DeepEqual(got.Metadata, map[string]string{"role": "mentor", "status": "alive\n"}) {
		t.Fatalf("metadata = %#v", got.Metadata)
	}
	request.Aliases[0] = "Changed"
	request.Tags[0] = "changed"
	request.Metadata["role"] = "changed"
	if !reflect.DeepEqual(got.Aliases, []string{"Ben", "Old Ben"}) || !reflect.DeepEqual(got.Tags, []string{"jedi", "mentor"}) {
		t.Fatalf("result aliases/tags aliased = %#v", got)
	}
	if !reflect.DeepEqual(got.Metadata, map[string]string{"role": "mentor", "status": "alive\n"}) {
		t.Fatalf("result metadata aliased = %#v", got.Metadata)
	}
}

func TestNormalizeUpdateRequestRejectsInvalidRouteIDAndRevision(t *testing.T) {
	t.Parallel()

	// Test: update normalization rejects malformed route IDs, invalid revision tokens, and invalid mutable fields before persistence.
	// Requirements: M3-R03, M3-R17
	current := Entry{ID: "char_0123456789abcdef0123", Type: TypeCharacter, Name: "Ben", Aliases: []string{}, Tags: []string{}, Metadata: map[string]string{}}
	cases := []struct {
		name    string
		entryID string
		request SaveEntryRequest
		want    error
	}{
		{name: "invalid route ID", entryID: "bad", request: SaveEntryRequest{Name: "Ben", ExpectedRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}, want: ErrInvalidID},
		{name: "invalid revision", entryID: current.ID, request: SaveEntryRequest{Name: "Ben", ExpectedRevision: "stale"}, want: ErrInvalidRevision},
		{name: "invalid mutable field", entryID: current.ID, request: SaveEntryRequest{Name: "Ben", Tags: []string{"Bad Tag"}, ExpectedRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}, want: ErrInvalidTag},
	}
	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if _, err := NormalizeUpdateRequest(testCase.entryID, current, testCase.request); !errors.Is(err, testCase.want) {
				t.Fatalf("NormalizeUpdateRequest() error = %v, want %v", err, testCase.want)
			}
		})
	}
}
