// BDD Scenario: 7.2.1 - Resolve different active facts by scene
// Requirements: M7-R03, M7-R18
// Test purpose: Selection and scene context material loads one coherent canonical snapshot.

package story

import (
	"context"
	"errors"
	"testing"

	"storywork/internal/codex"
	"storywork/internal/contextpack"
	"storywork/internal/project"
)

// Test: selection material returns the exact canonical UTF-8 byte range.
// Requirements: M7-R18.
func TestLoadSelectionMaterialReturnsExactCanonicalSelection(t *testing.T) {
	t.Parallel()

	outline := mustMultiSceneOutline(t)
	scene := SceneDocument{
		ID: "scn_00000000000000000001", ChapterID: "ch_00000000000000000001",
		Title: "Opening", Markdown: "Alpha beta gamma.\n",
		Revision:    "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		FrontMatter: SceneFrontMatter{Status: "draft"},
	}
	files := &fakeFileStore{loadOutline: outline, scenes: map[string]SceneDocument{scene.ID: scene}}
	service := newContextMaterialService(t, files)

	result, err := service.LoadSelectionMaterial(context.Background(), SelectionMaterialRequest{
		SceneID: scene.ID, SceneRevision: scene.Revision,
		StartByte: 6, EndByte: 10, SelectedText: "beta",
	})
	if err != nil {
		t.Fatalf("LoadSelectionMaterial() error = %v", err)
	}
	if result.Material.Scope != contextpack.ScopeSelection || result.Material.SelectionText != "beta" {
		t.Fatalf("material = %#v, want selection beta", result.Material)
	}
	if result.TargetRevision != scene.Revision {
		t.Fatalf("revision = %q, want %q", result.TargetRevision, scene.Revision)
	}
}

// Test: scene material returns scene prose, outline neighbors, and codex candidates.
// Requirements: M7-R03, M7-R05.
func TestLoadSceneMaterialReturnsSceneOutlineCodexAndProgressions(t *testing.T) {
	t.Parallel()

	outline := mustMultiSceneOutline(t)
	scenes := mustAllSceneDocuments(t, outline)
	scenes["scn_00000000000000000001"] = SceneDocument{
		ID: "scn_00000000000000000001", ChapterID: "ch_00000000000000000001",
		Title: "Opening", Markdown: "Ann arrives.\n",
		Revision:    "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		FrontMatter: SceneFrontMatter{Status: "draft"},
	}
	scenes["scn_00000000000000000002"] = SceneDocument{
		ID: "scn_00000000000000000002", ChapterID: "ch_00000000000000000001",
		Title: "Departure", Markdown: "Ann leaves.\n",
		Revision:    "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		FrontMatter: SceneFrontMatter{Status: "draft"},
	}
	files := &fakeFileStore{
		loadOutline: outline, scenes: scenes,
		codexEntries: []codex.Entry{{
			ID: "char_0123456789abcdef0123", Type: codex.TypeCharacter, Name: "Ann",
			Description: "Pilot.", Aliases: []string{}, Tags: []string{}, Metadata: map[string]string{},
		}},
		codexProgressions: codex.ProgressionDocument{
			EntryID: "char_0123456789abcdef0123",
			Progressions: []codex.Progression{{
				ID:      "prog_0123456789abcdef0123",
				Anchor:  codex.ProgressionAnchor{Type: "scene", ID: "scn_00000000000000000001", Timing: "after"},
				Changes: codex.ProgressionChange{Description: strPtr("Veteran.")},
			}},
		},
	}
	service := newContextMaterialService(t, files)

	result, err := service.LoadSceneMaterial(context.Background(), "scn_00000000000000000001", scenes["scn_00000000000000000001"].Revision)
	if err != nil {
		t.Fatalf("LoadSceneMaterial() error = %v", err)
	}
	if result.Material.SceneMarkdown != "Ann arrives.\n" {
		t.Fatalf("scene markdown = %q", result.Material.SceneMarkdown)
	}
	if len(result.Material.SceneOrder) != 3 {
		t.Fatalf("scene order = %#v", result.Material.SceneOrder)
	}
	if len(result.Material.CodexCandidates) != 1 || result.Material.CodexCandidates[0].EntryID != "char_0123456789abcdef0123" {
		t.Fatalf("codex candidates = %#v", result.Material.CodexCandidates)
	}
	if len(result.Material.CodexCandidates[0].Progressions) != 1 {
		t.Fatalf("progressions = %#v", result.Material.CodexCandidates[0].Progressions)
	}
	if len(result.Material.OutlineNeighbors) < 1 || result.Material.OutlineNeighbors[0].Kind != "scene" {
		t.Fatalf("outline neighbors = %#v", result.Material.OutlineNeighbors)
	}
}

// Test: scene material rejects missing, malformed, and exclude_from_ai scenes.
// Requirements: M7-R03.
func TestLoadSceneMaterialRejectsMissingMalformedAndExcludedScene(t *testing.T) {
	t.Parallel()

	outline := mustMultiSceneOutline(t)
	excluded := SceneDocument{
		ID: "scn_00000000000000000001", ChapterID: "ch_00000000000000000001",
		Title: "Opening", Markdown: "Hidden.\n",
		Revision:    "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		FrontMatter: SceneFrontMatter{Status: "draft", ExcludeFromAI: true},
	}
	service := newContextMaterialService(t, &fakeFileStore{loadOutline: outline, scenes: map[string]SceneDocument{excluded.ID: excluded}})

	if _, err := service.LoadSceneMaterial(context.Background(), "scn_ffffffffffffffffffff", excluded.Revision); !errors.Is(err, ErrSceneNotFound) {
		t.Fatalf("missing scene error = %v, want ErrSceneNotFound", err)
	}
	if _, err := service.LoadSceneMaterial(context.Background(), excluded.ID, excluded.Revision); !errors.Is(err, ErrExcludedFromAI) {
		t.Fatalf("excluded scene error = %v, want ErrExcludedFromAI", err)
	}

	malformed := &fakeFileStore{
		loadOutline:  outline,
		loadSceneErr: errors.New("decode scenes/scn_00000000000000000001.md: invalid markdown"),
	}
	service = newContextMaterialService(t, malformed)
	if _, err := service.LoadSceneMaterial(context.Background(), "scn_00000000000000000001", excluded.Revision); err == nil {
		t.Fatal("malformed scene error = nil, want failure")
	}
}

// Test: scene material returns the current canonical revision.
// Requirements: M7-R18.
func TestLoadSceneMaterialReturnsCurrentRevision(t *testing.T) {
	t.Parallel()

	outline := mustMultiSceneOutline(t)
	scenes := mustAllSceneDocuments(t, outline)
	scenes["scn_00000000000000000001"] = SceneDocument{
		ID: "scn_00000000000000000001", ChapterID: "ch_00000000000000000001",
		Title: "Opening", Markdown: "Text.\n",
		Revision:    "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		FrontMatter: SceneFrontMatter{Status: "draft"},
	}
	service := newContextMaterialService(t, &fakeFileStore{loadOutline: outline, scenes: scenes})

	target := scenes["scn_00000000000000000001"]
	result, err := service.LoadSceneMaterial(context.Background(), target.ID, target.Revision)
	if err != nil {
		t.Fatalf("LoadSceneMaterial() error = %v", err)
	}
	if result.TargetRevision != target.Revision {
		t.Fatalf("revision = %q, want %q", result.TargetRevision, target.Revision)
	}
}

func mustMultiSceneOutline(t *testing.T) Outline {
	t.Helper()
	outline := mustSceneOutline(t)
	var err error
	outline, err = AddScene(outline, "ch_00000000000000000001", "scn_00000000000000000002", "Departure")
	if err != nil {
		t.Fatalf("AddScene() error = %v", err)
	}
	outline, err = AddChapter(outline, "arc_00000000000000000001", "ch_00000000000000000002", "Aftermath")
	if err != nil {
		t.Fatalf("AddChapter() error = %v", err)
	}
	outline, err = AddScene(outline, "ch_00000000000000000002", "scn_00000000000000000003", "Closing")
	if err != nil {
		t.Fatalf("AddScene() error = %v", err)
	}
	return outline
}

func newContextMaterialService(t *testing.T, files *fakeFileStore) *Service {
	t.Helper()
	return NewService(
		&fakeSession{current: project.Project{Path: t.TempDir()}, ok: true},
		files,
		&fakeGitStore{clean: true},
		&fakeIndexStore{},
		&fakeIDGenerator{},
	)
}

func strPtr(value string) *string {
	return &value
}

func mustAllSceneDocuments(t *testing.T, outline Outline) map[string]SceneDocument {
	t.Helper()
	scenes := make(map[string]SceneDocument)
	for _, position := range flattenOutlinePositions(outline) {
		scenes[position.sceneID] = SceneDocument{
			ID: position.sceneID, ChapterID: position.chapterID,
			Markdown:    "Placeholder.\n",
			Revision:    "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
			FrontMatter: SceneFrontMatter{Status: "draft"},
		}
	}
	return scenes
}
