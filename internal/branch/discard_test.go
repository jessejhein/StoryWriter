// BDD Scenario: 8.5.1 - Discard the active experiment
// Requirements: M8-R17
// Test purpose: Discard refuses dirty worktrees and stale heads.

package branch_test

import (
	"context"
	"errors"
	"testing"

	"storywork/internal/branch"
	"storywork/internal/mutation"
)

// Test: dirty worktree refuses discard.
// Requirements: M8-R17.
func TestDiscardExperimentRejectsDirtyWorktree(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{status: branch.RepositoryStatus{IsClean: false}}
	service := branch.NewService(repo, &fakeIndex{}, mutation.NewCoordinator(), branch.SessionAdapter{PathFn: func() (string, bool) { return "/tmp/project", true }}, nil, nil, &staticIDs{id: "brn_0123456789abcdef0123"})
	_, err := service.DiscardExperiment(context.Background(), "brn_0123456789abcdef0123", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	if !errors.Is(err, branch.ErrDirtyWorktree) {
		t.Fatalf("err = %v, want ErrDirtyWorktree", err)
	}
}

// Test: active discard switches to main, rebuilds once, deletes only the
// expected experiment, and returns canon status.
// Requirements: M8-R03, M8-R17.
func TestDiscardActiveExperimentSwitchesIndexesAndDeletes(t *testing.T) {
	t.Parallel()
	head := branch.CommitID("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	repo := &fakeRepo{
		status:      branch.RepositoryStatus{ActiveBranch: "branch/test-exp-0123456789abcdef0123", IsManaged: true, IsClean: true, ExperimentID: "brn_0123456789abcdef0123", ExperimentHead: head},
		experiments: []branch.ExperimentRef{{ID: "brn_0123456789abcdef0123", BranchName: "branch/test-exp-0123456789abcdef0123", Head: head}},
	}
	index := &fakeIndex{}
	service := branch.NewService(repo, index, mutation.NewCoordinator(), branch.SessionAdapter{PathFn: func() (string, bool) { return "/tmp/project", true }}, nil, nil, &staticIDs{id: "brn_0123456789abcdef0123"})
	status, err := service.DiscardExperiment(context.Background(), "brn_0123456789abcdef0123", head)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if status.ActiveBranch != branch.CanonBranchName || len(repo.experiments) != 0 || index.rebuilds != 1 {
		t.Fatalf("status=%#v experiments=%#v rebuilds=%d", status, repo.experiments, index.rebuilds)
	}
}

// Test: stale discard stops before switch, index, or deletion.
// Requirements: M8-R17.
func TestDiscardExperimentRejectsStaleHeadWithoutSideEffects(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{status: branch.RepositoryStatus{ActiveBranch: "main", IsCanon: true, IsClean: true}, experiments: []branch.ExperimentRef{{ID: "brn_0123456789abcdef0123", BranchName: "branch/test-exp-0123456789abcdef0123", Head: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}}}
	index := &fakeIndex{}
	service := branch.NewService(repo, index, mutation.NewCoordinator(), branch.SessionAdapter{PathFn: func() (string, bool) { return "/tmp/project", true }}, nil, nil, &staticIDs{id: "brn_0123456789abcdef0123"})
	_, err := service.DiscardExperiment(context.Background(), "brn_0123456789abcdef0123", "cccccccccccccccccccccccccccccccccccccccc")
	if !errors.Is(err, branch.ErrStaleRef) || index.rebuilds != 0 || len(repo.experiments) != 1 {
		t.Fatalf("error=%v rebuilds=%d experiments=%#v", err, index.rebuilds, repo.experiments)
	}
}
