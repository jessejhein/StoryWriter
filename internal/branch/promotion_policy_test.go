// BDD Scenario: 8.4.2 - Reject a path changed on canon
// Requirements: M8-R12, M8-R13
// Test purpose: Pure promotion conflict policy intersects selected and main-changed paths.

package branch_test

import (
	"errors"
	"testing"

	"storywork/internal/branch"
)

// Test: selected paths changed on main since base conflict.
// Requirements: M8-R13.
func TestPromotionConflictsDetectsMainDivergence(t *testing.T) {
	t.Parallel()
	selected := []branch.ProjectPath{"outline.yaml", "scenes/scn_00000000000000000001.md"}
	mainChanged := []branch.ProjectPath{"outline.yaml"}
	conflicts := branch.PromotionConflicts(selected, mainChanged)
	if len(conflicts) != 1 || conflicts[0] != "outline.yaml" {
		t.Fatalf("conflicts = %#v", conflicts)
	}
}

// Test: stale refs are rejected before mutation.
// Requirements: M8-R12.
func TestValidatePromotionPreflightRejectsStaleRefs(t *testing.T) {
	t.Parallel()
	comparison := branch.Comparison{
		MainHead:       "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ExperimentHead: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Fingerprint:    "sha256:00",
		Files:          []branch.ChangedFile{{Path: "outline.yaml", Status: branch.StatusModified}},
	}
	err := branch.ValidatePromotionPreflight(comparison, []branch.ProjectPath{"outline.yaml"}, nil,
		"cccccccccccccccccccccccccccccccccccccccc", comparison.ExperimentHead, "sha256:00")
	if !errors.Is(err, branch.ErrStaleRef) {
		t.Fatalf("err = %v, want ErrStaleRef", err)
	}
}
