// BDD Scenario: 3.3.3 - Apply multiple progressions deterministically
// Requirements: M3-R06, M3-R07
// Test purpose: Pure active-state resolution applies multiple progressions in chronology, before/after order, and document order while preserving base aliases and tags.
package codex

import (
	"reflect"
	"testing"
)

// Test: later chronology wins, before changes apply before after changes at one anchor, document order breaks ties, and aliases/tags stay on the base entry.
// Requirements: M3-R06, M3-R07
func TestResolveActiveStateAppliesProgressionsDeterministically(t *testing.T) {
	t.Parallel()

	beforeDescription := "Before scene one."
	afterDescription := "After scene one."
	laterDescription := "After scene two."
	entry := Entry{
		ID:          "char_0123456789abcdef0123",
		Type:        TypeCharacter,
		Name:        "Leia",
		Aliases:     []string{"General"},
		Tags:        []string{"leader"},
		Description: "Base",
		Metadata: map[string]string{
			"status": "active",
			"rank":   "general",
		},
	}
	progressions := []Progression{
		{
			ID:      "prog_00000000000000000001",
			Anchor:  ProgressionAnchor{Type: "scene", ID: "scn_00000000000000000001", Timing: "after"},
			Changes: ProgressionChange{Description: &afterDescription, Metadata: map[string]string{"status": "injured"}},
		},
		{
			ID:      "prog_00000000000000000002",
			Anchor:  ProgressionAnchor{Type: "scene", ID: "scn_00000000000000000001", Timing: "before"},
			Changes: ProgressionChange{Description: &beforeDescription, Metadata: map[string]string{"rank": "commander"}},
		},
		{
			ID:      "prog_00000000000000000003",
			Anchor:  ProgressionAnchor{Type: "scene", ID: "scn_00000000000000000002", Timing: "after"},
			Changes: ProgressionChange{Description: &laterDescription, Metadata: map[string]string{"status": "recovered", "rank": "commander"}},
		},
	}

	activeAtSceneThree, err := ResolveActiveState(entry, progressions, []SceneRef{
		{ID: "scn_00000000000000000001"},
		{ID: "scn_00000000000000000002"},
		{ID: "scn_00000000000000000003"},
	}, "scn_00000000000000000003")
	if err != nil {
		t.Fatalf("ResolveActiveState(scene three) error = %v", err)
	}
	if activeAtSceneThree.Entry.Description != "After scene two." {
		t.Fatalf("scene three description = %q", activeAtSceneThree.Entry.Description)
	}
	if !reflect.DeepEqual(activeAtSceneThree.Entry.Metadata, map[string]string{"rank": "commander", "status": "recovered"}) {
		t.Fatalf("scene three metadata = %#v", activeAtSceneThree.Entry.Metadata)
	}
	if !reflect.DeepEqual(activeAtSceneThree.AppliedProgressionIDs, []string{
		"prog_00000000000000000002",
		"prog_00000000000000000001",
		"prog_00000000000000000003",
	}) {
		t.Fatalf("scene three applied IDs = %#v", activeAtSceneThree.AppliedProgressionIDs)
	}
	if !reflect.DeepEqual(activeAtSceneThree.Entry.Aliases, []string{"General"}) || !reflect.DeepEqual(activeAtSceneThree.Entry.Tags, []string{"leader"}) {
		t.Fatalf("base fields changed = %#v", activeAtSceneThree.Entry)
	}
}
