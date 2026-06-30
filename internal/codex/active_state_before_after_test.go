// BDD Scenario: 3.3.1 - Resolve before and after an anchor
// Requirements: M3-R07
// Test purpose: Pure active-state resolution excludes after-anchor progressions at the anchor scene and includes them at later scenes.
package codex

import (
	"reflect"
	"testing"
)

// Test: an after-anchor progression is excluded at its anchor scene and included at a later scene.
// Requirements: M3-R07
func TestResolveActiveStateExcludesAfterAnchorAtSceneAndIncludesItLater(t *testing.T) {
	t.Parallel()

	afterDescription := "After scene one."
	entry := Entry{
		ID:          "char_0123456789abcdef0123",
		Type:        TypeCharacter,
		Name:        "Leia",
		Aliases:     []string{"General"},
		Tags:        []string{"leader"},
		Description: "Base",
		Metadata:    map[string]string{"status": "active"},
	}
	progressions := []Progression{{
		ID:      "prog_00000000000000000001",
		Anchor:  ProgressionAnchor{Type: "scene", ID: "scn_00000000000000000001", Timing: "after"},
		Changes: ProgressionChange{Description: &afterDescription, Metadata: map[string]string{"status": "injured"}},
	}}
	orderedScenes := []SceneRef{{ID: "scn_00000000000000000001"}, {ID: "scn_00000000000000000002"}}

	activeAtAnchor, err := ResolveActiveState(entry, progressions, orderedScenes, "scn_00000000000000000001")
	if err != nil {
		t.Fatalf("ResolveActiveState(anchor) error = %v", err)
	}
	if activeAtAnchor.Entry.Description != "Base" {
		t.Fatalf("anchor description = %q", activeAtAnchor.Entry.Description)
	}
	if len(activeAtAnchor.AppliedProgressionIDs) != 0 {
		t.Fatalf("anchor applied IDs = %#v", activeAtAnchor.AppliedProgressionIDs)
	}

	activeLater, err := ResolveActiveState(entry, progressions, orderedScenes, "scn_00000000000000000002")
	if err != nil {
		t.Fatalf("ResolveActiveState(later) error = %v", err)
	}
	if activeLater.Entry.Description != "After scene one." {
		t.Fatalf("later description = %q", activeLater.Entry.Description)
	}
	if !reflect.DeepEqual(activeLater.AppliedProgressionIDs, []string{"prog_00000000000000000001"}) {
		t.Fatalf("later applied IDs = %#v", activeLater.AppliedProgressionIDs)
	}
}
