// BDD Scenario: 3.2.1 - Save ordered progressions
// Requirements: M3-R05, M3-R06
// Test purpose: Pure progression ID reconciliation preserves current IDs, identifies genuinely new rows, and never mutates inputs.
package codex

import (
	"errors"
	"reflect"
	"testing"
)

func TestReconcileProgressionIDsPreservesExistingRowsAndFlagsNewOnes(t *testing.T) {
	t.Parallel()

	// Test: reconciliation preserves supplied existing IDs, reuses matching current IDs for missing-ID anchors, keeps request order, and reports indexes that still need generated IDs.
	// Requirements: M3-R05, M3-R06
	current := []Progression{
		{ID: "prog_0123456789abcdef0001", Anchor: ProgressionAnchor{Type: "scene", ID: "scn_0123456789abcdef0001", Timing: "after"}},
		{ID: "prog_0123456789abcdef0002", Anchor: ProgressionAnchor{Type: "scene", ID: "scn_0123456789abcdef0002", Timing: "before"}},
	}
	requested := []Progression{
		{ID: "prog_0123456789abcdef0002", Anchor: ProgressionAnchor{Type: "scene", ID: "scn_09999999999999999999", Timing: "after"}},
		{Anchor: ProgressionAnchor{Type: "scene", ID: "scn_0123456789abcdef0001", Timing: "after"}},
		{Anchor: ProgressionAnchor{Type: "scene", ID: "scn_0123456789abcdef0003", Timing: "before"}},
		{Anchor: ProgressionAnchor{Type: "scene", ID: "scn_0123456789abcdef0004", Timing: "after"}},
	}
	currentBefore := append([]Progression(nil), current...)
	requestedBefore := append([]Progression(nil), requested...)

	got, needsID, err := ReconcileProgressionIDs(current, requested)
	if err != nil {
		t.Fatalf("ReconcileProgressionIDs() error = %v", err)
	}
	if got[0].ID != "prog_0123456789abcdef0002" {
		t.Fatalf("supplied ID changed = %q", got[0].ID)
	}
	if got[1].ID != "prog_0123456789abcdef0001" {
		t.Fatalf("matching anchor ID = %q", got[1].ID)
	}
	if got[2].ID != "" || got[3].ID != "" {
		t.Fatalf("new rows should still need IDs: %#v", got)
	}
	if !reflect.DeepEqual(needsID, []int{2, 3}) {
		t.Fatalf("needsID = %#v, want %#v", needsID, []int{2, 3})
	}
	if !reflect.DeepEqual(current, currentBefore) || !reflect.DeepEqual(requested, requestedBefore) {
		t.Fatalf("inputs mutated: current=%#v requested=%#v", current, requested)
	}
}

func TestReconcileProgressionIDsRejectsFabricatedOrDuplicateClaims(t *testing.T) {
	t.Parallel()

	// Test: reconciliation rejects fabricated supplied IDs, duplicate supplied claims, and duplicate missing-ID requests that try to reuse one current row twice.
	// Requirements: M3-R06
	current := []Progression{
		{ID: "prog_0123456789abcdef0001", Anchor: ProgressionAnchor{Type: "scene", ID: "scn_0123456789abcdef0001", Timing: "after"}},
	}
	cases := []struct {
		name      string
		requested []Progression
		want      error
	}{
		{
			name:      "fabricated supplied ID",
			requested: []Progression{{ID: "prog_0123456789abcdef0999", Anchor: ProgressionAnchor{Type: "scene", ID: "scn_0123456789abcdef0002", Timing: "before"}}},
			want:      ErrInvalidProgression,
		},
		{
			name: "duplicate supplied claim",
			requested: []Progression{
				{ID: "prog_0123456789abcdef0001", Anchor: ProgressionAnchor{Type: "scene", ID: "scn_0123456789abcdef0001", Timing: "after"}},
				{ID: "prog_0123456789abcdef0001", Anchor: ProgressionAnchor{Type: "scene", ID: "scn_0123456789abcdef0003", Timing: "before"}},
			},
			want: ErrInvalidProgression,
		},
		{
			name: "duplicate missing anchor claim",
			requested: []Progression{
				{Anchor: ProgressionAnchor{Type: "scene", ID: "scn_0123456789abcdef0001", Timing: "after"}},
				{Anchor: ProgressionAnchor{Type: "scene", ID: "scn_0123456789abcdef0001", Timing: "after"}},
			},
			want: nil,
		},
	}
	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			got, needsID, err := ReconcileProgressionIDs(current, testCase.requested)
			if testCase.want != nil {
				if !errors.Is(err, testCase.want) {
					t.Fatalf("ReconcileProgressionIDs() error = %v, want %v", err, testCase.want)
				}
				return
			}
			if err != nil {
				t.Fatalf("ReconcileProgressionIDs() error = %v", err)
			}
			if got[0].ID != "prog_0123456789abcdef0001" || got[1].ID != "" || !reflect.DeepEqual(needsID, []int{1}) {
				t.Fatalf("duplicate missing-anchor result = %#v needsID=%#v", got, needsID)
			}
		})
	}
}
