package story

import (
	"context"
	"testing"
	"time"

	"storywork/internal/project"
)

// BDD trace:
//   - Requirement: M2-R07, M2-R14.
//   - Scenario: 2.3.3 — Overlapping app mutations.
//   - Test purpose: verify a scene save shares the structural mutation lock so
//     a reorder cannot overlap the save's read-modify-write cycle.
func TestSceneSavesSerializeWithStructuralMutations(t *testing.T) {
	t.Parallel()

	firstLoadEntered := make(chan struct{})
	releaseFirstLoad := make(chan struct{})
	files := &fakeFileStore{
		exists:      map[string]bool{},
		loadOutline: mustSceneOutline(t),
		scene: SceneDocument{
			ID:          "scn_00000000000000000001",
			ChapterID:   "ch_00000000000000000001",
			Title:       "The Duel",
			FrontMatter: SceneFrontMatter{Status: "draft"},
			Markdown:    "Original.\n",
			Revision:    "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			Canonical:   []byte("old"),
		},
		sceneBytes: []byte("new"),
	}
	files.loadHook = func(call int) {
		if call == 1 {
			close(firstLoadEntered)
			<-releaseFirstLoad
		}
	}
	service := NewService(
		&fakeSession{current: project.Project{Path: "/tmp/story"}, ok: true},
		files,
		&fakeGitStore{clean: true},
		&fakeIndexStore{},
		&fakeIDGenerator{ids: []string{"arc_00000000000000000002"}},
	)

	saveDone := make(chan error, 1)
	go func() {
		_, err := service.SaveScene(context.Background(), "scn_00000000000000000001", SaveSceneRequest{
			Title:            "The Duel",
			FrontMatter:      SceneFrontMatter{Status: "revised"},
			Markdown:         "Changed.\n",
			ExpectedRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		})
		saveDone <- err
	}()
	<-firstLoadEntered

	reorderDone := make(chan error, 1)
	go func() {
		_, err := service.CreateArc(context.Background(), "Act Two")
		reorderDone <- err
	}()

	select {
	case err := <-reorderDone:
		t.Fatalf("structural mutation completed before save was released: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	close(releaseFirstLoad)
	if err := <-saveDone; err != nil {
		t.Fatalf("SaveScene() error = %v", err)
	}
	if err := <-reorderDone; err != nil {
		t.Fatalf("CreateArc() error = %v", err)
	}
}
