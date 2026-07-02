package story

// scene_patch_rollback_service_test.go verifies the reviewed AI patch transaction.

import (
	"context"
	"errors"
	"testing"

	"storywork/internal/project"
)

// BDD trace:
//   - Requirements: M4-R13, M4-R14, M4-R15.
//   - Scenario: 4.4.3.
//   - Test purpose: verify every patch persistence failure boundary either
//     leaves canon untouched or runs the shared rollback transaction.
func TestAcceptScenePatchHandlesEveryPersistenceFailureBoundary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                   string
		configure              func(*fakeFileStore, *fakeGitStore, *fakeIndexStore, error)
		wantRollback           int
		wantUnstage            int
		wantRebuild            int
		wantCommitCalls        int
		wantCommitMessageCalls int
	}{
		{
			name: "atomic write",
			configure: func(files *fakeFileStore, _ *fakeGitStore, _ *fakeIndexStore, cause error) {
				files.writeErr = cause
			},
		},
		{
			name: "index rebuild",
			configure: func(_ *fakeFileStore, _ *fakeGitStore, index *fakeIndexStore, cause error) {
				index.rebuildErr = cause
			},
			wantRollback: 1, wantUnstage: 1, wantRebuild: 2,
		},
		{
			name: "Git checkpoint",
			configure: func(_ *fakeFileStore, git *fakeGitStore, _ *fakeIndexStore, cause error) {
				git.commitMessageErr = cause
			},
			wantRollback: 1, wantUnstage: 1, wantRebuild: 2, wantCommitMessageCalls: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cause := errors.New(test.name + " failed")
			files := patchFileStore(t)
			git := &fakeGitStore{clean: true}
			index := &fakeIndexStore{}
			test.configure(files, git, index, cause)
			service := NewService(
				&fakeSession{current: project.Project{Path: "/tmp/story"}, ok: true},
				files, git, index, &fakeIDGenerator{},
			)

			_, err := service.AcceptScenePatch(context.Background(), patchRequest())
			if !errors.Is(err, cause) {
				t.Fatalf("AcceptScenePatch() error = %v, want %v", err, cause)
			}
			if files.rollbackCalls != test.wantRollback || git.unstageCalls != test.wantUnstage || index.rebuildCalls != test.wantRebuild || git.commitMessageCalls != test.wantCommitMessageCalls {
				t.Fatalf("rollback/unstage/rebuild/commit = %d/%d/%d/%d, want %d/%d/%d/%d", files.rollbackCalls, git.unstageCalls, index.rebuildCalls, git.commitMessageCalls, test.wantRollback, test.wantUnstage, test.wantRebuild, test.wantCommitMessageCalls)
			}
			if git.commitMessageCalls == 1 && git.commitMessages[0] != "Accept AI patch run_0123456789abcdef0123" {
				t.Fatalf("commit message = %q", git.commitMessages[0])
			}
		})
	}
}

func patchFileStore(t *testing.T) *fakeFileStore {
	t.Helper()
	return &fakeFileStore{
		loadOutline: mustSceneOutline(t),
		scene: SceneDocument{
			ID: "scn_00000000000000000001", ChapterID: "ch_00000000000000000001",
			Title: "The Duel", FrontMatter: SceneFrontMatter{Status: "draft"},
			Markdown: "Alpha beta.\n", Revision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			Canonical: []byte("old canonical bytes"),
		},
		sceneBytes: []byte("new canonical bytes"),
	}
}

func patchRequest() AcceptScenePatchRequest {
	return AcceptScenePatchRequest{
		RunID: "run_0123456789abcdef0123", SceneID: "scn_00000000000000000001",
		RunSceneRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ExpectedRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		StartByte:        0, EndByte: 5, OriginalText: "Alpha", ReplacementText: "Polished",
	}
}
