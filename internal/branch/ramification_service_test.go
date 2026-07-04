// BDD Scenario: 8.3.1 - Run only after explicit authorization
// Requirements: M8-R09
// Test purpose: Ramification service revalidates fingerprint under read lock.

package branch_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"storywork/internal/branch"
	"storywork/internal/gitstore"
	"storywork/internal/mutation"
)

type captureAnalyzer struct {
	calls  int
	packet branch.AnalysisPacket
}

func (a *captureAnalyzer) Analyze(_ context.Context, packet branch.AnalysisPacket) (branch.AnalysisResult, error) {
	a.calls++
	a.packet = packet
	return branch.AnalysisResult{Summary: "ok", Findings: []branch.RamificationFinding{}}, nil
}

type analysisRepo struct {
	*fakeRepo
	blobs      map[branch.CommitID]branch.TextSide
	reads      int
	diffText   string
	diffErr    error
	diffBudget int
}

func (r *analysisRepo) ReadTextBlob(_ context.Context, _ string, commit branch.CommitID, _ branch.ProjectPath) (branch.TextSide, error) {
	r.reads++
	return r.blobs[commit], nil
}

func (r *analysisRepo) UnifiedDiff(_ context.Context, _ string, _, _ branch.CommitID, _ []branch.ProjectPath, maxBytes int) (string, error) {
	r.diffBudget = maxBytes
	return r.diffText, r.diffErr
}

// Test: stale fingerprint stops before provider call.
// Requirements: M8-R09.
func TestAnalyzeRamificationsRejectsStaleFingerprint(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{
		status:      branch.RepositoryStatus{IsClean: true},
		experiments: []branch.ExperimentRef{{ID: "brn_0123456789abcdef0123", BranchName: "branch/test-exp-0123456789abcdef0123", Head: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}},
		mainHead:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	service := branch.NewService(repo, &fakeIndex{}, mutation.NewCoordinator(), branch.SessionAdapter{PathFn: func() (string, bool) { return "/tmp/project", true }}, nil, nil, &staticIDs{id: "brn_0123456789abcdef0123"})
	_, err := service.AnalyzeRamifications(context.Background(), "brn_0123456789abcdef0123", branch.AnalysisRequest{
		Goal:                   "test",
		ProfileID:              "local",
		Model:                  "qwen",
		ExpectedMainHead:       "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ExpectedExperimentHead: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		ExpectedFingerprint:    "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
	})
	if !errors.Is(err, branch.ErrStaleFingerprint) {
		t.Fatalf("err = %v", err)
	}
}

// Test: explicit analysis sends a bounded unified-diff packet and releases the
// mutation lock before calling the provider.
// Requirements: M8-R09, M8-R11.
func TestAnalyzeRamificationsBuildsReviewedUnifiedDiffPacket(t *testing.T) {
	t.Parallel()
	mainHead := branch.CommitID("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	experimentHead := branch.CommitID("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	repo := &analysisRepo{
		fakeRepo: &fakeRepo{
			experiments: []branch.ExperimentRef{{ID: "brn_0123456789abcdef0123", BranchName: "branch/test-exp-0123456789abcdef0123", Head: experimentHead}},
			mainHead:    mainHead,
		},
		diffText: "--- a/outline.yaml\n+++ b/outline.yaml\n@@ -1,1 +1,1 @@\n-old line\n+new line\n",
	}
	comparison, err := branch.ComputeFingerprint(mainHead, experimentHead, "cccccccccccccccccccccccccccccccccccccccc", []branch.ChangedFile{{Path: "outline.yaml", Status: branch.StatusModified}})
	if err != nil {
		t.Fatal(err)
	}
	analyzer := &captureAnalyzer{}
	service := branch.NewService(repo, &fakeIndex{}, mutation.NewCoordinator(), branch.SessionAdapter{PathFn: func() (string, bool) { return "/tmp/project", true }}, nil, analyzer, &staticIDs{id: "brn_0123456789abcdef0123"})
	_, err = service.AnalyzeRamifications(context.Background(), "brn_0123456789abcdef0123", branch.AnalysisRequest{
		Goal: "Review the change", ProfileID: "local", Model: "model",
		ExpectedMainHead: mainHead, ExpectedExperimentHead: experimentHead, ExpectedFingerprint: comparison,
	})
	if err != nil {
		t.Fatalf("AnalyzeRamifications() error = %v", err)
	}
	for _, fragment := range []string{"--- a/outline.yaml", "+++ b/outline.yaml", "@@ -1,1 +1,1 @@", "-old line", "+new line"} {
		if !strings.Contains(analyzer.packet.DiffText, fragment) {
			t.Fatalf("diff %q missing %q", analyzer.packet.DiffText, fragment)
		}
	}
}

// Test: oversized diff fails before the provider call and returns no partial
// packet.
// Requirements: M8-R09.
func TestAnalyzeRamificationsRejectsOversizedDiffBeforeProvider(t *testing.T) {
	t.Parallel()
	mainHead := branch.CommitID("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	experimentHead := branch.CommitID("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	files := []branch.ChangedFile{{Path: "outline.yaml", Status: branch.StatusModified}}
	fingerprint, err := branch.ComputeFingerprint(mainHead, experimentHead, "cccccccccccccccccccccccccccccccccccccccc", files)
	if err != nil {
		t.Fatal(err)
	}
	repo := &analysisRepo{
		fakeRepo: &fakeRepo{
			experiments:  []branch.ExperimentRef{{ID: "brn_0123456789abcdef0123", BranchName: "branch/test-exp-0123456789abcdef0123", Head: experimentHead}},
			mainHead:     mainHead,
			compareFiles: files,
		},
		diffErr: gitstore.ErrDiffTooLarge,
	}
	analyzer := &captureAnalyzer{}
	service := branch.NewService(repo, &fakeIndex{}, mutation.NewCoordinator(), branch.SessionAdapter{PathFn: func() (string, bool) { return "/tmp/project", true }}, nil, analyzer, &staticIDs{id: "brn_0123456789abcdef0123"})
	_, err = service.AnalyzeRamifications(context.Background(), "brn_0123456789abcdef0123", branch.AnalysisRequest{
		Goal: "Review", ProfileID: "local", Model: "model",
		ExpectedMainHead: mainHead, ExpectedExperimentHead: experimentHead, ExpectedFingerprint: fingerprint,
	})
	if !errors.Is(err, branch.ErrAnalysisBudget) || analyzer.calls != 0 {
		t.Fatalf("error=%v calls=%d", err, analyzer.calls)
	}
}

// Test: more than 100 changed files fails before any blob read or provider call.
// Requirements: M8-R09.
func TestAnalyzeRamificationsRejectsFileBudgetBeforeReadingBlobs(t *testing.T) {
	t.Parallel()
	mainHead := branch.CommitID("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	experimentHead := branch.CommitID("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	files := make([]branch.ChangedFile, 0, branch.MaxAnalysisFiles+1)
	for index := 0; index <= branch.MaxAnalysisFiles; index++ {
		files = append(files, branch.ChangedFile{Path: branch.ProjectPath(fmt.Sprintf("scenes/scn_%020x.md", index)), Status: branch.StatusModified})
	}
	repo := &analysisRepo{fakeRepo: &fakeRepo{
		experiments: []branch.ExperimentRef{{ID: "brn_0123456789abcdef0123", BranchName: "branch/test-exp-0123456789abcdef0123", Head: experimentHead}},
		mainHead:    mainHead,
	}}
	repo.fakeRepo.compareFiles = files
	fingerprint, err := branch.ComputeFingerprint(mainHead, experimentHead, "cccccccccccccccccccccccccccccccccccccccc", files)
	if err != nil {
		t.Fatal(err)
	}
	analyzer := &captureAnalyzer{}
	service := branch.NewService(repo, &fakeIndex{}, mutation.NewCoordinator(), branch.SessionAdapter{PathFn: func() (string, bool) { return "/tmp/project", true }}, nil, analyzer, &staticIDs{id: "brn_0123456789abcdef0123"})
	_, err = service.AnalyzeRamifications(context.Background(), "brn_0123456789abcdef0123", branch.AnalysisRequest{Goal: "Review", ProfileID: "local", Model: "model", ExpectedMainHead: mainHead, ExpectedExperimentHead: experimentHead, ExpectedFingerprint: fingerprint})
	if !errors.Is(err, branch.ErrAnalysisBudget) || repo.reads != 0 || analyzer.calls != 0 {
		t.Fatalf("error=%v reads=%d calls=%d", err, repo.reads, analyzer.calls)
	}
}

// Test: UnifiedDiff receives the exact remaining budget after prompt overhead.
// Requirements: M8-R09, M8-R20.
func TestAnalyzeRamificationsPassesExactDiffBudget(t *testing.T) {
	t.Parallel()
	mainHead := branch.CommitID("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	experimentHead := branch.CommitID("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	files := []branch.ChangedFile{
		{Path: "outline.yaml", Status: branch.StatusModified},
		{Path: "scenes/scn_001.md", Status: branch.StatusAdded},
	}
	fingerprint, err := branch.ComputeFingerprint(mainHead, experimentHead, "cccccccccccccccccccccccccccccccccccccccc", files)
	if err != nil {
		t.Fatal(err)
	}
	goal := "Review the changes"
	repo := &analysisRepo{
		fakeRepo: &fakeRepo{
			experiments:  []branch.ExperimentRef{{ID: "brn_0123456789abcdef0123", BranchName: "branch/test-exp-0123456789abcdef0123", Head: experimentHead}},
			mainHead:     mainHead,
			compareFiles: files,
		},
		diffText: "--- a/outline.yaml\n+++ b/outline.yaml\n@@ +1,1 -1,1 @@\n-old\n+new\n",
	}
	analyzer := &captureAnalyzer{}
	service := branch.NewService(repo, &fakeIndex{}, mutation.NewCoordinator(), branch.SessionAdapter{PathFn: func() (string, bool) { return "/tmp/project", true }}, nil, analyzer, &staticIDs{id: "brn_0123456789abcdef0123"})
	_, err = service.AnalyzeRamifications(context.Background(), "brn_0123456789abcdef0123", branch.AnalysisRequest{
		Goal: goal, ProfileID: "local", Model: "model",
		ExpectedMainHead: mainHead, ExpectedExperimentHead: experimentHead, ExpectedFingerprint: fingerprint,
	})
	if err != nil {
		t.Fatalf("AnalyzeRamifications() error = %v", err)
	}
	expectedBudget := branch.MaxAnalysisPacket - branch.AnalysisPromptOverhead(goal, files)
	if repo.diffBudget != expectedBudget {
		t.Fatalf("diffBudget = %d, want %d", repo.diffBudget, expectedBudget)
	}
}

// Test: malformed profile identifiers and model values fail before provider
// execution or repository diff work.
// Requirements: M8-R09, M8-R11.
func TestAnalyzeRamificationsRejectsInvalidProfileAndModelBeforeProvider(t *testing.T) {
	t.Parallel()
	mainHead := branch.CommitID("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	experimentHead := branch.CommitID("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	files := []branch.ChangedFile{{Path: "outline.yaml", Status: branch.StatusModified}}
	fingerprint, err := branch.ComputeFingerprint(mainHead, experimentHead, "cccccccccccccccccccccccccccccccccccccccc", files)
	if err != nil {
		t.Fatal(err)
	}
	repo := &analysisRepo{
		fakeRepo: &fakeRepo{
			experiments:  []branch.ExperimentRef{{ID: "brn_0123456789abcdef0123", BranchName: "branch/test-exp-0123456789abcdef0123", Head: experimentHead}},
			mainHead:     mainHead,
			compareFiles: files,
		},
	}
	analyzer := &captureAnalyzer{}
	service := branch.NewService(repo, &fakeIndex{}, mutation.NewCoordinator(), branch.SessionAdapter{PathFn: func() (string, bool) { return "/tmp/project", true }}, nil, analyzer, &staticIDs{id: "brn_0123456789abcdef0123"})
	_, err = service.AnalyzeRamifications(context.Background(), "brn_0123456789abcdef0123", branch.AnalysisRequest{
		Goal: "Review", ProfileID: "bad-profile-id", Model: "",
		ExpectedMainHead: mainHead, ExpectedExperimentHead: experimentHead, ExpectedFingerprint: fingerprint,
	})
	if !errors.Is(err, branch.ErrInvalidAnalysis) || analyzer.calls != 0 || repo.diffBudget != 0 {
		t.Fatalf("error=%v calls=%d diffBudget=%d", err, analyzer.calls, repo.diffBudget)
	}
}
