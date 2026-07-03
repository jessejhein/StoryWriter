// BDD Scenario: 8.1.2 - Reject unsafe branch state via inspectable sentinels
// Requirements: M8-R03, M8-R17
// Test purpose: branch.mapRepositoryError uses errors.Is for wrapped gitstore sentinels.

package branch_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"storywork/internal/branch"
	"storywork/internal/gitstore"
	"storywork/internal/mutation"
)

type errorRepo struct {
	err error
	branch.RepositoryStatus
}

func (r *errorRepo) Status(context.Context, string) (branch.RepositoryStatus, error) {
	return r.RepositoryStatus, r.err
}
func (r *errorRepo) ListExperiments(context.Context, string) ([]branch.ExperimentRef, error) {
	return nil, nil
}
func (r *errorRepo) CreateAndSwitch(context.Context, string, branch.ExperimentRef, branch.CommitID) error {
	return nil
}
func (r *errorRepo) Switch(context.Context, string, branch.BranchRef) error { return nil }
func (r *errorRepo) DeleteExperiment(context.Context, string, branch.ExperimentRef, branch.CommitID) error {
	return nil
}
func (r *errorRepo) CompareTrees(context.Context, string, branch.CommitID, branch.CommitID) ([]branch.ChangedFile, error) {
	return nil, nil
}
func (r *errorRepo) ReadTextBlob(context.Context, string, branch.CommitID, branch.ProjectPath) (branch.TextSide, error) {
	return branch.TextSide{}, nil
}
func (r *errorRepo) MergeBase(context.Context, string, branch.CommitID, branch.CommitID) (branch.CommitID, error) {
	return "", nil
}
func (r *errorRepo) PathsChanged(context.Context, string, branch.CommitID, branch.CommitID) ([]branch.ProjectPath, error) {
	return nil, nil
}
func (r *errorRepo) UnifiedDiff(context.Context, string, branch.CommitID, branch.CommitID, []branch.ProjectPath, int) (string, error) {
	return "", nil
}
func (r *errorRepo) SnapshotMainPaths(context.Context, string, branch.CommitID, []branch.ProjectPath) ([]branch.PathSnapshot, error) {
	return nil, nil
}
func (r *errorRepo) ApplyPaths(context.Context, string, branch.CommitID, []branch.ChangedFile, []branch.ProjectPath) error {
	return nil
}
func (r *errorRepo) StagePaths(context.Context, string, []branch.ProjectPath) error   { return nil }
func (r *errorRepo) UnstagePaths(context.Context, string, []branch.ProjectPath) error { return nil }
func (r *errorRepo) RestoreSnapshots(context.Context, string, []branch.PathSnapshot) error {
	return nil
}
func (r *errorRepo) CommitPromotion(context.Context, string, branch.PromotionCommit) (branch.CommitID, error) {
	return "", nil
}
func (r *errorRepo) ResolveCommit(context.Context, string, string) (branch.CommitID, error) {
	return "", nil
}
func (r *errorRepo) IsClean(context.Context, string) (bool, error) { return true, nil }

func newErrorService(err error) *branch.Service {
	repo := &errorRepo{err: err}
	return branch.NewService(repo, &fakeIndex{}, mutation.NewCoordinator(),
		branch.SessionAdapter{PathFn: func() (string, bool) { return "/tmp/project", true }},
		nil, nil, &staticIDs{id: "brn_0123456789abcdef0123"})
}

// Test: a wrapped gitstore.ErrDirtyWorktree with differing text maps to
// branch.ErrDirtyWorktree, not the generic repository-state error.
func TestMapRepositoryErrorDirtyWorktreeSentinel(t *testing.T) {
	t.Parallel()
	wrapped := fmt.Errorf("adapter: %w", gitstore.ErrDirtyWorktree)
	_, err := newErrorService(wrapped).Status(context.Background())
	if !errors.Is(err, branch.ErrDirtyWorktree) {
		t.Fatalf("err = %v, want errors.Is branch.ErrDirtyWorktree", err)
	}
}

// Test: a wrapped gitstore.ErrStaleExperimentHead with differing text maps to
// branch.ErrStaleRef.
func TestMapRepositoryErrorStaleHeadSentinel(t *testing.T) {
	t.Parallel()
	wrapped := fmt.Errorf("adapter: %w", gitstore.ErrStaleExperimentHead)
	_, err := newErrorService(wrapped).Status(context.Background())
	if !errors.Is(err, branch.ErrStaleRef) {
		t.Fatalf("err = %v, want errors.Is branch.ErrStaleRef", err)
	}
}
