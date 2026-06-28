package story

import (
	"context"
	"errors"
	"testing"

	"storywork/internal/project"
)

// BDD trace:
//   - Requirement: M2-R06, M2-R11.
//   - Scenario: 2.2.3 — No-op save.
//   - Test purpose: verify byte-identical scene saves are rejected before index
//     rebuild or checkpoint work, leaving canonical and Git history unchanged.
func TestSaveSceneRejectsNoOpRequests(t *testing.T) {
	t.Parallel()

	files := &fakeFileStore{
		loadOutline: mustSceneOutline(t),
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
			Canonical: []byte("same"),
		},
		sceneBytes: []byte("same"),
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

	_, err := service.SaveScene(context.Background(), "scn_00000000000000000001", SaveSceneRequest{
		Title: "The Duel",
		FrontMatter: SceneFrontMatter{
			POV:           "Luke",
			Status:        "draft",
			ExcludeFromAI: false,
		},
		Markdown:         "Original.\n",
		ExpectedRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	})
	if !errors.Is(err, ErrNoSceneChanges) {
		t.Fatalf("SaveScene() error = %v, want ErrNoSceneChanges", err)
	}
	if files.writeCalls != 0 || index.rebuildCalls != 0 || git.commitCalls != 0 {
		t.Fatalf("write/rebuild/commit calls = %d/%d/%d, want 0/0/0", files.writeCalls, index.rebuildCalls, git.commitCalls)
	}
}
