// BDD Scenario: 7.1.2 - Run with the previewed scope
// Requirements: M7-R01
// Test purpose: Tagged action targets validate exactly one scope payload.

package action

import (
	"errors"
	"testing"

	"storywork/internal/agent"
	"storywork/internal/contextpack"
	"storywork/internal/story"
)

// Test: selection target requires revision, byte range, and exact text.
// Requirements: M7-R01.
func TestValidateSelectionTargetRequiresRevisionRangeAndText(t *testing.T) {
	t.Parallel()

	valid := SelectionTarget{
		SceneID:       "scn_0123456789abcdef0123",
		SceneRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		StartByte:     0, EndByte: 5, SelectedText: "Alpha",
	}
	if err := ValidateSelectionTarget(valid); err != nil {
		t.Fatalf("ValidateSelectionTarget() error = %v", err)
	}
	for _, invalid := range []SelectionTarget{
		{SceneRevision: valid.SceneRevision, StartByte: 0, EndByte: 5, SelectedText: "Alpha"},
		{SceneID: valid.SceneID, StartByte: 0, EndByte: 5, SelectedText: "Alpha"},
		{SceneID: valid.SceneID, SceneRevision: "bad", StartByte: 0, EndByte: 5, SelectedText: "Alpha"},
		{SceneID: valid.SceneID, SceneRevision: valid.SceneRevision, StartByte: -1, EndByte: 5, SelectedText: "Alpha"},
		{SceneID: valid.SceneID, SceneRevision: valid.SceneRevision, StartByte: 0, SelectedText: "Alpha"},
	} {
		if err := ValidateSelectionTarget(invalid); err == nil {
			t.Fatalf("ValidateSelectionTarget(%#v) error = nil, want failure", invalid)
		}
	}
}

// Test: scene target requires scene ID and revision.
// Requirements: M7-R01.
func TestValidateSceneTargetRequiresSceneAndRevision(t *testing.T) {
	t.Parallel()

	valid := SceneTarget{
		SceneID:       "scn_0123456789abcdef0123",
		SceneRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	if err := ValidateSceneTarget(valid); err != nil {
		t.Fatalf("ValidateSceneTarget() error = %v", err)
	}
	if err := ValidateSceneTarget(SceneTarget{}); err == nil {
		t.Fatal("empty scene target should fail")
	}
}

// Test: chapter target requires chapter ID and fingerprint.
// Requirements: M7-R01.
func TestValidateChapterTargetRequiresChapterAndFingerprint(t *testing.T) {
	t.Parallel()

	valid := ChapterReviewTarget{
		ChapterID:   "ch_0123456789abcdef0123",
		Fingerprint: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}
	if err := ValidateChapterReviewTarget(valid); err != nil {
		t.Fatalf("ValidateChapterReviewTarget() error = %v", err)
	}
	if err := ValidateChapterReviewTarget(ChapterReviewTarget{ChapterID: valid.ChapterID}); err == nil {
		t.Fatal("missing fingerprint should fail")
	}
}

// Test: tagged target rejects mixed or mismatched scope payloads.
// Requirements: M7-R01.
func TestValidateTargetRejectsMixedOrMismatchedScopePayloads(t *testing.T) {
	t.Parallel()

	if err := ValidateTaggedTarget(TaggedTarget{
		Scope:     contextpack.ScopeSelection,
		Selection: &SelectionTarget{SceneID: "scn_0123456789abcdef0123", SceneRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", StartByte: 0, EndByte: 1, SelectedText: "A"},
		Scene:     &SceneTarget{SceneID: "scn_0123456789abcdef0123", SceneRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
	}); err == nil {
		t.Fatal("mixed payloads should fail")
	}
	if err := ValidateTaggedTarget(TaggedTarget{
		Scope:     contextpack.ScopeScene,
		Selection: &SelectionTarget{SceneID: "scn_0123456789abcdef0123", SceneRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", StartByte: 0, EndByte: 1, SelectedText: "A"},
	}); err == nil {
		t.Fatal("scope/payload mismatch should fail")
	}
}

// Test: legacy selection request normalizes to a tagged selection target.
// Requirements: M7-R01.
func TestLegacySelectionRequestNormalizesToTaggedTarget(t *testing.T) {
	t.Parallel()

	target, err := NormalizeLegacyRunRequest(RunRequest{
		AgentID: "line_polish", StyleID: "precise_editor",
		Surface: agent.SurfaceEditor, InputScope: agent.InputScopeSelection,
		SceneID:       "scn_0123456789abcdef0123",
		SceneRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Selection:     Selection{StartByte: 0, EndByte: 5, Text: "Alpha"},
	})
	if err != nil {
		t.Fatalf("NormalizeLegacyRunRequest() error = %v", err)
	}
	if target.Scope != contextpack.ScopeSelection || target.Selection == nil {
		t.Fatalf("target = %#v", target)
	}
	if target.Selection.SelectedText != "Alpha" {
		t.Fatalf("selected text = %q", target.Selection.SelectedText)
	}
}

// Test: tagged run request validates agent and style IDs.
// Requirements: M7-R01.
func TestValidateTaggedRunRequestRejectsInvalidIDs(t *testing.T) {
	t.Parallel()

	request := TaggedRunRequest{
		AgentID: "line_polish", StyleID: "precise_editor",
		Target: TaggedTarget{
			Scope: contextpack.ScopeSelection,
			Selection: &SelectionTarget{
				SceneID:       "scn_0123456789abcdef0123",
				SceneRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				StartByte:     0, EndByte: 5, SelectedText: "Alpha",
			},
		},
	}
	if err := ValidateTaggedRunRequest(request); err != nil {
		t.Fatalf("ValidateTaggedRunRequest() error = %v", err)
	}
	request.AgentID = "Bad Agent"
	if err := ValidateTaggedRunRequest(request); !errors.Is(err, ErrInvalidRunRequest) {
		t.Fatalf("invalid agent error = %v", err)
	}
	if err := story.ValidateSceneID(""); err == nil {
		t.Fatal("expected scene validation helper")
	}
}
