package contextpack

// BDD Scenario: 7.2.2 - Exclude irrelevant Codex entries
// Requirements: M7-R07, M7-R08
// Test purpose: Deterministic budgeting reserves output space, never truncates required text, and reports omissions.

import (
	"errors"
	"strings"
	"testing"
)

// Test: budget reserves output before input material.
// Requirements: M7-R08.
func TestBudgetReservesOutputBeforeInputMaterial(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	_, manifest, err := builder.Build(BuildRequest{
		Scope: ScopeSelection, Policy: linePolishPolicy(), RAGMode: RAGModeNone,
		Budget:    Budget{MaxInputEstimatedTokens: 20, ReservedOutputEstimatedTokens: 15},
		Material:  Material{Scope: ScopeSelection, Style: StyleSheet{ID: "precise_editor", SystemPrompt: "0123456789"}, SelectionText: "ABCDEFGHIJ"},
		Estimator: ByteEstimator{},
	})
	if !errors.Is(err, ErrBudgetOverflow) {
		t.Fatalf("Build() error = %v, want %v", err, ErrBudgetOverflow)
	}
	_ = manifest
}

// Test: budget never truncates required target text.
// Requirements: M7-R07.
func TestBudgetNeverTruncatesRequiredTargetText(t *testing.T) {
	t.Parallel()

	longScene := strings.Repeat("Ann waited. ", 200)
	builder := NewBuilder()
	_, _, err := builder.Build(BuildRequest{
		Scope: ScopeScene, Policy: sceneRewritePolicy(), RAGMode: RAGModeTimelineAware,
		Budget: Budget{MaxInputEstimatedTokens: 100, ReservedOutputEstimatedTokens: 10},
		Material: Material{
			Scope: ScopeScene, Style: StyleSheet{ID: "precise_editor", SystemPrompt: "Edit"},
			SceneMarkdown: longScene, SceneOrder: []SceneOrderRef{{ID: "scn_0123456789abcdef0123"}},
			CodexCandidates: []CodexEntryCandidate{{EntryID: "char_0123456789abcdef0123", EntryType: "character", Name: "Ann"}},
		},
		Estimator: ByteEstimator{},
	})
	if !errors.Is(err, ErrBudgetOverflow) {
		t.Fatalf("Build() error = %v, want overflow instead of truncation", err)
	}
}

// Test: budget includes relevant Codex in rank order.
// Requirements: M7-R06, M7-R08.
func TestBudgetIncludesRelevantCodexInRankOrder(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	packet, manifest, err := builder.Build(BuildRequest{
		Scope: ScopeScene, Policy: sceneRewritePolicy(), RAGMode: RAGModeTimelineAware,
		Budget: Budget{MaxInputEstimatedTokens: 500, ReservedOutputEstimatedTokens: 50},
		Material: Material{
			Scope: ScopeScene, Style: StyleSheet{ID: "precise_editor", SystemPrompt: "Edit"},
			SceneMarkdown: "Ann met Mira.", SceneOrder: []SceneOrderRef{{ID: "scn_0123456789abcdef0123"}},
			CodexCandidates: []CodexEntryCandidate{
				{EntryID: "char_bbbbbbbbbbbbbbbbbbbb", EntryType: "character", Name: "Mira"},
				{EntryID: "char_aaaaaaaaaaaaaaaaaaaa", EntryType: "character", Name: "Ann"},
			},
		},
		Estimator: ByteEstimator{},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	scenePacket := packet.(ScenePacket)
	if len(scenePacket.ActiveCodex) != 2 || scenePacket.ActiveCodex[0].EntryID != "char_aaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("active codex order = %#v", scenePacket.ActiveCodex)
	}
	if manifest.ActiveCodex[0].EntryID != "char_aaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("manifest active codex = %#v", manifest.ActiveCodex)
	}
}

// Test: budget trims optional neighbors nearest-first.
// Requirements: M7-R08.
func TestBudgetTrimsOptionalNeighborsNearestFirst(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	packet, manifest, err := builder.Build(BuildRequest{
		Scope: ScopeScene, Policy: sceneRewritePolicy(), RAGMode: RAGModeTimelineAware,
		Budget: Budget{MaxInputEstimatedTokens: 180, ReservedOutputEstimatedTokens: 20},
		Material: Material{
			Scope: ScopeScene, Style: StyleSheet{ID: "precise_editor", SystemPrompt: "Edit"},
			SceneMarkdown: "Ann.", SceneOrder: []SceneOrderRef{{ID: "scn_0123456789abcdef0123"}},
			CodexCandidates: []CodexEntryCandidate{{EntryID: "char_0123456789abcdef0123", EntryType: "character", Name: "Ann"}},
			OutlineNeighbors: []OutlineNeighbor{
				{Kind: "scene", ID: "scn_nearestaaaaaaaaaaaa", Text: strings.Repeat("n", 40)},
				{Kind: "chapter", ID: "ch_fartheraaaaaaaaaaaaa", Text: strings.Repeat("f", 80)},
			},
		},
		Estimator: ByteEstimator{},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	scenePacket := packet.(ScenePacket)
	if len(scenePacket.OutlineNeighbors) != 1 || scenePacket.OutlineNeighbors[0].ID != "scn_nearestaaaaaaaaaaaa" {
		t.Fatalf("outline neighbors = %#v", scenePacket.OutlineNeighbors)
	}
	if !containsOmittedPack(manifest.PacksOmitted, PackOutlineNeighbor) {
		t.Fatalf("packs_omitted = %#v", manifest.PacksOmitted)
	}
}

// Test: budget reports deterministic omissions.
// Requirements: M7-R10.
func TestBudgetReportsDeterministicOmissions(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	_, manifest, err := builder.Build(BuildRequest{
		Scope: ScopeScene, Policy: sceneRewritePolicy(), RAGMode: RAGModeTimelineAware,
		Budget: Budget{MaxInputEstimatedTokens: 120, ReservedOutputEstimatedTokens: 20},
		Material: Material{
			Scope: ScopeScene, Style: StyleSheet{ID: "precise_editor", SystemPrompt: "Edit"},
			SceneMarkdown: "Ann.", SceneOrder: []SceneOrderRef{{ID: "scn_0123456789abcdef0123"}},
			CodexCandidates:  []CodexEntryCandidate{{EntryID: "char_0123456789abcdef0123", EntryType: "character", Name: "Ann"}},
			OutlineNeighbors: []OutlineNeighbor{{Kind: "scene", ID: "scn_0123456789abcdef0124", Text: strings.Repeat("x", 80)}},
		},
		Estimator: ByteEstimator{},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(manifest.PacksOmitted) != 1 || manifest.PacksOmitted[0].Reason != OmissionReasonBudget {
		t.Fatalf("packs_omitted = %#v", manifest.PacksOmitted)
	}
}

// Test: budget uses the lower agent or provider limit.
// Requirements: M7-R08.
func TestBudgetUsesLowerAgentOrProviderLimit(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	_, _, err := builder.Build(BuildRequest{
		Scope: ScopeSelection, Policy: linePolishPolicy(), RAGMode: RAGModeNone,
		Budget:    Budget{MaxInputEstimatedTokens: 500, ReservedOutputEstimatedTokens: 50, ProviderMaxInputTokens: 40},
		Material:  Material{Scope: ScopeSelection, Style: StyleSheet{ID: "precise_editor", SystemPrompt: "0123456789"}, SelectionText: "ABCDEFGHIJKLMNOPQRSTUVWXYZ"},
		Estimator: ByteEstimator{},
	})
	if !errors.Is(err, ErrBudgetOverflow) {
		t.Fatalf("Build() error = %v, want provider limit overflow", err)
	}
}

// Test: budget overflow fails before provider use.
// Requirements: M7-R07, M7-R08.
func TestBudgetOverflowFailsBeforeProviderUse(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	_, _, err := builder.Build(BuildRequest{
		Scope: ScopeSelection, Policy: linePolishPolicy(), RAGMode: RAGModeNone,
		Budget:    Budget{MaxInputEstimatedTokens: 5, ReservedOutputEstimatedTokens: 4},
		Material:  Material{Scope: ScopeSelection, Style: StyleSheet{ID: "precise_editor"}, SelectionText: "Too long"},
		Estimator: ByteEstimator{},
	})
	if !errors.Is(err, ErrBudgetOverflow) {
		t.Fatalf("Build() error = %v, want %v", err, ErrBudgetOverflow)
	}
}

func containsOmittedPack(omissions []PackOmission, pack Pack) bool {
	for _, omission := range omissions {
		if omission.Pack == pack {
			return true
		}
	}
	return false
}
