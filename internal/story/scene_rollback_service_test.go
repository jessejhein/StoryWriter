package story

import (
	"context"
	"errors"
	"testing"

	"storywork/internal/project"
)

// BDD trace:
//   - Requirement: M2-R10, M2-R13.
//   - Scenario: 2.4.2 — Index rebuild failure.
//   - Test purpose: verify a failed index rebuild restores the original scene,
//     unstages app changes, rebuilds the restored index, and reports failure.
func TestSaveSceneRollsBackWhenIndexRebuildFails(t *testing.T) {
	t.Parallel()

	cause := errors.New("index failed")
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
	}
	git := &fakeGitStore{clean: true}
	index := &fakeIndexStore{rebuildErr: cause}
	service := NewService(
		&fakeSession{current: project.Project{Path: "/tmp/story"}, ok: true},
		files,
		git,
		index,
		&fakeIDGenerator{},
	)

	_, err := service.SaveScene(context.Background(), "scn_00000000000000000001", SaveSceneRequest{
		Title:            "The Duel",
		FrontMatter:      SceneFrontMatter{Status: "revised"},
		Markdown:         "Changed.\n",
		ExpectedRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	})
	if !errors.Is(err, cause) {
		t.Fatalf("SaveScene() error = %v, want %v", err, cause)
	}
	if files.rollbackCalls != 1 || git.unstageCalls != 1 || index.rebuildCalls != 2 || git.commitCalls != 0 {
		t.Fatalf("rollback/unstage/rebuild/commit = %d/%d/%d/%d", files.rollbackCalls, git.unstageCalls, index.rebuildCalls, git.commitCalls)
	}
}

// BDD trace:
//   - Requirement: M2-R09, M2-R13.
//   - Scenario: 2.4.1 — Atomic write failure.
//   - Test purpose: verify a failed canonical write returns immediately without
//     rebuilding the index, unstaging changes, or creating a checkpoint.
func TestSaveSceneStopsBeforeRollbackWorkWhenAtomicWriteFails(t *testing.T) {
	t.Parallel()

	cause := errors.New("replace failed")
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
		writeErr:   cause,
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
		Title:            "The Duel",
		FrontMatter:      SceneFrontMatter{Status: "revised"},
		Markdown:         "Changed.\n",
		ExpectedRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	})
	if !errors.Is(err, cause) {
		t.Fatalf("SaveScene() error = %v, want %v", err, cause)
	}
	if files.rollbackCalls != 0 || git.unstageCalls != 0 || index.rebuildCalls != 0 || git.commitCalls != 0 {
		t.Fatalf("rollback/unstage/rebuild/commit = %d/%d/%d/%d", files.rollbackCalls, git.unstageCalls, index.rebuildCalls, git.commitCalls)
	}
}

// BDD trace:
//   - Requirement: M2-R11, M2-R13.
//   - Scenario: 2.4.3 — Git checkpoint failure.
//   - Test purpose: verify a failed checkpoint restores the original scene,
//     unstages app changes, rebuilds the restored index, and never reports a
//     successful save.
func TestSaveSceneRollsBackWhenCheckpointFails(t *testing.T) {
	t.Parallel()

	cause := errors.New("commit failed")
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
	}
	git := &fakeGitStore{clean: true, commitErr: cause}
	index := &fakeIndexStore{}
	service := NewService(
		&fakeSession{current: project.Project{Path: "/tmp/story"}, ok: true},
		files,
		git,
		index,
		&fakeIDGenerator{},
	)

	_, err := service.SaveScene(context.Background(), "scn_00000000000000000001", SaveSceneRequest{
		Title:            "The Duel",
		FrontMatter:      SceneFrontMatter{Status: "revised"},
		Markdown:         "Changed.\n",
		ExpectedRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	})
	if !errors.Is(err, cause) {
		t.Fatalf("SaveScene() error = %v, want %v", err, cause)
	}
	if files.rollbackCalls != 1 || git.unstageCalls != 1 || index.rebuildCalls != 2 || git.commitCalls != 1 {
		t.Fatalf("rollback/unstage/rebuild/commit = %d/%d/%d/%d", files.rollbackCalls, git.unstageCalls, index.rebuildCalls, git.commitCalls)
	}
}
