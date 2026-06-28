package codex

import (
	"errors"
	"testing"
)

// BDD Scenario: 3.2.1 - Save ordered progressions
// Requirements: M3-R05, M3-R06
// Test purpose: Plain-English description of the progression validation rules for stable scene anchors, before-after timing, and effective changes.
func TestNormalizeProgressionsAcceptsCanonicalRows(t *testing.T) {
	t.Parallel()

	description := "Gone, but influential.\r\n"
	// Test: accepts ordered progressions with valid scene anchors and normalizes their changes without reordering the list.
	// Requirements: M3-R05
	progressions, err := NormalizeProgressions("char_0123456789abcdef0123", []Progression{
		{
			ID: "prog_0123456789abcdef0123",
			Anchor: ProgressionAnchor{
				Type:   "scene",
				ID:     "scn_0123456789abcdef0123",
				Timing: "after",
			},
			Changes: ProgressionChange{
				Description: &description,
				Metadata:    map[string]string{"status": "deceased"},
			},
		},
	}, map[string]struct{}{"scn_0123456789abcdef0123": {}})
	if err != nil {
		t.Fatalf("NormalizeProgressions() error = %v", err)
	}
	if len(progressions) != 1 || progressions[0].Anchor.Timing != "after" {
		t.Fatalf("progressions = %#v", progressions)
	}
	if *progressions[0].Changes.Description != "Gone, but influential.\n" {
		t.Fatalf("description = %q", *progressions[0].Changes.Description)
	}
}

// BDD Scenario: 3.2.2 - Reject invalid progressions
// Requirements: M3-R05, M3-R06
// Test purpose: Plain-English description of the invalid progression cases for malformed IDs, duplicate anchors, invalid timing, unknown scenes, and ineffective changes.
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
			want:         ErrSceneNotFound,
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
			name:         "malformed progression ID",
			progressions: []Progression{{ID: "bad", Anchor: ProgressionAnchor{Type: "scene", ID: "scn_0123456789abcdef0123", Timing: "before"}, Changes: ProgressionChange{Description: &description}}},
			sceneIDs:     map[string]struct{}{"scn_0123456789abcdef0123": {}},
			want:         ErrInvalidID,
		},
	}

	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			// Test: rejects each invalid progression class without silently repairing anchors, timing, or change sets.
			// Requirements: M3-R06
			_, err := NormalizeProgressions("char_0123456789abcdef0123", testCase.progressions, testCase.sceneIDs)
			if !errors.Is(err, testCase.want) {
				t.Fatalf("NormalizeProgressions() error = %v, want %v", err, testCase.want)
			}
		})
	}
}
