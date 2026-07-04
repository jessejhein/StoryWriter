// BDD Scenario: 8.1.1 - Create from current canon
// Requirements: M8-R01, M8-R02, M8-R03
// Test purpose: Branch service orchestrates create/switch with coordinator and index.

package branch_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"storywork/internal/branch"
	"storywork/internal/mutation"
)

type fakeRepo struct {
	status       branch.RepositoryStatus
	experiments  []branch.ExperimentRef
	mainHead     branch.CommitID
	compareFiles []branch.ChangedFile
	blobSides    map[string]branch.TextSide
}

func (f *fakeRepo) Status(context.Context, string) (branch.RepositoryStatus, error) {
	return f.status, nil
}
func (f *fakeRepo) ListExperiments(context.Context, string) ([]branch.ExperimentRef, error) {
	return f.experiments, nil
}
func (f *fakeRepo) CreateAndSwitch(_ context.Context, _ string, ref branch.ExperimentRef, _ branch.CommitID) error {
	f.status.ActiveBranch = string(ref.BranchName)
	f.status.IsManaged = true
	f.status.ExperimentID = ref.ID
	f.status.ExperimentHead = ref.Head
	f.experiments = append(f.experiments, ref)
	return nil
}
func (f *fakeRepo) Switch(_ context.Context, _ string, ref branch.BranchRef) error {
	f.status.ActiveBranch = string(ref)
	f.status.IsCanon = ref == branch.CanonBranchName
	f.status.IsManaged = branch.IsManagedExperimentRef(string(ref))
	if ref == branch.CanonBranchName {
		f.status.ExperimentID = ""
		f.status.ExperimentHead = ""
	}
	return nil
}
func (f *fakeRepo) DeleteExperiment(_ context.Context, _ string, ref branch.ExperimentRef, _ branch.CommitID) error {
	filtered := f.experiments[:0]
	for _, experiment := range f.experiments {
		if experiment.ID == ref.ID {
			continue
		}
		filtered = append(filtered, experiment)
	}
	f.experiments = filtered
	return nil
}
func (f *fakeRepo) CompareTrees(context.Context, string, branch.CommitID, branch.CommitID) ([]branch.ChangedFile, error) {
	if f.compareFiles != nil {
		return f.compareFiles, nil
	}
	return []branch.ChangedFile{{Path: "outline.yaml", Status: branch.StatusModified}}, nil
}

func (f *fakeRepo) ReadTextBlob(_ context.Context, _ string, commit branch.CommitID, path branch.ProjectPath) (branch.TextSide, error) {
	if f.blobSides != nil {
		return f.blobSides[string(commit)+"|"+string(path)], nil
	}
	return branch.TextSide{}, nil
}
func (f *fakeRepo) MergeBase(context.Context, string, branch.CommitID, branch.CommitID) (branch.CommitID, error) {
	return "cccccccccccccccccccccccccccccccccccccccc", nil
}
func (f *fakeRepo) PathsChanged(context.Context, string, branch.CommitID, branch.CommitID) ([]branch.ProjectPath, error) {
	return nil, nil
}
func (f *fakeRepo) UnifiedDiff(_ context.Context, _ string, _, _ branch.CommitID, _ []branch.ProjectPath, _ int) (string, error) {
	return "", nil
}
func (f *fakeRepo) SnapshotMainPaths(context.Context, string, branch.CommitID, []branch.ProjectPath) ([]branch.PathSnapshot, error) {
	return nil, nil
}
func (f *fakeRepo) ApplyPaths(context.Context, string, branch.CommitID, []branch.ChangedFile, []branch.ProjectPath) error {
	return nil
}
func (f *fakeRepo) StagePaths(context.Context, string, []branch.ProjectPath) error   { return nil }
func (f *fakeRepo) UnstagePaths(context.Context, string, []branch.ProjectPath) error { return nil }
func (f *fakeRepo) RestoreSnapshots(context.Context, string, []branch.PathSnapshot) error {
	return nil
}
func (f *fakeRepo) CommitPromotion(context.Context, string, branch.PromotionCommit) (branch.CommitID, error) {
	head := branch.CommitID("dddddddddddddddddddddddddddddddddddddddd")
	f.status.ActiveBranch = branch.CanonBranchName
	f.status.IsCanon = true
	f.status.IsManaged = false
	f.status.IsDetached = false
	f.status.IsClean = true
	f.status.ExperimentID = ""
	f.status.ExperimentHead = ""
	f.status.MainHead = head
	f.mainHead = head
	return head, nil
}
func (f *fakeRepo) ResolveCommit(context.Context, string, string) (branch.CommitID, error) {
	return f.mainHead, nil
}
func (f *fakeRepo) IsClean(context.Context, string) (bool, error) { return true, nil }

type fakeIndex struct {
	rebuilds int
	failNext bool
}

func (f *fakeIndex) Rebuild(context.Context, string) error {
	f.rebuilds++
	if f.failNext {
		f.failNext = false
		return errors.New("index rebuild failed")
	}
	return nil
}

type staticIDs struct{ id branch.ExperimentID }

func (s *staticIDs) NextExperimentID() (branch.ExperimentID, error) { return s.id, nil }

// Test: create locks, checks clean, creates, rebuilds index.
// Requirements: M8-R02, M8-R03.
func TestCreateExperimentRebuildsIndex(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{
		status:   branch.RepositoryStatus{ActiveBranch: "main", IsCanon: true, IsClean: true, MainHead: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		mainHead: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	index := &fakeIndex{}
	service := branch.NewService(repo, index, mutation.NewCoordinator(), branch.SessionAdapter{PathFn: func() (string, bool) { return "/tmp/project", true }}, nil, nil, &staticIDs{id: "brn_0123456789abcdef0123"})
	status, err := service.CreateExperiment(context.Background(), "Test Exp")
	if err != nil {
		t.Fatalf("CreateExperiment() error = %v", err)
	}
	if !status.IsManaged || index.rebuilds != 1 {
		t.Fatalf("status = %#v rebuilds = %d", status, index.rebuilds)
	}
}

// Test: index failure triggers recovery switch.
// Requirements: M8-R04.
func TestCreateExperimentRecoversOnIndexFailure(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{
		status:   branch.RepositoryStatus{ActiveBranch: "main", IsCanon: true, IsClean: true, MainHead: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		mainHead: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	index := &fakeIndex{failNext: true}
	service := branch.NewService(repo, index, mutation.NewCoordinator(), branch.SessionAdapter{PathFn: func() (string, bool) { return "/tmp/project", true }}, nil, nil, &staticIDs{id: "brn_0123456789abcdef0123"})
	if _, err := service.CreateExperiment(context.Background(), "Test Exp"); err == nil {
		t.Fatal("CreateExperiment() = nil, want index failure")
	}
	if repo.status.ActiveBranch != "main" {
		t.Fatalf("active branch = %q, want main", repo.status.ActiveBranch)
	}
	if len(repo.experiments) != 0 {
		t.Fatalf("failed creation left experiment refs: %#v", repo.experiments)
	}
	if index.rebuilds != 2 {
		t.Fatalf("index rebuilds = %d, want failed attempt plus recovery", index.rebuilds)
	}
}

// Test: concurrent mutation cannot interleave checkout.
// Requirements: M8-R03.
func TestCreateExperimentSerializesUnderCoordinator(t *testing.T) {
	t.Parallel()
	coordinator := mutation.NewCoordinator()
	repo := &fakeRepo{
		status:   branch.RepositoryStatus{ActiveBranch: "main", IsCanon: true, IsClean: true, MainHead: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		mainHead: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	service := branch.NewService(repo, &fakeIndex{}, coordinator, branch.SessionAdapter{PathFn: func() (string, bool) { return "/tmp/project", true }}, nil, nil, &staticIDs{id: "brn_0123456789abcdef0123"})
	coordinator.Lock()
	started := make(chan struct{})
	done := make(chan struct{})
	go func() {
		close(started)
		_, _ = service.CreateExperiment(context.Background(), "Blocked")
		close(done)
	}()
	<-started
	select {
	case <-done:
		t.Fatal("CreateExperiment() completed while lock held")
	case <-time.After(100 * time.Millisecond):
	}
	coordinator.Unlock()
	<-done
}

// Test: unmanaged active branches are outside the branch-changing contract and
// fail before ref or index mutation.
// Requirements: M8-R01, M8-R03.
func TestBranchChangesRejectUnmanagedActiveBranch(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{status: branch.RepositoryStatus{ActiveBranch: "user/topic", IsClean: true}, mainHead: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	index := &fakeIndex{}
	service := branch.NewService(repo, index, mutation.NewCoordinator(), branch.SessionAdapter{PathFn: func() (string, bool) { return "/tmp/project", true }}, nil, nil, &staticIDs{id: "brn_0123456789abcdef0123"})
	if _, err := service.CreateExperiment(context.Background(), "Unsafe"); !errors.Is(err, branch.ErrUnmanagedBranch) {
		t.Fatalf("CreateExperiment() error = %v", err)
	}
	if _, err := service.SwitchTarget(context.Background(), "main", nil); !errors.Is(err, branch.ErrUnmanagedBranch) {
		t.Fatalf("SwitchTarget() error = %v", err)
	}
	if index.rebuilds != 0 || len(repo.experiments) != 0 {
		t.Fatalf("rebuilds=%d experiments=%#v", index.rebuilds, repo.experiments)
	}
}
