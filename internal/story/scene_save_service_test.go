package story

import (
	"context"
	"testing"

	"storywork/internal/project"
)

// BDD trace:
//   - Requirement: M2-R04, M2-R06, M2-R07, M2-R09, M2-R10, M2-R11.
//   - Scenario: 2.2.1 — Save valid edits.
//   - Test purpose: verify a valid explicit scene save preserves immutable IDs,
//     writes one canonical file, rebuilds the index, commits exactly once, and
//     returns the reloaded saved scene.
func TestSaveScenePersistsCanonicalEditAndCheckpoint(t *testing.T) {
	t.Parallel()

	outline := mustSceneOutline(t)
	files := &fakeFileStore{
		loadOutline: outline,
		scene: SceneDocument{
			ID:        "scn_00000000000000000001",
			ChapterID: "ch_00000000000000000001",
			Title:     "The Duel",
			FrontMatter: SceneFrontMatter{
				POV:           "Luke",
				Status:        "draft",
				ExcludeFromAI: false,
			},
			Markdown:  "Original.\n",
			Revision:  "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			Canonical: []byte("old"),
		},
		sceneBytes: []byte("new"),
	}
	git := &fakeGitStore{clean: true}
	index := &fakeIndexStore{}
	service := NewService(
		&fakeSession{current: project.Project{Path: "/tmp/story"}, ok: true},
		files,
		git,
		index,
		&fakeIDGenerator{},
	)

	result, err := service.SaveScene(context.Background(), "scn_00000000000000000001", SaveSceneRequest{
		Title: "The Duel Revised",
		FrontMatter: SceneFrontMatter{
			POV:           "Luke",
			Status:        "revised",
			ExcludeFromAI: false,
		},
		Markdown:         "Revised.\n",
		ExpectedRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	})
	if err != nil {
		t.Fatalf("SaveScene() error = %v", err)
	}
	if got := string(files.writtenFiles["scenes/scn_00000000000000000001.md"]); got != "new" {
		t.Fatalf("written bytes = %q, want %q", got, "new")
	}
	if git.commitCalls != 1 || len(git.commitMessages) != 1 || git.commitMessages[0] != "Edit scene scn_00000000000000000001" {
		t.Fatalf("commit calls/messages = %d %#v", git.commitCalls, git.commitMessages)
	}
	if index.rebuildCalls != 1 {
		t.Fatalf("index rebuild calls = %d, want 1", index.rebuildCalls)
	}
	if files.loadSceneCalls != 1 {
		t.Fatalf("load scene calls = %d, want 1", files.loadSceneCalls)
	}
	if result.Title != "The Duel Revised" || result.Revision != ComputeRevision([]byte("new")) {
		t.Fatalf("result = %#v", result)
	}
	if result.ID != "scn_00000000000000000001" || result.ChapterID != "ch_00000000000000000001" {
		t.Fatalf("immutable IDs changed: %#v", result)
	}
}

func mustSceneOutline(t *testing.T) Outline {
	t.Helper()

	outline := NewOutline()
	var err error
	outline, err = AddArc(outline, "arc_00000000000000000001", "Act One")
	if err != nil {
		t.Fatalf("AddArc() error = %v", err)
	}
	outline, err = AddChapter(outline, "arc_00000000000000000001", "ch_00000000000000000001", "Arrival")
	if err != nil {
		t.Fatalf("AddChapter() error = %v", err)
	}
	outline, err = AddScene(outline, "ch_00000000000000000001", "scn_00000000000000000001", "The Duel")
	if err != nil {
		t.Fatalf("AddScene() error = %v", err)
	}
	return outline
}
