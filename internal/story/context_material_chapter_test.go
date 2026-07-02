// BDD Scenario: 7.3.1 - Build bounded chapter-review context
// Requirements: M7-R04, M7-R18
// Test purpose: Chapter material and fingerprint reject stale targets without inconsistent reads.

package story

import (
	"context"
	"testing"

	"storywork/internal/codex"
	"storywork/internal/contextpack"
)

// Test: chapter material returns ordered scene bodies and chapter neighbors.
// Requirements: M7-R04.
func TestLoadChapterMaterialReturnsOrderedScenesAndNeighbors(t *testing.T) {
	t.Parallel()

	outline := mustMultiSceneOutline(t)
	scenes := mustAllSceneDocuments(t, outline)
	scenes["scn_00000000000000000001"] = SceneDocument{
		ID: "scn_00000000000000000001", ChapterID: "ch_00000000000000000001",
		Title: "Opening", Markdown: "First.\n",
		Revision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		FrontMatter: SceneFrontMatter{Status: "draft"},
	}
	scenes["scn_00000000000000000002"] = SceneDocument{
		ID: "scn_00000000000000000002", ChapterID: "ch_00000000000000000001",
		Title: "Departure", Markdown: "Second.\n",
		Revision: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		FrontMatter: SceneFrontMatter{Status: "draft"},
	}
	scenes["scn_00000000000000000003"] = SceneDocument{
		ID: "scn_00000000000000000003", ChapterID: "ch_00000000000000000002",
		Title: "Closing", Markdown: "Third.\n",
		Revision: "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		FrontMatter: SceneFrontMatter{Status: "draft"},
	}
	files := &fakeFileStore{loadOutline: outline, scenes: scenes}
	service := newContextMaterialService(t, files)

	result, err := service.LoadChapterMaterial(context.Background(), "ch_00000000000000000001")
	if err != nil {
		t.Fatalf("LoadChapterMaterial() error = %v", err)
	}
	if result.Material.Scope != contextpack.ScopeChapterReview {
		t.Fatalf("scope = %q", result.Material.Scope)
	}
	if len(result.Material.ChapterScenes) != 2 {
		t.Fatalf("chapter scenes = %#v", result.Material.ChapterScenes)
	}
	if result.Material.ChapterScenes[0].Markdown != "First.\n" || result.Material.ChapterScenes[1].Markdown != "Second.\n" {
		t.Fatalf("ordered scenes = %#v", result.Material.ChapterScenes)
	}
	if len(result.Material.OutlineNeighbors) != 1 || result.Material.OutlineNeighbors[0].ID != "ch_00000000000000000002" {
		t.Fatalf("outline neighbors = %#v", result.Material.OutlineNeighbors)
	}
	if result.TargetRevision == "" {
		t.Fatal("fingerprint is empty")
	}
}

// Test: chapter fingerprint hashes outline bytes and ordered scene revisions.
// Requirements: M7-R04.
func TestChapterFingerprintUsesOutlineAndOrderedSceneRevisions(t *testing.T) {
	t.Parallel()

	outlineBytes := []byte("outline-bytes")
	pairs := []ChapterSceneRevision{
		{SceneID: "scn_00000000000000000001", Revision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		{SceneID: "scn_00000000000000000002", Revision: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
	}
	first := ComputeChapterFingerprint(outlineBytes, pairs)
	second := ComputeChapterFingerprint(outlineBytes, pairs)
	if first != second {
		t.Fatalf("fingerprints differ for same input: %q vs %q", first, second)
	}
	if first[:7] != "sha256:" {
		t.Fatalf("fingerprint = %q, want sha256 prefix", first)
	}
}

// Test: chapter fingerprint changes when order or scene revision changes.
// Requirements: M7-R04.
func TestChapterFingerprintChangesForOrderOrSceneChange(t *testing.T) {
	t.Parallel()

	outlineBytes := []byte("outline-bytes")
	base := []ChapterSceneRevision{
		{SceneID: "scn_00000000000000000001", Revision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		{SceneID: "scn_00000000000000000002", Revision: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
	}
	reordered := []ChapterSceneRevision{
		{SceneID: "scn_00000000000000000002", Revision: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
		{SceneID: "scn_00000000000000000001", Revision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
	}
	changedRevision := []ChapterSceneRevision{
		{SceneID: "scn_00000000000000000001", Revision: "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"},
		{SceneID: "scn_00000000000000000002", Revision: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
	}
	baseFP := ComputeChapterFingerprint(outlineBytes, base)
	if ComputeChapterFingerprint(outlineBytes, reordered) == baseFP {
		t.Fatal("reordered fingerprint should differ")
	}
	if ComputeChapterFingerprint(outlineBytes, changedRevision) == baseFP {
		t.Fatal("changed revision fingerprint should differ")
	}
	if ComputeChapterFingerprint(append([]byte("x"), outlineBytes...), base) == baseFP {
		t.Fatal("changed outline fingerprint should differ")
	}
}

// Test: chapter material loads codex candidates for chapter scenes.
// Requirements: M7-R05.
func TestLoadChapterMaterialIncludesCodexCandidates(t *testing.T) {
	t.Parallel()

	outline := mustMultiSceneOutline(t)
	scenes := mustAllSceneDocuments(t, outline)
	scenes["scn_00000000000000000001"] = SceneDocument{
		ID: "scn_00000000000000000001", ChapterID: "ch_00000000000000000001",
		Markdown: "Ann.\n", Revision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		FrontMatter: SceneFrontMatter{Status: "draft"},
	}
	files := &fakeFileStore{
		loadOutline: outline, scenes: scenes,
		codexEntries: []codex.Entry{{
			ID: "char_0123456789abcdef0123", Type: codex.TypeCharacter, Name: "Ann",
			Description: "Pilot.", Aliases: []string{}, Tags: []string{}, Metadata: map[string]string{},
		}},
		codexProgressions: codex.ProgressionDocument{EntryID: "char_0123456789abcdef0123"},
	}
	service := newContextMaterialService(t, files)

	result, err := service.LoadChapterMaterial(context.Background(), "ch_00000000000000000001")
	if err != nil {
		t.Fatalf("LoadChapterMaterial() error = %v", err)
	}
	if len(result.Material.CodexCandidates) != 1 {
		t.Fatalf("codex candidates = %#v", result.Material.CodexCandidates)
	}
}