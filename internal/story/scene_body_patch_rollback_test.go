// BDD Scenario: 7.2.3 - Review and accept one scene replacement
// Requirements: M7-R15
// Test purpose: Scene body patch acceptance rolls back on every persistence failure boundary.

package story

import (
	"context"
	"errors"
	"testing"

	"storywork/internal/project"
)

// Test: accept scene body patch rolls back every failure boundary.
// Requirements: M7-R15.
func TestAcceptSceneBodyPatchRollsBackEveryFailureBoundary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                   string
		configure              func(*fakeFileStore, *fakeGitStore, *fakeIndexStore, error)
		wantRollback           int
		wantUnstage            int
		wantRebuild            int
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

			_, err := service.AcceptSceneBodyPatch(context.Background(), AcceptSceneBodyPatchRequest{
				RunID: "run_0123456789abcdef0123", SceneID: "scn_00000000000000000001",
				RunSceneRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				ExpectedRevision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				OriginalMarkdown: "Alpha beta.\n", ReplacementMarkdown: "Rewritten.\n",
			})
			if !errors.Is(err, cause) {
				t.Fatalf("AcceptSceneBodyPatch() error = %v, want %v", err, cause)
			}
			if files.rollbackCalls != test.wantRollback || git.unstageCalls != test.wantUnstage || index.rebuildCalls != test.wantRebuild || git.commitMessageCalls != test.wantCommitMessageCalls {
				t.Fatalf("rollback/unstage/rebuild/commit = %d/%d/%d/%d, want %d/%d/%d/%d", files.rollbackCalls, git.unstageCalls, index.rebuildCalls, git.commitMessageCalls, test.wantRollback, test.wantUnstage, test.wantRebuild, test.wantCommitMessageCalls)
			}
		})
	}
}
