// BDD Scenario: 8.4.1 - Promote selected files to main
// Requirements: M8-R12, M8-R14, M8-R15
// Test purpose: Promotion preflight rejects conflicts before checkout.

package branch_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"storywork/internal/branch"
	"storywork/internal/mutation"
)

type promotionRepo struct {
	*fakeRepo
	calls       []string
	mainChanged []branch.ProjectPath
	fail        map[string]error
}

func (r *promotionRepo) record(name string) error {
	r.calls = append(r.calls, name)
	return r.fail[name]
}

func (r *promotionRepo) PathsChanged(context.Context, string, branch.CommitID, branch.CommitID) ([]branch.ProjectPath, error) {
	if err := r.record("paths_changed"); err != nil {
		return nil, err
	}
	return r.mainChanged, nil
}
func (r *promotionRepo) SnapshotMainPaths(_ context.Context, _ string, main branch.CommitID, paths []branch.ProjectPath) ([]branch.PathSnapshot, error) {
	if err := r.record("snapshot"); err != nil {
		return nil, err
	}
	result := make([]branch.PathSnapshot, len(paths))
	for index, path := range paths {
		result[index] = branch.PathSnapshot{Path: branch.ProjectPath(path), Exists: true, SourceCommit: main}
	}
	return result, nil
}
func (r *promotionRepo) Switch(ctx context.Context, path string, ref branch.BranchRef) error {
	if err := r.record("switch:" + string(ref)); err != nil {
		return err
	}
	return r.fakeRepo.Switch(ctx, path, ref)
}
func (r *promotionRepo) ApplyPaths(context.Context, string, branch.CommitID, []branch.ChangedFile, []branch.ProjectPath) error {
	return r.record("apply")
}
func (r *promotionRepo) StagePaths(context.Context, string, []branch.ProjectPath) error {
	return r.record("stage")
}
func (r *promotionRepo) RestoreSnapshots(context.Context, string, []branch.PathSnapshot) error {
	return r.record("restore")
}
func (r *promotionRepo) UnstagePaths(context.Context, string, []branch.ProjectPath) error {
	return r.record("unstage")
}
func (r *promotionRepo) CommitPromotion(_ context.Context, _ string, commit branch.PromotionCommit) (branch.CommitID, error) {
	if err := r.record("commit"); err != nil {
		return "", err
	}
	head := branch.CommitID("dddddddddddddddddddddddddddddddddddddddd")
	r.status.ActiveBranch = branch.CanonBranchName
	r.status.IsCanon = true
	r.status.IsManaged = false
	r.status.IsDetached = false
	r.status.IsClean = true
	r.status.ExperimentID = ""
	r.status.ExperimentHead = ""
	r.status.MainHead = head
	r.mainHead = head
	return head, nil
}

type promotionValidator struct {
	calls *[]string
	err   error
}

func (v promotionValidator) ValidateProject(context.Context, string) error {
	*v.calls = append(*v.calls, "validate")
	return v.err
}

type promotionIndex struct {
	calls    *[]string
	failures int
}

func (i *promotionIndex) Rebuild(context.Context, string) error {
	*i.calls = append(*i.calls, "index")
	if i.failures > 0 {
		i.failures--
		return errors.New("index failed")
	}
	return nil
}

func newPromotionService(repo *promotionRepo, index branch.Index, validator branch.CanonicalValidator) *branch.Service {
	return branch.NewService(repo, index, mutation.NewCoordinator(), branch.SessionAdapter{PathFn: func() (string, bool) { return "/tmp/project", true }}, validator, nil, &staticIDs{id: "brn_0123456789abcdef0123"})
}

func promotionFixture(t *testing.T) (*promotionRepo, branch.PromotionRequest) {
	t.Helper()
	mainHead := branch.CommitID("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	experimentHead := branch.CommitID("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	files := []branch.ChangedFile{{Path: "outline.yaml", Status: branch.StatusModified}}
	fingerprint, err := branch.ComputeFingerprint(mainHead, experimentHead, "cccccccccccccccccccccccccccccccccccccccc", files)
	if err != nil {
		t.Fatal(err)
	}
	repo := &promotionRepo{fakeRepo: &fakeRepo{
		status:      branch.RepositoryStatus{ActiveBranch: "branch/test-exp-0123456789abcdef0123", IsManaged: true, IsClean: true, ExperimentID: "brn_0123456789abcdef0123", ExperimentHead: experimentHead, MainHead: mainHead},
		experiments: []branch.ExperimentRef{{ID: "brn_0123456789abcdef0123", BranchName: "branch/test-exp-0123456789abcdef0123", Head: experimentHead}},
		mainHead:    mainHead, compareFiles: files,
	}, fail: map[string]error{}}
	return repo, branch.PromotionRequest{ExperimentID: "brn_0123456789abcdef0123", Paths: []branch.ProjectPath{"outline.yaml"}, ExpectedMainHead: mainHead, ExpectedExperimentHead: experimentHead, ExpectedFingerprint: fingerprint}
}

// Test: conflict exits before checkout.
// Requirements: M8-R13.
func TestPromoteSelectedFilesRejectsConflictBeforeCheckout(t *testing.T) {
	t.Parallel()
	repo, request := promotionFixture(t)
	repo.mainChanged = []branch.ProjectPath{"outline.yaml"}
	service := newPromotionService(repo, &promotionIndex{calls: &repo.calls}, promotionValidator{calls: &repo.calls})
	_, err := service.PromoteSelectedFiles(context.Background(), request)
	if !errors.Is(err, branch.ErrPromotionConflict) {
		t.Fatalf("err = %v", err)
	}
	if containsCall(repo.calls, "switch:main") {
		t.Fatalf("calls = %v", repo.calls)
	}
}

// Test: a URL/request experiment mismatch fails before checkout.
// Requirements: M8-R12.
func TestPromoteSelectedFilesRejectsMismatchedExperiment(t *testing.T) {
	t.Parallel()
	repo, request := promotionFixture(t)
	request.ExperimentID = "brn_ffffffffffffffffffff"
	service := newPromotionService(repo, &promotionIndex{calls: &repo.calls}, promotionValidator{calls: &repo.calls})
	_, err := service.PromoteSelectedFiles(context.Background(), request)
	if !errors.Is(err, branch.ErrStaleRef) || containsCall(repo.calls, "switch:main") {
		t.Fatalf("err=%v calls=%v", err, repo.calls)
	}
}

// Test: successful promotion follows validate/index/stage/commit order and returns selected paths.
// Requirements: M8-R12, M8-R13, M8-R14, M8-R16.
func TestPromoteSelectedFilesOrdersTransactionAndReturnsResult(t *testing.T) {
	t.Parallel()
	repo, request := promotionFixture(t)
	service := newPromotionService(repo, &promotionIndex{calls: &repo.calls}, promotionValidator{calls: &repo.calls})
	result, err := service.PromoteSelectedFiles(context.Background(), request)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	want := []string{"paths_changed", "paths_changed", "snapshot", "switch:main", "apply", "validate", "index", "stage", "commit"}
	if !reflect.DeepEqual(repo.calls, want) {
		t.Fatalf("calls=%v want=%v", repo.calls, want)
	}
	if result.ExperimentID != request.ExperimentID || !reflect.DeepEqual(result.PromotedPaths, request.Paths) {
		t.Fatalf("result=%#v", result)
	}
}

// Test: every post-checkout failure attempts restore, unstage, and index rebuild.
// Requirements: M8-R15.
func TestPromoteSelectedFilesRollsBackEveryFailureBoundary(t *testing.T) {
	t.Parallel()
	for _, failure := range []string{"apply", "validate", "index", "stage", "commit"} {
		t.Run(failure, func(t *testing.T) {
			repo, request := promotionFixture(t)
			validator := promotionValidator{calls: &repo.calls}
			index := &promotionIndex{calls: &repo.calls}
			if failure == "validate" {
				validator.err = errors.New("validate failed")
			} else if failure == "index" {
				index.failures = 1
			} else {
				repo.fail[failure] = errors.New(failure + " failed")
			}
			_, err := newPromotionService(repo, index, validator).PromoteSelectedFiles(context.Background(), request)
			if err == nil {
				t.Fatal("error = nil")
			}
			for _, call := range []string{"restore", "unstage"} {
				if !containsCall(repo.calls, call) {
					t.Fatalf("%s calls=%v", failure, repo.calls)
				}
			}
			if countCall(repo.calls, "index") < 1 {
				t.Fatalf("%s calls=%v", failure, repo.calls)
			}
		})
	}
}

// Test: rollback attempts all recovery actions and returns joined failures even
// when restoring bytes fails first.
// Requirements: M8-R15.
func TestPromoteSelectedFilesRollbackContinuesAfterRecoveryFailure(t *testing.T) {
	t.Parallel()
	repo, request := promotionFixture(t)
	repo.fail["validate"] = errors.New("unused")
	repo.fail["apply"] = errors.New("apply failed")
	repo.fail["restore"] = errors.New("restore failed")
	repo.fail["unstage"] = errors.New("unstage failed")
	index := &promotionIndex{calls: &repo.calls, failures: 1}
	_, err := newPromotionService(repo, index, promotionValidator{calls: &repo.calls}).PromoteSelectedFiles(context.Background(), request)
	if err == nil {
		t.Fatal("error = nil")
	}
	for _, call := range []string{"restore", "unstage", "index"} {
		if !containsCall(repo.calls, call) {
			t.Fatalf("calls=%v missing %s", repo.calls, call)
		}
	}
}

func containsCall(calls []string, target string) bool { return countCall(calls, target) > 0 }
func countCall(calls []string, target string) int {
	count := 0
	for _, call := range calls {
		if call == target {
			count++
		}
	}
	return count
}
