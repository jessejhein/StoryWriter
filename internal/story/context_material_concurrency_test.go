// BDD Scenario: 7.2.1 - Resolve different active facts by scene
// Requirements: M7-R18
// Test purpose: Context material reads wait for story mutations and never observe partial state.

package story

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"storywork/internal/mutation"
	"storywork/internal/project"
)

// Test: context material read waits while a story mutation holds the write lock.
// Requirements: M7-R18.
func TestContextMaterialReadWaitsForStoryMutation(t *testing.T) {
	t.Parallel()

	coordinator := mutation.NewCoordinator()
	outline := mustMultiSceneOutline(t)
	scene := SceneDocument{
		ID: "scn_00000000000000000001", ChapterID: "ch_00000000000000000001",
		Markdown: "Text.\n", Revision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		FrontMatter: SceneFrontMatter{Status: "draft"},
	}
	scenes := mustAllSceneDocuments(t, outline)
	scenes[scene.ID] = scene
	files := &fakeFileStore{loadOutline: outline, scenes: scenes}
	service := NewService(
		&fakeSession{current: project.Project{Path: t.TempDir()}, ok: true},
		files,
		&fakeGitStore{clean: true},
		&fakeIndexStore{},
		&fakeIDGenerator{ids: []string{"arc_00000000000000000002"}},
	).WithMutationCoordinator(coordinator)

	coordinator.Lock()
	done := make(chan error, 1)
	go func() {
		_, err := service.LoadSceneMaterial(context.Background(), scene.ID, scene.Revision)
		done <- err
	}()

	select {
	case err := <-done:
		t.Fatalf("LoadSceneMaterial() completed while mutation held lock: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	coordinator.Unlock()
	if err := <-done; err != nil {
		t.Fatalf("LoadSceneMaterial() error = %v", err)
	}
}

// Test: context material never observes a partially updated in-memory snapshot.
// Requirements: M7-R18.
func TestContextMaterialNeverObservesPartialMutation(t *testing.T) {
	t.Parallel()

	outline := mustMultiSceneOutline(t)
	scene := SceneDocument{
		ID: "scn_00000000000000000001", ChapterID: "ch_00000000000000000001",
		Markdown: "Before.\n", Revision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		FrontMatter: SceneFrontMatter{Status: "draft"},
	}
	scenes := mustAllSceneDocuments(t, outline)
	scenes[scene.ID] = scene
	files := &fakeFileStore{loadOutline: outline, scenes: scenes}
	var loadCount atomic.Int32
	files.loadSceneCallsHook = func() {
		if loadCount.Add(1) == 1 {
			files.scenes["scn_00000000000000000002"] = SceneDocument{
				ID: "scn_00000000000000000002", ChapterID: "ch_00000000000000000001",
				Markdown: "Mutated neighbor.\n",
				Revision: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				FrontMatter: SceneFrontMatter{Status: "draft"},
			}
		}
	}
	service := newContextMaterialService(t, files)

	result, err := service.LoadSceneMaterial(context.Background(), scene.ID, scene.Revision)
	if err != nil {
		t.Fatalf("LoadSceneMaterial() error = %v", err)
	}
	if result.Material.SceneMarkdown != "Before.\n" {
		t.Fatalf("observed markdown = %q, want coherent Before snapshot", result.Material.SceneMarkdown)
	}
}