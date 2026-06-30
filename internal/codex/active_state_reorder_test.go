// BDD Scenario: 3.3.4 - Reorder around a stable anchor
// Requirements: M3-R07, M3-R08
// Test purpose: Pure active-state resolution follows reordered scene chronology without changing the stored stable scene anchor IDs.
package codex

import "testing"

// Test: reordering scenes changes activation chronology while the progression still points to the same stable scene ID.
// Requirements: M3-R07, M3-R08
func TestResolveActiveStateFollowsReorderedSceneChronology(t *testing.T) {
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

	activeAfterReorder, err := ResolveActiveState(entry, progressions, []SceneRef{
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
