// BDD Scenario: 7.1.1 - Preview minimal Line Polish context
// Requirements: M7-R02, M7-R07
// Test purpose: Context policy is enforced before budgeting and cannot leak forbidden packs.

package contextpack

import (
	"errors"
	"testing"
)

func linePolishPolicy() Policy {
	return Policy{
		Required:  []Pack{PackSelectedText, PackStyleSheet},
		Optional:  []Pack{PackOutlineNeighbor},
		Forbidden: []Pack{PackActiveCodex, PackCurrentScene, PackCurrentChapter},
	}
}

func sceneRewritePolicy() Policy {
	return Policy{
		Required:  []Pack{PackCurrentScene, PackStyleSheet, PackActiveCodex},
		Optional:  []Pack{PackOutlineNeighbor},
		Forbidden: []Pack{PackSelectedText, PackCurrentChapter},
	}
}

// Test: builder includes every required pack.
// Requirements: M7-R07.
func TestBuilderIncludesEveryRequiredPack(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	packet, manifest, err := builder.Build(BuildRequest{
		Scope:   ScopeScene,
		Policy:  sceneRewritePolicy(),
		Budget:  Budget{MaxInputEstimatedTokens: 12000, ReservedOutputEstimatedTokens: 1000},
		RAGMode: RAGModeTimelineAware,
		Material: Material{
			Scope: ScopeScene, Style: StyleSheet{ID: "precise_editor", SystemPrompt: "Edit"},
			SceneMarkdown: "Ann waited.", SceneOrder: []SceneOrderRef{{ID: "scn_0123456789abcdef0123"}},
			CodexCandidates: []CodexEntryCandidate{
				{EntryID: "char_0123456789abcdef0123", EntryType: "character", Name: "Ann"},
			},
		},
		Estimator: ByteEstimator{},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	scenePacket, ok := packet.(ScenePacket)
	if !ok {
		t.Fatalf("packet type = %T, want ScenePacket", packet)
	}
	if scenePacket.SceneMarkdown == "" || scenePacket.Style.ID == "" || len(scenePacket.ActiveCodex) != 1 {
		t.Fatalf("scene packet = %#v", scenePacket)
	}
	for _, pack := range []Pack{PackCurrentScene, PackStyleSheet, PackActiveCodex} {
		if !containsUsedPack(manifest.PacksUsed, pack) {
			t.Fatalf("packs_used = %#v, missing %q", manifest.PacksUsed, pack)
		}
	}
}

// Test: builder rejects forbidden and undeclared optional packs.
// Requirements: M7-R07.
func TestBuilderRejectsForbiddenAndUndeclaredOptionalPacks(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	policy := linePolishPolicy()
	policy.Required = append(policy.Required, PackActiveCodex)
	_, _, err := builder.Build(BuildRequest{
		Scope: ScopeSelection, Policy: policy, Budget: Budget{MaxInputEstimatedTokens: 4096, ReservedOutputEstimatedTokens: 512},
		RAGMode:   RAGModeNone,
		Material:  Material{Scope: ScopeSelection, Style: StyleSheet{ID: "precise_editor"}, SelectionText: "Alpha"},
		Estimator: ByteEstimator{},
	})
	if !errors.Is(err, ErrForbiddenPack) {
		t.Fatalf("Build() error = %v, want %v", err, ErrForbiddenPack)
	}
}

// Test: Line Polish uses selected text and style only.
// Requirements: M7-R02.
func TestBuilderLinePolishUsesSelectedTextAndStyleOnly(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	packet, manifest, err := builder.Build(BuildRequest{
		Scope:   ScopeSelection,
		Policy:  linePolishPolicy(),
		Budget:  Budget{MaxInputEstimatedTokens: 4096, ReservedOutputEstimatedTokens: 512},
		RAGMode: RAGModeNone,
		Material: Material{
			Scope: ScopeSelection, Style: StyleSheet{ID: "precise_editor", SystemPrompt: "Edit"},
			SelectionText: "Alpha", SceneMarkdown: "Hidden scene", CodexCandidates: []CodexEntryCandidate{{EntryID: "char_0123456789abcdef0123", Name: "Ann"}},
		},
		Estimator: ByteEstimator{},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	selectionPacket := packet.(SelectionPacket)
	if selectionPacket.SelectedText != "Alpha" || selectionPacket.Style.ID != "precise_editor" {
		t.Fatalf("packet = %#v", selectionPacket)
	}
	if len(manifest.PacksUsed) != 2 || manifest.RAGMode != RAGModeNone || len(manifest.ActiveCodex) != 0 {
		t.Fatalf("manifest = %#v", manifest)
	}
}

// Test: builder does not alias agent policy slices.
// Requirements: M7-R07.
func TestBuilderDoesNotAliasAgentPolicySlices(t *testing.T) {
	t.Parallel()

	required := []Pack{PackSelectedText, PackStyleSheet}
	optional := []Pack{PackOutlineNeighbor}
	forbidden := []Pack{PackActiveCodex}
	policy := Policy{Required: required, Optional: optional, Forbidden: forbidden}
	builder := NewBuilder()
	_, _, err := builder.Build(BuildRequest{
		Scope: ScopeSelection, Policy: policy, Budget: Budget{MaxInputEstimatedTokens: 4096, ReservedOutputEstimatedTokens: 512},
		RAGMode:   RAGModeNone,
		Material:  Material{Scope: ScopeSelection, Style: StyleSheet{ID: "precise_editor"}, SelectionText: "Alpha"},
		Estimator: ByteEstimator{},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	required[0] = PackCurrentScene
	optional[0] = PackCurrentChapter
	forbidden[0] = PackSelectedText
	_, _, err = builder.Build(BuildRequest{
		Scope: ScopeSelection, Policy: policy, Budget: Budget{MaxInputEstimatedTokens: 4096, ReservedOutputEstimatedTokens: 512},
		RAGMode:   RAGModeNone,
		Material:  Material{Scope: ScopeSelection, Style: StyleSheet{ID: "precise_editor"}, SelectionText: "Alpha"},
		Estimator: ByteEstimator{},
	})
	if err != nil {
		t.Fatalf("second Build() error = %v", err)
	}
}

func containsUsedPack(values []Pack, pack Pack) bool {
	for _, value := range values {
		if value == pack {
			return true
		}
	}
	return false
}
