// BDD Scenario: 3.2.3 - Load stored progressions
// Requirements: M3-R05, M3-R18
// Test purpose: Stored progression validation enforces canonical document shape without consulting outline membership.
package codex

import (
	"errors"
	"reflect"
	"testing"
)

func TestNormalizeStoredProgressionsAcceptsCanonicalStoredRowsWithoutOutlineMembership(t *testing.T) {
	t.Parallel()

	// Test: stored progression normalization accepts well-formed rows even when their scene anchors are not checked against any current outline.
	// Requirements: M3-R05, M3-R18
	description := "Changed.\r\n"
	input := []Progression{{
		ID:      "prog_0123456789abcdef0123",
		Anchor:  ProgressionAnchor{Type: "scene", ID: "scn_09999999999999999999", Timing: "after"},
		Changes: ProgressionChange{Description: &description, Metadata: map[string]string{"status": "deceased"}},
	}}
	got, err := NormalizeStoredProgressions("char_0123456789abcdef0123", input)
	if err != nil {
		t.Fatalf("NormalizeStoredProgressions() error = %v", err)
	}
	if got[0].Anchor.ID != "scn_09999999999999999999" {
		t.Fatalf("anchor ID = %q", got[0].Anchor.ID)
	}
	if *got[0].Changes.Description != "Changed.\n" {
		t.Fatalf("description = %q", *got[0].Changes.Description)
	}
	if !reflect.DeepEqual(input[0].Changes.Metadata, map[string]string{"status": "deceased"}) {
		t.Fatalf("input mutated = %#v", input)
	}
}

func TestNormalizeStoredProgressionsRejectsMalformedStoredRows(t *testing.T) {
	t.Parallel()

	// Test: stored progression normalization rejects malformed IDs, duplicate IDs, duplicate anchors, invalid timing, and ineffective changes with typed errors.
	// Requirements: M3-R05, M3-R18
	description := "Changed."
	cases := []struct {
		name         string
		progressions []Progression
		want         error
	}{
		{name: "invalid progression ID", progressions: []Progression{{ID: "bad", Anchor: ProgressionAnchor{Type: "scene", ID: "scn_0123456789abcdef0123", Timing: "after"}, Changes: ProgressionChange{Description: &description}}}, want: ErrInvalidID},
		{name: "invalid scene ID", progressions: []Progression{{ID: "prog_0123456789abcdef0123", Anchor: ProgressionAnchor{Type: "scene", ID: "bad", Timing: "after"}, Changes: ProgressionChange{Description: &description}}}, want: ErrInvalidID},
		{name: "duplicate progression ID", progressions: []Progression{{ID: "prog_0123456789abcdef0123", Anchor: ProgressionAnchor{Type: "scene", ID: "scn_0123456789abcdef0123", Timing: "after"}, Changes: ProgressionChange{Description: &description}}, {ID: "prog_0123456789abcdef0123", Anchor: ProgressionAnchor{Type: "scene", ID: "scn_0123456789abcdef0124", Timing: "before"}, Changes: ProgressionChange{Description: &description}}}, want: ErrInvalidProgression},
		{name: "duplicate anchor", progressions: []Progression{{ID: "prog_0123456789abcdef0123", Anchor: ProgressionAnchor{Type: "scene", ID: "scn_0123456789abcdef0123", Timing: "after"}, Changes: ProgressionChange{Description: &description}}, {ID: "prog_0123456789abcdef0124", Anchor: ProgressionAnchor{Type: "scene", ID: "scn_0123456789abcdef0123", Timing: "after"}, Changes: ProgressionChange{Description: &description}}}, want: ErrInvalidProgression},
		{name: "invalid timing", progressions: []Progression{{ID: "prog_0123456789abcdef0123", Anchor: ProgressionAnchor{Type: "scene", ID: "scn_0123456789abcdef0123", Timing: "during"}, Changes: ProgressionChange{Description: &description}}}, want: ErrInvalidProgression},
		{name: "no changes", progressions: []Progression{{ID: "prog_0123456789abcdef0123", Anchor: ProgressionAnchor{Type: "scene", ID: "scn_0123456789abcdef0123", Timing: "after"}, Changes: ProgressionChange{}}}, want: ErrInvalidProgression},
	}
	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if _, err := NormalizeStoredProgressions("char_0123456789abcdef0123", testCase.progressions); !errors.Is(err, testCase.want) {
				t.Fatalf("NormalizeStoredProgressions() error = %v, want %v", err, testCase.want)
			}
		})
	}
}
