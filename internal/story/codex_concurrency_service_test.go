// BDD Scenario: 3.4.3 - Serialize overlapping mutations
// Requirements: M3-R13, M3-R14, M3-R17
// Test purpose: Codex, progression, and scene mutations serialize through the shared story mutation lock.
package story

import (
	"context"
	"testing"
	"time"

	"storywork/internal/codex"
	"storywork/internal/project"
)

func TestCodexMutationsShareTheSceneMutationLock(t *testing.T) {
	t.Parallel()

	firstLoadEntered := make(chan struct{})
	releaseFirstLoad := make(chan struct{})
	files := &fakeFileStore{
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
		codexEntry: codex.Entry{
			ID:          "char_0123456789abcdef0123",
			Type:        codex.TypeCharacter,
			Name:        "Obi-Wan Kenobi",
			Aliases:     []string{"Ben"},
			Tags:        []string{"mentor"},
			Description: "Guide.\n",
			Metadata:    map[string]string{"status": "alive"},
			Revision:    "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		},
		codexEntryBytesSequence: [][]byte{[]byte("updated"), []byte("current")},
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
		&fakeIDGenerator{},
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

	updateDone := make(chan error, 1)
	go func() {
		_, err := service.UpdateCodexEntry(context.Background(), "char_0123456789abcdef0123", codex.SaveEntryRequest{
			Name:             "Ben Kenobi",
			Aliases:          []string{"Ben"},
			Tags:             []string{"mentor"},
			Description:      "Guide.\n",
			Metadata:         map[string]string{"status": "alive"},
			ExpectedRevision: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		})
		updateDone <- err
	}()

	// Test: while a scene save holds the shared mutation lock, a concurrent Codex mutation waits until the scene save finishes instead of observing interleaved mutation state.
	// Requirements: M3-R13, M3-R14
	select {
	case err := <-updateDone:
		t.Fatalf("Codex mutation completed before scene save was released: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	close(releaseFirstLoad)
	if err := <-saveDone; err != nil {
		t.Fatalf("SaveScene() error = %v", err)
	}
	if err := <-updateDone; err != nil {
		t.Fatalf("UpdateCodexEntry() error = %v", err)
	}
}
