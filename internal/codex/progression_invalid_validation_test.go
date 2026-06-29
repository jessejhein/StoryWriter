// BDD Scenario: 3.2.2 - Reject invalid progressions
// Requirements: M3-R05, M3-R06
// Test purpose: Plain-English description of the invalid progression cases for malformed IDs, duplicate anchors, invalid timing, unknown scenes, and ineffective changes.
package codex

import (
	"errors"
	"testing"
)

func TestNormalizeProgressionsRejectsInvalidRows(t *testing.T) {
	t.Parallel()

	description := "Changed."
	cases := []struct {
		name         string
		progressions []Progression
		sceneIDs     map[string]struct{}
		want         error
	}{
		{
			name: "duplicate anchor and timing",
			progressions: []Progression{
				{Anchor: ProgressionAnchor{Type: "scene", ID: "scn_0123456789abcdef0123", Timing: "after"}, Changes: ProgressionChange{Description: &description}},
				{Anchor: ProgressionAnchor{Type: "scene", ID: "scn_0123456789abcdef0123", Timing: "after"}, Changes: ProgressionChange{Metadata: map[string]string{"status": "dead"}}},
			},
			sceneIDs: map[string]struct{}{"scn_0123456789abcdef0123": {}},
			want:     ErrInvalidProgression,
		},
		{
			name:         "unknown scene anchor",
			progressions: []Progression{{Anchor: ProgressionAnchor{Type: "scene", ID: "scn_0123456789abcdef0123", Timing: "after"}, Changes: ProgressionChange{Description: &description}}},
			sceneIDs:     map[string]struct{}{},
			want:         ErrInvalidProgression,
		},
		{
			name:         "invalid timing",
			progressions: []Progression{{Anchor: ProgressionAnchor{Type: "scene", ID: "scn_0123456789abcdef0123", Timing: "during"}, Changes: ProgressionChange{Description: &description}}},
			sceneIDs:     map[string]struct{}{"scn_0123456789abcdef0123": {}},
			want:         ErrInvalidProgression,
		},
		{
			name:         "no effective changes",
			progressions: []Progression{{Anchor: ProgressionAnchor{Type: "scene", ID: "scn_0123456789abcdef0123", Timing: "before"}, Changes: ProgressionChange{}}},
			sceneIDs:     map[string]struct{}{"scn_0123456789abcdef0123": {}},
			want:         ErrInvalidProgression,
		},
		{
			name:         "duplicate progression ID",
			progressions: []Progression{{ID: "prog_0123456789abcdef0123", Anchor: ProgressionAnchor{Type: "scene", ID: "scn_0123456789abcdef0123", Timing: "before"}, Changes: ProgressionChange{Description: &description}}, {ID: "prog_0123456789abcdef0123", Anchor: ProgressionAnchor{Type: "scene", ID: "scn_0123456789abcdef0124", Timing: "after"}, Changes: ProgressionChange{Description: &description}}},
			sceneIDs:     map[string]struct{}{"scn_0123456789abcdef0123": {}, "scn_0123456789abcdef0124": {}},
			want:         ErrInvalidProgression,
		},
	}

	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			// Test: rejects each invalid progression row with the typed contract error instead of accepting or repairing it.
			// Requirements: M3-R06
			_, err := NormalizeProgressions("char_0123456789abcdef0123", testCase.progressions, testCase.sceneIDs)
			if !errors.Is(err, testCase.want) {
				t.Fatalf("NormalizeProgressions() error = %v, want %v", err, testCase.want)
			}
		})
	}
}
