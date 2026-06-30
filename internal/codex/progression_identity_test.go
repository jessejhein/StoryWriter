// BDD Scenario: 3.2.2 - Reject invalid progressions
// Requirements: M3-R05, M3-R06, M3-R09
// Test purpose: Exported progression identity helpers validate canonical progression and scene ID shapes.
package codex

import (
	"errors"
	"reflect"
	"testing"
)

func TestProgressionIdentityHelpersValidateCanonicalShapes(t *testing.T) {
	t.Parallel()

	// Test: exported progression and scene ID helpers accept only the canonical lowercase hex shapes and reject malformed values.
	// Requirements: M3-R05, M3-R09
	if err := ValidateProgressionID("prog_0123456789abcdef0123"); err != nil {
		t.Fatalf("ValidateProgressionID(valid) error = %v", err)
	}
	if err := ValidateSceneID("scn_0123456789abcdef0123"); err != nil {
		t.Fatalf("ValidateSceneID(valid) error = %v", err)
	}
	for _, invalid := range []string{"PROG_0123456789abcdef0123", "prog_0123456789abcdef012", "prog_0123456789ABCDEF0123", "prog_0123456789abcdef01234"} {
		if err := ValidateProgressionID(invalid); !errors.Is(err, ErrInvalidID) {
			t.Fatalf("ValidateProgressionID(%q) error = %v, want %v", invalid, err, ErrInvalidID)
		}
	}
	for _, invalid := range []string{"SCN_0123456789abcdef0123", "scn_0123456789abcdef012", "scn_0123456789ABCDEF0123", "scn_0123456789abcdef01234"} {
		if err := ValidateSceneID(invalid); !errors.Is(err, ErrInvalidID) {
			t.Fatalf("ValidateSceneID(%q) error = %v, want %v", invalid, err, ErrInvalidID)
		}
	}
}

func TestNormalizeProgressionsPreservesOrderAndDoesNotMutateInput(t *testing.T) {
	t.Parallel()

	// Test: valid progression normalization preserves request order, canonicalizes changes, and leaves the caller input slice unchanged.
	// Requirements: M3-R05
	description := "After.\r\n"
	source := []Progression{
		{
			ID:      "prog_0123456789abcdef0123",
			Anchor:  ProgressionAnchor{Type: "scene", ID: "scn_0123456789abcdef0123", Timing: "after"},
			Changes: ProgressionChange{Description: &description, Metadata: map[string]string{" status ": "alive\r\n"}},
		},
		{
			ID:      "prog_0123456789abcdef0124",
			Anchor:  ProgressionAnchor{Type: "scene", ID: "scn_0123456789abcdef0124", Timing: "before"},
			Changes: ProgressionChange{Metadata: map[string]string{"role": "mentor"}},
		},
	}
	input := append([]Progression(nil), source...)
	got, err := NormalizeProgressions("char_0123456789abcdef0123", input, map[string]struct{}{
		"scn_0123456789abcdef0123": {},
		"scn_0123456789abcdef0124": {},
	})
	if err != nil {
		t.Fatalf("NormalizeProgressions() error = %v", err)
	}
	if got[0].ID != source[0].ID || got[1].ID != source[1].ID {
		t.Fatalf("order changed = %#v", got)
	}
	if *got[0].Changes.Description != "After.\n" {
		t.Fatalf("normalized description = %q", *got[0].Changes.Description)
	}
	if !reflect.DeepEqual(input, source) {
		t.Fatalf("input mutated = %#v want %#v", input, source)
	}
}
