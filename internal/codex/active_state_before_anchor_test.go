// BDD Scenario: 3.3.2 - Resolve a before anchor
// Requirements: M3-R07
// Test purpose: Pure active-state resolution includes before-anchor progressions at the anchor scene.
package codex

import (
	"reflect"
	"testing"
)

// Test: a before-anchor progression applies at its anchor scene.
// Requirements: M3-R07
func TestResolveActiveStateIncludesBeforeAnchorAtScene(t *testing.T) {
	t.Parallel()

	beforeDescription := "Before scene one."
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
		ID:      "prog_00000000000000000002",
		Anchor:  ProgressionAnchor{Type: "scene", ID: "scn_00000000000000000001", Timing: "before"},
		Changes: ProgressionChange{Description: &beforeDescription, Metadata: map[string]string{"status": "injured"}},
	}}

	activeAtAnchor, err := ResolveActiveState(entry, progressions, []SceneRef{{ID: "scn_00000000000000000001"}}, "scn_00000000000000000001")
	if err != nil {
		t.Fatalf("ResolveActiveState(anchor) error = %v", err)
	}
	if activeAtAnchor.Entry.Description != "Before scene one." {
		t.Fatalf("anchor description = %q", activeAtAnchor.Entry.Description)
	}
	if !reflect.DeepEqual(activeAtAnchor.AppliedProgressionIDs, []string{"prog_00000000000000000002"}) {
		t.Fatalf("anchor applied IDs = %#v", activeAtAnchor.AppliedProgressionIDs)
	}
}
