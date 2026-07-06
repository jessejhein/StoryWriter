// BDD Scenario: 8.2.1 - List exact changed files; 8.4.1 - Promote selected files to main
// Requirements: M8-R04, M8-R05, M8-R12, M8-R17
// Test purpose: Rewritten managed experiment refs fail closed for comparison and
// promotion before branch checkout or canonical mutation.

package branch_test

import (
	"context"
	"errors"
	"testing"

	"storywork/internal/branch"
)

// Test: comparison rejects a managed experiment whose current head no longer
// retains the validated merge base in its ancestry.
// Requirements: M8-R05, M8-R17.
func TestLoadComparisonRejectsRewrittenExperimentHistory(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{
		status:           branch.RepositoryStatus{ActiveBranch: "branch/test-exp-0123456789abcdef0123", IsManaged: true, IsClean: true, ExperimentID: "brn_0123456789abcdef0123", ExperimentHead: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", BaseHead: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", MainHead: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		experiments:      []branch.ExperimentRef{{ID: "brn_0123456789abcdef0123", BranchName: "branch/test-exp-0123456789abcdef0123", Head: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", BaseHead: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}},
		mainHead:         "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		forceNonAncestor: true,
	}
	service := branch.NewService(repo, &fakeIndex{}, nilCoordinator{}, branch.SessionAdapter{PathFn: func() (string, bool) { return "/tmp/project", true }}, nil, nil, &staticIDs{id: "brn_0123456789abcdef0123"})

	_, err := service.LoadComparison(context.Background(), "brn_0123456789abcdef0123")
	if !errors.Is(err, branch.ErrStaleRef) {
		t.Fatalf("err = %v, want errors.Is(branch.ErrStaleRef)", err)
	}
}

// Test: promotion rejects rewritten experiment history before switching to main.
// Requirements: M8-R12, M8-R17.
func TestPromoteSelectedFilesRejectsRewrittenExperimentHistoryBeforeCheckout(t *testing.T) {
	t.Parallel()
	repo, request := promotionFixture(t)
	repo.forceNonAncestor = true
	service := newPromotionService(repo, &promotionIndex{calls: &repo.calls}, promotionValidator{calls: &repo.calls})

	_, err := service.PromoteSelectedFiles(context.Background(), request)
	if !errors.Is(err, branch.ErrStaleRef) {
		t.Fatalf("err = %v, want errors.Is(branch.ErrStaleRef)", err)
	}
	if containsCall(repo.calls, "switch:main") {
		t.Fatalf("calls = %v", repo.calls)
	}
}

// Test: a missing recorded experiment base is treated as repository-state
// corruption before switch, comparison, or discard work.
// Requirements: M8-R05, M8-R12, M8-R17.
func TestMissingExperimentBaseFailsClosedBeforeBranchMutation(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{
		status: branch.RepositoryStatus{
			ActiveBranch:   "branch/test-exp-0123456789abcdef0123",
			IsManaged:      true,
			IsClean:        true,
			ExperimentID:   "brn_0123456789abcdef0123",
			ExperimentHead: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			MainHead:       "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		experiments: []branch.ExperimentRef{{ID: "brn_0123456789abcdef0123", BranchName: "branch/test-exp-0123456789abcdef0123", Head: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}},
		mainHead:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	service := branch.NewService(repo, &fakeIndex{}, nilCoordinator{}, branch.SessionAdapter{PathFn: func() (string, bool) { return "/tmp/project", true }}, nil, nil, &staticIDs{id: "brn_0123456789abcdef0123"})
	if _, err := service.LoadComparison(context.Background(), "brn_0123456789abcdef0123"); !errors.Is(err, branch.ErrRepositoryState) {
		t.Fatalf("LoadComparison() err = %v, want ErrRepositoryState", err)
	}
	if _, err := service.SwitchTarget(context.Background(), "brn_0123456789abcdef0123", ptrCommit("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")); !errors.Is(err, branch.ErrRepositoryState) {
		t.Fatalf("SwitchTarget() err = %v, want ErrRepositoryState", err)
	}
	if _, err := service.DiscardExperiment(context.Background(), "brn_0123456789abcdef0123", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"); !errors.Is(err, branch.ErrRepositoryState) {
		t.Fatalf("DiscardExperiment() err = %v, want ErrRepositoryState", err)
	}
}

func ptrCommit(value string) *branch.CommitID {
	id := branch.CommitID(value)
	return &id
}

// Test: a related but non-descendant experiment history is rejected before
// switch or discard mutations.
// Requirements: M8-R05, M8-R12, M8-R17.
func TestSwitchAndDiscardRejectRelatedRewrittenExperimentHistory(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{
		status: branch.RepositoryStatus{
			ActiveBranch:   "branch/test-exp-0123456789abcdef0123",
			IsManaged:      true,
			IsClean:        true,
			ExperimentID:   "brn_0123456789abcdef0123",
			ExperimentHead: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			BaseHead:       "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			MainHead:       "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		experiments: []branch.ExperimentRef{{
			ID:         "brn_0123456789abcdef0123",
			BranchName: "branch/test-exp-0123456789abcdef0123",
			Head:       "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			BaseHead:   "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		}},
		mainHead:         "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		forceNonAncestor: true,
	}
	service := branch.NewService(repo, &fakeIndex{}, nilCoordinator{}, branch.SessionAdapter{PathFn: func() (string, bool) { return "/tmp/project", true }}, nil, nil, &staticIDs{id: "brn_0123456789abcdef0123"})
	if _, err := service.SwitchTarget(context.Background(), "brn_0123456789abcdef0123", ptrCommit("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")); !errors.Is(err, branch.ErrStaleRef) {
		t.Fatalf("SwitchTarget() err = %v, want ErrStaleRef", err)
	}
	if _, err := service.DiscardExperiment(context.Background(), "brn_0123456789abcdef0123", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"); !errors.Is(err, branch.ErrStaleRef) {
		t.Fatalf("DiscardExperiment() err = %v, want ErrStaleRef", err)
	}
}

type nilCoordinator struct{}

func (nilCoordinator) Lock()    {}
func (nilCoordinator) Unlock()  {}
func (nilCoordinator) RLock()   {}
func (nilCoordinator) RUnlock() {}
