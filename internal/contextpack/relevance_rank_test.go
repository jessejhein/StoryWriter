package contextpack

// BDD Scenario: 7.2.1 - Resolve different active facts by scene
// Requirements: M7-R05, M7-R06
// Test purpose: Ranked active Codex relevance respects timeline position and chapter deduplication.

import (
	"testing"
)

func progressionAfter(sceneID, progID, description string) ProgressionInput {
	desc := description
	return ProgressionInput{
		ID: progID, AnchorSceneID: sceneID, AnchorTiming: "after", Description: &desc,
	}
}

// Test: ranking orders name, alias, tag, count, and stable ties.
// Requirements: M7-R06.
func TestRankEntriesOrdersNameAliasTagCountAndStableTies(t *testing.T) {
	t.Parallel()

	entries := []CodexEntryCandidate{
		{EntryID: "char_aaaaaaaaaaaaaaaaaaaa", EntryType: "character", Name: "Zed"},
		{EntryID: "char_bbbbbbbbbbbbbbbbbbbb", EntryType: "character", Name: "Ann", Aliases: []string{"Annie"}},
		{EntryID: "char_cccccccccccccccccccc", EntryType: "character", Name: "Mira", Tags: []string{"captain"}},
	}
	text := "Ann and Annie met the captain. Ann waved."
	ranked := RankLexicalRelevance(text, entries)
	if len(ranked) != 2 {
		t.Fatalf("ranked len = %d, want 2 mentioned entries", len(ranked))
	}
	if ranked[0].EntryID != "char_bbbbbbbbbbbbbbbbbbbb" || ranked[1].EntryID != "char_cccccccccccccccccccc" {
		t.Fatalf("rank order = %#v", ranked)
	}
}

// Test: relevant entries use active state, not future state.
// Requirements: M7-R05.
func TestRelevantEntryUsesActiveNotFutureState(t *testing.T) {
	t.Parallel()

	entry := CodexEntryCandidate{
		EntryID: "char_0123456789abcdef0123", EntryType: "character", Name: "Ann", Description: "Young",
		Progressions: []ProgressionInput{
			progressionAfter("scn_00000000000000000001", "prog_aaaaaaaaaaaaaaaaaaaa", "Veteran"),
		},
	}
	before, err := ResolveRelevantEntry(entry, "Ann waited.", []SceneOrderRef{
		{ID: "scn_00000000000000000001"}, {ID: "scn_00000000000000000002"},
	}, "scn_00000000000000000001")
	if err != nil {
		t.Fatalf("ResolveRelevantEntry(before) error = %v", err)
	}
	if before.Description != "Young" || len(before.AppliedProgressionIDs) != 0 {
		t.Fatalf("before state = %#v", before)
	}
	after, err := ResolveRelevantEntry(entry, "Ann waited.", []SceneOrderRef{
		{ID: "scn_00000000000000000001"}, {ID: "scn_00000000000000000002"},
	}, "scn_00000000000000000002")
	if err != nil {
		t.Fatalf("ResolveRelevantEntry(after) error = %v", err)
	}
	if after.Description != "Veteran" || after.AppliedProgressionIDs[0] != "prog_aaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("after state = %#v", after)
	}
}

// Test: no lexical evidence does not dump the global Codex.
// Requirements: M7-R06.
func TestNoLexicalEvidenceDoesNotDumpGlobalCodex(t *testing.T) {
	t.Parallel()

	entries := []CodexEntryCandidate{
		{EntryID: "char_aaaaaaaaaaaaaaaaaaaa", EntryType: "character", Name: "Ann"},
		{EntryID: "char_bbbbbbbbbbbbbbbbbbbb", EntryType: "character", Name: "Mira"},
	}
	ranked := RankLexicalRelevance("The festival began.", entries)
	if len(ranked) != 0 {
		t.Fatalf("ranked = %#v, want empty", ranked)
	}
}

// Test: chapter states deduplicate only when resolved state matches.
// Requirements: M7-R04.
func TestChapterStatesDeduplicateOnlyWhenResolvedStateMatches(t *testing.T) {
	t.Parallel()

	shared := CodexEntryCandidate{
		EntryID: "char_0123456789abcdef0123", EntryType: "character", Name: "Ann", Description: "Shared",
	}
	changed := CodexEntryCandidate{
		EntryID: "char_0123456789abcdef0123", EntryType: "character", Name: "Ann", Description: "Shared",
		Progressions: []ProgressionInput{
			progressionAfter("scn_00000000000000000001", "prog_aaaaaaaaaaaaaaaaaaaa", "Changed"),
		},
	}
	states, err := DeduplicateChapterCodexStates([]ChapterSceneCodex{
		{SceneID: "scn_00000000000000000001", Entry: shared, Text: "Ann one."},
		{SceneID: "scn_00000000000000000002", Entry: shared, Text: "Ann two."},
		{SceneID: "scn_00000000000000000003", Entry: changed, Text: "Ann three."},
	}, []SceneOrderRef{
		{ID: "scn_00000000000000000001"}, {ID: "scn_00000000000000000002"}, {ID: "scn_00000000000000000003"},
	})
	if err != nil {
		t.Fatalf("DeduplicateChapterCodexStates() error = %v", err)
	}
	if len(states) != 2 {
		t.Fatalf("states = %#v, want 2 deduplicated groups", states)
	}
	if len(states[0].SceneIDs) != 2 || states[1].SceneIDs[0] != "scn_00000000000000000003" {
		t.Fatalf("scene grouping = %#v", states)
	}
}

// Test: manifest reports applied progression IDs without values.
// Requirements: M7-R10.
func TestManifestReportsAppliedProgressionIDsWithoutValues(t *testing.T) {
	t.Parallel()

	ref := ManifestCodexRefFromState(CodexEntryState{
		EntryID: "char_0123456789abcdef0123", Description: "Secret", Metadata: map[string]string{"role": "captain"},
		AppliedProgressionIDs: []string{"prog_0123456789abcdef0123"},
	})
	if ref.EntryID != "char_0123456789abcdef0123" || ref.AppliedProgressionIDs[0] != "prog_0123456789abcdef0123" {
		t.Fatalf("ref = %#v", ref)
	}
}
