// BDD Scenario: 3.3.3 - Apply multiple progressions deterministically
// Requirements: M3-R06, M3-R07, M3-R08
// Test purpose: Plain-English description of the pure active-state resolution rules for chronology, before-after timing, document-order tie breaks, and stable scene anchors after reorder.
package codex

import (
	"reflect"
	"testing"
)

func TestResolveActiveStateAppliesProgressionsInChronology(t *testing.T) {
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
			Changes: ProgressionChange{Description: &laterDescription, Metadata: map[string]string{"status": "recovered"}},
		},
	}

	// Test: before applies at the anchor scene, after applies only later, and later chronology wins while aliases and tags stay on the base entry.
	// Requirements: M3-R07
	activeAtSceneOne, err := ResolveActiveState(entry, progressions, []SceneRef{
		{ID: "scn_00000000000000000001"},
		{ID: "scn_00000000000000000002"},
		{ID: "scn_00000000000000000003"},
	}, "scn_00000000000000000001")
	if err != nil {
		t.Fatalf("ResolveActiveState(scene one) error = %v", err)
	}
	if activeAtSceneOne.Entry.Description != "Before scene one." {
		t.Fatalf("scene one description = %q", activeAtSceneOne.Entry.Description)
	}
	if !reflect.DeepEqual(activeAtSceneOne.AppliedProgressionIDs, []string{"prog_00000000000000000002"}) {
		t.Fatalf("scene one applied IDs = %#v", activeAtSceneOne.AppliedProgressionIDs)
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
	if !reflect.DeepEqual(activeAtSceneThree.Entry.Aliases, []string{"General"}) || !reflect.DeepEqual(activeAtSceneThree.Entry.Tags, []string{"leader"}) {
		t.Fatalf("base fields changed = %#v", activeAtSceneThree.Entry)
	}

	// Test: reordering scenes changes activation chronology without changing the stored scene anchor IDs.
	// Requirements: M3-R08
	activeAfterReorder, err := ResolveActiveState(entry, progressions[:1], []SceneRef{
		{ID: "scn_00000000000000000002"},
		{ID: "scn_00000000000000000001"},
		{ID: "scn_00000000000000000003"},
	}, "scn_00000000000000000002")
	if err != nil {
		t.Fatalf("ResolveActiveState(reordered scene) error = %v", err)
	}
	if len(activeAfterReorder.AppliedProgressionIDs) != 0 {
		t.Fatalf("applied IDs after reorder = %#v, want none", activeAfterReorder.AppliedProgressionIDs)
	}
}
