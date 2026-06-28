package story

import (
	"context"
	"errors"
	"testing"

	"storywork/internal/project"
)

// BDD trace:
//   - Requirement: M2-R06, M2-R12, M2-R14.
//   - Scenario: 2.3.1 — Stale revision.
//   - Test purpose: verify stale scene revisions fail with conflict semantics
//     and leave canonical files, index rebuilds, and Git history untouched.
func TestSaveSceneRejectsStaleRevisionWithoutMutation(t *testing.T) {
	t.Parallel()

	service := NewService(
		&fakeSession{current: project.Project{Path: "/tmp/story"}, ok: true},
		&fakeFileStore{
			loadOutline: mustSceneOutline(t),
			scene: SceneDocument{
				ID:          "scn_00000000000000000001",
				ChapterID:   "ch_00000000000000000001",
				Title:       "The Duel",
				FrontMatter: SceneFrontMatter{Status: "draft"},
				Markdown:    "Original.\n",
				Revision:    "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				Canonical:   []byte("old"),
			},
		},
		&fakeGitStore{clean: true},
		&fakeIndexStore{},
		&fakeIDGenerator{},
	)

	_, err := service.SaveScene(context.Background(), "scn_00000000000000000001", SaveSceneRequest{
		Title:            "The Duel",
		FrontMatter:      SceneFrontMatter{Status: "draft"},
		Markdown:         "Original.\n",
		ExpectedRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	})
	if !errors.Is(err, ErrStaleRevision) {
		t.Fatalf("SaveScene() error = %v, want ErrStaleRevision", err)
	}
}

// BDD trace:
//   - Requirement: M2-R12.
//   - Scenario: 2.3.2 — Dirty project.
//   - Test purpose: verify dirty worktrees reject scene saves before any
//     canonical write, index rebuild, or checkpoint attempt.
func TestSaveSceneRejectsDirtyProjectBeforeMutation(t *testing.T) {
	t.Parallel()

	files := &fakeFileStore{loadOutline: mustSceneOutline(t)}
	git := &fakeGitStore{clean: false}
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
		FrontMatter:      SceneFrontMatter{Status: "draft"},
		Markdown:         "Original.\n",
		ExpectedRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	})
	if !errors.Is(err, ErrDirtyWorktree) {
		t.Fatalf("SaveScene() error = %v, want ErrDirtyWorktree", err)
	}
	if files.writeCalls != 0 || index.rebuildCalls != 0 || git.commitCalls != 0 {
		t.Fatalf("write/rebuild/commit calls = %d/%d/%d, want 0/0/0", files.writeCalls, index.rebuildCalls, git.commitCalls)
	}
}
