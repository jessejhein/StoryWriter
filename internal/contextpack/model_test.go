// BDD Scenario: 7.1.1 - Preview minimal Line Polish context
// Requirements: M7-R07, M7-R10
// Test purpose: Context targets, packets, material values, and manifests stay typed and redacted.

package contextpack

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// Test: targets require exactly one scope payload.
// Requirements: M7-R01.
func TestTargetsRequireExactlyOneMatchingScopePayload(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		target Target
	}{
		{name: "missing payload", target: Target{Scope: ScopeSelection}},
		{name: "mixed payloads", target: Target{
			Scope:     ScopeScene,
			Selection: &SelectionTarget{SceneID: "scn_0123456789abcdef0123", SceneRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
			Scene:     &SceneTarget{SceneID: "scn_0123456789abcdef0123", SceneRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		}},
		{name: "scope mismatch", target: Target{
			Scope: ScopeSelection,
			Scene: &SceneTarget{SceneID: "scn_0123456789abcdef0123", SceneRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		}},
	}
	for _, testCase := range cases {
		if err := ValidateTarget(testCase.target); err == nil {
			t.Fatalf("%s: ValidateTarget() error = nil, want failure", testCase.name)
		}
	}
	if err := ValidateTarget(Target{
		Scope: ScopeSelection,
		Selection: &SelectionTarget{
			SceneID: "scn_0123456789abcdef0123", SceneRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			StartByte: 0, EndByte: 4, SelectedText: "Alpha",
		},
	}); err != nil {
		t.Fatalf("ValidateTarget(selection) error = %v", err)
	}
}

// Test: builder rejects material whose typed scope does not match the request.
// Requirements: M7-R01, M7-R07.
func TestBuilderRejectsMaterialScopeMismatch(t *testing.T) {
	t.Parallel()

	_, _, err := NewBuilder().Build(BuildRequest{
		Scope: ScopeScene, Policy: Policy{Required: []Pack{PackCurrentScene, PackStyleSheet}},
		Budget: Budget{MaxInputEstimatedTokens: 100}, RAGMode: RAGModeNone,
		Material: Material{Scope: ScopeSelection, SceneMarkdown: "wrong shape"},
	})
	if !errors.Is(err, ErrInvalidTarget) {
		t.Fatalf("Build() error = %v, want ErrInvalidTarget", err)
	}
}

// Test: packets use explicit selection, scene, and chapter variants.
// Requirements: M7-R02, M7-R03, M7-R04.
func TestPacketsUseExplicitSelectionSceneAndChapterVariants(t *testing.T) {
	t.Parallel()

	selection := SelectionPacket{SelectedText: "Alpha", Style: StyleSheet{ID: "precise_editor"}}
	scene := ScenePacket{SceneMarkdown: "Scene body", Style: StyleSheet{ID: "precise_editor"}}
	chapter := ChapterReviewPacket{ChapterID: "ch_0123456789abcdef0123", Style: StyleSheet{ID: "precise_editor"}}
	if selection.Scope() != ScopeSelection || scene.Scope() != ScopeScene || chapter.Scope() != ScopeChapterReview {
		t.Fatalf("packet scopes = %s/%s/%s", selection.Scope(), scene.Scope(), chapter.Scope())
	}
}

// Test: context values clone slices and maps before returning packets.
// Requirements: M7-R18.
func TestContextValuesCloneSlicesAndMaps(t *testing.T) {
	t.Parallel()

	metadata := map[string]string{"role": "captain"}
	entry := CodexEntryState{
		EntryID: "char_0123456789abcdef0123", Name: "Ann", Metadata: metadata,
		AppliedProgressionIDs: []string{"prog_0123456789abcdef0123"},
	}
	packet := ScenePacket{
		SceneMarkdown: "Ann arrived.",
		ActiveCodex:   cloneCodexEntryStates([]CodexEntryState{entry}),
	}
	packet.ActiveCodex[0].Metadata["role"] = "changed"
	packet.ActiveCodex[0].AppliedProgressionIDs[0] = "prog_changed"
	if entry.Metadata["role"] != "captain" || entry.AppliedProgressionIDs[0] != "prog_0123456789abcdef0123" {
		t.Fatalf("entry mutated through packet alias: %#v", entry)
	}
	cloned := cloneCodexEntryState(entry)
	cloned.Metadata["role"] = "mutated"
	if entry.Metadata["role"] != "captain" {
		t.Fatal("cloneCodexEntryState() did not deep-clone metadata")
	}
}

// Test: manifest JSON contains only redacted references and counts.
// Requirements: M7-R10.
func TestManifestJSONContainsOnlyRedactedReferencesAndCounts(t *testing.T) {
	t.Parallel()

	manifest := Manifest{
		Scope:                   ScopeScene,
		PacksUsed:               []Pack{PackCurrentScene, PackStyleSheet, PackActiveCodex},
		PacksOmitted:            []PackOmission{{Pack: PackOutlineNeighbor, Reason: OmissionReasonBudget}},
		EstimatedInputTokens:    4312,
		MaxInputEstimatedTokens: 12000,
		RAGMode:                 RAGModeTimelineAware,
		ActiveCodex:             []ManifestCodexRef{{EntryID: "char_0123456789abcdef0123", AppliedProgressionIDs: []string{"prog_0123456789abcdef0123"}}},
		OutlineRefs:             []string{"scn_0123456789abcdef0123", "ch_0123456789abcdef0123"},
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	body := string(encoded)
	for _, forbidden := range []string{"Ann arrived", "captain", "system_prompt", "api_key", "http://"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("manifest leaked %q: %s", forbidden, body)
		}
	}
	for _, required := range []string{`"scope":"scene"`, `"packs_used"`, `"active_codex"`, `"outline_refs"`, `"estimated_input_tokens"`} {
		if !strings.Contains(body, required) {
			t.Fatalf("manifest missing %q: %s", required, body)
		}
	}
}

// Test: manifest uses stable pack and omission enums.
// Requirements: M7-R10.
func TestManifestUsesStablePackAndOmissionEnums(t *testing.T) {
	t.Parallel()

	if PackSelectedText != "selected_text" || OmissionReasonBudget != "budget" || RAGModeNone != "none" {
		t.Fatalf("enum drift: pack=%q omission=%q rag=%q", PackSelectedText, OmissionReasonBudget, RAGModeNone)
	}
}
