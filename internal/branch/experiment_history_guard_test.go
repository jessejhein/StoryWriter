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
		status:           branch.RepositoryStatus{ActiveBranch: "branch/test-exp-0123456789abcdef0123", IsManaged: true, IsClean: true, ExperimentID: "brn_0123456789abcdef0123", ExperimentHead: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", MainHead: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		experiments:      []branch.ExperimentRef{{ID: "brn_0123456789abcdef0123", BranchName: "branch/test-exp-0123456789abcdef0123", Head: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}},
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

type nilCoordinator struct{}

func (nilCoordinator) Lock()    {}
func (nilCoordinator) Unlock()  {}
func (nilCoordinator) RLock()   {}
func (nilCoordinator) RUnlock() {}
