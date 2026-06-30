// BDD Scenario: 3.1.2 - Create an entry
// Requirements: M3-R02, M3-R04
// Test purpose: Exported create-request normalization trims, canonicalizes, clones, and rejects invalid mutable fields.
package codex

import (
	"errors"
	"reflect"
	"testing"
)

func TestNormalizeCreateRequestCanonicalizesMutableFields(t *testing.T) {
	t.Parallel()

	// Test: create normalization trims values, sorts tags, normalizes line endings, clones metadata, and clears any supplied revision.
	// Requirements: M3-R04
	request := SaveEntryRequest{
		Type:             TypeCharacter,
		Name:             "  Obi-Wan Kenobi  ",
		Aliases:          []string{"  Ben  ", "Old Ben"},
		Tags:             []string{"mentor", "jedi"},
		Description:      "Guide.\r\nStill watching.\r",
		Metadata:         map[string]string{" status ": "alive\r\n", "role": "mentor"},
		ExpectedRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}

	got, err := NormalizeCreateRequest(request)
	if err != nil {
		t.Fatalf("NormalizeCreateRequest() error = %v", err)
	}
	if got.Name != "Obi-Wan Kenobi" {
		t.Fatalf("name = %q", got.Name)
	}
	if !reflect.DeepEqual(got.Aliases, []string{"Ben", "Old Ben"}) {
		t.Fatalf("aliases = %#v", got.Aliases)
	}
	if !reflect.DeepEqual(got.Tags, []string{"jedi", "mentor"}) {
		t.Fatalf("tags = %#v", got.Tags)
	}
	if got.Description != "Guide.\nStill watching.\n" {
		t.Fatalf("description = %q", got.Description)
	}
	if !reflect.DeepEqual(got.Metadata, map[string]string{"role": "mentor", "status": "alive\n"}) {
		t.Fatalf("metadata = %#v", got.Metadata)
	}
	if got.ExpectedRevision != "" {
		t.Fatalf("expected revision = %q, want empty", got.ExpectedRevision)
	}
	request.Aliases[0] = "Changed"
	request.Tags[0] = "changed"
	request.Metadata["role"] = "changed"
	if !reflect.DeepEqual(got.Aliases, []string{"Ben", "Old Ben"}) {
		t.Fatalf("aliases aliased = %#v", got.Aliases)
	}
	if !reflect.DeepEqual(got.Tags, []string{"jedi", "mentor"}) {
		t.Fatalf("tags aliased = %#v", got.Tags)
	}
	if !reflect.DeepEqual(got.Metadata, map[string]string{"role": "mentor", "status": "alive\n"}) {
		t.Fatalf("metadata aliased = %#v", got.Metadata)
	}
}

func TestNormalizeCreateRequestRejectsInvalidFields(t *testing.T) {
	t.Parallel()

	// Test: create normalization rejects unsupported types and invalid mutable field content with typed domain errors.
	// Requirements: M3-R02
	cases := []struct {
		name    string
		request SaveEntryRequest
		want    error
	}{
		{name: "invalid type", request: SaveEntryRequest{Type: "vehicle", Name: "Falcon"}, want: ErrInvalidType},
		{name: "empty name", request: SaveEntryRequest{Type: TypeCharacter, Name: "  "}, want: ErrInvalidName},
		{name: "duplicate tag", request: SaveEntryRequest{Type: TypeCharacter, Name: "Ben", Tags: []string{"mentor", "mentor"}}, want: ErrInvalidTag},
		{name: "invalid metadata", request: SaveEntryRequest{Type: TypeCharacter, Name: "Ben", Metadata: map[string]string{" ": "value"}}, want: ErrInvalidMetadata},
	}
	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if _, err := NormalizeCreateRequest(testCase.request); !errors.Is(err, testCase.want) {
				t.Fatalf("NormalizeCreateRequest() error = %v, want %v", err, testCase.want)
			}
		})
	}
}
