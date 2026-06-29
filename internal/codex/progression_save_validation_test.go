// BDD Scenario: 3.2.1 - Save ordered progressions
// Requirements: M3-R05, M3-R06
// Test purpose: Plain-English description of the progression validation rules for stable scene anchors, before-after timing, and effective changes.
package codex

import "testing"

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
